package parse

import (
	"strings"
	"testing"

	"oct/internal/ast"
	"oct/internal/lex"
	"oct/internal/source"
)

func TestBuildFileParsesFunctionWithNoParameters(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int { return 0 }")

	if len(file.Functions) != 1 {
		t.Fatalf("expected one function, got %d", len(file.Functions))
	}

	fn := file.Functions[0]
	if fn.Name != "Main" {
		t.Fatalf("expected function name Main, got %q", fn.Name)
	}
	if len(fn.Parameters) != 0 {
		t.Fatalf("expected no parameters, got %d", len(fn.Parameters))
	}
	if fn.ReturnType.Name != "Int" || fn.ReturnType.IsArray {
		t.Fatalf("expected return type Int, got %+v", fn.ReturnType)
	}
}

func TestBuildFileParsesParametersAndStatements(t *testing.T) {
	file := parseSource(t, "fn Add(x: Int, y: Int) -> Int { let sum = x + y return sum }")

	fn := file.Functions[0]
	if len(fn.Parameters) != 2 {
		t.Fatalf("expected two parameters, got %d", len(fn.Parameters))
	}
	if fn.Parameters[0].Name != "x" || fn.Parameters[0].Type.Name != "Int" || fn.Parameters[0].Type.IsArray {
		t.Fatalf("unexpected first parameter: %+v", fn.Parameters[0])
	}
	if len(fn.Body.Statements) != 2 {
		t.Fatalf("expected two statements, got %d", len(fn.Body.Statements))
	}
	if _, ok := fn.Body.Statements[0].(ast.LetStmt); !ok {
		t.Fatalf("expected first statement to be LetStmt, got %T", fn.Body.Statements[0])
	}
	if _, ok := fn.Body.Statements[1].(ast.ReturnStmt); !ok {
		t.Fatalf("expected second statement to be ReturnStmt, got %T", fn.Body.Statements[1])
	}
}

