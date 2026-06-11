package pkgmgr

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/lex"
	"github.com/yuechen-li-dev/oct/internal/newpkg"
	"github.com/yuechen-li-dev/oct/internal/parse"
	"github.com/yuechen-li-dev/oct/internal/source"
)

const (
	RegistriesConfigRelPath = ".oct/registries.oct"
	RegistryIndexFileName   = "registry.oct"
	ProjectPackagesRelDir   = ".oct/packages"
	PackageSourceFileName   = ".oct-package-source.oct"
)

type RegistrySource struct {
	Name string
	Path string
}

type RegistryConfig struct {
	Registries []RegistrySource
}

type PackageEntry struct {
	Name        string
	Version     string
	Kind        string
	SourceKind  string
	Source      string
	Ref         string
	Path        string
	Description string
}

type RegistryIndex struct {
	Packages []PackageEntry
}

type ResolvedPackage struct {
	Registry       RegistrySource
	RegistryRoot   string
	Entry          PackageEntry
	ResolvedCommit string
}

type RegistrySyncResult struct {
	Name           string
	Version        string
	Registry       string
	SourceKind     string
	Ref            string
	ResolvedCommit string
	Destination    string
	Chain          []string
}

var registryNamePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

func LoadRegistryConfig(projectRoot string) (RegistryConfig, error) {
	path := filepath.Join(projectRoot, RegistriesConfigRelPath)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return RegistryConfig{}, nil
		}
		return RegistryConfig{}, fmt.Errorf("read registry config %s: %w", path, err)
	}
	file, err := parseOctFile(path)
	if err != nil {
		return RegistryConfig{}, fmt.Errorf("parse registry config %s: %w", path, err)
	}
	return extractRegistryConfig(file)
}

func SaveRegistryConfig(projectRoot string, config RegistryConfig) error {
	path := filepath.Join(projectRoot, RegistriesConfigRelPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create registry config directory: %w", err)
	}
	return os.WriteFile(path, []byte(RenderRegistryConfig(config)), 0o644)
}

func AddRegistry(projectRoot string, name string, path string) (RegistryConfig, error) {
	name = strings.TrimSpace(name)
	if err := ValidateRegistryName(name); err != nil {
		return RegistryConfig{}, err
	}
	if strings.TrimSpace(path) == "" {
		return RegistryConfig{}, fmt.Errorf("registry path must not be empty")
	}
	config, err := LoadRegistryConfig(projectRoot)
	if err != nil {
		return RegistryConfig{}, err
	}
	for _, reg := range config.Registries {
		if reg.Name == name {
			return RegistryConfig{}, fmt.Errorf("package registry %q is already configured", name)
		}
	}
	config.Registries = append(config.Registries, RegistrySource{Name: name, Path: path})
	if err := SaveRegistryConfig(projectRoot, config); err != nil {
		return RegistryConfig{}, err
	}
	return config, nil
}

func RemoveRegistry(projectRoot string, name string) (RegistryConfig, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return RegistryConfig{}, fmt.Errorf("registry name must not be empty")
	}
	config, err := LoadRegistryConfig(projectRoot)
	if err != nil {
		return RegistryConfig{}, err
	}
	out := make([]RegistrySource, 0, len(config.Registries))
	removed := false
	for _, reg := range config.Registries {
		if reg.Name == name {
			removed = true
			continue
		}
		out = append(out, reg)
	}
	if !removed {
		return RegistryConfig{}, fmt.Errorf("package registry %q is not configured", name)
	}
	config.Registries = out
	if err := SaveRegistryConfig(projectRoot, config); err != nil {
		return RegistryConfig{}, err
	}
	return config, nil
}

func ValidateRegistryName(name string) error {
	if name == "" {
		return fmt.Errorf("registry name must not be empty")
	}
	if !registryNamePattern.MatchString(name) {
		return fmt.Errorf("invalid registry name %q: must match [A-Za-z][A-Za-z0-9_-]*", name)
	}
	return nil
}

