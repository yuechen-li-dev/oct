package interpret

import (
	"time"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

func timeWrapperBuiltins() map[string]wrapperBuiltinHandler {
	return map[string]wrapperBuiltinHandler{
		"TimeNowIso8601":       (*interpreter).evalTimeNowIso8601Builtin,
		"TimeParseIso8601":     (*interpreter).evalTimeParseIso8601Builtin,
		"TimeFormatIso8601":    (*interpreter).evalTimeFormatIso8601Builtin,
		"TimeUnixSecondsNow":   (*interpreter).evalTimeUnixSecondsNowBuiltin,
		"TimeFormatUnixSecond": (*interpreter).evalTimeFormatUnixSecondsBuiltin,
	}
}

func (i *interpreter) evalTimeNowIso8601Builtin(_ *environment, _ string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, nil, "", callee, argumentExprs)
	if err := call.expectArity(0); err != nil {
		return evalResult{}, err
	}
	return wrapperStringResult(time.Now().UTC().Format(time.RFC3339)), nil
}

func (i *interpreter) evalTimeParseIso8601Builtin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
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
	parsed, parseErr := parseIso8601(text)
	if parseErr != nil {
		return wrapperErrorResult(callee, parseErr), nil
	}
	return wrapperStringResult(parsed.Format(time.RFC3339)), nil
}

func (i *interpreter) evalTimeFormatIso8601Builtin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	return i.evalTimeParseIso8601Builtin(env, pkgName, callee, argumentExprs)
}

func (i *interpreter) evalTimeUnixSecondsNowBuiltin(_ *environment, _ string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, nil, "", callee, argumentExprs)
	if err := call.expectArity(0); err != nil {
		return evalResult{}, err
	}
	return wrapperIntResult(time.Now().UTC().Unix()), nil
}

func (i *interpreter) evalTimeFormatUnixSecondsBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(1); err != nil {
		return evalResult{}, err
	}
	seconds, errResult, err := call.intArg(0)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	return wrapperStringResult(time.Unix(seconds, 0).UTC().Format(time.RFC3339)), nil
}

func parseIso8601(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, wrapperErrorf(wrapperErrorInvalidData, "%v", err)
	}
	return parsed.UTC(), nil
}
