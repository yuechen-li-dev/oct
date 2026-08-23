//go:build integration

package build

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestCompiledQueryM0Benchmarks builds Oct query source through the embeddable
// compiled backend, then benchmarks the generated FLOW facade at the required
// scales. It is an evidence lane, not part of the fast unit suite.
func TestCompiledQueryM0Benchmarks(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "query.oct")
	if err := os.WriteFile(sourcePath, []byte(compiledQueryBenchmarkSource), 0o644); err != nil {
		t.Fatal(err)
	}
	generated, err := EmitGoSource(sourcePath, GoSourceOptions{PackageName: "generated"})
	if err != nil {
		t.Fatal(err)
	}
	generatedDir := filepath.Join(root, "generated")
	if err := os.MkdirAll(generatedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(generatedDir, "generated.go"), generated, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module querym0bench\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "query_benchmark_test.go"), []byte(compiledQueryBenchmarkHarness), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "test", "-run", "^$", "-bench", "^BenchmarkCompiledOctQuery$", "-benchmem", "-benchtime=30ms", "-count=1")
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("compiled Oct query benchmark failed: %v\n%s", err, output)
	}
	t.Logf("compiled Oct query benchmark:\n%s", output)
}

const compiledQueryBenchmarkSource = `package Main

record Job { ID: Int Status: Int }

fn IsReady(job: Job) -> Bool { return job.Status == 1 }
fn JobID(job: Job) -> Int { return job.ID }

query FilterOnly(jobs: Job[]) yields Job { filter IsReady }
query FilterTake10(jobs: Job[]) yields Job { filter IsReady take 10 }
query FilterMap(jobs: Job[]) yields Int { filter IsReady map JobID }
query CountAll(jobs: Job[]) yields Job { map { item } }

fn Main() -> Void {}
`

const compiledQueryBenchmarkHarness = `package querym0bench_test

import (
	"fmt"
	"testing"

	generated "querym0bench/generated"
)

var queryCount int
var queryID int

func BenchmarkCompiledOctQuery(b *testing.B) {
	for _, size := range []int{1_000, 10_000, 100_000} {
		jobs := make([]generated.Main_Job, size)
		for index := range jobs {
			status := 0
			if index%4 == 0 { status = 1 }
			jobs[index] = generated.Main_Job{ID: index, Status: status}
		}
		b.Run(fmt.Sprintf("records=%d", size), func(b *testing.B) {
			b.Run("Filter", func(b *testing.B) {
				b.ReportAllocs(); b.ReportMetric(float64(size), "records/op")
				for iteration := 0; iteration < b.N; iteration++ { queryCount = runFilter(jobs) }
			})
			b.Run("FilterTake10", func(b *testing.B) {
				b.ReportAllocs(); b.ReportMetric(37, "records/op")
				for iteration := 0; iteration < b.N; iteration++ { queryCount = runTake(jobs) }
			})
			b.Run("FilterMap", func(b *testing.B) {
				b.ReportAllocs(); b.ReportMetric(float64(size), "records/op")
				for iteration := 0; iteration < b.N; iteration++ { queryCount, queryID = runMap(jobs) }
			})
			b.Run("Count", func(b *testing.B) {
				b.ReportAllocs(); b.ReportMetric(float64(size), "records/op")
				for iteration := 0; iteration < b.N; iteration++ { queryCount = runCount(jobs) }
			})
		})
	}
}

func runFilter(jobs []generated.Main_Job) int {
	machine := generated.NewFilterOnly(jobs); count := 0
	for { turn, err := machine.Step(); if err != nil { panic(err) }; if turn.DidYield() { count++ }; if turn.Complete() { return count } }
}

func runTake(jobs []generated.Main_Job) int {
	machine := generated.NewFilterTake10(jobs); count := 0
	for { turn, err := machine.Step(); if err != nil { panic(err) }; if turn.DidYield() { count++ }; if turn.Complete() { return count } }
}

func runMap(jobs []generated.Main_Job) (int, int) {
	machine := generated.NewFilterMap(jobs); count, last := 0, 0
	for { turn, err := machine.Step(); if err != nil { panic(err) }; if turn.DidYield() { count++; last, err = turn.Yielded(); if err != nil { panic(err) } }; if turn.Complete() { return count, last } }
}

func runCount(jobs []generated.Main_Job) int {
	machine := generated.NewCountAll(jobs); count := 0
	for { turn, err := machine.Step(); if err != nil { panic(err) }; if turn.DidYield() { count++ }; if turn.Complete() { return count } }
}
`
