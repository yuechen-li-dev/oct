package typecheck

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"oct/internal/ast"
	"oct/internal/builtin"
	"oct/internal/dimension"
	"oct/internal/project"
)

const decisionLadderIfElseDiagnostic = "nested decision-ladder `if/else` is not allowed in Oct by design. Use a condition-switch expression instead:\nlet value = switch {\n    case cond1 => value1\n    case cond2 => value2\n    else => defaultValue\n}"

var (
	uiPixelDimension  = mustDimension("px")
	uiAnchorDimension = mustDimension("ui")
	timeSecondDim     = mustDimension("s")
)

type BaseType string

const (
	BaseTypeInt     BaseType = "Int"
	BaseTypeFloat   BaseType = "Float"
	BaseTypeComplex BaseType = "Complex"
	BaseTypeBool    BaseType = "Bool"
	BaseTypeRange   BaseType = "Range"
	BaseTypeString  BaseType = "String"
	BaseTypeBytes   BaseType = "Bytes"
	BaseTypeError   BaseType = "Error"
	BaseTypeVoid    BaseType = "Void"
	BaseTypeUI      BaseType = "UI"
	BaseTypeIndex   BaseType = "Index"
)

type Type struct {
	Base              BaseType
	Name              string
	Dimension         dimension.Dimension
	Tuple             *tupleType
	IsArray           bool
	ArrayDepth        int
	IsVector          bool
	IsMatrix          bool
	IsFunction        bool
	FunctionSignature string
	IsFlowInstance    bool
	FlowIdentity      string
	FlowResultType    string
	FlowResult        *Type
}

type tupleType struct {
	Elements []Type
}

func mustDimension(name string) dimension.Dimension {
	dim, ok := dimension.FromBaseName(name)
	if !ok {
		panic(fmt.Sprintf("unknown built-in dimension %q", name))
	}
	return dim
}

func withArrayDepth(t Type, depth int) Type {
	t.ArrayDepth = depth
	t.IsArray = depth > 0
	return t
}

func peelArrayType(t Type) Type {
	if !t.IsArray || t.ArrayDepth <= 0 {
		return t
	}
	return withArrayDepth(t, t.ArrayDepth-1)
}

func (t Type) String() string {
	if t.Tuple != nil {
		parts := make([]string, 0, len(t.Tuple.Elements))
		for _, element := range t.Tuple.Elements {
			parts = append(parts, element.String())
		}
		return "(" + strings.Join(parts, ", ") + ")"
	}
	base := t.Name
	if base == "" {
		base = string(t.Base)
	}
	if isDimensionCapableBaseType(t.Base) && !t.Dimension.IsDimensionless() {
		base += "<" + t.Dimension.String() + ">"
	}
	if t.IsArray {
		return base + strings.Repeat("[]", t.ArrayDepth)
	}
	if t.IsFunction {
		return t.FunctionSignature
	}
	if t.IsFlowInstance {
		return "FlowInstance<" + t.FlowResultType + ">"
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
	EinTerm   *einsteinTermType
}

type einsteinTermType struct {
	ScalarType Type
	Labels     [2]string
	HasLabels  bool
}

type functionSignature struct {
	parameters []Type
	returnType Type
	isFallible bool
}

func (s functionSignature) asType() Type {
	return Type{IsFunction: true, FunctionSignature: s.String()}
}

func (s functionSignature) String() string {
	parts := make([]string, 0, len(s.parameters))
	for _, parameter := range s.parameters {
		parts = append(parts, parameter.String())
	}
	result := "fn(" + strings.Join(parts, ", ") + ") -> " + s.returnType.String()
	if s.isFallible {
		result += " ! Error"
	}
	return result
}

type functionContext struct {
	name        string
	returnType  Type
	isFallible  bool
	isTestFile  bool
	isFact      bool
	isTheory    bool
	isBenchmark bool
	inFlow      bool
	inState     bool
	states      map[string]struct{}
	boardType   Type
	board       map[string]Type
}

func Check(file ast.File) error {
	checker := checker{
		functions:     make(map[string]functionSignature),
		functionTypes: make(map[string]functionSignature),
		records:       make(map[string]recordInfo),
		enums:         make(map[string]enumInfo),
		flows:         make(map[string]flowSignature),
		typeNames:     make(map[string]struct{}),
	}
	return checker.checkFile(file)
}

func CheckProgram(program project.Program) error {
	packageCheckers := make(map[string]checker, len(program.Packages))
	for name, pkg := range program.Packages {
		file := ast.File{Package: name, Imports: pkg.Imports, Records: pkg.Records, Enums: pkg.Enums, Functions: pkg.Functions, Flows: pkg.Flows}
		chk := checker{
			functions:                    make(map[string]functionSignature),
			functionTypes:                make(map[string]functionSignature),
			records:                      make(map[string]recordInfo),
			enums:                        make(map[string]enumInfo),
			flows:                        make(map[string]flowSignature),
			typeNames:                    make(map[string]struct{}),
			allowUnresolvedImportedTypes: true,
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
				flows:     imported.flows,
			}
		}
		chk.importedPackages = imports
		chk.allowUnresolvedImportedTypes = false
		file := ast.File{Package: name, Imports: program.Packages[name].Imports, Records: program.Packages[name].Records, Enums: program.Packages[name].Enums, Functions: program.Packages[name].Functions, Flows: program.Packages[name].Flows}
		if err := chk.rebindRecordTypes(file); err != nil {
			return err
		}
		packageCheckers[name] = chk
	}

	for name, pkg := range program.Packages {
		file := ast.File{Package: name, Imports: pkg.Imports, Records: pkg.Records, Enums: pkg.Enums, Functions: pkg.Functions, Flows: pkg.Flows}
		chk := packageCheckers[name]
		if err := chk.registerFunctionSignatures(file); err != nil {
			return err
		}
		packageCheckers[name] = chk
	}

	for name, pkg := range program.Packages {
		file := ast.File{Package: name, Imports: pkg.Imports, Records: pkg.Records, Enums: pkg.Enums, Functions: pkg.Functions, Flows: pkg.Flows}
		chk := packageCheckers[name]
		if err := chk.checkPackageFunctions(file); err != nil {
			return err
		}
	}
	return nil
}

func (c checker) rebindRecordTypes(file ast.File) error {
	for _, record := range file.Records {
		fields := make(map[string]Type, len(record.Fields))
		fieldOrder := make([]string, 0, len(record.Fields))
		for _, field := range record.Fields {
			fieldType, err := c.resolveNonReturnType(field.Type)
			if err != nil {
				return fmt.Errorf("record '%s' field '%s': %w", record.Name, field.Name, err)
			}
			fields[field.Name] = fieldType
			fieldOrder = append(fieldOrder, field.Name)
		}
		c.records[record.Name] = recordInfo{fields: fields, fieldOrder: fieldOrder}
	}
	return nil
}

type checker struct {
	functions                    map[string]functionSignature
	functionTypes                map[string]functionSignature
	records                      map[string]recordInfo
	enums                        map[string]enumInfo
	flows                        map[string]flowSignature
	typeNames                    map[string]struct{}
	importedPackages             map[string]packageSymbols
	allowUnresolvedImportedTypes bool
}

type packageSymbols struct {
	functions map[string]functionSignature
	records   map[string]recordInfo
	enums     map[string]enumInfo
	flows     map[string]flowSignature
}

type recordInfo struct {
	fields     map[string]Type
	fieldOrder []string
}

type enumInfo struct {
	variants map[string]enumVariantInfo
}

type enumVariantInfo struct {
	payload *Type
}

type flowSignature struct {
	parameters []Type
	returnType Type
	boardType  *Type
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
	if err := c.rebindRecordTypes(file); err != nil {
		return err
	}
	if err := c.registerFunctionSignatures(file); err != nil {
		return err
	}
	return c.checkPackageFunctions(file)
}

func isRandomBuiltinAlias(name string) bool {
	switch name {
	case "RngSeed", "RandInt", "RandFloat01", "RandFloatRange", "RandBernoulli", "RandNormal", "Gaussian", "CryptoRandInt", "CryptoRandFloat01", "CryptoRandBytes":
		return true
	default:
		return false
	}
}

