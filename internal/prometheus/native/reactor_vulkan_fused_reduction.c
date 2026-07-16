#if !defined(_WIN32) && !defined(_POSIX_C_SOURCE)
#define _POSIX_C_SOURCE 200809L
#endif

#include "reactor_vulkan.h"
#include "reactor_shader_registry.h"
#include "../shaders/sdslv/experimental/sgemm/cooperative/sgemm_cooperative_f16_f32_m16n16k16_spirv.h"
#include "../shaders/sdslv/experimental/attention/attention_pack_f32_to_f16_spirv.h"
#include "../shaders/sdslv/experimental/attention/attention_transpose_f32_spirv.h"
#include "../shaders/sdslv/experimental/attention/attention_scale_scores_f32_spirv.h"

#include <math.h>
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
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
#define PROM_REDUCTION_PIPELINE_COUNT 5u
#define PROM_REDUCTION_MIN_BINDING_BYTES ((VkDeviceSize)sizeof(float))
#define PROM_REDUCTION_QUERY_STRIDE 32u
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

typedef struct prom_reduction_slot {
  uint32_t slot_id;
  uint32_t generation;
  uint32_t state;
  uint64_t logical_request_id;
  VkCommandBuffer command_buffer;
  VkCommandBuffer consumer_command_buffer;
  VkFence fence;
  VkSemaphore producer_complete;
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
  uint64_t composed_command_reuse_count;
  uint64_t m42_command_reuse_count;
} prom_reduction_slot;

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
  VkQueryPool query_pool;
  uint32_t timestamp_supported;
  float timestamp_period_ns;
  uint32_t reduction_test_flags;
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

int prom_m40b_calculate_padding_plan(uint32_t m, uint32_t n, uint32_t k,
                                    prom_m40b_padding_plan* out_plan) {
  uint64_t a_elements;
  uint64_t b_elements;
  uint64_t c_elements;
  uint64_t logical_elements;
  uint64_t hash = 1469598103934665603ull;
  if (out_plan == NULL) return PROM_ERROR;
  memset(out_plan, 0, sizeof(*out_plan));
  if (m == 0u || n == 0u || k == 0u || m > UINT32_MAX - 15u ||
      n > UINT32_MAX - 15u || k > UINT32_MAX - 15u) return PROM_ERROR;
  out_plan->logical_m = m;
  out_plan->logical_n = n;
  out_plan->logical_k = k;
  out_plan->padded_m = (m + 15u) & ~15u;
  out_plan->padded_n = (n + 15u) & ~15u;
  out_plan->padded_k = (k + 15u) & ~15u;
  if (!prom_m40b_checked_product_u64(out_plan->padded_m, out_plan->padded_k, &a_elements) ||
      !prom_m40b_checked_product_u64(out_plan->padded_k, out_plan->padded_n, &b_elements) ||
      !prom_m40b_checked_product_u64(out_plan->padded_m, out_plan->padded_n, &c_elements) ||
      !prom_m40b_checked_product_u64(m, n, &logical_elements) ||
      a_elements > (UINT64_MAX - 1u) / 2u || b_elements > (UINT64_MAX - 1u) / 2u ||
      c_elements > UINT64_MAX / sizeof(float) || logical_elements > UINT64_MAX / sizeof(float)) {
    memset(out_plan, 0, sizeof(*out_plan));
    return PROM_ERROR;
  }
  out_plan->packed_a_bytes = ((a_elements + 1u) / 2u) * sizeof(uint32_t);
  out_plan->packed_b_bytes = ((b_elements + 1u) / 2u) * sizeof(uint32_t);
  out_plan->intermediate_c_bytes = c_elements * sizeof(float);
  out_plan->logical_output_bytes = logical_elements * sizeof(float);
  hash = prom_reduction_hash_u32(hash, m);
  hash = prom_reduction_hash_u32(hash, n);
  hash = prom_reduction_hash_u32(hash, k);
  hash = prom_reduction_hash_u32(hash, out_plan->padded_m);
  hash = prom_reduction_hash_u32(hash, out_plan->padded_n);
  hash = prom_reduction_hash_u32(hash, out_plan->padded_k);
  hash = prom_m40b_hash_u64(hash, out_plan->packed_a_bytes);
  hash = prom_m40b_hash_u64(hash, out_plan->packed_b_bytes);
  hash = prom_m40b_hash_u64(hash, out_plan->intermediate_c_bytes);
  out_plan->replay_id = hash;
  return PROM_OK;
}

int prom_m40b_validate_device_buffer_view(const prom_device_buffer_view* view,
                                          VkDevice expected_device,
                                          uint32_t expected_element_type,
                                          uint32_t expected_rows,
                                          uint32_t expected_columns,
                                          uint32_t expected_consumer_access,
                                          int32_t* out_detail) {
  uint64_t elements;
  uint64_t minimum_bytes;
  if (out_detail != NULL) *out_detail = PROM_M40B_DETAIL_INVALID_VIEW;
  if (view == NULL || view->buffer == VK_NULL_HANDLE || view->byte_length == 0u ||
      view->owning_device == VK_NULL_HANDLE || view->owning_lifetime_id == 0u ||
      view->owning_slot_generation == 0u || view->layout != PROM_DEVICE_LAYOUT_ROW_MAJOR ||
      view->logical_rows == 0u || view->logical_columns == 0u ||
      view->row_stride_elements < view->logical_columns ||
      view->element_type != expected_element_type || view->logical_rows != expected_rows ||
      view->logical_columns != expected_columns ||
      view->required_consumer_access != expected_consumer_access ||
      view->producer_access != PROM_DEVICE_ACCESS_COMPUTE_WRITE ||
      view->offset > UINT64_MAX - view->byte_length) return PROM_ERROR;
  if (expected_device == VK_NULL_HANDLE || view->owning_device != expected_device) {
    if (out_detail != NULL) *out_detail = PROM_M40B_DETAIL_CROSS_DEVICE;
    return PROM_ERROR;
  }
  if (!prom_m40b_checked_product_u64(view->logical_rows, view->row_stride_elements, &elements)) return PROM_ERROR;
  if (view->element_type == PROM_DEVICE_ELEMENT_F32) {
    if (elements > UINT64_MAX / sizeof(float)) return PROM_ERROR;
    minimum_bytes = elements * sizeof(float);
  } else if (view->element_type == PROM_DEVICE_ELEMENT_F16_PACKED_X2) {
    if (elements > UINT64_MAX - 1u) return PROM_ERROR;
    minimum_bytes = ((elements + 1u) / 2u) * sizeof(uint32_t);
  } else {
    return PROM_ERROR;
  }
  if (view->byte_length < minimum_bytes) return PROM_ERROR;
  if (out_detail != NULL) *out_detail = 0;
  return PROM_OK;
}

static void prom_m40b_trace_add(prom_m40b_command_trace* trace,
                                uint32_t operation,
                                uint32_t submit_index,
                                uint32_t reduction_stage_index,
                                uint32_t source_stage,
                                uint32_t destination_stage,
                                uint32_t source_access,
                                uint32_t destination_access) {
  prom_m40b_command_trace_entry* entry;
  if (trace == NULL || trace->entry_count >= PROM_M40B_MAX_COMMAND_TRACE_ENTRIES) return;
  entry = &trace->entries[trace->entry_count++];
  memset(entry, 0, sizeof(*entry));
  entry->operation = operation;
  entry->submit_index = submit_index;
  entry->reduction_stage_index = reduction_stage_index;
  entry->source_stage_mask = source_stage;
  entry->destination_stage_mask = destination_stage;
  entry->source_access_mask = source_access;
  entry->destination_access_mask = destination_access;
  entry->source_queue_family = VK_QUEUE_FAMILY_IGNORED;
  entry->destination_queue_family = VK_QUEUE_FAMILY_IGNORED;
}

void prom_m40b_plan_command_trace(uint32_t input_mode,
                                  uint32_t submit_plan,
                                  uint32_t reduction_stage_count,
                                  prom_m40b_command_trace* out_trace) {
  uint32_t stage_index;
  uint32_t consumer_submit = submit_plan == PROM_M40B_SUBMIT_TWO_BOUNDED ? 1u : 0u;
  uint64_t hash = 1469598103934665603ull;
  if (out_trace == NULL) return;
  memset(out_trace, 0, sizeof(*out_trace));
  out_trace->submit_count = submit_plan == PROM_M40B_SUBMIT_TWO_BOUNDED ? 2u : 1u;
  out_trace->intermediate_buffer_count = 1u;
  out_trace->final_readback_copy_count = 1u;
  if (input_mode == PROM_M40B_INPUT_HOST_A_PERSISTENT_B) {
    prom_m40b_trace_add(out_trace, PROM_M40B_TRACE_UPLOAD_A, 0u, UINT32_MAX,
                        VK_PIPELINE_STAGE_HOST_BIT | VK_PIPELINE_STAGE_TRANSFER_BIT,
                        VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                        VK_ACCESS_HOST_WRITE_BIT | VK_ACCESS_TRANSFER_WRITE_BIT,
                        VK_ACCESS_SHADER_READ_BIT);
  }
  prom_m40b_trace_add(out_trace, PROM_M40B_TRACE_BIND_SGEMM_PIPELINE, 0u, UINT32_MAX, 0u, 0u, 0u, 0u);
  prom_m40b_trace_add(out_trace, PROM_M40B_TRACE_BIND_SGEMM_DESCRIPTORS, 0u, UINT32_MAX, 0u, 0u, 0u, 0u);
  prom_m40b_trace_add(out_trace, PROM_M40B_TRACE_PUSH_SGEMM_CONSTANTS, 0u, UINT32_MAX, 0u, 0u, 0u, 0u);
  prom_m40b_trace_add(out_trace, PROM_M40B_TRACE_TIMESTAMP_SGEMM_BEGIN, 0u, UINT32_MAX, 0u, 0u, 0u, 0u);
  prom_m40b_trace_add(out_trace, PROM_M40B_TRACE_DISPATCH_SGEMM, 0u, UINT32_MAX, 0u, 0u, 0u, 0u);
  prom_m40b_trace_add(out_trace, PROM_M40B_TRACE_TIMESTAMP_SGEMM_END, 0u, UINT32_MAX, 0u, 0u, 0u, 0u);
  prom_m40b_trace_add(out_trace, PROM_M40B_TRACE_EXPOSE_DEVICE_C, 0u, UINT32_MAX, 0u, 0u, 0u, 0u);
  if (consumer_submit != 0u) {
    prom_m40b_trace_add(out_trace, PROM_M40B_TRACE_SUBMIT_DEPENDENCY, 1u, UINT32_MAX,
                        VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                        VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT);
  }
  prom_m40b_trace_add(out_trace, PROM_M40B_TRACE_COMPUTE_WRITE_TO_READ_BARRIER, consumer_submit, UINT32_MAX,
                      VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                      VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT);
  prom_m40b_trace_add(out_trace, PROM_M40B_TRACE_TIMESTAMP_SOFTMAX_BEGIN, consumer_submit, UINT32_MAX, 0u, 0u, 0u, 0u);
  for (stage_index = 0u; stage_index < reduction_stage_count; ++stage_index) {
    prom_m40b_trace_add(out_trace, PROM_M40B_TRACE_BIND_SOFTMAX_PIPELINE, consumer_submit, stage_index, 0u, 0u, 0u, 0u);
    prom_m40b_trace_add(out_trace, PROM_M40B_TRACE_BIND_SOFTMAX_DESCRIPTORS, consumer_submit, stage_index, 0u, 0u, 0u, 0u);
    prom_m40b_trace_add(out_trace, PROM_M40B_TRACE_PUSH_SOFTMAX_CONSTANTS, consumer_submit, stage_index, 0u, 0u, 0u, 0u);
    prom_m40b_trace_add(out_trace, PROM_M40B_TRACE_DISPATCH_SOFTMAX, consumer_submit, stage_index, 0u, 0u, 0u, 0u);
    if (stage_index + 1u < reduction_stage_count) {
      prom_m40b_trace_add(out_trace, PROM_M40B_TRACE_SOFTMAX_STAGE_BARRIER, consumer_submit, stage_index,
                          VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                          VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT);
    }
  }
  prom_m40b_trace_add(out_trace, PROM_M40B_TRACE_TIMESTAMP_SOFTMAX_END, consumer_submit, UINT32_MAX, 0u, 0u, 0u, 0u);
  prom_m40b_trace_add(out_trace, PROM_M40B_TRACE_COMPUTE_WRITE_TO_TRANSFER_READ_BARRIER, consumer_submit, UINT32_MAX,
                      VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT,
                      VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT);
  prom_m40b_trace_add(out_trace, PROM_M40B_TRACE_COPY_FINAL_READBACK, consumer_submit, UINT32_MAX, 0u, 0u, 0u, 0u);
  prom_m40b_trace_add(out_trace, PROM_M40B_TRACE_TIMESTAMP_READBACK_END, consumer_submit, UINT32_MAX, 0u, 0u, 0u, 0u);
  hash = prom_reduction_hash_u32(hash, input_mode);
  hash = prom_reduction_hash_u32(hash, submit_plan);
  hash = prom_reduction_hash_u32(hash, reduction_stage_count);
  for (stage_index = 0u; stage_index < out_trace->entry_count; ++stage_index) {
    const prom_m40b_command_trace_entry* entry = &out_trace->entries[stage_index];
    hash = prom_reduction_hash_u32(hash, entry->operation);
    hash = prom_reduction_hash_u32(hash, entry->submit_index);
    hash = prom_reduction_hash_u32(hash, entry->reduction_stage_index);
    hash = prom_reduction_hash_u32(hash, entry->source_stage_mask);
    hash = prom_reduction_hash_u32(hash, entry->destination_stage_mask);
    hash = prom_reduction_hash_u32(hash, entry->source_access_mask);
    hash = prom_reduction_hash_u32(hash, entry->destination_access_mask);
  }
  out_trace->replay_id = hash;
}

void prom_m40b_selector_evaluate(const prom_m40b_selector_facts* facts,
                                 prom_m40b_selector_decision* out_decision) {
  uint64_t hash = 1469598103934665603ull;
  uint32_t values[15];
  uint32_t index;
  if (out_decision == NULL) return;
  memset(out_decision, 0, sizeof(*out_decision));
  if (facts == NULL) { out_decision->reason = PROM_M40B_SELECTOR_DISABLED; return; }
  values[0] = facts->experimental_enabled;
  values[1] = facts->capability_state;
  values[2] = facts->tuple_m;
  values[3] = facts->tuple_n;
  values[4] = facts->tuple_k;
  values[5] = facts->shader_float16;
  values[6] = facts->vulkan_memory_model;
  values[7] = facts->precision_allows_f16_rounded;
  values[8] = facts->m;
  values[9] = facts->n;
  values[10] = facts->k;
  values[11] = facts->padding_supported;
  values[12] = facts->persistent_b_available;
  values[13] = facts->device_resident_composition;
  values[14] = facts->rollback_active;
  for (index = 0u; index < 15u; ++index) hash = prom_reduction_hash_u32(hash, values[index]);
  out_decision->replay_id = hash;
  if (facts->experimental_enabled == 0u) out_decision->reason = PROM_M40B_SELECTOR_DISABLED;
  else if (facts->capability_state != PROM_VK_COOPERATIVE_MATRIX_EXECUTABLE ||
           facts->shader_float16 == 0u || facts->vulkan_memory_model == 0u) out_decision->reason = PROM_M40B_SELECTOR_CAPABILITY;
  else if (facts->tuple_m != 16u || facts->tuple_n != 16u || facts->tuple_k != 16u) out_decision->reason = PROM_M40B_SELECTOR_TUPLE;
  else if (facts->precision_allows_f16_rounded == 0u) out_decision->reason = PROM_M40B_SELECTOR_PRECISION;
  else if (facts->m == 0u || facts->n == 0u || facts->k == 0u ||
           facts->m > PROM_REDUCTION_MAX_ROWS || facts->n > PROM_REDUCTION_MAX_ELEMENTS_PER_ROW) out_decision->reason = PROM_M40B_SELECTOR_SHAPE;
  else if (facts->padding_supported == 0u) out_decision->reason = PROM_M40B_SELECTOR_PADDING;
  else if (facts->persistent_b_available == 0u) out_decision->reason = PROM_M40B_SELECTOR_PERSISTENT_B;
  else if (facts->device_resident_composition == 0u) out_decision->reason = PROM_M40B_SELECTOR_RESIDENCY;
  else if (facts->rollback_active != 0u) out_decision->reason = PROM_M40B_SELECTOR_ROLLBACK;
  else {
    out_decision->eligible = 1u;
    out_decision->reason = PROM_M40B_SELECTOR_ELIGIBLE;
  }
  /* The experiment remains disabled as a production selection authority. */
  out_decision->selected = 0u;
}

static int prom_m42_round_up_16(uint32_t value, uint32_t* out) {
  if (out == NULL || value == 0u || value > UINT32_MAX - 15u) return 0;
  *out = (value + 15u) & ~15u;
  return 1;
}

static int prom_m42_valid_shape(uint32_t tokens,
                                uint32_t model_width,
                                uint32_t head_dim,
                                uint32_t value_dim,
                                uint32_t* padded_tokens,
                                uint32_t* padded_model_width,
                                uint32_t* padded_head_dim) {
  uint64_t elements;
  if (tokens == 0u || model_width == 0u || head_dim == 0u || value_dim != head_dim ||
      tokens > PROM_M42_MAX_TOKENS || model_width > PROM_M42_MAX_MODEL_WIDTH ||
      head_dim > PROM_M42_MAX_HEAD_DIM ||
      !prom_m42_round_up_16(tokens, padded_tokens) ||
      !prom_m42_round_up_16(model_width, padded_model_width) ||
      !prom_m42_round_up_16(head_dim, padded_head_dim)) return 0;
  if (!prom_m40b_checked_product_u64(tokens, model_width, &elements) ||
      elements > PROM_M42_MAX_MATRIX_ELEMENTS ||
      !prom_m40b_checked_product_u64(model_width, head_dim, &elements) ||
      elements > PROM_M42_MAX_MATRIX_ELEMENTS ||
      !prom_m40b_checked_product_u64(tokens, tokens, &elements) ||
      elements > PROM_M42_MAX_MATRIX_ELEMENTS ||
      !prom_m40b_checked_product_u64(tokens, head_dim, &elements) ||
      elements > PROM_M42_MAX_MATRIX_ELEMENTS) return 0;
  return 1;
}

static float prom_m42_resolve_scale(uint32_t head_dim, float scale, uint32_t scale_explicit) {
  return scale_explicit != 0u ? scale : 1.0f / sqrtf((float)head_dim);
}

static void prom_m42_add_stage(prom_m42_attention_plan* plan,
                               uint32_t operation,
                               uint32_t path,
                               uint32_t input,
                               uint32_t auxiliary,
                               uint32_t output,
                               uint32_t dispatch_count,
                               uint32_t barrier_before,
                               uint32_t barrier_after) {
  prom_m42_stage_plan* stage;
  if (plan == NULL || plan->stage_count >= PROM_M42_MAX_STAGES) return;
  stage = &plan->stages[plan->stage_count++];
  memset(stage, 0, sizeof(*stage));
  stage->operation = operation;
  stage->path = path;
  stage->input_buffer = input;
  stage->auxiliary_buffer = auxiliary;
  stage->output_buffer = output;
  stage->dispatch_count = dispatch_count;
  stage->barrier_before = barrier_before;
  stage->barrier_after = barrier_after;
  stage->source_queue_family = VK_QUEUE_FAMILY_IGNORED;
  stage->destination_queue_family = VK_QUEUE_FAMILY_IGNORED;
  if (operation == PROM_M42_STAGE_UPLOAD_X) {
    stage->source_stage_mask = VK_PIPELINE_STAGE_HOST_BIT | VK_PIPELINE_STAGE_TRANSFER_BIT;
    stage->destination_stage_mask = VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT;
    stage->source_access_mask = VK_ACCESS_HOST_WRITE_BIT | VK_ACCESS_TRANSFER_WRITE_BIT;
    stage->destination_access_mask = VK_ACCESS_SHADER_READ_BIT;
  } else if (operation == PROM_M42_STAGE_FINAL_READBACK) {
    stage->source_stage_mask = VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT;
    stage->destination_stage_mask = VK_PIPELINE_STAGE_TRANSFER_BIT;
    stage->source_access_mask = VK_ACCESS_SHADER_WRITE_BIT;
    stage->destination_access_mask = VK_ACCESS_TRANSFER_READ_BIT;
  } else if (barrier_before != 0u || barrier_after != 0u) {
    stage->source_stage_mask = VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT;
    stage->destination_stage_mask = VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT;
    stage->source_access_mask = VK_ACCESS_SHADER_WRITE_BIT;
    stage->destination_access_mask = VK_ACCESS_SHADER_READ_BIT;
    if (operation == PROM_M42_STAGE_SCALE) {
      stage->destination_access_mask |= VK_ACCESS_SHADER_WRITE_BIT;
    }
  }
}

static void prom_m42_add_buffer(prom_m42_attention_plan* plan,
                                uint32_t identity,
                                uint32_t element_type,
                                uint32_t rows,
                                uint32_t columns,
                                uint32_t stride,
                                uint32_t producer,
                                uint32_t consumer,
                                uint64_t bytes) {
  prom_m42_buffer_plan* buffer;
  if (plan == NULL || plan->buffer_count >= PROM_M42_MAX_BUFFERS) return;
  buffer = &plan->buffers[plan->buffer_count++];
  memset(buffer, 0, sizeof(*buffer));
  buffer->identity = identity;
  buffer->element_type = element_type;
  buffer->logical_rows = rows;
  buffer->logical_columns = columns;
  buffer->row_stride_elements = stride;
  buffer->first_producer_stage = producer;
  buffer->last_consumer_stage = consumer;
  buffer->retained_bytes = bytes;
}

