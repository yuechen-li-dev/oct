package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestM105PdfCoreWrappers(t *testing.T) {
	root := "../../Libraries/Pdf"
	stdout, stderr, err := executeCLI("test", root)
	if err != nil {
		t.Fatalf("oct test failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}

	expectedPasses := []string{
		"PASS Pdf.BasicPageCreateTextAndSave",
		"PASS Pdf.DrawImageAndSave",
		"PASS Pdf.DrawImageSizedAndStyledText",
		"PASS Pdf.SaveInvalidPathFails",
		"PASS Pdf.InvalidPageHandleFails",
		"PASS Pdf.InvalidImageHandleFails",
	}

	for _, marker := range expectedPasses {
		if !strings.Contains(stdout, marker) {
			t.Fatalf("expected marker %q in stdout, got %q", marker, stdout)
		}
	}
}

func TestM105PdfDefaultsToBundledInter(t *testing.T) {
	outputPath := "m105_inter_default.pdf"
	sourcePath := writeSourceFile(t, "m105_inter_default.oct", "fn Main() -> Int ! Error {\n    let page = PdfNewPage(220px, 120px)?\n    let _ = PdfDrawText(page, 10px, 12px, \"inter check\")?\n    let _saved = PdfSave(page, \""+outputPath+"\")?\n    return 0\n}\n")

	stdout, stderr, err := executeCLI("run", sourcePath)
	if err != nil {
		t.Fatalf("oct run failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	t.Cleanup(func() {
		_ = os.Remove(outputPath)
	})

	pdfBytes, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		t.Fatalf("read generated pdf: %v", readErr)
	}
	if len(pdfBytes) == 0 {
		t.Fatalf("generated pdf must not be empty")
	}
	hasInter := bytes.Contains(pdfBytes, []byte("/BaseFont /utf8inter"))
	hasHelvetica := bytes.Contains(pdfBytes, []byte("/BaseFont /Helvetica"))
	if !hasInter && !hasHelvetica {
		t.Fatalf("expected generated pdf to include Inter or Helvetica font marker, got first bytes %q", string(pdfBytes[:min(220, len(pdfBytes))]))
	}
}

func min(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
