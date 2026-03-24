package typecheck

import (
	"fmt"

	"oct/internal/ast"
	"oct/internal/dimension"
)

func isReservedBuiltinFunctionName(name string) bool {
	switch name {
	case "Len", "Abs", "Sqrt", "Sin", "Cos", "PlotLine", "PlotScatter":
		return true
	default:
		return false
	}
}

type BaseType string

const (
	BaseTypeInt    BaseType = "Int"
	BaseTypeFloat  BaseType = "Float"
	BaseTypeBool   BaseType = "Bool"
	BaseTypeRange  BaseType = "Range"
	BaseTypeString BaseType = "String"
	BaseTypeError  BaseType = "Error"
)

type Type struct {
	Base      BaseType
	Dimension dimension.Dimension
	IsArray   bool
}

func (t Type) String() string {
	base := string(t.Base)
	if isNumericBaseType(t.Base) && !t.Dimension.IsDimensionless() {
		base += "<" + t.Dimension.String() + ">"
	}
	if t.IsArray {
		return base + "[]"
	}
	return base
}

type ExprType struct {
	ValueType Type
	Fallible  bool
}

type functionSignature struct {
	parameters []Type
	returnType Type
	isFallible bool
}

type functionContext struct {
	name       string
	returnType Type
	isFallible bool
}

func Check(file ast.File) error {
	checker := checker{functions: make(map[string]functionSignature)}
	return checker.checkFile(file)
}

type checker struct {
	functions map[string]functionSignature
}

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
		if isReservedBuiltinFunctionName(function.Name) {
			return fmt.Errorf("function %s: cannot redeclare built-in function", function.Name)
		}
		signature, err := c.resolveFunctionSignature(function)
		if err != nil {
			return fmt.Errorf("function %s: %w", function.Name, err)
		}
		if _, exists := c.functions[function.Name]; exists {
			return fmt.Errorf("duplicate function: %s", function.Name)
		}
		c.functions[function.Name] = signature
	}

	for _, function := range file.Functions {
		if err := c.checkFunction(function); err != nil {
			return err
		}
	}
	return nil
}

func (c checker) resolveFunctionSignature(function ast.FunctionDecl) (functionSignature, error) {
	returnType, err := resolveType(function.ReturnType)
	if err != nil {
		return functionSignature{}, err
	}
	if function.IsFallible {
		if function.ErrorType.IsArray || function.ErrorType.Name != string(BaseTypeError) || function.ErrorType.HasUnit {
			return functionSignature{}, fmt.Errorf("only built-in Error is allowed in fallible signatures")
		}
	}

	parameters := make([]Type, 0, len(function.Parameters))
	for _, parameter := range function.Parameters {
		parameterType, err := resolveType(parameter.Type)
		if err != nil {
			return functionSignature{}, fmt.Errorf("parameter %s: %w", parameter.Name, err)
		}
		parameters = append(parameters, parameterType)
	}

	return functionSignature{parameters: parameters, returnType: returnType, isFallible: function.IsFallible}, nil
}

func (c checker) checkFunction(function ast.FunctionDecl) error {
	signature := c.functions[function.Name]
	functionScope := newScope(nil)
	for i, parameter := range function.Parameters {
		functionScope.define(parameter.Name, signature.parameters[i])
	}

	ctx := functionContext{name: function.Name, returnType: signature.returnType, isFallible: signature.isFallible}
	hasReturn, err := c.checkBlock(functionScope, function.Body, ctx)
	if err != nil {
		return err
	}
	if !hasReturn {
		return fmt.Errorf("function %s: missing return statement", function.Name)
	}

	return nil
}

func (c checker) checkBlock(parent *scope, block ast.Block, ctx functionContext) (bool, error) {
	blockScope := newScope(parent)
	hasReturn := false
	for _, statement := range block.Statements {
		returned, err := c.checkStmt(blockScope, statement, ctx)
		if err != nil {
			return false, err
		}
		if returned {
			hasReturn = true
		}
	}
	return hasReturn, nil
}

