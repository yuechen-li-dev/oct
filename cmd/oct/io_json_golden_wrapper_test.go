package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestIOJsonGoldenWrapper(t *testing.T) {
	requireSlowOctxiliary(t)
	t.Parallel()
	root := filepath.Join("..", "..", "Libraries", "IO", "IO.Json.octest")
	stdout, stderr, err := executeCLIWithSidecars(t, "test", root, "octxiliary-io", "octxiliary-json")
	if err != nil {
		t.Fatalf("oct test failed: %v stderr=%s stdout=%s", err, stderr, stdout)
	}
	assertNoMissingSidecarFallback(t, stdout, stderr)
	if !strings.Contains(stdout, "PASS IO.JsonNormalizeMinifiesValidDocument") {
		t.Fatalf("expected json normalize happy-path fact to pass, got %q", stdout)
	}
	if !strings.Contains(stdout, "PASS IO.JsonNormalizeRejectsInvalidDocument") {
		t.Fatalf("expected json normalize error-path fact to pass, got %q", stdout)
	}
}
