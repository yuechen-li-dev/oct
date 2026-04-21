package tester

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/pprof"
	"sort"
	"strings"
	"time"

	"oct/internal/build"
	"oct/internal/interpret"
	"oct/internal/project"
	"oct/internal/typecheck"
)

type BenchmarkOptions struct {
	OctagonOutPath string
	ProfileMode    string
	ProfileOutPath string
	Filter         string
}

type BenchmarkRun struct {
	Cases []BenchmarkCaseResult
}

type BenchmarkCaseResult struct {
	Name       string
	DurationNs int64
}

type benchmarkCase struct {
	pkg      string
	filePath string
	name     string
}

func ExecuteBenchmarks(path string, stdout io.Writer, options BenchmarkOptions) error {
	return executeForPathOrExperiment(path, stdout, "bench", func(singlePath string, singleStdout io.Writer) error {
		return executeBenchmarksSingleRoot(singlePath, singleStdout, options)
	})
}

func executeBenchmarksSingleRoot(path string, stdout io.Writer, options BenchmarkOptions) error {
	program, err := project.LoadForTest(path)
	if err != nil {
		return err
	}
	if err := typecheck.CheckProgram(program); err != nil {
		return err
	}

	var benchmarks []benchmarkCase
	for pkgName, pkg := range program.Packages {
		for _, fn := range pkg.Functions {
			if !fn.IsBenchmark {
				continue
			}
			benchmarks = append(benchmarks, benchmarkCase{pkg: pkgName, filePath: fn.SourcePath, name: fn.Name})
		}
	}

	sort.Slice(benchmarks, func(i, j int) bool {
		if benchmarks[i].pkg != benchmarks[j].pkg {
			return benchmarks[i].pkg < benchmarks[j].pkg
		}
		if benchmarks[i].filePath != benchmarks[j].filePath {
			return benchmarks[i].filePath < benchmarks[j].filePath
		}
		return benchmarks[i].name < benchmarks[j].name
	})

	if len(benchmarks) == 0 {
		return fmt.Errorf("no [Benchmark] functions found")
	}
	if options.Filter != "" {
		filtered := make([]benchmarkCase, 0, len(benchmarks))
		for _, benchmark := range benchmarks {
			qualified := fmt.Sprintf("%s.%s", benchmark.pkg, benchmark.name)
			if strings.Contains(qualified, options.Filter) {
				filtered = append(filtered, benchmark)
			}
		}
		benchmarks = filtered
		if len(benchmarks) == 0 {
			return fmt.Errorf("no benchmarks matched filter %q", options.Filter)
		}
	}

	failed := 0
	run := BenchmarkRun{Cases: make([]BenchmarkCaseResult, 0, len(benchmarks))}
	stopProfile, err := startBenchmarkProfile(path, options)
	if err != nil {
		return err
	}
	if stopProfile != nil {
		defer stopProfile()
	}
	for _, benchmark := range benchmarks {
		qualified := fmt.Sprintf("%s.%s", benchmark.pkg, benchmark.name)
		_, _ = fmt.Fprintf(stdout, "RUN  %s (%s)\n", qualified, shortPath(path, benchmark.filePath))
		start := time.Now()
		err := executeBenchmarkCompiled(program, benchmark)
		duration := time.Since(start)
		run.Cases = append(run.Cases, BenchmarkCaseResult{Name: qualified, DurationNs: duration.Nanoseconds()})
		if err != nil {
			failed++
			_, _ = fmt.Fprintf(stdout, "FAIL %s %s (%s): %v\n", qualified, duration.Round(time.Microsecond), shortPath(path, benchmark.filePath), err)
			continue
		}
		_, _ = fmt.Fprintf(stdout, "PASS %s %s (%s)\n", qualified, duration.Round(time.Microsecond), shortPath(path, benchmark.filePath))
	}

	_, _ = fmt.Fprintf(stdout, "Result: %d benchmark(s) passed, %d failed\n", len(benchmarks)-failed, failed)
	if failed > 0 {
		return fmt.Errorf("%d benchmark(s) failed", failed)
	}
	if options.OctagonOutPath != "" {
		if err := interpret.WriteOctagon(options.OctagonOutPath, benchmarkRunToOctagonValue(run)); err != nil {
			return err
		}
	}
	return nil
}