func (c checker) checkStmt(scope *scope, stmt ast.Stmt, ctx functionContext) (bool, error) {
	switch node := stmt.(type) {
	case ast.LetStmt:
		valueType, err := c.checkExpr(scope, node.Value, ctx)
		if err != nil {
			return false, fmt.Errorf("function %s: let %s: %w", ctx.name, node.Name, err)
		}
		if valueType.Fallible {
			return false, fmt.Errorf("function %s: let %s: fallible expression must be handled explicitly", ctx.name, node.Name)
		}
		scope.define(node.Name, valueType.ValueType)
		return false, nil
	case ast.ReturnStmt:
		valueType, err := c.checkExpr(scope, node.Value, ctx)
		if err != nil {
			return false, fmt.Errorf("function %s: %w", ctx.name, err)
		}
		if valueType.Fallible {
			return false, fmt.Errorf("function %s: return value must not be fallible; handle it with '?', '!', or match", ctx.name)
		}
		if ctx.isFallible {
			if isAssignable(valueType.ValueType, ctx.returnType) || valueType.ValueType == (Type{Base: BaseTypeError}) {
				return true, nil
			}
			return false, fmt.Errorf("function %s: function expects %s or Error, but return is %s", ctx.name, ctx.returnType, valueType.ValueType)
		}
		if !isAssignable(valueType.ValueType, ctx.returnType) {
			return false, fmt.Errorf("function %s: function expects %s, but return is %s", ctx.name, ctx.returnType, valueType.ValueType)
		}
		return true, nil
	case ast.ForStmt:
		rangeType, err := c.checkExpr(scope, node.Range, ctx)
		if err != nil {
			return false, fmt.Errorf("function %s: for %s: %w", ctx.name, node.Name, err)
		}
		if rangeType.Fallible {
			return false, fmt.Errorf("function %s: for %s: fallible expression must be handled explicitly", ctx.name, node.Name)
		}
		if rangeType.ValueType != (Type{Base: BaseTypeRange}) {
			return false, fmt.Errorf("function %s: for %s: expected Range, got %s", ctx.name, node.Name, rangeType.ValueType)
		}
		loopScope := newScope(scope)
		loopScope.define(node.Name, Type{Base: BaseTypeInt})
		_, err = c.checkBlock(loopScope, node.Body, ctx)
		if err != nil {
			return false, err
		}
		return false, nil
	case ast.MatchStmt:
		subjectType, err := c.checkExpr(scope, node.Subject, ctx)
		if err != nil {
			return false, fmt.Errorf("function %s: match: %w", ctx.name, err)
		}
		if !subjectType.Fallible {
			return false, fmt.Errorf("function %s: match requires fallible expression", ctx.name)
		}

		okScope := newScope(scope)
		okScope.define(node.OkName, subjectType.ValueType)
		okReturned, err := c.checkBlock(okScope, node.OkBody, ctx)
		if err != nil {
			return false, err
		}

		errScope := newScope(scope)
		errScope.define(node.ErrName, Type{Base: BaseTypeError})
		errReturned, err := c.checkBlock(errScope, node.ErrBody, ctx)
		if err != nil {
			return false, err
		}

		return okReturned && errReturned, nil
	case ast.IfStmt:
		conditionType, err := c.checkExpr(scope, node.Condition, ctx)
		if err != nil {
			return false, fmt.Errorf("function %s: if condition: %w", ctx.name, err)
		}
		if conditionType.Fallible {
			return false, fmt.Errorf("function %s: if condition: fallible expression must be handled explicitly", ctx.name)
		}
		if conditionType.ValueType != (Type{Base: BaseTypeBool}) {
			return false, fmt.Errorf("function %s: if condition must be Bool, got %s", ctx.name, conditionType.ValueType)
		}

		thenReturned, err := c.checkBlock(scope, node.ThenBody, ctx)
		if err != nil {
			return false, err
		}
		if node.ElseBody == nil {
			return false, nil
		}
		elseReturned, err := c.checkBlock(scope, *node.ElseBody, ctx)
		if err != nil {
			return false, err
		}
		return thenReturned && elseReturned, nil
	default:
		return false, fmt.Errorf("function %s: unsupported statement %T", ctx.name, stmt)
	}
}

