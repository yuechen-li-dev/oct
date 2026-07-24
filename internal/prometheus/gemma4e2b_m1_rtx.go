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
	gemma4e2bM1QHeads          = 8
	gemma4e2bM1KVHeads         = 1
	gemma4e2bM1HeadWidth       = 256
	gemma4e2bM1RMSNormEpsilon  = float32(1e-6)
	gemma4e2bM1ExactSourceHash = uint64(0x19dbb6c9cba24e13)
	gemma4e2bM1QHeadSourceHash = uint64(0x516bfcdf20536173)
	gemma4e2bM1KHeadSourceHash = uint64(0x2fc29800d1ddae2b)
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
		Layer0QNormalized  gemma4e2bReferenceBoundary `json:"layer0_q_normalized"`
		Layer0KNormalized  gemma4e2bReferenceBoundary `json:"layer0_k_normalized"`
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
	RuntimeEnvironment       string
	RMSNorm                  gemma4e2bBoundaryCheck
	ProjectionActivationBF16 CorrectnessResult
	Q                        gemma4e2bBoundaryCheck
	K                        gemma4e2bBoundaryCheck
	V                        gemma4e2bBoundaryCheck
	QNormalized              gemma4e2bBoundaryCheck
	KNormalized              gemma4e2bBoundaryCheck
	QProjectionBF16          CorrectnessResult
	KProjectionBF16          CorrectnessResult
	VProjectionBF16          CorrectnessResult
	QPortableProjection      gemma4e2bPortableProjectionComparison
	KPortableProjection      gemma4e2bPortableProjectionComparison
	VPortableProjection      gemma4e2bPortableProjectionComparison
	QActivationBF16CPU       CorrectnessResult
	KActivationBF16CPU       CorrectnessResult
	VActivationBF16CPU       CorrectnessResult
	QCPUContractHash         string
	KCPUContractHash         string
	VCPUContractHash         string
	QCPUContractDiff         gemma4e2bComparison
	KCPUContractDiff         gemma4e2bComparison
	VCPUContractDiff         gemma4e2bComparison
	QCPUContractPolicy       CorrectnessResult
	KCPUContractPolicy       CorrectnessResult
	VCPUContractPolicy       CorrectnessResult
	QRepeatedStable          bool
	KRepeatedStable          bool
	VRepeatedStable          bool
	QNormalizedPolicy        CorrectnessResult
	KNormalizedPolicy        CorrectnessResult
	QNormalizedCPUDiff       gemma4e2bComparison
	KNormalizedCPUDiff       gemma4e2bComparison
	QNormalizedCPU           CorrectnessResult
	KNormalizedCPU           CorrectnessResult
	QNormalizedPortable      gemma4e2bPortableProjectionComparison
	KNormalizedPortable      gemma4e2bPortableProjectionComparison
	QNormalizedStable        bool
	KNormalizedStable        bool
	QNormalizedNative        reactorGemma4E2BM1InputRMSNormResult
	KNormalizedNative        reactorGemma4E2BM1InputRMSNormResult
	QRopeNative              reactorGemma4E2BM1HeadRMSNormRopeResult
	KRopeNative              reactorGemma4E2BM1HeadRMSNormRopeResult
	QRope                    gemma4e2bPortableProjectionComparison
	KRope                    gemma4e2bPortableProjectionComparison
	QRopeResidentContract    gemma4e2bPortableProjectionComparison
	KRopeResidentContract    gemma4e2bPortableProjectionComparison
	QRopeStable              bool
	KRopeStable              bool
	RopeRecoveredAfterReject bool
	Scores                   []float32
	ScoreNative              reactorGemma4E2BM1AttentionScoresResult
	ScoreStageLocal          gemma4e2bComparison
	ScoreStageLocalExact     bool
	ScorePristine            gemma4e2bComparison
	ScorePristineExact       bool
	QOperands                gemma4e2bProjectionOperandAuthority
	KOperands                gemma4e2bProjectionOperandAuthority
	VOperands                gemma4e2bProjectionOperandAuthority
	RMSNormNative            reactorGemma4E2BM1InputRMSNormResult
	PreparationOrder         uint32
	SameSession              *gemma4e2bSameSessionTrace
}

