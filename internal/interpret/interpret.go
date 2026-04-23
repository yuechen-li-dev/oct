package interpret

import (
	"errors"
	"fmt"
	"io"
	"math"
	"math/cmplx"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/xuri/excelize/v2"

	"oct/internal/ast"
	"oct/internal/builtin"
	"oct/internal/dimension"
	"oct/internal/octagon"
	"oct/internal/project"
)

type ValueKind string

const (
	ValueInt     ValueKind = "Int"
	ValueFloat   ValueKind = "Float"
	ValueComplex ValueKind = "Complex"
	ValueBool    ValueKind = "Bool"
	ValueArray   ValueKind = "Array"
	ValueRange   ValueKind = "Range"
	ValueString  ValueKind = "String"
	ValueBytes   ValueKind = "Bytes"
	ValueError   ValueKind = "Error"
	ValueRecord  ValueKind = "Record"
	ValueEnum    ValueKind = "Enum"
	ValueVector  ValueKind = "Vector"
	ValueMatrix  ValueKind = "Matrix"
	ValueFunc    ValueKind = "Function"
	ValueFlow    ValueKind = "FlowInstance"
	ValueUI      ValueKind = "UI"
	ValueIndex   ValueKind = "Index"
	ValueDiffOp  ValueKind = "DiffOp"
	ValueFieldOp ValueKind = "FieldOp"
)

type Value struct {
	Kind      ValueKind
	Dimension dimension.Dimension
	Int       int64
	Float     float64
	Complex   complex128
	Bool      bool
	Text      string
	Bytes     []byte
	Array     []Value
	Vector    []Value
	Matrix    MatrixValue
	Range     RangeValue
	Error     ErrorValue
	Record    RecordValue
	Enum      EnumValue
	Function  FunctionValue
	Flow      *FlowRuntimeInstance
	UI        *uiirNode
	DiffOp    DifferentialOpValue
	FieldOp   FieldOpValue
}

type DifferentialOpValue struct {
	Operator string
	Operand  *Value
}

