package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPdfCoreWrappers(t *testing.T) {
	writeM30PNGFixture(t, filepath.Join("..", "..", "Libraries", "Pdf", "m30_fixture_rect.png"))
	t.Cleanup(func() {
		_ = os.Remove(filepath.Join("..", "..", "Libraries", "Pdf", "m30_fixture_rect.png"))
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
		"PASS Pdf.ImageEncodePngDrawImageBytesSizedAndSave",
		"PASS Pdf.DrawImageBytesRejectsUnsupportedFormat",
	}

	for _, marker := range expectedPasses {
		if !strings.Contains(stdout, marker) {
			t.Fatalf("expected marker %q in stdout, got %q", marker, stdout)
		}
	}
}

func TestCompiledPdfImageBytesInterop(t *testing.T) {
	repo := filepath.Join("..", "..")
	fixturePath := filepath.Join(repo, "Libraries", "Pdf", "m30_fixture_rect.png")
	writeM30PNGFixture(t, fixturePath)
	t.Cleanup(func() {
		_ = os.Remove(filepath.Join(repo, "m30_pdf_image_bytes.pdf"))
		_ = os.Remove(fixturePath)
	})
	binDir := t.TempDir()
	buildPdfOctxiliarySidecar(t, repo, binDir)
	buildImageOctxiliarySidecar(t, repo, binDir)
	buildIOOctxiliarySidecar(t, repo, binDir)

	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/Pdf/Pdf.ImageBytes.octest", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled Pdf image bytes tests failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	assertOutputContains(t, string(out),
		"PASS Pdf.ImageEncodePngDrawImageBytesSizedAndSave",
		"PASS Pdf.DrawImageBytesRejectsUnsupportedFormat",
	)
}

func writeM30PNGFixture(t *testing.T, path string) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 8, 6))
	for y := 0; y < 6; y++ {
		for x := 0; x < 8; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(20 + x*20),
				G: uint8(40 + y*25),
				B: 180,
				A: 255,
			})
		}
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
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

func buildIOOctxiliarySidecar(t *testing.T, repo string, binDir string) {
	t.Helper()
	outPath := filepath.Join(binDir, "octxiliary-io")
	build := exec.Command("go", "build", "-o", outPath, "./cmd/octxiliary-io")
	build.Dir = repo
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build octxiliary-io: %v\n%s", err, strings.TrimSpace(string(out)))
	}
}

func buildImageOctxiliarySidecar(t *testing.T, repo string, binDir string) {
	t.Helper()
	outPath := filepath.Join(binDir, "octxiliary-image")
	build := exec.Command("go", "build", "-o", outPath, "./cmd/octxiliary-image")
	build.Dir = repo
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build octxiliary-image: %v\n%s", err, strings.TrimSpace(string(out)))
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
