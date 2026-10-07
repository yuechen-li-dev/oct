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

// evalJsonCall evaluates `Json.Load<T>(path)` and `Json.Parse<T>(text)`.
//
// Every rule about JSON is in internal/octjson, which generated programs call
// as well: it reads the text as the schema of T and gives Octagon data. This
// function hands that data to the materialiser LoadOctagon uses, and so adds
// no rule of its own.
func (i interpreter) evalJsonCall(env *environment, pkgName string, json builtin.JsonBuiltin, typeArguments []ast.TypeRef, argumentExprs []ast.Expr) (evalResult, error) {
	name := json.Name()
	if len(typeArguments) != 1 || len(argumentExprs) != 1 {
		return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects 1 type argument and 1 argument", name)
	}
	if i.requestDiscovery && json.Source == builtin.JsonFromFile {
		return evalResult{}, fmt.Errorf("capability request is not statically discoverable: provider attempted effectful operation %s", name)
	}
	argument, err := i.evalExpr(env, pkgName, argumentExprs[0])
	if err != nil {
		return evalResult{}, err
	}
	if argument.hasError {
		return evalResult{hasError: true, errorVal: argument.errorVal}, nil
	}
	if argument.value.Kind != ValueString {
		return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects a String argument", name)
	}

	schema := jsontype.Of(i.program, pkgName, typeArguments[0])
	var data octjson.Data
	var refusal error
	if json.Source == builtin.JsonFromFile {
		data, refusal = octjson.LoadAs(argument.value.Text, schema, i.admitJsonValue)
	} else {
		data, refusal = octjson.ParseAs(argument.value.Text, schema, i.admitJsonValue)
	}
	if refusal != nil {
		return evalResult{hasError: true, errorVal: Value{Kind: ValueError, Error: ErrorValue{Message: refusal.Error()}}}, nil
	}
	expr, err := octagonDataExpr(data)
	if err != nil {
		return evalResult{}, fmt.Errorf("runtime invariant violation: %s: %w", name, err)
	}
	value, err := i.materializeOctagonValue(pkgName, typeArguments[0], expr)
	if err != nil {
		// octjson has read the text as T, so the data is a T.
		return evalResult{}, fmt.Errorf("runtime invariant violation: %s read a value its type does not take: %w", name, err)
	}
	return evalResult{value: value}, nil
}

// admitJsonValue is octjson.Admit: it answers whether a value meets the
// requirements of the refined concept `Package.Name`, by loading it as the
// concept, which runs the concept's checked constructor.
func (i interpreter) admitJsonValue(concept string, value octjson.Data) string {
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
