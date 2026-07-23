#include "reactor_shader_registry.h"
#include "reactor_api.h"

#include "reactor_vulkan_fp16_spirv.h"
#include "reactor_vulkan_packed4_spirv.h"
#include "reactor_vulkan_sgemm_b2x2_row_major_biased_spirv.h"
#include "reactor_vulkan_sgemm_a2x4_row_biased_accum8_spirv.h"
#include "reactor_vulkan_memory_conservative_spirv.h"
#include "reactor_vulkan_sgemm_scalar_plus_spirv.h"
#include "reactor_vulkan_sgemm_reg2x2_tile16x16_fp32_spirv.h"
#include "reactor_vulkan_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv.h"
#include "reactor_vulkan_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv.h"
#include "reactor_vulkan_sgemm_reg2x2_tile16x16_derive_fp32_spirv.h"
#include "reactor_vulkan_sgemm_tile16x16_shared_fp32_spirv.h"
#include "reactor_vulkan_sgemm_srt_2accum_k_spirv.h"
#include "reactor_vulkan_tiled_spirv.h"
#include "reactor_vulkan_inline_hlsl_bitcast_proof_spirv.h"
#include "reactor_vulkan_reduction_row_sum_spirv.h"
#include "reactor_vulkan_reduction_row_max_spirv.h"
#include "reactor_vulkan_reduction_softmax_exp_sum_spirv.h"
#include "reactor_vulkan_reduction_softmax_normalize_spirv.h"
#include "reactor_vulkan_reduction_softmax_fused_spirv.h"
#include "reactor_vulkan_reduction_row_sum_packed_short_spirv.h"
#include "reactor_vulkan_reduction_softmax_packed_short_spirv.h"
#include "reactor_vulkan_fft_bit_reverse_spirv.h"
#include "reactor_vulkan_fft_butterfly_spirv.h"
#include "reactor_vulkan_ray_query_capability_probe_spirv.h"
#include "reactor_vulkan_ray_query_raw_hit_spirv.h"
#include "reactor_vulkan_model_block_resident_identity_spirv.h"
#include "reactor_vulkan_zimage_nr0_bf16_ingress_spirv.h"
#include "reactor_vulkan_zimage_nr0_adaln_spirv.h"
#include "reactor_vulkan_zimage_nr0_attention_norm_modulate_spirv.h"
#include "reactor_vulkan_zimage_nr0_fused_qkv_spirv.h"
#include "reactor_vulkan_zimage_nr0_q_norm_rope_spirv.h"
#include "reactor_vulkan_zimage_nr0_k_norm_rope_spirv.h"
#if defined(PROMETHEUS_DVT2_MX5_VULKAN10_CONTROL)
#include "reactor_vulkan_zimage_nr0_attention_streaming_vulkan10_control_spirv.h"
#else
#include "reactor_vulkan_zimage_nr0_attention_streaming_spirv.h"
#endif
#include "reactor_vulkan_zimage_nr0_attention_projection_spirv.h"
#include "reactor_vulkan_zimage_nr0_attention_residual_spirv.h"
#include "reactor_vulkan_zimage_nr0_ffn_norm_modulate_spirv.h"
#include "reactor_vulkan_zimage_nr0_ffn_w1_w3_spirv.h"
#include "reactor_vulkan_zimage_nr0_ffn_gate_spirv.h"
#include "reactor_vulkan_zimage_nr0_ffn_w2_residual_spirv.h"
#include "reactor_vulkan_zimage_nr0_persistent_audit_summary_spirv.h"
#include "reactor_vulkan_zimage_context_refiner_qk_norm_rope_spirv.h"
#if defined(PROMETHEUS_DVT2_MX5_VULKAN10_CONTROL)
#include "reactor_vulkan_zimage_context_refiner_attention_streaming_vulkan10_control_spirv.h"
#include "reactor_vulkan_zimage_main_transformer_joint_attention_streaming_vulkan10_control_spirv.h"
#else
#include "reactor_vulkan_zimage_context_refiner_attention_streaming_spirv.h"
#include "reactor_vulkan_zimage_main_transformer_joint_attention_streaming_spirv.h"
#include "reactor_vulkan_zimage_main_transformer_joint_attention_subgroup_owned32_spirv.h"
#include "reactor_vulkan_zimage_main_transformer_subgroup_owned32_topology_probe_spirv.h"
#include "reactor_vulkan_zimage_main_transformer_joint_attention_builtin_topology_spirv.h"
#endif
#include "reactor_vulkan_zimage_main_transformer_joint_attention_subgroup_owned_spirv.h"
#include "reactor_vulkan_zimage_main_transformer_joint_attention_gemini_exact_spirv.h"
#include "reactor_vulkan_zimage_main_transformer_joint_attention_gemini_inplace_spirv.h"
#include "reactor_vulkan_zimage_main_transformer_joint_qk_norm_rope_spirv.h"
#include "reactor_vulkan_zimage_main_transformer_ffn_w1_w3_spirv.h"
#include "reactor_vulkan_zimage_main_transformer_ffn_gate_spirv.h"
#include "../shaders/sdslv/experimental/zimage/dvt2_m6a_pack_activation_f32_to_f16_spirv.h"
#include "../shaders/sdslv/experimental/zimage/dvt2_m6a_w1_w3_cooperative_f16_f32_spirv.h"

extern const uint32_t k_prom_sgemm_spirv[];
extern const size_t k_prom_sgemm_spirv_size_bytes;

#define PROM_META(tx, ty, om, on, tm, tn, tk, uk) { tx, ty, 1u, om, on, tm, tn, tk, uk }

static const prom_sgemm_kernel_dispatch_metadata k_meta_8x8 = PROM_META(8u, 8u, 1u, 1u, 8u, 8u, 8u, 1u);
static const prom_sgemm_kernel_dispatch_metadata k_meta_16x16 = PROM_META(16u, 16u, 1u, 1u, 16u, 16u, 16u, 1u);
static const prom_sgemm_kernel_dispatch_metadata k_meta_reg2x2 = PROM_META(8u, 8u, 2u, 2u, 16u, 16u, 16u, 1u);
static const prom_sgemm_kernel_dispatch_metadata k_meta_reg2x4 = PROM_META(8u, 8u, 2u, 4u, 16u, 32u, 16u, 1u);

#define PRODUCTION_ASSET_TAIL PROM_SHADER_AUTHORITY_PRODUCTION, 0u, 0u, 0u, 0u, 0u
#define ASSET(id, label, words, language, source, header, generated) \
  { id, label, PROM_SHADER_STAGE_COMPUTE, words, sizeof(words), "main", 0u, language, source, header, generated, 0u, 0u, NULL, PRODUCTION_ASSET_TAIL }

