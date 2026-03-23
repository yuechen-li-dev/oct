package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oct/internal/cli"
)

func TestRunCommandExecutesMainPrograms(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "int literal return",
			source: "fn Main() -> Int {\n" +
				"    return 42\n" +
				"}\n",
			want: "42\n",
		},
		{
			name: "float computation",
			source: "fn Main() -> Float {\n" +
				"    let x = 1\n" +
				"    let y = 2.5\n" +
				"    return x + y\n" +
				"}\n",
			want: "3.5\n",
		},
		{
			name: "bool return",
			source: "fn Main() -> Bool {\n" +
				"    return true\n" +
				"}\n",
			want: "true\n",
		},
		{
			name: "arithmetic precedence",
			source: "fn Main() -> Int {\n" +
				"    return 2 + 3 * 4\n" +
				"}\n",
			want: "14\n",
		},
		{
			name: "parenthesized arithmetic",
			source: "fn Main() -> Int {\n" +
				"    return (2 + 3) * 4\n" +
				"}\n",
			want: "20\n",
		},
		{
			name: "let chaining",
			source: "fn Main() -> Float {\n" +
				"    let a = 1\n" +
				"    let b = a + 2.5\n" +
				"    return b\n" +
				"}\n",
			want: "3.5\n",
		},
		{
			name: "integer division truncates",
			source: "fn Main() -> Int {\n" +
				"    return 5 / 2\n" +
				"}\n",
			want: "2\n",
		},
		{
			name: "mixed division promotes to float",
			source: "fn Main() -> Float {\n" +
				"    return 5 / 2.0\n" +
				"}\n",
			want: "2.5\n",
		},
		{
			name: "int array literal return",
			source: "fn Main() -> Int[] {\n" +
				"    return [1, 2, 3]\n" +
				"}\n",
			want: "[1, 2, 3]\n",
		},
		{
			name: "array indexing",
			source: "fn Main() -> Int {\n" +
				"    let x = [1, 2, 3]\n" +
				"    return x[1]\n" +
				"}\n",
			want: "2\n",
		},
		{
			name: "array arithmetic",
			source: "fn Main() -> Int[] {\n" +
				"    return [1, 2, 3] + [4, 5, 6]\n" +
				"}\n",
			want: "[5, 7, 9]\n",
		},
		{
			name: "float array arithmetic",
			source: "fn Main() -> Float[] {\n" +
				"    return [1.0, 2.0] + [3.0, 4.0]\n" +
				"}\n",
			want: "[4, 6]\n",
		},
		{
			name: "for loop returns first iteration",
			source: "fn Main() -> Int {\n" +
				"    for i in 0..3 {\n" +
				"        return i\n" +
				"    }\n" +
				"    return 0\n" +
				"}\n",
			want: "0\n",
		},
		{
			name: "for loop with step returns first iteration",
			source: "fn Main() -> Int {\n" +
				"    for i in 0..10 step 3 {\n" +
				"        return i\n" +
				"    }\n" +
				"    return 0\n" +
				"}\n",
			want: "0\n",
		},
		{
			name: "loop local scope",
			source: "fn Main() -> Int {\n" +
				"    for i in 0..1 {\n" +
				"        let x = i\n" +
				"        return x\n" +
				"    }\n" +
				"    return 0\n" +
				"}\n",
			want: "0\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sourcePath := writeSourceFile(t, test.name+".oct", test.source)

			stdout, stderr, err := executeCLI("run", sourcePath)
			if err != nil {
				t.Fatalf("run command failed: %v\nstdout:%s\nstderr:%s", err, stdout, stderr)
			}
			if stdout != test.want {
				t.Fatalf("expected stdout %q, got %q", test.want, stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestRunCommandSupportsM6ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantStdout  string
		wantMessage string
	}{
		{
			name: "infallible function call",
			source: `fn Add(x: Int, y: Int) -> Int {
    return x + y
}
fn Main() -> Int {
    return Add(1, 2)
}
`,
			wantStdout: "3\n",
		},
		{
			name: "fallible success",
			source: `fn Safe() -> Int ! Error {
    return 5
}
fn Main() -> Int ! Error {
    let x = Safe()?
    return x
}
`,
			wantStdout: "5\n",
		},
		{
			name: "fallible propagation",
			source: `fn Fail() -> Int ! Error {
    return error("bad input")
}
fn Main() -> Int ! Error {
    let x = Fail()?
    return x
}
`,
			wantMessage: "run failed: fatal error: bad input",
		},
		{
			name: "fatal unwrap",
			source: `fn Fail() -> Int ! Error {
    return error("boom")
}
fn Main() -> Int {
    let x = Fail()!
    return x
}
`,
			wantMessage: "run failed: fatal error: boom",
		},
		{
			name: "match success arm",
			source: `fn Safe() -> Int ! Error {
    return 7
}
fn Main() -> Int {
    match Safe() {
        ok(value) => {
            return value
        }
        err(e) => {
            return 0
        }
    }
}
`,
			wantStdout: "7\n",
		},
		{
			name: "match error arm",
			source: `fn Fail() -> Int ! Error {
    return error("bad")
}
fn Main() -> Int {
    match Fail() {
        ok(value) => {
            return value
        }
        err(e) => {
            return 0
        }
    }
}
`,
			wantStdout: "0\n",
		},
		{
			name: "fallible Main success",
			source: `fn Main() -> Int ! Error {
    return 9
}
`,
			wantStdout: "9\n",
		},
		{
			name: "fallible Main failure",
			source: `fn Main() -> Int ! Error {
    return error("top-level fail")
}
`,
			wantMessage: "run failed: fatal error: top-level fail",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sourcePath := writeSourceFile(t, test.name+".oct", test.source)
			stdout, stderr, err := executeCLI("run", sourcePath)
			if test.wantMessage != "" {
				if err == nil {
					t.Fatalf("expected failure, got success with stdout %q", stdout)
				}
				if stdout != "" {
					t.Fatalf("expected empty stdout on failure, got %q", stdout)
				}
				if !strings.Contains(stderr, test.wantMessage) {
					t.Fatalf("expected stderr to contain %q, got %q", test.wantMessage, stderr)
				}
				return
			}
			if err != nil {
				t.Fatalf("run command failed: %v\nstdout:%s\nstderr:%s", err, stdout, stderr)
			}
			if stdout != test.wantStdout {
				t.Fatalf("expected stdout %q, got %q", test.wantStdout, stdout)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
		})
	}
}

func TestRunCommandRejectsInvalidM6Programs(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantMessage string
	}{
		{
			name: "invalid question in infallible function",
			source: `fn Fail() -> Int ! Error {
    return error("bad")
}
fn Main() -> Int {
    let x = Fail()?
    return x
}
`,
			wantMessage: "run failed: function Main: let x: cannot use '?' in infallible function",
		},
		{
			name: "invalid question on infallible expression",
			source: `fn Main() -> Int {
    let x = 1?
    return x
}
`,
			wantMessage: "run failed: function Main: let x: operator '?' requires fallible expression",
		},
		{
			name: "invalid fallible signature",
			source: `fn Bad() -> Int ! MyError {
    return 1
}
`,
			wantMessage: "run failed: function Bad: only built-in Error is allowed in fallible signatures",
		},
		{
			name: "call arity mismatch",
			source: `fn Add(x: Int, y: Int) -> Int {
    return x + y
}
fn Main() -> Int {
    return Add(1)
}
`,
			wantMessage: "run failed: function Main: function 'Add' expects 2 arguments, got 1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sourcePath := writeSourceFile(t, test.name+".oct", test.source)
			stdout, stderr, err := executeCLI("run", sourcePath)
			if err == nil {
				t.Fatalf("expected failure, got success with stdout %q", stdout)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantMessage) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantMessage, stderr)
			}
		})
	}
}

