package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
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
			name: "dimensioned int literal",
			source: "fn Main() -> Int<m> {\n" +
				"    return 5m\n" +
				"}\n",
			want: "5m\n",
		},
		{
			name: "dimensioned float literal",
			source: "fn Main() -> Float<s> {\n" +
				"    return 2.5s\n" +
				"}\n",
			want: "2.5s\n",
		},
		{
			name: "dimensioned multiplication",
			source: "fn Main() -> Int<m*s> {\n" +
				"    return 2m * 3s\n" +
				"}\n",
			want: "6m*s\n",
		},
		{
			name: "dimensioned division",
			source: "fn Main() -> Float<m/s> {\n" +
				"    return 10m / 2s\n" +
				"}\n",
			want: "5m/s\n",
		},
		{
			name: "dimensioned sqrt",
			source: "fn Main() -> Float<m> {\n" +
				"    return Sqrt(4m^2)\n" +
				"}\n",
			want: "2m\n",
		},
		{
			name: "dimensioned arrays",
			source: "fn Main() -> Int<m>[] {\n" +
				"    return [1m, 2m, 3m]\n" +
				"}\n",
			want: "[1m, 2m, 3m]\n",
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
		{
			name: "if true returns then branch",
			source: "fn Main() -> Int {\n" +
				"    if true {\n" +
				"        return 1\n" +
				"    }\n" +
				"    return 0\n" +
				"}\n",
			want: "1\n",
		},
		{
			name: "if false returns else branch",
			source: "fn Main() -> Int {\n" +
				"    if false {\n" +
				"        return 1\n" +
				"    } else {\n" +
				"        return 2\n" +
				"    }\n" +
				"}\n",
			want: "2\n",
		},
		{
			name: "switch int expression",
			source: "fn Main() -> Int {\n" +
				"    let x = switch 1 {\n" +
				"        case 0 => 10\n" +
				"        case 1 => 20\n" +
				"        else => 30\n" +
				"    }\n" +
				"    return x\n" +
				"}\n",
			want: "20\n",
		},
		{
			name: "switch string expression",
			source: "fn Main() -> Int {\n" +
				"    let x = switch \"a\" {\n" +
				"        case \"b\" => 1\n" +
				"        case \"a\" => 2\n" +
				"        else => 3\n" +
				"    }\n" +
				"    return x\n" +
				"}\n",
			want: "2\n",
		},
		{
			name: "switch dimensioned result expression",
			source: "fn Main() -> Float<m> {\n" +
				"    return switch 1 {\n" +
				"        case 1 => 1m\n" +
				"        else => 2m\n" +
				"    }\n" +
				"}\n",
			want: "1m\n",
		},
		{
			name: "print int writes output and returns zero",
			source: "fn Main() -> Int {\n" +
				"    Print(1)\n" +
				"    return 0\n" +
				"}\n",
			want: "1\n0\n",
		},
		{
			name: "print float bool and string",
			source: "fn Main() -> Int {\n" +
				"    Print(2.5)\n" +
				"    Print(true)\n" +
				"    Print(\"hi\")\n" +
				"    return 0\n" +
				"}\n",
			want: "2.5\ntrue\nhi\n0\n",
		},
		{
			name: "print arrays and dimensions",
			source: "fn Main() -> Int {\n" +
				"    Print([1, 2, 3])\n" +
				"    Print(5m)\n" +
				"    return 0\n" +
				"}\n",
			want: "[1, 2, 3]\n5m\n0\n",
		},
		{
			name: "while false skips body",
			source: "fn Main() -> Int {\n" +
				"    while false {\n" +
				"        return 1\n" +
				"    }\n" +
				"    return 0\n" +
				"}\n",
			want: "0\n",
		},
		{
			name: "while true returns from body",
			source: "fn Main() -> Int {\n" +
				"    while true {\n" +
				"        return 7\n" +
				"    }\n" +
				"    return 0\n" +
				"}\n",
			want: "7\n",
		},
		{
			name: "while with nested if",
			source: "fn Main() -> Int {\n" +
				"    while true {\n" +
				"        if true {\n" +
				"            return 3\n" +
				"        } else {\n" +
				"            return 4\n" +
				"        }\n" +
				"    }\n" +
				"    return 0\n" +
				"}\n",
			want: "3\n",
		},
		{
			name: "fallible while return",
			source: "fn Main() -> Int ! Error {\n" +
				"    while true {\n" +
				"        return 5\n" +
				"    }\n" +
				"    return error(\"bad\")\n" +
				"}\n",
			want: "5\n",
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

func TestRunAndBuildRejectInvalidM9Programs(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantMessage string
	}{
		{
			name: "if condition must be bool",
			source: `fn Main() -> Int {
    if 1 {
        return 1
    }
    return 0
}
`,
			wantMessage: "if condition must be Bool, got Int",
		},
		{
			name: "switch requires else",
			source: `fn Main() -> Int {
    let x = switch 1 {
        case 1 => 2
    }
    return x
}
`,
			wantMessage: "switch requires else arm",
		},
		{
			name: "while condition must be bool",
			source: `fn Main() -> Int {
    while 1 {
        return 1
    }
    return 0
}
`,
			wantMessage: "while condition must be Bool, got Int",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sourcePath := writeSourceFile(t, test.name+".oct", test.source)
			for _, command := range []string{"run", "build"} {
				t.Run(command, func(t *testing.T) {
					stdout, stderr, err := executeCLI(command, sourcePath)
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

func TestRunCommandSupportsM7Builtins(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "len int array",
			source: `fn Main() -> Int {
    return Len([1, 2, 3])
}
`,
			want: "3\n",
		},
		{
			name: "len bool array variable",
			source: `fn Main() -> Int {
    let xs = [true, false, true, false]
    return Len(xs)
}
`,
			want: "4\n",
		},
		{
			name: "abs int",
			source: `fn Main() -> Int {
    return Abs(0 - 3)
}
`,
			want: "3\n",
		},
		{
			name: "abs float",
			source: `fn Main() -> Float {
    return Abs(0.0 - 3.5)
}
`,
			want: "3.5\n",
		},
		{
			name: "sqrt int result uses stable float formatting",
			source: `fn Main() -> Float {
    return Sqrt(4)
}
`,
			want: "2\n",
		},
		{
			name: "sqrt float",
			source: `fn Main() -> Float {
    return Sqrt(2.25)
}
`,
			want: "1.5\n",
		},
		{
			name: "sin zero",
			source: `fn Main() -> Float {
    return Sin(0)
}
`,
			want: "0\n",
		},
		{
			name: "cos zero",
			source: `fn Main() -> Float {
    return Cos(0)
}
`,
			want: "1\n",
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

func TestRunCommandRejectsInvalidM7Builtins(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantMessage string
	}{
		{
			name: "len wrong type",
			source: `fn Main() -> Int {
    return Len(1)
}
`,
			wantMessage: "run failed: function Main: function 'Len' argument 1 expects Int[], Float[], or Bool[], got Int",
		},
		{
			name: "abs wrong type",
			source: `fn Main() -> Int {
    return Abs(true)
}
`,
			wantMessage: "run failed: function Main: function 'Abs' argument 1 expects Int or Float, got Bool",
		},
		{
			name: "sqrt wrong type",
			source: `fn Main() -> Float {
    return Sqrt(true)
}
`,
			wantMessage: "run failed: function Main: function 'Sqrt' argument 1 expects Int or Float, got Bool",
		},
		{
			name: "cos wrong type",
			source: `fn Main() -> Float {
    return Cos(true)
}
`,
			wantMessage: "run failed: function Main: function 'Cos' argument 1 expects Int or Float, got Bool",
		},
		{
			name: "len arity mismatch",
			source: `fn Main() -> Int {
    return Len()
}
`,
			wantMessage: "run failed: function Main: function 'Len' expects 1 arguments, got 0",
		},
		{
			name: "sin arity mismatch",
			source: `fn Main() -> Float {
    return Sin(1, 2)
}
`,
			wantMessage: "run failed: function Main: function 'Sin' expects 1 arguments, got 2",
		},
		{
			name: "builtin collision",
			source: `fn Len(x: Int) -> Int {
    return x
}

fn Main() -> Int {
    return Len(1)
}
`,
			wantMessage: "run failed: function Len: cannot redeclare built-in function",
		},
		{
			name: "sqrt negative runtime error",
			source: `fn Main() -> Float {
    return Sqrt(0 - 1)
}
`,
			wantMessage: "run failed: runtime error: Sqrt expects non-negative input, got -1",
		},
		{
			name: "print arity mismatch",
			source: `fn Main() -> Int {
    return Print()
}
`,
			wantMessage: "run failed: function Main: function 'Print' expects 1 arguments, got 0",
		},
		{
			name: "print builtin collision",
			source: `fn Print(x: Int) -> Int {
    return x
}

fn Main() -> Int {
    return 0
}
`,
			wantMessage: "run failed: function Print: cannot redeclare built-in function",
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

func TestBuildCommandHandlesM7Builtins(t *testing.T) {
	t.Run("valid program builds", func(t *testing.T) {
		sourcePath := writeSourceFile(t, "m7_valid.oct", "fn Main() -> Float {\n    return Sqrt(4)\n}\n")
		stdout, stderr, err := executeCLI("build", sourcePath)
		if err != nil {
			t.Fatalf("build command failed: %v\nstdout:%s\nstderr:%s", err, stdout, stderr)
		}
		artifactPath := sourcePath + ".octbin"
		if _, err := os.Stat(artifactPath); err != nil {
			t.Fatalf("expected artifact %s: %v", artifactPath, err)
		}
		if !strings.Contains(stdout, "build succeeded: "+artifactPath) {
			t.Fatalf("expected build success output, got %q", stdout)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
	})

	t.Run("invalid program fails before artifact generation", func(t *testing.T) {
		sourcePath := writeSourceFile(t, "m7_invalid.oct", "fn Main() -> Int {\n    return Len(1)\n}\n")
		stdout, stderr, err := executeCLI("build", sourcePath)
		if err == nil {
			t.Fatalf("expected build failure, got success with stdout %q", stdout)
		}
		if stdout != "" {
			t.Fatalf("expected empty stdout, got %q", stdout)
		}
		want := "build failed: function Main: function 'Len' argument 1 expects Int[], Float[], or Bool[], got Int"
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
		if _, statErr := os.Stat(sourcePath + ".octbin"); !os.IsNotExist(statErr) {
			t.Fatalf("expected no artifact on build failure, stat err = %v", statErr)
		}
	})
}

func TestBuildCommandHandlesM12PrintAndWhile(t *testing.T) {
	t.Run("valid program builds", func(t *testing.T) {
		sourcePath := writeSourceFile(t, "m12_valid_build.oct", "fn Main() -> Int {\n    while false {\n        return 1\n    }\n    return Print(1)\n}\n")
		stdout, stderr, err := executeCLI("build", sourcePath)
		if err != nil {
			t.Fatalf("build command failed: %v\nstdout:%s\nstderr:%s", err, stdout, stderr)
		}
		artifactPath := sourcePath + ".octbin"
		if _, err := os.Stat(artifactPath); err != nil {
			t.Fatalf("expected artifact %s: %v", artifactPath, err)
		}
		if !strings.Contains(stdout, "build succeeded: "+artifactPath) {
			t.Fatalf("expected build success output, got %q", stdout)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
	})

	t.Run("invalid program fails before artifact generation", func(t *testing.T) {
		sourcePath := writeSourceFile(t, "m12_invalid_build.oct", "fn Main() -> Int {\n    while 1 {\n        return 1\n    }\n    return 0\n}\n")
		stdout, stderr, err := executeCLI("build", sourcePath)
		if err == nil {
			t.Fatalf("expected build failure, got success with stdout %q", stdout)
		}
		if stdout != "" {
			t.Fatalf("expected empty stdout, got %q", stdout)
		}
		want := "build failed: function Main: while condition must be Bool, got Int"
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
		artifactPath := sourcePath + ".octbin"
		if _, statErr := os.Stat(artifactPath); !os.IsNotExist(statErr) {
			t.Fatalf("expected no artifact on build failure, stat err = %v", statErr)
		}
	})
}

func TestRunCommandHandlesM10PlotBuiltins(t *testing.T) {
	t.Run("line plot writes png", func(t *testing.T) {
		outputPath := filepath.Join(t.TempDir(), "line.png")
		sourcePath := writeSourceFile(t, "m10_line.oct", "fn Main() -> Int {\n    let x = [0.0, 1.0, 2.0]\n    let y = [0.0, 1.0, 4.0]\n    return PlotLine(x, y, "+strconv.Quote(outputPath)+")\n}\n")

		stdout, stderr, err := executeCLI("run", sourcePath)
		if err != nil {
			t.Fatalf("run command failed: %v\nstdout:%s\nstderr:%s", err, stdout, stderr)
		}
		if stdout != "0\n" {
			t.Fatalf("expected stdout %q, got %q", "0\\n", stdout)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
		info, statErr := os.Stat(outputPath)
		if statErr != nil {
			t.Fatalf("expected output png %s: %v", outputPath, statErr)
		}
		if info.Size() == 0 {
			t.Fatalf("expected non-empty output png %s", outputPath)
		}
	})

	t.Run("scatter plot writes png for int arrays", func(t *testing.T) {
		outputPath := filepath.Join(t.TempDir(), "scatter.png")
		sourcePath := writeSourceFile(t, "m10_scatter.oct", "fn Main() -> Int {\n    let x = [0, 1, 2]\n    let y = [0, 1, 4]\n    return PlotScatter(x, y, "+strconv.Quote(outputPath)+")\n}\n")

		stdout, stderr, err := executeCLI("run", sourcePath)
		if err != nil {
			t.Fatalf("run command failed: %v\nstdout:%s\nstderr:%s", err, stdout, stderr)
		}
		if stdout != "0\n" {
			t.Fatalf("expected stdout %q, got %q", "0\\n", stdout)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
		info, statErr := os.Stat(outputPath)
		if statErr != nil {
			t.Fatalf("expected output png %s: %v", outputPath, statErr)
		}
		if info.Size() == 0 {
			t.Fatalf("expected non-empty output png %s", outputPath)
		}
	})

	t.Run("length mismatch fails at runtime", func(t *testing.T) {
		outputPath := filepath.Join(t.TempDir(), "bad.png")
		sourcePath := writeSourceFile(t, "m10_length_mismatch.oct", "fn Main() -> Int {\n    let x = [0.0, 1.0]\n    let y = [0.0, 1.0, 4.0]\n    return PlotLine(x, y, "+strconv.Quote(outputPath)+")\n}\n")

		stdout, stderr, err := executeCLI("run", sourcePath)
		if err == nil {
			t.Fatalf("expected failure, got success with stdout %q", stdout)
		}
		if stdout != "" {
			t.Fatalf("expected empty stdout, got %q", stdout)
		}
		want := "run failed: runtime error: function 'PlotLine' requires x and y arrays of equal length"
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	})

	t.Run("wrong extension fails deterministically", func(t *testing.T) {
		outputPath := filepath.Join(t.TempDir(), "plot.jpg")
		sourcePath := writeSourceFile(t, "m10_wrong_extension.oct", "fn Main() -> Int {\n    let x = [0.0]\n    let y = [1.0]\n    return PlotScatter(x, y, "+strconv.Quote(outputPath)+")\n}\n")

		stdout, stderr, err := executeCLI("run", sourcePath)
		if err == nil {
			t.Fatalf("expected failure, got success with stdout %q", stdout)
		}
		if stdout != "" {
			t.Fatalf("expected empty stdout, got %q", stdout)
		}
		want := "run failed: runtime error: plot output path must end with .png"
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
	})
}

func TestBuildCommandHandlesM10PlotBuiltins(t *testing.T) {
	t.Run("valid plotting program builds", func(t *testing.T) {
		sourcePath := writeSourceFile(t, "m10_valid_build.oct", "fn Main() -> Int {\n    return PlotLine([0.0], [1.0], \"plot.png\")\n}\n")
		stdout, stderr, err := executeCLI("build", sourcePath)
		if err != nil {
			t.Fatalf("build command failed: %v\nstdout:%s\nstderr:%s", err, stdout, stderr)
		}
		artifactPath := sourcePath + ".octbin"
		if _, err := os.Stat(artifactPath); err != nil {
			t.Fatalf("expected artifact %s: %v", artifactPath, err)
		}
		if !strings.Contains(stdout, "build succeeded: "+artifactPath) {
			t.Fatalf("expected build success output, got %q", stdout)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}
	})

	t.Run("invalid plotting program fails before artifact generation", func(t *testing.T) {
		sourcePath := writeSourceFile(t, "m10_invalid_build.oct", "fn Main() -> Int {\n    return PlotLine([1m], [2m], \"bad.png\")\n}\n")
		stdout, stderr, err := executeCLI("build", sourcePath)
		if err == nil {
			t.Fatalf("expected build failure, got success with stdout %q", stdout)
		}
		if stdout != "" {
			t.Fatalf("expected empty stdout, got %q", stdout)
		}
		want := "build failed: function Main: function 'PlotLine' does not accept dimensioned arrays"
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got %q", want, stderr)
		}
		if _, statErr := os.Stat(sourcePath + ".octbin"); !os.IsNotExist(statErr) {
			t.Fatalf("expected no artifact on build failure, stat err = %v", statErr)
		}
	})
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

func TestM13RecordsEnumsAndExprStmtRules(t *testing.T) {
	validTests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "record construction and field access",
			source: "record Point {\n" +
				"    X: Float<m>\n" +
				"    Y: Float<m>\n" +
				"}\n" +
				"fn Main() -> Float<m> {\n" +
				"    let p = Point {\n" +
				"        X: 1m\n" +
				"        Y: 2m\n" +
				"    }\n" +
				"    return p.X\n" +
				"}\n",
			want: "1m\n",
		},
		{
			name: "record field order independence",
			source: "record Point {\n" +
				"    X: Int\n" +
				"    Y: Int\n" +
				"}\n" +
				"fn Main() -> Int {\n" +
				"    let p = Point {\n" +
				"        Y: 2\n" +
				"        X: 1\n" +
				"    }\n" +
				"    return p.Y\n" +
				"}\n",
			want: "2\n",
		},
		{
			name: "basic enum value",
			source: "enum Method {\n" +
				"    Euler\n" +
				"    Rk4\n" +
				"}\n" +
				"fn Main() -> Method {\n" +
				"    return Method.Euler\n" +
				"}\n",
			want: "Method.Euler\n",
		},
		{
			name: "call expression statements remain allowed",
			source: "fn Main() -> Int {\n" +
				"    Print(1)\n" +
				"    return 0\n" +
				"}\n",
			want: "1\n0\n",
		},
		{
			name: "record printing",
			source: "record Point {\n" +
				"    X: Int\n" +
				"    Y: Int\n" +
				"}\n" +
				"fn Main() -> Point {\n" +
				"    return Point {\n" +
				"        X: 1\n" +
				"        Y: 2\n" +
				"    }\n" +
				"}\n",
			want: "Point{X: 1, Y: 2}\n",
		},
	}

	for _, test := range validTests {
		t.Run("valid/"+test.name, func(t *testing.T) {
			sourcePath := writeSourceFile(t, test.name+".oct", test.source)
			stdout, stderr, err := executeCLI("run", sourcePath)
			if err != nil {
				t.Fatalf("run failed: %v\nstdout:%s\nstderr:%s", err, stdout, stderr)
			}
			if stderr != "" {
				t.Fatalf("expected empty stderr, got %q", stderr)
			}
			if stdout != test.want {
				t.Fatalf("expected stdout %q, got %q", test.want, stdout)
			}
		})
	}

	invalidTests := []struct {
		name        string
		source      string
		wantMessage string
	}{
		{
			name: "record missing field",
			source: "record Point { X: Int Y: Int }\n" +
				"fn Main() -> Int {\n" +
				"    let p = Point { X: 1 }\n" +
				"    return 0\n" +
				"}\n",
			wantMessage: "record 'Point' missing field 'Y'",
		},
		{
			name: "record unknown field",
			source: "record Point { X: Int Y: Int }\n" +
				"fn Main() -> Int {\n" +
				"    let p = Point { X: 1 Z: 2 Y: 3 }\n" +
				"    return 0\n" +
				"}\n",
			wantMessage: "record 'Point' has no field 'Z'",
		},
		{
			name: "record duplicate field",
			source: "record Point { X: Int Y: Int }\n" +
				"fn Main() -> Int {\n" +
				"    let p = Point { X: 1 X: 2 Y: 3 }\n" +
				"    return 0\n" +
				"}\n",
			wantMessage: "record 'Point' field 'X' specified more than once",
		},
		{
			name: "field access on non record",
			source: "fn Main() -> Int {\n" +
				"    return 1.X\n" +
				"}\n",
			wantMessage: "field access requires record type, got Int",
		},
		{
			name: "enum unknown variant",
			source: "enum Method { Euler Rk4 }\n" +
				"fn Main() -> Method {\n" +
				"    return Method.Bogus\n" +
				"}\n",
			wantMessage: "enum 'Method' has no variant 'Bogus'",
		},
		{
			name: "enum duplicate variant",
			source: "enum Method { Euler Euler }\n" +
				"fn Main() -> Int { return 0 }\n",
			wantMessage: "enum 'Method' variant 'Euler' specified more than once",
		},
		{
			name: "expr stmt cleanup",
			source: "fn Main() -> Int {\n" +
				"    1 + 2\n" +
				"    return 0\n" +
				"}\n",
			wantMessage: "expression statements must be call expressions",
		},
		{
			name: "enum switch deferred",
			source: "enum Method { Euler Rk4 }\n" +
				"fn Main() -> Int {\n" +
				"    let method = Method.Euler\n" +
				"    return switch method {\n" +
				"        case Method.Euler => 1\n" +
				"        else => 0\n" +
				"    }\n" +
				"}\n",
			wantMessage: "switch case must use int, float, bool, or string literal",
		},
	}

	for _, test := range invalidTests {
		t.Run("invalid/"+test.name, func(t *testing.T) {
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

			buildStdout, buildStderr, buildErr := executeCLI("build", sourcePath)
			if buildErr == nil {
				t.Fatalf("expected build failure, got success with stdout %q", buildStdout)
			}
			if buildStdout != "" {
				t.Fatalf("expected empty build stdout, got %q", buildStdout)
			}
			if !strings.Contains(buildStderr, test.wantMessage) {
				t.Fatalf("expected build stderr to contain %q, got %q", test.wantMessage, buildStderr)
			}
			if _, statErr := os.Stat(sourcePath + ".octbin"); !os.IsNotExist(statErr) {
				t.Fatalf("expected no artifact on build failure, stat err = %v", statErr)
			}
		})
	}
}

func writeSourceFile(t *testing.T, name string, source string) string {
	t.Helper()
	tempDir := t.TempDir()
	sourcePath := filepath.Join(tempDir, name)
	trimmed := strings.TrimSpace(source)
	if strings.HasSuffix(name, ".oct") && !strings.HasPrefix(trimmed, "package ") {
		source = "package Main\n" + source
	}
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

func TestRunCommandRejectsInvalidDimensions(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		wantMessage string
	}{
		{
			name:        "mismatched addition",
			source:      "fn Main() -> Int {\n    return 1m + 2s\n}\n",
			wantMessage: "function Main: cannot add m and s",
		},
		{
			name:        "sin requires dimensionless input",
			source:      "fn Main() -> Float {\n    return Sin(1m)\n}\n",
			wantMessage: "function Main: Sin requires dimensionless input",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			sourcePath := writeSourceFile(t, test.name+".oct", test.source)
			stdout, stderr, err := executeCLI("run", sourcePath)
			if err == nil {
				t.Fatalf("expected run command to fail\nstdout:%s\nstderr:%s", stdout, stderr)
			}
			if !strings.Contains(err.Error(), test.wantMessage) {
				t.Fatalf("expected error to contain %q, got %q", test.wantMessage, err.Error())
			}
		})
	}
}

func TestM14ComparisonsRunAndBuild(t *testing.T) {
	t.Run("valid comparison program", func(t *testing.T) {
		sourcePath := writeSourceFile(t, "m14_valid.oct", "fn Main() -> Int {\n    if 3 >= 2 {\n        while 1 < 2 {\n            return 7\n        }\n    }\n    return 0\n}\n")
		stdout, stderr, err := executeCLI("run", sourcePath)
		if err != nil {
			t.Fatalf("run command failed: %v\nstdout:%s\nstderr:%s", err, stdout, stderr)
		}
		if stdout != "7\n" {
			t.Fatalf("expected stdout %q, got %q", "7\n", stdout)
		}
		if stderr != "" {
			t.Fatalf("expected empty stderr, got %q", stderr)
		}

		buildStdout, buildStderr, buildErr := executeCLI("build", sourcePath)
		if buildErr != nil {
			t.Fatalf("build command failed: %v\nstdout:%s\nstderr:%s", buildErr, buildStdout, buildStderr)
		}
		if !strings.Contains(buildStdout, "build succeeded: "+sourcePath+".octbin") {
			t.Fatalf("expected build success output, got %q", buildStdout)
		}
		if buildStderr != "" {
			t.Fatalf("expected empty build stderr, got %q", buildStderr)
		}
	})

	t.Run("invalid comparison program fails before artifact generation", func(t *testing.T) {
		sourcePath := writeSourceFile(t, "m14_invalid.oct", "fn Main() -> Bool {\n    return [1, 2] == [1, 2]\n}\n")
		stdout, stderr, err := executeCLI("build", sourcePath)
		if err == nil {
			t.Fatalf("expected build failure, got success with stdout %q", stdout)
		}
		if stdout != "" {
			t.Fatalf("expected empty build stdout, got %q", stdout)
		}
		if !strings.Contains(stderr, `operator "==" not defined for Int[] and Int[]`) {
			t.Fatalf("expected stderr to contain comparison error, got %q", stderr)
		}
		if _, statErr := os.Stat(sourcePath + ".octbin"); !os.IsNotExist(statErr) {
			t.Fatalf("expected no artifact on build failure, stat err = %v", statErr)
		}
	})
}

func TestM16VectorsMatricesRunAndBuild(t *testing.T) {
	valid := []struct {
		name   string
		source string
		want   string
	}{
		{"vector literal", "fn Main() -> Vector<Int> { return vector[1, 2, 3] }", "vector[1, 2, 3]\n"},
		{"matrix literal", "fn Main() -> Matrix<Int> { return matrix[[1, 2] [3, 4]] }", "matrix[[1, 2], [3, 4]]\n"},
		{"vector index", "fn Main() -> Int { let v = vector[1, 2, 3] return v[1] }", "2\n"},
		{"matrix index", "fn Main() -> Int { let m = matrix[[1, 2] [3, 4]] return m[1, 0] }", "3\n"},
		{"vector add", "fn Main() -> Vector<Int> { return vector[1, 2, 3] + vector[4, 5, 6] }", "vector[5, 7, 9]\n"},
		{"matrix elementwise multiply", "fn Main() -> Matrix<Int> { return matrix[[1, 2] [3, 4]] * matrix[[2, 3] [4, 5]] }", "matrix[[2, 6], [12, 20]]\n"},
		{"scalar expansion", "fn Main() -> Vector<Int> { return vector[1, 2, 3] + 1 }", "vector[2, 3, 4]\n"},
		{"matrix at vector", "fn Main() -> Vector<Int> { let m = matrix[[1, 2] [3, 4]] let v = vector[10, 20] return m @ v }", "vector[50, 110]\n"},
		{"matrix at matrix", "fn Main() -> Matrix<Int> { let a = matrix[[1, 2] [3, 4]] let b = matrix[[5, 6] [7, 8]] return a @ b }", "matrix[[19, 22], [43, 50]]\n"},
		{"dimensioned matrix vector", "fn Main() -> Vector<Float<m>> { let k = matrix[[1.0m/s, 0.0m/s] [0.0m/s, 2.0m/s]] let x = vector[3.0s, 4.0s] return k @ x }", "vector[3m, 8m]\n"},
	}
	for _, test := range valid {
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

	invalidBuild := []struct {
		name        string
		source      string
		wantMessage string
	}{
		{"ragged matrix", "fn Main() -> Matrix<Int> { return matrix[[1, 2] [3]] }", "matrix rows must all have equal length"},
		{"vector at vector unsupported", "fn Main() -> Int { return vector[1, 2] @ vector[3, 4] }", "operator '@' not defined for Vector<Int> and Vector<Int>"},
		{"vector compare unsupported", "fn Main() -> Bool { return vector[1, 2] == vector[1, 2] }", "operator \"==\" not defined for Vector<Int> and Vector<Int>"},
	}
	for _, test := range invalidBuild {
		t.Run(test.name, func(t *testing.T) {
			sourcePath := writeSourceFile(t, test.name+".oct", test.source)
			stdout, stderr, err := executeCLI("build", sourcePath)
			if err == nil {
				t.Fatalf("expected build failure, got success with stdout %q", stdout)
			}
			if stdout != "" {
				t.Fatalf("expected empty build stdout, got %q", stdout)
			}
			if !strings.Contains(stderr, test.wantMessage) {
				t.Fatalf("expected stderr to contain %q, got %q", test.wantMessage, stderr)
			}
			if _, statErr := os.Stat(sourcePath + ".octbin"); !os.IsNotExist(statErr) {
				t.Fatalf("expected no artifact on build failure, stat err = %v", statErr)
			}
		})
	}

	invalidRun := []struct {
		name        string
		source      string
		wantMessage string
	}{
		{"vector length mismatch", "fn Main() -> Vector<Int> { return vector[1, 2, 3] + vector[1, 2] }", "vector lengths must match"},
		{"matrix shape mismatch", "fn Main() -> Matrix<Int> { return matrix[[1, 2] [3, 4]] + matrix[[1, 2, 3] [4, 5, 6]] }", "matrix shapes must match"},
		{"invalid at dimensions", "fn Main() -> Matrix<Int> { let a = matrix[[1, 2, 3] [4, 5, 6]] let b = matrix[[1, 2] [3, 4]] return a @ b }", "matrix multiplication requires left cols = right rows"},
	}
	for _, test := range invalidRun {
		t.Run(test.name+" runtime", func(t *testing.T) {
			sourcePath := writeSourceFile(t, test.name+"-runtime.oct", test.source)
			stdout, stderr, err := executeCLI("run", sourcePath)
			if err == nil {
				t.Fatalf("expected run failure, got success with stdout %q", stdout)
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
