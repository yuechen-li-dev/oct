package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildWasmTargetWritesDirectModuleAndIdentity(t *testing.T) {
	root := filepath.Join(t.TempDir(), "WasmCompute")
	copyDir(t, filepath.Join("..", "..", "Examples", "WasmCompute"), root)
	stdout, stderr, err := executeCLIArgs("build", root, "--target", "wasm")
	if err != nil {
		t.Fatalf("wasm build: %v\nstderr: %s", err, stderr)
	}
	artifact := filepath.Join(root, "WasmCompute.wasm")
	contents, err := os.ReadFile(artifact)
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) < 8 || string(contents[:4]) != "\x00asm" {
		t.Fatalf("%s is not a WebAssembly binary", artifact)
	}
	for _, want := range []string{"build succeeded: " + artifact, "target: wasm", "sha256: "} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q: %s", want, stdout)
		}
	}
}

func TestBuildRejectsUnknownTarget(t *testing.T) {
	_, stderr, err := executeCLIArgs("build", "ignored.oct", "--target", "wat")
	if err == nil || !strings.Contains(stderr, `unknown build target "wat"`) {
		t.Fatalf("unexpected error=%v stderr=%q", err, stderr)
	}
}
