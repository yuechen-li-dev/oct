//go:build integration

package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"io"
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
	if !bytes.Contains(firstMarkdown, []byte("# Adaptive Filter Experiment")) || !bytes.Contains(firstMarkdown, []byte("| Method | Output SNR (dB) | Status |")) || !bytes.Contains(firstMarkdown, []byte("![Adaptive estimator response under seeded noise](adaptive-flow.png)")) || !bytes.Contains(firstMarkdown, []byte("Figure 1")) {
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
	parts := map[string][]byte{}
	for _, file := range zr.File {
		rc, openErr := file.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		payload, readErr := io.ReadAll(rc)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr := rc.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		parts[file.Name] = payload
	}
	for _, required := range []string{"[Content_Types].xml", "_rels/.rels", "word/document.xml", "word/_rels/document.xml.rels", "word/styles.xml", "word/numbering.xml", "word/header1.xml", "word/footer1.xml"} {
		if _, ok := parts[required]; !ok {
			t.Errorf("DOCX package missing %s", required)
		}
	}
	mediaCount := 0
	for name := range parts {
		if strings.HasPrefix(name, "word/media/image-") {
			mediaCount++
		}
	}
	if mediaCount != 2 {
		t.Errorf("expected two deterministic content-addressed media parts, got %d", mediaCount)
	}
	documentXML := string(parts["word/document.xml"])
	for _, expected := range []string{`<wp:inline`, `<wp:anchor`, `<wp:posOffset>7200000</wp:posOffset>`, `<wp:posOffset>10800000</wp:posOffset>`, `cx="43200000"`, `relativeFrom="page"`, `wp:docPr id="1"`, `wp:docPr id="2"`} {
		if !strings.Contains(documentXML, expected) {
			t.Errorf("document.xml missing %s", expected)
		}
	}
	rels := string(parts["word/_rels/document.xml.rels"])
	for _, expected := range []string{`Id="rId3"`, `Id="rId4"`, `Id="rId5"`, `Id="rId6"`, `relationships/image`} {
		if !strings.Contains(rels, expected) {
			t.Errorf("document relationships missing %s", expected)
		}
	}
	if !bytes.Contains(parts["word/footer1.xml"], []byte(" PAGE ")) {
		t.Error("DOCX footer missing page-number field")
	}
}
