package interpret

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"oct/internal/ast"
	"oct/internal/builtin"
	"oct/internal/dimension"
)

type ValueKind string

const (
	ValueInt    ValueKind = "Int"
	ValueFloat  ValueKind = "Float"
	ValueBool   ValueKind = "Bool"
	ValueArray  ValueKind = "Array"
	ValueRange  ValueKind = "Range"
	ValueString ValueKind = "String"
	ValueError  ValueKind = "Error"
	ValueRecord ValueKind = "Record"
	ValueEnum   ValueKind = "Enum"
)

type Value struct {
	Kind      ValueKind
	Dimension dimension.Dimension
	Int       int64
	Float     float64
	Bool      bool
	Text      string
	Array     []Value
	Range     RangeValue
	Error     ErrorValue
	Record    RecordValue
	Enum      EnumValue
}

type ErrorValue struct {
	Message string
}

type RangeValue struct {
	Start int64
	End   int64
	Step  int64
}

type RecordValue struct {
	TypeName   string
	FieldOrder []string
	Fields     map[string]Value
}

type EnumValue struct {
	TypeName string
	Variant  string
}

func (v Value) String() string {
	switch v.Kind {
	case ValueInt:
		return strconv.FormatInt(v.Int, 10) + formatUnitSuffix(v.Dimension)
	case ValueFloat:
		return strconv.FormatFloat(v.Float, 'g', -1, 64) + formatUnitSuffix(v.Dimension)
	case ValueBool:
		return strconv.FormatBool(v.Bool)
	case ValueString:
		return v.Text
	case ValueArray:
		parts := make([]string, 0, len(v.Array))
		for _, element := range v.Array {
			parts = append(parts, element.String())
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case ValueRange:
		return fmt.Sprintf("%d..%d step %d", v.Range.Start, v.Range.End, v.Range.Step)
	case ValueError:
		return v.Error.Message
	case ValueRecord:
		parts := make([]string, 0, len(v.Record.FieldOrder))
		for _, fieldName := range v.Record.FieldOrder {
			parts = append(parts, fmt.Sprintf("%s: %s", fieldName, v.Record.Fields[fieldName].String()))
		}
		return fmt.Sprintf("%s{%s}", v.Record.TypeName, strings.Join(parts, ", "))
	case ValueEnum:
		return fmt.Sprintf("%s.%s", v.Enum.TypeName, v.Enum.Variant)
	default:
		return "<invalid>"
	}
}

type interpreter struct {
	functions map[string]ast.FunctionDecl
	records   map[string]ast.RecordDecl
	enums     map[string]ast.EnumDecl
	stdout    io.Writer
}

type environment struct {
	parent *environment
	values map[string]Value
}

type stmtResult struct {
	value    Value
	returned bool
}

type evalResult struct {
	value    Value
	hasError bool
	errorVal Value
}

type callResult struct {
	value    Value
	hasError bool
	errorVal Value
}

func ExecuteMain(file ast.File, stdout io.Writer) (Value, error) {
	interpreter := interpreter{
		functions: make(map[string]ast.FunctionDecl, len(file.Functions)),
		records:   make(map[string]ast.RecordDecl, len(file.Records)),
		enums:     make(map[string]ast.EnumDecl, len(file.Enums)),
		stdout:    stdout,
	}
	for _, record := range file.Records {
		interpreter.records[record.Name] = record
	}
	for _, enumDecl := range file.Enums {
		interpreter.enums[enumDecl.Name] = enumDecl
	}
	for _, function := range file.Functions {
		interpreter.functions[function.Name] = function
	}

	mainFunction, err := interpreter.findMain()
	if err != nil {
		return Value{}, err
	}

	result, err := interpreter.executeFunction(mainFunction, nil)
	if err != nil {
		return Value{}, err
	}
	if result.hasError {
		return Value{}, fmt.Errorf("fatal error: %s", result.errorVal.Error.Message)
	}
	return result.value, nil
}

func newEnvironment(parent *environment) *environment {
	return &environment{parent: parent, values: make(map[string]Value)}
}

func (e *environment) define(name string, value Value) {
	e.values[name] = value
}

func (e *environment) lookup(name string) (Value, bool) {
	for current := e; current != nil; current = current.parent {
		value, ok := current.values[name]
		if ok {
			return value, true
		}
	}
	return Value{}, false
}

func (i interpreter) findMain() (ast.FunctionDecl, error) {
	function, ok := i.functions["Main"]
	if !ok {
		return ast.FunctionDecl{}, errors.New("missing Main function")
	}
	if len(function.Parameters) != 0 {
		return ast.FunctionDecl{}, errors.New("Main must not have parameters")
	}
	return function, nil
}

func (i interpreter) executeFunction(function ast.FunctionDecl, arguments []Value) (callResult, error) {
	env := newEnvironment(nil)
	for index, parameter := range function.Parameters {
		env.define(parameter.Name, arguments[index])
	}

	result, err := i.executeBlock(env, function.Body)
	if err != nil {
		return callResult{}, err
	}
	if !result.returned {
		return callResult{}, fmt.Errorf("runtime invariant violation: %s completed without returning", function.Name)
	}
	if function.IsFallible && result.value.Kind == ValueError {
		return callResult{hasError: true, errorVal: result.value}, nil
	}
	return callResult{value: result.value}, nil
}

func (i interpreter) executeBlock(parent *environment, block ast.Block) (stmtResult, error) {
	blockEnv := newEnvironment(parent)
	for _, statement := range block.Statements {
		result, err := i.executeStmt(blockEnv, statement)
		if err != nil {
			return stmtResult{}, err
		}
		if result.returned {
			return result, nil
		}
	}
	return stmtResult{}, nil
}

func (i interpreter) executeStmt(env *environment, stmt ast.Stmt) (stmtResult, error) {
	switch node := stmt.(type) {
	case ast.LetStmt:
		value, err := i.evalExpr(env, node.Value)
		if err != nil {
			return stmtResult{}, err
		}
		if value.hasError {
			return stmtResult{value: value.errorVal, returned: true}, nil
		}
		env.define(node.Name, value.value)
		return stmtResult{}, nil
	case ast.ReturnStmt:
		value, err := i.evalExpr(env, node.Value)
		if err != nil {
			return stmtResult{}, err
		}
		if value.hasError {
			return stmtResult{value: value.errorVal, returned: true}, nil
		}
		return stmtResult{value: value.value, returned: true}, nil
	case ast.ExprStmt:
		value, err := i.evalExpr(env, node.Value)
		if err != nil {
			return stmtResult{}, err
		}
		if value.hasError {
			return stmtResult{value: value.errorVal, returned: true}, nil
		}
		return stmtResult{}, nil
	case ast.ForStmt:
		rangeValue, err := i.evalExpr(env, node.Range)
		if err != nil {
			return stmtResult{}, err
		}
		if rangeValue.hasError {
			return stmtResult{value: rangeValue.errorVal, returned: true}, nil
		}
		if rangeValue.value.Kind != ValueRange {
			return stmtResult{}, fmt.Errorf("runtime invariant violation: for loop expected Range, got %s", rangeValue.value.Kind)
		}
		for current := rangeValue.value.Range.Start; current < rangeValue.value.Range.End; current += rangeValue.value.Range.Step {
			iterationEnv := newEnvironment(env)
			iterationEnv.define(node.Name, Value{Kind: ValueInt, Int: current})
			result, err := i.executeBlock(iterationEnv, node.Body)
			if err != nil {
				return stmtResult{}, err
			}
			if result.returned {
				return result, nil
			}
		}
		return stmtResult{}, nil
	case ast.MatchStmt:
		subject, err := i.evalExpr(env, node.Subject)
		if err != nil {
			return stmtResult{}, err
		}
		armEnv := newEnvironment(env)
		if subject.hasError {
			armEnv.define(node.ErrName, subject.errorVal)
			return i.executeBlock(armEnv, node.ErrBody)
		}
		armEnv.define(node.OkName, subject.value)
		return i.executeBlock(armEnv, node.OkBody)
	case ast.IfStmt:
		condition, err := i.evalExpr(env, node.Condition)
		if err != nil {
			return stmtResult{}, err
		}
		if condition.hasError {
			return stmtResult{value: condition.errorVal, returned: true}, nil
		}
		if condition.value.Kind != ValueBool {
			return stmtResult{}, fmt.Errorf("runtime invariant violation: if condition must be Bool, got %s", condition.value.Kind)
		}
		if condition.value.Bool {
			return i.executeBlock(env, node.ThenBody)
		}
		if node.ElseBody != nil {
			return i.executeBlock(env, *node.ElseBody)
		}
		return stmtResult{}, nil
	case ast.WhileStmt:
		for {
			condition, err := i.evalExpr(env, node.Condition)
			if err != nil {
				return stmtResult{}, err
			}
			if condition.hasError {
				return stmtResult{value: condition.errorVal, returned: true}, nil
			}
			if condition.value.Kind != ValueBool {
				return stmtResult{}, fmt.Errorf("runtime invariant violation: while condition must be Bool, got %s", condition.value.Kind)
			}
			if !condition.value.Bool {
				return stmtResult{}, nil
			}

			result, err := i.executeBlock(env, node.Body)
			if err != nil {
				return stmtResult{}, err
			}
			if result.returned {
				return result, nil
			}
		}
	default:
		return stmtResult{}, fmt.Errorf("runtime invariant violation: unsupported statement %T", stmt)
	}
}

func (i interpreter) evalExpr(env *environment, expr ast.Expr) (evalResult, error) {
	switch node := expr.(type) {
	case ast.IntegerLiteral:
		value, err := strconv.ParseInt(node.Value, 10, 64)
		if err != nil {
			return evalResult{}, fmt.Errorf("runtime invariant violation: invalid integer literal %q: %w", node.Value, err)
		}
		return evalResult{value: Value{Kind: ValueInt, Int: value, Dimension: node.Dimension}}, nil
	case ast.FloatLiteral:
		value, err := strconv.ParseFloat(node.Value, 64)
		if err != nil {
			return evalResult{}, fmt.Errorf("runtime invariant violation: invalid float literal %q: %w", node.Value, err)
		}
		return evalResult{value: Value{Kind: ValueFloat, Float: value, Dimension: node.Dimension}}, nil
	case ast.BoolLiteral:
		return evalResult{value: Value{Kind: ValueBool, Bool: node.Value}}, nil
	case ast.StringLiteralExpr:
		return evalResult{value: Value{Kind: ValueString, Text: node.Value}}, nil
	case ast.ArrayLiteralExpr:
		value, err := i.evalArrayLiteralExpr(env, node)
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: value}, nil
	case ast.IdentifierExpr:
		value, ok := env.lookup(node.Name)
		if !ok {
			return evalResult{}, fmt.Errorf("runtime invariant violation: undefined variable %s", node.Name)
		}
		return evalResult{value: value}, nil
	case ast.CallExpr:
		return i.evalCallExpr(env, node)
	case ast.RecordLiteralExpr:
		return i.evalRecordLiteralExpr(env, node)
	case ast.EnumValueExpr:
		enumDecl, ok := i.enums[node.EnumName]
		if !ok {
			return evalResult{}, fmt.Errorf("runtime invariant violation: unknown enum type %s", node.EnumName)
		}
		found := false
		for _, variant := range enumDecl.Variants {
			if variant == node.Variant {
				found = true
				break
			}
		}
		if !found {
			return evalResult{}, fmt.Errorf("runtime invariant violation: enum '%s' has no variant '%s'", node.EnumName, node.Variant)
		}
		return evalResult{value: Value{Kind: ValueEnum, Enum: EnumValue{TypeName: node.EnumName, Variant: node.Variant}}}, nil
	case ast.IndexExpr:
		target, err := i.evalExpr(env, node.Target)
		if err != nil {
			return evalResult{}, err
		}
		if target.hasError {
			return evalResult{hasError: true, errorVal: target.errorVal}, nil
		}
		if target.value.Kind != ValueArray {
			return evalResult{}, fmt.Errorf("runtime invariant violation: cannot index non-array value of kind %s", target.value.Kind)
		}
		index, err := i.evalExpr(env, node.Index)
		if err != nil {
			return evalResult{}, err
		}
		if index.hasError {
			return evalResult{hasError: true, errorVal: index.errorVal}, nil
		}
		if index.value.Kind != ValueInt || !index.value.Dimension.IsDimensionless() {
			return evalResult{}, fmt.Errorf("runtime invariant violation: array index must be Int, got %s", valueTypeName(index.value))
		}
		if index.value.Int < 0 || index.value.Int >= int64(len(target.value.Array)) {
			return evalResult{}, fmt.Errorf("runtime error: index %d out of bounds for array of length %d", index.value.Int, len(target.value.Array))
		}
		return evalResult{value: target.value.Array[index.value.Int]}, nil
	case ast.FieldAccessExpr:
		if identifier, ok := node.Target.(ast.IdentifierExpr); ok {
			if enumDecl, enumExists := i.enums[identifier.Name]; enumExists {
				for _, variant := range enumDecl.Variants {
					if variant == node.Field {
						return evalResult{value: Value{Kind: ValueEnum, Enum: EnumValue{TypeName: identifier.Name, Variant: node.Field}}}, nil
					}
				}
				return evalResult{}, fmt.Errorf("runtime invariant violation: enum '%s' has no variant '%s'", identifier.Name, node.Field)
			}
		}
		target, err := i.evalExpr(env, node.Target)
		if err != nil {
			return evalResult{}, err
		}
		if target.hasError {
			return evalResult{hasError: true, errorVal: target.errorVal}, nil
		}
		if target.value.Kind != ValueRecord {
			return evalResult{}, fmt.Errorf("runtime invariant violation: field access requires record value, got %s", valueTypeName(target.value))
		}
		fieldValue, ok := target.value.Record.Fields[node.Field]
		if !ok {
			return evalResult{}, fmt.Errorf("runtime invariant violation: type '%s' has no field '%s'", target.value.Record.TypeName, node.Field)
		}
		return evalResult{value: fieldValue}, nil
	case ast.ParenExpr:
		return i.evalExpr(env, node.Inner)
	case ast.BinaryExpr:
		left, err := i.evalExpr(env, node.Left)
		if err != nil {
			return evalResult{}, err
		}
		if left.hasError {
			return evalResult{hasError: true, errorVal: left.errorVal}, nil
		}
		right, err := i.evalExpr(env, node.Right)
		if err != nil {
			return evalResult{}, err
		}
		if right.hasError {
			return evalResult{hasError: true, errorVal: right.errorVal}, nil
		}
		value, err := evalBinaryExpr(node.Operator, left.value, right.value)
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: value}, nil
	case ast.RangeExpr:
		value, err := i.evalRangeExpr(env, node)
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: value}, nil
	case ast.PropagateExpr:
		inner, err := i.evalExpr(env, node.Inner)
		if err != nil {
			return evalResult{}, err
		}
		if inner.hasError {
			return inner, nil
		}
		return evalResult{value: inner.value}, nil
	case ast.UnwrapExpr:
		inner, err := i.evalExpr(env, node.Inner)
		if err != nil {
			return evalResult{}, err
		}
		if inner.hasError {
			return evalResult{}, fmt.Errorf("fatal error: %s", inner.errorVal.Error.Message)
		}
		return evalResult{value: inner.value}, nil
	case ast.SwitchExpr:
		return i.evalSwitchExpr(env, node)
	default:
		return evalResult{}, fmt.Errorf("runtime invariant violation: unsupported expression %T", expr)
	}
}

