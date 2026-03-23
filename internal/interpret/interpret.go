package interpret

import (
	"errors"
	"fmt"
	"strconv"

	"oct/internal/ast"
)

type ValueKind string

const (
	ValueInt   ValueKind = "Int"
	ValueFloat ValueKind = "Float"
	ValueBool  ValueKind = "Bool"
)

type Value struct {
	Kind  ValueKind
	Int   int64
	Float float64
	Bool  bool
}

func (v Value) String() string {
	switch v.Kind {
	case ValueInt:
		return strconv.FormatInt(v.Int, 10)
	case ValueFloat:
		return strconv.FormatFloat(v.Float, 'g', -1, 64)
	case ValueBool:
		return strconv.FormatBool(v.Bool)
	default:
		return "<invalid>"
	}
}

func ExecuteMain(file ast.File) (Value, error) {
	mainFunction, err := findMain(file.Functions)
	if err != nil {
		return Value{}, err
	}

	env := make(environment)
	for _, statement := range mainFunction.Body.Statements {
		result, err := executeStmt(env, statement)
		if err != nil {
			return Value{}, err
		}
		if result.returned {
			return result.value, nil
		}
	}

	return Value{}, errors.New("runtime invariant violation: Main completed without returning")
}

type environment map[string]Value

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
		switch function.ReturnType.Name {
		case string(ValueInt), string(ValueFloat), string(ValueBool):
			return function, nil
		default:
			return ast.FunctionDecl{}, fmt.Errorf("Main must return Int, Float, or Bool, got %s", function.ReturnType.Name)
		}
	}

	return ast.FunctionDecl{}, errors.New("missing Main function")
}

func executeStmt(env environment, stmt ast.Stmt) (stmtResult, error) {
	switch node := stmt.(type) {
	case ast.LetStmt:
		value, err := evalExpr(env, node.Value)
		if err != nil {
			return stmtResult{}, err
		}
		env[node.Name] = value
		return stmtResult{}, nil
	case ast.ReturnStmt:
		value, err := evalExpr(env, node.Value)
		if err != nil {
			return stmtResult{}, err
		}
		return stmtResult{value: value, returned: true}, nil
	default:
		return stmtResult{}, fmt.Errorf("runtime invariant violation: unsupported statement %T", stmt)
	}
}

func evalExpr(env environment, expr ast.Expr) (Value, error) {
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
	case ast.IdentifierExpr:
		value, ok := env[node.Name]
		if !ok {
			return Value{}, fmt.Errorf("runtime invariant violation: undefined variable %s", node.Name)
		}
		return value, nil
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
	default:
		return Value{}, fmt.Errorf("runtime invariant violation: unsupported expression %T", expr)
	}
}

func evalBinaryExpr(operator string, left Value, right Value) (Value, error) {
	if left.Kind == ValueBool || right.Kind == ValueBool {
		return Value{}, fmt.Errorf("runtime invariant violation: operator %q not defined for %s and %s", operator, left.Kind, right.Kind)
	}

	// M3 rejects division by zero for both Int and Float operations to keep runtime errors
	// explicit and deterministic.
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

func evalIntBinaryExpr(operator string, left int64, right int64) (Value, error) {
	// Integer division intentionally follows truncating Int semantics for M3.
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
