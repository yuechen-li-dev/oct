package build

import (
	"fmt"
	"runtime"
	"sync"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/batchplan"
)

// These controls keep the pre-specialization mechanism available for direct
// before/after comparison. They intentionally benchmark execution shapes, not
// Oct semantics (which remain owned by Language/Concurrency/Batch).
type batchBenchResult struct {
	value int
	err   bool
}

type batchBenchRecord struct {
	left  int
	right int
}

func batchBenchOldWorkerPool[T any, U any](items []T, worker func(T) (U, bool)) ([]U, bool) {
	type itemResult struct {
		index int
		value U
		err   bool
	}
	workerCount := runtime.GOMAXPROCS(0)
	if workerCount > len(items) {
		workerCount = len(items)
	}
	if workerCount < 1 {
		return []U{}, false
	}
	jobs := make(chan int, len(items))
	results := make(chan itemResult, len(items))
	var wg sync.WaitGroup
	for range workerCount {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				value, failed := worker(items[index])
				results <- itemResult{index: index, value: value, err: failed}
			}
		}()
	}
	for index := range items {
		jobs <- index
	}
	close(jobs)
	wg.Wait()
	close(results)
	output := make([]U, len(items))
	failed := false
	for result := range results {
		output[result.index] = result.value
		failed = failed || result.err
	}
	return output, failed
}

func batchBenchSequential[T any, U any](items []T, worker func(T) (U, bool)) ([]U, bool) {
	output := make([]U, len(items))
	failed := false
	for index := range items {
		output[index], failed = worker(items[index])
		if failed {
			return nil, true
		}
	}
	return output, false
}

func batchBenchChunked[T any, U any](items []T, worker func(T) (U, bool)) ([]U, bool) {
	workerCount := runtime.GOMAXPROCS(0)
	if workerCount > len(items) {
		workerCount = len(items)
	}
	if workerCount < 1 {
		return []U{}, false
	}
	output := make([]U, len(items))
	failures := make([]bool, workerCount)
	chunkSize := (len(items) + workerCount - 1) / workerCount
	var wg sync.WaitGroup
	for workerIndex := range workerCount {
		start := workerIndex * chunkSize
		end := min(start+chunkSize, len(items))
		if start >= end {
			continue
		}
		wg.Add(1)
		go func(slot, start, end int) {
			defer wg.Done()
			for index := start; index < end; index++ {
				value, failed := worker(items[index])
				if failed {
					failures[slot] = true
				}
				output[index] = value
			}
		}(workerIndex, start, end)
	}
	wg.Wait()
	for _, failed := range failures {
		if failed {
			return nil, true
		}
	}
	return output, false
}

func batchBenchPlanned[T any, U any](items []T, worker func(T) (U, bool)) ([]U, bool) {
	plan := batchplan.Make(len(items), true)
	if plan.Strategy == batchplan.Sequential {
		return batchBenchSequential(items, worker)
	}
	output := make([]U, len(items))
	failures := make([]bool, plan.WorkerCount)
	var wg sync.WaitGroup
	for slot := 0; slot < plan.WorkerCount; slot++ {
		start := slot * plan.ChunkSize
		end := min(start+plan.ChunkSize, len(items))
		if start >= end {
			continue
		}
		wg.Add(1)
		go func(slot, start, end int) {
			defer wg.Done()
			for index := start; index < end; index++ {
				value, failed := worker(items[index])
				failures[slot] = failures[slot] || failed
				output[index] = value
			}
		}(slot, start, end)
	}
	wg.Wait()
	for _, failed := range failures {
		if failed {
			return nil, true
		}
	}
	return output, false
}

func BenchmarkBatchExecutionShapes(b *testing.B) {
	sizes := []int{1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024}
	strategies := []struct {
		name       string
		run        func([]int, func(int) (int, bool)) ([]int, bool)
		goroutines func(int) int
		channelOps func(int) int
	}{
		{name: "OldWorkerPool", run: batchBenchOldWorkerPool[int, int], goroutines: func(size int) int { return min(size, runtime.GOMAXPROCS(0)) }, channelOps: func(size int) int { return 2 * size }},
		{name: "Sequential", run: batchBenchSequential[int, int], goroutines: func(int) int { return 0 }, channelOps: func(int) int { return 0 }},
		{name: "ChunkedControl", run: batchBenchChunked[int, int], goroutines: func(size int) int { return min(size, runtime.GOMAXPROCS(0)) }, channelOps: func(int) int { return 0 }},
		{name: "Planned", run: batchBenchPlanned[int, int], goroutines: func(size int) int {
			plan := batchplan.Make(size, true)
			if plan.Strategy == batchplan.Sequential {
				return 0
			}
			return plan.WorkerCount
		}, channelOps: func(int) int { return 0 }},
	}
	bodies := []struct {
		name   string
		worker func(int) (int, bool)
	}{
		{name: "TrivialScalar", worker: func(item int) (int, bool) { return item + 1, false }},
		{name: "ModerateArithmetic", worker: func(item int) (int, bool) {
			value := item
			for range 24 {
				value = (value*33 + 17) ^ (value >> 3)
			}
			return value, false
		}},
		{name: "CapturedCall", worker: func(item int) (int, bool) { return batchBenchCapturedCall(item, 17), false }},
		{name: "Fallible", worker: func(item int) (int, bool) { return item + 1, item == 511 }},
	}
	for _, body := range bodies {
		for _, size := range sizes {
			items := make([]int, size)
			for index := range items {
				items[index] = index
			}
			for _, strategy := range strategies {
				b.Run(fmt.Sprintf("%s/N=%d/%s", body.name, size, strategy.name), func(b *testing.B) {
					b.ReportAllocs()
					b.ResetTimer()
					for range b.N {
						output, _ := strategy.run(items, body.worker)
						if len(output) > 0 {
							batchBenchSink = output[len(output)-1]
						}
					}
					b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*size), "ns/item")
					b.ReportMetric(float64(runtime.GOMAXPROCS(0)), "gomaxprocs")
					b.ReportMetric(float64(strategy.goroutines(size)), "goroutines/batch")
					b.ReportMetric(float64(strategy.channelOps(size)), "channelops/batch")
				})
			}
		}
	}
}

func BenchmarkBatchRecordResults(b *testing.B) {
	for _, size := range []int{64, 128, 256, 512, 1024} {
		items := make([]int, size)
		worker := func(item int) (batchBenchRecord, bool) {
			return batchBenchRecord{left: item + 1, right: item * item}, false
		}
		b.Run(fmt.Sprintf("N=%d/OldWorkerPool", size), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				output, _ := batchBenchOldWorkerPool(items, worker)
				batchBenchSink = output[len(output)-1].right
			}
		})
		b.Run(fmt.Sprintf("N=%d/Chunked", size), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				output, _ := batchBenchChunked(items, worker)
				batchBenchSink = output[len(output)-1].right
			}
		})
		b.Run(fmt.Sprintf("N=%d/Planned", size), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				output, _ := batchBenchPlanned(items, worker)
				batchBenchSink = output[len(output)-1].right
			}
		})
	}
}

func batchBenchCapturedCall(item, capture int) int { return item*item + capture }

var batchBenchSink int