func (i interpreter) evalSwitchExpr(env *environment, expr ast.SwitchExpr) (evalResult, error) {
	subject, err := i.evalExpr(env, expr.Subject)
	if err != nil {
		return evalResult{}, err
	}
	if subject.hasError {
		return evalResult{hasError: true, errorVal: subject.errorVal}, nil
	}

	for _, switchCase := range expr.Cases {
		matched, err := i.switchCaseMatches(env, subject.value, switchCase.Match)
		if err != nil {
			return evalResult{}, err
		}
		if !matched {
			continue
		}
		return i.evalExpr(env, switchCase.Value)
	}
	return i.evalExpr(env, expr.Else)
}

func (i interpreter) switchCaseMatches(env *environment, subject Value, matchExpr ast.Expr) (bool, error) {
	caseValueResult, err := i.evalExpr(env, matchExpr)
	if err != nil {
		return false, err
	}
	if caseValueResult.hasError {
		return false, fmt.Errorf("runtime invariant violation: case label evaluation produced error")
	}
	caseValue := caseValueResult.value
	if !subject.Dimension.IsDimensionless() || !caseValue.Dimension.IsDimensionless() {
		return false, fmt.Errorf("runtime invariant violation: switch case values must be dimensionless")
	}
	if subject.Kind != caseValue.Kind {
		return false, nil
	}

	switch subject.Kind {
	case ValueInt:
		return subject.Int == caseValue.Int, nil
	case ValueFloat:
		return subject.Float == caseValue.Float, nil
	case ValueBool:
		return subject.Bool == caseValue.Bool, nil
	case ValueString:
		return subject.Text == caseValue.Text, nil
	default:
		return false, fmt.Errorf("runtime invariant violation: unsupported switch subject kind %s", subject.Kind)
	}
}

