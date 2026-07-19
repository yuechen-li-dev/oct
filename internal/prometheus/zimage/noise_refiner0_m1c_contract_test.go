package zimage

import "testing"

func TestNoiseRefiner0M1CIngressFreezesFusedQKVViewsAndRowScratch(t *testing.T) {
	ingress := NewNoiseRefiner0M1CIngress()
	if err := ingress.Validate(); err != nil {
		t.Fatalf("canonical M1C ingress rejected: %v", err)
	}
	if NoiseRefiner0M1CScratchBytes != 8192 {
		t.Fatalf("M1C must retain the accepted two-row FP32 softmax scratch: got %d", NoiseRefiner0M1CScratchBytes)
	}
	if ingress.ValueOffset+ingress.ModelWidth != ingress.FusedTokenWidth {
		t.Fatalf("V view does not terminate the fused QKV row: %+v", ingress)
	}
}

func TestNoiseRefiner0M1CIngressRejectsCrossSpaceAndStrideDrift(t *testing.T) {
	ingress := NewNoiseRefiner0M1CIngress()
	ingress.SemanticQuery = "QueryHead"
	if err := ingress.Validate(); err == nil {
		t.Fatal("unpositioned query semantic space was accepted")
	}
	ingress = NewNoiseRefiner0M1CIngress()
	ingress.ValueOffset = ingress.KeyOffset
	if err := ingress.Validate(); err == nil {
		t.Fatal("K/V aliasing was accepted")
	}
}
