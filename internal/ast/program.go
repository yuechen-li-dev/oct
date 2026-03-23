package ast

import "oct/internal/source"

type File struct {
	Source    source.File
	Functions []FunctionDecl
}

type FunctionDecl struct {
	Name       string
	Parameters []Parameter
	ReturnType TypeRef
	Body       Block
}

type Parameter struct {
	Name string
	Type TypeRef
}

type TypeRef struct {
	Name    string
	IsArray bool
}

type Block struct {
	Statements []Stmt
}

type Stmt interface {
	stmtNode()
}

type LetStmt struct {
	Name  string
	Value Expr
}

func (LetStmt) stmtNode() {}

type ReturnStmt struct {
	Value Expr
}

func (ReturnStmt) stmtNode() {}

type Expr interface {
	exprNode()
}

type IntegerLiteral struct {
	Value string
}

func (IntegerLiteral) exprNode() {}

type FloatLiteral struct {
	Value string
}

func (FloatLiteral) exprNode() {}

type BoolLiteral struct {
	Value bool
}

func (BoolLiteral) exprNode() {}

type ArrayLiteralExpr struct {
	Elements []Expr
}

func (ArrayLiteralExpr) exprNode() {}

type IdentifierExpr struct {
	Name string
}

func (IdentifierExpr) exprNode() {}

type IndexExpr struct {
	Target Expr
	Index  Expr
}

func (IndexExpr) exprNode() {}

type BinaryExpr struct {
	Left     Expr
	Operator string
	Right    Expr
}

func (BinaryExpr) exprNode() {}

type ParenExpr struct {
	Inner Expr
}

func (ParenExpr) exprNode() {}
