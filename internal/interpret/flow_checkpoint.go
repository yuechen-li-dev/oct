package interpret

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
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
