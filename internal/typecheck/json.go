package typecheck

import (
	"fmt"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/builtin"
	"github.com/yuechen-li-dev/oct/internal/jsontype"
	"github.com/yuechen-li-dev/oct/internal/octjson"
)

// checkJsonCall types a call to a Json builtin from its entry in the builtin
// table. Every call is made at a type T with a JSON form, and octjson says
// whether a type has one.
func (c checker) checkJsonCall(scope *scope, json builtin.JsonBuiltin, call ast.CallExpr, ctx functionContext) (ExprType, error) {
	if json.Writes() {
		return c.checkJsonWrite(scope, json, call, ctx)
	}
	return c.checkJsonRead(scope, json, call, ctx)
}

// checkJsonRead types `Json.Load<T>(path)` and `Json.Parse<T>(text)`. T is
// written; the result is a T, and the call is fallible.
func (c checker) checkJsonRead(scope *scope, json builtin.JsonBuiltin, call ast.CallExpr, ctx functionContext) (ExprType, error) {
	name := json.Name()
	if len(call.TypeArguments) != 1 {
		return ExprType{}, fmt.Errorf("function '%s' expects 1 type argument, the type to read, got %d", name, len(call.TypeArguments))
	}
	if len(call.Arguments) != 1 {
		return ExprType{}, fmt.Errorf("function '%s' expects 1 argument, got %d", name, len(call.Arguments))
	}
	readType, err := c.resolveNonReturnType(call.TypeArguments[0])
	if err != nil {
		return ExprType{}, err
	}
	if err := octjson.Check(jsontype.Of(c.program, c.packageName, call.TypeArguments[0])); err != nil {
		return ExprType{}, fmt.Errorf("function '%s' cannot read %s: %w", name, readType, err)
	}
	if err := c.checkJsonStringArgument(scope, name, call.Arguments[0], 1, ctx); err != nil {
		return ExprType{}, err
	}
	return ExprType{ValueType: readType, Fallible: true}, nil
}

// checkJsonWrite types `Json.Save(path, value)`, `Json.Text(value)` and
// `Artifact.WriteJson(path, value)`. T is the type of the value. The lanes
// need it and cannot work it out, so it is written into the type-argument
// slot the parser made for the call (ast.CallType).
func (c checker) checkJsonWrite(scope *scope, json builtin.JsonBuiltin, call ast.CallExpr, ctx functionContext) (ExprType, error) {
	name := json.Name()
	if len(call.TypeArguments) != 1 || !call.TypeArguments[0].Inferred {
		return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments; it writes the type of its value", name)
	}
	parameters := 2
	if json.Action == builtin.JsonWriteText {
		parameters = 1
	}
	if len(call.Arguments) != parameters {
		noun := "arguments"
		if parameters == 1 {
			noun = "argument"
		}
		return ExprType{}, fmt.Errorf("function '%s' expects %d %s, got %d", name, parameters, noun, len(call.Arguments))
	}
	valueExpr := call.Arguments[parameters-1]
	value, err := c.checkExpr(scope, valueExpr, ctx)
	if err != nil {
		return ExprType{}, err
	}
	if value.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if parameters == 2 {
		if err := c.checkJsonStringArgument(scope, name, call.Arguments[0], 1, ctx); err != nil {
			return ExprType{}, err
		}
	}
	written, err := c.typeRefOf(value.ValueType)
	if err != nil {
		return ExprType{}, fmt.Errorf("function '%s' cannot write %s: it has no JSON form", name, value.ValueType)
	}
	if err := octjson.Check(jsontype.Of(c.program, c.packageName, written)); err != nil {
		return ExprType{}, fmt.Errorf("function '%s' cannot write %s: %w", name, value.ValueType, err)
	}
	written.Inferred = true
	call.TypeArguments[0] = written

	switch json.Action {
	case builtin.JsonWriteFile:
		return ExprType{ValueType: Type{Base: BaseTypeVoid}, Fallible: true}, nil
	case builtin.JsonWriteText:
		return ExprType{ValueType: Type{Base: BaseTypeString}}, nil
	default:
		return ExprType{ValueType: Type{Base: BaseTypeVoid}}, nil
	}
}

// checkJsonStringArgument checks the path or the text of a Json call.
func (c checker) checkJsonStringArgument(scope *scope, name string, argument ast.Expr, position int, ctx functionContext) error {
	argumentType, err := c.checkExpr(scope, argument, ctx)
	if err != nil {
		return err
	}
	if argumentType.Fallible {
		return fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if argumentType.ValueType != (Type{Base: BaseTypeString}) {
		return fmt.Errorf("function '%s' argument %d expects String, got %s", name, position, argumentType.ValueType)
	}
	return nil
}