static const prom_shader_asset k_shader_assets[] = {
  { 1u, "sgemm-baseline-scalar", PROM_SHADER_STAGE_COMPUTE, k_prom_sgemm_spirv, 2668u, "main", 0u, PROM_SHADER_SOURCE_SPIRV, "reactor_vulkan_sgemm.c", "embedded", 0u, 0u, 0u, NULL, PRODUCTION_ASSET_TAIL },
  ASSET(2u, "sgemm-tiled", k_prom_sgemm_tiled_spirv, PROM_SHADER_SOURCE_SPIRV, "historical generated", "reactor_vulkan_tiled_spirv.h", 1u),
  ASSET(3u, "sgemm-memory-conservative", k_prom_sgemm_memory_conservative_spirv, PROM_SHADER_SOURCE_SPIRV, "historical generated", "reactor_vulkan_memory_conservative_spirv.h", 1u),
  ASSET(4u, "sgemm-sdsl-scalar-plus", k_prom_sgemm_scalar_plus_spirv, PROM_SHADER_SOURCE_SDSLV, "internal/prometheus/shaders/sdslv/production/sgemm/sgemm_scalar_baseline_plus.sdslv", "reactor_vulkan_sgemm_scalar_plus_spirv.h", 1u),
  ASSET(5u, "sgemm-sdsl-tile16", k_prom_sgemm_tile16x16_shared_fp32_spirv, PROM_SHADER_SOURCE_SDSLV, "internal/prometheus/shaders/sdslv/production/sgemm/sgemm_tile16x16_shared_fp32.sdslv", "reactor_vulkan_sgemm_tile16x16_shared_fp32_spirv.h", 1u),
  ASSET(6u, "sgemm-sdsl-reg2x2", k_prom_sgemm_reg2x2_tile16x16_fp32_spirv, PROM_SHADER_SOURCE_SDSLV, "internal/prometheus/shaders/sdslv/production/sgemm/sgemm_reg2x2_tile16x16_fp32.sdslv", "reactor_vulkan_sgemm_reg2x2_tile16x16_fp32_spirv.h", 1u),
  ASSET(7u, "sgemm-sdsl-exacttail", k_prom_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv, PROM_SHADER_SOURCE_SDSLV, "internal/prometheus/shaders/sdslv/production/sgemm/sgemm_reg2x2_tile16x16_exacttail_fp32.sdslv", "reactor_vulkan_sgemm_reg2x2_tile16x16_exacttail_fp32_spirv.h", 1u),
  ASSET(8u, "sgemm-sdsl-flowboard", k_prom_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv, PROM_SHADER_SOURCE_SDSLV, "internal/prometheus/shaders/sdslv/production/sgemm/sgemm_reg2x2_tile16x16_flowboard_fp32.sdslv", "reactor_vulkan_sgemm_reg2x2_tile16x16_flowboard_fp32_spirv.h", 1u),
  ASSET(9u, "sgemm-sdsl-derive", k_prom_sgemm_reg2x2_tile16x16_derive_fp32_spirv, PROM_SHADER_SOURCE_SDSLV, "internal/prometheus/shaders/sdslv/production/sgemm/sgemm_reg2x2_tile16x16_derive_fp32.sdslv", "reactor_vulkan_sgemm_reg2x2_tile16x16_derive_fp32_spirv.h", 1u),
  { 10u, "sgemm-srt-2accum", PROM_SHADER_STAGE_COMPUTE, k_prom_sgemm_srt_2accum_k_spirv, sizeof(k_prom_sgemm_srt_2accum_k_spirv), "SgemmSrt2AccumK_CS", 0u, PROM_SHADER_SOURCE_SDSLV, "internal/prometheus/shaders/sdslv/production/sgemm/sgemm_srt_2accum_k.sdslv", "reactor_vulkan_sgemm_srt_2accum_k_spirv.h", 1u, 0u, 0u, NULL, PRODUCTION_ASSET_TAIL },
  { 11u, "sgemm-b2x2", PROM_SHADER_STAGE_COMPUTE, k_prom_sgemm_b2x2_row_major_biased_spirv, sizeof(k_prom_sgemm_b2x2_row_major_biased_spirv), "SgemmB2x2_CS", 0u, PROM_SHADER_SOURCE_SDSLV, "internal/prometheus/shaders/sdslv/production/sgemm/sgemm_b2x2_row_major_biased.sdslv", "reactor_vulkan_sgemm_b2x2_row_major_biased_spirv.h", 1u, 0u, 0u, NULL, PRODUCTION_ASSET_TAIL },
  { 12u, "sgemm-a2x4", PROM_SHADER_STAGE_COMPUTE, k_prom_sgemm_a2x4_row_biased_accum8_spirv, sizeof(k_prom_sgemm_a2x4_row_biased_accum8_spirv), "SgemmA2x4_CS", 0u, PROM_SHADER_SOURCE_SDSLV, "internal/prometheus/shaders/sdslv/production/sgemm/sgemm_a2x4_row_biased_accum8.sdslv", "reactor_vulkan_sgemm_a2x4_row_biased_accum8_spirv.h", 1u, 0u, 0u, NULL, PRODUCTION_ASSET_TAIL },
  { 13u, "sgemm-packed4", PROM_SHADER_STAGE_COMPUTE, k_prom_sgemm_packed4_spirv, sizeof(k_prom_sgemm_packed4_spirv), "SgemmPacked4_CS", 0u, PROM_SHADER_SOURCE_SDSLV, "internal/prometheus/shaders/sdslv/production/sgemm/sgemm_packed4_fp32.sdslv", "reactor_vulkan_packed4_spirv.h", 1u, 0u, 0u, NULL, PRODUCTION_ASSET_TAIL },
  { 14u, "sgemm-fp16-storage-fp32-accum", PROM_SHADER_STAGE_COMPUTE, k_prom_sgemm_fp16_storage_fp32accum_spirv, sizeof(k_prom_sgemm_fp16_storage_fp32accum_spirv), "SgemmFp16StorageFp32Accum_CS", 0u, PROM_SHADER_SOURCE_SDSLV, "internal/prometheus/shaders/sdslv/production/sgemm/sgemm_fp16_storage_fp32_accum.sdslv", "reactor_vulkan_fp16_spirv.h", 1u, 0u, 0u, NULL, PRODUCTION_ASSET_TAIL },
  { 15u, "sdslv-inline-hlsl-bitcast-proof", PROM_SHADER_STAGE_COMPUTE, k_prom_inline_hlsl_bitcast_proof_spirv, sizeof(k_prom_inline_hlsl_bitcast_proof_spirv), "InlineHlslBitCastProof_CS", 0u, PROM_SHADER_SOURCE_SDSLV, "internal/prometheus/shaders/sdslv/production/sgemm/inline_hlsl_bitcast_proof.sdslv", "reactor_vulkan_inline_hlsl_bitcast_proof_spirv.h", 1u, 1u, 2u, "HLSL", PRODUCTION_ASSET_TAIL },
  { 52u, "fft-radix2-bit-reverse", PROM_SHADER_STAGE_COMPUTE, k_prom_fft_bit_reverse_spirv, sizeof(k_prom_fft_bit_reverse_spirv), "FftRadix2BitReverse_CS", 0u, PROM_SHADER_SOURCE_SDSLV, "internal/prometheus/shaders/sdslv/production/fft/radix2_bit_reverse.sdslv", "reactor_vulkan_fft_bit_reverse_spirv.h", 1u, 0u, 0u, NULL, PROM_SHADER_AUTHORITY_PRODUCTION, 2u, 16u, 0u, 1u, 1048576u },
  { 53u, "fft-radix2-butterfly", PROM_SHADER_STAGE_COMPUTE, k_prom_fft_butterfly_spirv, sizeof(k_prom_fft_butterfly_spirv), "FftRadix2Butterfly_CS", 0u, PROM_SHADER_SOURCE_SDSLV, "internal/prometheus/shaders/sdslv/production/fft/radix2_butterfly.sdslv", "reactor_vulkan_fft_butterfly_spirv.h", 1u, 1u, 3u, "HLSL", PROM_SHADER_AUTHORITY_PRODUCTION, 2u, 32u, 0u, 1u, 1048576u },
  { 54u, "ray-query-capability-probe", PROM_SHADER_STAGE_COMPUTE, k_prom_ray_query_capability_probe_spirv, sizeof(k_prom_ray_query_capability_probe_spirv), "RayQueryCapabilityProbe_CS", 0u, PROM_SHADER_SOURCE_SDSLV, "internal/prometheus/shaders/sdslv/production/rayquery/ray_query_capability_probe.sdslv", "reactor_vulkan_ray_query_capability_probe_spirv.h", 1u, 0u, 0u, NULL, PROM_SHADER_AUTHORITY_PRODUCTION, 2u, 0u, 0u, 1u, 1u },
  { 55u, "ray-query-raw-hit", PROM_SHADER_STAGE_COMPUTE, k_prom_ray_query_raw_hit_spirv, sizeof(k_prom_ray_query_raw_hit_spirv), "RayQueryRawHit_CS", 0u, PROM_SHADER_SOURCE_SDSLV, "internal/prometheus/shaders/sdslv/production/rayquery/ray_query_raw_hit.sdslv", "reactor_vulkan_ray_query_raw_hit_spirv.h", 1u, 0u, 0u, NULL, PROM_SHADER_AUTHORITY_PRODUCTION, 5u, 0u, 0u, 1u, 1u },
  { 23u, "model-block-resident-identity", PROM_SHADER_STAGE_COMPUTE,
    k_prom_model_block_resident_identity_spirv, sizeof(k_prom_model_block_resident_identity_spirv),
    "ResidentModelBlockIdentity_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/model_block/resident_identity.sdslv",
    "reactor_vulkan_model_block_resident_identity_spirv.h", 1u, 0u, 0u, NULL,
    PROM_SHADER_AUTHORITY_PRODUCTION, 3u, 8u, 0u, 1u, 4194304u },
  { 24u, "zimage-nr0-adaln", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_nr0_adaln_spirv, sizeof(k_prom_zimage_nr0_adaln_spirv),
    "Nr0AdaLN_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/nr0_adaln.sdslv",
    "reactor_vulkan_zimage_nr0_adaln_spirv.h", 1u, 1u, 1u, "HLSL",
    PROM_SHADER_AUTHORITY_PRODUCTION, 8u, 8u, 0u, 1u, 15360u },
  { 25u, "zimage-nr0-attention-norm-modulate", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_nr0_attention_norm_modulate_spirv, sizeof(k_prom_zimage_nr0_attention_norm_modulate_spirv),
    "Nr0AttentionNormModulate_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/nr0_attention_norm_modulate.sdslv",
    "reactor_vulkan_zimage_nr0_attention_norm_modulate_spirv.h", 1u, 1u, 1u, "HLSL",
    PROM_SHADER_AUTHORITY_PRODUCTION, 5u, 16u, 0u, 1u, 3840u },
  { 26u, "zimage-nr0-fused-qkv", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_nr0_fused_qkv_spirv, sizeof(k_prom_zimage_nr0_fused_qkv_spirv),
    "Nr0FusedQkv_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/nr0_fused_qkv.sdslv",
    "reactor_vulkan_zimage_nr0_fused_qkv_spirv.h", 1u, 0u, 0u, NULL,
    PROM_SHADER_AUTHORITY_PRODUCTION, 3u, 16u, 0u, 1u, 11520u },
  { 27u, "zimage-nr0-q-norm-rope", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_nr0_q_norm_rope_spirv, sizeof(k_prom_zimage_nr0_q_norm_rope_spirv),
    "Nr0QueryNormRope_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/nr0_q_norm_rope.sdslv",
    "reactor_vulkan_zimage_nr0_q_norm_rope_spirv.h", 1u, 1u, 4u, "HLSL",
    PROM_SHADER_AUTHORITY_PRODUCTION, 3u, 16u, 0u, 1u, 128u },
  { 28u, "zimage-nr0-k-norm-rope", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_nr0_k_norm_rope_spirv, sizeof(k_prom_zimage_nr0_k_norm_rope_spirv),
    "Nr0KeyNormRope_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/nr0_k_norm_rope.sdslv",
    "reactor_vulkan_zimage_nr0_k_norm_rope_spirv.h", 1u, 1u, 4u, "HLSL",
    PROM_SHADER_AUTHORITY_PRODUCTION, 3u, 16u, 0u, 1u, 128u },
  { 29u, "zimage-nr0-bf16-ingress", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_nr0_bf16_ingress_spirv, sizeof(k_prom_zimage_nr0_bf16_ingress_spirv),
    "Nr0Bf16Ingress_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/nr0_bf16_ingress.sdslv",
    "reactor_vulkan_zimage_nr0_bf16_ingress_spirv.h", 1u, 1u, 1u, "HLSL",
    PROM_SHADER_AUTHORITY_PRODUCTION, 4u, 8u, 0u, 1u, 3932160u },
  { 30u, "zimage-nr0-attention-streaming", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_nr0_attention_streaming_spirv, sizeof(k_prom_zimage_nr0_attention_streaming_spirv),
    "Nr0AttentionStreaming_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/nr0_attention_streaming.sdslv",
    "reactor_vulkan_zimage_nr0_attention_streaming_spirv.h", 1u, 1u, 1u, "HLSL",
    PROM_SHADER_AUTHORITY_PRODUCTION, 3u, 16u, 0u, 1024u, 1024u },
  { 31u, "zimage-nr0-attention-projection", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_nr0_attention_projection_spirv, sizeof(k_prom_zimage_nr0_attention_projection_spirv),
    "Nr0AttentionProjection_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/nr0_attention_projection.sdslv",
    "reactor_vulkan_zimage_nr0_attention_projection_spirv.h", 1u, 0u, 0u, NULL,
    PROM_SHADER_AUTHORITY_PRODUCTION, 3u, 16u, 0u, 1u, 3840u },
  { 32u, "zimage-nr0-attention-residual", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_nr0_attention_residual_spirv, sizeof(k_prom_zimage_nr0_attention_residual_spirv),
    "Nr0AttentionResidual_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/nr0_attention_residual.sdslv",
    "reactor_vulkan_zimage_nr0_attention_residual_spirv.h", 1u, 1u, 1u, "HLSL",
    PROM_SHADER_AUTHORITY_PRODUCTION, 5u, 16u, 0u, 1u, 3840u },
  { 33u, "zimage-nr0-ffn-norm-modulate", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_nr0_ffn_norm_modulate_spirv, sizeof(k_prom_zimage_nr0_ffn_norm_modulate_spirv),
    "Nr0FfnNormModulate_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/nr0_ffn_norm_modulate.sdslv",
    "reactor_vulkan_zimage_nr0_ffn_norm_modulate_spirv.h", 1u, 1u, 1u, "HLSL",
    PROM_SHADER_AUTHORITY_PRODUCTION, 5u, 16u, 0u, 1u, 3840u },
  { 34u, "zimage-nr0-ffn-w1-w3", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_nr0_ffn_w1_w3_spirv, sizeof(k_prom_zimage_nr0_ffn_w1_w3_spirv),
    "Nr0FfnW1W3_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/nr0_ffn_w1_w3.sdslv",
    "reactor_vulkan_zimage_nr0_ffn_w1_w3_spirv.h", 1u, 0u, 0u, NULL,
    PROM_SHADER_AUTHORITY_PRODUCTION, 7u, 16u, 0u, 1u, 10240u },
  { 35u, "zimage-nr0-ffn-gate", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_nr0_ffn_gate_spirv, sizeof(k_prom_zimage_nr0_ffn_gate_spirv),
    "Nr0FfnGate_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/nr0_ffn_gate.sdslv",
    "reactor_vulkan_zimage_nr0_ffn_gate_spirv.h", 1u, 1u, 1u, "HLSL",
    PROM_SHADER_AUTHORITY_PRODUCTION, 4u, 16u, 0u, 1u, 10485760u },
  { 36u, "zimage-nr0-ffn-w2-residual", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_nr0_ffn_w2_residual_spirv, sizeof(k_prom_zimage_nr0_ffn_w2_residual_spirv),
    "Nr0FfnW2Residual_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/nr0_ffn_w2_residual.sdslv",
    "reactor_vulkan_zimage_nr0_ffn_w2_residual_spirv.h", 1u, 1u, 1u, "HLSL",
    PROM_SHADER_AUTHORITY_PRODUCTION, 7u, 16u, 0u, 1u, 10240u },
  /* Audit-only: this asset is deliberately absent from the model's 13-pipeline
     execution portfolio.  It can be instantiated only by the static audit
     batch owner, so no-audit model creation and production execution identity
     remain unchanged. */
  { 37u, "zimage-nr0-persistent-audit-summary", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_nr0_persistent_audit_summary_spirv,
    sizeof(k_prom_zimage_nr0_persistent_audit_summary_spirv),
    "Nr0PersistentAuditSummary_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/nr0_persistent_audit_summary.sdslv",
    "reactor_vulkan_zimage_nr0_persistent_audit_summary_spirv.h", 1u, 1u, 1u, "HLSL",
    PROM_SHADER_AUTHORITY_PRODUCTION, 4u, 96u, 0u, 1u, 10485760u },
  { 38u, "zimage-context-refiner-qk-norm-rope", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_context_refiner_qk_norm_rope_spirv,
    sizeof(k_prom_zimage_context_refiner_qk_norm_rope_spirv),
    "ContextQkNormRope_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/context_refiner_qk_norm_rope.sdslv",
    "reactor_vulkan_zimage_context_refiner_qk_norm_rope_spirv.h", 1u, 1u, 5u, "HLSL",
    PROM_SHADER_AUTHORITY_PRODUCTION, 3u, 20u, 0u, 1u, 128u },
  { 39u, "zimage-context-refiner-attention-streaming", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_context_refiner_attention_streaming_spirv,
    sizeof(k_prom_zimage_context_refiner_attention_streaming_spirv),
    "ContextAttentionStreaming_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/context_refiner_attention_streaming.sdslv",
    "reactor_vulkan_zimage_context_refiner_attention_streaming_spirv.h", 1u, 1u, 2u, "HLSL",
    PROM_SHADER_AUTHORITY_PRODUCTION, 3u, 16u, 0u, 1u, 32u },
  { 40u, "zimage-main-transformer-joint-qk-norm-rope", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_main_transformer_joint_qk_norm_rope_spirv,
    sizeof(k_prom_zimage_main_transformer_joint_qk_norm_rope_spirv),
    "MainTransformerJointQkNormRope_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/main_transformer_joint_qk_norm_rope.sdslv",
    "reactor_vulkan_zimage_main_transformer_joint_qk_norm_rope_spirv.h", 1u, 1u, 5u, "HLSL",
    PROM_SHADER_AUTHORITY_PRODUCTION, 3u, 24u, 0u, 1u, 128u },
  { 41u, "zimage-main-transformer-joint-attention-streaming", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_main_transformer_joint_attention_streaming_spirv,
    sizeof(k_prom_zimage_main_transformer_joint_attention_streaming_spirv),
    "MainTransformerJointAttentionStreaming_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/main_transformer_joint_attention_streaming.sdslv",
    "reactor_vulkan_zimage_main_transformer_joint_attention_streaming_spirv.h", 1u, 1u, 1u, "HLSL",
    PROM_SHADER_AUTHORITY_PRODUCTION, 3u, 16u, 0u, 1056u, 1056u },
  { 47u, "zimage-main-transformer-joint-attention-subgroup-owned32", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_main_transformer_joint_attention_subgroup_owned32_spirv,
    sizeof(k_prom_zimage_main_transformer_joint_attention_subgroup_owned32_spirv),
    "MainTransformerJointAttentionSubgroupOwned_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/main_transformer_joint_attention_subgroup_owned32.sdslv",
    "reactor_vulkan_zimage_main_transformer_joint_attention_subgroup_owned32_spirv.h", 1u, 1u, 1u, "HLSL",
    PROM_SHADER_AUTHORITY_PRODUCTION, 3u, 16u, 0u, 1056u, 1056u },
  { 48u, "zimage-main-transformer-subgroup-owned32-topology-probe", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_main_transformer_subgroup_owned32_topology_probe_spirv,
    sizeof(k_prom_zimage_main_transformer_subgroup_owned32_topology_probe_spirv),
    "MainTransformerSubgroupOwned32TopologyProbe_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/main_transformer_subgroup_owned32_topology_probe.sdslv",
    "reactor_vulkan_zimage_main_transformer_subgroup_owned32_topology_probe_spirv.h", 1u, 1u, 1u, "HLSL",
    PROM_SHADER_AUTHORITY_PRODUCTION, 1u, 0u, 0u, 256u, 256u },
  { 49u, "zimage-main-transformer-joint-attention-builtin-topology", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_main_transformer_joint_attention_builtin_topology_spirv,
    sizeof(k_prom_zimage_main_transformer_joint_attention_builtin_topology_spirv),
    "MainTransformerJointAttentionBuiltinTopology_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/main_transformer_joint_attention_builtin_topology.sdslv",
    "reactor_vulkan_zimage_main_transformer_joint_attention_builtin_topology_spirv.h", 1u, 1u, 1u, "HLSL",
    PROM_SHADER_AUTHORITY_PRODUCTION, 3u, 16u, 0u, 1056u, 1056u },
  { 44u, "zimage-main-transformer-joint-attention-subgroup-owned", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_main_transformer_joint_attention_subgroup_owned_spirv,
    sizeof(k_prom_zimage_main_transformer_joint_attention_subgroup_owned_spirv),
    "MainTransformerJointAttentionSubgroupOwned_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/experimental/attention/main_transformer_joint_attention_subgroup_owned.sdslv",
    "reactor_vulkan_zimage_main_transformer_joint_attention_subgroup_owned_spirv.h", 1u, 1u, 1u, "HLSL",
    PROM_SHADER_AUTHORITY_EXPERIMENTAL, 3u, 16u, 0u, 1056u, 1056u },
  { 45u, "zimage-main-transformer-joint-attention-gemini-exact", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_main_transformer_joint_attention_gemini_exact_spirv,
    sizeof(k_prom_zimage_main_transformer_joint_attention_gemini_exact_spirv),
    "MainTransformerJointAttentionSubgroupOwned_CS", 0u, PROM_SHADER_SOURCE_HLSL,
    "internal/prometheus/shaders/hlsl/external/m5b-gemini-exact.hlsl",
    "reactor_vulkan_zimage_main_transformer_joint_attention_gemini_exact_spirv.h", 1u, 0u, 0u, NULL,
    PROM_SHADER_AUTHORITY_EXPERIMENTAL, 3u, 16u, 0u, 1056u, 1056u },
  { 46u, "zimage-main-transformer-joint-attention-gemini-inplace", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_main_transformer_joint_attention_gemini_inplace_spirv,
    sizeof(k_prom_zimage_main_transformer_joint_attention_gemini_inplace_spirv),
    "MainTransformerJointAttentionSubgroupOwned_CS", 0u, PROM_SHADER_SOURCE_HLSL,
    "internal/prometheus/shaders/hlsl/experimental/m5b-gemini-inplace.hlsl",
    "reactor_vulkan_zimage_main_transformer_joint_attention_gemini_inplace_spirv.h", 1u, 0u, 0u, NULL,
    PROM_SHADER_AUTHORITY_EXPERIMENTAL, 3u, 16u, 0u, 1056u, 1056u },
  { 42u, "zimage-main-transformer-ffn-w1-w3", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_main_transformer_ffn_w1_w3_spirv,
    sizeof(k_prom_zimage_main_transformer_ffn_w1_w3_spirv),
    "MainTransformerFfnW1W3_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/main_transformer_ffn_w1_w3.sdslv",
    "reactor_vulkan_zimage_main_transformer_ffn_w1_w3_spirv.h", 1u, 0u, 0u, NULL,
    PROM_SHADER_AUTHORITY_PRODUCTION, 7u, 16u, 0u, 1u, 10240u },
  { 43u, "zimage-main-transformer-ffn-gate", PROM_SHADER_STAGE_COMPUTE,
    k_prom_zimage_main_transformer_ffn_gate_spirv,
    sizeof(k_prom_zimage_main_transformer_ffn_gate_spirv),
    "MainTransformerFfnGate_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/production/zimage/main_transformer_ffn_gate.sdslv",
    "reactor_vulkan_zimage_main_transformer_ffn_gate_spirv.h", 1u, 1u, 1u, "HLSL",
    PROM_SHADER_AUTHORITY_PRODUCTION, 4u, 16u, 0u, 1u, 10813440u },
  { 50u, "dvt2-m6a-pack-activation-f32-to-f16", PROM_SHADER_STAGE_COMPUTE,
    k_prom_dvt2_m6a_pack_activation_spirv, sizeof(k_prom_dvt2_m6a_pack_activation_spirv),
    "Dvt2M6aPackActivationF32ToF16_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/experimental/zimage/dvt2_m6a_pack_activation_f32_to_f16.sdslv",
    "../shaders/sdslv/experimental/zimage/dvt2_m6a_pack_activation_f32_to_f16_spirv.h", 1u, 0u, 0u, NULL,
    PROM_SHADER_AUTHORITY_EXPERIMENTAL, 2u, 8u, 0u, 1u, 4055040u },
  { 51u, "dvt2-m6a-w1-w3-cooperative-f16-f32", PROM_SHADER_STAGE_COMPUTE,
    k_prom_dvt2_m6a_w1_w3_cooperative_spirv, sizeof(k_prom_dvt2_m6a_w1_w3_cooperative_spirv),
    "Dvt2M6aW1W3CooperativeF16F32_CS", 0u, PROM_SHADER_SOURCE_SDSLV,
    "internal/prometheus/shaders/sdslv/experimental/zimage/dvt2_m6a_w1_w3_cooperative_f16_f32.sdslv",
    "../shaders/sdslv/experimental/zimage/dvt2_m6a_w1_w3_cooperative_f16_f32_spirv.h", 1u, 1u, 1u, "HLSL",
    PROM_SHADER_AUTHORITY_EXPERIMENTAL, 3u, 12u, 0u, 1056u, 10240u },
};

