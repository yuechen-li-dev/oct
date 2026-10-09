//go:build toolchain

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIOXlsxWrapper(t *testing.T) {
	requireSlowOctxiliary(t)
	t.Parallel()

	workDir := newWrapperTempProject(t)
	copyFixtureDir(t, repoPath(t, "Libraries", "IO", "testdata"), filepath.Join(workDir, "Libraries", "IO", "testdata"))
	outputPath := filepath.Join(workDir, "io_xlsx_m0.xlsx")
	root := repoPath(t, "Libraries", "IO")
	stdout, stderr, err := executeCLIWithSidecarsInDir(t, workDir, "test", root, "octxiliary-io", "octxiliary-csv", "octxiliary-xlsx")
	if err != nil {
		t.Fatalf("oct test failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	assertNoMissingSidecarFallback(t, stdout, stderr)

	if !strings.Contains(stdout, "PASS IO.XlsxWriteMiniWorkflow") {
		t.Fatalf("expected workflow fact pass output, got %q", stdout)
	}
	if !strings.Contains(stdout, "PASS IO.XlsxRejectsMissingSheetWrites") {
		t.Fatalf("expected missing-sheet fact pass output, got %q", stdout)
	}
	if !strings.Contains(stdout, "PASS IO.XlsxRejectsInvalidWorkbookHandle") {
		t.Fatalf("expected invalid-handle fact pass output, got %q", stdout)
	}
	if !strings.Contains(stdout, "PASS IO.XlsxRejectsInvalidSavePathExtension") {
		t.Fatalf("expected invalid save path fact pass output, got %q", stdout)
	}
	if !strings.Contains(stdout, "PASS IO.XlsxRejectsSaveWithInvalidWorkbookHandle") {
		t.Fatalf("expected invalid handle save fact pass output, got %q", stdout)
	}

	if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
		t.Fatalf("successful XLSX contract must clean its output %s: %v", outputPath, statErr)
	}
}

func TestCompiledIOXlsxWrapper(t *testing.T) {
	requireSlowOctxiliary(t)
	t.Parallel()

	workDir := newWrapperTempProject(t)
	outputPath := filepath.Join(workDir, "io_xlsx_m0.xlsx")
	target := repoPath(t, "Libraries", "IO", "IO.Xlsx.octest")
	stdout, stderr, err := executeOctWithSidecarsInDir(t, workDir, []string{"test", target, "--execution", "compiled"}, "octxiliary-xlsx", "octxiliary-io")
	if err != nil {
		t.Fatalf("compiled IO xlsx wrapper tests failed: %v\nstderr:%s\nstdout:%s", err, strings.TrimSpace(stderr), stdout)
	}
	assertNoCompiledFallback(t, stdout, stderr)
	assertCompiledCountAtLeast(t, stdout, 1)
	assertOutputContains(t, stdout,
		"PASS IO.XlsxWriteMiniWorkflow",
		"PASS IO.XlsxRejectsMissingSheetWrites",
		"PASS IO.XlsxRejectsInvalidWorkbookHandle",
		"PASS IO.XlsxRejectsInvalidSavePathExtension",
		"PASS IO.XlsxRejectsSaveWithInvalidWorkbookHandle",
	)
	if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
		t.Fatalf("successful compiled XLSX contract must clean its output %s: %v", outputPath, statErr)
	}
}

func TestCompiledIOXlsxMissingSidecarDiagnostic(t *testing.T) {
	requireSlowOctxiliary(t)
	workDir := newWrapperTempProject(t)
	target := repoPath(t, "Libraries", "IO", "IO.Xlsx.octest")
	stdout, stderr, err := executeOctWithCustomWrapperPathInDir(t, workDir, t.TempDir(), []string{"test", target, "--execution", "compiled"})
	if err == nil {
		t.Fatalf("expected missing xlsx sidecar failure, got success:\n%s%s", stdout, stderr)
	}
	if !strings.Contains(stdout+stderr, `Octxiliary sidecar "octxiliary-xlsx" not found`) {
		t.Fatalf("expected xlsx missing sidecar diagnostic, got:\nstdout:%s\nstderr:%s", stdout, stderr)
	}
}
