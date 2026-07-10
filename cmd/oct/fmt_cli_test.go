package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/cli"
)

func writeSourceFileAtPath(t *testing.T, path string, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestFmtHelpShowsCanonicalModes(t *testing.T) {
	t.Parallel()
	var out, errOut bytes.Buffer
	if err := cli.Execute([]string{"fmt", "--help"}, &out, &errOut); err != nil {
		t.Fatalf("help failed: %v", err)
	}
	help := out.String()
	if !strings.Contains(help, "en-llm") || !strings.Contains(help, "en-llm-compact") {
		t.Fatalf("missing canonical modes in help: %s", help)
	}
	if strings.Contains(help, "readable|compact") {
		t.Fatalf("legacy modes should not be advertised: %s", help)
	}
}

func TestFmtModeVariantsAndAliases(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := writeSourceFileAtPath(t, filepath.Join(root, "sample.oct"), "package Main\nfn main()->Int{return 1}\n")
	for _, mode := range []string{"en-llm", "en-llm-compact", "readable", "compact"} {
		var out, errOut bytes.Buffer
		err := cli.Execute([]string{"fmt", path, "--mode", mode, "--check"}, &out, &errOut)
		if err == nil {
			continue
		}
		if strings.Contains(errOut.String(), "is not formatted") {
			var out2, errOut2 bytes.Buffer
			if err2 := cli.Execute([]string{"fmt", path, "--mode", mode}, &out2, &errOut2); err2 != nil {
				t.Fatalf("format mode %s failed: %v stderr=%q", mode, err2, errOut2.String())
			}
			continue
		}
		t.Fatalf("unexpected failure for mode %s: %v stderr=%q", mode, err, errOut.String())
	}
}

func TestFmtInvalidModeDiagnostic(t *testing.T) {
	t.Parallel()
	path := writeSourceFileAtPath(t, filepath.Join(t.TempDir(), "fmt_invalid_mode.oct"), "package Main\nfn main()->Int{return 1}\n")
	var out, errOut bytes.Buffer
	err := cli.Execute([]string{"fmt", path, "--mode", "invalid"}, &out, &errOut)
	if err == nil {
		t.Fatalf("expected invalid mode error")
	}
	if !strings.Contains(errOut.String(), "invalid --mode \"invalid\"; expected en-llm|en-llm-compact") {
		t.Fatalf("unexpected stderr: %q", errOut.String())
	}
}
