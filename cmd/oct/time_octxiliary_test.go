package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompiledTimeOctxiliaryWrapper(t *testing.T) {
	repo := filepath.Join("..", "..")
	binDir := sharedTestSidecarDir(t, "octxiliary-time")

	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/Time", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled time wrapper tests failed: %v\n%s", err, strings.TrimSpace(string(out)))
	}
	assertOutputContains(t, string(out),
		"PASS Time.TimeParseFormatIso8601Sanity",
		"PASS Time.TimeDeterministicIso8601Fixtures",
		"PASS Time.TimeUnixSecondsFormattingSanity",
		"PASS Time.TimeInvalidIso8601Fails",
	)
}

func TestCompiledTimeOctxiliaryMissingSidecarMessage(t *testing.T) {
	repo := filepath.Join("..", "..")
	cmd := exec.Command("go", "run", "./cmd/oct", "test", "Libraries/Time", "--execution", "compiled")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected missing time sidecar failure, got success:\n%s", string(out))
	}
	if !strings.Contains(string(out), `Octxiliary sidecar "octxiliary-time" not found`) {
		t.Fatalf("expected clear missing time sidecar message, got:\n%s", string(out))
	}
}
