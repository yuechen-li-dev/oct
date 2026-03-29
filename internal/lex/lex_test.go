package lex

import (
	"strings"
	"testing"

	"oct/internal/source"
)

func TestAnalyzeTokenizesFrozenSubset(t *testing.T) {
	file := source.File{Path: "example.oct", Text: "fn Main(x: Int) -> Float { let answer = 1 + 2.0 / 3 return false }"}

	result, err := Analyze(file)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	assertTokenKinds(t, result.Tokens,
		KeywordFn,
		Identifier,
		LeftParen,
		Identifier,
		Colon,
		Identifier,
		RightParen,
		Arrow,
		Identifier,
		LeftBrace,
		KeywordLet,
		Identifier,
		Assign,
		IntLiteral,
		Plus,
		FloatLiteral,
		Slash,
		IntLiteral,
		KeywordReturn,
		KeywordFalse,
		RightBrace,
		EOF,
	)
}

func TestAnalyzeTokenizesM6OperatorsAndStrings(t *testing.T) {
	file := source.File{Path: "example.oct", Text: "fn Main() -> Int ! Error { match Fail()? { ok(value) => { return value } err(e) => { return error(\"boom\")! } } }"}

	result, err := Analyze(file)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	assertTokenKinds(t, result.Tokens,
		KeywordFn,
		Identifier,
		LeftParen,
		RightParen,
		Arrow,
		Identifier,
		Bang,
		Identifier,
		LeftBrace,
		KeywordMatch,
		Identifier,
		LeftParen,
		RightParen,
		Question,
		LeftBrace,
		Identifier,
		LeftParen,
		Identifier,
		RightParen,
		FatArrow,
		LeftBrace,
		KeywordReturn,
		Identifier,
		RightBrace,
		Identifier,
		LeftParen,
		Identifier,
		RightParen,
		FatArrow,
		LeftBrace,
		KeywordReturn,
		Identifier,
		LeftParen,
		StringLiteral,
		RightParen,
		Bang,
		RightBrace,
		RightBrace,
		RightBrace,
		EOF,
	)
}

func TestAnalyzeTokenizesRangesAndForLoops(t *testing.T) {
	file := source.File{Path: "example.oct", Text: "fn Main() -> Int { for i in 0..10 step 2 { return i } return 0 }"}

	result, err := Analyze(file)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	assertTokenKinds(t, result.Tokens,
		KeywordFn,
		Identifier,
		LeftParen,
		RightParen,
		Arrow,
		Identifier,
		LeftBrace,
		KeywordFor,
		Identifier,
		KeywordIn,
		IntLiteral,
		DotDot,
		IntLiteral,
		KeywordStep,
		IntLiteral,
		LeftBrace,
		KeywordReturn,
		Identifier,
		RightBrace,
		KeywordReturn,
		IntLiteral,
		RightBrace,
		EOF,
	)
}

func TestAnalyzeTokenizesVarKeyword(t *testing.T) {
	file := source.File{Path: "example.oct", Text: "fn Main() -> Int { var x = 1 x = 2 return x }"}

	result, err := Analyze(file)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	assertTokenKinds(t, result.Tokens,
		KeywordFn,
		Identifier,
		LeftParen,
		RightParen,
		Arrow,
		Identifier,
		LeftBrace,
		KeywordVar,
		Identifier,
		Assign,
		IntLiteral,
		Identifier,
		Assign,
		IntLiteral,
		KeywordReturn,
		Identifier,
		RightBrace,
		EOF,
	)
}