int prom_m42_attention_plan_build(const prom_m42_plan_request* request,
                                  prom_m42_attention_plan* out_plan) {
  PrometheusReductionRequest reduction_request;
  PrometheusReductionPlan reduction_plan;
  uint32_t padded_tokens;
  uint32_t padded_model;
  uint32_t padded_head;
  uint32_t selected;
  uint32_t reduced;
  uint32_t path_stride_tokens;
  uint32_t path_stride_head;
  uint64_t elements;
  uint64_t command_hash = 1469598103934665603ull;
  uint64_t replay_hash = 1469598103934665603ull;
  uint32_t index;
  if (out_plan == NULL) return PROM_ERROR;
  memset(out_plan, 0, sizeof(*out_plan));
  if (request == NULL ||
      !prom_m42_valid_shape(request->tokens, request->model_width, request->head_dim,
                            request->value_dim, &padded_tokens, &padded_model, &padded_head) ||
      request->precision_policy < PROM_M42_PRECISION_F16_ROUNDED ||
      request->precision_policy > PROM_M42_PRECISION_FP32 ||
      request->preferred_path < PROM_M42_PATH_COOPERATIVE ||
      request->preferred_path > PROM_M42_PATH_CONVENTIONAL_FP16 ||
      request->input_mode < PROM_M42_INPUT_HOST_X || request->input_mode > PROM_M42_INPUT_RESIDENT_X) {
    return PROM_ERROR;
  }
  out_plan->tokens = request->tokens;
  out_plan->model_width = request->model_width;
  out_plan->head_dim = request->head_dim;
  out_plan->value_dim = request->value_dim;
  out_plan->padded_tokens = padded_tokens;
  out_plan->padded_model_width = padded_model;
  out_plan->padded_head_dim = padded_head;
  out_plan->scale = prom_m42_resolve_scale(request->head_dim, request->scale, request->scale_explicit);
  if (!isfinite(out_plan->scale)) return PROM_ERROR;
  out_plan->precision_policy = request->precision_policy;
  out_plan->preferred_path = request->preferred_path;
  selected = request->preferred_path;
  out_plan->selector_reason = request->preferred_path == PROM_M42_PATH_COOPERATIVE
                                  ? PROM_M42_SELECTOR_REQUESTED
                                  : PROM_M42_SELECTOR_EXPLICIT_CONVENTIONAL;
  if (request->preferred_path == PROM_M42_PATH_COOPERATIVE &&
      request->precision_policy != PROM_M42_PRECISION_F16_ROUNDED) {
    if (request->allow_fallback == 0u) return PROM_ERROR;
    selected = PROM_M42_PATH_A2X4;
    out_plan->fallback_used = 1u;
    out_plan->selector_reason = PROM_M42_SELECTOR_PRECISION_FALLBACK;
  } else if (request->preferred_path == PROM_M42_PATH_COOPERATIVE &&
             request->cooperative_capability_state < PROM_VK_COOPERATIVE_MATRIX_DEVICE_FEATURE_ENABLED) {
    if (request->allow_fallback == 0u) return PROM_ERROR;
    selected = PROM_M42_PATH_CONVENTIONAL_FP16;
    out_plan->fallback_used = 1u;
    out_plan->selector_reason = PROM_M42_SELECTOR_CAPABILITY_FALLBACK;
  } else if (request->preferred_path == PROM_M42_PATH_COOPERATIVE && request->rollback_active != 0u) {
    if (request->allow_fallback == 0u) return PROM_ERROR;
    selected = PROM_M42_PATH_A2X4;
    out_plan->fallback_used = 1u;
    out_plan->selector_reason = PROM_M42_SELECTOR_ROLLBACK_FALLBACK;
  } else if (request->preferred_path == PROM_M42_PATH_CONVENTIONAL_FP16 &&
             request->precision_policy != PROM_M42_PRECISION_F16_ROUNDED) {
    if (request->allow_fallback == 0u) return PROM_ERROR;
    selected = PROM_M42_PATH_A2X4;
    out_plan->fallback_used = 1u;
    out_plan->selector_reason = PROM_M42_SELECTOR_PRECISION_FALLBACK;
  } else if (request->preferred_path == PROM_M42_PATH_A2X4 &&
             request->precision_policy != PROM_M42_PRECISION_FP32) {
    if (request->allow_fallback == 0u) return PROM_ERROR;
    selected = PROM_M42_PATH_CONVENTIONAL_FP16;
    out_plan->fallback_used = 1u;
    out_plan->selector_reason = PROM_M42_SELECTOR_PRECISION_FALLBACK;
  }
  out_plan->selected_path = selected;
  reduced = selected != PROM_M42_PATH_A2X4;
  out_plan->k_layout_strategy = reduced != 0u ? PROM_M42_K_LAYOUT_PACK_TRANSPOSE_F16
                                              : PROM_M42_K_LAYOUT_TRANSPOSE_F32;
  out_plan->probability_strategy = reduced != 0u ? PROM_M42_PROBABILITY_PACK_F16
                                                  : PROM_M42_PROBABILITY_F32_DIRECT;
  out_plan->submit_count = 1u;
  out_plan->final_readback_copy_count = 1u;

  memset(&reduction_request, 0, sizeof(reduction_request));
  reduction_request.struct_size = sizeof(reduction_request);
  reduction_request.row_count = request->tokens;
  reduction_request.elements_per_row = request->tokens;
  reduction_request.input_element_count = (uint64_t)request->tokens * request->tokens;
  reduction_request.output_element_count = reduction_request.input_element_count;
  reduction_request.operation = PROM_REDUCTION_OPERATION_SOFTMAX;
  reduction_request.finalization = PROM_REDUCTION_FINALIZATION_STABLE_SOFTMAX;
  if (prom_reactor_reduction_plan_impl(&reduction_request, &reduction_plan) != PROM_OK) return PROM_ERROR;
  out_plan->reduction_stage_count = reduction_plan.stage_count;
  out_plan->reduction_replay_id = reduction_plan.replay_id;

  if (request->input_mode == PROM_M42_INPUT_HOST_X) {
    prom_m42_add_stage(out_plan, PROM_M42_STAGE_UPLOAD_X, selected, PROM_M42_BUFFER_X, 0u,
                       PROM_M42_BUFFER_X, 0u, 0u, 1u);
  }
  prom_m42_add_stage(out_plan, PROM_M42_STAGE_PROJECT_Q, selected, PROM_M42_BUFFER_X, 0u,
                     PROM_M42_BUFFER_Q, 1u, 0u, 1u);
  prom_m42_add_stage(out_plan, PROM_M42_STAGE_PROJECT_K, selected, PROM_M42_BUFFER_X, 0u,
                     PROM_M42_BUFFER_K, 1u, 0u, 1u);
  prom_m42_add_stage(out_plan, PROM_M42_STAGE_PROJECT_V, selected, PROM_M42_BUFFER_X, 0u,
                     PROM_M42_BUFFER_V, 1u, 0u, 1u);
  if (reduced != 0u) {
    prom_m42_add_stage(out_plan, PROM_M42_STAGE_PACK_Q, selected, PROM_M42_BUFFER_Q, 0u,
                       PROM_M42_BUFFER_Q_PACKED, 1u, 1u, 1u);
  }
  prom_m42_add_stage(out_plan, PROM_M42_STAGE_LAYOUT_K, selected, PROM_M42_BUFFER_K, 0u,
                     PROM_M42_BUFFER_K_TRANSPOSED, 1u, 1u, 1u);
  if (reduced != 0u) {
    prom_m42_add_stage(out_plan, PROM_M42_STAGE_PACK_V, selected, PROM_M42_BUFFER_V, 0u,
                       PROM_M42_BUFFER_V_PACKED, 1u, 1u, 1u);
  }
  prom_m42_add_stage(out_plan, PROM_M42_STAGE_QK_TRANSPOSE, selected,
                     reduced != 0u ? PROM_M42_BUFFER_Q_PACKED : PROM_M42_BUFFER_Q,
                     PROM_M42_BUFFER_K_TRANSPOSED, PROM_M42_BUFFER_SCORES, 1u, 1u, 1u);
  prom_m42_add_stage(out_plan, PROM_M42_STAGE_SCALE, selected, PROM_M42_BUFFER_SCORES, 0u,
                     PROM_M42_BUFFER_SCORES, 1u, 1u, 1u);
  prom_m42_add_stage(out_plan, PROM_M42_STAGE_SOFTMAX, selected, PROM_M42_BUFFER_SCORES, 0u,
                     PROM_M42_BUFFER_PROBABILITIES, reduction_plan.stage_count, 1u, 1u);
  if (reduced != 0u) {
    prom_m42_add_stage(out_plan, PROM_M42_STAGE_PACK_P, selected, PROM_M42_BUFFER_PROBABILITIES, 0u,
                       PROM_M42_BUFFER_P_PACKED, 1u, 1u, 1u);
  }
  prom_m42_add_stage(out_plan, PROM_M42_STAGE_PV, selected,
                     reduced != 0u ? PROM_M42_BUFFER_P_PACKED : PROM_M42_BUFFER_PROBABILITIES,
                     reduced != 0u ? PROM_M42_BUFFER_V_PACKED : PROM_M42_BUFFER_V,
                     PROM_M42_BUFFER_OUTPUT, 1u, 1u, 1u);
  prom_m42_add_stage(out_plan, PROM_M42_STAGE_FINAL_READBACK, selected, PROM_M42_BUFFER_OUTPUT, 0u,
                     PROM_M42_BUFFER_OUTPUT, 0u, 1u, 0u);

  path_stride_tokens = reduced != 0u ? padded_tokens : request->tokens;
  path_stride_head = reduced != 0u ? padded_head : request->head_dim;
  prom_m40b_checked_product_u64(reduced != 0u ? padded_tokens : request->tokens,
                                reduced != 0u ? padded_model : request->model_width, &elements);
  prom_m42_add_buffer(out_plan, PROM_M42_BUFFER_X,
                      reduced != 0u ? PROM_DEVICE_ELEMENT_F16_PACKED_X2 : PROM_DEVICE_ELEMENT_F32,
                      request->tokens, request->model_width,
                      reduced != 0u ? padded_model : request->model_width,
                      PROM_M42_STAGE_UPLOAD_X, PROM_M42_STAGE_PROJECT_V,
                      reduced != 0u ? ((elements + 1u) / 2u) * sizeof(uint32_t) : elements * sizeof(float));
  prom_m40b_checked_product_u64(reduced != 0u ? padded_tokens : request->tokens,
                                reduced != 0u ? padded_head : request->head_dim, &elements);
  prom_m42_add_buffer(out_plan, PROM_M42_BUFFER_Q, PROM_DEVICE_ELEMENT_F32, request->tokens,
                      request->head_dim, path_stride_head, PROM_M42_STAGE_PROJECT_Q,
                      reduced != 0u ? PROM_M42_STAGE_PACK_Q : PROM_M42_STAGE_QK_TRANSPOSE,
                      elements * sizeof(float));
  prom_m42_add_buffer(out_plan, PROM_M42_BUFFER_K, PROM_DEVICE_ELEMENT_F32, request->tokens,
                      request->head_dim, path_stride_head, PROM_M42_STAGE_PROJECT_K,
                      PROM_M42_STAGE_LAYOUT_K, elements * sizeof(float));
  prom_m42_add_buffer(out_plan, PROM_M42_BUFFER_V, PROM_DEVICE_ELEMENT_F32, request->tokens,
                      request->head_dim, path_stride_head, PROM_M42_STAGE_PROJECT_V,
                      reduced != 0u ? PROM_M42_STAGE_PACK_V : PROM_M42_STAGE_PV,
                      elements * sizeof(float));
  if (reduced != 0u) {
    prom_m42_add_buffer(out_plan, PROM_M42_BUFFER_Q_PACKED, PROM_DEVICE_ELEMENT_F16_PACKED_X2,
                        request->tokens, request->head_dim, padded_head, PROM_M42_STAGE_PACK_Q,
                        PROM_M42_STAGE_QK_TRANSPOSE, ((elements + 1u) / 2u) * sizeof(uint32_t));
    prom_m42_add_buffer(out_plan, PROM_M42_BUFFER_V_PACKED, PROM_DEVICE_ELEMENT_F16_PACKED_X2,
                        request->tokens, request->head_dim, padded_head, PROM_M42_STAGE_PACK_V,
                        PROM_M42_STAGE_PV, ((elements + 1u) / 2u) * sizeof(uint32_t));
  }
  prom_m40b_checked_product_u64(reduced != 0u ? padded_head : request->head_dim,
                                reduced != 0u ? padded_tokens : request->tokens, &elements);
  prom_m42_add_buffer(out_plan, PROM_M42_BUFFER_K_TRANSPOSED,
                      reduced != 0u ? PROM_DEVICE_ELEMENT_F16_PACKED_X2 : PROM_DEVICE_ELEMENT_F32,
                      request->head_dim, request->tokens, path_stride_tokens,
                      PROM_M42_STAGE_LAYOUT_K, PROM_M42_STAGE_QK_TRANSPOSE,
                      reduced != 0u ? ((elements + 1u) / 2u) * sizeof(uint32_t) : elements * sizeof(float));
  prom_m40b_checked_product_u64(reduced != 0u ? padded_tokens : request->tokens,
                                reduced != 0u ? padded_tokens : request->tokens, &elements);
  prom_m42_add_buffer(out_plan, PROM_M42_BUFFER_SCORES, PROM_DEVICE_ELEMENT_F32,
                      request->tokens, request->tokens, path_stride_tokens,
                      PROM_M42_STAGE_QK_TRANSPOSE, PROM_M42_STAGE_SOFTMAX, elements * sizeof(float));
  prom_m40b_checked_product_u64(request->tokens, request->tokens, &elements);
  prom_m42_add_buffer(out_plan, PROM_M42_BUFFER_PROBABILITIES, PROM_DEVICE_ELEMENT_F32,
                      request->tokens, request->tokens, request->tokens,
                      PROM_M42_STAGE_SOFTMAX,
                      reduced != 0u ? PROM_M42_STAGE_PACK_P : PROM_M42_STAGE_PV,
                      elements * sizeof(float));
  if (reduced != 0u) {
    prom_m40b_checked_product_u64(padded_tokens, padded_tokens, &elements);
    prom_m42_add_buffer(out_plan, PROM_M42_BUFFER_P_PACKED, PROM_DEVICE_ELEMENT_F16_PACKED_X2,
                        request->tokens, request->tokens, padded_tokens,
                        PROM_M42_STAGE_PACK_P, PROM_M42_STAGE_PV,
                        ((elements + 1u) / 2u) * sizeof(uint32_t));
  }
  prom_m40b_checked_product_u64(reduced != 0u ? padded_tokens : request->tokens,
                                reduced != 0u ? padded_head : request->head_dim, &elements);
  prom_m42_add_buffer(out_plan, PROM_M42_BUFFER_OUTPUT, PROM_DEVICE_ELEMENT_F32,
                      request->tokens, request->head_dim, path_stride_head,
                      PROM_M42_STAGE_PV, PROM_M42_STAGE_FINAL_READBACK, elements * sizeof(float));

  for (index = 0u; index < out_plan->stage_count; ++index) {
    const prom_m42_stage_plan* stage = &out_plan->stages[index];
    command_hash = prom_reduction_hash_u32(command_hash, stage->operation);
    command_hash = prom_reduction_hash_u32(command_hash, stage->path);
    command_hash = prom_reduction_hash_u32(command_hash, stage->input_buffer);
    command_hash = prom_reduction_hash_u32(command_hash, stage->auxiliary_buffer);
    command_hash = prom_reduction_hash_u32(command_hash, stage->output_buffer);
    command_hash = prom_reduction_hash_u32(command_hash, stage->dispatch_count);
    command_hash = prom_reduction_hash_u32(command_hash, stage->barrier_before);
    command_hash = prom_reduction_hash_u32(command_hash, stage->barrier_after);
    command_hash = prom_reduction_hash_u32(command_hash, stage->source_stage_mask);
    command_hash = prom_reduction_hash_u32(command_hash, stage->destination_stage_mask);
    command_hash = prom_reduction_hash_u32(command_hash, stage->source_access_mask);
    command_hash = prom_reduction_hash_u32(command_hash, stage->destination_access_mask);
    command_hash = prom_reduction_hash_u32(command_hash, stage->source_queue_family);
    command_hash = prom_reduction_hash_u32(command_hash, stage->destination_queue_family);
  }
  out_plan->command_plan_replay_id = command_hash;
  replay_hash = prom_reduction_hash_u32(replay_hash, request->tokens);
  replay_hash = prom_reduction_hash_u32(replay_hash, request->model_width);
  replay_hash = prom_reduction_hash_u32(replay_hash, request->head_dim);
  replay_hash = prom_reduction_hash_u32(replay_hash, request->value_dim);
  { uint32_t scale_bits = 0u; memcpy(&scale_bits, &out_plan->scale, sizeof(scale_bits)); replay_hash = prom_reduction_hash_u32(replay_hash, scale_bits); }
  replay_hash = prom_reduction_hash_u32(replay_hash, request->precision_policy);
  replay_hash = prom_reduction_hash_u32(replay_hash, selected);
  replay_hash = prom_reduction_hash_u32(replay_hash, out_plan->k_layout_strategy);
  replay_hash = prom_reduction_hash_u32(replay_hash, out_plan->probability_strategy);
  replay_hash = prom_m40b_hash_u64(replay_hash, request->wq_generation);
  replay_hash = prom_m40b_hash_u64(replay_hash, request->wk_generation);
  replay_hash = prom_m40b_hash_u64(replay_hash, request->wv_generation);
  replay_hash = prom_m40b_hash_u64(replay_hash, request->wq_hash);
  replay_hash = prom_m40b_hash_u64(replay_hash, request->wk_hash);
  replay_hash = prom_m40b_hash_u64(replay_hash, request->wv_hash);
  replay_hash = prom_m40b_hash_u64(replay_hash, PROM_M40B_COOPERATIVE_SHADER_HASH);
  replay_hash = prom_m40b_hash_u64(replay_hash, PROM_M42_PACK_SHADER_HASH);
  replay_hash = prom_m40b_hash_u64(replay_hash, PROM_M42_TRANSPOSE_SHADER_HASH);
  replay_hash = prom_m40b_hash_u64(replay_hash, PROM_M42_SCALE_SHADER_HASH);
  replay_hash = prom_m40b_hash_u64(replay_hash, reduction_plan.replay_id);
  replay_hash = prom_m40b_hash_u64(replay_hash, command_hash);
  out_plan->replay_id = replay_hash;
  return PROM_OK;
}

uint64_t prom_m42_k_transpose_index(uint32_t token,
                                    uint32_t head_column,
                                    uint32_t tokens,
                                    uint32_t padded_tokens) {
  if (tokens == 0u || padded_tokens < tokens || token >= tokens) return UINT64_MAX;
  return (uint64_t)head_column * padded_tokens + token;
}

static float prom_m42_round_f16(float value) {
  return prom_sgemm_fp16_bits_to_float32(prom_sgemm_float32_to_fp16_bits(value));
}

static int prom_m42_finite_matrix(const float* values, uint64_t count) {
  uint64_t index;
  if (values == NULL) return 0;
  for (index = 0u; index < count; ++index) if (!isfinite(values[index])) return 0;
  return 1;
}

int prom_m42_attention_cpu_reference(const prom_m42_reference_request* request,
                                     prom_m42_reference_result* out_result) {
  float* q = NULL;
  float* k = NULL;
  float* v = NULL;
  float* scores = NULL;
  float* probabilities = NULL;
  uint32_t padded_tokens;
  uint32_t padded_model;
  uint32_t padded_head;
  uint64_t x_count;
  uint64_t weight_count;
  uint64_t q_count;
  uint64_t score_count;
  uint32_t row;
  uint32_t reduced;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  out_result->detail_code = PROM_M42_DETAIL_INVALID_REQUEST;
  if (request == NULL || request->output == NULL ||
      !prom_m42_valid_shape(request->tokens, request->model_width, request->head_dim,
                            request->value_dim, &padded_tokens, &padded_model, &padded_head) ||
      request->precision_policy < PROM_M42_PRECISION_F16_ROUNDED ||
      request->precision_policy > PROM_M42_PRECISION_FP32) return PROM_ERROR;
  (void)padded_tokens; (void)padded_model; (void)padded_head;
  x_count = (uint64_t)request->tokens * request->model_width;
  weight_count = (uint64_t)request->model_width * request->head_dim;
  q_count = (uint64_t)request->tokens * request->head_dim;
  score_count = (uint64_t)request->tokens * request->tokens;
  if (!prom_m42_finite_matrix(request->x, x_count) ||
      !prom_m42_finite_matrix(request->wq, weight_count) ||
      !prom_m42_finite_matrix(request->wk, weight_count) ||
      !prom_m42_finite_matrix(request->wv, weight_count)) {
    out_result->detail_code = PROM_M42_DETAIL_NONFINITE_INPUT;
    return PROM_ERROR;
  }
  out_result->resolved_scale = prom_m42_resolve_scale(request->head_dim, request->scale, request->scale_explicit);
  if (!isfinite(out_result->resolved_scale) || q_count > SIZE_MAX / sizeof(float) ||
      score_count > SIZE_MAX / sizeof(float)) return PROM_ERROR;
  q = (float*)calloc((size_t)q_count, sizeof(float));
  k = (float*)calloc((size_t)q_count, sizeof(float));
  v = (float*)calloc((size_t)q_count, sizeof(float));
  scores = (float*)calloc((size_t)score_count, sizeof(float));
  probabilities = (float*)calloc((size_t)score_count, sizeof(float));
  if (q == NULL || k == NULL || v == NULL || scores == NULL || probabilities == NULL) {
    out_result->detail_code = PROM_M42_DETAIL_RESOURCE;
    goto fail;
  }
  reduced = request->precision_policy == PROM_M42_PRECISION_F16_ROUNDED;
  for (row = 0u; row < request->tokens; ++row) {
    uint32_t column;
    for (column = 0u; column < request->head_dim; ++column) {
      uint32_t inner;
      float q_value = 0.0f;
      float k_value = 0.0f;
      float v_value = 0.0f;
      for (inner = 0u; inner < request->model_width; ++inner) {
        float x_value = request->x[(uint64_t)row * request->model_width + inner];
        float wq_value = request->wq[(uint64_t)inner * request->head_dim + column];
        float wk_value = request->wk[(uint64_t)inner * request->head_dim + column];
        float wv_value = request->wv[(uint64_t)inner * request->head_dim + column];
        if (reduced != 0u) {
          x_value = prom_m42_round_f16(x_value);
          wq_value = prom_m42_round_f16(wq_value);
          wk_value = prom_m42_round_f16(wk_value);
          wv_value = prom_m42_round_f16(wv_value);
        }
        q_value += x_value * wq_value;
        k_value += x_value * wk_value;
        v_value += x_value * wv_value;
      }
      q[(uint64_t)row * request->head_dim + column] = q_value;
      k[(uint64_t)row * request->head_dim + column] = k_value;
      v[(uint64_t)row * request->head_dim + column] = v_value;
    }
  }
  for (row = 0u; row < request->tokens; ++row) {
    uint32_t token;
    float maximum = -INFINITY;
    float denominator = 0.0f;
    float row_sum = 0.0f;
    for (token = 0u; token < request->tokens; ++token) {
      uint32_t inner;
      float score = 0.0f;
      for (inner = 0u; inner < request->head_dim; ++inner) {
        float q_value = q[(uint64_t)row * request->head_dim + inner];
        float k_value = k[(uint64_t)token * request->head_dim + inner];
        if (reduced != 0u) { q_value = prom_m42_round_f16(q_value); k_value = prom_m42_round_f16(k_value); }
        score += q_value * k_value;
      }
      score *= out_result->resolved_scale;
      scores[(uint64_t)row * request->tokens + token] = score;
      if (score > maximum) maximum = score;
    }
    for (token = 0u; token < request->tokens; ++token) {
      denominator += expf(scores[(uint64_t)row * request->tokens + token] - maximum);
    }
    for (token = 0u; token < request->tokens; ++token) {
      float probability = expf(scores[(uint64_t)row * request->tokens + token] - maximum) / denominator;
      probabilities[(uint64_t)row * request->tokens + token] = probability;
      row_sum += probability;
    }
    if (row == 0u || row_sum < out_result->minimum_probability_row_sum) out_result->minimum_probability_row_sum = row_sum;
    if (row == 0u || row_sum > out_result->maximum_probability_row_sum) out_result->maximum_probability_row_sum = row_sum;
  }
  for (row = 0u; row < request->tokens; ++row) {
    uint32_t column;
    for (column = 0u; column < request->head_dim; ++column) {
      uint32_t token;
      float output = 0.0f;
      for (token = 0u; token < request->tokens; ++token) {
        float probability = probabilities[(uint64_t)row * request->tokens + token];
        float value = v[(uint64_t)token * request->head_dim + column];
        if (reduced != 0u) { probability = prom_m42_round_f16(probability); value = prom_m42_round_f16(value); }
        output += probability * value;
      }
      request->output[(uint64_t)row * request->head_dim + column] = output;
    }
  }
  if (request->q != NULL) memcpy(request->q, q, (size_t)(q_count * sizeof(float)));
  if (request->k != NULL) memcpy(request->k, k, (size_t)(q_count * sizeof(float)));
  if (request->v != NULL) memcpy(request->v, v, (size_t)(q_count * sizeof(float)));
  if (request->scores != NULL) memcpy(request->scores, scores, (size_t)(score_count * sizeof(float)));
  if (request->probabilities != NULL) memcpy(request->probabilities, probabilities, (size_t)(score_count * sizeof(float)));
  out_result->all_finite = prom_m42_finite_matrix(request->output, q_count) &&
                           prom_m42_finite_matrix(probabilities, score_count);
  if (out_result->all_finite == 0u) { out_result->detail_code = PROM_M42_DETAIL_MISMATCH; goto fail; }
  out_result->stage = 0u;
  out_result->detail_code = 0;
  free(probabilities); free(scores); free(v); free(k); free(q);
  return PROM_OK;
fail:
  free(probabilities); free(scores); free(v); free(k); free(q);
  return PROM_ERROR;
}

int prom_m42_attention_compare(uint32_t stage,
                               const float* expected,
                               const float* actual,
                               uint32_t rows,
                               uint32_t columns,
                               uint32_t padded_rows,
                               uint32_t padded_columns,
                               float absolute_tolerance,
                               float relative_tolerance,
                               uint64_t operator_replay_id,
                               uint64_t reduction_replay_id,
                               prom_m42_mismatch* out_mismatch) {
  uint32_t row;
  if (out_mismatch == NULL) return PROM_ERROR;
  memset(out_mismatch, 0, sizeof(*out_mismatch));
  out_mismatch->matched = 1u;
  out_mismatch->stage = stage;
  out_mismatch->logical_rows = rows;
  out_mismatch->logical_columns = columns;
  out_mismatch->padded_rows = padded_rows;
  out_mismatch->padded_columns = padded_columns;
  out_mismatch->operator_replay_id = operator_replay_id;
  out_mismatch->reduction_replay_id = reduction_replay_id;
  if (expected == NULL || actual == NULL || rows == 0u || columns == 0u ||
      padded_rows < rows || padded_columns < columns || absolute_tolerance < 0.0f ||
      relative_tolerance < 0.0f) return PROM_ERROR;
  for (row = 0u; row < rows; ++row) {
    uint32_t column;
    for (column = 0u; column < columns; ++column) {
      const uint64_t index = (uint64_t)row * columns + column;
      const float expected_value = expected[index];
      const float actual_value = actual[index];
      const float absolute_error = fabsf(expected_value - actual_value);
      const float relative_error = absolute_error / fmaxf(fabsf(expected_value), 1.0e-12f);
      if (!isfinite(expected_value) || !isfinite(actual_value) ||
          (absolute_error > absolute_tolerance && relative_error > relative_tolerance)) {
        out_mismatch->matched = 0u;
        out_mismatch->row = row;
        out_mismatch->column = column;
        out_mismatch->expected = expected_value;
        out_mismatch->actual = actual_value;
        out_mismatch->absolute_error = absolute_error;
        out_mismatch->relative_error = relative_error;
        return PROM_ERROR;
      }
    }
  }
  return PROM_OK;
}

static uint64_t prom_reduction_plan_replay_id(const PrometheusReductionPlan* plan) {
  uint64_t hash = 1469598103934665603ull;
  uint32_t stage_index;
  hash = prom_reduction_hash_u32(hash, plan->operation);
  hash = prom_reduction_hash_u32(hash, plan->row_count);
  hash = prom_reduction_hash_u32(hash, plan->elements_per_row);
  hash = prom_reduction_hash_u32(hash, plan->strategy);
  hash = prom_reduction_hash_u32(hash, plan->partial_count);
  hash = prom_reduction_hash_u32(hash, plan->stage_count);
  for (stage_index = 0u; stage_index < plan->stage_count; ++stage_index) {
    const PrometheusReductionStageDispatch* stage = &plan->stages[stage_index];
    hash = prom_reduction_hash_u32(hash, stage->stage_role);
    hash = prom_reduction_hash_u32(hash, stage->shader_id);
    hash = prom_reduction_hash_u32(hash, stage->implementation_id);
    hash = prom_reduction_hash_u32(hash, stage->groups_x);
    hash = prom_reduction_hash_u32(hash, stage->input_elements_per_row);
    hash = prom_reduction_hash_u32(hash, stage->output_partials_per_row);
    hash = prom_reduction_hash_u32(hash, stage->temporary_role);
    hash = prom_reduction_hash_u32(hash, (uint32_t)stage->temporary_bytes_written);
    hash = prom_reduction_hash_u32(hash, (uint32_t)(stage->temporary_bytes_written >> 32u));
  }
  return hash;
}

static int prom_reduction_validate_request(const PrometheusReductionRequest* request,
                                           uint32_t require_pointers,
                                           uint64_t* out_total_elements,
                                           uint64_t* out_output_elements,
                                           int32_t* out_detail) {
  uint64_t total;
  uint64_t output;
  if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_INVALID_REQUEST;
  if (request == NULL || request->struct_size < sizeof(PrometheusReductionRequest)) return 0;
  if (require_pointers != 0u && request->input == NULL) {
    if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_NULL_INPUT;
    return 0;
  }
  if (require_pointers != 0u && request->output == NULL) {
    if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_NULL_OUTPUT;
    return 0;
  }
  if (request->row_count == 0u) {
    if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_ZERO_ROW_COUNT;
    return 0;
  }
  if (request->elements_per_row == 0u) {
    if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_ZERO_ROW_WIDTH;
    return 0;
  }
  if (request->row_count > PROM_REDUCTION_MAX_ROWS) {
    if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_ROW_LIMIT;
    return 0;
  }
  if (request->elements_per_row > PROM_REDUCTION_MAX_ELEMENTS_PER_ROW) {
    if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_WIDTH_LIMIT;
    return 0;
  }
  total = (uint64_t)request->row_count * (uint64_t)request->elements_per_row;
  if (total == 0u || total > PROM_REDUCTION_MAX_TOTAL_ELEMENTS) {
    if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_ELEMENT_LIMIT;
    return 0;
  }
  if (request->operation != PROM_REDUCTION_OPERATION_SUM &&
      request->operation != PROM_REDUCTION_OPERATION_MAX &&
      request->operation != PROM_REDUCTION_OPERATION_SOFTMAX) {
    if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_UNSUPPORTED_OPERATION;
    return 0;
  }
  if (request->operation == PROM_REDUCTION_OPERATION_SOFTMAX) {
    if (request->finalization != PROM_REDUCTION_FINALIZATION_STABLE_SOFTMAX) {
      if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_INVALID_FINALIZATION;
      return 0;
    }
    output = total;
  } else {
    if (request->finalization != PROM_REDUCTION_FINALIZATION_NONE) {
      if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_INVALID_FINALIZATION;
      return 0;
    }
    output = request->row_count;
  }
  if ((request->flags & ~(PROM_REDUCTION_FLAG_FORCE_FUSED | PROM_REDUCTION_FLAG_FORCE_COMPOSED)) != 0u ||
      ((request->flags & PROM_REDUCTION_FLAG_FORCE_FUSED) != 0u &&
       (request->flags & PROM_REDUCTION_FLAG_FORCE_COMPOSED) != 0u)) {
    if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_UNSUPPORTED_STRATEGY;
    return 0;
  }
  if (request->operation != PROM_REDUCTION_OPERATION_SOFTMAX && request->flags != 0u) {
    if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_UNSUPPORTED_STRATEGY;
    return 0;
  }
  if ((request->flags & PROM_REDUCTION_FLAG_FORCE_FUSED) != 0u &&
      request->elements_per_row > PROM_REDUCTION_SINGLE_STAGE_THRESHOLD) {
    if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_UNSUPPORTED_STRATEGY;
    return 0;
  }
  if (request->input_element_count != total) {
    if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_INPUT_SIZE_MISMATCH;
    return 0;
  }
  if (request->output_element_count != output) {
    if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_OUTPUT_SIZE_MISMATCH;
    return 0;
  }
  if (out_total_elements != NULL) *out_total_elements = total;
  if (out_output_elements != NULL) *out_output_elements = output;
  if (out_detail != NULL) *out_detail = 0;
  return 1;
}

static void prom_reduction_add_stage(PrometheusReductionPlan* plan,
                                     uint32_t role,
                                     uint32_t shader_id,
                                     uint32_t implementation_id,
                                     uint32_t groups_x,
                                     uint32_t input_elements_per_row,
                                     uint32_t output_partials_per_row) {
  PrometheusReductionStageDispatch* stage = &plan->stages[plan->stage_count];
  memset(stage, 0, sizeof(*stage));
  stage->stage_role = role;
  stage->shader_id = shader_id;
  stage->implementation_id = implementation_id;
  stage->groups_x = groups_x;
  stage->groups_y = 1u;
  stage->groups_z = 1u;
  stage->input_elements_per_row = input_elements_per_row;
  stage->output_partials_per_row = output_partials_per_row;
  plan->stage_count += 1u;
}

