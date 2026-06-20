package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMakeListDryRunCommandTraceAndValidation(t *testing.T) {
	root := repoTempDir(t)
	makeFile := filepath.Join(root, "Make.oct")
	writeFile(t, makeFile, `package Main
import Make

fn Plan() -> Make.Plan {
    return Make.Plan {
        Default: "Build"
        CommandTargets: [
            Make.CommandTarget { Name: "Build" Inputs: [] Outputs: ["go-version.txt"] Deps: [] Program: "go" Args: ["version"] Cwd: "" }
        ]
        FunctionTargets: []
        PhonyTargets: [
            Make.PhonyTarget { Name: "All" Deps: ["Build"] }
        ]
    }
}
`)
	stdout, stderr, err := executeCLIArgs("make", "--file", makeFile, "--list")
	if err != nil {
		t.Fatalf("list failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "Build") || !strings.Contains(stdout, "command") || !strings.Contains(stdout, "All") || !strings.Contains(stdout, "phony") {
		t.Fatalf("unexpected list output: %q", stdout)
	}

	stdout, stderr, err = executeCLIArgs("make", "--file", makeFile, "--dry-run", "--trace")
	if err != nil {
		t.Fatalf("dry-run failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "run Build") {
		t.Fatalf("unexpected dry-run output: %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(root, "go-version.txt")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created output or stat failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".octmake", "trace.octagon")); err != nil {
		t.Fatalf("trace missing: %v", err)
	}

	stdout, stderr, err = executeCLIArgs("make", "--file", makeFile, "All")
	if err != nil {
		t.Fatalf("make All failed: %v stdout=%s stderr=%s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "go version") {
		t.Fatalf("expected command stdout, got %q", stdout)
	}

	_, stderr, err = executeCLIArgs("make", "--file", makeFile, "--backend", "ninja")
	if err == nil || !strings.Contains(stderr, "unsupported make backend \"ninja\"") {
		t.Fatalf("expected backend error, err=%v stderr=%q", err, stderr)
	}
}

func TestMakeValidationFailures(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"missing dep", `package Main
import Make
fn Plan() -> Make.Plan { return Make.Plan { Default: "Build" CommandTargets: [] FunctionTargets: [] PhonyTargets: [Make.PhonyTarget { Name: "Build" Deps: ["Missing"] }] } }
`, `dependency "Missing" does not exist`},
		{"duplicate", `package Main
import Make
fn Plan() -> Make.Plan { return Make.Plan { Default: "Build" CommandTargets: [Make.CommandTarget { Name: "Build" Inputs: [] Outputs: [] Deps: [] Program: "go" Args: ["version"] Cwd: "" }] FunctionTargets: [] PhonyTargets: [Make.PhonyTarget { Name: "Build" Deps: [] }] } }
`, `duplicate target name`},
		{"cycle", `package Main
import Make
fn Plan() -> Make.Plan { return Make.Plan { Default: "A" CommandTargets: [] FunctionTargets: [] PhonyTargets: [Make.PhonyTarget { Name: "A" Deps: ["B"] }, Make.PhonyTarget { Name: "B" Deps: ["A"] }] } }
`, `dependency cycle detected`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := repoTempDir(t)
			makeFile := filepath.Join(root, "Make.oct")
			writeFile(t, makeFile, tc.body)
			_, stderr, err := executeCLIArgs("make", "--file", makeFile)
			if err == nil || !strings.Contains(stderr, tc.want) {
				t.Fatalf("want %q, err=%v stderr=%q", tc.want, err, stderr)
			}
		})
	}
}

func TestMakeFunctionTargetGetsAuthority(t *testing.T) {
	requireSlowOctxiliary(t)
	root := repoTempDir(t)
	makeFile := filepath.Join(root, "Make.oct")
	writeFile(t, makeFile, `package Main
import Make

fn Plan() -> Make.Plan { return Make.Plan { Default: "Build" CommandTargets: [] FunctionTargets: [Make.FunctionTarget { Name: "Build" Inputs: [] Outputs: ["out.txt"] Deps: [] Function: "Build" }] PhonyTargets: [] } }
fn Build() -> Void ! Error { let _w = Make.WriteText("out.txt", "made")? }
`)
	old := os.Getenv("OCT_WRAPPER_PATH")
	os.Setenv("OCT_WRAPPER_PATH", sharedTestSidecarDir(t, "octxiliary-makehost"))
	defer os.Setenv("OCT_WRAPPER_PATH", old)
	stdout, stderr, err := executeCLIArgs("make", "--file", makeFile)
	if err != nil {
		t.Fatalf("make function failed: %v stdout=%s stderr=%s", err, stdout, stderr)
	}
	b, err := os.ReadFile(filepath.Join(root, "out.txt"))
	if err != nil || string(b) != "made" {
		t.Fatalf("output=%q err=%v", b, err)
	}
	stdout, stderr, err = executeCLIArgs("make", "--file", makeFile)
	if err != nil || !strings.Contains(stdout, "skip Build") {
		t.Fatalf("expected staleness skip err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
}

func repoTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(".", ".oct-make-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	abs, _ := filepath.Abs(dir)
	return abs
}
func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}
