package prometheus

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"

	"github.com/yuechen-li-dev/oct/internal/prometheus/gemma4e2b"
)

const (
	gemma4e2bM1Width           = 1536
	gemma4e2bM1Tokens          = 15
	gemma4e2bM1QColumns        = 2048
	gemma4e2bM1KColumns        = 256
	gemma4e2bM1VColumns        = 256
	gemma4e2bM1RMSNormEpsilon  = float32(1e-6)
	gemma4e2bM1ExactSourceHash = uint64(0x19dbb6c9cba24e13)
)

type gemma4e2bReferenceBoundary struct {
	Shape       []int   `json:"shape"`
	Minimum     float64 `json:"minimum"`
	Maximum     float64 `json:"maximum"`
	SHA256F32LE string  `json:"sha256_f32_le"`
}

type gemma4e2bReferenceRecord struct {
	Boundaries struct {
		Layer0InputRMSNorm gemma4e2bReferenceBoundary `json:"layer0_input_rmsnorm"`
		Layer0QLinear      gemma4e2bReferenceBoundary `json:"layer0_q_linear"`
		Layer0KLinear      gemma4e2bReferenceBoundary `json:"layer0_k_linear"`
		Layer0VLinear      gemma4e2bReferenceBoundary `json:"layer0_v_linear"`
	} `json:"boundaries"`
}

type gemma4e2bBoundaryCheck struct {
	Name            string
	Shape           []int
	ReferenceHash   string
	ActualQuantized string
	HashMatch       bool
	Minimum         float32
	Maximum         float32
	FiniteCount     int
	NaNCount        int
	InfinityCount   int
	ZeroCount       int
	NonZeroCount    int
}

type gemma4e2bCanonicalSliceResult struct {
	RuntimeEnvironment string
	RMSNorm            gemma4e2bBoundaryCheck
	Q                  gemma4e2bBoundaryCheck
	K                  gemma4e2bBoundaryCheck
	V                  gemma4e2bBoundaryCheck
	QCPUContractHash   string
	KCPUContractHash   string
	VCPUContractHash   string
	QCPUContractDiff   gemma4e2bComparison
	KCPUContractDiff   gemma4e2bComparison
	VCPUContractDiff   gemma4e2bComparison
	QCPUContractPolicy CorrectnessResult
	KCPUContractPolicy CorrectnessResult
	VCPUContractPolicy CorrectnessResult
	QRepeatedStable    bool
	KRepeatedStable    bool
	VRepeatedStable    bool
	QOperands          gemma4e2bProjectionOperandAuthority
	KOperands          gemma4e2bProjectionOperandAuthority
	VOperands          gemma4e2bProjectionOperandAuthority
	RMSNormNative      reactorGemma4E2BM1InputRMSNormResult
}

type gemma4e2bProjectionOperandAuthority struct {
	ActivationPointer      string
	ActivationShape        []int
	ActivationRowStride    int
	ActivationElementCount int
	ActivationPrecision    string
	ActivationSHA256F32LE  string
	WeightTensor           string
	WeightDType            string
	WeightLogicalShape     []uint64
	WeightBF16FileRange    [2]uint64
	WeightBF16Bytes        uint64
	WeightBF16SHA256       string
	WeightTranspose        string
	WeightSamples          [3]float32
	UploadedFP32Bytes      int
	DestinationShape       []int
	DestinationRowStride   int
	DestinationBytes       int
	DescriptorRangeBytes   int
	PushConstants          [3]int
	DispatchGroups         [3]int
	SelectedVariant        string
	PackageIdentity        string
}

type gemma4e2bComparison struct {
	MaxAbsoluteError float32
	RelativeL2       float64
	WorstIndex       int
	Actual           float32
	Reference        float32
}

