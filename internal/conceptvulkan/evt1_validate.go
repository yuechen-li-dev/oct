package conceptvulkan

import (
	"fmt"
	"strings"
)

type evt1Scope struct {
	parent *evt1Scope
	values map[string]evt1ValueBinding
}

type evt1ValueBinding struct {
	t       EVT1Type
	mutable bool
}

type evt1LValue struct {
	t         EVT1Type
	mutable   bool
	wholeValue bool
}

func newEVT1Scope(parent *evt1Scope) *evt1Scope {
	return &evt1Scope{parent: parent, values: map[string]evt1ValueBinding{}}
}

func (s *evt1Scope) declare(name string, binding evt1ValueBinding) {
	s.values[name] = binding
}

func (s *evt1Scope) lookup(name string) (evt1ValueBinding, bool) {
	for scope := s; scope != nil; scope = scope.parent {
		if t, ok := scope.values[name]; ok {
			return t, true
		}
	}
	return evt1ValueBinding{}, false
}

func validateEVT1Module(module EVT1Module) error {
	_, err := analyzeEVT1Module(module)
	return err
}

func analyzeEVT1Module(module EVT1Module) (*evt1Env, error) {
	env := newEVT1Env()
	typeNames := map[string]Span{}
	for _, enumDecl := range module.Enums {
		if _, exists := env.enums[enumDecl.Name]; exists {
			return nil, evt1Diagnostic("CV4101", fmt.Sprintf("duplicate enum declaration %s", enumDecl.Name), enumDecl.Span)
		}
		if other, exists := typeNames[enumDecl.Name]; exists {
			return nil, evt1Diagnostic("CV4101", fmt.Sprintf("duplicate type declaration %s", enumDecl.Name), other)
		}
		typeNames[enumDecl.Name] = enumDecl.Span
		env.enums[enumDecl.Name] = enumDecl
	}
	for _, structDecl := range module.Structs {
		if _, exists := env.structs[structDecl.Name]; exists {
			return nil, evt1Diagnostic("CV4122", fmt.Sprintf("duplicate struct declaration %s", structDecl.Name), structDecl.Span)
		}
		if _, exists := typeNames[structDecl.Name]; exists {
			return nil, evt1Diagnostic("CV4122", fmt.Sprintf("duplicate type declaration %s", structDecl.Name), structDecl.Span)
		}
		typeNames[structDecl.Name] = structDecl.Span
		env.structs[structDecl.Name] = structDecl
	}
	for _, conceptDecl := range module.Concepts {
		if _, exists := env.concepts[conceptDecl.Name]; exists {
			return nil, evt1Diagnostic("CV4146", fmt.Sprintf("duplicate concept declaration %s", conceptDecl.Name), conceptDecl.Span)
		}
		env.concepts[conceptDecl.Name] = conceptDecl
	}
	for _, fn := range module.Functions {
		if _, exists := env.functions[fn.Name]; exists {
			return nil, evt1Diagnostic("CV4021", fmt.Sprintf("duplicate function declaration %s", fn.Name), fn.Span)
		}
		env.functions[fn.Name] = fn
	}
	for _, structDecl := range module.Structs {
		fields := map[string]EVT1Type{}
		if len(structDecl.Fields) == 0 {
			return nil, evt1Diagnostic("CV4123", fmt.Sprintf("empty struct %s is not supported", structDecl.Name), structDecl.Span)
		}
		for _, field := range structDecl.Fields {
			if _, exists := fields[field.Name]; exists {
				return nil, evt1Diagnostic("CV4124", fmt.Sprintf("duplicate field %s.%s", structDecl.Name, field.Name), field.Span)
			}
			if err := validateKnownType(env, field.Type, field.Span, "", false); err != nil {
				return nil, err
			}
			fields[field.Name] = evt1CanonicalType(env, field.Type.valueType())
		}
		env.fieldSets[structDecl.Name] = fields
	}
	for i, enumDecl := range module.Enums {
		seen := map[string]bool{}
		for j, variant := range enumDecl.Variants {
			if seen[variant.Name] {
				return nil, evt1Diagnostic("CV4100", fmt.Sprintf("duplicate variant %s::%s", enumDecl.Name, variant.Name), variant.Span)
			}
			seen[variant.Name] = true
			module.Enums[i].Variants[j].Tag = j
			for _, field := range variant.Payload {
				if err := validateKnownType(env, field.Type, field.Span, "", false); err != nil {
					return nil, err
				}
			}
		}
		env.enums[enumDecl.Name] = module.Enums[i]
	}
	if err := validateValueLayoutCycles(env); err != nil {
		return nil, err
	}
	for _, structDecl := range module.Structs {
		for _, field := range structDecl.Fields {
			if !field.Type.isBorrowLike() && evt1IsImmovableValueType(env, field.Type) {
				return nil, evt1Diagnostic("CV4138", fmt.Sprintf("struct %s cannot embed immovable field %s", structDecl.Name, field.Type.String()), field.Span)
			}
		}
	}
	for _, enumDecl := range module.Enums {
		for _, variant := range enumDecl.Variants {
			for _, field := range variant.Payload {
				if evt1IsImmovableValueType(env, field.Type) {
					return nil, evt1Diagnostic("CV4139", fmt.Sprintf("enum payload %s::%s cannot contain immovable type %s", enumDecl.Name, variant.Name, field.Type.String()), field.Span)
				}
				if !evt1TypeCopyable(env, field.Type) {
					return nil, evt1Diagnostic("CV4133", fmt.Sprintf("enum payload %s::%s cannot copy non-copyable type %s", enumDecl.Name, variant.Name, field.Type.String()), field.Span)
				}
			}
		}
	}
	for _, conceptDecl := range module.Concepts {
		for _, req := range conceptDecl.Requirements {
			switch r := req.(type) {
			case *EVT1OperationRequirement:
				if err := validateKnownType(env, r.ReturnType, r.Span, conceptDecl.TypeParam, false); err != nil {
					return nil, err
				}
				for _, param := range r.Params {
					if err := validateKnownType(env, param.Type, param.Span, conceptDecl.TypeParam, false); err != nil {
						return nil, err
					}
				}
			case *EVT1PrerequisiteRequirement:
				if _, ok := env.concepts[r.ConceptName]; !ok {
					return nil, evt1Diagnostic("CV4152", fmt.Sprintf("unknown prerequisite concept %s", r.ConceptName), r.Span)
				}
				if r.TypeArg.Kind != EVT1TypeConceptParam || r.TypeArg.Name != conceptDecl.TypeParam {
					return nil, evt1Diagnostic("CV4152", fmt.Sprintf("prerequisite %s must use the concept parameter %s", r.ConceptName, conceptDecl.TypeParam), r.Span)
				}
			default:
				return nil, evt1Diagnostic("CV4147", "unsupported concept requirement", req.requirementSpan())
			}
		}
	}
	if err := validateConceptCycles(env); err != nil {
		return nil, err
	}
	for _, fn := range module.Functions {
		if err := validateFunctionSignature(env, fn); err != nil {
			return nil, err
		}
		if fn.Body == nil {
			continue
		}
		scope := newEVT1Scope(nil)
		for _, param := range fn.Params {
			scope.declare(param.Name, evt1ValueBinding{
				t:       evt1CanonicalType(env, param.Type),
				mutable: !(param.Type.isBorrowLike() && param.Type.Const),
			})
		}
		collectEscapedArmBindings(fn.Body, env)
		if err := validateBlock(env, scope, fn.ReturnType, *fn.Body); err != nil {
			return nil, err
		}
	}
	for _, assertion := range module.Assertions {
		if _, ok := env.concepts[assertion.ConceptName]; !ok {
			return nil, evt1Diagnostic("CV4151", fmt.Sprintf("unknown concept %s", assertion.ConceptName), assertion.Span)
		}
		if err := validateKnownType(env, assertion.ConcreteType, assertion.Span, "", false); err != nil {
			return nil, err
		}
		if err := checkConceptSatisfaction(env, assertion.ConceptName, assertion.ConcreteType, nil, assertion.Span); err != nil {
			return nil, err
		}
	}
	return env, nil
}

