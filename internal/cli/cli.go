package cli

import (
	"errors"
	"fmt"
	"io"

	"oct/internal/build"
	"oct/internal/run"
	"oct/internal/tester"
)

func Execute(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) != 2 {
		return writeUsage(stderr)
	}

	command := args[0]
	path := args[1]

	switch command {
	case "run":
		if err := run.Execute(path, stdout); err != nil {
			return reportCommandError(stderr, command, err)
		}
		return nil
	case "build":
		result, err := build.Compile(path)
		if err != nil {
			return reportCommandError(stderr, command, err)
		}
		_, err = fmt.Fprintf(stdout, "build succeeded: %s\n", result.ArtifactPath)
		return err
	case "test":
		if err := tester.Execute(path, stdout); err != nil {
			return reportCommandError(stderr, command, err)
		}
		return nil
	case "artifact":
		if err := tester.ExecuteArtifacts(path, stdout); err != nil {
			return reportCommandError(stderr, command, err)
		}
		return nil
	case "bench":
		if err := tester.ExecuteBenchmarks(path, stdout); err != nil {
			return reportCommandError(stderr, command, err)
		}
		return nil
	default:
		return writeUsage(stderr)
	}
}

func reportCommandError(stderr io.Writer, command string, err error) error {
	_, writeErr := fmt.Fprintf(stderr, "%s failed: %v\n", command, err)
	if writeErr != nil {
		return errors.Join(err, writeErr)
	}
	return err
}

func writeUsage(stderr io.Writer) error {
	_, err := fmt.Fprintln(stderr, "usage: oct <run|build|test|artifact|bench> <file-or-root>")
	return err
}
