//go:build toolchain

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
	cmd := exec.Command(sharedTestOctBinary(t), "test", "Language/Testing/CompiledOctxiliary/valid/generic_wrapper_m6.octest", "--execution", "compiled")
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
	requireSlowOctxiliary(t)
	repo := filepath.Join("..", "..")
	cmd := exec.Command(sharedTestOctBinary(t), "test", "Language/Testing/CompiledOctxiliary/valid/generic_wrapper_m6.octest", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing sidecar failure, got success:\n%s", string(out))
	}
	if !strings.Contains(string(out), `Octxiliary sidecar "octxiliary-test-wrapper" not found`) {
		t.Fatalf("expected clear missing sidecar message, got:\n%s", string(out))
	}
	if strings.Contains(string(out), "panic:") || strings.Contains(string(out), "goroutine ") {
		t.Fatalf("expected missing-sidecar Oct diagnostic without Go panic substrate, got:\n%s", string(out))
	}
}

func TestCompiledGenericOctxiliarySupportsRecordReturn(t *testing.T) {
	requireSlowOctxiliary(t)
	repo := filepath.Join("..", "..")
	binDir := sharedTestSidecarDir(t, "octxiliary-test-wrapper")
	cmd := exec.Command(sharedTestOctBinary(t), "test", "Language/Testing/CompiledOctxiliary/valid/generic_wrapper_m6.octest", "--execution", "compiled")
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
	cmd := exec.Command(sharedTestOctBinary(t), "pkg", "wrappers")
	cmd.Dir = filepath.Join(repo, "Language", "Testing", "CompiledOctxiliary", "invalid", "Packages", "WrapperUndeclaredRecordArg")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected undeclared record arg failure, got success:\n%s", string(out))
	}
	if !strings.Contains(string(out), "unsupported transport type") {
		t.Fatalf("expected unsupported transport type diagnostic, got:\n%s", string(out))
	}
}

func TestInterpretedGenericOctxiliaryWrapperFixture(t *testing.T) {
	requireSlowOctxiliary(t)
	t.Parallel()
	repo := filepath.Join("..", "..")
	binDir := sharedTestSidecarDir(t, "octxiliary-test-wrapper")
	cmd := exec.Command(sharedTestOctBinary(t), "test", "Language/Testing/InterpretedOctxiliary/valid/interpreted_generic_wrapper_w7b.octest", "--execution", "interpreted")
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
	cmd := exec.Command(sharedTestOctBinary(t), "test", "Language/Testing/InterpretedOctxiliary/valid/interpreted_generic_wrapper_w7b.octest", "--execution", "interpreted")
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
