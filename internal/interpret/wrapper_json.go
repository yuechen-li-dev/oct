package interpret

import (
	"bytes"
	"encoding/json"

	"oct/internal/ast"
)

func jsonWrapperBuiltins() map[string]wrapperBuiltinHandler {
	return map[string]wrapperBuiltinHandler{
		"JsonNormalize": (*interpreter).evalJSONNormalizeBuiltin,
	}
}

func (i *interpreter) evalJSONNormalizeBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(1); err != nil {
		return evalResult{}, err
	}
	input, errResult, err := call.stringArg(0)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(input)); err != nil {
		return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorInvalidData, "%v", err)), nil
	}
	return wrapperStringResult(compact.String()), nil
}
