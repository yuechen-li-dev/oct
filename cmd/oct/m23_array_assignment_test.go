package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestM23BasicArrayElementAssignment(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"fn Main() -> Float {",
		"    var x = [1.0, 2.0, 3.0]",
		"    x[1] = 5.0",
		"    return x[1]",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "5\n" {
		t.Fatalf("expected 5.0 output, got %q", stdout)
	}
}

func TestM23LoopDrivenArrayPopulation(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"fn Main() -> Float {",
		"    var x = [0.0, 0.0, 0.0]",
		"    var i = 0",
		"    while i < 3 {",
		"        x[i] = i * 2.0",
		"        i = i + 1",
		"    }",
		"    return x[2]",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "4\n" {
		t.Fatalf("expected 4.0 output, got %q", stdout)
	}
}

func TestM23RejectsImmutableArrayBindingAssignment(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "Main", "main.oct")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"fn Main() -> Int {",
		"    let x = [1, 2, 3]",
		"    x[0] = 5",
		"    return x[0]",
		"}",
	}, "\n"))

	_, stderr, err := executeCLI("build", entry)
	if err == nil {
		t.Fatal("expected build failure")
	}
	if !strings.Contains(stderr, "cannot assign to immutable binding") {
		t.Fatalf("unexpected stderr %q", stderr)
	}
}

func TestM23RejectsArrayElementTypeMismatch(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "Main", "main.oct")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"fn Main() -> Int {",
		"    var x = [1, 2, 3]",
		"    x[0] = 1.5",
		"    return x[0]",
		"}",
	}, "\n"))

	_, stderr, err := executeCLI("build", entry)
	if err == nil {
		t.Fatal("expected build failure")
	}
	if !strings.Contains(stderr, "assigned value type does not match array element type") {
		t.Fatalf("unexpected stderr %q", stderr)
	}
}

func TestM23RejectsNonIntArrayIndexAssignment(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "Main", "main.oct")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"fn Main() -> Int {",
		"    var x = [1, 2, 3]",
		"    x[true] = 1",
		"    return x[0]",
		"}",
	}, "\n"))

	_, stderr, err := executeCLI("build", entry)
	if err == nil {
		t.Fatal("expected build failure")
	}
	if !strings.Contains(stderr, "array index must be Int") {
		t.Fatalf("unexpected stderr %q", stderr)
	}
}

func TestM23RejectsOutOfBoundsArrayIndexAssignmentAtRuntime(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "Main", "main.oct")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"fn Main() -> Int {",
		"    var x = [1, 2, 3]",
		"    x[5] = 1",
		"    return x[0]",
		"}",
	}, "\n"))

	_, stderr, err := executeCLI("run", entry)
	if err == nil {
		t.Fatal("expected runtime failure")
	}
	if !strings.Contains(stderr, "out of bounds") {
		t.Fatalf("unexpected stderr %q", stderr)
	}
}

func TestM23SupportsRecordArrayElementAssignment(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "Main", "main.oct")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"record P { X: Int }",
		"fn Main() -> Int {",
		"    var x = [P { X: 1 }]",
		"    x[0] = P { X: 2 }",
		"    return x[0].X",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("run", entry)
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "2\n" {
		t.Fatalf("expected 2 output, got %q", stdout)
	}
}

func TestM23BuildArtifactBehavior(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "Main", "main.oct")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"fn Main() -> Int {",
		"    var x = [1, 2, 3]",
		"    x[0] = 5",
		"    return x[0]",
		"}",
	}, "\n"))

	buildStdout, buildStderr, err := executeCLI("build", entry)
	if err != nil {
		t.Fatalf("build failed: %v stdout=%s stderr=%s", err, buildStdout, buildStderr)
	}
	if !strings.Contains(buildStdout, "build succeeded") {
		t.Fatalf("expected build success output, got %q", buildStdout)
	}
	if _, statErr := os.Stat(entry + ".octbin"); statErr != nil {
		t.Fatalf("expected build artifact, stat err=%v", statErr)
	}

	invalidEntry := filepath.Join(root, "Bad", "main.oct")
	writePkgFile(t, root, "Bad", "main.oct", strings.Join([]string{
		"package Bad",
		"fn Main() -> Int {",
		"    var x = [1, 2, 3]",
		"    x[0] = 1.5",
		"    return x[0]",
		"}",
	}, "\n"))

	_, _, invalidErr := executeCLI("build", invalidEntry)
	if invalidErr == nil {
		t.Fatal("expected invalid build to fail")
	}
	if _, statErr := os.Stat(invalidEntry + ".octbin"); !os.IsNotExist(statErr) {
		t.Fatalf("expected no artifact for invalid build, stat err=%v", statErr)
	}
}
