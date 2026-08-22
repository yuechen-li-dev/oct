package interpret

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/compileddata"
)

func artifactWrapperBuiltins() map[string]wrapperBuiltinHandler {
	return map[string]wrapperBuiltinHandler{
		"ArtifactWriteText":     (*interpreter).evalArtifactWriteTextBuiltin,
		"ArtifactWriteLines":    (*interpreter).evalArtifactWriteLinesBuiltin,
		"ArtifactWriteMarkdown": (*interpreter).evalArtifactWriteMarkdownBuiltin,
		"ArtifactWriteCsv":      (*interpreter).evalArtifactWriteCsvBuiltin,
		"ArtifactWriteJson":     (*interpreter).evalArtifactWriteJsonBuiltin,
		"ArtifactWriteOctagon":  (*interpreter).evalArtifactWriteOctagonBuiltin,
		"ArtifactCompileData":   (*interpreter).evalArtifactWriteCompiledDataBuiltin,
		"ArtifactProgress":      (*interpreter).evalArtifactProgressBuiltin,
		"ArtifactCheckpoint":    (*interpreter).evalArtifactCheckpointBuiltin,
	}
}

func (i *interpreter) evalArtifactWriteCompiledDataBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	if err := i.beginArtifactWrite(); err != nil {
		return evalResult{}, err
	}
	defer i.endArtifactWrite()
	if len(argumentExprs) != 3 {
		return evalResult{}, fmt.Errorf("runtime invariant violation: ArtifactWriteCompiledData expects 3 arguments")
	}
	args := make([]Value, 3)
	for index, expr := range argumentExprs {
		result, err := i.evalExpr(env, pkgName, expr)
		if err != nil {
			return evalResult{}, err
		}
		if result.hasError {
			return result, nil
		}
		args[index] = result.value
	}
	typ, err := i.compiledDataType(pkgName, args[2])
	if err != nil {
		return evalResult{}, fmt.Errorf("Artifact.WriteCompiledData: %w", err)
	}
	result, err := compileddata.EmitGo(compileddata.Dataset{Symbol: args[1].Text, Type: typ, Value: compiledDataValue(args[2])})
	if err != nil {
		return evalResult{}, fmt.Errorf("Artifact.WriteCompiledData: %w", err)
	}
	logical := attributedOutputPath(args[0].Text)
	actual := logical
	if i.artifactCapability != nil {
		actual, err = i.artifactCapability.StageArtifactOutput(ArtifactOutputRequest{Path: logical, Package: i.artifactPackage, Function: i.currentFunctionName, SourcePath: i.artifactSourcePath, Kind: "compiled-data.go"})
		if err != nil {
			return evalResult{}, err
		}
	}
	if err := os.WriteFile(actual, result.Source, 0o644); err != nil {
		return evalResult{}, fmt.Errorf("Artifact.WriteCompiledData: %w", err)
	}
	i.recordArtifactWrite(logical)
	return evalResult{value: Value{Kind: ValueInt, Int: 0}}, nil
}

func (i *interpreter) compiledDataType(pkgName string, value Value) (compileddata.Type, error) {
	switch value.Kind {
	case ValueInt:
		return compileddata.Type{Kind: compileddata.Int}, nil
	case ValueFloat:
		return compileddata.Type{Kind: compileddata.Float}, nil
	case ValueBool:
		return compileddata.Type{Kind: compileddata.Bool}, nil
	case ValueString:
		return compileddata.Type{Kind: compileddata.String}, nil
	case ValueArray:
		if len(value.Array) == 0 {
			return compileddata.Type{}, fmt.Errorf("cannot infer the element type of an untyped empty root array")
		}
		elem, err := i.compiledDataType(pkgName, value.Array[0])
		if err != nil {
			return compileddata.Type{}, err
		}
		return compileddata.Type{Kind: compileddata.Array, Elem: &elem}, nil
	case ValueEnum:
		decl, resolved, ok := i.lookupEnumDecl(pkgName, value.Enum.TypeName)
		if !ok {
			return compileddata.Type{}, fmt.Errorf("unknown enum type %s", value.Enum.TypeName)
		}
		variants := make([]string, 0, len(decl.Variants))
		for _, variant := range decl.Variants {
			if variant.Payload != nil {
				return compileddata.Type{}, fmt.Errorf("payload enum %s is not yet supported by compiled-data publication", resolved)
			}
			variants = append(variants, variant.Name)
		}
		return compileddata.Type{Kind: compileddata.Enum, Name: resolved, Variants: variants}, nil
	case ValueRecord:
		decl, resolved, ok := i.lookupRecordDecl(pkgName, value.Record.TypeName)
		if !ok {
			return compileddata.Type{}, fmt.Errorf("unknown record type %s", value.Record.TypeName)
		}
		fields := make([]compileddata.Field, 0, len(decl.Fields))
		for _, field := range decl.Fields {
			fieldType, err := i.compiledDataTypeRef(pkgName, field.Type)
			if err != nil {
				return compileddata.Type{}, fmt.Errorf("%s.%s: %w", resolved, field.Name, err)
			}
			fields = append(fields, compileddata.Field{Name: field.Name, Type: fieldType})
		}
		return compileddata.Type{Kind: compileddata.Record, Name: resolved, Fields: fields, Table: decl.IsTable}, nil
	default:
		return compileddata.Type{}, fmt.Errorf("unsupported typed data value %s", value.Kind)
	}
}

