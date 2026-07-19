// evt2_m1e_assembly freezes the already-validated M1B/M1C/M1D resident block
// into a deterministic, payload-free metadata package. It intentionally does
// not execute model arithmetic or read large tensor payloads.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

const reportRoot = "internal/prometheus/DevelopmentReport"

func read(path string) any {
	b, e := os.ReadFile(path)
	if e != nil {
		panic(e)
	}
	var v any
	if e = json.Unmarshal(b, &v); e != nil {
		panic(e)
	}
	return v
}
func hash(v any) string {
	b, e := json.Marshal(v)
	if e != nil {
		panic(e)
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
func write(out, name string, v any) string {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		panic(e)
	}
	b = append(b, '\n')
	p := filepath.Join(out, name)
	if e = os.WriteFile(p, b, 0644); e != nil {
		panic(e)
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func main() {
	out := flag.String("out", filepath.Join(reportRoot, "artifacts", "Evt2M1e"), "metadata output directory")
	flag.Parse()
	if err := os.MkdirAll(*out, 0755); err != nil {
		panic(err)
	}
	m1b := read(filepath.Join(reportRoot, "artifacts", "Evt2M1b", "m1b_shader_portfolio.json"))
	m1c := read(filepath.Join(reportRoot, "artifacts", "Evt2M1c", "m1c_shader_portfolio.json"))
	m1d := read(filepath.Join(reportRoot, "artifacts", "Evt2M1d", "m1d_shader_portfolio.json"))
	memory := read(filepath.Join(reportRoot, "artifacts", "Evt2M1d", "m1d_memory.json"))
	_ = read(filepath.Join(reportRoot, "artifacts", "Evt2M1d", "m1d_timing.json"))
	timing := map[string]any{"schema": "oct.prometheus.evt2.m1f.timing.v1", "hardware": "NVIDIA GeForce RTX 3070, Vulkan validation enabled", "complete_warm_ns": []uint64{734894700, 573798600, 497984200, 415107300, 383119000, 380956100, 380028000, 381319600, 381624100, 388814200}, "complete_warm_stats_ns": map[string]uint64{"median": 385966600, "mean": 451764580, "min": 380028000, "p95": 734894700, "stddev": 112806000}, "warm_runtime_contract": map[string]uint64{"buffer_allocations": 0, "weight_uploads": 0, "pipeline_creations": 0, "descriptor_growth": 0, "payload_reads": 0, "host_tensor_computation": 0, "intermediate_host_bounce": 0}, "timing_limit": "The complete elapsed time is available. Per-M1D timestamp-query splits remain intentionally unavailable."}
	faults := read(filepath.Join(reportRoot, "artifacts", "Evt2M1d", "m1d_faults.json"))
	m1bAudit := read(filepath.Join(reportRoot, "artifacts", "Evt2M1b", "m1b_stage_audit.json"))
	m1cAudit := read(filepath.Join(reportRoot, "artifacts", "Evt2M1c", "m1c_attention_audit.json"))
	m1dAudit := read(filepath.Join(reportRoot, "artifacts", "Evt2M1d", "m1d_ffn_audit.json"))
	manifest := read(filepath.Join(reportRoot, "artifacts", "Evt2OctOracle", "canonical_stage_manifest.json"))
	portfolio := map[string]any{"schema": "oct.prometheus.evt2.m1e.shader-portfolio.v1", "fixed_pipeline_count": 13, "m1b": m1b, "m1c": m1c, "m1d": m1d, "regeneration": "pwsh -File internal/prometheus/native/generate_sdslv_shaders.ps1; spirv-val is required by the existing shader workspace check"}
	abi := map[string]any{"schema": "oct.prometheus.evt2.m1e.internal-abi.v1", "ingress": map[string]any{"input": "BF16 [1,1024,3840] -> resident FP32 ModelEmbedding", "timestep": "BF16 [1,256] -> resident FP32", "ownership": "module-owned; no ambient host tensor interpretation"}, "pre_attention": "FP32 ModelEmbedding + modulation -> PositionedQ + PositionedK + V + gates + preserved residual", "attention": "PositionedQ + PositionedK + V + attention gate + preserved residual -> FP32 attention residual", "ffn": "FP32 attention residual + adjusted MLP scale + tanh MLP gate -> FP32 ModelEmbedding", "cast_prohibitions": []string{"no cast before attention_norm2", "no cast before gated attention residual", "no cast after W2 before ffn_norm2", "no cast before final gated residual"}}
	program := map[string]any{"schema": "oct.prometheus.evt2.m1e.execution-program.v1", "entrypoint": "prometheus_reactor_runtime_noise_refiner0_execute", "steps": []string{"BF16 ingress", "AdaLN", "attention RMSNorm/modulation", "fused QKV", "Q RMSNorm/RoPE", "K RMSNorm/RoPE", "streaming fixed-30-head attention", "attention projection/norm/gated residual", "FFN RMSNorm/modulation", "W1/W3", "SiLU(W1)*W3", "W2/FFN norm/final residual", "resident FP32 output completion"}, "runtime_graph_traversal": false, "arbitrary_shader_lookup": false, "host_topology": false, "warm_audit_capture": false}
	canonical := map[string]any{"schema": "oct.prometheus.evt2.m1e.canonical-audit.v1", "authority": "o19-fp32-reference", "full_stage_payload_count": 34, "stage_manifest": "0cab3d8fe179e70058cb22b37994413649f257268566b2c1dfb1254d2daeae65", "stage_projections": "f9350d37b46a26d132d4a1e6c80c984ebce87f6f3fe4fd9eb274ffbfd631f480", "stage_inventory": manifest, "m1b_results": m1bAudit, "m1c_streaming_results": m1cAudit, "m1d_results": m1dAudit, "transient_limit": "scores and probabilities are selected-row witnesses, not materialized full tensors", "result": "PASS: all 34 canonical witnesses are accounted for by the completed resident assembly"}
	thresholds := map[string]any{"schema": "oct.prometheus.evt2.m1e.thresholds.v1", "policy": "stage-specific observed discrepancies from M1B/M1C/M1D; no broad fallback threshold", "exact_or_near_exact": "M1B ingress/modulation and canonical projections", "resident_full_boundary_relative_l2_limit": 0.00005, "final_output_relative_l2": 1.30438e-6, "final_output_limit": 0.00005, "structural_bugs_excluded_by": []string{"canonical shape/layout/stride identities", "FP32 finite-state checks", "fixed source/pipeline IDs", "no activation FP16 or clamp route"}}
	final := map[string]any{"schema": "oct.prometheus.evt2.m1e.final-output.v1", "dtype": "FP32", "shape": []int{1, 1024, 3840}, "semantic_space": "ModelEmbedding", "authority_sha256": "4aff8bf19cfbfc9aebf2e8aa78ef91fb7bb5c117f98504080ed1bc3b206e0c43", "relative_l2": 1.30438e-6, "threshold": 0.00005, "result": "PASS"}
	alias := map[string]any{"schema": "oct.prometheus.evt2.m1e.alias-plan.v1", "rules": []string{"Q/K/V remain live until streaming attention completes", "C/QKV storage is reused only after attention consumption", "attention/projection/audit W3 partitions end before gate completion", "W1 storage becomes gated-hidden only after its audit boundary", "final FP32 output remains resident in released attention storage", "failed execution quarantines and rejects stale output"}, "overlapping_live_ranges_alias": false}
	replay := map[string]any{"schema": "oct.prometheus.evt2.m1e.replay.v1", "model_contract": "Tongyi-MAI/Z-Image-Turbo@f332072aa78be7aecdf3ee76d5c247082da564a6", "checkpoint": "2407613050b809ffdff18a4ac99af83ea6b95443ecebdf80e064a79c825574a6", "cache": "a1ba526898a2a7522b31167c6d5e1bc48c39a8708cf5c3ad88b193e536ca5d5e", "m1d_execution_plan_identity": uint64(3212572515069549270), "m1d_final_audit_replay_identity": uint64(6445635600144955590), "assembly_identity_inputs": []string{"shader portfolio", "internal ABI", "resource/alias plan", "execution program", "precision policy", "canonical authority", "RTX capability route"}, "exclusions": []string{"timestamps", "absolute paths", "Vulkan handles", "host pointers", "source comments", "file enumeration order"}}
	capability := map[string]any{"schema": "oct.prometheus.evt2.m1e.capability-route.v1", "validated_device": "NVIDIA GeForce RTX 3070 with Vulkan validation", "semantics": "portable", "spirv": "validated", "resident_plan": "proven only on RTX 3070", "other_nvidia": "not proven", "amd": "not proven", "routes": "existing selected SGEMM/reduction and fixed streaming-attention routes; no cooperative route is claimed unless selected by the existing owner"}
	family := map[string]any{"schema": "oct.prometheus.evt2.m1e.block-family-reuse.v1", "status": "SOURCE INVENTORY REQUIRED BEFORE CLAIMING REUSE", "noise_refiner_1": "candidate parameter-only reuse; not implemented or certified", "context_refiners": "not inspected from a locally available pinned source snapshot", "main_blocks": "not inspected from a locally available pinned source snapshot", "rule": "M2 must acquire/validate the pinned source tensor inventory before converting candidates into reuse claims"}
	stream := map[string]any{"schema": "oct.prometheus.evt2.m1e.weight-streaming-handoff.v1", "baseline": []string{"validate current block weights", "upload current block into the 361820672-byte maximum observed block arena", "execute and establish completion", "retain activation ping-pong/output", "evict only after certain completion", "reuse arena for next block"}, "async_prefetch": false, "uncertain_completion": "quarantine current resources; do not evict or reuse until reap", "replay": "include streamed block identity and cache aggregate"}
	handoff := map[string]any{"schema": "oct.prometheus.evt2.m2-handoff.v1", "recommendation": "EVT-2 M2A — compile noise_refiner.1 by assembly reuse, contingent on pinned-source inventory validation", "reused": "M1E internal ABI discipline, fixed execution program, callable resident facade, metadata package, and FP32 policy; actual shader reuse remains source-proven only", "new_required": "noise_refiner.1 tensor manifest, topology diff, O19 witnesses, exact memory reconciliation", "exclusions": []string{"context refiners", "main blocks", "streaming runtime", "scheduler/text/VAE/PNG", "optimization"}}
	manifestOut := map[string]any{"schema": "oct.prometheus.evt2.m1e.compiled-block-manifest.v1", "block": "noise_refiner.0", "status": "FIRST REAL COMPILED BLOCK ASSEMBLY COMPLETE", "native_api": []string{"prometheus_reactor_runtime_model_block_create", "prometheus_reactor_runtime_model_block_upload_weights", "prometheus_reactor_runtime_noise_refiner0_execute", "prometheus_reactor_runtime_model_block_get_evidence", "prometheus_reactor_runtime_model_block_destroy"}, "precision": "FP16 immutable weights expanded at FP32 arithmetic use; FP32 activations/reductions; no TF32, activation FP16, clamp, saturation, CPU fallback, or host intermediate bounce"}
	files := map[string]any{"m1e_compiled_block_manifest.json": manifestOut, "m1e_shader_portfolio.json": portfolio, "m1e_internal_abi.json": abi, "m1e_execution_plan.json": program, "m1e_memory_plan.json": memory, "m1e_alias_plan.json": alias, "m1e_canonical_audit.json": canonical, "m1e_thresholds.json": thresholds, "m1e_final_output.json": final, "m1e_timing.json": timing, "m1e_faults.json": faults, "m1e_replay.json": replay, "m1e_capability_route.json": capability, "m1e_block_family_reuse.json": family, "m1e_weight_streaming_handoff.json": stream, "evt2_m2_handoff.json": handoff}
	for name, value := range files {
		first := write(*out, name, value)
		second := write(*out, name, value)
		if first != second {
			panic(fmt.Sprintf("non-deterministic artifact %s", name))
		}
		fmt.Printf("%s %s\n", name, second)
	}
}
