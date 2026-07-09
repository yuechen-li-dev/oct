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
