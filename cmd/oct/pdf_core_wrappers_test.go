package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPdfCoreWrappers(t *testing.T) {
	t.Parallel()

	workDir := newWrapperTempProject(t)
	writeM30PNGFixture(t, filepath.Join(workDir, "Libraries", "Pdf", "m30_fixture_rect.png"))
	root := repoPath(t, "Libraries", "Pdf")
	stdout, stderr, err := executeCLIWithSidecarsInDir(t, workDir, "test", root, "octxiliary-pdf", "octxiliary-image", "octxiliary-io")
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
	t.Parallel()

	workDir := newWrapperTempProject(t)
	writeM30PNGFixture(t, filepath.Join(workDir, "Libraries", "Pdf", "m30_fixture_rect.png"))
	target := repoPath(t, "Libraries", "Pdf", "Pdf.ImageBytes.octest")
	stdout, stderr, err := executeOctWithSidecarsInDir(t, workDir, []string{"test", target, "--execution", "compiled"}, "octxiliary-pdf", "octxiliary-image", "octxiliary-io")
	if err != nil {
		t.Fatalf("compiled Pdf image bytes tests failed: %v\nstderr:%s\nstdout:%s", err, strings.TrimSpace(stderr), stdout)
	}
	assertNoCompiledFallback(t, stdout, stderr)
	assertCompiledCountAtLeast(t, stdout, 1)
	assertOutputContains(t, stdout,
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

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
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
	t.Parallel()

	workDir := newWrapperTempProject(t)
	target := repoPath(t, "Libraries", "Pdf", "Pdf.CompiledText.octest")
	stdout, stderr, err := executeOctWithSidecarsInDir(t, workDir, []string{"test", target, "--execution", "compiled"}, "octxiliary-pdf")
	if err != nil {
		t.Fatalf("compiled Pdf text tests failed: %v\nstderr:%s\nstdout:%s", err, strings.TrimSpace(stderr), stdout)
	}
	assertNoCompiledFallback(t, stdout, stderr)
	assertCompiledCountAtLeast(t, stdout, 1)
	assertOutputContains(t, stdout,
		"PASS Pdf.CompiledBasicTextSave",
		"PASS Pdf.CompiledStyledTextSave",
		"PASS Pdf.CompiledInvalidPageHandleFails",
		"PASS Pdf.CompiledSaveInvalidPathFails",
	)
}

func TestCompiledPdfMissingSidecarDiagnostic(t *testing.T) {
	workDir := newWrapperTempProject(t)
	target := repoPath(t, "Libraries", "Pdf", "Pdf.CompiledText.octest")
	stdout, stderr, err := executeOctWithCustomWrapperPathInDir(t, workDir, t.TempDir(), []string{"test", target, "--execution", "compiled"})
	if err == nil {
		t.Fatalf("expected missing pdf sidecar failure, got success:\n%s%s", stdout, stderr)
	}
	if !strings.Contains(stdout+stderr, `Octxiliary sidecar "octxiliary-pdf" not found`) {
		t.Fatalf("expected pdf missing sidecar diagnostic, got:\nstdout:%s\nstderr:%s", stdout, stderr)
	}
}
