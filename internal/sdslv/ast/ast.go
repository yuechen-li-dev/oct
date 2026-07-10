package ast

import "github.com/yuechen-li-dev/oct/internal/source"

type Module struct {
	Source    source.File
	Namespace string
	Uses      []string
	Decls     []Decl
}

type Decl interface{ declNode() }

type TemplateParam struct {
	Name        string
	ConceptName string
}

type AttributePlacement string

const (
	AttributePlacementField AttributePlacement = "field"
	AttributePlacementStmt  AttributePlacement = "stmt"
	AttributePlacementExpr  AttributePlacement = "expr"
)

type Attribute struct {
	Name      string
	Arguments []Expr
	Placement AttributePlacement
	Line      int
	Column    int
}

type TypeAliasDecl struct {
	Name string
	Type TypeRef
}

func (TypeAliasDecl) declNode() {}

type RecordDecl struct {
	Name   string
	Fields []Field
}

func (RecordDecl) declNode() {}

type BoardDecl struct {
	Name   string
	Fields []Field
}

func (BoardDecl) declNode() {}

type StreamDecl struct {
	Name   string
	Fields []Field
}

func (StreamDecl) declNode() {}

type ConceptDecl struct {
	Name         string
	Members      []ConceptMember
	Requirements []RequireStmt
}

func (ConceptDecl) declNode() {}

type ConceptMember interface{ conceptMemberNode() }

type ConceptField struct {
	Name         string
	Type         TypeRef
	DefaultValue Expr
}

func (ConceptField) conceptMemberNode() {}

type ConceptGroup struct {
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
	Path  string
	Value Expr
	Style ConfigAssignmentStyle
}

type ConfigDecl struct {
	Name         string
	ConceptName  string
	Fields       []ConfigField
	Requirements []RequireStmt
}

func (ConfigDecl) declNode() {}

type EnumDecl struct {
	Name     string
	Variants []EnumVariant
}

func (EnumDecl) declNode() {}

type EnumVariant struct {
	Name    string
	Fields  []Field
	Payload bool
}

type ShaderDecl struct {
	Name               string
	Template           *TemplateParam
	ResourceBundleName string
	Resources          []ResourceDecl
	Workgroups         []WorkgroupDecl
	StaticAsserts      []StaticAssertStmt
	Methods            []FunctionDecl
	SpecializedConfig  map[string]uint32
}

func (ShaderDecl) declNode() {}

type CompileDecl struct {
	ShaderName string
	ConfigName string
	AliasName  string
}

func (CompileDecl) declNode() {}

type FunctionDecl struct {
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
}

func (UnsupportedDecl) declNode() {}

type NumThreads struct {
	X Expr
	Y Expr
	Z Expr
}

type ResourceDecl struct {
	Name       string
	Access     string
	Type       TypeRef
	Attributes []Attribute
}

type WorkgroupDecl struct {
	Name string
	Type TypeRef
}

type Field struct {
	Name       string
	Access     string
	Type       TypeRef
	Attributes []Attribute
}

type Parameter struct {
	Name string
	Type TypeRef
}

type TypeRef struct {
	Name         string
	Args         []TypeRef
	ArraySize    Expr
	HasArraySize bool
	TileRows     Expr
	TileCols     Expr
	HasTileShape bool
	Access       string
	ZeroAllowed  bool
}

func (t TypeRef) String() string {
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
	Statements []Stmt
}

type Stmt interface{ stmtNode() }

type LetStmt struct {
	Name  string
	Type  TypeRef
	Value Expr
}

func (LetStmt) stmtNode() {}

type ComptimeLetStmt struct {
	Name  string
	Type  TypeRef
	Value Expr
}

func (ComptimeLetStmt) stmtNode() {}

type AssignStmt struct {
	Target Expr
	Value  Expr
}

func (AssignStmt) stmtNode() {}

type GuardedWriteStmt struct {
	Target    Expr
	Value     Expr
	Condition Expr
}

func (GuardedWriteStmt) stmtNode() {}

type ReturnStmt struct {
	Value Expr
}

func (ReturnStmt) stmtNode() {}

type ExprStmt struct {
	Value Expr
}

func (ExprStmt) stmtNode() {}

type IfStmt struct {
	Condition Expr
	ThenBody  Block
	ElseBody  *Block
}

func (IfStmt) stmtNode() {}

type GuardWhenStmt struct {
	Cases    []GuardWhenCase
	ElseBody *Block
}

