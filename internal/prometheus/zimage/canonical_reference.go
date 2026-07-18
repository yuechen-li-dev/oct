package zimage

// This file is deliberately a one-block reference, not a tensor framework.

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

type CanonicalNoiseRefiner0Paths struct {
	CacheRoot  string
	OracleRoot string
}

type CanonicalNoiseRefiner0Result struct {
	Final  []float32
	Stages map[string][]float32
}

type canonicalWeights struct {
	adalnBias, adaln, qNorm, kNorm, attnOut, qkv         []float32
	attnNorm1, attnNorm2, w1, w2, w3, ffnNorm1, ffnNorm2 []float32
}

func canonicalBF16(bits uint16) float32 { return math.Float32frombits(uint32(bits) << 16) }

func canonicalRead16(path string, convert func(uint16) float32) ([]float32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data)%2 != 0 {
		return nil, fmt.Errorf("%s is not a 16-bit payload", path)
	}
	values := make([]float32, len(data)/2)
	for index := range values {
		values[index] = convert(binary.LittleEndian.Uint16(data[index*2:]))
	}
	return values, nil
}

func canonicalLoadWeights(bundle NoiseRefiner0PayloadBundle) (canonicalWeights, error) {
	byName := map[string]CacheTensor{}
	for _, tensor := range bundle.CacheManifest.Tensors {
		byName[tensor.SourceName] = tensor
	}
	load := func(name string) ([]float32, error) {
		tensor, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("canonical reference cache lacks %s", name)
		}
		return canonicalRead16(filepath.Join(bundle.CacheBlockPath, tensor.DestinationName), FP16ToFloat32)
	}
	var w canonicalWeights
	var err error
	for _, item := range []struct {
		name string
		dst  *[]float32
	}{
		{"noise_refiner.0.adaLN_modulation.0.bias", &w.adalnBias}, {"noise_refiner.0.adaLN_modulation.0.weight", &w.adaln},
		{"noise_refiner.0.attention.q_norm.weight", &w.qNorm}, {"noise_refiner.0.attention.k_norm.weight", &w.kNorm},
		{"noise_refiner.0.attention.out.weight", &w.attnOut}, {"noise_refiner.0.attention.qkv.weight", &w.qkv},
		{"noise_refiner.0.attention_norm1.weight", &w.attnNorm1}, {"noise_refiner.0.attention_norm2.weight", &w.attnNorm2},
		{"noise_refiner.0.feed_forward.w1.weight", &w.w1}, {"noise_refiner.0.feed_forward.w2.weight", &w.w2},
		{"noise_refiner.0.feed_forward.w3.weight", &w.w3}, {"noise_refiner.0.ffn_norm1.weight", &w.ffnNorm1}, {"noise_refiner.0.ffn_norm2.weight", &w.ffnNorm2},
	} {
		*item.dst, err = load(item.name)
		if err != nil {
			return canonicalWeights{}, err
		}
	}
	return w, nil
}

func canonicalCopy(value []float32) []float32 {
	out := make([]float32, len(value))
	copy(out, value)
	return out
}

func canonicalF32ToFP16(bits uint32) uint16 {
	sign := uint16((bits >> 16) & 0x8000)
	exponent := int((bits >> 23) & 0xff)
	mantissa := bits & 0x7fffff
	if exponent == 0xff {
		if mantissa == 0 {
			return sign | 0x7c00
		}
		return sign | 0x7e00
	}
	e16 := exponent - 127 + 15
	if e16 >= 31 {
		return sign | 0x7c00
	}
	if e16 <= 0 {
		if e16 < -10 {
			return sign
		}
		mantissa |= 0x800000
		shift := uint(14 - e16)
		out := uint16(mantissa >> shift)
		remainder := mantissa & ((uint32(1) << shift) - 1)
		half := uint32(1) << (shift - 1)
		if remainder > half || (remainder == half && out&1 != 0) {
			out++
		}
		return sign | out
	}
	out := sign | uint16(e16<<10) | uint16(mantissa>>13)
	remainder := mantissa & 0x1fff
	if remainder > 0x1000 || (remainder == 0x1000 && out&1 != 0) {
		out++
	}
	return out
}

func canonicalFP16RoundTrip(values []float32) []float32 {
	out := make([]float32, len(values))
	for i, v := range values {
		out[i] = FP16ToFloat32(canonicalF32ToFP16(math.Float32bits(v)))
	}
	return out
}
func canonicalStage(stages map[string][]float32, name string, value []float32) {
	if stages != nil {
		stages[name] = canonicalCopy(value)
	}
}

