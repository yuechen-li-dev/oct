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
	tools := t.TempDir()
	fakeCompiler := filepath.Join(tools, "fail-compiler.cmd")
	if err := os.WriteFile(fakeCompiler, []byte("@echo off\r\necho deterministic fake compiler failure 1>&2\r\nexit /b 17\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// This checks launcher failure propagation, not Concept installation/build.
	// Give the real launcher deterministic prerequisite and package-tool seams.
	header := filepath.Join(tools, "libraries", "Vulkan", "native", "concept_vulkan.h")
	if err := os.MkdirAll(filepath.Dir(header), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(header, []byte("/* prerequisite fixture only */\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fakeGoSource := `package main
import ("fmt"; "os"; "strings")
func main() {
 switch strings.Join(os.Args[1:], " ") {
 case "run ./tools/prometheus_concept -build-package": fmt.Println("deterministic fake Concept package success")
 case "run ./tools/prometheus_native_manifest -check": fmt.Println("deterministic fake manifest success")
 default: fmt.Fprintf(os.Stderr, "unexpected package tool args: %q\n", os.Args[1:]); os.Exit(19)
 }
}
`
	sourcePath := filepath.Join(tools, "package-tool.go")
	if err := os.WriteFile(sourcePath, []byte(fakeGoSource), 0644); err != nil {
		t.Fatal(err)
	}
	build := exec.Command("go", "build", "-o", filepath.Join(tools, "go.exe"), sourcePath)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build package-tool fixture: %v\n%s", err, out)
	}
	t.Setenv("PATH", tools+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CONCEPT_LIBRARY_ROOT", filepath.Join(tools, "libraries"))
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
	if !strings.Contains(text, "deterministic fake Concept package success") || !strings.Contains(text, "compile common native sources") || !strings.Contains(text, "exit code 17") {
		t.Fatalf("launcher omitted exact failed stage:\n%s", text)
	}
}
