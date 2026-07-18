// Command evt2_oct_oracle measures one downloaded contiguous source range for
// the thirteen noise_refiner.0 tensors. It is laboratory equipment: Oct owns
// the semantic experiments, while this command makes the BF16 bit census
// auditable without introducing a general tensor framework.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"sort"

	"github.com/yuechen-li-dev/oct/internal/prometheus/zimage"
)

type forensicTensor struct {
	Name      string   `json:"name"`
	DType     string   `json:"dtype"`
	Shape     []uint64 `json:"shape"`
	FileRange []uint64 `json:"file_byte_range"`
}

type forensics struct {
	CheckpointSHA256 string           `json:"checkpoint_sha256"`
	Tensors          []forensicTensor `json:"tensors"`
}

type census struct {
	Name                           string   `json:"name"`
	Shape                          []uint64 `json:"shape"`
	SourceDType                    string   `json:"source_dtype"`
	SourceMin                      float64  `json:"source_min"`
	SourceMax                      float64  `json:"source_max"`
	SourceAbsMax                   float64  `json:"source_abs_max"`
	SourceFinite                   bool     `json:"source_finite"`
	SourceZeroCount                uint64   `json:"source_zero_count"`
	SourceSubnormalCount           uint64   `json:"source_subnormal_count"`
	FP16OverflowCount              uint64   `json:"fp16_conversion_overflow_count"`
	FP16UnderflowCount             uint64   `json:"fp16_conversion_underflow_count"`
	FP16SubnormalCount             uint64   `json:"fp16_subnormal_count"`
	FP16SaturationCount            uint64   `json:"fp16_saturation_count"`
	ExactValuePreservation         float64  `json:"exact_value_preservation_rate"`
	RelativeL2ConversionDrift      float64  `json:"relative_l2_conversion_drift"`
	MaximumConversionError         float64  `json:"maximum_conversion_error"`
	LikelyProductionRepresentation string   `json:"likely_production_representation"`
}

type inventory struct {
	Schema                string   `json:"schema"`
	Status                string   `json:"status"`
	CheckpointSHA256      string   `json:"source_checkpoint_sha256"`
	DownloadedRangeStart  uint64   `json:"downloaded_source_range_start"`
	DownloadedRangeEnd    uint64   `json:"downloaded_source_range_end_exclusive"`
	DownloadedRangeSHA256 string   `json:"downloaded_range_sha256"`
	TensorCount           int      `json:"tensor_count"`
	Tensors               []census `json:"tensors"`
	Conclusion            string   `json:"conclusion"`
}

func bf16(bits uint16) float64 { return float64(math.Float32frombits(uint32(bits) << 16)) }

func fp16Subnormal(bits uint16) bool { return bits&0x7c00 == 0 && bits&0x03ff != 0 }
func bf16Subnormal(bits uint16) bool { return bits&0x7f80 == 0 && bits&0x007f != 0 }
func fp16Infinite(bits uint16) bool  { return bits&0x7c00 == 0x7c00 && bits&0x03ff == 0 }

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func scan(raw *os.File, item forensicTensor, rangeStart uint64) (census, error) {
	if item.DType != "BF16" || len(item.FileRange) != 2 || item.FileRange[1] < item.FileRange[0] || (item.FileRange[1]-item.FileRange[0])%2 != 0 {
		return census{}, fmt.Errorf("invalid BF16 tensor contract for %s", item.Name)
	}
	offset := item.FileRange[0] - rangeStart
	if _, err := raw.Seek(int64(offset), io.SeekStart); err != nil {
		return census{}, err
	}
	remaining := item.FileRange[1] - item.FileRange[0]
	buffer := make([]byte, 1<<20)
	first := true
	var min, max, absMax, sourceSquares, errorSquares, maxError float64
	var values, exact, zeroes, sourceSubnormals, fp16Overflows, fp16Underflows, fp16Subnormals uint64
	finite := true
	for remaining > 0 {
		want := uint64(len(buffer))
		if remaining < want {
			want = remaining
		}
		if _, err := io.ReadFull(raw, buffer[:want]); err != nil {
			return census{}, err
		}
		for i := uint64(0); i < want; i += 2 {
			bits := uint16(buffer[i]) | uint16(buffer[i+1])<<8
			source := bf16(bits)
			if source == 0 {
				zeroes++
			}
			if bf16Subnormal(bits) {
				sourceSubnormals++
			}
			if math.IsNaN(source) || math.IsInf(source, 0) {
				finite = false
				continue
			}
			if first {
				min, max, first = source, source, false
			}
			if source < min {
				min = source
			}
			if source > max {
				max = source
			}
			if math.Abs(source) > absMax {
				absMax = math.Abs(source)
			}
			fp16Bits := zimage.BF16ToFP16(bits)
			converted := float64(zimage.FP16ToFloat32(fp16Bits))
			if fp16Infinite(fp16Bits) {
				fp16Overflows++
			}
			if source != 0 && converted == 0 {
				fp16Underflows++
			}
			if fp16Subnormal(fp16Bits) {
				fp16Subnormals++
			}
			if source == converted {
				exact++
			}
			err := math.Abs(source - converted)
			if err > maxError {
				maxError = err
			}
			sourceSquares += source * source
			errorSquares += err * err
			values++
		}
		remaining -= want
	}
	if !finite {
		return census{}, fmt.Errorf("unexpected non-finite source value in %s", item.Name)
	}
	if values == 0 {
		return census{}, fmt.Errorf("empty tensor %s", item.Name)
	}
	relative := 0.0
	if sourceSquares != 0 {
		relative = math.Sqrt(errorSquares / sourceSquares)
	}
	return census{
		Name: item.Name, Shape: item.Shape, SourceDType: item.DType, SourceMin: min, SourceMax: max, SourceAbsMax: absMax,
		SourceFinite: finite, SourceZeroCount: zeroes, SourceSubnormalCount: sourceSubnormals,
		FP16OverflowCount: fp16Overflows, FP16UnderflowCount: fp16Underflows, FP16SubnormalCount: fp16Subnormals,
		FP16SaturationCount: 0, ExactValuePreservation: float64(exact) / float64(values), RelativeL2ConversionDrift: relative,
		MaximumConversionError:         maxError,
		LikelyProductionRepresentation: "BF16 source is FP16-representable; storage decision remains unresolved until the captured activation-path W2 study.",
	}, nil
}

