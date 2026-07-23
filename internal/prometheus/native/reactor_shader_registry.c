#include "reactor_shader_registry.h"
#include "reactor_api.h"

/* This is a small, binary-free selector projection.  The complete factual
 * catalog, provenance, artifact digests, and object bytes are owned by the
 * per-runtime prometheus.core package.  Kernel N maps exactly to the emitted
 * package variant kernel-N-default. */
#define PROM_STRINGIFY_INNER(value) #value
#define PROM_STRINGIFY(value) PROM_STRINGIFY_INNER(value)
#define META(id, label, language, entry, auth) \
  { id##u, label, PROM_SHADER_STAGE_COMPUTE, "kernel-" PROM_STRINGIFY(id) "-default", \
    entry, 0u, language, NULL, NULL, 0u, 0u, 0u, NULL, auth, 0u, 0u, 0u, 0u, 0u }
#define REDUCTION_ASSET(id, label, entry, role) \
  { id##u, label, PROM_SHADER_STAGE_COMPUTE, "kernel-" PROM_STRINGIFY(id) "-default", \
    entry, 0u, PROM_SHADER_SOURCE_SDSLV, NULL, "reactor_vulkan_reduction_row_sum_spirv.h", 0u, 0u, 0u, NULL, PROM_SHADER_AUTHORITY_PRODUCTION, 4u, 32u, role, 0u, 0u }
