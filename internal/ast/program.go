package ast

import (
	"github.com/yuechen-li-dev/oct/internal/dimension"
	"github.com/yuechen-li-dev/oct/internal/source"
)

type File struct {
	Source     source.File
	IsTest     bool
	IsMakeFile bool
	// Profile is compile-time compilation-unit metadata. M0 recognizes only
	// "Verilog" and never exposes the declaration to runtime evaluation.
	Profile   string
	Package   string
	Imports   []string
	Concepts  []ConceptDecl
	Records   []RecordDecl
	Enums     []EnumDecl
	Functions []FunctionDecl
	Flows     []FlowDecl
	// MarkupSpans lists the source extent of every Oct-XML element in the
	// file, nested elements included, in the order their parse completed.
	// Tooling that must leave markup text alone, such as the formatter, reads
	// it; no compilation phase does.
	MarkupSpans []MarkupSpan
}

// MarkupSpan is the byte range of one Oct-XML element in its source file,
// from the '<' of the opening tag to the end of the closing tag or of '/>'.
type MarkupSpan struct {
	Offset    int
	EndOffset int
	// BodyOffset is where the element's body begins, just after the '>' of
	// its opening tag. A self-closing element has no body and BodyOffset
	// equals EndOffset.
	BodyOffset int
	// TerminatorOffset is where the element's final delimiter begins: the
	// '</' of its closing tag, or the '/' of a self-closing '/>'.
	TerminatorOffset int
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
	// IsAsync marks C#-style `async fn` source. Project loading lowers these
	// declarations into ordinary FLOW machines before type checking/execution.
	IsAsync    bool
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
	// TestLane restricts a [Fact] or [Theory] to one execution lane:
	// "interpreted" or "compiled". Empty means both. TestLaneReason is the
	// author's stated reason and is required whenever TestLane is set.
	TestLane       string
	TestLaneReason string
	CycleTime      Expr
	Parameters     []Parameter
	ReturnType     TypeRef
	IsFallible     bool
	ErrorType      TypeRef
	Body           Block
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
	// AsyncLowering is inspectable compiler provenance for a source async fn.
	// The executable semantics remain the ordinary Board/States representation.
	AsyncLowering *AsyncLoweringInfo
}

type AsyncLoweringInfo struct {
	LiftedLocals  []string
	Continuations []string
}

