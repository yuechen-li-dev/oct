package ast

import "github.com/yuechen-li-dev/oct/internal/source"

type Module struct {
	Source    source.File
	Span      source.Span
	Namespace string
	Uses      []string
	Decls     []Decl
}

type Decl interface{ declNode() }

type TemplateParam struct {
	Span        source.Span
	Name        string
	ConceptName string
}

type AttributePlacement string

const (
	AttributePlacementField    AttributePlacement = "field"
	AttributePlacementStmt     AttributePlacement = "stmt"
	AttributePlacementExpr     AttributePlacement = "expr"
	AttributePlacementFunction AttributePlacement = "function"
)

type Attribute struct {
	Span      source.Span
	Name      string
	Arguments []Expr
	Placement AttributePlacement
	Line      int
	Column    int
}

type TypeAliasDecl struct {
	Span source.Span
	Name string
	Type TypeRef
}

func (TypeAliasDecl) declNode() {}

// SpaceGroupDecl is parser-facing sugar. BuildModule deterministically expands
// it to ordinary TypeAliasDecl nodes before validation and lowering.
type SpaceGroupDecl struct {
	Span     source.Span
	PathSpan source.Span
	Path     string
	Members  []SpaceGroupMember
}

func (SpaceGroupDecl) declNode() {}

type SpaceGroupMember struct {
	Span     source.Span
	NameSpan source.Span
	Name     string
	Type     TypeRef
}

type RecordDecl struct {
	Span   source.Span
	Name   string
	Fields []Field
}

func (RecordDecl) declNode() {}

type BoardDecl struct {
	Span   source.Span
	Name   string
	Fields []Field
}

func (BoardDecl) declNode() {}

type StreamDecl struct {
	Span   source.Span
	Name   string
	Fields []Field
}

func (StreamDecl) declNode() {}

type ConceptDecl struct {
	Span         source.Span
	Name         string
	Members      []ConceptMember
	Requirements []RequireStmt
}

func (ConceptDecl) declNode() {}

type ConceptMember interface{ conceptMemberNode() }

type ConceptField struct {
	Span         source.Span
	Name         string
	Type         TypeRef
	DefaultValue Expr
}

func (ConceptField) conceptMemberNode() {}

type ConceptGroup struct {
	Span    source.Span
	Name    string
	Members []ConceptMember
}

func (ConceptGroup) conceptMemberNode() {}

type ConfigAssignmentStyle string

const (
	ConfigAssignmentLegacy   ConfigAssignmentStyle = "legacy"
	ConfigAssignmentFatArrow ConfigAssignmentStyle = "fat_arrow"
)

type ConfigField struct {
	Span  source.Span
	Path  string
	Value Expr
	Style ConfigAssignmentStyle
}

type ConfigDecl struct {
	Span         source.Span
	Name         string
	ConceptName  string
	Fields       []ConfigField
	Requirements []RequireStmt
}

func (ConfigDecl) declNode() {}

type EnumDecl struct {
	Span     source.Span
	Name     string
	Variants []EnumVariant
}

func (EnumDecl) declNode() {}

type EnumVariant struct {
	Span    source.Span
	Name    string
	Fields  []Field
	Payload bool
}

type ShaderDecl struct {
	Span               source.Span
	Name               string
	Template           *TemplateParam
	ResourceBundleName string
	Resources          []ResourceDecl
	Material           *MaterialDecl
	Workgroups         []WorkgroupDecl
	StaticAsserts      []StaticAssertStmt
	Methods            []FunctionDecl
	SpecializedConfig  map[string]uint32
}

func (ShaderDecl) declNode() {}

// MaterialDecl is graphics authoring sugar. Validation and lowering project it
// into one immutable uniform resource and a deterministic layout; it is not a
// second resource or reflection system.
type MaterialDecl struct {
	Span   source.Span
	Fields []Field
}

type CompileDecl struct {
	Span       source.Span
	ShaderName string
	ConfigName string
	AliasName  string
}

func (CompileDecl) declNode() {}

