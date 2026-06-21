package typecheck

import (
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/lex"
	"github.com/yuechen-li-dev/oct/internal/parse"
	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func TestCheckValidPrograms(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			name: "single integer return",
			src:  "fn Main() -> Int { return 1 }",
		},
		{
			name: "let binding and parameters",
			src:  "fn Add(x: Int, y: Int) -> Int { let sum = x + y return sum } fn Main() -> Int { return Add(1, 2) }",
		},
		{
			name: "mixed numeric return",
			src:  "fn Mix(a: Int, b: Float) -> Float { return a + b } fn Main() -> Float { return Mix(1, 2.0) }",
		},
		{
			name: "int array return",
			src:  "fn Main() -> Int[] { return [1, 2, 3] }",
		},
		{
			name: "float array arithmetic",
			src:  "fn Main() -> Float[] { return [1.0, 2.0] + [3.0, 4.0] }",
		},
		{
			name: "array indexing",
			src:  "fn Main() -> Int { let x = [1, 2, 3] return x[1] }",
		},
		{
			name: "nested array return and indexing",
			src:  "fn Grid() -> Int[][] { return [[1, 2], [3, 4]] } fn Main() -> Int { let g = Grid() return g[1][0] }",
		},
		{
			name: "for loop over range",
			src:  "fn Main() -> Int { for i in 0..3 { return i } return 0 }",
		},
		{
			name: "for loop scope shadowing",
			src:  "fn Main() -> Int { let i = 9 for i in 0..1 { return i } return i }",
		},
		{
			name: "for loop with expression bounds and implicit step",
			src:  "fn Main() -> Int { let xs = [1, 2, 3] var sum = 0 for i in 0..Len(xs) { sum = sum + xs[i] } return sum }",
		},
		{
			name: "for loop with expression step",
			src:  "fn Main() -> Int { var sum = 0 let stride = 2 for i in 0..10 step stride { sum = sum + i } return sum }",
		},
		{
			name: "fallible propagation",
			src:  "fn Safe() -> Int ! Error { return 5 } fn Main() -> Int ! Error { let x = Safe()? return x }",
		},
		{
			name: "fallible call expression statement with unwrap",
			src:  "fn Safe() -> Void ! Error { return } fn Main() -> Int { Safe()! return 0 }",
		},
		{
			name: "match fallible result",
			src:  "fn Safe() -> Int ! Error { return 7 } fn Main() -> Int { match Safe() { ok(value) => { return value } err(e) => { return 0 } } }",
		},
		{
			name: "print builtin and while bool condition",
			src:  "fn Main() -> Int { while false { return 1 } return Print(5m) }",
		},
		{
			name: "var declaration and reassignment",
			src:  "fn Main() -> Int { var x = 1 x = 2 return x }",
		},
		{
			name: "while loop with mutable local",
			src:  "fn Main() -> Int { var x = 0 while x < 3 { x = x + 1 } return x }",
		},
		{
			name: "array index assignment",
			src:  "fn Main() -> Int { var x = [1, 2, 3] x[0] = 5 return x[0] }",
		},
		{
			name: "record array index assignment",
			src:  "record P { X: Int } fn Main() -> Int { var x = [P { X: 1 }] x[0] = P { X: 2 } return x[0].X }",
		},
		{
			name: "matrix index assignment",
			src:  "fn Main() -> Int { var m = matrix[[1, 2] [3, 4]] m[0, 1] = 9 return m[0, 1] }",
		},
		{
			name: "named record arrays in signatures",
			src:  "record Ingredient { Name: String Grams: Float } fn First(xs: Ingredient[]) -> Ingredient { return xs[0] } fn Main() -> String { let xs = [Ingredient { Name: \"Flour\" Grams: 500.0 }] return First(xs).Name }",
		},
		{
			name: "named enum arrays in signatures",
			src:  "enum Phase { Solid Liquid Gas } fn Allowed() -> Phase[] { return [Phase.Solid, Phase.Liquid] } fn Main() -> Bool { let phases = Allowed() return phases[1] == Phase.Liquid }",
		},
		{
			name: "record field typed as enum in single-file check path",
			src:  "enum PolicyMode { Safe Fast } record Scenario { Mode: PolicyMode } fn Main() -> PolicyMode { let scenario = Scenario { Mode: PolicyMode.Safe } return scenario.Mode }",
		},
		{
			name: "whole value record reassignment",
			src:  "record Point { X: Int Y: Int } fn Main() -> Point { var p = Point { X: 1 Y: 2 } p = Point { X: 3 Y: 4 } return p }",
		},
		{
			name: "string concatenation",
			src:  `fn Main() -> String { return "hello" + " world" }`,
		},
		{
			name: "Len on string",
			src:  `fn Main() -> Int { return Len("abc") }`,
		},
		{
			name: "Len on bytes",
			src:  `fn Main() -> Int ! Error { let payload = FileReadBytes("payload.bin")? return Len(payload) }`,
		},
		{
			name: "batch expression over array",
			src:  "fn Main() -> Int[] { return batch [1, 2, 3] as item { let next = item + 1 return next } }",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			file := parseSource(t, test.src)
			if err := Check(file); err != nil {
				t.Fatalf("Check returned error: %v", err)
			}
		})
	}
}

func TestArtifactProgressBuiltinTypeErrors(t *testing.T) {
	assertTypeErrorContainsWithArtifactImport(t, "fn Main() -> Void { Artifact.Progress(\"x\", 1) }", "function Main: function 'Artifact.Progress' expects (label: String, current: Int, total: Int); got 2 arguments. Use Artifact.Checkpoint(label) for message-only progress")
	assertTypeErrorContainsWithArtifactImport(t, "fn Main() -> Void { Artifact.Progress(\"x\", \"1\", 3) }", "function Main: function 'Artifact.Progress' expects (label: String, current: Int, total: Int); argument 2 expects Int, got String. Use Artifact.Checkpoint(label) for message-only progress")
	assertTypeErrorContainsWithArtifactImport(t, "fn Main() -> Void { Artifact.Progress(\"x\") }", "function Main: function 'Artifact.Progress' expects (label: String, current: Int, total: Int); got 1 arguments. Use Artifact.Checkpoint(label) for message-only progress")
	assertTypeErrorContainsWithArtifactImport(t, "fn Main() -> Void { Artifact.Checkpoint(1) }", "function Main: function 'Artifact.Checkpoint' argument 1 expects String, got Int")
}

func assertTypeErrorContainsWithArtifactImport(t *testing.T, text string, want string) {
	t.Helper()
	file := parseSource(t, "import Artifact\n"+text)
	chk := checker{
		functions:     make(map[string]functionSignature),
		functionTypes: make(map[string]functionSignature),
		records:       make(map[string]recordInfo),
		enums:         make(map[string]enumInfo),
		flows:         make(map[string]flowSignature),
		typeNames:     make(map[string]struct{}),
		importedPackages: map[string]packageSymbols{
			"Artifact": {},
		},
	}
	err := chk.checkFile(file)
	if err == nil {
		t.Fatal("expected type error")
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error to contain %q, got %q", want, err.Error())
	}
}

func TestCheckRejectsReturnTypeMismatches(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int { return 1.0 }", "function Main: function expects Int, but return is Float")
	assertTypeErrorContains(t, "fn Main() -> Float { return true }", "function Main: function expects Float, but return is Bool")
}

func TestCheckRejectsInvalidOperatorUsage(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int { return true + 1 }", `function Main: operator "+" not defined for Bool and Int`)
	assertTypeErrorContains(t, "fn Main() -> Bool[] { return [true, false] + [true, true] }", `function Main: operator "+" not defined for Bool[] and Bool[]`)
	assertTypeErrorContains(t, `fn Main() -> String { return "value: " + 1 }`, `function Main: operator "+" not defined for String and Int`)
	assertTypeErrorContains(t, `fn Main() -> String { return 1 + "x" }`, `function Main: operator "+" not defined for Int and String`)
	assertTypeErrorContains(t, "fn Main() -> Matrix<Int> { return [[1, 2], [3, 4]] }", "function Main: function expects Matrix<Int>, but return is Int[][]")
}

func TestCheckRejectsUndefinedVariable(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int { return x }", "function Main: undefined variable: x")
}

func TestCheckValidatesLetBindingTypes(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int { let x = 1 return x }")
	if err := Check(file); err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	assertTypeErrorContains(t, "fn Main() -> Int { let x = 1.0 return x }", "function Main: function expects Int, but return is Float")
}

func TestCheckAllowsMixedNumericExpressions(t *testing.T) {
	file := parseSource(t, "fn Main() -> Float { return 1 + 2.0 }")
	if err := Check(file); err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
}

func TestCheckRejectsUnknownTypes(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Number { return 1 }", "function Main: unknown type: Number")
	assertTypeErrorContains(t, "fn Main(x: Number) -> Int { return 1 }", "function Main: parameter x: unknown type: Number")
	assertTypeErrorContains(t, "enum PolicyMode { Safe Fast } enum OtherMode { Turbo } record Scenario { Mode: PolicyMode } fn Main() -> PolicyMode { let scenario = Scenario { Mode: OtherMode.Turbo } return scenario.Mode }", "function Main: let scenario: record 'Scenario' field 'Mode' expects PolicyMode, got OtherMode")
}

func TestCheckRejectsMissingReturnStatement(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int { let x = 1 }", "function Main: missing return statement")
}

func TestCheckValidatesVoidReturnRules(t *testing.T) {
	file := parseSource(t, "fn Main() -> Void { return }")
	if err := Check(file); err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	file = parseSource(t, "fn Main() -> Void { let x = 1 }")
	if err := Check(file); err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	assertTypeErrorContains(t, "fn Main() -> Void { return 1 }", "function Main: Void function cannot return a value")
	assertTypeErrorContains(t, "fn Main(x: Void) -> Int { return 0 }", "function Main: parameter x: Void is only allowed as a function return type")
	assertTypeErrorContains(t, "record R { X: Void } fn Main() -> Int { return 0 }", "record 'R' field 'X': Void is only allowed as a function return type")
	assertTypeErrorContains(t, "fn Main() -> Int { let x = Nop() return 0 } fn Nop() -> Void { return }", "function Main: let x: Void result cannot be used as a value")
}

