package interpret

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/project"
)

const FlowCheckpointVersion = 1

const FlowCheckpointCursorTopLevelNext = "top-level-statement-next"

type FlowCheckpointUnsupportedReason string

const (
	FlowCheckpointNotSuspended                   FlowCheckpointUnsupportedReason = "NotSuspended"
	FlowCheckpointCompletedCheckpointUnsupported FlowCheckpointUnsupportedReason = "CompletedCheckpointUnsupported"
	FlowCheckpointStateLocalsUnsupported         FlowCheckpointUnsupportedReason = "StateLocalsUnsupported"
	FlowCheckpointUtilityStateUnsupported        FlowCheckpointUnsupportedReason = "UtilityStateUnsupported"
	FlowCheckpointBoardSnapshotUnsupported       FlowCheckpointUnsupportedReason = "BoardSnapshotUnsupported"
	FlowCheckpointStateMissing                   FlowCheckpointUnsupportedReason = "StateMissing"
	FlowCheckpointInstructionIndexOutOfRange     FlowCheckpointUnsupportedReason = "InstructionIndexOutOfRange"
	FlowCheckpointResumeTargetMissing            FlowCheckpointUnsupportedReason = "ResumeTargetMissing"
	FlowCheckpointUnsupportedValueType           FlowCheckpointUnsupportedReason = "UnsupportedValueType"
	FlowCheckpointUnsupportedCheckpointVersion   FlowCheckpointUnsupportedReason = "UnsupportedCheckpointVersion"
	FlowCheckpointPackageMismatch                FlowCheckpointUnsupportedReason = "PackageMismatch"
	FlowCheckpointFlowMismatch                   FlowCheckpointUnsupportedReason = "FlowMismatch"
	FlowCheckpointFlowFingerprintMismatch        FlowCheckpointUnsupportedReason = "FlowFingerprintMismatch"
	FlowCheckpointStateBodyChanged               FlowCheckpointUnsupportedReason = "StateBodyChanged"
	FlowCheckpointBoardSchemaMismatch            FlowCheckpointUnsupportedReason = "BoardSchemaMismatch"
	FlowCheckpointBoardValueTypeMismatch         FlowCheckpointUnsupportedReason = "BoardValueTypeMismatch"
	FlowCheckpointResumeCursorInvalid            FlowCheckpointUnsupportedReason = "ResumeCursorInvalid"
	FlowCheckpointStateHistoryInvalid            FlowCheckpointUnsupportedReason = "StateHistoryInvalid"
)

type FlowCheckpointError struct {
	Reason FlowCheckpointUnsupportedReason
	Detail string
}

func (e FlowCheckpointError) Error() string {
	if e.Detail == "" {
		return "flow checkpoint unsupported: " + string(e.Reason)
	}
	return fmt.Sprintf("flow checkpoint unsupported: %s: %s", e.Reason, e.Detail)
}

func IsFlowCheckpointUnsupported(err error, reason FlowCheckpointUnsupportedReason) bool {
	var checkpointErr FlowCheckpointError
	return errors.As(err, &checkpointErr) && checkpointErr.Reason == reason
}

type FlowCheckpointOptions struct{ StepCount int }

type FlowRestoreOptions struct{}

func InstantiateFlowFromCheckpoint(program project.Program, pkg string, flow string, checkpoint FlowCheckpoint, opts FlowRestoreOptions) (*FlowRuntimeInstance, error) {
	interpreter, err := newInterpreter(program, io.Discard)
	if err != nil {
		return nil, err
	}
	defer interpreter.close()
	return interpreter.instantiateFlowFromCheckpoint(pkg, flow, checkpoint, opts)
}

