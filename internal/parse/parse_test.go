package parse

import (
	"reflect"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/lex"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func TestBuildFileParsesUnifiedArrowsWithEquivalentAst(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
	}{
		{
			name:  "function signature arrow",
			left:  "fn Main() -> Int { return 0 }",
			right: "fn Main() => Int { return 0 }",
		},
		{
			name:  "function type arrow",
			left:  "fn Apply(f: fn(Int) -> Int) -> Int { return f(1) }",
			right: "fn Apply(f: fn(Int) => Int) => Int { return f(1) }",
		},
		{
			name:  "match arm arrow",
			left:  "fn Main() -> Int ! Error { match Safe() { ok(v) => { return v } err(e) => { return 0 } } }",
			right: "fn Main() -> Int ! Error { match Safe() { ok(v) -> { return v } err(e) -> { return 0 } } }",
		},
		{
			name:  "switch arrow",
			left:  "fn Main() -> Int { return switch 1 { case 1 => 2 else => 3 } }",
			right: "fn Main() -> Int { return switch 1 { case 1 -> 2 else -> 3 } }",
		},
		{
			name:  "flow and when arrows",
			left:  "flow Patrol(flag: Bool) -> Int { state Search { when { case flag -> goto Track else -> suspend } } state Track { return 1 } }",
			right: "flow Patrol(flag: Bool) => Int { state Search { when { case flag => goto Track else => suspend } } state Track { return 1 } }",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			left := parseSource(t, tc.left)
			right := parseSource(t, tc.right)
			left.Source.Text = ""
			right.Source.Text = ""
			if !reflect.DeepEqual(left, right) {
				t.Fatalf("expected equivalent AST for unified arrows\nleft: %#v\nright: %#v", left, right)
			}
		})
	}
}

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

func TestBuildFileParsesCallExpressionStatementInMain(t *testing.T) {
	file := parseSource(t, "fn main() -> Int { SimplePasses() return 0 }")
	fn := file.Functions[0]
	if len(fn.Body.Statements) != 2 {
		t.Fatalf("expected two statements, got %d", len(fn.Body.Statements))
	}
	exprStmt, ok := fn.Body.Statements[0].(ast.ExprStmt)
	if !ok {
		t.Fatalf("expected first statement to be ExprStmt, got %T", fn.Body.Statements[0])
	}
	if _, ok := exprStmt.Value.(ast.CallExpr); !ok {
		t.Fatalf("expected expression statement to contain CallExpr, got %T", exprStmt.Value)
	}
}

func TestBuildFileParsesPackageQualifiedTypeReferences(t *testing.T) {
	file := parseSource(t, "import Geometry\nfn UsePoint(p: Geometry.Point) -> Geometry.Point { return p }")

	fn := file.Functions[0]
	if fn.Parameters[0].Type.Package != "Geometry" || fn.Parameters[0].Type.Name != "Point" {
		t.Fatalf("expected qualified parameter type Geometry.Point, got %+v", fn.Parameters[0].Type)
	}
	if fn.ReturnType.Package != "Geometry" || fn.ReturnType.Name != "Point" {
		t.Fatalf("expected qualified return type Geometry.Point, got %+v", fn.ReturnType)
	}
}

func TestBuildFileParsesPackageQualifiedRecordLiteral(t *testing.T) {
	file := parseSource(t, "import Geometry\nfn Main() -> Int { let p = Geometry.Point { X: 1 Y: 2 } return p.X }")
	letStmt := file.Functions[0].Body.Statements[0].(ast.LetStmt)
	recordLiteral, ok := letStmt.Value.(ast.RecordLiteralExpr)
	if !ok {
		t.Fatalf("expected RecordLiteralExpr, got %T", letStmt.Value)
	}
	if recordLiteral.TypeName != "Geometry.Point" {
		t.Fatalf("expected qualified record literal type, got %q", recordLiteral.TypeName)
	}
}

func TestBuildFileParsesRecordUpdateExpr(t *testing.T) {
	file := parseSource(t, "record Model { Value: Int Flag: Bool }\nfn Main() -> Model { let m = Model { Value: 1 Flag: false } return m with { Value: 2 Flag: true } }")
	ret := file.Functions[0].Body.Statements[1].(ast.ReturnStmt)
	update, ok := ret.Value.(ast.RecordUpdateExpr)
	if !ok {
		t.Fatalf("expected RecordUpdateExpr, got %T", ret.Value)
	}
	if len(update.Fields) != 2 || update.Fields[0].Name != "Value" || update.Fields[1].Name != "Flag" {
		t.Fatalf("unexpected update fields: %#v", update.Fields)
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
	callee, isName := call.Callee.(ast.IdentifierExpr)
	if !ok || !isName || callee.Name != "Safe" {
		t.Fatalf("expected propagated Safe() call, got %T %#v", propagate.Inner, propagate.Inner)
	}

	matchStmt, ok := fn.Body.Statements[1].(ast.MatchStmt)
	if !ok {
		t.Fatalf("expected match statement, got %T", fn.Body.Statements[1])
	}
	matchCall, ok := matchStmt.Subject.(ast.CallExpr)
	matchCallee, isMatchName := matchCall.Callee.(ast.IdentifierExpr)
	if !ok || !isMatchName || matchCallee.Name != "Safe" {
		t.Fatalf("expected match subject Safe() call, got %T %#v", matchStmt.Subject, matchStmt.Subject)
	}
	if matchStmt.OkName != "value" || matchStmt.ErrName != "e" {
		t.Fatalf("unexpected match bindings: ok=%q err=%q", matchStmt.OkName, matchStmt.ErrName)
	}
}

func TestBuildFileParsesCallTypeArguments(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int ! Error { return LoadOctagon<Int[]>(\"x.octagon\")? }")
	ret := file.Functions[0].Body.Statements[0].(ast.ReturnStmt)
	propagate := ret.Value.(ast.PropagateExpr)
	call := propagate.Inner.(ast.CallExpr)
	if len(call.TypeArguments) != 1 {
		t.Fatalf("expected one type argument, got %d", len(call.TypeArguments))
	}
	if call.TypeArguments[0].Name != "Int" || !call.TypeArguments[0].IsArray {
		t.Fatalf("expected Int[] type argument, got %+v", call.TypeArguments[0])
	}
}

func TestBuildFileRejectsLegacyBracketTypeArguments(t *testing.T) {
	assertParseErrorContains(t, "fn Main() -> Int ! Error { return LoadOctagon[Int](\"x.octagon\")? }", "type arguments must use '<...>'")
}

func TestBuildFileParsesSuiteAttribute(t *testing.T) {
	file := parseSourceWithPath(t, "x.octest", "package Main\n[Suite(\"A\")]\n[Fact]\nfn Alpha() -> Void { return }\n[Suite(\"A\")]\n[Suite(\"B\")]\n[Theory]\n[InlineData(1)]\nfn Beta(x: Int) -> Void { return }\n")
	if got := file.Functions[0].Suites; len(got) != 1 || got[0] != "A" {
		t.Fatalf("unexpected fact suites: %#v", got)
	}
	if got := file.Functions[1].Suites; len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Fatalf("unexpected theory suites: %#v", got)
	}
}

func TestBuildFileRejectsInvalidSuiteAttribute(t *testing.T) {
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Suite(123)]\n[Fact]\nfn Bad() -> Void { return }\n", "[Suite] requires a non-empty string literal")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Suite(\" \")]\n[Fact]\nfn Bad() -> Void { return }\n", "[Suite] requires a non-empty string literal")
}

func TestBuildFileRejectsTupleSyntax(t *testing.T) {
	assertParseErrorContains(t, "fn Bad() -> (Int, Int) { return 0 }", "tuple types are not supported")
	assertParseErrorContains(t, "fn Main() -> Int { a, b = Pair() return 0 }", "destructuring assignment is not supported")
}

