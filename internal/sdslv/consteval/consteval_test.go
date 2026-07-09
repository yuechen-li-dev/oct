package consteval

import (
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/sdslv/lex"
	"github.com/yuechen-li-dev/oct/internal/sdslv/parse"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func TestEvalSupportsArithmeticComparisonAndBoolean(t *testing.T) {
	value := evalExpr(t, "!(1u + 2u * 3u == 7u) || (10u % 4u == 2u)")
	if value.Type.Name != "bool" || !value.Bool {
		t.Fatalf("value = %#v, want true bool", value)
	}
}

func TestEvalSupportsConfigFieldReferences(t *testing.T) {
	value, err := Eval(parseExpr(t, "C.TILE_M * TILE_K <= 256u"), map[string]Value{
		"C.TILE_M": {Type: ast.TypeRef{Name: "u32"}, Int32: 16, IsKnown: true},
		"TILE_K":   {Type: ast.TypeRef{Name: "u32"}, Int32: 16, IsKnown: true},
	})
	if err != nil {
		t.Fatalf("Eval() error = %v", err)
	}
	if value.Type.Name != "bool" || !value.Bool {
		t.Fatalf("value = %#v, want true bool", value)
	}
}

func TestEvalSupportsNestedConfigFieldReferences(t *testing.T) {
	value, err := Eval(parseExpr(t, "C.Threads.X * Tile.K <= 256u"), map[string]Value{
		"C.Threads.X": {Type: ast.TypeRef{Name: "u32"}, Int32: 16, IsKnown: true},
		"Tile.K":      {Type: ast.TypeRef{Name: "u32"}, Int32: 16, IsKnown: true},
	})
	if err != nil {
		t.Fatalf("Eval() error = %v", err)
	}
	if value.Type.Name != "bool" || !value.Bool {
		t.Fatalf("value = %#v, want true bool", value)
	}
}

func TestEvalRejectsTypeMismatch(t *testing.T) {
	_, err := Eval(parseExpr(t, "1u && true"), nil)
	if err == nil || !strings.Contains(err.Error(), "bool operands") {
		t.Fatalf("error = %v, want bool operand diagnostic", err)
	}
}

func evalExpr(t *testing.T, expr string) Value {
	t.Helper()
	value, err := Eval(parseExpr(t, expr), nil)
	if err != nil {
		t.Fatalf("Eval() error = %v", err)
	}
	return value
}

func parseExpr(t *testing.T, expr string) ast.Expr {
	t.Helper()
	text := "fn F() -> void { let x: bool = " + expr + "; return; }"
	tokens, err := lex.Analyze(source.File{Path: "test.sdslv", Text: text})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	module, err := parse.BuildModule(tokens)
	if err != nil {
		t.Fatalf("BuildModule() error = %v", err)
	}
	fn := module.Decls[0].(ast.FunctionDecl)
	letStmt := fn.Body.Statements[0].(ast.LetStmt)
	return letStmt.Value
}
