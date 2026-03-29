package tester

import (
	"fmt"
	"io"
	"sort"
	"time"

	"oct/internal/interpret"
	"oct/internal/project"
	"oct/internal/typecheck"
)

type BenchmarkOptions struct {
	OctagonOutPath string
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