func TestRunCommandRejectsDivisionByZero(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantMessage string
	}{
		{
			name: "int division by zero",
			source: "fn Main() -> Int {\n" +
				"    return 5 / 0\n" +
				"}\n",
			wantMessage: "run failed: runtime error: division by zero",
		},
		{
			name: "float division by zero",
			source: "fn Main() -> Float {\n" +
				"    return 5.0 / 0.0\n" +
				"}\n",
			wantMessage: "run failed: runtime error: division by zero",
		},
		{
			name: "array division by zero",
			source: "fn Main() -> Int[] {\n" +
				"    return [4, 6] / [2, 0]\n" +
				"}\n",
			wantMessage: "run failed: runtime error: division by zero",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sourcePath := writeSourceFile(t, test.name+".oct", test.source)
			stdout, stderr, err := executeCLI("run", sourcePath)
			if err == nil {
				t.Fatalf("expected runtime failure, got success with stdout %q", stdout)
			}
			if stdout != "" {
				t.Fatalf("expected no computed result on stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantMessage) {
				t.Fatalf("expected deterministic division-by-zero error %q, got %q", test.wantMessage, stderr)
			}
		})
	}
}

func TestRunCommandRejectsArrayRuntimeErrors(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantMessage string
	}{
		{
			name: "out of bounds index",
			source: "fn Main() -> Int {\n" +
				"    let x = [1, 2, 3]\n" +
				"    return x[5]\n" +
				"}\n",
			wantMessage: "run failed: runtime error: index 5 out of bounds for array of length 3",
		},
		{
			name: "array size mismatch",
			source: "fn Main() -> Int[] {\n" +
				"    return [1, 2] + [1, 2, 3]\n" +
				"}\n",
			wantMessage: "run failed: runtime error: array length mismatch: 2 vs 3",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sourcePath := writeSourceFile(t, test.name+".oct", test.source)
			stdout, stderr, err := executeCLI("run", sourcePath)
			if err == nil {
				t.Fatalf("expected runtime failure, got success with stdout %q", stdout)
			}
			if stdout != "" {
				t.Fatalf("expected no computed result on stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantMessage) {
				t.Fatalf("expected deterministic array runtime error %q, got %q", test.wantMessage, stderr)
			}
		})
	}
}