func RenderRegistryConfig(config RegistryConfig) string {
	var b strings.Builder
	b.WriteString("package Registries\n\n")
	b.WriteString("record RegistryConfig {\n")
	b.WriteString("    Registries: RegistrySource[]\n")
	b.WriteString("}\n\n")
	b.WriteString("record RegistrySource {\n")
	b.WriteString("    Name: String\n")
	b.WriteString("    Path: String\n")
	b.WriteString("}\n\n")
	b.WriteString("fn Registries() -> RegistryConfig {\n")
	b.WriteString("    return RegistryConfig {\n")
	b.WriteString("        Registries: [")
	if len(config.Registries) > 0 {
		b.WriteString("\n")
		for idx, reg := range config.Registries {
			comma := ","
			if idx == len(config.Registries)-1 {
				comma = ""
			}
			b.WriteString("            RegistrySource { Name: ")
			b.WriteString(strconv.Quote(reg.Name))
			b.WriteString(" Path: ")
			b.WriteString(strconv.Quote(reg.Path))
			b.WriteString(" }")
			b.WriteString(comma)
			b.WriteString("\n")
		}
		b.WriteString("        ")
	}
	b.WriteString("]\n")
	b.WriteString("    }\n")
	b.WriteString("}\n")
	return b.String()
}

func LoadRegistryIndex(registryRoot string) (RegistryIndex, error) {
	path := filepath.Join(registryRoot, RegistryIndexFileName)
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return RegistryIndex{}, fmt.Errorf("registry index not found: %s", path)
		}
		return RegistryIndex{}, fmt.Errorf("read registry index %s: %w", path, err)
	}
	file, err := parseOctFile(path)
	if err != nil {
		return RegistryIndex{}, fmt.Errorf("parse registry index %s: %w", path, err)
	}
	return extractRegistryIndex(file)
}

func ResolveRegistryPackage(projectRoot string, name string, version string, registryName string) (ResolvedPackage, error) {
	if err := newpkg.ValidateName(name); err != nil {
		return ResolvedPackage{}, err
	}
	if err := ValidateExactVersion(version); err != nil {
		return ResolvedPackage{}, fmt.Errorf("dependency %s: %w", name, err)
	}
	config, err := LoadRegistryConfig(projectRoot)
	if err != nil {
		return ResolvedPackage{}, err
	}
	if len(config.Registries) == 0 {
		return ResolvedPackage{}, fmt.Errorf("No package registries configured. Use oct pkg registry add <name> <path>.")
	}
	regs := config.Registries
	if registryName != "" {
		found := false
		for _, reg := range config.Registries {
			if reg.Name == registryName {
				regs = []RegistrySource{reg}
				found = true
				break
			}
		}
		if !found {
			return ResolvedPackage{}, fmt.Errorf("unknown package registry %q", registryName)
		}
	}
	var matches []ResolvedPackage
	searched := make([]string, 0, len(regs))
	for _, reg := range regs {
		searched = append(searched, reg.Name)
		root := resolveRegistryRoot(projectRoot, reg.Path)
		idx, err := LoadRegistryIndex(root)
		if err != nil {
			return ResolvedPackage{}, fmt.Errorf("registry %s: %w", reg.Name, err)
		}
		for _, entry := range idx.Packages {
			if entry.Name == name && entry.Version == version {
				matches = append(matches, ResolvedPackage{Registry: reg, RegistryRoot: root, Entry: entry})
			}
		}
	}
	if len(matches) == 0 {
		return ResolvedPackage{}, fmt.Errorf("dependency %s %s was not found in registries: %s", name, version, strings.Join(searched, ", "))
	}
	if len(matches) > 1 {
		names := make([]string, 0, len(matches))
		for _, match := range matches {
			names = append(names, match.Registry.Name)
		}
		return ResolvedPackage{}, fmt.Errorf("dependency %s %s is available in multiple registries: %s; rerun with --registry <name>", name, version, strings.Join(names, ", "))
	}
	return matches[0], nil
}

