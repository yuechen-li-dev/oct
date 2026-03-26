package typecheck

import (
	"fmt"
	"strings"

	"oct/internal/ast"
	"oct/internal/builtin"
	"oct/internal/dimension"
	"oct/internal/project"
)

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
	Name      string
	Dimension dimension.Dimension
	IsArray   bool
	IsVector  bool
	IsMatrix  bool
}

func (t Type) String() string {
	base := t.Name
	if base == "" {
		base = string(t.Base)
	}
	if isNumericBaseType(t.Base) && !t.Dimension.IsDimensionless() {
		base += "<" + t.Dimension.String() + ">"
	}
	if t.IsArray {
		return base + "[]"
	}
	if t.IsVector {
		return "Vector<" + base + ">"
	}
	if t.IsMatrix {
		return "Matrix<" + base + ">"
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
	checker := checker{
		functions: make(map[string]functionSignature),
		records:   make(map[string]recordInfo),
		enums:     make(map[string]enumInfo),
		typeNames: make(map[string]struct{}),
	}
	return checker.checkFile(file)
}

func CheckProgram(program project.Program) error {
	packageCheckers := make(map[string]checker, len(program.Packages))
	for name, pkg := range program.Packages {
		file := ast.File{Package: name, Imports: pkg.Imports, Records: pkg.Records, Enums: pkg.Enums, Functions: pkg.Functions}
		chk := checker{
			functions: make(map[string]functionSignature),
			records:   make(map[string]recordInfo),
			enums:     make(map[string]enumInfo),
			typeNames: make(map[string]struct{}),
		}
		if err := chk.registerPackageDeclarations(file); err != nil {
			return err
		}
		packageCheckers[name] = chk
	}

	for name, chk := range packageCheckers {
		imports := make(map[string]packageSymbols)
		for _, imp := range program.Packages[name].Imports {
			imported, ok := packageCheckers[imp]
			if !ok {
				return fmt.Errorf("unknown package '%s'", imp)
			}
			imports[imp] = packageSymbols{
				functions: imported.functions,
				records:   imported.records,
				enums:     imported.enums,
			}
		}
		chk.importedPackages = imports
		packageCheckers[name] = chk
	}

	for name, pkg := range program.Packages {
		file := ast.File{Package: name, Imports: pkg.Imports, Records: pkg.Records, Enums: pkg.Enums, Functions: pkg.Functions}
		chk := packageCheckers[name]
		if err := chk.registerFunctionSignatures(file); err != nil {
			return err
		}
		packageCheckers[name] = chk
	}

	for name, pkg := range program.Packages {
		file := ast.File{Package: name, Imports: pkg.Imports, Records: pkg.Records, Enums: pkg.Enums, Functions: pkg.Functions}
		chk := packageCheckers[name]
		if err := chk.checkPackageFunctions(file); err != nil {
			return err
		}
	}
	return nil
}

type checker struct {
	functions        map[string]functionSignature
	records          map[string]recordInfo
	enums            map[string]enumInfo
	typeNames        map[string]struct{}
	importedPackages map[string]packageSymbols
}

type packageSymbols struct {
	functions map[string]functionSignature
	records   map[string]recordInfo
	enums     map[string]enumInfo
}

type recordInfo struct {
	fields     map[string]Type
	fieldOrder []string
}

type enumInfo struct {
	variants map[string]struct{}
}

type scope struct {
	parent *scope
	values map[string]binding
}

type binding struct {
	valueType Type
	mutable   bool
}

func newScope(parent *scope) *scope {
	return &scope{parent: parent, values: make(map[string]binding)}
}

func (s *scope) define(name string, valueType Type, mutable bool) {
	s.values[name] = binding{valueType: valueType, mutable: mutable}
}

func (s *scope) lookup(name string) (binding, bool) {
	for current := s; current != nil; current = current.parent {
		value, ok := current.values[name]
		if ok {
			return value, true
		}
	}
	return binding{}, false
}

func (c checker) checkFile(file ast.File) error {
	if err := c.registerPackageDeclarations(file); err != nil {
		return err
	}
	if err := c.registerFunctionSignatures(file); err != nil {
		return err
	}
	return c.checkPackageFunctions(file)
}

func (c checker) registerPackageDeclarations(file ast.File) error {
	for _, builtinTypeName := range []string{string(BaseTypeInt), string(BaseTypeFloat), string(BaseTypeBool), string(BaseTypeString), string(BaseTypeError)} {
		c.typeNames[builtinTypeName] = struct{}{}
	}

	for _, record := range file.Records {
		if err := c.registerRecord(record); err != nil {
			return err
		}
	}
	for _, enumDecl := range file.Enums {
		if err := c.registerEnum(enumDecl); err != nil {
			return err
		}
	}

	for _, function := range file.Functions {
		if builtin.IsName(function.Name) {
			return fmt.Errorf("function %s: cannot redeclare built-in function", function.Name)
		}
		if _, exists := c.functions[function.Name]; exists {
			return fmt.Errorf("duplicate function: %s", function.Name)
		}
		c.functions[function.Name] = functionSignature{}
	}

	return nil
}

func (c checker) registerFunctionSignatures(file ast.File) error {
	for _, function := range file.Functions {
		signature, err := c.resolveFunctionSignature(function)
		if err != nil {
			return fmt.Errorf("function %s: %w", function.Name, err)
		}
		c.functions[function.Name] = signature
	}
	return nil
}

func (c checker) checkPackageFunctions(file ast.File) error {
	for _, function := range file.Functions {
		if err := c.checkFunction(function); err != nil {
			return err
		}
	}
	return nil
}

func (c checker) registerRecord(record ast.RecordDecl) error {
	if _, exists := c.typeNames[record.Name]; exists {
		return fmt.Errorf("duplicate type: %s", record.Name)
	}
	c.typeNames[record.Name] = struct{}{}

	fields := make(map[string]Type, len(record.Fields))
	fieldOrder := make([]string, 0, len(record.Fields))
	for _, field := range record.Fields {
		if _, exists := fields[field.Name]; exists {
			return fmt.Errorf("record '%s' field '%s' specified more than once", record.Name, field.Name)
		}
		fieldType, err := c.resolveType(field.Type)
		if err != nil {
			return fmt.Errorf("record '%s' field '%s': %w", record.Name, field.Name, err)
		}
		fields[field.Name] = fieldType
		fieldOrder = append(fieldOrder, field.Name)
	}
	c.records[record.Name] = recordInfo{fields: fields, fieldOrder: fieldOrder}
	return nil
}

func (c checker) registerEnum(enumDecl ast.EnumDecl) error {
	if _, exists := c.typeNames[enumDecl.Name]; exists {
		return fmt.Errorf("duplicate type: %s", enumDecl.Name)
	}
	if len(enumDecl.Variants) == 0 {
		return fmt.Errorf("enum '%s' must declare at least one variant", enumDecl.Name)
	}
	c.typeNames[enumDecl.Name] = struct{}{}

	variants := make(map[string]struct{}, len(enumDecl.Variants))
	for _, variant := range enumDecl.Variants {
		if _, exists := variants[variant]; exists {
			return fmt.Errorf("enum '%s' variant '%s' specified more than once", enumDecl.Name, variant)
		}
		variants[variant] = struct{}{}
	}
	c.enums[enumDecl.Name] = enumInfo{variants: variants}
	return nil
}

func (c checker) resolveFunctionSignature(function ast.FunctionDecl) (functionSignature, error) {
	returnType, err := c.resolveType(function.ReturnType)
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
		parameterType, err := c.resolveType(parameter.Type)
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
		functionScope.define(parameter.Name, signature.parameters[i], false)
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
		scope.define(node.Name, valueType.ValueType, false)
		return false, nil
	case ast.VarStmt:
		valueType, err := c.checkExpr(scope, node.Value, ctx)
		if err != nil {
			return false, fmt.Errorf("function %s: var %s: %w", ctx.name, node.Name, err)
		}
		if valueType.Fallible {
			return false, fmt.Errorf("function %s: var %s: fallible expression must be handled explicitly", ctx.name, node.Name)
		}
		scope.define(node.Name, valueType.ValueType, true)
		return false, nil
	case ast.AssignStmt:
		target, ok := scope.lookup(node.Name)
		if !ok {
			return false, fmt.Errorf("function %s: unknown binding '%s'", ctx.name, node.Name)
		}
		if !target.mutable {
			return false, fmt.Errorf("function %s: cannot assign to immutable binding '%s'", ctx.name, node.Name)
		}
		valueType, err := c.checkExpr(scope, node.Value, ctx)
		if err != nil {
			return false, fmt.Errorf("function %s: assignment to %s: %w", ctx.name, node.Name, err)
		}
		if valueType.Fallible {
			return false, fmt.Errorf("function %s: assignment to %s: fallible expression must be handled explicitly", ctx.name, node.Name)
		}
		if !isAssignable(valueType.ValueType, target.valueType) {
			return false, fmt.Errorf("function %s: assignment to %s: expected %s, got %s", ctx.name, node.Name, target.valueType, valueType.ValueType)
		}
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
	case ast.ExprStmt:
		if _, ok := node.Value.(ast.CallExpr); !ok {
			return false, fmt.Errorf("function %s: expression statements must be call expressions", ctx.name)
		}
		valueType, err := c.checkExpr(scope, node.Value, ctx)
		if err != nil {
			return false, fmt.Errorf("function %s: %w", ctx.name, err)
		}
		if valueType.Fallible {
			return false, fmt.Errorf("function %s: expression statement must not be fallible; handle it with '?', '!', or match", ctx.name)
		}
		return false, nil
	case ast.ForStmt:
		rangeType, err := c.checkExpr(scope, node.Range, ctx)
		if err != nil {
			return false, fmt.Errorf("function %s: for %s: %w", ctx.name, node.Name, err)
		}
		if rangeType.Fallible {
			return false, fmt.Errorf("function %s: for %s: fallible expression must be handled explicitly", ctx.name, node.Name)
		}
		if rangeType.ValueType != (Type{Base: BaseTypeRange}) {
			return false, fmt.Errorf("function %s: for %s: range expression expects Range, got %s", ctx.name, node.Name, rangeType.ValueType)
		}
		loopScope := newScope(scope)
		loopScope.define(node.Name, Type{Base: BaseTypeInt}, false)
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
		okScope.define(node.OkName, subjectType.ValueType, false)
		okReturned, err := c.checkBlock(okScope, node.OkBody, ctx)
		if err != nil {
			return false, err
		}

		errScope := newScope(scope)
		errScope.define(node.ErrName, Type{Base: BaseTypeError}, false)
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
	case ast.WhileStmt:
		conditionType, err := c.checkExpr(scope, node.Condition, ctx)
		if err != nil {
			return false, fmt.Errorf("function %s: while condition: %w", ctx.name, err)
		}
		if conditionType.Fallible {
			return false, fmt.Errorf("function %s: while condition: fallible expression must be handled explicitly", ctx.name)
		}
		if conditionType.ValueType != (Type{Base: BaseTypeBool}) {
			return false, fmt.Errorf("function %s: while condition must be Bool, got %s", ctx.name, conditionType.ValueType)
		}

		_, err = c.checkBlock(scope, node.Body, ctx)
		if err != nil {
			return false, err
		}
		return false, nil
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
	case ast.VectorLiteralExpr:
		valueType, err := c.checkVectorLiteralExpr(scope, node, ctx)
		if err != nil {
			return ExprType{}, err
		}
		return ExprType{ValueType: valueType}, nil
	case ast.MatrixLiteralExpr:
		valueType, err := c.checkMatrixLiteralExpr(scope, node, ctx)
		if err != nil {
			return ExprType{}, err
		}
		return ExprType{ValueType: valueType}, nil
	case ast.IdentifierExpr:
		valueBinding, ok := scope.lookup(node.Name)
		if !ok {
			return ExprType{}, fmt.Errorf("undefined variable: %s", node.Name)
		}
		return ExprType{ValueType: valueBinding.valueType}, nil
	case ast.CallExpr:
		return c.checkCallExpr(scope, node, ctx)
	case ast.RecordLiteralExpr:
		return c.checkRecordLiteralExpr(scope, node, ctx)
	case ast.EnumValueExpr:
		enumDecl, ok := c.lookupEnum(node.EnumName)
		if !ok {
			return ExprType{}, fmt.Errorf("unknown enum type: %s", node.EnumName)
		}
		if _, ok := enumDecl.variants[node.Variant]; !ok {
			return ExprType{}, fmt.Errorf("enum '%s' has no variant '%s'", node.EnumName, node.Variant)
		}
		return ExprType{ValueType: Type{Name: node.EnumName}}, nil
	case ast.IndexExpr:
		targetType, err := c.checkExpr(scope, node.Target, ctx)
		if err != nil {
			return ExprType{}, err
		}
		if targetType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly")
		}
		for _, idxExpr := range node.Indices {
			indexType, err := c.checkExpr(scope, idxExpr, ctx)
			if err != nil {
				return ExprType{}, err
			}
			if indexType.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly")
			}
			if indexType.ValueType != (Type{Base: BaseTypeInt}) {
				return ExprType{}, fmt.Errorf("index must be Int, got %s", indexType.ValueType)
			}
		}
		switch {
		case targetType.ValueType.IsArray:
			if len(node.Indices) != 1 {
				return ExprType{}, fmt.Errorf("array indexing requires exactly 1 index, got %d", len(node.Indices))
			}
			return ExprType{ValueType: Type{Base: targetType.ValueType.Base, Dimension: targetType.ValueType.Dimension}}, nil
		case targetType.ValueType.IsVector:
			if len(node.Indices) != 1 {
				return ExprType{}, fmt.Errorf("vector indexing requires exactly 1 index, got %d", len(node.Indices))
			}
			return ExprType{ValueType: Type{Base: targetType.ValueType.Base, Dimension: targetType.ValueType.Dimension}}, nil
		case targetType.ValueType.IsMatrix:
			if len(node.Indices) != 2 {
				return ExprType{}, fmt.Errorf("matrix indexing requires exactly 2 indices, got %d", len(node.Indices))
			}
			return ExprType{ValueType: Type{Base: targetType.ValueType.Base, Dimension: targetType.ValueType.Dimension}}, nil
		default:
			return ExprType{}, fmt.Errorf("cannot index non-indexable value of type %s", targetType.ValueType)
		}
	case ast.FieldAccessExpr:
		if identifier, ok := node.Target.(ast.IdentifierExpr); ok {
			if enumDecl, enumExists := c.lookupEnum(identifier.Name); enumExists {
				if _, variantExists := enumDecl.variants[node.Field]; !variantExists {
					return ExprType{}, fmt.Errorf("enum '%s' has no variant '%s'", identifier.Name, node.Field)
				}
				return ExprType{ValueType: Type{Name: identifier.Name}}, nil
			}
		}
		if enumName, ok := c.flattenEnumTypeName(node.Target); ok {
			enumDecl, enumExists := c.lookupEnum(enumName)
			if enumExists {
				if _, variantExists := enumDecl.variants[node.Field]; !variantExists {
					return ExprType{}, fmt.Errorf("enum '%s' has no variant '%s'", enumName, node.Field)
				}
				return ExprType{ValueType: Type{Name: enumName}}, nil
			}
			if c.isKnownTypeName(enumName) {
				return ExprType{}, fmt.Errorf("type '%s' is not an enum", enumName)
			}
			return ExprType{}, fmt.Errorf("unknown enum type: %s", enumName)
		}
		targetType, err := c.checkExpr(scope, node.Target, ctx)
		if err != nil {
			return ExprType{}, err
		}
		if targetType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly")
		}
		recordDecl, ok := c.lookupRecord(targetType.ValueType.Name)
		if !ok || targetType.ValueType.IsArray || targetType.ValueType.Base != "" {
			return ExprType{}, fmt.Errorf("field access requires record type, got %s", targetType.ValueType)
		}
		fieldType, ok := recordDecl.fields[node.Field]
		if !ok {
			return ExprType{}, fmt.Errorf("type '%s' has no field '%s'", targetType.ValueType.Name, node.Field)
		}
		return ExprType{ValueType: fieldType}, nil
	case ast.ParenExpr:
		return c.checkExpr(scope, node.Inner, ctx)
	case ast.BinaryExpr:
		if isComparisonOperator(node.Operator) {
			if leftBinary, ok := node.Left.(ast.BinaryExpr); ok && isComparisonOperator(leftBinary.Operator) {
				return ExprType{}, fmt.Errorf("chained comparisons are not supported")
			}
			if rightBinary, ok := node.Right.(ast.BinaryExpr); ok && isComparisonOperator(rightBinary.Operator) {
				return ExprType{}, fmt.Errorf("chained comparisons are not supported")
			}
		}
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
		resultType, err := c.checkBinaryExpr(node.Operator, leftType.ValueType, rightType.ValueType)
		if err != nil {
			return ExprType{}, err
		}
		return ExprType{ValueType: resultType}, nil
	case ast.UnaryExpr:
		operandType, err := c.checkExpr(scope, node.Operand, ctx)
		if err != nil {
			return ExprType{}, err
		}
		if operandType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly")
		}
		switch node.Operator {
		case "not":
			if operandType.ValueType != (Type{Base: BaseTypeBool}) {
				return ExprType{}, fmt.Errorf("operator 'not' requires Bool operand")
			}
			return ExprType{ValueType: Type{Base: BaseTypeBool}}, nil
		default:
			return ExprType{}, fmt.Errorf("unsupported unary operator %q", node.Operator)
		}
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
	case ast.IfExpr:
		return c.checkIfExpr(scope, node, ctx)
	default:
		return ExprType{}, fmt.Errorf("unsupported expression %T", expr)
	}
}