func TestBuildFileParsesFatalUnwrapAndStrings(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int { let x = Fail()! return x }")

	letStmt := file.Functions[0].Body.Statements[0].(ast.LetStmt)
	unwrap, ok := letStmt.Value.(ast.UnwrapExpr)
	if !ok {
		t.Fatalf("expected unwrap expression, got %T", letStmt.Value)
	}
	call, ok := unwrap.Inner.(ast.CallExpr)
	callee, isName := call.Callee.(ast.IdentifierExpr)
	if !ok || !isName || callee.Name != "Fail" {
		t.Fatalf("expected Fail() call inside unwrap, got %T %#v", unwrap.Inner, unwrap.Inner)
	}

	errorFile := parseSource(t, "fn Fail() -> Int ! Error { return error(\"boom\") }")
	ret := errorFile.Functions[0].Body.Statements[0].(ast.ReturnStmt)
	errorCall, ok := ret.Value.(ast.CallExpr)
	errorCallee, isErrorName := errorCall.Callee.(ast.IdentifierExpr)
	if !ok || !isErrorName || errorCallee.Name != "error" {
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

func TestBuildFileParsesModuloWithMultiplicativePrecedence(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int { return 20 / 4 % 3 }")

	ret := file.Functions[0].Body.Statements[0].(ast.ReturnStmt)
	expr, ok := ret.Value.(ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", ret.Value)
	}
	if expr.Operator != "%" {
		t.Fatalf("expected top-level '%%' operator, got %q", expr.Operator)
	}
	left, ok := expr.Left.(ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected left branch BinaryExpr, got %T", expr.Left)
	}
	if left.Operator != "/" {
		t.Fatalf("expected nested '/' operator, got %q", left.Operator)
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

func TestBuildFileParsesComparisonPrecedenceBelowArithmetic(t *testing.T) {
	file := parseSource(t, "fn Main() -> Bool { return 1 + 2 < 4 }")
	ret := file.Functions[0].Body.Statements[0].(ast.ReturnStmt)
	expr, ok := ret.Value.(ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", ret.Value)
	}
	if expr.Operator != "<" {
		t.Fatalf("expected top-level '<' operator, got %q", expr.Operator)
	}
	left, ok := expr.Left.(ast.BinaryExpr)
	if !ok || left.Operator != "+" {
		t.Fatalf("expected left branch '+' BinaryExpr, got %T %#v", expr.Left, expr.Left)
	}
}

func TestBuildFileParsesLogicalOperatorPrecedence(t *testing.T) {
	file := parseSource(t, "fn Main() -> Bool { return true or false and false }")
	ret := file.Functions[0].Body.Statements[0].(ast.ReturnStmt)
	expr, ok := ret.Value.(ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", ret.Value)
	}
	if expr.Operator != "or" {
		t.Fatalf("expected top-level 'or', got %q", expr.Operator)
	}
	right, ok := expr.Right.(ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected right branch BinaryExpr, got %T", expr.Right)
	}
	if right.Operator != "and" {
		t.Fatalf("expected nested 'and', got %q", right.Operator)
	}
}

func TestBuildFileParsesNotAfterComparisons(t *testing.T) {
	file := parseSource(t, "fn Main() -> Bool { return not 1 == 2 }")
	ret := file.Functions[0].Body.Statements[0].(ast.ReturnStmt)
	expr, ok := ret.Value.(ast.UnaryExpr)
	if !ok {
		t.Fatalf("expected UnaryExpr, got %T", ret.Value)
	}
	if expr.Operator != "not" {
		t.Fatalf("expected unary operator 'not', got %q", expr.Operator)
	}
	operand, ok := expr.Operand.(ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected not operand to be BinaryExpr, got %T", expr.Operand)
	}
	if operand.Operator != "==" {
		t.Fatalf("expected comparison operand, got %q", operand.Operator)
	}
}

func TestBuildFileParsesUnaryMinusLiteral(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int { return -1 }")
	ret := file.Functions[0].Body.Statements[0].(ast.ReturnStmt)
	expr, ok := ret.Value.(ast.UnaryExpr)
	if !ok {
		t.Fatalf("expected UnaryExpr, got %T", ret.Value)
	}
	if expr.Operator != "-" {
		t.Fatalf("expected unary operator '-', got %q", expr.Operator)
	}
	if _, ok := expr.Operand.(ast.IntegerLiteral); !ok {
		t.Fatalf("expected integer literal operand, got %T", expr.Operand)
	}
}

func TestBuildFileUnaryMinusBindsTighterThanMultiplication(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int { return -1 * 2 }")
	ret := file.Functions[0].Body.Statements[0].(ast.ReturnStmt)
	expr, ok := ret.Value.(ast.BinaryExpr)
	if !ok {
		t.Fatalf("expected BinaryExpr, got %T", ret.Value)
	}
	if expr.Operator != "*" {
		t.Fatalf("expected top-level '*', got %q", expr.Operator)
	}
	if _, ok := expr.Left.(ast.UnaryExpr); !ok {
		t.Fatalf("expected unary minus on left operand, got %T", expr.Left)
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

func TestBuildFileParsesIfExpression(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int { let x = if true { 1 } else { 2 } return x }")

	letStmt := file.Functions[0].Body.Statements[0].(ast.LetStmt)
	ifExpr, ok := letStmt.Value.(ast.IfExpr)
	if !ok {
		t.Fatalf("expected IfExpr, got %T", letStmt.Value)
	}
	if _, ok := ifExpr.ThenExpr.(ast.IntegerLiteral); !ok {
		t.Fatalf("expected integer then branch, got %T", ifExpr.ThenExpr)
	}
	if _, ok := ifExpr.ElseExpr.(ast.IntegerLiteral); !ok {
		t.Fatalf("expected integer else branch, got %T", ifExpr.ElseExpr)
	}
}

func TestBuildFileParsesBatchExpression(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int[] { return batch [1, 2, 3] as item { let next = item + 1 return next } }")
	returnStmt, ok := file.Functions[0].Body.Statements[0].(ast.ReturnStmt)
	if !ok {
		t.Fatalf("expected return statement, got %T", file.Functions[0].Body.Statements[0])
	}
	batchExpr, ok := returnStmt.Value.(ast.BatchExpr)
	if !ok {
		t.Fatalf("expected batch expression, got %T", returnStmt.Value)
	}
	if batchExpr.ItemName != "item" {
		t.Fatalf("expected item binding name 'item', got %q", batchExpr.ItemName)
	}
	if len(batchExpr.Body.Statements) != 2 {
		t.Fatalf("expected 2 statements in batch body, got %d", len(batchExpr.Body.Statements))
	}
	if _, ok := batchExpr.Body.Statements[1].(ast.ReturnStmt); !ok {
		t.Fatalf("expected batch body to end in return, got %T", batchExpr.Body.Statements[1])
	}
}

func TestBuildFileRejectsIfExpressionWithoutElse(t *testing.T) {
	lexed, err := lex.Analyze(source.File{Path: "example.oct", Text: "package Main\nfn Main() -> Int { return if true { 1 } }"})
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	_, err = BuildFile(lexed)
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.Contains(err.Error(), "if expression requires else branch") {
		t.Fatalf("expected missing else error, got %q", err.Error())
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

func TestBuildFileParsesIndexAssignmentStatement(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int { var x = [1, 2, 3] x[1] = 5 return x[1] }")
	fn := file.Functions[0]
	if len(fn.Body.Statements) != 3 {
		t.Fatalf("expected three statements, got %d", len(fn.Body.Statements))
	}
	assignStmt, ok := fn.Body.Statements[1].(ast.IndexAssignStmt)
	if !ok {
		t.Fatalf("expected second statement to be IndexAssignStmt, got %T", fn.Body.Statements[1])
	}
	if assignStmt.Target != "x" {
		t.Fatalf("expected target x, got %q", assignStmt.Target)
	}
	if len(assignStmt.Indices) != 1 {
		t.Fatalf("expected one index, got %d", len(assignStmt.Indices))
	}
	if _, ok := assignStmt.Indices[0].(ast.IntegerLiteral); !ok {
		t.Fatalf("expected integer index, got %T", assignStmt.Indices[0])
	}
	if _, ok := assignStmt.Value.(ast.IntegerLiteral); !ok {
		t.Fatalf("expected integer value, got %T", assignStmt.Value)
	}
}

func TestBuildFileParsesMatrixIndexAssignmentStatement(t *testing.T) {
	file := parseSource(t, "fn Main() -> Matrix<Int> { var m = matrix[[1, 2] [3, 4]] m[0, 1] = 9 return m }")
	fn := file.Functions[0]
	assignStmt, ok := fn.Body.Statements[1].(ast.IndexAssignStmt)
	if !ok {
		t.Fatalf("expected second statement to be IndexAssignStmt, got %T", fn.Body.Statements[1])
	}
	if len(assignStmt.Indices) != 2 {
		t.Fatalf("expected two indices, got %d", len(assignStmt.Indices))
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

func TestBuildFileParsesForLoopWithExpressionBoundsAndImplicitStep(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int { let start = 1 let finish = Len([1, 2, 3]) for i in start..finish + 1 { return i } return 0 }")

	fn := file.Functions[0]
	forStmt, ok := fn.Body.Statements[2].(ast.ForStmt)
	if !ok {
		t.Fatalf("expected third statement to be ForStmt, got %T", fn.Body.Statements[2])
	}

	rangeExpr, ok := forStmt.Range.(ast.RangeExpr)
	if !ok {
		t.Fatalf("expected for range to be RangeExpr, got %T", forStmt.Range)
	}
	if _, ok := rangeExpr.Start.(ast.IdentifierExpr); !ok {
		t.Fatalf("expected range start identifier expression, got %T", rangeExpr.Start)
	}
	if _, ok := rangeExpr.End.(ast.BinaryExpr); !ok {
		t.Fatalf("expected range end binary expression, got %T", rangeExpr.End)
	}
	if rangeExpr.Step != nil {
		t.Fatalf("expected implicit step to be nil, got %T", rangeExpr.Step)
	}
}

func TestBuildFileParsesIfElseStatements(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int { if true { return 1 } else { return 2 } }")
	ifStmt, ok := file.Functions[0].Body.Statements[0].(ast.IfStmt)
	if !ok {
		t.Fatalf("expected IfStmt, got %T", file.Functions[0].Body.Statements[0])
	}
	if _, ok := ifStmt.Condition.(ast.BoolLiteral); !ok {
		t.Fatalf("expected bool condition, got %T", ifStmt.Condition)
	}
	if ifStmt.ElseBody == nil {
		t.Fatal("expected else body")
	}
}

func TestBuildFileParsesWhileStatements(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int { while true { return 7 } return 0 }")
	whileStmt, ok := file.Functions[0].Body.Statements[0].(ast.WhileStmt)
	if !ok {
		t.Fatalf("expected WhileStmt, got %T", file.Functions[0].Body.Statements[0])
	}
	if _, ok := whileStmt.Condition.(ast.BoolLiteral); !ok {
		t.Fatalf("expected bool condition, got %T", whileStmt.Condition)
	}
}

func TestBuildFileParsesVarAndAssignStatements(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int { var x = 1 x = 2 return x }")
	varStmt, ok := file.Functions[0].Body.Statements[0].(ast.VarStmt)
	if !ok {
		t.Fatalf("expected VarStmt, got %T", file.Functions[0].Body.Statements[0])
	}
	if varStmt.Name != "x" {
		t.Fatalf("expected var name x, got %q", varStmt.Name)
	}
	assignStmt, ok := file.Functions[0].Body.Statements[1].(ast.AssignStmt)
	if !ok {
		t.Fatalf("expected AssignStmt, got %T", file.Functions[0].Body.Statements[1])
	}
	if assignStmt.Name != "x" {
		t.Fatalf("expected assignment target x, got %q", assignStmt.Name)
	}
}

func TestBuildFileParsesSwitchExpression(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int { let x = switch 1 { case 0 => 10 case 1 => 20 else => 30 } return x }")
	letStmt := file.Functions[0].Body.Statements[0].(ast.LetStmt)
	switchExpr, ok := letStmt.Value.(ast.SwitchExpr)
	if !ok {
		t.Fatalf("expected SwitchExpr, got %T", letStmt.Value)
	}
	if len(switchExpr.Cases) != 2 {
		t.Fatalf("expected 2 cases, got %d", len(switchExpr.Cases))
	}
}

func TestBuildFileParsesConditionSwitchExpression(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int { let x = switch { case true => 1 else => 2 } return x }")
	letStmt := file.Functions[0].Body.Statements[0].(ast.LetStmt)
	switchExpr, ok := letStmt.Value.(ast.SwitchExpr)
	if !ok {
		t.Fatalf("expected SwitchExpr, got %T", letStmt.Value)
	}
	if switchExpr.Subject != nil {
		t.Fatalf("expected nil subject for condition switch, got %T", switchExpr.Subject)
	}
	if len(switchExpr.Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(switchExpr.Cases))
	}
}

func TestBuildFileRejectsMissingReturnType(t *testing.T) {
	assertParseErrorContains(t, "fn Main() { return 0 }", "expected arrow before return type")
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

func TestBuildFileParsesFieldAssignment(t *testing.T) {
	file := parseSource(t, "record Point { X: Int }\nfn Main() -> Int { var board = Point { X: 1 } board.X = 2 return board.X }")
	assignStmt, ok := file.Functions[0].Body.Statements[1].(ast.FieldAssignStmt)
	if !ok {
		t.Fatalf("expected FieldAssignStmt, got %T", file.Functions[0].Body.Statements[1])
	}
	if assignStmt.Target != "board" {
		t.Fatalf("expected assignment target board, got %q", assignStmt.Target)
	}
	if assignStmt.Field != "X" {
		t.Fatalf("expected assignment field X, got %q", assignStmt.Field)
	}
}

func TestBuildFileParsesNestedIndexAssignmentTarget(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int { var xs = [[1, 2]] xs[0][0] = 3 return 0 }")
	assignStmt, ok := file.Functions[0].Body.Statements[1].(ast.IndexAssignStmt)
	if !ok {
		t.Fatalf("expected IndexAssignStmt, got %T", file.Functions[0].Body.Statements[1])
	}
	if assignStmt.Target != "xs" {
		t.Fatalf("expected assignment target xs, got %q", assignStmt.Target)
	}
	if got := len(assignStmt.Indices); got != 2 {
		t.Fatalf("expected two indices, got %d", got)
	}
}

func TestBuildFileRejectsInvalidTopLevelContent(t *testing.T) {
	assertParseErrorContains(t, "let x = 1", "expected 'record', 'enum', 'fn', or 'flow' at top level")
}

func TestBuildFileRejectsUnterminatedBlock(t *testing.T) {
	assertParseErrorContains(t, "fn Main() -> Int { return 0", "expected '}' to close block")
}

func TestBuildFileParsesEmptyArrayLiteral(t *testing.T) {
	file := parseSource(t, "fn Main() -> Int[] { return [] }")
	ret := file.Functions[0].Body.Statements[0].(ast.ReturnStmt)
	arrayLiteral, ok := ret.Value.(ast.ArrayLiteralExpr)
	if !ok {
		t.Fatalf("expected ArrayLiteralExpr, got %T", ret.Value)
	}
	if len(arrayLiteral.Elements) != 0 {
		t.Fatalf("expected empty array literal, got %d elements", len(arrayLiteral.Elements))
	}
}

func TestBuildFileParsesNestedArrayTypeSyntax(t *testing.T) {
	file := parseSource(t, "fn Main(grid: Int[][]) -> Int[][] { return grid }")
	fn := file.Functions[0]
	if !fn.Parameters[0].Type.IsArray || fn.Parameters[0].Type.ArrayDepth != 2 {
		t.Fatalf("expected Int[][] parameter type, got %+v", fn.Parameters[0].Type)
	}
	if !fn.ReturnType.IsArray || fn.ReturnType.ArrayDepth != 2 {
		t.Fatalf("expected Int[][] return type, got %+v", fn.ReturnType)
	}
}

func TestBuildFileRejectsMalformedMatch(t *testing.T) {
	assertParseErrorContains(t, "fn Main() -> Int { match Safe() { err(e) => { return 0 } ok(v) => { return v } } }", "expected 'ok' arm")
}

func TestBuildFileParsesSwitchWithoutElse(t *testing.T) {
	file := parseSource(t, "enum Method { Euler Rk4 } fn Main() -> Int { let x = switch Method.Euler { case Method.Euler => 1 case Method.Rk4 => 4 } return x }")
	letStmt := file.Functions[0].Body.Statements[0].(ast.LetStmt)
	switchExpr := letStmt.Value.(ast.SwitchExpr)
	if switchExpr.Else != nil {
		t.Fatalf("expected nil else arm for switch without else")
	}
}

func TestBuildFileRejectsNonLiteralSwitchCase(t *testing.T) {
	assertParseErrorContains(t, "fn Main() -> Int { let x = switch 1 { case 1 + 1 => 2 else => 3 } return x }", "expected arrow after case label")
}

func TestBuildFileParsesFactInOctest(t *testing.T) {
	file := parseSourceWithPath(t, "example.octest", "package Main\n[Fact]\nfn Works() -> Void { return }\n")
	if !file.IsTest {
		t.Fatal("expected .octest to mark test file")
	}
	if len(file.Functions) != 1 || !file.Functions[0].IsFact {
		t.Fatalf("expected one [Fact] function, got %+v", file.Functions)
	}
}

func TestBuildFileRejectsInvalidFactUsage(t *testing.T) {
	assertParseErrorContains(t, "package Main\n[Fact]\nfn Nope() -> Int { return 0 }\n", "[Fact] is only valid in .octest files")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Fact]\nrecord R { X: Int }\n", "test attributes must apply to a function declaration")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Fact]\n[Fact]\nfn Dup() -> Void { return }\n", "duplicate [Fact] attribute on function")
}

func TestBuildFileParsesTheoryInlineDataInOrder(t *testing.T) {
	file := parseSourceWithPath(t, "example.octest", "package Main\n[Theory]\n[InlineData(1, 2)]\n[InlineData(3, 4)]\nfn Sum(a: Int, b: Int) -> Void { return }\n")
	if len(file.Functions) != 1 {
		t.Fatalf("expected one function, got %d", len(file.Functions))
	}
	function := file.Functions[0]
	if !function.IsTheory || function.IsFact {
		t.Fatalf("expected theory metadata, got %+v", function)
	}
	if len(function.InlineData) != 2 {
		t.Fatalf("expected two inline data rows, got %d", len(function.InlineData))
	}
	firstA := function.InlineData[0].Values[0].(ast.IntegerLiteral).Value
	secondA := function.InlineData[1].Values[0].(ast.IntegerLiteral).Value
	if firstA != "1" || secondA != "3" {
		t.Fatalf("expected inline data order preserved, got %q then %q", firstA, secondA)
	}
}

func TestBuildFileRejectsInvalidTheoryUsage(t *testing.T) {
	assertParseErrorContains(t, "package Main\n[Theory]\nfn Nope(x: Int) -> Void { return }\n", "[Theory] is only valid in .octest files")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Theory]\nfn Bad() -> Void { return }\n", "[Theory] function must declare at least one parameter")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Theory]\n[InlineData(1)]\nfn Bad(x: Int) -> Int { return x }\n", "[Theory] function must return Void")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Theory]\n[Theory]\n[InlineData(1)]\nfn Bad(x: Int) -> Void { return }\n", "duplicate [Theory] attribute on function")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Fact]\n[Theory]\n[InlineData(1)]\nfn Bad(x: Int) -> Void { return }\n", "[Fact] and [Theory] cannot both apply to the same function")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Theory]\nfn MissingData(x: Int) -> Void { return }\n", "[Theory] function must declare at least one [InlineData] row")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[InlineData(1)]\nfn NotTheory(x: Int) -> Void { return }\n", "[InlineData] must apply to a [Theory] function")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[InlineData([1])]\n[Theory]\nfn NotAllowed(x: Int[]) -> Void { return }\n", "[InlineData] supports only scalar literals and enum values in M24b")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[CycleTime(1.0s)]\n[Fact]\nfn Bad() -> Void { return }\n", "[CycleTime] is only valid on [Theory] functions")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[CycleTime(1.0s)]\nfn Bad() -> Void { return }\n", "[CycleTime] must apply to a [Theory] function")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[CycleTime(1.0s)]\n[CycleTime(2.0s)]\n[Theory]\n[InlineData(1)]\nfn Bad(x: Int) -> Void { return }\n", "duplicate [CycleTime] attribute on function")
}

