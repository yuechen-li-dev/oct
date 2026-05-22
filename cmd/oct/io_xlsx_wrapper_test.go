package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIOXlsxWrapper(t *testing.T) {
	outputPath := filepath.Join("io_xlsx_m0.xlsx")
	_ = os.Remove(outputPath)
	t.Cleanup(func() {
		_ = os.Remove(outputPath)
	})

	root := filepath.Join("..", "..", "Libraries", "IO")
	stdout, stderr, err := executeCLI("test", root)
	if err != nil {
		t.Fatalf("oct test failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}

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