func (c checker) registerPackageDeclarations(file ast.File) error {
	for _, builtinTypeName := range []string{string(BaseTypeInt), string(BaseTypeFloat), string(BaseTypeComplex), string(BaseTypeBool), string(BaseTypeString), string(BaseTypeBytes), string(BaseTypeError), string(BaseTypeVoid), string(BaseTypeUI), string(BaseTypeIndex)} {
		c.typeNames[builtinTypeName] = struct{}{}
	}
	for _, record := range file.Records {
		if _, exists := c.typeNames[record.Name]; exists {
			return fmt.Errorf("duplicate type: %s", record.Name)
		}
		c.typeNames[record.Name] = struct{}{}
	}
	for _, enumDecl := range file.Enums {
		if _, exists := c.typeNames[enumDecl.Name]; exists {
			return fmt.Errorf("duplicate type: %s", enumDecl.Name)
		}
		c.typeNames[enumDecl.Name] = struct{}{}
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
		if builtin.IsName(function.Name) && !(file.Package == "Random" && isRandomBuiltinAlias(function.Name)) {
			return fmt.Errorf("function %s: cannot redeclare built-in function", function.Name)
		}
		if _, exists := c.functions[function.Name]; exists {
			return fmt.Errorf("duplicate function: %s", function.Name)
		}
		c.functions[function.Name] = functionSignature{}
	}
	for _, flow := range file.Flows {
		if builtin.IsName(flow.Name) {
			return fmt.Errorf("flow %s: cannot redeclare built-in function", flow.Name)
		}
		if _, exists := c.functions[flow.Name]; exists {
			return fmt.Errorf("duplicate flow: %s", flow.Name)
		}
		c.functions[flow.Name] = functionSignature{}
		c.flows[flow.Name] = flowSignature{}
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
	for _, flow := range file.Flows {
		signature, err := c.resolveFlowSignature(flow)
		if err != nil {
			return fmt.Errorf("flow %s: %w", flow.Name, err)
		}
		c.flows[flow.Name] = signature
	}
	return nil
}

func (c checker) checkPackageFunctions(file ast.File) error {
	for _, function := range file.Functions {
		if err := c.checkFunction(function); err != nil {
			return err
		}
	}
	for _, flow := range file.Flows {
		if err := c.checkFlow(flow); err != nil {
			return err
		}
	}
	return nil
}

func (c checker) registerRecord(record ast.RecordDecl) error {
	fields := make(map[string]Type, len(record.Fields))
	fieldOrder := make([]string, 0, len(record.Fields))
	for _, field := range record.Fields {
		if _, exists := fields[field.Name]; exists {
			return fmt.Errorf("record '%s' field '%s' specified more than once", record.Name, field.Name)
		}
		fieldType, err := c.resolveNonReturnType(field.Type)
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
	if len(enumDecl.Variants) == 0 {
		return fmt.Errorf("enum '%s' must declare at least one variant", enumDecl.Name)
	}

	variants := make(map[string]enumVariantInfo, len(enumDecl.Variants))
	for _, variant := range enumDecl.Variants {
		if _, exists := variants[variant.Name]; exists {
			return fmt.Errorf("enum '%s' variant '%s' specified more than once", enumDecl.Name, variant.Name)
		}
		var payloadType *Type
		if variant.Payload != nil {
			resolved, err := c.resolveNonReturnType(*variant.Payload)
			if err != nil {
				return fmt.Errorf("enum '%s' variant '%s': %w", enumDecl.Name, variant.Name, err)
			}
			payloadType = &resolved
		}
		variants[variant.Name] = enumVariantInfo{payload: payloadType}
	}
	c.enums[enumDecl.Name] = enumInfo{variants: variants}
	return nil
}

func (c checker) resolveFunctionSignature(function ast.FunctionDecl) (functionSignature, error) {
	returnType, err := c.resolveReturnType(function.ReturnType)
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
		parameterType, err := c.resolveNonReturnType(parameter.Type)
		if err != nil {
			return functionSignature{}, fmt.Errorf("parameter %s: %w", parameter.Name, err)
		}
		parameters = append(parameters, parameterType)
	}

	return functionSignature{parameters: parameters, returnType: returnType, isFallible: function.IsFallible}, nil
}

func (c checker) resolveFlowSignature(flow ast.FlowDecl) (flowSignature, error) {
	returnType, err := c.resolveReturnType(flow.ReturnType)
	if err != nil {
		return flowSignature{}, err
	}
	parameters := make([]Type, 0, len(flow.Parameters))
	for _, parameter := range flow.Parameters {
		parameterType, err := c.resolveNonReturnType(parameter.Type)
		if err != nil {
			return flowSignature{}, fmt.Errorf("parameter %s: %w", parameter.Name, err)
		}
		parameters = append(parameters, parameterType)
	}
	var boardType *Type
	if len(flow.Board) > 0 {
		snapshot := Type{Name: flow.Name + "BoardSnapshot"}
		fields := make(map[string]Type, len(flow.Board))
		fieldOrder := make([]string, 0, len(flow.Board))
		for _, field := range flow.Board {
			fieldType, err := c.resolveFlowBoardFieldType(field.Type)
			if err != nil {
				return flowSignature{}, err
			}
			fields[field.Name] = fieldType
			fieldOrder = append(fieldOrder, field.Name)
		}
		c.records[snapshot.Name] = recordInfo{fields: fields, fieldOrder: fieldOrder}
		c.typeNames[snapshot.Name] = struct{}{}
		boardType = &snapshot
	}
	return flowSignature{parameters: parameters, returnType: returnType, boardType: boardType}, nil
}

func (c checker) checkFunction(function ast.FunctionDecl) error {
	signature := c.functions[function.Name]
	if function.IsTheory {
		for rowIndex, row := range function.InlineData {
			if len(row.Values) != len(signature.parameters) {
				return fmt.Errorf("function %s: [InlineData] row %d argument count mismatch: expected %d, got %d", function.Name, rowIndex, len(signature.parameters), len(row.Values))
			}
			for argIndex, value := range row.Values {
				inlineType, err := c.checkInlineDataExpr(value)
				if err != nil {
					return fmt.Errorf("function %s: [InlineData] row %d argument %d: %w", function.Name, rowIndex, argIndex, err)
				}
				if inlineType != signature.parameters[argIndex] {
					return fmt.Errorf("function %s: [InlineData] row %d argument %d expects %s, got %s", function.Name, rowIndex, argIndex, signature.parameters[argIndex], inlineType)
				}
			}
		}
	}
	functionScope := newScope(nil)
	for i, parameter := range function.Parameters {
		functionScope.define(parameter.Name, signature.parameters[i], false)
	}
	if function.CycleTime != nil {
		cycleType, err := c.checkExpr(functionScope, function.CycleTime, functionContext{})
		if err != nil {
			return fmt.Errorf("function %s: [CycleTime] argument: %w", function.Name, err)
		}
		if cycleType.ValueType.Base != BaseTypeFloat || cycleType.ValueType.IsArray || cycleType.ValueType.IsVector || cycleType.ValueType.IsMatrix || cycleType.ValueType.Dimension != timeSecondDim {
			return fmt.Errorf("function %s: [CycleTime] expects a time quantity of type Float<s>, got %s", function.Name, cycleType.ValueType)
		}
		if literal, ok := function.CycleTime.(ast.FloatLiteral); ok {
			value, parseErr := strconv.ParseFloat(literal.Value, 64)
			if parseErr == nil && value <= 0 {
				return fmt.Errorf("function %s: [CycleTime] must be > 0<s>", function.Name)
			}
		}
	}

	ctx := functionContext{name: function.Name, returnType: signature.returnType, isFallible: signature.isFallible, isTestFile: function.IsTestFile, isFact: function.IsFact, isTheory: function.IsTheory, isBenchmark: function.IsBenchmark}
	hasReturn, err := c.checkBlock(functionScope, function.Body, ctx)
	if err != nil {
		return err
	}
	if signature.returnType.Base == BaseTypeVoid {
		return nil
	}
	if !hasReturn {
		return fmt.Errorf("function %s: missing return statement", function.Name)
	}

	return nil
}

func (c checker) checkFlow(flow ast.FlowDecl) error {
	signature := c.flows[flow.Name]
	if len(flow.States) == 0 {
		return fmt.Errorf("flow %s: must declare at least one state", flow.Name)
	}
	if flow.EntryState == "" || flow.EntryState != flow.States[0].Name {
		return fmt.Errorf("flow %s: entry state must be the first declared state", flow.Name)
	}

	flowScope := newScope(nil)
	for i, parameter := range flow.Parameters {
		if parameter.Name == "board" && len(flow.Board) > 0 {
			return fmt.Errorf("flow %s: parameter name 'board' conflicts with flow board declaration", flow.Name)
		}
		flowScope.define(parameter.Name, signature.parameters[i], false)
	}
	boardFields := make(map[string]Type, len(flow.Board))
	if len(flow.Board) > 0 {
		for _, field := range flow.Board {
			if _, exists := boardFields[field.Name]; exists {
				return fmt.Errorf("flow %s: duplicate board field '%s'", flow.Name, field.Name)
			}
			fieldType, err := c.resolveFlowBoardFieldType(field.Type)
			if err != nil {
				return fmt.Errorf("flow %s board field %s: %w", flow.Name, field.Name, err)
			}
			boardFields[field.Name] = fieldType
		}
		flowScope.define("board", Type{Name: "__flow_board_" + flow.Name}, false)
	}
	stateSet := make(map[string]struct{}, len(flow.States))
	for _, state := range flow.States {
		if _, exists := stateSet[state.Name]; exists {
			return fmt.Errorf("flow %s: duplicate state '%s'", flow.Name, state.Name)
		}
		stateSet[state.Name] = struct{}{}
	}

	ctx := functionContext{
		name:       flow.Name,
		returnType: signature.returnType,
		inFlow:     true,
		states:     stateSet,
		boardType:  Type{Name: "__flow_board_" + flow.Name},
		board:      boardFields,
	}
	for _, state := range flow.States {
		stateCtx := ctx
		stateCtx.inState = true
		if _, err := c.checkBlock(flowScope, state.Body, stateCtx); err != nil {
			return err
		}
	}
	return nil
}

func (c checker) resolveFlowBoardFieldType(ref ast.TypeRef) (Type, error) {
	t, err := c.resolveNonReturnType(ref)
	if err != nil {
		return Type{}, err
	}
	if t.IsArray || t.IsVector || t.IsMatrix || t.Name != "" || t.Base == BaseTypeError || t.Base == BaseTypeVoid || t.Base == BaseTypeComplex || t.Base == BaseTypeRange || t.Base == BaseTypeUI || t.Base == BaseTypeIndex {
		return Type{}, fmt.Errorf("board fields must be one of Bool, Int, Float, or String")
	}
	switch t.Base {
	case BaseTypeBool, BaseTypeInt, BaseTypeFloat, BaseTypeString:
		return t, nil
	default:
		return Type{}, fmt.Errorf("board fields must be one of Bool, Int, Float, or String")
	}
}

func (c checker) checkInlineDataExpr(expr ast.Expr) (Type, error) {
	switch node := expr.(type) {
	case ast.IntegerLiteral:
		return Type{Base: BaseTypeInt, Dimension: node.Dimension}, nil
	case ast.FloatLiteral:
		return Type{Base: BaseTypeFloat, Dimension: node.Dimension}, nil
	case ast.BoolLiteral:
		return Type{Base: BaseTypeBool}, nil
	case ast.StringLiteralExpr:
		return Type{Base: BaseTypeString}, nil
	case ast.FieldAccessExpr:
		enumName, variant, ok := flattenEnumValueExpr(node)
		if !ok {
			return Type{}, fmt.Errorf("enum value must be qualified as EnumName.Variant")
		}
		enumDecl, exists := c.lookupEnum(enumName)
		if !exists {
			return Type{}, fmt.Errorf("unknown enum type: %s", enumName)
		}
		if _, ok := enumDecl.variants[variant]; !ok {
			return Type{}, fmt.Errorf("enum '%s' has no variant '%s'", enumName, variant)
		}
		return Type{Name: enumName}, nil
	default:
		return Type{}, fmt.Errorf("unsupported inline data value")
	}
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

func genericUnhandledFallibleMessage() string {
	return "fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error"
}

func unhandledFallibleMessage(expr ast.Expr) string {
	if isResultFlowCall(expr) {
		return "Result(machine) is fallible because a flow may not have completed. Use Result(machine)! in tests when completion is required, Result(machine)? to propagate, or match Result(machine) to handle the not-completed/error case. For non-result inspection, use Active(machine), Complete(machine), or StateHistory(machine)."
	}
	if isBoardSnapshotFlowCall(expr) {
		return "BoardSnapshot(machine) is fallible because not every flow has a board and snapshot extraction can fail. Use BoardSnapshot(machine)! when the flow is known to declare a board, ? to propagate, or match to handle failure."
	}
	return genericUnhandledFallibleMessage()
}

func isResultFlowCall(expr ast.Expr) bool {
	call, ok := expr.(ast.CallExpr)
	if !ok {
		return false
	}
	return calleeEndsWithIdentifier(call.Callee, "Result")
}

func isBoardSnapshotFlowCall(expr ast.Expr) bool {
	call, ok := expr.(ast.CallExpr)
	if !ok {
		return false
	}
	return calleeEndsWithIdentifier(call.Callee, "BoardSnapshot")
}

func calleeEndsWithIdentifier(expr ast.Expr, target string) bool {
	switch node := expr.(type) {
	case ast.IdentifierExpr:
		return node.Name == target
	case ast.FieldAccessExpr:
		return node.Field == target
	default:
		return false
	}
}
func (c checker) checkStmt(scope *scope, stmt ast.Stmt, ctx functionContext) (bool, error) {
	switch node := stmt.(type) {
	case ast.LetStmt:
		var expected *Type
		if node.TypeHint != nil {
			resolved, resolveErr := c.resolveNonReturnType(*node.TypeHint)
			if resolveErr != nil {
				return false, fmt.Errorf("function %s: let %s: %w", ctx.name, node.Name, resolveErr)
			}
			expected = &resolved
		}
		valueType, err := c.checkExprWithExpected(scope, node.Value, ctx, expected)
		if err != nil {
			return false, fmt.Errorf("function %s: let %s: %w", ctx.name, node.Name, err)
		}
		if valueType.Fallible {
			return false, fmt.Errorf("function %s: let %s: %s", ctx.name, node.Name, unhandledFallibleMessage(node.Value))
		}
		if valueType.ValueType.Base == BaseTypeVoid {
			return false, fmt.Errorf("function %s: let %s: Void result cannot be used as a value", ctx.name, node.Name)
		}
		if expected != nil && !isAssignable(valueType.ValueType, *expected) {
			return false, fmt.Errorf("function %s: let %s: expected %s, got %s", ctx.name, node.Name, *expected, valueType.ValueType)
		}
		if expected != nil {
			valueType.ValueType = *expected
		}
		scope.define(node.Name, valueType.ValueType, false)
		return false, nil
	case ast.VarStmt:
		var expected *Type
		if node.TypeHint != nil {
			resolved, resolveErr := c.resolveNonReturnType(*node.TypeHint)
			if resolveErr != nil {
				return false, fmt.Errorf("function %s: var %s: %w", ctx.name, node.Name, resolveErr)
			}
			expected = &resolved
		}
		valueType, err := c.checkExprWithExpected(scope, node.Value, ctx, expected)
		if err != nil {
			return false, fmt.Errorf("function %s: var %s: %w", ctx.name, node.Name, err)
		}
		if valueType.Fallible {
			return false, fmt.Errorf("function %s: var %s: %s", ctx.name, node.Name, unhandledFallibleMessage(node.Value))
		}
		if valueType.ValueType.Base == BaseTypeVoid {
			return false, fmt.Errorf("function %s: var %s: Void result cannot be used as a value", ctx.name, node.Name)
		}
		if expected != nil && !isAssignable(valueType.ValueType, *expected) {
			return false, fmt.Errorf("function %s: var %s: expected %s, got %s", ctx.name, node.Name, *expected, valueType.ValueType)
		}
		if expected != nil {
			valueType.ValueType = *expected
		}
		scope.define(node.Name, valueType.ValueType, true)
		return false, nil
	case ast.AssignStmt:
		target, ok := scope.lookup(node.Name)
		if !ok {
			return false, fmt.Errorf("function %s: unknown binding '%s'", ctx.name, node.Name)
		}
		if !target.mutable {
			return false, fmt.Errorf("function %s: cannot assign to immutable binding '%s'; use `var %s = ...` for bindings that must be reassigned, or bind a new value with `let`", ctx.name, node.Name, node.Name)
		}
		valueType, err := c.checkExprWithExpected(scope, node.Value, ctx, &target.valueType)
		if err != nil {
			return false, fmt.Errorf("function %s: assignment to %s: %w", ctx.name, node.Name, err)
		}
		if valueType.Fallible {
			return false, fmt.Errorf("function %s: assignment to %s: %s", ctx.name, node.Name, unhandledFallibleMessage(node.Value))
		}
		if valueType.ValueType.Tuple != nil {
			return false, fmt.Errorf("function %s: assignment to %s: tuple return values must be destructured", ctx.name, node.Name)
		}
		if !isAssignable(valueType.ValueType, target.valueType) {
			return false, fmt.Errorf("function %s: assignment to %s: expected %s, got %s", ctx.name, node.Name, target.valueType, valueType.ValueType)
		}
		return false, nil
	case ast.DestructureAssignStmt:
		valueType, err := c.checkExpr(scope, node.Value, ctx)
		if err != nil {
			return false, fmt.Errorf("function %s: destructuring assignment: %w", ctx.name, err)
		}
		if valueType.Fallible {
			return false, fmt.Errorf("function %s: destructuring assignment: %s", ctx.name, unhandledFallibleMessage(node.Value))
		}
		if valueType.ValueType.Tuple == nil {
			return false, fmt.Errorf("function %s: destructuring assignment requires tuple return/value", ctx.name)
		}
		if len(node.Names) != len(valueType.ValueType.Tuple.Elements) {
			return false, fmt.Errorf("function %s: destructuring assignment expected %d targets, got %d", ctx.name, len(valueType.ValueType.Tuple.Elements), len(node.Names))
		}
		for i, name := range node.Names {
			target, ok := scope.lookup(name)
			if !ok {
				scope.define(name, valueType.ValueType.Tuple.Elements[i], false)
				continue
			}
			if !target.mutable {
				return false, fmt.Errorf("function %s: cannot assign to immutable binding '%s'; use `var %s = ...` for bindings that must be reassigned, or bind a new value with `let`", ctx.name, name, name)
			}
			if !isAssignable(valueType.ValueType.Tuple.Elements[i], target.valueType) {
				return false, fmt.Errorf("function %s: destructuring assignment tuple element %d to %s: expected %s, got %s", ctx.name, i, name, target.valueType, valueType.ValueType.Tuple.Elements[i])
			}
		}
		return false, nil
	case ast.IndexAssignStmt:
		target, ok := scope.lookup(node.Target)
		if !ok {
			return false, fmt.Errorf("function %s: unknown binding '%s'", ctx.name, node.Target)
		}
		if !target.mutable {
			return false, fmt.Errorf("function %s: cannot assign to immutable binding '%s'; use `var %s = ...` for bindings that must be reassigned, or bind a new value with `let`", ctx.name, node.Target, node.Target)
		}
		if len(node.Indices) == 0 {
			return false, fmt.Errorf("function %s: index assignment requires at least one index", ctx.name)
		}
		for _, idxExpr := range node.Indices {
			indexType, err := c.checkExpr(scope, idxExpr, ctx)
			if err != nil {
				return false, fmt.Errorf("function %s: index assignment: %w", ctx.name, err)
			}
			if indexType.Fallible {
				return false, fmt.Errorf("function %s: index assignment: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error", ctx.name)
			}
			if indexType.ValueType != (Type{Base: BaseTypeInt}) {
				return false, fmt.Errorf("function %s: index assignment indices must be Int", ctx.name)
			}
		}

		var elementType Type
		switch {
		case target.valueType.IsArray:
			if len(node.Indices) != 1 {
				return false, fmt.Errorf("function %s: array index assignment (`x[i] = ...`) requires exactly 1 index, got %d", ctx.name, len(node.Indices))
			}
			elementType = peelArrayType(target.valueType)
		case target.valueType.IsMatrix:
			if len(node.Indices) != 2 {
				return false, fmt.Errorf("function %s: matrix index assignment (`x[i, j] = ...`) requires exactly 2 indices, got %d", ctx.name, len(node.Indices))
			}
			elementType = Type{Base: target.valueType.Base, Dimension: target.valueType.Dimension}
		default:
			return false, fmt.Errorf("function %s: index assignment (`x[i] = ...`) requires an array or matrix target, got %s. For records, use immutable update (`x = x with { Field: value }`); for scalars, assign the whole value", ctx.name, target.valueType)
		}

		valueType, err := c.checkExpr(scope, node.Value, ctx)
		if err != nil {
			return false, fmt.Errorf("function %s: index assignment: %w", ctx.name, err)
		}
		if valueType.Fallible {
			return false, fmt.Errorf("function %s: index assignment: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error", ctx.name)
		}
		if !isAssignable(valueType.ValueType, elementType) {
			return false, fmt.Errorf("function %s: assigned value type does not match indexed element type", ctx.name)
		}
		return false, nil
	case ast.FieldAssignStmt:
		if !ctx.inState {
			return false, fmt.Errorf("function %s: field assignment (`x.field = ...`) is only valid for `board.field` inside flow state bodies; for records, use immutable update (`x = x with { Field: value }`)", ctx.name)
		}
		if node.Target != "board" {
			return false, fmt.Errorf("function %s: inside flow state bodies, field assignment is only supported on `board`, e.g. `board.Count = board.Count + 1`; ordinary records are immutable and must use `with`", ctx.name)
		}
		target, ok := scope.lookup(node.Target)
		if !ok {
			return false, fmt.Errorf("function %s: unknown binding '%s'", ctx.name, node.Target)
		}
		var fieldType Type
		if ctx.board != nil && target.valueType == ctx.boardType {
			var exists bool
			fieldType, exists = ctx.board[node.Field]
			if !exists {
				return false, fmt.Errorf("function %s: type '%s' has no field '%s'", ctx.name, target.valueType.Name, node.Field)
			}
		} else {
			recordDecl, recordOk := c.lookupRecord(target.valueType.Name)
			if !recordOk || target.valueType.IsArray || target.valueType.Base != "" {
				return false, fmt.Errorf("function %s: board field assignment requires board to be a record type, got %s", ctx.name, target.valueType)
			}
			var exists bool
			fieldType, exists = recordDecl.fields[node.Field]
			if !exists {
				return false, fmt.Errorf("function %s: type '%s' has no field '%s'", ctx.name, target.valueType.Name, node.Field)
			}
		}
		valueType, err := c.checkExpr(scope, node.Value, ctx)
		if err != nil {
			return false, fmt.Errorf("function %s: assignment to %s.%s: %w", ctx.name, node.Target, node.Field, err)
		}
		if valueType.Fallible {
			return false, fmt.Errorf("function %s: assignment to %s.%s: %s", ctx.name, node.Target, node.Field, unhandledFallibleMessage(node.Value))
		}
		if !isAssignable(valueType.ValueType, fieldType) {
			return false, fmt.Errorf("function %s: assignment to %s.%s: expected %s, got %s", ctx.name, node.Target, node.Field, fieldType, valueType.ValueType)
		}
		return false, nil
	case ast.ReturnStmt:
		if node.Value == nil {
			if ctx.returnType.Base != BaseTypeVoid {
				return false, fmt.Errorf("function %s: function expects %s, but return has no value", ctx.name, ctx.returnType)
			}
			return true, nil
		}
		if ctx.returnType.Base == BaseTypeVoid {
			return false, fmt.Errorf("function %s: Void function cannot return a value", ctx.name)
		}
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
			return false, fmt.Errorf("function %s: This standalone expression cannot be ran. In Oct, a statement like this must be a function call (for side effects), an assignment, or a return. If you meant to keep the value, assign it to a variable; if you meant to return it, use return.", ctx.name)
		}
		valueType, err := c.checkExpr(scope, node.Value, ctx)
		if err != nil {
			return false, fmt.Errorf("function %s: %w", ctx.name, err)
		}
		if valueType.Fallible {
			return false, fmt.Errorf("function %s: expression statement must not be fallible; %s", ctx.name, unhandledFallibleMessage(node.Value))
		}
		return false, nil
	case ast.ForStmt:
		rangeType, err := c.checkExpr(scope, node.Range, ctx)
		if err != nil {
			return false, fmt.Errorf("function %s: for %s: %w", ctx.name, node.Name, err)
		}
		if rangeType.Fallible {
			return false, fmt.Errorf("function %s: for %s: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error", ctx.name, node.Name)
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
		if isDecisionLadderElseIf(node) {
			return false, fmt.Errorf("function %s: %s", ctx.name, decisionLadderIfElseDiagnostic)
		}
		conditionType, err := c.checkExpr(scope, node.Condition, ctx)
		if err != nil {
			return false, fmt.Errorf("function %s: if condition: %w", ctx.name, err)
		}
		if conditionType.Fallible {
			return false, fmt.Errorf("function %s: if condition: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error", ctx.name)
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
			return false, fmt.Errorf("function %s: while condition: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error", ctx.name)
		}
		if conditionType.ValueType != (Type{Base: BaseTypeBool}) {
			return false, fmt.Errorf("function %s: while condition must be Bool, got %s", ctx.name, conditionType.ValueType)
		}

		_, err = c.checkBlock(scope, node.Body, ctx)
		if err != nil {
			return false, err
		}
		return false, nil
	case ast.PrometheusStmt:
		if !ctx.isBenchmark {
			return false, fmt.Errorf("function %s: PROMETHEUS blocks are only valid inside [Benchmark] functions", ctx.name)
		}
		_, err := c.checkBlock(scope, node.Body, ctx)
		if err != nil {
			return false, err
		}
		return false, nil
	case ast.GotoStmt:
		if !ctx.inState {
			return false, fmt.Errorf("function %s: goto is only valid inside flow state bodies", ctx.name)
		}
		if _, ok := ctx.states[node.Target]; !ok {
			return false, fmt.Errorf("flow %s: goto target '%s' does not exist", ctx.name, node.Target)
		}
		return false, nil
	case ast.SuspendStmt:
		if !ctx.inState {
			return false, fmt.Errorf("function %s: suspend is only valid inside flow state bodies", ctx.name)
		}
		return false, nil
	case ast.RememberStmt:
		if !ctx.inState {
			return false, fmt.Errorf("function %s: remember is only valid inside flow state bodies", ctx.name)
		}
		return false, nil
	case ast.ResumeStmt:
		if !ctx.inState {
			return false, fmt.Errorf("function %s: resume is only valid inside flow state bodies", ctx.name)
		}
		return false, nil
	case ast.WhenStmt:
		if !ctx.inState {
			return false, fmt.Errorf("function %s: guard when is only valid inside flow state bodies; outside flows use switch or when utility", ctx.name)
		}
		if node.Else == nil {
			return false, fmt.Errorf("function %s: when requires else branch", ctx.name)
		}
		caseReturns := make([]bool, 0, len(node.Cases))
		for _, whenCase := range node.Cases {
			conditionType, err := c.checkExpr(scope, whenCase.Condition, ctx)
			if err != nil {
				return false, fmt.Errorf("function %s: when case condition: %w", ctx.name, err)
			}
			if conditionType.Fallible {
				return false, fmt.Errorf("function %s: when case condition: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error", ctx.name)
			}
			if conditionType.ValueType != (Type{Base: BaseTypeBool}) {
				return false, fmt.Errorf("function %s: when case condition must be Bool, got %s", ctx.name, conditionType.ValueType)
			}
			returned, err := c.checkWhenAction(scope, whenCase.Action, ctx)
			if err != nil {
				return false, err
			}
			caseReturns = append(caseReturns, returned)
		}
		elseReturned, err := c.checkWhenAction(scope, node.Else, ctx)
		if err != nil {
			return false, err
		}
		allReturned := elseReturned
		for _, caseReturned := range caseReturns {
			allReturned = allReturned && caseReturned
		}
		return allReturned, nil
	default:
		return false, fmt.Errorf("function %s: unsupported statement %T", ctx.name, stmt)
	}
}

func (c checker) checkWhenAction(scope *scope, action ast.WhenAction, ctx functionContext) (bool, error) {
	switch node := action.(type) {
	case ast.WhenGotoAction:
		if _, ok := ctx.states[node.Target]; !ok {
			return false, fmt.Errorf("flow %s: goto target '%s' does not exist", ctx.name, node.Target)
		}
		return false, nil
	case ast.WhenSuspendAction:
		return false, nil
	case ast.WhenReturnAction:
		return c.checkStmt(scope, ast.ReturnStmt{Value: node.Value}, ctx)
	case ast.WhenBlockAction:
		if len(node.Statements) == 0 {
			return false, fmt.Errorf("function %s: when block action must contain at least one statement", ctx.name)
		}
		for i, statement := range node.Statements {
			switch statement.(type) {
			case ast.RememberStmt, ast.ResumeStmt, ast.GotoStmt, ast.SuspendStmt, ast.ReturnStmt, ast.FieldAssignStmt:
				// allowed
			default:
				return false, fmt.Errorf("function %s: a `when` action block may only contain flow-control actions (`remember`, `resume`, `goto`, `suspend`, `return`) and `board.field = ...` updates; move ordinary computation before the `when`, or use a normal block outside the flow guard", ctx.name)
			}
			isLast := i == len(node.Statements)-1
			if !isLast {
				switch statement.(type) {
				case ast.ReturnStmt, ast.GotoStmt, ast.SuspendStmt, ast.ResumeStmt:
					return false, fmt.Errorf("function %s: when block action has unreachable statements after control transfer", ctx.name)
				}
			}
		}
		switch node.Statements[len(node.Statements)-1].(type) {
		case ast.ReturnStmt, ast.GotoStmt, ast.SuspendStmt, ast.ResumeStmt:
			// required for deterministic when action lowering.
		default:
			return false, fmt.Errorf("function %s: a `when` action block must end with a control transfer (`goto`, `suspend`, `resume`, or `return`) so the flow transition is deterministic", ctx.name)
		}
		return c.checkBlock(scope, ast.Block{Statements: node.Statements}, ctx)
	default:
		return false, fmt.Errorf("function %s: when branch action must be goto, suspend, return, or action block", ctx.name)
	}
}

func isDecisionLadderElseIf(node ast.IfStmt) bool {
	if node.ElseBody == nil || len(node.ElseBody.Statements) != 1 {
		return false
	}
	_, ok := node.ElseBody.Statements[0].(ast.IfStmt)
	return ok
}

func (c checker) checkExpr(scope *scope, expr ast.Expr, ctx functionContext) (ExprType, error) {
	return c.checkExprWithExpected(scope, expr, ctx, nil)
}

func (c checker) checkExprWithExpected(scope *scope, expr ast.Expr, ctx functionContext, expected *Type) (ExprType, error) {
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
		valueType, err := c.checkArrayLiteralExpr(scope, node, ctx, expected)
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
		if ok {
			return ExprType{ValueType: valueBinding.valueType}, nil
		}
		if signature, exists := c.functions[node.Name]; exists {
			functionType := signature.asType()
			c.functionTypes[functionType.FunctionSignature] = signature
			return ExprType{ValueType: functionType}, nil
		}
		return ExprType{}, fmt.Errorf("undefined variable: %s", node.Name)
	case ast.CallExpr:
		return c.checkCallExpr(scope, node, ctx)
	case ast.RecordLiteralExpr:
		return c.checkRecordLiteralExpr(scope, node, ctx)
	case ast.RecordUpdateExpr:
		return c.checkRecordUpdateExpr(scope, node, ctx)
	case ast.EnumValueExpr:
		enumDecl, ok := c.lookupEnum(node.EnumName)
		if !ok {
			return ExprType{}, fmt.Errorf("unknown enum type: %s", node.EnumName)
		}
		variant, ok := enumDecl.variants[node.Variant]
		if !ok {
			return ExprType{}, fmt.Errorf("enum '%s' has no variant '%s'", node.EnumName, node.Variant)
		}
		if variant.payload != nil {
			return ExprType{}, fmt.Errorf("enum '%s' variant '%s' requires one payload argument", node.EnumName, node.Variant)
		}
		return ExprType{ValueType: Type{Name: node.EnumName}}, nil
	case ast.IndexExpr:
		targetType, err := c.checkExpr(scope, node.Target, ctx)
		if err != nil {
			return ExprType{}, err
		}
		if targetType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		indexTypes := make([]Type, 0, len(node.Indices))
		for _, idxExpr := range node.Indices {
			indexType, err := c.checkExpr(scope, idxExpr, ctx)
			if err != nil {
				return ExprType{}, err
			}
			if indexType.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			indexTypes = append(indexTypes, indexType.ValueType)
		}
		switch {
		case targetType.ValueType == (Type{Base: BaseTypeBytes}):
			if len(node.Indices) != 1 {
				return ExprType{}, fmt.Errorf("bytes indexing requires exactly 1 index, got %d", len(node.Indices))
			}
			if indexTypes[0] != (Type{Base: BaseTypeInt}) {
				return ExprType{}, fmt.Errorf("bytes indexing index must be Int, got %s", indexTypes[0])
			}
			return ExprType{ValueType: Type{Base: BaseTypeInt}}, nil
		case targetType.ValueType.IsArray:
			if len(node.Indices) != 1 {
				return ExprType{}, fmt.Errorf("array indexing requires exactly 1 index, got %d", len(node.Indices))
			}
			if indexTypes[0] != (Type{Base: BaseTypeInt}) {
				return ExprType{}, fmt.Errorf("array indexing index must be Int, got %s", indexTypes[0])
			}
			return ExprType{ValueType: peelArrayType(targetType.ValueType)}, nil
		case targetType.ValueType.IsVector:
			if len(node.Indices) != 1 {
				return ExprType{}, fmt.Errorf("vector indexing requires exactly 1 index, got %d", len(node.Indices))
			}
			if indexTypes[0] != (Type{Base: BaseTypeInt}) {
				return ExprType{}, fmt.Errorf("vector indexing index must be Int, got %s", indexTypes[0])
			}
			return ExprType{ValueType: Type{Base: targetType.ValueType.Base, Dimension: targetType.ValueType.Dimension}}, nil
		case targetType.ValueType.IsMatrix:
			if len(node.Indices) != 2 {
				return ExprType{}, fmt.Errorf("matrix indexing requires exactly 2 indices, got %d", len(node.Indices))
			}
			allInt := indexTypes[0] == (Type{Base: BaseTypeInt}) && indexTypes[1] == (Type{Base: BaseTypeInt})
			allIndex := indexTypes[0] == (Type{Base: BaseTypeIndex}) && indexTypes[1] == (Type{Base: BaseTypeIndex})
			if allInt {
				return ExprType{ValueType: Type{Base: targetType.ValueType.Base, Dimension: targetType.ValueType.Dimension}}, nil
			}
			if allIndex {
				labels, hasLabels := einsteinIndexNames(node)
				if hasLabels && labels[0] == labels[1] {
					return ExprType{}, fmt.Errorf("trace-style contraction '[%s,%s]' is not supported in M3; use Trace(...) for now", labels[0], labels[1])
				}
				return ExprType{
					ValueType: Type{Base: targetType.ValueType.Base, Dimension: targetType.ValueType.Dimension, IsMatrix: true},
					EinTerm: &einsteinTermType{
						ScalarType: Type{Base: targetType.ValueType.Base, Dimension: targetType.ValueType.Dimension},
						Labels:     labels,
						HasLabels:  hasLabels,
					},
				}, nil
			}
			return ExprType{}, fmt.Errorf("matrix indexing expects either [Int, Int] element access or [Index, Index] Einstein term access, got [%s, %s]", indexTypes[0], indexTypes[1])
		default:
			return ExprType{}, fmt.Errorf("cannot index non-indexable value of type %s", targetType.ValueType)
		}
	case ast.FieldAccessExpr:
		if pkgName, symbol, ok := c.flattenQualifiedFunctionName(node); ok {
			if imported, pkgExists := c.importedPackages[pkgName]; pkgExists {
				if signature, fnExists := imported.functions[symbol]; fnExists {
					qualified := c.qualifyImportedSignature(pkgName, signature)
					functionType := qualified.asType()
					c.functionTypes[functionType.FunctionSignature] = qualified
					return ExprType{ValueType: functionType}, nil
				}
			}
		}
		chain, chainOK := flattenFieldAccessChain(node)
		targetType, err := c.checkExpr(scope, node.Target, ctx)
		if err == nil {
			if targetType.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if ctx.board != nil && targetType.ValueType == ctx.boardType {
				fieldType, ok := ctx.board[node.Field]
				if !ok {
					return ExprType{}, c.missingFieldError(targetType.ValueType.Name, node.Field, chain, chainOK)
				}
				return ExprType{ValueType: fieldType}, nil
			}
			if targetType.ValueType.IsMatrix {
				switch node.Field {
				case "rows", "cols":
					return ExprType{ValueType: Type{Base: BaseTypeInt}}, nil
				default:
					return ExprType{}, missingTypeFieldError(fmt.Sprint(targetType.ValueType), node.Field, chain, chainOK)
				}
			}
			recordDecl, ok := c.lookupRecord(targetType.ValueType.Name)
			if !ok || targetType.ValueType.IsArray || targetType.ValueType.Base != "" {
				if !chainOK || strings.Count(chain, ".") < 2 {
					return ExprType{}, missingTypeFieldError(fmt.Sprint(targetType.ValueType), node.Field, chain, chainOK)
				}
				msg := fmt.Sprintf("type '%s' has no field '%s'", targetType.ValueType, node.Field)
				if chainOK {
					msg += fmt.Sprintf("\nwhile resolving '%s'\n'%s' has type %s, which has no fields", chain, chainParentPath(chain), targetType.ValueType)
				}
				return ExprType{}, errors.New(msg)
			}
			fieldType, ok := recordDecl.fields[node.Field]
			if !ok {
				return ExprType{}, c.missingFieldError(targetType.ValueType.Name, node.Field, chain, chainOK)
			}
			return ExprType{ValueType: fieldType}, nil
		}
		if chainOK && strings.Count(chain, ".") >= 2 {
			if enumName, ok := c.flattenEnumTypeName(node.Target); ok {
				if _, enumExists := c.lookupEnum(enumName); !enumExists {
					root := chainRoot(chain)
					return ExprType{}, fmt.Errorf("unknown name '%s'\nwhile resolving '%s'", root, chain)
				}
			} else {
				root := chainRoot(chain)
				return ExprType{}, fmt.Errorf("unknown name '%s'\nwhile resolving '%s'", root, chain)
			}
		}
		if identifier, ok := node.Target.(ast.IdentifierExpr); ok {
			if enumDecl, enumExists := c.lookupEnum(identifier.Name); enumExists {
				variant, variantExists := enumDecl.variants[node.Field]
				if !variantExists {
					return ExprType{}, fmt.Errorf("enum '%s' has no variant '%s'", identifier.Name, node.Field)
				}
				if variant.payload != nil {
					return ExprType{}, fmt.Errorf("enum '%s' variant '%s' requires one payload argument", identifier.Name, node.Field)
				}
				return ExprType{ValueType: Type{Name: identifier.Name}}, nil
			}
		}
		if enumName, ok := c.flattenEnumTypeName(node.Target); ok {
			enumDecl, enumExists := c.lookupEnum(enumName)
			if enumExists {
				variant, variantExists := enumDecl.variants[node.Field]
				if !variantExists {
					return ExprType{}, fmt.Errorf("enum '%s' has no variant '%s'", enumName, node.Field)
				}
				if variant.payload != nil {
					return ExprType{}, fmt.Errorf("enum '%s' variant '%s' requires one payload argument", enumName, node.Field)
				}
				return ExprType{ValueType: Type{Name: enumName}}, nil
			}
			if c.isKnownTypeName(enumName) {
				return ExprType{}, fmt.Errorf("type '%s' is not an enum", enumName)
			}
			return ExprType{}, fmt.Errorf("unknown enum type: %s", enumName)
		}
		return ExprType{}, err
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
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		rightType, err := c.checkExpr(scope, node.Right, ctx)
		if err != nil {
			return ExprType{}, err
		}
		if rightType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if leftType.EinTerm != nil || rightType.EinTerm != nil {
			return c.checkEinsteinBinaryExpr(node, leftType, rightType)
		}
		resultType, err := c.checkBinaryExpr(node.Operator, leftType.ValueType, rightType.ValueType)
		if err != nil {
			return ExprType{}, err
		}
		if node.Operator == "%" {
			if divisor, ok := staticIntegerValue(node.Right); ok && divisor == 0 {
				return ExprType{}, fmt.Errorf("modulo by zero")
			}
		}
		return ExprType{ValueType: resultType}, nil
	case ast.UnaryExpr:
		operandType, err := c.checkExpr(scope, node.Operand, ctx)
		if err != nil {
			return ExprType{}, err
		}
		if operandType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		switch node.Operator {
		case "not":
			if operandType.ValueType != (Type{Base: BaseTypeBool}) {
				return ExprType{}, fmt.Errorf("operator 'not' requires Bool operand")
			}
			return ExprType{ValueType: Type{Base: BaseTypeBool}}, nil
		case "-":
			if !isRealNumericScalar(operandType.ValueType) {
				return ExprType{}, fmt.Errorf("operator '-' requires Int or Float operand")
			}
			return ExprType{ValueType: operandType.ValueType}, nil
		default:
			return ExprType{}, fmt.Errorf("unsupported unary operator %q", node.Operator)
		}
	case ast.RangeExpr:
		startType, err := c.checkExpr(scope, node.Start, ctx)
		if err != nil {
			return ExprType{}, err
		}
		if startType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if startType.ValueType != (Type{Base: BaseTypeInt}) {
			return ExprType{}, fmt.Errorf("range start must be Int, got %s", startType.ValueType)
		}
		endType, err := c.checkExpr(scope, node.End, ctx)
		if err != nil {
			return ExprType{}, err
		}
		if endType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
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
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if stepType.ValueType != (Type{Base: BaseTypeInt}) {
				return ExprType{}, fmt.Errorf("range step must be Int, got %s", stepType.ValueType)
			}
			if stepValue, ok := staticIntegerValue(node.Step); ok && stepValue <= 0 {
				return ExprType{}, fmt.Errorf("range step must be positive, got %d", stepValue)
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
	case ast.MatchExpr:
		return c.checkEnumMatchExpr(scope, node, ctx)
	case ast.IfExpr:
		return c.checkIfExpr(scope, node, ctx)
	case ast.BatchExpr:
		return c.checkBatchExpr(scope, node, ctx)
	case ast.UtilityWhenExpr:
		return c.checkUtilityWhenExpr(scope, node, ctx)
	default:
		return ExprType{}, fmt.Errorf("unsupported expression %T", expr)
	}
}

func (c checker) checkUtilityWhenExpr(scope *scope, expr ast.UtilityWhenExpr, ctx functionContext) (ExprType, error) {
	if expr.ControllerBound && !ctx.inState {
		return ExprType{}, fmt.Errorf("when policy is only valid inside flow state bodies; outside flows use switch or when utility")
	}

	hysteresisType, err := c.checkExpr(scope, expr.Policy.Hysteresis, ctx)
	if err != nil {
		return ExprType{}, fmt.Errorf("utility when policy hysteresis: %w", err)
	}
	if hysteresisType.Fallible {
		return ExprType{}, fmt.Errorf("utility when policy hysteresis: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if hysteresisType.ValueType != (Type{Base: BaseTypeInt}) {
		return ExprType{}, fmt.Errorf("utility when policy hysteresis must be Int")
	}

	minCommitType, err := c.checkExpr(scope, expr.Policy.MinCommit, ctx)
	if err != nil {
		return ExprType{}, fmt.Errorf("utility when policy min_commit: %w", err)
	}
	if minCommitType.Fallible {
		return ExprType{}, fmt.Errorf("utility when policy min_commit: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if minCommitType.ValueType != (Type{Base: BaseTypeInt}) {
		return ExprType{}, fmt.Errorf("utility when policy min_commit must be Int")
	}

	if expr.Else == nil {
		return ExprType{}, fmt.Errorf("utility when requires else arm")
	}

	var resultType Type
	hasResultType := false
	for idx, whenCase := range expr.Cases {
		conditionType, err := c.checkExpr(scope, whenCase.Condition, ctx)
		if err != nil {
			return ExprType{}, fmt.Errorf("utility when case %d condition: %w", idx+1, err)
		}
		if conditionType.Fallible {
			return ExprType{}, fmt.Errorf("utility when case %d condition: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error", idx+1)
		}
		if conditionType.ValueType != (Type{Base: BaseTypeBool}) {
			return ExprType{}, fmt.Errorf("utility when case condition must be Bool")
		}

		scoreType, err := c.checkExpr(scope, whenCase.Score, ctx)
		if err != nil {
			return ExprType{}, fmt.Errorf("utility when case %d score: %w", idx+1, err)
		}
		if scoreType.Fallible {
			return ExprType{}, fmt.Errorf("utility when case %d score: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error", idx+1)
		}
		if scoreType.ValueType != (Type{Base: BaseTypeInt}) {
			return ExprType{}, fmt.Errorf("utility when case score must be Int")
		}

		valueType, err := c.checkExpr(scope, whenCase.Value, ctx)
		if err != nil {
			return ExprType{}, fmt.Errorf("utility when case %d value: %w", idx+1, err)
		}
		if valueType.Fallible {
			return ExprType{}, fmt.Errorf("utility when case %d value: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error", idx+1)
		}
		if !hasResultType {
			resultType = valueType.ValueType
			hasResultType = true
			continue
		}
		if valueType.ValueType != resultType {
			return ExprType{}, fmt.Errorf("utility when result arms must have matching types")
		}
	}

	elseType, err := c.checkExpr(scope, expr.Else, ctx)
	if err != nil {
		return ExprType{}, fmt.Errorf("utility when else: %w", err)
	}
	if elseType.Fallible {
		return ExprType{}, fmt.Errorf("utility when else: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if !hasResultType {
		return ExprType{ValueType: elseType.ValueType}, nil
	}
	if elseType.ValueType != resultType {
		return ExprType{}, fmt.Errorf("utility when result arms must have matching types")
	}
	return ExprType{ValueType: resultType}, nil
}

func (c checker) checkBatchExpr(scope *scope, expr ast.BatchExpr, ctx functionContext) (ExprType, error) {
	inputType, err := c.checkExpr(scope, expr.Input, ctx)
	if err != nil {
		return ExprType{}, fmt.Errorf("batch input: %w", err)
	}
	if inputType.Fallible {
		return ExprType{}, fmt.Errorf("batch input: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if !inputType.ValueType.IsArray {
		return ExprType{}, fmt.Errorf("batch input must be an array, got %s", inputType.ValueType)
	}

	if len(expr.Body.Statements) == 0 {
		return ExprType{}, fmt.Errorf("batch body must end with 'return <expr>'")
	}
	for idx, statement := range expr.Body.Statements[:len(expr.Body.Statements)-1] {
		if containsReturnStmt(statement) {
			return ExprType{}, fmt.Errorf("batch body must have exactly one return statement at the end (found early return at statement %d)", idx+1)
		}
	}

	returnStmt, ok := expr.Body.Statements[len(expr.Body.Statements)-1].(ast.ReturnStmt)
	if !ok || returnStmt.Value == nil {
		return ExprType{}, fmt.Errorf("batch body must end with 'return <expr>'")
	}

	itemType := inputType.ValueType
	itemType = peelArrayType(itemType)
	batchScope := newScope(scope)
	batchScope.define(expr.ItemName, itemType, false)

	bodyStatements := expr.Body.Statements[:len(expr.Body.Statements)-1]
	for _, statement := range bodyStatements {
		if _, err := c.checkStmt(batchScope, statement, ctx); err != nil {
			return ExprType{}, err
		}
	}

	resultType, err := c.checkExpr(batchScope, returnStmt.Value, ctx)
	if err != nil {
		return ExprType{}, fmt.Errorf("batch return: %w", err)
	}
	if resultType.Fallible {
		return ExprType{}, fmt.Errorf("batch return: return value must not be fallible; handle it with '?', '!', or match")
	}
	if resultType.ValueType.Base == BaseTypeVoid {
		return ExprType{}, fmt.Errorf("batch return: Void result cannot be used as a value")
	}

	outputType := resultType.ValueType
	outputType = withArrayDepth(outputType, outputType.ArrayDepth+1)
	return ExprType{ValueType: outputType}, nil
}

func containsReturnStmt(stmt ast.Stmt) bool {
	switch node := stmt.(type) {
	case ast.ReturnStmt:
		return true
	case ast.IfStmt:
		if containsReturnInBlock(node.ThenBody) {
			return true
		}
		return node.ElseBody != nil && containsReturnInBlock(*node.ElseBody)
	case ast.ForStmt:
		return containsReturnInBlock(node.Body)
	case ast.WhileStmt:
		return containsReturnInBlock(node.Body)
	case ast.MatchStmt:
		return containsReturnInBlock(node.OkBody) || containsReturnInBlock(node.ErrBody)
	default:
		return false
	}
}

func containsReturnInBlock(block ast.Block) bool {
	for _, statement := range block.Statements {
		if containsReturnStmt(statement) {
			return true
		}
	}
	return false
}

func staticIntegerValue(expr ast.Expr) (int64, bool) {
	switch node := expr.(type) {
	case ast.IntegerLiteral:
		if !node.Dimension.IsDimensionless() {
			return 0, false
		}
		value, err := strconv.ParseInt(node.Value, 10, 64)
		if err != nil {
			return 0, false
		}
		return value, true
	case ast.ParenExpr:
		return staticIntegerValue(node.Inner)
	case ast.BinaryExpr:
		left, ok := staticIntegerValue(node.Left)
		if !ok {
			return 0, false
		}
		right, ok := staticIntegerValue(node.Right)
		if !ok {
			return 0, false
		}
		switch node.Operator {
		case "+":
			return left + right, true
		case "-":
			return left - right, true
		case "*":
			return left * right, true
		case "/":
			if right == 0 {
				return 0, false
			}
			return left / right, true
		case "%":
			if right == 0 {
				return 0, false
			}
			remainder := left % right
			if remainder < 0 {
				if right > 0 {
					remainder += right
				} else {
					remainder -= right
				}
			}
			return remainder, true
		}
	}
	return 0, false
}

func (c checker) checkIfExpr(scope *scope, expr ast.IfExpr, ctx functionContext) (ExprType, error) {
	conditionType, err := c.checkExpr(scope, expr.Condition, ctx)
	if err != nil {
		return ExprType{}, fmt.Errorf("if expression condition: %w", err)
	}
	if conditionType.Fallible {
		return ExprType{}, fmt.Errorf("if expression condition: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
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
	if expr.Subject == nil {
		return c.checkConditionSwitchExpr(scope, expr, ctx)
	}

	subjectType, err := c.checkExpr(scope, expr.Subject, ctx)
	if err != nil {
		return ExprType{}, fmt.Errorf("switch subject: %w", err)
	}
	if subjectType.Fallible {
		return ExprType{}, fmt.Errorf("switch subject: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
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
			return ExprType{}, fmt.Errorf("switch case %d: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error", index+1)
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
			return ExprType{}, fmt.Errorf("switch else: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
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

func (c checker) checkEnumMatchExpr(scope *scope, expr ast.MatchExpr, ctx functionContext) (ExprType, error) {
	subjectType, err := c.checkExpr(scope, expr.Subject, ctx)
	if err != nil {
		return ExprType{}, fmt.Errorf("match subject: %w", err)
	}
	if subjectType.Fallible {
		return ExprType{}, fmt.Errorf("match subject: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if subjectType.ValueType.Name == "" {
		return ExprType{}, fmt.Errorf("match subject must be an enum, got %s", subjectType.ValueType)
	}
	enumDecl, exists := c.lookupEnum(subjectType.ValueType.Name)
	if !exists {
		return ExprType{}, fmt.Errorf("match subject must be an enum, got %s", subjectType.ValueType)
	}
	if len(expr.Cases) == 0 {
		return ExprType{}, fmt.Errorf("match must include at least one case")
	}

	seenVariants := make(map[string]struct{}, len(expr.Cases))
	var resultType Type
	hasResultType := false
	for index, matchCase := range expr.Cases {
		variantInfo, ok := enumDecl.variants[matchCase.Variant]
		if !ok {
			return ExprType{}, fmt.Errorf("match case %d: enum '%s' has no variant '%s'", index+1, subjectType.ValueType.Name, matchCase.Variant)
		}
		if _, duplicate := seenVariants[matchCase.Variant]; duplicate {
			return ExprType{}, fmt.Errorf("match case %d: duplicate case '%s.%s'", index+1, subjectType.ValueType.Name, matchCase.Variant)
		}
		seenVariants[matchCase.Variant] = struct{}{}
		if variantInfo.payload == nil && matchCase.Binding != "" {
			return ExprType{}, fmt.Errorf("match case %d: enum '%s' variant '%s' does not carry a payload", index+1, subjectType.ValueType.Name, matchCase.Variant)
		}
		if variantInfo.payload != nil && matchCase.Binding == "" {
			return ExprType{}, fmt.Errorf("match case %d: enum '%s' variant '%s' requires payload binding syntax", index+1, subjectType.ValueType.Name, matchCase.Variant)
		}

		caseScope := newScope(scope)
		if matchCase.Binding != "" {
			caseScope.define(matchCase.Binding, *variantInfo.payload, false)
		}
		caseValueType, err := c.checkExpr(caseScope, matchCase.Value, ctx)
		if err != nil {
			return ExprType{}, fmt.Errorf("match case %d: %w", index+1, err)
		}
		if caseValueType.Fallible {
			return ExprType{}, fmt.Errorf("match case %d: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error", index+1)
		}
		if !hasResultType {
			resultType = caseValueType.ValueType
			hasResultType = true
			continue
		}
		if caseValueType.ValueType != resultType {
			return ExprType{}, fmt.Errorf("match case %d: result type %s does not match %s", index+1, caseValueType.ValueType, resultType)
		}
	}

	if !hasAllEnumVariantsCovered(enumDecl, seenVariants) {
		return ExprType{}, fmt.Errorf("non-exhaustive match over enum '%s'", subjectType.ValueType.Name)
	}
	return ExprType{ValueType: resultType}, nil
}

func (c checker) checkConditionSwitchExpr(scope *scope, expr ast.SwitchExpr, ctx functionContext) (ExprType, error) {
	var resultType Type
	hasResultType := false

	for index, switchCase := range expr.Cases {
		conditionType, err := c.checkExpr(scope, switchCase.Match, ctx)
		if err != nil {
			return ExprType{}, fmt.Errorf("condition switch case %d: %w", index+1, err)
		}
		if conditionType.Fallible {
			return ExprType{}, fmt.Errorf("condition switch case %d: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error", index+1)
		}
		if conditionType.ValueType != (Type{Base: BaseTypeBool}) {
			return ExprType{}, fmt.Errorf("condition switch case must be Bool")
		}

		caseValueType, err := c.checkExpr(scope, switchCase.Value, ctx)
		if err != nil {
			return ExprType{}, fmt.Errorf("condition switch case %d: %w", index+1, err)
		}
		if caseValueType.Fallible {
			return ExprType{}, fmt.Errorf("condition switch case %d: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error", index+1)
		}
		if !hasResultType {
			resultType = caseValueType.ValueType
			hasResultType = true
			continue
		}
		if caseValueType.ValueType != resultType {
			if hasMatchingShapeDifferentDimensions(caseValueType.ValueType, resultType) {
				return ExprType{}, fmt.Errorf("condition switch result arms must have matching dimensions")
			}
			return ExprType{}, fmt.Errorf("condition switch result arms must have matching types")
		}
	}

	if expr.Else == nil {
		return ExprType{}, fmt.Errorf("condition switch requires else arm")
	}

	elseType, err := c.checkExpr(scope, expr.Else, ctx)
	if err != nil {
		return ExprType{}, fmt.Errorf("condition switch else: %w", err)
	}
	if elseType.Fallible {
		return ExprType{}, fmt.Errorf("condition switch else: fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if !hasResultType {
		return ExprType{ValueType: elseType.ValueType}, nil
	}
	if elseType.ValueType != resultType {
		if hasMatchingShapeDifferentDimensions(elseType.ValueType, resultType) {
			return ExprType{}, fmt.Errorf("condition switch result arms must have matching dimensions")
		}
		return ExprType{}, fmt.Errorf("condition switch result arms must have matching types")
	}

	return ExprType{ValueType: resultType}, nil
}

func hasMatchingShapeDifferentDimensions(left Type, right Type) bool {
	return left.Base == right.Base &&
		left.Name == right.Name &&
		left.IsArray == right.IsArray &&
		left.ArrayDepth == right.ArrayDepth &&
		left.IsVector == right.IsVector &&
		left.IsMatrix == right.IsMatrix &&
		left.Dimension != right.Dimension
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
	if enumName, variantName, ok := c.enumVariantFromCallee(expr.Callee); ok {
		enumDecl, exists := c.lookupEnum(enumName)
		if !exists {
			goto regularCall
		}
		variant, exists := enumDecl.variants[variantName]
		if !exists {
			return ExprType{}, fmt.Errorf("enum '%s' has no variant '%s'", enumName, variantName)
		}
		if variant.payload == nil {
			if len(expr.Arguments) != 0 {
				return ExprType{}, fmt.Errorf("enum '%s' variant '%s' does not accept a payload", enumName, variantName)
			}
			return ExprType{ValueType: Type{Name: enumName}}, nil
		}
		if len(expr.Arguments) != 1 {
			return ExprType{}, fmt.Errorf("enum '%s' variant '%s' requires exactly 1 payload argument", enumName, variantName)
		}
		payloadType, err := c.checkExpr(scope, expr.Arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if payloadType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if !isAssignable(payloadType.ValueType, *variant.payload) {
			return ExprType{}, fmt.Errorf("enum '%s' variant '%s' payload expects %s, got %s", enumName, variantName, *variant.payload, payloadType.ValueType)
		}
		return ExprType{ValueType: Type{Name: enumName}}, nil
	}

regularCall:
	calleeName, hasDirectName := flattenDirectCallName(expr.Callee)
	if hasDirectName && strings.HasPrefix(calleeName, "Assert.") {
		if len(expr.TypeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", calleeName)
		}
		if !ctx.isTestFile {
			return ExprType{}, fmt.Errorf("Assert is only available in .octest files")
		}
		return c.checkAssertCallExpr(scope, calleeName, expr.Arguments, ctx)
	}
	if hasDirectName && calleeName == "SkipTest" {
		if len(expr.TypeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", calleeName)
		}
		if !ctx.isTestFile {
			return ExprType{}, fmt.Errorf("SkipTest is only available in .octest files")
		}
		if !ctx.isFact && !ctx.isTheory {
			return ExprType{}, fmt.Errorf("SkipTest is only available in [Fact] or [Theory] tests")
		}
		return c.checkSkipTestCallExpr(scope, expr.Arguments, ctx)
	}
	if hasDirectName && calleeName == "error" {
		if len(expr.TypeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'error' does not accept type arguments")
		}
		if len(expr.Arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'error' expects 1 argument, got %d", len(expr.Arguments))
		}
		if _, ok := expr.Arguments[0].(ast.StringLiteralExpr); !ok {
			return ExprType{}, fmt.Errorf("error() requires a string literal")
		}
		return ExprType{ValueType: Type{Base: BaseTypeError}}, nil
	}
	if hasDirectName {
		if namespace, symbol, ok := splitTwoSegmentQualifiedName(calleeName); ok {
			if builtinName, mapped := builtin.ResolveNamespacedAlias(namespace, symbol); mapped {
				if _, imported := c.importedPackages[namespace]; !imported {
					return ExprType{}, fmt.Errorf("unknown namespace/module '%s'; did you forget `import %s`?", namespace, namespace)
				}
				return c.checkBuiltinCallExpr(scope, builtinName, expr.TypeArguments, expr.Arguments, ctx)
			}
		}
	}
	if hasDirectName && calleeName == "Int" && len(expr.Arguments) == 1 {
		return ExprType{}, fmt.Errorf("Int(...) is not a conversion in Oct because float-to-int conversion must choose a rounding policy explicitly. Use FloorToInt(x), CeilToInt(x), or RoundToInt(x). For sample counts, FloorToInt(sampleRate * duration) is usually intended.")
	}
	if hasDirectName && builtin.IsName(calleeName) {
		return c.checkBuiltinCallExpr(scope, calleeName, expr.TypeArguments, expr.Arguments, ctx)
	}
	if len(expr.TypeArguments) > 0 {
		if hasDirectName {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", calleeName)
		}
		return ExprType{}, fmt.Errorf("call target does not accept type arguments")
	}

	if hasDirectName {
		if signature, ok := c.lookupNamedFlowSignature(calleeName); ok {
			return c.checkFlowCallArguments(calleeName, signature, scope, expr.Arguments, ctx)
		}
		if signature, ok := c.lookupNamedFunctionSignature(calleeName); ok {
			return c.checkFunctionCallArguments(calleeName, signature, scope, expr.Arguments, ctx)
		}
		if dot := strings.Index(calleeName, "."); dot >= 0 {
			pkgName := calleeName[:dot]
			symbol := calleeName[dot+1:]
			imported, ok := c.importedPackages[pkgName]
			if !ok {
				return ExprType{}, fmt.Errorf("unknown package '%s'", pkgName)
			}
			if _, exists := imported.records[symbol]; exists {
				return ExprType{}, fmt.Errorf("package-qualified type '%s.%s' used where a function is required", pkgName, symbol)
			}
			if _, exists := imported.enums[symbol]; exists {
				return ExprType{}, fmt.Errorf("package-qualified type '%s.%s' used where a function is required", pkgName, symbol)
			}
			return ExprType{}, fmt.Errorf("package '%s' has no function '%s'", pkgName, symbol)
		}
	}
	calleeType, err := c.checkExpr(scope, expr.Callee, ctx)
	if err != nil {
		return ExprType{}, err
	}
	if calleeType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if !calleeType.ValueType.IsFunction {
		if hasDirectName {
			return ExprType{}, fmt.Errorf("undefined function: %s", calleeName)
		}
		return ExprType{}, fmt.Errorf("call target must be a function value, got %s", calleeType.ValueType)
	}
	signature, ok := c.functionTypes[calleeType.ValueType.FunctionSignature]
	if !ok {
		return ExprType{}, fmt.Errorf("internal error: missing function type metadata for %s", calleeType.ValueType.FunctionSignature)
	}
	return c.checkFunctionCallArguments(calleeType.ValueType.FunctionSignature, signature, scope, expr.Arguments, ctx)
}

func (c checker) checkSkipTestCallExpr(scope *scope, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	if len(arguments) != 1 {
		return ExprType{}, fmt.Errorf("function 'SkipTest' expects 1 argument, got %d", len(arguments))
	}
	reasonType, err := c.checkExpr(scope, arguments[0], ctx)
	if err != nil {
		return ExprType{}, err
	}
	if reasonType.ValueType != (Type{Base: BaseTypeString}) {
		return ExprType{}, fmt.Errorf("function 'SkipTest' argument 1 expects String, got %s", reasonType.ValueType)
	}
	if literal, ok := arguments[0].(ast.StringLiteralExpr); ok {
		if strings.TrimSpace(literal.Value) == "" {
			return ExprType{}, fmt.Errorf("SkipTest reason must be non-empty")
		}
	}
	return ExprType{ValueType: Type{Base: BaseTypeVoid}}, nil
}

func (c checker) enumVariantFromCallee(expr ast.Expr) (string, string, bool) {
	switch node := expr.(type) {
	case ast.FieldAccessExpr:
		enumName, variantName, ok := flattenEnumValueExpr(node)
		return enumName, variantName, ok
	default:
		return "", "", false
	}
}

func flowInstanceType(flowName string, resultType Type) Type {
	resultCopy := resultType
	return Type{IsFlowInstance: true, FlowIdentity: flowName, FlowResultType: resultType.String(), FlowResult: &resultCopy}
}

func (c checker) checkFlowCallArguments(displayName string, signature flowSignature, scope *scope, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	if len(arguments) != len(signature.parameters) {
		return ExprType{}, fmt.Errorf("flow '%s' expects %d arguments, got %d", displayName, len(signature.parameters), len(arguments))
	}
	for i, argumentExpr := range arguments {
		argumentType, err := c.checkExpr(scope, argumentExpr, ctx)
		if err != nil {
			return ExprType{}, err
		}
		if argumentType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		expected := signature.parameters[i]
		if isAssignable(argumentType.ValueType, expected) {
			continue
		}
		return ExprType{}, fmt.Errorf("flow '%s' argument %d expects %s, got %s", displayName, i+1, expected, argumentType.ValueType)
	}
	return ExprType{ValueType: flowInstanceType(displayName, signature.returnType)}, nil
}

func (c checker) checkFunctionCallArguments(displayName string, signature functionSignature, scope *scope, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	if len(arguments) != len(signature.parameters) {
		return ExprType{}, fmt.Errorf("function '%s' expects %d arguments, got %d", displayName, len(signature.parameters), len(arguments))
	}
	for i, argumentExpr := range arguments {
		argumentType, err := c.checkExpr(scope, argumentExpr, ctx)
		if err != nil {
			return ExprType{}, err
		}
		if argumentType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if argumentType.ValueType.Base == BaseTypeVoid {
			return ExprType{}, fmt.Errorf("Void result cannot be used as a value")
		}
		if !isAssignable(argumentType.ValueType, signature.parameters[i]) {
			return ExprType{}, fmt.Errorf("function '%s' argument %d expects %s, got %s", displayName, i+1, signature.parameters[i], argumentType.ValueType)
		}
	}
	return ExprType{ValueType: signature.returnType, Fallible: signature.isFallible}, nil
}

func (c checker) lookupNamedFunctionSignature(name string) (functionSignature, bool) {
	lookupName := name
	lookupTable := c.functions
	calleePackage := ""
	if dot := strings.Index(name, "."); dot >= 0 {
		pkgName := name[:dot]
		symbol := name[dot+1:]
		imported, ok := c.importedPackages[pkgName]
		if !ok {
			return functionSignature{}, false
		}
		lookupName = symbol
		lookupTable = imported.functions
		calleePackage = pkgName
	}
	signature, ok := lookupTable[lookupName]
	if !ok {
		return functionSignature{}, false
	}
	if calleePackage != "" {
		signature = c.qualifyImportedSignature(calleePackage, signature)
	}
	return signature, true
}

func (c checker) lookupNamedFlowSignature(name string) (flowSignature, bool) {
	lookupName := name
	lookupTable := c.flows
	if dot := strings.Index(name, "."); dot >= 0 {
		pkgName := name[:dot]
		symbol := name[dot+1:]
		imported, ok := c.importedPackages[pkgName]
		if !ok {
			return flowSignature{}, false
		}
		lookupName = symbol
		lookupTable = imported.flows
	}
	signature, ok := lookupTable[lookupName]
	return signature, ok
}

func flattenDirectCallName(expr ast.Expr) (string, bool) {
	switch node := expr.(type) {
	case ast.IdentifierExpr:
		return node.Name, true
	case ast.FieldAccessExpr:
		left, ok := node.Target.(ast.IdentifierExpr)
		if !ok {
			return "", false
		}
		return left.Name + "." + node.Field, true
	default:
		return "", false
	}
}

func splitTwoSegmentQualifiedName(name string) (string, string, bool) {
	dot := strings.Index(name, ".")
	if dot <= 0 || dot >= len(name)-1 {
		return "", "", false
	}
	if strings.Index(name[dot+1:], ".") >= 0 {
		return "", "", false
	}
	return name[:dot], name[dot+1:], true
}

func (c checker) checkAssertCallExpr(scope *scope, callee string, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	voidType := ExprType{ValueType: Type{Base: BaseTypeVoid}}
	switch callee {
	case "Assert.True", "Assert.False":
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
		}
		condType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if condType.ValueType != (Type{Base: BaseTypeBool}) {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects Bool, got %s", callee, condType.ValueType)
		}
		msgType, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if msgType.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function '%s' argument 2 expects String, got %s", callee, msgType.ValueType)
		}
		return voidType, nil
	case "Assert.Equal":
		if len(arguments) != 3 {
			return ExprType{}, fmt.Errorf("function '%s' expects 3 arguments, got %d", callee, len(arguments))
		}
		expectedType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		actualType, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if !isAssignable(actualType.ValueType, expectedType.ValueType) || !isAssignable(expectedType.ValueType, actualType.ValueType) {
			return ExprType{}, fmt.Errorf("function '%s' arguments 1 and 2 must have the same type", callee)
		}
		if !supportsAssertEqualType(expectedType.ValueType) {
			return ExprType{}, fmt.Errorf("Assert.Equal does not support type %s in M24a", expectedType.ValueType)
		}
		msgType, err := c.checkExpr(scope, arguments[2], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if msgType.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function '%s' argument 3 expects String, got %s", callee, msgType.ValueType)
		}
		return voidType, nil
	case "Assert.Near":
		if len(arguments) != 4 {
			return ExprType{}, fmt.Errorf("function '%s' expects 4 arguments, got %d", callee, len(arguments))
		}
		expectedType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		actualType, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		toleranceType, err := c.checkExpr(scope, arguments[2], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if expectedType.ValueType.Base != BaseTypeFloat || actualType.ValueType.Base != BaseTypeFloat || toleranceType.ValueType.Base != BaseTypeFloat ||
			expectedType.ValueType.IsArray || actualType.ValueType.IsArray || toleranceType.ValueType.IsArray ||
			expectedType.ValueType.IsVector || actualType.ValueType.IsVector || toleranceType.ValueType.IsVector ||
			expectedType.ValueType.IsMatrix || actualType.ValueType.IsMatrix || toleranceType.ValueType.IsMatrix ||
			expectedType.ValueType.Name != "" || actualType.ValueType.Name != "" || toleranceType.ValueType.Name != "" {
			return ExprType{}, fmt.Errorf("Assert.Near supports only Float scalars in M24a")
		}
		if expectedType.ValueType.Dimension != actualType.ValueType.Dimension || expectedType.ValueType.Dimension != toleranceType.ValueType.Dimension {
			return ExprType{}, fmt.Errorf("Assert.Near requires matching dimensions for expected, actual, and tolerance")
		}
		msgType, err := c.checkExpr(scope, arguments[3], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if msgType.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function '%s' argument 4 expects String, got %s", callee, msgType.ValueType)
		}
		return voidType, nil
	case "Assert.Error":
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
		}
		resultType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if !resultType.Fallible {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects fallible expression, got %s", callee, resultType.ValueType)
		}
		msgType, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if msgType.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function '%s' argument 2 expects String, got %s", callee, msgType.ValueType)
		}
		return voidType, nil
	case "Assert.LGTM":
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
		}
		resultType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if !resultType.Fallible {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects fallible expression, got %s", callee, resultType.ValueType)
		}
		msgType, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if msgType.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function '%s' argument 2 expects String, got %s", callee, msgType.ValueType)
		}
		return ExprType{ValueType: resultType.ValueType}, nil
	default:
		return ExprType{}, fmt.Errorf("unsupported Assert function '%s'", callee)
	}
}

func supportsAssertEqualType(valueType Type) bool {
	if valueType.IsArray || valueType.IsVector || valueType.IsMatrix {
		return false
	}
	if valueType.Name != "" {
		return true
	}
	switch valueType.Base {
	case BaseTypeBool, BaseTypeInt, BaseTypeFloat, BaseTypeString:
		return true
	default:
		return false
	}
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

func (c checker) checkBuiltinCallExpr(scope *scope, callee string, typeArguments []ast.TypeRef, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	if callee == "TupleProbe" || callee == "BoolIntProbe" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		if len(arguments) != 0 {
			return ExprType{}, fmt.Errorf("function '%s' expects 0 arguments, got %d", callee, len(arguments))
		}
		if callee == "TupleProbe" {
			return ExprType{ValueType: Type{Tuple: &tupleType{Elements: []Type{{Base: BaseTypeInt}, {Base: BaseTypeInt}}}}}, nil
		}
		return ExprType{ValueType: Type{Tuple: &tupleType{Elements: []Type{{Base: BaseTypeBool}, {Base: BaseTypeInt}}}}}, nil
	}
	randomBuiltin := callee
	switch callee {
	case "RngSeed", "RandInt", "RandFloat01", "RandFloatRange", "RandBernoulli", "RandNormal", "Gaussian", "CryptoRandInt", "CryptoRandFloat01", "CryptoRandBytes":
		randomBuiltin = "Random." + callee
	}
	if randomBuiltin == "Random.RngSeed" || randomBuiltin == "Random.RandInt" || randomBuiltin == "Random.RandFloat01" || randomBuiltin == "Random.RandFloatRange" || randomBuiltin == "Random.RandBernoulli" || randomBuiltin == "Random.RandNormal" || randomBuiltin == "Random.Gaussian" || randomBuiltin == "Random.CryptoRandInt" || randomBuiltin == "Random.CryptoRandFloat01" || randomBuiltin == "Random.CryptoRandBytes" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		resultPrefix := "Random."
		if !strings.Contains(callee, ".") {
			resultPrefix = ""
		}
		rngType := Type{Name: resultPrefix + "Rng"}
		randIntResultType := Type{Name: resultPrefix + "RandIntResult"}
		randFloatResultType := Type{Name: resultPrefix + "RandFloatResult"}
		randBoolResultType := Type{Name: resultPrefix + "RandBoolResult"}
		switch randomBuiltin {
		case "Random.RngSeed":
			if len(arguments) != 1 {
				return ExprType{}, fmt.Errorf("function '%s' expects 1 argument, got %d", callee, len(arguments))
			}
			return ExprType{ValueType: rngType}, nil
		case "Random.RandInt":
			if len(arguments) != 3 {
				return ExprType{}, fmt.Errorf("function '%s' expects 3 arguments, got %d", callee, len(arguments))
			}
			return ExprType{ValueType: randIntResultType}, nil
		case "Random.RandFloat01", "Random.RandFloatRange", "Random.RandNormal", "Random.Gaussian":
			if (randomBuiltin == "Random.RandFloat01" && len(arguments) != 1) || (randomBuiltin != "Random.RandFloat01" && len(arguments) != 3) {
				return ExprType{}, fmt.Errorf("function '%s' arity mismatch", callee)
			}
			return ExprType{ValueType: randFloatResultType}, nil
		case "Random.RandBernoulli":
			if len(arguments) != 2 {
				return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
			}
			return ExprType{ValueType: randBoolResultType}, nil
		case "Random.CryptoRandInt":
			return ExprType{ValueType: Type{Base: BaseTypeInt}, Fallible: true}, nil
		case "Random.CryptoRandFloat01":
			return ExprType{ValueType: Type{Base: BaseTypeFloat}, Fallible: true}, nil
		default:
			return ExprType{ValueType: Type{Base: BaseTypeBytes}, Fallible: true}, nil
		}
	}
	if callee == "PlotLine" || callee == "PlotScatter" || callee == "PlotRenderLine" || callee == "PlotRenderScatter" || callee == "PlotRenderHistogram" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		return c.checkPlotBuiltinCallExpr(scope, callee, arguments, ctx)
	}
	if callee == "WriteOctagon" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'WriteOctagon' does not accept type arguments")
		}
		return c.checkWriteOctagonBuiltinCallExpr(scope, callee, arguments, ctx)
	}
	if callee == "ArtifactWriteText" || callee == "ArtifactWriteLines" || callee == "ArtifactWriteMarkdown" || callee == "ArtifactWriteCsv" || callee == "ArtifactWriteJson" || callee == "ArtifactWriteOctagon" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		return c.checkArtifactBuiltinCallExpr(scope, callee, arguments, ctx)
	}
	if callee == "LoadOctagon" {
		return c.checkLoadOctagonBuiltinCallExpr(scope, callee, typeArguments, arguments, ctx)
	}
	if callee == "XlsxCreateWorkbook" || callee == "XlsxAddSheet" || callee == "XlsxSetCellString" || callee == "XlsxSetCellFloat" || callee == "XlsxSaveWorkbook" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		return c.checkXlsxBuiltinCallExpr(scope, callee, arguments, ctx)
	}
	if callee == "ImageLoad" || callee == "ImageSave" || callee == "ImageWidth" || callee == "ImageHeight" || callee == "ImageFormat" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		return c.checkImageBuiltinCallExpr(scope, callee, arguments, ctx)
	}
	if callee == "PdfNewPage" || callee == "PdfDrawText" || callee == "PdfDrawTextStyled" || callee == "PdfDrawImage" || callee == "PdfDrawImageSized" || callee == "PdfSave" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		return c.checkPdfBuiltinCallExpr(scope, callee, arguments, ctx)
	}
	if callee == "JsonNormalize" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		return c.checkJSONBuiltinCallExpr(scope, callee, arguments, ctx)
	}
	if callee == "JsonLower" || callee == "JsonLoadStructured" {
		return c.checkJSONStructuredBuiltinCallExpr(scope, callee, typeArguments, arguments, ctx)
	}
	if callee == "JsonParse" || callee == "JsonStringify" || callee == "JsonLoad" || callee == "JsonSave" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		return c.checkJSONBuiltinCallExpr(scope, callee, arguments, ctx)
	}
	if callee == "FileReadText" || callee == "FileWriteText" || callee == "FileReadBytes" || callee == "FileWriteBytes" || callee == "FileReadLines" || callee == "FileWriteLines" || callee == "FileExists" || callee == "FileDelete" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		return c.checkFileBuiltinCallExpr(scope, callee, arguments, ctx)
	}
	if callee == "PathJoin" || callee == "PathBaseName" || callee == "PathExtension" || callee == "PathStem" || callee == "PathParent" || callee == "PathClean" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		return c.checkPathBuiltinCallExpr(scope, callee, arguments, ctx)
	}
	if callee == "DirectoryList" || callee == "DirectoryMake" || callee == "DirectoryMakeAll" || callee == "DirectoryRemoveAll" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		return c.checkDirectoryBuiltinCallExpr(scope, callee, arguments, ctx)
	}
	if callee == "CsvRead" || callee == "CsvReadRows" || callee == "CsvReadTable" || callee == "CsvReadMatrix" || callee == "CsvWrite" || callee == "CsvWriteRows" || callee == "CsvWriteTable" || callee == "CsvWriteMatrix" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		return c.checkCSVBuiltinCallExpr(scope, callee, arguments, ctx)
	}
	if callee == "ZipListEntries" || callee == "ZipExtractAll" || callee == "ZipCreateFromFiles" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		return c.checkArchiveBuiltinCallExpr(scope, callee, arguments, ctx)
	}
	if callee == "GzipCompressBytes" || callee == "GzipDecompressBytes" || callee == "GzipCompressFile" || callee == "GzipDecompressFile" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		return c.checkCompressionBuiltinCallExpr(scope, callee, arguments, ctx)
	}
	if callee == "HashSha256Bytes" || callee == "HashSha256Text" || callee == "HashSha256File" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		return c.checkHashBuiltinCallExpr(scope, callee, arguments, ctx)
	}
	if callee == "RegexIsMatch" || callee == "RegexFindAll" || callee == "RegexReplaceAll" || callee == "RegexSplit" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		return c.checkRegexBuiltinCallExpr(scope, callee, arguments, ctx)
	}
	if callee == "TimeNowIso8601" || callee == "TimeParseIso8601" || callee == "TimeFormatIso8601" || callee == "TimeUnixSecondsNow" || callee == "TimeFormatUnixSecond" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		return c.checkTimeBuiltinCallExpr(scope, callee, arguments, ctx)
	}
	if callee == "Append" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'Append' does not accept type arguments")
		}
		return c.checkAppendBuiltinCallExpr(scope, callee, arguments, ctx)
	}
	if callee == "Step" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'Step' does not accept type arguments")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'Step' expects 1 argument, got %d", len(arguments))
		}
		flowType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if flowType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if !flowType.ValueType.IsFlowInstance {
			return ExprType{}, fmt.Errorf("function 'Step' argument 1 expects FlowInstance<T>, got %s", flowType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeVoid}}, nil
	}
	if callee == "Active" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'Active' does not accept type arguments")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'Active' expects 1 argument, got %d", len(arguments))
		}
		flowType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if flowType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if !flowType.ValueType.IsFlowInstance {
			return ExprType{}, fmt.Errorf("function 'Active' argument 1 expects FlowInstance<T>, got %s", flowType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeString}}, nil
	}
	if callee == "Result" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'Result' does not accept type arguments")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'Result' expects 1 argument, got %d", len(arguments))
		}
		flowType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if flowType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if !flowType.ValueType.IsFlowInstance {
			return ExprType{}, fmt.Errorf("function 'Result' argument 1 expects FlowInstance<T>, got %s", flowType.ValueType)
		}
		if flowType.ValueType.FlowResult == nil {
			return ExprType{}, fmt.Errorf("internal error: missing flow result type metadata")
		}
		return ExprType{ValueType: *flowType.ValueType.FlowResult, Fallible: true}, nil
	}
	if callee == "Complete" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'Complete' does not accept type arguments")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'Complete' expects 1 argument, got %d", len(arguments))
		}
		flowType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if flowType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if !flowType.ValueType.IsFlowInstance {
			return ExprType{}, fmt.Errorf("function 'Complete' argument 1 expects FlowInstance<T>, got %s", flowType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeBool}}, nil
	}
	if callee == "StateHistory" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'StateHistory' does not accept type arguments")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'StateHistory' expects 1 argument, got %d", len(arguments))
		}
		flowType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if flowType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if !flowType.ValueType.IsFlowInstance {
			return ExprType{}, fmt.Errorf("function 'StateHistory' argument 1 expects FlowInstance<T>, got %s", flowType.ValueType)
		}
		return ExprType{ValueType: withArrayDepth(Type{Base: BaseTypeString}, 1)}, nil
	}
	if callee == "ResumeTarget" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'ResumeTarget' does not accept type arguments")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'ResumeTarget' expects 1 argument, got %d", len(arguments))
		}
		flowType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if flowType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if !flowType.ValueType.IsFlowInstance {
			return ExprType{}, fmt.Errorf("function 'ResumeTarget' argument 1 expects FlowInstance<T>, got %s", flowType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeString}}, nil
	}
	if callee == "BoardSnapshot" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'BoardSnapshot' does not accept type arguments")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'BoardSnapshot' expects 1 argument, got %d", len(arguments))
		}
		flowType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if flowType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if !flowType.ValueType.IsFlowInstance {
			return ExprType{}, fmt.Errorf("function 'BoardSnapshot' argument 1 expects FlowInstance<T>, got %s", flowType.ValueType)
		}
		if flowType.ValueType.FlowIdentity == "" {
			return ExprType{}, fmt.Errorf("BoardSnapshot currently requires a concrete flow instance; flow identity metadata was not preserved for this value")
		}
		flowSignature, ok := c.flows[flowType.ValueType.FlowIdentity]
		if !ok {
			return ExprType{}, fmt.Errorf("internal error: missing flow metadata for '%s'", flowType.ValueType.FlowIdentity)
		}
		if flowSignature.boardType == nil {
			return ExprType{}, fmt.Errorf("flow %s has no board; BoardSnapshot requires a flow with a declared board", flowType.ValueType.FlowIdentity)
		}
		return ExprType{ValueType: *flowSignature.boardType, Fallible: true}, nil
	}
	if callee == "UIText" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'UIText' does not accept type arguments")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'UIText' expects 1 argument, got %d", len(arguments))
		}
		contentType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if contentType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if contentType.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function 'UIText' argument 1 expects String, got %s", contentType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeUI}}, nil
	}
	if callee == "UIButton" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'UIButton' does not accept type arguments")
		}
		if len(arguments) != 3 {
			return ExprType{}, fmt.Errorf("function 'UIButton' expects 3 arguments, got %d", len(arguments))
		}
		labelType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if labelType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if labelType.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function 'UIButton' argument 1 expects String, got %s", labelType.ValueType)
		}
		eventType, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if eventType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if eventType.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function 'UIButton' argument 2 expects String, got %s", eventType.ValueType)
		}
		enabledType, err := c.checkExpr(scope, arguments[2], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if enabledType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if enabledType.ValueType != (Type{Base: BaseTypeBool}) {
			return ExprType{}, fmt.Errorf("function 'UIButton' argument 3 expects Bool, got %s", enabledType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeUI}}, nil
	}
	if callee == "FormatFloat" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'FormatFloat' does not accept type arguments")
		}
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function 'FormatFloat' expects 2 arguments, got %d", len(arguments))
		}
		valueType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if valueType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if valueType.ValueType != (Type{Base: BaseTypeFloat}) {
			return ExprType{}, fmt.Errorf("function 'FormatFloat' argument 1 expects Float, got %s", valueType.ValueType)
		}
		precisionType, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if precisionType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if precisionType.ValueType != (Type{Base: BaseTypeInt}) {
			return ExprType{}, fmt.Errorf("function 'FormatFloat' argument 2 expects Int, got %s", precisionType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeString}}, nil
	}
	if callee == "Float" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'Float' does not accept type arguments")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'Float' expects 1 argument, got %d", len(arguments))
		}
		valueType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if valueType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if valueType.ValueType.Base != BaseTypeInt || valueType.ValueType.Name != "" || valueType.ValueType.IsArray || valueType.ValueType.IsVector || valueType.ValueType.IsMatrix || valueType.ValueType.IsFunction || valueType.ValueType.IsFlowInstance {
			return ExprType{}, fmt.Errorf("function 'Float' argument 1 expects Int, got %s", valueType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeFloat, Dimension: valueType.ValueType.Dimension}}, nil
	}
	if callee == "StringFrom" {
		if len(typeArguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'String.From' requires an explicit type argument, e.g. String.From<Int>(value)")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'String.From' expects 1 argument, got %d", len(arguments))
		}
		targetType, err := c.resolveNonReturnType(typeArguments[0])
		if err != nil {
			return ExprType{}, err
		}
		valueType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if valueType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		switch targetType {
		case Type{Base: BaseTypeInt}, Type{Base: BaseTypeFloat}, Type{Base: BaseTypeBool}, Type{Base: BaseTypeString}:
			if valueType.ValueType != targetType {
				return ExprType{}, fmt.Errorf("function 'String.From' argument 1 expects %s, got %s", targetType, valueType.ValueType)
			}
			return ExprType{ValueType: Type{Base: BaseTypeString}}, nil
		default:
			return ExprType{}, fmt.Errorf("String.From<T> supports Int, Float, Bool, and String in M0")
		}
	}
	if callee == "ToString" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'ToString' does not accept type arguments")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'ToString' expects 1 argument, got %d", len(arguments))
		}
		valueType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if valueType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		switch valueType.ValueType {
		case Type{Base: BaseTypeInt}, Type{Base: BaseTypeFloat}, Type{Base: BaseTypeBool}:
			return ExprType{ValueType: Type{Base: BaseTypeString}}, nil
		default:
			return ExprType{}, fmt.Errorf("function 'ToString' argument 1 expects Int, Float, or Bool, got %s", valueType.ValueType)
		}
	}
	if callee == "Contains" || callee == "StartsWith" || callee == "EndsWith" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
		}
		textType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if textType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if textType.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects String, got %s", callee, textType.ValueType)
		}
		partType, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if partType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if partType.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function '%s' argument 2 expects String, got %s", callee, partType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeBool}}, nil
	}
	if callee == "Trim" || callee == "Lower" || callee == "Upper" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function '%s' expects 1 argument, got %d", callee, len(arguments))
		}
		textType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if textType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if textType.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects String, got %s", callee, textType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeString}}, nil
	}
	if callee == "Join" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'Join' does not accept type arguments")
		}
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function 'Join' expects 2 arguments, got %d", len(arguments))
		}
		partsType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if partsType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if partsType.ValueType != withArrayDepth(Type{Base: BaseTypeString}, 1) {
			return ExprType{}, fmt.Errorf("function 'Join' argument 1 expects String[], got %s", partsType.ValueType)
		}
		sepType, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if sepType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if sepType.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function 'Join' argument 2 expects String, got %s", sepType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeString}}, nil
	}
	if callee == "StringByteLength" || callee == "StringRuneCount" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function '%s' expects 1 argument, got %d", callee, len(arguments))
		}
		textType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if textType.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects String, got %s", callee, textType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeInt}}, nil
	}
	if callee == "StringTrim" || callee == "StringEscapeJSON" || callee == "StringQuoteJSON" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function '%s' expects 1 argument, got %d", callee, len(arguments))
		}
		textType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if textType.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects String, got %s", callee, textType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeString}}, nil
	}
	if callee == "StringSplitLines" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'StringSplitLines' does not accept type arguments")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'StringSplitLines' expects 1 argument, got %d", len(arguments))
		}
		textType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if textType.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function 'StringSplitLines' argument 1 expects String, got %s", textType.ValueType)
		}
		return ExprType{ValueType: withArrayDepth(Type{Base: BaseTypeString}, 1)}, nil
	}
	if callee == "StringContains" || callee == "StringStartsWith" || callee == "StringEndsWith" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
		}
		for i := 0; i < 2; i++ {
			tp, err := c.checkExpr(scope, arguments[i], ctx)
			if err != nil {
				return ExprType{}, err
			}
			if tp.ValueType != (Type{Base: BaseTypeString}) {
				return ExprType{}, fmt.Errorf("function '%s' argument %d expects String, got %s", callee, i+1, tp.ValueType)
			}
		}
		return ExprType{ValueType: Type{Base: BaseTypeBool}}, nil
	}
	if callee == "StringConcat" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'StringConcat' does not accept type arguments")
		}
		return c.checkSingleStringArrayArgBuiltin(scope, "StringConcat", arguments, ctx, ExprType{ValueType: Type{Base: BaseTypeString}})
	}
	if callee == "StringJoin" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'StringJoin' does not accept type arguments")
		}
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function 'StringJoin' expects 2 arguments, got %d", len(arguments))
		}
		partsType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		sepType, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if partsType.ValueType != withArrayDepth(Type{Base: BaseTypeString}, 1) {
			return ExprType{}, fmt.Errorf("function 'StringJoin' argument 1 expects String[], got %s", partsType.ValueType)
		}
		if sepType.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function 'StringJoin' argument 2 expects String, got %s", sepType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeString}}, nil
	}
	if callee == "StringReplaceAll" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'StringReplaceAll' does not accept type arguments")
		}
		if len(arguments) != 3 {
			return ExprType{}, fmt.Errorf("function 'StringReplaceAll' expects 3 arguments, got %d", len(arguments))
		}
		for i := 0; i < 3; i++ {
			tp, err := c.checkExpr(scope, arguments[i], ctx)
			if err != nil {
				return ExprType{}, err
			}
			if tp.ValueType != (Type{Base: BaseTypeString}) {
				return ExprType{}, fmt.Errorf("function 'StringReplaceAll' argument %d expects String, got %s", i+1, tp.ValueType)
			}
		}
		return ExprType{ValueType: Type{Base: BaseTypeString}}, nil
	}
	if callee == "MarkdownEscapeText" || callee == "MarkdownEscapeTableCell" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function '%s' expects 1 argument, got %d", callee, len(arguments))
		}
		t, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if t.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects String, got %s", callee, t.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeString}}, nil
	}
	if callee == "MarkdownH1" || callee == "MarkdownH2" || callee == "MarkdownH3" || callee == "MarkdownParagraph" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function '%s' expects 1 argument, got %d", callee, len(arguments))
		}
		t, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if t.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects String, got %s", callee, t.ValueType)
		}
		return ExprType{ValueType: withArrayDepth(Type{Base: BaseTypeString}, 1)}, nil
	}
	if callee == "MarkdownBlank" || callee == "MarkdownHorizontalRule" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		if len(arguments) != 0 {
			return ExprType{}, fmt.Errorf("function '%s' expects 0 arguments, got %d", callee, len(arguments))
		}
		return ExprType{ValueType: withArrayDepth(Type{Base: BaseTypeString}, 1)}, nil
	}
	if callee == "MarkdownBullets" || callee == "MarkdownNumbered" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function '%s' expects 1 argument, got %d", callee, len(arguments))
		}
		t, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if t.ValueType != withArrayDepth(Type{Base: BaseTypeString}, 1) {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects String[], got %s", callee, t.ValueType)
		}
		return ExprType{ValueType: withArrayDepth(Type{Base: BaseTypeString}, 1)}, nil
	}
	if callee == "MarkdownCodeBlock" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'MarkdownCodeBlock' does not accept type arguments")
		}
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function 'MarkdownCodeBlock' expects 2 arguments, got %d", len(arguments))
		}
		a, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		b, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if a.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function 'MarkdownCodeBlock' argument 1 expects String, got %s", a.ValueType)
		}
		if b.ValueType != withArrayDepth(Type{Base: BaseTypeString}, 1) {
			return ExprType{}, fmt.Errorf("function 'MarkdownCodeBlock' argument 2 expects String[], got %s", b.ValueType)
		}
		return ExprType{ValueType: withArrayDepth(Type{Base: BaseTypeString}, 1)}, nil
	}
	if callee == "MarkdownCallout" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'MarkdownCallout' does not accept type arguments")
		}
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function 'MarkdownCallout' expects 2 arguments, got %d", len(arguments))
		}
		a, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		b, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if a.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function 'MarkdownCallout' argument 1 expects String, got %s", a.ValueType)
		}
		if b.ValueType != withArrayDepth(Type{Base: BaseTypeString}, 1) {
			return ExprType{}, fmt.Errorf("function 'MarkdownCallout' argument 2 expects String[], got %s. Markdown.Callout expects a String[] list of lines; use [\"text\"] for a single-line callout", b.ValueType)
		}
		return ExprType{ValueType: withArrayDepth(Type{Base: BaseTypeString}, 1)}, nil
	}
	if callee == "MarkdownImage" || callee == "MarkdownFigure" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
		}
		for i := 0; i < 2; i++ {
			t, err := c.checkExpr(scope, arguments[i], ctx)
			if err != nil {
				return ExprType{}, err
			}
			if t.ValueType != (Type{Base: BaseTypeString}) {
				return ExprType{}, fmt.Errorf("function '%s' argument %d expects String, got %s", callee, i+1, t.ValueType)
			}
		}
		return ExprType{ValueType: withArrayDepth(Type{Base: BaseTypeString}, 1)}, nil
	}
	if callee == "MarkdownKeyValueTable" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'MarkdownKeyValueTable' does not accept type arguments")
		}
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function 'MarkdownKeyValueTable' expects 2 arguments, got %d. Markdown.KeyValueTable expects two arrays: keys and values. Example: Markdown.KeyValueTable([\"sampleRate\"], [\"2000\"])", len(arguments))
		}
		for i := 0; i < 2; i++ {
			t, err := c.checkExpr(scope, arguments[i], ctx)
			if err != nil {
				return ExprType{}, err
			}
			if t.ValueType != withArrayDepth(Type{Base: BaseTypeString}, 1) {
				return ExprType{}, fmt.Errorf("function 'MarkdownKeyValueTable' argument %d expects String[], got %s", i+1, t.ValueType)
			}
		}
		return ExprType{ValueType: withArrayDepth(Type{Base: BaseTypeString}, 1)}, nil
	}
	if callee == "MarkdownReport" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'MarkdownReport' does not accept type arguments")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'MarkdownReport' expects 1 argument, got %d", len(arguments))
		}
		t, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if t.ValueType != withArrayDepth(Type{Base: BaseTypeString}, 2) {
			return ExprType{}, fmt.Errorf("function 'MarkdownReport' argument 1 expects String[][], got %s", t.ValueType)
		}
		return ExprType{ValueType: withArrayDepth(Type{Base: BaseTypeString}, 1)}, nil
	}
	if callee == "MarkdownSection" || callee == "MarkdownSubsection" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
		}
		t1, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		t2, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if t1.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects String, got %s", callee, t1.ValueType)
		}
		if t2.ValueType != withArrayDepth(Type{Base: BaseTypeString}, 2) {
			return ExprType{}, fmt.Errorf("function '%s' argument 2 expects String[][], got %s", callee, t2.ValueType)
		}
		return ExprType{ValueType: withArrayDepth(Type{Base: BaseTypeString}, 1)}, nil
	}
	if callee == "MarkdownTable" || callee == "MarkdownTableWithColumns" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		if (callee == "MarkdownTable" && len(arguments) != 1) || (callee == "MarkdownTableWithColumns" && len(arguments) != 2) {
			return ExprType{}, fmt.Errorf("function '%s' arity mismatch", callee)
		}
		t, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if t.ValueType.Name == "" {
			if t.ValueType == withArrayDepth(Type{Base: BaseTypeString}, 2) {
				return ExprType{}, fmt.Errorf("function '%s' argument 1 expects a columnar record of String[] fields, got %s. For row-major data, convert to a columnar record first or use Markdown.KeyValueTable for scalar metadata", callee, t.ValueType)
			}
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects a columnar record of String[] fields, got %s", callee, t.ValueType)
		}
		if callee == "MarkdownTableWithColumns" {
			c2, err := c.checkExpr(scope, arguments[1], ctx)
			if err != nil {
				return ExprType{}, err
			}
			if c2.ValueType != withArrayDepth(Type{Base: BaseTypeString}, 1) {
				return ExprType{}, fmt.Errorf("function 'MarkdownTableWithColumns' argument 2 expects String[], got %s", c2.ValueType)
			}
		}
		return ExprType{ValueType: withArrayDepth(Type{Base: BaseTypeString}, 1)}, nil
	}
	if callee == "Idx" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'Idx' does not accept type arguments")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'Idx' expects 1 argument, got %d", len(arguments))
		}
		nameType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if nameType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if nameType.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function 'Idx' argument 1 expects String, got %s", nameType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeIndex}}, nil
	}
	if callee == "PrometheusMatMul" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'PrometheusMatMul' does not accept type arguments")
		}
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function 'PrometheusMatMul' expects 2 arguments, got %d", len(arguments))
		}
		leftType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if leftType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		rightType, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if rightType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		floatMatrix := Type{Base: BaseTypeFloat, IsMatrix: true}
		if leftType.ValueType != floatMatrix {
			return ExprType{}, fmt.Errorf("function 'PrometheusMatMul' argument 1 expects Matrix<Float>, got %s", leftType.ValueType)
		}
		if rightType.ValueType != floatMatrix {
			return ExprType{}, fmt.Errorf("function 'PrometheusMatMul' argument 2 expects Matrix<Float>, got %s", rightType.ValueType)
		}
		return ExprType{ValueType: floatMatrix}, nil
	}

	if callee == "Matrix.tabulate" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'Matrix.tabulate' does not accept type arguments")
		}
		if len(arguments) != 3 {
			return ExprType{}, fmt.Errorf("function 'Matrix.tabulate' expects 3 arguments, got %d", len(arguments))
		}
		for idx := 0; idx < 2; idx++ {
			dimType, err := c.checkExpr(scope, arguments[idx], ctx)
			if err != nil {
				return ExprType{}, err
			}
			if dimType.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if dimType.ValueType != (Type{Base: BaseTypeInt}) {
				return ExprType{}, fmt.Errorf("function 'Matrix.tabulate' argument %d expects Int, got %s", idx+1, dimType.ValueType)
			}
		}
		callbackType, err := c.checkExpr(scope, arguments[2], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if callbackType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if !callbackType.ValueType.IsFunction {
			return ExprType{}, fmt.Errorf("function 'Matrix.tabulate' argument 3 expects function (Int, Int) -> T, got %s", callbackType.ValueType)
		}
		signature, ok := c.functionTypes[callbackType.ValueType.FunctionSignature]
		if !ok {
			return ExprType{}, fmt.Errorf("internal error: missing function type metadata for %s", callbackType.ValueType.FunctionSignature)
		}
		if len(signature.parameters) != 2 || signature.parameters[0] != (Type{Base: BaseTypeInt}) || signature.parameters[1] != (Type{Base: BaseTypeInt}) {
			return ExprType{}, fmt.Errorf("function 'Matrix.tabulate' argument 3 expects function (Int, Int) -> T, got %s", callbackType.ValueType)
		}
		if !isNumericBaseType(signature.returnType.Base) || signature.returnType.IsArray || signature.returnType.IsVector || signature.returnType.IsMatrix || signature.returnType.IsFunction || signature.returnType.IsFlowInstance {
			return ExprType{}, fmt.Errorf("function 'Matrix.tabulate' callback must return numeric scalar value, got %s", signature.returnType)
		}
		return ExprType{ValueType: Type{Base: signature.returnType.Base, Dimension: signature.returnType.Dimension, IsMatrix: true}}, nil
	}
	if callee == "Matrix.zeros" {
		if len(typeArguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'Matrix.zeros' expects 1 type arguments, got %d", len(typeArguments))
		}
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function 'Matrix.zeros' expects 2 arguments, got %d", len(arguments))
		}
		for idx := 0; idx < 2; idx++ {
			dimType, err := c.checkExpr(scope, arguments[idx], ctx)
			if err != nil {
				return ExprType{}, err
			}
			if dimType.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if dimType.ValueType != (Type{Base: BaseTypeInt}) {
				return ExprType{}, fmt.Errorf("function 'Matrix.zeros' argument %d expects Int, got %s", idx+1, dimType.ValueType)
			}
		}
		elemType, err := c.resolveType(typeArguments[0], false)
		if err != nil {
			return ExprType{}, err
		}
		if !isNumericBaseType(elemType.Base) || elemType.IsArray || elemType.IsVector || elemType.IsMatrix {
			return ExprType{}, fmt.Errorf("function 'Matrix.zeros' type argument must be numeric scalar, got %s", elemType)
		}
		return ExprType{ValueType: Type{Base: elemType.Base, Dimension: elemType.Dimension, IsMatrix: true}}, nil
	}
	if callee == "Matrix.fill" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'Matrix.fill' does not accept type arguments")
		}
		if len(arguments) != 3 {
			return ExprType{}, fmt.Errorf("function 'Matrix.fill' expects 3 arguments, got %d", len(arguments))
		}
		for idx := 0; idx < 2; idx++ {
			dimType, err := c.checkExpr(scope, arguments[idx], ctx)
			if err != nil {
				return ExprType{}, err
			}
			if dimType.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if dimType.ValueType != (Type{Base: BaseTypeInt}) {
				return ExprType{}, fmt.Errorf("function 'Matrix.fill' argument %d expects Int, got %s", idx+1, dimType.ValueType)
			}
		}
		valueType, err := c.checkExpr(scope, arguments[2], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if valueType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if !isNumericBaseType(valueType.ValueType.Base) || valueType.ValueType.IsArray || valueType.ValueType.IsVector || valueType.ValueType.IsMatrix {
			return ExprType{}, fmt.Errorf("function 'Matrix.fill' argument 3 expects numeric scalar, got %s", valueType.ValueType)
		}
		return ExprType{ValueType: Type{Base: valueType.ValueType.Base, Dimension: valueType.ValueType.Dimension, IsMatrix: true}}, nil
	}
	if callee == "Matrix.identity" {
		if len(typeArguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'Matrix.identity' expects 1 type arguments, got %d", len(typeArguments))
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'Matrix.identity' expects 1 argument, got %d", len(arguments))
		}
		sizeType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if sizeType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if sizeType.ValueType != (Type{Base: BaseTypeInt}) {
			return ExprType{}, fmt.Errorf("function 'Matrix.identity' argument 1 expects Int, got %s", sizeType.ValueType)
		}
		elemType, err := c.resolveType(typeArguments[0], false)
		if err != nil {
			return ExprType{}, err
		}
		if !isNumericBaseType(elemType.Base) || elemType.IsArray || elemType.IsVector || elemType.IsMatrix {
			return ExprType{}, fmt.Errorf("function 'Matrix.identity' type argument must be numeric scalar, got %s", elemType)
		}
		return ExprType{ValueType: Type{Base: elemType.Base, Dimension: elemType.Dimension, IsMatrix: true}}, nil
	}
	if callee == "EinMul" || callee == "EinAdd" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		if len(arguments) != 6 {
			return ExprType{}, fmt.Errorf("function '%s' expects 6 arguments, got %d", callee, len(arguments))
		}
		leftType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if leftType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		rightType, err := c.checkExpr(scope, arguments[3], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if rightType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if !leftType.ValueType.IsMatrix {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects Matrix, got %s", callee, leftType.ValueType)
		}
		if !rightType.ValueType.IsMatrix {
			return ExprType{}, fmt.Errorf("function '%s' argument 4 expects Matrix, got %s", callee, rightType.ValueType)
		}
		for idx := 1; idx <= 2; idx++ {
			indexType, err := c.checkExpr(scope, arguments[idx], ctx)
			if err != nil {
				return ExprType{}, err
			}
			if indexType.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if indexType.ValueType != (Type{Base: BaseTypeIndex}) {
				return ExprType{}, fmt.Errorf("function '%s' argument %d expects Index, got %s", callee, idx+1, indexType.ValueType)
			}
		}
		for idx := 4; idx <= 5; idx++ {
			indexType, err := c.checkExpr(scope, arguments[idx], ctx)
			if err != nil {
				return ExprType{}, err
			}
			if indexType.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if indexType.ValueType != (Type{Base: BaseTypeIndex}) {
				return ExprType{}, fmt.Errorf("function '%s' argument %d expects Index, got %s", callee, idx+1, indexType.ValueType)
			}
		}

		leftScalar := Type{Base: leftType.ValueType.Base, Dimension: leftType.ValueType.Dimension}
		rightScalar := Type{Base: rightType.ValueType.Base, Dimension: rightType.ValueType.Dimension}
		op := "*"
		if callee == "EinAdd" {
			op = "+"
		}
		resultScalar, err := c.checkBinaryExpr(op, leftScalar, rightScalar)
		if err != nil {
			return ExprType{}, err
		}
		return ExprType{ValueType: Type{Base: resultScalar.Base, Dimension: resultScalar.Dimension, IsMatrix: true}}, nil
	}
	if callee == "Trace" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'Trace' does not accept type arguments")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'Trace' expects 1 argument, got %d", len(arguments))
		}
		matrixType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if matrixType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if !matrixType.ValueType.IsMatrix {
			return ExprType{}, fmt.Errorf("function 'Trace' argument 1 expects Matrix, got %s", matrixType.ValueType)
		}
		return ExprType{ValueType: Type{Base: matrixType.ValueType.Base, Dimension: matrixType.ValueType.Dimension}}, nil
	}
	if callee == "Grad" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'Grad' does not accept type arguments")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'Grad' expects 1 argument, got %d", len(arguments))
		}
		operandType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if operandType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if !isNumericBaseType(operandType.ValueType.Base) || operandType.ValueType.IsArray {
			return ExprType{}, fmt.Errorf("function 'Grad' argument 1 expects numeric Scalar or Vector, got %s", operandType.ValueType)
		}
		if operandType.ValueType.IsVector {
			return ExprType{ValueType: Type{Base: operandType.ValueType.Base, Dimension: operandType.ValueType.Dimension, IsMatrix: true}}, nil
		}
		if operandType.ValueType.IsMatrix {
			return ExprType{}, fmt.Errorf("function 'Grad' argument 1 expects numeric Scalar or Vector, got %s", operandType.ValueType)
		}
		return ExprType{ValueType: Type{Base: operandType.ValueType.Base, Dimension: operandType.ValueType.Dimension, IsVector: true}}, nil
	}
	if callee == "Div" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'Div' does not accept type arguments")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'Div' expects 1 argument, got %d", len(arguments))
		}
		operandType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if operandType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if !isNumericBaseType(operandType.ValueType.Base) || operandType.ValueType.IsArray || (!operandType.ValueType.IsVector && !operandType.ValueType.IsMatrix) {
			return ExprType{}, fmt.Errorf("function 'Div' argument 1 expects numeric Vector or Matrix, got %s", operandType.ValueType)
		}
		if operandType.ValueType.IsMatrix {
			return ExprType{ValueType: Type{Base: operandType.ValueType.Base, Dimension: operandType.ValueType.Dimension, IsVector: true}}, nil
		}
		return ExprType{ValueType: Type{Base: operandType.ValueType.Base, Dimension: operandType.ValueType.Dimension}}, nil
	}
	if callee == "SymGrad" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'SymGrad' does not accept type arguments")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'SymGrad' expects 1 argument, got %d", len(arguments))
		}
		operandType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if operandType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if !isNumericBaseType(operandType.ValueType.Base) || operandType.ValueType.IsArray || !operandType.ValueType.IsVector {
			return ExprType{}, fmt.Errorf("function 'SymGrad' argument 1 expects numeric Vector, got %s", operandType.ValueType)
		}
		return ExprType{ValueType: Type{Base: operandType.ValueType.Base, Dimension: operandType.ValueType.Dimension, IsMatrix: true}}, nil
	}
	if callee == "UIColumn" || callee == "UIRow" || callee == "UICanvas" || callee == "UIGrid" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function '%s' expects 1 argument, got %d", callee, len(arguments))
		}
		childrenType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if childrenType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if childrenType.ValueType != withArrayDepth(Type{Base: BaseTypeUI}, 1) {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects UI[], got %s", callee, childrenType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeUI}}, nil
	}

	if callee == "UISpacer" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'UISpacer' does not accept type arguments")
		}
		if len(arguments) != 0 {
			return ExprType{}, fmt.Errorf("function 'UISpacer' expects 0 arguments, got %d", len(arguments))
		}
		return ExprType{ValueType: Type{Base: BaseTypeUI}}, nil
	}
	if callee == "UIPlaceAbsolute" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'UIPlaceAbsolute' does not accept type arguments")
		}
		if len(arguments) != 6 {
			return ExprType{}, fmt.Errorf("function 'UIPlaceAbsolute' expects 6 arguments, got %d", len(arguments))
		}
		for idx := 0; idx < 4; idx++ {
			valueType, err := c.checkExpr(scope, arguments[idx], ctx)
			if err != nil {
				return ExprType{}, err
			}
			if valueType.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if valueType.ValueType != (Type{Base: BaseTypeFloat, Dimension: uiPixelDimension}) && valueType.ValueType != (Type{Base: BaseTypeFloat}) {
				return ExprType{}, fmt.Errorf("function 'UIPlaceAbsolute' argument %d expects Float<px>, got %s", idx+1, valueType.ValueType)
			}
		}
		childType, err := c.checkExpr(scope, arguments[4], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if childType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if childType.ValueType != (Type{Base: BaseTypeUI}) {
			return ExprType{}, fmt.Errorf("function 'UIPlaceAbsolute' argument 5 expects UI, got %s", childType.ValueType)
		}
		zType, err := c.checkExpr(scope, arguments[5], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if zType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if zType.ValueType != (Type{Base: BaseTypeInt}) {
			return ExprType{}, fmt.Errorf("function 'UIPlaceAbsolute' argument 6 expects Int, got %s", zType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeUI}}, nil
	}
	if callee == "UIPlaceAnchored" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'UIPlaceAnchored' does not accept type arguments")
		}
		if len(arguments) != 6 {
			return ExprType{}, fmt.Errorf("function 'UIPlaceAnchored' expects 6 arguments, got %d", len(arguments))
		}
		for idx := 0; idx < 4; idx++ {
			valueType, err := c.checkExpr(scope, arguments[idx], ctx)
			if err != nil {
				return ExprType{}, err
			}
			if valueType.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if valueType.ValueType != (Type{Base: BaseTypeFloat, Dimension: uiAnchorDimension}) && valueType.ValueType != (Type{Base: BaseTypeFloat}) {
				return ExprType{}, fmt.Errorf("function 'UIPlaceAnchored' argument %d expects Float<ui>, got %s", idx+1, valueType.ValueType)
			}
		}
		childType, err := c.checkExpr(scope, arguments[4], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if childType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if childType.ValueType != (Type{Base: BaseTypeUI}) {
			return ExprType{}, fmt.Errorf("function 'UIPlaceAnchored' argument 5 expects UI, got %s", childType.ValueType)
		}
		zType, err := c.checkExpr(scope, arguments[5], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if zType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if zType.ValueType != (Type{Base: BaseTypeInt}) {
			return ExprType{}, fmt.Errorf("function 'UIPlaceAnchored' argument 6 expects Int, got %s", zType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeUI}}, nil
	}
	if callee == "UIMount" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'UIMount' does not accept type arguments")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'UIMount' expects 1 argument, got %d", len(arguments))
		}
		rootType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if rootType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if rootType.ValueType != (Type{Base: BaseTypeUI}) {
			return ExprType{}, fmt.Errorf("function 'UIMount' argument 1 expects UI, got %s", rootType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeInt}}, nil
	}
	if callee == "UIPatch" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'UIPatch' does not accept type arguments")
		}
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function 'UIPatch' expects 2 arguments, got %d", len(arguments))
		}
		mountType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if mountType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if mountType.ValueType != (Type{Base: BaseTypeInt}) {
			return ExprType{}, fmt.Errorf("function 'UIPatch' argument 1 expects Int, got %s", mountType.ValueType)
		}
		rootType, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if rootType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if rootType.ValueType != (Type{Base: BaseTypeUI}) {
			return ExprType{}, fmt.Errorf("function 'UIPatch' argument 2 expects UI, got %s", rootType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeInt}}, nil
	}
	if callee == "UIUnmount" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'UIUnmount' does not accept type arguments")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'UIUnmount' expects 1 argument, got %d", len(arguments))
		}
		mountType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if mountType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if mountType.ValueType != (Type{Base: BaseTypeInt}) {
			return ExprType{}, fmt.Errorf("function 'UIUnmount' argument 1 expects Int, got %s", mountType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeInt}}, nil
	}
	if callee == "UIEmit" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'UIEmit' does not accept type arguments")
		}
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function 'UIEmit' expects 2 arguments, got %d", len(arguments))
		}
		mountType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if mountType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if mountType.ValueType != (Type{Base: BaseTypeInt}) {
			return ExprType{}, fmt.Errorf("function 'UIEmit' argument 1 expects Int, got %s", mountType.ValueType)
		}
		eventType, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if eventType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if eventType.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function 'UIEmit' argument 2 expects String, got %s", eventType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeInt}, Fallible: true}, nil
	}
	if callee == "UIDrainEvents" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'UIDrainEvents' does not accept type arguments")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'UIDrainEvents' expects 1 argument, got %d", len(arguments))
		}
		mountType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if mountType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if mountType.ValueType != (Type{Base: BaseTypeInt}) {
			return ExprType{}, fmt.Errorf("function 'UIDrainEvents' argument 1 expects Int, got %s", mountType.ValueType)
		}
		return ExprType{ValueType: withArrayDepth(Type{Base: BaseTypeString}, 1)}, nil
	}
	if callee == "UISignature" {
		if len(typeArguments) > 0 {
			return ExprType{}, fmt.Errorf("function 'UISignature' does not accept type arguments")
		}
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'UISignature' expects 1 argument, got %d", len(arguments))
		}
		rootType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if rootType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if rootType.ValueType != (Type{Base: BaseTypeUI}) {
			return ExprType{}, fmt.Errorf("function 'UISignature' argument 1 expects UI, got %s", rootType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeString}}, nil
	}
	if callee == "fft" {
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function 'fft' expects 1 argument, got %d", len(arguments))
		}
		argumentType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if argumentType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if argumentType.ValueType != withArrayDepth(Type{Base: BaseTypeComplex}, 1) {
			return ExprType{}, fmt.Errorf("function 'fft' argument 1 expects Complex[], got %s", argumentType.ValueType)
		}
		return ExprType{ValueType: withArrayDepth(Type{Base: BaseTypeComplex}, 1), Fallible: true}, nil
	}

	if len(typeArguments) > 0 {
		return ExprType{}, fmt.Errorf("function '%s' does not accept type arguments", callee)
	}

	if callee == "Pi" || callee == "E" {
		if len(arguments) != 0 {
			return ExprType{}, fmt.Errorf("function '%s' expects 0 arguments, got %d", callee, len(arguments))
		}
		return ExprType{ValueType: Type{Base: BaseTypeFloat}}, nil
	}
	if callee == "I" {
		if len(arguments) != 0 {
			return ExprType{}, fmt.Errorf("function '%s' expects 0 arguments, got %d", callee, len(arguments))
		}
		return ExprType{ValueType: Type{Base: BaseTypeComplex}}, nil
	}
	if callee == "Complex" {
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
		}
		for idx, argument := range arguments {
			argumentType, err := c.checkExpr(scope, argument, ctx)
			if err != nil {
				return ExprType{}, err
			}
			if argumentType.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if !isRealNumericScalar(argumentType.ValueType) {
				return ExprType{}, fmt.Errorf("function '%s' argument %d expects Int or Float, got %s", callee, idx+1, argumentType.ValueType)
			}
			if !argumentType.ValueType.Dimension.IsDimensionless() {
				return ExprType{}, fmt.Errorf("%s requires dimensionless input", callee)
			}
		}
		return ExprType{ValueType: Type{Base: BaseTypeComplex}}, nil
	}
	if callee == "ComplexPolar" {
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
		}
		for idx, argument := range arguments {
			argumentType, err := c.checkExpr(scope, argument, ctx)
			if err != nil {
				return ExprType{}, err
			}
			if argumentType.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if !isRealNumericScalar(argumentType.ValueType) {
				return ExprType{}, fmt.Errorf("function '%s' argument %d expects Int or Float, got %s", callee, idx+1, argumentType.ValueType)
			}
			if !argumentType.ValueType.Dimension.IsDimensionless() {
				return ExprType{}, fmt.Errorf("%s requires dimensionless input", callee)
			}
		}
		return ExprType{ValueType: Type{Base: BaseTypeComplex}}, nil
	}
	if callee == "Require" {
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function 'Require' expects 2 arguments, got %d", len(arguments))
		}
		conditionType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if conditionType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if conditionType.ValueType != (Type{Base: BaseTypeBool}) {
			return ExprType{}, fmt.Errorf("function 'Require' argument 1 expects Bool, got %s", conditionType.ValueType)
		}
		messageType, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if messageType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if messageType.ValueType != (Type{Base: BaseTypeString}) {
			return ExprType{}, fmt.Errorf("function 'Require' argument 2 expects String, got %s", messageType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeVoid}}, nil
	}
	if callee == "Atan2" {
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
		}
		for idx, argument := range arguments {
			argumentType, err := c.checkExpr(scope, argument, ctx)
			if err != nil {
				return ExprType{}, err
			}
			if argumentType.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if !isRealNumericScalar(argumentType.ValueType) {
				return ExprType{}, fmt.Errorf("function '%s' argument %d expects Int or Float, got %s", callee, idx+1, argumentType.ValueType)
			}
			if !argumentType.ValueType.Dimension.IsDimensionless() {
				return ExprType{}, fmt.Errorf("%s requires dimensionless input", callee)
			}
		}
		return ExprType{ValueType: Type{Base: BaseTypeFloat}}, nil
	}
	if callee == "Pow" {
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
		}
		for idx, argument := range arguments {
			argumentType, err := c.checkExpr(scope, argument, ctx)
			if err != nil {
				return ExprType{}, err
			}
			if argumentType.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if !isRealNumericScalar(argumentType.ValueType) {
				return ExprType{}, fmt.Errorf("function '%s' argument %d expects Int or Float, got %s", callee, idx+1, argumentType.ValueType)
			}
			if !argumentType.ValueType.Dimension.IsDimensionless() {
				return ExprType{}, fmt.Errorf("%s requires dimensionless input", callee)
			}
		}
		return ExprType{ValueType: Type{Base: BaseTypeFloat}}, nil
	}

	if len(arguments) != 1 {
		return ExprType{}, fmt.Errorf("function '%s' expects 1 argument, got %d", callee, len(arguments))
	}

	argumentType, err := c.checkExpr(scope, arguments[0], ctx)
	if err != nil {
		return ExprType{}, err
	}
	if argumentType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}

	switch callee {
	case "Print":
		if isPrintableType(argumentType.ValueType) {
			return ExprType{ValueType: Type{Base: BaseTypeInt}}, nil
		}
		return ExprType{}, fmt.Errorf("function 'Print' argument 1 has unsupported type %s", argumentType.ValueType)
	case "Len":
		if argumentType.ValueType == (Type{Base: BaseTypeString}) {
			return ExprType{ValueType: Type{Base: BaseTypeInt}}, nil
		}
		if argumentType.ValueType == (Type{Base: BaseTypeBytes}) {
			return ExprType{ValueType: Type{Base: BaseTypeInt}}, nil
		}
		if !argumentType.ValueType.IsArray {
			return ExprType{}, fmt.Errorf("function 'Len' argument 1 expects String, Bytes, or array type, got %s", argumentType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeInt}}, nil
	case "Abs":
		if isRealNumericScalar(argumentType.ValueType) {
			return ExprType{ValueType: argumentType.ValueType}, nil
		}
		if isComplexScalar(argumentType.ValueType) {
			return ExprType{ValueType: Type{Base: BaseTypeFloat}}, nil
		}
		return ExprType{}, fmt.Errorf("function 'Abs' argument 1 expects Int, Float, or Complex, got %s", argumentType.ValueType)
	case "Real", "Imag", "Arg":
		if !isComplexScalar(argumentType.ValueType) {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects Complex, got %s", callee, argumentType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeFloat}}, nil
	case "Conj":
		if !isComplexScalar(argumentType.ValueType) {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects Complex, got %s", callee, argumentType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeComplex}}, nil
	case "Sqrt":
		if !isRealNumericScalar(argumentType.ValueType) {
			return ExprType{}, fmt.Errorf("function 'Sqrt' argument 1 expects Int or Float, got %s", argumentType.ValueType)
		}
		if !argumentType.ValueType.Dimension.CanSqrt() {
			return ExprType{}, fmt.Errorf("Sqrt requires even dimension exponents")
		}
		return ExprType{ValueType: Type{Base: BaseTypeFloat, Dimension: argumentType.ValueType.Dimension.Sqrt()}}, nil
	case "Sin", "Cos", "Tan":
		if !isRealNumericScalar(argumentType.ValueType) {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects Int or Float, got %s", callee, argumentType.ValueType)
		}
		if !argumentType.ValueType.Dimension.IsDimensionless() {
			return ExprType{}, fmt.Errorf("%s requires dimensionless input", callee)
		}
		return ExprType{ValueType: Type{Base: BaseTypeFloat}}, nil
	case "Asin", "Acos":
		if !isRealNumericScalar(argumentType.ValueType) {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects Int or Float, got %s", callee, argumentType.ValueType)
		}
		if !argumentType.ValueType.Dimension.IsDimensionless() {
			return ExprType{}, fmt.Errorf("%s requires dimensionless input", callee)
		}
		return ExprType{ValueType: Type{Base: BaseTypeFloat}}, nil
	case "Exp", "Ln":
		if isComplexScalar(argumentType.ValueType) {
			return ExprType{ValueType: Type{Base: BaseTypeComplex}}, nil
		}
		if !isRealNumericScalar(argumentType.ValueType) {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects Int, Float, or Complex, got %s", callee, argumentType.ValueType)
		}
		if !argumentType.ValueType.Dimension.IsDimensionless() {
			return ExprType{}, fmt.Errorf("%s requires dimensionless input", callee)
		}
		return ExprType{ValueType: Type{Base: BaseTypeFloat}}, nil
	case "Atan", "Log10", "Sinh", "Cosh", "Tanh":
		if !isRealNumericScalar(argumentType.ValueType) {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects Int or Float, got %s", callee, argumentType.ValueType)
		}
		if !argumentType.ValueType.Dimension.IsDimensionless() {
			return ExprType{}, fmt.Errorf("%s requires dimensionless input", callee)
		}
		return ExprType{ValueType: Type{Base: BaseTypeFloat}}, nil
	case "FloorToInt", "CeilToInt", "RoundToInt":
		if argumentType.ValueType.Base != BaseTypeFloat || argumentType.ValueType.IsArray || argumentType.ValueType.IsVector || argumentType.ValueType.IsMatrix {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects Float, got %s", callee, argumentType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeInt}}, nil
	case "BaseValue":
		if argumentType.ValueType.Base != BaseTypeFloat || argumentType.ValueType.IsArray || argumentType.ValueType.IsVector || argumentType.ValueType.IsMatrix {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects Float, got %s", callee, argumentType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeFloat}}, nil
	default:
		return ExprType{}, fmt.Errorf("unsupported built-in function '%s'", callee)
	}
}

func (c checker) checkXlsxBuiltinCallExpr(scope *scope, callee string, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	intType := Type{Base: BaseTypeInt}
	stringType := Type{Base: BaseTypeString}
	floatType := Type{Base: BaseTypeFloat}
	switch callee {
	case "XlsxCreateWorkbook":
		if len(arguments) != 0 {
			return ExprType{}, fmt.Errorf("function '%s' expects 0 arguments, got %d", callee, len(arguments))
		}
		return ExprType{ValueType: intType}, nil
	case "XlsxAddSheet":
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
		}
		workbookType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if workbookType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if workbookType.ValueType != intType {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects Int, got %s", callee, workbookType.ValueType)
		}
		sheetType, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if sheetType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if sheetType.ValueType != stringType {
			return ExprType{}, fmt.Errorf("function '%s' argument 2 expects String, got %s", callee, sheetType.ValueType)
		}
		return ExprType{ValueType: intType, Fallible: true}, nil
	case "XlsxSetCellString", "XlsxSetCellFloat":
		if len(arguments) != 4 {
			return ExprType{}, fmt.Errorf("function '%s' expects 4 arguments, got %d", callee, len(arguments))
		}
		workbookType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if workbookType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if workbookType.ValueType != intType {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects Int, got %s", callee, workbookType.ValueType)
		}
		sheetType, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if sheetType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if sheetType.ValueType != stringType {
			return ExprType{}, fmt.Errorf("function '%s' argument 2 expects String, got %s", callee, sheetType.ValueType)
		}
		cellType, err := c.checkExpr(scope, arguments[2], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if cellType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if cellType.ValueType != stringType {
			return ExprType{}, fmt.Errorf("function '%s' argument 3 expects String, got %s", callee, cellType.ValueType)
		}
		valueType, err := c.checkExpr(scope, arguments[3], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if valueType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if callee == "XlsxSetCellString" && valueType.ValueType != stringType {
			return ExprType{}, fmt.Errorf("function '%s' argument 4 expects String, got %s", callee, valueType.ValueType)
		}
		if callee == "XlsxSetCellFloat" && valueType.ValueType != floatType && valueType.ValueType != intType {
			return ExprType{}, fmt.Errorf("function '%s' argument 4 expects Int or Float, got %s", callee, valueType.ValueType)
		}
		return ExprType{ValueType: intType, Fallible: true}, nil
	case "XlsxSaveWorkbook":
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
		}
		workbookType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if workbookType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if workbookType.ValueType != intType {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects Int, got %s", callee, workbookType.ValueType)
		}
		pathType, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if pathType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if pathType.ValueType != stringType {
			return ExprType{}, fmt.Errorf("function '%s' argument 2 expects String, got %s", callee, pathType.ValueType)
		}
		if pathLiteral, ok := arguments[1].(ast.StringLiteralExpr); ok && !strings.HasSuffix(pathLiteral.Value, ".xlsx") {
			return ExprType{}, fmt.Errorf("XlsxSaveWorkbook path must end with .xlsx")
		}
		return ExprType{ValueType: intType, Fallible: true}, nil
	default:
		return ExprType{}, fmt.Errorf("unsupported built-in function '%s'", callee)
	}
}

func (c checker) checkJSONBuiltinCallExpr(scope *scope, callee string, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	stringType := Type{Base: BaseTypeString}
	switch callee {
	case "JsonNormalize", "JsonParse", "JsonStringify", "JsonLoad":
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function '%s' expects 1 argument, got %d", callee, len(arguments))
		}
		textType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if textType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if textType.ValueType != stringType {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects String, got %s", callee, textType.ValueType)
		}
		return ExprType{ValueType: stringType, Fallible: true}, nil
	case "JsonSave":
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
		}
		for idx := 0; idx < 2; idx++ {
			currentType, err := c.checkExpr(scope, arguments[idx], ctx)
			if err != nil {
				return ExprType{}, err
			}
			if currentType.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if currentType.ValueType != stringType {
				return ExprType{}, fmt.Errorf("function '%s' argument %d expects String, got %s", callee, idx+1, currentType.ValueType)
			}
		}
		return ExprType{ValueType: Type{Base: BaseTypeInt}, Fallible: true}, nil
	default:
		return ExprType{}, fmt.Errorf("unsupported built-in function '%s'", callee)
	}
}

func (c checker) checkImageBuiltinCallExpr(scope *scope, callee string, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	intType := Type{Base: BaseTypeInt}
	pixelIntType := Type{Base: BaseTypeInt, Dimension: uiPixelDimension}
	stringType := Type{Base: BaseTypeString}
	if callee == "ImageLoad" {
		return c.checkSingleStringArgBuiltin(scope, callee, arguments, ctx, ExprType{ValueType: intType, Fallible: true})
	}
	if callee == "ImageSave" {
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
		}
		handleType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if handleType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if handleType.ValueType != intType {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects Int, got %s", callee, handleType.ValueType)
		}
		pathType, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if pathType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if pathType.ValueType != stringType {
			return ExprType{}, fmt.Errorf("function '%s' argument 2 expects String, got %s", callee, pathType.ValueType)
		}
		return ExprType{ValueType: intType, Fallible: true}, nil
	}
	if callee == "ImageWidth" || callee == "ImageHeight" {
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function '%s' expects 1 argument, got %d", callee, len(arguments))
		}
		handleType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if handleType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if handleType.ValueType != intType {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects Int, got %s", callee, handleType.ValueType)
		}
		return ExprType{ValueType: pixelIntType}, nil
	}
	if callee == "ImageFormat" {
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function '%s' expects 1 argument, got %d", callee, len(arguments))
		}
		handleType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if handleType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if handleType.ValueType != intType {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects Int, got %s", callee, handleType.ValueType)
		}
		return ExprType{ValueType: stringType}, nil
	}
	return ExprType{}, fmt.Errorf("unsupported built-in function '%s'", callee)
}

