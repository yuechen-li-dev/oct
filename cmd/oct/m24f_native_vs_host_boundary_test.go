package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestM24fConditionSwitchContractsRunAsNativeOctTests(t *testing.T) {
	root := filepath.Join("..", "..", "Language", "ControlFlow", "ConditionSwitch", "valid")
	stdout, stderr, err := executeCLI("test", root)
	if err != nil {
		t.Fatalf("oct test failed: %v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS ConditionSwitch.ConditionSwitchSelectsFirstMatchingCase") {
		t.Fatalf("expected migrated fact output, got %q", stdout)
	}
	if !strings.Contains(stdout, "PASS ConditionSwitch.ConditionSwitchStopsEvaluatingLaterConditionsAfterMatch") {
		t.Fatalf("expected migrated laziness output, got %q", stdout)
	}
}

func TestM24fConditionSwitchInvalidFixturesRunAsNativeOctFail(t *testing.T) {
	root := filepath.Join("..", "..", "Language", "ControlFlow", "ConditionSwitch", "invalid")
	stdout, stderr, err := executeCLI("test", root)
	if err != nil {
		t.Fatalf("expected .octfail fixtures to pass, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS condition_switch_non_bool_case.octfail") {
		t.Fatalf("expected condition-switch invalid fixture pass output, got %q", stdout)
	}
}

func TestM24fBoundaryPolicyDocumentedInReadme(t *testing.T) {
	readme := filepath.Join("..", "..", "README.md")
	data, err := os.ReadFile(readme)
	if err != nil {
		t.Fatalf("read %s: %v", readme, err)
	}
	text := string(data)

	required := []string{
		"## M24f native-vs-host test boundary formalization",
		"### What belongs in `.octest`",
		"### What remains in Go tests",
		"### Decision rule for new tests",
		"### Demo vs test separation",
	}
	for _, needle := range required {
		if !strings.Contains(text, needle) {
			t.Fatalf("README is missing boundary section %q", needle)
		}
	}
}
