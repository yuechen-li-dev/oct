package zimage

// This file owns the one closed M2C MainTransformer package. It deliberately
// names the source `layers.0` prefix rather than accepting an arbitrary
// transformer-layer selector: M2C proves one representative parameter set,
// while M2D will bind the already-inventoried remaining 29 packages.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	MainTransformerTransformID           = "zimage-main-transformer.0-bf16-fp16-transpose-v1"
	MainTransformerCacheSchema           = "oct.prometheus.evt2.m2c.fp16-cache.v1"
	MainTransformer0CacheAggregateSHA256 = "48e987811885741ae5f1bf16b28db33ca7f23e09f1e99c1c2fe3d81bdd1caeb6"
	MainTransformerBlock                 = "layers.0"
	MainTransformerTokens                = 1056 // 1024 image tokens followed by 32 context tokens.
	MainTransformerImageTokens           = 1024
	MainTransformerContextTokens         = 32
	MainTransformerWidth                 = 3840
	MainTransformerHeads                 = 30
	MainTransformerHeadWidth             = 128
	MainTransformerHiddenWidth           = 10240
)

var mainTransformer0Specs = []cacheSpec{
	{"layers.0.adaLN_modulation.0.bias", "AdaLN bias", false, []uint64{15360}},
	{"layers.0.adaLN_modulation.0.weight", "AdaLN projection", true, []uint64{15360, 256}},
	{"layers.0.attention.k_norm.weight", "K RMSNorm", false, []uint64{128}},
	{"layers.0.attention.out.weight", "attention output projection", true, []uint64{3840, 3840}},
	{"layers.0.attention.q_norm.weight", "Q RMSNorm", false, []uint64{128}},
	{"layers.0.attention.qkv.weight", "fused QKV projection; logical Q,K,V views in Q,K,V order", true, []uint64{11520, 3840}},
	{"layers.0.attention_norm1.weight", "attention pre-norm", false, []uint64{3840}},
	{"layers.0.attention_norm2.weight", "attention post-norm", false, []uint64{3840}},
	{"layers.0.feed_forward.w1.weight", "FFN W1", true, []uint64{10240, 3840}},
	{"layers.0.feed_forward.w2.weight", "FFN W2", true, []uint64{3840, 10240}},
	{"layers.0.feed_forward.w3.weight", "FFN W3", true, []uint64{10240, 3840}},
	{"layers.0.ffn_norm1.weight", "FFN pre-norm", false, []uint64{3840}},
	{"layers.0.ffn_norm2.weight", "FFN post-norm", false, []uint64{3840}},
}

// MainTransformerLayerBlock validates the only legal package namespace for a
// source-defined MainTransformer block.  The runtime receives resolved layer
// identities from the lock; this helper is solely the deterministic cache
// producer and rejects arbitrary tensor prefixes.
func MainTransformerLayerBlock(index uint32) (string, error) {
	if index >= 30 {
		return "", fmt.Errorf("main transformer layer index %d is outside the closed 30-block model", index)
	}
	return "layers." + strconv.FormatUint(uint64(index), 10), nil
}

func mainTransformerSpecsForBlock(block string) []cacheSpec {
	specs := make([]cacheSpec, 0, len(mainTransformer0Specs))
	for _, spec := range mainTransformer0Specs {
		copy := spec
		copy.name = strings.Replace(spec.name, MainTransformerBlock, block, 1)
		specs = append(specs, copy)
	}
	return specs
}

func MainTransformerCacheRoot(cacheRoot string) string {
	return MainTransformerLayerCacheRoot(cacheRoot, 0)
}

func MainTransformerLayerCacheRoot(cacheRoot string, index uint32) string {
	block, err := MainTransformerLayerBlock(index)
	if err != nil {
		return ""
	}
	return filepath.Join(cacheRoot, "layers", NoiseRefiner0SourceCheckpointSHA256, block)
}

func MainTransformerCanonicalRoot(cacheRoot string) string {
	return filepath.Join(cacheRoot, "canonical", NoiseRefiner0OracleRevision, "m2c-fp32-reference", MainTransformerBlock)
}

// BuildMainTransformerCache reads exactly the selected representative's
// immutable tensor-role set. The cache never claims that another layer's
// same-shaped tensors are interchangeable; M2D must create a separate closed
// package and prove its aggregate identity before rebinding it.
func BuildMainTransformerCache(sourcePath, cacheRoot, expectedSHA256 string) (CacheManifest, []NoiseRefiner1TensorInventory, error) {
	return BuildMainTransformerLayerCache(sourcePath, cacheRoot, expectedSHA256, 0)
}

