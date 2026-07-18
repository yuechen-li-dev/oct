// Command evt2_activation_cast measures an IEEE FP32 -> FP16 -> FP32
// diagnostic round trip for captured stages. It does not alter production data.
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

type stage struct {
	Name          string  `json:"name"`
	Count         int     `json:"count"`
	Overflow      int     `json:"fp16_overflow_count"`
	LInf          float64 `json:"linf"`
	RelativeL2    float64 `json:"relative_l2"`
	MaxErrorIndex int     `json:"max_error_index"`
	Candidate     string  `json:"candidate"`
}
type report struct {
	Schema     string  `json:"schema"`
	Stages     []stage `json:"stages"`
	Conclusion string  `json:"conclusion"`
}

func f32ToF16(value float32) uint16 {
	b := math.Float32bits(value)
	sign := uint16((b >> 16) & 0x8000)
	exp := int((b >> 23) & 0xff)
	mant := b & 0x7fffff
	if exp == 0xff {
		if mant == 0 {
			return sign | 0x7c00
		}
		return sign | 0x7e00
	}
	e := exp - 127 + 15
	if e >= 31 {
		return sign | 0x7c00
	}
	if e <= 0 {
		if e < -10 {
			return sign
		}
		mant |= 0x800000
		shift := uint(14 - e)
		out := uint16(mant >> shift)
		rem := mant & ((uint32(1) << shift) - 1)
		half := uint32(1) << (shift - 1)
		if rem > half || (rem == half && out&1 != 0) {
			out++
		}
		return sign | out
	}
	out := sign | uint16(e<<10) | uint16(mant>>13)
	rem := mant & 0x1fff
	if rem > 0x1000 || (rem == 0x1000 && out&1 != 0) {
		out++
	}
	return out
}

func main() {
	stagesPath := flag.String("stages", "", "corrected stage directory")
	out := flag.String("out", "", "JSON output")
	flag.Parse()
	if *stagesPath == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	entries, err := os.ReadDir(*stagesPath)
	if err != nil {
		panic(err)
	}
	rows := []stage{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".bin" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(*stagesPath, entry.Name()))
		if err != nil {
			panic(err)
		}
		if len(b)%4 != 0 {
			continue
		}
		r := stage{Name: entry.Name(), Count: len(b) / 4, MaxErrorIndex: -1}
		var errorSq, valueSq float64
		for i := 0; i < r.Count; i++ {
			v := math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
			h := f32ToF16(v)
			q := zimage.FP16ToFloat32(h)
			if h&0x7c00 == 0x7c00 && h&0x03ff == 0 && !math.IsInf(float64(v), 0) {
				r.Overflow++
			}
			e := math.Abs(float64(v - q))
			if e > r.LInf {
				r.LInf = e
				r.MaxErrorIndex = i
			}
			errorSq += e * e
			valueSq += float64(v) * float64(v)
		}
		if valueSq > 0 {
			r.RelativeL2 = math.Sqrt(errorSq / valueSq)
		}
		if r.Overflow > 0 {
			r.Candidate = "prohibited: overflow"
		} else {
			r.Candidate = "range-only candidate; downstream error replay required"
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	encoded, err := json.MarshalIndent(report{Schema: "oct.prometheus.evt2.o13.activation-fp16-roundtrip.v1", Stages: rows, Conclusion: "Round-trip range/error evidence only; a candidate is not an accepted activation-storage policy until downstream replay."}, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(*out, append(encoded, '\n'), 0644); err != nil {
		panic(err)
	}
	fmt.Printf("stages=%d\n", len(rows))
}
