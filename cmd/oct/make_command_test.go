package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

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
            Make.CommandTarget { Name: "Build" Inputs: [] Outputs: ["go-version.txt"] Deps: [] Program: "go" Args: ["version"] Cwd: "" Env: [] }
        ]
        FunctionTargets: []
        FlowTargets: []
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
fn Plan() -> Make.Plan { return Make.Plan { Default: "Build" Config: Make.DefaultConfig() CommandTargets: [] FunctionTargets: [] FlowTargets: [] PhonyTargets: [Make.PhonyTarget { Name: "Build" Deps: ["Missing"] }] } }
`, `dependency "Missing" does not exist`},
		{"duplicate", `package Main
import Make
fn Plan() -> Make.Plan { return Make.Plan { Default: "Build" Config: Make.DefaultConfig() CommandTargets: [Make.CommandTarget { Name: "Build" Inputs: [] Outputs: [] Deps: [] Program: "go" Args: ["version"] Cwd: "" Env: [] }] FunctionTargets: [] FlowTargets: [] PhonyTargets: [Make.PhonyTarget { Name: "Build" Deps: [] }] } }
`, `duplicate target name`},
		{"cycle", `package Main
import Make
fn Plan() -> Make.Plan { return Make.Plan { Default: "A" Config: Make.DefaultConfig() CommandTargets: [] FunctionTargets: [] FlowTargets: [] PhonyTargets: [Make.PhonyTarget { Name: "A" Deps: ["B"] }, Make.PhonyTarget { Name: "B" Deps: ["A"] }] } }
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
	outPath := filepath.ToSlash(filepath.Join(root, "out.txt"))
	writeFile(t, makeFile, fmt.Sprintf(`package Main
import Make

fn Plan() -> Make.Plan { return Make.Plan { Default: "Build" Config: Make.DefaultConfig() CommandTargets: [] FunctionTargets: [Make.FunctionTarget { Name: "Build" Inputs: [] Outputs: ["%s"] Deps: [] Function: "Build" }] FlowTargets: [] PhonyTargets: [] } }
fn Build() -> Void ! Error { let _w = Make.WriteText("%s", "made")? }
`, outPath, outPath))
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
	outPath := filepath.ToSlash(filepath.Join(root, "out.txt"))
	writeFile(t, makeFile, fmt.Sprintf(`package Main
import Make

fn BaseConfig() -> Make.Config {
    return Make.Config {
        Profile: "Debug"
        StateDir: ".octmake"
        Trace: false
        Staleness: Make.Staleness.Timestamp
    }
}

fn ReleaseConfig() -> Make.Config {
    return BaseConfig() with {
        Profile: "Release"
        Trace: true
        StateDir: "state-release"
    }
}

fn Plan() -> Make.Plan {
    return Make.Plan {
        Default: "Build"
        Config: ReleaseConfig()
        CommandTargets: []
        FunctionTargets: [Make.FunctionTarget { Name: "Build" Inputs: [] Outputs: ["%s"] Deps: [] Function: "Build" }]
        FlowTargets: []
        PhonyTargets: []
    }
}
fn Build() -> Void ! Error { let _w = Make.WriteText("%s", "made")? }
`, outPath, outPath))
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
	outPath := filepath.ToSlash(filepath.Join(root, "out.txt"))
	writeFile(t, makeFile, fmt.Sprintf(`package Main
import Make

fn Plan() -> Make.Plan {
    return Make.Plan {
        Default: "Build"
        Config: Make.Config { Profile: "Always" StateDir: "" Trace: false Staleness: Make.Staleness.Always }
        CommandTargets: []
        FunctionTargets: [Make.FunctionTarget { Name: "Build" Inputs: [] Outputs: ["%s"] Deps: [] Function: "Build" }]
        FlowTargets: []
        PhonyTargets: []
    }
}
fn Build() -> Void ! Error { let _w = Make.WriteText("%s", "made")? }
`, outPath, outPath))
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

	time.Sleep(1100 * time.Millisecond)
	writeFile(t, makeFile, fmt.Sprintf(`package Main
import Make
fn Plan() -> Make.Plan { return Make.Plan { Default: "Build" Config: Make.Config { Profile: "Fail" StateDir: ".octmake" Trace: true Staleness: Make.Staleness.Always } CommandTargets: [] FunctionTargets: [Make.FunctionTarget { Name: "Build" Inputs: [] Outputs: ["%s"] Deps: [] Function: "Build" }] FlowTargets: [] PhonyTargets: [] } }
fn Build() -> Int ! Error { return error("boom") }
`, outPath))
	_, stderr, err := executeCLIArgs("make", "--file", makeFile)
	if err == nil {
		t.Fatalf("expected function failure stderr=%q", stderr)
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
        CommandTargets: [Make.CommandTarget { Name: "Build" Inputs: [] Outputs: ["out.txt"] Deps: [] Program: "go" Args: ["definitely-not-a-go-subcommand"] Cwd: "" Env: [] }]
        FunctionTargets: []
        FlowTargets: []
        PhonyTargets: []
    }
}
`)
	_, stderr, err := executeCLIArgs("make", "--file", makeFile)
	if err == nil {
		t.Fatalf("expected command failure")
	}
	if !strings.Contains(stderr, "failure artifact:") {
		t.Fatalf("stderr missing failure artifact path:\n%s", stderr)
	}
	artifactPath := failureArtifactPathFromOutput(t, stderr)
	if _, err := octagon.Load(artifactPath); err != nil {
		t.Fatalf("failure artifact invalid: %v", err)
	}
	artifactBody, _ := os.ReadFile(artifactPath)
	for _, want := range []string{`MakeFailureArtifact`, `Target: "Build"`, `TargetKind: "command"`, `Program: "go"`, `Args: ["definitely-not-a-go-subcommand"]`, `ExitCode: 2`, `CommandHash: "`} {
		if !strings.Contains(string(artifactBody), want) {
			t.Fatalf("failure artifact missing %s:\n%s", want, artifactBody)
		}
	}
	trace := filepath.Join(root, ".octmake", "trace.octagon")
	if _, err := octagon.Load(trace); err != nil {
		t.Fatalf("command failure trace invalid: %v", err)
	}
	body, _ := os.ReadFile(trace)
	for _, want := range []string{`Status: "Failed"`, `CommandProgram: "go"`, `ExitCode: 2`, `FailureArtifactPath: "`} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("command failure trace missing %s:\n%s", want, body)
		}
	}
	stateBody, _ := os.ReadFile(filepath.Join(root, ".octmake", "state.octagon"))
	if !strings.Contains(string(stateBody), `LastStatus: "Failed"`) {
		t.Fatalf("command failure state missing failed status:\n%s", stateBody)
	}
}