func validateFunctionSignature(env *evt1Env, fn EVT1FunctionDecl) error {
	if err := validateKnownType(env, fn.ReturnType, fn.Span, "", false); err != nil {
		return err
	}
	if err := validateByValueBoundary(env, fn.ReturnType, fn.Span, "return"); err != nil {
		return err
	}
	for _, param := range fn.Params {
		if err := validateKnownType(env, param.Type, param.Span, "", false); err != nil {
			return err
		}
		if err := validateByValueBoundary(env, param.Type, param.Span, "parameter"); err != nil {
			return err
		}
	}
	return nil
}

func validateByValueBoundary(env *evt1Env, t EVT1Type, span Span, context string) error {
	if t.isBorrowLike() {
		return nil
	}
	if evt1IsImmovableValueType(env, t) {
		switch context {
		case "parameter":
			return evt1Diagnostic("CV4136", fmt.Sprintf("immovable type %s cannot be passed by value", t.String()), span)
		case "return":
			return evt1Diagnostic("CV4137", fmt.Sprintf("immovable type %s cannot be returned by value", t.String()), span)
		}
	}
	if !evt1TypeCopyable(env, t) {
		return evt1Diagnostic("CV4133", fmt.Sprintf("non-copyable type %s cannot cross a by-value %s boundary", t.String(), context), span)
	}
	return nil
}

func collectEscapedArmBindings(block *EVT1Block, env *evt1Env) {
	for _, stmt := range block.Statements {
		switch s := stmt.(type) {
		case *EVT1MatchStmt:
			for _, arm := range s.Arms {
				for _, binding := range arm.Pattern.Bindings {
					env.escapedArmBinding[binding] = arm.Pattern.Span
				}
				collectEscapedArmBindings(&arm.Block, env)
			}
		case *EVT1Block:
			collectEscapedArmBindings(s, env)
		}
	}
}

