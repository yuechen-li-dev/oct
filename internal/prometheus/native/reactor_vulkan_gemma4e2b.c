#if !defined(_WIN32) && !defined(_POSIX_C_SOURCE)
#define _POSIX_C_SOURCE 200809L
#endif

#include "reactor_vulkan.h"
#include "reactor_shader_registry.h"

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