func TestMakeFailureArtifactStateDirAndNoArtifactReadOnlyModes(t *testing.T) {
	root := repoTempDir(t)
	makeFile := filepath.Join(root, "Make.oct")
	writeFile(t, makeFile, `package Main
import Make
fn Plan() -> Make.Plan {
    return Make.Plan {
        Default: "Build"
        Config: Make.Config { Profile: "CommandFail" StateDir: "custom-state" Trace: false Staleness: Make.Staleness.Always }
        CommandTargets: [Make.CommandTarget { Name: "Build" Inputs: [] Outputs: ["out.txt"] Deps: [] Program: "go" Args: ["definitely-not-a-go-subcommand"] Cwd: "" Env: [] }]
        FunctionTargets: []
        FlowTargets: []
        PhonyTargets: []
    }
}
`)
	stdout, stderr, err := executeCLIArgs("make", "--file", makeFile, "--dry-run")
	if err != nil || !strings.Contains(stdout, "run Build") {
		t.Fatalf("dry-run failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "custom-state", "failures")); !os.IsNotExist(err) {
		t.Fatalf("dry-run created failure artifacts/stat=%v", err)
	}
	if _, stderr, err := executeCLIArgs("make", "explain", "--file", makeFile); err != nil {
		t.Fatalf("explain failed: %v stderr=%s", err, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "custom-state", "failures")); !os.IsNotExist(err) {
		t.Fatalf("explain created failure artifacts/stat=%v", err)
	}
	if _, stderr, err := executeCLIArgs("make", "doctor", "--file", makeFile); err != nil {
		t.Fatalf("doctor failed: %v stderr=%s", err, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "custom-state", "failures")); !os.IsNotExist(err) {
		t.Fatalf("doctor created failure artifacts/stat=%v", err)
	}
	planOut := filepath.Join(root, "plan.octagon")
	if _, stderr, err := executeCLIArgs("make", "--file", makeFile, "--plan-out", planOut); err != nil {
		t.Fatalf("plan-out failed: %v stderr=%s", err, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "custom-state", "failures")); !os.IsNotExist(err) {
		t.Fatalf("plan-out created failure artifacts/stat=%v", err)
	}

	_, stderr, err = executeCLIArgs("make", "--file", makeFile)
	if err == nil {
		t.Fatalf("expected failure")
	}
	artifactPath := failureArtifactPathFromOutput(t, stderr)
	if !strings.Contains(filepath.ToSlash(artifactPath), "/custom-state/failures/Build/") {
		t.Fatalf("artifact not under custom state dir: %s", artifactPath)
	}
	if _, err := octagon.Load(artifactPath); err != nil {
		t.Fatalf("custom state artifact invalid: %v", err)
	}
}

func TestMakeFunctionFailureWritesArtifact(t *testing.T) {
	root := repoTempDir(t)
	makeFile := filepath.Join(root, "Make.oct")
	writeFile(t, makeFile, `package Main
import Make
fn Plan() -> Make.Plan {
    return Make.Plan {
        Default: "Build"
        Config: Make.Config { Profile: "FunctionFail" StateDir: ".octmake" Trace: false Staleness: Make.Staleness.Always }
        CommandTargets: []
        FunctionTargets: [Make.FunctionTarget { Name: "Build" Inputs: [] Outputs: [] Deps: [] Function: "Build" }]
        FlowTargets: []
        PhonyTargets: []
    }
}
fn Build() -> Int ! Error { return error("boom") }
`)
	_, stderr, err := executeCLIArgs("make", "--file", makeFile)
	if err == nil {
		t.Fatalf("expected function failure")
	}
	artifactPath := failureArtifactPathFromOutput(t, stderr)
	if _, err := octagon.Load(artifactPath); err != nil {
		t.Fatalf("function failure artifact invalid: %v", err)
	}
	body, _ := os.ReadFile(artifactPath)
	for _, want := range []string{`Target: "Build"`, `TargetKind: "function"`, `Function: "Build"`, `Error: "`, `boom`} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("function artifact missing %s:\n%s", want, body)
		}
	}
}

