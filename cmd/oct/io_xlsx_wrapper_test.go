package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIOXlsxWrapper(t *testing.T) {
	outputPath := filepath.Join("..", "..", "io_xlsx_m0.xlsx")
	_ = os.Remove(outputPath)
	t.Cleanup(func() {
		_ = os.Remove(outputPath)
	})

	root := filepath.Join("..", "..", "Libraries", "IO")
	stdout, stderr, err := executeCLIWithSidecars(t, "test", root, "octxiliary-io", "octxiliary-csv", "octxiliary-json", "octxiliary-xlsx")
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

	info, statErr := os.Stat(outputPath)
	if statErr != nil {
		t.Fatalf("expected xlsx artifact at %s: %v", outputPath, statErr)
	}
	if info.Size() == 0 {
		t.Fatalf("expected non-empty xlsx artifact at %s", outputPath)
	}
}

func TestCompiledIOXlsxWrapper(t *testing.T) {
	outputPath := filepath.Join("..", "..", "io_xlsx_m0.xlsx")
	_ = os.Remove(outputPath)
	t.Cleanup(func() {
		_ = os.Remove(outputPath)
	})

	repo := filepath.Join("..", "..")
	binDir := sharedTestSidecarDir(t, "octxiliary-xlsx")

	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/IO/IO.Xlsx.octest", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled IO xlsx wrapper tests failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	assertNoCompiledFallback(t, string(out), "")
	assertCompiledCountAtLeast(t, string(out), 1)
	assertOutputContains(t, string(out),
		"PASS IO.XlsxWriteMiniWorkflow",
		"PASS IO.XlsxRejectsMissingSheetWrites",
		"PASS IO.XlsxRejectsInvalidWorkbookHandle",
		"PASS IO.XlsxRejectsInvalidSavePathExtension",
		"PASS IO.XlsxRejectsSaveWithInvalidWorkbookHandle",
	)
}

func TestCompiledIOXlsxMissingSidecarDiagnostic(t *testing.T) {
	repo := filepath.Join("..", "..")
	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/IO/IO.Xlsx.octest", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing xlsx sidecar failure, got success:\n%s", string(out))
	}
	if !strings.Contains(string(out), `Octxiliary sidecar "octxiliary-xlsx" not found`) {
		t.Fatalf("expected xlsx missing sidecar diagnostic, got:\n%s", string(out))
	}
}
