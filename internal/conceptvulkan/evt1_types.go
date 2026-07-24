package conceptvulkan

import (
	"fmt"
	"strings"
)

const EVT1CompilerID = "concept-vulkan-evt1-m1b-b"

type EVT1TypeKind string

const (
	EVT1TypeBuiltin      EVT1TypeKind = "builtin"
	EVT1TypeEnum         EVT1TypeKind = "enum"
	EVT1TypeStruct       EVT1TypeKind = "struct"
	EVT1TypePointer      EVT1TypeKind = "pointer"
	EVT1TypeConceptParam EVT1TypeKind = "concept_param"
	EVT1TypeApplied      EVT1TypeKind = "applied"
)

type EVT1Type struct {
	Name      string       `json:"name"`
	Kind      EVT1TypeKind `json:"kind"`
	Ownership string       `json:"ownership,omitempty"`
	Const     bool         `json:"const,omitempty"`
	Imported  bool         `json:"imported,omitempty"`
	Unsafe    bool         `json:"unsafe,omitempty"`
	PointerTo *EVT1Type    `json:"pointer_to,omitempty"`
	TypeArgs  []EVT1Type   `json:"type_args,omitempty"`
	Span      Span         `json:"span"`
}

func (t EVT1Type) String() string {
	var parts []string
	if t.Unsafe {
		parts = append(parts, "unsafe")
	}
	if t.Imported {
		parts = append(parts, "imported")
	}
	if t.Ownership != "" {
		parts = append(parts, t.Ownership)
	}
	if t.Const {
		parts = append(parts, "const")
	}
	base := t.Name
	if t.PointerTo != nil {
		base = t.PointerTo.String() + "*"
	} else if len(t.TypeArgs) > 0 {
		var args []string
		for _, arg := range t.TypeArgs {
			args = append(args, arg.String())
		}
		base = base + "<" + strings.Join(args, ", ") + ">"
	}
	parts = append(parts, base)
	return strings.Join(parts, " ")
}

func (t EVT1Type) Equal(other EVT1Type) bool {
	if t.Name != other.Name ||
		t.Kind != other.Kind ||
		t.Ownership != other.Ownership ||
		t.Const != other.Const ||
		t.Imported != other.Imported ||
		t.Unsafe != other.Unsafe ||
		len(t.TypeArgs) != len(other.TypeArgs) {
		return false
	}
	for i := range t.TypeArgs {
		if !t.TypeArgs[i].Equal(other.TypeArgs[i]) {
			return false
		}
	}
	if t.PointerTo == nil || other.PointerTo == nil {
		return t.PointerTo == nil && other.PointerTo == nil
	}
	return t.PointerTo.Equal(*other.PointerTo)
}

func (t EVT1Type) SameValueType(other EVT1Type) bool {
	a := t.valueType()
	b := other.valueType()
	return a.Equal(b)
}

func (t EVT1Type) valueType() EVT1Type {
	out := t
	out.Ownership = ""
	out.Const = false
	out.Imported = false
	out.Unsafe = false
	return out
}

func (t EVT1Type) isBorrow() bool {
	return t.Ownership == "borrow"
}

func (t EVT1Type) isOwned() bool {
	return t.Ownership == "owned"
}

func (t EVT1Type) isBorrowLike() bool {
	return t.isBorrow() || t.PointerTo != nil
}

func (t EVT1Type) borrowBase() EVT1Type {
	if t.PointerTo != nil {
		base := *t.PointerTo
		base.Const = base.Const || t.Const
		return base.valueType()
	}
	return t.valueType()
}

type EVT1Field struct {
	Type EVT1Type `json:"type"`
	Name string   `json:"name"`
	Span Span     `json:"span"`
}