// canonicalLinear is the only matrix helper. Weights are cached row-major
// [input,output]; each output accumulation sees input channels 0..N-1 in that
// exact order. Rows are independent but intentionally evaluated serially.
func canonicalLinear(input []float32, rows, in, out int, weight, bias []float32) ([]float32, error) {
	if len(input) != rows*in || len(weight) != in*out || (bias != nil && len(bias) != out) {
		return nil, fmt.Errorf("canonical linear dimension mismatch")
	}
	result := make([]float32, rows*out)
	for row := 0; row < rows; row++ {
		dst := result[row*out : (row+1)*out]
		if bias != nil {
			copy(dst, bias)
		}
		for channel := 0; channel < in; channel++ {
			value := input[row*in+channel]
			weights := weight[channel*out : (channel+1)*out]
			for output := 0; output < out; output++ {
				dst[output] += value * weights[output]
			}
		}
	}
	return result, nil
}

func canonicalNormRows(input, scale []float32, rows, width int) ([]float32, error) {
	if len(input) != rows*width || len(scale) != width {
		return nil, fmt.Errorf("canonical RMSNorm dimension mismatch")
	}
	output := make([]float32, len(input))
	for row := 0; row < rows; row++ {
		value, err := CanonicalRMSNorm(input[row*width:(row+1)*width], scale, 1e-5)
		if err != nil {
			return nil, err
		}
		copy(output[row*width:], value)
	}
	return output, nil
}

func canonicalScale(input, scale []float32) []float32 {
	out := make([]float32, len(input))
	for token := 0; token < CanonicalNoiseRefiner0Tokens; token++ {
		for c := 0; c < CanonicalNoiseRefiner0Width; c++ {
			out[token*CanonicalNoiseRefiner0Width+c] = input[token*CanonicalNoiseRefiner0Width+c] * scale[c]
		}
	}
	return out
}
func canonicalAddGate(residual, update, gate []float32) []float32 {
	out := make([]float32, len(residual))
	for token := 0; token < CanonicalNoiseRefiner0Tokens; token++ {
		for c := 0; c < CanonicalNoiseRefiner0Width; c++ {
			i := token*CanonicalNoiseRefiner0Width + c
			out[i] = residual[i] + gate[c]*update[i]
		}
	}
	return out
}

func canonicalRope(q, k []float32) error {
	for token := 0; token < CanonicalNoiseRefiner0Tokens; token++ {
		coordinate, err := CanonicalImageTokenCoordinate(15, token)
		if err != nil {
			return err
		}
		axes := [3]float32{float32(coordinate.Frame), float32(coordinate.Row), float32(coordinate.Col)}
		base := 0
		for axis, width := range CanonicalRopeAxes {
			for pair := 0; pair < width/2; pair++ {
				for head := 0; head < CanonicalNoiseRefiner0Heads; head++ {
					i := (token*CanonicalNoiseRefiner0Heads+head)*CanonicalNoiseRefiner0HeadSize + base + 2*pair
					q0, q1, err := CanonicalRopeRotate(q[i], q[i+1], axes[axis], width, pair)
					if err != nil {
						return err
					}
					k0, k1, err := CanonicalRopeRotate(k[i], k[i+1], axes[axis], width, pair)
					if err != nil {
						return err
					}
					q[i], q[i+1], k[i], k[i+1] = q0, q1, k0, k1
				}
			}
			base += width
		}
	}
	return nil
}

