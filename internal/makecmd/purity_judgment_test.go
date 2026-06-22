package makecmd

import (
	"testing"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

func TestMakePurityJudgmentRanksHostAuthorityOverUnknownCall(t *testing.T) {
	fn := ast.FunctionDecl{
		Name:       "Mixed",
		IsMakePure: true,
		Body: ast.Block{Statements: []ast.Stmt{
			ast.LetStmt{Name: "path", Value: ast.CallExpr{Callee: ast.IdentifierExpr{Name: "NormalizePath"}, Arguments: []ast.Expr{ast.StringLiteralExpr{Value: "input.txt"}}}},
			ast.ReturnStmt{Value: ast.PropagateExpr{Inner: ast.CallExpr{Callee: ast.FieldAccessExpr{Target: ast.IdentifierExpr{Name: "Make"}, Field: "ReadText"}, Arguments: []ast.Expr{ast.IdentifierExpr{Name: "path"}}}}},
		}},
	}
	decision := decideMakePurity(fn, map[string]bool{"Mixed": true, "NormalizePath": true}, map[string]bool{"Mixed": true})
	if decision.OK {
		t.Fatal("expected concerning purity evidence")
	}
	if decision.Primary.Kind != makePurityHostAuthority {
		t.Fatalf("primary kind=%s want %s", decision.Primary.Kind, makePurityHostAuthority)
	}
	if decision.Trace.Winner == "" {
		t.Fatal("expected internal/judgment winner trace")
	}
}
