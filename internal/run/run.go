package run

import (
	"fmt"
	"io"

	"oct/internal/interpret"
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

	value, err := interpret.ExecuteMain(program)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(output, value.String())
	return err
}