func (GuardWhenStmt) stmtNode() {}

type GuardWhenCase struct {
	Condition Expr
	Body      Block
}

type FlowStmt struct {
	Name   string
	Boards []FlowBoardDecl
	States []StateBlock
}

func (FlowStmt) stmtNode() {}

type FlowBoardDecl struct {
	Name        string
	Type        TypeRef
	Initializer Expr
}

type StateBlock struct {
	Name string
	Body Block
}

type ComptimeIfStmt struct {
	Condition Expr
	ThenBody  Block
	ElseBody  *Block
}

func (ComptimeIfStmt) stmtNode() {}

type ComptimeMatchStmt struct {
	Subject Expr
	Arms    []ComptimeMatchArm
}

func (ComptimeMatchStmt) stmtNode() {}

type ComptimeMatchArm struct {
	Pattern Expr
	IsElse  bool
	Body    Block
}

type ComptimeWhenUtilityStmt struct {
	Cases    []ComptimeWhenUtilityCase
	ElseBody *Block
}

func (ComptimeWhenUtilityStmt) stmtNode() {}

type ComptimeWhenUtilityCase struct {
	Label     string
	Condition Expr
	Score     Expr
	Body      Block
}

type ComptimeForStmt struct {
	Name  string
	Start Expr
	End   Expr
	Body  Block
}

func (ComptimeForStmt) stmtNode() {}

type ForStmt struct {
	Attributes []Attribute
	Name       string
	Start      Expr
	End        Expr
	Step       Expr
	Body       Block
}

func (ForStmt) stmtNode() {}

type RequireStmt struct {
	Expr Expr
	Text string
}

type StaticAssertStmt struct {
	Expr Expr
	Text string
}

func (StaticAssertStmt) stmtNode() {}

type Expr interface{ exprNode() }

type ReductionOp string

const (
	ReductionSum     ReductionOp = "sum"
	ReductionProduct ReductionOp = "product"
	ReductionMax     ReductionOp = "max"
	ReductionMin     ReductionOp = "min"
)

type IntegerLiteral struct{ Value string }

func (IntegerLiteral) exprNode() {}

type FloatLiteral struct{ Value string }

func (FloatLiteral) exprNode() {}

type BoolLiteral struct{ Value bool }

func (BoolLiteral) exprNode() {}

type StringLiteral struct{ Value string }

func (StringLiteral) exprNode() {}

type IdentifierExpr struct{ Name string }

func (IdentifierExpr) exprNode() {}

type FieldAccessExpr struct {
	Target Expr
	Field  string
}

func (FieldAccessExpr) exprNode() {}

type IndexExpr struct {
	Target    Expr
	Index     Expr
	Index2    Expr
	HasSecond bool
}

func (IndexExpr) exprNode() {}

type GuardedReadExpr struct {
	Target    Expr
	Condition Expr
	Fallback  Expr
}

func (GuardedReadExpr) exprNode() {}

type CallExpr struct {
	Callee    Expr
	Arguments []Expr
}

func (CallExpr) exprNode() {}

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

type ParenExpr struct{ Inner Expr }

func (ParenExpr) exprNode() {}

type WhenUtilityExpr struct {
	Cases []UtilityCase
	Else  Expr
}

func (WhenUtilityExpr) exprNode() {}

type UtilityCase struct {
	Value     Expr
	Condition Expr
	Score     Expr
}

type WithExpr struct {
	Base    Expr
	Updates []FieldUpdate
}

func (WithExpr) exprNode() {}

type ReductionExpr struct {
	Attributes []Attribute
	Op         ReductionOp
	Name       string
	Start      Expr
	End        Expr
	Step       Expr
	Body       Expr
}

func (ReductionExpr) exprNode() {}

type FieldUpdate struct {
	Name  string
	Value Expr
}

type EnumConstructExpr struct {
	EnumName    string
	VariantName string
	Fields      []FieldInit
}

func (EnumConstructExpr) exprNode() {}

type BoardLiteralExpr struct {
	TypeName string
	Fields   []FieldInit
}

func (BoardLiteralExpr) exprNode() {}

type FieldInit struct {
	Name  string
	Value Expr
}

type MatchExpr struct {
	Subject Expr
	Arms    []MatchArm
}

func (MatchExpr) exprNode() {}

type MatchArm struct {
	EnumName    string
	VariantName string
	BindingName string
	Value       Expr
}