func executeBenchmarkCompiled(program project.Program, benchmark benchmarkCase) error {
	pkg, ok := program.Packages[benchmark.pkg]
	if !ok {
		return fmt.Errorf("unknown benchmark package %q", benchmark.pkg)
	}
	runnerPath, cleanupRunner, err := writeBenchmarkRunner(pkg.Directory, benchmark.pkg, benchmark.name)
	if err != nil {
		return err
	}
	defer cleanupRunner()

	result, err := build.CompileForTest(runnerPath)
	if err != nil {
		return err
	}
	defer cleanupArtifact(result.ArtifactPath)

	cmd := exec.Command(result.ArtifactPath)
	if prefix := interpret.CurrentOutputPathPrefix(); prefix != "" {
		cmd.Env = append(os.Environ(), "OCT_OUTPUT_PATH_PREFIX="+prefix)
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			return fmt.Errorf("run compiled benchmark binary: %w", err)
		}
		return fmt.Errorf("run compiled benchmark binary: %w: %s", err, msg)
	}
	return nil
}

func writeBenchmarkRunner(pkgDir string, pkgName string, benchmarkName string) (string, func(), error) {
	fileName := fmt.Sprintf("zz_oct_bench_runner_%d_%d.oct", os.Getpid(), time.Now().UnixNano())
	runnerPath := filepath.Join(pkgDir, fileName)
	source := strings.Join([]string{
		"package " + pkgName,
		"fn main() -> Int {",
		fmt.Sprintf("    %s()", benchmarkName),
		"    return 0",
		"}",
		"",
	}, "\n")
	if err := os.WriteFile(runnerPath, []byte(source), 0o644); err != nil {
		return "", nil, err
	}
	cleanup := func() {
		_ = os.Remove(runnerPath)
	}
	return runnerPath, cleanup, nil
}

func cleanupArtifact(path string) {
	_ = os.Remove(path)
	_ = os.Remove(path + ".mir")
}

func DefaultCPUProfilePath(path string) string {
	info, err := os.Stat(path)
	if err == nil && !info.IsDir() {
		dir := filepath.Dir(path)
		base := filepath.Base(path)
		return filepath.Join(dir, base+".bench.cpu.pprof")
	}
	return filepath.Join(path, "bench.cpu.pprof")
}

func startBenchmarkProfile(path string, options BenchmarkOptions) (func(), error) {
	if options.ProfileMode == "" {
		return nil, nil
	}
	if options.ProfileMode != "cpu" {
		return nil, fmt.Errorf("unsupported benchmark profile mode: %s", options.ProfileMode)
	}
	profilePath := options.ProfileOutPath
	if profilePath == "" {
		profilePath = DefaultCPUProfilePath(path)
	}
	if err := os.MkdirAll(filepath.Dir(profilePath), 0o755); err != nil {
		return nil, err
	}
	file, err := os.Create(profilePath)
	if err != nil {
		return nil, err
	}
	if err := pprof.StartCPUProfile(file); err != nil {
		_ = file.Close()
		return nil, err
	}
	return func() {
		pprof.StopCPUProfile()
		_ = file.Close()
	}, nil
}

func benchmarkRunToOctagonValue(run BenchmarkRun) interpret.Value {
	cases := make([]interpret.Value, 0, len(run.Cases))
	for _, result := range run.Cases {
		cases = append(cases, interpret.Value{
			Kind: interpret.ValueRecord,
			Record: interpret.RecordValue{
				TypeName:   "BenchmarkCaseResult",
				FieldOrder: []string{"Name", "DurationNs"},
				Fields: map[string]interpret.Value{
					"Name":       {Kind: interpret.ValueString, Text: result.Name},
					"DurationNs": {Kind: interpret.ValueInt, Int: result.DurationNs},
				},
			},
		})
	}

	return interpret.Value{
		Kind: interpret.ValueRecord,
		Record: interpret.RecordValue{
			TypeName:   "BenchmarkRun",
			FieldOrder: []string{"Cases"},
			Fields: map[string]interpret.Value{
				"Cases": {Kind: interpret.ValueArray, Array: cases},
			},
		},
	}
}