func TestCheckRejectsMixedTypeArrayLiteral(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int[] { return [1, 2.0] }", "function Main: array literal elements must all have the same type; found Int and Float")
}

func TestCheckRejectsInvalidIndexing(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int { return 1[0] }", "function Main: cannot index non-indexable value of type Int")
	assertTypeErrorContains(t, "fn Main() -> Int { let x = [1, 2, 3] return x[true] }", "function Main: array indexing index must be Int, got Bool")
}

func TestCheckRejectsInvalidRanges(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int { for i in 1.0..10 { return i } return 0 }", "function Main: for i: range start must be Int, got Float")
	assertTypeErrorContains(t, "fn Main() -> Int { for i in 0..10.0 { return i } return 0 }", "function Main: for i: range end must be Int, got Float")
	assertTypeErrorContains(t, "fn Main() -> Int { for i in 0..10 step 1.0 { return i } return 0 }", "function Main: for i: range step must be Int, got Float")
	assertTypeErrorContains(t, "fn Main() -> Int { for i in 1..10 step 0 { return i } return 0 }", "function Main: for i: range step must be positive, got 0")
	assertTypeErrorContains(t, "fn Main() -> Int { for i in 1..10 step (0 - 1) { return i } return 0 }", "function Main: for i: range step must be positive, got -1")
}

func TestCheckRejectsInvalidBatchExpressions(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int[] { return batch 7 as item { return item } }", "function Main: batch input must be an array, got Int")
	assertTypeErrorContains(t, "fn Main() -> Int[] { return batch [1, 2] as item { let x = item + 1 } }", "function Main: batch body must end with 'return <expr>'")
	assertTypeErrorContains(t, "fn Main() -> Int[] { return batch [1, 2] as item { if item > 0 { return item } return item + 1 } }", "function Main: batch body must have exactly one return statement at the end")
}

func TestCheckRejectsLoopVariableOutsideLoop(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int { for i in 0..1 { return i } return i }", "function Main: undefined variable: i")
}

func TestCheckValidatesM18Assignments(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int { let x = 1 x = 2 return x }", "function Main: cannot assign to immutable binding 'x'")
	assertTypeErrorContains(t, "fn Main() -> Int { x = 1 return 0 }", "function Main: unknown binding 'x'")
	assertTypeErrorContains(t, "fn Main() -> Int<m> { var d = 1m d = 2s return d }", "function Main: assignment to d: expected Int<m>, got Int<s>")
	assertTypeErrorContains(t, "fn Main() -> Int { let x = [1, 2, 3] x[0] = 5 return x[0] }", "function Main: cannot assign to immutable binding")
	assertTypeErrorContains(t, "fn Main() -> Int { var x = 1 x[0] = 5 return x }", "function Main: index assignment (`x[i] = ...`) requires an array or matrix target")
	assertTypeErrorContains(t, "fn Main() -> Int { var x = [1, 2, 3] x[0] = 1.5 return x[0] }", "function Main: assigned value type does not match indexed element type")
	assertTypeErrorContains(t, "fn Main() -> Int { var x = [1, 2, 3] x[true] = 1 return x[0] }", "function Main: index assignment indices must be Int")
	assertTypeErrorContains(t, "fn Main() -> Matrix<Int> { var m = matrix[[1, 2] [3, 4]] m[0, 1] = 1.5 return m }", "function Main: assigned value type does not match indexed element type")
	assertTypeErrorContains(t, "fn Main() -> Matrix<Int> { var m = matrix[[1, 2] [3, 4]] m[0] = 9 return m }", "function Main: matrix index assignment (`x[i, j] = ...`) requires exactly 2 indices, got 1")
	assertTypeErrorContains(t, "fn Main() -> Void { PROMETHEUS { return } }", "function Main: PROMETHEUS blocks are only valid inside [Benchmark] functions")
	assertTypeErrorContains(t, "record Board { Flag: Bool } fn Main() -> Int { var board = Board { Flag: false } board.Flag = true return 0 }", "function Main: field assignment (`x.field = ...`) is only valid for `board.field` inside flow state bodies; for records, use immutable update (`x = x with { Field: value }`)")
	assertTypeErrorContains(t, "record Board { Flag: Bool } flow F(board: Board) -> Int { state S { let shadow = board shadow.Flag = true return 0 } }", "function F: inside flow state bodies, field assignment is only supported on `board`")
	assertTypeErrorContains(t, "record Board { Count: Int } flow F(board: Board) -> Int { state S { board.Count = 1.5 return 0 } }", "function F: assignment to board.Count: expected Int, got Float")
}

func TestCheckRejectsInvalidPropagationAndFallibility(t *testing.T) {
	assertTypeErrorContains(t, "fn Fail() -> Int ! Error { return error(\"bad\") } fn Main() -> Int { let x = Fail()? return x }", "function Main: let x: cannot use '?' in infallible function")
	assertTypeErrorContains(t, "fn Main() -> Int { let x = 1? return x }", "function Main: let x: operator '?' requires fallible expression")
	assertTypeErrorContains(t, "fn Bad() -> Int ! MyError { return 1 }", "function Bad: only built-in Error is allowed in fallible signatures")
	assertTypeErrorContains(t, "fn Add(x: Int, y: Int) -> Int { return x + y } fn Main() -> Int { return Add(1) }", "function Main: function 'Add' expects 2 arguments, got 1")
}

func TestCheckRejectsCallTypeMismatchAndUnhandledFallibleUsage(t *testing.T) {
	assertTypeErrorContains(t, "fn Add(x: Int, y: Int) -> Int { return x + y } fn Main() -> Int { return Add(true, 1) }", "function Main: function 'Add' argument 1 expects Int, got Bool")
	assertTypeErrorContains(t, "fn Safe() -> Int ! Error { return 5 } fn Main() -> Int { return Safe() }", "function Main: return value must not be fallible; handle it with '?', '!', or match")
	assertTypeErrorContains(t, "fn Safe() -> Int ! Error { return 5 } fn Main() -> Int { let x = Safe() return 0 }", "function Main: let x: fallible expression must be handled explicitly")
	assertTypeErrorContains(t, "fn Main() -> Int { match 1 { ok(value) => { return value } err(e) => { return 0 } } }", "function Main: match requires fallible expression")
}

func TestCheckRejectsInvalidErrorConstruction(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Error { return error(1) }", "function Main: error() requires a string literal")
}

func TestCheckValidatesM7Builtins(t *testing.T) {
	validPrograms := []string{
		"fn Main() -> Int { return Len([1, 2, 3]) }",
		"fn Main() -> Int { let xs = [true, false, true, false] return Len(xs) }",
		"fn Main() -> Int { return Abs(0 - 3) }",
		"fn Main() -> Float { return Abs(0.0 - 3.5) }",
		"fn Main() -> Float { return Sqrt(4) }",
		"fn Main() -> Float { return Sqrt(2.25) }",
		"fn Main() -> Float { return Sin(0) }",
		"fn Main() -> Float { return Cos(0) }",
		"fn Main() -> Float { return Tan(0) }",
		"fn Main() -> Float { return Asin(1) }",
		"fn Main() -> Float { return Acos(1) }",
		"fn Main() -> Float { return Atan(1) }",
		"fn Main() -> Float { return Atan2(1, 1) }",
		"fn Main() -> Float { return Exp(1) }",
		"fn Main() -> Float { return Sinh(1) }",
		"fn Main() -> Float { return Cosh(1) }",
		"fn Main() -> Float { return Tanh(1) }",
		"fn Main() -> Float { return Ln(1) }",
		"fn Main() -> Float { return Log10(1000) }",
		"fn Main() -> Float { return Sin(90deg) }",
		"fn Main() -> Float { return Pi() }",
		"fn Main() -> Float { return E() }",
		"fn Main() -> String { return FormatFloat(1.25, 2) }",
		"fn Main() -> Float { return Float(3) }",
		"fn Main() -> Float<m> { return Float(3m) }",
		"fn Main() -> Int { return -1 }",
		"fn Main() -> Float<m> { let x = 2m return -x }",
		"fn Main() -> String { return ToString(1) }",
		"fn Main() -> String { return ToString(1.5) }",
		"fn Main() -> String { return ToString(true) }",
		"fn Main() -> Bool { return Contains(\"storefront\", \"front\") }",
		"fn Main() -> Bool { return StartsWith(\"status: ok\", \"status\") }",
		"fn Main() -> Bool { return EndsWith(\"status: ok\", \"ok\") }",
		"fn Main() -> String { return Trim(\"  ready  \") }",
		"fn Main() -> String { return Lower(\"MiXeD\") }",
		"fn Main() -> String { return Upper(\"MiXeD\") }",
		"fn Main() -> String { return Join([\"desk\", \"gear\", \"sale\"], \" / \") }",
		"fn Main() -> Int[] { var xs = [1, 2] xs = Append(xs, 3) return xs }",
		"record P { X: Int } fn Main() -> Int { var xs = [P { X: 1 }] xs = Append(xs, P { X: 2 }) return xs[1].X }",
		"fn Main() -> Int<m>[] { var xs = [1m, 2m] xs = Append(xs, 3m) return xs }",
		"enum Mode { A B } fn Main() -> Int { return WriteOctagon(\"artifact.octagon\", Mode.A) }",
		"record Payload { Name: String Count: Int } fn Main() -> Payload ! Error { return LoadOctagon<Payload>(\"artifact.octagon\")? }",
		"fn Main() -> String ! Error { return JsonNormalize(\"{}\")? }",
	}

	for _, src := range validPrograms {
		file := parseSource(t, src)
		if err := Check(file); err != nil {
			t.Fatalf("Check returned error for %q: %v", src, err)
		}
	}
}