func (i interpreter) evalCallExpr(env *environment, expr ast.CallExpr) (evalResult, error) {
	if expr.Callee == "error" {
		if len(expr.Arguments) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: error() expects 1 argument")
		}
		messageValue, err := i.evalExpr(env, expr.Arguments[0])
		if err != nil {
			return evalResult{}, err
		}
		if messageValue.hasError {
			return evalResult{hasError: true, errorVal: messageValue.errorVal}, nil
		}
		if messageValue.value.Kind != ValueString {
			return evalResult{}, fmt.Errorf("runtime invariant violation: error() expects String, got %s", messageValue.value.Kind)
		}
		return evalResult{value: Value{Kind: ValueError, Error: ErrorValue{Message: messageValue.value.Text}}}, nil
	}
	if builtin.IsName(expr.Callee) {
		return i.evalBuiltinCallExpr(env, expr)
	}

	function, ok := i.functions[expr.Callee]
	if !ok {
		return evalResult{}, fmt.Errorf("runtime invariant violation: undefined function %s", expr.Callee)
	}

	arguments := make([]Value, 0, len(expr.Arguments))
	for _, argumentExpr := range expr.Arguments {
		argument, err := i.evalExpr(env, argumentExpr)
		if err != nil {
			return evalResult{}, err
		}
		if argument.hasError {
			return evalResult{hasError: true, errorVal: argument.errorVal}, nil
		}
		arguments = append(arguments, argument.value)
	}

	result, err := i.executeFunction(function, arguments)
	if err != nil {
		return evalResult{}, err
	}
	if result.hasError {
		return evalResult{hasError: true, errorVal: result.errorVal}, nil
	}
	return evalResult{value: result.value}, nil
}

