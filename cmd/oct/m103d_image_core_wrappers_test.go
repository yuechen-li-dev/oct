package main

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"strings"
	"testing"
)

func TestMx103dImageCoreWrappers(t *testing.T) {
	if err := synthesizeImageCoreFixtures(); err != nil {
		t.Fatalf("synthesize fixtures: %v", err)
	}
	t.Cleanup(func() {
		for _, path := range []string{
			"mx103d_fixture_rect.png",
			"mx103d_fixture_rect.jpg",
			"mx103d_fixture_corrupt.img",
		} {
			_ = os.Remove(path)
		}
	})

	root := "../../Libraries/Image"
	stdout, stderr, err := executeCLI("test", root)
	if err != nil {
		t.Fatalf("oct test failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}

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

func synthesizeImageCoreFixtures() error {
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

	if err := writePNG("mx103d_fixture_rect.png", rect); err != nil {
		return err
	}
	if err := writeJPEG("mx103d_fixture_rect.jpg", rect); err != nil {
		return err
	}
	return os.WriteFile("mx103d_fixture_corrupt.img", []byte("not an image payload\n"), 0o644)
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