func (c checker) checkExpr(scope *scope, expr ast.Expr, ctx functionContext) (ExprType, error) {
	switch node := expr.(type) {
	case ast.IntegerLiteral:
		return ExprType{ValueType: Type{Base: BaseTypeInt, Dimension: node.Dimension}}, nil
	case ast.FloatLiteral:
		return ExprType{ValueType: Type{Base: BaseTypeFloat, Dimension: node.Dimension}}, nil
	case ast.BoolLiteral:
		return ExprType{ValueType: Type{Base: BaseTypeBool}}, nil
	case ast.StringLiteralExpr:
		return ExprType{ValueType: Type{Base: BaseTypeString}}, nil
	case ast.ArrayLiteralExpr:
		valueType, err := c.checkArrayLiteralExpr(scope, node, ctx)
		if err != nil {
			return ExprType{}, err
		}
		return ExprType{ValueType: valueType}, nil
	case ast.IdentifierExpr:
		valueType, ok := scope.lookup(node.Name)
		if !ok {
			return ExprType{}, fmt.Errorf("undefined variable: %s", node.Name)
		}
		return ExprType{ValueType: valueType}, nil
	case ast.CallExpr:
		return c.checkCallExpr(scope, node, ctx)
	case ast.IndexExpr:
		targetType, err := c.checkExpr(scope, node.Target, ctx)
		if err != nil {
			return ExprType{}, err
		}
		if targetType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly")
		}
		if !targetType.ValueType.IsArray {
			return ExprType{}, fmt.Errorf("cannot index non-array value of type %s", targetType.ValueType)
		}
		indexType, err := c.checkExpr(scope, node.Index, ctx)
		if err != nil {
			return ExprType{}, err
		}
		if indexType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly")
		}
		if indexType.ValueType != (Type{Base: BaseTypeInt}) {
			return ExprType{}, fmt.Errorf("array index must be Int, got %s", indexType.ValueType)
		}
		return ExprType{ValueType: Type{Base: targetType.ValueType.Base, Dimension: targetType.ValueType.Dimension}}, nil
	case ast.ParenExpr:
		return c.checkExpr(scope, node.Inner, ctx)
	case ast.BinaryExpr:
		leftType, err := c.checkExpr(scope, node.Left, ctx)
		if err != nil {
			return ExprType{}, err
		}
		if leftType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly")
		}
		rightType, err := c.checkExpr(scope, node.Right, ctx)
		if err != nil {
			return ExprType{}, err
		}
		if rightType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly")
		}
		resultType, err := checkBinaryExpr(node.Operator, leftType.ValueType, rightType.ValueType)
		if err != nil {
			return ExprType{}, err
		}
		return ExprType{ValueType: resultType}, nil
	case ast.RangeExpr:
		startType, err := c.checkExpr(scope, node.Start, ctx)
		if err != nil {
			return ExprType{}, err
		}
		if startType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly")
		}
		if startType.ValueType != (Type{Base: BaseTypeInt}) {
			return ExprType{}, fmt.Errorf("range start must be Int, got %s", startType.ValueType)
		}
		endType, err := c.checkExpr(scope, node.End, ctx)
		if err != nil {
			return ExprType{}, err
		}
		if endType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly")
		}
		if endType.ValueType != (Type{Base: BaseTypeInt}) {
			return ExprType{}, fmt.Errorf("range end must be Int, got %s", endType.ValueType)
		}
		if node.Step != nil {
			stepType, err := c.checkExpr(scope, node.Step, ctx)
			if err != nil {
				return ExprType{}, err
			}
			if stepType.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly")
			}
			if stepType.ValueType != (Type{Base: BaseTypeInt}) {
				return ExprType{}, fmt.Errorf("range step must be Int, got %s", stepType.ValueType)
			}
			if integerLiteral, ok := node.Step.(ast.IntegerLiteral); ok && integerLiteral.Value == "0" && integerLiteral.Dimension.IsDimensionless() {
				return ExprType{}, fmt.Errorf("range step must be positive, got 0")
			}
		}
		return ExprType{ValueType: Type{Base: BaseTypeRange}}, nil
	case ast.PropagateExpr:
		innerType, err := c.checkExpr(scope, node.Inner, ctx)
		if err != nil {
			return ExprType{}, err
		}
		if !innerType.Fallible {
			return ExprType{}, fmt.Errorf("operator '?' requires fallible expression")
		}
		if !ctx.isFallible {
			return ExprType{}, fmt.Errorf("cannot use '?' in infallible function")
		}
		return ExprType{ValueType: innerType.ValueType}, nil
	case ast.UnwrapExpr:
		innerType, err := c.checkExpr(scope, node.Inner, ctx)
		if err != nil {
			return ExprType{}, err
		}
		if !innerType.Fallible {
			return ExprType{}, fmt.Errorf("operator '!' requires fallible expression")
		}
		return ExprType{ValueType: innerType.ValueType}, nil
	case ast.SwitchExpr:
		return c.checkSwitchExpr(scope, node, ctx)
	default:
		return ExprType{}, fmt.Errorf("unsupported expression %T", expr)
	}
}