type FieldOpValue struct {
	Operator string
	Left     *Value
	Right    *Value
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

type MatrixValue struct {
	Rows     int
	Cols     int
	Elements []Value
}

type FunctionValue struct {
	Key string
}

type FlowRuntimeInstance struct {
	Decl             ast.FlowDecl
	Package          string
	RootEnv          *environment
	StateEnv         *environment
	CurrentState     string
	HasResumeTarget  bool
	ResumeTarget     string
	InstructionIndex int
	Completed        bool
	Result           Value
	StateHistory     []string
	UtilityWhenSites map[int]utilityWhenSiteState
	DirtyBoardFields map[string]struct{}
}

type utilityWhenSiteState struct {
	HasCurrent bool
	Current    Value
	Score      int64
	CommitAge  int64
}

func (v Value) String() string {
	switch v.Kind {
	case ValueInt:
		return strconv.FormatInt(v.Int, 10) + formatUnitSuffix(v.Dimension)
	case ValueFloat:
		return strconv.FormatFloat(v.Float, 'g', -1, 64) + formatUnitSuffix(v.Dimension)
	case ValueComplex:
		return fmt.Sprintf("Complex{Re:%g, Im:%g}", real(v.Complex), imag(v.Complex))
	case ValueBool:
		return strconv.FormatBool(v.Bool)
	case ValueString:
		return v.Text
	case ValueBytes:
		parts := make([]string, 0, len(v.Bytes))
		for _, b := range v.Bytes {
			parts = append(parts, strconv.FormatInt(int64(b), 10))
		}
		return "bytes[" + strings.Join(parts, ", ") + "]"
	case ValueArray:
		parts := make([]string, 0, len(v.Array))
		for _, element := range v.Array {
			parts = append(parts, element.String())
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case ValueVector:
		parts := make([]string, 0, len(v.Vector))
		for _, element := range v.Vector {
			parts = append(parts, element.String())
		}
		return "vector[" + strings.Join(parts, ", ") + "]"
	case ValueMatrix:
		rows := make([]string, 0, v.Matrix.Rows)
		for r := 0; r < v.Matrix.Rows; r++ {
			parts := make([]string, 0, v.Matrix.Cols)
			for c := 0; c < v.Matrix.Cols; c++ {
				parts = append(parts, v.Matrix.Elements[r*v.Matrix.Cols+c].String())
			}
			rows = append(rows, "["+strings.Join(parts, ", ")+"]")
		}
		return "matrix[" + strings.Join(rows, ", ") + "]"
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
	case ValueFunc:
		return v.Function.Key
	case ValueFlow:
		if v.Flow == nil {
			return "FlowInstance<invalid>"
		}
		return fmt.Sprintf("FlowInstance<%s>", v.Flow.Decl.ReturnType.Name)
	case ValueUI:
		if v.UI == nil {
			return "UI<invalid>"
		}
		return "UI<" + uiirSignature(withUIIRNodeIDs(v.UI)) + ">"
	case ValueIndex:
		return "Index(" + v.Text + ")"
	case ValueDiffOp:
		if v.DiffOp.Operand == nil {
			return v.DiffOp.Operator + "(<invalid>)"
		}
		return v.DiffOp.Operator + "(" + v.DiffOp.Operand.String() + ")"
	case ValueFieldOp:
		if v.FieldOp.Left == nil || v.FieldOp.Right == nil {
			return "FieldOp(<invalid>)"
		}
		return "(" + v.FieldOp.Left.String() + " " + v.FieldOp.Operator + " " + v.FieldOp.Right.String() + ")"
	default:
		return "<invalid>"
	}
}

type interpreter struct {
	functions      map[string]ast.FunctionDecl
	records        map[string]ast.RecordDecl
	enums          map[string]ast.EnumDecl
	flows          map[string]ast.FlowDecl
	functionSource map[string]string
	flowSource     map[string]string
	stdout         io.Writer
	workbooks      wrapperHandleStore[*xlsxWorkbook]
	images         wrapperHandleStore[*wrapperImage]
	uiMounts       wrapperHandleStore[*uiMount]
	wrappers       wrapperBuiltinRegistry
}

type xlsxWorkbook struct {
	file *excelize.File
}

type ExecuteOptions struct {
	OutputPathPrefix string
}

type environment struct {
	parent *environment
	values map[string]binding
}

type binding struct {
	value   Value
	mutable bool
}

type stmtResult struct {
	value    Value
	returned bool
}

type evalResult struct {
	value    Value
	hasError bool
	errorVal Value
	einTerm  *einsteinIndexedTerm
}

type einsteinIndexedTerm struct {
	matrix Value
	labels []string
}

type callResult struct {
	value    Value
	hasError bool
	errorVal Value
}

func ExecuteMain(program project.Program, stdout io.Writer) (Value, error) {
	interpreter := interpreter{
		functions:      make(map[string]ast.FunctionDecl),
		records:        make(map[string]ast.RecordDecl),
		enums:          make(map[string]ast.EnumDecl),
		flows:          make(map[string]ast.FlowDecl),
		functionSource: make(map[string]string),
		flowSource:     make(map[string]string),
		stdout:         stdout,
		workbooks:      newWrapperHandleStore[*xlsxWorkbook]("workbook"),
		images:         newWrapperHandleStore[*wrapperImage]("image"),
		uiMounts:       newWrapperHandleStore[*uiMount]("ui mount"),
		wrappers:       newWrapperBuiltinRegistry(xlsxWrapperBuiltins(), imageWrapperBuiltins(), plotWrapperBuiltins(), jsonWrapperBuiltins(), fileWrapperBuiltins(), pathWrapperBuiltins(), directoryWrapperBuiltins(), csvWrapperBuiltins(), archiveWrapperBuiltins(), compressionWrapperBuiltins(), hashWrapperBuiltins(), regexWrapperBuiltins(), timeWrapperBuiltins()),
	}
	for pkgName, pkg := range program.Packages {
		for _, record := range pkg.Records {
			interpreter.records[pkgName+"."+record.Name] = record
		}
		for _, enumDecl := range pkg.Enums {
			interpreter.enums[pkgName+"."+enumDecl.Name] = enumDecl
		}
		for _, function := range pkg.Functions {
			key := pkgName + "." + function.Name
			interpreter.functions[key] = function
			interpreter.functionSource[key] = pkgName
		}
		for _, flow := range pkg.Flows {
			key := pkgName + "." + flow.Name
			interpreter.flows[key] = flow
			interpreter.flowSource[key] = pkgName
		}
	}

	mainFunction, err := interpreter.findMain(program.Entry)
	if err != nil {
		return Value{}, err
	}

	result, err := interpreter.executeFunction(mainFunction, program.Entry, nil)
	if err != nil {
		return Value{}, err
	}
	if result.hasError {
		return Value{}, fmt.Errorf("fatal error: %s", result.errorVal.Error.Message)
	}
	return result.value, nil
}

func ExecuteFunction(program project.Program, pkgName string, functionName string, stdout io.Writer) error {
	return ExecuteFunctionWithArgsAndOptions(program, pkgName, functionName, nil, stdout, ExecuteOptions{})
}

func ExecuteFunctionWithArgs(program project.Program, pkgName string, functionName string, arguments []Value, stdout io.Writer) error {
	return ExecuteFunctionWithArgsAndOptions(program, pkgName, functionName, arguments, stdout, ExecuteOptions{})
}

func ExecuteFunctionWithArgsAndOptions(program project.Program, pkgName string, functionName string, arguments []Value, stdout io.Writer, options ExecuteOptions) error {
	clearPrefix := func() {}
	if options.OutputPathPrefix != "" {
		clearPrefix = SetOutputPathPrefix(options.OutputPathPrefix)
	}
	defer clearPrefix()

	interpreter := newInterpreter(program, stdout)
	key := pkgName + "." + functionName
	function, ok := interpreter.functions[key]
	if !ok {
		return fmt.Errorf("missing function %s", key)
	}
	if len(function.Parameters) != len(arguments) {
		return fmt.Errorf("test function %s expects %d arguments, got %d", key, len(function.Parameters), len(arguments))
	}
	result, err := interpreter.executeFunction(function, pkgName, arguments)
	if err != nil {
		return err
	}
	if result.hasError {
		return fmt.Errorf("fatal error: %s", result.errorVal.Error.Message)
	}
	return nil
}

func newInterpreter(program project.Program, stdout io.Writer) interpreter {
	interp := interpreter{
		functions:      make(map[string]ast.FunctionDecl),
		records:        make(map[string]ast.RecordDecl),
		enums:          make(map[string]ast.EnumDecl),
		flows:          make(map[string]ast.FlowDecl),
		functionSource: make(map[string]string),
		flowSource:     make(map[string]string),
		stdout:         stdout,
		workbooks:      newWrapperHandleStore[*xlsxWorkbook]("workbook"),
		images:         newWrapperHandleStore[*wrapperImage]("image"),
		uiMounts:       newWrapperHandleStore[*uiMount]("ui mount"),
		wrappers:       newWrapperBuiltinRegistry(xlsxWrapperBuiltins(), imageWrapperBuiltins(), plotWrapperBuiltins(), jsonWrapperBuiltins(), fileWrapperBuiltins(), pathWrapperBuiltins(), directoryWrapperBuiltins(), csvWrapperBuiltins(), archiveWrapperBuiltins(), compressionWrapperBuiltins(), hashWrapperBuiltins(), regexWrapperBuiltins(), timeWrapperBuiltins()),
	}
	for currentPkg, pkg := range program.Packages {
		for _, record := range pkg.Records {
			interp.records[currentPkg+"."+record.Name] = record
		}
		for _, enumDecl := range pkg.Enums {
			interp.enums[currentPkg+"."+enumDecl.Name] = enumDecl
		}
		for _, function := range pkg.Functions {
			key := currentPkg + "." + function.Name
			interp.functions[key] = function
			interp.functionSource[key] = currentPkg
		}
		for _, flow := range pkg.Flows {
			key := currentPkg + "." + flow.Name
			interp.flows[key] = flow
			interp.flowSource[key] = currentPkg
		}
	}
	return interp
}

func newEnvironment(parent *environment) *environment {
	return &environment{parent: parent, values: make(map[string]binding)}
}

func (e *environment) define(name string, value Value, mutable bool) {
	e.values[name] = binding{value: value, mutable: mutable}
}

func (e *environment) lookup(name string) (binding, bool) {
	for current := e; current != nil; current = current.parent {
		value, ok := current.values[name]
		if ok {
			return value, true
		}
	}
	return binding{}, false
}

func (e *environment) assign(name string, value Value) bool {
	for current := e; current != nil; current = current.parent {
		bindingValue, ok := current.values[name]
		if !ok {
			continue
		}
		if !bindingValue.mutable {
			return false
		}
		bindingValue.value = value
		current.values[name] = bindingValue
		return true
	}
	return false
}

func assignBindingValue(env *environment, name string, value Value) bool {
	for current := env; current != nil; current = current.parent {
		bindingValue, ok := current.values[name]
		if !ok {
			continue
		}
		bindingValue.value = value
		current.values[name] = bindingValue
		return true
	}
	return false
}

func snapshotEnvironment(env *environment) *environment {
	snapshot := newEnvironment(nil)
	chain := make([]*environment, 0)
	for current := env; current != nil; current = current.parent {
		chain = append(chain, current)
	}
	for idx := len(chain) - 1; idx >= 0; idx-- {
		for name, bindingValue := range chain[idx].values {
			snapshot.values[name] = binding{value: cloneValue(bindingValue.value), mutable: bindingValue.mutable}
		}
	}
	return snapshot
}

func cloneValue(value Value) Value {
	copied := value
	switch value.Kind {
	case ValueArray:
		copied.Array = make([]Value, len(value.Array))
		for index := range value.Array {
			copied.Array[index] = cloneValue(value.Array[index])
		}
	case ValueVector:
		copied.Vector = make([]Value, len(value.Vector))
		for index := range value.Vector {
			copied.Vector[index] = cloneValue(value.Vector[index])
		}
	case ValueMatrix:
		copied.Matrix.Elements = make([]Value, len(value.Matrix.Elements))
		for index := range value.Matrix.Elements {
			copied.Matrix.Elements[index] = cloneValue(value.Matrix.Elements[index])
		}
	case ValueRecord:
		copied.Record.FieldOrder = append([]string(nil), value.Record.FieldOrder...)
		copied.Record.Fields = make(map[string]Value, len(value.Record.Fields))
		for key, fieldValue := range value.Record.Fields {
			copied.Record.Fields[key] = cloneValue(fieldValue)
		}
	case ValueDiffOp:
		if value.DiffOp.Operand != nil {
			operandCopy := cloneValue(*value.DiffOp.Operand)
			copied.DiffOp.Operand = &operandCopy
		}
	case ValueFieldOp:
		if value.FieldOp.Left != nil {
			leftCopy := cloneValue(*value.FieldOp.Left)
			copied.FieldOp.Left = &leftCopy
		}
		if value.FieldOp.Right != nil {
			rightCopy := cloneValue(*value.FieldOp.Right)
			copied.FieldOp.Right = &rightCopy
		}
	}
	return copied
}

func (i interpreter) findMain(entryPackage string) (ast.FunctionDecl, error) {
	function, ok := i.functions[entryPackage+".Main"]
	if !ok {
		return ast.FunctionDecl{}, errors.New("missing Main function")
	}
	if len(function.Parameters) != 0 {
		return ast.FunctionDecl{}, errors.New("Main must not have parameters")
	}
	return function, nil
}

func (i interpreter) executeFunction(function ast.FunctionDecl, pkgName string, arguments []Value) (callResult, error) {
	env := newEnvironment(nil)
	for index, parameter := range function.Parameters {
		env.define(parameter.Name, arguments[index], false)
	}

	result, err := i.executeBlock(env, pkgName, function.Body)
	if err != nil {
		return callResult{}, err
	}
	if !result.returned {
		if function.ReturnType.Name == "Void" {
			return callResult{}, nil
		}
		return callResult{}, fmt.Errorf("runtime invariant violation: %s completed without returning", function.Name)
	}
	if function.IsFallible && result.value.Kind == ValueError {
		return callResult{hasError: true, errorVal: result.value}, nil
	}
	return callResult{value: result.value}, nil
}

func (i interpreter) instantiateFlow(flow ast.FlowDecl, pkgName string, arguments []Value) *FlowRuntimeInstance {
	rootEnv := newEnvironment(nil)
	for index, parameter := range flow.Parameters {
		rootEnv.define(parameter.Name, arguments[index], false)
	}
	if len(flow.Board) > 0 {
		boardFields := make(map[string]Value, len(flow.Board))
		fieldOrder := make([]string, 0, len(flow.Board))
		for _, field := range flow.Board {
			boardFields[field.Name] = defaultFlowBoardValue(field.Type)
			fieldOrder = append(fieldOrder, field.Name)
		}
		rootEnv.define("board", Value{
			Kind: ValueRecord,
			Record: RecordValue{
				TypeName:   "__flow_board_" + flow.Name,
				Fields:     boardFields,
				FieldOrder: fieldOrder,
			},
		}, false)
	}
	return &FlowRuntimeInstance{
		Decl:             flow,
		Package:          pkgName,
		RootEnv:          rootEnv,
		StateEnv:         nil,
		StateHistory:     nil,
		UtilityWhenSites: make(map[int]utilityWhenSiteState),
		DirtyBoardFields: make(map[string]struct{}),
	}
}

func defaultFlowBoardValue(fieldType ast.TypeRef) Value {
	switch fieldType.Name {
	case "Bool":
		return Value{Kind: ValueBool, Bool: false}
	case "Int":
		return Value{Kind: ValueInt, Int: 0}
	case "Float":
		return Value{Kind: ValueFloat, Float: 0.0}
	case "String":
		return Value{Kind: ValueString, Text: ""}
	default:
		return Value{}
	}
}

type flowSignalKind int

const (
	flowSignalNone flowSignalKind = iota
	flowSignalSuspend
	flowSignalGoto
	flowSignalReturn
)

type flowSignal struct {
	kind   flowSignalKind
	target string
	value  Value
}

const flowInstanceBindingName = "__oct_flow_instance"

func (i interpreter) stepFlow(instance *FlowRuntimeInstance) error {
	if instance.Completed {
		return nil
	}
	if instance.CurrentState == "" {
		instance.CurrentState = instance.Decl.EntryState
		instance.InstructionIndex = 0
		instance.StateEnv = newEnvironment(instance.RootEnv)
		instance.StateEnv.define(flowInstanceBindingName, Value{Kind: ValueFlow, Flow: instance}, false)
		instance.StateHistory = append(instance.StateHistory, instance.CurrentState)
	}
	for {
		state, ok := findFlowState(instance.Decl, instance.CurrentState)
		if !ok {
			return fmt.Errorf("runtime invariant violation: unknown flow state %s", instance.CurrentState)
		}
		if instance.InstructionIndex >= len(state.Body.Statements) {
			return fmt.Errorf("runtime invariant violation: flow state %s exited without suspend or return", state.Name)
		}
		signal, err := i.executeFlowStmt(instance.StateEnv, instance.Package, state.Body.Statements[instance.InstructionIndex])
		if err != nil {
			return err
		}
		switch signal.kind {
		case flowSignalNone:
			instance.InstructionIndex++
		case flowSignalSuspend:
			instance.InstructionIndex++
			return nil
		case flowSignalGoto:
			instance.CurrentState = signal.target
			instance.InstructionIndex = 0
			instance.StateEnv = newEnvironment(instance.RootEnv)
			instance.StateEnv.define(flowInstanceBindingName, Value{Kind: ValueFlow, Flow: instance}, false)
			instance.StateHistory = append(instance.StateHistory, instance.CurrentState)
		case flowSignalReturn:
			instance.Completed = true
			instance.CurrentState = ""
			instance.Result = signal.value
			return nil
		}
	}
}

func findFlowState(flow ast.FlowDecl, name string) (ast.StateDecl, bool) {
	for _, state := range flow.States {
		if state.Name == name {
			return state, true
		}
	}
	return ast.StateDecl{}, false
}

func (i interpreter) executeBlock(parent *environment, pkgName string, block ast.Block) (stmtResult, error) {
	blockEnv := newEnvironment(parent)
	for _, statement := range block.Statements {
		result, err := i.executeStmt(blockEnv, pkgName, statement)
		if err != nil {
			return stmtResult{}, err
		}
		if result.returned {
			return result, nil
		}
	}
	return stmtResult{}, nil
}

func (i interpreter) executeStmt(env *environment, pkgName string, stmt ast.Stmt) (stmtResult, error) {
	switch node := stmt.(type) {
	case ast.LetStmt:
		value, err := i.evalExpr(env, pkgName, node.Value)
		if err != nil {
			return stmtResult{}, err
		}
		if value.hasError {
			return stmtResult{value: value.errorVal, returned: true}, nil
		}
		env.define(node.Name, value.value, false)
		return stmtResult{}, nil
	case ast.VarStmt:
		value, err := i.evalExpr(env, pkgName, node.Value)
		if err != nil {
			return stmtResult{}, err
		}
		if value.hasError {
			return stmtResult{value: value.errorVal, returned: true}, nil
		}
		env.define(node.Name, value.value, true)
		return stmtResult{}, nil
	case ast.AssignStmt:
		value, err := i.evalExpr(env, pkgName, node.Value)
		if err != nil {
			return stmtResult{}, err
		}
		if value.hasError {
			return stmtResult{value: value.errorVal, returned: true}, nil
		}
		if !env.assign(node.Name, value.value) {
			return stmtResult{}, fmt.Errorf("runtime invariant violation: assignment target '%s' is not a mutable binding", node.Name)
		}
		return stmtResult{}, nil
	case ast.IndexAssignStmt:
		targetBinding, ok := env.lookup(node.Target)
		if !ok {
			return stmtResult{}, fmt.Errorf("runtime invariant violation: undefined variable %s", node.Target)
		}
		if !targetBinding.mutable {
			return stmtResult{}, fmt.Errorf("runtime invariant violation: assignment target '%s' is not a mutable binding", node.Target)
		}
		if len(node.Indices) == 0 {
			return stmtResult{}, fmt.Errorf("runtime invariant violation: index assignment requires at least one index")
		}
		indices := make([]int64, 0, len(node.Indices))
		for _, idxExpr := range node.Indices {
			index, err := i.evalExpr(env, pkgName, idxExpr)
			if err != nil {
				return stmtResult{}, err
			}
			if index.hasError {
				return stmtResult{value: index.errorVal, returned: true}, nil
			}
			if index.value.Kind != ValueInt || !index.value.Dimension.IsDimensionless() {
				return stmtResult{}, fmt.Errorf("runtime invariant violation: index must be Int, got %s", valueTypeName(index.value))
			}
			indices = append(indices, index.value.Int)
		}

		value, err := i.evalExpr(env, pkgName, node.Value)
		if err != nil {
			return stmtResult{}, err
		}
		if value.hasError {
			return stmtResult{value: value.errorVal, returned: true}, nil
		}

		updated := targetBinding.value
		switch updated.Kind {
		case ValueArray:
			if len(indices) != 1 {
				return stmtResult{}, fmt.Errorf("runtime invariant violation: array index assignment requires exactly 1 index, got %d", len(indices))
			}
			if indices[0] < 0 || indices[0] >= int64(len(updated.Array)) {
				return stmtResult{}, fmt.Errorf("runtime error: index %d out of bounds for array of length %d", indices[0], len(updated.Array))
			}
			updated.Array[indices[0]] = value.value
		case ValueMatrix:
			if len(indices) != 2 {
				return stmtResult{}, fmt.Errorf("runtime invariant violation: matrix index assignment requires exactly 2 indices, got %d", len(indices))
			}
			r := indices[0]
			c := indices[1]
			if r < 0 || c < 0 || r >= int64(updated.Matrix.Rows) || c >= int64(updated.Matrix.Cols) {
				return stmtResult{}, fmt.Errorf("runtime error: index [%d, %d] out of bounds for matrix of shape %dx%d", r, c, updated.Matrix.Rows, updated.Matrix.Cols)
			}
			updated.Matrix.Elements[r*int64(updated.Matrix.Cols)+c] = value.value
		default:
			return stmtResult{}, fmt.Errorf("runtime invariant violation: index assignment requires array or matrix target")
		}
		if !env.assign(node.Target, updated) {
			return stmtResult{}, fmt.Errorf("runtime invariant violation: assignment target '%s' is not a mutable binding", node.Target)
		}
		return stmtResult{}, nil
	case ast.FieldAssignStmt:
		targetBinding, ok := env.lookup(node.Target)
		if !ok {
			return stmtResult{}, fmt.Errorf("runtime invariant violation: undefined variable %s", node.Target)
		}
		if targetBinding.value.Kind != ValueRecord {
			return stmtResult{}, fmt.Errorf("runtime invariant violation: field assignment requires record target")
		}
		value, err := i.evalExpr(env, pkgName, node.Value)
		if err != nil {
			return stmtResult{}, err
		}
		if value.hasError {
			return stmtResult{value: value.errorVal, returned: true}, nil
		}
		updated := targetBinding.value
		if _, exists := updated.Record.Fields[node.Field]; !exists {
			return stmtResult{}, fmt.Errorf("runtime invariant violation: record type '%s' has no field '%s'", updated.Record.TypeName, node.Field)
		}
		updated.Record.Fields[node.Field] = value.value
		if !assignBindingValue(env, node.Target, updated) {
			return stmtResult{}, fmt.Errorf("runtime invariant violation: undefined variable %s", node.Target)
		}
		if node.Target == "board" {
			if instance, flowErr := flowInstanceFromEnv(env); flowErr == nil {
				instance.DirtyBoardFields[node.Field] = struct{}{}
			}
		}
		return stmtResult{}, nil
	case ast.ReturnStmt:
		if node.Value == nil {
			return stmtResult{returned: true}, nil
		}
		value, err := i.evalExpr(env, pkgName, node.Value)
		if err != nil {
			return stmtResult{}, err
		}
		if value.hasError {
			return stmtResult{value: value.errorVal, returned: true}, nil
		}
		return stmtResult{value: value.value, returned: true}, nil
	case ast.ExprStmt:
		value, err := i.evalExpr(env, pkgName, node.Value)
		if err != nil {
			return stmtResult{}, err
		}
		if value.hasError {
			return stmtResult{value: value.errorVal, returned: true}, nil
		}
		return stmtResult{}, nil
	case ast.ForStmt:
		rangeValue, err := i.evalExpr(env, pkgName, node.Range)
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
			iterationEnv.define(node.Name, Value{Kind: ValueInt, Int: current}, false)
			result, err := i.executeBlock(iterationEnv, pkgName, node.Body)
			if err != nil {
				return stmtResult{}, err
			}
			if result.returned {
				return result, nil
			}
		}
		return stmtResult{}, nil
	case ast.MatchStmt:
		subject, err := i.evalExpr(env, pkgName, node.Subject)
		if err != nil {
			return stmtResult{}, err
		}
		armEnv := newEnvironment(env)
		if subject.hasError {
			armEnv.define(node.ErrName, subject.errorVal, false)
			return i.executeBlock(armEnv, pkgName, node.ErrBody)
		}
		armEnv.define(node.OkName, subject.value, false)
		return i.executeBlock(armEnv, pkgName, node.OkBody)
	case ast.IfStmt:
		condition, err := i.evalExpr(env, pkgName, node.Condition)
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
			return i.executeBlock(env, pkgName, node.ThenBody)
		}
		if node.ElseBody != nil {
			return i.executeBlock(env, pkgName, *node.ElseBody)
		}
		return stmtResult{}, nil
	case ast.WhileStmt:
		for {
			condition, err := i.evalExpr(env, pkgName, node.Condition)
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

			result, err := i.executeBlock(env, pkgName, node.Body)
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

func (i interpreter) executeFlowBlock(parent *environment, pkgName string, block ast.Block) (flowSignal, error) {
	blockEnv := newEnvironment(parent)
	for _, statement := range block.Statements {
		signal, err := i.executeFlowStmt(blockEnv, pkgName, statement)
		if err != nil {
			return flowSignal{}, err
		}
		if signal.kind != flowSignalNone {
			return signal, nil
		}
	}
	return flowSignal{}, nil
}

func (i interpreter) executeFlowStmt(env *environment, pkgName string, stmt ast.Stmt) (flowSignal, error) {
	switch node := stmt.(type) {
	case ast.GotoStmt:
		return flowSignal{kind: flowSignalGoto, target: node.Target}, nil
	case ast.SuspendStmt:
		return flowSignal{kind: flowSignalSuspend}, nil
	case ast.RememberStmt:
		instance, err := flowInstanceFromEnv(env)
		if err != nil {
			return flowSignal{}, err
		}
		instance.HasResumeTarget = true
		instance.ResumeTarget = instance.CurrentState
		return flowSignal{}, nil
	case ast.ResumeStmt:
		instance, err := flowInstanceFromEnv(env)
		if err != nil {
			return flowSignal{}, err
		}
		if !instance.HasResumeTarget {
			return flowSignal{}, fmt.Errorf("runtime error: resume called with empty resume slot")
		}
		target := instance.ResumeTarget
		instance.HasResumeTarget = false
		instance.ResumeTarget = ""
		return flowSignal{kind: flowSignalGoto, target: target}, nil
	case ast.ReturnStmt:
		if node.Value == nil {
			return flowSignal{kind: flowSignalReturn}, nil
		}
		value, err := i.evalExpr(env, pkgName, node.Value)
		if err != nil {
			return flowSignal{}, err
		}
		if value.hasError {
			return flowSignal{kind: flowSignalReturn, value: value.errorVal}, nil
		}
		return flowSignal{kind: flowSignalReturn, value: value.value}, nil
	case ast.IfStmt:
		condition, err := i.evalExpr(env, pkgName, node.Condition)
		if err != nil {
			return flowSignal{}, err
		}
		if condition.hasError {
			return flowSignal{kind: flowSignalReturn, value: condition.errorVal}, nil
		}
		if condition.value.Kind != ValueBool {
			return flowSignal{}, fmt.Errorf("runtime invariant violation: if condition must be Bool, got %s", condition.value.Kind)
		}
		if condition.value.Bool {
			return i.executeFlowBlock(env, pkgName, node.ThenBody)
		}
		if node.ElseBody != nil {
			return i.executeFlowBlock(env, pkgName, *node.ElseBody)
		}
		return flowSignal{}, nil
	case ast.WhileStmt:
		for {
			condition, err := i.evalExpr(env, pkgName, node.Condition)
			if err != nil {
				return flowSignal{}, err
			}
			if condition.hasError {
				return flowSignal{kind: flowSignalReturn, value: condition.errorVal}, nil
			}
			if condition.value.Kind != ValueBool {
				return flowSignal{}, fmt.Errorf("runtime invariant violation: while condition must be Bool, got %s", condition.value.Kind)
			}
			if !condition.value.Bool {
				return flowSignal{}, nil
			}
			signal, err := i.executeFlowBlock(env, pkgName, node.Body)
			if err != nil {
				return flowSignal{}, err
			}
			if signal.kind != flowSignalNone {
				return signal, nil
			}
		}
	case ast.ForStmt:
		rangeValue, err := i.evalExpr(env, pkgName, node.Range)
		if err != nil {
			return flowSignal{}, err
		}
		if rangeValue.hasError {
			return flowSignal{kind: flowSignalReturn, value: rangeValue.errorVal}, nil
		}
		if rangeValue.value.Kind != ValueRange {
			return flowSignal{}, fmt.Errorf("runtime invariant violation: for loop expected Range, got %s", rangeValue.value.Kind)
		}
		for current := rangeValue.value.Range.Start; current < rangeValue.value.Range.End; current += rangeValue.value.Range.Step {
			iterationEnv := newEnvironment(env)
			iterationEnv.define(node.Name, Value{Kind: ValueInt, Int: current}, false)
			signal, err := i.executeFlowBlock(iterationEnv, pkgName, node.Body)
			if err != nil {
				return flowSignal{}, err
			}
			if signal.kind != flowSignalNone {
				return signal, nil
			}
		}
		return flowSignal{}, nil
	case ast.MatchStmt:
		subject, err := i.evalExpr(env, pkgName, node.Subject)
		if err != nil {
			return flowSignal{}, err
		}
		armEnv := newEnvironment(env)
		if subject.hasError {
			armEnv.define(node.ErrName, subject.errorVal, false)
			return i.executeFlowBlock(armEnv, pkgName, node.ErrBody)
		}
		armEnv.define(node.OkName, subject.value, false)
		return i.executeFlowBlock(armEnv, pkgName, node.OkBody)
	case ast.WhenStmt:
		for _, whenCase := range node.Cases {
			condition, err := i.evalExpr(env, pkgName, whenCase.Condition)
			if err != nil {
				return flowSignal{}, err
			}
			if condition.hasError {
				return flowSignal{kind: flowSignalReturn, value: condition.errorVal}, nil
			}
			if condition.value.Kind != ValueBool {
				return flowSignal{}, fmt.Errorf("runtime invariant violation: when case condition must be Bool, got %s", condition.value.Kind)
			}
			if condition.value.Bool {
				return i.executeWhenAction(env, pkgName, whenCase.Action)
			}
		}
		return i.executeWhenAction(env, pkgName, node.Else)
	default:
		result, err := i.executeStmt(env, pkgName, stmt)
		if err != nil {
			return flowSignal{}, err
		}
		if result.returned {
			return flowSignal{kind: flowSignalReturn, value: result.value}, nil
		}
		return flowSignal{}, nil
	}
}

func flowInstanceFromEnv(env *environment) (*FlowRuntimeInstance, error) {
	bindingValue, ok := env.lookup(flowInstanceBindingName)
	if !ok {
		return nil, fmt.Errorf("runtime invariant violation: flow instance binding is missing")
	}
	if bindingValue.value.Kind != ValueFlow || bindingValue.value.Flow == nil {
		return nil, fmt.Errorf("runtime invariant violation: flow instance binding is invalid")
	}
	return bindingValue.value.Flow, nil
}

func (i interpreter) executeWhenAction(env *environment, pkgName string, action ast.WhenAction) (flowSignal, error) {
	switch node := action.(type) {
	case ast.WhenGotoAction:
		return flowSignal{kind: flowSignalGoto, target: node.Target}, nil
	case ast.WhenSuspendAction:
		return flowSignal{kind: flowSignalSuspend}, nil
	case ast.WhenReturnAction:
		return i.executeFlowStmt(env, pkgName, ast.ReturnStmt{Value: node.Value})
	case ast.WhenBlockAction:
		for _, statement := range node.Statements {
			signal, err := i.executeFlowStmt(env, pkgName, statement)
			if err != nil {
				return flowSignal{}, err
			}
			if signal.kind != flowSignalNone {
				return signal, nil
			}
		}
		return flowSignal{}, nil
	default:
		return flowSignal{}, fmt.Errorf("runtime invariant violation: unsupported when action %T", action)
	}
}

func (i interpreter) evalExpr(env *environment, pkgName string, expr ast.Expr) (evalResult, error) {
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
		value, err := i.evalArrayLiteralExpr(env, pkgName, node)
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: value}, nil
	case ast.VectorLiteralExpr:
		value, err := i.evalVectorLiteralExpr(env, pkgName, node)
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: value}, nil
	case ast.MatrixLiteralExpr:
		value, err := i.evalMatrixLiteralExpr(env, pkgName, node)
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: value}, nil
	case ast.IdentifierExpr:
		valueBinding, ok := env.lookup(node.Name)
		if ok {
			return evalResult{value: valueBinding.value}, nil
		}
		functionKey := pkgName + "." + node.Name
		if _, exists := i.functions[functionKey]; exists {
			return evalResult{value: Value{Kind: ValueFunc, Function: FunctionValue{Key: functionKey}}}, nil
		}
		return evalResult{}, fmt.Errorf("runtime invariant violation: undefined variable %s", node.Name)
	case ast.CallExpr:
		return i.evalCallExpr(env, pkgName, node)
	case ast.RecordLiteralExpr:
		return i.evalRecordLiteralExpr(env, pkgName, node)
	case ast.RecordUpdateExpr:
		return i.evalRecordUpdateExpr(env, pkgName, node)
	case ast.EnumValueExpr:
		enumDecl, enumTypeName, ok := i.lookupEnumDecl(pkgName, node.EnumName)
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
		return evalResult{value: Value{Kind: ValueEnum, Enum: EnumValue{TypeName: enumTypeName, Variant: node.Variant}}}, nil
	case ast.IndexExpr:
		target, err := i.evalExpr(env, pkgName, node.Target)
		if err != nil {
			return evalResult{}, err
		}
		if target.hasError {
			return evalResult{hasError: true, errorVal: target.errorVal}, nil
		}
		indices := make([]int64, 0, len(node.Indices))
		indexLabels := make([]string, 0, len(node.Indices))
		allIntIndices := true
		allIndexIndices := true
		for _, idxExpr := range node.Indices {
			index, err := i.evalExpr(env, pkgName, idxExpr)
			if err != nil {
				return evalResult{}, err
			}
			if index.hasError {
				return evalResult{hasError: true, errorVal: index.errorVal}, nil
			}
			if index.value.Kind == ValueInt && index.value.Dimension.IsDimensionless() {
				indices = append(indices, index.value.Int)
				allIndexIndices = false
				continue
			}
			if index.value.Kind == ValueIndex {
				indexLabels = append(indexLabels, index.value.Text)
				allIntIndices = false
				continue
			}
			return evalResult{}, fmt.Errorf("runtime invariant violation: index must be Int or Index, got %s", valueTypeName(index.value))
		}
		switch target.value.Kind {
		case ValueArray:
			if !allIntIndices {
				return evalResult{}, fmt.Errorf("runtime invariant violation: array indexing requires Int index")
			}
			if len(indices) != 1 {
				return evalResult{}, fmt.Errorf("runtime invariant violation: array indexing requires exactly 1 index, got %d", len(indices))
			}
			if indices[0] < 0 || indices[0] >= int64(len(target.value.Array)) {
				return evalResult{}, fmt.Errorf("runtime error: index %d out of bounds for array of length %d", indices[0], len(target.value.Array))
			}
			return evalResult{value: target.value.Array[indices[0]]}, nil
		case ValueBytes:
			if !allIntIndices {
				return evalResult{}, fmt.Errorf("runtime invariant violation: bytes indexing requires Int index")
			}
			if len(indices) != 1 {
				return evalResult{}, fmt.Errorf("runtime invariant violation: bytes indexing requires exactly 1 index, got %d", len(indices))
			}
			if indices[0] < 0 || indices[0] >= int64(len(target.value.Bytes)) {
				return evalResult{}, fmt.Errorf("runtime error: index %d out of bounds for bytes of length %d", indices[0], len(target.value.Bytes))
			}
			return evalResult{value: Value{Kind: ValueInt, Int: int64(target.value.Bytes[indices[0]])}}, nil
		case ValueVector:
			if !allIntIndices {
				return evalResult{}, fmt.Errorf("runtime invariant violation: vector indexing requires Int index")
			}
			if len(indices) != 1 {
				return evalResult{}, fmt.Errorf("runtime invariant violation: vector indexing requires exactly 1 index, got %d", len(indices))
			}
			if indices[0] < 0 || indices[0] >= int64(len(target.value.Vector)) {
				return evalResult{}, fmt.Errorf("runtime error: index %d out of bounds for vector of length %d", indices[0], len(target.value.Vector))
			}
			return evalResult{value: target.value.Vector[indices[0]]}, nil
		case ValueMatrix:
			if allIndexIndices {
				if len(indexLabels) != 2 {
					return evalResult{}, fmt.Errorf("runtime invariant violation: matrix Einstein term access requires exactly 2 indices, got %d", len(indexLabels))
				}
				if strings.TrimSpace(indexLabels[0]) == "" || strings.TrimSpace(indexLabels[1]) == "" {
					return evalResult{}, fmt.Errorf("runtime error: Einstein indices must be non-empty")
				}
				if indexLabels[0] == indexLabels[1] {
					return evalResult{}, fmt.Errorf("runtime error: trace-style contraction '[%s,%s]' is not supported in M3; use Trace(...) for now", indexLabels[0], indexLabels[1])
				}
				return evalResult{einTerm: &einsteinIndexedTerm{matrix: target.value, labels: indexLabels}}, nil
			}
			if !allIntIndices {
				return evalResult{}, fmt.Errorf("runtime invariant violation: matrix indexing requires either Int,Int element access or Index,Index Einstein term access")
			}
			if len(indices) != 2 {
				return evalResult{}, fmt.Errorf("runtime invariant violation: matrix indexing requires exactly 2 indices, got %d", len(indices))
			}
			r, c := indices[0], indices[1]
			if r < 0 || r >= int64(target.value.Matrix.Rows) || c < 0 || c >= int64(target.value.Matrix.Cols) {
				return evalResult{}, fmt.Errorf("runtime error: index [%d, %d] out of bounds for matrix of shape %dx%d", r, c, target.value.Matrix.Rows, target.value.Matrix.Cols)
			}
			return evalResult{value: target.value.Matrix.Elements[int(r)*target.value.Matrix.Cols+int(c)]}, nil
		case ValueDiffOp, ValueFieldOp:
			if !allIntIndices {
				return evalResult{}, fmt.Errorf("runtime invariant violation: representational field expression indexing requires Int indices")
			}
			if len(indices) == 0 || len(indices) > 2 {
				return evalResult{}, fmt.Errorf("runtime invariant violation: representational field expression indexing requires 1 or 2 indices, got %d", len(indices))
			}
			projected, err := buildFieldProjection(target.value, indices)
			if err != nil {
				return evalResult{}, err
			}
			return evalResult{value: projected}, nil
		default:
			return evalResult{}, fmt.Errorf("runtime invariant violation: cannot index value of kind %s", target.value.Kind)
		}
	case ast.FieldAccessExpr:
		if pkgIdentifier, ok := node.Target.(ast.IdentifierExpr); ok {
			functionKey := pkgIdentifier.Name + "." + node.Field
			if _, exists := i.functions[functionKey]; exists {
				return evalResult{value: Value{Kind: ValueFunc, Function: FunctionValue{Key: functionKey}}}, nil
			}
		}
		if identifier, ok := node.Target.(ast.IdentifierExpr); ok {
			if enumDecl, enumTypeName, enumExists := i.lookupEnumDecl(pkgName, identifier.Name); enumExists {
				for _, variant := range enumDecl.Variants {
					if variant == node.Field {
						return evalResult{value: Value{Kind: ValueEnum, Enum: EnumValue{TypeName: enumTypeName, Variant: node.Field}}}, nil
					}
				}
				return evalResult{}, fmt.Errorf("runtime invariant violation: enum '%s' has no variant '%s'", identifier.Name, node.Field)
			}
		}
		if enumName, ok := flattenQualifiedEnumTarget(node.Target); ok {
			if enumDecl, enumTypeName, enumExists := i.lookupEnumDecl(pkgName, enumName); enumExists {
				for _, variant := range enumDecl.Variants {
					if variant == node.Field {
						return evalResult{value: Value{Kind: ValueEnum, Enum: EnumValue{TypeName: enumTypeName, Variant: node.Field}}}, nil
					}
				}
				return evalResult{}, fmt.Errorf("runtime invariant violation: enum '%s' has no variant '%s'", enumName, node.Field)
			}
		}
		target, err := i.evalExpr(env, pkgName, node.Target)
		if err != nil {
			return evalResult{}, err
		}
		if target.hasError {
			return evalResult{hasError: true, errorVal: target.errorVal}, nil
		}
		if target.value.Kind == ValueMatrix {
			switch node.Field {
			case "rows":
				return evalResult{value: Value{Kind: ValueInt, Int: int64(target.value.Matrix.Rows)}}, nil
			case "cols":
				return evalResult{value: Value{Kind: ValueInt, Int: int64(target.value.Matrix.Cols)}}, nil
			default:
				return evalResult{}, fmt.Errorf("runtime invariant violation: type 'Matrix' has no field '%s'", node.Field)
			}
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
		return i.evalExpr(env, pkgName, node.Inner)
	case ast.BinaryExpr:
		left, err := i.evalExpr(env, pkgName, node.Left)
		if err != nil {
			return evalResult{}, err
		}
		if left.hasError {
			return evalResult{hasError: true, errorVal: left.errorVal}, nil
		}
		if node.Operator == "and" {
			if left.value.Kind != ValueBool {
				return evalResult{}, fmt.Errorf("runtime invariant violation: operator 'and' requires Bool operands")
			}
			if !left.value.Bool {
				return evalResult{value: Value{Kind: ValueBool, Bool: false}}, nil
			}
		}
		if node.Operator == "or" {
			if left.value.Kind != ValueBool {
				return evalResult{}, fmt.Errorf("runtime invariant violation: operator 'or' requires Bool operands")
			}
			if left.value.Bool {
				return evalResult{value: Value{Kind: ValueBool, Bool: true}}, nil
			}
		}
		right, err := i.evalExpr(env, pkgName, node.Right)
		if err != nil {
			return evalResult{}, err
		}
		if right.hasError {
			return evalResult{hasError: true, errorVal: right.errorVal}, nil
		}
		if left.einTerm != nil || right.einTerm != nil {
			result, labels, err := evalEinsteinIndexedBinaryExpr(node.Operator, left, right)
			if err != nil {
				return evalResult{}, err
			}
			return evalResult{
				value:   result,
				einTerm: &einsteinIndexedTerm{matrix: result, labels: labels},
			}, nil
		}
		value, err := evalBinaryExpr(node.Operator, left.value, right.value)
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: value}, nil
	case ast.UnaryExpr:
		operand, err := i.evalExpr(env, pkgName, node.Operand)
		if err != nil {
			return evalResult{}, err
		}
		if operand.hasError {
			return evalResult{hasError: true, errorVal: operand.errorVal}, nil
		}
		switch node.Operator {
		case "not":
			if operand.value.Kind != ValueBool {
				return evalResult{}, fmt.Errorf("runtime invariant violation: operator 'not' requires Bool operand")
			}
			return evalResult{value: Value{Kind: ValueBool, Bool: !operand.value.Bool}}, nil
		case "-":
			switch operand.value.Kind {
			case ValueInt:
				return evalResult{value: Value{Kind: ValueInt, Int: -operand.value.Int, Dimension: operand.value.Dimension}}, nil
			case ValueFloat:
				return evalResult{value: Value{Kind: ValueFloat, Float: -operand.value.Float, Dimension: operand.value.Dimension}}, nil
			default:
				return evalResult{}, fmt.Errorf("runtime invariant violation: operator '-' requires Int or Float operand")
			}
		default:
			return evalResult{}, fmt.Errorf("runtime invariant violation: unsupported unary operator %q", node.Operator)
		}
	case ast.RangeExpr:
		value, err := i.evalRangeExpr(env, pkgName, node)
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: value}, nil
	case ast.PropagateExpr:
		inner, err := i.evalExpr(env, pkgName, node.Inner)
		if err != nil {
			return evalResult{}, err
		}
		if inner.hasError {
			return inner, nil
		}
		return evalResult{value: inner.value}, nil
	case ast.UnwrapExpr:
		inner, err := i.evalExpr(env, pkgName, node.Inner)
		if err != nil {
			return evalResult{}, err
		}
		if inner.hasError {
			return evalResult{}, fmt.Errorf("fatal error: %s", inner.errorVal.Error.Message)
		}
		return evalResult{value: inner.value}, nil
	case ast.SwitchExpr:
		return i.evalSwitchExpr(env, pkgName, node)
	case ast.IfExpr:
		return i.evalIfExpr(env, pkgName, node)
	case ast.UtilityWhenExpr:
		return i.evalUtilityWhenExpr(env, pkgName, node)
	case ast.BatchExpr:
		return i.evalBatchExpr(env, pkgName, node)
	default:
		return evalResult{}, fmt.Errorf("runtime invariant violation: unsupported expression %T", expr)
	}
}

