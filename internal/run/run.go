package run

import (
	"fmt"
	"io"

	"oct/internal/lex"
	"oct/internal/parse"
	"oct/internal/source"
)

func Execute(path string, output io.Writer) error {
	file, err := source.Load(path)
	if err != nil {
		return err
	}

	tokens := lex.Analyze(file)
	_ = parse.BuildProgram(tokens)

	_, err = fmt.Fprintln(output, "hello from oct")
	return err
}
