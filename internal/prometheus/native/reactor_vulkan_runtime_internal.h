#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_VULKAN_RUNTIME_INTERNAL_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_VULKAN_RUNTIME_INTERNAL_H

#if !defined(_WIN32) && !defined(_POSIX_C_SOURCE)
#define _POSIX_C_SOURCE 200809L
#endif

/* Temporary M0 migration seam. The historical runtime and slot records are
   structurally mixed; this header makes that debt explicit while the reduction
   implementation is extracted from the transformer remainder. */
#include "reactor_vulkan.h"
#include <string.h>

#if defined(_WIN32)
#ifndef WIN32_LEAN_AND_MEAN
#define WIN32_LEAN_AND_MEAN 1
#endif
#include <windows.h>
#else
#include <time.h>
#endif

/* M39b is a row-wise FP32 reactor.  All dispatches for one request are recorded
   into one slot-owned command buffer and submitted behind one fence. */

#define PROM_REDUCTION_STATE_MAGIC 0x52333942u
#define PROM_REDUCTION_RING_MAX_DEPTH 4u
#define PROM_REDUCTION_PIPELINE_COUNT 7u
#define PROM_REDUCTION_MIN_BINDING_BYTES ((VkDeviceSize)sizeof(float))
#define PROM_REDUCTION_QUERY_STRIDE 1024u
#define PROM_M40B_QUERY_COUNT 8u
#define PROM_M40B_SGEMM_PIPELINE_COUNT 3u
#define PROM_M40B_COOPERATIVE_SHADER_HASH 0x247e410eb526f25cull
#define PROM_M42_DESCRIPTOR_SET_COUNT 15u
#define PROM_M42_PIPELINE_COUNT 3u
#define PROM_M42_QUERY_COUNT 26u
#define PROM_M42_MAX_TOKENS 1024u
#define PROM_M42_MAX_MODEL_WIDTH 4096u
#define PROM_M42_MAX_HEAD_DIM 1024u
#define PROM_M42_MAX_MATRIX_ELEMENTS 16777216ull
#define PROM_M42_PACK_SHADER_HASH 0xd5c3fd6a2af8965full
#define PROM_M42_TRANSPOSE_SHADER_HASH 0xfb812063431bc816ull
#define PROM_M42_SCALE_SHADER_HASH 0x083d00ecc944435full
#define PROM_M43_DESCRIPTOR_SET_COUNT (PROM_M43_HEAD_COUNT * PROM_M42_DESCRIPTOR_SET_COUNT + 1u)
#define PROM_M43_QUERY_COUNT 199u
#define PROM_M43_QUERY_HEAD_BASE 4u
#define PROM_M43_QUERY_HEAD_STRIDE 24u
#define PROM_M43_QUERY_GROUP_END 196u
#define PROM_M43_QUERY_READBACK_BEGIN 197u
#define PROM_M43_QUERY_READBACK_END 198u
#define PROM_M43_CAPACITY_LIMIT_BYTES (512ull * 1024ull * 1024ull)
#define PROM_M44_COMMON_DESCRIPTOR_SET_COUNT 2u
#define PROM_M44_WIDE_DESCRIPTOR_BINDING_COUNT 10u
#define PROM_M44_PIPELINE_COUNT 2u
#define PROM_M44_QUERY_BASE 199u
#define PROM_M44_QUERY_AGGREGATION_BEGIN 199u
#define PROM_M44_QUERY_AGGREGATION_END 200u
#define PROM_M44_QUERY_PROJECTION_BEGIN 201u
#define PROM_M44_QUERY_PROJECTION_END 202u
#define PROM_M44_QUERY_ACCUMULATION_BEGIN 203u
#define PROM_M44_QUERY_ACCUMULATION_END 204u
#define PROM_M44_QUERY_READBACK_BEGIN 205u
#define PROM_M44_QUERY_READBACK_END 206u
#define PROM_M44_QUERY_COUNT 8u
#define PROM_M44_TOTAL_QUERY_COUNT 207u
#define PROM_M44_CAPACITY_LIMIT_BYTES (1024ull * 1024ull * 1024ull)
#define PROM_M44_INTERLEAVE_SHADER_HASH 0xef5bd1d4aac8cce9ull
#define PROM_M44_DIRECT_SHADER_HASH 0x1f2c7914051d28c2ull
#define PROM_M45_PIPELINE_COUNT 1u
#define PROM_M45_QUERY_BASE 207u
#define PROM_M45_QUERY_RESIDUAL_BEGIN 207u
#define PROM_M45_QUERY_RESIDUAL_END 208u
#define PROM_M45_QUERY_READBACK_BEGIN 209u
#define PROM_M45_QUERY_READBACK_END 210u
#define PROM_M45_QUERY_COUNT 4u
#define PROM_M45_TOTAL_QUERY_COUNT 211u
#define PROM_M45_CAPACITY_LIMIT_BYTES (1024ull * 1024ull * 1024ull)
#define PROM_M45_RESIDUAL_SHADER_HASH 0xc6e177b9fb86f1e5ull
#define PROM_M46_PIPELINE_COUNT 2u
#define PROM_M46_CAPACITY_LIMIT_BYTES (1024ull * 1024ull * 1024ull)
#define PROM_M46_REDUCE_SHADER_HASH 0xa9af5205bcd60571ull
#define PROM_M46_APPLY_SHADER_HASH 0x78750b592ac5470eull
#define PROM_M46_QUERY_BASE 211u
#define PROM_M46_QUERY_REDUCTION_BEGIN 211u
#define PROM_M46_QUERY_PARTIAL_END 212u
#define PROM_M46_QUERY_FINAL_END 213u
#define PROM_M46_QUERY_APPLY_BEGIN 214u
#define PROM_M46_QUERY_APPLY_END 215u
#define PROM_M46_QUERY_READBACK_BEGIN 216u
#define PROM_M46_QUERY_READBACK_END 217u
#define PROM_M46_QUERY_COUNT 7u
#define PROM_M47_PIPELINE_COUNT 2u
#define PROM_M47_DESCRIPTOR_SET_COUNT 7u
#define PROM_M47_QUERY_BASE 218u
#define PROM_M47_QUERY_PACK_N_BEGIN 218u
#define PROM_M47_QUERY_PACK_N_END 219u
#define PROM_M47_QUERY_GATE_BEGIN 220u
#define PROM_M47_QUERY_GATE_END 221u
#define PROM_M47_QUERY_UP_BEGIN 222u
#define PROM_M47_QUERY_UP_END 223u
#define PROM_M47_QUERY_ACTIVATION_BEGIN 224u
#define PROM_M47_QUERY_ACTIVATION_END 225u
#define PROM_M47_QUERY_MULTIPLY_BEGIN 226u
#define PROM_M47_QUERY_MULTIPLY_END 227u
#define PROM_M47_QUERY_HIDDEN_PACK_BEGIN 228u
#define PROM_M47_QUERY_HIDDEN_PACK_END 229u
#define PROM_M47_QUERY_DOWN_BEGIN 230u
#define PROM_M47_QUERY_DOWN_END 231u
#define PROM_M47_QUERY_RESIDUAL_BEGIN 232u
#define PROM_M47_QUERY_RESIDUAL_END 233u
#define PROM_M47_QUERY_READBACK_BEGIN 234u
#define PROM_M47_QUERY_READBACK_END 235u
#define PROM_M47_QUERY_COUNT 18u
#define PROM_M48_STANDARD_DESCRIPTOR_SET_COUNT \
  (PROM_M43_DESCRIPTOR_SET_COUNT + PROM_M44_COMMON_DESCRIPTOR_SET_COUNT + 3u + \
   PROM_M47_DESCRIPTOR_SET_COUNT)
