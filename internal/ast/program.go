package ast

import (
	"github.com/yuechen-li-dev/oct/internal/dimension"
	"github.com/yuechen-li-dev/oct/internal/source"
)

type File struct {
	Source     source.File
	IsTest     bool
	IsMakeFile bool
	Package    string
	Imports    []string
	Concepts   []ConceptDecl
	Records    []RecordDecl
	Enums      []EnumDecl
	Functions  []FunctionDecl
	Flows      []FlowDecl
}

// ConceptDecl is a transparent named value description. Record-shaped
// concepts reuse RecordDecl with IsConcept set so every existing record
// operation and backend representation remains authoritative.
type ConceptDecl struct {
	Name         string
	Target       TypeRef
	Requirements []RefinementRequirement
	Doc          *DocComment
	Line         int
	Column       int
}

// RefinementRequirement is the single typed expression used for both static
// admission and the explicit runtime constructor synthesized by concept
// expansion. Explanation is deliberately stored as source text, not as a
// runtime descriptor.
type RefinementRequirement struct {
	Condition   Expr
	Explanation string
	Line        int
	Column      int
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
	Name string
	// TypeParameters are present only on thin template authoring declarations.
	// The project elaborator removes those declarations and emits ordinary
	// concrete records before type checking or execution.
	TypeParameters []string
	IsTemplate     bool
	TemplateOrigin *TemplateOrigin
	Doc            *DocComment
	Fields         []RecordField
	IsTable        bool
	IsConcept      bool
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
	Name           string
	TypeParameters []string
	IsTemplate     bool
	TemplateOrigin *TemplateOrigin
	SelectorOwner  *TypeRef
	SelectorField  string
	Doc            *DocComment
	SourcePath     string
	// IsGoImport marks the narrow, bodyless `go fn` declaration accepted only
	// in an OctGo *.contracts.oct companion. The Go host validates and binds it;
	// ordinary Oct execution never supplies an implementation body.
	IsGoImport bool
	IsTestFile bool
	IsFact     bool
	IsTheory   bool
	IsArtifact bool
	// ArtifactCapabilityProvider names the package-local, zero-argument
	// function whose typed value describes the authority requested by this
	// artifact. It is metadata only; the value is never an authority token.
	ArtifactCapabilityProvider string
	IsBenchmark                bool
	IsMakeFile                 bool
	IsMakePlan                 bool
	IsMakePure                 bool
	IsMakeNoWhile              bool
	RequiresMakeAuthority      bool
	InlineData                 []InlineDataRow
	Suites                     []string
	CycleTime                  Expr
	Parameters                 []Parameter
	ReturnType                 TypeRef
	IsFallible                 bool
	ErrorType                  TypeRef
	Body                       Block
	// IsRefinementConstructor marks compiler-generated, package-local checked
	// construction. It permits the final base-representation return to acquire
	// the declared refinement; user functions never receive this privilege.
	IsRefinementConstructor bool
}

type FlowDecl struct {
	Name           string
	TypeParameters []string
	IsTemplate     bool
	TemplateOrigin *TemplateOrigin
	Parameters     []Parameter
	TurnInput      *Parameter
	YieldType      *TypeRef
	ReturnType     TypeRef
	Board          []BoardField
	States         []StateDecl
	EntryState     string
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
	Package       string
	Name          string
	TypeArguments []TypeRef
	TupleOf       []TypeRef
	Dimension     dimension.Dimension
	HasUnit       bool
	IsArray       bool
	ArrayDepth    int
	VectorOf      *TypeRef
	MatrixOf      *TypeRef
	Function      *FunctionTypeRef
	// These fields retain compile-time provenance after Selector<R, F>
	// erases to the exact ordinary function type fn(R) -> F.
	SelectorOwner  *TypeRef
	SelectorResult *TypeRef
}

// TemplateOrigin survives elaboration on each concrete declaration so
// diagnostics and future discovery tooling can explain where specialization
// came from without introducing runtime template metadata.
type TemplateOrigin struct {
	Package       string
	Declaration   string
	TypeArguments []TypeRef
	// InstantiationChain is diagnostic-only provenance from the outermost
	// request through this concrete specialization. It is erased with the
	// rest of TemplateOrigin before runtime.
	InstantiationChain []string
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

type FieldIndexAssignStmt struct {
	Target  string
	Field   string
	Indices []Expr
	Value   Expr
}

func (FieldIndexAssignStmt) stmtNode() {}

type ReturnStmt struct {
	Value Expr
}

func (ReturnStmt) stmtNode() {}

type ExprStmt struct {
	Value Expr
}

func (ExprStmt) stmtNode() {}

type ForDirection int

const (
	ForDirectionAsc ForDirection = iota
	ForDirectionDesc
)

type ForStmt struct {
	Name        string
	Range       Expr
	Direction   ForDirection
	DescendStep Expr
	Body        Block
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

type YieldStmt struct{ Value Expr }

func (YieldStmt) stmtNode() {}

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
	Line          int
	Column        int
}

func (CallExpr) exprNode() {}

// FunctionExpr is an anonymous function value. Captures are an explicit,
// ordered environment constructed when the expression is evaluated.
type FunctionExpr struct {
	Parameters []Parameter
	ReturnType TypeRef
	IsFallible bool
	ErrorType  *TypeRef
	Captures   []CaptureBinding
	Body       Block
	Line       int
	Column     int
}

func (FunctionExpr) exprNode() {}

type CaptureBinding struct {
	Name   string
	Value  Expr
	Line   int
	Column int
}

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
	EnumTarget      *TypeRef
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
	TypeName      string
	TypeArguments []TypeRef
	Fields        []RecordLiteralField
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

// SelectorExpr is the contextual `.Field` authoring form. It is resolved by
// early parametric elaboration to a generated exact-signature getter function.
// No selector node reaches the ordinary typechecker, interpreter, or backend.
type SelectorExpr struct {
	Field  string
	Line   int
	Column int
}

func (SelectorExpr) exprNode() {}

type EnumValueExpr struct {
	EnumName string
	Variant  string
}

func (EnumValueExpr) exprNode() {}