func RunFlowToCompletionFromCheckpointWithOptions(program project.Program, pkg string, flow string, checkpoint FlowCheckpoint, maxSteps int, stdout io.Writer, options ExecuteOptions) (FlowRunResult, error) {
	if maxSteps <= 0 {
		return FlowRunResult{}, fmt.Errorf("flow %q MaxSteps must be positive", flow)
	}
	interpreter, err := newInterpreter(program, stdout)
	if err != nil {
		return FlowRunResult{}, err
	}
	defer interpreter.close()
	interpreter.assertRecorder = options.AssertionRecorder
	interpreter.artifactProgressRecorder = options.ArtifactProgressRecorder
	interpreter.ctx = options.Context
	instance, err := interpreter.instantiateFlowFromCheckpoint(pkg, flow, checkpoint, FlowRestoreOptions{})
	if err != nil {
		return FlowRunResult{}, err
	}
	total := 0
	for !instance.Completed {
		if err := interpreter.checkCancelled(); err != nil {
			return flowRunResult(instance, total, false), err
		}
		remaining := maxSteps - total
		if remaining <= 0 {
			return flowRunResult(instance, total, false), fmt.Errorf("flow %q exceeded MaxSteps %d", flow, maxSteps)
		}
		steps, exhausted, suspended, err := interpreter.stepFlowWithTransitionLimit(instance, remaining)
		total += steps
		if err != nil {
			return flowRunResult(instance, total, false), err
		}
		if exhausted {
			return flowRunResult(instance, total, false), fmt.Errorf("flow %q exceeded MaxSteps %d", flow, maxSteps)
		}
		if suspended {
			return flowRunResult(instance, total, true), nil
		}
	}
	return flowRunResult(instance, total, false), nil
}

func (i interpreter) instantiateFlowFromCheckpoint(pkg string, flowName string, checkpoint FlowCheckpoint, opts FlowRestoreOptions) (*FlowRuntimeInstance, error) {
	if checkpoint.Version != FlowCheckpointVersion {
		return nil, checkpointErr(FlowCheckpointUnsupportedCheckpointVersion, fmt.Sprintf("version %d", checkpoint.Version))
	}
	if checkpoint.Package != pkg {
		return nil, checkpointErr(FlowCheckpointPackageMismatch, fmt.Sprintf("checkpoint %q requested %q", checkpoint.Package, pkg))
	}
	if checkpoint.Flow != flowName {
		return nil, checkpointErr(FlowCheckpointFlowMismatch, fmt.Sprintf("checkpoint %q requested %q", checkpoint.Flow, flowName))
	}
	key := pkg + "." + flowName
	flow, ok := i.flows[key]
	if !ok {
		return nil, checkpointErr(FlowCheckpointFlowMismatch, "missing flow "+key)
	}
	if len(flow.Parameters) != 0 {
		return nil, checkpointErr(FlowCheckpointStateLocalsUnsupported, "flow parameters are not checkpointed in H2")
	}
	if flow.ReturnType.Name != "Int" || flow.ReturnType.IsArray || flow.ReturnType.ArrayDepth > 0 {
		return nil, checkpointErr(FlowCheckpointFlowMismatch, "flow return shape is not compatible with H1 Make subset")
	}
	inst := i.instantiateFlow(flow, pkg, nil)
	if checkpoint.FlowFingerprint != "" && checkpoint.FlowFingerprint != flowFingerprint(inst) {
		return nil, checkpointErr(FlowCheckpointFlowFingerprintMismatch, "current flow does not match checkpoint fingerprint")
	}
	state, ok := findFlowState(flow, checkpoint.CurrentState)
	if !ok {
		return nil, checkpointErr(FlowCheckpointStateMissing, checkpoint.CurrentState)
	}
	if checkpoint.Cursor.CursorKind != FlowCheckpointCursorTopLevelNext {
		return nil, checkpointErr(FlowCheckpointResumeCursorInvalid, checkpoint.Cursor.CursorKind)
	}
	if checkpoint.Cursor.InstructionIndex < 0 || checkpoint.Cursor.InstructionIndex >= len(state.Body.Statements) {
		return nil, checkpointErr(FlowCheckpointInstructionIndexOutOfRange, fmt.Sprintf("%s[%d] len=%d", state.Name, checkpoint.Cursor.InstructionIndex, len(state.Body.Statements)))
	}
	if checkpoint.Cursor.StateBodyFingerprint != "" && checkpoint.Cursor.StateBodyFingerprint != stateBodyFingerprint(state) {
		return nil, checkpointErr(FlowCheckpointStateBodyChanged, checkpoint.CurrentState)
	}
	if checkpoint.HasResumeTarget {
		if _, ok := findFlowState(flow, checkpoint.ResumeTarget); !ok {
			return nil, checkpointErr(FlowCheckpointResumeTargetMissing, checkpoint.ResumeTarget)
		}
	}
	for _, historyState := range checkpoint.StateHistory {
		if _, ok := findFlowState(flow, historyState); !ok {
			return nil, checkpointErr(FlowCheckpointStateHistoryInvalid, historyState)
		}
	}
	if err := restoreFlowBoardCheckpoint(inst, checkpoint.Board); err != nil {
		return nil, err
	}
	inst.CurrentState = checkpoint.CurrentState
	inst.InstructionIndex = checkpoint.Cursor.InstructionIndex
	inst.HasResumeTarget = checkpoint.HasResumeTarget
	inst.ResumeTarget = checkpoint.ResumeTarget
	inst.StateHistory = append([]string(nil), checkpoint.StateHistory...)
	inst.StateEnv = newEnvironment(inst.RootEnv)
	inst.StateEnv.define(flowInstanceBindingName, Value{Kind: ValueFlow, Flow: inst}, false)
	inst.Completed = false
	inst.Result = Value{}
	inst.UtilityWhenSites = make(map[int]utilityWhenSiteState)
	inst.DirtyBoardFields = make(map[string]struct{})
	return inst, nil
}

