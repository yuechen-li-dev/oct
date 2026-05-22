package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"oct/internal/build"
	"oct/internal/exprun"
	"oct/internal/ocfmt"
	"oct/internal/pkgmgr"
	"oct/internal/prometheus"
	"oct/internal/run"
	"oct/internal/tester"
)

func Execute(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) < 1 {
		return writeTopLevelHelp(stdout)
	}

	command := args[0]
	if command == "--help" || command == "-h" || command == "help" {
		return writeTopLevelHelp(stdout)
	}

	switch command {
	case "pkg":
		return executePkg(args[1:], stdout, stderr)
	case "exp":
		return executeExp(args[1:], stdout, stderr)
	case "run":
		if isHelpArg(args[1:]) {
			return writeRunHelp(stdout)
		}
		if len(args) != 2 {
			return reportCommandError(stderr, command, fmt.Errorf("missing path; run oct run --help for usage"))
		}
		path := args[1]
		if err := run.Execute(path, stdout); err != nil {
			return reportCommandError(stderr, command, err)
		}
		return nil
	case "build":
		if isHelpArg(args[1:]) {
			return writeBuildHelp(stdout)
		}
		if len(args) != 2 {
			return reportCommandError(stderr, command, fmt.Errorf("missing path; run oct build --help for usage"))
		}
		path := args[1]
		result, err := build.Compile(path)
		if err != nil {
			return reportCommandError(stderr, command, err)
		}
		_, err = fmt.Fprintf(stdout, "build succeeded: %s\n", result.ArtifactPath)
		return err
	case "fmt":
		if isHelpArg(args[1:]) {
			return writeFmtHelp(stdout)
		}
		fmtOptions, err := parseFmtOptions(args[1:])
		if err != nil {
			return reportCommandError(stderr, command, err)
		}
		if err := ocfmt.FormatPathWithOptions(fmtOptions.path, fmtOptions.options); err != nil {
			return reportCommandError(stderr, command, err)
		}
		return nil
	case "test":
		if isHelpArg(args[1:]) {
			return writeTestHelp(stdout)
		}
		if len(args) < 2 {
			return reportCommandError(stderr, command, fmt.Errorf("missing path; run oct test --help for usage"))
		}
		options, paths, err := parseTestOptions(args[1:])
		if err != nil {
			return reportCommandError(stderr, command, err)
		}
		for _, path := range paths {
			if err := tester.ExecuteWithOptions(path, stdout, options); err != nil {
				return reportCommandError(stderr, command, err)
			}
		}
		return nil
	case "artifact":
		if isHelpArg(args[1:]) {
			return writeArtifactHelp(stdout)
		}
		if len(args) != 2 {
			return reportCommandError(stderr, command, fmt.Errorf("missing path; run oct artifact --help for usage"))
		}
		path := args[1]
		if err := tester.ExecuteArtifacts(path, stdout); err != nil {
			return reportCommandError(stderr, command, err)
		}
		return nil
	case "bench":
		if isHelpArg(args[1:]) {
			return writeBenchHelp(stdout)
		}
		if len(args) < 2 {
			return reportCommandError(stderr, command, fmt.Errorf("missing path; run oct bench --help for usage"))
		}
		path := args[1]
		options, err := parseBenchOptions(path, args[2:])
		if err != nil {
			return reportCommandError(stderr, command, err)
		}
		if err := tester.ExecuteBenchmarks(path, stdout, options); err != nil {
			return reportCommandError(stderr, command, err)
		}
		return nil
	case "prometheus-sgemm":
		if len(args) < 2 {
			return reportCommandError(stderr, command, fmt.Errorf("missing backend path; run oct %s --help for usage", command))
		}
		path := args[1]
		octagonOut, err := parseBenchOctagonOut(args[2:])
		if err != nil {
			return reportCommandError(stderr, command, err)
		}
		backend := prometheus.Backend(path)
		report, runErr := prometheus.RunStarterCorpus(backend)
		for _, item := range report.Runs {
			_, _ = fmt.Fprintf(stdout, "SGEMM M=%d N=%d K=%d backend_requested=%s backend_used=%s status=%s correctness=%t cpu=%dns vulkan=%dns vulkan_env=%s wall=%dns\n",
				item.Shape.M, item.Shape.N, item.Shape.K, item.RequestedBackend, item.UsedBackend, item.Status.String(), item.Correctness.Pass, item.CPUTimeNs, item.VulkanTimeNs, item.VulkanEnv, item.WallTimeNs)
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
	case "prometheus-m1-async":
		octagonOut, err := parseBenchOctagonOut(args[1:])
		if err != nil {
			return reportCommandError(stderr, command, err)
		}
		result, runErr := prometheus.ValidateAsyncSGEMMOnHardware(prometheus.Shape{M: 64, N: 64, K: 32})
		_, _ = fmt.Fprintf(stdout,
			"backend_requested=%s backend_used=%s outcome=%s correctness=%t submit_stage=%s submit_detail_code=%d submit_detail_name=%s query_lifecycle=%s query_ready=%t query_failed=%t query_consumed=%t query_outstanding=%d query_attempts=%d consume_stage=%s consume_detail_code=%d consume_detail_name=%s vulkan_env=%s wall=%dns\n",
			result.RequestedBackend, result.UsedBackend, result.Outcome, result.Correctness.Pass,
			result.SubmitStage, result.SubmitDetailCode, result.SubmitDetailName,
			result.QueryLifecycle, result.QueryReady, result.QueryFailed, result.QueryConsumed, result.QueryOutstanding, result.QueryAttempts,
			result.ConsumeStage, result.ConsumeDetailCode, result.ConsumeDetailName,
			result.Environment, result.WallTimeNs)
		if octagonOut != "" {
			if err := prometheus.WriteAsyncValidationOctagon(octagonOut, result); err != nil {
				return reportCommandError(stderr, command, err)
			}
		}
		if runErr != nil {
			return reportCommandError(stderr, command, runErr)
		}
		return nil
	default:
		return reportCommandError(stderr, command, fmt.Errorf("unknown command %q; run oct --help for available commands", command))
	}
}
func isHelpArg(args []string) bool { return len(args) == 1 && (args[0] == "--help" || args[0] == "-h") }

