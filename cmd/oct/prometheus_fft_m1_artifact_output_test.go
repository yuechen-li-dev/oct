//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/octagon"
)

func TestPrometheusFftAlgorithmLabM1ArtifactWritesDeterministicVisibleOutputs(t *testing.T) {
	workDir := t.TempDir()
	outDir := filepath.Join(workDir, "out", "prometheus_fft_algorithm_lab", "m1")
	project := repoPath(t, "Experiments", "PrometheusFftAlgorithmLab", "M1")
	stdout, stderr, err := runOctWithWrapperPath(t, workDir, "", "artifact", project, "--output-root", workDir)
	if err != nil {
		t.Fatalf("artifact command failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if !strings.Contains(strings.ToLower(stdout), "checkpoint") {
		t.Fatalf("expected artifact output to include checkpoint logs, got:\n%s", stdout)
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