type EVT1StructDecl struct {
	Name      string      `json:"name"`
	Immovable bool        `json:"immovable"`
	Fields    []EVT1Field `json:"fields"`
	Span      Span        `json:"span"`
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

type EVT1ConceptRequirement interface {
	evt1ConceptRequirement()
	requirementSpan() Span
}

type EVT1OperationRequirement struct {
	ReturnType EVT1Type    `json:"return_type"`
	Name       string      `json:"name"`
	Params     []EVT1Param `json:"params,omitempty"`
	Span       Span        `json:"span"`
}

func (*EVT1OperationRequirement) evt1ConceptRequirement() {}
func (r *EVT1OperationRequirement) requirementSpan() Span { return r.Span }

type EVT1PrerequisiteRequirement struct {
	ConceptName string   `json:"concept_name"`
	TypeArg     EVT1Type `json:"type_arg"`
	Span        Span     `json:"span"`
}

func (*EVT1PrerequisiteRequirement) evt1ConceptRequirement() {}
func (r *EVT1PrerequisiteRequirement) requirementSpan() Span { return r.Span }

type EVT1ConceptDecl struct {
	Name         string                   `json:"name"`
	TypeParam    string                   `json:"type_param"`
	Requirements []EVT1ConceptRequirement `json:"requirements,omitempty"`
	Span         Span                     `json:"span"`
}

type EVT1ConceptAssertion struct {
	ConceptName  string   `json:"concept_name"`
	ConcreteType EVT1Type `json:"concrete_type"`
	Span         Span     `json:"span"`
}

type EVT1TemplateConstraint struct {
	ConceptName string   `json:"concept_name"`
	TypeArg     EVT1Type `json:"type_arg"`
	Span        Span     `json:"span"`
}

type EVT1TemplateDecl struct {
	Name          string                 `json:"name"`
	TypeParam     string                 `json:"type_param"`
	TypeParamSpan Span                   `json:"type_param_span"`
	Constraint    EVT1TemplateConstraint `json:"constraint"`
	ReturnType    EVT1Type               `json:"return_type"`
	Params        []EVT1Param            `json:"params,omitempty"`
	Body          *EVT1Block             `json:"body,omitempty"`
	Span          Span                   `json:"span"`
}

type EVT1FunctionDecl struct {
	Name       string      `json:"name"`
	ReturnType EVT1Type    `json:"return_type"`
	Params     []EVT1Param `json:"params,omitempty"`
	Body       *EVT1Block  `json:"body,omitempty"`
	Span       Span        `json:"span"`
}

type EVT1Module struct {
	Path       string                 `json:"path"`
	Imports    []string               `json:"imports,omitempty"`
	Structs    []EVT1StructDecl       `json:"structs,omitempty"`
	Enums      []EVT1EnumDecl         `json:"enums,omitempty"`
	Concepts   []EVT1ConceptDecl      `json:"concepts,omitempty"`
	Assertions []EVT1ConceptAssertion `json:"assertions,omitempty"`
	Templates  []EVT1TemplateDecl     `json:"templates,omitempty"`
	Functions  []EVT1FunctionDecl     `json:"functions,omitempty"`
}

type EVT1Block struct {
	Statements []EVT1Statement `json:"statements,omitempty"`
	Span       Span            `json:"span"`
}

func (*EVT1Block) evt1Statement()        {}
func (s *EVT1Block) statementSpan() Span { return s.Span }

type EVT1Statement interface {
	evt1Statement()
	statementSpan() Span
}

type EVT1VarDecl struct {
	Type  EVT1Type `json:"type"`
	Name  string   `json:"name"`
	Value EVT1Expr `json:"value"`
	Span  Span     `json:"span"`
}

func (*EVT1VarDecl) evt1Statement()        {}
func (s *EVT1VarDecl) statementSpan() Span { return s.Span }

type EVT1AssignStmt struct {
	Target EVT1Expr `json:"target"`
	Value  EVT1Expr `json:"value"`
	Span   Span     `json:"span"`
}

func (*EVT1AssignStmt) evt1Statement()        {}
func (s *EVT1AssignStmt) statementSpan() Span { return s.Span }

type EVT1ReturnStmt struct {
	Value EVT1Expr `json:"value,omitempty"`
	Span  Span     `json:"span"`
}

func (*EVT1ReturnStmt) evt1Statement()        {}
func (s *EVT1ReturnStmt) statementSpan() Span { return s.Span }

type EVT1ExprStmt struct {
	Value EVT1Expr `json:"value"`
	Span  Span     `json:"span"`
}

func (*EVT1ExprStmt) evt1Statement()        {}
func (s *EVT1ExprStmt) statementSpan() Span { return s.Span }

type EVT1MatchStmt struct {
	Subject EVT1Expr           `json:"subject"`
	Arms    []EVT1StatementArm `json:"arms"`
	Span    Span               `json:"span"`
}

func (*EVT1MatchStmt) evt1Statement()        {}
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
	Value int  `json:"value"`
	Span  Span `json:"span"`
}

