package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompiledGenericOctxiliaryWrapperFixture(t *testing.T) {
	requireSlowOctxiliary(t)
	t.Parallel()
	repo := filepath.Join("..", "..")
	binDir := sharedTestSidecarDir(t, "octxiliary-test-wrapper")
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
	requireSlowOctxiliary(t)
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
	requireSlowOctxiliary(t)
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
	requireSlowOctxiliary(t)
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

func TestCompiledGenericOctxiliarySupportsRecordReturn(t *testing.T) {
	requireSlowOctxiliary(t)
	repo := filepath.Join("..", "..")
	binDir := sharedTestSidecarDir(t, "octxiliary-test-wrapper")
	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Language/Testing/CompiledOctxiliary/valid/generic_wrapper_m6.octest", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected compiled record return fixture to succeed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	if !strings.Contains(string(out), "PASS Main.GenericWrapperM6ScalarListBytesAndVoid") {
		t.Fatalf("expected record return fixture pass, got:\n%s", string(out))
	}
}

func TestCompiledGenericOctxiliaryRejectsUndeclaredRecordArg(t *testing.T) {
	requireSlowOctxiliary(t)
	repo := filepath.Join("..", "..")
	cmd := exec.Command("go", "run", filepath.Join("..", "..", "..", "..", "..", "cmd", "oct"), "pkg", "wrappers")
	cmd.Dir = filepath.Join(repo, "Language", "Testing", "CompiledOctxiliary", "invalid", "undeclared_record_arg")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected undeclared record arg failure, got success:\n%s", string(out))
	}
	if !strings.Contains(string(out), "unsupported transport type") {
		t.Fatalf("expected unsupported transport type diagnostic, got:\n%s", string(out))
	}
}

func TestCompiledGenericOctxiliaryRejectsRecordArgMismatch(t *testing.T) {
	requireSlowOctxiliary(t)
	repo := filepath.Join("..", "..")
	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Language/Testing/CompiledOctxiliary/invalid/record_arg_mismatch/bad_record_arg_mismatch.octest", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected record arg mismatch failure, got success:\n%s", string(out))
	}
	text := string(out)
	if !strings.Contains(text, "argument 1 expects Main.TestOptions, got String") {
		t.Fatalf("expected record arg mismatch diagnostic, got:\n%s", text)
	}
}

func TestInterpretedGenericOctxiliaryWrapperFixture(t *testing.T) {
	requireSlowOctxiliary(t)
	t.Parallel()
	repo := filepath.Join("..", "..")
	binDir := sharedTestSidecarDir(t, "octxiliary-test-wrapper")
	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Language/Testing/InterpretedOctxiliary/valid/interpreted_generic_wrapper_w7b.octest", "--execution", "interpreted")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("interpreted generic wrapper fixture failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	text := string(out)
	for _, expected := range []string{"PASS Main.InterpretedGenericWrapperW7bSuccess", "PASS Main.InterpretedGenericWrapperW7bSidecarError", "PASS Main.InterpretedGenericWrapperW7bSourcePrecedence"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected %s, got:\n%s", expected, text)
		}
	}
}

func TestInterpretedGenericOctxiliaryMissingSidecarMessage(t *testing.T) {
	requireSlowOctxiliary(t)
	repo := filepath.Join("..", "..")
	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Language/Testing/InterpretedOctxiliary/valid/interpreted_generic_wrapper_w7b.octest", "--execution", "interpreted")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing sidecar failure, got success:\n%s", string(out))
	}
	text := string(out)
	for _, expected := range []string{`wrapper Main.EchoStringRaw`, `family TestWrapper`, `wire TestEchoString`, `Octxiliary sidecar "octxiliary-test-wrapper" not found`, `set OCT_WRAPPER_PATH`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("expected missing sidecar diagnostic to contain %q, got:\n%s", expected, text)
		}
	}
}