func failureArtifactPathFromOutput(t *testing.T, output string) string {
	t.Helper()
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "failure artifact: ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "failure artifact: "))
		}
	}
	t.Fatalf("missing failure artifact path in output:\n%s", output)
	return ""
}

func TestMakeFlowTargetSuccessTraceListDryRunAndPhony(t *testing.T) {
	root := repoTempDir(t)
	makeFile := filepath.Join(root, "Make.oct")
	writeFile(t, makeFile, `package Main
import Make

flow BuildFlow() -> Int {
    state Start { goto Done }
    state Done { return 0 }
}

fn Plan() -> Make.Plan {
    return Make.Plan {
        Default: "All"
        Config: Make.Config { Profile: "Flow" StateDir: ".octmake" Trace: true Staleness: Make.Staleness.Always }
        CommandTargets: []
        FunctionTargets: []
        FlowTargets: [Make.FlowTarget { Name: "BuildFlowTarget" Inputs: [] Outputs: [] Deps: [] Flow: "BuildFlow" MaxSteps: 10 }]
        PhonyTargets: [Make.PhonyTarget { Name: "All" Deps: ["BuildFlowTarget"] }]
    }
}
`)
	stdout, stderr, err := executeCLIArgs("make", "--file", makeFile, "--list")
	if err != nil {
		t.Fatalf("list failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "BuildFlowTarget") || !strings.Contains(stdout, "flow") {
		t.Fatalf("list missing flow target: %q", stdout)
	}
	stdout, stderr, err = executeCLIArgs("make", "--file", makeFile, "BuildFlowTarget", "--dry-run", "--trace")
	if err != nil {
		t.Fatalf("dry run failed: %v stderr=%s", err, stderr)
	}
	if !strings.Contains(stdout, "run BuildFlowTarget") {
		t.Fatalf("dry run missing flow decision: %q", stdout)
	}
	tracePath := filepath.Join(root, ".octmake", "trace.octagon")
	body, _ := os.ReadFile(tracePath)
	if strings.Contains(string(body), `StateHistory: ["Start"`) {
		t.Fatalf("dry run should not step flow:\n%s", body)
	}
	stdout, stderr, err = executeCLIArgs("make", "--file", makeFile)
	if err != nil {
		t.Fatalf("flow make failed: %v stdout=%s stderr=%s", err, stdout, stderr)
	}
	body, _ = os.ReadFile(tracePath)
	for _, want := range []string{`Kind: "flow"`, `Flow: "BuildFlow"`, `ResultCode: 0`, `StateHistory: ["Start", "Done"]`, `Suspended: false`} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("flow trace missing %s:\n%s", want, body)
		}
	}
	stateBody, _ := os.ReadFile(filepath.Join(root, ".octmake", "state.octagon"))
	if !strings.Contains(string(stateBody), `Kind: "flow"`) || !strings.Contains(string(stateBody), `LastStatus: "Succeeded"`) {
		t.Fatalf("flow state missing evidence:\n%s", stateBody)
	}
}