func (*EVT1IntLiteral) evt1Expr()     {}
func (e *EVT1IntLiteral) exprSpan() Span { return e.Span }

type EVT1BoolLiteral struct {
	Value bool `json:"value"`
	Span  Span `json:"span"`
}

func (*EVT1BoolLiteral) evt1Expr()     {}
func (e *EVT1BoolLiteral) exprSpan() Span { return e.Span }

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

type EVT1TemplateCallExpr struct {
	Callee  string     `json:"callee"`
	TypeArg EVT1Type   `json:"type_arg"`
	Args    []EVT1Expr `json:"args,omitempty"`
	Span    Span       `json:"span"`
}

func (*EVT1TemplateCallExpr) evt1Expr()     {}
func (e *EVT1TemplateCallExpr) exprSpan() Span { return e.Span }

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

type EVT1StructConstructExpr struct {
	StructName string     `json:"struct_name"`
	Args       []EVT1Expr `json:"args,omitempty"`
	Span       Span       `json:"span"`
}

func (*EVT1StructConstructExpr) evt1Expr()     {}
func (e *EVT1StructConstructExpr) exprSpan() Span { return e.Span }

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

type EVT1MIR struct {
	Schema     string                `json:"schema"`
	Module     string                `json:"module"`
	Structs    []EVT1MIRStruct       `json:"structs,omitempty"`
	Enums      []EVT1MIREnum         `json:"enums,omitempty"`
	Concepts   []EVT1MIRConcept      `json:"concepts,omitempty"`
	Assertions []EVT1MIRAssertion    `json:"assertions,omitempty"`
	Templates  []EVT1MIRTemplate     `json:"templates,omitempty"`
	Instances  []EVT1MIRInstance     `json:"instances,omitempty"`
	Functions  []EVT1MIRFunction     `json:"functions"`
}

type EVT1MIRStruct struct {
	Name       string         `json:"name"`
	CName      string         `json:"c_name"`
	Immovable  bool           `json:"immovable"`
	Copyable   bool           `json:"copyable"`
	Fields     []EVT1MIRName  `json:"fields,omitempty"`
	SourceSpan Span           `json:"source_span"`
}

type EVT1MIREnum struct {
	Name       string           `json:"name"`
	CName      string           `json:"c_name"`
	SourceSpan Span             `json:"source_span"`
	Variants   []EVT1MIRVariant `json:"variants"`
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

type EVT1MIRConcept struct {
	Name         string                      `json:"name"`
	TypeParam    string                      `json:"type_param"`
	Requirements []EVT1MIRConceptRequirement `json:"requirements,omitempty"`
	SourceSpan   Span                        `json:"source_span"`
}

type EVT1MIRConceptRequirement struct {
	Kind       string        `json:"kind"`
	Name       string        `json:"name"`
	ReturnType *EVT1Type     `json:"return_type,omitempty"`
	Params     []EVT1MIRName `json:"params,omitempty"`
	Detail     string        `json:"detail,omitempty"`
	SourceSpan Span          `json:"source_span"`
}

type EVT1MIRAssertion struct {
	ConceptName  string   `json:"concept_name"`
	ConcreteType EVT1Type `json:"concrete_type"`
	Satisfied    bool     `json:"satisfied"`
	SourceSpan   Span     `json:"source_span"`
}

type EVT1MIRTemplateConstraint struct {
	ConceptName string   `json:"concept_name"`
	TypeParam   string   `json:"type_param"`
	SourceSpan  Span     `json:"source_span"`
}

type EVT1MIRClosureEntry struct {
	ConceptName string   `json:"concept_name"`
	Path        []string `json:"path,omitempty"`
}

type EVT1MIRRequirementBinding struct {
	RequirementID string   `json:"requirement_id"`
	ConceptName   string   `json:"concept_name"`
	Name          string   `json:"name"`
	Signature     string   `json:"signature"`
	Path          []string `json:"path,omitempty"`
}

type EVT1MIRTemplate struct {
	Name         string                    `json:"name"`
	TypeParam    string                    `json:"type_param"`
	Constraint   EVT1MIRTemplateConstraint `json:"constraint"`
	Closure      []EVT1MIRClosureEntry     `json:"closure,omitempty"`
	Requirements []EVT1MIRRequirementBinding `json:"requirements,omitempty"`
	ReturnType   EVT1Type                  `json:"return_type"`
	Params       []EVT1MIRName             `json:"params,omitempty"`
	Operations   []EVT1MIROperation        `json:"operations,omitempty"`
	SourceSpan   Span                      `json:"source_span"`
}

type EVT1MIRInstance struct {
	ID                  string                  `json:"id"`
	TemplateName        string                  `json:"template_name"`
	ConcreteType        EVT1Type                `json:"concrete_type"`
	GeneratedSymbol     string                  `json:"generated_symbol"`
	ConstraintConcept   string                  `json:"constraint_concept"`
	Closure             []EVT1MIRClosureEntry   `json:"closure,omitempty"`
	RequirementBindings []EVT1MIRRequirementBinding `json:"requirement_bindings,omitempty"`
	ReturnType          EVT1Type                `json:"return_type"`
	Params              []EVT1MIRName           `json:"params,omitempty"`
	Operations          []EVT1MIROperation      `json:"operations,omitempty"`
	InvocationSpans     []Span                  `json:"invocation_spans,omitempty"`
	SourceSpan          Span                    `json:"source_span"`
}

type EVT1MIRFunction struct {
	Name       string             `json:"name"`
	ReturnType EVT1Type           `json:"return_type"`
	Params     []EVT1MIRName      `json:"params,omitempty"`
	Operations []EVT1MIROperation `json:"operations"`
	SourceSpan Span               `json:"source_span"`
}

type EVT1MIROperation struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Type       string `json:"type,omitempty"`
	Detail     string `json:"detail,omitempty"`
	SourceSpan Span   `json:"source_span"`
}

