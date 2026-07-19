package zimage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

const (
	NoiseRefiner0SourceCheckpointSHA256 = "2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6"
	NoiseRefiner0CacheAggregateSHA256   = "a1ba526898a2a7522b31167c6d5e1bc48c39a8708cf5c3ad88b193e536ca5d5e"
	NoiseRefiner0OracleRevision         = "f332072aa78be7aecdf3ee76d5c247082da564a6"
	NoiseRefiner0FP16ReferenceSHA256    = "7e1e6d3d802402a5f0055b6fb257ef04a57cff8166b5925dcce8f9a235f281a7"
	NoiseRefiner0StageProjectionsSHA256 = "f9350d37b46a26d132d4a1e6c80c984ebce87f6f3fe4fd9eb274ffbfd631f480"
	NoiseRefiner0PayloadGuide           = "See docs/EVT2_LOCAL_PAYLOADS.md"
)

type NoiseRefiner0PayloadPaths struct {
	// CacheRoot is the directory that directly contains layers/. It is not the
	// block directory itself; retaining that distinction makes cache-relative
	// identities stable across machines.
	CacheRoot string
	// OracleRoot is the revision directory that directly contains run_02/ and
	// m075/. It is intentionally local and must never be committed as payload.
	OracleRoot string
}

// NoiseRefiner0PayloadPathsFromEnvironment resolves the deliberately local
// EVT-2 authority. It does not guess another cache location or fall back to a
// downloaded checkpoint: an unset variable is ordinary setup and the error
// points directly to the repository-owned setup guide.
func NoiseRefiner0PayloadPathsFromEnvironment() (NoiseRefiner0PayloadPaths, error) {
	cacheRoot := os.Getenv("OCT_EVT2_CACHE")
	if cacheRoot == "" {
		return NoiseRefiner0PayloadPaths{}, fmt.Errorf("OCT_EVT2_CACHE is unset; %s", NoiseRefiner0PayloadGuide)
	}
	oracleRoot := os.Getenv("OCT_EVT2_ORACLE")
	if oracleRoot == "" {
		return NoiseRefiner0PayloadPaths{}, fmt.Errorf("OCT_EVT2_ORACLE is unset; %s", NoiseRefiner0PayloadGuide)
	}
	return NoiseRefiner0PayloadPaths{CacheRoot: cacheRoot, OracleRoot: oracleRoot}, nil
}

type NoiseRefiner0Payload struct {
	Role   string
	Path   string
	Bytes  uint64
	SHA256 string
	DType  string
	Shape  []uint64
}

// NoiseRefiner0PayloadBundle is the immutable local authority consumed by the
// compiled M1 module. It deliberately holds identities and paths only: callers
// retain bounded file ownership and do not materialize the full block on host.
type NoiseRefiner0PayloadBundle struct {
	CacheRoot       string
	OracleRoot      string
	CacheBlockPath  string
	CacheManifest   CacheManifest
	CacheBytes      uint64
	Input           NoiseRefiner0Payload
	Timestep        NoiseRefiner0Payload
	OfficialBF16Out NoiseRefiner0Payload
	FP16Reference   NoiseRefiner0Payload
	StageNames      []string
}

type noiseRefiner0Capture struct {
	Artifacts map[string]struct {
		ByteCount uint64   `json:"byte_count"`
		DType     string   `json:"dtype"`
		SHA256    string   `json:"sha256"`
		Shape     []uint64 `json:"shape"`
	} `json:"artifacts"`
	InternalStageSummaries map[string]json.RawMessage `json:"internal_stage_summaries"`
}

func noiseRefiner0CacheBlockPath(root string) string {
	return filepath.Join(root, "layers", NoiseRefiner0SourceCheckpointSHA256, "noise_refiner.0")
}

