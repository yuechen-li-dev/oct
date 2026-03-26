package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestM22dSignalGenerationCorrectness(t *testing.T) {
	root := setupM22dFixture(t)
	stdout, stderr, err := executeCLI("test", root)
	if err != nil {
		t.Fatalf("oct test failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS Signal.AnalyzeSignalReturnsConsistentShapes") {
		t.Fatalf("expected fact pass output, got %q", stdout)
	}
	if !strings.Contains(stdout, "PASS Signal.GeneratedSignalContainsKnownSamples[0]") {
		t.Fatalf("expected theory case pass output, got %q", stdout)
	}
}

func TestM22dPlotGeneration(t *testing.T) {
	root := setupM22dSignalFixture(t)
	outputPath := filepath.Join(root, "signal_plot.png")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"import Signal",
		"",
		"fn Main() -> Int {",
		"    let result = Signal.AnalyzeSignal(0.0, 0.5, 3)",
		"    return PlotLine(result.X, result.Smoothed, " + strconv.Quote(outputPath) + ")",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("run", filepath.Join(root, "Main", "main.oct"))
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if stdout != "0\n" {
		t.Fatalf("expected success code 0, got %q", stdout)
	}

	info, statErr := os.Stat(outputPath)
	if statErr != nil {
		t.Fatalf("expected output png %s: %v", outputPath, statErr)
	}
	if info.Size() == 0 {
		t.Fatalf("expected non-empty output png %s", outputPath)
	}
}

func TestM22dPackageIntegrationRunAndBuild(t *testing.T) {
	root := setupM22dFixture(t)
	entry := filepath.Join(root, "Main", "main.oct")

	stdout, stderr, err := executeCLI("run", entry)
	if err != nil {
		t.Fatalf("run failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "12\n") {
		t.Fatalf("expected length print output, got %q", stdout)
	}
	if !strings.Contains(stdout, "0\n") {
		t.Fatalf("expected first sample print output, got %q", stdout)
	}

	buildStdout, buildStderr, buildErr := executeCLI("build", entry)
	if buildErr != nil {
		t.Fatalf("build failed: %v stdout=%s stderr=%s", buildErr, buildStdout, buildStderr)
	}
	if !strings.Contains(buildStdout, "build succeeded") {
		t.Fatalf("expected build success output, got %q", buildStdout)
	}
}

func TestM22dBuildFailureDoesNotEmitArtifact(t *testing.T) {
	root := setupM22dSignalFixture(t)
	entry := filepath.Join(root, "Main", "main.oct")
	writePkgFile(t, root, "Main", "main.oct", strings.Join([]string{
		"package Main",
		"import Signal",
		"",
		"fn Main() -> Int {",
		"    let result = Signal.AnalyzeSignal(\"bad\", 0.5, 3)",
		"    Print(result)",
		"    return 0",
		"}",
	}, "\n"))

	stdout, stderr, err := executeCLI("build", entry)
	if err == nil {
		t.Fatalf("expected build failure, got success with stdout %q", stdout)
	}
	if !strings.Contains(stderr, "function 'Signal.AnalyzeSignal' argument 1 expects Float, got String") {
		t.Fatalf("unexpected stderr %q", stderr)
	}
	if _, statErr := os.Stat(entry + ".octbin"); !os.IsNotExist(statErr) {
		t.Fatalf("expected no artifact, stat err = %v", statErr)
	}
}

func setupM22dSignalFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyDir(t, filepath.Join("..", "..", "testdata", "m22d", "valid", "Signal"), filepath.Join(root, "Signal"))
	return root
}

func setupM22dFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	copyDir(t, filepath.Join("..", "..", "testdata", "m22d", "valid", "Signal"), filepath.Join(root, "Signal"))
	copyDir(t, filepath.Join("..", "..", "testdata", "m22d", "valid", "Main"), filepath.Join(root, "Main"))
	return root
}