func TestMakeFlowTargetFailureModes(t *testing.T) {
	tests := []struct {
		name  string
		flow  string
		plan  string
		want  string
		trace []string
	}{
		{
			name:  "nonzero",
			flow:  `flow BuildFlow() -> Int { state Start { return 1 } }`,
			plan:  `Make.FlowTarget { Name: "Build" Inputs: [] Outputs: [] Deps: [] Flow: "BuildFlow" MaxSteps: 10 }`,
			want:  `flow "BuildFlow" returned nonzero result 1`,
			trace: []string{`ResultCode: 1`, `Status: "Failed"`},
		},
		{
			name:  "suspend",
			flow:  `flow BuildFlow() -> Int { state Start { suspend return 0 } }`,
			plan:  `Make.FlowTarget { Name: "Build" Inputs: [] Outputs: [] Deps: [] Flow: "BuildFlow" MaxSteps: 10 }`,
			want:  `target "Build": flow "BuildFlow" suspended before completion; persistent make flow resume is not supported in MAKE4`,
			trace: []string{`Suspended: true`, `StateHistory: ["Start"]`},
		},
		{
			name:  "maxsteps",
			flow:  `flow BuildFlow() -> Int { state A { goto B } state B { goto A } }`,
			plan:  `Make.FlowTarget { Name: "Build" Inputs: [] Outputs: [] Deps: [] Flow: "BuildFlow" MaxSteps: 3 }`,
			want:  `flow "BuildFlow" exceeded MaxSteps 3`,
			trace: []string{`Steps: 4`, `Status: "Failed"`},
		},
		{
			name:  "missing",
			flow:  `flow OtherFlow() -> Int { state Start { return 0 } }`,
			plan:  `Make.FlowTarget { Name: "Build" Inputs: [] Outputs: [] Deps: [] Flow: "BuildFlow" MaxSteps: 10 }`,
			want:  `missing flow Main.BuildFlow`,
			trace: []string{`Flow: "BuildFlow"`, `Status: "Failed"`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := repoTempDir(t)
			makeFile := filepath.Join(root, "Make.oct")
			writeFile(t, makeFile, `package Main
import Make
`+tc.flow+`
fn Plan() -> Make.Plan {
    return Make.Plan {
        Default: "Build"
        Config: Make.Config { Profile: "FlowFail" StateDir: ".octmake" Trace: true Staleness: Make.Staleness.Always }
        CommandTargets: []
        FunctionTargets: []
        FlowTargets: [`+tc.plan+`]
        PhonyTargets: []
    }
}
`)
			_, _, err := executeCLIArgs("make", "--file", makeFile)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q error, got %v", tc.want, err)
			}
			body, _ := os.ReadFile(filepath.Join(root, ".octmake", "trace.octagon"))
			for _, want := range tc.trace {
				if !strings.Contains(string(body), want) {
					t.Fatalf("trace missing %s:\n%s", want, body)
				}
			}
		})
	}
}

