// evt2_m1a_artifacts writes deterministic, payload-free M1a metadata from the
// validated local EVT-2 cache and oracle roots. It never copies tensor bytes.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

import "github.com/yuechen-li-dev/oct/internal/prometheus/zimage"

const outputDirectory = "internal/prometheus/DevelopmentReport/artifacts/Evt2M1a"

func writeJSON(path string, value any) error {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return os.WriteFile(path, encoded, 0o644)
}

func main() {
	cacheRoot := os.Getenv("OCT_EVT2_CACHE")
	oracleRoot := os.Getenv("OCT_EVT2_ORACLE")
	bundle, err := zimage.LoadNoiseRefiner0PayloadBundle(zimage.NoiseRefiner0PayloadPaths{CacheRoot: cacheRoot, OracleRoot: oracleRoot})
	if err != nil {
		panic(fmt.Errorf("load EVT-2 M1a payload contract: %w", err))
	}
	contract, err := zimage.NewNoiseRefiner0ModuleContract(bundle, zimage.NoiseRefiner0ResidentProofShaderSHA256, "rtx-3070-vulkan", []zimage.NoiseRefiner0ShaderIdentity{{
		ID:         zimage.NoiseRefiner0ResidentProofShaderID,
		SHA256:     zimage.NoiseRefiner0ResidentProofShaderSHA256,
		PipelineID: zimage.NoiseRefiner0ResidentProofPipelineID,
	}})
	if err != nil {
		panic(fmt.Errorf("create EVT-2 M1a module contract: %w", err))
	}
	plan, err := zimage.NewNoiseRefiner0ResidentBlockPlan(bundle, contract)
	if err != nil {
		panic(fmt.Errorf("create EVT-2 M1a resident plan: %w", err))
	}
	if err := os.MkdirAll(outputDirectory, 0o755); err != nil {
		panic(err)
	}
	contractDocument := struct {
		Schema       string                                `json:"schema"`
		Model        zimage.NoiseRefiner0ModuleContract    `json:"model"`
		ResidentPlan zimage.NoiseRefiner0ResidentBlockPlan `json:"resident_plan"`
		NativeABI    struct {
			Header      string   `json:"header"`
			EntryPoints []string `json:"entry_points"`
			ClosedSteps []string `json:"closed_steps"`
		} `json:"native_abi"`
	}{Schema: "oct.prometheus.evt2.m1a.resident-model-block.v1", Model: contract, ResidentPlan: plan}
	contractDocument.NativeABI.Header = "internal/prometheus/native/reactor_api.h"
	contractDocument.NativeABI.EntryPoints = []string{"create", "upload_weights", "execute", "get_evidence", "destroy"}
	contractDocument.NativeABI.ClosedSteps = []string{"bind-pipeline", "bind-declared-resources", "push-declared-constants", "dispatch", "barrier", "audit-copy", "output-copy"}
	memoryDocument := struct {
		Schema string                                 `json:"schema"`
		Memory zimage.NoiseRefiner0ResidentMemoryPlan `json:"memory"`
		Proof  struct {
			WeightBindingCount uint32 `json:"weight_binding_count"`
			ColdBufferCount    uint32 `json:"cold_buffer_count"`
			WarmBufferAllocs   uint32 `json:"warm_buffer_allocations"`
			PipelineCreations  uint32 `json:"pipeline_creations"`
			DescriptorSets     uint32 `json:"descriptor_sets"`
		} `json:"resident_proof_expectation"`
	}{Schema: "oct.prometheus.evt2.m1a.memory.v1", Memory: plan.Memory}
	memoryDocument.Proof.WeightBindingCount = 13
	memoryDocument.Proof.ColdBufferCount = 20
	memoryDocument.Proof.WarmBufferAllocs = 0
	memoryDocument.Proof.PipelineCreations = 1
	memoryDocument.Proof.DescriptorSets = 1
	replayDocument := struct {
		Schema                     string   `json:"schema"`
		ModelContractIdentity      string   `json:"model_contract_identity"`
		WeightIdentity             string   `json:"weight_identity"`
		ShaderPortfolioIdentity    string   `json:"shader_portfolio_identity"`
		ExecutionPlanIdentity      string   `json:"execution_plan_identity"`
		ResidentReplaySeedIdentity string   `json:"resident_replay_seed_identity"`
		ReplayInputs               []string `json:"replay_inputs"`
	}{
		Schema:                     "oct.prometheus.evt2.m1a.replay.v1",
		ModelContractIdentity:      contract.ModelContractID,
		WeightIdentity:             contract.WeightID,
		ShaderPortfolioIdentity:    contract.ShaderPortfolioID,
		ExecutionPlanIdentity:      plan.ExecutionPlanID,
		ResidentReplaySeedIdentity: plan.ResidentReplaySeedID,
		ReplayInputs:               []string{"model-contract", "weight-bundle", "shader-portfolio", "fixed-plan", "precision-policy", "capability-route", "input-identity", "audit-mode", "runtime-version"},
	}
	faultDocument := struct {
		Schema string `json:"schema"`
		Faults []struct {
			Name       string `json:"name"`
			DetailCode int32  `json:"detail_code"`
			Expected   string `json:"expected"`
		} `json:"faults"`
	}{Schema: "oct.prometheus.evt2.m1a.faults.v1"}
	for _, fault := range []struct {
		name string
		code int32
		text string
	}{
		{"malformed-fixed-plan", -6901, "reject before resource allocation"},
		{"wrong-or-missing-weight-binding", -6904, "reject immutable bundle before upload"},
		{"pipeline-create", -6906, "clean partial state and permit later creation"},
		{"weight-upload", -6907, "reject bundle and allow destruction"},
		{"queue-submit", -6909, "reject stale output and recover after one-shot fault"},
		{"completion-uncertain", -6910, "quarantine, reap fence, then recover"},
		{"audit-copy", -6911, "reject output and preserve no-stale-output rule"},
	} {
		faultDocument.Faults = append(faultDocument.Faults, struct {
			Name       string `json:"name"`
			DetailCode int32  `json:"detail_code"`
			Expected   string `json:"expected"`
		}{fault.name, fault.code, fault.text})
	}
	handoffDocument := struct {
		Schema    string   `json:"schema"`
		Milestone string   `json:"milestone"`
		Reuse     []string `json:"reuse"`
		Add       []string `json:"add"`
		Absent    []string `json:"deliberately_absent"`
	}{
		Schema:    "oct.prometheus.evt2.m1b-handoff.v1",
		Milestone: "EVT-2 M1b - modulation normalization and RoPE",
		Reuse: []string{
			"validated 13-tensor cache contract and resident bindings",
			"fixed model-block create/upload/execute/audit/destroy lifecycle",
			"closed production SDSL-V portfolio registration and shader identity",
			"resident memory declaration, replay identity, quarantine and reap behavior",
		},
		Add: []string{
			"timestep linear and AdaLN split/gate/scale shader steps",
			"attention-input RMSNorm/modulation and Q/K RMSNorm",
			"three-axis RoPE semantic-space transitions and bounded stage audits",
			"M0.5 witnesses for AdaLN, normalized input, normalized Q/K and positioned Q/K",
		},
		Absent: []string{"fused 30-head QKV", "attention score/softmax/value aggregation", "output projection and residual", "10240-wide FFN", "scheduler and remaining model blocks"},
	}
	for name, document := range map[string]any{
		"resident_model_block_contract.json": contractDocument,
		"resident_model_block_memory.json":   memoryDocument,
		"resident_model_block_replay.json":   replayDocument,
		"resident_model_block_faults.json":   faultDocument,
		"evt2_m1b_handoff.json":              handoffDocument,
	} {
		if err := writeJSON(filepath.Join(outputDirectory, name), document); err != nil {
			panic(fmt.Errorf("write %s: %w", name, err))
		}
	}
}