// gemma4e2bSameSessionTrace is a compact observation of the known lifecycle
// boundary.  It deliberately records only values exposed by the existing
// native result ABI; it does not manufacture a runtime/session owner ID or a
// hidden M46 snapshot that the current ABI does not return.
type gemma4e2bSameSessionTrace struct {
	SessionLabel                         string
	FirstOperationEpoch                  uint64
	SecondOperationEpoch                 uint64
	FirstPreparationOrder                uint32
	SecondPreparationOrder               uint32
	FirstScore                           reactorGemma4E2BM1AttentionScoresResult
	SecondBoundary                       reactorGemma4E2BM1AttentionScoresResult
	SecondCallError                      string
	SecondM46PreparationBoundaryObserved bool
	M49RequiredWeightValidationRejected  bool
	PositionalDispatchBegun              bool
	ScoreDispatchBegun                   bool
	ScoreDestinationWritten              bool
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

// gemma4e2bPortableProjectionComparison is the owner-selected portable
// projection policy: BF16 operands and outputs, FP32 accumulation, with a
// one-BF16-step allowance only for a different valid FP32 reduction order.
type gemma4e2bPortableProjectionComparison struct {
	ExactMatches          int
	Differing             int
	MaximumBF16ULP        uint16
	MaximumAbsolute       float32
	RelativeL2            float64
	WorstIndex            int
	Reference             float32
	Actual                float32
	ReferenceBF16         uint16
	ActualBF16            uint16
	FiniteCount           int
	ZeroCount             int
	NaNCount              int
	InfinityCount         int
	WithinPortablePolicy  bool
	WithinBF16StagePolicy bool
}

type gemma4e2bValidationLane uint8

const (
	gemma4e2bFreshSessionLane gemma4e2bValidationLane = iota
	gemma4e2bSameSessionLane
)

func runGemma4e2bCanonicalQKVRTX(checkpointRoot string) (gemma4e2bCanonicalSliceResult, error) {
	return runGemma4e2bValidationLane(checkpointRoot, 1, gemma4e2bFreshSessionLane)
}

func runGemma4e2bFreshSessionRawScoreAuthority(checkpointRoot string, preparationOrder uint32) (gemma4e2bCanonicalSliceResult, error) {
	return runGemma4e2bValidationLane(checkpointRoot, preparationOrder, gemma4e2bFreshSessionLane)
}

func runGemma4e2bSameSessionLifecycleCharacterization(checkpointRoot string) (gemma4e2bCanonicalSliceResult, error) {
	return runGemma4e2bValidationLane(checkpointRoot, 1, gemma4e2bSameSessionLane)
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func runGemma4e2bValidationLane(checkpointRoot string, preparationOrder uint32, lane gemma4e2bValidationLane) (gemma4e2bCanonicalSliceResult, error) {
	var result gemma4e2bCanonicalSliceResult
	result.PreparationOrder = preparationOrder
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
	normOutput, _, nativeResult, err := runtime.Gemma4E2BM1ProjectionActivationRMSNorm(
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
	projectionActivationReference, err := loadBF16Fixture(
		filepath.Join(tempRoot, "g4e2b-m1-fixtures", "layer0_input_rmsnorm.bf16le.bin"),
		gemma4e2bM1Tokens*gemma4e2bM1Width,
	)
	if err != nil {
		return result, err
	}
	result.ProjectionActivationBF16 = compareAgainstOracle(projectionActivationReference, roundTripBF16Slice(normOutput))

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
	qCPU := roundTripBF16Slice(cpuMatmulRowMajor(normOutput, qWeight, gemma4e2bM1Tokens, gemma4e2bM1QColumns, gemma4e2bM1Width))
	result.QCPUContractHash = hashQuantizedBF16AsFloat32(qCPU)
	result.QCPUContractDiff = compareVectors(qOutput, qCPU)
	result.QCPUContractPolicy = compareAgainstOracle(qCPU, roundTripBF16Slice(qOutput))
	result.QPortableProjection = comparePortableProjection(qCPU, roundTripBF16Slice(qOutput))
	qProjectionBF16Reference, err := loadBF16Fixture(filepath.Join(tempRoot, "g4e2b-m1-fixtures", "layer0_q_linear.bf16le.bin"), gemma4e2bM1Tokens*gemma4e2bM1QColumns)
	if err != nil {
		return result, err
	}
	result.QProjectionBF16 = compareAgainstOracle(qProjectionBF16Reference, roundTripBF16Slice(qOutput))
	result.QActivationBF16CPU = compareAgainstOracle(
		qProjectionBF16Reference,
		qCPU,
	)

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
	kCPU := roundTripBF16Slice(cpuMatmulRowMajor(normOutput, kWeight, gemma4e2bM1Tokens, gemma4e2bM1KColumns, gemma4e2bM1Width))
	result.KCPUContractHash = hashQuantizedBF16AsFloat32(kCPU)
	result.KCPUContractDiff = compareVectors(kOutput, kCPU)
	result.KCPUContractPolicy = compareAgainstOracle(kCPU, roundTripBF16Slice(kOutput))
	result.KPortableProjection = comparePortableProjection(kCPU, roundTripBF16Slice(kOutput))
	kProjectionBF16Reference, err := loadBF16Fixture(filepath.Join(tempRoot, "g4e2b-m1-fixtures", "layer0_k_linear.bf16le.bin"), gemma4e2bM1Tokens*gemma4e2bM1KColumns)
	if err != nil {
		return result, err
	}
	result.KProjectionBF16 = compareAgainstOracle(kProjectionBF16Reference, roundTripBF16Slice(kOutput))
	result.KActivationBF16CPU = compareAgainstOracle(
		kProjectionBF16Reference,
		kCPU,
	)

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
	vCPU := roundTripBF16Slice(cpuMatmulRowMajor(normOutput, vWeight, gemma4e2bM1Tokens, gemma4e2bM1VColumns, gemma4e2bM1Width))
	result.VCPUContractHash = hashQuantizedBF16AsFloat32(vCPU)
	result.VCPUContractDiff = compareVectors(vOutput, vCPU)
	result.VCPUContractPolicy = compareAgainstOracle(vCPU, roundTripBF16Slice(vOutput))
	result.VPortableProjection = comparePortableProjection(vCPU, roundTripBF16Slice(vOutput))
	vProjectionBF16Reference, err := loadBF16Fixture(filepath.Join(tempRoot, "g4e2b-m1-fixtures", "layer0_v_linear.bf16le.bin"), gemma4e2bM1Tokens*gemma4e2bM1VColumns)
	if err != nil {
		return result, err
	}
	result.VProjectionBF16 = compareAgainstOracle(vProjectionBF16Reference, roundTripBF16Slice(vOutput))
	result.VActivationBF16CPU = compareAgainstOracle(
		vProjectionBF16Reference,
		vCPU,
	)

	// Q is logically [token, query_head, head_channel]. The Q projection's
	// row-major [token, 2048] staging is therefore exactly the flattened
	// per-head layout required by the authoritative head RMSNorm.
	qNormWeight, err := decodeBF16Vector(checkpoint, "model.language_model.layers.0.self_attn.q_norm.weight", gemma4e2bM1HeadWidth)
	if err != nil {
		return result, err
	}
	qNormalizedReference, err := loadBF16Fixture(filepath.Join(tempRoot, "g4e2b-m1-fixtures", "layer0_q_normalized.bf16le.bin"), gemma4e2bM1Tokens*gemma4e2bM1QHeads*gemma4e2bM1HeadWidth)
	if err != nil {
		return result, err
	}
	qNormalized, _, qNormalizedNative, err := runtime.Gemma4E2BM1HeadRMSNorm(qOutput, qNormWeight, gemma4e2bM1Tokens*gemma4e2bM1QHeads, gemma4e2bM1HeadWidth, gemma4e2bM1HeadWidth, gemma4e2bM1RMSNormEpsilon, 2, 2, gemma4e2bM1QHeadSourceHash)
	if err != nil {
		return result, fmt.Errorf("q head normalization: %w", err)
	}
	result.QNormalizedNative = qNormalizedNative
	result.QNormalized = summarizeGemmaBoundary("layer0_q_normalized", qNormalized, reference.Boundaries.Layer0QNormalized)
	result.QNormalizedPolicy = compareAgainstOracle(qNormalizedReference, qNormalized)
	// The projection has already been accepted under the portable BF16 policy.
	// Reconstruct the independent head boundary from that exact resident-BF16
	// projection image, not from the historical oneDNN capture.
	qNormalizedCPU := roundTripBF16Slice(cpuHeadRMSNorm(roundTripBF16Slice(qOutput), qNormWeight, gemma4e2bM1HeadWidth, gemma4e2bM1RMSNormEpsilon))
	result.QNormalizedCPUDiff = compareVectors(qNormalized, qNormalizedCPU)
	result.QNormalizedCPU = compareAgainstOracle(qNormalizedCPU, qNormalized)
	result.QNormalizedPortable = comparePortableProjection(qNormalizedCPU, qNormalized)

	// K is [token, one_kv_head, head_channel], so its compact projection rows
	// already are the authority's one-head normalization rows.
	kNormWeight, err := decodeBF16Vector(checkpoint, "model.language_model.layers.0.self_attn.k_norm.weight", gemma4e2bM1HeadWidth)
	if err != nil {
		return result, err
	}
	kNormalizedReference, err := loadBF16Fixture(filepath.Join(tempRoot, "g4e2b-m1-fixtures", "layer0_k_normalized.bf16le.bin"), gemma4e2bM1Tokens*gemma4e2bM1KVHeads*gemma4e2bM1HeadWidth)
	if err != nil {
		return result, err
	}
	kNormalized, _, kNormalizedNative, err := runtime.Gemma4E2BM1HeadRMSNorm(kOutput, kNormWeight, gemma4e2bM1Tokens*gemma4e2bM1KVHeads, gemma4e2bM1HeadWidth, gemma4e2bM1HeadWidth, gemma4e2bM1RMSNormEpsilon, 3, 3, gemma4e2bM1KHeadSourceHash)
	if err != nil {
		return result, fmt.Errorf("k head normalization: %w", err)
	}
	result.KNormalizedNative = kNormalizedNative
	result.KNormalized = summarizeGemmaBoundary("layer0_k_normalized", kNormalized, reference.Boundaries.Layer0KNormalized)
	result.KNormalizedPolicy = compareAgainstOracle(kNormalizedReference, kNormalized)
	kNormalizedCPU := roundTripBF16Slice(cpuHeadRMSNorm(roundTripBF16Slice(kOutput), kNormWeight, gemma4e2bM1HeadWidth, gemma4e2bM1RMSNormEpsilon))
	result.KNormalizedCPUDiff = compareVectors(kNormalized, kNormalizedCPU)
	result.KNormalizedCPU = compareAgainstOracle(kNormalizedCPU, kNormalized)
	result.KNormalizedPortable = comparePortableProjection(kNormalizedCPU, kNormalized)

	ropeCosine, err := loadBF16Fixture(filepath.Join(tempRoot, "g4e2b-m1-fixtures", "layer0_rope_cos.bf16le.bin"), gemma4e2bM1Tokens*gemma4e2bM1HeadWidth)
	if err != nil {
		return result, err
	}
	ropeSine, err := loadBF16Fixture(filepath.Join(tempRoot, "g4e2b-m1-fixtures", "layer0_rope_sin.bf16le.bin"), gemma4e2bM1Tokens*gemma4e2bM1HeadWidth)
	if err != nil {
		return result, err
	}
	qRopeReference, err := loadBF16Fixture(filepath.Join(tempRoot, "g4e2b-m1-fixtures", "layer0_q_rope.bf16le.bin"), gemma4e2bM1Tokens*gemma4e2bM1QHeads*gemma4e2bM1HeadWidth)
	if err != nil {
		return result, err
	}
	kRopeReference, err := loadBF16Fixture(filepath.Join(tempRoot, "g4e2b-m1-fixtures", "layer0_k_rope.bf16le.bin"), gemma4e2bM1Tokens*gemma4e2bM1KVHeads*gemma4e2bM1HeadWidth)
	if err != nil {
		return result, err
	}
	qRope, qRopeNative, err := runtime.Gemma4E2BM1HeadRMSNormRope(qOutput, qNormWeight, ropeCosine, ropeSine, gemma4e2bM1Tokens, gemma4e2bM1QHeads, gemma4e2bM1HeadWidth, gemma4e2bM1RMSNormEpsilon, 6, 6, 1, gemma4e2bM1QHeadSourceHash)
	if err != nil {
		return result, fmt.Errorf("q rope: %w", err)
	}
	result.QRopeNative = qRopeNative
	result.QRope = comparePortableProjection(qRopeReference, qRope)
	result.QRopeResidentContract = comparePortableProjection(cpuGemma4e2bRope(qNormalized, ropeCosine, ropeSine, gemma4e2bM1Tokens, gemma4e2bM1QHeads, gemma4e2bM1HeadWidth), qRope)
	kRope, kRopeNative, err := runtime.Gemma4E2BM1HeadRMSNormRope(kOutput, kNormWeight, ropeCosine, ropeSine, gemma4e2bM1Tokens, gemma4e2bM1KVHeads, gemma4e2bM1HeadWidth, gemma4e2bM1RMSNormEpsilon, 7, 7, 1, gemma4e2bM1KHeadSourceHash)
	if err != nil {
		return result, fmt.Errorf("k rope: %w", err)
	}
	result.KRopeNative = kRopeNative
	result.KRope = comparePortableProjection(kRopeReference, kRope)
	result.KRopeResidentContract = comparePortableProjection(cpuGemma4e2bRope(kNormalized, ropeCosine, ropeSine, gemma4e2bM1Tokens, gemma4e2bM1KVHeads, gemma4e2bM1HeadWidth), kRope)
	// This is the closed resident chain: the native operation prepares Q and K
	// without individual RoPE readbacks, retains both kernel-68 destinations,
	// dispatches package-backed kernel 69, and returns only final FP32 scores.
	// Keep the live score proof in its own model-private runtime. The accepted
	// positional comparison above deliberately retains Q/K for its own evidence;
	// sharing that session would make its old pins part of this operation's
	// lifetime contract instead of proving the score chain's fresh ownership.
	scoreRuntime, err := newNativeRuntime()
	if err != nil {
		return result, err
	}
	defer scoreRuntime.Close()
	scores, scoreNative, err := scoreRuntime.Gemma4E2BM1AttentionScores(
		qOutput, kOutput, qNormWeight, kNormWeight, ropeCosine, ropeSine,
		gemma4e2bM1Tokens, gemma4e2bM1QHeads, gemma4e2bM1KVHeads, gemma4e2bM1HeadWidth,
		gemma4e2bM1RMSNormEpsilon, 0.0625,
		preparationOrder, 16, 15, 16, 15, 2, gemma4e2bM1QHeadSourceHash, gemma4e2bM1KHeadSourceHash,
	)
	if err != nil {
		return result, fmt.Errorf("resident raw attention scores: %w", err)
	}
	result.Scores = scores
	result.ScoreNative = scoreNative
	result.ScoreStageLocal = compareVectors(scores, cpuGemma4e2bRawScores(qRope, kRope))
	result.ScoreStageLocalExact = equalFloat32Bits(scores, cpuGemma4e2bRawScores(qRope, kRope))
	pristineScores := cpuGemma4e2bRawScores(qRopeReference, kRopeReference)
	result.ScorePristine = compareVectors(scores, pristineScores)
	result.ScorePristineExact = equalFloat32Bits(scores, pristineScores)
	if lane == gemma4e2bSameSessionLane {
		// The second call is intentionally expected to reject.  The current
		// outer ABI does not return the successful M46 generation/hash on the
		// following M49 early rejection; zero propagation fields plus the exact
		// M49 detail and boundary are the available observation.
		_, secondNative, secondErr := scoreRuntime.Gemma4E2BM1AttentionScores(
			qOutput, kOutput, qNormWeight, kNormWeight, ropeCosine, ropeSine,
			gemma4e2bM1Tokens, gemma4e2bM1QHeads, gemma4e2bM1KVHeads, gemma4e2bM1HeadWidth,
			gemma4e2bM1RMSNormEpsilon, 0.0625,
			0, 100, 101, 100, 101, 3, gemma4e2bM1QHeadSourceHash, gemma4e2bM1KHeadSourceHash,
		)
		trace := &gemma4e2bSameSessionTrace{
			SessionLabel:                        "one native runtime handle; identity not exported by current ABI",
			FirstOperationEpoch:                 1,
			SecondOperationEpoch:                2,
			FirstPreparationOrder:               preparationOrder,
			SecondPreparationOrder:              0,
			FirstScore:                          scoreNative,
			SecondBoundary:                      secondNative,
			SecondCallError:                     errorString(secondErr),
			M49RequiredWeightValidationRejected: secondErr != nil && secondNative.DetailCode == -7406 && secondNative.PositionalDispatchCount == 0,
			PositionalDispatchBegun:             secondNative.PositionalDispatchCount != 0,
			ScoreDispatchBegun:                  secondNative.ScoreDispatchCount != 0,
			ScoreDestinationWritten:             secondNative.ScoreWritten,
		}
		trace.SecondM46PreparationBoundaryObserved = trace.M49RequiredWeightValidationRejected &&
			secondNative.ObservedWeightGeneration == 0 && secondNative.RequestedWeightGeneration == 0
		result.SameSession = trace
		if secondErr == nil {
			return result, fmt.Errorf("same-session characterization: M49 unexpectedly succeeded")
		}
		return result, nil
	}
	// The same persistent descriptor set must be rewritten across the Q/K
	// shape change. Repeat both directions to prove that no stale range or
	// source binding is retained by the package-backed pipeline.
	qRopeRepeated, _, err := runtime.Gemma4E2BM1HeadRMSNormRope(qOutput, qNormWeight, ropeCosine, ropeSine, gemma4e2bM1Tokens, gemma4e2bM1QHeads, gemma4e2bM1HeadWidth, gemma4e2bM1RMSNormEpsilon, 8, 8, 1, gemma4e2bM1QHeadSourceHash)
	if err != nil {
		return result, fmt.Errorf("repeat q rope: %w", err)
	}
	qRopeRepeatedAgain, _, err := runtime.Gemma4E2BM1HeadRMSNormRope(qOutput, qNormWeight, ropeCosine, ropeSine, gemma4e2bM1Tokens, gemma4e2bM1QHeads, gemma4e2bM1HeadWidth, gemma4e2bM1RMSNormEpsilon, 9, 9, 1, gemma4e2bM1QHeadSourceHash)
	if err != nil {
		return result, fmt.Errorf("second repeat q rope: %w", err)
	}
	kRopeRepeated, _, err := runtime.Gemma4E2BM1HeadRMSNormRope(kOutput, kNormWeight, ropeCosine, ropeSine, gemma4e2bM1Tokens, gemma4e2bM1KVHeads, gemma4e2bM1HeadWidth, gemma4e2bM1RMSNormEpsilon, 10, 10, 1, gemma4e2bM1KHeadSourceHash)
	if err != nil {
		return result, fmt.Errorf("repeat k rope: %w", err)
	}
	kRopeRepeatedAgain, _, err := runtime.Gemma4E2BM1HeadRMSNormRope(kOutput, kNormWeight, ropeCosine, ropeSine, gemma4e2bM1Tokens, gemma4e2bM1KVHeads, gemma4e2bM1HeadWidth, gemma4e2bM1RMSNormEpsilon, 11, 11, 1, gemma4e2bM1KHeadSourceHash)
	if err != nil {
		return result, fmt.Errorf("second repeat k rope: %w", err)
	}
	result.QRopeStable = equalFloat32Bits(qRope, qRopeRepeated) && equalFloat32Bits(qRope, qRopeRepeatedAgain)
	result.KRopeStable = equalFloat32Bits(kRope, kRopeRepeated) && equalFloat32Bits(kRope, kRopeRepeatedAgain)
	// Invalid head count is rejected before command recording. The immediately
	// following valid calls prove the runtime remains reusable after rejection.
	if _, _, rejected := runtime.Gemma4E2BM1HeadRMSNormRope(kOutput, kNormWeight, ropeCosine, ropeSine, gemma4e2bM1Tokens, 2, gemma4e2bM1HeadWidth, gemma4e2bM1RMSNormEpsilon, 12, 12, 1, gemma4e2bM1KHeadSourceHash); rejected == nil {
		return result, fmt.Errorf("rope invalid-head rejection unexpectedly succeeded")
	}
	qRopeRecovered, _, err := runtime.Gemma4E2BM1HeadRMSNormRope(qOutput, qNormWeight, ropeCosine, ropeSine, gemma4e2bM1Tokens, gemma4e2bM1QHeads, gemma4e2bM1HeadWidth, gemma4e2bM1RMSNormEpsilon, 13, 13, 1, gemma4e2bM1QHeadSourceHash)
	if err != nil {
		return result, fmt.Errorf("q rope after rejection: %w", err)
	}
	kRopeRecovered, _, err := runtime.Gemma4E2BM1HeadRMSNormRope(kOutput, kNormWeight, ropeCosine, ropeSine, gemma4e2bM1Tokens, gemma4e2bM1KVHeads, gemma4e2bM1HeadWidth, gemma4e2bM1RMSNormEpsilon, 14, 14, 1, gemma4e2bM1KHeadSourceHash)
	if err != nil {
		return result, fmt.Errorf("k rope after rejection: %w", err)
	}
	result.RopeRecoveredAfterReject = equalFloat32Bits(qRope, qRopeRecovered) && equalFloat32Bits(kRope, kRopeRecovered)

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
	normalizationRuntime, err := newNativeRuntime()
	if err != nil {
		return result, err
	}
	defer normalizationRuntime.Close()
	qNormalizedRepeated, _, _, err := normalizationRuntime.Gemma4E2BM1HeadRMSNorm(qOutput, qNormWeight, gemma4e2bM1Tokens*gemma4e2bM1QHeads, gemma4e2bM1HeadWidth, gemma4e2bM1HeadWidth, gemma4e2bM1RMSNormEpsilon, 4, 4, gemma4e2bM1QHeadSourceHash)
	if err != nil {
		return result, fmt.Errorf("repeat q head normalization: %w", err)
	}
	kNormalizedRepeated, _, _, err := normalizationRuntime.Gemma4E2BM1HeadRMSNorm(kOutput, kNormWeight, gemma4e2bM1Tokens*gemma4e2bM1KVHeads, gemma4e2bM1HeadWidth, gemma4e2bM1HeadWidth, gemma4e2bM1RMSNormEpsilon, 5, 5, gemma4e2bM1KHeadSourceHash)
	if err != nil {
		return result, fmt.Errorf("repeat k head normalization: %w", err)
	}
	result.QNormalizedStable = equalFloat32Bits(qNormalized, qNormalizedRepeated)
	result.KNormalizedStable = equalFloat32Bits(kNormalized, kNormalizedRepeated)
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
		ActivationPrecision:    "bf16_storage_reexpanded_fp32",
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

func roundTripBF16Slice(values []float32) []float32 {
	rounded := make([]float32, len(values))
	for i, value := range values {
		rounded[i] = bf16ToFloat32(float32ToBF16(value))
	}
	return rounded
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

func cpuHeadRMSNorm(input, weight []float32, headWidth int, epsilon float32) []float32 {
	if headWidth <= 0 || len(input)%headWidth != 0 || len(weight) != headWidth {
		return nil
	}
	output := make([]float32, len(input))
	for row := 0; row < len(input)/headWidth; row++ {
		start := row * headWidth
		var sum float32
		for channel := 0; channel < headWidth; channel++ {
			value := input[start+channel]
			sum += value * value
		}
		invRMS := float32(1.0 / math.Sqrt(float64(sum/float32(headWidth)+epsilon)))
		for channel := 0; channel < headWidth; channel++ {
			output[start+channel] = input[start+channel] * invRMS * weight[channel]
		}
	}
	return output
}

// cpuGemma4e2bRope is an implementation-boundary witness only. It reproduces
// the accepted BF16 storage graph from the supplied resident source image:
// BF16 product, BF16 sum/difference, and half-split pairing.
func cpuGemma4e2bRope(source, cosine, sine []float32, tokens, heads, headWidth int) []float32 {
	if tokens != gemma4e2bM1Tokens || (heads != gemma4e2bM1QHeads && heads != gemma4e2bM1KVHeads) || headWidth != gemma4e2bM1HeadWidth || len(source) != tokens*heads*headWidth || len(cosine) != tokens*headWidth || len(sine) != tokens*headWidth {
		return nil
	}
	output := make([]float32, len(source))
	for token := 0; token < tokens; token++ {
		for head := 0; head < heads; head++ {
			base := (token*heads + head) * headWidth
			for component := 0; component < headWidth; component++ {
				frequency := component % (headWidth / 2)
				mate := base + frequency + headWidth/2
				if component >= headWidth/2 {
					mate = base + frequency
				}
				first := bf16ToFloat32(float32ToBF16(source[base+component] * cosine[token*headWidth+frequency]))
				second := bf16ToFloat32(float32ToBF16(source[mate] * sine[token*headWidth+frequency]))
				value := first - second
				if component >= headWidth/2 {
					value = first + second
				}
				output[base+component] = bf16ToFloat32(float32ToBF16(value))
			}
		}
	}
	return output
}

// cpuGemma4e2bRawScores is the explicit stage-local pre-mask authority. It
// intentionally has no masking, softmax, query pre-scale, or logit soft-cap.
func cpuGemma4e2bRawScores(query, key []float32) []float32 {
	if len(query) != gemma4e2bM1Tokens*gemma4e2bM1QHeads*gemma4e2bM1HeadWidth ||
		len(key) != gemma4e2bM1Tokens*gemma4e2bM1KVHeads*gemma4e2bM1HeadWidth {
		return nil
	}
	scores := make([]float32, gemma4e2bM1QHeads*gemma4e2bM1Tokens*gemma4e2bM1Tokens)
	for queryHead := 0; queryHead < gemma4e2bM1QHeads; queryHead++ {
		for queryPosition := 0; queryPosition < gemma4e2bM1Tokens; queryPosition++ {
			queryBase := (queryPosition*gemma4e2bM1QHeads + queryHead) * gemma4e2bM1HeadWidth
			for keyPosition := 0; keyPosition < gemma4e2bM1Tokens; keyPosition++ {
				keyBase := keyPosition * gemma4e2bM1HeadWidth // Every Q head maps to KV head 0.
				accumulator := float32(0)
				for coordinate := 0; coordinate < gemma4e2bM1HeadWidth; coordinate++ {
					accumulator += query[queryBase+coordinate] * key[keyBase+coordinate]
				}
				scores[(queryHead*gemma4e2bM1Tokens+queryPosition)*gemma4e2bM1Tokens+keyPosition] = accumulator * 0.0625
			}
		}
	}
	return scores
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

func comparePortableProjection(reference, actual []float32) gemma4e2bPortableProjectionComparison {
	comparison := gemma4e2bPortableProjectionComparison{WorstIndex: -1}
	if len(reference) != len(actual) {
		return comparison
	}
	var sumSquareDiff float64
	var sumSquareReference float64
	allFiniteDifferencesWithinOneStep := true
	for index := range reference {
		referenceValue := reference[index]
		actualValue := actual[index]
		if math.IsNaN(float64(referenceValue)) || math.IsNaN(float64(actualValue)) {
			comparison.NaNCount++
			allFiniteDifferencesWithinOneStep = false
			continue
		}
		if math.IsInf(float64(referenceValue), 0) || math.IsInf(float64(actualValue), 0) {
			comparison.InfinityCount++
			if referenceValue != actualValue {
				allFiniteDifferencesWithinOneStep = false
			}
			continue
		}
		comparison.FiniteCount++
		if referenceValue == 0 {
			comparison.ZeroCount++
		}
		sumSquareReference += float64(referenceValue) * float64(referenceValue)
		if math.Float32bits(referenceValue) == math.Float32bits(actualValue) {
			comparison.ExactMatches++
			continue
		}
		comparison.Differing++
		referenceBits := float32ToBF16(referenceValue)
		actualBits := float32ToBF16(actualValue)
		ulp := orderedBF16Distance(referenceBits, actualBits)
		if ulp > comparison.MaximumBF16ULP {
			comparison.MaximumBF16ULP = ulp
		}
		if ulp > 1 {
			allFiniteDifferencesWithinOneStep = false
		}
		diff := actualValue - referenceValue
		absolute := float32(math.Abs(float64(diff)))
		if comparison.WorstIndex < 0 || absolute > comparison.MaximumAbsolute {
			comparison.MaximumAbsolute = absolute
			comparison.WorstIndex = index
			comparison.Reference = referenceValue
			comparison.Actual = actualValue
			comparison.ReferenceBF16 = referenceBits
			comparison.ActualBF16 = actualBits
		}
		sumSquareDiff += float64(diff) * float64(diff)
	}
	if sumSquareReference > 0 {
		comparison.RelativeL2 = math.Sqrt(sumSquareDiff / sumSquareReference)
	} else if sumSquareDiff > 0 {
		comparison.RelativeL2 = math.Inf(1)
	}
	comparison.WithinPortablePolicy = comparison.NaNCount == 0 && allFiniteDifferencesWithinOneStep && comparison.RelativeL2 <= 1e-5
	// A final BF16 stage is evaluated in BF16 ordered-ULP space. Its compact
	// operational tolerance is intentionally separate from the projection
	// policy because one legal head-reduction crossing can affect a scale.
	comparison.WithinBF16StagePolicy = comparison.NaNCount == 0 && allFiniteDifferencesWithinOneStep && comparison.RelativeL2 <= 1e-4
	return comparison
}

func orderedBF16Distance(left, right uint16) uint16 {
	leftOrdered := orderedBF16(left)
	rightOrdered := orderedBF16(right)
	if leftOrdered >= rightOrdered {
		return leftOrdered - rightOrdered
	}
	return rightOrdered - leftOrdered
}

func orderedBF16(bits uint16) uint16 {
	if bits&0x8000 != 0 {
		return ^bits
	}
	return bits | 0x8000
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