func (i interpreter) evalBatchExpr(env *environment, pkgName string, expr ast.BatchExpr) (evalResult, error) {
	input, err := i.evalExpr(env, pkgName, expr.Input)
	if err != nil {
		return evalResult{}, err
	}
	if input.hasError {
		return evalResult{hasError: true, errorVal: input.errorVal}, nil
	}
	if input.value.Kind != ValueArray {
		return evalResult{}, fmt.Errorf("runtime invariant violation: batch input must be array, got %s", input.value.Kind)
	}

	items := input.value.Array
	if len(items) == 0 {
		return evalResult{value: Value{Kind: ValueArray, Array: []Value{}}}, nil
	}

	type batchItemResult struct {
		index    int
		value    Value
		hasError bool
		errorVal Value
		err      error
	}

	workerCount := runtime.GOMAXPROCS(0)
	if workerCount < 1 {
		workerCount = 1
	}
	if workerCount > len(items) {
		workerCount = len(items)
	}

	jobs := make(chan int, len(items))
	results := make(chan batchItemResult, len(items))
	var wg sync.WaitGroup
	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				itemEnv := snapshotEnvironment(env)
				itemEnv.define(expr.ItemName, items[index], false)
				outcome, execErr := i.executeBlock(itemEnv, pkgName, expr.Body)
				if execErr != nil {
					results <- batchItemResult{index: index, err: execErr}
					continue
				}
				if !outcome.returned {
					results <- batchItemResult{index: index, err: fmt.Errorf("runtime invariant violation: batch body must return exactly one value per item")}
					continue
				}
				if outcome.value.Kind == ValueError {
					results <- batchItemResult{index: index, hasError: true, errorVal: outcome.value}
					continue
				}
				results <- batchItemResult{index: index, value: cloneValue(outcome.value)}
			}
		}()
	}

	for idx := range items {
		jobs <- idx
	}
	close(jobs)
	wg.Wait()
	close(results)

	output := make([]Value, len(items))
	var firstError error
	var firstErrorVal Value
	hasErrorValue := false
	for result := range results {
		if firstError == nil && result.err != nil {
			firstError = result.err
			continue
		}
		if firstError == nil && result.hasError {
			hasErrorValue = true
			firstErrorVal = result.errorVal
			continue
		}
		output[result.index] = result.value
	}
	if firstError != nil {
		return evalResult{}, firstError
	}
	if hasErrorValue {
		return evalResult{hasError: true, errorVal: firstErrorVal}, nil
	}
	return evalResult{value: Value{Kind: ValueArray, Array: output}}, nil
}

func (i interpreter) evalIfExpr(env *environment, pkgName string, expr ast.IfExpr) (evalResult, error) {
	condition, err := i.evalExpr(env, pkgName, expr.Condition)
	if err != nil {
		return evalResult{}, err
	}
	if condition.hasError {
		return evalResult{hasError: true, errorVal: condition.errorVal}, nil
	}
	if condition.value.Kind != ValueBool {
		return evalResult{}, fmt.Errorf("runtime invariant violation: if expression condition must be Bool, got %s", condition.value.Kind)
	}
	if condition.value.Bool {
		return i.evalExpr(env, pkgName, expr.ThenExpr)
	}
	return i.evalExpr(env, pkgName, expr.ElseExpr)
}

