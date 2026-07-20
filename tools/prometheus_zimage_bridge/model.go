package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/prometheus/zimage"
)

const (
	bridgeABIVersion       = uint32(4)
	acceptedLockSHA256     = "71ef202b4e34b562bd0d8526d1e0c674640cbba02fb7c484d8dadf981c8b226e"
	checkpointSHA256       = "2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6"
	imageTokens            = uint32(1024)
	contextTokens          = uint32(32)
	jointTokens            = imageTokens + contextTokens
	modelWidth             = uint32(3840)
	timestepWidth          = uint32(256)
	imageBF16Bytes         = uint64(imageTokens) * uint64(modelWidth) * 2
	contextFP32Bytes       = uint64(contextTokens) * uint64(modelWidth) * 4
	timestepBF16Bytes      = uint64(timestepWidth) * 2
	jointFP32Bytes         = uint64(jointTokens) * uint64(modelWidth) * 4
	modelAllocationCeiling = uint64(643587076)
)

var (
	modulatedSuffixes = []string{
		".adaLN_modulation.0.bias",
		".adaLN_modulation.0.weight",
		".attention.k_norm.weight",
		".attention.out.weight",
		".attention.q_norm.weight",
		".attention.qkv.weight",
		".attention_norm1.weight",
		".attention_norm2.weight",
		".feed_forward.w1.weight",
		".feed_forward.w2.weight",
		".feed_forward.w3.weight",
		".ffn_norm1.weight",
		".ffn_norm2.weight",
	}
	modulatedBytes = []uint64{
		30720, 7864320, 256, 29491200, 256, 88473600, 7680,
		7680, 78643200, 78643200, 78643200, 7680, 7680,
	}
	contextSuffixes = []string{
		".attention.k_norm.weight",
		".attention.out.weight",
		".attention.q_norm.weight",
		".attention.qkv.weight",
		".attention_norm1.weight",
		".attention_norm2.weight",
		".feed_forward.w1.weight",
		".feed_forward.w2.weight",
		".feed_forward.w3.weight",
		".ffn_norm1.weight",
		".ffn_norm2.weight",
	}
	contextBytes = []uint64{
		256, 29491200, 256, 88473600, 7680, 7680,
		78643200, 78643200, 78643200, 7680, 7680,
	}
)

type payloadTensor struct {
	path            string
	byteCount       uint64
	sha256          string
	contentIdentity uint64
	layoutIdentity  uint64
}

type payloadBlock struct {
	name      string
	aggregate string
	tensors   []payloadTensor
}

type validatedPayload struct {
	root       string
	lockPath   string
	lockSHA256 string
	noise      [2]payloadBlock
	context    [2]payloadBlock
	main       [30]payloadBlock
}

type runMetrics struct {
	wallTimeNS             uint64
	modelExecutionNS       uint64
	parameterRebindNS      uint64
	uploadedWeightBytes    uint64
	allocationCeilingBytes uint64
	persistentBytes        uint64
	reusableBytes          uint64
	auditBytes             uint64
	hostPackageCacheBytes  uint64
	hostPackageCacheHits   uint64
	prefetchTransferNS     uint64
	prefetchOverlapNS      uint64
	prefetchWaitNS         uint64
	prefetchCount          uint32
	mainLayerCount         uint32
	contextReused          bool
	stageExecutionNS       [34]uint64
	stageRebindNS          [34]uint64
	stagePayloadReadNS     [34]uint64
	stageUploadedBytes     [34]uint64
}

type cacheLockRecord struct {
	block     string
	aggregate string
}

var cacheRecordPattern = regexp.MustCompile(`Parameters:\s*"([^"]+)"\s+Cache:\s*"sha256:([0-9a-f]{64})"`)

