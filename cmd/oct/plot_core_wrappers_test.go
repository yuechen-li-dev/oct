package main

import (
	"os"
	"strings"
	"testing"
)

func TestConveniencePlotBuiltinsRemainNoImport(t *testing.T) {
	outputPath := "m104_convenience_plot.png"
	sourcePath := writeSourceFile(t, "m104_convenience_plot.oct", "fn Main() -> Int {\n    return PlotLine([0.0, 1.0, 2.0], [0.0, 1.0, 4.0], \""+outputPath+"\")\n}\n")

	stdout, stderr, err := executeCLI("run", sourcePath)
	if err != nil {
		t.Fatalf("expected convenience PlotLine to run without import: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if _, statErr := os.Stat(outputPath); statErr != nil {
		t.Fatalf("expected convenience plot artifact %q to exist: %v", outputPath, statErr)
	}
	_ = os.Remove(outputPath)
}

func TestPlotCoreWrappers(t *testing.T) {
	t.Parallel()
	workDir := newWrapperTempProject(t)
	root := repoPath(t, "Libraries", "Plot")
	stdout, stderr, err := executeCLIWithSidecarsInDir(t, workDir, "test", root, "octxiliary-plot", "octxiliary-io")
	if err != nil {
		t.Fatalf("oct test failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	assertNoCompiledFallback(t, stdout, stderr)
	assertCompiledCountAtLeast(t, stdout, 1)

	expectedPasses := []string{
		"PASS Plot.LinePlotWritesConfiguredPng",
		"PASS Plot.ScatterPlotWritesConfiguredPng",
		"PASS Plot.HistogramWritesConfiguredPng",
		"PASS Plot.InvalidOutputPathFails",
		"PASS Plot.MismatchedDataLengthsFail",
		"PASS Plot.InvalidHistogramBinsFail",
	}

	for _, marker := range expectedPasses {
		if !strings.Contains(stdout, marker) {
			t.Fatalf("expected marker %q in stdout, got %q", marker, stdout)
		}
	}
}
