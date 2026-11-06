package build

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"testing"
)

// This is the host source-emission boundary, not a second float semantics test.
// Reuse the exact existing specimen reported in FB-102.
func TestEmbeddableFloatSourceTypeChecksWithoutProcessEntrypoint(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "performance", "scalar_loop_recon", "main.oct")
	source, err := EmitGoSource(path, GoSourceOptions{PackageName: "floatconsumer"})
	if err != nil {
		t.Fatal(err)
	}
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, "generated.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range file.Decls {
		if function, ok := declaration.(*ast.FuncDecl); ok && function.Recv == nil && function.Name.Name == "main" {
			t.Fatal("embeddable emission introduced a process entrypoint")
		}
	}
	config := types.Config{Importer: importer.Default()}
	if _, err := config.Check("floatconsumer", files, []*ast.File{file}, nil); err != nil {
		t.Fatal(err)
	}
}
