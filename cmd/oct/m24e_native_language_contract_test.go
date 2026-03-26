package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestM24eIfExpressionContractsRunAsNativeOctTests(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "m24e", "valid")
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

func TestM24eIfExpressionInvalidFixtureRejectedByBuild(t *testing.T) {
	entry := filepath.Join("..", "..", "testdata", "m24e", "invalid", "Main", "main.oct")
	stdout, stderr, err := executeCLI("build", entry)
	if err == nil {
		t.Fatalf("expected build failure, got success with stdout %q", stdout)
	}
	if !strings.Contains(stderr, "if expression branches must have matching types") {
		t.Fatalf("expected if-expression type mismatch error, got %q", stderr)
	}
}
