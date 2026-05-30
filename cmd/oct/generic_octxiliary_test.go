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

func TestCompiledGenericOctxiliaryRejectsManifestReturnMismatch(t *testing.T) {
	repo := filepath.Join("..", "..")
	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Language/Testing/CompiledOctxiliary/invalid/return_mismatch/bad_return.octest", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected manifest return mismatch failure, got success:\n%s", string(out))
	}
	text := string(out)
	if !strings.Contains(text, "manifest return") || !strings.Contains(text, "BadReturn") {
		t.Fatalf("expected manifest return mismatch diagnostic for BadReturn, got:\n%s", text)
	}
}

func TestCompiledGenericOctxiliaryRejectsManifestFallibleMismatch(t *testing.T) {
	repo := filepath.Join("..", "..")
	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Language/Testing/CompiledOctxiliary/invalid/fallible_mismatch/bad_fallible.octest", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected manifest fallible mismatch failure, got success:\n%s", string(out))
	}
	text := string(out)
	if !strings.Contains(text, "manifest fallible") || !strings.Contains(text, "BadFallible") {
		t.Fatalf("expected manifest fallible mismatch diagnostic for BadFallible, got:\n%s", text)
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
