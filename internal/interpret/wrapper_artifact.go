package interpret

import (
	"errors"
	"fmt"
	"strings"

	"oct/internal/ast"
)

func artifactWrapperBuiltins() map[string]wrapperBuiltinHandler {
	return map[string]wrapperBuiltinHandler{
		"ArtifactWriteText":     (*interpreter).evalArtifactWriteTextBuiltin,
		"ArtifactWriteLines":    (*interpreter).evalArtifactWriteLinesBuiltin,
		"ArtifactWriteMarkdown": (*interpreter).evalArtifactWriteMarkdownBuiltin,
		"ArtifactWriteCsv":      (*interpreter).evalArtifactWriteCsvBuiltin,
		"ArtifactWriteJson":     (*interpreter).evalArtifactWriteJsonBuiltin,
		"ArtifactWriteOctagon":  (*interpreter).evalArtifactWriteOctagonBuiltin,
		"ArtifactProgress":      (*interpreter).evalArtifactProgressBuiltin,
		"ArtifactCheckpoint":    (*interpreter).evalArtifactCheckpointBuiltin,
	}
}

func (i *interpreter) evalArtifactWriteTextBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	return i.evalArtifactWriteBuiltin(env, pkgName, callee, "FileWriteText", argumentExprs)
}

func (i *interpreter) evalArtifactWriteLinesBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	return i.evalArtifactWriteBuiltin(env, pkgName, callee, "FileWriteLines", argumentExprs)
}

func (i *interpreter) evalArtifactWriteMarkdownBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	return i.evalArtifactWriteBuiltin(env, pkgName, callee, "FileWriteLines", argumentExprs)
}

func (i *interpreter) evalArtifactWriteCsvBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	return i.evalArtifactWriteBuiltin(env, pkgName, callee, "CsvWrite", argumentExprs)
}

func (i *interpreter) evalArtifactWriteJsonBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	return i.evalArtifactWriteBuiltin(env, pkgName, callee, "JsonSave", argumentExprs)
}

func (i *interpreter) evalArtifactWriteOctagonBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	if len(argumentExprs) != 2 {
		return evalResult{}, fmt.Errorf("runtime invariant violation: ArtifactWriteOctagon expects 2 arguments")
	}
	callResult, err := i.evalWriteOctagonBuiltinCallExpr(env, pkgName, argumentExprs)
	if err != nil {
		return evalResult{}, errors.New(strings.ReplaceAll(err.Error(), "WriteOctagon", "Artifact.WriteOctagon"))
	}
	if callResult.hasError {
		return evalResult{}, fmt.Errorf("runtime error: Artifact.WriteOctagon failed: %s", callResult.errorVal.Text)
	}
	return evalResult{value: Value{Kind: ValueInt, Int: 0}}, nil
}

func (i *interpreter) evalArtifactWriteBuiltin(env *environment, pkgName string, callee string, delegate string, argumentExprs []ast.Expr) (evalResult, error) {
	handler, ok := i.wrappers.handlers[delegate]
	if !ok {
		return evalResult{}, fmt.Errorf("runtime invariant violation: missing %s wrapper", delegate)
	}
	callResult, err := handler(i, env, pkgName, delegate, argumentExprs)
	if err != nil {
		return evalResult{}, err
	}
	if callResult.hasError {
		return evalResult{}, fmt.Errorf("runtime error: Artifact.%s failed: %s", strings.TrimPrefix(callee, "ArtifactWrite"), callResult.errorVal.Text)
	}
	return evalResult{value: Value{Kind: ValueInt, Int: 0}}, nil
}

func (i *interpreter) evalArtifactCheckpointBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	if len(argumentExprs) != 1 {
		return evalResult{}, fmt.Errorf("runtime invariant violation: ArtifactCheckpoint expects 1 argument")
	}
	label, err := i.evalExpr(env, pkgName, argumentExprs[0])
	if err != nil {
		return evalResult{}, err
	}
	if label.hasError {
		return evalResult{hasError: true, errorVal: label.errorVal}, nil
	}
	if i.artifactProgressRecorder != nil {
		i.artifactProgressRecorder(ArtifactProgressEvent{Kind: "checkpoint", Function: i.currentFunctionName, Label: label.value.Text})
	}
	return evalResult{value: Value{Kind: ValueInt, Int: 0}}, nil
}

func (i *interpreter) evalArtifactProgressBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	if len(argumentExprs) != 3 {
		return evalResult{}, fmt.Errorf("runtime invariant violation: ArtifactProgress expects 3 arguments")
	}
	label, err := i.evalExpr(env, pkgName, argumentExprs[0])
	if err != nil {
		return evalResult{}, err
	}
	if label.hasError {
		return evalResult{hasError: true, errorVal: label.errorVal}, nil
	}
	current, err := i.evalExpr(env, pkgName, argumentExprs[1])
	if err != nil {
		return evalResult{}, err
	}
	if current.hasError {
		return evalResult{hasError: true, errorVal: current.errorVal}, nil
	}
	total, err := i.evalExpr(env, pkgName, argumentExprs[2])
	if err != nil {
		return evalResult{}, err
	}
	if total.hasError {
		return evalResult{hasError: true, errorVal: total.errorVal}, nil
	}
	if i.artifactProgressRecorder != nil {
		i.artifactProgressRecorder(ArtifactProgressEvent{
			Kind:     "progress",
			Function: i.currentFunctionName,
			Label:    label.value.Text,
			Current:  current.value.Int,
			Total:    total.value.Int,
		})
	}
	return evalResult{value: Value{Kind: ValueInt, Int: 0}}, nil
}
