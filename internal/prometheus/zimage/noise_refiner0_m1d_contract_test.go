package zimage

import "testing"

func TestNoiseRefiner0M1DIngressFreezesResidentFfnSeam(t *testing.T) {
	ingress := NewNoiseRefiner0M1DIngress()
	if err := ingress.Validate(); err != nil {
		t.Fatalf("canonical M1D ingress rejected: %v", err)
	}
	if ingress.HiddenWidth != 10240 || ingress.AttentionResidualID != "m1c-attention-residual-fp32" {
		t.Fatalf("M1D drifted from the accepted resident ABI: %+v", ingress)
	}
}

func TestNoiseRefiner0M1DIngressRejectsCrossSpaceFinalResidual(t *testing.T) {
	ingress := NewNoiseRefiner0M1DIngress()
	ingress.OutputSpace = "FfnProjectedOutput"
	if err := ingress.Validate(); err == nil {
		t.Fatal("M1D accepted a final residual that does not return ModelEmbedding")
	}
}
