package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestM23eDecisionLadderContractsRunAsNativeOctTests(t *testing.T) {
	validRoot := filepath.Join("..", "..", "Language", "ControlFlow", "DecisionLadder", "valid")
	validStdout, validStderr, err := executeCLI("test", validRoot)
	if err != nil {
		t.Fatalf("expected valid decision-ladder fixtures to pass, err=%v stderr=%q stdout=%q", err, validStderr, validStdout)
	}

	requiredValidPasses := []string{
		"PASS DecisionLadderContracts.NestedThenIfAllowed",
		"PASS DecisionLadderContracts.NestedControlInWhileAllowed",
		"PASS DecisionLadderContracts.ConditionSwitchReplacementIsValid",
	}
	for _, pass := range requiredValidPasses {
		if !strings.Contains(validStdout, pass) {
			t.Fatalf("expected stdout to contain %q, got %q", pass, validStdout)
		}
	}

	invalidRoot := filepath.Join("..", "..", "Language", "ControlFlow", "DecisionLadder", "invalid")
	invalidStdout, invalidStderr, err := executeCLI("test", invalidRoot)
	if err != nil {
		t.Fatalf("expected invalid decision-ladder .octfail fixtures to pass, err=%v stderr=%q stdout=%q", err, invalidStderr, invalidStdout)
	}
	if !strings.Contains(invalidStdout, "PASS decision_ladder_rejected.octfail") {
		t.Fatalf("expected invalid fixture pass output, got %q", invalidStdout)
	}
}

func TestM23eProofSignalUsesConditionSwitch(t *testing.T) {
	signalPath := filepath.Join("..", "..", "Packages", "Signal", "signal.oct")
	bytes, err := os.ReadFile(signalPath)
	if err != nil {
		t.Fatalf("read signal proof package: %v", err)
	}
	src := string(bytes)
	if !strings.Contains(src, "let shape = switch {") {
		t.Fatalf("expected signal proof package to use condition-switch shape selection")
	}
	if strings.Contains(src, "} else {\n            if") {
		t.Fatalf("expected signal proof package to avoid nested decision-ladder if/else")
	}
}
