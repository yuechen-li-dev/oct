package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompiledTextOctxiliaryWrapper(t *testing.T) {
	repo := filepath.Join("..", "..")
	binDir := sharedTestSidecarDir(t, "octxiliary-text")

	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/Text", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled text wrapper tests failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	assertOutputContains(t, string(out),
		"PASS Text.RegexMatchFindReplaceSplit",
		"PASS Text.RegexInvalidPatternFails",
		"PASS Text.RegexNegativeMatchReturnsFalse",
		"PASS Text.TextCompiledRegexSmoke",
	)
}

func TestCompiledTextOctxiliaryMissingSidecarMessage(t *testing.T) {
	repo := filepath.Join("..", "..")
	target := filepath.Join("Libraries", "Text", "Text.CompiledSmoke.octest")
	cmd := exec.Command("go", "run", "./cmd/oct", "test", target, "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing text sidecar failure, got success:\n%s", string(out))
	}
	if !strings.Contains(string(out), `Octxiliary sidecar "octxiliary-text" not found`) {
		t.Fatalf("expected clear missing text sidecar message, got:\n%s", string(out))
	}
}
