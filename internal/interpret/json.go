package interpret

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/builtin"
	"github.com/yuechen-li-dev/oct/internal/dimension"
	"github.com/yuechen-li-dev/oct/internal/jsontype"
	"github.com/yuechen-li-dev/oct/internal/octjson"
)

// evalJsonCall evaluates a call to a Json builtin at the type it is made at:
// `Json.Load<T>(path)`, `Json.Parse<T>(text)`, `Json.Save(path, value)`,
// `Json.Text(value)` and `Artifact.WriteJson(path, value)`.
//
// Every rule about JSON is in internal/octjson, which generated programs call
// as well. A read hands the Octagon data octjson gives to the materialiser
// LoadOctagon uses; a write hands octjson the value as Octagon data. This
// file adds no rule of its own.
func (i *interpreter) evalJsonCall(env *environment, pkgName string, json builtin.JsonBuiltin, callType ast.TypeRef, argumentExprs []ast.Expr) (evalResult, error) {
	name := json.Name()
	if json.Action == builtin.JsonReadFile || json.Action == builtin.JsonWriteFile {
		// The two that touch a file the program names.
		if i.requestDiscovery {
			return evalResult{}, fmt.Errorf("capability request is not statically discoverable: provider attempted effectful operation %s", name)
		}
		if i.artifactCapability != nil {
			if json.Action == builtin.JsonReadFile {
				return evalResult{}, fmt.Errorf("artifact evaluation rejected ambient operation %s", name)
			}
			return evalResult{}, fmt.Errorf("artifact evaluation rejected %s outside Artifact.Write*; use the compiler-owned Artifact capability", name)
		}
	}
	arguments := make([]Value, len(argumentExprs))
	for index, expr := range argumentExprs {
		argument, err := i.evalExpr(env, pkgName, expr)
		if err != nil {
			return evalResult{}, err
		}
		if argument.hasError {
			return evalResult{hasError: true, errorVal: argument.errorVal}, nil
		}
		arguments[index] = argument.value
	}
	expected := 2
	if json.Action != builtin.JsonWriteFile && json.Action != builtin.JsonWriteArtifact {
		expected = 1
	}
	if len(arguments) != expected || (json.Action != builtin.JsonWriteText && arguments[0].Kind != ValueString) {
		return evalResult{}, fmt.Errorf("runtime invariant violation: %s was given arguments the typechecker refuses", name)
	}
	schema := jsontype.Of(i.program, pkgName, callType)
	if json.Writes() {
		return i.evalJsonWrite(json, schema, arguments)
	}

	var data octjson.Data
	var refusal error
	if json.Action == builtin.JsonReadFile {
		data, refusal = octjson.LoadAs(arguments[0].Text, schema, i.admitJsonValue)
	} else {
		data, refusal = octjson.ParseAs(arguments[0].Text, schema, i.admitJsonValue)
	}
	if refusal != nil {
		return evalResult{hasError: true, errorVal: Value{Kind: ValueError, Error: ErrorValue{Message: refusal.Error()}}}, nil
	}
	expr, err := octagonDataExpr(data)
	if err != nil {
		return evalResult{}, fmt.Errorf("runtime invariant violation: %s: %w", name, err)
	}
	value, err := i.materializeOctagonValue(pkgName, callType, expr)
	if err != nil {
		// octjson has read the text as T, so the data is a T.
		return evalResult{}, fmt.Errorf("runtime invariant violation: %s read a value its type does not take: %w", name, err)
	}
	return evalResult{value: value}, nil
}

// evalJsonWrite writes a value as the type the schema describes. A value JSON
// cannot hold, such as a NaN, stops the program; a file that cannot be
// written is an Error `Json.Save` gives the program.
func (i *interpreter) evalJsonWrite(json builtin.JsonBuiltin, schema *octjson.Schema, arguments []Value) (evalResult, error) {
	name := json.Name()
	data, err := octagonDataOf(arguments[len(arguments)-1], schema)
	if err != nil {
		return evalResult{}, fmt.Errorf("runtime invariant violation: %s: %w", name, err)
	}
	switch json.Action {
	case builtin.JsonWriteText:
		text, err := octjson.TextAs(octjson.OperationText, "", data, schema)
		if err != nil {
			return evalResult{}, fmt.Errorf("runtime error: %w", err)
		}
		return evalResult{value: Value{Kind: ValueString, Text: text}}, nil
	case builtin.JsonWriteFile:
		err := octjson.SaveAs(arguments[0].Text, data, schema)
		switch {
		case err == nil:
			return evalResult{}, nil
		case octjson.Stops(err):
			return evalResult{}, fmt.Errorf("runtime error: %w", err)
		default:
			return evalResult{hasError: true, errorVal: Value{Kind: ValueError, Error: ErrorValue{Message: err.Error()}}}, nil
		}
	default:
		return i.evalJsonArtifactWrite(arguments[0].Text, data, schema)
	}
}