func (c checker) checkPdfBuiltinCallExpr(scope *scope, callee string, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	intType := Type{Base: BaseTypeInt}
	pixelIntType := Type{Base: BaseTypeInt, Dimension: uiPixelDimension}
	stringType := Type{Base: BaseTypeString}

	switch callee {
	case "PdfNewPage":
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
		}
		for idx := 0; idx < 2; idx++ {
			dimType, err := c.checkExpr(scope, arguments[idx], ctx)
			if err != nil {
				return ExprType{}, err
			}
			if dimType.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if dimType.ValueType != pixelIntType {
				return ExprType{}, fmt.Errorf("function '%s' argument %d expects %s, got %s", callee, idx+1, pixelIntType, dimType.ValueType)
			}
		}
		return ExprType{ValueType: intType, Fallible: true}, nil
	case "PdfDrawText":
		if len(arguments) != 4 {
			return ExprType{}, fmt.Errorf("function '%s' expects 4 arguments, got %d", callee, len(arguments))
		}
		for idx, expected := range []Type{intType, pixelIntType, pixelIntType, stringType} {
			current, err := c.checkExpr(scope, arguments[idx], ctx)
			if err != nil {
				return ExprType{}, err
			}
			if current.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if current.ValueType != expected {
				return ExprType{}, fmt.Errorf("function '%s' argument %d expects %s, got %s", callee, idx+1, expected, current.ValueType)
			}
		}
		return ExprType{ValueType: intType, Fallible: true}, nil
	case "PdfDrawTextStyled":
		if len(arguments) != 8 {
			return ExprType{}, fmt.Errorf("function '%s' expects 8 arguments, got %d", callee, len(arguments))
		}
		expected := []Type{intType, pixelIntType, pixelIntType, stringType, pixelIntType, intType, intType, intType}
		for idx, want := range expected {
			current, err := c.checkExpr(scope, arguments[idx], ctx)
			if err != nil {
				return ExprType{}, err
			}
			if current.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if current.ValueType != want {
				return ExprType{}, fmt.Errorf("function '%s' argument %d expects %s, got %s", callee, idx+1, want, current.ValueType)
			}
		}
		return ExprType{ValueType: intType, Fallible: true}, nil
	case "PdfDrawImage":
		if len(arguments) != 4 {
			return ExprType{}, fmt.Errorf("function '%s' expects 4 arguments, got %d", callee, len(arguments))
		}
		for idx, expected := range []Type{intType, intType, pixelIntType, pixelIntType} {
			current, err := c.checkExpr(scope, arguments[idx], ctx)
			if err != nil {
				return ExprType{}, err
			}
			if current.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if current.ValueType != expected {
				return ExprType{}, fmt.Errorf("function '%s' argument %d expects %s, got %s", callee, idx+1, expected, current.ValueType)
			}
		}
		return ExprType{ValueType: intType, Fallible: true}, nil
	case "PdfDrawImageSized":
		if len(arguments) != 6 {
			return ExprType{}, fmt.Errorf("function '%s' expects 6 arguments, got %d", callee, len(arguments))
		}
		for idx, expected := range []Type{intType, intType, pixelIntType, pixelIntType, pixelIntType, pixelIntType} {
			current, err := c.checkExpr(scope, arguments[idx], ctx)
			if err != nil {
				return ExprType{}, err
			}
			if current.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if current.ValueType != expected {
				return ExprType{}, fmt.Errorf("function '%s' argument %d expects %s, got %s", callee, idx+1, expected, current.ValueType)
			}
		}
		return ExprType{ValueType: intType, Fallible: true}, nil
	case "PdfSave":
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
		}
		for idx, expected := range []Type{intType, stringType} {
			current, err := c.checkExpr(scope, arguments[idx], ctx)
			if err != nil {
				return ExprType{}, err
			}
			if current.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if current.ValueType != expected {
				return ExprType{}, fmt.Errorf("function '%s' argument %d expects %s, got %s", callee, idx+1, expected, current.ValueType)
			}
		}
		return ExprType{ValueType: intType, Fallible: true}, nil
	default:
		return ExprType{}, fmt.Errorf("unsupported built-in function '%s'", callee)
	}
}