func (c checker) checkSwitchExpr(scope *scope, expr ast.SwitchExpr, ctx functionContext) (ExprType, error) {
	subjectType, err := c.checkExpr(scope, expr.Subject, ctx)
	if err != nil {
		return ExprType{}, fmt.Errorf("switch subject: %w", err)
	}
	if subjectType.Fallible {
		return ExprType{}, fmt.Errorf("switch subject: fallible expression must be handled explicitly")
	}
	if !isSwitchSubjectTypeSupported(subjectType.ValueType) {
		return ExprType{}, fmt.Errorf("switch subject type %s is not supported", subjectType.ValueType)
	}

	var resultType Type
	hasResultType := false
	seenLabels := make(map[string]struct{}, len(expr.Cases))
	for index, switchCase := range expr.Cases {
		caseLabelType, err := c.checkSwitchCaseLabelType(scope, switchCase.Match, ctx)
		if err != nil {
			return ExprType{}, fmt.Errorf("switch case %d: %w", index+1, err)
		}
		if caseLabelType != subjectType.ValueType {
			return ExprType{}, fmt.Errorf("switch case %d: case type %s does not match subject type %s", index+1, caseLabelType, subjectType.ValueType)
		}
		labelKey, err := switchCaseKey(switchCase.Match)
		if err != nil {
			return ExprType{}, fmt.Errorf("switch case %d: %w", index+1, err)
		}
		if _, exists := seenLabels[labelKey]; exists {
			return ExprType{}, fmt.Errorf("switch case %d: duplicate case label", index+1)
		}
		seenLabels[labelKey] = struct{}{}

		caseValueType, err := c.checkExpr(scope, switchCase.Value, ctx)
		if err != nil {
			return ExprType{}, fmt.Errorf("switch case %d: %w", index+1, err)
		}
		if caseValueType.Fallible {
			return ExprType{}, fmt.Errorf("switch case %d: fallible expression must be handled explicitly", index+1)
		}
		if !hasResultType {
			resultType = caseValueType.ValueType
			hasResultType = true
		} else if caseValueType.ValueType != resultType {
			return ExprType{}, fmt.Errorf("switch case %d: result type %s does not match %s", index+1, caseValueType.ValueType, resultType)
		}
	}

	elseType, err := c.checkExpr(scope, expr.Else, ctx)
	if err != nil {
		return ExprType{}, fmt.Errorf("switch else: %w", err)
	}
	if elseType.Fallible {
		return ExprType{}, fmt.Errorf("switch else: fallible expression must be handled explicitly")
	}
	if !hasResultType {
		resultType = elseType.ValueType
		return ExprType{ValueType: resultType}, nil
	}
	if elseType.ValueType != resultType {
		return ExprType{}, fmt.Errorf("switch else: result type %s does not match %s", elseType.ValueType, resultType)
	}
	return ExprType{ValueType: resultType}, nil
}

func (c checker) checkSwitchCaseLabelType(scope *scope, expr ast.Expr, ctx functionContext) (Type, error) {
	exprType, err := c.checkExpr(scope, expr, ctx)
	if err != nil {
		return Type{}, err
	}
	if exprType.Fallible {
		return Type{}, fmt.Errorf("case label must not be fallible")
	}
	switch expr.(type) {
	case ast.IntegerLiteral, ast.FloatLiteral, ast.BoolLiteral, ast.StringLiteralExpr:
		return exprType.ValueType, nil
	default:
		return Type{}, fmt.Errorf("case label must be int, float, bool, or string literal")
	}
}