static void prom_reduction_assign_temporary_metadata(PrometheusReductionPlan* plan) {
  uint64_t row_bytes = (uint64_t)plan->row_count * sizeof(float);
  uint64_t partial_bytes = plan->partial_count > 1u
                               ? (uint64_t)plan->row_count * plan->partial_count * sizeof(float)
                               : 0u;
  uint32_t index;
  plan->temporary_alignment_bytes = (uint32_t)sizeof(float);
  for (index = 0u; index < plan->stage_count; ++index) {
    PrometheusReductionStageDispatch* stage = &plan->stages[index];
    stage->temporary_role = PROM_REDUCTION_TEMPORARY_NONE;
    stage->temporary_bytes_written = 0u;
  }
  if (plan->operation == PROM_REDUCTION_OPERATION_SUM || plan->operation == PROM_REDUCTION_OPERATION_MAX) {
    if (plan->partial_count > 1u) {
      plan->stages[0].temporary_role = PROM_REDUCTION_TEMPORARY_PARTIALS;
      plan->stages[0].temporary_bytes_written = partial_bytes;
    }
    plan->temporary_bytes = partial_bytes;
    return;
  }
  if (plan->strategy == PROM_REDUCTION_STRATEGY_FUSED_SINGLE_WORKGROUP) {
    plan->temporary_bytes = 0u;
    return;
  }
  if (plan->partial_count > 1u) {
    plan->stages[0].temporary_role = PROM_REDUCTION_TEMPORARY_PARTIALS;
    plan->stages[0].temporary_bytes_written = partial_bytes;
    plan->stages[1].temporary_role = PROM_REDUCTION_TEMPORARY_ROW_MAX;
    plan->stages[1].temporary_bytes_written = row_bytes;
    plan->stages[2].temporary_role = PROM_REDUCTION_TEMPORARY_PARTIALS;
    plan->stages[2].temporary_bytes_written = partial_bytes;
    plan->stages[3].temporary_role = PROM_REDUCTION_TEMPORARY_ROW_SUM;
    plan->stages[3].temporary_bytes_written = row_bytes;
  } else {
    plan->stages[0].temporary_role = PROM_REDUCTION_TEMPORARY_ROW_MAX;
    plan->stages[0].temporary_bytes_written = row_bytes;
    plan->stages[1].temporary_role = PROM_REDUCTION_TEMPORARY_ROW_SUM;
    plan->stages[1].temporary_bytes_written = row_bytes;
  }
  plan->temporary_bytes = partial_bytes + 2u * row_bytes;
}

int prom_reactor_reduction_plan_impl(const PrometheusReductionRequest* request,
                                     PrometheusReductionPlan* out_plan) {
  uint64_t total_elements;
  uint64_t output_elements;
  uint32_t partial_count;
  uint32_t composed;
  int32_t detail;
  if (out_plan == NULL) return PROM_ERROR;
  memset(out_plan, 0, sizeof(*out_plan));
  out_plan->struct_size = sizeof(*out_plan);
  if (!prom_reduction_validate_request(request, 0u, &total_elements, &output_elements, &detail)) return PROM_ERROR;
  (void)output_elements;
  partial_count = prom_reduction_ceil_div_u32(request->elements_per_row, PROM_REDUCTION_ELEMENTS_PER_PARTIAL);
  out_plan->operation = request->operation;
  out_plan->row_count = request->row_count;
  out_plan->elements_per_row = request->elements_per_row;
  out_plan->local_size = PROM_REDUCTION_LOCAL_SIZE;
  out_plan->elements_per_partial = PROM_REDUCTION_ELEMENTS_PER_PARTIAL;
  out_plan->partial_count = partial_count;

  if (request->operation == PROM_REDUCTION_OPERATION_SUM || request->operation == PROM_REDUCTION_OPERATION_MAX) {
    uint32_t role = request->operation == PROM_REDUCTION_OPERATION_SUM ? PROM_REDUCTION_STAGE_ROW_SUM
                                                                       : PROM_REDUCTION_STAGE_ROW_MAX;
    uint32_t shader = request->operation == PROM_REDUCTION_OPERATION_SUM ? PROM_REDUCTION_SHADER_ROW_SUM
                                                                         : PROM_REDUCTION_SHADER_ROW_MAX;
    uint32_t implementation = request->operation == PROM_REDUCTION_OPERATION_SUM
                                  ? PROM_REDUCTION_IMPLEMENTATION_ROW_SUM
                                  : PROM_REDUCTION_IMPLEMENTATION_ROW_MAX;
    out_plan->strategy = PROM_REDUCTION_STRATEGY_COMPOSED;
    prom_reduction_add_stage(out_plan, role, shader, implementation, request->row_count * partial_count,
                             request->elements_per_row, partial_count);
    if (partial_count > 1u) {
      prom_reduction_add_stage(out_plan, role, shader, implementation, request->row_count, partial_count, 1u);
    }
  } else {
    composed = (request->flags & PROM_REDUCTION_FLAG_FORCE_COMPOSED) != 0u ||
               request->elements_per_row > PROM_REDUCTION_SINGLE_STAGE_THRESHOLD;
    if (!composed) {
      out_plan->strategy = PROM_REDUCTION_STRATEGY_FUSED_SINGLE_WORKGROUP;
      prom_reduction_add_stage(out_plan, PROM_REDUCTION_STAGE_SOFTMAX_FUSED,
                               PROM_REDUCTION_SHADER_SOFTMAX_FUSED,
                               PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_FUSED,
                               request->row_count, request->elements_per_row, 1u);
    } else {
      out_plan->strategy = PROM_REDUCTION_STRATEGY_COMPOSED;
      prom_reduction_add_stage(out_plan, PROM_REDUCTION_STAGE_ROW_MAX,
                               PROM_REDUCTION_SHADER_ROW_MAX,
                               PROM_REDUCTION_IMPLEMENTATION_ROW_MAX,
                               request->row_count * partial_count, request->elements_per_row, partial_count);
      if (partial_count > 1u) {
        prom_reduction_add_stage(out_plan, PROM_REDUCTION_STAGE_ROW_MAX,
                                 PROM_REDUCTION_SHADER_ROW_MAX,
                                 PROM_REDUCTION_IMPLEMENTATION_ROW_MAX,
                                 request->row_count, partial_count, 1u);
      }
      prom_reduction_add_stage(out_plan, PROM_REDUCTION_STAGE_SOFTMAX_EXP_SUM,
                               PROM_REDUCTION_SHADER_SOFTMAX_EXP_SUM,
                               PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_EXP_SUM,
                               request->row_count * partial_count, request->elements_per_row, partial_count);
      if (partial_count > 1u) {
        prom_reduction_add_stage(out_plan, PROM_REDUCTION_STAGE_ROW_SUM,
                                 PROM_REDUCTION_SHADER_ROW_SUM,
                                 PROM_REDUCTION_IMPLEMENTATION_ROW_SUM,
                                 request->row_count, partial_count, 1u);
      }
      prom_reduction_add_stage(out_plan, PROM_REDUCTION_STAGE_SOFTMAX_NORMALIZE,
                               PROM_REDUCTION_SHADER_SOFTMAX_NORMALIZE,
                               PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_NORMALIZE,
                               prom_reduction_ceil_div_u32((uint32_t)total_elements, PROM_REDUCTION_LOCAL_SIZE),
                               request->elements_per_row, 1u);
    }
  }
  if (out_plan->stage_count == 0u || out_plan->stage_count > PROM_REDUCTION_MAX_STAGES) return PROM_ERROR;
  prom_reduction_assign_temporary_metadata(out_plan);
  out_plan->replay_id = prom_reduction_plan_replay_id(out_plan);
  return PROM_OK;
}

int prom_reduction_validate_plan_for_test(const PrometheusReductionPlan* plan,
                                          uint64_t available_temporary_bytes,
                                          int32_t* out_detail) {
  uint32_t stage_index;
  if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_MALFORMED_PLAN;
  if (plan == NULL || plan->struct_size < sizeof(PrometheusReductionPlan) ||
      plan->row_count == 0u || plan->elements_per_row == 0u ||
      plan->stage_count == 0u || plan->stage_count > PROM_REDUCTION_MAX_STAGES ||
      plan->local_size != PROM_REDUCTION_LOCAL_SIZE ||
      plan->elements_per_partial != PROM_REDUCTION_ELEMENTS_PER_PARTIAL ||
      plan->partial_count == 0u || plan->temporary_alignment_bytes != sizeof(float) ||
      plan->replay_id != prom_reduction_plan_replay_id(plan)) return PROM_ERROR;
  for (stage_index = 0u; stage_index < plan->stage_count; ++stage_index) {
    const PrometheusReductionStageDispatch* stage = &plan->stages[stage_index];
    const prom_shader_asset* asset = prom_shader_registry_find_shader(stage->shader_id);
    const prom_compute_implementation* implementation =
        prom_shader_registry_find_compute_implementation(stage->implementation_id);
    if (stage->groups_x == 0u || stage->groups_y != 1u || stage->groups_z != 1u ||
        stage->input_elements_per_row == 0u || stage->output_partials_per_row == 0u ||
        asset == NULL || implementation == NULL || implementation->shader_id != stage->shader_id ||
        implementation->reduction_dispatch == NULL || asset->stage_role != stage->stage_role) return PROM_ERROR;
  }
  if (available_temporary_bytes < plan->temporary_bytes) {
    if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_TEMPORARY_UNDERSIZED;
    return PROM_ERROR;
  }
  if (out_detail != NULL) *out_detail = 0;
  return PROM_OK;
}

static void prom_reduction_destroy_pipeline(VkDevice device, prom_reduction_pipeline* pipeline) {
  if (pipeline->pipeline != VK_NULL_HANDLE) vkDestroyPipeline(device, pipeline->pipeline, NULL);
  if (pipeline->shader_module != VK_NULL_HANDLE) vkDestroyShaderModule(device, pipeline->shader_module, NULL);
  memset(pipeline, 0, sizeof(*pipeline));
}

void prom_reactor_runtime_reduction_cleanup_state(void* opaque_state, VkDevice device) {
  prom_reduction_runtime_state* state = (prom_reduction_runtime_state*)opaque_state;
  uint32_t slot_index;
  uint32_t pipeline_index;
  VkCommandBuffer command_buffers[PROM_REDUCTION_RING_MAX_DEPTH * 2u];
  uint32_t command_count = 0u;
  if (state == NULL) return;
  if (device == VK_NULL_HANDLE) device = state->device;
  for (slot_index = 0u; slot_index < PROM_REDUCTION_RING_MAX_DEPTH; ++slot_index) {
    prom_reduction_slot* slot = &state->slots[slot_index];
    prom_vk_destroy_buffer(device, &slot->row_sum);
    prom_vk_destroy_buffer(device, &slot->row_max);
    prom_vk_destroy_buffer(device, &slot->scratch);
    prom_vk_destroy_buffer(device, &slot->output);
    prom_vk_destroy_buffer(device, &slot->input);
    prom_vk_destroy_buffer(device, &slot->composed_readback);
    prom_vk_destroy_buffer(device, &slot->composed_softmax_output);
    prom_vk_destroy_buffer(device, &slot->composed_c);
    prom_vk_destroy_buffer(device, &slot->composed_a);
    prom_vk_destroy_buffer(device, &slot->composed_a_upload);
    prom_vk_destroy_buffer(device, &slot->m42_audit_readback);
    prom_vk_destroy_buffer(device, &slot->m42_readback);
    prom_vk_destroy_buffer(device, &slot->m42_output);
    prom_vk_destroy_buffer(device, &slot->m42_p_packed);
    prom_vk_destroy_buffer(device, &slot->m42_probabilities);
    prom_vk_destroy_buffer(device, &slot->m42_scores);
    prom_vk_destroy_buffer(device, &slot->m42_v_packed);
    prom_vk_destroy_buffer(device, &slot->m42_k_transposed);
    prom_vk_destroy_buffer(device, &slot->m42_q_packed);
    prom_vk_destroy_buffer(device, &slot->m42_v);
    prom_vk_destroy_buffer(device, &slot->m42_k);
    prom_vk_destroy_buffer(device, &slot->m42_q);
    prom_vk_destroy_buffer(device, &slot->m42_x_packed);
    prom_vk_destroy_buffer(device, &slot->m42_x);
    prom_vk_destroy_buffer(device, &slot->m42_x_upload);
    if (slot->producer_complete != VK_NULL_HANDLE) vkDestroySemaphore(device, slot->producer_complete, NULL);
    if (slot->fence != VK_NULL_HANDLE) vkDestroyFence(device, slot->fence, NULL);
    if (slot->command_buffer != VK_NULL_HANDLE) command_buffers[command_count++] = slot->command_buffer;
    if (slot->consumer_command_buffer != VK_NULL_HANDLE) command_buffers[command_count++] = slot->consumer_command_buffer;
  }
  if (command_count > 0u && state->command_pool != VK_NULL_HANDLE) {
    vkFreeCommandBuffers(device, state->command_pool, command_count, command_buffers);
  }
  for (pipeline_index = 0u; pipeline_index < PROM_REDUCTION_PIPELINE_COUNT; ++pipeline_index) {
    prom_reduction_destroy_pipeline(device, &state->pipelines[pipeline_index]);
  }
  for (pipeline_index = 0u; pipeline_index < PROM_M40B_SGEMM_PIPELINE_COUNT; ++pipeline_index) {
    prom_reduction_destroy_pipeline(device, &state->m40b_sgemm_pipelines[pipeline_index]);
  }
  for (pipeline_index = 0u; pipeline_index < PROM_M42_PIPELINE_COUNT; ++pipeline_index) {
    prom_reduction_destroy_pipeline(device, &state->m42_pipelines[pipeline_index]);
  }
  for (pipeline_index = 0u; pipeline_index < 3u; ++pipeline_index) {
    prom_vk_destroy_buffer(device, &state->m42_weight_f16[pipeline_index]);
    prom_vk_destroy_buffer(device, &state->m42_weight_f32[pipeline_index]);
    prom_vk_destroy_buffer(device, &state->m42_weight_upload[pipeline_index]);
  }
  prom_vk_destroy_buffer(device, &state->m42_resident_x_f16);
  prom_vk_destroy_buffer(device, &state->m42_resident_x_f32);
  prom_vk_destroy_buffer(device, &state->m42_resident_x_upload);
  prom_vk_destroy_buffer(device, &state->resident_a);
  prom_vk_destroy_buffer(device, &state->resident_a_upload);
  prom_vk_destroy_buffer(device, &state->persistent_b);
  prom_vk_destroy_buffer(device, &state->persistent_b_upload);
  if (state->query_pool != VK_NULL_HANDLE) vkDestroyQueryPool(device, state->query_pool, NULL);
  if (state->pipeline_layout != VK_NULL_HANDLE) vkDestroyPipelineLayout(device, state->pipeline_layout, NULL);
  if (state->descriptor_pool != VK_NULL_HANDLE) vkDestroyDescriptorPool(device, state->descriptor_pool, NULL);
  if (state->descriptor_set_layout != VK_NULL_HANDLE) vkDestroyDescriptorSetLayout(device, state->descriptor_set_layout, NULL);
  state->magic = 0u;
  free(state);
}

static int prom_reduction_create_pipelines(prom_reduction_runtime_state* state) {
  static const uint32_t implementation_ids[PROM_REDUCTION_PIPELINE_COUNT] = {
      PROM_REDUCTION_IMPLEMENTATION_ROW_SUM,
      PROM_REDUCTION_IMPLEMENTATION_ROW_MAX,
      PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_EXP_SUM,
      PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_NORMALIZE,
      PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_FUSED,
  };
  uint32_t index;
  for (index = 0u; index < PROM_REDUCTION_PIPELINE_COUNT; ++index) {
    const prom_compute_implementation* implementation =
        prom_shader_registry_find_compute_implementation(implementation_ids[index]);
    const prom_shader_asset* asset;
    VkShaderModuleCreateInfo module_info;
    VkPipelineShaderStageCreateInfo stage_info;
    VkComputePipelineCreateInfo pipeline_info;
    VkResult result;
    if (implementation == NULL || implementation->reduction_dispatch == NULL || implementation->dispatchable == 0u) return 0;
    asset = prom_shader_registry_find_shader(implementation->shader_id);
    if (asset == NULL || asset->spirv_words == NULL || asset->spirv_size_bytes == 0u ||
        asset->entry_point == NULL || asset->descriptor_binding_count != 4u || asset->push_constant_bytes != 32u) return 0;
    memset(&module_info, 0, sizeof(module_info));
    module_info.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
    module_info.codeSize = asset->spirv_size_bytes;
    module_info.pCode = asset->spirv_words;
    result = vkCreateShaderModule(state->device, &module_info, NULL, &state->pipelines[index].shader_module);
    if (result != VK_SUCCESS) return 0;
    memset(&stage_info, 0, sizeof(stage_info));
    stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
    stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
    stage_info.module = state->pipelines[index].shader_module;
    stage_info.pName = asset->entry_point;
    memset(&pipeline_info, 0, sizeof(pipeline_info));
    pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
    pipeline_info.stage = stage_info;
    pipeline_info.layout = state->pipeline_layout;
    result = vkCreateComputePipelines(state->device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL,
                                      &state->pipelines[index].pipeline);
    if (result != VK_SUCCESS) return 0;
    state->pipelines[index].shader_id = asset->shader_id;
    state->pipelines[index].implementation_id = implementation->implementation_id;
    state->diagnostics.pipeline_create_count += 1u;
  }
  return 1;
}

static int prom_reduction_initialize_state(const prom_vk_runtime_services* services,
                                           prom_reduction_runtime_state** out_state) {
  prom_reduction_runtime_state* state;
  VkDescriptorSetLayoutBinding bindings[4];
  VkDescriptorSetLayoutCreateInfo layout_info;
  VkDescriptorPoolSize pool_size;
  VkDescriptorPoolCreateInfo pool_info;
  VkPushConstantRange push_range;
  VkPipelineLayoutCreateInfo pipeline_layout_info;
  VkDescriptorSetLayout layouts[PROM_REDUCTION_RING_MAX_DEPTH *
                                (PROM_REDUCTION_MAX_STAGES + PROM_M42_DESCRIPTOR_SET_COUNT)];
  VkDescriptorSet descriptor_sets[PROM_REDUCTION_RING_MAX_DEPTH *
                                  (PROM_REDUCTION_MAX_STAGES + PROM_M42_DESCRIPTOR_SET_COUNT)];
  VkDescriptorSetAllocateInfo descriptor_allocate_info;
  VkCommandBuffer command_buffers[PROM_REDUCTION_RING_MAX_DEPTH * 2u];
  VkCommandBufferAllocateInfo command_allocate_info;
  VkFenceCreateInfo fence_info;
  VkSemaphoreCreateInfo semaphore_info;
  VkQueryPoolCreateInfo query_info;
  uint32_t descriptor_index;
  uint32_t slot_index;
  VkResult result;
  *out_state = NULL;
  state = (prom_reduction_runtime_state*)calloc(1u, sizeof(*state));
  if (state == NULL) return 0;
  state->magic = PROM_REDUCTION_STATE_MAGIC;
  state->ring_depth = services->reduction_ring_depth;
  if (state->ring_depth == 0u || state->ring_depth > PROM_REDUCTION_RING_MAX_DEPTH) state->ring_depth = 2u;
  state->next_logical_request_id = 1u;
  state->physical_device = services->physical_device;
  state->device = services->device;
  state->queue = services->compute_queue;
  state->command_pool = services->compute_command_pool;
  state->timestamp_supported = services->timestamp_query_supported;
  state->timestamp_period_ns = services->timestamp_period_ns;
  state->reduction_test_flags = services->reduction_test_flags;
  state->diagnostics.struct_size = sizeof(state->diagnostics);
  state->diagnostics.experimental_enabled = 0u;
  state->diagnostics.production_enabled = 1u;
  state->diagnostics.configured_ring_depth = state->ring_depth;
  state->diagnostics.physical_slot_count = state->ring_depth;
  state->diagnostics.next_logical_request_id = 1u;
  state->diagnostics.validation_enabled = services->validation_enabled;
  state->diagnostics.validation_error_count = services->validation_error_count;

  memset(bindings, 0, sizeof(bindings));
  for (descriptor_index = 0u; descriptor_index < 4u; ++descriptor_index) {
    bindings[descriptor_index].binding = descriptor_index;
    bindings[descriptor_index].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
    bindings[descriptor_index].descriptorCount = 1u;
    bindings[descriptor_index].stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
  }
  memset(&layout_info, 0, sizeof(layout_info));
  layout_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO;
  layout_info.bindingCount = 4u;
  layout_info.pBindings = bindings;
  result = vkCreateDescriptorSetLayout(state->device, &layout_info, NULL, &state->descriptor_set_layout);
  if (result != VK_SUCCESS) goto fail;

  memset(&push_range, 0, sizeof(push_range));
  push_range.stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
  push_range.size = sizeof(prom_reduction_push_constants);
  memset(&pipeline_layout_info, 0, sizeof(pipeline_layout_info));
  pipeline_layout_info.sType = VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO;
  pipeline_layout_info.setLayoutCount = 1u;
  pipeline_layout_info.pSetLayouts = &state->descriptor_set_layout;
  pipeline_layout_info.pushConstantRangeCount = 1u;
  pipeline_layout_info.pPushConstantRanges = &push_range;
  result = vkCreatePipelineLayout(state->device, &pipeline_layout_info, NULL, &state->pipeline_layout);
  if (result != VK_SUCCESS) goto fail;

  pool_size.type = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  pool_size.descriptorCount = 4u * state->ring_depth *
                              (PROM_REDUCTION_MAX_STAGES + PROM_M42_DESCRIPTOR_SET_COUNT);
  memset(&pool_info, 0, sizeof(pool_info));
  pool_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO;
  pool_info.maxSets = state->ring_depth * (PROM_REDUCTION_MAX_STAGES + PROM_M42_DESCRIPTOR_SET_COUNT);
  pool_info.poolSizeCount = 1u;
  pool_info.pPoolSizes = &pool_size;
  result = vkCreateDescriptorPool(state->device, &pool_info, NULL, &state->descriptor_pool);
  if (result != VK_SUCCESS) goto fail;
  for (descriptor_index = 0u;
       descriptor_index < state->ring_depth * (PROM_REDUCTION_MAX_STAGES + PROM_M42_DESCRIPTOR_SET_COUNT);
       ++descriptor_index) {
    layouts[descriptor_index] = state->descriptor_set_layout;
  }
  memset(&descriptor_allocate_info, 0, sizeof(descriptor_allocate_info));
  descriptor_allocate_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO;
  descriptor_allocate_info.descriptorPool = state->descriptor_pool;
  descriptor_allocate_info.descriptorSetCount = state->ring_depth *
                                                (PROM_REDUCTION_MAX_STAGES + PROM_M42_DESCRIPTOR_SET_COUNT);
  descriptor_allocate_info.pSetLayouts = layouts;
  result = vkAllocateDescriptorSets(state->device, &descriptor_allocate_info, descriptor_sets);
  if (result != VK_SUCCESS) goto fail;

  memset(&command_allocate_info, 0, sizeof(command_allocate_info));
  command_allocate_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO;
  command_allocate_info.commandPool = state->command_pool;
  command_allocate_info.level = VK_COMMAND_BUFFER_LEVEL_PRIMARY;
  command_allocate_info.commandBufferCount = state->ring_depth * 2u;
  result = vkAllocateCommandBuffers(state->device, &command_allocate_info, command_buffers);
  if (result != VK_SUCCESS) goto fail;
  memset(&fence_info, 0, sizeof(fence_info));
  fence_info.sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO;
  memset(&semaphore_info, 0, sizeof(semaphore_info));
  semaphore_info.sType = VK_STRUCTURE_TYPE_SEMAPHORE_CREATE_INFO;
  for (slot_index = 0u; slot_index < state->ring_depth; ++slot_index) {
    uint32_t stage_index;
    prom_reduction_slot* slot = &state->slots[slot_index];
    slot->slot_id = slot_index;
    slot->state = PROM_ASYNC_PHYSICAL_EMPTY;
    slot->command_buffer = command_buffers[slot_index * 2u];
    slot->consumer_command_buffer = command_buffers[slot_index * 2u + 1u];
    for (stage_index = 0u; stage_index < PROM_REDUCTION_MAX_STAGES; ++stage_index) {
      slot->descriptor_sets[stage_index] =
          descriptor_sets[slot_index * (PROM_REDUCTION_MAX_STAGES + PROM_M42_DESCRIPTOR_SET_COUNT) + stage_index];
    }
    for (stage_index = 0u; stage_index < PROM_M42_DESCRIPTOR_SET_COUNT; ++stage_index) {
      slot->m42_descriptor_sets[stage_index] =
          descriptor_sets[slot_index * (PROM_REDUCTION_MAX_STAGES + PROM_M42_DESCRIPTOR_SET_COUNT) +
                          PROM_REDUCTION_MAX_STAGES + stage_index];
    }
    result = vkCreateFence(state->device, &fence_info, NULL, &slot->fence);
    if (result != VK_SUCCESS) goto fail;
    result = vkCreateSemaphore(state->device, &semaphore_info, NULL, &slot->producer_complete);
    if (result != VK_SUCCESS) goto fail;
  }
  if (state->timestamp_supported != 0u && state->timestamp_period_ns > 0.0f) {
    memset(&query_info, 0, sizeof(query_info));
    query_info.sType = VK_STRUCTURE_TYPE_QUERY_POOL_CREATE_INFO;
    query_info.queryType = VK_QUERY_TYPE_TIMESTAMP;
    query_info.queryCount = PROM_REDUCTION_QUERY_STRIDE * state->ring_depth;
    result = vkCreateQueryPool(state->device, &query_info, NULL, &state->query_pool);
    if (result != VK_SUCCESS) {
      state->query_pool = VK_NULL_HANDLE;
      state->timestamp_supported = 0u;
    }
  }
  if (!prom_reduction_create_pipelines(state)) goto fail;
  state->initialized = 1u;
  state->diagnostics.initialized = 1u;
  *out_state = state;
  return 1;

fail:
  prom_reactor_runtime_reduction_cleanup_state(state, state->device);
  return 0;
}

static prom_reduction_runtime_state* prom_reduction_ensure_state(void* handle, int32_t* out_detail) {
  prom_reduction_runtime_state* state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  prom_vk_runtime_services services;
  if (state != NULL) return state;
  if (prom_reactor_runtime_get_vk_services(handle, &services) != PROM_OK) {
    if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_RUNTIME_UNAVAILABLE;
    return NULL;
  }
  if (!prom_shader_registry_validate() || !prom_reduction_initialize_state(&services, &state)) {
    if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_RESOURCE_CREATE_FAILED;
    return NULL;
  }
  if (prom_reactor_runtime_set_reduction_state(handle, state) != PROM_OK) {
    prom_reactor_runtime_reduction_cleanup_state(state, services.device);
    if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_RESOURCE_CREATE_FAILED;
    return NULL;
  }
  return state;
}

static int prom_reduction_ensure_buffer(prom_reduction_runtime_state* state,
                                        prom_vk_buffer* buffer,
                                        VkDeviceSize required_size,
                                        VkBufferUsageFlags usage,
                                        VkMemoryPropertyFlags properties,
                                        int map_memory) {
  prom_vk_buffer replacement;
  VkResult result;
  if (required_size < PROM_REDUCTION_MIN_BINDING_BYTES) required_size = PROM_REDUCTION_MIN_BINDING_BYTES;
  if (buffer->buffer != VK_NULL_HANDLE && buffer->size >= required_size) {
    state->diagnostics.buffer_reuse_count += 1u;
    return 1;
  }
  memset(&replacement, 0, sizeof(replacement));
  result = prom_vk_create_buffer(state->physical_device, state->device, 0u, required_size, usage,
                                 properties, map_memory, &replacement);
  if (result != VK_SUCCESS) return 0;
  prom_vk_destroy_buffer(state->device, buffer);
  *buffer = replacement;
  state->diagnostics.buffer_allocation_count += 1u;
  return 1;
}

