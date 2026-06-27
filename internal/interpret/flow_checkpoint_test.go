package interpret

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

func checkpointProgram(t *testing.T, source string) project.Program {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.oct"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	program, err := project.Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatalf("typecheck: %v", err)
	}
	return program
}

func suspendedInstance(t *testing.T, source, flow string, steps int) (*FlowRuntimeInstance, FlowRunResult) {
	t.Helper()
	program := checkpointProgram(t, source)
	interp, err := newInterpreter(program, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("interpreter: %v", err)
	}
	t.Cleanup(interp.close)
	decl := interp.flows["Main."+flow]
	inst := interp.instantiateFlow(decl, "Main", nil)
	ran, _, suspended, err := interp.stepFlowWithTransitionLimit(inst, steps)
	if err != nil {
		t.Fatalf("step: %v", err)
	}
	return inst, flowRunResult(inst, ran, suspended)
}

func requireCheckpointReason(t *testing.T, err error, reason FlowCheckpointUnsupportedReason) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s error", reason)
	}
	var checkpointErr FlowCheckpointError
	if !errors.As(err, &checkpointErr) {
		t.Fatalf("expected FlowCheckpointError, got %T %v", err, err)
	}
	if checkpointErr.Reason != reason {
		t.Fatalf("expected reason %s, got %s (%v)", reason, checkpointErr.Reason, err)
	}
}

func TestExportFlowCheckpointSuspendedBoardless(t *testing.T) {
	inst, result := suspendedInstance(t, `package Main
flow Waiter() -> Int { state Start { suspend return 7 } }
fn Main() -> Int { return 0 }
`, "Waiter", 10)
	cp, err := ExportFlowCheckpoint(inst, FlowCheckpointOptions{StepCount: result.Steps})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if cp.Package != "Main" || cp.Flow != "Waiter" || cp.CurrentState != "Start" {
		t.Fatalf("bad identity/state: %#v", cp)
	}
	if cp.Cursor.InstructionIndex != 1 || cp.Cursor.CursorKind != FlowCheckpointCursorTopLevelNext {
		t.Fatalf("bad cursor: %#v", cp.Cursor)
	}
	if len(cp.StateHistory) != 1 || cp.StateHistory[0] != "Start" {
		t.Fatalf("bad history: %#v", cp.StateHistory)
	}
	if cp.StepCount != result.Steps {
		t.Fatalf("step count = %d, want %d", cp.StepCount, result.Steps)
	}
	if cp.FlowFingerprint == "" || cp.Cursor.StateBodyFingerprint == "" {
		t.Fatalf("expected fingerprints: %#v", cp)
	}
	if cp.Board.TypeName != "" || len(cp.Board.Fields) != 0 {
		t.Fatalf("expected empty board checkpoint, got %#v", cp.Board)
	}
}

func TestExportFlowCheckpointBoardScalarsPreservesOrderAndValues(t *testing.T) {
	inst, _ := suspendedInstance(t, `package Main
flow Boarded() -> Int {
    board { Flag: Bool Count: Int Ratio: Float Label: String }
    state Start { board.Flag = true board.Count = 3 board.Ratio = 2.5 board.Label = "ready" suspend return board.Count }
}
fn Main() -> Int { return 0 }
`, "Boarded", 10)
	cp, err := ExportFlowCheckpoint(inst, FlowCheckpointOptions{})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if cp.Board.TypeName != "Boarded" {
		t.Fatalf("board type = %q", cp.Board.TypeName)
	}
	names := []string{"Flag", "Count", "Ratio", "Label"}
	if len(cp.Board.Fields) != len(names) {
		t.Fatalf("fields: %#v", cp.Board.Fields)
	}
	for i, name := range names {
		if cp.Board.Fields[i].Name != name {
			t.Fatalf("field %d name = %q", i, cp.Board.Fields[i].Name)
		}
	}
	if !cp.Board.Fields[0].Value.Bool || cp.Board.Fields[1].Value.Int != 3 || cp.Board.Fields[2].Value.Float != 2.5 || cp.Board.Fields[3].Value.String != "ready" {
		t.Fatalf("bad values: %#v", cp.Board.Fields)
	}
}

func TestExportFlowCheckpointResumeSlot(t *testing.T) {
	inst, _ := suspendedInstance(t, `package Main
flow Remembering() -> Int { state Start { remember goto Hold } state Hold { suspend resume } }
fn Main() -> Int { return 0 }
`, "Remembering", 10)
	cp, err := ExportFlowCheckpoint(inst, FlowCheckpointOptions{})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if !cp.HasResumeTarget || cp.ResumeTarget != "Start" {
		t.Fatalf("bad resume slot: %#v", cp)
	}
	if len(cp.StateHistory) != 2 || cp.StateHistory[0] != "Start" || cp.StateHistory[1] != "Hold" {
		t.Fatalf("bad history: %#v", cp.StateHistory)
	}
}