type evt1Env struct {
	enums             map[string]EVT1EnumDecl
	structs           map[string]EVT1StructDecl
	functions         map[string][]EVT1FunctionDecl
	templates         map[string]EVT1TemplateDecl
	concepts          map[string]EVT1ConceptDecl
	fieldSets         map[string]map[string]EVT1Type
	escapedArmBinding map[string]Span
	copyableCache     map[string]bool
	templateInfos     map[string]*evt1TemplateInfo
	templateInstances map[string]*evt1TemplateInstance
}

func newEVT1Env() *evt1Env {
	intType := EVT1Type{Name: "int", Kind: EVT1TypeBuiltin}
	return &evt1Env{
		enums:     map[string]EVT1EnumDecl{},
		structs:   map[string]EVT1StructDecl{},
		functions: map[string][]EVT1FunctionDecl{},
		templates: map[string]EVT1TemplateDecl{},
		concepts:  map[string]EVT1ConceptDecl{},
		fieldSets: map[string]map[string]EVT1Type{
			"VulkanError": {
				"Code": intType,
			},
		},
		escapedArmBinding: map[string]Span{},
		copyableCache:     map[string]bool{},
		templateInfos:     map[string]*evt1TemplateInfo{},
		templateInstances: map[string]*evt1TemplateInstance{},
	}
}

type evt1TemplateRequirement struct {
	ID        string
	Path      []string
	Concept   string
	Operation EVT1OperationRequirement
}

type evt1TemplateClosureEntry struct {
	Concept string
	Path    []string
}

type evt1TemplateCallBinding struct {
	CallSpan     Span
	Requirement  evt1TemplateRequirement
}

type evt1TemplateInfo struct {
	Decl         EVT1TemplateDecl
	Closure      []evt1TemplateClosureEntry
	Requirements []evt1TemplateRequirement
	CallBindings map[string]evt1TemplateCallBinding
}

type evt1InstanceRequirementBinding struct {
	Requirement evt1TemplateRequirement
	Function    EVT1FunctionDecl
}

type evt1TemplateInstance struct {
	Key                 string
	TemplateName        string
	ConcreteType        EVT1Type
	TypeIdentity        string
	GeneratedSymbol     string
	ConstraintConcept   string
	Closure             []evt1TemplateClosureEntry
	RequirementBindings []evt1InstanceRequirementBinding
	Function            EVT1FunctionDecl
	InvocationSpans     []Span
	SourceSpan          Span
}

func evt1BuiltinType(name string, span Span) (EVT1Type, bool) {
	switch name {
	case "int", "void", "bool", "uint64", "PipelineLayout", "Pipeline", "VulkanError", "VkBuffer", "VkCommandPool":
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

func evt1Unexpected(expr EVT1Expr) string {
	return fmt.Sprintf("%T", expr)
}