func validateBlock(env *evt1Env, scope *evt1Scope, returnType EVT1Type, block EVT1Block) error {
	local := newEVT1Scope(scope)
	for _, stmt := range block.Statements {
		switch s := stmt.(type) {
		case *EVT1VarDecl:
			if err := validateKnownType(env, s.Type, s.Span, "", false); err != nil {
				return err
			}
			valueType, err := validateExpr(env, local, s.Value)
			if err != nil {
				return err
			}
			if !evt1CanonicalType(env, s.Type).Equal(evt1CanonicalType(env, valueType)) {
				return evt1Diagnostic("CV4106", fmt.Sprintf("constructor or initializer for %s expected %s but got %s", s.Name, s.Type.String(), valueType.String()), s.Value.exprSpan())
			}
			if !evt1CanDirectInitialize(env, s.Type, s.Value) && !evt1TypeCopyable(env, s.Type) {
				if evt1IsImmovableValueType(env, s.Type) {
					return evt1Diagnostic("CV4134", fmt.Sprintf("immovable value %s must be constructed directly in final storage", s.Type.String()), s.Span)
				}
				return evt1Diagnostic("CV4133", fmt.Sprintf("copy of non-copyable type %s is not allowed", s.Type.String()), s.Span)
			}
			local.declare(s.Name, evt1ValueBinding{t: evt1CanonicalType(env, s.Type), mutable: true})
		case *EVT1AssignStmt:
			target, err := validateAssignable(env, local, s.Target)
			if err != nil {
				return err
			}
			if !target.mutable {
				return evt1Diagnostic("CV4128", "mutation through a const access path is not allowed", s.Target.exprSpan())
			}
			valueType, err := validateExpr(env, local, s.Value)
			if err != nil {
				return err
			}
			if !evt1CanonicalType(env, target.t).Equal(evt1CanonicalType(env, valueType)) {
				return evt1Diagnostic("CV4107", fmt.Sprintf("assignment to %s expected %s but got %s", exprLabel(s.Target), target.t.String(), valueType.String()), s.Value.exprSpan())
			}
			if !evt1TypeCopyable(env, target.t) {
				if evt1IsImmovableValueType(env, target.t) && target.wholeValue {
					return evt1Diagnostic("CV4135", fmt.Sprintf("immovable value %s cannot be assigned as a whole", target.t.String()), s.Span)
				}
				return evt1Diagnostic("CV4133", fmt.Sprintf("assignment copies non-copyable type %s", target.t.String()), s.Span)
			}
		case *EVT1ReturnStmt:
			if s.Value == nil {
				if returnType.Name != "void" {
					return evt1Diagnostic("CV4022", fmt.Sprintf("return requires a %s value", returnType.String()), s.Span)
				}
				continue
			}
			valueType, err := validateExpr(env, local, s.Value)
			if err != nil {
				return err
			}
			if !evt1CanonicalType(env, returnType).Equal(evt1CanonicalType(env, valueType)) {
				return evt1Diagnostic("CV4116", fmt.Sprintf("expression result type mismatch: expected %s but got %s", returnType.String(), valueType.String()), s.Value.exprSpan())
			}
		case *EVT1ExprStmt:
			if _, err := validateExpr(env, local, s.Value); err != nil {
				return err
			}
		case *EVT1MatchStmt:
			if err := validateMatchStmt(env, local, *s, returnType); err != nil {
				return err
			}
		case *EVT1Block:
			if err := validateBlock(env, local, returnType, *s); err != nil {
				return err
			}
		default:
			return evt1Diagnostic("CV4023", "unsupported statement", stmt.statementSpan())
		}
	}
	return nil
}

func validateKnownType(env *evt1Env, t EVT1Type, span Span, conceptParam string, allowConceptApp bool) error {
	if t.PointerTo != nil {
		return validateKnownType(env, *t.PointerTo, span, conceptParam, allowConceptApp)
	}
	if t.Kind == EVT1TypeConceptParam {
		if conceptParam != "" && t.Name == conceptParam {
			return nil
		}
		return evt1Diagnostic("CV4148", fmt.Sprintf("unknown concept parameter %s", t.Name), span)
	}
	if len(t.TypeArgs) > 0 {
		if _, ok := env.concepts[t.Name]; ok {
			if !allowConceptApp {
				return evt1Diagnostic("CV4164", fmt.Sprintf("concept %s cannot be used as a runtime type", t.String()), span)
			}
			if len(t.TypeArgs) != 1 {
				return evt1Diagnostic("CV4149", fmt.Sprintf("concept %s requires exactly one type argument", t.Name), span)
			}
			return validateKnownType(env, t.TypeArgs[0], span, conceptParam, false)
		}
		return evt1Diagnostic("CV4102", fmt.Sprintf("unknown type application %s", t.String()), span)
	}
	if _, ok := evt1BuiltinType(t.Name, span); ok {
		return nil
	}
	if _, ok := env.enums[t.Name]; ok {
		return nil
	}
	if _, ok := env.structs[t.Name]; ok {
		return nil
	}
	return evt1Diagnostic("CV4102", fmt.Sprintf("unknown enum or type %s", t.Name), span)
}

