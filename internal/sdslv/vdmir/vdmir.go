package vdmir

import "github.com/yuechen-li-dev/oct/internal/source"

type Module struct {
	Provenance Provenance
	// ForeignTargets is the explicit portability requirement collected during
	// lowering. A backend must reject a module that names a target it cannot own.
	ForeignTargets []string
	// Requirements are closed compiler-owned execution contracts discovered
	// during lowering. They are semantic requirements, not user-defined target
	// strings; backends decide whether and how they can satisfy each contract.
	Requirements        []CapabilityRequirement
	Namespace           string
	TypeAliases         []TypeAlias
	Records             []Record
	Boards              []Board
	Streams             []Stream
	Enums               []Enum
	Resources           []Resource
	Materials           []Material
	Workgroups          []WorkgroupMemoryDecl
	Functions           []Function
	EntryPoints         []ComputeEntryPoint
	GraphicsEntryPoints []GraphicsEntryPoint
	GraphicsPrograms    []GraphicsProgram
	Flows               []Flow
}

type CapabilityRequirement struct {
	Kind          string
	Scope         string
	M             uint32
	N             uint32
	K             uint32
	AComponent    string
	BComponent    string
	CComponent    string
	Result        string
	InputPacking  string
	LogicalLayout string
}

const CapabilityCooperativeMatrixF16F32M16N16K16Subgroup = "cooperative-matrix-f16-f32-m16-n16-k16-subgroup"

const (
	CapabilityGraphicsVertexPixel = "graphics-vertex-pixel"
	CapabilitySampledTexture2D    = "sampled-texture2d"
	CapabilityUniformMaterial     = "uniform-material"
)

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
	ID   int
	Name string
	// Body is ordinary backend-neutral lowering; the transition is retained in
	// Terminator so backends never need to inspect source syntax.
	Body                Block
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
	Role       StreamRole
	Fields     []Field
}

type StreamRole string

const (
	StreamRoleStageValue StreamRole = "stage-value"
	StreamRoleResource   StreamRole = "resource"
	StreamRoleBuiltin    StreamRole = "builtin"
)

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
	Provenance    Provenance
	Name          string
	Type          Type
	Location      int
	HasLocation   bool
	Target        int
	HasTarget     bool
	Builtin       string
	Interpolation string
	Semantic      string
}

type Resource struct {
	Provenance  Provenance
	BundleName  string
	Name        string
	Kind        ResourceKind
	Type        Type
	ElementType Type
	Access      ResourceAccess
	Binding     Binding
}

type ResourceKind string

const (
	ResourceStorageBuffer ResourceKind = "storage-buffer"
	ResourceUniform       ResourceKind = "uniform"
	ResourceTexture2D     ResourceKind = "texture2d"
	ResourceSampler       ResourceKind = "sampler"
)

type Material struct {
	Provenance Provenance
	ShaderName string
	TypeName   string
	Binding    Binding
	Size       uint32
	Fields     []MaterialField
}

