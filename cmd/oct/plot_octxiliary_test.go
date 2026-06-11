package main

import (
	"strings"
	"testing"
)

func TestCompiledPlotOctxiliaryWrapper(t *testing.T) {
	t.Parallel()
	workDir := newWrapperTempProject(t)
	target := repoPath(t, "Libraries", "Plot")

	stdout, stderr, err := executeOctWithSidecarsInDir(t, workDir, []string{"test", target, "--execution", "compiled"}, "octxiliary-plot", "octxiliary-io")
	if err != nil {
		t.Fatalf("compiled plot wrapper tests failed: %v\nstderr:%s\nstdout:%s", err, strings.TrimSpace(stderr), stdout)
	}
	assertNoCompiledFallback(t, stdout, stderr)
	assertCompiledCountAtLeast(t, stdout, 1)
	assertOutputContains(t, stdout,
		"PASS Plot.LinePlotWritesConfiguredPng",
		"PASS Plot.ScatterPlotWritesConfiguredPng",
		"PASS Plot.HistogramWritesConfiguredPng",
		"PASS Plot.InvalidOutputPathFails",
		"PASS Plot.MismatchedDataLengthsFail",
		"PASS Plot.InvalidHistogramBinsFail",
	)
}

func TestCompiledPlotOctxiliaryMissingSidecarMessage(t *testing.T) {
	workDir := newWrapperTempProject(t)
	target := repoPath(t, "Libraries", "Plot")
	stdout, stderr, err := executeOctWithCustomWrapperPathInDir(t, workDir, t.TempDir(), []string{"test", target, "--execution", "compiled"})
	if err == nil {
		t.Fatalf("expected missing plot sidecar failure, got success:\n%s%s", stdout, stderr)
	}
	if !strings.Contains(stdout+stderr, `Octxiliary sidecar "octxiliary-plot" not found`) {
		t.Fatalf("expected clear missing plot sidecar message, got:\nstdout:%s\nstderr:%s", stdout, stderr)
	}
}
