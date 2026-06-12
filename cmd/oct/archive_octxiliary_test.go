package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCompiledArchiveOctxiliaryWrapper(t *testing.T) {
	requireSlowOctxiliary(t)
	t.Parallel()
	workDir := newWrapperTempProject(t)
	copyOctxiliaryFixtureDir(t, "mx103c_zip_src", filepath.Join(workDir, "mx103c_zip_src"))
	target := repoPath(t, "Libraries", "Archive")

	stdout, stderr, err := executeOctWithSidecarsInDir(t, workDir, []string{"test", target, "--execution", "compiled"}, "octxiliary-archive", "octxiliary-io")
	if err != nil {
		t.Fatalf("compiled archive wrapper tests failed: %v\nstderr:%s\nstdout:%s", err, strings.TrimSpace(stderr), stdout)
	}
	assertNoCompiledFallback(t, stdout, stderr)
	assertCompiledCountAtLeast(t, stdout, 1)
	assertOutputContains(t, stdout,
		"PASS Archive.ZipListEntriesAndExtractAllRoundTrip",
		"PASS Archive.ZipListEntriesMissingArchiveFails",
		"PASS Archive.ArchiveCompiledMissingZipFails",
	)
}

func TestCompiledArchiveOctxiliaryMissingSidecarMessage(t *testing.T) {
	requireSlowOctxiliary(t)
	binDir := t.TempDir()
	buildTestSidecarsInDir(t, binDir, "octxiliary-io")
	workDir := newWrapperTempProject(t)
	target := repoPath(t, "Libraries", "Archive", "Archive.Zip.octest")
	stdout, stderr, err := executeOctWithCustomWrapperPathInDir(t, workDir, binDir, []string{"test", target, "--execution", "compiled"})
	if err == nil {
		t.Fatalf("expected missing archive sidecar failure, got success:\n%s%s", stdout, stderr)
	}
	if !strings.Contains(stdout+stderr, `Octxiliary sidecar "octxiliary-archive" not found`) {
		t.Fatalf("expected clear missing archive sidecar message, got:\nstdout:%s\nstderr:%s", stdout, stderr)
	}
}
