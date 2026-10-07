package tester

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// `Artifact.WriteJson(path, value)` publishes the JSON text of a typed value.
// The fixture is evaluated as `oct artifact` evaluates it, and each published
// file is compared with its golden file, byte for byte. What the text of a
// value is, is the contract of Language/Builtins/Json/valid; this test is
// about the bytes reaching the output root through the Artifact capability.
func TestArtifactWriteJsonPublishesTheTextOfAValue(t *testing.T) {
	root := filepath.Join("..", "..", "Language", "Builtins", "Json")
	outputRoot := t.TempDir()
	report := &ArtifactReport{}
	var stdout bytes.Buffer
	if err := ExecuteArtifactsWithOptions(filepath.Join(root, "artifact", "json_artifact.octest"), &stdout, ArtifactOptions{OutputRoot: outputRoot, Report: report}); err != nil {
		t.Fatalf("artifact evaluation failed: %v\n%s", err, stdout.String())
	}
	goldens := map[string]string{
		"json/summary.json": "artifact_summary.json",
		"json/cases.json":   "artifact_cases.json",
	}
	if len(report.Artifacts) != len(goldens) {
		t.Fatalf("published %+v, want %d files", report.Artifacts, len(goldens))
	}
	for published, golden := range goldens {
		got, err := os.ReadFile(filepath.Join(outputRoot, filepath.FromSlash(published)))
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(filepath.Join(root, "data", golden))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s:\n got  %q\n want %q", published, got, want)
		}
	}

	// Evaluated again, the same value is the same bytes: nothing is replaced.
	report = &ArtifactReport{}
	if err := ExecuteArtifactsWithOptions(filepath.Join(root, "artifact", "json_artifact.octest"), &stdout, ArtifactOptions{OutputRoot: outputRoot, Report: report}); err != nil {
		t.Fatalf("second evaluation failed: %v\n%s", err, stdout.String())
	}
	for _, artifact := range report.Artifacts {
		if artifact.Status != "unchanged" {
			t.Errorf("%s was %s on the second evaluation, want unchanged", artifact.Path, artifact.Status)
		}
	}
}