// evalJsonArtifactWrite is `Artifact.WriteJson(path, value)`: the text goes
// to the output the Artifact capability stages, as `Artifact.WriteText`
// sends its text.
func (i *interpreter) evalJsonArtifactWrite(path string, data octjson.Data, schema *octjson.Schema) (evalResult, error) {
	if err := i.beginArtifactWrite(); err != nil {
		return evalResult{}, err
	}
	defer i.endArtifactWrite()
	text, err := octjson.TextAs(octjson.OperationArtifactWrite, path, data, schema)
	if err != nil {
		return evalResult{}, fmt.Errorf("runtime error: %w", err)
	}
	actualPath, logicalPath, err := i.prepareArtifactOutput(path)
	if err != nil {
		return evalResult{}, err
	}
	if err := fileWriteText(actualPath, text); err != nil {
		return evalResult{}, fmt.Errorf("runtime error: %s: %s: %v", octjson.OperationArtifactWrite, path, err)
	}
	i.recordArtifactWrite(logicalPath)
	return evalResult{}, nil
}

// octagonDataOf is a value as Octagon data, which is what octjson writes.
// The schema says what the value is: a record table is its columns, a
// vector an array, a matrix an array of rows, and a value of a refined
// concept a value of its base type.
func octagonDataOf(value Value, schema *octjson.Schema) (octjson.Data, error) {
	mismatch := func() (octjson.Data, error) {
		return octjson.Data{}, fmt.Errorf("a value of kind %s where the type says kind %d", string(value.Kind), schema.Kind)
	}
	list := func(items []Value, element *octjson.Schema) (octjson.Data, error) {
		array := octjson.Data{Kind: octjson.DataArray, Array: make([]octjson.Data, len(items))}
		for index, item := range items {
			converted, err := octagonDataOf(item, element)
			if err != nil {
				return octjson.Data{}, err
			}
			array.Array[index] = converted
		}
		return array, nil
	}
	switch schema.Kind {
	case octjson.KindBool:
		if value.Kind != ValueBool {
			return mismatch()
		}
		return octjson.Data{Kind: octjson.DataBool, Bool: value.Bool}, nil
	case octjson.KindInt:
		if value.Kind != ValueInt {
			return mismatch()
		}
		return octjson.Data{Kind: octjson.DataInt, Int: value.Int, Dimension: schema.Dimension}, nil
	case octjson.KindFloat:
		if value.Kind != ValueFloat {
			return mismatch()
		}
		return octjson.Data{Kind: octjson.DataFloat, Float: value.Float, Dimension: schema.Dimension}, nil
	case octjson.KindString:
		if value.Kind != ValueString {
			return mismatch()
		}
		return octjson.Data{Kind: octjson.DataString, Text: value.Text}, nil
	case octjson.KindEnum:
		if value.Kind != ValueEnum {
			return mismatch()
		}
		return octjson.Data{Kind: octjson.DataEnum, EnumType: schema.Name, Variant: value.Enum.Variant}, nil
	case octjson.KindOption:
		if value.Kind != ValueEnum || value.Enum.TypeName != ast.OptionTypeName {
			return mismatch()
		}
		option := octjson.Data{Kind: octjson.DataEnum, EnumType: octjson.OptionType, Variant: value.Enum.Variant}
		if value.Enum.Payload != nil {
			payload, err := octagonDataOf(*value.Enum.Payload, schema.Elem)
			if err != nil {
				return octjson.Data{}, err
			}
			option.Payload = &payload
		}
		return option, nil
	case octjson.KindRecord, octjson.KindTable:
		if value.Kind != ValueRecord {
			return mismatch()
		}
		record := octjson.Data{Kind: octjson.DataRecord, RecordType: schema.Name, Fields: make([]octjson.DataField, len(schema.Fields))}
		for index, field := range schema.Fields {
			held, ok := value.Record.Fields[field.Name]
			if !ok {
				return octjson.Data{}, fmt.Errorf("a %s with no field %s", schema.Name, field.Name)
			}
			var converted octjson.Data
			var err error
			if schema.Kind == octjson.KindTable {
				// A table holds each field as a whole column.
				if held.Kind != ValueArray {
					return octjson.Data{}, fmt.Errorf("the column %s of %s is not an array", field.Name, schema.Name)
				}
				converted, err = list(held.Array, field.Type)
			} else {
				converted, err = octagonDataOf(held, field.Type)
			}
			if err != nil {
				return octjson.Data{}, err
			}
			record.Fields[index] = octjson.DataField{Name: field.Name, Value: converted}
		}
		return record, nil
	case octjson.KindArray:
		if value.Kind != ValueArray {
			return mismatch()
		}
		return list(value.Array, schema.Elem)
	case octjson.KindVector:
		if value.Kind != ValueVector {
			return mismatch()
		}
		return list(value.Vector, schema.Elem)
	case octjson.KindMatrix:
		if value.Kind != ValueMatrix {
			return mismatch()
		}
		rows := octjson.Data{Kind: octjson.DataArray, Array: make([]octjson.Data, value.Matrix.Rows)}
		for row := range rows.Array {
			converted, err := list(value.Matrix.Elements[row*value.Matrix.Cols:(row+1)*value.Matrix.Cols], schema.Elem)
			if err != nil {
				return octjson.Data{}, err
			}
			rows.Array[row] = converted
		}
		return rows, nil
	default:
		return mismatch()
	}
}

