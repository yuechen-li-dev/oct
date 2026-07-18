package zimage

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const NoiseRefiner0TransformID = "zimage-noise-refiner.0-bf16-fp16-transpose-v1"

type CacheTensor struct {
	SourceName        string    `json:"source_name"`
	SourceShape       []uint64  `json:"source_shape"`
	SourceRange       [2]uint64 `json:"source_file_byte_range"`
	SourceSHA256      string    `json:"source_sha256"`
	DestinationName   string    `json:"destination_name"`
	DestinationShape  []uint64  `json:"destination_shape"`
	DestinationLayout string    `json:"destination_layout"`
	Transpose         bool      `json:"transpose"`
	Bytes             uint64    `json:"destination_bytes"`
	SHA256            string    `json:"destination_sha256"`
	Consumer          string    `json:"consumer"`
}

type CacheManifest struct {
	Schema                 string        `json:"schema"`
	TransformID            string        `json:"transform_id"`
	SourceCheckpointSHA256 string        `json:"source_checkpoint_sha256"`
	Block                  string        `json:"block"`
	DType                  string        `json:"dtype"`
	AggregateSHA256        string        `json:"aggregate_sha256"`
	Tensors                []CacheTensor `json:"tensors"`
}

type cacheSpec struct {
	name, consumer string
	transpose      bool
	shape          []uint64
}

var noiseRefiner0Specs = []cacheSpec{
	{"noise_refiner.0.adaLN_modulation.0.bias", "AdaLN bias", false, []uint64{15360}},
	{"noise_refiner.0.adaLN_modulation.0.weight", "AdaLN projection", true, []uint64{15360, 256}},
	{"noise_refiner.0.attention.k_norm.weight", "K RMSNorm", false, []uint64{128}},
	{"noise_refiner.0.attention.out.weight", "attention output projection", true, []uint64{3840, 3840}},
	{"noise_refiner.0.attention.q_norm.weight", "Q RMSNorm", false, []uint64{128}},
	{"noise_refiner.0.attention.qkv.weight", "fused QKV projection; M1 takes Q/K/V views in Q,K,V order", true, []uint64{11520, 3840}},
	{"noise_refiner.0.attention_norm1.weight", "attention pre-norm", false, []uint64{3840}},
	{"noise_refiner.0.attention_norm2.weight", "attention post-norm", false, []uint64{3840}},
	{"noise_refiner.0.feed_forward.w1.weight", "FFN W1", true, []uint64{10240, 3840}},
	{"noise_refiner.0.feed_forward.w2.weight", "FFN W2", true, []uint64{3840, 10240}},
	{"noise_refiner.0.feed_forward.w3.weight", "FFN W3", true, []uint64{10240, 3840}},
	{"noise_refiner.0.ffn_norm1.weight", "FFN pre-norm", false, []uint64{3840}},
	{"noise_refiner.0.ffn_norm2.weight", "FFN post-norm", false, []uint64{3840}},
}

// BF16ToFP16 converts an IEEE BF16 bit pattern through its exact Float32 value
// then rounds to IEEE FP16 nearest-even. NaNs are canonicalized to quiet FP16
// NaNs (sign preserved); infinities and signed zeros are preserved.
func BF16ToFP16(bits uint16) uint16 {
	f := uint32(bits) << 16
	sign := uint16((f >> 16) & 0x8000)
	exp := int((f >> 23) & 0xff)
	mant := f & 0x7fffff
	if exp == 0xff {
		if mant == 0 {
			return sign | 0x7c00
		}
		return sign | 0x7e00
	}
	e16 := exp - 127 + 15
	if e16 >= 31 {
		return sign | 0x7c00
	}
	if e16 <= 0 {
		if e16 < -10 {
			return sign
		}
		mant |= 0x800000
		shift := uint(14 - e16)
		out := uint16(mant >> shift)
		rem := mant & ((uint32(1) << shift) - 1)
		half := uint32(1) << (shift - 1)
		if rem > half || (rem == half && (out&1) != 0) {
			out++
		}
		return sign | out
	}
	out := sign | uint16(e16<<10) | uint16(mant>>13)
	rem := mant & 0x1fff
	if rem > 0x1000 || (rem == 0x1000 && (out&1) != 0) {
		out++
	}
	return out
}

func fp16Name(name string) string { return strings.ReplaceAll(name, "/", "_") + ".fp16.bin" }

func findTensor(manifest Manifest, name string) (Tensor, bool) {
	for _, tensor := range manifest.Tensors {
		if tensor.Name == name {
			return tensor, true
		}
	}
	return Tensor{}, false
}

