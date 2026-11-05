#if !defined(_WIN32) && !defined(_POSIX_C_SOURCE)
#define _POSIX_C_SOURCE 200809L
#endif

#include "reactor_vulkan.h"
#include "reactor_shader_registry.h"
#include "reactor_concept_rmsnorm.h"

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

#include "reactor_vulkan_runtime_internal.h"

static int prom_m43_checked_add_u64(uint64_t left, uint64_t right, uint64_t* out_value) {
  if (out_value == NULL || left > UINT64_MAX - right) return 0;
  *out_value = left + right;
  return 1;
}

static int prom_m45_view_required_bytes(const prom_device_buffer_view* view,
                                        uint64_t* out_bytes) {
  uint64_t elements;
  if (view == NULL || out_bytes == NULL || view->logical_rows == 0u ||
      view->logical_columns == 0u || view->row_stride_elements == 0u ||
      !prom_m40b_checked_product_u64(view->logical_rows, view->row_stride_elements, &elements) ||
      elements > UINT64_MAX / sizeof(float)) return 0;
  *out_bytes = elements * sizeof(float);
  return 1;
}

static void prom_m46_add_barrier(prom_rmsnorm_plan* plan,
                                 uint32_t buffer_identity,
                                 uint64_t byte_offset,
                                 uint64_t byte_length,
                                 uint32_t source_stage,
                                 uint32_t destination_stage,
                                 uint32_t source_access,
                                 uint32_t destination_access) {
  prom_m46_barrier_trace* barrier = &plan->barriers[plan->barrier_count];
  memset(barrier, 0, sizeof(*barrier));
  barrier->sequence = plan->barrier_count;
  barrier->buffer_identity = buffer_identity;
  barrier->byte_offset = byte_offset;
  barrier->byte_length = byte_length;
  barrier->source_stage_mask = source_stage;
  barrier->destination_stage_mask = destination_stage;
  barrier->source_access_mask = source_access;
  barrier->destination_access_mask = destination_access;
  barrier->source_queue_family = VK_QUEUE_FAMILY_IGNORED;
  barrier->destination_queue_family = VK_QUEUE_FAMILY_IGNORED;
  plan->barrier_count += 1u;
}

static void prom_m46_add_stage(prom_rmsnorm_plan* plan,
                               uint32_t operation,
                               uint32_t dispatch_count,
                               uint32_t barrier_begin,
                               uint32_t barrier_count,
                               uint32_t copy_regions,
                               uint32_t timestamp_begin,
                               uint32_t timestamp_end) {
  prom_m46_stage_plan* stage = &plan->stages[plan->stage_count];
  memset(stage, 0, sizeof(*stage));
  stage->sequence = plan->stage_count;
  stage->operation = operation;
  stage->dispatch_count = dispatch_count;
  stage->barrier_begin = barrier_begin;
  stage->barrier_count = barrier_count;
  stage->copy_region_count = copy_regions;
  stage->timestamp_begin = timestamp_begin;
  stage->timestamp_end = timestamp_end;
  plan->stage_count += 1u;
  plan->dispatch_count += dispatch_count;
  plan->copy_region_count += copy_regions;
}

