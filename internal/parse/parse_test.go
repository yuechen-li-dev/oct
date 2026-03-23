package parse

import (
	"testing"

	"oct/internal/lex"
	"oct/internal/source"
)

func TestBuildProgramCreatesASTProgram(t *testing.T) {
	result := lex.Result{
		Source: source.File{Path: "example.oct", Text: "ignored"},
		Tokens: []string{"M0StubToken"},
	}

	program := BuildProgram(result)

	if program.SourcePath != "example.oct" {
		t.Fatalf("expected source path %q, got %q", "example.oct", program.SourcePath)
	}

	if program.SourceText != "ignored" {
		t.Fatalf("expected source text %q, got %q", "ignored", program.SourceText)
	}

	if len(program.Tokens) != 1 || program.Tokens[0] != "M0StubToken" {
		t.Fatalf("expected tokens %v, got %v", result.Tokens, program.Tokens)
	}
}