func (c checker) checkJSONStructuredBuiltinCallExpr(scope *scope, callee string, typeArguments []ast.TypeRef, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	stringType := Type{Base: BaseTypeString}
	if len(typeArguments) != 1 {
		return ExprType{}, fmt.Errorf("function '%s' expects 1 type argument, got %d", callee, len(typeArguments))
	}
	targetValueType, err := c.resolveNonReturnType(typeArguments[0])
	if err != nil {
		return ExprType{}, fmt.Errorf("function '%s' type argument is invalid: %w", callee, err)
	}
	if targetValueType != (Type{Name: "JsonRawGraph"}) && targetValueType != (Type{Name: "IO.JsonRawGraph"}) {
		return ExprType{}, fmt.Errorf("function '%s' type argument expects JsonRawGraph, got %s", callee, targetValueType)
	}
	if len(arguments) != 1 {
		return ExprType{}, fmt.Errorf("function '%s' expects 1 argument, got %d", callee, len(arguments))
	}
	inputType, err := c.checkExpr(scope, arguments[0], ctx)
	if err != nil {
		return ExprType{}, err
	}
	if inputType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if inputType.ValueType != stringType {
		return ExprType{}, fmt.Errorf("function '%s' argument 1 expects String, got %s", callee, inputType.ValueType)
	}
	return ExprType{ValueType: targetValueType, Fallible: true}, nil
}

