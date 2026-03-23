package parse

import (
	"oct/internal/ast"
	"oct/internal/lex"
)

func BuildProgram(result lex.Result) ast.Program {
	return ast.Program{
		SourcePath: result.Source.Path,
		SourceText: result.Source.Text,
		Tokens:     append([]string(nil), result.Tokens...),
	}
}
