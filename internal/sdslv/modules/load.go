// Package modules loads and binds an SDSL-V source graph before validation.
package modules

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/diagnostic"
	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/source"
)

// Options bounds loading to explicit local files and declared namespace roots.
// There are no package downloads, implicit directory scans, or shader execution.
type Options struct{ SearchPaths []string }

// Error retains the owning source location at compiler and conformance boundaries.
type Error struct{ Diagnostic diagnostic.Diagnostic }

func (e *Error) Error() string { return diagnostic.Render(e.Diagnostic) }

func symbolKind(decl ast.Decl) string {
	switch decl.(type) {
	case ast.FunctionDecl:
		return "function"
	case ast.ConfigDecl:
		return "config"
	case ast.CompileDecl:
		return "compile"
	default:
		return "type"
	}
}

type unit struct {
	abs, path string
	prefix    string
	module    ast.Module
	bytes     []byte
	imports   map[string]*unit
	names     map[string]string
	decls     map[string][]ast.Decl
}

func (u *unit) bound(name, kind string) string {
	bound := u.names[name]
	if u.prefix != "" && kind != "type" && len(u.decls[name]) > 1 {
		return bound + "_" + kind
	}
	return bound
}

type loader struct {
	entry      string
	options    Options
	units      map[string]*unit
	namespaces map[string]*unit
	active     map[string]bool
	stack      []string
	order      []*unit
	totalBytes int
}

func Load(path string) (ast.Module, error) { return LoadWithOptions(path, Options{}) }

func LoadWithOptions(path string, options Options) (ast.Module, error) {
	abs, err := canonical(path)
	if err != nil {
		return ast.Module{}, err
	}
	if len(options.SearchPaths) == 0 {
		options.SearchPaths = []string{filepath.Dir(abs)}
	}
	l := loader{entry: abs, options: options, units: map[string]*unit{}, namespaces: map[string]*unit{}, active: map[string]bool{}}
	root, err := l.load(path, source.Span{})
	if err != nil {
		return ast.Module{}, err
	}
	// Preserve all existing single-file artifacts, identities, and span formats.
	if len(root.module.Imports) == 0 {
		return root.module, nil
	}
	out := root.module
	out.Decls = nil
	all := map[string][]ast.Decl{}
	var ordered []string
	for _, u := range l.order {
		b := binder{unit: u}
		for _, decl := range u.module.Decls {
			bound, err := b.declaration(decl)
			if err != nil {
				return ast.Module{}, err
			}
			name, _ := ast.Declaration(bound)
			if u == root {
				out.Decls = append(out.Decls, bound)
			} else {
				all[name] = append(all[name], bound)
				ordered = append(ordered, name)
			}
		}
	}
	// Only selected library declarations and their private closure enter the
	// compiler. Imported concrete shader/test entries are never implicit roots.
	needed := map[string]bool{}
	var require func(ast.Decl)
	require = func(decl ast.Decl) {
		visitStrings(reflect.ValueOf(decl), func(name string) {
			dependency, ok := all[name]
			if !ok || needed[name] {
				return
			}
			needed[name] = true
			for _, decl := range dependency {
				require(decl)
			}
		})
	}
	for _, decl := range out.Decls {
		require(decl)
	}
	var dependencies []ast.Decl
	emitted := map[string]bool{}
	for _, name := range ordered {
		if needed[name] && !emitted[name] {
			dependencies = append(dependencies, all[name]...)
			emitted[name] = true
		}
	}
	out.Decls = append(dependencies, out.Decls...)
	for _, u := range l.order {
		relative, err := filepath.Rel(filepath.Dir(l.entry), u.abs)
		if err != nil {
			return ast.Module{}, err
		}
		sum := sha256.Sum256(u.bytes)
		out.Dependencies = append(out.Dependencies, ast.Dependency{Path: filepath.ToSlash(relative), Namespace: u.module.Namespace, SHA256: hex.EncodeToString(sum[:])})
	}
	sort.Slice(out.Dependencies, func(i, j int) bool { return out.Dependencies[i].Path < out.Dependencies[j].Path })
	encoded, err := json.Marshal(out.Dependencies)
	if err != nil {
		return ast.Module{}, err
	}
	sum := sha256.Sum256(encoded)
	out.DependencyIdentity = hex.EncodeToString(sum[:])
	return out, nil
}

