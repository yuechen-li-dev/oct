package conceptvulkan

import (
	"fmt"
	"strings"
)

const EVT1CompilerID = "concept-vulkan-evt1-m1a"

type EVT1TypeKind string

const (
	EVT1TypeBuiltin EVT1TypeKind = "builtin"
	EVT1TypeEnum    EVT1TypeKind = "enum"
	EVT1TypePointer EVT1TypeKind = "pointer"
)

type EVT1Type struct {
	Qualifier string       `json:"qualifier,omitempty"`
	Name      string       `json:"name"`
	Kind      EVT1TypeKind `json:"kind"`
	PointerTo *EVT1Type    `json:"pointer_to,omitempty"`
	Span      Span         `json:"span"`
}

func (t EVT1Type) String() string {
	if t.PointerTo != nil {
		return t.PointerTo.String() + "*"
	}
	if t.Qualifier != "" {
		return t.Qualifier + " " + t.Name
	}
	return t.Name
}

func (t EVT1Type) Equal(other EVT1Type) bool {
	if t.Qualifier != other.Qualifier || t.Name != other.Name || t.Kind != other.Kind {
		return false
	}
	if t.PointerTo == nil || other.PointerTo == nil {
		return t.PointerTo == nil && other.PointerTo == nil
	}
	return t.PointerTo.Equal(*other.PointerTo)
}

type EVT1Field struct {
	Type EVT1Type `json:"type"`
	Name string   `json:"name"`
	Span Span     `json:"span"`
}

type EVT1VariantDecl struct {
	Name    string      `json:"name"`
	Payload []EVT1Field `json:"payload,omitempty"`
	Tag     int         `json:"tag"`
	Span    Span        `json:"span"`
}

type EVT1EnumDecl struct {
	Name     string            `json:"name"`
	Variants []EVT1VariantDecl `json:"variants"`
	Span     Span              `json:"span"`
}

type EVT1Param struct {
	Type EVT1Type `json:"type"`
	Name string   `json:"name"`
	Span Span     `json:"span"`
}

type EVT1FunctionDecl struct {
	Name       string      `json:"name"`
	ReturnType EVT1Type    `json:"return_type"`
	Params     []EVT1Param `json:"params,omitempty"`
	Body       *EVT1Block  `json:"body,omitempty"`
	Span       Span        `json:"span"`
}

type EVT1Module struct {
	Path      string             `json:"path"`
	Imports   []string           `json:"imports,omitempty"`
	Enums     []EVT1EnumDecl     `json:"enums,omitempty"`
	Functions []EVT1FunctionDecl `json:"functions,omitempty"`
}

type EVT1Block struct {
	Statements []EVT1Statement `json:"statements,omitempty"`
	Span       Span            `json:"span"`
}

func (*EVT1Block) evt1Statement()      {}
func (s *EVT1Block) statementSpan() Span { return s.Span }

type EVT1Statement interface {
	evt1Statement()
	statementSpan() Span
}

type EVT1VarDecl struct {
	Type  EVT1Type     `json:"type"`
	Name  string       `json:"name"`
	Value EVT1Expr     `json:"value"`
	Span  Span         `json:"span"`
	Typed *EVT1TypedID `json:"typed,omitempty"`
}

func (*EVT1VarDecl) evt1Statement()      {}
func (s *EVT1VarDecl) statementSpan() Span { return s.Span }

type EVT1ReturnStmt struct {
	Value EVT1Expr `json:"value,omitempty"`
	Span  Span     `json:"span"`
}

func (*EVT1ReturnStmt) evt1Statement()      {}
func (s *EVT1ReturnStmt) statementSpan() Span { return s.Span }

type EVT1ExprStmt struct {
	Value EVT1Expr `json:"value"`
	Span  Span     `json:"span"`
}

func (*EVT1ExprStmt) evt1Statement()      {}
func (s *EVT1ExprStmt) statementSpan() Span { return s.Span }

