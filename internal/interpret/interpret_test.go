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

func TestExecuteMainPowBuiltinUsesTwoArgumentForm(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "main.oct")
	source := `package Main

fn Main() -> Float {
    let sutMpa = 900.0
    let a = 4.51
    let b = 0.0 - 0.265
    let surface = a * Pow(sutMpa, b)
    let ratio = 2.0
    let size = Pow(ratio, 0.0 - 0.107)
    return surface + size
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
	if result.Kind != ValueFloat {
		t.Fatalf("expected float result, got %#v", result)
	}
	if result.Float <= 0.0 {
		t.Fatalf("expected positive result from Pow expression composition, got %#v", result)
	}
}
