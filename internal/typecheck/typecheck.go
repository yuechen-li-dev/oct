package typecheck

import (
	"fmt"

	"oct/internal/ast"
)

type BaseType string

const (
	BaseTypeInt   BaseType = "Int"
	BaseTypeFloat BaseType = "Float"
	BaseTypeBool  BaseType = "Bool"
	BaseTypeRange BaseType = "Range"
)

type Type struct {
	Base    BaseType
	IsArray bool
}

func (t Type) String() string {
	if t.IsArray {
		return string(t.Base) + "[]"
	}
	return string(t.Base)
}

func Check(file ast.File) error {
	checker := checker{}
	return checker.checkFile(file)
}

type checker struct{}

type scope struct {
	parent *scope
	values map[string]Type
}

func newScope(parent *scope) *scope {
	return &scope{parent: parent, values: make(map[string]Type)}
}

func (s *scope) define(name string, valueType Type) {
	s.values[name] = valueType
}

func (s *scope) lookup(name string) (Type, bool) {
	for current := s; current != nil; current = current.parent {
		valueType, ok := current.values[name]
		if ok {
			return valueType, true
		}
	}
	return Type{}, false
}

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

	functionScope := newScope(nil)
	for _, parameter := range function.Parameters {
		parameterType, err := resolveType(parameter.Type)
		if err != nil {
			return fmt.Errorf("function %s: parameter %s: %w", function.Name, parameter.Name, err)
		}
		functionScope.define(parameter.Name, parameterType)
	}

	hasReturn, err := c.checkBlock(functionScope, function.Body, returnType, function.Name)
	if err != nil {
		return err
	}
	if !hasReturn {
		return fmt.Errorf("function %s: missing return statement", function.Name)
	}

	return nil
}

func (c checker) checkBlock(parent *scope, block ast.Block, returnType Type, functionName string) (bool, error) {
	blockScope := newScope(parent)
	hasReturn := false
	for _, statement := range block.Statements {
		returned, err := c.checkStmt(blockScope, statement, returnType, functionName)
		if err != nil {
			return false, err
		}
		if returned {
			hasReturn = true
		}
	}
	return hasReturn, nil
}

func (c checker) checkStmt(scope *scope, stmt ast.Stmt, returnType Type, functionName string) (bool, error) {
	switch node := stmt.(type) {
	case ast.LetStmt:
		valueType, err := c.checkExpr(scope, node.Value)
		if err != nil {
			return false, fmt.Errorf("function %s: let %s: %w", functionName, node.Name, err)
		}
		scope.define(node.Name, valueType)
		return false, nil
	case ast.ReturnStmt:
		valueType, err := c.checkExpr(scope, node.Value)
		if err != nil {
			return false, fmt.Errorf("function %s: %w", functionName, err)
		}
		if valueType != returnType {
			return false, fmt.Errorf("function %s: function expects %s, but return is %s", functionName, returnType, valueType)
		}
		return true, nil
	case ast.ForStmt:
		rangeType, err := c.checkExpr(scope, node.Range)
		if err != nil {
			return false, fmt.Errorf("function %s: for %s: %w", functionName, node.Name, err)
		}
		if rangeType != (Type{Base: BaseTypeRange}) {
			return false, fmt.Errorf("function %s: for %s: expected Range, got %s", functionName, node.Name, rangeType)
		}
		loopScope := newScope(scope)
		loopScope.define(node.Name, Type{Base: BaseTypeInt})
		_, err = c.checkBlock(loopScope, node.Body, returnType, functionName)
		if err != nil {
			return false, err
		}
		return false, nil
	default:
		return false, fmt.Errorf("function %s: unsupported statement %T", functionName, stmt)
	}
}