func TestCheckValidatesM10PlotBuiltins(t *testing.T) {
	validPrograms := []string{
		"fn Main() -> Int { return PlotLine([0.0, 1.0], [0.0, 1.0], \"line.png\") }",
		"fn Main() -> Int { return PlotScatter([0, 1, 2], [0, 1, 4], \"scatter.png\") }",
		"fn Main() -> Int { return PlotLine([0, 1, 2], [0.0, 1.0, 4.0], \"mixed.png\") }",
		"fn Main() -> Int ! Error { return PlotRenderLine([0.0, 1.0], [0.0, 1.0], \"line.png\", 640px, 480px, \"Demo\", \"x\", \"y\", \"series\")? }",
		"fn Main() -> Int ! Error { return PlotRenderScatter([0, 1, 2], [0, 1, 4], \"scatter.png\", 400px, 300px, \"Scatter\", \"time\", \"value\", \"samples\")? }",
		"fn Main() -> Int ! Error { return PlotRenderHistogram([0.1, 0.2, 0.3], 5, \"hist.png\", 512px, 384px, \"Histogram\", \"value\", \"count\", \"distribution\")? }",
	}

	for _, src := range validPrograms {
		file := parseSource(t, src)
		if err := Check(file); err != nil {
			t.Fatalf("Check returned error for %q: %v", src, err)
		}
	}
}