func TestRunCommandRejectsInvalidRanges(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantMessage string
	}{
		{
			name: "zero step",
			source: "fn Main() -> Int {\n" +
				"    for i in 1..10 step 0 {\n" +
				"        return i\n" +
				"    }\n" +
				"    return 0\n" +
				"}\n",
			wantMessage: "run failed: function Main: for i: range step must be positive, got 0",
		},
		{
			name: "reverse range",
			source: "fn Main() -> Int {\n" +
				"    for i in 5..0 {\n" +
				"        return i\n" +
				"    }\n" +
				"    return 0\n" +
				"}\n",
			wantMessage: "run failed: runtime error: range start must be less than or equal to end, got 5..0",
		},
		{
			name: "loop variable out of scope",
			source: "fn Main() -> Int {\n" +
				"    for i in 0..1 {\n" +
				"        return i\n" +
				"    }\n" +
				"    return i\n" +
				"}\n",
			wantMessage: "run failed: function Main: undefined variable: i",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sourcePath := writeSourceFile(t, test.name+".oct", test.source)
			stdout, stderr, err := executeCLI("run", sourcePath)
			if err == nil {
				t.Fatalf("expected failure, got success with stdout %q", stdout)
			}
			if stdout != "" {
				t.Fatalf("expected no computed result on stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantMessage) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantMessage, stderr)
			}
		})
	}
}

