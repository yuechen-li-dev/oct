package lex

import "oct/internal/source"

type Result struct {
	Source source.File
	Tokens []string
}

func Analyze(file source.File) Result {
	return Result{
		Source: file,
		Tokens: []string{"M0StubToken"},
	}
}
