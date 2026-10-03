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

// The CLI leaves arrows as written unless --arrows asks for one spelling.
func TestFmtArrowsSetting(t *testing.T) {
	t.Parallel()
	source := "package Main\nfn main() => Int { return switch 1 { case 1 => 2 else -> 3 } }\n"
	cases := []struct {
		args []string
		want string
	}{
		{nil, source},
		{[]string{"--arrows", "keep"}, source},
		{[]string{"--arrows", "thin"}, strings.ReplaceAll(source, "=>", "->")},
		{[]string{"--arrows", "fat"}, strings.ReplaceAll(source, "->", "=>")},
	}
	for _, c := range cases {
		path := writeSourceFileAtPath(t, filepath.Join(t.TempDir(), "arrows.oct"), source)
		var out, errOut bytes.Buffer
		if err := cli.Execute(append([]string{"fmt", path}, c.args...), &out, &errOut); err != nil {
			t.Fatalf("fmt %v failed: %v stderr=%q", c.args, err, errOut.String())
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != c.want {
			t.Errorf("fmt %v wrote %q, want %q", c.args, got, c.want)
		}
	}

	path := writeSourceFileAtPath(t, filepath.Join(t.TempDir(), "arrows.oct"), source)
	var out, errOut bytes.Buffer
	if err := cli.Execute([]string{"fmt", path, "--arrows", "double"}, &out, &errOut); err == nil || !strings.Contains(errOut.String(), "expected keep|thin|fat") {
		t.Fatalf("expected an invalid --arrows diagnostic, got %v stderr=%q", err, errOut.String())
	}
	if err := cli.Execute([]string{"fmt", path, "--arrows"}, &out, &errOut); err == nil {
		t.Fatalf("expected --arrows without a value to fail")
	}
	var help, helpErr bytes.Buffer
	if err := cli.Execute([]string{"fmt", "--help"}, &help, &helpErr); err != nil || !strings.Contains(help.String(), "--arrows keep|thin|fat") {
		t.Fatalf("help does not describe --arrows: %v %q", err, help.String())
	}
}
