package main

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImageCoreWrappers(t *testing.T) {
	t.Parallel()

	workDir := newWrapperTempProject(t)
	if err := synthesizeImageCoreFixtures(workDir); err != nil {
		t.Fatalf("synthesize fixtures: %v", err)
	}

	root := repoPath(t, "Libraries", "Image")
	stdout, stderr, err := executeCLIWithSidecarsInDir(t, workDir, "test", root, "octxiliary-image")
	if err != nil {
		t.Fatalf("oct test failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	assertNoMissingSidecarFallback(t, stdout, stderr)

	expectedPasses := []string{
		"PASS Image.LoadInspectAndSaveRoundTrip",
		"PASS Image.MetadataMatchesJpegFixture",
		"PASS Image.LoadMissingFails",
		"PASS Image.LoadCorruptImageFails",
		"PASS Image.SaveUnsupportedExtensionFails",
	}

	for _, marker := range expectedPasses {
		if !strings.Contains(stdout, marker) {
			t.Fatalf("expected marker %q in stdout, got %q", marker, stdout)
		}
	}
}

func TestCompiledImageCoreWrappers(t *testing.T) {
	t.Parallel()

	workDir := newWrapperTempProject(t)
	if err := synthesizeImageCoreFixtures(workDir); err != nil {
		t.Fatalf("synthesize fixtures: %v", err)
	}

	target := repoPath(t, "Libraries", "Image")
	stdout, stderr, err := executeOctWithSidecarsInDir(t, workDir, []string{"test", target, "--execution", "compiled"}, "octxiliary-image")
	if err != nil {
		t.Fatalf("compiled Image tests failed: %v\nstderr:%s\nstdout:%s", err, strings.TrimSpace(stderr), stdout)
	}
	assertNoCompiledFallback(t, stdout, stderr)
	assertCompiledCountAtLeast(t, stdout, 1)
	for _, marker := range []string{
		"PASS Image.LoadInspectAndSaveRoundTrip",
		"PASS Image.MetadataMatchesJpegFixture",
		"PASS Image.LoadMissingFails",
		"PASS Image.LoadCorruptImageFails",
		"PASS Image.SaveUnsupportedExtensionFails",
	} {
		if !strings.Contains(stdout, marker) {
			t.Fatalf("expected marker %q in compiled output, got:\n%s", marker, stdout)
		}
	}
}

func TestCompiledImageMissingSidecarDiagnostic(t *testing.T) {
	workDir := newWrapperTempProject(t)
	target := repoPath(t, "Libraries", "Image", "Image.Core.octest")
	stdout, stderr, err := executeOctWithCustomWrapperPathInDir(t, workDir, t.TempDir(), []string{"test", target, "--execution", "compiled"})
	if err == nil {
		t.Fatalf("expected missing image sidecar failure, got success:\n%s%s", stdout, stderr)
	}
	if !strings.Contains(stdout+stderr, `Octxiliary sidecar "octxiliary-image" not found`) {
		t.Fatalf("expected image missing sidecar diagnostic, got:\nstdout:%s\nstderr:%s", stdout, stderr)
	}
}

func synthesizeImageCoreFixtures(dir string) error {
	rect := image.NewRGBA(image.Rect(0, 0, 3, 2))
	palette := []color.RGBA{
		{R: 255, G: 0, B: 0, A: 255},
		{R: 0, G: 255, B: 0, A: 255},
		{R: 0, G: 0, B: 255, A: 255},
		{R: 255, G: 255, B: 0, A: 255},
		{R: 255, G: 0, B: 255, A: 255},
		{R: 0, G: 255, B: 255, A: 255},
	}
	index := 0
	for y := 0; y < 2; y++ {
		for x := 0; x < 3; x++ {
			rect.SetRGBA(x, y, palette[index])
			index++
		}
	}

	if err := writePNG(filepath.Join(dir, "mx103d_fixture_rect.png"), rect); err != nil {
		return err
	}
	if err := writeJPEG(filepath.Join(dir, "mx103d_fixture_rect.jpg"), rect); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "mx103d_fixture_corrupt.img"), []byte("not an image payload\n"), 0o644)
}

func writePNG(path string, data image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, data)
}

func writeJPEG(path string, data image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return jpeg.Encode(file, data, &jpeg.Options{Quality: 95})
}