static void prom_reduction_refresh_temporary_capacity(prom_reduction_runtime_state* state) {
  uint64_t total = 0u;
  uint32_t slot_index;
  for (slot_index = 0u; slot_index < state->ring_depth; ++slot_index) {
    total += (uint64_t)state->slots[slot_index].scratch.size;
    total += (uint64_t)state->slots[slot_index].row_max.size;
    total += (uint64_t)state->slots[slot_index].row_sum.size;
  }
  state->diagnostics.temporary_capacity_bytes = total;
}

static int prom_reduction_prepare_slot_buffers(prom_reduction_runtime_state* state,
                                               prom_reduction_slot* slot,
                                               const PrometheusReductionRequest* request,
                                               const PrometheusReductionPlan* plan,
                                               uint64_t total_elements,
                                               uint64_t output_elements) {
  VkDeviceSize input_bytes = (VkDeviceSize)(total_elements * sizeof(float));
  VkDeviceSize output_bytes = (VkDeviceSize)(output_elements * sizeof(float));
  VkDeviceSize scratch_bytes = plan->partial_count > 1u
                                   ? (VkDeviceSize)((uint64_t)request->row_count * plan->partial_count * sizeof(float))
                                   : PROM_REDUCTION_MIN_BINDING_BYTES;
  VkDeviceSize row_bytes = plan->operation == PROM_REDUCTION_OPERATION_SOFTMAX &&
                                   plan->strategy == PROM_REDUCTION_STRATEGY_COMPOSED
                               ? (VkDeviceSize)((uint64_t)request->row_count * sizeof(float))
                               : PROM_REDUCTION_MIN_BINDING_BYTES;
  if (!prom_reduction_ensure_buffer(state, &slot->input, input_bytes, VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                    VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1)) return 0;
  if (!prom_reduction_ensure_buffer(state, &slot->output, output_bytes, VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                    VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1)) return 0;
  if (!prom_reduction_ensure_buffer(state, &slot->scratch, scratch_bytes, VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                    VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) return 0;
  if (!prom_reduction_ensure_buffer(state, &slot->row_max, row_bytes, VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                    VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) return 0;
  if (!prom_reduction_ensure_buffer(state, &slot->row_sum, row_bytes, VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                    VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) return 0;
  prom_reduction_refresh_temporary_capacity(state);
  return 1;
}

static void prom_reduction_reap_slots(prom_reduction_runtime_state* state, uint32_t allow_wait) {
  uint32_t slot_index;
  for (slot_index = 0u; slot_index < state->ring_depth; ++slot_index) {
    prom_reduction_slot* slot = &state->slots[slot_index];
    VkResult result;
    if (slot->state != PROM_ASYNC_PHYSICAL_QUARANTINED) continue;
    result = allow_wait != 0u
                 ? vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX)
                 : vkGetFenceStatus(state->device, slot->fence);
    if (result == VK_SUCCESS) {
      slot->state = PROM_ASYNC_PHYSICAL_READY;
      state->diagnostics.reap_count += 1u;
    }
  }
}

static prom_reduction_slot* prom_reduction_acquire_slot(prom_reduction_runtime_state* state,
                                                        uint64_t logical_request_id) {
  uint32_t offset;
  prom_reduction_reap_slots(state, 0u);
  for (offset = 0u; offset < state->ring_depth; ++offset) {
    uint32_t index = (state->acquire_cursor + offset) % state->ring_depth;
    prom_reduction_slot* slot = &state->slots[index];
    if (slot->state == PROM_ASYNC_PHYSICAL_READY) {
      slot->state = PROM_ASYNC_PHYSICAL_EMPTY;
      state->diagnostics.physical_recycle_count += 1u;
    }
    if (slot->state == PROM_ASYNC_PHYSICAL_EMPTY) {
      slot->generation += 1u;
      slot->logical_request_id = logical_request_id;
      slot->state = PROM_ASYNC_PHYSICAL_PREPARING;
      state->acquire_cursor = (index + 1u) % state->ring_depth;
      state->diagnostics.acquire_cursor = state->acquire_cursor;
      return slot;
    }
  }
  prom_reduction_reap_slots(state, 1u);
  for (offset = 0u; offset < state->ring_depth; ++offset) {
    prom_reduction_slot* slot = &state->slots[offset];
    if (slot->state == PROM_ASYNC_PHYSICAL_READY) {
      slot->state = PROM_ASYNC_PHYSICAL_PREPARING;
      slot->generation += 1u;
      slot->logical_request_id = logical_request_id;
      state->diagnostics.physical_recycle_count += 1u;
      state->acquire_cursor = (offset + 1u) % state->ring_depth;
      state->diagnostics.acquire_cursor = state->acquire_cursor;
      return slot;
    }
  }
  return NULL;
}

static VkPipeline prom_reduction_pipeline_for_implementation(const prom_reduction_runtime_state* state,
                                                             uint32_t implementation_id) {
  uint32_t index;
  for (index = 0u; index < PROM_REDUCTION_PIPELINE_COUNT; ++index) {
    if (state->pipelines[index].implementation_id == implementation_id) return state->pipelines[index].pipeline;
  }
  return VK_NULL_HANDLE;
}

static void prom_reduction_stage_bindings_for_io(const prom_reduction_slot* slot,
                                                 const PrometheusReductionPlan* plan,
                                                 uint32_t stage_index,
                                                 const prom_vk_buffer* operation_input,
                                                 const prom_vk_buffer* operation_output,
                                                 prom_reduction_buffer_bindings* out) {
  const PrometheusReductionStageDispatch* stage = &plan->stages[stage_index];
  out->input = operation_input;
  out->auxiliary0 = &slot->row_max;
  out->auxiliary1 = &slot->row_sum;
  out->output = operation_output;
  if (plan->operation == PROM_REDUCTION_OPERATION_SUM || plan->operation == PROM_REDUCTION_OPERATION_MAX) {
    if (plan->partial_count > 1u && stage_index == 0u) out->output = &slot->scratch;
    if (plan->partial_count > 1u && stage_index == 1u) out->input = &slot->scratch;
    return;
  }
  if (plan->strategy == PROM_REDUCTION_STRATEGY_FUSED_SINGLE_WORKGROUP) return;
  if (plan->partial_count > 1u) {
    if (stage_index == 0u) out->output = &slot->scratch;
    if (stage_index == 1u) { out->input = &slot->scratch; out->output = &slot->row_max; }
    if (stage_index == 2u) out->output = &slot->scratch;
    if (stage_index == 3u) { out->input = &slot->scratch; out->output = &slot->row_sum; }
  } else {
    if (stage_index == 0u) out->output = &slot->row_max;
    if (stage_index == 1u) out->output = &slot->row_sum;
  }
  (void)stage;
}

static void prom_reduction_stage_bindings(const prom_reduction_slot* slot,
                                          const PrometheusReductionPlan* plan,
                                          uint32_t stage_index,
                                          prom_reduction_buffer_bindings* out) {
  prom_reduction_stage_bindings_for_io(slot, plan, stage_index, &slot->input, &slot->output, out);
}

static void prom_reduction_update_descriptor_set(prom_reduction_runtime_state* state,
                                                 VkDescriptorSet descriptor_set,
                                                 const prom_reduction_buffer_bindings* bindings) {
  const prom_vk_buffer* buffers[4] = {bindings->input, bindings->auxiliary0, bindings->auxiliary1, bindings->output};
  VkDescriptorBufferInfo infos[4];
  VkWriteDescriptorSet writes[4];
  uint32_t binding;
  memset(infos, 0, sizeof(infos));
  memset(writes, 0, sizeof(writes));
  for (binding = 0u; binding < 4u; ++binding) {
    infos[binding].buffer = buffers[binding]->buffer;
    infos[binding].range = buffers[binding]->size;
    writes[binding].sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET;
    writes[binding].dstSet = descriptor_set;
    writes[binding].dstBinding = binding;
    writes[binding].descriptorCount = 1u;
    writes[binding].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
    writes[binding].pBufferInfo = &infos[binding];
  }
  vkUpdateDescriptorSets(state->device, 4u, writes, 0u, NULL);
  state->diagnostics.descriptor_update_count += 1u;
}

static void prom_reduction_record_barrier(VkCommandBuffer command_buffer) {
  VkMemoryBarrier barrier;
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT;
  vkCmdPipelineBarrier(command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0u, 1u, &barrier, 0u, NULL, 0u, NULL);
}

static int prom_reduction_record_request(prom_reduction_runtime_state* state,
                                         prom_reduction_slot* slot,
                                         const PrometheusReductionRequest* request,
                                         const PrometheusReductionPlan* plan,
                                         uint32_t total_elements) {
  VkCommandBufferBeginInfo begin_info;
  uint32_t stage_index;
  VkResult result;
  if (prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_COMMAND_RECORD)) return 0;
  result = vkResetCommandBuffer(slot->command_buffer, 0u);
  if (result != VK_SUCCESS) return 0;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  result = vkBeginCommandBuffer(slot->command_buffer, &begin_info);
  if (result != VK_SUCCESS) return 0;
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
    vkCmdResetQueryPool(slot->command_buffer, state->query_pool,
                        slot->slot_id * PROM_REDUCTION_QUERY_STRIDE, 2u);
    vkCmdWriteTimestamp(slot->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                        state->query_pool, slot->slot_id * PROM_REDUCTION_QUERY_STRIDE);
  }
  for (stage_index = 0u; stage_index < plan->stage_count; ++stage_index) {
    const PrometheusReductionStageDispatch* stage = &plan->stages[stage_index];
    prom_reduction_buffer_bindings bindings;
    prom_reduction_push_constants push;
    VkPipeline pipeline = prom_reduction_pipeline_for_implementation(state, stage->implementation_id);
    if (pipeline == VK_NULL_HANDLE) return 0;
    prom_reduction_stage_bindings(slot, plan, stage_index, &bindings);
    prom_reduction_update_descriptor_set(state, slot->descriptor_sets[stage_index], &bindings);
    memset(&push, 0, sizeof(push));
    push.row_count = request->row_count;
    push.elements_per_row = stage->input_elements_per_row;
    push.partials_per_row = stage->output_partials_per_row;
    push.input_row_stride = stage->input_elements_per_row;
    push.chunk_elements = PROM_REDUCTION_ELEMENTS_PER_PARTIAL;
    push.total_elements = total_elements;
    push.stage_role = stage->stage_role;
    vkCmdBindPipeline(slot->command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, pipeline);
    vkCmdBindDescriptorSets(slot->command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, state->pipeline_layout,
                            0u, 1u, &slot->descriptor_sets[stage_index], 0u, NULL);
    vkCmdPushConstants(slot->command_buffer, state->pipeline_layout, VK_SHADER_STAGE_COMPUTE_BIT,
                       0u, sizeof(push), &push);
    vkCmdDispatch(slot->command_buffer, stage->groups_x, stage->groups_y, stage->groups_z);
    if (stage_index + 1u < plan->stage_count) prom_reduction_record_barrier(slot->command_buffer);
  }
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
    vkCmdWriteTimestamp(slot->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                        state->query_pool, slot->slot_id * PROM_REDUCTION_QUERY_STRIDE + 1u);
  }
  result = vkEndCommandBuffer(slot->command_buffer);
  if (result != VK_SUCCESS) return 0;
  slot->state = PROM_ASYNC_PHYSICAL_RECORDED;
  state->diagnostics.command_record_count += 1u;
  return 1;
}

static int prom_m40b_wait_all_slots(prom_reduction_runtime_state* state) {
  uint32_t index;
  prom_reduction_reap_slots(state, 1u);
  for (index = 0u; index < state->ring_depth; ++index) {
    prom_reduction_slot* slot = &state->slots[index];
    if (slot->state == PROM_ASYNC_PHYSICAL_SUBMITTED) {
      if (vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX) != VK_SUCCESS) return 0;
      slot->state = PROM_ASYNC_PHYSICAL_READY;
    }
    if (slot->state != PROM_ASYNC_PHYSICAL_EMPTY && slot->state != PROM_ASYNC_PHYSICAL_READY) return 0;
  }
  return 1;
}

static int prom_m40b_ensure_buffer(prom_reduction_runtime_state* state,
                                   prom_vk_buffer* buffer,
                                   VkDeviceSize size,
                                   VkBufferUsageFlags usage,
                                   VkMemoryPropertyFlags properties,
                                   int map_memory,
                                   uint32_t* out_reused) {
  uint64_t allocations_before = state->diagnostics.buffer_allocation_count;
  if (!prom_reduction_ensure_buffer(state, buffer, size, usage, properties, map_memory)) return 0;
  if (state->diagnostics.buffer_allocation_count != allocations_before) {
    state->m40b_buffer_grow_count += 1u;
    if (out_reused != NULL) *out_reused = 0u;
  } else {
    state->m40b_buffer_reuse_count += 1u;
    if (out_reused != NULL) *out_reused = 1u;
  }
  return 1;
}

static int prom_m40b_pack_matrix(const float* values,
                                 uint32_t logical_rows,
                                 uint32_t logical_columns,
                                 uint32_t storage_rows,
                                 uint32_t storage_columns,
                                 uint32_t kernel,
                                 void** out_payload,
                                 size_t* out_bytes) {
  uint64_t element_count;
  uint64_t byte_count;
  uint32_t row;
  void* payload;
  if (values == NULL || out_payload == NULL || out_bytes == NULL ||
      logical_rows == 0u || logical_columns == 0u ||
      storage_rows < logical_rows || storage_columns < logical_columns ||
      !prom_m40b_checked_product_u64(storage_rows, storage_columns, &element_count)) return 0;
  if (kernel == PROM_M40B_KERNEL_A2X4) {
    if (element_count > SIZE_MAX / sizeof(float)) return 0;
    byte_count = element_count * sizeof(float);
  } else if (kernel == PROM_M40B_KERNEL_COOPERATIVE || kernel == PROM_M40B_KERNEL_CONVENTIONAL_FP16) {
    if (element_count > UINT64_MAX - 1u || ((element_count + 1u) / 2u) > SIZE_MAX / sizeof(uint32_t)) return 0;
    byte_count = ((element_count + 1u) / 2u) * sizeof(uint32_t);
  } else {
    return 0;
  }
  payload = calloc(1u, (size_t)byte_count);
  if (payload == NULL) return 0;
  for (row = 0u; row < logical_rows; ++row) {
    uint32_t column;
    for (column = 0u; column < logical_columns; ++column) {
      const float value = values[(uint64_t)row * logical_columns + column];
      const uint64_t destination = (uint64_t)row * storage_columns + column;
      if (!isfinite(value)) { free(payload); return 0; }
      if (kernel == PROM_M40B_KERNEL_A2X4) {
        ((float*)payload)[destination] = value;
      } else {
        uint32_t* words = (uint32_t*)payload;
        uint32_t bits = prom_sgemm_float32_to_fp16_bits(value);
        if ((destination & 1u) == 0u) words[destination / 2u] = (words[destination / 2u] & 0xffff0000u) | bits;
        else words[destination / 2u] = (words[destination / 2u] & 0x0000ffffu) | (bits << 16u);
      }
    }
  }
  *out_payload = payload;
  *out_bytes = (size_t)byte_count;
  return 1;
}

static void prom_m40b_compute_dimensions(uint32_t kernel,
                                         const prom_m40b_padding_plan* padding,
                                         uint32_t* out_m,
                                         uint32_t* out_n,
                                         uint32_t* out_k) {
  if (kernel == PROM_M40B_KERNEL_COOPERATIVE) {
    *out_m = padding->padded_m;
    *out_n = padding->padded_n;
    *out_k = padding->padded_k;
  } else {
    *out_m = padding->logical_m;
    *out_n = padding->logical_n;
    *out_k = padding->logical_k;
  }
}

static int prom_m40b_upload_persistent(prom_reduction_runtime_state* state,
                                       prom_vk_buffer* upload,
                                       prom_vk_buffer* storage,
                                       const void* payload,
                                       size_t payload_bytes,
                                       uint64_t logical_request_id,
                                       uint64_t* out_upload_ns,
                                       uint32_t* out_reused) {
  prom_reduction_slot* slot;
  VkCommandBufferBeginInfo begin_info;
  VkBufferMemoryBarrier barrier;
  VkBufferCopy copy;
  VkSubmitInfo submit;
  VkResult result;
  uint64_t begin_ns;
  uint32_t upload_reused = 0u;
  uint32_t storage_reused = 0u;
  if (!prom_m40b_wait_all_slots(state)) return 0;
  slot = prom_reduction_acquire_slot(state, logical_request_id);
  if (slot == NULL) return 0;
  if (!prom_m40b_ensure_buffer(state, upload, (VkDeviceSize)payload_bytes,
                               VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                               VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1,
                               &upload_reused) ||
      !prom_m40b_ensure_buffer(state, storage, (VkDeviceSize)payload_bytes,
                               VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &storage_reused)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    return 0;
  }
  memcpy(upload->mapped, payload, payload_bytes);
  if (vkResetCommandBuffer(slot->command_buffer, 0u) != VK_SUCCESS) { slot->state = PROM_ASYNC_PHYSICAL_READY; return 0; }
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(slot->command_buffer, &begin_info) != VK_SUCCESS) { slot->state = PROM_ASYNC_PHYSICAL_READY; return 0; }
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_HOST_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = upload->buffer;
  barrier.size = payload_bytes;
  vkCmdPipelineBarrier(slot->command_buffer, VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT,
                       0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  memset(&copy, 0, sizeof(copy)); copy.size = payload_bytes;
  vkCmdCopyBuffer(slot->command_buffer, upload->buffer, storage->buffer, 1u, &copy);
  barrier.srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
  barrier.buffer = storage->buffer;
  vkCmdPipelineBarrier(slot->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  if (vkEndCommandBuffer(slot->command_buffer) != VK_SUCCESS ||
      vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    return 0;
  }
  memset(&submit, 0, sizeof(submit)); submit.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit.commandBufferCount = 1u; submit.pCommandBuffers = &slot->command_buffer;
  begin_ns = prom_reduction_now_ns();
  result = vkQueueSubmit(state->queue, 1u, &submit, slot->fence);
  if (result != VK_SUCCESS) { slot->state = PROM_ASYNC_PHYSICAL_READY; return 0; }
  slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
  result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
  if (result != VK_SUCCESS) { slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED; return 0; }
  if (out_upload_ns != NULL) *out_upload_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  if (out_reused != NULL) *out_reused = upload_reused != 0u && storage_reused != 0u;
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  return 1;
}

static int prom_m40b_prepare_persistent_common(void* handle,
                                               const prom_m40b_prepare_request* request,
                                               uint32_t prepare_b,
                                               prom_m40b_prepare_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_m40b_padding_plan padding;
  uint32_t storage_rows;
  uint32_t storage_columns;
  uint32_t compute_m;
  uint32_t compute_n;
  uint32_t compute_k;
  void* payload = NULL;
  size_t payload_bytes = 0u;
  uint64_t conversion_begin;
  uint64_t current_generation;
  int32_t detail = 0;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  if (request == NULL || request->values == NULL || request->generation == 0u ||
      request->kernel < PROM_M40B_KERNEL_COOPERATIVE || request->kernel > PROM_M40B_KERNEL_CONVENTIONAL_FP16 ||
      prom_m40b_calculate_padding_plan(request->m, request->n, request->k, &padding) != PROM_OK ||
      request->m > PROM_REDUCTION_MAX_ROWS || request->n > PROM_REDUCTION_MAX_ELEMENTS_PER_ROW ||
      (uint64_t)request->m * request->n > PROM_REDUCTION_MAX_TOTAL_ELEMENTS) {
    out_result->stage = PROM_STAGE_INIT; out_result->detail_code = PROM_M40B_DETAIL_INVALID_REQUEST; return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL) { out_result->stage = PROM_STAGE_INIT; out_result->detail_code = detail; return PROM_ERROR; }
  current_generation = prepare_b != 0u ? state->persistent_b_generation : state->resident_a_generation;
  if (request->generation <= current_generation) {
    out_result->stage = PROM_STAGE_INIT; out_result->detail_code = PROM_M40B_DETAIL_STALE_GENERATION; return PROM_ERROR;
  }
  prom_m40b_compute_dimensions(request->kernel, &padding, &compute_m, &compute_n, &compute_k);
  if (prepare_b != 0u) { storage_rows = compute_k; storage_columns = compute_n; }
  else { storage_rows = compute_m; storage_columns = compute_k; }
  conversion_begin = prom_reduction_now_ns();
  if (!prom_m40b_pack_matrix(request->values,
                             prepare_b != 0u ? request->k : request->m,
                             prepare_b != 0u ? request->n : request->k,
                             storage_rows, storage_columns, request->kernel,
                             &payload, &payload_bytes)) {
    out_result->stage = PROM_STAGE_TRANSFER_IN; out_result->detail_code = PROM_M40B_DETAIL_INVALID_REQUEST; return PROM_ERROR;
  }
  out_result->conversion_ns = prom_reduction_elapsed_ns(conversion_begin, prom_reduction_now_ns());
  if (!prom_m40b_upload_persistent(state,
                                   prepare_b != 0u ? &state->persistent_b_upload : &state->resident_a_upload,
                                   prepare_b != 0u ? &state->persistent_b : &state->resident_a,
                                   payload, payload_bytes, state->next_logical_request_id++,
                                   &out_result->upload_ns, &out_result->buffer_reused)) {
    free(payload); out_result->stage = PROM_STAGE_TRANSFER_IN; out_result->detail_code = PROM_M40B_DETAIL_RESOURCE; return PROM_ERROR;
  }
  state->diagnostics.next_logical_request_id = state->next_logical_request_id;
  free(payload);
  if (prepare_b != 0u) {
    state->persistent_b_generation = request->generation;
    state->persistent_b_kernel = request->kernel;
    state->persistent_b_padding = padding;
  } else {
    state->resident_a_generation = request->generation;
    state->resident_a_kernel = request->kernel;
    state->resident_a_padding = padding;
  }
  out_result->generation = request->generation;
  out_result->retained_bytes = payload_bytes;
  out_result->replaced = current_generation != 0u ? 1u : 0u;
  out_result->padding = padding;
  return PROM_OK;
}

int prom_reactor_runtime_m40b_prepare_persistent_b(void* handle,
                                                   const prom_m40b_prepare_request* request,
                                                   prom_m40b_prepare_result* out_result) {
  return prom_m40b_prepare_persistent_common(handle, request, 1u, out_result);
}

int prom_reactor_runtime_m40b_prepare_resident_a(void* handle,
                                                 const prom_m40b_prepare_request* request,
                                                 prom_m40b_prepare_result* out_result) {
  return prom_m40b_prepare_persistent_common(handle, request, 0u, out_result);
}

static int prom_m40b_ensure_sgemm_pipeline(prom_reduction_runtime_state* state, uint32_t kernel) {
  prom_reduction_pipeline* destination;
  const uint32_t* words = NULL;
  size_t bytes = 0u;
  const char* entry = NULL;
  const prom_shader_asset* asset = NULL;
  VkShaderModuleCreateInfo module_info;
  VkPipelineShaderStageCreateInfo stage_info;
  VkComputePipelineCreateInfo pipeline_info;
  VkResult result;
  if (kernel < PROM_M40B_KERNEL_COOPERATIVE || kernel > PROM_M40B_KERNEL_CONVENTIONAL_FP16) return 0;
  destination = &state->m40b_sgemm_pipelines[kernel - 1u];
  if (destination->pipeline != VK_NULL_HANDLE) return 1;
  if (kernel == PROM_M40B_KERNEL_COOPERATIVE) {
    words = k_prom_m40a_cooperative_sgemm_spirv;
    bytes = sizeof(k_prom_m40a_cooperative_sgemm_spirv);
    entry = "CooperativeSgemmF16F32M16N16K16_CS";
  } else {
    asset = prom_shader_registry_find_shader(kernel == PROM_M40B_KERNEL_A2X4 ? 12u : 14u);
    if (asset == NULL) return 0;
    words = asset->spirv_words; bytes = asset->spirv_size_bytes; entry = asset->entry_point;
  }
  memset(&module_info, 0, sizeof(module_info));
  module_info.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
  module_info.codeSize = bytes; module_info.pCode = words;
  result = vkCreateShaderModule(state->device, &module_info, NULL, &destination->shader_module);
  if (result != VK_SUCCESS) return 0;
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = destination->shader_module;
  stage_info.pName = entry;
#ifdef VK_PIPELINE_SHADER_STAGE_CREATE_REQUIRE_FULL_SUBGROUPS_BIT
  if (kernel == PROM_M40B_KERNEL_COOPERATIVE) stage_info.flags |= VK_PIPELINE_SHADER_STAGE_CREATE_REQUIRE_FULL_SUBGROUPS_BIT;
#endif
  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = state->pipeline_layout;
  result = vkCreateComputePipelines(state->device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, &destination->pipeline);
  if (result != VK_SUCCESS) {
    prom_reduction_destroy_pipeline(state->device, destination);
    return 0;
  }
  destination->shader_id = kernel == PROM_M40B_KERNEL_COOPERATIVE ? 0u : asset->shader_id;
  destination->implementation_id = kernel;
  state->m40b_pipeline_create_count += 1u;
  return 1;
}

static void prom_m40b_update_sgemm_descriptor(prom_reduction_runtime_state* state,
                                              VkDescriptorSet set,
                                              const prom_vk_buffer* a,
                                              const prom_vk_buffer* b,
                                              const prom_vk_buffer* c) {
  prom_reduction_buffer_bindings bindings;
  bindings.input = a; bindings.auxiliary0 = b; bindings.auxiliary1 = c; bindings.output = c;
  prom_reduction_update_descriptor_set(state, set, &bindings);
  state->m40b_descriptor_update_count += 1u;
}

static int prom_m40b_prepare_slot_buffers(prom_reduction_runtime_state* state,
                                          prom_reduction_slot* slot,
                                          const prom_m40b_execution_request* request,
                                          const prom_m40b_padding_plan* padding,
                                          const PrometheusReductionPlan* reduction_plan,
                                          uint32_t compute_m,
                                          uint32_t compute_n,
                                          uint32_t compute_k) {
  uint64_t a_elements;
  uint64_t c_elements;
  VkDeviceSize a_bytes;
  VkDeviceSize c_bytes;
  VkDeviceSize logical_bytes = (VkDeviceSize)padding->logical_output_bytes;
  VkDeviceSize scratch_bytes = reduction_plan->partial_count > 1u
                                   ? (VkDeviceSize)((uint64_t)request->m * reduction_plan->partial_count * sizeof(float))
                                   : PROM_REDUCTION_MIN_BINDING_BYTES;
  VkDeviceSize row_bytes = reduction_plan->strategy == PROM_REDUCTION_STRATEGY_COMPOSED
                               ? (VkDeviceSize)((uint64_t)request->m * sizeof(float))
                               : PROM_REDUCTION_MIN_BINDING_BYTES;
  if (!prom_m40b_checked_product_u64(compute_m, compute_k, &a_elements) ||
      !prom_m40b_checked_product_u64(compute_m, compute_n, &c_elements)) return 0;
  a_bytes = request->kernel == PROM_M40B_KERNEL_A2X4
                ? (VkDeviceSize)(a_elements * sizeof(float))
                : (VkDeviceSize)(((a_elements + 1u) / 2u) * sizeof(uint32_t));
  c_bytes = (VkDeviceSize)(c_elements * sizeof(float));
  if (request->input_mode == PROM_M40B_INPUT_HOST_A_PERSISTENT_B &&
      (!prom_m40b_ensure_buffer(state, &slot->composed_a_upload, a_bytes, VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                                VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1, NULL) ||
       !prom_m40b_ensure_buffer(state, &slot->composed_a, a_bytes,
                                VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL))) return 0;
  if (!prom_m40b_ensure_buffer(state, &slot->composed_c, c_bytes, VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
      !prom_m40b_ensure_buffer(state, &slot->composed_softmax_output, logical_bytes,
                               VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
      !prom_m40b_ensure_buffer(state, &slot->composed_readback, logical_bytes, VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                               VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1, NULL) ||
      !prom_m40b_ensure_buffer(state, &slot->scratch, scratch_bytes, VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
      !prom_m40b_ensure_buffer(state, &slot->row_max, row_bytes, VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
      !prom_m40b_ensure_buffer(state, &slot->row_sum, row_bytes, VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL)) return 0;
  return 1;
}

static int prom_m40b_record_producer(prom_reduction_runtime_state* state,
                                     prom_reduction_slot* slot,
                                     const prom_m40b_execution_request* request,
                                     VkDeviceSize a_copy_bytes,
                                     uint32_t compute_m,
                                     uint32_t compute_n,
                                     uint32_t compute_k,
                                     uint32_t leave_open) {
  VkCommandBuffer command_buffer = slot->command_buffer;
  VkCommandBufferBeginInfo begin_info;
  VkBufferMemoryBarrier barrier;
  VkBufferCopy copy;
  prom_m40b_sgemm_push_constants push;
  VkPipeline pipeline = state->m40b_sgemm_pipelines[request->kernel - 1u].pipeline;
  uint32_t query_base = slot->slot_id * PROM_REDUCTION_QUERY_STRIDE;
  uint32_t groups_x;
  uint32_t groups_y;
  if (pipeline == VK_NULL_HANDLE || vkResetCommandBuffer(command_buffer, 0u) != VK_SUCCESS) return 0;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(command_buffer, &begin_info) != VK_SUCCESS) return 0;
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
    vkCmdResetQueryPool(command_buffer, state->query_pool, query_base, PROM_REDUCTION_QUERY_STRIDE);
    vkCmdWriteTimestamp(command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, state->query_pool, query_base);
  }
  if (request->input_mode == PROM_M40B_INPUT_HOST_A_PERSISTENT_B) {
    memset(&barrier, 0, sizeof(barrier));
    barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barrier.srcAccessMask = VK_ACCESS_HOST_WRITE_BIT;
    barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
    barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barrier.buffer = slot->composed_a_upload.buffer;
    barrier.size = slot->composed_a_upload.size;
    vkCmdPipelineBarrier(command_buffer, VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT,
                         0u, 0u, NULL, 1u, &barrier, 0u, NULL);
    memset(&copy, 0, sizeof(copy)); copy.size = a_copy_bytes;
    vkCmdCopyBuffer(command_buffer, slot->composed_a_upload.buffer, slot->composed_a.buffer, 1u, &copy);
    barrier.srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
    barrier.dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
    barrier.buffer = slot->composed_a.buffer;
    barrier.size = slot->composed_a.size;
    vkCmdPipelineBarrier(command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                         0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  }
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
    vkCmdWriteTimestamp(command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, state->query_pool, query_base + 1u);
    vkCmdWriteTimestamp(command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, state->query_pool, query_base + 2u);
  }
  memset(&push, 0, sizeof(push)); push.m = compute_m; push.n = compute_n; push.k = compute_k;
  vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, pipeline);
  vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, state->pipeline_layout,
                          0u, 1u, &slot->descriptor_sets[0], 0u, NULL);
  vkCmdPushConstants(command_buffer, state->pipeline_layout, VK_SHADER_STAGE_COMPUTE_BIT,
                     0u, sizeof(push), &push);
  if (request->kernel == PROM_M40B_KERNEL_COOPERATIVE) {
    groups_x = prom_reduction_ceil_div_u32(compute_m, 16u);
    groups_y = prom_reduction_ceil_div_u32(compute_n, 16u);
  } else if (request->kernel == PROM_M40B_KERNEL_A2X4) {
    groups_x = prom_reduction_ceil_div_u32(compute_m, 16u);
    groups_y = prom_reduction_ceil_div_u32(compute_n, 32u);
  } else {
    groups_x = prom_reduction_ceil_div_u32(compute_m, 8u);
    groups_y = prom_reduction_ceil_div_u32(compute_n, 8u);
  }
  vkCmdDispatch(command_buffer, groups_x, groups_y, 1u);
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
    vkCmdWriteTimestamp(command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, state->query_pool, query_base + 3u);
  }
  if (leave_open == 0u && vkEndCommandBuffer(command_buffer) != VK_SUCCESS) return 0;
  return 1;
}

