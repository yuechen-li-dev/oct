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
	for _, templateDecl := range module.Templates {
		if _, exists := env.templates[templateDecl.Name]; exists {
			return nil, evt1Diagnostic("CV4168", fmt.Sprintf("duplicate template declaration %s", templateDecl.Name), templateDecl.Span)
		}
		env.templates[templateDecl.Name] = templateDecl
	}
	for _, fn := range module.Functions {
		for _, existing := range env.functions[fn.Name] {
			if evt1FunctionParamSignature(existing) == evt1FunctionParamSignature(fn) {
				return nil, evt1Diagnostic("CV4021", fmt.Sprintf("duplicate function declaration %s", fn.Name), fn.Span)
			}
		}
		env.functions[fn.Name] = append(env.functions[fn.Name], fn)
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
	for _, templateDecl := range module.Templates {
		if err := validateTemplateSignature(env, templateDecl); err != nil {
			return nil, err
		}
		info, err := buildTemplateInfo(env, templateDecl)
		if err != nil {
			return nil, err
		}
		env.templateInfos[templateDecl.Name] = info
	}
	for _, fn := range module.Functions {
		if err := validateFunctionSignature(env, fn); err != nil {
			return nil, err
		}
	}
	for _, templateDecl := range module.Templates {
		scope := newEVT1Scope(nil)
		for _, param := range templateDecl.Params {
			scope.declare(param.Name, evt1ValueBinding{
				t:       evt1CanonicalType(env, param.Type),
				mutable: !(param.Type.isBorrowLike() && param.Type.Const),
			})
		}
		if err := validateBlock(env, scope, templateDecl.ReturnType, *templateDecl.Body, env.templateInfos[templateDecl.Name]); err != nil {
			return nil, err
		}
	}
	for _, fn := range module.Functions {
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
		if err := validateBlock(env, scope, fn.ReturnType, *fn.Body, nil); err != nil {
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

func validateTemplateSignature(env *evt1Env, templateDecl EVT1TemplateDecl) error {
	if _, ok := env.concepts[templateDecl.Constraint.ConceptName]; !ok {
		return evt1Diagnostic("CV4169", fmt.Sprintf("unknown concept %s in template constraint", templateDecl.Constraint.ConceptName), templateDecl.Constraint.Span)
	}
	constraintConcept := env.concepts[templateDecl.Constraint.ConceptName]
	if templateDecl.Constraint.TypeArg.Kind != EVT1TypeConceptParam || templateDecl.Constraint.TypeArg.Name != templateDecl.TypeParam {
		return evt1Diagnostic("CV4170", fmt.Sprintf("template constraint %s must apply to template parameter %s", templateDecl.Constraint.ConceptName, templateDecl.TypeParam), templateDecl.Constraint.Span)
	}
	if err := validateKnownType(env, templateDecl.ReturnType, templateDecl.ReturnType.Span, templateDecl.TypeParam, false); err != nil {
		return err
	}
	if err := validateTemplateByValueBoundary(env, templateDecl.ReturnType, templateDecl.Span, "return", templateDecl.TypeParam); err != nil {
		return err
	}
	if constraintConcept.TypeParam == "" {
		return evt1Diagnostic("CV4171", fmt.Sprintf("template constraint %s must be a named one-parameter concept", templateDecl.Constraint.ConceptName), templateDecl.Constraint.Span)
	}
	for _, param := range templateDecl.Params {
		if err := validateKnownType(env, param.Type, param.Span, templateDecl.TypeParam, false); err != nil {
			return err
		}
		if err := validateTemplateByValueBoundary(env, param.Type, param.Span, "parameter", templateDecl.TypeParam); err != nil {
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

func validateTemplateByValueBoundary(env *evt1Env, t EVT1Type, span Span, context, typeParam string) error {
	if evt1TypeDependsOnParam(t, typeParam) {
		return nil
	}
	return validateByValueBoundary(env, t, span, context)
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

func validateBlock(env *evt1Env, scope *evt1Scope, returnType EVT1Type, block EVT1Block, templateInfo *evt1TemplateInfo) error {
	local := newEVT1Scope(scope)
	for _, stmt := range block.Statements {
		switch s := stmt.(type) {
		case *EVT1VarDecl:
			typeParam := ""
			if templateInfo != nil {
				typeParam = templateInfo.Decl.TypeParam
			}
			if err := validateKnownType(env, s.Type, s.Span, typeParam, false); err != nil {
				return err
			}
			valueType, err := validateExpr(env, local, s.Value, templateInfo)
			if err != nil {
				return err
			}
			if !evt1TypesCompatible(env, s.Type, valueType, typeParam) {
				return evt1Diagnostic("CV4106", fmt.Sprintf("constructor or initializer for %s expected %s but got %s", s.Name, s.Type.String(), valueType.String()), s.Value.exprSpan())
			}
			if !evt1CanDirectInitialize(env, s.Type, s.Value) && !evt1TypeCopyable(env, s.Type) && !evt1TypeDependsOnParam(s.Type, typeParam) {
				if evt1IsImmovableValueType(env, s.Type) {
					return evt1Diagnostic("CV4134", fmt.Sprintf("immovable value %s must be constructed directly in final storage", s.Type.String()), s.Span)
				}
				return evt1Diagnostic("CV4133", fmt.Sprintf("copy of non-copyable type %s is not allowed", s.Type.String()), s.Span)
			}
			local.declare(s.Name, evt1ValueBinding{t: evt1CanonicalType(env, s.Type), mutable: true})
		case *EVT1AssignStmt:
			target, err := validateAssignable(env, local, s.Target, templateInfo)
			if err != nil {
				return err
			}
			if !target.mutable {
				return evt1Diagnostic("CV4128", "mutation through a const access path is not allowed", s.Target.exprSpan())
			}
			valueType, err := validateExpr(env, local, s.Value, templateInfo)
			if err != nil {
				return err
			}
			typeParam := ""
			if templateInfo != nil {
				typeParam = templateInfo.Decl.TypeParam
			}
			if !evt1TypesCompatible(env, target.t, valueType, typeParam) {
				return evt1Diagnostic("CV4107", fmt.Sprintf("assignment to %s expected %s but got %s", exprLabel(s.Target), target.t.String(), valueType.String()), s.Value.exprSpan())
			}
			if !evt1TypeCopyable(env, target.t) && !evt1TypeDependsOnParam(target.t, typeParam) {
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
			valueType, err := validateExpr(env, local, s.Value, templateInfo)
			if err != nil {
				return err
			}
			typeParam := ""
			if templateInfo != nil {
				typeParam = templateInfo.Decl.TypeParam
			}
			if !evt1TypesCompatible(env, returnType, valueType, typeParam) {
				return evt1Diagnostic("CV4116", fmt.Sprintf("expression result type mismatch: expected %s but got %s", returnType.String(), valueType.String()), s.Value.exprSpan())
			}
		case *EVT1ExprStmt:
			if _, err := validateExpr(env, local, s.Value, templateInfo); err != nil {
				return err
			}
		case *EVT1MatchStmt:
			if err := validateMatchStmt(env, local, *s, returnType, templateInfo); err != nil {
				return err
			}
		case *EVT1Block:
			if err := validateBlock(env, local, returnType, *s, templateInfo); err != nil {
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

func validateExpr(env *evt1Env, scope *evt1Scope, expr EVT1Expr, templateInfo *evt1TemplateInfo) (EVT1Type, error) {
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
		receiverType, err := validateExpr(env, scope, e.Receiver, templateInfo)
		if err != nil {
			return EVT1Type{}, err
		}
		if templateInfo != nil && evt1TypeDependsOnParam(receiverType, templateInfo.Decl.TypeParam) {
			return EVT1Type{}, evt1Diagnostic("CV4172", "dependent field access is not allowed in EVT1 M1B-B templates", e.Span)
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
		if templateInfo != nil {
			return validateTemplateCallExpr(env, scope, *e, templateInfo)
		}
		argTypes := make([]EVT1Type, 0, len(e.Args))
		for _, arg := range e.Args {
			argType, err := validateExpr(env, scope, arg, templateInfo)
			if err != nil {
				return EVT1Type{}, err
			}
			argTypes = append(argTypes, argType)
		}
		fn, err := evt1ResolveOrdinaryCall(env, scope, e.Callee, e.Args, argTypes, templateInfo, e.Span)
		if err != nil {
			if _, exists := env.templates[e.Callee]; exists {
				return EVT1Type{}, evt1Diagnostic("CV4173", fmt.Sprintf("template call %s requires an explicit concrete type argument", e.Callee), e.Span)
			}
			return EVT1Type{}, err
		}
		for i, arg := range e.Args {
			if err := validateCallArgument(env, scope, fn.Params[i].Type, arg, argTypes[i], templateInfo); err != nil {
				return EVT1Type{}, err
			}
		}
		return evt1CanonicalType(env, fn.ReturnType), nil
	case *EVT1TemplateCallExpr:
		if templateInfo != nil {
			return EVT1Type{}, evt1Diagnostic("CV4174", "templates cannot invoke templates in EVT1 M1B-B", e.Span)
		}
		instance, err := instantiateTemplate(env, e.Callee, e.TypeArg, e.Span)
		if err != nil {
			return EVT1Type{}, err
		}
		if len(instance.Function.Params) != len(e.Args) {
			return EVT1Type{}, evt1Diagnostic("CV4106", fmt.Sprintf("wrong constructor or call payload count for %s: expected %d but got %d", e.Callee, len(instance.Function.Params), len(e.Args)), e.Span)
		}
		for i, arg := range e.Args {
			argType, err := validateExpr(env, scope, arg, nil)
			if err != nil {
				return EVT1Type{}, err
			}
			if err := validateCallArgument(env, scope, instance.Function.Params[i].Type, arg, argType, nil); err != nil {
				return EVT1Type{}, err
			}
		}
		instance.InvocationSpans = append(instance.InvocationSpans, e.Span)
		return evt1CanonicalType(env, instance.Function.ReturnType), nil
	case *EVT1BinaryExpr:
		leftType, err := validateExpr(env, scope, e.Left, templateInfo)
		if err != nil {
			return EVT1Type{}, err
		}
		rightType, err := validateExpr(env, scope, e.Right, templateInfo)
		if err != nil {
			return EVT1Type{}, err
		}
		if templateInfo != nil && (evt1TypeDependsOnParam(leftType, templateInfo.Decl.TypeParam) || evt1TypeDependsOnParam(rightType, templateInfo.Decl.TypeParam)) {
			return EVT1Type{}, evt1Diagnostic("CV4175", "dependent operators are not allowed in EVT1 M1B-B templates", e.Span)
		}
		if leftType.Name == "int" && rightType.Name == "int" {
			if e.Op == "<" || e.Op == ">" {
				out, _ := evt1BuiltinType("bool", e.Span)
				return out, nil
			}
			return leftType, nil
		}
		if leftType.Name == "uint64" && rightType.Name == "uint64" {
			if e.Op == "<" || e.Op == ">" {
				out, _ := evt1BuiltinType("bool", e.Span)
				return out, nil
			}
			return leftType, nil
		}
		return EVT1Type{}, evt1Diagnostic("CV4028", "only int/uint64 additive and comparison expressions are supported in EVT1", e.Span)
	case *EVT1ConstructExpr:
		return validateConstructExpr(env, scope, *e)
	case *EVT1StructConstructExpr:
		return validateStructConstructExpr(env, scope, *e)
	case *EVT1MatchExpr:
		return validateMatchExpr(env, scope, *e, templateInfo)
	default:
		return EVT1Type{}, evt1Diagnostic("CV4029", fmt.Sprintf("unsupported expression %s", evt1Unexpected(expr)), expr.exprSpan())
	}
}

func validateCallArgument(env *evt1Env, scope *evt1Scope, paramType EVT1Type, arg EVT1Expr, argType EVT1Type, templateInfo *evt1TemplateInfo) error {
	typeParam := ""
	if templateInfo != nil {
		typeParam = templateInfo.Decl.TypeParam
	}
	if paramType.isBorrow() {
		required := paramType.borrowBase()
		if argType.isBorrowLike() {
			if !evt1TypesCompatible(env, required, argType.borrowBase(), typeParam) {
				return evt1Diagnostic("CV4154", fmt.Sprintf("borrow argument expected %s but got %s", paramType.String(), argType.String()), arg.exprSpan())
			}
			if !paramType.Const && argType.Const {
				return evt1Diagnostic("CV4155", fmt.Sprintf("mutable borrow argument for %s cannot accept const %s", required.String(), argType.String()), arg.exprSpan())
			}
			return nil
		}
		if !evt1TypesCompatible(env, required, argType.valueType(), typeParam) {
			return evt1Diagnostic("CV4154", fmt.Sprintf("borrow argument expected %s but got %s", paramType.String(), argType.String()), arg.exprSpan())
		}
		lvalue, err := validateAssignable(env, scope, arg, templateInfo)
		if err != nil {
			return evt1Diagnostic("CV4127", fmt.Sprintf("borrow argument for %s requires an assignable access path", required.String()), arg.exprSpan())
		}
		if !paramType.Const && !lvalue.mutable {
			return evt1Diagnostic("CV4155", fmt.Sprintf("mutable borrow argument for %s cannot bind a const access path", required.String()), arg.exprSpan())
		}
		return nil
	}
	if !evt1TypesCompatible(env, paramType, argType, typeParam) {
		return evt1Diagnostic("CV4107", fmt.Sprintf("wrong payload type for call argument: expected %s but got %s", paramType.String(), argType.String()), arg.exprSpan())
	}
	if !evt1TypeCopyable(env, argType) && !evt1TypeDependsOnParam(argType, typeParam) {
		return evt1Diagnostic("CV4133", fmt.Sprintf("call copies non-copyable type %s", argType.String()), arg.exprSpan())
	}
	return nil
}

func validateAssignable(env *evt1Env, scope *evt1Scope, expr EVT1Expr, templateInfo *evt1TemplateInfo) (evt1LValue, error) {
	switch e := expr.(type) {
	case *EVT1NameExpr:
		binding, ok := scope.lookup(e.Name)
		if !ok {
			return evt1LValue{}, evt1Diagnostic("CV4127", fmt.Sprintf("unknown assignable target %s", e.Name), e.Span)
		}
		return evt1LValue{t: evt1CanonicalType(env, binding.t), mutable: binding.mutable, wholeValue: true}, nil
	case *EVT1FieldExpr:
		receiver, err := validateAssignable(env, scope, e.Receiver, templateInfo)
		if err != nil {
			return evt1LValue{}, err
		}
		if templateInfo != nil && evt1TypeDependsOnParam(receiver.t, templateInfo.Decl.TypeParam) {
			return evt1LValue{}, evt1Diagnostic("CV4172", "dependent field access is not allowed in EVT1 M1B-B templates", e.Span)
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
		argType, err := validateExpr(env, scope, arg, nil)
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
		argType, err := validateExpr(env, scope, arg, nil)
		if err != nil {
			return EVT1Type{}, err
		}
		if !evt1CanInitializeStoredType(env, structDecl.Fields[i].Type, argType) {
			return EVT1Type{}, evt1Diagnostic("CV4107", fmt.Sprintf("wrong initializer type for %s field %s: expected %s but got %s", expr.StructName, structDecl.Fields[i].Name, structDecl.Fields[i].Type.String(), argType.String()), arg.exprSpan())
		}
	}
	return EVT1Type{Name: structDecl.Name, Kind: EVT1TypeStruct, Span: expr.Span}, nil
}

func validateMatchExpr(env *evt1Env, scope *evt1Scope, expr EVT1MatchExpr, templateInfo *evt1TemplateInfo) (EVT1Type, error) {
	subjectType, enumDecl, err := validateMatchSubject(env, scope, expr.Subject, templateInfo)
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
		valueType, err := validateExpr(env, armScope, arm.Value, templateInfo)
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

func validateMatchStmt(env *evt1Env, scope *evt1Scope, stmt EVT1MatchStmt, returnType EVT1Type, templateInfo *evt1TemplateInfo) error {
	subjectType, enumDecl, err := validateMatchSubject(env, scope, stmt.Subject, templateInfo)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, arm := range stmt.Arms {
		armScope, _, err := validatePattern(env, scope, subjectType, enumDecl, arm.Pattern, seen)
		if err != nil {
			return err
		}
		if err := validateBlock(env, armScope, returnType, arm.Block, templateInfo); err != nil {
			return err
		}
	}
	if missing := evt1MissingVariants(enumDecl, seen); len(missing) > 0 {
		return evt1Diagnostic("CV4115", "non-exhaustive match, missing variants: "+strings.Join(missing, ", "), stmt.Span)
	}
	return nil
}

func validateMatchSubject(env *evt1Env, scope *evt1Scope, subject EVT1Expr, templateInfo *evt1TemplateInfo) (EVT1Type, EVT1EnumDecl, error) {
	subjectType, err := validateExpr(env, scope, subject, templateInfo)
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

func evt1ResolveOrdinaryCall(env *evt1Env, scope *evt1Scope, name string, args []EVT1Expr, argTypes []EVT1Type, templateInfo *evt1TemplateInfo, span Span) (EVT1FunctionDecl, error) {
	candidates, ok := env.functions[name]
	if !ok || len(candidates) == 0 {
		return EVT1FunctionDecl{}, evt1Diagnostic("CV4027", fmt.Sprintf("unknown function %s", name), span)
	}
	var matches []EVT1FunctionDecl
	for _, fn := range candidates {
		if len(fn.Params) != len(args) {
			continue
		}
		match := true
		for i, arg := range args {
			if err := validateCallArgument(env, scope, fn.Params[i].Type, arg, argTypes[i], templateInfo); err != nil {
				match = false
				break
			}
		}
		if match {
			matches = append(matches, fn)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return EVT1FunctionDecl{}, evt1Diagnostic("CV4182", fmt.Sprintf("call %s is ambiguous under exact-signature matching", name), span)
	}
	return EVT1FunctionDecl{}, evt1Diagnostic("CV4107", fmt.Sprintf("no exact call target matched %s", name), span)
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
			if _, err := evt1LookupRequiredOperation(env, required, span, strings.Join(path, " -> ")); err != nil {
				return err
			}
		}
	}
	return nil
}

func evt1LookupRequiredOperation(env *evt1Env, required EVT1OperationRequirement, span Span, prefix string) (EVT1FunctionDecl, error) {
	candidates := env.functions[required.Name]
	if len(candidates) == 0 {
		return EVT1FunctionDecl{}, evt1Diagnostic("CV4153", fmt.Sprintf("%s is missing required operation %s", prefix, evt1Signature(required.ReturnType, required.Name, required.Params)), span)
	}
	var exact []EVT1FunctionDecl
	for _, fn := range candidates {
		if len(fn.Params) != len(required.Params) {
			continue
		}
		paramsMatch := true
		qualifierMismatch := false
		for i := range fn.Params {
			if !evt1CanonicalType(env, fn.Params[i].Type).Equal(evt1CanonicalType(env, required.Params[i].Type)) {
				if evt1CanonicalType(env, fn.Params[i].Type.valueType()).Equal(evt1CanonicalType(env, required.Params[i].Type.valueType())) {
					qualifierMismatch = true
				}
				paramsMatch = false
				break
			}
		}
		if !paramsMatch {
			if qualifierMismatch {
				return EVT1FunctionDecl{}, evt1Diagnostic("CV4155", fmt.Sprintf("%s requires %s but found %s", prefix, evt1Signature(required.ReturnType, required.Name, required.Params), evt1FunctionSignature(fn)), span)
			}
			continue
		}
		if !evt1CanonicalType(env, fn.ReturnType).Equal(evt1CanonicalType(env, required.ReturnType)) {
			return EVT1FunctionDecl{}, evt1Diagnostic("CV4156", fmt.Sprintf("%s requires %s but found %s", prefix, evt1Signature(required.ReturnType, required.Name, required.Params), evt1FunctionSignature(fn)), span)
		}
		exact = append(exact, fn)
	}
	if len(exact) == 1 {
		return exact[0], nil
	}
	if len(exact) > 1 {
		return EVT1FunctionDecl{}, evt1Diagnostic("CV4182", fmt.Sprintf("%s has ambiguous required operation %s", prefix, evt1Signature(required.ReturnType, required.Name, required.Params)), span)
	}
	for _, fn := range candidates {
		if len(fn.Params) == len(required.Params) {
			code := "CV4154"
			for i := range fn.Params {
				if i < len(required.Params) && evt1CanonicalType(env, fn.Params[i].Type.valueType()).Equal(evt1CanonicalType(env, required.Params[i].Type.valueType())) && !evt1CanonicalType(env, fn.Params[i].Type).Equal(evt1CanonicalType(env, required.Params[i].Type)) {
					code = "CV4155"
					break
				}
			}
			return EVT1FunctionDecl{}, evt1Diagnostic(code, fmt.Sprintf("%s requires %s but found %s", prefix, evt1Signature(required.ReturnType, required.Name, required.Params), evt1FunctionSignature(fn)), span)
		}
	}
	return EVT1FunctionDecl{}, evt1Diagnostic("CV4153", fmt.Sprintf("%s is missing required operation %s", prefix, evt1Signature(required.ReturnType, required.Name, required.Params)), span)
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

func evt1FunctionParamSignature(fn EVT1FunctionDecl) string {
	var parts []string
	for _, param := range fn.Params {
		parts = append(parts, param.Type.String())
	}
	return fmt.Sprintf("%s(%s)", fn.Name, strings.Join(parts, ", "))
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

func evt1TypeDependsOnParam(t EVT1Type, typeParam string) bool {
	if typeParam == "" {
		return false
	}
	if t.Kind == EVT1TypeConceptParam && t.Name == typeParam {
		return true
	}
	if t.PointerTo != nil && evt1TypeDependsOnParam(*t.PointerTo, typeParam) {
		return true
	}
	for _, arg := range t.TypeArgs {
		if evt1TypeDependsOnParam(arg, typeParam) {
			return true
		}
	}
	return false
}

func evt1TypesCompatible(env *evt1Env, expected EVT1Type, actual EVT1Type, typeParam string) bool {
	expected = evt1CanonicalType(env, expected.valueType())
	actual = evt1CanonicalType(env, actual.valueType())
	if !evt1TypeDependsOnParam(expected, typeParam) && !evt1TypeDependsOnParam(actual, typeParam) {
		return expected.Equal(actual)
	}
	return evt1SymbolicTypeEqual(expected, actual, typeParam)
}

func evt1SymbolicTypeEqual(a EVT1Type, b EVT1Type, typeParam string) bool {
	if a.Kind == EVT1TypeConceptParam || b.Kind == EVT1TypeConceptParam {
		return a.Kind == b.Kind && a.Name == b.Name
	}
	if (a.PointerTo == nil) != (b.PointerTo == nil) {
		return false
	}
	if a.PointerTo != nil {
		return a.Const == b.Const && evt1SymbolicTypeEqual(*a.PointerTo, *b.PointerTo, typeParam)
	}
	if a.Name != b.Name || a.Kind != b.Kind || a.Ownership != b.Ownership || a.Const != b.Const || a.Imported != b.Imported || a.Unsafe != b.Unsafe || len(a.TypeArgs) != len(b.TypeArgs) {
		return false
	}
	for i := range a.TypeArgs {
		if !evt1SymbolicTypeEqual(a.TypeArgs[i], b.TypeArgs[i], typeParam) {
			return false
		}
	}
	return true
}

func buildTemplateInfo(env *evt1Env, templateDecl EVT1TemplateDecl) (*evt1TemplateInfo, error) {
	info := &evt1TemplateInfo{
		Decl:         templateDecl,
		CallBindings: map[string]evt1TemplateCallBinding{},
	}
	seenConcepts := map[string]bool{}
	seenRequirements := map[string]bool{}
	typeParamType := EVT1Type{Name: templateDecl.TypeParam, Kind: EVT1TypeConceptParam, Span: templateDecl.TypeParamSpan}
	var walk func(conceptName string, path []string) error
	walk = func(conceptName string, path []string) error {
		if !seenConcepts[conceptName] {
			info.Closure = append(info.Closure, evt1TemplateClosureEntry{
				Concept: conceptName,
				Path:    append([]string{}, path...),
			})
			seenConcepts[conceptName] = true
		}
		conceptDecl := env.concepts[conceptName]
		for _, rawReq := range conceptDecl.Requirements {
			switch req := rawReq.(type) {
			case *EVT1PrerequisiteRequirement:
				nextPath := append(append([]string{}, path...), req.ConceptName)
				if err := walk(req.ConceptName, nextPath); err != nil {
					return err
				}
			case *EVT1OperationRequirement:
				substituted := evt1SubstituteRequirement(*req, conceptDecl.TypeParam, typeParamType)
				key := evt1RequirementKey(substituted)
				if seenRequirements[key] {
					continue
				}
				seenRequirements[key] = true
				info.Requirements = append(info.Requirements, evt1TemplateRequirement{
					ID:        fmt.Sprintf("%s.req.%02d", templateDecl.Name, len(info.Requirements)+1),
					Path:      append([]string{}, path...),
					Concept:   conceptName,
					Operation: substituted,
				})
			}
		}
		return nil
	}
	rootPath := []string{templateDecl.Constraint.ConceptName}
	if err := walk(templateDecl.Constraint.ConceptName, rootPath); err != nil {
		return nil, err
	}
	return info, nil
}

func evt1RequirementKey(req EVT1OperationRequirement) string {
	return evt1Signature(req.ReturnType, req.Name, req.Params)
}

func evt1SpanKey(span Span) string {
	return fmt.Sprintf("%d:%d", span.Line, span.Column)
}

func validateTemplateCallExpr(env *evt1Env, scope *evt1Scope, call EVT1CallExpr, templateInfo *evt1TemplateInfo) (EVT1Type, error) {
	argTypes := make([]EVT1Type, 0, len(call.Args))
	dependent := false
	for _, arg := range call.Args {
		argType, err := validateExpr(env, scope, arg, templateInfo)
		if err != nil {
			return EVT1Type{}, err
		}
		if evt1TypeDependsOnParam(argType, templateInfo.Decl.TypeParam) {
			dependent = true
		}
		argTypes = append(argTypes, argType)
	}
	if !dependent {
		fn, err := evt1ResolveOrdinaryCall(env, scope, call.Callee, call.Args, argTypes, templateInfo, call.Span)
		if err != nil {
			if _, exists := env.templates[call.Callee]; exists {
				return EVT1Type{}, evt1Diagnostic("CV4174", "templates cannot invoke templates in EVT1 M1B-B", call.Span)
			}
			return EVT1Type{}, err
		}
		for i, arg := range call.Args {
			if err := validateCallArgument(env, scope, fn.Params[i].Type, arg, argTypes[i], templateInfo); err != nil {
				return EVT1Type{}, err
			}
		}
		return evt1CanonicalType(env, fn.ReturnType), nil
	}
	var matches []evt1TemplateRequirement
	for _, req := range templateInfo.Requirements {
		if req.Operation.Name != call.Callee || len(req.Operation.Params) != len(call.Args) {
			continue
		}
		ok := true
		for i, arg := range call.Args {
			if err := validateCallArgument(env, scope, req.Operation.Params[i].Type, arg, argTypes[i], templateInfo); err != nil {
				ok = false
				break
			}
		}
		if ok {
			matches = append(matches, req)
		}
	}
	if len(matches) == 0 {
		return EVT1Type{}, evt1Diagnostic("CV4176", fmt.Sprintf("template body call %s is not guaranteed by constraint %s", call.Callee, templateInfo.Decl.Constraint.ConceptName), call.Span)
	}
	if len(matches) > 1 {
		return EVT1Type{}, evt1Diagnostic("CV4177", fmt.Sprintf("template body call %s is ambiguously guaranteed by constraint %s", call.Callee, templateInfo.Decl.Constraint.ConceptName), call.Span)
	}
	templateInfo.CallBindings[evt1SpanKey(call.Span)] = evt1TemplateCallBinding{
		CallSpan:    call.Span,
		Requirement: matches[0],
	}
	return evt1CanonicalType(env, matches[0].Operation.ReturnType), nil
}

func instantiateTemplate(env *evt1Env, templateName string, concreteType EVT1Type, span Span) (*evt1TemplateInstance, error) {
	templateDecl, ok := env.templates[templateName]
	if !ok {
		return nil, evt1Diagnostic("CV4178", fmt.Sprintf("unknown template %s", templateName), span)
	}
	if err := validateTemplateTypeArgument(env, concreteType, span); err != nil {
		return nil, err
	}
	concreteType = evt1CanonicalType(env, concreteType)
	key := templateName + "|" + evt1TypeIdentity(concreteType)
	if instance, ok := env.templateInstances[key]; ok {
		return instance, nil
	}
	if err := checkConceptSatisfaction(env, templateDecl.Constraint.ConceptName, concreteType, nil, span); err != nil {
		return nil, err
	}
	info := env.templateInfos[templateName]
	var bindings []evt1InstanceRequirementBinding
	for _, req := range info.Requirements {
		concreteReq := evt1SubstituteRequirement(req.Operation, templateDecl.TypeParam, concreteType)
		fn, err := evt1LookupRequiredOperation(env, concreteReq, span, templateName+"<"+concreteType.String()+">")
		if err != nil {
			return nil, err
		}
		bindings = append(bindings, evt1InstanceRequirementBinding{
			Requirement: req,
			Function:    fn,
		})
	}
	instFn, err := evt1InstantiateTemplateFunction(templateDecl, concreteType)
	if err != nil {
		return nil, err
	}
	if err := validateFunctionSignature(env, instFn); err != nil {
		return nil, err
	}
	scope := newEVT1Scope(nil)
	for _, param := range instFn.Params {
		scope.declare(param.Name, evt1ValueBinding{
			t:       evt1CanonicalType(env, param.Type),
			mutable: !(param.Type.isBorrowLike() && param.Type.Const),
		})
	}
	if err := validateBlock(env, scope, instFn.ReturnType, *instFn.Body, nil); err != nil {
		return nil, err
	}
	instance := &evt1TemplateInstance{
		Key:                 key,
		TemplateName:        templateName,
		ConcreteType:        concreteType,
		TypeIdentity:        evt1TypeIdentity(concreteType),
		GeneratedSymbol:     evt1TemplateInstanceSymbol(templateName, concreteType),
		ConstraintConcept:   templateDecl.Constraint.ConceptName,
		Closure:             append([]evt1TemplateClosureEntry{}, info.Closure...),
		RequirementBindings: bindings,
		Function:            instFn,
		SourceSpan:          templateDecl.Span,
	}
	env.templateInstances[key] = instance
	return instance, nil
}

func validateTemplateTypeArgument(env *evt1Env, concreteType EVT1Type, span Span) error {
	if concreteType.PointerTo != nil || concreteType.Ownership != "" || concreteType.Const || concreteType.Imported || concreteType.Unsafe || len(concreteType.TypeArgs) > 0 || concreteType.Kind == EVT1TypeConceptParam {
		return evt1Diagnostic("CV4179", "template calls require one concrete non-template type argument", span)
	}
	return validateKnownType(env, concreteType, span, "", false)
}

func evt1TypeIdentity(t EVT1Type) string {
	if t.PointerTo != nil {
		return "ptr_" + evt1TypeIdentity(*t.PointerTo)
	}
	if len(t.TypeArgs) > 0 {
		var parts []string
		for _, arg := range t.TypeArgs {
			parts = append(parts, evt1TypeIdentity(arg))
		}
		return evt1CName(t.Name)[len("concept_vulkan_"):] + "_" + strings.Join(parts, "_")
	}
	return evt1CName(t.Name)[len("concept_vulkan_"):]
}

func evt1TemplateInstanceSymbol(templateName string, concreteType EVT1Type) string {
	return "concept_vulkan_template_" + evt1CName(templateName)[len("concept_vulkan_"):] + "__" + evt1TypeIdentity(concreteType)
}

func evt1InstantiateTemplateFunction(templateDecl EVT1TemplateDecl, concreteType EVT1Type) (EVT1FunctionDecl, error) {
	body, err := evt1SubstituteBlock(*templateDecl.Body, templateDecl.TypeParam, concreteType)
	if err != nil {
		return EVT1FunctionDecl{}, err
	}
	fn := EVT1FunctionDecl{
		Name:       templateDecl.Name,
		ReturnType: evt1SubstituteType(templateDecl.ReturnType, templateDecl.TypeParam, concreteType),
		Span:       templateDecl.Span,
		Body:       &body,
	}
	for _, param := range templateDecl.Params {
		fn.Params = append(fn.Params, EVT1Param{
			Type: evt1SubstituteType(param.Type, templateDecl.TypeParam, concreteType),
			Name: param.Name,
			Span: param.Span,
		})
	}
	return fn, nil
}

func evt1SubstituteBlock(block EVT1Block, typeParam string, concreteType EVT1Type) (EVT1Block, error) {
	out := EVT1Block{Span: block.Span}
	for _, stmt := range block.Statements {
		sub, err := evt1SubstituteStatement(stmt, typeParam, concreteType)
		if err != nil {
			return EVT1Block{}, err
		}
		out.Statements = append(out.Statements, sub)
	}
	return out, nil
}

func evt1SubstituteStatement(stmt EVT1Statement, typeParam string, concreteType EVT1Type) (EVT1Statement, error) {
	switch s := stmt.(type) {
	case *EVT1VarDecl:
		value, err := evt1SubstituteExpr(s.Value, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		return &EVT1VarDecl{Type: evt1SubstituteType(s.Type, typeParam, concreteType), Name: s.Name, Value: value, Span: s.Span}, nil
	case *EVT1AssignStmt:
		target, err := evt1SubstituteExpr(s.Target, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		value, err := evt1SubstituteExpr(s.Value, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		return &EVT1AssignStmt{Target: target, Value: value, Span: s.Span}, nil
	case *EVT1ReturnStmt:
		if s.Value == nil {
			return &EVT1ReturnStmt{Span: s.Span}, nil
		}
		value, err := evt1SubstituteExpr(s.Value, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		return &EVT1ReturnStmt{Value: value, Span: s.Span}, nil
	case *EVT1ExprStmt:
		value, err := evt1SubstituteExpr(s.Value, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		return &EVT1ExprStmt{Value: value, Span: s.Span}, nil
	case *EVT1MatchStmt:
		subject, err := evt1SubstituteExpr(s.Subject, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		out := &EVT1MatchStmt{Subject: subject, Span: s.Span}
		for _, arm := range s.Arms {
			block, err := evt1SubstituteBlock(arm.Block, typeParam, concreteType)
			if err != nil {
				return nil, err
			}
			out.Arms = append(out.Arms, EVT1StatementArm{Pattern: arm.Pattern, Block: block, Span: arm.Span})
		}
		return out, nil
	case *EVT1Block:
		block, err := evt1SubstituteBlock(*s, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		return &block, nil
	default:
		return nil, evt1Diagnostic("CV4180", "unsupported template statement during instantiation", stmt.statementSpan())
	}
}

func evt1SubstituteExpr(expr EVT1Expr, typeParam string, concreteType EVT1Type) (EVT1Expr, error) {
	switch e := expr.(type) {
	case *EVT1NameExpr:
		return &EVT1NameExpr{Name: e.Name, Span: e.Span}, nil
	case *EVT1IntLiteral:
		return &EVT1IntLiteral{Value: e.Value, Span: e.Span}, nil
	case *EVT1BoolLiteral:
		return &EVT1BoolLiteral{Value: e.Value, Span: e.Span}, nil
	case *EVT1FieldExpr:
		receiver, err := evt1SubstituteExpr(e.Receiver, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		return &EVT1FieldExpr{Receiver: receiver, Field: e.Field, Span: e.Span}, nil
	case *EVT1CallExpr:
		out := &EVT1CallExpr{Callee: e.Callee, Span: e.Span}
		for _, arg := range e.Args {
			sub, err := evt1SubstituteExpr(arg, typeParam, concreteType)
			if err != nil {
				return nil, err
			}
			out.Args = append(out.Args, sub)
		}
		return out, nil
	case *EVT1TemplateCallExpr:
		return nil, evt1Diagnostic("CV4174", "templates cannot invoke templates in EVT1 M1B-B", e.Span)
	case *EVT1BinaryExpr:
		left, err := evt1SubstituteExpr(e.Left, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		right, err := evt1SubstituteExpr(e.Right, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		return &EVT1BinaryExpr{Op: e.Op, Left: left, Right: right, Span: e.Span}, nil
	case *EVT1ConstructExpr:
		out := &EVT1ConstructExpr{EnumName: e.EnumName, VariantName: e.VariantName, Span: e.Span}
		for _, arg := range e.Args {
			sub, err := evt1SubstituteExpr(arg, typeParam, concreteType)
			if err != nil {
				return nil, err
			}
			out.Args = append(out.Args, sub)
		}
		return out, nil
	case *EVT1StructConstructExpr:
		out := &EVT1StructConstructExpr{StructName: e.StructName, Span: e.Span}
		for _, arg := range e.Args {
			sub, err := evt1SubstituteExpr(arg, typeParam, concreteType)
			if err != nil {
				return nil, err
			}
			out.Args = append(out.Args, sub)
		}
		return out, nil
	case *EVT1MatchExpr:
		subject, err := evt1SubstituteExpr(e.Subject, typeParam, concreteType)
		if err != nil {
			return nil, err
		}
		out := &EVT1MatchExpr{Subject: subject, Span: e.Span}
		for _, arm := range e.Arms {
			value, err := evt1SubstituteExpr(arm.Value, typeParam, concreteType)
			if err != nil {
				return nil, err
			}
			out.Arms = append(out.Arms, EVT1ExprArm{Pattern: arm.Pattern, Value: value, Span: arm.Span})
		}
		return out, nil
	default:
		return nil, evt1Diagnostic("CV4181", "unsupported template expression during instantiation", expr.exprSpan())
	}
}
