package interpret

import (
	"errors"
	"os"

	"oct/internal/ast"
)

func fileWrapperBuiltins() map[string]wrapperBuiltinHandler {
	return map[string]wrapperBuiltinHandler{
		"FileReadText":   (*interpreter).evalFileReadTextBuiltin,
		"FileWriteText":  (*interpreter).evalFileWriteTextBuiltin,
		"FileExists":     (*interpreter).evalFileExistsBuiltin,
		"FileDelete":     (*interpreter).evalFileDeleteBuiltin,
		"FileReadBytes":  (*interpreter).evalFileReadBytesBuiltin,
		"FileWriteBytes": (*interpreter).evalFileWriteBytesBuiltin,
	}
}

func (i *interpreter) evalFileReadTextBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
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
	return wrapperStringResult(string(contents)), nil
}

func (i *interpreter) evalFileWriteTextBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
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
	text, errResult, err := call.stringArg(1)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	if writeErr := os.WriteFile(path, []byte(text), 0o644); writeErr != nil {
		return wrapperErrorResult(callee, mapPathError(path, writeErr)), nil
	}
	return wrapperIntResult(0), nil
}

func (i *interpreter) evalFileExistsBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
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
	_, statErr := os.Stat(path)
	if statErr == nil {
		return wrapperBoolResult(true), nil
	}
	if errors.Is(statErr, os.ErrNotExist) {
		return wrapperBoolResult(false), nil
	}
	return wrapperErrorResult(callee, mapPathError(path, statErr)), nil
}

func (i *interpreter) evalFileDeleteBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
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
	if removeErr := os.Remove(path); removeErr != nil {
		return wrapperErrorResult(callee, mapPathError(path, removeErr)), nil
	}
	return wrapperIntResult(0), nil
}

func (i *interpreter) evalFileReadBytesBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
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
	items := make([]Value, 0, len(contents))
	for _, current := range contents {
		items = append(items, Value{Kind: ValueInt, Int: int64(current)})
	}
	return wrapperArrayResult(items), nil
}

func (i *interpreter) evalFileWriteBytesBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
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
	value, errResult, err := call.evalArg(1)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	if value.Kind != ValueArray {
		return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorInvalidArgument, "argument 2 expects Int[]")), nil
	}
	bytes := make([]byte, len(value.Array))
	for idx, current := range value.Array {
		if current.Kind != ValueInt {
			return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorInvalidArgument, "argument 2 expects Int[]")), nil
		}
		if current.Int < 0 || current.Int > 255 {
			return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorInvalidArgument, "argument 2 byte at index %d out of range [0,255]", idx)), nil
		}
		bytes[idx] = byte(current.Int)
	}
	if writeErr := os.WriteFile(path, bytes, 0o644); writeErr != nil {
		return wrapperErrorResult(callee, mapPathError(path, writeErr)), nil
	}
	return wrapperIntResult(0), nil
}

func mapPathError(path string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return wrapperErrorf(wrapperErrorNotFound, "%s: %v", path, err)
	}
	if errors.Is(err, os.ErrExist) {
		return wrapperErrorf(wrapperErrorConflict, "%s: %v", path, err)
	}
	if errors.Is(err, os.ErrPermission) {
		return wrapperErrorf(wrapperErrorInvalidArgument, "%s: %v", path, err)
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		if errors.Is(pathErr.Err, os.ErrNotExist) {
			return wrapperErrorf(wrapperErrorNotFound, "%s: %v", path, err)
		}
	}
	return wrapperErrorf(wrapperErrorBackendFailure, "%s: %v", path, err)
}

func mapJSONError(err error) error {
	if err == nil {
		return nil
	}
	return wrapperErrorf(wrapperErrorInvalidData, "%v", err)
}

func mapCSVError(err error) error {
	if err == nil {
		return nil
	}
	return wrapperErrorf(wrapperErrorInvalidData, "%v", err)
}
