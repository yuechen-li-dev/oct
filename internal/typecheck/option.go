package typecheck

import (
	"fmt"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

// `Option<T>` is the builtin enum with the variants `None` and `Some(T)`.
//
// The checker names a type by a string, and an option type is named
// `Option<` + the name of its payload type + `>`. The payload type cannot be
// read back out of that name, because a name does not say whether it is a
// record, an enum or a refined concept, so every option type the checker
// forms is recorded in checker.options, name to payload type. Everything else
// that the language says of an enum follows from lookupEnum answering for an
// option name with the two variants.

const optionTypePrefix = ast.OptionTypeName + "<"

// isOptionTypeName reports whether a type name is that of an option type.
func isOptionTypeName(name string) bool {
	return strings.HasPrefix(name, optionTypePrefix) && strings.HasSuffix(name, ">")
}

// optionType is the type `Option<payload>`.
func (c checker) optionType(payload Type) Type {
	name := optionTypePrefix + payload.String() + ">"
	c.options[name] = payload
	return Type{Name: name}
}

// optionPayload is T for the type `Option<T>`. An array of options is not an
// option.
func (c checker) optionPayload(t Type) (Type, bool) {
	if t.IsArray || !isOptionTypeName(t.Name) {
		return Type{}, false
	}
	payload, ok := c.options[t.Name]
	return payload, ok
}

// optionEnumInfo is an option type seen as the enum it is.
func (c checker) optionEnumInfo(typeName string) (enumInfo, bool) {
	payload, ok := c.options[typeName]
	if !ok {
		return enumInfo{}, false
	}
	return enumInfo{variants: map[string]enumVariantInfo{
		ast.OptionNoneVariant: {},
		ast.OptionSomeVariant: {payload: &payload},
	}}, true
}

// resolveOptionType resolves the written type `Option<T>`, at any array depth.
func (c checker) resolveOptionType(typeRef ast.TypeRef) (Type, error) {
	if len(typeRef.TypeArguments) != 1 {
		return Type{}, fmt.Errorf("Option takes one type argument, got %d", len(typeRef.TypeArguments))
	}
	argument := typeRef.TypeArguments[0]
	if argument.Package == "" && argument.Name == string(BaseTypeVoid) && !argument.IsArray {
		return Type{}, fmt.Errorf("Option<Void> is not a type: `Option.Some` holds a value, and Void has none")
	}
	payload, err := c.resolveType(argument, false)
	if err != nil {
		return Type{}, err
	}
	arrayDepth := typeRef.ArrayDepth
	if typeRef.IsArray && arrayDepth == 0 {
		arrayDepth = 1
	}
	return withArrayDepth(c.optionType(payload), arrayDepth), nil
}

// qualifyImportedOptionType names, as the checked package names it, an option
// type that package pkgName names: the payload type is qualified, and the
// option of that is formed here.
func (c checker) qualifyImportedOptionType(pkgName string, valueType Type) Type {
	payload, ok := c.importedPackages[pkgName].options[valueType.Name]
	if !ok {
		return valueType
	}
	qualified := c.optionType(c.qualifyImportedType(pkgName, payload))
	return withArrayDepth(qualified, valueType.ArrayDepth)
}

// optionExpected is expected when that is an option type or an array of them,
// and nil otherwise. It is how the type a site declares reaches an
// `Option.None` or `Option.Some(value)` written there through the expressions
// that only pass a value along, without changing how those expressions check
// anything else.
func (c checker) optionExpected(expected *Type) *Type {
	if expected == nil || !isOptionTypeName(expected.Name) {
		return nil
	}
	return expected
}

// isUntypedOptionConstruction reports whether expr is `Option.None` or
// `Option.Some(value)` with no type argument, looking through parentheses.
func isUntypedOptionConstruction(expr ast.Expr) bool {
	for {
		paren, ok := expr.(ast.ParenExpr)
		if !ok {
			break
		}
		expr = paren.Inner
	}
	construction, ok := ast.AsOptionConstruction(expr)
	return ok && construction.Payload.Inferred
}

// checkOptionConstruction types `Option.None`, `Option.Some(value)` and the
// two with a written type argument. The type argument is the one written, or
// else the one the site declares; it is never worked out from the payload,
// because `Option.None` has none and the two variants must read alike.
//
// Where the type argument comes from the site, it is written into the
// construction (see ast.AsOptionConstruction), so the interpreter and the
// compiled lane read it there and do not work it out a second and a third
// time. This is the one thing the typechecker tells the passes after it.
func (c checker) checkOptionConstruction(scope *scope, node ast.CallExpr, construction ast.OptionConstruction, ctx functionContext, expected *Type) (ExprType, error) {
	spelling := "Option." + construction.Variant
	switch construction.Variant {
	case ast.OptionNoneVariant:
	case ast.OptionSomeVariant:
		spelling += "(...)"
	default:
		return ExprType{}, fmt.Errorf("enum 'Option' has no variant '%s'; its variants are `Option.None` and `Option.Some(value)`", construction.Variant)
	}

	var optionType Type
	switch {
	case !construction.Payload.Inferred:
		resolved, err := c.resolveOptionType(ast.TypeRef{Name: ast.OptionTypeName, TypeArguments: node.TypeArguments})
		if err != nil {
			return ExprType{}, err
		}
		optionType = resolved
	case expected != nil:
		if _, ok := c.optionPayload(*expected); !ok {
			return ExprType{}, fmt.Errorf("expected %s, got `%s`", *expected, spelling)
		}
		optionType = *expected
	default:
		typed := "Option<T>." + construction.Variant
		if construction.Variant == ast.OptionSomeVariant {
			typed += "(...)"
		}
		return ExprType{}, fmt.Errorf("`%s` does not say which Option it is, and nothing here declares it; declare the type where the value goes (`let x: Option<T> = %s`), or write `%s`", spelling, spelling, typed)
	}
	payload, _ := c.optionPayload(optionType)
	if construction.Payload.Inferred {
		written, err := c.typeRefOf(payload)
		if err != nil {
			return ExprType{}, fmt.Errorf("`%s`: %w", spelling, err)
		}
		written.Inferred = true
		node.TypeArguments[0] = written
	}

	if construction.Variant == ast.OptionNoneVariant {
		return ExprType{ValueType: optionType}, nil
	}
	if len(construction.Arguments) != 1 {
		return ExprType{}, fmt.Errorf("enum 'Option' variant 'Some' requires exactly 1 payload argument")
	}
	argument := construction.Arguments[0]
	payloadType, err := c.checkExprWithExpected(scope, argument, ctx, &payload)
	if err != nil {
		return ExprType{}, err
	}
	if payloadType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if !isAssignable(payloadType.ValueType, payload) {
		if !c.isRefinedExpected(payload) {
			return ExprType{}, fmt.Errorf("enum '%s' variant 'Some' payload expects %s, got %s", optionType, payload, payloadType.ValueType)
		}
		if err := c.admitRefined(scope, argument, payloadType.ValueType, payload); err != nil {
			return ExprType{}, fmt.Errorf("enum '%s' variant 'Some' payload: %w", optionType, err)
		}
	}
	return ExprType{ValueType: optionType}, nil
}

// typeRefOf writes a type as the source of the checked package would. It
// serves the one place where the typechecker hands a type on: the type
// argument of an Option construction that the source left to the site.
func (c checker) typeRefOf(t Type) (ast.TypeRef, error) {
	if t.IsArray {
		element, err := c.typeRefOf(withArrayDepth(t, 0))
		if err != nil {
			return ast.TypeRef{}, err
		}
		element.IsArray = true
		element.ArrayDepth = t.ArrayDepth
		return element, nil
	}
	switch {
	case t.Tuple != nil, t.IsFlowInstance:
		// Neither has a type syntax, so neither can be the T of an option.
		return ast.TypeRef{}, fmt.Errorf("internal error: the type %s has no written form", t)
	case t.IsFunction:
		signature, ok := c.functionTypes[t.FunctionSignature]
		if !ok {
			return ast.TypeRef{}, fmt.Errorf("internal error: missing function type metadata for %s", t.FunctionSignature)
		}
		function := ast.FunctionTypeRef{IsFallible: signature.isFallible}
		for _, parameterType := range signature.parameters {
			parameter, err := c.typeRefOf(parameterType)
			if err != nil {
				return ast.TypeRef{}, err
			}
			function.Parameters = append(function.Parameters, parameter)
		}
		result, err := c.typeRefOf(signature.returnType)
		if err != nil {
			return ast.TypeRef{}, err
		}
		function.ReturnType = result
		if signature.isFallible {
			function.ErrorType = &ast.TypeRef{Name: string(BaseTypeError)}
		}
		return ast.TypeRef{Function: &function}, nil
	case t.IsVector || t.IsMatrix:
		element := ast.TypeRef{Name: string(t.Base), Dimension: t.Dimension, HasUnit: !t.Dimension.IsDimensionless()}
		if t.IsVector {
			return ast.TypeRef{VectorOf: &element}, nil
		}
		return ast.TypeRef{MatrixOf: &element}, nil
	}
	if payload, ok := c.optionPayload(t); ok {
		argument, err := c.typeRefOf(payload)
		if err != nil {
			return ast.TypeRef{}, err
		}
		return ast.TypeRef{Name: ast.OptionTypeName, TypeArguments: []ast.TypeRef{argument}}, nil
	}
	if t.Name != "" {
		// A record, an enum or a refined concept, by the name the checked
		// package knows it by.
		if pkgName, localName, qualified := splitQualifiedTypeName(t.Name); qualified {
			return ast.TypeRef{Package: pkgName, Name: localName}, nil
		}
		return ast.TypeRef{Name: t.Name}, nil
	}
	return ast.TypeRef{Name: string(t.Base), Dimension: t.Dimension, HasUnit: !t.Dimension.IsDimensionless()}, nil
}

// checkOptionCaseLabel types the switch case label `Option.<variant>`. Like
// any enum label it names a variant without a payload, which for an Option
// is `None`.
func (c checker) checkOptionCaseLabel(variant string, subjectType Type) (Type, error) {
	switch variant {
	case ast.OptionNoneVariant:
	case ast.OptionSomeVariant:
		return Type{}, fmt.Errorf("enum 'Option' variant 'Some' requires one payload argument")
	default:
		return Type{}, fmt.Errorf("enum 'Option' has no variant '%s'; its variants are `Option.None` and `Option.Some(value)`", variant)
	}
	if _, ok := c.optionPayload(subjectType); !ok {
		return Type{}, fmt.Errorf("case type Option does not match subject type %s", subjectType)
	}
	return subjectType, nil
}

// checkOptionUtilityCandidate checks a candidate of `when utility Option<T>`.
// The target is the type the candidate is declared to be, so `Option.None`
// and `Option.Some(value)` take T from it.
func (c checker) checkOptionUtilityCandidate(scope *scope, ctx functionContext, target Type, expr ast.Expr, arm string) error {
	if _, ok := ast.AsOptionConstruction(expr); !ok {
		return fmt.Errorf("utility enum cases must use qualified variants, e.g. Option.None")
	}
	candidate, err := c.checkExprWithExpected(scope, expr, ctx, &target)
	if err != nil {
		return err
	}
	if candidate.Fallible {
		return fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if candidate.ValueType != target {
		return fmt.Errorf("utility %s for %s cannot return %s", arm, target, candidate.ValueType)
	}
	return nil
}
