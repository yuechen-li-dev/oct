// Command evt2_adverse_census scans the pinned source/cache and corrected
// diagnostic stages for adverse numerical witnesses. It is laboratory
// equipment, not a production tensor runtime.
package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/yuechen-li-dev/oct/internal/prometheus/zimage"
)

const sourceRangeStart uint64 = 11584667040

type forensicTensor struct {
	Name      string   `json:"name"`
	Shape     []int    `json:"shape"`
	FileRange []uint64 `json:"file_byte_range"`
}
type forensics struct {
	Tensors []forensicTensor `json:"tensors"`
}
type rowFinding struct {
	Row   int     `json:"row"`
	Value float64 `json:"value"`
}
type tensorFinding struct {
	Name                     string     `json:"name"`
	Shape                    []int      `json:"shape"`
	MaxAbsRow                rowFinding `json:"max_abs_row"`
	MaxErrorRow              rowFinding `json:"max_conversion_error_row"`
	MaxRelativeDriftRow      rowFinding `json:"max_relative_drift_row"`
	StrongestCancellationRow rowFinding `json:"strongest_cancellation_row"`
	ExactCount               uint64     `json:"exact_count"`
	Count                    uint64     `json:"count"`
	MaxError                 float64    `json:"max_conversion_error"`
}
type stageFinding struct {
	Name     string  `json:"name"`
	Count    int     `json:"count"`
	Finite   bool    `json:"finite"`
	Min      float32 `json:"min"`
	Max      float32 `json:"max"`
	AbsMax   float32 `json:"abs_max"`
	MinIndex int     `json:"min_index"`
	MaxIndex int     `json:"max_index"`
}
type report struct {
	Schema               string          `json:"schema"`
	WeightRows           []tensorFinding `json:"weight_rows"`
	Stages               []stageFinding  `json:"stages"`
	PrecisionConclusions []string        `json:"precision_conclusions"`
}

func f16(b []byte, index int) float32 {
	return zimage.FP16ToFloat32(binary.LittleEndian.Uint16(b[index*2:]))
}
func bf16(b []byte, index int) float32 {
	return math.Float32frombits(uint32(binary.LittleEndian.Uint16(b[index*2:])) << 16)
}
func f32(b []byte, index int) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(b[index*4:]))
}

