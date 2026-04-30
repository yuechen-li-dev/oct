package interpret

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oct/internal/project"
	"oct/internal/typecheck"
)

func TestExecuteMainResumeWithEmptySlotFailsDeterministically(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "main.oct")
	source := `package Main

flow BadResume() -> Int {
    state Start {
        resume
    }
}

fn Main() -> Void {
    let f = BadResume()
    Step(f)
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	program, err := project.Load(dir)
	if err != nil {
		t.Fatalf("load program: %v", err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatalf("typecheck program: %v", err)
	}

	_, err = ExecuteMain(program, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected ExecuteMain to fail")
	}
	if !strings.Contains(err.Error(), "runtime error: resume called with empty resume slot") {
		t.Fatalf("expected deterministic empty-slot resume error, got %q", err.Error())
	}
}

func TestExecuteMainFormatFloatRejectsNegativePrecision(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "main.oct")
	source := `package Main

fn Main() -> String {
    return FormatFloat(1.234, 0 - 1)
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	program, err := project.Load(dir)
	if err != nil {
		t.Fatalf("load program: %v", err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatalf("typecheck program: %v", err)
	}

	_, err = ExecuteMain(program, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected ExecuteMain to fail")
	}
	if !strings.Contains(err.Error(), "FormatFloat precision must be >= 0") {
		t.Fatalf("expected negative precision error, got %q", err.Error())
	}
}

func TestExecuteMainFloatBuiltinConvertsInt(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "main.oct")
	source := `package Main

fn Main() -> Float {
    let count = 3
    return Float(count) / 2.0
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	program, err := project.Load(dir)
	if err != nil {
		t.Fatalf("load program: %v", err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatalf("typecheck program: %v", err)
	}

	result, err := ExecuteMain(program, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("execute program: %v", err)
	}
	if result.Kind != ValueFloat || result.Float != 1.5 {
		t.Fatalf("expected float 1.5, got %#v", result)
	}
}

func TestExecuteMainTupleProbeDestructure(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "main.oct")
	source := `package Main

fn Main() -> Int {
    var a = 0
    var b = 0
    a, b = TupleProbe()
    if a != 1 {
        return 100
    }
    if b != 2 {
        return 200
    }
    return a + b
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	program, err := project.Load(dir)
	if err != nil {
		t.Fatalf("load program: %v", err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatalf("typecheck program: %v", err)
	}
	result, err := ExecuteMain(program, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("execute program: %v", err)
	}
	if result.Kind != ValueInt || result.Int != 3 {
		t.Fatalf("expected 3 got %#v", result)
	}
}

func TestExecuteMainBoolIntProbeDestructure(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "main.oct")
	source := `package Main

fn Main() -> Int {
    var flag = false
    var n = 0
    flag, n = BoolIntProbe()
    if flag {
        return n
    }
    return 0
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	program, err := project.Load(dir)
	if err != nil {
		t.Fatalf("load program: %v", err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatalf("typecheck program: %v", err)
	}
	result, err := ExecuteMain(program, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("execute program: %v", err)
	}
	if result.Kind != ValueInt || result.Int != 7 {
		t.Fatalf("expected 7 got %#v", result)
	}
}