func ValidateExactVersion(version string) error {
	version = strings.TrimSpace(version)
	if version == "" {
		return fmt.Errorf("version must not be empty")
	}
	if version == "latest" || strings.HasPrefix(version, "^") || strings.HasPrefix(version, "~") || strings.HasPrefix(version, ">=") || strings.HasPrefix(version, "<=") || strings.HasPrefix(version, ">") || strings.HasPrefix(version, "<") || strings.Contains(version, "*") || strings.Contains(version, " ") {
		return fmt.Errorf("version %q is not an exact version; PM2 supports exact versions only", version)
	}
	return nil
}

func SyncRegistryDependency(projectRoot string, dep DependencyMetadata) (RegistrySyncResult, error) {
	resolved, err := ResolveRegistryPackage(projectRoot, dep.Name, dep.VersionRequirement, "")
	if err != nil {
		return RegistrySyncResult{}, err
	}
	return syncResolvedRegistryPackage(projectRoot, resolved)
}

func syncResolvedRegistryPackage(projectRoot string, resolved ResolvedPackage) (RegistrySyncResult, error) {
	entry := resolved.Entry
	switch entry.SourceKind {
	case "local":
		sourceRoot := entry.Source
		if !filepath.IsAbs(sourceRoot) {
			sourceRoot = filepath.Join(resolved.RegistryRoot, sourceRoot)
		}
		sourceRoot = filepath.Clean(sourceRoot)
		packageRoot, err := safePackageSourcePath(sourceRoot, entry.Path)
		if err != nil {
			return RegistrySyncResult{}, fmt.Errorf("dependency %s %s from registry %s: %w", entry.Name, entry.Version, resolved.Registry.Name, err)
		}
		return installResolvedPackage(projectRoot, resolved, packageRoot)
	case "git":
		return syncGitRegistryPackage(projectRoot, resolved)
	default:
		return RegistrySyncResult{}, fmt.Errorf("dependency %s %s from registry %s has unsupported SourceKind %q; supported SourceKind values are local, git", entry.Name, entry.Version, resolved.Registry.Name, entry.SourceKind)
	}
}

func syncGitRegistryPackage(projectRoot string, resolved ResolvedPackage) (RegistrySyncResult, error) {
	entry := resolved.Entry
	baseDir := filepath.Join(projectRoot, ProjectPackagesRelDir, entry.Name)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return RegistrySyncResult{}, fmt.Errorf("create package cache directory: %w", err)
	}
	cloneDir, err := os.MkdirTemp(baseDir, ".tmp-git-*")
	if err != nil {
		return RegistrySyncResult{}, fmt.Errorf("create temporary git clone directory: %w", err)
	}
	defer os.RemoveAll(cloneDir)
	if err := runGitPackageCommand(entry, resolved.Registry.Name, "clone", "git", "clone", entry.Source, cloneDir); err != nil {
		return RegistrySyncResult{}, err
	}
	if err := runGitPackageCommand(entry, resolved.Registry.Name, "checkout", "git", "-C", cloneDir, "checkout", "--detach", entry.Ref); err != nil {
		return RegistrySyncResult{}, err
	}
	commit, err := runGitPackageOutput(entry, resolved.Registry.Name, "rev-parse", "git", "-C", cloneDir, "rev-parse", "HEAD")
	if err != nil {
		return RegistrySyncResult{}, err
	}
	resolved.ResolvedCommit = commit
	packageRoot, err := safePackageSourcePath(cloneDir, entry.Path)
	if err != nil {
		return RegistrySyncResult{}, fmt.Errorf("dependency %s %s from registry %s git source %s ref %s: %w", entry.Name, entry.Version, resolved.Registry.Name, entry.Source, entry.Ref, err)
	}
	return installResolvedPackage(projectRoot, resolved, packageRoot)
}

