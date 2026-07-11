package vdmir

import "github.com/yuechen-li-dev/oct/internal/source"

type Module struct {
	Provenance Provenance
	// ForeignTargets is the explicit portability requirement collected during
	// lowering. A backend must reject a module that names a target it cannot own.
	ForeignTargets []string
	Namespace      string
	TypeAliases    []TypeAlias
	Records        []Record
	Boards         []Board
	Streams        []Stream
	Enums          []Enum
	Resources      []Resource
	Workgroups     []WorkgroupMemoryDecl
	Functions      []Function
	EntryPoints    []ComputeEntryPoint
	Flows          []Flow
}

type FlowTerminatorKind string

const (
	FlowTerminatorFallthrough FlowTerminatorKind = "fallthrough"
	FlowTerminatorPush        FlowTerminatorKind = "push"
	FlowTerminatorPop         FlowTerminatorKind = "pop"
	FlowTerminatorGoto        FlowTerminatorKind = "goto"
	FlowTerminatorFinish      FlowTerminatorKind = "finish"
)

const FlowCompleteStateID = -1

type Flow struct {
	Provenance    Provenance
	Name          string
	FunctionName  string
	ShaderName    string
	Entry         int
	States        []FlowState
	MaxStackDepth uint32
	HasPushPop    bool
	HasGoto       bool
	SourceSpan    source.Span
}

type FlowState struct {
	ID                  int
	Name                string
	Terminator          FlowTerminator
	HasWorkgroupBarrier bool
	Reachable           bool
	ReachableDepths     []uint32
	SourceSpan          source.Span
	NameSpan            source.Span
}

type FlowTerminator struct {
	Kind     FlowTerminatorKind
	Target   int
	ReturnTo int
	Span     source.Span
}

// TestProgram is the backend-neutral executable projection for an .sdslvtest
// compilation group. It intentionally contains shader semantics only; host
// display names, artifact paths, process policy, and manifest DTOs stay in
// test orchestration.
type TestProgram struct {
	Module Module
	ABI    TestResultContract
	Groups []TestCompilationGroup
}

const TestInputResourceName = "__sdslv_test_input"

type TestInputContract struct {
	ABIVersion   uint32
	Binding      Binding
	ValueKind    TestInputValueKind
	ElementCount uint32
	PayloadWords []uint32
}

type TestInputValueKind string

const (
	TestInputValueNone  TestInputValueKind = "none"
	TestInputValueBool  TestInputValueKind = "bool"
	TestInputValueInt   TestInputValueKind = "int"
	TestInputValueUInt  TestInputValueKind = "uint"
	TestInputValueFloat TestInputValueKind = "float"
)

type TestResultContract struct {
	ABIVersion  uint32
	LinearIndex TestInvocationLinearIndex
}

type TestInvocationLinearIndex struct{ UsesXYZ bool }

type TestCompilationGroup struct {
	ID            string
	WorkgroupSize [3]uint32
	Entries       []TestEntry
}

type TestEntry struct {
	Selector       uint32
	FunctionName   string
	TheoryRow      *TestTheoryRow
	DispatchGroups [3]uint32
	Input          TestInputContract
}

type TestTheoryRow struct {
	Index  uint32
	Values []LiteralExpr
}

type Provenance struct {
	Path string
}

func ProvenanceFromFile(file source.File) Provenance {
	return Provenance{Path: file.Path}
}

type TypeAlias struct {
	Provenance Provenance
	Name       string
	Target     Type
}

type Record struct {
	Provenance Provenance
	Name       string
	Fields     []Field
}

type Board struct {
	Provenance Provenance
	Name       string
	Fields     []Field
}

type Stream struct {
	Provenance Provenance
	Name       string
	Fields     []Field
}

type Enum struct {
	Provenance Provenance
	Name       string
	Variants   []EnumVariant
}

type EnumVariant struct {
	Name       string
	Payload    []Field
	HasPayload bool
}

type Field struct {
	Provenance Provenance
	Name       string
	Type       Type
}

type Resource struct {
	Provenance  Provenance
	BundleName  string
	Name        string
	ElementType Type
	Access      ResourceAccess
	Binding     Binding
}

type WorkgroupMemoryDecl struct {
	Provenance  Provenance
	ShaderName  string
	Name        string
	Type        Type
	ElementType Type
	Length      int
	Rows        int
	Cols        int
	IsTile      bool
}

