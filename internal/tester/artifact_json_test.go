package tester

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/interpret"
	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
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

// The interpreted lane stops an ordinary program when it reaches
// Artifact.WriteJson with a value, as it does for Artifact.WriteText. The
// compiled lane refuses to build the same program, which is the contract in
// Language/Builtins/Json/invalid/artifact_write_outside_the_phase.octfail.
func TestArtifactWriteJsonOutsideThePhaseStopsAnInterpretedProgram(t *testing.T) {
	program, err := project.Load(filepath.Join("..", "..", "testdata", "artifact_phase_json", "write_json_outside_phase.oct"))
	if err != nil {
		t.Fatal(err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatal(err)
	}
	if _, err := interpret.ExecuteMain(program, nil); err == nil || !strings.Contains(err.Error(), "only during `oct artifact` evaluation") {
		t.Fatalf("expected phase capability diagnostic, got %v", err)
	}
	if _, err := os.Stat("outside.json"); err == nil {
		t.Errorf("outside.json was written")
		os.Remove("outside.json")
	}
}

// Until the first Json library is removed, `Artifact.WriteJson(path, text)`
// given a String publishes that JSON text, compact, and not a JSON string.
// This test goes when that library does.
func TestArtifactWriteJsonGivenAStringIsStillTheFirstLibrarys(t *testing.T) {
	outputRoot := t.TempDir()
	var stdout bytes.Buffer
	if err := ExecuteArtifactsWithOptions(artifactLanguageFixture("valid", "build_time_artifact_evaluation.octest"), &stdout, ArtifactOptions{OutputRoot: outputRoot}); err != nil {
		t.Fatalf("artifact evaluation failed: %v\n%s", err, stdout.String())
	}
	got, err := os.ReadFile(filepath.Join(outputRoot, "nested", "model.json"))
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"name":"typed-artifact","count":3}`; string(got) != want {
		t.Errorf("published %q, want %q", got, want)
	}
}
