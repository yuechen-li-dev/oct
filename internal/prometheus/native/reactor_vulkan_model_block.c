#include "reactor_vulkan_runtime_internal.h"

#include <stdio.h>
#include <stdlib.h>
#include "reactor_shader_registry.h"
#include "../models/zimage-turbo/resolved_descriptor.h"
#include "../models/zimage-turbo/resolved_audit_schedule.h"

#include <limits.h>

/* EVT-2 M1a deliberately owns one compiled command program.  This is not a
   graph executor: the create request must spell the one closed seven-step
   program below and the only accepted shader is the production resident proof.
   Subsequent model milestones replace that proof portfolio with additional
   fixed program steps; they do not add runtime operator selection. */

#define PROM_MODEL_BLOCK_RESIDENT_SHADER_ID 23u
#define PROM_MODEL_BLOCK_WORKGROUP_SIZE 256u
#define PROM_MODEL_BLOCK_M1B_ADALN_SHADER_ID 24u
#define PROM_MODEL_BLOCK_M1B_NORM_SHADER_ID 25u
#define PROM_MODEL_BLOCK_M1B_QKV_SHADER_ID 26u
#define PROM_MODEL_BLOCK_M1B_Q_ROPE_SHADER_ID 27u
#define PROM_MODEL_BLOCK_M1B_K_ROPE_SHADER_ID 28u
#define PROM_MODEL_BLOCK_M1B_INGRESS_SHADER_ID 29u
#define PROM_MODEL_BLOCK_M1C_ATTENTION_SHADER_ID 30u
#define PROM_MODEL_BLOCK_M1C_PROJECTION_SHADER_ID 31u
#define PROM_MODEL_BLOCK_M1C_RESIDUAL_SHADER_ID 32u
#define PROM_MODEL_BLOCK_M1D_NORM_SHADER_ID 33u
#define PROM_MODEL_BLOCK_M1D_W1_W3_SHADER_ID 34u
#define PROM_MODEL_BLOCK_M1D_GATE_SHADER_ID 35u
#define PROM_MODEL_BLOCK_M1D_W2_RESIDUAL_SHADER_ID 36u
#define PROM_MODEL_BLOCK_AUDIT_SUMMARY_SHADER_ID 37u
#define PROM_MODEL_BLOCK_CONTEXT_QK_ROPE_SHADER_ID 38u
#define PROM_MODEL_BLOCK_CONTEXT_ATTENTION_SHADER_ID 39u
#define PROM_MODEL_BLOCK_MAIN_QK_ROPE_SHADER_ID 40u
#define PROM_MODEL_BLOCK_MAIN_ATTENTION_SERIAL_SHADER_ID 41u
#define PROM_MODEL_BLOCK_MAIN_ATTENTION_SUBGROUP_OWNED32_SHADER_ID 47u
#define PROM_MODEL_BLOCK_MAIN_ATTENTION_SERIAL_GROUPS 31680u
#define PROM_MODEL_BLOCK_MAIN_ATTENTION_SUBGROUP_OWNED32_GROUPS 3960u
#define PROM_MODEL_BLOCK_MAIN_W1_W3_SHADER_ID 42u
#define PROM_MODEL_BLOCK_MAIN_GATE_SHADER_ID 43u
#define PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS (1024u * 3840u)
#define PROM_MODEL_BLOCK_M1B_TIMESTEP_ELEMENTS 256u
#define PROM_MODEL_BLOCK_M1B_QKV_ELEMENTS (1024u * 11520u)
#define PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS 3840u
#define PROM_MODEL_BLOCK_M1D_HIDDEN_ELEMENTS (1024u * 10240u)
#define PROM_MODEL_BLOCK_CONTEXT_TOKENS 32u
#define PROM_MODEL_BLOCK_CONTEXT_MODEL_ELEMENTS (PROM_MODEL_BLOCK_CONTEXT_TOKENS * 3840u)
#define PROM_MODEL_BLOCK_CONTEXT_QKV_ELEMENTS (PROM_MODEL_BLOCK_CONTEXT_TOKENS * 11520u)
#define PROM_MODEL_BLOCK_CONTEXT_HIDDEN_ELEMENTS (PROM_MODEL_BLOCK_CONTEXT_TOKENS * 10240u)
#define PROM_MODEL_BLOCK_MAIN_TOKENS 1056u
#define PROM_MODEL_BLOCK_MAIN_IMAGE_TOKENS 1024u
#define PROM_MODEL_BLOCK_MAIN_MODEL_ELEMENTS (PROM_MODEL_BLOCK_MAIN_TOKENS * 3840u)
#define PROM_MODEL_BLOCK_MAIN_QKV_ELEMENTS (PROM_MODEL_BLOCK_MAIN_TOKENS * 11520u)
#define PROM_MODEL_BLOCK_MAIN_HIDDEN_ELEMENTS (PROM_MODEL_BLOCK_MAIN_TOKENS * 10240u)
#define PROM_MODEL_BLOCK_MAIN_QUERY_COUNT 16u
#define PROM_MODEL_BLOCK_M1B_BF16_BYTES(elements) ((VkDeviceSize)(elements) * sizeof(uint16_t))
#define PROM_MODEL_BLOCK_M1B_FP32_BYTES(elements) ((VkDeviceSize)(elements) * sizeof(float))

typedef struct prom_model_block_m1b_ingress_constants {
  uint32_t input_elements;
  uint32_t timestep_elements;
} prom_model_block_m1b_ingress_constants;

typedef struct prom_model_block_m1b_adaln_constants {
  uint32_t projection_width;
  uint32_t input_width;
} prom_model_block_m1b_adaln_constants;

typedef struct prom_model_block_m1b_norm_constants {
  float epsilon;
  uint32_t token_count;
  uint32_t width;
  uint32_t reserved;
} prom_model_block_m1b_norm_constants;

typedef struct prom_model_block_m1b_qkv_constants {
  uint32_t token_count;
  uint32_t input_width;
  uint32_t output_width;
  uint32_t reserved;
} prom_model_block_m1b_qkv_constants;

typedef struct prom_model_block_m1b_head_constants {
  float epsilon;
  uint32_t token_count;
  uint32_t head_count;
  uint32_t head_width;
} prom_model_block_m1b_head_constants;

typedef struct prom_model_block_main_qk_constants {
  float epsilon;
  uint32_t token_count;
  uint32_t head_count;
  uint32_t head_width;
  uint32_t segment_offset;
  uint32_t image_tokens;
} prom_model_block_main_qk_constants;

typedef struct prom_model_block_context_qk_constants {
  float epsilon;
  uint32_t token_count;
  uint32_t head_count;
  uint32_t head_width;
  uint32_t segment_offset;
} prom_model_block_context_qk_constants;

typedef struct prom_model_block_m1c_attention_constants {
  uint32_t token_count;
  uint32_t head_count;
  uint32_t head_width;
  uint32_t fused_width;
} prom_model_block_m1c_attention_constants;

typedef struct prom_model_block_m1d_w2_constants {
  float epsilon;
  uint32_t token_count;
  uint32_t model_width;
  uint32_t hidden_width;
} prom_model_block_m1d_w2_constants;

typedef struct prom_model_block_audit_constants {
  uint32_t source_base_element;
  uint32_t element_count;
  uint32_t stage_id;
  uint32_t execution_generation;
  uint32_t part0_element_count;
  uint32_t part1_element_count;
  uint32_t destination_word_offset;
  uint32_t projection_key_count;
  uint32_t source_mode;
  uint32_t projection_keys[15];
} prom_model_block_audit_constants;

typedef struct prom_model_block_m1d_gate_constants {
  uint32_t element_count;
  uint32_t reserved0;
  uint32_t reserved1;
  uint32_t reserved2;
} prom_model_block_m1d_gate_constants;

static const uint32_t k_prom_model_block_m1b_shader_ids[PROM_MODEL_BLOCK_M1B_PIPELINE_COUNT] = {
    PROM_MODEL_BLOCK_M1B_INGRESS_SHADER_ID, PROM_MODEL_BLOCK_M1B_ADALN_SHADER_ID,
    PROM_MODEL_BLOCK_M1B_NORM_SHADER_ID,
    PROM_MODEL_BLOCK_M1B_QKV_SHADER_ID, PROM_MODEL_BLOCK_M1B_Q_ROPE_SHADER_ID,
    PROM_MODEL_BLOCK_M1B_K_ROPE_SHADER_ID};

/* Cache-manifest lexical order.  The native owner never guesses tensor
   layouts: the adapter presents this fixed declaration order after validating
   the names, shapes, individual SHA-256 values, and aggregate identity. */
static const uint64_t k_prom_model_block_m1b_weight_bytes[PROM_MODEL_BLOCK_MAX_WEIGHTS] = {
    30720ull, 7864320ull, 256ull, 29491200ull, 256ull, 88473600ull, 7680ull,
    7680ull, 78643200ull, 78643200ull, 78643200ull, 7680ull, 7680ull};

/* ContextRefiner's manifest-local tensor order is deliberately not its
   physical owner order. These are the generated semantic tensor roles shared
   with the NoiseRefiner/MainTransformer portfolio. */
static const uint32_t k_prom_context_shared_weight_slots[11] = {
    2u, 3u, 4u, 5u, 6u, 7u, 8u, 9u, 10u, 11u, 12u};

typedef struct prom_model_block_push_constants {
  uint32_t element_count;
  uint32_t audit_element_count;
} prom_model_block_push_constants;

static const uint32_t k_prom_model_block_steps[PROM_MODEL_BLOCK_MAX_STEPS] = {
    PROM_MODEL_BLOCK_STEP_BIND_PIPELINE,
    PROM_MODEL_BLOCK_STEP_BIND_RESOURCES,
    PROM_MODEL_BLOCK_STEP_PUSH_CONSTANTS,
    PROM_MODEL_BLOCK_STEP_DISPATCH,
    PROM_MODEL_BLOCK_STEP_BARRIER,
    PROM_MODEL_BLOCK_STEP_AUDIT_COPY,
    PROM_MODEL_BLOCK_STEP_OUTPUT_COPY,
};

static uint64_t prom_context_refiner_expected_aggregate(uint32_t parameter_set);
static int prom_model_block_is_context_refiner(const prom_model_block_state* block);
static uint64_t prom_main_transformer_expected_aggregate(uint32_t parameter_set);
static int prom_model_block_is_main_transformer(const prom_model_block_state* block);
static int prom_context_refiner_create_buffers(prom_reduction_runtime_state* state,
                                               prom_model_block_state* block,
                                               VkDeviceSize max_weight_bytes);
static int prom_main_transformer_create_buffers(prom_reduction_runtime_state* state,
                                                prom_model_block_state* block,
                                                VkDeviceSize max_weight_bytes);
static int prom_main_transformer_record_execute(prom_reduction_runtime_state* state,
                                                prom_model_block_state* block,
                                                prom_compiled_model_session_state* session,
                                                uint64_t requested_joint_generation,
                                                uint32_t resident_chain_mode,
                                                int32_t* out_detail);
static int prom_context_refiner_record_execute(prom_reduction_runtime_state* state,
                                               prom_model_block_state* block, int resident_input,
                                               int32_t* out_detail);
static int prom_context_refiner_record_static_audit(
    prom_reduction_runtime_state* state, prom_model_block_state* block, uint32_t execution_generation,
    int resident_input, int32_t* out_detail);

/* Cache-manifest lexical order for the unmodulated ContextRefiner package. */
static const uint64_t k_prom_model_block_context_weight_bytes[11u] = {
    256ull, 29491200ull, 256ull, 88473600ull, 7680ull, 7680ull,
    78643200ull, 78643200ull, 78643200ull, 7680ull, 7680ull};

static uint64_t prom_model_block_hash_u64(uint64_t hash, uint64_t value) {
  return prom_m40b_hash_u64(hash, value);
}

static uint64_t prom_noise_refiner_expected_aggregate(uint32_t parameter_set) {
  uint32_t index;
  for (index = 0u; index < 2u; ++index) {
    if (k_prom_zimage_turbo_noise_refiner_blocks[index].parameter_set == parameter_set)
      return k_prom_zimage_turbo_noise_refiner_blocks[index].parameter_set_aggregate_identity;
  }
  return 0u;
}

static int prom_model_block_add_bytes(uint64_t* total, uint64_t value) {
  if (total == NULL || value > UINT64_MAX - *total) return 0;
  *total += value;
  return 1;
}

static int prom_model_block_bytes_for_elements(uint64_t elements, uint64_t* bytes) {
  if (bytes == NULL || elements == 0u || elements > UINT64_MAX / sizeof(float)) return 0;
  *bytes = elements * sizeof(float);
  return 1;
}

static int prom_model_block_steps_are_exact(const PrometheusModelBlockCreateRequest* request) {
  uint32_t index;
  if (request == NULL || request->step_count != PROM_MODEL_BLOCK_MAX_STEPS) return 0;
  for (index = 0u; index < PROM_MODEL_BLOCK_MAX_STEPS; ++index) {
    if (request->steps[index] != k_prom_model_block_steps[index]) return 0;
  }
  return 1;
}

static uint64_t prom_model_block_weight_bytes(const prom_model_block_state* block) {
  uint64_t total = 0u;
  uint32_t index;
  if (block == NULL) return 0u;
  for (index = 0u; index < block->weight_count; ++index) {
    if (!prom_model_block_add_bytes(&total, block->weights[index].byte_count)) return 0u;
  }
  if (block->prefetch_queue != VK_NULL_HANDLE) {
    for (index = 0u; index < PROM_MODEL_BLOCK_MAX_WEIGHTS; ++index) {
      if (!prom_model_block_add_bytes(&total, (uint64_t)block->prefetch_weights[index].device.size)) return 0u;
    }
  }
  return total;
}

static void prom_model_block_fill_evidence(const prom_model_block_state* block,
                                           int32_t detail,
                                           PrometheusModelBlockEvidence* out_evidence) {
  uint64_t persistent_bytes;
  uint64_t reusable_bytes;
  uint64_t audit_bytes;
  uint64_t external_bytes;
  uint64_t total_bytes;
  uint32_t boundary;
  if (out_evidence == NULL) return;
  memset(out_evidence, 0, sizeof(*out_evidence));
  out_evidence->struct_size = (uint32_t)sizeof(*out_evidence);
  out_evidence->detail_code = detail;
  if (block == NULL) return;
  persistent_bytes = prom_model_block_weight_bytes(block);
  reusable_bytes = (uint64_t)block->input_upload.size + (uint64_t)block->input_bf16_device.size +
                   (uint64_t)block->input_device.size + (uint64_t)block->resident_boundary_device.size +
                   (uint64_t)block->output_device.size + (uint64_t)block->output_readback.size +
                   (uint64_t)block->weight_upload.size + (uint64_t)block->timestep_upload.size +
                   (uint64_t)block->timestep_bf16_device.size +
                   (uint64_t)block->timestep_device.size + (uint64_t)block->adaln_projection.size +
                   (uint64_t)block->attention_scale.size + (uint64_t)block->attention_gate.size +
                   (uint64_t)block->mlp_scale.size + (uint64_t)block->mlp_gate.size +
                   (uint64_t)block->modulated.size + (uint64_t)block->norm_audit.size +
                   (uint64_t)block->qkv.size + (uint64_t)block->attention.size +
                    (uint64_t)block->attention_projection.size +
                    (uint64_t)block->attention_residual.size + (uint64_t)block->context_unit.size +
                    (uint64_t)block->context_w3.size;
  audit_bytes = (uint64_t)block->audit_device.size + (uint64_t)block->audit_readback.size;
  if (block->shared_owner != 0u) {
    /* The shared target exposes bounded views, not additional allocations.
       Exclude host staging and the four alias views from the device ceiling;
       retain the original four-byte Main ingress reservation as a conservative
       accounting floor so the established one-window ceiling is unchanged. */
    reusable_bytes -= (uint64_t)block->input_upload.size +
                      (uint64_t)block->input_bf16_device.size +
                      (uint64_t)block->resident_boundary_device.size +
                      (uint64_t)block->context_unit.size +
                      (uint64_t)block->context_w3.size;
    reusable_bytes += sizeof(uint32_t);
  }
  external_bytes = block->external_input_bytes + block->external_output_bytes;
  total_bytes = persistent_bytes + reusable_bytes + audit_bytes;
  out_evidence->created = block->created;
  out_evidence->weights_uploaded = block->weights_uploaded;
  out_evidence->quarantined = block->quarantined;
  out_evidence->output_valid = block->output_valid;
  out_evidence->audit_valid = block->audit_valid;
  out_evidence->fixed_step_count = block->step_count;
  out_evidence->weight_count = block->weight_count;
  out_evidence->persistent_bytes = persistent_bytes;
  out_evidence->reusable_bytes = reusable_bytes;
  out_evidence->audit_bytes = audit_bytes;
  out_evidence->external_bytes = external_bytes;
  out_evidence->total_committed_bytes = total_bytes;
  out_evidence->peak_plan_bytes = total_bytes;
  out_evidence->cold_buffer_allocation_count = block->cold_buffer_allocation_count;
  out_evidence->warm_buffer_allocation_count = block->warm_buffer_allocation_count;
  out_evidence->pipeline_create_count = block->pipeline_create_count;
  out_evidence->descriptor_set_count = block->descriptor_set_count;
  out_evidence->weight_upload_count = block->weight_upload_count;
  out_evidence->execution_count = block->execution_count;
  out_evidence->last_execution_ns = block->last_execution_ns;
  out_evidence->execution_plan_identity = block->execution_plan_identity;
  out_evidence->replay_identity = block->replay_identity;
  out_evidence->assembly_family = block->assembly_family;
  out_evidence->parameter_set = block->parameter_set;
  out_evidence->binding_state = block->binding_state;
  out_evidence->active_weight_window = block->active_weight_window;
  out_evidence->parameter_set_aggregate_identity = block->parameter_set_aggregate_identity;
  out_evidence->binding_generation = block->binding_generation;
  out_evidence->output_generation = block->output_generation;
  out_evidence->descriptor_generation = block->descriptor_generation;
  for (boundary = 0u; boundary < PROM_MODEL_BLOCK_M1B_PIPELINE_COUNT; ++boundary) {
    out_evidence->m1b_boundary_gpu_ns[boundary] = block->m1b_boundary_gpu_ns[boundary];
  }
  out_evidence->gpu_total_begin_tick = block->gpu_total_begin_tick;
  out_evidence->gpu_total_end_tick = block->gpu_total_end_tick;
  out_evidence->gpu_total_ns = block->gpu_total_ns;
  out_evidence->gpu_compute_begin_tick = block->gpu_compute_begin_tick;
  out_evidence->gpu_compute_end_tick = block->gpu_compute_end_tick;
  out_evidence->gpu_compute_ns = block->gpu_compute_ns;
  out_evidence->gpu_ingress_transfer_ns = block->gpu_ingress_transfer_ns;
  out_evidence->gpu_joint_copy_ns = block->gpu_joint_copy_ns;
  out_evidence->gpu_readback_ns = block->gpu_readback_ns;
  for (boundary = 0u; boundary < PROM_MODEL_BLOCK_MAIN_STAGE_COUNT; ++boundary) {
    out_evidence->main_stage_gpu_begin_tick[boundary] = block->main_stage_gpu_begin_tick[boundary];
    out_evidence->main_stage_gpu_end_tick[boundary] = block->main_stage_gpu_end_tick[boundary];
    out_evidence->main_stage_gpu_ns[boundary] = block->main_stage_gpu_ns[boundary];
  }
#define PROM_COPY_BLOCK_EVIDENCE_FIELD(name) out_evidence->name = block->name
  PROM_COPY_BLOCK_EVIDENCE_FIELD(last_active_target_validation_ns);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(last_command_reset_ns);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(last_command_begin_ns);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(last_command_record_ns);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(last_command_end_ns);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(last_queue_submit_ns);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(last_fence_wait_ns);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(last_descriptor_update_ns);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(last_staging_memcpy_ns);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(last_output_readback_ns);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(vk_create_buffer_count);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(vk_destroy_buffer_count);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(vk_allocate_memory_count);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(vk_free_memory_count);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(vk_create_shader_module_count);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(vk_destroy_shader_module_count);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(vk_create_compute_pipelines_count);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(vk_allocate_descriptor_sets_count);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(vk_update_descriptor_sets_count);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(vk_create_command_pool_count);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(vk_allocate_command_buffers_count);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(vk_reset_command_buffer_count);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(vk_queue_submit_count);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(vk_fence_wait_count);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(vk_timeline_wait_count);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(vk_map_memory_count);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(vk_unmap_memory_count);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(vk_flush_count);
  PROM_COPY_BLOCK_EVIDENCE_FIELD(vk_invalidate_count);
#undef PROM_COPY_BLOCK_EVIDENCE_FIELD
}

static void prom_model_block_mark_failure(prom_model_block_state* block, int32_t detail) {
  if (block == NULL) return;
  block->output_valid = 0u;
  block->audit_valid = 0u;
  block->last_detail_code = detail;
}

static int prom_model_block_validate_create_request(const PrometheusModelBlockCreateRequest* request,
                                                    int32_t* out_detail) {
  uint64_t total_weight_bytes = 0u;
  uint64_t total_bytes = 0u;
  uint32_t index;
  if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST;
  if (request == NULL || request->struct_size != sizeof(*request) ||
      request->model_contract_identity == 0u || request->weight_identity == 0u ||
      request->shader_portfolio_identity == 0u || request->precision_policy_identity == 0u ||
      request->capability_route_identity == 0u || request->memory_ceiling_bytes == 0u ||
       (request->shader_id != PROM_MODEL_BLOCK_RESIDENT_SHADER_ID &&
        request->shader_id != PROM_MODEL_BLOCK_M1B_ADALN_SHADER_ID &&
        request->shader_id != PROM_MODEL_BLOCK_M1B_NORM_SHADER_ID &&
        request->shader_id != PROM_MODEL_BLOCK_MAIN_QK_ROPE_SHADER_ID) || request->weight_count == 0u ||
      request->weight_count > PROM_MODEL_BLOCK_MAX_WEIGHTS ||
      request->audit_bytes == 0u ||
      (request->external_input_bytes % sizeof(float)) != 0u ||
      (request->external_output_bytes % sizeof(float)) != 0u ||
      (request->audit_bytes % sizeof(float)) != 0u ||
      !prom_model_block_steps_are_exact(request)) return 0;
  if (request->shader_id != PROM_MODEL_BLOCK_MAIN_QK_ROPE_SHADER_ID &&
      request->external_input_bytes == 0u) return 0;
  if (request->shader_id == PROM_MODEL_BLOCK_RESIDENT_SHADER_ID && request->external_output_bytes == 0u) return 0;
  if (request->assembly_family == PROM_NOISE_REFINER_FAMILY_Z_IMAGE_TURBO &&
      ((request->parameter_set != PROM_NOISE_REFINER_PARAMETER_SET_0 &&
        request->parameter_set != PROM_NOISE_REFINER_PARAMETER_SET_1) ||
       request->parameter_set_aggregate_identity != prom_noise_refiner_expected_aggregate(request->parameter_set) ||
       request->shader_id != PROM_MODEL_BLOCK_M1B_ADALN_SHADER_ID)) return 0;
  if (request->assembly_family == PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO &&
      ((request->parameter_set != PROM_CONTEXT_REFINER_PARAMETER_SET_0 &&
        request->parameter_set != PROM_CONTEXT_REFINER_PARAMETER_SET_1) ||
       request->parameter_set_aggregate_identity != prom_context_refiner_expected_aggregate(request->parameter_set) ||
       request->shader_id != PROM_MODEL_BLOCK_M1B_NORM_SHADER_ID)) return 0;
  if (request->assembly_family == PROM_MAIN_TRANSFORMER_FAMILY_Z_IMAGE_TURBO &&
      (request->parameter_set == 0u || request->parameter_set > 30u ||
       request->parameter_set_aggregate_identity != prom_main_transformer_expected_aggregate(request->parameter_set) ||
       request->shader_id != PROM_MODEL_BLOCK_MAIN_QK_ROPE_SHADER_ID)) return 0;
  if (request->assembly_family != 0u &&
      request->assembly_family != PROM_NOISE_REFINER_FAMILY_Z_IMAGE_TURBO &&
      request->assembly_family != PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO &&
      request->assembly_family != PROM_MAIN_TRANSFORMER_FAMILY_Z_IMAGE_TURBO) return 0;
  if (request->assembly_family == 0u &&
      (request->parameter_set != 0u || request->parameter_set_aggregate_identity != 0u)) return 0;
  if (request->shader_id == PROM_MODEL_BLOCK_M1B_ADALN_SHADER_ID &&
      (request->weight_count != PROM_MODEL_BLOCK_MAX_WEIGHTS ||
       request->external_input_bytes != PROM_MODEL_BLOCK_M1B_BF16_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS) ||
       request->external_output_bytes != 0u ||
       request->audit_bytes > PROM_ZIMAGE_TURBO_AUDIT_ARENA_BYTES)) return 0;
  if (request->assembly_family == PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO &&
      (request->weight_count != 11u ||
       request->external_input_bytes != PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_CONTEXT_MODEL_ELEMENTS) ||
       request->external_output_bytes != 0u ||
       request->audit_bytes > PROM_ZIMAGE_TURBO_AUDIT_ARENA_BYTES)) return 0;
  if (request->assembly_family == PROM_MAIN_TRANSFORMER_FAMILY_Z_IMAGE_TURBO &&
      (request->weight_count != PROM_MODEL_BLOCK_MAX_WEIGHTS ||
       request->external_input_bytes != 0u ||
       request->external_output_bytes != 0u ||
       request->audit_bytes > PROM_ZIMAGE_TURBO_AUDIT_ARENA_BYTES)) return 0;
  for (index = 0u; index < request->weight_count; ++index) {
    const PrometheusModelBlockWeightDeclaration* weight = &request->weights[index];
    if (weight->content_identity == 0u || weight->layout_identity == 0u || weight->byte_count == 0u ||
        !prom_model_block_add_bytes(&total_weight_bytes, weight->byte_count)) return 0;
    if (request->shader_id == PROM_MODEL_BLOCK_M1B_ADALN_SHADER_ID &&
        weight->byte_count != k_prom_model_block_m1b_weight_bytes[index]) return 0;
    if (request->assembly_family == PROM_MAIN_TRANSFORMER_FAMILY_Z_IMAGE_TURBO &&
        weight->byte_count != k_prom_model_block_m1b_weight_bytes[index]) return 0;
    if (request->assembly_family == PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO &&
        weight->byte_count != k_prom_model_block_context_weight_bytes[index]) return 0;
  }
  if (request->shader_id == PROM_MODEL_BLOCK_M1B_ADALN_SHADER_ID) {
    /* The 128 MiB reservation is device-local FP32 scratch.  Host staging is
       deliberately outside this VRAM contract and is reused for every cold
       upload; it is never a persistent FP32 weight mirror. */
    total_bytes = total_weight_bytes;
    if (!prom_model_block_add_bytes(&total_bytes, 128ull * 1024ull * 1024ull) ||
        total_bytes > request->memory_ceiling_bytes) return 0;
    return 1;
  }
  if (request->assembly_family == PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO) {
    total_bytes = total_weight_bytes;
    if (!prom_model_block_add_bytes(&total_bytes, 16ull * 1024ull * 1024ull) ||
        total_bytes > request->memory_ceiling_bytes) return 0;
    return 1;
  }
  if (request->assembly_family == PROM_MAIN_TRANSFORMER_FAMILY_Z_IMAGE_TURBO) {
    total_bytes = total_weight_bytes;
    if (!prom_model_block_add_bytes(&total_bytes, 192ull * 1024ull * 1024ull) ||
        total_bytes > request->memory_ceiling_bytes) return 0;
    return 1;
  }
  if (!prom_model_block_add_bytes(&total_bytes, total_weight_bytes) ||
      !prom_model_block_add_bytes(&total_bytes, request->external_input_bytes) ||
      !prom_model_block_add_bytes(&total_bytes, request->external_input_bytes) ||
      !prom_model_block_add_bytes(&total_bytes, request->external_output_bytes) ||
      !prom_model_block_add_bytes(&total_bytes, request->external_output_bytes) ||
      !prom_model_block_add_bytes(&total_bytes, request->audit_bytes) ||
      !prom_model_block_add_bytes(&total_bytes, request->audit_bytes) ||
      !prom_model_block_add_bytes(&total_bytes, total_weight_bytes) ||
      total_bytes > request->memory_ceiling_bytes) return 0;
  return 1;
}

static uint64_t prom_model_block_plan_identity(const prom_model_block_state* block) {
  uint64_t hash = 1469598103934665603ull;
  uint32_t index;
  hash = prom_model_block_hash_u64(hash, block->model_contract_identity);
  hash = prom_model_block_hash_u64(hash, block->assembly_family);
  hash = prom_model_block_hash_u64(hash, block->parameter_set);
  hash = prom_model_block_hash_u64(hash, block->parameter_set_aggregate_identity);
  hash = prom_model_block_hash_u64(hash, block->weight_identity);
  hash = prom_model_block_hash_u64(hash, block->shader_portfolio_identity);
  hash = prom_model_block_hash_u64(hash, block->precision_policy_identity);
  hash = prom_model_block_hash_u64(hash, block->capability_route_identity);
  hash = prom_model_block_hash_u64(hash, block->shader_id);
  hash = prom_model_block_hash_u64(hash, block->external_input_bytes);
  hash = prom_model_block_hash_u64(hash, block->external_output_bytes);
  hash = prom_model_block_hash_u64(hash, block->declared_audit_bytes);
  /* The fixed M1C third binding is topology, not incidental scratch. */
  hash = prom_model_block_hash_u64(hash, PROM_MODEL_BLOCK_M1C_TRANSIENT_AUDIT_FLOATS * sizeof(float));
  hash = prom_model_block_hash_u64(hash, 3u);
  for (index = 0u; index < block->weight_count; ++index) {
    hash = prom_model_block_hash_u64(hash, block->weights[index].content_identity);
    hash = prom_model_block_hash_u64(hash, block->weights[index].layout_identity);
    hash = prom_model_block_hash_u64(hash, block->weights[index].byte_count);
  }
  for (index = 0u; index < block->step_count; ++index) {
    hash = prom_model_block_hash_u64(hash, block->steps[index]);
  }
  if (block->shader_id == PROM_MODEL_BLOCK_M1B_ADALN_SHADER_ID ||
      prom_model_block_is_context_refiner(block) ||
      prom_model_block_is_main_transformer(block)) {
    for (index = 0u; index < PROM_MODEL_BLOCK_M1B_PIPELINE_COUNT; ++index) {
      hash = prom_model_block_hash_u64(hash, block->m1b_pipelines[index].shader_id);
      hash = prom_model_block_hash_u64(hash, block->m1b_pipelines[index].binding_count);
      hash = prom_model_block_hash_u64(hash, block->m1b_pipelines[index].push_constant_bytes);
    }
    for (index = 0u; index < PROM_MODEL_BLOCK_M1C_PIPELINE_COUNT; ++index) {
      hash = prom_model_block_hash_u64(hash, block->m1c_pipelines[index].shader_id);
      hash = prom_model_block_hash_u64(hash, block->m1c_pipelines[index].binding_count);
      hash = prom_model_block_hash_u64(hash, block->m1c_pipelines[index].push_constant_bytes);
    }
    for (index = 0u; index < PROM_MODEL_BLOCK_M1D_PIPELINE_COUNT; ++index) {
      hash = prom_model_block_hash_u64(hash, block->m1d_pipelines[index].shader_id);
      hash = prom_model_block_hash_u64(hash, block->m1d_pipelines[index].binding_count);
      hash = prom_model_block_hash_u64(hash, block->m1d_pipelines[index].push_constant_bytes);
    }
  }
  return hash;
}

static int prom_model_block_create_buffer(prom_reduction_runtime_state* state,
                                          prom_model_block_state* block,
                                          prom_vk_buffer* buffer,
                                          VkDeviceSize size,
                                          VkBufferUsageFlags usage,
                                          VkMemoryPropertyFlags properties,
                                          int map_memory) {
  VkResult result;
  if (state == NULL || block == NULL || buffer == NULL || size == 0u) return 0;
  result = prom_vk_create_buffer(state->physical_device, state->device, block->test_flags, size,
                                 usage, properties, map_memory, buffer);
  if (result != VK_SUCCESS) return 0;
  block->cold_buffer_allocation_count += 1u;
  block->vk_create_buffer_count += 1u;
  block->vk_allocate_memory_count += 1u;
  if (map_memory != 0) block->vk_map_memory_count += 1u;
  return 1;
}

static int prom_model_block_create_descriptor_resources(prom_reduction_runtime_state* state,
                                                        prom_model_block_state* block) {
  VkDescriptorSetLayoutBinding bindings[3];
  VkDescriptorSetLayoutCreateInfo layout_info;
  VkDescriptorPoolSize pool_size;
  VkDescriptorPoolCreateInfo pool_info;
  VkDescriptorSetAllocateInfo set_info;
  VkPushConstantRange push_range;
  VkPipelineLayoutCreateInfo pipeline_layout_info;
  VkDescriptorBufferInfo buffer_infos[3];
  VkWriteDescriptorSet writes[3];
  uint32_t index;
  if (state == NULL || block == NULL) return 0;
  memset(bindings, 0, sizeof(bindings));
  for (index = 0u; index < 3u; ++index) {
    bindings[index].binding = index;
    bindings[index].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
    bindings[index].descriptorCount = 1u;
    bindings[index].stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
  }
  memset(&layout_info, 0, sizeof(layout_info));
  layout_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO;
  layout_info.bindingCount = 3u;
  layout_info.pBindings = bindings;
  if (vkCreateDescriptorSetLayout(state->device, &layout_info, NULL, &block->descriptor_set_layout) != VK_SUCCESS) return 0;
  memset(&pool_size, 0, sizeof(pool_size));
  pool_size.type = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  pool_size.descriptorCount = 3u;
  memset(&pool_info, 0, sizeof(pool_info));
  pool_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO;
  pool_info.maxSets = 1u;
  pool_info.poolSizeCount = 1u;
  pool_info.pPoolSizes = &pool_size;
  if (vkCreateDescriptorPool(state->device, &pool_info, NULL, &block->descriptor_pool) != VK_SUCCESS) return 0;
  memset(&set_info, 0, sizeof(set_info));
  set_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO;
  set_info.descriptorPool = block->descriptor_pool;
  set_info.descriptorSetCount = 1u;
  set_info.pSetLayouts = &block->descriptor_set_layout;
  if (vkAllocateDescriptorSets(state->device, &set_info, &block->descriptor_set) != VK_SUCCESS) return 0;
  block->descriptor_set_count = 1u;
  block->vk_allocate_descriptor_sets_count += 1u;
  memset(&push_range, 0, sizeof(push_range));
  push_range.stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
  push_range.size = sizeof(prom_model_block_push_constants);
  memset(&pipeline_layout_info, 0, sizeof(pipeline_layout_info));
  pipeline_layout_info.sType = VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO;
  pipeline_layout_info.setLayoutCount = 1u;
  pipeline_layout_info.pSetLayouts = &block->descriptor_set_layout;
  pipeline_layout_info.pushConstantRangeCount = 1u;
  pipeline_layout_info.pPushConstantRanges = &push_range;
  if (vkCreatePipelineLayout(state->device, &pipeline_layout_info, NULL, &block->pipeline_layout) != VK_SUCCESS) return 0;
  memset(buffer_infos, 0, sizeof(buffer_infos));
  buffer_infos[0].buffer = block->input_device.buffer;
  buffer_infos[0].range = block->input_device.size;
  buffer_infos[1].buffer = block->output_device.buffer;
  buffer_infos[1].range = block->output_device.size;
  buffer_infos[2].buffer = block->audit_device.buffer;
  buffer_infos[2].range = block->audit_device.size;
  memset(writes, 0, sizeof(writes));
  for (index = 0u; index < 3u; ++index) {
    writes[index].sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET;
    writes[index].dstSet = block->descriptor_set;
    writes[index].dstBinding = index;
    writes[index].descriptorCount = 1u;
    writes[index].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
    writes[index].pBufferInfo = &buffer_infos[index];
  }
  vkUpdateDescriptorSets(state->device, 3u, writes, 0u, NULL);
  block->vk_update_descriptor_sets_count += 1u;
  return 1;
}

static int prom_model_block_create_pipeline(prom_reduction_runtime_state* state,
                                            prom_model_block_state* block) {
  const prom_shader_asset* asset;
  VkShaderModuleCreateInfo module_info;
  VkPipelineShaderStageCreateInfo stage_info;
  VkComputePipelineCreateInfo pipeline_info;
  if (state == NULL || block == NULL) return 0;
  asset = prom_shader_registry_find_shader(block->shader_id);
  if (asset == NULL || asset->stage != PROM_SHADER_STAGE_COMPUTE ||
      asset->authority != PROM_SHADER_AUTHORITY_PRODUCTION || asset->source_language != PROM_SHADER_SOURCE_SDSLV ||
      asset->descriptor_binding_count != 3u || asset->push_constant_bytes != sizeof(prom_model_block_push_constants) ||
      asset->spirv_words == NULL || asset->spirv_size_bytes == 0u || asset->entry_point == NULL ||
      (block->test_flags & PROM_TESTCFG_FAIL_PIPELINE_CREATE) != 0u) return 0;
  memset(&module_info, 0, sizeof(module_info));
  module_info.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
  module_info.codeSize = asset->spirv_size_bytes;
  module_info.pCode = asset->spirv_words;
  if (vkCreateShaderModule(state->device, &module_info, NULL, &block->pipeline.shader_module) != VK_SUCCESS) return 0;
  block->vk_create_shader_module_count += 1u;
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = block->pipeline.shader_module;
  stage_info.pName = asset->entry_point;
  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = block->pipeline_layout;
  if (vkCreateComputePipelines(state->device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL,
                               &block->pipeline.pipeline) != VK_SUCCESS) return 0;
  block->vk_create_compute_pipelines_count += 1u;
  block->pipeline.shader_id = asset->shader_id;
  block->pipeline_create_count = 1u;
  return 1;
}

