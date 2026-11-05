package build

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"reflect"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

// These are backend boundary checks. Oct behavior is exercised by the loop
// corpus; this test verifies the Go compiler receives real branch edges.
func TestScalarLoopEmitsDirectGoBackedge(t *testing.T) {
	program, err := project.Load("../../testdata/performance/scalar_loop_recon/main.oct")
	if err != nil {
		t.Fatal(err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatal(err)
	}
	module, err := lowerProgram(program, compileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	source, err := emitGo(module)
	if err != nil {
		t.Fatal(err)
	}
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, "generated.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	config := types.Config{Importer: importer.Default()}
	if _, err := config.Check("loop", files, []*ast.File{file}, nil); err != nil {
		t.Fatal(err)
	}
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "fn_Main_main" {
			continue
		}
		labels := map[string]token.Pos{}
		var jumps []*ast.BranchStmt
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.SwitchStmt:
				t.Error("scalar loop still contains a dispatcher")
			case *ast.LabeledStmt:
				labels[node.Label.Name] = node.Pos()
			case *ast.BranchStmt:
				if node.Tok == token.GOTO {
					jumps = append(jumps, node)
				}
			}
			return true
		})
		for _, jump := range jumps {
			if pos, ok := labels[jump.Label.Name]; ok && pos < jump.Pos() {
				return
			}
		}
		t.Fatal("scalar loop has no direct Go backedge")
	}
	t.Fatal("missing generated scalar function")
}

func TestDirectGoBlocksValidateAndPreserveMIR(t *testing.T) {
	tests := []struct {
		name   string
		blocks []MIRBlock
		error  string
	}{
		{name: "empty", error: "no MIR blocks"},
		{name: "duplicate", blocks: []MIRBlock{
			{Label: "entry", Terminator: MIRReturn{}},
			{Label: "entry", Terminator: MIRReturn{}},
		}, error: "duplicate MIR block"},
		{name: "missing target", blocks: []MIRBlock{
			{Label: "entry", Terminator: MIRJump{Target: "missing"}},
		}, error: "missing target missing"},
		{name: "missing false target", blocks: []MIRBlock{
			{Label: "entry", Terminator: MIRBranch{Cond: mirBool(true), TrueTarget: "entry", FalseTarget: "missing"}},
		}, error: "missing target missing"},
		{name: "unterminated final block", blocks: []MIRBlock{{Label: "entry"}}, error: "no terminator"},
		{name: "fallthrough and unreachable block", blocks: []MIRBlock{
			{Label: "entry"},
			{Label: "return", Terminator: MIRReturn{}},
			{Label: "unreachable", Terminator: MIRReturn{}},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := append([]MIRBlock(nil), tt.blocks...)
			source, err := emitGoWithOptions(MIRModule{Functions: []MIRFunction{{
				Package: "Main", Name: "Main", Return: "Void", Blocks: tt.blocks,
			}}}, goEmitOptions{packageName: "generated"})
			if !reflect.DeepEqual(before, tt.blocks) {
				t.Fatal("Go emission mutated the input MIR")
			}
			if tt.error != "" {
				if err == nil || !strings.Contains(err.Error(), tt.error) {
					t.Fatalf("error = %v, want %q", err, tt.error)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			files := token.NewFileSet()
			file, err := parser.ParseFile(files, "generated.go", source, 0)
			if err != nil {
				t.Fatal(err)
			}
			config := types.Config{Importer: importer.Default()}
			if _, err := config.Check("generated", files, []*ast.File{file}, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}