#define PROM_M48_COMMAND_BUFFER_COUNT PROM_M48_LAYER_COUNT
#define PROM_M48_SEMAPHORE_COUNT (PROM_M48_LAYER_COUNT - 1u)
#define PROM_STANDARD_DESCRIPTOR_SETS_PER_SLOT \
  (PROM_REDUCTION_MAX_STAGES + PROM_M42_DESCRIPTOR_SET_COUNT + \
   PROM_M43_DESCRIPTOR_SET_COUNT + PROM_M44_COMMON_DESCRIPTOR_SET_COUNT + \
   PROM_M47_DESCRIPTOR_SET_COUNT)
#define PROM_ALL_STANDARD_DESCRIPTOR_SETS_PER_SLOT \
  (PROM_STANDARD_DESCRIPTOR_SETS_PER_SLOT + \
   PROM_M48_LAYER_COUNT * PROM_M48_STANDARD_DESCRIPTOR_SET_COUNT)
#define PROM_ALL_WIDE_DESCRIPTOR_SETS_PER_SLOT (1u + PROM_M48_LAYER_COUNT)

#include "reactor_vulkan_transformer_internal.h"

typedef struct prom_m40b_sgemm_push_constants {
  uint32_t m;
  uint32_t n;
  uint32_t k;
} prom_m40b_sgemm_push_constants;

typedef struct prom_m42_pack_push_constants {
  uint32_t logical_rows;
  uint32_t logical_columns;
  uint32_t input_row_stride;
  uint32_t output_rows;
  uint32_t output_columns;
  uint32_t transpose;
  uint32_t packed_word_count;
  uint32_t reserved;
} prom_m42_pack_push_constants;

typedef struct prom_m42_transpose_push_constants {
  uint32_t input_rows;
  uint32_t input_columns;
  uint32_t input_row_stride;
  uint32_t output_row_stride;
  uint32_t output_element_count;
  uint32_t reserved0;
  uint32_t reserved1;
  uint32_t reserved2;
} prom_m42_transpose_push_constants;

typedef struct prom_m42_scale_push_constants {
  uint32_t rows;
  uint32_t columns;
  uint32_t row_stride;
  uint32_t total_elements;
  float scale;
  uint32_t reserved0;
  uint32_t reserved1;
  uint32_t reserved2;
} prom_m42_scale_push_constants;

typedef struct prom_m44_interleave_push_constants {
  uint32_t tokens;
  uint32_t head_dim;
  uint32_t head_row_stride;
  uint32_t output_rows;
  uint32_t output_columns;
  uint32_t packed_output;
  uint32_t work_item_count;
  uint32_t reserved;
} prom_m44_interleave_push_constants;

typedef struct prom_m44_direct_push_constants {
  uint32_t tokens;
  uint32_t head_dim;
  uint32_t model_width;
  uint32_t head_row_stride;
  uint32_t padded_model_width;
  uint32_t total_output_elements;
  uint32_t reserved0;
  uint32_t reserved1;
} prom_m44_direct_push_constants;

typedef struct prom_m45_residual_push_constants {
  uint32_t tokens;
  uint32_t model_width;
  uint32_t x_row_stride;
  uint32_t y_row_stride;
  uint32_t z_row_stride;
  uint32_t logical_element_count;
  uint32_t reserved0;
  uint32_t reserved1;
} prom_m45_residual_push_constants;

typedef struct prom_m46_reduce_push_constants {
  uint32_t row_count;
  uint32_t elements_per_row;
  uint32_t partials_per_row;
  uint32_t input_row_stride;
  uint32_t chunk_elements;
  uint32_t total_elements;
  uint32_t stage_role;
  float epsilon;
} prom_m46_reduce_push_constants;

typedef struct prom_m46_apply_push_constants {
  uint32_t tokens;
  uint32_t model_width;
  uint32_t z_row_stride;
  uint32_t n_row_stride;
  uint32_t logical_element_count;
  uint32_t reserved0;
  uint32_t reserved1;
  uint32_t reserved2;
} prom_m46_apply_push_constants;