func hashReader(reader io.Reader) (string, error) {
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func noiseRefiner0Aggregate(manifest CacheManifest) (string, error) {
	if manifest.TransformID != NoiseRefiner0TransformID {
		return "", fmt.Errorf("noise_refiner.0 cache transform mismatch: got %q, want %q", manifest.TransformID, NoiseRefiner0TransformID)
	}
	if manifest.SourceCheckpointSHA256 != NoiseRefiner0SourceCheckpointSHA256 {
		return "", fmt.Errorf("noise_refiner.0 cache checkpoint mismatch: got %s, want %s", manifest.SourceCheckpointSHA256, NoiseRefiner0SourceCheckpointSHA256)
	}
	items := append([]CacheTensor(nil), manifest.Tensors...)
	sort.Slice(items, func(i, j int) bool { return items[i].SourceName < items[j].SourceName })
	hash := sha256.New()
	_, _ = io.WriteString(hash, manifest.TransformID+"\n"+manifest.SourceCheckpointSHA256+"\n")
	for _, item := range items {
		_, _ = io.WriteString(hash, item.SourceName+"\n"+item.SourceSHA256+"\n"+item.SHA256+"\n")
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func noiseRefiner0Payload(path, role, dtype string, shape []uint64, expectedBytes uint64, expectedSHA256 string) (NoiseRefiner0Payload, error) {
	info, err := os.Stat(path)
	if err != nil {
		return NoiseRefiner0Payload{}, fmt.Errorf("noise_refiner.0 %s: %w", role, err)
	}
	if uint64(info.Size()) != expectedBytes {
		return NoiseRefiner0Payload{}, fmt.Errorf("noise_refiner.0 %s byte count mismatch: got %d, want %d", role, info.Size(), expectedBytes)
	}
	file, err := os.Open(path)
	if err != nil {
		return NoiseRefiner0Payload{}, fmt.Errorf("noise_refiner.0 %s open: %w", role, err)
	}
	defer file.Close()
	actual, err := hashReader(file)
	if err != nil {
		return NoiseRefiner0Payload{}, fmt.Errorf("noise_refiner.0 %s hash: %w", role, err)
	}
	if actual != expectedSHA256 {
		return NoiseRefiner0Payload{}, fmt.Errorf("noise_refiner.0 %s identity mismatch: got %s, want %s", role, actual, expectedSHA256)
	}
	return NoiseRefiner0Payload{Role: role, Path: path, Bytes: expectedBytes, SHA256: actual, DType: dtype, Shape: append([]uint64(nil), shape...)}, nil
}

// LoadNoiseRefiner0PayloadBundle validates the exact local M0.5/M0.75 payload
// bundle. It performs only bounded streaming reads, validates all thirteen
// cache tensors, and never falls back to a checkpoint or a regenerated oracle.
func LoadNoiseRefiner0PayloadBundle(paths NoiseRefiner0PayloadPaths) (bundle NoiseRefiner0PayloadBundle, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("%w; %s", err, NoiseRefiner0PayloadGuide)
		}
	}()
	if paths.CacheRoot == "" {
		return NoiseRefiner0PayloadBundle{}, fmt.Errorf("OCT_EVT2_CACHE is unset")
	}
	if paths.OracleRoot == "" {
		return NoiseRefiner0PayloadBundle{}, fmt.Errorf("OCT_EVT2_ORACLE is unset")
	}
	cacheInfo, statErr := os.Stat(paths.CacheRoot)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return NoiseRefiner0PayloadBundle{}, fmt.Errorf("OCT_EVT2_CACHE directory absent: %s", paths.CacheRoot)
		}
		return NoiseRefiner0PayloadBundle{}, fmt.Errorf("OCT_EVT2_CACHE directory check: %w", statErr)
	}
	if !cacheInfo.IsDir() {
		return NoiseRefiner0PayloadBundle{}, fmt.Errorf("OCT_EVT2_CACHE is not a directory: %s", paths.CacheRoot)
	}
	oracleInfo, statErr := os.Stat(paths.OracleRoot)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return NoiseRefiner0PayloadBundle{}, fmt.Errorf("OCT_EVT2_ORACLE directory absent: %s", paths.OracleRoot)
		}
		return NoiseRefiner0PayloadBundle{}, fmt.Errorf("OCT_EVT2_ORACLE directory check: %w", statErr)
	}
	if !oracleInfo.IsDir() {
		return NoiseRefiner0PayloadBundle{}, fmt.Errorf("OCT_EVT2_ORACLE is not a directory: %s", paths.OracleRoot)
	}
	block := noiseRefiner0CacheBlockPath(paths.CacheRoot)
	manifestPath := filepath.Join(block, "manifest.json")
	encoded, err := os.ReadFile(manifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return NoiseRefiner0PayloadBundle{}, fmt.Errorf("noise_refiner.0 cache manifest absent: %s", manifestPath)
		}
		return NoiseRefiner0PayloadBundle{}, fmt.Errorf("noise_refiner.0 cache manifest: %w", err)
	}
	var manifest CacheManifest
	if err := json.Unmarshal(encoded, &manifest); err != nil {
		return NoiseRefiner0PayloadBundle{}, fmt.Errorf("noise_refiner.0 cache manifest decode: %w", err)
	}
	if manifest.Schema != "oct.prometheus.evt2m075.fp16-cache.v1" || manifest.Block != "noise_refiner.0" || manifest.DType != "FP16 little-endian IEEE-754" {
		return NoiseRefiner0PayloadBundle{}, fmt.Errorf("noise_refiner.0 cache manifest contract mismatch")
	}
	if len(manifest.Tensors) != len(noiseRefiner0Specs) {
		return NoiseRefiner0PayloadBundle{}, fmt.Errorf("noise_refiner.0 cache tensor count mismatch: got %d, want %d", len(manifest.Tensors), len(noiseRefiner0Specs))
	}
	aggregate, err := noiseRefiner0Aggregate(manifest)
	if err != nil {
		return NoiseRefiner0PayloadBundle{}, err
	}
	if manifest.AggregateSHA256 != NoiseRefiner0CacheAggregateSHA256 || aggregate != NoiseRefiner0CacheAggregateSHA256 {
		return NoiseRefiner0PayloadBundle{}, fmt.Errorf("noise_refiner.0 cache aggregate mismatch: got %s, want %s", aggregate, NoiseRefiner0CacheAggregateSHA256)
	}
	byName := make(map[string]CacheTensor, len(manifest.Tensors))
	for _, tensor := range manifest.Tensors {
		if _, exists := byName[tensor.SourceName]; exists {
			return NoiseRefiner0PayloadBundle{}, fmt.Errorf("noise_refiner.0 cache has duplicate tensor %q", tensor.SourceName)
		}
		byName[tensor.SourceName] = tensor
	}
	var cacheBytes uint64
	for _, spec := range noiseRefiner0Specs {
		tensor, ok := byName[spec.name]
		wantLayout := "vector preserved"
		if spec.transpose {
			wantLayout = "row-major [in,out], transposed from PyTorch [out,in]"
		}
		if !ok || !sameShape(tensor.SourceShape, spec.shape) || tensor.Transpose != spec.transpose ||
			!sameShape(tensor.DestinationShape, destinationShape(spec)) || tensor.DestinationLayout != wantLayout {
			return NoiseRefiner0PayloadBundle{}, fmt.Errorf("noise_refiner.0 cache tensor contract mismatch for %q", spec.name)
		}
		path := filepath.Join(block, tensor.DestinationName)
		payload, err := noiseRefiner0Payload(path, tensor.DestinationName, "FP16", tensor.DestinationShape, tensor.Bytes, tensor.SHA256)
		if err != nil {
			return NoiseRefiner0PayloadBundle{}, err
		}
		cacheBytes += payload.Bytes
	}
	if cacheBytes != 361820672 {
		return NoiseRefiner0PayloadBundle{}, fmt.Errorf("noise_refiner.0 cache byte count mismatch: got %d, want %d", cacheBytes, 361820672)
	}
	capturePath := filepath.Join(paths.OracleRoot, "run_02", "capture.json")
	captureBytes, err := os.ReadFile(capturePath)
	if err != nil {
		return NoiseRefiner0PayloadBundle{}, fmt.Errorf("noise_refiner.0 oracle capture: %w", err)
	}
	var capture noiseRefiner0Capture
	if err := json.Unmarshal(captureBytes, &capture); err != nil {
		return NoiseRefiner0PayloadBundle{}, fmt.Errorf("noise_refiner.0 oracle capture decode: %w", err)
	}
	loadCaptured := func(key, role, wantDType string, wantShape []uint64, wantBytes uint64, wantSHA256 string) (NoiseRefiner0Payload, error) {
		item, ok := capture.Artifacts[key]
		if !ok {
			return NoiseRefiner0Payload{}, fmt.Errorf("noise_refiner.0 oracle missing %s metadata", key)
		}
		if item.DType != wantDType || !sameShape(item.Shape, wantShape) || item.ByteCount != wantBytes || item.SHA256 != wantSHA256 {
			return NoiseRefiner0Payload{}, fmt.Errorf("noise_refiner.0 %s metadata contract mismatch", role)
		}
		return noiseRefiner0Payload(filepath.Join(paths.OracleRoot, "run_02", key+".bin"), role, wantDType, wantShape, wantBytes, wantSHA256)
	}
	input, err := loadCaptured(
		"noise_refiner_0_input", "block input", "bfloat16", []uint64{1, 1024, 3840}, 7864320,
		"857cea75e69d665c43779c9bc860796e76ac8b78c5c70882e02a04940e78fded",
	)
	if err != nil {
		return NoiseRefiner0PayloadBundle{}, err
	}
	timestep, err := loadCaptured(
		"noise_refiner_0_timestep", "timestep", "bfloat16", []uint64{1, 256}, 512,
		"bc0ba90e94f5ae98779c6f7c44e7d1346f8aa6aa1cc048f62a748d96076823b2",
	)
	if err != nil {
		return NoiseRefiner0PayloadBundle{}, err
	}
	official, err := loadCaptured(
		"noise_refiner_0_output", "official BF16 output", "bfloat16", []uint64{1, 1024, 3840}, 7864320,
		"6dae8d91b2118e7c425ee16d5db214ec0d8df1e988487e855aebd1fe81575873",
	)
	if err != nil {
		return NoiseRefiner0PayloadBundle{}, err
	}
	fp16, err := noiseRefiner0Payload(filepath.Join(paths.OracleRoot, "m075", "noise_refiner_0_fp16_weight_output.bin"), "FP16-weight FP32-compute output", "float32", []uint64{1, 1024, 3840}, 15728640, NoiseRefiner0FP16ReferenceSHA256)
	if err != nil {
		return NoiseRefiner0PayloadBundle{}, err
	}
	stageNames := make([]string, 0, len(capture.InternalStageSummaries))
	for name := range capture.InternalStageSummaries {
		stageNames = append(stageNames, name)
	}
	sort.Strings(stageNames)
	if len(stageNames) == 0 {
		return NoiseRefiner0PayloadBundle{}, fmt.Errorf("noise_refiner.0 oracle has no internal stage witnesses")
	}
	return NoiseRefiner0PayloadBundle{CacheRoot: paths.CacheRoot, OracleRoot: paths.OracleRoot, CacheBlockPath: block, CacheManifest: manifest, CacheBytes: cacheBytes, Input: input, Timestep: timestep, OfficialBF16Out: official, FP16Reference: fp16, StageNames: stageNames}, nil
}