// BuildMainTransformerLayerCache is the closed all-layer package producer. It
// accepts one of the pinned source's thirty numeric layer IDs, validates the
// exact thirteen-role set, and emits a distinct aggregate for that immutable
// parameter package.
func BuildMainTransformerLayerCache(sourcePath, cacheRoot, expectedSHA256 string, index uint32) (CacheManifest, []NoiseRefiner1TensorInventory, error) {
	block, err := MainTransformerLayerBlock(index)
	if err != nil {
		return CacheManifest{}, nil, err
	}
	schema := "oct.prometheus.evt2.m2d.fp16-cache.v1"
	transform := "zimage-main-transformer-bf16-fp16-transpose-v1"
	if index == 0 {
		schema = MainTransformerCacheSchema
		transform = MainTransformerTransformID
	}
	return buildClosedFP16Cache(sourcePath, cacheRoot, expectedSHA256, block, schema, transform, mainTransformerSpecsForBlock(block))
}

// LoadMainTransformerCacheManifest verifies the full representative package,
// including every named role, source/cache identity, and aggregate derivation.
// The caller receives only immutable descriptor material, never a mutable role
// map or a runtime-selected tensor list.
func LoadMainTransformerCacheManifest(cacheRoot string) (CacheManifest, error) {
	encoded, err := os.ReadFile(filepath.Join(MainTransformerCacheRoot(cacheRoot), "manifest.json"))
	if err != nil {
		return CacheManifest{}, err
	}
	var manifest CacheManifest
	if err = json.Unmarshal(encoded, &manifest); err != nil {
		return CacheManifest{}, err
	}
	if manifest.Schema != MainTransformerCacheSchema || manifest.TransformID != MainTransformerTransformID ||
		manifest.Block != MainTransformerBlock || manifest.SourceCheckpointSHA256 != NoiseRefiner0SourceCheckpointSHA256 ||
		manifest.AggregateSHA256 != MainTransformer0CacheAggregateSHA256 || len(manifest.Tensors) != len(mainTransformer0Specs) {
		return CacheManifest{}, fmt.Errorf("main transformer representative cache manifest contract mismatch")
	}
	byName := make(map[string]CacheTensor, len(manifest.Tensors))
	for _, tensor := range manifest.Tensors {
		byName[tensor.SourceName] = tensor
	}
	h := sha256.New()
	_, _ = h.Write([]byte(manifest.TransformID + "\n" + manifest.SourceCheckpointSHA256 + "\n"))
	var total uint64
	for _, spec := range mainTransformer0Specs {
		tensor, ok := byName[spec.name]
		if !ok || !sameShape(tensor.SourceShape, spec.shape) || tensor.Transpose != spec.transpose {
			return CacheManifest{}, fmt.Errorf("main transformer tensor contract mismatch for %q", spec.name)
		}
		data, readErr := os.ReadFile(filepath.Join(MainTransformerCacheRoot(cacheRoot), tensor.DestinationName))
		if readErr != nil || uint64(len(data)) != tensor.Bytes {
			return CacheManifest{}, fmt.Errorf("main transformer cache payload missing or truncated: %s", tensor.DestinationName)
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != tensor.SHA256 {
			return CacheManifest{}, fmt.Errorf("main transformer cache payload identity mismatch: %s", tensor.DestinationName)
		}
		total += tensor.Bytes
	}
	if total != 361820672 {
		return CacheManifest{}, fmt.Errorf("main transformer cache byte count mismatch: got %d want 361820672", total)
	}
	tensors := append([]CacheTensor(nil), manifest.Tensors...)
	sort.Slice(tensors, func(i, j int) bool { return tensors[i].SourceName < tensors[j].SourceName })
	for _, tensor := range tensors {
		_, _ = h.Write([]byte(tensor.SourceName + "\n" + tensor.SourceSHA256 + "\n" + tensor.SHA256 + "\n"))
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != MainTransformer0CacheAggregateSHA256 {
		return CacheManifest{}, fmt.Errorf("main transformer cache aggregate mismatch: got %s want %s", got, MainTransformer0CacheAggregateSHA256)
	}
	return manifest, nil
}
