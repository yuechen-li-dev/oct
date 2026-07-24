package conceptvulkan

import (
	"fmt"
	"strings"
)

type evt1Scope struct {
	parent *evt1Scope
	values map[string]EVT1Type
}

func newEVT1Scope(parent *evt1Scope) *evt1Scope {
	return &evt1Scope{parent: parent, values: map[string]EVT1Type{}}
}

func (s *evt1Scope) declare(name string, t EVT1Type) {
	s.values[name] = t
}

func (s *evt1Scope) lookup(name string) (EVT1Type, bool) {
	for scope := s; scope != nil; scope = scope.parent {
		if t, ok := scope.values[name]; ok {
			return t, true
		}
	}
	return EVT1Type{}, false
}

func validateEVT1Module(module EVT1Module) error {
	env := newEVT1Env()
	for _, enumDecl := range module.Enums {
		if _, exists := env.enums[enumDecl.Name]; exists {
			return evt1Diagnostic("CV4101", fmt.Sprintf("duplicate enum declaration %s", enumDecl.Name), enumDecl.Span)
		}
		seen := map[string]bool{}
		for i, variant := range enumDecl.Variants {
			if seen[variant.Name] {
				return evt1Diagnostic("CV4100", fmt.Sprintf("duplicate variant %s::%s", enumDecl.Name, variant.Name), variant.Span)
			}
			seen[variant.Name] = true
			enumDecl.Variants[i].Tag = i
			for _, field := range variant.Payload {
				if field.Type.Qualifier == "owned" {
					return evt1Diagnostic("CV4119", fmt.Sprintf("owned payloads are not yet supported in enum variant %s::%s", enumDecl.Name, variant.Name), field.Span)
				}
			}
		}
		env.enums[enumDecl.Name] = enumDecl
	}
	for _, fn := range module.Functions {
		if _, exists := env.functions[fn.Name]; exists {
			return evt1Diagnostic("CV4021", fmt.Sprintf("duplicate function declaration %s", fn.Name), fn.Span)
		}
		env.functions[fn.Name] = fn
	}
	for _, fn := range module.Functions {
		if fn.Body == nil {
			continue
		}
		scope := newEVT1Scope(nil)
		for _, param := range fn.Params {
			scope.declare(param.Name, param.Type)
		}
		collectEscapedArmBindings(fn.Body, env)
		if err := validateBlock(env, scope, fn.ReturnType, *fn.Body); err != nil {
			return err
		}
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
			if err := validateKnownType(env, s.Type, s.Span); err != nil {
				return err
			}
			valueType, err := validateExpr(env, local, s.Value)
			if err != nil {
				return err
			}
			if !s.Type.Equal(valueType) {
				return evt1Diagnostic("CV4106", fmt.Sprintf("constructor or initializer for %s expected %s but got %s", s.Name, s.Type.String(), valueType.String()), s.Value.exprSpan())
			}
			local.declare(s.Name, s.Type)
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
			if !returnType.Equal(valueType) {
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

func validateKnownType(env *evt1Env, t EVT1Type, span Span) error {
	if t.PointerTo != nil {
		return validateKnownType(env, *t.PointerTo, span)
	}
	if _, ok := evt1BuiltinType(t.Name, span); ok {
		return nil
	}
	if _, ok := env.enums[t.Name]; ok {
		return nil
	}
	return evt1Diagnostic("CV4102", fmt.Sprintf("unknown enum or type %s", t.Name), span)
}

func validateExpr(env *evt1Env, scope *evt1Scope, expr EVT1Expr) (EVT1Type, error) {
	switch e := expr.(type) {
	case *EVT1IntLiteral:
		t, _ := evt1BuiltinType("int", e.Span)
		return t, nil
	case *EVT1NameExpr:
		if t, ok := scope.lookup(e.Name); ok {
			return t, nil
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
		fields, ok := env.recordFields[receiverType.Name]
		if !ok {
			return EVT1Type{}, evt1Diagnostic("CV4025", fmt.Sprintf("type %s has no fields", receiverType.Name), e.Span)
		}
		fieldType, ok := fields[e.Field]
		if !ok {
			return EVT1Type{}, evt1Diagnostic("CV4026", fmt.Sprintf("unknown field %s on %s", e.Field, receiverType.Name), e.Span)
		}
		return fieldType, nil
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
			if !fn.Params[i].Type.Equal(argType) {
				return EVT1Type{}, evt1Diagnostic("CV4107", fmt.Sprintf("wrong payload type for %s argument %d: expected %s but got %s", e.Callee, i+1, fn.Params[i].Type.String(), argType.String()), arg.exprSpan())
			}
		}
		return fn.ReturnType, nil
	case *EVT1BinaryExpr:
		leftType, err := validateExpr(env, scope, e.Left)
		if err != nil {
			return EVT1Type{}, err
		}
		rightType, err := validateExpr(env, scope, e.Right)
		if err != nil {
			return EVT1Type{}, err
		}
		if leftType.Name != "int" || rightType.Name != "int" {
			return EVT1Type{}, evt1Diagnostic("CV4028", "only int + int is supported in EVT1 expressions", e.Span)
		}
		return leftType, nil
	case *EVT1ConstructExpr:
		return validateConstructExpr(env, scope, *e)
	case *EVT1MatchExpr:
		return validateMatchExpr(env, scope, *e)
	default:
		return EVT1Type{}, evt1Diagnostic("CV4029", fmt.Sprintf("unsupported expression %s", evt1Unexpected(expr)), expr.exprSpan())
	}
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
		if !variant.Payload[i].Type.Equal(argType) {
			return EVT1Type{}, evt1Diagnostic("CV4107", fmt.Sprintf("wrong constructor payload type for %s::%s position %d: expected %s but got %s", expr.EnumName, expr.VariantName, i+1, variant.Payload[i].Type.String(), argType.String()), arg.exprSpan())
		}
	}
	return EVT1Type{Name: enumDecl.Name, Kind: EVT1TypeEnum, Span: expr.Span}, nil
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
			resultType = valueType
		} else if !resultType.Equal(valueType) {
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
	enumDecl := env.enums[subjectType.Name]
	return subjectType, enumDecl, nil
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
		armScope.declare(binding, variant.Payload[i].Type)
	}
	return armScope, variant, nil
}
