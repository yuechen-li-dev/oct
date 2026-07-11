package ast

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"
)

// Every concrete struct in ast.go is explicitly classified. A means parsed
// source syntax and requires Span storage; B is a deliberately synthetic or
// recovery node. Interfaces are intentionally absent because they store no
// source data. Keeping this table next to the AST makes additions fail loudly.
var astSpanInventory = map[string]rune{
	"Module": 'A', "TemplateParam": 'A', "Attribute": 'A', "TypeAliasDecl": 'A', "RecordDecl": 'A', "BoardDecl": 'A', "StreamDecl": 'A', "ConceptDecl": 'A', "ConceptField": 'A', "ConceptGroup": 'A', "ConfigField": 'A', "ConfigDecl": 'A', "EnumDecl": 'A', "EnumVariant": 'A', "ShaderDecl": 'A', "CompileDecl": 'A', "FunctionDecl": 'A', "UnsupportedDecl": 'B', "NumThreads": 'A', "ResourceDecl": 'A', "WorkgroupDecl": 'A', "Field": 'A', "Parameter": 'A', "TypeRef": 'A', "Block": 'A', "LetStmt": 'A', "ComptimeLetStmt": 'A', "AssignStmt": 'A', "GuardedWriteStmt": 'A', "ReturnStmt": 'A', "ExprStmt": 'A', "ForeignShaderStmt": 'A', "IfStmt": 'A', "GuardWhenStmt": 'A', "GuardWhenCase": 'A', "FlowStmt": 'A', "FlowBoardDecl": 'A', "StateBlock": 'A', "ComptimeIfStmt": 'A', "ComptimeMatchStmt": 'A', "ComptimeMatchArm": 'A', "ComptimeWhenUtilityStmt": 'A', "ComptimeWhenUtilityCase": 'A', "ComptimeForStmt": 'A', "ForStmt": 'A', "RequireStmt": 'A', "StaticAssertStmt": 'A', "IntegerLiteral": 'A', "FloatLiteral": 'A', "BoolLiteral": 'A', "StringLiteral": 'A', "IdentifierExpr": 'A', "ForeignShaderExpr": 'A', "FieldAccessExpr": 'A', "IndexExpr": 'A', "GuardedReadExpr": 'A', "CallExpr": 'A', "BinaryExpr": 'A', "UnaryExpr": 'A', "ParenExpr": 'A', "WhenUtilityExpr": 'A', "UtilityCase": 'A', "WithExpr": 'A', "DeriveExpr": 'A', "DeriveField": 'A', "ReductionExpr": 'A', "FieldUpdate": 'A', "EnumConstructExpr": 'A', "BoardLiteralExpr": 'A', "FieldInit": 'A', "MatchExpr": 'A', "MatchArm": 'A',
}

func TestSdslvAstInventoryClassifiesEveryConcreteNode(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(filepath.Dir(file), "ast.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, s := range gd.Specs {
			ts, ok := s.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			seen[ts.Name.Name] = true
			category, ok := astSpanInventory[ts.Name.Name]
			if !ok {
				t.Errorf("%s lacks span classification", ts.Name.Name)
				continue
			}
			if category == 'A' && !hasSpan(st) {
				t.Errorf("source node %s lacks Span storage", ts.Name.Name)
			}
		}
	}
	for name := range astSpanInventory {
		if !seen[name] {
			t.Errorf("inventory names non-existent node %s", name)
		}
	}
}

func hasSpan(st *ast.StructType) bool {
	for _, f := range st.Fields.List {
		for _, n := range f.Names {
			if n.Name == "Span" {
				return true
			}
		}
	}
	return false
}