func TestCheckRejectsInvalidM10PlotBuiltins(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int { return PlotLine(1, [1], \"x.png\") }", "function Main: function 'PlotLine' argument 1 expects Int[] or Float[], got Int")
	assertTypeErrorContains(t, "fn Main() -> Int { return PlotLine([true], [false], \"x.png\") }", "function Main: function 'PlotLine' argument 1 expects Int[] or Float[], got Bool[]")
	assertTypeErrorContains(t, "fn Main() -> Int { return PlotLine([1m], [2m], \"x.png\") }", "function Main: function 'PlotLine' does not accept dimensioned arrays")
	assertTypeErrorContains(t, "fn Main() -> Int { return PlotLine([1], [2], 3) }", "function Main: function 'PlotLine' argument 3 expects String, got Int")
	assertTypeErrorContains(t, "fn Main() -> Int { return PlotScatter([1], [2]) }", "function Main: function 'PlotScatter' expects 3 arguments, got 2")
	assertTypeErrorContains(t, "fn Main() -> Int ! Error { return PlotRenderLine([1], [2], \"x.png\", 640, 480px, \"t\", \"x\", \"y\", \"s\")? }", "function Main: function 'PlotRenderLine' argument 4 expects Int<px>, got Int")
	assertTypeErrorContains(t, "fn Main() -> Int ! Error { return PlotRenderHistogram([1], 3.5, \"x.png\", 640px, 480px, \"t\", \"x\", \"y\", \"s\")? }", "function Main: function 'PlotRenderHistogram' argument 2 expects Int, got Float")
	assertTypeErrorContains(t, "fn Main() -> Int { return PlotRenderScatter([1], [2], \"x.png\", 640px, 480px, \"t\", \"x\", \"y\", \"s\") }", "function Main: return value must not be fallible; handle it with '?', '!', or match")
	assertTypeErrorContains(t, "fn PlotLine(x: Int[], y: Int[], path: String) -> Int { return 0 } fn Main() -> Int { return 0 }", "function PlotLine: cannot redeclare built-in function")
}

func TestCheckValidatesMx103dImageBuiltins(t *testing.T) {
	validPrograms := []string{
		"fn Main() -> Int<px> ! Error { let h = ImageLoad(\"x.png\")? let _ = ImageSave(h, \"y.jpg\")? return ImageWidth(h) + ImageHeight(h) }",
		"fn Main() -> String ! Error { let h = ImageLoad(\"x.jpeg\")? return ImageFormat(h) }",
		"fn Main() -> Int ! Error { let h = ImageLoad(\"x.png\")? return h }",
	}
	for _, src := range validPrograms {
		file := parseSource(t, src)
		if err := Check(file); err != nil {
			t.Fatalf("Check returned error for %q: %v", src, err)
		}
	}
}

func TestCheckRejectsInvalidMx103dImageBuiltins(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int ! Error { return ImageLoad(1)? }", "function Main: function 'ImageLoad' argument 1 expects String, got Int")
	assertTypeErrorContains(t, "fn Main() -> Int ! Error { return ImageSave(\"handle\", \"x.png\")? }", "function Main: function 'ImageSave' argument 1 expects Int, got String")
	assertTypeErrorContains(t, "fn Main() -> Int { return ImageWidth(\"h\") }", "function Main: function 'ImageWidth' argument 1 expects Int, got String")
	assertTypeErrorContains(t, "fn Main() -> Int { return ImageHeight(1, 2) }", "function Main: function 'ImageHeight' expects 1 argument, got 2")
	assertTypeErrorContains(t, "fn Main() -> String { return ImageFormat(ImageLoad(\"x.png\")) }", "function Main: fallible expression must be handled explicitly")
	assertTypeErrorContains(t, "fn Main() -> Int ! Error { let h = ImageLoad(\"x.png\")? return ImageWidth(h) }", "function Main: function expects Int or Error, but return is Int<px>")
	assertTypeErrorContains(t, "fn Main() -> Int<px> ! Error { let h = ImageLoad(\"x.png\")? return h }", "function Main: function expects Int<px> or Error, but return is Int")
	assertTypeErrorContains(t, "fn ImageLoad(path: String) -> Int { return 0 } fn Main() -> Int { return 0 }", "function ImageLoad: cannot redeclare built-in function")
}

func TestCheckValidatesM105PdfBuiltins(t *testing.T) {
	validPrograms := []string{
		"fn Main() -> Int ! Error { let p = PdfNewPage(640px, 480px)? let _ = PdfDrawText(p, 24px, 30px, \"Hello\")? let _ = PdfSave(p, \"out.pdf\")? return 0 }",
		"fn Main() -> Int ! Error { let p = PdfNewPage(800px, 600px)? let img = ImageLoad(\"plot.png\")? let _ = PdfDrawImage(p, img, 8px, 12px)? let _ = PdfDrawImageSized(p, img, 16px, 18px, 200px, 100px)? return 0 }",
		"fn Main() -> Int ! Error { let p = PdfNewPage(200px, 100px)? let _ = PdfDrawTextStyled(p, 8px, 10px, \"A\", 20px, 255, 0, 64)? return 0 }",
	}
	for _, src := range validPrograms {
		file := parseSource(t, src)
		if err := Check(file); err != nil {
			t.Fatalf("Check returned error for %q: %v", src, err)
		}
	}
}

func TestCheckRejectsInvalidM105PdfBuiltins(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int ! Error { return PdfNewPage(640, 480px)? }", "function Main: function 'PdfNewPage' argument 1 expects Int<px>, got Int")
	assertTypeErrorContains(t, "fn Main() -> Int ! Error { return PdfDrawText(1, 0px, 0px, 5)? }", "function Main: function 'PdfDrawText' argument 4 expects String, got Int")
	assertTypeErrorContains(t, "fn Main() -> Int ! Error { return PdfDrawImage(1, \"img\", 0px, 0px)? }", "function Main: function 'PdfDrawImage' argument 2 expects Int, got String")
	assertTypeErrorContains(t, "fn Main() -> Int ! Error { return PdfDrawImageSized(1, 2, 0px, 0px, 100, 100px)? }", "function Main: function 'PdfDrawImageSized' argument 5 expects Int<px>, got Int")
	assertTypeErrorContains(t, "fn Main() -> Int ! Error { return PdfDrawImageBytes(1, \"not bytes\", \"png\", 0px, 0px)? }", "function Main: function 'PdfDrawImageBytes' argument 2 expects Bytes, got String")
	assertTypeErrorContains(t, "fn Main() -> Int ! Error { return PdfDrawTextStyled(1, 0px, 0px, \"x\", 12px, 0, 0)? }", "function Main: function 'PdfDrawTextStyled' expects 8 arguments, got 7")
	assertTypeErrorContains(t, "fn Main() -> Int { return PdfSave(1, \"out.pdf\") }", "function Main: return value must not be fallible; handle it with '?', '!', or match")
}

func TestCheckRejectsInvalidM7Builtins(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int { return Len(1) }", "function Main: function 'Len' argument 1 expects String, Bytes, array type, or Vector, got Int")
	assertTypeErrorContains(t, "fn Main() -> Int { return Abs(true) }", "function Main: function 'Abs' argument 1 expects Int, Float, or Complex, got Bool")
	assertTypeErrorContains(t, "fn Main() -> Float { return Sqrt(true) }", "function Main: function 'Sqrt' argument 1 expects Int or Float, got Bool")
	assertTypeErrorContains(t, "fn Main() -> Float { return Cos(true) }", "function Main: function 'Cos' argument 1 expects Int or Float, got Bool")
	assertTypeErrorContains(t, "fn Main() -> Float { return Pi(1) }", "function Main: function 'Pi' expects 0 arguments, got 1")
	assertTypeErrorContains(t, "fn Main() -> Float { return Atan2(1) }", "function Main: function 'Atan2' expects 2 arguments, got 1")
	assertTypeErrorContains(t, "fn Main() -> Float { return Atan2(1m, 1) }", "function Main: Atan2 requires dimensionless input")
	assertTypeErrorContains(t, "fn Main() -> Float { return Atan2(1, 1s) }", "function Main: Atan2 requires dimensionless input")
	assertTypeErrorContains(t, "fn Main() -> String { return FormatFloat(1, 2) }", "function Main: function 'FormatFloat' argument 1 expects Float, got Int")
	assertTypeErrorContains(t, "fn Main() -> String { return FormatFloat(1.5, 2.0) }", "function Main: function 'FormatFloat' argument 2 expects Int, got Float")
	assertTypeErrorContains(t, "fn Main() -> Float { return Float(1.5) }", "function Main: function 'Float' argument 1 expects Int, got Float")
	assertTypeErrorContains(t, "fn Main() -> Float { return Float(true) }", "function Main: function 'Float' argument 1 expects Int, got Bool")
	assertTypeErrorContains(t, "fn Main() -> Bool { return -true }", "function Main: operator '-' requires Int or Float operand")
	assertTypeErrorContains(t, "fn Main() -> String { return ToString(\"x\") }", "function Main: function 'ToString' argument 1 expects Int, Float, or Bool, got String")
	assertTypeErrorContains(t, "fn Main() -> Bool { return Contains(1, \"x\") }", "function Main: function 'Contains' argument 1 expects String, got Int")
	assertTypeErrorContains(t, "fn Main() -> Bool { return StartsWith(\"x\", 1) }", "function Main: function 'StartsWith' argument 2 expects String, got Int")
	assertTypeErrorContains(t, "fn Main() -> String { return Trim(1) }", "function Main: function 'Trim' argument 1 expects String, got Int")
	assertTypeErrorContains(t, "fn Main() -> String { return Join([1, 2], \",\") }", "function Main: function 'Join' argument 1 expects String[], got Int[]")
	assertTypeErrorContains(t, "fn Main() -> String { return Join([\"x\"], 1) }", "function Main: function 'Join' argument 2 expects String, got Int")
	assertTypeErrorContains(t, "fn Main() -> Float { return Sinh(3m) }", "function Main: Sinh requires dimensionless input")
	assertTypeErrorContains(t, "fn Main() -> Float { return Cosh(2kg) }", "function Main: Cosh requires dimensionless input")
	assertTypeErrorContains(t, "fn Main() -> Float { return Tanh(5A) }", "function Main: Tanh requires dimensionless input")
	assertTypeErrorContains(t, "fn Main() -> Int { return Len() }", "function Main: function 'Len' expects 1 argument, got 0")
	assertTypeErrorContains(t, "fn Main() -> Float { return Sin(1, 2) }", "function Main: function 'Sin' expects 1 argument, got 2")
	assertTypeErrorContains(t, "fn Main() -> Int[] { return Append(1, 2) }", "function Main: Append requires array as first argument")
	assertTypeErrorContains(t, "fn Main() -> Int[] { var xs = [1, 2] return Append(xs, 3.0) }", "function Main: Append element type must match array element type")
	assertTypeErrorContains(t, "fn Main() -> Int[] { return Append([1, 2], 3, 4) }", "function Main: function 'Append' expects 2 arguments, got 3")
	assertTypeErrorContains(t, "fn Main() -> Int { return WriteOctagon(\"artifact.txt\", 1) }", "function Main: WriteOctagon path must end with .octagon")
	assertTypeErrorContains(t, "fn Main() -> Int { return WriteOctagon(1, 2) }", "function Main: function 'WriteOctagon' argument 1 expects String, got Int")
	assertTypeErrorContains(t, "fn Main() -> Int { return WriteOctagon(\"artifact.octagon\", vector[1, 2]) }", "function Main: function 'WriteOctagon' argument 2 expects .octagon-representable value, got Vector<Int>")
	assertTypeErrorContains(t, "fn Main() -> Int { return WriteOctagon(\"a.octagon\") }", "function Main: function 'WriteOctagon' expects 2 arguments, got 1")
	assertTypeErrorContains(t, "fn Main() -> UI { return UIButton(\"go\", \"evt\") }", "function Main: function 'UIButton' expects 3 arguments, got 2")
	assertTypeErrorContains(t, "fn Main() -> UI { return UIButton(\"go\", \"evt\", \"yes\") }", "function Main: function 'UIButton' argument 3 expects Bool, got String")
	assertTypeErrorContains(t, "fn Main() -> UI { return UIPlaceAbsolute(0.0, 0.0, 10.0, Text(\"x\")) } fn Text(content: String) -> UI { return UIText(content) }", "function Main: function 'UIPlaceAbsolute' expects 6 arguments, got 4")
	assertTypeErrorContains(t, "fn Main() -> UI { return UIPlaceAbsolute(0.1ui, 0.0px, 10.0px, 10.0px, UIText(\"x\"), 0) }", "function Main: function 'UIPlaceAbsolute' argument 1 expects Float<px>, got Float<ui>")
	assertTypeErrorContains(t, "fn Main() -> UI { return UIPlaceAnchored(1.0px, 0.0ui, 1.0ui, 1.0ui, UIText(\"x\"), 0) }", "function Main: function 'UIPlaceAnchored' argument 1 expects Float<ui>, got Float<px>")
	assertTypeErrorContains(t, "fn Main() -> UI { return UIPlaceAnchored(0.0ui, 0.0ui, 1.0ui, 1.0ui, \"x\", 0) }", "function Main: function 'UIPlaceAnchored' argument 5 expects UI, got String")
	assertTypeErrorContains(t, "fn Main() -> UI { return UIPlaceAbsolute(0.0px, 0.0px, 1.0px, 1.0px, UIText(\"x\"), 0.1) }", "function Main: function 'UIPlaceAbsolute' argument 6 expects Int, got Float")
	assertTypeErrorContains(t, "fn Main() -> UI { return UISpacer(1) }", "function Main: function 'UISpacer' expects 0 arguments, got 1")
	assertTypeErrorContains(t, "fn Main() -> Int ! Error { return LoadOctagon(\"a.octagon\")? }", "function Main: function 'LoadOctagon' expects 1 type argument, got 0")
	assertTypeErrorContains(t, "fn Main() -> Int ! Error { return LoadOctagon<Int>(1)? }", "function Main: function 'LoadOctagon' argument 1 expects String, got Int")
	assertTypeErrorContains(t, "fn Main() -> Int ! Error { return LoadOctagon<Int>(\"a.txt\")? }", "function Main: LoadOctagon path must end with .octagon")
	assertTypeErrorContains(t, "fn Main() -> Int ! Error { return LoadOctagon<Vector<Int>>(\"a.octagon\")? }", "function Main: function 'LoadOctagon' type argument expects .octagon-representable type, got Vector<Int>")
	assertTypeErrorContains(t, "fn Main() -> String ! Error { return JsonNormalize(1)? }", "function Main: function 'JsonNormalize' argument 1 expects String, got Int")
	assertTypeErrorContains(t, "fn Main() -> String ! Error { return JsonNormalize(\"{}\", \"x\")? }", "function Main: function 'JsonNormalize' expects 1 argument, got 2")
	assertTypeErrorContains(t, "fn Len(x: Int) -> Int { return x } fn Main() -> Int { return Len(1) }", "function Len: cannot redeclare built-in function")
	assertTypeErrorContains(t, "fn Append(xs: Int[], x: Int) -> Int[] { return xs } fn Main() -> Int { return 0 }", "function Append: cannot redeclare built-in function")
	assertTypeErrorContains(t, "fn WriteOctagon(path: String, value: Int) -> Int { return 0 } fn Main() -> Int { return 0 }", "function WriteOctagon: cannot redeclare built-in function")
}

func TestCheckFileBytesBuiltins(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int ! Error { let data = FileReadBytes(\"p.bin\")? return data + data }", `function Main: operator "+" not defined for Bytes and Bytes`)
	assertTypeErrorContains(t, "fn Main() -> Int ! Error { let _ = FileWriteBytes(\"p.bin\", [1,2,3])? return 0 }", "function Main: let _: function 'FileWriteBytes' argument 2 expects Bytes, got Int[]")
}

func TestCheckValidatesM12PrintAndWhile(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int { return Print() }", "function Main: function 'Print' expects 1 argument, got 0")
	assertTypeErrorContains(t, "fn Main() -> Int { return Print(1, 2) }", "function Main: function 'Print' expects 1 argument, got 2")
	assertTypeErrorContains(t, "fn Main() -> Int { while 1 { return 1 } return 0 }", "function Main: while condition must be Bool, got Int")
	assertTypeErrorContains(t, "fn Print(x: Int) -> Int { return x } fn Main() -> Int { return 0 }", "function Print: cannot redeclare built-in function")
}

func TestCheckValidatesM20IfExpressions(t *testing.T) {
	validPrograms := []string{
		"fn Main() -> Int { let x = if true { 1 } else { 2 } return x }",
		"fn Main() -> Int { return (if true { 1 } else { 2 }) + 3 }",
		"fn Main() -> Int<m> { return if true { 1m } else { 2m } }",
		"record Point { X: Int Y: Int } fn Main() -> Point { return if true { Point { X: 1 Y: 2 } } else { Point { X: 3 Y: 4 } } }",
	}
	for _, src := range validPrograms {
		file := parseSource(t, src)
		if err := Check(file); err != nil {
			t.Fatalf("Check returned error for %q: %v", src, err)
		}
	}

	assertTypeErrorContains(t, "fn Main() -> Int { return if 1 { 1 } else { 2 } }", "if expression condition must be Bool, got Int")
	assertTypeErrorContains(t, "fn Main() -> Int<m> { return if true { 1m } else { 2s } }", "if expression branches must have matching types")
	assertTypeErrorContains(t, "fn Main() -> Int { return if true { 1 } else { 2.0 } }", "if expression branches must have matching types")
}

func parseSource(t *testing.T, text string) ast.File {
	t.Helper()
	if !strings.HasPrefix(strings.TrimSpace(text), "package ") {
		text = "package Main\n" + text
	}

	lexed, err := lex.Analyze(source.File{Path: "example.oct", Text: text})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	file, err := parse.BuildFile(lexed)
	if err != nil {
		t.Fatalf("BuildFile returned error: %v", err)
	}
	return file
}

func assertTypeErrorContains(t *testing.T, text string, want string) {
	t.Helper()

	file := parseSource(t, text)
	err := Check(file)
	if err == nil {
		t.Fatal("expected type error")
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error to contain %q, got %q", want, err.Error())
	}
}

func TestCheckValidatesM8Dimensions(t *testing.T) {
	validPrograms := []string{
		"fn Main() -> Int<m> { return 5m }",
		"fn Main() -> Float<s> { return 2.5s }",
		"fn Main() -> Int<m> { return 1m + 2m }",
		"fn Main() -> Int<m*s> { return 2m * 3s }",
		"fn Main() -> Float<m/s> { return 10m / 2s }",
		"fn Main() -> Int { return 10m / 2m }",
		"fn Speed(distance: Float<m>, time: Float<s>) -> Float<m/s> { return distance / time } fn Main() -> Float<m/s> { return Speed(10m, 2s) }",
		"fn Main() -> Float<m> { return Abs(0m - 3m) }",
		"fn Main() -> Float<m> { return Sqrt(4m^2) }",
		"fn Main() -> Int<m>[] { return [1m, 2m, 3m] }",
	}

	for _, src := range validPrograms {
		file := parseSource(t, src)
		if err := Check(file); err != nil {
			t.Fatalf("Check returned error for %q: %v", src, err)
		}
	}
}

func TestCheckRejectsInvalidM8Dimensions(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int { return 1m + 2s }", "function Main: cannot add m and s")
	assertTypeErrorContains(t, "fn Main() -> Int<m> { return 1 + 2m }", "function Main: cannot add dimensionless and m")
	assertTypeErrorContains(t, "fn Speed(distance: Float<m>, time: Float<s>) -> Float<m/s> { return distance / time } fn Main() -> Float<m/s> { return Speed(10s, 2m) }", "function Main: function 'Speed' argument 1 expects Float<m>, got Int<s>")
	assertTypeErrorContains(t, "fn Main() -> Float { return Sin(1m) }", "function Main: Sin requires dimensionless input")
	assertTypeErrorContains(t, "fn Main() -> Float { return Ln(2m) }", "function Main: Ln requires dimensionless input")
	assertTypeErrorContains(t, "fn Main() -> Float { return Exp(5s) }", "function Main: Exp requires dimensionless input")
	assertTypeErrorContains(t, "fn Main() -> Float { return Sqrt(4m) }", "function Main: Sqrt requires even dimension exponents")
	assertTypeErrorContains(t, "fn Main() -> Int<m>[] { return [1m, 2s] }", "function Main: array literal elements must all have the same type; found Int<m> and Int<s>")
	assertTypeErrorContains(t, "fn Main() -> Bool<m> { return true }", "function Main: invalid dimension-qualified type syntax: Bool<m>")
	assertTypeErrorContains(t, "fn Main() -> Float<m/s> { return 1m }", "function Main: function expects Float<m/s>, but return is Int<m>")
}

func TestCheckValidatesM9Conditionals(t *testing.T) {
	validPrograms := []string{
		"fn Main() -> Int { if true { return 1 } return 0 }",
		"fn Main() -> Int { if false { return 1 } else { return 2 } }",
		"fn Main() -> Int { let x = switch 1 { case 0 => 10 case 1 => 20 else => 30 } return x }",
		"fn Main() -> Int { let x = switch { case true => 10 else => 20 } return x }",
		"fn Main() -> Int { let x = switch true { case true => 1 else => 0 } return x }",
		"fn Main() -> Int { let x = switch \"a\" { case \"b\" => 1 case \"a\" => 2 else => 3 } return x }",
		"fn Main() -> Float<m> { return switch 1 { case 1 => 1m else => 2m } }",
		"fn Main() -> Int<m> { return switch { case true => 1m else => 2m } }",
		"enum Method { Euler Rk4 } fn Main() -> Int { let m = Method.Euler return switch m { case Method.Euler => 1 case Method.Rk4 => 4 } }",
		"enum Method { Euler Rk4 } fn Main() -> Int { let m = Method.Euler return switch m { case Method.Euler => 1 else => 0 } }",
		"fn Main() -> Int ! Error { if true { return 1 } else { return error(\"bad\") } }",
		"fn Safe() -> Int ! Error { return 5 } fn Main() -> Int ! Error { let x = switch 1 { case 1 => Safe()? else => 0 } return x }",
	}

	for _, src := range validPrograms {
		file := parseSource(t, src)
		if err := Check(file); err != nil {
			t.Fatalf("Check returned error for %q: %v", src, err)
		}
	}
}

func TestCheckValidatesM14Comparisons(t *testing.T) {
	validPrograms := []string{
		"fn Main() -> Bool { return 1 == 1 }",
		"fn Main() -> Bool { return 1 < 2.5 }",
		"fn Main() -> Bool { return 1 + 2 < 4 }",
		"fn Main() -> Bool { return 1m == 1m }",
		"fn Main() -> Bool { return 1m < 2m }",
		"fn Main() -> Bool { return true != false }",
		"fn Main() -> Bool { return \"a\" == \"a\" }",
		"enum Method { Euler Rk4 } fn Main() -> Bool { return Method.Euler != Method.Rk4 }",
		"fn Main() -> Int { while 1 < 2 { return 7 } return 0 }",
		"fn Main() -> Int { if 3 >= 2 { return 1 } else { return 0 } }",
	}
	for _, src := range validPrograms {
		file := parseSource(t, src)
		if err := Check(file); err != nil {
			t.Fatalf("Check returned error for %q: %v", src, err)
		}
	}
}

func TestCheckValidatesM17LogicalOperators(t *testing.T) {
	validPrograms := []string{
		"fn Main() -> Bool { return true and true }",
		"fn Main() -> Bool { return false or true }",
		"fn Main() -> Bool { return not false }",
		"fn Main() -> Bool { return 1 < 2 and 3 < 4 }",
		"fn Main() -> Bool { return true or false and false }",
		"fn Main() -> Bool { return not 1 == 2 }",
		"fn Main() -> Int { if true and not false { return 1 } else { return 0 } }",
		"fn Main() -> Int { while true and not false { return 7 } return 0 }",
	}
	for _, src := range validPrograms {
		file := parseSource(t, src)
		if err := Check(file); err != nil {
			t.Fatalf("Check returned error for %q: %v", src, err)
		}
	}
}

func TestCheckRejectsInvalidM17LogicalOperators(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Bool { return 1 and 2 }", "operator 'and' requires Bool operands")
	assertTypeErrorContains(t, "fn Main() -> Bool { return true or 1 }", "operator 'or' requires Bool operands")
	assertTypeErrorContains(t, "fn Main() -> Bool { return not 1 }", "operator 'not' requires Bool operand")
	assertTypeErrorContains(t, "fn Main() -> Int { if 1 { return 1 } return 0 }", "if condition must be Bool, got Int")
}

func TestCheckRejectsInvalidM14Comparisons(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Bool { return 1m == 1s }", "cannot compare Int<m> and Int<s>")
	assertTypeErrorContains(t, "fn Main() -> Bool { return 1m < 2s }", "cannot compare Int<m> and Int<s>")
	assertTypeErrorContains(t, "fn Main() -> Bool { return true < false }", `operator "<" not defined for Bool`)
	assertTypeErrorContains(t, "fn Main() -> Bool { return \"a\" < \"b\" }", `operator "<" not defined for String`)
	assertTypeErrorContains(t, "enum A { X } enum B { X } fn Main() -> Bool { return A.X == B.X }", `operator "==" requires matching enum types`)
	assertTypeErrorContains(t, "enum Method { Euler Rk4 } fn Main() -> Bool { return Method.Euler < Method.Rk4 }", `operator "<" not defined for Method`)
	assertTypeErrorContains(t, "fn Main() -> Bool { return [1, 2] == [1, 2] }", `operator "==" not defined for Int[] and Int[]`)
	assertTypeErrorContains(t, "record Point { X: Int Y: Int } fn Main() -> Bool { let p1 = Point { X: 1 Y: 2 } let p2 = Point { X: 1 Y: 2 } return p1 == p2 }", `operator "==" not defined for Point and Point`)
	assertTypeErrorContains(t, "fn Main() -> Bool { return 0 < 1 < 2 }", "chained comparisons are not supported")
}

func TestCheckRejectsInvalidM9Conditionals(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int { if 1 { return 1 } return 0 }", "function Main: if condition must be Bool, got Int")
	assertTypeErrorContains(t, "fn Main() -> Int { return switch { case 1 => 1 else => 2 } }", "function Main: condition switch case must be Bool")
	assertTypeErrorContains(t, "fn Main() -> Int { return switch { case true => 1 } }", "function Main: condition switch requires else arm")
	assertTypeErrorContains(t, "fn Main() -> Int { return switch { case true => 1 else => 2.0 } }", "function Main: condition switch result arms must have matching types")
	assertTypeErrorContains(t, "fn Main() -> Int<m> { return switch { case true => 1m else => 2s } }", "function Main: condition switch result arms must have matching dimensions")
	assertTypeErrorContains(t, "fn Main() -> Int { let x = switch 1 { case true => 2 else => 3 } return x }", "function Main: let x: switch case 1: case type Bool does not match subject type Int")
	assertTypeErrorContains(t, "fn Main() -> Int { let x = switch 1 { case 1 => 2 else => 3.0 } return x }", "function Main: let x: switch else: result type Float does not match Int")
	assertTypeErrorContains(t, "fn Main() -> Float<m> { return switch 1 { case 1 => 1m else => 2s } }", "function Main: switch else: result type Int<s> does not match Int<m>")
	assertTypeErrorContains(t, "fn Main() -> Int { let x = switch [1, 2] { else => 0 } return x }", "function Main: let x: switch subject type Int[] is not supported")
	assertTypeErrorContains(t, "fn Main() -> Int { let x = switch 1m { case 1m => 2 else => 3 } return x }", "function Main: let x: switch subject type Int<m> is not supported")
	assertTypeErrorContains(t, "enum Method { Euler Rk4 } fn Main() -> Int { let m = Method.Euler return switch m { case Method.Unknown => 1 else => 0 } }", "enum 'Method' has no variant 'Unknown'")
	assertTypeErrorContains(t, "enum Method { Euler Rk4 } enum OtherEnum { Value } fn Main() -> Int { let m = Method.Euler return switch m { case OtherEnum.Value => 1 else => 0 } }", "case type OtherEnum does not match subject type Method")
	assertTypeErrorContains(t, "enum Method { Euler Rk4 } fn Main() -> Int { let m = Method.Euler return switch m { case Method.Euler => 1 case Method.Euler => 2 else => 0 } }", "duplicate case 'Method.Euler'")
	assertTypeErrorContains(t, "enum Method { Euler Rk4 } fn Main() -> Int { let m = Method.Euler return switch m { case Method.Euler => 1 } }", "non-exhaustive switch over enum 'Method'; missing cases and no else")
	assertTypeErrorContains(t, "enum Method { Euler Rk4 } fn Main() -> Int { let m = Method.Euler return switch m { case Method.Euler => 1 case 1 => 2 else => 0 } }", "mixing enum and non-enum case labels is not allowed")
}

func TestCheckValidatesM23eDecisionLadderShape(t *testing.T) {
	assertTypeErrorContains(t,
		"fn Main() -> Int { if true { return 1 } else { if false { return 2 } else { return 3 } } }",
		"function Main: nested decision-ladder `if/else` is not allowed in Oct by design. Use a condition-switch expression instead:")

	validPrograms := []string{
		"fn Main() -> Int { if true { if true { return 1 } return 2 } return 3 }",
		"fn Main() -> Int { var x = 0 while x < 1 { if true { if false { return 2 } x = 1 } } return x }",
		"fn Main() -> Int { return switch { case true => 1 case false => 2 else => 3 } }",
	}

	for _, src := range validPrograms {
		file := parseSource(t, src)
		if err := Check(file); err != nil {
			t.Fatalf("Check returned error for %q: %v", src, err)
		}
	}
}

func TestCheckValidatesM16VectorsMatrices(t *testing.T) {
	validPrograms := []string{
		"fn Main() -> Vector<Int> { return vector[1, 2, 3] }",
		"fn Main() -> Matrix<Int> { return matrix[[1, 2] [3, 4]] }",
		"fn Main() -> Int { let m = matrix[[1, 2] [3, 4]] return m[1, 0] }",
		"fn Main() -> Vector<Int> { let m = matrix[[1, 2] [3, 4]] let v = vector[10, 20] return m @ v }",
		"fn Main() -> Matrix<Int> { let a = matrix[[1, 2] [3, 4]] let b = matrix[[5, 6] [7, 8]] return a @ b }",
		"fn Main() -> Vector<Int> { let v = vector[10, 20] let m = matrix[[1, 2] [3, 4]] return v @ m }",
		"fn Main() -> Int { return vector[1, 2] @ vector[3, 4] }",
		"fn Main() -> Vector<Int> { return vector[1, 2, 3] + 1 }",
	}
	for _, src := range validPrograms {
		file := parseSource(t, src)
		if err := Check(file); err != nil {
			t.Fatalf("Check returned error for %q: %v", src, err)
		}
	}
}

func TestCheckRejectsInvalidM16VectorsMatrices(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Matrix<Int> { return matrix[[1, 2] [3]] }", "matrix rows must all have equal length")
	assertTypeErrorContains(t, "fn Main() -> Vector<Int> { return vector[1, 2.0] }", "Vector literals require homogeneous element type")
	assertTypeErrorContains(t, "fn Main() -> Vector<Int> { return [1, 2] }", "function Main: function expects Vector<Int>, but return is Int[]")
	assertTypeErrorContains(t, "fn Main() -> Matrix<Int> { return [1, 2] }", "function Main: function expects Matrix<Int>, but return is Int[]")
	assertTypeErrorContains(t, "fn Main() -> Matrix<Int> { return [[1, 2] [3, 4]] }", "array indexing requires exactly 1 index, got 2")
	assertTypeErrorContains(t, "fn Main() -> Vector<Int> { return vector[1, 2] == vector[1, 2] }", "operator \"==\" not defined for Vector<Int> and Vector<Int>")
	assertTypeErrorContains(t, "fn Main() -> Int { let m = matrix[[1, 2] [3, 4]] return m[0] }", "matrix indexing requires exactly 2 indices, got 1")
	assertTypeErrorContains(t, "fn Main() -> Int { let v = vector[1, 2] return v[0, 0] }", "vector indexing requires exactly 1 index, got 2")
}

func TestCheckValidatesMatrixConstructionSurfaceMx100a(t *testing.T) {
	validPrograms := []string{
		"fn Elem(r: Int, c: Int) -> Float { return Float(r * 3 + c * 5) } fn Main() -> Matrix<Float> { return Matrix.tabulate(2, 3, Elem) }",
		"fn Main() -> Matrix<Int> { return Matrix.zeros<Int>(2, 3) }",
		"fn Main() -> Matrix<Float> { return Matrix.fill(2, 3, 1.5) }",
		"fn Main() -> Matrix<Int> { return Matrix.identity<Int>(3) }",
		"fn Main() -> Int { let m = Matrix.fill(2, 3, 9) return m.rows + m.cols }",
	}
	for _, src := range validPrograms {
		file := parseSource(t, src)
		if err := Check(file); err != nil {
			t.Fatalf("Check returned error for %q: %v", src, err)
		}
	}
}

func TestCheckRejectsInvalidMatrixConstructionSurfaceMx100a(t *testing.T) {
	assertTypeErrorContains(t, "fn Bad(r: Int) -> Int { return r } fn Main() -> Matrix<Int> { return Matrix.tabulate(2, 2, Bad) }", "function 'Matrix.tabulate' argument 3 expects function (Int, Int) -> T")
	assertTypeErrorContains(t, "fn Main() -> Matrix<Int> { return Matrix.zeros<Int>(2.0, 2) }", "function 'Matrix.zeros' argument 1 expects Int, got Float")
	assertTypeErrorContains(t, "fn Main() -> Matrix<Int> { return Matrix.identity<Int>(2.0) }", "function 'Matrix.identity' argument 1 expects Int, got Float")
	assertTypeErrorContains(t, "fn Main() -> Int { let m = matrix[[1, 2] [3, 4]] return m.depth }", "type 'Matrix<Int>' has no field 'depth'")
}

func TestCheckFieldAccessDiagnosticsForRecordsAndChains(t *testing.T) {
	assertTypeErrorContains(t, "record Adaptive { FinalA: Float } record Run { AdaptiveKF: Adaptive } fn Main() -> Int { let run = Run { AdaptiveKF: Adaptive { FinalA: 1.0 } } return run.AdaptiveKF.FinelA }", "record has no field 'FinelA'")
	assertTypeErrorContains(t, "record Adaptive { FinalA: Float } record Run { AdaptiveKF: Adaptive } fn Main() -> Int { let run = Run { AdaptiveKF: Adaptive { FinalA: 1.0 } } return run.AdaptiveKF.FinelA }", "while resolving 'run.AdaptiveKF.FinelA'")
	assertTypeErrorContains(t, "record Adaptive { FinalA: Float } record Run { AdaptiveKF: Adaptive } fn Main() -> Int { let run = Run { AdaptiveKF: Adaptive { FinalA: 1.0 } } return run.AdaptiveKF.FinelA }", "available fields: FinalA")
	assertTypeErrorContains(t, "record Adaptive { FinalA: Float } record Run { AdaptiveKF: Adaptive } fn Main() -> Int { let run = Run { AdaptiveKF: Adaptive { FinalA: 1.0 } } return run.AdaptiveKF.FinalA.Value }", "type 'Float' has no field 'Value'")
	assertTypeErrorContains(t, "record Adaptive { FinalA: Float } record Run { AdaptiveKF: Adaptive } fn Main() -> Int { let run = Run { AdaptiveKF: Adaptive { FinalA: 1.0 } } return run.AdaptiveKF.FinalA.Value }", "while resolving 'run.AdaptiveKF.FinalA.Value'")
	assertTypeErrorContains(t, "record Adaptive { FinalA: Float } record Run { AdaptiveKF: Adaptive } fn Main() -> Int { let run = Run { AdaptiveKF: Adaptive { FinalA: 1.0 } } return missing.AdaptiveKF.FinalA }", "unknown name 'missing'\nwhile resolving 'missing.AdaptiveKF.FinalA'")
}

func TestCheckValidatesM92DimensionedLinearAlgebra(t *testing.T) {
	validPrograms := []string{
		"fn Main() -> Vector<Float<m>> { return vector[1.0m, 2.0m] }",
		"fn Main() -> Matrix<Float<kg/s^2>> { return matrix[[1.0kg/s^2, 2.0kg/s^2] [3.0kg/s^2, 4.0kg/s^2]] }",
		"fn Main() -> Vector<Float<kg*m/s^2>> { let k = matrix[[2.0kg/s^2, 0.0kg/s^2] [0.0kg/s^2, 3.0kg/s^2]] let u = vector[4.0m, 5.0m] return k @ u }",
		"fn Main() -> Matrix<Float<kg^2/s^3>> { let a = matrix[[1.0kg/s, 0.0kg/s] [0.0kg/s, 2.0kg/s]] let b = matrix[[3.0kg/s^2, 0.0kg/s^2] [0.0kg/s^2, 4.0kg/s^2]] return a @ b }",
		"fn Main() -> Vector<Float<kg*m/s^2>> { let u = vector[4.0m, 5.0m] let k = matrix[[2.0kg/s^2, 0.0kg/s^2] [0.0kg/s^2, 3.0kg/s^2]] return u @ k }",
		"fn Main() -> Float<kg*m/s^2> { return vector[1.0m, 2.0m] @ vector[3.0kg/s^2, 4.0kg/s^2] }",
	}
	for _, src := range validPrograms {
		file := parseSource(t, src)
		if err := Check(file); err != nil {
			t.Fatalf("Check returned error for %q: %v", src, err)
		}
	}
}

func TestCheckRejectsInvalidM92DimensionedLinearAlgebra(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Vector<Float<kg*m/s^2>> { let k = matrix[[2.0kg/s^2, 0.0kg/s^2] [0.0kg/s^2, 3.0kg/s^2]] let u = vector[4.0m, 5.0m] let f = k @ u return f + vector[1.0s, 2.0s] }", "cannot add m*kg/s^2 and s")
	assertTypeErrorContains(t, "fn Main() -> Vector<Float<m>> { return [1.0m, 2.0m] }", "function Main: function expects Vector<Float<m>>, but return is Float<m>[]")
	assertTypeErrorContains(t, "fn Main() -> Matrix<Float<kg/s^2>> { return [[1.0kg/s^2, 2.0kg/s^2], [3.0kg/s^2, 4.0kg/s^2]] }", "function Main: function expects Matrix<Float<kg/s^2>>, but return is Float<kg/s^2>[]")
}

func TestCheckValidatesM41aFlowStaticSurface(t *testing.T) {
	validPrograms := []string{
		"flow Idle() -> Void { state Start { suspend } } fn Main() -> Int { return 0 }",
		"flow Patrol(input: Int) -> Int { state Search { if input > 0 { goto Track } suspend } state Track { if input == 0 { goto Search } return input } } fn Main() -> Int { return 0 }",
		"flow Compute(x: Int) -> Int { state Start { let y = x + 1 if y > 0 { return y } suspend } } fn Main() -> Int { return 0 }",
		"flow Guarded(input: Int) -> Int { state Start { when { case input > 1 -> goto Next case input == 1 -> return 1 else -> suspend } } state Next { return input } } fn Main() -> Int { return 0 }",
		"flow Boarded(flag: Bool) -> Int { board { FaultLatched: Bool Count: Int Cooldown: Float Label: String } state S { when { case flag -> { remember board.FaultLatched = true board.Count = board.Count + 1 goto Hold } else -> { board.Label = \"idle\" suspend } } } state Hold { return board.Count } } fn Main() -> Int { return 0 }",
		"flow Utility(flag: Bool) -> Int { state S { let x = when policy { hysteresis: 2 min_commit: 1 } { case 7 when flag score 100 else 3 } return x } } fn Main() -> Int { return 0 }",
		"fn Main(flag: Bool) -> Int { return when utility { case 7 when flag score 100 else 3 } }",
		"fn Main(flag: Bool) -> Int { return when utility { hysteresis: 2 } { case 7 when flag score 100 else 3 } }",
		"flow Remembered(flag: Bool) -> Int { state A { remember goto B } state B { if flag { resume } return 3 } } fn Main() -> Int { return 0 }",
	}
	for _, src := range validPrograms {
		file := parseSource(t, src)
		if err := Check(file); err != nil {
			t.Fatalf("Check returned error for %q: %v", src, err)
		}
	}
}

func TestCheckRejectsInvalidM41aFlowStaticSurface(t *testing.T) {
	assertTypeErrorContains(t, "flow Empty() -> Int { } fn Main() -> Int { return 0 }", "flow Empty: must declare at least one state")
	assertTypeErrorContains(t, "flow Dup() -> Int { state S { suspend } state S { return 1 } } fn Main() -> Int { return 0 }", "flow Dup: duplicate state 'S'")
	assertTypeErrorContains(t, "flow Missing() -> Int { state S { goto T } } fn Main() -> Int { return 0 }", "flow Missing: goto target 'T' does not exist")
	assertTypeErrorContains(t, "fn Main() -> Int { goto S return 0 }", "function Main: goto is only valid inside flow state bodies")
	assertTypeErrorContains(t, "fn Main() -> Int { suspend return 0 }", "function Main: suspend is only valid inside flow state bodies")
	assertTypeErrorContains(t, "fn Main() -> Int { remember return 0 }", "function Main: remember is only valid inside flow state bodies")
	assertTypeErrorContains(t, "fn Main() -> Int { resume return 0 }", "function Main: resume is only valid inside flow state bodies")
	assertTypeErrorContains(t, "flow BadReturn() -> Int { state S { return true } } fn Main() -> Int { return 0 }", "function BadReturn: function expects Int, but return is Bool")
	assertTypeErrorContains(t, "fn Main() -> Int { when { case true -> return 1 else -> return 2 } }", "function Main: guard when is only valid inside flow state bodies; outside flows use switch or when utility")
	assertTypeErrorContains(t, "flow MissingElse(input: Bool) -> Int { state S { when { case input -> return 1 } } } fn Main() -> Int { return 0 }", "function MissingElse: when requires else branch")
	assertTypeErrorContains(t, "flow BadCond(input: Int) -> Int { state S { when { case input -> return 1 else -> return 0 } } } fn Main() -> Int { return 0 }", "function BadCond: when case condition must be Bool, got Int")
	assertTypeErrorContains(t, "flow BadGoto(input: Bool) -> Int { state S { when { case input -> goto Missing else -> return 0 } } } fn Main() -> Int { return 0 }", "flow BadGoto: goto target 'Missing' does not exist")
	assertTypeErrorContains(t, "flow BadWhenReturn(input: Bool) -> Int { state S { when { case input -> return true else -> return 0 } } } fn Main() -> Int { return 0 }", "function BadWhenReturn: function expects Int, but return is Bool")
	assertTypeErrorContains(t, "flow BadWhenBlock(input: Bool) -> Int { state S { when { case input -> { let x = 1 return x } else -> return 0 } } } fn Main() -> Int { return 0 }", "function BadWhenBlock: a `when` action block may only contain flow-control actions (`remember`, `resume`, `goto`, `suspend`, `return`) and `board.field = ...` updates; move ordinary computation before the `when`, or use a normal block outside the flow guard")
	assertTypeErrorContains(t, "flow BadWhenBlockEnd(input: Bool) -> Int { state S { when { case input -> { remember } else -> return 0 } } } fn Main() -> Int { return 0 }", "function BadWhenBlockEnd: a `when` action block must end with a control transfer (`goto`, `suspend`, `resume`, or `return`) so the flow transition is deterministic")
	assertTypeErrorContains(t, "flow BadBoardField() -> Int { board { A: Bool A: Int } state S { return 0 } } fn Main() -> Int { return 0 }", "flow BadBoardField: duplicate board field 'A'")
	assertTypeErrorContains(t, "flow BadBoardType() -> Int { board { Slots: Complex[] } state S { return 0 } } fn Main() -> Int { return 0 }", "flow BadBoardType board field Slots: board fields must be Bool, String, Int/Int<D>, Float/Float<D>, or arrays of those types")
	assertTypeErrorContains(t, "flow BadBoardAccess() -> Int { board { A: Bool } state S { board.B = true return 0 } } fn Main() -> Int { return 0 }", "function BadBoardAccess: type '__flow_board_BadBoardAccess' has no field 'B'")
	assertTypeErrorContains(t, "flow BadBoardAssign() -> Int { board { Count: Int } state S { board.Count = 1.5 return 0 } } fn Main() -> Int { return 0 }", "function BadBoardAssign: assignment to board.Count: expected Int, got Float")
	assertTypeErrorContains(t, "flow BadBoardParam(board: Int) -> Int { board { Flag: Bool } state S { return 0 } } fn Main() -> Int { return 0 }", "flow BadBoardParam: parameter name 'board' conflicts with flow board declaration")
	assertTypeErrorContains(t, "fn Main() -> Int { return when policy { hysteresis: 1 min_commit: 1 } { case 1 when true score 1 else 0 } }", "function Main: when policy is only valid inside flow state bodies; outside flows use switch or when utility")
	assertTypeErrorContains(t, "flow BadUtilityCond(x: Int) -> Int { state S { let y = when policy { hysteresis: 1 min_commit: 1 } { case 1 when x score 1 else 0 } return y } } fn Main() -> Int { return 0 }", "function BadUtilityCond: let y: utility when case condition must be Bool")
	assertTypeErrorContains(t, "flow BadUtilityScore(flag: Bool) -> Int { state S { let y = when policy { hysteresis: 1 min_commit: 1 } { case 1 when flag score 1.5 else 0 } return y } } fn Main() -> Int { return 0 }", "function BadUtilityScore: let y: utility when case score must be Int")
	assertTypeErrorContains(t, "flow BadUtilityPolicy(flag: Bool) -> Int { state S { let y = when policy { hysteresis: true min_commit: 1 } { case 1 when flag score 1 else 0 } return y } } fn Main() -> Int { return 0 }", "function BadUtilityPolicy: let y: utility when policy hysteresis must be Int")
	assertTypeErrorContains(t, "flow BadUtilityResult(flag: Bool) -> Int { state S { let y = when policy { hysteresis: 1 min_commit: 1 } { case 1 when flag score 1 else false } return y } } fn Main() -> Int { return 0 }", "function BadUtilityResult: let y: utility when result arms must have matching types")
}

func TestCheckValidatesPrometheusMatMulBuiltin(t *testing.T) {
	file := parseSource(t, "fn Main() -> Matrix<Float> { let a = Matrix.fill(2, 2, 1.0) let b = Matrix.fill(2, 2, 2.0) return PrometheusMatMul(a, b) }")
	if err := Check(file); err != nil {
		t.Fatalf("Check returned error: %v", err)
	}

	assertTypeErrorContains(t, "fn Main() -> Matrix<Float> { let a = Matrix.fill(2, 2, 1.0) let b = Matrix.fill(2, 2, 2) return PrometheusMatMul(a, b) }", "function Main: function 'PrometheusMatMul' argument 2 expects Matrix<Float>, got Matrix<Int>")
	assertTypeErrorContains(t, "fn Main() -> Matrix<Float> { let a = Matrix.fill(2, 2, 1.0) return PrometheusMatMul(a) }", "function Main: function 'PrometheusMatMul' expects 2 arguments, got 1")
}

func TestCheckEnumTargetedUtilityWhen(t *testing.T) {
	validPrograms := []string{
		"enum Decision { Hold Run Fault } fn Judge(flag: Bool) -> Decision { return when utility Decision { case Decision.Fault when flag score 100 else Decision.Hold } }",
		"enum Decision { Hold Run Fault(String) } fn Judge(flag: Bool) -> Decision { return when utility Decision { case Decision.Run when flag score 10 else Decision.Hold } }",
		"enum Decision { Hold Run(Int) Fault } fn Judge(flag: Bool) -> Decision { return when utility Decision { case Decision.Run(3) when flag score 10 else Decision.Hold } }",
		"enum Decision { Hold Run Fault(String) } fn Judge(flag: Bool) -> Decision { return when utility Decision { case Decision.Run when flag score 10 else Decision.Fault(\"fallback\") } }",
		"enum Decision { Hold Run Fault } fn Judge(flag: Bool) -> Decision { return when utility Decision { case Decision.Run when flag score 10 else Decision.Hold } }",
	}
	for _, src := range validPrograms {
		file := parseSource(t, src)
		if err := Check(file); err != nil {
			t.Fatalf("Check returned error for %q: %v", src, err)
		}
	}

	assertTypeErrorContains(t, "record Decision { Value: Int } fn Judge(flag: Bool) -> Decision { return when utility Decision { case Decision.Value when flag score 10 else Decision.Value } }", "when utility target must be an enum type")
	assertTypeErrorContains(t, "enum Decision { Hold Run } enum Other { Run } fn Judge(flag: Bool) -> Decision { return when utility Decision { case Other.Run when flag score 10 else Decision.Hold } }", "utility case for Decision cannot return Other.Run")
	assertTypeErrorContains(t, "enum Decision { Hold Run } fn Judge(flag: Bool) -> Decision { return when utility Decision { case Run when flag score 10 else Decision.Hold } }", "utility enum cases must use qualified variants")
	assertTypeErrorContains(t, "enum Decision { Hold Run } fn Judge(flag: Bool) -> Decision { return when utility Decision { case Decision.Run when flag score 10 } }", "enum utility selection requires else fallback")
	assertTypeErrorContains(t, "enum Decision { Hold Run } fn Judge(flag: Int) -> Decision { return when utility Decision { case Decision.Run when flag score 10 else Decision.Hold } }", "utility case condition must be Bool")
	assertTypeErrorContains(t, "enum Decision { Hold Run } fn Judge(flag: Bool) -> Decision { return when utility Decision { case Decision.Run when flag score 10.5 else Decision.Hold } }", "utility score must be Int")
	assertTypeErrorContains(t, "enum Decision { Hold Run(String) } fn Judge(flag: Bool) -> Decision { return when utility Decision { case Decision.Run(1) when flag score 10 else Decision.Hold } }", "payload for Decision.Run must be String")
	assertTypeErrorContains(t, "enum Decision { Hold Run } enum Other { Hold } fn Judge(flag: Bool) -> Decision { return when utility Decision { case Decision.Run when flag score 10 else Other.Hold } }", "utility else for Decision cannot return Other.Hold")
	assertTypeErrorContains(t, "enum Decision { Hold(String) Run } fn Judge(flag: Bool) -> Decision { return when utility Decision { case Decision.Run when flag score 10 else Decision.Hold } }", "enum variant Decision.Hold requires a payload")
	assertTypeErrorContains(t, "enum Decision { Hold Run } fn Judge(flag: Bool) -> Decision { return when utility Decision { case Decision.Run(1) when flag score 10 else Decision.Hold } }", "enum variant Decision.Run does not accept a payload")
}

func TestCheckFirstClassRangeExpressionTypes(t *testing.T) {
	programs := []string{
		"fn Main() -> Range { let r = 1..3 return r }",
		"fn Main() -> Range { let r = 1.. return r }",
		"fn Main() -> Range { let r = ..3 return r }",
		"fn Main() -> Range { let r = .. return r }",
		"fn Main() -> Range { return 1..10 step 2 }",
		"fn MakeRange() -> Range { return 2..8 } fn Main() -> Range { return MakeRange() }",
		"record Window { Bounds: Range } fn Main() -> Range { let w = Window { Bounds: ..10 } return w.Bounds }",
	}
	for _, src := range programs {
		file := parseSource(t, src)
		if err := Check(file); err != nil {
			t.Fatalf("Check returned error for %q: %v", src, err)
		}
	}
}

func TestCheckFirstClassRangeDiagnostics(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Range { return true..10 }", "range start must be Int")
	assertTypeErrorContains(t, "fn Main() -> Range { return 1..false }", "range end must be Int")
	assertTypeErrorContains(t, "fn Main() -> Range { return 1..10 step false }", "range step must be Int")
	assertTypeErrorContains(t, "fn Main() -> Int { for i in 0.. { return i } return 0 }", "for loop range requires start and end")
	assertTypeErrorContains(t, "fn Main() -> Int { for i in ..10 { return i } return 0 }", "for loop range requires start and end")
}

func TestMakeRequiresAuthorityForDirectHostPrimitives(t *testing.T) {
	valid := []string{
		`import Make
[RequiresAuthority]
fn CheckTools() -> String ! Error { return Make.Tool("go")? }`,
		`import Make
[RequiresAuthority]
fn ReadEnv() -> Make.EnvValue ! Error { return Make.Env("PATH")? }`,
		`import Make
[RequiresAuthority]
fn Run() -> Int ! Error { let result = Make.Exec("go", ["version"])? return result.ExitCode }`,
		`import Make
[Pure]
[NoWhile]
fn Target() -> Make.CommandTarget { return Make.CommandTarget { Name: "Build" Inputs: [] Outputs: [] Deps: [] Program: "go" Args: ["build"] Cwd: "." Env: [] } }`,
		`import Make
fn Plan() -> Make.Plan { return Make.Plan { Default: "Build" Config: Make.DefaultConfig() CommandTargets: [Make.CommandTarget { Name: "Build" Inputs: [] Outputs: [] Deps: [] Program: "go" Args: ["build"] Cwd: "." Env: [] }] FunctionTargets: [] FlowTargets: [] PhonyTargets: [] } }`,
	}
	for _, src := range valid {
		program := makeAuthorityTestProgram(t, "Make.oct", src)
		if err := CheckProgram(program); err != nil {
			t.Fatalf("Check returned error for %q: %v", src, err)
		}
	}

	invalid := []struct {
		name string
		src  string
		want string
	}{
		{"tool", `import Make
fn CheckTools() -> String ! Error { return Make.Tool("go")? }`, "function CheckTools calls Make.Tool and must be marked [RequiresAuthority]"},
		{"env", `import Make
fn ReadEnv() -> Make.EnvValue ! Error { return Make.Env("PATH")? }`, "function ReadEnv calls Make.Env and must be marked [RequiresAuthority]"},
		{"exec", `import Make
fn Run() -> Int ! Error { let result = Make.Exec("go", ["version"])? return result.ExitCode }`, "function Run calls Make.Exec and must be marked [RequiresAuthority]"},
		{"pure", `import Make
[Pure]
fn CheckTools() -> String ! Error { return Make.Tool("go")? }`, "function CheckTools calls Make.Tool and must be marked [RequiresAuthority]"},
		{"helper direct only", `import Make
fn Helper() -> String ! Error { return Make.Tool("go")? }
fn CheckTools() -> String ! Error { return Helper()? }`, "function Helper calls Make.Tool and must be marked [RequiresAuthority]"},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			program := makeAuthorityTestProgram(t, "Make.oct", tt.src)
			err := CheckProgram(program)
			if err == nil {
				t.Fatal("expected type error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error to contain %q, got %q", tt.want, err.Error())
			}
		})
	}
}

