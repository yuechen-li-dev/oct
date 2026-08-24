// Package templatecatalog discovers documented *.template.oct declarations.
// The suffix is a tooling convention only: files are parsed as ordinary Oct.
package templatecatalog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/lex"
	"github.com/yuechen-li-dev/oct/internal/parse"
	"github.com/yuechen-li-dev/oct/internal/source"
)

type Entry struct {
	Name               string   `json:"name"`
	Kind               string   `json:"kind"`
	Category           string   `json:"category"`
	Summary            string   `json:"summary"`
	TypeParameters     []string `json:"type_parameters"`
	ConfigurableFields []string `json:"configurable_fields"`
	Requirements       []string `json:"requirements"`
	RequirementKinds   []string `json:"requirement_enforcement"`
	ProvidedSemantics  []string `json:"provided_semantics"`
	UseWhen            []string `json:"use_when"`
	AvoidWhen          []string `json:"avoid_when"`
	SourcePath         string   `json:"source_path"`
}

func Load(root string) ([]Entry, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve catalog root: %w", err)
	}
	var paths []string
	err = filepath.WalkDir(absRoot, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".template.oct") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover template catalog: %w", err)
	}
	sort.Strings(paths)
	entries := make([]Entry, 0)
	categoryConcepts := make(map[string]ast.ConceptDecl)
	for _, path := range paths {
		fileEntries, concepts, err := loadFile(absRoot, path)
		if err != nil {
			return nil, err
		}
		for _, concept := range concepts {
			if _, exists := categoryConcepts[concept.Name]; exists {
				return nil, fmt.Errorf("duplicate catalog category Concept %s", concept.Name)
			}
			categoryConcepts[concept.Name] = concept
		}
		entries = append(entries, fileEntries...)
	}
	for _, entry := range entries {
		concept, ok := categoryConcepts[entry.Category]
		if !ok {
			return nil, fmt.Errorf("template %s in %s Category %s is not a catalog Concept", entry.Name, entry.SourcePath, entry.Category)
		}
		if formatType(concept.Target) != "String" || len(concept.Requirements) == 0 {
			return nil, fmt.Errorf("template %s Category Concept %s must refine String with Require", entry.Name, entry.Category)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Category != entries[j].Category {
			return entries[i].Category < entries[j].Category
		}
		if entries[i].Name != entries[j].Name {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].SourcePath < entries[j].SourcePath
	})
	return entries, nil
}

func loadFile(root, path string) ([]Entry, []ast.ConceptDecl, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read template source %s: %w", path, err)
	}
	fileSource := source.File{Path: path, Text: string(data)}
	lexed, err := lex.Analyze(fileSource)
	if err != nil {
		return nil, nil, fmt.Errorf("lex template source %s: %w", path, err)
	}
	parsed, err := parse.BuildFile(lexed)
	if err != nil {
		return nil, nil, fmt.Errorf("parse template source %s: %w", path, err)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)
	entries := make([]Entry, 0)
	for _, decl := range parsed.Records {
		if decl.IsTemplate {
			fields := make([]string, 0, len(decl.Fields))
			for _, field := range decl.Fields {
				fields = append(fields, field.Name+": "+formatType(field.Type))
			}
			entry, err := documentedEntry(decl.Name, "record", decl.TypeParameters, fields, decl.Doc, rel)
			if err != nil {
				return nil, nil, err
			}
			entries = append(entries, entry)
		}
	}
	for _, decl := range parsed.Functions {
		if decl.IsTemplate {
			params := make([]string, 0, len(decl.Parameters))
			for _, param := range decl.Parameters {
				params = append(params, param.Name+": "+formatType(param.Type))
			}
			entry, err := documentedEntry(decl.Name, "function", decl.TypeParameters, params, decl.Doc, rel)
			if err != nil {
				return nil, nil, err
			}
			entries = append(entries, entry)
		}
	}
	for _, decl := range parsed.Flows {
		if decl.IsTemplate {
			params := make([]string, 0, len(decl.Parameters))
			for _, param := range decl.Parameters {
				params = append(params, param.Name+": "+formatType(param.Type))
			}
			kind := "flow"
			if sourceDeclKind(string(data), decl.Name) == "query" {
				kind = "query"
			}
			entry, err := documentedEntry(decl.Name, kind, decl.TypeParameters, params, nil, rel)
			if err != nil {
				// Query-M0 lowers syntax to FLOW before docs are attached to the
				// resulting declaration. Recover only the declaration's adjacent
				// ordinary /// block; execution semantics remain parser-owned.
				doc := sourceDocForDeclaration(string(data), kind, decl.Name)
				entry, err = documentedEntry(decl.Name, kind, decl.TypeParameters, params, doc, rel)
			}
			if err != nil {
				return nil, nil, err
			}
			entries = append(entries, entry)
		}
	}
	return entries, parsed.Concepts, nil
}

