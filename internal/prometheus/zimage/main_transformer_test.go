package zimage

import "testing"

func TestMainTransformerRepresentativeIsClosedAndModulated(t *testing.T) {
	if MainTransformerBlock != "layers.0" || len(mainTransformer0Specs) != 13 {
		t.Fatalf("M2C must name one closed 13-tensor representative, got %q/%d", MainTransformerBlock, len(mainTransformer0Specs))
	}
	if MainTransformerTokens != MainTransformerImageTokens+MainTransformerContextTokens ||
		MainTransformerWidth != 3840 || MainTransformerHeads != 30 ||
		MainTransformerHeadWidth != 128 || MainTransformerHiddenWidth != 10240 {
		t.Fatalf("joint MainTransformer geometry drifted")
	}
	if MainTransformer0CacheAggregateSHA256 == "" || MainTransformer0CacheAggregateSHA256 == NoiseRefiner0CacheAggregateSHA256 {
		t.Fatalf("representative cache needs its own immutable aggregate identity")
	}
	seenAdaLN := 0
	for _, spec := range mainTransformer0Specs {
		if len(spec.name) < len(MainTransformerBlock) || spec.name[:len(MainTransformerBlock)] != MainTransformerBlock {
			t.Fatalf("representative tensor escaped its closed namespace: %q", spec.name)
		}
		if spec.consumer == "AdaLN bias" || spec.consumer == "AdaLN projection" {
			seenAdaLN++
		}
	}
	if seenAdaLN != 2 {
		t.Fatalf("main block must retain the source AdaLN contract, got %d roles", seenAdaLN)
	}
}
