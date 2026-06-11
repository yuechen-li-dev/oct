package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompiledCsvOctxiliaryWrapper(t *testing.T) {
	t.Parallel()
	repo := filepath.Join("..", "..")
	binDir := sharedTestSidecarDir(t, "octxiliary-csv")

	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/Csv", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled csv wrapper tests failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	assertNoCompiledFallback(t, string(out), "")
	assertCompiledCountAtLeast(t, string(out), 1)
	assertOutputContains(t, string(out),
		"PASS Csv.CsvSimpleReadFixture",
		"PASS Csv.CsvSimpleWriteReadBack",
		"PASS Csv.CsvEscapedCellRoundTrip",
		"PASS Csv.CsvEmptyOuterRowsWriteReadBack",
		"PASS Csv.CsvRaggedRowsRoundTrip",
		"PASS Csv.CsvMissingFileReturnsError",
	)
}

func TestCompiledCsvOctxiliaryMissingSidecarMessage(t *testing.T) {
	repo := filepath.Join("..", "..")
	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/Csv/Csv.octest", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing csv sidecar failure, got success:\n%s", string(out))
	}
	if !strings.Contains(string(out), `Octxiliary sidecar "octxiliary-csv" not found`) {
		t.Fatalf("expected clear missing csv sidecar message, got:\n%s", string(out))
	}
}

func TestCompiledIOCsvRowMajorOctxiliaryWrapper(t *testing.T) {
	repo := filepath.Join("..", "..")
	binDir := sharedTestSidecarDir(t, "octxiliary-csv")

	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/IO/IO.Csv.CompiledSmoke.octest", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled IO csv wrapper tests failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	assertNoCompiledFallback(t, string(out), "")
	assertCompiledCountAtLeast(t, string(out), 1)
	assertOutputContains(t, string(out), "PASS IO.IOCsvRowMajorCompiledSmoke")
}

func TestAutoIOCsvOctxiliarySidecarDiscoveryDoesNotFallback(t *testing.T) {
	stdout, stderr, err := executeCLIWithSidecars(t, "test", filepath.Join("..", "..", "Libraries", "IO", "IO.Csv.CompiledSmoke.octest"), "octxiliary-csv")
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
