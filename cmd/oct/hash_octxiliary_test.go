package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompiledHashOctxiliaryWrapper(t *testing.T) {
	repo := filepath.Join("..", "..")
	binDir := t.TempDir()
	buildHashOctxiliarySidecars(t, repo, binDir)

	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/Hash", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled hash wrapper tests failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	assertOutputContains(t, string(out),
		"PASS Hash.Sha256TextKnownValueChecks",
		"PASS Hash.Sha256BytesKnownValueChecks",
		"PASS Hash.Sha256FileKnownValueChecks",
		"PASS Hash.Sha256FileMissingFails",
	)
}

func TestCompiledHashOctxiliaryMissingSidecarMessage(t *testing.T) {
	repo := filepath.Join("..", "..")
	target := filepath.Join("Libraries", "Hash", "Hash.CompiledSmoke.octest")
	cmd := exec.Command("go", "run", "./cmd/oct", "test", target, "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing hash sidecar failure, got success:\n%s", string(out))
	}
	if !strings.Contains(string(out), `Octxiliary sidecar "octxiliary-hash" not found`) {
		t.Fatalf("expected clear missing hash sidecar message, got:\n%s", string(out))
	}
}

func buildHashOctxiliarySidecars(t *testing.T, repo string, binDir string) {
	t.Helper()
	for _, sidecar := range []string{"octxiliary-hash", "octxiliary-io"} {
		outPath := filepath.Join(binDir, sidecar)
		build := exec.Command("go", "build", "-o", outPath, "./cmd/"+sidecar)
		build.Dir = repo
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", sidecar, err, strings.TrimSpace(string(out)))
		}
	}
}
