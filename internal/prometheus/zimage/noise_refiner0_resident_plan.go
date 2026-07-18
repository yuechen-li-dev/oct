package zimage

import (
	"fmt"
	"sort"
)

const (
	NoiseRefiner0ResidentProofShaderID     = "model-block-resident-identity"
	NoiseRefiner0ResidentProofPipelineID   = "resident-model-block-v1"
	NoiseRefiner0ResidentProofShaderSHA256 = "5c3010d829c99d20865f9c0c64365eb4ff87dc73f8e9aceacbbe724150d673b1"

	noiseRefiner0ModelElements      = uint64(1024 * 3840)
	noiseRefiner0ResidentIOBytes    = noiseRefiner0ModelElements * 4
	noiseRefiner0ResidentAuditBytes = uint64(1024 * 4)
	noiseRefiner0ResidentMemoryCap  = uint64(512 * 1024 * 1024)
)

// NoiseRefiner0ResidentStep is the complete M1a command vocabulary. Its
// order is fixed by the native owner and deliberately cannot express graph
// traversal, dynamic shader selection, or arbitrary host callbacks.
type NoiseRefiner0ResidentStep string

const (
	NoiseRefiner0ResidentBindPipeline  NoiseRefiner0ResidentStep = "bind-pipeline"
	NoiseRefiner0ResidentBindResources NoiseRefiner0ResidentStep = "bind-declared-resources"
	NoiseRefiner0ResidentPushConstants NoiseRefiner0ResidentStep = "push-declared-constants"
	NoiseRefiner0ResidentDispatch      NoiseRefiner0ResidentStep = "dispatch"
	NoiseRefiner0ResidentBarrier       NoiseRefiner0ResidentStep = "barrier"
	NoiseRefiner0ResidentAuditCopy     NoiseRefiner0ResidentStep = "audit-copy"
	NoiseRefiner0ResidentOutputCopy    NoiseRefiner0ResidentStep = "output-copy"
)

// NoiseRefiner0ResidentWeight is one immutable cache-relative binding. The
// payload is not read here; LoadNoiseRefiner0PayloadBundle has already checked
// its exact byte and SHA-256 identity using bounded streaming reads.
type NoiseRefiner0ResidentWeight struct {
	SourceName        string
	CacheRelativePath string
	Bytes             uint64
	ContentSHA256     string
	Layout            string
}

// NoiseRefiner0ResidentMemoryPlan describes only the M1a vessel. The future
// attention and FFN temporary plan is intentionally absent from this owner.
type NoiseRefiner0ResidentMemoryPlan struct {
	PersistentWeightBytes uint64
	ReusableBytes         uint64
	AuditBytes            uint64
	ExternalInputBytes    uint64
	ExternalOutputBytes   uint64
	TotalCommittedBytes   uint64
	MemoryCeilingBytes    uint64
}

// NoiseRefiner0ResidentBlockPlan is the exact adapter from the validated
// Z-Image payload/contract into the closed resident model-block ABI. It is a
// compiled declaration, not an operator graph or a model registry entry.
type NoiseRefiner0ResidentBlockPlan struct {
	ModelContractID      string
	WeightID             string
	ShaderPortfolioID    string
	PrecisionPolicy      string
	ShaderID             string
	ShaderSHA256         string
	PipelineID           string
	ExecutionSteps       []NoiseRefiner0ResidentStep
	Weights              []NoiseRefiner0ResidentWeight
	Memory               NoiseRefiner0ResidentMemoryPlan
	ExecutionPlanID      string
	ResidentReplaySeedID string
}

func noiseRefiner0ResidentProofShader(contract NoiseRefiner0ModuleContract) (NoiseRefiner0ShaderIdentity, bool) {
	for _, shader := range contract.ShaderPortfolio {
		if shader.ID == NoiseRefiner0ResidentProofShaderID &&
			shader.SHA256 == NoiseRefiner0ResidentProofShaderSHA256 &&
			shader.PipelineID == NoiseRefiner0ResidentProofPipelineID {
			return shader, true
		}
	}
	return NoiseRefiner0ShaderIdentity{}, false
}

