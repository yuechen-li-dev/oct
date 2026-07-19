package zimage

// This file is the bounded ContextRefiner laboratory. It intentionally owns
// only the source-derived context ingress and two fixed unmodulated blocks; it
// is neither a generic transformer evaluator nor a runtime fallback.

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

const (
	ContextRefinerTokens      = 32 // source SEQ_MULTI_OF padding for the captured 15 text tokens
	ContextRefinerWidth       = 3840
	ContextRefinerHeads       = 30
	ContextRefinerHeadWidth   = 128
	ContextRefinerInputSHA256 = "f6e4a2842dbbdfa7e983fb8260ab15a9e0ea1f763a6615fcaf170ec8cb8838bd"
)

type ContextRefinerCanonicalResult struct {
	Input  []float32
	Final  []float32
	Stages map[string][]float32
}

type contextCanonicalWeights struct {
	qNorm, kNorm, attnOut, qkv []float32
	attnNorm1, attnNorm2       []float32
	w1, w2, w3                 []float32
	ffnNorm1, ffnNorm2         []float32
}

func contextCacheWeights(cacheRoot, block string) (contextCanonicalWeights, error) {
	manifest, err := LoadContextRefinerCacheManifest(cacheRoot, block)
	if err != nil {
		return contextCanonicalWeights{}, err
	}
	byName := map[string]CacheTensor{}
	for _, item := range manifest.Tensors {
		byName[item.SourceName] = item
	}
	load := func(suffix string) ([]float32, error) {
		item, ok := byName[block+suffix]
		if !ok {
			return nil, fmt.Errorf("%s cache lacks %s", block, suffix)
		}
		return canonicalRead16(filepath.Join(ContextRefinerCacheRoot(cacheRoot, block), item.DestinationName), FP16ToFloat32)
	}
	var w contextCanonicalWeights
	for _, item := range []struct {
		suffix string
		dst    *[]float32
	}{
		{".attention.q_norm.weight", &w.qNorm}, {".attention.k_norm.weight", &w.kNorm},
		{".attention.out.weight", &w.attnOut}, {".attention.qkv.weight", &w.qkv},
		{".attention_norm1.weight", &w.attnNorm1}, {".attention_norm2.weight", &w.attnNorm2},
		{".feed_forward.w1.weight", &w.w1}, {".feed_forward.w2.weight", &w.w2},
		{".feed_forward.w3.weight", &w.w3}, {".ffn_norm1.weight", &w.ffnNorm1}, {".ffn_norm2.weight", &w.ffnNorm2},
	} {
		*item.dst, err = load(item.suffix)
		if err != nil {
			return contextCanonicalWeights{}, err
		}
	}
	return w, nil
}

func contextCheckpointTensor(file *os.File, manifest Manifest, name string, shape []uint64) ([]float32, error) {
	tensor, ok := findTensor(manifest, name)
	if !ok || tensor.DType != "BF16" || !sameShape(tensor.Shape, shape) {
		return nil, fmt.Errorf("context ingress tensor contract mismatch: %s", name)
	}
	data := make([]byte, tensor.Bytes)
	if _, err := file.ReadAt(data, int64(tensor.FileRange[0])); err != nil {
		return nil, err
	}
	values := make([]float32, tensor.Elements)
	for i := range values {
		values[i] = math.Float32frombits(uint32(binary.LittleEndian.Uint16(data[i*2:])) << 16)
	}
	return values, nil
}