#define META_DETAIL(id, label, language, entry, source, header, generated, inline_hlsl, inline_count, targets, auth, bindings, push, minimum, maximum) \
  { id##u, label, PROM_SHADER_STAGE_COMPUTE, "kernel-" PROM_STRINGIFY(id) "-default", entry, 0u, language, source, header, generated, inline_hlsl, inline_count, targets, auth, bindings, push, 0u, minimum, maximum }

static const prom_sgemm_kernel_dispatch_metadata k_meta_8x8 =
    { 8u, 8u, 1u, 1u, 8u, 8u, 8u, 1u };
static const prom_sgemm_kernel_dispatch_metadata k_meta_16x16 =
    { 16u, 16u, 1u, 1u, 16u, 16u, 16u, 1u };
static const prom_sgemm_kernel_dispatch_metadata k_meta_reg2x2 =
    { 8u, 8u, 2u, 2u, 16u, 16u, 16u, 1u };
static const prom_sgemm_kernel_dispatch_metadata k_meta_reg2x4 =
    { 8u, 8u, 2u, 4u, 16u, 32u, 16u, 1u };

static const prom_shader_asset k_shader_assets[] = {
  META(1, "sgemm-baseline-scalar", PROM_SHADER_SOURCE_SPIRV, "main", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(2, "sgemm-tiled", PROM_SHADER_SOURCE_SPIRV, "main", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(3, "sgemm-memory-conservative", PROM_SHADER_SOURCE_SPIRV, "main", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(4, "sgemm-sdsl-scalar-plus", PROM_SHADER_SOURCE_SDSLV, "SgemmScalarBaselinePlus8x8_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(5, "sgemm-sdsl-tile16", PROM_SHADER_SOURCE_SDSLV, "SgemmTile16x16SharedFp32_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(6, "sgemm-sdsl-reg2x2", PROM_SHADER_SOURCE_SDSLV, "SgemmReg2x2Tile16x16Fp32Kernel_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(7, "sgemm-sdsl-exacttail", PROM_SHADER_SOURCE_SDSLV, "SgemmReg2x2Tile16x16ExactTailFp32Kernel_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(8, "sgemm-sdsl-flowboard", PROM_SHADER_SOURCE_SDSLV, "SgemmReg2x2Tile16x16FlowBoardFp32Kernel_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(9, "sgemm-sdsl-derive", PROM_SHADER_SOURCE_SDSLV, "SgemmReg2x2Tile16x16DeriveFp32Kernel_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META_DETAIL(10, "sgemm-srt-2accum", PROM_SHADER_SOURCE_SDSLV, "SgemmSrt2AccumK_CS", "internal/prometheus/shaders/sdslv/production/sgemm/sgemm_srt_2accum_k.sdslv", "reactor_vulkan_sgemm_srt_2accum_k_spirv.h", 1u, 0u, 0u, NULL, PROM_SHADER_AUTHORITY_PRODUCTION, 0u, 0u, 0u, 0u),
  META_DETAIL(11, "sgemm-b2x2", PROM_SHADER_SOURCE_SDSLV, "SgemmB2x2_CS", "internal/prometheus/shaders/sdslv/production/sgemm/sgemm_b2x2_row_major_biased.sdslv", "reactor_vulkan_sgemm_b2x2_row_major_biased_spirv.h", 1u, 0u, 0u, NULL, PROM_SHADER_AUTHORITY_PRODUCTION, 0u, 0u, 0u, 0u),
  META_DETAIL(12, "sgemm-a2x4", PROM_SHADER_SOURCE_SDSLV, "SgemmA2x4_CS", "internal/prometheus/shaders/sdslv/production/sgemm/sgemm_a2x4_row_biased_accum8.sdslv", "reactor_vulkan_sgemm_a2x4_row_biased_accum8_spirv.h", 1u, 0u, 0u, NULL, PROM_SHADER_AUTHORITY_PRODUCTION, 0u, 0u, 0u, 0u),
  META_DETAIL(13, "sgemm-packed4", PROM_SHADER_SOURCE_SDSLV, "SgemmPacked4_CS", "internal/prometheus/shaders/sdslv/production/sgemm/sgemm_packed4_fp32.sdslv", "reactor_vulkan_packed4_spirv.h", 1u, 0u, 0u, NULL, PROM_SHADER_AUTHORITY_PRODUCTION, 0u, 0u, 0u, 0u),
  META_DETAIL(14, "sgemm-fp16-storage-fp32-accum", PROM_SHADER_SOURCE_SDSLV, "SgemmFp16StorageFp32Accum_CS", "internal/prometheus/shaders/sdslv/production/sgemm/sgemm_fp16_storage_fp32_accum.sdslv", "reactor_vulkan_fp16_spirv.h", 1u, 0u, 0u, NULL, PROM_SHADER_AUTHORITY_PRODUCTION, 0u, 0u, 0u, 0u),
  META_DETAIL(15, "sdslv-inline-hlsl-bitcast-proof", PROM_SHADER_SOURCE_SDSLV, "InlineHlslBitCastProof_CS", "internal/prometheus/shaders/sdslv/production/sgemm/inline_hlsl_bitcast_proof.sdslv", "reactor_vulkan_inline_hlsl_bitcast_proof_spirv.h", 1u, 1u, 2u, "HLSL", PROM_SHADER_AUTHORITY_PRODUCTION, 0u, 0u, 0u, 0u),
  META_DETAIL(23, "model-block-resident-identity", PROM_SHADER_SOURCE_SDSLV, "ResidentModelBlockIdentity_CS", "internal/prometheus/shaders/sdslv/production/model_block/resident_identity.sdslv", NULL, 0u, 0u, 0u, NULL, PROM_SHADER_AUTHORITY_PRODUCTION, 3u, 8u, 0u, 0u),
  META(24, "zimage-nr0-adaln", PROM_SHADER_SOURCE_SDSLV, "Nr0AdaLN_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(25, "zimage-nr0-attention-norm-modulate", PROM_SHADER_SOURCE_SDSLV, "Nr0AttentionNormModulate_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(26, "zimage-nr0-fused-qkv", PROM_SHADER_SOURCE_SDSLV, "Nr0FusedQkv_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(27, "zimage-nr0-q-norm-rope", PROM_SHADER_SOURCE_SDSLV, "Nr0QueryNormRope_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(28, "zimage-nr0-k-norm-rope", PROM_SHADER_SOURCE_SDSLV, "Nr0KeyNormRope_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(29, "zimage-nr0-bf16-ingress", PROM_SHADER_SOURCE_SDSLV, "Nr0Bf16Ingress_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(30, "zimage-nr0-attention-streaming", PROM_SHADER_SOURCE_SDSLV, "Nr0AttentionStreaming_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(31, "zimage-nr0-attention-projection", PROM_SHADER_SOURCE_SDSLV, "Nr0AttentionProjection_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(32, "zimage-nr0-attention-residual", PROM_SHADER_SOURCE_SDSLV, "Nr0AttentionResidual_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(33, "zimage-nr0-ffn-norm-modulate", PROM_SHADER_SOURCE_SDSLV, "Nr0FfnNormModulate_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(34, "zimage-nr0-ffn-w1-w3", PROM_SHADER_SOURCE_SDSLV, "Nr0FfnW1W3_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(35, "zimage-nr0-ffn-gate", PROM_SHADER_SOURCE_SDSLV, "Nr0FfnGate_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(36, "zimage-nr0-ffn-w2-residual", PROM_SHADER_SOURCE_SDSLV, "Nr0FfnW2Residual_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META_DETAIL(37, "zimage-nr0-persistent-audit-summary", PROM_SHADER_SOURCE_SDSLV, "Nr0PersistentAuditSummary_CS", "internal/prometheus/shaders/sdslv/production/zimage/nr0_persistent_audit_summary.sdslv", NULL, 0u, 0u, 0u, NULL, PROM_SHADER_AUTHORITY_PRODUCTION, 4u, 96u, 0u, 0u),
  META(38, "zimage-context-refiner-qk-norm-rope", PROM_SHADER_SOURCE_SDSLV, "ContextQkNormRope_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(39, "zimage-context-refiner-attention-streaming", PROM_SHADER_SOURCE_SDSLV, "ContextAttentionStreaming_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META_DETAIL(40, "zimage-main-transformer-joint-qk-norm-rope", PROM_SHADER_SOURCE_SDSLV, "MainTransformerJointQkNormRope_CS", "internal/prometheus/shaders/sdslv/production/zimage/main_transformer_joint_qk_norm_rope.sdslv", NULL, 0u, 0u, 0u, NULL, PROM_SHADER_AUTHORITY_PRODUCTION, 0u, 24u, 0u, 0u),
  META_DETAIL(41, "zimage-main-transformer-joint-attention-streaming", PROM_SHADER_SOURCE_SDSLV, "MainTransformerJointAttentionStreaming_CS", "internal/prometheus/shaders/sdslv/production/zimage/main_transformer_joint_attention_streaming.sdslv", NULL, 0u, 0u, 0u, NULL, PROM_SHADER_AUTHORITY_PRODUCTION, 0u, 0u, 1056u, 1056u),
  META(42, "zimage-main-transformer-ffn-w1-w3", PROM_SHADER_SOURCE_SDSLV, "MainTransformerFfnW1W3_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META_DETAIL(43, "zimage-main-transformer-ffn-gate", PROM_SHADER_SOURCE_SDSLV, "MainTransformerFfnGate_CS", NULL, NULL, 0u, 0u, 0u, NULL, PROM_SHADER_AUTHORITY_PRODUCTION, 0u, 0u, 0u, 10813440u),
  META(44, "zimage-main-transformer-joint-attention-subgroup-owned", PROM_SHADER_SOURCE_SDSLV, "MainTransformerJointAttentionSubgroupOwned_CS", PROM_SHADER_AUTHORITY_EXPERIMENTAL),
  META_DETAIL(45, "zimage-main-transformer-joint-attention-gemini-exact", PROM_SHADER_SOURCE_HLSL, "MainTransformerJointAttentionSubgroupOwned_CS", "internal/prometheus/shaders/hlsl/external/m5b-gemini-exact.hlsl", NULL, 0u, 0u, 0u, NULL, PROM_SHADER_AUTHORITY_EXPERIMENTAL, 0u, 0u, 0u, 0u),
  META_DETAIL(46, "zimage-main-transformer-joint-attention-gemini-inplace", PROM_SHADER_SOURCE_HLSL, "MainTransformerJointAttentionSubgroupOwned_CS", "internal/prometheus/shaders/hlsl/experimental/m5b-gemini-inplace.hlsl", NULL, 0u, 0u, 0u, NULL, PROM_SHADER_AUTHORITY_EXPERIMENTAL, 0u, 0u, 0u, 0u),
  META(47, "zimage-main-transformer-joint-attention-subgroup-owned32", PROM_SHADER_SOURCE_SDSLV, "MainTransformerJointAttentionSubgroupOwned_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META(48, "zimage-main-transformer-subgroup-owned32-topology-probe", PROM_SHADER_SOURCE_SDSLV, "MainTransformerSubgroupOwned32TopologyProbe_CS", PROM_SHADER_AUTHORITY_PRODUCTION),
  META_DETAIL(49, "zimage-main-transformer-joint-attention-builtin-topology", PROM_SHADER_SOURCE_SDSLV, "MainTransformerJointAttentionBuiltinTopology_CS", "internal/prometheus/shaders/sdslv/production/zimage/main_transformer_joint_attention_builtin_topology.sdslv", NULL, 0u, 0u, 0u, NULL, PROM_SHADER_AUTHORITY_PRODUCTION, 0u, 0u, 0u, 0u),
  META(50, "dvt2-m6a-pack-activation-f32-to-f16", PROM_SHADER_SOURCE_SDSLV, "Dvt2M6aPackActivationF32ToF16_CS", PROM_SHADER_AUTHORITY_EXPERIMENTAL),
  META(51, "dvt2-m6a-w1-w3-cooperative-f16-f32", PROM_SHADER_SOURCE_SDSLV, "Dvt2M6aW1W3CooperativeF16F32_CS", PROM_SHADER_AUTHORITY_EXPERIMENTAL),
};

static const prom_shader_asset k_reduction_shader_assets[] = {
  REDUCTION_ASSET(16, "reduction-row-sum-stage", "RowSumStage_CS", PROM_REDUCTION_STAGE_ROW_SUM),
  REDUCTION_ASSET(17, "reduction-row-max-stage", "RowMaxStage_CS", PROM_REDUCTION_STAGE_ROW_MAX),
  REDUCTION_ASSET(18, "reduction-softmax-exp-sum-stage", "SoftmaxExpSumStage_CS", PROM_REDUCTION_STAGE_SOFTMAX_EXP_SUM),
  REDUCTION_ASSET(19, "reduction-softmax-normalize", "SoftmaxNormalize_CS", PROM_REDUCTION_STAGE_SOFTMAX_NORMALIZE),
  REDUCTION_ASSET(20, "reduction-softmax-fused", "SoftmaxFused_CS", PROM_REDUCTION_STAGE_SOFTMAX_FUSED),
  REDUCTION_ASSET(21, "reduction-row-sum-packed-short", "RowSumPackedShort_CS", PROM_REDUCTION_STAGE_ROW_SUM_PACKED_SHORT),
  REDUCTION_ASSET(22, "reduction-softmax-packed-short", "SoftmaxPackedShort_CS", PROM_REDUCTION_STAGE_SOFTMAX_PACKED_SHORT),
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
static const prom_reduction_kernel_dispatch_metadata k_reduction_sum_meta = REDUCTION_META(PROM_REDUCTION_STAGE_ROW_SUM, PROM_REDUCTION_MAX_ELEMENTS_PER_ROW);
static const prom_reduction_kernel_dispatch_metadata k_reduction_max_meta = REDUCTION_META(PROM_REDUCTION_STAGE_ROW_MAX, PROM_REDUCTION_MAX_ELEMENTS_PER_ROW);
static const prom_reduction_kernel_dispatch_metadata k_softmax_exp_sum_meta = REDUCTION_META(PROM_REDUCTION_STAGE_SOFTMAX_EXP_SUM, PROM_REDUCTION_MAX_ELEMENTS_PER_ROW);
static const prom_reduction_kernel_dispatch_metadata k_softmax_normalize_meta = REDUCTION_META(PROM_REDUCTION_STAGE_SOFTMAX_NORMALIZE, PROM_REDUCTION_MAX_ELEMENTS_PER_ROW);
static const prom_reduction_kernel_dispatch_metadata k_softmax_fused_meta = REDUCTION_META(PROM_REDUCTION_STAGE_SOFTMAX_FUSED, PROM_REDUCTION_SINGLE_STAGE_THRESHOLD);
static const prom_reduction_kernel_dispatch_metadata k_reduction_sum_packed_short_meta = REDUCTION_META(PROM_REDUCTION_STAGE_ROW_SUM_PACKED_SHORT, PROM_REDUCTION_PACKED_SHORT_WIDTH_MAX);
static const prom_reduction_kernel_dispatch_metadata k_softmax_packed_short_meta = REDUCTION_META(PROM_REDUCTION_STAGE_SOFTMAX_PACKED_SHORT, PROM_REDUCTION_PACKED_SHORT_WIDTH_MAX);
#define REDUCTION_IMPL(id, operation, label, shader, metadata, index) \
  { id, operation, label, shader, NULL, 0u, 1u, 0u, 1u, PROM_COMPUTE_PIPELINE_COUNT, metadata, \
    PROM_SHADER_AUTHORITY_PRODUCTION, PROM_COMPUTE_PIPELINE_FAMILY_REDUCTION, index }
static const prom_compute_implementation k_reduction_compute_implementations[] = {
  REDUCTION_IMPL(PROM_REDUCTION_IMPLEMENTATION_ROW_SUM, PROM_SHADER_OPERATION_REDUCTION_SUM, "reduction-row-sum-stage", PROM_REDUCTION_SHADER_ROW_SUM, &k_reduction_sum_meta, 0u),
  REDUCTION_IMPL(PROM_REDUCTION_IMPLEMENTATION_ROW_MAX, PROM_SHADER_OPERATION_REDUCTION_MAX, "reduction-row-max-stage", PROM_REDUCTION_SHADER_ROW_MAX, &k_reduction_max_meta, 1u),
  REDUCTION_IMPL(PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_EXP_SUM, PROM_SHADER_OPERATION_SOFTMAX, "reduction-softmax-exp-sum-stage", PROM_REDUCTION_SHADER_SOFTMAX_EXP_SUM, &k_softmax_exp_sum_meta, 2u),
  REDUCTION_IMPL(PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_NORMALIZE, PROM_SHADER_OPERATION_SOFTMAX, "reduction-softmax-normalize", PROM_REDUCTION_SHADER_SOFTMAX_NORMALIZE, &k_softmax_normalize_meta, 3u),
  REDUCTION_IMPL(PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_FUSED, PROM_SHADER_OPERATION_SOFTMAX, "reduction-softmax-fused", PROM_REDUCTION_SHADER_SOFTMAX_FUSED, &k_softmax_fused_meta, 4u),
  REDUCTION_IMPL(PROM_REDUCTION_IMPLEMENTATION_ROW_SUM_PACKED_SHORT, PROM_SHADER_OPERATION_REDUCTION_SUM, "reduction-row-sum-packed-short", PROM_REDUCTION_SHADER_ROW_SUM_PACKED_SHORT, &k_reduction_sum_packed_short_meta, 5u),
  REDUCTION_IMPL(PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_PACKED_SHORT, PROM_SHADER_OPERATION_SOFTMAX, "reduction-softmax-packed-short", PROM_REDUCTION_SHADER_SOFTMAX_PACKED_SHORT, &k_softmax_packed_short_meta, 6u),
};

const prom_shader_asset* prom_shader_registry_find_shader(uint32_t id) { size_t i; for (i=0u;i<sizeof(k_shader_assets)/sizeof(k_shader_assets[0]);++i) if(k_shader_assets[i].shader_id==id) return &k_shader_assets[i]; for (i=0u;i<sizeof(k_reduction_shader_assets)/sizeof(k_reduction_shader_assets[0]);++i) if(k_reduction_shader_assets[i].shader_id==id) return &k_reduction_shader_assets[i]; return NULL; }
size_t prom_shader_registry_shader_asset_count(void) { return sizeof(k_shader_assets)/sizeof(k_shader_assets[0]); }
const prom_shader_asset* prom_shader_registry_shader_asset_at(size_t index) { return index < prom_shader_registry_shader_asset_count() ? &k_shader_assets[index] : NULL; }
size_t prom_shader_registry_reduction_shader_asset_count(void) { return sizeof(k_reduction_shader_assets)/sizeof(k_reduction_shader_assets[0]); }
const prom_shader_asset* prom_shader_registry_reduction_shader_asset_at(size_t index) { return index < prom_shader_registry_reduction_shader_asset_count() ? &k_reduction_shader_assets[index] : NULL; }
size_t prom_shader_registry_experimental_shader_asset_count(void) { return 0u; }
const prom_shader_asset* prom_shader_registry_experimental_shader_asset_at(size_t index) { (void)index; return NULL; }
const prom_compute_implementation* prom_shader_registry_find_compute_implementation(uint32_t id) { size_t i; for(i=0u;i<sizeof(k_compute_implementations)/sizeof(k_compute_implementations[0]);++i) if(k_compute_implementations[i].implementation_id==id) return &k_compute_implementations[i]; for(i=0u;i<sizeof(k_reduction_compute_implementations)/sizeof(k_reduction_compute_implementations[0]);++i) if(k_reduction_compute_implementations[i].implementation_id==id) return &k_reduction_compute_implementations[i]; return NULL; }
size_t prom_shader_registry_compute_implementation_count(void) { return sizeof(k_compute_implementations)/sizeof(k_compute_implementations[0]); }
const prom_compute_implementation* prom_shader_registry_compute_implementation_at(size_t index) { return index < prom_shader_registry_compute_implementation_count() ? &k_compute_implementations[index] : NULL; }
size_t prom_shader_registry_reduction_compute_implementation_count(void) { return sizeof(k_reduction_compute_implementations)/sizeof(k_reduction_compute_implementations[0]); }
const prom_compute_implementation* prom_shader_registry_reduction_compute_implementation_at(size_t index) { return index < prom_shader_registry_reduction_compute_implementation_count() ? &k_reduction_compute_implementations[index] : NULL; }
size_t prom_shader_registry_experimental_compute_implementation_count(void) { return 0u; }
const prom_compute_implementation* prom_shader_registry_experimental_compute_implementation_at(size_t index) { (void)index; return NULL; }
const prom_sgemm_kernel_dispatch_metadata* prom_shader_registry_dispatch_metadata(uint32_t id) { const prom_compute_implementation* impl=prom_shader_registry_find_compute_implementation(id); return impl == NULL ? NULL : impl->dispatch; }
uint32_t prom_shader_registry_is_dispatchable(uint32_t id) { const prom_compute_implementation* impl=prom_shader_registry_find_compute_implementation(id); return impl != NULL && impl->dispatchable != 0u; }
uint32_t prom_shader_registry_is_selector_eligible(uint32_t id) { const prom_compute_implementation* impl=prom_shader_registry_find_compute_implementation(id); return impl != NULL && impl->selector_eligible != 0u; }
uint32_t prom_shader_registry_validate_tables(const prom_shader_asset* assets, size_t asset_count, const prom_compute_implementation* implementations, size_t implementation_count) { size_t i,j,k; if(assets==NULL||implementations==NULL||asset_count==0u||implementation_count==0u)return 0u; for(i=0u;i<asset_count;++i){const prom_shader_asset* a=&assets[i]; if(a->shader_id==0u||a->stage!=PROM_SHADER_STAGE_COMPUTE||a->package_variant_id==NULL||a->entry_point==NULL||a->entry_point[0]=='\0')return 0u; for(j=i+1u;j<asset_count;++j)if(a->shader_id==assets[j].shader_id)return 0u;} for(i=0u;i<implementation_count;++i){const prom_compute_implementation* c=&implementations[i];const prom_shader_asset* a=NULL;for(k=0u;k<asset_count;++k)if(assets[k].shader_id==c->shader_id){a=&assets[k];break;}if(c->implementation_id==0u||a==NULL||a->authority!=c->authority||(c->selector_eligible!=0u&&c->dispatchable==0u))return 0u;}return 1u; }
uint32_t prom_shader_registry_validate(void) { return prom_shader_registry_validate_tables(k_shader_assets,prom_shader_registry_shader_asset_count(),k_compute_implementations,prom_shader_registry_compute_implementation_count()) && prom_shader_registry_validate_tables(k_reduction_shader_assets,prom_shader_registry_reduction_shader_asset_count(),k_reduction_compute_implementations,prom_shader_registry_reduction_compute_implementation_count()); }
void prom_shader_registry_initialize_pipeline_instances(prom_compute_pipeline_instance* instances,size_t count,const VkPipeline* pipelines,size_t pipeline_count) { size_t i; for(i=0u;i<count;++i){instances[i].implementation=prom_shader_registry_compute_implementation_at(i);instances[i].pipeline=VK_NULL_HANDLE;instances[i].status=PROM_PIPELINE_UNAVAILABLE;instances[i].failure_detail=0;if(instances[i].implementation!=NULL&&instances[i].implementation->pipeline_slot<pipeline_count){instances[i].pipeline=pipelines[instances[i].implementation->pipeline_slot];instances[i].status=instances[i].pipeline!=VK_NULL_HANDLE?PROM_PIPELINE_READY:PROM_PIPELINE_FAILED;}} }
VkPipeline prom_shader_registry_pipeline_for_variant(const prom_compute_pipeline_instance* instances,size_t count,uint32_t id) { size_t i; for(i=0u;i<count;++i)if(instances[i].implementation!=NULL&&instances[i].implementation->implementation_id==id&&instances[i].status==PROM_PIPELINE_READY)return instances[i].pipeline;return VK_NULL_HANDLE; }
