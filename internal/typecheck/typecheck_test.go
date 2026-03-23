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
			src:  "fn Add(x: Int, y: Int) -> Int { let sum = x + y return sum }",
		},
		{
			name: "mixed numeric return",
			src:  "fn Mix(a: Int, b: Float) -> Float { return a + b }",
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
