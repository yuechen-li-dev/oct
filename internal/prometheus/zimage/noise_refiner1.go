package zimage

// This file owns the second, closed noise-refiner weight package.  It is not a
// generic checkpoint importer: the source-derived 13-tensor declaration is
// deliberately repeated here so a block-0 package cannot be accepted as a
// block-1 package merely because the shapes agree.

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
)

const (
	NoiseRefiner1TransformID                 = "zimage-noise-refiner.1-bf16-fp16-transpose-v1"
	NoiseRefiner1CacheSchema                 = "oct.prometheus.evt2.m2a.fp16-cache.v1"
	NoiseRefiner1CacheBytes           uint64 = 361820672
	NoiseRefiner1CacheAggregateSHA256        = "80c0cd75f44cc434d9306c0fd9a8f02e48b593ecc254de01c1f8fcc29f4bc7c8"
	NoiseRefiner1CanonicalFinalSHA256        = "9b133c9ed3772f782e1bd77ff5b89732dc28406eec2078f1692d1899e2eb39e7"
)

func NoiseRefiner1CacheRoot(cacheRoot string) string {
	return filepath.Join(cacheRoot, "layers", NoiseRefiner0SourceCheckpointSHA256, "noise_refiner.1")
}

func NoiseRefiner1CanonicalRoot(cacheRoot string) string {
	return filepath.Join(cacheRoot, "canonical", NoiseRefiner0OracleRevision, "o19-fp32-reference", "noise_refiner.1")
}

