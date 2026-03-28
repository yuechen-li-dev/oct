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

func load(path string, includeTests bool) (Program, error) {
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
	return loadFromFile(path, includeTests)
}

func loadFromFile(path string, includeTests bool) (Program, error) {
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
	builder := builder{root: root, includeTests: includeTests, requireManifests: requireManifests, packages: make(map[string]Package), visiting: make(map[string]struct{}), visited: make(map[string]struct{})}
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
	builder := builder{root: root, includeTests: includeTests, requireManifests: requireManifests, packages: make(map[string]Package), visiting: make(map[string]struct{}), visited: make(map[string]struct{})}
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
	includeTests     bool
	requireManifests bool
	packages         map[string]Package
	visiting         map[string]struct{}
	visited          map[string]struct{}
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

	files, err := loadPackageFiles(directory, b.includeTests)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("unknown package '%s'", packageName)
	}
	if err := b.validateManifest(packageName, directory); err != nil {
		return err
	}

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
		importDir := filepath.Join(b.root, imp)
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
		files, err := loadPackageFiles(dir, b.includeTests)
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

func loadPackageFiles(directory string, includeTests bool) ([]ast.File, error) {
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
	return (name[1] >= '0' && name[1] <= '9') || name[1] == 'x'
}

func (b *builder) validateManifest(packageName string, directory string) error {
	if !b.requireManifests {
		return nil
	}
	manifestPath := filepath.Join(directory, "manifest.oct")
	if _, err := os.Stat(manifestPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("package manifest missing")
		}
		return fmt.Errorf("read package manifest %s: %w", manifestPath, err)
	}
	manifestFile, err := parseFile(manifestPath)
	if err != nil {
		return err
	}
	if err := validateManifestFile(packageName, manifestFile); err != nil {
		return err
	}
	return nil
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
	files, err := loadPackageFiles(root, includeTests)
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
