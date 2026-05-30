package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompiledCsvOctxiliaryWrapper(t *testing.T) {
	repo := filepath.Join("..", "..")
	binDir := t.TempDir()
	buildCsvOctxiliarySidecar(t, repo, binDir)

	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/Csv", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled csv wrapper tests failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
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
	binDir := t.TempDir()
	buildCsvOctxiliarySidecar(t, repo, binDir)

	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/IO/IO.Csv.CompiledSmoke.octest", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled IO csv wrapper tests failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	assertOutputContains(t, string(out), "PASS IO.IOCsvRowMajorCompiledSmoke")
}

func buildCsvOctxiliarySidecar(t *testing.T, repo string, binDir string) {
	t.Helper()
	outPath := filepath.Join(binDir, "octxiliary-csv")
	build := exec.Command("go", "build", "-o", outPath, "./cmd/octxiliary-csv")
	build.Dir = repo
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build octxiliary-csv: %v\n%s", err, strings.TrimSpace(string(out)))
	}
}
