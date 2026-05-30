package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompiledMarkdownHelpersNoSidecar(t *testing.T) {
	repo := filepath.Join("..", "..")
	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/Markdown", "--execution", "compiled")
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled markdown tests failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	assertOutputContains(t, string(out),
		"PASS Markdown.MarkdownBlocksM0",
		"PASS Markdown.MarkdownM1Helpers",
		"PASS Markdown.MarkdownReportAndTablesM0",
	)
}