func TestBuildFileParsesArtifactInOctest(t *testing.T) {
	file := parseSourceWithPath(t, "example.octest", "package Main\n[Artifact]\nfn Generate() -> Void { return }\n")
	if len(file.Functions) != 1 || !file.Functions[0].IsArtifact {
		t.Fatalf("expected one [Artifact] function, got %+v", file.Functions)
	}
}

func TestBuildFileRejectsInvalidArtifactUsage(t *testing.T) {
	assertParseErrorContains(t, "package Main\n[Artifact]\nfn Nope() -> Void { return }\n", "[Artifact] is only valid in .octest files")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Artifact]\nrecord R { X: Int }\n", "test attributes must apply to a function declaration")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Artifact]\n[Artifact]\nfn Dup() -> Void { return }\n", "duplicate [Artifact] attribute on function")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Artifact]\nfn WrongReturn() -> Int { return 1 }\n", "[Artifact] function must return Void")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Artifact]\nfn NeedsNoArgs(x: Int) -> Void { return }\n", "[Artifact] function must not declare parameters")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Artifact]\n[Fact]\nfn Bad() -> Void { return }\n", "[Artifact] cannot be combined with [Fact]")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Artifact]\n[Theory]\n[InlineData(1)]\nfn Bad(x: Int) -> Void { return }\n", "[Artifact] cannot be combined with [Theory]")
}