type Binding struct {
	Set      int
	Binding  int
	Explicit bool
}

type LoopHint string

const (
	LoopHintNone   LoopHint = ""
	LoopHintUnroll LoopHint = "unroll"
	LoopHintLoop   LoopHint = "loop"
)

type ResourceAccess string

const (
	ResourceReadOnly  ResourceAccess = "readonly"
	ResourceReadWrite ResourceAccess = "readwrite"
)

type ComputeBuiltin string

const (
	BuiltinDispatchThreadID ComputeBuiltin = "DispatchThreadID"
	BuiltinGroupThreadID    ComputeBuiltin = "GroupThreadID"
	BuiltinGroupID          ComputeBuiltin = "GroupID"
	BuiltinGroupIndex       ComputeBuiltin = "GroupIndex"
)

type ComputeEntryPoint struct {
	Provenance   Provenance
	ShaderName   string
	FunctionName string
	EmittedName  string
	NumThreadsX  int
	NumThreadsY  int
	NumThreadsZ  int
	Metadata     []MetadataField
	ConfigValues []MetadataField
	Params       []Parameter
	ThreadParams []ComputeThreadBinding
	Builtins     []BuiltinParam
}

type MetadataField struct {
	Name  string
	Value uint32
}

type ComputeThreadBinding struct {
	ParamName string
	TypeName  string
	Fields    []ComputeThreadFieldBinding
}

type ComputeThreadFieldBinding struct {
	FieldName    string
	BuiltinName  string
	BuiltinField string
}

type BuiltinParam struct {
	Name       string
	Type       Type
	Semantic   string
	Builtin    ComputeBuiltin
	Available  bool
	Referenced bool
}

type Function struct {
	Provenance  Provenance
	Name        string
	EmittedName string
	ShaderName  string
	Params      []Parameter
	ReturnType  Type
	Locals      []Local
	Body        Block
}

type Parameter struct {
	Provenance Provenance
	Name       string
	Type       Type
}

type Local struct {
	Provenance Provenance
	Name       string
	Type       Type
}

type Block struct {
	Statements []Stmt
}

type Stmt interface{ stmtNode() }

type LetStmt struct {
	Provenance Provenance
	Name       string
	Type       Type
	Value      Expr
}

func (LetStmt) stmtNode() {}

type AssignStmt struct {
	Provenance Provenance
	Target     Expr
	Value      Expr
}

func (AssignStmt) stmtNode() {}

type GuardedWriteStmt struct {
	Provenance Provenance
	Target     Expr
	Value      Expr
	Condition  Expr
}

func (GuardedWriteStmt) stmtNode() {}

type ReturnStmt struct {
	Provenance Provenance
	Value      Expr
}

func (ReturnStmt) stmtNode() {}

type IfStmt struct {
	Provenance Provenance
	Condition  Expr
	ThenBody   Block
	ElseBody   *Block
}

func (IfStmt) stmtNode() {}

type ForRangeStmt struct {
	Provenance Provenance
	LoopHint   LoopHint
	Name       string
	Type       Type
	Start      Expr
	End        Expr
	Step       Expr
	Body       Block
}

func (ForRangeStmt) stmtNode() {}

type ExprStmt struct {
	Provenance Provenance
	Value      Expr
}

func (ExprStmt) stmtNode() {}

// AssertStmt is the test-only, compiler-owned assertion operation.  Its
// operands are already ordinary lowered VD-MIR expressions; HLSL emission
// receives no AST call syntax, manifest assertion metadata, or source text.
// Expected/Actual/Tolerance are evaluated into fresh locals by the backend in
// source order before the comparison is made.
type AssertStmt struct {
	Provenance     Provenance
	Kind           AssertKind
	Expected       Expr
	Actual         Expr
	Tolerance      Expr
	CallSpan       source.Span
	OperandSpans   []source.Span
	LexicalIndex   int
	ValueKind      AssertValueKind
	ComponentCount uint32
}

func (AssertStmt) stmtNode() {}

type AssertKind string

const (
	AssertTrue     AssertKind = "Assert.True"
	AssertFalse    AssertKind = "Assert.False"
	AssertEqual    AssertKind = "Assert.Equal"
	AssertNotEqual AssertKind = "Assert.NotEqual"
	AssertNear     AssertKind = "Assert.Near"
)

type AssertValueKind uint32

