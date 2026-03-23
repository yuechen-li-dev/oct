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

	lexed, err := lex.Analyze(file)
	if err != nil {
		return err
	}
	if _, err := parse.BuildFile(lexed); err != nil {
		return err
	}

	_, err = fmt.Fprintln(output, "hello from oct")
	return err
}
