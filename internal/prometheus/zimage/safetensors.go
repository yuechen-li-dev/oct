// Package zimage provides the bounded, dependency-free safetensors inspection
// seam used by the EVT-2 importer preflight. It deliberately parses metadata
// and validates ranges without materializing tensor payloads.
package zimage

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const MaxHeaderBytes uint64 = 64 << 20

type Tensor struct {
	Name        string    `json:"name"`
	DType       string    `json:"dtype"`
	Shape       []uint64  `json:"shape"`
	DataOffsets [2]uint64 `json:"data_offsets"`
	Elements    uint64    `json:"element_count"`
	Bytes       uint64    `json:"byte_count"`
	FileRange   [2]uint64 `json:"file_byte_range"`
}

type Manifest struct {
	Format        string            `json:"format"`
	Path          string            `json:"path"`
	FileBytes     uint64            `json:"file_bytes"`
	HeaderBytes   uint64            `json:"header_bytes"`
	HeaderSHA256  string            `json:"header_sha256"`
	PayloadOffset uint64            `json:"payload_offset"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Tensors       []Tensor          `json:"tensors"`
	DTypeBytes    map[string]uint64 `json:"dtype_bytes"`
	PayloadBytes  uint64            `json:"payload_bytes"`
}

type rawTensor struct {
	DType       string    `json:"dtype"`
	Shape       []uint64  `json:"shape"`
	DataOffsets [2]uint64 `json:"data_offsets"`
}

var dtypeBytes = map[string]uint64{
	"BOOL": 1, "F8_E4M3FN": 1, "F8_E4M3FNUZ": 1, "F8_E5M2": 1, "F8_E5M2FNUZ": 1,
	"I8": 1, "U8": 1, "F16": 2, "BF16": 2, "I16": 2, "U16": 2,
	"F32": 4, "I32": 4, "U32": 4, "F64": 8, "I64": 8, "U64": 8,
}

func checkedAdd(a, b uint64) (uint64, error) {
	c := a + b
	if c < a {
		return 0, fmt.Errorf("uint64 addition overflow")
	}
	return c, nil
}

func checkedMul(a, b uint64) (uint64, error) {
	if a != 0 && b > ^uint64(0)/a {
		return 0, fmt.Errorf("uint64 multiplication overflow")
	}
	return a * b, nil
}

// ReadManifest reads just the safetensors header and validates every payload
// extent against the actual file. It rejects duplicate keys, unsupported dtypes,
// offset overflows, malformed ranges, and overlapping tensors.
func ReadManifest(path, displayPath string) (Manifest, error) {
	var result Manifest
	f, err := os.Open(path)
	if err != nil {
		return result, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return result, err
	}
	if info.Size() < 8 {
		return result, fmt.Errorf("safetensors file is smaller than its header length")
	}
	var prefix [8]byte
	if _, err := io.ReadFull(f, prefix[:]); err != nil {
		return result, err
	}
	headerBytes := binary.LittleEndian.Uint64(prefix[:])
	if headerBytes == 0 || headerBytes > MaxHeaderBytes {
		return result, fmt.Errorf("invalid safetensors header size %d", headerBytes)
	}
	payloadOffset, err := checkedAdd(8, headerBytes)
	if err != nil {
		return result, fmt.Errorf("payload offset: %w", err)
	}
	if payloadOffset > uint64(info.Size()) {
		return result, fmt.Errorf("header extends beyond file")
	}
	header := make([]byte, headerBytes)
	if _, err := io.ReadFull(f, header); err != nil {
		return result, err
	}

	decoder := json.NewDecoder(bytesReader(header))
	open, err := decoder.Token()
	if err != nil || open != json.Delim('{') {
		return result, fmt.Errorf("safetensors header must be one JSON object")
	}
	seen := map[string]bool{}
	metadata := map[string]string{}
	var tensors []Tensor
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return result, fmt.Errorf("header key: %w", err)
		}
		name, ok := token.(string)
		if !ok {
			return result, fmt.Errorf("header key is not a string")
		}
		if seen[name] {
			return result, fmt.Errorf("duplicate tensor/header key %q", name)
		}
		seen[name] = true
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return result, fmt.Errorf("header value %q: %w", name, err)
		}
		if name == "__metadata__" {
			if err := json.Unmarshal(raw, &metadata); err != nil {
				return result, fmt.Errorf("metadata: %w", err)
			}
			continue
		}
		var value rawTensor
		if err := json.Unmarshal(raw, &value); err != nil {
			return result, fmt.Errorf("tensor %q: %w", name, err)
		}
		item, err := validateTensor(name, value, payloadOffset, uint64(info.Size())-payloadOffset)
		if err != nil {
			return result, err
		}
		tensors = append(tensors, item)
	}
	close, err := decoder.Token()
	if err != nil || close != json.Delim('}') {
		return result, fmt.Errorf("unterminated safetensors header object")
	}
	if decoder.More() {
		return result, fmt.Errorf("unexpected trailing JSON token")
	}
	sort.Slice(tensors, func(i, j int) bool { return tensors[i].Name < tensors[j].Name })
	byOffset := append([]Tensor(nil), tensors...)
	sort.Slice(byOffset, func(i, j int) bool { return byOffset[i].DataOffsets[0] < byOffset[j].DataOffsets[0] })
	for i := 1; i < len(byOffset); i++ {
		if byOffset[i-1].DataOffsets[1] > byOffset[i].DataOffsets[0] {
			return result, fmt.Errorf("overlapping tensor ranges %q and %q", byOffset[i-1].Name, byOffset[i].Name)
		}
	}
	dtypeTotals := map[string]uint64{}
	var payloadBytes uint64
	for _, item := range tensors {
		var addErr error
		dtypeTotals[item.DType], addErr = checkedAdd(dtypeTotals[item.DType], item.Bytes)
		if addErr != nil {
			return result, fmt.Errorf("dtype byte total: %w", addErr)
		}
		payloadBytes, addErr = checkedAdd(payloadBytes, item.Bytes)
		if addErr != nil {
			return result, fmt.Errorf("payload byte total: %w", addErr)
		}
	}
	hash := sha256.Sum256(header)
	return Manifest{Format: "safetensors", Path: displayPath, FileBytes: uint64(info.Size()), HeaderBytes: headerBytes, HeaderSHA256: hex.EncodeToString(hash[:]), PayloadOffset: payloadOffset, Metadata: metadata, Tensors: tensors, DTypeBytes: dtypeTotals, PayloadBytes: payloadBytes}, nil
}

func validateTensor(name string, value rawTensor, payloadOffset, payloadBytes uint64) (Tensor, error) {
	width, ok := dtypeBytes[value.DType]
	if !ok {
		return Tensor{}, fmt.Errorf("tensor %q has unsupported dtype %q", name, value.DType)
	}
	if value.DataOffsets[1] < value.DataOffsets[0] {
		return Tensor{}, fmt.Errorf("tensor %q has inverted byte range", name)
	}
	if value.DataOffsets[1] > payloadBytes {
		return Tensor{}, fmt.Errorf("tensor %q extends beyond payload", name)
	}
	elements := uint64(1)
	for _, dim := range value.Shape {
		var err error
		elements, err = checkedMul(elements, dim)
		if err != nil {
			return Tensor{}, fmt.Errorf("tensor %q shape: %w", name, err)
		}
	}
	bytes, err := checkedMul(elements, width)
	if err != nil {
		return Tensor{}, fmt.Errorf("tensor %q bytes: %w", name, err)
	}
	if bytes != value.DataOffsets[1]-value.DataOffsets[0] {
		return Tensor{}, fmt.Errorf("tensor %q extent %d does not match shape/dtype bytes %d", name, value.DataOffsets[1]-value.DataOffsets[0], bytes)
	}
	start, err := checkedAdd(payloadOffset, value.DataOffsets[0])
	if err != nil {
		return Tensor{}, fmt.Errorf("tensor %q start: %w", name, err)
	}
	end, err := checkedAdd(payloadOffset, value.DataOffsets[1])
	if err != nil {
		return Tensor{}, fmt.Errorf("tensor %q end: %w", name, err)
	}
	return Tensor{Name: name, DType: value.DType, Shape: value.Shape, DataOffsets: value.DataOffsets, Elements: elements, Bytes: bytes, FileRange: [2]uint64{start, end}}, nil
}

// HashFile is deliberately streaming so model identity checks do not create a
// second full checkpoint allocation.
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func VerifySHA256(path, expected string) error {
	if len(expected) != 64 {
		return fmt.Errorf("expected SHA-256 must be 64 lowercase hexadecimal characters")
	}
	if expected != strings.ToLower(expected) {
		return fmt.Errorf("expected SHA-256 must use lowercase hexadecimal")
	}
	if _, err := hex.DecodeString(expected); err != nil {
		return fmt.Errorf("expected SHA-256: %w", err)
	}
	actual, err := HashFile(path)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("model identity mismatch: got %s, want %s", actual, expected)
	}
	return nil
}

// ValidateDisplayPath rejects absolute and escaping paths so a committed
// artifact cannot accidentally disclose a user-specific machine path.
func ValidateDisplayPath(path string) error {
	normalized := filepath.ToSlash(path)
	if path == "" || filepath.IsAbs(path) || filepath.VolumeName(path) != "" || strings.HasPrefix(normalized, "/") {
		return fmt.Errorf("display path must be a non-empty relative path")
	}
	for _, part := range strings.FieldsFunc(normalized, func(r rune) bool { return r == '/' }) {
		if part == ".." {
			return fmt.Errorf("display path must not escape its cache root")
		}
	}
	return nil
}