#define REDUCTION_ASSET(id, label, words, entry, source, header, inline_count, role, max_width) \
  { id, label, PROM_SHADER_STAGE_COMPUTE, words, sizeof(words), entry, 0u, PROM_SHADER_SOURCE_SDSLV, source, header, 1u, \
    (inline_count) != 0u, inline_count, (inline_count) != 0u ? "HLSL" : NULL, PROM_SHADER_AUTHORITY_PRODUCTION, \
    4u, 32u, role, 1u, max_width }

static const prom_shader_asset k_reduction_shader_assets[] = {
  REDUCTION_ASSET(PROM_REDUCTION_SHADER_ROW_SUM, "reduction-row-sum-stage", k_prom_reduction_row_sum_spirv,
                  "RowSumStage_CS", "internal/prometheus/shaders/sdslv/production/reduction/row_sum_stage.sdslv",
                  "reactor_vulkan_reduction_row_sum_spirv.h", 0u, PROM_REDUCTION_STAGE_ROW_SUM,
                  PROM_REDUCTION_MAX_ELEMENTS_PER_ROW),
  REDUCTION_ASSET(PROM_REDUCTION_SHADER_ROW_MAX, "reduction-row-max-stage", k_prom_reduction_row_max_spirv,
                  "RowMaxStage_CS", "internal/prometheus/shaders/sdslv/production/reduction/row_max_stage.sdslv",
                  "reactor_vulkan_reduction_row_max_spirv.h", 0u, PROM_REDUCTION_STAGE_ROW_MAX,
                  PROM_REDUCTION_MAX_ELEMENTS_PER_ROW),
  REDUCTION_ASSET(PROM_REDUCTION_SHADER_SOFTMAX_EXP_SUM, "reduction-softmax-exp-sum-stage",
                  k_prom_reduction_softmax_exp_sum_spirv, "SoftmaxExpSumStage_CS",
                  "internal/prometheus/shaders/sdslv/production/reduction/softmax_exp_sum_stage.sdslv",
                  "reactor_vulkan_reduction_softmax_exp_sum_spirv.h", 1u, PROM_REDUCTION_STAGE_SOFTMAX_EXP_SUM,
                  PROM_REDUCTION_MAX_ELEMENTS_PER_ROW),
  REDUCTION_ASSET(PROM_REDUCTION_SHADER_SOFTMAX_NORMALIZE, "reduction-softmax-normalize",
                  k_prom_reduction_softmax_normalize_spirv, "SoftmaxNormalize_CS",
                  "internal/prometheus/shaders/sdslv/production/reduction/softmax_normalize.sdslv",
                  "reactor_vulkan_reduction_softmax_normalize_spirv.h", 1u, PROM_REDUCTION_STAGE_SOFTMAX_NORMALIZE,
                  PROM_REDUCTION_MAX_ELEMENTS_PER_ROW),
  REDUCTION_ASSET(PROM_REDUCTION_SHADER_SOFTMAX_FUSED, "reduction-softmax-fused",
                  k_prom_reduction_softmax_fused_spirv, "SoftmaxFused_CS",
                  "internal/prometheus/shaders/sdslv/production/reduction/softmax_fused.sdslv",
                  "reactor_vulkan_reduction_softmax_fused_spirv.h", 1u, PROM_REDUCTION_STAGE_SOFTMAX_FUSED,
                  PROM_REDUCTION_SINGLE_STAGE_THRESHOLD),
  REDUCTION_ASSET(PROM_REDUCTION_SHADER_ROW_SUM_PACKED_SHORT, "reduction-row-sum-packed-short",
                  k_prom_reduction_row_sum_packed_short_spirv, "RowSumPackedShort_CS",
                  "internal/prometheus/shaders/sdslv/production/reduction/row_sum_packed_short.sdslv",
                  "reactor_vulkan_reduction_row_sum_packed_short_spirv.h", 0u,
                  PROM_REDUCTION_STAGE_ROW_SUM_PACKED_SHORT, PROM_REDUCTION_PACKED_SHORT_WIDTH_MAX),
  REDUCTION_ASSET(PROM_REDUCTION_SHADER_SOFTMAX_PACKED_SHORT, "reduction-softmax-packed-short",
                  k_prom_reduction_softmax_packed_short_spirv, "SoftmaxPackedShort_CS",
                  "internal/prometheus/shaders/sdslv/production/reduction/softmax_packed_short.sdslv",
                  "reactor_vulkan_reduction_softmax_packed_short_spirv.h", 1u,
                  PROM_REDUCTION_STAGE_SOFTMAX_PACKED_SHORT, PROM_REDUCTION_PACKED_SHORT_WIDTH_MAX),
};