func canonicalAttention(q, k, v []float32, stages map[string][]float32) []float32 {
	result := make([]float32, CanonicalNoiseRefiner0Tokens*CanonicalNoiseRefiner0Width)
	scores := make([]float32, CanonicalNoiseRefiner0Tokens)
	probs := make([]float32, CanonicalNoiseRefiner0Tokens)
	scale := float32(1 / math.Sqrt(CanonicalNoiseRefiner0HeadSize))
	for token := 0; token < CanonicalNoiseRefiner0Tokens; token++ {
		for head := 0; head < CanonicalNoiseRefiner0Heads; head++ {
			maximum := float32(math.Inf(-1))
			for key := 0; key < CanonicalNoiseRefiner0Tokens; key++ {
				var dot float32
				for c := 0; c < CanonicalNoiseRefiner0HeadSize; c++ {
					dot += q[(token*CanonicalNoiseRefiner0Heads+head)*CanonicalNoiseRefiner0HeadSize+c] * k[(key*CanonicalNoiseRefiner0Heads+head)*CanonicalNoiseRefiner0HeadSize+c]
				}
				scores[key] = dot * scale
				if scores[key] > maximum {
					maximum = scores[key]
				}
			}
			var sum float32
			for key := 0; key < CanonicalNoiseRefiner0Tokens; key++ {
				probs[key] = float32(math.Exp(float64(scores[key] - maximum)))
				sum += probs[key]
			}
			for key := 0; key < CanonicalNoiseRefiner0Tokens; key++ {
				probs[key] /= sum
			}
			if (token == 0 && head == 0) || (token == CanonicalNoiseRefiner0Tokens-1 && head == CanonicalNoiseRefiner0Heads-1) {
				prefix := "attention_logits_token0_head0"
				probPrefix := "attention_probabilities_token0_head0"
				if token != 0 {
					prefix = "attention_logits_token1023_head29"
					probPrefix = "attention_probabilities_token1023_head29"
				}
				canonicalStage(stages, prefix, scores)
				canonicalStage(stages, probPrefix, probs)
			}
			for c := 0; c < CanonicalNoiseRefiner0HeadSize; c++ {
				var total float32
				for key := 0; key < CanonicalNoiseRefiner0Tokens; key++ {
					total += probs[key] * v[(key*CanonicalNoiseRefiner0Heads+head)*CanonicalNoiseRefiner0HeadSize+c]
				}
				result[(token*CanonicalNoiseRefiner0Heads+head)*CanonicalNoiseRefiner0HeadSize+c] = total
			}
		}
	}
	return result
}

// RunCanonicalNoiseRefiner0 executes the exact one-block source contract with
// no framework imports and no historical-output value in its computation.
func RunCanonicalNoiseRefiner0(paths CanonicalNoiseRefiner0Paths, capture bool) (CanonicalNoiseRefiner0Result, error) {
	return runCanonicalNoiseRefiner0(paths, capture, nil)
}

// RunCanonicalNoiseRefiner0WithF16StageCasts is diagnostic-only equipment for
// controlled activation-storage experiments; production must not use it.
func RunCanonicalNoiseRefiner0WithF16StageCasts(paths CanonicalNoiseRefiner0Paths, capture bool, casts map[string]bool) (CanonicalNoiseRefiner0Result, error) {
	return runCanonicalNoiseRefiner0(paths, capture, casts)
}