int prom_m46_rmsnorm_plan_build(const prom_m46_plan_request* request,
                                prom_rmsnorm_plan* out_plan) {
  uint64_t z_bytes = 0u;
  uint64_t logical_elements = 0u;
  uint64_t compact_bytes = 0u;
  uint64_t partial_elements = 0u;
  uint64_t total = 0u;
  uint64_t eligibility_hash = 1469598103934665603ull;
  uint64_t command_hash = 1469598103934665603ull;
  uint64_t replay_hash = 1469598103934665603ull;
  uint32_t epsilon_bits = 0u;
  uint32_t reason = PROM_M46_ELIGIBLE;
  uint32_t index;
  if (out_plan == NULL) return PROM_ERROR;
  memset(out_plan, 0, sizeof(*out_plan));
  if (request == NULL || request->tokens == 0u || request->tokens > PROM_M42_MAX_TOKENS ||
      request->model_width == 0u || request->model_width > PROM_M42_MAX_MODEL_WIDTH ||
      request->strategy < PROM_M46_STRATEGY_SEPARATE_OUTPUT ||
      request->strategy > PROM_M46_STRATEGY_IN_PLACE_Z ||
      request->submit_policy < PROM_M46_SUBMIT_ONE_COMMAND_BUFFER ||
      request->submit_policy > PROM_M46_SUBMIT_TWO_BOUNDED ||
      request->requested_reduction_plan > PROM_M46_REDUCTION_FORCE_STAGED ||
      (request->requested_reduction_plan == PROM_M46_REDUCTION_FORCE_FUSED &&
       request->model_width > 1024u) ||
      request->m45_replay_id == 0u ||
      !prom_m40b_checked_product_u64(request->tokens, request->model_width, &logical_elements) ||
      logical_elements > UINT64_MAX / sizeof(float)) return PROM_ERROR;
  memcpy(&epsilon_bits, &request->epsilon, sizeof(epsilon_bits));
  compact_bytes = logical_elements * sizeof(float);
  out_plan->tokens = request->tokens;
  out_plan->model_width = request->model_width;
  out_plan->z_row_stride = request->z_view.row_stride_elements;
  out_plan->n_row_stride = request->strategy == PROM_M46_STRATEGY_IN_PLACE_Z
                             ? request->z_view.row_stride_elements : request->model_width;
  out_plan->epsilon = request->epsilon;
  out_plan->strategy = request->strategy;
  out_plan->submit_policy = request->submit_policy;
  out_plan->reduction_plan = request->requested_reduction_plan == PROM_M46_REDUCTION_FORCE_STAGED
                               ? PROM_M46_REDUCTION_STAGED
                               : (request->model_width <= 1024u
                                    ? PROM_M46_REDUCTION_FUSED : PROM_M46_REDUCTION_STAGED);
  out_plan->partials_per_row = prom_reduction_ceil_div_u32(request->model_width, 1024u);
  out_plan->submit_count = request->submit_policy == PROM_M46_SUBMIT_TWO_BOUNDED ? 2u : 1u;
  out_plan->intermediate_host_copy_count = 0u;
  out_plan->final_readback_count = request->final_readback != 0u ? 1u : 0u;
  out_plan->z_generation = request->expected_z_generation;
  out_plan->weight_generation = request->weight_generation;
  out_plan->weight_hash = request->weight_hash;
  out_plan->reduce_shader_hash = PROM_M46_REDUCE_SHADER_HASH;
  out_plan->apply_shader_hash = PROM_M46_APPLY_SHADER_HASH;
  out_plan->m45_replay_id = request->m45_replay_id;
  out_plan->memory.capacity_limit_bytes = PROM_M46_CAPACITY_LIMIT_BYTES;
  out_plan->memory.reusable_descriptor_set_count = out_plan->reduction_plan == PROM_M46_REDUCTION_STAGED ? 3u : 2u;
  out_plan->memory.descriptor_binding_count = 4u;
  if (!prom_m45_view_required_bytes(&request->z_view, &z_bytes) ||
      request->z_view.buffer == VK_NULL_HANDLE ||
      request->z_view.element_type != PROM_DEVICE_ELEMENT_F32 ||
      request->z_view.layout != PROM_DEVICE_LAYOUT_ROW_MAJOR ||
      request->z_view.owning_device == VK_NULL_HANDLE ||
      request->z_view.byte_length < z_bytes || request->z_view.offset > UINT64_MAX - z_bytes)
    reason = PROM_M46_INELIGIBLE_VIEW;
  else if (request->z_view.logical_rows != request->tokens ||
           request->z_view.logical_columns != request->model_width)
    reason = PROM_M46_INELIGIBLE_SHAPE;
  else if (request->z_view.row_stride_elements < request->model_width)
    reason = PROM_M46_INELIGIBLE_STRIDE;
  else if (request->expected_z_generation == 0u ||
           request->z_view.owning_lifetime_id != request->expected_z_generation ||
           request->z_view.owning_slot_generation == 0u)
    reason = PROM_M46_INELIGIBLE_GENERATION;
  else if (request->weight_generation == 0u || request->weight_hash == 0u)
    reason = PROM_M46_INELIGIBLE_WEIGHT;
  else if (!isfinite(request->epsilon) || request->epsilon <= 0.0f)
    reason = PROM_M46_INELIGIBLE_EPSILON;
  else if (request->strategy == PROM_M46_STRATEGY_IN_PLACE_Z &&
           (request->z_exclusive == 0u || request->pre_normalization_z_consumer_count != 0u))
    reason = PROM_M46_INELIGIBLE_EXCLUSIVITY;
  out_plan->memory.z_view_bytes = z_bytes;
  /* Concept's shared allocations are uploaded/read directly. There is no
     duplicate staging weight or N readback allocation in the current path. */
  out_plan->memory.weight_upload_bytes = 0u;
  out_plan->memory.weight_device_bytes = (uint64_t)request->model_width * sizeof(float);
  if (out_plan->reduction_plan == PROM_M46_REDUCTION_STAGED) {
    if (!prom_m40b_checked_product_u64(request->tokens, out_plan->partials_per_row,
                                       &partial_elements) ||
        partial_elements > UINT64_MAX / sizeof(float)) return PROM_ERROR;
    out_plan->memory.partial_sum_bytes = partial_elements * sizeof(float);
  }
  out_plan->memory.inv_rms_bytes = (uint64_t)request->tokens * sizeof(float);
  out_plan->memory.n_device_bytes = request->strategy == PROM_M46_STRATEGY_SEPARATE_OUTPUT
                                      ? compact_bytes : 0u;
  out_plan->memory.n_readback_bytes = 0u;
  out_plan->memory.in_place_saved_bytes = compact_bytes;
  if (!prom_m43_checked_add_u64(z_bytes, out_plan->memory.weight_device_bytes, &total) ||
      !prom_m43_checked_add_u64(total, out_plan->memory.partial_sum_bytes, &total) ||
      !prom_m43_checked_add_u64(total, out_plan->memory.inv_rms_bytes, &total) ||
      !prom_m43_checked_add_u64(total, out_plan->memory.n_device_bytes, &total) ||
      !prom_m43_checked_add_u64(total, out_plan->memory.n_readback_bytes, &total)) return PROM_ERROR;
  out_plan->memory.exact_request_bytes = total;
  if (reason == PROM_M46_ELIGIBLE && total > PROM_M46_CAPACITY_LIMIT_BYTES)
    reason = PROM_M46_INELIGIBLE_CAPACITY;
  eligibility_hash = prom_reduction_hash_u32(eligibility_hash, request->tokens);
  eligibility_hash = prom_reduction_hash_u32(eligibility_hash, request->model_width);
  eligibility_hash = prom_reduction_hash_u32(eligibility_hash, request->z_view.row_stride_elements);
  eligibility_hash = prom_reduction_hash_u32(eligibility_hash, epsilon_bits);
  eligibility_hash = prom_reduction_hash_u32(eligibility_hash, request->strategy);
  eligibility_hash = prom_reduction_hash_u32(eligibility_hash, request->submit_policy);
  eligibility_hash = prom_reduction_hash_u32(eligibility_hash, request->requested_reduction_plan);
  eligibility_hash = prom_reduction_hash_u32(eligibility_hash, reason);
  eligibility_hash = prom_m40b_hash_u64(eligibility_hash, total);
  out_plan->eligibility_reason = reason;
  out_plan->eligibility_eligible = reason == PROM_M46_ELIGIBLE ? 1u : 0u;
  out_plan->eligibility_replay_id = eligibility_hash;
  if (reason != PROM_M46_ELIGIBLE) return PROM_OK;

  prom_m46_add_barrier(out_plan, PROM_M46_BUFFER_Z, request->z_view.offset, z_bytes,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT);
  prom_m46_add_stage(out_plan, PROM_M46_STAGE_Z_READY, 0u, 0u, 1u, 0u, 0u, 1u);
  if (out_plan->reduction_plan == PROM_M46_REDUCTION_STAGED) {
    prom_m46_add_barrier(out_plan, PROM_M46_BUFFER_PARTIALS, 0u,
                         out_plan->memory.partial_sum_bytes,
                         VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                         VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                         VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT);
    prom_m46_add_stage(out_plan, PROM_M46_STAGE_SUM_SQUARES, 1u, 1u, 1u, 0u, 1u, 2u);
    prom_m46_add_stage(out_plan, PROM_M46_STAGE_FINAL_REDUCTION, 1u, 2u, 0u, 0u, 2u, 3u);
  } else {
    prom_m46_add_stage(out_plan, PROM_M46_STAGE_SUM_SQUARES, 1u, 1u, 0u, 0u, 1u, 2u);
  }
  prom_m46_add_barrier(out_plan, PROM_M46_BUFFER_INV_RMS, 0u,
                       out_plan->memory.inv_rms_bytes,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT);
  prom_m46_add_stage(out_plan, PROM_M46_STAGE_INV_RMS_READY, 0u,
                     out_plan->barrier_count - 1u, 1u, 0u, 2u, 3u);
  prom_m46_add_barrier(out_plan, PROM_M46_BUFFER_Z, request->z_view.offset, z_bytes,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_ACCESS_SHADER_READ_BIT,
                       request->strategy == PROM_M46_STRATEGY_IN_PLACE_Z
                         ? VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT
                         : VK_ACCESS_SHADER_READ_BIT);
  prom_m46_add_barrier(out_plan, PROM_M46_BUFFER_N, 0u,
                       request->strategy == PROM_M46_STRATEGY_IN_PLACE_Z ? z_bytes : compact_bytes,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       request->final_readback != 0u ? VK_PIPELINE_STAGE_HOST_BIT
                                                     : VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_ACCESS_SHADER_WRITE_BIT,
                       request->final_readback != 0u ? VK_ACCESS_HOST_READ_BIT
                                                     : VK_ACCESS_SHADER_READ_BIT);
  prom_m46_add_stage(out_plan, PROM_M46_STAGE_APPLY, 1u, out_plan->barrier_count - 2u,
                     2u, 0u, 3u, 4u);
  if (request->final_readback != 0u) {
    prom_m46_add_barrier(out_plan, PROM_M46_BUFFER_INV_RMS, 0u,
                         out_plan->memory.inv_rms_bytes,
                         VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_HOST_BIT,
                         VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_HOST_READ_BIT);
    prom_m46_add_stage(out_plan, PROM_M46_STAGE_FINAL_READBACK, 0u,
                       out_plan->barrier_count - 2u, 2u, 0u, 4u, 5u);
  }
  command_hash = prom_reduction_hash_u32(command_hash, out_plan->submit_policy);
  command_hash = prom_reduction_hash_u32(command_hash, out_plan->reduction_plan);
  command_hash = prom_reduction_hash_u32(command_hash, out_plan->strategy);
  command_hash = prom_reduction_hash_u32(command_hash, out_plan->stage_count);
  command_hash = prom_reduction_hash_u32(command_hash, out_plan->barrier_count);
  for (index = 0u; index < out_plan->barrier_count; ++index) {
    const prom_m46_barrier_trace* barrier = &out_plan->barriers[index];
    command_hash = prom_reduction_hash_u32(command_hash, barrier->buffer_identity);
    command_hash = prom_m40b_hash_u64(command_hash, barrier->byte_offset);
    command_hash = prom_m40b_hash_u64(command_hash, barrier->byte_length);
    command_hash = prom_reduction_hash_u32(command_hash, barrier->source_access_mask);
    command_hash = prom_reduction_hash_u32(command_hash, barrier->destination_access_mask);
  }
  out_plan->command_plan_replay_id = command_hash;
  replay_hash = prom_reduction_hash_u32(replay_hash, request->tokens);
  replay_hash = prom_reduction_hash_u32(replay_hash, request->model_width);
  replay_hash = prom_reduction_hash_u32(replay_hash, request->z_view.row_stride_elements);
  replay_hash = prom_reduction_hash_u32(replay_hash, epsilon_bits);
  replay_hash = prom_reduction_hash_u32(replay_hash, request->strategy);
  replay_hash = prom_reduction_hash_u32(replay_hash, request->submit_policy);
  replay_hash = prom_reduction_hash_u32(replay_hash, request->requested_reduction_plan);
  replay_hash = prom_reduction_hash_u32(replay_hash, out_plan->reduction_plan);
  replay_hash = prom_m40b_hash_u64(replay_hash, request->expected_z_generation);
  replay_hash = prom_m40b_hash_u64(replay_hash, request->weight_generation);
  replay_hash = prom_m40b_hash_u64(replay_hash, request->weight_hash);
  replay_hash = prom_m40b_hash_u64(replay_hash, PROM_M46_REDUCE_SHADER_HASH);
  replay_hash = prom_m40b_hash_u64(replay_hash, PROM_M46_APPLY_SHADER_HASH);
  replay_hash = prom_m40b_hash_u64(replay_hash, request->m45_replay_id);
  replay_hash = prom_m40b_hash_u64(replay_hash, command_hash);
  out_plan->replay_id = replay_hash;
  out_plan->n_generation = prom_m40b_hash_u64(replay_hash, request->expected_z_generation ^
                                                           request->weight_generation);
  if (out_plan->n_generation == 0u) out_plan->n_generation = 1u;
  return PROM_OK;
}