const (
	AssertValueUnknown AssertValueKind = iota
	AssertValueBool
	AssertValueInt
	AssertValueUInt
	AssertValueFloat
)

type ForeignShaderStmt struct {
	Provenance                Provenance
	TargetLanguage, RawSource string
	Captures                  []string
	SourceLine                int
}

func (ForeignShaderStmt) stmtNode() {}

type BlockStmt struct {
	Provenance Provenance
	Body       Block
}

func (BlockStmt) stmtNode() {}

type Expr interface {
	exprNode()
	Type() Type
}

type ReductionOp string

const (
	ReductionSum     ReductionOp = "sum"
	ReductionProduct ReductionOp = "product"
	ReductionMax     ReductionOp = "max"
	ReductionMin     ReductionOp = "min"
)

type Intrinsic string

const (
	IntrinsicWorkgroupBarrier               Intrinsic = "WorkgroupBarrier"
	IntrinsicWorkgroupMemoryBarrier         Intrinsic = "WorkgroupMemoryBarrier"
	IntrinsicWorkgroupMemoryBarrierWithSync Intrinsic = "WorkgroupMemoryBarrierWithSync"
)

type LiteralExpr struct {
	Provenance Provenance
	ExprType   Type
	Kind       LiteralKind
	Value      string
}

func (LiteralExpr) exprNode()    {}
func (e LiteralExpr) Type() Type { return e.ExprType }

type LiteralKind string

const (
	LiteralInteger LiteralKind = "integer"
	LiteralFloat   LiteralKind = "float"
	LiteralBool    LiteralKind = "bool"
	LiteralString  LiteralKind = "string"
)

type VarRefExpr struct {
	Provenance Provenance
	ExprType   Type
	Name       string
	Kind       VarKind
}

func (VarRefExpr) exprNode()    {}
func (e VarRefExpr) Type() Type { return e.ExprType }

type ForeignShaderExpr struct {
	Provenance                Provenance
	ExprType                  Type
	TargetLanguage, RawSource string
	Captures                  []string
	SourceLine                int
}

func (ForeignShaderExpr) exprNode()    {}
func (e ForeignShaderExpr) Type() Type { return e.ExprType }

type VarKind string

const (
	VarLocal    VarKind = "local"
	VarParam    VarKind = "param"
	VarResource VarKind = "resource"
	VarBuiltin  VarKind = "builtin"
	VarFunction VarKind = "function"
)

type FieldAccessExpr struct {
	Provenance Provenance
	ExprType   Type
	Target     Expr
	Field      string
}

func (FieldAccessExpr) exprNode()    {}
func (e FieldAccessExpr) Type() Type { return e.ExprType }

type IndexExpr struct {
	Provenance Provenance
	ExprType   Type
	Target     Expr
	Index      Expr
}

func (IndexExpr) exprNode()    {}
func (e IndexExpr) Type() Type { return e.ExprType }

type Index2DExpr struct {
	Provenance Provenance
	ExprType   Type
	Target     Expr
	Row        Expr
	Col        Expr
	Stride     Expr
}

func (Index2DExpr) exprNode()    {}
func (e Index2DExpr) Type() Type { return e.ExprType }

type GuardedReadExpr struct {
	Provenance Provenance
	ExprType   Type
	Target     Expr
	Condition  Expr
	Fallback   Expr
}

func (GuardedReadExpr) exprNode()    {}
func (e GuardedReadExpr) Type() Type { return e.ExprType }

type RegTileZeroExpr struct {
	Provenance Provenance
	ExprType   Type
}

func (RegTileZeroExpr) exprNode()    {}
func (e RegTileZeroExpr) Type() Type { return e.ExprType }

type RowMajorViewExpr struct {
	Provenance Provenance
	ExprType   Type
	Buffer     Expr
	Rows       Expr
	Cols       Expr
	Access     ResourceAccess
}

func (RowMajorViewExpr) exprNode()    {}
func (e RowMajorViewExpr) Type() Type { return e.ExprType }

type CallExpr struct {
	Provenance Provenance
	ExprType   Type
	Callee     Expr
	Arguments  []Expr
}

func (CallExpr) exprNode()    {}
func (e CallExpr) Type() Type { return e.ExprType }

type IntrinsicCallExpr struct {
	Provenance Provenance
	ExprType   Type
	Intrinsic  Intrinsic
	Arguments  []Expr
}

func (IntrinsicCallExpr) exprNode()    {}
func (e IntrinsicCallExpr) Type() Type { return e.ExprType }