type FlowCheckpoint struct {
	Version         int
	Package         string
	Flow            string
	FlowFingerprint string
	CurrentState    string
	Cursor          FlowResumeCursor
	HasResumeTarget bool
	ResumeTarget    string
	Board           FlowBoardCheckpoint
	StateHistory    []string
	StepCount       int
}

type FlowResumeCursor struct {
	InstructionIndex     int
	CursorKind           string
	StateBodyFingerprint string
}

type FlowBoardCheckpoint struct {
	TypeName string
	Fields   []FlowCheckpointField
}

type FlowCheckpointField struct {
	Name  string
	Type  string
	Value FlowCheckpointValue
}

type FlowCheckpointValue struct {
	Kind      string
	Dimension string
	Int       int64
	Float     float64
	Bool      bool
	String    string
	Array     []FlowCheckpointValue
}

func (inst *FlowRuntimeInstance) ExportCheckpoint(opts FlowCheckpointOptions) (FlowCheckpoint, error) {
	return ExportFlowCheckpoint(inst, opts)
}

func ExportFlowCheckpoint(inst *FlowRuntimeInstance, opts FlowCheckpointOptions) (FlowCheckpoint, error) {
	if inst == nil {
		return FlowCheckpoint{}, checkpointErr(FlowCheckpointNotSuspended, "nil flow instance")
	}
	if inst.Completed {
		return FlowCheckpoint{}, checkpointErr(FlowCheckpointCompletedCheckpointUnsupported, "completed flow snapshots are deferred")
	}
	if inst.CurrentState == "" {
		return FlowCheckpoint{}, checkpointErr(FlowCheckpointNotSuspended, "flow has not suspended at a current state")
	}
	state, ok := findFlowState(inst.Decl, inst.CurrentState)
	if !ok {
		return FlowCheckpoint{}, checkpointErr(FlowCheckpointStateMissing, inst.CurrentState)
	}
	if inst.InstructionIndex <= 0 {
		return FlowCheckpoint{}, checkpointErr(FlowCheckpointNotSuspended, "cursor has not advanced past a suspend statement")
	}
	if inst.InstructionIndex > len(state.Body.Statements) {
		return FlowCheckpoint{}, checkpointErr(FlowCheckpointInstructionIndexOutOfRange, fmt.Sprintf("%s[%d] len=%d", state.Name, inst.InstructionIndex, len(state.Body.Statements)))
	}
	if inst.HasResumeTarget {
		if _, ok := findFlowState(inst.Decl, inst.ResumeTarget); !ok {
			return FlowCheckpoint{}, checkpointErr(FlowCheckpointResumeTargetMissing, inst.ResumeTarget)
		}
	}
	if hasUserStateLocals(inst.StateEnv) {
		return FlowCheckpoint{}, checkpointErr(FlowCheckpointStateLocalsUnsupported, "state environment contains user bindings")
	}
	if len(inst.UtilityWhenSites) > 0 {
		return FlowCheckpoint{}, checkpointErr(FlowCheckpointUtilityStateUnsupported, "utility when controller state is not serialized in H1")
	}
	board, err := exportFlowBoardCheckpoint(inst)
	if err != nil {
		return FlowCheckpoint{}, err
	}
	return FlowCheckpoint{
		Version: FlowCheckpointVersion, Package: inst.Package, Flow: inst.Decl.Name,
		FlowFingerprint: flowFingerprint(inst), CurrentState: inst.CurrentState,
		Cursor:          FlowResumeCursor{InstructionIndex: inst.InstructionIndex, CursorKind: FlowCheckpointCursorTopLevelNext, StateBodyFingerprint: stateBodyFingerprint(state)},
		HasResumeTarget: inst.HasResumeTarget, ResumeTarget: inst.ResumeTarget, Board: board,
		StateHistory: append([]string(nil), inst.StateHistory...), StepCount: opts.StepCount,
	}, nil
}