func (i interpreter) evalUtilityWhenExpr(env *environment, pkgName string, expr ast.UtilityWhenExpr) (evalResult, error) {
	var instance *FlowRuntimeInstance
	if expr.ControllerBound {
		flowBinding, ok := env.lookup(flowInstanceBindingName)
		if !ok || flowBinding.value.Kind != ValueFlow || flowBinding.value.Flow == nil {
			return evalResult{}, fmt.Errorf("runtime invariant violation: utility when requires flow instance context")
		}
		instance = flowBinding.value.Flow
	}

	hysteresisResult, err := i.evalExpr(env, pkgName, expr.Policy.Hysteresis)
	if err != nil {
		return evalResult{}, err
	}
	if hysteresisResult.hasError {
		return evalResult{hasError: true, errorVal: hysteresisResult.errorVal}, nil
	}
	if hysteresisResult.value.Kind != ValueInt {
		return evalResult{}, fmt.Errorf("runtime invariant violation: utility when policy hysteresis must be Int, got %s", hysteresisResult.value.Kind)
	}

	minCommitResult, err := i.evalExpr(env, pkgName, expr.Policy.MinCommit)
	if err != nil {
		return evalResult{}, err
	}
	if minCommitResult.hasError {
		return evalResult{hasError: true, errorVal: minCommitResult.errorVal}, nil
	}
	if minCommitResult.value.Kind != ValueInt {
		return evalResult{}, fmt.Errorf("runtime invariant violation: utility when policy min_commit must be Int, got %s", minCommitResult.value.Kind)
	}
	hysteresis := hysteresisResult.value.Int
	minCommit := minCommitResult.value.Int

	type candidate struct {
		value Value
		score int64
	}
	validCandidates := make([]candidate, 0, len(expr.Cases))
	for _, whenCase := range expr.Cases {
		condition, err := i.evalExpr(env, pkgName, whenCase.Condition)
		if err != nil {
			return evalResult{}, err
		}
		if condition.hasError {
			return evalResult{hasError: true, errorVal: condition.errorVal}, nil
		}
		if condition.value.Kind != ValueBool {
			return evalResult{}, fmt.Errorf("runtime invariant violation: utility when case condition must be Bool, got %s", condition.value.Kind)
		}
		if !condition.value.Bool {
			continue
		}

		scoreResult, err := i.evalExpr(env, pkgName, whenCase.Score)
		if err != nil {
			return evalResult{}, err
		}
		if scoreResult.hasError {
			return evalResult{hasError: true, errorVal: scoreResult.errorVal}, nil
		}
		if scoreResult.value.Kind != ValueInt {
			return evalResult{}, fmt.Errorf("runtime invariant violation: utility when case score must be Int, got %s", scoreResult.value.Kind)
		}

		valueResult, err := i.evalExpr(env, pkgName, whenCase.Value)
		if err != nil {
			return evalResult{}, err
		}
		if valueResult.hasError {
			return evalResult{hasError: true, errorVal: valueResult.errorVal}, nil
		}
		validCandidates = append(validCandidates, candidate{value: valueResult.value, score: scoreResult.value.Int})
	}

	var next candidate
	hasSelection := false
	if len(validCandidates) == 0 {
		elseValue, err := i.evalExpr(env, pkgName, expr.Else)
		if err != nil {
			return evalResult{}, err
		}
		if elseValue.hasError {
			return evalResult{hasError: true, errorVal: elseValue.errorVal}, nil
		}
		next = candidate{value: elseValue.value, score: 0}
		hasSelection = true
	} else {
		next = validCandidates[0]
		hasSelection = true
		for _, c := range validCandidates[1:] {
			if c.score > next.score {
				next = c
			}
		}
	}
	if !hasSelection {
		return evalResult{}, fmt.Errorf("runtime invariant violation: utility when could not select value")
	}

	if expr.ControllerBound {
		siteState := instance.UtilityWhenSites[expr.SiteID]
		if siteState.HasCurrent {
			currentStillValid := false
			for _, c := range validCandidates {
				if valuesEqual(c.value, siteState.Current) {
					currentStillValid = true
					break
				}
			}
			if currentStillValid {
				commitActive := siteState.CommitAge < minCommit
				hysteresisBlocks := next.score <= siteState.Score+hysteresis
				if commitActive || hysteresisBlocks {
					next = candidate{value: siteState.Current, score: siteState.Score}
				}
			}
		}
		updated := siteState
		if !updated.HasCurrent || !valuesEqual(updated.Current, next.value) {
			updated.HasCurrent = true
			updated.Current = cloneValue(next.value)
			updated.Score = next.score
			updated.CommitAge = 1
		} else {
			updated.Score = next.score
			updated.CommitAge++
		}
		instance.UtilityWhenSites[expr.SiteID] = updated
	}

	return evalResult{value: next.value}, nil
}

func (i interpreter) evalSwitchExpr(env *environment, pkgName string, expr ast.SwitchExpr) (evalResult, error) {
	if expr.Subject == nil {
		for _, switchCase := range expr.Cases {
			condition, err := i.evalExpr(env, pkgName, switchCase.Match)
			if err != nil {
				return evalResult{}, err
			}
			if condition.hasError {
				return evalResult{hasError: true, errorVal: condition.errorVal}, nil
			}
			if condition.value.Kind != ValueBool {
				return evalResult{}, fmt.Errorf("runtime invariant violation: condition switch case must be Bool, got %s", condition.value.Kind)
			}
			if !condition.value.Bool {
				continue
			}
			return i.evalExpr(env, pkgName, switchCase.Value)
		}
		if expr.Else != nil {
			return i.evalExpr(env, pkgName, expr.Else)
		}
		return evalResult{}, fmt.Errorf("runtime invariant violation: condition switch requires else arm")
	}

	subject, err := i.evalExpr(env, pkgName, expr.Subject)
	if err != nil {
		return evalResult{}, err
	}
	if subject.hasError {
		return evalResult{hasError: true, errorVal: subject.errorVal}, nil
	}

	for _, switchCase := range expr.Cases {
		matched, err := i.switchCaseMatches(env, pkgName, subject.value, switchCase.Match)
		if err != nil {
			return evalResult{}, err
		}
		if !matched {
			continue
		}
		return i.evalExpr(env, pkgName, switchCase.Value)
	}
	if expr.Else != nil {
		return i.evalExpr(env, pkgName, expr.Else)
	}
	return evalResult{}, fmt.Errorf("runtime invariant violation: non-exhaustive switch without else")
}

func (i interpreter) switchCaseMatches(env *environment, pkgName string, subject Value, matchExpr ast.Expr) (bool, error) {
	caseValueResult, err := i.evalExpr(env, pkgName, matchExpr)
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
	case ValueEnum:
		return subject.Enum.TypeName == caseValue.Enum.TypeName && subject.Enum.Variant == caseValue.Enum.Variant, nil
	default:
		return false, fmt.Errorf("runtime invariant violation: unsupported switch subject kind %s", subject.Kind)
	}
}

func (i interpreter) evalCallExpr(env *environment, pkgName string, expr ast.CallExpr) (evalResult, error) {
	calleeName, hasDirectName := flattenDirectCallName(expr.Callee)
	if hasDirectName && calleeName == "error" {
		if len(expr.TypeArguments) != 0 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: error() does not accept type arguments")
		}
		if len(expr.Arguments) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: error() expects 1 argument")
		}
		messageValue, err := i.evalExpr(env, pkgName, expr.Arguments[0])
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
	if hasDirectName && builtin.IsName(calleeName) {
		return i.evalBuiltinCallExpr(env, pkgName, calleeName, expr.TypeArguments, expr.Arguments)
	}
	if hasDirectName && strings.HasPrefix(calleeName, "Assert.") {
		if len(expr.TypeArguments) != 0 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Assert functions do not accept type arguments")
		}
		return i.evalAssertCallExpr(env, pkgName, calleeName, expr.Arguments)
	}

	functionKey := ""
	if hasDirectName {
		functionKey = calleeName
		if !strings.Contains(functionKey, ".") {
			functionKey = pkgName + "." + functionKey
		}
	} else {
		calleeValue, err := i.evalExpr(env, pkgName, expr.Callee)
		if err != nil {
			return evalResult{}, err
		}
		if calleeValue.hasError {
			return evalResult{hasError: true, errorVal: calleeValue.errorVal}, nil
		}
		if calleeValue.value.Kind != ValueFunc {
			return evalResult{}, fmt.Errorf("runtime error: call target must be function value")
		}
		functionKey = calleeValue.value.Function.Key
	}
	function, ok := i.functions[functionKey]

	arguments := make([]Value, 0, len(expr.Arguments))
	for _, argumentExpr := range expr.Arguments {
		argument, err := i.evalExpr(env, pkgName, argumentExpr)
		if err != nil {
			return evalResult{}, err
		}
		if argument.hasError {
			return evalResult{hasError: true, errorVal: argument.errorVal}, nil
		}
		arguments = append(arguments, argument.value)
	}
	if flowDecl, isFlow := i.flows[functionKey]; isFlow {
		targetPkg := pkgName
		if dot := strings.Index(functionKey, "."); dot >= 0 {
			targetPkg = functionKey[:dot]
		}
		instance := i.instantiateFlow(flowDecl, targetPkg, arguments)
		return evalResult{value: Value{Kind: ValueFlow, Flow: instance}}, nil
	}
	if !ok {
		return evalResult{}, fmt.Errorf("runtime invariant violation: undefined function %s", functionKey)
	}

	targetPkg := pkgName
	if dot := strings.Index(functionKey, "."); dot >= 0 {
		targetPkg = functionKey[:dot]
	}
	result, err := i.executeFunction(function, targetPkg, arguments)
	if err != nil {
		return evalResult{}, err
	}
	if result.hasError {
		return evalResult{hasError: true, errorVal: result.errorVal}, nil
	}
	if targetPkg != pkgName {
		result.value = qualifyCrossPackageValue(result.value, targetPkg)
	}
	return evalResult{value: result.value}, nil
}

func (i interpreter) evalAssertCallExpr(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	fail := func(message string) (evalResult, error) {
		return evalResult{}, fmt.Errorf("assertion failed: %s", message)
	}

	evalArg := func(index int) (evalResult, error) {
		argument, err := i.evalExpr(env, pkgName, argumentExprs[index])
		if err != nil {
			return evalResult{}, err
		}
		return argument, nil
	}

	switch callee {
	case "Assert.True":
		condition, err := evalArg(0)
		if err != nil {
			return evalResult{}, err
		}
		if condition.hasError {
			return evalResult{hasError: true, errorVal: condition.errorVal}, nil
		}
		message, err := evalArg(1)
		if err != nil {
			return evalResult{}, err
		}
		if message.hasError {
			return evalResult{hasError: true, errorVal: message.errorVal}, nil
		}
		if !condition.value.Bool {
			return fail(message.value.Text)
		}
	case "Assert.False":
		condition, err := evalArg(0)
		if err != nil {
			return evalResult{}, err
		}
		if condition.hasError {
			return evalResult{hasError: true, errorVal: condition.errorVal}, nil
		}
		message, err := evalArg(1)
		if err != nil {
			return evalResult{}, err
		}
		if message.hasError {
			return evalResult{hasError: true, errorVal: message.errorVal}, nil
		}
		if condition.value.Bool {
			return fail(message.value.Text)
		}
	case "Assert.Equal":
		expected, err := evalArg(0)
		if err != nil {
			return evalResult{}, err
		}
		if expected.hasError {
			return evalResult{hasError: true, errorVal: expected.errorVal}, nil
		}
		actual, err := evalArg(1)
		if err != nil {
			return evalResult{}, err
		}
		if actual.hasError {
			return evalResult{hasError: true, errorVal: actual.errorVal}, nil
		}
		message, err := evalArg(2)
		if err != nil {
			return evalResult{}, err
		}
		if message.hasError {
			return evalResult{hasError: true, errorVal: message.errorVal}, nil
		}
		if !valuesEqual(expected.value, actual.value) {
			return fail(message.value.Text)
		}
	case "Assert.Near":
		expected, err := evalArg(0)
		if err != nil {
			return evalResult{}, err
		}
		if expected.hasError {
			return evalResult{hasError: true, errorVal: expected.errorVal}, nil
		}
		actual, err := evalArg(1)
		if err != nil {
			return evalResult{}, err
		}
		if actual.hasError {
			return evalResult{hasError: true, errorVal: actual.errorVal}, nil
		}
		tolerance, err := evalArg(2)
		if err != nil {
			return evalResult{}, err
		}
		if tolerance.hasError {
			return evalResult{hasError: true, errorVal: tolerance.errorVal}, nil
		}
		message, err := evalArg(3)
		if err != nil {
			return evalResult{}, err
		}
		if message.hasError {
			return evalResult{hasError: true, errorVal: message.errorVal}, nil
		}
		if math.Abs(expected.value.Float-actual.value.Float) > tolerance.value.Float {
			return fail(message.value.Text)
		}
	case "Assert.Error":
		result, err := evalArg(0)
		if err != nil {
			return evalResult{}, err
		}
		message, err := evalArg(1)
		if err != nil {
			return evalResult{}, err
		}
		if message.hasError {
			return evalResult{hasError: true, errorVal: message.errorVal}, nil
		}
		if !result.hasError {
			return fail(message.value.Text)
		}
	case "Assert.LGTM":
		result, err := evalArg(0)
		if err != nil {
			return evalResult{}, err
		}
		message, err := evalArg(1)
		if err != nil {
			return evalResult{}, err
		}
		if message.hasError {
			return evalResult{hasError: true, errorVal: message.errorVal}, nil
		}
		if result.hasError {
			return fail(fmt.Sprintf("%s\nunderlying error: %s", message.value.Text, result.errorVal))
		}
		return evalResult{value: result.value}, nil
	default:
		return evalResult{}, fmt.Errorf("runtime invariant violation: unsupported Assert function %s", callee)
	}
	return evalResult{}, nil
}

func valuesEqual(left Value, right Value) bool {
	if left.Kind != right.Kind || left.Dimension != right.Dimension {
		return false
	}
	switch left.Kind {
	case ValueBool:
		return left.Bool == right.Bool
	case ValueInt:
		return left.Int == right.Int
	case ValueFloat:
		return left.Float == right.Float
	case ValueComplex:
		return left.Complex == right.Complex
	case ValueString:
		return left.Text == right.Text
	case ValueEnum:
		return left.Enum == right.Enum
	default:
		return false
	}
}

func qualifyCrossPackageValue(value Value, pkgName string) Value {
	switch value.Kind {
	case ValueRecord:
		for fieldName, fieldValue := range value.Record.Fields {
			value.Record.Fields[fieldName] = qualifyCrossPackageValue(fieldValue, pkgName)
		}
	case ValueEnum:
		if !strings.Contains(value.Enum.TypeName, ".") {
			value.Enum.TypeName = pkgName + "." + value.Enum.TypeName
		}
	case ValueArray:
		for index := range value.Array {
			value.Array[index] = qualifyCrossPackageValue(value.Array[index], pkgName)
		}
	case ValueVector:
		for index := range value.Vector {
			value.Vector[index] = qualifyCrossPackageValue(value.Vector[index], pkgName)
		}
	case ValueMatrix:
		for index := range value.Matrix.Elements {
			value.Matrix.Elements[index] = qualifyCrossPackageValue(value.Matrix.Elements[index], pkgName)
		}
	case ValueDiffOp:
		if value.DiffOp.Operand != nil {
			operand := qualifyCrossPackageValue(*value.DiffOp.Operand, pkgName)
			value.DiffOp.Operand = &operand
		}
	case ValueFieldOp:
		if value.FieldOp.Left != nil {
			left := qualifyCrossPackageValue(*value.FieldOp.Left, pkgName)
			value.FieldOp.Left = &left
		}
		if value.FieldOp.Right != nil {
			right := qualifyCrossPackageValue(*value.FieldOp.Right, pkgName)
			value.FieldOp.Right = &right
		}
	}
	return value
}

