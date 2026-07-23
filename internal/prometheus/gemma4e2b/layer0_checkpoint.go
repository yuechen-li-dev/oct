// Package gemma4e2b owns the closed host-side checkpoint contract for the
// Prometheus G4-E2B-M1 layer-0 assembly. It never supplies fallback weights.
package gemma4e2b

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/yuechen-li-dev/oct/internal/prometheus/zimage"
)

const (
	Repository      = "google/gemma-4-E2B-it"
	Revision        = "3e22461f65e89153144f8adb70e3b8c2cc9845a7"
	ModelSHA256     = "2db5482b20d746879bb3ef79b5203e9075a2e2b98f54ec7c2f281c1477ddc550"
	AuthoritySchema = "oct.prometheus.g4-e2b.checkpoint-authority.v1"
)

// TensorRange is an exact BF16 byte range in the one authoritative external
// safetensors file. The range is validated before it can be read for upload.
type TensorRange struct {
	Name        string
	DType       string
	Shape       []uint64
	DataOffsets [2]uint64
	FileRange   [2]uint64
	Bytes       uint64
}

// Layer0Checkpoint holds no weights. It owns only an open handle and the exact
// selected tensor map; callers explicitly read the bounded bytes they upload.
type Layer0Checkpoint struct {
	file    *os.File
	tensors map[string]TensorRange
}

