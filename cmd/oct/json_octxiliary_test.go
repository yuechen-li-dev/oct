//go:build toolchain

package main

import (
	"strings"
	"testing"
)

func TestCompiledJsonOctxiliaryWrapper(t *testing.T) {
	requireSlowOctxiliary(t)
	t.Parallel()
	workDir := newWrapperTempProject(t)
	target := repoPath(t, "Libraries", "Json")

	stdout, stderr, err := executeOctWithSidecarsInDir(t, workDir, []string{"test", target, "--execution", "compiled"}, "octxiliary-json", "octxiliary-io")
	if err != nil {
		t.Fatalf("compiled json wrapper tests failed: %v\nstderr:%s\nstdout:%s", err, strings.TrimSpace(stderr), stdout)
	}
	assertNoCompiledFallback(t, stdout, stderr)
	assertCompiledCountAtLeast(t, stdout, 1)
	assertOutputContains(t, stdout,
		"PASS Json.JsonSaveLoadRoundTrip",
		"PASS Json.JsonInvalidSaveFails",
		"PASS Json.JsonCompiledMissingFileFails",
	)
}

func TestCompiledJsonOctxiliaryMissingSidecarMessage(t *testing.T) {
	requireSlowOctxiliary(t)
	workDir := newWrapperTempProject(t)
	target := repoPath(t, "Libraries", "Json", "Json.octest")
	stdout, stderr, err := executeOctWithCustomWrapperPathInDir(t, workDir, t.TempDir(), []string{"test", target, "--execution", "compiled"})
	if err == nil {
		t.Fatalf("expected missing json sidecar failure, got success:\n%s%s", stdout, stderr)
	}
	if !strings.Contains(stdout+stderr, `Octxiliary sidecar "octxiliary-json" not found`) {
		t.Fatalf("expected clear missing json sidecar message, got:\nstdout:%s\nstderr:%s", stdout, stderr)
	}
}
