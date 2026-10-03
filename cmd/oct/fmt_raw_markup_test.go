package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The value of a raw Oct-XML body depends on the white space inside it, and
// the formatter rewrites white space. This boundary check runs a fixture whose
// facts state those values exactly, formats the fixture, and runs it again.
// The fixture is badly indented on purpose, so the formatter has real work to
// do on every element.
func TestFmtKeepsRawMarkupValues(t *testing.T) {
	original, err := os.ReadFile(repoPath(t, "testdata", "ocfmt_raw_markup", "raw_markup.octest"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "raw_markup.octest")
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	runFacts := func(when string) {
		t.Helper()
		stdout, stderr, err := executeCLIArgs("test", dir, "--execution", "interpreted")
		if err != nil {
			t.Fatalf("%s formatting, the facts fail: %v\nstdout:\n%s\nstderr:\n%s", when, err, stdout, stderr)
		}
	}

	runFacts("before")
	if stdout, stderr, err := executeCLIArgs("fmt", path); err != nil {
		t.Fatalf("fmt: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	formatted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(formatted) == string(original) {
		t.Fatal("the formatter left the badly indented fixture unchanged, so this test checks nothing")
	}
	runFacts("after")

	if stdout, stderr, err := executeCLIArgs("fmt", path, "--check"); err != nil {
		t.Fatalf("the formatted fixture does not pass --check: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
}
