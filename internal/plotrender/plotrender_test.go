package plotrender

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestRenderUsesRequestedPixelDimensions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "plot.png")
	err := Render(Request{FunctionName: "Plot.RenderLine", Kind: KindLine, XS: []float64{0, 1}, YS: []float64{1, 2}, OutputPath: path, Width: PixelLength(400), Height: PixelLength(300)})
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	config, err := png.DecodeConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	if config.Width != 400 || config.Height != 300 {
		t.Fatalf("got %dx%d, want 400x300", config.Width, config.Height)
	}
}