static int prom_model_block_m1b_shader_asset_is_admitted(const prom_shader_asset* asset, uint32_t shader_id) {
  if (asset == NULL || asset->stage != PROM_SHADER_STAGE_COMPUTE) return 0;
  if (asset->authority == PROM_SHADER_AUTHORITY_PRODUCTION && asset->source_language == PROM_SHADER_SOURCE_SDSLV) {
    return 1;
  }
#if defined(PROMETHEUS_DVT2_M5B_SUBGROUP_OWNED_EXPERIMENT)
  if (shader_id == 44u && asset->authority == PROM_SHADER_AUTHORITY_EXPERIMENTAL &&
      asset->source_language == PROM_SHADER_SOURCE_SDSLV) return 1;
#endif
#if defined(PROMETHEUS_DVT2_M5B_GEMINI_EXACT_EXPERIMENT)
  if (shader_id == 45u && asset->authority == PROM_SHADER_AUTHORITY_EXPERIMENTAL &&
      asset->source_language == PROM_SHADER_SOURCE_HLSL) return 1;
#endif
#if defined(PROMETHEUS_DVT2_M5B_GEMINI_INPLACE_EXPERIMENT)
  if (shader_id == 46u && asset->authority == PROM_SHADER_AUTHORITY_EXPERIMENTAL &&
      asset->source_language == PROM_SHADER_SOURCE_HLSL) return 1;
#endif
  return 0;
}

static int prom_model_block_m1b_create_pipeline(
    prom_reduction_runtime_state* state, prom_model_block_state* block,
    prom_model_block_m1b_pipeline* pipeline, uint32_t shader_id,
    const prom_vk_buffer* const* buffers, uint32_t buffer_count, uint32_t expected_push_constant_bytes) {
  const prom_shader_asset* asset;
  VkDescriptorSetLayoutBinding bindings[8];
  VkDescriptorSetLayoutCreateInfo layout_info;
  VkDescriptorPoolSize pool_size;
  VkDescriptorPoolCreateInfo pool_info;
  VkDescriptorSetAllocateInfo set_info;
  VkPushConstantRange push_range;
  VkPipelineLayoutCreateInfo pipeline_layout_info;
  VkDescriptorBufferInfo buffer_infos[8];
  VkWriteDescriptorSet writes[8];
  VkShaderModuleCreateInfo module_info;
  VkPipelineShaderStageCreateInfo stage_info;
  VkComputePipelineCreateInfo compute_info;
  VkResult create_result;
  int diagnostics_enabled;
  uint32_t index;
  if (state == NULL || block == NULL || pipeline == NULL || buffers == NULL ||
      buffer_count == 0u || buffer_count > 8u) return 0;
  diagnostics_enabled = getenv("OCT_EVT2_M1C_PIPELINE_DIAGNOSTICS") != NULL;
  asset = prom_shader_registry_find_shader(shader_id);
  if (diagnostics_enabled && (!prom_model_block_m1b_shader_asset_is_admitted(asset, shader_id) ||
      asset->descriptor_binding_count != buffer_count || asset->push_constant_bytes != expected_push_constant_bytes ||
      asset->spirv_words == NULL || asset->spirv_size_bytes == 0u || asset->entry_point == NULL)) {
    fprintf(stderr, "EVT2_M1C_CREATE_PRECHECK shader=%u asset=%p bindings=%u expected_bindings=%u push=%u expected_push=%u stage=%u authority=%u source=%u spirv=%zu entry=%p\n",
            shader_id, (const void*)asset, asset == NULL ? 0u : asset->descriptor_binding_count, buffer_count,
            asset == NULL ? 0u : asset->push_constant_bytes, expected_push_constant_bytes,
            asset == NULL ? 0u : (uint32_t)asset->stage, asset == NULL ? 0u : (uint32_t)asset->authority,
            asset == NULL ? 0u : (uint32_t)asset->source_language, asset == NULL ? 0u : asset->spirv_size_bytes,
            asset == NULL ? NULL : (const void*)asset->entry_point);
  }
  if (!prom_model_block_m1b_shader_asset_is_admitted(asset, shader_id) ||
      asset->descriptor_binding_count != buffer_count || asset->push_constant_bytes != expected_push_constant_bytes ||
      asset->spirv_words == NULL || asset->spirv_size_bytes == 0u || asset->entry_point == NULL ||
      (block->test_flags & PROM_TESTCFG_FAIL_PIPELINE_CREATE) != 0u) return 0;
  memset(bindings, 0, sizeof(bindings));
  for (index = 0u; index < buffer_count; ++index) {
    if (buffers[index] == NULL || buffers[index]->buffer == VK_NULL_HANDLE || buffers[index]->size == 0u) return 0;
    bindings[index].binding = index;
    bindings[index].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
    bindings[index].descriptorCount = 1u;
    bindings[index].stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
  }
  memset(&layout_info, 0, sizeof(layout_info));
  layout_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO;
  layout_info.bindingCount = buffer_count;
  layout_info.pBindings = bindings;
  create_result = vkCreateDescriptorSetLayout(state->device, &layout_info, NULL, &pipeline->descriptor_set_layout);
  if (diagnostics_enabled) fprintf(stderr, "EVT2_M1C_CREATE shader=%u bindings=%u descriptor_set_layout=%d\n", shader_id, buffer_count, create_result);
  if (create_result != VK_SUCCESS) return 0;
  memset(&pool_size, 0, sizeof(pool_size));
  pool_size.type = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  pool_size.descriptorCount = buffer_count;
  memset(&pool_info, 0, sizeof(pool_info));
  pool_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO;
  pool_info.maxSets = 1u;
  pool_info.poolSizeCount = 1u;
  pool_info.pPoolSizes = &pool_size;
  if (vkCreateDescriptorPool(state->device, &pool_info, NULL, &pipeline->descriptor_pool) != VK_SUCCESS) return 0;
  memset(&set_info, 0, sizeof(set_info));
  set_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO;
  set_info.descriptorPool = pipeline->descriptor_pool;
  set_info.descriptorSetCount = 1u;
  set_info.pSetLayouts = &pipeline->descriptor_set_layout;
  if (vkAllocateDescriptorSets(state->device, &set_info, &pipeline->descriptor_set) != VK_SUCCESS) return 0;
  block->vk_allocate_descriptor_sets_count += 1u;
  memset(&push_range, 0, sizeof(push_range));
  push_range.stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
  push_range.size = expected_push_constant_bytes;
  memset(&pipeline_layout_info, 0, sizeof(pipeline_layout_info));
  pipeline_layout_info.sType = VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO;
  pipeline_layout_info.setLayoutCount = 1u;
  pipeline_layout_info.pSetLayouts = &pipeline->descriptor_set_layout;
  pipeline_layout_info.pushConstantRangeCount = 1u;
  pipeline_layout_info.pPushConstantRanges = &push_range;
  create_result = vkCreatePipelineLayout(state->device, &pipeline_layout_info, NULL, &pipeline->pipeline_layout);
  if (diagnostics_enabled) fprintf(stderr, "EVT2_M1C_CREATE shader=%u pipeline_layout=%d push_constants=%u\n", shader_id, create_result, expected_push_constant_bytes);
  if (create_result != VK_SUCCESS) return 0;
  memset(buffer_infos, 0, sizeof(buffer_infos));
  memset(writes, 0, sizeof(writes));
  for (index = 0u; index < buffer_count; ++index) {
    buffer_infos[index].buffer = buffers[index]->buffer;
    buffer_infos[index].range = buffers[index]->size;
    writes[index].sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET;
    writes[index].dstSet = pipeline->descriptor_set;
    writes[index].dstBinding = index;
    writes[index].descriptorCount = 1u;
    writes[index].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
    writes[index].pBufferInfo = &buffer_infos[index];
  }
  vkUpdateDescriptorSets(state->device, buffer_count, writes, 0u, NULL);
  block->vk_update_descriptor_sets_count += 1u;
  memset(&module_info, 0, sizeof(module_info));
  module_info.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
  module_info.codeSize = asset->spirv_size_bytes;
  module_info.pCode = asset->spirv_words;
  create_result = vkCreateShaderModule(state->device, &module_info, NULL, &pipeline->pipeline.shader_module);
  if (diagnostics_enabled) fprintf(stderr, "EVT2_M1C_CREATE shader=%u shader_module=%d spirv_bytes=%zu\n", shader_id, create_result, asset->spirv_size_bytes);
  if (create_result != VK_SUCCESS) return 0;
  block->vk_create_shader_module_count += 1u;
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = pipeline->pipeline.shader_module;
  stage_info.pName = asset->entry_point;
  memset(&compute_info, 0, sizeof(compute_info));
  compute_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  compute_info.stage = stage_info;
  compute_info.layout = pipeline->pipeline_layout;
  create_result = vkCreateComputePipelines(state->device, VK_NULL_HANDLE, 1u, &compute_info, NULL,
                                            &pipeline->pipeline.pipeline);
  if (diagnostics_enabled) fprintf(stderr, "EVT2_M1C_CREATE shader=%u compute_pipeline=%d entry=%s\n", shader_id, create_result, asset->entry_point);
  if (create_result != VK_SUCCESS) return 0;
  block->vk_create_compute_pipelines_count += 1u;
  pipeline->shader_id = asset->shader_id;
  pipeline->binding_count = buffer_count;
  pipeline->push_constant_bytes = expected_push_constant_bytes;
  block->pipeline_create_count += 1u;
  block->descriptor_set_count += 1u;
  return 1;
}

