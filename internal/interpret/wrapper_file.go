package interpret

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

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
		"FileReadLines":  (*interpreter).evalFileReadLinesBuiltin,
		"FileWriteLines": (*interpreter).evalFileWriteLinesBuiltin,
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
	text, readErr := fileReadText(path)
	if readErr != nil {
		return wrapperErrorResult(callee, readErr), nil
	}
	return wrapperStringResult(text), nil
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
	if writeErr := fileWriteText(path, text); writeErr != nil {
		return wrapperErrorResult(callee, writeErr), nil
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
	exists, existsErr := fileExists(path)
	if existsErr != nil {
		return wrapperErrorResult(callee, existsErr), nil
	}
	return wrapperBoolResult(exists), nil
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
	return wrapperBytesResult(contents), nil
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
	bytes, errResult, err := call.bytesArg(1)
	if err != nil {
		return wrapperErrorResult(callee, err), nil
	}
	if errResult != nil {
		return *errResult, nil
	}
	if writeErr := os.WriteFile(path, bytes, 0o644); writeErr != nil {
		if mkdirErr := ensureParentDir(path); mkdirErr == nil {
			if retryErr := os.WriteFile(path, bytes, 0o644); retryErr == nil {
				return wrapperIntResult(0), nil
			} else {
				return wrapperErrorResult(callee, mapPathError(path, retryErr)), nil
			}
		}
		return wrapperErrorResult(callee, mapPathError(path, writeErr)), nil
	}
	return wrapperIntResult(0), nil
}

func (i *interpreter) evalFileReadLinesBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
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
	lines := splitLinesPreservingTerminal(strings.ReplaceAll(string(contents), "\r\n", "\n"))
	return wrapperStringArrayResult(lines), nil
}

func (i *interpreter) evalFileWriteLinesBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
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
	lines, decodeErr := stringArrayArg(value)
	if decodeErr != nil {
		return wrapperErrorResult(callee, decodeErr), nil
	}
	payload := ""
	if len(lines) > 0 {
		payload = strings.Join(lines, "\n") + "\n"
	}
	if writeErr := os.WriteFile(path, []byte(payload), 0o644); writeErr != nil {
		if mkdirErr := ensureParentDir(path); mkdirErr == nil {
			if retryErr := os.WriteFile(path, []byte(payload), 0o644); retryErr == nil {
				return wrapperIntResult(0), nil
			} else {
				return wrapperErrorResult(callee, mapPathError(path, retryErr)), nil
			}
		}
		return wrapperErrorResult(callee, mapPathError(path, writeErr)), nil
	}
	return wrapperIntResult(0), nil
}

func stringArrayArg(value Value) ([]string, error) {
	if value.Kind != ValueArray {
		return nil, wrapperErrorf(wrapperErrorInvalidArgument, "argument expects String[]")
	}
	out := make([]string, 0, len(value.Array))
	for i, element := range value.Array {
		if element.Kind != ValueString {
			return nil, wrapperErrorf(wrapperErrorInvalidArgument, "argument expects String[] (index %d)", i)
		}
		out = append(out, element.Text)
	}
	return out, nil
}

func splitLinesPreservingTerminal(text string) []string {
	if text == "" {
		return []string{}
	}
	parts := strings.Split(text, "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		return parts[:len(parts)-1]
	}
	return parts
}

func ensureParentDir(path string) error {
	parent := filepath.Dir(path)
	if parent == "." || parent == "" {
		return nil
	}
	return os.MkdirAll(parent, 0o755)
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