type FunctionDecl struct {
	Span       source.Span
	Attributes []Attribute
	Line       int
	Column     int
	Name       string
	Stage      string
	NumThreads *NumThreads
	Parameters []Parameter
	ReturnType TypeRef
	Body       Block
}

func (FunctionDecl) declNode() {}

type UnsupportedDecl struct {
	Kind string
	Span source.Span
}

func (UnsupportedDecl) declNode() {}

type NumThreads struct {
	Span source.Span
	X    Expr
	Y    Expr
	Z    Expr
}

type ResourceDecl struct {
	Span       source.Span
	Name       string
	Access     string
	Type       TypeRef
	Attributes []Attribute
}

type WorkgroupDecl struct {
	Span source.Span
	Name string
	Type TypeRef
}

type Field struct {
	Span       source.Span
	Name       string
	Access     string
	Type       TypeRef
	Attributes []Attribute
}

type Parameter struct {
	Span source.Span
	Name string
	Type TypeRef
}

type TypeRef struct {
	Span         source.Span
	NameSpan     source.Span
	Name         string
	Args         []TypeRef
	ArraySize    Expr
	HasArraySize bool
	TileRows     Expr
	TileCols     Expr
	HasTileShape bool
	// NDArrayShape is separate from ArraySize: ndarray is a first-class
	// fixed-shape value type rather than recursive array syntax.
	NDArrayShape      []Expr
	NDArrayShapeSpan  source.Span
	NDArrayShapeOpen  source.Span
	NDArrayShapeClose source.Span
	Access            string
	ZeroAllowed       bool
	// Space is the canonical dotted coordinate-space identity written with
	// @space(...). It is retained through semantic typing and erased physically.
	Space          string
	SpaceSpan      source.Span
	AnnotationSpan source.Span
}

func (t TypeRef) String() string {
	if t.Name == "ndarray" {
		return "ndarray<" + t.Args[0].String() + ", [Shape...]>"
	}
	if t.Name != "array" {
		return t.Name
	}
	if len(t.Args) == 0 {
		return "array"
	}
	if t.HasArraySize {
		return "array<" + t.Args[0].String() + ", N>"
	}
	return "array<" + t.Args[0].String() + ">"
}

type Block struct {
	Span       source.Span
	Statements []Stmt
}

type Stmt interface{ stmtNode() }

// BindingMutability is source-level local-binding ownership. It is deliberately
// explicit so validation never has to infer mutability from later assignments.
type BindingMutability string

const (
	BindingMutabilityImmutable BindingMutability = "immutable"
	BindingMutabilityMutable   BindingMutability = "mutable"
)

type LetStmt struct {
	Span, KeywordSpan, NameSpan source.Span
	Name                        string
	Type                        TypeRef
	Value                       Expr
	Mutability                  BindingMutability
}

func (LetStmt) stmtNode() {}

type ComptimeLetStmt struct {
	Span  source.Span
	Name  string
	Type  TypeRef
	Value Expr
}

func (ComptimeLetStmt) stmtNode() {}

type AssignStmt struct {
	Span   source.Span
	Target Expr
	Value  Expr
}

func (AssignStmt) stmtNode() {}

// TensorAssignStmt keeps indexed tensor notation distinct from ordinary indexed
// assignment. Its index names are compiler control variables, not values.
type TensorAssignStmt struct {
	Span, KeywordSpan, DestinationSpan, IndicesSpan, OperatorSpan source.Span
	Destination                                                   Expr
	AssignmentKind                                                TensorAssignmentKind
	FreeIndices                                                   []TensorIndexBinding
	Value                                                         Expr
}

func (TensorAssignStmt) stmtNode() {}

type TensorAssignmentKind string

const (
	TensorAssignSet TensorAssignmentKind = "="
	TensorAssignAdd TensorAssignmentKind = "+="
)

type TensorIndexBinding struct {
	Name string
	Span source.Span
}

type GuardedWriteStmt struct {
	Span      source.Span
	Target    Expr
	Value     Expr
	Condition Expr
}

