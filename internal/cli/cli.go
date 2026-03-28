package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"oct/internal/build"
	"oct/internal/run"
	"oct/internal/tester"
)

func Execute(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) < 2 {
		return writeUsage(stderr)
	}

	command := args[0]
	path := args[1]

	if command != "bench" && len(args) != 2 {
		return writeUsage(stderr)
	}

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
		octagonOut, err := parseBenchOctagonOut(args[2:])
		if err != nil {
			return reportCommandError(stderr, command, err)
		}
		if err := tester.ExecuteBenchmarks(path, stdout, tester.BenchmarkOptions{OctagonOutPath: octagonOut}); err != nil {
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

func parseBenchOctagonOut(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	if len(args) != 2 {
		return "", fmt.Errorf("usage: oct bench <file-or-root> [--octagon-out <file.octagon>]")
	}
	if args[0] != "--octagon-out" {
		return "", fmt.Errorf("usage: oct bench <file-or-root> [--octagon-out <file.octagon>]")
	}
	if !strings.HasSuffix(args[1], ".octagon") {
		return "", fmt.Errorf("bench --octagon-out path must end with .octagon")
	}
	return args[1], nil
}
