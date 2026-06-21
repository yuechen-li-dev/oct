package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMakeHostOctxiliaryRequiresAuthority(t *testing.T) {
	requireSlowOctxiliary(t)
	binDir := sharedTestSidecarDir(t, "octxiliary-makehost")
	repo := filepath.Join("..", "..")
	cmd := exec.Command(sharedTestOctBinary(t), "test", "Libraries/Make", "--execution", "interpreted")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "OCT_WRAPPER_PATH="+binDir)
	cmd.Env = appendWithoutKey(cmd.Env, "OCT_MAKE_AUTHORITY")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected make host authority failure, got success: %s", out)
	}
	if !strings.Contains(string(out), "Make host capabilities are only available under oct make") {
		t.Fatalf("expected make authority diagnostic, got: %s", out)
	}
}

func appendWithoutKey(env []string, key string) []string {
	prefix := key + "="
	out := env[:0]
	for _, item := range env {
		if !strings.HasPrefix(item, prefix) {
			out = append(out, item)
		}
	}
	return out
}