func (i interpreter) evalBuiltinCallExpr(env *environment, pkgName string, callee string, typeArguments []ast.TypeRef, argumentExprs []ast.Expr) (evalResult, error) {
	if callee == "PlotLine" || callee == "PlotScatter" {
		if len(typeArguments) != 0 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: %s does not accept type arguments", callee)
		}
		value, err := i.evalPlotBuiltinCallExpr(env, pkgName, callee, argumentExprs)
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: value}, nil
	}
	if callee == "WriteOctagon" {
		if len(typeArguments) != 0 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: WriteOctagon does not accept type arguments")
		}
		return i.evalWriteOctagonBuiltinCallExpr(env, pkgName, argumentExprs)
	}
	if callee == "LoadOctagon" {
		return i.evalLoadOctagonBuiltinCallExpr(env, pkgName, typeArguments, argumentExprs)
	}
	if callee == "JsonLower" {
		return i.evalJSONLowerBuiltinCallExpr(env, pkgName, typeArguments, argumentExprs)
	}
	if callee == "JsonLoadStructured" {
		return i.evalJSONLoadStructuredBuiltinCallExpr(env, pkgName, typeArguments, argumentExprs)
	}
	if i.wrappers.has(callee) {
		if len(typeArguments) != 0 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: %s does not accept type arguments", callee)
		}
		return i.wrappers.eval(&i, env, pkgName, callee, argumentExprs)
	}
	if callee == "UIText" || callee == "UIButton" || callee == "UIColumn" || callee == "UIRow" || callee == "UICanvas" || callee == "UIGrid" || callee == "UISpacer" || callee == "UIPlaceAbsolute" || callee == "UIPlaceAnchored" || callee == "UIMount" || callee == "UIPatch" || callee == "UIUnmount" || callee == "UIEmit" || callee == "UIDrainEvents" || callee == "UISignature" {
		if len(typeArguments) != 0 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: %s does not accept type arguments", callee)
		}
		return i.evalUIBuiltinCallExpr(env, pkgName, callee, argumentExprs)
	}
	if callee == "Append" {
		if len(typeArguments) != 0 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Append does not accept type arguments")
		}
		return i.evalAppendBuiltinCallExpr(env, pkgName, argumentExprs)
	}
	if callee == "Step" {
		if len(typeArguments) != 0 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Step does not accept type arguments")
		}
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Step expects 1 argument")
		}
		argument, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if argument.hasError {
			return evalResult{hasError: true, errorVal: argument.errorVal}, nil
		}
		if argument.value.Kind != ValueFlow || argument.value.Flow == nil {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Step expects FlowInstance argument")
		}
		if err := i.stepFlow(argument.value.Flow); err != nil {
			return evalResult{}, err
		}
		return evalResult{value: Value{}}, nil
	}
	if callee == "Active" {
		if len(typeArguments) != 0 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Active does not accept type arguments")
		}
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Active expects 1 argument")
		}
		argument, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if argument.hasError {
			return evalResult{hasError: true, errorVal: argument.errorVal}, nil
		}
		if argument.value.Kind != ValueFlow || argument.value.Flow == nil {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Active expects FlowInstance argument")
		}
		return evalResult{value: Value{Kind: ValueString, Text: argument.value.Flow.CurrentState}}, nil
	}
	if callee == "Result" {
		if len(typeArguments) != 0 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Result does not accept type arguments")
		}
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Result expects 1 argument")
		}
		argument, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if argument.hasError {
			return evalResult{hasError: true, errorVal: argument.errorVal}, nil
		}
		if argument.value.Kind != ValueFlow || argument.value.Flow == nil {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Result expects FlowInstance argument")
		}
		if !argument.value.Flow.Completed {
			return evalResult{hasError: true, errorVal: Value{Kind: ValueError, Error: ErrorValue{Message: "Result() called before flow completion"}}}, nil
		}
		return evalResult{value: argument.value.Flow.Result}, nil
	}
	if callee == "Complete" {
		if len(typeArguments) != 0 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Complete does not accept type arguments")
		}
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Complete expects 1 argument")
		}
		argument, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if argument.hasError {
			return evalResult{hasError: true, errorVal: argument.errorVal}, nil
		}
		if argument.value.Kind != ValueFlow || argument.value.Flow == nil {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Complete expects FlowInstance argument")
		}
		return evalResult{value: Value{Kind: ValueBool, Bool: argument.value.Flow.Completed}}, nil
	}
	if callee == "StateHistory" {
		if len(typeArguments) != 0 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: StateHistory does not accept type arguments")
		}
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: StateHistory expects 1 argument")
		}
		argument, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if argument.hasError {
			return evalResult{hasError: true, errorVal: argument.errorVal}, nil
		}
		if argument.value.Kind != ValueFlow || argument.value.Flow == nil {
			return evalResult{}, fmt.Errorf("runtime invariant violation: StateHistory expects FlowInstance argument")
		}
		return evalResult{value: flowStateHistoryValue(argument.value.Flow)}, nil
	}
	if callee == "ResumeTarget" {
		if len(typeArguments) != 0 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: ResumeTarget does not accept type arguments")
		}
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: ResumeTarget expects 1 argument")
		}
		argument, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if argument.hasError {
			return evalResult{hasError: true, errorVal: argument.errorVal}, nil
		}
		if argument.value.Kind != ValueFlow || argument.value.Flow == nil {
			return evalResult{}, fmt.Errorf("runtime invariant violation: ResumeTarget expects FlowInstance argument")
		}
		target := ""
		if argument.value.Flow.HasResumeTarget {
			target = argument.value.Flow.ResumeTarget
		}
		return evalResult{value: Value{Kind: ValueString, Text: target}}, nil
	}
	if callee == "fft" {
		if len(typeArguments) != 0 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: fft does not accept type arguments")
		}
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: fft expects 1 argument")
		}
		argument, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if argument.hasError {
			return evalResult{hasError: true, errorVal: argument.errorVal}, nil
		}
		if argument.value.Kind != ValueArray {
			return evalResult{}, fmt.Errorf("runtime invariant violation: fft expects Complex[] argument")
		}
		transformed, err := fftCPU(argument.value.Array)
		if err != nil {
			return evalResult{hasError: true, errorVal: Value{Kind: ValueError, Error: ErrorValue{Message: err.Error()}}}, nil
		}
		return evalResult{value: Value{Kind: ValueArray, Array: transformed}}, nil
	}
	if len(typeArguments) != 0 && callee != "Matrix.zeros" && callee != "Matrix.identity" {
		return evalResult{}, fmt.Errorf("runtime invariant violation: %s does not accept type arguments", callee)
	}

	if callee == "Pi" || callee == "E" {
		if len(argumentExprs) != 0 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects 0 arguments", callee)
		}
		if callee == "Pi" {
			return evalResult{value: Value{Kind: ValueFloat, Float: math.Pi}}, nil
		}
		return evalResult{value: Value{Kind: ValueFloat, Float: math.E}}, nil
	}
	if callee == "I" {
		if len(argumentExprs) != 0 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects 0 arguments", callee)
		}
		return evalResult{value: Value{Kind: ValueComplex, Complex: complex(0, 1)}}, nil
	}
	if callee == "Complex" {
		if len(argumentExprs) != 2 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects 2 arguments", callee)
		}
		reResult, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if reResult.hasError {
			return evalResult{hasError: true, errorVal: reResult.errorVal}, nil
		}
		imResult, err := i.evalExpr(env, pkgName, argumentExprs[1])
		if err != nil {
			return evalResult{}, err
		}
		if imResult.hasError {
			return evalResult{hasError: true, errorVal: imResult.errorVal}, nil
		}
		if !reResult.value.Dimension.IsDimensionless() || !imResult.value.Dimension.IsDimensionless() {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Complex requires dimensionless input")
		}
		reValue, err := numericValueAsFloat(reResult.value, "Complex")
		if err != nil {
			return evalResult{}, err
		}
		imValue, err := numericValueAsFloat(imResult.value, "Complex")
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: Value{Kind: ValueComplex, Complex: complex(reValue, imValue)}}, nil
	}
	if callee == "ComplexPolar" {
		if len(argumentExprs) != 2 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects 2 arguments", callee)
		}
		rResult, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if rResult.hasError {
			return evalResult{hasError: true, errorVal: rResult.errorVal}, nil
		}
		thetaResult, err := i.evalExpr(env, pkgName, argumentExprs[1])
		if err != nil {
			return evalResult{}, err
		}
		if thetaResult.hasError {
			return evalResult{hasError: true, errorVal: thetaResult.errorVal}, nil
		}
		if !rResult.value.Dimension.IsDimensionless() || !thetaResult.value.Dimension.IsDimensionless() {
			return evalResult{}, fmt.Errorf("runtime invariant violation: ComplexPolar requires dimensionless input")
		}
		rValue, err := numericValueAsFloat(rResult.value, "ComplexPolar")
		if err != nil {
			return evalResult{}, err
		}
		thetaValue, err := numericValueAsFloat(thetaResult.value, "ComplexPolar")
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: Value{Kind: ValueComplex, Complex: cmplx.Rect(rValue, thetaValue)}}, nil
	}
	if callee == "Atan2" {
		if len(argumentExprs) != 2 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects 2 arguments", callee)
		}
		yArgument, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if yArgument.hasError {
			return evalResult{hasError: true, errorVal: yArgument.errorVal}, nil
		}
		xArgument, err := i.evalExpr(env, pkgName, argumentExprs[1])
		if err != nil {
			return evalResult{}, err
		}
		if xArgument.hasError {
			return evalResult{hasError: true, errorVal: xArgument.errorVal}, nil
		}
		if !yArgument.value.Dimension.IsDimensionless() || !xArgument.value.Dimension.IsDimensionless() {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Atan2 requires dimensionless input")
		}
		yValue, err := numericValueAsFloat(yArgument.value, "Atan2")
		if err != nil {
			return evalResult{}, err
		}
		xValue, err := numericValueAsFloat(xArgument.value, "Atan2")
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: Value{Kind: ValueFloat, Float: math.Atan2(yValue, xValue)}}, nil
	}
	if callee == "FormatFloat" {
		if len(argumentExprs) != 2 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects 2 arguments", callee)
		}
		valueResult, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if valueResult.hasError {
			return evalResult{hasError: true, errorVal: valueResult.errorVal}, nil
		}
		precisionResult, err := i.evalExpr(env, pkgName, argumentExprs[1])
		if err != nil {
			return evalResult{}, err
		}
		if precisionResult.hasError {
			return evalResult{hasError: true, errorVal: precisionResult.errorVal}, nil
		}
		if valueResult.value.Kind != ValueFloat || precisionResult.value.Kind != ValueInt {
			return evalResult{}, fmt.Errorf("runtime invariant violation: FormatFloat expects (Float, Int)")
		}
		if precisionResult.value.Int < 0 {
			return evalResult{}, fmt.Errorf("runtime error: FormatFloat precision must be >= 0, got %d", precisionResult.value.Int)
		}
		return evalResult{value: Value{Kind: ValueString, Text: strconv.FormatFloat(valueResult.value.Float, 'f', int(precisionResult.value.Int), 64)}}, nil
	}
	if callee == "ToString" {
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects 1 argument", callee)
		}
		valueResult, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if valueResult.hasError {
			return evalResult{hasError: true, errorVal: valueResult.errorVal}, nil
		}
		switch valueResult.value.Kind {
		case ValueInt:
			return evalResult{value: Value{Kind: ValueString, Text: strconv.FormatInt(valueResult.value.Int, 10)}}, nil
		case ValueFloat:
			return evalResult{value: Value{Kind: ValueString, Text: strconv.FormatFloat(valueResult.value.Float, 'g', -1, 64)}}, nil
		case ValueBool:
			return evalResult{value: Value{Kind: ValueString, Text: strconv.FormatBool(valueResult.value.Bool)}}, nil
		default:
			return evalResult{}, fmt.Errorf("runtime invariant violation: ToString expects Int, Float, or Bool")
		}
	}
	if callee == "Float" {
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects 1 argument", callee)
		}
		valueResult, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if valueResult.hasError {
			return evalResult{hasError: true, errorVal: valueResult.errorVal}, nil
		}
		if valueResult.value.Kind != ValueInt {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Float expects Int")
		}
		return evalResult{value: Value{Kind: ValueFloat, Float: float64(valueResult.value.Int), Dimension: valueResult.value.Dimension}}, nil
	}
	if callee == "Contains" || callee == "StartsWith" || callee == "EndsWith" {
		if len(argumentExprs) != 2 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects 2 arguments", callee)
		}
		textResult, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if textResult.hasError {
			return evalResult{hasError: true, errorVal: textResult.errorVal}, nil
		}
		partResult, err := i.evalExpr(env, pkgName, argumentExprs[1])
		if err != nil {
			return evalResult{}, err
		}
		if partResult.hasError {
			return evalResult{hasError: true, errorVal: partResult.errorVal}, nil
		}
		if textResult.value.Kind != ValueString || partResult.value.Kind != ValueString {
			return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects (String, String)", callee)
		}
		switch callee {
		case "Contains":
			return evalResult{value: Value{Kind: ValueBool, Bool: strings.Contains(textResult.value.Text, partResult.value.Text)}}, nil
		case "StartsWith":
			return evalResult{value: Value{Kind: ValueBool, Bool: strings.HasPrefix(textResult.value.Text, partResult.value.Text)}}, nil
		default:
			return evalResult{value: Value{Kind: ValueBool, Bool: strings.HasSuffix(textResult.value.Text, partResult.value.Text)}}, nil
		}
	}
	if callee == "Trim" || callee == "Lower" || callee == "Upper" {
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects 1 argument", callee)
		}
		textResult, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if textResult.hasError {
			return evalResult{hasError: true, errorVal: textResult.errorVal}, nil
		}
		if textResult.value.Kind != ValueString {
			return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects String argument", callee)
		}
		switch callee {
		case "Trim":
			return evalResult{value: Value{Kind: ValueString, Text: strings.TrimSpace(textResult.value.Text)}}, nil
		case "Lower":
			return evalResult{value: Value{Kind: ValueString, Text: strings.ToLower(textResult.value.Text)}}, nil
		default:
			return evalResult{value: Value{Kind: ValueString, Text: strings.ToUpper(textResult.value.Text)}}, nil
		}
	}
	if callee == "Join" {
		if len(argumentExprs) != 2 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Join expects 2 arguments")
		}
		partsResult, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if partsResult.hasError {
			return evalResult{hasError: true, errorVal: partsResult.errorVal}, nil
		}
		sepResult, err := i.evalExpr(env, pkgName, argumentExprs[1])
		if err != nil {
			return evalResult{}, err
		}
		if sepResult.hasError {
			return evalResult{hasError: true, errorVal: sepResult.errorVal}, nil
		}
		if partsResult.value.Kind != ValueArray || sepResult.value.Kind != ValueString {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Join expects (String[], String)")
		}
		parts := make([]string, 0, len(partsResult.value.Array))
		for _, value := range partsResult.value.Array {
			if value.Kind != ValueString {
				return evalResult{}, fmt.Errorf("runtime invariant violation: Join expects String[] as first argument")
			}
			parts = append(parts, value.Text)
		}
		return evalResult{value: Value{Kind: ValueString, Text: strings.Join(parts, sepResult.value.Text)}}, nil
	}
	if callee == "EinMul" || callee == "EinAdd" {
		if len(argumentExprs) != 6 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects 6 arguments", callee)
		}
		leftResult, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if leftResult.hasError {
			return evalResult{hasError: true, errorVal: leftResult.errorVal}, nil
		}
		rightResult, err := i.evalExpr(env, pkgName, argumentExprs[3])
		if err != nil {
			return evalResult{}, err
		}
		if rightResult.hasError {
			return evalResult{hasError: true, errorVal: rightResult.errorVal}, nil
		}
		if leftResult.value.Kind != ValueMatrix || rightResult.value.Kind != ValueMatrix {
			return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects Matrix arguments in positions 1 and 4", callee)
		}
		leftLabels := make([]string, 2)
		rightLabels := make([]string, 2)
		for idx := 0; idx < 2; idx++ {
			indexResult, err := i.evalExpr(env, pkgName, argumentExprs[idx+1])
			if err != nil {
				return evalResult{}, err
			}
			if indexResult.hasError {
				return evalResult{hasError: true, errorVal: indexResult.errorVal}, nil
			}
			if indexResult.value.Kind != ValueIndex {
				return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects Index arguments in positions 2,3,5,6", callee)
			}
			leftLabels[idx] = indexResult.value.Text
		}
		for idx := 0; idx < 2; idx++ {
			indexResult, err := i.evalExpr(env, pkgName, argumentExprs[idx+4])
			if err != nil {
				return evalResult{}, err
			}
			if indexResult.hasError {
				return evalResult{hasError: true, errorVal: indexResult.errorVal}, nil
			}
			if indexResult.value.Kind != ValueIndex {
				return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects Index arguments in positions 2,3,5,6", callee)
			}
			rightLabels[idx] = indexResult.value.Text
		}
		result, _, err := evalEinsteinBinaryMatrices(callee, leftResult.value, leftLabels, rightResult.value, rightLabels)
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: result}, nil
	}
	if callee == "Trace" {
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Trace expects 1 argument")
		}
		matrixResult, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if matrixResult.hasError {
			return evalResult{hasError: true, errorVal: matrixResult.errorVal}, nil
		}
		if isRepresentationalFieldValue(matrixResult.value) {
			operandCopy := cloneValue(matrixResult.value)
			return evalResult{value: Value{Kind: ValueDiffOp, DiffOp: DifferentialOpValue{Operator: "Trace", Operand: &operandCopy}}}, nil
		}
		if matrixResult.value.Kind != ValueMatrix {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Trace expects Matrix argument")
		}
		rows := matrixResult.value.Matrix.Rows
		cols := matrixResult.value.Matrix.Cols
		if rows != cols {
			return evalResult{}, fmt.Errorf("runtime error: Trace requires square matrix, got %dx%d", rows, cols)
		}
		if rows == 0 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Trace requires non-empty matrix")
		}
		sum := matrixResult.value.Matrix.Elements[0]
		for idx := 1; idx < rows; idx++ {
			next := matrixResult.value.Matrix.Elements[idx*cols+idx]
			sum, err = evalBinaryExpr("+", sum, next)
			if err != nil {
				return evalResult{}, err
			}
		}
		return evalResult{value: sum}, nil
	}
	if callee == "Grad" {
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Grad expects 1 argument")
		}
		operand, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if operand.hasError {
			return evalResult{hasError: true, errorVal: operand.errorVal}, nil
		}
		if !(isNumericValue(operand.value) || operand.value.Kind == ValueVector) || operand.value.Kind == ValueArray || operand.value.Kind == ValueMatrix {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Grad expects numeric Scalar or Vector argument")
		}
		operandCopy := operand.value
		return evalResult{value: Value{Kind: ValueDiffOp, DiffOp: DifferentialOpValue{Operator: "Grad", Operand: &operandCopy}}}, nil
	}
	if callee == "Div" {
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Div expects 1 argument")
		}
		operand, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if operand.hasError {
			return evalResult{hasError: true, errorVal: operand.errorVal}, nil
		}
		if !(operand.value.Kind == ValueVector || operand.value.Kind == ValueMatrix || isRepresentationalFieldValue(operand.value)) {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Div expects numeric Vector or Matrix argument")
		}
		operandCopy := operand.value
		return evalResult{value: Value{Kind: ValueDiffOp, DiffOp: DifferentialOpValue{Operator: "Div", Operand: &operandCopy}}}, nil
	}
	if callee == "SymGrad" {
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: SymGrad expects 1 argument")
		}
		operand, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if operand.hasError {
			return evalResult{hasError: true, errorVal: operand.errorVal}, nil
		}
		if operand.value.Kind != ValueVector {
			return evalResult{}, fmt.Errorf("runtime invariant violation: SymGrad expects numeric Vector argument")
		}
		operandCopy := operand.value
		return evalResult{value: Value{Kind: ValueDiffOp, DiffOp: DifferentialOpValue{Operator: "SymGrad", Operand: &operandCopy}}}, nil
	}
	if callee == "Idx" {
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Idx expects 1 argument")
		}
		nameResult, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if nameResult.hasError {
			return evalResult{hasError: true, errorVal: nameResult.errorVal}, nil
		}
		if nameResult.value.Kind != ValueString {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Idx expects String argument")
		}
		if strings.TrimSpace(nameResult.value.Text) == "" {
			return evalResult{}, fmt.Errorf("runtime error: Idx requires non-empty index name")
		}
		return evalResult{value: Value{Kind: ValueIndex, Text: nameResult.value.Text}}, nil
	}
	if callee == "Matrix.tabulate" {
		if len(argumentExprs) != 3 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Matrix.tabulate expects 3 arguments")
		}
		rowsResult, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if rowsResult.hasError {
			return evalResult{hasError: true, errorVal: rowsResult.errorVal}, nil
		}
		colsResult, err := i.evalExpr(env, pkgName, argumentExprs[1])
		if err != nil {
			return evalResult{}, err
		}
		if colsResult.hasError {
			return evalResult{hasError: true, errorVal: colsResult.errorVal}, nil
		}
		if rowsResult.value.Kind != ValueInt || colsResult.value.Kind != ValueInt || !rowsResult.value.Dimension.IsDimensionless() || !colsResult.value.Dimension.IsDimensionless() {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Matrix.tabulate expects Int dimensions")
		}
		if rowsResult.value.Int < 0 || colsResult.value.Int < 0 {
			return evalResult{}, fmt.Errorf("runtime error: Matrix.tabulate dimensions must be non-negative")
		}
		callbackResult, err := i.evalExpr(env, pkgName, argumentExprs[2])
		if err != nil {
			return evalResult{}, err
		}
		if callbackResult.hasError {
			return evalResult{hasError: true, errorVal: callbackResult.errorVal}, nil
		}
		if callbackResult.value.Kind != ValueFunc {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Matrix.tabulate expects callback function")
		}
		rows := int(rowsResult.value.Int)
		cols := int(colsResult.value.Int)
		elements := make([]Value, rows*cols)
		for r := 0; r < rows; r++ {
			for cIdx := 0; cIdx < cols; cIdx++ {
				callback, ok := i.functions[callbackResult.value.Function.Key]
				if !ok {
					return evalResult{}, fmt.Errorf("runtime invariant violation: undefined function %s", callbackResult.value.Function.Key)
				}
				callPkg := pkgName
				if dot := strings.Index(callbackResult.value.Function.Key, "."); dot >= 0 {
					callPkg = callbackResult.value.Function.Key[:dot]
				}
				callResult, err := i.executeFunction(callback, callPkg, []Value{
					{Kind: ValueInt, Int: int64(r)},
					{Kind: ValueInt, Int: int64(cIdx)},
				})
				if err != nil {
					return evalResult{}, err
				}
				if callResult.hasError {
					return evalResult{hasError: true, errorVal: callResult.errorVal}, nil
				}
				elements[r*cols+cIdx] = callResult.value
			}
		}
		return evalResult{value: Value{Kind: ValueMatrix, Matrix: MatrixValue{Rows: rows, Cols: cols, Elements: elements}}}, nil
	}
	if callee == "Matrix.zeros" {
		if len(typeArguments) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Matrix.zeros expects 1 type argument")
		}
		if len(argumentExprs) != 2 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Matrix.zeros expects 2 arguments")
		}
		rowsResult, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if rowsResult.hasError {
			return evalResult{hasError: true, errorVal: rowsResult.errorVal}, nil
		}
		colsResult, err := i.evalExpr(env, pkgName, argumentExprs[1])
		if err != nil {
			return evalResult{}, err
		}
		if colsResult.hasError {
			return evalResult{hasError: true, errorVal: colsResult.errorVal}, nil
		}
		if rowsResult.value.Kind != ValueInt || colsResult.value.Kind != ValueInt || !rowsResult.value.Dimension.IsDimensionless() || !colsResult.value.Dimension.IsDimensionless() {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Matrix.zeros expects Int dimensions")
		}
		if rowsResult.value.Int < 0 || colsResult.value.Int < 0 {
			return evalResult{}, fmt.Errorf("runtime error: Matrix.zeros dimensions must be non-negative")
		}
		zero := Value{Kind: ValueInt, Int: 0}
		if typeArguments[0].Name == "Float" {
			zero = Value{Kind: ValueFloat, Float: 0.0}
		}
		rows := int(rowsResult.value.Int)
		cols := int(colsResult.value.Int)
		elements := make([]Value, rows*cols)
		for idx := range elements {
			elements[idx] = cloneValue(zero)
		}
		return evalResult{value: Value{Kind: ValueMatrix, Matrix: MatrixValue{Rows: rows, Cols: cols, Elements: elements}}}, nil
	}
	if callee == "Matrix.fill" {
		if len(argumentExprs) != 3 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Matrix.fill expects 3 arguments")
		}
		rowsResult, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if rowsResult.hasError {
			return evalResult{hasError: true, errorVal: rowsResult.errorVal}, nil
		}
		colsResult, err := i.evalExpr(env, pkgName, argumentExprs[1])
		if err != nil {
			return evalResult{}, err
		}
		if colsResult.hasError {
			return evalResult{hasError: true, errorVal: colsResult.errorVal}, nil
		}
		valueResult, err := i.evalExpr(env, pkgName, argumentExprs[2])
		if err != nil {
			return evalResult{}, err
		}
		if valueResult.hasError {
			return evalResult{hasError: true, errorVal: valueResult.errorVal}, nil
		}
		if rowsResult.value.Kind != ValueInt || colsResult.value.Kind != ValueInt || !rowsResult.value.Dimension.IsDimensionless() || !colsResult.value.Dimension.IsDimensionless() {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Matrix.fill expects Int dimensions")
		}
		if rowsResult.value.Int < 0 || colsResult.value.Int < 0 {
			return evalResult{}, fmt.Errorf("runtime error: Matrix.fill dimensions must be non-negative")
		}
		rows := int(rowsResult.value.Int)
		cols := int(colsResult.value.Int)
		elements := make([]Value, rows*cols)
		for idx := range elements {
			elements[idx] = cloneValue(valueResult.value)
		}
		return evalResult{value: Value{Kind: ValueMatrix, Matrix: MatrixValue{Rows: rows, Cols: cols, Elements: elements}}}, nil
	}
	if callee == "PrometheusMatMul" {
		if len(typeArguments) != 0 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: PrometheusMatMul does not accept type arguments")
		}
		if len(argumentExprs) != 2 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: PrometheusMatMul expects 2 arguments")
		}
		leftResult, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if leftResult.hasError {
			return evalResult{hasError: true, errorVal: leftResult.errorVal}, nil
		}
		rightResult, err := i.evalExpr(env, pkgName, argumentExprs[1])
		if err != nil {
			return evalResult{}, err
		}
		if rightResult.hasError {
			return evalResult{hasError: true, errorVal: rightResult.errorVal}, nil
		}
		if leftResult.value.Kind != ValueMatrix || rightResult.value.Kind != ValueMatrix {
			return evalResult{}, fmt.Errorf("runtime invariant violation: PrometheusMatMul expects matrix arguments")
		}
		out, err := evalMatrixMultiply(leftResult.value, rightResult.value)
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: out}, nil
	}

	if callee == "Matrix.identity" {
		if len(typeArguments) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Matrix.identity expects 1 type argument")
		}
		if len(argumentExprs) != 1 {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Matrix.identity expects 1 argument")
		}
		sizeResult, err := i.evalExpr(env, pkgName, argumentExprs[0])
		if err != nil {
			return evalResult{}, err
		}
		if sizeResult.hasError {
			return evalResult{hasError: true, errorVal: sizeResult.errorVal}, nil
		}
		if sizeResult.value.Kind != ValueInt || !sizeResult.value.Dimension.IsDimensionless() {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Matrix.identity expects Int dimension")
		}
		if sizeResult.value.Int < 0 {
			return evalResult{}, fmt.Errorf("runtime error: Matrix.identity dimension must be non-negative")
		}
		n := int(sizeResult.value.Int)
		elements := make([]Value, n*n)
		zero := Value{Kind: ValueInt, Int: 0}
		one := Value{Kind: ValueInt, Int: 1}
		if typeArguments[0].Name == "Float" {
			zero = Value{Kind: ValueFloat, Float: 0.0}
			one = Value{Kind: ValueFloat, Float: 1.0}
		}
		for r := 0; r < n; r++ {
			for cIdx := 0; cIdx < n; cIdx++ {
				if r == cIdx {
					elements[r*n+cIdx] = cloneValue(one)
				} else {
					elements[r*n+cIdx] = cloneValue(zero)
				}
			}
		}
		return evalResult{value: Value{Kind: ValueMatrix, Matrix: MatrixValue{Rows: n, Cols: n, Elements: elements}}}, nil
	}

	if len(argumentExprs) != 1 {
		return evalResult{}, fmt.Errorf("runtime invariant violation: %s expects 1 argument", callee)
	}

	argument, err := i.evalExpr(env, pkgName, argumentExprs[0])
	if err != nil {
		return evalResult{}, err
	}
	if argument.hasError {
		return evalResult{hasError: true, errorVal: argument.errorVal}, nil
	}

	switch callee {
	case "Print":
		_, writeErr := fmt.Fprintln(i.stdout, argument.value.String())
		if writeErr != nil {
			return evalResult{}, fmt.Errorf("runtime error: write stdout: %w", writeErr)
		}
		return evalResult{value: Value{Kind: ValueInt, Int: 0}}, nil
	case "Len":
		if argument.value.Kind == ValueString {
			return evalResult{value: Value{Kind: ValueInt, Int: int64(len(argument.value.Text))}}, nil
		}
		if argument.value.Kind == ValueBytes {
			return evalResult{value: Value{Kind: ValueInt, Int: int64(len(argument.value.Bytes))}}, nil
		}
		if argument.value.Kind != ValueArray {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Len expects String, Bytes, or Array, got %s", argument.value.Kind)
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
		case ValueComplex:
			return evalResult{value: Value{Kind: ValueFloat, Float: cmplx.Abs(argument.value.Complex)}}, nil
		default:
			return evalResult{}, fmt.Errorf("runtime invariant violation: Abs expects Int, Float, or Complex, got %s", argument.value.Kind)
		}
	case "Real":
		if argument.value.Kind != ValueComplex {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Real expects Complex, got %s", argument.value.Kind)
		}
		return evalResult{value: Value{Kind: ValueFloat, Float: real(argument.value.Complex)}}, nil
	case "Imag":
		if argument.value.Kind != ValueComplex {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Imag expects Complex, got %s", argument.value.Kind)
		}
		return evalResult{value: Value{Kind: ValueFloat, Float: imag(argument.value.Complex)}}, nil
	case "Arg":
		if argument.value.Kind != ValueComplex {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Arg expects Complex, got %s", argument.value.Kind)
		}
		return evalResult{value: Value{Kind: ValueFloat, Float: math.Atan2(imag(argument.value.Complex), real(argument.value.Complex))}}, nil
	case "Conj":
		if argument.value.Kind != ValueComplex {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Conj expects Complex, got %s", argument.value.Kind)
		}
		return evalResult{value: Value{Kind: ValueComplex, Complex: cmplx.Conj(argument.value.Complex)}}, nil
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
	case "Tan":
		if !argument.value.Dimension.IsDimensionless() {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Tan requires dimensionless input")
		}
		value, err := numericValueAsFloat(argument.value, "Tan")
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: Value{Kind: ValueFloat, Float: math.Tan(value)}}, nil
	case "Asin":
		if !argument.value.Dimension.IsDimensionless() {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Asin requires dimensionless input")
		}
		value, err := numericValueAsFloat(argument.value, "Asin")
		if err != nil {
			return evalResult{}, err
		}
		if value < -1 || value > 1 {
			return evalResult{}, fmt.Errorf("runtime error: Asin expects input in [-1, 1], got %s", argument.value.String())
		}
		return evalResult{value: Value{Kind: ValueFloat, Float: math.Asin(value)}}, nil
	case "Acos":
		if !argument.value.Dimension.IsDimensionless() {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Acos requires dimensionless input")
		}
		value, err := numericValueAsFloat(argument.value, "Acos")
		if err != nil {
			return evalResult{}, err
		}
		if value < -1 || value > 1 {
			return evalResult{}, fmt.Errorf("runtime error: Acos expects input in [-1, 1], got %s", argument.value.String())
		}
		return evalResult{value: Value{Kind: ValueFloat, Float: math.Acos(value)}}, nil
	case "Atan":
		if !argument.value.Dimension.IsDimensionless() {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Atan requires dimensionless input")
		}
		value, err := numericValueAsFloat(argument.value, "Atan")
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: Value{Kind: ValueFloat, Float: math.Atan(value)}}, nil
	case "Exp":
		if argument.value.Kind == ValueComplex {
			return evalResult{value: Value{Kind: ValueComplex, Complex: cmplx.Exp(argument.value.Complex)}}, nil
		}
		if !argument.value.Dimension.IsDimensionless() {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Exp requires dimensionless input")
		}
		value, err := numericValueAsFloat(argument.value, "Exp")
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: Value{Kind: ValueFloat, Float: math.Exp(value)}}, nil
	case "Sinh":
		if !argument.value.Dimension.IsDimensionless() {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Sinh requires dimensionless input")
		}
		value, err := numericValueAsFloat(argument.value, "Sinh")
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: Value{Kind: ValueFloat, Float: math.Sinh(value)}}, nil
	case "Cosh":
		if !argument.value.Dimension.IsDimensionless() {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Cosh requires dimensionless input")
		}
		value, err := numericValueAsFloat(argument.value, "Cosh")
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: Value{Kind: ValueFloat, Float: math.Cosh(value)}}, nil
	case "Tanh":
		if !argument.value.Dimension.IsDimensionless() {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Tanh requires dimensionless input")
		}
		value, err := numericValueAsFloat(argument.value, "Tanh")
		if err != nil {
			return evalResult{}, err
		}
		return evalResult{value: Value{Kind: ValueFloat, Float: math.Tanh(value)}}, nil
	case "Ln":
		if argument.value.Kind == ValueComplex {
			return evalResult{value: Value{Kind: ValueComplex, Complex: cmplx.Log(argument.value.Complex)}}, nil
		}
		if !argument.value.Dimension.IsDimensionless() {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Ln requires dimensionless input")
		}
		value, err := numericValueAsFloat(argument.value, "Ln")
		if err != nil {
			return evalResult{}, err
		}
		if value <= 0 {
			return evalResult{}, fmt.Errorf("runtime error: Ln expects positive input, got %s", argument.value.String())
		}
		return evalResult{value: Value{Kind: ValueFloat, Float: math.Log(value)}}, nil
	case "Log10":
		if !argument.value.Dimension.IsDimensionless() {
			return evalResult{}, fmt.Errorf("runtime invariant violation: Log10 requires dimensionless input")
		}
		value, err := numericValueAsFloat(argument.value, "Log10")
		if err != nil {
			return evalResult{}, err
		}
		if value <= 0 {
			return evalResult{}, fmt.Errorf("runtime error: Log10 expects positive input, got %s", argument.value.String())
		}
		return evalResult{value: Value{Kind: ValueFloat, Float: math.Log10(value)}}, nil
	default:
		return evalResult{}, fmt.Errorf("runtime invariant violation: unsupported built-in function %s", callee)
	}
}

