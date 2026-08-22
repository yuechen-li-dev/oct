// Package batchplan owns the deterministic execution-shape policy for Oct's
// homogeneous batch map. It is deliberately a plan, not a scheduler.
package batchplan

import "runtime"

type Strategy uint8

const (
	Sequential Strategy = iota
	ChunkedParallel
)

// SequentialMaxItems and MinItemsPerWorker are benchmark-backed policy
// constants. On the Ryzen 7 7700X baseline recorded for BATCH-SPECIALIZE-M0,
// the old worker pool cost 118-174 ns/item at N=64..512 for cheap bodies while
// a direct loop cost 2-13 ns/item. Range parallelism becomes useful only once
// each worker owns substantial work, so M0 keeps N<=256 sequential and grants
// at least 128 contiguous items to every parallel worker.
const (
	SequentialMaxItems = 256
	MinItemsPerWorker  = 128
)

type Plan struct {
	ItemCount   int
	WorkerCount int
	ChunkSize   int
	Strategy    Strategy
}

func Make(itemCount int, parallelAllowed bool) Plan {
	plan := Plan{ItemCount: itemCount, WorkerCount: 1, ChunkSize: itemCount, Strategy: Sequential}
	if itemCount <= SequentialMaxItems || !parallelAllowed {
		return plan
	}
	maxWorkers := runtime.GOMAXPROCS(0)
	if maxWorkers < 1 {
		maxWorkers = 1
	}
	workersForSize := (itemCount + MinItemsPerWorker - 1) / MinItemsPerWorker
	if maxWorkers > workersForSize {
		maxWorkers = workersForSize
	}
	if maxWorkers <= 1 {
		return plan
	}
	plan.WorkerCount = maxWorkers
	plan.ChunkSize = (itemCount + maxWorkers - 1) / maxWorkers
	plan.Strategy = ChunkedParallel
	return plan
}
