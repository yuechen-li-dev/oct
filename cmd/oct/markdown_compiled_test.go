//go:build integration

package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestCompiledMarkdownHelpersNoSidecar(t *testing.T) {
	workDir := t.TempDir()
	target := repoPath(t, "Libraries", "Markdown")
	cmd := exec.Command(sharedTestOctBinary(t), "test", target, "--execution", "compiled")
	cmd.Dir = workDir
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