func main() {
	rawPath := flag.String("raw-range", "", "official contiguous BF16 range")
	forensicsPath := flag.String("forensics", "", "forensics JSON")
	cacheBlock := flag.String("cache-block", "", "validated cache block")
	stages := flag.String("stages", "", "corrected capture stages directory")
	out := flag.String("out", "", "adverse census JSON")
	flag.Parse()
	if *rawPath == "" || *forensicsPath == "" || *cacheBlock == "" || *stages == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	var source forensics
	data, err := os.ReadFile(*forensicsPath)
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(data, &source); err != nil {
		panic(err)
	}
	var manifest zimage.CacheManifest
	data, err = os.ReadFile(filepath.Join(*cacheBlock, "manifest.json"))
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(data, &manifest); err != nil {
		panic(err)
	}
	byName := map[string]zimage.CacheTensor{}
	for _, t := range manifest.Tensors {
		byName[t.SourceName] = t
	}
	raw, err := os.Open(*rawPath)
	if err != nil {
		panic(err)
	}
	defer raw.Close()
	findings := []tensorFinding{}
	for _, src := range source.Tensors {
		if len(src.Name) < 16 || src.Name[:16] != "noise_refiner.0." {
			continue
		}
		cache, ok := byName[src.Name]
		if !ok {
			continue
		}
		if len(src.Shape) == 0 || len(src.FileRange) != 2 {
			panic("bad source tensor")
		}
		sourceBytes := make([]byte, src.FileRange[1]-src.FileRange[0])
		if _, err := raw.ReadAt(sourceBytes, int64(src.FileRange[0]-sourceRangeStart)); err != nil {
			panic(err)
		}
		cacheBytes, err := os.ReadFile(filepath.Join(*cacheBlock, cache.DestinationName))
		if err != nil {
			panic(err)
		}
		rows := src.Shape[0]
		cols := 1
		for _, d := range src.Shape[1:] {
			cols *= d
		}
		if len(src.Shape) == 1 {
			rows = 1
			cols = src.Shape[0]
		}
		result := tensorFinding{Name: src.Name, Shape: src.Shape, MaxAbsRow: rowFinding{Value: -1}, MaxErrorRow: rowFinding{Value: -1}, MaxRelativeDriftRow: rowFinding{Value: -1}, StrongestCancellationRow: rowFinding{Value: math.Inf(1)}}
		for row := 0; row < rows; row++ {
			var maxAbs, maxErr, sourceSq, errorSq, pos, neg float64
			for col := 0; col < cols; col++ {
				si := row*cols + col
				ci := si
				if cache.Transpose && len(src.Shape) == 2 {
					ci = col*rows + row
				}
				a := float64(bf16(sourceBytes, si))
				b := float64(f16(cacheBytes, ci))
				e := math.Abs(a - b)
				if math.Abs(a) > maxAbs {
					maxAbs = math.Abs(a)
				}
				if e > maxErr {
					maxErr = e
				}
				sourceSq += a * a
				errorSq += e * e
				if a >= 0 {
					pos += a
				} else {
					neg -= a
				}
				result.Count++
				if a == b {
					result.ExactCount++
				}
				if e > result.MaxError {
					result.MaxError = e
				}
			}
			rel := 0.0
			if sourceSq > 0 {
				rel = math.Sqrt(errorSq / sourceSq)
			}
			cancellation := math.Abs(pos-neg) / (pos + neg + 1e-30)
			if maxAbs > result.MaxAbsRow.Value {
				result.MaxAbsRow = rowFinding{row, maxAbs}
			}
			if maxErr > result.MaxErrorRow.Value {
				result.MaxErrorRow = rowFinding{row, maxErr}
			}
			if rel > result.MaxRelativeDriftRow.Value {
				result.MaxRelativeDriftRow = rowFinding{row, rel}
			}
			if cancellation < result.StrongestCancellationRow.Value {
				result.StrongestCancellationRow = rowFinding{row, cancellation}
			}
		}
		findings = append(findings, result)
	}
	entries, err := os.ReadDir(*stages)
	if err != nil {
		panic(err)
	}
	stageFindings := []stageFinding{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".bin" {
			continue
		}
		bytes, err := os.ReadFile(filepath.Join(*stages, entry.Name()))
		if err != nil {
			panic(err)
		}
		if len(bytes)%4 != 0 {
			continue
		}
		r := stageFinding{Name: entry.Name(), Count: len(bytes) / 4, Finite: true}
		for i := 0; i < r.Count; i++ {
			v := f32(bytes, i)
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				r.Finite = false
			}
			if i == 0 || v < r.Min {
				r.Min = v
				r.MinIndex = i
			}
			if i == 0 || v > r.Max {
				r.Max = v
				r.MaxIndex = i
			}
			if i == 0 || float32(math.Abs(float64(v))) > r.AbsMax {
				r.AbsMax = float32(math.Abs(float64(v)))
			}
		}
		stageFindings = append(stageFindings, r)
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Name < findings[j].Name })
	sort.Slice(stageFindings, func(i, j int) bool { return stageFindings[i].Name < stageFindings[j].Name })
	r := report{Schema: "oct.prometheus.evt2.o10.adverse-census.v1", WeightRows: findings, Stages: stageFindings, PrecisionConclusions: []string{"All weights require source/cache comparison separate from activation range.", "W2 output exceeds finite FP16 range and must remain FP32 through ffn_norm2 and final gate/residual arithmetic.", "FP16-weight storage does not authorize FP16 activation storage."}}
	encoded, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(*out, append(encoded, '\n'), 0644); err != nil {
		panic(err)
	}
	fmt.Printf("weights=%d stages=%d\n", len(findings), len(stageFindings))
}
