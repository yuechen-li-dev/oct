package run

import (
	"fmt"
	"io"

	"oct/internal/interpret"
	"oct/internal/project"
	"oct/internal/typecheck"
)

func Execute(path string, output io.Writer) error {
	program, err := project.Load(path)
	if err != nil {
		return err
	}
	if err := typecheck.CheckProgram(program); err != nil {
		return err
	}

	value, err := interpret.ExecuteMain(program, output)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(output, value.String())
	return err
}
