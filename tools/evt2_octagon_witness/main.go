// Command evt2_octagon_witness exports bounded, typed witness inputs from an
// already validated local EVT-2 diagnostic capture. It intentionally exports
// one complete row, not a general tensor interchange format.
package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/prometheus/zimage"
)

type w2Analysis struct {
	Schema           string  `json:"schema"`
	InputCount       int     `json:"input_count"`
	FirstNonFinite   int     `json:"first_non_finite_index"`
	ProductMin       float32 `json:"product_min"`
	ProductMax       float32 `json:"product_max"`
	PartialSumMin    float32 `json:"partial_sum_min"`
	PartialSumMax    float32 `json:"partial_sum_max"`
	FixedOrderOutput float32 `json:"fixed_order_fp32_output"`
	CapturedW2Output float32 `json:"captured_w2_output"`
	Difference       float32 `json:"captured_minus_fixed_order"`
}

type w2PolicyAnalysis struct {
	Schema                 string  `json:"schema"`
	Terms                  int     `json:"terms"`
	WeightExactCount       int     `json:"weight_exact_count"`
	WeightMaxAbsDifference float32 `json:"weight_max_abs_difference"`
	BF16ExpandedOutput     float32 `json:"bf16_expanded_fp32_output"`
	FP16ExpandedOutput     float32 `json:"fp16_expanded_fp32_output"`
	OutputDifference       float32 `json:"bf16_minus_fp16"`
}

type linearPolicySpec struct {
	SourceName  string
	Offset      int
	InputStage  string
	OutputStage string
	Outputs     int
}

func readF32(path string, count int) ([]float32, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) < count*4 {
		return nil, fmt.Errorf("%s has %d bytes, want at least %d", path, len(b), count*4)
	}
	out := make([]float32, count)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return out, nil
}

func readFP16(path string) ([]float32, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b)%2 != 0 {
		return nil, fmt.Errorf("%s is not a 16-bit payload", path)
	}
	out := make([]float32, len(b)/2)
	for i := range out {
		out[i] = zimage.FP16ToFloat32(binary.LittleEndian.Uint16(b[i*2:]))
	}
	return out, nil
}

func readFP16Column(path string, rows, columns, column int) ([]float32, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(b) != rows*columns*2 || column < 0 || column >= columns {
		return nil, fmt.Errorf("invalid FP16 matrix %s", path)
	}
	out := make([]float32, rows)
	for row := range out {
		offset := 2 * (row*columns + column)
		out[row] = zimage.FP16ToFloat32(binary.LittleEndian.Uint16(b[offset:]))
	}
	return out, nil
}

func readBF16(path string, offset, count int) ([]float32, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := f.Seek(int64(offset), 0); err != nil {
		return nil, err
	}
	b := make([]byte, count*2)
	if _, err := f.Read(b); err != nil {
		return nil, err
	}
	out := make([]float32, count)
	for i := range out {
		out[i] = math.Float32frombits(uint32(binary.LittleEndian.Uint16(b[i*2:])) << 16)
	}
	return out, nil
}

func render(values []float32) string {
	parts := make([]string, len(values))
	for i, value := range values {
		text := fmt.Sprintf("%.9g", value)
		if !strings.ContainsAny(text, ".eE") {
			text += ".0"
		}
		parts[i] = text
	}
	return strings.Join(parts, ", ")
}