func TestBuildFileParsesBenchmarkInOctest(t *testing.T) {
	file := parseSourceWithPath(t, "example.octest", "package Main\n[Benchmark]\nfn Bench() -> Void { return }\n")
	if len(file.Functions) != 1 || !file.Functions[0].IsBenchmark {
		t.Fatalf("expected one [Benchmark] function, got %+v", file.Functions)
	}
}

func TestBuildFileParsesPrometheusBlockInBenchmark(t *testing.T) {
	file := parseSourceWithPath(t, "example.octest", "package Main\n[Benchmark]\nfn Bench() -> Void { let a = Matrix.fill(2, 2, 3.0) let b = Matrix.fill(2, 2, 5.0) PROMETHEUS { let _ = a @ b } }\n")
	if len(file.Functions) != 1 {
		t.Fatalf("expected one function, got %+v", file.Functions)
	}
	if len(file.Functions[0].Body.Statements) != 3 {
		t.Fatalf("expected benchmark setup + PROMETHEUS block, got %+v", file.Functions[0].Body.Statements)
	}
	if _, ok := file.Functions[0].Body.Statements[2].(ast.PrometheusStmt); !ok {
		t.Fatalf("expected PROMETHEUS block statement, got %T", file.Functions[0].Body.Statements[2])
	}
}