func runGemma4e2bCanonicalQKVRTX(checkpointRoot string) (gemma4e2bCanonicalSliceResult, error) {
	var result gemma4e2bCanonicalSliceResult
	tempRoot := os.TempDir()
	referencePath := filepath.Join(tempRoot, "g4e2b-m1-reference.json")
	fixturePath := filepath.Join(tempRoot, "g4e2b-m1-fixtures", "base_token_embedding.bf16le.bin")
	authorityPath, err := gemma4e2bAuthorityPath()
	if err != nil {
		return result, err
	}

	reference, err := loadGemma4e2bReference(referencePath)
	if err != nil {
		return result, err
	}
	input, err := loadBF16Fixture(fixturePath, gemma4e2bM1Tokens*gemma4e2bM1Width)
	if err != nil {
		return result, err
	}
	checkpoint, err := gemma4e2b.OpenLayer0Checkpoint(checkpointRoot, authorityPath)
	if err != nil {
		return result, err
	}
	defer checkpoint.Close()

	runtime, err := newNativeRuntime()
	if err != nil {
		return result, err
	}
	defer runtime.Close()
	result.RuntimeEnvironment = runtime.Environment()

	layerNormWeight, err := decodeBF16Vector(checkpoint, "model.language_model.layers.0.input_layernorm.weight", gemma4e2bM1Width)
	if err != nil {
		return result, err
	}
	normOutput, _, nativeResult, err := runtime.Gemma4E2BM1InputRMSNorm(
		input,
		layerNormWeight,
		gemma4e2bM1Tokens,
		gemma4e2bM1Width,
		gemma4e2bM1Width,
		gemma4e2bM1RMSNormEpsilon,
		1,
		1,
		gemma4e2bM1ExactSourceHash,
	)
	result.RMSNormNative = nativeResult
	if err != nil {
		return result, err
	}
	result.RMSNorm = summarizeGemmaBoundary("layer0_input_rmsnorm", normOutput, reference.Boundaries.Layer0InputRMSNorm)

	projectionRuntime, err := newNativeRuntime()
	if err != nil {
		return result, err
	}
	defer projectionRuntime.Close()

	qWeight, err := decodeTransposedBF16Matrix(checkpoint, "model.language_model.layers.0.self_attn.q_proj.weight", gemma4e2bM1QColumns, gemma4e2bM1Width)
	if err != nil {
		return result, err
	}
	result.QOperands, err = projectionOperandAuthority(checkpoint, "model.language_model.layers.0.self_attn.q_proj.weight", normOutput, qWeight, gemma4e2bM1Tokens, gemma4e2bM1QColumns, gemma4e2bM1Width)
	if err != nil {
		return result, err
	}
	qOutput, _, err := projectionRuntime.SGEMMWithStatus(int(gemma4e2bM1Tokens), gemma4e2bM1QColumns, gemma4e2bM1Width, normOutput, qWeight)
	if err != nil {
		return result, fmt.Errorf("q projection: %w", err)
	}
	result.Q = summarizeGemmaBoundary("layer0_q_linear", qOutput, reference.Boundaries.Layer0QLinear)
	qCPU := cpuMatmulRowMajor(normOutput, qWeight, gemma4e2bM1Tokens, gemma4e2bM1QColumns, gemma4e2bM1Width)
	result.QCPUContractHash = hashQuantizedBF16AsFloat32(qCPU)
	result.QCPUContractDiff = compareVectors(qOutput, qCPU)
	result.QCPUContractPolicy = compareAgainstOracle(qCPU, qOutput)

	kWeight, err := decodeTransposedBF16Matrix(checkpoint, "model.language_model.layers.0.self_attn.k_proj.weight", gemma4e2bM1KColumns, gemma4e2bM1Width)
	if err != nil {
		return result, err
	}
	result.KOperands, err = projectionOperandAuthority(checkpoint, "model.language_model.layers.0.self_attn.k_proj.weight", normOutput, kWeight, gemma4e2bM1Tokens, gemma4e2bM1KColumns, gemma4e2bM1Width)
	if err != nil {
		return result, err
	}
	kOutput, _, err := projectionRuntime.SGEMMWithStatus(int(gemma4e2bM1Tokens), gemma4e2bM1KColumns, gemma4e2bM1Width, normOutput, kWeight)
	if err != nil {
		return result, fmt.Errorf("k projection: %w", err)
	}
	result.K = summarizeGemmaBoundary("layer0_k_linear", kOutput, reference.Boundaries.Layer0KLinear)
	kCPU := cpuMatmulRowMajor(normOutput, kWeight, gemma4e2bM1Tokens, gemma4e2bM1KColumns, gemma4e2bM1Width)
	result.KCPUContractHash = hashQuantizedBF16AsFloat32(kCPU)
	result.KCPUContractDiff = compareVectors(kOutput, kCPU)
	result.KCPUContractPolicy = compareAgainstOracle(kCPU, kOutput)

	vWeight, err := decodeTransposedBF16Matrix(checkpoint, "model.language_model.layers.0.self_attn.v_proj.weight", gemma4e2bM1VColumns, gemma4e2bM1Width)
	if err != nil {
		return result, err
	}
	result.VOperands, err = projectionOperandAuthority(checkpoint, "model.language_model.layers.0.self_attn.v_proj.weight", normOutput, vWeight, gemma4e2bM1Tokens, gemma4e2bM1VColumns, gemma4e2bM1Width)
	if err != nil {
		return result, err
	}
	vOutput, _, err := projectionRuntime.SGEMMWithStatus(int(gemma4e2bM1Tokens), gemma4e2bM1VColumns, gemma4e2bM1Width, normOutput, vWeight)
	if err != nil {
		return result, fmt.Errorf("v projection: %w", err)
	}
	result.V = summarizeGemmaBoundary("layer0_v_linear", vOutput, reference.Boundaries.Layer0VLinear)
	vCPU := cpuMatmulRowMajor(normOutput, vWeight, gemma4e2bM1Tokens, gemma4e2bM1VColumns, gemma4e2bM1Width)
	result.VCPUContractHash = hashQuantizedBF16AsFloat32(vCPU)
	result.VCPUContractDiff = compareVectors(vOutput, vCPU)
	result.VCPUContractPolicy = compareAgainstOracle(vCPU, vOutput)

	qRepeated, _, err := projectionRuntime.SGEMMWithStatus(int(gemma4e2bM1Tokens), gemma4e2bM1QColumns, gemma4e2bM1Width, normOutput, qWeight)
	if err != nil {
		return result, fmt.Errorf("repeat q projection: %w", err)
	}
	kRepeated, _, err := projectionRuntime.SGEMMWithStatus(int(gemma4e2bM1Tokens), gemma4e2bM1KColumns, gemma4e2bM1Width, normOutput, kWeight)
	if err != nil {
		return result, fmt.Errorf("repeat k projection: %w", err)
	}
	vRepeated, _, err := projectionRuntime.SGEMMWithStatus(int(gemma4e2bM1Tokens), gemma4e2bM1VColumns, gemma4e2bM1Width, normOutput, vWeight)
	if err != nil {
		return result, fmt.Errorf("repeat v projection: %w", err)
	}
	result.QRepeatedStable = equalFloat32Bits(qOutput, qRepeated)
	result.KRepeatedStable = equalFloat32Bits(kOutput, kRepeated)
	result.VRepeatedStable = equalFloat32Bits(vOutput, vRepeated)
	return result, nil
}