static int prom_model_block_m1b_create_pipelines(prom_reduction_runtime_state* state,
                                                  prom_model_block_state* block) {
  const prom_vk_buffer* ingress_buffers[] = {&block->input_bf16_device,
                                              &block->timestep_bf16_device,
                                              &block->input_device,
                                              &block->timestep_device};
  const prom_vk_buffer* adaln_buffers[] = {&block->timestep_device, &block->weights[1u].device,
                                           &block->weights[0u].device, &block->adaln_projection,
                                           &block->attention_scale, &block->attention_gate,
                                           &block->mlp_scale, &block->mlp_gate};
  const prom_vk_buffer* norm_buffers[] = {&block->input_device, &block->weights[6u].device,
                                          &block->attention_scale, &block->modulated,
                                          &block->norm_audit};
  const prom_vk_buffer* qkv_buffers[] = {&block->modulated, &block->weights[5u].device, &block->qkv};
  const prom_vk_buffer* q_buffers[] = {&block->qkv, &block->weights[4u].device,
                                       &block->norm_audit};
  const prom_vk_buffer* k_buffers[] = {&block->qkv, &block->weights[2u].device,
                                       &block->norm_audit};
  return prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[0u], k_prom_model_block_m1b_shader_ids[0u], ingress_buffers, 4u,
                                               sizeof(prom_model_block_m1b_ingress_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[1u], k_prom_model_block_m1b_shader_ids[1u], adaln_buffers, 8u,
                                               sizeof(prom_model_block_m1b_adaln_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[2u], k_prom_model_block_m1b_shader_ids[2u], norm_buffers, 5u,
                                               sizeof(prom_model_block_m1b_norm_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[3u], k_prom_model_block_m1b_shader_ids[3u], qkv_buffers, 3u,
                                               sizeof(prom_model_block_m1b_qkv_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[4u], k_prom_model_block_m1b_shader_ids[4u], q_buffers, 3u,
                                               sizeof(prom_model_block_m1b_head_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[5u], k_prom_model_block_m1b_shader_ids[5u], k_buffers, 3u,
                                               sizeof(prom_model_block_m1b_head_constants));
}

static int prom_model_block_m1c_create_pipelines(prom_reduction_runtime_state* state,
                                                  prom_model_block_state* block) {
  const prom_vk_buffer* attention_buffers[] = {&block->qkv, &block->attention, &block->audit_device};
  const prom_vk_buffer* projection_buffers[] = {&block->attention, &block->weights[3u].device,
                                                 &block->attention_projection};
  const prom_vk_buffer* residual_buffers[] = {&block->attention_projection, &block->weights[7u].device,
                                               &block->attention_gate, &block->input_device,
                                               &block->attention_residual};
  return prom_model_block_m1b_create_pipeline(
             state, block, &block->m1c_pipelines[0u], PROM_MODEL_BLOCK_M1C_ATTENTION_SHADER_ID,
             attention_buffers, 3u, sizeof(prom_model_block_m1c_attention_constants)) &&
         prom_model_block_m1b_create_pipeline(
             state, block, &block->m1c_pipelines[1u], PROM_MODEL_BLOCK_M1C_PROJECTION_SHADER_ID,
             projection_buffers, 3u, sizeof(prom_model_block_m1b_qkv_constants)) &&
         prom_model_block_m1b_create_pipeline(
             state, block, &block->m1c_pipelines[2u], PROM_MODEL_BLOCK_M1C_RESIDUAL_SHADER_ID,
             residual_buffers, 5u, sizeof(prom_model_block_m1b_norm_constants));
}

static int prom_model_block_m1d_create_pipelines(prom_reduction_runtime_state* state,
                                                  prom_model_block_state* block) {
  const prom_vk_buffer* norm_buffers[] = {&block->attention_residual, &block->weights[11u].device,
                                          &block->mlp_scale, &block->norm_audit, &block->modulated};
  const prom_vk_buffer* projection_buffers[] = {&block->modulated, &block->weights[8u].device,
                                                &block->weights[10u].device, &block->attention,
                                                &block->attention_projection, &block->norm_audit, &block->qkv};
  const prom_vk_buffer* gate_buffers[] = {&block->qkv, &block->attention,
                                          &block->attention_projection, &block->norm_audit};
  const prom_vk_buffer* w2_buffers[] = {&block->qkv, &block->weights[9u].device,
                                        &block->weights[12u].device, &block->mlp_gate,
                                        &block->attention_residual, &block->input_device, &block->attention};
  return prom_model_block_m1b_create_pipeline(
             state, block, &block->m1d_pipelines[0u], PROM_MODEL_BLOCK_M1D_NORM_SHADER_ID,
             norm_buffers, 5u, sizeof(prom_model_block_m1b_norm_constants)) &&
         prom_model_block_m1b_create_pipeline(
             state, block, &block->m1d_pipelines[1u], PROM_MODEL_BLOCK_M1D_W1_W3_SHADER_ID,
             projection_buffers, 7u, sizeof(prom_model_block_m1b_qkv_constants)) &&
         prom_model_block_m1b_create_pipeline(
             state, block, &block->m1d_pipelines[2u], PROM_MODEL_BLOCK_M1D_GATE_SHADER_ID,
             gate_buffers, 4u, sizeof(prom_model_block_m1d_gate_constants)) &&
         prom_model_block_m1b_create_pipeline(
             state, block, &block->m1d_pipelines[3u], PROM_MODEL_BLOCK_M1D_W2_RESIDUAL_SHADER_ID,
             w2_buffers, 7u, sizeof(prom_model_block_m1d_w2_constants));
}

/* ContextRefiner shares the physical FP32 norm/projection/residual kernels,
   but owns the text-coordinate Q/K RoPE and fixed 32-token attention program.
   Its unit scale buffer makes the unmodulated residual contract explicit. */
static int prom_context_refiner_create_pipelines(prom_reduction_runtime_state* state,
                                                 prom_model_block_state* block) {
  const prom_vk_buffer* norm_buffers[] = {&block->input_device, &block->weights[4u].device,
                                          &block->context_unit, &block->modulated, &block->norm_audit};
  const prom_vk_buffer* qkv_buffers[] = {&block->modulated, &block->weights[3u].device, &block->qkv};
  const prom_vk_buffer* q_buffers[] = {&block->qkv, &block->weights[2u].device, &block->norm_audit};
  const prom_vk_buffer* k_buffers[] = {&block->qkv, &block->weights[0u].device, &block->norm_audit};
  const prom_vk_buffer* attention_buffers[] = {&block->qkv, &block->attention, &block->audit_device};
  const prom_vk_buffer* projection_buffers[] = {&block->attention, &block->weights[1u].device,
                                                 &block->attention_projection};
  const prom_vk_buffer* residual_buffers[] = {&block->attention_projection, &block->weights[5u].device,
                                               &block->context_unit, &block->input_device,
                                               &block->attention_residual};
  const prom_vk_buffer* ffn_norm_buffers[] = {&block->attention_residual, &block->weights[9u].device,
                                              &block->context_unit, &block->norm_audit, &block->modulated};
  const prom_vk_buffer* ffn_projection_buffers[] = {&block->modulated, &block->weights[6u].device,
                                                     &block->weights[8u].device, &block->context_w3,
                                                     &block->attention_projection, &block->norm_audit,
                                                     &block->qkv};
  const prom_vk_buffer* gate_buffers[] = {&block->qkv, &block->context_w3,
                                          &block->attention_projection, &block->norm_audit};
  const prom_vk_buffer* w2_buffers[] = {&block->qkv, &block->weights[7u].device,
                                        &block->weights[10u].device, &block->context_unit,
                                        &block->attention_residual, &block->attention_projection,
                                        &block->attention};
  return prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[0u],
                                               PROM_MODEL_BLOCK_M1B_NORM_SHADER_ID, norm_buffers, 5u,
                                               sizeof(prom_model_block_m1b_norm_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[1u],
                                               PROM_MODEL_BLOCK_M1B_QKV_SHADER_ID, qkv_buffers, 3u,
                                               sizeof(prom_model_block_m1b_qkv_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[2u],
                                               PROM_MODEL_BLOCK_CONTEXT_QK_ROPE_SHADER_ID, q_buffers, 3u,
                                               sizeof(prom_model_block_context_qk_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[3u],
                                               PROM_MODEL_BLOCK_CONTEXT_QK_ROPE_SHADER_ID, k_buffers, 3u,
                                               sizeof(prom_model_block_context_qk_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1c_pipelines[0u],
                                               PROM_MODEL_BLOCK_CONTEXT_ATTENTION_SHADER_ID, attention_buffers, 3u,
                                               sizeof(prom_model_block_m1c_attention_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1c_pipelines[1u],
                                               PROM_MODEL_BLOCK_M1C_PROJECTION_SHADER_ID, projection_buffers, 3u,
                                               sizeof(prom_model_block_m1b_qkv_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1c_pipelines[2u],
                                               PROM_MODEL_BLOCK_M1C_RESIDUAL_SHADER_ID, residual_buffers, 5u,
                                               sizeof(prom_model_block_m1b_norm_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1d_pipelines[0u],
                                               PROM_MODEL_BLOCK_M1D_NORM_SHADER_ID, ffn_norm_buffers, 5u,
                                               sizeof(prom_model_block_m1b_norm_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1d_pipelines[1u],
                                               PROM_MODEL_BLOCK_M1D_W1_W3_SHADER_ID, ffn_projection_buffers, 7u,
                                               sizeof(prom_model_block_m1b_qkv_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1d_pipelines[2u],
                                               PROM_MODEL_BLOCK_M1D_GATE_SHADER_ID, gate_buffers, 4u,
                                               sizeof(prom_model_block_m1d_gate_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1d_pipelines[3u],
                                               PROM_MODEL_BLOCK_M1D_W2_RESIDUAL_SHADER_ID, w2_buffers, 7u,
                                               sizeof(prom_model_block_m1d_w2_constants));
}

static int prom_main_transformer_create_pipelines(prom_reduction_runtime_state* state,
                                                  prom_model_block_state* block) {
  const prom_vk_buffer* ingress_buffers[] = {&block->input_bf16_device,
                                              &block->timestep_bf16_device,
                                              &block->input_device,
                                              &block->timestep_device};
  const prom_vk_buffer* adaln_buffers[] = {&block->timestep_device, &block->weights[1u].device,
                                           &block->weights[0u].device, &block->adaln_projection,
                                           &block->attention_scale, &block->attention_gate,
                                           &block->mlp_scale, &block->mlp_gate};
  const prom_vk_buffer* norm_buffers[] = {&block->input_device, &block->weights[6u].device,
                                          &block->attention_scale, &block->modulated,
                                          &block->norm_audit};
  const prom_vk_buffer* qkv_buffers[] = {&block->modulated, &block->weights[5u].device, &block->qkv};
  const prom_vk_buffer* q_buffers[] = {&block->qkv, &block->weights[4u].device,
                                       &block->norm_audit};
  const prom_vk_buffer* k_buffers[] = {&block->qkv, &block->weights[2u].device,
                                       &block->norm_audit};
  const prom_vk_buffer* attention_buffers[] = {&block->qkv, &block->attention, &block->audit_device};
  const prom_vk_buffer* projection_buffers[] = {&block->attention, &block->weights[3u].device,
                                                 &block->attention_projection};
  const prom_vk_buffer* residual_buffers[] = {&block->attention_projection, &block->weights[7u].device,
                                               &block->attention_gate, &block->input_device,
                                               &block->attention_residual};
  const prom_vk_buffer* ffn_norm_buffers[] = {&block->attention_residual, &block->weights[11u].device,
                                              &block->mlp_scale, &block->norm_audit, &block->modulated};
  const prom_vk_buffer* ffn_projection_buffers[] = {&block->modulated, &block->weights[8u].device,
                                                     &block->weights[10u].device, &block->attention,
                                                     &block->attention_projection, &block->norm_audit,
                                                     &block->qkv};
  const prom_vk_buffer* gate_buffers[] = {&block->qkv, &block->attention,
                                          &block->attention_projection, &block->norm_audit};
  const prom_vk_buffer* w2_buffers[] = {&block->qkv, &block->weights[9u].device,
                                        &block->weights[12u].device, &block->mlp_gate,
                                        &block->attention_residual, &block->input_device,
                                        &block->attention};
  const uint32_t attention_shader_id = state->compiled_session.main_attention_shader_id;
  if (attention_shader_id != PROM_MODEL_BLOCK_MAIN_ATTENTION_SERIAL_SHADER_ID &&
      attention_shader_id != PROM_MODEL_BLOCK_MAIN_ATTENTION_SUBGROUP_OWNED32_SHADER_ID) return 0;
  block->main_attention_shader_id = attention_shader_id;
  return prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[0u],
                                               PROM_MODEL_BLOCK_M1B_INGRESS_SHADER_ID, ingress_buffers, 4u,
                                               sizeof(prom_model_block_m1b_ingress_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[1u],
                                               PROM_MODEL_BLOCK_M1B_ADALN_SHADER_ID, adaln_buffers, 8u,
                                               sizeof(prom_model_block_m1b_adaln_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[2u],
                                               PROM_MODEL_BLOCK_M1B_NORM_SHADER_ID, norm_buffers, 5u,
                                               sizeof(prom_model_block_m1b_norm_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[3u],
                                               PROM_MODEL_BLOCK_M1B_QKV_SHADER_ID, qkv_buffers, 3u,
                                               sizeof(prom_model_block_m1b_qkv_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[4u],
                                               PROM_MODEL_BLOCK_MAIN_QK_ROPE_SHADER_ID, q_buffers, 3u,
                                               sizeof(prom_model_block_main_qk_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[5u],
                                               PROM_MODEL_BLOCK_MAIN_QK_ROPE_SHADER_ID, k_buffers, 3u,
                                               sizeof(prom_model_block_main_qk_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1c_pipelines[0u],
                                               attention_shader_id, attention_buffers, 3u,
                                               sizeof(prom_model_block_m1c_attention_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1c_pipelines[1u],
                                               PROM_MODEL_BLOCK_M1C_PROJECTION_SHADER_ID, projection_buffers, 3u,
                                               sizeof(prom_model_block_m1b_qkv_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1c_pipelines[2u],
                                               PROM_MODEL_BLOCK_M1C_RESIDUAL_SHADER_ID, residual_buffers, 5u,
                                               sizeof(prom_model_block_m1b_norm_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1d_pipelines[0u],
                                               PROM_MODEL_BLOCK_M1D_NORM_SHADER_ID, ffn_norm_buffers, 5u,
                                               sizeof(prom_model_block_m1b_norm_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1d_pipelines[1u],
                                               PROM_MODEL_BLOCK_MAIN_W1_W3_SHADER_ID, ffn_projection_buffers, 7u,
                                               sizeof(prom_model_block_m1b_qkv_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1d_pipelines[2u],
                                               PROM_MODEL_BLOCK_MAIN_GATE_SHADER_ID, gate_buffers, 4u,
                                               sizeof(prom_model_block_m1d_gate_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1d_pipelines[3u],
                                               PROM_MODEL_BLOCK_M1D_W2_RESIDUAL_SHADER_ID, w2_buffers, 7u,
                                               sizeof(prom_model_block_m1d_w2_constants));
}

static uint64_t prom_context_refiner_expected_aggregate(uint32_t parameter_set) {
  uint32_t index;
  for (index = 0u; index < 2u; ++index) {
    if (k_prom_zimage_turbo_context_refiner_blocks[index].parameter_set == parameter_set)
      return k_prom_zimage_turbo_context_refiner_blocks[index].parameter_set_aggregate_identity;
  }
  return 0u;
}

static uint64_t prom_main_transformer_expected_aggregate(uint32_t parameter_set) {
  uint32_t index;
  for (index = 0u; index < 30u; ++index) {
    if (k_prom_zimage_turbo_main_transformer_blocks[index].parameter_set == parameter_set)
      return k_prom_zimage_turbo_main_transformer_blocks[index].parameter_set_aggregate_identity;
  }
  return 0u;
}

static int prom_model_block_is_context_refiner(const prom_model_block_state* block) {
  return block != NULL && block->assembly_family == PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO;
}

static int prom_model_block_is_main_transformer(const prom_model_block_state* block) {
  return block != NULL && block->assembly_family == PROM_MAIN_TRANSFORMER_FAMILY_Z_IMAGE_TURBO;
}

static const prom_vk_buffer* prom_model_block_audit_source(
    const prom_model_block_state* block, uint32_t source_resource) {
  if (block == NULL) return NULL;
  if (prom_model_block_is_context_refiner(block) &&
      (source_resource == PROM_ZIMAGE_AUDIT_SOURCE_ADALN ||
       source_resource == PROM_ZIMAGE_AUDIT_SOURCE_ATTENTION_SCALE ||
       source_resource == PROM_ZIMAGE_AUDIT_SOURCE_ATTENTION_GATE ||
       source_resource == PROM_ZIMAGE_AUDIT_SOURCE_MLP_SCALE ||
       source_resource == PROM_ZIMAGE_AUDIT_SOURCE_MLP_GATE)) return &block->context_unit;
  if (prom_model_block_is_context_refiner(block) &&
      source_resource == PROM_ZIMAGE_AUDIT_SOURCE_W3_DECLARED_VIEWS) return &block->context_w3;
  switch (source_resource) {
    case PROM_ZIMAGE_AUDIT_SOURCE_ADALN: return &block->adaln_projection;
    case PROM_ZIMAGE_AUDIT_SOURCE_ATTENTION_SCALE: return &block->attention_scale;
    case PROM_ZIMAGE_AUDIT_SOURCE_ATTENTION_GATE: return &block->attention_gate;
    case PROM_ZIMAGE_AUDIT_SOURCE_MLP_SCALE: return &block->mlp_scale;
    case PROM_ZIMAGE_AUDIT_SOURCE_MLP_GATE: return &block->mlp_gate;
    case PROM_ZIMAGE_AUDIT_SOURCE_NORM: return &block->norm_audit;
    case PROM_ZIMAGE_AUDIT_SOURCE_MODULATED: return &block->modulated;
    case PROM_ZIMAGE_AUDIT_SOURCE_QKV: return &block->qkv;
    case PROM_ZIMAGE_AUDIT_SOURCE_ATTENTION: return &block->attention;
    case PROM_ZIMAGE_AUDIT_SOURCE_ATTENTION_PROJECTION: return &block->attention_projection;
    case PROM_ZIMAGE_AUDIT_SOURCE_ATTENTION_RESIDUAL: return &block->attention_residual;
    case PROM_ZIMAGE_AUDIT_SOURCE_INPUT: return &block->input_device;
    case PROM_ZIMAGE_AUDIT_SOURCE_W3_DECLARED_VIEWS: return &block->attention;
    default: return NULL;
  }
}

static int prom_model_block_audit_create_pipelines(prom_reduction_runtime_state* state,
                                                    prom_model_block_state* block) {
  uint32_t source_resource;
  for (source_resource = 1u; source_resource <= PROM_MODEL_BLOCK_AUDIT_SOURCE_COUNT; ++source_resource) {
    const prom_vk_buffer* source0 = prom_model_block_audit_source(block, source_resource);
    const prom_vk_buffer* source1 = source0;
    const prom_vk_buffer* source2 = source0;
    if (source_resource == PROM_ZIMAGE_AUDIT_SOURCE_W3_DECLARED_VIEWS) {
      source1 = &block->attention_projection;
      source2 = &block->norm_audit;
    }
    const prom_vk_buffer* buffers[] = {source0, source1, source2, &block->audit_readback};
    if (!prom_model_block_m1b_create_pipeline(
            state, block, &block->audit_pipelines[source_resource - 1u],
            PROM_MODEL_BLOCK_AUDIT_SUMMARY_SHADER_ID, buffers, 4u,
            sizeof(prom_model_block_audit_constants))) return 0;
  }
  return 1;
}

/* Descriptor writes are prepared in one bounded batch after every candidate
   weight upload has completed.  No dispatch is in flight at this point, and
   the active descriptors remain untouched until this call. */
static int prom_model_block_update_weight_descriptors(
    prom_reduction_runtime_state* state, prom_model_block_state* block,
    const prom_model_block_weight_resource weights[PROM_MODEL_BLOCK_MAX_WEIGHTS]) {
  VkDescriptorBufferInfo infos[49];
  VkWriteDescriptorSet writes[49];
  uint32_t count = 0u;
  uint32_t index;
  uint64_t update_begin_ns;
  prom_model_block_weight_resource logical[PROM_MODEL_BLOCK_MAX_WEIGHTS];
#define PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(pipe, binding, source) do { \
  infos[count].buffer = (source)->buffer; infos[count].offset = 0u; infos[count].range = (source)->size; \
  memset(&writes[count], 0, sizeof(writes[count])); writes[count].sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET; \
  writes[count].dstSet = (pipe)->descriptor_set; writes[count].dstBinding = (binding); \
  writes[count].descriptorCount = 1u; writes[count].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER; \
  writes[count].pBufferInfo = &infos[count]; ++count; \
} while (0)
  if (state == NULL || block == NULL || weights == NULL ||
      (block->test_flags & PROM_TESTCFG_FAIL_PIPELINE_CREATE) != 0u) return 0;
  if (block->shared_owner != 0u && prom_model_block_is_context_refiner(block)) {
    memset(logical, 0, sizeof(logical));
    for (index = 0u; index < 11u; ++index)
      logical[index] = weights[k_prom_context_shared_weight_slots[index]];
    weights = logical;
  }
  for (index = 0u; index < block->weight_count; ++index) {
    if (weights[index].device.buffer == VK_NULL_HANDLE || weights[index].uploaded == 0u) return 0;
  }
  if (prom_model_block_is_context_refiner(block)) {
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1b_pipelines[0u], 1u, &weights[4u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1b_pipelines[1u], 1u, &weights[3u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1b_pipelines[2u], 1u, &weights[2u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1b_pipelines[3u], 1u, &weights[0u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1c_pipelines[1u], 1u, &weights[1u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1c_pipelines[2u], 1u, &weights[5u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1d_pipelines[0u], 1u, &weights[9u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1d_pipelines[1u], 1u, &weights[6u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1d_pipelines[1u], 2u, &weights[8u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1d_pipelines[3u], 1u, &weights[7u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1d_pipelines[3u], 2u, &weights[10u].device);
  } else {
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1b_pipelines[1u], 1u, &weights[1u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1b_pipelines[1u], 2u, &weights[0u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1b_pipelines[2u], 1u, &weights[6u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1b_pipelines[3u], 1u, &weights[5u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1b_pipelines[4u], 1u, &weights[4u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1b_pipelines[5u], 1u, &weights[2u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1c_pipelines[1u], 1u, &weights[3u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1c_pipelines[2u], 1u, &weights[7u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1d_pipelines[0u], 1u, &weights[11u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1d_pipelines[1u], 1u, &weights[8u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1d_pipelines[1u], 2u, &weights[10u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1d_pipelines[3u], 1u, &weights[9u].device);
    PROM_MODEL_BLOCK_DESCRIPTOR_WRITE(&block->m1d_pipelines[3u], 2u, &weights[12u].device);
  }
  update_begin_ns = prom_reduction_now_ns();
  vkUpdateDescriptorSets(state->device, count, writes, 0u, NULL);
  block->last_descriptor_update_ns = prom_reduction_elapsed_ns(update_begin_ns, prom_reduction_now_ns());
  block->descriptor_update_count += 1u;
  block->vk_update_descriptor_sets_count += 1u;
#undef PROM_MODEL_BLOCK_DESCRIPTOR_WRITE
  return 1;
}

static int prom_model_block_create_command_resources(prom_reduction_runtime_state* state,
                                                     prom_model_block_state* block) {
  VkCommandBufferAllocateInfo command_info;
  VkFenceCreateInfo fence_info;
  VkQueryPoolCreateInfo query_info;
  if (state == NULL || block == NULL) return 0;
  memset(&command_info, 0, sizeof(command_info));
  command_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO;
  command_info.commandPool = state->command_pool;
  command_info.level = VK_COMMAND_BUFFER_LEVEL_PRIMARY;
  command_info.commandBufferCount = 1u;
  if (vkAllocateCommandBuffers(state->device, &command_info, &block->command_buffer) != VK_SUCCESS) return 0;
  block->vk_allocate_command_buffers_count += 1u;
  memset(&fence_info, 0, sizeof(fence_info));
  fence_info.sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO;
  fence_info.flags = VK_FENCE_CREATE_SIGNALED_BIT;
  if (vkCreateFence(state->device, &fence_info, NULL, &block->fence) != VK_SUCCESS) return 0;
  if ((block->shader_id == PROM_MODEL_BLOCK_M1B_ADALN_SHADER_ID ||
       block->shader_id == PROM_MODEL_BLOCK_M1B_NORM_SHADER_ID ||
       block->shader_id == PROM_MODEL_BLOCK_MAIN_QK_ROPE_SHADER_ID) &&
      state->timestamp_supported != 0u && state->timestamp_period_ns > 0.0f) {
    memset(&query_info, 0, sizeof(query_info));
    query_info.sType = VK_STRUCTURE_TYPE_QUERY_POOL_CREATE_INFO;
    query_info.queryType = VK_QUERY_TYPE_TIMESTAMP;
    query_info.queryCount = PROM_MODEL_BLOCK_MAIN_QUERY_COUNT;
    if (vkCreateQueryPool(state->device, &query_info, NULL,
                          &block->m1b_timestamp_query_pool) != VK_SUCCESS) return 0;
    block->m1b_timestamp_supported = 1u;
    block->m1b_timestamp_period_ns = state->timestamp_period_ns;
  }
  return 1;
}

static int prom_model_block_submit_and_wait(prom_reduction_runtime_state* state,
                                            prom_model_block_state* block,
                                            int32_t* out_detail) {
  VkSubmitInfo submit_info;
  VkResult result;
  uint64_t phase_begin_ns;
  if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_QUEUE_SUBMIT_FAILED;
  if (state == NULL || block == NULL || block->command_buffer == VK_NULL_HANDLE || block->fence == VK_NULL_HANDLE) return 0;
  result = vkResetFences(state->device, 1u, &block->fence);
  if (result != VK_SUCCESS) {
    if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_COMMAND_RECORD_FAILED;
    return 0;
  }
  memset(&submit_info, 0, sizeof(submit_info));
  submit_info.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit_info.commandBufferCount = 1u;
  submit_info.pCommandBuffers = &block->command_buffer;
  if ((block->test_flags & PROM_TESTCFG_FAIL_QUEUE_SUBMIT) != 0u ||
      prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_QUEUE_SUBMIT)) {
    if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_QUEUE_SUBMIT_FAILED;
    return 0;
  }
  phase_begin_ns = prom_reduction_now_ns();
  result = vkQueueSubmit(state->queue, 1u, &submit_info, block->fence);
  block->last_queue_submit_ns = prom_reduction_elapsed_ns(phase_begin_ns, prom_reduction_now_ns());
  block->vk_queue_submit_count += 1u;
  if (result != VK_SUCCESS) return 0;
  if ((block->test_flags & PROM_TESTCFG_SKIP_SUBMIT_WAIT) != 0u ||
      prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_COMPLETION_OBSERVATION)) {
    block->quarantined = 1u;
    if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN;
    return 0;
  }
  phase_begin_ns = prom_reduction_now_ns();
  result = vkWaitForFences(state->device, 1u, &block->fence, VK_TRUE, UINT64_MAX);
  block->last_fence_wait_ns = prom_reduction_elapsed_ns(phase_begin_ns, prom_reduction_now_ns());
  block->vk_fence_wait_count += 1u;
  if (result != VK_SUCCESS) {
    block->quarantined = 1u;
    if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN;
    return 0;
  }
  return 1;
}

static uint64_t prom_model_block_resolve_gpu_span(prom_reduction_runtime_state* state,
                                                   prom_model_block_state* block,
                                                   uint32_t end_query) {
  uint64_t ticks[2];
  if (state == NULL || block == NULL || block->m1b_timestamp_supported == 0u ||
      block->m1b_timestamp_query_pool == VK_NULL_HANDLE || end_query == 0u) return 0u;
  if (vkGetQueryPoolResults(state->device, block->m1b_timestamp_query_pool, 0u, 1u,
                            sizeof(uint64_t), &ticks[0], sizeof(uint64_t),
                            VK_QUERY_RESULT_64_BIT | VK_QUERY_RESULT_WAIT_BIT) != VK_SUCCESS ||
      vkGetQueryPoolResults(state->device, block->m1b_timestamp_query_pool, end_query, 1u,
                            sizeof(uint64_t), &ticks[1], sizeof(uint64_t),
                            VK_QUERY_RESULT_64_BIT | VK_QUERY_RESULT_WAIT_BIT) != VK_SUCCESS) return 0u;
  return (uint64_t)((double)(ticks[1] - ticks[0]) *
                    (double)block->m1b_timestamp_period_ns + 0.5);
}

static int prom_model_block_reap(prom_reduction_runtime_state* state,
                                 prom_model_block_state* block) {
  if (state == NULL || block == NULL) return 0;
  if (block->quarantined == 0u) return 1;
  if (block->fence == VK_NULL_HANDLE ||
      vkWaitForFences(state->device, 1u, &block->fence, VK_TRUE, UINT64_MAX) != VK_SUCCESS) return 0;
  block->quarantined = 0u;
  block->output_valid = 0u;
  block->audit_valid = 0u;
  return 1;
}

static int prom_model_block_record_upload(prom_reduction_runtime_state* state,
                                          prom_model_block_state* block,
                                          prom_model_block_weight_resource* destination,
                                          uint32_t weight_index) {
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  VkResult result;
  int32_t detail = PROM_MODEL_BLOCK_DETAIL_AUDIT_FAILED;
  if (state == NULL || block == NULL || destination == NULL || weight_index >= PROM_MODEL_BLOCK_MAX_WEIGHTS) return 0;
  if (vkResetCommandBuffer(block->command_buffer, 0u) != VK_SUCCESS) return 0;
  block->vk_reset_command_buffer_count += 1u;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(block->command_buffer, &begin_info) != VK_SUCCESS) return 0;
  memset(&copy, 0, sizeof(copy));
  copy.size = (VkDeviceSize)destination[weight_index].byte_count;
  vkCmdCopyBuffer(block->command_buffer, block->weight_upload.buffer,
                  destination[weight_index].device.buffer, 1u, &copy);
  result = (block->test_flags & PROM_TESTCFG_FAIL_COMMAND_END) != 0u
               ? VK_ERROR_INITIALIZATION_FAILED
               : vkEndCommandBuffer(block->command_buffer);
  if (result != VK_SUCCESS) return 0;
  return prom_model_block_submit_and_wait(state, block, &detail);
}

/* One bounded staging slot is reused only after its transfer fence signals.
   The bridge invokes this routine on its one scoped prefetch goroutine while
   the compute queue owns the active window. */
static int prom_model_block_record_prefetch_upload(prom_reduction_runtime_state* state,
                                                   prom_model_block_state* block,
                                                   uint32_t weight_index) {
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  VkBufferMemoryBarrier barrier;
  VkSubmitInfo submit_info;
  if (state == NULL || block == NULL || block->prefetch_queue == VK_NULL_HANDLE ||
      block->prefetch_command_buffer == VK_NULL_HANDLE || block->prefetch_fence == VK_NULL_HANDLE ||
      weight_index >= PROM_MODEL_BLOCK_MAX_WEIGHTS ||
      block->prefetch_weights[weight_index].device.buffer == VK_NULL_HANDLE) return 0;
  if (vkResetCommandBuffer(block->prefetch_command_buffer, 0u) != VK_SUCCESS ||
      vkResetFences(state->device, 1u, &block->prefetch_fence) != VK_SUCCESS) return 0;
  block->vk_reset_command_buffer_count += 1u;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(block->prefetch_command_buffer, &begin_info) != VK_SUCCESS) return 0;
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_HOST_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = block->prefetch_weight_upload.buffer;
  barrier.size = block->prefetch_weights[weight_index].byte_count;
  vkCmdPipelineBarrier(block->prefetch_command_buffer, VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT,
                       0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  memset(&copy, 0, sizeof(copy));
  copy.size = block->prefetch_weights[weight_index].byte_count;
  vkCmdCopyBuffer(block->prefetch_command_buffer, block->prefetch_weight_upload.buffer,
                  block->prefetch_weights[weight_index].device.buffer, 1u, &copy);
  /* A transfer-only queue cannot name a compute stage.  Completion is made
     visible to the compute queue by prom_model_block_acquire_prefetched_weights
     after this fence signals and before descriptors are committed. */
  if (vkEndCommandBuffer(block->prefetch_command_buffer) != VK_SUCCESS) return 0;
  memset(&submit_info, 0, sizeof(submit_info));
  submit_info.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit_info.commandBufferCount = 1u;
  submit_info.pCommandBuffers = &block->prefetch_command_buffer;
  block->vk_queue_submit_count += 1u;
  if (vkQueueSubmit(block->prefetch_queue, 1u, &submit_info, block->prefetch_fence) != VK_SUCCESS) return 0;
  block->vk_fence_wait_count += 1u;
  if (vkWaitForFences(state->device, 1u, &block->prefetch_fence, VK_TRUE, UINT64_MAX) != VK_SUCCESS) return 0;
  return 1;
}

static int prom_model_block_acquire_prefetched_weights(prom_reduction_runtime_state* state,
                                                        prom_model_block_state* block) {
  VkCommandBufferBeginInfo begin_info;
  VkBufferMemoryBarrier barriers[PROM_MODEL_BLOCK_MAX_WEIGHTS];
  uint32_t index;
  uint32_t count = 0u;
  int32_t detail = PROM_MODEL_BLOCK_DETAIL_COMMAND_RECORD_FAILED;
  if (state == NULL || block == NULL || block->prefetch_weight_count == 0u) return 0;
  if (vkResetCommandBuffer(block->command_buffer, 0u) != VK_SUCCESS) return 0;
  block->vk_reset_command_buffer_count += 1u;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(block->command_buffer, &begin_info) != VK_SUCCESS) return 0;
  memset(barriers, 0, sizeof(barriers));
  for (index = 0u; index < block->prefetch_weight_count; ++index) {
    if (block->prefetch_weights[index].device.buffer == VK_NULL_HANDLE) return 0;
    barriers[count].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barriers[count].srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
    barriers[count].dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
    barriers[count].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[count].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[count].buffer = block->prefetch_weights[index].device.buffer;
    barriers[count].size = block->prefetch_weights[index].byte_count;
    count += 1u;
  }
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0u, 0u, NULL, count, barriers, 0u, NULL);
  if (vkEndCommandBuffer(block->command_buffer) != VK_SUCCESS) return 0;
  return prom_model_block_submit_and_wait(state, block, &detail);
}

static int prom_model_block_record_execute(prom_reduction_runtime_state* state,
                                           prom_model_block_state* block,
                                           uint32_t element_count,
                                           uint32_t audit_element_count,
                                           uint32_t audit_enabled,
                                           int32_t* out_detail) {
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy input_copy;
  VkBufferCopy output_copy;
  VkBufferMemoryBarrier barrier;
  prom_model_block_push_constants constants;
  VkResult result;
  if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_COMMAND_RECORD_FAILED;
  if (state == NULL || block == NULL || element_count == 0u) return 0;
  if (prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_COMMAND_RECORD)) return 0;
  if (vkResetCommandBuffer(block->command_buffer, 0u) != VK_SUCCESS) return 0;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(block->command_buffer, &begin_info) != VK_SUCCESS) return 0;
  memset(&input_copy, 0, sizeof(input_copy));
  input_copy.size = block->external_input_bytes;
  vkCmdCopyBuffer(block->command_buffer, block->input_upload.buffer, block->input_device.buffer, 1u, &input_copy);
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = block->input_device.buffer;
  barrier.size = VK_WHOLE_SIZE;
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  vkCmdBindPipeline(block->command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, block->pipeline.pipeline);
  vkCmdBindDescriptorSets(block->command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, block->pipeline_layout,
                          0u, 1u, &block->descriptor_set, 0u, NULL);
  constants.element_count = element_count;
  constants.audit_element_count = audit_enabled != 0u ? audit_element_count : 0u;
  vkCmdPushConstants(block->command_buffer, block->pipeline_layout, VK_SHADER_STAGE_COMPUTE_BIT,
                     0u, sizeof(constants), &constants);
  if ((block->test_flags & PROM_TESTCFG_FAIL_DISPATCH) != 0u) return 0;
  vkCmdDispatch(block->command_buffer, prom_reduction_ceil_div_u32(element_count, PROM_MODEL_BLOCK_WORKGROUP_SIZE), 1u, 1u);
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = block->output_device.buffer;
  barrier.size = VK_WHOLE_SIZE;
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  memset(&output_copy, 0, sizeof(output_copy));
  output_copy.size = block->external_output_bytes;
  vkCmdCopyBuffer(block->command_buffer, block->output_device.buffer, block->output_readback.buffer, 1u, &output_copy);
  if (audit_enabled != 0u) {
    barrier.buffer = block->audit_device.buffer;
    vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                         VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
    output_copy.size = block->declared_audit_bytes;
    vkCmdCopyBuffer(block->command_buffer, block->audit_device.buffer, block->audit_readback.buffer, 1u, &output_copy);
  }
  result = (block->test_flags & PROM_TESTCFG_FAIL_COMMAND_END) != 0u
               ? VK_ERROR_INITIALIZATION_FAILED
               : vkEndCommandBuffer(block->command_buffer);
  if (result != VK_SUCCESS) return 0;
  return prom_model_block_submit_and_wait(state, block, out_detail);
}

static int prom_model_block_m1b_create_buffers(prom_reduction_runtime_state* state,
                                               prom_model_block_state* block,
                                               VkDeviceSize max_weight_bytes) {
  const VkBufferUsageFlags device_storage = VK_BUFFER_USAGE_STORAGE_BUFFER_BIT |
                                            VK_BUFFER_USAGE_TRANSFER_DST_BIT |
                                            VK_BUFFER_USAGE_TRANSFER_SRC_BIT;
  const VkMemoryPropertyFlags host_visible = VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT |
                                              VK_MEMORY_PROPERTY_HOST_COHERENT_BIT;
  return prom_model_block_create_buffer(state, block, &block->input_upload,
                                        PROM_MODEL_BLOCK_M1B_BF16_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS),
                                        VK_BUFFER_USAGE_TRANSFER_SRC_BIT, host_visible, 1) &&
         prom_model_block_create_buffer(state, block, &block->input_bf16_device,
                                        PROM_MODEL_BLOCK_M1B_BF16_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->input_device,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->resident_boundary_device,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->timestep_upload,
                                        PROM_MODEL_BLOCK_M1B_BF16_BYTES(PROM_MODEL_BLOCK_M1B_TIMESTEP_ELEMENTS),
                                        VK_BUFFER_USAGE_TRANSFER_SRC_BIT, host_visible, 1) &&
         prom_model_block_create_buffer(state, block, &block->timestep_bf16_device,
                                        PROM_MODEL_BLOCK_M1B_BF16_BYTES(PROM_MODEL_BLOCK_M1B_TIMESTEP_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->timestep_device,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_TIMESTEP_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->adaln_projection,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(15360u), device_storage,
                                        VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->attention_scale,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->attention_gate,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->mlp_scale,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->mlp_gate,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->modulated,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->norm_audit,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->qkv,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_QKV_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->attention,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->attention_projection,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->attention_residual,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         /* M1C's fixed third descriptor is a 64-float device-local audit/status
            record. It is bounded and separate from the full host readback. */
         prom_model_block_create_buffer(state, block, &block->audit_device,
                                        PROM_MODEL_BLOCK_M1C_TRANSIENT_AUDIT_FLOATS * sizeof(float),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->audit_readback, block->declared_audit_bytes,
                                        VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                        host_visible, 1) &&
         prom_model_block_create_buffer(state, block, &block->weight_upload, max_weight_bytes,
                                        VK_BUFFER_USAGE_TRANSFER_SRC_BIT, host_visible, 1);
}

static void prom_model_block_copy_active_portfolio(prom_model_block_state* block, uint32_t family,
                                                   int save) {
  prom_model_block_m1b_pipeline* m1b = NULL;
  prom_model_block_m1b_pipeline* m1c = NULL;
  prom_model_block_m1b_pipeline* m1d = NULL;
  prom_model_block_m1b_pipeline* audit = NULL;
  if (block == NULL) return;
  if (family == PROM_NOISE_REFINER_FAMILY_Z_IMAGE_TURBO) {
    m1b = block->noise_m1b_pipelines; m1c = block->noise_m1c_pipelines;
    m1d = block->noise_m1d_pipelines; audit = block->noise_audit_pipelines;
  } else if (family == PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO) {
    m1b = block->context_m1b_pipelines; m1c = block->context_m1c_pipelines;
    m1d = block->context_m1d_pipelines; audit = block->context_audit_pipelines;
  } else if (family == PROM_MAIN_TRANSFORMER_FAMILY_Z_IMAGE_TURBO) {
    m1b = block->main_m1b_pipelines; m1c = block->main_m1c_pipelines;
    m1d = block->main_m1d_pipelines;
  }
  if (m1b == NULL || m1c == NULL || m1d == NULL) return;
  if (save != 0) {
    memcpy(m1b, block->m1b_pipelines, sizeof(block->m1b_pipelines));
    memcpy(m1c, block->m1c_pipelines, sizeof(block->m1c_pipelines));
    memcpy(m1d, block->m1d_pipelines, sizeof(block->m1d_pipelines));
    if (audit != NULL) memcpy(audit, block->audit_pipelines, sizeof(block->audit_pipelines));
  } else {
    memcpy(block->m1b_pipelines, m1b, sizeof(block->m1b_pipelines));
    memcpy(block->m1c_pipelines, m1c, sizeof(block->m1c_pipelines));
    memcpy(block->m1d_pipelines, m1d, sizeof(block->m1d_pipelines));
    if (audit != NULL) memcpy(block->audit_pipelines, audit, sizeof(block->audit_pipelines));
    else memset(block->audit_pipelines, 0, sizeof(block->audit_pipelines));
  }
}

static int prom_model_block_create_shared_buffers(prom_reduction_runtime_state* state,
                                                  prom_model_block_state* block,
                                                  VkDeviceSize max_weight_bytes) {
  const VkMemoryPropertyFlags host_visible = VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT |
                                              VK_MEMORY_PROPERTY_HOST_COHERENT_BIT;
  if (!prom_main_transformer_create_buffers(state, block, max_weight_bytes)) return 0;
  /* Main owns the largest activation resources.  The smaller family-only
     views alias them only where their generated lifetimes cannot overlap. */
  if (!prom_model_block_create_buffer(state, block, &block->input_upload,
                                      PROM_MODEL_BLOCK_M1B_BF16_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS),
                                      VK_BUFFER_USAGE_TRANSFER_SRC_BIT, host_visible, 1)) return 0;
  if (!prom_model_block_create_buffer(state, block, &block->context_unit,
                                      PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS),
                                      VK_BUFFER_USAGE_STORAGE_BUFFER_BIT, host_visible, 1)) return 0;
  for (uint32_t index = 0u; index < PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS; ++index)
    ((float*)block->context_unit.mapped)[index] = 1.0f;
  prom_vk_destroy_buffer(state->device, &block->input_bf16_device);
  block->input_bf16_device = block->attention_residual;
  block->resident_boundary_device = block->attention_residual;
  /* Context W3 and QKV are both consumed by the gated FFN stage, so they
     cannot alias. The closed audit arena is storage-capable and otherwise
     inactive in production Context execution. */
  block->context_w3 = block->audit_readback;
  return 1;
}

static void prom_model_block_destroy_portfolio(prom_reduction_runtime_state* state,
                                              prom_model_block_m1b_pipeline* pipelines,
                                              uint32_t count) {
  uint32_t index;
  if (state == NULL || state->device == VK_NULL_HANDLE || pipelines == NULL) return;
  for (index = 0u; index < count; ++index) {
    prom_model_block_m1b_pipeline* pipeline = &pipelines[index];
    prom_reduction_destroy_pipeline(state->device, &pipeline->pipeline);
    if (pipeline->pipeline_layout != VK_NULL_HANDLE) vkDestroyPipelineLayout(state->device, pipeline->pipeline_layout, NULL);
    if (pipeline->descriptor_pool != VK_NULL_HANDLE) vkDestroyDescriptorPool(state->device, pipeline->descriptor_pool, NULL);
    if (pipeline->descriptor_set_layout != VK_NULL_HANDLE) vkDestroyDescriptorSetLayout(state->device, pipeline->descriptor_set_layout, NULL);
  }
}

static int prom_model_block_create_shared_portfolios(prom_reduction_runtime_state* state,
                                                     prom_model_block_state* block) {
  uint32_t saved_family = block->assembly_family;
  uint32_t saved_shader = block->shader_id;
  if (!prom_model_block_m1b_create_pipelines(state, block) ||
      !prom_model_block_m1c_create_pipelines(state, block) ||
      !prom_model_block_m1d_create_pipelines(state, block) ||
      !prom_model_block_audit_create_pipelines(state, block)) return 0;
  prom_model_block_copy_active_portfolio(block, PROM_NOISE_REFINER_FAMILY_Z_IMAGE_TURBO, 1);
  memset(block->m1b_pipelines, 0, sizeof(block->m1b_pipelines));
  memset(block->m1c_pipelines, 0, sizeof(block->m1c_pipelines));
  memset(block->m1d_pipelines, 0, sizeof(block->m1d_pipelines));
  memset(block->audit_pipelines, 0, sizeof(block->audit_pipelines));
  block->assembly_family = PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO;
  if (!prom_context_refiner_create_pipelines(state, block) ||
      !prom_model_block_audit_create_pipelines(state, block)) return 0;
  prom_model_block_copy_active_portfolio(block, PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO, 1);
  memset(block->m1b_pipelines, 0, sizeof(block->m1b_pipelines));
  memset(block->m1c_pipelines, 0, sizeof(block->m1c_pipelines));
  memset(block->m1d_pipelines, 0, sizeof(block->m1d_pipelines));
  memset(block->audit_pipelines, 0, sizeof(block->audit_pipelines));
  block->assembly_family = PROM_MAIN_TRANSFORMER_FAMILY_Z_IMAGE_TURBO;
  if (!prom_main_transformer_create_pipelines(state, block)) return 0;
  prom_model_block_copy_active_portfolio(block, PROM_MAIN_TRANSFORMER_FAMILY_Z_IMAGE_TURBO, 1);
  block->assembly_family = saved_family;
  block->shader_id = saved_shader;
  prom_model_block_copy_active_portfolio(block, saved_family, 0);
  return 1;
}

static int prom_main_transformer_create_buffers(prom_reduction_runtime_state* state,
                                                prom_model_block_state* block,
                                                VkDeviceSize max_weight_bytes) {
  const VkBufferUsageFlags device_storage = VK_BUFFER_USAGE_STORAGE_BUFFER_BIT |
                                            VK_BUFFER_USAGE_TRANSFER_DST_BIT |
                                            VK_BUFFER_USAGE_TRANSFER_SRC_BIT;
  const VkMemoryPropertyFlags host_visible = VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT |
                                              VK_MEMORY_PROPERTY_HOST_COHERENT_BIT;
  return prom_model_block_create_buffer(state, block, &block->input_bf16_device,
                                        sizeof(uint32_t), device_storage,
                                        VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->input_device,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_MAIN_MODEL_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->timestep_upload,
                                        PROM_MODEL_BLOCK_M1B_BF16_BYTES(PROM_MODEL_BLOCK_M1B_TIMESTEP_ELEMENTS),
                                        VK_BUFFER_USAGE_TRANSFER_SRC_BIT, host_visible, 1) &&
         prom_model_block_create_buffer(state, block, &block->timestep_bf16_device,
                                        PROM_MODEL_BLOCK_M1B_BF16_BYTES(PROM_MODEL_BLOCK_M1B_TIMESTEP_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->timestep_device,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_TIMESTEP_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->adaln_projection,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(15360u), device_storage,
                                        VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->attention_scale,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->attention_gate,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->mlp_scale,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->mlp_gate,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->modulated,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_MAIN_MODEL_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->norm_audit,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_MAIN_MODEL_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->qkv,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_MAIN_QKV_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->attention,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_MAIN_MODEL_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->attention_projection,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_MAIN_MODEL_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->attention_residual,
                                        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_MAIN_MODEL_ELEMENTS),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->audit_device,
                                        PROM_MODEL_BLOCK_M1C_TRANSIENT_AUDIT_FLOATS * sizeof(float),
                                        device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) &&
         prom_model_block_create_buffer(state, block, &block->audit_readback, block->declared_audit_bytes,
                                        VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                        host_visible, 1) &&
         prom_model_block_create_buffer(state, block, &block->weight_upload, max_weight_bytes,
                                        VK_BUFFER_USAGE_TRANSFER_SRC_BIT, host_visible, 1);
}

static void prom_model_block_m1b_bind_and_dispatch(VkCommandBuffer command_buffer,
                                                   const prom_model_block_m1b_pipeline* pipeline,
                                                   const void* constants, uint32_t constant_bytes,
                                                   uint32_t x, uint32_t y, uint32_t z) {
  vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, pipeline->pipeline.pipeline);
  vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, pipeline->pipeline_layout,
                          0u, 1u, &pipeline->descriptor_set, 0u, NULL);
  vkCmdPushConstants(command_buffer, pipeline->pipeline_layout, VK_SHADER_STAGE_COMPUTE_BIT,
                     0u, constant_bytes, constants);
  vkCmdDispatch(command_buffer, x, y, z);
}

static void prom_model_block_m1b_record_audit_capture(
    VkCommandBuffer command_buffer, const prom_vk_buffer* source, VkDeviceSize source_offset,
    VkDeviceSize bytes, int qkv_segmented, const prom_vk_buffer* audit_readback) {
  VkBufferMemoryBarrier barrier;
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = source->buffer;
  barrier.offset = source_offset;
  barrier.size = qkv_segmented != 0 ? VK_WHOLE_SIZE : bytes;
  vkCmdPipelineBarrier(command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  if (qkv_segmented != 0) {
    VkBufferCopy regions[1024u];
    const VkDeviceSize row_bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS);
    const VkDeviceSize fused_row_bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(11520u);
    uint32_t token;
    for (token = 0u; token < 1024u; ++token) {
      regions[token].srcOffset = (VkDeviceSize)token * fused_row_bytes + source_offset;
      regions[token].dstOffset = (VkDeviceSize)token * row_bytes;
      regions[token].size = row_bytes;
    }
    vkCmdCopyBuffer(command_buffer, source->buffer, audit_readback->buffer, 1024u, regions);
  } else {
    VkBufferCopy copy;
    memset(&copy, 0, sizeof(copy));
    copy.srcOffset = source_offset;
    copy.size = bytes;
    vkCmdCopyBuffer(command_buffer, source->buffer, audit_readback->buffer, 1u, &copy);
  }
  barrier.srcAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
  barrier.dstAccessMask = VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT;
  barrier.offset = 0u;
  barrier.size = VK_WHOLE_SIZE;
  vkCmdPipelineBarrier(command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
}

static void prom_model_block_record_small_audit_capture(
    VkCommandBuffer command_buffer, const prom_vk_buffer* source, VkDeviceSize bytes,
    const prom_vk_buffer* audit_readback, VkDeviceSize destination_offset) {
  VkBufferMemoryBarrier barrier;
  VkBufferCopy copy;
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = source->buffer;
  barrier.size = bytes;
  vkCmdPipelineBarrier(command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  memset(&copy, 0, sizeof(copy));
  copy.size = bytes;
  copy.dstOffset = destination_offset;
  vkCmdCopyBuffer(command_buffer, source->buffer, audit_readback->buffer, 1u, &copy);
}

static void prom_model_block_m1b_capture_stage(prom_model_block_state* block,
                                                uint32_t audit_stage,
                                                uint32_t boundary) {
  const prom_vk_buffer* source = NULL;
  VkDeviceSize offset = 0u;
  VkDeviceSize bytes = 0u;
  int segmented = 0;
  if (block == NULL || audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_NONE) return;
  if (boundary == 0u) {
    if (audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_INGRESS_INPUT) {
      source = &block->input_device;
      bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS);
    } else if (audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_INGRESS_TIMESTEP) {
      source = &block->timestep_device;
      bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_TIMESTEP_ELEMENTS);
    }
  } else if (boundary == 1u) {
    bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS);
    if (audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_ADALN_PROJECTION) {
      source = &block->adaln_projection;
      bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(15360u);
    } else if (audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_ATTENTION_SCALE_RAW) {
      source = &block->adaln_projection;
    } else if (audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_ATTENTION_SCALE_ADJUSTED) {
      source = &block->attention_scale;
    } else if (audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_ATTENTION_GATE_RAW) {
      source = &block->adaln_projection;
      offset = PROM_MODEL_BLOCK_M1B_FP32_BYTES(3840u);
    } else if (audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_ATTENTION_GATE_TANH) {
      source = &block->attention_gate;
    } else if (audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_MLP_SCALE_RAW) {
      source = &block->adaln_projection;
      offset = PROM_MODEL_BLOCK_M1B_FP32_BYTES(7680u);
    } else if (audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_MLP_SCALE_ADJUSTED) {
      source = &block->mlp_scale;
    } else if (audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_MLP_GATE_RAW) {
      source = &block->adaln_projection;
      offset = PROM_MODEL_BLOCK_M1B_FP32_BYTES(11520u);
    } else if (audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_MLP_GATE_TANH) {
      source = &block->mlp_gate;
    }
  } else if (boundary == 2u) {
    bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS);
    if (audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_ATTENTION_NORM) source = &block->norm_audit;
    if (audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_ATTENTION_MODULATED) source = &block->modulated;
  } else if (boundary == 3u) {
    if (audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_FUSED_QKV) {
      source = &block->qkv;
      bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_QKV_ELEMENTS);
    } else if (audit_stage >= PROM_MODEL_BLOCK_M1B_AUDIT_Q &&
               audit_stage <= PROM_MODEL_BLOCK_M1B_AUDIT_V) {
      source = &block->qkv;
      offset = PROM_MODEL_BLOCK_M1B_FP32_BYTES(
          (audit_stage - PROM_MODEL_BLOCK_M1B_AUDIT_Q) * PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS);
      bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS);
      segmented = 1;
    }
  } else if (boundary == 4u) {
    bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS);
    if (audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_Q_NORM) source = &block->norm_audit;
    if (audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_POSITIONED_Q) {
      source = &block->qkv;
      segmented = 1;
    }
  } else if (boundary == 5u) {
    bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS);
    if (audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_K_NORM) source = &block->norm_audit;
    if (audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_POSITIONED_K) {
      source = &block->qkv;
      offset = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS);
      segmented = 1;
    }
  }
  if (source != NULL) {
    prom_model_block_m1b_record_audit_capture(block->command_buffer, source, offset, bytes,
                                               segmented, &block->audit_readback);
  }
}

static int prom_model_block_m1b_record_execute(prom_reduction_runtime_state* state,
                                                prom_model_block_state* block,
                                                uint32_t audit_stage,
                                                int resident_input,
                                                int32_t* out_detail) {
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  VkBufferMemoryBarrier transfer_barriers[2];
  VkBufferMemoryBarrier resident_copy_barrier;
  prom_model_block_m1b_ingress_constants ingress_constants = {
      PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS, PROM_MODEL_BLOCK_M1B_TIMESTEP_ELEMENTS};
  prom_model_block_m1b_adaln_constants adaln_constants = {15360u, 256u};
  prom_model_block_m1b_norm_constants norm_constants = {1.0e-5f, 1024u, 3840u, 0u};
  prom_model_block_m1b_qkv_constants qkv_constants = {1024u, 3840u, 11520u, 0u};
  prom_model_block_m1b_head_constants head_constants = {1.0e-5f, 1024u, 30u, 128u};
  VkResult result;
  if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_COMMAND_RECORD_FAILED;
  if (state == NULL || block == NULL || prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_COMMAND_RECORD)) return 0;
  if (vkResetCommandBuffer(block->command_buffer, 0u) != VK_SUCCESS) return 0;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(block->command_buffer, &begin_info) != VK_SUCCESS) return 0;
  memset(&copy, 0, sizeof(copy));
  if (resident_input == 1) {
    copy.size = block->attention.size;
    vkCmdCopyBuffer(block->command_buffer, block->attention.buffer, block->resident_boundary_device.buffer, 1u, &copy);
    memset(&resident_copy_barrier, 0, sizeof(resident_copy_barrier));
    resident_copy_barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    resident_copy_barrier.srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
    resident_copy_barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
    resident_copy_barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    resident_copy_barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    resident_copy_barrier.buffer = block->resident_boundary_device.buffer;
    resident_copy_barrier.size = VK_WHOLE_SIZE;
    vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                         VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 1u, &resident_copy_barrier, 0u, NULL);
    vkCmdCopyBuffer(block->command_buffer, block->resident_boundary_device.buffer, block->input_device.buffer, 1u, &copy);
  } else if (resident_input == 2) {
    copy.size = block->resident_boundary_device.size;
    vkCmdCopyBuffer(block->command_buffer, block->resident_boundary_device.buffer, block->input_device.buffer, 1u, &copy);
  } else if (resident_input == 0) {
    copy.size = block->input_upload.size;
    vkCmdCopyBuffer(block->command_buffer, block->input_upload.buffer, block->input_bf16_device.buffer, 1u, &copy);
    copy.size = block->timestep_upload.size;
    vkCmdCopyBuffer(block->command_buffer, block->timestep_upload.buffer, block->timestep_bf16_device.buffer, 1u, &copy);
  }
  memset(transfer_barriers, 0, sizeof(transfer_barriers));
  for (uint32_t index = 0u; index < (resident_input == 1 ? 2u : (resident_input == 0 ? 2u : 1u)); ++index) {
    transfer_barriers[index].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    transfer_barriers[index].srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
    transfer_barriers[index].dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
    transfer_barriers[index].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    transfer_barriers[index].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    transfer_barriers[index].buffer = resident_input == 1 ?
        (index == 0u ? block->resident_boundary_device.buffer : block->input_device.buffer) :
        (resident_input == 2 ? block->input_device.buffer :
         (index == 0u ? block->input_bf16_device.buffer : block->timestep_bf16_device.buffer));
    transfer_barriers[index].size = VK_WHOLE_SIZE;
  }
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0u, 0u, NULL,
                       resident_input == 0 ? 2u : (resident_input == 1 ? 2u : 1u), transfer_barriers, 0u, NULL);
  if (block->m1b_timestamp_supported != 0u &&
      block->m1b_timestamp_query_pool != VK_NULL_HANDLE) {
    vkCmdResetQueryPool(block->command_buffer, block->m1b_timestamp_query_pool, 0u,
                        PROM_MODEL_BLOCK_M1B_PIPELINE_COUNT + 1u);
    vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                        block->m1b_timestamp_query_pool, 0u);
  }
  if ((block->test_flags & PROM_TESTCFG_FAIL_DISPATCH) != 0u) {
    if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_INGRESS_DISPATCH_FAILED;
    return 0;
  }
  if (resident_input == 0) {
    prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[0u],
                                           &ingress_constants, sizeof(ingress_constants), 15360u, 1u, 1u);
  }
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(
      block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
      block->m1b_timestamp_query_pool, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (resident_input == 0) prom_model_block_m1b_capture_stage(block, audit_stage, 0u);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[1u],
                                         &adaln_constants, sizeof(adaln_constants), 60u, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(
      block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
      block->m1b_timestamp_query_pool, 2u);
  prom_model_block_m1b_capture_stage(block, audit_stage, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[2u],
                                         &norm_constants, sizeof(norm_constants), 1024u, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(
      block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
      block->m1b_timestamp_query_pool, 3u);
  prom_model_block_m1b_capture_stage(block, audit_stage, 2u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[3u],
                                         &qkv_constants, sizeof(qkv_constants), 64u, 720u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(
      block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
      block->m1b_timestamp_query_pool, 4u);
  prom_model_block_m1b_capture_stage(block, audit_stage, 3u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[4u],
                                         &head_constants, sizeof(head_constants), 30720u, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(
      block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
      block->m1b_timestamp_query_pool, 5u);
  prom_model_block_m1b_capture_stage(block, audit_stage, 4u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[5u],
                                         &head_constants, sizeof(head_constants), 30720u, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(
      block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
      block->m1b_timestamp_query_pool, 6u);
  prom_model_block_m1b_capture_stage(block, audit_stage, 5u);
  result = (block->test_flags & PROM_TESTCFG_FAIL_COMMAND_END) != 0u
               ? VK_ERROR_INITIALIZATION_FAILED : vkEndCommandBuffer(block->command_buffer);
  if (result != VK_SUCCESS) return 0;
  return prom_model_block_submit_and_wait(state, block, out_detail);
}

void prom_model_block_cleanup_state(prom_reduction_runtime_state* state) {
  prom_model_block_state* block;
  uint64_t next_block_id;
  uint32_t index;
  if (state == NULL) return;
  block = &state->model_block;
  next_block_id = block->next_block_id;
  if (state->device != VK_NULL_HANDLE && block->fence != VK_NULL_HANDLE && block->quarantined != 0u) {
    (void)vkWaitForFences(state->device, 1u, &block->fence, VK_TRUE, UINT64_MAX);
  }
  if (state->device != VK_NULL_HANDLE) {
    if (block->shared_owner != 0u) {
      /* Active arrays are views into these closed portfolios. Destroy the
         physical portfolio once, then clear aliases so generic cleanup owns
         each shared buffer exactly once. */
      prom_model_block_destroy_portfolio(state, block->noise_m1b_pipelines, PROM_MODEL_BLOCK_M1B_PIPELINE_COUNT);
      prom_model_block_destroy_portfolio(state, block->noise_m1c_pipelines, PROM_MODEL_BLOCK_M1C_PIPELINE_COUNT);
      prom_model_block_destroy_portfolio(state, block->noise_m1d_pipelines, PROM_MODEL_BLOCK_M1D_PIPELINE_COUNT);
      prom_model_block_destroy_portfolio(state, block->noise_audit_pipelines, PROM_MODEL_BLOCK_AUDIT_SOURCE_COUNT);
      prom_model_block_destroy_portfolio(state, block->context_m1b_pipelines, PROM_MODEL_BLOCK_M1B_PIPELINE_COUNT);
      prom_model_block_destroy_portfolio(state, block->context_m1c_pipelines, PROM_MODEL_BLOCK_M1C_PIPELINE_COUNT);
      prom_model_block_destroy_portfolio(state, block->context_m1d_pipelines, PROM_MODEL_BLOCK_M1D_PIPELINE_COUNT);
      prom_model_block_destroy_portfolio(state, block->context_audit_pipelines, PROM_MODEL_BLOCK_AUDIT_SOURCE_COUNT);
      prom_model_block_destroy_portfolio(state, block->main_m1b_pipelines, PROM_MODEL_BLOCK_M1B_PIPELINE_COUNT);
      prom_model_block_destroy_portfolio(state, block->main_m1c_pipelines, PROM_MODEL_BLOCK_M1C_PIPELINE_COUNT);
      prom_model_block_destroy_portfolio(state, block->main_m1d_pipelines, PROM_MODEL_BLOCK_M1D_PIPELINE_COUNT);
      memset(block->m1b_pipelines, 0, sizeof(block->m1b_pipelines));
      memset(block->m1c_pipelines, 0, sizeof(block->m1c_pipelines));
      memset(block->m1d_pipelines, 0, sizeof(block->m1d_pipelines));
      memset(block->audit_pipelines, 0, sizeof(block->audit_pipelines));
      memset(&block->input_bf16_device, 0, sizeof(block->input_bf16_device));
      memset(&block->resident_boundary_device, 0, sizeof(block->resident_boundary_device));
      memset(&block->context_w3, 0, sizeof(block->context_w3));
      block->owner_destruction_count += 1u;
    }
    for (index = 0u; index < PROM_MODEL_BLOCK_M1B_PIPELINE_COUNT; ++index) {
      prom_model_block_m1b_pipeline* pipeline = &block->m1b_pipelines[index];
      prom_reduction_destroy_pipeline(state->device, &pipeline->pipeline);
      if (pipeline->pipeline_layout != VK_NULL_HANDLE) vkDestroyPipelineLayout(state->device, pipeline->pipeline_layout, NULL);
      if (pipeline->descriptor_pool != VK_NULL_HANDLE) vkDestroyDescriptorPool(state->device, pipeline->descriptor_pool, NULL);
      if (pipeline->descriptor_set_layout != VK_NULL_HANDLE) vkDestroyDescriptorSetLayout(state->device, pipeline->descriptor_set_layout, NULL);
    }
    for (index = 0u; index < PROM_MODEL_BLOCK_AUDIT_SOURCE_COUNT; ++index) {
      prom_model_block_m1b_pipeline* pipeline = &block->audit_pipelines[index];
      prom_reduction_destroy_pipeline(state->device, &pipeline->pipeline);
      if (pipeline->pipeline_layout != VK_NULL_HANDLE) vkDestroyPipelineLayout(state->device, pipeline->pipeline_layout, NULL);
      if (pipeline->descriptor_pool != VK_NULL_HANDLE) vkDestroyDescriptorPool(state->device, pipeline->descriptor_pool, NULL);
      if (pipeline->descriptor_set_layout != VK_NULL_HANDLE) vkDestroyDescriptorSetLayout(state->device, pipeline->descriptor_set_layout, NULL);
    }
    for (index = 0u; index < PROM_MODEL_BLOCK_M1C_PIPELINE_COUNT; ++index) {
      prom_model_block_m1b_pipeline* pipeline = &block->m1c_pipelines[index];
      prom_reduction_destroy_pipeline(state->device, &pipeline->pipeline);
      if (pipeline->pipeline_layout != VK_NULL_HANDLE) vkDestroyPipelineLayout(state->device, pipeline->pipeline_layout, NULL);
      if (pipeline->descriptor_pool != VK_NULL_HANDLE) vkDestroyDescriptorPool(state->device, pipeline->descriptor_pool, NULL);
      if (pipeline->descriptor_set_layout != VK_NULL_HANDLE) vkDestroyDescriptorSetLayout(state->device, pipeline->descriptor_set_layout, NULL);
    }
    for (index = 0u; index < PROM_MODEL_BLOCK_M1D_PIPELINE_COUNT; ++index) {
      prom_model_block_m1b_pipeline* pipeline = &block->m1d_pipelines[index];
      prom_reduction_destroy_pipeline(state->device, &pipeline->pipeline);
      if (pipeline->pipeline_layout != VK_NULL_HANDLE) vkDestroyPipelineLayout(state->device, pipeline->pipeline_layout, NULL);
      if (pipeline->descriptor_pool != VK_NULL_HANDLE) vkDestroyDescriptorPool(state->device, pipeline->descriptor_pool, NULL);
      if (pipeline->descriptor_set_layout != VK_NULL_HANDLE) vkDestroyDescriptorSetLayout(state->device, pipeline->descriptor_set_layout, NULL);
    }
    for (index = 0u; index < PROM_MODEL_BLOCK_MAX_WEIGHTS; ++index) {
      prom_vk_destroy_buffer(state->device, &block->weights[index].device);
      prom_vk_destroy_buffer(state->device, &block->pending_weights[index].device);
      prom_vk_destroy_buffer(state->device, &block->prefetch_weights[index].device);
    }
    prom_vk_destroy_buffer(state->device, &block->weight_upload);
    prom_vk_destroy_buffer(state->device, &block->prefetch_weight_upload);
    prom_vk_destroy_buffer(state->device, &block->audit_readback);
    prom_vk_destroy_buffer(state->device, &block->audit_device);
    prom_vk_destroy_buffer(state->device, &block->qkv);
    prom_vk_destroy_buffer(state->device, &block->attention_residual);
    prom_vk_destroy_buffer(state->device, &block->attention_projection);
    prom_vk_destroy_buffer(state->device, &block->attention);
    prom_vk_destroy_buffer(state->device, &block->norm_audit);
    prom_vk_destroy_buffer(state->device, &block->modulated);
    prom_vk_destroy_buffer(state->device, &block->mlp_gate);
    prom_vk_destroy_buffer(state->device, &block->mlp_scale);
    prom_vk_destroy_buffer(state->device, &block->attention_gate);
    prom_vk_destroy_buffer(state->device, &block->attention_scale);
    prom_vk_destroy_buffer(state->device, &block->adaln_projection);
    prom_vk_destroy_buffer(state->device, &block->timestep_device);
    prom_vk_destroy_buffer(state->device, &block->timestep_bf16_device);
    prom_vk_destroy_buffer(state->device, &block->timestep_upload);
    prom_vk_destroy_buffer(state->device, &block->output_readback);
    prom_vk_destroy_buffer(state->device, &block->output_device);
    prom_vk_destroy_buffer(state->device, &block->resident_boundary_device);
    prom_vk_destroy_buffer(state->device, &block->input_device);
    prom_vk_destroy_buffer(state->device, &block->input_bf16_device);
    prom_vk_destroy_buffer(state->device, &block->input_upload);
    prom_vk_destroy_buffer(state->device, &block->context_unit);
    prom_vk_destroy_buffer(state->device, &block->context_w3);
    prom_reduction_destroy_pipeline(state->device, &block->pipeline);
    if (block->m1b_timestamp_query_pool != VK_NULL_HANDLE) {
      vkDestroyQueryPool(state->device, block->m1b_timestamp_query_pool, NULL);
    }
    if (block->fence != VK_NULL_HANDLE) vkDestroyFence(state->device, block->fence, NULL);
    if (block->prefetch_fence != VK_NULL_HANDLE) vkDestroyFence(state->device, block->prefetch_fence, NULL);
    if (block->command_buffer != VK_NULL_HANDLE && state->command_pool != VK_NULL_HANDLE) {
      vkFreeCommandBuffers(state->device, state->command_pool, 1u, &block->command_buffer);
    }
    if (block->prefetch_command_buffer != VK_NULL_HANDLE && block->prefetch_command_pool != VK_NULL_HANDLE) {
      vkFreeCommandBuffers(state->device, block->prefetch_command_pool, 1u, &block->prefetch_command_buffer);
    }
    if (block->pipeline_layout != VK_NULL_HANDLE) vkDestroyPipelineLayout(state->device, block->pipeline_layout, NULL);
    if (block->descriptor_pool != VK_NULL_HANDLE) vkDestroyDescriptorPool(state->device, block->descriptor_pool, NULL);
    if (block->descriptor_set_layout != VK_NULL_HANDLE) vkDestroyDescriptorSetLayout(state->device, block->descriptor_set_layout, NULL);
  }
  memset(block, 0, sizeof(*block));
  block->next_block_id = next_block_id == 0u ? 1u : next_block_id;
}

int prom_reactor_runtime_model_block_create_impl(
    void* handle, const PrometheusModelBlockCreateRequest* request, uint64_t* out_block_id,
    PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  prom_vk_runtime_services services;
  uint64_t max_weight_bytes = 0u;
  uint64_t next_block_id;
  uint32_t index;
  uint32_t shared_owner = 0u;
  uint32_t prefetch_owner = 0u;
  uint32_t prefetch_transfer_family = UINT32_MAX;
  int32_t detail = PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST;
  if (out_block_id != NULL) *out_block_id = 0u;
  if (!prom_model_block_validate_create_request(request, &detail) || out_block_id == NULL ||
      !prom_reactor_runtime_validate_handle(handle)) {
    prom_model_block_fill_evidence(NULL, detail, out_evidence);
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL || prom_reactor_runtime_get_vk_services(handle, &services) != PROM_OK) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_RESOURCE_CREATE_FAILED, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  if (block->created != 0u) {
    prom_model_block_fill_evidence(block, PROM_MODEL_BLOCK_DETAIL_ALREADY_CREATED, out_evidence);
    return PROM_ERROR;
  }
  next_block_id = block->next_block_id;
  shared_owner = state->model_block_create_shared_owner;
  state->model_block_create_shared_owner = 0u;
  prefetch_owner = state->model_block_create_prefetch;
  prefetch_transfer_family = state->model_block_create_transfer_queue_family;
  state->model_block_create_prefetch = 0u;
  state->model_block_create_transfer_queue_family = UINT32_MAX;
  memset(block, 0, sizeof(*block));
  block->next_block_id = next_block_id == 0u ? 1u : next_block_id;
  block->test_flags = services.test_flags | state->model_block_create_test_flags;
  state->model_block_create_test_flags = 0u;
  if (prefetch_owner != 0u &&
      (services.transfer_queue_available == 0u || services.transfer_queue == VK_NULL_HANDLE ||
       services.transfer_command_pool == VK_NULL_HANDLE ||
       prefetch_transfer_family != services.transfer_queue_family_index ||
       services.compute_queue_family_index == services.transfer_queue_family_index)) goto fail;
  block->prefetch_queue = prefetch_owner != 0u ? services.transfer_queue : VK_NULL_HANDLE;
  block->prefetch_queue_family = prefetch_owner != 0u ? services.transfer_queue_family_index : UINT32_MAX;
  block->prefetch_command_pool = prefetch_owner != 0u ? services.transfer_command_pool : VK_NULL_HANDLE;
  block->active_weight_window = 0u;
  block->prefetch_state = PROM_MODEL_WEIGHT_WINDOW_EMPTY;
  block->model_contract_identity = request->model_contract_identity;
  block->assembly_family = request->assembly_family;
  block->parameter_set = request->parameter_set;
  block->parameter_set_aggregate_identity = request->parameter_set_aggregate_identity;
  block->binding_generation = (request->assembly_family == PROM_NOISE_REFINER_FAMILY_Z_IMAGE_TURBO ||
                               request->assembly_family == PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO ||
                               request->assembly_family == PROM_MAIN_TRANSFORMER_FAMILY_Z_IMAGE_TURBO) ? 1u : 0u;
  block->binding_state = (request->assembly_family == PROM_NOISE_REFINER_FAMILY_Z_IMAGE_TURBO ||
                          request->assembly_family == PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO ||
                          request->assembly_family == PROM_MAIN_TRANSFORMER_FAMILY_Z_IMAGE_TURBO)
                             ? PROM_NOISE_REFINER_BINDING_BOUND : 0u;
  block->weight_identity = request->weight_identity;
  block->shader_portfolio_identity = request->shader_portfolio_identity;
  block->precision_policy_identity = request->precision_policy_identity;
  block->capability_route_identity = request->capability_route_identity;
  block->memory_ceiling_bytes = request->memory_ceiling_bytes;
  block->external_input_bytes = request->external_input_bytes;
  block->external_output_bytes = request->external_output_bytes;
  block->declared_audit_bytes = request->audit_bytes;
  block->shader_id = request->shader_id;
  block->weight_count = request->weight_count;
  block->step_count = request->step_count;
  memcpy(block->steps, request->steps, sizeof(block->steps));
  for (index = 0u; index < block->weight_count; ++index) {
    block->weights[index].content_identity = request->weights[index].content_identity;
    block->weights[index].layout_identity = request->weights[index].layout_identity;
    block->weights[index].byte_count = request->weights[index].byte_count;
    if (block->weights[index].byte_count > max_weight_bytes) max_weight_bytes = block->weights[index].byte_count;
  }
  if (shared_owner != 0u) {
    if (!prom_model_block_create_shared_buffers(state, block, (VkDeviceSize)max_weight_bytes)) goto fail;
  } else if (block->shader_id == PROM_MODEL_BLOCK_M1B_ADALN_SHADER_ID) {
    if (!prom_model_block_m1b_create_buffers(state, block, (VkDeviceSize)max_weight_bytes)) goto fail;
  } else if (prom_model_block_is_main_transformer(block)) {
    if (!prom_main_transformer_create_buffers(state, block, (VkDeviceSize)max_weight_bytes)) goto fail;
  } else if (prom_model_block_is_context_refiner(block)) {
    if (!prom_context_refiner_create_buffers(state, block, (VkDeviceSize)max_weight_bytes)) goto fail;
  } else if (!prom_model_block_create_buffer(state, block, &block->input_upload, (VkDeviceSize)block->external_input_bytes,
                                      VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                                      VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1) ||
      !prom_model_block_create_buffer(state, block, &block->input_device, (VkDeviceSize)block->external_input_bytes,
                                      VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                                      VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_model_block_create_buffer(state, block, &block->output_device, (VkDeviceSize)block->external_output_bytes,
                                      VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                                      VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_model_block_create_buffer(state, block, &block->output_readback, (VkDeviceSize)block->external_output_bytes,
                                      VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                                      VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1) ||
      !prom_model_block_create_buffer(state, block, &block->audit_device, (VkDeviceSize)block->declared_audit_bytes,
                                      VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                                      VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_model_block_create_buffer(state, block, &block->audit_readback, (VkDeviceSize)block->declared_audit_bytes,
                                      VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                                      VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1) ||
      !prom_model_block_create_buffer(state, block, &block->weight_upload, (VkDeviceSize)max_weight_bytes,
                                      VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                                      VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1)) goto fail;
  for (index = 0u; index < block->weight_count; ++index) {
    if ((prefetch_owner != 0u
             ? prom_vk_create_buffer_shared_between_families(
                   state->physical_device, state->device, block->test_flags,
                   (VkDeviceSize)block->weights[index].byte_count,
                   VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                   VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, services.compute_queue_family_index,
                   services.transfer_queue_family_index, &block->weights[index].device)
             : prom_model_block_create_buffer(state, block, &block->weights[index].device,
                                              (VkDeviceSize)block->weights[index].byte_count,
                                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) !=
        (prefetch_owner != 0u ? VK_SUCCESS : 1)) goto fail;
  }
  if (prefetch_owner != 0u) {
    VkCommandBufferAllocateInfo prefetch_command_info;
    VkFenceCreateInfo prefetch_fence_info;
    if (!prom_model_block_create_buffer(state, block, &block->prefetch_weight_upload, (VkDeviceSize)max_weight_bytes,
                                        VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                                        VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1)) goto fail;
    for (index = 0u; index < PROM_MODEL_BLOCK_MAX_WEIGHTS; ++index) {
      if (prom_vk_create_buffer_shared_between_families(
              state->physical_device, state->device, block->test_flags, block->weights[index].device.size,
              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_DST_BIT,
              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, services.compute_queue_family_index,
              services.transfer_queue_family_index, &block->prefetch_weights[index].device) != VK_SUCCESS) goto fail;
    }
    memset(&prefetch_command_info, 0, sizeof(prefetch_command_info));
    prefetch_command_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO;
    prefetch_command_info.commandPool = block->prefetch_command_pool;
    prefetch_command_info.level = VK_COMMAND_BUFFER_LEVEL_PRIMARY;
    prefetch_command_info.commandBufferCount = 1u;
    if (vkAllocateCommandBuffers(state->device, &prefetch_command_info, &block->prefetch_command_buffer) != VK_SUCCESS) goto fail;
    memset(&prefetch_fence_info, 0, sizeof(prefetch_fence_info));
    prefetch_fence_info.sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO;
    prefetch_fence_info.flags = VK_FENCE_CREATE_SIGNALED_BIT;
    if (vkCreateFence(state->device, &prefetch_fence_info, NULL, &block->prefetch_fence) != VK_SUCCESS) goto fail;
  }
  if (shared_owner != 0u) {
    if (!prom_model_block_create_shared_portfolios(state, block)) goto pipeline_fail;
  } else if (block->shader_id == PROM_MODEL_BLOCK_M1B_ADALN_SHADER_ID) {
    if (!prom_model_block_m1b_create_pipelines(state, block) ||
        !prom_model_block_m1c_create_pipelines(state, block) ||
        !prom_model_block_m1d_create_pipelines(state, block) ||
        !prom_model_block_audit_create_pipelines(state, block)) goto pipeline_fail;
  } else if (prom_model_block_is_main_transformer(block)) {
    if (!prom_main_transformer_create_pipelines(state, block)) goto pipeline_fail;
  } else if (prom_model_block_is_context_refiner(block)) {
    if (!prom_context_refiner_create_pipelines(state, block) ||
        !prom_model_block_audit_create_pipelines(state, block)) goto pipeline_fail;
  } else {
    if (!prom_model_block_create_descriptor_resources(state, block)) goto pipeline_fail;
    if (!prom_model_block_create_pipeline(state, block)) goto pipeline_fail;
  }
  if (!prom_model_block_create_command_resources(state, block)) goto fail;
  block->block_id = block->next_block_id++;
  block->created = 1u;
  block->shared_owner = shared_owner;
  block->owner_construction_count = shared_owner != 0u ? 1u : 0u;
  block->execution_plan_identity = prom_model_block_plan_identity(block);
  block->last_detail_code = 0;
  *out_block_id = block->block_id;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;

pipeline_fail:
  detail = block->shader_id == PROM_MODEL_BLOCK_M1B_ADALN_SHADER_ID &&
                   block->m1b_pipelines[0u].pipeline.pipeline == VK_NULL_HANDLE
               ? PROM_MODEL_BLOCK_DETAIL_INGRESS_PIPELINE_CREATE_FAILED
               : PROM_MODEL_BLOCK_DETAIL_PIPELINE_CREATE_FAILED;
  goto fail;
fail:
  prom_model_block_cleanup_state(state);
  prom_model_block_fill_evidence(NULL, detail == 0 ? PROM_MODEL_BLOCK_DETAIL_RESOURCE_CREATE_FAILED : detail, out_evidence);
  return PROM_ERROR;
}

int prom_reactor_runtime_model_block_upload_weights_impl(
    void* handle, uint64_t block_id, const PrometheusModelBlockWeightUpload* uploads,
    uint32_t upload_count, PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  uint32_t index;
  if (!prom_reactor_runtime_validate_handle(handle) || uploads == NULL) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  if (block->weights_uploaded != 0u || block->quarantined != 0u || upload_count != block->weight_count ||
      (block->test_flags & PROM_TESTCFG_FAIL_UPLOAD) != 0u) {
    prom_model_block_mark_failure(block, (block->test_flags & PROM_TESTCFG_FAIL_UPLOAD) != 0u
                                             ? PROM_MODEL_BLOCK_DETAIL_UPLOAD_FAILED
                                             : PROM_MODEL_BLOCK_DETAIL_WEIGHT_MISMATCH);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  for (index = 0u; index < upload_count; ++index) {
    const PrometheusModelBlockWeightUpload* upload = &uploads[index];
    if (upload->binding_index != index || upload->bytes == NULL ||
        upload->byte_count != block->weights[index].byte_count ||
        upload->content_identity != block->weights[index].content_identity ||
        upload->layout_identity != block->weights[index].layout_identity ||
        upload->byte_count > (uint64_t)block->weight_upload.size) {
      prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_WEIGHT_MISMATCH);
      prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
      return PROM_ERROR;
    }
  }
  for (index = 0u; index < upload_count; ++index) {
    memcpy(block->weight_upload.mapped, uploads[index].bytes, (size_t)uploads[index].byte_count);
    if (!prom_model_block_record_upload(state, block, block->weights, index)) {
      prom_model_block_mark_failure(block, block->quarantined != 0u
                                               ? PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN
                                               : PROM_MODEL_BLOCK_DETAIL_UPLOAD_FAILED);
      prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
      return PROM_ERROR;
    }
    block->weights[index].uploaded = 1u;
    block->weight_upload_count += 1u;
  }
  block->weights_uploaded = 1u;
  block->last_detail_code = 0;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
}

static void prom_model_block_destroy_pending_weights(prom_reduction_runtime_state* state,
                                                     prom_model_block_state* block) {
  uint32_t index;
  if (state == NULL || block == NULL || state->device == VK_NULL_HANDLE) return;
  for (index = 0u; index < PROM_MODEL_BLOCK_MAX_WEIGHTS; ++index) {
    prom_vk_destroy_buffer(state->device, &block->pending_weights[index].device);
  }
  memset(block->pending_weights, 0, sizeof(block->pending_weights));
}

int prom_reactor_runtime_noise_refiner_rebind_impl(
    void* handle, uint64_t block_id, const PrometheusNoiseRefinerRebindRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  prom_model_block_weight_resource old_weights[PROM_MODEL_BLOCK_MAX_WEIGHTS];
  const PrometheusNoiseRefinerResolvedDescriptor* descriptor;
  uint32_t index;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL ||
      request->struct_size != sizeof(*request) || request->uploads == NULL ||
      request->upload_count != PROM_MODEL_BLOCK_MAX_WEIGHTS) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  descriptor = prom_zimage_turbo_resolve_noise_refiner_descriptor(request->lock_identity, request->model_local_block_id);
  if (descriptor == NULL || descriptor->assembly_family != PROM_NOISE_REFINER_FAMILY_Z_IMAGE_TURBO ||
      descriptor->parameter_set != PROM_NOISE_REFINER_PARAMETER_SET_1 ||
      descriptor->parameter_set_aggregate_identity != prom_noise_refiner_expected_aggregate(descriptor->parameter_set)) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  if (block->assembly_family != PROM_NOISE_REFINER_FAMILY_Z_IMAGE_TURBO ||
      block->parameter_set != PROM_NOISE_REFINER_PARAMETER_SET_0 || block->weights_uploaded == 0u ||
      block->quarantined != 0u ||
      (block->binding_state != PROM_NOISE_REFINER_BINDING_BOUND &&
       block->binding_state != PROM_NOISE_REFINER_BINDING_FAILED_BEFORE_COMMIT) ||
      !prom_model_block_reap(state, block)) {
    block->binding_state = block->quarantined != 0u ? PROM_NOISE_REFINER_BINDING_QUARANTINED
                                                     : PROM_NOISE_REFINER_BINDING_COMPLETION_UNCERTAIN;
    prom_model_block_mark_failure(block, block->quarantined != 0u ? PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN
                                                                   : PROM_MODEL_BLOCK_DETAIL_REBIND_FAILED);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  block->binding_state = PROM_NOISE_REFINER_BINDING_VALIDATING;
  for (index = 0u; index < request->upload_count; ++index) {
    const PrometheusModelBlockWeightUpload* upload = &request->uploads[index];
    if (upload->binding_index != index || upload->bytes == NULL || upload->content_identity == 0u ||
        upload->layout_identity == 0u || upload->byte_count != k_prom_model_block_m1b_weight_bytes[index] ||
        upload->byte_count > (uint64_t)block->weight_upload.size) {
      block->binding_state = PROM_NOISE_REFINER_BINDING_FAILED_BEFORE_COMMIT;
      block->last_detail_code = PROM_MODEL_BLOCK_DETAIL_WEIGHT_MISMATCH;
      prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
      return PROM_ERROR;
    }
  }
  prom_model_block_destroy_pending_weights(state, block);
  for (index = 0u; index < request->upload_count; ++index) {
    const PrometheusModelBlockWeightUpload* upload = &request->uploads[index];
    block->pending_weights[index].content_identity = upload->content_identity;
    block->pending_weights[index].layout_identity = upload->layout_identity;
    block->pending_weights[index].byte_count = upload->byte_count;
    if (!prom_model_block_create_buffer(state, block, &block->pending_weights[index].device,
                                        (VkDeviceSize)upload->byte_count,
                                        VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                                        VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) {
      prom_model_block_destroy_pending_weights(state, block);
      block->binding_state = PROM_NOISE_REFINER_BINDING_FAILED_BEFORE_COMMIT;
      block->last_detail_code = PROM_MODEL_BLOCK_DETAIL_RESOURCE_CREATE_FAILED;
      prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
      return PROM_ERROR;
    }
  }
  block->binding_state = PROM_NOISE_REFINER_BINDING_UPLOADING;
  for (index = 0u; index < request->upload_count; ++index) {
    memcpy(block->weight_upload.mapped, request->uploads[index].bytes, (size_t)request->uploads[index].byte_count);
    if (!prom_model_block_record_upload(state, block, block->pending_weights, index)) {
      prom_model_block_destroy_pending_weights(state, block);
      block->binding_state = block->quarantined != 0u ? PROM_NOISE_REFINER_BINDING_QUARANTINED
                                                       : PROM_NOISE_REFINER_BINDING_FAILED_BEFORE_COMMIT;
      prom_model_block_mark_failure(block, block->quarantined != 0u ? PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN
                                                                     : PROM_MODEL_BLOCK_DETAIL_UPLOAD_FAILED);
      prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
      return PROM_ERROR;
    }
    block->pending_weights[index].uploaded = 1u;
  }
  block->binding_state = PROM_NOISE_REFINER_BINDING_UPDATING_DESCRIPTORS;
  if (!prom_model_block_update_weight_descriptors(state, block, block->pending_weights)) {
    prom_model_block_destroy_pending_weights(state, block);
    block->binding_state = PROM_NOISE_REFINER_BINDING_FAILED_BEFORE_COMMIT;
    block->last_detail_code = PROM_MODEL_BLOCK_DETAIL_DESCRIPTOR_UPDATE_FAILED;
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  block->binding_state = PROM_NOISE_REFINER_BINDING_READY_TO_COMMIT;
  memcpy(old_weights, block->weights, sizeof(old_weights));
  memcpy(block->weights, block->pending_weights, sizeof(block->weights));
  memset(block->pending_weights, 0, sizeof(block->pending_weights));
  for (index = 0u; index < PROM_MODEL_BLOCK_MAX_WEIGHTS; ++index) {
    prom_vk_destroy_buffer(state->device, &old_weights[index].device);
  }
  block->parameter_set = descriptor->parameter_set;
  block->parameter_set_aggregate_identity = descriptor->parameter_set_aggregate_identity;
  block->binding_generation += 1u;
  block->descriptor_generation += 1u;
  block->output_valid = 0u;
  block->audit_valid = 0u;
  block->resident_input_generation = 0u;
  block->replay_identity = 0u;
  block->m1b_prefix_replay_identity = 0u;
  block->binding_state = PROM_NOISE_REFINER_BINDING_BOUND;
  block->weight_upload_count += request->upload_count;
  block->last_detail_code = 0;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
}

int prom_reactor_runtime_model_block_execute_impl(
    void* handle, uint64_t block_id, const PrometheusModelBlockExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  uint64_t input_bytes;
  uint64_t audit_bytes;
  uint64_t begin_ns;
  uint32_t audit_elements;
  int32_t execution_detail;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL ||
      request->struct_size != sizeof(*request) || request->input == NULL || request->output == NULL ||
      request->input_identity == 0u || request->audit_enabled > 1u ||
      !prom_model_block_bytes_for_elements(request->element_count, &input_bytes)) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  if (block->weights_uploaded == 0u || input_bytes != block->external_input_bytes ||
      input_bytes != block->external_output_bytes || request->element_count > 4194304u) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_STALE_OUTPUT);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  if (!prom_model_block_reap(state, block)) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  audit_elements = (uint32_t)(block->declared_audit_bytes / sizeof(float));
  audit_bytes = request->audit_element_capacity * sizeof(float);
  if (request->audit_enabled != 0u && (request->audit_output == NULL || audit_bytes < block->declared_audit_bytes)) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_AUDIT_FAILED);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  block->output_valid = 0u;
  block->audit_valid = 0u;
  memcpy(block->input_upload.mapped, request->input, (size_t)input_bytes);
  begin_ns = prom_reduction_now_ns();
  if (!prom_model_block_record_execute(state, block, (uint32_t)request->element_count, audit_elements,
                                       request->audit_enabled, &execution_detail)) {
    prom_model_block_mark_failure(block, execution_detail);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  if ((block->test_flags & PROM_TESTCFG_FAIL_DOWNLOAD) != 0u) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_AUDIT_FAILED);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  memcpy(request->output, block->output_readback.mapped, (size_t)block->external_output_bytes);
  block->output_valid = 1u;
  if (request->audit_enabled != 0u) {
    memcpy(request->audit_output, block->audit_readback.mapped, (size_t)block->declared_audit_bytes);
    block->audit_valid = 1u;
  }
  block->execution_count += 1u;
  block->last_execution_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  block->replay_identity = prom_model_block_hash_u64(block->execution_plan_identity, request->input_identity);
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, request->audit_enabled);
  block->last_detail_code = 0;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
}

int prom_reactor_runtime_model_block_execute_m1b_impl(
    void* handle, uint64_t block_id, const PrometheusModelBlockM1BExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  VkDeviceSize audit_copy_bytes = 0u;
  uint64_t audit_bytes;
  uint64_t begin_ns;
  int32_t execution_detail;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL ||
      request->struct_size != sizeof(*request) || request->model_input_bf16 == NULL ||
      request->timestep_bf16 == NULL || request->input_identity == 0u ||
      request->timestep_identity == 0u || request->audit_enabled > 1u ||
      (request->audit_enabled == 0u && request->audit_stage != PROM_MODEL_BLOCK_M1B_AUDIT_NONE) ||
      (request->audit_enabled != 0u &&
       (request->audit_stage < PROM_MODEL_BLOCK_M1B_AUDIT_INGRESS_INPUT ||
        request->audit_stage > PROM_MODEL_BLOCK_M1B_AUDIT_POSITIONED_K))) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  if (request->model_input_bytes != PROM_MODEL_BLOCK_M1B_BF16_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS)) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_INGRESS_INPUT_SIZE_MISMATCH);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  if (request->timestep_bytes != PROM_MODEL_BLOCK_M1B_BF16_BYTES(PROM_MODEL_BLOCK_M1B_TIMESTEP_ELEMENTS)) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_INGRESS_TIMESTEP_SIZE_MISMATCH);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  if (block->shader_id != PROM_MODEL_BLOCK_M1B_ADALN_SHADER_ID || block->weights_uploaded == 0u) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_STALE_OUTPUT);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  if (!prom_model_block_reap(state, block)) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  if (request->audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_INGRESS_INPUT) {
    audit_copy_bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS);
  } else if (request->audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_INGRESS_TIMESTEP) {
    audit_copy_bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_TIMESTEP_ELEMENTS);
  } else if (request->audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_ADALN_PROJECTION) {
    audit_copy_bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(15360u);
  } else if (request->audit_stage >= PROM_MODEL_BLOCK_M1B_AUDIT_ATTENTION_SCALE_RAW &&
             request->audit_stage <= PROM_MODEL_BLOCK_M1B_AUDIT_MLP_GATE_TANH) {
    audit_copy_bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS);
  } else if (request->audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_FUSED_QKV) {
    audit_copy_bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_QKV_ELEMENTS);
  } else if (request->audit_stage >= PROM_MODEL_BLOCK_M1B_AUDIT_ATTENTION_NORM) {
    audit_copy_bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS);
  }
  audit_bytes = request->audit_element_capacity * sizeof(float);
  if (request->audit_enabled != 0u &&
      (request->audit_output == NULL || audit_bytes < audit_copy_bytes ||
       audit_copy_bytes > block->declared_audit_bytes)) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_AUDIT_FAILED);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  block->output_valid = 0u;
  block->audit_valid = 0u;
  memcpy(block->input_upload.mapped, request->model_input_bf16, (size_t)block->input_upload.size);
  memcpy(block->timestep_upload.mapped, request->timestep_bf16, (size_t)block->timestep_upload.size);
  begin_ns = prom_reduction_now_ns();
  if (!prom_model_block_m1b_record_execute(state, block, request->audit_stage, 0,
                                           &execution_detail)) {
    prom_model_block_mark_failure(block, execution_detail);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  if (block->m1b_timestamp_supported != 0u &&
      block->m1b_timestamp_query_pool != VK_NULL_HANDLE) {
    uint64_t timestamps[PROM_MODEL_BLOCK_M1B_PIPELINE_COUNT + 1u];
    uint32_t boundary;
    block->gpu_compute_ns = 0u;
    if (vkGetQueryPoolResults(state->device, block->m1b_timestamp_query_pool, 0u,
                              PROM_MODEL_BLOCK_M1B_PIPELINE_COUNT + 1u,
                              sizeof(timestamps), timestamps, sizeof(uint64_t),
                              VK_QUERY_RESULT_64_BIT | VK_QUERY_RESULT_WAIT_BIT) != VK_SUCCESS) {
      prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_AUDIT_FAILED);
      prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
      return PROM_ERROR;
    }
    for (boundary = 0u; boundary < PROM_MODEL_BLOCK_M1B_PIPELINE_COUNT; ++boundary) {
      block->m1b_boundary_gpu_ns[boundary] = (uint64_t)(
          (double)(timestamps[boundary + 1u] - timestamps[boundary]) *
              (double)block->m1b_timestamp_period_ns + 0.5);
      block->gpu_compute_ns += block->m1b_boundary_gpu_ns[boundary];
    }
  }
  if ((block->test_flags & PROM_TESTCFG_FAIL_DOWNLOAD) != 0u) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_AUDIT_FAILED);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  if (request->audit_enabled != 0u) {
    memcpy(request->audit_output, block->audit_readback.mapped, (size_t)audit_copy_bytes);
    block->audit_valid = 1u;
  }
  /* The QKV buffer owns the resident Q, K, and V segments.  No host output or
     intermediate activation copy is part of this M1B ingress. */
  block->output_valid = 1u;
  block->execution_count += 1u;
  block->last_execution_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  block->replay_identity = prom_model_block_hash_u64(block->execution_plan_identity, request->input_identity);
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, request->timestep_identity);
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, request->audit_enabled);
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, request->audit_stage);
  block->m1b_prefix_replay_identity = block->replay_identity;
  block->last_detail_code = 0;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
}

static int prom_model_block_m1c_record_execute(prom_reduction_runtime_state* state,
                                                prom_model_block_state* block,
                                                uint32_t audit_stage,
                                                int capture_audit,
                                                int32_t* out_detail) {
  VkCommandBufferBeginInfo begin_info;
  prom_model_block_m1c_attention_constants attention_constants = {1024u, 30u, 128u, 11520u};
  prom_model_block_m1b_qkv_constants projection_constants = {1024u, 3840u, 3840u, 0u};
  prom_model_block_m1b_norm_constants residual_constants = {1.0e-5f, 1024u, 3840u, 0u};
  if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_COMMAND_RECORD_FAILED;
  if (state == NULL || block == NULL ||
      prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_COMMAND_RECORD)) return 0;
  if (vkResetCommandBuffer(block->command_buffer, 0u) != VK_SUCCESS) return 0;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(block->command_buffer, &begin_info) != VK_SUCCESS) return 0;
  if (block->m1b_timestamp_supported != 0u) {
    vkCmdResetQueryPool(block->command_buffer, block->m1b_timestamp_query_pool, 0u, 2u);
    vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                        block->m1b_timestamp_query_pool, 0u);
  }
  if ((block->test_flags & PROM_TESTCFG_FAIL_DISPATCH) != 0u) {
    if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_INGRESS_DISPATCH_FAILED;
    return 0;
  }
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1c_pipelines[0u],
                                         &attention_constants, sizeof(attention_constants), 30720u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1c_pipelines[1u],
                                         &projection_constants, sizeof(projection_constants), 64u, 240u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1c_pipelines[2u],
                                         &residual_constants, sizeof(residual_constants), 1024u, 1u, 1u);
  if (capture_audit != 0) {
    const prom_vk_buffer* audit_source = &block->attention_residual;
    if (audit_stage == PROM_MODEL_BLOCK_M1C_AUDIT_ATTENTION) audit_source = &block->attention;
    if (audit_stage == PROM_MODEL_BLOCK_M1C_AUDIT_PROJECTION) audit_source = &block->attention_projection;
    prom_model_block_m1b_record_audit_capture(block->command_buffer, audit_source, 0u,
        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS), 0, &block->audit_readback);
    prom_model_block_record_small_audit_capture(block->command_buffer, &block->audit_device,
        PROM_MODEL_BLOCK_M1C_TRANSIENT_AUDIT_FLOATS * sizeof(float), &block->audit_readback,
        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS));
  }
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(
      block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
      block->m1b_timestamp_query_pool, 1u);
  if (vkEndCommandBuffer(block->command_buffer) != VK_SUCCESS) return 0;
  return prom_model_block_submit_and_wait(state, block, out_detail);
}

int prom_reactor_runtime_model_block_execute_m1c_impl(
    void* handle, uint64_t block_id, const PrometheusModelBlockM1CExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  uint64_t required_elements = PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS;
  uint64_t begin_ns;
  int32_t execution_detail;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL ||
      request->struct_size != sizeof(*request) || request->output == NULL || request->output_identity == 0u ||
      request->m1b_prefix_replay_identity == 0u || request->output_element_capacity < required_elements ||
      request->audit_stage < PROM_MODEL_BLOCK_M1C_AUDIT_ATTENTION ||
      request->audit_stage > PROM_MODEL_BLOCK_M1C_AUDIT_RESIDUAL) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  if (block->shader_id != PROM_MODEL_BLOCK_M1B_ADALN_SHADER_ID || block->weights_uploaded == 0u ||
      block->m1b_prefix_replay_identity != request->m1b_prefix_replay_identity ||
      block->audit_readback.size < PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS)) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_STALE_OUTPUT);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  if (!prom_model_block_reap(state, block)) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  block->output_valid = 0u;
  block->audit_valid = 0u;
  block->resident_input_generation = 0u;
  begin_ns = prom_reduction_now_ns();
  if (!prom_model_block_m1c_record_execute(state, block, request->audit_stage, 1, &execution_detail)) {
    prom_model_block_mark_failure(block, execution_detail);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  if ((block->test_flags & PROM_TESTCFG_FAIL_DOWNLOAD) != 0u) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_AUDIT_FAILED);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  const float* transient_audit = (const float*)((const uint8_t*)block->audit_readback.mapped +
      PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS));
  if (transient_audit[30u] != 1.0f || transient_audit[62u] != 1.0f) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_M1C_SOFTMAX_INVALID);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  if (request->transient_audit != NULL &&
      request->transient_audit_element_capacity >= PROM_MODEL_BLOCK_M1C_TRANSIENT_AUDIT_FLOATS) {
    memcpy(request->transient_audit, transient_audit,
           PROM_MODEL_BLOCK_M1C_TRANSIENT_AUDIT_FLOATS * sizeof(float));
  }
  memcpy(request->output, block->audit_readback.mapped,
         (size_t)PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS));
  block->output_valid = 1u;
  block->execution_count += 1u;
  block->last_execution_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  block->replay_identity = prom_model_block_hash_u64(block->m1b_prefix_replay_identity, request->output_identity);
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, request->audit_stage);
  block->last_detail_code = 0;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
}