func TestBuildFileAttachesDocCommentsToFunction(t *testing.T) {
	file := parseSource(t, "/// Computes answer.\n/// Returns: constant answer.\nfn Main() -> Int { return 42 }")
	doc := file.Functions[0].Doc
	if doc == nil {
		t.Fatal("expected function doc comment")
	}
	if len(doc.Lines) != 2 || doc.Lines[0] != "Computes answer." {
		t.Fatalf("unexpected doc lines: %+v", doc.Lines)
	}
	if len(doc.Structured) != 1 || doc.Structured[0].Keyword != "Returns" || doc.Structured[0].Text != "constant answer." {
		t.Fatalf("unexpected structured doc sections: %+v", doc.Structured)
	}
}

func TestBuildFileAttachesDocCommentsToRecordAndField(t *testing.T) {
	file := parseSource(t, "/// Planar point.\nrecord Point {\n/// X-axis coordinate.\nX: Float<m>\n/// Y-axis coordinate.\nY: Float<m>\n}")
	record := file.Records[0]
	if record.Doc == nil || record.Doc.Lines[0] != "Planar point." {
		t.Fatalf("expected record doc comment, got %+v", record.Doc)
	}
	if record.Fields[0].Doc == nil || record.Fields[0].Doc.Lines[0] != "X-axis coordinate." {
		t.Fatalf("expected field doc on X, got %+v", record.Fields[0].Doc)
	}
	if record.Fields[1].Doc == nil || record.Fields[1].Doc.Lines[0] != "Y-axis coordinate." {
		t.Fatalf("expected field doc on Y, got %+v", record.Fields[1].Doc)
	}
}

func TestBuildFileAttachesDocCommentsToEnum(t *testing.T) {
	file := parseSource(t, "/// Physical mode.\nenum Mode { Idle Active }")
	if file.Enums[0].Doc == nil || file.Enums[0].Doc.Lines[0] != "Physical mode." {
		t.Fatalf("expected enum doc comment, got %+v", file.Enums[0].Doc)
	}
}

func TestBuildFileDocCommentRequiresNoBlankLineBeforeDeclaration(t *testing.T) {
	file := parseSource(t, "/// Should not attach.\n\nfn Main() -> Int { return 0 }")
	if file.Functions[0].Doc != nil {
		t.Fatalf("expected nil function doc after blank line, got %+v", file.Functions[0].Doc)
	}
}

func TestBuildFileTreatsOrdinaryCommentsAsNonDocComments(t *testing.T) {
	file := parseSource(t, "// not docs\nfn Main() -> Int { return 0 }")
	if file.Functions[0].Doc != nil {
		t.Fatalf("expected nil doc comment for ordinary // comment, got %+v", file.Functions[0].Doc)
	}
}

func TestBuildFileParsesStructuredDocCommentKeywords(t *testing.T) {
	file := parseSource(t, "/// Computes stress.\n/// Param force: Applied force.\n/// Param area: Cross-section.\n/// Returns: Stress value.\n/// Units: force [N], area [m^2], result [Pa].\n/// Remarks: Linear-elastic only.\n/// Example: NormalStress(100, 2).\nfn NormalStress(force: Float<kg*m/s^2>, area: Float<m^2>) -> Float<kg/m/s^2> { return force / area }")
	doc := file.Functions[0].Doc
	if doc == nil {
		t.Fatal("expected function doc comment")
	}
	if len(doc.Structured) != 6 {
		t.Fatalf("expected 6 structured sections, got %d (%+v)", len(doc.Structured), doc.Structured)
	}
	if doc.Structured[0].Keyword != "Param" || doc.Structured[0].Target != "force" || doc.Structured[0].Text != "Applied force." {
		t.Fatalf("unexpected first structured section: %+v", doc.Structured[0])
	}
	if doc.Structured[5].Keyword != "Example" {
		t.Fatalf("expected Example section, got %+v", doc.Structured[5])
	}
}

func TestBuildFileRejectsInvalidBenchmarkUsage(t *testing.T) {
	assertParseErrorContains(t, "package Main\n[Benchmark]\nfn Nope() -> Void { return }\n", "[Benchmark] is only valid in .octest files")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Benchmark]\nrecord R { X: Int }\n", "test attributes must apply to a function declaration")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Benchmark]\n[Benchmark]\nfn Dup() -> Void { return }\n", "duplicate [Benchmark] attribute on function")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Benchmark]\nfn WrongReturn() -> Int { return 1 }\n", "[Benchmark] function must return Void")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Benchmark]\nfn NeedsNoArgs(x: Int) -> Void { return }\n", "[Benchmark] function must not declare parameters")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Benchmark]\n[Fact]\nfn Bad() -> Void { return }\n", "[Benchmark] cannot be combined with [Fact]")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Benchmark]\n[Theory]\n[InlineData(1)]\nfn Bad(x: Int) -> Void { return }\n", "[Benchmark] cannot be combined with [Theory]")
	assertParseErrorContainsWithPath(t, "bad.octest", "package Main\n[Benchmark]\n[Artifact]\nfn Bad() -> Void { return }\n", "[Benchmark] cannot be combined with [Artifact]")
}

func parseSource(t *testing.T, text string) ast.File {
	return parseSourceWithPath(t, "example.oct", text)
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

	file, err := BuildFile(lexed)
	if err != nil {
		t.Fatalf("BuildFile returned error: %v", err)
	}
	return file
}

func assertParseErrorContains(t *testing.T, text string, want string) {
	assertParseErrorContainsWithPath(t, "example.oct", text, want)
}

