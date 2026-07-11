package lex

import (
	"testing"

	"github.com/yuechen-li-dev/oct/internal/sdslv/token"
	"github.com/yuechen-li-dev/oct/internal/source"
)

func TestAnalyzeComputeShaderTokens(t *testing.T) {
	result, err := Analyze(source.File{Path: "test.sdslv", Text: "shader S { stage compute [numthreads(16, 16, 1)] fn CS() -> void { return; } }"})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	kinds := map[token.Kind]bool{}
	for _, tok := range result.Tokens {
		kinds[tok.Kind] = true
	}
	for _, kind := range []token.Kind{token.KeywordShader, token.KeywordStage, token.KeywordCompute, token.LeftBracket, token.Semicolon} {
		if !kinds[kind] {
			t.Fatalf("missing token kind %s in %#v", kind, result.Tokens)
		}
	}
}

func TestAnalyzeReductionKeywords(t *testing.T) {
	result, err := Analyze(source.File{Path: "test.sdslv", Text: "fn F() -> f32 { return sum i in 0u..4u { max(1.0, product); }; }"})
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	kinds := map[token.Kind]bool{}
	for _, tok := range result.Tokens {
		kinds[tok.Kind] = true
	}
	for _, kind := range []token.Kind{token.KeywordSum, token.KeywordMax, token.KeywordProduct, token.KeywordIn} {
		if !kinds[kind] {
			t.Fatalf("missing token kind %s in %#v", kind, result.Tokens)
		}
	}
}

func TestSdslvTokenSpanCoversIdentifier(t *testing.T) {
	const text = "\u03b1bc"
	result, err := Analyze(source.File{Path: "test.sdslv", Text: text})
	if err != nil {
		t.Fatal(err)
	}
	tok := result.Tokens[0]
	if got := text[tok.Span.Start.Offset:tok.Span.End.Offset]; got != text {
		t.Fatalf("token slice = %q, want %q", got, text)
	}
	if tok.Span.Start.Line != 1 || tok.Span.Start.Column != 1 || tok.Span.End.Column != 4 {
		t.Fatalf("span = %#v, want rune columns 1..4", tok.Span)
	}
}

func TestSdslvTokenSpanTracksEndOfFile(t *testing.T) {
	result, err := Analyze(source.File{Path: "test.sdslv", Text: "x\n"})
	if err != nil {
		t.Fatal(err)
	}
	eof := result.Tokens[len(result.Tokens)-1]
	if eof.Kind != token.EOF || eof.Span.Start != eof.Span.End || eof.Span.Start.Offset != 2 || eof.Span.Start.Line != 2 || eof.Span.Start.Column != 1 {
		t.Fatalf("EOF = %#v", eof)
	}
}
