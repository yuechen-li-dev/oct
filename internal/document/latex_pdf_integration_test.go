//go:build integration

package document

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestLatexPDFMaterializationUsesExactBundleAndIsReproducible(t *testing.T) {
	if _, err := latexEngine(); err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "plot.png"), tinyPNG(t), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "references.bib"), []byte("@article{smith2024,title={Stable},author={Smith, Ada},year={2024}}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bundle, err := Latex(latexFixture(dir))
	if err != nil {
		t.Fatal(err)
	}
	first, err := PDF(bundle)
	if err != nil {
		t.Fatal(err)
	}
	second, err := PDF(bundle)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(first.Bytes, []byte("%PDF-")) {
		t.Fatal("materialized output is not a PDF")
	}
	if !bytes.Equal(first.Bytes, second.Bytes) {
		t.Fatal("PDF bytes differ under the same installed toolchain")
	}
	if first.Engine == "" || first.EngineVersion == "" {
		t.Fatal("PDF evidence did not record engine identity")
	}
}

func tinyPNG(t *testing.T) []byte {
	t.Helper()
	var out bytes.Buffer
	canvas := image.NewRGBA(image.Rect(0, 0, 1, 1))
	canvas.Set(0, 0, color.RGBA{A: 255})
	if err := png.Encode(&out, canvas); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