func validatePayloadRoot(lockPath, root string) (validatedPayload, error) {
	lockPath, err := filepath.Abs(lockPath)
	if err != nil {
		return validatedPayload{}, fmt.Errorf("resolve lock path: %w", err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return validatedPayload{}, fmt.Errorf("resolve payload root: %w", err)
	}
	lockData, err := os.ReadFile(lockPath)
	if err != nil {
		return validatedPayload{}, fmt.Errorf("read compiled-model lock: %w", err)
	}
	lockSum := sha256.Sum256(lockData)
	lockDigest := hex.EncodeToString(lockSum[:])
	if lockDigest != acceptedLockSHA256 {
		return validatedPayload{}, fmt.Errorf("compiled-model lock SHA-256 mismatch: got %s want %s", lockDigest, acceptedLockSHA256)
	}
	if info, statErr := os.Stat(root); statErr != nil || !info.IsDir() {
		if statErr != nil {
			return validatedPayload{}, fmt.Errorf("payload root is unavailable: %w", statErr)
		}
		return validatedPayload{}, fmt.Errorf("payload root is not a directory: %s", root)
	}

	records := make(map[string]string)
	for _, match := range cacheRecordPattern.FindAllSubmatch(lockData, -1) {
		block, aggregate := string(match[1]), string(match[2])
		if _, duplicate := records[block]; duplicate {
			return validatedPayload{}, fmt.Errorf("compiled-model lock repeats parameter block %q", block)
		}
		records[block] = aggregate
	}
	if len(records) != 34 {
		return validatedPayload{}, fmt.Errorf("compiled-model lock exposes %d payload blocks; want 34", len(records))
	}

	result := validatedPayload{root: root, lockPath: lockPath, lockSHA256: lockDigest}
	for index := 0; index < 2; index++ {
		name := fmt.Sprintf("noise_refiner.%d", index)
		block, blockErr := validatePayloadBlock(root, records, name, modulatedSuffixes, modulatedBytes)
		if blockErr != nil {
			return validatedPayload{}, blockErr
		}
		result.noise[index] = block

		name = fmt.Sprintf("context_refiner.%d", index)
		block, blockErr = validatePayloadBlock(root, records, name, contextSuffixes, contextBytes)
		if blockErr != nil {
			return validatedPayload{}, blockErr
		}
		result.context[index] = block
	}
	for index := 0; index < 30; index++ {
		name := fmt.Sprintf("layers.%d", index)
		block, blockErr := validatePayloadBlock(root, records, name, modulatedSuffixes, modulatedBytes)
		if blockErr != nil {
			return validatedPayload{}, blockErr
		}
		result.main[index] = block
	}
	return result, nil
}

func validatePayloadBlock(root string, records map[string]string, block string, suffixes []string, byteCounts []uint64) (payloadBlock, error) {
	expectedAggregate, ok := records[block]
	if !ok {
		return payloadBlock{}, fmt.Errorf("compiled-model lock does not resolve %s", block)
	}
	dir := filepath.Join(root, "layers", checkpointSHA256, block)
	manifestPath := filepath.Join(dir, "manifest.json")
	encoded, err := os.ReadFile(manifestPath)
	if err != nil {
		return payloadBlock{}, fmt.Errorf("read %s cache manifest: %w", block, err)
	}
	var manifest zimage.CacheManifest
	if err = json.Unmarshal(encoded, &manifest); err != nil {
		return payloadBlock{}, fmt.Errorf("decode %s cache manifest: %w", block, err)
	}
	if manifest.Block != block || manifest.SourceCheckpointSHA256 != checkpointSHA256 ||
		manifest.AggregateSHA256 != expectedAggregate || len(manifest.Tensors) != len(suffixes) {
		return payloadBlock{}, fmt.Errorf("%s cache manifest does not match its lock-resolved package", block)
	}
	byName := make(map[string]zimage.CacheTensor, len(manifest.Tensors))
	for _, tensor := range manifest.Tensors {
		if _, duplicate := byName[tensor.SourceName]; duplicate {
			return payloadBlock{}, fmt.Errorf("%s cache manifest repeats tensor %q", block, tensor.SourceName)
		}
		byName[tensor.SourceName] = tensor
	}
	validated := payloadBlock{name: block, aggregate: expectedAggregate, tensors: make([]payloadTensor, len(suffixes))}
	for index, suffix := range suffixes {
		name := block + suffix
		tensor, found := byName[name]
		if !found || tensor.Bytes != byteCounts[index] || tensor.SHA256 == "" || tensor.DestinationName == "" {
			return payloadBlock{}, fmt.Errorf("%s cache tensor contract mismatch for %q", block, name)
		}
		path := filepath.Join(dir, tensor.DestinationName)
		if err = requireWithin(dir, path); err != nil {
			return payloadBlock{}, fmt.Errorf("%s cache tensor path: %w", block, err)
		}
		digest, size, hashErr := hashFile(path)
		if hashErr != nil {
			return payloadBlock{}, fmt.Errorf("validate %s: %w", path, hashErr)
		}
		if uint64(size) != tensor.Bytes || digest != tensor.SHA256 {
			return payloadBlock{}, fmt.Errorf("%s cache payload identity mismatch: got bytes=%d sha256=%s", name, size, digest)
		}
		validated.tensors[index] = payloadTensor{
			path:            path,
			byteCount:       tensor.Bytes,
			sha256:          tensor.SHA256,
			contentIdentity: identityFromHex(tensor.SHA256),
			layoutIdentity:  identityFromText(name + "\n" + tensor.DestinationLayout),
		}
	}
	return validated, nil
}

func requireWithin(root, path string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return fmt.Errorf("path escapes payload directory: %s", path)
	}
	return nil
}

func hashFile(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}

func identityFromHex(value string) uint64 {
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) < 8 {
		return identityFromText(value)
	}
	identity := binary.BigEndian.Uint64(decoded[:8])
	if identity == 0 {
		return 1
	}
	return identity
}

func identityFromText(value string) uint64 {
	sum := sha256.Sum256([]byte(value))
	identity := binary.BigEndian.Uint64(sum[:8])
	if identity == 0 {
		return 1
	}
	return identity
}

func payloadBlockNames(payload validatedPayload) []string {
	names := make([]string, 0, 34)
	for _, block := range payload.noise {
		names = append(names, block.name)
	}
	for _, block := range payload.context {
		names = append(names, block.name)
	}
	for _, block := range payload.main {
		names = append(names, block.name)
	}
	sort.Strings(names)
	return names
}