func (c checker) checkFileBuiltinCallExpr(scope *scope, callee string, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	stringType := Type{Base: BaseTypeString}
	bytesType := Type{Base: BaseTypeBytes}
	if callee == "FileReadText" {
		return c.checkSingleStringArgBuiltin(scope, callee, arguments, ctx, ExprType{ValueType: stringType, Fallible: true})
	}
	if callee == "FileReadBytes" {
		return c.checkSingleStringArgBuiltin(scope, callee, arguments, ctx, ExprType{ValueType: bytesType, Fallible: true})
	}
	if callee == "FileReadLines" {
		return c.checkSingleStringArgBuiltin(scope, callee, arguments, ctx, ExprType{ValueType: withArrayDepth(Type{Base: BaseTypeString}, 1), Fallible: true})
	}
	if callee == "FileExists" {
		return c.checkSingleStringArgBuiltin(scope, callee, arguments, ctx, ExprType{ValueType: Type{Base: BaseTypeBool}})
	}
	if callee == "FileDelete" {
		return c.checkSingleStringArgBuiltin(scope, callee, arguments, ctx, ExprType{ValueType: Type{Base: BaseTypeInt}, Fallible: true})
	}
	if callee == "FileWriteText" || callee == "FileWriteBytes" || callee == "FileWriteLines" {
		if len(arguments) != 2 {
			return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
		}
		pathType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if pathType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if pathType.ValueType != stringType {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects String, got %s", callee, pathType.ValueType)
		}
		valueType, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if valueType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if callee == "FileWriteText" && valueType.ValueType != stringType {
			return ExprType{}, fmt.Errorf("function '%s' argument 2 expects String, got %s", callee, valueType.ValueType)
		}
		if callee == "FileWriteBytes" && valueType.ValueType != bytesType {
			return ExprType{}, fmt.Errorf("function '%s' argument 2 expects Bytes, got %s", callee, valueType.ValueType)
		}
		if callee == "FileWriteLines" && valueType.ValueType != (withArrayDepth(Type{Base: BaseTypeString}, 1)) {
			return ExprType{}, fmt.Errorf("function '%s' argument 2 expects String[], got %s", callee, valueType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeInt}, Fallible: true}, nil
	}
	return ExprType{}, fmt.Errorf("unsupported built-in function '%s'", callee)
}

