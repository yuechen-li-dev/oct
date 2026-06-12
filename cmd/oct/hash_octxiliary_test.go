package main

import (
	"strings"
	"testing"
)

func TestCompiledHashOctxiliaryWrapper(t *testing.T) {
	requireSlowOctxiliary(t)
	t.Parallel()
	workDir := newWrapperTempProject(t)
	target := repoPath(t, "Libraries", "Hash")

	stdout, stderr, err := executeOctWithSidecarsInDir(t, workDir, []string{"test", target, "--execution", "compiled"}, "octxiliary-hash", "octxiliary-io")
	if err != nil {
		t.Fatalf("compiled hash wrapper tests failed: %v\nstderr:%s\nstdout:%s", err, strings.TrimSpace(stderr), stdout)
	}
	assertNoCompiledFallback(t, stdout, stderr)
	assertCompiledCountAtLeast(t, stdout, 1)
	assertOutputContains(t, stdout,
		"PASS Hash.Sha256TextKnownValueChecks",
		"PASS Hash.Sha256BytesKnownValueChecks",
		"PASS Hash.Sha256FileKnownValueChecks",
		"PASS Hash.Sha256FileMissingFails",
	)
}

func TestCompiledHashOctxiliaryMissingSidecarMessage(t *testing.T) {
	requireSlowOctxiliary(t)
	workDir := newWrapperTempProject(t)
	target := repoPath(t, "Libraries", "Hash", "Hash.CompiledSmoke.octest")
	stdout, stderr, err := executeOctWithCustomWrapperPathInDir(t, workDir, t.TempDir(), []string{"test", target, "--execution", "compiled"})
	if err == nil {
		t.Fatalf("expected missing hash sidecar failure, got success:\n%s%s", stdout, stderr)
	}
	if !strings.Contains(stdout+stderr, `Octxiliary sidecar "octxiliary-hash" not found`) {
		t.Fatalf("expected clear missing hash sidecar message, got:\nstdout:%s\nstderr:%s", stdout, stderr)
	}
}