type BinaryExpr struct {
	Provenance Provenance
	ExprType   Type
	Left       Expr
	Operator   string
	Right      Expr
}

func (BinaryExpr) exprNode()    {}
func (e BinaryExpr) Type() Type { return e.ExprType }

type UnaryExpr struct {
	Provenance Provenance
	ExprType   Type
	Operator   string
	Operand    Expr
}

func (UnaryExpr) exprNode()    {}
func (e UnaryExpr) Type() Type { return e.ExprType }

type WhenUtilityExpr struct {
	Provenance Provenance
	ExprType   Type
	Cases      []WhenUtilityCase
	Else       Expr
}

func (WhenUtilityExpr) exprNode()    {}
func (e WhenUtilityExpr) Type() Type { return e.ExprType }

type WhenUtilityCase struct {
	Provenance Provenance
	Value      Expr
	Guard      Expr
	Score      Expr
}

type WithExpr struct {
	Provenance Provenance
	ExprType   Type
	Base       Expr
	Updates    []FieldUpdate
}

func (WithExpr) exprNode()    {}
func (e WithExpr) Type() Type { return e.ExprType }

type ReductionExpr struct {
	Provenance Provenance
	ExprType   Type
	LoopHint   LoopHint
	Op         ReductionOp
	Name       string
	IndexType  Type
	Start      Expr
	End        Expr
	Step       Expr
	Body       Expr
}

func (ReductionExpr) exprNode()    {}
func (e ReductionExpr) Type() Type { return e.ExprType }

type FieldUpdate struct {
	Name  string
	Value Expr
}

type EnumConstructExpr struct {
	Provenance  Provenance
	ExprType    Type
	EnumName    string
	VariantName string
	Fields      []FieldInit
}

func (EnumConstructExpr) exprNode()    {}
func (e EnumConstructExpr) Type() Type { return e.ExprType }

type BoardConstructExpr struct {
	Provenance Provenance
	ExprType   Type
	TypeName   string
	Fields     []FieldInit
}

func (BoardConstructExpr) exprNode()    {}
func (e BoardConstructExpr) Type() Type { return e.ExprType }

type DeriveExpr struct {
	Provenance Provenance
	ExprType   Type
	TypeName   string
	Fields     []DeriveField
}

func (DeriveExpr) exprNode()    {}
func (e DeriveExpr) Type() Type { return e.ExprType }

type FieldInit struct {
	Name  string
	Value Expr
}

type DeriveField struct {
	Name     string
	TempName string
	Value    Expr
}

type MatchExpr struct {
	Provenance Provenance
	ExprType   Type
	Subject    Expr
	Arms       []MatchArm
}

func (MatchExpr) exprNode()    {}
func (e MatchExpr) Type() Type { return e.ExprType }

type MatchArm struct {
	EnumName     string
	VariantName  string
	BindingName  string
	BindingType  Type
	VariantIndex int
	Value        Expr
}

type Type struct {
	Kind         TypeKind
	Name         string
	Element      *Type
	ArraySize    int
	HasArraySize bool
	Rows         int
	Cols         int
	Access       ResourceAccess
}

type TypeKind string

const (
	TypeVoid         TypeKind = "void"
	TypeBool         TypeKind = "bool"
	TypeI32          TypeKind = "i32"
	TypeU32          TypeKind = "u32"
	TypeF32          TypeKind = "f32"
	TypeUint2        TypeKind = "uint2"
	TypeUint3        TypeKind = "uint3"
	TypeUint4        TypeKind = "uint4"
	TypeFloat2       TypeKind = "float2"
	TypeFloat3       TypeKind = "float3"
	TypeFloat4       TypeKind = "float4"
	TypeRuntimeArray TypeKind = "runtime_array"
	TypeArray        TypeKind = "array"
	TypeTile         TypeKind = "tile"
	TypeRegTile      TypeKind = "reg_tile"
	TypeMatrixView   TypeKind = "matrix_view"
	TypeRecord       TypeKind = "record"
	TypeBoard        TypeKind = "board"
	TypeStream       TypeKind = "stream"
	TypeEnum         TypeKind = "enum"
	TypeAliasKind    TypeKind = "alias"
	TypeBuiltin      TypeKind = "builtin"
)

func (t Type) IsArray() bool {
	return t.Kind == TypeArray || t.Kind == TypeRuntimeArray
}