func (i interpreter) evalLoadOctagonBuiltinCallExpr(env *environment, pkgName string, typeArguments []ast.TypeRef, argumentExprs []ast.Expr) (evalResult, error) {
	if len(typeArguments) != 1 {
		return evalResult{}, fmt.Errorf("runtime invariant violation: LoadOctagon expects 1 type argument")
	}
	if len(argumentExprs) != 1 {
		return evalResult{}, fmt.Errorf("runtime invariant violation: LoadOctagon expects 1 argument")
	}
	pathValue, err := i.evalExpr(env, pkgName, argumentExprs[0])
	if err != nil {
		return evalResult{}, err
	}
	if pathValue.hasError {
		return evalResult{hasError: true, errorVal: pathValue.errorVal}, nil
	}
	if pathValue.value.Kind != ValueString {
		return evalResult{}, fmt.Errorf("runtime invariant violation: LoadOctagon expects String path argument")
	}

	rawValue, err := octagon.Load(pathValue.value.Text)
	if err != nil {
		return evalResult{hasError: true, errorVal: Value{Kind: ValueError, Error: ErrorValue{Message: fmt.Sprintf("LoadOctagon %s: %v", pathValue.value.Text, err)}}}, nil
	}
	typedValue, err := i.materializeOctagonValue(pkgName, typeArguments[0], rawValue)
	if err != nil {
		return evalResult{hasError: true, errorVal: Value{Kind: ValueError, Error: ErrorValue{Message: fmt.Sprintf("LoadOctagon %s: %v", pathValue.value.Text, err)}}}, nil
	}
	return evalResult{value: typedValue}, nil
}