func isSwitchSubjectTypeSupported(valueType Type) bool {
	if valueType.IsArray {
		return false
	}
	switch valueType.Base {
	case BaseTypeInt, BaseTypeFloat:
		return valueType.Dimension.IsDimensionless()
	case BaseTypeBool, BaseTypeString:
		return true
	default:
		return false
	}
}

func switchCaseKey(expr ast.Expr) (string, error) {
	switch node := expr.(type) {
	case ast.IntegerLiteral:
		return "int:" + node.Value, nil
	case ast.FloatLiteral:
		return "float:" + node.Value, nil
	case ast.BoolLiteral:
		if node.Value {
			return "bool:true", nil
		}
		return "bool:false", nil
	case ast.StringLiteralExpr:
		return "string:" + node.Value, nil
	default:
		return "", fmt.Errorf("case label must be int, float, bool, or string literal")
	}
}

func (c checker) checkCallExpr(scope *scope, expr ast.CallExpr, ctx functionContext) (ExprType, error) {
	if expr.Callee == "error" {
		if len(expr.Arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'error' expects 1 arguments, got %d", len(expr.Arguments))
		}
		if _, ok := expr.Arguments[0].(ast.StringLiteralExpr); !ok {
			return ExprType{}, fmt.Errorf("error() requires a string literal")
		}
		return ExprType{ValueType: Type{Base: BaseTypeError}}, nil
	}
	if isReservedBuiltinFunctionName(expr.Callee) {
		return c.checkBuiltinCallExpr(scope, expr, ctx)
	}

	signature, ok := c.functions[expr.Callee]
	if !ok {
		return ExprType{}, fmt.Errorf("undefined function: %s", expr.Callee)
	}
	if len(expr.Arguments) != len(signature.parameters) {
		return ExprType{}, fmt.Errorf("function '%s' expects %d arguments, got %d", expr.Callee, len(signature.parameters), len(expr.Arguments))
	}
	for i, argumentExpr := range expr.Arguments {
		argumentType, err := c.checkExpr(scope, argumentExpr, ctx)
		if err != nil {
			return ExprType{}, err
		}
		if argumentType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly")
		}
		if !isAssignable(argumentType.ValueType, signature.parameters[i]) {
			return ExprType{}, fmt.Errorf("function '%s' argument %d expects %s, got %s", expr.Callee, i+1, signature.parameters[i], argumentType.ValueType)
		}
	}
	return ExprType{ValueType: signature.returnType, Fallible: signature.isFallible}, nil
}

func (c checker) checkBuiltinCallExpr(scope *scope, expr ast.CallExpr, ctx functionContext) (ExprType, error) {
	if expr.Callee == "PlotLine" || expr.Callee == "PlotScatter" {
		return c.checkPlotBuiltinCallExpr(scope, expr, ctx)
	}

	if len(expr.Arguments) != 1 {
		return ExprType{}, fmt.Errorf("function '%s' expects 1 arguments, got %d", expr.Callee, len(expr.Arguments))
	}

	argumentType, err := c.checkExpr(scope, expr.Arguments[0], ctx)
	if err != nil {
		return ExprType{}, err
	}
	if argumentType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly")
	}

	switch expr.Callee {
	case "Len":
		if !argumentType.ValueType.IsArray {
			return ExprType{}, fmt.Errorf("function 'Len' argument 1 expects Int[], Float[], or Bool[], got %s", argumentType.ValueType)
		}
		switch argumentType.ValueType.Base {
		case BaseTypeInt, BaseTypeFloat, BaseTypeBool:
			return ExprType{ValueType: Type{Base: BaseTypeInt}}, nil
		default:
			return ExprType{}, fmt.Errorf("function 'Len' argument 1 expects Int[], Float[], or Bool[], got %s", argumentType.ValueType)
		}
	case "Abs":
		if isNumericScalar(argumentType.ValueType) {
			return ExprType{ValueType: argumentType.ValueType}, nil
		}
		return ExprType{}, fmt.Errorf("function 'Abs' argument 1 expects Int or Float, got %s", argumentType.ValueType)
	case "Sqrt":
		if !isNumericScalar(argumentType.ValueType) {
			return ExprType{}, fmt.Errorf("function 'Sqrt' argument 1 expects Int or Float, got %s", argumentType.ValueType)
		}
		if !argumentType.ValueType.Dimension.CanSqrt() {
			return ExprType{}, fmt.Errorf("Sqrt requires even dimension exponents")
		}
		return ExprType{ValueType: Type{Base: BaseTypeFloat, Dimension: argumentType.ValueType.Dimension.Sqrt()}}, nil
	case "Sin", "Cos":
		if !isNumericScalar(argumentType.ValueType) {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects Int or Float, got %s", expr.Callee, argumentType.ValueType)
		}
		if !argumentType.ValueType.Dimension.IsDimensionless() {
			return ExprType{}, fmt.Errorf("%s requires dimensionless input", expr.Callee)
		}
		return ExprType{ValueType: Type{Base: BaseTypeFloat}}, nil
	default:
		return ExprType{}, fmt.Errorf("unsupported built-in function: %s", expr.Callee)
	}
}