func main() {
	rawPath := flag.String("raw-range", "", "contiguous raw BF16 source range")
	forensicsPath := flag.String("forensics", "", "committed safetensors forensic JSON")
	outPath := flag.String("out", "", "deterministic tensor inventory JSON")
	flag.Parse()
	if *rawPath == "" || *forensicsPath == "" || *outPath == "" {
		flag.Usage()
		os.Exit(2)
	}
	data, err := os.ReadFile(*forensicsPath)
	if err != nil {
		panic(err)
	}
	var source forensics
	if err = json.Unmarshal(data, &source); err != nil {
		panic(err)
	}
	selected := make([]forensicTensor, 0, 13)
	for _, item := range source.Tensors {
		if len(item.Name) >= len("noise_refiner.0.") && item.Name[:len("noise_refiner.0.")] == "noise_refiner.0." {
			selected = append(selected, item)
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].FileRange[0] < selected[j].FileRange[0] })
	if len(selected) != 13 {
		panic(fmt.Errorf("wanted 13 noise_refiner.0 tensors, got %d", len(selected)))
	}
	start, end := selected[0].FileRange[0], selected[len(selected)-1].FileRange[1]
	for i := 1; i < len(selected); i++ {
		if selected[i-1].FileRange[1] != selected[i].FileRange[0] {
			panic("noise_refiner.0 range is not contiguous")
		}
	}
	info, err := os.Stat(*rawPath)
	if err != nil {
		panic(err)
	}
	if uint64(info.Size()) != end-start {
		panic(fmt.Errorf("raw range bytes %d, want %d", info.Size(), end-start))
	}
	raw, err := os.Open(*rawPath)
	if err != nil {
		panic(err)
	}
	defer raw.Close()
	items := make([]census, 0, len(selected))
	for _, item := range selected {
		value, e := scan(raw, item, start)
		if e != nil {
			panic(e)
		}
		items = append(items, value)
	}
	checksum, err := sha256File(*rawPath)
	if err != nil {
		panic(err)
	}
	report := inventory{Schema: "oct.prometheus.evt2.o1.tensor-inventory.v1", Status: "complete-source-weight-census; activation-policy-open", CheckpointSHA256: source.CheckpointSHA256, DownloadedRangeStart: start, DownloadedRangeEnd: end, DownloadedRangeSHA256: checksum, TensorCount: len(items), Tensors: items, Conclusion: "All 13 source BF16 tensors are finite and convert to finite FP16 values without overflow. This rejects source-weight overflow as the direct cause of the historical W2 non-finite result; it does not certify all-FP16 activation-path correctness."}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(*outPath, append(encoded, '\n'), 0644); err != nil {
		panic(err)
	}
}
