package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPdfCoreWrappers(t *testing.T) {
	t.Cleanup(func() {
		_ = os.Remove("m21_pdf_compiled_text.pdf")
		_ = os.Remove("m21_pdf_compiled_styled.pdf")
	})
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

func TestCompiledPdfTextWrappers(t *testing.T) {
	repo := filepath.Join("..", "..")
	t.Cleanup(func() {
		_ = os.Remove(filepath.Join(repo, "m21_pdf_compiled_text.pdf"))
		_ = os.Remove(filepath.Join(repo, "m21_pdf_compiled_styled.pdf"))
	})
	binDir := t.TempDir()
	buildPdfOctxiliarySidecar(t, repo, binDir)

	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/Pdf/Pdf.CompiledText.octest", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled Pdf text tests failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	assertOutputContains(t, string(out),
		"PASS Pdf.CompiledBasicTextSave",
		"PASS Pdf.CompiledStyledTextSave",
		"PASS Pdf.CompiledInvalidPageHandleFails",
		"PASS Pdf.CompiledSaveInvalidPathFails",
	)
}

func TestCompiledPdfMissingSidecarDiagnostic(t *testing.T) {
	repo := filepath.Join("..", "..")
	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/Pdf/Pdf.CompiledText.octest", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing pdf sidecar failure, got success:\n%s", string(out))
	}
	if !strings.Contains(string(out), `Octxiliary sidecar "octxiliary-pdf" not found`) {
		t.Fatalf("expected pdf missing sidecar diagnostic, got:\n%s", string(out))
	}
}

func buildPdfOctxiliarySidecar(t *testing.T, repo string, binDir string) {
	t.Helper()
	outPath := filepath.Join(binDir, "octxiliary-pdf")
	build := exec.Command("go", "build", "-o", outPath, "./cmd/octxiliary-pdf")
	build.Dir = repo
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build octxiliary-pdf: %v\n%s", err, strings.TrimSpace(string(out)))
	}
}

func TestPdfDefaultsToBundledInter(t *testing.T) {
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