func validateExpr(env *evt1Env, scope *evt1Scope, expr EVT1Expr) (EVT1Type, error) {
	switch e := expr.(type) {
	case *EVT1IntLiteral:
		t, _ := evt1BuiltinType("int", e.Span)
		return t, nil
	case *EVT1BoolLiteral:
		t, _ := evt1BuiltinType("bool", e.Span)
		return t, nil
	case *EVT1NameExpr:
		if binding, ok := scope.lookup(e.Name); ok {
			return evt1CanonicalType(env, binding.t), nil
		}
		if bindingSpan, ok := env.escapedArmBinding[e.Name]; ok {
			return EVT1Type{}, evt1Diagnostic("CV4114", fmt.Sprintf("payload binding %s is scoped to its match arm", e.Name), bindingSpan)
		}
		return EVT1Type{}, evt1Diagnostic("CV4024", fmt.Sprintf("unknown name %s", e.Name), e.Span)
	case *EVT1FieldExpr:
		receiverType, err := validateExpr(env, scope, e.Receiver)
		if err != nil {
			return EVT1Type{}, err
		}
		fields, baseName, err := evt1FieldSet(env, receiverType)
		if err != nil {
			return EVT1Type{}, evt1Diagnostic("CV4025", err.Error(), e.Span)
		}
		fieldType, ok := fields[e.Field]
		if !ok {
			return EVT1Type{}, evt1Diagnostic("CV4026", fmt.Sprintf("unknown field %s on %s", e.Field, baseName), e.Span)
		}
		return evt1CanonicalType(env, fieldType), nil
	case *EVT1CallExpr:
		fn, ok := env.functions[e.Callee]
		if !ok {
			return EVT1Type{}, evt1Diagnostic("CV4027", fmt.Sprintf("unknown function %s", e.Callee), e.Span)
		}
		if len(fn.Params) != len(e.Args) {
			return EVT1Type{}, evt1Diagnostic("CV4106", fmt.Sprintf("wrong constructor or call payload count for %s: expected %d but got %d", e.Callee, len(fn.Params), len(e.Args)), e.Span)
		}
		for i, arg := range e.Args {
			argType, err := validateExpr(env, scope, arg)
			if err != nil {
				return EVT1Type{}, err
			}
			if err := validateCallArgument(env, scope, fn.Params[i].Type, arg, argType); err != nil {
				return EVT1Type{}, err
			}
		}
		return evt1CanonicalType(env, fn.ReturnType), nil
	case *EVT1BinaryExpr:
		leftType, err := validateExpr(env, scope, e.Left)
		if err != nil {
			return EVT1Type{}, err
		}
		rightType, err := validateExpr(env, scope, e.Right)
		if err != nil {
			return EVT1Type{}, err
		}
		if leftType.Name == "int" && rightType.Name == "int" {
			return leftType, nil
		}
		if leftType.Name == "uint64" && rightType.Name == "uint64" {
			return leftType, nil
		}
		return EVT1Type{}, evt1Diagnostic("CV4028", "only int + int or uint64 + uint64 is supported in EVT1 expressions", e.Span)
	case *EVT1ConstructExpr:
		return validateConstructExpr(env, scope, *e)
	case *EVT1StructConstructExpr:
		return validateStructConstructExpr(env, scope, *e)
	case *EVT1MatchExpr:
		return validateMatchExpr(env, scope, *e)
	default:
		return EVT1Type{}, evt1Diagnostic("CV4029", fmt.Sprintf("unsupported expression %s", evt1Unexpected(expr)), expr.exprSpan())
	}
}

func validateCallArgument(env *evt1Env, scope *evt1Scope, paramType EVT1Type, arg EVT1Expr, argType EVT1Type) error {
	if paramType.isBorrow() {
		required := paramType.borrowBase()
		if argType.isBorrowLike() {
			if !argType.borrowBase().Equal(required) {
				return evt1Diagnostic("CV4154", fmt.Sprintf("borrow argument expected %s but got %s", paramType.String(), argType.String()), arg.exprSpan())
			}
			if !paramType.Const && argType.Const {
				return evt1Diagnostic("CV4155", fmt.Sprintf("mutable borrow argument for %s cannot accept const %s", required.String(), argType.String()), arg.exprSpan())
			}
			return nil
		}
		if !evt1CanonicalType(env, argType.valueType()).Equal(evt1CanonicalType(env, required)) {
			return evt1Diagnostic("CV4154", fmt.Sprintf("borrow argument expected %s but got %s", paramType.String(), argType.String()), arg.exprSpan())
		}
		lvalue, err := validateAssignable(env, scope, arg)
		if err != nil {
			return evt1Diagnostic("CV4127", fmt.Sprintf("borrow argument for %s requires an assignable access path", required.String()), arg.exprSpan())
		}
		if !paramType.Const && !lvalue.mutable {
			return evt1Diagnostic("CV4155", fmt.Sprintf("mutable borrow argument for %s cannot bind a const access path", required.String()), arg.exprSpan())
		}
		return nil
	}
	if !evt1CanonicalType(env, paramType).Equal(evt1CanonicalType(env, argType)) {
		return evt1Diagnostic("CV4107", fmt.Sprintf("wrong payload type for call argument: expected %s but got %s", paramType.String(), argType.String()), arg.exprSpan())
	}
	if !evt1TypeCopyable(env, argType) {
		return evt1Diagnostic("CV4133", fmt.Sprintf("call copies non-copyable type %s", argType.String()), arg.exprSpan())
	}
	return nil
}