func assertParseErrorContainsWithPath(t *testing.T, path string, text string, want string) {
	t.Helper()
	if !strings.HasPrefix(strings.TrimSpace(text), "package ") && !strings.Contains(want, "package declaration") {
		text = "package Main\n" + text
	}

	lexed, err := lex.Analyze(source.File{Path: path, Text: text})
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

func TestBuildFileParsesDimensionQualifiedTypesAndLiterals(t *testing.T) {
	file := parseSource(t, "fn Speed(distance: Float<m>, time: Float<s>) -> Float<m/s> { return 10m/s }")

	fn := file.Functions[0]
	if got := fn.Parameters[0].Type.Dimension.String(); got != "m" {
		t.Fatalf("expected first parameter dimension m, got %q", got)
	}
	if got := fn.Parameters[1].Type.Dimension.String(); got != "s" {
		t.Fatalf("expected second parameter dimension s, got %q", got)
	}
	if got := fn.ReturnType.Dimension.String(); got != "m/s" {
		t.Fatalf("expected return dimension m/s, got %q", got)
	}

	ret := fn.Body.Statements[0].(ast.ReturnStmt)
	literal, ok := ret.Value.(ast.IntegerLiteral)
	if !ok {
		t.Fatalf("expected FloatLiteral, got %T", ret.Value)
	}
	if got := literal.Dimension.String(); got != "m/s" {
		t.Fatalf("expected literal dimension m/s, got %q", got)
	}
}

func TestBuildFileParsesSpaceSeparatedUnitSuffixes(t *testing.T) {
	file := parseSource(t, "fn Main() -> Float<px> { return 320 px }")
	fn := file.Functions[0]
	ret := fn.Body.Statements[0].(ast.ReturnStmt)
	literal, ok := ret.Value.(ast.IntegerLiteral)
	if !ok {
		t.Fatalf("expected IntegerLiteral, got %T", ret.Value)
	}
	if got := literal.Dimension.String(); got != "px" {
		t.Fatalf("expected literal dimension px, got %q", got)
	}
}

func TestBuildFileRejectsUnknownUnitInType(t *testing.T) {
	assertParseErrorContains(t, "fn Main() -> Int<foo> { return 1 }", "unknown base unit: foo")
}

func TestBuildFileRejectsMalformedUnitSuffix(t *testing.T) {
	assertParseErrorContains(t, "fn Main() -> Int<m> { return 1m/ }", "expected expression")
}

func TestBuildFileParsesDegreeLiteralAsDimensionlessRadians(t *testing.T) {
	file := parseSource(t, "fn Main() -> Float { return 180deg }")
	fn := file.Functions[0]
	ret := fn.Body.Statements[0].(ast.ReturnStmt)
	literal, ok := ret.Value.(ast.FloatLiteral)
	if !ok {
		t.Fatalf("expected FloatLiteral, got %T", ret.Value)
	}
	if literal.Dimension.String() != "" {
		t.Fatalf("expected dimensionless literal, got %q", literal.Dimension.String())
	}
	if literal.Value != "3.141592653589793" {
		t.Fatalf("expected radian value, got %q", literal.Value)
	}
}

func TestBuildFileParsesCelsiusLiteralAsKelvinFloat(t *testing.T) {
	file := parseSource(t, "fn Main() -> Float<K> { return 100C }")
	fn := file.Functions[0]
	ret := fn.Body.Statements[0].(ast.ReturnStmt)
	literal, ok := ret.Value.(ast.FloatLiteral)
	if !ok {
		t.Fatalf("expected FloatLiteral, got %T", ret.Value)
	}
	if literal.Dimension.String() != "K" {
		t.Fatalf("expected kelvin literal, got %q", literal.Dimension.String())
	}
	if literal.Value != "373.15" {
		t.Fatalf("expected kelvin converted value, got %q", literal.Value)
	}
}

func TestBuildFileRejectsCelsiusInDimensionExpressions(t *testing.T) {
	assertParseErrorContains(t, "fn Main() -> Float<K/m> { return 20C/m }", "unknown base unit: C")
}

func TestBuildFileParsesM16VectorMatrixSyntax(t *testing.T) {
	file := parseSource(t, "fn Main(v: Vector<Int>, m: Matrix<Float<m>>) -> Int { return m[0, 0] + v[0] }")
	fn := file.Functions[0]
	if fn.Parameters[0].Type.VectorOf == nil {
		t.Fatalf("expected Vector type, got %+v", fn.Parameters[0].Type)
	}
	if fn.Parameters[1].Type.MatrixOf == nil {
		t.Fatalf("expected Matrix type, got %+v", fn.Parameters[1].Type)
	}
	ret := fn.Body.Statements[0].(ast.ReturnStmt)
	sum := ret.Value.(ast.BinaryExpr)
	left := sum.Left.(ast.IndexExpr)
	if len(left.Indices) != 2 {
		t.Fatalf("expected matrix index to have 2 indices, got %d", len(left.Indices))
	}
	right := sum.Right.(ast.IndexExpr)
	if len(right.Indices) != 1 {
		t.Fatalf("expected vector index to have 1 index, got %d", len(right.Indices))
	}
}

func TestBuildFileParsesFlowStateGotoSuspend(t *testing.T) {
	file := parseSource(t, "flow Patrol(input: Int) -> Int { state Search { if input == 0 { goto Track } suspend } state Track { return input } }")
	if len(file.Flows) != 1 {
		t.Fatalf("expected one flow, got %d", len(file.Flows))
	}
	flow := file.Flows[0]
	if flow.Name != "Patrol" {
		t.Fatalf("expected flow Patrol, got %q", flow.Name)
	}
	if flow.EntryState != "Search" {
		t.Fatalf("expected entry state Search, got %q", flow.EntryState)
	}
	if len(flow.States) != 2 {
		t.Fatalf("expected two states, got %d", len(flow.States))
	}
	if _, ok := flow.States[0].Body.Statements[1].(ast.SuspendStmt); !ok {
		t.Fatalf("expected suspend statement in Search state, got %T", flow.States[0].Body.Statements[1])
	}
}

func TestBuildFileParsesWhenInFlowState(t *testing.T) {
	file := parseSource(t, "flow Patrol(input: Int) -> Int { state Search { when { case input == 0 -> goto Track case input < 0 -> return 0 else -> suspend } } state Track { return input } }")
	whenStmt, ok := file.Flows[0].States[0].Body.Statements[0].(ast.WhenStmt)
	if !ok {
		t.Fatalf("expected when statement, got %T", file.Flows[0].States[0].Body.Statements[0])
	}
	if len(whenStmt.Cases) != 2 {
		t.Fatalf("expected two when cases, got %d", len(whenStmt.Cases))
	}
	if _, ok := whenStmt.Cases[0].Action.(ast.WhenGotoAction); !ok {
		t.Fatalf("expected first when action goto, got %T", whenStmt.Cases[0].Action)
	}
	if _, ok := whenStmt.Cases[1].Action.(ast.WhenReturnAction); !ok {
		t.Fatalf("expected second when action return, got %T", whenStmt.Cases[1].Action)
	}
	if _, ok := whenStmt.Else.(ast.WhenSuspendAction); !ok {
		t.Fatalf("expected else action suspend, got %T", whenStmt.Else)
	}
}

func TestBuildFileParsesFlowBoardDeclaration(t *testing.T) {
	file := parseSource(t, "flow Patrol(input: Bool) -> Int { board { FaultLatched: Bool Cooldown: Int } state Start { if input { board.FaultLatched = true } return board.Cooldown } }")
	flow := file.Flows[0]
	if len(flow.Board) != 2 {
		t.Fatalf("expected 2 board fields, got %d", len(flow.Board))
	}
	if flow.Board[0].Name != "FaultLatched" || flow.Board[0].Type.Name != "Bool" {
		t.Fatalf("unexpected first board field: %#v", flow.Board[0])
	}
}

func TestBuildFileParsesWhenBlockAction(t *testing.T) {
	file := parseSource(t, "flow Patrol(flag: Bool) -> Int { state Search { when { case flag -> { remember goto Track } else -> { suspend } } } state Track { return 1 } }")
	whenStmt, ok := file.Flows[0].States[0].Body.Statements[0].(ast.WhenStmt)
	if !ok {
		t.Fatalf("expected when statement, got %T", file.Flows[0].States[0].Body.Statements[0])
	}
	caseAction, ok := whenStmt.Cases[0].Action.(ast.WhenBlockAction)
	if !ok {
		t.Fatalf("expected block action, got %T", whenStmt.Cases[0].Action)
	}
	if len(caseAction.Statements) != 2 {
		t.Fatalf("expected 2 statements in when block action, got %d", len(caseAction.Statements))
	}
	if _, ok := caseAction.Statements[0].(ast.RememberStmt); !ok {
		t.Fatalf("expected remember statement first in when block, got %T", caseAction.Statements[0])
	}
}

func TestBuildFileParsesRememberResumeInFlowState(t *testing.T) {
	file := parseSource(t, "flow Patrol(input: Int) -> Int { state Search { remember goto Track } state Track { if input > 0 { resume } return input } }")
	if len(file.Flows) != 1 {
		t.Fatalf("expected one flow, got %d", len(file.Flows))
	}
	searchStatements := file.Flows[0].States[0].Body.Statements
	if _, ok := searchStatements[0].(ast.RememberStmt); !ok {
		t.Fatalf("expected remember statement in Search state, got %T", searchStatements[0])
	}
	if _, ok := searchStatements[1].(ast.GotoStmt); !ok {
		t.Fatalf("expected goto statement in Search state, got %T", searchStatements[1])
	}

	trackIf, ok := file.Flows[0].States[1].Body.Statements[0].(ast.IfStmt)
	if !ok {
		t.Fatalf("expected if statement in Track state, got %T", file.Flows[0].States[1].Body.Statements[0])
	}
	if _, ok := trackIf.ThenBody.Statements[0].(ast.ResumeStmt); !ok {
		t.Fatalf("expected resume statement in Track if branch, got %T", trackIf.ThenBody.Statements[0])
	}
}

func TestBuildFileParsesUtilityWhenExpressionInState(t *testing.T) {
	file := parseSource(t, "flow Patrol(threat: Bool, flank: Bool) -> Int { state Search { let next = when policy { hysteresis: 3 min_commit: 2 } { case 1 when threat score 100 case 2 when flank score 80 else 0 } return next } }")
	letStmt, ok := file.Flows[0].States[0].Body.Statements[0].(ast.LetStmt)
	if !ok {
		t.Fatalf("expected let statement, got %T", file.Flows[0].States[0].Body.Statements[0])
	}
	whenExpr, ok := letStmt.Value.(ast.UtilityWhenExpr)
	if !ok {
		t.Fatalf("expected utility when expression, got %T", letStmt.Value)
	}
	if len(whenExpr.Cases) != 2 {
		t.Fatalf("expected 2 utility when cases, got %d", len(whenExpr.Cases))
	}
	if whenExpr.Else == nil {
		t.Fatal("expected utility when else arm")
	}
	if !whenExpr.ControllerBound {
		t.Fatal("expected when policy to be controller bound")
	}
}

func TestBuildFileParsesStandaloneUtilityWhenWithDefaults(t *testing.T) {
	file := parseSource(t, "fn Main(flag: Bool) -> Int { return when utility { case 1 when flag score 10 else 0 } }")
	whenExpr, ok := file.Functions[0].Body.Statements[0].(ast.ReturnStmt)
	if !ok {
		t.Fatalf("expected return statement, got %T", file.Functions[0].Body.Statements[0])
	}
	value, ok := whenExpr.Value.(ast.UtilityWhenExpr)
	if !ok {
		t.Fatalf("expected utility when expression, got %T", whenExpr.Value)
	}
	if value.ControllerBound {
		t.Fatal("expected when utility to be expression bound")
	}
}

func TestBuildFileParsesRecordTableCellSchema(t *testing.T) {
	file := parseSource(t, "record table Measurements { Stage: String Samples: Float[] }")
	if len(file.Records) != 1 || !file.Records[0].IsTable {
		t.Fatalf("expected one record table, got %#v", file.Records)
	}
	if got := file.Records[0].Fields[1].Type.ArrayDepth; got != 1 {
		t.Fatalf("expected Samples cell type to retain one declared array depth, got %d", got)
	}
}

func TestBuildFileParsesStandaloneUtilityWhenWithExplicitPolicy(t *testing.T) {
	file := parseSource(t, "fn Main(flag: Bool) -> Int { return when utility { hysteresis: 5 } { case 1 when flag score 10 else 0 } }")
	returnStmt, ok := file.Functions[0].Body.Statements[0].(ast.ReturnStmt)
	if !ok {
		t.Fatalf("expected return statement, got %T", file.Functions[0].Body.Statements[0])
	}
	whenExpr, ok := returnStmt.Value.(ast.UtilityWhenExpr)
	if !ok {
		t.Fatalf("expected utility when expression, got %T", returnStmt.Value)
	}
	if _, ok := whenExpr.Policy.Hysteresis.(ast.IntegerLiteral); !ok {
		t.Fatalf("expected explicit hysteresis policy literal, got %T", whenExpr.Policy.Hysteresis)
	}
	if _, ok := whenExpr.Policy.MinCommit.(ast.IntegerLiteral); !ok {
		t.Fatalf("expected default min_commit literal, got %T", whenExpr.Policy.MinCommit)
	}
}

func TestBuildFileParsesUtilityWhenWithoutElseForTypecheckDiagnostic(t *testing.T) {
	file := parseSource(t, "flow Patrol(flag: Bool) -> Int { state Search { let x = when policy { hysteresis: 1 min_commit: 1 } { case 1 when flag score 10 } return x } }")
	letStmt, ok := file.Flows[0].States[0].Body.Statements[0].(ast.LetStmt)
	if !ok {
		t.Fatalf("expected let statement, got %T", file.Flows[0].States[0].Body.Statements[0])
	}
	whenExpr, ok := letStmt.Value.(ast.UtilityWhenExpr)
	if !ok {
		t.Fatalf("expected utility when expression, got %T", letStmt.Value)
	}
	if whenExpr.Else != nil {
		t.Fatal("expected missing else to remain nil for typecheck diagnostic")
	}
}

func TestBuildFileParsesEnumTargetedUtilityWhen(t *testing.T) {
	file := parseSource(t, "enum Decision { Hold Run(Int) Fault(String) } fn Main(flag: Bool) -> Decision { return when utility Decision { case Decision.Run(3) when flag score 10 else Decision.Fault(\"fallback\") } }")
	returnStmt, ok := file.Functions[0].Body.Statements[0].(ast.ReturnStmt)
	if !ok {
		t.Fatalf("expected return statement, got %T", file.Functions[0].Body.Statements[0])
	}
	whenExpr, ok := returnStmt.Value.(ast.UtilityWhenExpr)
	if !ok {
		t.Fatalf("expected utility when expression, got %T", returnStmt.Value)
	}
	if whenExpr.EnumTarget == nil || whenExpr.EnumTarget.Name != "Decision" {
		t.Fatalf("expected enum target Decision, got %#v", whenExpr.EnumTarget)
	}
	if len(whenExpr.Cases) != 1 || whenExpr.Else == nil {
		t.Fatalf("expected one case and else, got %d and %v", len(whenExpr.Cases), whenExpr.Else)
	}
	caseCall, ok := whenExpr.Cases[0].Value.(ast.CallExpr)
	if !ok || len(caseCall.Arguments) != 1 {
		t.Fatalf("expected payload case constructor, got %T", whenExpr.Cases[0].Value)
	}
	elseCall, ok := whenExpr.Else.(ast.CallExpr)
	if !ok || len(elseCall.Arguments) != 1 {
		t.Fatalf("expected payload else constructor, got %T", whenExpr.Else)
	}

}

func TestBuildFileRejectsMalformedFlowState(t *testing.T) {
	assertParseErrorContains(t, "flow Patrol() -> Int { let x = 1 }", "expected 'state' declaration inside flow")
}

func TestBuildFileParsesContextualKeywordsAsIdentifiers(t *testing.T) {
	file := parseSource(t, "record Example { state: Int step: Int flow: Int }\nfn Echo(state: Int, step: Int, flow: Int) -> Int { let state = state let step = step let flow = flow let xs = 1..10 step 2 let ys = batch [1, 2, 3] as state { return state + 1 } return flow + step + state + xs[0] + ys[0] }\nflow Idle() -> Void { state Start { suspend } }")
	if len(file.Records) != 1 || len(file.Records[0].Fields) != 3 {
		t.Fatalf("expected contextual-keyword record fields, got %+v", file.Records)
	}
	if file.Records[0].Fields[0].Name != "state" || file.Records[0].Fields[1].Name != "step" || file.Records[0].Fields[2].Name != "flow" {
		t.Fatalf("unexpected record field names: %+v", file.Records[0].Fields)
	}
	if len(file.Functions) != 1 || len(file.Functions[0].Parameters) != 3 {
		t.Fatalf("expected contextual-keyword parameters, got %+v", file.Functions)
	}
	if file.Functions[0].Parameters[0].Name != "state" || file.Functions[0].Parameters[1].Name != "step" || file.Functions[0].Parameters[2].Name != "flow" {
		t.Fatalf("unexpected parameter names: %+v", file.Functions[0].Parameters)
	}
	if len(file.Flows) != 1 || len(file.Flows[0].States) != 1 || file.Flows[0].States[0].Name != "Start" {
		t.Fatalf("expected unchanged flow/state parse, got %+v", file.Flows)
	}
}

func TestBuildFileParsesContinueAndBreakAsNonStatementIdentifiers(t *testing.T) {
	file := parseSource(t, "record Example { continue: Int break: Int }\nfn Echo(continue: Int, break: Int) -> Int { let continue = continue let break = break return continue + break }")
	if len(file.Records) != 1 || len(file.Records[0].Fields) != 2 {
		t.Fatalf("expected continue/break record fields, got %+v", file.Records)
	}
	if file.Records[0].Fields[0].Name != "continue" || file.Records[0].Fields[1].Name != "break" {
		t.Fatalf("unexpected record field names: %+v", file.Records[0].Fields)
	}
	if len(file.Functions) != 1 || len(file.Functions[0].Parameters) != 2 {
		t.Fatalf("expected continue/break parameters, got %+v", file.Functions)
	}
	if file.Functions[0].Parameters[0].Name != "continue" || file.Functions[0].Parameters[1].Name != "break" {
		t.Fatalf("unexpected parameter names: %+v", file.Functions[0].Parameters)
	}
}

func TestBuildFileParsesFirstClassRangeExpressionForms(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		hasStart bool
		hasEnd   bool
		hasStep  bool
	}{
		{name: "closed", source: "fn Main() -> Int { let r = 1..3 return 0 }", hasStart: true, hasEnd: true},
		{name: "open end", source: "fn Main() -> Int { let r = 1.. return 0 }", hasStart: true},
		{name: "open start", source: "fn Main() -> Int { let r = ..3 return 0 }", hasEnd: true},
		{name: "all open", source: "fn Main() -> Int { let r = .. return 0 }"},
		{name: "closed stepped", source: "fn Main() -> Int { let r = 1..10 step 2 return 0 }", hasStart: true, hasEnd: true, hasStep: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			file := parseSource(t, tc.source)
			letStmt, ok := file.Functions[0].Body.Statements[0].(ast.LetStmt)
			if !ok {
				t.Fatalf("expected first statement to be LetStmt, got %T", file.Functions[0].Body.Statements[0])
			}
			rangeExpr, ok := letStmt.Value.(ast.RangeExpr)
			if !ok {
				t.Fatalf("expected let value to be RangeExpr, got %T", letStmt.Value)
			}
			if (rangeExpr.Start != nil) != tc.hasStart {
				t.Fatalf("expected hasStart=%v, got start %#v", tc.hasStart, rangeExpr.Start)
			}
			if (rangeExpr.End != nil) != tc.hasEnd {
				t.Fatalf("expected hasEnd=%v, got end %#v", tc.hasEnd, rangeExpr.End)
			}
			if (rangeExpr.Step != nil) != tc.hasStep {
				t.Fatalf("expected hasStep=%v, got step %#v", tc.hasStep, rangeExpr.Step)
			}
		})
	}
}