func (i *interpreter) workbookByHandle(handle int64) (*xlsxWorkbook, error) {
	return i.workbooks.get(handle)
}

func xlsxWrapperBuiltins() map[string]wrapperBuiltinHandler {
	return map[string]wrapperBuiltinHandler{
		"XlsxCreateWorkbook": (*interpreter).evalXlsxCreateWorkbookBuiltin,
		"XlsxAddSheet":       (*interpreter).evalXlsxAddSheetBuiltin,
		"XlsxSetCellString":  (*interpreter).evalXlsxSetCellStringBuiltin,
		"XlsxSetCellFloat":   (*interpreter).evalXlsxSetCellFloatBuiltin,
		"XlsxSaveWorkbook":   (*interpreter).evalXlsxSaveWorkbookBuiltin,
	}
}

func (i *interpreter) evalXlsxCreateWorkbookBuiltin(_ *environment, _ string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, nil, "", callee, argumentExprs)
	if err := call.expectArity(0); err != nil {
		return evalResult{}, err
	}
	file := excelize.NewFile()
	file.DeleteSheet("Sheet1")
	handle := i.workbooks.allocate(&xlsxWorkbook{file: file})
	return wrapperIntResult(handle), nil
}

func (i *interpreter) evalXlsxAddSheetBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(2); err != nil {
		return evalResult{}, err
	}
	workbook, sheetName, errResult, err := i.evalWorkbookAndSheetArgs(call, 0, 1)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	sheetIndex, sheetErr := workbook.file.GetSheetIndex(sheetName)
	if sheetErr != nil {
		return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorBackendFailure, "%v", sheetErr)), nil
	}
	if sheetIndex != -1 {
		return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorConflict, "sheet %q already exists", sheetName)), nil
	}
	if _, err := workbook.file.NewSheet(sheetName); err != nil {
		return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorBackendFailure, "%v", err)), nil
	}
	return wrapperIntResult(0), nil
}

func (i *interpreter) evalXlsxSetCellStringBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(4); err != nil {
		return evalResult{}, err
	}
	workbook, sheetName, cell, errResult, err := i.evalWorkbookSheetCellArgs(call, 0, 1, 2)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	value, valueErrResult, err := call.stringArg(3)
	if err != nil {
		return evalResult{}, err
	}
	if valueErrResult != nil {
		return *valueErrResult, nil
	}
	sheetIndex, sheetErr := workbook.file.GetSheetIndex(sheetName)
	if sheetErr != nil {
		return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorBackendFailure, "%v", sheetErr)), nil
	}
	if sheetIndex == -1 {
		return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorNotFound, "unknown sheet %q", sheetName)), nil
	}
	if err := workbook.file.SetCellStr(sheetName, cell, value); err != nil {
		return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorBackendFailure, "%v", err)), nil
	}
	return wrapperIntResult(0), nil
}

func (i *interpreter) evalXlsxSetCellFloatBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(4); err != nil {
		return evalResult{}, err
	}
	workbook, sheetName, cell, errResult, err := i.evalWorkbookSheetCellArgs(call, 0, 1, 2)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	floatValue, valueErrResult, err := call.floatArg(3)
	if err != nil {
		return evalResult{}, err
	}
	if valueErrResult != nil {
		return *valueErrResult, nil
	}
	sheetIndex, sheetErr := workbook.file.GetSheetIndex(sheetName)
	if sheetErr != nil {
		return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorBackendFailure, "%v", sheetErr)), nil
	}
	if sheetIndex == -1 {
		return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorNotFound, "unknown sheet %q", sheetName)), nil
	}
	if err := workbook.file.SetCellFloat(sheetName, cell, floatValue, -1, 64); err != nil {
		return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorBackendFailure, "%v", err)), nil
	}
	return wrapperIntResult(0), nil
}

func (i *interpreter) evalXlsxSaveWorkbookBuiltin(env *environment, pkgName string, callee string, argumentExprs []ast.Expr) (evalResult, error) {
	call := newWrapperCall(i, env, pkgName, callee, argumentExprs)
	if err := call.expectArity(2); err != nil {
		return evalResult{}, err
	}
	workbook, path, errResult, err := i.evalWorkbookAndSheetArgs(call, 0, 1)
	if err != nil {
		return evalResult{}, err
	}
	if errResult != nil {
		return *errResult, nil
	}
	if !strings.HasSuffix(path, ".xlsx") {
		return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorInvalidArgument, "path must end with .xlsx")), nil
	}
	if len(workbook.file.GetSheetList()) == 0 {
		return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorInvalidData, "workbook has no sheets")), nil
	}
	if err := workbook.file.SaveAs(path); err != nil {
		return wrapperErrorResult(callee, wrapperErrorf(wrapperErrorBackendFailure, "%s: %v", path, err)), nil
	}
	return wrapperIntResult(0), nil
}

func (i *interpreter) evalWorkbookAndSheetArgs(call wrapperCall, workbookArgIndex int, sheetArgIndex int) (*xlsxWorkbook, string, *evalResult, error) {
	workbookHandle, errResult, err := call.intArg(workbookArgIndex)
	if err != nil {
		return nil, "", nil, err
	}
	if errResult != nil {
		return nil, "", errResult, nil
	}
	workbook, err := i.workbookByHandle(workbookHandle)
	if err != nil {
		errResult := wrapperErrorResult(call.callee, err)
		return nil, "", &errResult, nil
	}
	sheetName, errResult, err := call.stringArg(sheetArgIndex)
	if err != nil {
		return nil, "", nil, err
	}
	if errResult != nil {
		return nil, "", errResult, nil
	}
	return workbook, sheetName, nil, nil
}

func (i *interpreter) evalWorkbookSheetCellArgs(call wrapperCall, workbookArgIndex int, sheetArgIndex int, cellArgIndex int) (*xlsxWorkbook, string, string, *evalResult, error) {
	workbook, sheetName, errResult, err := i.evalWorkbookAndSheetArgs(call, workbookArgIndex, sheetArgIndex)
	if err != nil || errResult != nil {
		return nil, "", "", errResult, err
	}
	cellValue, errResult, err := call.stringArg(cellArgIndex)
	if err != nil {
		return nil, "", "", nil, err
	}
	if errResult != nil {
		return nil, "", "", errResult, nil
	}
	return workbook, sheetName, cellValue, nil, nil
}

func (i interpreter) evalAppendBuiltinCallExpr(env *environment, pkgName string, argumentExprs []ast.Expr) (evalResult, error) {
	if len(argumentExprs) != 2 {
		return evalResult{}, fmt.Errorf("runtime invariant violation: Append expects 2 arguments")
	}

	array, err := i.evalExpr(env, pkgName, argumentExprs[0])
	if err != nil {
		return evalResult{}, err
	}
	if array.hasError {
		return evalResult{hasError: true, errorVal: array.errorVal}, nil
	}
	if array.value.Kind != ValueArray {
		return evalResult{}, fmt.Errorf("runtime invariant violation: Append expects Array as first argument, got %s", array.value.Kind)
	}

	element, err := i.evalExpr(env, pkgName, argumentExprs[1])
	if err != nil {
		return evalResult{}, err
	}
	if element.hasError {
		return evalResult{hasError: true, errorVal: element.errorVal}, nil
	}

	if len(array.value.Array) > 0 && !sameValueType(array.value.Array[0], element.value) {
		return evalResult{}, fmt.Errorf("runtime invariant violation: Append element type mismatch: expected %s, got %s", valueTypeName(array.value.Array[0]), valueTypeName(element.value))
	}

	result := make([]Value, 0, len(array.value.Array)+1)
	result = append(result, array.value.Array...)
	result = append(result, element.value)
	return evalResult{value: Value{Kind: ValueArray, Array: result}}, nil
}