func (i interpreter) evalBuiltinCallExpr(env *environment, expr ast.CallExpr) (evalResult, error) {
	if expr.Callee == "PlotLine" || expr.Callee == "PlotScatter" {
		value, err := i.evalPlotBuiltinCallExpr(env, expr)
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: value}, nil
	}

	if len(expr.Arguments) != 1 {
		return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects 1 argument", expr.Callee)
	}

	argument, err := i.evalExpr(env, expr.Arguments[0])
	if err != nil {
		return evalResult{}, err
	}
	if argument.hasError {
		return evalResult{hasError: true, errorVal: argument.errorVal}, nil
	}

	switch expr.Callee {
	case "Print":
		_, writeErr := fmt.Fprintln(i.stdout, argument.value.String())
		if writeErr != nil {
			return evalResult{}, fmt.Errorf("runtime error: write stdout: %w", writeErr)
		}
		return evalResult{value: Value{Kind: ValueInt, Int: 0}}, nil
	case "Len":
		if argument.value.Kind != ValueArray {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Len expects Array, got %s", argument.value.Kind)
		}
		return evalResult{value: Value{Kind: ValueInt, Int: int64(len(argument.value.Array))}}, nil
	case "Abs":
		switch argument.value.Kind {
		case ValueInt:
			value := argument.value.Int
			if value < 0 {
				value = -value
			}
			return evalResult{value: Value{Kind: ValueInt, Int: value, Dimension: argument.value.Dimension}}, nil
		case ValueFloat:
			return evalResult{value: Value{Kind: ValueFloat, Float: math.Abs(argument.value.Float), Dimension: argument.value.Dimension}}, nil
		default:
			return evalResult{}, fmt.Errorf("runtime invariant violation: Abs expects Int or Float, got %s", argument.value.Kind)
		}
	case "Sqrt":
		if !argument.value.Dimension.CanSqrt() {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Sqrt requires even dimension exponents")
		}
		value, err := numericValueAsFloat(argument.value, "Sqrt")
		if err != nil {
			return evalResult{}, err
		}
		if value < 0 {
			return evalResult{}, fmt.Errorf("runtime error: Sqrt expects non-negative input, got %s", argument.value.String())
		}
		return evalResult{value: Value{Kind: ValueFloat, Float: math.Sqrt(value), Dimension: argument.value.Dimension.Sqrt()}}, nil
	case "Sin":
		if !argument.value.Dimension.IsDimensionless() {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Sin requires dimensionless input")
		}
		value, err := numericValueAsFloat(argument.value, "Sin")
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: Value{Kind: ValueFloat, Float: math.Sin(value)}}, nil
	case "Cos":
		if !argument.value.Dimension.IsDimensionless() {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Cos requires dimensionless input")
		}
		value, err := numericValueAsFloat(argument.value, "Cos")
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: Value{Kind: ValueFloat, Float: math.Cos(value)}}, nil
	default:
		return evalResult{}, fmt.Errorf("runtime invariant violation: unsupported built-in function %s", expr.Callee)
	}
}