type BoardField struct {
	Name string
	Type TypeRef
	// AsyncHandle is compiler-generated persistent state. It is intentionally
	// not legal in programmer-authored FLOW boards.
	AsyncHandle bool
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
	// FlowInstanceOf is an internal-only type produced by async lowering.
	// There is no public Future/Task/awaiter type syntax.
	FlowInstanceOf *TypeRef
	FlowIdentity   string
	// These fields retain compile-time provenance after Selector<R, F>
	// erases to the exact ordinary function type fn(R) -> F.
	SelectorOwner  *TypeRef
	SelectorResult *TypeRef
	// Inferred marks a type argument the source did not write, as in
	// `Option.None`. The parser leaves the rest of the TypeRef empty and the
	// typechecker fills it in; see AsOptionConstruction.
	Inferred bool
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

// MarkupElementExpr is the parse-time Oct-XML representation. Project
// elaboration removes every instance by producing ordinary CallExpr and
// ArrayLiteralExpr nodes before type checking, interpretation, or MIR lowering.
type MarkupElementExpr struct {
	Tag             string
	Attributes      []MarkupAttribute
	Children        []MarkupChild
	RawBody         string
	StructuredError string
	Line            int
	Column          int
}

func (MarkupElementExpr) exprNode() {}

type MarkupAttribute struct {
	Name   string
	Value  Expr
	Line   int
	Column int
}

type MarkupChild struct {
	Text   string
	Value  Expr
	Line   int
	Column int
}

type ArrayLiteralExpr struct {
	Elements []Expr
}

// RepeatExpr is one element of an array, vector or matrix-row literal that
// stands for several: `Value ... Count` is Count elements, and `Value ...`
// with no count fills the rest of an array whose length the surrounding
// construct fixes. It appears nowhere else.
//
// Value is evaluated once for each element it produces, in order, exactly as
// if the element had been written out that many times. Count is evaluated
// once, before any of them.
type RepeatExpr struct {
	Value Expr
	// Count is nil for the fill form.
	Count  Expr
	Line   int
	Column int
}

func (RepeatExpr) exprNode() {}

// IsFill reports whether the repetition has no count of its own.
func (r RepeatExpr) IsFill() bool { return r.Count == nil }

// EndsInFill reports whether expr is an array literal whose last element is
// `value ...`, and returns the literal. Only the last element can be one.
func EndsInFill(expr Expr) (ArrayLiteralExpr, bool) {
	literal, ok := expr.(ArrayLiteralExpr)
	if !ok || len(literal.Elements) == 0 {
		return ArrayLiteralExpr{}, false
	}
	repeat, repeated := literal.Elements[len(literal.Elements)-1].(RepeatExpr)
	return literal, repeated && repeat.IsFill()
}

func (ArrayLiteralExpr) exprNode() {}

type VectorLiteralExpr struct {
	Elements []Expr
}

func (VectorLiteralExpr) exprNode() {}

type MatrixLiteralExpr struct {
	Rows [][]Expr
	// RowCounts is nil, or has one entry for each row: the count of a row
	// written `[...] ... count`, and nil for a row written once. A pass that
	// rebuilds the literal must carry it over.
	RowCounts []Expr
}

// RowCount is the repetition count of row index, or nil.
func (m MatrixLiteralExpr) RowCount(index int) Expr {
	if index < len(m.RowCounts) {
		return m.RowCounts[index]
	}
	return nil
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

type AwaitExpr struct {
	Inner  Expr
	Line   int
	Column int
}

func (AwaitExpr) exprNode() {}

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

// OptionTypeName is the builtin enum `Option<T>`: `None` or `Some(T)`. It has
// no declaration. Its variants are written `Option.None` and
// `Option.Some(value)`, and with the type argument `Option<T>.None` and
// `Option<T>.Some(value)`.
const (
	OptionTypeName    = "Option"
	OptionNoneVariant = "None"
	OptionSomeVariant = "Some"
)

// OptionConstruction describes an expression that constructs an Option.
type OptionConstruction struct {
	// Variant is the name written after `Option.`; it need not be a variant.
	Variant string
	// Payload is T of the `Option<T>` constructed. Where the source wrote no
	// type argument it is the slot the typechecker fills in (TypeRef.Inferred),
	// and Resolved reports whether it has.
	Payload  TypeRef
	Resolved bool
	// Arguments is the argument list after the variant, which the parser
	// requires `Option.None` to be without.
	Arguments []Expr
}

// AsOptionConstruction recognises an Option value. The parser gives all four
// spellings one shape: a call of `Option.Variant` that carries T as its one
// type argument. A call is a node every pass that walks expressions already
// knows, and a type argument is one they already carry along.
//
// `Option.None` and `Option.Some(value)` do not say what T is. Their type
// argument is a slot marked Inferred, and the typechecker, which alone knows
// the type the site declares, writes T into it (see checkOptionConstruction
// in internal/typecheck). A pass that runs after the typechecker therefore
// reads every Option construction as if its type argument had been written.
func AsOptionConstruction(expr Expr) (OptionConstruction, bool) {
	node, ok := expr.(CallExpr)
	if !ok || len(node.TypeArguments) != 1 {
		return OptionConstruction{}, false
	}
	callee, ok := node.Callee.(FieldAccessExpr)
	if !ok {
		return OptionConstruction{}, false
	}
	target, ok := callee.Target.(IdentifierExpr)
	if !ok || target.Name != OptionTypeName {
		return OptionConstruction{}, false
	}
	payload := node.TypeArguments[0]
	return OptionConstruction{
		Variant:   callee.Field,
		Payload:   payload,
		Resolved:  !payload.IsUnresolved(),
		Arguments: node.Arguments,
	}, true
}

// IsUnresolved reports whether an inferred type has yet to be filled in.
func (t TypeRef) IsUnresolved() bool {
	return t.Inferred && t.Name == "" && t.Function == nil && t.VectorOf == nil && t.MatrixOf == nil && t.FlowInstanceOf == nil && len(t.TupleOf) == 0
}

// CallType is the type a call is made at: its one type argument, once that
// is resolved. `Json.Load<Ticket>(path)` writes it. `Json.Text(value)` leaves
// it to the typechecker, which fills in the slot the parser made; until then,
// and for a call with no type argument or several, there is none.
func CallType(call CallExpr) (TypeRef, bool) {
	if len(call.TypeArguments) != 1 || call.TypeArguments[0].IsUnresolved() {
		return TypeRef{}, false
	}
	return call.TypeArguments[0], true
}

// IsOptionType reports whether a type reference is `Option<T>` itself, not an
// array of them.
func IsOptionType(t TypeRef) bool {
	return t.Package == "" && t.Name == OptionTypeName && len(t.TypeArguments) == 1 && !t.IsArray && t.ArrayDepth == 0
}

type EnumValueExpr struct {
	EnumName string
	Variant  string
}

func (EnumValueExpr) exprNode() {}