func (GuardedWriteStmt) stmtNode() {}

type ReturnStmt struct {
	Span  source.Span
	Value Expr
}

func (ReturnStmt) stmtNode() {}

type ExprStmt struct {
	Span  source.Span
	Value Expr
}

func (ExprStmt) stmtNode() {}

// ForeignShaderStmt/Expr are target-generic on purpose; HLSL is merely the first
// registered target-language boundary.
type ForeignShaderStmt struct {
	Span                      source.Span
	TargetLanguage, RawSource string
	Captures                  []string
	Line, Column              int
}

func (ForeignShaderStmt) stmtNode() {}

type IfStmt struct {
	Span      source.Span
	Condition Expr
	ThenBody  Block
	ElseBody  *Block
}

func (IfStmt) stmtNode() {}

type GuardWhenStmt struct {
	Span     source.Span
	Cases    []GuardWhenCase
	ElseBody *Block
}

func (GuardWhenStmt) stmtNode() {}

type GuardWhenCase struct {
	Span      source.Span
	Condition Expr
	Body      Block
}

type FlowStmt struct {
	Span   source.Span
	Name   string
	Boards []FlowBoardDecl
	States []StateBlock
}

func (FlowStmt) stmtNode() {}

// Flow transitions are source-level control operations. They are deliberately
// not calls: validation resolves them to state IDs for the M31b handoff.
type GotoFlowStateStmt struct {
	Span       source.Span
	Target     string
	TargetSpan source.Span
}

func (GotoFlowStateStmt) stmtNode() {}

type PushFlowStateStmt struct {
	Span       source.Span
	Target     string
	TargetSpan source.Span
}

func (PushFlowStateStmt) stmtNode() {}

type PopFlowStateStmt struct{ Span source.Span }

func (PopFlowStateStmt) stmtNode() {}

type FinishFlowStmt struct{ Span source.Span }

func (FinishFlowStmt) stmtNode() {}

type FlowBoardDecl struct {
	Span        source.Span
	Name        string
	Type        TypeRef
	Initializer Expr
}

type StateBlock struct {
	Span     source.Span
	Name     string
	NameSpan source.Span
	Body     Block
}

type ComptimeIfStmt struct {
	Span      source.Span
	Condition Expr
	ThenBody  Block
	ElseBody  *Block
}

func (ComptimeIfStmt) stmtNode() {}

type ComptimeMatchStmt struct {
	Span    source.Span
	Subject Expr
	Arms    []ComptimeMatchArm
}

func (ComptimeMatchStmt) stmtNode() {}

type ComptimeMatchArm struct {
	Span    source.Span
	Pattern Expr
	IsElse  bool
	Body    Block
}

type ComptimeWhenUtilityStmt struct {
	Span     source.Span
	Cases    []ComptimeWhenUtilityCase
	ElseBody *Block
}

func (ComptimeWhenUtilityStmt) stmtNode() {}

type ComptimeWhenUtilityCase struct {
	Span      source.Span
	Label     string
	Condition Expr
	Score     Expr
	Body      Block
}

type ComptimeForStmt struct {
	Span  source.Span
	Name  string
	Start Expr
	End   Expr
	Body  Block
}

func (ComptimeForStmt) stmtNode() {}

type ForStmt struct {
	Span       source.Span
	Attributes []Attribute
	Name       string
	Start      Expr
	End        Expr
	Step       Expr
	Body       Block
}

func (ForStmt) stmtNode() {}

type RequireStmt struct {
	Span source.Span
	Expr Expr
	Text string
}

type StaticAssertStmt struct {
	Span source.Span
	Expr Expr
	Text string
}

func (StaticAssertStmt) stmtNode() {}

type Expr interface{ exprNode() }