int prom_m46_rmsnorm_cpu_reference(const prom_m46_reference_request* request) {
  uint64_t z_count;
  uint64_t n_count;
  uint32_t token;
  if (request == NULL || request->z == NULL || request->weight == NULL || request->n == NULL ||
      request->tokens == 0u || request->tokens > PROM_M42_MAX_TOKENS ||
      request->model_width == 0u || request->model_width > PROM_M42_MAX_MODEL_WIDTH ||
      request->z_row_stride < request->model_width || request->n_row_stride < request->model_width ||
      !isfinite(request->epsilon) || request->epsilon <= 0.0f ||
      !prom_m40b_checked_product_u64(request->tokens, request->z_row_stride, &z_count) ||
      !prom_m40b_checked_product_u64(request->tokens, request->n_row_stride, &n_count) ||
      request->z_element_count < z_count || request->n_element_count < n_count ||
      request->weight_element_count != request->model_width) return PROM_ERROR;
  for (token = 0u; token < request->tokens; ++token) {
    float sumsq = 0.0f;
    float inv_rms;
    uint32_t column;
    for (column = 0u; column < request->model_width; ++column) {
      const float z = request->z[(uint64_t)token * request->z_row_stride + column];
      if (!isfinite(z) || !isfinite(request->weight[column])) return PROM_ERROR;
      sumsq += z * z;
    }
    inv_rms = 1.0f / sqrtf(sumsq / (float)request->model_width + request->epsilon);
    if (!isfinite(sumsq) || !isfinite(inv_rms)) return PROM_ERROR;
    if (request->inv_rms != NULL) request->inv_rms[token] = inv_rms;
    for (column = 0u; column < request->model_width; ++column) {
      const float n = request->z[(uint64_t)token * request->z_row_stride + column] * inv_rms *
                      request->weight[column];
      if (!isfinite(n)) return PROM_ERROR;
      request->n[(uint64_t)token * request->n_row_stride + column] = n;
    }
  }
  return PROM_OK;
}