static int prom_m40b_record_consumer(prom_reduction_runtime_state* state,
                                     prom_reduction_slot* slot,
                                     const prom_m40b_execution_request* request,
                                     const PrometheusReductionPlan* plan,
                                     uint32_t input_row_stride,
                                     uint32_t already_open) {
  VkCommandBuffer command_buffer = already_open != 0u ? slot->command_buffer : slot->consumer_command_buffer;
  VkCommandBufferBeginInfo begin_info;
  VkBufferMemoryBarrier barrier;
  VkBufferCopy copy;
  uint32_t query_base = slot->slot_id * PROM_REDUCTION_QUERY_STRIDE;
  uint32_t stage_index;
  if (already_open == 0u) {
    if (vkResetCommandBuffer(command_buffer, 0u) != VK_SUCCESS) return 0;
    memset(&begin_info, 0, sizeof(begin_info));
    begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
    begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
    if (vkBeginCommandBuffer(command_buffer, &begin_info) != VK_SUCCESS) return 0;
  }
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = slot->composed_c.buffer;
  barrier.size = slot->composed_c.size;
  vkCmdPipelineBarrier(command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
    vkCmdWriteTimestamp(command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, state->query_pool, query_base + 4u);
  }
  for (stage_index = 0u; stage_index < plan->stage_count; ++stage_index) {
    const PrometheusReductionStageDispatch* stage = &plan->stages[stage_index];
    prom_reduction_buffer_bindings bindings;
    prom_reduction_push_constants push;
    VkPipeline pipeline = prom_reduction_pipeline_for_implementation(state, stage->implementation_id);
    if (pipeline == VK_NULL_HANDLE || stage_index + 1u >= PROM_REDUCTION_MAX_STAGES) return 0;
    prom_reduction_stage_bindings_for_io(slot, plan, stage_index, &slot->composed_c,
                                         &slot->composed_softmax_output, &bindings);
    prom_reduction_update_descriptor_set(state, slot->descriptor_sets[stage_index + 1u], &bindings);
    state->m40b_descriptor_update_count += 1u;
    memset(&push, 0, sizeof(push));
    push.row_count = request->m;
    push.elements_per_row = stage->input_elements_per_row;
    push.partials_per_row = stage->output_partials_per_row;
    push.input_row_stride = bindings.input == &slot->composed_c ? input_row_stride : stage->input_elements_per_row;
    push.chunk_elements = PROM_REDUCTION_ELEMENTS_PER_PARTIAL;
    push.total_elements = request->m * request->n;
    push.stage_role = stage->stage_role;
    vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, pipeline);
    vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, state->pipeline_layout,
                            0u, 1u, &slot->descriptor_sets[stage_index + 1u], 0u, NULL);
    vkCmdPushConstants(command_buffer, state->pipeline_layout, VK_SHADER_STAGE_COMPUTE_BIT,
                       0u, sizeof(push), &push);
    vkCmdDispatch(command_buffer, stage->groups_x, stage->groups_y, stage->groups_z);
    if (stage_index + 1u < plan->stage_count) prom_reduction_record_barrier(command_buffer);
  }
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
    vkCmdWriteTimestamp(command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, state->query_pool, query_base + 5u);
  }
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = slot->composed_softmax_output.buffer;
  barrier.size = slot->composed_softmax_output.size;
  vkCmdPipelineBarrier(command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT,
                       0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
    vkCmdWriteTimestamp(command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, state->query_pool, query_base + 6u);
  }
  memset(&copy, 0, sizeof(copy)); copy.size = (VkDeviceSize)((uint64_t)request->m * request->n * sizeof(float));
  vkCmdCopyBuffer(command_buffer, slot->composed_softmax_output.buffer, slot->composed_readback.buffer, 1u, &copy);
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
    vkCmdWriteTimestamp(command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, state->query_pool, query_base + 7u);
  }
  barrier.srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_HOST_READ_BIT;
  barrier.buffer = slot->composed_readback.buffer;
  barrier.size = copy.size;
  vkCmdPipelineBarrier(command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_HOST_BIT,
                       0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  if (vkEndCommandBuffer(command_buffer) != VK_SUCCESS) return 0;
  return 1;
}

static uint64_t prom_m40b_retained_bytes(const prom_reduction_runtime_state* state,
                                         const prom_reduction_slot* slot) {
  return (uint64_t)state->persistent_b.size + (uint64_t)state->persistent_b_upload.size +
         (uint64_t)state->resident_a.size + (uint64_t)state->resident_a_upload.size +
         (uint64_t)slot->composed_a_upload.size + (uint64_t)slot->composed_a.size +
         (uint64_t)slot->composed_c.size + (uint64_t)slot->composed_softmax_output.size +
         (uint64_t)slot->composed_readback.size + (uint64_t)slot->scratch.size +
         (uint64_t)slot->row_max.size + (uint64_t)slot->row_sum.size;
}

int prom_reactor_runtime_m40b_execute(void* handle,
                                      const prom_m40b_execution_request* request,
                                      prom_m40b_execution_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_reduction_slot* slot = NULL;
  prom_m40b_padding_plan padding;
  PrometheusReductionRequest reduction_request;
  PrometheusReductionPlan reduction_plan;
  prom_vk_runtime_services services_before;
  prom_vk_runtime_services services_after;
  const prom_vk_buffer* a_buffer;
  void* packed_a = NULL;
  size_t packed_a_bytes = 0u;
  uint32_t compute_m;
  uint32_t compute_n;
  uint32_t compute_k;
  uint64_t timestamps[PROM_REDUCTION_QUERY_STRIDE];
  uint64_t begin_ns = prom_reduction_now_ns();
  uint64_t conversion_begin;
  uint64_t submit_begin;
  uint64_t readback_begin;
  uint64_t readback_cpu_ns;
  uint64_t logical_request_id;
  uint32_t query_base;
  int32_t detail = 0;
  VkSubmitInfo submits[2];
  VkPipelineStageFlags wait_stage = VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT;
  VkResult result;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  if (request == NULL || request->output == NULL || request->m == 0u || request->n == 0u || request->k == 0u ||
      request->kernel < PROM_M40B_KERNEL_COOPERATIVE || request->kernel > PROM_M40B_KERNEL_CONVENTIONAL_FP16 ||
      request->input_mode < PROM_M40B_INPUT_HOST_A_PERSISTENT_B || request->input_mode > PROM_M40B_INPUT_DEVICE_A_PERSISTENT_B ||
      request->submit_plan < PROM_M40B_SUBMIT_ONE_COMMAND_BUFFER || request->submit_plan > PROM_M40B_SUBMIT_TWO_BOUNDED ||
      (request->input_mode == PROM_M40B_INPUT_HOST_A_PERSISTENT_B && request->host_a == NULL) ||
      prom_m40b_calculate_padding_plan(request->m, request->n, request->k, &padding) != PROM_OK ||
      request->m > PROM_REDUCTION_MAX_ROWS || request->n > PROM_REDUCTION_MAX_ELEMENTS_PER_ROW ||
      (uint64_t)request->m * request->n > PROM_REDUCTION_MAX_TOTAL_ELEMENTS) {
    out_result->stage = PROM_STAGE_INIT; out_result->detail_code = PROM_M40B_DETAIL_INVALID_REQUEST; return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL) { out_result->stage = PROM_STAGE_INIT; out_result->detail_code = detail; return PROM_ERROR; }
  if (prom_reactor_runtime_get_vk_services(handle, &services_before) != PROM_OK) {
    out_result->stage = PROM_STAGE_INIT; out_result->detail_code = PROM_M40B_DETAIL_CAPABILITY; return PROM_ERROR;
  }
  if (request->kernel == PROM_M40B_KERNEL_COOPERATIVE &&
      (services_before.cooperative_matrix_state < PROM_VK_COOPERATIVE_MATRIX_DEVICE_FEATURE_ENABLED ||
       services_before.cooperative_matrix_feature_enabled == 0u || services_before.subgroup_size != 32u)) {
    out_result->stage = PROM_STAGE_INIT; out_result->detail_code = PROM_M40B_DETAIL_CAPABILITY; return PROM_ERROR;
  }
  if (state->persistent_b_generation == 0u || request->required_b_generation != state->persistent_b_generation ||
      state->persistent_b_kernel != request->kernel || state->persistent_b_padding.replay_id != padding.replay_id) {
    out_result->stage = PROM_STAGE_INIT; out_result->detail_code = PROM_M40B_DETAIL_STALE_GENERATION; return PROM_ERROR;
  }
  if (request->input_mode == PROM_M40B_INPUT_DEVICE_A_PERSISTENT_B &&
      (state->resident_a_generation == 0u || request->required_a_generation != state->resident_a_generation ||
       state->resident_a_kernel != request->kernel || state->resident_a_padding.replay_id != padding.replay_id)) {
    out_result->stage = PROM_STAGE_INIT; out_result->detail_code = PROM_M40B_DETAIL_STALE_GENERATION; return PROM_ERROR;
  }
  if (!prom_m40b_ensure_sgemm_pipeline(state, request->kernel)) {
    out_result->stage = PROM_STAGE_INIT; out_result->detail_code = PROM_M40B_DETAIL_RESOURCE; return PROM_ERROR;
  }
  prom_m40b_compute_dimensions(request->kernel, &padding, &compute_m, &compute_n, &compute_k);
  memset(&reduction_request, 0, sizeof(reduction_request));
  reduction_request.struct_size = sizeof(reduction_request);
  reduction_request.row_count = request->m;
  reduction_request.elements_per_row = request->n;
  reduction_request.input_element_count = (uint64_t)request->m * request->n;
  reduction_request.output_element_count = reduction_request.input_element_count;
  reduction_request.operation = PROM_REDUCTION_OPERATION_SOFTMAX;
  reduction_request.finalization = PROM_REDUCTION_FINALIZATION_STABLE_SOFTMAX;
  if (prom_reactor_reduction_plan_impl(&reduction_request, &reduction_plan) != PROM_OK) {
    out_result->stage = PROM_STAGE_INIT; out_result->detail_code = PROM_M40B_DETAIL_INVALID_REQUEST; return PROM_ERROR;
  }
  logical_request_id = state->next_logical_request_id++;
  state->diagnostics.next_logical_request_id = state->next_logical_request_id;
  slot = prom_reduction_acquire_slot(state, logical_request_id);
  if (slot == NULL) { out_result->stage = PROM_STAGE_SUBMIT; out_result->detail_code = PROM_M40B_DETAIL_COMPLETION_UNCERTAIN; return PROM_ERROR; }
  out_result->logical_request_id = logical_request_id;
  out_result->physical_slot_id = slot->slot_id;
  out_result->physical_slot_generation = slot->generation;
  out_result->padding = padding;
  out_result->reduction_replay_id = reduction_plan.replay_id;
  out_result->reduction_stage_count = reduction_plan.stage_count;
  out_result->cooperative_shader_hash = request->kernel == PROM_M40B_KERNEL_COOPERATIVE ? PROM_M40B_COOPERATIVE_SHADER_HASH : 0u;
  out_result->persistent_b_generation = state->persistent_b_generation;
  out_result->resident_a_generation = request->input_mode == PROM_M40B_INPUT_DEVICE_A_PERSISTENT_B ? state->resident_a_generation : 0u;
  prom_m40b_plan_command_trace(request->input_mode, request->submit_plan, reduction_plan.stage_count,
                               &out_result->command_trace);
  out_result->command_plan_replay_id = out_result->command_trace.replay_id;
  if (!prom_m40b_prepare_slot_buffers(state, slot, request, &padding, &reduction_plan,
                                      compute_m, compute_n, compute_k)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY; out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_TRANSFER_IN; out_result->detail_code = PROM_M40B_DETAIL_RESOURCE; return PROM_ERROR;
  }
  if (request->input_mode == PROM_M40B_INPUT_HOST_A_PERSISTENT_B) {
    conversion_begin = prom_reduction_now_ns();
    if (!prom_m40b_pack_matrix(request->host_a, request->m, request->k, compute_m, compute_k,
                               request->kernel, &packed_a, &packed_a_bytes) ||
        packed_a_bytes > slot->composed_a_upload.size) {
      free(packed_a); slot->state = PROM_ASYNC_PHYSICAL_READY; out_result->physical_slot_recyclable = 1u;
      out_result->stage = PROM_STAGE_TRANSFER_IN; out_result->detail_code = PROM_M40B_DETAIL_INVALID_REQUEST; return PROM_ERROR;
    }
    memcpy(slot->composed_a_upload.mapped, packed_a, packed_a_bytes);
    free(packed_a);
    out_result->a_conversion_ns = prom_reduction_elapsed_ns(conversion_begin, prom_reduction_now_ns());
    a_buffer = &slot->composed_a;
  } else {
    a_buffer = &state->resident_a;
  }
  prom_m40b_update_sgemm_descriptor(state, slot->descriptor_sets[0], a_buffer,
                                    &state->persistent_b, &slot->composed_c);
  memset(&out_result->intermediate_c, 0, sizeof(out_result->intermediate_c));
  out_result->intermediate_c.buffer = slot->composed_c.buffer;
  out_result->intermediate_c.byte_length = slot->composed_c.size;
  out_result->intermediate_c.element_type = PROM_DEVICE_ELEMENT_F32;
  out_result->intermediate_c.logical_rows = request->m;
  out_result->intermediate_c.logical_columns = request->n;
  out_result->intermediate_c.row_stride_elements = compute_n;
  out_result->intermediate_c.layout = PROM_DEVICE_LAYOUT_ROW_MAJOR;
  out_result->intermediate_c.producer_access = PROM_DEVICE_ACCESS_COMPUTE_WRITE;
  out_result->intermediate_c.required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
  out_result->intermediate_c.owning_device = state->device;
  out_result->intermediate_c.owning_lifetime_id = logical_request_id;
  out_result->intermediate_c.owning_slot_id = slot->slot_id;
  out_result->intermediate_c.owning_slot_generation = slot->generation;
  if (prom_m40b_validate_device_buffer_view(&out_result->intermediate_c, state->device,
                                            PROM_DEVICE_ELEMENT_F32, request->m, request->n,
                                            PROM_DEVICE_ACCESS_COMPUTE_READ, &detail) != PROM_OK) {
    slot->state = PROM_ASYNC_PHYSICAL_READY; out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_INIT; out_result->detail_code = detail; return PROM_ERROR;
  }
  if (prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_COMMAND_RECORD) ||
      !prom_m40b_record_producer(state, slot, request, (VkDeviceSize)packed_a_bytes,
                                 compute_m, compute_n, compute_k,
                                 request->submit_plan == PROM_M40B_SUBMIT_ONE_COMMAND_BUFFER) ||
      !prom_m40b_record_consumer(state, slot, request, &reduction_plan, compute_n,
                                 request->submit_plan == PROM_M40B_SUBMIT_ONE_COMMAND_BUFFER)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY; out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_SUBMIT; out_result->detail_code = PROM_M40B_DETAIL_COMMAND; return PROM_ERROR;
  }
  slot->composed_command_reuse_count += 1u;
  if (vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS ||
      prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_QUEUE_SUBMIT)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY; out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_SUBMIT; out_result->detail_code = PROM_M40B_DETAIL_SUBMIT; return PROM_ERROR;
  }
  memset(submits, 0, sizeof(submits));
  submits[0].sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submits[0].commandBufferCount = 1u;
  submits[0].pCommandBuffers = &slot->command_buffer;
  if (request->submit_plan == PROM_M40B_SUBMIT_TWO_BOUNDED) {
    submits[0].signalSemaphoreCount = 1u;
    submits[0].pSignalSemaphores = &slot->producer_complete;
    submits[1].sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
    submits[1].waitSemaphoreCount = 1u;
    submits[1].pWaitSemaphores = &slot->producer_complete;
    submits[1].pWaitDstStageMask = &wait_stage;
    submits[1].commandBufferCount = 1u;
    submits[1].pCommandBuffers = &slot->consumer_command_buffer;
  }
  submit_begin = prom_reduction_now_ns();
  result = vkQueueSubmit(state->queue,
                         request->submit_plan == PROM_M40B_SUBMIT_TWO_BOUNDED ? 2u : 1u,
                         submits, slot->fence);
  out_result->cpu_submission_ns = prom_reduction_elapsed_ns(submit_begin, prom_reduction_now_ns());
  if (result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_READY; out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_SUBMIT; out_result->detail_code = PROM_M40B_DETAIL_SUBMIT; return PROM_ERROR;
  }
  slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
  if (prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_COMPLETION_OBSERVATION)) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    out_result->stage = PROM_STAGE_SUBMIT; out_result->detail_code = PROM_M40B_DETAIL_COMPLETION_UNCERTAIN; return PROM_ERROR;
  }
  result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
  if (result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    out_result->stage = PROM_STAGE_SUBMIT; out_result->detail_code = PROM_M40B_DETAIL_COMPLETION_UNCERTAIN; return PROM_ERROR;
  }
  slot->state = PROM_ASYNC_PHYSICAL_COMPLETE;
  query_base = slot->slot_id * PROM_REDUCTION_QUERY_STRIDE;
  memset(timestamps, 0, sizeof(timestamps));
  if (state->timestamp_supported == 0u || state->query_pool == VK_NULL_HANDLE ||
      vkGetQueryPoolResults(state->device, state->query_pool, query_base, PROM_M40B_QUERY_COUNT,
                            sizeof(timestamps), timestamps, sizeof(uint64_t), VK_QUERY_RESULT_64_BIT) != VK_SUCCESS ||
      timestamps[1] < timestamps[0] || timestamps[2] < timestamps[1] || timestamps[3] <= timestamps[2] ||
      timestamps[4] < timestamps[3] || timestamps[5] <= timestamps[4] ||
      timestamps[6] < timestamps[5] || timestamps[7] <= timestamps[6]) {
    slot->state = PROM_ASYNC_PHYSICAL_READY; out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_TRANSFER_OUT; out_result->detail_code = PROM_M40B_DETAIL_QUERY; return PROM_ERROR;
  }
  out_result->a_upload_ns = (uint64_t)((double)(timestamps[1] - timestamps[0]) * state->timestamp_period_ns);
  out_result->sgemm_gpu_ns = (uint64_t)((double)(timestamps[3] - timestamps[2]) * state->timestamp_period_ns);
  out_result->handoff_gpu_ns = (uint64_t)((double)(timestamps[4] - timestamps[3]) * state->timestamp_period_ns);
  out_result->softmax_gpu_ns = (uint64_t)((double)(timestamps[5] - timestamps[4]) * state->timestamp_period_ns);
  out_result->combined_gpu_ns = (uint64_t)((double)(timestamps[5] - timestamps[2]) * state->timestamp_period_ns);
  readback_begin = prom_reduction_now_ns();
  memcpy(request->output, slot->composed_readback.mapped, (size_t)padding.logical_output_bytes);
  readback_cpu_ns = prom_reduction_elapsed_ns(readback_begin, prom_reduction_now_ns());
  out_result->final_readback_ns = (uint64_t)((double)(timestamps[7] - timestamps[6]) * state->timestamp_period_ns) + readback_cpu_ns;
  out_result->correctness_readback_count = 1u;
  out_result->no_intermediate_host_copy = 1u;
  out_result->submit_count = request->submit_plan == PROM_M40B_SUBMIT_TWO_BOUNDED ? 2u : 1u;
  out_result->retained_bytes = prom_m40b_retained_bytes(state, slot);
  out_result->buffer_allocation_count = state->m40b_buffer_grow_count;
  out_result->buffer_reuse_count = state->m40b_buffer_reuse_count;
  out_result->descriptor_update_count = state->m40b_descriptor_update_count;
  out_result->pipeline_create_count = state->m40b_pipeline_create_count + state->diagnostics.pipeline_create_count;
  out_result->command_buffer_reuse_count = slot->composed_command_reuse_count;
  out_result->end_to_end_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  out_result->validation_error_count_before = services_before.validation_error_count;
  if (request->kernel == PROM_M40B_KERNEL_COOPERATIVE) {
    (void)prom_reactor_runtime_mark_cooperative_matrix_executable(handle);
  }
  if (prom_reactor_runtime_get_vk_services(handle, &services_after) == PROM_OK) {
    out_result->validation_error_count_after = services_after.validation_error_count;
  }
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->physical_slot_recyclable = 1u;
  out_result->stage = PROM_STAGE_NONE;
  out_result->detail_code = 0;
  return PROM_OK;
}

static int prom_m42_ensure_buffer(prom_reduction_runtime_state* state,
                                  prom_vk_buffer* buffer,
                                  VkDeviceSize size,
                                  VkBufferUsageFlags usage,
                                  VkMemoryPropertyFlags properties,
                                  int map_memory,
                                  uint32_t* out_reused) {
  uint64_t allocations_before;
  if (state == NULL || buffer == NULL) return 0;
  allocations_before = state->diagnostics.buffer_allocation_count;
  if (!prom_reduction_ensure_buffer(state, buffer, size, usage, properties, map_memory)) return 0;
  if (state->diagnostics.buffer_allocation_count != allocations_before) {
    state->m42_buffer_grow_count += 1u;
    if (out_reused != NULL) *out_reused = 0u;
  } else {
    state->m42_buffer_reuse_count += 1u;
    if (out_reused != NULL) *out_reused = 1u;
  }
  return 1;
}

static int prom_m42_ensure_pipelines(prom_reduction_runtime_state* state) {
  static const uint32_t* words[PROM_M42_PIPELINE_COUNT] = {
      k_prom_m42_attention_pack_f32_to_f16_spirv,
      k_prom_m42_attention_transpose_f32_spirv,
      k_prom_m42_attention_scale_scores_f32_spirv,
  };
  static const size_t bytes[PROM_M42_PIPELINE_COUNT] = {
      sizeof(k_prom_m42_attention_pack_f32_to_f16_spirv),
      sizeof(k_prom_m42_attention_transpose_f32_spirv),
      sizeof(k_prom_m42_attention_scale_scores_f32_spirv),
  };
  static const char* entries[PROM_M42_PIPELINE_COUNT] = {
      "AttentionPackF32ToF16_CS",
      "AttentionTransposeF32_CS",
      "AttentionScaleScoresF32_CS",
  };
  uint32_t index;
  if (state == NULL) return 0;
  for (index = 0u; index < PROM_M42_PIPELINE_COUNT; ++index) {
    prom_reduction_pipeline* destination = &state->m42_pipelines[index];
    VkShaderModuleCreateInfo module_info;
    VkPipelineShaderStageCreateInfo stage_info;
    VkComputePipelineCreateInfo pipeline_info;
    if (destination->pipeline != VK_NULL_HANDLE) continue;
    memset(&module_info, 0, sizeof(module_info));
    module_info.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
    module_info.codeSize = bytes[index];
    module_info.pCode = words[index];
    if (vkCreateShaderModule(state->device, &module_info, NULL, &destination->shader_module) != VK_SUCCESS) return 0;
    memset(&stage_info, 0, sizeof(stage_info));
    stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
    stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
    stage_info.module = destination->shader_module;
    stage_info.pName = entries[index];
    memset(&pipeline_info, 0, sizeof(pipeline_info));
    pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
    pipeline_info.stage = stage_info;
    pipeline_info.layout = state->pipeline_layout;
    if (vkCreateComputePipelines(state->device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL,
                                 &destination->pipeline) != VK_SUCCESS) {
      prom_reduction_destroy_pipeline(state->device, destination);
      return 0;
    }
    destination->implementation_id = index + 1u;
    state->m42_pipeline_create_count += 1u;
  }
  return 1;
}

