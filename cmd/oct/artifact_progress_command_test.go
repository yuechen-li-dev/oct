//go:build integration

package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/cli"
)

func TestOctArtifactPrintsProgressAndCheckpointEvents(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Artifact", "artifact.oct", "package Artifact\nfn Checkpoint(label: String) -> Void { ArtifactCheckpoint(label) }\nfn Progress(label: String, current: Int, total: Int) -> Void { ArtifactProgress(label, current, total) }\n")
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "artifact.octest", "package Main\nimport Artifact\n[Artifact]\nfn Emit() -> Void {\nArtifact.Checkpoint(\"start\")\nArtifact.Progress(\"work\", 1, 3)\nArtifact.Progress(\"work\", 3, 3)\nArtifact.Checkpoint(\"done\")\n}\n")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if err := cli.Execute([]string{"artifact", root}, &stdout, &stderr); err != nil {
		t.Fatalf("artifact failed: err=%v stderr=%q stdout=%q", err, stderr.String(), stdout.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"CHECKPOINT Main.Emit: start",
		"PROGRESS Main.Emit: work 1/3",
		"PROGRESS Main.Emit: work 3/3",
		"CHECKPOINT Main.Emit: done",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got %q", want, out)
		}
	}
}