typedef struct prom_m47_gate_push_constants {
  uint32_t logical_rows;
  uint32_t logical_columns;
  uint32_t input_row_stride;
  uint32_t output_rows;
  uint32_t output_columns;
  uint32_t mode;
  uint32_t element_count;
  uint32_t reserved;
} prom_m47_gate_push_constants;

typedef struct prom_m47_gate_pack_push_constants {
  uint32_t logical_rows;
  uint32_t logical_columns;
  uint32_t input_row_stride;
  uint32_t output_rows;
  uint32_t output_columns;
  uint32_t mode;
  uint32_t packed_word_count;
  uint32_t reserved;
} prom_m47_gate_pack_push_constants;

typedef struct prom_reduction_push_constants {
  uint32_t row_count;
  uint32_t elements_per_row;
  uint32_t partials_per_row;
  uint32_t input_row_stride;
  uint32_t chunk_elements;
  uint32_t total_elements;
  uint32_t stage_role;
  uint32_t reserved;
} prom_reduction_push_constants;

typedef struct prom_reduction_pipeline {
  uint32_t shader_id;
  uint32_t implementation_id;
  VkShaderModule shader_module;
  VkPipeline pipeline;
} prom_reduction_pipeline;

typedef struct prom_m43_head_slot {
  prom_vk_buffer q;
  prom_vk_buffer k;
  prom_vk_buffer v;
  prom_vk_buffer q_packed;
  prom_vk_buffer k_transposed;
  prom_vk_buffer v_packed;
  prom_vk_buffer scores;
  prom_vk_buffer probabilities;
  prom_vk_buffer p_packed;
  prom_vk_buffer output;
} prom_m43_head_slot;

/* M1a owns exactly one compiled model-block vessel. It lives inside the
   established reactor state so command-pool, device, teardown, and quarantine
   policy remain centralized while the historical state split is completed. */
typedef struct prom_model_block_weight_resource {
  prom_vk_buffer device;
  uint64_t content_identity;
  uint64_t layout_identity;
  uint64_t byte_count;
  uint32_t uploaded;
} prom_model_block_weight_resource;

typedef struct prom_model_block_m1b_pipeline {
  uint32_t shader_id;
  uint32_t binding_count;
  uint32_t push_constant_bytes;
  VkDescriptorSetLayout descriptor_set_layout;
  VkDescriptorPool descriptor_pool;
  VkPipelineLayout pipeline_layout;
  VkDescriptorSet descriptor_set;
  prom_reduction_pipeline pipeline;
} prom_model_block_m1b_pipeline;

#define PROM_MODEL_BLOCK_M1C_PIPELINE_COUNT 3u
#define PROM_MODEL_BLOCK_M1D_PIPELINE_COUNT 4u
#define PROM_MODEL_BLOCK_AUDIT_SOURCE_COUNT 13u