func (c checker) checkPlotBuiltinCallExpr(scope *scope, expr ast.CallExpr, ctx functionContext) (ExprType, error) {
	if len(expr.Arguments) != 3 {
		return ExprType{}, fmt.Errorf("function '%s' expects 3 arguments, got %d", expr.Callee, len(expr.Arguments))
	}

	xType, err := c.checkExpr(scope, expr.Arguments[0], ctx)
	if err != nil {
		return ExprType{}, err
	}
	if xType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly")
	}
	if err := requirePlotArrayType(expr.Callee, 1, xType.ValueType); err != nil {
		return ExprType{}, err
	}

	yType, err := c.checkExpr(scope, expr.Arguments[1], ctx)
	if err != nil {
		return ExprType{}, err
	}
	if yType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly")
	}
	if err := requirePlotArrayType(expr.Callee, 2, yType.ValueType); err != nil {
		return ExprType{}, err
	}

	pathType, err := c.checkExpr(scope, expr.Arguments[2], ctx)
	if err != nil {
		return ExprType{}, err
	}
	if pathType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly")
	}
	if pathType.ValueType != (Type{Base: BaseTypeString}) {
		return ExprType{}, fmt.Errorf("function '%s' argument 3 expects String, got %s", expr.Callee, pathType.ValueType)
	}

	return ExprType{ValueType: Type{Base: BaseTypeInt}}, nil
}

func requirePlotArrayType(functionName string, index int, valueType Type) error {
	if !valueType.IsArray {
		return fmt.Errorf("function '%s' argument %d expects Int[] or Float[], got %s", functionName, index, valueType)
	}
	if !valueType.Dimension.IsDimensionless() {
		return fmt.Errorf("%s does not accept dimensioned arrays", functionName)
	}
	switch valueType.Base {
	case BaseTypeInt, BaseTypeFloat:
		return nil
	default:
		return fmt.Errorf("function '%s' argument %d expects Int[] or Float[], got %s", functionName, index, valueType)
	}
}

func (c checker) checkArrayLiteralExpr(scope *scope, expr ast.ArrayLiteralExpr, ctx functionContext) (Type, error) {
	if len(expr.Elements) == 0 {
		return Type{}, fmt.Errorf("empty array literals are not supported")
	}

	firstType, err := c.checkExpr(scope, expr.Elements[0], ctx)
	if err != nil {
		return Type{}, err
	}
	if firstType.Fallible {
		return Type{}, fmt.Errorf("fallible expression must be handled explicitly")
	}
	if firstType.ValueType.IsArray {
		return Type{}, fmt.Errorf("nested arrays are not supported")
	}

	for _, element := range expr.Elements[1:] {
		elementType, err := c.checkExpr(scope, element, ctx)
		if err != nil {
			return Type{}, err
		}
		if elementType.Fallible {
			return Type{}, fmt.Errorf("fallible expression must be handled explicitly")
		}
		if elementType.ValueType != firstType.ValueType {
			return Type{}, fmt.Errorf("array literal elements must all have the same type; found %s and %s", firstType.ValueType, elementType.ValueType)
		}
	}

	return Type{Base: firstType.ValueType.Base, Dimension: firstType.ValueType.Dimension, IsArray: true}, nil
}

