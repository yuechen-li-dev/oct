package main

// This file is the closed resolver for the current Z-Image NoiseRefiner
// assembly.  It is intentionally part of lock generation rather than a native
// table: native code consumes only the generated result below.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	auditCeilingBytes = 47186176
	auditAlignment    = 256
	auditSummaryBytes = 256
	acceptedLockID    = "71ef202b4e34b562bd0d8526d1e0c674640cbba02fb7c484d8dadf981c8b226e"
)

type auditStageSpec struct {
	ID       uint32
	Name     string
	Policy   string
	Source   string
	Base     uint32
	Count    uint32
	Layout   string
	Capture  string
	Lifetime string
	Keys     []uint32
}

func auditKeys(count uint32, extra ...uint32) []uint32 {
	seen := map[uint32]bool{}
	keys := make([]uint32, 0, 8)
	for _, key := range append([]uint32{0, count - 1, count / 2}, extra...) {
		if key < count && !seen[key] && len(keys) < 15 {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	return keys
}

func resolvedAuditStages() []auditStageSpec {
	const model = 1024 * 3840
	const hidden = 1024 * 10240
	const vector = 3840
	const qkv = 1024 * 11520
	return []auditStageSpec{
		{1, "timestep_linear", "Full", "AdalnProjection", 0, 15360, "Vector", "after_adaln", "before_attention_norm", auditKeys(15360, 3839, 7680, 11519)},
		{2, "attention_scale_raw", "Full", "AdalnProjection", 0, vector, "Vector", "after_adaln", "before_attention_norm", auditKeys(vector)},
		{3, "attention_scale_adjusted", "Full", "AttentionScale", 0, vector, "Vector", "after_adaln", "before_attention_norm", auditKeys(vector)},
		{4, "attention_gate_raw", "Full", "AdalnProjection", vector, vector, "Vector", "after_adaln", "before_attention_residual", auditKeys(vector)},
		{5, "attention_gate_tanh", "Full", "AttentionGate", 0, vector, "Vector", "after_adaln", "before_attention_residual", auditKeys(vector)},
		{6, "mlp_scale_raw", "Full", "AdalnProjection", 2 * vector, vector, "Vector", "after_adaln", "before_ffn_norm", auditKeys(vector)},
		{7, "mlp_scale_adjusted", "Full", "MlpScale", 0, vector, "Vector", "after_adaln", "before_ffn_norm", auditKeys(vector)},
		{8, "mlp_gate_raw", "Full", "AdalnProjection", 3 * vector, vector, "Vector", "after_adaln", "before_final_residual", auditKeys(vector)},
		{9, "mlp_gate_tanh", "Full", "MlpGate", 0, vector, "Vector", "after_adaln", "before_final_residual", auditKeys(vector)},
		{10, "attention_norm", "ProjectionAndSummary", "NormAudit", 0, model, "TokenChannel", "after_attention_norm", "before_qkv", auditKeys(model, vector-1, model-vector)},
		{11, "attention_modulated", "ProjectionAndSummary", "Modulated", 0, model, "TokenChannel", "after_attention_norm", "before_qkv", auditKeys(model, vector-1, model-vector)},
		{12, "qkv", "ProjectionAndSummary", "Qkv", 0, qkv, "FusedQkv", "after_qkv", "before_q_rope", auditKeys(qkv, model-1, model, 2*model-1, 2*model)},
		{13, "q", "ProjectionAndSummary", "Qkv", 0, model, "TokenHeadChannel", "after_qkv", "before_q_rope", auditKeys(model, 127, 128, model-vector)},
		{14, "k", "ProjectionAndSummary", "Qkv", model, model, "TokenHeadChannel", "after_qkv", "before_k_rope", auditKeys(model, 127, 128, model-vector)},
		{15, "v", "ProjectionAndSummary", "Qkv", 2 * model, model, "TokenHeadChannel", "after_qkv", "before_attention", auditKeys(model, 127, 128, model-vector)},
		{16, "q_norm", "ProjectionAndSummary", "NormAudit", 0, model, "TokenHeadChannel", "after_q_norm", "before_q_rope", auditKeys(model, 127, 128, model-vector)},
		{17, "k_norm", "ProjectionAndSummary", "NormAudit", 0, model, "TokenHeadChannel", "after_k_norm", "before_k_rope", auditKeys(model, 127, 128, model-vector)},
		{18, "q_rope", "ProjectionAndSummary", "Qkv", 0, model, "TokenHeadChannel", "after_q_rope", "before_attention", auditKeys(model, 63, 64, 127, 128, model-vector)},
		{19, "k_rope", "ProjectionAndSummary", "Qkv", model, model, "TokenHeadChannel", "after_k_rope", "before_attention", auditKeys(model, 63, 64, 127, 128, model-vector)},
		{20, "attention_aggregation", "ProjectionAndSummary", "Attention", 0, model, "TokenHeadChannel", "after_attention", "before_projection", auditKeys(model, 127, 128, model-vector)},
		{21, "attention_projection", "ProjectionAndSummary", "AttentionProjection", 0, model, "TokenChannel", "after_projection", "before_attention_residual", auditKeys(model, vector-1, model-vector)},
		{22, "attention_residual", "ProjectionAndSummary", "AttentionResidual", 0, model, "TokenChannel", "after_attention_residual", "before_ffn_norm", auditKeys(model, vector-1, model-vector)},
		{23, "ffn_norm", "ProjectionAndSummary", "NormAudit", 0, model, "TokenChannel", "after_ffn_norm", "before_w1_w3", auditKeys(model, vector-1, model-vector)},
		{24, "ffn_modulated", "ProjectionAndSummary", "Modulated", 0, model, "TokenChannel", "after_ffn_norm", "before_w1_w3", auditKeys(model, vector-1, model-vector)},
		{25, "w1", "ProjectionAndSummary", "Qkv", 0, hidden, "FfnHidden", "after_w1_w3", "before_gate", auditKeys(hidden, 10239, hidden-10240)},
		// W3 is the assembly's declared three-view alias.  The generated native
		// consumer expands this one logical resource into its three fixed views;
		// no arbitrary scatter/stride expression is admitted.
		{26, "w3", "ProjectionAndSummary", "W3DeclaredViews", 0, hidden, "FfnHidden", "after_w1_w3", "before_gate", auditKeys(hidden, 10239, hidden-10240)},
		{27, "ffn_gated_hidden", "ProjectionAndSummary", "Qkv", 0, hidden, "FfnHidden", "after_gate", "before_w2", auditKeys(hidden, 10239, hidden-10240)},
		{28, "w2", "ProjectionAndSummary", "Input", 0, model, "TokenChannel", "after_w2", "before_final_residual", auditKeys(model, vector-1, model-vector)},
		{29, "final_output", "ProjectionAndSummary", "Attention", 0, model, "TokenChannel", "after_final_residual", "resident_output_replaced", auditKeys(model, vector-1, model-vector)},
	}
}

// ContextRefiner has no AdaLN, gates, or BF16 ingress. Its sixteen persistent
// witnesses plus the two short-attention transient rows are independently
// scheduled from the NoiseRefiner profile below.
func resolvedContextAuditStages() []auditStageSpec {
	const model = 32 * 3840
	const hidden = 32 * 10240
	return []auditStageSpec{
		{1, "context_embedding_input", "ProjectionAndSummary", "Input", 0, model, "TokenChannel", "after_context_input", "before_attention_norm", auditKeys(model, 3839)},
		{2, "attention_norm", "ProjectionAndSummary", "NormAudit", 0, model, "TokenChannel", "after_attention_norm", "before_qkv", auditKeys(model, 3839)},
		{3, "qkv", "ProjectionAndSummary", "Qkv", 0, 3 * model, "FusedQkv", "after_qkv", "before_q_rope", auditKeys(3*model, model-1, model, 2*model)},
		{4, "q_norm", "ProjectionAndSummary", "NormAudit", 0, model, "TokenHeadChannel", "after_q_norm", "before_q_rope", auditKeys(model, 127, 128)},
		{5, "k_norm", "ProjectionAndSummary", "NormAudit", 0, model, "TokenHeadChannel", "after_k_norm", "before_k_rope", auditKeys(model, 127, 128)},
		{6, "q_rope", "ProjectionAndSummary", "Qkv", 0, model, "TokenHeadChannel", "after_q_rope", "before_attention", auditKeys(model, 63, 64, 127, 128)},
		{7, "k_rope", "ProjectionAndSummary", "Qkv", model, model, "TokenHeadChannel", "after_k_rope", "before_attention", auditKeys(model, 63, 64, 127, 128)},
		{8, "attention_aggregation", "ProjectionAndSummary", "Attention", 0, model, "TokenHeadChannel", "after_attention", "before_projection", auditKeys(model, 127, 128)},
		{9, "attention_projection", "ProjectionAndSummary", "AttentionProjection", 0, model, "TokenChannel", "after_projection", "before_attention_residual", auditKeys(model, 3839)},
		{10, "attention_residual", "ProjectionAndSummary", "AttentionResidual", 0, model, "TokenChannel", "after_attention_residual", "before_ffn_norm", auditKeys(model, 3839)},
		{11, "ffn_norm", "ProjectionAndSummary", "Modulated", 0, model, "TokenChannel", "after_ffn_norm", "before_w1_w3", auditKeys(model, 3839)},
		{12, "w1", "ProjectionAndSummary", "Qkv", 0, hidden, "FfnHidden", "after_w1_w3", "before_gate", auditKeys(hidden, 10239)},
		{13, "w3", "ProjectionAndSummary", "W3DeclaredViews", 0, hidden, "FfnHidden", "after_w1_w3", "before_gate", auditKeys(hidden, 10239)},
		{14, "ffn_gated_hidden", "ProjectionAndSummary", "Qkv", 0, hidden, "FfnHidden", "after_gate", "before_w2", auditKeys(hidden, 10239)},
		{15, "w2", "ProjectionAndSummary", "AttentionProjection", 0, model, "TokenChannel", "after_w2", "before_final_residual", auditKeys(model, 3839)},
		{16, "final_output", "ProjectionAndSummary", "Attention", 0, model, "TokenChannel", "after_final_residual", "resident_output_replaced", auditKeys(model, 3839)},
	}
}

func alignAudit(value uint64) uint64 { return (value + auditAlignment - 1) &^ (auditAlignment - 1) }

func auditScheduleProjection(lock []byte) (string, string, error) {
	header, layout, err := auditScheduleProjectionForStages(lock, resolvedAuditStages(), auditCeilingBytes)
	if err != nil {
		return "", "", err
	}
	contextHeader, contextLayout, err := contextAuditScheduleProjection(lock)
	if err != nil {
		return "", "", err
	}
	header = strings.Replace(header, "\n#endif", "\n"+contextHeader+"\n#endif", 1)
	trimmed := strings.TrimSuffix(strings.TrimSpace(layout), "}")
	layout = trimmed + ",\n  \"context_refiner\": " + contextLayout + "\n}\n"
	return header, layout, nil
}

func contextAuditScheduleProjection(lock []byte) (string, string, error) {
	const ceiling = uint64(1048576)
	const model = uint32(32 * 3840)
	const hidden = uint32(32 * 10240)
	type renderedStage struct {
		auditStageSpec
		ProjectionOffset  uint32 `json:"projection_key_offset"`
		DestinationOffset uint64 `json:"audit_destination_offset"`
	}
	var offset uint64
	keys := make([]uint32, 0, 128)
	rendered := make([]renderedStage, 0, 16)
	for _, stage := range resolvedContextAuditStages() {
		if stage.ID == 0 || stage.Count == 0 || capturePoint(stage.Capture) == 0 ||
			lifetimePoint(stage.Lifetime) < capturePoint(stage.Capture) || sourceResource(stage.Source) == 0 ||
			layoutKind(stage.Layout) == 0 || uint64(stage.Base)+uint64(stage.Count) > uint64(contextSourceElementCount(stage.Source, model, hidden)) {
			return "", "", fmt.Errorf("malformed ContextRefiner audit stage %s", stage.Name)
		}
		for _, key := range stage.Keys {
			if key >= stage.Count {
				return "", "", fmt.Errorf("invalid ContextRefiner projection key %d at %s", key, stage.Name)
			}
		}
		offset = alignAudit(offset)
		if offset+auditSummaryBytes > ceiling {
			return "", "", fmt.Errorf("ContextRefiner audit ceiling exceeded at %s", stage.Name)
		}
		rendered = append(rendered, renderedStage{stage, uint32(len(keys)), offset})
		keys = append(keys, stage.Keys...)
		offset += auditSummaryBytes
	}
	offset = alignAudit(offset)
	transientOffset := offset
	if offset+64*4 > ceiling {
		return "", "", fmt.Errorf("ContextRefiner transient attention witness exceeds audit ceiling")
	}
	offset += 64 * 4
	var header strings.Builder
	fmt.Fprintf(&header, "#define PROM_ZIMAGE_TURBO_CONTEXT_AUDIT_ARENA_BYTES %du\n", ceiling)
	fmt.Fprintf(&header, "#define PROM_ZIMAGE_TURBO_CONTEXT_AUDIT_REQUIRED_BYTES %du\n", offset)
	fmt.Fprintf(&header, "#define PROM_ZIMAGE_TURBO_CONTEXT_AUDIT_TRANSIENT_ATTENTION_OFFSET %du\n", transientOffset)
	fmt.Fprintln(&header, "#define PROM_ZIMAGE_TURBO_CONTEXT_AUDIT_TRANSIENT_ATTENTION_BYTES 256u")
	fmt.Fprintf(&header, "#define PROM_ZIMAGE_TURBO_CONTEXT_AUDIT_STAGE_COUNT %du\n", len(rendered))
	fmt.Fprintln(&header, "static const uint32_t k_prom_zimage_turbo_context_audit_projection_keys[] = {")
	for index, key := range keys {
		if index%8 == 0 {
			fmt.Fprint(&header, "  ")
		}
		fmt.Fprintf(&header, "%du,", key)
		if index%8 == 7 {
			fmt.Fprintln(&header)
		}
	}
	if len(keys)%8 != 0 {
		fmt.Fprintln(&header)
	}
	fmt.Fprintln(&header, "};")
	fmt.Fprintln(&header, "static const prom_zimage_turbo_audit_schedule_entry k_prom_zimage_turbo_context_audit_schedule[] = {")
	for _, stage := range rendered {
		fmt.Fprintf(&header, "  {PROM_ZIMAGE_TURBO_AUDIT_LOCK_ID,%du,%du,%du,%du,%du,%du,%du,%du,%du,%du,%du}, /* %s */\n", stage.ID, capturePolicy(stage.Policy), sourceResource(stage.Source), layoutKind(stage.Layout), stage.Base, stage.Count, stage.ProjectionOffset, len(stage.Keys), stage.DestinationOffset, capturePoint(stage.Capture), lifetimePoint(stage.Lifetime), stage.Name)
	}
	fmt.Fprintln(&header, "};")
	contextLayout, err := json.MarshalIndent(map[string]any{
		"profile": "ContextRefinerPersistentProjectionSummary.v1", "ceiling_bytes": ceiling,
		"required_bytes": offset, "slack_bytes": ceiling - offset, "stage_count": len(rendered),
		"projection_key_count": len(keys), "transient_attention_record": map[string]any{
			"audit_destination_offset": transientOffset, "bytes": 64 * 4,
			"description": "fixed first and last ContextRefiner attention rows; no score or probability tensor is materialized",
		},
	}, "", "  ")
	if err != nil {
		return "", "", err
	}
	return header.String(), string(contextLayout), nil
}

func auditScheduleProjectionForStages(lock []byte, stages []auditStageSpec, ceilingBytes uint64) (string, string, error) {
	text := string(lock)
	for _, required := range []string{
		"Schema: \"oct.sdslv.compiled-model-lock-tagon.v1\"",
		"NoiseRefinerPersistentProjectionSummary.v1",
		"AuditBudgetBytes: 47186176",
		"no repeated prefix replay",
		"ModelSemanticIdentity: \"sha256:ed6a7b765d0d7ece22a02a4416734fbd55d8d46684c8acb3dda1044b555bbeb7\"",
		"ProductionExecutionIdentity: \"sha256:6a883d7797b0ebe363bc5024cf8f21cc721e879558f1520be2c124779d515c25\"",
		"AuditProfileIdentity: \"sha256:df3a1340b6999daeb2769803a33d18a83151255bc2090e2621e4eb1c129afd11\"",
	} {
		if !strings.Contains(text, required) {
			return "", "", fmt.Errorf("lock missing audit authority %q", required)
		}
	}
	if acceptedLockID != "" && digest(lock) != acceptedLockID {
		return "", "", fmt.Errorf("foreign complete lock identity %s", digest(lock))
	}
	var offset uint64
	keys := make([]uint32, 0, 256)
	type renderedStage struct {
		auditStageSpec
		ProjectionOffset  uint32 `json:"projection_key_offset"`
		DestinationOffset uint64 `json:"audit_destination_offset"`
		Bytes             uint64 `json:"bytes"`
	}
	rendered := make([]renderedStage, 0, len(stages))
	seenStageIDs := make(map[uint32]bool, len(stages))
	for _, stage := range stages {
		if stage.ID == 0 || stage.Count == 0 || capturePoint(stage.Capture) == 0 ||
			lifetimePoint(stage.Lifetime) < capturePoint(stage.Capture) || sourceResource(stage.Source) == 0 ||
			layoutKind(stage.Layout) == 0 || len(stage.Keys) > 15 || seenStageIDs[stage.ID] {
			return "", "", fmt.Errorf("malformed audit stage %s", stage.Name)
		}
		seenStageIDs[stage.ID] = true
		for _, key := range stage.Keys {
			if key >= stage.Count {
				return "", "", fmt.Errorf("invalid projection key %d at %s", key, stage.Name)
			}
		}
		if uint64(stage.Base)+uint64(stage.Count) > uint64(sourceElementCount(stage.Source)) {
			return "", "", fmt.Errorf("source range out of bounds at %s", stage.Name)
		}
		offset = alignAudit(offset)
		bytes := uint64(auditSummaryBytes)
		if stage.Policy == "Full" {
			bytes = uint64(stage.Count) * 4
		}
		if offset+bytes > ceilingBytes {
			return "", "", fmt.Errorf("audit ceiling exceeded at %s", stage.Name)
		}
		rendered = append(rendered, renderedStage{stage, uint32(len(keys)), offset, bytes})
		keys = append(keys, stage.Keys...)
		offset += bytes
	}
	offset = alignAudit(offset)
	transientAttentionOffset := offset
	const transientAttentionBytes = 64 * 4
	if offset+transientAttentionBytes > ceilingBytes {
		return "", "", fmt.Errorf("audit ceiling exceeded at transient attention witness")
	}
	offset += transientAttentionBytes
	if len(rendered) != 29 {
		return "", "", fmt.Errorf("resolved schedule has %d stages, expected 29", len(rendered))
	}
	lockHash := sha256.Sum256(lock)
	var header strings.Builder
	fmt.Fprintln(&header, "/* Generated by tools/compiled_model_lock from lock-tagon.octagon. Do not edit. */")
	fmt.Fprint(&header, "#ifndef OCT_ZIMAGE_TURBO_RESOLVED_AUDIT_SCHEDULE_H\n#define OCT_ZIMAGE_TURBO_RESOLVED_AUDIT_SCHEDULE_H\n\n")
	fmt.Fprintf(&header, "#define PROM_ZIMAGE_TURBO_AUDIT_LOCK_ID 0x%sull\n", hex.EncodeToString(lockHash[:])[:16])
	fmt.Fprintf(&header, "#define PROM_ZIMAGE_TURBO_AUDIT_ARENA_BYTES %du\n", ceilingBytes)
	fmt.Fprintf(&header, "#define PROM_ZIMAGE_TURBO_AUDIT_REQUIRED_BYTES %du\n", offset)
	fmt.Fprintf(&header, "#define PROM_ZIMAGE_TURBO_AUDIT_SLACK_BYTES %du\n", ceilingBytes-offset)
	fmt.Fprintf(&header, "#define PROM_ZIMAGE_TURBO_AUDIT_TRANSIENT_ATTENTION_OFFSET %du\n", transientAttentionOffset)
	fmt.Fprintf(&header, "#define PROM_ZIMAGE_TURBO_AUDIT_TRANSIENT_ATTENTION_BYTES %du\n", transientAttentionBytes)
	fmt.Fprintf(&header, "#define PROM_ZIMAGE_TURBO_AUDIT_STAGE_COUNT %du\n", len(rendered))
	fmt.Fprintln(&header, "typedef struct prom_zimage_turbo_audit_schedule_entry { uint64_t authority_identity; uint32_t stage_id; uint32_t capture_policy; uint32_t source_resource; uint32_t layout_kind; uint32_t source_base_element; uint32_t element_count; uint32_t projection_key_offset; uint32_t projection_key_count; uint32_t audit_destination_offset; uint32_t legal_capture_point; uint32_t last_legal_lifetime_point; } prom_zimage_turbo_audit_schedule_entry;")
	fmt.Fprintln(&header, "enum { PROM_ZIMAGE_AUDIT_CAPTURE_FULL=1u, PROM_ZIMAGE_AUDIT_CAPTURE_PROJECTION=2u, PROM_ZIMAGE_AUDIT_CAPTURE_SUMMARY=3u, PROM_ZIMAGE_AUDIT_CAPTURE_PROJECTION_AND_SUMMARY=4u };")
	fmt.Fprintln(&header, "enum { PROM_ZIMAGE_AUDIT_LAYOUT_VECTOR=1u, PROM_ZIMAGE_AUDIT_LAYOUT_TOKEN_CHANNEL=2u, PROM_ZIMAGE_AUDIT_LAYOUT_FUSED_QKV=3u, PROM_ZIMAGE_AUDIT_LAYOUT_TOKEN_HEAD_CHANNEL=4u, PROM_ZIMAGE_AUDIT_LAYOUT_FFN_HIDDEN=5u };")
	fmt.Fprintln(&header, "enum { PROM_ZIMAGE_AUDIT_SOURCE_ADALN=1u, PROM_ZIMAGE_AUDIT_SOURCE_ATTENTION_SCALE=2u, PROM_ZIMAGE_AUDIT_SOURCE_ATTENTION_GATE=3u, PROM_ZIMAGE_AUDIT_SOURCE_MLP_SCALE=4u, PROM_ZIMAGE_AUDIT_SOURCE_MLP_GATE=5u, PROM_ZIMAGE_AUDIT_SOURCE_NORM=6u, PROM_ZIMAGE_AUDIT_SOURCE_MODULATED=7u, PROM_ZIMAGE_AUDIT_SOURCE_QKV=8u, PROM_ZIMAGE_AUDIT_SOURCE_ATTENTION=9u, PROM_ZIMAGE_AUDIT_SOURCE_ATTENTION_PROJECTION=10u, PROM_ZIMAGE_AUDIT_SOURCE_ATTENTION_RESIDUAL=11u, PROM_ZIMAGE_AUDIT_SOURCE_INPUT=12u, PROM_ZIMAGE_AUDIT_SOURCE_W3_DECLARED_VIEWS=13u };")
	fmt.Fprintln(&header, "static const uint32_t k_prom_zimage_turbo_audit_projection_keys[] = {")
	for index, key := range keys {
		if index%8 == 0 {
			fmt.Fprint(&header, "  ")
		}
		fmt.Fprintf(&header, "%du,", key)
		if index%8 == 7 {
			fmt.Fprintln(&header)
		}
	}
	if len(keys)%8 != 0 {
		fmt.Fprintln(&header)
	}
	fmt.Fprintln(&header, "};")
	fmt.Fprintln(&header, "static const prom_zimage_turbo_audit_schedule_entry k_prom_zimage_turbo_audit_schedule[] = {")
	for _, stage := range rendered {
		fmt.Fprintf(&header, "  {PROM_ZIMAGE_TURBO_AUDIT_LOCK_ID,%du,%du,%du,%du,%du,%du,%du,%du,%du,%du,%du}, /* %s: %s -> %s */\n", stage.ID, capturePolicy(stage.Policy), sourceResource(stage.Source), layoutKind(stage.Layout), stage.Base, stage.Count, stage.ProjectionOffset, len(stage.Keys), stage.DestinationOffset, capturePoint(stage.Capture), lifetimePoint(stage.Lifetime), stage.Name, stage.Capture, stage.Lifetime)
	}
	fmt.Fprintln(&header, "};\n#endif")
	layout := map[string]any{"schema": "oct.prometheus.evt2.audit-arena-layout.v1", "lock_identity": hex.EncodeToString(lockHash[:]), "ceiling_bytes": ceilingBytes, "required_bytes": offset, "slack_bytes": ceilingBytes - offset, "alignment_bytes": auditAlignment, "summary_record_bytes": auditSummaryBytes, "entries": rendered, "projection_key_count": len(keys), "transient_attention_record": map[string]any{"audit_destination_offset": transientAttentionOffset, "bytes": transientAttentionBytes, "description": "fixed 64-float first/last-head attention witness; no score or probability tensor is materialized"}}
	jsonBytes, err := json.MarshalIndent(layout, "", "  ")
	if err != nil {
		return "", "", err
	}
	return header.String(), string(append(jsonBytes, '\n')), nil
}

func capturePolicy(value string) uint32 {
	if value == "Full" {
		return 1
	}
	return 4
}
func sourceResource(value string) uint32 {
	return map[string]uint32{"AdalnProjection": 1, "AttentionScale": 2, "AttentionGate": 3, "MlpScale": 4, "MlpGate": 5, "NormAudit": 6, "Modulated": 7, "Qkv": 8, "Attention": 9, "AttentionProjection": 10, "AttentionResidual": 11, "Input": 12, "W3DeclaredViews": 13}[value]
}

func sourceElementCount(value string) uint32 {
	const model = 1024 * 3840
	return map[string]uint32{"AdalnProjection": 15360, "AttentionScale": 3840, "AttentionGate": 3840, "MlpScale": 3840, "MlpGate": 3840, "NormAudit": model, "Modulated": model, "Qkv": 3 * model, "Attention": model, "AttentionProjection": model, "AttentionResidual": model, "Input": model, "W3DeclaredViews": 1024 * 10240}[value]
}
func contextSourceElementCount(value string, model, hidden uint32) uint32 {
	return map[string]uint32{"NormAudit": model, "Modulated": model, "Qkv": 3 * model,
		"Attention": model, "AttentionProjection": model, "AttentionResidual": model,
		"Input": model, "W3DeclaredViews": hidden}[value]
}
func layoutKind(value string) uint32 {
	return map[string]uint32{"Vector": 1, "TokenChannel": 2, "FusedQkv": 3, "TokenHeadChannel": 4, "FfnHidden": 5}[value]
}

func capturePoint(value string) uint32 {
	return map[string]uint32{"after_context_input": 1, "after_adaln": 1, "after_attention_norm": 2, "after_qkv": 3, "after_q_norm": 4, "after_q_rope": 4, "after_k_norm": 5, "after_k_rope": 5, "after_attention": 6, "after_projection": 7, "after_attention_residual": 8, "after_ffn_norm": 9, "after_w1_w3": 10, "after_gate": 11, "after_w2": 12, "after_final_residual": 13}[value]
}

func lifetimePoint(value string) uint32 {
	return map[string]uint32{"before_attention_norm": 2, "before_qkv": 3, "before_q_rope": 4, "before_k_rope": 5, "before_attention": 6, "before_projection": 7, "before_attention_residual": 8, "before_ffn_norm": 9, "before_w1_w3": 10, "before_gate": 11, "before_w2": 12, "before_final_residual": 13, "resident_output_replaced": 14}[value]
}
