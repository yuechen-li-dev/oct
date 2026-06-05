package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompiledCompressionOctxiliaryWrapper(t *testing.T) {
	repo := filepath.Join("..", "..")
	binDir := sharedTestSidecarDir(t, "octxiliary-compression", "octxiliary-io")

	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/Compression", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled compression wrapper tests failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	assertOutputContains(t, string(out),
		"PASS Compression.GzipCompressAndDecompressBytesRoundTrip",
		"PASS Compression.GzipCompressAndDecompressTextBytesRoundTrip",
		"PASS Compression.GzipCompressAndDecompressFileRoundTrip",
		"PASS Compression.GzipDecompressRejectsInvalidPayload",
		"PASS Compression.GzipCompressMissingFileFails",
	)
}

func TestCompiledCompressionOctxiliaryMissingSidecarMessage(t *testing.T) {
	repo := filepath.Join("..", "..")
	binDir := t.TempDir()
	buildTestSidecarsInDir(t, binDir, "octxiliary-io")

	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/Compression", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing compression sidecar failure, got success:\n%s", string(out))
	}
	if !strings.Contains(string(out), `Octxiliary sidecar "octxiliary-compression" not found`) {
		t.Fatalf("expected clear missing compression sidecar message, got:\n%s", string(out))
	}
}
