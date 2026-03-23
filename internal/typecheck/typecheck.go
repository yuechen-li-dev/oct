package typecheck

import (
	"fmt"

	"oct/internal/ast"
)

type Type string

const (
	TypeInt   Type = "Int"
	TypeFloat Type = "Float"
	TypeBool  Type = "Bool"
)

func Check(file ast.File) error {
	checker := checker{}
	return checker.checkFile(file)
}

type checker struct{}

type env map[string]Type

func (c checker) checkFile(file ast.File) error {
	for _, function := range file.Functions {
		if err := c.checkFunction(function); err != nil {
			return err
		}
	}
	return nil
}

func (c checker) checkFunction(function ast.FunctionDecl) error {
	returnType, err := resolveType(function.ReturnType)
	if err != nil {
		return fmt.Errorf("function %s: %w", function.Name, err)
	}

	scope := make(env, len(function.Parameters))
	for _, parameter := range function.Parameters {
		parameterType, err := resolveType(parameter.Type)
		if err != nil {
			return fmt.Errorf("function %s: parameter %s: %w", function.Name, parameter.Name, err)
		}
		scope[parameter.Name] = parameterType
	}

	hasReturn := false
	for _, statement := range function.Body.Statements {
		switch stmt := statement.(type) {
		case ast.LetStmt:
			valueType, err := c.checkExpr(scope, stmt.Value)
			if err != nil {
				return fmt.Errorf("function %s: let %s: %w", function.Name, stmt.Name, err)
			}
			scope[stmt.Name] = valueType
		case ast.ReturnStmt:
			hasReturn = true
			valueType, err := c.checkExpr(scope, stmt.Value)
			if err != nil {
				return fmt.Errorf("function %s: %w", function.Name, err)
			}
			if valueType != returnType {
				return fmt.Errorf("function %s: function expects %s, but return is %s", function.Name, returnType, valueType)
			}
		default:
			return fmt.Errorf("function %s: unsupported statement %T", function.Name, statement)
		}
	}

	if !hasReturn {
		return fmt.Errorf("function %s: missing return statement", function.Name)
	}

	return nil
}

func (c checker) checkExpr(scope env, expr ast.Expr) (Type, error) {
	switch node := expr.(type) {
	case ast.IntegerLiteral:
		return TypeInt, nil
	case ast.FloatLiteral:
		return TypeFloat, nil
	case ast.BoolLiteral:
		return TypeBool, nil
	case ast.IdentifierExpr:
		valueType, ok := scope[node.Name]
		if !ok {
			return "", fmt.Errorf("undefined variable: %s", node.Name)
		}
		return valueType, nil
	case ast.ParenExpr:
		return c.checkExpr(scope, node.Inner)
	case ast.BinaryExpr:
		leftType, err := c.checkExpr(scope, node.Left)
		if err != nil {
			return "", err
		}
		rightType, err := c.checkExpr(scope, node.Right)
		if err != nil {
			return "", err
		}
		return checkBinaryExpr(node.Operator, leftType, rightType)
	default:
		return "", fmt.Errorf("unsupported expression %T", expr)
	}
}

func resolveType(typeRef ast.TypeRef) (Type, error) {
	switch Type(typeRef.Name) {
	case TypeInt, TypeFloat, TypeBool:
		return Type(typeRef.Name), nil
	default:
		return "", fmt.Errorf("unknown type: %s", typeRef.Name)
	}
}

func checkBinaryExpr(operator string, leftType Type, rightType Type) (Type, error) {
	if leftType == TypeBool || rightType == TypeBool {
		return "", fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}

	if leftType == TypeFloat || rightType == TypeFloat {
		return TypeFloat, nil
	}
	return TypeInt, nil
}