func TestRunCommandValidatesMainEntryPoint(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantMessage string
	}{
		{
			name: "missing Main",
			source: "fn Add(x: Int, y: Int) -> Int {\n" +
				"    return x + y\n" +
				"}\n",
			wantMessage: "run failed: missing Main function",
		},
		{
			name: "Main has parameters",
			source: "fn Main(x: Int) -> Int {\n" +
				"    return x\n" +
				"}\n",
			wantMessage: "run failed: Main must not have parameters",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sourcePath := writeSourceFile(t, test.name+".oct", test.source)
			stdout, stderr, err := executeCLI("run", sourcePath)
			if err == nil {
				t.Fatalf("expected entrypoint failure, got success with stdout %q", stdout)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantMessage) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantMessage, stderr)
			}
		})
	}
}

func TestBuildCommandSucceedsAfterTypeCheck(t *testing.T) {
	sourcePath := writeSourceFile(t, "example.oct", "fn Main() -> Int {\n    return 0\n}\n")

	stdout, stderr, err := executeCLI("build", sourcePath)
	if err != nil {
		t.Fatalf("build command failed: %v\nstdout:%s\nstderr:%s", err, stdout, stderr)
	}

	artifactPath := sourcePath + ".octbin"
	artifact, err := os.ReadFile(artifactPath)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}

	if string(artifact) != "oct m0 placeholder artifact\n" {
		t.Fatalf("unexpected artifact body %q", artifact)
	}
	if !strings.Contains(stdout, "build succeeded: "+artifactPath) {
		t.Fatalf("expected build success output, got %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestTypeErrorsFailBeforeExecution(t *testing.T) {
	sourcePath := writeSourceFile(t, "bad_type.oct", "fn Main() -> Int {\n    return 1.0\n}\n")

	for _, command := range []string{"run", "build"} {
		t.Run(command, func(t *testing.T) {
			stdout, stderr, err := executeCLI(command, sourcePath)
			if err == nil {
				t.Fatalf("expected failure, got success with stdout %q", stdout)
			}

			expected := command + " failed: function Main: function expects Int, but return is Float"
			if !strings.Contains(stderr, expected) {
				t.Fatalf("expected deterministic type error %q, got %q", expected, stderr)
			}
			if stdout != "" {
				t.Fatalf("expected no stdout on type error, got %q", stdout)
			}
			if command == "build" {
				if _, statErr := os.Stat(sourcePath + ".octbin"); !os.IsNotExist(statErr) {
					t.Fatalf("expected no artifact on type error, stat err = %v", statErr)
				}
			}
		})
	}
}

func TestSyntaxErrorsFailDeterministically(t *testing.T) {
	sourcePath := writeSourceFile(t, "bad.oct", "fn Main() {\n    return 0\n}\n")

	for _, command := range []string{"run", "build"} {
		t.Run(command, func(t *testing.T) {
			stdout, stderr, err := executeCLI(command, sourcePath)
			if err == nil {
				t.Fatalf("expected failure, got success with stdout %q", stdout)
			}

			expected := command + " failed: parse " + sourcePath + ": expected '->' before return type"
			if !strings.Contains(stderr, expected) {
				t.Fatalf("expected deterministic syntax error %q, got %q", expected, stderr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout on parse failure, got %q", stdout)
			}
		})
	}
}

func TestMissingFileFailsDeterministically(t *testing.T) {
	tempDir := t.TempDir()
	missingPath := filepath.Join(tempDir, "missing.oct")

	for _, command := range []string{"run", "build"} {
		t.Run(command, func(t *testing.T) {
			stdout, stderr, err := executeCLI(command, missingPath)
			if err == nil {
				t.Fatalf("expected failure, got success with stdout %q", stdout)
			}

			expected := command + " failed: source file not found: " + missingPath
			if !strings.Contains(stderr, expected) {
				t.Fatalf("expected deterministic error %q, got %q", expected, stderr)
			}
			if stdout != "" {
				t.Fatalf("expected empty stdout on missing file, got %q", stdout)
			}
		})
	}
}

func writeSourceFile(t *testing.T, name string, source string) string {
	t.Helper()
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, name)
	if err := os.WriteFile(sourcePath, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	return sourcePath
}

func executeCLI(command string, sourcePath string) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err := cli.Execute([]string{command, sourcePath}, &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}