func TestMakeFlowTargetValidationAndStaleness(t *testing.T) {
	root := repoTempDir(t)
	makeFile := filepath.Join(root, "Make.oct")
	writeFile(t, makeFile, `package Main
import Make
flow BuildFlow() -> Int { state Start { return 0 } }
fn Plan() -> Make.Plan { return Make.Plan { Default: "Build" Config: Make.DefaultConfig() CommandTargets: [] FunctionTargets: [] FlowTargets: [Make.FlowTarget { Name: "Build" Inputs: [] Outputs: [] Deps: [] Flow: "BuildFlow" MaxSteps: 0 }] PhonyTargets: [] } }
`)
	_, _, err := executeCLIArgs("make", "--file", makeFile)
	if err == nil || !strings.Contains(err.Error(), "MaxSteps must be positive") {
		t.Fatalf("expected max steps validation error, got %v", err)
	}

	out := filepath.Join(root, "out.txt")
	writeFile(t, out, "existing")
	writeFile(t, makeFile, `package Main
import Make
flow BuildFlow() -> Int { state Start { goto Done } state Done { return 0 } }
fn Plan() -> Make.Plan { return Make.Plan { Default: "Build" Config: Make.Config { Profile: "FlowStale" StateDir: ".octmake" Trace: true Staleness: Make.Staleness.Timestamp } CommandTargets: [] FunctionTargets: [] FlowTargets: [Make.FlowTarget { Name: "Build" Inputs: [] Outputs: ["out.txt"] Deps: [] Flow: "BuildFlow" MaxSteps: 10 }] PhonyTargets: [] } }
`)
	stdout, stderr, err := executeCLIArgs("make", "--file", makeFile)
	if err != nil {
		t.Fatalf("up-to-date flow failed: %v stdout=%s stderr=%s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "skip Build") {
		t.Fatalf("expected up-to-date skip, got %q", stdout)
	}
	writeFile(t, makeFile, `package Main
import Make
flow BuildFlow() -> Int { state Start { goto Done } state Done { return 0 } }
fn Plan() -> Make.Plan { return Make.Plan { Default: "Build" Config: Make.Config { Profile: "FlowAlways" StateDir: ".octmake" Trace: true Staleness: Make.Staleness.Always } CommandTargets: [] FunctionTargets: [] FlowTargets: [Make.FlowTarget { Name: "Build" Inputs: [] Outputs: ["out.txt"] Deps: [] Flow: "BuildFlow" MaxSteps: 10 }] PhonyTargets: [] } }
`)
	stdout, stderr, err = executeCLIArgs("make", "--file", makeFile)
	if err != nil {
		t.Fatalf("always flow failed: %v stdout=%s stderr=%s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "run Build") {
		t.Fatalf("expected always run, got %q", stdout)
	}
}

func TestMakePlanOutExplainAndDoctor(t *testing.T) {
	root := repoTempDir(t)
	makeFile := filepath.Join(root, "Make.oct")
	input := filepath.Join(root, "input.txt")
	output := filepath.Join(root, "output.txt")
	writeFile(t, input, "in")
	writeFile(t, output, "out")
	writeFile(t, makeFile, `package Main
import Make

fn Plan() -> Make.Plan {
    return Make.Plan {
        Default: "All"
        Config: Make.Config { Profile: "Report" StateDir: ".octmake/report" Trace: false Staleness: Make.Staleness.Timestamp }
        CommandTargets: [
            Make.CommandTarget { Name: "Copy" Inputs: ["input.txt"] Outputs: ["output.txt"] Deps: [] Program: "sh" Args: ["-c", "cp input.txt output.txt"] Cwd: "" Env: [] },
            Make.CommandTarget { Name: "Missing" Inputs: [] Outputs: ["missing.txt"] Deps: [] Program: "sh" Args: ["-c", "touch missing.txt"] Cwd: "" Env: [] }
        ]
        FunctionTargets: [Make.FunctionTarget { Name: "Check" Inputs: [] Outputs: [] Deps: [] Function: "Check" }]
        FlowTargets: []
        PhonyTargets: [Make.PhonyTarget { Name: "All" Deps: ["Copy", "Check", "Missing"] }]
    }
}
fn Check() -> Void { }
`)
	planOut := filepath.Join(root, "snapshots", "plan.octagon")
	stdout, stderr, err := executeCLIArgs("make", "--file", makeFile, "--plan-out", planOut)
	if err != nil {
		t.Fatalf("plan-out failed: %v stdout=%s stderr=%s", err, stdout, stderr)
	}
	if _, err := octagon.Load(planOut); err != nil {
		t.Fatalf("plan snapshot is not valid octagon: %v", err)
	}
	body, _ := os.ReadFile(planOut)
	for _, want := range []string{`MakePlanSnapshot`, `Default: "All"`, `Profile: "Report"`, `Name: "Copy"`, `Function: "Check"`} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("plan snapshot missing %s:\n%s", want, body)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "missing.txt")); !os.IsNotExist(err) {
		t.Fatalf("plan-out unexpectedly created output/stat=%v", err)
	}

	stdout, stderr, err = executeCLIArgs("make", "explain", "All", "--file", makeFile)
	if err != nil {
		t.Fatalf("explain failed: %v stderr=%s", err, stderr)
	}
	for _, want := range []string{"Explain target All:", "Copy [command]: would run", "reason: CommandHashMissing", "Missing [command]: would run", "reason: MissingOutput", "Check [function]: would run", "reason: NoOutputs", "All [phony]: would run", "reason: Phony"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("explain missing %q:\n%s", want, stdout)
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".octmake", "report", "state.octagon")); !os.IsNotExist(err) {
		t.Fatalf("explain created state/stat=%v", err)
	}

	stdout, stderr, err = executeCLIArgs("make", "doctor", "--file", makeFile)
	if err != nil {
		t.Fatalf("doctor failed: %v stderr=%s", err, stderr)
	}
	for _, want := range []string{"oct make doctor", "Make file:", "Profile: Report", "State dir: .octmake/report", "Backend: direct", "Default target: All", "command: 2", "function: 1", "phony: 1", "Validation: ok", "State:", "Trace:", "Command identity hashing: enabled", "Programs referenced:", "sh"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("doctor missing %q:\n%s", want, stdout)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "missing.txt")); !os.IsNotExist(err) {
		t.Fatalf("doctor unexpectedly executed target/stat=%v", err)
	}
}

