//go:build integration

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// An .octfail with `expect artifact error:` holds a test source whose
// [Artifact] entry points must fail when evaluated. The fixtures under
// testdata/octfail_artifact are about the test form itself: `accepted` holds
// fixtures the runner must pass, `refused` holds fixtures it must fail.
// Artifact evaluation has one implementation, so every execution mode gives
// the same answer.
func TestArtifactOctFailFixtures(t *testing.T) {
	root := filepath.Join("..", "..", "testdata", "octfail_artifact")
	for _, mode := range []string{"interpreted", "compiled", "auto"} {
		t.Run(mode, func(t *testing.T) {
			stdout, stderr, err := executeCLIArgs("test", filepath.Join(root, "accepted"), "--execution", mode)
			if err != nil {
				t.Fatalf("accepted fixtures failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
			}
			for _, name := range []string{"artifact_fails", "artifact_imports_a_library", "artifact_source_rejected"} {
				if !strings.Contains(stdout, "PASS "+name+".octfail") {
					t.Errorf("missing PASS for %s:\n%s", name, stdout)
				}
			}
			if !strings.Contains(stdout, "Result: 3 passed, 0 failed, 0 skipped") {
				t.Errorf("unexpected result line:\n%s", stdout)
			}

			stdout, _, err = executeCLIArgs("test", filepath.Join(root, "refused"), "--execution", mode)
			if err == nil {
				t.Fatalf("refused fixtures passed:\n%s", stdout)
			}
			for _, want := range []string{
				"FAIL completes.octfail",
				`actual: "artifact evaluation completed"`,
				"FAIL wrong_message.octfail",
				`expected artifact error containing: "the artifact stops here"`,
				`expected artifact error containing: "a text the failure does not contain"`,
				"fatal error: the artifact stops here; 1 artifact(s) failed",
				"Result: 0 passed, 2 failed, 0 skipped",
			} {
				if !strings.Contains(stdout, want) {
					t.Errorf("missing %q in:\n%s", want, stdout)
				}
			}
		})
	}
}