// NewNoiseRefiner0ResidentBlockPlan instantiates the M1a vessel from the real
// thirteen-tensor cache contract. It reserves FP32 model-width external
// buffers for the later M1b modulation path but never claims to execute it.
func NewNoiseRefiner0ResidentBlockPlan(bundle NoiseRefiner0PayloadBundle, contract NoiseRefiner0ModuleContract) (NoiseRefiner0ResidentBlockPlan, error) {
	if bundle.CacheManifest.AggregateSHA256 != NoiseRefiner0CacheAggregateSHA256 || bundle.CacheBytes != 361820672 {
		return NoiseRefiner0ResidentBlockPlan{}, fmt.Errorf("noise_refiner.0 resident plan requires the validated M0.75 cache")
	}
	if contract.ModelID != "Tongyi-MAI/Z-Image-Turbo" || contract.ModelRevision != NoiseRefiner0OracleRevision ||
		contract.SourceCheckpointSHA != NoiseRefiner0SourceCheckpointSHA256 || contract.CacheAggregateSHA != NoiseRefiner0CacheAggregateSHA256 ||
		contract.TopologyVersion != NoiseRefiner0TopologyVersion || contract.PrecisionPolicy != NoiseRefiner0PrecisionPolicy ||
		contract.ModelContractID == "" || contract.WeightID == "" || contract.ShaderPortfolioID == "" {
		return NoiseRefiner0ResidentBlockPlan{}, fmt.Errorf("noise_refiner.0 resident plan requires a complete matching module contract")
	}
	shader, ok := noiseRefiner0ResidentProofShader(contract)
	if !ok {
		return NoiseRefiner0ResidentBlockPlan{}, fmt.Errorf("noise_refiner.0 resident plan requires the registered M1a resident proof shader")
	}
	if len(bundle.CacheManifest.Tensors) != 13 {
		return NoiseRefiner0ResidentBlockPlan{}, fmt.Errorf("noise_refiner.0 resident plan requires exactly 13 weight declarations")
	}
	tensors := append([]CacheTensor(nil), bundle.CacheManifest.Tensors...)
	sort.Slice(tensors, func(i, j int) bool { return tensors[i].SourceName < tensors[j].SourceName })
	weights := make([]NoiseRefiner0ResidentWeight, 0, len(tensors))
	var largestWeight uint64
	for _, tensor := range tensors {
		if tensor.SourceName == "" || tensor.DestinationName == "" || tensor.Bytes == 0 || !validSHA256(tensor.SHA256) || tensor.DestinationLayout == "" {
			return NoiseRefiner0ResidentBlockPlan{}, fmt.Errorf("noise_refiner.0 resident plan has incomplete cache binding %q", tensor.SourceName)
		}
		if tensor.Bytes > largestWeight {
			largestWeight = tensor.Bytes
		}
		weights = append(weights, NoiseRefiner0ResidentWeight{
			SourceName:        tensor.SourceName,
			CacheRelativePath: tensor.DestinationName,
			Bytes:             tensor.Bytes,
			ContentSHA256:     tensor.SHA256,
			Layout:            tensor.DestinationLayout,
		})
	}
	steps := []NoiseRefiner0ResidentStep{
		NoiseRefiner0ResidentBindPipeline,
		NoiseRefiner0ResidentBindResources,
		NoiseRefiner0ResidentPushConstants,
		NoiseRefiner0ResidentDispatch,
		NoiseRefiner0ResidentBarrier,
		NoiseRefiner0ResidentAuditCopy,
		NoiseRefiner0ResidentOutputCopy,
	}
	reusable := largestWeight + (noiseRefiner0ResidentIOBytes * 4)
	total := bundle.CacheBytes + reusable + (noiseRefiner0ResidentAuditBytes * 2)
	if total > noiseRefiner0ResidentMemoryCap {
		return NoiseRefiner0ResidentBlockPlan{}, fmt.Errorf("noise_refiner.0 resident plan exceeds M1a memory ceiling: %d > %d", total, noiseRefiner0ResidentMemoryCap)
	}
	planIDParts := []string{contract.ModelContractID, contract.WeightID, contract.ShaderPortfolioID, shader.ID, shader.SHA256, shader.PipelineID, NoiseRefiner0PrecisionPolicy}
	for _, step := range steps {
		planIDParts = append(planIDParts, string(step))
	}
	for _, weight := range weights {
		planIDParts = append(planIDParts, weight.SourceName, weight.ContentSHA256, weight.Layout)
	}
	planID := noiseRefiner0Identity(planIDParts...)
	return NoiseRefiner0ResidentBlockPlan{
		ModelContractID:   contract.ModelContractID,
		WeightID:          contract.WeightID,
		ShaderPortfolioID: contract.ShaderPortfolioID,
		PrecisionPolicy:   NoiseRefiner0PrecisionPolicy,
		ShaderID:          shader.ID,
		ShaderSHA256:      shader.SHA256,
		PipelineID:        shader.PipelineID,
		ExecutionSteps:    steps,
		Weights:           weights,
		Memory: NoiseRefiner0ResidentMemoryPlan{
			PersistentWeightBytes: bundle.CacheBytes,
			ReusableBytes:         reusable,
			AuditBytes:            noiseRefiner0ResidentAuditBytes * 2,
			ExternalInputBytes:    noiseRefiner0ResidentIOBytes,
			ExternalOutputBytes:   noiseRefiner0ResidentIOBytes,
			TotalCommittedBytes:   total,
			MemoryCeilingBytes:    noiseRefiner0ResidentMemoryCap,
		},
		ExecutionPlanID:      planID,
		ResidentReplaySeedID: noiseRefiner0Identity(contract.ExecutionReplayID, planID),
	}, nil
}
