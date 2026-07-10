//go:build integration

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectedFileCompiledRunnerSelectedFileCompiledRunnerLinkage(t *testing.T) {
	parallelBoundaryTest(t)
	target, err := filepath.Abs(filepath.Join("..", "..", "Language", "Testing", "SelectedFileCompiled", "selected_pass.octest"))
	if err != nil {
		t.Fatalf("resolve selected fixture path: %v", err)
	}

	stdout, stderr, err := executeCLIArgs("test", target, "--execution", "compiled")
	if err != nil {
		t.Fatalf("compiled mode must pass for selected file target, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS Main.SelectedPass") {
		t.Fatalf("expected selected pass output, got %q", stdout)
	}

	stdout, stderr, err = executeCLIArgs("test", target, "--execution", "auto")
	if err != nil {
		t.Fatalf("auto mode must pass for selected file target, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "Execution summary: compiled: 1 interpreted fallback: 0") {
		t.Fatalf("expected compiled execution in auto mode, got %q", stdout)
	}

	stdout, stderr, err = executeCLIArgs("test", target, "--execution", "interpreted")
	if err != nil {
		t.Fatalf("interpreted mode must pass for selected file target, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS Main.SelectedPass") {
		t.Fatalf("expected selected pass output in interpreted mode, got %q", stdout)
	}
}

func TestSelectedFileCompiledRunnerDirectoryTargetIncludesInvalidSiblingFixture(t *testing.T) {
	parallelBoundaryTest(t)
	targetDir, err := filepath.Abs(filepath.Join("..", "..", "Language", "Testing", "SelectedFileCompiled"))
	if err != nil {
		t.Fatalf("resolve fixture dir path: %v", err)
	}
	stdout, stderr, err := executeCLIArgs("test", targetDir, "--execution", "compiled")
	if err != nil {
		t.Fatalf("directory target should pass with sibling .octfail fixture, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS sibling_invalid.octfail") {
		t.Fatalf("expected directory mode to include sibling .octfail fixture, got stdout=%q stderr=%q", stdout, stderr)
	}
}
