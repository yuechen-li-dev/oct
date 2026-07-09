package consteval

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
)

type Value struct {
	Type    ast.TypeRef
	Int32   int64
	Bool    bool
	IsKnown bool
}

func Eval(expr ast.Expr, env map[string]Value) (Value, error) {
	switch e := expr.(type) {
	case ast.IntegerLiteral:
		value, err := strconv.ParseInt(strings.TrimRight(e.Value, "uU"), 10, 32)
		if err != nil {
			return Value{}, fmt.Errorf("invalid integer literal %s", e.Value)
		}
		typ := ast.TypeRef{Name: "i32"}
		if strings.HasSuffix(e.Value, "u") || strings.HasSuffix(e.Value, "U") {
			typ = ast.TypeRef{Name: "u32"}
		}
		return Value{Type: typ, Int32: value, IsKnown: true}, nil
	case ast.BoolLiteral:
		return Value{Type: ast.TypeRef{Name: "bool"}, Bool: e.Value, IsKnown: true}, nil
	case ast.FloatLiteral:
		return Value{}, fmt.Errorf("float constant expressions are not implemented in SDSL-V M6")
	case ast.IdentifierExpr:
		if env == nil {
			return Value{}, fmt.Errorf("unknown constant identifier %s", e.Name)
		}
		value, ok := env[e.Name]
		if !ok {
			return Value{}, fmt.Errorf("unknown constant identifier %s", e.Name)
		}
		return value, nil
	case ast.ParenExpr:
		return Eval(e.Inner, env)
	case ast.UnaryExpr:
		value, err := Eval(e.Operand, env)
		if err != nil {
			return Value{}, err
		}
		switch e.Operator {
		case "-":
			if !IsInteger(value.Type) {
				return Value{}, fmt.Errorf("unary - requires integer constant operand")
			}
			value.Int32 = -value.Int32
			return value, nil
		case "not":
			if value.Type.Name != "bool" {
				return Value{}, fmt.Errorf("operator `not` requires bool operand")
			}
			value.Bool = !value.Bool
			return value, nil
		default:
			return Value{}, fmt.Errorf("unsupported unary constant operator %s", e.Operator)
		}
	case ast.FieldAccessExpr:
		path, ok := constFieldPath(e)
		if !ok {
			return Value{}, fmt.Errorf("unsupported constant field access")
		}
		if env == nil {
			return Value{}, fmt.Errorf("unknown constant field %s", path)
		}
		if value, ok := env[path]; ok {
			return value, nil
		}
		if leaf := lastPathSegment(path); leaf != "" {
			if value, ok := env[leaf]; ok {
				return value, nil
			}
		}
		return Value{}, fmt.Errorf("unknown constant field %s", path)
	case ast.BinaryExpr:
		left, err := Eval(e.Left, env)
		if err != nil {
			return Value{}, err
		}
		right, err := Eval(e.Right, env)
		if err != nil {
			return Value{}, err
		}
		switch e.Operator {
		case "+", "-", "*", "/", "%":
			if !IsInteger(left.Type) || !IsInteger(right.Type) {
				return Value{}, fmt.Errorf("arithmetic constant expressions require integer operands")
			}
			if (e.Operator == "/" || e.Operator == "%") && right.Int32 == 0 {
				return Value{}, fmt.Errorf("division by zero in constant expression")
			}
			out := Value{Type: left.Type, Int32: left.Int32, IsKnown: true}
			if left.Type.Name == "u32" || right.Type.Name == "u32" {
				out.Type = ast.TypeRef{Name: "u32"}
			}
			switch e.Operator {
			case "+":
				out.Int32 = left.Int32 + right.Int32
			case "-":
				out.Int32 = left.Int32 - right.Int32
			case "*":
				out.Int32 = left.Int32 * right.Int32
			case "/":
				out.Int32 = left.Int32 / right.Int32
			case "%":
				out.Int32 = left.Int32 % right.Int32
			}
			return out, nil
		case "==", "!=", "<", "<=", ">", ">=":
			if left.Type.Name == "bool" && right.Type.Name == "bool" {
				return compareBool(e.Operator, left.Bool, right.Bool)
			}
			if !IsInteger(left.Type) || !IsInteger(right.Type) {
				return Value{}, fmt.Errorf("comparison constant expressions require integer or bool operands")
			}
			return compareInt(e.Operator, left.Int32, right.Int32)
		case "and", "or":
			if left.Type.Name != "bool" || right.Type.Name != "bool" {
				return Value{}, fmt.Errorf("operator `%s` requires bool operands", e.Operator)
			}
			result := left.Bool && right.Bool
			if e.Operator == "or" {
				result = left.Bool || right.Bool
			}
			return Value{Type: ast.TypeRef{Name: "bool"}, Bool: result, IsKnown: true}, nil
		default:
			return Value{}, fmt.Errorf("unsupported constant operator %s", e.Operator)
		}
	default:
		return Value{}, fmt.Errorf("expression is not a valid SDSL-V M6 constant expression")
	}
}

func LiteralExpr(value Value) ast.Expr {
	if value.Type.Name == "bool" {
		return ast.BoolLiteral{Value: value.Bool}
	}
	text := strconv.FormatInt(value.Int32, 10)
	if value.Type.Name == "u32" {
		text += "u"
	}
	return ast.IntegerLiteral{Value: text}
}

func IsInteger(ref ast.TypeRef) bool {
	return ref.Name == "i32" || ref.Name == "u32"
}

func compareInt(op string, left, right int64) (Value, error) {
	result := false
	switch op {
	case "==":
		result = left == right
	case "!=":
		result = left != right
	case "<":
		result = left < right
	case "<=":
		result = left <= right
	case ">":
		result = left > right
	case ">=":
		result = left >= right
	default:
		return Value{}, fmt.Errorf("unsupported comparison operator %s", op)
	}
	return Value{Type: ast.TypeRef{Name: "bool"}, Bool: result, IsKnown: true}, nil
}

func compareBool(op string, left, right bool) (Value, error) {
	result := false
	switch op {
	case "==":
		result = left == right
	case "!=":
		result = left != right
	default:
		return Value{}, fmt.Errorf("bool constant expressions support only == and !=")
	}
	return Value{Type: ast.TypeRef{Name: "bool"}, Bool: result, IsKnown: true}, nil
}

func constFieldPath(expr ast.Expr) (string, bool) {
	switch e := expr.(type) {
	case ast.IdentifierExpr:
		return e.Name, true
	case ast.FieldAccessExpr:
		prefix, ok := constFieldPath(e.Target)
		if !ok {
			return "", false
		}
		if prefix == "" {
			return e.Field, true
		}
		return prefix + "." + e.Field, true
	default:
		return "", false
	}
}

func lastPathSegment(path string) string {
	if idx := strings.LastIndexByte(path, '.'); idx >= 0 && idx+1 < len(path) {
		return path[idx+1:]
	}
	return path
}
