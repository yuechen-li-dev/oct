package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/octagon"
)

func TestMakeListDryRunCommandTraceAndValidation(t *testing.T) {
	root := repoTempDir(t)
	makeFile := filepath.Join(root, "Make.oct")
	writeFile(t, makeFile, `package Main
import Make

fn Plan() -> Make.Plan {
    return Make.Plan {
        Default: "Build"
        Config: Make.DefaultConfig()
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
fn Plan() -> Make.Plan { return Make.Plan { Default: "Build" Config: Make.DefaultConfig() CommandTargets: [] FunctionTargets: [] PhonyTargets: [Make.PhonyTarget { Name: "Build" Deps: ["Missing"] }] } }
`, `dependency "Missing" does not exist`},
		{"duplicate", `package Main
import Make
fn Plan() -> Make.Plan { return Make.Plan { Default: "Build" Config: Make.DefaultConfig() CommandTargets: [Make.CommandTarget { Name: "Build" Inputs: [] Outputs: [] Deps: [] Program: "go" Args: ["version"] Cwd: "" }] FunctionTargets: [] PhonyTargets: [Make.PhonyTarget { Name: "Build" Deps: [] }] } }
`, `duplicate target name`},
		{"cycle", `package Main
import Make
fn Plan() -> Make.Plan { return Make.Plan { Default: "A" Config: Make.DefaultConfig() CommandTargets: [] FunctionTargets: [] PhonyTargets: [Make.PhonyTarget { Name: "A" Deps: ["B"] }, Make.PhonyTarget { Name: "B" Deps: ["A"] }] } }
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

fn Plan() -> Make.Plan { return Make.Plan { Default: "Build" Config: Make.DefaultConfig() CommandTargets: [] FunctionTargets: [Make.FunctionTarget { Name: "Build" Inputs: [] Outputs: ["out.txt"] Deps: [] Function: "Build" }] PhonyTargets: [] } }
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

func TestMakeConfigStateTraceStalenessAndWith(t *testing.T) {
	requireSlowOctxiliary(t)
	root := repoTempDir(t)
	makeFile := filepath.Join(root, "Make.oct")
	writeFile(t, makeFile, `package Main
import Make

let Base = Make.Config {
    Profile: "Debug"
    StateDir: ".octmake"
    Trace: false
    Staleness: Make.Staleness.Timestamp
}

let Release = Base with {
    Profile: "Release"
    Trace: true
    StateDir: "state-release"
}

fn Plan() -> Make.Plan {
    return Make.Plan {
        Default: "Build"
        Config: Release
        CommandTargets: []
        FunctionTargets: [Make.FunctionTarget { Name: "Build" Inputs: [] Outputs: ["out.txt"] Deps: [] Function: "Build" }]
        PhonyTargets: []
    }
}
fn Build() -> Void ! Error { let _w = Make.WriteText("out.txt", "made")? }
`)
	old := os.Getenv("OCT_WRAPPER_PATH")
	os.Setenv("OCT_WRAPPER_PATH", sharedTestSidecarDir(t, "octxiliary-makehost"))
	defer os.Setenv("OCT_WRAPPER_PATH", old)
	stdout, stderr, err := executeCLIArgs("make", "--file", makeFile)
	if err != nil {
		t.Fatalf("make failed: %v stdout=%s stderr=%s", err, stdout, stderr)
	}
	state := filepath.Join(root, "state-release", "state.octagon")
	trace := filepath.Join(root, "state-release", "trace.octagon")
	if _, err := octagon.Load(state); err != nil {
		t.Fatalf("state not valid octagon: %v", err)
	}
	if _, err := octagon.Load(trace); err != nil {
		t.Fatalf("trace not valid octagon: %v", err)
	}
	body, _ := os.ReadFile(trace)
	for _, want := range []string{`Profile: "Release"`, `StateDir: "state-release"`, `SelectedTarget: "Build"`, `DryRun: false`, `MakeDecision`, `Status: "Ran"`} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("trace missing %s:\n%s", want, body)
		}
	}
	stdout, stderr, err = executeCLIArgs("make", "--file", makeFile)
	if err != nil || !strings.Contains(stdout, "skip Build") {
		t.Fatalf("timestamp should skip err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
}

func TestMakeConfigAlwaysDryRunAndFailureTrace(t *testing.T) {
	requireSlowOctxiliary(t)
	root := repoTempDir(t)
	makeFile := filepath.Join(root, "Make.oct")
	writeFile(t, makeFile, `package Main
import Make

fn Plan() -> Make.Plan {
    return Make.Plan {
        Default: "Build"
        Config: Make.Config { Profile: "Always" StateDir: "" Trace: false Staleness: Make.Staleness.Always }
        CommandTargets: []
        FunctionTargets: [Make.FunctionTarget { Name: "Build" Inputs: [] Outputs: ["out.txt"] Deps: [] Function: "Build" }]
        PhonyTargets: []
    }
}
fn Build() -> Void ! Error { let _w = Make.WriteText("out.txt", "made")? }
`)
	old := os.Getenv("OCT_WRAPPER_PATH")
	os.Setenv("OCT_WRAPPER_PATH", sharedTestSidecarDir(t, "octxiliary-makehost"))
	defer os.Setenv("OCT_WRAPPER_PATH", old)
	if stdout, stderr, err := executeCLIArgs("make", "--file", makeFile); err != nil || !strings.Contains(stdout, "run Build") {
		t.Fatalf("initial always failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	if stdout, stderr, err := executeCLIArgs("make", "--file", makeFile); err != nil || !strings.Contains(stdout, "run Build") {
		t.Fatalf("always rerun failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	before, _ := os.ReadFile(filepath.Join(root, "out.txt"))
	if stdout, stderr, err := executeCLIArgs("make", "--file", makeFile, "--dry-run", "--trace"); err != nil || !strings.Contains(stdout, "run Build") {
		t.Fatalf("dry trace failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	after, _ := os.ReadFile(filepath.Join(root, "out.txt"))
	if string(before) != string(after) {
		t.Fatalf("dry-run mutated output")
	}
	trace := filepath.Join(root, ".octmake", "trace.octagon")
	if _, err := octagon.Load(trace); err != nil {
		t.Fatalf("dry trace invalid: %v", err)
	}
	body, _ := os.ReadFile(trace)
	if !strings.Contains(string(body), `DryRun: true`) || !strings.Contains(string(body), `Reason: "DryRunWouldRun"`) {
		t.Fatalf("dry trace missing evidence:\n%s", body)
	}

	writeFile(t, makeFile, `package Main
import Make
fn Plan() -> Make.Plan { return Make.Plan { Default: "Build" Config: Make.Config { Profile: "Fail" StateDir: ".octmake" Trace: true Staleness: Make.Staleness.Always } CommandTargets: [] FunctionTargets: [Make.FunctionTarget { Name: "Build" Inputs: [] Outputs: ["out.txt"] Deps: [] Function: "Build" }] PhonyTargets: [] } }
fn Build() -> Void ! Error { return Error("boom") }
`)
	_, _, err := executeCLIArgs("make", "--file", makeFile)
	if err == nil {
		t.Fatalf("expected function failure")
	}
	body, _ = os.ReadFile(trace)
	if !strings.Contains(string(body), `Status: "Failed"`) || !strings.Contains(string(body), `Error:`) {
		t.Fatalf("failure trace missing status/error:\n%s", body)
	}
	stateBody, _ := os.ReadFile(filepath.Join(root, ".octmake", "state.octagon"))
	if !strings.Contains(string(stateBody), `LastStatus: "Failed"`) {
		t.Fatalf("failure state missing failed status:\n%s", stateBody)
	}
}

func TestMakeCommandFailureRecordsTraceAndState(t *testing.T) {
	root := repoTempDir(t)
	makeFile := filepath.Join(root, "Make.oct")
	writeFile(t, makeFile, `package Main
import Make
fn Plan() -> Make.Plan {
    return Make.Plan {
        Default: "Build"
        Config: Make.Config { Profile: "CommandFail" StateDir: ".octmake" Trace: true Staleness: Make.Staleness.Always }
        CommandTargets: [Make.CommandTarget { Name: "Build" Inputs: [] Outputs: ["out.txt"] Deps: [] Program: "go" Args: ["definitely-not-a-go-subcommand"] Cwd: "" }]
        FunctionTargets: []
        PhonyTargets: []
    }
}
`)
	_, _, err := executeCLIArgs("make", "--file", makeFile)
	if err == nil {
		t.Fatalf("expected command failure")
	}
	trace := filepath.Join(root, ".octmake", "trace.octagon")
	if _, err := octagon.Load(trace); err != nil {
		t.Fatalf("command failure trace invalid: %v", err)
	}
	body, _ := os.ReadFile(trace)
	for _, want := range []string{`Status: "Failed"`, `CommandProgram: "go"`, `ExitCode: 2`} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("command failure trace missing %s:\n%s", want, body)
		}
	}
	stateBody, _ := os.ReadFile(filepath.Join(root, ".octmake", "state.octagon"))
	if !strings.Contains(string(stateBody), `LastStatus: "Failed"`) {
		t.Fatalf("command failure state missing failed status:\n%s", stateBody)
	}
}
