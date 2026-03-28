package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"oct/internal/octagon"
	"oct/internal/project"
	"oct/internal/typecheck"
)

func TestLowerProgramBuildsMIRShape(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn main() -> Int {
    let x = 2
    if x > 1 {
        return x + 3
    } else {
        return 0
    }
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	program, err := project.Load(mainPath)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatalf("typecheck: %v", err)
	}
	module, err := lowerProgram(program)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	if len(module.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(module.Functions))
	}
	if len(module.Functions[0].Blocks) < 3 {
		t.Fatalf("expected control-flow blocks, got %d", len(module.Functions[0].Blocks))
	}
	text := dumpMIR(module)
	if !strings.Contains(text, "branch") {
		t.Fatalf("expected branch in dump, got:\n%s", text)
	}
}

func TestCompileWritesMIRDumpWhenRequested(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn main() -> Int {
    return 7
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCT_MIR_DUMP", "1")
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if result.MIRDumpPath == "" {
		t.Fatal("expected MIR dump path")
	}
	data, err := os.ReadFile(result.MIRDumpPath)
	if err != nil {
		t.Fatalf("read MIR dump: %v", err)
	}
	if !strings.Contains(string(data), "fn Main.main") {
		t.Fatalf("unexpected MIR dump:\n%s", string(data))
	}
}

func TestCompileAndRunSubsetProgram(t *testing.T) {
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "Main"), 0o755)
	os.Mkdir(filepath.Join(root, "Math"), 0o755)
	mainSrc := `package Main

import Math

fn main() -> Int {
    let arr = [1, 2]
    let arr2 = Append(arr, 3)
    let n = Len(arr2)
    let pair = Math.Make(4, 5)
    if n == 3 {
        return pair.Left + n
    }
    return 0
}
`
	mathSrc := `package Math

record Pair {
    Left: Int
    Right: Int
}

fn Make(a: Int, b: Int) -> Pair {
    return Pair {
        Left: a
        Right: b
    }
}
`
	if err := os.WriteFile(filepath.Join(root, "Main", "main.oct"), []byte(mainSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Math", "math.oct"), []byte(mathSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(root)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "7" {
		t.Fatalf("expected 7, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunBatchParameterSweepAndOrder(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn main() -> Int {
    let values = batch [1, 2, 3, 4] as x {
        return x * x
    }
    return values[0] + values[1] + values[2] + values[3]
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "30" {
		t.Fatalf("expected 30, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunBatchDeterministicRepeatedRuns(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn main() -> Int {
    let input = [2, 4, 6]
    let first = batch input as x {
        return x + 1
    }
    let second = batch input as x {
        return x + 1
    }
    if first[0] == second[0] and first[1] == second[1] and first[2] == second[2] {
        return 1
    }
    return 0
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "1" {
		t.Fatalf("expected 1, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunBatchEmptyAndSingleElement(t *testing.T) {
	root := t.TempDir()
	emptyPath := filepath.Join(root, "empty.octagon")
	if err := os.WriteFile(emptyPath, []byte("[]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(root, "main.oct")
	src := fmt.Sprintf(`package Main

fn main() -> Int ! Error {
    let source = LoadOctagon[Int[]](%q)?
    let empty = batch source as x {
        return x + 1
    }
    let one = batch [9] as x {
        return x * 2
    }
    if Len(empty) == 0 and Len(one) == 1 {
        return one[0]
    }
    return 0
}
`, emptyPath)
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "18" {
		t.Fatalf("expected 18, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunBatchFailWholeBatch(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn MaybeSample(n: Int) -> Int ! Error {
    if n == 3 {
        return error("sample failed")
    }
    return n + 10
}

fn Collect() -> Int[] ! Error {
    let values = [1, 2, 3, 4]
    let results = batch values as item {
        return MaybeSample(item)?
    }
    return results
}

fn main() -> Int {
    match Collect() {
        ok(v) => { return 0 }
        err(e) => { return 42 }
    }
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "42" {
		t.Fatalf("expected 42, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunFalliblePropagationAndMatch(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn Parse(x: Int) -> Int ! Error {
    if x > 0 {
        return x + 10
    }
    return error("bad input")
}

fn Chain(x: Int) -> Int ! Error {
    let value = Parse(x)?
    return value + 1
}

fn main() -> Int {
    match Chain(5) {
        ok(v) => { return v }
        err(e) => { return 0 }
    }
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "16" {
		t.Fatalf("expected 16, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunFallibleMatchErrBranch(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn Parse(x: Int) -> Int ! Error {
    if x > 0 {
        return x
    }
    return error("bad input")
}

fn main() -> Int {
    match Parse(0) {
        ok(v) => { return v }
        err(e) => { return 42 }
    }
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "42" {
		t.Fatalf("expected 42, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunFallibleUnwrap(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn Parse(x: Int) -> Int ! Error {
    if x > 0 {
        return x
    }
    return error("bad input")
}

fn main() -> Int {
    let value = Parse(5)!
    return value + 2
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "7" {
		t.Fatalf("expected 7, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileFallibleUnwrapFailureIsFatal(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn Parse(x: Int) -> Int ! Error {
    if x > 0 {
        return x
    }
    return error("bad input")
}

fn main() -> Int {
    return Parse(0)!
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err == nil {
		t.Fatalf("expected fatal unwrap failure, got success output %q", strings.TrimSpace(string(out)))
	}
	if !strings.Contains(string(out), "unwrap failed: bad input") {
		t.Fatalf("expected unwrap failure message, got %q", string(out))
	}
}

func TestCompileAndRunWriteOctagonSuccess(t *testing.T) {
	root := t.TempDir()
	outPath := filepath.Join(root, "compiled-write.octagon")
	mainPath := filepath.Join(root, "main.oct")
	src := fmt.Sprintf(`package Main

record Payload {
    Name: String
    Count: Int
}

fn main() -> Int {
    let payload = Payload {
        Name: "run"
        Count: 3
    }
    WriteOctagon(%q, payload)
    return 0
}
`, outPath)
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if out, err := exec.Command(result.ArtifactPath).CombinedOutput(); err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	body, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if strings.TrimSpace(string(body)) != "Payload { Name: \"run\" Count: 3 }" {
		t.Fatalf("unexpected .octagon body: %q", string(body))
	}
	if _, err := octagon.Load(outPath); err != nil {
		t.Fatalf("expected written .octagon to be loadable, got %v", err)
	}
}

func TestCompileAndRunLoadOctagonSuccessAndFallibleIntegration(t *testing.T) {
	root := t.TempDir()
	inPath := filepath.Join(root, "input.octagon")
	if err := os.WriteFile(inPath, []byte("Payload { Name: \"run\" Count: 7 }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(root, "main.oct")
	src := fmt.Sprintf(`package Main

record Payload {
    Name: String
    Count: Int
}

fn LoadCount(path: String) -> Int ! Error {
    let payload = LoadOctagon[Payload](path)?
    return payload.Count
}

fn main() -> Int {
    match LoadCount(%q) {
        ok(v) => { return v }
        err(e) => { return 0 }
    }
}
`, inPath)
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "7" {
		t.Fatalf("expected 7, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunOctagonRoundTrip(t *testing.T) {
	root := t.TempDir()
	artifactPath := filepath.Join(root, "roundtrip.octagon")
	mainPath := filepath.Join(root, "main.oct")
	src := fmt.Sprintf(`package Main

record Payload {
    Name: String
    Samples: Int[]
}

fn main() -> Int ! Error {
    let payload = Payload {
        Name: "trial"
        Samples: [1, 2, 3]
    }
    WriteOctagon(%q, payload)
    let loaded = LoadOctagon[Payload](%q)?
    return loaded.Samples[2]
}
`, artifactPath, artifactPath)
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "3" {
		t.Fatalf("expected 3, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileAndRunLoadOctagonFailureReturnsErr(t *testing.T) {
	root := t.TempDir()
	inPath := filepath.Join(root, "bad.octagon")
	if err := os.WriteFile(inPath, []byte("7\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mainPath := filepath.Join(root, "main.oct")
	src := fmt.Sprintf(`package Main

record Payload {
    Name: String
    Count: Int
}

fn main() -> Int {
    match LoadOctagon[Payload](%q) {
        ok(v) => { return 0 }
        err(e) => { return 1 }
    }
}
`, inPath)
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "1" {
		t.Fatalf("expected err-branch result 1, got %q", strings.TrimSpace(string(out)))
	}
}

func TestMIRDumpShowsExplicitOctagonRuntimeCalls(t *testing.T) {
	root := t.TempDir()
	artifactPath := filepath.Join(root, "roundtrip.octagon")
	mainPath := filepath.Join(root, "main.oct")
	src := fmt.Sprintf(`package Main

record Payload {
    Count: Int
}

fn main() -> Int ! Error {
    WriteOctagon(%q, Payload { Count: 1 })
    let loaded = LoadOctagon[Payload](%q)?
    return loaded.Count
}
`, artifactPath, artifactPath)
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCT_MIR_DUMP", "1")
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	data, err := os.ReadFile(result.MIRDumpPath)
	if err != nil {
		t.Fatalf("read MIR dump: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "call WriteOctagon(") {
		t.Fatalf("expected WriteOctagon runtime call in MIR, got:\n%s", text)
	}
	if !strings.Contains(text, "call LoadOctagon(") {
		t.Fatalf("expected LoadOctagon runtime call in MIR, got:\n%s", text)
	}
}

func TestMIRDumpShowsExplicitBatchLowering(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

fn main() -> Int {
    let values = batch [1, 2] as x {
        return x + 1
    }
    return values[0]
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OCT_MIR_DUMP", "1")
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	data, err := os.ReadFile(result.MIRDumpPath)
	if err != nil {
		t.Fatalf("read MIR dump: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "batch_map") {
		t.Fatalf("expected batch_map in MIR, got:\n%s", text)
	}
	if !strings.Contains(text, "__batch_main_0") {
		t.Fatalf("expected lowered batch worker in MIR, got:\n%s", text)
	}
}

func TestCompileAndRunFlowCoreRuntimeBuiltins(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

flow Machine(input: Int) -> Int {
    state Start {
        if input > 0 {
            goto Track
        }
        suspend
        return 0
    }

    state Track {
        suspend
        return input
    }
}

fn main() -> Int ! Error {
    let f = Machine(4)
    if Active(f) != "" {
        return error("active should be empty before first step")
    }
    if Complete(f) {
        return error("new flow must start incomplete")
    }
    Step(f)
    if Active(f) != "Track" {
        return error("first step should enter Track")
    }
    let h1 = StateHistory(f)
    if h1[0] != "Start" or h1[1] != "Track" {
        return error("history should include Start and Track after goto")
    }
    Step(f)
    if not Complete(f) {
        return error("flow should be complete after second step")
    }
    if Active(f) != "" {
        return error("active should clear after completion")
    }
    Step(f)
    if Active(f) != "" {
        return error("step on completed flow should be no-op")
    }
    let value = Result(f)?
    return value
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "4" {
		t.Fatalf("expected 4, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileFlowResultBeforeCompletionFails(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

flow OneSuspend() -> Int {
    state Start {
        suspend
        return 7
    }
}

fn main() -> Int {
    let f = OneSuspend()
    match Result(f) {
        ok(v) => { return 0 }
        err(e) => {
            return 1
        }
    }
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Compile(mainPath)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	out, err := exec.Command(result.ArtifactPath).CombinedOutput()
	if err != nil {
		t.Fatalf("run artifact: %v (%s)", err, string(out))
	}
	if strings.TrimSpace(string(out)) != "1" {
		t.Fatalf("expected 1, got %q", strings.TrimSpace(string(out)))
	}
}

func TestCompileFlowStillRejectsDeferredOctomataFeatures(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "ordered-when",
			src: `package Main
flow F() -> Int { state S { when { case true -> return 1 else -> return 0 } } }
fn main() -> Int { return 0 }`,
		},
		{
			name: "remember",
			src: `package Main
flow F() -> Int { state A { remember goto B } state B { return 1 } }
fn main() -> Int { return 0 }`,
		},
		{
			name: "resume",
			src: `package Main
flow F() -> Int { state A { resume } }
fn main() -> Int { return 0 }`,
		},
		{
			name: "resume-target-builtin",
			src: `package Main
flow F() -> Int { state A { return 1 } }
fn main() -> String { let f = F() return ResumeTarget(f) }`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			mainPath := filepath.Join(root, "main.oct")
			if err := os.WriteFile(mainPath, []byte(tc.src), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Compile(mainPath)
			if err == nil {
				t.Fatal("expected compile to fail")
			}
			if !strings.Contains(err.Error(), "compiled mode does not yet support") {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestCompileFlowDecisionDoesNotUseSpecialCaseShimPath(t *testing.T) {
	root := t.TempDir()
	mainPath := filepath.Join(root, "main.oct")
	src := `package Main

flow Machine(flag: Bool) -> Int {
    state Start {
        when {
            case flag -> return 1
            else -> return 0
        }
    }
}

fn main() -> Int {
    let f = Machine(true)
    Step(f)
    match Result(f) {
        ok(v) => { return v }
        err(e) => { return 0 }
    }
}
`
	if err := os.WriteFile(mainPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Compile(mainPath)
	if err == nil {
		t.Fatal("expected compile to fail")
	}
	if !strings.Contains(err.Error(), "compiled mode does not yet support Octomata flow/state runtime in compiled mode (M64)") {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(err.Error(), "go build generated program") {
		t.Fatalf("expected normal lowering rejection, got shim-like build error: %v", err)
	}
}