func (c checker) checkPathBuiltinCallExpr(scope *scope, callee string, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	if callee == "PathJoin" {
		return c.checkSingleStringArrayArgBuiltin(scope, callee, arguments, ctx, ExprType{ValueType: Type{Base: BaseTypeString}})
	}
	return c.checkSingleStringArgBuiltin(scope, callee, arguments, ctx, ExprType{ValueType: Type{Base: BaseTypeString}})
}

func (c checker) checkDirectoryBuiltinCallExpr(scope *scope, callee string, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	switch callee {
	case "DirectoryList":
		return c.checkSingleStringArgBuiltin(scope, callee, arguments, ctx, ExprType{ValueType: withArrayDepth(Type{Base: BaseTypeString}, 1), Fallible: true})
	case "DirectoryMake", "DirectoryMakeAll", "DirectoryRemoveAll":
		return c.checkSingleStringArgBuiltin(scope, callee, arguments, ctx, ExprType{ValueType: Type{Base: BaseTypeInt}, Fallible: true})
	default:
		return ExprType{}, fmt.Errorf("unsupported built-in function '%s'", callee)
	}
}

func (c checker) checkCSVBuiltinCallExpr(scope *scope, callee string, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	stringType := Type{Base: BaseTypeString}
	stringMatrixType := withArrayDepth(stringType, 2)
	if callee == "CsvRead" || callee == "CsvReadRows" {
		return c.checkSingleStringArgBuiltin(scope, callee, arguments, ctx, ExprType{ValueType: stringMatrixType, Fallible: true})
	}
	if callee == "CsvReadTable" {
		return c.checkSingleStringArgBuiltin(scope, callee, arguments, ctx, ExprType{ValueType: Type{Name: "Csv.Table"}, Fallible: true})
	}
	if callee == "CsvReadMatrix" {
		return c.checkSingleStringArgBuiltin(scope, callee, arguments, ctx, ExprType{ValueType: withArrayDepth(Type{Base: BaseTypeFloat}, 2), Fallible: true})
	}
	if len(arguments) != 2 {
		return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
	}
	pathType, err := c.checkExpr(scope, arguments[0], ctx)
	if err != nil {
		return ExprType{}, err
	}
	if pathType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if pathType.ValueType != stringType {
		return ExprType{}, fmt.Errorf("function '%s' argument 1 expects String, got %s", callee, pathType.ValueType)
	}
	rowsType, err := c.checkExpr(scope, arguments[1], ctx)
	if err != nil {
		return ExprType{}, err
	}
	if rowsType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	expectedRowsType := stringMatrixType
	if callee == "CsvWriteMatrix" {
		expectedRowsType = withArrayDepth(Type{Base: BaseTypeFloat}, 2)
	}
	if callee == "CsvWriteTable" {
		expectedRowsType = Type{Name: "Csv.Table"}
	}
	if rowsType.ValueType != expectedRowsType {
		return ExprType{}, fmt.Errorf("function '%s' argument 2 expects %s, got %s", callee, expectedRowsType, rowsType.ValueType)
	}
	return ExprType{ValueType: Type{Base: BaseTypeInt}, Fallible: true}, nil
}