func TestAnalyzeTokenizesIfElseAndSwitch(t *testing.T) {
	file := source.File{Path: "example.oct", Text: "fn Main() -> Int { while false { return 9 } if true { return switch 1 { case 1 => 2 else => 3 } } else { return 0 } }"}

	result, err := Analyze(file)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	assertTokenKinds(t, result.Tokens,
		KeywordFn,
		Identifier,
		LeftParen,
		RightParen,
		Arrow,
		Identifier,
		LeftBrace,
		KeywordWhile,
		KeywordFalse,
		LeftBrace,
		KeywordReturn,
		IntLiteral,
		RightBrace,
		KeywordIf,
		KeywordTrue,
		LeftBrace,
		KeywordReturn,
		KeywordSwitch,
		IntLiteral,
		LeftBrace,
		KeywordCase,
		IntLiteral,
		FatArrow,
		IntLiteral,
		KeywordElse,
		FatArrow,
		IntLiteral,
		RightBrace,
		RightBrace,
		KeywordElse,
		LeftBrace,
		KeywordReturn,
		IntLiteral,
		RightBrace,
		RightBrace,
		EOF,
	)
}

func TestAnalyzeTokenizesComparisonOperators(t *testing.T) {
	file := source.File{Path: "example.oct", Text: "fn Main() -> Bool { return 1 == 1 != 2 < 3 <= 4 > 5 >= 6 }"}

	result, err := Analyze(file)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	assertTokenKinds(t, result.Tokens,
		KeywordFn,
		Identifier,
		LeftParen,
		RightParen,
		Arrow,
		Identifier,
		LeftBrace,
		KeywordReturn,
		IntLiteral,
		EqualEqual,
		IntLiteral,
		BangEqual,
		IntLiteral,
		LeftAngle,
		IntLiteral,
		LeftEqual,
		IntLiteral,
		RightAngle,
		IntLiteral,
		RightEqual,
		IntLiteral,
		RightBrace,
		EOF,
	)
}

func TestAnalyzeTokenizesBatchExpression(t *testing.T) {
	file := source.File{Path: "example.oct", Text: "fn Main() -> Int[] { return batch [1, 2] as x { return x + 1 } }"}

	result, err := Analyze(file)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	assertTokenKinds(t, result.Tokens,
		KeywordFn,
		Identifier,
		LeftParen,
		RightParen,
		Arrow,
		Identifier,
		LeftBracket,
		RightBracket,
		LeftBrace,
		KeywordReturn,
		KeywordBatch,
		LeftBracket,
		IntLiteral,
		Comma,
		IntLiteral,
		RightBracket,
		KeywordAs,
		Identifier,
		LeftBrace,
		KeywordReturn,
		Identifier,
		Plus,
		IntLiteral,
		RightBrace,
		RightBrace,
		EOF,
	)
}

func TestAnalyzeTokenizesFlowStateGotoSuspendWhen(t *testing.T) {
	file := source.File{Path: "example.oct", Text: "flow Patrol() -> Void { state Search { when { case true -> goto Track else -> suspend } } state Track { suspend } }"}

	result, err := Analyze(file)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	assertTokenKinds(t, result.Tokens,
		KeywordFlow,
		Identifier,
		LeftParen,
		RightParen,
		Arrow,
		Identifier,
		LeftBrace,
		KeywordState,
		Identifier,
		LeftBrace,
		KeywordWhen,
		LeftBrace,
		KeywordCase,
		KeywordTrue,
		Arrow,
		KeywordGoto,
		Identifier,
		KeywordElse,
		Arrow,
		KeywordSuspend,
		RightBrace,
		RightBrace,
		KeywordState,
		Identifier,
		LeftBrace,
		KeywordSuspend,
		RightBrace,
		RightBrace,
		EOF,
	)
}

func TestAnalyzeTokenizesFlowStateRememberResume(t *testing.T) {
	file := source.File{Path: "example.oct", Text: "flow Patrol() -> Int { state Search { remember goto Track } state Track { resume } }"}

	result, err := Analyze(file)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	assertTokenKinds(t, result.Tokens,
		KeywordFlow,
		Identifier,
		LeftParen,
		RightParen,
		Arrow,
		Identifier,
		LeftBrace,
		KeywordState,
		Identifier,
		LeftBrace,
		KeywordRemember,
		KeywordGoto,
		Identifier,
		RightBrace,
		KeywordState,
		Identifier,
		LeftBrace,
		KeywordResume,
		RightBrace,
		RightBrace,
		EOF,
	)
}