static int prom_model_block_m1d_record_execute(prom_reduction_runtime_state* state,
                                                prom_model_block_state* block,
                                                uint32_t audit_stage,
                                                int capture_audit,
                                                int32_t* out_detail) {
  VkCommandBufferBeginInfo begin_info;
  prom_model_block_m1b_norm_constants norm_constants = {1.0e-5f, 1024u, 3840u, 0u};
  prom_model_block_m1b_qkv_constants projection_constants = {1024u, 3840u, 10240u, 0u};
  prom_model_block_m1d_gate_constants gate_constants = {PROM_MODEL_BLOCK_M1D_HIDDEN_ELEMENTS, 0u, 0u, 0u};
  prom_model_block_m1d_w2_constants w2_constants = {1.0e-5f, 1024u, 3840u, 10240u};
  if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_COMMAND_RECORD_FAILED;
  if (state == NULL || block == NULL ||
      prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_COMMAND_RECORD)) return 0;
  if (vkResetCommandBuffer(block->command_buffer, 0u) != VK_SUCCESS) return 0;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(block->command_buffer, &begin_info) != VK_SUCCESS) return 0;
  if (block->m1b_timestamp_supported != 0u) {
    vkCmdResetQueryPool(block->command_buffer, block->m1b_timestamp_query_pool, 0u, 2u);
    vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                        block->m1b_timestamp_query_pool, 0u);
  }
  if ((block->test_flags & PROM_TESTCFG_FAIL_DISPATCH) != 0u) {
    if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_INGRESS_DISPATCH_FAILED;
    return 0;
  }
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[0u],
                                         &norm_constants, sizeof(norm_constants), 1024u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (capture_audit != 0 && audit_stage == PROM_MODEL_BLOCK_M1D_AUDIT_FFN_NORM) {
    prom_model_block_m1b_record_audit_capture(block->command_buffer, &block->norm_audit, 0u,
        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS), 0, &block->audit_readback);
  }
  if (capture_audit != 0 && audit_stage == PROM_MODEL_BLOCK_M1D_AUDIT_FFN_MODULATED) {
    prom_model_block_m1b_record_audit_capture(block->command_buffer, &block->modulated, 0u,
        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS), 0, &block->audit_readback);
  }
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[1u],
                                         &projection_constants, sizeof(projection_constants), 64u, 640u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (capture_audit != 0 && audit_stage == PROM_MODEL_BLOCK_M1D_AUDIT_W1) {
    prom_model_block_m1b_record_audit_capture(block->command_buffer, &block->qkv, 0u,
        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1D_HIDDEN_ELEMENTS), 0, &block->audit_readback);
  }
  if (capture_audit != 0 && audit_stage == PROM_MODEL_BLOCK_M1D_AUDIT_W3) {
    prom_model_block_record_small_audit_capture(block->command_buffer, &block->attention,
        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS), &block->audit_readback, 0u);
    prom_model_block_record_small_audit_capture(block->command_buffer, &block->attention_projection,
        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS), &block->audit_readback,
        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS));
    prom_model_block_record_small_audit_capture(block->command_buffer, &block->norm_audit,
        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1D_HIDDEN_ELEMENTS - 2u * PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS),
        &block->audit_readback, 2u * PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS));
  }
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[2u],
                                         &gate_constants, sizeof(gate_constants), 40960u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (capture_audit != 0 && audit_stage == PROM_MODEL_BLOCK_M1D_AUDIT_GATED_HIDDEN) {
    prom_model_block_m1b_record_audit_capture(block->command_buffer, &block->qkv, 0u,
        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1D_HIDDEN_ELEMENTS), 0, &block->audit_readback);
  }
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[3u],
                                         &w2_constants, sizeof(w2_constants), 1024u, 1u, 1u);
  if (capture_audit != 0 && audit_stage == PROM_MODEL_BLOCK_M1D_AUDIT_W2) {
    prom_model_block_m1b_record_audit_capture(block->command_buffer, &block->input_device, 0u,
        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS), 0, &block->audit_readback);
  }
  if (capture_audit != 0 && audit_stage == PROM_MODEL_BLOCK_M1D_AUDIT_FINAL_OUTPUT) {
    prom_model_block_m1b_record_audit_capture(block->command_buffer, &block->attention, 0u,
        PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS), 0, &block->audit_readback);
  }
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(
      block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
      block->m1b_timestamp_query_pool, 1u);
  if (vkEndCommandBuffer(block->command_buffer) != VK_SUCCESS) return 0;
  return prom_model_block_submit_and_wait(state, block, out_detail);
}

