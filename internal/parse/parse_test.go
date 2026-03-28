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
	file := parseSource(t, "fn Main() -> Int ! Error { return LoadOctagon[Int[]](\"x.octagon\")? }")
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
	if _, ok := assignStmt.Index.(ast.IntegerLiteral); !ok {
		t.Fatalf("expected integer index, got %T", assignStmt.Index)
	}
	if _, ok := assignStmt.Value.(ast.IntegerLiteral); !ok {
		t.Fatalf("expected integer value, got %T", assignStmt.Value)
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

func TestBuildFileRejectsNestedIndexAssignmentTarget(t *testing.T) {
	assertParseErrorContains(t, "fn Main() -> Int { let xs = [1, 2] xs[0][0] = 3 return 0 }", "nested index assignment targets are not supported")
}

func TestBuildFileRejectsInvalidTopLevelContent(t *testing.T) {
	assertParseErrorContains(t, "let x = 1", "expected 'record', 'enum', 'fn', or 'flow' at top level")
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

func TestBuildFileRejectsMalformedFlowState(t *testing.T) {
	assertParseErrorContains(t, "flow Patrol() -> Int { let x = 1 }", "expected 'state' declaration inside flow")
}