typedef struct prom_model_block_state {
  uint64_t block_id;
  uint64_t next_block_id;
  uint64_t model_contract_identity;
  uint64_t weight_identity;
  uint64_t shader_portfolio_identity;
  uint64_t precision_policy_identity;
  uint64_t capability_route_identity;
  uint64_t memory_ceiling_bytes;
  uint64_t external_input_bytes;
  uint64_t external_output_bytes;
  uint64_t declared_audit_bytes;
  uint64_t execution_plan_identity;
  uint64_t replay_identity;
  uint64_t m1b_prefix_replay_identity;
  uint64_t parameter_set_aggregate_identity;
  uint64_t binding_generation;
  uint64_t output_generation;
  /* The resident chain owns a single immutable FP32 predecessor generation.
     Audit replays may consume the copied device-local input, but may never
     silently substitute a later block output for that predecessor. */
  uint64_t resident_input_generation;
  uint64_t descriptor_generation;
  uint32_t created;
  uint32_t weights_uploaded;
  uint32_t quarantined;
  uint32_t output_valid;
  uint32_t audit_valid;
  int32_t last_detail_code;
  uint32_t shader_id;
  uint32_t main_attention_shader_id;
  uint32_t test_flags;
  uint32_t weight_count;
  uint32_t step_count;
  uint32_t assembly_family;
  uint32_t parameter_set;
  uint32_t binding_state;
  uint32_t steps[PROM_MODEL_BLOCK_MAX_STEPS];
  uint64_t cold_buffer_allocation_count;
  uint64_t warm_buffer_allocation_count;
  uint64_t pipeline_create_count;
  uint64_t descriptor_set_count;
  uint64_t weight_upload_count;
  uint64_t execution_count;
  uint64_t last_execution_ns;
  VkDescriptorSetLayout descriptor_set_layout;
  VkDescriptorPool descriptor_pool;
  VkPipelineLayout pipeline_layout;
  VkDescriptorSet descriptor_set;
  VkCommandBuffer command_buffer;
  VkFence fence;
  VkQueryPool m1b_timestamp_query_pool;
  float m1b_timestamp_period_ns;
  uint32_t m1b_timestamp_supported;
  uint64_t m1b_boundary_gpu_ns[PROM_MODEL_BLOCK_M1B_PIPELINE_COUNT];
  uint64_t gpu_total_begin_tick;
  uint64_t gpu_total_end_tick;
  uint64_t gpu_total_ns;
  uint64_t gpu_compute_begin_tick;
  uint64_t gpu_compute_end_tick;
  uint64_t gpu_compute_ns;
  uint64_t gpu_ingress_transfer_ns;
  uint64_t gpu_joint_copy_ns;
  uint64_t gpu_readback_ns;
  uint64_t main_stage_gpu_begin_tick[PROM_MODEL_BLOCK_MAIN_STAGE_COUNT];
  uint64_t main_stage_gpu_end_tick[PROM_MODEL_BLOCK_MAIN_STAGE_COUNT];
  uint64_t main_stage_gpu_ns[PROM_MODEL_BLOCK_MAIN_STAGE_COUNT];
  uint64_t m6a_activation_pack_gpu_ns;
  uint64_t m6a_cooperative_execute_gpu_ns;
  uint64_t m6a_w3_segment_repack_gpu_ns;
  uint64_t last_active_target_validation_ns;
  uint64_t last_command_reset_ns;
  uint64_t last_command_begin_ns;
  uint64_t last_command_record_ns;
  uint64_t last_command_end_ns;
  uint64_t last_queue_submit_ns;
  uint64_t last_fence_wait_ns;
  uint64_t last_descriptor_update_ns;
  uint64_t last_staging_memcpy_ns;
  uint64_t last_output_readback_ns;
  uint64_t vk_create_buffer_count;
  uint64_t vk_destroy_buffer_count;
  uint64_t vk_allocate_memory_count;
  uint64_t vk_free_memory_count;
  uint64_t vk_create_shader_module_count;
  uint64_t vk_destroy_shader_module_count;
  uint64_t vk_create_compute_pipelines_count;
  uint64_t vk_allocate_descriptor_sets_count;
  uint64_t vk_update_descriptor_sets_count;
  uint64_t vk_create_command_pool_count;
  uint64_t vk_allocate_command_buffers_count;
  uint64_t vk_reset_command_buffer_count;
  uint64_t vk_queue_submit_count;
  uint64_t vk_fence_wait_count;
  uint64_t vk_timeline_wait_count;
  uint64_t vk_map_memory_count;
  uint64_t vk_unmap_memory_count;
  uint64_t vk_flush_count;
  uint64_t vk_invalidate_count;
  prom_reduction_pipeline pipeline;
  prom_vk_buffer input_upload;
  prom_vk_buffer input_bf16_device;
  prom_vk_buffer input_device;
  prom_vk_buffer resident_boundary_device;
  prom_vk_buffer output_device;
  prom_vk_buffer output_readback;
  prom_vk_buffer audit_device;
  prom_vk_buffer audit_readback;
  prom_vk_buffer weight_upload;
  prom_vk_buffer timestep_upload;
  prom_vk_buffer timestep_bf16_device;
  prom_vk_buffer timestep_device;
  prom_vk_buffer adaln_projection;
  prom_vk_buffer attention_scale;
  prom_vk_buffer attention_gate;
  prom_vk_buffer mlp_scale;
  prom_vk_buffer mlp_gate;
  prom_vk_buffer modulated;
  /* DVT2-M6A experimental-only allocation. Production builds leave it null. */
  prom_vk_buffer m6a_modulated_fp16;
  prom_vk_buffer m6a_w3_fp32;
  prom_vk_buffer m6a_raw_audit;
  prom_vk_buffer norm_audit;
  prom_vk_buffer qkv;
  prom_vk_buffer attention;
  prom_vk_buffer attention_projection;
  prom_vk_buffer attention_residual;
  /* A single host-visible, immutable FP32 vector of ones. ContextRefiner binds
     it to the shared physical modulation/gate ports, making those ports exact
     identity operations without introducing an AdaLN or a learned gate. */
  prom_vk_buffer context_unit;
  /* ContextRefiner W3 has 32*10240 FP32 elements and cannot alias the
     32*3840 attention output buffer used by the NoiseRefiner partition plan. */
  prom_vk_buffer context_w3;
  prom_model_block_m1b_pipeline m1b_pipelines[PROM_MODEL_BLOCK_M1B_PIPELINE_COUNT];
  prom_model_block_m1b_pipeline m1c_pipelines[PROM_MODEL_BLOCK_M1C_PIPELINE_COUNT];
  prom_model_block_m1b_pipeline m1d_pipelines[PROM_MODEL_BLOCK_M1D_PIPELINE_COUNT];
  prom_model_block_m1b_pipeline m6a_pack_pipeline;
  prom_model_block_m1b_pipeline m6a_w1_pipeline;
  prom_model_block_m1b_pipeline m6a_w3_pipeline;
  prom_model_block_m1b_pipeline audit_pipelines[PROM_MODEL_BLOCK_AUDIT_SOURCE_COUNT];
  prom_model_block_weight_resource weights[PROM_MODEL_BLOCK_MAX_WEIGHTS];
  /* A complete candidate bundle is uploaded here before the descriptor
     transaction.  It is never visible to a dispatch until commit. */
  prom_model_block_weight_resource pending_weights[PROM_MODEL_BLOCK_MAX_WEIGHTS];
  /* M2 owns exactly one inactive, complete weight window.  It is deliberately
     separate from the active descriptors and receives only the lock-derived
     immediate successor. */
  prom_model_block_weight_resource prefetch_weights[PROM_MODEL_BLOCK_MAX_WEIGHTS];
  prom_vk_buffer prefetch_weight_upload;
  VkCommandBuffer prefetch_command_buffer;
  VkFence prefetch_fence;
  VkQueue prefetch_queue;
  VkCommandPool prefetch_command_pool;
  uint32_t prefetch_queue_family;
  uint32_t active_weight_window;
  uint32_t prefetch_state;
  uint32_t prefetch_weight_count;
  uint32_t prefetch_assembly_family;
  uint32_t prefetch_parameter_set;
  uint64_t prefetch_parameter_set_aggregate_identity;
  uint64_t prefetch_generation;
  uint64_t prefetch_descriptor_generation;
  uint64_t prefetch_target_position;
  /* M1 keeps one physical owner while selecting one generated family view. */
  uint32_t shared_owner;
  uint64_t owner_construction_count;
  uint64_t owner_destruction_count;
  uint64_t retarget_count;
  uint64_t descriptor_update_count;
  prom_model_block_m1b_pipeline noise_m1b_pipelines[PROM_MODEL_BLOCK_M1B_PIPELINE_COUNT];
  prom_model_block_m1b_pipeline noise_m1c_pipelines[PROM_MODEL_BLOCK_M1C_PIPELINE_COUNT];
  prom_model_block_m1b_pipeline noise_m1d_pipelines[PROM_MODEL_BLOCK_M1D_PIPELINE_COUNT];
  prom_model_block_m1b_pipeline noise_audit_pipelines[PROM_MODEL_BLOCK_AUDIT_SOURCE_COUNT];
  prom_model_block_m1b_pipeline context_m1b_pipelines[PROM_MODEL_BLOCK_M1B_PIPELINE_COUNT];
  prom_model_block_m1b_pipeline context_m1c_pipelines[PROM_MODEL_BLOCK_M1C_PIPELINE_COUNT];
  prom_model_block_m1b_pipeline context_m1d_pipelines[PROM_MODEL_BLOCK_M1D_PIPELINE_COUNT];
  prom_model_block_m1b_pipeline context_audit_pipelines[PROM_MODEL_BLOCK_AUDIT_SOURCE_COUNT];
  prom_model_block_m1b_pipeline main_m1b_pipelines[PROM_MODEL_BLOCK_M1B_PIPELINE_COUNT];
  prom_model_block_m1b_pipeline main_m1c_pipelines[PROM_MODEL_BLOCK_M1C_PIPELINE_COUNT];
  prom_model_block_m1b_pipeline main_m1d_pipelines[PROM_MODEL_BLOCK_M1D_PIPELINE_COUNT];
} prom_model_block_state;

