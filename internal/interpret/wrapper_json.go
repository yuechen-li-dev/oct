package interpret

import (
	"bytes"
	"encoding/json"
	"os"

	"oct/internal/ast"
)

func jsonWrapperBuiltins() map[string]wrapperBuiltinHandler {
	return map[string]wrapperBuiltinHandler{
		"JsonNormalize": (*interpreter).evalJSONNormalizeBuiltin,
		"JsonParse":     (*interpreter).evalJSONParseBuiltin,
		"JsonStringify": (*interpreter).evalJSONStringifyBuiltin,
		"JsonLoad":      (*interpreter).evalJSONLoadBuiltin,
		"JsonSave":      (*interpreter).evalJSONSaveBuiltin,
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
		return wrapperErrorResult(callee, mapJSONError(err)), nil
	}
	return wrapperStringResult(compact.String()), nil
}

func (i *interpreter) evalJSONParseBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	return i.evalJSONNormalizeBuiltin(env, pkgName, callee, argumentExprs)
}

func (i *interpreter) evalJSONStringifyBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	return i.evalJSONNormalizeBuiltin(env, pkgName, callee, argumentExprs)
}

func (i *interpreter) evalJSONLoadBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(1); err != nil {
		return evalResult{}, err
	}
	path, errResult, err := call.stringArg(0)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	contents, readErr := os.ReadFile(path)
	if readErr != nil {
		return wrapperErrorResult(callee, mapPathError(path, readErr)), nil
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, contents); err != nil {
		return wrapperErrorResult(callee, mapJSONError(err)), nil
	}
	return wrapperStringResult(compact.String()), nil
}

func (i *interpreter) evalJSONSaveBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(2); err != nil {
		return evalResult{}, err
	}
	path, errResult, err := call.stringArg(0)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	input, errResult, err := call.stringArg(1)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(input)); err != nil {
		return wrapperErrorResult(callee, mapJSONError(err)), nil
	}
	if writeErr := os.WriteFile(path, compact.Bytes(), 0o644); writeErr != nil {
		return wrapperErrorResult(callee, mapPathError(path, writeErr)), nil
	}
	return wrapperIntResult(0), nil
}
