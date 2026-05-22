package interpret

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

type jsonCompatGraphRow struct {
	id          int
	parentID    int
	key         string
	kind        string
	numberValue string
	stringValue string
	boolValue   bool
	isNull      bool
}

func parseJSONToCompatExpr(input []byte) (ast.Expr, error) {
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, mapJSONError(err)
	}
	if decoder.More() {
		return nil, wrapperErrorf(wrapperErrorInvalidData, "document must contain a single JSON value")
	}
	rows := make([]jsonCompatGraphRow, 0, 32)
	_, err := appendJSONGraphRows(&rows, value, -1, "")
	if err != nil {
		return nil, err
	}
	nodes := make([]ast.Expr, 0, len(rows))
	for _, row := range rows {
		nodes = append(nodes, ast.RecordLiteralExpr{TypeName: "JsonRawGraphNode", Fields: []ast.RecordLiteralField{
			{Name: "Id", Value: ast.IntegerLiteral{Value: strconv.Itoa(row.id)}},
			{Name: "ParentId", Value: ast.IntegerLiteral{Value: strconv.Itoa(row.parentID)}},
			{Name: "Key", Value: ast.StringLiteralExpr{Value: row.key}},
			{Name: "Kind", Value: ast.StringLiteralExpr{Value: row.kind}},
			{Name: "NumberValue", Value: ast.FloatLiteral{Value: row.numberValue}},
			{Name: "StringValue", Value: ast.StringLiteralExpr{Value: row.stringValue}},
			{Name: "BoolValue", Value: ast.BoolLiteral{Value: row.boolValue}},
			{Name: "IsNull", Value: ast.BoolLiteral{Value: row.isNull}},
		}})
	}
	return ast.RecordLiteralExpr{TypeName: "JsonRawGraph", Fields: []ast.RecordLiteralField{
		{Name: "Nodes", Value: ast.ArrayLiteralExpr{Elements: nodes}},
	}}, nil
}

func appendJSONGraphRows(rows *[]jsonCompatGraphRow, value any, parentID int, key string) (int, error) {
	id := len(*rows) + 1
	row := jsonCompatGraphRow{id: id, parentID: parentID, key: key, numberValue: "0", stringValue: ""}
	switch current := value.(type) {
	case map[string]any:
		row.kind = "object"
		*rows = append(*rows, row)
		keys := make([]string, 0, len(current))
		for currentKey := range current {
			keys = append(keys, currentKey)
		}
		sort.Strings(keys)
		for _, childKey := range keys {
			if _, err := appendJSONGraphRows(rows, current[childKey], id, childKey); err != nil {
				return 0, err
			}
		}
	case []any:
		row.kind = "array"
		*rows = append(*rows, row)
		for index, childValue := range current {
			if _, err := appendJSONGraphRows(rows, childValue, id, strconv.Itoa(index)); err != nil {
				return 0, err
			}
		}
	case string:
		row.kind = "string"
		row.stringValue = current
		*rows = append(*rows, row)
	case json.Number:
		if _, err := current.Float64(); err != nil {
			return 0, wrapperErrorf(wrapperErrorInvalidData, "invalid number %q", current.String())
		}
		row.kind = "number"
		row.numberValue = current.String()
		*rows = append(*rows, row)
	case float64:
		row.kind = "number"
		row.numberValue = strconv.FormatFloat(current, 'g', -1, 64)
		*rows = append(*rows, row)
	case bool:
		row.kind = "bool"
		row.boolValue = current
		*rows = append(*rows, row)
	case nil:
		row.kind = "null"
		row.isNull = true
		*rows = append(*rows, row)
	default:
		return 0, fmt.Errorf("unsupported JSON value kind %T", value)
	}
	return id, nil
}

func isJsonRawGraphType(typeRef ast.TypeRef) bool {
	if typeRef.IsArray || typeRef.Function != nil || typeRef.VectorOf != nil || typeRef.MatrixOf != nil {
		return false
	}
	if typeRef.Package == "" {
		return typeRef.Name == "JsonRawGraph"
	}
	return typeRef.Package == "IO" && typeRef.Name == "JsonRawGraph"
}

func (i interpreter) evalJSONLowerBuiltinCallExpr(env *environment, pkgName string, typeArguments []ast.TypeRef, argumentExprs []ast.Expr) (evalResult, error) {
	if len(typeArguments) != 1 {
		return evalResult{}, fmt.Errorf("runtime invariant violation: JsonLower expects 1 type argument")
	}
	if len(argumentExprs) != 1 {
		return evalResult{}, fmt.Errorf("runtime invariant violation: JsonLower expects 1 argument")
	}
	if !isJsonRawGraphType(typeArguments[0]) {
		return evalResult{}, fmt.Errorf("runtime invariant violation: JsonLower type argument must be JsonRawGraph")
	}
	argument, err := i.evalExpr(env, pkgName, argumentExprs[0])
	if err != nil {
		return evalResult{}, err
	}
	if argument.hasError {
		return evalResult{hasError: true, errorVal: argument.errorVal}, nil
	}
	if argument.value.Kind != ValueString {
		return evalResult{}, fmt.Errorf("runtime invariant violation: JsonLower expects String argument")
	}
	loweredExpr, lowerErr := parseJSONToCompatExpr([]byte(argument.value.Text))
	if lowerErr != nil {
		return wrapperErrorResult("JsonLower", lowerErr), nil
	}
	value, materializeErr := i.materializeOctagonValue(pkgName, typeArguments[0], loweredExpr)
	if materializeErr != nil {
		return wrapperErrorResult("JsonLower", materializeErr), nil
	}
	return evalResult{value: value}, nil
}

func (i interpreter) evalJSONLoadStructuredBuiltinCallExpr(env *environment, pkgName string, typeArguments []ast.TypeRef, argumentExprs []ast.Expr) (evalResult, error) {
	if len(typeArguments) != 1 {
		return evalResult{}, fmt.Errorf("runtime invariant violation: JsonLoadStructured expects 1 type argument")
	}
	if len(argumentExprs) != 1 {
		return evalResult{}, fmt.Errorf("runtime invariant violation: JsonLoadStructured expects 1 argument")
	}
	if !isJsonRawGraphType(typeArguments[0]) {
		return evalResult{}, fmt.Errorf("runtime invariant violation: JsonLoadStructured type argument must be JsonRawGraph")
	}
	pathResult, err := i.evalExpr(env, pkgName, argumentExprs[0])
	if err != nil {
		return evalResult{}, err
	}
	if pathResult.hasError {
		return evalResult{hasError: true, errorVal: pathResult.errorVal}, nil
	}
	if pathResult.value.Kind != ValueString {
		return evalResult{}, fmt.Errorf("runtime invariant violation: JsonLoadStructured expects String path argument")
	}
	contents, readErr := os.ReadFile(pathResult.value.Text)
	if readErr != nil {
		return wrapperErrorResult("JsonLoadStructured", mapPathError(pathResult.value.Text, readErr)), nil
	}
	loweredExpr, lowerErr := parseJSONToCompatExpr(contents)
	if lowerErr != nil {
		return wrapperErrorResult("JsonLoadStructured", lowerErr), nil
	}
	value, materializeErr := i.materializeOctagonValue(pkgName, typeArguments[0], loweredExpr)
	if materializeErr != nil {
		return wrapperErrorResult("JsonLoadStructured", materializeErr), nil
	}
	return evalResult{value: value}, nil
}
