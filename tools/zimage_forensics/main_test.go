package main

import "testing"

func TestLayerGroupingAndTransforms(t *testing.T) {
	cases := []struct{ name, wantGroup, wantConsumer string }{
		{"noise_refiner.0.attention.qkv.weight", "noise_refiner", "M1 packed QKV projection; split Q/K/V and transpose each"},
		{"layers.29.feed_forward.w2.weight", "transformer_blocks", "linear projection"},
		{"context_refiner.1.attention_norm1.weight", "context_refiner", "elementwise norm/modulation/constant"},
		{"unexpected.weight", "unclassified", "manual review"},
	}
	for _, tc := range cases {
		group, _, consumer, _ := classify(tc.name)
		if group != tc.wantGroup || consumer != tc.wantConsumer {
			t.Fatalf("%s: got %q / %q", tc.name, group, consumer)
		}
	}
	if index := blockIndex("layers.29.feed_forward.w2.weight"); index == nil || *index != 29 {
		t.Fatal("missing main layer index")
	}
}
