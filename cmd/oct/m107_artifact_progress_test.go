package main

import (
	"bytes"
	"strings"
	"testing"

	"oct/internal/cli"
)

func TestOctArtifactPrintsProgressAndCheckpointEvents(t *testing.T) {
	root := t.TempDir()
	writeOctPkgFile(t, root, "Main", "main.oct", "package Main\nfn Main() -> Int { return 0 }\n")
	writeOctPkgFile(t, root, "Main", "artifact.octest", "package Main\n[Artifact]\nfn Emit() -> Void {\nArtifactCheckpoint(\"start\")\nArtifactProgress(\"work\", 1, 3)\nArtifactProgress(\"work\", 3, 3)\nArtifactCheckpoint(\"done\")\n}\n")

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
