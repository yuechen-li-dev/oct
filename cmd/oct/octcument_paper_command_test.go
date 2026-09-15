//go:build integration

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/cli"
)

func TestOctCumentPaperPDFUsesArtifactLatexBytes(t *testing.T) {
	if os.Getenv("OCT_LATEX_ENGINE") == "" {
		t.Skip("OCT_LATEX_ENGINE is required for the PDF materialization lane")
	}
	project := filepath.Join("..", "..", "Examples", "OctCumentPaper")
	outputRoot := t.TempDir()
	var stdout, stderr bytes.Buffer
	if err := cli.Execute([]string{"artifact", project, "--output-root", outputRoot, "--execution", "compiled"}, &stdout, &stderr); err != nil {
		t.Fatalf("academic artifact failed: %v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	pdfTex, err := os.ReadFile(filepath.Join(outputRoot, "paper.tex"))
	if err != nil {
		t.Fatal(err)
	}
	latexAPI, err := os.ReadFile(filepath.Join(outputRoot, "latex-only", "paper.tex"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pdfTex, latexAPI) {
		t.Fatal("Artifact.Pdf and Artifact.Latex did not preserve the exact same textual lowering")
	}
	pdf, err := os.ReadFile(filepath.Join(outputRoot, "paper.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatal("paper.pdf is not parseable as a PDF header")
	}
}