func validateAssignable(env *evt1Env, scope *evt1Scope, expr EVT1Expr) (evt1LValue, error) {
	switch e := expr.(type) {
	case *EVT1NameExpr:
		binding, ok := scope.lookup(e.Name)
		if !ok {
			return evt1LValue{}, evt1Diagnostic("CV4127", fmt.Sprintf("unknown assignable target %s", e.Name), e.Span)
		}
		return evt1LValue{t: evt1CanonicalType(env, binding.t), mutable: binding.mutable, wholeValue: true}, nil
	case *EVT1FieldExpr:
		receiver, err := validateAssignable(env, scope, e.Receiver)
		if err != nil {
			return evt1LValue{}, err
		}
		fields, baseName, err := evt1FieldSet(env, receiver.t)
		if err != nil {
			return evt1LValue{}, evt1Diagnostic("CV4025", err.Error(), e.Span)
		}
		fieldType, ok := fields[e.Field]
		if !ok {
			return evt1LValue{}, evt1Diagnostic("CV4026", fmt.Sprintf("unknown field %s on %s", e.Field, baseName), e.Span)
		}
		return evt1LValue{t: evt1CanonicalType(env, fieldType), mutable: receiver.mutable, wholeValue: false}, nil
	default:
		return evt1LValue{}, evt1Diagnostic("CV4127", "assignment requires a local or field access target", expr.exprSpan())
	}
}

func evt1FieldSet(env *evt1Env, t EVT1Type) (map[string]EVT1Type, string, error) {
	base := t
	if t.isBorrowLike() {
		base = t.borrowBase()
	}
	fields, ok := env.fieldSets[base.Name]
	if !ok {
		return nil, base.Name, fmt.Errorf("type %s has no fields", base.Name)
	}
	return fields, base.Name, nil
}

func validateConstructExpr(env *evt1Env, scope *evt1Scope, expr EVT1ConstructExpr) (EVT1Type, error) {
	enumDecl, ok := env.enums[expr.EnumName]
	if !ok {
		return EVT1Type{}, evt1Diagnostic("CV4102", fmt.Sprintf("unknown enum type %s in qualified construction", expr.EnumName), expr.Span)
	}
	variant, ok := evt1LookupVariant(enumDecl, expr.VariantName)
	if !ok {
		return EVT1Type{}, evt1Diagnostic("CV4103", fmt.Sprintf("unknown variant %s::%s", expr.EnumName, expr.VariantName), expr.Span)
	}
	if len(variant.Payload) == 0 && len(expr.Args) > 0 {
		return EVT1Type{}, evt1Diagnostic("CV4104", fmt.Sprintf("unit variant %s::%s does not accept payload arguments", expr.EnumName, expr.VariantName), expr.Span)
	}
	if len(variant.Payload) > 0 && len(expr.Args) == 0 {
		return EVT1Type{}, evt1Diagnostic("CV4104", fmt.Sprintf("payload variant %s::%s requires construction arguments", expr.EnumName, expr.VariantName), expr.Span)
	}
	if len(variant.Payload) != len(expr.Args) {
		return EVT1Type{}, evt1Diagnostic("CV4106", fmt.Sprintf("wrong constructor payload count for %s::%s: expected %d but got %d", expr.EnumName, expr.VariantName, len(variant.Payload), len(expr.Args)), expr.Span)
	}
	for i, arg := range expr.Args {
		argType, err := validateExpr(env, scope, arg)
		if err != nil {
			return EVT1Type{}, err
		}
		if !evt1CanInitializeStoredType(env, variant.Payload[i].Type, argType) {
			return EVT1Type{}, evt1Diagnostic("CV4107", fmt.Sprintf("wrong constructor payload type for %s::%s position %d: expected %s but got %s", expr.EnumName, expr.VariantName, i+1, variant.Payload[i].Type.String(), argType.String()), arg.exprSpan())
		}
		if !evt1TypeCopyable(env, argType) {
			return EVT1Type{}, evt1Diagnostic("CV4133", fmt.Sprintf("enum construction copies non-copyable type %s", argType.String()), arg.exprSpan())
		}
	}
	return EVT1Type{Name: enumDecl.Name, Kind: EVT1TypeEnum, Span: expr.Span}, nil
}

func validateStructConstructExpr(env *evt1Env, scope *evt1Scope, expr EVT1StructConstructExpr) (EVT1Type, error) {
	structDecl, ok := env.structs[expr.StructName]
	if !ok {
		return EVT1Type{}, evt1Diagnostic("CV4125", fmt.Sprintf("unknown struct type %s", expr.StructName), expr.Span)
	}
	if len(structDecl.Fields) != len(expr.Args) {
		return EVT1Type{}, evt1Diagnostic("CV4126", fmt.Sprintf("wrong initializer count for %s: expected %d but got %d", expr.StructName, len(structDecl.Fields), len(expr.Args)), expr.Span)
	}
	for i, arg := range expr.Args {
		argType, err := validateExpr(env, scope, arg)
		if err != nil {
			return EVT1Type{}, err
		}
		if !evt1CanInitializeStoredType(env, structDecl.Fields[i].Type, argType) {
			return EVT1Type{}, evt1Diagnostic("CV4107", fmt.Sprintf("wrong initializer type for %s field %s: expected %s but got %s", expr.StructName, structDecl.Fields[i].Name, structDecl.Fields[i].Type.String(), argType.String()), arg.exprSpan())
		}
	}
	return EVT1Type{Name: structDecl.Name, Kind: EVT1TypeStruct, Span: expr.Span}, nil
}

