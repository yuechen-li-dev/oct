package interpret

import (
	"fmt"
	"io"
	"path/filepath"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

func BenchmarkInterpretedBatch(b *testing.B) {
	target := filepath.Join("..", "..", "Language", "Concurrency", "Batch", "valid", "batch_valid.octest")
	program, err := project.LoadForTest(target)
	if err != nil {
		b.Fatal(err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		b.Fatal(err)
	}
	interp, err := newInterpreter(program, io.Discard)
	if err != nil {
		b.Fatal(err)
	}
	defer interp.close()
	functions := []struct {
		name      string
		arguments func(Value) []Value
	}{
		{name: "BenchmarkBatchTrivial", arguments: func(items Value) []Value { return []Value{items} }},
		{name: "BenchmarkBatchModerate", arguments: func(items Value) []Value { return []Value{items} }},
		{name: "BenchmarkBatchCaptured", arguments: func(items Value) []Value { return []Value{items, Value{Kind: ValueInt, Int: 17}} }},
		{name: "BenchmarkBatchFallible", arguments: func(items Value) []Value { return []Value{items} }},
	}
	for _, size := range []int{1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024} {
		values := make([]Value, size)
		for index := range values {
			values[index] = Value{Kind: ValueInt, Int: int64(index)}
		}
		items := Value{Kind: ValueArray, Array: values}
		for _, benchmark := range functions {
			function := interp.functions["BatchValid."+benchmark.name]
			arguments := benchmark.arguments(items)
			expectedFailure := benchmark.name == "BenchmarkBatchFallible" && size > 2
			b.Run(fmt.Sprintf("%s/N=%d", benchmark.name, size), func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()
				for range b.N {
					result, err := interp.executeFunction(function, "BatchValid", arguments)
					if err != nil {
						b.Fatal(err)
					}
					if result.hasError != expectedFailure || (!result.hasError && len(result.value.Array) != size) {
						b.Fatalf("unexpected batch result: %+v", result)
					}
				}
				b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*size), "ns/item")
			})
		}
	}
}
