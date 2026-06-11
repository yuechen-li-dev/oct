package main

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestImageCoreWrappers(t *testing.T) {
	repo := filepath.Join("..", "..")
	if err := synthesizeImageCoreFixtures(repo); err != nil {
		t.Fatalf("synthesize fixtures: %v", err)
	}
	t.Cleanup(func() {
		for _, path := range []string{
			"mx103d_fixture_rect.png",
			"mx103d_fixture_rect.jpg",
			"mx103d_fixture_corrupt.img",
			"mx103d_roundtrip.jpg",
		} {
			_ = os.Remove(filepath.Join(repo, path))
		}
	})

	root := "../../Libraries/Image"
	stdout, stderr, err := executeCLIWithSidecars(t, "test", root, "octxiliary-image")
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
	repo := filepath.Join("..", "..")
	if err := synthesizeImageCoreFixtures(repo); err != nil {
		t.Fatalf("synthesize fixtures: %v", err)
	}
	t.Cleanup(func() {
		for _, path := range []string{
			"mx103d_fixture_rect.png",
			"mx103d_fixture_rect.jpg",
			"mx103d_fixture_corrupt.img",
			"mx103d_roundtrip.jpg",
		} {
			_ = os.Remove(filepath.Join(repo, path))
		}
	})

	binDir := sharedTestSidecarDir(t, "octxiliary-image")

	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/Image", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled Image tests failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	assertNoCompiledFallback(t, string(out), "")
	assertCompiledCountAtLeast(t, string(out), 1)
	for _, marker := range []string{
		"PASS Image.LoadInspectAndSaveRoundTrip",
		"PASS Image.MetadataMatchesJpegFixture",
		"PASS Image.LoadMissingFails",
		"PASS Image.LoadCorruptImageFails",
		"PASS Image.SaveUnsupportedExtensionFails",
	} {
		if !strings.Contains(string(out), marker) {
			t.Fatalf("expected marker %q in compiled output, got:\n%s", marker, string(out))
		}
	}
}

func TestCompiledImageMissingSidecarDiagnostic(t *testing.T) {
	repo := filepath.Join("..", "..")
	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/Image/Image.Core.octest", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing image sidecar failure, got success:\n%s", string(out))
	}
	if !strings.Contains(string(out), `Octxiliary sidecar "octxiliary-image" not found`) {
		t.Fatalf("expected image missing sidecar diagnostic, got:\n%s", string(out))
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