int prom_reactor_runtime_model_block_execute_m1d_impl(
    void* handle, uint64_t block_id, const PrometheusModelBlockM1DExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  uint64_t required_elements;
  uint64_t output_bytes;
  uint64_t begin_ns;
  int32_t execution_detail;
  required_elements = (request != NULL &&
                       (request->audit_stage == PROM_MODEL_BLOCK_M1D_AUDIT_W1 ||
                        request->audit_stage == PROM_MODEL_BLOCK_M1D_AUDIT_W3 ||
                        request->audit_stage == PROM_MODEL_BLOCK_M1D_AUDIT_GATED_HIDDEN))
                          ? PROM_MODEL_BLOCK_M1D_HIDDEN_ELEMENTS
                          : PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS;
  output_bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(required_elements);
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL ||
      request->struct_size != sizeof(*request) || request->output == NULL || request->output_identity == 0u ||
      request->m1c_prefix_replay_identity == 0u ||
      request->output_element_capacity < required_elements ||
      request->audit_stage < PROM_MODEL_BLOCK_M1D_AUDIT_FFN_NORM ||
      request->audit_stage > PROM_MODEL_BLOCK_M1D_AUDIT_FINAL_OUTPUT) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  if (block->shader_id != PROM_MODEL_BLOCK_M1B_ADALN_SHADER_ID || block->weights_uploaded == 0u ||
      block->replay_identity != request->m1c_prefix_replay_identity ||
      block->audit_readback.size < output_bytes) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_STALE_OUTPUT);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  if (!prom_model_block_reap(state, block)) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  block->output_valid = 0u;
  block->audit_valid = 0u;
  begin_ns = prom_reduction_now_ns();
  if (!prom_model_block_m1d_record_execute(state, block, request->audit_stage, 1, &execution_detail)) {
    prom_model_block_mark_failure(block, execution_detail);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  if ((block->test_flags & PROM_TESTCFG_FAIL_DOWNLOAD) != 0u) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_AUDIT_FAILED);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  memcpy(request->output, block->audit_readback.mapped, (size_t)output_bytes);
  block->output_valid = 1u;
  block->audit_valid = 1u;
  block->execution_count += 1u;
  block->last_execution_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  block->replay_identity = prom_model_block_hash_u64(request->m1c_prefix_replay_identity, request->output_identity);
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, request->audit_stage);
  block->last_detail_code = 0;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
}

/* This facade deliberately shares the established M1B ingress implementation,
   then records M1C/M1D without audit captures or host copies. The three
   resident transitions remain the implementation authority; this is only the
   narrow model-specific callable assembly. */
static int prom_reactor_runtime_noise_refiner_execute_external_impl(
    void* handle, uint64_t block_id, const PrometheusNoiseRefiner0ExecuteRequest* request,
    uint32_t required_parameter_set, PrometheusModelBlockEvidence* out_evidence) {
  PrometheusModelBlockM1BExecuteRequest m1b;
  PrometheusModelBlockEvidence ignored_evidence;
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  uint64_t begin_ns;
  int32_t execution_detail;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL ||
      request->struct_size != sizeof(*request) || request->model_input_bf16 == NULL ||
      request->timestep_bf16 == NULL || request->model_input_bytes !=
          PROM_MODEL_BLOCK_M1B_BF16_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS) ||
      request->timestep_bytes != PROM_MODEL_BLOCK_M1B_BF16_BYTES(PROM_MODEL_BLOCK_M1B_TIMESTEP_ELEMENTS) ||
      request->input_identity == 0u || request->timestep_identity == 0u ||
      request->output_identity == 0u || request->audit_enabled != 0u) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  memset(&m1b, 0, sizeof(m1b));
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  block = state == NULL ? NULL : &state->model_block;
  if (block == NULL || block->created == 0u || block->block_id != block_id ||
      block->assembly_family != PROM_NOISE_REFINER_FAMILY_Z_IMAGE_TURBO ||
      block->parameter_set != required_parameter_set || block->binding_state != PROM_NOISE_REFINER_BINDING_BOUND) {
    prom_model_block_fill_evidence(block, PROM_MODEL_BLOCK_DETAIL_PARAMETER_SET_MISMATCH, out_evidence);
    return PROM_ERROR;
  }
  /* An external BF16 ingress starts a new chain; it cannot reuse a prior
     resident predecessor identity. */
  block->resident_input_generation = 0u;
  m1b.struct_size = sizeof(m1b);
  m1b.model_input_bf16 = request->model_input_bf16;
  m1b.timestep_bf16 = request->timestep_bf16;
  m1b.model_input_bytes = request->model_input_bytes;
  m1b.timestep_bytes = request->timestep_bytes;
  m1b.input_identity = request->input_identity;
  m1b.timestep_identity = request->timestep_identity;
  m1b.audit_enabled = 0u;
  m1b.audit_stage = PROM_MODEL_BLOCK_M1B_AUDIT_NONE;
  if (prom_reactor_runtime_model_block_execute_m1b_impl(handle, block_id, &m1b,
                                                        &ignored_evidence) != PROM_OK) {
    if (out_evidence != NULL) *out_evidence = ignored_evidence;
    return PROM_ERROR;
  }
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  block = state == NULL ? NULL : &state->model_block;
  if (block == NULL || block->m1b_prefix_replay_identity == 0u || !prom_model_block_reap(state, block)) {
    if (block != NULL) prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN);
    prom_model_block_fill_evidence(block, block == NULL ? PROM_MODEL_BLOCK_DETAIL_NOT_FOUND : block->last_detail_code,
                                  out_evidence);
    return PROM_ERROR;
  }
  block->output_valid = 0u;
  block->audit_valid = 0u;
  begin_ns = prom_reduction_now_ns();
  if (!prom_model_block_m1c_record_execute(state, block, PROM_MODEL_BLOCK_M1C_AUDIT_RESIDUAL, 0,
                                            &execution_detail)) {
    prom_model_block_mark_failure(block, execution_detail);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  block->replay_identity = prom_model_block_hash_u64(block->m1b_prefix_replay_identity, request->output_identity);
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, PROM_MODEL_BLOCK_M1C_AUDIT_RESIDUAL);
  if (!prom_model_block_reap(state, block)) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  if (!prom_model_block_m1d_record_execute(state, block, PROM_MODEL_BLOCK_M1D_AUDIT_FINAL_OUTPUT, 0,
                                            &execution_detail)) {
    prom_model_block_mark_failure(block, execution_detail);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  block->output_valid = 1u;
  block->output_generation += 1u;
  block->audit_valid = 0u;
  block->execution_count += 2u;
  block->last_execution_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, request->output_identity);
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, PROM_MODEL_BLOCK_M1D_AUDIT_FINAL_OUTPUT);
  block->last_detail_code = 0;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
}

int prom_reactor_runtime_noise_refiner0_execute_impl(
    void* handle, uint64_t block_id, const PrometheusNoiseRefiner0ExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_noise_refiner_execute_external_impl(
      handle, block_id, request, PROM_NOISE_REFINER_PARAMETER_SET_0, out_evidence);
}

int prom_reactor_runtime_noise_refiner1_execute_impl(
    void* handle, uint64_t block_id, const PrometheusNoiseRefiner1ExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_noise_refiner_execute_external_impl(
      handle, block_id, request, PROM_NOISE_REFINER_PARAMETER_SET_1, out_evidence);
}

int prom_reactor_runtime_context_refiner_create_impl(
    void* handle, const PrometheusContextRefinerCreateRequest* request, uint64_t* out_block_id,
    PrometheusModelBlockEvidence* out_evidence) {
  const PrometheusContextRefinerResolvedDescriptor* descriptor;
  PrometheusModelBlockCreateRequest closed;
  uint32_t index;
#if defined(PROMETHEUS_DVT2_M5B_SUBGROUP_OWNED_EXPERIMENT)
  prom_vk_runtime_services services;
#endif
  if (out_block_id != NULL) *out_block_id = 0u;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL || out_block_id == NULL ||
      request->struct_size != sizeof(*request) || request->uploads == NULL || request->upload_count != 11u) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
#if defined(PROMETHEUS_DVT2_M5B_SUBGROUP_OWNED_EXPERIMENT)
  if (prom_reactor_runtime_get_vk_services(handle, &services) != PROM_OK ||
      prom_vk_subgroup_owned_attention_admission_reason(&services) != NULL) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_RESOURCE_CREATE_FAILED, out_evidence);
    return PROM_ERROR;
  }
#endif
  descriptor = prom_zimage_turbo_resolve_context_refiner_descriptor(request->lock_identity, request->model_local_block_id);
  if (descriptor == NULL || descriptor->assembly_family != PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO ||
      descriptor->parameter_set != PROM_CONTEXT_REFINER_PARAMETER_SET_0 ||
      descriptor->parameter_set_aggregate_identity != prom_context_refiner_expected_aggregate(descriptor->parameter_set)) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  memset(&closed, 0, sizeof(closed));
  closed.struct_size = (uint32_t)sizeof(closed);
  closed.assembly_family = descriptor->assembly_family;
  closed.parameter_set = descriptor->parameter_set;
  closed.parameter_set_aggregate_identity = descriptor->parameter_set_aggregate_identity;
  closed.model_contract_identity = descriptor->internal_abi_identity == 0u ? descriptor->lock_identity : descriptor->internal_abi_identity;
  closed.weight_identity = descriptor->parameter_set_aggregate_identity;
  closed.shader_portfolio_identity = descriptor->execution_plan_identity;
  closed.precision_policy_identity = descriptor->precision_policy_identity == 0u ? descriptor->lock_identity : descriptor->precision_policy_identity;
  closed.capability_route_identity = descriptor->canonical_authority_identity == 0u
                                        ? descriptor->lock_identity : descriptor->canonical_authority_identity;
  closed.memory_ceiling_bytes = descriptor->memory_plan_identity;
  closed.external_input_bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_CONTEXT_MODEL_ELEMENTS);
  closed.audit_bytes = PROM_ZIMAGE_TURBO_AUDIT_ARENA_BYTES;
  closed.shader_id = PROM_MODEL_BLOCK_M1B_NORM_SHADER_ID;
  closed.weight_count = 11u;
  closed.step_count = PROM_MODEL_BLOCK_MAX_STEPS;
  memcpy(closed.steps, k_prom_model_block_steps, sizeof(closed.steps));
  for (index = 0u; index < closed.weight_count; ++index) {
    closed.weights[index].content_identity = request->uploads[index].content_identity;
    closed.weights[index].layout_identity = request->uploads[index].layout_identity;
    closed.weights[index].byte_count = request->uploads[index].byte_count;
  }
  if (prom_reactor_runtime_model_block_create_impl(handle, &closed, out_block_id, out_evidence) != PROM_OK) return PROM_ERROR;
  if (prom_reactor_runtime_model_block_upload_weights_impl(handle, *out_block_id, request->uploads,
                                                           request->upload_count, out_evidence) != PROM_OK) {
    /* A facade create is atomic: a rejected initial payload may not leave an
       unbound ContextRefiner owner behind for a later mixed-family request. */
    prom_reactor_runtime_model_block_destroy_impl(handle, *out_block_id);
    *out_block_id = 0u;
    return PROM_ERROR;
  }
  return PROM_OK;
}

int prom_reactor_runtime_main_transformer_create_impl(
    void* handle, const PrometheusMainTransformerCreateRequest* request, uint64_t* out_block_id,
    PrometheusModelBlockEvidence* out_evidence) {
  const PrometheusMainTransformerResolvedDescriptor* descriptor;
  PrometheusModelBlockCreateRequest closed;
  uint32_t index;
#if defined(PROMETHEUS_DVT2_M5B_SUBGROUP_OWNED_EXPERIMENT) || defined(PROMETHEUS_DVT2_M5B_GEMINI_EXACT_EXPERIMENT) || defined(PROMETHEUS_DVT2_M5B_GEMINI_INPLACE_EXPERIMENT)
  prom_vk_runtime_services services;
#endif
  if (out_block_id != NULL) *out_block_id = 0u;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL || out_block_id == NULL ||
      request->struct_size != sizeof(*request) || request->uploads == NULL ||
      request->upload_count != PROM_MODEL_BLOCK_MAX_WEIGHTS) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
#if defined(PROMETHEUS_DVT2_M5B_SUBGROUP_OWNED_EXPERIMENT) || defined(PROMETHEUS_DVT2_M5B_GEMINI_EXACT_EXPERIMENT) || defined(PROMETHEUS_DVT2_M5B_GEMINI_INPLACE_EXPERIMENT)
  if (prom_reactor_runtime_get_vk_services(handle, &services) != PROM_OK ||
      prom_vk_subgroup_owned_attention_admission_reason(&services) != NULL) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_RESOURCE_CREATE_FAILED, out_evidence);
    return PROM_ERROR;
  }
#endif
  descriptor = prom_zimage_turbo_resolve_main_transformer_descriptor(request->lock_identity,
                                                                     request->model_local_block_id);
  if (descriptor == NULL ||
      descriptor->assembly_family != PROM_MAIN_TRANSFORMER_FAMILY_Z_IMAGE_TURBO ||
      descriptor->parameter_set != descriptor->model_local_block_id + 1u ||
      descriptor->parameter_set_aggregate_identity != prom_main_transformer_expected_aggregate(descriptor->parameter_set) ||
      descriptor->prepared_image_role != PROM_ZIMAGE_STREAM_PREPARED_IMAGE ||
      descriptor->prepared_context_role != PROM_ZIMAGE_STREAM_PREPARED_CONTEXT ||
      descriptor->joint_working_role != PROM_ZIMAGE_STREAM_JOINT_WORKING ||
      descriptor->image_token_count != PROM_MODEL_BLOCK_MAIN_IMAGE_TOKENS ||
      descriptor->context_token_count != PROM_MODEL_BLOCK_CONTEXT_TOKENS ||
      descriptor->joint_token_count != PROM_MODEL_BLOCK_MAIN_TOKENS ||
      descriptor->hidden_width != PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  memset(&closed, 0, sizeof(closed));
  closed.struct_size = (uint32_t)sizeof(closed);
  closed.assembly_family = descriptor->assembly_family;
  closed.parameter_set = descriptor->parameter_set;
  closed.parameter_set_aggregate_identity = descriptor->parameter_set_aggregate_identity;
  closed.model_contract_identity = descriptor->internal_abi_identity;
  closed.weight_identity = descriptor->parameter_set_aggregate_identity;
  closed.shader_portfolio_identity = descriptor->execution_plan_identity;
  closed.precision_policy_identity = descriptor->precision_policy_identity;
  closed.capability_route_identity = descriptor->canonical_authority_identity;
  closed.memory_ceiling_bytes = descriptor->memory_plan_identity;
  closed.external_input_bytes = 0u;
  closed.external_output_bytes = 0u;
  closed.audit_bytes = PROM_ZIMAGE_TURBO_AUDIT_ARENA_BYTES;
  closed.shader_id = PROM_MODEL_BLOCK_MAIN_QK_ROPE_SHADER_ID;
  closed.weight_count = PROM_MODEL_BLOCK_MAX_WEIGHTS;
  closed.step_count = PROM_MODEL_BLOCK_MAX_STEPS;
  memcpy(closed.steps, k_prom_model_block_steps, sizeof(closed.steps));
  for (index = 0u; index < closed.weight_count; ++index) {
    closed.weights[index].content_identity = request->uploads[index].content_identity;
    closed.weights[index].layout_identity = request->uploads[index].layout_identity;
    closed.weights[index].byte_count = request->uploads[index].byte_count;
  }
  if (prom_reactor_runtime_model_block_create_impl(handle, &closed, out_block_id, out_evidence) != PROM_OK) return PROM_ERROR;
  if (prom_reactor_runtime_model_block_upload_weights_impl(handle, *out_block_id, request->uploads,
                                                           request->upload_count, out_evidence) != PROM_OK) {
    prom_reactor_runtime_model_block_destroy_impl(handle, *out_block_id);
    *out_block_id = 0u;
    return PROM_ERROR;
  }
  return PROM_OK;
}

int prom_reactor_runtime_context_refiner_rebind_impl(
    void* handle, uint64_t block_id, const PrometheusContextRefinerRebindRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  prom_model_block_weight_resource old_weights[PROM_MODEL_BLOCK_MAX_WEIGHTS];
  const PrometheusContextRefinerResolvedDescriptor* descriptor;
  uint32_t index;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL || request->uploads == NULL ||
      request->struct_size != sizeof(*request) || request->upload_count != 11u) goto invalid;
  descriptor = prom_zimage_turbo_resolve_context_refiner_descriptor(request->lock_identity, request->model_local_block_id);
  if (descriptor == NULL || descriptor->assembly_family != PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO ||
      descriptor->parameter_set != PROM_CONTEXT_REFINER_PARAMETER_SET_1 ||
      descriptor->parameter_set_aggregate_identity != prom_context_refiner_expected_aggregate(descriptor->parameter_set)) goto invalid;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  if (!prom_model_block_is_context_refiner(block) || block->parameter_set != PROM_CONTEXT_REFINER_PARAMETER_SET_0 ||
      block->weights_uploaded == 0u || block->quarantined != 0u ||
      (block->binding_state != PROM_NOISE_REFINER_BINDING_BOUND &&
       block->binding_state != PROM_NOISE_REFINER_BINDING_FAILED_BEFORE_COMMIT) || !prom_model_block_reap(state, block)) goto failed;
  block->binding_state = PROM_NOISE_REFINER_BINDING_VALIDATING;
  for (index = 0u; index < 11u; ++index) {
    const PrometheusModelBlockWeightUpload* upload = &request->uploads[index];
    if (upload->binding_index != index || upload->bytes == NULL || upload->content_identity == 0u ||
        upload->layout_identity == 0u || upload->byte_count != k_prom_model_block_context_weight_bytes[index] ||
        upload->byte_count > (uint64_t)block->weight_upload.size) goto mismatch;
  }
  prom_model_block_destroy_pending_weights(state, block);
  for (index = 0u; index < 11u; ++index) {
    const PrometheusModelBlockWeightUpload* upload = &request->uploads[index];
    block->pending_weights[index].content_identity = upload->content_identity;
    block->pending_weights[index].layout_identity = upload->layout_identity;
    block->pending_weights[index].byte_count = upload->byte_count;
    if (!prom_model_block_create_buffer(state, block, &block->pending_weights[index].device,
                                        (VkDeviceSize)upload->byte_count,
                                        VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                                        VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) goto resource_fail;
  }
  block->binding_state = PROM_NOISE_REFINER_BINDING_UPLOADING;
  for (index = 0u; index < 11u; ++index) {
    memcpy(block->weight_upload.mapped, request->uploads[index].bytes, (size_t)request->uploads[index].byte_count);
    if (!prom_model_block_record_upload(state, block, block->pending_weights, index)) goto upload_fail;
    block->pending_weights[index].uploaded = 1u;
  }
  block->binding_state = PROM_NOISE_REFINER_BINDING_UPDATING_DESCRIPTORS;
  if (!prom_model_block_update_weight_descriptors(state, block, block->pending_weights)) goto descriptor_fail;
  block->binding_state = PROM_NOISE_REFINER_BINDING_READY_TO_COMMIT;
  memcpy(old_weights, block->weights, sizeof(old_weights));
  memcpy(block->weights, block->pending_weights, sizeof(block->weights));
  memset(block->pending_weights, 0, sizeof(block->pending_weights));
  for (index = 0u; index < PROM_MODEL_BLOCK_MAX_WEIGHTS; ++index) prom_vk_destroy_buffer(state->device, &old_weights[index].device);
  block->parameter_set = descriptor->parameter_set;
  block->parameter_set_aggregate_identity = descriptor->parameter_set_aggregate_identity;
  block->binding_generation += 1u;
  block->descriptor_generation += 1u;
  block->output_valid = 0u;
  block->audit_valid = 0u;
  block->resident_input_generation = 0u;
  block->replay_identity = 0u;
  block->binding_state = PROM_NOISE_REFINER_BINDING_BOUND;
  block->weight_upload_count += 11u;
  block->last_detail_code = 0;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
mismatch:
  block->last_detail_code = PROM_MODEL_BLOCK_DETAIL_WEIGHT_MISMATCH;
  goto rebind_fail;
resource_fail:
  block->last_detail_code = PROM_MODEL_BLOCK_DETAIL_RESOURCE_CREATE_FAILED;
  goto rebind_fail;
upload_fail:
  block->last_detail_code = block->quarantined != 0u ? PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN : PROM_MODEL_BLOCK_DETAIL_UPLOAD_FAILED;
  goto rebind_fail;
descriptor_fail:
  block->last_detail_code = PROM_MODEL_BLOCK_DETAIL_DESCRIPTOR_UPDATE_FAILED;
rebind_fail:
  prom_model_block_destroy_pending_weights(state, block);
  block->binding_state = block->quarantined != 0u ? PROM_NOISE_REFINER_BINDING_QUARANTINED : PROM_NOISE_REFINER_BINDING_FAILED_BEFORE_COMMIT;
  prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
  return PROM_ERROR;
failed:
  prom_model_block_mark_failure(block, block->quarantined != 0u ? PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN : PROM_MODEL_BLOCK_DETAIL_REBIND_FAILED);
  prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
  return PROM_ERROR;
invalid:
  prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
  return PROM_ERROR;
}

static int prom_context_refiner_execute_closed(
    void* handle, uint64_t block_id, uint32_t required_parameter_set, const float* input,
    uint64_t input_bytes, uint64_t input_generation, uint64_t input_identity,
    uint64_t output_identity, uint32_t audit_enabled, int resident_input,
    PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  uint64_t begin_ns;
  uint32_t output_generation;
  int first_resident_execution;
  int resident_input_mode;
  int32_t detail;
  if (!prom_reactor_runtime_validate_handle(handle) || output_identity == 0u || audit_enabled != 0u ||
      (!resident_input && (input == NULL || input_identity == 0u ||
                           input_bytes != PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_CONTEXT_MODEL_ELEMENTS)))) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  first_resident_execution = resident_input && block->resident_input_generation == 0u;
  resident_input_mode = resident_input ? (first_resident_execution ? 1 : 2) : 0;
  if (!prom_model_block_is_context_refiner(block) || block->parameter_set != required_parameter_set ||
      block->weights_uploaded == 0u || block->quarantined != 0u ||
      block->binding_state != PROM_NOISE_REFINER_BINDING_BOUND || !prom_model_block_reap(state, block) ||
      (resident_input && (input_generation == 0u ||
                          (first_resident_execution ? input_generation != block->output_generation
                                                    : input_generation != block->resident_input_generation)))) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_STALE_OUTPUT);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  output_generation = (uint32_t)(block->output_generation + 1u);
  if (output_generation == 0u) {
    prom_model_block_fill_evidence(block, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  block->output_valid = 0u;
  block->audit_valid = 0u;
  if (!resident_input) memcpy(block->input_upload.mapped, input, (size_t)input_bytes);
  begin_ns = prom_reduction_now_ns();
  if (!prom_context_refiner_record_execute(state, block, resident_input_mode, &detail)) {
    prom_model_block_mark_failure(block, detail);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  block->gpu_compute_ns = prom_model_block_resolve_gpu_span(state, block, 1u);
  block->gpu_total_ns = block->gpu_compute_ns;
  block->output_valid = 1u;
  block->output_generation = output_generation;
  if (first_resident_execution) block->resident_input_generation = input_generation;
  block->execution_count += 1u;
  block->last_execution_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  block->replay_identity = prom_model_block_hash_u64(block->execution_plan_identity,
                                                     resident_input ? input_generation : input_identity);
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, output_identity);
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, block->parameter_set_aggregate_identity);
  block->last_detail_code = 0;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
}

int prom_reactor_runtime_context_refiner0_execute_impl(
    void* handle, uint64_t block_id, const PrometheusContextRefiner0ExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  if (request == NULL || request->struct_size != sizeof(*request)) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  return prom_context_refiner_execute_closed(handle, block_id, PROM_CONTEXT_REFINER_PARAMETER_SET_0,
                                             request->context_input, request->context_input_bytes, 0u,
                                             request->input_identity, request->output_identity,
                                             request->audit_enabled, 0, out_evidence);
}

int prom_reactor_runtime_context_refiner_execute_resident_impl(
    void* handle, uint64_t block_id, const PrometheusContextRefinerResidentExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  if (request == NULL || request->struct_size != sizeof(*request)) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  return prom_context_refiner_execute_closed(handle, block_id, PROM_CONTEXT_REFINER_PARAMETER_SET_1,
                                             NULL, 0u, request->input_generation, 0u,
                                             request->output_identity, request->audit_enabled, 1, out_evidence);
}

int prom_reactor_runtime_context_refiner_audit_final_impl(
    void* handle, uint64_t block_id, const PrometheusContextRefinerFinalAuditRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  VkCommandBufferBeginInfo begin_info;
  VkBufferMemoryBarrier barrier;
  VkBufferCopy copy;
  const uint64_t output_bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_CONTEXT_MODEL_ELEMENTS);
  int32_t detail = PROM_MODEL_BLOCK_DETAIL_AUDIT_FAILED;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL || request->output == NULL ||
      request->struct_size != sizeof(*request) || request->output_identity == 0u ||
      request->required_output_generation == 0u || request->output_element_capacity < PROM_MODEL_BLOCK_CONTEXT_MODEL_ELEMENTS) goto invalid;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  if (!prom_model_block_is_context_refiner(block) || block->output_valid == 0u ||
      block->output_generation != request->required_output_generation || block->audit_readback.size < output_bytes ||
      !prom_model_block_reap(state, block)) goto stale;
  if (vkResetCommandBuffer(block->command_buffer, 0u) != VK_SUCCESS) goto failure;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(block->command_buffer, &begin_info) != VK_SUCCESS) goto failure;
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = block->attention.buffer;
  barrier.size = output_bytes;
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  memset(&copy, 0, sizeof(copy));
  copy.size = output_bytes;
  vkCmdCopyBuffer(block->command_buffer, block->attention.buffer, block->audit_readback.buffer, 1u, &copy);
  if (vkEndCommandBuffer(block->command_buffer) != VK_SUCCESS ||
      !prom_model_block_submit_and_wait(state, block, &detail)) goto failure;
  memcpy(request->output, block->audit_readback.mapped, (size_t)output_bytes);
  block->audit_valid = 1u;
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, request->output_identity);
  block->last_detail_code = 0;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
stale:
  prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_STALE_OUTPUT);
  prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
  return PROM_ERROR;
failure:
  prom_model_block_mark_failure(block, detail == 0 ? PROM_MODEL_BLOCK_DETAIL_AUDIT_FAILED : detail);
  prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
  return PROM_ERROR;
invalid:
  prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
  return PROM_ERROR;
}

int prom_reactor_runtime_context_refiner_execute_static_audit_impl(
    void* handle, uint64_t block_id, const PrometheusContextRefinerStaticAuditRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  uint64_t begin_ns;
  uint32_t execution_generation;
  int first_resident_execution;
  int resident_input_mode;
  int32_t detail = PROM_MODEL_BLOCK_DETAIL_COMMAND_RECORD_FAILED;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL ||
      request->struct_size != sizeof(*request) || request->lock_identity != PROM_ZIMAGE_TURBO_AUDIT_LOCK_ID ||
      request->input_generation == 0u || request->output_identity == 0u || request->audit_arena == NULL ||
      request->audit_arena_capacity_bytes < PROM_ZIMAGE_TURBO_CONTEXT_AUDIT_ARENA_BYTES) goto invalid;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  first_resident_execution = block->parameter_set == PROM_CONTEXT_REFINER_PARAMETER_SET_1 &&
                             block->resident_input_generation == 0u;
  resident_input_mode = block->parameter_set == PROM_CONTEXT_REFINER_PARAMETER_SET_1 ?
      (first_resident_execution ? 1 : 2) : 0;
  if (!prom_model_block_is_context_refiner(block) || block->weights_uploaded == 0u ||
      block->binding_state != PROM_NOISE_REFINER_BINDING_BOUND || block->quarantined != 0u ||
      ((block->parameter_set == PROM_CONTEXT_REFINER_PARAMETER_SET_0 &&
        request->input_generation != block->output_generation) ||
       (block->parameter_set == PROM_CONTEXT_REFINER_PARAMETER_SET_1 &&
        (request->input_generation == 0u ||
         (first_resident_execution ? request->input_generation != block->output_generation
                                   : request->input_generation != block->resident_input_generation)))) ||
      !prom_model_block_reap(state, block)) goto stale;
  execution_generation = (uint32_t)(block->output_generation + 1u);
  if (execution_generation == 0u) goto invalid;
  block->output_valid = 0u;
  block->audit_valid = 0u;
  begin_ns = prom_reduction_now_ns();
  if (!prom_context_refiner_record_static_audit(state, block, execution_generation,
                                                resident_input_mode, &detail)) goto failure;
  memset(request->audit_arena, 0, (size_t)request->audit_arena_capacity_bytes);
  memcpy(request->audit_arena, block->audit_readback.mapped,
         PROM_ZIMAGE_TURBO_CONTEXT_AUDIT_REQUIRED_BYTES);
  block->output_valid = 1u;
  block->audit_valid = 1u;
  if (first_resident_execution) block->resident_input_generation = request->input_generation;
  block->output_generation = execution_generation;
  block->execution_count += 1u;
  block->last_execution_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, request->output_identity);
  block->last_detail_code = 0;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
stale:
  prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_STALE_OUTPUT);
  prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
  return PROM_ERROR;
failure:
  prom_model_block_mark_failure(block, detail);
  prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
  return PROM_ERROR;
invalid:
  prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
  return PROM_ERROR;
}

static int prom_context_refiner_record_execute(prom_reduction_runtime_state* state,
                                               prom_model_block_state* block, int resident_input,
                                               int32_t* out_detail) {
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  VkBufferMemoryBarrier barrier;
  prom_model_block_m1b_norm_constants norm = {1.0e-5f, PROM_MODEL_BLOCK_CONTEXT_TOKENS, 3840u, 0u};
  prom_model_block_m1b_qkv_constants qkv = {PROM_MODEL_BLOCK_CONTEXT_TOKENS, 3840u, 11520u, 0u};
  prom_model_block_context_qk_constants q_head = {1.0e-5f, PROM_MODEL_BLOCK_CONTEXT_TOKENS, 30u, 128u, 0u};
  prom_model_block_context_qk_constants k_head = {1.0e-5f, PROM_MODEL_BLOCK_CONTEXT_TOKENS, 30u, 128u, 3840u};
  prom_model_block_m1c_attention_constants attention = {PROM_MODEL_BLOCK_CONTEXT_TOKENS, 30u, 128u, 11520u};
  prom_model_block_m1b_qkv_constants projection = {PROM_MODEL_BLOCK_CONTEXT_TOKENS, 3840u, 3840u, 0u};
  prom_model_block_m1b_qkv_constants hidden = {PROM_MODEL_BLOCK_CONTEXT_TOKENS, 3840u, 10240u, 0u};
  prom_model_block_m1d_gate_constants gate = {PROM_MODEL_BLOCK_CONTEXT_HIDDEN_ELEMENTS, 0u, 0u, 0u};
  prom_model_block_m1d_w2_constants w2 = {1.0e-5f, PROM_MODEL_BLOCK_CONTEXT_TOKENS, 3840u, 10240u};
  if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_COMMAND_RECORD_FAILED;
  if (state == NULL || block == NULL || !prom_model_block_is_context_refiner(block) ||
      prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_COMMAND_RECORD) ||
      vkResetCommandBuffer(block->command_buffer, 0u) != VK_SUCCESS) return 0;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(block->command_buffer, &begin_info) != VK_SUCCESS) return 0;
  if (block->m1b_timestamp_supported != 0u) {
    vkCmdResetQueryPool(block->command_buffer, block->m1b_timestamp_query_pool, 0u, 2u);
    vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                        block->m1b_timestamp_query_pool, 0u);
  }
  memset(&copy, 0, sizeof(copy));
  copy.size = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_CONTEXT_MODEL_ELEMENTS);
  if (resident_input == 1) {
    vkCmdCopyBuffer(block->command_buffer, block->attention.buffer, block->resident_boundary_device.buffer, 1u, &copy);
    memset(&barrier, 0, sizeof(barrier));
    barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barrier.srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
    barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
    barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barrier.buffer = block->resident_boundary_device.buffer;
    barrier.size = VK_WHOLE_SIZE;
    vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                         VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  }
  if (resident_input != 0) {
    vkCmdCopyBuffer(block->command_buffer, block->resident_boundary_device.buffer, block->input_device.buffer, 1u, &copy);
  } else {
    vkCmdCopyBuffer(block->command_buffer, block->input_upload.buffer, block->input_device.buffer, 1u, &copy);
  }
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = block->input_device.buffer;
  barrier.size = VK_WHOLE_SIZE;
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  if ((block->test_flags & PROM_TESTCFG_FAIL_DISPATCH) != 0u) {
    if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_INGRESS_DISPATCH_FAILED;
    return 0;
  }
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[0u], &norm, sizeof(norm), 32u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[1u], &qkv, sizeof(qkv), 2u, 720u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[2u], &q_head, sizeof(q_head), 960u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[3u], &k_head, sizeof(k_head), 960u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1c_pipelines[0u], &attention, sizeof(attention), 960u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1c_pipelines[1u], &projection, sizeof(projection), 2u, 240u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1c_pipelines[2u], &norm, sizeof(norm), 32u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[0u], &norm, sizeof(norm), 32u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[1u], &hidden, sizeof(hidden), 2u, 640u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[2u], &gate, sizeof(gate), 1280u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[3u], &w2, sizeof(w2), 32u, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(
      block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
      block->m1b_timestamp_query_pool, 1u);
  if (vkEndCommandBuffer(block->command_buffer) != VK_SUCCESS) return 0;
  return prom_model_block_submit_and_wait(state, block, out_detail);
}

static int prom_context_refiner_record_static_audit_entry(
    prom_model_block_state* block, uint32_t schedule_index, uint32_t execution_generation,
    uint32_t capture_point) {
  const prom_zimage_turbo_audit_schedule_entry* entry;
  const prom_vk_buffer* source;
  prom_model_block_audit_constants constants;
  VkBufferMemoryBarrier barrier;
  uint32_t key_index;
  if (block == NULL || schedule_index >= PROM_ZIMAGE_TURBO_CONTEXT_AUDIT_STAGE_COUNT) return 0;
  entry = &k_prom_zimage_turbo_context_audit_schedule[schedule_index];
  if (capture_point != entry->legal_capture_point || capture_point > entry->last_legal_lifetime_point ||
      entry->capture_policy != PROM_ZIMAGE_AUDIT_CAPTURE_PROJECTION_AND_SUMMARY) return 0;
  source = prom_model_block_audit_source(block, entry->source_resource);
  if (source == NULL || source->buffer == VK_NULL_HANDLE) return 0;
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = source->buffer;
  barrier.size = VK_WHOLE_SIZE;
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  memset(&constants, 0, sizeof(constants));
  constants.source_base_element = entry->source_base_element;
  constants.element_count = entry->element_count;
  constants.stage_id = entry->stage_id;
  constants.execution_generation = execution_generation;
  constants.part0_element_count = entry->element_count;
  if (entry->source_resource == PROM_ZIMAGE_AUDIT_SOURCE_QKV &&
      entry->layout_kind == PROM_ZIMAGE_AUDIT_LAYOUT_TOKEN_HEAD_CHANNEL) {
    constants.source_mode = 1u;
    constants.source_base_element =
        (entry->source_base_element / PROM_MODEL_BLOCK_CONTEXT_MODEL_ELEMENTS) * PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS;
  }
  constants.destination_word_offset = entry->audit_destination_offset / sizeof(uint32_t);
  constants.projection_key_count = entry->projection_key_count;
  for (key_index = 0u; key_index < entry->projection_key_count; ++key_index) {
    constants.projection_keys[key_index] =
        k_prom_zimage_turbo_context_audit_projection_keys[entry->projection_key_offset + key_index];
  }
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer,
                                         &block->audit_pipelines[entry->source_resource - 1u],
                                         &constants, sizeof(constants), 1u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  return 1;
}