func canonical(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

func fail(span source.Span, code, format string, args ...any) error {
	return &Error{Diagnostic: diagnostic.Diagnostic{Path: span.Path, Span: span, Code: code, Severity: diagnostic.SeverityError, Message: fmt.Sprintf(format, args...)}}
}

func (l *loader) load(path string, importedAt source.Span) (*unit, error) {
	abs, err := canonical(path)
	if err != nil {
		if importedAt.Known() {
			return nil, fail(importedAt, "SDSL-V4401", "cannot load import %q: %v", path, err)
		}
		return nil, err
	}
	if l.active[abs] {
		return nil, fail(importedAt, "SDSL-V4402", "import cycle: %s -> %s", strings.Join(l.stack, " -> "), filepath.ToSlash(path))
	}
	if u, ok := l.units[abs]; ok {
		return u, nil
	}
	if len(l.units) >= 256 {
		return nil, fail(importedAt, "SDSL-V4408", "source graph exceeds 256 modules")
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	l.totalBytes += len(data)
	if len(data) > 8*1024*1024 || l.totalBytes > 64*1024*1024 {
		return nil, fail(importedAt, "SDSL-V4408", "source graph exceeds its byte budget")
	}
	file := source.File{Path: filepath.Clean(path), Text: strings.TrimPrefix(string(data), "\ufeff")}
	tokens, err := lex.Analyze(file)
	if err != nil {
		return nil, err
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		return nil, err
	}
	u := &unit{abs: abs, path: file.Path, module: module, bytes: data, imports: map[string]*unit{}, names: map[string]string{}, decls: map[string][]ast.Decl{}}
	if abs == l.entry && len(module.Imports) == 0 {
		return u, nil
	}
	if abs != l.entry {
		if module.Namespace == "" {
			return nil, fail(importedAt, "SDSL-V4404", "imported library %q must declare its namespace", file.Path)
		}
	}
	if module.Namespace != "" {
		if prior, exists := l.namespaces[module.Namespace]; exists && prior.abs != abs {
			return nil, fail(importedAt, "SDSL-V4404", "namespace %s belongs to both %s and %s", module.Namespace, prior.path, file.Path)
		}
		l.namespaces[module.Namespace] = u
	}
	prefix := ""
	if abs != l.entry {
		sum := sha256.Sum256([]byte(module.Namespace))
		prefix = "__sdslv_module_" + hex.EncodeToString(sum[:6]) + "_"
	}
	u.prefix = prefix
	for _, decl := range module.Decls {
		name, span := ast.Declaration(decl)
		span.Path = file.Path
		if name == "" {
			continue
		}
		for _, prior := range u.decls[name] {
			if symbolKind(prior) == symbolKind(decl) {
				return nil, fail(span, "SDSL-V1509", "duplicate %s declaration %s", symbolKind(decl), name)
			}
		}
		u.decls[name], u.names[name] = append(u.decls[name], decl), prefix+name
		if abs != l.entry {
			switch d := decl.(type) {
			case ast.ShaderDecl:
				if d.Template == nil {
					return nil, fail(span, "SDSL-V4406", "imported libraries declare helpers, types, configs, and shader templates; concrete shader %s must be an entry module", name)
				}
			case ast.CompileDecl:
				return nil, fail(span, "SDSL-V4406", "compile materializations belong in the entry module")
			case ast.FunctionDecl:
				for _, attr := range d.Attributes {
					if attr.Name == "Fact" || attr.Name == "Theory" || attr.Name == "Benchmark" {
						return nil, fail(span, "SDSL-V4406", "test and benchmark entries cannot be imported")
					}
				}
			}
		}
	}
	l.units[abs], l.active[abs] = u, true
	l.stack = append(l.stack, filepath.ToSlash(path))
	defer func() { l.active[abs] = false; l.stack = l.stack[:len(l.stack)-1] }()
	for _, imp := range module.Imports {
		imp.Span.Path = file.Path
		resolved, err := l.resolve(u, imp)
		if err != nil {
			return nil, err
		}
		child, err := l.load(resolved, imp.Span)
		if err != nil {
			return nil, err
		}
		if !imp.Relative && child.module.Namespace != imp.Path {
			return nil, fail(imp.Span, "SDSL-V4404", "import %s resolved to namespace %s", imp.Path, child.module.Namespace)
		}
		alias := imp.Alias
		if alias == "" {
			parts := strings.Split(child.module.Namespace, ".")
			alias = parts[len(parts)-1]
		}
		if _, exists := u.imports[alias]; exists {
			return nil, fail(imp.Span, "SDSL-V4405", "duplicate import alias %s", alias)
		}
		if _, exists := u.decls[alias]; exists {
			return nil, fail(imp.Span, "SDSL-V4405", "import alias %s conflicts with a declaration", alias)
		}
		u.imports[alias] = child
	}
	l.order = append(l.order, u)
	return u, nil
}

func (l *loader) resolve(parent *unit, imp ast.Import) (string, error) {
	if imp.Relative {
		if filepath.IsAbs(imp.Path) || strings.Contains(imp.Path, ":") {
			return "", fail(imp.Span, "SDSL-V4401", "file imports must be local relative paths")
		}
		path := imp.Path
		if filepath.Ext(path) == "" {
			path += ".sdslv"
		}
		if filepath.Ext(path) != ".sdslv" {
			return "", fail(imp.Span, "SDSL-V4401", "library imports require .sdslv source")
		}
		return filepath.Clean(filepath.Join(filepath.Dir(parent.path), filepath.FromSlash(path))), nil
	}
	var found []string
	seen := map[string]bool{}
	for _, root := range l.options.SearchPaths {
		path := filepath.Join(root, filepath.FromSlash(strings.ReplaceAll(imp.Path, ".", "/"))+".sdslv")
		abs, err := canonical(path)
		if err == nil && !seen[abs] {
			found = append(found, path)
			seen[abs] = true
		}
	}
	if len(found) != 1 {
		return "", fail(imp.Span, "SDSL-V4401", "namespace import %s resolved to %d sources in declared roots", imp.Path, len(found))
	}
	return found[0], nil
}

func visitStrings(value reflect.Value, visit func(string)) {
	if !value.IsValid() {
		return
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if !value.IsNil() {
			visitStrings(value.Elem(), visit)
		}
	case reflect.Struct:
		if value.Type() == reflect.TypeOf(source.Span{}) || value.Type() == reflect.TypeOf(ast.StringLiteral{}) {
			return
		}
		for i := 0; i < value.NumField(); i++ {
			visitStrings(value.Field(i), visit)
		}
	case reflect.Slice:
		for i := 0; i < value.Len(); i++ {
			visitStrings(value.Index(i), visit)
		}
	case reflect.String:
		visit(value.String())
	}
}