func TestMakeCommandHashStateExplainTraceAndPlan(t *testing.T) {
	root := repoTempDir(t)
	helper := filepath.Join(root, "writefile.go")
	writeFile(t, helper, `package main
import (
    "os"
)
func main() {
    if len(os.Args) < 3 { os.Exit(2) }
    if err := os.WriteFile(os.Args[1], []byte(os.Args[2]), 0644); err != nil { os.Exit(1) }
}
`)
	makeFile := filepath.Join(root, "Make.oct")
	writeCommandHashMakefile := func(argValue, envValue string) {
		writeFile(t, makeFile, fmt.Sprintf(`package Main
import Make
fn Plan() -> Make.Plan {
    return Make.Plan {
        Default: "Build"
        Config: Make.Config { Profile: "Hash" StateDir: ".octmake" Trace: true Staleness: Make.Staleness.Timestamp }
        CommandTargets: [Make.CommandTarget { Name: "Build" Inputs: [] Outputs: ["out.txt"] Deps: [] Program: "go" Args: ["run", "writefile.go", "out.txt", "%s"] Cwd: "" Env: ["OCT_MAKE_HASH_TEST=%s"] }]
        FunctionTargets: []
        FlowTargets: []
        PhonyTargets: []
    }
}
`, argValue, envValue))
	}

	writeCommandHashMakefile("one", "A")
	stdout, stderr, err := executeCLIArgs("make", "--file", makeFile)
	if err != nil || !strings.Contains(stdout, "run Build") {
		t.Fatalf("initial make failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	statePath := filepath.Join(root, ".octmake", "state.octagon")
	stateBody, _ := os.ReadFile(statePath)
	if !strings.Contains(string(stateBody), `CommandHash: "`) {
		t.Fatalf("state missing command hash:\n%s", stateBody)
	}
	beforeState := string(stateBody)

	stdout, stderr, err = executeCLIArgs("make", "explain", "--file", makeFile, "Build")
	if err != nil || !strings.Contains(stdout, "Build [command]: would skip") || !strings.Contains(stdout, "reason: UpToDate") {
		t.Fatalf("same command explain failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	afterExplain, _ := os.ReadFile(statePath)
	if string(afterExplain) != beforeState {
		t.Fatalf("explain mutated state")
	}

	writeCommandHashMakefile("two", "A")
	stdout, stderr, err = executeCLIArgs("make", "explain", "--file", makeFile, "Build")
	if err != nil || !strings.Contains(stdout, "reason: CommandChanged") {
		t.Fatalf("arg change explain failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	afterArgExplain, _ := os.ReadFile(statePath)
	if string(afterArgExplain) != beforeState {
		t.Fatalf("arg-change explain mutated state")
	}

	writeCommandHashMakefile("one", "B")
	stdout, stderr, err = executeCLIArgs("make", "explain", "--file", makeFile, "Build")
	if err != nil || !strings.Contains(stdout, "reason: CommandChanged") {
		t.Fatalf("env change explain failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}

	mutated := strings.Replace(beforeState, regexp.MustCompile(`(?m)^            CommandHash: "[^"]+"\n`).FindString(beforeState), "", 1)
	if err := os.WriteFile(statePath, []byte(mutated), 0644); err != nil {
		t.Fatal(err)
	}
	writeCommandHashMakefile("one", "A")
	stdout, stderr, err = executeCLIArgs("make", "explain", "--file", makeFile, "Build")
	if err != nil || !strings.Contains(stdout, "reason: CommandHashMissing") {
		t.Fatalf("missing hash explain failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	if body, _ := os.ReadFile(statePath); string(body) != mutated {
		t.Fatalf("missing-hash explain mutated state")
	}

	tracePath := filepath.Join(root, ".octmake", "trace.octagon")
	stdout, stderr, err = executeCLIArgs("make", "--file", makeFile, "Build", "--dry-run", "--trace")
	if err != nil || !strings.Contains(stdout, "run Build") {
		t.Fatalf("dry-run failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	traceBody, _ := os.ReadFile(tracePath)
	if !strings.Contains(string(traceBody), `CommandHash: "`) || !strings.Contains(string(traceBody), `PreviousCommandHash: "`) {
		t.Fatalf("trace missing command hashes:\n%s", traceBody)
	}

	planOut := filepath.Join(root, "plan.octagon")
	stdout, stderr, err = executeCLIArgs("make", "--file", makeFile, "--plan-out", planOut)
	if err != nil {
		t.Fatalf("plan-out failed err=%v stdout=%q stderr=%q", err, stdout, stderr)
	}
	planBody, _ := os.ReadFile(planOut)
	if !strings.Contains(string(planBody), `CommandHash: "`) {
		t.Fatalf("plan snapshot missing command hash:\n%s", planBody)
	}
}
