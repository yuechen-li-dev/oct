package zimage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

const (
	ContextRefinerTransformID                  = "zimage-context-refiner-bf16-fp16-transpose-v1"
	ContextRefinerCacheSchema                  = "oct.prometheus.evt2.m2b.fp16-cache.v1"
	ContextRefinerCacheBytes            uint64 = 353925632
	ContextRefiner0CacheAggregateSHA256        = "c08b908a921a80e16995abc3f3eefcadd1a94a78cd76b56e58d6b21e6ce412ae"
	ContextRefiner1CacheAggregateSHA256        = "30268c3b0d7a6fafc411119c929326854e73d394ec95ebeba21f89aa43dfc95f"
	ContextRefiner0CanonicalFinalSHA256        = "d2b8167de614da25211eb69991e1b7700992bf5f4ae527bff8a55366ea1ae6df"
	ContextRefiner1CanonicalFinalSHA256        = "08377e8a46b65cff998b740a3fd0ba3c1565b471dad71fcca3f4310617c220b0"
)

// ContextRefinerTensorInventory has the shared, per-tensor conversion proof
// shape. Its owning package and aggregate remain ContextRefiner-specific.
type ContextRefinerTensorInventory = NoiseRefiner1TensorInventory

type ContextRefinerCacheResult struct {
	Manifest  CacheManifest                   `json:"manifest"`
	Inventory []ContextRefinerTensorInventory `json:"inventory"`
}

var contextRefiner0Specs = contextRefinerSpecs("context_refiner.0")
var contextRefiner1Specs = contextRefinerSpecs("context_refiner.1")

func contextRefinerSpecs(block string) []cacheSpec {
	return []cacheSpec{
		{block + ".attention.k_norm.weight", "K RMSNorm", false, []uint64{128}},
		{block + ".attention.out.weight", "attention output projection", true, []uint64{3840, 3840}},
		{block + ".attention.q_norm.weight", "Q RMSNorm", false, []uint64{128}},
		{block + ".attention.qkv.weight", "fused QKV projection; logical Q,K,V views in Q,K,V order", true, []uint64{11520, 3840}},
		{block + ".attention_norm1.weight", "attention pre-norm", false, []uint64{3840}},
		{block + ".attention_norm2.weight", "attention post-norm", false, []uint64{3840}},
		{block + ".feed_forward.w1.weight", "FFN W1", true, []uint64{10240, 3840}},
		{block + ".feed_forward.w2.weight", "FFN W2", true, []uint64{3840, 10240}},
		{block + ".feed_forward.w3.weight", "FFN W3", true, []uint64{10240, 3840}},
		{block + ".ffn_norm1.weight", "FFN pre-norm", false, []uint64{3840}},
		{block + ".ffn_norm2.weight", "FFN post-norm", false, []uint64{3840}},
	}
}

func ContextRefinerCacheRoot(cacheRoot, block string) string {
	return filepath.Join(cacheRoot, "layers", NoiseRefiner0SourceCheckpointSHA256, block)
}

func ContextRefinerCanonicalRoot(cacheRoot, block string) string {
	return filepath.Join(cacheRoot, "canonical", NoiseRefiner0OracleRevision, "m2b-fp32-reference", block)
}

// BuildContextRefinerCache accepts exactly one of the two source-derived
// ContextRefiner packages. It cannot be pointed at a NoiseRefiner name.
func BuildContextRefinerCache(sourcePath, cacheRoot, expectedSHA256, block string) (ContextRefinerCacheResult, error) {
	var specs []cacheSpec
	switch block {
	case "context_refiner.0":
		specs = contextRefiner0Specs
	case "context_refiner.1":
		specs = contextRefiner1Specs
	default:
		return ContextRefinerCacheResult{}, fmt.Errorf("unknown closed ContextRefiner block %q", block)
	}
	manifest, inventory, err := buildClosedFP16Cache(sourcePath, cacheRoot, expectedSHA256, block,
		ContextRefinerCacheSchema, ContextRefinerTransformID, specs)
	if err != nil {
		return ContextRefinerCacheResult{}, err
	}
	return ContextRefinerCacheResult{Manifest: manifest, Inventory: inventory}, nil
}