func (c checker) checkIfExpr(scope *scope, expr ast.IfExpr, ctx functionContext) (ExprType, error) {
	conditionType, err := c.checkExpr(scope, expr.Condition, ctx)
	if err != nil {
		return ExprType{}, fmt.Errorf("if expression condition: %w", err)
	}
	if conditionType.Fallible {
		return ExprType{}, fmt.Errorf("if expression condition: fallible expression must be handled explicitly")
	}
	if conditionType.ValueType != (Type{Base: BaseTypeBool}) {
		return ExprType{}, fmt.Errorf("if expression condition must be Bool, got %s", conditionType.ValueType)
	}

	thenType, err := c.checkExpr(scope, expr.ThenExpr, ctx)
	if err != nil {
		return ExprType{}, fmt.Errorf("if expression then branch: %w", err)
	}
	elseType, err := c.checkExpr(scope, expr.ElseExpr, ctx)
	if err != nil {
		return ExprType{}, fmt.Errorf("if expression else branch: %w", err)
	}

	if thenType.ValueType != elseType.ValueType || thenType.Fallible != elseType.Fallible {
		return ExprType{}, fmt.Errorf(
			"if expression branches must have matching types, got %s%s and %s%s",
			thenType.ValueType,
			fallibilitySuffix(thenType.Fallible),
			elseType.ValueType,
			fallibilitySuffix(elseType.Fallible),
		)
	}
	return thenType, nil
}

