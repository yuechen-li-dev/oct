package zimage

import "testing"

func TestNoiseRefiner1SourceContractIsClosedAndMatchesNoiseRefiner0AssemblyShape(t *testing.T) {
	if len(noiseRefiner1Specs) != 13 || len(noiseRefiner0Specs) != len(noiseRefiner1Specs) {
		t.Fatalf("noise_refiner.1 must freeze the source-derived 13-tensor assembly")
	}
	for index := range noiseRefiner0Specs {
		left, right := noiseRefiner0Specs[index], noiseRefiner1Specs[index]
		if left.transpose != right.transpose || !sameShape(left.shape, right.shape) || left.consumer != right.consumer {
			t.Fatalf("block tensor role %d differs: %#v vs %#v", index, left, right)
		}
	}
	if NoiseRefiner1CacheBytes != 361820672 {
		t.Fatalf("noise_refiner.1 cache envelope drifted: %d", NoiseRefiner1CacheBytes)
	}
}
