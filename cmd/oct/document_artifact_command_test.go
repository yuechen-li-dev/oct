//go:build integration

package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/cli"
)

func TestOctCumentArtifactsUseOneSemanticDocumentAndRemainDeterministic(t *testing.T) {
	project := filepath.Join("..", "..", "Examples", "OctCumentScientificReport")
	outputRoot := filepath.Join(t.TempDir(), "artifacts")

	run := func() string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if err := cli.Execute([]string{"artifact", project, "--output-root", outputRoot, "--execution", "interpreted"}, &stdout, &stderr); err != nil {
			t.Fatalf("OctCument artifact run failed: %v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
		}
		return stdout.String()
	}

	first := run()
	if !strings.Contains(first, "PRODUCED scientific_report.md") || !strings.Contains(first, "PRODUCED scientific_report.docx") {
		t.Fatalf("expected both renderer artifacts, got %q", first)
	}
	docxPath := filepath.Join(outputRoot, "scientific_report.docx")
	markdownPath := filepath.Join(outputRoot, "scientific_report.md")
	firstDOCX, err := os.ReadFile(docxPath)
	if err != nil {
		t.Fatal(err)
	}
	firstMarkdown, err := os.ReadFile(markdownPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(firstMarkdown, []byte("# Adaptive Filter Experiment")) || !bytes.Contains(firstMarkdown, []byte("| Method | Output SNR (dB) | Status |")) {
		t.Fatalf("Markdown renderer lost semantic content: %q", firstMarkdown)
	}

	second := run()
	if !strings.Contains(second, "UNCHANGED scientific_report.md") || !strings.Contains(second, "UNCHANGED scientific_report.docx") {
		t.Fatalf("expected deterministic unchanged artifacts, got %q", second)
	}
	secondDOCX, err := os.ReadFile(docxPath)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(firstDOCX) != sha256.Sum256(secondDOCX) {
		t.Fatal("DOCX renderer was not byte deterministic")
	}

	zr, err := zip.OpenReader(docxPath)
	if err != nil {
		t.Fatalf("DOCX is not an openable ZIP package: %v", err)
	}
	defer zr.Close()
	parts := map[string]bool{}
	for _, file := range zr.File {
		parts[file.Name] = true
	}
	for _, required := range []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml", "word/_rels/document.xml.rels", "word/styles.xml", "word/numbering.xml"} {
		if !parts[required] {
			t.Errorf("DOCX package missing %s", required)
		}
	}
}
