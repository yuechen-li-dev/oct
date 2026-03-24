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

func TestAnalyzeTokenizesIfElseAndSwitch(t *testing.T) {
	file := source.File{Path: "example.oct", Text: "fn Main() -> Int { if true { return switch 1 { case 1 => 2 else => 3 } } else { return 0 } }"}

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

func TestAnalyzeRejectsInvalidToken(t *testing.T) {
	file := source.File{Path: "example.oct", Text: "fn Main() -> Int { return @ }"}

	_, err := Analyze(file)
	if err == nil {
		t.Fatal("expected invalid token error")
	}

	if got := err.Error(); !strings.Contains(got, `invalid token`) || !strings.Contains(got, `"@"`) {
		t.Fatalf("expected deterministic invalid token error, got %q", got)
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
