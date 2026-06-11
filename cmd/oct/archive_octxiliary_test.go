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
	binDir := sharedTestSidecarDir(t, "octxiliary-archive", "octxiliary-io")

	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/Archive", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled archive wrapper tests failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	assertNoCompiledFallback(t, string(out), "")
	assertCompiledCountAtLeast(t, string(out), 1)
	assertOutputContains(t, string(out),
		"PASS Archive.ZipListEntriesAndExtractAllRoundTrip",
		"PASS Archive.ZipListEntriesMissingArchiveFails",
		"PASS Archive.ArchiveCompiledMissingZipFails",
	)
}

func TestCompiledArchiveOctxiliaryMissingSidecarMessage(t *testing.T) {
	repo := filepath.Join("..", "..")
	binDir := t.TempDir()
	buildTestSidecarsInDir(t, binDir, "octxiliary-io")
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
