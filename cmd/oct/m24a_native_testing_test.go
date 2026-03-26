package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oct/internal/cli"
)

func TestOctTestRunsFactFilesAndReportsPassFail(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Math", "math.oct", "package Math\nfn FortyTwo() -> Int { return 42 }\n")
	writeOctPkgFile(t, root, "Math", "math.octest", strings.Join([]string{
		"package Math",
		"[Fact]",
		"fn Alpha() -> Void { Assert.Equal(42, FortyTwo(), \"alpha\") }",
		"[Fact]",
		"fn Beta() -> Void { Assert.True(false, \"beta fails\") }",
		"[Fact]",
		"fn Gamma() -> Void { let x = 1 / 0 Assert.True(x > 0, \"unreachable\") }",
	}, "\n")+"\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{"test", root}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("expected test failure")
	}
	if !strings.Contains(stdout.String(), "PASS Math.Alpha") {
		t.Fatalf("expected pass output, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "FAIL Math.Beta") {
		t.Fatalf("expected fail output, got %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "FAIL Math.Gamma") {
		t.Fatalf("expected runtime fail output, got %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "test failed: 2 test(s) failed") {
		t.Fatalf("expected command failure summary, got %q", stderr.String())
	}
}

func TestOctTestDeterministicOrderingAndVoidReturns(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "order.octest", strings.Join([]string{
		"package Main",
		"[Fact]",
		"fn Zeta() -> Void { return }",
		"[Fact]",
		"fn Alpha() -> Void { Assert.Near(2.0, 1.9999, 0.01, \"near passes\") }",
	}, "\n")+"\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"test", root}, &stdout, &stderr); err != nil {
		t.Fatalf("expected success, got err=%v stderr=%q", err, stderr.String())
	}
	alpha := strings.Index(stdout.String(), "PASS Main.Alpha")
	zeta := strings.Index(stdout.String(), "PASS Main.Zeta")
	if alpha < 0 || zeta < 0 || alpha > zeta {
		t.Fatalf("expected alphabetical order, got %q", stdout.String())
	}
}

func TestOctTestRejectsInvalidFactShapesAndVoidRules(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "bad.octest", "package Main\n[Fact]\nfn Bad(x: Int) -> Void { return }\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{"test", root}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "[Fact] function must not declare parameters") {
		t.Fatalf("expected invalid [Fact] error, got err=%v stderr=%q", err, stderr.String())
	}

	root2 := t.TempDir()
	writeOctPkgFile(t, root2, "Main", "main.oct", "package Main\n[Fact]\nfn NotAllowed() -> Int { return 1 }\nfn Main() -> Int { return 0 }\n")
	stdout.Reset()
	stderr.Reset()
	err = cli.Execute([]string{"build", filepath.Join(root2, "Main", "main.oct")}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "[Fact] is only valid in .octest files") {
		t.Fatalf("expected .oct [Fact] rejection, got err=%v stderr=%q", err, stderr.String())
	}
}

func TestOctTestMigratedNumericsProofPackage(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{"test", filepath.Join("..", "..", "testdata", "m22a", "valid")}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("oct test on migrated numerics package failed: %v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "PASS Numerics.BisectionConvergesForSqrt2") {
		t.Fatalf("expected migrated test pass output, got %q", stdout.String())
	}
}

func writeOctPkgFile(t *testing.T, root, pkg, name, content string) {
	t.Helper()
	dir := filepath.Join(root, pkg)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
