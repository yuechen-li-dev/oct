// zimage_dvt2_m6a_weight_audit characterizes the exact BF16-to-FP16
// conversion applied to the real W1/W3 tensors in all 30 Z-Image layers.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuechen-li-dev/oct/internal/prometheus/zimage"
)

const schema = "prometheus.dvt2.m6a.bf16-fp16-weight-audit.v1"

type tensorResult struct {
	Name                   string    `json:"name"`
	Layer                  uint32    `json:"layer"`
	Role                   string    `json:"role"`
	Shape                  []uint64  `json:"shape"`
	SourceByteRange        [2]uint64 `json:"source_byte_range"`
	ElementCount           uint64    `json:"element_count"`
	NonFiniteCount         uint64    `json:"non_finite_count"`
	FP16OverflowCount      uint64    `json:"fp16_overflow_count"`
	UnderflowToZeroCount   uint64    `json:"underflow_to_zero_count"`
	ExactConversionCount   uint64    `json:"exact_conversion_count"`
	MaximumAbsoluteError   float64   `json:"maximum_absolute_error"`
	MaximumRelativeError   float64   `json:"maximum_relative_error"`
	SourceFiniteMinimum    float64   `json:"source_finite_minimum"`
	SourceFiniteMaximum    float64   `json:"source_finite_maximum"`
	ConvertedFiniteMinimum float64   `json:"converted_finite_minimum"`
	ConvertedFiniteMaximum float64   `json:"converted_finite_maximum"`
	SourceSHA256           string    `json:"source_sha256"`
}

type totals struct {
	TensorCount          uint64  `json:"tensor_count"`
	ElementCount         uint64  `json:"element_count"`
	NonFiniteCount       uint64  `json:"non_finite_count"`
	FP16OverflowCount    uint64  `json:"fp16_overflow_count"`
	UnderflowToZeroCount uint64  `json:"underflow_to_zero_count"`
	ExactConversionCount uint64  `json:"exact_conversion_count"`
	MaximumAbsoluteError float64 `json:"maximum_absolute_error"`
	MaximumRelativeError float64 `json:"maximum_relative_error"`
}

type report struct {
	Schema                  string         `json:"schema"`
	GeneratedUTC            string         `json:"generated_utc"`
	CheckpointDisplayPath   string         `json:"checkpoint_display_path"`
	CheckpointSHA256        string         `json:"checkpoint_sha256"`
	Conversion              string         `json:"conversion"`
	ExactCountDefinition    string         `json:"exact_count_definition"`
	RelativeErrorDefinition string         `json:"relative_error_definition"`
	Totals                  totals         `json:"totals"`
	Tensors                 []tensorResult `json:"tensors"`
}

func bf16Float(bits uint16) float32 {
	return math.Float32frombits(uint32(bits) << 16)
}