#define IMPL(id, label, shader, meta, slot) \
  { id, PROM_SHADER_OPERATION_SGEMM, label, shader, meta, 0u, 1u, 1u, 1u, slot, NULL, \
    PROM_SHADER_AUTHORITY_PRODUCTION, PROM_COMPUTE_PIPELINE_FAMILY_SGEMM, slot }
static const prom_compute_implementation k_compute_implementations[] = {
  IMPL(1u, "baseline-scalar", 1u, &k_meta_8x8, PROM_COMPUTE_PIPELINE_BASELINE),
  IMPL(2u, "memory-conservative", 3u, &k_meta_8x8, PROM_COMPUTE_PIPELINE_MEMORY_CONSERVATIVE),
  IMPL(3u, "small-register-tile", 10u, &k_meta_8x8, PROM_COMPUTE_PIPELINE_SRT),
  IMPL(4u, "balanced-2x2-accum4", 11u, &k_meta_reg2x2, PROM_COMPUTE_PIPELINE_B2X2),
  IMPL(5u, "aggressive-4x4-accum8", 12u, &k_meta_reg2x4, PROM_COMPUTE_PIPELINE_A2X4),
  IMPL(6u, "sdsl-scalar-plus", 4u, &k_meta_8x8, PROM_COMPUTE_PIPELINE_SDSL_SCALAR_PLUS),
  IMPL(7u, "sdsl-tile16x16-shared-fp32", 5u, &k_meta_16x16, PROM_COMPUTE_PIPELINE_SDSL_TILE16),
  IMPL(8u, "sdsl-reg2x2-tile16x16-fp32", 6u, &k_meta_reg2x2, PROM_COMPUTE_PIPELINE_SDSL_REG2X2),
  IMPL(9u, "sdsl-reg2x2-tile16x16-exacttail-fp32", 7u, &k_meta_reg2x2, PROM_COMPUTE_PIPELINE_SDSL_EXACTTAIL),
  IMPL(10u, "sdsl-reg2x2-tile16x16-flowboard-fp32", 8u, &k_meta_reg2x2, PROM_COMPUTE_PIPELINE_SDSL_FLOWBOARD),
  IMPL(11u, "sdsl-reg2x2-tile16x16-derive-fp32", 9u, &k_meta_reg2x2, PROM_COMPUTE_PIPELINE_SDSL_DERIVE),
};

