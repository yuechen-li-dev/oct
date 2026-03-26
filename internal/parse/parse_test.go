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

func TestBuildFileRejectsFieldAssignment(t *testing.T) {
	assertParseErrorContains(t, "record Point { X: Int }\nfn Main() -> Int { let p = Point { X: 1 } p.X = 2 return 0 }", "expected statement")
}

func TestBuildFileRejectsIndexAssignment(t *testing.T) {
	assertParseErrorContains(t, "fn Main() -> Int { let xs = [1, 2] xs[0] = 3 return 0 }", "expected statement")
}

func TestBuildFileRejectsInvalidTopLevelContent(t *testing.T) {
	assertParseErrorContains(t, "let x = 1", "expected 'record', 'enum', or 'fn' at top level")
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

func TestBuildFileParsesSwitchWithoutElse(t *testing.T) {
	file := parseSource(t, "enum Method { Euler Rk4 } fn Main() -> Int { let x = switch Method.Euler { case Method.Euler => 1 case Method.Rk4 => 4 } return x }")
	letStmt := file.Functions[0].Body.Statements[0].(ast.LetStmt)
	switchExpr := letStmt.Value.(ast.SwitchExpr)
	if switchExpr.Else != nil {
		t.Fatalf("expected nil else arm for switch without else")
	}
}

func TestBuildFileRejectsNonLiteralSwitchCase(t *testing.T) {
	assertParseErrorContains(t, "fn Main() -> Int { let x = switch 1 { case 1 + 1 => 2 else => 3 } return x }", "expected '=>' after case label")
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

	file, err := BuildFile(lexed)
	if err != nil {
		t.Fatalf("BuildFile returned error: %v", err)
	}
	return file
}

func assertParseErrorContains(t *testing.T, text string, want string) {
	t.Helper()
	if !strings.HasPrefix(strings.TrimSpace(text), "package ") && !strings.Contains(want, "package declaration") {
		text = "package Main\n" + text
	}

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

func TestBuildFileRejectsUnknownUnitInType(t *testing.T) {
	assertParseErrorContains(t, "fn Main() -> Int<foo> { return 1 }", "unknown base unit: foo")
}

func TestBuildFileRejectsMalformedUnitSuffix(t *testing.T) {
	assertParseErrorContains(t, "fn Main() -> Int<m> { return 1m/ }", "expected expression")
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