type authorityFile struct {
	Path   string `json:"path"`
	Bytes  uint64 `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type authorityTensor struct {
	Name   string    `json:"name"`
	DType  string    `json:"dtype"`
	Shape  []uint64  `json:"shape"`
	Bytes  uint64    `json:"bytes"`
	Offset [2]uint64 `json:"data_offsets"`
}

type authorityRecord struct {
	Schema      string          `json:"schema"`
	Repository  string          `json:"repository"`
	Revision    string          `json:"revision"`
	Files       []authorityFile `json:"files"`
	Safetensors struct {
		Path                string            `json:"path"`
		Bytes               uint64            `json:"bytes"`
		SHA256              string            `json:"sha256"`
		HeaderBytes         uint64            `json:"header_bytes"`
		HeaderSHA256        string            `json:"header_sha256"`
		PayloadOffset       uint64            `json:"payload_offset"`
		LogicalPayloadBytes uint64            `json:"logical_payload_bytes"`
		TensorCount         int               `json:"tensor_count"`
		Tensors             []authorityTensor `json:"tensors"`
	} `json:"safetensors"`
}

type requiredTensor struct {
	name  string
	shape []uint64
}

var layer0Required = []requiredTensor{
	{"model.language_model.embed_tokens.weight", []uint64{262144, 1536}},
	{"model.language_model.embed_tokens_per_layer.weight", []uint64{262144, 8960}},
	{"model.language_model.per_layer_model_projection.weight", []uint64{8960, 1536}},
	{"model.language_model.per_layer_projection_norm.weight", []uint64{256}},
	{"model.language_model.layers.0.input_layernorm.weight", []uint64{1536}},
	{"model.language_model.layers.0.post_attention_layernorm.weight", []uint64{1536}},
	{"model.language_model.layers.0.pre_feedforward_layernorm.weight", []uint64{1536}},
	{"model.language_model.layers.0.post_feedforward_layernorm.weight", []uint64{1536}},
	{"model.language_model.layers.0.self_attn.q_proj.weight", []uint64{2048, 1536}},
	{"model.language_model.layers.0.self_attn.k_proj.weight", []uint64{256, 1536}},
	{"model.language_model.layers.0.self_attn.v_proj.weight", []uint64{256, 1536}},
	{"model.language_model.layers.0.self_attn.o_proj.weight", []uint64{1536, 2048}},
	{"model.language_model.layers.0.self_attn.q_norm.weight", []uint64{256}},
	{"model.language_model.layers.0.self_attn.k_norm.weight", []uint64{256}},
	{"model.language_model.layers.0.mlp.gate_proj.weight", []uint64{6144, 1536}},
	{"model.language_model.layers.0.mlp.up_proj.weight", []uint64{6144, 1536}},
	{"model.language_model.layers.0.mlp.down_proj.weight", []uint64{1536, 6144}},
	{"model.language_model.layers.0.per_layer_input_gate.weight", []uint64{256, 1536}},
	{"model.language_model.layers.0.per_layer_projection.weight", []uint64{1536, 256}},
	{"model.language_model.layers.0.post_per_layer_input_norm.weight", []uint64{1536}},
	{"model.language_model.layers.0.layer_scalar", []uint64{1}},
}

// OpenLayer0Checkpoint verifies every pinned object and every one of the 2,011
// safetensors records before returning the exact M1 tensor ranges. No random,
// zero, generated, renamed, or partially matching payload is admissible.
func OpenLayer0Checkpoint(root, authorityPath string) (*Layer0Checkpoint, error) {
	authority, err := readAuthority(authorityPath)
	if err != nil {
		return nil, err
	}
	if err := validateFiles(root, authority.Files); err != nil {
		return nil, err
	}
	checkpointPath := filepath.Join(root, authority.Safetensors.Path)
	manifest, err := zimage.ReadManifest(checkpointPath, authority.Safetensors.Path)
	if err != nil {
		return nil, fmt.Errorf("layer-0 safetensors structure: %w", err)
	}
	if err := validateManifest(manifest, authority); err != nil {
		return nil, err
	}
	if err := zimage.VerifySHA256(checkpointPath, ModelSHA256); err != nil {
		return nil, fmt.Errorf("layer-0 checkpoint identity: %w", err)
	}
	byName := make(map[string]zimage.Tensor, len(manifest.Tensors))
	for _, tensor := range manifest.Tensors {
		byName[tensor.Name] = tensor
	}
	ranges := make(map[string]TensorRange, len(layer0Required))
	for _, required := range layer0Required {
		tensor, ok := byName[required.name]
		if !ok || tensor.DType != "BF16" || !sameShape(tensor.Shape, required.shape) {
			return nil, fmt.Errorf("required layer-0 tensor is absent or incompatible: %s", required.name)
		}
		ranges[required.name] = TensorRange{tensor.Name, tensor.DType, append([]uint64(nil), tensor.Shape...), tensor.DataOffsets, tensor.FileRange, tensor.Bytes}
	}
	file, err := os.Open(checkpointPath)
	if err != nil {
		return nil, fmt.Errorf("open validated checkpoint: %w", err)
	}
	return &Layer0Checkpoint{file: file, tensors: ranges}, nil
}

func readAuthority(path string) (authorityRecord, error) {
	var authority authorityRecord
	bytes, err := os.ReadFile(path)
	if err != nil {
		return authority, err
	}
	if err := json.Unmarshal(bytes, &authority); err != nil {
		return authority, fmt.Errorf("decode checkpoint authority: %w", err)
	}
	if authority.Schema != AuthoritySchema || authority.Repository != Repository || authority.Revision != Revision ||
		authority.Safetensors.Path != "model.safetensors" || authority.Safetensors.SHA256 != ModelSHA256 || authority.Safetensors.TensorCount != 2011 {
		return authority, fmt.Errorf("checkpoint authority is not the accepted G4-E2B-M0 identity")
	}
	return authority, nil
}

func validateFiles(root string, files []authorityFile) error {
	if len(files) != 9 {
		return fmt.Errorf("checkpoint authority has %d files; want 9", len(files))
	}
	seen := map[string]bool{}
	for _, file := range files {
		if file.Path == "" || seen[file.Path] || filepath.Base(file.Path) != file.Path {
			return fmt.Errorf("checkpoint authority file set is malformed")
		}
		seen[file.Path] = true
		info, err := os.Stat(filepath.Join(root, file.Path))
		if err != nil || info.IsDir() || uint64(info.Size()) != file.Bytes {
			return fmt.Errorf("required checkpoint object is missing or has wrong size: %s", file.Path)
		}
		hash, err := zimage.HashFile(filepath.Join(root, file.Path))
		if err != nil || hash != file.SHA256 {
			return fmt.Errorf("required checkpoint object identity mismatch: %s", file.Path)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	if len(entries) != len(seen) {
		return fmt.Errorf("checkpoint root has unexpected, renamed, or directory entries")
	}
	for _, entry := range entries {
		if entry.IsDir() || !seen[entry.Name()] {
			return fmt.Errorf("checkpoint root has unexpected object: %s", entry.Name())
		}
	}
	return nil
}

func validateManifest(manifest zimage.Manifest, authority authorityRecord) error {
	if manifest.FileBytes != authority.Safetensors.Bytes || manifest.HeaderBytes != authority.Safetensors.HeaderBytes ||
		manifest.HeaderSHA256 != authority.Safetensors.HeaderSHA256 || manifest.PayloadOffset != authority.Safetensors.PayloadOffset ||
		manifest.PayloadBytes != authority.Safetensors.LogicalPayloadBytes || len(manifest.Tensors) != len(authority.Safetensors.Tensors) {
		return fmt.Errorf("safetensors physical identity mismatch")
	}
	for i, got := range manifest.Tensors {
		want := authority.Safetensors.Tensors[i]
		if got.Name != want.Name || got.DType != want.DType || got.Bytes != want.Bytes || got.DataOffsets != want.Offset || !sameShape(got.Shape, want.Shape) {
			return fmt.Errorf("safetensors authority mismatch at sorted index %d", i)
		}
	}
	return nil
}

// Tensor returns the immutable validated range for one exact layer-0 tensor.
func (checkpoint *Layer0Checkpoint) Tensor(name string) (TensorRange, error) {
	if checkpoint == nil || checkpoint.file == nil {
		return TensorRange{}, fmt.Errorf("layer-0 checkpoint is closed")
	}
	tensor, ok := checkpoint.tensors[name]
	if !ok {
		return TensorRange{}, fmt.Errorf("tensor is not in the closed layer-0 contract: %s", name)
	}
	return tensor, nil
}

// Read reads precisely one validated tensor payload for bounded staging. It
// does not infer layouts or read a neighboring range.
func (checkpoint *Layer0Checkpoint) Read(name string) ([]byte, error) {
	tensor, err := checkpoint.Tensor(name)
	if err != nil {
		return nil, err
	}
	if tensor.Bytes > uint64(^uint(0)>>1) {
		return nil, fmt.Errorf("tensor is too large for host staging: %s", name)
	}
	bytes := make([]byte, int(tensor.Bytes))
	if _, err := checkpoint.file.ReadAt(bytes, int64(tensor.FileRange[0])); err != nil && err != io.EOF {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	return bytes, nil
}

// ReadRows reads exact contiguous BF16 rows from a validated rank-two tensor.
// It is the bounded embedding/PLE ingress path and rejects invalid token IDs.
func (checkpoint *Layer0Checkpoint) ReadRows(name string, rows []uint32) ([]byte, error) {
	tensor, err := checkpoint.Tensor(name)
	if err != nil {
		return nil, err
	}
	if len(tensor.Shape) != 2 || tensor.DType != "BF16" || tensor.Shape[1] > ^uint64(0)/2 {
		return nil, fmt.Errorf("tensor does not support BF16 row reads: %s", name)
	}
	rowBytes := tensor.Shape[1] * 2
	if uint64(len(rows)) > ^uint64(0)/rowBytes || uint64(len(rows))*rowBytes > uint64(^uint(0)>>1) {
		return nil, fmt.Errorf("row read exceeds host staging capacity")
	}
	output := make([]byte, int(uint64(len(rows))*rowBytes))
	for index, row := range rows {
		if uint64(row) >= tensor.Shape[0] {
			return nil, fmt.Errorf("row index %d is outside %s", row, name)
		}
		offset := tensor.FileRange[0] + uint64(row)*rowBytes
		if offset < tensor.FileRange[0] || offset+rowBytes > tensor.FileRange[1] {
			return nil, fmt.Errorf("row range overflows validated tensor: %s", name)
		}
		start := uint64(index) * rowBytes
		if _, err := checkpoint.file.ReadAt(output[start:start+rowBytes], int64(offset)); err != nil && err != io.EOF {
			return nil, fmt.Errorf("read row %d of %s: %w", row, name, err)
		}
	}
	return output, nil
}

// Close releases the external checkpoint handle. It never deletes or changes it.
func (checkpoint *Layer0Checkpoint) Close() error {
	if checkpoint == nil || checkpoint.file == nil {
		return nil
	}
	err := checkpoint.file.Close()
	checkpoint.file = nil
	return err
}

// RequiredLayer0TensorNames returns the deterministic lexical contract list.
func RequiredLayer0TensorNames() []string {
	names := make([]string, len(layer0Required))
	for i, tensor := range layer0Required {
		names[i] = tensor.name
	}
	sort.Strings(names)
	return names
}

func sameShape(a, b []uint64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Digest is a small helper for callers recording upload evidence without
// retaining a second host copy of the source checkpoint.
func Digest(bytes []byte) string { sum := sha256.Sum256(bytes); return hex.EncodeToString(sum[:]) }