func checkpointErr(reason FlowCheckpointUnsupportedReason, detail string) FlowCheckpointError {
	return FlowCheckpointError{Reason: reason, Detail: detail}
}

func hasUserStateLocals(env *environment) bool {
	if env == nil {
		return false
	}
	for name := range env.values {
		if name != flowInstanceBindingName {
			return true
		}
	}
	return false
}

func exportFlowBoardCheckpoint(inst *FlowRuntimeInstance) (FlowBoardCheckpoint, error) {
	if len(inst.Decl.Board) == 0 {
		return FlowBoardCheckpoint{}, nil
	}
	binding, ok := inst.RootEnv.lookup("board")
	if !ok || binding.value.Kind != ValueRecord {
		return FlowBoardCheckpoint{}, checkpointErr(FlowCheckpointBoardSnapshotUnsupported, "missing board binding")
	}
	record := binding.value.Record
	fields := make([]FlowCheckpointField, 0, len(inst.Decl.Board))
	for _, declField := range inst.Decl.Board {
		value, ok := record.Fields[declField.Name]
		if !ok {
			return FlowBoardCheckpoint{}, checkpointErr(FlowCheckpointBoardSnapshotUnsupported, "missing board field "+declField.Name)
		}
		cpValue, err := checkpointValue(value)
		if err != nil {
			return FlowBoardCheckpoint{}, checkpointErr(FlowCheckpointUnsupportedValueType, declField.Name+": "+err.Error())
		}
		fields = append(fields, FlowCheckpointField{Name: declField.Name, Type: expectedTypeString(declField.Type), Value: cpValue})
	}
	return FlowBoardCheckpoint{TypeName: strings.TrimPrefix(record.TypeName, "__flow_board_"), Fields: fields}, nil
}

func checkpointValue(value Value) (FlowCheckpointValue, error) {
	switch value.Kind {
	case ValueBool:
		return FlowCheckpointValue{Kind: string(ValueBool), Bool: value.Bool}, nil
	case ValueString:
		return FlowCheckpointValue{Kind: string(ValueString), String: value.Text}, nil
	case ValueInt:
		return FlowCheckpointValue{Kind: string(ValueInt), Dimension: checkpointDimension(value), Int: value.Int}, nil
	case ValueFloat:
		return FlowCheckpointValue{Kind: string(ValueFloat), Dimension: checkpointDimension(value), Float: value.Float}, nil
	case ValueArray:
		out := make([]FlowCheckpointValue, 0, len(value.Array))
		for idx, element := range value.Array {
			cv, err := checkpointValue(element)
			if err != nil {
				return FlowCheckpointValue{}, fmt.Errorf("array[%d]: %w", idx, err)
			}
			out = append(out, cv)
		}
		return FlowCheckpointValue{Kind: string(ValueArray), Array: out}, nil
	default:
		return FlowCheckpointValue{}, fmt.Errorf("%s", value.Kind)
	}
}

func checkpointDimension(value Value) string {
	if value.Dimension.IsDimensionless() {
		return ""
	}
	return value.Dimension.String()
}

