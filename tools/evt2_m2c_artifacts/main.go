// evt2_m2c_artifacts projects the closed M2C source/cache/oracle facts into
// reviewable JSON.  It reads the prior checkpoint forensic inventory instead
// of maintaining a per-layer table by hand.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/prometheus/zimage"
)

const (
	pinnedSourceRevision = "26f23eda626ffadda020b04ff79488e1d72004cd"
	pinnedModelRevision  = "f332072aa78be7aecdf3ee76d5c247082da564a6"
)

type forensicTensor struct {
	Name  string   `json:"name"`
	DType string   `json:"dtype"`
	Shape []uint64 `json:"shape"`
}

type forensicDocument struct {
	Safetensors struct {
		Tensors []forensicTensor `json:"tensors"`
	} `json:"safetensors"`
}

type blockFingerprint struct {
	Block       string   `json:"block"`
	TensorCount int      `json:"tensor_count"`
	Roles       []string `json:"roles"`
	Fingerprint string   `json:"fingerprint_sha256"`
}

func main() {
	cacheRoot := flag.String("cache-root", "", "EVT-2 local cache root")
	forensics := flag.String("forensics", "internal/prometheus/DevelopmentReport/artifacts/Evt2M0/z_image_turbo_forensics.json", "pinned checkpoint forensic inventory")
	out := flag.String("out", "internal/prometheus/DevelopmentReport/artifacts/Evt2M2c", "artifact output directory")
	flag.Parse()
	if *cacheRoot == "" {
		fmt.Fprintln(os.Stderr, "-cache-root is required")
		os.Exit(2)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail(err)
	}
	var forensic forensicDocument
	if err := readJSON(*forensics, &forensic); err != nil {
		fail(fmt.Errorf("read checkpoint forensic inventory: %w", err))
	}
	fingerprints, err := structuralFingerprints(forensic.Safetensors.Tensors)
	if err != nil {
		fail(err)
	}
	if len(fingerprints) != 30 {
		fail(fmt.Errorf("expected 30 MainTransformer blocks, found %d", len(fingerprints)))
	}
	representative := fingerprints[0]
	for _, fingerprint := range fingerprints[1:] {
		if fingerprint.Fingerprint != representative.Fingerprint || fingerprint.TensorCount != representative.TensorCount {
			fail(fmt.Errorf("source structural fingerprint differs at %s", fingerprint.Block))
		}
	}
	cachePath := zimage.MainTransformerCacheRoot(*cacheRoot)
	canonicalPath := zimage.MainTransformerCanonicalRoot(*cacheRoot)
	var cacheInventory any
	if err := readJSON(filepath.Join(cachePath, "tensor_inventory.json"), &cacheInventory); err != nil {
		fail(fmt.Errorf("read derived cache inventory: %w", err))
	}
	var canonical any
	if err := readJSON(filepath.Join(canonicalPath, "manifest.json"), &canonical); err != nil {
		fail(fmt.Errorf("read canonical authority: %w", err))
	}

	write(*out, "m2c_main_source_audit.json", map[string]any{
		"schema": "oct.prometheus.evt2.m2c.main-source-audit.v1",
		"source": map[string]any{
			"repository": "Tongyi-MAI/Z-Image",
			"revision":   pinnedSourceRevision,
			"path":       "src/zimage/transformer.py",
			"url":        "https://raw.githubusercontent.com/Tongyi-MAI/Z-Image/" + pinnedSourceRevision + "/src/zimage/transformer.py",
		},
		"main_family": map[string]any{
			"constructor":                  "ZImageTransformerBlock(modulation=True)",
			"container":                    "self.layers",
			"block_count":                  30,
			"indexed_source_special_cases": false,
			"checkpoint_prefix":            "layers.N",
		},
		"attention":    "QKV projection; per-head Q/K RMSNorm; three-axis RoPE; non-causal stable softmax attention; output projection",
		"feed_forward": "w2(SiLU(w1(x)) * w3(x))",
		"modulation":   "linear(adaln_input) split Q scale/gate then MLP scale/gate; scales are 1 + projection; gates use tanh",
		"egress":       "the source applies final projection only to the image prefix; M2C still audits the updated context suffix before that projection",
	})
	write(*out, "m2c_main_block_equations.json", map[string]any{
		"schema":             "oct.prometheus.evt2.m2c.main-equations.v1",
		"input":              "X = Concat(Image[1024,3840], Context[32,3840])",
		"adaln":              "[sA,gA,sM,gM] = split(linear(timestep)); sA=1+sA; sM=1+sM; gA=tanh(gA); gM=tanh(gM)",
		"attention":          "A = OutProj(Softmax((RoPE(RMSNorm(Q)) @ RoPE(RMSNorm(K))^T) / sqrt(128)) @ V)",
		"attention_residual": "R = X + gA * RMSNorm(A)",
		"ffn":                "H = SiLU(W1(RMSNorm(R) * sM)) * W3(RMSNorm(R) * sM); Y = R + gM * RMSNorm(W2(H))",
		"precision":          "FP16 immutable weights widened at use; all listed arithmetic and resident stages remain FP32",
	})
	write(*out, "m2c_structural_fingerprints.json", map[string]any{
		"schema":            "oct.prometheus.evt2.m2c.structural-fingerprints.v1",
		"checkpoint":        "sha256:" + zimage.NoiseRefiner0SourceCheckpointSHA256,
		"fingerprint_basis": "source dtype plus role suffix plus shape; values and cache aggregates remain package-specific",
		"representative":    zimage.MainTransformerBlock,
		"all_blocks":        fingerprints,
	})
	write(*out, "m2c_representative_selection.json", map[string]any{
		"schema":   "oct.prometheus.evt2.m2c.representative-selection.v1",
		"selected": zimage.MainTransformerBlock,
		"selection_basis": []string{
			"pinned source constructs every self.layers entry with the same modulation-enabled block class",
			"all thirty checkpoint prefixes have the same 13 source role/shape/dtype fingerprint",
			"layers.0 is the first source-defined main block and has a separate immutable cache aggregate",
		},
		"non_claim": "Structural equivalence does not permit value substitution: M2D must bind the remaining 29 aggregate-identified packages.",
	})
	write(*out, "m2c_stream_contract.json", map[string]any{
		"schema":      "oct.prometheus.evt2.m2c.joint-stream-contract.v1",
		"composition": "Joint = Concat(Image, Context)",
		"layout": []map[string]any{
			{"stream": "Image", "token_offset": 0, "tokens": 1024, "width": 3840, "positions": "[33, token//32, token%32]"},
			{"stream": "Context", "token_offset": 1024, "tokens": 32, "width": 3840, "positions": "[token+1, 0, 0]"},
		},
		"joint_tokens":      1056,
		"dtype":             "FP32",
		"semantics":         "both prefixes participate in every main-block attention, residual, and FFN update",
		"ingress_authority": "NoiseRefiner1 and ContextRefiner1 accepted FP32 final resident outputs",
	})
	write(*out, "m2c_cache_inventory.json", cacheInventory)
	write(*out, "m2c_canonical_authority.json", canonical)
	write(*out, "m2c_precision_policy.json", map[string]any{
		"schema":        "oct.prometheus.evt2.m2c.precision-policy.v1",
		"weights":       "checkpoint BF16 -> immutable FP16 cache -> FP32 at use",
		"fp32_resident": []string{"joint ingress", "AdaLN", "RMSNorm", "QKV", "RoPE", "attention logits/probabilities/value accumulation", "output projection", "FFN", "residuals", "image and context egress"},
		"prohibited":    []string{"activation FP16 narrowing", "score/probability tensor materialization", "tolerance widening", "saturation or clamping"},
	})
	write(*out, "m2c_shader_portfolio.json", map[string]any{
		"schema": "oct.prometheus.evt2.m2c.shader-portfolio.v1",
		"reused_physical_kernels": []map[string]any{
			{"id": 24, "role": "AdaLN"}, {"id": 25, "role": "RMSNorm plus modulation"},
			{"id": 26, "role": "fused QKV"}, {"id": 31, "role": "attention output projection"},
			{"id": 32, "role": "gated attention residual"}, {"id": 33, "role": "FFN norm plus modulation"},
			{"id": 36, "role": "W2 plus final residual"},
		},
		"new_physical_kernels": []map[string]any{
			{"id": 40, "role": "joint Q/K RMSNorm plus two-domain RoPE", "reason": "image prefix and text suffix coordinate programs"},
			{"id": 41, "role": "1056-token joint one-query-head attention", "reason": "1024 and 32 token workgroup arrays are ineligible"},
			{"id": 42, "role": "1056-token W1/W3 projection", "reason": "fixed 1024-token W3 view partition is ineligible"},
			{"id": 43, "role": "1056-token SiLU gate", "reason": "fixed 1024-token W3 view partition is ineligible"},
		},
		"semantic_wrapper_rule": "each reused physical kernel retains an assembly-specific descriptor binding and named semantic boundary; there is no source copy presented as a new primitive",
	})
	write(*out, "m2c_semantic_space_contract.json", map[string]any{
		"schema":      "oct.prometheus.evt2.m2c.semantic-space-contract.v1",
		"group":       "zimage.main_transformer",
		"spaces":      []string{"JointEmbedding", "JointQueryHead", "JointKeyHead", "PositionedJointQueryHead", "PositionedJointKeyHead", "ImageCoordinate", "ContextCoordinate"},
		"transitions": []string{"JointEmbedding -> JointQueryHead", "JointEmbedding -> JointKeyHead", "JointQueryHead -> PositionedJointQueryHead", "JointKeyHead -> PositionedJointKeyHead"},
		"invalid_fixtures": []map[string]string{
			{"path": "examples/SDSL-V/conformance/invalid/MainTransformerContextKeyAsImageKey.sdslvinvalid", "diagnostic": "SDSL-V4123", "mistake": "cross-stream positioned-key use"},
			{"path": "examples/SDSL-V/conformance/invalid/MainTransformerContextCoordinateAsImageCoordinate.sdslvinvalid", "diagnostic": "SDSL-V4123", "mistake": "context coordinate passed to image RoPE"},
		},
		"erasure":   "nominal vector-value aliases erase before HLSL/SPIR-V; descriptor ABI, buffer layout, and runtime tags are unchanged",
		"non_claim": "token axes and probability-to-value token alignment remain out of scope for current SDSL-V tensor-axis typing",
	})
}

