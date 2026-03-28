package build

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

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

func TestCompileRejectsUnsupportedBatch(t *testing.T) {
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
	_, err := Compile(mainPath)
	if err == nil {
		t.Fatal("expected compile to fail")
	}
	if !strings.Contains(err.Error(), "compiled mode does not yet support batch") {
		t.Fatalf("unexpected error: %v", err)
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
