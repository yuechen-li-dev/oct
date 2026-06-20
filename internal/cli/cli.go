package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/build"
	"github.com/yuechen-li-dev/oct/internal/exprun"
	"github.com/yuechen-li-dev/oct/internal/makecmd"
	"github.com/yuechen-li-dev/oct/internal/newpkg"
	"github.com/yuechen-li-dev/oct/internal/ocfmt"
	"github.com/yuechen-li-dev/oct/internal/pkgmgr"
	"github.com/yuechen-li-dev/oct/internal/prometheus"
	"github.com/yuechen-li-dev/oct/internal/run"
	"github.com/yuechen-li-dev/oct/internal/tester"
)

var version = "dev"

func Execute(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) < 1 {
		return writeTopLevelHelp(stdout)
	}

	command := args[0]
	if command == "--help" || command == "-h" || command == "help" {
		return writeTopLevelHelp(stdout)
	}
	if command == "--version" || command == "version" {
		return writeVersion(stdout)
	}

	switch command {
	case "new":
		return executeNew(args[1:], stdout, stderr)
	case "init":
		return executeInit(args[1:], stdout, stderr)
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
	case "make":
		if isHelpArg(args[1:]) {
			return writeMakeHelp(stdout)
		}
		options, err := parseMakeOptions(args[1:])
		if err != nil {
			return reportCommandError(stderr, command, err)
		}
		if err := makecmd.Execute(options, stdout, stderr); err != nil {
			return reportCommandError(stderr, command, err)
		}
		return nil
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
		options, paths, err := parseArtifactOptions(args[1:])
		if err != nil {
			return reportCommandError(stderr, command, err)
		}
		for _, path := range paths {
			if err := tester.ExecuteArtifactsWithOptions(path, stdout, options); err != nil {
				return reportCommandError(stderr, command, err)
			}
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

func executeNew(args []string, stdout io.Writer, stderr io.Writer) error {
	if isHelpArg(args) {
		return writeNewHelp(stdout)
	}
	if len(args) != 2 && len(args) != 3 {
		return reportCommandError(stderr, "new", fmt.Errorf("usage: oct new <experiment|library|wrapper-library> <Name> [path]"))
	}
	kind := newpkg.Kind(args[0])
	switch kind {
	case newpkg.KindExperiment, newpkg.KindLibrary, newpkg.KindWrapperLibrary:
		// recognized below
	default:
		return reportCommandError(stderr, "new", fmt.Errorf("usage: oct new <experiment|library|wrapper-library> <Name> [path]"))
	}
	name := args[1]
	target := defaultNewTarget(kind, name, args[2:])
	if err := newpkg.Write(newpkg.Options{Kind: kind, Name: name, Dir: target}); err != nil {
		return reportCommandError(stderr, "new", err)
	}
	_, err := fmt.Fprintf(stdout, "Created %s package %s at %s\n", kind, name, target)
	return err
}

func defaultNewTarget(kind newpkg.Kind, name string, explicit []string) string {
	if len(explicit) > 0 {
		return explicit[0]
	}
	switch kind {
	case newpkg.KindExperiment:
		if isDirectory("Experiments") {
			return filepath.Join("Experiments", name)
		}
	case newpkg.KindLibrary, newpkg.KindWrapperLibrary:
		if isDirectory("Libraries") {
			return filepath.Join("Libraries", name)
		}
	}
	return name
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func executeInit(args []string, stdout io.Writer, stderr io.Writer) error {
	if isHelpArg(args) {
		return writeInitHelp(stdout)
	}
	if len(args) == 2 && isHelpArg(args[1:]) {
		kind := newpkg.Kind(args[0])
		switch kind {
		case newpkg.KindExperiment, newpkg.KindLibrary, newpkg.KindWrapperLibrary:
			return writeInitKindHelp(stdout, kind)
		default:
			return reportCommandError(stderr, "init", fmt.Errorf("usage: oct init <experiment|library|wrapper-library>"))
		}
	}
	if len(args) != 1 {
		return reportCommandError(stderr, "init", fmt.Errorf("usage: oct init <experiment|library|wrapper-library>"))
	}
	kind := newpkg.Kind(args[0])
	switch kind {
	case newpkg.KindExperiment, newpkg.KindLibrary, newpkg.KindWrapperLibrary:
		// recognized below
	default:
		return reportCommandError(stderr, "init", fmt.Errorf("usage: oct init <experiment|library|wrapper-library>"))
	}
	cwd, err := os.Getwd()
	if err != nil {
		return reportCommandError(stderr, "init", fmt.Errorf("read current directory: %w", err))
	}
	name := filepath.Base(cwd)
	if err := newpkg.InitWrite(newpkg.Options{Kind: kind, Name: name, Dir: "."}); err != nil {
		if strings.Contains(err.Error(), "invalid package name") {
			err = fmt.Errorf("%w; oct init derives the package name from the current directory basename %q; rename the directory or wait for future explicit-name support", err, name)
		}
		return reportCommandError(stderr, "init", err)
	}
	_, err = fmt.Fprintf(stdout, "Initialized %s package %s in %s\n", kind, name, cwd)
	return err
}

func executePkg(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) < 1 {
		return reportCommandError(stderr, "pkg", fmt.Errorf("usage: oct pkg <get|list|sync|lock|registry|add|wrappers|build-wrappers>"))
	}
	if isHelpArg(args) {
		return writePkgHelp(stdout)
	}
	manager, err := pkgmgr.NewManager()
	if err != nil {
		return reportCommandError(stderr, "pkg", err)
	}
	switch args[0] {
	case "registry":
		return executePkgRegistry(args[1:], stdout, stderr)
	case "add":
		if isHelpArg(args[1:]) {
			return writePkgAddHelp(stdout)
		}
		name, registryName, err := parsePkgAddArgs(args[1:])
		if err != nil {
			return reportCommandError(stderr, "pkg add", err)
		}
		result, err := pkgmgr.AddDependency(".", name, registryName)
		if err != nil {
			return reportCommandError(stderr, "pkg add", err)
		}
		_, err = fmt.Fprintf(stdout, "Added dependency %s %s from registry %s\nRun oct pkg sync to fetch package sources.\n", result.Name, result.Version, result.Registry)
		return err
	case "get":
		if isHelpArg(args[1:]) {
			return writePkgGetHelp(stdout)
		}
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
		if isHelpArg(args[1:]) {
			return writePkgListHelp(stdout)
		}
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
	case "lock":
		if isHelpArg(args[1:]) {
			return writePkgLockHelp(stdout)
		}
		if len(args) != 1 {
			return reportCommandError(stderr, "pkg lock", fmt.Errorf("usage: oct pkg lock"))
		}
		result, err := manager.Lock(".")
		if err != nil {
			return reportCommandError(stderr, "pkg lock", err)
		}
		_, err = fmt.Fprintf(stdout, "Resolved package graph: %d packages\n", len(result.Lock.Packages))
		if err != nil {
			return err
		}
		for _, warning := range result.LocalWarnings {
			if _, err := fmt.Fprintln(stdout, warning); err != nil {
				return err
			}
		}
		_, err = fmt.Fprintln(stdout, "Wrote lock.octagon")
		return err
	case "sync":
		if isHelpArg(args[1:]) {
			return writePkgSyncHelp(stdout)
		}
		locked := false
		if len(args) == 2 && args[1] == "--locked" {
			locked = true
		} else if len(args) != 1 {
			return reportCommandError(stderr, "pkg sync", fmt.Errorf("usage: oct pkg sync [--locked]"))
		}
		if locked {
			result, err := manager.SyncLocked(".")
			if err != nil {
				return reportCommandError(stderr, "pkg sync", err)
			}
			if _, err := fmt.Fprintln(stdout, "Loaded lock.octagon"); err != nil {
				return err
			}
			for _, dep := range result.Packages {
				if dep.SourceKind == "git" {
					if _, err := fmt.Fprintf(stdout, "Synced %s %s from locked git commit %s\n", dep.Name, dep.Version, dep.ResolvedCommit); err != nil {
						return err
					}
				} else {
					if _, err := fmt.Fprintf(stdout, "Synced %s %s from locked local source %s\n", dep.Name, dep.Version, dep.Source); err != nil {
						return err
					}
				}
			}
			_, err = fmt.Fprintf(stdout, "Package sync complete: %d packages\n", len(result.Packages))
			return err
		}
		result, err := manager.Sync(".")
		if err != nil {
			return reportCommandError(stderr, "pkg sync", err)
		}
		_, err = fmt.Fprintf(stdout, "sync project: %s\nmanifest: %s\ndependencies: %d\n", result.ProjectPath, result.ManifestPath, len(result.Dependencies)+len(result.RegistryDependencies))
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
		for _, dep := range result.RegistryDependencies {
			relDest, relErr := filepath.Rel(result.ProjectPath, dep.Destination)
			if relErr != nil {
				relDest = dep.Destination
			}
			chainNote := ""
			if len(dep.Chain) > 2 {
				chainNote = " required by " + strings.Join(dep.Chain[1:len(dep.Chain)-1], " -> ")
			}
			_, err = fmt.Fprintf(stdout, "Resolved %s %s from registry %s%s\n", dep.Name, dep.Version, dep.Registry, chainNote)
			if err != nil {
				return err
			}
			if dep.SourceKind == "git" {
				_, err = fmt.Fprintf(stdout, "Cloned %s %s ref %s resolved %s\n", dep.Name, dep.Version, dep.Ref, dep.ResolvedCommit)
				if err != nil {
					return err
				}
				if !pkgmgr.IsFullCommitSHA(dep.Ref) {
					_, err = fmt.Fprintf(stdout, "warning: Git ref %q is not a full commit SHA; recorded resolved commit %s\n", dep.Ref, dep.ResolvedCommit)
					if err != nil {
						return err
					}
				}
			}
			_, err = fmt.Fprintf(stdout, "Synced %s %s to %s\n", dep.Name, dep.Version, relDest)
			if err != nil {
				return err
			}
		}
		_, err = fmt.Fprintf(stdout, "Package sync complete: %d package\nsync complete\n", len(result.Dependencies)+len(result.RegistryDependencies))
		return err
	case "build-wrappers":
		if isHelpArg(args[1:]) {
			return writePkgBuildWrappersHelp(stdout)
		}
		allowNative, err := parsePkgBuildWrappersArgs(args[1:])
		if err != nil {
			return reportCommandError(stderr, "pkg build-wrappers", err)
		}
		if !allowNative {
			return reportCommandError(stderr, "pkg build-wrappers", fmt.Errorf("native wrapper builds require --allow-native; use oct pkg wrappers to inspect sidecars without building"))
		}
		targets, err := manager.BuildWrapperBuildTargetsForProject(".")
		if err != nil {
			return reportCommandError(stderr, "pkg build-wrappers", err)
		}
		if err := writePkgBuildWrappersSummary(stdout, targets, allowNative); err != nil {
			return err
		}
		if len(targets) == 0 {
			_, err = fmt.Fprintln(stdout, "No wrapper sidecars to build.")
			return err
		}
		result, err := pkgmgr.BuildWrapperTargets(targets)
		if err != nil {
			return reportCommandError(stderr, "pkg build-wrappers", err)
		}
		platform := targets[0].Platform
		_, err = fmt.Fprintf(stdout, "Built wrapper sidecars: %d\nSet OCT_WRAPPER_PATH=.oct/wrappers/%s to use these sidecars with current runtime discovery.\n", len(result.Targets), platform)
		return err
	case "wrappers":
		if isHelpArg(args[1:]) {
			return writePkgWrappersHelp(stdout)
		}
		registryOut, err := parsePkgWrappersArgs(args[1:])
		if err != nil {
			return reportCommandError(stderr, "pkg wrappers", err)
		}
		plan, err := manager.BuildWrapperPlanForProject(".")
		if err != nil {
			return reportCommandError(stderr, "pkg wrappers", err)
		}
		if err := writePkgWrappersSummary(stdout, plan); err != nil {
			return err
		}
		if registryOut != "" {
			registry, err := pkgmgr.BuildOctxiliaryRegistry(plan)
			if err != nil {
				return reportCommandError(stderr, "pkg wrappers", err)
			}
			if err := pkgmgr.WriteOctxiliaryRegistryOctagon(registryOut, registry); err != nil {
				return reportCommandError(stderr, "pkg wrappers", err)
			}
			if _, err := fmt.Fprintf(stdout, "Wrote Octxiliary registry: %s\n", registryOut); err != nil {
				return err
			}
		}
		_, err = fmt.Fprintln(stdout, "No wrapper sidecars were built or executed.")
		return err
	default:
		return reportCommandError(stderr, "pkg", fmt.Errorf("usage: oct pkg <get|list|sync|lock|registry|add|wrappers|build-wrappers>"))
	}
}

func executePkgRegistry(args []string, stdout io.Writer, stderr io.Writer) error {
	if len(args) < 1 {
		return reportCommandError(stderr, "pkg registry", fmt.Errorf("usage: oct pkg registry <add|list|remove>"))
	}
	if isHelpArg(args) {
		return writePkgRegistryHelp(stdout)
	}
	switch args[0] {
	case "add":
		if len(args) != 3 {
			return reportCommandError(stderr, "pkg registry add", fmt.Errorf("usage: oct pkg registry add <name> <path>"))
		}
		if _, err := pkgmgr.AddRegistry(".", args[1], args[2]); err != nil {
			return reportCommandError(stderr, "pkg registry add", err)
		}
		_, err := fmt.Fprintf(stdout, "Added package registry %s: %s\n", args[1], args[2])
		return err
	case "list":
		if len(args) != 1 {
			return reportCommandError(stderr, "pkg registry list", fmt.Errorf("usage: oct pkg registry list"))
		}
		config, err := pkgmgr.LoadRegistryConfig(".")
		if err != nil {
			return reportCommandError(stderr, "pkg registry list", err)
		}
		if len(config.Registries) == 0 {
			_, err = fmt.Fprintln(stdout, "No package registries configured. Use oct pkg registry add <name> <path>.")
			return err
		}
		if _, err := fmt.Fprintln(stdout, "Configured package registries:"); err != nil {
			return err
		}
		for _, reg := range config.Registries {
			if _, err := fmt.Fprintf(stdout, "* %s %s\n", reg.Name, reg.Path); err != nil {
				return err
			}
		}
		return nil
	case "remove":
		if len(args) != 2 {
			return reportCommandError(stderr, "pkg registry remove", fmt.Errorf("usage: oct pkg registry remove <name>"))
		}
		if _, err := pkgmgr.RemoveRegistry(".", args[1]); err != nil {
			return reportCommandError(stderr, "pkg registry remove", err)
		}
		_, err := fmt.Fprintf(stdout, "Removed package registry %s\n", args[1])
		return err
	default:
		return reportCommandError(stderr, "pkg registry", fmt.Errorf("usage: oct pkg registry <add|list|remove>"))
	}
}

func parsePkgAddArgs(args []string) (string, string, error) {
	if len(args) != 1 && len(args) != 3 {
		return "", "", fmt.Errorf("usage: oct pkg add <Name>@<exact-version> [--registry <name>]")
	}
	if len(args) == 1 {
		return args[0], "", nil
	}
	if args[1] != "--registry" || strings.TrimSpace(args[2]) == "" {
		return "", "", fmt.Errorf("usage: oct pkg add <Name>@<exact-version> [--registry <name>]")
	}
	return args[0], args[2], nil
}

func parsePkgBuildWrappersArgs(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	if len(args) == 1 && args[0] == "--allow-native" {
		return true, nil
	}
	return false, fmt.Errorf("usage: oct pkg build-wrappers --allow-native")
}

func parsePkgWrappersArgs(args []string) (string, error) {
	if len(args) == 0 {
		return "", nil
	}
	if len(args) != 2 || args[0] != "--registry-out" || strings.TrimSpace(args[1]) == "" {
		return "", fmt.Errorf("usage: oct pkg wrappers [--registry-out <path>]")
	}
	return args[1], nil
}

func writePkgWrappersSummary(stdout io.Writer, plan pkgmgr.WrapperBuildPlan) error {
	if _, err := fmt.Fprintln(stdout, "Wrapper build plan:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "native wrappers: %s\n", yesNo(plan.HasNativeWrappers)); err != nil {
		return err
	}
	if plan.HasNativeWrappers {
		if _, err := fmt.Fprintf(stdout, "requires native build permission: %s\n", yesNo(plan.RequiresNativeBuildPermission)); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(stdout, "sidecars: %d\n", len(plan.Sidecars)); err != nil {
		return err
	}
	if len(plan.Sidecars) == 0 {
		_, err := fmt.Fprintln(stdout)
		return err
	}
	if _, err := fmt.Fprintln(stdout); err != nil {
		return err
	}
	for _, sidecar := range plan.Sidecars {
		if _, err := fmt.Fprintf(stdout, "* package %s %s\n", sidecar.PackageName, wrapperPackageVersion(plan, sidecar.PackageName)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "  wrapper: %s\n", sidecar.WrapperName); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "  family: %s\n", sidecar.Family); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "  command: %s\n", sidecar.SidecarCommand); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "  protocol: %s\n", sidecar.Protocol); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "  module: %s\n", sidecar.GoModuleDir); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "  module path: %s\n", sidecar.GoModulePath); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "  functions: %d\n", len(sidecar.Functions)); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(stdout); err != nil {
		return err
	}
	return nil
}