func documentedEntry(name, kind string, typeParams, fields []string, doc *ast.DocComment, sourcePath string) (Entry, error) {
	entry := Entry{Name: name, Kind: kind, TypeParameters: append([]string(nil), typeParams...), ConfigurableFields: append([]string(nil), fields...), SourcePath: sourcePath}
	if doc != nil {
		for _, line := range doc.Lines {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "Category:"):
				entry.Category = strings.TrimSpace(strings.TrimPrefix(line, "Category:"))
			case strings.HasPrefix(line, "Requires["):
				end := strings.Index(line, "]:")
				if end < 0 {
					return Entry{}, fmt.Errorf("template %s in %s has malformed Requires enforcement", name, sourcePath)
				}
				kind := strings.TrimSpace(strings.TrimPrefix(line[:end], "Requires["))
				switch kind {
				case "Require", "Type", "Structure", "Application":
				default:
					return Entry{}, fmt.Errorf("template %s in %s has unknown Requires enforcement %s", name, sourcePath, kind)
				}
				entry.RequirementKinds = append(entry.RequirementKinds, kind)
				entry.Requirements = append(entry.Requirements, strings.TrimSpace(line[end+2:]))
			case strings.HasPrefix(line, "Requires:"):
				return Entry{}, fmt.Errorf("template %s in %s must classify Requires as Require, Type, Structure, or Application", name, sourcePath)
			case strings.HasPrefix(line, "Provides:"):
				entry.ProvidedSemantics = append(entry.ProvidedSemantics, strings.TrimSpace(strings.TrimPrefix(line, "Provides:")))
			case strings.HasPrefix(line, "Use when:"):
				entry.UseWhen = append(entry.UseWhen, strings.TrimSpace(strings.TrimPrefix(line, "Use when:")))
			case strings.HasPrefix(line, "Avoid when:"):
				entry.AvoidWhen = append(entry.AvoidWhen, strings.TrimSpace(strings.TrimPrefix(line, "Avoid when:")))
			case entry.Summary == "" && line != "":
				entry.Summary = line
			}
		}
	}
	if entry.Category == "" || entry.Summary == "" || len(entry.Requirements) == 0 || len(entry.ProvidedSemantics) == 0 {
		return Entry{}, fmt.Errorf("template %s in %s requires summary, Category, Requires, and Provides documentation", name, sourcePath)
	}
	return entry, nil
}

func sourceDeclKind(text, name string) string {
	for _, kind := range []string{"query", "flow"} {
		if strings.Contains(text, "template "+kind+" "+name+"<") {
			return kind
		}
	}
	return "flow"
}

func sourceDocForDeclaration(text, kind, name string) *ast.DocComment {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	needle := "template " + kind + " " + name + "<"
	for i, line := range lines {
		if !strings.Contains(line, needle) {
			continue
		}
		var reversed []string
		for j := i - 1; j >= 0; j-- {
			trimmed := strings.TrimSpace(lines[j])
			if !strings.HasPrefix(trimmed, "///") {
				break
			}
			reversed = append(reversed, strings.TrimSpace(strings.TrimPrefix(trimmed, "///")))
		}
		docLines := make([]string, len(reversed))
		for j := range reversed {
			docLines[len(reversed)-1-j] = reversed[j]
		}
		return &ast.DocComment{Lines: docLines}
	}
	return nil
}

func formatType(t ast.TypeRef) string {
	if t.Function != nil {
		params := make([]string, len(t.Function.Parameters))
		for i := range t.Function.Parameters {
			params[i] = formatType(t.Function.Parameters[i])
		}
		result := "fn(" + strings.Join(params, ", ") + ") -> " + formatType(t.Function.ReturnType)
		if t.Function.IsFallible && t.Function.ErrorType != nil {
			result += " ! " + formatType(*t.Function.ErrorType)
		}
		return result + strings.Repeat("[]", t.ArrayDepth)
	}
	name := t.Name
	if t.Package != "" {
		name = t.Package + "." + name
	}
	if len(t.TypeArguments) > 0 {
		args := make([]string, len(t.TypeArguments))
		for i := range t.TypeArguments {
			args[i] = formatType(t.TypeArguments[i])
		}
		name += "<" + strings.Join(args, ", ") + ">"
	}
	depth := t.ArrayDepth
	if t.IsArray && depth == 0 {
		depth = 1
	}
	return name + strings.Repeat("[]", depth)
}
