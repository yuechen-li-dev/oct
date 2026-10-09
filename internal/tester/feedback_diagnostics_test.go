package tester

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAssertionDiagnosticReportsValuesInBothLanes(t *testing.T) {
	path := filepath.Join("..", "..", "Language", "Testing", "FeedbackStabilization", "diagnostics", "assertion_values.octest")
	for _, lane := range []string{"interpreted", "compiled"} {
		t.Run(lane, func(t *testing.T) {
			var output bytes.Buffer
			err := ExecuteWithOptions(path, &output, TestOptions{Execution: lane})
			if err == nil || !strings.Contains(output.String(), "expected 3, actual 4") {
				t.Fatalf("missing comparison diagnostic: %v\n%s", err, output.String())
			}
		})
	}
}

func TestEmptyDirectoryReportsMissingTests(t *testing.T) {
	var output bytes.Buffer
	err := Execute(t.TempDir(), &output)
	if err == nil || !strings.Contains(err.Error(), "no .octest or .octfail tests found") {
		t.Fatalf("unexpected diagnostic: %v", err)
	}
}

func TestStagedCsvArtifactReadback(t *testing.T) {
	path := filepath.Join("..", "..", "Language", "Testing", "FeedbackStabilization", "artifact", "csv.octest")
	var output bytes.Buffer
	if err := ExecuteArtifactsWithOptions(path, &output, ArtifactOptions{OutputRoot: t.TempDir()}); err != nil {
		t.Fatalf("staged CSV readback failed: %v\n%s", err, output.String())
	}
}

func TestCodedRuntimeDiagnosticHasNoGoStack(t *testing.T) {
	path := filepath.Join("..", "..", "Language", "Testing", "FeedbackStabilization", "invalid", "coded_runtime.octfail")
	fixtures, err := discoverOctFailCases(path)
	if err != nil || len(fixtures) != 1 {
		t.Fatalf("discover: %v", err)
	}
	source := filepath.Join(t.TempDir(), "main.oct")
	if err := os.WriteFile(source, []byte(fixtures[0].source), 0o644); err != nil {
		t.Fatal(err)
	}
	message, failed, err := runOctFailCompiled(source, filepath.Dir(path))
	if err != nil || !failed || !strings.Contains(message, "runtime error [OCT-RTBL003]") || strings.Contains(message, "goroutine") || strings.Contains(message, "panic:") {
		t.Fatalf("unexpected diagnostic: %v, failed=%v\n%s", err, failed, message)
	}
}
