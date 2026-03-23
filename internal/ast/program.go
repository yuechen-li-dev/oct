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
	IsFallible bool
	ErrorType  TypeRef
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

type ForStmt struct {
	Name  string
	Range Expr
	Body  Block
}

func (ForStmt) stmtNode() {}

type MatchStmt struct {
	Subject Expr
	OkName  string
	OkBody  Block
	ErrName string
	ErrBody Block
}

func (MatchStmt) stmtNode() {}

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

type StringLiteralExpr struct {
	Value string
}

func (StringLiteralExpr) exprNode() {}

type ArrayLiteralExpr struct {
	Elements []Expr
}

func (ArrayLiteralExpr) exprNode() {}

type IdentifierExpr struct {
	Name string
}

func (IdentifierExpr) exprNode() {}

type CallExpr struct {
	Callee    string
	Arguments []Expr
}

func (CallExpr) exprNode() {}

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

type RangeExpr struct {
	Start Expr
	End   Expr
	Step  Expr
}

func (RangeExpr) exprNode() {}

type ParenExpr struct {
	Inner Expr
}

func (ParenExpr) exprNode() {}

type PropagateExpr struct {
	Inner Expr
}

func (PropagateExpr) exprNode() {}

type UnwrapExpr struct {
	Inner Expr
}

func (UnwrapExpr) exprNode() {}