// LoadNoiseRefiner1CacheManifest verifies the whole closed block package. It
// does not accept a same-shaped block-0 payload or a partial tensor set.
func LoadNoiseRefiner1CacheManifest(cacheRoot string) (CacheManifest, error) {
	root := NoiseRefiner1CacheRoot(cacheRoot)
	encoded, err := os.ReadFile(filepath.Join(root, "manifest.json"))
	if err != nil {
		return CacheManifest{}, err
	}
	var manifest CacheManifest
	if err = json.Unmarshal(encoded, &manifest); err != nil {
		return CacheManifest{}, err
	}
	if manifest.Schema != NoiseRefiner1CacheSchema || manifest.TransformID != NoiseRefiner1TransformID || manifest.Block != "noise_refiner.1" || manifest.SourceCheckpointSHA256 != NoiseRefiner0SourceCheckpointSHA256 || manifest.AggregateSHA256 != NoiseRefiner1CacheAggregateSHA256 || len(manifest.Tensors) != len(noiseRefiner1Specs) {
		return CacheManifest{}, fmt.Errorf("noise_refiner.1 cache manifest contract mismatch")
	}
	byName := make(map[string]CacheTensor, len(manifest.Tensors))
	for _, tensor := range manifest.Tensors {
		byName[tensor.SourceName] = tensor
	}
	var total uint64
	for _, spec := range noiseRefiner1Specs {
		tensor, ok := byName[spec.name]
		if !ok || !sameShape(tensor.SourceShape, spec.shape) || tensor.Transpose != spec.transpose || tensor.Bytes == 0 {
			return CacheManifest{}, fmt.Errorf("noise_refiner.1 tensor contract mismatch for %q", spec.name)
		}
		data, readErr := os.ReadFile(filepath.Join(root, tensor.DestinationName))
		if readErr != nil || uint64(len(data)) != tensor.Bytes {
			return CacheManifest{}, fmt.Errorf("noise_refiner.1 cache payload missing or truncated: %s", tensor.DestinationName)
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != tensor.SHA256 {
			return CacheManifest{}, fmt.Errorf("noise_refiner.1 cache payload identity mismatch: %s", tensor.DestinationName)
		}
		total += tensor.Bytes
	}
	if total != NoiseRefiner1CacheBytes {
		return CacheManifest{}, fmt.Errorf("noise_refiner.1 cache byte count mismatch: %d", total)
	}
	return manifest, nil
}

var noiseRefiner1Specs = []cacheSpec{
	{"noise_refiner.1.adaLN_modulation.0.bias", "AdaLN bias", false, []uint64{15360}},
	{"noise_refiner.1.adaLN_modulation.0.weight", "AdaLN projection", true, []uint64{15360, 256}},
	{"noise_refiner.1.attention.k_norm.weight", "K RMSNorm", false, []uint64{128}},
	{"noise_refiner.1.attention.out.weight", "attention output projection", true, []uint64{3840, 3840}},
	{"noise_refiner.1.attention.q_norm.weight", "Q RMSNorm", false, []uint64{128}},
	{"noise_refiner.1.attention.qkv.weight", "fused QKV projection; M1 takes Q/K/V views in Q,K,V order", true, []uint64{11520, 3840}},
	{"noise_refiner.1.attention_norm1.weight", "attention pre-norm", false, []uint64{3840}},
	{"noise_refiner.1.attention_norm2.weight", "attention post-norm", false, []uint64{3840}},
	{"noise_refiner.1.feed_forward.w1.weight", "FFN W1", true, []uint64{10240, 3840}},
	{"noise_refiner.1.feed_forward.w2.weight", "FFN W2", true, []uint64{3840, 10240}},
	{"noise_refiner.1.feed_forward.w3.weight", "FFN W3", true, []uint64{10240, 3840}},
	{"noise_refiner.1.ffn_norm1.weight", "FFN pre-norm", false, []uint64{3840}},
	{"noise_refiner.1.ffn_norm2.weight", "FFN post-norm", false, []uint64{3840}},
}

// NoiseRefiner1TensorInventory is deliberately payload-free.  It freezes the
// source range and conversion evidence needed to reject a substituted tensor.
type NoiseRefiner1TensorInventory struct {
	CanonicalName        string   `json:"canonical_name"`
	SemanticRole         string   `json:"semantic_role"`
	SourceShape          []uint64 `json:"source_shape"`
	SourceDType          string   `json:"source_dtype"`
	SourceBytes          uint64   `json:"source_bytes"`
	Finite               bool     `json:"finite"`
	Min                  float32  `json:"min"`
	Max                  float32  `json:"max"`
	AbsoluteMax          float32  `json:"absolute_max"`
	FP16Representable    bool     `json:"fp16_representable"`
	FP16OverflowCount    uint64   `json:"fp16_conversion_overflow_count"`
	FP16UnderflowCount   uint64   `json:"fp16_conversion_underflow_count"`
	FP16MaxAbsoluteDrift float32  `json:"fp16_conversion_max_absolute_drift"`
	CacheOrientation     string   `json:"cache_orientation"`
	CacheBytes           uint64   `json:"cache_bytes"`
	SourceSHA256         string   `json:"source_sha256"`
	CacheSHA256          string   `json:"cache_sha256"`
}

type NoiseRefiner1CacheResult struct {
	Manifest  CacheManifest                  `json:"manifest"`
	Inventory []NoiseRefiner1TensorInventory `json:"inventory"`
}

func fp16Finite(bits uint16) bool { return (bits & 0x7c00) != 0x7c00 }

func noiseRefiner1Inventory(input, output []byte, tensor Tensor, spec cacheSpec, sourceHash, cacheHash string) (NoiseRefiner1TensorInventory, error) {
	if len(input)%2 != 0 || len(output) != len(input) {
		return NoiseRefiner1TensorInventory{}, fmt.Errorf("noise_refiner.1 %s conversion size mismatch", spec.name)
	}
	record := NoiseRefiner1TensorInventory{CanonicalName: spec.name, SemanticRole: spec.consumer,
		SourceShape: append([]uint64(nil), spec.shape...), SourceDType: tensor.DType, SourceBytes: tensor.Bytes,
		Finite: true, FP16Representable: true, CacheBytes: uint64(len(output)), SourceSHA256: sourceHash, CacheSHA256: cacheHash,
		CacheOrientation: "vector preserved"}
	if spec.transpose {
		record.CacheOrientation = "row-major [in,out], transposed from PyTorch [out,in]"
	}
	for index := 0; index < len(input); index += 2 {
		bf16 := binary.LittleEndian.Uint16(input[index:])
		fp16 := BF16ToFP16(bf16)
		value := math.Float32frombits(uint32(bf16) << 16)
		cached := FP16ToFloat32(fp16)
		if index == 0 {
			record.Min, record.Max = value, value
		}
		if !isFinite32(value) {
			record.Finite = false
			continue
		}
		if value < record.Min {
			record.Min = value
		}
		if value > record.Max {
			record.Max = value
		}
		if absolute := float32(math.Abs(float64(value))); absolute > record.AbsoluteMax {
			record.AbsoluteMax = absolute
		}
		if !fp16Finite(fp16) {
			record.FP16OverflowCount++
			record.FP16Representable = false
		} else if value != 0 && cached == 0 {
			record.FP16UnderflowCount++
		}
		if drift := float32(math.Abs(float64(value - cached))); drift > record.FP16MaxAbsoluteDrift {
			record.FP16MaxAbsoluteDrift = drift
		}
	}
	return record, nil
}

func isFinite32(value float32) bool {
	return !math.IsNaN(float64(value)) && !math.IsInf(float64(value), 0)
}

// BuildNoiseRefiner1Cache reads exactly the second pinned block.  It never
// calls the block-0 builder and it never writes in the block-0 directory.
func BuildNoiseRefiner1Cache(sourcePath, cacheRoot, expectedSHA256 string) (NoiseRefiner1CacheResult, error) {
	if err := VerifySHA256(sourcePath, expectedSHA256); err != nil {
		return NoiseRefiner1CacheResult{}, err
	}
	manifest, err := ReadManifest(sourcePath, "local-model-cache/z_image_turbo_bf16.safetensors")
	if err != nil {
		return NoiseRefiner1CacheResult{}, err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return NoiseRefiner1CacheResult{}, err
	}
	defer source.Close()
	dir := filepath.Join(cacheRoot, "layers", expectedSHA256, "noise_refiner.1")
	if err = os.MkdirAll(dir, 0755); err != nil {
		return NoiseRefiner1CacheResult{}, err
	}
	result := NoiseRefiner1CacheResult{Manifest: CacheManifest{Schema: NoiseRefiner1CacheSchema, TransformID: NoiseRefiner1TransformID,
		SourceCheckpointSHA256: expectedSHA256, Block: "noise_refiner.1", DType: "FP16 little-endian IEEE-754", Tensors: make([]CacheTensor, 0, len(noiseRefiner1Specs))},
		Inventory: make([]NoiseRefiner1TensorInventory, 0, len(noiseRefiner1Specs))}
	for _, spec := range noiseRefiner1Specs {
		tensor, ok := findTensor(manifest, spec.name)
		if !ok {
			return NoiseRefiner1CacheResult{}, fmt.Errorf("required tensor %q missing", spec.name)
		}
		if tensor.DType != "BF16" || !sameShape(tensor.Shape, spec.shape) {
			return NoiseRefiner1CacheResult{}, fmt.Errorf("tensor %q contract mismatch", spec.name)
		}
		input := make([]byte, tensor.Bytes)
		if _, err = source.ReadAt(input, int64(tensor.FileRange[0])); err != nil {
			return NoiseRefiner1CacheResult{}, err
		}
		output, err := convertTensor(source, tensor, spec)
		if err != nil {
			return NoiseRefiner1CacheResult{}, err
		}
		sourceDigest := sha256.Sum256(input)
		cacheDigest := sha256.Sum256(output)
		path := filepath.Join(dir, fp16Name(spec.name))
		if current, readErr := os.ReadFile(path); readErr != nil || len(current) != len(output) || sha256.Sum256(current) != cacheDigest {
			if err = writeAtomic(path, output); err != nil {
				return NoiseRefiner1CacheResult{}, err
			}
		}
		sourceHash, cacheHash := hex.EncodeToString(sourceDigest[:]), hex.EncodeToString(cacheDigest[:])
		entry, err := noiseRefiner1Inventory(input, output, tensor, spec, sourceHash, cacheHash)
		if err != nil {
			return NoiseRefiner1CacheResult{}, err
		}
		layout := entry.CacheOrientation
		result.Manifest.Tensors = append(result.Manifest.Tensors, CacheTensor{SourceName: spec.name, SourceShape: append([]uint64(nil), tensor.Shape...), SourceRange: tensor.FileRange, SourceSHA256: sourceHash, DestinationName: fp16Name(spec.name), DestinationShape: destinationShape(spec), DestinationLayout: layout, Transpose: spec.transpose, Bytes: uint64(len(output)), SHA256: cacheHash, Consumer: spec.consumer})
		result.Inventory = append(result.Inventory, entry)
	}
	sort.Slice(result.Manifest.Tensors, func(i, j int) bool {
		return result.Manifest.Tensors[i].SourceName < result.Manifest.Tensors[j].SourceName
	})
	sort.Slice(result.Inventory, func(i, j int) bool { return result.Inventory[i].CanonicalName < result.Inventory[j].CanonicalName })
	h := sha256.New()
	_, _ = h.Write([]byte(result.Manifest.TransformID + "\n" + result.Manifest.SourceCheckpointSHA256 + "\n"))
	for _, tensor := range result.Manifest.Tensors {
		_, _ = h.Write([]byte(tensor.SourceName + "\n" + tensor.SourceSHA256 + "\n" + tensor.SHA256 + "\n"))
	}
	result.Manifest.AggregateSHA256 = hex.EncodeToString(h.Sum(nil))
	if err = writeNoiseRefiner1JSON(filepath.Join(dir, "manifest.json"), result.Manifest); err != nil {
		return NoiseRefiner1CacheResult{}, err
	}
	if err = writeNoiseRefiner1JSON(filepath.Join(dir, "tensor_inventory.json"), result.Inventory); err != nil {
		return NoiseRefiner1CacheResult{}, err
	}
	return result, nil
}

func writeNoiseRefiner1JSON(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, append(encoded, '\n'))
}
