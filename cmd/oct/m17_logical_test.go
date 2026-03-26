package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestM17BuildAcceptsValidLogicalPrograms(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Bool { return true and not false or false }\n")

	stdout, stderr, err := executeCLI("build", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("build failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "build succeeded") {
		t.Fatalf("expected build success output, got %q", stdout)
	}
}

func TestM17BuildRejectsInvalidLogicalPrograms(t *testing.T) {
	root := t.TempDir()
	writePkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Bool { return 1 and 2 }\n")

	stdout, stderr, err := executeCLI("build", filepath.Join(root, "Main", "main.oct"))
	if err == nil {
		t.Fatalf("expected build to fail, stdout=%q", stdout)
	}
	if strings.Contains(stdout, "build succeeded") {
		t.Fatalf("did not expect build success output, got %q", stdout)
	}
	if !strings.Contains(stderr, "operator 'and' requires Bool operands") {
		t.Fatalf("unexpected stderr: %q", stderr)
	}
}