int prom_m46_rmsnorm_compare(const float* expected,
                             const float* actual,
                             uint32_t tokens,
                             uint32_t model_width,
                             float absolute_tolerance,
                             float relative_tolerance,
                             const prom_rmsnorm_plan* plan,
                             const float* sumsq,
                             const float* inv_rms,
                             prom_m46_mismatch* out_mismatch) {
  uint32_t token;
  if (out_mismatch == NULL) return PROM_ERROR;
  memset(out_mismatch, 0, sizeof(*out_mismatch));
  out_mismatch->matched = 1u;
  if (plan != NULL) {
    out_mismatch->strategy = plan->strategy;
    out_mismatch->epsilon = plan->epsilon;
    out_mismatch->z_generation = plan->z_generation;
    out_mismatch->weight_generation = plan->weight_generation;
    out_mismatch->n_generation = plan->n_generation;
    out_mismatch->m45_replay_id = plan->m45_replay_id;
    out_mismatch->m46_replay_id = plan->replay_id;
  }
  if (expected == NULL || actual == NULL || plan == NULL || tokens == 0u || model_width == 0u ||
      absolute_tolerance < 0.0f || relative_tolerance < 0.0f) return PROM_ERROR;
  for (token = 0u; token < tokens; ++token) {
    uint32_t column;
    for (column = 0u; column < model_width; ++column) {
      const uint64_t element = (uint64_t)token * model_width + column;
      const float absolute_error = fabsf(expected[element] - actual[element]);
      const float relative_error = absolute_error / fmaxf(fabsf(expected[element]), 1.0e-12f);
      if (!isfinite(expected[element]) || !isfinite(actual[element]) ||
          (absolute_error > absolute_tolerance && relative_error > relative_tolerance)) {
        out_mismatch->matched = 0u;
        out_mismatch->token = token;
        out_mismatch->column = column;
        out_mismatch->expected = expected[element];
        out_mismatch->actual = actual[element];
        out_mismatch->absolute_error = absolute_error;
        out_mismatch->relative_error = relative_error;
        out_mismatch->sumsq = sumsq != NULL ? sumsq[token] : 0.0f;
        out_mismatch->inv_rms = inv_rms != NULL ? inv_rms[token] : 0.0f;
        return PROM_ERROR;
      }
    }
  }
  return PROM_OK;
}

typedef struct prom_gemma4e2b_m1_rope_push {
  uint32_t token_count;
  uint32_t head_count;
  uint32_t head_dim;
  uint32_t reserved[5];
} prom_gemma4e2b_m1_rope_push;

static int prom_gemma4e2b_m1_ensure_rope_pipeline(
    prom_reduction_runtime_state* state) {
  prom_reduction_pipeline* destination;
  const char* entry_point = NULL;
  prom_shader_package_diagnostic package_diagnostic;
  VkPipelineShaderStageCreateInfo stage_info;
  VkComputePipelineCreateInfo pipeline_info;
  if (state == NULL || state->pipeline_layout == VK_NULL_HANDLE) return 0;
  destination = &state->gemma4e2b_m1_rope_pipeline;
  if (destination->pipeline != VK_NULL_HANDLE) return 1;
  if (state->shader_package == NULL ||
      !prom_shader_package_create_module(state->shader_package, state->device,
                                         "kernel-68-default", &destination->shader_module,
                                         &entry_point, &package_diagnostic)) return 0;
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = destination->shader_module;
  stage_info.pName = entry_point;
  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = state->pipeline_layout;
  if (vkCreateComputePipelines(state->device, VK_NULL_HANDLE, 1u, &pipeline_info,
                               NULL, &destination->pipeline) != VK_SUCCESS) {
    prom_reduction_destroy_pipeline(state->device, destination);
    return 0;
  }
  destination->implementation_id = 68u;
  state->gemma4e2b_m1_rope_pipeline_create_count += 1u;
  return 1;
}

typedef struct prom_gemma4e2b_m1_attention_scores_push {
  uint32_t token_count;
  uint32_t query_head_count;
  uint32_t key_head_count;
  uint32_t head_dim;
  float scale;
  uint32_t reserved[3];
} prom_gemma4e2b_m1_attention_scores_push;

static int prom_gemma4e2b_m1_ensure_attention_scores_pipeline(
    prom_reduction_runtime_state* state) {
  prom_reduction_pipeline* destination;
  const char* entry_point = NULL;
  prom_shader_package_diagnostic package_diagnostic;
  VkPipelineShaderStageCreateInfo stage_info;
  VkComputePipelineCreateInfo pipeline_info;
  if (state == NULL || state->pipeline_layout == VK_NULL_HANDLE) return 0;
  destination = &state->gemma4e2b_m1_attention_scores_pipeline;
  if (destination->pipeline != VK_NULL_HANDLE) return 1;
  if (state->shader_package == NULL ||
      !prom_shader_package_create_module(state->shader_package, state->device,
                                         "kernel-69-default", &destination->shader_module,
                                         &entry_point, &package_diagnostic)) return 0;
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = destination->shader_module;
  stage_info.pName = entry_point;
  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = state->pipeline_layout;
  if (vkCreateComputePipelines(state->device, VK_NULL_HANDLE, 1u, &pipeline_info,
                               NULL, &destination->pipeline) != VK_SUCCESS) {
    prom_reduction_destroy_pipeline(state->device, destination);
    return 0;
  }
  destination->implementation_id = 69u;
  state->gemma4e2b_m1_attention_scores_pipeline_create_count += 1u;
  return 1;
}