type EVT1MatchStmt struct {
	Subject EVT1Expr          `json:"subject"`
	Arms    []EVT1StatementArm `json:"arms"`
	Span    Span              `json:"span"`
}

func (*EVT1MatchStmt) evt1Statement()      {}
func (s *EVT1MatchStmt) statementSpan() Span { return s.Span }

type EVT1StatementArm struct {
	Pattern EVT1Pattern `json:"pattern"`
	Block   EVT1Block   `json:"block"`
	Span    Span        `json:"span"`
}

type EVT1Pattern struct {
	EnumName    string   `json:"enum_name"`
	VariantName string   `json:"variant_name"`
	Bindings    []string `json:"bindings,omitempty"`
	Span        Span     `json:"span"`
}

type EVT1Expr interface {
	evt1Expr()
	exprSpan() Span
}

type EVT1NameExpr struct {
	Name string `json:"name"`
	Span Span   `json:"span"`
}

func (*EVT1NameExpr) evt1Expr()     {}
func (e *EVT1NameExpr) exprSpan() Span { return e.Span }

type EVT1IntLiteral struct {
	Value int `json:"value"`
	Span  Span `json:"span"`
}

func (*EVT1IntLiteral) evt1Expr()     {}
func (e *EVT1IntLiteral) exprSpan() Span { return e.Span }

type EVT1FieldExpr struct {
	Receiver EVT1Expr `json:"receiver"`
	Field    string   `json:"field"`
	Span     Span     `json:"span"`
}

func (*EVT1FieldExpr) evt1Expr()     {}
func (e *EVT1FieldExpr) exprSpan() Span { return e.Span }

type EVT1CallExpr struct {
	Callee string     `json:"callee"`
	Args   []EVT1Expr `json:"args,omitempty"`
	Span   Span       `json:"span"`
}

func (*EVT1CallExpr) evt1Expr()     {}
func (e *EVT1CallExpr) exprSpan() Span { return e.Span }

type EVT1BinaryExpr struct {
	Op    string   `json:"op"`
	Left  EVT1Expr `json:"left"`
	Right EVT1Expr `json:"right"`
	Span  Span     `json:"span"`
}

func (*EVT1BinaryExpr) evt1Expr()     {}
func (e *EVT1BinaryExpr) exprSpan() Span { return e.Span }

type EVT1ConstructExpr struct {
	EnumName    string     `json:"enum_name"`
	VariantName string     `json:"variant_name"`
	Args        []EVT1Expr `json:"args,omitempty"`
	Span        Span       `json:"span"`
}

func (*EVT1ConstructExpr) evt1Expr()     {}
func (e *EVT1ConstructExpr) exprSpan() Span { return e.Span }

type EVT1MatchExpr struct {
	Subject EVT1Expr      `json:"subject"`
	Arms    []EVT1ExprArm `json:"arms"`
	Span    Span          `json:"span"`
}

func (*EVT1MatchExpr) evt1Expr()     {}
func (e *EVT1MatchExpr) exprSpan() Span { return e.Span }

type EVT1ExprArm struct {
	Pattern EVT1Pattern `json:"pattern"`
	Value   EVT1Expr    `json:"value"`
	Span    Span        `json:"span"`
}

type EVT1TypedID struct {
	Name string   `json:"name"`
	Type EVT1Type `json:"type"`
}

type EVT1TypedExpr struct {
	Type EVT1Type `json:"type"`
	Kind string   `json:"kind"`
	Span Span     `json:"span"`
}

type EVT1MIR struct {
	Schema    string             `json:"schema"`
	Module    string             `json:"module"`
	Enums     []EVT1MIREnum      `json:"enums"`
	Functions []EVT1MIRFunction  `json:"functions"`
}

type EVT1MIREnum struct {
	Name       string             `json:"name"`
	CName      string             `json:"c_name"`
	SourceSpan Span               `json:"source_span"`
	Variants   []EVT1MIRVariant   `json:"variants"`
}

