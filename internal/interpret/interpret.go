package interpret

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"oct/internal/ast"
)

type ValueKind string

const (
	ValueInt   ValueKind = "Int"
	ValueFloat ValueKind = "Float"
	ValueBool  ValueKind = "Bool"
	ValueArray ValueKind = "Array"
	ValueRange ValueKind = "Range"
)

type Value struct {
	Kind  ValueKind
	Int   int64
	Float float64
	Bool  bool
	Array []Value
	Range RangeValue
}

type RangeValue struct {
	Start int64
	End   int64
	Step  int64
}

func (v Value) String() string {
	switch v.Kind {
	case ValueInt:
		return strconv.FormatInt(v.Int, 10)
	case ValueFloat:
		return strconv.FormatFloat(v.Float, 'g', -1, 64)
	case ValueBool:
		return strconv.FormatBool(v.Bool)
	case ValueArray:
		parts := make([]string, 0, len(v.Array))
		for _, element := range v.Array {
			parts = append(parts, element.String())
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case ValueRange:
		return fmt.Sprintf("%d..%d step %d", v.Range.Start, v.Range.End, v.Range.Step)
	default:
		return "<invalid>"
	}
}

func ExecuteMain(file ast.File) (Value, error) {
	mainFunction, err := findMain(file.Functions)
	if err != nil {
		return Value{}, err
	}

	env := newEnvironment(nil)
	result, err := executeBlock(env, mainFunction.Body)
	if err != nil {
		return Value{}, err
	}
	if result.returned {
		return result.value, nil
	}

	return Value{}, errors.New("runtime invariant violation: Main completed without returning")
}

type environment struct {
	parent *environment
	values map[string]Value
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

type stmtResult struct {
	value    Value
	returned bool
}

func findMain(functions []ast.FunctionDecl) (ast.FunctionDecl, error) {
	for _, function := range functions {
		if function.Name != "Main" {
			continue
		}
		if len(function.Parameters) != 0 {
			return ast.FunctionDecl{}, errors.New("Main must not have parameters")
		}
		if isSupportedMainReturnType(function.ReturnType) {
			return function, nil
		}
		if function.ReturnType.IsArray {
			return ast.FunctionDecl{}, fmt.Errorf("Main must return Int, Float, Bool, Int[], Float[], or Bool[], got %s[]", function.ReturnType.Name)
		}
		return ast.FunctionDecl{}, fmt.Errorf("Main must return Int, Float, Bool, Int[], Float[], or Bool[], got %s", function.ReturnType.Name)
	}

	return ast.FunctionDecl{}, errors.New("missing Main function")
}

func isSupportedMainReturnType(typeRef ast.TypeRef) bool {
	if typeRef.IsArray {
		switch typeRef.Name {
		case string(ValueInt), string(ValueFloat), string(ValueBool):
			return true
		default:
			return false
		}
	}

	switch typeRef.Name {
	case string(ValueInt), string(ValueFloat), string(ValueBool):
		return true
	default:
		return false
	}
}

func executeBlock(parent *environment, block ast.Block) (stmtResult, error) {
	blockEnv := newEnvironment(parent)
	for _, statement := range block.Statements {
		result, err := executeStmt(blockEnv, statement)
		if err != nil {
			return stmtResult{}, err
		}
		if result.returned {
			return result, nil
		}
	}
	return stmtResult{}, nil
}

func executeStmt(env *environment, stmt ast.Stmt) (stmtResult, error) {
	switch node := stmt.(type) {
	case ast.LetStmt:
		value, err := evalExpr(env, node.Value)
		if err != nil {
			return stmtResult{}, err
		}
		env.define(node.Name, value)
		return stmtResult{}, nil
	case ast.ReturnStmt:
		value, err := evalExpr(env, node.Value)
		if err != nil {
			return stmtResult{}, err
		}
		return stmtResult{value: value, returned: true}, nil
	case ast.ForStmt:
		rangeValue, err := evalExpr(env, node.Range)
		if err != nil {
			return stmtResult{}, err
		}
		if rangeValue.Kind != ValueRange {
			return stmtResult{}, fmt.Errorf("runtime invariant violation: for loop expected Range, got %s", rangeValue.Kind)
		}
		for current := rangeValue.Range.Start; current < rangeValue.Range.End; current += rangeValue.Range.Step {
			iterationEnv := newEnvironment(env)
			iterationEnv.define(node.Name, Value{Kind: ValueInt, Int: current})
			result, err := executeBlock(iterationEnv, node.Body)
			if err != nil {
				return stmtResult{}, err
			}
			if result.returned {
				return result, nil
			}
		}
		return stmtResult{}, nil
	default:
		return stmtResult{}, fmt.Errorf("runtime invariant violation: unsupported statement %T", stmt)
	}
}

func evalExpr(env *environment, expr ast.Expr) (Value, error) {
	switch node := expr.(type) {
	case ast.IntegerLiteral:
		value, err := strconv.ParseInt(node.Value, 10, 64)
		if err != nil {
			return Value{}, fmt.Errorf("runtime invariant violation: invalid integer literal %q: %w", node.Value, err)
		}
		return Value{Kind: ValueInt, Int: value}, nil
	case ast.FloatLiteral:
		value, err := strconv.ParseFloat(node.Value, 64)
		if err != nil {
			return Value{}, fmt.Errorf("runtime invariant violation: invalid float literal %q: %w", node.Value, err)
		}
		return Value{Kind: ValueFloat, Float: value}, nil
	case ast.BoolLiteral:
		return Value{Kind: ValueBool, Bool: node.Value}, nil
	case ast.ArrayLiteralExpr:
		return evalArrayLiteralExpr(env, node)
	case ast.IdentifierExpr:
		value, ok := env.lookup(node.Name)
		if !ok {
			return Value{}, fmt.Errorf("runtime invariant violation: undefined variable %s", node.Name)
		}
		return value, nil
	case ast.IndexExpr:
		target, err := evalExpr(env, node.Target)
		if err != nil {
			return Value{}, err
		}
		if target.Kind != ValueArray {
			return Value{}, fmt.Errorf("runtime invariant violation: cannot index non-array value of kind %s", target.Kind)
		}
		index, err := evalExpr(env, node.Index)
		if err != nil {
			return Value{}, err
		}
		if index.Kind != ValueInt {
			return Value{}, fmt.Errorf("runtime invariant violation: array index must be Int, got %s", index.Kind)
		}
		if index.Int < 0 || index.Int >= int64(len(target.Array)) {
			return Value{}, fmt.Errorf("runtime error: index %d out of bounds for array of length %d", index.Int, len(target.Array))
		}
		return target.Array[index.Int], nil
	case ast.ParenExpr:
		return evalExpr(env, node.Inner)
	case ast.BinaryExpr:
		left, err := evalExpr(env, node.Left)
		if err != nil {
			return Value{}, err
		}
		right, err := evalExpr(env, node.Right)
		if err != nil {
			return Value{}, err
		}
		return evalBinaryExpr(node.Operator, left, right)
	case ast.RangeExpr:
		return evalRangeExpr(env, node)
	default:
		return Value{}, fmt.Errorf("runtime invariant violation: unsupported expression %T", expr)
	}
}

func evalRangeExpr(env *environment, expr ast.RangeExpr) (Value, error) {
	start, err := evalExpr(env, expr.Start)
	if err != nil {
		return Value{}, err
	}
	if start.Kind != ValueInt {
		return Value{}, fmt.Errorf("runtime error: range start must be Int, got %s", start.Kind)
	}
	end, err := evalExpr(env, expr.End)
	if err != nil {
		return Value{}, err
	}
	if end.Kind != ValueInt {
		return Value{}, fmt.Errorf("runtime error: range end must be Int, got %s", end.Kind)
	}
	step := int64(1)
	if expr.Step != nil {
		stepValue, err := evalExpr(env, expr.Step)
		if err != nil {
			return Value{}, err
		}
		if stepValue.Kind != ValueInt {
			return Value{}, fmt.Errorf("runtime error: range step must be Int, got %s", stepValue.Kind)
		}
		step = stepValue.Int
	}
	if step <= 0 {
		return Value{}, fmt.Errorf("runtime error: range step must be positive, got %d", step)
	}
	if start.Int > end.Int {
		return Value{}, fmt.Errorf("runtime error: range start must be less than or equal to end, got %d..%d", start.Int, end.Int)
	}
	return Value{Kind: ValueRange, Range: RangeValue{Start: start.Int, End: end.Int, Step: step}}, nil
}

func evalArrayLiteralExpr(env *environment, expr ast.ArrayLiteralExpr) (Value, error) {
	elements := make([]Value, 0, len(expr.Elements))
	var elementKind ValueKind
	for i, elementExpr := range expr.Elements {
		element, err := evalExpr(env, elementExpr)
		if err != nil {
			return Value{}, err
		}
		if element.Kind == ValueArray {
			return Value{}, errors.New("runtime invariant violation: nested arrays are not supported")
		}
		if i == 0 {
			elementKind = element.Kind
		} else if element.Kind != elementKind {
			return Value{}, fmt.Errorf("runtime invariant violation: array literal has mixed element kinds %s and %s", elementKind, element.Kind)
		}
		elements = append(elements, element)
	}
	return Value{Kind: ValueArray, Array: elements}, nil
}

func evalBinaryExpr(operator string, left Value, right Value) (Value, error) {
	if left.Kind == ValueRange || right.Kind == ValueRange {
		return Value{}, fmt.Errorf("runtime invariant violation: operator %q not defined for %s and %s", operator, left.Kind, right.Kind)
	}
	if left.Kind == ValueArray || right.Kind == ValueArray {
		return evalArrayBinaryExpr(operator, left, right)
	}
	if left.Kind == ValueBool || right.Kind == ValueBool {
		return Value{}, fmt.Errorf("runtime invariant violation: operator %q not defined for %s and %s", operator, left.Kind, right.Kind)
	}

	if operator == "/" && isZero(right) {
		return Value{}, errors.New("runtime error: division by zero")
	}

	if left.Kind == ValueInt && right.Kind == ValueInt {
		return evalIntBinaryExpr(operator, left.Int, right.Int)
	}

	leftFloat, err := asFloat(left)
	if err != nil {
		return Value{}, err
	}
	rightFloat, err := asFloat(right)
	if err != nil {
		return Value{}, err
	}

	switch operator {
	case "+":
		return Value{Kind: ValueFloat, Float: leftFloat + rightFloat}, nil
	case "-":
		return Value{Kind: ValueFloat, Float: leftFloat - rightFloat}, nil
	case "*":
		return Value{Kind: ValueFloat, Float: leftFloat * rightFloat}, nil
	case "/":
		return Value{Kind: ValueFloat, Float: leftFloat / rightFloat}, nil
	default:
		return Value{}, fmt.Errorf("runtime invariant violation: unsupported operator %q", operator)
	}
}

func evalArrayBinaryExpr(operator string, left Value, right Value) (Value, error) {
	if left.Kind != ValueArray || right.Kind != ValueArray {
		return Value{}, fmt.Errorf("runtime invariant violation: operator %q not defined for %s and %s", operator, left.Kind, right.Kind)
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

func evalIntBinaryExpr(operator string, left int64, right int64) (Value, error) {
	switch operator {
	case "+":
		return Value{Kind: ValueInt, Int: left + right}, nil
	case "-":
		return Value{Kind: ValueInt, Int: left - right}, nil
	case "*":
		return Value{Kind: ValueInt, Int: left * right}, nil
	case "/":
		return Value{Kind: ValueInt, Int: left / right}, nil
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
