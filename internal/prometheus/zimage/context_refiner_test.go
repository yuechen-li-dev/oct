package zimage

import "testing"

func TestContextRefinerSourceContractIsClosedAndDistinctFromNoiseRefiner(t *testing.T) {
	for _, specs := range [][]cacheSpec{contextRefiner0Specs, contextRefiner1Specs} {
		if len(specs) != 11 {
			t.Fatalf("ContextRefiner must freeze its source-derived 11-tensor assembly, got %d", len(specs))
		}
		for _, spec := range specs {
			if spec.name == "" || spec.name[0:15] != "context_refiner" {
				t.Fatalf("ContextRefiner tensor escaped its closed namespace: %q", spec.name)
			}
			if spec.consumer == "AdaLN bias" || spec.consumer == "AdaLN projection" {
				t.Fatalf("unmodulated ContextRefiner must not contain AdaLN: %#v", spec)
			}
		}
	}
	if ContextRefinerCacheBytes != 353925632 {
		t.Fatalf("ContextRefiner cache envelope drifted: %d", ContextRefinerCacheBytes)
	}
}
