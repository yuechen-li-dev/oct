//go:build integration

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/cli"
	"github.com/yuechen-li-dev/oct/internal/octagon"
)

func TestPrometheusFftAlgorithmLabM1ArtifactWritesDeterministicVisibleOutputs(t *testing.T) {
	cwdStart, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	root := filepath.Clean(filepath.Join(cwdStart, "..", ".."))
	outDir := filepath.Join(root, "out", "prometheus_fft_algorithm_lab", "m1")
	_ = os.RemoveAll(outDir)

	if err := os.Chdir(root); err != nil {
		t.Fatalf("chdir repo root: %v", err)
	}
	defer func() { _ = os.Chdir(cwdStart) }()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"artifact", "Experiments/PrometheusFftAlgorithmLab/M1"}, &stdout, &stderr); err != nil {
		t.Fatalf("artifact command failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	if !strings.Contains(strings.ToLower(stdout.String()), "checkpoint") {
		t.Fatalf("expected artifact output to include checkpoint logs, got:\n%s", stdout.String())
	}

	paths := []string{
		filepath.Join(outDir, "m1_fft_cases.octagon"),
		filepath.Join(outDir, "m1_fft_results.octagon"),
		filepath.Join(outDir, "m1_fft_plan_traces.octagon"),
		filepath.Join(outDir, "m1_fft_report.md"),
	}
	for _, p := range paths {
		info, statErr := os.Stat(p)
		if statErr != nil {
			t.Fatalf("expected artifact at %s: %v", p, statErr)
		}
		if info.Size() == 0 {
			t.Fatalf("expected non-empty artifact at %s", p)
		}
	}

	octagonArtifacts := []string{
		filepath.Join(outDir, "m1_fft_cases.octagon"),
		filepath.Join(outDir, "m1_fft_results.octagon"),
		filepath.Join(outDir, "m1_fft_plan_traces.octagon"),
	}
	for _, artifactPath := range octagonArtifacts {
		if _, err := octagon.Load(artifactPath); err != nil {
			t.Fatalf("load fft octagon artifact %s: %v", artifactPath, err)
		}
	}

}
