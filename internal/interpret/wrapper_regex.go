package interpret

import (
	"regexp"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

func regexWrapperBuiltins() map[string]wrapperBuiltinHandler {
	return map[string]wrapperBuiltinHandler{
		"RegexIsMatch":    (*interpreter).evalRegexIsMatchBuiltin,
		"RegexFindAll":    (*interpreter).evalRegexFindAllBuiltin,
		"RegexReplaceAll": (*interpreter).evalRegexReplaceAllBuiltin,
		"RegexSplit":      (*interpreter).evalRegexSplitBuiltin,
	}
}

func (i *interpreter) evalRegexIsMatchBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	compiled, text, errResult, err := i.evalRegexPatternAndText(env, pkgName, callee, argumentExprs)
	if err != nil {
		return wrapperErrorResult(callee, err), nil
	}
	if errResult != nil {
		return *errResult, nil
	}
	return wrapperBoolResult(compiled.MatchString(text)), nil
}

func (i *interpreter) evalRegexFindAllBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	compiled, text, errResult, err := i.evalRegexPatternAndText(env, pkgName, callee, argumentExprs)
	if err != nil {
		return wrapperErrorResult(callee, err), nil
	}
	if errResult != nil {
		return *errResult, nil
	}
	matches := compiled.FindAllString(text, -1)
	if matches == nil {
		matches = []string{}
	}
	return wrapperStringArrayResult(matches), nil
}

func (i *interpreter) evalRegexReplaceAllBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(3); err != nil {
		return evalResult{}, err
	}
	pattern, errResult, err := call.stringArg(0)
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
	replacement, errResult, err := call.stringArg(2)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	compiled, compileErr := regexp.Compile(pattern)
	if compileErr != nil {
		return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorInvalidData, "%v", compileErr)), nil
	}
	return wrapperStringResult(compiled.ReplaceAllString(text, replacement)), nil
}

func (i *interpreter) evalRegexSplitBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	compiled, text, errResult, err := i.evalRegexPatternAndText(env, pkgName, callee, argumentExprs)
	if err != nil {
		return wrapperErrorResult(callee, err), nil
	}
	if errResult != nil {
		return *errResult, nil
	}
	return wrapperStringArrayResult(compiled.Split(text, -1)), nil
}

func (i *interpreter) evalRegexPatternAndText(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (*regexp.Regexp, string, *evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(2); err != nil {
		return nil, "", nil, err
	}
	pattern, errResult, err := call.stringArg(0)
	if err != nil {
		return nil, "", nil, err
	}
	if errResult != nil {
		return nil, "", errResult, nil
	}
	text, errResult, err := call.stringArg(1)
	if err != nil {
		return nil, "", nil, err
	}
	if errResult != nil {
		return nil, "", errResult, nil
	}
	compiled, compileErr := regexp.Compile(pattern)
	if compileErr != nil {
		return nil, "", nil, wrapperErrorf(wrapperErrorInvalidData, "%v", compileErr)
	}
	return compiled, text, nil, nil
}