static int prom_context_refiner_record_static_audit(
    prom_reduction_runtime_state* state, prom_model_block_state* block, uint32_t execution_generation,
    int resident_input, int32_t* out_detail) {
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  VkBufferMemoryBarrier barrier;
  prom_model_block_m1b_norm_constants norm = {1.0e-5f, PROM_MODEL_BLOCK_CONTEXT_TOKENS, 3840u, 0u};
  prom_model_block_m1b_qkv_constants qkv = {PROM_MODEL_BLOCK_CONTEXT_TOKENS, 3840u, 11520u, 0u};
  prom_model_block_context_qk_constants q_head = {1.0e-5f, PROM_MODEL_BLOCK_CONTEXT_TOKENS, 30u, 128u, 0u};
  prom_model_block_context_qk_constants k_head = {1.0e-5f, PROM_MODEL_BLOCK_CONTEXT_TOKENS, 30u, 128u, 3840u};
  prom_model_block_m1c_attention_constants attention = {PROM_MODEL_BLOCK_CONTEXT_TOKENS, 30u, 128u, 11520u};
  prom_model_block_m1b_qkv_constants projection = {PROM_MODEL_BLOCK_CONTEXT_TOKENS, 3840u, 3840u, 0u};
  prom_model_block_m1b_qkv_constants hidden = {PROM_MODEL_BLOCK_CONTEXT_TOKENS, 3840u, 10240u, 0u};
  prom_model_block_m1d_gate_constants gate = {PROM_MODEL_BLOCK_CONTEXT_HIDDEN_ELEMENTS, 0u, 0u, 0u};
  prom_model_block_m1d_w2_constants w2 = {1.0e-5f, PROM_MODEL_BLOCK_CONTEXT_TOKENS, 3840u, 10240u};
  if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_COMMAND_RECORD_FAILED;
  if (state == NULL || block == NULL || !prom_model_block_is_context_refiner(block) ||
      prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_COMMAND_RECORD) ||
      vkResetCommandBuffer(block->command_buffer, 0u) != VK_SUCCESS) return 0;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(block->command_buffer, &begin_info) != VK_SUCCESS) return 0;
  if (resident_input == 1) {
    memset(&copy, 0, sizeof(copy));
    copy.size = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_CONTEXT_MODEL_ELEMENTS);
    vkCmdCopyBuffer(block->command_buffer, block->attention.buffer, block->resident_boundary_device.buffer, 1u, &copy);
    memset(&barrier, 0, sizeof(barrier));
    barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barrier.srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
    barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
    barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barrier.buffer = block->resident_boundary_device.buffer;
    barrier.size = VK_WHOLE_SIZE;
    vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                         VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  }
  if (resident_input != 0) {
    memset(&copy, 0, sizeof(copy));
    copy.size = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_CONTEXT_MODEL_ELEMENTS);
    vkCmdCopyBuffer(block->command_buffer, block->resident_boundary_device.buffer, block->input_device.buffer, 1u, &copy);
    memset(&barrier, 0, sizeof(barrier));
    barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barrier.srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
    barrier.dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
    barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barrier.buffer = block->input_device.buffer;
    barrier.size = VK_WHOLE_SIZE;
    vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                         VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  }
  if (!prom_context_refiner_record_static_audit_entry(block, 0u, execution_generation, 1u)) return 0;
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[0u], &norm, sizeof(norm), 32u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_context_refiner_record_static_audit_entry(block, 1u, execution_generation, 2u)) return 0;
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[1u], &qkv, sizeof(qkv), 2u, 720u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_context_refiner_record_static_audit_entry(block, 2u, execution_generation, 3u)) return 0;
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[2u], &q_head, sizeof(q_head), 960u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_context_refiner_record_static_audit_entry(block, 3u, execution_generation, 4u) ||
      !prom_context_refiner_record_static_audit_entry(block, 5u, execution_generation, 4u)) return 0;
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[3u], &k_head, sizeof(k_head), 960u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_context_refiner_record_static_audit_entry(block, 4u, execution_generation, 5u) ||
      !prom_context_refiner_record_static_audit_entry(block, 6u, execution_generation, 5u)) return 0;
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1c_pipelines[0u], &attention, sizeof(attention), 960u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_context_refiner_record_static_audit_entry(block, 7u, execution_generation, 6u)) return 0;
  prom_model_block_record_small_audit_capture(block->command_buffer, &block->audit_device,
                                               PROM_ZIMAGE_TURBO_CONTEXT_AUDIT_TRANSIENT_ATTENTION_BYTES,
                                               &block->audit_readback,
                                               PROM_ZIMAGE_TURBO_CONTEXT_AUDIT_TRANSIENT_ATTENTION_OFFSET);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1c_pipelines[1u], &projection, sizeof(projection), 2u, 240u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_context_refiner_record_static_audit_entry(block, 8u, execution_generation, 7u)) return 0;
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1c_pipelines[2u], &norm, sizeof(norm), 32u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_context_refiner_record_static_audit_entry(block, 9u, execution_generation, 8u)) return 0;
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[0u], &norm, sizeof(norm), 32u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_context_refiner_record_static_audit_entry(block, 10u, execution_generation, 9u)) return 0;
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[1u], &hidden, sizeof(hidden), 2u, 640u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_context_refiner_record_static_audit_entry(block, 11u, execution_generation, 10u) ||
      !prom_context_refiner_record_static_audit_entry(block, 12u, execution_generation, 10u)) return 0;
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[2u], &gate, sizeof(gate), 1280u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_context_refiner_record_static_audit_entry(block, 13u, execution_generation, 11u)) return 0;
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[3u], &w2, sizeof(w2), 32u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_context_refiner_record_static_audit_entry(block, 14u, execution_generation, 12u) ||
      !prom_context_refiner_record_static_audit_entry(block, 15u, execution_generation, 13u)) return 0;
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT | VK_ACCESS_TRANSFER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_HOST_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = block->audit_readback.buffer;
  barrier.size = PROM_ZIMAGE_TURBO_CONTEXT_AUDIT_REQUIRED_BYTES;
  vkCmdPipelineBarrier(block->command_buffer,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT | VK_PIPELINE_STAGE_TRANSFER_BIT,
                       VK_PIPELINE_STAGE_HOST_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  if (vkEndCommandBuffer(block->command_buffer) != VK_SUCCESS) return 0;
  return prom_model_block_submit_and_wait(state, block, out_detail);
}

static int prom_context_refiner_create_buffers(prom_reduction_runtime_state* state,
                                               prom_model_block_state* block,
                                               VkDeviceSize max_weight_bytes) {
  const VkBufferUsageFlags device_storage = VK_BUFFER_USAGE_STORAGE_BUFFER_BIT |
                                            VK_BUFFER_USAGE_TRANSFER_DST_BIT |
                                            VK_BUFFER_USAGE_TRANSFER_SRC_BIT;
  const VkMemoryPropertyFlags host_visible = VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT |
                                              VK_MEMORY_PROPERTY_HOST_COHERENT_BIT;
  uint32_t index;
  if (!prom_model_block_create_buffer(state, block, &block->input_upload,
                                      PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_CONTEXT_MODEL_ELEMENTS),
                                      VK_BUFFER_USAGE_TRANSFER_SRC_BIT, host_visible, 1) ||
      !prom_model_block_create_buffer(state, block, &block->input_device,
                                      PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_CONTEXT_MODEL_ELEMENTS),
                                      device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_model_block_create_buffer(state, block, &block->resident_boundary_device,
                                      PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_CONTEXT_MODEL_ELEMENTS),
                                      device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_model_block_create_buffer(state, block, &block->context_unit,
                                      PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS),
                                      VK_BUFFER_USAGE_STORAGE_BUFFER_BIT, host_visible, 1) ||
      !prom_model_block_create_buffer(state, block, &block->modulated,
                                      PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_CONTEXT_MODEL_ELEMENTS),
                                      device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_model_block_create_buffer(state, block, &block->norm_audit,
                                      PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_CONTEXT_MODEL_ELEMENTS),
                                      device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_model_block_create_buffer(state, block, &block->qkv,
                                      PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_CONTEXT_QKV_ELEMENTS),
                                      device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_model_block_create_buffer(state, block, &block->attention,
                                      PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_CONTEXT_MODEL_ELEMENTS),
                                      device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_model_block_create_buffer(state, block, &block->attention_projection,
                                      PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_CONTEXT_MODEL_ELEMENTS),
                                      device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_model_block_create_buffer(state, block, &block->attention_residual,
                                      PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_CONTEXT_MODEL_ELEMENTS),
                                      device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_model_block_create_buffer(state, block, &block->context_w3,
                                      PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_CONTEXT_HIDDEN_ELEMENTS),
                                      device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_model_block_create_buffer(state, block, &block->audit_device,
                                      PROM_MODEL_BLOCK_M1C_TRANSIENT_AUDIT_FLOATS * sizeof(float),
                                      device_storage, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_model_block_create_buffer(state, block, &block->audit_readback, block->declared_audit_bytes,
                                      VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                      host_visible, 1) ||
      !prom_model_block_create_buffer(state, block, &block->weight_upload, max_weight_bytes,
                                      VK_BUFFER_USAGE_TRANSFER_SRC_BIT, host_visible, 1)) return 0;
  for (index = 0u; index < PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS; ++index) {
    ((float*)block->context_unit.mapped)[index] = 1.0f;
  }
  return 1;
}

static uint64_t prom_model_block_audit_source_elements(
    const prom_model_block_state* block, uint32_t source_resource) {
  const prom_vk_buffer* source = prom_model_block_audit_source(block, source_resource);
  if (source_resource == PROM_ZIMAGE_AUDIT_SOURCE_W3_DECLARED_VIEWS) {
    return PROM_MODEL_BLOCK_M1D_HIDDEN_ELEMENTS;
  }
  return source == NULL ? 0u : (uint64_t)source->size / sizeof(float);
}

static int prom_model_block_validate_static_audit_schedule(const prom_model_block_state* block) {
  uint64_t previous_end = 0u;
  uint32_t seen = 0u;
  uint32_t index;
  if (block == NULL || block->declared_audit_bytes != PROM_ZIMAGE_TURBO_AUDIT_ARENA_BYTES ||
      PROM_ZIMAGE_TURBO_AUDIT_REQUIRED_BYTES > PROM_ZIMAGE_TURBO_AUDIT_ARENA_BYTES) return 0;
  for (index = 0u; index < PROM_ZIMAGE_TURBO_AUDIT_STAGE_COUNT; ++index) {
    const prom_zimage_turbo_audit_schedule_entry* entry = &k_prom_zimage_turbo_audit_schedule[index];
    const prom_model_block_m1b_pipeline* pipeline;
    uint64_t source_end;
    uint64_t entry_bytes;
    uint64_t destination_end;
    uint32_t key_index;
    if (entry->authority_identity != PROM_ZIMAGE_TURBO_AUDIT_LOCK_ID ||
        entry->stage_id == 0u || entry->stage_id > PROM_ZIMAGE_TURBO_AUDIT_STAGE_COUNT ||
        (seen & (1u << (entry->stage_id - 1u))) != 0u ||
        entry->source_resource == 0u || entry->source_resource > PROM_MODEL_BLOCK_AUDIT_SOURCE_COUNT ||
        entry->projection_key_count > 15u || entry->legal_capture_point == 0u ||
        entry->last_legal_lifetime_point < entry->legal_capture_point ||
        (entry->audit_destination_offset & 255u) != 0u) return 0;
    seen |= 1u << (entry->stage_id - 1u);
    source_end = (uint64_t)entry->source_base_element + entry->element_count;
    if (source_end > prom_model_block_audit_source_elements(block, entry->source_resource)) return 0;
    if (entry->source_resource == PROM_ZIMAGE_AUDIT_SOURCE_W3_DECLARED_VIEWS &&
        (entry->source_base_element != 0u || entry->element_count != PROM_MODEL_BLOCK_M1D_HIDDEN_ELEMENTS)) return 0;
    entry_bytes = entry->capture_policy == PROM_ZIMAGE_AUDIT_CAPTURE_FULL
                      ? (uint64_t)entry->element_count * sizeof(float)
                      : 256u;
    destination_end = (uint64_t)entry->audit_destination_offset + entry_bytes;
    if (entry->audit_destination_offset < previous_end || destination_end > PROM_ZIMAGE_TURBO_AUDIT_REQUIRED_BYTES ||
        destination_end > block->audit_readback.size) return 0;
    previous_end = destination_end;
    for (key_index = 0u; key_index < entry->projection_key_count; ++key_index) {
      if (k_prom_zimage_turbo_audit_projection_keys[entry->projection_key_offset + key_index] >=
          entry->element_count) return 0;
    }
    pipeline = &block->audit_pipelines[entry->source_resource - 1u];
    if (entry->capture_policy != PROM_ZIMAGE_AUDIT_CAPTURE_FULL &&
        (pipeline->shader_id != PROM_MODEL_BLOCK_AUDIT_SUMMARY_SHADER_ID ||
         pipeline->binding_count != 4u ||
         pipeline->push_constant_bytes != sizeof(prom_model_block_audit_constants) ||
         pipeline->pipeline.pipeline == VK_NULL_HANDLE)) return 0;
  }
  return seen == ((1u << PROM_ZIMAGE_TURBO_AUDIT_STAGE_COUNT) - 1u);
}

static int prom_model_block_record_static_audit_entry(
    prom_model_block_state* block, uint32_t schedule_index, uint32_t execution_generation,
    uint32_t capture_point) {
  const prom_zimage_turbo_audit_schedule_entry* entry;
  const prom_vk_buffer* source;
  VkBufferMemoryBarrier barrier;
  if (block == NULL || schedule_index >= PROM_ZIMAGE_TURBO_AUDIT_STAGE_COUNT) return 0;
  entry = &k_prom_zimage_turbo_audit_schedule[schedule_index];
  if (capture_point != entry->legal_capture_point || capture_point > entry->last_legal_lifetime_point) return 0;
  source = prom_model_block_audit_source(block, entry->source_resource);
  if (source == NULL) return 0;
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
  barrier.dstAccessMask = entry->capture_policy == PROM_ZIMAGE_AUDIT_CAPTURE_FULL
                              ? VK_ACCESS_TRANSFER_READ_BIT : VK_ACCESS_SHADER_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = source->buffer;
  barrier.offset = (VkDeviceSize)entry->source_base_element * sizeof(float);
  barrier.size = (VkDeviceSize)entry->element_count * sizeof(float);
  if (entry->source_resource == PROM_ZIMAGE_AUDIT_SOURCE_W3_DECLARED_VIEWS) {
    barrier.offset = 0u;
    barrier.size = VK_WHOLE_SIZE;
  }
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       entry->capture_policy == PROM_ZIMAGE_AUDIT_CAPTURE_FULL
                           ? VK_PIPELINE_STAGE_TRANSFER_BIT : VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  if (entry->capture_policy == PROM_ZIMAGE_AUDIT_CAPTURE_FULL) {
    VkBufferCopy copy;
    memset(&copy, 0, sizeof(copy));
    copy.srcOffset = (VkDeviceSize)entry->source_base_element * sizeof(float);
    copy.dstOffset = entry->audit_destination_offset;
    copy.size = (VkDeviceSize)entry->element_count * sizeof(float);
    vkCmdCopyBuffer(block->command_buffer, source->buffer, block->audit_readback.buffer, 1u, &copy);
    barrier.srcAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
    barrier.dstAccessMask = VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT;
    vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                         VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  } else {
    prom_model_block_audit_constants constants;
    uint32_t key_index;
    memset(&constants, 0, sizeof(constants));
    constants.source_base_element = entry->source_base_element;
    constants.element_count = entry->element_count;
    constants.stage_id = entry->stage_id;
    constants.execution_generation = execution_generation;
    constants.part0_element_count = entry->element_count;
    if (entry->source_resource == PROM_ZIMAGE_AUDIT_SOURCE_QKV &&
        entry->layout_kind == PROM_ZIMAGE_AUDIT_LAYOUT_TOKEN_HEAD_CHANNEL) {
      constants.source_mode = 1u;
      constants.source_base_element =
          (entry->source_base_element / PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS) * PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS;
    }
    if (entry->source_resource == PROM_ZIMAGE_AUDIT_SOURCE_W3_DECLARED_VIEWS) {
      constants.source_base_element = 0u;
      constants.part0_element_count = PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS;
      constants.part1_element_count = PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS;
      constants.source_mode = 2u;
    }
    constants.destination_word_offset = entry->audit_destination_offset / sizeof(uint32_t);
    constants.projection_key_count = entry->projection_key_count;
    for (key_index = 0u; key_index < entry->projection_key_count; ++key_index) {
      constants.projection_keys[key_index] =
          k_prom_zimage_turbo_audit_projection_keys[entry->projection_key_offset + key_index];
    }
    prom_model_block_m1b_bind_and_dispatch(
        block->command_buffer, &block->audit_pipelines[entry->source_resource - 1u],
        &constants, sizeof(constants), 1u, 1u, 1u);
    prom_reduction_record_barrier(block->command_buffer);
  }
  return 1;
}

static int prom_model_block_record_static_audit_range(
    prom_model_block_state* block, uint32_t first_stage, uint32_t last_stage,
    uint32_t execution_generation, uint32_t capture_point) {
  uint32_t stage;
  for (stage = first_stage; stage <= last_stage; ++stage) {
    if (!prom_model_block_record_static_audit_entry(block, stage - 1u, execution_generation, capture_point)) return 0;
  }
  return 1;
}

static int prom_model_block_record_static_audit_batch(
    prom_reduction_runtime_state* state, prom_model_block_state* block,
    uint32_t execution_generation, int first_resident_execution, int32_t* out_detail) {
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  VkBufferMemoryBarrier barrier;
  prom_model_block_m1b_adaln_constants adaln = {15360u, 256u};
  prom_model_block_m1b_norm_constants norm = {1.0e-5f, 1024u, 3840u, 0u};
  prom_model_block_m1b_qkv_constants qkv = {1024u, 3840u, 11520u, 0u};
  prom_model_block_m1b_head_constants head = {1.0e-5f, 1024u, 30u, 128u};
  prom_model_block_m1c_attention_constants attention = {1024u, 30u, 128u, 11520u};
  prom_model_block_m1b_qkv_constants model_projection = {1024u, 3840u, 3840u, 0u};
  prom_model_block_m1d_gate_constants gate = {PROM_MODEL_BLOCK_M1D_HIDDEN_ELEMENTS, 0u, 0u, 0u};
  prom_model_block_m1b_qkv_constants hidden_projection = {1024u, 3840u, 10240u, 0u};
  prom_model_block_m1d_w2_constants w2 = {1.0e-5f, 1024u, 3840u, 10240u};
  if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_COMMAND_RECORD_FAILED;
  if (state == NULL || block == NULL || execution_generation == 0u ||
      prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_COMMAND_RECORD) ||
      vkResetCommandBuffer(block->command_buffer, 0u) != VK_SUCCESS) return 0;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(block->command_buffer, &begin_info) != VK_SUCCESS) return 0;
  memset(&copy, 0, sizeof(copy));
  copy.size = block->resident_boundary_device.size;
  if (first_resident_execution != 0) {
    vkCmdCopyBuffer(block->command_buffer, block->attention.buffer,
                    block->resident_boundary_device.buffer, 1u, &copy);
    memset(&barrier, 0, sizeof(barrier));
    barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barrier.srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
    barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
    barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barrier.buffer = block->resident_boundary_device.buffer;
    barrier.size = VK_WHOLE_SIZE;
    vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                         VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  }
  vkCmdCopyBuffer(block->command_buffer, block->resident_boundary_device.buffer,
                  block->input_device.buffer, 1u, &copy);
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = block->input_device.buffer;
  barrier.size = VK_WHOLE_SIZE;
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  if ((block->test_flags & PROM_TESTCFG_FAIL_DISPATCH) != 0u) {
    if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_INGRESS_DISPATCH_FAILED;
    return 0;
  }

  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[1u],
                                         &adaln, sizeof(adaln), 60u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_model_block_record_static_audit_range(block, 1u, 9u, execution_generation, 1u)) return 0;
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[2u],
                                         &norm, sizeof(norm), 1024u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_model_block_record_static_audit_range(block, 10u, 11u, execution_generation, 2u)) return 0;
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[3u],
                                         &qkv, sizeof(qkv), 64u, 720u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_model_block_record_static_audit_range(block, 12u, 15u, execution_generation, 3u)) return 0;
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[4u],
                                         &head, sizeof(head), 30720u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_model_block_record_static_audit_entry(block, 15u, execution_generation, 4u) ||
      !prom_model_block_record_static_audit_entry(block, 17u, execution_generation, 4u)) return 0;
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[5u],
                                         &head, sizeof(head), 30720u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_model_block_record_static_audit_entry(block, 16u, execution_generation, 5u) ||
      !prom_model_block_record_static_audit_entry(block, 18u, execution_generation, 5u)) return 0;

  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1c_pipelines[0u],
                                         &attention, sizeof(attention), 30720u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_model_block_record_static_audit_entry(block, 19u, execution_generation, 6u)) return 0;
  prom_model_block_record_small_audit_capture(
      block->command_buffer, &block->audit_device,
      PROM_ZIMAGE_TURBO_AUDIT_TRANSIENT_ATTENTION_BYTES, &block->audit_readback,
      PROM_ZIMAGE_TURBO_AUDIT_TRANSIENT_ATTENTION_OFFSET);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1c_pipelines[1u],
                                         &model_projection, sizeof(model_projection), 64u, 240u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_model_block_record_static_audit_entry(block, 20u, execution_generation, 7u)) return 0;
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1c_pipelines[2u],
                                         &norm, sizeof(norm), 1024u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_model_block_record_static_audit_entry(block, 21u, execution_generation, 8u)) return 0;

  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[0u],
                                         &norm, sizeof(norm), 1024u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_model_block_record_static_audit_range(block, 23u, 24u, execution_generation, 9u)) return 0;
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[1u],
                                         &hidden_projection, sizeof(hidden_projection), 64u, 640u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_model_block_record_static_audit_range(block, 25u, 26u, execution_generation, 10u)) return 0;
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[2u],
                                         &gate, sizeof(gate), 40960u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_model_block_record_static_audit_entry(block, 26u, execution_generation, 11u)) return 0;
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[3u],
                                         &w2, sizeof(w2), 1024u, 1u, 1u);
  prom_reduction_record_barrier(block->command_buffer);
  if (!prom_model_block_record_static_audit_entry(block, 27u, execution_generation, 12u) ||
      !prom_model_block_record_static_audit_entry(block, 28u, execution_generation, 13u)) return 0;

  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT | VK_ACCESS_TRANSFER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_HOST_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = block->audit_readback.buffer;
  barrier.size = PROM_ZIMAGE_TURBO_AUDIT_REQUIRED_BYTES;
  vkCmdPipelineBarrier(block->command_buffer,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT | VK_PIPELINE_STAGE_TRANSFER_BIT,
                       VK_PIPELINE_STAGE_HOST_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  if (vkEndCommandBuffer(block->command_buffer) != VK_SUCCESS) return 0;
  return prom_model_block_submit_and_wait(state, block, out_detail);
}

int prom_reactor_runtime_noise_refiner_execute_static_audit_impl(
    void* handle, uint64_t block_id, const PrometheusNoiseRefinerStaticAuditRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  uint64_t begin_ns;
  uint32_t execution_generation;
  int first_resident_execution;
  int32_t detail = PROM_MODEL_BLOCK_DETAIL_COMMAND_RECORD_FAILED;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL ||
      request->struct_size != sizeof(*request) || request->lock_identity != PROM_ZIMAGE_TURBO_AUDIT_LOCK_ID ||
      request->input_generation == 0u || request->output_identity == 0u || request->audit_arena == NULL ||
      request->audit_arena_capacity_bytes < PROM_ZIMAGE_TURBO_AUDIT_ARENA_BYTES) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  first_resident_execution = block->resident_input_generation == 0u;
  if (block->assembly_family != PROM_NOISE_REFINER_FAMILY_Z_IMAGE_TURBO ||
      block->parameter_set != PROM_NOISE_REFINER_PARAMETER_SET_1 ||
      block->binding_state != PROM_NOISE_REFINER_BINDING_BOUND || block->weights_uploaded == 0u ||
      (first_resident_execution ? request->input_generation != block->output_generation
                                : request->input_generation != block->resident_input_generation) ||
      !prom_model_block_reap(state, block) || !prom_model_block_validate_static_audit_schedule(block)) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_STALE_OUTPUT);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  execution_generation = (uint32_t)(block->output_generation + 1u);
  if (execution_generation == 0u) {
    prom_model_block_fill_evidence(block, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  block->output_valid = 0u;
  block->audit_valid = 0u;
  begin_ns = prom_reduction_now_ns();
  if (!prom_model_block_record_static_audit_batch(
          state, block, execution_generation, first_resident_execution, &detail)) {
    prom_model_block_mark_failure(block, detail);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  memcpy(request->audit_arena, block->audit_readback.mapped, PROM_ZIMAGE_TURBO_AUDIT_ARENA_BYTES);
  if (first_resident_execution) block->resident_input_generation = request->input_generation;
  block->output_valid = 1u;
  block->audit_valid = 1u;
  block->output_generation = execution_generation;
  block->execution_count += 1u;
  block->last_execution_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  block->replay_identity = prom_model_block_hash_u64(PROM_ZIMAGE_TURBO_AUDIT_LOCK_ID, request->output_identity);
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, execution_generation);
  block->m1b_prefix_replay_identity = 0u;
  block->last_detail_code = 0;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
}

int prom_reactor_runtime_noise_refiner_execute_resident_impl(
    void* handle, uint64_t block_id, const PrometheusNoiseRefinerResidentExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  uint64_t begin_ns;
  uint64_t audit_elements = 0u;
  uint64_t gpu_compute_ns = 0u;
  int capture_m1b;
  int capture_m1c;
  int capture_m1d;
  int32_t execution_detail;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL ||
      request->struct_size != sizeof(*request) || request->input_generation == 0u ||
      request->output_identity == 0u || request->audit_enabled > 1u ||
      (request->audit_enabled == 0u && (request->audit_family != PROM_NOISE_REFINER_AUDIT_NONE ||
                                        request->audit_stage != 0u || request->audit_output != NULL ||
                                        request->audit_element_capacity != 0u)) ||
      (request->audit_enabled != 0u && (request->audit_family < PROM_NOISE_REFINER_AUDIT_M1B ||
                                        request->audit_family > PROM_NOISE_REFINER_AUDIT_M1D ||
                                        request->audit_output == NULL))) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  capture_m1b = request->audit_enabled != 0u && request->audit_family == PROM_NOISE_REFINER_AUDIT_M1B;
  capture_m1c = request->audit_enabled != 0u && request->audit_family == PROM_NOISE_REFINER_AUDIT_M1C;
  capture_m1d = request->audit_enabled != 0u && request->audit_family == PROM_NOISE_REFINER_AUDIT_M1D;
  if (capture_m1b) {
    if (request->audit_stage < PROM_MODEL_BLOCK_M1B_AUDIT_INGRESS_INPUT || request->audit_stage > PROM_MODEL_BLOCK_M1B_AUDIT_POSITIONED_K) goto invalid_audit;
    audit_elements = request->audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_ADALN_PROJECTION ? 15360u :
        (request->audit_stage >= PROM_MODEL_BLOCK_M1B_AUDIT_FUSED_QKV && request->audit_stage <= PROM_MODEL_BLOCK_M1B_AUDIT_FUSED_QKV ? PROM_MODEL_BLOCK_M1B_QKV_ELEMENTS :
        (request->audit_stage == PROM_MODEL_BLOCK_M1B_AUDIT_INGRESS_TIMESTEP ? 256u :
        ((request->audit_stage >= PROM_MODEL_BLOCK_M1B_AUDIT_ATTENTION_SCALE_RAW && request->audit_stage <= PROM_MODEL_BLOCK_M1B_AUDIT_MLP_GATE_TANH) ? 3840u : PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS)));
  } else if (capture_m1c) {
    if (request->audit_stage < PROM_MODEL_BLOCK_M1C_AUDIT_ATTENTION || request->audit_stage > PROM_MODEL_BLOCK_M1C_AUDIT_RESIDUAL) goto invalid_audit;
    audit_elements = PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS;
  } else if (capture_m1d) {
    if (request->audit_stage < PROM_MODEL_BLOCK_M1D_AUDIT_FFN_NORM || request->audit_stage > PROM_MODEL_BLOCK_M1D_AUDIT_FINAL_OUTPUT) goto invalid_audit;
    audit_elements = (request->audit_stage == PROM_MODEL_BLOCK_M1D_AUDIT_W1 || request->audit_stage == PROM_MODEL_BLOCK_M1D_AUDIT_W3 || request->audit_stage == PROM_MODEL_BLOCK_M1D_AUDIT_GATED_HIDDEN) ? PROM_MODEL_BLOCK_M1D_HIDDEN_ELEMENTS : PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS;
  }
  if (request->audit_enabled != 0u && request->audit_element_capacity < audit_elements) goto invalid_audit;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  if (block->assembly_family != PROM_NOISE_REFINER_FAMILY_Z_IMAGE_TURBO ||
      block->parameter_set != PROM_NOISE_REFINER_PARAMETER_SET_1 ||
      block->binding_state != PROM_NOISE_REFINER_BINDING_BOUND || block->weights_uploaded == 0u ||
      (block->resident_input_generation == 0u ? request->input_generation != block->output_generation :
                                                request->input_generation != block->resident_input_generation) ||
      !prom_model_block_reap(state, block)) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_STALE_OUTPUT);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  block->output_valid = 0u;
  block->audit_valid = 0u;
  block->m1b_prefix_replay_identity = prom_model_block_hash_u64(block->assembly_family, block->parameter_set);
  block->m1b_prefix_replay_identity = prom_model_block_hash_u64(block->m1b_prefix_replay_identity, block->binding_generation);
  block->m1b_prefix_replay_identity = prom_model_block_hash_u64(block->m1b_prefix_replay_identity, request->input_generation);
  begin_ns = prom_reduction_now_ns();
  if (!prom_model_block_m1b_record_execute(state, block, capture_m1b ? request->audit_stage : PROM_MODEL_BLOCK_M1B_AUDIT_NONE,
                                            block->resident_input_generation == 0u ? 1 : 2,
                                            &execution_detail)) {
    prom_model_block_mark_failure(block, execution_detail);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  gpu_compute_ns += prom_model_block_resolve_gpu_span(
      state, block, PROM_MODEL_BLOCK_M1B_PIPELINE_COUNT);
  if (capture_m1b) {
    memcpy(request->audit_output, block->audit_readback.mapped, (size_t)(audit_elements * sizeof(float)));
    block->audit_valid = 1u;
    block->execution_count += 1u;
    block->last_execution_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
    block->replay_identity = prom_model_block_hash_u64(block->m1b_prefix_replay_identity, request->audit_stage);
    block->last_detail_code = 0;
    prom_model_block_fill_evidence(block, 0, out_evidence);
    return PROM_OK;
  }
  if (!prom_model_block_m1c_record_execute(state, block, capture_m1c ? request->audit_stage : PROM_MODEL_BLOCK_M1C_AUDIT_RESIDUAL, capture_m1c,
                                            &execution_detail)) {
    prom_model_block_mark_failure(block, execution_detail);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  gpu_compute_ns += prom_model_block_resolve_gpu_span(state, block, 1u);
  if (capture_m1c) {
    memcpy(request->audit_output, block->audit_readback.mapped, (size_t)(audit_elements * sizeof(float)));
    block->audit_valid = 1u;
    block->execution_count += 2u;
    block->last_execution_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
    block->replay_identity = prom_model_block_hash_u64(block->m1b_prefix_replay_identity, request->audit_stage);
    block->last_detail_code = 0;
    prom_model_block_fill_evidence(block, 0, out_evidence);
    return PROM_OK;
  }
  if (!prom_model_block_m1d_record_execute(state, block, capture_m1d ? request->audit_stage : PROM_MODEL_BLOCK_M1D_AUDIT_FINAL_OUTPUT, capture_m1d,
                                            &execution_detail)) {
    prom_model_block_mark_failure(block, execution_detail);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  gpu_compute_ns += prom_model_block_resolve_gpu_span(state, block, 1u);
  block->gpu_compute_ns = gpu_compute_ns;
  block->gpu_total_ns = gpu_compute_ns;
  block->output_valid = 1u;
  if (block->resident_input_generation == 0u) block->resident_input_generation = request->input_generation;
  if (request->audit_enabled != 0u) {
    memcpy(request->audit_output, block->audit_readback.mapped, (size_t)(audit_elements * sizeof(float)));
    block->audit_valid = 1u;
  }
  block->output_generation += 1u;
  block->execution_count += 3u;
  block->last_execution_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  block->replay_identity = prom_model_block_hash_u64(block->m1b_prefix_replay_identity, request->output_identity);
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, block->parameter_set_aggregate_identity);
  block->last_detail_code = 0;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
invalid_audit:
  prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
  return PROM_ERROR;
}

/* Audit is an explicit post-completion egress. It copies the already-resident
   final ModelEmbedding; it never participates in the block-to-block path. */
int prom_reactor_runtime_noise_refiner_audit_final_impl(
    void* handle, uint64_t block_id, const PrometheusNoiseRefinerFinalAuditRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  VkCommandBufferBeginInfo begin_info;
  VkBufferMemoryBarrier barrier;
  VkBufferCopy copy;
  int32_t detail = PROM_MODEL_BLOCK_DETAIL_AUDIT_FAILED;
  uint64_t output_bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS);
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL ||
      request->struct_size != sizeof(*request) || request->output == NULL || request->output_identity == 0u ||
      request->required_output_generation == 0u || request->output_element_capacity < PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  if (block->output_valid == 0u || block->output_generation != request->required_output_generation ||
      block->audit_readback.size < output_bytes || !prom_model_block_reap(state, block)) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_STALE_OUTPUT);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  if (vkResetCommandBuffer(block->command_buffer, 0u) != VK_SUCCESS) goto fail;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(block->command_buffer, &begin_info) != VK_SUCCESS) goto fail;
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = block->attention.buffer;
  barrier.size = output_bytes;
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  memset(&copy, 0, sizeof(copy));
  copy.size = output_bytes;
  vkCmdCopyBuffer(block->command_buffer, block->attention.buffer, block->audit_readback.buffer, 1u, &copy);
  if (vkEndCommandBuffer(block->command_buffer) != VK_SUCCESS ||
      !prom_model_block_submit_and_wait(state, block, &detail)) goto fail;
  memcpy(request->output, block->audit_readback.mapped, (size_t)output_bytes);
  block->audit_valid = 1u;
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, request->output_identity);
  block->last_detail_code = 0;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
fail:
  prom_model_block_mark_failure(block, detail == 0 ? PROM_MODEL_BLOCK_DETAIL_AUDIT_FAILED : detail);
  prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
  return PROM_ERROR;
}

int prom_reactor_runtime_model_block_test_bf16_ingress_impl(
    void* handle, uint64_t block_id, const uint16_t* input_bf16, uint32_t element_count,
    float* output_fp32, uint32_t output_capacity, PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  VkBufferMemoryBarrier barrier;
  prom_model_block_m1b_ingress_constants constants;
  VkDeviceSize input_bytes;
  VkDeviceSize output_bytes;
  VkResult result;
  int32_t detail = PROM_MODEL_BLOCK_DETAIL_COMMAND_RECORD_FAILED;
  if (!prom_reactor_runtime_validate_handle(handle) || input_bf16 == NULL || output_fp32 == NULL ||
      element_count == 0u || output_capacity < element_count) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  input_bytes = PROM_MODEL_BLOCK_M1B_BF16_BYTES(element_count);
  output_bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(element_count);
  if (block->shader_id != PROM_MODEL_BLOCK_M1B_ADALN_SHADER_ID ||
      input_bytes > block->input_upload.size || output_bytes > block->audit_readback.size) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  memcpy(block->input_upload.mapped, input_bf16, (size_t)input_bytes);
  if (vkResetCommandBuffer(block->command_buffer, 0u) != VK_SUCCESS) goto fail;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(block->command_buffer, &begin_info) != VK_SUCCESS) goto fail;
  memset(&copy, 0, sizeof(copy));
  copy.size = input_bytes;
  vkCmdCopyBuffer(block->command_buffer, block->input_upload.buffer,
                  block->input_bf16_device.buffer, 1u, &copy);
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = block->input_bf16_device.buffer;
  barrier.size = input_bytes;
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  if ((block->test_flags & PROM_TESTCFG_FAIL_DISPATCH) != 0u) {
    detail = PROM_MODEL_BLOCK_DETAIL_INGRESS_DISPATCH_FAILED;
    (void)vkEndCommandBuffer(block->command_buffer);
    goto fail;
  }
  constants.input_elements = element_count;
  constants.timestep_elements = 0u;
  prom_model_block_m1b_bind_and_dispatch(
      block->command_buffer, &block->m1b_pipelines[0u], &constants, sizeof(constants),
      prom_reduction_ceil_div_u32(element_count, PROM_MODEL_BLOCK_WORKGROUP_SIZE), 1u, 1u);
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = block->input_device.buffer;
  barrier.size = output_bytes;
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  copy.size = output_bytes;
  vkCmdCopyBuffer(block->command_buffer, block->input_device.buffer,
                  block->audit_readback.buffer, 1u, &copy);
  result = vkEndCommandBuffer(block->command_buffer);
  if (result != VK_SUCCESS || !prom_model_block_submit_and_wait(state, block, &detail)) goto fail;
  memcpy(output_fp32, block->audit_readback.mapped, (size_t)output_bytes);
  block->last_detail_code = 0;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;

fail:
  prom_model_block_mark_failure(block, detail);
  prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
  return PROM_ERROR;
}

int prom_reactor_runtime_model_block_get_evidence_impl(void* handle, uint64_t block_id,
                                                        PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  if (out_evidence == NULL || !prom_reactor_runtime_validate_handle(handle)) return PROM_ERROR;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  prom_model_block_fill_evidence(&state->model_block, state->model_block.last_detail_code, out_evidence);
  return PROM_OK;
}

int prom_reactor_runtime_model_block_destroy_impl(void* handle, uint64_t block_id) {
  prom_reduction_runtime_state* state;
  if (!prom_reactor_runtime_validate_handle(handle)) return PROM_ERROR;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) return PROM_ERROR;
  prom_model_block_cleanup_state(state);
  return PROM_OK;
}

static prom_compiled_model_stream_slot* prom_compiled_session_slot(
    prom_compiled_model_session_state* session, uint32_t role) {
  if (session == NULL || role == 0u || role > PROM_ZIMAGE_STREAM_SLOT_COUNT) return NULL;
  return &session->streams[role - 1u];
}

