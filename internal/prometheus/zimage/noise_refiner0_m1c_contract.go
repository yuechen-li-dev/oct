package zimage

import "fmt"

// NoiseRefiner0M1CIngress freezes the resident M1B-to-M1C ABI.  The three
// heads are views into the one M1B fused-QKV allocation: M1C must not copy or
// repack them before the fixed attention program consumes them.
type NoiseRefiner0M1CIngress struct {
	TokenCount          uint32
	HeadCount           uint32
	HeadWidth           uint32
	ModelWidth          uint32
	FusedTokenWidth     uint32
	QueryOffset         uint32
	KeyOffset           uint32
	ValueOffset         uint32
	AttentionGateOffset uint32
	OriginalResidualID  string
	SemanticQuery       string
	SemanticKey         string
	SemanticValue       string
	SemanticResidual    string
}

const (
	NoiseRefiner0M1CTokens       = uint32(1024)
	NoiseRefiner0M1CHeads        = uint32(30)
	NoiseRefiner0M1CHeadWidth    = uint32(128)
	NoiseRefiner0M1CModelWidth   = uint32(3840)
	NoiseRefiner0M1CFusedWidth   = uint32(11520)
	NoiseRefiner0M1CRowBytes     = uint64(1024 * 4)
	NoiseRefiner0M1CScratchBytes = NoiseRefiner0M1CRowBytes * 2
)

// NewNoiseRefiner0M1CIngress returns the only legal M1C source layout.  Its
// row-sized score/probability scratch follows the accepted lab reference: it
// processes every query row for one head in sequence, so all 30 heads remain
// non-causal and exact without allocating a 30-head score tensor.
func NewNoiseRefiner0M1CIngress() NoiseRefiner0M1CIngress {
	return NoiseRefiner0M1CIngress{
		TokenCount:          NoiseRefiner0M1CTokens,
		HeadCount:           NoiseRefiner0M1CHeads,
		HeadWidth:           NoiseRefiner0M1CHeadWidth,
		ModelWidth:          NoiseRefiner0M1CModelWidth,
		FusedTokenWidth:     NoiseRefiner0M1CFusedWidth,
		QueryOffset:         0,
		KeyOffset:           NoiseRefiner0M1CModelWidth,
		ValueOffset:         NoiseRefiner0M1CModelWidth * 2,
		AttentionGateOffset: 0,
		OriginalResidualID:  "m1b-input-device-fp32",
		SemanticQuery:       "PositionedQueryHead",
		SemanticKey:         "PositionedKeyHead",
		SemanticValue:       "ValueHead",
		SemanticResidual:    "ModelEmbedding",
	}
}

// Validate rejects the semantic and physical substitutions that would make a
// superficially plausible attention dispatch read the wrong QKV segment.
func (ingress NoiseRefiner0M1CIngress) Validate() error {
	if ingress.TokenCount != NoiseRefiner0M1CTokens ||
		ingress.HeadCount != NoiseRefiner0M1CHeads ||
		ingress.HeadWidth != NoiseRefiner0M1CHeadWidth ||
		ingress.ModelWidth != NoiseRefiner0M1CModelWidth ||
		ingress.FusedTokenWidth != NoiseRefiner0M1CFusedWidth {
		return fmt.Errorf("noise_refiner.0 M1C ingress shape does not match the fixed Z-Image contract")
	}
	if ingress.QueryOffset != 0 || ingress.KeyOffset != NoiseRefiner0M1CModelWidth ||
		ingress.ValueOffset != NoiseRefiner0M1CModelWidth*2 || ingress.AttentionGateOffset != 0 {
		return fmt.Errorf("noise_refiner.0 M1C ingress QKV or gate offset is not canonical")
	}
	if ingress.OriginalResidualID == "" || ingress.SemanticQuery != "PositionedQueryHead" ||
		ingress.SemanticKey != "PositionedKeyHead" || ingress.SemanticValue != "ValueHead" ||
		ingress.SemanticResidual != "ModelEmbedding" {
		return fmt.Errorf("noise_refiner.0 M1C ingress semantic spaces are not canonical")
	}
	return nil
}
