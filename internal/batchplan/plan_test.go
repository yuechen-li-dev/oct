package batchplan

import "testing"

func TestMakeSelectsDeterministicBoundedRanges(t *testing.T) {
	for _, itemCount := range []int{0, 1, SequentialMaxItems} {
		plan := Make(itemCount, true)
		if plan.Strategy != Sequential || plan.WorkerCount != 1 {
			t.Fatalf("Make(%d, true) = %+v, want sequential", itemCount, plan)
		}
	}
	plan := Make(1024, true)
	if plan.WorkerCount < 1 || plan.WorkerCount > 8 || plan.ChunkSize < MinItemsPerWorker {
		t.Fatalf("unexpected bounded plan: %+v", plan)
	}
	if nested := Make(1024, false); nested.Strategy != Sequential || nested.WorkerCount != 1 {
		t.Fatalf("nested plan = %+v, want sequential", nested)
	}
}