static int prom_main_transformer_resolve_timestamps(prom_reduction_runtime_state* state,
                                                     prom_model_block_state* block) {
  uint64_t timestamps[PROM_MODEL_BLOCK_MAIN_QUERY_COUNT];
  uint32_t stage;
  if (state == NULL || block == NULL || block->m1b_timestamp_supported == 0u ||
      block->m1b_timestamp_query_pool == VK_NULL_HANDLE) return 0;
  if (vkGetQueryPoolResults(state->device, block->m1b_timestamp_query_pool, 0u,
                            PROM_MODEL_BLOCK_MAIN_QUERY_COUNT, sizeof(timestamps), timestamps,
                            sizeof(uint64_t), VK_QUERY_RESULT_64_BIT | VK_QUERY_RESULT_WAIT_BIT) != VK_SUCCESS) return 0;
  block->gpu_total_begin_tick = timestamps[0u];
  block->gpu_total_end_tick = timestamps[15u];
  block->gpu_compute_begin_tick = timestamps[1u];
  block->gpu_compute_end_tick = timestamps[14u];
  block->gpu_total_ns = (uint64_t)((double)(timestamps[15u] - timestamps[0u]) *
                                   (double)block->m1b_timestamp_period_ns + 0.5);
  block->gpu_compute_ns = (uint64_t)((double)(timestamps[14u] - timestamps[1u]) *
                                     (double)block->m1b_timestamp_period_ns + 0.5);
  block->gpu_ingress_transfer_ns = (uint64_t)((double)(timestamps[1u] - timestamps[0u]) *
                                              (double)block->m1b_timestamp_period_ns + 0.5);
  block->gpu_joint_copy_ns = (uint64_t)((double)(timestamps[15u] - timestamps[14u]) *
                                        (double)block->m1b_timestamp_period_ns + 0.5);
  for (stage = 0u; stage < PROM_MODEL_BLOCK_MAIN_STAGE_COUNT; ++stage) {
    block->main_stage_gpu_begin_tick[stage] = timestamps[stage + 1u];
    block->main_stage_gpu_end_tick[stage] = timestamps[stage + 2u];
    block->main_stage_gpu_ns[stage] = (uint64_t)(
        (double)(timestamps[stage + 2u] - timestamps[stage + 1u]) *
        (double)block->m1b_timestamp_period_ns + 0.5);
  }
  return 1;
}

static int prom_main_transformer_record_execute(prom_reduction_runtime_state* state,
                                                prom_model_block_state* block,
                                                prom_compiled_model_session_state* session,
                                                uint64_t requested_joint_generation,
                                                uint32_t resident_chain_mode,
                                                int32_t* out_detail) {
  prom_compiled_model_stream_slot* joint;
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copies[3];
  VkBufferMemoryBarrier barriers[2];
  prom_model_block_m1b_ingress_constants ingress = {0u, PROM_MODEL_BLOCK_M1B_TIMESTEP_ELEMENTS};
  prom_model_block_m1b_adaln_constants adaln = {15360u, 256u};
  prom_model_block_m1b_norm_constants norm = {1.0e-5f, PROM_MODEL_BLOCK_MAIN_TOKENS,
                                               PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS, 0u};
  prom_model_block_m1b_qkv_constants qkv = {PROM_MODEL_BLOCK_MAIN_TOKENS,
                                             PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS, 11520u, 0u};
  prom_model_block_main_qk_constants q_head = {1.0e-5f, PROM_MODEL_BLOCK_MAIN_TOKENS,
                                                30u, 128u, 0u,
                                                PROM_MODEL_BLOCK_MAIN_IMAGE_TOKENS};
  prom_model_block_main_qk_constants k_head = {1.0e-5f, PROM_MODEL_BLOCK_MAIN_TOKENS,
                                                30u, 128u, 3840u,
                                                PROM_MODEL_BLOCK_MAIN_IMAGE_TOKENS};
  prom_model_block_m1c_attention_constants attention = {PROM_MODEL_BLOCK_MAIN_TOKENS, 30u, 128u, 11520u};
  prom_model_block_m1b_qkv_constants projection = {PROM_MODEL_BLOCK_MAIN_TOKENS,
                                                    PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS, 3840u, 0u};
  prom_model_block_m1b_qkv_constants hidden = {PROM_MODEL_BLOCK_MAIN_TOKENS,
                                                PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS, 10240u, 0u};
  prom_model_block_m1d_gate_constants gate = {PROM_MODEL_BLOCK_MAIN_HIDDEN_ELEMENTS, 0u, 0u, 0u};
  prom_model_block_m1d_w2_constants w2 = {1.0e-5f, PROM_MODEL_BLOCK_MAIN_TOKENS,
                                           PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS, 10240u};
  VkResult result;
  uint64_t phase_begin_ns;
  uint64_t record_begin_ns;
  if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_COMMAND_RECORD_FAILED;
  if (state == NULL || block == NULL || session == NULL ||
      prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_COMMAND_RECORD)) return 0;
  joint = prom_compiled_session_slot(session, PROM_ZIMAGE_STREAM_JOINT_WORKING);
  if (joint == NULL || joint->valid == 0u || joint->generation != requested_joint_generation ||
      joint->device.size != PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_MAIN_MODEL_ELEMENTS) ||
      block->input_device.size != joint->device.size || block->timestep_upload.mapped == NULL) return 0;
  phase_begin_ns = prom_reduction_now_ns();
  if (vkResetCommandBuffer(block->command_buffer, 0u) != VK_SUCCESS) return 0;
  block->last_command_reset_ns = prom_reduction_elapsed_ns(phase_begin_ns, prom_reduction_now_ns());
  block->vk_reset_command_buffer_count += 1u;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  phase_begin_ns = prom_reduction_now_ns();
  if (vkBeginCommandBuffer(block->command_buffer, &begin_info) != VK_SUCCESS) return 0;
  block->last_command_begin_ns = prom_reduction_elapsed_ns(phase_begin_ns, prom_reduction_now_ns());
  record_begin_ns = prom_reduction_now_ns();
  if (block->m1b_timestamp_supported != 0u) {
    vkCmdResetQueryPool(block->command_buffer, block->m1b_timestamp_query_pool, 0u,
                        PROM_MODEL_BLOCK_MAIN_QUERY_COUNT);
    vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                        block->m1b_timestamp_query_pool, 0u);
  }
  memset(copies, 0, sizeof(copies));
  copies[0].size = joint->device.size;
  copies[1].size = block->timestep_upload.size;
  vkCmdCopyBuffer(block->command_buffer, joint->device.buffer, block->input_device.buffer, 1u, &copies[0]);
  vkCmdCopyBuffer(block->command_buffer, block->timestep_upload.buffer,
                  block->timestep_bf16_device.buffer, 1u, &copies[1]);
  memset(barriers, 0, sizeof(barriers));
  barriers[0].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barriers[0].srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
  barriers[0].dstAccessMask = VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT;
  barriers[0].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barriers[0].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barriers[0].buffer = block->input_device.buffer;
  barriers[0].size = block->input_device.size;
  barriers[1] = barriers[0];
  barriers[1].buffer = block->timestep_bf16_device.buffer;
  barriers[1].size = block->timestep_bf16_device.size;
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0u, 0u, NULL, 2u, barriers, 0u, NULL);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(
      block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
      block->m1b_timestamp_query_pool, 1u);
  if ((block->test_flags & PROM_TESTCFG_FAIL_DISPATCH) != 0u) {
    if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_INGRESS_DISPATCH_FAILED;
    return 0;
  }
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[0u],
                                         &ingress, sizeof(ingress), 1u, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 2u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[1u],
                                         &adaln, sizeof(adaln), 60u, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 3u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[2u],
                                         &norm, sizeof(norm), PROM_MODEL_BLOCK_MAIN_TOKENS, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 4u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[3u],
                                         &qkv, sizeof(qkv), 66u, 720u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 5u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[4u],
                                         &q_head, sizeof(q_head), 31680u, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 6u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[5u],
                                         &k_head, sizeof(k_head), 31680u, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 7u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1c_pipelines[0u],
                                         &attention, sizeof(attention),
                                         (block->main_attention_shader_id == PROM_MODEL_BLOCK_MAIN_ATTENTION_SUBGROUP_OWNED32_SHADER_ID
                                             ? PROM_MODEL_BLOCK_MAIN_ATTENTION_SUBGROUP_OWNED32_GROUPS
                                             : PROM_MODEL_BLOCK_MAIN_ATTENTION_SERIAL_GROUPS), 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 8u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1c_pipelines[1u],
                                         &projection, sizeof(projection), 66u, 240u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 9u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1c_pipelines[2u],
                                         &norm, sizeof(norm), PROM_MODEL_BLOCK_MAIN_TOKENS, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 10u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[0u],
                                         &norm, sizeof(norm), PROM_MODEL_BLOCK_MAIN_TOKENS, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 11u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[1u],
                                         &hidden, sizeof(hidden), 66u, 640u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 12u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[2u],
                                         &gate, sizeof(gate), 42240u, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 13u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[3u],
                                         &w2, sizeof(w2), PROM_MODEL_BLOCK_MAIN_TOKENS, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 14u);
  prom_reduction_record_barrier(block->command_buffer);
  /* W2 writes its projected intermediate to input_device and the final FP32
     gated residual to attention. Chain mode copies that final buffer back to
     the lock-owned JointWorking slot before completion, making attention and
     and JointWorking the two fixed ping/pong states for the next layer. */
  if (resident_chain_mode != 0u) {
  barriers[0].srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
  barriers[0].dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
  barriers[0].buffer = block->attention.buffer;
  barriers[0].size = block->attention.size;
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 1u, barriers, 0u, NULL);
  memset(&copies[2], 0, sizeof(copies[2]));
  copies[2].size = joint->device.size;
  vkCmdCopyBuffer(block->command_buffer, block->attention.buffer, joint->device.buffer, 1u, &copies[2]);
  barriers[0].srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
  barriers[0].dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT | VK_ACCESS_SHADER_READ_BIT;
  barriers[0].buffer = joint->device.buffer;
  barriers[0].size = joint->device.size;
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                       VK_PIPELINE_STAGE_TRANSFER_BIT | VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       0u, 0u, NULL, 1u, barriers, 0u, NULL);
  }
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(
      block->command_buffer,
      resident_chain_mode != 0u ? VK_PIPELINE_STAGE_TRANSFER_BIT : VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
      block->m1b_timestamp_query_pool, 15u);
  block->last_command_record_ns = prom_reduction_elapsed_ns(record_begin_ns, prom_reduction_now_ns());
  phase_begin_ns = prom_reduction_now_ns();
  result = (block->test_flags & PROM_TESTCFG_FAIL_COMMAND_END) != 0u
               ? VK_ERROR_INITIALIZATION_FAILED
               : vkEndCommandBuffer(block->command_buffer);
  block->last_command_end_ns = prom_reduction_elapsed_ns(phase_begin_ns, prom_reduction_now_ns());
  if (result != VK_SUCCESS) return 0;
  return prom_model_block_submit_and_wait(state, block, out_detail);
}

static void prom_compiled_session_fill_evidence(const prom_compiled_model_session_state* session,
                                                PrometheusCompiledModelSessionEvidence* out_evidence) {
  const prom_compiled_model_stream_slot* image;
  const prom_compiled_model_stream_slot* context;
  const prom_compiled_model_stream_slot* joint;
  if (out_evidence == NULL) return;
  memset(out_evidence, 0, sizeof(*out_evidence));
  out_evidence->struct_size = (uint32_t)sizeof(*out_evidence);
  if (session == NULL) {
    out_evidence->detail_code = PROM_MODEL_SESSION_DETAIL_INVALID_REQUEST;
    return;
  }
  image = &session->streams[PROM_ZIMAGE_STREAM_PREPARED_IMAGE - 1u];
  context = &session->streams[PROM_ZIMAGE_STREAM_PREPARED_CONTEXT - 1u];
  joint = &session->streams[PROM_ZIMAGE_STREAM_JOINT_WORKING - 1u];
  out_evidence->detail_code = session->last_detail_code;
  out_evidence->created = session->created;
  out_evidence->quarantined = session->quarantined;
  out_evidence->session_identity = session->session_id;
  out_evidence->lock_identity = session->lock_identity;
  out_evidence->active_block_id = session->active_block_id;
  out_evidence->binding_generation = session->binding_generation;
  out_evidence->replay_identity = session->replay_identity;
  out_evidence->prepared_image_generation = image->generation;
  out_evidence->prepared_context_generation = context->generation;
  out_evidence->joint_generation = joint->generation;
  out_evidence->joint_image_generation = session->joint_image_generation;
  out_evidence->joint_context_generation = session->joint_context_generation;
  out_evidence->prepared_image_bytes = image->device.size;
  out_evidence->prepared_context_bytes = context->device.size;
  out_evidence->joint_bytes = joint->device.size;
  out_evidence->cold_buffer_allocation_count = session->cold_buffer_allocation_count;
  out_evidence->warm_buffer_allocation_count = session->warm_buffer_allocation_count;
  out_evidence->composition_count = session->composition_count;
  out_evidence->requested_execution_profile = session->requested_execution_profile;
  out_evidence->selected_execution_profile = session->selected_execution_profile;
  out_evidence->profile_fallback_reason = session->profile_fallback_reason;
  out_evidence->requested_main_attention_route = session->requested_main_attention_route;
  out_evidence->selected_main_attention_route = session->selected_main_attention_route;
  out_evidence->main_attention_fallback_reason = session->main_attention_fallback_reason;
  out_evidence->main_attention_shader_id = session->main_attention_shader_id;
}

static int prom_compiled_session_submit_and_wait(prom_reduction_runtime_state* state,
                                                 prom_compiled_model_session_state* session) {
  VkSubmitInfo submit_info;
  if (state == NULL || session == NULL || session->command_buffer == VK_NULL_HANDLE ||
      session->fence == VK_NULL_HANDLE || vkResetFences(state->device, 1u, &session->fence) != VK_SUCCESS) return 0;
  memset(&submit_info, 0, sizeof(submit_info));
  submit_info.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit_info.commandBufferCount = 1u;
  submit_info.pCommandBuffers = &session->command_buffer;
  if (vkQueueSubmit(state->queue, 1u, &submit_info, session->fence) != VK_SUCCESS ||
      vkWaitForFences(state->device, 1u, &session->fence, VK_TRUE, UINT64_MAX) != VK_SUCCESS) {
    session->quarantined = 1u;
    session->last_detail_code = PROM_MODEL_SESSION_DETAIL_COMPLETION_UNCERTAIN;
    return 0;
  }
  return 1;
}

void prom_compiled_model_session_cleanup_state(prom_reduction_runtime_state* state) {
  prom_compiled_model_session_state* session;
  uint64_t next_session_id;
  uint32_t index;
  if (state == NULL) return;
  session = &state->compiled_session;
  next_session_id = session->next_session_id;
  if (state->device != VK_NULL_HANDLE && session->fence != VK_NULL_HANDLE && session->quarantined != 0u) {
    (void)vkWaitForFences(state->device, 1u, &session->fence, VK_TRUE, UINT64_MAX);
  }
  if (state->device != VK_NULL_HANDLE) {
    for (index = 0u; index < PROM_ZIMAGE_STREAM_SLOT_COUNT; ++index)
      prom_vk_destroy_buffer(state->device, &session->streams[index].device);
    if (session->fence != VK_NULL_HANDLE) vkDestroyFence(state->device, session->fence, NULL);
    if (session->command_buffer != VK_NULL_HANDLE && state->command_pool != VK_NULL_HANDLE)
      vkFreeCommandBuffers(state->device, state->command_pool, 1u, &session->command_buffer);
  }
  memset(session, 0, sizeof(*session));
  session->next_session_id = next_session_id == 0u ? 1u : next_session_id;
}

int prom_reactor_runtime_compiled_model_session_create_impl(
    void* handle, const PrometheusCompiledModelSessionCreateRequest* request, uint64_t* out_session_id,
    PrometheusCompiledModelSessionEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_compiled_model_session_state* session;
  VkCommandBufferAllocateInfo command_info;
  VkFenceCreateInfo fence_info;
  const PrometheusCompiledModelResidentStreamDescriptor* image_descriptor;
  const PrometheusCompiledModelResidentStreamDescriptor* context_descriptor;
  const PrometheusCompiledModelResidentStreamDescriptor* joint_descriptor;
  uint64_t next_session_id;
  int32_t detail = 0;
  const VkBufferUsageFlags usage = VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT |
                                   VK_BUFFER_USAGE_TRANSFER_DST_BIT;
  VkDeviceSize image_bytes;
  VkDeviceSize context_bytes;
  VkDeviceSize joint_bytes;
  if (out_session_id != NULL) *out_session_id = 0u;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL || out_session_id == NULL ||
      request->struct_size != sizeof(*request) || request->lock_identity != PROM_ZIMAGE_TURBO_LOCK_ID ||
      (request->execution_profile != PROM_MODEL_EXECUTION_PROFILE_MINIMUM_MEMORY &&
       request->execution_profile != PROM_MODEL_EXECUTION_PROFILE_PREFETCH) ||
      (request->main_attention_route_policy != PROM_MAIN_ATTENTION_ROUTE_AUTO &&
       request->main_attention_route_policy != PROM_MAIN_ATTENTION_ROUTE_SERIAL_CANONICAL &&
       request->main_attention_route_policy != PROM_MAIN_ATTENTION_ROUTE_SUBGROUP_OWNED32)) {
    prom_compiled_session_fill_evidence(NULL, out_evidence);
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL || state->compiled_session.created != 0u) {
    prom_compiled_session_fill_evidence(state == NULL ? NULL : &state->compiled_session, out_evidence);
    return PROM_ERROR;
  }
  session = &state->compiled_session;
  next_session_id = session->next_session_id;
  memset(session, 0, sizeof(*session));
  session->next_session_id = next_session_id == 0u ? 1u : next_session_id;
  session->lock_identity = request->lock_identity;
  session->requested_execution_profile = request->execution_profile;
  session->requested_main_attention_route = request->main_attention_route_policy;
  /* The owner performs the final admission after it has observed the runtime
     transfer queue.  Session creation freezes the request for replay. */
  session->selected_execution_profile = PROM_MODEL_EXECUTION_PROFILE_MINIMUM_MEMORY;
  session->selected_main_attention_route = PROM_MAIN_ATTENTION_ROUTE_SERIAL_CANONICAL;
  session->main_attention_shader_id = PROM_MODEL_BLOCK_MAIN_ATTENTION_SERIAL_SHADER_ID;
  image_descriptor = prom_zimage_turbo_resolve_resident_stream_descriptor(
      request->lock_identity, PROM_ZIMAGE_STREAM_PREPARED_IMAGE);
  context_descriptor = prom_zimage_turbo_resolve_resident_stream_descriptor(
      request->lock_identity, PROM_ZIMAGE_STREAM_PREPARED_CONTEXT);
  joint_descriptor = prom_zimage_turbo_resolve_resident_stream_descriptor(
      request->lock_identity, PROM_ZIMAGE_STREAM_JOINT_WORKING);
  if (image_descriptor == NULL || context_descriptor == NULL || joint_descriptor == NULL ||
      image_descriptor->byte_count == 0u || context_descriptor->byte_count == 0u ||
      joint_descriptor->byte_count != image_descriptor->byte_count + context_descriptor->byte_count ||
      image_descriptor->token_count != 1024u || context_descriptor->token_count != 32u ||
      joint_descriptor->token_count != 1056u || image_descriptor->hidden_width != 3840u ||
      context_descriptor->hidden_width != 3840u || joint_descriptor->hidden_width != 3840u) goto fail;
  image_bytes = (VkDeviceSize)image_descriptor->byte_count;
  context_bytes = (VkDeviceSize)context_descriptor->byte_count;
  joint_bytes = (VkDeviceSize)joint_descriptor->byte_count;
  if (prom_vk_create_buffer(state->physical_device, state->device, 0u, image_bytes, usage,
                            VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &session->streams[0u].device) != VK_SUCCESS ||
      prom_vk_create_buffer(state->physical_device, state->device, 0u, context_bytes, usage,
                            VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &session->streams[1u].device) != VK_SUCCESS ||
      prom_vk_create_buffer(state->physical_device, state->device, 0u, joint_bytes, usage,
                            VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &session->streams[2u].device) != VK_SUCCESS) goto fail;
  session->cold_buffer_allocation_count = 3u;
  memset(&command_info, 0, sizeof(command_info));
  command_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO;
  command_info.commandPool = state->command_pool;
  command_info.level = VK_COMMAND_BUFFER_LEVEL_PRIMARY;
  command_info.commandBufferCount = 1u;
  if (vkAllocateCommandBuffers(state->device, &command_info, &session->command_buffer) != VK_SUCCESS) goto fail;
  memset(&fence_info, 0, sizeof(fence_info));
  fence_info.sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO;
  fence_info.flags = VK_FENCE_CREATE_SIGNALED_BIT;
  if (vkCreateFence(state->device, &fence_info, NULL, &session->fence) != VK_SUCCESS) goto fail;
  session->session_id = session->next_session_id++;
  session->created = 1u;
  session->last_detail_code = 0;
  *out_session_id = session->session_id;
  prom_compiled_session_fill_evidence(session, out_evidence);
  return PROM_OK;
fail:
  session->last_detail_code = PROM_MODEL_BLOCK_DETAIL_RESOURCE_CREATE_FAILED;
  prom_compiled_model_session_cleanup_state(state);
  prom_compiled_session_fill_evidence(NULL, out_evidence);
  return PROM_ERROR;
}

int prom_reactor_runtime_compiled_model_session_capture_completed_impl(
    void* handle, uint64_t session_id, uint64_t completed_block_id,
    const PrometheusCompiledModelSessionCaptureRequest* request,
    PrometheusCompiledModelSessionEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_compiled_model_session_state* session;
  prom_model_block_state* block;
  prom_compiled_model_stream_slot* slot;
  VkCommandBufferBeginInfo begin_info;
  VkBufferMemoryBarrier barriers[2];
  VkBufferCopy copy;
  const PrometheusCompiledModelResidentStreamDescriptor* descriptor;
  uint32_t role;
  VkDeviceSize bytes;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL || request->struct_size != sizeof(*request)) goto invalid;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL) goto invalid;
  session = &state->compiled_session;
  block = &state->model_block;
  if (session->created == 0u || session->session_id != session_id || session->quarantined != 0u ||
      block->created == 0u || block->block_id != completed_block_id || block->output_valid == 0u ||
      block->output_generation != request->source_output_generation || !prom_model_block_reap(state, block)) goto stale;
  if (block->assembly_family == PROM_NOISE_REFINER_FAMILY_Z_IMAGE_TURBO &&
      block->parameter_set == PROM_NOISE_REFINER_PARAMETER_SET_1) {
    role = PROM_ZIMAGE_STREAM_PREPARED_IMAGE;
    bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS);
  } else if (block->assembly_family == PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO &&
             block->parameter_set == PROM_CONTEXT_REFINER_PARAMETER_SET_1) {
    role = PROM_ZIMAGE_STREAM_PREPARED_CONTEXT;
    bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_CONTEXT_MODEL_ELEMENTS);
  } else goto mismatch;
  slot = prom_compiled_session_slot(session, role);
  descriptor = prom_zimage_turbo_resolve_resident_stream_descriptor(session->lock_identity, role);
  if (descriptor == NULL || descriptor->byte_count != bytes || slot == NULL ||
      block->attention.size < bytes || slot->device.size != bytes ||
      vkResetCommandBuffer(session->command_buffer, 0u) != VK_SUCCESS) goto invalid;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(session->command_buffer, &begin_info) != VK_SUCCESS) goto invalid;
  memset(barriers, 0, sizeof(barriers));
  barriers[0].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barriers[0].srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
  barriers[0].dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
  barriers[0].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barriers[0].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barriers[0].buffer = block->attention.buffer;
  barriers[0].size = bytes;
  barriers[1].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barriers[1].dstAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
  barriers[1].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barriers[1].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barriers[1].buffer = slot->device.buffer;
  barriers[1].size = bytes;
  vkCmdPipelineBarrier(session->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT | VK_PIPELINE_STAGE_TRANSFER_BIT,
                       VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 2u, barriers, 0u, NULL);
  memset(&copy, 0, sizeof(copy));
  copy.size = bytes;
  vkCmdCopyBuffer(session->command_buffer, block->attention.buffer, slot->device.buffer, 1u, &copy);
  if (vkEndCommandBuffer(session->command_buffer) != VK_SUCCESS || !prom_compiled_session_submit_and_wait(state, session)) goto stale;
  slot->generation += 1u;
  slot->producer_block_id = completed_block_id;
  slot->producer_output_generation = block->output_generation;
  slot->valid = 1u;
  session->active_block_id = completed_block_id;
  session->binding_generation = block->binding_generation;
  session->replay_identity = prom_model_block_hash_u64(session->replay_identity == 0u ? session->lock_identity : session->replay_identity,
                                                        completed_block_id);
  session->replay_identity = prom_model_block_hash_u64(session->replay_identity, slot->generation);
  session->last_detail_code = 0;
  prom_compiled_session_fill_evidence(session, out_evidence);
  return PROM_OK;
mismatch:
  session->last_detail_code = PROM_MODEL_SESSION_DETAIL_STREAM_MISMATCH;
  prom_compiled_session_fill_evidence(session, out_evidence);
  return PROM_ERROR;
stale:
  session->last_detail_code = session->quarantined != 0u ? PROM_MODEL_SESSION_DETAIL_COMPLETION_UNCERTAIN : PROM_MODEL_SESSION_DETAIL_STALE_STREAM;
  prom_compiled_session_fill_evidence(session, out_evidence);
  return PROM_ERROR;
invalid:
  prom_compiled_session_fill_evidence(NULL, out_evidence);
  return PROM_ERROR;
}

int prom_reactor_runtime_compiled_model_session_compose_joint_impl(
    void* handle, uint64_t session_id, const PrometheusCompiledModelSessionComposeRequest* request,
    PrometheusCompiledModelSessionEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_compiled_model_session_state* session;
  prom_compiled_model_stream_slot* image;
  prom_compiled_model_stream_slot* context;
  prom_compiled_model_stream_slot* joint;
  VkCommandBufferBeginInfo begin_info;
  VkBufferMemoryBarrier barriers[3];
  VkBufferCopy copies[2];
  const PrometheusCompiledModelResidentStreamDescriptor* joint_descriptor;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL || request->struct_size != sizeof(*request)) goto invalid;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL) goto invalid;
  session = &state->compiled_session;
  image = prom_compiled_session_slot(session, PROM_ZIMAGE_STREAM_PREPARED_IMAGE);
  context = prom_compiled_session_slot(session, PROM_ZIMAGE_STREAM_PREPARED_CONTEXT);
  joint = prom_compiled_session_slot(session, PROM_ZIMAGE_STREAM_JOINT_WORKING);
  if (session->created == 0u || session->session_id != session_id || session->quarantined != 0u ||
      image == NULL || context == NULL || joint == NULL || image->valid == 0u || context->valid == 0u ||
      request->required_image_generation != image->generation || request->required_context_generation != context->generation)
    goto stale;
  joint_descriptor = prom_zimage_turbo_resolve_resident_stream_descriptor(
      session->lock_identity, PROM_ZIMAGE_STREAM_JOINT_WORKING);
  if (joint_descriptor == NULL || joint_descriptor->byte_count != image->device.size + context->device.size ||
      joint->device.size != joint_descriptor->byte_count || vkResetCommandBuffer(session->command_buffer, 0u) != VK_SUCCESS) goto invalid;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(session->command_buffer, &begin_info) != VK_SUCCESS) goto invalid;
  memset(barriers, 0, sizeof(barriers));
  barriers[0].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barriers[0].srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
  barriers[0].dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
  barriers[0].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barriers[0].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barriers[0].buffer = image->device.buffer;
  barriers[0].size = image->device.size;
  barriers[1] = barriers[0];
  barriers[1].buffer = context->device.buffer;
  barriers[1].size = context->device.size;
  barriers[2].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barriers[2].dstAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
  barriers[2].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barriers[2].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barriers[2].buffer = joint->device.buffer;
  barriers[2].size = joint->device.size;
  vkCmdPipelineBarrier(session->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                       VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 3u, barriers, 0u, NULL);
  memset(copies, 0, sizeof(copies));
  copies[0].size = image->device.size;
  copies[1].dstOffset = image->device.size;
  copies[1].size = context->device.size;
  vkCmdCopyBuffer(session->command_buffer, image->device.buffer, joint->device.buffer, 1u, &copies[0]);
  vkCmdCopyBuffer(session->command_buffer, context->device.buffer, joint->device.buffer, 1u, &copies[1]);
  barriers[0].srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
  barriers[0].dstAccessMask = VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT;
  barriers[0].buffer = joint->device.buffer;
  barriers[0].size = joint->device.size;
  vkCmdPipelineBarrier(session->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0u, 0u, NULL, 1u, barriers, 0u, NULL);
  if (vkEndCommandBuffer(session->command_buffer) != VK_SUCCESS || !prom_compiled_session_submit_and_wait(state, session)) goto stale;
  joint->generation += 1u;
  joint->producer_block_id = 0u;
  joint->producer_output_generation = 0u;
  joint->valid = 1u;
  session->joint_image_generation = image->generation;
  session->joint_context_generation = context->generation;
  session->composition_count += 1u;
  session->replay_identity = prom_model_block_hash_u64(session->replay_identity, image->generation);
  session->replay_identity = prom_model_block_hash_u64(session->replay_identity, context->generation);
  session->replay_identity = prom_model_block_hash_u64(session->replay_identity, joint->generation);
  session->last_detail_code = 0;
  prom_compiled_session_fill_evidence(session, out_evidence);
  return PROM_OK;
stale:
  if (state != NULL) {
    session = &state->compiled_session;
    session->last_detail_code = session->quarantined != 0u ? PROM_MODEL_SESSION_DETAIL_COMPLETION_UNCERTAIN : PROM_MODEL_SESSION_DETAIL_STALE_STREAM;
    prom_compiled_session_fill_evidence(session, out_evidence);
  } else prom_compiled_session_fill_evidence(NULL, out_evidence);
  return PROM_ERROR;
invalid:
  prom_compiled_session_fill_evidence(NULL, out_evidence);
  return PROM_ERROR;
}

/* Rebinding is deliberately limited to the lock's immediate successor.  The
   transaction replaces immutable weights and descriptors only; JointWorking
   remains device-resident and is never copied through the host. */
int prom_reactor_runtime_main_transformer_rebind_impl(
    void* handle, uint64_t block_id, const PrometheusMainTransformerRebindRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  const PrometheusMainTransformerResolvedDescriptor* descriptor;
  prom_model_block_weight_resource old_weights[PROM_MODEL_BLOCK_MAX_WEIGHTS];
  uint32_t index;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL || request->uploads == NULL ||
      request->struct_size != sizeof(*request) || request->upload_count != PROM_MODEL_BLOCK_MAX_WEIGHTS) goto invalid;
  descriptor = prom_zimage_turbo_resolve_main_transformer_descriptor(request->lock_identity,
                                                                     request->model_local_block_id);
  if (descriptor == NULL || descriptor->assembly_family != PROM_MAIN_TRANSFORMER_FAMILY_Z_IMAGE_TURBO ||
      descriptor->parameter_set != descriptor->model_local_block_id + 1u ||
      descriptor->parameter_set_aggregate_identity != prom_main_transformer_expected_aggregate(descriptor->parameter_set)) goto invalid;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  if (!prom_model_block_is_main_transformer(block) || block->weights_uploaded == 0u ||
      block->quarantined != 0u || block->binding_state != PROM_NOISE_REFINER_BINDING_BOUND ||
      descriptor->parameter_set != block->parameter_set + 1u || !prom_model_block_reap(state, block)) goto failed;
  block->binding_state = PROM_NOISE_REFINER_BINDING_VALIDATING;
  for (index = 0u; index < PROM_MODEL_BLOCK_MAX_WEIGHTS; ++index) {
    const PrometheusModelBlockWeightUpload* upload = &request->uploads[index];
    if (upload->binding_index != index || upload->bytes == NULL || upload->content_identity == 0u ||
        upload->layout_identity == 0u || upload->byte_count != k_prom_model_block_m1b_weight_bytes[index] ||
        upload->byte_count > (uint64_t)block->weight_upload.size) goto mismatch;
  }
  prom_model_block_destroy_pending_weights(state, block);
  for (index = 0u; index < PROM_MODEL_BLOCK_MAX_WEIGHTS; ++index) {
    const PrometheusModelBlockWeightUpload* upload = &request->uploads[index];
    block->pending_weights[index].content_identity = upload->content_identity;
    block->pending_weights[index].layout_identity = upload->layout_identity;
    block->pending_weights[index].byte_count = upload->byte_count;
    if (!prom_model_block_create_buffer(state, block, &block->pending_weights[index].device,
                                        (VkDeviceSize)upload->byte_count,
                                        VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                                        VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) goto resource_fail;
  }
  block->binding_state = PROM_NOISE_REFINER_BINDING_UPLOADING;
  for (index = 0u; index < PROM_MODEL_BLOCK_MAX_WEIGHTS; ++index) {
    const PrometheusModelBlockWeightUpload* upload = &request->uploads[index];
    memcpy(block->weight_upload.mapped, upload->bytes, (size_t)upload->byte_count);
    if (!prom_model_block_record_upload(state, block, block->pending_weights, index)) goto upload_fail;
    block->pending_weights[index].uploaded = 1u;
  }
  block->binding_state = PROM_NOISE_REFINER_BINDING_UPDATING_DESCRIPTORS;
  if (!prom_model_block_update_weight_descriptors(state, block, block->pending_weights)) goto descriptor_fail;
  block->binding_state = PROM_NOISE_REFINER_BINDING_READY_TO_COMMIT;
  memcpy(old_weights, block->weights, sizeof(old_weights));
  memcpy(block->weights, block->pending_weights, sizeof(block->weights));
  memset(block->pending_weights, 0, sizeof(block->pending_weights));
  for (index = 0u; index < PROM_MODEL_BLOCK_MAX_WEIGHTS; ++index) prom_vk_destroy_buffer(state->device, &old_weights[index].device);
  block->parameter_set = descriptor->parameter_set;
  block->parameter_set_aggregate_identity = descriptor->parameter_set_aggregate_identity;
  block->binding_generation += 1u;
  block->descriptor_generation += 1u;
  block->output_valid = 0u;
  block->audit_valid = 0u;
  block->resident_input_generation = 0u;
  block->replay_identity = 0u;
  block->binding_state = PROM_NOISE_REFINER_BINDING_BOUND;
  block->weight_upload_count += PROM_MODEL_BLOCK_MAX_WEIGHTS;
  block->last_detail_code = 0;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
mismatch:
  block->last_detail_code = PROM_MODEL_BLOCK_DETAIL_WEIGHT_MISMATCH;
  goto rebind_fail;
resource_fail:
  block->last_detail_code = PROM_MODEL_BLOCK_DETAIL_RESOURCE_CREATE_FAILED;
  goto rebind_fail;
upload_fail:
  block->last_detail_code = block->quarantined != 0u ? PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN : PROM_MODEL_BLOCK_DETAIL_UPLOAD_FAILED;
  goto rebind_fail;
descriptor_fail:
  block->last_detail_code = PROM_MODEL_BLOCK_DETAIL_DESCRIPTOR_UPDATE_FAILED;
rebind_fail:
  prom_model_block_destroy_pending_weights(state, block);
  block->binding_state = block->quarantined != 0u ? PROM_NOISE_REFINER_BINDING_QUARANTINED : PROM_NOISE_REFINER_BINDING_FAILED_BEFORE_COMMIT;
  prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
  return PROM_ERROR;
failed:
  prom_model_block_mark_failure(block, block->quarantined != 0u ? PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN : PROM_MODEL_BLOCK_DETAIL_REBIND_FAILED);
  prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
  return PROM_ERROR;
invalid:
  prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
  return PROM_ERROR;
}

