package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runJSON runs `oct json ...` in a directory that holds the documents given.
func runJSON(t *testing.T, documents map[string]string, args ...string) (stdout string, stderr string, err error) {
	t.Helper()
	directory := t.TempDir()
	for name, text := range documents {
		if writeErr := os.WriteFile(filepath.Join(directory, name), []byte(text), 0o644); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	var out, problems bytes.Buffer
	err = ExecuteWithContext(append([]string{"json"}, args...), ExecutionContext{WorkingDir: directory, Stdout: &out, Stderr: &problems})
	return out.String(), problems.String(), err
}

// What is inferred, and why, is the business of internal/jsoninfer and its
// tests. These are about the command: its arguments, what goes to which
// stream, and when it fails.
func TestJSONInfer(t *testing.T) {
	documents := map[string]string{
		"tickets.json": `[{"id": "T-1", "assignee": "sam"}, {"id": "T-2", "assignee": null}]`,
		"tagged.json":  `[{"kind": "circle", "radius": 1}, {"kind": "square", "side": 2}]`,
		"broken.json":  "[1,\n 2,]",
	}

	stdout, stderr, err := runJSON(t, documents, "infer", "tickets.json")
	want := "record table Tickets {\n    Id: String\n    Assignee: Option<String>\n}\n\n// Json.Load<Tickets>(\"tickets.json\")?\n"
	if err != nil || stderr != "" || stdout != want {
		t.Errorf("infer tickets.json: %v\nstderr %q\nstdout:\n%s", err, stderr, stdout)
	}

	// The options may come before or after the file.
	stdout, _, err = runJSON(t, documents, "infer", "--name", "Ticket", "tickets.json", "--explain")
	if err != nil || !strings.HasPrefix(stdout, "record table Ticket {\n") || !strings.Contains(stdout, "// Json.Load<Ticket>(\"tickets.json\")?\n//\n// Choices:\n//   $: table\n") {
		t.Errorf("infer --name Ticket --explain: %v\n%s", err, stdout)
	}

	// A value with no declaration is printed with its place, and the
	// command fails.
	stdout, stderr, err = runJSON(t, documents, "infer", "tagged.json")
	if err == nil || !strings.Contains(stdout, "//   $ (line 1, column 1): a tagged array: \"kind\"") || strings.Contains(stdout, "Json.Load") {
		t.Errorf("infer tagged.json: %v\n%s", err, stdout)
	}
	if stderr != "json infer failed: 1 value has no declaration in tagged.json; see the end of the output\n" {
		t.Errorf("infer tagged.json wrote %q to stderr", stderr)
	}

	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"infer"}, "missing file"},
		{[]string{"infer", ""}, "missing file"},
		{[]string{"infer", "tickets.json", "tagged.json"}, "one file at a time"},
		{[]string{"infer", "tickets.json", "--names"}, "unknown option --names"},
		{[]string{"infer", "tickets.json", "--name"}, "--name needs a name"},
		{[]string{"infer", "tickets.json", "--name", "ticket"}, "is not a name a record can take"},
		{[]string{"infer", "missing.json"}, "missing.json: "},
		// A document that is not JSON is reported as Json.Load reports it.
		{[]string{"infer", "broken.json"}, "oct json infer: broken.json: (line 2, column 4): expected a value, found ']'"},
		{[]string{"guess", "tickets.json"}, `unknown json command "guess"`},
	} {
		stdout, stderr, err := runJSON(t, documents, c.args...)
		if err == nil || stdout != "" || !strings.Contains(stderr, c.want) {
			t.Errorf("oct json %v: %v\nstdout %q\nstderr %q, want it to hold %q", c.args, err, stdout, stderr, c.want)
		}
	}

	for _, args := range [][]string{{}, {"--help"}, {"infer", "--help"}} {
		stdout, stderr, err := runJSON(t, documents, args...)
		if err != nil || stderr != "" || !strings.HasPrefix(stdout, jsonInferUsage) {
			t.Errorf("oct json %v: %v\nstdout %q\nstderr %q", args, err, stdout, stderr)
		}
	}
}
