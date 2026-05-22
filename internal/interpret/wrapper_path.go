package interpret

import (
	"path/filepath"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

func pathWrapperBuiltins() map[string]wrapperBuiltinHandler {
	return map[string]wrapperBuiltinHandler{
		"PathJoin":      (*interpreter).evalPathJoinBuiltin,
		"PathBaseName":  (*interpreter).evalPathBaseNameBuiltin,
		"PathExtension": (*interpreter).evalPathExtensionBuiltin,
		"PathStem":      (*interpreter).evalPathStemBuiltin,
		"PathParent":    (*interpreter).evalPathParentBuiltin,
		"PathClean":     (*interpreter).evalPathCleanBuiltin,
	}
}

func (i *interpreter) evalPathJoinBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(1); err != nil {
		return evalResult{}, err
	}
	value, errResult, err := call.evalArg(0)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	if value.Kind != ValueArray {
		return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorInvalidArgument, "argument 1 expects String[]")), nil
	}
	parts := make([]string, 0, len(value.Array))
	for idx, current := range value.Array {
		if current.Kind != ValueString {
			return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorInvalidArgument, "argument 1 expects String[] (element %d)", idx)), nil
		}
		parts = append(parts, current.Text)
	}
	if len(parts) == 0 {
		return wrapperStringResult(""), nil
	}
	return wrapperStringResult(filepath.Join(parts...)), nil
}

func (i *interpreter) evalPathBaseNameBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	return i.evalSinglePathTransform(env, pkgName, callee, argumentExprs, filepath.Base)
}

func (i *interpreter) evalPathExtensionBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	return i.evalSinglePathTransform(env, pkgName, callee, argumentExprs, filepath.Ext)
}

func (i *interpreter) evalPathStemBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	return i.evalSinglePathTransform(env, pkgName, callee, argumentExprs, func(path string) string {
		base := filepath.Base(path)
		ext := filepath.Ext(base)
		return strings.TrimSuffix(base, ext)
	})
}

func (i *interpreter) evalPathParentBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	return i.evalSinglePathTransform(env, pkgName, callee, argumentExprs, filepath.Dir)
}

func (i *interpreter) evalPathCleanBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	return i.evalSinglePathTransform(env, pkgName, callee, argumentExprs, filepath.Clean)
}

func (i *interpreter) evalSinglePathTransform(env *environment, pkgName string, callee string, argumentExprs []ast.Expr, transform func(string) string) (evalResult, error) {
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
	return wrapperStringResult(transform(path)), nil
}