func TestAnalyzeTokenizesLogicalOperators(t *testing.T) {
	file := source.File{Path: "example.oct", Text: "fn Main() -> Bool { return not false and true or false }"}

	result, err := Analyze(file)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	assertTokenKinds(t, result.Tokens,
		KeywordFn,
		Identifier,
		LeftParen,
		RightParen,
		Arrow,
		Identifier,
		LeftBrace,
		KeywordReturn,
		KeywordNot,
		KeywordFalse,
		KeywordAnd,
		KeywordTrue,
		KeywordOr,
		KeywordFalse,
		RightBrace,
		EOF,
	)
}

func TestAnalyzeSkipsCommentsAndWhitespace(t *testing.T) {
	file := source.File{Path: "example.oct", Text: "// heading\n\nfn Main() -> Int {\n  // inside\n  return true\n}\n"}

	result, err := Analyze(file)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	assertTokenKinds(t, result.Tokens,
		KeywordFn,
		Identifier,
		LeftParen,
		RightParen,
		Arrow,
		Identifier,
		LeftBrace,
		KeywordReturn,
		KeywordTrue,
		RightBrace,
		EOF,
	)
}

func TestAnalyzeSkipsDocCommentsAsComments(t *testing.T) {
	file := source.File{Path: "example.oct", Text: "/// heading\nfn Main() -> Int { return 0 }\n"}

	result, err := Analyze(file)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	assertTokenKinds(t, result.Tokens,
		KeywordFn,
		Identifier,
		LeftParen,
		RightParen,
		Arrow,
		Identifier,
		LeftBrace,
		KeywordReturn,
		IntLiteral,
		RightBrace,
		EOF,
	)
}

func TestAnalyzeRejectsInvalidToken(t *testing.T) {
	file := source.File{Path: "example.oct", Text: "fn Main() -> Int { return $ }"}

	_, err := Analyze(file)
	if err == nil {
		t.Fatal("expected invalid token error")
	}

	if got := err.Error(); !strings.Contains(got, `invalid token`) || !strings.Contains(got, `"$"`) {
		t.Fatalf("expected deterministic invalid token error, got %q", got)
	}
}

func TestAnalyzeTokenizesScientificNotationFloats(t *testing.T) {
	file := source.File{Path: "example.oct", Text: "fn Main() -> Float { return 2e11 + 2.0e11 + 1e-6 + 1.5e-6 + 6E3 + 3.25E-2 }"}

	result, err := Analyze(file)
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}

	assertTokenKinds(t, result.Tokens,
		KeywordFn,
		Identifier,
		LeftParen,
		RightParen,
		Arrow,
		Identifier,
		LeftBrace,
		KeywordReturn,
		FloatLiteral,
		Plus,
		FloatLiteral,
		Plus,
		FloatLiteral,
		Plus,
		FloatLiteral,
		Plus,
		FloatLiteral,
		Plus,
		FloatLiteral,
		RightBrace,
		EOF,
	)
}

func TestAnalyzeRejectsMalformedScientificNotation(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "missing exponent", source: "fn Main() -> Float { return 2e }"},
		{name: "missing exponent after plus sign", source: "fn Main() -> Float { return 2e+ }"},
		{name: "missing exponent after minus sign", source: "fn Main() -> Float { return 2e- }"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Analyze(source.File{Path: "example.oct", Text: test.source})
			if err == nil {
				t.Fatal("expected invalid float literal error")
			}
			if got := err.Error(); !strings.Contains(got, "invalid float literal") {
				t.Fatalf("expected invalid float literal error, got %q", got)
			}
		})
	}
}

func assertTokenKinds(t *testing.T, tokens []Token, expected ...TokenKind) {
	t.Helper()
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %#v", len(expected), len(tokens), tokens)
	}

	for i, kind := range expected {
		if tokens[i].Kind != kind {
			t.Fatalf("token %d: expected %s, got %s (%q)", i, kind, tokens[i].Kind, tokens[i].Lexeme)
		}
	}
}