func gemma4e2bAuthorityPath() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("locate g4e2b authority path: runtime caller unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	return filepath.Join(root, "internal", "prometheus", "DevelopmentReport", "artifacts", "G4E2BM0", "checkpoint_authority.json"), nil
}

func loadGemma4e2bReference(path string) (gemma4e2bReferenceRecord, error) {
	var record gemma4e2bReferenceRecord
	bytes, err := os.ReadFile(path)
	if err != nil {
		return record, fmt.Errorf("read accepted g4e2b m1 reference: %w", err)
	}
	bytes = sanitizeNonFiniteJSON(bytes)
	if err := json.Unmarshal(bytes, &record); err != nil {
		return record, fmt.Errorf("decode accepted g4e2b m1 reference: %w", err)
	}
	return record, nil
}

func loadBF16Fixture(path string, elements int) ([]float32, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read accepted g4e2b fixture %s: %w", filepath.Base(path), err)
	}
	if len(bytes) != elements*2 {
		return nil, fmt.Errorf("fixture %s has %d bytes; want %d", filepath.Base(path), len(bytes), elements*2)
	}
	values := make([]float32, elements)
	for i := 0; i < elements; i++ {
		values[i] = bf16ToFloat32(binary.LittleEndian.Uint16(bytes[i*2:]))
	}
	return values, nil
}

func decodeBF16Vector(checkpoint *gemma4e2b.Layer0Checkpoint, name string, elements int) ([]float32, error) {
	bytes, err := checkpoint.Read(name)
	if err != nil {
		return nil, err
	}
	if len(bytes) != elements*2 {
		return nil, fmt.Errorf("%s has %d bytes; want %d", name, len(bytes), elements*2)
	}
	values := make([]float32, elements)
	for i := 0; i < elements; i++ {
		values[i] = bf16ToFloat32(binary.LittleEndian.Uint16(bytes[i*2:]))
	}
	return values, nil
}

