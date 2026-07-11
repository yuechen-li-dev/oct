//go:build windows

package prometheus

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsNativeBuildPropagatesCompilerFailure(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	fakeCompiler := filepath.Join(t.TempDir(), "fail-compiler.cmd")
	if err := os.WriteFile(fakeCompiler, []byte("@echo off\r\necho deterministic fake compiler failure 1>&2\r\nexit /b 17\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, "internal", "prometheus", "native", "build_windows.cmd")
	cmd := exec.Command("cmd.exe", "/d", "/c", script)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "PROMETHEUS_NATIVE_CC="+fakeCompiler)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("launcher reported success after compiler failure:\n%s", out)
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok || exitErr.ExitCode() != 17 {
		t.Fatalf("launcher exit = %v, want 17:\n%s", err, out)
	}
	text := string(out)
	if !strings.Contains(text, "compile common native sources") || !strings.Contains(text, "exit code 17") {
		t.Fatalf("launcher omitted exact failed stage:\n%s", text)
	}
}