static void prom_m42_update_descriptor(prom_reduction_runtime_state* state,
                                       VkDescriptorSet set,
                                       const prom_vk_buffer* input,
                                       const prom_vk_buffer* auxiliary,
                                       const prom_vk_buffer* output) {
  prom_reduction_buffer_bindings bindings;
  bindings.input = input;
  bindings.auxiliary0 = auxiliary != NULL ? auxiliary : input;
  bindings.auxiliary1 = output;
  bindings.output = output;
  prom_reduction_update_descriptor_set(state, set, &bindings);
  state->m42_descriptor_update_count += 1u;
}

static void prom_m42_buffer_barrier(VkCommandBuffer command_buffer,
                                    const prom_vk_buffer* buffer,
                                    VkAccessFlags source_access,
                                    VkAccessFlags destination_access,
                                    VkPipelineStageFlags source_stage,
                                    VkPipelineStageFlags destination_stage) {
  VkBufferMemoryBarrier barrier;
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = source_access;
  barrier.dstAccessMask = destination_access;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = buffer->buffer;
  barrier.offset = 0u;
  barrier.size = buffer->size;
  vkCmdPipelineBarrier(command_buffer, source_stage, destination_stage, 0u,
                       0u, NULL, 1u, &barrier, 0u, NULL);
}

static void prom_m42_record_pack(prom_reduction_runtime_state* state,
                                 VkCommandBuffer command_buffer,
                                 VkDescriptorSet descriptor_set,
                                 uint32_t logical_rows,
                                 uint32_t logical_columns,
                                 uint32_t input_row_stride,
                                 uint32_t output_rows,
                                 uint32_t output_columns,
                                 uint32_t transpose) {
  prom_m42_pack_push_constants push;
  uint64_t output_elements = (uint64_t)output_rows * output_columns;
  memset(&push, 0, sizeof(push));
  push.logical_rows = logical_rows;
  push.logical_columns = logical_columns;
  push.input_row_stride = input_row_stride;
  push.output_rows = output_rows;
  push.output_columns = output_columns;
  push.transpose = transpose;
  push.packed_word_count = (uint32_t)((output_elements + 1u) / 2u);
  vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, state->m42_pipelines[0].pipeline);
  vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, state->pipeline_layout,
                          0u, 1u, &descriptor_set, 0u, NULL);
  vkCmdPushConstants(command_buffer, state->pipeline_layout, VK_SHADER_STAGE_COMPUTE_BIT,
                     0u, sizeof(push), &push);
  vkCmdDispatch(command_buffer, prom_reduction_ceil_div_u32(push.packed_word_count, 256u), 1u, 1u);
}

static uint64_t prom_m42_hash_finite_matrix(const float* values, uint64_t count, uint32_t* out_finite) {
  uint64_t hash = 1469598103934665603ull;
  uint64_t index;
  if (out_finite != NULL) *out_finite = 0u;
  if (values == NULL) return 0u;
  for (index = 0u; index < count; ++index) {
    uint32_t bits;
    if (!isfinite(values[index])) return 0u;
    memcpy(&bits, &values[index], sizeof(bits));
    hash = prom_reduction_hash_u32(hash, bits);
  }
  if (out_finite != NULL) *out_finite = 1u;
  return hash;
}

static uint64_t prom_m42_weight_retained_bytes(const prom_reduction_runtime_state* state) {
  uint64_t total = 0u;
  uint32_t index;
  for (index = 0u; index < 3u; ++index) {
    total += (uint64_t)state->m42_weight_upload[index].size;
    total += (uint64_t)state->m42_weight_f32[index].size;
    total += (uint64_t)state->m42_weight_f16[index].size;
  }
  return total;
}

int prom_reactor_runtime_m42_prepare_weights(void* handle,
                                             const prom_m42_weight_prepare_request* request,
                                             prom_m42_weight_prepare_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_reduction_slot* slot = NULL;
  prom_vk_runtime_services services;
  const float* values[3];
  uint32_t padded_model;
  uint32_t padded_head;
  uint64_t element_count;
  VkDeviceSize f32_bytes;
  VkDeviceSize f16_bytes;
  uint64_t begin_ns;
  uint64_t submit_begin;
  uint64_t timestamps[2];
  uint32_t index;
  uint32_t all_reused = 1u;
  uint32_t had_previous = 0u;
  int32_t detail = 0;
  VkCommandBufferBeginInfo begin_info;
  VkBufferMemoryBarrier upload_barriers[3];
  VkBufferMemoryBarrier storage_barriers[3];
  VkBufferCopy copies[3];
  VkSubmitInfo submit;
  VkResult result;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  if (request == NULL || request->wq == NULL || request->wk == NULL || request->wv == NULL ||
      request->generation == 0u || request->value_dim != request->head_dim ||
      request->model_width == 0u || request->head_dim == 0u ||
      request->model_width > PROM_M42_MAX_MODEL_WIDTH || request->head_dim > PROM_M42_MAX_HEAD_DIM ||
      !prom_m42_round_up_16(request->model_width, &padded_model) ||
      !prom_m42_round_up_16(request->head_dim, &padded_head) ||
      !prom_m40b_checked_product_u64(request->model_width, request->head_dim, &element_count) ||
      element_count > PROM_M42_MAX_MATRIX_ELEMENTS || element_count > SIZE_MAX / sizeof(float)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M42_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  values[0] = request->wq; values[1] = request->wk; values[2] = request->wv;
  begin_ns = prom_reduction_now_ns();
  for (index = 0u; index < 3u; ++index) {
    uint32_t finite = 0u;
    const uint64_t hash = prom_m42_hash_finite_matrix(values[index], element_count, &finite);
    if (finite == 0u) {
      out_result->stage = PROM_STAGE_TRANSFER_IN;
      out_result->detail_code = PROM_M42_DETAIL_NONFINITE_INPUT;
      return PROM_ERROR;
    }
    if (index == 0u) out_result->wq_hash = hash;
    else if (index == 1u) out_result->wk_hash = hash;
    else out_result->wv_hash = hash;
  }
  out_result->validation_hash_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL) {
    out_result->stage = PROM_STAGE_INIT; out_result->detail_code = detail; return PROM_ERROR;
  }
  for (index = 0u; index < 3u; ++index) {
    if (request->generation <= state->m42_weight_generation[index]) {
      out_result->stage = PROM_STAGE_INIT;
      out_result->detail_code = PROM_M42_DETAIL_STALE_WEIGHT_GENERATION;
      return PROM_ERROR;
    }
  }
  had_previous = state->m42_weight_generation[0] != 0u;
  if (prom_reactor_runtime_get_vk_services(handle, &services) != PROM_OK ||
      !prom_m40b_wait_all_slots(state) || !prom_m42_ensure_pipelines(state) ||
      !prom_m40b_ensure_sgemm_pipeline(state, PROM_M40B_KERNEL_A2X4) ||
      !prom_m40b_ensure_sgemm_pipeline(state, PROM_M40B_KERNEL_CONVENTIONAL_FP16) ||
      (services.cooperative_matrix_feature_enabled != 0u &&
       !prom_m40b_ensure_sgemm_pipeline(state, PROM_M40B_KERNEL_COOPERATIVE))) {
    out_result->stage = PROM_STAGE_INIT; out_result->detail_code = PROM_M42_DETAIL_RESOURCE; return PROM_ERROR;
  }
  slot = prom_reduction_acquire_slot(state, state->next_logical_request_id++);
  state->diagnostics.next_logical_request_id = state->next_logical_request_id;
  if (slot == NULL) {
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M42_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  f32_bytes = (VkDeviceSize)(element_count * sizeof(float));
  f16_bytes = (VkDeviceSize)((((uint64_t)padded_model * padded_head) + 1u) / 2u * sizeof(uint32_t));
  for (index = 0u; index < 3u; ++index) {
    uint32_t reused_upload = 0u;
    uint32_t reused_f32 = 0u;
    uint32_t reused_f16 = 0u;
    if (!prom_m42_ensure_buffer(state, &state->m42_weight_upload[index], f32_bytes,
                                VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                                VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                                1, &reused_upload) ||
        !prom_m42_ensure_buffer(state, &state->m42_weight_f32[index], f32_bytes,
                                VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &reused_f32) ||
        !prom_m42_ensure_buffer(state, &state->m42_weight_f16[index], f16_bytes,
                                VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &reused_f16)) {
      slot->state = PROM_ASYNC_PHYSICAL_READY;
      out_result->stage = PROM_STAGE_TRANSFER_IN;
      out_result->detail_code = PROM_M42_DETAIL_RESOURCE;
      return PROM_ERROR;
    }
    if (reused_upload == 0u || reused_f32 == 0u || reused_f16 == 0u) all_reused = 0u;
    memcpy(state->m42_weight_upload[index].mapped, values[index], (size_t)f32_bytes);
    prom_m42_update_descriptor(state, slot->m42_descriptor_sets[index],
                               &state->m42_weight_f32[index], NULL, &state->m42_weight_f16[index]);
  }
  if (vkResetCommandBuffer(slot->command_buffer, 0u) != VK_SUCCESS) goto command_fail;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(slot->command_buffer, &begin_info) != VK_SUCCESS) goto command_fail;
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
    vkCmdResetQueryPool(slot->command_buffer, state->query_pool,
                        slot->slot_id * PROM_REDUCTION_QUERY_STRIDE, 2u);
    vkCmdWriteTimestamp(slot->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, state->query_pool,
                        slot->slot_id * PROM_REDUCTION_QUERY_STRIDE);
  }
  memset(upload_barriers, 0, sizeof(upload_barriers));
  memset(storage_barriers, 0, sizeof(storage_barriers));
  memset(copies, 0, sizeof(copies));
  for (index = 0u; index < 3u; ++index) {
    upload_barriers[index].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    upload_barriers[index].srcAccessMask = VK_ACCESS_HOST_WRITE_BIT;
    upload_barriers[index].dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
    upload_barriers[index].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    upload_barriers[index].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    upload_barriers[index].buffer = state->m42_weight_upload[index].buffer;
    upload_barriers[index].size = f32_bytes;
    copies[index].size = f32_bytes;
  }
  vkCmdPipelineBarrier(slot->command_buffer, VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT,
                       0u, 0u, NULL, 3u, upload_barriers, 0u, NULL);
  for (index = 0u; index < 3u; ++index) {
    vkCmdCopyBuffer(slot->command_buffer, state->m42_weight_upload[index].buffer,
                    state->m42_weight_f32[index].buffer, 1u, &copies[index]);
    storage_barriers[index].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    storage_barriers[index].srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
    storage_barriers[index].dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
    storage_barriers[index].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    storage_barriers[index].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    storage_barriers[index].buffer = state->m42_weight_f32[index].buffer;
    storage_barriers[index].size = f32_bytes;
  }
  vkCmdPipelineBarrier(slot->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0u, 0u, NULL,
                       3u, storage_barriers, 0u, NULL);
  for (index = 0u; index < 3u; ++index) {
    prom_m42_record_pack(state, slot->command_buffer, slot->m42_descriptor_sets[index],
                         request->model_width, request->head_dim, request->head_dim,
                         padded_model, padded_head, 0u);
  }
  for (index = 0u; index < 3u; ++index) {
    prom_m42_buffer_barrier(slot->command_buffer, &state->m42_weight_f16[index],
                            VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
  }
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
    vkCmdWriteTimestamp(slot->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, state->query_pool,
                        slot->slot_id * PROM_REDUCTION_QUERY_STRIDE + 1u);
  }
  if (vkEndCommandBuffer(slot->command_buffer) != VK_SUCCESS ||
      vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS) goto command_fail;
  memset(&submit, 0, sizeof(submit));
  submit.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit.commandBufferCount = 1u;
  submit.pCommandBuffers = &slot->command_buffer;
  submit_begin = prom_reduction_now_ns();
  result = vkQueueSubmit(state->queue, 1u, &submit, slot->fence);
  if (result != VK_SUCCESS) goto submit_fail;
  slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
  result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
  out_result->upload_and_pack_ns = prom_reduction_elapsed_ns(submit_begin, prom_reduction_now_ns());
  if (result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M42_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE &&
      vkGetQueryPoolResults(state->device, state->query_pool,
                            slot->slot_id * PROM_REDUCTION_QUERY_STRIDE, 2u,
                            sizeof(timestamps), timestamps, sizeof(uint64_t),
                            VK_QUERY_RESULT_64_BIT) == VK_SUCCESS && timestamps[1] >= timestamps[0]) {
    out_result->gpu_upload_and_pack_ns =
        (uint64_t)((double)(timestamps[1] - timestamps[0]) * state->timestamp_period_ns);
  }
  state->m42_weight_model_width = request->model_width;
  state->m42_weight_head_dim = request->head_dim;
  state->m42_weight_generation[0] = request->generation;
  state->m42_weight_generation[1] = request->generation;
  state->m42_weight_generation[2] = request->generation;
  state->m42_weight_hash[0] = out_result->wq_hash;
  state->m42_weight_hash[1] = out_result->wk_hash;
  state->m42_weight_hash[2] = out_result->wv_hash;
  out_result->wq_generation = request->generation;
  out_result->wk_generation = request->generation;
  out_result->wv_generation = request->generation;
  out_result->replaced = had_previous;
  out_result->buffer_reused = all_reused;
  out_result->retained_bytes = prom_m42_weight_retained_bytes(state);
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  return PROM_OK;

command_fail:
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->stage = PROM_STAGE_TRANSFER_IN;
  out_result->detail_code = PROM_M42_DETAIL_COMMAND;
  return PROM_ERROR;
submit_fail:
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->stage = PROM_STAGE_TRANSFER_IN;
  out_result->detail_code = PROM_M42_DETAIL_SUBMIT;
  return PROM_ERROR;
}

int prom_reactor_runtime_m42_prepare_resident_x(void* handle,
                                                const prom_m42_resident_x_prepare_request* request,
                                                prom_m42_resident_x_prepare_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_reduction_slot* slot;
  uint32_t padded_tokens;
  uint32_t padded_model;
  uint32_t padded_head;
  uint32_t finite = 0u;
  uint32_t reused_upload = 0u;
  uint32_t reused_f32 = 0u;
  uint32_t reused_f16 = 0u;
  uint64_t element_count;
  VkDeviceSize f32_bytes;
  VkDeviceSize f16_bytes;
  uint64_t begin_ns;
  uint64_t submit_begin;
  uint64_t timestamps[2];
  int32_t detail = 0;
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  VkSubmitInfo submit;
  VkResult result;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  if (request == NULL || request->x == NULL || request->generation == 0u ||
      !prom_m42_valid_shape(request->tokens, request->model_width, 1u, 1u,
                            &padded_tokens, &padded_model, &padded_head) ||
      !prom_m40b_checked_product_u64(request->tokens, request->model_width, &element_count) ||
      element_count > SIZE_MAX / sizeof(float)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M42_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  (void)padded_head;
  begin_ns = prom_reduction_now_ns();
  out_result->hash = prom_m42_hash_finite_matrix(request->x, element_count, &finite);
  out_result->validation_hash_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  if (finite == 0u) {
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M42_DETAIL_NONFINITE_INPUT;
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL) { out_result->stage = PROM_STAGE_INIT; out_result->detail_code = detail; return PROM_ERROR; }
  if (request->generation <= state->m42_resident_x_generation) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M42_DETAIL_STALE_X_GENERATION;
    return PROM_ERROR;
  }
  if (!prom_m40b_wait_all_slots(state) || !prom_m42_ensure_pipelines(state)) {
    out_result->stage = PROM_STAGE_INIT; out_result->detail_code = PROM_M42_DETAIL_RESOURCE; return PROM_ERROR;
  }
  slot = prom_reduction_acquire_slot(state, state->next_logical_request_id++);
  state->diagnostics.next_logical_request_id = state->next_logical_request_id;
  if (slot == NULL) {
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M42_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  f32_bytes = (VkDeviceSize)(element_count * sizeof(float));
  f16_bytes = (VkDeviceSize)((((uint64_t)padded_tokens * padded_model) + 1u) / 2u * sizeof(uint32_t));
  if (!prom_m42_ensure_buffer(state, &state->m42_resident_x_upload, f32_bytes,
                              VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                              1, &reused_upload) ||
      !prom_m42_ensure_buffer(state, &state->m42_resident_x_f32, f32_bytes,
                              VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &reused_f32) ||
      !prom_m42_ensure_buffer(state, &state->m42_resident_x_f16, f16_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &reused_f16)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M42_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  memcpy(state->m42_resident_x_upload.mapped, request->x, (size_t)f32_bytes);
  prom_m42_update_descriptor(state, slot->m42_descriptor_sets[0],
                             &state->m42_resident_x_f32, NULL, &state->m42_resident_x_f16);
  if (vkResetCommandBuffer(slot->command_buffer, 0u) != VK_SUCCESS) goto resident_command_fail;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(slot->command_buffer, &begin_info) != VK_SUCCESS) goto resident_command_fail;
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
    vkCmdResetQueryPool(slot->command_buffer, state->query_pool,
                        slot->slot_id * PROM_REDUCTION_QUERY_STRIDE, 2u);
    vkCmdWriteTimestamp(slot->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, state->query_pool,
                        slot->slot_id * PROM_REDUCTION_QUERY_STRIDE);
  }
  prom_m42_buffer_barrier(slot->command_buffer, &state->m42_resident_x_upload,
                          VK_ACCESS_HOST_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                          VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
  memset(&copy, 0, sizeof(copy)); copy.size = f32_bytes;
  vkCmdCopyBuffer(slot->command_buffer, state->m42_resident_x_upload.buffer,
                  state->m42_resident_x_f32.buffer, 1u, &copy);
  prom_m42_buffer_barrier(slot->command_buffer, &state->m42_resident_x_f32,
                          VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                          VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
  prom_m42_record_pack(state, slot->command_buffer, slot->m42_descriptor_sets[0],
                       request->tokens, request->model_width, request->model_width,
                       padded_tokens, padded_model, 0u);
  prom_m42_buffer_barrier(slot->command_buffer, &state->m42_resident_x_f16,
                          VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                          VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
    vkCmdWriteTimestamp(slot->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, state->query_pool,
                        slot->slot_id * PROM_REDUCTION_QUERY_STRIDE + 1u);
  }
  if (vkEndCommandBuffer(slot->command_buffer) != VK_SUCCESS ||
      vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS) goto resident_command_fail;
  memset(&submit, 0, sizeof(submit)); submit.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit.commandBufferCount = 1u; submit.pCommandBuffers = &slot->command_buffer;
  submit_begin = prom_reduction_now_ns();
  result = vkQueueSubmit(state->queue, 1u, &submit, slot->fence);
  if (result != VK_SUCCESS) goto resident_submit_fail;
  slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
  result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
  out_result->upload_and_pack_ns = prom_reduction_elapsed_ns(submit_begin, prom_reduction_now_ns());
  if (result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M42_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE &&
      vkGetQueryPoolResults(state->device, state->query_pool,
                            slot->slot_id * PROM_REDUCTION_QUERY_STRIDE, 2u,
                            sizeof(timestamps), timestamps, sizeof(uint64_t),
                            VK_QUERY_RESULT_64_BIT) == VK_SUCCESS && timestamps[1] >= timestamps[0]) {
    out_result->gpu_upload_and_pack_ns =
        (uint64_t)((double)(timestamps[1] - timestamps[0]) * state->timestamp_period_ns);
  }
  out_result->replaced = state->m42_resident_x_generation != 0u ? 1u : 0u;
  state->m42_resident_x_generation = request->generation;
  state->m42_resident_x_hash = out_result->hash;
  state->m42_resident_x_tokens = request->tokens;
  state->m42_resident_x_model_width = request->model_width;
  out_result->generation = request->generation;
  out_result->buffer_reused = reused_upload != 0u && reused_f32 != 0u && reused_f16 != 0u;
  out_result->retained_bytes = (uint64_t)state->m42_resident_x_upload.size +
                               (uint64_t)state->m42_resident_x_f32.size +
                               (uint64_t)state->m42_resident_x_f16.size;
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  return PROM_OK;

resident_command_fail:
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->stage = PROM_STAGE_TRANSFER_IN;
  out_result->detail_code = PROM_M42_DETAIL_COMMAND;
  return PROM_ERROR;
resident_submit_fail:
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->stage = PROM_STAGE_TRANSFER_IN;
  out_result->detail_code = PROM_M42_DETAIL_SUBMIT;
  return PROM_ERROR;
}

static void prom_m42_record_sgemm(prom_reduction_runtime_state* state,
                                  VkCommandBuffer command_buffer,
                                  VkDescriptorSet descriptor_set,
                                  uint32_t path,
                                  uint32_t m,
                                  uint32_t n,
                                  uint32_t k) {
  prom_m40b_sgemm_push_constants push;
  uint32_t groups_x;
  uint32_t groups_y;
  memset(&push, 0, sizeof(push));
  push.m = m; push.n = n; push.k = k;
  vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                    state->m40b_sgemm_pipelines[path - 1u].pipeline);
  vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, state->pipeline_layout,
                          0u, 1u, &descriptor_set, 0u, NULL);
  vkCmdPushConstants(command_buffer, state->pipeline_layout, VK_SHADER_STAGE_COMPUTE_BIT,
                     0u, sizeof(push), &push);
  if (path == PROM_M42_PATH_COOPERATIVE) {
    groups_x = prom_reduction_ceil_div_u32(m, 16u);
    groups_y = prom_reduction_ceil_div_u32(n, 16u);
  } else if (path == PROM_M42_PATH_A2X4) {
    groups_x = prom_reduction_ceil_div_u32(m, 16u);
    groups_y = prom_reduction_ceil_div_u32(n, 32u);
  } else {
    groups_x = prom_reduction_ceil_div_u32(m, 8u);
    groups_y = prom_reduction_ceil_div_u32(n, 8u);
  }
  vkCmdDispatch(command_buffer, groups_x, groups_y, 1u);
}

static void prom_m42_record_transpose(prom_reduction_runtime_state* state,
                                      VkCommandBuffer command_buffer,
                                      VkDescriptorSet descriptor_set,
                                      uint32_t input_rows,
                                      uint32_t input_columns,
                                      uint32_t input_stride,
                                      uint32_t output_stride) {
  prom_m42_transpose_push_constants push;
  memset(&push, 0, sizeof(push));
  push.input_rows = input_rows;
  push.input_columns = input_columns;
  push.input_row_stride = input_stride;
  push.output_row_stride = output_stride;
  push.output_element_count = input_rows * input_columns;
  vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, state->m42_pipelines[1].pipeline);
  vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, state->pipeline_layout,
                          0u, 1u, &descriptor_set, 0u, NULL);
  vkCmdPushConstants(command_buffer, state->pipeline_layout, VK_SHADER_STAGE_COMPUTE_BIT,
                     0u, sizeof(push), &push);
  vkCmdDispatch(command_buffer, prom_reduction_ceil_div_u32(push.output_element_count, 256u), 1u, 1u);
}

static void prom_m42_record_scale(prom_reduction_runtime_state* state,
                                  VkCommandBuffer command_buffer,
                                  VkDescriptorSet descriptor_set,
                                  uint32_t rows,
                                  uint32_t columns,
                                  uint32_t row_stride,
                                  float scale) {
  prom_m42_scale_push_constants push;
  memset(&push, 0, sizeof(push));
  push.rows = rows;
  push.columns = columns;
  push.row_stride = row_stride;
  push.total_elements = rows * columns;
  push.scale = scale;
  vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, state->m42_pipelines[2].pipeline);
  vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, state->pipeline_layout,
                          0u, 1u, &descriptor_set, 0u, NULL);
  vkCmdPushConstants(command_buffer, state->pipeline_layout, VK_SHADER_STAGE_COMPUTE_BIT,
                     0u, sizeof(push), &push);
  vkCmdDispatch(command_buffer, prom_reduction_ceil_div_u32(push.total_elements, 256u), 1u, 1u);
}