func decodeTransposedBF16Matrix(checkpoint *gemma4e2b.Layer0Checkpoint, name string, rows, columns int) ([]float32, error) {
	bytes, err := checkpoint.Read(name)
	if err != nil {
		return nil, err
	}
	if len(bytes) != rows*columns*2 {
		return nil, fmt.Errorf("%s has %d bytes; want %d", name, len(bytes), rows*columns*2)
	}
	values := make([]float32, rows*columns)
	for row := 0; row < rows; row++ {
		for column := 0; column < columns; column++ {
			source := (row*columns + column) * 2
			values[column*rows+row] = bf16ToFloat32(binary.LittleEndian.Uint16(bytes[source:]))
		}
	}
	return values, nil
}

func projectionOperandAuthority(checkpoint *gemma4e2b.Layer0Checkpoint, name string, activation, uploadedWeight []float32, m, n, k int) (gemma4e2bProjectionOperandAuthority, error) {
	tensor, err := checkpoint.Tensor(name)
	if err != nil {
		return gemma4e2bProjectionOperandAuthority{}, err
	}
	bf16, err := checkpoint.Read(name)
	if err != nil {
		return gemma4e2bProjectionOperandAuthority{}, err
	}
	if len(activation) == 0 || len(bf16) < 2 {
		return gemma4e2bProjectionOperandAuthority{}, fmt.Errorf("empty projection operand: %s", name)
	}
	sampleOffsets := [3]int{0, len(bf16)/2 - len(bf16)/2%2, len(bf16) - 2}
	samples := [3]float32{}
	for index, offset := range sampleOffsets {
		samples[index] = bf16ToFloat32(binary.LittleEndian.Uint16(bf16[offset : offset+2]))
	}
	return gemma4e2bProjectionOperandAuthority{
		ActivationPointer:      fmt.Sprintf("%p", &activation[0]),
		ActivationShape:        []int{m, k},
		ActivationRowStride:    k,
		ActivationElementCount: len(activation),
		ActivationPrecision:    "fp32",
		ActivationSHA256F32LE:  hashFloat32LE(activation),
		WeightTensor:           tensor.Name,
		WeightDType:            tensor.DType,
		WeightLogicalShape:     append([]uint64(nil), tensor.Shape...),
		WeightBF16FileRange:    tensor.FileRange,
		WeightBF16Bytes:        tensor.Bytes,
		WeightBF16SHA256:       gemma4e2b.Digest(bf16),
		WeightTranspose:        "checkpoint [N,K] is transposed to row-major SGEMM B[K,N]",
		WeightSamples:          samples,
		UploadedFP32Bytes:      len(uploadedWeight) * 4,
		DestinationShape:       []int{m, n},
		DestinationRowStride:   n,
		DestinationBytes:       m * n * 4,
		DescriptorRangeBytes:   m * n * 4,
		PushConstants:          [3]int{m, n, k},
		DispatchGroups:         [3]int{(m + 7) / 8, (n + 7) / 8, 1},
		SelectedVariant:        "baseline-scalar-package-kernel-1",
		PackageIdentity:        "prometheus.core@1",
	}, nil
}

func hashFloat32LE(values []float32) string {
	encoded := make([]byte, len(values)*4)
	for index, value := range values {
		binary.LittleEndian.PutUint32(encoded[index*4:], math.Float32bits(value))
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func summarizeGemmaBoundary(name string, values []float32, reference gemma4e2bReferenceBoundary) gemma4e2bBoundaryCheck {
	minimum := float32(0)
	maximum := float32(0)
	finiteCount := 0
	nanCount := 0
	infinityCount := 0
	zeroCount := 0
	nonZeroCount := 0
	for i, value := range values {
		if math.IsNaN(float64(value)) {
			nanCount++
			continue
		}
		if math.IsInf(float64(value), 0) {
			infinityCount++
			continue
		}
		if value == 0 {
			zeroCount++
		} else {
			nonZeroCount++
		}
		if finiteCount == 0 || value < minimum {
			minimum = value
		}
		if finiteCount == 0 || value > maximum {
			maximum = value
		}
		finiteCount++
		if i == 0 && finiteCount == 1 {
			minimum = value
			maximum = value
		}
	}
	actualHash := hashQuantizedBF16AsFloat32(values)
	return gemma4e2bBoundaryCheck{
		Name:            name,
		Shape:           append([]int(nil), reference.Shape...),
		ReferenceHash:   reference.SHA256F32LE,
		ActualQuantized: actualHash,
		HashMatch:       actualHash == reference.SHA256F32LE,
		Minimum:         minimum,
		Maximum:         maximum,
		FiniteCount:     finiteCount,
		NaNCount:        nanCount,
		InfinityCount:   infinityCount,
		ZeroCount:       zeroCount,
		NonZeroCount:    nonZeroCount,
	}
}

func equalFloat32Bits(left, right []float32) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if math.Float32bits(left[index]) != math.Float32bits(right[index]) {
			return false
		}
	}
	return true
}

