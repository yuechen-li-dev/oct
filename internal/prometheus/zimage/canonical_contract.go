package zimage

import (
	"fmt"
	"math"
)

// CanonicalNoiseRefiner0SourceRevision is the upstream program authority for
// the repository-owned reference. It is intentionally separate from the
// historical ComfyUI compatibility environment.
const CanonicalNoiseRefiner0SourceRevision = "26f23eda626ffadda020b04ff79488e1d72004cd"

const (
	CanonicalNoiseRefiner0Tokens   = 1024
	CanonicalNoiseRefiner0Width    = 3840
	CanonicalNoiseRefiner0Heads    = 30
	CanonicalNoiseRefiner0HeadSize = 128
	CanonicalNoiseRefiner0FFNWidth = 10240
	CanonicalTextSequenceMultiple  = 32
	CanonicalRopeTheta             = 256
)

var CanonicalRopeAxes = [3]int{32, 48, 48}

// CanonicalImageCoordinate is the exact flattened image-token mapping in the
// pinned Z-Image patchify_and_embed program. It is not a ComfyUI convention.
type CanonicalImageCoordinate struct {
	Frame int `json:"frame"`
	Row   int `json:"row"`
	Col   int `json:"column"`
}

// CanonicalTextPadding follows the pinned source:
// cap_padding_len = (-cap_ori_len) % 32. The text positions begin at frame 1,
// then image positions begin after the padded text range plus the reserved
// frame-zero slot.
func CanonicalTextPadding(rawTextTokens int) (int, error) {
	if rawTextTokens <= 0 {
		return 0, fmt.Errorf("canonical text token count must be positive")
	}
	return (CanonicalTextSequenceMultiple - rawTextTokens%CanonicalTextSequenceMultiple) % CanonicalTextSequenceMultiple, nil
}

func CanonicalImageFrame(rawTextTokens int) (int, error) {
	padding, err := CanonicalTextPadding(rawTextTokens)
	if err != nil {
		return 0, err
	}
	return rawTextTokens + padding + 1, nil
}

// CanonicalImageTokenCoordinate implements create_coordinate_grid(...).flatten
// for a single 1 x 32 x 32 image grid. Token order is row-major:
// token = row*32 + column.
func CanonicalImageTokenCoordinate(rawTextTokens, token int) (CanonicalImageCoordinate, error) {
	if token < 0 || token >= CanonicalNoiseRefiner0Tokens {
		return CanonicalImageCoordinate{}, fmt.Errorf("canonical image token %d outside [0,%d)", token, CanonicalNoiseRefiner0Tokens)
	}
	frame, err := CanonicalImageFrame(rawTextTokens)
	if err != nil {
		return CanonicalImageCoordinate{}, err
	}
	return CanonicalImageCoordinate{Frame: frame, Row: token / 32, Col: token % 32}, nil
}

// CanonicalRopeFrequency is the scalar angle frequency for one rotary pair.
// The source creates arange(0, width, 2) / width, so pair is the scalar-pair
// index rather than a channel index.
func CanonicalRopeFrequency(width, pair int) (float32, error) {
	if width <= 0 || width%2 != 0 || pair < 0 || pair >= width/2 {
		return 0, fmt.Errorf("invalid canonical RoPE width/pair %d/%d", width, pair)
	}
	return float32(1.0 / math.Pow(CanonicalRopeTheta, float64(2*pair)/float64(width))), nil
}

// CanonicalRopeRotate applies the pinned source's complex multiplication:
// (even + i*odd) * (cos(angle) + i*sin(angle)). All public values are float32.
func CanonicalRopeRotate(even, odd, coordinate float32, width, pair int) (float32, float32, error) {
	frequency, err := CanonicalRopeFrequency(width, pair)
	if err != nil {
		return 0, 0, err
	}
	angle := coordinate * frequency
	cosine := float32(math.Cos(float64(angle)))
	sine := float32(math.Sin(float64(angle)))
	return even*cosine - odd*sine, even*sine + odd*cosine, nil
}

// CanonicalRMSNorm is the exact source formula for one vector. It deliberately
// has no mean subtraction and keeps the reduction order left-to-right.
func CanonicalRMSNorm(input, scale []float32, epsilon float32) ([]float32, error) {
	if len(input) == 0 || len(input) != len(scale) || epsilon <= 0 {
		return nil, fmt.Errorf("invalid RMSNorm dimensions or epsilon")
	}
	var sum float32
	for _, value := range input {
		sum += value * value
	}
	denominator := float32(1) / float32(math.Sqrt(float64(sum/float32(len(input))+epsilon)))
	result := make([]float32, len(input))
	for index, value := range input {
		result[index] = value * denominator * scale[index]
	}
	return result, nil
}
