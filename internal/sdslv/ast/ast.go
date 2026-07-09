package ast

import "github.com/yuechen-li-dev/oct/internal/source"

type Module struct {
	Source    source.File
	Namespace string
	Uses      []string
	Decls     []Decl
}

type Decl interface{ declNode() }

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

type StreamDecl struct {
	Name   string
	Fields []Field
}

func (StreamDecl) declNode() {}

type EnumDecl struct {
	Name     string
	Variants []string
}

func (EnumDecl) declNode() {}

type ShaderDecl struct {
	Name               string
	ResourceBundleName string
	Resources          []ResourceDecl
	Workgroups         []WorkgroupDecl
	Methods            []FunctionDecl
}

func (ShaderDecl) declNode() {}

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
	X int
	Y int
	Z int
}

type ResourceDecl struct {
	Name   string
	Access string
	Type   TypeRef
}

type WorkgroupDecl struct {
	Name string
	Type TypeRef
}

type Field struct {
	Name   string
	Access string
	Type   TypeRef
}

type Parameter struct {
	Name string
	Type TypeRef
}

type TypeRef struct {
	Name         string
	Args         []TypeRef
	ArraySize    int
	HasArraySize bool
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

type AssignStmt struct {
	Target Expr
	Value  Expr
}

func (AssignStmt) stmtNode() {}

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

type ForStmt struct {
	Name  string
	Start Expr
	End   Expr
	Step  Expr
	Body  Block
}

func (ForStmt) stmtNode() {}

type Expr interface{ exprNode() }

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
	Target Expr
	Index  Expr
}

func (IndexExpr) exprNode() {}

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

type FieldUpdate struct {
	Name  string
	Value Expr
}