func TestBuildFileRejectsOpenEndedSteppedRangeForms(t *testing.T) {
	assertParseErrorContains(t, "fn Main() -> Int { let r = 1.. step 2 return 0 }", "open-ended stepped ranges are not supported in M0")
	assertParseErrorContains(t, "fn Main() -> Int { let r = ..10 step 2 return 0 }", "open-ended stepped ranges are not supported in M0")
	assertParseErrorContains(t, "fn Main() -> Int { let r = .. step 2 return 0 }", "open-ended stepped ranges are not supported in M0")
}

func TestBuildFileParsesMakeAttributesInMakeOct(t *testing.T) {
	file := parseSourceWithPath(t, "Make.oct", `package Main
import Make
[MakePlan]
[Pure]
[NoWhile]
fn Plan() -> Make.Plan { return Make.Plan { Default: "All" Config: Make.DefaultConfig() CommandTargets: [] FunctionTargets: [] FlowTargets: [] PhonyTargets: [] } }
[RequiresAuthority]
fn CheckTools() -> Int ! Error { return 0 }
`)
	if !file.IsMakeFile {
		t.Fatalf("expected Make.oct file flag")
	}
	if len(file.Functions) != 2 {
		t.Fatalf("expected two functions, got %d", len(file.Functions))
	}
	plan := file.Functions[0]
	if !plan.IsMakeFile || !plan.IsMakePlan || !plan.IsMakePure || !plan.IsMakeNoWhile || plan.RequiresMakeAuthority {
		t.Fatalf("unexpected Plan make attributes: %+v", plan)
	}
	check := file.Functions[1]
	if !check.RequiresMakeAuthority || check.IsMakePlan || check.IsMakePure || check.IsMakeNoWhile {
		t.Fatalf("unexpected CheckTools make attributes: %+v", check)
	}
}