static int prom_m42_prepare_execution_buffers(prom_reduction_runtime_state* state,
                                              prom_reduction_slot* slot,
                                              const prom_m42_attention_request* request,
                                              const prom_m42_attention_plan* plan,
                                              const PrometheusReductionPlan* reduction_plan) {
  const uint32_t reduced = plan->selected_path != PROM_M42_PATH_A2X4;
  const uint32_t storage_tokens = reduced != 0u ? plan->padded_tokens : request->tokens;
  const uint32_t storage_model = reduced != 0u ? plan->padded_model_width : request->model_width;
  const uint32_t storage_head = reduced != 0u ? plan->padded_head_dim : request->head_dim;
  const uint64_t x_elements = (uint64_t)storage_tokens * storage_model;
  const uint64_t q_elements = (uint64_t)storage_tokens * storage_head;
  const uint64_t score_elements = (uint64_t)storage_tokens * storage_tokens;
  const uint64_t logical_q_elements = (uint64_t)request->tokens * request->head_dim;
  const uint64_t logical_score_elements = (uint64_t)request->tokens * request->tokens;
  const VkDeviceSize x_bytes = reduced != 0u
                                   ? (VkDeviceSize)(((x_elements + 1u) / 2u) * sizeof(uint32_t))
                                   : (VkDeviceSize)(x_elements * sizeof(float));
  const VkDeviceSize q_bytes = (VkDeviceSize)(q_elements * sizeof(float));
  const VkDeviceSize score_bytes = (VkDeviceSize)(score_elements * sizeof(float));
  const VkDeviceSize logical_score_bytes = (VkDeviceSize)(logical_score_elements * sizeof(float));
  const VkDeviceSize packed_q_bytes = (VkDeviceSize)(((q_elements + 1u) / 2u) * sizeof(uint32_t));
  const VkDeviceSize packed_score_bytes = (VkDeviceSize)(((score_elements + 1u) / 2u) * sizeof(uint32_t));
  const VkDeviceSize k_transpose_bytes = reduced != 0u
                                            ? packed_q_bytes
                                            : (VkDeviceSize)((uint64_t)request->head_dim * request->tokens * sizeof(float));
  const VkDeviceSize logical_output_bytes = (VkDeviceSize)(logical_q_elements * sizeof(float));
  const VkDeviceSize scratch_bytes = reduction_plan->partial_count > 1u
                                         ? (VkDeviceSize)((uint64_t)request->tokens *
                                                          reduction_plan->partial_count * sizeof(float))
                                         : PROM_REDUCTION_MIN_BINDING_BYTES;
  const VkDeviceSize row_bytes = reduction_plan->strategy == PROM_REDUCTION_STRATEGY_COMPOSED
                                     ? (VkDeviceSize)((uint64_t)request->tokens * sizeof(float))
                                     : PROM_REDUCTION_MIN_BINDING_BYTES;
  const VkDeviceSize audit_bytes = (VkDeviceSize)((3u * logical_q_elements + 2u * logical_score_elements) *
                                                   sizeof(float));
  if (request->input_mode == PROM_M42_INPUT_HOST_X &&
      (!prom_m42_ensure_buffer(state, &slot->m42_x_upload, x_bytes, VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                               VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                               1, NULL) ||
       !(reduced != 0u
             ? prom_m42_ensure_buffer(state, &slot->m42_x_packed, x_bytes,
                                      VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                      VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL)
             : prom_m42_ensure_buffer(state, &slot->m42_x, x_bytes,
                                      VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                      VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL)))) return 0;
  if (!prom_m42_ensure_buffer(state, &slot->m42_q, q_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
      !prom_m42_ensure_buffer(state, &slot->m42_k, q_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
      !prom_m42_ensure_buffer(state, &slot->m42_v, q_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
      !prom_m42_ensure_buffer(state, &slot->m42_k_transposed, k_transpose_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
      !prom_m42_ensure_buffer(state, &slot->m42_scores, score_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
      !prom_m42_ensure_buffer(state, &slot->m42_probabilities, logical_score_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
      !prom_m42_ensure_buffer(state, &slot->m42_output, q_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
      !prom_m42_ensure_buffer(state, &slot->m42_readback, logical_output_bytes,
                              VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                              VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                              1, NULL) ||
      !prom_m42_ensure_buffer(state, &slot->scratch, scratch_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
      !prom_m42_ensure_buffer(state, &slot->row_max, row_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
      !prom_m42_ensure_buffer(state, &slot->row_sum, row_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL)) return 0;
  if (reduced != 0u &&
      (!prom_m42_ensure_buffer(state, &slot->m42_q_packed, packed_q_bytes,
                               VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
       !prom_m42_ensure_buffer(state, &slot->m42_v_packed, packed_q_bytes,
                               VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
       !prom_m42_ensure_buffer(state, &slot->m42_p_packed, packed_score_bytes,
                               VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL))) return 0;
  if (request->audit_intermediates != 0u &&
      !prom_m42_ensure_buffer(state, &slot->m42_audit_readback, audit_bytes,
                              VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                              VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                              1, NULL)) return 0;
  return 1;
}

static void prom_m42_write_timestamp(const prom_reduction_runtime_state* state,
                                     const prom_reduction_slot* slot,
                                     VkCommandBuffer command_buffer,
                                     VkPipelineStageFlagBits stage,
                                     uint32_t query_offset) {
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
    vkCmdWriteTimestamp(command_buffer, stage, state->query_pool,
                        slot->slot_id * PROM_REDUCTION_QUERY_STRIDE + query_offset);
  }
}

static int prom_m42_record_attention(prom_reduction_runtime_state* state,
                                     prom_reduction_slot* slot,
                                     const prom_m42_attention_request* request,
                                     const prom_m42_attention_plan* plan,
                                     const PrometheusReductionPlan* reduction_plan,
                                     const prom_vk_buffer* x_buffer,
                                     VkDeviceSize x_copy_bytes,
                                     uint32_t* out_partial_fault) {
  const uint32_t reduced = plan->selected_path != PROM_M42_PATH_A2X4;
  const uint32_t storage_tokens = reduced != 0u ? plan->padded_tokens : request->tokens;
  const uint32_t storage_model = reduced != 0u ? plan->padded_model_width : request->model_width;
  const uint32_t storage_head = reduced != 0u ? plan->padded_head_dim : request->head_dim;
  VkCommandBuffer command_buffer = slot->command_buffer;
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  uint32_t stage_index;
  if (out_partial_fault != NULL) *out_partial_fault = 0u;
  if (vkResetCommandBuffer(command_buffer, 0u) != VK_SUCCESS) return 0;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(command_buffer, &begin_info) != VK_SUCCESS) return 0;
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
    vkCmdResetQueryPool(command_buffer, state->query_pool,
                        slot->slot_id * PROM_REDUCTION_QUERY_STRIDE, PROM_M42_QUERY_COUNT);
  }
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, 0u);
  if (request->input_mode == PROM_M42_INPUT_HOST_X) {
    prom_m42_buffer_barrier(command_buffer, &slot->m42_x_upload,
                            VK_ACCESS_HOST_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                            VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
    memset(&copy, 0, sizeof(copy)); copy.size = x_copy_bytes;
    vkCmdCopyBuffer(command_buffer, slot->m42_x_upload.buffer, x_buffer->buffer, 1u, &copy);
    prom_m42_buffer_barrier(command_buffer, x_buffer,
                            VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                            VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
  }
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, 1u);

  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 2u);
  prom_m42_record_sgemm(state, command_buffer, slot->m42_descriptor_sets[0], plan->selected_path,
                        storage_tokens, storage_head, storage_model);
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 3u);
  if (request->fault_point == PROM_M42_FAULT_AFTER_Q_PROJECTION) {
    if (out_partial_fault != NULL) *out_partial_fault = PROM_M42_FAULT_AFTER_Q_PROJECTION;
    return vkEndCommandBuffer(command_buffer) == VK_SUCCESS;
  }

  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 4u);
  prom_m42_record_sgemm(state, command_buffer, slot->m42_descriptor_sets[1], plan->selected_path,
                        storage_tokens, storage_head, storage_model);
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 5u);
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 6u);
  prom_m42_record_sgemm(state, command_buffer, slot->m42_descriptor_sets[2], plan->selected_path,
                        storage_tokens, storage_head, storage_model);
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 7u);
  prom_m42_buffer_barrier(command_buffer, &slot->m42_q, VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                          VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
  prom_m42_buffer_barrier(command_buffer, &slot->m42_k, VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                          VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
  prom_m42_buffer_barrier(command_buffer, &slot->m42_v, VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                          VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);

  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 8u);
  if (reduced != 0u) {
    prom_m42_record_pack(state, command_buffer, slot->m42_descriptor_sets[3],
                         request->tokens, request->head_dim, storage_head,
                         storage_tokens, storage_head, 0u);
    prom_m42_buffer_barrier(command_buffer, &slot->m42_q_packed,
                            VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
  }
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 9u);

  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 10u);
  if (reduced != 0u) {
    prom_m42_record_pack(state, command_buffer, slot->m42_descriptor_sets[4],
                         request->tokens, request->head_dim, storage_head,
                         storage_head, storage_tokens, 1u);
  } else {
    prom_m42_record_transpose(state, command_buffer, slot->m42_descriptor_sets[4],
                              request->tokens, request->head_dim, request->head_dim, request->tokens);
  }
  prom_m42_buffer_barrier(command_buffer, &slot->m42_k_transposed,
                          VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                          VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 11u);

  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 12u);
  if (reduced != 0u) {
    prom_m42_record_pack(state, command_buffer, slot->m42_descriptor_sets[5],
                         request->tokens, request->head_dim, storage_head,
                         storage_tokens, storage_head, 0u);
    prom_m42_buffer_barrier(command_buffer, &slot->m42_v_packed,
                            VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
  }
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 13u);

  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 14u);
  prom_m42_record_sgemm(state, command_buffer, slot->m42_descriptor_sets[6], plan->selected_path,
                        storage_tokens, storage_tokens, storage_head);
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 15u);
  if (request->fault_point == PROM_M42_FAULT_AFTER_QK) {
    if (out_partial_fault != NULL) *out_partial_fault = PROM_M42_FAULT_AFTER_QK;
    return vkEndCommandBuffer(command_buffer) == VK_SUCCESS;
  }
  prom_m42_buffer_barrier(command_buffer, &slot->m42_scores,
                          VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT,
                          VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);

  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 16u);
  prom_m42_record_scale(state, command_buffer, slot->m42_descriptor_sets[7],
                        request->tokens, request->tokens, storage_tokens, plan->scale);
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 17u);
  prom_m42_buffer_barrier(command_buffer, &slot->m42_scores,
                          VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                          VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);

  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 18u);
  for (stage_index = 0u; stage_index < reduction_plan->stage_count; ++stage_index) {
    const PrometheusReductionStageDispatch* stage = &reduction_plan->stages[stage_index];
    prom_reduction_buffer_bindings bindings;
    prom_reduction_push_constants push;
    VkPipeline pipeline = prom_reduction_pipeline_for_implementation(state, stage->implementation_id);
    if (pipeline == VK_NULL_HANDLE || 8u + stage_index >= PROM_M42_DESCRIPTOR_SET_COUNT) return 0;
    prom_reduction_stage_bindings_for_io(slot, reduction_plan, stage_index,
                                         &slot->m42_scores, &slot->m42_probabilities, &bindings);
    prom_reduction_update_descriptor_set(state, slot->m42_descriptor_sets[8u + stage_index], &bindings);
    state->m42_descriptor_update_count += 1u;
    memset(&push, 0, sizeof(push));
    push.row_count = request->tokens;
    push.elements_per_row = stage->input_elements_per_row;
    push.partials_per_row = stage->output_partials_per_row;
    push.input_row_stride = bindings.input == &slot->m42_scores ? storage_tokens : stage->input_elements_per_row;
    push.chunk_elements = PROM_REDUCTION_ELEMENTS_PER_PARTIAL;
    push.total_elements = request->tokens * request->tokens;
    push.stage_role = stage->stage_role;
    vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, pipeline);
    vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, state->pipeline_layout,
                            0u, 1u, &slot->m42_descriptor_sets[8u + stage_index], 0u, NULL);
    vkCmdPushConstants(command_buffer, state->pipeline_layout, VK_SHADER_STAGE_COMPUTE_BIT,
                       0u, sizeof(push), &push);
    vkCmdDispatch(command_buffer, stage->groups_x, stage->groups_y, stage->groups_z);
    if (stage_index + 1u < reduction_plan->stage_count) prom_reduction_record_barrier(command_buffer);
  }
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 19u);
  if (request->fault_point == PROM_M42_FAULT_AFTER_SOFTMAX) {
    if (out_partial_fault != NULL) *out_partial_fault = PROM_M42_FAULT_AFTER_SOFTMAX;
    return vkEndCommandBuffer(command_buffer) == VK_SUCCESS;
  }
  prom_m42_buffer_barrier(command_buffer, &slot->m42_probabilities,
                          VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                          VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);

  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 20u);
  if (reduced != 0u) {
    prom_m42_record_pack(state, command_buffer, slot->m42_descriptor_sets[13],
                         request->tokens, request->tokens, request->tokens,
                         storage_tokens, storage_tokens, 0u);
    prom_m42_buffer_barrier(command_buffer, &slot->m42_p_packed,
                            VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
  }
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 21u);

  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 22u);
  prom_m42_record_sgemm(state, command_buffer, slot->m42_descriptor_sets[14], plan->selected_path,
                        storage_tokens, storage_head, storage_tokens);
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 23u);
  prom_m42_buffer_barrier(command_buffer, &slot->m42_output,
                          VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                          VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, 24u);
  for (stage_index = 0u; stage_index < request->tokens; ++stage_index) {
    memset(&copy, 0, sizeof(copy));
    copy.srcOffset = (VkDeviceSize)((uint64_t)stage_index * storage_head * sizeof(float));
    copy.dstOffset = (VkDeviceSize)((uint64_t)stage_index * request->head_dim * sizeof(float));
    copy.size = (VkDeviceSize)((uint64_t)request->head_dim * sizeof(float));
    vkCmdCopyBuffer(command_buffer, slot->m42_output.buffer, slot->m42_readback.buffer, 1u, &copy);
  }
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, 25u);
  prom_m42_buffer_barrier(command_buffer, &slot->m42_readback,
                          VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_HOST_READ_BIT,
                          VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_HOST_BIT);
  return vkEndCommandBuffer(command_buffer) == VK_SUCCESS;
}

static int prom_m42_audit_readback(prom_reduction_runtime_state* state,
                                   prom_reduction_slot* slot,
                                   const prom_m42_attention_request* request,
                                   const prom_m42_attention_plan* plan,
                                   uint64_t* out_ns) {
  const uint32_t reduced = plan->selected_path != PROM_M42_PATH_A2X4;
  const uint32_t q_stride = reduced != 0u ? plan->padded_head_dim : request->head_dim;
  const uint32_t score_stride = reduced != 0u ? plan->padded_tokens : request->tokens;
  const uint64_t q_elements = (uint64_t)request->tokens * request->head_dim;
  const uint64_t score_elements = (uint64_t)request->tokens * request->tokens;
  const VkDeviceSize q_bytes = (VkDeviceSize)(q_elements * sizeof(float));
  const VkDeviceSize score_bytes = (VkDeviceSize)(score_elements * sizeof(float));
  const VkDeviceSize q_offset = 0u;
  const VkDeviceSize k_offset = q_bytes;
  const VkDeviceSize v_offset = q_bytes * 2u;
  const VkDeviceSize scores_offset = q_bytes * 3u;
  const VkDeviceSize probabilities_offset = scores_offset + score_bytes;
  VkCommandBuffer command_buffer = slot->consumer_command_buffer;
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  VkSubmitInfo submit;
  VkResult result;
  uint64_t begin_ns = prom_reduction_now_ns();
  uint32_t row;
  if (request->audit_q == NULL || request->audit_k == NULL || request->audit_v == NULL ||
      request->audit_scores == NULL || request->audit_probabilities == NULL) return 0;
  if (vkResetCommandBuffer(command_buffer, 0u) != VK_SUCCESS) return 0;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(command_buffer, &begin_info) != VK_SUCCESS) return 0;
  prom_m42_buffer_barrier(command_buffer, &slot->m42_q, VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                          VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
  prom_m42_buffer_barrier(command_buffer, &slot->m42_k, VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                          VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
  prom_m42_buffer_barrier(command_buffer, &slot->m42_v, VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                          VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
  prom_m42_buffer_barrier(command_buffer, &slot->m42_scores, VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                          VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
  prom_m42_buffer_barrier(command_buffer, &slot->m42_probabilities, VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                          VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
  for (row = 0u; row < request->tokens; ++row) {
    memset(&copy, 0, sizeof(copy));
    copy.srcOffset = (VkDeviceSize)((uint64_t)row * q_stride * sizeof(float));
    copy.size = (VkDeviceSize)((uint64_t)request->head_dim * sizeof(float));
    copy.dstOffset = q_offset + (VkDeviceSize)((uint64_t)row * request->head_dim * sizeof(float));
    vkCmdCopyBuffer(command_buffer, slot->m42_q.buffer, slot->m42_audit_readback.buffer, 1u, &copy);
    copy.dstOffset = k_offset + (VkDeviceSize)((uint64_t)row * request->head_dim * sizeof(float));
    vkCmdCopyBuffer(command_buffer, slot->m42_k.buffer, slot->m42_audit_readback.buffer, 1u, &copy);
    copy.dstOffset = v_offset + (VkDeviceSize)((uint64_t)row * request->head_dim * sizeof(float));
    vkCmdCopyBuffer(command_buffer, slot->m42_v.buffer, slot->m42_audit_readback.buffer, 1u, &copy);
    memset(&copy, 0, sizeof(copy));
    copy.srcOffset = (VkDeviceSize)((uint64_t)row * score_stride * sizeof(float));
    copy.dstOffset = scores_offset + (VkDeviceSize)((uint64_t)row * request->tokens * sizeof(float));
    copy.size = (VkDeviceSize)((uint64_t)request->tokens * sizeof(float));
    vkCmdCopyBuffer(command_buffer, slot->m42_scores.buffer, slot->m42_audit_readback.buffer, 1u, &copy);
  }
  memset(&copy, 0, sizeof(copy));
  copy.srcOffset = 0u; copy.dstOffset = probabilities_offset; copy.size = score_bytes;
  vkCmdCopyBuffer(command_buffer, slot->m42_probabilities.buffer, slot->m42_audit_readback.buffer, 1u, &copy);
  prom_m42_buffer_barrier(command_buffer, &slot->m42_audit_readback,
                          VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_HOST_READ_BIT,
                          VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_HOST_BIT);
  if (vkEndCommandBuffer(command_buffer) != VK_SUCCESS ||
      vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS) return 0;
  memset(&submit, 0, sizeof(submit)); submit.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit.commandBufferCount = 1u; submit.pCommandBuffers = &command_buffer;
  result = vkQueueSubmit(state->queue, 1u, &submit, slot->fence);
  if (result != VK_SUCCESS) return 0;
  slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
  result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
  if (result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    return 0;
  }
  memcpy(request->audit_q, (const unsigned char*)slot->m42_audit_readback.mapped + q_offset, (size_t)q_bytes);
  memcpy(request->audit_k, (const unsigned char*)slot->m42_audit_readback.mapped + k_offset, (size_t)q_bytes);
  memcpy(request->audit_v, (const unsigned char*)slot->m42_audit_readback.mapped + v_offset, (size_t)q_bytes);
  memcpy(request->audit_scores, (const unsigned char*)slot->m42_audit_readback.mapped + scores_offset, (size_t)score_bytes);
  memcpy(request->audit_probabilities,
         (const unsigned char*)slot->m42_audit_readback.mapped + probabilities_offset, (size_t)score_bytes);
  if (out_ns != NULL) *out_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  return 1;
}

static uint64_t prom_m42_retained_bytes(const prom_reduction_runtime_state* state,
                                        const prom_reduction_slot* slot) {
  return prom_m42_weight_retained_bytes(state) +
         (uint64_t)state->m42_resident_x_upload.size + (uint64_t)state->m42_resident_x_f32.size +
         (uint64_t)state->m42_resident_x_f16.size + (uint64_t)slot->m42_x_upload.size +
         (uint64_t)slot->m42_x.size + (uint64_t)slot->m42_x_packed.size +
         (uint64_t)slot->m42_q.size + (uint64_t)slot->m42_k.size + (uint64_t)slot->m42_v.size +
         (uint64_t)slot->m42_q_packed.size + (uint64_t)slot->m42_k_transposed.size +
         (uint64_t)slot->m42_v_packed.size + (uint64_t)slot->m42_scores.size +
         (uint64_t)slot->m42_probabilities.size + (uint64_t)slot->m42_p_packed.size +
         (uint64_t)slot->m42_output.size + (uint64_t)slot->m42_readback.size +
         (uint64_t)slot->m42_audit_readback.size + (uint64_t)slot->scratch.size +
         (uint64_t)slot->row_max.size + (uint64_t)slot->row_sum.size;
}

int prom_reactor_runtime_m42_execute(void* handle,
                                     const prom_m42_attention_request* request,
                                     prom_m42_attention_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_reduction_slot* slot = NULL;
  prom_vk_runtime_services services_before;
  prom_vk_runtime_services services_after;
  prom_m42_plan_request plan_request;
  PrometheusReductionRequest reduction_request;
  PrometheusReductionPlan reduction_plan;
  const prom_vk_buffer* x_buffer;
  const prom_vk_buffer* weight_buffers[3];
  void* packed_x = NULL;
  size_t packed_x_bytes = 0u;
  uint32_t reduced;
  uint32_t storage_tokens;
  uint32_t storage_model;
  uint32_t storage_head;
  uint32_t partial_fault = 0u;
  uint32_t query_index;
  uint64_t timestamps[PROM_M42_QUERY_COUNT];
  uint64_t begin_ns = prom_reduction_now_ns();
  uint64_t conversion_begin;
  uint64_t submit_begin;
  uint64_t readback_begin;
  uint64_t readback_cpu_ns;
  uint64_t logical_request_id;
  int32_t detail = 0;
  VkSubmitInfo submit;
  VkResult result;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  if (request == NULL || request->output == NULL ||
      request->fault_point > PROM_M42_FAULT_AFTER_PV_SUBMIT ||
      (request->input_mode == PROM_M42_INPUT_HOST_X && request->host_x == NULL) ||
      (request->audit_intermediates != 0u &&
       (request->audit_q == NULL || request->audit_k == NULL || request->audit_v == NULL ||
        request->audit_scores == NULL || request->audit_probabilities == NULL))) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M42_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL || prom_reactor_runtime_get_vk_services(handle, &services_before) != PROM_OK) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = state == NULL ? detail : PROM_M42_DETAIL_CAPABILITY;
    return PROM_ERROR;
  }
  if (state->m42_weight_model_width != request->model_width ||
      state->m42_weight_head_dim != request->head_dim ||
      state->m42_weight_generation[0] == 0u ||
      request->required_wq_generation != state->m42_weight_generation[0] ||
      request->required_wk_generation != state->m42_weight_generation[1] ||
      request->required_wv_generation != state->m42_weight_generation[2]) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M42_DETAIL_STALE_WEIGHT_GENERATION;
    return PROM_ERROR;
  }
  memset(&plan_request, 0, sizeof(plan_request));
  plan_request.tokens = request->tokens;
  plan_request.model_width = request->model_width;
  plan_request.head_dim = request->head_dim;
  plan_request.value_dim = request->value_dim;
  plan_request.scale = request->scale;
  plan_request.scale_explicit = request->scale_explicit;
  plan_request.precision_policy = request->precision_policy;
  plan_request.preferred_path = request->preferred_path;
  plan_request.allow_fallback = request->allow_fallback;
  plan_request.input_mode = request->input_mode;
  plan_request.cooperative_capability_state = services_before.cooperative_matrix_state;
  plan_request.rollback_active = request->rollback_active;
  plan_request.wq_generation = state->m42_weight_generation[0];
  plan_request.wk_generation = state->m42_weight_generation[1];
  plan_request.wv_generation = state->m42_weight_generation[2];
  plan_request.wq_hash = state->m42_weight_hash[0];
  plan_request.wk_hash = state->m42_weight_hash[1];
  plan_request.wv_hash = state->m42_weight_hash[2];
  if (prom_m42_attention_plan_build(&plan_request, &out_result->plan) != PROM_OK) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M42_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  out_result->selected_path = out_result->plan.selected_path;
  out_result->fallback_used = out_result->plan.fallback_used;
  out_result->selector_reason = out_result->plan.selector_reason;
  if (request->input_mode == PROM_M42_INPUT_RESIDENT_X &&
      (state->m42_resident_x_generation == 0u ||
       request->required_x_generation != state->m42_resident_x_generation ||
       state->m42_resident_x_tokens != request->tokens ||
       state->m42_resident_x_model_width != request->model_width)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M42_DETAIL_STALE_X_GENERATION;
    return PROM_ERROR;
  }
  if (out_result->selected_path == PROM_M42_PATH_COOPERATIVE &&
      (services_before.cooperative_matrix_feature_enabled == 0u || services_before.subgroup_size != 32u)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M42_DETAIL_CAPABILITY;
    return PROM_ERROR;
  }
  if (!prom_m42_ensure_pipelines(state) ||
      !prom_m40b_ensure_sgemm_pipeline(state, out_result->selected_path)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M42_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  memset(&reduction_request, 0, sizeof(reduction_request));
  reduction_request.struct_size = sizeof(reduction_request);
  reduction_request.row_count = request->tokens;
  reduction_request.elements_per_row = request->tokens;
  reduction_request.input_element_count = (uint64_t)request->tokens * request->tokens;
  reduction_request.output_element_count = reduction_request.input_element_count;
  reduction_request.operation = PROM_REDUCTION_OPERATION_SOFTMAX;
  reduction_request.finalization = PROM_REDUCTION_FINALIZATION_STABLE_SOFTMAX;
  if (prom_reactor_reduction_plan_impl(&reduction_request, &reduction_plan) != PROM_OK) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M42_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  logical_request_id = state->next_logical_request_id++;
  state->diagnostics.next_logical_request_id = state->next_logical_request_id;
  slot = prom_reduction_acquire_slot(state, logical_request_id);
  if (slot == NULL) {
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M42_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  out_result->logical_request_id = logical_request_id;
  out_result->physical_slot_id = slot->slot_id;
  out_result->physical_slot_generation = slot->generation;
  if (!prom_m42_prepare_execution_buffers(state, slot, request, &out_result->plan, &reduction_plan)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M42_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  reduced = out_result->selected_path != PROM_M42_PATH_A2X4;
  storage_tokens = reduced != 0u ? out_result->plan.padded_tokens : request->tokens;
  storage_model = reduced != 0u ? out_result->plan.padded_model_width : request->model_width;
  storage_head = reduced != 0u ? out_result->plan.padded_head_dim : request->head_dim;
  if (request->input_mode == PROM_M42_INPUT_HOST_X) {
    conversion_begin = prom_reduction_now_ns();
    if (!prom_m40b_pack_matrix(request->host_x, request->tokens, request->model_width,
                               storage_tokens, storage_model,
                               reduced != 0u ? PROM_M40B_KERNEL_COOPERATIVE : PROM_M40B_KERNEL_A2X4,
                               &packed_x, &packed_x_bytes) || packed_x_bytes > slot->m42_x_upload.size) {
      free(packed_x);
      slot->state = PROM_ASYNC_PHYSICAL_READY;
      out_result->physical_slot_recyclable = 1u;
      out_result->stage = PROM_STAGE_TRANSFER_IN;
      out_result->detail_code = PROM_M42_DETAIL_NONFINITE_INPUT;
      return PROM_ERROR;
    }
    memcpy(slot->m42_x_upload.mapped, packed_x, packed_x_bytes);
    free(packed_x);
    out_result->x_conversion_ns = prom_reduction_elapsed_ns(conversion_begin, prom_reduction_now_ns());
    x_buffer = reduced != 0u ? &slot->m42_x_packed : &slot->m42_x;
  } else {
    x_buffer = reduced != 0u ? &state->m42_resident_x_f16 : &state->m42_resident_x_f32;
    packed_x_bytes = 0u;
    out_result->x_generation = state->m42_resident_x_generation;
  }
  for (query_index = 0u; query_index < 3u; ++query_index) {
    weight_buffers[query_index] = reduced != 0u ? &state->m42_weight_f16[query_index]
                                                : &state->m42_weight_f32[query_index];
  }
  prom_m42_update_descriptor(state, slot->m42_descriptor_sets[0], x_buffer, weight_buffers[0], &slot->m42_q);
  prom_m42_update_descriptor(state, slot->m42_descriptor_sets[1], x_buffer, weight_buffers[1], &slot->m42_k);
  prom_m42_update_descriptor(state, slot->m42_descriptor_sets[2], x_buffer, weight_buffers[2], &slot->m42_v);
  if (reduced != 0u) {
    prom_m42_update_descriptor(state, slot->m42_descriptor_sets[3], &slot->m42_q, NULL, &slot->m42_q_packed);
    prom_m42_update_descriptor(state, slot->m42_descriptor_sets[4], &slot->m42_k, NULL, &slot->m42_k_transposed);
    prom_m42_update_descriptor(state, slot->m42_descriptor_sets[5], &slot->m42_v, NULL, &slot->m42_v_packed);
    prom_m42_update_descriptor(state, slot->m42_descriptor_sets[6], &slot->m42_q_packed,
                               &slot->m42_k_transposed, &slot->m42_scores);
    prom_m42_update_descriptor(state, slot->m42_descriptor_sets[13], &slot->m42_probabilities,
                               NULL, &slot->m42_p_packed);
    prom_m42_update_descriptor(state, slot->m42_descriptor_sets[14], &slot->m42_p_packed,
                               &slot->m42_v_packed, &slot->m42_output);
  } else {
    prom_m42_update_descriptor(state, slot->m42_descriptor_sets[4], &slot->m42_k, NULL,
                               &slot->m42_k_transposed);
    prom_m42_update_descriptor(state, slot->m42_descriptor_sets[6], &slot->m42_q,
                               &slot->m42_k_transposed, &slot->m42_scores);
    prom_m42_update_descriptor(state, slot->m42_descriptor_sets[14], &slot->m42_probabilities,
                               &slot->m42_v, &slot->m42_output);
  }
  prom_m42_update_descriptor(state, slot->m42_descriptor_sets[7], &slot->m42_scores,
                             NULL, &slot->m42_scores);
  memset(&out_result->output_view, 0, sizeof(out_result->output_view));
  out_result->output_view.buffer = slot->m42_output.buffer;
  out_result->output_view.byte_length = slot->m42_output.size;
  out_result->output_view.element_type = PROM_DEVICE_ELEMENT_F32;
  out_result->output_view.logical_rows = request->tokens;
  out_result->output_view.logical_columns = request->head_dim;
  out_result->output_view.row_stride_elements = storage_head;
  out_result->output_view.layout = PROM_DEVICE_LAYOUT_ROW_MAJOR;
  out_result->output_view.producer_access = PROM_DEVICE_ACCESS_COMPUTE_WRITE;
  out_result->output_view.required_consumer_access = PROM_DEVICE_ACCESS_HOST_READ;
  out_result->output_view.owning_device = state->device;
  out_result->output_view.owning_lifetime_id = logical_request_id;
  out_result->output_view.owning_slot_id = slot->slot_id;
  out_result->output_view.owning_slot_generation = slot->generation;
  if (!prom_m42_record_attention(state, slot, request, &out_result->plan, &reduction_plan,
                                 x_buffer, (VkDeviceSize)packed_x_bytes, &partial_fault)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M42_DETAIL_COMMAND;
    return PROM_ERROR;
  }
  slot->m42_command_reuse_count += 1u;
  if (vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M42_DETAIL_SUBMIT;
    return PROM_ERROR;
  }
  memset(&submit, 0, sizeof(submit)); submit.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit.commandBufferCount = 1u; submit.pCommandBuffers = &slot->command_buffer;
  submit_begin = prom_reduction_now_ns();
  result = vkQueueSubmit(state->queue, 1u, &submit, slot->fence);
  out_result->cpu_submission_ns = prom_reduction_elapsed_ns(submit_begin, prom_reduction_now_ns());
  if (result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M42_DETAIL_SUBMIT;
    return PROM_ERROR;
  }
  slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
  if (request->fault_point == PROM_M42_FAULT_AFTER_PV_SUBMIT) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    out_result->stage = PROM_M42_STAGE_PV;
    out_result->detail_code = PROM_M42_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
  if (result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M42_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  if (partial_fault != 0u) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = partial_fault == PROM_M42_FAULT_AFTER_Q_PROJECTION
                            ? PROM_M42_STAGE_PROJECT_Q
                            : (partial_fault == PROM_M42_FAULT_AFTER_QK
                                   ? PROM_M42_STAGE_QK_TRANSPOSE
                                   : PROM_M42_STAGE_SOFTMAX);
    out_result->detail_code = PROM_M42_DETAIL_FAULT_INJECTED;
    return PROM_ERROR;
  }
  memset(timestamps, 0, sizeof(timestamps));
  if (state->timestamp_supported == 0u || state->query_pool == VK_NULL_HANDLE ||
      vkGetQueryPoolResults(state->device, state->query_pool,
                            slot->slot_id * PROM_REDUCTION_QUERY_STRIDE, PROM_M42_QUERY_COUNT,
                            sizeof(timestamps), timestamps, sizeof(uint64_t),
                            VK_QUERY_RESULT_64_BIT) != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_TRANSFER_OUT;
    out_result->detail_code = PROM_M42_DETAIL_QUERY;
    return PROM_ERROR;
  }
  for (query_index = 1u; query_index < PROM_M42_QUERY_COUNT; ++query_index) {
    if (timestamps[query_index] < timestamps[query_index - 1u]) {
      slot->state = PROM_ASYNC_PHYSICAL_READY;
      out_result->physical_slot_recyclable = 1u;
      out_result->stage = PROM_STAGE_TRANSFER_OUT;
      out_result->detail_code = PROM_M42_DETAIL_QUERY;
      return PROM_ERROR;
    }
  }
#define PROM_M42_DURATION(BEGIN, END) \
  ((uint64_t)((double)(timestamps[(END)] - timestamps[(BEGIN)]) * state->timestamp_period_ns))
  out_result->x_upload_ns = PROM_M42_DURATION(0u, 1u);
  out_result->q_projection_gpu_ns = PROM_M42_DURATION(2u, 3u);
  out_result->k_projection_gpu_ns = PROM_M42_DURATION(4u, 5u);
  out_result->v_projection_gpu_ns = PROM_M42_DURATION(6u, 7u);
  out_result->q_pack_gpu_ns = PROM_M42_DURATION(8u, 9u);
  out_result->k_layout_gpu_ns = PROM_M42_DURATION(10u, 11u);
  out_result->v_pack_gpu_ns = PROM_M42_DURATION(12u, 13u);
  out_result->qk_gpu_ns = PROM_M42_DURATION(14u, 15u);
  out_result->scale_gpu_ns = PROM_M42_DURATION(16u, 17u);
  out_result->softmax_gpu_ns = PROM_M42_DURATION(18u, 19u);
  out_result->p_pack_gpu_ns = PROM_M42_DURATION(20u, 21u);
  out_result->pv_gpu_ns = PROM_M42_DURATION(22u, 23u);
  out_result->total_attention_gpu_ns = PROM_M42_DURATION(2u, 23u);
#undef PROM_M42_DURATION
  readback_begin = prom_reduction_now_ns();
  memcpy(request->output, slot->m42_readback.mapped,
         (size_t)((uint64_t)request->tokens * request->head_dim * sizeof(float)));
  readback_cpu_ns = prom_reduction_elapsed_ns(readback_begin, prom_reduction_now_ns());
  out_result->final_readback_ns =
      (uint64_t)((double)(timestamps[25] - timestamps[24]) * state->timestamp_period_ns) + readback_cpu_ns;
  out_result->final_readback_count = 1u;
  out_result->submit_count = 1u;
  out_result->no_intermediate_host_copy = 1u;
  if (request->audit_intermediates != 0u) {
    if (!prom_m42_audit_readback(state, slot, request, &out_result->plan, &out_result->audit_readback_ns)) {
      if (slot->state != PROM_ASYNC_PHYSICAL_QUARANTINED) slot->state = PROM_ASYNC_PHYSICAL_READY;
      out_result->physical_slot_recyclable = slot->state == PROM_ASYNC_PHYSICAL_READY;
      out_result->stage = PROM_STAGE_TRANSFER_OUT;
      out_result->detail_code = slot->state == PROM_ASYNC_PHYSICAL_QUARANTINED
                                    ? PROM_M42_DETAIL_COMPLETION_UNCERTAIN
                                    : PROM_M42_DETAIL_READBACK;
      return PROM_ERROR;
    }
    out_result->audit_readback_count = 5u;
    out_result->submit_count = 2u;
  }
  out_result->qkv_gpu_producer_dispatch_count = 3u;
  out_result->retained_bytes = prom_m42_retained_bytes(state, slot);
  out_result->buffer_allocation_count = state->m42_buffer_grow_count;
  out_result->buffer_reuse_count = state->m42_buffer_reuse_count;
  out_result->descriptor_update_count = state->m42_descriptor_update_count;
  out_result->pipeline_create_count = state->m42_pipeline_create_count + state->m40b_pipeline_create_count +
                                      state->diagnostics.pipeline_create_count;
  out_result->command_buffer_reuse_count = slot->m42_command_reuse_count;
  out_result->wq_generation = state->m42_weight_generation[0];
  out_result->wk_generation = state->m42_weight_generation[1];
  out_result->wv_generation = state->m42_weight_generation[2];
  out_result->validation_error_count_before = services_before.validation_error_count;
  if (out_result->selected_path == PROM_M42_PATH_COOPERATIVE) {
    (void)prom_reactor_runtime_mark_cooperative_matrix_executable(handle);
  }
  if (prom_reactor_runtime_get_vk_services(handle, &services_after) == PROM_OK) {
    out_result->validation_error_count_after = services_after.validation_error_count;
  }
  out_result->end_to_end_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->physical_slot_recyclable = 1u;
  out_result->stage = 0u;
  out_result->detail_code = 0;
  return PROM_OK;
}

static void prom_reduction_result_failure(PrometheusReductionExecutionResult* result,
                                          uint32_t stage,
                                          int32_t detail) {
  result->stage = stage;
  result->detail_code = detail;
}

static int prom_reduction_find_nonfinite(const float* input, uint64_t count, uint64_t* out_index) {
  uint64_t index;
  for (index = 0u; index < count; ++index) {
    if (!isfinite(input[index])) {
      if (out_index != NULL) *out_index = index;
      return 1;
    }
  }
  return 0;
}

int prom_reactor_runtime_reduction_impl(void* handle,
                                        const PrometheusReductionRequest* request,
                                        PrometheusReductionExecutionResult* out_result) {
  PrometheusReductionExecutionResult local_result;
  PrometheusReductionExecutionResult* result = out_result != NULL ? out_result : &local_result;
  PrometheusReductionPlan plan;
  prom_reduction_runtime_state* state;
  prom_reduction_slot* slot;
  prom_vk_runtime_services services_before;
  prom_vk_runtime_services services_after;
  uint64_t total_elements = 0u;
  uint64_t output_elements = 0u;
  uint64_t begin_ns = prom_reduction_now_ns();
  uint64_t end_ns;
  uint64_t timestamps[2];
  uint64_t available_temporary;
  uint32_t malformed_injected = 0u;
  int32_t detail = 0;
  VkSubmitInfo submit_info;
  VkResult vk_result;
  memset(result, 0, sizeof(*result));
  result->struct_size = sizeof(*result);
  result->physical_slot_id = UINT32_MAX;
  result->first_nonfinite_index = UINT64_MAX;
  if (!prom_reactor_runtime_validate_handle(handle)) {
    prom_reduction_result_failure(result, PROM_STAGE_INIT, PROM_REDUCTION_DETAIL_RUNTIME_UNAVAILABLE);
    return PROM_INVALID_HANDLE;
  }
  if (!prom_reduction_validate_request(request, 1u, &total_elements, &output_elements, &detail)) {
    prom_reduction_result_failure(result, PROM_STAGE_INIT, detail);
    return PROM_ERROR;
  }
  if (prom_reduction_find_nonfinite(request->input, total_elements, &result->first_nonfinite_index)) {
    prom_reduction_result_failure(result, PROM_STAGE_TRANSFER_IN, PROM_REDUCTION_DETAIL_NONFINITE_INPUT);
    return PROM_ERROR;
  }
  if (prom_reactor_reduction_plan_impl(request, &plan) != PROM_OK) {
    prom_reduction_result_failure(result, PROM_STAGE_INIT, PROM_REDUCTION_DETAIL_MALFORMED_PLAN);
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL) {
    prom_reduction_result_failure(result, PROM_STAGE_INIT, detail);
    return PROM_ERROR;
  }
  state->diagnostics.total_requests += 1u;
  result->logical_request_id = state->next_logical_request_id++;
  state->diagnostics.next_logical_request_id = state->next_logical_request_id;
  result->plan = plan;
  if (prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_MALFORMED_STAGE_METADATA)) {
    result->plan.stages[0].shader_id = 0u;
    malformed_injected = 1u;
  }
  available_temporary = result->plan.temporary_bytes;
  if (malformed_injected == 0u &&
      prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_TEMPORARY_UNDERSIZED) &&
      available_temporary > 0u) {
    available_temporary -= 1u;
  }
  if (prom_reduction_validate_plan_for_test(&result->plan, available_temporary, &detail) != PROM_OK) {
    prom_reduction_result_failure(result, PROM_STAGE_INIT, detail);
    state->diagnostics.logical_failure_count += 1u;
    state->diagnostics.last_detail_code = detail;
    return PROM_ERROR;
  }
  if (prom_reactor_runtime_get_vk_services(handle, &services_before) == PROM_OK) {
    result->validation_error_count_before = services_before.validation_error_count;
  }
  slot = prom_reduction_acquire_slot(state, result->logical_request_id);
  if (slot == NULL) {
    prom_reduction_result_failure(result, PROM_STAGE_SUBMIT, PROM_REDUCTION_DETAIL_COMPLETION_UNCERTAIN);
    state->diagnostics.logical_failure_count += 1u;
    return PROM_ERROR;
  }
  result->physical_slot_id = slot->slot_id;
  result->physical_slot_generation = slot->generation;
  if (!prom_reduction_prepare_slot_buffers(state, slot, request, &plan, total_elements, output_elements)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    result->physical_slot_recyclable = 1u;
    prom_reduction_result_failure(result, PROM_STAGE_TRANSFER_IN, PROM_REDUCTION_DETAIL_RESOURCE_CREATE_FAILED);
    state->diagnostics.logical_failure_count += 1u;
    return PROM_ERROR;
  }
  memcpy(slot->input.mapped, request->input, (size_t)(total_elements * sizeof(float)));
  memset(slot->output.mapped, 0, (size_t)(output_elements * sizeof(float)));
  if (!prom_reduction_record_request(state, slot, request, &plan, (uint32_t)total_elements)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    result->physical_slot_recyclable = 1u;
    prom_reduction_result_failure(result, PROM_STAGE_SUBMIT, PROM_REDUCTION_DETAIL_COMMAND_RECORD_FAILED);
    state->diagnostics.logical_failure_count += 1u;
    return PROM_ERROR;
  }
  vk_result = vkResetFences(state->device, 1u, &slot->fence);
  if (vk_result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    result->physical_slot_recyclable = 1u;
    prom_reduction_result_failure(result, PROM_STAGE_SUBMIT, PROM_REDUCTION_DETAIL_QUEUE_SUBMIT_FAILED);
    state->diagnostics.logical_failure_count += 1u;
    return PROM_ERROR;
  }
  if (prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_QUEUE_SUBMIT)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    result->physical_slot_recyclable = 1u;
    prom_reduction_result_failure(result, PROM_STAGE_SUBMIT, PROM_REDUCTION_DETAIL_QUEUE_SUBMIT_FAILED);
    state->diagnostics.logical_failure_count += 1u;
    return PROM_ERROR;
  }
  memset(&submit_info, 0, sizeof(submit_info));
  submit_info.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit_info.commandBufferCount = 1u;
  submit_info.pCommandBuffers = &slot->command_buffer;
  vk_result = vkQueueSubmit(state->queue, 1u, &submit_info, slot->fence);
  if (vk_result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    result->physical_slot_recyclable = 1u;
    prom_reduction_result_failure(result, PROM_STAGE_SUBMIT, PROM_REDUCTION_DETAIL_QUEUE_SUBMIT_FAILED);
    state->diagnostics.logical_failure_count += 1u;
    return PROM_ERROR;
  }
  slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
  state->diagnostics.queue_submit_count += 1u;
  if (prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_COMPLETION_OBSERVATION)) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    state->diagnostics.logical_failure_count += 1u;
    result->physical_slot_recyclable = 0u;
    prom_reduction_result_failure(result, PROM_STAGE_SUBMIT, PROM_REDUCTION_DETAIL_COMPLETION_UNCERTAIN);
    return PROM_ERROR;
  }
  vk_result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
  if (vk_result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    state->diagnostics.logical_failure_count += 1u;
    prom_reduction_result_failure(result, PROM_STAGE_SUBMIT, PROM_REDUCTION_DETAIL_COMPLETION_UNCERTAIN);
    return PROM_ERROR;
  }
  slot->state = PROM_ASYNC_PHYSICAL_COMPLETE;
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
    memset(timestamps, 0, sizeof(timestamps));
    vk_result = vkGetQueryPoolResults(state->device, state->query_pool,
                                      slot->slot_id * PROM_REDUCTION_QUERY_STRIDE, 2u,
                                      sizeof(timestamps), timestamps, sizeof(uint64_t), VK_QUERY_RESULT_64_BIT);
    if (vk_result != VK_SUCCESS || timestamps[1] <= timestamps[0]) {
      slot->state = PROM_ASYNC_PHYSICAL_READY;
      result->physical_slot_recyclable = 1u;
      state->diagnostics.logical_failure_count += 1u;
      prom_reduction_result_failure(result, PROM_STAGE_TRANSFER_OUT, PROM_REDUCTION_DETAIL_QUERY_FAILED);
      return PROM_ERROR;
    }
    result->gpu_timestamp_valid = 1u;
    result->gpu_duration_ns = (uint64_t)((double)(timestamps[1] - timestamps[0]) * state->timestamp_period_ns);
  }
  memcpy(request->output, slot->output.mapped, (size_t)(output_elements * sizeof(float)));
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  result->physical_slot_recyclable = 1u;
  result->stage = PROM_STAGE_NONE;
  result->detail_code = 0;
  end_ns = prom_reduction_now_ns();
  result->end_to_end_ns = prom_reduction_elapsed_ns(begin_ns, end_ns);
  if (prom_reactor_runtime_get_vk_services(handle, &services_after) == PROM_OK) {
    result->validation_error_count_after = services_after.validation_error_count;
    state->diagnostics.validation_enabled = services_after.validation_enabled;
    state->diagnostics.validation_error_count = services_after.validation_error_count;
  }
  state->diagnostics.successful_requests += 1u;
  state->diagnostics.last_replay_id = plan.replay_id;
  state->diagnostics.last_gpu_duration_ns = result->gpu_duration_ns;
  state->diagnostics.last_end_to_end_ns = result->end_to_end_ns;
  state->diagnostics.last_stage_count = plan.stage_count;
  state->diagnostics.last_physical_slot_id = slot->slot_id;
  state->diagnostics.last_physical_slot_generation = slot->generation;
  state->diagnostics.last_detail_code = 0;
  return PROM_OK;
}