func numericValueAsFloat(value Value, functionName string) (float64, error) {
	switch value.Kind {
	case ValueInt:
		return float64(value.Int), nil
	case ValueFloat:
		return value.Float, nil
	default:
		return 0, fmt.Errorf("runtime invariant violation: %s expects Int or Float, got %s", functionName, value.Kind)
	}
}

func (i interpreter) evalRangeExpr(env *environment, expr ast.RangeExpr) (Value, error) {
	start, err := i.evalExpr(env, expr.Start)
	if err != nil {
		return Value{}, err
	}
	if start.hasError {
		return Value{}, fmt.Errorf("runtime invariant violation: unhandled error reached range start")
	}
	if start.value.Kind != ValueInt || !start.value.Dimension.IsDimensionless() {
		return Value{}, fmt.Errorf("runtime error: range start must be Int, got %s", valueTypeName(start.value))
	}
	end, err := i.evalExpr(env, expr.End)
	if err != nil {
		return Value{}, err
	}
	if end.hasError {
		return Value{}, fmt.Errorf("runtime invariant violation: unhandled error reached range end")
	}
	if end.value.Kind != ValueInt || !end.value.Dimension.IsDimensionless() {
		return Value{}, fmt.Errorf("runtime error: range end must be Int, got %s", valueTypeName(end.value))
	}
	step := int64(1)
	if expr.Step != nil {
		stepValue, err := i.evalExpr(env, expr.Step)
		if err != nil {
			return Value{}, err
		}
		if stepValue.hasError {
			return Value{}, fmt.Errorf("runtime invariant violation: unhandled error reached range step")
		}
		if stepValue.value.Kind != ValueInt || !stepValue.value.Dimension.IsDimensionless() {
			return Value{}, fmt.Errorf("runtime error: range step must be Int, got %s", valueTypeName(stepValue.value))
		}
		step = stepValue.value.Int
	}
	if step <= 0 {
		return Value{}, fmt.Errorf("runtime error: range step must be positive, got %d", step)
	}
	if start.value.Int > end.value.Int {
		return Value{}, fmt.Errorf("runtime error: range start must be less than or equal to end, got %d..%d", start.value.Int, end.value.Int)
	}
	return Value{Kind: ValueRange, Range: RangeValue{Start: start.value.Int, End: end.value.Int, Step: step}}, nil
}

