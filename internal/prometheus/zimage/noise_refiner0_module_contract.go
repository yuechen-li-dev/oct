package zimage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

const (
	NoiseRefiner0TopologyVersion = "zimage-noise-refiner.0-topology-v1"
	NoiseRefiner0PrecisionPolicy = "fp16-weights-fp32-norm-reduction-softmax-rope-v1"
	NoiseRefiner0RuntimeVersion  = "prometheus-evt2-m1-v1"
)

// NoiseRefiner0ShaderIdentity identifies one generated shader without making
// its local path part of replay identity. IDs and generated SHA-256 values are
// the durable execution facts; source paths remain report-only provenance.
type NoiseRefiner0ShaderIdentity struct {
	ID         string
	SHA256     string
	PipelineID string
}

// NoiseRefiner0ModuleContract is deliberately one model/block owner, not a
// registry entry or a graph node. Vulkan resource ownership is added only when
// this immutable contract has been accepted by the module creator.
type NoiseRefiner0ModuleContract struct {
	ModelID               string
	ModelRevision         string
	SourceCheckpointSHA   string
	CacheAggregateSHA     string
	TopologyVersion       string
	PrecisionPolicy       string
	SemanticSourceSHA256  string
	DeviceCapabilityRoute string
	RuntimeVersion        string
	ShaderPortfolio       []NoiseRefiner0ShaderIdentity
	ModelContractID       string
	WeightID              string
	ShaderPortfolioID     string
	ExecutionReplayID     string
}

func noiseRefiner0Identity(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = hash.Write([]byte(part))
		_, _ = hash.Write([]byte{'\n'})
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func validSHA256(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

// NewNoiseRefiner0ModuleContract creates only the deterministic model identity
// authority. It refuses incomplete shader identities so pipeline creation can
// never silently select an unrecorded experimental artifact.
func NewNoiseRefiner0ModuleContract(bundle NoiseRefiner0PayloadBundle, semanticSourceSHA256, deviceCapabilityRoute string, shaders []NoiseRefiner0ShaderIdentity) (NoiseRefiner0ModuleContract, error) {
	if bundle.CacheManifest.AggregateSHA256 != NoiseRefiner0CacheAggregateSHA256 || bundle.CacheBytes != 361820672 {
		return NoiseRefiner0ModuleContract{}, fmt.Errorf("noise_refiner.0 module requires validated M0.75 cache")
	}
	if !validSHA256(semanticSourceSHA256) {
		return NoiseRefiner0ModuleContract{}, fmt.Errorf("noise_refiner.0 semantic source identity must be a lowercase SHA-256")
	}
	if deviceCapabilityRoute == "" {
		return NoiseRefiner0ModuleContract{}, fmt.Errorf("noise_refiner.0 device capability route is required")
	}
	portfolio := append([]NoiseRefiner0ShaderIdentity(nil), shaders...)
	sort.Slice(portfolio, func(i, j int) bool { return portfolio[i].ID < portfolio[j].ID })
	for index, shader := range portfolio {
		if shader.ID == "" || shader.PipelineID == "" || !validSHA256(shader.SHA256) {
			return NoiseRefiner0ModuleContract{}, fmt.Errorf("noise_refiner.0 shader portfolio entry %d is incomplete", index)
		}
		if index != 0 && portfolio[index-1].ID == shader.ID {
			return NoiseRefiner0ModuleContract{}, fmt.Errorf("noise_refiner.0 shader portfolio has duplicate ID %q", shader.ID)
		}
	}
	shaderParts := make([]string, 0, len(portfolio)*3)
	for _, shader := range portfolio {
		shaderParts = append(shaderParts, shader.ID, shader.SHA256, shader.PipelineID)
	}
	shaderID := noiseRefiner0Identity(shaderParts...)
	weightID := noiseRefiner0Identity(NoiseRefiner0SourceCheckpointSHA256, NoiseRefiner0CacheAggregateSHA256)
	modelID := noiseRefiner0Identity(
		"Tongyi-MAI/Z-Image-Turbo", NoiseRefiner0OracleRevision, NoiseRefiner0TopologyVersion,
		NoiseRefiner0SourceCheckpointSHA256, NoiseRefiner0CacheAggregateSHA256, NoiseRefiner0PrecisionPolicy,
		semanticSourceSHA256,
	)
	executionID := noiseRefiner0Identity(modelID, weightID, shaderID, deviceCapabilityRoute, NoiseRefiner0RuntimeVersion)
	return NoiseRefiner0ModuleContract{
		ModelID:               "Tongyi-MAI/Z-Image-Turbo",
		ModelRevision:         NoiseRefiner0OracleRevision,
		SourceCheckpointSHA:   NoiseRefiner0SourceCheckpointSHA256,
		CacheAggregateSHA:     NoiseRefiner0CacheAggregateSHA256,
		TopologyVersion:       NoiseRefiner0TopologyVersion,
		PrecisionPolicy:       NoiseRefiner0PrecisionPolicy,
		SemanticSourceSHA256:  semanticSourceSHA256,
		DeviceCapabilityRoute: deviceCapabilityRoute,
		RuntimeVersion:        NoiseRefiner0RuntimeVersion,
		ShaderPortfolio:       portfolio,
		ModelContractID:       modelID,
		WeightID:              weightID,
		ShaderPortfolioID:     shaderID,
		ExecutionReplayID:     executionID,
	}, nil
}