func validateMatchExpr(env *evt1Env, scope *evt1Scope, expr EVT1MatchExpr) (EVT1Type, error) {
	subjectType, enumDecl, err := validateMatchSubject(env, scope, expr.Subject)
	if err != nil {
		return EVT1Type{}, err
	}
	seen := map[string]bool{}
	var resultType EVT1Type
	for index, arm := range expr.Arms {
		armScope, _, err := validatePattern(env, scope, subjectType, enumDecl, arm.Pattern, seen)
		if err != nil {
			return EVT1Type{}, err
		}
		valueType, err := validateExpr(env, armScope, arm.Value)
		if err != nil {
			return EVT1Type{}, err
		}
		if index == 0 {
			resultType = evt1CanonicalType(env, valueType)
		} else if !evt1CanonicalType(env, resultType).Equal(evt1CanonicalType(env, valueType)) {
			return EVT1Type{}, evt1Diagnostic("CV4116", fmt.Sprintf("incompatible expression-arm result types: expected %s but got %s", resultType.String(), valueType.String()), arm.Value.exprSpan())
		}
	}
	if missing := evt1MissingVariants(enumDecl, seen); len(missing) > 0 {
		return EVT1Type{}, evt1Diagnostic("CV4115", "non-exhaustive match, missing variants: "+strings.Join(missing, ", "), expr.Span)
	}
	return resultType, nil
}

func validateMatchStmt(env *evt1Env, scope *evt1Scope, stmt EVT1MatchStmt, returnType EVT1Type) error {
	subjectType, enumDecl, err := validateMatchSubject(env, scope, stmt.Subject)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, arm := range stmt.Arms {
		armScope, _, err := validatePattern(env, scope, subjectType, enumDecl, arm.Pattern, seen)
		if err != nil {
			return err
		}
		if err := validateBlock(env, armScope, returnType, arm.Block); err != nil {
			return err
		}
	}
	if missing := evt1MissingVariants(enumDecl, seen); len(missing) > 0 {
		return evt1Diagnostic("CV4115", "non-exhaustive match, missing variants: "+strings.Join(missing, ", "), stmt.Span)
	}
	return nil
}

func validateMatchSubject(env *evt1Env, scope *evt1Scope, subject EVT1Expr) (EVT1Type, EVT1EnumDecl, error) {
	subjectType, err := validateExpr(env, scope, subject)
	if err != nil {
		return EVT1Type{}, EVT1EnumDecl{}, err
	}
	if subjectType.Kind != EVT1TypeEnum {
		enumDecl, ok := env.enums[subjectType.Name]
		if !ok {
			return EVT1Type{}, EVT1EnumDecl{}, evt1Diagnostic("CV4108", fmt.Sprintf("match subject must be an enum, got %s", subjectType.String()), subject.exprSpan())
		}
		return subjectType, enumDecl, nil
	}
	return subjectType, env.enums[subjectType.Name], nil
}

func validatePattern(env *evt1Env, scope *evt1Scope, subjectType EVT1Type, enumDecl EVT1EnumDecl, pattern EVT1Pattern, seen map[string]bool) (*evt1Scope, EVT1VariantDecl, error) {
	if pattern.EnumName != enumDecl.Name {
		if _, ok := env.enums[pattern.EnumName]; ok {
			return nil, EVT1VariantDecl{}, evt1Diagnostic("CV4109", fmt.Sprintf("match arm pattern uses %s on subject of type %s", pattern.EnumName, subjectType.Name), pattern.Span)
		}
		return nil, EVT1VariantDecl{}, evt1Diagnostic("CV4102", fmt.Sprintf("unknown enum type %s in match arm", pattern.EnumName), pattern.Span)
	}
	variant, ok := evt1LookupVariant(enumDecl, pattern.VariantName)
	if !ok {
		return nil, EVT1VariantDecl{}, evt1Diagnostic("CV4110", fmt.Sprintf("unknown match variant %s::%s", pattern.EnumName, pattern.VariantName), pattern.Span)
	}
	key := pattern.EnumName + "::" + pattern.VariantName
	if seen[key] {
		return nil, EVT1VariantDecl{}, evt1Diagnostic("CV4113", fmt.Sprintf("duplicate match arm %s", key), pattern.Span)
	}
	seen[key] = true
	if len(variant.Payload) == 0 && len(pattern.Bindings) > 0 {
		return nil, EVT1VariantDecl{}, evt1Diagnostic("CV4112", fmt.Sprintf("unit variant %s::%s cannot bind payload names", pattern.EnumName, pattern.VariantName), pattern.Span)
	}
	if len(variant.Payload) != len(pattern.Bindings) {
		return nil, EVT1VariantDecl{}, evt1Diagnostic("CV4111", fmt.Sprintf("payload binding count mismatch for %s::%s: expected %d but got %d", pattern.EnumName, pattern.VariantName, len(variant.Payload), len(pattern.Bindings)), pattern.Span)
	}
	armScope := newEVT1Scope(scope)
	seenBindings := map[string]bool{}
	for i, binding := range pattern.Bindings {
		if seenBindings[binding] {
			return nil, EVT1VariantDecl{}, evt1Diagnostic("CV4112", fmt.Sprintf("duplicate payload binding name %s", binding), pattern.Span)
		}
		seenBindings[binding] = true
		armScope.declare(binding, evt1ValueBinding{t: evt1CanonicalType(env, variant.Payload[i].Type), mutable: true})
	}
	return armScope, variant, nil
}