func (c checker) checkArchiveBuiltinCallExpr(scope *scope, callee string, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	if callee == "ZipListEntries" {
		return c.checkSingleStringArgBuiltin(scope, callee, arguments, ctx, ExprType{ValueType: withArrayDepth(Type{Base: BaseTypeString}, 1), Fallible: true})
	}
	if callee == "ZipExtractAll" {
		return c.checkTwoStringArgBuiltin(scope, callee, arguments, ctx, ExprType{ValueType: Type{Base: BaseTypeInt}, Fallible: true})
	}
	if len(arguments) != 2 {
		return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
	}
	pathType, err := c.checkExpr(scope, arguments[0], ctx)
	if err != nil {
		return ExprType{}, err
	}
	if pathType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if pathType.ValueType != (Type{Base: BaseTypeString}) {
		return ExprType{}, fmt.Errorf("function '%s' argument 1 expects String, got %s", callee, pathType.ValueType)
	}
	pathsType, err := c.checkExpr(scope, arguments[1], ctx)
	if err != nil {
		return ExprType{}, err
	}
	if pathsType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if pathsType.ValueType != withArrayDepth(Type{Base: BaseTypeString}, 1) {
		return ExprType{}, fmt.Errorf("function '%s' argument 2 expects String[], got %s", callee, pathsType.ValueType)
	}
	return ExprType{ValueType: Type{Base: BaseTypeInt}, Fallible: true}, nil
}

func (c checker) checkCompressionBuiltinCallExpr(scope *scope, callee string, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	bytesType := Type{Base: BaseTypeBytes}
	if callee == "GzipCompressBytes" || callee == "GzipDecompressBytes" {
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function '%s' expects 1 argument, got %d", callee, len(arguments))
		}
		argType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if argType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if argType.ValueType != bytesType {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects Bytes, got %s", callee, argType.ValueType)
		}
		return ExprType{ValueType: bytesType, Fallible: true}, nil
	}
	return c.checkTwoStringArgBuiltin(scope, callee, arguments, ctx, ExprType{ValueType: Type{Base: BaseTypeInt}, Fallible: true})
}

func (c checker) checkHashBuiltinCallExpr(scope *scope, callee string, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	if callee == "HashSha256Bytes" {
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function '%s' expects 1 argument, got %d", callee, len(arguments))
		}
		argType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if argType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if argType.ValueType != (Type{Base: BaseTypeBytes}) {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects Bytes, got %s", callee, argType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeString}}, nil
	}
	if callee == "HashSha256Text" {
		return c.checkSingleStringArgBuiltin(scope, callee, arguments, ctx, ExprType{ValueType: Type{Base: BaseTypeString}})
	}
	return c.checkSingleStringArgBuiltin(scope, callee, arguments, ctx, ExprType{ValueType: Type{Base: BaseTypeString}, Fallible: true})
}

func (c checker) checkRegexBuiltinCallExpr(scope *scope, callee string, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	if callee == "RegexReplaceAll" {
		if len(arguments) != 3 {
			return ExprType{}, fmt.Errorf("function '%s' expects 3 arguments, got %d", callee, len(arguments))
		}
		for idx := 0; idx < 3; idx++ {
			argType, err := c.checkExpr(scope, arguments[idx], ctx)
			if err != nil {
				return ExprType{}, err
			}
			if argType.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if argType.ValueType != (Type{Base: BaseTypeString}) {
				return ExprType{}, fmt.Errorf("function '%s' argument %d expects String, got %s", callee, idx+1, argType.ValueType)
			}
		}
		return ExprType{ValueType: Type{Base: BaseTypeString}, Fallible: true}, nil
	}
	if err := c.requireTwoStringArgs(scope, callee, arguments, ctx); err != nil {
		return ExprType{}, err
	}
	if callee == "RegexIsMatch" {
		return ExprType{ValueType: Type{Base: BaseTypeBool}, Fallible: true}, nil
	}
	if callee == "RegexFindAll" || callee == "RegexSplit" {
		return ExprType{ValueType: withArrayDepth(Type{Base: BaseTypeString}, 1), Fallible: true}, nil
	}
	return ExprType{}, fmt.Errorf("unsupported built-in function '%s'", callee)
}

func (c checker) checkTimeBuiltinCallExpr(scope *scope, callee string, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	if callee == "TimeNowIso8601" {
		if len(arguments) != 0 {
			return ExprType{}, fmt.Errorf("function '%s' expects 0 arguments, got %d", callee, len(arguments))
		}
		return ExprType{ValueType: Type{Base: BaseTypeString}}, nil
	}
	if callee == "TimeUnixSecondsNow" {
		if len(arguments) != 0 {
			return ExprType{}, fmt.Errorf("function '%s' expects 0 arguments, got %d", callee, len(arguments))
		}
		return ExprType{ValueType: Type{Base: BaseTypeInt}}, nil
	}
	if callee == "TimeFormatUnixSecond" {
		if len(arguments) != 1 {
			return ExprType{}, fmt.Errorf("function '%s' expects 1 argument, got %d", callee, len(arguments))
		}
		argType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if argType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if argType.ValueType != (Type{Base: BaseTypeInt}) {
			return ExprType{}, fmt.Errorf("function '%s' argument 1 expects Int, got %s", callee, argType.ValueType)
		}
		return ExprType{ValueType: Type{Base: BaseTypeString}, Fallible: true}, nil
	}
	return c.checkSingleStringArgBuiltin(scope, callee, arguments, ctx, ExprType{ValueType: Type{Base: BaseTypeString}, Fallible: true})
}

func (c checker) checkTwoStringArgBuiltin(scope *scope, callee string, arguments []ast.Expr, ctx functionContext, result ExprType) (ExprType, error) {
	if err := c.requireTwoStringArgs(scope, callee, arguments, ctx); err != nil {
		return ExprType{}, err
	}
	return result, nil
}

func (c checker) requireTwoStringArgs(scope *scope, callee string, arguments []ast.Expr, ctx functionContext) error {
	if len(arguments) != 2 {
		return fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
	}
	for idx := 0; idx < 2; idx++ {
		argType, err := c.checkExpr(scope, arguments[idx], ctx)
		if err != nil {
			return err
		}
		if argType.Fallible {
			return fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if argType.ValueType != (Type{Base: BaseTypeString}) {
			return fmt.Errorf("function '%s' argument %d expects String, got %s", callee, idx+1, argType.ValueType)
		}
	}
	return nil
}

func (c checker) checkSingleStringArgBuiltin(scope *scope, callee string, arguments []ast.Expr, ctx functionContext, result ExprType) (ExprType, error) {
	if len(arguments) != 1 {
		return ExprType{}, fmt.Errorf("function '%s' expects 1 argument, got %d", callee, len(arguments))
	}
	argType, err := c.checkExpr(scope, arguments[0], ctx)
	if err != nil {
		return ExprType{}, err
	}
	if argType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if argType.ValueType != (Type{Base: BaseTypeString}) {
		return ExprType{}, fmt.Errorf("function '%s' argument 1 expects String, got %s", callee, argType.ValueType)
	}
	return result, nil
}

func (c checker) checkSingleStringArrayArgBuiltin(scope *scope, callee string, arguments []ast.Expr, ctx functionContext, result ExprType) (ExprType, error) {
	if len(arguments) != 1 {
		return ExprType{}, fmt.Errorf("function '%s' expects 1 argument, got %d", callee, len(arguments))
	}
	argType, err := c.checkExpr(scope, arguments[0], ctx)
	if err != nil {
		return ExprType{}, err
	}
	if argType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if argType.ValueType != withArrayDepth(Type{Base: BaseTypeString}, 1) {
		return ExprType{}, fmt.Errorf("function '%s' argument 1 expects String[], got %s", callee, argType.ValueType)
	}
	return result, nil
}

func (c checker) checkLoadOctagonBuiltinCallExpr(scope *scope, callee string, typeArguments []ast.TypeRef, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	if len(typeArguments) != 1 {
		return ExprType{}, fmt.Errorf("function '%s' expects 1 type argument, got %d", callee, len(typeArguments))
	}
	if len(arguments) != 1 {
		return ExprType{}, fmt.Errorf("function '%s' expects 1 argument, got %d", callee, len(arguments))
	}

	expectedType, err := c.resolveNonReturnType(typeArguments[0])
	if err != nil {
		return ExprType{}, err
	}
	if !isOctagonRepresentableType(expectedType) {
		return ExprType{}, fmt.Errorf("function 'LoadOctagon' type argument expects .octagon-representable type, got %s", expectedType)
	}

	pathType, err := c.checkExpr(scope, arguments[0], ctx)
	if err != nil {
		return ExprType{}, err
	}
	if pathType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if pathType.ValueType != (Type{Base: BaseTypeString}) {
		return ExprType{}, fmt.Errorf("function 'LoadOctagon' argument 1 expects String, got %s", pathType.ValueType)
	}
	if pathLiteral, ok := arguments[0].(ast.StringLiteralExpr); ok && !strings.HasSuffix(pathLiteral.Value, ".octagon") {
		return ExprType{}, fmt.Errorf("LoadOctagon path must end with .octagon")
	}

	return ExprType{ValueType: expectedType, Fallible: true}, nil
}

func (c checker) checkWriteOctagonBuiltinCallExpr(scope *scope, callee string, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	if len(arguments) != 2 {
		return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
	}

	pathType, err := c.checkExpr(scope, arguments[0], ctx)
	if err != nil {
		return ExprType{}, err
	}
	if pathType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if pathType.ValueType != (Type{Base: BaseTypeString}) {
		return ExprType{}, fmt.Errorf("function 'WriteOctagon' argument 1 expects String, got %s", pathType.ValueType)
	}
	if pathLiteral, ok := arguments[0].(ast.StringLiteralExpr); ok && !strings.HasSuffix(pathLiteral.Value, ".octagon") {
		return ExprType{}, fmt.Errorf("WriteOctagon path must end with .octagon")
	}

	valueType, err := c.checkExpr(scope, arguments[1], ctx)
	if err != nil {
		return ExprType{}, err
	}
	if valueType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if !isOctagonRepresentableType(valueType.ValueType) {
		return ExprType{}, fmt.Errorf("function 'WriteOctagon' argument 2 expects .octagon-representable value, got %s", valueType.ValueType)
	}
	return ExprType{ValueType: Type{Base: BaseTypeInt}}, nil
}

func (c checker) checkArtifactBuiltinCallExpr(scope *scope, callee string, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	delegate := ""
	switch callee {
	case "ArtifactWriteText":
		delegate = "FileWriteText"
	case "ArtifactWriteLines", "ArtifactWriteMarkdown":
		delegate = "FileWriteLines"
	case "ArtifactWriteCsv":
		delegate = "CsvWrite"
	case "ArtifactWriteJson":
		delegate = "JsonSave"
	case "ArtifactWriteOctagon":
		delegate = "WriteOctagon"
	default:
		return ExprType{}, fmt.Errorf("runtime invariant violation: unknown artifact builtin '%s'", callee)
	}
	_, err := c.checkBuiltinCallExpr(scope, delegate, nil, arguments, ctx)
	if err != nil {
		return ExprType{}, err
	}
	return ExprType{ValueType: Type{Base: BaseTypeVoid}}, nil
}

func isOctagonRepresentableType(valueType Type) bool {
	if valueType.IsVector || valueType.IsMatrix || valueType.IsFunction {
		return false
	}
	if valueType.IsArray {
		elementType := valueType
		elementType = peelArrayType(elementType)
		return isOctagonRepresentableType(elementType)
	}
	if valueType.Name != "" {
		return true
	}
	switch valueType.Base {
	case BaseTypeInt, BaseTypeFloat, BaseTypeBool, BaseTypeString, BaseTypeBytes:
		return true
	default:
		return false
	}
}

func (c checker) checkAppendBuiltinCallExpr(scope *scope, callee string, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	if len(arguments) != 2 {
		return ExprType{}, fmt.Errorf("function '%s' expects 2 arguments, got %d", callee, len(arguments))
	}

	arrayType, err := c.checkExpr(scope, arguments[0], ctx)
	if err != nil {
		return ExprType{}, err
	}
	if arrayType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if !arrayType.ValueType.IsArray {
		return ExprType{}, fmt.Errorf("Append requires array as first argument")
	}

	elementType, err := c.checkExpr(scope, arguments[1], ctx)
	if err != nil {
		return ExprType{}, err
	}
	if elementType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}

	expectedElementType := arrayType.ValueType
	expectedElementType = peelArrayType(expectedElementType)
	if elementType.ValueType != expectedElementType {
		return ExprType{}, fmt.Errorf("Append element type must match array element type: expected %s, got %s", expectedElementType, elementType.ValueType)
	}

	return ExprType{ValueType: arrayType.ValueType}, nil
}

func isPrintableType(valueType Type) bool {
	if valueType.Name != "" {
		return !valueType.IsArray && !valueType.IsVector && !valueType.IsMatrix
	}
	if valueType.IsArray {
		return valueType.Base == BaseTypeInt || valueType.Base == BaseTypeFloat || valueType.Base == BaseTypeComplex || valueType.Base == BaseTypeBool || valueType.Base == BaseTypeString
	}
	if valueType.IsVector || valueType.IsMatrix {
		return valueType.Base == BaseTypeInt || valueType.Base == BaseTypeFloat || valueType.Base == BaseTypeComplex
	}
	return valueType.Base == BaseTypeInt || valueType.Base == BaseTypeFloat || valueType.Base == BaseTypeComplex || valueType.Base == BaseTypeBool || valueType.Base == BaseTypeString || valueType.Base == BaseTypeBytes || valueType.Base == BaseTypeError
}

