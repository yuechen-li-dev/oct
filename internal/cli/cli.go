package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"oct/internal/build"
	"oct/internal/pkgmgr"
	"oct/internal/prometheus"
	"oct/internal/run"
	"oct/internal/tester"
)

func Execute(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) < 1 {
		return writeUsage(stderr)
	}

	command := args[0]

	switch command {
	case "pkg":
		return executePkg(args[1:], stdout, stderr)
	case "run":
		if len(args) != 2 {
			return writeUsage(stderr)
		}
		path := args[1]
		if err := run.Execute(path, stdout); err != nil {
			return reportCommandError(stderr, command, err)
		}
		return nil
	case "build":
		if len(args) != 2 {
			return writeUsage(stderr)
		}
		path := args[1]
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
		path := args[1]
		if err := tester.Execute(path, stdout); err != nil {
			return reportCommandError(stderr, command, err)
		}
		return nil
	case "artifact":
		if len(args) != 2 {
			return writeUsage(stderr)
		}
		path := args[1]
		if err := tester.ExecuteArtifacts(path, stdout); err != nil {
			return reportCommandError(stderr, command, err)
		}
		return nil
	case "bench":
		if len(args) < 2 {
			return writeUsage(stderr)
		}
		path := args[1]
		octagonOut, err := parseBenchOctagonOut(args[2:])
		if err != nil {
			return reportCommandError(stderr, command, err)
		}
		if err := tester.ExecuteBenchmarks(path, stdout, tester.BenchmarkOptions{OctagonOutPath: octagonOut}); err != nil {
			return reportCommandError(stderr, command, err)
		}
		return nil
	case "prometheus-sgemm":
		if len(args) < 2 {
			return writeUsage(stderr)
		}
		path := args[1]
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

func executePkg(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) < 1 {
		return writeUsage(stderr)
	}
	manager, err := pkgmgr.NewManager()
	if err != nil {
		return reportCommandError(stderr, "pkg", err)
	}
	switch args[0] {
	case "get":
		if len(args) != 2 {
			return reportCommandError(stderr, "pkg get", fmt.Errorf("usage: oct pkg get <git-url>"))
		}
		result, err := manager.Get(args[1])
		if err != nil {
			return reportCommandError(stderr, "pkg get", err)
		}
		status := "fetched"
		if result.Hit {
			status = "cache hit"
		}
		_, err = fmt.Fprintf(stdout, "%s\nsource: %s\ncache: %s\nkey: %s\n", status, result.Source, result.Path, result.CacheKey)
		if err != nil {
			return err
		}
		if result.Name != "" || result.Version != "" {
			_, err = fmt.Fprintf(stdout, "package: %s@%s\n", result.Name, result.Version)
			if err != nil {
				return err
			}
		}
		_, err = fmt.Fprintf(stdout, "dependencies: %d\n", len(result.Manifest.Dependencies))
		if err != nil {
			return err
		}
		if result.Head != "" {
			_, err = fmt.Fprintf(stdout, "head: %s\n", result.Head)
			if err != nil {
				return err
			}
		}
		return nil
	case "list":
		if len(args) != 1 {
			return reportCommandError(stderr, "pkg list", fmt.Errorf("usage: oct pkg list"))
		}
		entries, err := manager.List()
		if err != nil {
			return reportCommandError(stderr, "pkg list", err)
		}
		if len(entries) == 0 {
			_, err = fmt.Fprintf(stdout, "no cached packages (cache: %s)\n", manager.CacheDir())
			return err
		}
		_, err = fmt.Fprintf(stdout, "cached packages (%d) [cache: %s]\n", len(entries), manager.CacheDir())
		if err != nil {
			return err
		}
		for _, entry := range entries {
			identity := entry.Name
			if identity == "" {
				identity = "(unknown)"
			}
			if entry.Version != "" {
				identity += "@" + entry.Version
			}
			head := entry.Head
			if len(head) > 12 {
				head = head[:12]
			}
			_, err = fmt.Fprintf(stdout, "- %s\n  source: %s\n  key: %s\n  head: %s\n  deps: %d\n  path: %s\n", identity, entry.Source, entry.CacheKey, head, len(entry.Dependencies), entry.Path)
			if err != nil {
				return err
			}
		}
		return nil
	default:
		return reportCommandError(stderr, "pkg", fmt.Errorf("usage: oct pkg <get|list>"))
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
	_, err := fmt.Fprintln(stderr, "usage: oct <run|build|test|artifact|bench|prometheus-sgemm> <file-or-root|cpu|prometheus>\n       oct pkg <get|list> [args]")
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
