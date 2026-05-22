package interpret

import (
	"crypto/sha256"
	"encoding/hex"
	"os"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

func hashWrapperBuiltins() map[string]wrapperBuiltinHandler {
	return map[string]wrapperBuiltinHandler{
		"HashSha256Bytes": (*interpreter).evalHashSha256BytesBuiltin,
		"HashSha256Text":  (*interpreter).evalHashSha256TextBuiltin,
		"HashSha256File":  (*interpreter).evalHashSha256FileBuiltin,
	}
}

func (i *interpreter) evalHashSha256BytesBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(1); err != nil {
		return evalResult{}, err
	}
	data, errResult, err := call.bytesArg(0)
	if err != nil {
		return wrapperErrorResult(callee, err), nil
	}
	if errResult != nil {
		return *errResult, nil
	}
	return wrapperStringResult(sha256Hex(data)), nil
}

func (i *interpreter) evalHashSha256TextBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(1); err != nil {
		return evalResult{}, err
	}
	text, errResult, err := call.stringArg(0)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	return wrapperStringResult(sha256Hex([]byte(text))), nil
}

func (i *interpreter) evalHashSha256FileBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
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
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return wrapperErrorResult(callee, mapPathError(path, readErr)), nil
	}
	return wrapperStringResult(sha256Hex(data)), nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