func installResolvedPackage(projectRoot string, resolved ResolvedPackage, packageRoot string) (RegistrySyncResult, error) {
	entry := resolved.Entry
	if _, err := os.Stat(filepath.Join(packageRoot, manifestFileName)); err != nil {
		if os.IsNotExist(err) {
			return RegistrySyncResult{}, fmt.Errorf("dependency %s %s from registry %s source %s has no manifest.oct", entry.Name, entry.Version, resolved.Registry.Name, packageRoot)
		}
		return RegistrySyncResult{}, fmt.Errorf("read dependency %s %s source manifest: %w", entry.Name, entry.Version, err)
	}
	baseDir := filepath.Join(projectRoot, ProjectPackagesRelDir, entry.Name)
	finalDir := filepath.Join(baseDir, entry.Version)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return RegistrySyncResult{}, fmt.Errorf("create package cache directory: %w", err)
	}
	tmp, err := os.MkdirTemp(baseDir, ".tmp-install-*")
	if err != nil {
		return RegistrySyncResult{}, fmt.Errorf("create temporary package sync directory: %w", err)
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(tmp)
		}
	}()
	if err := copyPackageSource(packageRoot, tmp); err != nil {
		return RegistrySyncResult{}, err
	}
	if err := validateCopiedManifest(tmp, entry); err != nil {
		return RegistrySyncResult{}, fmt.Errorf("dependency %s %s from registry %s source %s: %w", entry.Name, entry.Version, resolved.Registry.Name, packageRoot, err)
	}
	if err := writePackageSourceMetadata(tmp, resolved); err != nil {
		return RegistrySyncResult{}, err
	}
	if err := os.RemoveAll(finalDir); err != nil {
		return RegistrySyncResult{}, fmt.Errorf("replace synced package %s: %w", finalDir, err)
	}
	if err := os.Rename(tmp, finalDir); err != nil {
		return RegistrySyncResult{}, fmt.Errorf("install synced package %s: %w", finalDir, err)
	}
	cleanup = false
	return RegistrySyncResult{Name: entry.Name, Version: entry.Version, Registry: resolved.Registry.Name, SourceKind: entry.SourceKind, Ref: entry.Ref, ResolvedCommit: resolved.ResolvedCommit, Destination: finalDir}, nil
}

func runGitPackageCommand(entry PackageEntry, registry string, operation string, name string, args ...string) error {
	_, err := runGitPackageOutput(entry, registry, operation, name, args...)
	return err
}

func runGitPackageOutput(entry PackageEntry, registry string, operation string, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("git executable not found while syncing %s %s from registry %s source %s ref %s during %s", entry.Name, entry.Version, registry, entry.Source, entry.Ref, operation)
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("git %s timed out while syncing %s %s from registry %s source %s ref %s", operation, entry.Name, entry.Version, registry, entry.Source, entry.Ref)
		}
		if trimmed == "" {
			trimmed = err.Error()
		}
		return "", fmt.Errorf("git %s failed while syncing %s %s from registry %s source %s ref %s: %s", operation, entry.Name, entry.Version, registry, entry.Source, entry.Ref, trimmed)
	}
	return trimmed, nil
}

func parseOctFile(path string) (ast.File, error) {
	file, err := source.Load(path)
	if err != nil {
		return ast.File{}, err
	}
	lexed, err := lex.Analyze(file)
	if err != nil {
		return ast.File{}, err
	}
	return parse.BuildFile(lexed)
}