int prom_reactor_runtime_reduction_diagnostics_impl(void* handle,
                                                    PrometheusReductionDiagnostics* out_diag) {
  prom_reduction_runtime_state* state;
  prom_vk_runtime_services services;
  uint32_t slot_index;
  if (out_diag == NULL) return PROM_ERROR;
  memset(out_diag, 0, sizeof(*out_diag));
  out_diag->struct_size = sizeof(*out_diag);
  if (!prom_reactor_runtime_validate_handle(handle)) return PROM_INVALID_HANDLE;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL) {
    out_diag->configured_ring_depth = 2u;
    out_diag->physical_slot_count = out_diag->configured_ring_depth;
    if (prom_reactor_runtime_get_vk_services(handle, &services) == PROM_OK) {
      out_diag->configured_ring_depth = services.reduction_ring_depth;
      out_diag->physical_slot_count = services.reduction_ring_depth;
      out_diag->validation_enabled = services.validation_enabled;
      out_diag->validation_error_count = services.validation_error_count;
    }
    return PROM_OK;
  }
  *out_diag = state->diagnostics;
  out_diag->struct_size = sizeof(*out_diag);
  out_diag->outstanding_slots = 0u;
  out_diag->quarantined_slots = 0u;
  for (slot_index = 0u; slot_index < state->ring_depth; ++slot_index) {
    if (state->slots[slot_index].state == PROM_ASYNC_PHYSICAL_SUBMITTED) out_diag->outstanding_slots += 1u;
    if (state->slots[slot_index].state == PROM_ASYNC_PHYSICAL_QUARANTINED) out_diag->quarantined_slots += 1u;
  }
  if (prom_reactor_runtime_get_vk_services(handle, &services) == PROM_OK) {
    out_diag->validation_enabled = services.validation_enabled;
    out_diag->validation_error_count = services.validation_error_count;
  }
  return PROM_OK;
}

int prom_reduction_cpu_reference(const PrometheusReductionRequest* request,
                                 float* output,
                                 int32_t* out_detail) {
  uint64_t total_elements;
  uint64_t output_elements;
  uint32_t row;
  int32_t detail;
  if (!prom_reduction_validate_request(request, 1u, &total_elements, &output_elements, &detail) || output == NULL) {
    if (out_detail != NULL) *out_detail = output == NULL ? PROM_REDUCTION_DETAIL_NULL_OUTPUT : detail;
    return PROM_ERROR;
  }
  if (prom_reduction_find_nonfinite(request->input, total_elements, NULL)) {
    if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_NONFINITE_INPUT;
    return PROM_ERROR;
  }
  for (row = 0u; row < request->row_count; ++row) {
    const float* values = request->input + (uint64_t)row * request->elements_per_row;
    uint32_t column;
    if (request->operation == PROM_REDUCTION_OPERATION_SUM) {
      double accumulator = 0.0;
      for (column = 0u; column < request->elements_per_row; ++column) accumulator += (double)values[column];
      output[row] = (float)accumulator;
    } else if (request->operation == PROM_REDUCTION_OPERATION_MAX) {
      float maximum = values[0];
      for (column = 1u; column < request->elements_per_row; ++column) if (values[column] > maximum) maximum = values[column];
      output[row] = maximum;
    } else {
      float maximum = values[0];
      double denominator = 0.0;
      for (column = 1u; column < request->elements_per_row; ++column) if (values[column] > maximum) maximum = values[column];
      for (column = 0u; column < request->elements_per_row; ++column) denominator += exp((double)values[column] - maximum);
      for (column = 0u; column < request->elements_per_row; ++column) {
        output[(uint64_t)row * request->elements_per_row + column] =
            (float)(exp((double)values[column] - maximum) / denominator);
      }
    }
  }
  (void)output_elements;
  if (out_detail != NULL) *out_detail = 0;
  return PROM_OK;
}

static float prom_reduction_relative_error(float expected, float actual) {
  float denominator = fabsf(expected);
  if (denominator < 1.0e-20f) denominator = 1.0f;
  return fabsf(actual - expected) / denominator;
}

int prom_reduction_compare(const PrometheusReductionRequest* request,
                           const float* expected,
                           const float* actual,
                           PrometheusReductionBenchmarkResult* out_result) {
  uint64_t output_count = request->operation == PROM_REDUCTION_OPERATION_SOFTMAX
                              ? request->input_element_count
                              : request->row_count;
  uint64_t index;
  float absolute_tolerance = request->operation == PROM_REDUCTION_OPERATION_SOFTMAX
                                 ? 2.0e-5f
                                 : (request->operation == PROM_REDUCTION_OPERATION_MAX
                                        ? 0.0f
                                        : 2.0e-5f * request->elements_per_row);
  float relative_tolerance = request->operation == PROM_REDUCTION_OPERATION_SOFTMAX
                                 ? 2.0e-4f
                                 : (request->operation == PROM_REDUCTION_OPERATION_MAX ? 0.0f : 2.0e-5f);
  for (index = 0u; index < output_count; ++index) {
    float absolute_error = fabsf(actual[index] - expected[index]);
    float relative_error = prom_reduction_relative_error(expected[index], actual[index]);
    if (!isfinite(actual[index]) ||
        (absolute_error > absolute_tolerance && relative_error > relative_tolerance)) {
      if (out_result != NULL) {
        out_result->first_mismatch_row = request->operation == PROM_REDUCTION_OPERATION_SOFTMAX
                                             ? (uint32_t)(index / request->elements_per_row)
                                             : (uint32_t)index;
        out_result->first_mismatch_column = request->operation == PROM_REDUCTION_OPERATION_SOFTMAX
                                                ? (uint32_t)(index % request->elements_per_row)
                                                : UINT32_MAX;
        out_result->first_expected = expected[index];
        out_result->first_actual = actual[index];
        out_result->first_absolute_error = absolute_error;
        out_result->first_relative_error = relative_error;
      }
      return PROM_ERROR;
    }
  }
  if (request->operation == PROM_REDUCTION_OPERATION_SOFTMAX) {
    uint32_t row;
    for (row = 0u; row < request->row_count; ++row) {
      double row_sum = 0.0;
      uint32_t column;
      for (column = 0u; column < request->elements_per_row; ++column) {
        float value = actual[(uint64_t)row * request->elements_per_row + column];
        if (!isfinite(value) || value < -2.0e-7f) {
          if (out_result != NULL) {
            out_result->first_mismatch_row = row;
            out_result->first_mismatch_column = column;
            out_result->first_expected = 0.0f;
            out_result->first_actual = value;
            out_result->first_absolute_error = fabsf(value);
            out_result->first_relative_error = fabsf(value);
          }
          return PROM_ERROR;
        }
        row_sum += value;
      }
      if (fabs(row_sum - 1.0) > 3.0e-4) {
        if (out_result != NULL) {
          out_result->first_mismatch_row = row;
          out_result->first_mismatch_column = UINT32_MAX;
          out_result->first_expected = 1.0f;
          out_result->first_actual = (float)row_sum;
          out_result->first_absolute_error = (float)fabs(row_sum - 1.0);
          out_result->first_relative_error = out_result->first_absolute_error;
        }
        return PROM_ERROR;
      }
    }
  }
  return PROM_OK;
}

static int prom_reduction_compare_u64(const void* left, const void* right) {
  uint64_t a = *(const uint64_t*)left;
  uint64_t b = *(const uint64_t*)right;
  return a < b ? -1 : (a > b ? 1 : 0);
}

int prom_reactor_runtime_reduction_benchmark_impl(void* handle,
                                                  const PrometheusReductionBenchmarkRequest* request,
                                                  PrometheusReductionBenchmarkResult* out_result) {
  PrometheusReductionRequest execution_request;
  PrometheusReductionExecutionResult execution_result;
  PrometheusReductionDiagnostics diagnostics_before;
  PrometheusReductionDiagnostics diagnostics_after;
  uint64_t* gpu_samples;
  uint64_t* end_to_end_samples;
  float* expected;
  uint32_t iteration;
  uint64_t output_count;
  int32_t detail;
  if (out_result == NULL) return PROM_ERROR;
  memset(&execution_result, 0, sizeof(execution_result));
  memset(out_result, 0, sizeof(*out_result));
  out_result->struct_size = sizeof(*out_result);
  out_result->first_mismatch_row = UINT32_MAX;
  out_result->first_mismatch_column = UINT32_MAX;
  if (request == NULL || request->struct_size < sizeof(PrometheusReductionBenchmarkRequest) ||
      request->measured_iterations == 0u || request->measured_iterations > 10000u) {
    out_result->detail_code = PROM_REDUCTION_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  execution_request = request->reduction;
  output_count = execution_request.operation == PROM_REDUCTION_OPERATION_SOFTMAX
                     ? execution_request.input_element_count
                     : execution_request.row_count;
  expected = (float*)malloc((size_t)(output_count * sizeof(float)));
  gpu_samples = (uint64_t*)malloc((size_t)request->measured_iterations * sizeof(uint64_t));
  end_to_end_samples = (uint64_t*)malloc((size_t)request->measured_iterations * sizeof(uint64_t));
  if (expected == NULL || gpu_samples == NULL || end_to_end_samples == NULL) {
    free(end_to_end_samples); free(gpu_samples); free(expected);
    out_result->detail_code = PROM_REDUCTION_DETAIL_RESOURCE_CREATE_FAILED;
    return PROM_ERROR;
  }
  if (prom_reduction_cpu_reference(&execution_request, expected, &detail) != PROM_OK) {
    free(end_to_end_samples); free(gpu_samples); free(expected);
    out_result->detail_code = detail;
    return PROM_ERROR;
  }
  (void)prom_reactor_runtime_reduction_diagnostics_impl(handle, &diagnostics_before);
  for (iteration = 0u; iteration < request->warmup_iterations; ++iteration) {
    if (prom_reactor_runtime_reduction_impl(handle, &execution_request, &execution_result) != PROM_OK) {
      out_result->detail_code = execution_result.detail_code;
      free(end_to_end_samples); free(gpu_samples); free(expected);
      return PROM_ERROR;
    }
  }
  for (iteration = 0u; iteration < request->measured_iterations; ++iteration) {
    if (prom_reactor_runtime_reduction_impl(handle, &execution_request, &execution_result) != PROM_OK) {
      out_result->detail_code = execution_result.detail_code;
      free(end_to_end_samples); free(gpu_samples); free(expected);
      return PROM_ERROR;
    }
    gpu_samples[iteration] = execution_result.gpu_duration_ns;
    end_to_end_samples[iteration] = execution_result.end_to_end_ns;
    out_result->completed_iterations += 1u;
  }
  if (prom_reduction_compare(&execution_request, expected, execution_request.output, out_result) != PROM_OK) {
    out_result->detail_code = PROM_REDUCTION_DETAIL_READBACK_FAILED;
    free(end_to_end_samples); free(gpu_samples); free(expected);
    return PROM_ERROR;
  }
  out_result->correctness_passed = 1u;
  qsort(gpu_samples, request->measured_iterations, sizeof(uint64_t), prom_reduction_compare_u64);
  qsort(end_to_end_samples, request->measured_iterations, sizeof(uint64_t), prom_reduction_compare_u64);
  out_result->gpu_min_ns = gpu_samples[0];
  out_result->gpu_median_ns = gpu_samples[request->measured_iterations / 2u];
  out_result->gpu_max_ns = gpu_samples[request->measured_iterations - 1u];
  out_result->end_to_end_min_ns = end_to_end_samples[0];
  out_result->end_to_end_median_ns = end_to_end_samples[request->measured_iterations / 2u];
  out_result->end_to_end_max_ns = end_to_end_samples[request->measured_iterations - 1u];
  out_result->stage_count = execution_result.plan.stage_count;
  out_result->temporary_bytes = execution_result.plan.temporary_bytes;
  out_result->replay_id = execution_result.plan.replay_id;
  (void)prom_reactor_runtime_reduction_diagnostics_impl(handle, &diagnostics_after);
  out_result->validation_passed = diagnostics_before.validation_error_count == 0u &&
                                          diagnostics_after.validation_error_count == 0u
                                      ? 1u
                                      : 0u;
  out_result->detail_code = 0;
  free(end_to_end_samples); free(gpu_samples); free(expected);
  return PROM_OK;
}
