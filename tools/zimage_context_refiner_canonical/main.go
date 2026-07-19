// zimage_context_refiner_canonical writes one deterministic local ContextRefiner
// laboratory bundle. The block-1 input is an explicit FP32 resident boundary.
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

func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func encode(values []float32) []byte {
	data := make([]byte, len(values)*4)
	for i, value := range values {
		binary.LittleEndian.PutUint32(data[i*4:], math.Float32bits(value))
	}
	return data
}
func decode(path string) ([]float32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data)%4 != 0 {
		return nil, fmt.Errorf("%s is not FP32", path)
	}
	values := make([]float32, len(data)/4)
	for i := range values {
		values[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return values, nil
}

func write(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func main() {
	cache := flag.String("cache-root", "", "EVT-2 cache root")
	oracle := flag.String("oracle-root", "", "pinned EVT-2 oracle revision root")
	checkpoint := flag.String("checkpoint", "", "pinned Z-Image Turbo safetensors checkpoint")
	block := flag.String("block", "", "context_refiner.0 or context_refiner.1")
	input := flag.String("input-f32", "", "required resident FP32 boundary for context_refiner.1")
	out := flag.String("out", "", "local canonical bundle output directory")
	flag.Parse()
	if *cache == "" || *oracle == "" || *checkpoint == "" || *block == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	var boundary []float32
	var err error
	if *input != "" {
		boundary, err = decode(*input)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	result, err := zimage.RunCanonicalContextRefiner(*cache, *oracle, *checkpoint, *block, boundary, true)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	inputData, finalData := encode(result.Input), encode(result.Final)
	if err = write(filepath.Join(*out, "input.f32.bin"), inputData); err != nil {
		panic(err)
	}
	if err = write(filepath.Join(*out, "final_output.f32.bin"), finalData); err != nil {
		panic(err)
	}
	names := make([]string, 0, len(result.Stages))
	for name := range result.Stages {
		names = append(names, name)
	}
	sort.Strings(names)
	stages := map[string]any{}
	for _, name := range names {
		data := encode(result.Stages[name])
		relative := "stages/" + name + ".f32.bin"
		if err = write(filepath.Join(*out, filepath.FromSlash(relative)), data); err != nil {
			panic(err)
		}
		stages[name] = map[string]any{"relative_path": relative, "sha256": digest(data), "bytes": len(data), "dtype": "float32", "logical_layout": "row-major contiguous"}
	}
	manifest := map[string]any{
		"schema": "oct.prometheus.evt2.m2b.context-refiner-canonical.v1", "block": *block,
		"source_revision": "26f23eda626ffadda020b04ff79488e1d72004cd", "source_transformer_blob": "sha1:866a00ba95c7df49bbfaf9d2f7bc3fa3bfa9d431",
		"precision_policy": "FP16 immutable ContextRefiner weights expanded to FP32; FP32 context embedding, arithmetic, reductions, softmax, and RoPE; no activation FP16",
		"input":            map[string]any{"relative_path": "input.f32.bin", "sha256": digest(inputData), "shape": []int{1, zimage.ContextRefinerTokens, zimage.ContextRefinerWidth}, "dtype": "float32", "semantic_space": "ContextEmbedding"},
		"final_output":     map[string]any{"relative_path": "final_output.f32.bin", "sha256": digest(finalData), "shape": []int{1, zimage.ContextRefinerTokens, zimage.ContextRefinerWidth}, "dtype": "float32", "semantic_space": "ContextEmbedding"},
		"stages":           stages,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		panic(err)
	}
	if err = write(filepath.Join(*out, "manifest.json"), append(data, '\n')); err != nil {
		panic(err)
	}
	fmt.Printf("block=%s input=%s final=%s stages=%d\n", *block, digest(inputData), digest(finalData), len(names))
}
