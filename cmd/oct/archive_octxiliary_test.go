package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompiledArchiveOctxiliaryWrapper(t *testing.T) {
	repo := filepath.Join("..", "..")
	binDir := t.TempDir()
	buildArchiveOctxiliarySidecars(t, repo, binDir, "octxiliary-archive", "octxiliary-io")

	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/Archive", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled archive wrapper tests failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	assertOutputContains(t, string(out),
		"PASS Archive.ZipListEntriesAndExtractAllRoundTrip",
		"PASS Archive.ZipListEntriesMissingArchiveFails",
		"PASS Archive.ArchiveCompiledMissingZipFails",
	)
}

func TestCompiledArchiveOctxiliaryMissingSidecarMessage(t *testing.T) {
	repo := filepath.Join("..", "..")
	binDir := t.TempDir()
	buildArchiveOctxiliarySidecars(t, repo, binDir, "octxiliary-io")
	target := filepath.Join("Libraries", "Archive", "Archive.Zip.octest")
	cmd := exec.Command("go", "run", "./cmd/oct", "test", target, "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing archive sidecar failure, got success:\n%s", string(out))
	}
	if !strings.Contains(string(out), `Octxiliary sidecar "octxiliary-archive" not found`) {
		t.Fatalf("expected clear missing archive sidecar message, got:\n%s", string(out))
	}
}

func buildArchiveOctxiliarySidecars(t *testing.T, repo string, binDir string, sidecars ...string) {
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
