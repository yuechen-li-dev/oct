// Command evt2_oracle_compare records numerical comparisons for the bounded
// EVT-2 diagnostic replay. It reads fixed binary payloads only; it owns no
// model semantics and does not invoke a production shader.
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
)

type scan struct {
	Count          uint64  `json:"count"`
	Finite         bool    `json:"finite"`
	FirstNonFinite *uint64 `json:"first_non_finite,omitempty"`
	Min            float64 `json:"min,omitempty"`
	Max            float64 `json:"max,omitempty"`
	AbsMax         float64 `json:"abs_max,omitempty"`
	SHA256         string  `json:"sha256"`
}

type comparison struct {
	Count      uint64  `json:"count"`
	Finite     bool    `json:"finite"`
	LInf       float64 `json:"linf"`
	RelativeL2 float64 `json:"relative_l2"`
	MaxIndex   uint64  `json:"max_error_flat_index"`
}

type report struct {
	Schema           string      `json:"schema"`
	Policy           string      `json:"policy"`
	Final            scan        `json:"final_output_f32"`
	W2               scan        `json:"w2_f32"`
	Reference        scan        `json:"reference_output_bf16"`
	FinalVsRef       comparison  `json:"final_vs_reference_bf16"`
	Conclusion       string      `json:"conclusion"`
	Candidate        *scan       `json:"candidate_final_output_f32,omitempty"`
	FinalVsCandidate *comparison `json:"final_vs_candidate,omitempty"`
}

func compareF32(actualPath, expectedPath string) (comparison, error) {
	a, err := os.ReadFile(actualPath)
	if err != nil {
		return comparison{}, err
	}
	b, err := os.ReadFile(expectedPath)
	if err != nil {
		return comparison{}, err
	}
	if len(a) != len(b) || len(a)%4 != 0 {
		return comparison{}, fmt.Errorf("f32 element count mismatch")
	}
	r := comparison{Count: uint64(len(a) / 4), Finite: true}
	var es, rs float64
	for i := 0; i < len(a); i += 4 {
		x := float64(math.Float32frombits(binary.LittleEndian.Uint32(a[i:])))
		y := float64(math.Float32frombits(binary.LittleEndian.Uint32(b[i:])))
		if math.IsNaN(x) || math.IsInf(x, 0) {
			r.Finite = false
			continue
		}
		d := math.Abs(x - y)
		if d > r.LInf {
			r.LInf = d
			r.MaxIndex = uint64(i / 4)
		}
		es += d * d
		rs += y * y
	}
	if rs > 0 {
		r.RelativeL2 = math.Sqrt(es / rs)
	}
	return r, nil
}

func hashFile(path string) (string, error) {
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

func scanF32(path string) (scan, error) {
	f, err := os.Open(path)
	if err != nil {
		return scan{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size()%4 != 0 {
		return scan{}, fmt.Errorf("invalid f32 payload %s", path)
	}
	result := scan{Count: uint64(info.Size() / 4), Finite: true}
	result.SHA256, err = hashFile(path)
	if err != nil {
		return scan{}, err
	}
	buf := make([]byte, 1<<20)
	var index uint64
	first := true
	for {
		n, readErr := f.Read(buf)
		if n%4 != 0 {
			return scan{}, fmt.Errorf("unaligned read from %s", path)
		}
		for i := 0; i < n; i += 4 {
			value := float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[i:])))
			if math.IsNaN(value) || math.IsInf(value, 0) {
				if result.Finite {
					at := index
					result.FirstNonFinite = &at
				}
				result.Finite = false
			} else {
				if first {
					result.Min, result.Max, first = value, value, false
				}
				if value < result.Min {
					result.Min = value
				}
				if value > result.Max {
					result.Max = value
				}
				if math.Abs(value) > result.AbsMax {
					result.AbsMax = math.Abs(value)
				}
			}
			index++
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return scan{}, readErr
		}
	}
	return result, nil
}

