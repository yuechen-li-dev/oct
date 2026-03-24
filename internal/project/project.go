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
}

type Program struct {
	Root        string
	Entry       string
	EntrySource string
	Packages    map[string]Package
}

func Load(path string) (Program, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Program{}, fmt.Errorf("source file not found: %s", path)
		}
		return Program{}, fmt.Errorf("load source %s: %w", path, err)
	}

	if info.IsDir() {
		return loadFromDir(path)
	}
	return loadFromFile(path)
}

func loadFromFile(path string) (Program, error) {
	packageDir := filepath.Dir(path)
	entryFile, err := parseFile(path)
	if err != nil {
		return Program{}, err
	}
	root := packageDir
	if filepath.Base(packageDir) == entryFile.Package {
		root = filepath.Dir(packageDir)
	}
	builder := builder{root: root, packages: make(map[string]Package), visiting: make(map[string]struct{}), visited: make(map[string]struct{})}
	if err := builder.loadPackage(entryFile.Package, packageDir); err != nil {
		return Program{}, err
	}
	return Program{Root: root, Entry: entryFile.Package, EntrySource: path, Packages: builder.packages}, nil
}

func loadFromDir(root string) (Program, error) {
	builder := builder{root: root, packages: make(map[string]Package), visiting: make(map[string]struct{}), visited: make(map[string]struct{})}
	mainDir := filepath.Join(root, "Main")
	if _, err := os.Stat(mainDir); err == nil {
		if err := builder.loadPackage("Main", mainDir); err != nil {
			return Program{}, err
		}
		return Program{Root: root, Entry: "Main", EntrySource: mainDir, Packages: builder.packages}, nil
	}
	if err := builder.loadPackage("Main", root); err != nil {
		return Program{}, err
	}
	return Program{Root: root, Entry: "Main", EntrySource: root, Packages: builder.packages}, nil
}

type builder struct {
	root     string
	packages map[string]Package
	visiting map[string]struct{}
	visited  map[string]struct{}
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

	files, err := loadPackageFiles(directory)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("unknown package '%s'", packageName)
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

func loadPackageFiles(directory string) ([]ast.File, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read package directory %s: %w", directory, err)
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".oct" {
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
