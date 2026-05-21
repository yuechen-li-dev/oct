package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"oct/internal/ast"
	"oct/internal/lex"
	"oct/internal/parse"
	"oct/internal/pkgmgr"
	"oct/internal/source"
)

type Package struct {
	Name      string
	Directory string
	Imports   []string
	Records   []ast.RecordDecl
	Enums     []ast.EnumDecl
	Functions []ast.FunctionDecl
	Flows     []ast.FlowDecl
}

type Program struct {
	Root        string
	Entry       string
	EntrySource string
	Packages    map[string]Package
}

func Load(path string) (Program, error) {
	return load(path, false)
}

func LoadForTest(path string) (Program, error) {
	return load(path, true)
}

func LoadForTestWithSelectedFiles(path string, selectedFiles []string) (Program, error) {
	return loadWithSelectedFiles(path, true, selectedFiles)
}

func load(path string, includeTests bool) (Program, error) {
	return loadWithSelectedFiles(path, includeTests, nil)
}

func loadWithSelectedFiles(path string, includeTests bool, selectedFiles []string) (Program, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Program{}, fmt.Errorf("source file not found: %s", path)
		}
		return Program{}, fmt.Errorf("load source %s: %w", path, err)
	}

	if info.IsDir() {
		return loadFromDir(path, includeTests)
	}
	return loadFromFile(path, includeTests, selectedFiles)
}

func loadFromFile(path string, includeTests bool, explicitSelected []string) (Program, error) {
	packageDir := filepath.Dir(path)
	entryFile, err := parseFile(path)
	if err != nil {
		return Program{}, err
	}
	root := packageDir
	if filepath.Base(packageDir) == entryFile.Package {
		root = filepath.Dir(packageDir)
	}
	requireManifests, err := detectManifestedRoot(root)
	if err != nil {
		return Program{}, err
	}
	if includeTests && (filepath.Ext(path) == ".octest" || filepath.Ext(path) == ".oct") {
		requireManifests = false
	}
	builder := builder{
		root:             root,
		repoRoot:         detectRepoRoot(root),
		includeTests:     includeTests,
		requireManifests: requireManifests,
		packages:         make(map[string]Package),
		visiting:         make(map[string]struct{}),
		visited:          make(map[string]struct{}),
		manifestDeps:     make(map[string]map[string]struct{}),
	}
	if includeTests && (filepath.Ext(path) == ".octest" || filepath.Ext(path) == ".oct") {
		selected := map[string]struct{}{}
		if len(explicitSelected) == 0 {
			explicitSelected = []string{path}
		}
		for _, selectedPath := range explicitSelected {
			absEntry, absErr := filepath.Abs(selectedPath)
			if absErr != nil {
				return Program{}, absErr
			}
			selected[filepath.Clean(absEntry)] = struct{}{}
		}
		builder.selectedFiles = map[string]map[string]struct{}{
			entryFile.Package: selected,
		}
	}
	if err := builder.loadPackage(entryFile.Package, packageDir); err != nil {
		return Program{}, err
	}
	return Program{Root: root, Entry: entryFile.Package, EntrySource: path, Packages: builder.packages}, nil
}

func loadFromDir(root string, includeTests bool) (Program, error) {
	requireManifests, err := detectManifestedRoot(root)
	if err != nil {
		return Program{}, err
	}
	builder := builder{
		root:             root,
		repoRoot:         detectRepoRoot(root),
		includeTests:     includeTests,
		requireManifests: requireManifests,
		packages:         make(map[string]Package),
		visiting:         make(map[string]struct{}),
		visited:          make(map[string]struct{}),
		manifestDeps:     make(map[string]map[string]struct{}),
	}
	mainDir := filepath.Join(root, "Main")
	if _, err := os.Stat(mainDir); err == nil {
		if err := builder.loadPackage("Main", mainDir); err != nil {
			return Program{}, err
		}
		if includeTests {
			if err := builder.loadAllPackagesInRoot(); err != nil {
				return Program{}, err
			}
		}
		return Program{Root: root, Entry: "Main", EntrySource: mainDir, Packages: builder.packages}, nil
	}
	packageName, err := detectSinglePackageName(root, includeTests)
	if err != nil {
		return Program{}, err
	}
	if packageName == "" {
		packageName = "Main"
	}
	if err := builder.loadPackage(packageName, root); err != nil {
		return Program{}, err
	}
	if includeTests {
		if err := builder.loadAllPackagesInRoot(); err != nil {
			return Program{}, err
		}
	}
	return Program{Root: root, Entry: packageName, EntrySource: root, Packages: builder.packages}, nil
}