func structuralFingerprints(tensors []forensicTensor) ([]blockFingerprint, error) {
	byBlock := make(map[int][]string)
	for _, tensor := range tensors {
		if !strings.HasPrefix(tensor.Name, "layers.") {
			continue
		}
		parts := strings.SplitN(tensor.Name, ".", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("malformed main tensor name %q", tensor.Name)
		}
		index, err := strconv.Atoi(parts[1])
		if err != nil || index < 0 || index >= 30 {
			return nil, fmt.Errorf("malformed main tensor index in %q", tensor.Name)
		}
		role := tensor.DType + ":" + parts[2] + ":" + fmt.Sprint(tensor.Shape)
		byBlock[index] = append(byBlock[index], role)
	}
	result := make([]blockFingerprint, 0, 30)
	for index := 0; index < 30; index++ {
		roles := append([]string(nil), byBlock[index]...)
		sort.Strings(roles)
		if len(roles) == 0 {
			return nil, fmt.Errorf("missing layers.%d structural inventory", index)
		}
		encoded, err := json.Marshal(roles)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(encoded)
		result = append(result, blockFingerprint{Block: fmt.Sprintf("layers.%d", index), TensorCount: len(roles), Roles: roles, Fingerprint: hex.EncodeToString(sum[:])})
	}
	return result, nil
}

func readJSON(path string, out any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

func write(out, name string, value any) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(filepath.Join(out, name), append(encoded, '\n'), 0o644); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