func fallibilitySuffix(fallible bool) string {
	if fallible {
		return "!"
	}
	return ""
}

func (c checker) checkSwitchExpr(scope *scope, expr ast.SwitchExpr, ctx functionContext) (ExprType, error) {
	subjectType, err := c.checkExpr(scope, expr.Subject, ctx)
	if err != nil {
		return ExprType{}, fmt.Errorf("switch subject: %w", err)
	}
	if subjectType.Fallible {
		return ExprType{}, fmt.Errorf("switch subject: fallible expression must be handled explicitly")
	}
	if !isSwitchSubjectTypeSupported(c, subjectType.ValueType) {
		return ExprType{}, fmt.Errorf("switch subject type %s is not supported", subjectType.ValueType)
	}

	subjectEnum, isEnumSubject := c.lookupEnum(subjectType.ValueType.Name)
	var resultType Type
	hasResultType := false
	seenLabels := make(map[string]struct{}, len(expr.Cases))
	seenEnumVariants := make(map[string]struct{}, len(expr.Cases))
	enumCaseCount := 0
	nonEnumCaseCount := 0
	for index, switchCase := range expr.Cases {
		_, isEnumCaseLabel := switchCase.Match.(ast.FieldAccessExpr)
		if isEnumSubject && ((isEnumCaseLabel && nonEnumCaseCount > 0) || (!isEnumCaseLabel && enumCaseCount > 0)) {
			return ExprType{}, fmt.Errorf("mixing enum and non-enum case labels is not allowed")
		}

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
			if isEnumSubject {
				return ExprType{}, fmt.Errorf("switch case %d: duplicate case '%s'", index+1, labelKey)
			}
			return ExprType{}, fmt.Errorf("switch case %d: duplicate case label", index+1)
		}
		seenLabels[labelKey] = struct{}{}
		if enumCaseLabel, ok := switchCase.Match.(ast.FieldAccessExpr); ok {
			enumCaseCount++
			if isEnumSubject {
				if variant, ok := extractEnumVariantName(enumCaseLabel, subjectType.ValueType.Name); ok {
					seenEnumVariants[variant] = struct{}{}
				}
			}
		} else {
			nonEnumCaseCount++
		}

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

	hasElse := expr.Else != nil
	if hasElse {
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

	if isEnumSubject {
		if !hasAllEnumVariantsCovered(subjectEnum, seenEnumVariants) {
			return ExprType{}, fmt.Errorf("non-exhaustive switch over enum '%s'; missing cases and no else", subjectType.ValueType.Name)
		}
	} else {
		return ExprType{}, fmt.Errorf("switch requires else arm")
	}

	if !hasResultType {
		return ExprType{}, fmt.Errorf("switch must include at least one case or else arm")
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
	case ast.FieldAccessExpr:
		if _, ok := c.lookupEnum(exprType.ValueType.Name); !ok {
			return Type{}, fmt.Errorf("case labels must match switch subject type")
		}
		return exprType.ValueType, nil
	default:
		return Type{}, fmt.Errorf("case label must be int, float, bool, string literal, or qualified enum variant")
	}
}

func isSwitchSubjectTypeSupported(c checker, valueType Type) bool {
	if valueType.IsArray || valueType.IsVector || valueType.IsMatrix {
		return false
	}
	switch valueType.Base {
	case BaseTypeInt, BaseTypeFloat:
		return valueType.Dimension.IsDimensionless()
	case BaseTypeBool, BaseTypeString:
		return true
	default:
		return isEnumType(c, valueType)
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
	case ast.FieldAccessExpr:
		return enumCaseLabel(node)
	default:
		return "", fmt.Errorf("case label must be int, float, bool, string literal, or qualified enum variant")
	}
}

func enumCaseLabel(expr ast.FieldAccessExpr) (string, error) {
	enumTypeName, variant, ok := flattenEnumValueExpr(expr)
	if !ok {
		return "", fmt.Errorf("switch case enum label must be qualified as EnumName.Variant")
	}
	return enumTypeName + "." + variant, nil
}

func flattenEnumValueExpr(expr ast.FieldAccessExpr) (string, string, bool) {
	switch target := expr.Target.(type) {
	case ast.IdentifierExpr:
		return target.Name, expr.Field, true
	case ast.FieldAccessExpr:
		enumName, ok := flattenEnumTypeNameFromFieldAccess(target)
		if !ok {
			return "", "", false
		}
		return enumName, expr.Field, true
	default:
		return "", "", false
	}
}

func flattenEnumTypeNameFromFieldAccess(expr ast.FieldAccessExpr) (string, bool) {
	parts := make([]string, 0, 2)
	current := ast.Expr(expr)
	for {
		fieldExpr, ok := current.(ast.FieldAccessExpr)
		if !ok {
			break
		}
		parts = append([]string{fieldExpr.Field}, parts...)
		current = fieldExpr.Target
	}
	identifier, ok := current.(ast.IdentifierExpr)
	if !ok {
		return "", false
	}
	parts = append([]string{identifier.Name}, parts...)
	if len(parts) != 2 {
		return "", false
	}
	return parts[0] + "." + parts[1], true
}

func extractEnumVariantName(expr ast.FieldAccessExpr, expectedEnumName string) (string, bool) {
	enumName, variant, ok := flattenEnumValueExpr(expr)
	if !ok || enumName != expectedEnumName {
		return "", false
	}
	return variant, true
}

func hasAllEnumVariantsCovered(info enumInfo, seen map[string]struct{}) bool {
	if len(info.variants) != len(seen) {
		return false
	}
	for variant := range info.variants {
		if _, ok := seen[variant]; !ok {
			return false
		}
	}
	return true
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
	if builtin.IsName(expr.Callee) {
		return c.checkBuiltinCallExpr(scope, expr, ctx)
	}

	lookupName := expr.Callee
	lookupTable := c.functions
	calleePackage := ""
	if dot := strings.Index(expr.Callee, "."); dot >= 0 {
		pkgName := expr.Callee[:dot]
		symbol := expr.Callee[dot+1:]
		imported, ok := c.importedPackages[pkgName]
		if !ok {
			return ExprType{}, fmt.Errorf("unknown package '%s'", pkgName)
		}
		lookupName = symbol
		lookupTable = imported.functions
		calleePackage = pkgName
	}

	signature, ok := lookupTable[lookupName]
	if !ok {
		if dot := strings.Index(expr.Callee, "."); dot >= 0 {
			pkgName := expr.Callee[:dot]
			symbol := expr.Callee[dot+1:]
			if imported, ok := c.importedPackages[pkgName]; ok {
				if _, exists := imported.records[symbol]; exists {
					return ExprType{}, fmt.Errorf("package-qualified type '%s.%s' used where a function is required", pkgName, symbol)
				}
				if _, exists := imported.enums[symbol]; exists {
					return ExprType{}, fmt.Errorf("package-qualified type '%s.%s' used where a function is required", pkgName, symbol)
				}
			}
			return ExprType{}, fmt.Errorf("package '%s' has no function '%s'", pkgName, symbol)
		}
		return ExprType{}, fmt.Errorf("undefined function: %s", expr.Callee)
	}
	if calleePackage != "" {
		signature = c.qualifyImportedSignature(calleePackage, signature)
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

func (c checker) qualifyImportedSignature(pkgName string, signature functionSignature) functionSignature {
	qualified := functionSignature{
		parameters: make([]Type, 0, len(signature.parameters)),
		returnType: c.qualifyImportedType(pkgName, signature.returnType),
		isFallible: signature.isFallible,
	}
	for _, parameter := range signature.parameters {
		qualified.parameters = append(qualified.parameters, c.qualifyImportedType(pkgName, parameter))
	}
	return qualified
}

func (c checker) qualifyImportedType(pkgName string, valueType Type) Type {
	if valueType.Name == "" || valueType.IsArray {
		return valueType
	}
	if strings.Contains(valueType.Name, ".") {
		return valueType
	}
	imported := c.importedPackages[pkgName]
	if _, ok := imported.records[valueType.Name]; ok {
		valueType.Name = pkgName + "." + valueType.Name
		return valueType
	}
	if _, ok := imported.enums[valueType.Name]; ok {
		valueType.Name = pkgName + "." + valueType.Name
		return valueType
	}
	return valueType
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
	case "Print":
		if isPrintableType(argumentType.ValueType) {
			return ExprType{ValueType: Type{Base: BaseTypeInt}}, nil
		}
		return ExprType{}, fmt.Errorf("function 'Print' argument 1 has unsupported type %s", argumentType.ValueType)
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
		return ExprType{}, fmt.Errorf("unsupported built-in function '%s'", expr.Callee)
	}
}

func isPrintableType(valueType Type) bool {
	if valueType.Name != "" {
		return !valueType.IsArray && !valueType.IsVector && !valueType.IsMatrix
	}
	if valueType.IsArray {
		return valueType.Base == BaseTypeInt || valueType.Base == BaseTypeFloat || valueType.Base == BaseTypeBool || valueType.Base == BaseTypeString
	}
	if valueType.IsVector || valueType.IsMatrix {
		return valueType.Base == BaseTypeInt || valueType.Base == BaseTypeFloat
	}
	return valueType.Base == BaseTypeInt || valueType.Base == BaseTypeFloat || valueType.Base == BaseTypeBool || valueType.Base == BaseTypeString || valueType.Base == BaseTypeError
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
		return fmt.Errorf("function '%s' does not accept dimensioned arrays", functionName)
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

func (c checker) checkVectorLiteralExpr(scope *scope, expr ast.VectorLiteralExpr, ctx functionContext) (Type, error) {
	if len(expr.Elements) == 0 {
		return Type{}, fmt.Errorf("empty vector literals are not supported")
	}
	firstType, err := c.checkExpr(scope, expr.Elements[0], ctx)
	if err != nil {
		return Type{}, err
	}
	if firstType.Fallible {
		return Type{}, fmt.Errorf("fallible expression must be handled explicitly")
	}
	if !isNumericScalar(firstType.ValueType) {
		return Type{}, fmt.Errorf("Vector literals require numeric elements, got %s", firstType.ValueType)
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
			return Type{}, fmt.Errorf("Vector literals require homogeneous element type")
		}
	}
	return Type{Base: firstType.ValueType.Base, Dimension: firstType.ValueType.Dimension, IsVector: true}, nil
}

func (c checker) checkMatrixLiteralExpr(scope *scope, expr ast.MatrixLiteralExpr, ctx functionContext) (Type, error) {
	if len(expr.Rows) == 0 {
		return Type{}, fmt.Errorf("empty matrix literals are not supported")
	}
	if len(expr.Rows[0]) == 0 {
		return Type{}, fmt.Errorf("matrix rows must not be empty")
	}
	expectedCols := len(expr.Rows[0])
	firstType, err := c.checkExpr(scope, expr.Rows[0][0], ctx)
	if err != nil {
		return Type{}, err
	}
	if firstType.Fallible {
		return Type{}, fmt.Errorf("fallible expression must be handled explicitly")
	}
	if !isNumericScalar(firstType.ValueType) {
		return Type{}, fmt.Errorf("Matrix literals require numeric elements, got %s", firstType.ValueType)
	}
	for _, row := range expr.Rows {
		if len(row) != expectedCols {
			return Type{}, fmt.Errorf("matrix rows must all have equal length")
		}
		for _, element := range row {
			elementType, err := c.checkExpr(scope, element, ctx)
			if err != nil {
				return Type{}, err
			}
			if elementType.Fallible {
				return Type{}, fmt.Errorf("fallible expression must be handled explicitly")
			}
			if elementType.ValueType != firstType.ValueType {
				return Type{}, fmt.Errorf("Matrix literals require homogeneous element type")
			}
		}
	}
	return Type{Base: firstType.ValueType.Base, Dimension: firstType.ValueType.Dimension, IsMatrix: true}, nil
}

func (c checker) checkRecordLiteralExpr(scope *scope, expr ast.RecordLiteralExpr, ctx functionContext) (ExprType, error) {
	recordDecl, ok := c.lookupRecord(expr.TypeName)
	if !ok {
		if c.isKnownTypeName(expr.TypeName) {
			return ExprType{}, fmt.Errorf("record literal requires record type, got %s", expr.TypeName)
		}
		return ExprType{}, fmt.Errorf("unknown record type: %s", expr.TypeName)
	}
	seen := make(map[string]struct{}, len(expr.Fields))
	for _, field := range expr.Fields {
		if _, exists := seen[field.Name]; exists {
			return ExprType{}, fmt.Errorf("record '%s' field '%s' specified more than once", expr.TypeName, field.Name)
		}
		seen[field.Name] = struct{}{}

		expectedType, exists := recordDecl.fields[field.Name]
		if !exists {
			return ExprType{}, fmt.Errorf("record '%s' has no field '%s'", expr.TypeName, field.Name)
		}
		actualType, err := c.checkExpr(scope, field.Value, ctx)
		if err != nil {
			return ExprType{}, err
		}
		if actualType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly")
		}
		if !isAssignable(actualType.ValueType, expectedType) {
			return ExprType{}, fmt.Errorf("record '%s' field '%s' expects %s, got %s", expr.TypeName, field.Name, expectedType, actualType.ValueType)
		}
	}
	for _, fieldName := range recordDecl.fieldOrder {
		if _, exists := seen[fieldName]; !exists {
			return ExprType{}, fmt.Errorf("record '%s' missing field '%s'", expr.TypeName, fieldName)
		}
	}
	return ExprType{ValueType: Type{Name: expr.TypeName}}, nil
}

func (c checker) resolveType(typeRef ast.TypeRef) (Type, error) {
	if typeRef.VectorOf != nil || typeRef.MatrixOf != nil {
		var elementRef ast.TypeRef
		isVector := typeRef.VectorOf != nil
		if isVector {
			elementRef = *typeRef.VectorOf
		} else {
			elementRef = *typeRef.MatrixOf
		}
		elementType, err := c.resolveType(elementRef)
		if err != nil {
			return Type{}, err
		}
		if !isNumericScalar(elementType) {
			if isVector {
				return Type{}, fmt.Errorf("Vector does not support %s elements in M16", elementType)
			}
			return Type{}, fmt.Errorf("Matrix does not support %s elements in M16", elementType)
		}
		return Type{Base: elementType.Base, Dimension: elementType.Dimension, IsVector: isVector, IsMatrix: !isVector}, nil
	}

	qualifiedName := typeRef.Name
	if typeRef.Package != "" {
		qualifiedName = typeRef.Package + "." + typeRef.Name
		imported, ok := c.importedPackages[typeRef.Package]
		if !ok {
			return Type{}, fmt.Errorf("unknown package '%s'", typeRef.Package)
		}
		if _, ok := imported.records[typeRef.Name]; ok {
			if typeRef.HasUnit {
				return Type{}, fmt.Errorf("invalid dimension-qualified type syntax: %s<%s>", qualifiedName, typeRef.Dimension.String())
			}
			if typeRef.IsArray {
				return Type{}, fmt.Errorf("unknown type: %s[]", qualifiedName)
			}
			return Type{Name: qualifiedName}, nil
		}
		if _, ok := imported.enums[typeRef.Name]; ok {
			if typeRef.HasUnit {
				return Type{}, fmt.Errorf("invalid dimension-qualified type syntax: %s<%s>", qualifiedName, typeRef.Dimension.String())
			}
			if typeRef.IsArray {
				return Type{}, fmt.Errorf("unknown type: %s[]", qualifiedName)
			}
			return Type{Name: qualifiedName}, nil
		}
		if _, ok := imported.functions[typeRef.Name]; ok {
			return Type{}, fmt.Errorf("package-qualified function '%s.%s' used where a type is required", typeRef.Package, typeRef.Name)
		}
		return Type{}, fmt.Errorf("package '%s' has no type '%s'", typeRef.Package, typeRef.Name)
	}

	baseType, err := resolveBaseType(typeRef.Name)
	if err != nil {
		if _, isRecord := c.records[typeRef.Name]; !isRecord {
			if _, isEnum := c.enums[typeRef.Name]; !isEnum {
				return Type{}, err
			}
		}
		if typeRef.HasUnit {
			return Type{}, fmt.Errorf("invalid dimension-qualified type syntax: %s<%s>", typeRef.Name, typeRef.Dimension.String())
		}
		if typeRef.IsArray {
			return Type{}, fmt.Errorf("unknown type: %s[]", typeRef.Name)
		}
		return Type{Name: typeRef.Name}, nil
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

func (c checker) checkBinaryExpr(operator string, leftType Type, rightType Type) (Type, error) {
	if operator == "and" || operator == "or" {
		if leftType != (Type{Base: BaseTypeBool}) || rightType != (Type{Base: BaseTypeBool}) {
			return Type{}, fmt.Errorf("operator '%s' requires Bool operands", operator)
		}
		return Type{Base: BaseTypeBool}, nil
	}
	if isComparisonOperator(operator) {
		return c.checkComparisonExpr(operator, leftType, rightType)
	}
	if leftType.IsVector || leftType.IsMatrix || rightType.IsVector || rightType.IsMatrix {
		return c.checkLinearAlgebraBinaryExpr(operator, leftType, rightType)
	}
	if leftType.Base == BaseTypeRange || rightType.Base == BaseTypeRange || leftType.Base == BaseTypeString || rightType.Base == BaseTypeString || leftType.Base == BaseTypeError || rightType.Base == BaseTypeError {
		return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}
	if leftType.IsArray || rightType.IsArray {
		return c.checkArrayBinaryExpr(operator, leftType, rightType)
	}
	if leftType.Base == BaseTypeBool || rightType.Base == BaseTypeBool || leftType.Name != "" || rightType.Name != "" {
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

func (c checker) checkLinearAlgebraBinaryExpr(operator string, leftType Type, rightType Type) (Type, error) {
	isLeftContainer := leftType.IsVector || leftType.IsMatrix
	isRightContainer := rightType.IsVector || rightType.IsMatrix
	leftScalar := Type{Base: leftType.Base, Dimension: leftType.Dimension}
	rightScalar := Type{Base: rightType.Base, Dimension: rightType.Dimension}

	if operator == "@" {
		if leftType.IsMatrix && rightType.IsMatrix {
			result, err := c.checkBinaryExpr("*", leftScalar, rightScalar)
			if err != nil {
				return Type{}, err
			}
			return Type{Base: result.Base, Dimension: result.Dimension, IsMatrix: true}, nil
		}
		if leftType.IsMatrix && rightType.IsVector {
			result, err := c.checkBinaryExpr("*", leftScalar, rightScalar)
			if err != nil {
				return Type{}, err
			}
			return Type{Base: result.Base, Dimension: result.Dimension, IsVector: true}, nil
		}
		return Type{}, fmt.Errorf("operator '@' not defined for %s and %s", leftType, rightType)
	}

	if isLeftContainer && isRightContainer {
		if leftType.IsVector != rightType.IsVector || leftType.IsMatrix != rightType.IsMatrix {
			return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
		}
		result, err := c.checkBinaryExpr(operator, leftScalar, rightScalar)
		if err != nil {
			return Type{}, err
		}
		result.IsVector = leftType.IsVector
		result.IsMatrix = leftType.IsMatrix
		return result, nil
	}

	if (operator == "+" || operator == "-" || operator == "*" || operator == "/") && isLeftContainer && isNumericScalar(rightType) {
		result, err := c.checkBinaryExpr(operator, leftScalar, rightScalar)
		if err != nil {
			return Type{}, err
		}
		result.IsVector = leftType.IsVector
		result.IsMatrix = leftType.IsMatrix
		return result, nil
	}
	if (operator == "+" || operator == "-" || operator == "*" || operator == "/") && isRightContainer && isNumericScalar(leftType) {
		result, err := c.checkBinaryExpr(operator, leftScalar, rightScalar)
		if err != nil {
			return Type{}, err
		}
		result.IsVector = rightType.IsVector
		result.IsMatrix = rightType.IsMatrix
		return result, nil
	}
	return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
}

func (c checker) checkArrayBinaryExpr(operator string, leftType Type, rightType Type) (Type, error) {
	if !leftType.IsArray || !rightType.IsArray {
		return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}
	if leftType.Base == BaseTypeBool || leftType.Base == BaseTypeString || leftType.Base == BaseTypeError || rightType.Base == BaseTypeBool || rightType.Base == BaseTypeString || rightType.Base == BaseTypeError {
		return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}
	result, err := c.checkBinaryExpr(operator, Type{Base: leftType.Base, Dimension: leftType.Dimension}, Type{Base: rightType.Base, Dimension: rightType.Dimension})
	if err != nil {
		return Type{}, err
	}
	result.IsArray = true
	return result, nil
}

func (c checker) checkComparisonExpr(operator string, leftType Type, rightType Type) (Type, error) {
	if isOrderingOperator(operator) && (leftType == (Type{Base: BaseTypeBool}) || leftType == (Type{Base: BaseTypeString})) && leftType == rightType {
		return Type{}, fmt.Errorf("operator %q not defined for %s", operator, leftType)
	}
	if leftType.IsArray || rightType.IsArray || leftType.IsVector || rightType.IsVector || leftType.IsMatrix || rightType.IsMatrix || leftType.Base == BaseTypeRange || rightType.Base == BaseTypeRange || leftType.Base == BaseTypeError || rightType.Base == BaseTypeError {
		return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}

	if isNumericScalar(leftType) && isNumericScalar(rightType) {
		if leftType.Dimension != rightType.Dimension {
			return Type{}, fmt.Errorf("cannot compare %s and %s", leftType, rightType)
		}
		return Type{Base: BaseTypeBool}, nil
	}

	if isEqualityOperator(operator) {
		if leftType == (Type{Base: BaseTypeBool}) && rightType == (Type{Base: BaseTypeBool}) {
			return Type{Base: BaseTypeBool}, nil
		}
		if leftType == (Type{Base: BaseTypeString}) && rightType == (Type{Base: BaseTypeString}) {
			return Type{Base: BaseTypeBool}, nil
		}
		if leftType.Name != "" && rightType.Name != "" {
			if _, leftIsEnum := c.enums[leftType.Name]; leftIsEnum {
				if _, rightIsEnum := c.enums[rightType.Name]; rightIsEnum {
					if leftType.Name != rightType.Name {
						return Type{}, fmt.Errorf("operator %q requires matching enum types", operator)
					}
					return Type{Base: BaseTypeBool}, nil
				}
			}
		}
	}

	if isOrderingOperator(operator) && (leftType == (Type{Base: BaseTypeBool}) || leftType == (Type{Base: BaseTypeString}) || isEnumType(c, leftType)) && leftType == rightType {
		return Type{}, fmt.Errorf("operator %q not defined for %s", operator, leftType)
	}
	return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
}

func isEnumType(c checker, valueType Type) bool {
	if valueType.IsArray || valueType.Name == "" {
		return false
	}
	_, ok := c.lookupEnum(valueType.Name)
	return ok
}

func (c checker) lookupRecord(typeName string) (recordInfo, bool) {
	if pkgName, localName, ok := splitQualifiedTypeName(typeName); ok {
		imported, exists := c.importedPackages[pkgName]
		if !exists {
			return recordInfo{}, false
		}
		recordDecl, exists := imported.records[localName]
		return recordDecl, exists
	}
	recordDecl, exists := c.records[typeName]
	return recordDecl, exists
}

func (c checker) lookupEnum(typeName string) (enumInfo, bool) {
	if pkgName, localName, ok := splitQualifiedTypeName(typeName); ok {
		imported, exists := c.importedPackages[pkgName]
		if !exists {
			return enumInfo{}, false
		}
		enumDecl, exists := imported.enums[localName]
		return enumDecl, exists
	}
	enumDecl, exists := c.enums[typeName]
	return enumDecl, exists
}

func (c checker) isKnownTypeName(typeName string) bool {
	if _, exists := c.lookupRecord(typeName); exists {
		return true
	}
	if _, exists := c.lookupEnum(typeName); exists {
		return true
	}
	return false
}

func (c checker) flattenEnumTypeName(expr ast.Expr) (string, bool) {
	fieldAccess, ok := expr.(ast.FieldAccessExpr)
	if !ok {
		return "", false
	}
	pkgIdentifier, ok := fieldAccess.Target.(ast.IdentifierExpr)
	if !ok {
		return "", false
	}
	return pkgIdentifier.Name + "." + fieldAccess.Field, true
}

func splitQualifiedTypeName(typeName string) (string, string, bool) {
	dot := strings.Index(typeName, ".")
	if dot <= 0 || dot == len(typeName)-1 {
		return "", "", false
	}
	if strings.Index(typeName[dot+1:], ".") >= 0 {
		return "", "", false
	}
	return typeName[:dot], typeName[dot+1:], true
}

func isComparisonOperator(operator string) bool {
	return isEqualityOperator(operator) || isOrderingOperator(operator)
}

func isEqualityOperator(operator string) bool {
	return operator == "==" || operator == "!="
}

func isOrderingOperator(operator string) bool {
	return operator == "<" || operator == "<=" || operator == ">" || operator == ">="
}

func isNumericBaseType(baseType BaseType) bool {
	return baseType == BaseTypeInt || baseType == BaseTypeFloat
}

func isNumericScalar(valueType Type) bool {
	return !valueType.IsArray && !valueType.IsVector && !valueType.IsMatrix && isNumericBaseType(valueType.Base)
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
	if actual.Name != "" || expected.Name != "" {
		return false
	}
	if actual.IsArray != expected.IsArray || actual.IsVector != expected.IsVector || actual.IsMatrix != expected.IsMatrix || actual.Dimension != expected.Dimension {
		return false
	}
	return actual.Base == BaseTypeInt && expected.Base == BaseTypeFloat
}