func hashQuantizedBF16AsFloat32(values []float32) string {
	encoded := make([]byte, len(values)*4)
	for i, value := range values {
		rounded := bf16ToFloat32(float32ToBF16(value))
		binary.LittleEndian.PutUint32(encoded[i*4:], math.Float32bits(rounded))
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func cpuMatmulRowMajor(a, b []float32, m, n, k int) []float32 {
	output := make([]float32, m*n)
	for row := 0; row < m; row++ {
		aRow := row * k
		outRow := row * n
		for column := 0; column < n; column++ {
			sum := float32(0)
			for depth := 0; depth < k; depth++ {
				sum += a[aRow+depth] * b[depth*n+column]
			}
			output[outRow+column] = sum
		}
	}
	return output
}

func compareVectors(actual, reference []float32) gemma4e2bComparison {
	if len(actual) != len(reference) {
		return gemma4e2bComparison{WorstIndex: -1}
	}
	var sumSquareDiff float64
	var sumSquareReference float64
	worstIndex := -1
	var maxAbs float32
	var actualAtWorst float32
	var referenceAtWorst float32
	for i := range actual {
		diff := actual[i] - reference[i]
		absDiff := float32(math.Abs(float64(diff)))
		if worstIndex < 0 || absDiff > maxAbs {
			maxAbs = absDiff
			worstIndex = i
			actualAtWorst = actual[i]
			referenceAtWorst = reference[i]
		}
		sumSquareDiff += float64(diff) * float64(diff)
		sumSquareReference += float64(reference[i]) * float64(reference[i])
	}
	relativeL2 := 0.0
	if sumSquareReference > 0 {
		relativeL2 = math.Sqrt(sumSquareDiff / sumSquareReference)
	} else if sumSquareDiff > 0 {
		relativeL2 = math.Inf(1)
	}
	return gemma4e2bComparison{
		MaxAbsoluteError: maxAbs,
		RelativeL2:       relativeL2,
		WorstIndex:       worstIndex,
		Actual:           actualAtWorst,
		Reference:        referenceAtWorst,
	}
}

func bf16ToFloat32(bits uint16) float32 {
	return math.Float32frombits(uint32(bits) << 16)
}

func float32ToBF16(value float32) uint16 {
	bits := math.Float32bits(value)
	lsb := (bits >> 16) & 1
	rounded := bits + 0x7FFF + lsb
	return uint16(rounded >> 16)
}

func sanitizeNonFiniteJSON(bytes []byte) []byte {
	bytes = replaceJSONToken(bytes, []byte("-Infinity"), []byte("null"))
	bytes = replaceJSONToken(bytes, []byte("Infinity"), []byte("null"))
	bytes = replaceJSONToken(bytes, []byte("NaN"), []byte("null"))
	return bytes
}

func replaceJSONToken(bytes, old, replacement []byte) []byte {
	for {
		index := indexJSONToken(bytes, old)
		if index < 0 {
			return bytes
		}
		bytes = append(append(append([]byte(nil), bytes[:index]...), replacement...), bytes[index+len(old):]...)
	}
}

func indexJSONToken(bytes, token []byte) int {
	for i := 0; i+len(token) <= len(bytes); i++ {
		if string(bytes[i:i+len(token)]) != string(token) {
			continue
		}
		beforeOK := i == 0 || !isJSONIdentifierByte(bytes[i-1])
		afterIndex := i + len(token)
		afterOK := afterIndex == len(bytes) || !isJSONIdentifierByte(bytes[afterIndex])
		if beforeOK && afterOK {
			return i
		}
	}
	return -1
}

func isJSONIdentifierByte(value byte) bool {
	return (value >= 'A' && value <= 'Z') || (value >= 'a' && value <= 'z')
}