// admitJsonValue is octjson.Admit: it answers whether a value meets the
// requirements of the refined concept `Package.Name`, by loading it as the
// concept, which runs the concept's checked constructor.
func (i *interpreter) admitJsonValue(concept string, value octjson.Data) string {
	pkgName, name, _ := strings.Cut(concept, ".")
	expr, err := octagonDataExpr(value)
	if err == nil {
		_, err = i.materializeOctagonValue(pkgName, ast.TypeRef{Name: name}, expr)
	}
	if err != nil {
		return err.Error()
	}
	return ""
}

// octagonDataExpr writes Octagon data as the expression an `.octagon` file
// holds for it, which is what the materialiser takes.
func octagonDataExpr(data octjson.Data) (ast.Expr, error) {
	switch data.Kind {
	case octjson.DataInt, octjson.DataFloat:
		unit, ok := dimension.Parse(data.Dimension)
		if !ok {
			return nil, fmt.Errorf("unknown dimension %q", data.Dimension)
		}
		if data.Kind == octjson.DataInt {
			return ast.IntegerLiteral{Value: strconv.FormatInt(data.Int, 10), Dimension: unit, HasUnit: data.Dimension != ""}, nil
		}
		return ast.FloatLiteral{Value: strconv.FormatFloat(data.Float, 'g', -1, 64), Dimension: unit, HasUnit: data.Dimension != ""}, nil
	case octjson.DataBool:
		return ast.BoolLiteral{Value: data.Bool}, nil
	case octjson.DataString:
		return ast.StringLiteralExpr{Value: data.Text}, nil
	case octjson.DataArray:
		elements := make([]ast.Expr, len(data.Array))
		for index, item := range data.Array {
			element, err := octagonDataExpr(item)
			if err != nil {
				return nil, err
			}
			elements[index] = element
		}
		return ast.ArrayLiteralExpr{Elements: elements}, nil
	case octjson.DataRecord:
		fields := make([]ast.RecordLiteralField, len(data.Fields))
		for index, field := range data.Fields {
			value, err := octagonDataExpr(field.Value)
			if err != nil {
				return nil, err
			}
			fields[index] = ast.RecordLiteralField{Name: field.Name, Value: value}
		}
		return ast.RecordLiteralExpr{TypeName: data.RecordType, Fields: fields}, nil
	case octjson.DataEnum:
		if data.EnumType != octjson.OptionType {
			return ast.EnumValueExpr{EnumName: data.EnumType, Variant: data.Variant}, nil
		}
		// `Option.None` and `Option.Some(value)`, as the parser shapes them.
		option := ast.CallExpr{
			Callee:        ast.FieldAccessExpr{Target: ast.IdentifierExpr{Name: ast.OptionTypeName}, Field: data.Variant},
			TypeArguments: []ast.TypeRef{{Inferred: true}},
		}
		if data.Payload != nil {
			payload, err := octagonDataExpr(*data.Payload)
			if err != nil {
				return nil, err
			}
			option.Arguments = []ast.Expr{payload}
		}
		return option, nil
	default:
		return nil, fmt.Errorf("unknown kind of Octagon data %d", data.Kind)
	}
}