func TestBuildFileParsesFallibleFunctionCallsAndMatch(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int ! Error { let x = Safe()? match Safe() { ok(value) => { return value } err(e) => { return error(\"bad\") } } }")

	fn := file.Functions[0]
	if !fn.IsFallible {
		t.Fatal("expected function to be fallible")
	}
	if fn.ErrorType.Name != "Error" {
		t.Fatalf("expected Error error type, got %+v", fn.ErrorType)
	}

	letStmt, ok := fn.Body.Statements[0].(ast.LetStmt)
	if !ok {
		t.Fatalf("expected let statement, got %T", fn.Body.Statements[0])
	}
	propagate, ok := letStmt.Value.(ast.PropagateExpr)
	if !ok {
		t.Fatalf("expected propagate expression, got %T", letStmt.Value)
	}
	call, ok := propagate.Inner.(ast.CallExpr)
	if !ok || call.Callee != "Safe" {
		t.Fatalf("expected propagated Safe() call, got %T %#v", propagate.Inner, propagate.Inner)
	}

	matchStmt, ok := fn.Body.Statements[1].(ast.MatchStmt)
	if !ok {
		t.Fatalf("expected match statement, got %T", fn.Body.Statements[1])
	}
	matchCall, ok := matchStmt.Subject.(ast.CallExpr)
	if !ok || matchCall.Callee != "Safe" {
		t.Fatalf("expected match subject Safe() call, got %T %#v", matchStmt.Subject, matchStmt.Subject)
	}
	if matchStmt.OkName != "value" || matchStmt.ErrName != "e" {
		t.Fatalf("unexpected match bindings: ok=%q err=%q", matchStmt.OkName, matchStmt.ErrName)
	}
}

func TestBuildFileParsesFatalUnwrapAndStrings(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int { let x = Fail()! return x }")

	letStmt := file.Functions[0].Body.Statements[0].(ast.LetStmt)
	unwrap, ok := letStmt.Value.(ast.UnwrapExpr)
	if !ok {
		t.Fatalf("expected unwrap expression, got %T", letStmt.Value)
	}
	call, ok := unwrap.Inner.(ast.CallExpr)
	if !ok || call.Callee != "Fail" {
		t.Fatalf("expected Fail() call inside unwrap, got %T %#v", unwrap.Inner, unwrap.Inner)
	}

	errorFile := parseSource(t, "fn Fail() -> Int ! Error { return error(\"boom\") }")
	ret := errorFile.Functions[0].Body.Statements[0].(ast.ReturnStmt)
	errorCall, ok := ret.Value.(ast.CallExpr)
	if !ok || errorCall.Callee != "error" {
		t.Fatalf("expected error() call, got %T %#v", ret.Value, ret.Value)
	}
	stringArg, ok := errorCall.Arguments[0].(ast.StringLiteralExpr)
	if !ok || stringArg.Value != "boom" {
		t.Fatalf("expected string literal argument, got %T %#v", errorCall.Arguments[0], errorCall.Arguments[0])
	}
}

func TestBuildFileRespectsExpressionPrecedence(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int { return 1 + 2 * 3 }")

	ret := file.Functions[0].Body.Statements[0].(ast.ReturnStmt)
	expr, ok := ret.Value.(ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", ret.Value)
	}
	if expr.Operator != "+" {
		t.Fatalf("expected top-level '+' operator, got %q", expr.Operator)
	}
	right, ok := expr.Right.(ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected right branch BinaryExpr, got %T", expr.Right)
	}
	if right.Operator != "*" {
		t.Fatalf("expected nested '*' operator, got %q", right.Operator)
	}
}

func TestBuildFilePreservesParenthesizedExpression(t *testing.T) {
	file := parseSource(t, "fn Main() -> Float { return (a + b) * 2.0 }")

	ret := file.Functions[0].Body.Statements[0].(ast.ReturnStmt)
	expr := ret.Value.(ast.BinaryExpr)
	if expr.Operator != "*" {
		t.Fatalf("expected top-level '*' operator, got %q", expr.Operator)
	}
	left, ok := expr.Left.(ast.ParenExpr)
	if !ok {
		t.Fatalf("expected left branch ParenExpr, got %T", expr.Left)
	}
	inner, ok := left.Inner.(ast.BinaryExpr)
	if !ok || inner.Operator != "+" {
		t.Fatalf("expected parenthesized inner '+' expression, got %T %#v", left.Inner, left.Inner)
	}
}

func TestBuildFileParsesMultipleFunctions(t *testing.T) {
	file := parseSource(t, "fn One() -> Int { return 1 } fn Two() -> Bool { return true }")

	if len(file.Functions) != 2 {
		t.Fatalf("expected two functions, got %d", len(file.Functions))
	}
	if file.Functions[0].Name != "One" || file.Functions[1].Name != "Two" {
		t.Fatalf("unexpected function names: %+v", file.Functions)
	}
}

func TestBuildFileParsesArrayTypesLiteralsAndIndexing(t *testing.T) {
	file := parseSource(t, "fn Main(values: Int[]) -> Int[] { return [values[0], values[1]] }")

	fn := file.Functions[0]
	if !fn.Parameters[0].Type.IsArray || fn.Parameters[0].Type.Name != "Int" {
		t.Fatalf("expected array parameter type, got %+v", fn.Parameters[0].Type)
	}
	if !fn.ReturnType.IsArray || fn.ReturnType.Name != "Int" {
		t.Fatalf("expected array return type, got %+v", fn.ReturnType)
	}

	ret := fn.Body.Statements[0].(ast.ReturnStmt)
	arrayLiteral, ok := ret.Value.(ast.ArrayLiteralExpr)
	if !ok {
		t.Fatalf("expected ArrayLiteralExpr, got %T", ret.Value)
	}
	if len(arrayLiteral.Elements) != 2 {
		t.Fatalf("expected two elements, got %d", len(arrayLiteral.Elements))
	}
	indexExpr, ok := arrayLiteral.Elements[0].(ast.IndexExpr)
	if !ok {
		t.Fatalf("expected first element to be IndexExpr, got %T", arrayLiteral.Elements[0])
	}
	identifier, ok := indexExpr.Target.(ast.IdentifierExpr)
	if !ok || identifier.Name != "values" {
		t.Fatalf("expected index target values identifier, got %T %#v", indexExpr.Target, indexExpr.Target)
	}
}

func TestBuildFileParsesRangeExpressionsAndForLoops(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int { for i in 0..10 step 2 { return i } return 0 }")

	fn := file.Functions[0]
	if len(fn.Body.Statements) != 2 {
		t.Fatalf("expected two statements, got %d", len(fn.Body.Statements))
	}

	forStmt, ok := fn.Body.Statements[0].(ast.ForStmt)
	if !ok {
		t.Fatalf("expected first statement to be ForStmt, got %T", fn.Body.Statements[0])
	}
	if forStmt.Name != "i" {
		t.Fatalf("expected loop variable i, got %q", forStmt.Name)
	}

	rangeExpr, ok := forStmt.Range.(ast.RangeExpr)
	if !ok {
		t.Fatalf("expected for range to be RangeExpr, got %T", forStmt.Range)
	}
	if _, ok := rangeExpr.Start.(ast.IntegerLiteral); !ok {
		t.Fatalf("expected range start integer literal, got %T", rangeExpr.Start)
	}
	if _, ok := rangeExpr.End.(ast.IntegerLiteral); !ok {
		t.Fatalf("expected range end integer literal, got %T", rangeExpr.End)
	}
	if _, ok := rangeExpr.Step.(ast.IntegerLiteral); !ok {
		t.Fatalf("expected range step integer literal, got %T", rangeExpr.Step)
	}
}

func TestBuildFileRejectsMissingReturnType(t *testing.T) {
	assertParseErrorContains(t, "fn Main() { return 0 }", "expected '->' before return type")
}

func TestBuildFileRejectsMalformedParameterList(t *testing.T) {
	assertParseErrorContains(t, "fn Add(x Int, y: Int) -> Int { return x + y }", "expected ':' after parameter name")
}

func TestBuildFileRejectsMalformedLetStatement(t *testing.T) {
	assertParseErrorContains(t, "fn Main() -> Int { let x = return 0 }", "expected expression")
}

func TestBuildFileRejectsMalformedReturnExpression(t *testing.T) {
	assertParseErrorContains(t, "fn Main() -> Int { return 1 + }", "expected expression")
}

func TestBuildFileRejectsInvalidTopLevelContent(t *testing.T) {
	assertParseErrorContains(t, "let x = 1", "expected 'fn' at top level")
}

func TestBuildFileRejectsUnterminatedBlock(t *testing.T) {
	assertParseErrorContains(t, "fn Main() -> Int { return 0", "expected '}' to close block")
}

func TestBuildFileRejectsEmptyArrayLiteral(t *testing.T) {
	assertParseErrorContains(t, "fn Main() -> Int[] { return [] }", "empty array literals are not supported")
}

func TestBuildFileRejectsNestedArrayTypeSyntax(t *testing.T) {
	assertParseErrorContains(t, "fn Main() -> Int[][] { return [1] }", "nested array types are not supported")
}

func TestBuildFileRejectsMalformedMatch(t *testing.T) {
	assertParseErrorContains(t, "fn Main() -> Int { match Safe() { err(e) => { return 0 } ok(v) => { return v } } }", "expected 'ok' arm")
}

func parseSource(t *testing.T, text string) ast.File {
	t.Helper()

	lexed, err := lex.Analyze(source.File{Path: "example.oct", Text: text})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	file, err := BuildFile(lexed)
	if err != nil {
		t.Fatalf("BuildFile returned error: %v", err)
	}
	return file
}

func assertParseErrorContains(t *testing.T, text string, want string) {
	t.Helper()

	lexed, err := lex.Analyze(source.File{Path: "example.oct", Text: text})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	_, err = BuildFile(lexed)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("expected error to contain %q, got %q", want, err.Error())
	}
}
