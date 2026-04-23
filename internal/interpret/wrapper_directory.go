package interpret

import (
	"os"
	"sort"

	"oct/internal/ast"
)

func directoryWrapperBuiltins() map[string]wrapperBuiltinHandler {
	return map[string]wrapperBuiltinHandler{
		"DirectoryList":      (*interpreter).evalDirectoryListBuiltin,
		"DirectoryMake":      (*interpreter).evalDirectoryMakeBuiltin,
		"DirectoryMakeAll":   (*interpreter).evalDirectoryMakeAllBuiltin,
		"DirectoryRemoveAll": (*interpreter).evalDirectoryRemoveAllBuiltin,
	}
}

func (i *interpreter) evalDirectoryListBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
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
	entries, readErr := os.ReadDir(path)
	if readErr != nil {
		return wrapperErrorResult(callee, mapPathError(path, readErr)), nil
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return wrapperStringArrayResult(names), nil
}

func (i *interpreter) evalDirectoryMakeBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	return i.evalDirectoryMutation(env, pkgName, callee, argumentExprs, func(path string) error {
		return os.Mkdir(path, 0o755)
	})
}

func (i *interpreter) evalDirectoryMakeAllBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	return i.evalDirectoryMutation(env, pkgName, callee, argumentExprs, func(path string) error {
		return os.MkdirAll(path, 0o755)
	})
}

func (i *interpreter) evalDirectoryRemoveAllBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	return i.evalDirectoryMutation(env, pkgName, callee, argumentExprs, os.RemoveAll)
}

func (i *interpreter) evalDirectoryMutation(env *environment, pkgName string, callee string, argumentExprs []ast.Expr, mutate func(string) error) (evalResult, error) {
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
	if mutateErr := mutate(path); mutateErr != nil {
		return wrapperErrorResult(callee, mapPathError(path, mutateErr)), nil
	}
	return wrapperIntResult(0), nil
}
