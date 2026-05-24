package main

import (
	"strings"
	"testing"
)

func TestIOJsonGoldenWrapper(t *testing.T) {
	t.Parallel()
	root := "../../Libraries/IO"
	stdout, stderr, err := executeCLI("test", root)
	if err != nil {
		t.Fatalf("oct test failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	if !strings.Contains(stdout, "PASS IO.JsonNormalizeMinifiesValidDocument") {
		t.Fatalf("expected json normalize happy-path fact to pass, got %q", stdout)
	}
	if !strings.Contains(stdout, "PASS IO.JsonNormalizeRejectsInvalidDocument") {
		t.Fatalf("expected json normalize error-path fact to pass, got %q", stdout)
	}
}
