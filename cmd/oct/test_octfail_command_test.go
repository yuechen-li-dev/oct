package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOctFailOctFailDoesNotBreakOctestExecution(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "m24g", "valid")
	stdout, stderr, err := executeCLI("test", root)
	if err != nil {
		t.Fatalf("expected valid .octest suite to pass, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS ConditionSwitch.ConditionSwitchSelectsExpectedArm") {
		t.Fatalf("expected .octest pass output, got %q", stdout)
	}
}

func TestOctFailOctFailFailsWhenExpectedMessageDoesNotMatch(t *testing.T) {
	root := t.TempDir()
	fixture := strings.Join([]string{
		"expect error: \"totally different message\"",
		"",
		"fn Main() -> Int {",
		"    return \"oops\"",
		"}",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(root, "mismatch.octfail"), []byte(fixture), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	stdout, stderr, err := executeCLI("test", root)
	if err == nil {
		t.Fatalf("expected mismatch failure, stdout=%q", stdout)
	}
	if !strings.Contains(stdout, "FAIL mismatch.octfail") {
		t.Fatalf("expected failing case output, got %q", stdout)
	}
	if !strings.Contains(stdout, "expected error containing: \"totally different message\"") {
		t.Fatalf("expected mismatch expectation output, got %q", stdout)
	}
	if !strings.Contains(stderr, "test failed: 1 test(s) failed") {
		t.Fatalf("expected summary failure, got %q", stderr)
	}
}

func TestOctFailOctFailFailsWhenCompilationUnexpectedlySucceeds(t *testing.T) {
	root := t.TempDir()
	fixture := strings.Join([]string{
		"expect error: \"must fail\"",
		"",
		"package Main",
		"",
		"fn Main() -> Int {",
		"    return 1",
		"}",
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(root, "unexpected_success.octfail"), []byte(fixture), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	stdout, _, err := executeCLI("test", root)
	if err == nil {
		t.Fatalf("expected unexpected-success failure")
	}
	if !strings.Contains(stdout, "FAIL unexpected_success.octfail") {
		t.Fatalf("expected failing output, got %q", stdout)
	}
	if !strings.Contains(stdout, "expected compilation failure but build succeeded") {
		t.Fatalf("expected unexpected-success detail, got %q", stdout)
	}
}

func TestOctFailOctFailRejectsMalformedHeader(t *testing.T) {
	root := t.TempDir()
	fixture := "expect error \"missing colon\"\n\nfn Main() -> Int { return 1 }\n"
	if err := os.WriteFile(filepath.Join(root, "bad_header.octfail"), []byte(fixture), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	_, stderr, err := executeCLI("test", root)
	if err == nil {
		t.Fatalf("expected malformed header failure")
	}
	if !strings.Contains(stderr, "malformed expectation header") {
		t.Fatalf("expected malformed header error, got %q", stderr)
	}
}