func scanBF16(path string) (scan, error) {
	f, err := os.Open(path)
	if err != nil {
		return scan{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size()%2 != 0 {
		return scan{}, fmt.Errorf("invalid bf16 payload %s", path)
	}
	result := scan{Count: uint64(info.Size() / 2), Finite: true}
	result.SHA256, err = hashFile(path)
	if err != nil {
		return scan{}, err
	}
	buf := make([]byte, 1<<20)
	var index uint64
	first := true
	for {
		n, readErr := f.Read(buf)
		if n%2 != 0 {
			return scan{}, fmt.Errorf("unaligned read from %s", path)
		}
		for i := 0; i < n; i += 2 {
			value := float64(math.Float32frombits(uint32(binary.LittleEndian.Uint16(buf[i:])) << 16))
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return scan{}, fmt.Errorf("reference BF16 is non-finite at %d", index)
			}
			if first {
				result.Min, result.Max, first = value, value, false
			}
			if value < result.Min {
				result.Min = value
			}
			if value > result.Max {
				result.Max = value
			}
			if math.Abs(value) > result.AbsMax {
				result.AbsMax = math.Abs(value)
			}
			index++
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return scan{}, readErr
		}
	}
	return result, nil
}

func compare(finalPath, referencePath string) (comparison, error) {
	final, err := os.Open(finalPath)
	if err != nil {
		return comparison{}, err
	}
	defer final.Close()
	ref, err := os.Open(referencePath)
	if err != nil {
		return comparison{}, err
	}
	defer ref.Close()
	finalInfo, err := final.Stat()
	if err != nil {
		return comparison{}, err
	}
	refInfo, err := ref.Stat()
	if err != nil || finalInfo.Size()/4 != refInfo.Size()/2 {
		return comparison{}, fmt.Errorf("final/reference element count mismatch")
	}
	result := comparison{Count: uint64(finalInfo.Size() / 4), Finite: true}
	finalBuf, refBuf := make([]byte, 1<<20), make([]byte, 1<<20)
	var errorSquares, refSquares float64
	var index uint64
	for index < result.Count {
		chunk := int64(len(finalBuf) / 4)
		if int64(result.Count-index) < chunk {
			chunk = int64(result.Count - index)
		}
		if _, err = io.ReadFull(final, finalBuf[:chunk*4]); err != nil {
			return comparison{}, err
		}
		if _, err = io.ReadFull(ref, refBuf[:chunk*2]); err != nil {
			return comparison{}, err
		}
		for i := int64(0); i < chunk; i++ {
			actual := float64(math.Float32frombits(binary.LittleEndian.Uint32(finalBuf[i*4:])))
			expected := float64(math.Float32frombits(uint32(binary.LittleEndian.Uint16(refBuf[i*2:])) << 16))
			if math.IsNaN(actual) || math.IsInf(actual, 0) {
				result.Finite = false
				continue
			}
			delta := math.Abs(actual - expected)
			if delta > result.LInf {
				result.LInf, result.MaxIndex = delta, index+uint64(i)
			}
			errorSquares += delta * delta
			refSquares += expected * expected
		}
		index += uint64(chunk)
	}
	if refSquares != 0 {
		result.RelativeL2 = math.Sqrt(errorSquares / refSquares)
	}
	return result, nil
}

func main() {
	finalPath := flag.String("final", "", "corrected diagnostic float32 final output")
	w2Path := flag.String("w2", "", "corrected diagnostic float32 W2 output")
	referencePath := flag.String("reference", "", "captured BF16 block output")
	candidatePath := flag.String("candidate", "", "optional candidate float32 final output")
	outPath := flag.String("out", "", "report JSON path")
	flag.Parse()
	if *finalPath == "" || *w2Path == "" || *referencePath == "" || *outPath == "" {
		flag.Usage()
		os.Exit(2)
	}
	final, err := scanF32(*finalPath)
	if err != nil {
		panic(err)
	}
	w2, err := scanF32(*w2Path)
	if err != nil {
		panic(err)
	}
	reference, err := scanBF16(*referencePath)
	if err != nil {
		panic(err)
	}
	diff, err := compare(*finalPath, *referencePath)
	if err != nil {
		panic(err)
	}
	result := report{Schema: "oct.prometheus.evt2.o2.corrected-diagnostic-replay.v1", Policy: "FP16 cache values expanded to FP32; fixed-order FP32 reductions; corrected IEEE FP16 decode", Final: final, W2: w2, Reference: reference, FinalVsRef: diff, Conclusion: "Corrected diagnostic evidence only; Comfy BF16 output remains a compatibility boundary, not canonical authority."}
	if *candidatePath != "" {
		candidate, e := scanF32(*candidatePath)
		if e != nil {
			panic(e)
		}
		candidateDiff, e := compareF32(*candidatePath, *finalPath)
		if e != nil {
			panic(e)
		}
		result.Candidate = &candidate
		result.FinalVsCandidate = &candidateDiff
		result.Conclusion = "Controlled diagnostic candidate comparison; not canonical authority."
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(*outPath, append(encoded, '\n'), 0644); err != nil {
		panic(err)
	}
}
