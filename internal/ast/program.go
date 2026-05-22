package ast

import (
	"github.com/yuechen-li-dev/oct/internal/dimension"
	"github.com/yuechen-li-dev/oct/internal/source"
)

type File struct {
	Source    source.File
	IsTest    bool
	Package   string
	Imports   []string
	Records   []RecordDecl
	Enums     []EnumDecl
	Functions []FunctionDecl
	Flows     []FlowDecl
}

type DocComment struct {
	Lines      []string
	Structured []DocSection
}

type DocSection struct {
	Keyword string
	Target  string
	Text    string
}

type RecordDecl struct {
	Name   string
	Doc    *DocComment
	Fields []RecordField
}

type RecordField struct {
	Name string
	Type TypeRef
	Doc  *DocComment
}

type EnumDecl struct {
	Name     string
	Doc      *DocComment
	Variants []EnumVariantDecl
}

type EnumVariantDecl struct {
	Name    string
	Payload *TypeRef
}

type FunctionDecl struct {
	Name        string
	Doc         *DocComment
	SourcePath  string
	IsTestFile  bool
	IsFact      bool
	IsTheory    bool
	IsArtifact  bool
	IsBenchmark bool
	InlineData  []InlineDataRow
	Suites      []string
	CycleTime   Expr
	Parameters  []Parameter
	ReturnType  TypeRef
	IsFallible  bool
	ErrorType   TypeRef
	Body        Block
}

type FlowDecl struct {
	Name       string
	Parameters []Parameter
	ReturnType TypeRef
	Board      []BoardField
	States     []StateDecl
	EntryState string
}

type BoardField struct {
	Name string
	Type TypeRef
}

type StateDecl struct {
	Name string
	Body Block
}

type InlineDataRow struct {
	Values []Expr
}

type Parameter struct {
	Name string
	Type TypeRef
}

type TypeRef struct {
	Package    string
	Name       string
	TupleOf    []TypeRef
	Dimension  dimension.Dimension
	HasUnit    bool
	IsArray    bool
	ArrayDepth int
	VectorOf   *TypeRef
	MatrixOf   *TypeRef
	Function   *FunctionTypeRef
}

type FunctionTypeRef struct {
	Parameters []TypeRef
	ReturnType TypeRef
	IsFallible bool
	ErrorType  *TypeRef
}

type Block struct {
	Statements []Stmt
}

type Stmt interface {
	stmtNode()
}

type LetStmt struct {
	Name     string
	TypeHint *TypeRef
	Value    Expr
}

func (LetStmt) stmtNode() {}

type VarStmt struct {
	Name     string
	TypeHint *TypeRef
	Value    Expr
}

func (VarStmt) stmtNode() {}

type AssignStmt struct {
	Name  string
	Value Expr
}

func (AssignStmt) stmtNode() {}

type DestructureAssignStmt struct {
	Names []string
	Value Expr
}

func (DestructureAssignStmt) stmtNode() {}

type IndexAssignStmt struct {
	Target  string
	Indices []Expr
	Value   Expr
}

func (IndexAssignStmt) stmtNode() {}

type FieldAssignStmt struct {
	Target string
	Field  string
	Value  Expr
}

func (FieldAssignStmt) stmtNode() {}

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

type PrometheusStmt struct {
	Body Block
}

func (PrometheusStmt) stmtNode() {}

type GotoStmt struct {
	Target string
}

func (GotoStmt) stmtNode() {}

type SuspendStmt struct{}

func (SuspendStmt) stmtNode() {}

type RememberStmt struct{}

func (RememberStmt) stmtNode() {}

type ResumeStmt struct{}

func (ResumeStmt) stmtNode() {}

type WhenStmt struct {
	Cases []WhenCase
	Else  WhenAction
}

func (WhenStmt) stmtNode() {}

type WhenCase struct {
	Condition Expr
	Action    WhenAction
}

type WhenAction interface {
	whenActionNode()
}

type WhenGotoAction struct {
	Target string
}

func (WhenGotoAction) whenActionNode() {}

type WhenSuspendAction struct{}

func (WhenSuspendAction) whenActionNode() {}

type WhenReturnAction struct {
	Value Expr
}

func (WhenReturnAction) whenActionNode() {}

type WhenBlockAction struct {
	Statements []Stmt
}

func (WhenBlockAction) whenActionNode() {}

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
	Callee        Expr
	TypeArguments []TypeRef
	Arguments     []Expr
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

type MatchCase struct {
	Variant string
	Binding string
	Value   Expr
}

type MatchExpr struct {
	Subject Expr
	Cases   []MatchCase
}

func (MatchExpr) exprNode() {}

type IfExpr struct {
	Condition Expr
	ThenExpr  Expr
	ElseExpr  Expr
}

func (IfExpr) exprNode() {}

type UtilityWhenPolicy struct {
	Hysteresis Expr
	MinCommit  Expr
}

type UtilityWhenCase struct {
	Value     Expr
	Condition Expr
	Score     Expr
}

type UtilityWhenExpr struct {
	SiteID          int
	Policy          UtilityWhenPolicy
	Cases           []UtilityWhenCase
	Else            Expr
	ControllerBound bool
}

func (UtilityWhenExpr) exprNode() {}

type BatchExpr struct {
	Input    Expr
	ItemName string
	Body     Block
}

func (BatchExpr) exprNode() {}

type RecordLiteralExpr struct {
	TypeName string
	Fields   []RecordLiteralField
}

func (RecordLiteralExpr) exprNode() {}

type RecordLiteralField struct {
	Name  string
	Value Expr
}

type RecordUpdateExpr struct {
	Source Expr
	Fields []RecordLiteralField
}

func (RecordUpdateExpr) exprNode() {}

type EnumValueExpr struct {
	EnumName string
	Variant  string
}

func (EnumValueExpr) exprNode() {}