#define REDUCTION_META(role, max_width) \
  { PROM_REDUCTION_LOCAL_SIZE, 1u, 1u, 4u, PROM_REDUCTION_ELEMENTS_PER_PARTIAL, 4u, 32u, role, 1u, max_width }
static const prom_reduction_kernel_dispatch_metadata k_reduction_sum_meta =
    REDUCTION_META(PROM_REDUCTION_STAGE_ROW_SUM, PROM_REDUCTION_MAX_ELEMENTS_PER_ROW);
static const prom_reduction_kernel_dispatch_metadata k_reduction_max_meta =
    REDUCTION_META(PROM_REDUCTION_STAGE_ROW_MAX, PROM_REDUCTION_MAX_ELEMENTS_PER_ROW);
static const prom_reduction_kernel_dispatch_metadata k_softmax_exp_sum_meta =
    REDUCTION_META(PROM_REDUCTION_STAGE_SOFTMAX_EXP_SUM, PROM_REDUCTION_MAX_ELEMENTS_PER_ROW);
static const prom_reduction_kernel_dispatch_metadata k_softmax_normalize_meta =
    REDUCTION_META(PROM_REDUCTION_STAGE_SOFTMAX_NORMALIZE, PROM_REDUCTION_MAX_ELEMENTS_PER_ROW);