type builder struct {
	root             string
	repoRoot         string
	includeTests     bool
	requireManifests bool
	packages         map[string]Package
	visiting         map[string]struct{}
	visited          map[string]struct{}
	manifestDeps     map[string]map[string]struct{}
	cachedDeps       map[string]string
	selectedFiles    map[string]map[string]struct{}
}

func (b *builder) loadPackage(packageName string, directory string) error {
	if _, ok := b.visited[packageName]; ok {
		return nil
	}
	if _, ok := b.visiting[packageName]; ok {
		return fmt.Errorf("import cycle detected at package '%s'", packageName)
	}
	b.visiting[packageName] = struct{}{}
	defer delete(b.visiting, packageName)

	files, err := loadPackageFiles(directory, b.includeTests, b.selectedFiles[packageName])
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("unknown package '%s'", packageName)
	}
	manifestDeps, err := b.validateManifest(packageName, directory)
	if err != nil {
		return err
	}
	b.manifestDeps[packageName] = manifestDeps

	pkg := Package{Name: packageName, Directory: directory}
	importSet := make(map[string]struct{})
	declSet := make(map[string]struct{})
	for _, file := range files {
		if file.Package != packageName {
			return fmt.Errorf("inconsistent package names in directory '%s': expected '%s', got '%s'", directory, packageName, file.Package)
		}
		for _, imp := range file.Imports {
			if imp == packageName {
				return fmt.Errorf("package '%s' cannot import itself", packageName)
			}
			importSet[imp] = struct{}{}
		}
		for _, record := range file.Records {
			if _, exists := declSet[record.Name]; exists {
				return fmt.Errorf("duplicate declaration '%s' in package '%s'", record.Name, packageName)
			}
			declSet[record.Name] = struct{}{}
			pkg.Records = append(pkg.Records, record)
		}
		for _, enumDecl := range file.Enums {
			if _, exists := declSet[enumDecl.Name]; exists {
				return fmt.Errorf("duplicate declaration '%s' in package '%s'", enumDecl.Name, packageName)
			}
			declSet[enumDecl.Name] = struct{}{}
			pkg.Enums = append(pkg.Enums, enumDecl)
		}
		for _, fn := range file.Functions {
			if _, exists := declSet[fn.Name]; exists {
				return fmt.Errorf("duplicate declaration '%s' in package '%s'", fn.Name, packageName)
			}
			declSet[fn.Name] = struct{}{}
			pkg.Functions = append(pkg.Functions, fn)
		}
		for _, flow := range file.Flows {
			if _, exists := declSet[flow.Name]; exists {
				return fmt.Errorf("duplicate declaration '%s' in package '%s'", flow.Name, packageName)
			}
			declSet[flow.Name] = struct{}{}
			pkg.Flows = append(pkg.Flows, flow)
		}
	}

	for imp := range importSet {
		pkg.Imports = append(pkg.Imports, imp)
	}
	sort.Strings(pkg.Imports)
	b.packages[packageName] = pkg

	for _, imp := range pkg.Imports {
		importDir, err := b.resolveImportDirectory(packageName, imp)
		if err != nil {
			return err
		}
		if err := b.loadPackage(imp, importDir); err != nil {
			return err
		}
	}

	b.visited[packageName] = struct{}{}
	return nil
}