type MaterialField struct {
	Name      string
	Type      Type
	Offset    uint32
	Size      uint32
	Alignment uint32
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
	BuiltinDispatchThreadID          ComputeBuiltin = "DispatchThreadID"
	BuiltinGroupThreadID             ComputeBuiltin = "GroupThreadID"
	BuiltinGroupID                   ComputeBuiltin = "GroupID"
	BuiltinGroupIndex                ComputeBuiltin = "GroupIndex"
	BuiltinSubgroupID                ComputeBuiltin = "SubgroupId"
	BuiltinSubgroupLocalInvocationID ComputeBuiltin = "SubgroupLocalInvocationId"
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

type ShaderStage string

const (
	StageCompute ShaderStage = "compute"
	StageVertex  ShaderStage = "vertex"
	StagePixel   ShaderStage = "pixel"
)

type GraphicsEntryPoint struct {
	Provenance   Provenance
	ProgramName  string
	FunctionName string
	EmittedName  string
	Stage        ShaderStage
	Params       []GraphicsParameter
	ReturnType   Type
	Inputs       []InterfaceField
	Outputs      []InterfaceField
	Builtins     []BuiltinUse
	Targets      []PixelTarget
}

type GraphicsParameter struct {
	Name    string
	Type    Type
	Role    StreamRole
	Emitted bool
}

type InterfaceField struct {
	Stream        string
	Name          string
	Type          Type
	Location      int
	HasLocation   bool
	Builtin       string
	Interpolation string
}

type BuiltinUse struct {
	Name     string
	Builtin  string
	Type     Type
	Semantic string
}

type PixelTarget struct {
	Name   string
	Target int
	Type   Type
}

type GraphicsProgram struct {
	Provenance Provenance
	Name       string
	Vertex     string
	Pixel      string
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
	Name                string
	Type                Type
	Semantic            string
	SPIRVInputBuiltinID uint32
	Builtin             ComputeBuiltin
	Available           bool
	Referenced          bool
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

// TensorAssign is the validated, backend-neutral expansion boundary for an
// indexed tensor statement.  Its expressions are ordinary VD-MIR expressions;
// only iteration and singular destination-update semantics are structured here.
type TensorAssign struct {
	Provenance     Provenance
	Destination    Expr
	Value          Expr
	ElementType    Type
	AssignmentKind TensorAssignmentKind
	FreeIndices    []TensorIndex
	AliasPolicy    TensorAliasPolicy
	SourceSpan     source.Span
	LoopOrder      []string
}

func (TensorAssign) stmtNode() {}

type TensorAssignmentKind string

const (
	TensorAssignSet TensorAssignmentKind = "set"
	TensorAssignAdd TensorAssignmentKind = "add"
)

type TensorAliasPolicy string

const (
	TensorAliasNoDestinationRead TensorAliasPolicy = "no-destination-read"
	TensorAliasIdenticalRead     TensorAliasPolicy = "identical-index-read"
)

type TensorIndex struct {
	ID     string
	Name   string
	Extent uint32
	Span   source.Span
}

// TensorReductionExpr is an explicit Sum with ordered static iteration
// domains.  It is never source syntax and can be consumed without re-inferring
// shapes or resolving index names.
type TensorReductionExpr struct {
	Provenance Provenance
	ExprType   Type
	Kind       string
	Indices    []TensorIndex
	Body       Expr
	Identity   Expr
	Span       source.Span
}

func (TensorReductionExpr) exprNode()    {}
func (e TensorReductionExpr) Type() Type { return e.ExprType }

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
	Reason         string
	ReasonSpan     source.Span
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

// FlowStmt is emitted only for transition-bearing flows. Legacy flows retain
// their direct BlockStmt lowering and therefore pay no dispatcher overhead.
type FlowStmt struct {
	Provenance Provenance
	Flow       Flow
}

func (FlowStmt) stmtNode() {}

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
	IntrinsicWorkgroupBarrier                         Intrinsic = "WorkgroupBarrier"
	IntrinsicWorkgroupMemoryBarrier                   Intrinsic = "WorkgroupMemoryBarrier"
	IntrinsicWorkgroupMemoryBarrierWithSync           Intrinsic = "WorkgroupMemoryBarrierWithSync"
	IntrinsicDot                                      Intrinsic = "Dot"
	IntrinsicCross                                    Intrinsic = "Cross"
	IntrinsicNormalize                                Intrinsic = "Normalize"
	IntrinsicSaturate                                 Intrinsic = "Saturate"
	IntrinsicLerp                                     Intrinsic = "Lerp"
	IntrinsicReflect                                  Intrinsic = "Reflect"
	IntrinsicSampleTexture2D                          Intrinsic = "SampleTexture2D"
	IntrinsicPackF16x2                                Intrinsic = "PackF16x2"
	IntrinsicUnpackF16x2                              Intrinsic = "UnpackF16x2"
	IntrinsicBitcast                                  Intrinsic = "Bitcast"
	IntrinsicConvert                                  Intrinsic = "Convert"
	IntrinsicCooperativeMatMulF16F32M16N16K16Subgroup Intrinsic = "CooperativeMatMulF16F32M16N16K16Subgroup"
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

// NDArrayLiteral retains source-order dense values after target-type validation.
type NDArrayLiteral struct {
	Provenance Provenance
	ExprType   Type
	Elements   []Expr
}

func (NDArrayLiteral) exprNode()    {}
func (e NDArrayLiteral) Type() Type { return e.ExprType }

// Fixed-shape construction remains backend-neutral. Shape is compiler-owned
// and ordered outermost-to-innermost; HLSL never infers it from source.
type FillConstruct struct {
	Provenance Provenance
	ExprType   Type
	Value      Expr
}

func (FillConstruct) exprNode()    {}
func (e FillConstruct) Type() Type { return e.ExprType }

type GenerateConstruct struct {
	Provenance Provenance
	ExprType   Type
	Binders    []string
	Body       Expr
}

func (GenerateConstruct) exprNode()    {}
func (e GenerateConstruct) Type() Type { return e.ExprType }

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

// VectorExtractExpr is distinct from records: validation has already resolved
// the component and scalar result, so backends never guess vector semantics.
type VectorExtractExpr struct {
	Provenance Provenance
	ExprType   Type
	Target     Expr
	Component  string
}

func (VectorExtractExpr) exprNode()    {}
func (e VectorExtractExpr) Type() Type { return e.ExprType }

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

// IndexNExpr retains rank-general semantic indexing.  Categories with an
// existing physical layout may still use Index2DExpr; fixed arrays use this
// node and the backend applies deterministic row-major linearization.
type IndexNExpr struct {
	Provenance Provenance
	ExprType   Type
	Target     Expr
	Indices    []Expr
	// Extents is the ordered, compiler-known physical shape. Fixed arrays use
	// RowMajorLinear layout; rank-limited categories retain their own nodes.
	Extents []uint32
	Layout  IndexLayout
}

func (IndexNExpr) exprNode()    {}
func (e IndexNExpr) Type() Type { return e.ExprType }

type IndexLayout string

const (
	IndexLayoutRowMajorLinear IndexLayout = "row-major-linear"
)

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
	// TypeArgument is compiler-validated destination type or packed-format
	// descriptor. It is never inferred by a backend.
	TypeArgument Type
	Arguments    []Expr
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
	// Shape belongs to TypeNDArray and is ordered outermost-to-innermost.
	Shape  []uint32
	Rows   int
	Cols   int
	Access ResourceAccess
	Space  string
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
	TypeNDArray      TypeKind = "ndarray"
	TypeTile         TypeKind = "tile"
	TypeRegTile      TypeKind = "reg_tile"
	TypeMatrixView   TypeKind = "matrix_view"
	TypeRecord       TypeKind = "record"
	TypeBoard        TypeKind = "board"
	TypeStream       TypeKind = "stream"
	TypeEnum         TypeKind = "enum"
	TypeAliasKind    TypeKind = "alias"
	TypeBuiltin      TypeKind = "builtin"
	TypeTexture2D    TypeKind = "texture2d"
	TypeSampler      TypeKind = "sampler"
	TypeUniform      TypeKind = "uniform"
)

func (t Type) IsArray() bool {
	return t.Kind == TypeArray || t.Kind == TypeNDArray || t.Kind == TypeRuntimeArray
}