func (c checker) checkExpr(scope *scope, expr ast.Expr) (Type, error) {
	switch node := expr.(type) {
	case ast.IntegerLiteral:
		return Type{Base: BaseTypeInt}, nil
	case ast.FloatLiteral:
		return Type{Base: BaseTypeFloat}, nil
	case ast.BoolLiteral:
		return Type{Base: BaseTypeBool}, nil
	case ast.ArrayLiteralExpr:
		return c.checkArrayLiteralExpr(scope, node)
	case ast.IdentifierExpr:
		valueType, ok := scope.lookup(node.Name)
		if !ok {
			return Type{}, fmt.Errorf("undefined variable: %s", node.Name)
		}
		return valueType, nil
	case ast.IndexExpr:
		targetType, err := c.checkExpr(scope, node.Target)
		if err != nil {
			return Type{}, err
		}
		if !targetType.IsArray {
			return Type{}, fmt.Errorf("cannot index non-array value of type %s", targetType)
		}
		indexType, err := c.checkExpr(scope, node.Index)
		if err != nil {
			return Type{}, err
		}
		if indexType != (Type{Base: BaseTypeInt}) {
			return Type{}, fmt.Errorf("array index must be Int, got %s", indexType)
		}
		return Type{Base: targetType.Base}, nil
	case ast.ParenExpr:
		return c.checkExpr(scope, node.Inner)
	case ast.BinaryExpr:
		leftType, err := c.checkExpr(scope, node.Left)
		if err != nil {
			return Type{}, err
		}
		rightType, err := c.checkExpr(scope, node.Right)
		if err != nil {
			return Type{}, err
		}
		return checkBinaryExpr(node.Operator, leftType, rightType)
	case ast.RangeExpr:
		startType, err := c.checkExpr(scope, node.Start)
		if err != nil {
			return Type{}, err
		}
		if startType != (Type{Base: BaseTypeInt}) {
			return Type{}, fmt.Errorf("range start must be Int, got %s", startType)
		}
		endType, err := c.checkExpr(scope, node.End)
		if err != nil {
			return Type{}, err
		}
		if endType != (Type{Base: BaseTypeInt}) {
			return Type{}, fmt.Errorf("range end must be Int, got %s", endType)
		}
		if node.Step != nil {
			stepType, err := c.checkExpr(scope, node.Step)
			if err != nil {
				return Type{}, err
			}
			if stepType != (Type{Base: BaseTypeInt}) {
				return Type{}, fmt.Errorf("range step must be Int, got %s", stepType)
			}
			if integerLiteral, ok := node.Step.(ast.IntegerLiteral); ok && integerLiteral.Value == "0" {
				return Type{}, fmt.Errorf("range step must be positive, got 0")
			}
		}
		return Type{Base: BaseTypeRange}, nil
	default:
		return Type{}, fmt.Errorf("unsupported expression %T", expr)
	}
}

func (c checker) checkArrayLiteralExpr(scope *scope, expr ast.ArrayLiteralExpr) (Type, error) {
	if len(expr.Elements) == 0 {
		return Type{}, fmt.Errorf("empty array literals are not supported")
	}

	firstType, err := c.checkExpr(scope, expr.Elements[0])
	if err != nil {
		return Type{}, err
	}
	if firstType.IsArray {
		return Type{}, fmt.Errorf("nested arrays are not supported")
	}

	for _, element := range expr.Elements[1:] {
		elementType, err := c.checkExpr(scope, element)
		if err != nil {
			return Type{}, err
		}
		if elementType != firstType {
			return Type{}, fmt.Errorf("array literal elements must all have the same type; found %s and %s", firstType, elementType)
		}
	}

	return Type{Base: firstType.Base, IsArray: true}, nil
}

func resolveType(typeRef ast.TypeRef) (Type, error) {
	baseType, err := resolveBaseType(typeRef.Name)
	if err != nil {
		return Type{}, err
	}
	if typeRef.IsArray {
		return Type{Base: baseType, IsArray: true}, nil
	}
	return Type{Base: baseType}, nil
}

func resolveBaseType(name string) (BaseType, error) {
	switch BaseType(name) {
	case BaseTypeInt, BaseTypeFloat, BaseTypeBool:
		return BaseType(name), nil
	default:
		return "", fmt.Errorf("unknown type: %s", name)
	}
}

func checkBinaryExpr(operator string, leftType Type, rightType Type) (Type, error) {
	if leftType.Base == BaseTypeRange || rightType.Base == BaseTypeRange {
		return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}
	if leftType.IsArray || rightType.IsArray {
		return checkArrayBinaryExpr(operator, leftType, rightType)
	}

	if leftType.Base == BaseTypeBool || rightType.Base == BaseTypeBool {
		return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}

	if leftType.Base == BaseTypeFloat || rightType.Base == BaseTypeFloat {
		return Type{Base: BaseTypeFloat}, nil
	}
	return Type{Base: BaseTypeInt}, nil
}

func checkArrayBinaryExpr(operator string, leftType Type, rightType Type) (Type, error) {
	if !leftType.IsArray || !rightType.IsArray {
		return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}
	if leftType.Base != rightType.Base {
		return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}
	if leftType.Base == BaseTypeBool {
		return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}
	return leftType, nil
}