type EVT1MIRVariant struct {
	Name       string        `json:"name"`
	TagName    string        `json:"tag_name"`
	Payload    []EVT1MIRName `json:"payload,omitempty"`
	Tag        int           `json:"tag"`
	SourceSpan Span          `json:"source_span"`
}

type EVT1MIRName struct {
	Name string   `json:"name"`
	Type EVT1Type `json:"type"`
}

type EVT1MIRFunction struct {
	Name       string            `json:"name"`
	ReturnType EVT1Type          `json:"return_type"`
	Params     []EVT1MIRName     `json:"params,omitempty"`
	Operations []EVT1MIROperation `json:"operations"`
	SourceSpan Span              `json:"source_span"`
}

type EVT1MIROperation struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	Type       string   `json:"type,omitempty"`
	Detail     string   `json:"detail,omitempty"`
	SourceSpan Span     `json:"source_span"`
}

type evt1Env struct {
	enums             map[string]EVT1EnumDecl
	functions         map[string]EVT1FunctionDecl
	recordFields      map[string]map[string]EVT1Type
	escapedArmBinding map[string]Span
}

func newEVT1Env() *evt1Env {
	intType := EVT1Type{Name: "int", Kind: EVT1TypeBuiltin}
	return &evt1Env{
		enums:     map[string]EVT1EnumDecl{},
		functions: map[string]EVT1FunctionDecl{},
		recordFields: map[string]map[string]EVT1Type{
			"VulkanError": {
				"Code": intType,
			},
		},
		escapedArmBinding: map[string]Span{},
	}
}

func evt1BuiltinType(name string, span Span) (EVT1Type, bool) {
	switch name {
	case "int", "void", "PipelineLayout", "Pipeline", "VulkanError":
		return EVT1Type{Name: name, Kind: EVT1TypeBuiltin, Span: span}, true
	default:
		return EVT1Type{}, false
	}
}

func evt1Diagnostic(code, message string, span Span) error {
	return Diagnostic{Code: code, Message: message, Span: span}
}

func evt1MissingVariants(enumDecl EVT1EnumDecl, seen map[string]bool) []string {
	var missing []string
	for _, variant := range enumDecl.Variants {
		key := enumDecl.Name + "::" + variant.Name
		if !seen[key] {
			missing = append(missing, key)
		}
	}
	return missing
}

func evt1LookupVariant(enumDecl EVT1EnumDecl, name string) (EVT1VariantDecl, bool) {
	for _, variant := range enumDecl.Variants {
		if variant.Name == name {
			return variant, true
		}
	}
	return EVT1VariantDecl{}, false
}

func evt1CName(name string) string {
	var out []byte
	lastLower := false
	for i := 0; i < len(name); i++ {
		c := name[i]
		isUpper := c >= 'A' && c <= 'Z'
		isLower := c >= 'a' && c <= 'z'
		if i > 0 && isUpper && lastLower {
			out = append(out, '_')
		}
		if isUpper {
			out = append(out, c+'a'-'A')
		} else {
			out = append(out, c)
		}
		lastLower = isLower
	}
	return "concept_vulkan_" + string(out)
}

func evt1TagName(enumName, variantName string) string {
	return strings.ToUpper(evt1CName(enumName) + "_" + evt1CName(variantName)[len("concept_vulkan_"):])
}

func evt1PayloadFieldName(variantName string) string {
	return evt1CName(variantName)[len("concept_vulkan_"):]
}

func evt1Require(cond bool, code, message string, span Span) error {
	if cond {
		return nil
	}
	return evt1Diagnostic(code, message, span)
}

func evt1TypeNameOrDie(t EVT1Type) string {
	return t.String()
}

func evt1Typed(kind string, t EVT1Type, span Span) *EVT1TypedExpr {
	return &EVT1TypedExpr{Kind: kind, Type: t, Span: span}
}

func evt1Unexpected(expr EVT1Expr) string {
	return fmt.Sprintf("%T", expr)
}