func flowFingerprint(inst *FlowRuntimeInstance) string {
	h := sha256.New()
	fmt.Fprintf(h, "pkg:%s\nflow:%s\n", inst.Package, inst.Decl.Name)
	for _, field := range inst.Decl.Board {
		fmt.Fprintf(h, "board:%s:%s\n", field.Name, expectedTypeString(field.Type))
	}
	for _, state := range inst.Decl.States {
		fmt.Fprintf(h, "state:%s:%s\n", state.Name, stateBodyFingerprint(state))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func stateBodyFingerprint(state ast.StateDecl) string {
	h := sha256.New()
	fmt.Fprintf(h, "state:%s:len:%d\n", state.Name, len(state.Body.Statements))
	for idx, stmt := range state.Body.Statements {
		fmt.Fprintf(h, "%d:%s\n", idx, reflect.TypeOf(stmt).String())
	}
	return hex.EncodeToString(h.Sum(nil))
}

func restoreFlowBoardCheckpoint(inst *FlowRuntimeInstance, checkpoint FlowBoardCheckpoint) error {
	if len(inst.Decl.Board) == 0 {
		if checkpoint.TypeName != "" || len(checkpoint.Fields) != 0 {
			return checkpointErr(FlowCheckpointBoardSchemaMismatch, "boardless flow cannot accept board checkpoint data")
		}
		return nil
	}
	if checkpoint.TypeName != "" && checkpoint.TypeName != inst.Decl.Name {
		return checkpointErr(FlowCheckpointBoardSchemaMismatch, fmt.Sprintf("board type %q for flow %q", checkpoint.TypeName, inst.Decl.Name))
	}
	if len(checkpoint.Fields) != len(inst.Decl.Board) {
		return checkpointErr(FlowCheckpointBoardSchemaMismatch, fmt.Sprintf("field count %d want %d", len(checkpoint.Fields), len(inst.Decl.Board)))
	}
	binding, ok := inst.RootEnv.lookup("board")
	if !ok || binding.value.Kind != ValueRecord {
		return checkpointErr(FlowCheckpointBoardSchemaMismatch, "missing runtime board binding")
	}
	record := binding.value
	fields := make(map[string]Value, len(inst.Decl.Board))
	order := make([]string, 0, len(inst.Decl.Board))
	for idx, declField := range inst.Decl.Board {
		cpField := checkpoint.Fields[idx]
		expectedType := expectedTypeString(declField.Type)
		if cpField.Name != declField.Name || cpField.Type != expectedType {
			return checkpointErr(FlowCheckpointBoardSchemaMismatch, fmt.Sprintf("field %d checkpoint %s:%s want %s:%s", idx, cpField.Name, cpField.Type, declField.Name, expectedType))
		}
		value, err := restoreCheckpointValue(cpField.Value, declField.Type)
		if err != nil {
			return checkpointErr(FlowCheckpointBoardValueTypeMismatch, cpField.Name+": "+err.Error())
		}
		fields[declField.Name] = value
		order = append(order, declField.Name)
	}
	record.Record.Fields = fields
	record.Record.FieldOrder = order
	assignBindingValue(inst.RootEnv, "board", record)
	inst.DirtyBoardFields = make(map[string]struct{})
	return nil
}

func restoreCheckpointValue(checkpoint FlowCheckpointValue, expected ast.TypeRef) (Value, error) {
	if expected.IsArray || expected.ArrayDepth > 0 {
		if checkpoint.Kind != string(ValueArray) {
			return Value{}, fmt.Errorf("kind %s is not Array", checkpoint.Kind)
		}
		elementType := expected
		if elementType.ArrayDepth > 0 {
			elementType.ArrayDepth--
		}
		if elementType.ArrayDepth == 0 {
			elementType.IsArray = false
		}
		out := make([]Value, 0, len(checkpoint.Array))
		for idx, element := range checkpoint.Array {
			value, err := restoreCheckpointValue(element, elementType)
			if err != nil {
				return Value{}, fmt.Errorf("array[%d]: %w", idx, err)
			}
			out = append(out, value)
		}
		return Value{Kind: ValueArray, Array: out}, nil
	}
	switch expected.Name {
	case "Bool":
		if checkpoint.Kind != string(ValueBool) {
			return Value{}, fmt.Errorf("kind %s is not Bool", checkpoint.Kind)
		}
		return Value{Kind: ValueBool, Bool: checkpoint.Bool}, nil
	case "String":
		if checkpoint.Kind != string(ValueString) {
			return Value{}, fmt.Errorf("kind %s is not String", checkpoint.Kind)
		}
		return Value{Kind: ValueString, Text: checkpoint.String}, nil
	case "Int":
		if checkpoint.Kind != string(ValueInt) {
			return Value{}, fmt.Errorf("kind %s is not Int", checkpoint.Kind)
		}
		if checkpoint.Dimension != expected.Dimension.String() {
			return Value{}, fmt.Errorf("dimension %q want %q", checkpoint.Dimension, expected.Dimension.String())
		}
		return Value{Kind: ValueInt, Int: checkpoint.Int, Dimension: expected.Dimension}, nil
	case "Float":
		if checkpoint.Kind != string(ValueFloat) {
			return Value{}, fmt.Errorf("kind %s is not Float", checkpoint.Kind)
		}
		if checkpoint.Dimension != expected.Dimension.String() {
			return Value{}, fmt.Errorf("dimension %q want %q", checkpoint.Dimension, expected.Dimension.String())
		}
		return Value{Kind: ValueFloat, Float: checkpoint.Float, Dimension: expected.Dimension}, nil
	default:
		return Value{}, fmt.Errorf("unsupported board type %s", expectedTypeString(expected))
	}
}
