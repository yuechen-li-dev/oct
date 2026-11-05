package modules_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/modules"
	sdslvtest "github.com/yuechen-li-dev/oct/internal/sdslv/test"
	"github.com/yuechen-li-dev/oct/internal/sdslv/validate"
)

func fixtureRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "Language", "SDSL-V", "Modules"))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func copyFixtureTree(t *testing.T) string {
	t.Helper()
	root, dest := fixtureRoot(t), t.TempDir()
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		output := filepath.Join(dest, relative)
		if entry.IsDir() {
			return os.MkdirAll(output, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(output, data, 0644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dest
}

func TestGraphBindingRetainsProvenanceAndPrunesUnusedExports(t *testing.T) {
	path := filepath.Join(fixtureRoot(t), "TransitiveImports.sdslvtest")
	module, err := modules.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(module.Dependencies) != 4 || len(module.DependencyIdentity) != 64 {
		t.Fatalf("dependency metadata: %#v", module.Dependencies)
	}
	seen, doubles := map[string]bool{}, map[string]bool{}
	for _, decl := range module.Decls {
		name, span := ast.Declaration(decl)
		if span.Path == "" {
			t.Fatalf("missing owning source: %s", name)
		}
		if strings.HasSuffix(name, "_Unused") {
			t.Fatal("unreferenced export became a compiler root")
		}
		if strings.HasSuffix(name, "_Double") {
			doubles[name] = true
		}
		// Functions and types can share a name, but diamonds cannot duplicate a declaration.
		key := reflect.TypeOf(decl).String() + ":" + name
		if seen[key] {
			t.Fatalf("duplicate bound declaration %s", key)
		}
		seen[key] = true
	}
	if len(doubles) != 2 {
		t.Fatalf("private declaration ownership: %#v", doubles)
	}
	if _, err := sdslvtest.Prepare(path); err != nil {
		t.Fatal(err)
	}
}

func TestDependencyEditsInvalidateReplayIdentityWithoutRenamingEntry(t *testing.T) {
	root := copyFixtureTree(t)
	path := filepath.Join(root, "TransitiveImports.sdslvtest")
	before, err := sdslvtest.Discover(path)
	if err != nil {
		t.Fatal(err)
	}
	library := filepath.Join(root, "library", "Other.sdslv")
	file, err := os.OpenFile(library, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("\n// Changed transitive source snapshot.\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	after, err := sdslvtest.Discover(path)
	if err != nil {
		t.Fatal(err)
	}
	if before.DependencyIdentity == after.DependencyIdentity || before.Cases[0].StableID == after.Cases[0].StableID {
		t.Fatal("transitive edit reused stale replay identity")
	}
	if before.Cases[0].Function != after.Cases[0].Function {
		t.Fatal("dependency changed public entry name")
	}
	copy := copyFixtureTree(t)
	a, err := modules.Load(filepath.Join(copy, "TransitiveImports.sdslvtest"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := modules.Load(filepath.Join(fixtureRoot(t), "TransitiveImports.sdslvtest"))
	if err != nil {
		t.Fatal(err)
	}
	if a.DependencyIdentity != b.DependencyIdentity {
		t.Fatal("absolute checkout path entered dependency identity")
	}
}

func TestNamespaceResolutionUsesExplicitRoots(t *testing.T) {
	root := t.TempDir()
	library, err := os.ReadFile(filepath.Join(fixtureRoot(t), "library", "Shader", "Arithmetic.sdslv"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "Shader"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Shader", "Arithmetic.sdslv"), library, 0644); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(fixtureRoot(t), "library", "NamespaceImport.sdslvvalid")
	module, err := modules.LoadWithOptions(entry, modules.Options{SearchPaths: []string{root}})
	if err != nil {
		t.Fatal(err)
	}
	if len(module.Dependencies) != 2 {
		t.Fatalf("dependencies: %#v", module.Dependencies)
	}
}

func TestImportedDiagnosticUsesTheDeclaringFile(t *testing.T) {
	root := fixtureRoot(t)
	module, err := modules.Load(filepath.Join(root, "ImportedDiagnostic.sdslvinvalid"))
	if err != nil {
		t.Fatal(err)
	}
	diagnostics := validate.Diagnostics(module)
	if len(diagnostics) == 0 {
		t.Fatal("expected the registered invalid fixture's diagnostic")
	}
	want := filepath.Join(root, "library", "BadReturn.sdslv")
	if diagnostics[0].Path != want || diagnostics[0].Span.Path != want {
		t.Fatalf("diagnostic lost owning file: %#v", diagnostics[0])
	}
}

func TestExportedTypeDoesNotPullInItsPrivateNamesakeFunction(t *testing.T) {
	module, err := modules.Load(filepath.Join(fixtureRoot(t), "ImportedTypeOnly.sdslvvalid"))
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range module.Decls {
		if _, function := decl.(ast.FunctionDecl); function {
			t.Fatal("a type reference pulled in the unrelated private function namespace")
		}
	}
}