func executePkg(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) < 1 {
		return reportCommandError(stderr, "pkg", fmt.Errorf("usage: oct pkg <get|list|sync>"))
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
	case "sync":
		if len(args) != 1 {
			return reportCommandError(stderr, "pkg sync", fmt.Errorf("usage: oct pkg sync"))
		}
		result, err := manager.Sync(".")
		if err != nil {
			return reportCommandError(stderr, "pkg sync", err)
		}
		_, err = fmt.Fprintf(stdout, "sync project: %s\nmanifest: %s\ndependencies: %d\n", result.ProjectPath, result.ManifestPath, len(result.Dependencies))
		if err != nil {
			return err
		}
		for _, dep := range result.Dependencies {
			status := "fetched"
			if dep.GetResult.Hit {
				status = "cache hit"
			}
			_, err = fmt.Fprintf(stdout, "- %s (%s) [%s]\n  source: %s\n  cache: %s\n", dep.Name, dep.VersionRequirement, status, dep.Source, dep.GetResult.Path)
			if err != nil {
				return err
			}
		}
		_, err = fmt.Fprintln(stdout, "sync complete")
		return err
	default:
		return reportCommandError(stderr, "pkg", fmt.Errorf("usage: oct pkg <get|list|sync>"))
	}
}

func executeExp(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) < 1 {
		return reportCommandError(stderr, "exp", fmt.Errorf("usage: oct exp run <git-url>"))
	}
	switch args[0] {
	case "run":
		if len(args) != 2 {
			return reportCommandError(stderr, "exp run", fmt.Errorf("usage: oct exp run <git-url>"))
		}
		_, err := exprun.RunFromGit(args[1], stdout)
		if err != nil {
			return reportCommandError(stderr, "exp run", err)
		}
		return nil
	default:
		return reportCommandError(stderr, "exp", fmt.Errorf("usage: oct exp run <git-url>"))
	}
}

func reportCommandError(stderr io.Writer, command string, err error) error {
	_, writeErr := fmt.Fprintf(stderr, "%s failed: %v\n", command, err)
	if writeErr != nil {
		return errors.Join(err, writeErr)
	}
	return err
}

type fmtOptions struct {
	path    string
	options ocfmt.Options
}

func parseFmtOptions(args []string) (fmtOptions, error) {
	if len(args) < 1 {
		return fmtOptions{}, fmt.Errorf("usage: oct fmt <file-or-root> [--mode readable|compact|en-llm] [--check]")
	}
	result := fmtOptions{path: args[0]}
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--check":
			result.options.Check = true
		case "--mode":
			if i+1 >= len(args) {
				return fmtOptions{}, fmt.Errorf("fmt --mode requires a value")
			}
			i++
			result.options.Mode = ocfmt.Mode(args[i])
		default:
			return fmtOptions{}, fmt.Errorf("usage: oct fmt <file-or-root> [--mode readable|compact|en-llm] [--check]")
		}
	}
	return result, nil
}
func writeTopLevelHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct <command> [options]\n\ncommands:\n  run        Run a program\n  build      Compile a program\n  test       Run octest suites\n  artifact   Run artifact generators\n  bench      Run benchmark suites\n  fmt        Format Oct source files\n  pkg        Package manager commands\n  exp        Run experiment repos\n\nrun 'oct <command> --help' for command details.")
	return err
}
func writeRunHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct run <file-or-root>\nRun a program entrypoint.")
	return err
}
func writeBuildHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct build <file-or-root>\nCompile a program and emit an artifact.")
	return err
}
func writeFmtHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct fmt <file-or-root> [--mode readable|compact|en-llm] [--check]\nFormat Oct source files.")
	return err
}
func writeTestHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct test <file-or-root> [--suite <name>] [--execution <auto|compiled|interpreted>]\nRun octest files.\nOptions: --suite, --execution\nExample: oct test Language/Testing --execution compiled")
	return err
}
func writeArtifactHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct artifact <file-or-root>\nRun artifact generation for discovered artifact blocks.")
	return err
}
func writeBenchHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct bench <file-or-root> [--octagon-out <file.octagon>] [--profile] [--profile-format <octagon|pprof|both>] [--filter <pattern>]\nRun benchmark suites with optional profiling.")
	return err
}