func (i *interpreter) compiledDataTypeRef(pkgName string, ref ast.TypeRef) (compileddata.Type, error) {
	depth := ref.ArrayDepth
	if ref.IsArray && depth == 0 {
		depth = 1
	}
	if depth > 0 {
		element := ref
		element.ArrayDepth--
		element.IsArray = element.ArrayDepth > 0
		elem, err := i.compiledDataTypeRef(pkgName, element)
		if err != nil {
			return compileddata.Type{}, err
		}
		return compileddata.Type{Kind: compileddata.Array, Elem: &elem}, nil
	}
	switch ref.Name {
	case "Int":
		return compileddata.Type{Kind: compileddata.Int}, nil
	case "Float":
		return compileddata.Type{Kind: compileddata.Float}, nil
	case "Bool":
		return compileddata.Type{Kind: compileddata.Bool}, nil
	case "String":
		return compileddata.Type{Kind: compileddata.String}, nil
	}
	name := expectedTypeName(ref)
	if refinement, resolvedPkg, ok := i.lookupRefinementDecl(pkgName, name); ok {
		base, err := i.compiledDataTypeRef(resolvedPkg, refinement.Target)
		if err != nil {
			return compileddata.Type{}, err
		}
		return compileddata.Type{Kind: compileddata.Refinement, Name: expectedTypeName(ast.TypeRef{Package: resolvedPkg, Name: refinement.Name}), Base: &base}, nil
	}
	if enum, resolved, ok := i.lookupEnumDecl(pkgName, name); ok {
		variants := make([]string, 0, len(enum.Variants))
		for _, variant := range enum.Variants {
			if variant.Payload != nil {
				return compileddata.Type{}, fmt.Errorf("payload enum %s is not supported", resolved)
			}
			variants = append(variants, variant.Name)
		}
		return compileddata.Type{Kind: compileddata.Enum, Name: resolved, Variants: variants}, nil
	}
	if record, resolved, ok := i.lookupRecordDecl(pkgName, name); ok {
		fields := make([]compileddata.Field, 0, len(record.Fields))
		for _, field := range record.Fields {
			t, err := i.compiledDataTypeRef(pkgName, field.Type)
			if err != nil {
				return compileddata.Type{}, err
			}
			fields = append(fields, compileddata.Field{Name: field.Name, Type: t})
		}
		return compileddata.Type{Kind: compileddata.Record, Name: resolved, Fields: fields, Table: record.IsTable}, nil
	}
	return compileddata.Type{}, fmt.Errorf("unsupported type %s", name)
}

func compiledDataValue(value Value) compileddata.Value {
	out := compileddata.Value{Int: value.Int, Float: value.Float, Bool: value.Bool, Text: value.Text, Variant: value.Enum.Variant}
	switch value.Kind {
	case ValueInt:
		out.Kind = compileddata.Int
	case ValueFloat:
		out.Kind = compileddata.Float
	case ValueBool:
		out.Kind = compileddata.Bool
	case ValueString:
		out.Kind = compileddata.String
	case ValueArray:
		out.Kind = compileddata.Array
		out.Array = make([]compileddata.Value, len(value.Array))
		for index := range value.Array {
			out.Array[index] = compiledDataValue(value.Array[index])
		}
	case ValueEnum:
		out.Kind = compileddata.Enum
	case ValueRecord:
		out.Kind = compileddata.Record
		out.Fields = make(map[string]compileddata.Value, len(value.Record.Fields))
		for name, field := range value.Record.Fields {
			out.Fields[name] = compiledDataValue(field)
		}
	}
	return out
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
	if err := i.beginArtifactWrite(); err != nil {
		return evalResult{}, err
	}
	defer i.endArtifactWrite()
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
	if err := i.beginArtifactWrite(); err != nil {
		return evalResult{}, err
	}
	defer i.endArtifactWrite()
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
	if i.artifactCapability == nil && i.artifactProgressRecorder == nil {
		return evalResult{}, fmt.Errorf("Artifact.Checkpoint is available only during `oct artifact` evaluation")
	}
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
	if i.artifactCapability == nil && i.artifactProgressRecorder == nil {
		return evalResult{}, fmt.Errorf("Artifact.Progress is available only during `oct artifact` evaluation")
	}
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