func TestMakeRequiresAuthorityScopeOnlyMakeOct(t *testing.T) {
	for _, path := range []string{"Ordinary.oct", "Example.octest"} {
		program := makeAuthorityTestProgram(t, path, `import Make
fn CheckTools() -> String ! Error { return Make.Tool("go")? }`)
		if err := CheckProgram(program); err != nil {
			t.Fatalf("Check returned error for %s: %v", path, err)
		}
	}
}

func makeAuthorityTestProgram(t *testing.T, mainPath string, mainSource string) project.Program {
	t.Helper()
	mainFile := parseSourceWithPath(t, mainPath, mainSource)
	makeFile := parseSourceWithPath(t, "Make.Core.oct", `package Make
record ProcessResult { ExitCode: Int Stdout: String Stderr: String }
record EnvValue { Name: String Present: Bool Value: String }
enum Staleness { Timestamp Always }
record Config { Profile: String StateDir: String Trace: Bool Staleness: Staleness }
record CommandTarget { Name: String Inputs: String[] Outputs: String[] Deps: String[] Program: String Args: String[] Cwd: String Env: String[] }
record FunctionTarget { Name: String Inputs: String[] Outputs: String[] Deps: String[] Function: String }
record FlowTarget { Name: String Inputs: String[] Outputs: String[] Deps: String[] Flow: String MaxSteps: Int }
record PhonyTarget { Name: String Deps: String[] }
record Plan { Default: String Config: Config CommandTargets: CommandTarget[] FunctionTargets: FunctionTarget[] FlowTargets: FlowTarget[] PhonyTargets: PhonyTarget[] }
fn DefaultConfig() -> Config { return Config { Profile: "Default" StateDir: ".octmake" Trace: false Staleness: Staleness.Timestamp } }
fn Tool(name: String) -> String ! Error { return error("host") }
fn Env(name: String) -> EnvValue ! Error { return error("host") }
fn Exec(program: String, args: String[]) -> ProcessResult ! Error { return error("host") }
`)
	return project.Program{Entry: mainFile.Package, Packages: map[string]project.Package{
		mainFile.Package: {Name: mainFile.Package, Imports: mainFile.Imports, Records: mainFile.Records, Enums: mainFile.Enums, Functions: mainFile.Functions, Flows: mainFile.Flows},
		makeFile.Package: {Name: makeFile.Package, Imports: makeFile.Imports, Records: makeFile.Records, Enums: makeFile.Enums, Functions: makeFile.Functions, Flows: makeFile.Flows},
	}}
}

func parseSourceWithPath(t *testing.T, path string, text string) ast.File {
	t.Helper()
	if !strings.HasPrefix(strings.TrimSpace(text), "package ") {
		text = "package Main\n" + text
	}
	lexed, err := lex.Analyze(source.File{Path: path, Text: text})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	file, err := parse.BuildFile(lexed)
	if err != nil {
		t.Fatalf("BuildFile returned error: %v", err)
	}
	return file
}