// ExprSpan returns the compiler-owned span of an ordinary expression. Unknown
// is returned only for legacy expression forms not yet supplied by a parser.
func ExprSpan(e Expr) source.Span {
	switch x := e.(type) {
	case IntegerLiteral:
		return x.Span
	case FloatLiteral:
		return x.Span
	case BoolLiteral:
		return x.Span
	case StringLiteral:
		return x.Span
	case ArrayLiteral:
		return x.Span
	case FillExpr:
		return x.Span
	case GenerateExpr:
		return x.Span
	case IdentifierExpr:
		return x.Span
	case ForeignShaderExpr:
		return x.Span
	case FieldAccessExpr:
		return x.Span
	case IndexExpr:
		return x.Span
	case GuardedReadExpr:
		return x.Span
	case CallExpr:
		return x.Span
	case BinaryExpr:
		return x.Span
	case UnaryExpr:
		return x.Span
	case ParenExpr:
		return x.Span
	case WhenUtilityExpr:
		return x.Span
	case WithExpr:
		return x.Span
	case DeriveExpr:
		return x.Span
	case ReductionExpr:
		return x.Span
	case TensorReductionExpr:
		return x.Span
	case EnumConstructExpr:
		return x.Span
	case BoardLiteralExpr:
		return x.Span
	case MatchExpr:
		return x.Span
	}
	return source.Span{}
}

func StmtSpan(s Stmt) source.Span {
	switch x := s.(type) {
	case LetStmt:
		return x.Span
	case ComptimeLetStmt:
		return x.Span
	case AssignStmt:
		return x.Span
	case TensorAssignStmt:
		return x.Span
	case GuardedWriteStmt:
		return x.Span
	case ReturnStmt:
		return x.Span
	case ExprStmt:
		return x.Span
	case ForeignShaderStmt:
		return x.Span
	case IfStmt:
		return x.Span
	case GuardWhenStmt:
		return x.Span
	case FlowStmt:
		return x.Span
	case GotoFlowStateStmt:
		return x.Span
	case PushFlowStateStmt:
		return x.Span
	case PopFlowStateStmt:
		return x.Span
	case FinishFlowStmt:
		return x.Span
	case ComptimeIfStmt:
		return x.Span
	case ComptimeMatchStmt:
		return x.Span
	case ComptimeWhenUtilityStmt:
		return x.Span
	case ComptimeForStmt:
		return x.Span
	case ForStmt:
		return x.Span
	case StaticAssertStmt:
		return x.Span
	}
	return source.Span{}
}

type ReductionOp string

const (
	ReductionSum     ReductionOp = "sum"
	ReductionProduct ReductionOp = "product"
	ReductionMax     ReductionOp = "max"
	ReductionMin     ReductionOp = "min"
)

type IntegerLiteral struct {
	Span  source.Span
	Value string
}

func (IntegerLiteral) exprNode() {}

type FloatLiteral struct {
	Span  source.Span
	Value string
}

func (FloatLiteral) exprNode() {}

type BoolLiteral struct {
	Span  source.Span
	Value bool
}

func (BoolLiteral) exprNode() {}

type StringLiteral struct {
	Span  source.Span
	Value string
}

func (StringLiteral) exprNode() {}

// ArrayLiteral is target-typed; M33a uses it for dense flat ndarray payloads.
type ArrayLiteral struct {
	Span     source.Span
	Elements []Expr
}

func (ArrayLiteral) exprNode() {}

// FillExpr constructs every element of a contextually supplied fixed shape.
type FillExpr struct {
	Span, KeywordSpan, OpenParenSpan, CloseParenSpan source.Span
	Value                                            Expr // first argument retained for concise consumers
	Arguments                                        []Expr
}

func (FillExpr) exprNode() {}

// GenerateExpr constructs a contextually supplied fixed shape from ordered
// immutable coordinate binders.
type GenerateExpr struct {
	Span, KeywordSpan, OpenBracketSpan, CloseBracketSpan, OpenParenSpan, CloseParenSpan source.Span
	Binders                                                                             []IdentifierExpr
	Body                                                                                Expr
}

func (GenerateExpr) exprNode() {}

type IdentifierExpr struct {
	Span source.Span
	Name string
}

func (IdentifierExpr) exprNode() {}