func (c checker) checkPlotBuiltinCallExpr(scope *scope, callee string, arguments []ast.Expr, ctx functionContext) (ExprType, error) {
	intType := Type{Base: BaseTypeInt}
	stringType := Type{Base: BaseTypeString}
	pixelIntType := Type{Base: BaseTypeInt, Dimension: uiPixelDimension}
	if callee == "PlotLine" || callee == "PlotScatter" {
		if len(arguments) != 3 {
			return ExprType{}, fmt.Errorf("function '%s' expects 3 arguments, got %d", callee, len(arguments))
		}
		xType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if xType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if err := requirePlotArrayType(callee, 1, xType.ValueType); err != nil {
			return ExprType{}, err
		}
		yType, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if yType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if err := requirePlotArrayType(callee, 2, yType.ValueType); err != nil {
			return ExprType{}, err
		}
		pathType, err := c.checkExpr(scope, arguments[2], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if pathType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if pathType.ValueType != stringType {
			return ExprType{}, fmt.Errorf("function '%s' argument 3 expects String, got %s", callee, pathType.ValueType)
		}
		return ExprType{ValueType: intType}, nil
	}
	if callee == "PlotRenderLine" || callee == "PlotRenderScatter" {
		if len(arguments) != 9 {
			return ExprType{}, fmt.Errorf("function '%s' expects 9 arguments, got %d", callee, len(arguments))
		}
		xType, err := c.checkExpr(scope, arguments[0], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if xType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if err := requirePlotArrayType(callee, 1, xType.ValueType); err != nil {
			return ExprType{}, err
		}
		yType, err := c.checkExpr(scope, arguments[1], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if yType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if err := requirePlotArrayType(callee, 2, yType.ValueType); err != nil {
			return ExprType{}, err
		}
		for idx, expected := range []Type{stringType, pixelIntType, pixelIntType, stringType, stringType, stringType, stringType} {
			current, err := c.checkExpr(scope, arguments[idx+2], ctx)
			if err != nil {
				return ExprType{}, err
			}
			if current.Fallible {
				return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
			}
			if current.ValueType != expected {
				return ExprType{}, fmt.Errorf("function '%s' argument %d expects %s, got %s", callee, idx+3, expected, current.ValueType)
			}
		}
		return ExprType{ValueType: intType, Fallible: true}, nil
	}
	if len(arguments) != 9 {
		return ExprType{}, fmt.Errorf("function '%s' expects 9 arguments, got %d", callee, len(arguments))
	}
	valuesType, err := c.checkExpr(scope, arguments[0], ctx)
	if err != nil {
		return ExprType{}, err
	}
	if valuesType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if err := requirePlotArrayType(callee, 1, valuesType.ValueType); err != nil {
		return ExprType{}, err
	}
	binsType, err := c.checkExpr(scope, arguments[1], ctx)
	if err != nil {
		return ExprType{}, err
	}
	if binsType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if binsType.ValueType != intType {
		return ExprType{}, fmt.Errorf("function '%s' argument 2 expects Int, got %s", callee, binsType.ValueType)
	}
	for idx, expected := range []Type{stringType, pixelIntType, pixelIntType, stringType, stringType, stringType, stringType} {
		current, err := c.checkExpr(scope, arguments[idx+2], ctx)
		if err != nil {
			return ExprType{}, err
		}
		if current.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if current.ValueType != expected {
			return ExprType{}, fmt.Errorf("function '%s' argument %d expects %s, got %s", callee, idx+3, expected, current.ValueType)
		}
	}
	return ExprType{ValueType: intType, Fallible: true}, nil
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

func (c checker) checkArrayLiteralExpr(scope *scope, expr ast.ArrayLiteralExpr, ctx functionContext, expected *Type) (Type, error) {
	if len(expr.Elements) == 0 {
		if expected == nil {
			return Type{}, fmt.Errorf("empty array literal `[]` requires an expected array type; write `var values: Int[] = []` or assign it where an array type is already known")
		}
		if !expected.IsArray {
			return Type{}, fmt.Errorf("empty array literal `[]` requires array type context, got %s; use an explicit annotation like `var values: Int[] = []`", *expected)
		}
		return *expected, nil
	}

	firstType, err := c.checkExpr(scope, expr.Elements[0], ctx)
	if err != nil {
		return Type{}, err
	}
	if firstType.Fallible {
		return Type{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	for _, element := range expr.Elements[1:] {
		elementType, err := c.checkExpr(scope, element, ctx)
		if err != nil {
			return Type{}, err
		}
		if elementType.Fallible {
			return Type{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if elementType.ValueType != firstType.ValueType {
			return Type{}, fmt.Errorf("array literal elements must all have the same type; found %s and %s", firstType.ValueType, elementType.ValueType)
		}
	}

	return withArrayDepth(Type{Name: firstType.ValueType.Name, Base: firstType.ValueType.Base, Dimension: firstType.ValueType.Dimension}, firstType.ValueType.ArrayDepth+1), nil
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
		return Type{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
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
			return Type{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
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
		return Type{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
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
				return Type{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
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
		actualType, err := c.checkExprWithExpected(scope, field.Value, ctx, &expectedType)
		if err != nil {
			return ExprType{}, err
		}
		if actualType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
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

func (c checker) checkRecordUpdateExpr(scope *scope, expr ast.RecordUpdateExpr, ctx functionContext) (ExprType, error) {
	sourceType, err := c.checkExpr(scope, expr.Source, ctx)
	if err != nil {
		return ExprType{}, err
	}
	if sourceType.Fallible {
		return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
	}
	if sourceType.ValueType.Base != "" || sourceType.ValueType.Name == "" || sourceType.ValueType.IsArray {
		return ExprType{}, fmt.Errorf("record update source must be a record value, got %s", sourceType.ValueType)
	}
	recordDecl, ok := c.lookupRecord(sourceType.ValueType.Name)
	if !ok {
		return ExprType{}, fmt.Errorf("unknown record type: %s", sourceType.ValueType.Name)
	}
	for _, field := range expr.Fields {
		fieldType, exists := recordDecl.fields[field.Name]
		if !exists {
			return ExprType{}, fmt.Errorf("type '%s' has no field '%s'", sourceType.ValueType.Name, field.Name)
		}
		valueType, err := c.checkExpr(scope, field.Value, ctx)
		if err != nil {
			return ExprType{}, err
		}
		if valueType.Fallible {
			return ExprType{}, fmt.Errorf("fallible expression must be handled explicitly; use '?' to propagate, '!' to assert success, or match to handle the Error")
		}
		if !isAssignable(valueType.ValueType, fieldType) {
			return ExprType{}, fmt.Errorf("field '%s' on type '%s' expects %s, got %s", field.Name, sourceType.ValueType.Name, fieldType, valueType.ValueType)
		}
	}
	return sourceType, nil
}

func (c checker) resolveReturnType(typeRef ast.TypeRef) (Type, error) {
	if typeRef.Function != nil {
		return Type{}, fmt.Errorf("function return types are not supported in M32")
	}
	return c.resolveType(typeRef, true)
}

func (c checker) resolveNonReturnType(typeRef ast.TypeRef) (Type, error) {
	return c.resolveType(typeRef, false)
}

func (c checker) resolveType(typeRef ast.TypeRef, allowVoid bool) (Type, error) {
	if len(typeRef.TupleOf) > 0 {
		if typeRef.IsArray || typeRef.ArrayDepth > 0 || typeRef.VectorOf != nil || typeRef.MatrixOf != nil || typeRef.Function != nil || typeRef.Name != "" || typeRef.Package != "" || typeRef.HasUnit {
			return Type{}, fmt.Errorf("tuple types are not part of Oct's public language. Use a record with named fields instead")
		}
		if len(typeRef.TupleOf) < 2 {
			return Type{}, fmt.Errorf("tuple types are not part of Oct's public language. Use a record with named fields instead")
		}
		elements := make([]Type, 0, len(typeRef.TupleOf))
		for _, elementRef := range typeRef.TupleOf {
			if len(elementRef.TupleOf) > 0 {
				return Type{}, fmt.Errorf("tuple types are not part of Oct's public language. Use a record with named fields instead")
			}
			elementType, err := c.resolveType(elementRef, false)
			if err != nil {
				return Type{}, err
			}
			elements = append(elements, elementType)
		}
		return Type{Tuple: &tupleType{Elements: elements}}, nil
	}

	arrayDepth := typeRef.ArrayDepth
	if typeRef.IsArray && arrayDepth == 0 {
		arrayDepth = 1
	}

	if typeRef.Function != nil {
		functionRef := typeRef.Function
		if functionRef.ReturnType.Function != nil {
			return Type{}, fmt.Errorf("function types cannot return function types in M32")
		}
		parameters := make([]Type, 0, len(functionRef.Parameters))
		for _, parameterRef := range functionRef.Parameters {
			parameterType, err := c.resolveType(parameterRef, false)
			if err != nil {
				return Type{}, err
			}
			parameters = append(parameters, parameterType)
		}
		returnType, err := c.resolveType(functionRef.ReturnType, false)
		if err != nil {
			return Type{}, err
		}
		signature := functionSignature{parameters: parameters, returnType: returnType, isFallible: functionRef.IsFallible}
		if functionRef.IsFallible {
			if functionRef.ErrorType == nil {
				return Type{}, fmt.Errorf("fallible function type must specify Error")
			}
			errorType, err := c.resolveType(*functionRef.ErrorType, false)
			if err != nil {
				return Type{}, err
			}
			if errorType != (Type{Base: BaseTypeError}) {
				return Type{}, fmt.Errorf("fallible function type must use Error")
			}
		}
		key := signature.String()
		c.functionTypes[key] = signature
		return Type{IsFunction: true, FunctionSignature: key}, nil
	}

	if typeRef.VectorOf != nil || typeRef.MatrixOf != nil {
		var elementRef ast.TypeRef
		isVector := typeRef.VectorOf != nil
		if isVector {
			elementRef = *typeRef.VectorOf
		} else {
			elementRef = *typeRef.MatrixOf
		}
		elementType, err := c.resolveType(elementRef, false)
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
			if c.allowUnresolvedImportedTypes {
				if typeRef.HasUnit {
					return Type{}, fmt.Errorf("invalid dimension-qualified type syntax: %s<%s>", qualifiedName, typeRef.Dimension.String())
				}
				return withArrayDepth(Type{Name: qualifiedName}, arrayDepth), nil
			}
			return Type{}, fmt.Errorf("unknown package '%s'", typeRef.Package)
		}
		if _, ok := imported.records[typeRef.Name]; ok {
			if typeRef.HasUnit {
				return Type{}, fmt.Errorf("invalid dimension-qualified type syntax: %s<%s>", qualifiedName, typeRef.Dimension.String())
			}
			return withArrayDepth(Type{Name: qualifiedName}, arrayDepth), nil
		}
		if _, ok := imported.enums[typeRef.Name]; ok {
			if typeRef.HasUnit {
				return Type{}, fmt.Errorf("invalid dimension-qualified type syntax: %s<%s>", qualifiedName, typeRef.Dimension.String())
			}
			return withArrayDepth(Type{Name: qualifiedName}, arrayDepth), nil
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
				if _, isDeclaredType := c.typeNames[typeRef.Name]; !isDeclaredType {
					return Type{}, err
				}
			}
		}
		if typeRef.HasUnit {
			return Type{}, fmt.Errorf("invalid dimension-qualified type syntax: %s<%s>", typeRef.Name, typeRef.Dimension.String())
		}
		return withArrayDepth(Type{Name: typeRef.Name}, arrayDepth), nil
	}
	if typeRef.HasUnit && !isDimensionCapableBaseType(baseType) {
		return Type{}, fmt.Errorf("invalid dimension-qualified type syntax: %s<%s>", typeRef.Name, typeRef.Dimension.String())
	}
	if arrayDepth > 0 {
		if baseType == BaseTypeError || baseType == BaseTypeRange || baseType == BaseTypeVoid {
			return Type{}, fmt.Errorf("unknown type: %s%s", typeRef.Name, strings.Repeat("[]", arrayDepth))
		}
		return withArrayDepth(Type{Base: baseType, Dimension: typeRef.Dimension}, arrayDepth), nil
	}
	if baseType == BaseTypeVoid && !allowVoid {
		return Type{}, fmt.Errorf("Void is only allowed as a function return type")
	}
	return Type{Base: baseType, Dimension: typeRef.Dimension}, nil
}

func resolveBaseType(name string) (BaseType, error) {
	switch BaseType(name) {
	case BaseTypeInt, BaseTypeFloat, BaseTypeComplex, BaseTypeBool, BaseTypeString, BaseTypeBytes, BaseTypeError, BaseTypeVoid, BaseTypeUI:
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
	if operator == "+" && leftType == (Type{Base: BaseTypeString}) && rightType == (Type{Base: BaseTypeString}) {
		return Type{Base: BaseTypeString}, nil
	}
	if leftType.Base == BaseTypeRange || rightType.Base == BaseTypeRange || leftType.Base == BaseTypeString || rightType.Base == BaseTypeString || leftType.Base == BaseTypeBytes || rightType.Base == BaseTypeBytes || leftType.Base == BaseTypeError || rightType.Base == BaseTypeError {
		return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}
	if leftType.IsArray || rightType.IsArray {
		return c.checkArrayBinaryExpr(operator, leftType, rightType)
	}
	if leftType.Base == BaseTypeBool || rightType.Base == BaseTypeBool || leftType.Name != "" || rightType.Name != "" {
		return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}
	if isComplexScalar(leftType) || isComplexScalar(rightType) {
		return checkComplexBinaryExpr(operator, leftType, rightType)
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
	case "%":
		if leftType.Base != BaseTypeInt || rightType.Base != BaseTypeInt {
			return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
		}
		if !leftType.Dimension.IsDimensionless() || !rightType.Dimension.IsDimensionless() {
			return Type{}, fmt.Errorf("modulo requires dimensionless Int operands")
		}
		return Type{Base: BaseTypeInt}, nil
	default:
		return Type{}, fmt.Errorf("unsupported operator %q", operator)
	}
}

func (c checker) checkEinsteinBinaryExpr(node ast.BinaryExpr, left ExprType, right ExprType) (ExprType, error) {
	operator := node.Operator
	if left.EinTerm == nil || right.EinTerm == nil {
		return ExprType{}, fmt.Errorf("indexed tensor expressions must appear on both sides of '%s' (left indexed=%t, right indexed=%t)", operator, left.EinTerm != nil, right.EinTerm != nil)
	}
	if operator != "*" && operator != "+" {
		return ExprType{}, fmt.Errorf("indexed tensor expressions only support '+' and '*' in M1")
	}
	leftLabels, leftOK := left.EinTerm.Labels, left.EinTerm.HasLabels
	rightLabels, rightOK := right.EinTerm.Labels, right.EinTerm.HasLabels
	if !leftOK {
		leftLabels, leftOK = einsteinIndexNames(node.Left)
	}
	if !rightOK {
		rightLabels, rightOK = einsteinIndexNames(node.Right)
	}
	var resultLabels [2]string
	var resultHasLabels bool
	if leftOK && rightOK {
		if operator == "+" {
			if leftLabels[0] == leftLabels[1] || rightLabels[0] == rightLabels[1] {
				return ExprType{}, fmt.Errorf("EinAdd requires distinct free indices per matrix term (left=[%s,%s], right=[%s,%s])", leftLabels[0], leftLabels[1], rightLabels[0], rightLabels[1])
			}
			if leftLabels[0] != rightLabels[0] || leftLabels[1] != rightLabels[1] {
				return ExprType{}, fmt.Errorf("EinAdd requires matching free-index order on both terms (left=[%s,%s], right=[%s,%s])", leftLabels[0], leftLabels[1], rightLabels[0], rightLabels[1])
			}
			resultLabels = leftLabels
			resultHasLabels = true
		}
		if operator == "*" {
			ordered := []string{leftLabels[0], leftLabels[1], rightLabels[0], rightLabels[1]}
			counts := map[string]int{}
			free := make([]string, 0, 2)
			freeCount := 0
			for _, label := range ordered {
				counts[label]++
			}
			for _, label := range ordered {
				count := counts[label]
				if count > 2 {
					return ExprType{}, fmt.Errorf("index '%s' appears %d times in [%s,%s]*[%s,%s]; only 1 (free) or 2 (contracted) are allowed in M0", label, count, leftLabels[0], leftLabels[1], rightLabels[0], rightLabels[1])
				}
				if count == 1 {
					freeCount++
					if len(free) == 0 || free[len(free)-1] != label {
						free = append(free, label)
					}
				}
			}
			if freeCount != 2 {
				return ExprType{}, fmt.Errorf("EinMul requires exactly 2 free indices in M0, got %d for [%s,%s]*[%s,%s]", freeCount, leftLabels[0], leftLabels[1], rightLabels[0], rightLabels[1])
			}
			resultLabels = [2]string{free[0], free[1]}
			resultHasLabels = true
		}
	}
	resultScalar, err := c.checkBinaryExpr(operator, left.EinTerm.ScalarType, right.EinTerm.ScalarType)
	if err != nil {
		return ExprType{}, err
	}
	return ExprType{
		ValueType: Type{Base: resultScalar.Base, Dimension: resultScalar.Dimension, IsMatrix: true},
		EinTerm: &einsteinTermType{
			ScalarType: resultScalar,
			Labels:     resultLabels,
			HasLabels:  resultHasLabels,
		},
	}, nil
}

func einsteinIndexNames(expr ast.Expr) ([2]string, bool) {
	indexExpr, ok := expr.(ast.IndexExpr)
	if !ok || len(indexExpr.Indices) != 2 {
		return [2]string{}, false
	}
	left, ok := indexExpr.Indices[0].(ast.IdentifierExpr)
	if !ok {
		return [2]string{}, false
	}
	right, ok := indexExpr.Indices[1].(ast.IdentifierExpr)
	if !ok {
		return [2]string{}, false
	}
	return [2]string{left.Name, right.Name}, true
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
	if operator == "%" {
		return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}
	if leftType.ArrayDepth != rightType.ArrayDepth {
		return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}
	leftElementType := peelArrayType(leftType)
	rightElementType := peelArrayType(rightType)
	if leftElementType.Base == BaseTypeBool || leftElementType.Base == BaseTypeString || leftElementType.Base == BaseTypeError ||
		rightElementType.Base == BaseTypeBool || rightElementType.Base == BaseTypeString || rightElementType.Base == BaseTypeError {
		return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}
	result, err := c.checkBinaryExpr(operator, leftElementType, rightElementType)
	if err != nil {
		return Type{}, err
	}
	return withArrayDepth(result, result.ArrayDepth+1), nil
}

func (c checker) checkComparisonExpr(operator string, leftType Type, rightType Type) (Type, error) {
	if isOrderingOperator(operator) && (leftType == (Type{Base: BaseTypeBool}) || leftType == (Type{Base: BaseTypeString}) || leftType == (Type{Base: BaseTypeBytes})) && leftType == rightType {
		return Type{}, fmt.Errorf("operator %q not defined for %s", operator, leftType)
	}
	if leftType.IsArray || rightType.IsArray || leftType.IsVector || rightType.IsVector || leftType.IsMatrix || rightType.IsMatrix || leftType.Base == BaseTypeBytes || rightType.Base == BaseTypeBytes || leftType.Base == BaseTypeRange || rightType.Base == BaseTypeRange || leftType.Base == BaseTypeError || rightType.Base == BaseTypeError {
		return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}

	if isRealNumericScalar(leftType) && isRealNumericScalar(rightType) {
		if leftType.Dimension != rightType.Dimension {
			return Type{}, fmt.Errorf("cannot compare %s and %s", leftType, rightType)
		}
		return Type{Base: BaseTypeBool}, nil
	}

	if isEqualityOperator(operator) {
		if isComplexScalar(leftType) && isComplexScalar(rightType) {
			return Type{Base: BaseTypeBool}, nil
		}
		if leftType == (Type{Base: BaseTypeBool}) && rightType == (Type{Base: BaseTypeBool}) {
			return Type{Base: BaseTypeBool}, nil
		}
		if leftType == (Type{Base: BaseTypeString}) && rightType == (Type{Base: BaseTypeString}) {
			return Type{Base: BaseTypeBool}, nil
		}
		if leftType.Name != "" && rightType.Name != "" {
			if isEnumType(c, leftType) && isEnumType(c, rightType) {
				if leftType.Name != rightType.Name {
					return Type{}, fmt.Errorf("operator %q requires matching enum types", operator)
				}
				return Type{Base: BaseTypeBool}, nil
			}
		}
	}

	if isOrderingOperator(operator) && (leftType == (Type{Base: BaseTypeBool}) || leftType == (Type{Base: BaseTypeString}) || leftType == (Type{Base: BaseTypeBytes}) || isEnumType(c, leftType)) && leftType == rightType {
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

func (c checker) flattenQualifiedFunctionName(expr ast.FieldAccessExpr) (string, string, bool) {
	pkgIdentifier, ok := expr.Target.(ast.IdentifierExpr)
	if !ok {
		return "", "", false
	}
	return pkgIdentifier.Name, expr.Field, true
}

func flattenFieldAccessChain(expr ast.Expr) (string, bool) {
	switch node := expr.(type) {
	case ast.IdentifierExpr:
		return node.Name, true
	case ast.FieldAccessExpr:
		left, ok := flattenFieldAccessChain(node.Target)
		if !ok {
			return "", false
		}
		return left + "." + node.Field, true
	default:
		return "", false
	}
}

func chainRoot(chain string) string {
	dot := strings.Index(chain, ".")
	if dot < 0 {
		return chain
	}
	return chain[:dot]
}

func chainParentPath(chain string) string {
	dot := strings.LastIndex(chain, ".")
	if dot < 0 {
		return chain
	}
	return chain[:dot]
}

func (c checker) missingFieldError(recordName string, field string, chain string, includeChain bool) error {
	message := fmt.Sprintf("record has no field '%s'", field)
	if includeChain {
		message += fmt.Sprintf("\nwhile resolving '%s'", chain)
	}
	if recordDecl, ok := c.lookupRecord(recordName); ok {
		fields := make([]string, 0, len(recordDecl.fields))
		for name := range recordDecl.fields {
			fields = append(fields, name)
		}
		slices.Sort(fields)
		message += fmt.Sprintf("\navailable fields: %s", strings.Join(fields, ", "))
	}
	return errors.New(message)
}

func missingTypeFieldError(typeName string, field string, chain string, includeChain bool) error {
	message := fmt.Sprintf("type '%s' has no field '%s'", typeName, field)
	if includeChain {
		message += fmt.Sprintf("\nwhile resolving '%s'", chain)
	}
	return errors.New(message)
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
	return baseType == BaseTypeInt || baseType == BaseTypeFloat || baseType == BaseTypeComplex
}

func isDimensionCapableBaseType(baseType BaseType) bool {
	return baseType == BaseTypeInt || baseType == BaseTypeFloat
}

func isNumericScalar(valueType Type) bool {
	return !valueType.IsArray && !valueType.IsVector && !valueType.IsMatrix && isNumericBaseType(valueType.Base)
}

func isRealNumericScalar(valueType Type) bool {
	return !valueType.IsArray && !valueType.IsVector && !valueType.IsMatrix && (valueType.Base == BaseTypeInt || valueType.Base == BaseTypeFloat)
}

func isComplexScalar(valueType Type) bool {
	return !valueType.IsArray && !valueType.IsVector && !valueType.IsMatrix && valueType.Base == BaseTypeComplex
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
	if actual.Tuple != nil || expected.Tuple != nil {
		if actual.Tuple == nil || expected.Tuple == nil {
			return false
		}
		if len(actual.Tuple.Elements) != len(expected.Tuple.Elements) {
			return false
		}
		for i := range actual.Tuple.Elements {
			if !isAssignable(actual.Tuple.Elements[i], expected.Tuple.Elements[i]) {
				return false
			}
		}
		return true
	}
	if actual.IsFlowInstance || expected.IsFlowInstance {
		return actual.IsFlowInstance && expected.IsFlowInstance && actual.FlowResultType == expected.FlowResultType
	}
	if actual.Name != "" || expected.Name != "" {
		return false
	}
	if actual.IsArray != expected.IsArray || actual.ArrayDepth != expected.ArrayDepth || actual.IsVector != expected.IsVector || actual.IsMatrix != expected.IsMatrix || actual.Dimension != expected.Dimension {
		return false
	}
	return (actual.Base == BaseTypeInt && expected.Base == BaseTypeFloat) ||
		(isRealNumericScalar(actual) && isComplexScalar(expected))
}

func isComplexCompatibleScalar(valueType Type) bool {
	return isRealNumericScalar(valueType) || isComplexScalar(valueType)
}

func checkComplexBinaryExpr(operator string, leftType Type, rightType Type) (Type, error) {
	if !isComplexCompatibleScalar(leftType) || !isComplexCompatibleScalar(rightType) {
		return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}
	switch operator {
	case "+", "-", "*", "/":
		return Type{Base: BaseTypeComplex}, nil
	default:
		return Type{}, fmt.Errorf("operator %q not defined for %s and %s", operator, leftType, rightType)
	}
}
