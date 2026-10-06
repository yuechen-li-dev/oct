package interpret

import (
	"fmt"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

// `Option<T>` is the builtin enum `None | Some(T)`. A value of it is an enum
// value whose type name is `Option`: the interpreter has no static types, and
// nothing it does with an enum value depends on T once the payload is the
// declared kind. T is at hand where it matters, at the construction, because
// the typechecker writes it there (ast.AsOptionConstruction).

// evalOptionConstruction evaluates `Option<T>.None` and `Option<T>.Some(x)`.
func (i interpreter) evalOptionConstruction(env *environment, pkgName string, construction ast.OptionConstruction) (evalResult, error) {
	if !construction.Resolved {
		return evalResult{}, fmt.Errorf("runtime invariant violation: `Option.%s` reached the interpreter without the type the typechecker gives it", construction.Variant)
	}
	switch construction.Variant {
	case ast.OptionNoneVariant:
		// The parser gives `None` no argument list.
		return evalResult{value: Value{Kind: ValueEnum, Enum: EnumValue{TypeName: ast.OptionTypeName, Variant: ast.OptionNoneVariant}}}, nil
	case ast.OptionSomeVariant:
		if len(construction.Arguments) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: enum 'Option' variant 'Some' requires exactly 1 payload argument")
		}
		payload, err := i.evalExpr(env, pkgName, construction.Arguments[0])
		if err != nil {
			return evalResult{}, err
		}
		if payload.hasError {
			return evalResult{hasError: true, errorVal: payload.errorVal}, nil
		}
		payloadValue := i.conform(payload.value, construction.Payload, pkgName)
		return evalResult{value: Value{Kind: ValueEnum, Enum: EnumValue{TypeName: ast.OptionTypeName, Variant: ast.OptionSomeVariant, Payload: &payloadValue}}}, nil
	default:
		return evalResult{}, fmt.Errorf("runtime invariant violation: enum 'Option' has no variant '%s'", construction.Variant)
	}
}

// optionEnumDecl is `Option<payload>` as the declaration it would have.
func optionEnumDecl(payload ast.TypeRef) ast.EnumDecl {
	return ast.EnumDecl{Name: ast.OptionTypeName, Variants: []ast.EnumVariantDecl{
		{Name: ast.OptionNoneVariant},
		{Name: ast.OptionSomeVariant, Payload: &payload},
	}}
}

// lookupEnumDeclOf is lookupEnumDecl for a type that is at hand as written:
// it also answers for `Option<T>`, with T as the payload of `Some`.
func (i interpreter) lookupEnumDeclOf(currentPackage string, declared ast.TypeRef) (ast.EnumDecl, string, bool) {
	if declared.Package == "" && declared.Name == ast.OptionTypeName && len(declared.TypeArguments) == 1 {
		return optionEnumDecl(declared.TypeArguments[0]), ast.OptionTypeName, true
	}
	return i.lookupEnumDecl(currentPackage, qualifiedTypeRefName(declared))
}

// materializeOctagonOption loads `Option.None` or `Option.Some(value)` as the
// declared `Option<T>`; the payload is loaded as T.
func (i interpreter) materializeOctagonOption(currentPkg string, expectedType ast.TypeRef, expr ast.Expr) (Value, error) {
	construction, ok := ast.AsOptionConstruction(expr)
	if !ok {
		return Value{}, fmt.Errorf("expected %s, got %s", expectedTypeString(expectedType), octagonDataDescription(expr))
	}
	switch construction.Variant {
	case ast.OptionNoneVariant:
		return Value{Kind: ValueEnum, Enum: EnumValue{TypeName: ast.OptionTypeName, Variant: ast.OptionNoneVariant}}, nil
	case ast.OptionSomeVariant:
		if len(construction.Arguments) != 1 {
			return Value{}, fmt.Errorf("enum %s variant Some requires exactly 1 payload argument, got %d", expectedTypeString(expectedType), len(construction.Arguments))
		}
		payload, err := i.materializeOctagonValue(currentPkg, expectedType.TypeArguments[0], construction.Arguments[0])
		if err != nil {
			return Value{}, fmt.Errorf("enum %s variant Some payload mismatch: %w", expectedTypeString(expectedType), err)
		}
		return Value{Kind: ValueEnum, Enum: EnumValue{TypeName: ast.OptionTypeName, Variant: ast.OptionSomeVariant, Payload: &payload}}, nil
	default:
		return Value{}, fmt.Errorf("enum %s has no variant %s", expectedTypeString(expectedType), construction.Variant)
	}
}

// octagonDataDescription names the kind of a data value for a mismatch
// message, where the Go type of its syntax node would say nothing to the
// reader of the file.
func octagonDataDescription(expr ast.Expr) string {
	switch node := expr.(type) {
	case ast.IntegerLiteral:
		return "the Int " + node.Value
	case ast.FloatLiteral:
		return "the Float " + node.Value
	case ast.BoolLiteral:
		return "a Bool"
	case ast.StringLiteralExpr:
		return "a String"
	case ast.ArrayLiteralExpr:
		return "an array"
	case ast.RecordLiteralExpr:
		return "a record"
	case ast.FieldAccessExpr, ast.CallExpr:
		return "an enum value of another type"
	default:
		return fmt.Sprintf("%T", expr)
	}
}
