//go:build integration

package main

import (
	"strings"
	"testing"
)

func TestJsonIntentRecoveryLabCorpusValidation(t *testing.T) {
	// The experiment names its corpus files relative to the repository root,
	// as every fixture does, so it is run as a process started there.
	stdout, stderr, err := runOctInRepository(t, repoPath(t), "test", "Experiments/JsonIntentRecoveryLab/M0")
	if err != nil {
		t.Fatalf("oct test failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}

	expectedPasses := []string{
		"PASS JsonIntentRecoveryLabM0.CorpusFilesLoadAsJsonAndNormalizeDeterministically",
		"PASS JsonIntentRecoveryLabM0.CorpusRepresentativeRootKindsRemainStable",
	}

	for _, marker := range expectedPasses {
		if !strings.Contains(stdout, marker) {
			t.Fatalf("expected marker %q in stdout, got %q", marker, stdout)
		}
	}
}