typedef struct prom_reduction_slot {
  uint32_t slot_id;
  uint32_t generation;
  uint32_t state;
  uint64_t logical_request_id;
  uint32_t active_query_base;
  VkCommandBuffer command_buffer;
  VkCommandBuffer consumer_command_buffer;
  VkFence fence;
  VkSemaphore producer_complete;
  VkCommandBuffer m48_command_buffers[PROM_M48_COMMAND_BUFFER_COUNT];
  VkSemaphore m48_semaphores[PROM_M48_SEMAPHORE_COUNT];
  prom_transformer_descriptor_bank m48_descriptors[PROM_M48_LAYER_COUNT];
  VkDescriptorSet descriptor_sets[PROM_REDUCTION_MAX_STAGES];
  prom_vk_buffer input;
  prom_vk_buffer output;
  prom_vk_buffer scratch;
  prom_vk_buffer row_max;
  prom_vk_buffer row_sum;
  prom_vk_buffer composed_a_upload;
  prom_vk_buffer composed_a;
  prom_vk_buffer composed_c;
  prom_vk_buffer composed_softmax_output;
  prom_vk_buffer composed_readback;
  VkDescriptorSet m42_descriptor_sets[PROM_M42_DESCRIPTOR_SET_COUNT];
  prom_vk_buffer m42_x_upload;
  prom_vk_buffer m42_x;
  prom_vk_buffer m42_x_packed;
  prom_vk_buffer m42_q;
  prom_vk_buffer m42_k;
  prom_vk_buffer m42_v;
  prom_vk_buffer m42_q_packed;
  prom_vk_buffer m42_k_transposed;
  prom_vk_buffer m42_v_packed;
  prom_vk_buffer m42_scores;
  prom_vk_buffer m42_probabilities;
  prom_vk_buffer m42_p_packed;
  prom_vk_buffer m42_output;
  prom_vk_buffer m42_readback;
  prom_vk_buffer m42_audit_readback;
  VkDescriptorSet m43_descriptor_sets[PROM_M43_DESCRIPTOR_SET_COUNT];
  prom_vk_buffer m43_x_upload;
  prom_vk_buffer m43_x_f32;
  prom_vk_buffer m43_x_f16;
  prom_m43_head_slot m43_head[PROM_M43_HEAD_COUNT];
  prom_vk_buffer m43_readback;
  VkDescriptorSet m44_sgemm_descriptor_set;
  VkDescriptorSet m45_descriptor_set;
  VkDescriptorSet m47_descriptor_sets[PROM_M47_DESCRIPTOR_SET_COUNT];
  VkDescriptorSet m44_descriptor_set;
  prom_vk_buffer m44_concat_upload;
  prom_vk_buffer m44_concat_f32;
  prom_vk_buffer m44_concat_f16;
  prom_vk_buffer m44_output;
  prom_vk_buffer m44_readback;
  prom_vk_buffer m45_output;
  prom_vk_buffer m45_x_readback;
  prom_vk_buffer m46_partials;
  prom_vk_buffer m46_inv_rms;
  prom_vk_buffer m46_output;
  prom_vk_buffer m46_readback;
  prom_vk_buffer m49a_m46_z;
  prom_vk_buffer m47_n_packed;
  prom_vk_buffer m47_gate;
  prom_vk_buffer m47_up;
  prom_vk_buffer m47_activated_gate;
  prom_vk_buffer m47_hidden;
  prom_vk_buffer m47_hidden_packed;
  prom_vk_buffer m47_down;
  prom_vk_buffer m47_output;
  prom_vk_buffer m47_readback;
  prom_vk_buffer m48_host_initial_upload;
  prom_vk_buffer m48_host_initial;
  prom_vk_buffer m48_activation[2u];
  prom_vk_buffer m48_readback;
  /* Fixed-size host-visible capture for the transformer controller.  It is
     owned by the stack slot and therefore cannot outlive a recycled slot. */
  prom_vk_buffer m49b_canary_readback;
  uint64_t composed_command_reuse_count;
  uint64_t m42_command_reuse_count;
  uint64_t m43_command_reuse_count;
  uint64_t m44_command_reuse_count;
  uint64_t m45_command_reuse_count;
  uint64_t m46_command_reuse_count;
  uint64_t m47_command_reuse_count;
  uint64_t m48_command_reuse_count;
} prom_reduction_slot;