func sameShape(got, want []uint64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func tensorHash(file *os.File, tensor Tensor) (string, error) {
	h := sha256.New()
	if _, err := file.Seek(int64(tensor.FileRange[0]), io.SeekStart); err != nil {
		return "", err
	}
	if _, err := io.CopyN(h, file, int64(tensor.Bytes)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func destinationShape(spec cacheSpec) []uint64 {
	if !spec.transpose {
		return append([]uint64(nil), spec.shape...)
	}
	return []uint64{spec.shape[1], spec.shape[0]}
}

func convertTensor(source *os.File, tensor Tensor, spec cacheSpec) ([]byte, error) {
	if tensor.Bytes > uint64(^uint(0)>>1) {
		return nil, fmt.Errorf("%s exceeds host slice limit", tensor.Name)
	}
	input := make([]byte, tensor.Bytes)
	if _, err := source.ReadAt(input, int64(tensor.FileRange[0])); err != nil {
		return nil, err
	}
	output := make([]byte, len(input))
	write := func(index uint64, bits uint16) { binary.LittleEndian.PutUint16(output[index*2:], BF16ToFP16(bits)) }
	if !spec.transpose {
		for i := uint64(0); i < tensor.Elements; i++ {
			write(i, binary.LittleEndian.Uint16(input[i*2:]))
		}
		return output, nil
	}
	rows, cols := spec.shape[0], spec.shape[1]
	for row := uint64(0); row < rows; row++ {
		for col := uint64(0); col < cols; col++ {
			write(col*rows+row, binary.LittleEndian.Uint16(input[(row*cols+col)*2:]))
		}
	}
	return output, nil
}

func writeAtomic(path string, data []byte) error {
	temporary := path + ".tmp"
	defer os.Remove(temporary)
	if err := os.WriteFile(temporary, data, 0644); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

// BuildNoiseRefiner0Cache reads exactly thirteen source tensor ranges. It is a
// deliberately narrow M1 cache format, not a generic model importer.
func BuildNoiseRefiner0Cache(sourcePath, cacheRoot, expectedSHA256 string) (CacheManifest, error) {
	if err := VerifySHA256(sourcePath, expectedSHA256); err != nil {
		return CacheManifest{}, err
	}
	manifest, err := ReadManifest(sourcePath, "local-model-cache/z_image_turbo_bf16.safetensors")
	if err != nil {
		return CacheManifest{}, err
	}
	source, err := os.Open(sourcePath)
	if err != nil {
		return CacheManifest{}, err
	}
	defer source.Close()
	dir := filepath.Join(cacheRoot, "layers", expectedSHA256, "noise_refiner.0")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return CacheManifest{}, err
	}
	result := CacheManifest{Schema: "oct.prometheus.evt2m075.fp16-cache.v1", TransformID: NoiseRefiner0TransformID, SourceCheckpointSHA256: expectedSHA256, Block: "noise_refiner.0", DType: "FP16 little-endian IEEE-754", Tensors: make([]CacheTensor, 0, len(noiseRefiner0Specs))}
	for _, spec := range noiseRefiner0Specs {
		tensor, ok := findTensor(manifest, spec.name)
		if !ok {
			return CacheManifest{}, fmt.Errorf("required tensor %q missing", spec.name)
		}
		if tensor.DType != "BF16" || !sameShape(tensor.Shape, spec.shape) {
			return CacheManifest{}, fmt.Errorf("tensor %q contract mismatch", spec.name)
		}
		sourceHash, err := tensorHash(source, tensor)
		if err != nil {
			return CacheManifest{}, err
		}
		path := filepath.Join(dir, fp16Name(spec.name))
		// Always regenerate the bounded tensor payload before accepting a
		// resumable cache entry. This makes an interrupted or externally altered
		// file detectable without trusting a previous manifest.
		out, err := convertTensor(source, tensor, spec)
		if err != nil {
			return CacheManifest{}, err
		}
		desiredHash := sha256.Sum256(out)
		current, readErr := os.ReadFile(path)
		needsWrite := readErr != nil || uint64(len(current)) != tensor.Bytes
		if !needsWrite {
			currentHash := sha256.Sum256(current)
			needsWrite = currentHash != desiredHash
		}
		if needsWrite {
			if err = writeAtomic(path, out); err != nil {
				return CacheManifest{}, err
			}
		}
		destinationHash := desiredHash
		layout := "vector preserved"
		if spec.transpose {
			layout = "row-major [in,out], transposed from PyTorch [out,in]"
		}
		result.Tensors = append(result.Tensors, CacheTensor{spec.name, append([]uint64(nil), tensor.Shape...), tensor.FileRange, sourceHash, fp16Name(spec.name), destinationShape(spec), layout, spec.transpose, uint64(len(out)), hex.EncodeToString(destinationHash[:]), spec.consumer})
	}
	sort.Slice(result.Tensors, func(i, j int) bool { return result.Tensors[i].SourceName < result.Tensors[j].SourceName })
	h := sha256.New()
	_, _ = io.WriteString(h, result.TransformID+"\n"+result.SourceCheckpointSHA256+"\n")
	for _, t := range result.Tensors {
		_, _ = io.WriteString(h, t.SourceName+"\n"+t.SourceSHA256+"\n"+t.SHA256+"\n")
	}
	result.AggregateSHA256 = hex.EncodeToString(h.Sum(nil))
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return CacheManifest{}, err
	}
	if err = writeAtomic(filepath.Join(dir, "manifest.json"), append(encoded, '\n')); err != nil {
		return CacheManifest{}, err
	}
	return result, nil
}

func FP16ToFloat32(bits uint16) float32 {
	sign := uint32(bits&0x8000) << 16
	exp := int((bits >> 10) & 31)
	mant := uint32(bits & 0x3ff)
	if exp == 0 {
		if mant == 0 {
			return math.Float32frombits(sign)
		}
		exp = -14
		for mant&0x400 == 0 {
			mant <<= 1
			exp--
		}
		mant &= 0x3ff
	} else {
		// Normal binary16 exponents use bias 15. Convert the stored exponent
		// to its unbiased form before applying binary32's bias 127.
		exp -= 15
	}
	if exp == 31 {
		return math.Float32frombits(sign | 0x7f800000 | (mant << 13))
	}
	return math.Float32frombits(sign | (uint32(exp+127) << 23) | (mant << 13))
}