static void prom_gemma4e2b_m1_update_attention_scores_descriptor(
    prom_reduction_runtime_state* state, VkDescriptorSet set,
    const prom_vk_buffer* query, const prom_vk_buffer* key,
    const prom_vk_buffer* score) {
  prom_reduction_buffer_bindings bindings;
  bindings.input = query;       /* binding 0: retained positional Q */
  bindings.auxiliary0 = key;    /* binding 1: retained positional K */
  bindings.auxiliary1 = key;    /* binding 2 is intentionally unused by kernel 69. */
  bindings.output = score;      /* binding 3: distinct FP32 raw-score output */
  prom_reduction_update_descriptor_set(state, set, &bindings);
  state->gemma4e2b_m1_attention_scores_descriptor_update_count += 1u;
}

static int prom_gemma4e2b_m1_ensure_rope_buffer(
    prom_reduction_runtime_state* state, prom_vk_buffer* buffer,
    VkDeviceSize size, VkBufferUsageFlags usage,
    VkMemoryPropertyFlags properties, int map_memory) {
  const uint64_t allocations_before = state->diagnostics.buffer_allocation_count;
  if (!prom_reduction_ensure_buffer(state, buffer, size, usage, properties, map_memory)) return 0;
  if (state->diagnostics.buffer_allocation_count != allocations_before)
    state->gemma4e2b_m1_rope_buffer_grow_count += 1u;
  else
    state->gemma4e2b_m1_rope_buffer_reuse_count += 1u;
  return 1;
}

