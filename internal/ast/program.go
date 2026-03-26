package ast

import (
	"oct/internal/dimension"
	"oct/internal/source"
)

type File struct {
	Source    source.File
	IsTest    bool
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
	SourcePath string
	IsTestFile bool
	IsFact     bool
	IsTheory   bool
	IsArtifact bool
	InlineData []InlineDataRow
	Parameters []Parameter
	ReturnType TypeRef
	IsFallible bool
	ErrorType  TypeRef
	Body       Block
}

type InlineDataRow struct {
	Values []Expr
}

type Parameter struct {
	Name string
	Type TypeRef
}

type TypeRef struct {
	Package   string
	Name      string
	Dimension dimension.Dimension
	HasUnit   bool
	IsArray   bool
	VectorOf  *TypeRef
	MatrixOf  *TypeRef
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

type VarStmt struct {
	Name  string
	Value Expr
}

func (VarStmt) stmtNode() {}

type AssignStmt struct {
	Name  string
	Value Expr
}

func (AssignStmt) stmtNode() {}

type IndexAssignStmt struct {
	Target string
	Index  Expr
	Value  Expr
}

func (IndexAssignStmt) stmtNode() {}

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

type VectorLiteralExpr struct {
	Elements []Expr
}

func (VectorLiteralExpr) exprNode() {}

type MatrixLiteralExpr struct {
	Rows [][]Expr
}

func (MatrixLiteralExpr) exprNode() {}

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
	Target  Expr
	Indices []Expr
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

type UnaryExpr struct {
	Operator string
	Operand  Expr
}

func (UnaryExpr) exprNode() {}

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

type IfExpr struct {
	Condition Expr
	ThenExpr  Expr
	ElseExpr  Expr
}

func (IfExpr) exprNode() {}

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