func extractRegistryConfig(file ast.File) (RegistryConfig, error) {
	if file.Package != "Registries" {
		return RegistryConfig{}, fmt.Errorf("registries config must declare package Registries")
	}
	if err := requireRecordShape(file, "RegistryConfig", map[string]ast.TypeRef{"Registries": {Name: "RegistrySource", IsArray: true}}, nil); err != nil {
		return RegistryConfig{}, err
	}
	if err := requireRecordShape(file, "RegistrySource", map[string]ast.TypeRef{"Name": {Name: "String"}, "Path": {Name: "String"}}, nil); err != nil {
		return RegistryConfig{}, err
	}
	fn, ok := findNoArgFunction(file, "Registries", "RegistryConfig")
	if !ok {
		return RegistryConfig{}, fmt.Errorf("registries config must define fn Registries() -> RegistryConfig")
	}
	record, err := singleReturnRecord(fn, "RegistryConfig")
	if err != nil {
		return RegistryConfig{}, err
	}
	fields, err := literalFields(record, map[string]bool{"Registries": true})
	if err != nil {
		return RegistryConfig{}, err
	}
	arr, ok := fields["Registries"].(ast.ArrayLiteralExpr)
	if !ok {
		return RegistryConfig{}, fmt.Errorf("registry config field 'Registries' must be a RegistrySource[] literal")
	}
	config := RegistryConfig{Registries: make([]RegistrySource, 0, len(arr.Elements))}
	seen := map[string]bool{}
	for idx, expr := range arr.Elements {
		rec, ok := expr.(ast.RecordLiteralExpr)
		if !ok || rec.TypeName != "RegistrySource" {
			return RegistryConfig{}, fmt.Errorf("registry config entry at index %d must be a RegistrySource literal", idx)
		}
		entryFields, err := literalFields(rec, map[string]bool{"Name": true, "Path": true})
		if err != nil {
			return RegistryConfig{}, fmt.Errorf("registry config entry at index %d: %w", idx, err)
		}
		name, err := stringField(entryFields, "Name")
		if err != nil {
			return RegistryConfig{}, fmt.Errorf("registry config entry at index %d: %w", idx, err)
		}
		path, err := stringField(entryFields, "Path")
		if err != nil {
			return RegistryConfig{}, fmt.Errorf("registry config entry at index %d: %w", idx, err)
		}
		if err := ValidateRegistryName(name); err != nil {
			return RegistryConfig{}, fmt.Errorf("registry config entry at index %d: %w", idx, err)
		}
		if strings.TrimSpace(path) == "" {
			return RegistryConfig{}, fmt.Errorf("registry config entry at index %d has empty Path", idx)
		}
		if seen[name] {
			return RegistryConfig{}, fmt.Errorf("registry config contains duplicate registry name %q", name)
		}
		seen[name] = true
		config.Registries = append(config.Registries, RegistrySource{Name: name, Path: path})
	}
	return config, nil
}