func (i interpreter) evalWriteOctagonBuiltinCallExpr(env *environment, pkgName string, argumentExprs []ast.Expr) (evalResult, error) {
	if len(argumentExprs) != 2 {
		return evalResult{}, fmt.Errorf("runtime invariant violation: WriteOctagon expects 2 arguments")
	}

	pathValue, err := i.evalExpr(env, pkgName, argumentExprs[0])
	if err != nil {
		return evalResult{}, err
	}
	if pathValue.hasError {
		return evalResult{hasError: true, errorVal: pathValue.errorVal}, nil
	}
	if pathValue.value.Kind != ValueString {
		return evalResult{}, fmt.Errorf("runtime invariant violation: WriteOctagon expects String path argument")
	}
	if !strings.HasSuffix(pathValue.value.Text, ".octagon") {
		return evalResult{}, fmt.Errorf("runtime error: WriteOctagon path must end with .octagon")
	}

	contentValue, err := i.evalExpr(env, pkgName, argumentExprs[1])
	if err != nil {
		return evalResult{}, err
	}
	if contentValue.hasError {
		return evalResult{hasError: true, errorVal: contentValue.errorVal}, nil
	}

	if err := WriteOctagon(pathValue.value.Text, contentValue.value); err != nil {
		return evalResult{}, fmt.Errorf("runtime error: %w", err)
	}
	return evalResult{value: Value{Kind: ValueInt, Int: 0}}, nil
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

func isNumericValue(value Value) bool {
	return value.Kind == ValueInt || value.Kind == ValueFloat || value.Kind == ValueComplex
}

func flowStateHistoryValue(instance *FlowRuntimeInstance) Value {
	history := make([]Value, 0, len(instance.StateHistory))
	for _, state := range instance.StateHistory {
		history = append(history, Value{Kind: ValueString, Text: state})
	}
	return Value{Kind: ValueArray, Array: history}
}

func (i interpreter) evalRangeExpr(env *environment, pkgName string, expr ast.RangeExpr) (Value, error) {
	start, err := i.evalExpr(env, pkgName, expr.Start)
	if err != nil {
		return Value{}, err
	}
	if start.hasError {
		return Value{}, fmt.Errorf("runtime invariant violation: unhandled error reached range start")
	}
	if start.value.Kind != ValueInt || !start.value.Dimension.IsDimensionless() {
		return Value{}, fmt.Errorf("runtime error: range start must be Int, got %s", valueTypeName(start.value))
	}
	end, err := i.evalExpr(env, pkgName, expr.End)
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
		stepValue, err := i.evalExpr(env, pkgName, expr.Step)
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

func (i interpreter) evalArrayLiteralExpr(env *environment, pkgName string, expr ast.ArrayLiteralExpr) (Value, error) {
	elements := make([]Value, 0, len(expr.Elements))
	var firstType string
	for idx, elementExpr := range expr.Elements {
		element, err := i.evalExpr(env, pkgName, elementExpr)
		if err != nil {
			return Value{}, err
		}
		if element.hasError {
			return Value{}, fmt.Errorf("runtime invariant violation: unhandled error reached array literal element %d", idx)
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

func (i interpreter) evalVectorLiteralExpr(env *environment, pkgName string, expr ast.VectorLiteralExpr) (Value, error) {
	if len(expr.Elements) == 0 {
		return Value{}, errors.New("runtime invariant violation: empty vector literals are not supported")
	}
	elements := make([]Value, 0, len(expr.Elements))
	var firstType string
	for idx, elementExpr := range expr.Elements {
		element, err := i.evalExpr(env, pkgName, elementExpr)
		if err != nil {
			return Value{}, err
		}
		if element.hasError {
			return Value{}, fmt.Errorf("runtime invariant violation: unhandled error reached vector literal element %d", idx)
		}
		if !isNumericValue(element.value) {
			return Value{}, fmt.Errorf("runtime invariant violation: vector elements must be numeric, got %s", valueTypeName(element.value))
		}
		if idx == 0 {
			firstType = valueTypeName(element.value)
		} else if valueTypeName(element.value) != firstType {
			return Value{}, errors.New("runtime invariant violation: Vector literals require homogeneous element type")
		}
		elements = append(elements, element.value)
	}
	return Value{Kind: ValueVector, Vector: elements}, nil
}

func (i interpreter) evalMatrixLiteralExpr(env *environment, pkgName string, expr ast.MatrixLiteralExpr) (Value, error) {
	if len(expr.Rows) == 0 {
		return Value{}, errors.New("runtime invariant violation: empty matrix literals are not supported")
	}
	cols := len(expr.Rows[0])
	if cols == 0 {
		return Value{}, errors.New("runtime invariant violation: matrix rows must not be empty")
	}
	elements := make([]Value, 0, len(expr.Rows)*cols)
	var firstType string
	for r, row := range expr.Rows {
		if len(row) != cols {
			return Value{}, errors.New("runtime invariant violation: matrix rows must all have equal length")
		}
		for c, elementExpr := range row {
			element, err := i.evalExpr(env, pkgName, elementExpr)
			if err != nil {
				return Value{}, err
			}
			if element.hasError {
				return Value{}, fmt.Errorf("runtime invariant violation: unhandled error reached matrix literal element [%d, %d]", r, c)
			}
			if !isNumericValue(element.value) {
				return Value{}, fmt.Errorf("runtime invariant violation: matrix elements must be numeric, got %s", valueTypeName(element.value))
			}
			if r == 0 && c == 0 {
				firstType = valueTypeName(element.value)
			} else if valueTypeName(element.value) != firstType {
				return Value{}, errors.New("runtime invariant violation: Matrix literals require homogeneous element type")
			}
			elements = append(elements, element.value)
		}
	}
	return Value{Kind: ValueMatrix, Matrix: MatrixValue{Rows: len(expr.Rows), Cols: cols, Elements: elements}}, nil
}

func (i interpreter) evalRecordLiteralExpr(env *environment, pkgName string, expr ast.RecordLiteralExpr) (evalResult, error) {
	recordDecl, resolvedTypeName, ok := i.lookupRecordDecl(pkgName, expr.TypeName)
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
		value, err := i.evalExpr(env, pkgName, field.Value)
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
	return evalResult{value: Value{Kind: ValueRecord, Record: RecordValue{TypeName: resolvedTypeName, FieldOrder: fieldOrder, Fields: fieldValues}}}, nil
}

func (i interpreter) evalRecordUpdateExpr(env *environment, pkgName string, expr ast.RecordUpdateExpr) (evalResult, error) {
	source, err := i.evalExpr(env, pkgName, expr.Source)
	if err != nil {
		return evalResult{}, err
	}
	if source.hasError {
		return evalResult{hasError: true, errorVal: source.errorVal}, nil
	}
	if source.value.Kind != ValueRecord {
		return evalResult{}, fmt.Errorf("runtime invariant violation: record update requires record value, got %s", valueTypeName(source.value))
	}

	fields := make(map[string]Value, len(source.value.Record.Fields))
	for name, value := range source.value.Record.Fields {
		fields[name] = value
	}
	for _, field := range expr.Fields {
		value, err := i.evalExpr(env, pkgName, field.Value)
		if err != nil {
			return evalResult{}, err
		}
		if value.hasError {
			return evalResult{hasError: true, errorVal: value.errorVal}, nil
		}
		fields[field.Name] = value.value
	}

	return evalResult{value: Value{Kind: ValueRecord, Record: RecordValue{TypeName: source.value.Record.TypeName, Fields: fields}}}, nil
}

func (i interpreter) lookupRecordDecl(currentPackage string, typeName string) (ast.RecordDecl, string, bool) {
	if pkgName, localName, ok := splitQualifiedTypeName(typeName); ok {
		recordDecl, exists := i.records[pkgName+"."+localName]
		return recordDecl, pkgName + "." + localName, exists
	}
	recordDecl, exists := i.records[currentPackage+"."+typeName]
	return recordDecl, typeName, exists
}

func (i interpreter) lookupEnumDecl(currentPackage string, typeName string) (ast.EnumDecl, string, bool) {
	if pkgName, localName, ok := splitQualifiedTypeName(typeName); ok {
		enumDecl, exists := i.enums[pkgName+"."+localName]
		return enumDecl, pkgName + "." + localName, exists
	}
	enumDecl, exists := i.enums[currentPackage+"."+typeName]
	return enumDecl, typeName, exists
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

func flattenQualifiedEnumTarget(expr ast.Expr) (string, bool) {
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

func evalBinaryExpr(operator string, left Value, right Value) (Value, error) {
	if operator == "and" || operator == "or" {
		if left.Kind != ValueBool || right.Kind != ValueBool {
			return Value{}, fmt.Errorf("runtime invariant violation: operator %q requires Bool operands", operator)
		}
		if operator == "and" {
			return Value{Kind: ValueBool, Bool: left.Bool && right.Bool}, nil
		}
		return Value{Kind: ValueBool, Bool: left.Bool || right.Bool}, nil
	}
	if isComparisonOperator(operator) {
		return evalComparisonExpr(operator, left, right)
	}
	if isRepresentationalFieldValue(left) || isRepresentationalFieldValue(right) {
		return evalRepresentationalFieldBinaryExpr(operator, left, right)
	}
	if left.Kind == ValueVector || right.Kind == ValueVector || left.Kind == ValueMatrix || right.Kind == ValueMatrix {
		return evalLinearBinaryExpr(operator, left, right)
	}
	if operator == "+" && left.Kind == ValueString && right.Kind == ValueString {
		return Value{Kind: ValueString, Text: left.Text + right.Text}, nil
	}
	if left.Kind == ValueRange || right.Kind == ValueRange || left.Kind == ValueString || right.Kind == ValueString || left.Kind == ValueBytes || right.Kind == ValueBytes || left.Kind == ValueError || right.Kind == ValueError || left.Kind == ValueEnum || right.Kind == ValueEnum || left.Kind == ValueRecord || right.Kind == ValueRecord {
		return Value{}, fmt.Errorf("runtime invariant violation: operator %q not defined for %s and %s", operator, valueTypeName(left), valueTypeName(right))
	}
	if left.Kind == ValueArray || right.Kind == ValueArray {
		return evalArrayBinaryExpr(operator, left, right)
	}
	if left.Kind == ValueBool || right.Kind == ValueBool {
		return Value{}, fmt.Errorf("runtime invariant violation: operator %q not defined for %s and %s", operator, valueTypeName(left), valueTypeName(right))
	}
	if left.Kind == ValueComplex || right.Kind == ValueComplex {
		return evalComplexBinaryExpr(operator, left, right)
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

func isRepresentationalFieldValue(value Value) bool {
	return value.Kind == ValueDiffOp || value.Kind == ValueFieldOp
}

func evalRepresentationalFieldBinaryExpr(operator string, left Value, right Value) (Value, error) {
	if !isLinearElementwiseOperator(operator) {
		return Value{}, fmt.Errorf("runtime invariant violation: operator %q not defined for %s and %s", operator, valueTypeName(left), valueTypeName(right))
	}
	if right.Kind == ValueArray || right.Kind == ValueRecord || right.Kind == ValueEnum || right.Kind == ValueError || right.Kind == ValueRange || right.Kind == ValueString || right.Kind == ValueBool {
		return Value{}, fmt.Errorf("runtime invariant violation: operator %q not defined for %s and %s", operator, valueTypeName(left), valueTypeName(right))
	}
	if left.Kind == ValueArray || left.Kind == ValueRecord || left.Kind == ValueEnum || left.Kind == ValueError || left.Kind == ValueRange || left.Kind == ValueString || left.Kind == ValueBool {
		return Value{}, fmt.Errorf("runtime invariant violation: operator %q not defined for %s and %s", operator, valueTypeName(left), valueTypeName(right))
	}
	leftCopy := cloneValue(left)
	rightCopy := cloneValue(right)
	return Value{Kind: ValueFieldOp, FieldOp: FieldOpValue{Operator: operator, Left: &leftCopy, Right: &rightCopy}}, nil
}

func buildFieldProjection(target Value, indices []int64) (Value, error) {
	labels := make([]string, 0, len(indices))
	for _, index := range indices {
		labels = append(labels, strconv.FormatInt(index, 10))
	}
	indexVector := Value{Kind: ValueVector, Vector: make([]Value, len(labels))}
	for i := range labels {
		indexVector.Vector[i] = Value{Kind: ValueString, Text: labels[i]}
	}
	indexVectorCopy := cloneValue(indexVector)
	targetCopy := cloneValue(target)
	return Value{Kind: ValueDiffOp, DiffOp: DifferentialOpValue{
		Operator: "Project",
		Operand: &Value{
			Kind: ValueFieldOp,
			FieldOp: FieldOpValue{
				Operator: ",",
				Left:     &targetCopy,
				Right:    &indexVectorCopy,
			},
		},
	}}, nil
}

func evalLinearBinaryExpr(operator string, left Value, right Value) (Value, error) {
	if operator == "@" {
		if left.Kind == ValueMatrix && right.Kind == ValueMatrix {
			return evalMatrixMultiply(left, right)
		}
		if left.Kind == ValueMatrix && right.Kind == ValueVector {
			return evalMatrixVectorMultiply(left, right)
		}
		return Value{}, fmt.Errorf("runtime invariant violation: operator '@' not defined for %s and %s", valueTypeName(left), valueTypeName(right))
	}
	if !isLinearElementwiseOperator(operator) {
		return Value{}, fmt.Errorf("runtime invariant violation: operator %q not defined for %s and %s", operator, valueTypeName(left), valueTypeName(right))
	}
	if (left.Kind == ValueVector || left.Kind == ValueMatrix) && isNumericValue(right) {
		return evalLinearScalarExpansion(operator, left, right)
	}
	if (right.Kind == ValueVector || right.Kind == ValueMatrix) && isNumericValue(left) {
		return evalLinearScalarExpansion(operator, right, left)
	}
	if left.Kind == ValueVector && right.Kind == ValueVector {
		if len(left.Vector) != len(right.Vector) {
			return Value{}, fmt.Errorf("runtime error: vector lengths must match; got %d and %d", len(left.Vector), len(right.Vector))
		}
		result := make([]Value, len(left.Vector))
		for i := range left.Vector {
			v, err := evalBinaryExpr(operator, left.Vector[i], right.Vector[i])
			if err != nil {
				return Value{}, err
			}
			result[i] = v
		}
		return Value{Kind: ValueVector, Vector: result}, nil
	}
	if left.Kind == ValueMatrix && right.Kind == ValueMatrix {
		if left.Matrix.Rows != right.Matrix.Rows || left.Matrix.Cols != right.Matrix.Cols {
			return Value{}, fmt.Errorf("runtime error: matrix shapes must match; got %dx%d and %dx%d", left.Matrix.Rows, left.Matrix.Cols, right.Matrix.Rows, right.Matrix.Cols)
		}
		result := make([]Value, len(left.Matrix.Elements))
		for i := range left.Matrix.Elements {
			v, err := evalBinaryExpr(operator, left.Matrix.Elements[i], right.Matrix.Elements[i])
			if err != nil {
				return Value{}, err
			}
			result[i] = v
		}
		return Value{Kind: ValueMatrix, Matrix: MatrixValue{Rows: left.Matrix.Rows, Cols: left.Matrix.Cols, Elements: result}}, nil
	}
	return Value{}, fmt.Errorf("runtime invariant violation: operator %q not defined for %s and %s", operator, valueTypeName(left), valueTypeName(right))
}

func isLinearElementwiseOperator(operator string) bool {
	return operator == "+" || operator == "-" || operator == "*" || operator == "/"
}

func evalLinearScalarExpansion(operator string, container Value, scalar Value) (Value, error) {
	if container.Kind == ValueVector {
		result := make([]Value, len(container.Vector))
		for i := range container.Vector {
			v, err := evalBinaryExpr(operator, container.Vector[i], scalar)
			if err != nil {
				return Value{}, err
			}
			result[i] = v
		}
		return Value{Kind: ValueVector, Vector: result}, nil
	}
	result := make([]Value, len(container.Matrix.Elements))
	for i := range container.Matrix.Elements {
		v, err := evalBinaryExpr(operator, container.Matrix.Elements[i], scalar)
		if err != nil {
			return Value{}, err
		}
		result[i] = v
	}
	return Value{Kind: ValueMatrix, Matrix: MatrixValue{Rows: container.Matrix.Rows, Cols: container.Matrix.Cols, Elements: result}}, nil
}

func evalMatrixVectorMultiply(left Value, right Value) (Value, error) {
	if left.Matrix.Cols != len(right.Vector) {
		return Value{}, fmt.Errorf("runtime error: matrix multiplication requires left cols = right rows; got %dx%d and %d", left.Matrix.Rows, left.Matrix.Cols, len(right.Vector))
	}
	result := make([]Value, left.Matrix.Rows)
	for r := 0; r < left.Matrix.Rows; r++ {
		acc := Value{}
		for c := 0; c < left.Matrix.Cols; c++ {
			term, err := evalBinaryExpr("*", left.Matrix.Elements[r*left.Matrix.Cols+c], right.Vector[c])
			if err != nil {
				return Value{}, err
			}
			if c == 0 {
				acc = term
			} else {
				acc, err = evalBinaryExpr("+", acc, term)
				if err != nil {
					return Value{}, err
				}
			}
		}
		result[r] = acc
	}
	return Value{Kind: ValueVector, Vector: result}, nil
}

func evalMatrixMultiply(left Value, right Value) (Value, error) {
	if left.Matrix.Cols != right.Matrix.Rows {
		return Value{}, fmt.Errorf("runtime error: matrix multiplication requires left cols = right rows; got %dx%d and %dx%d", left.Matrix.Rows, left.Matrix.Cols, right.Matrix.Rows, right.Matrix.Cols)
	}
	rows, cols, inner := left.Matrix.Rows, right.Matrix.Cols, left.Matrix.Cols
	result := make([]Value, rows*cols)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			acc := Value{}
			for k := 0; k < inner; k++ {
				term, err := evalBinaryExpr("*", left.Matrix.Elements[r*inner+k], right.Matrix.Elements[k*cols+c])
				if err != nil {
					return Value{}, err
				}
				if k == 0 {
					acc = term
				} else {
					acc, err = evalBinaryExpr("+", acc, term)
					if err != nil {
						return Value{}, err
					}
				}
			}
			result[r*cols+c] = acc
		}
	}
	return Value{Kind: ValueMatrix, Matrix: MatrixValue{Rows: rows, Cols: cols, Elements: result}}, nil
}

func evalArrayBinaryExpr(operator string, left Value, right Value) (Value, error) {
	if isComparisonOperator(operator) {
		return Value{}, fmt.Errorf("runtime invariant violation: operator %q not defined for %s and %s", operator, valueTypeName(left), valueTypeName(right))
	}
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

func evalComplexBinaryExpr(operator string, left Value, right Value) (Value, error) {
	leftComplex, err := asComplex(left)
	if err != nil {
		return Value{}, err
	}
	rightComplex, err := asComplex(right)
	if err != nil {
		return Value{}, err
	}
	if operator == "/" && rightComplex == 0 {
		return Value{}, errors.New("runtime error: division by zero")
	}
	switch operator {
	case "+":
		return Value{Kind: ValueComplex, Complex: leftComplex + rightComplex}, nil
	case "-":
		return Value{Kind: ValueComplex, Complex: leftComplex - rightComplex}, nil
	case "*":
		return Value{Kind: ValueComplex, Complex: leftComplex * rightComplex}, nil
	case "/":
		return Value{Kind: ValueComplex, Complex: leftComplex / rightComplex}, nil
	default:
		return Value{}, fmt.Errorf("runtime invariant violation: operator %q not defined for %s and %s", operator, valueTypeName(left), valueTypeName(right))
	}
}

func evalComparisonExpr(operator string, left Value, right Value) (Value, error) {
	if left.Kind == ValueArray || right.Kind == ValueArray || left.Kind == ValueBytes || right.Kind == ValueBytes || left.Kind == ValueVector || right.Kind == ValueVector || left.Kind == ValueMatrix || right.Kind == ValueMatrix || left.Kind == ValueRecord || right.Kind == ValueRecord || left.Kind == ValueRange || right.Kind == ValueRange || left.Kind == ValueError || right.Kind == ValueError {
		return Value{}, fmt.Errorf("runtime invariant violation: operator %q not defined for %s and %s", operator, valueTypeName(left), valueTypeName(right))
	}
	if (left.Kind == ValueInt || left.Kind == ValueFloat) && (right.Kind == ValueInt || right.Kind == ValueFloat) {
		if left.Dimension != right.Dimension {
			return Value{}, fmt.Errorf("runtime invariant violation: cannot compare %s and %s", valueTypeName(left), valueTypeName(right))
		}
		leftFloat, _ := asFloat(left)
		rightFloat, _ := asFloat(right)
		return Value{Kind: ValueBool, Bool: compareFloat(operator, leftFloat, rightFloat)}, nil
	}
	if isEqualityOperator(operator) {
		if left.Kind == ValueComplex && right.Kind == ValueComplex {
			equal := left.Complex == right.Complex
			if operator == "!=" {
				equal = !equal
			}
			return Value{Kind: ValueBool, Bool: equal}, nil
		}
		if left.Kind == ValueBool && right.Kind == ValueBool {
			equal := left.Bool == right.Bool
			if operator == "!=" {
				equal = !equal
			}
			return Value{Kind: ValueBool, Bool: equal}, nil
		}
		if left.Kind == ValueString && right.Kind == ValueString {
			equal := left.Text == right.Text
			if operator == "!=" {
				equal = !equal
			}
			return Value{Kind: ValueBool, Bool: equal}, nil
		}
		if left.Kind == ValueEnum && right.Kind == ValueEnum {
			if left.Enum.TypeName != right.Enum.TypeName {
				return Value{}, fmt.Errorf("runtime invariant violation: operator %q requires matching enum types", operator)
			}
			equal := left.Enum.Variant == right.Enum.Variant
			if operator == "!=" {
				equal = !equal
			}
			return Value{Kind: ValueBool, Bool: equal}, nil
		}
	}
	return Value{}, fmt.Errorf("runtime invariant violation: operator %q not defined for %s and %s", operator, valueTypeName(left), valueTypeName(right))
}

func compareFloat(operator string, left float64, right float64) bool {
	switch operator {
	case "==":
		return left == right
	case "!=":
		return left != right
	case "<":
		return left < right
	case "<=":
		return left <= right
	case ">":
		return left > right
	case ">=":
		return left >= right
	default:
		return false
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

func asComplex(value Value) (complex128, error) {
	switch value.Kind {
	case ValueInt:
		return complex(float64(value.Int), 0), nil
	case ValueFloat:
		return complex(value.Float, 0), nil
	case ValueComplex:
		return value.Complex, nil
	default:
		return 0, fmt.Errorf("runtime invariant violation: expected Int, Float, or Complex, got %s", value.Kind)
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
	if value.Kind == ValueVector {
		element := "Unknown"
		if len(value.Vector) > 0 {
			element = valueTypeName(value.Vector[0])
		}
		return "Vector<" + element + ">"
	}
	if value.Kind == ValueMatrix {
		element := "Unknown"
		if len(value.Matrix.Elements) > 0 {
			element = valueTypeName(value.Matrix.Elements[0])
		}
		return "Matrix<" + element + ">"
	}
	if value.Kind == ValueFlow {
		if value.Flow == nil {
			return "FlowInstance<invalid>"
		}
		return fmt.Sprintf("FlowInstance<%s>", value.Flow.Decl.ReturnType.Name)
	}
	if value.Kind == ValueFieldOp {
		return "FieldOp"
	}
	base := string(value.Kind)
	if (value.Kind == ValueInt || value.Kind == ValueFloat) && !value.Dimension.IsDimensionless() {
		base += "<" + value.Dimension.String() + ">"
	}
	return base
}

func sameValueType(left Value, right Value) bool {
	if left.Kind != right.Kind {
		return false
	}

	switch left.Kind {
	case ValueInt, ValueFloat:
		return left.Dimension == right.Dimension
	case ValueComplex:
		return true
	case ValueRecord:
		return left.Record.TypeName == right.Record.TypeName
	case ValueEnum:
		return left.Enum.TypeName == right.Enum.TypeName
	default:
		return true
	}
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

func isComparisonOperator(operator string) bool {
	return isEqualityOperator(operator) || operator == "<" || operator == "<=" || operator == ">" || operator == ">="
}

func isEqualityOperator(operator string) bool {
	return operator == "==" || operator == "!="
}
