package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadForTestIgnoresGeneratedCompiledTestRunnerFiles(t *testing.T) {
	root := t.TempDir()
	pkgDir := filepath.Join(root, "Pkg")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatalf("mkdir package: %v", err)
	}
	src := "package Pkg\n[Fact]\nfn KeepsRunning() -> Void { Assert.True(true) }\n"
	if err := os.WriteFile(filepath.Join(pkgDir, "pkg.octest"), []byte(src), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	runner := "package Pkg\nfn main() -> Int { return 0 }\n"
	if err := os.WriteFile(filepath.Join(pkgDir, "zz_oct_test_runner_123_456.octest"), []byte(runner), 0o644); err != nil {
		t.Fatalf("write generated runner: %v", err)
	}
	if _, err := LoadForTest(pkgDir); err != nil {
		t.Fatalf("LoadForTest should ignore generated runners, got error: %v", err)
	}
}