func validateValueLayoutCycles(env *evt1Env) error {
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var dfs func(name string, span Span) error
	dfs = func(name string, span Span) error {
		if visiting[name] {
			return evt1Diagnostic("CV4129", fmt.Sprintf("recursive by-value type cycle involving %s", name), span)
		}
		if visited[name] {
			return nil
		}
		visiting[name] = true
		visited[name] = true
		if structDecl, ok := env.structs[name]; ok {
			for _, field := range structDecl.Fields {
				if target, ok := evt1ByValueTypeName(field.Type); ok {
					if err := dfs(target, field.Span); err != nil {
						return err
					}
				}
			}
		}
		if enumDecl, ok := env.enums[name]; ok {
			for _, variant := range enumDecl.Variants {
				for _, field := range variant.Payload {
					if target, ok := evt1ByValueTypeName(field.Type); ok {
						if err := dfs(target, field.Span); err != nil {
							return err
						}
					}
				}
			}
		}
		visiting[name] = false
		return nil
	}
	for _, structDecl := range env.structs {
		if err := dfs(structDecl.Name, structDecl.Span); err != nil {
			return err
		}
	}
	return nil
}

func evt1ByValueTypeName(t EVT1Type) (string, bool) {
	if t.isBorrowLike() {
		return "", false
	}
	if len(t.TypeArgs) > 0 {
		return "", false
	}
	if t.Kind == EVT1TypeBuiltin || t.Kind == EVT1TypeConceptParam {
		return "", false
	}
	return t.Name, true
}

func evt1TypeCopyable(env *evt1Env, t EVT1Type) bool {
	if t.isBorrowLike() {
		return true
	}
	if t.isOwned() {
		return false
	}
	if len(t.TypeArgs) > 0 {
		return false
	}
	if _, ok := evt1BuiltinType(t.Name, t.Span); ok {
		return true
	}
	if cached, ok := env.copyableCache[t.Name]; ok {
		return cached
	}
	if structDecl, ok := env.structs[t.Name]; ok {
		if structDecl.Immovable {
			env.copyableCache[t.Name] = false
			return false
		}
		env.copyableCache[t.Name] = true
		for _, field := range structDecl.Fields {
			if !evt1TypeCopyable(env, field.Type) {
				env.copyableCache[t.Name] = false
				return false
			}
		}
		return true
	}
	if enumDecl, ok := env.enums[t.Name]; ok {
		env.copyableCache[t.Name] = true
		for _, variant := range enumDecl.Variants {
			for _, field := range variant.Payload {
				if !evt1TypeCopyable(env, field.Type) {
					env.copyableCache[t.Name] = false
					return false
				}
			}
		}
		return true
	}
	return true
}

func evt1IsImmovableValueType(env *evt1Env, t EVT1Type) bool {
	if t.isBorrowLike() || len(t.TypeArgs) > 0 {
		return false
	}
	if structDecl, ok := env.structs[t.Name]; ok {
		return structDecl.Immovable
	}
	return false
}

func evt1CanDirectInitialize(env *evt1Env, t EVT1Type, expr EVT1Expr) bool {
	construct, ok := expr.(*EVT1StructConstructExpr)
	if !ok {
		return false
	}
	return construct.StructName == t.Name && evt1LookupStruct(env, t.Name)
}

func evt1LookupStruct(env *evt1Env, name string) bool {
	_, ok := env.structs[name]
	return ok
}

func validateConceptCycles(env *evt1Env) error {
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var dfs func(name string, span Span) error
	dfs = func(name string, span Span) error {
		if visiting[name] {
			return evt1Diagnostic("CV4162", fmt.Sprintf("concept dependency cycle involving %s", name), span)
		}
		if visited[name] {
			return nil
		}
		visiting[name] = true
		visited[name] = true
		for _, req := range env.concepts[name].Requirements {
			ref, ok := req.(*EVT1PrerequisiteRequirement)
			if !ok {
				continue
			}
			if err := dfs(ref.ConceptName, ref.Span); err != nil {
				return err
			}
		}
		visiting[name] = false
		return nil
	}
	for _, conceptDecl := range env.concepts {
		if err := dfs(conceptDecl.Name, conceptDecl.Span); err != nil {
			return err
		}
	}
	return nil
}

