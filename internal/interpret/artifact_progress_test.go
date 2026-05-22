package interpret

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"oct/internal/project"
	"oct/internal/typecheck"
)

func TestArtifactProgressRecorderReceivesOrderedEvents(t *testing.T) {
	root := t.TempDir()
	source := "package Main\n[Artifact]\nfn Emit() -> Void {\nArtifactCheckpoint(\"start\")\nArtifactProgress(\"work\", 1, 3)\nArtifactProgress(\"work\", 3, 3)\nArtifactCheckpoint(\"done\")\n}\n"
	if err := os.WriteFile(filepath.Join(root, "artifact.octest"), []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	program, err := project.LoadForTest(root)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatalf("typecheck: %v", err)
	}
	var events []ArtifactProgressEvent
	if err := ExecuteFunctionWithArgsAndOptions(program, "Main", "Emit", nil, &bytes.Buffer{}, ExecuteOptions{
		ArtifactProgressRecorder: func(event ArtifactProgressEvent) {
			events = append(events, event)
		},
	}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}
	if events[0].Kind != "checkpoint" || events[0].Label != "start" {
		t.Fatalf("unexpected first event: %#v", events[0])
	}
	if events[1].Kind != "progress" || events[1].Current != 1 || events[1].Total != 3 {
		t.Fatalf("unexpected second event: %#v", events[1])
	}
	if events[3].Kind != "checkpoint" || events[3].Label != "done" {
		t.Fatalf("unexpected fourth event: %#v", events[3])
	}
}
