package octgen

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteUsesExistingTypedInterpreter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "generator.oct")
	source := "package External\nrecord Result { Value: Int }\nfn Generate() -> Result { return Result { Value: 42 } }\n"
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	value, err := Execute(path)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if value.Kind != ValueRecord || value.Record.TypeName != "Result" || value.Record.Fields["Value"].Int != 42 {
		t.Fatalf("unexpected interpreted value: %#v", value)
	}
}

func TestExecuteReportsOctSourceDiagnostics(t *testing.T) {
	path := filepath.Join("testdata", "invalid_source", "generator.oct")
	_, err := Execute(path)
	if err == nil || !strings.Contains(err.Error(), "type-check OctGen generator") || !strings.Contains(err.Error(), "invalid_source") {
		t.Fatalf("Execute error = %v", err)
	}
}

func TestMultiOutputCheckAndTransactionalValidation(t *testing.T) {
	directory := t.TempDir()
	generator := filepath.Join(directory, "generator.oct")
	if err := os.WriteFile(generator, []byte("package External\nfn Generate() -> Int { return 1 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(directory, "first.generated.go")
	second := filepath.Join(directory, "second.generated.go")
	artifacts := []Artifact{{Path: first, Content: []byte("package example\n")}, {Path: second, Content: []byte("package example\n")}}
	if err := Write(generator, artifacts); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := Check(generator, artifacts); err != nil {
		t.Fatalf("fresh Check: %v", err)
	}
	before, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	invalid := []Artifact{{Path: first, Content: []byte("package changed\n")}, {Path: first, Content: []byte("package changed\n")}}
	if err := Write(generator, invalid); err == nil || !strings.Contains(err.Error(), "duplicated") {
		t.Fatalf("duplicate Write error = %v", err)
	}
	after, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatal("failed validation modified a coordinated output")
	}
	if err := os.WriteFile(second, []byte("package stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Check(generator, artifacts); err == nil || !strings.Contains(err.Error(), second) {
		t.Fatalf("stale Check error = %v", err)
	}
	if err := ValidateArtifacts(generator, []Artifact{{Path: filepath.Join(directory, "..", "escape.generated.go"), Content: []byte("package example\n")}}); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("traversal error = %v", err)
	}
}