func checkConceptSatisfaction(env *evt1Env, conceptName string, concreteType EVT1Type, path []string, span Span) error {
	path = append(path, fmt.Sprintf("%s<%s>", conceptName, concreteType.String()))
	conceptDecl := env.concepts[conceptName]
	for _, req := range conceptDecl.Requirements {
		switch r := req.(type) {
		case *EVT1PrerequisiteRequirement:
			if err := checkConceptSatisfaction(env, r.ConceptName, concreteType, path, span); err != nil {
				return err
			}
		case *EVT1OperationRequirement:
			required := evt1SubstituteRequirement(*r, conceptDecl.TypeParam, concreteType)
			fn, ok := env.functions[r.Name]
			if !ok {
				return evt1Diagnostic("CV4153", fmt.Sprintf("%s is missing required operation %s", strings.Join(path, " -> "), evt1Signature(required.ReturnType, required.Name, required.Params)), span)
			}
			if len(fn.Params) != len(required.Params) {
				return evt1Diagnostic("CV4154", fmt.Sprintf("%s requires %s but found %s", strings.Join(path, " -> "), evt1Signature(required.ReturnType, required.Name, required.Params), evt1FunctionSignature(fn)), span)
			}
			for i := range fn.Params {
				if !evt1CanonicalType(env, fn.Params[i].Type).Equal(evt1CanonicalType(env, required.Params[i].Type)) {
					code := "CV4154"
					if evt1CanonicalType(env, fn.Params[i].Type.valueType()).Equal(evt1CanonicalType(env, required.Params[i].Type.valueType())) {
						code = "CV4155"
					}
					return evt1Diagnostic(code, fmt.Sprintf("%s requires %s but found %s", strings.Join(path, " -> "), evt1Signature(required.ReturnType, required.Name, required.Params), evt1FunctionSignature(fn)), span)
				}
			}
			if !evt1CanonicalType(env, fn.ReturnType).Equal(evt1CanonicalType(env, required.ReturnType)) {
				return evt1Diagnostic("CV4156", fmt.Sprintf("%s requires %s but found %s", strings.Join(path, " -> "), evt1Signature(required.ReturnType, required.Name, required.Params), evt1FunctionSignature(fn)), span)
			}
		}
	}
	return nil
}

func evt1SubstituteRequirement(req EVT1OperationRequirement, typeParam string, concreteType EVT1Type) EVT1OperationRequirement {
	out := req
	out.ReturnType = evt1SubstituteType(req.ReturnType, typeParam, concreteType)
	out.Params = make([]EVT1Param, 0, len(req.Params))
	for _, param := range req.Params {
		out.Params = append(out.Params, EVT1Param{
			Type: evt1SubstituteType(param.Type, typeParam, concreteType),
			Name: param.Name,
			Span: param.Span,
		})
	}
	return out
}

func evt1SubstituteType(t EVT1Type, typeParam string, concreteType EVT1Type) EVT1Type {
	if t.Kind == EVT1TypeConceptParam && t.Name == typeParam {
		out := concreteType
		out.Ownership = t.Ownership
		out.Const = t.Const
		out.Imported = t.Imported
		out.Unsafe = t.Unsafe
		return out
	}
	if t.PointerTo != nil {
		base := evt1SubstituteType(*t.PointerTo, typeParam, concreteType)
		t.PointerTo = &base
		return t
	}
	for i := range t.TypeArgs {
		t.TypeArgs[i] = evt1SubstituteType(t.TypeArgs[i], typeParam, concreteType)
	}
	return t
}

func evt1Signature(retType EVT1Type, name string, params []EVT1Param) string {
	var parts []string
	for _, param := range params {
		parts = append(parts, param.Type.String())
	}
	return fmt.Sprintf("%s %s(%s)", retType.String(), name, strings.Join(parts, ", "))
}

func evt1FunctionSignature(fn EVT1FunctionDecl) string {
	return evt1Signature(fn.ReturnType, fn.Name, fn.Params)
}

func exprLabel(expr EVT1Expr) string {
	switch e := expr.(type) {
	case *EVT1NameExpr:
		return e.Name
	case *EVT1FieldExpr:
		return exprLabel(e.Receiver) + "." + e.Field
	default:
		return "expression"
	}
}

func evt1CanonicalType(env *evt1Env, t EVT1Type) EVT1Type {
	if t.PointerTo != nil {
		base := evt1CanonicalType(env, *t.PointerTo)
		t.PointerTo = &base
		return t
	}
	for i := range t.TypeArgs {
		t.TypeArgs[i] = evt1CanonicalType(env, t.TypeArgs[i])
	}
	if t.Kind == EVT1TypeConceptParam || len(t.TypeArgs) > 0 {
		return t
	}
	if _, ok := evt1BuiltinType(t.Name, t.Span); ok {
		t.Kind = EVT1TypeBuiltin
		return t
	}
	if _, ok := env.enums[t.Name]; ok {
		t.Kind = EVT1TypeEnum
		return t
	}
	if _, ok := env.structs[t.Name]; ok {
		t.Kind = EVT1TypeStruct
		return t
	}
	return t
}

func evt1CanInitializeStoredType(env *evt1Env, expected EVT1Type, actual EVT1Type) bool {
	return evt1CanonicalType(env, expected.valueType()).Equal(evt1CanonicalType(env, actual.valueType()))
}