func (i interpreter) evalArrayLiteralExpr(env *environment, expr ast.ArrayLiteralExpr) (Value, error) {
	elements := make([]Value, 0, len(expr.Elements))
	var firstType string
	for idx, elementExpr := range expr.Elements {
		element, err := i.evalExpr(env, elementExpr)
		if err != nil {
			return Value{}, err
		}
		if element.hasError {
			return Value{}, fmt.Errorf("runtime invariant violation: unhandled error reached array literal element %d", idx)
		}
		if element.value.Kind == ValueArray {
			return Value{}, errors.New("runtime invariant violation: nested arrays are not supported")
		}
		if idx == 0 {
			firstType = valueTypeName(element.value)
		} else if valueTypeName(element.value) != firstType {
			return Value{}, fmt.Errorf("runtime invariant violation: array literal has mixed element kinds %s and %s", firstType, valueTypeName(element.value))
		}
		elements = append(elements, element.value)
	}
	return Value{Kind: ValueArray, Array: elements}, nil
}

func (i interpreter) evalRecordLiteralExpr(env *environment, expr ast.RecordLiteralExpr) (evalResult, error) {
	recordDecl, ok := i.records[expr.TypeName]
	if !ok {
		return evalResult{}, fmt.Errorf("runtime invariant violation: unknown record type %s", expr.TypeName)
	}

	fieldValues := make(map[string]Value, len(recordDecl.Fields))
	seen := make(map[string]struct{}, len(expr.Fields))
	for _, field := range expr.Fields {
		if _, exists := seen[field.Name]; exists {
			return evalResult{}, fmt.Errorf("runtime invariant violation: record '%s' field '%s' specified more than once", expr.TypeName, field.Name)
		}
		seen[field.Name] = struct{}{}

		foundDecl := false
		for _, declField := range recordDecl.Fields {
			if declField.Name == field.Name {
				foundDecl = true
				break
			}
		}
		if !foundDecl {
			return evalResult{}, fmt.Errorf("runtime invariant violation: record '%s' has no field '%s'", expr.TypeName, field.Name)
		}
		value, err := i.evalExpr(env, field.Value)
		if err != nil {
			return evalResult{}, err
		}
		if value.hasError {
			return evalResult{hasError: true, errorVal: value.errorVal}, nil
		}
		fieldValues[field.Name] = value.value
	}

	fieldOrder := make([]string, 0, len(recordDecl.Fields))
	for _, field := range recordDecl.Fields {
		if _, exists := seen[field.Name]; !exists {
			return evalResult{}, fmt.Errorf("runtime invariant violation: record '%s' missing field '%s'", expr.TypeName, field.Name)
		}
		fieldOrder = append(fieldOrder, field.Name)
	}
	return evalResult{value: Value{Kind: ValueRecord, Record: RecordValue{TypeName: expr.TypeName, FieldOrder: fieldOrder, Fields: fieldValues}}}, nil
}

