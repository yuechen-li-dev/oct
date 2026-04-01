package tester

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/pprof"
	"sort"
	"time"

	"oct/internal/interpret"
	"oct/internal/project"
	"oct/internal/typecheck"
)

type BenchmarkOptions struct {
	OctagonOutPath string
	ProfileMode    string
	ProfileOutPath string
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
		err := interpret.ExecuteFunction(program, benchmark.pkg, benchmark.name, io.Discard)
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
