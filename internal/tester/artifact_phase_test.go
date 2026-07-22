package tester

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yuechen-li-dev/oct/internal/interpret"
	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

func artifactLanguageFixture(parts ...string) string {
	all := append([]string{"..", "..", "Language", "Tooling", "Artifacts"}, parts...)
	return filepath.Join(all...)
}

func TestBuildTimeArtifactEvaluationPublishesTypedOutputsWithoutBackend(t *testing.T) {
	outputRoot := t.TempDir()
	target := artifactLanguageFixture("valid", "build_time_artifact_evaluation.octest")
	preexisting := filepath.Join(outputRoot, "nested", "model.txt")
	if err := os.MkdirAll(filepath.Dir(preexisting), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(preexisting, []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	report := &ArtifactReport{}
	var stdout bytes.Buffer
	if err := ExecuteArtifactsWithOptions(target, &stdout, ArtifactOptions{OutputRoot: outputRoot, Report: report}); err != nil {
		t.Fatalf("artifact evaluation failed: %v\n%s", err, stdout.String())
	}
	if report.Execution != "build-time-interpreted" || report.RequestedExecution != "interpreted" {
		t.Fatalf("unexpected execution report: %+v", report)
	}
	if len(report.Artifacts) != 6 {
		t.Fatalf("expected six typed outputs, got %+v", report.Artifacts)
	}
	for _, artifact := range report.Artifacts {
		if artifact.Status != "produced" {
			t.Fatalf("first publication status for %s = %s", artifact.Path, artifact.Status)
		}
		if _, err := os.Stat(filepath.Join(outputRoot, filepath.FromSlash(artifact.Path))); err != nil {
			t.Fatalf("missing published artifact %s: %v", artifact.Path, err)
		}
	}

	modelPath := filepath.Join(outputRoot, "nested", "model.txt")
	firstInfo, err := os.Stat(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	firstModTime := firstInfo.ModTime()
	time.Sleep(20 * time.Millisecond)
	report = &ArtifactReport{}
	stdout.Reset()
	if err := ExecuteArtifactsWithOptions(target, &stdout, ArtifactOptions{Execution: "compiled", OutputRoot: outputRoot, Report: report}); err != nil {
		t.Fatalf("compiled compatibility delegation failed: %v\n%s", err, stdout.String())
	}
	if !strings.Contains(stdout.String(), "no backend is generated or compiled") {
		t.Fatalf("missing explicit compatibility delegation: %s", stdout.String())
	}
	for _, artifact := range report.Artifacts {
		if artifact.Status != "unchanged" {
			t.Fatalf("second publication status for %s = %s", artifact.Path, artifact.Status)
		}
	}
	secondInfo, err := os.Stat(modelPath)
	if err != nil {
		t.Fatal(err)
	}
	if !secondInfo.ModTime().Equal(firstModTime) {
		t.Fatalf("unchanged artifact modification time changed: %v -> %v", firstModTime, secondInfo.ModTime())
	}
}

func TestArtifactCapabilityRejectsUnsafePathsDuplicatesEffectsAndFailures(t *testing.T) {
	cases := []struct {
		file string
		want string
	}{
		{"path_traversal.octest", "escapes the artifact output root"},
		{"absolute_path.octest", "must be non-empty and relative"},
		{"duplicate_output.octest", "duplicate artifact output path"},
		{"ambient_write.octest", "outside Artifact.Write*"},
		{"fallible_failure.octest", "artifact failure is visible"},
	}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			var stdout bytes.Buffer
			outputRoot := t.TempDir()
			err := ExecuteArtifactsWithOptions(artifactLanguageFixture("invalid", tc.file), &stdout, ArtifactOptions{OutputRoot: outputRoot})
			if err == nil || !strings.Contains(stdout.String()+err.Error(), tc.want) {
				t.Fatalf("expected %q, got err=%v output=%s", tc.want, err, stdout.String())
			}
			if tc.file == "fallible_failure.octest" {
				if _, statErr := os.Stat(filepath.Join(outputRoot, "must-not-publish.txt")); !os.IsNotExist(statErr) {
					t.Fatalf("failed artifact evaluation published staged output: %v", statErr)
				}
			}
		})
	}
}

func TestArtifactEntryPointAndCapabilityCannotBeUsedAsOrdinaryRuntime(t *testing.T) {
	program, err := project.LoadForTest(artifactLanguageFixture("invalid", "direct_entry_call.octest"))
	if err != nil {
		t.Fatal(err)
	}
	if err := typecheck.CheckProgram(program); err == nil || !strings.Contains(err.Error(), "directly calls artifact entry point") {
		t.Fatalf("expected direct artifact entry diagnostic, got %v", err)
	}

	program, err = project.Load(artifactLanguageFixture("invalid", "artifact_write_outside_phase.oct"))
	if err != nil {
		t.Fatal(err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatal(err)
	}
	if _, err := interpret.ExecuteMain(program, nil); err == nil || !strings.Contains(err.Error(), "only during `oct artifact` evaluation") {
		t.Fatalf("expected phase capability diagnostic, got %v", err)
	}
}

func TestArtifactDiscoveryOrderIsDeterministic(t *testing.T) {
	target := artifactLanguageFixture("invalid", "duplicate_output.octest")
	program, err := project.LoadForTest(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatal(err)
	}
	artifacts, err := discoverArtifactCases(program, target, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 2 || artifacts[0].name != "FirstWriter" || artifacts[1].name != "SecondWriter" {
		t.Fatalf("unexpected artifact discovery order: %+v", artifacts)
	}
}

func TestArtifactPublisherRejectsPortableAbsoluteAndTraversalSpellings(t *testing.T) {
	publisher, err := newArtifactPublisher(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer publisher.close()
	for _, path := range []string{"/absolute.txt", `C:\absolute.txt`, `..\escape.txt`} {
		if _, err := publisher.StageArtifactOutput(interpret.ArtifactOutputRequest{Path: path, Function: "Probe", SourcePath: "probe.octest"}); err == nil {
			t.Fatalf("expected portable path rejection for %q", path)
		}
	}
}
