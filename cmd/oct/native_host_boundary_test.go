//go:build integration

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeHostBoundaryConditionSwitchContractsRunAsNativeOctTests(t *testing.T) {
	parallelBoundaryTest(t)
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

func TestNativeHostBoundaryConditionSwitchInvalidFixturesRunAsNativeOctFail(t *testing.T) {
	root := filepath.Join("..", "..", "Language", "ControlFlow", "ConditionSwitch", "invalid")
	stdout, stderr, err := executeCLI("test", root)
	if err != nil {
		t.Fatalf("expected .octfail fixtures to pass, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS condition_switch_non_bool_case.octfail") {
		t.Fatalf("expected condition-switch invalid fixture pass output, got %q", stdout)
	}
}

func TestNativeHostBoundaryBoundaryPolicyDocumentedInTestingDoc(t *testing.T) {
	testingDoc := filepath.Join("..", "..", "docs", "TESTING.md")
	data, err := os.ReadFile(testingDoc)
	if err != nil {
		t.Fatalf("read %s: %v", testingDoc, err)
	}
	text := string(data)

	required := []string{
		"## Semantic Contracts",
		"## Implementation and Integration Tests",
		"## Test Placement Rules",
		"Go-side tests should validate implementation and integration boundaries",
		"These contracts live under `Language/` and are the canonical source for language behavior.",
	}
	for _, needle := range required {
		if !strings.Contains(text, needle) {
			t.Fatalf("docs/TESTING.md is missing boundary section %q", needle)
		}
	}
}