typedef struct prom_compiled_model_stream_slot {
  prom_vk_buffer device;
  uint64_t generation;
  uint64_t producer_block_id;
  uint64_t producer_output_generation;
  uint32_t valid;
} prom_compiled_model_stream_slot;

/* One session, one serialized executor, and fixed lock-owned stream slots.
   This is intentionally neither a graph nor a string-keyed resource map. */
typedef struct prom_compiled_model_session_state {
  uint64_t session_id;
  uint64_t next_session_id;
  uint64_t lock_identity;
  uint64_t active_block_id;
  uint64_t binding_generation;
  uint64_t replay_identity;
  uint64_t joint_image_generation;
  uint64_t joint_context_generation;
  uint64_t cold_buffer_allocation_count;
  uint64_t warm_buffer_allocation_count;
  uint64_t composition_count;
  uint64_t evaluation_generation;
  uint32_t requested_execution_profile;
  uint32_t selected_execution_profile;
  uint32_t profile_fallback_reason;
  uint32_t requested_main_attention_route;
  uint32_t selected_main_attention_route;
  uint32_t main_attention_fallback_reason;
  uint32_t main_attention_shader_id;
  uint32_t retarget_position;
  uint32_t evaluation_complete;
  uint32_t created;
  uint32_t quarantined;
  int32_t last_detail_code;
  VkCommandBuffer command_buffer;
  VkFence fence;
  prom_compiled_model_stream_slot streams[PROM_ZIMAGE_STREAM_SLOT_COUNT];
} prom_compiled_model_session_state;

typedef struct prom_reduction_runtime_state {
  uint32_t magic;
  uint32_t initialized;
  uint32_t ring_depth;
  uint32_t acquire_cursor;
  uint64_t next_logical_request_id;
  VkPhysicalDevice physical_device;
  VkDevice device;
  VkQueue queue;
  VkCommandPool command_pool;
  VkDescriptorSetLayout descriptor_set_layout;
  VkDescriptorPool descriptor_pool;
  VkPipelineLayout pipeline_layout;
  VkDescriptorSetLayout m44_descriptor_set_layout;
  VkDescriptorPool m44_descriptor_pool;
  VkPipelineLayout m44_pipeline_layout;
  VkQueryPool query_pool;
  uint32_t timestamp_supported;
  float timestamp_period_ns;
  uint32_t reduction_test_flags;
  uint32_t model_block_create_test_flags;
  uint32_t model_block_create_shared_owner;
  uint32_t model_block_create_prefetch;
  uint32_t model_block_create_transfer_queue_family;
  prom_reduction_pipeline pipelines[PROM_REDUCTION_PIPELINE_COUNT];
  prom_reduction_pipeline m40b_sgemm_pipelines[PROM_M40B_SGEMM_PIPELINE_COUNT];
  prom_vk_buffer persistent_b_upload;
  prom_vk_buffer persistent_b;
  prom_vk_buffer resident_a_upload;
  prom_vk_buffer resident_a;
  prom_m40b_padding_plan persistent_b_padding;
  prom_m40b_padding_plan resident_a_padding;
  uint64_t persistent_b_generation;
  uint64_t resident_a_generation;
  uint32_t persistent_b_kernel;
  uint32_t resident_a_kernel;
  uint64_t m40b_buffer_grow_count;
  uint64_t m40b_buffer_reuse_count;
  uint64_t m40b_descriptor_update_count;
  uint64_t m40b_pipeline_create_count;
  prom_reduction_pipeline m42_pipelines[PROM_M42_PIPELINE_COUNT];
  prom_vk_buffer m42_weight_upload[3];
  prom_vk_buffer m42_weight_f32[3];
  prom_vk_buffer m42_weight_f16[3];
  prom_vk_buffer m42_resident_x_upload;
  prom_vk_buffer m42_resident_x_f32;
  prom_vk_buffer m42_resident_x_f16;
  uint64_t m42_weight_generation[3];
  uint64_t m42_weight_hash[3];
  uint32_t m42_weight_model_width;
  uint32_t m42_weight_head_dim;
  uint64_t m42_resident_x_generation;
  uint64_t m42_resident_x_hash;
  uint32_t m42_resident_x_tokens;
  uint32_t m42_resident_x_model_width;
  uint64_t m42_buffer_grow_count;
  uint64_t m42_buffer_reuse_count;
  uint64_t m42_descriptor_update_count;
  uint64_t m42_pipeline_create_count;
  prom_vk_buffer m43_weight_upload[PROM_M43_HEAD_COUNT][PROM_M43_WEIGHT_KIND_COUNT];
  prom_vk_buffer m43_weight_f32[PROM_M43_HEAD_COUNT][PROM_M43_WEIGHT_KIND_COUNT];
  prom_vk_buffer m43_weight_f16[PROM_M43_HEAD_COUNT][PROM_M43_WEIGHT_KIND_COUNT];
  uint64_t m43_weight_generation[PROM_M43_HEAD_COUNT][PROM_M43_WEIGHT_KIND_COUNT];
  uint64_t m43_weight_hash[PROM_M43_HEAD_COUNT][PROM_M43_WEIGHT_KIND_COUNT];
  uint32_t m43_weight_model_width[PROM_M43_HEAD_COUNT][PROM_M43_WEIGHT_KIND_COUNT];
  uint32_t m43_weight_head_dim[PROM_M43_HEAD_COUNT][PROM_M43_WEIGHT_KIND_COUNT];
  prom_vk_buffer m43_resident_x_upload;
  prom_vk_buffer m43_resident_x_f32;
  prom_vk_buffer m43_resident_x_f16;
  uint64_t m43_resident_x_generation;
  uint64_t m43_resident_x_hash;
  uint32_t m43_resident_x_tokens;
  uint32_t m43_resident_x_model_width;
  uint64_t m43_buffer_grow_count;
  uint64_t m43_buffer_reuse_count;
  uint64_t m43_descriptor_update_count;
  prom_reduction_pipeline m44_pipelines[PROM_M44_PIPELINE_COUNT];
  prom_vk_buffer m44_wo_upload;
  prom_vk_buffer m44_wo_f32;
  prom_vk_buffer m44_wo_f16;
  uint64_t m44_wo_generation;
  uint64_t m44_wo_hash;
  uint32_t m44_wo_head_dim;
  uint32_t m44_wo_model_width;
  uint64_t m44_buffer_grow_count;
  uint64_t m44_buffer_reuse_count;
  uint64_t m44_descriptor_update_count;
  uint64_t m44_pipeline_create_count;
  prom_reduction_pipeline m45_pipelines[PROM_M45_PIPELINE_COUNT];
  uint64_t m45_buffer_grow_count;
  uint64_t m45_buffer_reuse_count;
  uint64_t m45_descriptor_update_count;
  uint64_t m45_pipeline_create_count;
  prom_reduction_pipeline m46_pipelines[PROM_M46_PIPELINE_COUNT];
  prom_vk_buffer m46_weight_upload;
  prom_vk_buffer m46_weight;
  uint64_t m46_weight_generation;
  uint64_t m46_weight_hash;
  uint32_t m46_weight_model_width;
  uint64_t m46_buffer_grow_count;
  uint64_t m46_buffer_reuse_count;
  uint64_t m46_descriptor_update_count;
  uint64_t m46_pipeline_create_count;
  prom_reduction_pipeline m47_pipelines[PROM_M47_PIPELINE_COUNT];
  prom_vk_buffer m47_weight_upload[PROM_M47_WEIGHT_COUNT];
  prom_vk_buffer m47_weight_f32[PROM_M47_WEIGHT_COUNT];
  prom_vk_buffer m47_weight_f16[PROM_M47_WEIGHT_COUNT];
  uint64_t m47_weight_generation[PROM_M47_WEIGHT_COUNT];
  uint64_t m47_weight_hash[PROM_M47_WEIGHT_COUNT];
  uint32_t m47_weight_model_width[PROM_M47_WEIGHT_COUNT];
  uint32_t m47_weight_ffn_width[PROM_M47_WEIGHT_COUNT];
  uint64_t m47_buffer_grow_count;
  uint64_t m47_buffer_reuse_count;
  uint64_t m47_descriptor_update_count;
  uint64_t m47_pipeline_create_count;
  prom_transformer_layer_resources m48_layer[PROM_M48_LAYER_COUNT];
  prom_vk_buffer m48_initial_upload;
  prom_vk_buffer m48_initial_f32;
  uint64_t m48_initial_generation;
  uint64_t m48_initial_hash;
  uint32_t m48_initial_tokens;
  uint32_t m48_initial_model_width;
  uint64_t m48_buffer_grow_count;
  uint64_t m48_buffer_reuse_count;
  uint64_t m48_descriptor_update_count;
  prom_num_m49b_controller m49b_controller;

  prom_model_block_state model_block;
  prom_compiled_model_session_state compiled_session;
  uint64_t m49b_next_execution_index;
  uint32_t m49b_enabled;
  prom_reduction_slot slots[PROM_REDUCTION_RING_MAX_DEPTH];
  PrometheusReductionDiagnostics diagnostics;
} prom_reduction_runtime_state;