type ForeignShaderExpr struct {
	Span           source.Span
	TargetLanguage string
	ResultType     TypeRef
	RawSource      string
	Captures       []string
	Line, Column   int
}

func (ForeignShaderExpr) exprNode() {}

type FieldAccessExpr struct {
	Span   source.Span
	Target Expr
	Field  string
}

func (FieldAccessExpr) exprNode() {}

type IndexExpr struct {
	Span   source.Span
	Target Expr
	// Indices is the canonical ordered index list. Index/Index2/HasSecond are
	// retained only as compatibility adapters for pre-M32a consumers.
	Indices   []Expr
	Index     Expr
	Index2    Expr
	HasSecond bool
}

func (IndexExpr) exprNode() {}

func IndexExpressions(e IndexExpr) []Expr {
	if len(e.Indices) != 0 {
		return e.Indices
	}
	indices := []Expr{e.Index}
	if e.HasSecond {
		indices = append(indices, e.Index2)
	}
	return indices
}

type GuardedReadExpr struct {
	Span      source.Span
	Target    Expr
	Condition Expr
	Fallback  Expr
}

func (GuardedReadExpr) exprNode() {}

type CallExpr struct {
	Span   source.Span
	Callee Expr
	// TypeArgument is deliberately available only to compiler-owned intrinsic
	// families. The parser records its spans; validation rejects generic syntax
	// on ordinary functions before lowering.
	TypeArgument                  *TypeRef
	OpenAngleSpan, CloseAngleSpan source.Span
	Arguments                     []Expr
}

func (CallExpr) exprNode() {}

type BinaryExpr struct {
	Span     source.Span
	Left     Expr
	Operator string
	Right    Expr
}

func (BinaryExpr) exprNode() {}

type UnaryExpr struct {
	Span     source.Span
	Operator string
	Operand  Expr
}

func (UnaryExpr) exprNode() {}

type ParenExpr struct {
	Span  source.Span
	Inner Expr
}

func (ParenExpr) exprNode() {}

type WhenUtilityExpr struct {
	Span  source.Span
	Cases []UtilityCase
	Else  Expr
}

func (WhenUtilityExpr) exprNode() {}

type UtilityCase struct {
	Span      source.Span
	Value     Expr
	Condition Expr
	Score     Expr
}

type WithExpr struct {
	Span    source.Span
	Base    Expr
	Updates []FieldUpdate
}

func (WithExpr) exprNode() {}

type DeriveExpr struct {
	Span   source.Span
	Fields []DeriveField
}

func (DeriveExpr) exprNode() {}

type DeriveField struct {
	Span  source.Span
	Name  string
	Value Expr
}

type ReductionExpr struct {
	Span       source.Span
	Attributes []Attribute
	Op         ReductionOp
	Name       string
	Start      Expr
	End        Expr
	Step       Expr
	Body       Expr
}

func (ReductionExpr) exprNode() {}

// TensorReductionExpr is the explicit indexed contraction form: Sum[k](body).
type TensorReductionExpr struct {
	Span, SumSpan, IndicesSpan, BodySpan source.Span
	Kind                                 string
	Indices                              []TensorReductionBinding
	Value                                Expr
}

func (TensorReductionExpr) exprNode() {}

type TensorReductionBinding struct {
	Name string
	Span source.Span
}

type FieldUpdate struct {
	Span  source.Span
	Name  string
	Value Expr
}

type EnumConstructExpr struct {
	Span        source.Span
	EnumName    string
	VariantName string
	Fields      []FieldInit
}

func (EnumConstructExpr) exprNode() {}

type BoardLiteralExpr struct {
	Span     source.Span
	TypeName string
	Fields   []FieldInit
}

func (BoardLiteralExpr) exprNode() {}

type FieldInit struct {
	Span  source.Span
	Name  string
	Value Expr
}

type MatchExpr struct {
	Span    source.Span
	Subject Expr
	Arms    []MatchArm
}

func (MatchExpr) exprNode() {}

type MatchArm struct {
	Span        source.Span
	EnumName    string
	VariantName string
	BindingName string
	Value       Expr
}