func (b *builder) loadAllPackagesInRoot() error {
	entries, err := os.ReadDir(b.root)
	if err != nil {
		return fmt.Errorf("read package root %s: %w", b.root, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if isMilestoneDir(entry.Name()) {
			continue
		}
		dir := filepath.Join(b.root, entry.Name())
		files, err := loadPackageFiles(dir, b.includeTests, b.selectedFiles[entry.Name()])
		if err != nil {
			return err
		}
		if len(files) == 0 {
			continue
		}
		if err := b.loadPackage(entry.Name(), dir); err != nil {
			return err
		}
	}
	return nil
}

func loadPackageFiles(directory string, includeTests bool, selected map[string]struct{}) ([]ast.File, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read package directory %s: %w", directory, err)
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			if !isMilestoneDir(entry.Name()) {
				continue
			}
			milestoneDir := filepath.Join(directory, entry.Name())
			milestoneEntries, err := os.ReadDir(milestoneDir)
			if err != nil {
				return nil, fmt.Errorf("read milestone directory %s: %w", milestoneDir, err)
			}
			for _, milestoneEntry := range milestoneEntries {
				if milestoneEntry.IsDir() || milestoneEntry.Name() == "manifest.oct" {
					continue
				}
				ext := filepath.Ext(milestoneEntry.Name())
				if ext != ".oct" && (!includeTests || ext != ".octest") {
					continue
				}
				files = append(files, filepath.Join(milestoneDir, milestoneEntry.Name()))
			}
			continue
		}
		if entry.Name() == "manifest.oct" {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".oct" && (!includeTests || ext != ".octest") {
			continue
		}
		files = append(files, filepath.Join(directory, entry.Name()))
	}
	sort.Strings(files)
	if len(selected) > 0 {
		filtered := make([]string, 0, len(files))
		for _, candidate := range files {
			if filepath.Ext(candidate) != ".octest" {
				filtered = append(filtered, candidate)
				continue
			}
			absCandidate, err := filepath.Abs(candidate)
			if err != nil {
				return nil, err
			}
			if _, ok := selected[filepath.Clean(absCandidate)]; ok {
				filtered = append(filtered, candidate)
			}
		}
		files = filtered
	}
	result := make([]ast.File, 0, len(files))
	for _, path := range files {
		parsed, err := parseFile(path)
		if err != nil {
			return nil, err
		}
		result = append(result, parsed)
	}
	return result, nil
}

func isMilestoneDir(name string) bool {
	if len(name) < 2 || name[0] != 'M' {
		return false
	}
	offset := 1
	if len(name) >= 3 && name[1] == 'x' {
		offset = 2
	}
	if offset >= len(name) {
		return false
	}
	idx := offset
	for idx < len(name) && name[idx] >= '0' && name[idx] <= '9' {
		idx++
	}
	if idx == offset {
		return false
	}
	if idx == len(name) {
		return true
	}
	return idx+1 == len(name) && name[idx] >= 'a' && name[idx] <= 'z'
}

func (b *builder) validateManifest(packageName string, directory string) (map[string]struct{}, error) {
	if !b.requireManifests {
		return nil, nil
	}
	manifestPath := filepath.Join(directory, "manifest.oct")
	if _, err := os.Stat(manifestPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("package manifest missing")
		}
		return nil, fmt.Errorf("read package manifest %s: %w", manifestPath, err)
	}
	manifestFile, err := parseFile(manifestPath)
	if err != nil {
		return nil, err
	}
	if err := validateManifestFile(packageName, manifestFile); err != nil {
		return nil, err
	}
	return manifestDependencySet(manifestFile), nil
}

func (b *builder) resolveImportDirectory(packageName string, importName string) (string, error) {
	searched := make([]string, 0, 4)
	for _, searchRoot := range b.importSearchRoots() {
		candidateDir := filepath.Join(searchRoot, importName)
		searched = append(searched, candidateDir)
		files, err := loadPackageFiles(candidateDir, b.includeTests, b.selectedFiles[importName])
		if err != nil {
			return "", err
		}
		if len(files) > 0 {
			return candidateDir, nil
		}
	}
	if !b.requireManifests {
		return "", fmt.Errorf("unknown package '%s' imported by package '%s' (active root: %s; searched: %s)",
			importName,
			packageName,
			b.root,
			strings.Join(searched, ", "),
		)
	}
	deps := b.manifestDeps[packageName]
	if deps == nil {
		return "", fmt.Errorf("unknown package '%s' imported by package '%s' (active root: %s; searched: %s; manifest mode: enabled; dependency declaration: missing)",
			importName,
			packageName,
			b.root,
			strings.Join(searched, ", "),
		)
	}
	if _, declared := deps[importName]; !declared {
		return "", fmt.Errorf("unknown package '%s' imported by package '%s' (active root: %s; searched: %s; manifest mode: enabled; dependency declaration: missing)",
			importName,
			packageName,
			b.root,
			strings.Join(searched, ", "),
		)
	}
	cachedDir, ok, err := b.resolveCachedDependencyDir(importName)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("unknown package '%s' imported by package '%s' (active root: %s; searched: %s; manifest mode: enabled; dependency declaration: present; package cache: miss)",
			importName,
			packageName,
			b.root,
			strings.Join(searched, ", "),
		)
	}
	return cachedDir, nil
}

