package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompiledPlotOctxiliaryWrapper(t *testing.T) {
	repo := filepath.Join("..", "..")
	binDir := t.TempDir()
	buildPlotOctxiliarySidecar(t, repo, binDir)
	buildPlotIOSidecar(t, repo, binDir)

	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/Plot", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled plot wrapper tests failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	assertOutputContains(t, string(out),
		"PASS Plot.LinePlotWritesConfiguredPng",
		"PASS Plot.ScatterPlotWritesConfiguredPng",
		"PASS Plot.HistogramWritesConfiguredPng",
		"PASS Plot.InvalidOutputPathFails",
		"PASS Plot.MismatchedDataLengthsFail",
		"PASS Plot.InvalidHistogramBinsFail",
	)
}

func TestCompiledPlotOctxiliaryMissingSidecarMessage(t *testing.T) {
	repo := filepath.Join("..", "..")
	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/Plot", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing plot sidecar failure, got success:\n%s", string(out))
	}
	if !strings.Contains(string(out), `Octxiliary sidecar "octxiliary-plot" not found`) {
		t.Fatalf("expected clear missing plot sidecar message, got:\n%s", string(out))
	}
}

func buildPlotOctxiliarySidecar(t *testing.T, repo string, binDir string) {
	t.Helper()
	outPath := filepath.Join(binDir, "octxiliary-plot")
	build := exec.Command("go", "build", "-o", outPath, "./cmd/octxiliary-plot")
	build.Dir = repo
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build octxiliary-plot: %v\n%s", err, strings.TrimSpace(string(out)))
	}
}

func buildPlotIOSidecar(t *testing.T, repo string, binDir string) {
	t.Helper()
	outPath := filepath.Join(binDir, "octxiliary-io")
	build := exec.Command("go", "build", "-o", outPath, "./cmd/octxiliary-io")
	build.Dir = repo
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build octxiliary-io: %v\n%s", err, strings.TrimSpace(string(out)))
	}
}