// BuildCanonicalContextEmbedding reproduces the pinned source boundary:
// captured 15x2560 prompt embeddings are padded by repeating the final row to
// 32, RMS-normalized with cap_embedder.0, then projected by cap_embedder.1.
func BuildCanonicalContextEmbedding(checkpoint, oracleRoot string) ([]float32, error) {
	if err := VerifySHA256(checkpoint, NoiseRefiner0SourceCheckpointSHA256); err != nil {
		return nil, err
	}
	promptPath := filepath.Join(oracleRoot, "run_02", "prompt_embeddings.bin")
	prompt, err := canonicalReadF32(promptPath)
	if err != nil {
		return nil, err
	}
	if len(prompt) != 15*2560 {
		return nil, fmt.Errorf("context prompt boundary dimensions: got %d want %d", len(prompt), 15*2560)
	}
	data, err := os.ReadFile(promptPath)
	if err != nil {
		return nil, err
	}
	if got := sha256Hex(data); got != ContextRefinerInputSHA256 {
		return nil, fmt.Errorf("context prompt boundary identity mismatch: got %s", got)
	}
	manifest, err := ReadManifest(checkpoint, "local-model-cache/z_image_turbo_bf16.safetensors")
	if err != nil {
		return nil, err
	}
	f, err := os.Open(checkpoint)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	normScale, err := contextCheckpointTensor(f, manifest, "cap_embedder.0.weight", []uint64{2560})
	if err != nil {
		return nil, err
	}
	weightPyTorch, err := contextCheckpointTensor(f, manifest, "cap_embedder.1.weight", []uint64{3840, 2560})
	if err != nil {
		return nil, err
	}
	bias, err := contextCheckpointTensor(f, manifest, "cap_embedder.1.bias", []uint64{3840})
	if err != nil {
		return nil, err
	}
	input := make([]float32, ContextRefinerTokens*2560)
	for token := 0; token < ContextRefinerTokens; token++ {
		sourceToken := token
		if sourceToken >= 15 {
			sourceToken = 14
		}
		copy(input[token*2560:], prompt[sourceToken*2560:(sourceToken+1)*2560])
	}
	normalized, err := canonicalNormRows(input, normScale, ContextRefinerTokens, 2560)
	if err != nil {
		return nil, err
	}
	// The package cache layout is [in,out]; convert only this small ingress
	// matrix from the checkpoint's [out,in] layout before its one-time use.
	weight := make([]float32, len(weightPyTorch))
	for out := 0; out < ContextRefinerWidth; out++ {
		for in := 0; in < 2560; in++ {
			weight[in*ContextRefinerWidth+out] = weightPyTorch[out*2560+in]
		}
	}
	return canonicalLinear(normalized, ContextRefinerTokens, 2560, ContextRefinerWidth, weight, bias)
}

func sha256Hex(data []byte) string { sum := sha256.Sum256(data); return fmt.Sprintf("%x", sum[:]) }

