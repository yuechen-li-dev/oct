package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompiledGenericOctxiliaryWrapperFixture(t *testing.T) {
	repo := filepath.Join("..", "..")
	binDir := t.TempDir()
	sidecar := filepath.Join(binDir, "octxiliary-test-wrapper")
	build := exec.Command("go", "build", "-o", sidecar, "./cmd/octxiliary-test-wrapper")
	build.Dir = repo
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build test sidecar: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Language/Testing/CompiledOctxiliary/valid/generic_wrapper_m6.octest", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled generic wrapper fixture failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	if !strings.Contains(string(out), "PASS Main.GenericWrapperM6ScalarListBytesAndVoid") || !strings.Contains(string(out), "PASS Main.GenericWrapperM6SidecarErrorPropagates") {
		t.Fatalf("expected generic wrapper fixture passes, got:\n%s", string(out))
	}
}

func TestCompiledGenericOctxiliaryMissingSidecarMessage(t *testing.T) {
	repo := filepath.Join("..", "..")
	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Language/Testing/CompiledOctxiliary/valid/generic_wrapper_m6.octest", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing sidecar failure, got success:\n%s", string(out))
	}
	if !strings.Contains(string(out), `Octxiliary sidecar "octxiliary-test-wrapper" not found`) {
		t.Fatalf("expected clear missing sidecar message, got:\n%s", string(out))
	}
}