func extractRegistryIndex(file ast.File) (RegistryIndex, error) {
	if file.Package != "Registry" {
		return RegistryIndex{}, fmt.Errorf("registry.oct must declare package Registry")
	}
	if err := requireRecordShape(file, "RegistryIndex", map[string]ast.TypeRef{"Packages": {Name: "PackageEntry", IsArray: true}}, nil); err != nil {
		return RegistryIndex{}, err
	}
	req := map[string]ast.TypeRef{"Name": {Name: "String"}, "Version": {Name: "String"}, "Kind": {Name: "String"}, "SourceKind": {Name: "String"}, "Source": {Name: "String"}, "Path": {Name: "String"}, "Description": {Name: "String"}}
	optional := map[string]ast.TypeRef{"Ref": {Name: "String"}}
	if err := requireRecordShape(file, "PackageEntry", req, optional); err != nil {
		return RegistryIndex{}, err
	}
	fn, ok := findNoArgFunction(file, "Registry", "RegistryIndex")
	if !ok {
		return RegistryIndex{}, fmt.Errorf("registry.oct must define fn Registry() -> RegistryIndex")
	}
	record, err := singleReturnRecord(fn, "RegistryIndex")
	if err != nil {
		return RegistryIndex{}, err
	}
	fields, err := literalFields(record, map[string]bool{"Packages": true})
	if err != nil {
		return RegistryIndex{}, err
	}
	arr, ok := fields["Packages"].(ast.ArrayLiteralExpr)
	if !ok {
		return RegistryIndex{}, fmt.Errorf("registry field 'Packages' must be a PackageEntry[] literal")
	}
	idx := RegistryIndex{Packages: make([]PackageEntry, 0, len(arr.Elements))}
	seen := map[string]bool{}
	allowed := map[string]bool{"Name": true, "Version": true, "Kind": true, "SourceKind": true, "Source": true, "Ref": true, "Path": true, "Description": true}
	for i, expr := range arr.Elements {
		rec, ok := expr.(ast.RecordLiteralExpr)
		if !ok || rec.TypeName != "PackageEntry" {
			return RegistryIndex{}, fmt.Errorf("registry package entry at index %d must be a PackageEntry literal", i)
		}
		fs, err := literalFields(rec, allowed)
		if err != nil {
			return RegistryIndex{}, fmt.Errorf("registry package entry at index %d: %w", i, err)
		}
		entry := PackageEntry{}
		for _, field := range []struct {
			name string
			dst  *string
		}{{"Name", &entry.Name}, {"Version", &entry.Version}, {"Kind", &entry.Kind}, {"SourceKind", &entry.SourceKind}, {"Source", &entry.Source}, {"Path", &entry.Path}, {"Description", &entry.Description}} {
			value, err := stringField(fs, field.name)
			if err != nil {
				return RegistryIndex{}, fmt.Errorf("registry package entry at index %d: %w", i, err)
			}
			*field.dst = value
		}
		ref, err := optionalStringField(fs, "Ref")
		if err != nil {
			return RegistryIndex{}, fmt.Errorf("registry package entry at index %d: %w", i, err)
		}
		entry.Ref = ref
		if err := validatePackageEntry(entry); err != nil {
			return RegistryIndex{}, fmt.Errorf("registry package entry at index %d: %w", i, err)
		}
		key := entry.Name + "\x00" + entry.Version
		if seen[key] {
			return RegistryIndex{}, fmt.Errorf("registry contains duplicate package entry %s %s", entry.Name, entry.Version)
		}
		seen[key] = true
		idx.Packages = append(idx.Packages, entry)
	}
	return idx, nil
}

func validatePackageEntry(entry PackageEntry) error {
	if err := newpkg.ValidateName(entry.Name); err != nil {
		return err
	}
	if err := ValidateExactVersion(entry.Version); err != nil {
		return err
	}
	switch entry.Kind {
	case "library", "experiment", "wrapper":
	default:
		return fmt.Errorf("Kind must be one of library, experiment, wrapper")
	}
	switch entry.SourceKind {
	case "local":
		if strings.TrimSpace(entry.Ref) != "" {
			return fmt.Errorf("Ref must be empty for local sources")
		}
	case "git":
		if strings.TrimSpace(entry.Ref) == "" {
			return fmt.Errorf("Ref is required for git sources")
		}
	default:
		return fmt.Errorf("SourceKind must be one of local, git")
	}
	if strings.TrimSpace(entry.Source) == "" {
		return fmt.Errorf("Source must not be empty")
	}
	if strings.TrimSpace(entry.Path) == "" {
		return fmt.Errorf("Path must not be empty")
	}
	return nil
}

func IsFullCommitSHA(ref string) bool {
	if len(ref) != 40 {
		return false
	}
	for _, r := range ref {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			continue
		}
		return false
	}
	return true
}

func findNoArgFunction(file ast.File, name string, returnType string) (ast.FunctionDecl, bool) {
	for _, fn := range file.Functions {
		if fn.Name == name && len(fn.Parameters) == 0 && fn.ReturnType.Name == returnType && fn.ReturnType.Package == "" && !fn.ReturnType.IsArray {
			return fn, true
		}
	}
	return ast.FunctionDecl{}, false
}

