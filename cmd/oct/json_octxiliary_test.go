package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompiledJsonOctxiliaryWrapper(t *testing.T) {
	repo := filepath.Join("..", "..")
	binDir := t.TempDir()
	buildJsonOctxiliarySidecars(t, repo, binDir, "octxiliary-json", "octxiliary-io")

	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/Json", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled json wrapper tests failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	assertOutputContains(t, string(out),
		"PASS Json.JsonSaveLoadRoundTrip",
		"PASS Json.JsonInvalidSaveFails",
		"PASS Json.JsonCompiledMissingFileFails",
	)
}

func TestCompiledJsonOctxiliaryMissingSidecarMessage(t *testing.T) {
	repo := filepath.Join("..", "..")
	target := filepath.Join("Libraries", "Json", "Json.octest")
	cmd := exec.Command("go", "run", "./cmd/oct", "test", target, "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing json sidecar failure, got success:\n%s", string(out))
	}
	if !strings.Contains(string(out), `Octxiliary sidecar "octxiliary-json" not found`) {
		t.Fatalf("expected clear missing json sidecar message, got:\n%s", string(out))
	}
}

func buildJsonOctxiliarySidecars(t *testing.T, repo string, binDir string, sidecars ...string) {
	t.Helper()
	for _, sidecar := range sidecars {
		outPath := filepath.Join(binDir, sidecar)
		build := exec.Command("go", "build", "-o", outPath, "./cmd/"+sidecar)
		build.Dir = repo
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v\n%s", sidecar, err, strings.TrimSpace(string(out)))
		}
	}
}
