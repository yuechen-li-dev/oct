package ast

import (
	"oct/internal/dimension"
	"oct/internal/source"
)

type File struct {
	Source    source.File
	Package   string
	Imports   []string
	Records   []RecordDecl
	Enums     []EnumDecl
	Functions []FunctionDecl
}

type RecordDecl struct {
	Name   string
	Fields []RecordField
}

type RecordField struct {
	Name string
	Type TypeRef
}

type EnumDecl struct {
	Name     string
	Variants []string
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
	Name      string
	Dimension dimension.Dimension
	HasUnit   bool
	IsArray   bool
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

type ExprStmt struct {
	Value Expr
}

func (ExprStmt) stmtNode() {}

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

type IfStmt struct {
	Condition Expr
	ThenBody  Block
	ElseBody  *Block
}

func (IfStmt) stmtNode() {}

type WhileStmt struct {
	Condition Expr
	Body      Block
}

func (WhileStmt) stmtNode() {}

type Expr interface {
	exprNode()
}

type IntegerLiteral struct {
	Value     string
	Dimension dimension.Dimension
	HasUnit   bool
}

func (IntegerLiteral) exprNode() {}

type FloatLiteral struct {
	Value     string
	Dimension dimension.Dimension
	HasUnit   bool
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

type FieldAccessExpr struct {
	Target Expr
	Field  string
}

func (FieldAccessExpr) exprNode() {}

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

type SwitchCase struct {
	Match Expr
	Value Expr
}

type SwitchExpr struct {
	Subject Expr
	Cases   []SwitchCase
	Else    Expr
}

func (SwitchExpr) exprNode() {}

type RecordLiteralExpr struct {
	TypeName string
	Fields   []RecordLiteralField
}

func (RecordLiteralExpr) exprNode() {}

type RecordLiteralField struct {
	Name  string
	Value Expr
}

type EnumValueExpr struct {
	EnumName string
	Variant  string
}

func (EnumValueExpr) exprNode() {}