func evalBinaryExpr(operator string, left Value, right Value) (Value, error) {
	if left.Kind == ValueRange || right.Kind == ValueRange || left.Kind == ValueString || right.Kind == ValueString || left.Kind == ValueError || right.Kind == ValueError {
		return Value{}, fmt.Errorf("runtime invariant violation: operator %q not defined for %s and %s", operator, valueTypeName(left), valueTypeName(right))
	}
	if left.Kind == ValueArray || right.Kind == ValueArray {
		return evalArrayBinaryExpr(operator, left, right)
	}
	if left.Kind == ValueBool || right.Kind == ValueBool {
		return Value{}, fmt.Errorf("runtime invariant violation: operator %q not defined for %s and %s", operator, valueTypeName(left), valueTypeName(right))
	}

	if operator == "/" && isZero(right) {
		return Value{}, errors.New("runtime error: division by zero")
	}
	if (operator == "+" || operator == "-") && left.Dimension != right.Dimension {
		return Value{}, fmt.Errorf("runtime invariant violation: cannot %s %s and %s", operatorName(operator), formatDimension(left.Dimension), formatDimension(right.Dimension))
	}

	if left.Kind == ValueInt && right.Kind == ValueInt {
		resultDim := combineDimensions(operator, left.Dimension, right.Dimension)
		if operator != "/" || resultDim.IsDimensionless() {
			return evalIntBinaryExpr(operator, left, right)
		}
	}

	leftFloat, err := asFloat(left)
	if err != nil {
		return Value{}, err
	}
	rightFloat, err := asFloat(right)
	if err != nil {
		return Value{}, err
	}

	resultDim := combineDimensions(operator, left.Dimension, right.Dimension)
	switch operator {
	case "+":
		return Value{Kind: ValueFloat, Float: leftFloat + rightFloat, Dimension: resultDim}, nil
	case "-":
		return Value{Kind: ValueFloat, Float: leftFloat - rightFloat, Dimension: resultDim}, nil
	case "*":
		return Value{Kind: ValueFloat, Float: leftFloat * rightFloat, Dimension: resultDim}, nil
	case "/":
		return Value{Kind: ValueFloat, Float: leftFloat / rightFloat, Dimension: resultDim}, nil
	default:
		return Value{}, fmt.Errorf("runtime invariant violation: unsupported operator %q", operator)
	}
}