func writePkgBuildWrappersSummary(stdout io.Writer, targets []pkgmgr.WrapperBuildTarget, allowNative bool) error {
	platform := runtime.GOOS + "-" + runtime.GOARCH
	outputDir := filepath.Join(".oct", "wrappers", platform)
	if len(targets) > 0 {
		platform = targets[0].Platform
		outputDir = filepath.Dir(targets[0].OutputPath)
	}
	if _, err := fmt.Fprintln(stdout, "Wrapper sidecar build:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "platform: %s\n", platform); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "output dir: %s\n", outputDir); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "sidecars: %d\n", len(targets)); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(stdout, "native permission: %s\n", yesNo(allowNative)); err != nil {
		return err
	}
	if len(targets) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(stdout); err != nil {
		return err
	}
	for _, target := range targets {
		if _, err := fmt.Fprintf(stdout, "* package %s %s\n", target.PackageName, target.PackageVersion); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "  wrapper: %s\n", target.WrapperName); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "  family: %s\n", target.Family); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "  command: %s\n", target.SidecarCommand); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "  protocol: %s\n", target.Protocol); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "  module: %s\n", target.GoModuleDir); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "  source dir: %s\n", target.SourceDir); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "  output path: %s\n", target.OutputPath); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(stdout, "  functions: %d\n", target.FunctionCount); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(stdout)
	return err
}

