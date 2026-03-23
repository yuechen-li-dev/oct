package typecheck

import (
	"strings"
	"testing"

	"oct/internal/ast"
	"oct/internal/lex"
	"oct/internal/parse"
	"oct/internal/source"
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
			name: "for loop over range",
			src:  "fn Main() -> Int { for i in 0..3 { return i } return 0 }",
		},
		{
			name: "for loop scope shadowing",
			src:  "fn Main() -> Int { let i = 9 for i in 0..1 { return i } return i }",
		},
		{
			name: "fallible propagation",
			src:  "fn Safe() -> Int ! Error { return 5 } fn Main() -> Int ! Error { let x = Safe()? return x }",
		},
		{
			name: "match fallible result",
			src:  "fn Safe() -> Int ! Error { return 7 } fn Main() -> Int { match Safe() { ok(value) => { return value } err(e) => { return 0 } } }",
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

func TestCheckRejectsReturnTypeMismatches(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int { return 1.0 }", "function Main: function expects Int, but return is Float")
	assertTypeErrorContains(t, "fn Main() -> Float { return true }", "function Main: function expects Float, but return is Bool")
}

func TestCheckRejectsInvalidOperatorUsage(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int { return true + 1 }", `function Main: operator "+" not defined for Bool and Int`)
	assertTypeErrorContains(t, "fn Main() -> Int[] { return [1, 2] + 1 }", `function Main: operator "+" not defined for Int[] and Int`)
	assertTypeErrorContains(t, "fn Main() -> Bool[] { return [true, false] + [true, true] }", `function Main: operator "+" not defined for Bool[] and Bool[]`)
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
}

func TestCheckRejectsMissingReturnStatement(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int { let x = 1 }", "function Main: missing return statement")
}

func TestCheckRejectsMixedTypeArrayLiteral(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int[] { return [1, 2.0] }", "function Main: array literal elements must all have the same type; found Int and Float")
}

func TestCheckRejectsInvalidIndexing(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int { return 1[0] }", "function Main: cannot index non-array value of type Int")
	assertTypeErrorContains(t, "fn Main() -> Int { let x = [1, 2, 3] return x[true] }", "function Main: array index must be Int, got Bool")
}

func TestCheckRejectsInvalidRanges(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int { for i in 1.0..10 { return i } return 0 }", "function Main: for i: range start must be Int, got Float")
	assertTypeErrorContains(t, "fn Main() -> Int { for i in 1..10 step 0 { return i } return 0 }", "function Main: for i: range step must be positive, got 0")
}

func TestCheckRejectsLoopVariableOutsideLoop(t *testing.T) {
	assertTypeErrorContains(t, "fn Main() -> Int { for i in 0..1 { return i } return i }", "function Main: undefined variable: i")
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

func parseSource(t *testing.T, text string) ast.File {
	t.Helper()

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
