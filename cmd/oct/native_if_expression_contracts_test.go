//go:build integration

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNativeIfExpressionIfExpressionContractsRunAsNativeOctTests(t *testing.T) {
	root := filepath.Join("..", "..", "Language", "ControlFlow", "IfExpression", "valid")
	stdout, stderr, err := executeCLI("test", root)
	if err != nil {
		t.Fatalf("oct test failed: %v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS IfContracts.IfExpressionSelectsThenArm") {
		t.Fatalf("expected migrated fact output, got %q", stdout)
	}
	if !strings.Contains(stdout, "PASS IfContracts.IfExpressionValueSelectionBucketsBySign[2]") {
		t.Fatalf("expected migrated theory output, got %q", stdout)
	}
}

func TestNativeIfExpressionIfExpressionInvalidFixtureRunsAsNativeOctFail(t *testing.T) {
	root := filepath.Join("..", "..", "Language", "ControlFlow", "IfExpression", "invalid")
	stdout, stderr, err := executeCLI("test", root)
	if err != nil {
		t.Fatalf("expected .octfail fixtures to pass, err=%v stderr=%q stdout=%q", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS if_expression_branch_type_mismatch.octfail") {
		t.Fatalf("expected if-expression invalid fixture pass output, got %q", stdout)
	}
}