func wrapperPackageVersion(plan pkgmgr.WrapperBuildPlan, packageName string) string {
	for _, pkg := range plan.Packages {
		if pkg.PackageName == packageName {
			return pkg.Version
		}
	}
	return ""
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
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
		return fmtOptions{}, fmt.Errorf("usage: oct fmt <file-or-root> [--mode en-llm|en-llm-compact] [--check]")
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
			return fmtOptions{}, fmt.Errorf("usage: oct fmt <file-or-root> [--mode en-llm|en-llm-compact] [--check]")
		}
	}
	return result, nil
}
func writeTopLevelHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct <command> [options]\n\ncommands:\n  run        Run a program\n  build      Compile a program\n  test       Run octest suites\n  artifact   Run artifact generators\n  bench      Run benchmark suites\n  make       Run Make.oct targets\n  fmt        Format Oct source files\n  new        Create a new package scaffold\n  pkg        Package manager commands\n  exp        Run experiment repos\n  version    Print Oct version information\n\nrun 'oct <command> --help' for command details.")
	return err
}

func writeVersion(out io.Writer) error {
	_, err := fmt.Fprintf(out, "oct %s\n", version)
	return err
}

func writePkgHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct pkg <get|list|sync|lock|registry|add|wrappers|build-wrappers>\n\ncommands:\n  registry add <name> <path>    Add a local/source-controlled package registry\n  registry list                 List configured package registries\n  registry remove <name>        Remove a configured package registry\n  add <Name>@<Version>          Add an exact registry dependency to manifest.oct\n  sync [--locked]               Sync exact dependency graph sources; --locked uses lock.octagon\n  lock                          Write optional project-root lock.octagon\n  wrappers [--registry-out p]   Inspect manifest-declared wrappers without building sidecars\n  build-wrappers --allow-native Build declared native wrapper sidecars explicitly\n  get <git-url>                 Fetch a package source into the local cache\n  list                          List cached packages")
	return err
}

func writePkgRegistryHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct pkg registry <add|list|remove>\n\ncommands:\n  add <name> <path>    Add a local package registry directory containing registry.oct\n  list                 List project-local registry configuration\n  remove <name>        Remove a registry by name\n\nexample:\n  oct pkg registry add oct <repo>/Registry")
	return err
}

func writePkgAddHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct pkg add <Name>@<exact-version> [--registry <name>]\nAdd an exact registry dependency to manifest.oct. Example: oct pkg add Mathematics@0.1.0")
	return err
}

func writePkgSyncHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct pkg sync [--locked]\nSync package sources for the current project. --locked requires lock.octagon and syncs the locked exact graph.")
	return err
}

func writePkgLockHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct pkg lock\nWrite optional project-root lock.octagon from the current exact dependency graph.")
	return err
}

func writePkgWrappersHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct pkg wrappers [--registry-out <path>]\nInspect manifest-declared wrapper sidecars without building or running native code.")
	return err
}

func writePkgBuildWrappersHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct pkg build-wrappers --allow-native\nBuild manifest-declared native wrapper sidecars explicitly. Package sync does not build sidecars.")
	return err
}

func writePkgGetHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct pkg get <git-url>\nFetch one package source into the local package cache.")
	return err
}

func writePkgListHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct pkg list\nList packages currently stored in the local package cache.")
	return err
}

func writeNewHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct new <experiment|library|wrapper-library> <Name> [path]\nCreate a deterministic package scaffold. Defaults to Experiments/<Name> for experiments when Experiments/ exists, Libraries/<Name> for libraries and wrapper-libraries when Libraries/ exists, and ./<Name> otherwise. Explicit [path] is preserved.")
	return err
}

func writeInitHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct init <experiment|library|wrapper-library>\nInitialize the current existing directory by creating manifest.oct. Refuses to overwrite an existing manifest.")
	return err
}

func writeInitKindHelp(out io.Writer, kind newpkg.Kind) error {
	_, err := fmt.Fprintf(out, "usage: oct init %s\nCreate manifest.oct for the current existing %s directory. The package name is derived from the directory basename. Existing manifests are never overwritten.\n", kind, kind)
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
func writeMakeHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct make [target] [--file <path>] [--backend direct] [--list] [--dry-run] [--trace]\nRun Make.oct targets with the direct backend. Make.oct is discovered at the project root unless --file is provided. Project configuration belongs in Make.Plan.Config records; compose profiles with record `with` updates. CLI flags select execution behavior only. --backend only accepts direct; Ninja is not implemented.")
	return err
}

func writeFmtHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct fmt <file-or-root> [--mode en-llm|en-llm-compact] [--check]\nFormat Oct source files with deterministic structural whitespace normalization.\nDefault mode: en-llm.\nModes:\n  en-llm          LLM-oriented readable structural formatting.\n  en-llm-compact  LLM-oriented compact structural formatting.\nAuto line wrapping/reflow is intentionally not enabled in v0.1.\nExamples:\n  oct fmt Language/Testing --mode en-llm\n  oct fmt Language/Testing --mode en-llm-compact --check")
	return err
}
func writeTestHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct test <file-or-root> [--suite <name>] [--execution <auto|compiled|interpreted>] [--all-packages]\nRun octest files.\nOptions: --suite, --execution, --all-packages\nExample: oct test Language/Testing --execution compiled --all-packages")
	return err
}
func writeArtifactHelp(out io.Writer) error {
	_, err := fmt.Fprintln(out, "usage: oct artifact <file-or-root> [--execution <compiled|interpreted>]\nRun artifact generation for discovered artifact blocks.\nDefault execution: interpreted.\nExample: oct artifact path/to/file.oct --execution compiled")
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

func parseArtifactOptions(args []string) (tester.ArtifactOptions, []string, error) {
	options := tester.ArtifactOptions{}
	paths := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--execution" {
			if i+1 >= len(args) {
				return tester.ArtifactOptions{}, nil, fmt.Errorf("usage: oct artifact <file-or-root> [--execution <compiled|interpreted>]")
			}
			i++
			options.Execution = strings.TrimSpace(args[i])
			if options.Execution == "" {
				return tester.ArtifactOptions{}, nil, fmt.Errorf("--execution requires a non-empty value")
			}
			if options.Execution != "compiled" && options.Execution != "interpreted" {
				return tester.ArtifactOptions{}, nil, fmt.Errorf("invalid artifact execution mode %q (expected compiled|interpreted)", options.Execution)
			}
			continue
		}
		paths = append(paths, arg)
	}
	if len(paths) == 0 {
		return tester.ArtifactOptions{}, nil, fmt.Errorf("usage: oct artifact <file-or-root> [--execution <compiled|interpreted>]")
	}
	return options, paths, nil
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
				return tester.TestOptions{}, nil, fmt.Errorf("usage: oct test <file-or-root> [--suite <name>] [--execution <auto|compiled|interpreted>] [--all-packages]")
			}
			i++
			options.Execution = strings.TrimSpace(args[i])
			if options.Execution == "" {
				return tester.TestOptions{}, nil, fmt.Errorf("--execution requires a non-empty value")
			}
			continue
		}
		if arg == "--all-packages" {
			options.AllPackages = true
			continue
		}
		paths = append(paths, arg)
	}
	if len(paths) == 0 {
		return tester.TestOptions{}, nil, fmt.Errorf("usage: oct test <file-or-root> [--suite <name>] [--execution <auto|compiled|interpreted>] [--all-packages]")
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

func parseMakeOptions(args []string) (makecmd.Options, error) {
	options := makecmd.Options{Backend: "direct"}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--file":
			if i+1 >= len(args) {
				return options, fmt.Errorf("make --file requires a value")
			}
			i++
			options.File = args[i]
		case "--backend":
			if i+1 >= len(args) {
				return options, fmt.Errorf("make --backend requires a value")
			}
			i++
			options.Backend = args[i]
		case "--list":
			options.List = true
		case "--dry-run":
			options.DryRun = true
		case "--trace":
			options.Trace = true
		default:
			if strings.HasPrefix(args[i], "--") {
				return options, fmt.Errorf("unknown make flag %s", args[i])
			}
			if options.Target != "" {
				return options, fmt.Errorf("oct make accepts at most one target")
			}
			options.Target = args[i]
		}
	}
	return options, nil
}