func TestBuildFileParsesConventionalUnmarkedMakePlan(t *testing.T) {
	file := parseSourceWithPath(t, "Make.oct", `package Main
import Make
fn Plan() -> Make.Plan { return Make.Plan { Default: "All" Config: Make.DefaultConfig() CommandTargets: [] FunctionTargets: [] FlowTargets: [] PhonyTargets: [] } }
`)
	if len(file.Functions) != 1 || !file.Functions[0].IsMakeFile || file.Functions[0].IsMakePlan {
		t.Fatalf("unexpected conventional Plan flags: %+v", file.Functions)
	}
}

func TestBuildFileRejectsInvalidMakeAttributes(t *testing.T) {
	cases := []struct{ name, path, source, want string }{
		{"ordinary oct rejects", "Main.oct", "package Main\n[MakePlan]\nfn Plan() -> Int { return 0 }\n", "[MakePlan] is only valid in .octest files or Make.oct"},
		{"octest rejects make", "x.octest", "package Main\n[MakePlan]\nfn Plan() -> Void { return }\n", "unsupported attribute [MakePlan]"},
		{"make rejects octest", "Make.oct", "package Main\n[Fact]\nfn Plan() -> Int { return 0 }\n", "unsupported Make attribute [Fact]"},
		{"make rejects unknown", "Make.oct", "package Main\n[Unknown]\nfn Plan() -> Int { return 0 }\n", "unsupported Make attribute [Unknown]"},
		{"record attachment", "Make.oct", "package Main\n[Pure]\nrecord X { Value: Int }\n", "Make attributes must apply to a function declaration"},
		{"enum attachment", "Make.oct", "package Main\n[Pure]\nenum X { A }\n", "Make attributes must apply to a function declaration"},
		{"duplicate", "Make.oct", "package Main\n[Pure]\n[Pure]\nfn X() -> Int { return 0 }\n", "duplicate [Pure] attribute on function"},
		{"pure authority", "Make.oct", "package Main\n[Pure]\n[RequiresAuthority]\nfn X() -> Int { return 0 }\n", "[Pure] and [RequiresAuthority] cannot both apply"},
		{"makeplan params", "Make.oct", "package Main\nimport Make\n[MakePlan]\nfn Plan(x: Int) -> Make.Plan { return Make.Plan { Default: \"All\" Config: Make.DefaultConfig() CommandTargets: [] FunctionTargets: [] FlowTargets: [] PhonyTargets: [] } }\n", "[MakePlan] function must not declare parameters"},
		{"makeplan return", "Make.oct", "package Main\n[MakePlan]\nfn Plan() -> Int { return 0 }\n", "[MakePlan] function must return Make.Plan"},
		{"nowhile", "Make.oct", "package Main\n[NoWhile]\nfn X() -> Int { while true { } return 0 }\n", "[NoWhile] function must not contain while statements"},
		{"payload", "Make.oct", "package Main\n[Pure(\"x\")]\nfn X() -> Int { return 0 }\n", "Make attribute [Pure] does not accept a payload"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertParseErrorContainsWithPath(t, tc.path, tc.source, tc.want)
		})
	}
}
