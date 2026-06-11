package main

import (
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
		_ = os.Remove(filepath.Join("..", "..", "m21_pdf_compiled_text.pdf"))
		_ = os.Remove(filepath.Join("..", "..", "m21_pdf_compiled_styled.pdf"))
	})
	root := "../../Libraries/Pdf"
	stdout, stderr, err := executeCLIWithSidecars(t, "test", root, "octxiliary-pdf", "octxiliary-image", "octxiliary-io")
	if err != nil {
		t.Fatalf("oct test failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	assertNoMissingSidecarFallback(t, stdout, stderr)

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
	binDir := sharedTestSidecarDir(t, "octxiliary-pdf", "octxiliary-image", "octxiliary-io")

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
	binDir := sharedTestSidecarDir(t, "octxiliary-pdf")

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
