package typecheck

import (
	"fmt"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/builtin"
	"github.com/yuechen-li-dev/oct/internal/jsontype"
	"github.com/yuechen-li-dev/oct/internal/octjson"
)

// checkJsonCall types a call to a Json builtin from its entry in the builtin
// table: `Json.Load<T>(path)` and `Json.Parse<T>(text)`. T is any type with a
// JSON form, and octjson says whether it has one. The result is a T, and the
// call is fallible.
func (c checker) checkJsonCall(scope *scope, json builtin.JsonBuiltin, typeArguments []ast.TypeRef, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	name := json.Name()
	if len(typeArguments) != 1 {
		return ExprType{}, fmt.Errorf("function '%s' expects 1 type argument, the type to read, got %d", name, len(typeArguments))
	}
	if len(arguments) != 1 {
		return ExprType{}, fmt.Errorf("function '%s' expects 1 argument, got %d", name, len(arguments))
	}
	readType, err := c.resolveNonReturnType(typeArguments[0])
	if err != nil {
		return ExprType{}, err
	}
	if err := octjson.Check(jsontype.Of(c.program, c.packageName, typeArguments[0])); err != nil {
		return ExprType{}, fmt.Errorf("function '%s' cannot read %s: %w", name, readType, err)
	}
	argumentType, err := c.checkExpr(scope, arguments[0], ctx)
	if err != nil {
		return ExprType{}, err
	}
	if argumentType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if argumentType.ValueType != (Type{Base: BaseTypeString}) {
		return ExprType{}, fmt.Errorf("function '%s' argument 1 expects String, got %s", name, argumentType.ValueType)
	}
	return ExprType{ValueType: readType, Fallible: true}, nil
}