func singleReturnRecord(fn ast.FunctionDecl, typeName string) (ast.RecordLiteralExpr, error) {
	if len(fn.Body.Statements) != 1 {
		return ast.RecordLiteralExpr{}, fmt.Errorf("function %s must contain a single return statement", fn.Name)
	}
	ret, ok := fn.Body.Statements[0].(ast.ReturnStmt)
	if !ok {
		return ast.RecordLiteralExpr{}, fmt.Errorf("function %s must contain a single return statement", fn.Name)
	}
	rec, ok := ret.Value.(ast.RecordLiteralExpr)
	if !ok || rec.TypeName != typeName {
		return ast.RecordLiteralExpr{}, fmt.Errorf("function %s must return %s literal", fn.Name, typeName)
	}
	return rec, nil
}

func resolveRegistryRoot(projectRoot string, registryPath string) string {
	if filepath.IsAbs(registryPath) {
		return filepath.Clean(registryPath)
	}
	return filepath.Clean(filepath.Join(projectRoot, registryPath))
}

func safePackageSourcePath(sourceRoot string, rel string) (string, error) {
	if strings.TrimSpace(rel) == "" {
		return "", fmt.Errorf("Path must not be empty")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("Path must be relative to Source")
	}
	cleanRel := filepath.Clean(rel)
	if cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("Path escapes Source")
	}
	root, err := filepath.Abs(sourceRoot)
	if err != nil {
		return "", err
	}
	candidate := filepath.Join(root, cleanRel)
	cleanCandidate := filepath.Clean(candidate)
	if cleanCandidate != root && !strings.HasPrefix(cleanCandidate, root+string(filepath.Separator)) {
		return "", fmt.Errorf("Path escapes Source")
	}
	return cleanCandidate, nil
}

func copyPackageSource(src string, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		name := d.Name()
		if d.IsDir() && (name == ".git" || rel == filepath.Join(".oct", "wrappers") || rel == filepath.Join(".oct", "packages")) {
			return filepath.SkipDir
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("package sync does not support symlink %s in PM2", path)
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
}

func validateCopiedManifest(root string, entry PackageEntry) error {
	metadata, err := LoadManifestMetadata(filepath.Join(root, manifestFileName))
	if err != nil {
		return err
	}
	if metadata.Name != entry.Name || metadata.Version != entry.Version || !registryKindMatchesManifest(entry.Kind, metadata.Kind) {
		return fmt.Errorf("copied manifest mismatch: requested %s %s kind %s; copied manifest name=%s version=%s kind=%s", entry.Name, entry.Version, entry.Kind, metadata.Name, metadata.Version, metadata.Kind)
	}
	return nil
}

func registryKindMatchesManifest(registryKind string, manifestKind string) bool {
	switch registryKind {
	case "library":
		return manifestKind == "" || manifestKind == "pure" || manifestKind == "library"
	case "experiment":
		return manifestKind == "experiment"
	case "wrapper":
		return manifestKind == "wrapper"
	default:
		return false
	}
}

func writePackageSourceMetadata(root string, resolved ResolvedPackage) error {
	path := filepath.Join(root, PackageSourceFileName)
	entry := resolved.Entry
	content := fmt.Sprintf(`package PackageSource

record PackageSource {
    Name: String
    Version: String
    Registry: String
    RegistryPath: String
    SourceKind: String
    Source: String
    Ref: String
    ResolvedCommit: String
    Path: String
}

fn PackageSource() -> PackageSource {
    return PackageSource {
        Name: %s
        Version: %s
        Registry: %s
        RegistryPath: %s
        SourceKind: %s
        Source: %s
        Ref: %s
        ResolvedCommit: %s
        Path: %s
    }
}
`, strconv.Quote(entry.Name), strconv.Quote(entry.Version), strconv.Quote(resolved.Registry.Name), strconv.Quote(resolved.Registry.Path), strconv.Quote(entry.SourceKind), strconv.Quote(entry.Source), strconv.Quote(entry.Ref), strconv.Quote(resolved.ResolvedCommit), strconv.Quote(entry.Path))
	return os.WriteFile(path, []byte(content), 0o644)
}
