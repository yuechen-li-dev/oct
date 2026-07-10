//go:build toolchain

package main

import (
	"strings"
	"testing"
)

func TestCompiledCsvOctxiliaryWrapper(t *testing.T) {
	requireSlowOctxiliary(t)
	t.Parallel()
	workDir := newWrapperTempProject(t)
	target := repoPath(t, "Libraries", "Csv")

	stdout, stderr, err := executeOctWithSidecarsInDir(t, workDir, []string{"test", target, "--execution", "compiled"}, "octxiliary-csv")
	if err != nil {
		t.Fatalf("compiled csv wrapper tests failed: %v\nstderr:%s\nstdout:%s", err, strings.TrimSpace(stderr), stdout)
	}
	assertNoCompiledFallback(t, stdout, stderr)
	assertCompiledCountAtLeast(t, stdout, 1)
	assertOutputContains(t, stdout,
		"PASS Csv.CsvSimpleReadFixture",
		"PASS Csv.CsvSimpleWriteReadBack",
		"PASS Csv.CsvEscapedCellRoundTrip",
		"PASS Csv.CsvEmptyOuterRowsWriteReadBack",
		"PASS Csv.CsvRaggedRowsRoundTrip",
		"PASS Csv.CsvMissingFileReturnsError",
	)
}

func TestCompiledCsvOctxiliaryMissingSidecarMessage(t *testing.T) {
	requireSlowOctxiliary(t)
	workDir := newWrapperTempProject(t)
	target := repoPath(t, "Libraries", "Csv", "Csv.octest")
	stdout, stderr, err := executeOctWithCustomWrapperPathInDir(t, workDir, t.TempDir(), []string{"test", target, "--execution", "compiled"})
	if err == nil {
		t.Fatalf("expected missing csv sidecar failure, got success:\n%s%s", stdout, stderr)
	}
	if !strings.Contains(stdout+stderr, `Octxiliary sidecar "octxiliary-csv" not found`) {
		t.Fatalf("expected clear missing csv sidecar message, got:\nstdout:%s\nstderr:%s", stdout, stderr)
	}
}

func TestCompiledIOCsvRowMajorOctxiliaryWrapper(t *testing.T) {
	requireSlowOctxiliary(t)
	t.Parallel()
	workDir := newWrapperTempProject(t)
	target := repoPath(t, "Libraries", "IO", "IO.Csv.CompiledSmoke.octest")

	stdout, stderr, err := executeOctWithSidecarsInDir(t, workDir, []string{"test", target, "--execution", "compiled"}, "octxiliary-csv")
	if err != nil {
		t.Fatalf("compiled IO csv wrapper tests failed: %v\nstderr:%s\nstdout:%s", err, strings.TrimSpace(stderr), stdout)
	}
	assertNoCompiledFallback(t, stdout, stderr)
	assertCompiledCountAtLeast(t, stdout, 1)
	assertOutputContains(t, stdout, "PASS IO.IOCsvRowMajorCompiledSmoke")
}

func TestAutoIOCsvOctxiliarySidecarDiscoveryDoesNotFallback(t *testing.T) {
	requireSlowOctxiliary(t)
	t.Parallel()
	workDir := newWrapperTempProject(t)
	target := repoPath(t, "Libraries", "IO", "IO.Csv.CompiledSmoke.octest")
	stdout, stderr, err := executeCLIWithSidecarsInDir(t, workDir, "test", target, "octxiliary-csv")
	if err != nil {
		t.Fatalf("auto IO csv wrapper test failed: %v\nstderr:%s\nstdout:%s", err, stderr, stdout)
	}
	assertNoMissingSidecarFallback(t, stdout, stderr)
	assertNoCompiledFallback(t, stdout, stderr)
	assertCompiledCountAtLeast(t, stdout, 1)
	assertOutputContains(t, stdout,
		"PASS IO.IOCsvRowMajorCompiledSmoke",
		"Execution summary: compiled: 1 interpreted fallback: 0",
	)
}
