//go:build integration

package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yuechen-li-dev/oct/internal/lex"
	"github.com/yuechen-li-dev/oct/internal/parse"
	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/source"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

func TestParametricM0CompileScaling(t *testing.T) {
	for _, count := range []int{1, 10, 100} {
		root, err := os.MkdirTemp("../../.tmp", "parametric-scale-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(root) })
		path := filepath.Join(root, fmt.Sprintf("scale_%d.oct", count))
		sourceText := parametricScaleSource(count)
		if err := os.WriteFile(path, []byte(sourceText), 0o644); err != nil {
			t.Fatal(err)
		}
		parseStarted := time.Now()
		lexed, err := lex.Analyze(source.File{Path: path, Text: sourceText})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := parse.BuildFile(lexed); err != nil {
			t.Fatal(err)
		}
		parseElapsed := time.Since(parseStarted)
		loadStarted := time.Now()
		program, err := project.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		loadElapsed := time.Since(loadStarted)
		typeStarted := time.Now()
		if err := typecheck.CheckProgram(program); err != nil {
			t.Fatal(err)
		}
		typeElapsed := time.Since(typeStarted)
		emitStarted := time.Now()
		module, err := lowerProgram(program, compileOptions{})
		if err != nil {
			t.Fatal(err)
		}
		generated, err := emitGo(module)
		if err != nil {
			t.Fatal(err)
		}
		emitElapsed := time.Since(emitStarted)
		stats := program.Parametrics
		if stats.FunctionInstantiations != count {
			t.Fatalf("count=%d instantiated=%d", count, stats.FunctionInstantiations)
		}
		t.Logf("scale=%d parse_ns=%d project_load_ns=%d monomorphize_ns=%d typecheck_ns=%d lower_emit_ns=%d generated_bytes=%d instantiations=%d", count, parseElapsed.Nanoseconds(), loadElapsed.Nanoseconds(), stats.ElaborationNanoseconds, typeElapsed.Nanoseconds(), emitElapsed.Nanoseconds(), len(generated), stats.FunctionInstantiations)
	}
}

func parametricScaleSource(count int) string {
	var source strings.Builder
	source.WriteString("package Main\n\ntemplate fn Identity<T>(value: T) -> T { return value }\n\n")
	for i := 0; i < count; i++ {
		fmt.Fprintf(&source, "record R%d { Value: Int }\n", i)
	}
	source.WriteString("\nfn Touch() -> Int {\n    var total = 0\n")
	for i := 0; i < count; i++ {
		fmt.Fprintf(&source, "    let r%d = Identity<R%d>(R%d { Value: %d })\n    total = total + r%d.Value\n", i, i, i, i, i)
	}
	source.WriteString("    return total\n}\n\nfn Main() -> Void {}\n")
	return source.String()
}

// TestParametricM0RuntimeParityBenchmark is an opt-in evidence lane. Both
// variants pass through the same existing FLOW backend; the benchmark checks
// that early monomorphization adds no execution-side generic mechanism.
func TestParametricM0RuntimeParityBenchmark(t *testing.T) {
	source, err := EmitGoSource("../../docs/internal/evidence/OCT_PARAMETRICS_M0/benchmarks/query_runtime.oct", GoSourceOptions{PackageName: "generated"})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	generatedDir := filepath.Join(root, "generated")
	if err := os.MkdirAll(generatedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(generatedDir, "generated.go"), source, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module parametricm0bench\n\ngo 1.23\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "query_benchmark_test.go"), []byte(parametricM0BenchmarkHarness), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "test", "-run", "^$", "-bench", "^BenchmarkQueryParity$", "-benchmem", "-benchtime=500ms", "-count=5")
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("parametric runtime parity benchmark failed: %v\n%s", err, output)
	}
	t.Logf("parametric versus handwritten query benchmark:\n%s", output)
}

const parametricM0BenchmarkHarness = `package parametricm0bench_test

import (
    "testing"
    generated "parametricm0bench/generated"
)

var parityCount int

func BenchmarkQueryParity(b *testing.B) {
    jobs := make([]generated.Main_Job, 10000)
    for i := range jobs { jobs[i] = generated.Main_Job{ID: i, Status: i % 4} }
    b.Run("handwritten", func(b *testing.B) {
        b.ReportAllocs()
        for i := 0; i < b.N; i++ { parityCount = runHandwritten(jobs) }
    })
    b.Run("parametric", func(b *testing.B) {
        b.ReportAllocs()
        for i := 0; i < b.N; i++ { parityCount = runParametric(jobs) }
    })
}

func runParametric(jobs []generated.Main_Job) int {
    machine := generated.NewParametricFilter__Job(jobs)
    count := 0
    for { turn, err := machine.Step(); if err != nil { panic(err) }; if turn.DidYield() { count++ }; if turn.Complete() { return count } }
}

func runHandwritten(jobs []generated.Main_Job) int {
    machine := generated.NewHandwrittenFilter(jobs)
    count := 0
    for { turn, err := machine.Step(); if err != nil { panic(err) }; if turn.DidYield() { count++ }; if turn.Complete() { return count } }
}
`