static const prom_reduction_kernel_dispatch_metadata k_softmax_fused_meta =
    REDUCTION_META(PROM_REDUCTION_STAGE_SOFTMAX_FUSED, PROM_REDUCTION_SINGLE_STAGE_THRESHOLD);
static const prom_reduction_kernel_dispatch_metadata k_reduction_sum_packed_short_meta =
    REDUCTION_META(PROM_REDUCTION_STAGE_ROW_SUM_PACKED_SHORT, PROM_REDUCTION_PACKED_SHORT_WIDTH_MAX);
static const prom_reduction_kernel_dispatch_metadata k_softmax_packed_short_meta =
    REDUCTION_META(PROM_REDUCTION_STAGE_SOFTMAX_PACKED_SHORT, PROM_REDUCTION_PACKED_SHORT_WIDTH_MAX);

#define REDUCTION_IMPL(id, operation, label, shader, metadata, index) \
  { id, operation, label, shader, NULL, 0u, 1u, 0u, 1u, PROM_COMPUTE_PIPELINE_COUNT, metadata, \
    PROM_SHADER_AUTHORITY_PRODUCTION, PROM_COMPUTE_PIPELINE_FAMILY_REDUCTION, index }
static const prom_compute_implementation k_reduction_compute_implementations[] = {
  REDUCTION_IMPL(PROM_REDUCTION_IMPLEMENTATION_ROW_SUM, PROM_SHADER_OPERATION_REDUCTION_SUM,
                 "reduction-row-sum-stage", PROM_REDUCTION_SHADER_ROW_SUM, &k_reduction_sum_meta, 0u),
  REDUCTION_IMPL(PROM_REDUCTION_IMPLEMENTATION_ROW_MAX, PROM_SHADER_OPERATION_REDUCTION_MAX,
                 "reduction-row-max-stage", PROM_REDUCTION_SHADER_ROW_MAX, &k_reduction_max_meta, 1u),
  REDUCTION_IMPL(PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_EXP_SUM, PROM_SHADER_OPERATION_SOFTMAX,
                 "reduction-softmax-exp-sum-stage", PROM_REDUCTION_SHADER_SOFTMAX_EXP_SUM,
                 &k_softmax_exp_sum_meta, 2u),
  REDUCTION_IMPL(PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_NORMALIZE, PROM_SHADER_OPERATION_SOFTMAX,
                 "reduction-softmax-normalize", PROM_REDUCTION_SHADER_SOFTMAX_NORMALIZE,
                 &k_softmax_normalize_meta, 3u),
  REDUCTION_IMPL(PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_FUSED, PROM_SHADER_OPERATION_SOFTMAX,
                 "reduction-softmax-fused", PROM_REDUCTION_SHADER_SOFTMAX_FUSED, &k_softmax_fused_meta, 4u),
  REDUCTION_IMPL(PROM_REDUCTION_IMPLEMENTATION_ROW_SUM_PACKED_SHORT, PROM_SHADER_OPERATION_REDUCTION_SUM,
                 "reduction-row-sum-packed-short", PROM_REDUCTION_SHADER_ROW_SUM_PACKED_SHORT,
                 &k_reduction_sum_packed_short_meta, 5u),
  REDUCTION_IMPL(PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_PACKED_SHORT, PROM_SHADER_OPERATION_SOFTMAX,
                 "reduction-softmax-packed-short", PROM_REDUCTION_SHADER_SOFTMAX_PACKED_SHORT,
                 &k_softmax_packed_short_meta, 6u),
};