func runCanonicalNoiseRefiner0(paths CanonicalNoiseRefiner0Paths, capture bool, casts map[string]bool) (CanonicalNoiseRefiner0Result, error) {
	bundle, err := LoadNoiseRefiner0PayloadBundle(NoiseRefiner0PayloadPaths{CacheRoot: paths.CacheRoot, OracleRoot: paths.OracleRoot})
	if err != nil {
		return CanonicalNoiseRefiner0Result{}, err
	}
	w, err := canonicalLoadWeights(bundle)
	if err != nil {
		return CanonicalNoiseRefiner0Result{}, err
	}
	input, err := canonicalRead16(bundle.Input.Path, canonicalBF16)
	if err != nil {
		return CanonicalNoiseRefiner0Result{}, err
	}
	timestep, err := canonicalRead16(bundle.Timestep.Path, canonicalBF16)
	if err != nil {
		return CanonicalNoiseRefiner0Result{}, err
	}
	var stages map[string][]float32
	if capture {
		stages = map[string][]float32{}
	}
	canonicalStage(stages, "timestep_input", timestep)
	adaln, err := canonicalLinear(timestep, 1, 256, 15360, w.adaln, w.adalnBias)
	if err != nil {
		return CanonicalNoiseRefiner0Result{}, err
	}
	canonicalStage(stages, "timestep_linear", adaln)
	parts := make([][]float32, 4)
	for i := range parts {
		parts[i] = canonicalCopy(adaln[i*3840 : (i+1)*3840])
	}
	scaleA, gateA, scaleM, gateM := parts[0], parts[1], parts[2], parts[3]
	canonicalStage(stages, "attention_scale_raw", scaleA)
	canonicalStage(stages, "attention_gate_raw", gateA)
	canonicalStage(stages, "mlp_scale_raw", scaleM)
	canonicalStage(stages, "mlp_gate_raw", gateM)
	for i := 0; i < 3840; i++ {
		gateA[i] = float32(math.Tanh(float64(gateA[i])))
		gateM[i] = float32(math.Tanh(float64(gateM[i])))
		scaleA[i] += 1
		scaleM[i] += 1
	}
	canonicalStage(stages, "attention_scale_adjusted", scaleA)
	canonicalStage(stages, "attention_gate_tanh", gateA)
	canonicalStage(stages, "mlp_scale_adjusted", scaleM)
	canonicalStage(stages, "mlp_gate_tanh", gateM)
	norm, err := canonicalNormRows(input, w.attnNorm1, 1024, 3840)
	if err != nil {
		return CanonicalNoiseRefiner0Result{}, err
	}
	canonicalStage(stages, "attention_norm", norm)
	if casts["attention_norm"] {
		norm = canonicalFP16RoundTrip(norm)
	}
	mod := canonicalScale(norm, scaleA)
	canonicalStage(stages, "attention_modulated", mod)
	qkv, err := canonicalLinear(mod, 1024, 3840, 11520, w.qkv, nil)
	if err != nil {
		return CanonicalNoiseRefiner0Result{}, err
	}
	canonicalStage(stages, "qkv", qkv)
	q := make([]float32, 1024*3840)
	k := make([]float32, len(q))
	v := make([]float32, len(q))
	for t := 0; t < 1024; t++ {
		copy(q[t*3840:], qkv[t*11520:t*11520+3840])
		copy(k[t*3840:], qkv[t*11520+3840:t*11520+7680])
		copy(v[t*3840:], qkv[t*11520+7680:t*11520+11520])
	}
	canonicalStage(stages, "q", q)
	canonicalStage(stages, "k", k)
	canonicalStage(stages, "v", v)
	q, err = canonicalNormRows(q, w.qNorm, 1024*30, 128)
	if err != nil {
		return CanonicalNoiseRefiner0Result{}, err
	}
	k, err = canonicalNormRows(k, w.kNorm, 1024*30, 128)
	if err != nil {
		return CanonicalNoiseRefiner0Result{}, err
	}
	canonicalStage(stages, "q_norm", q)
	canonicalStage(stages, "k_norm", k)
	if err = canonicalRope(q, k); err != nil {
		return CanonicalNoiseRefiner0Result{}, err
	}
	canonicalStage(stages, "q_rope", q)
	canonicalStage(stages, "k_rope", k)
	attn := canonicalAttention(q, k, v, stages)
	canonicalStage(stages, "attention_aggregation", attn)
	projected, err := canonicalLinear(attn, 1024, 3840, 3840, w.attnOut, nil)
	if err != nil {
		return CanonicalNoiseRefiner0Result{}, err
	}
	canonicalStage(stages, "attention_projection", projected)
	norm2, err := canonicalNormRows(projected, w.attnNorm2, 1024, 3840)
	if err != nil {
		return CanonicalNoiseRefiner0Result{}, err
	}
	residual := canonicalAddGate(input, norm2, gateA)
	canonicalStage(stages, "attention_residual", residual)
	ffnNorm, err := canonicalNormRows(residual, w.ffnNorm1, 1024, 3840)
	if err != nil {
		return CanonicalNoiseRefiner0Result{}, err
	}
	canonicalStage(stages, "ffn_norm", ffnNorm)
	if casts["ffn_norm"] {
		ffnNorm = canonicalFP16RoundTrip(ffnNorm)
	}
	ffnIn := canonicalScale(ffnNorm, scaleM)
	canonicalStage(stages, "ffn_modulated", ffnIn)
	w1, err := canonicalLinear(ffnIn, 1024, 3840, 10240, w.w1, nil)
	if err != nil {
		return CanonicalNoiseRefiner0Result{}, err
	}
	canonicalStage(stages, "w1", w1)
	w3, err := canonicalLinear(ffnIn, 1024, 3840, 10240, w.w3, nil)
	if err != nil {
		return CanonicalNoiseRefiner0Result{}, err
	}
	canonicalStage(stages, "w3", w3)
	hidden := make([]float32, len(w1))
	for i := range hidden {
		hidden[i] = (w1[i] / (1 + float32(math.Exp(float64(-w1[i]))))) * w3[i]
	}
	canonicalStage(stages, "ffn_gated_hidden", hidden)
	w2, err := canonicalLinear(hidden, 1024, 10240, 3840, w.w2, nil)
	if err != nil {
		return CanonicalNoiseRefiner0Result{}, err
	}
	canonicalStage(stages, "w2", w2)
	ffnOut, err := canonicalNormRows(w2, w.ffnNorm2, 1024, 3840)
	if err != nil {
		return CanonicalNoiseRefiner0Result{}, err
	}
	final := canonicalAddGate(residual, ffnOut, gateM)
	canonicalStage(stages, "final_output", final)
	return CanonicalNoiseRefiner0Result{Final: final, Stages: stages}, nil
}
