package vdmir

import "github.com/yuechen-li-dev/oct/internal/source"

type Module struct {
	Provenance  Provenance
	Namespace   string
	TypeAliases []TypeAlias
	Records     []Record
	Enums       []Enum
	Resources   []Resource
	Functions   []Function
	EntryPoints []ComputeEntryPoint
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

type Enum struct {
	Provenance Provenance
	Name       string
	Variants   []string
}

type Field struct {
	Provenance Provenance
	Name       string
	Type       Type
}

type Resource struct {
	Provenance  Provenance
	Name        string
	ElementType Type
	Access      ResourceAccess
	Binding     Binding
}

type Binding struct {
	Set     int
	Binding int
}

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
	Params       []Parameter
	Builtins     []BuiltinParam
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

type Expr interface {
	exprNode()
	Type() Type
}

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

type CallExpr struct {
	Provenance Provenance
	ExprType   Type
	Callee     Expr
	Arguments  []Expr
}

func (CallExpr) exprNode()    {}
func (e CallExpr) Type() Type { return e.ExprType }

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

type Type struct {
	Kind         TypeKind
	Name         string
	Element      *Type
	ArraySize    int
	HasArraySize bool
}

type TypeKind string

const (
	TypeVoid         TypeKind = "void"
	TypeBool         TypeKind = "bool"
	TypeI32          TypeKind = "i32"
	TypeU32          TypeKind = "u32"
	TypeF32          TypeKind = "f32"
	TypeFloat2       TypeKind = "float2"
	TypeFloat3       TypeKind = "float3"
	TypeFloat4       TypeKind = "float4"
	TypeRuntimeArray TypeKind = "runtime_array"
	TypeArray        TypeKind = "array"
	TypeRecord       TypeKind = "record"
	TypeEnum         TypeKind = "enum"
	TypeAliasKind    TypeKind = "alias"
	TypeBuiltin      TypeKind = "builtin"
)

func (t Type) IsArray() bool {
	return t.Kind == TypeArray || t.Kind == TypeRuntimeArray
}