typedef struct prom_reduction_buffer_bindings {
  const prom_vk_buffer* input;
  const prom_vk_buffer* auxiliary0;
  const prom_vk_buffer* auxiliary1;
  const prom_vk_buffer* output;
} prom_reduction_buffer_bindings;

static int prom_reduction_take_test_flag(prom_reduction_runtime_state* state, uint32_t flag) {
  if ((state->reduction_test_flags & flag) == 0u) return 0;
  state->reduction_test_flags &= ~flag;
  return 1;
}

static uint64_t prom_reduction_now_ns(void) {
#if defined(_WIN32)
  static LARGE_INTEGER frequency;
  static uint32_t initialized = 0u;
  LARGE_INTEGER counter;
  if (initialized == 0u) {
    if (QueryPerformanceFrequency(&frequency) == 0) frequency.QuadPart = 0;
    initialized = 1u;
  }
  if (frequency.QuadPart <= 0 || QueryPerformanceCounter(&counter) == 0) return 0u;
  return (uint64_t)((counter.QuadPart * 1000000000ll) / frequency.QuadPart);
#else
  struct timespec value;
  if (clock_gettime(CLOCK_MONOTONIC, &value) != 0) return 0u;
  return ((uint64_t)value.tv_sec * 1000000000ull) + (uint64_t)value.tv_nsec;
#endif
}

static uint64_t prom_reduction_elapsed_ns(uint64_t begin, uint64_t end) {
  if (begin == 0u || end == 0u || end < begin) return 0u;
  return end - begin;
}

static uint32_t prom_reduction_ceil_div_u32(uint32_t value, uint32_t divisor) {
  return value / divisor + ((value % divisor) != 0u ? 1u : 0u);
}

static uint64_t prom_reduction_hash_u32(uint64_t hash, uint32_t value) {
  uint32_t byte_index;
  for (byte_index = 0u; byte_index < 4u; ++byte_index) {
    hash ^= (uint64_t)((value >> (byte_index * 8u)) & 0xffu);
    hash *= 1099511628211ull;
  }
  return hash;
}

static uint64_t prom_m40b_hash_u64(uint64_t hash, uint64_t value) {
  hash = prom_reduction_hash_u32(hash, (uint32_t)value);
  return prom_reduction_hash_u32(hash, (uint32_t)(value >> 32u));
}