func auditTensor(file *os.File, tensor zimage.Tensor, layer uint32, role string) (tensorResult, error) {
	result := tensorResult{
		Name: tensor.Name, Layer: layer, Role: role, Shape: append([]uint64(nil), tensor.Shape...),
		SourceByteRange: tensor.FileRange, ElementCount: tensor.Elements,
		SourceFiniteMinimum: math.Inf(1), SourceFiniteMaximum: math.Inf(-1),
		ConvertedFiniteMinimum: math.Inf(1), ConvertedFiniteMaximum: math.Inf(-1),
	}
	section := io.NewSectionReader(file, int64(tensor.FileRange[0]), int64(tensor.Bytes))
	reader := bufio.NewReaderSize(section, 8*1024*1024)
	hash := sha256.New()
	buffer := make([]byte, 8*1024*1024)
	var consumed uint64
	for {
		n, err := reader.Read(buffer)
		if n != 0 {
			if n%2 != 0 {
				return tensorResult{}, fmt.Errorf("%s returned odd BF16 byte count", tensor.Name)
			}
			_, _ = hash.Write(buffer[:n])
			for offset := 0; offset < n; offset += 2 {
				bits := binary.LittleEndian.Uint16(buffer[offset:])
				source := bf16Float(bits)
				convertedBits := zimage.BF16ToFP16(bits)
				converted := zimage.FP16ToFloat32(convertedBits)
				consumed++
				if math.IsNaN(float64(source)) || math.IsInf(float64(source), 0) {
					result.NonFiniteCount++
					continue
				}
				s := float64(source)
				c := float64(converted)
				result.SourceFiniteMinimum = math.Min(result.SourceFiniteMinimum, s)
				result.SourceFiniteMaximum = math.Max(result.SourceFiniteMaximum, s)
				if math.IsInf(c, 0) {
					result.FP16OverflowCount++
					continue
				}
				result.ConvertedFiniteMinimum = math.Min(result.ConvertedFiniteMinimum, c)
				result.ConvertedFiniteMaximum = math.Max(result.ConvertedFiniteMaximum, c)
				if source != 0 && converted == 0 {
					result.UnderflowToZeroCount++
				}
				if source == converted {
					result.ExactConversionCount++
				}
				absolute := math.Abs(c - s)
				result.MaximumAbsoluteError = math.Max(result.MaximumAbsoluteError, absolute)
				if source != 0 {
					result.MaximumRelativeError = math.Max(result.MaximumRelativeError, absolute/math.Abs(s))
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return tensorResult{}, fmt.Errorf("read %s: %w", tensor.Name, err)
		}
	}
	if consumed != tensor.Elements {
		return tensorResult{}, fmt.Errorf("%s: read %d elements, expected %d", tensor.Name, consumed, tensor.Elements)
	}
	result.SourceSHA256 = hex.EncodeToString(hash.Sum(nil))
	return result, nil
}

func main() {
	checkpoint := flag.String("checkpoint", "", "pinned Z-Image BF16 safetensors checkpoint")
	out := flag.String("out", "", "output JSON path")
	flag.Parse()
	if *checkpoint == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := zimage.VerifySHA256(*checkpoint, zimage.NoiseRefiner0SourceCheckpointSHA256); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	manifest, err := zimage.ReadManifest(*checkpoint, "local-model-cache/z_image_turbo_bf16.safetensors")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	file, err := os.Open(*checkpoint)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer file.Close()
	result := report{
		Schema: schema, GeneratedUTC: time.Now().UTC().Format(time.RFC3339),
		CheckpointDisplayPath:   "local-model-cache/z_image_turbo_bf16.safetensors",
		CheckpointSHA256:        zimage.NoiseRefiner0SourceCheckpointSHA256,
		Conversion:              "IEEE BF16 exact FP32 value to IEEE FP16 round-to-nearest-even",
		ExactCountDefinition:    "finite source values whose FP16 value widened to FP32 equals the BF16 value exactly",
		RelativeErrorDefinition: "abs(fp16_widened-source)/abs(source) for finite nonzero source values",
	}
	byName := make(map[string]zimage.Tensor, len(manifest.Tensors))
	for _, tensor := range manifest.Tensors {
		byName[tensor.Name] = tensor
	}
	for layer := uint32(0); layer < 30; layer++ {
		for _, role := range []string{"w1", "w3"} {
			name := fmt.Sprintf("layers.%d.feed_forward.%s.weight", layer, role)
			tensor, ok := byName[name]
			if !ok {
				fmt.Fprintf(os.Stderr, "required tensor %s is missing\n", name)
				os.Exit(1)
			}
			if tensor.DType != "BF16" || tensor.Elements != 10240*3840 || len(tensor.Shape) != 2 || tensor.Shape[0] != 10240 || tensor.Shape[1] != 3840 {
				fmt.Fprintf(os.Stderr, "%s has unexpected dtype/shape\n", name)
				os.Exit(1)
			}
			audited, auditErr := auditTensor(file, tensor, layer, strings.ToUpper(role))
			if auditErr != nil {
				fmt.Fprintln(os.Stderr, auditErr)
				os.Exit(1)
			}
			result.Tensors = append(result.Tensors, audited)
			result.Totals.TensorCount++
			result.Totals.ElementCount += audited.ElementCount
			result.Totals.NonFiniteCount += audited.NonFiniteCount
			result.Totals.FP16OverflowCount += audited.FP16OverflowCount
			result.Totals.UnderflowToZeroCount += audited.UnderflowToZeroCount
			result.Totals.ExactConversionCount += audited.ExactConversionCount
			result.Totals.MaximumAbsoluteError = math.Max(result.Totals.MaximumAbsoluteError, audited.MaximumAbsoluteError)
			result.Totals.MaximumRelativeError = math.Max(result.Totals.MaximumRelativeError, audited.MaximumRelativeError)
		}
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err = os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	temporary := *out + ".tmp"
	if err = os.WriteFile(temporary, append(encoded, '\n'), 0o644); err == nil {
		err = os.Rename(temporary, *out)
	}
	if err != nil {
		_ = os.Remove(temporary)
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%s tensors=%d elements=%d exact=%d overflow=%d underflow=%d nonfinite=%d\n", schema,
		result.Totals.TensorCount, result.Totals.ElementCount, result.Totals.ExactConversionCount,
		result.Totals.FP16OverflowCount, result.Totals.UnderflowToZeroCount, result.Totals.NonFiniteCount)
}