const prom_shader_asset* prom_shader_registry_find_shader(uint32_t id) { for (size_t i=0u;i<sizeof(k_shader_assets)/sizeof(k_shader_assets[0]);++i) if(k_shader_assets[i].shader_id==id) return &k_shader_assets[i]; for (size_t i=0u;i<sizeof(k_reduction_shader_assets)/sizeof(k_reduction_shader_assets[0]);++i) if(k_reduction_shader_assets[i].shader_id==id) return &k_reduction_shader_assets[i]; return NULL; }
size_t prom_shader_registry_shader_asset_count(void) { return sizeof(k_shader_assets)/sizeof(k_shader_assets[0]); }
const prom_shader_asset* prom_shader_registry_shader_asset_at(size_t index) { return index < prom_shader_registry_shader_asset_count() ? &k_shader_assets[index] : NULL; }
size_t prom_shader_registry_reduction_shader_asset_count(void) { return sizeof(k_reduction_shader_assets)/sizeof(k_reduction_shader_assets[0]); }
const prom_shader_asset* prom_shader_registry_reduction_shader_asset_at(size_t index) { return index < prom_shader_registry_reduction_shader_asset_count() ? &k_reduction_shader_assets[index] : NULL; }
size_t prom_shader_registry_experimental_shader_asset_count(void) { return 0u; }
const prom_shader_asset* prom_shader_registry_experimental_shader_asset_at(size_t index) { (void)index; return NULL; }
const prom_compute_implementation* prom_shader_registry_find_compute_implementation(uint32_t id) { for (size_t i=0u;i<sizeof(k_compute_implementations)/sizeof(k_compute_implementations[0]);++i) if(k_compute_implementations[i].implementation_id==id) return &k_compute_implementations[i]; for (size_t i=0u;i<sizeof(k_reduction_compute_implementations)/sizeof(k_reduction_compute_implementations[0]);++i) if(k_reduction_compute_implementations[i].implementation_id==id) return &k_reduction_compute_implementations[i]; return NULL; }
size_t prom_shader_registry_compute_implementation_count(void) { return sizeof(k_compute_implementations)/sizeof(k_compute_implementations[0]); }
const prom_compute_implementation* prom_shader_registry_compute_implementation_at(size_t index) { return index < prom_shader_registry_compute_implementation_count() ? &k_compute_implementations[index] : NULL; }
size_t prom_shader_registry_reduction_compute_implementation_count(void) { return sizeof(k_reduction_compute_implementations)/sizeof(k_reduction_compute_implementations[0]); }
const prom_compute_implementation* prom_shader_registry_reduction_compute_implementation_at(size_t index) { return index < prom_shader_registry_reduction_compute_implementation_count() ? &k_reduction_compute_implementations[index] : NULL; }
size_t prom_shader_registry_experimental_compute_implementation_count(void) { return 0u; }
const prom_compute_implementation* prom_shader_registry_experimental_compute_implementation_at(size_t index) { (void)index; return NULL; }
const prom_sgemm_kernel_dispatch_metadata* prom_shader_registry_dispatch_metadata(uint32_t id) { const prom_compute_implementation* impl=prom_shader_registry_find_compute_implementation(id); return impl == NULL ? NULL : impl->dispatch; }
uint32_t prom_shader_registry_is_dispatchable(uint32_t id) { const prom_compute_implementation* impl=prom_shader_registry_find_compute_implementation(id); return impl != NULL && impl->dispatchable != 0u; }
uint32_t prom_shader_registry_is_selector_eligible(uint32_t id) { const prom_compute_implementation* impl=prom_shader_registry_find_compute_implementation(id); return impl != NULL && impl->selector_eligible != 0u; }
uint32_t prom_shader_registry_validate_tables(const prom_shader_asset* assets, size_t asset_count,
                                              const prom_compute_implementation* implementations, size_t implementation_count) {
  if (assets == NULL || implementations == NULL || asset_count == 0u || implementation_count == 0u) return 0u;
  for (size_t i=0u;i<asset_count;++i) { const prom_shader_asset* a=&assets[i]; if(a->shader_id==0u||a->stage!=PROM_SHADER_STAGE_COMPUTE||a->entry_point==NULL||a->entry_point[0]=='\0'||a->spirv_words==NULL||a->spirv_size_bytes==0u||(a->spirv_size_bytes%sizeof(uint32_t))!=0u||(a->contains_inline_hlsl==0u&&a->inline_hlsl_block_count!=0u)||(a->contains_inline_hlsl!=0u&&(a->inline_hlsl_block_count==0u||a->foreign_targets==NULL||a->foreign_targets[0]=='\0'))) return 0u; for(size_t j=i+1u;j<asset_count;++j) if(a->shader_id==assets[j].shader_id) return 0u; }
  for (size_t i=0u;i<implementation_count;++i) { const prom_compute_implementation* c=&implementations[i]; const prom_shader_asset* a=NULL; for(size_t k=0u;k<asset_count;++k) if(assets[k].shader_id==c->shader_id) { a=&assets[k]; break; } if(c->implementation_id==0u||a==NULL||a->stage!=PROM_SHADER_STAGE_COMPUTE||a->authority!=c->authority||(c->selector_eligible!=0u&&c->dispatchable==0u)||(c->benchmark_enabled!=0u&&c->dispatchable==0u)) return 0u; if(c->operation_id==PROM_SHADER_OPERATION_SGEMM){if(c->dispatch==NULL||c->pipeline_slot>=PROM_COMPUTE_PIPELINE_COUNT||c->pipeline_family!=PROM_COMPUTE_PIPELINE_FAMILY_SGEMM)return 0u;}else if(c->operation_id==PROM_SHADER_OPERATION_REDUCTION_SUM||c->operation_id==PROM_SHADER_OPERATION_REDUCTION_MAX||c->operation_id==PROM_SHADER_OPERATION_SOFTMAX){if(c->reduction_dispatch==NULL||c->pipeline_family!=PROM_COMPUTE_PIPELINE_FAMILY_REDUCTION||c->selector_eligible!=0u)return 0u;}else{return 0u;} for(size_t j=i+1u;j<implementation_count;++j) if(c->implementation_id==implementations[j].implementation_id) return 0u; } return 1u;
}
uint32_t prom_shader_registry_validate(void) {
  const prom_shader_asset* serial;
  const prom_shader_asset* subgroup_owned;
  if(!prom_shader_registry_validate_tables(k_shader_assets, prom_shader_registry_shader_asset_count(), k_compute_implementations, prom_shader_registry_compute_implementation_count()))return 0u;
  if(!prom_shader_registry_validate_tables(k_reduction_shader_assets, prom_shader_registry_reduction_shader_asset_count(), k_reduction_compute_implementations, prom_shader_registry_reduction_compute_implementation_count()))return 0u;
  for(size_t i=0u;i<prom_shader_registry_shader_asset_count();++i)for(size_t j=0u;j<prom_shader_registry_reduction_shader_asset_count();++j)if(k_shader_assets[i].shader_id==k_reduction_shader_assets[j].shader_id)return 0u;
  for(size_t i=0u;i<prom_shader_registry_compute_implementation_count();++i)for(size_t j=0u;j<prom_shader_registry_reduction_compute_implementation_count();++j)if(k_compute_implementations[i].implementation_id==k_reduction_compute_implementations[j].implementation_id)return 0u;
  serial = prom_shader_registry_find_shader(41u);
  subgroup_owned = prom_shader_registry_find_shader(49u);
  if (serial == NULL || subgroup_owned == NULL ||
      serial->authority != PROM_SHADER_AUTHORITY_PRODUCTION ||
      subgroup_owned->authority != PROM_SHADER_AUTHORITY_PRODUCTION ||
      serial->source_language != PROM_SHADER_SOURCE_SDSLV ||
      subgroup_owned->source_language != PROM_SHADER_SOURCE_SDSLV ||
      strcmp(serial->source_path, "internal/prometheus/shaders/sdslv/production/zimage/main_transformer_joint_attention_streaming.sdslv") != 0 ||
      strcmp(subgroup_owned->source_path, "internal/prometheus/shaders/sdslv/production/zimage/main_transformer_joint_attention_builtin_topology.sdslv") != 0)
    return 0u;
  return 1u;
}
void prom_shader_registry_initialize_pipeline_instances(prom_compute_pipeline_instance* instances,size_t count,const VkPipeline* pipelines,size_t pipeline_count) { for(size_t i=0u;i<count;++i){ instances[i].implementation=prom_shader_registry_compute_implementation_at(i); instances[i].pipeline=VK_NULL_HANDLE; instances[i].status=PROM_PIPELINE_UNAVAILABLE; instances[i].failure_detail=0; if(instances[i].implementation!=NULL&&instances[i].implementation->pipeline_slot<pipeline_count){instances[i].pipeline=pipelines[instances[i].implementation->pipeline_slot];instances[i].status=instances[i].pipeline!=VK_NULL_HANDLE?PROM_PIPELINE_READY:PROM_PIPELINE_FAILED;} } }
VkPipeline prom_shader_registry_pipeline_for_variant(const prom_compute_pipeline_instance* instances,size_t count,uint32_t id) { for(size_t i=0u;i<count;++i) if(instances[i].implementation!=NULL&&instances[i].implementation->implementation_id==id&&instances[i].status==PROM_PIPELINE_READY) return instances[i].pipeline; return VK_NULL_HANDLE; }
