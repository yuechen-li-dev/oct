package lex

import (
	"testing"

	"oct/internal/source"
)

func TestAnalyzeReturnsStubToken(t *testing.T) {
	file := source.File{Path: "example.oct", Text: "ignored"}

	result := Analyze(file)

	if result.Source != file {
		t.Fatalf("expected source %+v, got %+v", file, result.Source)
	}

	expected := []string{"M0StubToken"}
	if len(result.Tokens) != len(expected) || result.Tokens[0] != expected[0] {
		t.Fatalf("expected tokens %v, got %v", expected, result.Tokens)
	}
}
