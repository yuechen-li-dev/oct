package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppendAppendIntArray(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "Main", "main.oct")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"fn Main() -> Int[] {",
		"    var xs = [1, 2]",
		"    xs = Append(xs, 3)",
		"    return xs",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("run", entry)
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "[1, 2, 3]\n" {
		t.Fatalf("expected [1, 2, 3], got %q", stdout)
	}
}

func TestAppendAppendFloatArray(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "Main", "main.oct")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"fn Main() -> Float[] {",
		"    var xs = [1.0]",
		"    xs = Append(xs, 2.0)",
		"    return xs",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("run", entry)
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "[1, 2]\n" {
		t.Fatalf("expected [1, 2], got %q", stdout)
	}
}

func TestAppendAppendStringArray(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "Main", "main.oct")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"fn Main() -> String[] {",
		"    var xs = [\"a\"]",
		"    xs = Append(xs, \"b\")",
		"    return xs",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("run", entry)
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "[a, b]\n" {
		t.Fatalf("expected [a, b], got %q", stdout)
	}
}

func TestAppendAppendRecordArray(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "Main", "main.oct")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"record P { X: Int }",
		"fn Main() -> Int {",
		"    var xs = [P { X: 1 }]",
		"    xs = Append(xs, P { X: 2 })",
		"    return xs[1].X",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("run", entry)
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "2\n" {
		t.Fatalf("expected 2, got %q", stdout)
	}
}

func TestAppendAppendDimensionedArray(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "Main", "main.oct")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"fn Main() -> Int<m>[] {",
		"    var xs = [1m, 2m]",
		"    xs = Append(xs, 3m)",
		"    return xs",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("run", entry)
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "[1m, 2m, 3m]\n" {
		t.Fatalf("expected [1m, 2m, 3m], got %q", stdout)
	}
}

func TestAppendRejectsAppendElementTypeMismatch(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "Main", "main.oct")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"fn Main() -> Int[] {",
		"    var xs = [1, 2]",
		"    xs = Append(xs, 3.0)",
		"    return xs",
		"}",
	}, "\n"))

	_, stderr, err := executeCLI("build", entry)
	if err == nil {
		t.Fatal("expected build failure")
	}
	if !strings.Contains(stderr, "Append element type must match array element type") {
		t.Fatalf("unexpected stderr %q", stderr)
	}
}

func TestAppendRejectsAppendOnNonArray(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "Main", "main.oct")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"fn Main() -> Int[] {",
		"    return Append(1, 2)",
		"}",
	}, "\n"))

	_, stderr, err := executeCLI("build", entry)
	if err == nil {
		t.Fatal("expected build failure")
	}
	if !strings.Contains(stderr, "Append requires array as first argument") {
		t.Fatalf("unexpected stderr %q", stderr)
	}
}

func TestAppendAppendDynamicGrowthLoop(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "Main", "main.oct")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"fn Main() -> Int[] {",
		"    var xs = [0]",
		"    var i = 1",
		"    while i < 4 {",
		"        xs = Append(xs, i)",
		"        i = i + 1",
		"    }",
		"    return xs",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("run", entry)
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "[0, 1, 2, 3]\n" {
		t.Fatalf("expected [0, 1, 2, 3], got %q", stdout)
	}
}

func TestAppendBuildArtifactBehavior(t *testing.T) {
	root := t.TempDir()
	entry := filepath.Join(root, "Main", "main.oct")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"fn Main() -> Int[] {",
		"    var xs = [1, 2]",
		"    xs = Append(xs, 3)",
		"    return xs",
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
		"fn Main() -> Int[] {",
		"    var xs = [1, 2]",
		"    xs = Append(xs, 3.0)",
		"    return xs",
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