func main() {
	mode := flag.String("mode", "qnorm", "witness kind: qnorm or attention")
	stages := flag.String("stages", "", "corrected capture stages directory")
	cacheBlock := flag.String("cache-block", "", "validated cache block directory")
	out := flag.String("out", "", "Octagon output path")
	analysisOut := flag.String("analysis-out", "", "optional W2 range-analysis JSON output")
	rawRange := flag.String("raw-range", "", "official contiguous BF16 source range for w2policy mode")
	tensor := flag.String("tensor", "", "qkv, w1, or w3 for linearpolicy mode")
	flag.Parse()
	if *stages == "" || *out == "" || (*mode == "qnorm" && *cacheBlock == "") {
		flag.Usage()
		os.Exit(2)
	}
	if *mode == "attention" {
		logits, err := readF32(filepath.Join(*stages, "attention_logits_token0_head0.f32.bin"), 1024)
		if err != nil {
			panic(err)
		}
		expected, err := readF32(filepath.Join(*stages, "attention_probabilities_token0_head0.f32.bin"), 1024)
		if err != nil {
			panic(err)
		}
		text := "AttentionRowWitness {\n    Logits: [" + render(logits) + "]\n    Expected: [" + render(expected) + "]\n}\n"
		if err := os.WriteFile(*out, []byte(text), 0644); err != nil {
			panic(err)
		}
		return
	}
	if *mode == "w2" {
		manifestBytes, err := os.ReadFile(filepath.Join(*cacheBlock, "manifest.json"))
		if err != nil {
			panic(err)
		}
		var manifest zimage.CacheManifest
		if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
			panic(err)
		}
		w2Name := ""
		for _, tensor := range manifest.Tensors {
			if tensor.SourceName == "noise_refiner.0.feed_forward.w2.weight" {
				w2Name = tensor.DestinationName
				break
			}
		}
		if w2Name == "" {
			panic("W2 absent from cache manifest")
		}
		input, err := readF32(filepath.Join(*stages, "ffn_gated_hidden.f32.bin"), 10240)
		if err != nil {
			panic(err)
		}
		expected, err := readF32(filepath.Join(*stages, "w2.f32.bin"), 1)
		if err != nil {
			panic(err)
		}
		weight, err := readFP16Column(filepath.Join(*cacheBlock, w2Name), 10240, 3840, 0)
		if err != nil {
			panic(err)
		}
		text := "W2DotWitness {\n    Input: [" + render(input) + "]\n    Weight: [" + render(weight) + "]\n    Expected: " + render(expected) + "\n}\n"
		if err := os.WriteFile(*out, []byte(text), 0644); err != nil {
			panic(err)
		}
		if *analysisOut != "" {
			analysis := w2Analysis{Schema: "oct.prometheus.evt2.o7.w2-dot-range.v1", InputCount: len(input), FirstNonFinite: -1, CapturedW2Output: expected[0]}
			var sum float32
			for i := range input {
				product := input[i] * weight[i]
				sum += product
				if i == 0 || product < analysis.ProductMin {
					analysis.ProductMin = product
				}
				if i == 0 || product > analysis.ProductMax {
					analysis.ProductMax = product
				}
				if i == 0 || sum < analysis.PartialSumMin {
					analysis.PartialSumMin = sum
				}
				if i == 0 || sum > analysis.PartialSumMax {
					analysis.PartialSumMax = sum
				}
				if analysis.FirstNonFinite < 0 && (math.IsNaN(float64(product)) || math.IsInf(float64(product), 0) || math.IsNaN(float64(sum)) || math.IsInf(float64(sum), 0)) {
					analysis.FirstNonFinite = i
				}
			}
			analysis.FixedOrderOutput = sum
			analysis.Difference = expected[0] - sum
			encoded, err := json.MarshalIndent(analysis, "", "  ")
			if err != nil {
				panic(err)
			}
			if err := os.WriteFile(*analysisOut, append(encoded, '\n'), 0644); err != nil {
				panic(err)
			}
		}
		return
	}
	if *mode == "w2policy" {
		if *rawRange == "" {
			panic("w2policy requires -raw-range")
		}
		manifestBytes, err := os.ReadFile(filepath.Join(*cacheBlock, "manifest.json"))
		if err != nil {
			panic(err)
		}
		var manifest zimage.CacheManifest
		if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
			panic(err)
		}
		w2Name := ""
		for _, tensor := range manifest.Tensors {
			if tensor.SourceName == "noise_refiner.0.feed_forward.w2.weight" {
				w2Name = tensor.DestinationName
				break
			}
		}
		if w2Name == "" {
			panic("W2 absent from cache manifest")
		}
		input, err := readF32(filepath.Join(*stages, "ffn_gated_hidden.f32.bin"), 10240)
		if err != nil {
			panic(err)
		}
		fp16Weight, err := readFP16Column(filepath.Join(*cacheBlock, w2Name), 10240, 3840, 0)
		if err != nil {
			panic(err)
		}
		// The source range begins at 11584667040; W2's BF16 payload begins at
		// 11789185952. Source W2 is [out,in], so output channel zero is its
		// first contiguous 10240-value row.
		bf16Weight, err := readBF16(*rawRange, 204518912, 10240)
		if err != nil {
			panic(err)
		}
		text := "W2PolicyWitness {\n    Input: [" + render(input) + "]\n    BF16Weight: [" + render(bf16Weight) + "]\n    FP16Weight: [" + render(fp16Weight) + "]\n}\n"
		if err := os.WriteFile(*out, []byte(text), 0644); err != nil {
			panic(err)
		}
		if *analysisOut != "" {
			analysis := w2PolicyAnalysis{Schema: "oct.prometheus.evt2.o8.w2-bf16-fp16-policy.v1", Terms: 10240}
			var bf16Sum, fp16Sum float32
			for i := range input {
				if bf16Weight[i] == fp16Weight[i] {
					analysis.WeightExactCount++
				}
				delta := float32(math.Abs(float64(bf16Weight[i] - fp16Weight[i])))
				if delta > analysis.WeightMaxAbsDifference {
					analysis.WeightMaxAbsDifference = delta
				}
				bf16Sum += input[i] * bf16Weight[i]
				fp16Sum += input[i] * fp16Weight[i]
			}
			analysis.BF16ExpandedOutput, analysis.FP16ExpandedOutput = bf16Sum, fp16Sum
			analysis.OutputDifference = bf16Sum - fp16Sum
			encoded, err := json.MarshalIndent(analysis, "", "  ")
			if err != nil {
				panic(err)
			}
			if err := os.WriteFile(*analysisOut, append(encoded, '\n'), 0644); err != nil {
				panic(err)
			}
		}
		return
	}
	if *mode == "linearpolicy" {
		if *rawRange == "" {
			panic("linearpolicy requires -raw-range")
		}
		specs := map[string]linearPolicySpec{
			"qkv": {"noise_refiner.0.attention.qkv.weight", 37386752, "attention_modulated.f32.bin", "qkv.f32.bin", 11520},
			"w1":  {"noise_refiner.0.feed_forward.w1.weight", 125875712, "ffn_modulated.f32.bin", "w1.f32.bin", 10240},
			"w3":  {"noise_refiner.0.feed_forward.w3.weight", 283162112, "ffn_modulated.f32.bin", "w3.f32.bin", 10240},
		}
		spec, ok := specs[*tensor]
		if !ok {
			panic(fmt.Errorf("unknown linearpolicy tensor %q", *tensor))
		}
		manifestBytes, err := os.ReadFile(filepath.Join(*cacheBlock, "manifest.json"))
		if err != nil {
			panic(err)
		}
		var manifest zimage.CacheManifest
		if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
			panic(err)
		}
		cacheName := ""
		for _, candidate := range manifest.Tensors {
			if candidate.SourceName == spec.SourceName {
				cacheName = candidate.DestinationName
				break
			}
		}
		if cacheName == "" {
			panic("linear weight absent from cache manifest")
		}
		input, err := readF32(filepath.Join(*stages, spec.InputStage), 3840)
		if err != nil {
			panic(err)
		}
		expected, err := readF32(filepath.Join(*stages, spec.OutputStage), 1)
		if err != nil {
			panic(err)
		}
		fp16Weight, err := readFP16Column(filepath.Join(*cacheBlock, cacheName), 3840, spec.Outputs, 0)
		if err != nil {
			panic(err)
		}
		bf16Weight, err := readBF16(*rawRange, spec.Offset, 3840)
		if err != nil {
			panic(err)
		}
		text := "LinearPolicyWitness {\n    Input: [" + render(input) + "]\n    BF16Weight: [" + render(bf16Weight) + "]\n    FP16Weight: [" + render(fp16Weight) + "]\n    Expected: " + render(expected) + "\n}\n"
		if err := os.WriteFile(*out, []byte(text), 0644); err != nil {
			panic(err)
		}
		if *analysisOut != "" {
			analysis := w2PolicyAnalysis{Schema: "oct.prometheus.evt2.o9.linear-bf16-fp16-policy.v1", Terms: 3840}
			var bf16Sum, fp16Sum float32
			for i := range input {
				if bf16Weight[i] == fp16Weight[i] {
					analysis.WeightExactCount++
				}
				delta := float32(math.Abs(float64(bf16Weight[i] - fp16Weight[i])))
				if delta > analysis.WeightMaxAbsDifference {
					analysis.WeightMaxAbsDifference = delta
				}
				bf16Sum += input[i] * bf16Weight[i]
				fp16Sum += input[i] * fp16Weight[i]
			}
			analysis.BF16ExpandedOutput, analysis.FP16ExpandedOutput, analysis.OutputDifference = bf16Sum, fp16Sum, bf16Sum-fp16Sum
			encoded, err := json.MarshalIndent(analysis, "", "  ")
			if err != nil {
				panic(err)
			}
			if err := os.WriteFile(*analysisOut, append(encoded, '\n'), 0644); err != nil {
				panic(err)
			}
		}
		return
	}
	if *mode != "qnorm" {
		panic(fmt.Errorf("unknown mode %q", *mode))
	}
	manifestBytes, err := os.ReadFile(filepath.Join(*cacheBlock, "manifest.json"))
	if err != nil {
		panic(err)
	}
	var manifest zimage.CacheManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		panic(err)
	}
	scaleName := ""
	for _, tensor := range manifest.Tensors {
		if tensor.SourceName == "noise_refiner.0.attention.q_norm.weight" {
			scaleName = tensor.DestinationName
			break
		}
	}
	if scaleName == "" {
		panic("q_norm weight absent from cache manifest")
	}
	input, err := readF32(filepath.Join(*stages, "q.f32.bin"), 128)
	if err != nil {
		panic(err)
	}
	expected, err := readF32(filepath.Join(*stages, "q_norm.f32.bin"), 128)
	if err != nil {
		panic(err)
	}
	scale, err := readFP16(filepath.Join(*cacheBlock, scaleName))
	if err != nil {
		panic(err)
	}
	if len(scale) != 128 {
		panic(fmt.Errorf("q_norm scale length %d, want 128", len(scale)))
	}
	text := "QNormRowWitness {\n    Input: [" + render(input) + "]\n    Scale: [" + render(scale) + "]\n    Expected: [" + render(expected) + "]\n}\n"
	if err := os.WriteFile(*out, []byte(text), 0644); err != nil {
		panic(err)
	}
}