static int prom_m40b_checked_product_u64(uint64_t left, uint64_t right, uint64_t* out) {
  if (out == NULL || (right != 0u && left > UINT64_MAX / right)) return 0;
  *out = left * right;
  return 1;
}

/* Reduction-owned operations used by the transformer recorder during the
   migration. They do not transfer buffer, descriptor, or slot ownership. */
prom_reduction_runtime_state* prom_reduction_ensure_state(void* handle, int32_t* out_detail);
int prom_reduction_ensure_buffer(prom_reduction_runtime_state* state, prom_vk_buffer* buffer,
                                 uint64_t required_bytes, VkBufferUsageFlags usage,
                                 VkMemoryPropertyFlags properties, int map_memory);
void prom_reduction_reap_slots(prom_reduction_runtime_state* state, uint32_t allow_wait);
prom_reduction_slot* prom_reduction_acquire_slot(prom_reduction_runtime_state* state,
                                                  uint64_t logical_request_id);
void prom_reduction_update_descriptor_set(prom_reduction_runtime_state* state, VkDescriptorSet descriptor_set,
                                          const prom_reduction_buffer_bindings* bindings);
void prom_reduction_record_barrier(VkCommandBuffer command_buffer);
int prom_reduction_find_nonfinite(const float* input, uint64_t count, uint64_t* out_index);
void prom_reduction_destroy_pipeline(VkDevice device, prom_reduction_pipeline* pipeline);
void prom_model_block_cleanup_state(prom_reduction_runtime_state* state);
void prom_compiled_model_session_cleanup_state(prom_reduction_runtime_state* state);
int prom_reactor_runtime_noise_refiner0_execute_impl(
    void* handle, uint64_t block_id, const PrometheusNoiseRefiner0ExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence);
int prom_reactor_runtime_noise_refiner1_execute_impl(
    void* handle, uint64_t block_id, const PrometheusNoiseRefiner1ExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence);
int prom_reactor_runtime_noise_refiner_rebind_impl(
    void* handle, uint64_t block_id, const PrometheusNoiseRefinerRebindRequest* request,
    PrometheusModelBlockEvidence* out_evidence);
int prom_reactor_runtime_noise_refiner_execute_resident_impl(
    void* handle, uint64_t block_id, const PrometheusNoiseRefinerResidentExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence);
int prom_reactor_runtime_noise_refiner_execute_static_audit_impl(
    void* handle, uint64_t block_id, const PrometheusNoiseRefinerStaticAuditRequest* request,
    PrometheusModelBlockEvidence* out_evidence);
int prom_reactor_runtime_noise_refiner_audit_final_impl(
    void* handle, uint64_t block_id, const PrometheusNoiseRefinerFinalAuditRequest* request,
    PrometheusModelBlockEvidence* out_evidence);
int prom_reactor_runtime_compiled_model_owner_create_impl(
    void* handle, const PrometheusNoiseRefinerRebindRequest* request, uint64_t* out_block_id,
    PrometheusModelBlockEvidence* out_evidence);
int prom_reactor_runtime_compiled_model_retarget_impl(
    void* handle, const PrometheusCompiledModelRetargetRequest* request,
    PrometheusModelBlockEvidence* out_evidence);
int prom_reactor_runtime_compiled_model_evaluation_reset_impl(
    void* handle, uint64_t session_id, PrometheusCompiledModelSessionEvidence* out_evidence);
int prom_reactor_runtime_context_refiner_create_impl(
    void* handle, const PrometheusContextRefinerCreateRequest* request, uint64_t* out_block_id,
    PrometheusModelBlockEvidence* out_evidence);
int prom_reactor_runtime_context_refiner_rebind_impl(
    void* handle, uint64_t block_id, const PrometheusContextRefinerRebindRequest* request,
    PrometheusModelBlockEvidence* out_evidence);
int prom_reactor_runtime_context_refiner0_execute_impl(
    void* handle, uint64_t block_id, const PrometheusContextRefiner0ExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence);
int prom_reactor_runtime_context_refiner_execute_resident_impl(
    void* handle, uint64_t block_id, const PrometheusContextRefinerResidentExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence);
int prom_reactor_runtime_context_refiner_execute_static_audit_impl(
    void* handle, uint64_t block_id, const PrometheusContextRefinerStaticAuditRequest* request,
    PrometheusModelBlockEvidence* out_evidence);
int prom_reactor_runtime_context_refiner_audit_final_impl(
    void* handle, uint64_t block_id, const PrometheusContextRefinerFinalAuditRequest* request,
    PrometheusModelBlockEvidence* out_evidence);
int prom_reactor_runtime_main_transformer_audit_final_impl(
    void* handle, uint64_t block_id, const PrometheusMainTransformerFinalAuditRequest* request,
    PrometheusModelBlockEvidence* out_evidence);
int prom_m40b_wait_all_slots(prom_reduction_runtime_state* state);
int prom_m40b_ensure_sgemm_pipeline(prom_reduction_runtime_state* state, uint32_t kernel);
int prom_m40b_pack_matrix(const float* values, uint32_t rows, uint32_t columns,
                          uint32_t padded_rows, uint32_t padded_columns, uint32_t kernel,
                          void** out_payload, size_t* out_bytes);
VkPipeline prom_reduction_pipeline_for_implementation(const prom_reduction_runtime_state* state,
                                                       uint32_t implementation_id);
void prom_reduction_stage_bindings_for_io(const prom_reduction_slot* slot,
                                          const PrometheusReductionPlan* plan, uint32_t stage_index,
                                          const prom_vk_buffer* operation_input,
                                          const prom_vk_buffer* operation_output,
                                          prom_reduction_buffer_bindings* bindings);

#endif