func (b *builder) importSearchRoots() []string {
	roots := []string{b.root}
	if experimentFamilyRoot, ok := containingExperimentFamilyRoot(b.root); ok {
		roots = append(roots, experimentFamilyRoot)
	}
	if b.repoRoot == "" {
		return roots
	}
	for _, name := range []string{"Libraries", "Packages"} {
		root := filepath.Join(b.repoRoot, name)
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		roots = append(roots, root)
	}
	return dedupePaths(roots)
}

func containingExperimentFamilyRoot(root string) (string, bool) {
	clean := filepath.Clean(root)
	base := filepath.Base(clean)
	if !isMilestoneDir(base) && base != "Shared" {
		return "", false
	}
	parent := filepath.Dir(clean)
	if parent == "." || parent == clean {
		return "", false
	}
	if !hasFileAt(parent, "manifest.oct") || !hasFileAt(parent, "REPORT.md") {
		return "", false
	}
	return parent, true
}

func hasFileAt(root string, name string) bool {
	info, err := os.Stat(filepath.Join(root, name))
	return err == nil && !info.IsDir()
}

func (b *builder) resolveCachedDependencyDir(importName string) (string, bool, error) {
	if b.cachedDeps == nil {
		cachedDeps, err := loadCachedDependencyPaths()
		if err != nil {
			return "", false, err
		}
		b.cachedDeps = cachedDeps
	}
	dir, ok := b.cachedDeps[importName]
	return dir, ok, nil
}

func loadCachedDependencyPaths() (map[string]string, error) {
	manager, err := pkgmgr.NewManager()
	if err != nil {
		return nil, fmt.Errorf("resolve package cache: %w", err)
	}
	entries, err := manager.List()
	if err != nil {
		return nil, fmt.Errorf("read package cache index: %w", err)
	}
	byName := make(map[string]string, len(entries))
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			continue
		}
		if _, exists := byName[name]; exists {
			continue
		}
		byName[name] = entry.Path
	}
	return byName, nil
}

func detectManifestedRoot(root string) (bool, error) {
	if _, err := os.Stat(filepath.Join(root, "manifest.oct")); err == nil {
		return true, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return false, fmt.Errorf("read package root %s: %w", root, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(root, entry.Name(), "manifest.oct")
		if _, err := os.Stat(path); err == nil {
			return true, nil
		}
	}
	return false, nil
}

func detectSinglePackageName(root string, includeTests bool) (string, error) {
	files, err := loadPackageFiles(root, includeTests, nil)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", nil
	}
	packageName := files[0].Package
	for _, file := range files[1:] {
		if file.Package != packageName {
			return "", fmt.Errorf("inconsistent package names in directory '%s': expected '%s', got '%s'", root, packageName, file.Package)
		}
	}
	return packageName, nil
}

func detectRepoRoot(start string) string {
	current := start
	for {
		if hasRepoImportRoots(current) {
			return current
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

func hasRepoImportRoots(root string) bool {
	for _, name := range []string{"Libraries", "Packages"} {
		path := filepath.Join(root, name)
		info, err := os.Stat(path)
		if err == nil && info.IsDir() {
			return true
		}
	}
	return false
}

func dedupePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		clean := filepath.Clean(path)
		if _, exists := seen[clean]; exists {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	return out
}

func parseFile(path string) (ast.File, error) {
	file, err := source.Load(path)
	if err != nil {
		return ast.File{}, err
	}
	lexed, err := lex.Analyze(file)
	if err != nil {
		return ast.File{}, err
	}
	parsed, err := parse.BuildFile(lexed)
	if err != nil {
		if strings.Contains(err.Error(), "missing package declaration") {
			return ast.File{}, fmt.Errorf("missing package declaration")
		}
		return ast.File{}, err
	}
	return parsed, nil
}