int prom_reactor_runtime_main_transformer_execute_impl(
    void* handle, uint64_t block_id, const PrometheusMainTransformerExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  prom_compiled_model_session_state* session;
  prom_compiled_model_stream_slot* image;
  prom_compiled_model_stream_slot* context;
  prom_compiled_model_stream_slot* joint;
  const PrometheusMainTransformerResolvedDescriptor* descriptor;
  uint64_t begin_ns;
  uint64_t validation_begin_ns = prom_reduction_now_ns();
  uint64_t memcpy_begin_ns;
  int32_t detail;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL ||
      request->struct_size != sizeof(*request) || request->audit_enabled != 0u || request->resident_chain_mode > 1u ||
      request->lock_identity != PROM_ZIMAGE_TURBO_LOCK_ID ||
      request->timestep_bf16 == NULL ||
      request->timestep_bytes != PROM_MODEL_BLOCK_M1B_BF16_BYTES(PROM_MODEL_BLOCK_M1B_TIMESTEP_ELEMENTS) ||
      request->timestep_identity == 0u || request->output_identity == 0u ||
      request->required_image_generation == 0u || request->required_context_generation == 0u ||
      request->required_joint_generation == 0u) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  descriptor = prom_zimage_turbo_resolve_main_transformer_descriptor(request->lock_identity,
                                                                     request->model_local_block_id);
  if (descriptor == NULL ||
      descriptor->assembly_family != PROM_MAIN_TRANSFORMER_FAMILY_Z_IMAGE_TURBO ||
      descriptor->parameter_set != descriptor->model_local_block_id + 1u ||
      descriptor->joint_token_count != PROM_MODEL_BLOCK_MAIN_TOKENS ||
      descriptor->image_token_count != PROM_MODEL_BLOCK_MAIN_IMAGE_TOKENS ||
      descriptor->context_token_count != PROM_MODEL_BLOCK_CONTEXT_TOKENS ||
      descriptor->hidden_width != PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  session = &state->compiled_session;
  image = prom_compiled_session_slot(session, PROM_ZIMAGE_STREAM_PREPARED_IMAGE);
  context = prom_compiled_session_slot(session, PROM_ZIMAGE_STREAM_PREPARED_CONTEXT);
  joint = prom_compiled_session_slot(session, PROM_ZIMAGE_STREAM_JOINT_WORKING);
  if (!prom_model_block_is_main_transformer(block) ||
      block->parameter_set != descriptor->parameter_set ||
      block->parameter_set_aggregate_identity != descriptor->parameter_set_aggregate_identity ||
      block->weights_uploaded == 0u || block->quarantined != 0u ||
      block->binding_state != PROM_NOISE_REFINER_BINDING_BOUND ||
      session->created == 0u || session->session_id != request->session_identity ||
      session->lock_identity != request->lock_identity || session->quarantined != 0u ||
      image == NULL || context == NULL || joint == NULL ||
      image->valid == 0u || context->valid == 0u || joint->valid == 0u ||
      image->generation != request->required_image_generation ||
      context->generation != request->required_context_generation ||
      joint->generation != request->required_joint_generation ||
      session->joint_image_generation != image->generation ||
      session->joint_context_generation != context->generation ||
      joint->device.size != PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_MAIN_MODEL_ELEMENTS) ||
      image->device.size != PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS) ||
      context->device.size != PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_CONTEXT_MODEL_ELEMENTS) ||
      !prom_model_block_reap(state, block)) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_STALE_OUTPUT);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  block->last_active_target_validation_ns =
      prom_reduction_elapsed_ns(validation_begin_ns, prom_reduction_now_ns());
  block->output_valid = 0u;
  block->audit_valid = 0u;
  memcpy_begin_ns = prom_reduction_now_ns();
  memcpy(block->timestep_upload.mapped, request->timestep_bf16, (size_t)request->timestep_bytes);
  block->last_staging_memcpy_ns = prom_reduction_elapsed_ns(memcpy_begin_ns, prom_reduction_now_ns());
  begin_ns = prom_reduction_now_ns();
  if (!prom_main_transformer_record_execute(state, block, session, request->required_joint_generation,
                                            request->resident_chain_mode, &detail)) {
    prom_model_block_mark_failure(block, detail);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  if (block->m1b_timestamp_supported != 0u &&
      !prom_main_transformer_resolve_timestamps(state, block)) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_AUDIT_FAILED);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  block->output_valid = 1u;
  block->output_generation += 1u;
  if (request->resident_chain_mode != 0u) {
    joint->generation += 1u;
    joint->producer_block_id = block_id;
    joint->producer_output_generation = block->output_generation;
  }
  block->execution_count += 1u;
  block->last_execution_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  block->replay_identity = prom_model_block_hash_u64(request->lock_identity, request->session_identity);
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, request->required_image_generation);
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, request->required_context_generation);
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, request->required_joint_generation);
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, block->parameter_set_aggregate_identity);
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, request->timestep_identity);
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, request->output_identity);
  session->active_block_id = block_id;
  session->binding_generation = block->binding_generation;
  session->replay_identity = prom_model_block_hash_u64(session->replay_identity, block->replay_identity);
  block->last_detail_code = 0;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
}

/* The static-audit entry point deliberately replays one completed, lock-bound
   MainTransformer invocation from the resident JointWorking generation.  The
   shared execute recorder owns the fixed kernel order and final gated-residual
   target; this wrapper adds the audit egress without creating an activation
   ingress or changing the session generation. */
int prom_reactor_runtime_main_transformer_execute_static_audit_impl(
    void* handle, uint64_t block_id, const PrometheusMainTransformerStaticAuditRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  prom_compiled_model_session_state* session;
  prom_compiled_model_stream_slot* image;
  prom_compiled_model_stream_slot* context;
  prom_compiled_model_stream_slot* joint;
  const PrometheusMainTransformerResolvedDescriptor* descriptor;
  VkCommandBufferBeginInfo begin_info;
  VkBufferMemoryBarrier barrier;
  VkBufferCopy copy;
  uint64_t begin_ns;
  int32_t detail = PROM_MODEL_BLOCK_DETAIL_AUDIT_FAILED;
  const uint64_t output_bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_MAIN_MODEL_ELEMENTS);
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL ||
      request->struct_size != sizeof(*request) || request->lock_identity != PROM_ZIMAGE_TURBO_AUDIT_LOCK_ID ||
      request->model_local_block_id != 0u || request->session_identity == 0u ||
      request->required_image_generation == 0u || request->required_context_generation == 0u ||
      request->required_joint_generation == 0u || request->output_identity == 0u ||
      request->audit_arena == NULL || request->audit_arena_capacity_bytes < output_bytes) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  descriptor = prom_zimage_turbo_resolve_main_transformer_descriptor(request->lock_identity,
                                                                     request->model_local_block_id);
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (descriptor == NULL || state == NULL || state->model_block.created == 0u ||
      state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  session = &state->compiled_session;
  image = prom_compiled_session_slot(session, PROM_ZIMAGE_STREAM_PREPARED_IMAGE);
  context = prom_compiled_session_slot(session, PROM_ZIMAGE_STREAM_PREPARED_CONTEXT);
  joint = prom_compiled_session_slot(session, PROM_ZIMAGE_STREAM_JOINT_WORKING);
  if (!prom_model_block_is_main_transformer(block) ||
      block->parameter_set != descriptor->parameter_set ||
      block->parameter_set_aggregate_identity != descriptor->parameter_set_aggregate_identity ||
      block->weights_uploaded == 0u || block->quarantined != 0u ||
      block->binding_state != PROM_NOISE_REFINER_BINDING_BOUND ||
      session->created == 0u || session->session_id != request->session_identity ||
      session->lock_identity != request->lock_identity || session->quarantined != 0u ||
      image == NULL || context == NULL || joint == NULL || image->valid == 0u ||
      context->valid == 0u || joint->valid == 0u ||
      image->generation != request->required_image_generation ||
      context->generation != request->required_context_generation ||
      joint->generation != request->required_joint_generation ||
      session->joint_image_generation != image->generation ||
      session->joint_context_generation != context->generation || !prom_model_block_reap(state, block)) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_STALE_OUTPUT);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  block->output_valid = 0u;
  block->audit_valid = 0u;
  begin_ns = prom_reduction_now_ns();
  if (!prom_main_transformer_record_execute(state, block, session, request->required_joint_generation, 0u, &detail)) goto failure;
  if (vkResetCommandBuffer(block->command_buffer, 0u) != VK_SUCCESS) goto failure;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(block->command_buffer, &begin_info) != VK_SUCCESS) goto failure;
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = block->attention.buffer;
  barrier.size = output_bytes;
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  memset(&copy, 0, sizeof(copy));
  copy.size = output_bytes;
  vkCmdCopyBuffer(block->command_buffer, block->attention.buffer, block->audit_readback.buffer, 1u, &copy);
  if (vkEndCommandBuffer(block->command_buffer) != VK_SUCCESS ||
      !prom_model_block_submit_and_wait(state, block, &detail)) goto failure;
  memset(request->audit_arena, 0, (size_t)request->audit_arena_capacity_bytes);
  memcpy(request->audit_arena, block->audit_readback.mapped, (size_t)output_bytes);
  block->output_valid = 1u;
  block->audit_valid = 1u;
  block->output_generation += 1u;
  block->execution_count += 1u;
  block->last_execution_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, request->output_identity);
  block->last_detail_code = 0;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
failure:
  prom_model_block_mark_failure(block, detail == 0 ? PROM_MODEL_BLOCK_DETAIL_AUDIT_FAILED : detail);
  prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
  return PROM_ERROR;
}

int prom_reactor_runtime_main_transformer_audit_final_impl(
    void* handle, uint64_t block_id, const PrometheusMainTransformerFinalAuditRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  VkCommandBufferBeginInfo begin_info;
  VkBufferMemoryBarrier barrier;
  VkBufferCopy copy;
  const uint64_t output_bytes = PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_MAIN_MODEL_ELEMENTS);
  uint64_t readback_begin_ns;
  int32_t detail = PROM_MODEL_BLOCK_DETAIL_AUDIT_FAILED;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL || request->output == NULL ||
      request->struct_size != sizeof(*request) || request->output_identity == 0u ||
      request->required_output_generation == 0u || request->output_element_capacity < PROM_MODEL_BLOCK_MAIN_MODEL_ELEMENTS) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  if (!prom_model_block_is_main_transformer(block) || block->output_valid == 0u ||
      block->output_generation != request->required_output_generation || block->audit_readback.size < output_bytes ||
      !prom_model_block_reap(state, block)) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_STALE_OUTPUT);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  if (vkResetCommandBuffer(block->command_buffer, 0u) != VK_SUCCESS) goto failure;
  block->vk_reset_command_buffer_count += 1u;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(block->command_buffer, &begin_info) != VK_SUCCESS) goto failure;
  if (block->m1b_timestamp_supported != 0u) {
    vkCmdResetQueryPool(block->command_buffer, block->m1b_timestamp_query_pool, 0u, 2u);
    vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                        block->m1b_timestamp_query_pool, 0u);
  }
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = block->attention.buffer;
  barrier.size = output_bytes;
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  memset(&copy, 0, sizeof(copy));
  copy.size = output_bytes;
  vkCmdCopyBuffer(block->command_buffer, block->attention.buffer, block->audit_readback.buffer, 1u, &copy);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(
      block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
      block->m1b_timestamp_query_pool, 1u);
  if (vkEndCommandBuffer(block->command_buffer) != VK_SUCCESS ||
      !prom_model_block_submit_and_wait(state, block, &detail)) goto failure;
  if (block->m1b_timestamp_supported != 0u) {
    uint64_t timestamps[2u];
    if (vkGetQueryPoolResults(state->device, block->m1b_timestamp_query_pool, 0u, 2u,
                              sizeof(timestamps), timestamps, sizeof(uint64_t),
                              VK_QUERY_RESULT_64_BIT | VK_QUERY_RESULT_WAIT_BIT) != VK_SUCCESS) goto failure;
    block->gpu_readback_ns = (uint64_t)((double)(timestamps[1u] - timestamps[0u]) *
                                        (double)block->m1b_timestamp_period_ns + 0.5);
  }
  readback_begin_ns = prom_reduction_now_ns();
  memcpy(request->output, block->audit_readback.mapped, (size_t)output_bytes);
  block->last_output_readback_ns = prom_reduction_elapsed_ns(readback_begin_ns, prom_reduction_now_ns());
  block->audit_valid = 1u;
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, request->output_identity);
  block->last_detail_code = 0;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
failure:
  prom_model_block_mark_failure(block, detail == 0 ? PROM_MODEL_BLOCK_DETAIL_AUDIT_FAILED : detail);
  prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
  return PROM_ERROR;
}

int prom_reactor_runtime_compiled_model_session_get_evidence_impl(
    void* handle, uint64_t session_id, PrometheusCompiledModelSessionEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  if (out_evidence == NULL || !prom_reactor_runtime_validate_handle(handle)) return PROM_ERROR;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->compiled_session.created == 0u || state->compiled_session.session_id != session_id) {
    prom_compiled_session_fill_evidence(NULL, out_evidence);
    return PROM_ERROR;
  }
  prom_compiled_session_fill_evidence(&state->compiled_session, out_evidence);
  return PROM_OK;
}

int prom_reactor_runtime_compiled_model_session_destroy_impl(void* handle, uint64_t session_id) {
  prom_reduction_runtime_state* state;
  if (!prom_reactor_runtime_validate_handle(handle)) return PROM_ERROR;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->compiled_session.created == 0u || state->compiled_session.session_id != session_id) return PROM_ERROR;
  prom_compiled_model_session_cleanup_state(state);
  return PROM_OK;
}

int prom_reactor_runtime_model_block_test_inject_impl(void* handle, uint64_t block_id,
                                                      uint32_t reduction_test_flags) {
  prom_reduction_runtime_state* state;
  if (!prom_reactor_runtime_validate_handle(handle) || reduction_test_flags == 0u) return PROM_ERROR;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) return PROM_ERROR;
  state->reduction_test_flags |= reduction_test_flags;
  return PROM_OK;
}

int prom_reactor_runtime_model_block_test_inject_create_fault_impl(void* handle,
                                                                   uint32_t test_flags) {
  prom_reduction_runtime_state* state;
  int32_t detail = 0;
  if (!prom_reactor_runtime_validate_handle(handle) || test_flags == 0u ||
      (test_flags & ~(PROM_TESTCFG_FAIL_PIPELINE_CREATE | PROM_TESTCFG_FAIL_UPLOAD)) != 0u) return PROM_ERROR;
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL || state->model_block.created != 0u) return PROM_ERROR;
  state->model_block_create_test_flags |= test_flags;
  return PROM_OK;
}

int prom_reactor_runtime_model_block_test_inject_execution_fault_impl(void* handle,
                                                                      uint64_t block_id,
                                                                      uint32_t test_flags) {
  prom_reduction_runtime_state* state;
  if (!prom_reactor_runtime_validate_handle(handle) ||
      (test_flags != PROM_TESTCFG_FAIL_DOWNLOAD && test_flags != PROM_TESTCFG_FAIL_DISPATCH)) return PROM_ERROR;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) return PROM_ERROR;
  state->model_block.test_flags |= test_flags;
  return PROM_OK;
}

int prom_reactor_runtime_compiled_model_owner_create_impl(
    void* handle, const PrometheusNoiseRefinerRebindRequest* request, uint64_t* out_block_id,
    PrometheusModelBlockEvidence* out_evidence) {
  const PrometheusNoiseRefinerResolvedDescriptor* descriptor;
  PrometheusModelBlockCreateRequest closed;
  prom_reduction_runtime_state* state;
  uint32_t index;
  if (out_block_id != NULL) *out_block_id = 0u;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL || out_block_id == NULL ||
      request->struct_size != sizeof(*request) || request->model_local_block_id != 0u ||
      request->uploads == NULL || request->upload_count != PROM_MODEL_BLOCK_MAX_WEIGHTS) goto invalid;
  descriptor = prom_zimage_turbo_resolve_noise_refiner_descriptor(request->lock_identity, 0u);
  if (descriptor == NULL || descriptor->parameter_set != PROM_NOISE_REFINER_PARAMETER_SET_0) goto invalid;
  state = prom_reduction_ensure_state(handle, NULL);
  if (state == NULL || state->model_block.created != 0u || state->compiled_session.created == 0u ||
      state->compiled_session.lock_identity != request->lock_identity) goto invalid;
  if (state->compiled_session.requested_execution_profile == PROM_MODEL_EXECUTION_PROFILE_PREFETCH) {
    prom_vk_runtime_services services;
    if (prom_reactor_runtime_get_vk_services(handle, &services) == PROM_OK &&
        services.transfer_queue_available != 0u && services.transfer_queue != VK_NULL_HANDLE &&
        services.transfer_command_pool != VK_NULL_HANDLE &&
        services.transfer_queue_family_index != services.compute_queue_family_index) {
      state->model_block_create_prefetch = 1u;
      state->model_block_create_transfer_queue_family = services.transfer_queue_family_index;
      state->compiled_session.selected_execution_profile = PROM_MODEL_EXECUTION_PROFILE_PREFETCH;
      state->compiled_session.profile_fallback_reason = 0u;
    } else {
      state->compiled_session.selected_execution_profile = PROM_MODEL_EXECUTION_PROFILE_MINIMUM_MEMORY;
      state->compiled_session.profile_fallback_reason = 1u; /* no independent transfer family */
    }
  } else {
    state->compiled_session.selected_execution_profile = PROM_MODEL_EXECUTION_PROFILE_MINIMUM_MEMORY;
    state->compiled_session.profile_fallback_reason = 0u;
  }
  {
    prom_vk_runtime_services services;
    prom_main_attention_route_decision decision;
    const char* route_error = prom_reactor_runtime_get_vk_services(handle, &services) == PROM_OK
                                  ? prom_main_attention_route_select(
                                        state->compiled_session.requested_main_attention_route, &services,
                                        PROM_MODEL_BLOCK_MAIN_TOKENS, 30u, 128u, 11520u, 3840u,
                                        PROM_MODEL_BLOCK_MAIN_ATTENTION_SUBGROUP_OWNED32_GROUPS, &decision)
                                  : "missing Vulkan 1.4 runtime services";
    if (route_error != NULL) goto invalid;
    state->compiled_session.selected_main_attention_route = decision.selected_route;
    state->compiled_session.main_attention_fallback_reason = decision.fallback_reason;
    state->compiled_session.main_attention_shader_id = decision.shader_id;
  }
  memset(&closed, 0, sizeof(closed));
  closed.struct_size = sizeof(closed);
  closed.assembly_family = descriptor->assembly_family;
  closed.parameter_set = descriptor->parameter_set;
  closed.parameter_set_aggregate_identity = descriptor->parameter_set_aggregate_identity;
  closed.model_contract_identity = descriptor->lock_identity;
  closed.weight_identity = descriptor->parameter_set_aggregate_identity;
  closed.shader_portfolio_identity = descriptor->execution_plan_identity;
  closed.precision_policy_identity = descriptor->lock_identity;
  closed.capability_route_identity = descriptor->canonical_authority_identity == 0u
                                        ? descriptor->lock_identity : descriptor->canonical_authority_identity;
  closed.memory_ceiling_bytes = descriptor->memory_plan_identity;
  closed.external_input_bytes = PROM_MODEL_BLOCK_M1B_BF16_BYTES(PROM_MODEL_BLOCK_M1B_MODEL_ELEMENTS);
  closed.audit_bytes = PROM_ZIMAGE_TURBO_AUDIT_ARENA_BYTES;
  closed.shader_id = PROM_MODEL_BLOCK_M1B_ADALN_SHADER_ID;
  closed.weight_count = PROM_MODEL_BLOCK_MAX_WEIGHTS;
  closed.step_count = PROM_MODEL_BLOCK_MAX_STEPS;
  memcpy(closed.steps, k_prom_model_block_steps, sizeof(closed.steps));
  for (index = 0u; index < closed.weight_count; ++index) {
    closed.weights[index].content_identity = request->uploads[index].content_identity;
    closed.weights[index].layout_identity = request->uploads[index].layout_identity;
    closed.weights[index].byte_count = request->uploads[index].byte_count;
  }
  state->model_block_create_shared_owner = 1u;
  if (prom_reactor_runtime_model_block_create_impl(handle, &closed, out_block_id, out_evidence) != PROM_OK) return PROM_ERROR;
  if (prom_reactor_runtime_model_block_upload_weights_impl(handle, *out_block_id, request->uploads,
                                                           request->upload_count, out_evidence) != PROM_OK) {
    prom_reactor_runtime_model_block_destroy_impl(handle, *out_block_id);
    *out_block_id = 0u;
    return PROM_ERROR;
  }
  state->compiled_session.evaluation_generation = 1u;
  state->compiled_session.retarget_position = 1u;
  state->compiled_session.evaluation_complete = 0u;
  return PROM_OK;
invalid:
  prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
  return PROM_ERROR;
}

int prom_reactor_runtime_compiled_model_retarget_impl(
    void* handle, const PrometheusCompiledModelRetargetRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  prom_compiled_model_session_state* session;
  uint32_t family;
  uint32_t parameter_set;
  uint64_t aggregate;
  uint32_t expected_count;
  uint32_t index;
  uint64_t memcpy_total_ns = 0u;
  const uint64_t* expected_bytes;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL ||
      request->struct_size != sizeof(*request) || request->uploads == NULL ||
      request->lock_identity != PROM_ZIMAGE_TURBO_LOCK_ID) goto invalid;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL) goto invalid;
  block = &state->model_block;
  session = &state->compiled_session;
  if (block->created == 0u || block->shared_owner == 0u || session->created == 0u ||
      session->session_id != request->session_identity || session->lock_identity != request->lock_identity ||
      session->quarantined != 0u || block->quarantined != 0u ||
      block->binding_state != PROM_NOISE_REFINER_BINDING_BOUND || !prom_model_block_reap(state, block)) goto failed;
  if (session->retarget_position == 0u) {
    family = PROM_NOISE_REFINER_FAMILY_Z_IMAGE_TURBO; parameter_set = PROM_NOISE_REFINER_PARAMETER_SET_0;
    aggregate = prom_noise_refiner_expected_aggregate(parameter_set); expected_count = PROM_MODEL_BLOCK_MAX_WEIGHTS;
    expected_bytes = k_prom_model_block_m1b_weight_bytes;
    if (request->model_local_block_id != 0u ||
        prom_zimage_turbo_resolve_noise_refiner_descriptor(request->lock_identity, request->model_local_block_id) == NULL) goto invalid;
  } else if (session->retarget_position == 1u) {
    family = PROM_NOISE_REFINER_FAMILY_Z_IMAGE_TURBO; parameter_set = PROM_NOISE_REFINER_PARAMETER_SET_1;
    aggregate = prom_noise_refiner_expected_aggregate(parameter_set); expected_count = PROM_MODEL_BLOCK_MAX_WEIGHTS;
    expected_bytes = k_prom_model_block_m1b_weight_bytes;
    if (request->model_local_block_id != 1u ||
        prom_zimage_turbo_resolve_noise_refiner_descriptor(request->lock_identity, request->model_local_block_id) == NULL) goto invalid;
  } else if (session->retarget_position == 2u) {
    family = PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO; parameter_set = PROM_CONTEXT_REFINER_PARAMETER_SET_0;
    aggregate = prom_context_refiner_expected_aggregate(parameter_set); expected_count = 11u;
    expected_bytes = k_prom_model_block_context_weight_bytes;
    if (request->model_local_block_id != 0u ||
        prom_zimage_turbo_resolve_context_refiner_descriptor(request->lock_identity, request->model_local_block_id) == NULL) goto invalid;
  } else if (session->retarget_position == 3u) {
    family = PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO; parameter_set = PROM_CONTEXT_REFINER_PARAMETER_SET_1;
    aggregate = prom_context_refiner_expected_aggregate(parameter_set); expected_count = 11u;
    expected_bytes = k_prom_model_block_context_weight_bytes;
    if (request->model_local_block_id != 1u ||
        prom_zimage_turbo_resolve_context_refiner_descriptor(request->lock_identity, request->model_local_block_id) == NULL) goto invalid;
  } else if (session->retarget_position >= 4u && session->retarget_position < 34u) {
    uint32_t layer = session->retarget_position - 4u;
    const PrometheusMainTransformerResolvedDescriptor* descriptor;
    descriptor = prom_zimage_turbo_resolve_main_transformer_descriptor(request->lock_identity, request->model_local_block_id);
    if (request->model_local_block_id != layer || descriptor == NULL || descriptor->parameter_set != layer + 1u) goto invalid;
    family = PROM_MAIN_TRANSFORMER_FAMILY_Z_IMAGE_TURBO; parameter_set = layer + 1u;
    aggregate = prom_main_transformer_expected_aggregate(parameter_set); expected_count = PROM_MODEL_BLOCK_MAX_WEIGHTS;
    expected_bytes = k_prom_model_block_m1b_weight_bytes;
  } else goto invalid;
  if (aggregate == 0u || request->upload_count != expected_count) goto invalid;
  for (index = 0u; index < expected_count; ++index) {
    const PrometheusModelBlockWeightUpload* upload = &request->uploads[index];
    uint32_t physical = family == PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO
                            ? k_prom_context_shared_weight_slots[index] : index;
    if (upload->binding_index != index || upload->bytes == NULL || upload->content_identity == 0u ||
        upload->layout_identity == 0u || upload->byte_count != expected_bytes[index] ||
        upload->byte_count > (uint64_t)block->weight_upload.size ||
        upload->byte_count > (uint64_t)block->weights[physical].device.size) goto invalid;
  }
  if ((block->test_flags & PROM_TESTCFG_FAIL_PIPELINE_CREATE) != 0u) goto descriptor_failed;
  block->binding_state = PROM_NOISE_REFINER_BINDING_UPLOADING;
  for (index = 0u; index < expected_count; ++index) {
    const PrometheusModelBlockWeightUpload* upload = &request->uploads[index];
    uint32_t physical = family == PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO
                            ? k_prom_context_shared_weight_slots[index] : index;
    uint64_t memcpy_begin_ns = prom_reduction_now_ns();
    memcpy(block->weight_upload.mapped, upload->bytes, (size_t)upload->byte_count);
    memcpy_total_ns += prom_reduction_elapsed_ns(memcpy_begin_ns, prom_reduction_now_ns());
    block->weights[physical].content_identity = upload->content_identity;
    block->weights[physical].layout_identity = upload->layout_identity;
    block->weights[physical].byte_count = upload->byte_count;
    if (!prom_model_block_record_upload(state, block, block->weights, physical)) goto upload_failed;
    block->weights[physical].uploaded = 1u;
    block->weight_upload_count += 1u;
  }
  block->assembly_family = family;
  block->parameter_set = parameter_set;
  block->parameter_set_aggregate_identity = aggregate;
  block->weight_count = expected_count;
  prom_model_block_copy_active_portfolio(block, family, 0);
  block->binding_state = PROM_NOISE_REFINER_BINDING_UPDATING_DESCRIPTORS;
  if (!prom_model_block_update_weight_descriptors(state, block, block->weights)) goto descriptor_failed;
  block->binding_generation += 1u;
  block->descriptor_generation += 1u;
  block->retarget_count += 1u;
  block->weights_uploaded = 1u;
  block->output_valid = 0u;
  block->audit_valid = 0u;
  block->resident_input_generation = 0u;
  block->replay_identity = 0u;
  block->binding_state = PROM_NOISE_REFINER_BINDING_BOUND;
  block->last_detail_code = 0;
  block->last_staging_memcpy_ns = memcpy_total_ns;
  session->retarget_position += 1u;
  session->active_block_id = block->block_id;
  session->binding_generation = block->binding_generation;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
upload_failed:
  block->binding_state = block->quarantined != 0u ? PROM_NOISE_REFINER_BINDING_QUARANTINED : PROM_NOISE_REFINER_BINDING_FAILED_BEFORE_COMMIT;
  block->last_detail_code = block->quarantined != 0u ? PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN : PROM_MODEL_BLOCK_DETAIL_UPLOAD_FAILED;
  prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
  return PROM_ERROR;
descriptor_failed:
  block->binding_state = PROM_NOISE_REFINER_BINDING_FAILED_BEFORE_COMMIT;
  block->last_detail_code = PROM_MODEL_BLOCK_DETAIL_DESCRIPTOR_UPDATE_FAILED;
  prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
  return PROM_ERROR;
failed:
  if (state != NULL && state->model_block.created != 0u) {
    prom_model_block_mark_failure(&state->model_block, state->model_block.quarantined != 0u
                                  ? PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN : PROM_MODEL_BLOCK_DETAIL_REBIND_FAILED);
    prom_model_block_fill_evidence(&state->model_block, state->model_block.last_detail_code, out_evidence);
  } else prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
  return PROM_ERROR;
invalid:
  prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
  return PROM_ERROR;
}

int prom_reactor_runtime_compiled_model_prefetch_impl(
    void* handle, const PrometheusCompiledModelPrefetchRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  prom_compiled_model_session_state* session;
  uint32_t family, parameter_set, expected_count, index;
  uint64_t aggregate;
  uint64_t memcpy_total_ns = 0u;
  const uint64_t* expected_bytes;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL ||
      request->struct_size != sizeof(*request) || request->uploads == NULL ||
      request->lock_identity != PROM_ZIMAGE_TURBO_LOCK_ID) goto invalid;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL) goto invalid;
  block = &state->model_block;
  session = &state->compiled_session;
  if (block->created == 0u || session->created == 0u || session->session_id != request->session_identity ||
      session->selected_execution_profile != PROM_MODEL_EXECUTION_PROFILE_PREFETCH ||
      block->prefetch_state != PROM_MODEL_WEIGHT_WINDOW_EMPTY || block->prefetch_queue == VK_NULL_HANDLE ||
      block->prefetch_weight_upload.mapped == NULL || block->quarantined != 0u) goto failed;
  if (session->retarget_position == 1u) {
    family = PROM_NOISE_REFINER_FAMILY_Z_IMAGE_TURBO; parameter_set = PROM_NOISE_REFINER_PARAMETER_SET_1;
    aggregate = prom_noise_refiner_expected_aggregate(parameter_set); expected_count = PROM_MODEL_BLOCK_MAX_WEIGHTS;
    expected_bytes = k_prom_model_block_m1b_weight_bytes;
    if (request->model_local_block_id != 1u ||
        prom_zimage_turbo_resolve_noise_refiner_descriptor(request->lock_identity, 1u) == NULL) goto invalid;
  } else if (session->retarget_position == 2u) {
    family = PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO; parameter_set = PROM_CONTEXT_REFINER_PARAMETER_SET_0;
    aggregate = prom_context_refiner_expected_aggregate(parameter_set); expected_count = 11u;
    expected_bytes = k_prom_model_block_context_weight_bytes;
    if (request->model_local_block_id != 0u ||
        prom_zimage_turbo_resolve_context_refiner_descriptor(request->lock_identity, 0u) == NULL) goto invalid;
  } else if (session->retarget_position == 3u) {
    family = PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO; parameter_set = PROM_CONTEXT_REFINER_PARAMETER_SET_1;
    aggregate = prom_context_refiner_expected_aggregate(parameter_set); expected_count = 11u;
    expected_bytes = k_prom_model_block_context_weight_bytes;
    if (request->model_local_block_id != 1u ||
        prom_zimage_turbo_resolve_context_refiner_descriptor(request->lock_identity, 1u) == NULL) goto invalid;
  } else if (session->retarget_position >= 4u && session->retarget_position < 34u) {
    uint32_t layer = session->retarget_position - 4u;
    const PrometheusMainTransformerResolvedDescriptor* descriptor =
        prom_zimage_turbo_resolve_main_transformer_descriptor(request->lock_identity, request->model_local_block_id);
    if (request->model_local_block_id != layer || descriptor == NULL || descriptor->parameter_set != layer + 1u) goto invalid;
    family = PROM_MAIN_TRANSFORMER_FAMILY_Z_IMAGE_TURBO; parameter_set = layer + 1u;
    aggregate = prom_main_transformer_expected_aggregate(parameter_set); expected_count = PROM_MODEL_BLOCK_MAX_WEIGHTS;
    expected_bytes = k_prom_model_block_m1b_weight_bytes;
  } else goto invalid;
  if (aggregate == 0u || request->upload_count != expected_count) goto invalid;
  block->prefetch_state = PROM_MODEL_WEIGHT_WINDOW_HOST_PREPARED;
  for (index = 0u; index < expected_count; ++index) {
    const PrometheusModelBlockWeightUpload* upload = &request->uploads[index];
    uint32_t physical = family == PROM_CONTEXT_REFINER_FAMILY_Z_IMAGE_TURBO
                            ? k_prom_context_shared_weight_slots[index] : index;
    if (upload->binding_index != index || upload->bytes == NULL || upload->content_identity == 0u ||
        upload->layout_identity == 0u || upload->byte_count != expected_bytes[index] ||
        upload->byte_count > (uint64_t)block->prefetch_weight_upload.size ||
        upload->byte_count > (uint64_t)block->prefetch_weights[physical].device.size) goto failed_before_submit;
    {
      uint64_t memcpy_begin_ns = prom_reduction_now_ns();
      memcpy(block->prefetch_weight_upload.mapped, upload->bytes, (size_t)upload->byte_count);
      memcpy_total_ns += prom_reduction_elapsed_ns(memcpy_begin_ns, prom_reduction_now_ns());
    }
    block->prefetch_weights[physical].content_identity = upload->content_identity;
    block->prefetch_weights[physical].layout_identity = upload->layout_identity;
    block->prefetch_weights[physical].byte_count = upload->byte_count;
    block->prefetch_state = PROM_MODEL_WEIGHT_WINDOW_UPLOAD_SUBMITTED;
    if (!prom_model_block_record_prefetch_upload(state, block, physical)) goto uncertain;
    block->prefetch_weights[physical].uploaded = 1u;
  }
  block->prefetch_weight_count = expected_count;
  block->prefetch_assembly_family = family;
  block->prefetch_parameter_set = parameter_set;
  block->prefetch_parameter_set_aggregate_identity = aggregate;
  block->prefetch_target_position = session->retarget_position;
  block->prefetch_generation += 1u;
  block->prefetch_state = PROM_MODEL_WEIGHT_WINDOW_DEVICE_READY;
  block->last_staging_memcpy_ns = memcpy_total_ns;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
failed_before_submit:
  block->prefetch_state = PROM_MODEL_WEIGHT_WINDOW_EMPTY;
  prom_model_block_fill_evidence(block, PROM_MODEL_BLOCK_DETAIL_WEIGHT_MISMATCH, out_evidence);
  return PROM_ERROR;
uncertain:
  block->prefetch_state = PROM_MODEL_WEIGHT_WINDOW_QUARANTINED;
  prom_model_block_fill_evidence(block, PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN, out_evidence);
  return PROM_ERROR;
failed:
  if (block != NULL) prom_model_block_fill_evidence(block, PROM_MODEL_BLOCK_DETAIL_REBIND_FAILED, out_evidence);
  else prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_REBIND_FAILED, out_evidence);
  return PROM_ERROR;
invalid:
  prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
  return PROM_ERROR;
}

int prom_reactor_runtime_compiled_model_activate_prefetch_impl(
    void* handle, uint64_t session_id, PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  prom_compiled_model_session_state* session;
  prom_model_block_weight_resource old_weights[PROM_MODEL_BLOCK_MAX_WEIGHTS];
  if (!prom_reactor_runtime_validate_handle(handle)) goto invalid;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL) goto invalid;
  block = &state->model_block;
  session = &state->compiled_session;
  if (block->created == 0u || session->created == 0u || session->session_id != session_id ||
      session->selected_execution_profile != PROM_MODEL_EXECUTION_PROFILE_PREFETCH ||
      block->prefetch_state != PROM_MODEL_WEIGHT_WINDOW_DEVICE_READY ||
      block->prefetch_target_position != session->retarget_position || block->quarantined != 0u) goto failed;
  /* The scoped bridge waits for current compute before activation.  The fence
     is checked again here so a stale or uncertain upload can never become the
     active descriptor set. */
  block->vk_fence_wait_count += 1u;
  if (vkWaitForFences(state->device, 1u, &block->prefetch_fence, VK_TRUE, UINT64_MAX) != VK_SUCCESS) goto uncertain;
  /* The window uses concurrent sharing across the dedicated transfer and
     compute families, so no ownership handoff is required.  A compute-queue
     acquire barrier is still mandatory before shader reads can observe the
     transfer writes. */
  if (!prom_model_block_acquire_prefetched_weights(state, block)) goto uncertain;
  memcpy(old_weights, block->weights, sizeof(old_weights));
  memcpy(block->weights, block->prefetch_weights, sizeof(block->weights));
  memcpy(block->prefetch_weights, old_weights, sizeof(block->prefetch_weights));
  block->assembly_family = block->prefetch_assembly_family;
  block->parameter_set = block->prefetch_parameter_set;
  block->parameter_set_aggregate_identity = block->prefetch_parameter_set_aggregate_identity;
  block->weight_count = block->prefetch_weight_count;
  prom_model_block_copy_active_portfolio(block, block->assembly_family, 0);
  if (!prom_model_block_update_weight_descriptors(state, block, block->weights)) goto descriptor_failed;
  block->binding_generation += 1u;
  block->descriptor_generation += 1u;
  block->prefetch_descriptor_generation = block->descriptor_generation;
  block->retarget_count += 1u;
  block->weights_uploaded = 1u;
  block->output_valid = 0u;
  block->audit_valid = 0u;
  block->resident_input_generation = 0u;
  block->replay_identity = 0u;
  block->active_weight_window ^= 1u;
  block->prefetch_state = PROM_MODEL_WEIGHT_WINDOW_EMPTY;
  block->prefetch_weight_count = 0u;
  session->retarget_position += 1u;
  session->active_block_id = block->block_id;
  session->binding_generation = block->binding_generation;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
uncertain:
  block->prefetch_state = PROM_MODEL_WEIGHT_WINDOW_QUARANTINED;
  prom_model_block_fill_evidence(block, PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN, out_evidence);
  return PROM_ERROR;
descriptor_failed:
  memcpy(block->prefetch_weights, block->weights, sizeof(block->prefetch_weights));
  memcpy(block->weights, old_weights, sizeof(block->weights));
  block->prefetch_state = PROM_MODEL_WEIGHT_WINDOW_DEVICE_READY;
  prom_model_block_fill_evidence(block, PROM_MODEL_BLOCK_DETAIL_DESCRIPTOR_UPDATE_FAILED, out_evidence);
  return PROM_ERROR;
failed:
  if (state != NULL && state->model_block.created != 0u)
    prom_model_block_fill_evidence(&state->model_block, PROM_MODEL_BLOCK_DETAIL_REBIND_FAILED, out_evidence);
  else prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_REBIND_FAILED, out_evidence);
  return PROM_ERROR;
invalid:
  prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
  return PROM_ERROR;
}

int prom_reactor_runtime_compiled_model_evaluation_reset_impl(
    void* handle, uint64_t session_id, PrometheusCompiledModelSessionEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_compiled_model_session_state* session;
  prom_model_block_state* block;
  uint32_t index;
  if (!prom_reactor_runtime_validate_handle(handle)) goto invalid;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL) goto invalid;
  session = &state->compiled_session;
  block = &state->model_block;
  if (session->created == 0u || session->session_id != session_id || block->created == 0u ||
      block->shared_owner == 0u || session->quarantined != 0u || !prom_model_block_reap(state, block)) goto invalid;
  session->evaluation_generation += 1u;
  session->retarget_position = 0u;
  session->evaluation_complete = 0u;
  session->active_block_id = 0u;
  session->binding_generation = 0u;
  session->replay_identity = 0u;
  session->joint_image_generation = 0u;
  session->joint_context_generation = 0u;
  for (index = 0u; index < PROM_ZIMAGE_STREAM_SLOT_COUNT; ++index) {
    session->streams[index].valid = 0u;
    session->streams[index].producer_block_id = 0u;
    session->streams[index].producer_output_generation = 0u;
  }
  block->output_valid = 0u;
  block->audit_valid = 0u;
  block->replay_identity = 0u;
  session->last_detail_code = 0;
  prom_compiled_session_fill_evidence(session, out_evidence);
  return PROM_OK;
invalid:
  prom_compiled_session_fill_evidence(NULL, out_evidence);
  return PROM_ERROR;
}