func contextRope(q, k []float32) error {
	for token := 0; token < ContextRefinerTokens; token++ {
		axes := [3]float32{float32(token + 1), 0, 0}
		base := 0
		for axis, width := range CanonicalRopeAxes {
			for pair := 0; pair < width/2; pair++ {
				for head := 0; head < ContextRefinerHeads; head++ {
					i := (token*ContextRefinerHeads+head)*ContextRefinerHeadWidth + base + 2*pair
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

func contextAttention(q, k, v []float32, stages map[string][]float32) []float32 {
	output := make([]float32, ContextRefinerTokens*ContextRefinerWidth)
	scores, probabilities := make([]float32, ContextRefinerTokens), make([]float32, ContextRefinerTokens)
	scale := float32(1 / math.Sqrt(ContextRefinerHeadWidth))
	for token := 0; token < ContextRefinerTokens; token++ {
		for head := 0; head < ContextRefinerHeads; head++ {
			maximum := float32(math.Inf(-1))
			for key := 0; key < ContextRefinerTokens; key++ {
				var dot float32
				for c := 0; c < ContextRefinerHeadWidth; c++ {
					dot += q[(token*ContextRefinerHeads+head)*ContextRefinerHeadWidth+c] * k[(key*ContextRefinerHeads+head)*ContextRefinerHeadWidth+c]
				}
				scores[key] = dot * scale
				if scores[key] > maximum {
					maximum = scores[key]
				}
			}
			var sum float32
			for key := range scores {
				probabilities[key] = float32(math.Exp(float64(scores[key] - maximum)))
				sum += probabilities[key]
			}
			for key := range probabilities {
				probabilities[key] /= sum
			}
			if token == 0 && head == 0 {
				canonicalStage(stages, "attention_logits_token0_head0", scores)
				canonicalStage(stages, "attention_probabilities_token0_head0", probabilities)
			}
			for c := 0; c < ContextRefinerHeadWidth; c++ {
				var total float32
				for key := 0; key < ContextRefinerTokens; key++ {
					total += probabilities[key] * v[(key*ContextRefinerHeads+head)*ContextRefinerHeadWidth+c]
				}
				output[(token*ContextRefinerHeads+head)*ContextRefinerHeadWidth+c] = total
			}
		}
	}
	return output
}

func contextAdd(left, right []float32) []float32 {
	out := make([]float32, len(left))
	for i := range out {
		out[i] = left[i] + right[i]
	}
	return out
}

func runCanonicalContextRefiner(input []float32, w contextCanonicalWeights, stages map[string][]float32) ([]float32, error) {
	norm, err := canonicalNormRows(input, w.attnNorm1, ContextRefinerTokens, ContextRefinerWidth)
	if err != nil {
		return nil, err
	}
	canonicalStage(stages, "attention_norm", norm)
	qkv, err := canonicalLinear(norm, ContextRefinerTokens, ContextRefinerWidth, 3*ContextRefinerWidth, w.qkv, nil)
	if err != nil {
		return nil, err
	}
	canonicalStage(stages, "qkv", qkv)
	q, k, v := make([]float32, len(input)), make([]float32, len(input)), make([]float32, len(input))
	for token := 0; token < ContextRefinerTokens; token++ {
		base := token * 3 * ContextRefinerWidth
		copy(q[token*ContextRefinerWidth:], qkv[base:base+ContextRefinerWidth])
		copy(k[token*ContextRefinerWidth:], qkv[base+ContextRefinerWidth:base+2*ContextRefinerWidth])
		copy(v[token*ContextRefinerWidth:], qkv[base+2*ContextRefinerWidth:base+3*ContextRefinerWidth])
	}
	q, err = canonicalNormRows(q, w.qNorm, ContextRefinerTokens*ContextRefinerHeads, ContextRefinerHeadWidth)
	if err != nil {
		return nil, err
	}
	k, err = canonicalNormRows(k, w.kNorm, ContextRefinerTokens*ContextRefinerHeads, ContextRefinerHeadWidth)
	if err != nil {
		return nil, err
	}
	canonicalStage(stages, "q_norm", q)
	canonicalStage(stages, "k_norm", k)
	if err = contextRope(q, k); err != nil {
		return nil, err
	}
	canonicalStage(stages, "q_rope", q)
	canonicalStage(stages, "k_rope", k)
	attn := contextAttention(q, k, v, stages)
	canonicalStage(stages, "attention_aggregation", attn)
	projected, err := canonicalLinear(attn, ContextRefinerTokens, ContextRefinerWidth, ContextRefinerWidth, w.attnOut, nil)
	if err != nil {
		return nil, err
	}
	canonicalStage(stages, "attention_projection", projected)
	attnNorm, err := canonicalNormRows(projected, w.attnNorm2, ContextRefinerTokens, ContextRefinerWidth)
	if err != nil {
		return nil, err
	}
	residual := contextAdd(input, attnNorm)
	canonicalStage(stages, "attention_residual", residual)
	ffnNorm, err := canonicalNormRows(residual, w.ffnNorm1, ContextRefinerTokens, ContextRefinerWidth)
	if err != nil {
		return nil, err
	}
	canonicalStage(stages, "ffn_norm", ffnNorm)
	w1, err := canonicalLinear(ffnNorm, ContextRefinerTokens, ContextRefinerWidth, 10240, w.w1, nil)
	if err != nil {
		return nil, err
	}
	canonicalStage(stages, "w1", w1)
	w3, err := canonicalLinear(ffnNorm, ContextRefinerTokens, ContextRefinerWidth, 10240, w.w3, nil)
	if err != nil {
		return nil, err
	}
	canonicalStage(stages, "w3", w3)
	hidden := make([]float32, len(w1))
	for i := range hidden {
		hidden[i] = (w1[i] / (1 + float32(math.Exp(float64(-w1[i]))))) * w3[i]
	}
	canonicalStage(stages, "ffn_gated_hidden", hidden)
	w2, err := canonicalLinear(hidden, ContextRefinerTokens, 10240, ContextRefinerWidth, w.w2, nil)
	if err != nil {
		return nil, err
	}
	canonicalStage(stages, "w2", w2)
	ffnOut, err := canonicalNormRows(w2, w.ffnNorm2, ContextRefinerTokens, ContextRefinerWidth)
	if err != nil {
		return nil, err
	}
	final := contextAdd(residual, ffnOut)
	canonicalStage(stages, "final_output", final)
	return final, nil
}

// RunCanonicalContextRefiner executes one source-pinned ContextRefiner block.
// Block 0 derives its legal ContextEmbedding ingress; block 1 requires the
// caller to pass the resident FP32 output of block 0 as its boundary.
func RunCanonicalContextRefiner(cacheRoot, oracleRoot, checkpoint, block string, input []float32, capture bool) (ContextRefinerCanonicalResult, error) {
	if len(input) == 0 {
		if block != "context_refiner.0" {
			return ContextRefinerCanonicalResult{}, fmt.Errorf("%s requires a resident ContextEmbedding boundary", block)
		}
		var err error
		input, err = BuildCanonicalContextEmbedding(checkpoint, oracleRoot)
		if err != nil {
			return ContextRefinerCanonicalResult{}, err
		}
	}
	if len(input) != ContextRefinerTokens*ContextRefinerWidth {
		return ContextRefinerCanonicalResult{}, fmt.Errorf("%s context input dimensions: got %d want %d", block, len(input), ContextRefinerTokens*ContextRefinerWidth)
	}
	w, err := contextCacheWeights(cacheRoot, block)
	if err != nil {
		return ContextRefinerCanonicalResult{}, err
	}
	var stages map[string][]float32
	if capture {
		stages = map[string][]float32{}
	}
	canonicalStage(stages, "context_embedding_input", input)
	final, err := runCanonicalContextRefiner(input, w, stages)
	if err != nil {
		return ContextRefinerCanonicalResult{}, err
	}
	return ContextRefinerCanonicalResult{Input: input, Final: final, Stages: stages}, nil
}