func evalArrayBinaryExpr(operator string, left Value, right Value) (Value, error) {
	if left.Kind != ValueArray || right.Kind != ValueArray {
		return Value{}, fmt.Errorf("runtime invariant violation: operator %q not defined for %s and %s", operator, valueTypeName(left), valueTypeName(right))
	}
	if len(left.Array) != len(right.Array) {
		return Value{}, fmt.Errorf("runtime error: array length mismatch: %d vs %d", len(left.Array), len(right.Array))
	}

	result := make([]Value, 0, len(left.Array))
	for i := range left.Array {
		element, err := evalBinaryExpr(operator, left.Array[i], right.Array[i])
		if err != nil {
			return Value{}, err
		}
		if element.Kind == ValueArray {
			return Value{}, errors.New("runtime invariant violation: nested array result is not supported")
		}
		result = append(result, element)
	}

	return Value{Kind: ValueArray, Array: result}, nil
}

func evalIntBinaryExpr(operator string, left Value, right Value) (Value, error) {
	resultDim := combineDimensions(operator, left.Dimension, right.Dimension)
	switch operator {
	case "+":
		return Value{Kind: ValueInt, Int: left.Int + right.Int, Dimension: resultDim}, nil
	case "-":
		return Value{Kind: ValueInt, Int: left.Int - right.Int, Dimension: resultDim}, nil
	case "*":
		return Value{Kind: ValueInt, Int: left.Int * right.Int, Dimension: resultDim}, nil
	case "/":
		return Value{Kind: ValueInt, Int: left.Int / right.Int, Dimension: resultDim}, nil
	default:
		return Value{}, fmt.Errorf("runtime invariant violation: unsupported operator %q", operator)
	}
}

func asFloat(value Value) (float64, error) {
	switch value.Kind {
	case ValueInt:
		return float64(value.Int), nil
	case ValueFloat:
		return value.Float, nil
	default:
		return 0, fmt.Errorf("runtime invariant violation: expected numeric value, got %s", value.Kind)
	}
}

func isZero(value Value) bool {
	switch value.Kind {
	case ValueInt:
		return value.Int == 0
	case ValueFloat:
		return value.Float == 0
	default:
		return false
	}
}

func combineDimensions(operator string, left dimension.Dimension, right dimension.Dimension) dimension.Dimension {
	switch operator {
	case "+", "-":
		return left
	case "*":
		return left.Multiply(right)
	case "/":
		return left.Divide(right)
	default:
		return dimension.Zero()
	}
}

func valueTypeName(value Value) string {
	if value.Kind == ValueRecord {
		return value.Record.TypeName
	}
	if value.Kind == ValueEnum {
		return value.Enum.TypeName
	}
	base := string(value.Kind)
	if (value.Kind == ValueInt || value.Kind == ValueFloat) && !value.Dimension.IsDimensionless() {
		base += "<" + value.Dimension.String() + ">"
	}
	return base
}

func formatUnitSuffix(dim dimension.Dimension) string {
	if dim.IsDimensionless() {
		return ""
	}
	return dim.String()
}

func formatDimension(dim dimension.Dimension) string {
	if dim.IsDimensionless() {
		return "dimensionless"
	}
	return dim.String()
}

func operatorName(operator string) string {
	switch operator {
	case "+":
		return "add"
	case "-":
		return "subtract"
	default:
		return operator
	}
}
