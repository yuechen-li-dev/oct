package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/yuechen-li-dev/oct/internal/prometheus/zimage"
)

func hash(data []byte) string { value := sha256.Sum256(data); return hex.EncodeToString(value[:]) }
func encode(values []float32) []byte {
	data := make([]byte, len(values)*4)
	for i, v := range values {
		binary.LittleEndian.PutUint32(data[i*4:], math.Float32bits(v))
	}
	return data
}

func stageShape(name string, count int) []int {
	if name == "timestep_input" {
		return []int{1, 256}
	}
	if name == "timestep_linear" {
		return []int{1, 15360}
	}
	if len(name) >= 12 && (name[:12] == "attention_lo" || name[:12] == "attention_pr") {
		return []int{1024}
	}
	if name == "w1" || name == "w3" || name == "ffn_gated_hidden" {
		return []int{1, 1024, 10240}
	}
	if name == "qkv" {
		return []int{1, 1024, 11520}
	}
	if name == "q" || name == "k" || name == "v" || name == "q_norm" || name == "k_norm" || name == "q_rope" || name == "k_rope" {
		return []int{1, 1024, 30, 128}
	}
	if count == 3840 {
		return []int{1, 3840}
	}
	return []int{1, 1024, 3840}
}

func projection(values []float32) map[string]any {
	min, max := values[0], values[0]
	var sum, squares float64
	finite := true
	for _, v := range values {
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			finite = false
			continue
		}
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += float64(v)
		squares += float64(v) * float64(v)
	}
	if !finite {
		return map[string]any{"finite": false, "min": nil, "max": nil, "mean": nil, "rms": nil, "l2": nil, "first_element": fmt.Sprint(values[0]), "last_element": fmt.Sprint(values[len(values)-1]), "selected_coordinates": "contains non-finite values; full payload is retained for diagnosis"}
	}
	count := float64(len(values))
	indices := []int{0, 1, len(values) / 3, 2 * len(values) / 3, len(values) - 2, len(values) - 1}
	selected := make([]map[string]any, 0, len(indices))
	for _, i := range indices {
		selected = append(selected, map[string]any{"flat_index": i, "value": values[i]})
	}
	return map[string]any{"finite": true, "min": min, "max": max, "mean": sum / count, "rms": math.Sqrt(squares / count), "l2": math.Sqrt(squares), "first_element": values[0], "last_element": values[len(values)-1], "selected_coordinates": selected}
}

func main() {
	cacheRoot := flag.String("cache-root", "", "EVT-2 root containing layers")
	oracleRoot := flag.String("oracle-root", "", "revision directory containing run_02 and m075")
	out := flag.String("out", "", "local canonical bundle output directory")
	capture := flag.Bool("capture", false, "write full local stage payloads")
	flag.Parse()
	if *cacheRoot == "" || *oracleRoot == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	result, err := zimage.RunCanonicalNoiseRefiner0(zimage.CanonicalNoiseRefiner0Paths{CacheRoot: *cacheRoot, OracleRoot: *oracleRoot}, *capture)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err = os.MkdirAll(*out, 0755); err != nil {
		panic(err)
	}
	final := encode(result.Final)
	if err = os.WriteFile(filepath.Join(*out, "final_output.f32.bin"), final, 0644); err != nil {
		panic(err)
	}
	manifest := map[string]any{"schema": "oct.prometheus.evt2m1b0r.canonical-local-bundle.v1", "source_revision": zimage.CanonicalNoiseRefiner0SourceRevision, "rope_frame": 33, "final_output": map[string]any{"relative_path": "final_output.f32.bin", "sha256": hash(final), "shape": []int{1, 1024, 3840}, "dtype": "float32"}, "capture": *capture}
	if *capture {
		names := make([]string, 0, len(result.Stages))
		for name := range result.Stages {
			names = append(names, name)
		}
		sort.Strings(names)
		stages := map[string]any{}
		for _, name := range names {
			data := encode(result.Stages[name])
			file := "stages/" + name + ".f32.bin"
			if err = os.MkdirAll(filepath.Join(*out, "stages"), 0755); err != nil {
				panic(err)
			}
			if err = os.WriteFile(filepath.Join(*out, file), data, 0644); err != nil {
				panic(err)
			}
			metadata := map[string]any{"relative_path": file, "sha256": hash(data), "bytes": len(data), "shape": stageShape(name, len(result.Stages[name])), "dtype": "float32", "logical_layout": "row-major contiguous", "physical_strides": "contiguous"}
			metadata["projection"] = projection(result.Stages[name])
			projectionData, _ := json.Marshal(metadata["projection"])
			metadata["projection_sha256"] = hash(projectionData)
			stages[name] = metadata
		}
		manifest["stages"] = stages
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = os.WriteFile(filepath.Join(*out, "manifest.json"), append(data, '\n'), 0644); err != nil {
		panic(err)
	}
	fmt.Println(manifest["final_output"])
}