func resolveType(typeRef ast.TypeRef) (Type, error) {
	baseType, err := resolveBaseType(typeRef.Name)
	if err != nil {
		return Type{}, err
	}
	if typeRef.HasUnit && !isNumericBaseType(baseType) {
		return Type{}, fmt.Errorf("invalid dimension-qualified type syntax: %s<%s>", typeRef.Name, typeRef.Dimension.String())
	}
	if typeRef.IsArray {
		if baseType == BaseTypeError || baseType == BaseTypeRange {
			return Type{}, fmt.Errorf("unknown type: %s[]", typeRef.Name)
		}
		return Type{Base: baseType, Dimension: typeRef.Dimension, IsArray: true}, nil
	}
	return Type{Base: baseType, Dimension: typeRef.Dimension}, nil
}

func resolveBaseType(name string) (BaseType, error) {
	switch BaseType(name) {
	case BaseTypeInt, BaseTypeFloat, BaseTypeBool, BaseTypeString, BaseTypeError:
		return BaseType(name), nil
	default:
		return "", fmt.Errorf("unknown type: %s", name)
	}
}

func checkBinaryExpr(operator string, leftType Type, rightType Type) (Type, error) {
	if leftType.Base == BaseTypeRange || rightType.Base == BaseTypeRange || leftType.Base == BaseTypeString || rightType.Base == BaseTypeString || leftType.Base == BaseTypeError || rightType.Base == BaseTypeError {
		return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}
	if leftType.IsArray || rightType.IsArray {
		return checkArrayBinaryExpr(operator, leftType, rightType)
	}
	if leftType.Base == BaseTypeBool || rightType.Base == BaseTypeBool {
		return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}

	resultBase := BaseTypeInt
	if leftType.Base == BaseTypeFloat || rightType.Base == BaseTypeFloat {
		resultBase = BaseTypeFloat
	}

	switch operator {
	case "+", "-":
		if leftType.Dimension != rightType.Dimension {
			return Type{}, fmt.Errorf("cannot %s %s and %s", operatorName(operator), formatDimension(leftType.Dimension), formatDimension(rightType.Dimension))
		}
		return Type{Base: resultBase, Dimension: leftType.Dimension}, nil
	case "*":
		return Type{Base: resultBase, Dimension: leftType.Dimension.Multiply(rightType.Dimension)}, nil
	case "/":
		resultDimension := leftType.Dimension.Divide(rightType.Dimension)
		if resultBase == BaseTypeInt && !resultDimension.IsDimensionless() {
			resultBase = BaseTypeFloat
		}
		return Type{Base: resultBase, Dimension: resultDimension}, nil
	default:
		return Type{}, fmt.Errorf("unsupported operator %q", operator)
	}
}

func checkArrayBinaryExpr(operator string, leftType Type, rightType Type) (Type, error) {
	if !leftType.IsArray || !rightType.IsArray {
		return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}
	if leftType.Base == BaseTypeBool || leftType.Base == BaseTypeString || leftType.Base == BaseTypeError || rightType.Base == BaseTypeBool || rightType.Base == BaseTypeString || rightType.Base == BaseTypeError {
		return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}
	result, err := checkBinaryExpr(operator, Type{Base: leftType.Base, Dimension: leftType.Dimension}, Type{Base: rightType.Base, Dimension: rightType.Dimension})
	if err != nil {
		return Type{}, err
	}
	result.IsArray = true
	return result, nil
}

func isNumericBaseType(baseType BaseType) bool {
	return baseType == BaseTypeInt || baseType == BaseTypeFloat
}

func isNumericScalar(valueType Type) bool {
	return !valueType.IsArray && isNumericBaseType(valueType.Base)
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

func isAssignable(actual Type, expected Type) bool {
	if actual == expected {
		return true
	}
	if actual.IsArray != expected.IsArray || actual.Dimension != expected.Dimension {
		return false
	}
	return actual.Base == BaseTypeInt && expected.Base == BaseTypeFloat
}