// LoadContextRefinerCacheManifest validates a closed, already-derived package.
// Aggregate identity is returned to the lock author; callers must not infer it
// from same-shaped NoiseRefiner tensors.
func LoadContextRefinerCacheManifest(cacheRoot, block string) (CacheManifest, error) {
	var specs []cacheSpec
	switch block {
	case "context_refiner.0":
		specs = contextRefiner0Specs
	case "context_refiner.1":
		specs = contextRefiner1Specs
	default:
		return CacheManifest{}, fmt.Errorf("unknown closed ContextRefiner block %q", block)
	}
	encoded, err := os.ReadFile(filepath.Join(ContextRefinerCacheRoot(cacheRoot, block), "manifest.json"))
	if err != nil {
		return CacheManifest{}, err
	}
	var manifest CacheManifest
	if err = json.Unmarshal(encoded, &manifest); err != nil {
		return CacheManifest{}, err
	}
	if manifest.Schema != ContextRefinerCacheSchema || manifest.TransformID != ContextRefinerTransformID ||
		manifest.Block != block || manifest.SourceCheckpointSHA256 != NoiseRefiner0SourceCheckpointSHA256 || len(manifest.Tensors) != len(specs) {
		return CacheManifest{}, fmt.Errorf("%s cache manifest contract mismatch", block)
	}
	wantAggregate := ContextRefiner0CacheAggregateSHA256
	if block == "context_refiner.1" {
		wantAggregate = ContextRefiner1CacheAggregateSHA256
	}
	if manifest.AggregateSHA256 != wantAggregate {
		return CacheManifest{}, fmt.Errorf("%s cache aggregate mismatch: got %s want %s", block, manifest.AggregateSHA256, wantAggregate)
	}
	byName := make(map[string]CacheTensor, len(manifest.Tensors))
	var total uint64
	for _, tensor := range manifest.Tensors {
		byName[tensor.SourceName] = tensor
	}
	h := sha256.New()
	_, _ = h.Write([]byte(manifest.TransformID + "\n" + manifest.SourceCheckpointSHA256 + "\n"))
	for _, spec := range specs {
		tensor, ok := byName[spec.name]
		if !ok || !sameShape(tensor.SourceShape, spec.shape) || tensor.Transpose != spec.transpose {
			return CacheManifest{}, fmt.Errorf("%s tensor contract mismatch for %q", block, spec.name)
		}
		data, readErr := os.ReadFile(filepath.Join(ContextRefinerCacheRoot(cacheRoot, block), tensor.DestinationName))
		if readErr != nil || uint64(len(data)) != tensor.Bytes {
			return CacheManifest{}, fmt.Errorf("%s cache payload missing or truncated: %s", block, tensor.DestinationName)
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != tensor.SHA256 {
			return CacheManifest{}, fmt.Errorf("%s cache payload identity mismatch: %s", block, tensor.DestinationName)
		}
		total += tensor.Bytes
	}
	if total != ContextRefinerCacheBytes {
		return CacheManifest{}, fmt.Errorf("%s cache byte count mismatch: got %d want %d", block, total, ContextRefinerCacheBytes)
	}
	tensors := append([]CacheTensor(nil), manifest.Tensors...)
	sort.Slice(tensors, func(i, j int) bool { return tensors[i].SourceName < tensors[j].SourceName })
	for _, tensor := range tensors {
		_, _ = h.Write([]byte(tensor.SourceName + "\n" + tensor.SourceSHA256 + "\n" + tensor.SHA256 + "\n"))
	}
	if actual := hex.EncodeToString(h.Sum(nil)); actual != manifest.AggregateSHA256 {
		return CacheManifest{}, fmt.Errorf("%s cache aggregate mismatch: got %s want %s", block, actual, manifest.AggregateSHA256)
	}
	return manifest, nil
}
