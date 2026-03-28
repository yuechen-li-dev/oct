package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"oct/internal/build"
	"oct/internal/prometheus"
	"oct/internal/run"
	"oct/internal/tester"
)

func Execute(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) < 2 {
		return writeUsage(stderr)
	}

	command := args[0]
	path := args[1]

	switch command {
	case "run":
		if len(args) != 2 {
			return writeUsage(stderr)
		}
		if err := run.Execute(path, stdout); err != nil {
			return reportCommandError(stderr, command, err)
		}
		return nil
	case "build":
		if len(args) != 2 {
			return writeUsage(stderr)
		}
		result, err := build.Compile(path)
		if err != nil {
			return reportCommandError(stderr, command, err)
		}
		_, err = fmt.Fprintf(stdout, "build succeeded: %s\n", result.ArtifactPath)
		return err
	case "test":
		if len(args) != 2 {
			return writeUsage(stderr)
		}
		if err := tester.Execute(path, stdout); err != nil {
			return reportCommandError(stderr, command, err)
		}
		return nil
	case "artifact":
		if len(args) != 2 {
			return writeUsage(stderr)
		}
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
	case "prometheus-sgemm":
		octagonOut, err := parseBenchOctagonOut(args[2:])
		if err != nil {
			return reportCommandError(stderr, command, err)
		}
		backend := prometheus.Backend(path)
		report, runErr := prometheus.RunStarterCorpus(backend)
		for _, item := range report.Runs {
			_, _ = fmt.Fprintf(stdout, "SGEMM M=%d N=%d K=%d backend_requested=%s backend_used=%s status=%s correctness=%t wall=%dns\n",
				item.Shape.M, item.Shape.N, item.Shape.K, item.RequestedBackend, item.UsedBackend, item.Status.String(), item.Correctness.Pass, item.WallTimeNs)
		}
		if octagonOut != "" {
			if err := prometheus.WriteOctagonReport(octagonOut, report); err != nil {
				return reportCommandError(stderr, command, err)
			}
		}
		if runErr != nil {
			return reportCommandError(stderr, command, runErr)
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
	_, err := fmt.Fprintln(stderr, "usage: oct <run|build|test|artifact|bench|prometheus-sgemm> <file-or-root|cpu|prometheus>")
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
