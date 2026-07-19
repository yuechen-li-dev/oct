package zimage

import "fmt"

// NoiseRefiner0M1DIngress freezes the resident M1C-to-M1D seam. The attention
// residual is immutable through both FFN projections and is the only legal
// residual operand for the final channel-wise MLP-gated addition.
type NoiseRefiner0M1DIngress struct {
	TokenCount          uint32
	ModelWidth          uint32
	HiddenWidth         uint32
	AttentionResidualID string
	AdjustedScaleID     string
	TanhGateID          string
	AttentionSpace      string
	NormalizedSpace     string
	W1Space             string
	W3Space             string
	GatedSpace          string
	ProjectedSpace      string
	OutputSpace         string
}

const (
	NoiseRefiner0M1DTokens      = uint32(1024)
	NoiseRefiner0M1DModelWidth  = uint32(3840)
	NoiseRefiner0M1DHiddenWidth = uint32(10240)
)

func NewNoiseRefiner0M1DIngress() NoiseRefiner0M1DIngress {
	return NoiseRefiner0M1DIngress{
		TokenCount:          NoiseRefiner0M1DTokens,
		ModelWidth:          NoiseRefiner0M1DModelWidth,
		HiddenWidth:         NoiseRefiner0M1DHiddenWidth,
		AttentionResidualID: "m1c-attention-residual-fp32",
		AdjustedScaleID:     "m1b-mlp-scale-fp32",
		TanhGateID:          "m1b-mlp-gate-tanh-fp32",
		AttentionSpace:      "ModelEmbedding",
		NormalizedSpace:     "FfnNormalizedEmbedding",
		W1Space:             "FfnHiddenW1",
		W3Space:             "FfnHiddenW3",
		GatedSpace:          "FfnGatedHidden",
		ProjectedSpace:      "FfnProjectedOutput",
		OutputSpace:         "ModelEmbedding",
	}
}

func (ingress NoiseRefiner0M1DIngress) Validate() error {
	if ingress.TokenCount != NoiseRefiner0M1DTokens || ingress.ModelWidth != NoiseRefiner0M1DModelWidth || ingress.HiddenWidth != NoiseRefiner0M1DHiddenWidth {
		return fmt.Errorf("noise_refiner.0 M1D ingress shape does not match the fixed Z-Image contract")
	}
	if ingress.AttentionResidualID == "" || ingress.AdjustedScaleID == "" || ingress.TanhGateID == "" ||
		ingress.AttentionSpace != "ModelEmbedding" || ingress.NormalizedSpace != "FfnNormalizedEmbedding" ||
		ingress.W1Space != "FfnHiddenW1" || ingress.W3Space != "FfnHiddenW3" ||
		ingress.GatedSpace != "FfnGatedHidden" || ingress.ProjectedSpace != "FfnProjectedOutput" ||
		ingress.OutputSpace != "ModelEmbedding" {
		return fmt.Errorf("noise_refiner.0 M1D ingress has an incompatible semantic-space transition")
	}
	return nil
}
