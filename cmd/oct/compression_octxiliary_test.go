package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCompiledCompressionOctxiliaryWrapper(t *testing.T) {
	requireSlowOctxiliary(t)
	t.Parallel()
	workDir := newWrapperTempProject(t)
	copyOctxiliaryFixture(t, "mx103c_gzip_src.txt", filepath.Join(workDir, "mx103c_gzip_src.txt"))
	copyOctxiliaryFixture(t, "m8_gzip_text_bytes_src.txt", filepath.Join(workDir, "m8_gzip_text_bytes_src.txt"))
	copyOctxiliaryFixture(t, "mx103c_gzip_file_src.txt", filepath.Join(workDir, "mx103c_gzip_file_src.txt"))
	target := repoPath(t, "Libraries", "Compression")

	stdout, stderr, err := executeOctWithSidecarsInDir(t, workDir, []string{"test", target, "--execution", "compiled"}, "octxiliary-compression", "octxiliary-io")
	if err != nil {
		t.Fatalf("compiled compression wrapper tests failed: %v\nstderr:%s\nstdout:%s", err, strings.TrimSpace(stderr), stdout)
	}
	assertNoCompiledFallback(t, stdout, stderr)
	assertCompiledCountAtLeast(t, stdout, 1)
	assertOutputContains(t, stdout,
		"PASS Compression.GzipCompressAndDecompressBytesRoundTrip",
		"PASS Compression.GzipCompressAndDecompressTextBytesRoundTrip",
		"PASS Compression.GzipCompressAndDecompressFileRoundTrip",
		"PASS Compression.GzipDecompressRejectsInvalidPayload",
		"PASS Compression.GzipCompressMissingFileFails",
	)
}

func TestCompiledCompressionOctxiliaryMissingSidecarMessage(t *testing.T) {
	requireSlowOctxiliary(t)
	binDir := t.TempDir()
	buildTestSidecarsInDir(t, binDir, "octxiliary-io")

	workDir := newWrapperTempProject(t)
	target := repoPath(t, "Libraries", "Compression")
	stdout, stderr, err := executeOctWithCustomWrapperPathInDir(t, workDir, binDir, []string{"test", target, "--execution", "compiled"})
	if err == nil {
		t.Fatalf("expected missing compression sidecar failure, got success:\n%s%s", stdout, stderr)
	}
	if !strings.Contains(stdout+stderr, `Octxiliary sidecar "octxiliary-compression" not found`) {
		t.Fatalf("expected clear missing compression sidecar message, got:\nstdout:%s\nstderr:%s", stdout, stderr)
	}
}
