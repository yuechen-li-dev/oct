/* M39b fused reduction runtime: plans, execution, diagnostics, CPU
   validation, and the shared Vulkan lifecycle used by transformer recording. */
#include "reactor_vulkan_runtime_internal.h"
#include "reactor_shader_registry.h"
#include "reactor_shader_package.h"
#include "reactor_numerical_research.h"

#include <math.h>
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>

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
      request->elements_per_row > PROM_REDUCTION_FORCE_FUSED_MAX_ELEMENTS_PER_ROW) {
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
  if (plan->strategy == PROM_REDUCTION_STRATEGY_FUSED_SINGLE_WORKGROUP ||
      plan->strategy == PROM_REDUCTION_STRATEGY_PACKED_SHORT_ROWS) {
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

/* The short-row crossover is a fixed RTX 3070 witness fact, not a scheduler:
   packed sum repays its partition overhead only at high row counts, while the
   packed stable-softmax path wins across its supported short-width envelope. */
static uint32_t prom_reduction_select_packed_short(const PrometheusReductionRequest* request) {
  if (request->operation == PROM_REDUCTION_OPERATION_SOFTMAX) {
    return request->flags == 0u && request->elements_per_row <= PROM_REDUCTION_PACKED_SHORT_WIDTH_MAX;
  }
  if (request->operation == PROM_REDUCTION_OPERATION_SUM) {
    if (request->elements_per_row <= 96u && request->row_count >= PROM_REDUCTION_PACKED_SHORT_SUM_MIN_ROWS) return 1u;
    return request->elements_per_row <= PROM_REDUCTION_PACKED_SHORT_WIDTH_MAX &&
           request->row_count >= PROM_REDUCTION_PACKED_SHORT_SUM_WIDE_MIN_ROWS;
  }
  return 0u;
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
    const uint32_t packed_short = prom_reduction_select_packed_short(request);
    out_plan->strategy = packed_short != 0u ? PROM_REDUCTION_STRATEGY_PACKED_SHORT_ROWS
                                            : PROM_REDUCTION_STRATEGY_COMPOSED;
    if (packed_short != 0u) {
      prom_reduction_add_stage(out_plan, PROM_REDUCTION_STAGE_ROW_SUM_PACKED_SHORT,
                               PROM_REDUCTION_SHADER_ROW_SUM_PACKED_SHORT,
                               PROM_REDUCTION_IMPLEMENTATION_ROW_SUM_PACKED_SHORT,
                               prom_reduction_ceil_div_u32(request->row_count,
                                                           PROM_REDUCTION_PACKED_SHORT_ROWS_PER_GROUP),
                               request->elements_per_row, 1u);
    } else {
      prom_reduction_add_stage(out_plan, role, shader, implementation, request->row_count * partial_count,
                               request->elements_per_row, partial_count);
    }
    if (partial_count > 1u) {
      prom_reduction_add_stage(out_plan, role, shader, implementation, request->row_count, partial_count, 1u);
    }
  } else {
    composed = (request->flags & PROM_REDUCTION_FLAG_FORCE_COMPOSED) != 0u ||
               ((request->flags & PROM_REDUCTION_FLAG_FORCE_FUSED) == 0u &&
                request->elements_per_row > PROM_REDUCTION_SINGLE_STAGE_THRESHOLD);
    const uint32_t packed_short = prom_reduction_select_packed_short(request);
    if (packed_short != 0u) {
      out_plan->strategy = PROM_REDUCTION_STRATEGY_PACKED_SHORT_ROWS;
      prom_reduction_add_stage(out_plan, PROM_REDUCTION_STAGE_SOFTMAX_PACKED_SHORT,
                               PROM_REDUCTION_SHADER_SOFTMAX_PACKED_SHORT,
                               PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_PACKED_SHORT,
                               prom_reduction_ceil_div_u32(request->row_count,
                                                           PROM_REDUCTION_PACKED_SHORT_ROWS_PER_GROUP),
                               request->elements_per_row, 1u);
    } else if (!composed) {
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

void prom_reduction_destroy_pipeline(VkDevice device, prom_reduction_pipeline* pipeline) {
  if (pipeline->pipeline != VK_NULL_HANDLE) vkDestroyPipeline(device, pipeline->pipeline, NULL);
  if (pipeline->shader_module != VK_NULL_HANDLE) vkDestroyShaderModule(device, pipeline->shader_module, NULL);
  memset(pipeline, 0, sizeof(*pipeline));
}

static void prom_transformer_destroy_parameter(VkDevice device,
                                               prom_transformer_parameter_resource* resource) {
  prom_vk_destroy_buffer(device, &resource->f16);
  prom_vk_destroy_buffer(device, &resource->f32);
  prom_vk_destroy_buffer(device, &resource->upload);
  memset(resource, 0, sizeof(*resource));
}

static void prom_transformer_destroy_layer(VkDevice device,
                                           prom_transformer_layer_resources* layer) {
  uint32_t head;
  uint32_t weight;
  for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
    for (weight = 0u; weight < PROM_M43_WEIGHT_KIND_COUNT; ++weight)
      prom_transformer_destroy_parameter(device, &layer->attention[head][weight]);
  }
  prom_transformer_destroy_parameter(device, &layer->wo);
  prom_transformer_destroy_parameter(device, &layer->rmsnorm);
  for (weight = 0u; weight < PROM_M47_WEIGHT_COUNT; ++weight)
    prom_transformer_destroy_parameter(device, &layer->ffn[weight]);
  memset(layer, 0, sizeof(*layer));
}

void prom_reactor_runtime_reduction_cleanup_state(void* opaque_state, VkDevice device) {
  prom_reduction_runtime_state* state = (prom_reduction_runtime_state*)opaque_state;
  uint32_t slot_index;
  uint32_t pipeline_index;
  VkCommandBuffer command_buffers[PROM_REDUCTION_RING_MAX_DEPTH *
                                  (2u + PROM_M48_COMMAND_BUFFER_COUNT)];
  uint32_t command_count = 0u;
  if (state == NULL) return;
  if (device == VK_NULL_HANDLE) device = state->device;
  prom_model_block_cleanup_state(state);
  prom_compiled_model_session_cleanup_state(state);
  for (slot_index = 0u; slot_index < PROM_REDUCTION_RING_MAX_DEPTH; ++slot_index) {
    prom_reduction_slot* slot = &state->slots[slot_index];
    uint32_t m43_head_index;
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
    prom_vk_destroy_buffer(device, &slot->m43_readback);
    for (m43_head_index = 0u; m43_head_index < PROM_M43_HEAD_COUNT; ++m43_head_index) {
      prom_m43_head_slot* head = &slot->m43_head[m43_head_index];
      prom_vk_destroy_buffer(device, &head->output);
      prom_vk_destroy_buffer(device, &head->p_packed);
      prom_vk_destroy_buffer(device, &head->probabilities);
      prom_vk_destroy_buffer(device, &head->scores);
      prom_vk_destroy_buffer(device, &head->v_packed);
      prom_vk_destroy_buffer(device, &head->k_transposed);
      prom_vk_destroy_buffer(device, &head->q_packed);
      prom_vk_destroy_buffer(device, &head->v);
      prom_vk_destroy_buffer(device, &head->k);
      prom_vk_destroy_buffer(device, &head->q);
    }
    prom_vk_destroy_buffer(device, &slot->m43_x_f16);
    prom_vk_destroy_buffer(device, &slot->m43_x_f32);
    prom_vk_destroy_buffer(device, &slot->m43_x_upload);
    prom_vk_destroy_buffer(device, &slot->m47_readback);
    prom_vk_destroy_buffer(device, &slot->m47_output);
    prom_vk_destroy_buffer(device, &slot->m47_down);
    prom_vk_destroy_buffer(device, &slot->m47_hidden_packed);
    prom_vk_destroy_buffer(device, &slot->m47_hidden);
    prom_vk_destroy_buffer(device, &slot->m47_activated_gate);
    prom_vk_destroy_buffer(device, &slot->m47_up);
    prom_vk_destroy_buffer(device, &slot->m47_gate);
    prom_vk_destroy_buffer(device, &slot->m47_n_packed);
    prom_vk_destroy_buffer(device, &slot->m46_readback);
    prom_vk_destroy_buffer(device, &slot->gemma4e2b_m1_rope_readback);
    prom_vk_destroy_buffer(device, &slot->gemma4e2b_m1_rope_output);
    prom_vk_destroy_buffer(device, &slot->gemma4e2b_m1_bf16_roundtrip);
    prom_vk_destroy_buffer(device, &slot->m49a_m46_z);
    prom_vk_destroy_buffer(device, &slot->m46_output);
    prom_vk_destroy_buffer(device, &slot->m46_inv_rms);
    prom_vk_destroy_buffer(device, &slot->m46_partials);
    prom_vk_destroy_buffer(device, &slot->m45_x_readback);
    prom_vk_destroy_buffer(device, &slot->m45_output);
    prom_vk_destroy_buffer(device, &slot->m44_readback);
    prom_vk_destroy_buffer(device, &slot->m44_output);
    prom_vk_destroy_buffer(device, &slot->m44_concat_f16);
    prom_vk_destroy_buffer(device, &slot->m44_concat_f32);
    prom_vk_destroy_buffer(device, &slot->m44_concat_upload);
    prom_vk_destroy_buffer(device, &slot->m48_readback);
    prom_vk_destroy_buffer(device, &slot->m49b_canary_readback);
    prom_vk_destroy_buffer(device, &slot->m48_activation[1u]);
    prom_vk_destroy_buffer(device, &slot->m48_activation[0u]);
    prom_vk_destroy_buffer(device, &slot->m48_host_initial);
    prom_vk_destroy_buffer(device, &slot->m48_host_initial_upload);
    if (slot->producer_complete != VK_NULL_HANDLE) vkDestroySemaphore(device, slot->producer_complete, NULL);
    for (m43_head_index = 0u; m43_head_index < PROM_M48_SEMAPHORE_COUNT; ++m43_head_index) {
      if (slot->m48_semaphores[m43_head_index] != VK_NULL_HANDLE)
        vkDestroySemaphore(device, slot->m48_semaphores[m43_head_index], NULL);
    }
    if (slot->fence != VK_NULL_HANDLE) vkDestroyFence(device, slot->fence, NULL);
    if (slot->command_buffer != VK_NULL_HANDLE) command_buffers[command_count++] = slot->command_buffer;
    if (slot->consumer_command_buffer != VK_NULL_HANDLE) command_buffers[command_count++] = slot->consumer_command_buffer;
    for (m43_head_index = 0u; m43_head_index < PROM_M48_COMMAND_BUFFER_COUNT; ++m43_head_index) {
      if (slot->m48_command_buffers[m43_head_index] != VK_NULL_HANDLE)
        command_buffers[command_count++] = slot->m48_command_buffers[m43_head_index];
    }
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
  for (pipeline_index = 0u; pipeline_index < PROM_M46_PIPELINE_COUNT; ++pipeline_index) {
    prom_reduction_destroy_pipeline(device, &state->m46_pipelines[pipeline_index]);
  }
  prom_reduction_destroy_pipeline(device, &state->gemma4e2b_m1_bf16_roundtrip_pipeline);
  prom_reduction_destroy_pipeline(device, &state->gemma4e2b_m1_rope_pipeline);
  prom_reduction_destroy_pipeline(device, &state->gemma4e2b_m1_attention_scores_pipeline);
  for (pipeline_index = 0u; pipeline_index < PROM_M47_PIPELINE_COUNT; ++pipeline_index) {
    prom_reduction_destroy_pipeline(device, &state->m47_pipelines[pipeline_index]);
  }
  for (pipeline_index = 0u; pipeline_index < PROM_M45_PIPELINE_COUNT; ++pipeline_index) {
    prom_reduction_destroy_pipeline(device, &state->m45_pipelines[pipeline_index]);
  }
  for (pipeline_index = 0u; pipeline_index < PROM_M44_PIPELINE_COUNT; ++pipeline_index) {
    prom_reduction_destroy_pipeline(device, &state->m44_pipelines[pipeline_index]);
  }
  prom_vk_destroy_buffer(device, &state->gemma4e2b_m1_rope_sine);
  prom_vk_destroy_buffer(device, &state->gemma4e2b_m1_rope_sine_upload);
  prom_vk_destroy_buffer(device, &state->gemma4e2b_m1_rope_cosine);
  prom_vk_destroy_buffer(device, &state->gemma4e2b_m1_rope_cosine_upload);
  for (pipeline_index = 0u; pipeline_index < 3u; ++pipeline_index) {
    prom_vk_destroy_buffer(device, &state->m42_weight_f16[pipeline_index]);
    prom_vk_destroy_buffer(device, &state->m42_weight_f32[pipeline_index]);
    prom_vk_destroy_buffer(device, &state->m42_weight_upload[pipeline_index]);
  }
  prom_vk_destroy_buffer(device, &state->m42_resident_x_f16);
  prom_vk_destroy_buffer(device, &state->m42_resident_x_f32);
  prom_vk_destroy_buffer(device, &state->m42_resident_x_upload);
  for (slot_index = 0u; slot_index < PROM_M43_HEAD_COUNT; ++slot_index) {
    for (pipeline_index = 0u; pipeline_index < PROM_M43_WEIGHT_KIND_COUNT; ++pipeline_index) {
      prom_vk_destroy_buffer(device, &state->m43_weight_f16[slot_index][pipeline_index]);
      prom_vk_destroy_buffer(device, &state->m43_weight_f32[slot_index][pipeline_index]);
      prom_vk_destroy_buffer(device, &state->m43_weight_upload[slot_index][pipeline_index]);
    }
  }
  prom_vk_destroy_buffer(device, &state->m43_resident_x_f16);
  prom_vk_destroy_buffer(device, &state->m43_resident_x_f32);
  prom_vk_destroy_buffer(device, &state->m43_resident_x_upload);
  prom_vk_destroy_buffer(device, &state->m46_weight);
  prom_vk_destroy_buffer(device, &state->m46_weight_upload);
  for (pipeline_index = 0u; pipeline_index < PROM_M47_WEIGHT_COUNT; ++pipeline_index) {
    prom_vk_destroy_buffer(device, &state->m47_weight_f16[pipeline_index]);
    prom_vk_destroy_buffer(device, &state->m47_weight_f32[pipeline_index]);
    prom_vk_destroy_buffer(device, &state->m47_weight_upload[pipeline_index]);
  }
  prom_vk_destroy_buffer(device, &state->m44_wo_f16);
  prom_vk_destroy_buffer(device, &state->m44_wo_f32);
  prom_vk_destroy_buffer(device, &state->m44_wo_upload);
  for (slot_index = 0u; slot_index < PROM_M48_LAYER_COUNT; ++slot_index)
    prom_transformer_destroy_layer(device, &state->m48_layer[slot_index]);
  prom_vk_destroy_buffer(device, &state->m48_initial_f32);
  prom_vk_destroy_buffer(device, &state->m48_initial_upload);
  prom_vk_destroy_buffer(device, &state->resident_a);
  prom_vk_destroy_buffer(device, &state->resident_a_upload);
  prom_vk_destroy_buffer(device, &state->persistent_b);
  prom_vk_destroy_buffer(device, &state->persistent_b_upload);
  if (state->query_pool != VK_NULL_HANDLE) vkDestroyQueryPool(device, state->query_pool, NULL);
  if (state->m44_pipeline_layout != VK_NULL_HANDLE)
    vkDestroyPipelineLayout(device, state->m44_pipeline_layout, NULL);
  if (state->m44_descriptor_pool != VK_NULL_HANDLE)
    vkDestroyDescriptorPool(device, state->m44_descriptor_pool, NULL);
  if (state->m44_descriptor_set_layout != VK_NULL_HANDLE)
    vkDestroyDescriptorSetLayout(device, state->m44_descriptor_set_layout, NULL);
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
      PROM_REDUCTION_IMPLEMENTATION_ROW_SUM_PACKED_SHORT,
      PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_PACKED_SHORT,
  };
  uint32_t index;
  for (index = 0u; index < PROM_REDUCTION_PIPELINE_COUNT; ++index) {
    const prom_compute_implementation* implementation =
        prom_shader_registry_find_compute_implementation(implementation_ids[index]);
    const prom_shader_asset* asset;
    char variant_id[32];
    const char* entry_point = NULL;
    prom_shader_package_diagnostic package_diagnostic;
    VkPipelineShaderStageCreateInfo stage_info;
    VkComputePipelineCreateInfo pipeline_info;
    VkResult result;
    if (implementation == NULL || implementation->reduction_dispatch == NULL || implementation->dispatchable == 0u) return 0;
    asset = prom_shader_registry_find_shader(implementation->shader_id);
    if (asset == NULL || state->shader_package == NULL ||
        asset->entry_point == NULL || asset->descriptor_binding_count != 4u || asset->push_constant_bytes != 32u) return 0;
    if (snprintf(variant_id, sizeof(variant_id), "kernel-%u-default", asset->shader_id) < 0 ||
        !prom_shader_package_create_module(state->shader_package, state->device, variant_id,
                                           &state->pipelines[index].shader_module, &entry_point,
                                           &package_diagnostic)) return 0;
    memset(&stage_info, 0, sizeof(stage_info));
    stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
    stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
    stage_info.module = state->pipelines[index].shader_module;
    stage_info.pName = entry_point;
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
                                           prom_shader_package* shader_package,
                                           prom_reduction_runtime_state** out_state) {
  prom_reduction_runtime_state* state;
  VkDescriptorSetLayoutBinding bindings[4];
  VkDescriptorSetLayoutBinding m44_bindings[PROM_M44_WIDE_DESCRIPTOR_BINDING_COUNT];
  VkDescriptorSetLayoutCreateInfo layout_info;
  VkDescriptorPoolSize pool_size;
  VkDescriptorPoolCreateInfo pool_info;
  VkDescriptorPoolSize m44_pool_size;
  VkDescriptorPoolCreateInfo m44_pool_info;
  VkPushConstantRange push_range;
  VkPipelineLayoutCreateInfo pipeline_layout_info;
  VkDescriptorSetLayout layouts[PROM_REDUCTION_RING_MAX_DEPTH *
                                PROM_ALL_STANDARD_DESCRIPTOR_SETS_PER_SLOT];
  VkDescriptorSet descriptor_sets[PROM_REDUCTION_RING_MAX_DEPTH *
                                  PROM_ALL_STANDARD_DESCRIPTOR_SETS_PER_SLOT];
  VkDescriptorSetLayout m44_layouts[PROM_REDUCTION_RING_MAX_DEPTH *
                                    PROM_ALL_WIDE_DESCRIPTOR_SETS_PER_SLOT];
  VkDescriptorSet m44_descriptor_sets[PROM_REDUCTION_RING_MAX_DEPTH *
                                      PROM_ALL_WIDE_DESCRIPTOR_SETS_PER_SLOT];
  VkDescriptorSetAllocateInfo descriptor_allocate_info;
  VkCommandBuffer command_buffers[PROM_REDUCTION_RING_MAX_DEPTH *
                                  (2u + PROM_M48_COMMAND_BUFFER_COUNT)];
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
  state->shader_package = shader_package;
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
  for (slot_index = 0u; slot_index < PROM_M48_LAYER_COUNT; ++slot_index)
    state->m48_layer[slot_index].layer_index = slot_index;

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
                              PROM_ALL_STANDARD_DESCRIPTOR_SETS_PER_SLOT;
  memset(&pool_info, 0, sizeof(pool_info));
  pool_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO;
  pool_info.maxSets = state->ring_depth * PROM_ALL_STANDARD_DESCRIPTOR_SETS_PER_SLOT;
  pool_info.poolSizeCount = 1u;
  pool_info.pPoolSizes = &pool_size;
  result = vkCreateDescriptorPool(state->device, &pool_info, NULL, &state->descriptor_pool);
  if (result != VK_SUCCESS) goto fail;
  for (descriptor_index = 0u;
       descriptor_index < state->ring_depth * PROM_ALL_STANDARD_DESCRIPTOR_SETS_PER_SLOT;
       ++descriptor_index) {
    layouts[descriptor_index] = state->descriptor_set_layout;
  }
  memset(&descriptor_allocate_info, 0, sizeof(descriptor_allocate_info));
  descriptor_allocate_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO;
  descriptor_allocate_info.descriptorPool = state->descriptor_pool;
  descriptor_allocate_info.descriptorSetCount =
      state->ring_depth * PROM_ALL_STANDARD_DESCRIPTOR_SETS_PER_SLOT;
  descriptor_allocate_info.pSetLayouts = layouts;
  result = vkAllocateDescriptorSets(state->device, &descriptor_allocate_info, descriptor_sets);
  if (result != VK_SUCCESS) goto fail;

  memset(m44_bindings, 0, sizeof(m44_bindings));
  for (descriptor_index = 0u; descriptor_index < PROM_M44_WIDE_DESCRIPTOR_BINDING_COUNT;
       ++descriptor_index) {
    m44_bindings[descriptor_index].binding = descriptor_index;
    m44_bindings[descriptor_index].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
    m44_bindings[descriptor_index].descriptorCount = 1u;
    m44_bindings[descriptor_index].stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
  }
  memset(&layout_info, 0, sizeof(layout_info));
  layout_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO;
  layout_info.bindingCount = PROM_M44_WIDE_DESCRIPTOR_BINDING_COUNT;
  layout_info.pBindings = m44_bindings;
  result = vkCreateDescriptorSetLayout(state->device, &layout_info, NULL,
                                       &state->m44_descriptor_set_layout);
  if (result != VK_SUCCESS) goto fail;
  memset(&pipeline_layout_info, 0, sizeof(pipeline_layout_info));
  pipeline_layout_info.sType = VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO;
  pipeline_layout_info.setLayoutCount = 1u;
  pipeline_layout_info.pSetLayouts = &state->m44_descriptor_set_layout;
  pipeline_layout_info.pushConstantRangeCount = 1u;
  pipeline_layout_info.pPushConstantRanges = &push_range;
  result = vkCreatePipelineLayout(state->device, &pipeline_layout_info, NULL,
                                  &state->m44_pipeline_layout);
  if (result != VK_SUCCESS) goto fail;
  m44_pool_size.type = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  m44_pool_size.descriptorCount = PROM_M44_WIDE_DESCRIPTOR_BINDING_COUNT * state->ring_depth *
                                  PROM_ALL_WIDE_DESCRIPTOR_SETS_PER_SLOT;
  memset(&m44_pool_info, 0, sizeof(m44_pool_info));
  m44_pool_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO;
  m44_pool_info.maxSets = state->ring_depth * PROM_ALL_WIDE_DESCRIPTOR_SETS_PER_SLOT;
  m44_pool_info.poolSizeCount = 1u;
  m44_pool_info.pPoolSizes = &m44_pool_size;
  result = vkCreateDescriptorPool(state->device, &m44_pool_info, NULL, &state->m44_descriptor_pool);
  if (result != VK_SUCCESS) goto fail;
  for (descriptor_index = 0u;
       descriptor_index < state->ring_depth * PROM_ALL_WIDE_DESCRIPTOR_SETS_PER_SLOT;
       ++descriptor_index)
    m44_layouts[descriptor_index] = state->m44_descriptor_set_layout;
  memset(&descriptor_allocate_info, 0, sizeof(descriptor_allocate_info));
  descriptor_allocate_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO;
  descriptor_allocate_info.descriptorPool = state->m44_descriptor_pool;
  descriptor_allocate_info.descriptorSetCount =
      state->ring_depth * PROM_ALL_WIDE_DESCRIPTOR_SETS_PER_SLOT;
  descriptor_allocate_info.pSetLayouts = m44_layouts;
  result = vkAllocateDescriptorSets(state->device, &descriptor_allocate_info, m44_descriptor_sets);
  if (result != VK_SUCCESS) goto fail;

  memset(&command_allocate_info, 0, sizeof(command_allocate_info));
  command_allocate_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO;
  command_allocate_info.commandPool = state->command_pool;
  command_allocate_info.level = VK_COMMAND_BUFFER_LEVEL_PRIMARY;
  command_allocate_info.commandBufferCount =
      state->ring_depth * (2u + PROM_M48_COMMAND_BUFFER_COUNT);
  result = vkAllocateCommandBuffers(state->device, &command_allocate_info, command_buffers);
  if (result != VK_SUCCESS) goto fail;
  memset(&fence_info, 0, sizeof(fence_info));
  fence_info.sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO;
  memset(&semaphore_info, 0, sizeof(semaphore_info));
  semaphore_info.sType = VK_STRUCTURE_TYPE_SEMAPHORE_CREATE_INFO;
  for (slot_index = 0u; slot_index < state->ring_depth; ++slot_index) {
    uint32_t stage_index;
    uint32_t layer_index;
    const uint32_t standard_base = slot_index * PROM_ALL_STANDARD_DESCRIPTOR_SETS_PER_SLOT;
    const uint32_t command_base = slot_index * (2u + PROM_M48_COMMAND_BUFFER_COUNT);
    const uint32_t wide_base = slot_index * PROM_ALL_WIDE_DESCRIPTOR_SETS_PER_SLOT;
    prom_reduction_slot* slot = &state->slots[slot_index];
    slot->slot_id = slot_index;
    slot->active_query_base = slot_index * PROM_REDUCTION_QUERY_STRIDE;
    slot->state = PROM_ASYNC_PHYSICAL_EMPTY;
    slot->command_buffer = command_buffers[command_base];
    slot->consumer_command_buffer = command_buffers[command_base + 1u];
    slot->m44_descriptor_set = m44_descriptor_sets[wide_base];
    for (layer_index = 0u; layer_index < PROM_M48_COMMAND_BUFFER_COUNT; ++layer_index)
      slot->m48_command_buffers[layer_index] = command_buffers[command_base + 2u + layer_index];
    for (stage_index = 0u; stage_index < PROM_REDUCTION_MAX_STAGES; ++stage_index) {
      slot->descriptor_sets[stage_index] =
          descriptor_sets[standard_base + stage_index];
    }
    for (stage_index = 0u; stage_index < PROM_M42_DESCRIPTOR_SET_COUNT; ++stage_index) {
      slot->m42_descriptor_sets[stage_index] =
          descriptor_sets[standard_base +
                          PROM_REDUCTION_MAX_STAGES + stage_index];
    }
    for (stage_index = 0u; stage_index < PROM_M43_DESCRIPTOR_SET_COUNT; ++stage_index) {
      slot->m43_descriptor_sets[stage_index] =
          descriptor_sets[standard_base +
                          PROM_REDUCTION_MAX_STAGES + PROM_M42_DESCRIPTOR_SET_COUNT + stage_index];
    }
    slot->m44_sgemm_descriptor_set =
        descriptor_sets[standard_base +
                        PROM_REDUCTION_MAX_STAGES + PROM_M42_DESCRIPTOR_SET_COUNT +
                        PROM_M43_DESCRIPTOR_SET_COUNT];
    slot->m45_descriptor_set =
        descriptor_sets[standard_base +
                        PROM_REDUCTION_MAX_STAGES + PROM_M42_DESCRIPTOR_SET_COUNT +
                        PROM_M43_DESCRIPTOR_SET_COUNT + 1u];
    for (stage_index = 0u; stage_index < PROM_M47_DESCRIPTOR_SET_COUNT; ++stage_index) {
      slot->m47_descriptor_sets[stage_index] =
          descriptor_sets[standard_base +
                          PROM_REDUCTION_MAX_STAGES + PROM_M42_DESCRIPTOR_SET_COUNT +
                          PROM_M43_DESCRIPTOR_SET_COUNT + PROM_M44_COMMON_DESCRIPTOR_SET_COUNT +
                          stage_index];
    }
    {
      uint32_t next = standard_base + PROM_STANDARD_DESCRIPTOR_SETS_PER_SLOT;
      for (layer_index = 0u; layer_index < PROM_M48_LAYER_COUNT; ++layer_index) {
        prom_transformer_descriptor_bank* bank = &slot->m48_descriptors[layer_index];
        for (stage_index = 0u; stage_index < PROM_M43_DESCRIPTOR_SET_COUNT; ++stage_index)
          bank->m43[stage_index] = descriptor_sets[next++];
        bank->m44_sgemm = descriptor_sets[next++];
        bank->m45 = descriptor_sets[next++];
        for (stage_index = 0u; stage_index < 3u; ++stage_index)
          bank->m46[stage_index] = descriptor_sets[next++];
        for (stage_index = 0u; stage_index < PROM_M47_DESCRIPTOR_SET_COUNT; ++stage_index)
          bank->m47[stage_index] = descriptor_sets[next++];
        bank->m44_wide = m44_descriptor_sets[wide_base + 1u + layer_index];
      }
    }
    result = vkCreateFence(state->device, &fence_info, NULL, &slot->fence);
    if (result != VK_SUCCESS) goto fail;
    result = vkCreateSemaphore(state->device, &semaphore_info, NULL, &slot->producer_complete);
    if (result != VK_SUCCESS) goto fail;
    for (layer_index = 0u; layer_index < PROM_M48_SEMAPHORE_COUNT; ++layer_index) {
      result = vkCreateSemaphore(state->device, &semaphore_info, NULL,
                                 &slot->m48_semaphores[layer_index]);
      if (result != VK_SUCCESS) goto fail;
    }
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

prom_reduction_runtime_state* prom_reduction_ensure_state(void* handle, int32_t* out_detail) {
  prom_reduction_runtime_state* state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  prom_vk_runtime_services services;
  prom_shader_package* shader_package = NULL;
  if (state != NULL) return state;
  if (prom_reactor_runtime_get_vk_services(handle, &services) != PROM_OK) {
    if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_RUNTIME_UNAVAILABLE;
    return NULL;
  }
  if (prom_reactor_runtime_get_shader_package(handle, &shader_package) != PROM_OK ||
      shader_package == NULL || !prom_shader_registry_validate() ||
      !prom_reduction_initialize_state(&services, shader_package, &state)) {
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

int prom_reduction_ensure_buffer(prom_reduction_runtime_state* state,
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

void prom_reduction_reap_slots(prom_reduction_runtime_state* state, uint32_t allow_wait) {
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

prom_reduction_slot* prom_reduction_acquire_slot(prom_reduction_runtime_state* state,
                                                        uint64_t logical_request_id) {
  uint32_t offset;
  prom_reduction_reap_slots(state, 0u);
  for (offset = 0u; offset < state->ring_depth; ++offset) {
    uint32_t index = (state->acquire_cursor + offset) % state->ring_depth;
    prom_reduction_slot* slot = &state->slots[index];
    if ((state->gemma4e2b_m1_rope_q_valid != 0u &&
         slot->slot_id == state->gemma4e2b_m1_rope_q_slot_id &&
         slot->generation == state->gemma4e2b_m1_rope_q_slot_generation) ||
        (state->gemma4e2b_m1_rope_k_valid != 0u &&
         slot->slot_id == state->gemma4e2b_m1_rope_k_slot_id &&
         slot->generation == state->gemma4e2b_m1_rope_k_slot_generation)) continue;
    if (slot->state == PROM_ASYNC_PHYSICAL_READY) {
      slot->state = PROM_ASYNC_PHYSICAL_EMPTY;
      state->diagnostics.physical_recycle_count += 1u;
    }
    if (slot->state == PROM_ASYNC_PHYSICAL_EMPTY) {
      slot->generation += 1u;
      slot->logical_request_id = logical_request_id;
      slot->active_query_base = slot->slot_id * PROM_REDUCTION_QUERY_STRIDE;
      slot->state = PROM_ASYNC_PHYSICAL_PREPARING;
      state->acquire_cursor = (index + 1u) % state->ring_depth;
      state->diagnostics.acquire_cursor = state->acquire_cursor;
      return slot;
    }
  }
  prom_reduction_reap_slots(state, 1u);
  for (offset = 0u; offset < state->ring_depth; ++offset) {
    prom_reduction_slot* slot = &state->slots[offset];
    if ((state->gemma4e2b_m1_rope_q_valid != 0u &&
         slot->slot_id == state->gemma4e2b_m1_rope_q_slot_id &&
         slot->generation == state->gemma4e2b_m1_rope_q_slot_generation) ||
        (state->gemma4e2b_m1_rope_k_valid != 0u &&
         slot->slot_id == state->gemma4e2b_m1_rope_k_slot_id &&
         slot->generation == state->gemma4e2b_m1_rope_k_slot_generation)) continue;
    if (slot->state == PROM_ASYNC_PHYSICAL_READY) {
      slot->state = PROM_ASYNC_PHYSICAL_PREPARING;
      slot->generation += 1u;
      slot->logical_request_id = logical_request_id;
      slot->active_query_base = slot->slot_id * PROM_REDUCTION_QUERY_STRIDE;
      state->diagnostics.physical_recycle_count += 1u;
      state->acquire_cursor = (offset + 1u) % state->ring_depth;
      state->diagnostics.acquire_cursor = state->acquire_cursor;
      return slot;
    }
  }
  return NULL;
}

VkPipeline prom_reduction_pipeline_for_implementation(const prom_reduction_runtime_state* state,
                                                             uint32_t implementation_id) {
  uint32_t index;
  for (index = 0u; index < PROM_REDUCTION_PIPELINE_COUNT; ++index) {
    if (state->pipelines[index].implementation_id == implementation_id) return state->pipelines[index].pipeline;
  }
  return VK_NULL_HANDLE;
}

void prom_reduction_stage_bindings_for_io(const prom_reduction_slot* slot,
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
  if (plan->strategy == PROM_REDUCTION_STRATEGY_FUSED_SINGLE_WORKGROUP ||
      plan->strategy == PROM_REDUCTION_STRATEGY_PACKED_SHORT_ROWS) return;
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

void prom_reduction_update_descriptor_set(prom_reduction_runtime_state* state,
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

void prom_reduction_record_barrier(VkCommandBuffer command_buffer) {
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

int prom_m40b_wait_all_slots(prom_reduction_runtime_state* state) {
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

int prom_m40b_pack_matrix(const float* values,
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

int prom_m40b_ensure_sgemm_pipeline(prom_reduction_runtime_state* state, uint32_t kernel) {
  prom_reduction_pipeline* destination;
  const char* entry = NULL;
  const prom_shader_asset* asset = NULL;
  const char* variant_id = NULL;
  prom_shader_package_diagnostic package_diagnostic;
  VkPipelineShaderStageCreateInfo stage_info;
  VkComputePipelineCreateInfo pipeline_info;
  VkResult result;
  if (kernel < PROM_M40B_KERNEL_COOPERATIVE || kernel > PROM_M40B_KERNEL_CONVENTIONAL_FP16) return 0;
  destination = &state->m40b_sgemm_pipelines[kernel - 1u];
  if (destination->pipeline != VK_NULL_HANDLE) return 1;
  if (kernel == PROM_M40B_KERNEL_COOPERATIVE) {
    variant_id = "kernel-56-default";
  } else {
    asset = prom_shader_registry_find_shader(kernel == PROM_M40B_KERNEL_A2X4 ? 12u : 14u);
    if (asset == NULL) return 0;
    variant_id = kernel == PROM_M40B_KERNEL_A2X4 ? "kernel-12-default" : "kernel-14-default";
  }
  if (state->shader_package == NULL ||
      !prom_shader_package_create_module(state->shader_package, state->device, variant_id,
                                         &destination->shader_module, &entry, &package_diagnostic)) return 0;
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
  destination->shader_id = kernel == PROM_M40B_KERNEL_COOPERATIVE ? 56u : asset->shader_id;
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

static void prom_reduction_result_failure(PrometheusReductionExecutionResult* result,
                                          uint32_t stage,
                                          int32_t detail) {
  result->stage = stage;
  result->detail_code = detail;
}

int prom_reduction_find_nonfinite(const float* input, uint64_t count, uint64_t* out_index) {
  uint64_t index;
  for (index = 0u; index < count; ++index) {
    if (!isfinite(input[index])) {
      if (out_index != NULL) *out_index = index;
      return 1;
    }
  }
  return 0;
}

static int prom_softmax_partial_overlap(const float* input, float* output, uint64_t element_count) {
  const uintptr_t input_begin = (uintptr_t)input;
  const uintptr_t output_begin = (uintptr_t)output;
  const uint64_t byte_count = element_count * sizeof(float);
  uintptr_t input_end;
  uintptr_t output_end;
  if (input == output) return 0;
  if (byte_count > (uint64_t)UINTPTR_MAX ||
      input_begin > UINTPTR_MAX - (uintptr_t)byte_count ||
      output_begin > UINTPTR_MAX - (uintptr_t)byte_count) return 1;
  input_end = input_begin + (uintptr_t)byte_count;
  output_end = output_begin + (uintptr_t)byte_count;
  return input_begin < output_end && output_begin < input_end;
}

static void prom_softmax_result_failure(PrometheusRowWiseSoftmaxResult* result,
                                        uint32_t stage, int32_t detail) {
  result->stage = stage;
  result->detail_code = detail;
  result->output_written = 0u;
}

int prom_reactor_runtime_row_wise_softmax_impl(
    void* handle, const PrometheusRowWiseSoftmaxRequest* request,
    PrometheusRowWiseSoftmaxResult* out_result) {
  PrometheusRowWiseSoftmaxResult local_result;
  PrometheusRowWiseSoftmaxResult* result = out_result != NULL ? out_result : &local_result;
  PrometheusReductionRequest reduction_request;
  PrometheusReductionExecutionResult reduction_result;
  prom_vk_runtime_services services;
  uint64_t total_elements;
  int status;

  memset(result, 0, sizeof(*result));
  result->struct_size = sizeof(*result);
  result->first_nonfinite_index = UINT64_MAX;
  if (request == NULL || request->struct_size < sizeof(*request)) {
    prom_softmax_result_failure(result, PROM_STAGE_INIT, PROM_SOFTMAX_DETAIL_INVALID_REQUEST);
    return PROM_ERROR;
  }
  total_elements = (uint64_t)request->row_count * (uint64_t)request->elements_per_row;
  if (request->input_element_count != total_elements || request->output_element_count != total_elements) {
    prom_softmax_result_failure(result, PROM_STAGE_INIT, PROM_SOFTMAX_DETAIL_COUNT_MISMATCH);
    return PROM_ERROR;
  }
  if (total_elements == 0u) {
    result->stage = PROM_STAGE_NONE;
    result->detail_code = 0;
    return PROM_OK;
  }
  if (request->input == NULL) {
    prom_softmax_result_failure(result, PROM_STAGE_INIT, PROM_SOFTMAX_DETAIL_NULL_INPUT);
    return PROM_ERROR;
  }
  if (request->output == NULL) {
    prom_softmax_result_failure(result, PROM_STAGE_INIT, PROM_SOFTMAX_DETAIL_NULL_OUTPUT);
    return PROM_ERROR;
  }
  if (request->row_count > PROM_ROW_WISE_SOFTMAX_MAX_ROWS) {
    prom_softmax_result_failure(result, PROM_STAGE_INIT, PROM_SOFTMAX_DETAIL_ROW_LIMIT);
    return PROM_ERROR;
  }
  if (request->elements_per_row > PROM_ROW_WISE_SOFTMAX_MAX_ELEMENTS_PER_ROW) {
    prom_softmax_result_failure(result, PROM_STAGE_INIT, PROM_SOFTMAX_DETAIL_WIDTH_LIMIT);
    return PROM_ERROR;
  }
  if (total_elements > PROM_ROW_WISE_SOFTMAX_MAX_TOTAL_ELEMENTS ||
      total_elements > UINT64_MAX / sizeof(float)) {
    prom_softmax_result_failure(result, PROM_STAGE_INIT, PROM_SOFTMAX_DETAIL_ELEMENT_LIMIT);
    return PROM_ERROR;
  }
  if (prom_softmax_partial_overlap(request->input, request->output, total_elements)) {
    prom_softmax_result_failure(result, PROM_STAGE_INIT, PROM_SOFTMAX_DETAIL_PARTIAL_ALIAS);
    return PROM_ERROR;
  }
  if (prom_reduction_find_nonfinite(request->input, total_elements, &result->first_nonfinite_index)) {
    prom_softmax_result_failure(result, PROM_STAGE_TRANSFER_IN, PROM_SOFTMAX_DETAIL_NONFINITE_INPUT);
    return PROM_ERROR;
  }
  if (!prom_reactor_runtime_validate_handle(handle)) {
    prom_softmax_result_failure(result, PROM_STAGE_INIT, PROM_SOFTMAX_DETAIL_RUNTIME_UNAVAILABLE);
    return PROM_INVALID_HANDLE;
  }
  if (prom_reactor_runtime_get_vk_services(handle, &services) != PROM_OK) {
    prom_softmax_result_failure(result, PROM_STAGE_INIT, PROM_SOFTMAX_DETAIL_RUNTIME_UNAVAILABLE);
    return PROM_ERROR;
  }
  if (services.subgroup_compute_supported == 0u || services.subgroup_arithmetic_supported == 0u ||
      services.subgroup_basic_supported == 0u) {
    prom_softmax_result_failure(result, PROM_STAGE_INIT, PROM_SOFTMAX_DETAIL_SUBGROUP_UNSUPPORTED);
    return PROM_ERROR;
  }
  if (services.subgroup_size == 0u || services.subgroup_size > PROM_REDUCTION_LOCAL_SIZE ||
      PROM_REDUCTION_LOCAL_SIZE % services.subgroup_size != 0u) {
    prom_softmax_result_failure(result, PROM_STAGE_INIT, PROM_SOFTMAX_DETAIL_TOPOLOGY_UNSUPPORTED);
    return PROM_ERROR;
  }

  memset(&reduction_request, 0, sizeof(reduction_request));
  reduction_request.struct_size = sizeof(reduction_request);
  reduction_request.input = request->input;
  reduction_request.output = request->output;
  reduction_request.row_count = request->row_count;
  reduction_request.elements_per_row = request->elements_per_row;
  reduction_request.input_element_count = total_elements;
  reduction_request.output_element_count = total_elements;
  reduction_request.operation = PROM_REDUCTION_OPERATION_SOFTMAX;
  reduction_request.finalization = PROM_REDUCTION_FINALIZATION_STABLE_SOFTMAX;
  reduction_request.flags = PROM_REDUCTION_FLAG_FORCE_FUSED;
  memset(&reduction_result, 0, sizeof(reduction_result));
  status = prom_reactor_runtime_reduction_impl(handle, &reduction_request, &reduction_result);
  result->stage = reduction_result.stage;
  result->detail_code = reduction_result.detail_code;
  result->gpu_timestamp_valid = reduction_result.gpu_timestamp_valid;
  result->gpu_duration_ns = reduction_result.gpu_duration_ns;
  result->end_to_end_ns = reduction_result.end_to_end_ns;
  result->first_nonfinite_index = reduction_result.first_nonfinite_index;
  if (status != PROM_OK) return status;
  /* The forced fused plan is structurally one group per row, one command
     dispatch, and one synchronous queue submission for every nonempty call. */
  result->physical_dispatch_count = 1u;
  result->physical_submission_count = 1u;
  result->output_written = 1u;
  return PROM_OK;
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