func TestExportFlowCheckpointRejectsNonSuspendedAndCompleted(t *testing.T) {
	inst, _ := suspendedInstance(t, `package Main
flow Done() -> Int { state Start { return 1 } }
fn Main() -> Int { return 0 }
`, "Done", 10)
	_, err := ExportFlowCheckpoint(inst, FlowCheckpointOptions{})
	requireCheckpointReason(t, err, FlowCheckpointCompletedCheckpointUnsupported)

	program := checkpointProgram(t, `package Main
flow Idle() -> Int { state Start { suspend return 1 } }
fn Main() -> Int { return 0 }
`)
	interp, err := newInterpreter(program, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("interpreter: %v", err)
	}
	defer interp.close()
	fresh := interp.instantiateFlow(interp.flows["Main.Idle"], "Main", nil)
	_, err = ExportFlowCheckpoint(fresh, FlowCheckpointOptions{})
	requireCheckpointReason(t, err, FlowCheckpointNotSuspended)
}

func TestExportFlowCheckpointRejectsStateLocalsUtilityAndMissingResume(t *testing.T) {
	inst, _ := suspendedInstance(t, `package Main
flow Local() -> Int { state Start { let x = 1 suspend return x } }
fn Main() -> Int { return 0 }
`, "Local", 10)
	_, err := ExportFlowCheckpoint(inst, FlowCheckpointOptions{})
	requireCheckpointReason(t, err, FlowCheckpointStateLocalsUnsupported)

	inst.StateEnv.values = map[string]binding{flowInstanceBindingName: inst.StateEnv.values[flowInstanceBindingName]}
	inst.UtilityWhenSites[1] = utilityWhenSiteState{HasCurrent: true, Current: Value{Kind: ValueInt, Int: 1}, Score: 1}
	_, err = ExportFlowCheckpoint(inst, FlowCheckpointOptions{})
	requireCheckpointReason(t, err, FlowCheckpointUtilityStateUnsupported)

	inst.UtilityWhenSites = map[int]utilityWhenSiteState{}
	inst.HasResumeTarget = true
	inst.ResumeTarget = "Missing"
	_, err = ExportFlowCheckpoint(inst, FlowCheckpointOptions{})
	requireCheckpointReason(t, err, FlowCheckpointResumeTargetMissing)
}

func TestExportFlowCheckpointRejectsUnsupportedBoardValueAndDoesNotMutate(t *testing.T) {
	inst, _ := suspendedInstance(t, `package Main
flow Boarded() -> Int { board { Count: Int } state Start { board.Count = 3 suspend return board.Count } }
fn Main() -> Int { return 0 }
`, "Boarded", 10)
	beforeIndex := inst.InstructionIndex
	beforeHistory := append([]string(nil), inst.StateHistory...)
	binding, _ := inst.RootEnv.lookup("board")
	record := binding.value
	record.Record.Fields["Count"] = Value{Kind: ValueRecord, Record: RecordValue{TypeName: "Unsupported"}}
	assignBindingValue(inst.RootEnv, "board", record)
	_, err := ExportFlowCheckpoint(inst, FlowCheckpointOptions{})
	requireCheckpointReason(t, err, FlowCheckpointUnsupportedValueType)
	if inst.InstructionIndex != beforeIndex {
		t.Fatalf("mutated instruction index")
	}
	if len(inst.StateHistory) != len(beforeHistory) || inst.StateHistory[0] != beforeHistory[0] {
		t.Fatalf("mutated history: %#v", inst.StateHistory)
	}
}

func TestSuspendedFlowRunResultExportsCheckpointWithoutExposingInstance(t *testing.T) {
	program := checkpointProgram(t, `package Main
flow Waiter() -> Int { state Start { suspend return 4 } }
fn Main() -> Int { return 0 }
`)
	result, err := RunFlowToSuspensionWithOptions(program, "Main", "Waiter", 10, &bytes.Buffer{}, ExecuteOptions{})
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !result.Suspended {
		t.Fatalf("expected suspension")
	}
	cp, err := result.ExportCheckpoint(FlowCheckpointOptions{})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if cp.StepCount != result.Steps {
		t.Fatalf("step count = %d, want %d", cp.StepCount, result.Steps)
	}
}
