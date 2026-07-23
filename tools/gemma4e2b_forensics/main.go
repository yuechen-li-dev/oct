// Command gemma4e2b_forensics creates and validates the payload-free authority
// record for google/gemma-4-E2B-it. It deliberately reads safetensors headers
// and streams hashes; it never materializes a tensor payload.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/yuechen-li-dev/oct/internal/prometheus/zimage"
)

const (
	repository = "google/gemma-4-E2B-it"
	revision   = "3e22461f65e89153144f8adb70e3b8c2cc9845a7"
	modelSHA   = "2db5482b20d746879bb3ef79b5203e9075a2e2b98f54ec7c2f281c1477ddc550"
)

type fileAuthority struct {
	Path   string `json:"path"`
	Role   string `json:"role"`
	Bytes  uint64 `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type tensorAuthority struct {
	Name   string    `json:"name"`
	DType  string    `json:"dtype"`
	Shape  []uint64  `json:"shape"`
	Bytes  uint64    `json:"bytes"`
	Offset [2]uint64 `json:"data_offsets"`
}

type safetensorsAuthority struct {
	Path                string            `json:"path"`
	Bytes               uint64            `json:"bytes"`
	SHA256              string            `json:"sha256"`
	HeaderBytes         uint64            `json:"header_bytes"`
	HeaderSHA256        string            `json:"header_sha256"`
	PayloadOffset       uint64            `json:"payload_offset"`
	LogicalPayloadBytes uint64            `json:"logical_payload_bytes"`
	DTypeBytes          map[string]uint64 `json:"dtype_bytes"`
	TensorCount         int               `json:"tensor_count"`
	Tensors             []tensorAuthority `json:"tensors"`
}

type authority struct {
	Schema      string               `json:"schema"`
	Repository  string               `json:"repository"`
	Revision    string               `json:"revision"`
	Files       []fileAuthority      `json:"files"`
	Safetensors safetensorsAuthority `json:"safetensors"`
}

type observedFile struct {
	Path   string `json:"path"`
	Bytes  uint64 `json:"bytes"`
	SHA256 string `json:"sha256"`
}

type validation struct {
	Schema        string         `json:"schema"`
	Authority     string         `json:"authority"`
	Root          string         `json:"root"`
	Valid         bool           `json:"valid"`
	Failures      []string       `json:"failures"`
	ObservedFiles []observedFile `json:"observed_files"`
}

var officialFiles = []fileAuthority{
	{Path: ".gitattributes", Role: "repository LFS rules", Bytes: 1570, SHA256: "34448b82c17d60fec9b65b1f093c115ddbaadc04beb1b0140b6bfed2e012a930"},
	{Path: "README.md", Role: "official model card", Bytes: 27955, SHA256: "3e0608a6be80e3eb040a5aa6c76707809503deb85be6360184a118fb6156526c"},
	{Path: "chat_template.jinja", Role: "instruction and thinking template", Bytes: 18569, SHA256: "0a2c8073c878ab1da004bee933a998606537bbb62016310352c7285c3f01c5b5"},
	{Path: "config.json", Role: "model architecture", Bytes: 4954, SHA256: "1b28f3d2c3100f6c594754b81107428bd7b822a7f48272ca681dae9d2ec38330"},
	{Path: "generation_config.json", Role: "generation defaults", Bytes: 208, SHA256: "d4226bbe3117d2d253ba4609720ba82c6c4ce4627a9a6ae05387c78983ac03de"},
	{Path: "model.safetensors", Role: "BF16 checkpoint", Bytes: 10246621918, SHA256: modelSHA},
	{Path: "processor_config.json", Role: "multimodal processor", Bytes: 1689, SHA256: "32bdf45d2ad4cc29a0822ddd157a182de76644f0419a6228d151495256e9813c"},
	{Path: "tokenizer.json", Role: "tokenizer vocabulary and merges", Bytes: 32169626, SHA256: "cc8d3a0ce36466ccc1278bf987df5f71db1719b9ca6b4118264f45cb627bfe0f"},
	{Path: "tokenizer_config.json", Role: "tokenizer and response surfaces", Bytes: 3082, SHA256: "9f4fec4b1dc6ecddf8f4a92e9caea5971c0e67d81309f3f9066a2bee8c362633"},
}

func main() {
	root := flag.String("root", "", "local Hugging Face snapshot root")
	authorityPath := flag.String("authority", "", "committed authority JSON to validate")
	writeAuthority := flag.String("write-authority", "", "write a payload-free authority JSON from the verified model file")
	out := flag.String("out", "", "write validation JSON")
	flag.Parse()
	if *root == "" || (*authorityPath == "" && *writeAuthority == "") {
		fmt.Fprintln(os.Stderr, "usage: gemma4e2b_forensics -root <snapshot> (-authority <authority.json> | -write-authority <authority.json>) [-out <validation.json>]")
		os.Exit(2)
	}
	if *writeAuthority != "" {
		if err := write(*root, *writeAuthority); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := validate(*root, *authorityPath, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func write(root, output string) error {
	manifest, err := zimage.ReadManifest(filepath.Join(root, "model.safetensors"), "model.safetensors")
	if err != nil {
		return fmt.Errorf("read model.safetensors: %w", err)
	}
	if err := zimage.VerifySHA256(filepath.Join(root, "model.safetensors"), modelSHA); err != nil {
		return err
	}
	tensors := make([]tensorAuthority, len(manifest.Tensors))
	for i, t := range manifest.Tensors {
		tensors[i] = tensorAuthority{t.Name, t.DType, t.Shape, t.Bytes, t.DataOffsets}
	}
	record := authority{Schema: "oct.prometheus.g4-e2b.checkpoint-authority.v1", Repository: repository, Revision: revision, Files: officialFiles,
		Safetensors: safetensorsAuthority{Path: "model.safetensors", Bytes: manifest.FileBytes, SHA256: modelSHA, HeaderBytes: manifest.HeaderBytes, HeaderSHA256: manifest.HeaderSHA256, PayloadOffset: manifest.PayloadOffset, LogicalPayloadBytes: manifest.PayloadBytes, DTypeBytes: manifest.DTypeBytes, TensorCount: len(tensors), Tensors: tensors}}
	return writeJSON(output, record)
}

func validate(root, authorityPath, output string) error {
	b, err := os.ReadFile(authorityPath)
	if err != nil {
		return err
	}
	var expected authority
	if err := json.Unmarshal(b, &expected); err != nil {
		return fmt.Errorf("decode authority: %w", err)
	}
	result := validation{Schema: "oct.prometheus.g4-e2b.checkpoint-validation.v1", Authority: filepath.ToSlash(authorityPath), Root: "local-model-cache/gemma-4-E2B-it"}
	if expected.Schema != "oct.prometheus.g4-e2b.checkpoint-authority.v1" || expected.Repository != repository || expected.Revision != revision {
		result.Failures = append(result.Failures, "authority identity is not the pinned google/gemma-4-E2B-it revision")
	}
	paths := map[string]fileAuthority{}
	for _, f := range expected.Files {
		paths[f.Path] = f
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			result.Failures = append(result.Failures, "unexpected directory "+entry.Name())
			continue
		}
		path := entry.Name()
		info, statErr := entry.Info()
		if statErr != nil {
			result.Failures = append(result.Failures, statErr.Error())
			continue
		}
		hash, hashErr := fileSHA256(filepath.Join(root, path))
		if hashErr != nil {
			result.Failures = append(result.Failures, hashErr.Error())
			continue
		}
		result.ObservedFiles = append(result.ObservedFiles, observedFile{path, uint64(info.Size()), hash})
		want, known := paths[path]
		if !known {
			result.Failures = append(result.Failures, "unexpected or renamed object "+path)
			continue
		}
		if uint64(info.Size()) != want.Bytes {
			result.Failures = append(result.Failures, fmt.Sprintf("object %s has %d bytes; want %d", path, info.Size(), want.Bytes))
		}
		if hash != want.SHA256 {
			result.Failures = append(result.Failures, fmt.Sprintf("object %s SHA-256 mismatch: got %s want %s", path, hash, want.SHA256))
		}
	}
	for path := range paths {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			result.Failures = append(result.Failures, "missing required object "+path)
		}
	}
	manifest, manifestErr := zimage.ReadManifest(filepath.Join(root, expected.Safetensors.Path), expected.Safetensors.Path)
	if manifestErr != nil {
		result.Failures = append(result.Failures, "safetensors structural validation: "+manifestErr.Error())
	} else {
		compareManifest(&result, manifest, expected.Safetensors)
	}
	sort.Strings(result.Failures)
	sort.Slice(result.ObservedFiles, func(i, j int) bool { return result.ObservedFiles[i].Path < result.ObservedFiles[j].Path })
	result.Valid = len(result.Failures) == 0
	if output != "" {
		if err := writeJSON(output, result); err != nil {
			return err
		}
	}
	if !result.Valid {
		return fmt.Errorf("checkpoint validation failed with %d issue(s)", len(result.Failures))
	}
	return nil
}

func compareManifest(result *validation, got zimage.Manifest, want safetensorsAuthority) {
	if got.FileBytes != want.Bytes || got.HeaderBytes != want.HeaderBytes || got.HeaderSHA256 != want.HeaderSHA256 || got.PayloadOffset != want.PayloadOffset || got.PayloadBytes != want.LogicalPayloadBytes || len(got.Tensors) != want.TensorCount {
		result.Failures = append(result.Failures, "safetensors physical/header identity mismatch")
	}
	if len(got.Tensors) != len(want.Tensors) {
		return
	}
	for i, t := range got.Tensors {
		w := want.Tensors[i]
		if t.Name != w.Name || t.DType != w.DType || t.Bytes != w.Bytes || t.DataOffsets != w.Offset || !sameShape(t.Shape, w.Shape) {
			result.Failures = append(result.Failures, "tensor authority mismatch at sorted index "+fmt.Sprint(i))
			return
		}
	}
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
func fileSHA256(path string) (string, error) {
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
func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
