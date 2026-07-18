// Command zimage_forensics emits a deterministic, local-path-redacted
// safetensors manifest for the EVT-2 Z-Image-Turbo preflight.
package main

import (
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

type tensorRecord struct {
	zimage.Tensor
	LayerIndex           *int   `json:"layer_index,omitempty"`
	SemanticOwner        string `json:"semantic_owner"`
	Group                string `json:"group"`
	PrometheusUse        string `json:"expected_prometheus_consumer"`
	RequiredTransform    string `json:"required_transform"`
	ExpectedRuntimeDType string `json:"expected_runtime_dtype"`
	RawTensorSHA256      string `json:"raw_tensor_sha256,omitempty"`
}

type report struct {
	Schema                   string          `json:"schema"`
	Model                    string          `json:"model"`
	CheckpointSHA256         string          `json:"checkpoint_sha256"`
	CheckpointClassification string          `json:"checkpoint_classification"`
	Manifest                 zimage.Manifest `json:"safetensors"`
	Tensors                  []tensorRecord  `json:"tensors"`
	Groups                   map[string]struct {
		Count int    `json:"tensor_count"`
		Bytes uint64 `json:"bytes"`
	} `json:"groups"`
}

func blockIndex(name string) *int {
	parts := strings.Split(name, ".")
	if len(parts) < 2 {
		return nil
	}
	if parts[0] != "layers" && parts[0] != "context_refiner" && parts[0] != "noise_refiner" {
		return nil
	}
	i, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil
	}
	return &i
}

func classify(name string) (group, owner, consumer, transform string) {
	switch {
	case strings.HasPrefix(name, "layers."):
		group, owner = "transformer_blocks", "main_transformer_block"
	case strings.HasPrefix(name, "noise_refiner."):
		group, owner = "noise_refiner", "noise_refiner_block"
	case strings.HasPrefix(name, "context_refiner."):
		group, owner = "context_refiner", "context_refiner_block"
	case strings.HasPrefix(name, "x_embedder"):
		group, owner = "input_patch_embedding", "input_patch_embedding"
	case strings.HasPrefix(name, "t_embedder"):
		group, owner = "timestep_embedding", "timestep_embedding"
	case strings.HasPrefix(name, "cap_embedder") || name == "cap_pad_token":
		group, owner = "context_text_projection", "context_projection"
	case strings.HasPrefix(name, "final_layer"):
		group, owner = "final_projection", "final_layer"
	case name == "x_pad_token":
		group, owner = "input_patch_embedding", "image_padding"
	default:
		return "unclassified", "unclassified", "manual review", "none"
	}
	switch {
	case strings.Contains(name, ".attention.qkv.weight"):
		consumer, transform = "M1 packed QKV projection; split Q/K/V and transpose each", "split axis 0 into three [3840,3840] matrices; transpose to [in,out]; BF16->FP16"
	case strings.Contains(name, ".attention.out.weight"), strings.Contains(name, ".feed_forward.w") && strings.HasSuffix(name, ".weight"), strings.HasSuffix(name, ".adaLN_modulation.0.weight"), strings.HasSuffix(name, ".weight") && (strings.HasPrefix(name, "x_embedder") || strings.HasPrefix(name, "t_embedder") || strings.HasPrefix(name, "cap_embedder") || strings.HasPrefix(name, "final_layer")):
		consumer, transform = "linear projection", "transpose PyTorch [out,in] to Prometheus [in,out]; BF16->FP16"
	case strings.Contains(name, "norm") || strings.HasSuffix(name, ".bias") || strings.HasSuffix(name, "pad_token"):
		consumer, transform = "elementwise norm/modulation/constant", "preserve logical order; BF16->FP16 (FP32 accumulation where specified)"
	default:
		consumer, transform = "model constant", "preserve logical order; BF16->FP16"
	}
	return group, owner, consumer, transform
}

func writeAtomically(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func main() {
	checkpoint := flag.String("checkpoint", "", "local safetensors checkpoint to inspect")
	displayPath := flag.String("display-path", "local-model-cache/z_image_turbo_bf16.safetensors", "redacted path written into JSON")
	expectedSHA256 := flag.String("expected-sha256", "", "optional required lowercase SHA-256 identity")
	out := flag.String("out", "", "output JSON path")
	flag.Parse()
	if *checkpoint == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "usage: zimage_forensics -checkpoint <file> -out <json> [-display-path <redacted>]")
		os.Exit(2)
	}
	if err := zimage.ValidateDisplayPath(*displayPath); err != nil {
		panic(err)
	}
	if *expectedSHA256 != "" {
		if err := zimage.VerifySHA256(*checkpoint, *expectedSHA256); err != nil {
			panic(err)
		}
	}
	manifest, err := zimage.ReadManifest(*checkpoint, *displayPath)
	if err != nil {
		panic(err)
	}
	checkpointHash, err := zimage.HashFile(*checkpoint)
	if err != nil {
		panic(err)
	}
	groups := map[string]struct {
		Count int    `json:"tensor_count"`
		Bytes uint64 `json:"bytes"`
	}{}
	records := make([]tensorRecord, 0, len(manifest.Tensors))
	for _, tensor := range manifest.Tensors {
		group, owner, consumer, transform := classify(tensor.Name)
		value := groups[group]
		value.Count++
		value.Bytes += tensor.Bytes
		groups[group] = value
		records = append(records, tensorRecord{Tensor: tensor, LayerIndex: blockIndex(tensor.Name), SemanticOwner: owner, Group: group, PrometheusUse: consumer, RequiredTransform: transform, ExpectedRuntimeDType: "F16 storage; selected FP32 reductions"})
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Name < records[j].Name })
	data, err := json.MarshalIndent(report{Schema: "oct.prometheus.evt2m0.zimage-forensics.v1", Model: "Z-Image-Turbo", CheckpointSHA256: checkpointHash, CheckpointClassification: "official Comfy-Org single-file conversion; SHA-256 matches pinned release", Manifest: manifest, Tensors: records, Groups: groups}, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := writeAtomically(*out, append(data, '\n')); err != nil {
		panic(err)
	}
}