static void prom_gemma4e2b_m1_update_rope_descriptor(
    prom_reduction_runtime_state* state, VkDescriptorSet set,
    const prom_vk_buffer* source, const prom_vk_buffer* cosine,
    const prom_vk_buffer* sine, const prom_vk_buffer* destination) {
  prom_reduction_buffer_bindings bindings;
  bindings.input = source;
  bindings.auxiliary0 = cosine;
  bindings.auxiliary1 = sine;
  bindings.output = destination;
  prom_reduction_update_descriptor_set(state, set, &bindings);
  state->gemma4e2b_m1_rope_descriptor_update_count += 1u;
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

static int prom_m43_buffer_barriers(VkCommandBuffer command_buffer,
                                    const prom_vk_buffer* const* buffers,
                                    const VkDeviceSize* sizes,
                                    uint32_t count,
                                    VkAccessFlags source_access,
                                    VkAccessFlags destination_access,
                                    VkPipelineStageFlags source_stage,
                                    VkPipelineStageFlags destination_stage) {
  VkBufferMemoryBarrier barriers[PROM_M43_HEAD_COUNT * PROM_M43_WEIGHT_KIND_COUNT];
  uint32_t index;
  if (count == 0u || count > PROM_M43_HEAD_COUNT * PROM_M43_WEIGHT_KIND_COUNT) return 0;
  memset(barriers, 0, sizeof(barriers));
  for (index = 0u; index < count; ++index) {
    if (buffers[index] == NULL || buffers[index]->buffer == VK_NULL_HANDLE || sizes[index] == 0u ||
        sizes[index] > buffers[index]->size) return 0;
    barriers[index].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    barriers[index].srcAccessMask = source_access;
    barriers[index].dstAccessMask = destination_access;
    barriers[index].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[index].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    barriers[index].buffer = buffers[index]->buffer;
    barriers[index].offset = 0u;
    barriers[index].size = sizes[index];
  }
  vkCmdPipelineBarrier(command_buffer, source_stage, destination_stage, 0u,
                       0u, NULL, count, barriers, 0u, NULL);
  return 1;
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

static int prom_m43_one_buffer_barrier(VkCommandBuffer command_buffer,
                                       const prom_vk_buffer* buffer,
                                       VkDeviceSize size,
                                       VkAccessFlags source_access,
                                       VkAccessFlags destination_access,
                                       VkPipelineStageFlags source_stage,
                                       VkPipelineStageFlags destination_stage) {
  const prom_vk_buffer* buffers[1] = {buffer};
  const VkDeviceSize sizes[1] = {size};
  return prom_m43_buffer_barriers(command_buffer, buffers, sizes, 1u,
                                  source_access, destination_access, source_stage, destination_stage);
}

typedef struct prom_m47_continuation prom_m47_continuation;

int prom_reactor_runtime_gemma4e2b_m1_rope(
    void* handle, const PrometheusGemma4E2BM1RopeRequest* request,
    PrometheusGemma4E2BM1RopeResult* out_result) {
  prom_reduction_runtime_state* state;
  prom_reduction_slot* slot;
  prom_gemma4e2b_m1_rope_push push;
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  VkSubmitInfo submit;
  VkResult vk_result;
  uint64_t source_elements;
  uint64_t table_elements;
  uint64_t source_bytes;
  uint64_t table_bytes;
  uint64_t allocations_before;
  uint64_t reuses_before;
  uint32_t finite = 0u;
  int32_t detail = 0;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  out_result->struct_size = sizeof(*out_result);
  if (request == NULL || request->struct_size < sizeof(*request) ||
      request->source == NULL || request->cosine == NULL || request->sine == NULL ||
      request->output == NULL || request->source == request->output ||
      request->cosine == request->output || request->sine == request->output ||
      request->cosine == request->sine || request->tokens != 15u ||
      (request->heads != 1u && request->heads != 8u) || request->head_width != 256u ||
      request->source_generation == 0u || request->table_generation == 0u ||
      !prom_m40b_checked_product_u64(request->tokens, request->heads, &source_elements) ||
      !prom_m40b_checked_product_u64(source_elements, request->head_width, &source_elements) ||
      !prom_m40b_checked_product_u64(request->tokens, request->head_width, &table_elements) ||
      request->source_element_count != source_elements ||
      request->output_element_count != source_elements ||
      request->cosine_element_count != table_elements ||
      request->sine_element_count != table_elements ||
      source_elements > UINT32_MAX || source_elements > SIZE_MAX / sizeof(float) ||
      table_elements > SIZE_MAX / sizeof(float)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M46_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  out_result->source_hash = prom_m42_hash_finite_matrix(request->source, source_elements, &finite);
  if (finite == 0u || out_result->source_hash == 0u) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M46_DETAIL_NONFINITE_INPUT;
    return PROM_ERROR;
  }
  out_result->cosine_hash = prom_m42_hash_finite_matrix(request->cosine, table_elements, &finite);
  if (finite == 0u || out_result->cosine_hash == 0u) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M46_DETAIL_NONFINITE_INPUT;
    return PROM_ERROR;
  }
  out_result->sine_hash = prom_m42_hash_finite_matrix(request->sine, table_elements, &finite);
  if (finite == 0u || out_result->sine_hash == 0u) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M46_DETAIL_NONFINITE_INPUT;
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL || !prom_gemma4e2b_m1_ensure_rope_pipeline(state)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = state == NULL ? detail : PROM_M46_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  slot = prom_reduction_acquire_slot(state, state->next_logical_request_id++);
  if (slot == NULL) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M46_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  source_bytes = source_elements * sizeof(float);
  table_bytes = table_elements * sizeof(float);
  allocations_before = state->gemma4e2b_m1_rope_buffer_grow_count;
  reuses_before = state->gemma4e2b_m1_rope_buffer_reuse_count;
  if (!prom_gemma4e2b_m1_ensure_rope_buffer(
          state, &slot->m48_host_initial_upload, (VkDeviceSize)source_bytes,
          VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
          VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1) ||
      !prom_gemma4e2b_m1_ensure_rope_buffer(
          state, &slot->gemma4e2b_m1_rope_input, (VkDeviceSize)source_bytes,
          VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
          VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_gemma4e2b_m1_ensure_rope_buffer(
          state, &state->gemma4e2b_m1_rope_cosine_upload, (VkDeviceSize)table_bytes,
          VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
          VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1) ||
      !prom_gemma4e2b_m1_ensure_rope_buffer(
          state, &state->gemma4e2b_m1_rope_cosine, (VkDeviceSize)table_bytes,
          VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
          VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_gemma4e2b_m1_ensure_rope_buffer(
          state, &state->gemma4e2b_m1_rope_sine_upload, (VkDeviceSize)table_bytes,
          VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
          VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1) ||
      !prom_gemma4e2b_m1_ensure_rope_buffer(
          state, &state->gemma4e2b_m1_rope_sine, (VkDeviceSize)table_bytes,
          VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
          VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_gemma4e2b_m1_ensure_rope_buffer(
          state, &slot->gemma4e2b_m1_output, (VkDeviceSize)source_bytes,
          VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
          VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_gemma4e2b_m1_ensure_rope_buffer(
          state, &slot->gemma4e2b_m1_readback, (VkDeviceSize)source_bytes,
          VK_BUFFER_USAGE_TRANSFER_DST_BIT,
          VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M46_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  memcpy(slot->m48_host_initial_upload.mapped, request->source, (size_t)source_bytes);
  memcpy(state->gemma4e2b_m1_rope_cosine_upload.mapped, request->cosine, (size_t)table_bytes);
  memcpy(state->gemma4e2b_m1_rope_sine_upload.mapped, request->sine, (size_t)table_bytes);
  prom_gemma4e2b_m1_update_rope_descriptor(
      state, slot->descriptor_sets[3], &slot->gemma4e2b_m1_rope_input,
      &state->gemma4e2b_m1_rope_cosine, &state->gemma4e2b_m1_rope_sine,
      &slot->gemma4e2b_m1_output);
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkResetCommandBuffer(slot->command_buffer, 0u) != VK_SUCCESS ||
      vkBeginCommandBuffer(slot->command_buffer, &begin_info) != VK_SUCCESS)
    goto rope_command_fail;
  prom_m42_buffer_barrier(slot->command_buffer, &slot->m48_host_initial_upload,
                          VK_ACCESS_HOST_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                          VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
  prom_m42_buffer_barrier(slot->command_buffer, &state->gemma4e2b_m1_rope_cosine_upload,
                          VK_ACCESS_HOST_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                          VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
  prom_m42_buffer_barrier(slot->command_buffer, &state->gemma4e2b_m1_rope_sine_upload,
                          VK_ACCESS_HOST_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                          VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
  memset(&copy, 0, sizeof(copy));
  copy.size = (VkDeviceSize)source_bytes;
  vkCmdCopyBuffer(slot->command_buffer, slot->m48_host_initial_upload.buffer,
                  slot->gemma4e2b_m1_rope_input.buffer, 1u, &copy);
  copy.size = (VkDeviceSize)table_bytes;
  vkCmdCopyBuffer(slot->command_buffer, state->gemma4e2b_m1_rope_cosine_upload.buffer,
                  state->gemma4e2b_m1_rope_cosine.buffer, 1u, &copy);
  vkCmdCopyBuffer(slot->command_buffer, state->gemma4e2b_m1_rope_sine_upload.buffer,
                  state->gemma4e2b_m1_rope_sine.buffer, 1u, &copy);
  if (!prom_m43_one_buffer_barrier(slot->command_buffer, &slot->gemma4e2b_m1_rope_input,
                                   (VkDeviceSize)source_bytes,
                                   VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT) ||
      !prom_m43_one_buffer_barrier(slot->command_buffer, &state->gemma4e2b_m1_rope_cosine,
                                   (VkDeviceSize)table_bytes,
                                   VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT) ||
      !prom_m43_one_buffer_barrier(slot->command_buffer, &state->gemma4e2b_m1_rope_sine,
                                   (VkDeviceSize)table_bytes,
                                   VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT))
    goto rope_command_fail;
  memset(&push, 0, sizeof(push));
  push.token_count = request->tokens;
  push.head_count = request->heads;
  push.head_dim = request->head_width;
  vkCmdBindPipeline(slot->command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                    state->gemma4e2b_m1_rope_pipeline.pipeline);
  vkCmdBindDescriptorSets(slot->command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                          state->pipeline_layout, 0u, 1u, &slot->descriptor_sets[3], 0u, NULL);
  vkCmdPushConstants(slot->command_buffer, state->pipeline_layout,
                     VK_SHADER_STAGE_COMPUTE_BIT, 0u, sizeof(push), &push);
  vkCmdDispatch(slot->command_buffer,
                prom_reduction_ceil_div_u32((uint32_t)source_elements, 256u), 1u, 1u);
  if (!prom_m43_one_buffer_barrier(slot->command_buffer, &slot->gemma4e2b_m1_output,
                                   (VkDeviceSize)source_bytes,
                                   VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT))
    goto rope_command_fail;
  memset(&copy, 0, sizeof(copy));
  copy.size = (VkDeviceSize)source_bytes;
  vkCmdCopyBuffer(slot->command_buffer, slot->gemma4e2b_m1_output.buffer,
                  slot->gemma4e2b_m1_readback.buffer, 1u, &copy);
  if (!prom_m43_one_buffer_barrier(slot->command_buffer, &slot->gemma4e2b_m1_readback,
                                   (VkDeviceSize)source_bytes,
                                   VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_HOST_READ_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT,
                                   VK_PIPELINE_STAGE_HOST_BIT) ||
      vkEndCommandBuffer(slot->command_buffer) != VK_SUCCESS ||
      vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS)
    goto rope_command_fail;
  memset(&submit, 0, sizeof(submit));
  submit.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit.commandBufferCount = 1u;
  submit.pCommandBuffers = &slot->command_buffer;
  if (prom_vk_queue_submit(state->queue, 1u, &submit, slot->fence) != VK_SUCCESS)
    goto rope_submit_fail;
  slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
  vk_result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
  if (vk_result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M46_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  memcpy(request->output, slot->gemma4e2b_m1_readback.mapped, (size_t)source_bytes);
  out_result->output_hash = prom_m42_hash_finite_matrix(request->output, source_elements, &finite);
  if (finite == 0u || out_result->output_hash == 0u) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->stage = PROM_STAGE_TRANSFER_OUT;
    out_result->detail_code = PROM_M46_DETAIL_READBACK;
    return PROM_ERROR;
  }
  out_result->stage = 0u;
  out_result->detail_code = 0;
  out_result->output_written = 1u;
  out_result->dispatch_count = 1u;
  out_result->buffer_allocation_count =
      state->gemma4e2b_m1_rope_buffer_grow_count - allocations_before;
  out_result->buffer_reuse_count =
      state->gemma4e2b_m1_rope_buffer_reuse_count - reuses_before;
  out_result->descriptor_update_count = state->gemma4e2b_m1_rope_descriptor_update_count;
  out_result->pipeline_create_count = state->gemma4e2b_m1_rope_pipeline_create_count;
  out_result->command_buffer_reuse_count = slot->gemma4e2b_m1_command_reuse_count;
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  return PROM_OK;

rope_command_fail:
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->stage = PROM_STAGE_SUBMIT;
  out_result->detail_code = PROM_M46_DETAIL_COMMAND;
  return PROM_ERROR;
rope_submit_fail:
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->stage = PROM_STAGE_SUBMIT;
  out_result->detail_code = PROM_M46_DETAIL_SUBMIT;
  return PROM_ERROR;
}

int prom_reactor_runtime_gemma4e2b_m1_attention_scores(
    void* handle, const PrometheusGemma4E2BM1AttentionScoresRequest* request,
    PrometheusGemma4E2BM1AttentionScoresResult* out_result) {
  prom_reduction_runtime_state* state;
  prom_device_buffer_view query_view, key_view;
  prom_vk_buffer query_buffer = {0}, key_buffer = {0};
  prom_reduction_slot* score_slot;
  prom_gemma4e2b_m1_attention_scores_push push;
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  VkSubmitInfo submit;
  VkResult vk_result;
  uint64_t query_elements;
  uint64_t key_elements;
  uint64_t score_elements;
  uint64_t query_bytes;
  uint64_t key_bytes;
  uint64_t score_bytes;
  uint64_t allocations_before;
  uint64_t reuses_before;
  int32_t detail = 0;
  uint32_t finite = 0u;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  out_result->struct_size = sizeof(*out_result);
  if (request == NULL || request->struct_size < sizeof(*request) || request->scores == NULL ||
      request->tokens != 15u || request->query_heads != 8u || request->key_heads != 1u ||
      request->head_width != 256u || request->scale != 0.0625f ||
      !prom_m40b_checked_product_u64(request->tokens, request->query_heads, &query_elements) ||
      !prom_m40b_checked_product_u64(query_elements, request->head_width, &query_elements) ||
      !prom_m40b_checked_product_u64(request->tokens, request->key_heads, &key_elements) ||
      !prom_m40b_checked_product_u64(key_elements, request->head_width, &key_elements) ||
      !prom_m40b_checked_product_u64(request->query_heads, request->tokens, &score_elements) ||
      !prom_m40b_checked_product_u64(score_elements, request->tokens, &score_elements) ||
      request->score_element_count != score_elements || score_elements > UINT32_MAX ||
      score_elements > SIZE_MAX / sizeof(float)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M46_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL || !prom_gemma4e2b_m1_ensure_attention_scores_pipeline(state)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = state == NULL ? detail : PROM_M46_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  if (prom_reactor_runtime_concept_norm_resident(handle, 8u, &query_view) != PROM_OK ||
      prom_reactor_runtime_concept_norm_resident(handle, 1u, &key_view) != PROM_OK ||
      query_view.owning_device != state->device || key_view.owning_device != state->device ||
      query_view.byte_length != query_elements * sizeof(float) ||
      key_view.byte_length != key_elements * sizeof(float)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M46_DETAIL_STALE_WEIGHT_GENERATION;
    return PROM_ERROR;
  }
  query_buffer.buffer = query_view.buffer; query_buffer.size = query_view.byte_length;
  key_buffer.buffer = key_view.buffer; key_buffer.size = key_view.byte_length;
  query_bytes = query_elements * sizeof(float);
  key_bytes = key_elements * sizeof(float);
  score_bytes = score_elements * sizeof(float);
  allocations_before = state->gemma4e2b_m1_rope_buffer_grow_count;
  reuses_before = state->gemma4e2b_m1_rope_buffer_reuse_count;
  score_slot = prom_reduction_acquire_slot(state, state->next_logical_request_id++);
  if (score_slot == NULL) {
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M46_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  if (!prom_gemma4e2b_m1_ensure_rope_buffer(
          state, &score_slot->gemma4e2b_m1_output, (VkDeviceSize)score_bytes,
          VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
          VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_gemma4e2b_m1_ensure_rope_buffer(
          state, &score_slot->gemma4e2b_m1_readback, (VkDeviceSize)score_bytes,
          VK_BUFFER_USAGE_TRANSFER_DST_BIT,
          VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1)) {
    score_slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M46_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkResetCommandBuffer(score_slot->command_buffer, 0u) != VK_SUCCESS ||
      vkBeginCommandBuffer(score_slot->command_buffer, &begin_info) != VK_SUCCESS)
    goto score_command_fail;
  if (!prom_m43_one_buffer_barrier(score_slot->command_buffer,
                                   &query_buffer,
                                   (VkDeviceSize)query_bytes,
                                   VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT) ||
      !prom_m43_one_buffer_barrier(score_slot->command_buffer,
                                   &key_buffer,
                                   (VkDeviceSize)key_bytes,
                                   VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT))
    goto score_command_fail;
  prom_gemma4e2b_m1_update_attention_scores_descriptor(
      state, score_slot->descriptor_sets[3], &query_buffer,
      &key_buffer, &score_slot->gemma4e2b_m1_output);
  memset(&push, 0, sizeof(push));
  push.token_count = request->tokens;
  push.query_head_count = request->query_heads;
  push.key_head_count = request->key_heads;
  push.head_dim = request->head_width;
  push.scale = request->scale;
  vkCmdBindPipeline(score_slot->command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                    state->gemma4e2b_m1_attention_scores_pipeline.pipeline);
  vkCmdBindDescriptorSets(score_slot->command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                          state->pipeline_layout, 0u, 1u, &score_slot->descriptor_sets[3],
                          0u, NULL);
  vkCmdPushConstants(score_slot->command_buffer, state->pipeline_layout,
                     VK_SHADER_STAGE_COMPUTE_BIT, 0u, sizeof(push), &push);
  vkCmdDispatch(score_slot->command_buffer,
                prom_reduction_ceil_div_u32((uint32_t)score_elements, 256u), 1u, 1u);
  if (!prom_m43_one_buffer_barrier(score_slot->command_buffer, &score_slot->gemma4e2b_m1_output,
                                   (VkDeviceSize)score_bytes,
                                   VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT) ||
      !prom_m43_one_buffer_barrier(score_slot->command_buffer, &score_slot->gemma4e2b_m1_readback,
                                   (VkDeviceSize)score_bytes,
                                   VK_ACCESS_HOST_READ_BIT, VK_ACCESS_TRANSFER_WRITE_BIT,
                                   VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT))
    goto score_command_fail;
  memset(&copy, 0, sizeof(copy));
  copy.size = (VkDeviceSize)score_bytes;
  vkCmdCopyBuffer(score_slot->command_buffer, score_slot->gemma4e2b_m1_output.buffer,
                  score_slot->gemma4e2b_m1_readback.buffer, 1u, &copy);
  if (!prom_m43_one_buffer_barrier(score_slot->command_buffer, &score_slot->gemma4e2b_m1_readback,
                                   (VkDeviceSize)score_bytes,
                                   VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_HOST_READ_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_HOST_BIT) ||
      vkEndCommandBuffer(score_slot->command_buffer) != VK_SUCCESS ||
      vkResetFences(state->device, 1u, &score_slot->fence) != VK_SUCCESS)
    goto score_command_fail;
  memset(&submit, 0, sizeof(submit));
  submit.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit.commandBufferCount = 1u;
  submit.pCommandBuffers = &score_slot->command_buffer;
  if (prom_vk_queue_submit(state->queue, 1u, &submit, score_slot->fence) != VK_SUCCESS)
    goto score_submit_fail;
  score_slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
  vk_result = vkWaitForFences(state->device, 1u, &score_slot->fence, VK_TRUE, UINT64_MAX);
  if (vk_result != VK_SUCCESS) {
    score_slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M46_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  memcpy(request->scores, score_slot->gemma4e2b_m1_readback.mapped, (size_t)score_bytes);
  (void)prom_m42_hash_finite_matrix(request->scores, score_elements, &finite);
  out_result->score_hash = prom_num_hash_float_bits(request->scores, score_elements);
  if (finite == 0u || out_result->score_hash == 0u) {
    score_slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->stage = PROM_STAGE_TRANSFER_OUT;
    out_result->detail_code = PROM_M46_DETAIL_READBACK;
    return PROM_ERROR;
  }
  out_result->stage = 0u;
  out_result->score_written = 1u;
  out_result->score_dispatch_count = 1u;
  out_result->score_readback_count = 1u;
  out_result->host_detour_count = 0u;
  out_result->query_slot_id = query_view.owning_slot_id;
  out_result->query_slot_generation = query_view.owning_slot_generation;
  out_result->key_slot_id = key_view.owning_slot_id;
  out_result->key_slot_generation = key_view.owning_slot_generation;
  out_result->score_slot_id = score_slot->slot_id;
  out_result->score_slot_generation = score_slot->generation;
  out_result->query_byte_range = query_bytes;
  out_result->key_byte_range = key_bytes;
  out_result->score_byte_range = score_bytes;
  out_result->buffer_allocation_count =
      state->gemma4e2b_m1_rope_buffer_grow_count - allocations_before;
  out_result->buffer_reuse_count = state->gemma4e2b_m1_rope_buffer_reuse_count - reuses_before;
  out_result->descriptor_update_count = state->gemma4e2b_m1_attention_scores_descriptor_update_count;
  out_result->pipeline_create_count = state->gemma4e2b_m1_attention_scores_pipeline_create_count;
  prom_reactor_runtime_concept_norm_consume(handle);
  score_slot->state = PROM_ASYNC_PHYSICAL_READY;
  return PROM_OK;

score_command_fail:
  score_slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->stage = PROM_STAGE_SUBMIT;
  out_result->detail_code = PROM_M46_DETAIL_COMMAND;
  return PROM_ERROR;
score_submit_fail:
  score_slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->stage = PROM_STAGE_SUBMIT;
  out_result->detail_code = PROM_M46_DETAIL_SUBMIT;
  return PROM_ERROR;
}
