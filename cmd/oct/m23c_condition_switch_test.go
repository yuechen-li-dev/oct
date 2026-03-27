package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestM23cConditionSwitchInvalidCasesStayHostOwned(t *testing.T) {
	root := filepath.Join("..", "..", "Language", "ControlFlow", "ConditionSwitch", "invalid")
	stdout, stderr, err := executeCLI("test", root)
	if err != nil {
		t.Fatalf("expected .octfail fixtures to pass, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	requiredPasses := []string{
		"PASS condition_switch_missing_else.octfail",
		"PASS condition_switch_non_bool_case.octfail",
		"PASS condition_switch_result_type_mismatch.octfail",
		"PASS condition_switch_dimension_mismatch.octfail",
	}
	for _, pass := range requiredPasses {
		if !strings.Contains(stdout, pass) {
			t.Fatalf("expected stdout to contain %q, got %q", pass, stdout)
		}
	}
}
