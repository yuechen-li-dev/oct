//go:build integration

package main

import (
	"strings"
	"testing"
)

func TestJsonIntentRecoveryLabCorpusValidation(t *testing.T) {
	path := "../../Experiments/JsonIntentRecoveryLab/M0/corpus_validation.octest"
	stdout, stderr, err := executeCLI("test", path)
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
