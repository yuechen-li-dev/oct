package run

import (
	"fmt"
	"io"

	"oct/internal/lex"
	"oct/internal/parse"
	"oct/internal/source"
	"oct/internal/typecheck"
)

func Execute(path string, output io.Writer) error {
	file, err := source.Load(path)
	if err != nil {
		return err
	}

	lexed, err := lex.Analyze(file)
	if err != nil {
		return err
	}
	program, err := parse.BuildFile(lexed)
	if err != nil {
		return err
	}
	if err := typecheck.Check(program); err != nil {
		return err
	}

	_, err = fmt.Fprintln(output, "hello from oct")
	return err
}