func parseBenchOptions(path string, args []string) (tester.BenchmarkOptions, error) {
	options := tester.BenchmarkOptions{
		ProfileMode: "cpu",
	}
	for i := 0; i < len(args); i++ {
		flag := args[i]
		switch flag {
		case "--octagon-out":
			if i+1 >= len(args) {
				return tester.BenchmarkOptions{}, fmt.Errorf("usage: oct bench <file-or-root> [--octagon-out <file.octagon>] [--profile] [--profile-format <octagon|pprof|both>] [--filter <pattern>]")
			}
			value := args[i+1]
			i++
			if !strings.HasSuffix(value, ".octagon") {
				return tester.BenchmarkOptions{}, fmt.Errorf("bench --octagon-out path must end with .octagon")
			}
			options.OctagonOutPath = value
		case "--profile":
			options.ProfileEnabled = true
		case "--filter":
			if i+1 >= len(args) {
				return tester.BenchmarkOptions{}, fmt.Errorf("usage: oct bench <file-or-root> [--octagon-out <file.octagon>] [--profile] [--profile-format <octagon|pprof|both>] [--filter <pattern>]")
			}
			i++
			value := args[i]
			options.Filter = value
		case "--profile-format":
			if i+1 >= len(args) {
				return tester.BenchmarkOptions{}, fmt.Errorf("usage: oct bench <file-or-root> [--octagon-out <file.octagon>] [--profile] [--profile-format <octagon|pprof|both>] [--filter <pattern>]")
			}
			i++
			value := args[i]
			if value != "octagon" && value != "pprof" && value != "both" {
				return tester.BenchmarkOptions{}, fmt.Errorf("bench --profile-format must be one of 'octagon', 'pprof', or 'both'")
			}
			options.ProfileFormat = value
		default:
			return tester.BenchmarkOptions{}, fmt.Errorf("usage: oct bench <file-or-root> [--octagon-out <file.octagon>] [--profile] [--profile-format <octagon|pprof|both>] [--filter <pattern>]")
		}
	}
	if options.ProfileFormat == "" {
		options.ProfileFormat = "octagon"
	}
	if options.ProfileFormat != "octagon" && !options.ProfileEnabled {
		return tester.BenchmarkOptions{}, fmt.Errorf("bench --profile-format requires --profile")
	}
	if options.ProfileEnabled {
		options.Invocation = "oct bench " + path
		if options.Filter != "" {
			options.Invocation += " --filter " + options.Filter
		}
		options.Invocation += " --profile"
		if options.ProfileFormat != "octagon" {
			options.Invocation += " --profile-format " + options.ProfileFormat
		}
		switch options.ProfileFormat {
		case "octagon":
			options.ProfileOctagonOutPath = tester.DefaultCPUProfileOctagonPath(path)
		case "pprof":
			options.ProfileOutPath = tester.DefaultCPUProfilePath(path)
			options.ProfileRawOut = options.ProfileOutPath
		case "both":
			options.ProfileOutPath = tester.DefaultCPUProfilePath(path)
			options.ProfileRawOut = options.ProfileOutPath
			options.ProfileOctagonOutPath = tester.DefaultCPUProfileOctagonPath(path)
		}
	}
	return options, nil
}

func parseTestOptions(args []string) (tester.TestOptions, []string, error) {
	options := tester.TestOptions{}
	paths := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--suite" {
			if i+1 >= len(args) {
				return tester.TestOptions{}, nil, fmt.Errorf("usage: oct test <file-or-root> [--suite <name>]")
			}
			i++
			options.Suite = strings.TrimSpace(args[i])
			if options.Suite == "" {
				return tester.TestOptions{}, nil, fmt.Errorf("--suite requires a non-empty value")
			}
			continue
		}
		if arg == "--execution" {
			if i+1 >= len(args) {
				return tester.TestOptions{}, nil, fmt.Errorf("usage: oct test <file-or-root> [--suite <name>] [--execution <auto|compiled|interpreted>]")
			}
			i++
			options.Execution = strings.TrimSpace(args[i])
			if options.Execution == "" {
				return tester.TestOptions{}, nil, fmt.Errorf("--execution requires a non-empty value")
			}
			continue
		}
		paths = append(paths, arg)
	}
	if len(paths) == 0 {
		return tester.TestOptions{}, nil, fmt.Errorf("usage: oct test <file-or-root> [--suite <name>] [--execution <auto|compiled|interpreted>]")
	}
	return options, paths, nil
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
