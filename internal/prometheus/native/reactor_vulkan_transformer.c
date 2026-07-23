#if !defined(_WIN32) && !defined(_POSIX_C_SOURCE)
#define _POSIX_C_SOURCE 200809L
#endif

#include "reactor_vulkan.h"
#include "reactor_shader_registry.h"
#include "reactor_numerical_research.h"
#include "reactor_vulkan_transformer_control.h"

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

static int prom_m43_checked_add_u64(uint64_t left, uint64_t right, uint64_t* out_value) {
  if (out_value == NULL || left > UINT64_MAX - right) return 0;
  *out_value = left + right;
  return 1;
}

static int prom_m43_checked_scale_u64(uint64_t value, uint64_t scale, uint64_t* out_value) {
  if (out_value == NULL || (scale != 0u && value > UINT64_MAX / scale)) return 0;
  *out_value = value * scale;
  return 1;
}

static int prom_m43_memory_plan_build(uint32_t tokens,
                                      uint32_t model_width,
                                      uint32_t head_dim,
                                      uint32_t padded_tokens,
                                      uint32_t padded_model,
                                      uint32_t padded_head,
                                      prom_m43_memory_plan* out_memory) {
  uint64_t logical_x;
  uint64_t padded_x;
  uint64_t logical_weight;
  uint64_t padded_weight;
  uint64_t padded_q;
  uint64_t padded_scores;
  uint64_t logical_scores;
  uint64_t logical_output;
  uint64_t packed_q_bytes;
  uint64_t compact_k_transpose_bytes;
  uint64_t total = 0u;
  uint64_t value;
  if (out_memory == NULL ||
      !prom_m40b_checked_product_u64(tokens, model_width, &logical_x) ||
      !prom_m40b_checked_product_u64(padded_tokens, padded_model, &padded_x) ||
      !prom_m40b_checked_product_u64(model_width, head_dim, &logical_weight) ||
      !prom_m40b_checked_product_u64(padded_model, padded_head, &padded_weight) ||
      !prom_m40b_checked_product_u64(padded_tokens, padded_head, &padded_q) ||
      !prom_m40b_checked_product_u64(padded_tokens, padded_tokens, &padded_scores) ||
      !prom_m40b_checked_product_u64(tokens, tokens, &logical_scores) ||
      !prom_m40b_checked_product_u64(tokens, head_dim, &logical_output)) return 0;
  memset(out_memory, 0, sizeof(*out_memory));
  out_memory->capacity_limit_bytes = PROM_M43_CAPACITY_LIMIT_BYTES;
  out_memory->shared_x_upload_bytes = logical_x * sizeof(float);
  out_memory->shared_x_f32_bytes = logical_x * sizeof(float);
  out_memory->shared_x_packed_bytes = ((padded_x + 1u) / 2u) * sizeof(uint32_t);
  if (!prom_m43_checked_scale_u64(logical_weight * sizeof(float),
                                  PROM_M43_HEAD_COUNT * PROM_M43_WEIGHT_KIND_COUNT, &value)) return 0;
  out_memory->persistent_weight_upload_bytes = value;
  out_memory->persistent_weight_f32_bytes = value;
  if (!prom_m43_checked_scale_u64(((padded_weight + 1u) / 2u) * sizeof(uint32_t),
                                  PROM_M43_HEAD_COUNT * PROM_M43_WEIGHT_KIND_COUNT, &value)) return 0;
  out_memory->persistent_weight_packed_bytes = value;
  if (!prom_m43_checked_scale_u64(padded_q * sizeof(float),
                                  PROM_M43_HEAD_COUNT * PROM_M43_WEIGHT_KIND_COUNT, &value)) return 0;
  out_memory->qkv_f32_bytes = value;
  packed_q_bytes = ((padded_q + 1u) / 2u) * sizeof(uint32_t);
  compact_k_transpose_bytes = logical_output * sizeof(float);
  if (compact_k_transpose_bytes < packed_q_bytes) compact_k_transpose_bytes = packed_q_bytes;
  if (!prom_m43_checked_scale_u64(2u * packed_q_bytes + compact_k_transpose_bytes,
                                  PROM_M43_HEAD_COUNT, &value)) return 0;
  out_memory->qkv_packed_bytes = value;
  if (!prom_m43_checked_scale_u64(padded_scores * sizeof(float), PROM_M43_HEAD_COUNT, &value)) return 0;
  out_memory->score_bytes = value;
  if (!prom_m43_checked_scale_u64(logical_scores * sizeof(float), PROM_M43_HEAD_COUNT, &value)) return 0;
  out_memory->probability_bytes = value;
  if (!prom_m43_checked_scale_u64(((padded_scores + 1u) / 2u) * sizeof(uint32_t),
                                  PROM_M43_HEAD_COUNT, &value)) return 0;
  out_memory->probability_packed_bytes = value;
  if (!prom_m43_checked_scale_u64(padded_q * sizeof(float), PROM_M43_HEAD_COUNT, &value)) return 0;
  out_memory->output_bytes = value;
  out_memory->reduction_temporary_bytes = 3u * sizeof(float);
  if (!prom_m43_checked_scale_u64(logical_output * sizeof(float), PROM_M43_HEAD_COUNT, &value)) return 0;
  out_memory->grouped_readback_bytes = value;
#define PROM_M43_ADD_MEMORY(FIELD) \
  do { if (!prom_m43_checked_add_u64(total, out_memory->FIELD, &total)) return 0; } while (0)
  PROM_M43_ADD_MEMORY(shared_x_upload_bytes);
  PROM_M43_ADD_MEMORY(shared_x_f32_bytes);
  PROM_M43_ADD_MEMORY(shared_x_packed_bytes);
  PROM_M43_ADD_MEMORY(persistent_weight_upload_bytes);
  PROM_M43_ADD_MEMORY(persistent_weight_f32_bytes);
  PROM_M43_ADD_MEMORY(persistent_weight_packed_bytes);
  PROM_M43_ADD_MEMORY(qkv_f32_bytes);
  PROM_M43_ADD_MEMORY(qkv_packed_bytes);
  PROM_M43_ADD_MEMORY(score_bytes);
  PROM_M43_ADD_MEMORY(probability_bytes);
  PROM_M43_ADD_MEMORY(probability_packed_bytes);
  PROM_M43_ADD_MEMORY(output_bytes);
  PROM_M43_ADD_MEMORY(reduction_temporary_bytes);
  PROM_M43_ADD_MEMORY(grouped_readback_bytes);
#undef PROM_M43_ADD_MEMORY
  out_memory->exact_retained_bytes = total;
  return 1;
}

void prom_m43_eligibility_evaluate(const prom_m43_eligibility_facts* facts,
                                   prom_m43_eligibility_decision* out_decision) {
  uint64_t hash = 1469598103934665603ull;
  if (out_decision == NULL) return;
  memset(out_decision, 0, sizeof(*out_decision));
  if (facts == NULL) {
    out_decision->reason = PROM_M43_INELIGIBLE_HEAD_COUNT;
    return;
  }
  hash = prom_reduction_hash_u32(hash, facts->head_count);
  hash = prom_reduction_hash_u32(hash, facts->cooperative_capability_state);
  hash = prom_reduction_hash_u32(hash, facts->precision_policy);
  hash = prom_reduction_hash_u32(hash, facts->tokens);
  hash = prom_reduction_hash_u32(hash, facts->model_width);
  hash = prom_reduction_hash_u32(hash, facts->head_dim);
  hash = prom_reduction_hash_u32(hash, facts->padding_supported);
  hash = prom_reduction_hash_u32(hash, facts->persistent_weight_count);
  hash = prom_reduction_hash_u32(hash, facts->shared_x_available);
  hash = prom_reduction_hash_u32(hash, facts->generations_valid);
  hash = prom_reduction_hash_u32(hash, facts->rollback_head_mask);
  hash = prom_m40b_hash_u64(hash, facts->required_capacity_bytes);
  hash = prom_m40b_hash_u64(hash, facts->available_capacity_bytes);
  out_decision->replay_id = hash;
  if (facts->head_count != PROM_M43_HEAD_COUNT) out_decision->reason = PROM_M43_INELIGIBLE_HEAD_COUNT;
  else if (facts->cooperative_capability_state < PROM_VK_COOPERATIVE_MATRIX_DEVICE_FEATURE_ENABLED)
    out_decision->reason = PROM_M43_INELIGIBLE_CAPABILITY;
  else if (facts->precision_policy != PROM_M42_PRECISION_F16_ROUNDED)
    out_decision->reason = PROM_M43_INELIGIBLE_PRECISION;
  else if (facts->tokens == 0u || facts->model_width == 0u || facts->head_dim == 0u)
    out_decision->reason = PROM_M43_INELIGIBLE_SHAPE;
  else if (facts->padding_supported == 0u) out_decision->reason = PROM_M43_INELIGIBLE_PADDING;
  else if (facts->persistent_weight_count != PROM_M43_HEAD_COUNT * PROM_M43_WEIGHT_KIND_COUNT ||
           facts->generations_valid == 0u)
    out_decision->reason = PROM_M43_INELIGIBLE_PERSISTENT_WEIGHTS;
  else if (facts->shared_x_available == 0u) out_decision->reason = PROM_M43_INELIGIBLE_SHARED_X;
  else if (facts->required_capacity_bytes > facts->available_capacity_bytes)
    out_decision->reason = PROM_M43_INELIGIBLE_CAPACITY;
  else if (facts->rollback_head_mask != 0u) out_decision->reason = PROM_M43_INELIGIBLE_ROLLBACK;
  else {
    out_decision->eligible = 1u;
    out_decision->reason = PROM_M43_ELIGIBLE;
  }
}

static uint32_t prom_m43_descriptor_for_operation(uint32_t head_index, uint32_t operation) {
  const uint32_t base = head_index * PROM_M42_DESCRIPTOR_SET_COUNT;
  if (operation == PROM_M42_STAGE_PROJECT_Q) return base;
  if (operation == PROM_M42_STAGE_PROJECT_K) return base + 1u;
  if (operation == PROM_M42_STAGE_PROJECT_V) return base + 2u;
  if (operation == PROM_M42_STAGE_PACK_Q) return base + 3u;
  if (operation == PROM_M42_STAGE_LAYOUT_K) return base + 4u;
  if (operation == PROM_M42_STAGE_PACK_V) return base + 5u;
  if (operation == PROM_M42_STAGE_QK_TRANSPOSE) return base + 6u;
  if (operation == PROM_M42_STAGE_SCALE) return base + 7u;
  if (operation == PROM_M42_STAGE_SOFTMAX) return base + 8u;
  if (operation == PROM_M42_STAGE_PACK_P) return base + 13u;
  if (operation == PROM_M42_STAGE_PV) return base + 14u;
  return UINT32_MAX;
}

static uint32_t prom_m43_query_stage_index(uint32_t operation) {
  if (operation == PROM_M42_STAGE_PROJECT_Q) return 0u;
  if (operation == PROM_M42_STAGE_PROJECT_K) return 1u;
  if (operation == PROM_M42_STAGE_PROJECT_V) return 2u;
  if (operation == PROM_M42_STAGE_PACK_Q) return 3u;
  if (operation == PROM_M42_STAGE_LAYOUT_K) return 4u;
  if (operation == PROM_M42_STAGE_PACK_V) return 5u;
  if (operation == PROM_M42_STAGE_QK_TRANSPOSE) return 6u;
  if (operation == PROM_M42_STAGE_SCALE) return 7u;
  if (operation == PROM_M42_STAGE_SOFTMAX) return 8u;
  if (operation == PROM_M42_STAGE_PACK_P) return 9u;
  return 10u;
}

static const prom_m42_stage_plan* prom_m43_find_head_stage(const prom_m42_attention_plan* plan,
                                                            uint32_t operation) {
  uint32_t index;
  for (index = 0u; index < plan->stage_count; ++index) {
    if (plan->stages[index].operation == operation) return &plan->stages[index];
  }
  return NULL;
}

static int prom_m43_add_stage(prom_m43_attention_plan* plan,
                              uint32_t head_index,
                              uint32_t operation,
                              uint32_t selected_path,
                              uint32_t dispatch_count,
                              uint32_t barrier_calls,
                              uint32_t barrier_buffers,
                              uint32_t copy_regions,
                              uint32_t descriptor_index,
                              uint32_t timestamp_begin,
                              uint32_t timestamp_end) {
  prom_m43_stage_plan* stage;
  if (plan == NULL || plan->stage_count >= PROM_M43_MAX_STAGES) return 0;
  stage = &plan->stages[plan->stage_count];
  memset(stage, 0, sizeof(*stage));
  stage->sequence = plan->stage_count;
  stage->head_index = head_index;
  stage->operation = operation;
  stage->selected_path = selected_path;
  stage->dispatch_count = dispatch_count;
  stage->barrier_call_count = barrier_calls;
  stage->barrier_buffer_count = barrier_buffers;
  stage->copy_region_count = copy_regions;
  stage->descriptor_index = descriptor_index;
  stage->timestamp_begin = timestamp_begin;
  stage->timestamp_end = timestamp_end;
  stage->source_queue_family = VK_QUEUE_FAMILY_IGNORED;
  stage->destination_queue_family = VK_QUEUE_FAMILY_IGNORED;
  plan->stage_count += 1u;
  plan->dispatch_count += dispatch_count;
  plan->barrier_call_count += barrier_calls;
  plan->barrier_buffer_count += barrier_buffers;
  plan->copy_region_count += copy_regions;
  return 1;
}

static int prom_m43_add_head_operation(prom_m43_attention_plan* plan,
                                       uint32_t head_index,
                                       uint32_t operation,
                                       uint32_t barrier_calls,
                                       uint32_t barrier_buffers) {
  const prom_m42_stage_plan* head_stage = prom_m43_find_head_stage(&plan->head_plan[head_index], operation);
  const uint32_t pair = prom_m43_query_stage_index(operation);
  const uint32_t timestamp_begin = PROM_M43_QUERY_HEAD_BASE + head_index * PROM_M43_QUERY_HEAD_STRIDE + pair * 2u;
  if (head_stage == NULL) return 1;
  return prom_m43_add_stage(plan, head_index, operation, plan->selected_path[head_index],
                            head_stage->dispatch_count, barrier_calls, barrier_buffers, 0u,
                            prom_m43_descriptor_for_operation(head_index, operation),
                            timestamp_begin, timestamp_begin + 1u);
}

static int prom_m43_add_head_tail(prom_m43_attention_plan* plan, uint32_t head_index) {
  const uint32_t reduced = plan->selected_path[head_index] != PROM_M42_PATH_A2X4;
  return prom_m43_add_head_operation(plan, head_index, PROM_M42_STAGE_PACK_Q, reduced, reduced) &&
         prom_m43_add_head_operation(plan, head_index, PROM_M42_STAGE_LAYOUT_K, 1u, 1u) &&
         prom_m43_add_head_operation(plan, head_index, PROM_M42_STAGE_PACK_V, reduced, reduced) &&
         prom_m43_add_head_operation(plan, head_index, PROM_M42_STAGE_QK_TRANSPOSE, 1u, 1u) &&
         prom_m43_add_head_operation(plan, head_index, PROM_M42_STAGE_SCALE, 1u, 1u) &&
         prom_m43_add_head_operation(plan, head_index, PROM_M42_STAGE_SOFTMAX, 1u, 1u) &&
         prom_m43_add_head_operation(plan, head_index, PROM_M42_STAGE_PACK_P, reduced, reduced) &&
         prom_m43_add_head_operation(plan, head_index, PROM_M42_STAGE_PV, 0u, 0u);
}

int prom_m43_attention_plan_build(const prom_m43_plan_request* request,
                                  prom_m43_attention_plan* out_plan) {
  uint32_t padded_tokens;
  uint32_t padded_model;
  uint32_t padded_head;
  uint32_t head;
  uint32_t weight;
  uint32_t rollback_mask = 0u;
  uint64_t command_hash = 1469598103934665603ull;
  uint64_t aggregate_hash = 1469598103934665603ull;
  if (out_plan == NULL) return PROM_ERROR;
  memset(out_plan, 0, sizeof(*out_plan));
  if (request == NULL || request->head_count != PROM_M43_HEAD_COUNT ||
      request->execution_strategy < PROM_M43_STRATEGY_COMPLETE_HEADS ||
      request->execution_strategy > PROM_M43_STRATEGY_EIGHT_SEQUENTIAL_M42 ||
      request->precision_policy < PROM_M42_PRECISION_F16_ROUNDED ||
      request->precision_policy > PROM_M42_PRECISION_FP32 ||
      request->input_mode < PROM_M42_INPUT_HOST_X || request->input_mode > PROM_M42_INPUT_RESIDENT_X ||
      request->shared_x_generation == 0u || request->shared_x_hash == 0u ||
      !prom_m42_valid_shape(request->tokens, request->model_width, request->head_dim,
                            request->head_dim, &padded_tokens, &padded_model, &padded_head)) return PROM_ERROR;
  out_plan->head_count = PROM_M43_HEAD_COUNT;
  out_plan->tokens = request->tokens;
  out_plan->model_width = request->model_width;
  out_plan->head_dim = request->head_dim;
  out_plan->padded_tokens = padded_tokens;
  out_plan->padded_model_width = padded_model;
  out_plan->padded_head_dim = padded_head;
  out_plan->scale = prom_m42_resolve_scale(request->head_dim, request->scale, request->scale_explicit);
  if (!isfinite(out_plan->scale)) return PROM_ERROR;
  out_plan->precision_policy = request->precision_policy;
  out_plan->input_mode = request->input_mode;
  out_plan->execution_strategy = request->execution_strategy;
  out_plan->output_layout = PROM_M43_OUTPUT_HEAD_MAJOR;
  out_plan->shared_x_generation = request->shared_x_generation;
  out_plan->shared_x_hash = request->shared_x_hash;
  out_plan->shared_x_conversion_count = request->input_mode == PROM_M42_INPUT_HOST_X ? 1u : 0u;
  out_plan->shared_x_upload_count = request->input_mode == PROM_M42_INPUT_HOST_X ? 1u : 0u;
  out_plan->shared_x_consumer_count = PROM_M43_HEAD_COUNT;
  out_plan->persistent_weight_count = PROM_M43_HEAD_COUNT * PROM_M43_WEIGHT_KIND_COUNT;
  out_plan->qkv_projection_dispatch_count = PROM_M43_HEAD_COUNT * PROM_M43_WEIGHT_KIND_COUNT;
  out_plan->intermediate_host_copy_count = 0u;
  out_plan->final_readback_count = request->execution_strategy == PROM_M43_STRATEGY_EIGHT_SEQUENTIAL_M42
                                       ? PROM_M43_HEAD_COUNT
                                       : 1u;
  out_plan->submit_count = request->execution_strategy == PROM_M43_STRATEGY_EIGHT_SEQUENTIAL_M42
                               ? PROM_M43_HEAD_COUNT
                               : 1u;
  if (!prom_m43_memory_plan_build(request->tokens, request->model_width, request->head_dim,
                                  padded_tokens, padded_model, padded_head, &out_plan->memory)) return PROM_ERROR;
  for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
    prom_m42_plan_request head_request;
    uint32_t preferred = request->preferred_path[head];
    uint64_t head_hash;
    if (preferred < PROM_M42_PATH_COOPERATIVE || preferred > PROM_M42_PATH_CONVENTIONAL_FP16) return PROM_ERROR;
    if (request->rollback_active[head] != 0u) {
      rollback_mask |= 1u << head;
      if (preferred == PROM_M42_PATH_COOPERATIVE && request->precision_policy == PROM_M42_PRECISION_F16_ROUNDED) {
        if (request->allow_fallback == 0u) return PROM_ERROR;
        preferred = PROM_M42_PATH_CONVENTIONAL_FP16;
      }
    }
    memset(&head_request, 0, sizeof(head_request));
    head_request.tokens = request->tokens;
    head_request.model_width = request->model_width;
    head_request.head_dim = request->head_dim;
    head_request.value_dim = request->head_dim;
    head_request.scale = request->scale;
    head_request.scale_explicit = request->scale_explicit;
    head_request.precision_policy = request->precision_policy;
    head_request.preferred_path = preferred;
    head_request.allow_fallback = request->allow_fallback;
    head_request.input_mode = PROM_M42_INPUT_RESIDENT_X;
    head_request.cooperative_capability_state = request->cooperative_capability_state;
    for (weight = 0u; weight < PROM_M43_WEIGHT_KIND_COUNT; ++weight) {
      if (request->weight_generation[head][weight] == 0u || request->weight_hash[head][weight] == 0u)
        return PROM_ERROR;
      out_plan->weight_generation[head][weight] = request->weight_generation[head][weight];
      out_plan->weight_hash[head][weight] = request->weight_hash[head][weight];
    }
    head_request.wq_generation = request->weight_generation[head][PROM_M43_WEIGHT_Q];
    head_request.wk_generation = request->weight_generation[head][PROM_M43_WEIGHT_K];
    head_request.wv_generation = request->weight_generation[head][PROM_M43_WEIGHT_V];
    head_request.wq_hash = request->weight_hash[head][PROM_M43_WEIGHT_Q];
    head_request.wk_hash = request->weight_hash[head][PROM_M43_WEIGHT_K];
    head_request.wv_hash = request->weight_hash[head][PROM_M43_WEIGHT_V];
    if (prom_m42_attention_plan_build(&head_request, &out_plan->head_plan[head]) != PROM_OK) return PROM_ERROR;
    out_plan->selected_path[head] = out_plan->head_plan[head].selected_path;
    out_plan->fallback_used[head] = out_plan->head_plan[head].fallback_used || request->rollback_active[head] != 0u;
    out_plan->selector_reason[head] = request->rollback_active[head] != 0u
                                          ? PROM_M42_SELECTOR_ROLLBACK_FALLBACK
                                          : out_plan->head_plan[head].selector_reason;
    head_hash = prom_m40b_hash_u64(out_plan->head_plan[head].replay_id, request->shared_x_generation);
    head_hash = prom_m40b_hash_u64(head_hash, request->shared_x_hash);
    head_hash = prom_reduction_hash_u32(head_hash, head);
    out_plan->head_replay_id[head] = head_hash;
  }
  {
    prom_m43_eligibility_facts facts;
    memset(&facts, 0, sizeof(facts));
    facts.head_count = request->head_count;
    facts.cooperative_capability_state = request->cooperative_capability_state;
    facts.precision_policy = request->precision_policy;
    facts.tokens = request->tokens;
    facts.model_width = request->model_width;
    facts.head_dim = request->head_dim;
    facts.padding_supported = 1u;
    facts.persistent_weight_count = PROM_M43_HEAD_COUNT * PROM_M43_WEIGHT_KIND_COUNT;
    facts.shared_x_available = 1u;
    facts.generations_valid = 1u;
    facts.rollback_head_mask = rollback_mask;
    facts.required_capacity_bytes = out_plan->memory.exact_retained_bytes;
    facts.available_capacity_bytes = out_plan->memory.capacity_limit_bytes;
    prom_m43_eligibility_evaluate(&facts, &out_plan->eligibility);
  }
  if (request->input_mode == PROM_M42_INPUT_HOST_X &&
      !prom_m43_add_stage(out_plan, UINT32_MAX, PROM_M42_STAGE_UPLOAD_X, 0u, 1u, 3u, 3u, 1u,
                          PROM_M43_DESCRIPTOR_SET_COUNT - 1u, 0u, 2u)) return PROM_ERROR;
  if (request->execution_strategy == PROM_M43_STRATEGY_PROJECTION_GROUPED) {
    for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head)
      if (!prom_m43_add_head_operation(out_plan, head, PROM_M42_STAGE_PROJECT_Q, 0u, 0u)) return PROM_ERROR;
    for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head)
      if (!prom_m43_add_head_operation(out_plan, head, PROM_M42_STAGE_PROJECT_K, 0u, 0u)) return PROM_ERROR;
    for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
      const uint32_t calls = head + 1u == PROM_M43_HEAD_COUNT ? 1u : 0u;
      const uint32_t buffers = head + 1u == PROM_M43_HEAD_COUNT ? PROM_M43_HEAD_COUNT * 3u : 0u;
      if (!prom_m43_add_head_operation(out_plan, head, PROM_M42_STAGE_PROJECT_V, calls, buffers)) return PROM_ERROR;
    }
    for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head)
      if (!prom_m43_add_head_tail(out_plan, head)) return PROM_ERROR;
  } else {
    for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
      if (!prom_m43_add_head_operation(out_plan, head, PROM_M42_STAGE_PROJECT_Q, 0u, 0u) ||
          !prom_m43_add_head_operation(out_plan, head, PROM_M42_STAGE_PROJECT_K, 0u, 0u) ||
          !prom_m43_add_head_operation(out_plan, head, PROM_M42_STAGE_PROJECT_V, 1u, 3u) ||
          !prom_m43_add_head_tail(out_plan, head)) return PROM_ERROR;
      if (request->execution_strategy == PROM_M43_STRATEGY_EIGHT_SEQUENTIAL_M42 &&
          !prom_m43_add_stage(out_plan, head, PROM_M42_STAGE_FINAL_READBACK,
                              out_plan->selected_path[head], 0u, 2u, 2u, request->tokens,
                              UINT32_MAX,
                              PROM_M43_QUERY_HEAD_BASE + head * PROM_M43_QUERY_HEAD_STRIDE + 22u,
                              PROM_M43_QUERY_HEAD_BASE + head * PROM_M43_QUERY_HEAD_STRIDE + 23u))
        return PROM_ERROR;
    }
  }
  if (request->execution_strategy != PROM_M43_STRATEGY_EIGHT_SEQUENTIAL_M42 &&
      !prom_m43_add_stage(out_plan, UINT32_MAX, PROM_M42_STAGE_FINAL_READBACK, 0u, 0u, 2u,
                          PROM_M43_HEAD_COUNT + 1u, PROM_M43_HEAD_COUNT * request->tokens,
                          UINT32_MAX, PROM_M43_QUERY_READBACK_BEGIN, PROM_M43_QUERY_READBACK_END)) return PROM_ERROR;
  for (head = 0u; head < out_plan->stage_count; ++head) {
    const prom_m43_stage_plan* stage = &out_plan->stages[head];
    command_hash = prom_reduction_hash_u32(command_hash, stage->sequence);
    command_hash = prom_reduction_hash_u32(command_hash, stage->head_index);
    command_hash = prom_reduction_hash_u32(command_hash, stage->operation);
    command_hash = prom_reduction_hash_u32(command_hash, stage->selected_path);
    command_hash = prom_reduction_hash_u32(command_hash, stage->dispatch_count);
    command_hash = prom_reduction_hash_u32(command_hash, stage->barrier_call_count);
    command_hash = prom_reduction_hash_u32(command_hash, stage->barrier_buffer_count);
    command_hash = prom_reduction_hash_u32(command_hash, stage->copy_region_count);
    command_hash = prom_reduction_hash_u32(command_hash, stage->descriptor_index);
    command_hash = prom_reduction_hash_u32(command_hash, stage->timestamp_begin);
    command_hash = prom_reduction_hash_u32(command_hash, stage->timestamp_end);
  }
  out_plan->command_plan_replay_id = command_hash;
  aggregate_hash = prom_reduction_hash_u32(aggregate_hash, request->head_count);
  aggregate_hash = prom_reduction_hash_u32(aggregate_hash, request->tokens);
  aggregate_hash = prom_reduction_hash_u32(aggregate_hash, request->model_width);
  aggregate_hash = prom_reduction_hash_u32(aggregate_hash, request->head_dim);
  aggregate_hash = prom_reduction_hash_u32(aggregate_hash, request->precision_policy);
  aggregate_hash = prom_reduction_hash_u32(aggregate_hash, request->input_mode);
  aggregate_hash = prom_reduction_hash_u32(aggregate_hash, request->execution_strategy);
  aggregate_hash = prom_m40b_hash_u64(aggregate_hash, request->shared_x_generation);
  aggregate_hash = prom_m40b_hash_u64(aggregate_hash, request->shared_x_hash);
  for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head)
    aggregate_hash = prom_m40b_hash_u64(aggregate_hash, out_plan->head_replay_id[head]);
  aggregate_hash = prom_m40b_hash_u64(aggregate_hash, command_hash);
  aggregate_hash = prom_m40b_hash_u64(aggregate_hash, out_plan->memory.exact_retained_bytes);
  out_plan->aggregate_replay_id = aggregate_hash;
  return PROM_OK;
}

int prom_m43_attention_cpu_reference(const prom_m43_reference_request* request,
                                     prom_m43_reference_result* out_result) {
  uint32_t head;
  const uint64_t head_elements = request != NULL ? (uint64_t)request->tokens * request->head_dim : 0u;
  const uint64_t x_elements = request != NULL ? (uint64_t)request->tokens * request->model_width : 0u;
  const uint64_t weight_elements = request != NULL ? (uint64_t)request->model_width * request->head_dim : 0u;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  out_result->detail_code = PROM_M43_DETAIL_INVALID_REQUEST;
  if (request == NULL || request->x == NULL || request->output == NULL ||
      request->head_count != PROM_M43_HEAD_COUNT || request->x_element_count != x_elements ||
      request->weight_element_count != weight_elements ||
      request->output_element_count != head_elements * PROM_M43_HEAD_COUNT) return PROM_ERROR;
  for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
    prom_m42_reference_request head_request;
    prom_m42_reference_result head_result;
    if (request->weight[head][PROM_M43_WEIGHT_Q] == NULL ||
        request->weight[head][PROM_M43_WEIGHT_K] == NULL ||
        request->weight[head][PROM_M43_WEIGHT_V] == NULL) return PROM_ERROR;
    memset(&head_request, 0, sizeof(head_request));
    head_request.x = request->x;
    head_request.wq = request->weight[head][PROM_M43_WEIGHT_Q];
    head_request.wk = request->weight[head][PROM_M43_WEIGHT_K];
    head_request.wv = request->weight[head][PROM_M43_WEIGHT_V];
    head_request.output = request->output + (uint64_t)head * head_elements;
    head_request.tokens = request->tokens;
    head_request.model_width = request->model_width;
    head_request.head_dim = request->head_dim;
    head_request.value_dim = request->head_dim;
    head_request.scale = request->scale;
    head_request.scale_explicit = request->scale_explicit;
    head_request.precision_policy = request->precision_policy;
    if (prom_m42_attention_cpu_reference(&head_request, &head_result) != PROM_OK) {
      out_result->head_index = head;
      out_result->stage = head_result.stage;
      out_result->detail_code = head_result.detail_code;
      return PROM_ERROR;
    }
    if (head == 0u || head_result.minimum_probability_row_sum < out_result->minimum_probability_row_sum)
      out_result->minimum_probability_row_sum = head_result.minimum_probability_row_sum;
    if (head == 0u || head_result.maximum_probability_row_sum > out_result->maximum_probability_row_sum)
      out_result->maximum_probability_row_sum = head_result.maximum_probability_row_sum;
  }
  out_result->all_finite = 1u;
  out_result->detail_code = 0;
  return PROM_OK;
}

int prom_m43_attention_compare(const float* expected,
                               const float* actual,
                               uint32_t head_count,
                               uint32_t tokens,
                               uint32_t head_dim,
                               float absolute_tolerance,
                               float relative_tolerance,
                               const prom_m43_attention_plan* plan,
                               prom_m43_mismatch* out_mismatch) {
  uint32_t head;
  const uint64_t head_elements = (uint64_t)tokens * head_dim;
  if (out_mismatch == NULL) return PROM_ERROR;
  memset(out_mismatch, 0, sizeof(*out_mismatch));
  out_mismatch->matched = 1u;
  if (expected == NULL || actual == NULL || plan == NULL || head_count != PROM_M43_HEAD_COUNT ||
      plan->head_count != PROM_M43_HEAD_COUNT) return PROM_ERROR;
  for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
    prom_m42_mismatch mismatch;
    if (prom_m42_attention_compare(PROM_M42_STAGE_PV,
                                   expected + (uint64_t)head * head_elements,
                                   actual + (uint64_t)head * head_elements,
                                   tokens, head_dim, plan->padded_tokens, plan->padded_head_dim,
                                   absolute_tolerance, relative_tolerance,
                                   plan->head_replay_id[head], plan->head_plan[head].reduction_replay_id,
                                   &mismatch) != PROM_OK) {
      out_mismatch->matched = 0u;
      out_mismatch->head_index = head;
      out_mismatch->stage_mismatch = mismatch;
      memcpy(out_mismatch->weight_generation, plan->weight_generation[head],
             sizeof(out_mismatch->weight_generation));
      out_mismatch->head_replay_id = plan->head_replay_id[head];
      out_mismatch->aggregate_replay_id = plan->aggregate_replay_id;
      return PROM_ERROR;
    }
  }
  return PROM_OK;
}

uint64_t prom_m43_output_index(uint32_t head,
                               uint32_t token,
                               uint32_t column,
                               uint32_t tokens,
                               uint32_t head_dim) {
  if (head >= PROM_M43_HEAD_COUNT || token >= tokens || column >= head_dim ||
      tokens == 0u || head_dim == 0u) return UINT64_MAX;
  return ((uint64_t)head * tokens + token) * head_dim + column;
}

uint64_t prom_m44_concat_index(uint32_t token,
                               uint32_t head,
                               uint32_t column,
                               uint32_t tokens,
                               uint32_t head_dim) {
  if (tokens == 0u || head_dim == 0u || token >= tokens || head >= PROM_M44_HEAD_COUNT ||
      column >= head_dim) return UINT64_MAX;
  return (uint64_t)token * PROM_M44_HEAD_COUNT * head_dim + (uint64_t)head * head_dim + column;
}

void prom_m44_eligibility_evaluate(const prom_m44_eligibility_facts* facts,
                                   prom_m44_eligibility_decision* out_decision) {
  uint64_t hash = 1469598103934665603ull;
  if (out_decision == NULL) return;
  memset(out_decision, 0, sizeof(*out_decision));
  if (facts == NULL) {
    out_decision->reason = PROM_M44_INELIGIBLE_HEAD_COUNT;
    return;
  }
  hash = prom_reduction_hash_u32(hash, facts->head_count);
  hash = prom_reduction_hash_u32(hash, facts->views_valid);
  hash = prom_reduction_hash_u32(hash, facts->shapes_match);
  hash = prom_reduction_hash_u32(hash, facts->generations_valid);
  hash = prom_reduction_hash_u32(hash, facts->non_overlapping);
  hash = prom_reduction_hash_u32(hash, facts->wo_valid);
  hash = prom_reduction_hash_u32(hash, facts->shape_valid);
  hash = prom_reduction_hash_u32(hash, facts->precision_valid);
  hash = prom_reduction_hash_u32(hash, facts->cooperative_capability_state);
  hash = prom_reduction_hash_u32(hash, facts->padding_supported);
  hash = prom_reduction_hash_u32(hash, facts->strategy_supported);
  hash = prom_reduction_hash_u32(hash, facts->rollback_active);
  hash = prom_m40b_hash_u64(hash, facts->required_capacity_bytes);
  hash = prom_m40b_hash_u64(hash, facts->available_capacity_bytes);
  out_decision->replay_id = hash;
  if (facts->head_count != PROM_M44_HEAD_COUNT) out_decision->reason = PROM_M44_INELIGIBLE_HEAD_COUNT;
  else if (facts->views_valid == 0u) out_decision->reason = PROM_M44_INELIGIBLE_VIEW;
  else if (facts->shapes_match == 0u) out_decision->reason = PROM_M44_INELIGIBLE_VIEW_SHAPE;
  else if (facts->generations_valid == 0u) out_decision->reason = PROM_M44_INELIGIBLE_VIEW_GENERATION;
  else if (facts->non_overlapping == 0u) out_decision->reason = PROM_M44_INELIGIBLE_VIEW_OVERLAP;
  else if (facts->wo_valid == 0u) out_decision->reason = PROM_M44_INELIGIBLE_WO;
  else if (facts->shape_valid == 0u) out_decision->reason = PROM_M44_INELIGIBLE_SHAPE;
  else if (facts->precision_valid == 0u) out_decision->reason = PROM_M44_INELIGIBLE_PRECISION;
  else if (facts->cooperative_capability_state == PROM_VK_COOPERATIVE_MATRIX_UNAVAILABLE)
    out_decision->reason = PROM_M44_INELIGIBLE_CAPABILITY;
  else if (facts->padding_supported == 0u) out_decision->reason = PROM_M44_INELIGIBLE_PADDING;
  else if (facts->strategy_supported == 0u) out_decision->reason = PROM_M44_INELIGIBLE_STRATEGY;
  else if (facts->required_capacity_bytes > facts->available_capacity_bytes)
    out_decision->reason = PROM_M44_INELIGIBLE_CAPACITY;
  else if (facts->rollback_active != 0u) out_decision->reason = PROM_M44_INELIGIBLE_ROLLBACK;
  else {
    out_decision->eligible = 1u;
    out_decision->reason = PROM_M44_ELIGIBLE;
  }
}

static int prom_m44_ranges_overlap(const prom_device_buffer_view* left,
                                   const prom_device_buffer_view* right) {
  VkDeviceSize left_end;
  VkDeviceSize right_end;
  if (left->buffer != right->buffer) return 0;
  if (left->offset > UINT64_MAX - left->byte_length || right->offset > UINT64_MAX - right->byte_length)
    return 1;
  left_end = left->offset + left->byte_length;
  right_end = right->offset + right->byte_length;
  return left->offset < right_end && right->offset < left_end;
}

static int prom_m44_add_stage(prom_m44_output_projection_plan* plan,
                              uint32_t operation,
                              uint32_t dispatch_count,
                              uint32_t barrier_calls,
                              uint32_t barrier_buffers,
                              uint32_t copy_regions,
                              uint32_t timestamp_begin,
                              uint32_t timestamp_end,
                              uint32_t source_stage,
                              uint32_t destination_stage,
                              uint32_t source_access,
                              uint32_t destination_access) {
  prom_m44_stage_plan* stage;
  if (plan == NULL || plan->stage_count >= PROM_M44_MAX_STAGES) return 0;
  stage = &plan->stages[plan->stage_count];
  memset(stage, 0, sizeof(*stage));
  stage->sequence = plan->stage_count;
  stage->operation = operation;
  stage->dispatch_count = dispatch_count;
  stage->barrier_call_count = barrier_calls;
  stage->barrier_buffer_count = barrier_buffers;
  stage->copy_region_count = copy_regions;
  stage->timestamp_begin = timestamp_begin;
  stage->timestamp_end = timestamp_end;
  stage->source_stage_mask = source_stage;
  stage->destination_stage_mask = destination_stage;
  stage->source_access_mask = source_access;
  stage->destination_access_mask = destination_access;
  stage->source_queue_family = VK_QUEUE_FAMILY_IGNORED;
  stage->destination_queue_family = VK_QUEUE_FAMILY_IGNORED;
  plan->stage_count += 1u;
  plan->dispatch_count += dispatch_count;
  plan->barrier_call_count += barrier_calls;
  plan->barrier_buffer_count += barrier_buffers;
  plan->copy_region_count += copy_regions;
  return 1;
}

static int prom_m44_memory_plan_build(const prom_m44_plan_request* request,
                                      uint32_t concatenated_width,
                                      uint32_t padded_tokens,
                                      uint32_t padded_concatenated,
                                      uint32_t padded_model,
                                      uint32_t head_stride,
                                      prom_m44_memory_plan* out_memory) {
  uint64_t source_elements;
  uint64_t logical_concat;
  uint64_t padded_concat;
  uint64_t logical_wo;
  uint64_t padded_wo;
  uint64_t logical_y;
  uint64_t padded_y;
  uint64_t total = 0u;
  if (request == NULL || out_memory == NULL ||
      !prom_m40b_checked_product_u64(request->tokens, head_stride, &source_elements) ||
      !prom_m43_checked_scale_u64(source_elements, PROM_M44_HEAD_COUNT, &source_elements) ||
      !prom_m40b_checked_product_u64(request->tokens, concatenated_width, &logical_concat) ||
      !prom_m40b_checked_product_u64(padded_tokens, padded_concatenated, &padded_concat) ||
      !prom_m40b_checked_product_u64(concatenated_width, request->model_width, &logical_wo) ||
      !prom_m40b_checked_product_u64(padded_concatenated, padded_model, &padded_wo) ||
      !prom_m40b_checked_product_u64(request->tokens, request->model_width, &logical_y) ||
      !prom_m40b_checked_product_u64(padded_tokens, padded_model, &padded_y)) return 0;
  memset(out_memory, 0, sizeof(*out_memory));
  out_memory->capacity_limit_bytes = PROM_M44_CAPACITY_LIMIT_BYTES;
  out_memory->source_head_bytes = source_elements * sizeof(float);
  if (request->aggregation_strategy == PROM_M44_AGGREGATION_INTERLEAVE) {
    if (request->projection_path == PROM_M44_PROJECTION_A2X4_FP32)
      out_memory->contiguous_f32_bytes = logical_concat * sizeof(float);
    else
      out_memory->contiguous_packed_bytes = ((padded_concat + 1u) / 2u) * sizeof(uint32_t);
  }
  out_memory->wo_upload_bytes = logical_wo * sizeof(float);
  out_memory->wo_f32_bytes = logical_wo * sizeof(float);
  out_memory->wo_packed_bytes = ((padded_wo + 1u) / 2u) * sizeof(uint32_t);
  out_memory->final_y_bytes = request->projection_path == PROM_M44_PROJECTION_A2X4_FP32 ||
                                      request->projection_path == PROM_M44_PROJECTION_DIRECT_SEGMENTED_FP16
                                  ? logical_y * sizeof(float)
                                  : padded_y * sizeof(float);
  out_memory->final_readback_bytes = logical_y * sizeof(float);
  out_memory->reusable_descriptor_set_count = 2u;
  out_memory->descriptor_binding_count = PROM_M44_WIDE_DESCRIPTOR_BINDING_COUNT + 4u;
#define PROM_M44_ADD_MEMORY(FIELD) \
  do { if (!prom_m43_checked_add_u64(total, out_memory->FIELD, &total)) return 0; } while (0)
  PROM_M44_ADD_MEMORY(source_head_bytes);
  PROM_M44_ADD_MEMORY(contiguous_f32_bytes);
  PROM_M44_ADD_MEMORY(contiguous_packed_bytes);
  PROM_M44_ADD_MEMORY(partial_output_bytes);
  PROM_M44_ADD_MEMORY(accumulation_bytes);
  PROM_M44_ADD_MEMORY(wo_upload_bytes);
  PROM_M44_ADD_MEMORY(wo_f32_bytes);
  PROM_M44_ADD_MEMORY(wo_packed_bytes);
  PROM_M44_ADD_MEMORY(final_y_bytes);
  PROM_M44_ADD_MEMORY(final_readback_bytes);
#undef PROM_M44_ADD_MEMORY
  out_memory->exact_request_bytes = total;
  return 1;
}

int prom_m44_output_projection_plan_build(const prom_m44_plan_request* request,
                                          prom_m44_output_projection_plan* out_plan) {
  uint32_t padded_tokens;
  uint32_t padded_concat;
  uint32_t padded_model;
  uint32_t head;
  uint32_t other;
  uint32_t concatenated_width;
  uint32_t views_valid = 1u;
  uint32_t shapes_match = 1u;
  uint32_t generations_valid = 1u;
  uint32_t non_overlapping = 1u;
  uint32_t precision_valid = 1u;
  uint32_t strategy_supported = 1u;
  uint64_t command_hash = 1469598103934665603ull;
  uint64_t replay_hash = 1469598103934665603ull;
  int32_t view_detail = 0;
  prom_m44_eligibility_facts facts;
  if (out_plan == NULL) return PROM_ERROR;
  memset(out_plan, 0, sizeof(*out_plan));
  if (request == NULL || request->head_count != PROM_M44_HEAD_COUNT ||
      request->aggregation_strategy < PROM_M44_AGGREGATION_INTERLEAVE ||
      request->aggregation_strategy > PROM_M44_AGGREGATION_DIRECT_SEGMENTED ||
      request->projection_path < PROM_M44_PROJECTION_COOPERATIVE ||
      request->projection_path > PROM_M44_PROJECTION_DIRECT_SEGMENTED_FP16 ||
      request->submit_plan < PROM_M44_SUBMIT_ONE_COMMAND_BUFFER ||
      request->submit_plan > PROM_M44_SUBMIT_TWO_BOUNDED ||
      request->wo_generation == 0u || request->wo_hash == 0u ||
      request->m43_aggregate_replay_id == 0u || request->tokens == 0u ||
      request->tokens > PROM_M42_MAX_TOKENS || request->head_dim == 0u ||
      request->head_dim > PROM_M42_MAX_HEAD_DIM || request->model_width == 0u ||
      request->model_width > PROM_M42_MAX_MODEL_WIDTH ||
      !prom_vk_checked_mul_u32(PROM_M44_HEAD_COUNT, request->head_dim, &concatenated_width) ||
      concatenated_width > 8192u || !prom_m42_round_up_16(request->tokens, &padded_tokens) ||
      !prom_m42_round_up_16(concatenated_width, &padded_concat) ||
      !prom_m42_round_up_16(request->model_width, &padded_model)) return PROM_ERROR;
  for (head = 0u; head < PROM_M44_HEAD_COUNT; ++head) {
    const prom_device_buffer_view* view = &request->head_views[head];
    prom_device_buffer_view structural_view = *view;
    if (structural_view.owning_lifetime_id == 0u) structural_view.owning_lifetime_id = 1u;
    if (structural_view.owning_slot_generation == 0u) structural_view.owning_slot_generation = 1u;
    if (prom_m40b_validate_device_buffer_view(&structural_view, request->head_views[0].owning_device,
                                              PROM_DEVICE_ELEMENT_F32, request->tokens,
                                              request->head_dim, PROM_DEVICE_ACCESS_COMPUTE_READ,
                                              &view_detail) != PROM_OK) views_valid = 0u;
    if (view->logical_rows != request->tokens || view->logical_columns != request->head_dim ||
        view->row_stride_elements != request->head_views[0].row_stride_elements)
      shapes_match = 0u;
    if (view->owning_lifetime_id == 0u || view->owning_slot_generation == 0u)
      generations_valid = 0u;
    for (other = head + 1u; other < PROM_M44_HEAD_COUNT; ++other)
      if (prom_m44_ranges_overlap(view, &request->head_views[other])) non_overlapping = 0u;
  }
  if (request->aggregation_strategy == PROM_M44_AGGREGATION_DIRECT_SEGMENTED) {
    if (request->projection_path != PROM_M44_PROJECTION_DIRECT_SEGMENTED_FP16)
      strategy_supported = 0u;
    if (request->precision_policy != PROM_M42_PRECISION_F16_ROUNDED) precision_valid = 0u;
  } else {
    if (request->projection_path == PROM_M44_PROJECTION_DIRECT_SEGMENTED_FP16)
      strategy_supported = 0u;
    if (request->projection_path == PROM_M44_PROJECTION_A2X4_FP32) {
      if (request->precision_policy != PROM_M42_PRECISION_FP32) precision_valid = 0u;
    } else if (request->precision_policy != PROM_M42_PRECISION_F16_ROUNDED) {
      precision_valid = 0u;
    }
  }
  out_plan->head_count = PROM_M44_HEAD_COUNT;
  out_plan->tokens = request->tokens;
  out_plan->head_dim = request->head_dim;
  out_plan->concatenated_width = concatenated_width;
  out_plan->model_width = request->model_width;
  out_plan->padded_tokens = padded_tokens;
  out_plan->padded_concatenated_width = padded_concat;
  out_plan->padded_model_width = padded_model;
  out_plan->head_row_stride = request->head_views[0].row_stride_elements;
  out_plan->output_row_stride = request->projection_path == PROM_M44_PROJECTION_A2X4_FP32 ||
                                        request->projection_path == PROM_M44_PROJECTION_DIRECT_SEGMENTED_FP16
                                    ? request->model_width
                                    : padded_model;
  out_plan->precision_policy = request->precision_policy;
  out_plan->aggregation_strategy = request->aggregation_strategy;
  out_plan->projection_path = request->projection_path;
  out_plan->submit_plan = request->submit_plan;
  out_plan->submit_count = request->submit_plan == PROM_M44_SUBMIT_TWO_BOUNDED ? 2u : 1u;
  out_plan->intermediate_host_copy_count = 0u;
  out_plan->final_readback_count = 1u;
  out_plan->output_element_type = PROM_DEVICE_ELEMENT_F32;
  out_plan->wo_generation = request->wo_generation;
  out_plan->wo_hash = request->wo_hash;
  out_plan->m43_aggregate_replay_id = request->m43_aggregate_replay_id;
  if (!prom_m44_memory_plan_build(request, concatenated_width, padded_tokens, padded_concat,
                                  padded_model, out_plan->head_row_stride, &out_plan->memory)) return PROM_ERROR;
  memset(&facts, 0, sizeof(facts));
  facts.head_count = request->head_count;
  facts.views_valid = views_valid;
  facts.shapes_match = shapes_match;
  facts.generations_valid = generations_valid;
  facts.non_overlapping = non_overlapping;
  facts.wo_valid = request->wo_generation != 0u && request->wo_hash != 0u;
  facts.shape_valid = 1u;
  facts.precision_valid = precision_valid;
  facts.cooperative_capability_state = request->projection_path == PROM_M44_PROJECTION_COOPERATIVE
                                           ? request->cooperative_capability_state
                                           : PROM_VK_COOPERATIVE_MATRIX_DEVICE_FEATURE_ENABLED;
  facts.padding_supported = 1u;
  facts.strategy_supported = strategy_supported;
  facts.rollback_active = request->rollback_active;
  facts.required_capacity_bytes = out_plan->memory.exact_request_bytes;
  facts.available_capacity_bytes = out_plan->memory.capacity_limit_bytes;
  prom_m44_eligibility_evaluate(&facts, &out_plan->eligibility);
  if (!prom_m44_add_stage(out_plan, PROM_M44_STAGE_HEADS_READY, 0u, 1u, PROM_M44_HEAD_COUNT,
                          0u, PROM_M43_QUERY_GROUP_END, PROM_M44_QUERY_AGGREGATION_BEGIN,
                          VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                          VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT)) return PROM_ERROR;
  if (request->aggregation_strategy == PROM_M44_AGGREGATION_INTERLEAVE) {
    if (!prom_m44_add_stage(out_plan, PROM_M44_STAGE_INTERLEAVE, 1u, 1u, 1u, 0u,
                            PROM_M44_QUERY_AGGREGATION_BEGIN, PROM_M44_QUERY_AGGREGATION_END,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                            VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT) ||
        !prom_m44_add_stage(out_plan, PROM_M44_STAGE_OUTPUT_PROJECTION, 1u, 1u, 1u, 0u,
                            PROM_M44_QUERY_PROJECTION_BEGIN, PROM_M44_QUERY_PROJECTION_END,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT,
                            VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT)) return PROM_ERROR;
  } else if (!prom_m44_add_stage(out_plan, PROM_M44_STAGE_DIRECT_PROJECTION, 1u, 1u, 1u, 0u,
                                 PROM_M44_QUERY_PROJECTION_BEGIN, PROM_M44_QUERY_PROJECTION_END,
                                 VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT,
                                 VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT)) {
    return PROM_ERROR;
  }
  if (!prom_m44_add_stage(out_plan, PROM_M44_STAGE_FINAL_READBACK, 0u, 1u, 1u,
                          request->tokens, PROM_M44_QUERY_READBACK_BEGIN, PROM_M44_QUERY_READBACK_END,
                          VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_HOST_BIT,
                          VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_HOST_READ_BIT)) return PROM_ERROR;
  for (head = 0u; head < out_plan->stage_count; ++head) {
    const prom_m44_stage_plan* stage = &out_plan->stages[head];
    command_hash = prom_reduction_hash_u32(command_hash, stage->sequence);
    command_hash = prom_reduction_hash_u32(command_hash, stage->operation);
    command_hash = prom_reduction_hash_u32(command_hash, stage->dispatch_count);
    command_hash = prom_reduction_hash_u32(command_hash, stage->barrier_call_count);
    command_hash = prom_reduction_hash_u32(command_hash, stage->barrier_buffer_count);
    command_hash = prom_reduction_hash_u32(command_hash, stage->copy_region_count);
    command_hash = prom_reduction_hash_u32(command_hash, stage->timestamp_begin);
    command_hash = prom_reduction_hash_u32(command_hash, stage->timestamp_end);
  }
  out_plan->command_plan_replay_id = command_hash;
  replay_hash = prom_reduction_hash_u32(replay_hash, request->head_count);
  replay_hash = prom_reduction_hash_u32(replay_hash, request->tokens);
  replay_hash = prom_reduction_hash_u32(replay_hash, request->head_dim);
  replay_hash = prom_reduction_hash_u32(replay_hash, request->model_width);
  replay_hash = prom_reduction_hash_u32(replay_hash, request->precision_policy);
  replay_hash = prom_reduction_hash_u32(replay_hash, request->aggregation_strategy);
  replay_hash = prom_reduction_hash_u32(replay_hash, request->projection_path);
  replay_hash = prom_reduction_hash_u32(replay_hash, request->submit_plan);
  replay_hash = prom_m40b_hash_u64(replay_hash, request->wo_generation);
  replay_hash = prom_m40b_hash_u64(replay_hash, request->wo_hash);
  replay_hash = prom_m40b_hash_u64(replay_hash, request->m43_aggregate_replay_id);
  replay_hash = prom_m40b_hash_u64(replay_hash, PROM_M44_INTERLEAVE_SHADER_HASH);
  replay_hash = prom_m40b_hash_u64(replay_hash, PROM_M44_DIRECT_SHADER_HASH);
  replay_hash = prom_m40b_hash_u64(replay_hash, command_hash);
  out_plan->replay_id = replay_hash;
  return PROM_OK;
}

int prom_m44_output_projection_cpu_reference(const prom_m44_reference_request* request,
                                             prom_m44_reference_result* out_result) {
  uint64_t head_elements;
  uint64_t wo_elements;
  uint64_t output_elements;
  uint32_t token;
  uint32_t reduced;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  out_result->detail_code = PROM_M44_DETAIL_INVALID_REQUEST;
  if (request == NULL || request->head_major == NULL || request->wo == NULL || request->output == NULL ||
      request->head_count != PROM_M44_HEAD_COUNT || request->tokens == 0u || request->head_dim == 0u ||
      request->model_width == 0u || request->precision_policy < PROM_M42_PRECISION_F16_ROUNDED ||
      request->precision_policy > PROM_M42_PRECISION_FP32 ||
      !prom_m40b_checked_product_u64(PROM_M44_HEAD_COUNT, request->tokens, &head_elements) ||
      !prom_m40b_checked_product_u64(head_elements, request->head_dim, &head_elements) ||
      !prom_m40b_checked_product_u64(PROM_M44_HEAD_COUNT, request->head_dim, &wo_elements) ||
      !prom_m40b_checked_product_u64(wo_elements, request->model_width, &wo_elements) ||
      !prom_m40b_checked_product_u64(request->tokens, request->model_width, &output_elements) ||
      request->head_major_element_count != head_elements || request->wo_element_count != wo_elements ||
      request->output_element_count != output_elements ||
      !prom_m42_finite_matrix(request->head_major, head_elements) ||
      !prom_m42_finite_matrix(request->wo, wo_elements)) return PROM_ERROR;
  reduced = request->precision_policy == PROM_M42_PRECISION_F16_ROUNDED;
  for (token = 0u; token < request->tokens; ++token) {
    uint32_t head;
    for (head = 0u; head < PROM_M44_HEAD_COUNT; ++head) {
      uint32_t column;
      for (column = 0u; column < request->head_dim; ++column) {
        const uint64_t source_index = ((uint64_t)head * request->tokens + token) * request->head_dim + column;
        const uint64_t concat_index = prom_m44_concat_index(token, head, column,
                                                            request->tokens, request->head_dim);
        float value = request->head_major[source_index];
        if (reduced != 0u) value = prom_m42_round_f16(value);
        if (request->concatenated != NULL) request->concatenated[concat_index] = value;
      }
    }
  }
  for (token = 0u; token < request->tokens; ++token) {
    uint32_t output_column;
    for (output_column = 0u; output_column < request->model_width; ++output_column) {
      uint32_t head;
      float accumulator = 0.0f;
      for (head = 0u; head < PROM_M44_HEAD_COUNT; ++head) {
        uint32_t column;
        for (column = 0u; column < request->head_dim; ++column) {
          const uint64_t source_index = ((uint64_t)head * request->tokens + token) * request->head_dim + column;
          const uint64_t wo_row = (uint64_t)head * request->head_dim + column;
          float value = request->head_major[source_index];
          float weight = request->wo[wo_row * request->model_width + output_column];
          if (reduced != 0u) {
            value = prom_m42_round_f16(value);
            weight = prom_m42_round_f16(weight);
          }
          accumulator += value * weight;
        }
      }
      request->output[(uint64_t)token * request->model_width + output_column] = accumulator;
    }
  }
  out_result->all_finite = prom_m42_finite_matrix(request->output, output_elements);
  if (out_result->all_finite == 0u) {
    out_result->detail_code = PROM_M44_DETAIL_MISMATCH;
    return PROM_ERROR;
  }
  out_result->detail_code = 0;
  return PROM_OK;
}

int prom_m44_output_projection_compare(const float* expected,
                                       const float* actual,
                                       uint32_t tokens,
                                       uint32_t model_width,
                                       float absolute_tolerance,
                                       float relative_tolerance,
                                       uint32_t strategy,
                                       uint64_t wo_generation,
                                       uint64_t m43_aggregate_replay_id,
                                       uint64_t m44_replay_id,
                                       prom_m44_mismatch* out_mismatch) {
  uint32_t token;
  if (out_mismatch == NULL) return PROM_ERROR;
  memset(out_mismatch, 0, sizeof(*out_mismatch));
  out_mismatch->matched = 1u;
  out_mismatch->strategy = strategy;
  out_mismatch->source_head = UINT32_MAX;
  out_mismatch->source_column = UINT32_MAX;
  out_mismatch->wo_generation = wo_generation;
  out_mismatch->m43_aggregate_replay_id = m43_aggregate_replay_id;
  out_mismatch->m44_replay_id = m44_replay_id;
  if (expected == NULL || actual == NULL || tokens == 0u || model_width == 0u ||
      absolute_tolerance < 0.0f || relative_tolerance < 0.0f) return PROM_ERROR;
  for (token = 0u; token < tokens; ++token) {
    uint32_t column;
    for (column = 0u; column < model_width; ++column) {
      const uint64_t index = (uint64_t)token * model_width + column;
      const float absolute_error = fabsf(expected[index] - actual[index]);
      const float relative_error = absolute_error / fmaxf(fabsf(expected[index]), 1.0e-12f);
      if (!isfinite(expected[index]) || !isfinite(actual[index]) ||
          (absolute_error > absolute_tolerance && relative_error > relative_tolerance)) {
        out_mismatch->matched = 0u;
        out_mismatch->token = token;
        out_mismatch->output_column = column;
        out_mismatch->expected = expected[index];
        out_mismatch->actual = actual[index];
        out_mismatch->absolute_error = absolute_error;
        out_mismatch->relative_error = relative_error;
        return PROM_ERROR;
      }
    }
  }
  return PROM_OK;
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

static int prom_m45_used_ranges_overlap(const prom_device_buffer_view* left,
                                        uint64_t left_bytes,
                                        const prom_device_buffer_view* right,
                                        uint64_t right_bytes) {
  uint64_t left_end;
  uint64_t right_end;
  if (left->buffer != right->buffer) return 0;
  if (left->offset > UINT64_MAX - left_bytes || right->offset > UINT64_MAX - right_bytes) return 1;
  left_end = left->offset + left_bytes;
  right_end = right->offset + right_bytes;
  return left->offset < right_end && right->offset < left_end;
}

static void prom_m45_add_barrier(prom_m45_residual_plan* plan,
                                 uint32_t buffer_identity,
                                 uint64_t byte_offset,
                                 uint64_t byte_length,
                                 uint32_t source_stage,
                                 uint32_t destination_stage,
                                 uint32_t source_access,
                                 uint32_t destination_access) {
  prom_m45_barrier_trace* barrier = &plan->barriers[plan->barrier_count];
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

static void prom_m45_add_stage(prom_m45_residual_plan* plan,
                               uint32_t operation,
                               uint32_t dispatch_count,
                               uint32_t barrier_begin,
                               uint32_t barrier_count,
                               uint32_t copy_regions,
                               uint32_t timestamp_begin,
                               uint32_t timestamp_end) {
  prom_m45_stage_plan* stage = &plan->stages[plan->stage_count];
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

int prom_m45_residual_plan_build(const prom_m45_plan_request* request,
                                 prom_m45_residual_plan* out_plan) {
  uint64_t x_bytes = 0u;
  uint64_t y_bytes = 0u;
  uint64_t logical_elements = 0u;
  uint64_t compact_bytes = 0u;
  uint64_t total = 0u;
  uint64_t eligibility_hash = 1469598103934665603ull;
  uint64_t command_hash = 1469598103934665603ull;
  uint64_t replay_hash = 1469598103934665603ull;
  uint32_t reason = PROM_M45_ELIGIBLE;
  int32_t detail = 0;
  if (out_plan == NULL) return PROM_ERROR;
  memset(out_plan, 0, sizeof(*out_plan));
  if (request == NULL || request->tokens == 0u || request->tokens > PROM_M42_MAX_TOKENS ||
      request->model_width == 0u || request->model_width > PROM_M42_MAX_MODEL_WIDTH ||
      request->strategy < PROM_M45_STRATEGY_SEPARATE_OUTPUT ||
      request->strategy > PROM_M45_STRATEGY_IN_PLACE_X_AUDIT ||
      request->submit_policy < PROM_M45_SUBMIT_ONE_COMMAND_BUFFER ||
      request->submit_policy > PROM_M45_SUBMIT_TWO_BOUNDED ||
      request->m44_replay_id == 0u ||
      !prom_m40b_checked_product_u64(request->tokens, request->model_width, &logical_elements) ||
      logical_elements > UINT64_MAX / sizeof(float)) return PROM_ERROR;
  compact_bytes = logical_elements * sizeof(float);
  out_plan->tokens = request->tokens;
  out_plan->model_width = request->model_width;
  out_plan->x_row_stride = request->x_view.row_stride_elements;
  out_plan->y_row_stride = request->y_view.row_stride_elements;
  out_plan->z_row_stride = request->strategy == PROM_M45_STRATEGY_IN_PLACE_Y
                             ? request->y_view.row_stride_elements : request->model_width;
  out_plan->strategy = request->strategy;
  out_plan->submit_policy = request->submit_policy;
  out_plan->precision_policy = request->precision_policy;
  out_plan->physical_alias_plan = request->strategy == PROM_M45_STRATEGY_IN_PLACE_Y ? 2u : 1u;
  out_plan->submit_count = request->submit_policy == PROM_M45_SUBMIT_TWO_BOUNDED ? 2u : 1u;
  out_plan->intermediate_host_copy_count = 0u;
  out_plan->final_readback_count = request->final_readback != 0u ? 1u : 0u;
  out_plan->x_generation = request->expected_x_generation;
  out_plan->y_generation = request->expected_y_generation;
  out_plan->shader_hash = PROM_M45_RESIDUAL_SHADER_HASH;
  out_plan->m44_replay_id = request->m44_replay_id;
  out_plan->memory.capacity_limit_bytes = PROM_M45_CAPACITY_LIMIT_BYTES;
  out_plan->memory.reusable_descriptor_set_count = 1u;
  out_plan->memory.descriptor_binding_count = 3u;
  if (!prom_m45_view_required_bytes(&request->x_view, &x_bytes) ||
      !prom_m45_view_required_bytes(&request->y_view, &y_bytes) ||
      request->x_view.buffer == VK_NULL_HANDLE || request->y_view.buffer == VK_NULL_HANDLE ||
      request->x_view.element_type != PROM_DEVICE_ELEMENT_F32 ||
      request->y_view.element_type != PROM_DEVICE_ELEMENT_F32 ||
      request->x_view.layout != PROM_DEVICE_LAYOUT_ROW_MAJOR ||
      request->y_view.layout != PROM_DEVICE_LAYOUT_ROW_MAJOR ||
      request->x_view.owning_device == VK_NULL_HANDLE || request->y_view.owning_device == VK_NULL_HANDLE ||
      request->x_view.byte_length < x_bytes || request->y_view.byte_length < y_bytes ||
      request->x_view.offset > UINT64_MAX - x_bytes || request->y_view.offset > UINT64_MAX - y_bytes)
    reason = PROM_M45_INELIGIBLE_VIEW;
  else if (request->x_view.logical_rows != request->tokens ||
           request->y_view.logical_rows != request->tokens ||
           request->x_view.logical_columns != request->model_width ||
           request->y_view.logical_columns != request->model_width)
    reason = PROM_M45_INELIGIBLE_SHAPE;
  else if (request->x_view.row_stride_elements < request->model_width ||
           request->y_view.row_stride_elements < request->model_width)
    reason = PROM_M45_INELIGIBLE_STRIDE;
  else if (request->expected_x_generation == 0u || request->expected_y_generation == 0u ||
           request->x_view.owning_lifetime_id != request->expected_x_generation ||
           request->y_view.owning_lifetime_id != request->expected_y_generation ||
           request->x_view.owning_slot_generation == 0u ||
           request->y_view.owning_slot_generation == 0u)
    reason = PROM_M45_INELIGIBLE_GENERATION;
  else if (request->x_view.owning_device != request->y_view.owning_device)
    reason = PROM_M45_INELIGIBLE_DEVICE;
  else if (prom_m45_used_ranges_overlap(&request->x_view, x_bytes, &request->y_view, y_bytes))
    reason = PROM_M45_INELIGIBLE_ALIAS;
  else if (request->precision_policy != PROM_M45_PRECISION_FP32)
    reason = PROM_M45_INELIGIBLE_PRECISION;
  else if (request->strategy == PROM_M45_STRATEGY_IN_PLACE_X_AUDIT)
    reason = PROM_M45_INELIGIBLE_IN_PLACE_X;
  else if (request->strategy == PROM_M45_STRATEGY_IN_PLACE_Y &&
           (request->y_exclusive == 0u || request->pre_residual_y_consumer_count != 0u))
    reason = PROM_M45_INELIGIBLE_EXCLUSIVITY;
  out_plan->memory.x_view_bytes = x_bytes;
  out_plan->memory.y_view_bytes = y_bytes;
  out_plan->memory.z_device_bytes = request->strategy == PROM_M45_STRATEGY_SEPARATE_OUTPUT
                                      ? compact_bytes : 0u;
  out_plan->memory.z_readback_bytes = request->final_readback != 0u ? compact_bytes : 0u;
  out_plan->memory.in_place_y_saved_bytes = compact_bytes;
  if (!prom_m43_checked_add_u64(x_bytes, y_bytes, &total) ||
      !prom_m43_checked_add_u64(total, out_plan->memory.z_device_bytes, &total) ||
      !prom_m43_checked_add_u64(total, out_plan->memory.z_readback_bytes, &total)) return PROM_ERROR;
  out_plan->memory.exact_request_bytes = total;
  if (reason == PROM_M45_ELIGIBLE && total > PROM_M45_CAPACITY_LIMIT_BYTES)
    reason = PROM_M45_INELIGIBLE_CAPACITY;
  eligibility_hash = prom_reduction_hash_u32(eligibility_hash, request->tokens);
  eligibility_hash = prom_reduction_hash_u32(eligibility_hash, request->model_width);
  eligibility_hash = prom_reduction_hash_u32(eligibility_hash, request->x_view.row_stride_elements);
  eligibility_hash = prom_reduction_hash_u32(eligibility_hash, request->y_view.row_stride_elements);
  eligibility_hash = prom_reduction_hash_u32(eligibility_hash, request->strategy);
  eligibility_hash = prom_reduction_hash_u32(eligibility_hash, request->submit_policy);
  eligibility_hash = prom_reduction_hash_u32(eligibility_hash, request->precision_policy);
  eligibility_hash = prom_reduction_hash_u32(eligibility_hash, request->y_exclusive);
  eligibility_hash = prom_reduction_hash_u32(eligibility_hash, request->pre_residual_y_consumer_count);
  eligibility_hash = prom_reduction_hash_u32(eligibility_hash, reason);
  eligibility_hash = prom_m40b_hash_u64(eligibility_hash, total);
  out_plan->eligibility.reason = reason;
  out_plan->eligibility.eligible = reason == PROM_M45_ELIGIBLE ? 1u : 0u;
  out_plan->eligibility.replay_id = eligibility_hash;
  if (reason != PROM_M45_ELIGIBLE) return PROM_OK;
  prom_m45_add_barrier(out_plan, PROM_M45_BUFFER_X, request->x_view.offset, x_bytes,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_ACCESS_SHADER_READ_BIT, VK_ACCESS_SHADER_READ_BIT);
  prom_m45_add_stage(out_plan, PROM_M45_STAGE_X_READY, 0u, 0u, 1u, 0u,
                     PROM_M44_QUERY_PROJECTION_END, PROM_M45_QUERY_RESIDUAL_BEGIN);
  prom_m45_add_barrier(out_plan, PROM_M45_BUFFER_Y, request->y_view.offset, y_bytes,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_ACCESS_SHADER_WRITE_BIT,
                       request->strategy == PROM_M45_STRATEGY_IN_PLACE_Y
                         ? VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT
                         : VK_ACCESS_SHADER_READ_BIT);
  prom_m45_add_stage(out_plan, PROM_M45_STAGE_Y_READY, 0u, 1u, 1u, 0u,
                     PROM_M44_QUERY_PROJECTION_END, PROM_M45_QUERY_RESIDUAL_BEGIN);
  prom_m45_add_barrier(out_plan, PROM_M45_BUFFER_Z, 0u,
                       request->strategy == PROM_M45_STRATEGY_IN_PLACE_Y ? y_bytes : compact_bytes,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       request->final_readback != 0u ? VK_PIPELINE_STAGE_TRANSFER_BIT
                                                     : VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_ACCESS_SHADER_WRITE_BIT,
                       request->final_readback != 0u ? VK_ACCESS_TRANSFER_READ_BIT
                                                     : VK_ACCESS_SHADER_READ_BIT);
  prom_m45_add_stage(out_plan, PROM_M45_STAGE_RESIDUAL_ADD, 1u, 2u, 1u, 0u,
                     PROM_M45_QUERY_RESIDUAL_BEGIN, PROM_M45_QUERY_RESIDUAL_END);
  if (request->final_readback != 0u) {
    prom_m45_add_barrier(out_plan, PROM_M45_BUFFER_READBACK, 0u, compact_bytes,
                         VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_HOST_BIT,
                         VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_HOST_READ_BIT);
    prom_m45_add_stage(out_plan, PROM_M45_STAGE_FINAL_READBACK, 0u, 3u, 1u,
                       request->tokens, PROM_M45_QUERY_READBACK_BEGIN,
                       PROM_M45_QUERY_READBACK_END);
  }
  command_hash = prom_reduction_hash_u32(command_hash, out_plan->submit_policy);
  command_hash = prom_reduction_hash_u32(command_hash, out_plan->physical_alias_plan);
  command_hash = prom_reduction_hash_u32(command_hash, out_plan->stage_count);
  command_hash = prom_reduction_hash_u32(command_hash, out_plan->barrier_count);
  command_hash = prom_reduction_hash_u32(command_hash, out_plan->dispatch_count);
  command_hash = prom_reduction_hash_u32(command_hash, out_plan->copy_region_count);
  for (uint32_t index = 0u; index < out_plan->barrier_count; ++index) {
    const prom_m45_barrier_trace* barrier = &out_plan->barriers[index];
    command_hash = prom_reduction_hash_u32(command_hash, barrier->buffer_identity);
    command_hash = prom_m40b_hash_u64(command_hash, barrier->byte_offset);
    command_hash = prom_m40b_hash_u64(command_hash, barrier->byte_length);
    command_hash = prom_reduction_hash_u32(command_hash, barrier->source_access_mask);
    command_hash = prom_reduction_hash_u32(command_hash, barrier->destination_access_mask);
  }
  out_plan->command_plan_replay_id = command_hash;
  replay_hash = prom_reduction_hash_u32(replay_hash, request->tokens);
  replay_hash = prom_reduction_hash_u32(replay_hash, request->model_width);
  replay_hash = prom_reduction_hash_u32(replay_hash, request->x_view.row_stride_elements);
  replay_hash = prom_reduction_hash_u32(replay_hash, request->y_view.row_stride_elements);
  replay_hash = prom_reduction_hash_u32(replay_hash, request->strategy);
  replay_hash = prom_reduction_hash_u32(replay_hash, out_plan->physical_alias_plan);
  replay_hash = prom_m40b_hash_u64(replay_hash, request->expected_x_generation);
  replay_hash = prom_m40b_hash_u64(replay_hash, request->expected_y_generation);
  replay_hash = prom_m40b_hash_u64(replay_hash, PROM_M45_RESIDUAL_SHADER_HASH);
  replay_hash = prom_m40b_hash_u64(replay_hash, request->m44_replay_id);
  replay_hash = prom_m40b_hash_u64(replay_hash, command_hash);
  out_plan->replay_id = replay_hash;
  out_plan->z_generation = prom_m40b_hash_u64(replay_hash, request->expected_x_generation ^
                                                           request->expected_y_generation);
  if (out_plan->z_generation == 0u) out_plan->z_generation = 1u;
  (void)detail;
  return PROM_OK;
}

int prom_m45_residual_cpu_reference(const prom_m45_reference_request* request) {
  uint64_t x_count;
  uint64_t y_count;
  uint64_t z_count;
  uint32_t token;
  if (request == NULL || request->x == NULL || request->y == NULL || request->z == NULL ||
      request->tokens == 0u || request->tokens > PROM_M42_MAX_TOKENS ||
      request->model_width == 0u || request->model_width > PROM_M42_MAX_MODEL_WIDTH ||
      request->x_row_stride < request->model_width || request->y_row_stride < request->model_width ||
      request->z_row_stride < request->model_width ||
      !prom_m40b_checked_product_u64(request->tokens, request->x_row_stride, &x_count) ||
      !prom_m40b_checked_product_u64(request->tokens, request->y_row_stride, &y_count) ||
      !prom_m40b_checked_product_u64(request->tokens, request->z_row_stride, &z_count) ||
      request->x_element_count < x_count || request->y_element_count < y_count ||
      request->z_element_count < z_count) return PROM_ERROR;
  for (token = 0u; token < request->tokens; ++token) {
    uint32_t column;
    for (column = 0u; column < request->model_width; ++column) {
      const float x = request->x[(uint64_t)token * request->x_row_stride + column];
      const float y = request->y[(uint64_t)token * request->y_row_stride + column];
      const float z = x + y;
      if (!isfinite(x) || !isfinite(y) || !isfinite(z)) return PROM_ERROR;
      request->z[(uint64_t)token * request->z_row_stride + column] = z;
    }
  }
  return PROM_OK;
}

int prom_m45_residual_compare(const float* expected,
                              const float* actual,
                              uint32_t tokens,
                              uint32_t model_width,
                              float absolute_tolerance,
                              float relative_tolerance,
                              const prom_m45_residual_plan* plan,
                              prom_m45_mismatch* out_mismatch) {
  uint32_t token;
  if (out_mismatch == NULL) return PROM_ERROR;
  memset(out_mismatch, 0, sizeof(*out_mismatch));
  out_mismatch->matched = 1u;
  if (plan != NULL) {
    out_mismatch->strategy = plan->strategy;
    out_mismatch->x_generation = plan->x_generation;
    out_mismatch->y_generation = plan->y_generation;
    out_mismatch->z_generation = plan->z_generation;
    out_mismatch->m44_replay_id = plan->m44_replay_id;
    out_mismatch->m45_replay_id = plan->replay_id;
  }
  if (expected == NULL || actual == NULL || plan == NULL || tokens == 0u || model_width == 0u ||
      absolute_tolerance < 0.0f || relative_tolerance < 0.0f) return PROM_ERROR;
  for (token = 0u; token < tokens; ++token) {
    uint32_t column;
    for (column = 0u; column < model_width; ++column) {
      const uint64_t index = (uint64_t)token * model_width + column;
      const float absolute_error = fabsf(expected[index] - actual[index]);
      const float relative_error = absolute_error / fmaxf(fabsf(expected[index]), 1.0e-12f);
      if (!isfinite(expected[index]) || !isfinite(actual[index]) ||
          (absolute_error > absolute_tolerance && relative_error > relative_tolerance)) {
        out_mismatch->matched = 0u;
        out_mismatch->token = token;
        out_mismatch->column = column;
        out_mismatch->expected = expected[index];
        out_mismatch->actual = actual[index];
        out_mismatch->absolute_error = absolute_error;
        out_mismatch->relative_error = relative_error;
        return PROM_ERROR;
      }
    }
  }
  return PROM_OK;
}

static void prom_m46_add_barrier(prom_m46_rmsnorm_plan* plan,
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

static void prom_m46_add_stage(prom_m46_rmsnorm_plan* plan,
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
                                prom_m46_rmsnorm_plan* out_plan) {
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
  out_plan->memory.weight_upload_bytes = (uint64_t)request->model_width * sizeof(float);
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
  out_plan->memory.n_readback_bytes = request->final_readback != 0u ? compact_bytes : 0u;
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
                       request->final_readback != 0u ? VK_PIPELINE_STAGE_TRANSFER_BIT
                                                     : VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_ACCESS_SHADER_WRITE_BIT,
                       request->final_readback != 0u ? VK_ACCESS_TRANSFER_READ_BIT
                                                     : VK_ACCESS_SHADER_READ_BIT);
  prom_m46_add_stage(out_plan, PROM_M46_STAGE_APPLY, 1u, out_plan->barrier_count - 2u,
                     2u, 0u, 3u, 4u);
  if (request->final_readback != 0u) {
    prom_m46_add_barrier(out_plan, PROM_M46_BUFFER_READBACK, 0u, compact_bytes,
                         VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_HOST_BIT,
                         VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_HOST_READ_BIT);
    prom_m46_add_stage(out_plan, PROM_M46_STAGE_FINAL_READBACK, 0u,
                       out_plan->barrier_count - 1u, 1u, request->tokens, 4u, 5u);
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
                             const prom_m46_rmsnorm_plan* plan,
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
  static const char* const variants[PROM_M42_PIPELINE_COUNT] = {
      "kernel-57-default", "kernel-58-default", "kernel-59-default"};
  uint32_t index;
  if (state == NULL) return 0;
  for (index = 0u; index < PROM_M42_PIPELINE_COUNT; ++index) {
    prom_reduction_pipeline* destination = &state->m42_pipelines[index];
    const char* entry_point = NULL;
    prom_shader_package_diagnostic package_diagnostic;
    VkPipelineShaderStageCreateInfo stage_info;
    VkComputePipelineCreateInfo pipeline_info;
    if (destination->pipeline != VK_NULL_HANDLE) continue;
    if (state->shader_package == NULL ||
        !prom_shader_package_create_module(state->shader_package, state->device, variants[index],
                                           &destination->shader_module, &entry_point, &package_diagnostic)) return 0;
    memset(&stage_info, 0, sizeof(stage_info));
    stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
    stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
    stage_info.module = destination->shader_module;
    stage_info.pName = entry_point;
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

static int prom_m44_ensure_pipelines(prom_reduction_runtime_state* state) {
  static const char* const variants[PROM_M44_PIPELINE_COUNT] = {
      "kernel-60-default", "kernel-61-default"};
  uint32_t index;
  if (state == NULL || state->m44_pipeline_layout == VK_NULL_HANDLE) return 0;
  for (index = 0u; index < PROM_M44_PIPELINE_COUNT; ++index) {
    prom_reduction_pipeline* destination = &state->m44_pipelines[index];
    const char* entry_point = NULL;
    prom_shader_package_diagnostic package_diagnostic;
    VkPipelineShaderStageCreateInfo stage_info;
    VkComputePipelineCreateInfo pipeline_info;
    if (destination->pipeline != VK_NULL_HANDLE) continue;
    if (state->shader_package == NULL ||
        !prom_shader_package_create_module(state->shader_package, state->device, variants[index],
                                           &destination->shader_module, &entry_point, &package_diagnostic)) return 0;
    memset(&stage_info, 0, sizeof(stage_info));
    stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
    stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
    stage_info.module = destination->shader_module;
    stage_info.pName = entry_point;
    memset(&pipeline_info, 0, sizeof(pipeline_info));
    pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
    pipeline_info.stage = stage_info;
    pipeline_info.layout = state->m44_pipeline_layout;
    if (vkCreateComputePipelines(state->device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL,
                                 &destination->pipeline) != VK_SUCCESS) {
      prom_reduction_destroy_pipeline(state->device, destination);
      return 0;
    }
    destination->implementation_id = index + 1u;
    state->m44_pipeline_create_count += 1u;
  }
  return 1;
}

static int prom_m45_ensure_pipeline(prom_reduction_runtime_state* state) {
  prom_reduction_pipeline* destination;
  const char* entry_point = NULL;
  prom_shader_package_diagnostic package_diagnostic;
  VkPipelineShaderStageCreateInfo stage_info;
  VkComputePipelineCreateInfo pipeline_info;
  if (state == NULL || state->pipeline_layout == VK_NULL_HANDLE) return 0;
  destination = &state->m45_pipelines[0];
  if (destination->pipeline != VK_NULL_HANDLE) return 1;
  if (state->shader_package == NULL ||
      !prom_shader_package_create_module(state->shader_package, state->device, "kernel-62-default",
                                         &destination->shader_module, &entry_point, &package_diagnostic)) return 0;
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = destination->shader_module;
  stage_info.pName = entry_point;
  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = state->pipeline_layout;
  if (vkCreateComputePipelines(state->device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL,
                               &destination->pipeline) != VK_SUCCESS) {
    prom_reduction_destroy_pipeline(state->device, destination);
    return 0;
  }
  destination->implementation_id = 1u;
  state->m45_pipeline_create_count += 1u;
  return 1;
}

static int prom_m46_ensure_pipelines(prom_reduction_runtime_state* state) {
  static const char* const variants[PROM_M46_PIPELINE_COUNT] = {
      "kernel-63-default", "kernel-64-default"};
  uint32_t index;
  if (state == NULL || state->pipeline_layout == VK_NULL_HANDLE) return 0;
  for (index = 0u; index < PROM_M46_PIPELINE_COUNT; ++index) {
    prom_reduction_pipeline* destination = &state->m46_pipelines[index];
    const char* entry_point = NULL;
    prom_shader_package_diagnostic package_diagnostic;
    VkPipelineShaderStageCreateInfo stage_info;
    VkComputePipelineCreateInfo pipeline_info;
    if (destination->pipeline != VK_NULL_HANDLE) continue;
    if (state->shader_package == NULL ||
        !prom_shader_package_create_module(state->shader_package, state->device, variants[index],
                                           &destination->shader_module, &entry_point, &package_diagnostic)) return 0;
    memset(&stage_info, 0, sizeof(stage_info));
    stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
    stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
    stage_info.module = destination->shader_module;
    stage_info.pName = entry_point;
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
    state->m46_pipeline_create_count += 1u;
  }
  return 1;
}

static int prom_m46_ensure_buffer(prom_reduction_runtime_state* state,
                                  prom_vk_buffer* buffer,
                                  VkDeviceSize size,
                                  VkBufferUsageFlags usage,
                                  VkMemoryPropertyFlags properties,
                                  int map_memory) {
  const uint64_t allocations_before = state->diagnostics.buffer_allocation_count;
  if (!prom_reduction_ensure_buffer(state, buffer, size, usage, properties, map_memory)) return 0;
  if (state->diagnostics.buffer_allocation_count != allocations_before)
    state->m46_buffer_grow_count += 1u;
  else
    state->m46_buffer_reuse_count += 1u;
  return 1;
}

static void prom_m46_update_descriptor(prom_reduction_runtime_state* state,
                                       VkDescriptorSet set,
                                       const prom_vk_buffer* input,
                                       const prom_vk_buffer* auxiliary0,
                                       const prom_vk_buffer* auxiliary1,
                                       const prom_vk_buffer* output) {
  prom_reduction_buffer_bindings bindings;
  bindings.input = input;
  bindings.auxiliary0 = auxiliary0;
  bindings.auxiliary1 = auxiliary1;
  bindings.output = output;
  prom_reduction_update_descriptor_set(state, set, &bindings);
  state->m46_descriptor_update_count += 1u;
}

static int prom_m47_ensure_pipelines(prom_reduction_runtime_state* state) {
  static const char* const variants[PROM_M47_PIPELINE_COUNT] = {
      "kernel-65-default", "kernel-66-default"};
  uint32_t index;
  if (state == NULL || state->pipeline_layout == VK_NULL_HANDLE) return 0;
  for (index = 0u; index < PROM_M47_PIPELINE_COUNT; ++index) {
    prom_reduction_pipeline* destination = &state->m47_pipelines[index];
    const char* entry_point = NULL;
    prom_shader_package_diagnostic package_diagnostic;
    VkPipelineShaderStageCreateInfo stage_info;
    VkComputePipelineCreateInfo pipeline_info;
    if (destination->pipeline != VK_NULL_HANDLE) continue;
    if (state->shader_package == NULL ||
        !prom_shader_package_create_module(state->shader_package, state->device, variants[index],
                                           &destination->shader_module, &entry_point, &package_diagnostic)) return 0;
    memset(&stage_info, 0, sizeof(stage_info));
    stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
    stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
    stage_info.module = destination->shader_module;
    stage_info.pName = entry_point;
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
    state->m47_pipeline_create_count += 1u;
  }
  return 1;
}

static int prom_m47_ensure_buffer(prom_reduction_runtime_state* state,
                                  prom_vk_buffer* buffer,
                                  VkDeviceSize size,
                                  VkBufferUsageFlags usage,
                                  VkMemoryPropertyFlags properties,
                                  int map_memory) {
  const uint64_t allocations_before = state->diagnostics.buffer_allocation_count;
  if (!prom_reduction_ensure_buffer(state, buffer, size, usage, properties, map_memory)) return 0;
  if (state->diagnostics.buffer_allocation_count != allocations_before)
    state->m47_buffer_grow_count += 1u;
  else
    state->m47_buffer_reuse_count += 1u;
  return 1;
}

static void prom_m47_update_descriptor(prom_reduction_runtime_state* state,
                                       VkDescriptorSet set,
                                       const prom_vk_buffer* input,
                                       const prom_vk_buffer* auxiliary0,
                                       const prom_vk_buffer* auxiliary1,
                                       const prom_vk_buffer* output) {
  prom_reduction_buffer_bindings bindings;
  bindings.input = input;
  bindings.auxiliary0 = auxiliary0;
  bindings.auxiliary1 = auxiliary1;
  bindings.output = output;
  prom_reduction_update_descriptor_set(state, set, &bindings);
  state->m47_descriptor_update_count += 1u;
}

static int prom_m45_ensure_buffer(prom_reduction_runtime_state* state,
                                  prom_vk_buffer* buffer,
                                  VkDeviceSize size,
                                  VkBufferUsageFlags usage,
                                  VkMemoryPropertyFlags properties,
                                  int map_memory) {
  const uint64_t allocations_before = state->diagnostics.buffer_allocation_count;
  if (!prom_reduction_ensure_buffer(state, buffer, size, usage, properties, map_memory)) return 0;
  if (state->diagnostics.buffer_allocation_count != allocations_before)
    state->m45_buffer_grow_count += 1u;
  else
    state->m45_buffer_reuse_count += 1u;
  return 1;
}

static void prom_m45_update_descriptor(prom_reduction_runtime_state* state,
                                       prom_reduction_slot* slot,
                                       const prom_vk_buffer* x,
                                       const prom_vk_buffer* y,
                                       const prom_vk_buffer* z) {
  prom_reduction_buffer_bindings bindings;
  bindings.input = x;
  bindings.auxiliary0 = y;
  bindings.auxiliary1 = z;
  bindings.output = z;
  prom_reduction_update_descriptor_set(state, slot->m45_descriptor_set, &bindings);
  state->m45_descriptor_update_count += 1u;
}

static int prom_m44_ensure_buffer(prom_reduction_runtime_state* state,
                                  prom_vk_buffer* buffer,
                                  VkDeviceSize size,
                                  VkBufferUsageFlags usage,
                                  VkMemoryPropertyFlags properties,
                                  int map_memory,
                                  uint32_t* out_reused) {
  uint64_t allocations_before;
  if (state == NULL || buffer == NULL || size == 0u) return 0;
  allocations_before = state->diagnostics.buffer_allocation_count;
  if (!prom_reduction_ensure_buffer(state, buffer, size, usage, properties, map_memory)) return 0;
  if (state->diagnostics.buffer_allocation_count != allocations_before) {
    state->m44_buffer_grow_count += 1u;
    if (out_reused != NULL) *out_reused = 0u;
  } else {
    state->m44_buffer_reuse_count += 1u;
    if (out_reused != NULL) *out_reused = 1u;
  }
  return 1;
}

static void prom_m44_update_wide_descriptor(prom_reduction_runtime_state* state,
                                            VkDescriptorSet set,
                                            const prom_vk_buffer* const* buffers) {
  VkDescriptorBufferInfo infos[PROM_M44_WIDE_DESCRIPTOR_BINDING_COUNT];
  VkWriteDescriptorSet writes[PROM_M44_WIDE_DESCRIPTOR_BINDING_COUNT];
  uint32_t index;
  memset(infos, 0, sizeof(infos));
  memset(writes, 0, sizeof(writes));
  for (index = 0u; index < PROM_M44_WIDE_DESCRIPTOR_BINDING_COUNT; ++index) {
    infos[index].buffer = buffers[index]->buffer;
    infos[index].offset = 0u;
    infos[index].range = buffers[index]->size;
    writes[index].sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET;
    writes[index].dstSet = set;
    writes[index].dstBinding = index;
    writes[index].descriptorCount = 1u;
    writes[index].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
    writes[index].pBufferInfo = &infos[index];
  }
  vkUpdateDescriptorSets(state->device, PROM_M44_WIDE_DESCRIPTOR_BINDING_COUNT, writes, 0u, NULL);
  state->m44_descriptor_update_count += 1u;
}

static void prom_m44_update_sgemm_descriptor(prom_reduction_runtime_state* state,
                                             VkDescriptorSet set,
                                             const prom_vk_buffer* input,
                                             const prom_vk_buffer* weight,
                                             const prom_vk_buffer* output) {
  prom_reduction_buffer_bindings bindings;
  bindings.input = input;
  bindings.auxiliary0 = weight;
  bindings.auxiliary1 = output;
  bindings.output = output;
  prom_reduction_update_descriptor_set(state, set, &bindings);
  state->m44_descriptor_update_count += 1u;
}

static uint64_t prom_m44_retained_bytes(const prom_reduction_runtime_state* state,
                                        const prom_reduction_slot* slot) {
  return (uint64_t)state->m44_wo_upload.size + (uint64_t)state->m44_wo_f32.size +
         (uint64_t)state->m44_wo_f16.size + (uint64_t)slot->m44_concat_upload.size +
         (uint64_t)slot->m44_concat_f32.size + (uint64_t)slot->m44_concat_f16.size +
         (uint64_t)slot->m44_output.size + (uint64_t)slot->m44_readback.size;
}

static uint64_t prom_m45_retained_bytes(const prom_reduction_slot* slot) {
  return slot != NULL ? (uint64_t)slot->m45_output.size : 0u;
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
      (services.cooperative_matrix_state >= PROM_VK_COOPERATIVE_MATRIX_DEVICE_FEATURE_ENABLED &&
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
                        slot->active_query_base + query_offset);
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

static int prom_m43_ensure_buffer(prom_reduction_runtime_state* state,
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
    state->m43_buffer_grow_count += 1u;
    if (out_reused != NULL) *out_reused = 0u;
  } else {
    state->m43_buffer_reuse_count += 1u;
    if (out_reused != NULL) *out_reused = 1u;
  }
  return 1;
}

static void prom_m43_update_descriptor(prom_reduction_runtime_state* state,
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
  state->m43_descriptor_update_count += 1u;
}

static uint64_t prom_m43_weight_retained_bytes(const prom_reduction_runtime_state* state) {
  uint64_t total = 0u;
  uint32_t head;
  uint32_t weight;
  for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
    for (weight = 0u; weight < PROM_M43_WEIGHT_KIND_COUNT; ++weight) {
      total += (uint64_t)state->m43_weight_upload[head][weight].size;
      total += (uint64_t)state->m43_weight_f32[head][weight].size;
      total += (uint64_t)state->m43_weight_f16[head][weight].size;
    }
  }
  return total;
}

static int prom_m43_record_upload_and_pack(prom_reduction_runtime_state* state,
                                           prom_reduction_slot* slot,
                                           const prom_vk_buffer* upload,
                                           const prom_vk_buffer* f32,
                                           const prom_vk_buffer* f16,
                                           uint32_t logical_rows,
                                           uint32_t logical_columns,
                                           uint32_t padded_rows,
                                           uint32_t padded_columns,
                                           VkDeviceSize f32_bytes,
                                           uint64_t* out_gpu_ns,
                                           uint64_t* out_wall_ns) {
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  VkSubmitInfo submit;
  VkResult result;
  uint64_t timestamps[2] = {0u, 0u};
  uint64_t begin_ns;
  if (vkResetCommandBuffer(slot->command_buffer, 0u) != VK_SUCCESS) return 0;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(slot->command_buffer, &begin_info) != VK_SUCCESS) return 0;
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
    vkCmdResetQueryPool(slot->command_buffer, state->query_pool,
                        slot->slot_id * PROM_REDUCTION_QUERY_STRIDE, 2u);
    vkCmdWriteTimestamp(slot->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, state->query_pool,
                        slot->slot_id * PROM_REDUCTION_QUERY_STRIDE);
  }
  prom_m42_buffer_barrier(slot->command_buffer, upload,
                          VK_ACCESS_HOST_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                          VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
  memset(&copy, 0, sizeof(copy));
  copy.size = f32_bytes;
  vkCmdCopyBuffer(slot->command_buffer, upload->buffer, f32->buffer, 1u, &copy);
  prom_m42_buffer_barrier(slot->command_buffer, f32,
                          VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                          VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
  prom_m42_record_pack(state, slot->command_buffer, slot->m43_descriptor_sets[0],
                       logical_rows, logical_columns, logical_columns,
                       padded_rows, padded_columns, 0u);
  prom_m42_buffer_barrier(slot->command_buffer, f16,
                          VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                          VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
    vkCmdWriteTimestamp(slot->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, state->query_pool,
                        slot->slot_id * PROM_REDUCTION_QUERY_STRIDE + 1u);
  }
  if (vkEndCommandBuffer(slot->command_buffer) != VK_SUCCESS ||
      vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS) return 0;
  memset(&submit, 0, sizeof(submit));
  submit.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit.commandBufferCount = 1u;
  submit.pCommandBuffers = &slot->command_buffer;
  begin_ns = prom_reduction_now_ns();
  result = vkQueueSubmit(state->queue, 1u, &submit, slot->fence);
  if (result != VK_SUCCESS) return 0;
  slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
  result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
  if (out_wall_ns != NULL) *out_wall_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  if (result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    return 0;
  }
  if (out_gpu_ns != NULL && state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE &&
      vkGetQueryPoolResults(state->device, state->query_pool,
                            slot->slot_id * PROM_REDUCTION_QUERY_STRIDE, 2u,
                            sizeof(timestamps), timestamps, sizeof(uint64_t),
                            VK_QUERY_RESULT_64_BIT) == VK_SUCCESS && timestamps[1] >= timestamps[0]) {
    *out_gpu_ns = (uint64_t)((double)(timestamps[1] - timestamps[0]) * state->timestamp_period_ns);
  }
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  return 1;
}

int prom_reactor_runtime_m43_prepare_weight(void* handle,
                                            const prom_m43_weight_prepare_request* request,
                                            prom_m43_weight_prepare_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_reduction_slot* slot;
  uint32_t padded_model;
  uint32_t padded_head;
  uint64_t element_count;
  VkDeviceSize f32_bytes;
  VkDeviceSize f16_bytes;
  uint32_t finite = 0u;
  uint32_t reused_upload = 0u;
  uint32_t reused_f32 = 0u;
  uint32_t reused_f16 = 0u;
  uint64_t begin_ns;
  uint32_t had_previous;
  int32_t detail = 0;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  if (request == NULL || request->values == NULL || request->head_index >= PROM_M43_HEAD_COUNT ||
      request->weight_kind >= PROM_M43_WEIGHT_KIND_COUNT || request->generation == 0u ||
      request->model_width == 0u || request->head_dim == 0u ||
      !prom_m42_round_up_16(request->model_width, &padded_model) ||
      !prom_m42_round_up_16(request->head_dim, &padded_head) ||
      !prom_m40b_checked_product_u64(request->model_width, request->head_dim, &element_count) ||
      request->element_count != element_count || element_count > PROM_M42_MAX_MATRIX_ELEMENTS ||
      element_count > SIZE_MAX / sizeof(float)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M43_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  out_result->head_index = request->head_index;
  out_result->weight_kind = request->weight_kind;
  begin_ns = prom_reduction_now_ns();
  out_result->hash = prom_m42_hash_finite_matrix(request->values, element_count, &finite);
  out_result->validation_hash_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  if (finite == 0u) {
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M43_DETAIL_NONFINITE_INPUT;
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = detail;
    return PROM_ERROR;
  }
  if (request->generation <= state->m43_weight_generation[request->head_index][request->weight_kind]) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M43_DETAIL_STALE_WEIGHT_GENERATION;
    return PROM_ERROR;
  }
  had_previous = state->m43_weight_generation[request->head_index][request->weight_kind] != 0u;
  if (!prom_m40b_wait_all_slots(state) || !prom_m42_ensure_pipelines(state)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M43_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  slot = prom_reduction_acquire_slot(state, state->next_logical_request_id++);
  state->diagnostics.next_logical_request_id = state->next_logical_request_id;
  if (slot == NULL) {
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M43_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  f32_bytes = (VkDeviceSize)(element_count * sizeof(float));
  f16_bytes = (VkDeviceSize)((((uint64_t)padded_model * padded_head + 1u) / 2u) * sizeof(uint32_t));
  if (!prom_m43_ensure_buffer(state,
                              &state->m43_weight_upload[request->head_index][request->weight_kind],
                              f32_bytes, VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                              1, &reused_upload) ||
      !prom_m43_ensure_buffer(state,
                              &state->m43_weight_f32[request->head_index][request->weight_kind],
                              f32_bytes, VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &reused_f32) ||
      !prom_m43_ensure_buffer(state,
                              &state->m43_weight_f16[request->head_index][request->weight_kind],
                              f16_bytes, VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &reused_f16)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M43_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  memcpy(state->m43_weight_upload[request->head_index][request->weight_kind].mapped,
         request->values, (size_t)f32_bytes);
  prom_m43_update_descriptor(state, slot->m43_descriptor_sets[0],
                             &state->m43_weight_f32[request->head_index][request->weight_kind], NULL,
                             &state->m43_weight_f16[request->head_index][request->weight_kind]);
  if (!prom_m43_record_upload_and_pack(
          state, slot,
          &state->m43_weight_upload[request->head_index][request->weight_kind],
          &state->m43_weight_f32[request->head_index][request->weight_kind],
          &state->m43_weight_f16[request->head_index][request->weight_kind],
          request->model_width, request->head_dim, padded_model, padded_head,
          f32_bytes, &out_result->gpu_upload_and_pack_ns, &out_result->upload_and_pack_ns)) {
    if (slot->state != PROM_ASYNC_PHYSICAL_QUARANTINED) slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = slot->state == PROM_ASYNC_PHYSICAL_QUARANTINED
                                  ? PROM_M43_DETAIL_COMPLETION_UNCERTAIN
                                  : PROM_M43_DETAIL_COMMAND;
    return PROM_ERROR;
  }
  state->m43_weight_generation[request->head_index][request->weight_kind] = request->generation;
  state->m43_weight_hash[request->head_index][request->weight_kind] = out_result->hash;
  state->m43_weight_model_width[request->head_index][request->weight_kind] = request->model_width;
  state->m43_weight_head_dim[request->head_index][request->weight_kind] = request->head_dim;
  out_result->generation = request->generation;
  out_result->replaced = had_previous;
  out_result->buffer_reused = reused_upload != 0u && reused_f32 != 0u && reused_f16 != 0u;
  out_result->retained_bytes = prom_m43_weight_retained_bytes(state);
  return PROM_OK;
}

int prom_reactor_runtime_m43_prepare_resident_x(void* handle,
                                                const prom_m43_resident_x_prepare_request* request,
                                                prom_m43_resident_x_prepare_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_reduction_slot* slot;
  uint32_t padded_tokens;
  uint32_t padded_model;
  uint32_t padded_head;
  uint64_t element_count;
  VkDeviceSize f32_bytes;
  VkDeviceSize f16_bytes;
  uint32_t finite = 0u;
  uint32_t reused_upload = 0u;
  uint32_t reused_f32 = 0u;
  uint32_t reused_f16 = 0u;
  uint64_t begin_ns;
  int32_t detail = 0;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  if (request == NULL || request->x == NULL || request->generation == 0u ||
      !prom_m42_valid_shape(request->tokens, request->model_width, 1u, 1u,
                            &padded_tokens, &padded_model, &padded_head) ||
      !prom_m40b_checked_product_u64(request->tokens, request->model_width, &element_count) ||
      request->element_count != element_count || element_count > SIZE_MAX / sizeof(float)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M43_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  (void)padded_head;
  begin_ns = prom_reduction_now_ns();
  out_result->hash = prom_m42_hash_finite_matrix(request->x, element_count, &finite);
  out_result->validation_hash_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  if (finite == 0u) {
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M43_DETAIL_NONFINITE_INPUT;
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = detail;
    return PROM_ERROR;
  }
  if (request->generation <= state->m43_resident_x_generation) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M43_DETAIL_STALE_X_GENERATION;
    return PROM_ERROR;
  }
  if (!prom_m40b_wait_all_slots(state) || !prom_m42_ensure_pipelines(state)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M43_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  slot = prom_reduction_acquire_slot(state, state->next_logical_request_id++);
  state->diagnostics.next_logical_request_id = state->next_logical_request_id;
  if (slot == NULL) {
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M43_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  f32_bytes = (VkDeviceSize)(element_count * sizeof(float));
  f16_bytes = (VkDeviceSize)((((uint64_t)padded_tokens * padded_model + 1u) / 2u) * sizeof(uint32_t));
  if (!prom_m43_ensure_buffer(state, &state->m43_resident_x_upload, f32_bytes,
                              VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                              1, &reused_upload) ||
      !prom_m43_ensure_buffer(state, &state->m43_resident_x_f32, f32_bytes,
                              VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT |
                                  VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &reused_f32) ||
      !prom_m43_ensure_buffer(state, &state->m43_resident_x_f16, f16_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &reused_f16)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M43_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  memcpy(state->m43_resident_x_upload.mapped, request->x, (size_t)f32_bytes);
  prom_m43_update_descriptor(state, slot->m43_descriptor_sets[0],
                             &state->m43_resident_x_f32, NULL, &state->m43_resident_x_f16);
  if (!prom_m43_record_upload_and_pack(state, slot, &state->m43_resident_x_upload,
                                       &state->m43_resident_x_f32, &state->m43_resident_x_f16,
                                       request->tokens, request->model_width, padded_tokens, padded_model,
                                       f32_bytes, &out_result->gpu_upload_and_pack_ns,
                                       &out_result->upload_and_pack_ns)) {
    if (slot->state != PROM_ASYNC_PHYSICAL_QUARANTINED) slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = slot->state == PROM_ASYNC_PHYSICAL_QUARANTINED
                                  ? PROM_M43_DETAIL_COMPLETION_UNCERTAIN
                                  : PROM_M43_DETAIL_COMMAND;
    return PROM_ERROR;
  }
  out_result->replaced = state->m43_resident_x_generation != 0u;
  state->m43_resident_x_generation = request->generation;
  state->m43_resident_x_hash = out_result->hash;
  state->m43_resident_x_tokens = request->tokens;
  state->m43_resident_x_model_width = request->model_width;
  out_result->generation = request->generation;
  out_result->buffer_reused = reused_upload != 0u && reused_f32 != 0u && reused_f16 != 0u;
  out_result->retained_bytes = (uint64_t)state->m43_resident_x_upload.size +
                               (uint64_t)state->m43_resident_x_f32.size +
                               (uint64_t)state->m43_resident_x_f16.size;
  return PROM_OK;
}

int prom_reactor_runtime_m44_prepare_wo(void* handle,
                                       const prom_m44_wo_prepare_request* request,
                                       prom_m44_wo_prepare_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_reduction_slot* slot;
  uint32_t concatenated_width;
  uint32_t padded_concat;
  uint32_t padded_model;
  uint32_t finite = 0u;
  uint32_t reused_upload = 0u;
  uint32_t reused_f32 = 0u;
  uint32_t reused_f16 = 0u;
  uint64_t elements;
  uint64_t begin_ns;
  uint64_t submit_begin;
  uint64_t timestamps[2] = {0u, 0u};
  VkDeviceSize f32_bytes;
  VkDeviceSize f16_bytes;
  int32_t detail = 0;
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  VkSubmitInfo submit;
  VkResult result;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  if (request == NULL || request->values == NULL || request->head_count != PROM_M44_HEAD_COUNT ||
      request->head_dim == 0u || request->head_dim > PROM_M42_MAX_HEAD_DIM ||
      request->model_width == 0u || request->model_width > PROM_M42_MAX_MODEL_WIDTH ||
      request->generation == 0u ||
      !prom_vk_checked_mul_u32(PROM_M44_HEAD_COUNT, request->head_dim, &concatenated_width) ||
      concatenated_width > 8192u || !prom_m42_round_up_16(concatenated_width, &padded_concat) ||
      !prom_m42_round_up_16(request->model_width, &padded_model) ||
      !prom_m40b_checked_product_u64(concatenated_width, request->model_width, &elements) ||
      request->element_count != elements || elements > SIZE_MAX / sizeof(float)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M44_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  begin_ns = prom_reduction_now_ns();
  out_result->hash = prom_m42_hash_finite_matrix(request->values, elements, &finite);
  out_result->validation_hash_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  if (finite == 0u) {
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M44_DETAIL_NONFINITE_INPUT;
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = detail;
    return PROM_ERROR;
  }
  if (request->generation <= state->m44_wo_generation) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M44_DETAIL_STALE_WO_GENERATION;
    return PROM_ERROR;
  }
  if (!prom_m40b_wait_all_slots(state) || !prom_m42_ensure_pipelines(state) ||
      !prom_m44_ensure_pipelines(state) ||
      !prom_m40b_ensure_sgemm_pipeline(state, PROM_M40B_KERNEL_A2X4) ||
      !prom_m40b_ensure_sgemm_pipeline(state, PROM_M40B_KERNEL_CONVENTIONAL_FP16)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M44_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  slot = prom_reduction_acquire_slot(state, state->next_logical_request_id++);
  state->diagnostics.next_logical_request_id = state->next_logical_request_id;
  if (slot == NULL) {
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M44_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  f32_bytes = (VkDeviceSize)(elements * sizeof(float));
  f16_bytes = (VkDeviceSize)((((uint64_t)padded_concat * padded_model + 1u) / 2u) * sizeof(uint32_t));
  if (!prom_m44_ensure_buffer(state, &state->m44_wo_upload, f32_bytes,
                              VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                              1, &reused_upload) ||
      !prom_m44_ensure_buffer(state, &state->m44_wo_f32, f32_bytes,
                              VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &reused_f32) ||
      !prom_m44_ensure_buffer(state, &state->m44_wo_f16, f16_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &reused_f16)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M44_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  memcpy(state->m44_wo_upload.mapped, request->values, (size_t)f32_bytes);
  prom_m44_update_sgemm_descriptor(state, slot->m44_sgemm_descriptor_set,
                                   &state->m44_wo_f32, &state->m44_wo_f32,
                                   &state->m44_wo_f16);
  if (vkResetCommandBuffer(slot->command_buffer, 0u) != VK_SUCCESS) goto m44_wo_command_fail;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(slot->command_buffer, &begin_info) != VK_SUCCESS) goto m44_wo_command_fail;
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
    vkCmdResetQueryPool(slot->command_buffer, state->query_pool,
                        slot->slot_id * PROM_REDUCTION_QUERY_STRIDE, 2u);
    vkCmdWriteTimestamp(slot->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, state->query_pool,
                        slot->slot_id * PROM_REDUCTION_QUERY_STRIDE);
  }
  prom_m42_buffer_barrier(slot->command_buffer, &state->m44_wo_upload,
                          VK_ACCESS_HOST_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                          VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
  memset(&copy, 0, sizeof(copy));
  copy.size = f32_bytes;
  vkCmdCopyBuffer(slot->command_buffer, state->m44_wo_upload.buffer,
                  state->m44_wo_f32.buffer, 1u, &copy);
  prom_m42_buffer_barrier(slot->command_buffer, &state->m44_wo_f32,
                          VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                          VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
  prom_m42_record_pack(state, slot->command_buffer, slot->m44_sgemm_descriptor_set,
                       concatenated_width, request->model_width, request->model_width,
                       padded_concat, padded_model, 0u);
  prom_m42_buffer_barrier(slot->command_buffer, &state->m44_wo_f16,
                          VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                          VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE)
    vkCmdWriteTimestamp(slot->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, state->query_pool,
                        slot->slot_id * PROM_REDUCTION_QUERY_STRIDE + 1u);
  if (vkEndCommandBuffer(slot->command_buffer) != VK_SUCCESS ||
      vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS) goto m44_wo_command_fail;
  memset(&submit, 0, sizeof(submit));
  submit.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit.commandBufferCount = 1u;
  submit.pCommandBuffers = &slot->command_buffer;
  submit_begin = prom_reduction_now_ns();
  result = vkQueueSubmit(state->queue, 1u, &submit, slot->fence);
  if (result != VK_SUCCESS) goto m44_wo_submit_fail;
  slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
  result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
  out_result->upload_and_pack_ns = prom_reduction_elapsed_ns(submit_begin, prom_reduction_now_ns());
  if (result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M44_DETAIL_COMPLETION_UNCERTAIN;
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
  out_result->replaced = state->m44_wo_generation != 0u;
  state->m44_wo_generation = request->generation;
  state->m44_wo_hash = out_result->hash;
  state->m44_wo_head_dim = request->head_dim;
  state->m44_wo_model_width = request->model_width;
  out_result->generation = request->generation;
  out_result->buffer_reused = reused_upload != 0u && reused_f32 != 0u && reused_f16 != 0u;
  out_result->retained_bytes = (uint64_t)state->m44_wo_upload.size +
                               (uint64_t)state->m44_wo_f32.size +
                               (uint64_t)state->m44_wo_f16.size;
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  return PROM_OK;

m44_wo_command_fail:
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->stage = PROM_STAGE_TRANSFER_IN;
  out_result->detail_code = PROM_M44_DETAIL_COMMAND;
  return PROM_ERROR;
m44_wo_submit_fail:
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->stage = PROM_STAGE_TRANSFER_IN;
  out_result->detail_code = PROM_M44_DETAIL_SUBMIT;
  return PROM_ERROR;
}

int prom_reactor_runtime_m46_prepare_weight(void* handle,
                                            const prom_m46_weight_prepare_request* request,
                                            prom_m46_weight_prepare_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_reduction_slot* slot;
  uint32_t finite = 0u;
  uint64_t begin_ns;
  VkDeviceSize bytes;
  int32_t detail = 0;
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  VkSubmitInfo submit;
  VkResult result;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  if (request == NULL || request->values == NULL || request->model_width == 0u ||
      request->model_width > PROM_M42_MAX_MODEL_WIDTH || request->generation == 0u ||
      request->element_count != request->model_width) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M46_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  begin_ns = prom_reduction_now_ns();
  out_result->hash = prom_m42_hash_finite_matrix(request->values, request->element_count, &finite);
  if (finite == 0u || out_result->hash == 0u) {
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M46_DETAIL_NONFINITE_INPUT;
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = detail;
    return PROM_ERROR;
  }
  if (request->generation <= state->m46_weight_generation) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M46_DETAIL_STALE_WEIGHT_GENERATION;
    return PROM_ERROR;
  }
  if (!prom_m40b_wait_all_slots(state) || !prom_m46_ensure_pipelines(state)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M46_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  slot = prom_reduction_acquire_slot(state, state->next_logical_request_id++);
  state->diagnostics.next_logical_request_id = state->next_logical_request_id;
  if (slot == NULL) {
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M46_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  bytes = (VkDeviceSize)((uint64_t)request->model_width * sizeof(float));
  if (!prom_m46_ensure_buffer(state, &state->m46_weight_upload, bytes,
                              VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                              1) ||
      !prom_m46_ensure_buffer(state, &state->m46_weight, bytes,
                              VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M46_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  memcpy(state->m46_weight_upload.mapped, request->values, (size_t)bytes);
  if (vkResetCommandBuffer(slot->command_buffer, 0u) != VK_SUCCESS) goto m46_weight_command_fail;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(slot->command_buffer, &begin_info) != VK_SUCCESS) goto m46_weight_command_fail;
  prom_m42_buffer_barrier(slot->command_buffer, &state->m46_weight_upload,
                          VK_ACCESS_HOST_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                          VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
  memset(&copy, 0, sizeof(copy));
  copy.size = bytes;
  vkCmdCopyBuffer(slot->command_buffer, state->m46_weight_upload.buffer,
                  state->m46_weight.buffer, 1u, &copy);
  prom_m42_buffer_barrier(slot->command_buffer, &state->m46_weight,
                          VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                          VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
  if (vkEndCommandBuffer(slot->command_buffer) != VK_SUCCESS ||
      vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS) goto m46_weight_command_fail;
  memset(&submit, 0, sizeof(submit));
  submit.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit.commandBufferCount = 1u;
  submit.pCommandBuffers = &slot->command_buffer;
  result = vkQueueSubmit(state->queue, 1u, &submit, slot->fence);
  if (result != VK_SUCCESS) goto m46_weight_submit_fail;
  slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
  result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
  if (result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M46_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  out_result->replaced = state->m46_weight_generation != 0u;
  state->m46_weight_generation = request->generation;
  state->m46_weight_hash = out_result->hash;
  state->m46_weight_model_width = request->model_width;
  out_result->generation = request->generation;
  out_result->retained_bytes = (uint64_t)state->m46_weight_upload.size +
                               (uint64_t)state->m46_weight.size;
  out_result->preparation_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  return PROM_OK;

m46_weight_command_fail:
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->stage = PROM_STAGE_TRANSFER_IN;
  out_result->detail_code = PROM_M46_DETAIL_COMMAND;
  return PROM_ERROR;
m46_weight_submit_fail:
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->stage = PROM_STAGE_TRANSFER_IN;
  out_result->detail_code = PROM_M46_DETAIL_SUBMIT;
  return PROM_ERROR;
}

int prom_reactor_runtime_m47_prepare_weight(void* handle,
                                            const prom_m47_weight_prepare_request* request,
                                            prom_m47_weight_prepare_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_reduction_slot* slot;
  uint32_t rows;
  uint32_t columns;
  uint32_t padded_rows;
  uint32_t padded_columns;
  uint32_t finite = 0u;
  uint32_t reused_upload = 0u;
  uint32_t reused_f32 = 0u;
  uint32_t reused_f16 = 0u;
  uint64_t elements;
  uint64_t begin_ns;
  uint64_t submit_begin;
  uint64_t timestamps[2] = {0u, 0u};
  VkDeviceSize f32_bytes;
  VkDeviceSize f16_bytes;
  int32_t detail = 0;
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  VkSubmitInfo submit;
  VkResult result;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  if (request == NULL || request->values == NULL || request->kind >= PROM_M47_WEIGHT_COUNT ||
      request->model_width == 0u || request->model_width > PROM_M42_MAX_MODEL_WIDTH ||
      request->ffn_width == 0u || request->ffn_width > 8192u || request->generation == 0u) {
    out_result->detail_code = PROM_M47_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  rows = request->kind == PROM_M47_WEIGHT_DOWN ? request->ffn_width : request->model_width;
  columns = request->kind == PROM_M47_WEIGHT_DOWN ? request->model_width : request->ffn_width;
  if (!prom_m42_round_up_16(rows, &padded_rows) ||
      !prom_m42_round_up_16(columns, &padded_columns) ||
      !prom_m40b_checked_product_u64(rows, columns, &elements) ||
      request->element_count != elements || elements > SIZE_MAX / sizeof(float)) {
    out_result->detail_code = PROM_M47_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  begin_ns = prom_reduction_now_ns();
  out_result->kind = request->kind;
  out_result->hash = prom_m42_hash_finite_matrix(request->values, elements, &finite);
  out_result->validation_hash_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  if (finite == 0u || out_result->hash == 0u) {
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M47_DETAIL_NONFINITE_INPUT;
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL) {
    out_result->detail_code = detail;
    return PROM_ERROR;
  }
  if (request->generation <= state->m47_weight_generation[request->kind]) {
    out_result->detail_code = PROM_M47_DETAIL_STALE_WEIGHT_GENERATION;
    return PROM_ERROR;
  }
  if (!prom_m40b_wait_all_slots(state) || !prom_m42_ensure_pipelines(state) ||
      !prom_m47_ensure_pipelines(state) ||
      !prom_m40b_ensure_sgemm_pipeline(state, PROM_M42_PATH_A2X4) ||
      !prom_m40b_ensure_sgemm_pipeline(state, PROM_M42_PATH_CONVENTIONAL_FP16)) {
    out_result->detail_code = PROM_M47_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  slot = prom_reduction_acquire_slot(state, state->next_logical_request_id++);
  state->diagnostics.next_logical_request_id = state->next_logical_request_id;
  if (slot == NULL) {
    out_result->detail_code = PROM_M47_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  f32_bytes = (VkDeviceSize)(elements * sizeof(float));
  f16_bytes = (VkDeviceSize)((((uint64_t)padded_rows * padded_columns + 1u) / 2u) * sizeof(uint32_t));
  {
    const uint64_t before = state->diagnostics.buffer_allocation_count;
    if (!prom_m47_ensure_buffer(state, &state->m47_weight_upload[request->kind], f32_bytes,
                                VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                                VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                                1)) goto m47_weight_resource_fail;
    reused_upload = state->diagnostics.buffer_allocation_count == before;
  }
  {
    const uint64_t before = state->diagnostics.buffer_allocation_count;
    if (!prom_m47_ensure_buffer(state, &state->m47_weight_f32[request->kind], f32_bytes,
                                VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) goto m47_weight_resource_fail;
    reused_f32 = state->diagnostics.buffer_allocation_count == before;
  }
  {
    const uint64_t before = state->diagnostics.buffer_allocation_count;
    if (!prom_m47_ensure_buffer(state, &state->m47_weight_f16[request->kind], f16_bytes,
                                VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) goto m47_weight_resource_fail;
    reused_f16 = state->diagnostics.buffer_allocation_count == before;
  }
  memcpy(state->m47_weight_upload[request->kind].mapped, request->values, (size_t)f32_bytes);
  prom_m47_update_descriptor(state, slot->m47_descriptor_sets[0],
                             &state->m47_weight_f32[request->kind],
                             &state->m47_weight_f32[request->kind],
                             &state->m47_weight_f32[request->kind],
                             &state->m47_weight_f16[request->kind]);
  if (vkResetCommandBuffer(slot->command_buffer, 0u) != VK_SUCCESS) goto m47_weight_command_fail;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(slot->command_buffer, &begin_info) != VK_SUCCESS) goto m47_weight_command_fail;
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
    vkCmdResetQueryPool(slot->command_buffer, state->query_pool,
                        slot->slot_id * PROM_REDUCTION_QUERY_STRIDE, 2u);
    vkCmdWriteTimestamp(slot->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, state->query_pool,
                        slot->slot_id * PROM_REDUCTION_QUERY_STRIDE);
  }
  prom_m42_buffer_barrier(slot->command_buffer, &state->m47_weight_upload[request->kind],
                          VK_ACCESS_HOST_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                          VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
  memset(&copy, 0, sizeof(copy));
  copy.size = f32_bytes;
  vkCmdCopyBuffer(slot->command_buffer, state->m47_weight_upload[request->kind].buffer,
                  state->m47_weight_f32[request->kind].buffer, 1u, &copy);
  prom_m42_buffer_barrier(slot->command_buffer, &state->m47_weight_f32[request->kind],
                          VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                          VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
  prom_m42_record_pack(state, slot->command_buffer, slot->m47_descriptor_sets[0],
                       rows, columns, columns, padded_rows, padded_columns, 0u);
  prom_m42_buffer_barrier(slot->command_buffer, &state->m47_weight_f16[request->kind],
                          VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                          VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE)
    vkCmdWriteTimestamp(slot->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, state->query_pool,
                        slot->slot_id * PROM_REDUCTION_QUERY_STRIDE + 1u);
  if (vkEndCommandBuffer(slot->command_buffer) != VK_SUCCESS ||
      vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS) goto m47_weight_command_fail;
  memset(&submit, 0, sizeof(submit));
  submit.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit.commandBufferCount = 1u;
  submit.pCommandBuffers = &slot->command_buffer;
  submit_begin = prom_reduction_now_ns();
  result = vkQueueSubmit(state->queue, 1u, &submit, slot->fence);
  if (result != VK_SUCCESS) goto m47_weight_submit_fail;
  slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
  result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
  out_result->preparation_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  if (result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    out_result->detail_code = PROM_M47_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE &&
      vkGetQueryPoolResults(state->device, state->query_pool,
                            slot->slot_id * PROM_REDUCTION_QUERY_STRIDE, 2u,
                            sizeof(timestamps), timestamps, sizeof(uint64_t),
                            VK_QUERY_RESULT_64_BIT) == VK_SUCCESS && timestamps[1] >= timestamps[0])
    out_result->gpu_upload_and_pack_ns =
        (uint64_t)((double)(timestamps[1] - timestamps[0]) * state->timestamp_period_ns);
  out_result->replaced = state->m47_weight_generation[request->kind] != 0u;
  state->m47_weight_generation[request->kind] = request->generation;
  state->m47_weight_hash[request->kind] = out_result->hash;
  state->m47_weight_model_width[request->kind] = request->model_width;
  state->m47_weight_ffn_width[request->kind] = request->ffn_width;
  out_result->generation = request->generation;
  out_result->buffer_reused = reused_upload != 0u && reused_f32 != 0u && reused_f16 != 0u;
  out_result->retained_upload_bytes = (uint64_t)state->m47_weight_upload[request->kind].size;
  out_result->retained_f32_bytes = (uint64_t)state->m47_weight_f32[request->kind].size;
  out_result->retained_packed_bytes = (uint64_t)state->m47_weight_f16[request->kind].size;
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  return PROM_OK;

m47_weight_resource_fail:
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->detail_code = PROM_M47_DETAIL_RESOURCE;
  return PROM_ERROR;
m47_weight_command_fail:
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->detail_code = PROM_M47_DETAIL_COMMAND;
  return PROM_ERROR;
m47_weight_submit_fail:
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->detail_code = PROM_M47_DETAIL_SUBMIT;
  return PROM_ERROR;
}

static int prom_m48_ensure_buffer(prom_reduction_runtime_state* state,
                                  prom_vk_buffer* buffer,
                                  VkDeviceSize size,
                                  VkBufferUsageFlags usage,
                                  VkMemoryPropertyFlags properties,
                                  int map_memory,
                                  uint32_t* out_reused) {
  const uint64_t allocations_before = state->diagnostics.buffer_allocation_count;
  if (!prom_reduction_ensure_buffer(state, buffer, size, usage, properties, map_memory)) return 0;
  if (state->diagnostics.buffer_allocation_count == allocations_before) {
    state->m48_buffer_reuse_count += 1u;
    if (out_reused != NULL) *out_reused = 1u;
  } else {
    state->m48_buffer_grow_count += 1u;
    if (out_reused != NULL) *out_reused = 0u;
  }
  return 1;
}

static int prom_m48_upload_f32_parameter(prom_reduction_runtime_state* state,
                                         prom_reduction_slot* slot,
                                         prom_transformer_parameter_resource* resource,
                                         VkDeviceSize bytes,
                                         uint64_t* out_gpu_ns) {
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  VkSubmitInfo submit;
  uint64_t timestamps[2] = {0u, 0u};
  VkResult result;
  if (vkResetCommandBuffer(slot->command_buffer, 0u) != VK_SUCCESS) return 0;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(slot->command_buffer, &begin_info) != VK_SUCCESS) return 0;
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
    vkCmdResetQueryPool(slot->command_buffer, state->query_pool,
                        slot->slot_id * PROM_REDUCTION_QUERY_STRIDE, 2u);
    vkCmdWriteTimestamp(slot->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                        state->query_pool, slot->slot_id * PROM_REDUCTION_QUERY_STRIDE);
  }
  prom_m42_buffer_barrier(slot->command_buffer, &resource->upload,
                          VK_ACCESS_HOST_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                          VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
  memset(&copy, 0, sizeof(copy));
  copy.size = bytes;
  vkCmdCopyBuffer(slot->command_buffer, resource->upload.buffer,
                  resource->f32.buffer, 1u, &copy);
  prom_m42_buffer_barrier(slot->command_buffer, &resource->f32,
                          VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                          VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE)
    vkCmdWriteTimestamp(slot->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                        state->query_pool, slot->slot_id * PROM_REDUCTION_QUERY_STRIDE + 1u);
  if (vkEndCommandBuffer(slot->command_buffer) != VK_SUCCESS ||
      vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS) return 0;
  memset(&submit, 0, sizeof(submit));
  submit.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit.commandBufferCount = 1u;
  submit.pCommandBuffers = &slot->command_buffer;
  result = vkQueueSubmit(state->queue, 1u, &submit, slot->fence);
  if (result != VK_SUCCESS) return 0;
  slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
  result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
  if (result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    return 0;
  }
  if (out_gpu_ns != NULL && state->timestamp_supported != 0u &&
      state->query_pool != VK_NULL_HANDLE &&
      vkGetQueryPoolResults(state->device, state->query_pool,
                            slot->slot_id * PROM_REDUCTION_QUERY_STRIDE, 2u,
                            sizeof(timestamps), timestamps, sizeof(uint64_t),
                            VK_QUERY_RESULT_64_BIT) == VK_SUCCESS &&
      timestamps[1] >= timestamps[0]) {
    *out_gpu_ns = (uint64_t)((double)(timestamps[1] - timestamps[0]) *
                             state->timestamp_period_ns);
  }
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  return 1;
}

int prom_reactor_runtime_m48_prepare_layer_weight(
    void* handle,
    const prom_m48_layer_weight_prepare_request* request,
    prom_m48_layer_weight_prepare_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_reduction_slot* slot;
  prom_transformer_layer_resources* layer;
  prom_transformer_parameter_resource* resource = NULL;
  uint64_t elements;
  uint64_t begin_ns;
  uint64_t upload_wall_ns = 0u;
  VkDeviceSize f32_bytes;
  VkDeviceSize f16_bytes = 0u;
  uint32_t logical_rows;
  uint32_t logical_columns;
  uint32_t padded_rows;
  uint32_t padded_columns;
  uint32_t finite = 0u;
  uint32_t reused_upload = 0u;
  uint32_t reused_f32 = 0u;
  uint32_t reused_f16 = 0u;
  uint32_t packed = 1u;
  int32_t detail = 0;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  if (request == NULL || request->values == NULL ||
      request->layer_index >= PROM_M48_LAYER_COUNT ||
      request->resource_index >= PROM_M48_RESOURCE_COUNT ||
      request->generation == 0u || request->model_width == 0u ||
      request->head_dim == 0u || request->ffn_width == 0u) {
    out_result->detail_code = PROM_M48_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  layer = NULL;
  logical_rows = request->model_width;
  logical_columns = request->head_dim;
  if (request->resource_index < PROM_M48_ATTENTION_RESOURCE_COUNT) {
    const uint32_t head = request->resource_index / PROM_M43_WEIGHT_KIND_COUNT;
    const uint32_t kind = request->resource_index % PROM_M43_WEIGHT_KIND_COUNT;
    if (!prom_m40b_checked_product_u64(request->model_width, request->head_dim, &elements)) {
      out_result->detail_code = PROM_M48_DETAIL_INVALID_REQUEST;
      return PROM_ERROR;
    }
    state = prom_reduction_ensure_state(handle, &detail);
    if (state != NULL) resource = &state->m48_layer[request->layer_index].attention[head][kind];
  } else if (request->resource_index == PROM_M48_RESOURCE_WO) {
    logical_columns = request->model_width;
    if (!prom_m40b_checked_product_u64(request->model_width, request->model_width, &elements)) {
      out_result->detail_code = PROM_M48_DETAIL_INVALID_REQUEST;
      return PROM_ERROR;
    }
    state = prom_reduction_ensure_state(handle, &detail);
    if (state != NULL) resource = &state->m48_layer[request->layer_index].wo;
  } else if (request->resource_index == PROM_M48_RESOURCE_RMSNORM) {
    logical_rows = 1u;
    logical_columns = request->model_width;
    elements = request->model_width;
    packed = 0u;
    state = prom_reduction_ensure_state(handle, &detail);
    if (state != NULL) resource = &state->m48_layer[request->layer_index].rmsnorm;
  } else {
    const uint32_t kind = request->resource_index - PROM_M48_RESOURCE_WGATE;
    logical_rows = kind == PROM_M47_WEIGHT_DOWN ? request->ffn_width : request->model_width;
    logical_columns = kind == PROM_M47_WEIGHT_DOWN ? request->model_width : request->ffn_width;
    if (!prom_m40b_checked_product_u64(logical_rows, logical_columns, &elements)) {
      out_result->detail_code = PROM_M48_DETAIL_INVALID_REQUEST;
      return PROM_ERROR;
    }
    state = prom_reduction_ensure_state(handle, &detail);
    if (state != NULL) resource = &state->m48_layer[request->layer_index].ffn[kind];
  }
  if (state == NULL || resource == NULL) {
    out_result->detail_code = state == NULL ? detail : PROM_M48_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  out_result->layer_index = request->layer_index;
  out_result->resource_index = request->resource_index;
  if (request->element_count != elements || elements > SIZE_MAX / sizeof(float) ||
      !prom_m42_round_up_16(logical_rows, &padded_rows) ||
      !prom_m42_round_up_16(logical_columns, &padded_columns)) {
    out_result->detail_code = PROM_M48_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  begin_ns = prom_reduction_now_ns();
  out_result->hash = prom_m42_hash_finite_matrix(request->values, elements, &finite);
  if (finite == 0u) {
    out_result->detail_code = PROM_M48_DETAIL_NONFINITE_INPUT;
    return PROM_ERROR;
  }
  if (request->generation <= resource->generation) {
    out_result->detail_code = PROM_M48_DETAIL_STALE_WEIGHT_GENERATION;
    return PROM_ERROR;
  }
  if (!prom_m40b_wait_all_slots(state) || !prom_m42_ensure_pipelines(state) ||
      !prom_m46_ensure_pipelines(state) || !prom_m47_ensure_pipelines(state) ||
      !prom_m44_ensure_pipelines(state) || !prom_m45_ensure_pipeline(state)) {
    out_result->detail_code = PROM_M48_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  slot = prom_reduction_acquire_slot(state, state->next_logical_request_id++);
  state->diagnostics.next_logical_request_id = state->next_logical_request_id;
  if (slot == NULL) {
    out_result->detail_code = PROM_M48_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  f32_bytes = (VkDeviceSize)(elements * sizeof(float));
  if (packed != 0u)
    f16_bytes = (VkDeviceSize)((((uint64_t)padded_rows * padded_columns + 1u) / 2u) *
                               sizeof(uint32_t));
  if (!prom_m48_ensure_buffer(state, &resource->upload, f32_bytes,
                              VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                              1, &reused_upload) ||
      !prom_m48_ensure_buffer(state, &resource->f32, f32_bytes,
                              VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &reused_f32) ||
      (packed != 0u &&
         !prom_m48_ensure_buffer(state, &resource->f16, f16_bytes,
                               VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &reused_f16))) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->detail_code = PROM_M48_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  memcpy(resource->upload.mapped, request->values, (size_t)f32_bytes);
  if (packed != 0u) {
    prom_m43_update_descriptor(state, slot->m43_descriptor_sets[0],
                               &resource->f32, NULL, &resource->f16);
    if (!prom_m43_record_upload_and_pack(state, slot, &resource->upload,
                                         &resource->f32, &resource->f16,
                                         logical_rows, logical_columns,
                                         padded_rows, padded_columns, f32_bytes,
                                         &out_result->gpu_upload_and_pack_ns,
                                         &upload_wall_ns)) {
      if (slot->state != PROM_ASYNC_PHYSICAL_QUARANTINED)
        slot->state = PROM_ASYNC_PHYSICAL_READY;
      out_result->detail_code = slot->state == PROM_ASYNC_PHYSICAL_QUARANTINED
                                    ? PROM_M48_DETAIL_COMPLETION_UNCERTAIN
                                    : PROM_M48_DETAIL_COMMAND;
      return PROM_ERROR;
    }
  } else if (!prom_m48_upload_f32_parameter(state, slot, resource, f32_bytes,
                                             &out_result->gpu_upload_and_pack_ns)) {
    if (slot->state != PROM_ASYNC_PHYSICAL_QUARANTINED)
      slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->detail_code = slot->state == PROM_ASYNC_PHYSICAL_QUARANTINED
                                  ? PROM_M48_DETAIL_COMPLETION_UNCERTAIN
                                  : PROM_M48_DETAIL_COMMAND;
    return PROM_ERROR;
  }
  out_result->replaced = resource->generation != 0u;
  resource->generation = request->generation;
  resource->hash = out_result->hash;
  resource->rows = logical_rows;
  resource->columns = logical_columns;
  layer = &state->m48_layer[request->layer_index];
  layer->model_width = request->model_width;
  layer->head_dim = request->head_dim;
  layer->ffn_width = request->ffn_width;
  out_result->generation = request->generation;
  out_result->retained_upload_bytes = resource->upload.size;
  out_result->retained_f32_bytes = resource->f32.size;
  out_result->retained_packed_bytes = resource->f16.size;
  out_result->buffer_reused = reused_upload != 0u && reused_f32 != 0u &&
                              (packed == 0u || reused_f16 != 0u);
  out_result->preparation_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  return PROM_OK;
}

int prom_reactor_runtime_m48_prepare_initial_activation(
    void* handle,
    const prom_m48_initial_activation_prepare_request* request,
    prom_m48_initial_activation_prepare_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_reduction_slot* slot;
  prom_transformer_parameter_resource resource;
  uint64_t elements;
  uint64_t begin_ns;
  VkDeviceSize bytes;
  uint32_t finite = 0u;
  uint32_t reused_upload = 0u;
  uint32_t reused_f32 = 0u;
  int32_t detail = 0;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  if (request == NULL || request->values == NULL || request->tokens == 0u ||
      request->model_width == 0u || request->generation == 0u ||
      !prom_m40b_checked_product_u64(request->tokens, request->model_width, &elements) ||
      request->element_count != elements || elements > SIZE_MAX / sizeof(float)) {
    out_result->detail_code = PROM_M48_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  begin_ns = prom_reduction_now_ns();
  out_result->hash = prom_m42_hash_finite_matrix(request->values, elements, &finite);
  if (finite == 0u) {
    out_result->detail_code = PROM_M48_DETAIL_NONFINITE_INPUT;
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL) {
    out_result->detail_code = detail;
    return PROM_ERROR;
  }
  if (request->generation <= state->m48_initial_generation) {
    out_result->detail_code = PROM_M48_DETAIL_STALE_INITIAL_GENERATION;
    return PROM_ERROR;
  }
  if (!prom_m40b_wait_all_slots(state)) {
    out_result->detail_code = PROM_M48_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  slot = prom_reduction_acquire_slot(state, state->next_logical_request_id++);
  state->diagnostics.next_logical_request_id = state->next_logical_request_id;
  if (slot == NULL) {
    out_result->detail_code = PROM_M48_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  bytes = (VkDeviceSize)(elements * sizeof(float));
  if (!prom_m48_ensure_buffer(state, &state->m48_initial_upload, bytes,
                              VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                              1, &reused_upload) ||
      !prom_m48_ensure_buffer(state, &state->m48_initial_f32, bytes,
                              VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &reused_f32)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->detail_code = PROM_M48_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  memcpy(state->m48_initial_upload.mapped, request->values, (size_t)bytes);
  memset(&resource, 0, sizeof(resource));
  resource.upload = state->m48_initial_upload;
  resource.f32 = state->m48_initial_f32;
  if (!prom_m48_upload_f32_parameter(state, slot, &resource, bytes,
                                     &out_result->gpu_upload_ns)) {
    if (slot->state != PROM_ASYNC_PHYSICAL_QUARANTINED)
      slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->detail_code = slot->state == PROM_ASYNC_PHYSICAL_QUARANTINED
                                  ? PROM_M48_DETAIL_COMPLETION_UNCERTAIN
                                  : PROM_M48_DETAIL_COMMAND;
    return PROM_ERROR;
  }
  out_result->replaced = state->m48_initial_generation != 0u;
  state->m48_initial_generation = request->generation;
  state->m48_initial_hash = out_result->hash;
  state->m48_initial_tokens = request->tokens;
  state->m48_initial_model_width = request->model_width;
  out_result->generation = request->generation;
  out_result->retained_upload_bytes = state->m48_initial_upload.size;
  out_result->retained_device_bytes = state->m48_initial_f32.size;
  out_result->buffer_reused = reused_upload != 0u && reused_f32 != 0u;
  out_result->preparation_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  return PROM_OK;
}

static int prom_m43_prepare_execution_buffers(prom_reduction_runtime_state* state,
                                              prom_reduction_slot* slot,
                                              const prom_m43_attention_group_request* request,
                                              const prom_m43_attention_plan* plan,
                                              uint32_t require_readback) {
  const uint64_t logical_x_elements = (uint64_t)request->tokens * request->model_width;
  const uint64_t padded_x_elements = (uint64_t)plan->padded_tokens * plan->padded_model_width;
  const uint64_t q_elements = (uint64_t)plan->padded_tokens * plan->padded_head_dim;
  const uint64_t score_elements = (uint64_t)plan->padded_tokens * plan->padded_tokens;
  const uint64_t logical_score_elements = (uint64_t)request->tokens * request->tokens;
  const uint64_t logical_output_elements = (uint64_t)request->tokens * request->head_dim;
  const VkDeviceSize x_f32_bytes = (VkDeviceSize)(logical_x_elements * sizeof(float));
  const VkDeviceSize x_f16_bytes = (VkDeviceSize)(((padded_x_elements + 1u) / 2u) * sizeof(uint32_t));
  const VkDeviceSize q_bytes = (VkDeviceSize)(q_elements * sizeof(float));
  const VkDeviceSize packed_q_bytes = (VkDeviceSize)(((q_elements + 1u) / 2u) * sizeof(uint32_t));
  const VkDeviceSize compact_k_transpose_bytes =
      (VkDeviceSize)((uint64_t)request->head_dim * request->tokens * sizeof(float));
  const VkDeviceSize k_transpose_bytes = packed_q_bytes > compact_k_transpose_bytes
                                             ? packed_q_bytes
                                             : compact_k_transpose_bytes;
  const VkDeviceSize score_bytes = (VkDeviceSize)(score_elements * sizeof(float));
  const VkDeviceSize probability_bytes = (VkDeviceSize)(logical_score_elements * sizeof(float));
  const VkDeviceSize packed_probability_bytes =
      (VkDeviceSize)(((score_elements + 1u) / 2u) * sizeof(uint32_t));
  const VkDeviceSize readback_bytes =
      (VkDeviceSize)(logical_output_elements * PROM_M43_HEAD_COUNT * sizeof(float));
  uint32_t head;
  if (request->input_mode == PROM_M42_INPUT_HOST_X &&
      (!prom_m43_ensure_buffer(state, &slot->m43_x_upload, x_f32_bytes,
                               VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                               VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                               1, NULL) ||
       !prom_m43_ensure_buffer(state, &slot->m43_x_f32, x_f32_bytes,
                               VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
       !prom_m43_ensure_buffer(state, &slot->m43_x_f16, x_f16_bytes,
                               VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL))) return 0;
  for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
    prom_m43_head_slot* head_slot = &slot->m43_head[head];
    if (!prom_m43_ensure_buffer(state, &head_slot->q, q_bytes,
                                VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
        !prom_m43_ensure_buffer(state, &head_slot->k, q_bytes,
                                VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
        !prom_m43_ensure_buffer(state, &head_slot->v, q_bytes,
                                VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
        !prom_m43_ensure_buffer(state, &head_slot->q_packed, packed_q_bytes,
                                VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
        !prom_m43_ensure_buffer(state, &head_slot->k_transposed, k_transpose_bytes,
                                VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
        !prom_m43_ensure_buffer(state, &head_slot->v_packed, packed_q_bytes,
                                VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
        !prom_m43_ensure_buffer(state, &head_slot->scores, score_bytes,
                                VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
        !prom_m43_ensure_buffer(state, &head_slot->probabilities, probability_bytes,
                                VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
        !prom_m43_ensure_buffer(state, &head_slot->p_packed, packed_probability_bytes,
                                VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
        !prom_m43_ensure_buffer(state, &head_slot->output, q_bytes,
                                VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                                VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL)) return 0;
  }
  if ((require_readback != 0u &&
       !prom_m43_ensure_buffer(state, &slot->m43_readback, readback_bytes,
                               VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                               VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                               1, NULL)) ||
      !prom_m43_ensure_buffer(state, &slot->scratch, PROM_REDUCTION_MIN_BINDING_BYTES,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
      !prom_m43_ensure_buffer(state, &slot->row_max, PROM_REDUCTION_MIN_BINDING_BYTES,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
      !prom_m43_ensure_buffer(state, &slot->row_sum, PROM_REDUCTION_MIN_BINDING_BYTES,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL)) return 0;
  return 1;
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

static int prom_m43_setup_descriptors(prom_reduction_runtime_state* state,
                                      prom_reduction_slot* slot,
                                      const prom_m43_attention_group_request* request,
                                      prom_m43_attention_group_result* out_result,
                                      const prom_vk_buffer* x_f32,
                                      const prom_vk_buffer* x_f16) {
  uint32_t head;
  for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
    const uint32_t base = head * PROM_M42_DESCRIPTOR_SET_COUNT;
    const uint32_t reduced = out_result->plan.selected_path[head] != PROM_M42_PATH_A2X4;
    const prom_vk_buffer* x = reduced != 0u ? x_f16 : x_f32;
    const prom_vk_buffer* wq = reduced != 0u
                                   ? &state->m43_weight_f16[head][PROM_M43_WEIGHT_Q]
                                   : &state->m43_weight_f32[head][PROM_M43_WEIGHT_Q];
    const prom_vk_buffer* wk = reduced != 0u
                                   ? &state->m43_weight_f16[head][PROM_M43_WEIGHT_K]
                                   : &state->m43_weight_f32[head][PROM_M43_WEIGHT_K];
    const prom_vk_buffer* wv = reduced != 0u
                                   ? &state->m43_weight_f16[head][PROM_M43_WEIGHT_V]
                                   : &state->m43_weight_f32[head][PROM_M43_WEIGHT_V];
    prom_m43_head_slot* head_slot = &slot->m43_head[head];
    prom_m43_update_descriptor(state, slot->m43_descriptor_sets[base], x, wq, &head_slot->q);
    prom_m43_update_descriptor(state, slot->m43_descriptor_sets[base + 1u], x, wk, &head_slot->k);
    prom_m43_update_descriptor(state, slot->m43_descriptor_sets[base + 2u], x, wv, &head_slot->v);
    prom_m43_update_descriptor(state, slot->m43_descriptor_sets[base + 4u], &head_slot->k, NULL,
                               &head_slot->k_transposed);
    prom_m43_update_descriptor(state, slot->m43_descriptor_sets[base + 7u], &head_slot->scores, NULL,
                               &head_slot->scores);
    if (reduced != 0u) {
      prom_m43_update_descriptor(state, slot->m43_descriptor_sets[base + 3u], &head_slot->q, NULL,
                                 &head_slot->q_packed);
      prom_m43_update_descriptor(state, slot->m43_descriptor_sets[base + 5u], &head_slot->v, NULL,
                                 &head_slot->v_packed);
      prom_m43_update_descriptor(state, slot->m43_descriptor_sets[base + 6u], &head_slot->q_packed,
                                 &head_slot->k_transposed, &head_slot->scores);
      prom_m43_update_descriptor(state, slot->m43_descriptor_sets[base + 13u],
                                 &head_slot->probabilities, NULL, &head_slot->p_packed);
      prom_m43_update_descriptor(state, slot->m43_descriptor_sets[base + 14u], &head_slot->p_packed,
                                 &head_slot->v_packed, &head_slot->output);
    } else {
      prom_m43_update_descriptor(state, slot->m43_descriptor_sets[base + 6u], &head_slot->q,
                                 &head_slot->k_transposed, &head_slot->scores);
      prom_m43_update_descriptor(state, slot->m43_descriptor_sets[base + 14u],
                                 &head_slot->probabilities, &head_slot->v, &head_slot->output);
    }
    memset(&out_result->head_output_view[head], 0, sizeof(out_result->head_output_view[head]));
    out_result->head_output_view[head].buffer = head_slot->output.buffer;
    out_result->head_output_view[head].byte_length = head_slot->output.size;
    out_result->head_output_view[head].element_type = PROM_DEVICE_ELEMENT_F32;
    out_result->head_output_view[head].logical_rows = request->tokens;
    out_result->head_output_view[head].logical_columns = request->head_dim;
    out_result->head_output_view[head].row_stride_elements =
        reduced != 0u ? out_result->plan.padded_head_dim : request->head_dim;
    out_result->head_output_view[head].layout = PROM_DEVICE_LAYOUT_ROW_MAJOR;
    out_result->head_output_view[head].producer_access = PROM_DEVICE_ACCESS_COMPUTE_WRITE;
    out_result->head_output_view[head].required_consumer_access = PROM_DEVICE_ACCESS_HOST_READ;
    out_result->head_output_view[head].owning_device = state->device;
    out_result->head_output_view[head].owning_lifetime_id = out_result->logical_request_id;
    out_result->head_output_view[head].owning_slot_id = slot->slot_id;
    out_result->head_output_view[head].owning_slot_generation = slot->generation;
  }
  if (request->input_mode == PROM_M42_INPUT_HOST_X) {
    prom_m43_update_descriptor(state,
                               slot->m43_descriptor_sets[PROM_M43_DESCRIPTOR_SET_COUNT - 1u],
                               &slot->m43_x_f32, NULL, &slot->m43_x_f16);
  }
  return 1;
}

static void prom_m43_write_head_timestamp(const prom_reduction_runtime_state* state,
                                          const prom_reduction_slot* slot,
                                          VkCommandBuffer command_buffer,
                                          uint32_t head,
                                          uint32_t operation,
                                          uint32_t end) {
  const uint32_t pair = prom_m43_query_stage_index(operation);
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                           PROM_M43_QUERY_HEAD_BASE + head * PROM_M43_QUERY_HEAD_STRIDE + pair * 2u + end);
}

static void prom_m43_record_projection(prom_reduction_runtime_state* state,
                                       prom_reduction_slot* slot,
                                       VkCommandBuffer command_buffer,
                                       const prom_m43_attention_group_request* request,
                                       const prom_m43_attention_plan* plan,
                                       uint32_t head,
                                       uint32_t operation) {
  const uint32_t reduced = plan->selected_path[head] != PROM_M42_PATH_A2X4;
  const uint32_t storage_tokens = reduced != 0u ? plan->padded_tokens : request->tokens;
  const uint32_t storage_model = reduced != 0u ? plan->padded_model_width : request->model_width;
  const uint32_t storage_head = reduced != 0u ? plan->padded_head_dim : request->head_dim;
  const uint32_t descriptor = prom_m43_descriptor_for_operation(head, operation);
  prom_m43_write_head_timestamp(state, slot, command_buffer, head, operation, 0u);
  prom_m42_record_sgemm(state, command_buffer, slot->m43_descriptor_sets[descriptor],
                        plan->selected_path[head], storage_tokens, storage_head, storage_model);
  prom_m43_write_head_timestamp(state, slot, command_buffer, head, operation, 1u);
}

static int prom_m43_record_projection_barrier(VkCommandBuffer command_buffer,
                                              prom_reduction_slot* slot,
                                              const prom_m43_attention_group_request* request,
                                              const prom_m43_attention_plan* plan,
                                              uint32_t first_head,
                                              uint32_t head_count) {
  const prom_vk_buffer* buffers[PROM_M43_HEAD_COUNT * PROM_M43_WEIGHT_KIND_COUNT];
  VkDeviceSize sizes[PROM_M43_HEAD_COUNT * PROM_M43_WEIGHT_KIND_COUNT];
  uint32_t head;
  uint32_t count = 0u;
  for (head = first_head; head < first_head + head_count; ++head) {
    const uint32_t reduced = plan->selected_path[head] != PROM_M42_PATH_A2X4;
    const uint32_t storage_tokens = reduced != 0u ? plan->padded_tokens : request->tokens;
    const uint32_t storage_head = reduced != 0u ? plan->padded_head_dim : request->head_dim;
    const VkDeviceSize bytes = (VkDeviceSize)((uint64_t)storage_tokens * storage_head * sizeof(float));
    buffers[count] = &slot->m43_head[head].q; sizes[count++] = bytes;
    buffers[count] = &slot->m43_head[head].k; sizes[count++] = bytes;
    buffers[count] = &slot->m43_head[head].v; sizes[count++] = bytes;
  }
  return prom_m43_buffer_barriers(command_buffer, buffers, sizes, count,
                                  VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                                  VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                  VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
}

static int prom_m43_record_head_tail(prom_reduction_runtime_state* state,
                                     prom_reduction_slot* slot,
                                     VkCommandBuffer command_buffer,
                                     const prom_m43_attention_group_request* request,
                                     const prom_m43_attention_plan* plan,
                                     const PrometheusReductionPlan* reduction_plan,
                                     uint32_t head,
                                     uint32_t* out_partial_fault,
                                     uint32_t* out_uncertain_fault) {
  const uint32_t base = head * PROM_M42_DESCRIPTOR_SET_COUNT;
  const uint32_t reduced = plan->selected_path[head] != PROM_M42_PATH_A2X4;
  const uint32_t storage_tokens = reduced != 0u ? plan->padded_tokens : request->tokens;
  const uint32_t storage_head = reduced != 0u ? plan->padded_head_dim : request->head_dim;
  const VkDeviceSize q_bytes = (VkDeviceSize)((uint64_t)storage_tokens * storage_head * sizeof(float));
  const VkDeviceSize packed_q_bytes =
      (VkDeviceSize)((((uint64_t)storage_tokens * storage_head + 1u) / 2u) * sizeof(uint32_t));
  const VkDeviceSize k_transpose_bytes = reduced != 0u
                                            ? packed_q_bytes
                                            : (VkDeviceSize)((uint64_t)request->head_dim * request->tokens *
                                                             sizeof(float));
  const VkDeviceSize score_bytes =
      (VkDeviceSize)((uint64_t)storage_tokens * storage_tokens * sizeof(float));
  const VkDeviceSize probability_bytes =
      (VkDeviceSize)((uint64_t)request->tokens * request->tokens * sizeof(float));
  const VkDeviceSize packed_probability_bytes =
      (VkDeviceSize)((((uint64_t)storage_tokens * storage_tokens + 1u) / 2u) * sizeof(uint32_t));
  prom_m43_head_slot* head_slot = &slot->m43_head[head];
  uint32_t stage_index;

  prom_m43_write_head_timestamp(state, slot, command_buffer, head, PROM_M42_STAGE_PACK_Q, 0u);
  if (reduced != 0u) {
    prom_m42_record_pack(state, command_buffer, slot->m43_descriptor_sets[base + 3u],
                         request->tokens, request->head_dim, storage_head,
                         storage_tokens, storage_head, 0u);
    if (!prom_m43_one_buffer_barrier(command_buffer, &head_slot->q_packed, packed_q_bytes,
                                     VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                                     VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                     VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT)) return 0;
  }
  prom_m43_write_head_timestamp(state, slot, command_buffer, head, PROM_M42_STAGE_PACK_Q, 1u);

  prom_m43_write_head_timestamp(state, slot, command_buffer, head, PROM_M42_STAGE_LAYOUT_K, 0u);
  if (reduced != 0u) {
    prom_m42_record_pack(state, command_buffer, slot->m43_descriptor_sets[base + 4u],
                         request->tokens, request->head_dim, storage_head,
                         storage_head, storage_tokens, 1u);
  } else {
    prom_m42_record_transpose(state, command_buffer, slot->m43_descriptor_sets[base + 4u],
                              request->tokens, request->head_dim, request->head_dim, request->tokens);
  }
  if (!prom_m43_one_buffer_barrier(command_buffer, &head_slot->k_transposed, k_transpose_bytes,
                                   VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT)) return 0;
  prom_m43_write_head_timestamp(state, slot, command_buffer, head, PROM_M42_STAGE_LAYOUT_K, 1u);

  prom_m43_write_head_timestamp(state, slot, command_buffer, head, PROM_M42_STAGE_PACK_V, 0u);
  if (reduced != 0u) {
    prom_m42_record_pack(state, command_buffer, slot->m43_descriptor_sets[base + 5u],
                         request->tokens, request->head_dim, storage_head,
                         storage_tokens, storage_head, 0u);
    if (!prom_m43_one_buffer_barrier(command_buffer, &head_slot->v_packed, packed_q_bytes,
                                     VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                                     VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                     VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT)) return 0;
  }
  prom_m43_write_head_timestamp(state, slot, command_buffer, head, PROM_M42_STAGE_PACK_V, 1u);

  prom_m43_write_head_timestamp(state, slot, command_buffer, head, PROM_M42_STAGE_QK_TRANSPOSE, 0u);
  prom_m42_record_sgemm(state, command_buffer, slot->m43_descriptor_sets[base + 6u],
                        plan->selected_path[head], storage_tokens, storage_tokens, storage_head);
  prom_m43_write_head_timestamp(state, slot, command_buffer, head, PROM_M42_STAGE_QK_TRANSPOSE, 1u);
  if (!prom_m43_one_buffer_barrier(command_buffer, &head_slot->scores, score_bytes,
                                   VK_ACCESS_SHADER_WRITE_BIT,
                                   VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT)) return 0;
  if (request->fault_point == PROM_M43_FAULT_HEAD_QK && request->fault_head == head) {
    *out_partial_fault = PROM_M43_FAULT_HEAD_QK;
    return 2;
  }

  prom_m43_write_head_timestamp(state, slot, command_buffer, head, PROM_M42_STAGE_SCALE, 0u);
  prom_m42_record_scale(state, command_buffer, slot->m43_descriptor_sets[base + 7u],
                        request->tokens, request->tokens, storage_tokens, plan->scale);
  prom_m43_write_head_timestamp(state, slot, command_buffer, head, PROM_M42_STAGE_SCALE, 1u);
  if (!prom_m43_one_buffer_barrier(command_buffer, &head_slot->scores, score_bytes,
                                   VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT)) return 0;

  prom_m43_write_head_timestamp(state, slot, command_buffer, head, PROM_M42_STAGE_SOFTMAX, 0u);
  for (stage_index = 0u; stage_index < reduction_plan->stage_count; ++stage_index) {
    const PrometheusReductionStageDispatch* stage = &reduction_plan->stages[stage_index];
    prom_reduction_buffer_bindings bindings;
    prom_reduction_push_constants push;
    VkPipeline pipeline = prom_reduction_pipeline_for_implementation(state, stage->implementation_id);
    if (pipeline == VK_NULL_HANDLE || base + 8u + stage_index >= base + 13u) return 0;
    prom_reduction_stage_bindings_for_io(slot, reduction_plan, stage_index,
                                         &head_slot->scores, &head_slot->probabilities, &bindings);
    prom_reduction_update_descriptor_set(state, slot->m43_descriptor_sets[base + 8u + stage_index], &bindings);
    state->m43_descriptor_update_count += 1u;
    memset(&push, 0, sizeof(push));
    push.row_count = request->tokens;
    push.elements_per_row = stage->input_elements_per_row;
    push.partials_per_row = stage->output_partials_per_row;
    push.input_row_stride = bindings.input == &head_slot->scores
                                ? storage_tokens
                                : stage->input_elements_per_row;
    push.chunk_elements = PROM_REDUCTION_ELEMENTS_PER_PARTIAL;
    push.total_elements = request->tokens * request->tokens;
    push.stage_role = stage->stage_role;
    vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, pipeline);
    vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, state->pipeline_layout,
                            0u, 1u, &slot->m43_descriptor_sets[base + 8u + stage_index], 0u, NULL);
    vkCmdPushConstants(command_buffer, state->pipeline_layout, VK_SHADER_STAGE_COMPUTE_BIT,
                       0u, sizeof(push), &push);
    vkCmdDispatch(command_buffer, stage->groups_x, stage->groups_y, stage->groups_z);
    if (stage_index + 1u < reduction_plan->stage_count) prom_reduction_record_barrier(command_buffer);
  }
  prom_m43_write_head_timestamp(state, slot, command_buffer, head, PROM_M42_STAGE_SOFTMAX, 1u);
  if (!prom_m43_one_buffer_barrier(command_buffer, &head_slot->probabilities, probability_bytes,
                                   VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT)) return 0;
  if (request->fault_point == PROM_M43_FAULT_HEAD_SOFTMAX && request->fault_head == head) {
    *out_partial_fault = PROM_M43_FAULT_HEAD_SOFTMAX;
    return 2;
  }

  prom_m43_write_head_timestamp(state, slot, command_buffer, head, PROM_M42_STAGE_PACK_P, 0u);
  if (reduced != 0u) {
    prom_m42_record_pack(state, command_buffer, slot->m43_descriptor_sets[base + 13u],
                         request->tokens, request->tokens, request->tokens,
                         storage_tokens, storage_tokens, 0u);
    if (!prom_m43_one_buffer_barrier(command_buffer, &head_slot->p_packed, packed_probability_bytes,
                                     VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                                     VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                     VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT)) return 0;
  }
  prom_m43_write_head_timestamp(state, slot, command_buffer, head, PROM_M42_STAGE_PACK_P, 1u);

  prom_m43_write_head_timestamp(state, slot, command_buffer, head, PROM_M42_STAGE_PV, 0u);
  prom_m42_record_sgemm(state, command_buffer, slot->m43_descriptor_sets[base + 14u],
                        plan->selected_path[head], storage_tokens, storage_head, storage_tokens);
  prom_m43_write_head_timestamp(state, slot, command_buffer, head, PROM_M42_STAGE_PV, 1u);
  if (plan->execution_strategy != PROM_M43_STRATEGY_EIGHT_SEQUENTIAL_M42) {
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M43_QUERY_HEAD_BASE + head * PROM_M43_QUERY_HEAD_STRIDE + 22u);
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M43_QUERY_HEAD_BASE + head * PROM_M43_QUERY_HEAD_STRIDE + 23u);
  }
  if (request->fault_point == PROM_M43_FAULT_HEAD_PV_SUBMIT && request->fault_head == head) {
    *out_uncertain_fault = PROM_M43_FAULT_HEAD_PV_SUBMIT;
  }
  (void)q_bytes;
  return 1;
}

static int prom_m43_record_grouped_internal(prom_reduction_runtime_state* state,
                                            prom_reduction_slot* slot,
                                            const prom_m43_attention_group_request* request,
                                            const prom_m43_attention_plan* plan,
                                            const PrometheusReductionPlan* reduction_plan,
                                            uint32_t* out_partial_fault,
                                            uint32_t* out_uncertain_fault,
                                            uint32_t final_readback,
                                            uint32_t leave_open,
                                            VkCommandBuffer caller_command_buffer,
                                            const VkDescriptorSet* caller_descriptors,
                                            uint32_t already_open,
                                            uint32_t input_already_prepared) {
  VkCommandBuffer command_buffer = caller_command_buffer != VK_NULL_HANDLE
                                       ? caller_command_buffer : slot->command_buffer;
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  uint32_t head;
  const uint64_t logical_x_elements = (uint64_t)request->tokens * request->model_width;
  const VkDeviceSize x_f32_bytes = (VkDeviceSize)(logical_x_elements * sizeof(float));
  const VkDeviceSize output_row_bytes = (VkDeviceSize)((uint64_t)request->head_dim * sizeof(float));
  if (out_partial_fault != NULL) *out_partial_fault = 0u;
  if (out_uncertain_fault != NULL) *out_uncertain_fault = 0u;
  if (caller_descriptors != NULL)
    memcpy(slot->m43_descriptor_sets, caller_descriptors,
           sizeof(slot->m43_descriptor_sets));
  if (already_open == 0u) {
    if (vkResetCommandBuffer(command_buffer, 0u) != VK_SUCCESS) return 0;
    memset(&begin_info, 0, sizeof(begin_info));
    begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
    begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
    if (vkBeginCommandBuffer(command_buffer, &begin_info) != VK_SUCCESS) return 0;
  }
  if (input_already_prepared == 0u && state->timestamp_supported != 0u &&
      state->query_pool != VK_NULL_HANDLE) {
    vkCmdResetQueryPool(command_buffer, state->query_pool,
                        slot->active_query_base, PROM_M43_QUERY_COUNT);
  }
  if (input_already_prepared == 0u)
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, 0u);
  if (input_already_prepared == 0u && request->input_mode == PROM_M42_INPUT_HOST_X) {
    if (!prom_m43_one_buffer_barrier(command_buffer, &slot->m43_x_upload, x_f32_bytes,
                                     VK_ACCESS_HOST_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                                     VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT)) return 0;
    memset(&copy, 0, sizeof(copy));
    copy.size = x_f32_bytes;
    vkCmdCopyBuffer(command_buffer, slot->m43_x_upload.buffer, slot->m43_x_f32.buffer, 1u, &copy);
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT, 1u);
    if (!prom_m43_one_buffer_barrier(command_buffer, &slot->m43_x_f32, x_f32_bytes,
                                     VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                                     VK_PIPELINE_STAGE_TRANSFER_BIT,
                                     VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT)) return 0;
    prom_m42_record_pack(state, command_buffer,
                         slot->m43_descriptor_sets[PROM_M43_DESCRIPTOR_SET_COUNT - 1u],
                         request->tokens, request->model_width, request->model_width,
                         plan->padded_tokens, plan->padded_model_width, 0u);
    if (!prom_m43_one_buffer_barrier(
            command_buffer, &slot->m43_x_f16,
            (VkDeviceSize)((((uint64_t)plan->padded_tokens * plan->padded_model_width + 1u) / 2u) *
                           sizeof(uint32_t)),
            VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT)) return 0;
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 2u);
    if (request->fault_point == PROM_M43_FAULT_SHARED_X_UPLOAD) {
      *out_partial_fault = PROM_M43_FAULT_SHARED_X_UPLOAD;
      return vkEndCommandBuffer(command_buffer) == VK_SUCCESS ? 2 : 0;
    }
  } else if (input_already_prepared == 0u) {
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 1u);
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 2u);
  }
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 3u);
  if (plan->execution_strategy == PROM_M43_STRATEGY_PROJECTION_GROUPED) {
    for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head)
      prom_m43_record_projection(state, slot, command_buffer, request, plan, head, PROM_M42_STAGE_PROJECT_Q);
    for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head)
      prom_m43_record_projection(state, slot, command_buffer, request, plan, head, PROM_M42_STAGE_PROJECT_K);
    for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
      prom_m43_record_projection(state, slot, command_buffer, request, plan, head, PROM_M42_STAGE_PROJECT_V);
      if (request->fault_point == PROM_M43_FAULT_MID_PROJECTIONS && head == 3u) {
        *out_partial_fault = PROM_M43_FAULT_MID_PROJECTIONS;
        return vkEndCommandBuffer(command_buffer) == VK_SUCCESS ? 2 : 0;
      }
    }
    if (!prom_m43_record_projection_barrier(command_buffer, slot, request, plan, 0u,
                                            PROM_M43_HEAD_COUNT)) return 0;
    for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
      const int tail_status = prom_m43_record_head_tail(state, slot, command_buffer, request, plan,
                                                        reduction_plan, head, out_partial_fault,
                                                        out_uncertain_fault);
      if (tail_status == 0) return 0;
      if (tail_status == 2) return vkEndCommandBuffer(command_buffer) == VK_SUCCESS ? 2 : 0;
    }
  } else {
    for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
      int tail_status;
      prom_m43_record_projection(state, slot, command_buffer, request, plan, head, PROM_M42_STAGE_PROJECT_Q);
      prom_m43_record_projection(state, slot, command_buffer, request, plan, head, PROM_M42_STAGE_PROJECT_K);
      prom_m43_record_projection(state, slot, command_buffer, request, plan, head, PROM_M42_STAGE_PROJECT_V);
      if (request->fault_point == PROM_M43_FAULT_MID_PROJECTIONS && head == 3u) {
        *out_partial_fault = PROM_M43_FAULT_MID_PROJECTIONS;
        return vkEndCommandBuffer(command_buffer) == VK_SUCCESS ? 2 : 0;
      }
      if (!prom_m43_record_projection_barrier(command_buffer, slot, request, plan, head, 1u)) return 0;
      tail_status = prom_m43_record_head_tail(state, slot, command_buffer, request, plan,
                                              reduction_plan, head, out_partial_fault,
                                              out_uncertain_fault);
      if (tail_status == 0) return 0;
      if (tail_status == 2) return vkEndCommandBuffer(command_buffer) == VK_SUCCESS ? 2 : 0;
    }
  }
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                           PROM_M43_QUERY_GROUP_END);
  if (final_readback == 0u) {
    if (request->fault_point == PROM_M43_FAULT_FINAL_READBACK) {
      *out_partial_fault = PROM_M43_FAULT_FINAL_READBACK;
      return vkEndCommandBuffer(command_buffer) == VK_SUCCESS ? 2 : 0;
    }
    if (leave_open != 0u) return 1;
    return vkEndCommandBuffer(command_buffer) == VK_SUCCESS ? 1 : 0;
  }
  if (request->fault_point == PROM_M43_FAULT_FINAL_READBACK) {
    *out_partial_fault = PROM_M43_FAULT_FINAL_READBACK;
    return vkEndCommandBuffer(command_buffer) == VK_SUCCESS ? 2 : 0;
  }
  {
    const prom_vk_buffer* outputs[PROM_M43_HEAD_COUNT];
    VkDeviceSize output_sizes[PROM_M43_HEAD_COUNT];
    for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
      const uint32_t reduced = plan->selected_path[head] != PROM_M42_PATH_A2X4;
      const uint32_t storage_tokens = reduced != 0u ? plan->padded_tokens : request->tokens;
      const uint32_t storage_head = reduced != 0u ? plan->padded_head_dim : request->head_dim;
      outputs[head] = &slot->m43_head[head].output;
      output_sizes[head] = (VkDeviceSize)((uint64_t)storage_tokens * storage_head * sizeof(float));
    }
    if (!prom_m43_buffer_barriers(command_buffer, outputs, output_sizes, PROM_M43_HEAD_COUNT,
                                  VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                                  VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                  VK_PIPELINE_STAGE_TRANSFER_BIT)) return 0;
  }
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                           PROM_M43_QUERY_READBACK_BEGIN);
  for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
    const uint32_t reduced = plan->selected_path[head] != PROM_M42_PATH_A2X4;
    const uint32_t storage_head = reduced != 0u ? plan->padded_head_dim : request->head_dim;
    uint32_t row;
    for (row = 0u; row < request->tokens; ++row) {
      memset(&copy, 0, sizeof(copy));
      copy.srcOffset = (VkDeviceSize)((uint64_t)row * storage_head * sizeof(float));
      copy.dstOffset = (VkDeviceSize)(((uint64_t)head * request->tokens + row) * request->head_dim *
                                     sizeof(float));
      copy.size = output_row_bytes;
      vkCmdCopyBuffer(command_buffer, slot->m43_head[head].output.buffer,
                      slot->m43_readback.buffer, 1u, &copy);
    }
  }
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                           PROM_M43_QUERY_READBACK_END);
  if (!prom_m43_one_buffer_barrier(
          command_buffer, &slot->m43_readback,
          (VkDeviceSize)((uint64_t)PROM_M43_HEAD_COUNT * request->tokens * request->head_dim * sizeof(float)),
          VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_HOST_READ_BIT,
          VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_HOST_BIT)) return 0;
  if (leave_open != 0u) return 1;
  return vkEndCommandBuffer(command_buffer) == VK_SUCCESS ? 1 : 0;
}

static int prom_m43_record_grouped(prom_reduction_runtime_state* state,
                                   prom_reduction_slot* slot,
                                   const prom_m43_attention_group_request* request,
                                   const prom_m43_attention_plan* plan,
                                   const PrometheusReductionPlan* reduction_plan,
                                   uint32_t* out_partial_fault,
                                   uint32_t* out_uncertain_fault) {
  return prom_m43_record_grouped_internal(state, slot, request, plan, reduction_plan,
                                          out_partial_fault, out_uncertain_fault, 1u, 0u,
                                          VK_NULL_HANDLE, NULL, 0u, 0u);
}

static int prom_m43_execute_sequential_baseline(prom_reduction_runtime_state* state,
                                                prom_reduction_slot* slot,
                                                const prom_m43_attention_group_request* request,
                                                const prom_m43_attention_plan* plan,
                                                const PrometheusReductionPlan* reduction_plan,
                                                uint64_t* timestamps,
                                                uint64_t* out_recording_ns,
                                                uint64_t* out_submission_ns) {
  uint32_t head;
  VkCommandBuffer command_buffer = slot->command_buffer;
  const VkDeviceSize output_row_bytes = (VkDeviceSize)((uint64_t)request->head_dim * sizeof(float));
  for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
    const uint32_t query_base = PROM_M43_QUERY_HEAD_BASE + head * PROM_M43_QUERY_HEAD_STRIDE;
    const uint32_t reduced = plan->selected_path[head] != PROM_M42_PATH_A2X4;
    const uint32_t storage_head = reduced != 0u ? plan->padded_head_dim : request->head_dim;
    const uint32_t storage_tokens = reduced != 0u ? plan->padded_tokens : request->tokens;
    const VkDeviceSize output_bytes =
        (VkDeviceSize)((uint64_t)storage_tokens * storage_head * sizeof(float));
    VkCommandBufferBeginInfo begin_info;
    VkBufferCopy copy;
    VkSubmitInfo submit;
    VkResult result;
    uint64_t begin_ns;
    uint32_t partial_fault = 0u;
    uint32_t uncertain_fault = 0u;
    uint32_t row;
    int tail_status;
    begin_ns = prom_reduction_now_ns();
    if (vkResetCommandBuffer(command_buffer, 0u) != VK_SUCCESS) return 0;
    memset(&begin_info, 0, sizeof(begin_info));
    begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
    begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
    if (vkBeginCommandBuffer(command_buffer, &begin_info) != VK_SUCCESS) return 0;
    if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE) {
      vkCmdResetQueryPool(command_buffer, state->query_pool,
                          slot->slot_id * PROM_REDUCTION_QUERY_STRIDE + query_base,
                          PROM_M43_QUERY_HEAD_STRIDE);
    }
    prom_m43_record_projection(state, slot, command_buffer, request, plan, head, PROM_M42_STAGE_PROJECT_Q);
    prom_m43_record_projection(state, slot, command_buffer, request, plan, head, PROM_M42_STAGE_PROJECT_K);
    prom_m43_record_projection(state, slot, command_buffer, request, plan, head, PROM_M42_STAGE_PROJECT_V);
    if (!prom_m43_record_projection_barrier(command_buffer, slot, request, plan, head, 1u)) return 0;
    tail_status = prom_m43_record_head_tail(state, slot, command_buffer, request, plan,
                                            reduction_plan, head, &partial_fault, &uncertain_fault);
    if (tail_status != 1 || partial_fault != 0u || uncertain_fault != 0u) return 0;
    if (!prom_m43_one_buffer_barrier(command_buffer, &slot->m43_head[head].output, output_bytes,
                                     VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                                     VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                     VK_PIPELINE_STAGE_TRANSFER_BIT)) return 0;
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                             query_base + 22u);
    for (row = 0u; row < request->tokens; ++row) {
      memset(&copy, 0, sizeof(copy));
      copy.srcOffset = (VkDeviceSize)((uint64_t)row * storage_head * sizeof(float));
      copy.dstOffset = (VkDeviceSize)(((uint64_t)head * request->tokens + row) * request->head_dim *
                                     sizeof(float));
      copy.size = output_row_bytes;
      vkCmdCopyBuffer(command_buffer, slot->m43_head[head].output.buffer,
                      slot->m43_readback.buffer, 1u, &copy);
    }
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                             query_base + 23u);
    if (!prom_m43_one_buffer_barrier(
            command_buffer, &slot->m43_readback,
            (VkDeviceSize)((uint64_t)PROM_M43_HEAD_COUNT * request->tokens * request->head_dim * sizeof(float)),
            VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_HOST_READ_BIT,
            VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_HOST_BIT)) return 0;
    if (vkEndCommandBuffer(command_buffer) != VK_SUCCESS) return 0;
    if (out_recording_ns != NULL)
      *out_recording_ns += prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
    if (vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS) return 0;
    memset(&submit, 0, sizeof(submit));
    submit.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
    submit.commandBufferCount = 1u;
    submit.pCommandBuffers = &command_buffer;
    begin_ns = prom_reduction_now_ns();
    result = vkQueueSubmit(state->queue, 1u, &submit, slot->fence);
    if (out_submission_ns != NULL)
      *out_submission_ns += prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
    if (result != VK_SUCCESS) return 0;
    slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
    result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
    if (result != VK_SUCCESS) {
      slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
      state->diagnostics.quarantine_count += 1u;
      return 0;
    }
    if (state->timestamp_supported == 0u || state->query_pool == VK_NULL_HANDLE ||
        vkGetQueryPoolResults(state->device, state->query_pool,
                              slot->slot_id * PROM_REDUCTION_QUERY_STRIDE + query_base,
                              PROM_M43_QUERY_HEAD_STRIDE,
                              sizeof(uint64_t) * PROM_M43_QUERY_HEAD_STRIDE,
                              timestamps + query_base, sizeof(uint64_t),
                              VK_QUERY_RESULT_64_BIT) != VK_SUCCESS) return 0;
    slot->state = PROM_ASYNC_PHYSICAL_PREPARING;
  }
  return 1;
}

static uint64_t prom_m43_retained_bytes(const prom_reduction_runtime_state* state,
                                        const prom_reduction_slot* slot) {
  uint64_t total = prom_m43_weight_retained_bytes(state) +
                   (uint64_t)state->m43_resident_x_upload.size +
                   (uint64_t)state->m43_resident_x_f32.size +
                   (uint64_t)state->m43_resident_x_f16.size +
                   (uint64_t)slot->m43_x_upload.size +
                   (uint64_t)slot->m43_x_f32.size +
                   (uint64_t)slot->m43_x_f16.size +
                   (uint64_t)slot->m43_readback.size +
                   (uint64_t)slot->scratch.size +
                   (uint64_t)slot->row_max.size +
                   (uint64_t)slot->row_sum.size;
  uint32_t head;
  for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
    const prom_m43_head_slot* head_slot = &slot->m43_head[head];
    total += (uint64_t)head_slot->q.size + (uint64_t)head_slot->k.size + (uint64_t)head_slot->v.size;
    total += (uint64_t)head_slot->q_packed.size + (uint64_t)head_slot->k_transposed.size +
             (uint64_t)head_slot->v_packed.size;
    total += (uint64_t)head_slot->scores.size + (uint64_t)head_slot->probabilities.size +
             (uint64_t)head_slot->p_packed.size + (uint64_t)head_slot->output.size;
  }
  return total;
}

int prom_reactor_runtime_m43_execute(void* handle,
                                     const prom_m43_attention_group_request* request,
                                     prom_m43_attention_group_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_reduction_slot* slot;
  prom_vk_runtime_services services_before;
  prom_vk_runtime_services services_after;
  prom_m43_plan_request plan_request;
  PrometheusReductionRequest reduction_request;
  PrometheusReductionPlan reduction_plan;
  const prom_vk_buffer* x_f32;
  const prom_vk_buffer* x_f16;
  uint64_t timestamps[PROM_M43_QUERY_COUNT];
  uint64_t begin_ns = prom_reduction_now_ns();
  uint64_t validation_begin;
  uint64_t recording_begin;
  uint64_t submission_begin;
  uint64_t readback_begin;
  uint64_t readback_cpu_ns;
  uint64_t shared_x_hash;
  uint64_t x_elements;
  uint64_t output_elements;
  uint32_t finite = 0u;
  uint32_t head;
  uint32_t weight;
  uint32_t partial_fault = 0u;
  uint32_t uncertain_fault = 0u;
  int record_status;
  int32_t detail = 0;
  VkSubmitInfo submit;
  VkResult result;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  if (request == NULL || request->output == NULL || request->head_count != PROM_M43_HEAD_COUNT ||
      request->fault_point > PROM_M43_FAULT_FINAL_READBACK ||
      (request->input_mode == PROM_M42_INPUT_HOST_X && request->host_x == NULL) ||
      (request->fault_point >= PROM_M43_FAULT_HEAD_QK &&
       request->fault_point <= PROM_M43_FAULT_HEAD_PV_SUBMIT &&
       request->fault_head >= PROM_M43_HEAD_COUNT) ||
      (request->execution_strategy == PROM_M43_STRATEGY_EIGHT_SEQUENTIAL_M42 &&
       (request->input_mode != PROM_M42_INPUT_RESIDENT_X || request->fault_point != PROM_M43_FAULT_NONE))) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = request != NULL && request->head_count != PROM_M43_HEAD_COUNT
                                  ? PROM_M43_DETAIL_HEAD_COUNT
                                  : PROM_M43_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  if (!prom_m40b_checked_product_u64(request->tokens, request->model_width, &x_elements) ||
      !prom_m40b_checked_product_u64(request->tokens, request->head_dim, &output_elements) ||
      !prom_m43_checked_scale_u64(output_elements, PROM_M43_HEAD_COUNT, &output_elements)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M43_DETAIL_SIZE_OVERFLOW;
    return PROM_ERROR;
  }
  if (request->output_element_count != output_elements ||
      (request->input_mode == PROM_M42_INPUT_HOST_X && request->host_x_element_count != x_elements) ||
      (request->input_mode == PROM_M42_INPUT_RESIDENT_X && request->host_x_element_count != 0u)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M43_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL || prom_reactor_runtime_get_vk_services(handle, &services_before) != PROM_OK) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = state == NULL ? detail : PROM_M43_DETAIL_CAPABILITY;
    return PROM_ERROR;
  }
  validation_begin = prom_reduction_now_ns();
  if (request->input_mode == PROM_M42_INPUT_HOST_X) {
    shared_x_hash = prom_m42_hash_finite_matrix(request->host_x, x_elements, &finite);
    if (finite == 0u) {
      out_result->stage = PROM_STAGE_TRANSFER_IN;
      out_result->detail_code = PROM_M43_DETAIL_NONFINITE_INPUT;
      return PROM_ERROR;
    }
  } else {
    if (state->m43_resident_x_generation == 0u ||
        request->shared_x_generation != state->m43_resident_x_generation ||
        state->m43_resident_x_tokens != request->tokens ||
        state->m43_resident_x_model_width != request->model_width) {
      out_result->stage = PROM_STAGE_INIT;
      out_result->detail_code = PROM_M43_DETAIL_STALE_X_GENERATION;
      return PROM_ERROR;
    }
    shared_x_hash = state->m43_resident_x_hash;
  }
  out_result->shared_x_validation_ns =
      prom_reduction_elapsed_ns(validation_begin, prom_reduction_now_ns());
  memset(&plan_request, 0, sizeof(plan_request));
  plan_request.head_count = request->head_count;
  plan_request.tokens = request->tokens;
  plan_request.model_width = request->model_width;
  plan_request.head_dim = request->head_dim;
  plan_request.scale = request->scale;
  plan_request.scale_explicit = request->scale_explicit;
  plan_request.precision_policy = request->precision_policy;
  plan_request.allow_fallback = request->allow_fallback;
  plan_request.input_mode = request->input_mode;
  plan_request.execution_strategy = request->execution_strategy;
  plan_request.cooperative_capability_state = services_before.cooperative_matrix_state;
  plan_request.shared_x_generation = request->shared_x_generation;
  plan_request.shared_x_hash = shared_x_hash;
  for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
    plan_request.preferred_path[head] = request->preferred_path[head];
    plan_request.rollback_active[head] = request->rollback_active[head];
    for (weight = 0u; weight < PROM_M43_WEIGHT_KIND_COUNT; ++weight) {
      if (state->m43_weight_generation[head][weight] == 0u ||
          request->required_weight_generation[head][weight] != state->m43_weight_generation[head][weight] ||
          state->m43_weight_model_width[head][weight] != request->model_width ||
          state->m43_weight_head_dim[head][weight] != request->head_dim) {
        out_result->stage = PROM_STAGE_INIT;
        out_result->detail_code = PROM_M43_DETAIL_STALE_WEIGHT_GENERATION;
        return PROM_ERROR;
      }
      plan_request.weight_generation[head][weight] = state->m43_weight_generation[head][weight];
      plan_request.weight_hash[head][weight] = state->m43_weight_hash[head][weight];
    }
  }
  if (prom_m43_attention_plan_build(&plan_request, &out_result->plan) != PROM_OK) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M43_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  if (out_result->plan.memory.exact_retained_bytes > out_result->plan.memory.capacity_limit_bytes) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M43_DETAIL_CAPACITY;
    return PROM_ERROR;
  }
  if (!prom_m42_ensure_pipelines(state)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M43_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
    if (!prom_m40b_ensure_sgemm_pipeline(state, out_result->plan.selected_path[head])) {
      out_result->stage = PROM_STAGE_INIT;
      out_result->detail_code = PROM_M43_DETAIL_RESOURCE;
      return PROM_ERROR;
    }
  }
  memset(&reduction_request, 0, sizeof(reduction_request));
  reduction_request.struct_size = sizeof(reduction_request);
  reduction_request.row_count = request->tokens;
  reduction_request.elements_per_row = request->tokens;
  reduction_request.input_element_count = (uint64_t)request->tokens * request->tokens;
  reduction_request.output_element_count = reduction_request.input_element_count;
  reduction_request.operation = PROM_REDUCTION_OPERATION_SOFTMAX;
  reduction_request.finalization = PROM_REDUCTION_FINALIZATION_STABLE_SOFTMAX;
  if (prom_reactor_reduction_plan_impl(&reduction_request, &reduction_plan) != PROM_OK ||
      reduction_plan.stage_count == 0u || reduction_plan.stage_count > 5u) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M43_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  out_result->logical_request_id = state->next_logical_request_id++;
  state->diagnostics.next_logical_request_id = state->next_logical_request_id;
  slot = prom_reduction_acquire_slot(state, out_result->logical_request_id);
  if (slot == NULL) {
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M43_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  out_result->physical_slot_id = slot->slot_id;
  out_result->physical_slot_generation = slot->generation;
  if (!prom_m43_prepare_execution_buffers(state, slot, request, &out_result->plan, 1u)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M43_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  if (request->input_mode == PROM_M42_INPUT_HOST_X) {
    memcpy(slot->m43_x_upload.mapped, request->host_x, (size_t)(x_elements * sizeof(float)));
    x_f32 = &slot->m43_x_f32;
    x_f16 = &slot->m43_x_f16;
  } else {
    x_f32 = &state->m43_resident_x_f32;
    x_f16 = &state->m43_resident_x_f16;
  }
  if (!prom_m43_setup_descriptors(state, slot, request, out_result, x_f32, x_f16)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M43_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  memset(timestamps, 0, sizeof(timestamps));
  if (request->execution_strategy == PROM_M43_STRATEGY_EIGHT_SEQUENTIAL_M42) {
    if (!prom_m43_execute_sequential_baseline(state, slot, request, &out_result->plan,
                                              &reduction_plan, timestamps,
                                              &out_result->cpu_recording_ns,
                                              &out_result->cpu_submission_ns)) {
      if (slot->state != PROM_ASYNC_PHYSICAL_QUARANTINED) slot->state = PROM_ASYNC_PHYSICAL_READY;
      out_result->physical_slot_recyclable = slot->state == PROM_ASYNC_PHYSICAL_READY;
      out_result->stage = PROM_STAGE_SUBMIT;
      out_result->detail_code = slot->state == PROM_ASYNC_PHYSICAL_QUARANTINED
                                    ? PROM_M43_DETAIL_COMPLETION_UNCERTAIN
                                    : PROM_M43_DETAIL_QUERY;
      return PROM_ERROR;
    }
  } else {
    recording_begin = prom_reduction_now_ns();
    record_status = prom_m43_record_grouped(state, slot, request, &out_result->plan, &reduction_plan,
                                            &partial_fault, &uncertain_fault);
    out_result->cpu_recording_ns =
        prom_reduction_elapsed_ns(recording_begin, prom_reduction_now_ns());
    if (record_status == 0) {
      slot->state = PROM_ASYNC_PHYSICAL_READY;
      out_result->physical_slot_recyclable = 1u;
      out_result->stage = PROM_STAGE_SUBMIT;
      out_result->detail_code = PROM_M43_DETAIL_COMMAND;
      return PROM_ERROR;
    }
    slot->m43_command_reuse_count += 1u;
    if (vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS) {
      slot->state = PROM_ASYNC_PHYSICAL_READY;
      out_result->physical_slot_recyclable = 1u;
      out_result->stage = PROM_STAGE_SUBMIT;
      out_result->detail_code = PROM_M43_DETAIL_SUBMIT;
      return PROM_ERROR;
    }
    memset(&submit, 0, sizeof(submit));
    submit.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
    submit.commandBufferCount = 1u;
    submit.pCommandBuffers = &slot->command_buffer;
    submission_begin = prom_reduction_now_ns();
    result = vkQueueSubmit(state->queue, 1u, &submit, slot->fence);
    out_result->cpu_submission_ns =
        prom_reduction_elapsed_ns(submission_begin, prom_reduction_now_ns());
    if (result != VK_SUCCESS) {
      slot->state = PROM_ASYNC_PHYSICAL_READY;
      out_result->physical_slot_recyclable = 1u;
      out_result->stage = PROM_STAGE_SUBMIT;
      out_result->detail_code = PROM_M43_DETAIL_SUBMIT;
      return PROM_ERROR;
    }
    slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
    if (uncertain_fault != 0u) {
      slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
      state->diagnostics.quarantine_count += 1u;
      out_result->stage = PROM_M42_STAGE_PV;
      out_result->detail_code = PROM_M43_DETAIL_COMPLETION_UNCERTAIN;
      return PROM_ERROR;
    }
    result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
    if (result != VK_SUCCESS) {
      slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
      state->diagnostics.quarantine_count += 1u;
      out_result->stage = PROM_STAGE_SUBMIT;
      out_result->detail_code = PROM_M43_DETAIL_COMPLETION_UNCERTAIN;
      return PROM_ERROR;
    }
    if (partial_fault != 0u) {
      slot->state = PROM_ASYNC_PHYSICAL_READY;
      out_result->physical_slot_recyclable = 1u;
      out_result->stage = partial_fault;
      out_result->detail_code = PROM_M43_DETAIL_FAULT_INJECTED;
      return PROM_ERROR;
    }
    if (state->timestamp_supported == 0u || state->query_pool == VK_NULL_HANDLE ||
        vkGetQueryPoolResults(state->device, state->query_pool,
                              slot->slot_id * PROM_REDUCTION_QUERY_STRIDE, PROM_M43_QUERY_COUNT,
                              sizeof(timestamps), timestamps, sizeof(uint64_t),
                              VK_QUERY_RESULT_64_BIT) != VK_SUCCESS) {
      slot->state = PROM_ASYNC_PHYSICAL_READY;
      out_result->physical_slot_recyclable = 1u;
      out_result->stage = PROM_STAGE_TRANSFER_OUT;
      out_result->detail_code = PROM_M43_DETAIL_QUERY;
      return PROM_ERROR;
    }
    if (timestamps[PROM_M43_QUERY_GROUP_END] < timestamps[3u] ||
        timestamps[PROM_M43_QUERY_READBACK_END] < timestamps[PROM_M43_QUERY_READBACK_BEGIN]) {
      slot->state = PROM_ASYNC_PHYSICAL_READY;
      out_result->physical_slot_recyclable = 1u;
      out_result->stage = PROM_STAGE_TRANSFER_OUT;
      out_result->detail_code = PROM_M43_DETAIL_QUERY;
      return PROM_ERROR;
    }
    out_result->shared_x_upload_gpu_ns =
        (uint64_t)((double)(timestamps[1u] - timestamps[0u]) * state->timestamp_period_ns);
    out_result->shared_x_pack_gpu_ns =
        (uint64_t)((double)(timestamps[2u] - timestamps[1u]) * state->timestamp_period_ns);
    out_result->grouped_attention_gpu_ns =
        (uint64_t)((double)(timestamps[PROM_M43_QUERY_GROUP_END] - timestamps[3u]) *
                   state->timestamp_period_ns);
  }
  for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
    const uint32_t base = PROM_M43_QUERY_HEAD_BASE + head * PROM_M43_QUERY_HEAD_STRIDE;
    uint64_t durations[11];
    uint32_t stage_index;
    for (stage_index = 0u; stage_index < 11u; ++stage_index) {
      const uint32_t first = base + stage_index * 2u;
      if (timestamps[first + 1u] < timestamps[first]) {
        slot->state = PROM_ASYNC_PHYSICAL_READY;
        out_result->physical_slot_recyclable = 1u;
        out_result->stage = PROM_STAGE_TRANSFER_OUT;
        out_result->detail_code = PROM_M43_DETAIL_QUERY;
        return PROM_ERROR;
      }
      durations[stage_index] =
          (uint64_t)((double)(timestamps[first + 1u] - timestamps[first]) * state->timestamp_period_ns);
    }
    out_result->q_projection_gpu_ns[head] = durations[0];
    out_result->k_projection_gpu_ns[head] = durations[1];
    out_result->v_projection_gpu_ns[head] = durations[2];
    out_result->q_pack_gpu_ns[head] = durations[3];
    out_result->k_layout_gpu_ns[head] = durations[4];
    out_result->v_pack_gpu_ns[head] = durations[5];
    out_result->qk_gpu_ns[head] = durations[6];
    out_result->scale_gpu_ns[head] = durations[7];
    out_result->softmax_gpu_ns[head] = durations[8];
    out_result->p_pack_gpu_ns[head] = durations[9];
    out_result->pv_gpu_ns[head] = durations[10];
    out_result->q_projection_total_gpu_ns += durations[0];
    out_result->k_projection_total_gpu_ns += durations[1];
    out_result->v_projection_total_gpu_ns += durations[2];
    out_result->q_pack_total_gpu_ns += durations[3];
    out_result->k_layout_total_gpu_ns += durations[4];
    out_result->v_pack_total_gpu_ns += durations[5];
    out_result->qk_total_gpu_ns += durations[6];
    out_result->scale_total_gpu_ns += durations[7];
    out_result->softmax_total_gpu_ns += durations[8];
    out_result->p_pack_total_gpu_ns += durations[9];
    out_result->pv_total_gpu_ns += durations[10];
    for (stage_index = 3u; stage_index < 11u; ++stage_index)
      out_result->post_projection_total_gpu_ns += durations[stage_index];
    if (request->execution_strategy == PROM_M43_STRATEGY_EIGHT_SEQUENTIAL_M42) {
      uint32_t stage_query;
      for (stage_query = 0u; stage_query < 11u; ++stage_query)
        out_result->grouped_attention_gpu_ns += durations[stage_query];
      if (timestamps[base + 23u] < timestamps[base + 22u]) {
        slot->state = PROM_ASYNC_PHYSICAL_READY;
        out_result->physical_slot_recyclable = 1u;
        out_result->stage = PROM_STAGE_TRANSFER_OUT;
        out_result->detail_code = PROM_M43_DETAIL_QUERY;
        return PROM_ERROR;
      }
      out_result->final_readback_ns +=
          (uint64_t)((double)(timestamps[base + 23u] - timestamps[base + 22u]) *
                     state->timestamp_period_ns);
    }
  }
  out_result->projection_total_gpu_ns = out_result->q_projection_total_gpu_ns +
                                        out_result->k_projection_total_gpu_ns +
                                        out_result->v_projection_total_gpu_ns;
  readback_begin = prom_reduction_now_ns();
  memcpy(request->output, slot->m43_readback.mapped,
         (size_t)((uint64_t)PROM_M43_HEAD_COUNT * request->tokens * request->head_dim * sizeof(float)));
  readback_cpu_ns = prom_reduction_elapsed_ns(readback_begin, prom_reduction_now_ns());
  if (request->execution_strategy != PROM_M43_STRATEGY_EIGHT_SEQUENTIAL_M42) {
    out_result->final_readback_ns =
        (uint64_t)((double)(timestamps[PROM_M43_QUERY_READBACK_END] -
                           timestamps[PROM_M43_QUERY_READBACK_BEGIN]) * state->timestamp_period_ns);
  }
  out_result->final_readback_ns += readback_cpu_ns;
  out_result->submit_count = out_result->plan.submit_count;
  out_result->final_readback_count = out_result->plan.final_readback_count;
  out_result->no_intermediate_host_copy = 1u;
  out_result->shared_x_conversion_count = out_result->plan.shared_x_conversion_count;
  out_result->shared_x_upload_count = out_result->plan.shared_x_upload_count;
  out_result->shared_x_consumer_count = out_result->plan.shared_x_consumer_count;
  out_result->persistent_weight_count = out_result->plan.persistent_weight_count;
  out_result->qkv_projection_dispatch_count = out_result->plan.qkv_projection_dispatch_count;
  out_result->exact_request_bytes = out_result->plan.memory.exact_retained_bytes;
  out_result->retained_bytes = prom_m43_retained_bytes(state, slot);
  out_result->buffer_allocation_count = state->m43_buffer_grow_count;
  out_result->buffer_reuse_count = state->m43_buffer_reuse_count;
  out_result->descriptor_update_count = state->m43_descriptor_update_count;
  out_result->pipeline_create_count = state->m42_pipeline_create_count + state->m40b_pipeline_create_count +
                                      state->diagnostics.pipeline_create_count;
  out_result->command_buffer_reuse_count = slot->m43_command_reuse_count;
  out_result->shared_x_generation = request->shared_x_generation;
  memcpy(out_result->weight_generation, state->m43_weight_generation,
         sizeof(out_result->weight_generation));
  out_result->validation_error_count_before = services_before.validation_error_count;
  for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
    if (out_result->plan.selected_path[head] == PROM_M42_PATH_COOPERATIVE) {
      (void)prom_reactor_runtime_mark_cooperative_matrix_executable(handle);
      break;
    }
  }
  if (prom_reactor_runtime_get_vk_services(handle, &services_after) == PROM_OK)
    out_result->validation_error_count_after = services_after.validation_error_count;
  out_result->end_to_end_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->physical_slot_recyclable = 1u;
  out_result->stage = 0u;
  out_result->detail_code = 0;
  return PROM_OK;
}

static int prom_m44_prepare_execution_buffers(prom_reduction_runtime_state* state,
                                              prom_reduction_slot* slot,
                                              const prom_m44_output_projection_plan* plan,
                                              uint32_t host_upload,
                                              uint32_t final_readback) {
  const uint64_t logical_concat_elements = (uint64_t)plan->tokens * plan->concatenated_width;
  const uint64_t padded_concat_elements =
      (uint64_t)plan->padded_tokens * plan->padded_concatenated_width;
  const VkDeviceSize f32_concat_bytes = (VkDeviceSize)(logical_concat_elements * sizeof(float));
  const VkDeviceSize f16_concat_bytes =
      (VkDeviceSize)(((padded_concat_elements + 1u) / 2u) * sizeof(uint32_t));
  const VkDeviceSize output_bytes = (VkDeviceSize)plan->memory.final_y_bytes;
  const VkDeviceSize readback_bytes = (VkDeviceSize)plan->memory.final_readback_bytes;
  if (host_upload != 0u &&
      !prom_m44_ensure_buffer(state, &slot->m44_concat_upload,
                              plan->projection_path == PROM_M44_PROJECTION_A2X4_FP32
                                  ? f32_concat_bytes
                                  : f16_concat_bytes,
                              VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                              1, NULL)) return 0;
  if (plan->aggregation_strategy == PROM_M44_AGGREGATION_INTERLEAVE || host_upload != 0u) {
    if (plan->projection_path == PROM_M44_PROJECTION_A2X4_FP32) {
      if (!prom_m44_ensure_buffer(state, &slot->m44_concat_f32, f32_concat_bytes,
                                  VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                  VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL)) return 0;
    } else if (!prom_m44_ensure_buffer(state, &slot->m44_concat_f16, f16_concat_bytes,
                                       VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                       VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL)) {
      return 0;
    }
  }
  if (!prom_m44_ensure_buffer(state, &slot->m44_output, output_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL)) return 0;
  if (final_readback != 0u &&
      !prom_m44_ensure_buffer(state, &slot->m44_readback, readback_bytes,
                              VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                              VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                              1, NULL)) return 0;
  return 1;
}

static int prom_m44_setup_descriptors(prom_reduction_runtime_state* state,
                                      prom_reduction_slot* slot,
                                      const prom_m44_output_projection_plan* plan) {
  const prom_vk_buffer* buffers[PROM_M44_WIDE_DESCRIPTOR_BINDING_COUNT];
  const prom_vk_buffer* concatenated;
  uint32_t head;
  if (plan->aggregation_strategy == PROM_M44_AGGREGATION_INTERLEAVE) {
    concatenated = plan->projection_path == PROM_M44_PROJECTION_A2X4_FP32
                       ? &slot->m44_concat_f32
                       : &slot->m44_concat_f16;
    for (head = 0u; head < PROM_M44_HEAD_COUNT; ++head) buffers[head] = &slot->m43_head[head].output;
    buffers[8] = concatenated;
    buffers[9] = concatenated;
    prom_m44_update_wide_descriptor(state, slot->m44_descriptor_set, buffers);
    prom_m44_update_sgemm_descriptor(state, slot->m44_sgemm_descriptor_set, concatenated,
                                     plan->projection_path == PROM_M44_PROJECTION_A2X4_FP32
                                         ? &state->m44_wo_f32
                                         : &state->m44_wo_f16,
                                     &slot->m44_output);
  } else {
    for (head = 0u; head < PROM_M44_HEAD_COUNT; ++head) buffers[head] = &slot->m43_head[head].output;
    buffers[8] = &state->m44_wo_f16;
    buffers[9] = &slot->m44_output;
    prom_m44_update_wide_descriptor(state, slot->m44_descriptor_set, buffers);
  }
  return 1;
}

static int prom_m44_record_projection_tail(prom_reduction_runtime_state* state,
                                           prom_reduction_slot* slot,
                                           const prom_m44_composed_request* request,
                                           const prom_m44_output_projection_plan* plan,
                                           VkCommandBuffer command_buffer,
                                           uint32_t already_open,
                                           uint32_t final_readback,
                                           uint32_t* out_partial_fault) {
  VkCommandBufferBeginInfo begin_info;
  const prom_vk_buffer* head_buffers[PROM_M44_HEAD_COUNT];
  VkDeviceSize head_sizes[PROM_M44_HEAD_COUNT];
  VkBufferCopy copy;
  uint32_t head;
  uint32_t row;
  if (out_partial_fault != NULL) *out_partial_fault = 0u;
  if (already_open == 0u) {
    if (vkResetCommandBuffer(command_buffer, 0u) != VK_SUCCESS) return 0;
    memset(&begin_info, 0, sizeof(begin_info));
    begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
    begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
    if (vkBeginCommandBuffer(command_buffer, &begin_info) != VK_SUCCESS) return 0;
  }
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE)
    vkCmdResetQueryPool(command_buffer, state->query_pool,
                        slot->active_query_base + PROM_M44_QUERY_BASE,
                        PROM_M44_QUERY_COUNT);
  for (head = 0u; head < PROM_M44_HEAD_COUNT; ++head) {
    head_buffers[head] = &slot->m43_head[head].output;
    head_sizes[head] =
        (VkDeviceSize)((uint64_t)request->attention.tokens * plan->head_row_stride * sizeof(float));
  }
  if (!prom_m43_buffer_barriers(command_buffer, head_buffers, head_sizes, PROM_M44_HEAD_COUNT,
                                VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                                VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT)) return 0;
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                           PROM_M44_QUERY_AGGREGATION_BEGIN);
  if (request->fault_point == PROM_M44_FAULT_BEFORE_AGGREGATION) {
    if (out_partial_fault != NULL) *out_partial_fault = request->fault_point;
    return 2;
  }
  if (plan->aggregation_strategy == PROM_M44_AGGREGATION_INTERLEAVE) {
    prom_m44_interleave_push_constants push;
    const uint32_t reduced = plan->projection_path != PROM_M44_PROJECTION_A2X4_FP32;
    const uint32_t output_rows = reduced != 0u ? plan->padded_tokens : plan->tokens;
    const uint32_t output_columns =
        reduced != 0u ? plan->padded_concatenated_width : plan->concatenated_width;
    const uint64_t output_elements = (uint64_t)output_rows * output_columns;
    memset(&push, 0, sizeof(push));
    push.tokens = plan->tokens;
    push.head_dim = plan->head_dim;
    push.head_row_stride = plan->head_row_stride;
    push.output_rows = output_rows;
    push.output_columns = output_columns;
    push.packed_output = reduced;
    push.work_item_count = (uint32_t)((output_elements + 1u) / 2u);
    vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                      state->m44_pipelines[0].pipeline);
    vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                            state->m44_pipeline_layout, 0u, 1u,
                            &slot->m44_descriptor_set, 0u, NULL);
    vkCmdPushConstants(command_buffer, state->m44_pipeline_layout,
                       VK_SHADER_STAGE_COMPUTE_BIT, 0u, sizeof(push), &push);
    vkCmdDispatch(command_buffer, prom_reduction_ceil_div_u32(push.work_item_count, 256u), 1u, 1u);
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M44_QUERY_AGGREGATION_END);
    if (!prom_m43_one_buffer_barrier(
            command_buffer,
            plan->projection_path == PROM_M44_PROJECTION_A2X4_FP32
                ? &slot->m44_concat_f32
                : &slot->m44_concat_f16,
            plan->projection_path == PROM_M44_PROJECTION_A2X4_FP32
                ? (VkDeviceSize)plan->memory.contiguous_f32_bytes
                : (VkDeviceSize)plan->memory.contiguous_packed_bytes,
            VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT)) return 0;
    if (request->fault_point == PROM_M44_FAULT_DURING_INTERLEAVE ||
        request->fault_point == PROM_M44_FAULT_AFTER_INTERLEAVE) {
      if (out_partial_fault != NULL) *out_partial_fault = request->fault_point;
      return 2;
    }
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M44_QUERY_PROJECTION_BEGIN);
    prom_m42_record_sgemm(state, command_buffer, slot->m44_sgemm_descriptor_set,
                          plan->projection_path,
                          plan->projection_path == PROM_M44_PROJECTION_A2X4_FP32
                              ? plan->tokens
                              : plan->padded_tokens,
                          plan->projection_path == PROM_M44_PROJECTION_A2X4_FP32
                              ? plan->model_width
                              : plan->padded_model_width,
                          plan->projection_path == PROM_M44_PROJECTION_A2X4_FP32
                              ? plan->concatenated_width
                              : plan->padded_concatenated_width);
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M44_QUERY_PROJECTION_END);
  } else {
    prom_m44_direct_push_constants push;
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M44_QUERY_AGGREGATION_END);
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M44_QUERY_PROJECTION_BEGIN);
    memset(&push, 0, sizeof(push));
    push.tokens = plan->tokens;
    push.head_dim = plan->head_dim;
    push.model_width = plan->model_width;
    push.head_row_stride = plan->head_row_stride;
    push.padded_model_width = plan->padded_model_width;
    push.total_output_elements = plan->tokens * plan->model_width;
    vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                      state->m44_pipelines[1].pipeline);
    vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                            state->m44_pipeline_layout, 0u, 1u,
                            &slot->m44_descriptor_set, 0u, NULL);
    vkCmdPushConstants(command_buffer, state->m44_pipeline_layout,
                       VK_SHADER_STAGE_COMPUTE_BIT, 0u, sizeof(push), &push);
    vkCmdDispatch(command_buffer,
                  prom_reduction_ceil_div_u32(push.total_output_elements, 256u), 1u, 1u);
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M44_QUERY_PROJECTION_END);
    if (request->fault_point == PROM_M44_FAULT_MID_DIRECT_PROJECTION) {
      if (out_partial_fault != NULL) *out_partial_fault = request->fault_point;
      return 2;
    }
  }
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                           PROM_M44_QUERY_ACCUMULATION_BEGIN);
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                           PROM_M44_QUERY_ACCUMULATION_END);
  if (final_readback == 0u) return 1;
  if (request->fault_point == PROM_M44_FAULT_BEFORE_FINAL_READBACK) {
    if (out_partial_fault != NULL) *out_partial_fault = request->fault_point;
    return 2;
  }
  if (!prom_m43_one_buffer_barrier(command_buffer, &slot->m44_output,
                                   (VkDeviceSize)plan->memory.final_y_bytes,
                                   VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT)) return 0;
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                           PROM_M44_QUERY_READBACK_BEGIN);
  for (row = 0u; row < plan->tokens; ++row) {
    memset(&copy, 0, sizeof(copy));
    copy.srcOffset = (VkDeviceSize)((uint64_t)row * plan->output_row_stride * sizeof(float));
    copy.dstOffset = (VkDeviceSize)((uint64_t)row * plan->model_width * sizeof(float));
    copy.size = (VkDeviceSize)((uint64_t)plan->model_width * sizeof(float));
    vkCmdCopyBuffer(command_buffer, slot->m44_output.buffer, slot->m44_readback.buffer, 1u, &copy);
  }
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                           PROM_M44_QUERY_READBACK_END);
  if (!prom_m43_one_buffer_barrier(command_buffer, &slot->m44_readback,
                                   (VkDeviceSize)plan->memory.final_readback_bytes,
                                   VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_HOST_READ_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_HOST_BIT)) return 0;
  return 1;
}

static int prom_m45_record_residual_tail(prom_reduction_runtime_state* state,
                                         prom_reduction_slot* slot,
                                         const prom_m45_composed_request* request,
                                         const prom_m45_residual_plan* plan,
                                         const prom_vk_buffer* x,
                                         const prom_vk_buffer* z,
                                         VkCommandBuffer command_buffer,
                                         uint32_t already_open,
                                         uint32_t* out_partial_fault) {
  VkCommandBufferBeginInfo begin_info;
  prom_m45_residual_push_constants push;
  VkBufferCopy copy;
  const VkDeviceSize x_bytes = (VkDeviceSize)plan->memory.x_view_bytes;
  const VkDeviceSize y_bytes = (VkDeviceSize)plan->memory.y_view_bytes;
  const VkDeviceSize z_bytes = plan->strategy == PROM_M45_STRATEGY_IN_PLACE_Y
                                ? y_bytes : (VkDeviceSize)plan->memory.z_device_bytes;
  uint32_t row;
  if (out_partial_fault != NULL) *out_partial_fault = 0u;
  if (already_open == 0u) {
    if (vkResetCommandBuffer(command_buffer, 0u) != VK_SUCCESS) return 0;
    memset(&begin_info, 0, sizeof(begin_info));
    begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
    begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
    if (vkBeginCommandBuffer(command_buffer, &begin_info) != VK_SUCCESS) return 0;
  }
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE)
    vkCmdResetQueryPool(command_buffer, state->query_pool,
                        slot->active_query_base + PROM_M45_QUERY_BASE,
                        PROM_M45_QUERY_COUNT);
  if (request->fault_point == PROM_M45_FAULT_BEFORE_RESIDUAL_BARRIERS) {
    if (out_partial_fault != NULL) *out_partial_fault = request->fault_point;
    return 2;
  }
  if (!prom_m43_one_buffer_barrier(command_buffer, x, x_bytes,
                                   VK_ACCESS_SHADER_READ_BIT, VK_ACCESS_SHADER_READ_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT)) return 0;
  if (request->fault_point == PROM_M45_FAULT_AFTER_X_BARRIER) {
    if (out_partial_fault != NULL) *out_partial_fault = request->fault_point;
    return 2;
  }
  if (!prom_m43_one_buffer_barrier(command_buffer, &slot->m44_output, y_bytes,
                                   VK_ACCESS_SHADER_WRITE_BIT,
                                   plan->strategy == PROM_M45_STRATEGY_IN_PLACE_Y
                                     ? VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT
                                     : VK_ACCESS_SHADER_READ_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT)) return 0;
  if (request->fault_point == PROM_M45_FAULT_AFTER_Y_BARRIER) {
    if (out_partial_fault != NULL) *out_partial_fault = request->fault_point;
    return 2;
  }
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                           PROM_M45_QUERY_RESIDUAL_BEGIN);
  memset(&push, 0, sizeof(push));
  push.tokens = plan->tokens;
  push.model_width = plan->model_width;
  push.x_row_stride = plan->x_row_stride;
  push.y_row_stride = plan->y_row_stride;
  push.z_row_stride = plan->z_row_stride;
  push.logical_element_count = plan->tokens * plan->model_width;
  vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                    state->m45_pipelines[0].pipeline);
  vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                          state->pipeline_layout, 0u, 1u, &slot->m45_descriptor_set, 0u, NULL);
  vkCmdPushConstants(command_buffer, state->pipeline_layout, VK_SHADER_STAGE_COMPUTE_BIT,
                     0u, sizeof(push), &push);
  vkCmdDispatch(command_buffer, prom_reduction_ceil_div_u32(push.logical_element_count, 256u), 1u, 1u);
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                           PROM_M45_QUERY_RESIDUAL_END);
  if (request->fault_point == PROM_M45_FAULT_DURING_RESIDUAL_DISPATCH) {
    if (out_partial_fault != NULL) *out_partial_fault = request->fault_point;
    return 2;
  }
  if (request->fault_point == PROM_M45_FAULT_BEFORE_FINAL_READBACK) {
    if (out_partial_fault != NULL) *out_partial_fault = request->fault_point;
    return 2;
  }
  if (!prom_m43_one_buffer_barrier(command_buffer, z, z_bytes,
                                   VK_ACCESS_SHADER_WRITE_BIT,
                                   plan->final_readback_count != 0u
                                     ? VK_ACCESS_TRANSFER_READ_BIT : VK_ACCESS_SHADER_READ_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                   plan->final_readback_count != 0u
                                     ? VK_PIPELINE_STAGE_TRANSFER_BIT
                                     : VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT)) return 0;
  if (plan->final_readback_count == 0u) return 1;
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                           PROM_M45_QUERY_READBACK_BEGIN);
  for (row = 0u; row < plan->tokens; ++row) {
    memset(&copy, 0, sizeof(copy));
    copy.srcOffset = (VkDeviceSize)((uint64_t)row * plan->z_row_stride * sizeof(float));
    copy.dstOffset = (VkDeviceSize)((uint64_t)row * plan->model_width * sizeof(float));
    copy.size = (VkDeviceSize)((uint64_t)plan->model_width * sizeof(float));
    vkCmdCopyBuffer(command_buffer, z->buffer, slot->m44_readback.buffer, 1u, &copy);
  }
  prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                           PROM_M45_QUERY_READBACK_END);
  if (!prom_m43_one_buffer_barrier(command_buffer, &slot->m44_readback,
                                   (VkDeviceSize)plan->memory.z_readback_bytes,
                                   VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_HOST_READ_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_HOST_BIT)) return 0;
  return 1;
}

typedef struct prom_m47_continuation prom_m47_continuation;

typedef struct prom_m46_continuation {
  const prom_m46_composed_request* request;
  prom_m46_composed_result* result;
  prom_vk_buffer* z;
  prom_vk_buffer* n;
  prom_m47_continuation* m47;
} prom_m46_continuation;

static int prom_m46_record_tail(prom_reduction_runtime_state* state,
                                prom_reduction_slot* slot,
                                const prom_m46_composed_request* request,
                                const prom_m46_rmsnorm_plan* plan,
                                prom_vk_buffer* z,
                                prom_vk_buffer* n,
                                VkCommandBuffer command_buffer,
                                uint32_t already_open,
                                uint32_t z_already_shader_readable,
                                uint32_t* out_partial_fault) {
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  uint32_t partial_fault = 0u;
  uint32_t row;
  if (out_partial_fault != NULL) *out_partial_fault = 0u;
  if (already_open == 0u) {
    if (vkResetCommandBuffer(command_buffer, 0u) != VK_SUCCESS) return 0;
    memset(&begin_info, 0, sizeof(begin_info));
    begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
    begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
    if (vkBeginCommandBuffer(command_buffer, &begin_info) != VK_SUCCESS) return 0;
  }
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE)
    vkCmdResetQueryPool(command_buffer, state->query_pool,
                        slot->active_query_base + PROM_M46_QUERY_BASE,
                        PROM_M46_QUERY_COUNT);
  if (request->fault_point == PROM_M46_FAULT_BEFORE_REDUCTION)
    partial_fault = request->fault_point;
  if (partial_fault == 0u) {
    prom_m46_reduce_push_constants push;
    if (!prom_m43_one_buffer_barrier(command_buffer, z,
                                     (VkDeviceSize)plan->memory.z_view_bytes,
                                     z_already_shader_readable != 0u
                                         ? VK_ACCESS_SHADER_READ_BIT : VK_ACCESS_SHADER_WRITE_BIT,
                                     VK_ACCESS_SHADER_READ_BIT,
                                     VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                     VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT)) return 0;
    prom_m42_write_timestamp(state, slot, command_buffer,
                             VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M46_QUERY_REDUCTION_BEGIN);
    memset(&push, 0, sizeof(push));
    push.row_count = plan->tokens;
    push.elements_per_row = plan->model_width;
    push.partials_per_row = plan->partials_per_row;
    push.input_row_stride = plan->z_row_stride;
    push.chunk_elements = 1024u;
    push.total_elements = plan->model_width;
    push.stage_role = plan->reduction_plan == PROM_M46_REDUCTION_STAGED ? 1u : 3u;
    push.epsilon = request->epsilon;
    vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                      state->m46_pipelines[0].pipeline);
    vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                            state->pipeline_layout, 0u, 1u, &slot->descriptor_sets[0], 0u, NULL);
    vkCmdPushConstants(command_buffer, state->pipeline_layout,
                       VK_SHADER_STAGE_COMPUTE_BIT, 0u, sizeof(push), &push);
    vkCmdDispatch(command_buffer, push.row_count * push.partials_per_row, 1u, 1u);
    prom_m42_write_timestamp(state, slot, command_buffer,
                             VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M46_QUERY_PARTIAL_END);
    if (request->fault_point == PROM_M46_FAULT_AFTER_FIRST_PARTIAL)
      partial_fault = request->fault_point;
    if (plan->reduction_plan == PROM_M46_REDUCTION_FUSED &&
        request->fault_point == PROM_M46_FAULT_BEFORE_FINAL_REDUCTION)
      partial_fault = request->fault_point;
  }
  if (partial_fault == 0u && plan->reduction_plan == PROM_M46_REDUCTION_STAGED) {
    prom_m46_reduce_push_constants push;
    if (request->fault_point == PROM_M46_FAULT_BEFORE_FINAL_REDUCTION)
      partial_fault = request->fault_point;
    if (partial_fault == 0u) {
      prom_m42_buffer_barrier(command_buffer, &slot->m46_partials,
                              VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                              VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                              VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
      memset(&push, 0, sizeof(push));
      push.row_count = plan->tokens;
      push.elements_per_row = plan->partials_per_row;
      push.partials_per_row = 1u;
      push.input_row_stride = plan->partials_per_row;
      push.chunk_elements = 1024u;
      push.total_elements = plan->model_width;
      push.stage_role = 2u;
      push.epsilon = request->epsilon;
      vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                        state->m46_pipelines[0].pipeline);
      vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                              state->pipeline_layout, 0u, 1u,
                              &slot->descriptor_sets[1], 0u, NULL);
      vkCmdPushConstants(command_buffer, state->pipeline_layout,
                         VK_SHADER_STAGE_COMPUTE_BIT, 0u, sizeof(push), &push);
      vkCmdDispatch(command_buffer, push.row_count, 1u, 1u);
    }
  }
  if (partial_fault == 0u) {
    prom_m42_write_timestamp(state, slot, command_buffer,
                             VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M46_QUERY_FINAL_END);
    if (request->fault_point == PROM_M46_FAULT_AFTER_INV_RMS_WRITE ||
        request->fault_point == PROM_M46_FAULT_BEFORE_APPLY)
      partial_fault = request->fault_point;
  }
  if (partial_fault == 0u) {
    prom_m46_apply_push_constants push;
    prom_m42_buffer_barrier(command_buffer, &slot->m46_inv_rms,
                            VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
    prom_m42_buffer_barrier(command_buffer, z, VK_ACCESS_SHADER_READ_BIT,
                            request->strategy == PROM_M46_STRATEGY_IN_PLACE_Z
                              ? VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT
                              : VK_ACCESS_SHADER_READ_BIT,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
    memset(&push, 0, sizeof(push));
    push.tokens = plan->tokens;
    push.model_width = plan->model_width;
    push.z_row_stride = plan->z_row_stride;
    push.n_row_stride = plan->n_row_stride;
    push.logical_element_count = plan->tokens * plan->model_width;
    prom_m42_write_timestamp(state, slot, command_buffer,
                             VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M46_QUERY_APPLY_BEGIN);
    vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                      state->m46_pipelines[1].pipeline);
    vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                            state->pipeline_layout, 0u, 1u,
                            &slot->descriptor_sets[2], 0u, NULL);
    vkCmdPushConstants(command_buffer, state->pipeline_layout,
                       VK_SHADER_STAGE_COMPUTE_BIT, 0u, sizeof(push), &push);
    vkCmdDispatch(command_buffer,
                  prom_reduction_ceil_div_u32(push.logical_element_count, 256u), 1u, 1u);
    prom_m42_write_timestamp(state, slot, command_buffer,
                             VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M46_QUERY_APPLY_END);
    if (request->fault_point == PROM_M46_FAULT_DURING_APPLY)
      partial_fault = request->fault_point;
  }
  if (partial_fault == 0u && request->output != NULL) {
    if (request->fault_point == PROM_M46_FAULT_BEFORE_FINAL_READBACK)
      partial_fault = request->fault_point;
    if (partial_fault == 0u) {
      prom_m42_buffer_barrier(command_buffer, n,
                              VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                              VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                              VK_PIPELINE_STAGE_TRANSFER_BIT);
      prom_m42_write_timestamp(state, slot, command_buffer,
                               VK_PIPELINE_STAGE_TRANSFER_BIT,
                               PROM_M46_QUERY_READBACK_BEGIN);
      for (row = 0u; row < plan->tokens; ++row) {
        memset(&copy, 0, sizeof(copy));
        copy.srcOffset = (VkDeviceSize)((uint64_t)row * plan->n_row_stride * sizeof(float));
        copy.dstOffset = (VkDeviceSize)((uint64_t)row * plan->model_width * sizeof(float));
        copy.size = (VkDeviceSize)((uint64_t)plan->model_width * sizeof(float));
        vkCmdCopyBuffer(command_buffer, n->buffer, slot->m46_readback.buffer, 1u, &copy);
      }
      prom_m42_write_timestamp(state, slot, command_buffer,
                               VK_PIPELINE_STAGE_TRANSFER_BIT,
                               PROM_M46_QUERY_READBACK_END);
      prom_m42_buffer_barrier(command_buffer, &slot->m46_readback,
                              VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_HOST_READ_BIT,
                              VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_HOST_BIT);
    }
  }
  if (out_partial_fault != NULL) *out_partial_fault = partial_fault;
  return partial_fault != 0u ? 2 : 1;
}

static int prom_m46_prepare_continuation(prom_reduction_runtime_state* state,
                                         prom_reduction_slot* slot,
                                         prom_m45_composed_result* upstream,
                                         prom_m46_continuation* continuation) {
  prom_m46_plan_request plan_request;
  prom_m46_composed_result* result = continuation->result;
  const prom_m46_composed_request* request = continuation->request;
  prom_vk_buffer* z = upstream->z_view.buffer == slot->m44_output.buffer
                        ? &slot->m44_output
                        : upstream->z_view.buffer == slot->m49a_m46_z.buffer
                            ? &slot->m49a_m46_z : &slot->m45_output;
  prom_vk_buffer* n;
  memset(&plan_request, 0, sizeof(plan_request));
  plan_request.z_view = upstream->z_view;
  plan_request.tokens = request->upstream.attention.tokens;
  plan_request.model_width = request->upstream.attention.model_width;
  plan_request.epsilon = request->epsilon;
  plan_request.strategy = request->strategy;
  plan_request.submit_policy = request->submit_policy;
  plan_request.z_exclusive = 1u;
  plan_request.pre_normalization_z_consumer_count = 0u;
  plan_request.final_readback = request->output != NULL ? 1u : 0u;
  plan_request.requested_reduction_plan = request->requested_reduction_plan;
  plan_request.expected_z_generation = upstream->residual_plan.z_generation;
  plan_request.weight_generation = state->m46_weight_generation;
  plan_request.weight_hash = state->m46_weight_hash;
  plan_request.m45_replay_id = upstream->residual_plan.replay_id;
  if (prom_m46_rmsnorm_plan_build(&plan_request, &result->rmsnorm_plan) != PROM_OK ||
      result->rmsnorm_plan.eligibility_eligible == 0u) return 0;
  if (!prom_m46_ensure_buffer(state, &slot->m46_partials,
                              (VkDeviceSize)(result->rmsnorm_plan.memory.partial_sum_bytes != 0u
                                                ? result->rmsnorm_plan.memory.partial_sum_bytes
                                                : sizeof(float)),
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_m46_ensure_buffer(state, &slot->m46_inv_rms,
                              (VkDeviceSize)result->rmsnorm_plan.memory.inv_rms_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT |
                                  VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      (request->strategy == PROM_M46_STRATEGY_SEPARATE_OUTPUT &&
       !prom_m46_ensure_buffer(state, &slot->m46_output,
                               (VkDeviceSize)result->rmsnorm_plan.memory.n_device_bytes,
                               VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) ||
      (request->output != NULL &&
       !prom_m46_ensure_buffer(state, &slot->m46_readback,
                               (VkDeviceSize)result->rmsnorm_plan.memory.n_readback_bytes,
                               VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                               VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                               1))) return 0;
  n = request->strategy == PROM_M46_STRATEGY_IN_PLACE_Z ? z : &slot->m46_output;
  prom_m46_update_descriptor(state, slot->descriptor_sets[0], z, &slot->m46_inv_rms,
                             &slot->m46_inv_rms,
                             result->rmsnorm_plan.reduction_plan == PROM_M46_REDUCTION_STAGED
                               ? &slot->m46_partials : &slot->m46_inv_rms);
  if (result->rmsnorm_plan.reduction_plan == PROM_M46_REDUCTION_STAGED)
    prom_m46_update_descriptor(state, slot->descriptor_sets[1], &slot->m46_partials,
                               &slot->m46_inv_rms, &slot->m46_inv_rms,
                               &slot->m46_inv_rms);
  prom_m46_update_descriptor(state, slot->descriptor_sets[2], z, &state->m46_weight,
                             &slot->m46_inv_rms, n);
  continuation->z = z;
  continuation->n = n;
  result->logical_request_id = upstream->logical_request_id;
  result->physical_slot_id = slot->slot_id;
  result->physical_slot_generation = slot->generation;
  return 1;
}

static int prom_m46_complete_continuation(prom_reduction_runtime_state* state,
                                          prom_reduction_slot* slot,
                                          prom_m45_composed_result* upstream,
                                          prom_m46_continuation* continuation,
                                          const uint64_t* timestamps,
                                          uint64_t begin_ns) {
  prom_m46_composed_result* result = continuation->result;
  const prom_m46_composed_request* request = continuation->request;
  const prom_m46_rmsnorm_plan* plan = &result->rmsnorm_plan;
  const uint64_t logical_elements = (uint64_t)plan->tokens * plan->model_width;
  uint64_t readback_begin;
  result->reduction_gpu_ns =
      (uint64_t)((double)(timestamps[PROM_M46_QUERY_PARTIAL_END] -
                         timestamps[PROM_M46_QUERY_REDUCTION_BEGIN]) * state->timestamp_period_ns);
  result->final_reduction_gpu_ns = plan->reduction_plan == PROM_M46_REDUCTION_STAGED
      ? (uint64_t)((double)(timestamps[PROM_M46_QUERY_FINAL_END] -
                           timestamps[PROM_M46_QUERY_PARTIAL_END]) * state->timestamp_period_ns)
      : 0u;
  result->inv_rms_gpu_ns =
      (uint64_t)((double)(timestamps[PROM_M46_QUERY_FINAL_END] -
                         timestamps[PROM_M46_QUERY_REDUCTION_BEGIN]) * state->timestamp_period_ns);
  result->apply_gpu_ns =
      (uint64_t)((double)(timestamps[PROM_M46_QUERY_APPLY_END] -
                         timestamps[PROM_M46_QUERY_APPLY_BEGIN]) * state->timestamp_period_ns);
  result->m46_gpu_ns =
      (uint64_t)((double)(timestamps[PROM_M46_QUERY_APPLY_END] -
                         timestamps[PROM_M46_QUERY_REDUCTION_BEGIN]) * state->timestamp_period_ns);
  result->total_m43_m44_m45_m46_gpu_ns =
      (uint64_t)((double)(timestamps[PROM_M46_QUERY_APPLY_END] - timestamps[3u]) *
                 state->timestamp_period_ns);
  if (request->output != NULL) {
    if (timestamps[PROM_M46_QUERY_READBACK_END] < timestamps[PROM_M46_QUERY_READBACK_BEGIN])
      return 0;
    readback_begin = prom_reduction_now_ns();
    memcpy(request->output, slot->m46_readback.mapped,
           (size_t)(logical_elements * sizeof(float)));
    result->final_readback_ns =
        (uint64_t)((double)(timestamps[PROM_M46_QUERY_READBACK_END] -
                           timestamps[PROM_M46_QUERY_READBACK_BEGIN]) * state->timestamp_period_ns) +
        prom_reduction_elapsed_ns(readback_begin, prom_reduction_now_ns());
  }
  memset(&result->n_view, 0, sizeof(result->n_view));
  result->n_view.buffer = continuation->n->buffer;
  result->n_view.byte_length = continuation->n->size;
  result->n_view.element_type = PROM_DEVICE_ELEMENT_F32;
  result->n_view.logical_rows = plan->tokens;
  result->n_view.logical_columns = plan->model_width;
  result->n_view.row_stride_elements = plan->n_row_stride;
  result->n_view.layout = PROM_DEVICE_LAYOUT_ROW_MAJOR;
  result->n_view.producer_access = request->output != NULL
                                     ? PROM_DEVICE_ACCESS_TRANSFER_READ
                                     : PROM_DEVICE_ACCESS_COMPUTE_WRITE;
  result->n_view.required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
  result->n_view.owning_device = state->device;
  result->n_view.owning_lifetime_id = plan->n_generation;
  result->n_view.owning_slot_id = slot->slot_id;
  result->n_view.owning_slot_generation = slot->generation;
  result->submit_count = request->submit_policy == PROM_M46_SUBMIT_TWO_BOUNDED ? 2u : 1u;
  result->final_readback_count = request->output != NULL ? 1u : 0u;
  result->no_intermediate_host_copy = 1u;
  result->z_generation = plan->z_generation;
  result->weight_generation = state->m46_weight_generation;
  result->n_generation = plan->n_generation;
  result->exact_request_bytes = plan->memory.exact_request_bytes;
  result->retained_bytes = upstream->retained_bytes +
                           (uint64_t)state->m46_weight_upload.size +
                           (uint64_t)state->m46_weight.size +
                           (uint64_t)slot->m46_partials.size +
                           (uint64_t)slot->m46_inv_rms.size +
                           (uint64_t)slot->m46_output.size +
                           (uint64_t)slot->m46_readback.size;
  result->buffer_allocation_count = upstream->buffer_allocation_count +
                                    state->m46_buffer_grow_count;
  result->buffer_reuse_count = upstream->buffer_reuse_count + state->m46_buffer_reuse_count;
  result->descriptor_update_count = upstream->descriptor_update_count +
                                    state->m46_descriptor_update_count;
  result->pipeline_create_count = upstream->pipeline_create_count +
                                  state->m46_pipeline_create_count;
  result->command_buffer_reuse_count = slot->m46_command_reuse_count;
  result->cpu_recording_ns = upstream->cpu_recording_ns;
  result->cpu_submission_ns = upstream->cpu_submission_ns;
  result->end_to_end_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  result->stage = 0u;
  result->detail_code = 0;
  return 1;
}

struct prom_m47_continuation {
  const prom_m47_composed_request* request;
  prom_m47_composed_result* result;
  prom_vk_buffer* n;
  prom_vk_buffer* output;
};

static int prom_m47_prepare_continuation(prom_reduction_runtime_state* state,
                                         prom_reduction_slot* slot,
                                         prom_m46_continuation* upstream,
                                         prom_m47_continuation* continuation) {
  prom_m47_plan_request plan_request;
  prom_m47_composed_result* result = continuation->result;
  const prom_m47_composed_request* request = continuation->request;
  const uint32_t reduced = request->projection_path != PROM_M47_PROJECTION_A2X4_FP32;
  const prom_vk_buffer* projection_input;
  const prom_vk_buffer* hidden_input;
  prom_vk_buffer* output;
  prom_vk_buffer* activated;
  prom_vk_buffer* hidden;
  uint32_t weight;
  memset(&plan_request, 0, sizeof(plan_request));
  plan_request.n_view.buffer = upstream->n->buffer;
  plan_request.n_view.byte_length = upstream->n->size;
  plan_request.n_view.element_type = PROM_DEVICE_ELEMENT_F32;
  plan_request.n_view.logical_rows = upstream->result->rmsnorm_plan.tokens;
  plan_request.n_view.logical_columns = upstream->result->rmsnorm_plan.model_width;
  plan_request.n_view.row_stride_elements = upstream->result->rmsnorm_plan.n_row_stride;
  plan_request.n_view.layout = PROM_DEVICE_LAYOUT_ROW_MAJOR;
  plan_request.n_view.producer_access = PROM_DEVICE_ACCESS_COMPUTE_WRITE;
  plan_request.n_view.required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
  plan_request.n_view.owning_device = state->device;
  plan_request.n_view.owning_lifetime_id = upstream->result->rmsnorm_plan.n_generation;
  plan_request.n_view.owning_slot_id = slot->slot_id;
  plan_request.n_view.owning_slot_generation = slot->generation;
  plan_request.tokens = upstream->result->rmsnorm_plan.tokens;
  plan_request.model_width = upstream->result->rmsnorm_plan.model_width;
  plan_request.ffn_width = request->ffn_width;
  plan_request.projection_path = request->projection_path;
  plan_request.gating_strategy = request->gating_strategy;
  plan_request.residual_strategy = request->residual_strategy;
  plan_request.submit_policy = request->submit_policy;
  plan_request.n_exclusive = 0u;
  plan_request.remaining_n_consumer_count = 1u;
  plan_request.final_readback = request->output != NULL ? 1u : 0u;
  plan_request.expected_n_generation = upstream->result->rmsnorm_plan.n_generation;
  plan_request.m46_replay_id = upstream->result->rmsnorm_plan.replay_id;
  for (weight = 0u; weight < PROM_M47_WEIGHT_COUNT; ++weight) {
    plan_request.weight_generation[weight] = state->m47_weight_generation[weight];
    plan_request.weight_hash[weight] = state->m47_weight_hash[weight];
  }
  if (prom_m47_gated_ffn_plan_build(&plan_request, &result->ffn_plan) != PROM_OK ||
      result->ffn_plan.eligibility_eligible == 0u) return 0;
  if (!prom_m47_ensure_buffer(state, &slot->m47_gate,
                              (VkDeviceSize)result->ffn_plan.memory.gate_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT |
                                  VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_m47_ensure_buffer(state, &slot->m47_up,
                              (VkDeviceSize)result->ffn_plan.memory.up_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT |
                                  VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_m47_ensure_buffer(state, &slot->m47_down,
                              (VkDeviceSize)result->ffn_plan.memory.down_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      (reduced != 0u &&
       !prom_m47_ensure_buffer(state, &slot->m47_n_packed,
                               (VkDeviceSize)result->ffn_plan.memory.n_packed_bytes,
                               VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) ||
      (request->gating_strategy == PROM_M47_GATING_SEPARATE &&
       !prom_m47_ensure_buffer(state, &slot->m47_activated_gate,
                               (VkDeviceSize)result->ffn_plan.memory.activated_gate_bytes,
                               VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) ||
      (request->gating_strategy != PROM_M47_GATING_FUSED_DIRECT_PACKED &&
       !prom_m47_ensure_buffer(state, &slot->m47_hidden,
                               (VkDeviceSize)result->ffn_plan.memory.hidden_f32_bytes,
                               VK_BUFFER_USAGE_STORAGE_BUFFER_BIT |
                                   VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) ||
      (reduced != 0u &&
       !prom_m47_ensure_buffer(state, &slot->m47_hidden_packed,
                               (VkDeviceSize)result->ffn_plan.memory.hidden_packed_bytes,
                               VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) ||
      (request->residual_strategy == PROM_M47_RESIDUAL_SEPARATE_OUTPUT &&
       !prom_m47_ensure_buffer(state, &slot->m47_output,
                               (VkDeviceSize)result->ffn_plan.memory.separate_output_bytes,
                               VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) ||
      (request->output != NULL &&
       !prom_m47_ensure_buffer(state, &slot->m47_readback,
                               (VkDeviceSize)result->ffn_plan.memory.final_readback_bytes,
                               VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                               VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                               1))) return 0;
  projection_input = reduced != 0u ? &slot->m47_n_packed : upstream->n;
  activated = request->gating_strategy == PROM_M47_GATING_SEPARATE
                ? &slot->m47_activated_gate : &slot->m47_gate;
  hidden = request->gating_strategy == PROM_M47_GATING_FUSED_DIRECT_PACKED
             ? &slot->m47_gate : &slot->m47_hidden;
  hidden_input = reduced != 0u ? &slot->m47_hidden_packed : &slot->m47_hidden;
  output = request->residual_strategy == PROM_M47_RESIDUAL_IN_PLACE_DOWN
             ? &slot->m47_down : &slot->m47_output;
  prom_m47_update_descriptor(state, slot->m47_descriptor_sets[0], upstream->n,
                             upstream->n, upstream->n,
                             reduced != 0u ? &slot->m47_n_packed : upstream->n);
  prom_m47_update_descriptor(state, slot->m47_descriptor_sets[1], projection_input,
                             reduced != 0u ? &state->m47_weight_f16[PROM_M47_WEIGHT_GATE]
                                           : &state->m47_weight_f32[PROM_M47_WEIGHT_GATE],
                             &slot->m47_gate, &slot->m47_gate);
  prom_m47_update_descriptor(state, slot->m47_descriptor_sets[2], projection_input,
                             reduced != 0u ? &state->m47_weight_f16[PROM_M47_WEIGHT_UP]
                                           : &state->m47_weight_f32[PROM_M47_WEIGHT_UP],
                             &slot->m47_up, &slot->m47_up);
  prom_m47_update_descriptor(state, slot->m47_descriptor_sets[3], &slot->m47_gate,
                             &slot->m47_up, activated, hidden);
  prom_m47_update_descriptor(state, slot->m47_descriptor_sets[4],
                             request->gating_strategy == PROM_M47_GATING_FUSED_DIRECT_PACKED
                               ? &slot->m47_gate : hidden,
                             &slot->m47_up, hidden,
                             reduced != 0u ? &slot->m47_hidden_packed : hidden);
  prom_m47_update_descriptor(state, slot->m47_descriptor_sets[5], hidden_input,
                             reduced != 0u ? &state->m47_weight_f16[PROM_M47_WEIGHT_DOWN]
                                           : &state->m47_weight_f32[PROM_M47_WEIGHT_DOWN],
                             &slot->m47_down, &slot->m47_down);
  prom_m47_update_descriptor(state, slot->m47_descriptor_sets[6], upstream->n,
                             &slot->m47_down, output, output);
  continuation->n = upstream->n;
  continuation->output = output;
  result->logical_request_id = upstream->result->logical_request_id;
  result->physical_slot_id = slot->slot_id;
  result->physical_slot_generation = slot->generation;
  return 1;
}

static int prom_m47_record_tail(prom_reduction_runtime_state* state,
                                prom_reduction_slot* slot,
                                const prom_m47_composed_request* request,
                                const prom_m47_gated_ffn_plan* plan,
                                prom_vk_buffer* n,
                                prom_vk_buffer* output,
                                VkCommandBuffer command_buffer,
                                uint32_t already_open,
                                uint32_t n_already_shader_readable,
                                uint32_t down_projection_path_override,
                                uint32_t* out_partial_fault) {
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  const uint32_t reduced = plan->projection_path != PROM_M47_PROJECTION_A2X4_FP32;
  const uint32_t projection_m = reduced != 0u ? plan->padded_tokens : plan->tokens;
  const uint32_t gate_n = reduced != 0u ? plan->padded_ffn_width : plan->ffn_width;
  const uint32_t gate_k = reduced != 0u ? plan->padded_model_width : plan->model_width;
  const uint32_t down_path = down_projection_path_override != 0u
                                 ? down_projection_path_override : plan->projection_path;
  const uint32_t down_reduced = down_path != PROM_M47_PROJECTION_A2X4_FP32;
  prom_vk_buffer* down = plan->residual_strategy == PROM_M47_RESIDUAL_IN_PLACE_DOWN
                             ? output : &slot->m47_down;
  uint32_t partial_fault = 0u;
  uint32_t row;
  if (out_partial_fault != NULL) *out_partial_fault = 0u;
  if (already_open == 0u) {
    if (vkResetCommandBuffer(command_buffer, 0u) != VK_SUCCESS) return 0;
    memset(&begin_info, 0, sizeof(begin_info));
    begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
    begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
    if (vkBeginCommandBuffer(command_buffer, &begin_info) != VK_SUCCESS) return 0;
  }
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE)
    vkCmdResetQueryPool(command_buffer, state->query_pool,
                        slot->active_query_base + PROM_M47_QUERY_BASE,
                        PROM_M47_QUERY_COUNT);
  if (!prom_m43_one_buffer_barrier(command_buffer, n,
                                   (VkDeviceSize)plan->memory.n_view_bytes,
                                   n_already_shader_readable != 0u
                                       ? VK_ACCESS_SHADER_READ_BIT : VK_ACCESS_SHADER_WRITE_BIT,
                                   VK_ACCESS_SHADER_READ_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT)) return 0;
  if (reduced != 0u) {
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M47_QUERY_PACK_N_BEGIN);
    prom_m42_record_pack(state, command_buffer, slot->m47_descriptor_sets[0],
                         plan->tokens, plan->model_width, plan->n_row_stride,
                         plan->padded_tokens, plan->padded_model_width, 0u);
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M47_QUERY_PACK_N_END);
    prom_m42_buffer_barrier(command_buffer, &slot->m47_n_packed,
                            VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
  } else {
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M47_QUERY_PACK_N_BEGIN);
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M47_QUERY_PACK_N_END);
  }
  if (request->fault_point == PROM_M47_FAULT_BEFORE_GATE) partial_fault = request->fault_point;
  if (partial_fault == 0u) {
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M47_QUERY_GATE_BEGIN);
    prom_m42_record_sgemm(state, command_buffer, slot->m47_descriptor_sets[1],
                          plan->projection_path, projection_m, gate_n, gate_k);
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M47_QUERY_GATE_END);
    if (request->fault_point == PROM_M47_FAULT_BETWEEN_GATE_UP) partial_fault = request->fault_point;
  }
  if (partial_fault == 0u) {
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M47_QUERY_UP_BEGIN);
    prom_m42_record_sgemm(state, command_buffer, slot->m47_descriptor_sets[2],
                          plan->projection_path, projection_m, gate_n, gate_k);
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M47_QUERY_UP_END);
    if (request->fault_point == PROM_M47_FAULT_BEFORE_GATING) partial_fault = request->fault_point;
  }
  if (partial_fault == 0u) {
    prom_m42_buffer_barrier(command_buffer, &slot->m47_gate,
                            VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
    prom_m42_buffer_barrier(command_buffer, &slot->m47_up,
                            VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
    if (plan->gating_strategy == PROM_M47_GATING_FUSED_DIRECT_PACKED) {
      prom_m47_gate_pack_push_constants push;
      memset(&push, 0, sizeof(push));
      push.logical_rows = plan->tokens;
      push.logical_columns = plan->ffn_width;
      push.input_row_stride = plan->gate_row_stride;
      push.output_rows = plan->padded_tokens;
      push.output_columns = plan->padded_ffn_width;
      push.mode = 1u;
      push.packed_word_count = (uint32_t)(((uint64_t)push.output_rows * push.output_columns + 1u) / 2u);
      prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                               PROM_M47_QUERY_ACTIVATION_BEGIN);
      vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                        state->m47_pipelines[1].pipeline);
      vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                              state->pipeline_layout, 0u, 1u,
                              &slot->m47_descriptor_sets[4], 0u, NULL);
      vkCmdPushConstants(command_buffer, state->pipeline_layout, VK_SHADER_STAGE_COMPUTE_BIT,
                         0u, sizeof(push), &push);
      vkCmdDispatch(command_buffer, prom_reduction_ceil_div_u32(push.packed_word_count, 256u), 1u, 1u);
      prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                               PROM_M47_QUERY_ACTIVATION_END);
      prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                               PROM_M47_QUERY_MULTIPLY_BEGIN);
      prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                               PROM_M47_QUERY_MULTIPLY_END);
      if (request->fault_point == PROM_M47_FAULT_DURING_FUSED_GATING)
        partial_fault = request->fault_point;
    } else {
      prom_m47_gate_push_constants push;
      memset(&push, 0, sizeof(push));
      push.logical_rows = plan->tokens;
      push.logical_columns = plan->ffn_width;
      push.input_row_stride = plan->gate_row_stride;
      push.output_rows = reduced != 0u ? plan->padded_tokens : plan->tokens;
      push.output_columns = plan->gate_row_stride;
      push.element_count = push.output_rows * push.output_columns;
      vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                        state->m47_pipelines[0].pipeline);
      vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                              state->pipeline_layout, 0u, 1u,
                              &slot->m47_descriptor_sets[3], 0u, NULL);
      prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                               PROM_M47_QUERY_ACTIVATION_BEGIN);
      push.mode = plan->gating_strategy == PROM_M47_GATING_SEPARATE ? 1u : 3u;
      vkCmdPushConstants(command_buffer, state->pipeline_layout, VK_SHADER_STAGE_COMPUTE_BIT,
                         0u, sizeof(push), &push);
      vkCmdDispatch(command_buffer, prom_reduction_ceil_div_u32(push.element_count, 256u), 1u, 1u);
      prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                               PROM_M47_QUERY_ACTIVATION_END);
      if (plan->gating_strategy == PROM_M47_GATING_SEPARATE) {
        if (request->fault_point == PROM_M47_FAULT_DURING_ACTIVATION)
          partial_fault = request->fault_point;
        if (partial_fault == 0u) {
          prom_m42_buffer_barrier(command_buffer, &slot->m47_activated_gate,
                                  VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                                  VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                  VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
          push.mode = 2u;
          prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                   PROM_M47_QUERY_MULTIPLY_BEGIN);
          vkCmdPushConstants(command_buffer, state->pipeline_layout, VK_SHADER_STAGE_COMPUTE_BIT,
                             0u, sizeof(push), &push);
          vkCmdDispatch(command_buffer, prom_reduction_ceil_div_u32(push.element_count, 256u), 1u, 1u);
          prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                   PROM_M47_QUERY_MULTIPLY_END);
        }
      } else {
        prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                 PROM_M47_QUERY_MULTIPLY_BEGIN);
        prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                 PROM_M47_QUERY_MULTIPLY_END);
        if (request->fault_point == PROM_M47_FAULT_DURING_FUSED_GATING)
          partial_fault = request->fault_point;
      }
    }
  }
  if (partial_fault == 0u) {
    if (plan->gating_strategy == PROM_M47_GATING_FUSED_DIRECT_PACKED) {
      prom_m42_buffer_barrier(command_buffer, &slot->m47_hidden_packed,
                              VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                              VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                              VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
      prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                               PROM_M47_QUERY_HIDDEN_PACK_BEGIN);
      prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                               PROM_M47_QUERY_HIDDEN_PACK_END);
    } else if (down_reduced != 0u) {
      prom_m42_buffer_barrier(command_buffer, &slot->m47_hidden,
                              VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                              VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                              VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
      prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                               PROM_M47_QUERY_HIDDEN_PACK_BEGIN);
      prom_m42_record_pack(state, command_buffer, slot->m47_descriptor_sets[4],
                           plan->tokens, plan->ffn_width, plan->hidden_row_stride,
                           plan->padded_tokens, plan->padded_ffn_width, 0u);
      prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                               PROM_M47_QUERY_HIDDEN_PACK_END);
      prom_m42_buffer_barrier(command_buffer, &slot->m47_hidden_packed,
                              VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                              VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                              VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
    } else {
      prom_m42_buffer_barrier(command_buffer, &slot->m47_hidden,
                              VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                              VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                              VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
      prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                               PROM_M47_QUERY_HIDDEN_PACK_BEGIN);
      prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                               PROM_M47_QUERY_HIDDEN_PACK_END);
    }
    if (request->fault_point == PROM_M47_FAULT_AFTER_HIDDEN) partial_fault = request->fault_point;
  }
  if (partial_fault == 0u) {
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M47_QUERY_DOWN_BEGIN);
    prom_m42_record_sgemm(state, command_buffer, slot->m47_descriptor_sets[5],
                          down_path,
                          down_reduced != 0u ? plan->padded_tokens : plan->tokens,
                          down_reduced != 0u ? plan->padded_model_width : plan->model_width,
                          down_reduced != 0u ? plan->padded_ffn_width : plan->ffn_width);
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M47_QUERY_DOWN_END);
    if (request->fault_point == PROM_M47_FAULT_DURING_DOWN ||
        request->fault_point == PROM_M47_FAULT_BEFORE_RESIDUAL)
      partial_fault = request->fault_point;
  }
  if (partial_fault == 0u) {
    prom_m45_residual_push_constants push;
    prom_m42_buffer_barrier(command_buffer, down,
                            VK_ACCESS_SHADER_WRITE_BIT,
                            plan->residual_strategy == PROM_M47_RESIDUAL_IN_PLACE_DOWN
                              ? VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT
                              : VK_ACCESS_SHADER_READ_BIT,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
    prom_m42_buffer_barrier(command_buffer, n, VK_ACCESS_SHADER_READ_BIT,
                            VK_ACCESS_SHADER_READ_BIT,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
    memset(&push, 0, sizeof(push));
    push.tokens = plan->tokens;
    push.model_width = plan->model_width;
    push.x_row_stride = plan->n_row_stride;
    push.y_row_stride = plan->down_row_stride;
    push.z_row_stride = plan->output_row_stride;
    push.logical_element_count = plan->tokens * plan->model_width;
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M47_QUERY_RESIDUAL_BEGIN);
    vkCmdBindPipeline(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                      state->m45_pipelines[0].pipeline);
    vkCmdBindDescriptorSets(command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE,
                            state->pipeline_layout, 0u, 1u,
                            &slot->m47_descriptor_sets[6], 0u, NULL);
    vkCmdPushConstants(command_buffer, state->pipeline_layout, VK_SHADER_STAGE_COMPUTE_BIT,
                       0u, sizeof(push), &push);
    vkCmdDispatch(command_buffer, prom_reduction_ceil_div_u32(push.logical_element_count, 256u), 1u, 1u);
    prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             PROM_M47_QUERY_RESIDUAL_END);
  }
  if (partial_fault == 0u && request->output != NULL) {
    if (request->fault_point == PROM_M47_FAULT_BEFORE_FINAL_READBACK)
      partial_fault = request->fault_point;
    if (partial_fault == 0u) {
      prom_m42_buffer_barrier(command_buffer, output,
                              VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                              VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                              VK_PIPELINE_STAGE_TRANSFER_BIT);
      prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                               PROM_M47_QUERY_READBACK_BEGIN);
      for (row = 0u; row < plan->tokens; ++row) {
        memset(&copy, 0, sizeof(copy));
        copy.srcOffset = (VkDeviceSize)((uint64_t)row * plan->output_row_stride * sizeof(float));
        copy.dstOffset = (VkDeviceSize)((uint64_t)row * plan->model_width * sizeof(float));
        copy.size = (VkDeviceSize)((uint64_t)plan->model_width * sizeof(float));
        vkCmdCopyBuffer(command_buffer, output->buffer, slot->m47_readback.buffer, 1u, &copy);
      }
      prom_m42_write_timestamp(state, slot, command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                               PROM_M47_QUERY_READBACK_END);
      prom_m42_buffer_barrier(command_buffer, &slot->m47_readback,
                              VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_HOST_READ_BIT,
                              VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_HOST_BIT);
    }
  } else {
    prom_m42_buffer_barrier(command_buffer, output,
                            VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
  }
  if (out_partial_fault != NULL) *out_partial_fault = partial_fault;
  return partial_fault != 0u ? 2 : 1;
}

static int prom_m47_complete_continuation(prom_reduction_runtime_state* state,
                                          prom_reduction_slot* slot,
                                          prom_m47_continuation* continuation,
                                          const uint64_t* timestamps,
                                          uint64_t begin_ns) {
  prom_m47_composed_result* result = continuation->result;
  const prom_m47_composed_request* request = continuation->request;
  const prom_m47_gated_ffn_plan* plan = &result->ffn_plan;
  uint64_t readback_begin;
#define PROM_M47_DURATION(first, last) \
  ((uint64_t)((double)(timestamps[(last)] - timestamps[(first)]) * state->timestamp_period_ns))
  result->n_pack_gpu_ns = PROM_M47_DURATION(PROM_M47_QUERY_PACK_N_BEGIN, PROM_M47_QUERY_PACK_N_END);
  result->gate_projection_gpu_ns = PROM_M47_DURATION(PROM_M47_QUERY_GATE_BEGIN, PROM_M47_QUERY_GATE_END);
  result->up_projection_gpu_ns = PROM_M47_DURATION(PROM_M47_QUERY_UP_BEGIN, PROM_M47_QUERY_UP_END);
  result->activation_gpu_ns = PROM_M47_DURATION(PROM_M47_QUERY_ACTIVATION_BEGIN,
                                                PROM_M47_QUERY_ACTIVATION_END);
  result->gating_multiply_gpu_ns = PROM_M47_DURATION(PROM_M47_QUERY_MULTIPLY_BEGIN,
                                                     PROM_M47_QUERY_MULTIPLY_END);
  result->fused_gating_gpu_ns = plan->gating_strategy == PROM_M47_GATING_SEPARATE
                                  ? 0u : PROM_M47_DURATION(PROM_M47_QUERY_ACTIVATION_BEGIN,
                                                           PROM_M47_QUERY_ACTIVATION_END);
  result->hidden_pack_gpu_ns = PROM_M47_DURATION(PROM_M47_QUERY_HIDDEN_PACK_BEGIN,
                                                 PROM_M47_QUERY_HIDDEN_PACK_END);
  result->down_projection_gpu_ns = PROM_M47_DURATION(PROM_M47_QUERY_DOWN_BEGIN,
                                                     PROM_M47_QUERY_DOWN_END);
  result->residual_gpu_ns = PROM_M47_DURATION(PROM_M47_QUERY_RESIDUAL_BEGIN,
                                              PROM_M47_QUERY_RESIDUAL_END);
  result->m47_gpu_ns = PROM_M47_DURATION(PROM_M47_QUERY_GATE_BEGIN, PROM_M47_QUERY_RESIDUAL_END);
  result->total_m43_m44_m45_m46_m47_gpu_ns =
      PROM_M47_DURATION(3u, PROM_M47_QUERY_RESIDUAL_END);
#undef PROM_M47_DURATION
  if (request->output != NULL) {
    readback_begin = prom_reduction_now_ns();
    memcpy(request->output, slot->m47_readback.mapped,
           (size_t)plan->memory.final_readback_bytes);
    result->final_readback_ns =
        (uint64_t)((double)(timestamps[PROM_M47_QUERY_READBACK_END] -
                           timestamps[PROM_M47_QUERY_READBACK_BEGIN]) * state->timestamp_period_ns) +
        prom_reduction_elapsed_ns(readback_begin, prom_reduction_now_ns());
  }
  memset(&result->output_view, 0, sizeof(result->output_view));
  result->output_view.buffer = continuation->output->buffer;
  result->output_view.byte_length = continuation->output->size;
  result->output_view.element_type = PROM_DEVICE_ELEMENT_F32;
  result->output_view.logical_rows = plan->tokens;
  result->output_view.logical_columns = plan->model_width;
  result->output_view.row_stride_elements = plan->output_row_stride;
  result->output_view.layout = PROM_DEVICE_LAYOUT_ROW_MAJOR;
  result->output_view.producer_access = request->output != NULL
                                         ? PROM_DEVICE_ACCESS_TRANSFER_READ
                                         : PROM_DEVICE_ACCESS_COMPUTE_WRITE;
  result->output_view.required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
  result->output_view.owning_device = state->device;
  result->output_view.owning_lifetime_id = plan->output_generation;
  result->output_view.owning_slot_id = slot->slot_id;
  result->output_view.owning_slot_generation = slot->generation;
  result->submit_count = plan->submit_count;
  result->final_readback_count = request->output != NULL ? 1u : 0u;
  result->no_intermediate_host_copy = 1u;
  result->n_generation = plan->n_generation;
  memcpy(result->weight_generation, state->m47_weight_generation,
         sizeof(result->weight_generation));
  result->output_generation = plan->output_generation;
  result->exact_request_bytes = plan->memory.exact_request_bytes;
  result->retained_bytes = result->upstream.retained_bytes;
  {
    uint32_t weight;
    for (weight = 0u; weight < PROM_M47_WEIGHT_COUNT; ++weight)
      result->retained_bytes += (uint64_t)state->m47_weight_upload[weight].size +
                                (uint64_t)state->m47_weight_f32[weight].size +
                                (uint64_t)state->m47_weight_f16[weight].size;
  }
  result->retained_bytes += (uint64_t)slot->m47_n_packed.size +
                            (uint64_t)slot->m47_gate.size +
                            (uint64_t)slot->m47_up.size +
                            (uint64_t)slot->m47_activated_gate.size +
                            (uint64_t)slot->m47_hidden.size +
                            (uint64_t)slot->m47_hidden_packed.size +
                            (uint64_t)slot->m47_down.size +
                            (uint64_t)slot->m47_output.size +
                            (uint64_t)slot->m47_readback.size;
  result->buffer_allocation_count = result->upstream.buffer_allocation_count +
                                    state->m47_buffer_grow_count;
  result->buffer_reuse_count = result->upstream.buffer_reuse_count + state->m47_buffer_reuse_count;
  result->descriptor_update_count = result->upstream.descriptor_update_count +
                                    state->m47_descriptor_update_count;
  result->pipeline_create_count = result->upstream.pipeline_create_count +
                                  state->m47_pipeline_create_count;
  result->command_buffer_reuse_count = slot->m47_command_reuse_count;
  result->cpu_recording_ns = result->upstream.cpu_recording_ns;
  result->cpu_submission_ns = result->upstream.cpu_submission_ns;
  result->end_to_end_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  result->stage = 0u;
  result->detail_code = 0;
  return 1;
}

static int prom_m44_fill_m43_timings(const prom_reduction_runtime_state* state,
                                     const uint64_t* timestamps,
                                     prom_m43_attention_group_result* result) {
  uint32_t head;
  result->shared_x_upload_gpu_ns =
      (uint64_t)((double)(timestamps[1u] - timestamps[0u]) * state->timestamp_period_ns);
  result->shared_x_pack_gpu_ns =
      (uint64_t)((double)(timestamps[2u] - timestamps[1u]) * state->timestamp_period_ns);
  result->grouped_attention_gpu_ns =
      (uint64_t)((double)(timestamps[PROM_M43_QUERY_GROUP_END] - timestamps[3u]) *
                 state->timestamp_period_ns);
  for (head = 0u; head < PROM_M44_HEAD_COUNT; ++head) {
    const uint32_t base = PROM_M43_QUERY_HEAD_BASE + head * PROM_M43_QUERY_HEAD_STRIDE;
    uint64_t durations[11];
    uint32_t stage;
    for (stage = 0u; stage < 11u; ++stage) {
      const uint32_t first = base + stage * 2u;
      if (timestamps[first + 1u] < timestamps[first]) return 0;
      durations[stage] =
          (uint64_t)((double)(timestamps[first + 1u] - timestamps[first]) * state->timestamp_period_ns);
    }
    result->q_projection_gpu_ns[head] = durations[0];
    result->k_projection_gpu_ns[head] = durations[1];
    result->v_projection_gpu_ns[head] = durations[2];
    result->q_pack_gpu_ns[head] = durations[3];
    result->k_layout_gpu_ns[head] = durations[4];
    result->v_pack_gpu_ns[head] = durations[5];
    result->qk_gpu_ns[head] = durations[6];
    result->scale_gpu_ns[head] = durations[7];
    result->softmax_gpu_ns[head] = durations[8];
    result->p_pack_gpu_ns[head] = durations[9];
    result->pv_gpu_ns[head] = durations[10];
    result->q_projection_total_gpu_ns += durations[0];
    result->k_projection_total_gpu_ns += durations[1];
    result->v_projection_total_gpu_ns += durations[2];
    result->q_pack_total_gpu_ns += durations[3];
    result->k_layout_total_gpu_ns += durations[4];
    result->v_pack_total_gpu_ns += durations[5];
    result->qk_total_gpu_ns += durations[6];
    result->scale_total_gpu_ns += durations[7];
    result->softmax_total_gpu_ns += durations[8];
    result->p_pack_total_gpu_ns += durations[9];
    result->pv_total_gpu_ns += durations[10];
    for (stage = 3u; stage < 11u; ++stage) result->post_projection_total_gpu_ns += durations[stage];
  }
  result->projection_total_gpu_ns = result->q_projection_total_gpu_ns +
                                    result->k_projection_total_gpu_ns +
                                    result->v_projection_total_gpu_ns;
  return 1;
}

static int prom_m44_strip_m43_final_readback(prom_m43_attention_plan* plan) {
  prom_m43_stage_plan* stage;
  uint64_t command_hash = 1469598103934665603ull;
  uint64_t aggregate_hash = 1469598103934665603ull;
  uint32_t index;
  if (plan == NULL || plan->stage_count == 0u) return 0;
  stage = &plan->stages[plan->stage_count - 1u];
  if (stage->operation != PROM_M42_STAGE_FINAL_READBACK ||
      plan->barrier_call_count < stage->barrier_call_count ||
      plan->barrier_buffer_count < stage->barrier_buffer_count ||
      plan->copy_region_count < stage->copy_region_count) return 0;
  plan->barrier_call_count -= stage->barrier_call_count;
  plan->barrier_buffer_count -= stage->barrier_buffer_count;
  plan->copy_region_count -= stage->copy_region_count;
  plan->stage_count -= 1u;
  plan->final_readback_count = 0u;
  memset(&plan->stages[plan->stage_count], 0, sizeof(plan->stages[plan->stage_count]));
  for (index = 0u; index < plan->stage_count; ++index) {
    stage = &plan->stages[index];
    command_hash = prom_reduction_hash_u32(command_hash, stage->sequence);
    command_hash = prom_reduction_hash_u32(command_hash, stage->head_index);
    command_hash = prom_reduction_hash_u32(command_hash, stage->operation);
    command_hash = prom_reduction_hash_u32(command_hash, stage->selected_path);
    command_hash = prom_reduction_hash_u32(command_hash, stage->dispatch_count);
    command_hash = prom_reduction_hash_u32(command_hash, stage->barrier_call_count);
    command_hash = prom_reduction_hash_u32(command_hash, stage->barrier_buffer_count);
    command_hash = prom_reduction_hash_u32(command_hash, stage->copy_region_count);
    command_hash = prom_reduction_hash_u32(command_hash, stage->descriptor_index);
    command_hash = prom_reduction_hash_u32(command_hash, stage->timestamp_begin);
    command_hash = prom_reduction_hash_u32(command_hash, stage->timestamp_end);
  }
  plan->command_plan_replay_id = command_hash;
  aggregate_hash = prom_reduction_hash_u32(aggregate_hash, plan->head_count);
  aggregate_hash = prom_reduction_hash_u32(aggregate_hash, plan->tokens);
  aggregate_hash = prom_reduction_hash_u32(aggregate_hash, plan->model_width);
  aggregate_hash = prom_reduction_hash_u32(aggregate_hash, plan->head_dim);
  aggregate_hash = prom_reduction_hash_u32(aggregate_hash, plan->precision_policy);
  aggregate_hash = prom_reduction_hash_u32(aggregate_hash, plan->input_mode);
  aggregate_hash = prom_reduction_hash_u32(aggregate_hash, plan->execution_strategy);
  aggregate_hash = prom_m40b_hash_u64(aggregate_hash, plan->shared_x_generation);
  aggregate_hash = prom_m40b_hash_u64(aggregate_hash, plan->shared_x_hash);
  for (index = 0u; index < PROM_M44_HEAD_COUNT; ++index)
    aggregate_hash = prom_m40b_hash_u64(aggregate_hash, plan->head_replay_id[index]);
  aggregate_hash = prom_m40b_hash_u64(aggregate_hash, command_hash);
  aggregate_hash = prom_m40b_hash_u64(aggregate_hash, plan->memory.exact_retained_bytes);
  plan->aggregate_replay_id = aggregate_hash;
  return 1;
}

static int prom_m45_strip_m44_final_readback(prom_m44_output_projection_plan* plan) {
  prom_m44_stage_plan* stage;
  uint64_t command_hash;
  uint64_t replay_hash;
  if (plan == NULL || plan->stage_count == 0u) return 0;
  stage = &plan->stages[plan->stage_count - 1u];
  if (stage->operation != PROM_M44_STAGE_FINAL_READBACK ||
      plan->barrier_call_count < stage->barrier_call_count ||
      plan->barrier_buffer_count < stage->barrier_buffer_count ||
      plan->copy_region_count < stage->copy_region_count ||
      plan->memory.exact_request_bytes < plan->memory.final_readback_bytes) return 0;
  plan->barrier_call_count -= stage->barrier_call_count;
  plan->barrier_buffer_count -= stage->barrier_buffer_count;
  plan->copy_region_count -= stage->copy_region_count;
  plan->stage_count -= 1u;
  plan->final_readback_count = 0u;
  plan->memory.exact_request_bytes -= plan->memory.final_readback_bytes;
  plan->memory.final_readback_bytes = 0u;
  memset(&plan->stages[plan->stage_count], 0, sizeof(plan->stages[plan->stage_count]));
  command_hash = prom_m40b_hash_u64(plan->command_plan_replay_id, 0x4d34352d6e6f7262ull);
  command_hash = prom_reduction_hash_u32(command_hash, plan->stage_count);
  command_hash = prom_reduction_hash_u32(command_hash, plan->barrier_call_count);
  command_hash = prom_reduction_hash_u32(command_hash, plan->copy_region_count);
  plan->command_plan_replay_id = command_hash;
  replay_hash = prom_m40b_hash_u64(plan->replay_id, 0x4d34352d72657369ull);
  replay_hash = prom_m40b_hash_u64(replay_hash, command_hash);
  plan->replay_id = replay_hash;
  return 1;
}

int prom_reactor_runtime_m44_execute_composed(void* handle,
                                              const prom_m44_composed_request* request,
                                              prom_m44_composed_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_reduction_slot* slot;
  prom_vk_runtime_services services_before;
  prom_vk_runtime_services services_after;
  prom_m44_composed_request effective_request;
  prom_m43_plan_request m43_plan_request;
  prom_m44_plan_request m44_plan_request;
  PrometheusReductionRequest reduction_request;
  PrometheusReductionPlan reduction_plan;
  const prom_vk_buffer* x_f32;
  const prom_vk_buffer* x_f16;
  uint64_t timestamps[PROM_M44_TOTAL_QUERY_COUNT];
  uint64_t begin_ns = prom_reduction_now_ns();
  uint64_t validation_begin;
  uint64_t recording_begin;
  uint64_t submission_begin;
  uint64_t readback_begin;
  uint64_t readback_cpu_ns;
  uint64_t shared_x_hash;
  uint64_t x_elements;
  uint64_t head_output_elements;
  uint64_t final_output_elements;
  uint32_t finite = 0u;
  uint32_t head;
  uint32_t weight;
  uint32_t partial_fault = 0u;
  uint32_t m43_partial_fault = 0u;
  uint32_t m43_uncertain_fault = 0u;
  uint32_t wait_stage = VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT;
  int m43_record_status;
  int m44_record_status;
  int32_t detail = 0;
  VkSubmitInfo submits[2];
  VkResult result;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  out_result->physical_slot_id = UINT32_MAX;
  if (request == NULL || request->output == NULL ||
      request->attention.head_count != PROM_M44_HEAD_COUNT ||
      request->attention.execution_strategy == PROM_M43_STRATEGY_EIGHT_SEQUENTIAL_M42 ||
      request->attention.fault_point != PROM_M43_FAULT_NONE ||
      request->fault_point > PROM_M44_FAULT_UNCERTAIN_COMPLETION ||
      request->aggregation_strategy < PROM_M44_AGGREGATION_INTERLEAVE ||
      request->aggregation_strategy > PROM_M44_AGGREGATION_DIRECT_SEGMENTED ||
      request->projection_path < PROM_M44_PROJECTION_COOPERATIVE ||
      request->projection_path > PROM_M44_PROJECTION_DIRECT_SEGMENTED_FP16 ||
      request->submit_plan < PROM_M44_SUBMIT_ONE_COMMAND_BUFFER ||
      request->submit_plan > PROM_M44_SUBMIT_TWO_BOUNDED ||
      request->required_wo_generation == 0u ||
      (request->attention.input_mode == PROM_M42_INPUT_HOST_X && request->attention.host_x == NULL)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = request != NULL && request->attention.head_count != PROM_M44_HEAD_COUNT
                                  ? PROM_M44_DETAIL_HEAD_COUNT
                                  : PROM_M44_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  if (!prom_m40b_checked_product_u64(request->attention.tokens,
                                     request->attention.model_width, &x_elements) ||
      !prom_m40b_checked_product_u64(request->attention.tokens,
                                     request->attention.head_dim, &head_output_elements) ||
      !prom_m43_checked_scale_u64(head_output_elements, PROM_M44_HEAD_COUNT,
                                  &head_output_elements) ||
      !prom_m40b_checked_product_u64(request->attention.tokens,
                                     request->attention.model_width, &final_output_elements)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M44_DETAIL_SIZE_OVERFLOW;
    return PROM_ERROR;
  }
  if (request->attention.output_element_count != head_output_elements ||
      request->output_element_count != final_output_elements ||
      (request->attention.input_mode == PROM_M42_INPUT_HOST_X &&
       request->attention.host_x_element_count != x_elements) ||
      (request->attention.input_mode == PROM_M42_INPUT_RESIDENT_X &&
       request->attention.host_x_element_count != 0u)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M44_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL || prom_reactor_runtime_get_vk_services(handle, &services_before) != PROM_OK) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = state == NULL ? detail : PROM_M44_DETAIL_CAPABILITY;
    return PROM_ERROR;
  }
  if (state->m44_wo_generation == 0u ||
      request->required_wo_generation != state->m44_wo_generation ||
      state->m44_wo_head_dim != request->attention.head_dim ||
      state->m44_wo_model_width != request->attention.model_width) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M44_DETAIL_STALE_WO_GENERATION;
    return PROM_ERROR;
  }
  effective_request = *request;
  if (effective_request.projection_path == PROM_M44_PROJECTION_COOPERATIVE &&
      (services_before.cooperative_matrix_feature_enabled == 0u || services_before.subgroup_size != 32u ||
       effective_request.rollback_active != 0u)) {
    if (effective_request.attention.allow_fallback == 0u) {
      out_result->stage = PROM_STAGE_INIT;
      out_result->detail_code = PROM_M44_DETAIL_CAPABILITY;
      return PROM_ERROR;
    }
    effective_request.projection_path = PROM_M44_PROJECTION_CONVENTIONAL_FP16;
  }
  validation_begin = prom_reduction_now_ns();
  if (effective_request.attention.input_mode == PROM_M42_INPUT_HOST_X) {
    shared_x_hash = prom_m42_hash_finite_matrix(effective_request.attention.host_x,
                                                x_elements, &finite);
    if (finite == 0u) {
      out_result->stage = PROM_STAGE_TRANSFER_IN;
      out_result->detail_code = PROM_M44_DETAIL_NONFINITE_INPUT;
      return PROM_ERROR;
    }
  } else {
    if (state->m43_resident_x_generation == 0u ||
        effective_request.attention.shared_x_generation != state->m43_resident_x_generation ||
        state->m43_resident_x_tokens != effective_request.attention.tokens ||
        state->m43_resident_x_model_width != effective_request.attention.model_width) {
      out_result->stage = PROM_STAGE_INIT;
      out_result->detail_code = PROM_M43_DETAIL_STALE_X_GENERATION;
      return PROM_ERROR;
    }
    shared_x_hash = state->m43_resident_x_hash;
  }
  out_result->attention.shared_x_validation_ns =
      prom_reduction_elapsed_ns(validation_begin, prom_reduction_now_ns());
  memset(&m43_plan_request, 0, sizeof(m43_plan_request));
  m43_plan_request.head_count = PROM_M44_HEAD_COUNT;
  m43_plan_request.tokens = effective_request.attention.tokens;
  m43_plan_request.model_width = effective_request.attention.model_width;
  m43_plan_request.head_dim = effective_request.attention.head_dim;
  m43_plan_request.scale = effective_request.attention.scale;
  m43_plan_request.scale_explicit = effective_request.attention.scale_explicit;
  m43_plan_request.precision_policy = effective_request.attention.precision_policy;
  m43_plan_request.allow_fallback = effective_request.attention.allow_fallback;
  m43_plan_request.input_mode = effective_request.attention.input_mode;
  m43_plan_request.execution_strategy = effective_request.attention.execution_strategy;
  m43_plan_request.cooperative_capability_state = services_before.cooperative_matrix_state;
  m43_plan_request.shared_x_generation = effective_request.attention.shared_x_generation;
  m43_plan_request.shared_x_hash = shared_x_hash;
  for (head = 0u; head < PROM_M44_HEAD_COUNT; ++head) {
    m43_plan_request.preferred_path[head] = effective_request.attention.preferred_path[head];
    m43_plan_request.rollback_active[head] = effective_request.attention.rollback_active[head];
    for (weight = 0u; weight < PROM_M43_WEIGHT_KIND_COUNT; ++weight) {
      if (state->m43_weight_generation[head][weight] == 0u ||
          effective_request.attention.required_weight_generation[head][weight] !=
              state->m43_weight_generation[head][weight] ||
          state->m43_weight_model_width[head][weight] != effective_request.attention.model_width ||
          state->m43_weight_head_dim[head][weight] != effective_request.attention.head_dim) {
        out_result->stage = PROM_STAGE_INIT;
        out_result->detail_code = PROM_M43_DETAIL_STALE_WEIGHT_GENERATION;
        return PROM_ERROR;
      }
      m43_plan_request.weight_generation[head][weight] = state->m43_weight_generation[head][weight];
      m43_plan_request.weight_hash[head][weight] = state->m43_weight_hash[head][weight];
    }
  }
  if (prom_m43_attention_plan_build(&m43_plan_request, &out_result->attention.plan) != PROM_OK ||
      !prom_m44_strip_m43_final_readback(&out_result->attention.plan)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M44_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  if (!prom_m42_ensure_pipelines(state) || !prom_m44_ensure_pipelines(state)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M44_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  for (head = 0u; head < PROM_M44_HEAD_COUNT; ++head) {
    if (!prom_m40b_ensure_sgemm_pipeline(state, out_result->attention.plan.selected_path[head])) {
      out_result->stage = PROM_STAGE_INIT;
      out_result->detail_code = PROM_M44_DETAIL_RESOURCE;
      return PROM_ERROR;
    }
  }
  if (effective_request.projection_path <= PROM_M44_PROJECTION_CONVENTIONAL_FP16 &&
      !prom_m40b_ensure_sgemm_pipeline(state, effective_request.projection_path)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M44_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  memset(&reduction_request, 0, sizeof(reduction_request));
  reduction_request.struct_size = sizeof(reduction_request);
  reduction_request.row_count = effective_request.attention.tokens;
  reduction_request.elements_per_row = effective_request.attention.tokens;
  reduction_request.input_element_count =
      (uint64_t)effective_request.attention.tokens * effective_request.attention.tokens;
  reduction_request.output_element_count = reduction_request.input_element_count;
  reduction_request.operation = PROM_REDUCTION_OPERATION_SOFTMAX;
  reduction_request.finalization = PROM_REDUCTION_FINALIZATION_STABLE_SOFTMAX;
  if (prom_reactor_reduction_plan_impl(&reduction_request, &reduction_plan) != PROM_OK) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M44_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  out_result->logical_request_id = state->next_logical_request_id++;
  state->diagnostics.next_logical_request_id = state->next_logical_request_id;
  out_result->attention.logical_request_id = out_result->logical_request_id;
  slot = prom_reduction_acquire_slot(state, out_result->logical_request_id);
  if (slot == NULL) {
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M44_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  out_result->physical_slot_id = slot->slot_id;
  out_result->physical_slot_generation = slot->generation;
  out_result->attention.physical_slot_id = slot->slot_id;
  out_result->attention.physical_slot_generation = slot->generation;
  if (!prom_m43_prepare_execution_buffers(state, slot, &effective_request.attention,
                                           &out_result->attention.plan, 0u)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M44_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  if (effective_request.attention.input_mode == PROM_M42_INPUT_HOST_X) {
    memcpy(slot->m43_x_upload.mapped, effective_request.attention.host_x,
           (size_t)(x_elements * sizeof(float)));
    x_f32 = &slot->m43_x_f32;
    x_f16 = &slot->m43_x_f16;
  } else {
    x_f32 = &state->m43_resident_x_f32;
    x_f16 = &state->m43_resident_x_f16;
  }
  if (!prom_m43_setup_descriptors(state, slot, &effective_request.attention,
                                  &out_result->attention, x_f32, x_f16)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M44_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  memset(&m44_plan_request, 0, sizeof(m44_plan_request));
  memcpy(m44_plan_request.head_views, out_result->attention.head_output_view,
         sizeof(m44_plan_request.head_views));
  for (head = 0u; head < PROM_M44_HEAD_COUNT; ++head) {
    m44_plan_request.head_views[head].required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
    out_result->attention.head_output_view[head].required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
  }
  m44_plan_request.head_count = PROM_M44_HEAD_COUNT;
  m44_plan_request.tokens = effective_request.attention.tokens;
  m44_plan_request.head_dim = effective_request.attention.head_dim;
  m44_plan_request.model_width = effective_request.attention.model_width;
  m44_plan_request.precision_policy =
      effective_request.projection_path == PROM_M44_PROJECTION_A2X4_FP32
          ? PROM_M42_PRECISION_FP32
          : PROM_M42_PRECISION_F16_ROUNDED;
  m44_plan_request.aggregation_strategy = effective_request.aggregation_strategy;
  m44_plan_request.projection_path = effective_request.projection_path;
  m44_plan_request.submit_plan = effective_request.submit_plan;
  m44_plan_request.cooperative_capability_state = services_before.cooperative_matrix_state;
  m44_plan_request.rollback_active = request->rollback_active;
  m44_plan_request.wo_generation = state->m44_wo_generation;
  m44_plan_request.wo_hash = state->m44_wo_hash;
  m44_plan_request.m43_aggregate_replay_id = out_result->attention.plan.aggregate_replay_id;
  if (prom_m44_output_projection_plan_build(&m44_plan_request, &out_result->plan) != PROM_OK ||
      out_result->plan.memory.exact_request_bytes > out_result->plan.memory.capacity_limit_bytes ||
      !prom_m44_prepare_execution_buffers(state, slot, &out_result->plan, 0u, 1u) ||
      !prom_m44_setup_descriptors(state, slot, &out_result->plan)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M44_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  memset(&out_result->output_view, 0, sizeof(out_result->output_view));
  out_result->output_view.buffer = slot->m44_output.buffer;
  out_result->output_view.byte_length = slot->m44_output.size;
  out_result->output_view.element_type = PROM_DEVICE_ELEMENT_F32;
  out_result->output_view.logical_rows = out_result->plan.tokens;
  out_result->output_view.logical_columns = out_result->plan.model_width;
  out_result->output_view.row_stride_elements = out_result->plan.output_row_stride;
  out_result->output_view.layout = PROM_DEVICE_LAYOUT_ROW_MAJOR;
  out_result->output_view.producer_access = PROM_DEVICE_ACCESS_TRANSFER_READ;
  out_result->output_view.required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
  out_result->output_view.owning_device = state->device;
  out_result->output_view.owning_lifetime_id = out_result->logical_request_id;
  out_result->output_view.owning_slot_id = slot->slot_id;
  out_result->output_view.owning_slot_generation = slot->generation;
  recording_begin = prom_reduction_now_ns();
  m43_record_status = prom_m43_record_grouped_internal(
      state, slot, &effective_request.attention, &out_result->attention.plan, &reduction_plan,
      &m43_partial_fault, &m43_uncertain_fault, 0u,
      effective_request.submit_plan == PROM_M44_SUBMIT_ONE_COMMAND_BUFFER,
      VK_NULL_HANDLE, NULL, 0u, 0u);
  if (m43_record_status != 1 || m43_partial_fault != 0u || m43_uncertain_fault != 0u) {
    if (m43_record_status == 1) (void)vkEndCommandBuffer(slot->command_buffer);
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M44_DETAIL_COMMAND;
    return PROM_ERROR;
  }
  m44_record_status = prom_m44_record_projection_tail(
      state, slot, &effective_request, &out_result->plan,
      effective_request.submit_plan == PROM_M44_SUBMIT_ONE_COMMAND_BUFFER
          ? slot->command_buffer
          : slot->consumer_command_buffer,
      effective_request.submit_plan == PROM_M44_SUBMIT_ONE_COMMAND_BUFFER,
      1u,
      &partial_fault);
  if (m44_record_status == 0 ||
      vkEndCommandBuffer(effective_request.submit_plan == PROM_M44_SUBMIT_ONE_COMMAND_BUFFER
                             ? slot->command_buffer
                             : slot->consumer_command_buffer) != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M44_DETAIL_COMMAND;
    return PROM_ERROR;
  }
  out_result->cpu_recording_ns = prom_reduction_elapsed_ns(recording_begin, prom_reduction_now_ns());
  slot->m44_command_reuse_count += 1u;
  if (vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M44_DETAIL_SUBMIT;
    return PROM_ERROR;
  }
  memset(submits, 0, sizeof(submits));
  submits[0].sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submits[0].commandBufferCount = 1u;
  submits[0].pCommandBuffers = &slot->command_buffer;
  if (effective_request.submit_plan == PROM_M44_SUBMIT_TWO_BOUNDED) {
    submits[0].signalSemaphoreCount = 1u;
    submits[0].pSignalSemaphores = &slot->producer_complete;
    submits[1].sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
    submits[1].waitSemaphoreCount = 1u;
    submits[1].pWaitSemaphores = &slot->producer_complete;
    submits[1].pWaitDstStageMask = &wait_stage;
    submits[1].commandBufferCount = 1u;
    submits[1].pCommandBuffers = &slot->consumer_command_buffer;
  }
  submission_begin = prom_reduction_now_ns();
  result = vkQueueSubmit(state->queue,
                         effective_request.submit_plan == PROM_M44_SUBMIT_TWO_BOUNDED ? 2u : 1u,
                         submits, slot->fence);
  out_result->cpu_submission_ns = prom_reduction_elapsed_ns(submission_begin, prom_reduction_now_ns());
  if (result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M44_DETAIL_SUBMIT;
    return PROM_ERROR;
  }
  slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
  if (request->fault_point == PROM_M44_FAULT_UNCERTAIN_COMPLETION) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    out_result->stage = PROM_M44_STAGE_OUTPUT_PROJECTION;
    out_result->detail_code = PROM_M44_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
  if (result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M44_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  if (partial_fault != 0u || request->fault_point == PROM_M44_FAULT_AFTER_PROJECTION_SUBMIT) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = partial_fault != 0u ? partial_fault : PROM_M44_STAGE_OUTPUT_PROJECTION;
    out_result->detail_code = PROM_M44_DETAIL_FAULT_INJECTED;
    return PROM_ERROR;
  }
  memset(timestamps, 0, sizeof(timestamps));
  if (state->timestamp_supported == 0u || state->query_pool == VK_NULL_HANDLE ||
      vkGetQueryPoolResults(state->device, state->query_pool,
                            slot->slot_id * PROM_REDUCTION_QUERY_STRIDE,
                            PROM_M43_QUERY_GROUP_END + 1u,
                            sizeof(uint64_t) * (PROM_M43_QUERY_GROUP_END + 1u), timestamps,
                            sizeof(uint64_t), VK_QUERY_RESULT_64_BIT) != VK_SUCCESS ||
      vkGetQueryPoolResults(state->device, state->query_pool,
                            slot->slot_id * PROM_REDUCTION_QUERY_STRIDE + PROM_M44_QUERY_BASE,
                            PROM_M44_QUERY_COUNT, sizeof(uint64_t) * PROM_M44_QUERY_COUNT,
                            timestamps + PROM_M44_QUERY_BASE,
                            sizeof(uint64_t), VK_QUERY_RESULT_64_BIT) != VK_SUCCESS ||
      timestamps[PROM_M43_QUERY_GROUP_END] < timestamps[3u] ||
      timestamps[PROM_M44_QUERY_AGGREGATION_END] < timestamps[PROM_M44_QUERY_AGGREGATION_BEGIN] ||
      timestamps[PROM_M44_QUERY_PROJECTION_END] < timestamps[PROM_M44_QUERY_PROJECTION_BEGIN] ||
      timestamps[PROM_M44_QUERY_ACCUMULATION_END] < timestamps[PROM_M44_QUERY_ACCUMULATION_BEGIN] ||
      timestamps[PROM_M44_QUERY_READBACK_END] < timestamps[PROM_M44_QUERY_READBACK_BEGIN] ||
      !prom_m44_fill_m43_timings(state, timestamps, &out_result->attention)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_TRANSFER_OUT;
    out_result->detail_code = PROM_M44_DETAIL_QUERY;
    return PROM_ERROR;
  }
#define PROM_M44_DURATION(BEGIN, END) \
  ((uint64_t)((double)(timestamps[(END)] - timestamps[(BEGIN)]) * state->timestamp_period_ns))
  out_result->aggregation_gpu_ns =
      PROM_M44_DURATION(PROM_M44_QUERY_AGGREGATION_BEGIN, PROM_M44_QUERY_AGGREGATION_END);
  out_result->projection_gpu_ns =
      PROM_M44_DURATION(PROM_M44_QUERY_PROJECTION_BEGIN, PROM_M44_QUERY_PROJECTION_END);
  out_result->accumulation_gpu_ns =
      PROM_M44_DURATION(PROM_M44_QUERY_ACCUMULATION_BEGIN, PROM_M44_QUERY_ACCUMULATION_END);
  out_result->m44_gpu_ns =
      PROM_M44_DURATION(PROM_M44_QUERY_AGGREGATION_BEGIN, PROM_M44_QUERY_PROJECTION_END);
  out_result->total_m43_m44_gpu_ns =
      PROM_M44_DURATION(3u, PROM_M44_QUERY_PROJECTION_END);
#undef PROM_M44_DURATION
  readback_begin = prom_reduction_now_ns();
  memcpy(request->output, slot->m44_readback.mapped,
         (size_t)out_result->plan.memory.final_readback_bytes);
  readback_cpu_ns = prom_reduction_elapsed_ns(readback_begin, prom_reduction_now_ns());
  out_result->final_readback_ns =
      (uint64_t)((double)(timestamps[PROM_M44_QUERY_READBACK_END] -
                         timestamps[PROM_M44_QUERY_READBACK_BEGIN]) * state->timestamp_period_ns) +
      readback_cpu_ns;
  out_result->submit_count = out_result->plan.submit_count;
  out_result->final_readback_count = 1u;
  out_result->no_intermediate_host_copy = 1u;
  out_result->exact_request_bytes = out_result->attention.plan.memory.exact_retained_bytes +
                                    out_result->plan.memory.exact_request_bytes;
  out_result->retained_bytes = prom_m43_retained_bytes(state, slot) + prom_m44_retained_bytes(state, slot);
  out_result->buffer_allocation_count = state->m43_buffer_grow_count + state->m44_buffer_grow_count;
  out_result->buffer_reuse_count = state->m43_buffer_reuse_count + state->m44_buffer_reuse_count;
  out_result->descriptor_update_count = state->m43_descriptor_update_count +
                                        state->m44_descriptor_update_count;
  out_result->pipeline_create_count = state->m42_pipeline_create_count +
                                      state->m40b_pipeline_create_count +
                                      state->m44_pipeline_create_count +
                                      state->diagnostics.pipeline_create_count;
  out_result->command_buffer_reuse_count = slot->m44_command_reuse_count;
  out_result->wo_generation = state->m44_wo_generation;
  out_result->validation_error_count_before = services_before.validation_error_count;
  out_result->attention.submit_count = 1u;
  out_result->attention.final_readback_count = 0u;
  out_result->attention.no_intermediate_host_copy = 1u;
  out_result->attention.shared_x_conversion_count = out_result->attention.plan.shared_x_conversion_count;
  out_result->attention.shared_x_upload_count = out_result->attention.plan.shared_x_upload_count;
  out_result->attention.shared_x_consumer_count = PROM_M44_HEAD_COUNT;
  out_result->attention.persistent_weight_count = PROM_M44_HEAD_COUNT * PROM_M43_WEIGHT_KIND_COUNT;
  out_result->attention.qkv_projection_dispatch_count = PROM_M44_HEAD_COUNT * PROM_M43_WEIGHT_KIND_COUNT;
  out_result->attention.exact_request_bytes = out_result->attention.plan.memory.exact_retained_bytes;
  out_result->attention.retained_bytes = prom_m43_retained_bytes(state, slot);
  out_result->attention.shared_x_generation = effective_request.attention.shared_x_generation;
  memcpy(out_result->attention.weight_generation, state->m43_weight_generation,
         sizeof(out_result->attention.weight_generation));
  for (head = 0u; head < PROM_M44_HEAD_COUNT; ++head) {
    if (out_result->attention.plan.selected_path[head] == PROM_M42_PATH_COOPERATIVE ||
        out_result->plan.projection_path == PROM_M44_PROJECTION_COOPERATIVE) {
      (void)prom_reactor_runtime_mark_cooperative_matrix_executable(handle);
      break;
    }
  }
  if (prom_reactor_runtime_get_vk_services(handle, &services_after) == PROM_OK) {
    out_result->validation_error_count_after = services_after.validation_error_count;
    out_result->attention.validation_error_count_after = services_after.validation_error_count;
  }
  out_result->attention.validation_error_count_before = services_before.validation_error_count;
  out_result->end_to_end_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->physical_slot_recyclable = 1u;
  out_result->attention.physical_slot_recyclable = 1u;
  out_result->stage = 0u;
  out_result->detail_code = 0;
  return PROM_OK;
}

typedef struct prom_transformer_recorded_block {
  prom_m43_attention_group_request attention_request;
  prom_m43_attention_plan attention_plan;
  prom_device_buffer_view head_view[PROM_M43_HEAD_COUNT];
  PrometheusReductionPlan reduction_plan;
  prom_m44_composed_request projection_request;
  prom_m44_output_projection_plan projection_plan;
  prom_m45_composed_request residual_request;
  prom_m45_residual_plan residual_plan;
  prom_m46_composed_request norm_request;
  prom_m46_rmsnorm_plan norm_plan;
  prom_m47_composed_request ffn_request;
  prom_m47_gated_ffn_plan ffn_plan;
  prom_device_buffer_view input_view;
  prom_device_buffer_view y_view;
  prom_device_buffer_view z_view;
  prom_device_buffer_view n_view;
  prom_device_buffer_view output_view;
  const prom_vk_buffer* input;
  prom_vk_buffer* z;
  prom_vk_buffer* n;
  prom_vk_buffer* output;
  uint64_t y_generation;
} prom_transformer_recorded_block;

static void prom_transformer_fill_view(prom_device_buffer_view* view,
                                       const prom_reduction_runtime_state* state,
                                       const prom_reduction_slot* slot,
                                       const prom_vk_buffer* buffer,
                                       uint32_t rows,
                                       uint32_t columns,
                                       uint32_t row_stride,
                                       uint64_t generation) {
  memset(view, 0, sizeof(*view));
  view->buffer = buffer->buffer;
  view->byte_length = buffer->size;
  view->element_type = PROM_DEVICE_ELEMENT_F32;
  view->logical_rows = rows;
  view->logical_columns = columns;
  view->row_stride_elements = row_stride;
  view->layout = PROM_DEVICE_LAYOUT_ROW_MAJOR;
  view->producer_access = PROM_DEVICE_ACCESS_COMPUTE_WRITE;
  view->required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
  view->owning_device = state->device;
  view->owning_lifetime_id = generation;
  view->owning_slot_id = slot->slot_id;
  view->owning_slot_generation = slot->generation;
}

static void prom_transformer_select_descriptor_bank(
    prom_reduction_slot* slot,
    const prom_transformer_descriptor_bank* bank) {
  memcpy(slot->m43_descriptor_sets, bank->m43, sizeof(slot->m43_descriptor_sets));
  slot->m44_sgemm_descriptor_set = bank->m44_sgemm;
  slot->m44_descriptor_set = bank->m44_wide;
  slot->m45_descriptor_set = bank->m45;
  memcpy(slot->descriptor_sets, bank->m46, sizeof(bank->m46));
  memcpy(slot->m47_descriptor_sets, bank->m47, sizeof(slot->m47_descriptor_sets));
}

static int prom_transformer_setup_attention_descriptors(
    prom_reduction_runtime_state* state,
    prom_reduction_slot* slot,
    const prom_transformer_layer_resources* layer,
    const prom_m43_attention_group_request* request,
    prom_transformer_descriptor_bank* bank,
    const prom_vk_buffer* x_f32,
    const prom_vk_buffer* x_f16,
    prom_transformer_recorded_block* block) {
  uint32_t head;
  for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
    const uint32_t base = head * PROM_M42_DESCRIPTOR_SET_COUNT;
    const uint32_t reduced = block->attention_plan.selected_path[head] != PROM_M42_PATH_A2X4;
    const prom_vk_buffer* x = reduced != 0u ? x_f16 : x_f32;
    const prom_transformer_parameter_resource* wq =
        &layer->attention[head][PROM_M43_WEIGHT_Q];
    const prom_transformer_parameter_resource* wk =
        &layer->attention[head][PROM_M43_WEIGHT_K];
    const prom_transformer_parameter_resource* wv =
        &layer->attention[head][PROM_M43_WEIGHT_V];
    prom_m43_head_slot* head_slot = &slot->m43_head[head];
    prom_m43_update_descriptor(state, bank->m43[base], x,
                               reduced != 0u ? &wq->f16 : &wq->f32, &head_slot->q);
    prom_m43_update_descriptor(state, bank->m43[base + 1u], x,
                               reduced != 0u ? &wk->f16 : &wk->f32, &head_slot->k);
    prom_m43_update_descriptor(state, bank->m43[base + 2u], x,
                               reduced != 0u ? &wv->f16 : &wv->f32, &head_slot->v);
    prom_m43_update_descriptor(state, bank->m43[base + 4u], &head_slot->k, NULL,
                               &head_slot->k_transposed);
    prom_m43_update_descriptor(state, bank->m43[base + 7u], &head_slot->scores, NULL,
                               &head_slot->scores);
    if (reduced != 0u) {
      prom_m43_update_descriptor(state, bank->m43[base + 3u], &head_slot->q, NULL,
                                 &head_slot->q_packed);
      prom_m43_update_descriptor(state, bank->m43[base + 5u], &head_slot->v, NULL,
                                 &head_slot->v_packed);
      prom_m43_update_descriptor(state, bank->m43[base + 6u], &head_slot->q_packed,
                                 &head_slot->k_transposed, &head_slot->scores);
      prom_m43_update_descriptor(state, bank->m43[base + 13u],
                                 &head_slot->probabilities, NULL, &head_slot->p_packed);
      prom_m43_update_descriptor(state, bank->m43[base + 14u], &head_slot->p_packed,
                                 &head_slot->v_packed, &head_slot->output);
    } else {
      prom_m43_update_descriptor(state, bank->m43[base + 6u], &head_slot->q,
                                 &head_slot->k_transposed, &head_slot->scores);
      prom_m43_update_descriptor(state, bank->m43[base + 14u],
                                 &head_slot->probabilities, &head_slot->v,
                                 &head_slot->output);
    }
    prom_transformer_fill_view(&block->head_view[head],
                               state, slot, &head_slot->output,
                               request->tokens, request->head_dim,
                               reduced != 0u ? block->attention_plan.padded_head_dim
                                             : request->head_dim,
                               block->attention_plan.head_replay_id[head]);
  }
  prom_m43_update_descriptor(state,
                             bank->m43[PROM_M43_DESCRIPTOR_SET_COUNT - 1u],
                             x_f32, NULL, x_f16);
  return 1;
}

static int prom_transformer_setup_projection_descriptors(
    prom_reduction_runtime_state* state,
    prom_reduction_slot* slot,
    const prom_transformer_layer_resources* layer,
    const prom_m44_output_projection_plan* plan,
    prom_transformer_descriptor_bank* bank) {
  const prom_vk_buffer* buffers[PROM_M44_WIDE_DESCRIPTOR_BINDING_COUNT];
  const prom_vk_buffer* concatenated;
  uint32_t head;
  for (head = 0u; head < PROM_M44_HEAD_COUNT; ++head)
    buffers[head] = &slot->m43_head[head].output;
  if (plan->aggregation_strategy == PROM_M44_AGGREGATION_INTERLEAVE) {
    concatenated = plan->projection_path == PROM_M44_PROJECTION_A2X4_FP32
                       ? &slot->m44_concat_f32 : &slot->m44_concat_f16;
    buffers[8] = concatenated;
    buffers[9] = concatenated;
    prom_m44_update_wide_descriptor(state, bank->m44_wide, buffers);
    prom_m44_update_sgemm_descriptor(
        state, bank->m44_sgemm, concatenated,
        plan->projection_path == PROM_M44_PROJECTION_A2X4_FP32
            ? &layer->wo.f32 : &layer->wo.f16,
        &slot->m44_output);
  } else {
    buffers[8] = &layer->wo.f16;
    buffers[9] = &slot->m44_output;
    prom_m44_update_wide_descriptor(state, bank->m44_wide, buffers);
  }
  return 1;
}

static void prom_transformer_update_residual_descriptor(
    prom_reduction_runtime_state* state,
    VkDescriptorSet set,
    const prom_vk_buffer* x,
    const prom_vk_buffer* y,
    const prom_vk_buffer* z) {
  prom_reduction_buffer_bindings bindings;
  bindings.input = x;
  bindings.auxiliary0 = y;
  bindings.auxiliary1 = z;
  bindings.output = z;
  prom_reduction_update_descriptor_set(state, set, &bindings);
  state->m48_descriptor_update_count += 1u;
}

static int prom_transformer_validate_layer_resources(
    const prom_transformer_layer_resources* layer,
    const prom_m48_stack_request* request,
    uint32_t layer_index) {
  uint32_t head;
  uint32_t weight;
  if (layer->layer_index != layer_index || layer->model_width != request->model_width ||
      layer->head_dim != request->head_dim || layer->ffn_width != request->ffn_width)
    return 0;
  for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
    for (weight = 0u; weight < PROM_M43_WEIGHT_KIND_COUNT; ++weight) {
      const uint32_t resource_index = head * PROM_M43_WEIGHT_KIND_COUNT + weight;
      const prom_transformer_parameter_resource* resource = &layer->attention[head][weight];
      if (resource->generation == 0u || resource->hash == 0u ||
          resource->generation != request->required_generation[layer_index][resource_index] ||
          resource->rows != request->model_width || resource->columns != request->head_dim)
        return 0;
    }
  }
  if (layer->wo.generation != request->required_generation[layer_index][PROM_M48_RESOURCE_WO] ||
      layer->wo.hash == 0u || layer->wo.rows != request->model_width ||
      layer->wo.columns != request->model_width ||
      layer->rmsnorm.generation !=
          request->required_generation[layer_index][PROM_M48_RESOURCE_RMSNORM] ||
      layer->rmsnorm.hash == 0u || layer->rmsnorm.columns != request->model_width)
    return 0;
  for (weight = 0u; weight < PROM_M47_WEIGHT_COUNT; ++weight) {
    const prom_transformer_parameter_resource* resource = &layer->ffn[weight];
    const uint32_t resource_index = PROM_M48_RESOURCE_WGATE + weight;
    const uint32_t rows = weight == PROM_M47_WEIGHT_DOWN
                              ? request->ffn_width : request->model_width;
    const uint32_t columns = weight == PROM_M47_WEIGHT_DOWN
                                 ? request->model_width : request->ffn_width;
    if (resource->generation == 0u || resource->hash == 0u ||
        resource->generation != request->required_generation[layer_index][resource_index] ||
        resource->rows != rows || resource->columns != columns)
      return 0;
  }
  return 1;
}

static int prom_transformer_prepare_block(
    prom_reduction_runtime_state* state,
    prom_reduction_slot* slot,
    const prom_vk_runtime_services* services,
    const prom_m48_stack_request* request,
    uint32_t layer_index,
    const prom_vk_buffer* input,
    uint32_t input_row_stride,
    uint64_t input_generation,
    prom_vk_buffer* output,
    prom_transformer_recorded_block* block) {
  const prom_transformer_layer_resources* layer = &state->m48_layer[layer_index];
  prom_transformer_descriptor_bank* bank = &slot->m48_descriptors[layer_index];
  prom_m43_plan_request attention_plan_request;
  prom_m44_plan_request projection_plan_request;
  prom_m45_plan_request residual_plan_request;
  prom_m46_plan_request norm_plan_request;
  prom_m47_plan_request ffn_plan_request;
  PrometheusReductionRequest reduction_request;
  uint64_t head_output_elements;
  uint64_t activation_elements;
  uint64_t input_hash;
  VkDeviceSize packed_x_bytes;
  uint32_t selected_path =
      request->numerical_control_mode == PROM_M48_NUMERICAL_CONTROL_M49B &&
              request->controller_layer_projection_path[layer_index] != 0u
          ? request->controller_layer_projection_path[layer_index]
          : request->audit_layer_projection_path[layer_index] != 0u
                ? request->audit_layer_projection_path[layer_index]
                : request->projection_path;
  uint32_t selected_precision;
  uint32_t selected_gating_strategy;
  uint32_t head;
  uint32_t weight;
  if (!prom_transformer_validate_layer_resources(layer, request, layer_index)) return 0;
  memset(block, 0, sizeof(*block));
  block->input = input;
  if (selected_path == PROM_M47_PROJECTION_COOPERATIVE &&
      (services->cooperative_matrix_feature_enabled == 0u || services->subgroup_size != 32u)) {
    if (request->allow_fallback == 0u) return 0;
    selected_path = PROM_M47_PROJECTION_CONVENTIONAL_FP16;
  }
  selected_precision = selected_path == PROM_M47_PROJECTION_A2X4_FP32
                           ? PROM_M42_PRECISION_FP32 : PROM_M42_PRECISION_F16_ROUNDED;
  selected_gating_strategy =
      selected_path == PROM_M47_PROJECTION_A2X4_FP32 &&
              request->gating_strategy == PROM_M47_GATING_FUSED_DIRECT_PACKED
          ? PROM_M47_GATING_FUSED_FP32 : request->gating_strategy;
  if (!prom_m40b_checked_product_u64(request->tokens, request->model_width,
                                     &activation_elements) ||
      !prom_m40b_checked_product_u64(request->tokens, request->head_dim,
                                     &head_output_elements) ||
      !prom_m43_checked_scale_u64(head_output_elements, PROM_M43_HEAD_COUNT,
                                  &head_output_elements)) return 0;
  input_hash = prom_m40b_hash_u64(1469598103934665603ull, input_generation);
  input_hash = prom_reduction_hash_u32(input_hash, layer_index);
  memset(&block->attention_request, 0, sizeof(block->attention_request));
  block->attention_request.head_count = PROM_M43_HEAD_COUNT;
  block->attention_request.tokens = request->tokens;
  block->attention_request.model_width = request->model_width;
  block->attention_request.head_dim = request->head_dim;
  block->attention_request.precision_policy = selected_precision;
  block->attention_request.allow_fallback = request->allow_fallback;
  block->attention_request.input_mode = PROM_M42_INPUT_RESIDENT_X;
  block->attention_request.execution_strategy = request->attention_strategy;
  block->attention_request.shared_x_generation = input_generation;
  block->attention_request.output_element_count = head_output_elements;
  if ((request->fault_point == PROM_M48_FAULT_DURING_LAYER_0_ATTENTION && layer_index == 0u) ||
      (request->fault_point == PROM_M48_FAULT_DURING_LAYER_3_ATTENTION && layer_index == 3u)) {
    block->attention_request.fault_point = PROM_M43_FAULT_MID_PROJECTIONS;
    block->attention_request.fault_head = 3u;
  }
  memset(&attention_plan_request, 0, sizeof(attention_plan_request));
  attention_plan_request.head_count = PROM_M43_HEAD_COUNT;
  attention_plan_request.tokens = request->tokens;
  attention_plan_request.model_width = request->model_width;
  attention_plan_request.head_dim = request->head_dim;
  attention_plan_request.precision_policy = selected_precision;
  attention_plan_request.allow_fallback = request->allow_fallback;
  attention_plan_request.input_mode = PROM_M42_INPUT_RESIDENT_X;
  attention_plan_request.execution_strategy = request->attention_strategy;
  attention_plan_request.cooperative_capability_state = services->cooperative_matrix_state;
  attention_plan_request.shared_x_generation = input_generation;
  attention_plan_request.shared_x_hash = input_hash;
  for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
    block->attention_request.preferred_path[head] = selected_path;
    attention_plan_request.preferred_path[head] = selected_path;
    for (weight = 0u; weight < PROM_M43_WEIGHT_KIND_COUNT; ++weight) {
      const prom_transformer_parameter_resource* resource = &layer->attention[head][weight];
      block->attention_request.required_weight_generation[head][weight] = resource->generation;
      attention_plan_request.weight_generation[head][weight] = resource->generation;
      attention_plan_request.weight_hash[head][weight] = resource->hash;
    }
  }
  if (prom_m43_attention_plan_build(&attention_plan_request, &block->attention_plan) != PROM_OK ||
      !prom_m44_strip_m43_final_readback(&block->attention_plan)) return 0;
  memset(&reduction_request, 0, sizeof(reduction_request));
  reduction_request.struct_size = sizeof(reduction_request);
  reduction_request.row_count = request->tokens;
  reduction_request.elements_per_row = request->tokens;
  reduction_request.input_element_count = (uint64_t)request->tokens * request->tokens;
  reduction_request.output_element_count = reduction_request.input_element_count;
  reduction_request.operation = PROM_REDUCTION_OPERATION_SOFTMAX;
  reduction_request.finalization = PROM_REDUCTION_FINALIZATION_STABLE_SOFTMAX;
  if (prom_reactor_reduction_plan_impl(&reduction_request, &block->reduction_plan) != PROM_OK ||
      !prom_m43_prepare_execution_buffers(state, slot, &block->attention_request,
                                           &block->attention_plan, 0u)) return 0;
  packed_x_bytes = (VkDeviceSize)((((uint64_t)block->attention_plan.padded_tokens *
                                    block->attention_plan.padded_model_width + 1u) / 2u) *
                                  sizeof(uint32_t));
  if (!prom_m48_ensure_buffer(state, &slot->m43_x_f16, packed_x_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
      !prom_transformer_setup_attention_descriptors(
          state, slot, layer, &block->attention_request, bank, input,
          &slot->m43_x_f16, block)) return 0;

  memset(&projection_plan_request, 0, sizeof(projection_plan_request));
  memcpy(projection_plan_request.head_views, block->head_view,
         sizeof(projection_plan_request.head_views));
  for (head = 0u; head < PROM_M44_HEAD_COUNT; ++head)
    projection_plan_request.head_views[head].required_consumer_access =
        PROM_DEVICE_ACCESS_COMPUTE_READ;
  projection_plan_request.head_count = PROM_M44_HEAD_COUNT;
  projection_plan_request.tokens = request->tokens;
  projection_plan_request.head_dim = request->head_dim;
  projection_plan_request.model_width = request->model_width;
  projection_plan_request.precision_policy = selected_path == PROM_M47_PROJECTION_A2X4_FP32
                                                  ? PROM_M42_PRECISION_FP32
                                                  : PROM_M42_PRECISION_F16_ROUNDED;
  projection_plan_request.aggregation_strategy = request->output_projection_strategy;
  projection_plan_request.projection_path = selected_path;
  projection_plan_request.submit_plan = PROM_M44_SUBMIT_ONE_COMMAND_BUFFER;
  projection_plan_request.cooperative_capability_state = services->cooperative_matrix_state;
  projection_plan_request.wo_generation = layer->wo.generation;
  projection_plan_request.wo_hash = layer->wo.hash;
  projection_plan_request.m43_aggregate_replay_id = block->attention_plan.aggregate_replay_id;
  if (prom_m44_output_projection_plan_build(&projection_plan_request,
                                             &block->projection_plan) != PROM_OK ||
      block->projection_plan.eligibility.eligible == 0u ||
      !prom_m44_prepare_execution_buffers(state, slot, &block->projection_plan, 0u, 0u) ||
      !prom_transformer_setup_projection_descriptors(state, slot, layer,
                                                       &block->projection_plan, bank) ||
      !prom_m45_strip_m44_final_readback(&block->projection_plan)) return 0;
  block->y_generation = prom_m40b_hash_u64(block->projection_plan.replay_id,
                                            input_generation);
  block->y_generation = prom_reduction_hash_u32(block->y_generation, layer_index);
  if (block->y_generation == 0u) block->y_generation = 1u;
  prom_transformer_fill_view(&block->input_view, state, slot, input,
                             request->tokens, request->model_width,
                             input_row_stride, input_generation);
  prom_transformer_fill_view(&block->y_view, state, slot, &slot->m44_output,
                             request->tokens, request->model_width,
                             block->projection_plan.output_row_stride,
                             block->y_generation);
  memset(&residual_plan_request, 0, sizeof(residual_plan_request));
  residual_plan_request.x_view = block->input_view;
  residual_plan_request.y_view = block->y_view;
  residual_plan_request.tokens = request->tokens;
  residual_plan_request.model_width = request->model_width;
  residual_plan_request.strategy = PROM_M45_STRATEGY_IN_PLACE_Y;
  residual_plan_request.submit_policy = PROM_M45_SUBMIT_ONE_COMMAND_BUFFER;
  residual_plan_request.precision_policy = PROM_M45_PRECISION_FP32;
  residual_plan_request.y_exclusive = 1u;
  residual_plan_request.expected_x_generation = input_generation;
  residual_plan_request.expected_y_generation = block->y_generation;
  residual_plan_request.m44_replay_id = block->projection_plan.replay_id;
  if (prom_m45_residual_plan_build(&residual_plan_request, &block->residual_plan) != PROM_OK ||
      block->residual_plan.eligibility.eligible == 0u) return 0;
  block->z = &slot->m44_output;
  prom_transformer_update_residual_descriptor(state, bank->m45, input,
                                               &slot->m44_output, block->z);
  prom_transformer_fill_view(&block->z_view, state, slot, block->z,
                             request->tokens, request->model_width,
                             block->residual_plan.z_row_stride,
                             block->residual_plan.z_generation);

  memset(&norm_plan_request, 0, sizeof(norm_plan_request));
  norm_plan_request.z_view = block->z_view;
  norm_plan_request.tokens = request->tokens;
  norm_plan_request.model_width = request->model_width;
  norm_plan_request.epsilon = request->epsilon;
  norm_plan_request.strategy = request->rmsnorm_strategy;
  norm_plan_request.submit_policy = PROM_M46_SUBMIT_ONE_COMMAND_BUFFER;
  norm_plan_request.z_exclusive = 1u;
  norm_plan_request.expected_z_generation = block->residual_plan.z_generation;
  norm_plan_request.weight_generation = layer->rmsnorm.generation;
  norm_plan_request.weight_hash = layer->rmsnorm.hash;
  norm_plan_request.m45_replay_id = block->residual_plan.replay_id;
  if (prom_m46_rmsnorm_plan_build(&norm_plan_request, &block->norm_plan) != PROM_OK ||
      block->norm_plan.eligibility_eligible == 0u ||
      !prom_m46_ensure_buffer(state, &slot->m46_partials,
                              (VkDeviceSize)(block->norm_plan.memory.partial_sum_bytes != 0u
                                                  ? block->norm_plan.memory.partial_sum_bytes
                                                  : sizeof(float)),
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_m46_ensure_buffer(state, &slot->m46_inv_rms,
                              (VkDeviceSize)block->norm_plan.memory.inv_rms_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      (request->rmsnorm_strategy == PROM_M46_STRATEGY_SEPARATE_OUTPUT &&
       !prom_m46_ensure_buffer(state, &slot->m46_output,
                               (VkDeviceSize)block->norm_plan.memory.n_device_bytes,
                               VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0))) return 0;
  block->n = request->rmsnorm_strategy == PROM_M46_STRATEGY_IN_PLACE_Z
                 ? block->z : &slot->m46_output;
  prom_m46_update_descriptor(state, bank->m46[0], block->z, &slot->m46_inv_rms,
                             &slot->m46_inv_rms,
                             block->norm_plan.reduction_plan == PROM_M46_REDUCTION_STAGED
                                 ? &slot->m46_partials : &slot->m46_inv_rms);
  if (block->norm_plan.reduction_plan == PROM_M46_REDUCTION_STAGED)
    prom_m46_update_descriptor(state, bank->m46[1], &slot->m46_partials,
                               &slot->m46_inv_rms, &slot->m46_inv_rms,
                               &slot->m46_inv_rms);
  prom_m46_update_descriptor(state, bank->m46[2], block->z, &layer->rmsnorm.f32,
                             &slot->m46_inv_rms, block->n);
  prom_transformer_fill_view(&block->n_view, state, slot, block->n,
                             request->tokens, request->model_width,
                             block->norm_plan.n_row_stride,
                             block->norm_plan.n_generation);

  memset(&ffn_plan_request, 0, sizeof(ffn_plan_request));
  ffn_plan_request.n_view = block->n_view;
  ffn_plan_request.tokens = request->tokens;
  ffn_plan_request.model_width = request->model_width;
  ffn_plan_request.ffn_width = request->ffn_width;
  ffn_plan_request.projection_path = selected_path;
  ffn_plan_request.gating_strategy = selected_gating_strategy;
  ffn_plan_request.residual_strategy = request->residual_strategy;
  ffn_plan_request.submit_policy = PROM_M47_SUBMIT_ONE_COMMAND_BUFFER;
  ffn_plan_request.remaining_n_consumer_count = 1u;
  ffn_plan_request.expected_n_generation = block->norm_plan.n_generation;
  ffn_plan_request.m46_replay_id = block->norm_plan.replay_id;
  for (weight = 0u; weight < PROM_M47_WEIGHT_COUNT; ++weight) {
    ffn_plan_request.weight_generation[weight] = layer->ffn[weight].generation;
    ffn_plan_request.weight_hash[weight] = layer->ffn[weight].hash;
  }
  if (prom_m47_gated_ffn_plan_build(&ffn_plan_request, &block->ffn_plan) != PROM_OK ||
      block->ffn_plan.eligibility_eligible == 0u ||
      !prom_m47_ensure_buffer(state, &slot->m47_gate,
                              (VkDeviceSize)block->ffn_plan.memory.gate_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT |
                                  VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_m47_ensure_buffer(state, &slot->m47_up,
                              (VkDeviceSize)block->ffn_plan.memory.up_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT |
                                  VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      (selected_path != PROM_M47_PROJECTION_A2X4_FP32 &&
       !prom_m47_ensure_buffer(state, &slot->m47_n_packed,
                               (VkDeviceSize)block->ffn_plan.memory.n_packed_bytes,
                               VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) ||
      (selected_gating_strategy == PROM_M47_GATING_SEPARATE &&
       !prom_m47_ensure_buffer(state, &slot->m47_activated_gate,
                               (VkDeviceSize)block->ffn_plan.memory.activated_gate_bytes,
                               VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) ||
      (selected_gating_strategy != PROM_M47_GATING_FUSED_DIRECT_PACKED &&
       !prom_m47_ensure_buffer(state, &slot->m47_hidden,
                               (VkDeviceSize)block->ffn_plan.memory.hidden_f32_bytes,
                               VK_BUFFER_USAGE_STORAGE_BUFFER_BIT |
                                   VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) ||
      (selected_path != PROM_M47_PROJECTION_A2X4_FP32 &&
       !prom_m47_ensure_buffer(state, &slot->m47_hidden_packed,
                               (VkDeviceSize)block->ffn_plan.memory.hidden_packed_bytes,
                               VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) ||
      (request->residual_strategy == PROM_M47_RESIDUAL_SEPARATE_OUTPUT &&
       !prom_m47_ensure_buffer(state, &slot->m47_down,
                               (VkDeviceSize)block->ffn_plan.memory.down_bytes,
                               VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                               VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) ||
      !prom_m48_ensure_buffer(state, output,
                              (VkDeviceSize)block->ffn_plan.memory.down_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL)) return 0;
  block->output = output;
  prom_transformer_fill_view(&block->output_view, state, slot, output,
                             request->tokens, request->model_width,
                             block->ffn_plan.output_row_stride,
                             block->ffn_plan.output_generation);
  {
    const uint32_t reduced = selected_path != PROM_M47_PROJECTION_A2X4_FP32;
    const prom_vk_buffer* projection_input = reduced != 0u
                                                 ? &slot->m47_n_packed : block->n;
    prom_vk_buffer* activated = selected_gating_strategy == PROM_M47_GATING_SEPARATE
                                    ? &slot->m47_activated_gate : &slot->m47_gate;
    prom_vk_buffer* hidden = selected_gating_strategy == PROM_M47_GATING_FUSED_DIRECT_PACKED
                                 ? &slot->m47_gate : &slot->m47_hidden;
    const prom_vk_buffer* hidden_input = reduced != 0u
                                             ? &slot->m47_hidden_packed : &slot->m47_hidden;
    prom_vk_buffer* down = request->residual_strategy == PROM_M47_RESIDUAL_IN_PLACE_DOWN
                               ? output : &slot->m47_down;
    prom_m47_update_descriptor(state, bank->m47[0], block->n, block->n, block->n,
                               reduced != 0u ? &slot->m47_n_packed : block->n);
    prom_m47_update_descriptor(state, bank->m47[1], projection_input,
                               reduced != 0u ? &layer->ffn[PROM_M47_WEIGHT_GATE].f16
                                             : &layer->ffn[PROM_M47_WEIGHT_GATE].f32,
                               &slot->m47_gate, &slot->m47_gate);
    prom_m47_update_descriptor(state, bank->m47[2], projection_input,
                               reduced != 0u ? &layer->ffn[PROM_M47_WEIGHT_UP].f16
                                             : &layer->ffn[PROM_M47_WEIGHT_UP].f32,
                               &slot->m47_up, &slot->m47_up);
    prom_m47_update_descriptor(state, bank->m47[3], &slot->m47_gate,
                               &slot->m47_up, activated, hidden);
    prom_m47_update_descriptor(state, bank->m47[4],
                               selected_gating_strategy == PROM_M47_GATING_FUSED_DIRECT_PACKED
                                   ? &slot->m47_gate : hidden,
                               &slot->m47_up, hidden,
                               reduced != 0u ? &slot->m47_hidden_packed : hidden);
    prom_m47_update_descriptor(state, bank->m47[5], hidden_input,
                               reduced != 0u ? &layer->ffn[PROM_M47_WEIGHT_DOWN].f16
                                             : &layer->ffn[PROM_M47_WEIGHT_DOWN].f32,
                               down, down);
    prom_m47_update_descriptor(state, bank->m47[6], block->n, down, output, output);
  }
  prom_transformer_select_descriptor_bank(slot, bank);
  memset(&block->projection_request, 0, sizeof(block->projection_request));
  block->projection_request.attention = block->attention_request;
  block->projection_request.aggregation_strategy = request->output_projection_strategy;
  block->projection_request.projection_path = selected_path;
  block->projection_request.submit_plan = PROM_M44_SUBMIT_ONE_COMMAND_BUFFER;
  block->projection_request.required_wo_generation = layer->wo.generation;
  memset(&block->residual_request, 0, sizeof(block->residual_request));
  block->residual_request.attention = block->attention_request;
  block->residual_request.aggregation_strategy = request->output_projection_strategy;
  block->residual_request.projection_path = selected_path;
  block->residual_request.residual_strategy = PROM_M45_STRATEGY_IN_PLACE_Y;
  block->residual_request.submit_policy = PROM_M45_SUBMIT_ONE_COMMAND_BUFFER;
  block->residual_request.required_wo_generation = layer->wo.generation;
  memset(&block->norm_request, 0, sizeof(block->norm_request));
  block->norm_request.epsilon = request->epsilon;
  block->norm_request.strategy = request->rmsnorm_strategy;
  block->norm_request.submit_policy = PROM_M46_SUBMIT_ONE_COMMAND_BUFFER;
  block->norm_request.required_weight_generation = layer->rmsnorm.generation;
  if (request->fault_point == PROM_M48_FAULT_DURING_LAYER_1_RMSNORM && layer_index == 1u)
    block->norm_request.fault_point = PROM_M46_FAULT_DURING_APPLY;
  memset(&block->ffn_request, 0, sizeof(block->ffn_request));
  block->ffn_request.ffn_width = request->ffn_width;
  block->ffn_request.projection_path = selected_path;
  block->ffn_request.gating_strategy = selected_gating_strategy;
  block->ffn_request.residual_strategy = request->residual_strategy;
  block->ffn_request.submit_policy = PROM_M47_SUBMIT_ONE_COMMAND_BUFFER;
  for (weight = 0u; weight < PROM_M47_WEIGHT_COUNT; ++weight)
    block->ffn_request.required_weight_generation[weight] = layer->ffn[weight].generation;
  if ((request->fault_point == PROM_M48_FAULT_DURING_LAYER_1_FFN && layer_index == 1u) ||
      (request->fault_point == PROM_M48_FAULT_DURING_LAYER_3_FFN && layer_index == 3u))
    block->ffn_request.fault_point = PROM_M47_FAULT_DURING_DOWN;
  return 1;
}

/* The bounded M47 compatibility split uses this exact prefix, followed by the
   shared suffix below. Neither helper owns command-buffer lifetime. */
static int prom_transformer_record_audit_destination_begin(
    VkCommandBuffer command_buffer,
    prom_vk_buffer* destination) {
  return prom_m43_one_buffer_barrier(command_buffer, destination, destination->size,
                                     VK_ACCESS_HOST_READ_BIT,
                                     VK_ACCESS_TRANSFER_WRITE_BIT,
                                     VK_PIPELINE_STAGE_HOST_BIT,
                                     VK_PIPELINE_STAGE_TRANSFER_BIT);
}

static int prom_transformer_record_audit_source(
    VkCommandBuffer command_buffer,
    prom_vk_buffer* source,
    uint32_t rows,
    uint32_t columns,
    uint32_t row_stride,
    uint32_t destination_row_offset,
    prom_vk_buffer* destination,
    VkAccessFlags restore_access) {
  VkBufferCopy copy;
  uint32_t row;
  if (!prom_m43_one_buffer_barrier(command_buffer, source, source->size,
                                   VK_ACCESS_SHADER_WRITE_BIT,
                                   VK_ACCESS_TRANSFER_READ_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT)) return 0;
  for (row = 0u; row < rows; ++row) {
    memset(&copy, 0, sizeof(copy));
    copy.srcOffset = (VkDeviceSize)((uint64_t)row * row_stride * sizeof(float));
    copy.dstOffset = (VkDeviceSize)((uint64_t)(destination_row_offset + row) *
                                    columns * sizeof(float));
    copy.size = (VkDeviceSize)((uint64_t)columns * sizeof(float));
    vkCmdCopyBuffer(command_buffer, source->buffer, destination->buffer, 1u, &copy);
  }
  if (restore_access != 0u &&
      !prom_m43_one_buffer_barrier(command_buffer, source, source->size,
                                   VK_ACCESS_TRANSFER_READ_BIT, restore_access,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT)) return 0;
  return 1;
}

static int prom_transformer_record_audit_destination_end(
    VkCommandBuffer command_buffer,
    prom_vk_buffer* destination) {
  return prom_m43_one_buffer_barrier(command_buffer, destination, destination->size,
                                     VK_ACCESS_TRANSFER_WRITE_BIT,
                                     VK_ACCESS_HOST_READ_BIT,
                                     VK_PIPELINE_STAGE_TRANSFER_BIT,
                                     VK_PIPELINE_STAGE_HOST_BIT);
}

static int prom_transformer_record_stage_audit(
    prom_reduction_slot* slot,
    prom_transformer_record_context* context,
    prom_transformer_recorded_block* block,
    uint32_t stage) {
  prom_vk_buffer* destination = context->audit_readback;
  uint32_t head;
  if (context->audit_stage != stage) return 1;
  if (destination == NULL ||
      !prom_transformer_record_audit_destination_begin(context->command_buffer,
                                                        destination)) return 0;
  if (stage == PROM_M48_AUDIT_STAGE_ATTENTION) {
    for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
      if (!prom_transformer_record_audit_source(
              context->command_buffer, &slot->m43_head[head].output,
              block->attention_plan.tokens, block->attention_plan.head_dim,
              block->head_view[head].row_stride_elements,
              head * block->attention_plan.tokens, destination,
              VK_ACCESS_SHADER_READ_BIT)) return 0;
    }
  } else if (stage == PROM_M48_AUDIT_STAGE_OUTPUT_PROJECTION) {
    if (!prom_transformer_record_audit_source(
            context->command_buffer, &slot->m44_output,
            block->projection_plan.tokens, block->projection_plan.model_width,
            block->projection_plan.output_row_stride, 0u, destination,
            VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT)) return 0;
  } else if (stage == PROM_M48_AUDIT_STAGE_FIRST_RESIDUAL) {
    if (!prom_transformer_record_audit_source(
            context->command_buffer, block->z,
            block->residual_plan.tokens, block->residual_plan.model_width,
            block->residual_plan.z_row_stride, 0u, destination,
            VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT)) return 0;
  } else if (stage == PROM_M48_AUDIT_STAGE_RMSNORM) {
    if (!prom_transformer_record_audit_source(
            context->command_buffer, block->n,
            block->norm_plan.tokens, block->norm_plan.model_width,
            block->norm_plan.n_row_stride, 0u, destination,
            VK_ACCESS_SHADER_READ_BIT)) return 0;
  } else if (stage == PROM_M48_AUDIT_STAGE_FFN) {
    if (!prom_transformer_record_audit_source(
            context->command_buffer, block->output,
            block->ffn_plan.tokens, block->ffn_plan.model_width,
            block->ffn_plan.output_row_stride, 0u, destination, 0u)) return 0;
  } else {
    return 0;
  }
  return prom_transformer_record_audit_destination_end(context->command_buffer,
                                                        destination);
}

static int prom_transformer_record_block_prefix(
    prom_reduction_runtime_state* state,
    prom_reduction_slot* slot,
    prom_transformer_record_context* context,
    prom_transformer_recorded_block* block,
    uint32_t* out_fault_stage) {
  uint32_t m43_partial = 0u;
  uint32_t m43_uncertain = 0u;
  uint32_t m44_partial = 0u;
  uint32_t m45_partial = 0u;
  uint32_t m46_partial = 0u;
  const uint32_t reduced = block->ffn_plan.projection_path != PROM_M47_PROJECTION_A2X4_FP32;
  if (out_fault_stage != NULL) *out_fault_stage = 0u;
  slot->active_query_base = context->query_base;
  prom_transformer_select_descriptor_bank(slot, context->descriptors);
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE)
    vkCmdResetQueryPool(context->command_buffer, state->query_pool,
                        context->query_base, context->query_count);
  prom_m42_write_timestamp(state, slot, context->command_buffer,
                           VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0u);
  prom_m42_write_timestamp(state, slot, context->command_buffer,
                           VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 1u);
  if (reduced != 0u) {
    prom_m42_record_pack(state, context->command_buffer,
                         context->descriptors->m43[PROM_M43_DESCRIPTOR_SET_COUNT - 1u],
                         block->attention_plan.tokens,
                         block->attention_plan.model_width,
                         block->input_view.row_stride_elements,
                         block->attention_plan.padded_tokens,
                         block->attention_plan.padded_model_width, 0u);
    if (!prom_m43_one_buffer_barrier(
            context->command_buffer, &slot->m43_x_f16,
            (VkDeviceSize)((((uint64_t)block->attention_plan.padded_tokens *
                             block->attention_plan.padded_model_width + 1u) / 2u) *
                           sizeof(uint32_t)),
            VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
            VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT)) return 0;
  }
  prom_m42_write_timestamp(state, slot, context->command_buffer,
                           VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 2u);
  if (prom_m43_record_grouped_internal(
          state, slot, &block->attention_request, &block->attention_plan,
          &block->reduction_plan, &m43_partial, &m43_uncertain, 0u, 1u,
          context->command_buffer, context->descriptors->m43, 1u, 1u) != 1 ||
      m43_partial != 0u || m43_uncertain != 0u) {
    if (out_fault_stage != NULL) *out_fault_stage = 43u;
    return 0;
  }
  if (!prom_transformer_record_stage_audit(slot, context, block,
                                           PROM_M48_AUDIT_STAGE_ATTENTION)) return 0;
  if (prom_m44_record_projection_tail(state, slot, &block->projection_request,
                                       &block->projection_plan,
                                       context->command_buffer, 1u, 0u,
                                       &m44_partial) != 1 || m44_partial != 0u) {
    if (out_fault_stage != NULL) *out_fault_stage = 44u;
    return 0;
  }
  if (!prom_transformer_record_stage_audit(
          slot, context, block, PROM_M48_AUDIT_STAGE_OUTPUT_PROJECTION)) return 0;
  if (prom_m45_record_residual_tail(state, slot, &block->residual_request,
                                     &block->residual_plan,
                                     block->input,
                                     block->z, context->command_buffer, 1u,
                                     &m45_partial) != 1 || m45_partial != 0u) {
    if (out_fault_stage != NULL) *out_fault_stage = 45u;
    return 0;
  }
  if (!prom_transformer_record_stage_audit(
          slot, context, block, PROM_M48_AUDIT_STAGE_FIRST_RESIDUAL)) return 0;
  if (prom_m46_record_tail(state, slot, &block->norm_request, &block->norm_plan,
                           block->z, block->n, context->command_buffer, 1u,
                           0u, &m46_partial) != 1 || m46_partial != 0u) {
    if (out_fault_stage != NULL) *out_fault_stage = 46u;
    return 0;
  }
  if (!prom_transformer_record_stage_audit(slot, context, block,
                                           PROM_M48_AUDIT_STAGE_RMSNORM)) return 0;
  return 1;
}

static int prom_transformer_record_block_suffix(
    prom_reduction_runtime_state* state,
    prom_reduction_slot* slot,
    prom_transformer_record_context* context,
    prom_transformer_recorded_block* block,
    uint32_t* out_fault_stage) {
  uint32_t m47_partial = 0u;
  prom_transformer_select_descriptor_bank(slot, context->descriptors);
  if (prom_m47_record_tail(state, slot, &block->ffn_request, &block->ffn_plan,
                            block->n, block->output, context->command_buffer, 1u,
                            0u, 0u, &m47_partial) != 1 || m47_partial != 0u) {
    if (out_fault_stage != NULL) *out_fault_stage = 47u;
    return 0;
  }
  if (!prom_transformer_record_stage_audit(slot, context, block,
                                           PROM_M48_AUDIT_STAGE_FFN)) return 0;
  return 1;
}

static int prom_transformer_record_block(
    prom_reduction_runtime_state* state,
    prom_reduction_slot* slot,
    prom_transformer_record_context* context,
    prom_transformer_recorded_block* block,
    uint32_t* out_fault_stage) {
  if (!prom_transformer_record_block_prefix(state, slot, context, block, out_fault_stage))
    return 0;
  return prom_transformer_record_block_suffix(state, slot, context, block,
                                               out_fault_stage);
}

/* This is a fixed-size transfer canary on the completed stack output.  It is
   intentionally kept beside block recording because it shares that output's
   exact row stride and command-buffer lifetime. */
static int prom_m49b_record_stack_canary(
    VkCommandBuffer command_buffer, const prom_transformer_recorded_block* final_block,
    const prom_m48_stack_request* request, prom_reduction_slot* slot,
    uint64_t execution_identity, uint32_t output_already_transfer_ready) {
  uint32_t coordinates[PROM_NUM_M49B_MAX_SAMPLES];
  uint32_t index;
  uint64_t elements;
  const uint64_t shape_identity = prom_m49b_shape_identity(request);
  if (command_buffer == VK_NULL_HANDLE || final_block == NULL || request == NULL || slot == NULL ||
      final_block->output == NULL ||
      !prom_m40b_checked_product_u64(request->tokens, request->model_width, &elements)) return 0;
  if (!prom_num_m49b_derive_coordinates(shape_identity, execution_identity,
                                        (uint32_t)elements,
                                        PROM_NUM_M49B_MAX_SAMPLES, coordinates)) return 0;
  if (output_already_transfer_ready == 0u &&
      !prom_m43_one_buffer_barrier(command_buffer, final_block->output,
                                   final_block->output->size,
                                   VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT)) return 0;
  for (index = 0u; index < PROM_NUM_M49B_MAX_SAMPLES; ++index) {
    VkBufferCopy copy;
    const uint32_t row = coordinates[index] / request->model_width;
    const uint32_t column = coordinates[index] % request->model_width;
    memset(&copy, 0, sizeof(copy));
    copy.srcOffset = (VkDeviceSize)((uint64_t)(row * final_block->ffn_plan.output_row_stride +
                                                column) * sizeof(float));
    copy.dstOffset = (VkDeviceSize)((uint64_t)index * sizeof(float));
    copy.size = sizeof(float);
    vkCmdCopyBuffer(command_buffer, final_block->output->buffer,
                    slot->m49b_canary_readback.buffer, 1u, &copy);
  }
  return prom_m43_one_buffer_barrier(command_buffer, &slot->m49b_canary_readback,
                                     (VkDeviceSize)(PROM_NUM_M49B_MAX_SAMPLES * sizeof(float)),
                                     VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_HOST_READ_BIT,
                                     VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_HOST_BIT);
}

static uint64_t prom_m48_duration_ns(const prom_reduction_runtime_state* state,
                                     const uint64_t* timestamps,
                                     uint32_t begin,
                                     uint32_t end) {
  if (timestamps[end] < timestamps[begin]) return 0u;
  return (uint64_t)((double)(timestamps[end] - timestamps[begin]) *
                    state->timestamp_period_ns);
}

static void prom_m48_fill_layer_timings(const prom_reduction_runtime_state* state,
                                        const uint64_t* timestamps,
                                        const prom_transformer_recorded_block* block,
                                        const prom_m48_layer_plan* plan,
                                        prom_m48_layer_execution_result* result) {
  memset(result, 0, sizeof(*result));
  result->layer_index = plan->layer;
  result->selected_projection_path = block->ffn_plan.projection_path;
  result->dispatch_count = block->attention_plan.dispatch_count +
                           block->projection_plan.dispatch_count +
                           block->residual_plan.dispatch_count +
                           block->norm_plan.dispatch_count +
                           block->ffn_plan.dispatch_count;
  result->attention_strategy = block->attention_plan.execution_strategy;
  result->output_projection_strategy = block->projection_plan.aggregation_strategy;
  result->rmsnorm_strategy = block->norm_plan.strategy;
  result->gating_strategy = block->ffn_plan.gating_strategy;
  result->residual_strategy = block->ffn_plan.residual_strategy;
  result->attention_gpu_ns = prom_m48_duration_ns(state, timestamps, 3u,
                                                   PROM_M43_QUERY_GROUP_END);
  result->output_projection_gpu_ns = prom_m48_duration_ns(
      state, timestamps, PROM_M44_QUERY_AGGREGATION_BEGIN,
      PROM_M44_QUERY_PROJECTION_END);
  result->first_residual_gpu_ns = prom_m48_duration_ns(
      state, timestamps, PROM_M45_QUERY_RESIDUAL_BEGIN, PROM_M45_QUERY_RESIDUAL_END);
  result->rmsnorm_gpu_ns = prom_m48_duration_ns(
      state, timestamps, PROM_M46_QUERY_REDUCTION_BEGIN, PROM_M46_QUERY_APPLY_END);
  result->gate_projection_gpu_ns = prom_m48_duration_ns(
      state, timestamps, PROM_M47_QUERY_GATE_BEGIN, PROM_M47_QUERY_GATE_END);
  result->up_projection_gpu_ns = prom_m48_duration_ns(
      state, timestamps, PROM_M47_QUERY_UP_BEGIN, PROM_M47_QUERY_UP_END);
  result->gating_gpu_ns = prom_m48_duration_ns(
      state, timestamps, PROM_M47_QUERY_ACTIVATION_BEGIN, PROM_M47_QUERY_MULTIPLY_END);
  result->down_projection_gpu_ns = prom_m48_duration_ns(
      state, timestamps, PROM_M47_QUERY_DOWN_BEGIN, PROM_M47_QUERY_DOWN_END);
  result->second_residual_gpu_ns = prom_m48_duration_ns(
      state, timestamps, PROM_M47_QUERY_RESIDUAL_BEGIN, PROM_M47_QUERY_RESIDUAL_END);
  result->total_gpu_ns = prom_m48_duration_ns(state, timestamps, 3u,
                                               PROM_M47_QUERY_RESIDUAL_END);
  result->replay_id = plan->replay_id;
  result->input_generation = plan->input_generation;
  result->output_generation = plan->output_generation;
}

int prom_reactor_runtime_m49b_set_parameters(
    void* handle, const prom_num_m49b_parameters* parameters,
    uint64_t* out_parameter_generation) {
  prom_reduction_runtime_state* state;
  int32_t detail = 0;
  if (out_parameter_generation != NULL) *out_parameter_generation = 0u;
  if (!prom_num_m49b_validate_parameters(parameters)) return PROM_ERROR;
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL || !prom_m40b_wait_all_slots(state)) return PROM_ERROR;
  if (state->m49b_enabled == 0u) {
    prom_num_m49b_init(&state->m49b_controller);
    state->m49b_enabled = 1u;
  }
  if (!prom_num_m49b_update_parameters(&state->m49b_controller, parameters)) return PROM_ERROR;
  if (out_parameter_generation != NULL)
    *out_parameter_generation = state->m49b_controller.parameter_generation;
  return PROM_OK;
}

int prom_reactor_runtime_m49b_reset(void* handle) {
  prom_reduction_runtime_state* state;
  int32_t detail = 0;
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL || !prom_m40b_wait_all_slots(state)) return PROM_ERROR;
  prom_num_m49b_init(&state->m49b_controller);
  state->m49b_enabled = 1u;
  state->m49b_next_execution_index = 0u;
  return PROM_OK;
}

int prom_reactor_runtime_m48_execute_stack(void* handle,
                                           const prom_m48_stack_request* request,
                                           prom_m48_stack_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_m48_stack_request effective_request;
  prom_reduction_slot* slot = NULL;
  prom_vk_runtime_services services;
  prom_m48_plan_request plan_request;
  prom_transformer_recorded_block block[PROM_M48_LAYER_COUNT];
  prom_transformer_descriptor_bank standalone_bank;
  VkCommandBufferBeginInfo begin_info;
  VkSubmitInfo submits[PROM_M48_LAYER_COUNT];
  VkPipelineStageFlags wait_stages[PROM_M48_LAYER_COUNT];
  uint64_t timestamps[PROM_M48_LAYER_COUNT][PROM_M48_QUERY_COUNT_PER_LAYER];
  const prom_vk_buffer* initial;
  const prom_vk_buffer* input;
  prom_vk_buffer* output;
  uint64_t initial_generation;
  uint64_t initial_hash;
  uint64_t elements;
  uint64_t begin_ns;
  uint64_t recording_begin;
  uint64_t submission_begin;
  uint64_t allocations_before;
  uint64_t reuses_before;
  uint64_t descriptors_before;
  uint64_t pipelines_before;
  VkDeviceSize logical_bytes;
  uint32_t initial_stride;
  uint32_t finite = 0u;
  uint32_t layer;
  uint32_t resource;
  uint32_t fault_stage = 0u;
  uint32_t command_count;
  uint32_t optional_readback;
  uint32_t stage_audit;
  uint32_t pipeline_path;
  uint32_t pipeline_layer;
  uint32_t host_wait_audit;
  uint32_t host_bounce_audit;
  uint32_t m49b_enabled = 0u;
  uint32_t m49b_canary_due = 0u;
  uint32_t m49b_internal_witness = 0u;
  uint32_t m49b_witness_failed = 0u;
  uint32_t m49b_state_before = PROM_NUM_M49B_UNIDENTIFIED;
  uint64_t m49b_execution_identity = 0u;
  prom_m48_stack_request m49b_witness_request;
  prom_m48_stack_result m49b_witness_result;
  prom_m49b_paired_estimate m49b_paired_estimate;
  int32_t detail = 0;
  VkResult vk_result;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  begin_ns = prom_reduction_now_ns();
  if (request == NULL || request->layer_count == 0u ||
      request->layer_count > PROM_M48_LAYER_COUNT ||
      (request->audit_mode == 0u && request->layer_count != PROM_M48_LAYER_COUNT) ||
      request->head_count != PROM_M43_HEAD_COUNT || request->tokens == 0u ||
      request->model_width == 0u || request->head_dim == 0u ||
      request->ffn_width == 0u || request->expected_initial_generation == 0u ||
      !isfinite(request->epsilon) || request->epsilon <= 0.0f ||
      !prom_m40b_checked_product_u64(request->tokens, request->model_width, &elements) ||
      elements > SIZE_MAX / sizeof(float) ||
      (request->output != NULL && request->output_element_count != elements) ||
      (request->output == NULL && request->output_element_count != 0u) ||
      (request->audit_stage_output != NULL &&
       (request->audit_mode == 0u || request->output != NULL ||
        request->audit_stage_output_element_count != elements ||
        request->audit_stage < PROM_M48_AUDIT_STAGE_ATTENTION ||
        request->audit_stage > PROM_M48_AUDIT_STAGE_FFN)) ||
      (request->audit_stage_output == NULL &&
       (request->audit_stage_output_element_count != 0u ||
        request->audit_stage != PROM_M48_AUDIT_STAGE_NONE)) ||
      request->numerical_witness_mode > 1u) {
    out_result->detail_code = PROM_M48_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  logical_bytes = (VkDeviceSize)(elements * sizeof(float));
  optional_readback = request->output != NULL ? 1u : 0u;
  stage_audit = request->audit_stage_output != NULL ? 1u : 0u;
  host_wait_audit = request->submit_topology == PROM_M48_SUBMIT_HOST_WAIT_PER_LAYER_AUDIT;
  host_bounce_audit = request->submit_topology == PROM_M48_SUBMIT_HOST_BOUNCE_PER_LAYER_AUDIT;
  if (host_bounce_audit != 0u &&
      (request->initial_activation_mode != PROM_M48_INITIAL_HOST || optional_readback == 0u)) {
    out_result->detail_code = PROM_M48_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL || prom_reactor_runtime_get_vk_services(handle, &services) != PROM_OK) {
    out_result->detail_code = state == NULL ? detail : PROM_M48_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  m49b_enabled = state->m49b_enabled;
  if (m49b_enabled != 0u) {
    effective_request = *request;
    request = &effective_request;
    m49b_internal_witness = request->numerical_witness_mode;
    m49b_execution_identity = m49b_internal_witness != 0u
                                  ? request->controller_execution_identity
                                  : ++state->m49b_next_execution_index;
    m49b_state_before = state->m49b_controller.state;
    if (m49b_internal_witness == 0u) {
      prom_m49b_apply_fixed_stack_policy(&state->m49b_controller, &effective_request,
                                         m49b_execution_identity);
      m49b_canary_due = prom_m49b_canary_due(&state->m49b_controller,
                                              m49b_execution_identity);
    } else {
      /* Internal witnesses are pinned to the selected request identity and
         always return the compact evidence capture.  They do not consume a
         cadence slot or mutate controller state. */
      m49b_canary_due = 1u;
    }
  }
  pipeline_path = request->projection_path;
  if (pipeline_path == PROM_M47_PROJECTION_COOPERATIVE &&
      (services.cooperative_matrix_feature_enabled == 0u || services.subgroup_size != 32u))
    pipeline_path = PROM_M47_PROJECTION_CONVENTIONAL_FP16;
  memset(&plan_request, 0, sizeof(plan_request));
  plan_request.host_initial_activation = request->host_initial_activation;
  plan_request.host_initial_element_count = request->host_initial_element_count;
  plan_request.initial_activation_mode = request->initial_activation_mode;
  plan_request.initial_activation_exclusive = 1u;
  plan_request.layer_count = request->layer_count;
  plan_request.audit_mode = request->audit_mode;
  plan_request.tokens = request->tokens;
  plan_request.model_width = request->model_width;
  plan_request.head_count = request->head_count;
  plan_request.head_dim = request->head_dim;
  plan_request.ffn_width = request->ffn_width;
  plan_request.precision_policy = request->precision_policy;
  plan_request.projection_path = request->projection_path;
  memcpy(plan_request.audit_layer_projection_path,
         request->audit_layer_projection_path,
         sizeof(plan_request.audit_layer_projection_path));
  plan_request.numerical_control_mode = request->numerical_control_mode;
  memcpy(plan_request.controller_layer_projection_path,
         request->controller_layer_projection_path,
         sizeof(plan_request.controller_layer_projection_path));
  plan_request.controller_parameter_generation = request->controller_parameter_generation;
  plan_request.controller_execution_identity = request->controller_execution_identity;
  plan_request.numerical_witness_mode = request->numerical_witness_mode;
  plan_request.attention_strategy = request->attention_strategy;
  plan_request.output_projection_strategy = request->output_projection_strategy;
  plan_request.rmsnorm_strategy = request->rmsnorm_strategy;
  plan_request.gating_strategy = request->gating_strategy;
  plan_request.residual_strategy = request->residual_strategy;
  plan_request.activation_strategy = PROM_M48_ACTIVATION_PING_PONG;
  plan_request.submit_topology = request->submit_topology;
  plan_request.optional_final_readback = optional_readback;
  plan_request.audit_stage = request->audit_stage;
  plan_request.expected_initial_generation = request->expected_initial_generation;
  if (request->initial_activation_mode == PROM_M48_INITIAL_HOST) {
    if (request->host_initial_activation == NULL ||
        request->host_initial_element_count != elements) {
      out_result->detail_code = PROM_M48_DETAIL_INVALID_REQUEST;
      return PROM_ERROR;
    }
    initial_hash = prom_m42_hash_finite_matrix(request->host_initial_activation,
                                               elements, &finite);
    if (finite == 0u) {
      out_result->detail_code = PROM_M48_DETAIL_NONFINITE_INPUT;
      return PROM_ERROR;
    }
  } else if (request->initial_activation_mode == PROM_M48_INITIAL_RESIDENT) {
    if (state->m48_initial_generation != request->expected_initial_generation ||
        state->m48_initial_hash == 0u || state->m48_initial_tokens != request->tokens ||
        state->m48_initial_model_width != request->model_width) {
      out_result->detail_code = PROM_M48_DETAIL_STALE_INITIAL_GENERATION;
      return PROM_ERROR;
    }
    initial_hash = state->m48_initial_hash;
  } else {
    out_result->detail_code = PROM_M48_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  plan_request.initial_content_hash = initial_hash;
  initial_generation = request->expected_initial_generation;
  for (layer = 0u; layer < request->layer_count; ++layer) {
    if (!prom_transformer_validate_layer_resources(&state->m48_layer[layer], request, layer)) {
      out_result->detail_code = PROM_M48_DETAIL_STALE_WEIGHT_GENERATION;
      return PROM_ERROR;
    }
    for (resource = 0u; resource < PROM_M48_RESOURCE_COUNT; ++resource) {
      plan_request.layer[layer].generation[resource] =
          request->required_generation[layer][resource];
      if (resource < PROM_M48_ATTENTION_RESOURCE_COUNT) {
        plan_request.layer[layer].content_hash[resource] =
            state->m48_layer[layer]
                .attention[resource / PROM_M43_WEIGHT_KIND_COUNT]
                          [resource % PROM_M43_WEIGHT_KIND_COUNT].hash;
      } else if (resource == PROM_M48_RESOURCE_WO) {
        plan_request.layer[layer].content_hash[resource] = state->m48_layer[layer].wo.hash;
      } else if (resource == PROM_M48_RESOURCE_RMSNORM) {
        plan_request.layer[layer].content_hash[resource] = state->m48_layer[layer].rmsnorm.hash;
      } else {
        plan_request.layer[layer].content_hash[resource] =
            state->m48_layer[layer].ffn[resource - PROM_M48_RESOURCE_WGATE].hash;
      }
    }
  }
  if (request->initial_activation_mode == PROM_M48_INITIAL_RESIDENT) {
    memset(&plan_request.resident_initial_activation, 0,
           sizeof(plan_request.resident_initial_activation));
    plan_request.resident_initial_activation.buffer = state->m48_initial_f32.buffer;
    plan_request.resident_initial_activation.byte_length = state->m48_initial_f32.size;
    plan_request.resident_initial_activation.element_type = PROM_DEVICE_ELEMENT_F32;
    plan_request.resident_initial_activation.logical_rows = request->tokens;
    plan_request.resident_initial_activation.logical_columns = request->model_width;
    plan_request.resident_initial_activation.row_stride_elements = request->model_width;
    plan_request.resident_initial_activation.layout = PROM_DEVICE_LAYOUT_ROW_MAJOR;
    plan_request.resident_initial_activation.producer_access = PROM_DEVICE_ACCESS_COMPUTE_WRITE;
    plan_request.resident_initial_activation.required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
    plan_request.resident_initial_activation.owning_device = state->device;
    plan_request.resident_initial_activation.owning_lifetime_id = initial_generation;
    plan_request.resident_initial_activation.owning_slot_generation = 1u;
  }
  if (prom_m48_transformer_stack_plan_build(&plan_request, &out_result->plan) != PROM_OK) {
    out_result->detail_code = out_result->plan.eligibility_reason == PROM_M48_INELIGIBLE_CAPACITY
                                  ? PROM_M48_DETAIL_CAPACITY
                                  : PROM_M48_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  if (!prom_m42_ensure_pipelines(state) || !prom_m44_ensure_pipelines(state) ||
      !prom_m45_ensure_pipeline(state) || !prom_m46_ensure_pipelines(state) ||
      !prom_m47_ensure_pipelines(state) ||
      !prom_m40b_ensure_sgemm_pipeline(state, pipeline_path)) {
    out_result->detail_code = PROM_M48_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  for (pipeline_layer = 0u; pipeline_layer < request->layer_count; ++pipeline_layer) {
    uint32_t layer_path =
        request->numerical_control_mode == PROM_M48_NUMERICAL_CONTROL_M49B &&
                request->controller_layer_projection_path[pipeline_layer] != 0u
            ? request->controller_layer_projection_path[pipeline_layer]
            : request->audit_layer_projection_path[pipeline_layer] != 0u
                  ? request->audit_layer_projection_path[pipeline_layer]
                  : pipeline_path;
    if (layer_path == PROM_M47_PROJECTION_COOPERATIVE &&
        (services.cooperative_matrix_feature_enabled == 0u || services.subgroup_size != 32u))
      layer_path = PROM_M47_PROJECTION_CONVENTIONAL_FP16;
    if (!prom_m40b_ensure_sgemm_pipeline(state, layer_path)) {
      out_result->detail_code = PROM_M48_DETAIL_RESOURCE;
      return PROM_ERROR;
    }
  }
  slot = prom_reduction_acquire_slot(state, state->next_logical_request_id++);
  if (slot == NULL) {
    out_result->detail_code = PROM_M48_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  out_result->logical_stack_id = slot->logical_request_id;
  out_result->physical_slot_id = slot->slot_id;
  out_result->physical_slot_generation = slot->generation;
  memcpy(standalone_bank.m43, slot->m43_descriptor_sets, sizeof(standalone_bank.m43));
  standalone_bank.m44_sgemm = slot->m44_sgemm_descriptor_set;
  standalone_bank.m44_wide = slot->m44_descriptor_set;
  standalone_bank.m45 = slot->m45_descriptor_set;
  memcpy(standalone_bank.m46, slot->descriptor_sets, sizeof(standalone_bank.m46));
  memcpy(standalone_bank.m47, slot->m47_descriptor_sets, sizeof(standalone_bank.m47));
  allocations_before = state->diagnostics.buffer_allocation_count;
  reuses_before = state->m48_buffer_reuse_count;
  descriptors_before = state->m48_descriptor_update_count;
  pipelines_before = state->diagnostics.pipeline_create_count;
  if (!prom_m48_ensure_buffer(state, &slot->m48_activation[0],
                              (VkDeviceSize)(out_result->plan.memory.activation_bytes / 2u),
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
      !prom_m48_ensure_buffer(state, &slot->m48_activation[1],
                              (VkDeviceSize)(out_result->plan.memory.activation_bytes / 2u),
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
      ((optional_readback != 0u || stage_audit != 0u) &&
       !prom_m48_ensure_buffer(state, &slot->m48_readback, logical_bytes,
                               VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                               VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT |
                                   VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                               1, NULL)) ||
      (m49b_canary_due != 0u &&
       !prom_m48_ensure_buffer(state, &slot->m49b_canary_readback,
                               (VkDeviceSize)(PROM_NUM_M49B_MAX_SAMPLES * sizeof(float)),
                               VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                               VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT |
                                   VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                               1, NULL))) {
    out_result->detail_code = PROM_M48_DETAIL_RESOURCE;
    goto known_fail;
  }
  if (request->initial_activation_mode == PROM_M48_INITIAL_HOST) {
    if (!prom_m48_ensure_buffer(state, &slot->m48_host_initial_upload, logical_bytes,
                                VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                                VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT |
                                    VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                                1, NULL) ||
        !prom_m48_ensure_buffer(state, &slot->m48_host_initial, logical_bytes,
                                VK_BUFFER_USAGE_TRANSFER_DST_BIT |
                                    VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                                VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL)) {
      out_result->detail_code = PROM_M48_DETAIL_RESOURCE;
      goto known_fail;
    }
    memcpy(slot->m48_host_initial_upload.mapped, request->host_initial_activation,
           (size_t)logical_bytes);
    initial = &slot->m48_host_initial;
    initial_stride = request->model_width;
  } else {
    initial = &state->m48_initial_f32;
    initial_stride = request->model_width;
  }
  input = initial;
  for (layer = 0u; layer < request->layer_count; ++layer) {
    if (host_bounce_audit != 0u && layer != 0u) input = &slot->m48_host_initial;
    output = &slot->m48_activation[(layer + 1u) & 1u];
    if (!prom_transformer_prepare_block(state, slot, &services, request, layer,
                                        input,
                                        layer == 0u ? initial_stride
                                                    : block[layer - 1u].ffn_plan.output_row_stride,
                                        out_result->plan.layer[layer].input_generation,
                                        output, &block[layer])) {
      out_result->stage = layer;
      out_result->detail_code = PROM_M48_DETAIL_RESOURCE;
      goto known_fail;
    }
    block[layer].output_view.owning_lifetime_id =
        out_result->plan.layer[layer].output_generation;
    input = output;
  }
  if (request->fault_point == PROM_M48_FAULT_BEFORE_LAYER_0) {
    out_result->detail_code = PROM_M48_DETAIL_FAULT_INJECTED;
    goto known_fail;
  }
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  command_count = request->submit_topology == PROM_M48_SUBMIT_ONE_STACK
                      ? 1u : request->layer_count;
  recording_begin = prom_reduction_now_ns();
  for (layer = 0u; layer < command_count; ++layer) {
    if (vkResetCommandBuffer(slot->m48_command_buffers[layer], 0u) != VK_SUCCESS ||
        vkBeginCommandBuffer(slot->m48_command_buffers[layer], &begin_info) != VK_SUCCESS) {
      out_result->detail_code = PROM_M48_DETAIL_COMMAND;
      goto known_fail;
    }
    slot->m48_command_reuse_count += 1u;
  }
  if (request->initial_activation_mode == PROM_M48_INITIAL_HOST) {
    const uint32_t upload_command_count = host_bounce_audit != 0u ? command_count : 1u;
    for (layer = 0u; layer < upload_command_count; ++layer) {
      VkBufferMemoryBarrier barrier;
      VkBufferCopy copy;
      VkCommandBuffer command_buffer = slot->m48_command_buffers[layer];
      memset(&barrier, 0, sizeof(barrier));
      barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
      barrier.srcAccessMask = VK_ACCESS_HOST_WRITE_BIT;
      barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
      barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
      barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
      barrier.buffer = slot->m48_host_initial_upload.buffer;
      barrier.size = logical_bytes;
      vkCmdPipelineBarrier(command_buffer, VK_PIPELINE_STAGE_HOST_BIT,
                           VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 1u,
                           &barrier, 0u, NULL);
      memset(&copy, 0, sizeof(copy));
      copy.size = logical_bytes;
      vkCmdCopyBuffer(command_buffer, slot->m48_host_initial_upload.buffer,
                      slot->m48_host_initial.buffer, 1u, &copy);
      if (!prom_m43_one_buffer_barrier(command_buffer, &slot->m48_host_initial,
                                       logical_bytes, VK_ACCESS_TRANSFER_WRITE_BIT,
                                       VK_ACCESS_SHADER_READ_BIT,
                                       VK_PIPELINE_STAGE_TRANSFER_BIT,
                                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT)) {
        out_result->detail_code = PROM_M48_DETAIL_COMMAND;
        goto known_fail;
      }
    }
  }
  for (layer = 0u; layer < request->layer_count; ++layer) {
    prom_transformer_record_context context;
    const uint32_t command_index = request->submit_topology == PROM_M48_SUBMIT_ONE_STACK
                                       ? 0u : layer;
    context.command_buffer = slot->m48_command_buffers[command_index];
    context.query_base = slot->slot_id * PROM_REDUCTION_QUERY_STRIDE +
                         layer * PROM_M48_QUERY_COUNT_PER_LAYER;
    context.query_count = PROM_M48_QUERY_COUNT_PER_LAYER;
    context.layer_index = layer;
    context.audit_stage = stage_audit != 0u && layer + 1u == request->layer_count
                              ? request->audit_stage : PROM_M48_AUDIT_STAGE_NONE;
    context.audit_readback = context.audit_stage != PROM_M48_AUDIT_STAGE_NONE
                                 ? &slot->m48_readback : NULL;
    context.descriptors = &slot->m48_descriptors[layer];
    if (!prom_transformer_record_block(state, slot, &context, &block[layer],
                                       &fault_stage)) {
      out_result->stage = layer;
      out_result->detail_code = request->fault_point != PROM_M48_FAULT_NONE
                                    ? PROM_M48_DETAIL_FAULT_INJECTED
                                    : PROM_M48_DETAIL_COMMAND;
      goto known_fail;
    }
    if (host_bounce_audit != 0u && layer + 1u < request->layer_count) {
      VkBufferCopy copy;
      uint32_t row;
      if (!prom_m43_one_buffer_barrier(context.command_buffer, block[layer].output,
                                       block[layer].output->size,
                                       VK_ACCESS_SHADER_WRITE_BIT,
                                       VK_ACCESS_TRANSFER_READ_BIT,
                                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                       VK_PIPELINE_STAGE_TRANSFER_BIT)) {
        out_result->detail_code = PROM_M48_DETAIL_COMMAND;
        goto known_fail;
      }
      for (row = 0u; row < request->tokens; ++row) {
        memset(&copy, 0, sizeof(copy));
        copy.srcOffset = (VkDeviceSize)((uint64_t)row *
            block[layer].ffn_plan.output_row_stride * sizeof(float));
        copy.dstOffset = (VkDeviceSize)((uint64_t)row * request->model_width * sizeof(float));
        copy.size = (VkDeviceSize)((uint64_t)request->model_width * sizeof(float));
        vkCmdCopyBuffer(context.command_buffer, block[layer].output->buffer,
                        slot->m48_readback.buffer, 1u, &copy);
      }
      if (!prom_m43_one_buffer_barrier(context.command_buffer, &slot->m48_readback,
                                       logical_bytes, VK_ACCESS_TRANSFER_WRITE_BIT,
                                       VK_ACCESS_HOST_READ_BIT,
                                       VK_PIPELINE_STAGE_TRANSFER_BIT,
                                       VK_PIPELINE_STAGE_HOST_BIT)) {
        out_result->detail_code = PROM_M48_DETAIL_COMMAND;
        goto known_fail;
      }
    }
    out_result->completed_layer_count = layer + 1u;
    if ((request->fault_point == PROM_M48_FAULT_AFTER_LAYER_0_OUTPUT && layer == 0u) ||
        (request->fault_point == PROM_M48_FAULT_AFTER_LAYER_2_OUTPUT && layer == 2u)) {
      out_result->stage = layer;
      out_result->detail_code = PROM_M48_DETAIL_FAULT_INJECTED;
      goto known_fail;
    }
  }
  if (request->fault_point == PROM_M48_FAULT_AFTER_FINAL_OUTPUT ||
      request->fault_point == PROM_M48_FAULT_BEFORE_FINAL_READBACK) {
    out_result->stage = request->layer_count - 1u;
    out_result->detail_code = PROM_M48_DETAIL_FAULT_INJECTED;
    goto known_fail;
  }
  if (optional_readback != 0u) {
    VkCommandBuffer command_buffer = slot->m48_command_buffers[command_count - 1u];
    uint32_t row;
    prom_m42_write_timestamp(state, slot, command_buffer,
                             VK_PIPELINE_STAGE_TRANSFER_BIT,
                             PROM_M47_QUERY_READBACK_BEGIN);
    if (!prom_m43_one_buffer_barrier(command_buffer, block[request->layer_count - 1u].output,
                                     block[request->layer_count - 1u].output->size,
                                     VK_ACCESS_SHADER_WRITE_BIT,
                                     VK_ACCESS_TRANSFER_READ_BIT,
                                     VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                     VK_PIPELINE_STAGE_TRANSFER_BIT)) {
      out_result->detail_code = PROM_M48_DETAIL_COMMAND;
      goto known_fail;
    }
    for (row = 0u; row < request->tokens; ++row) {
      VkBufferCopy copy;
      memset(&copy, 0, sizeof(copy));
      copy.srcOffset = (VkDeviceSize)((uint64_t)row *
          block[request->layer_count - 1u].ffn_plan.output_row_stride * sizeof(float));
      copy.dstOffset = (VkDeviceSize)((uint64_t)row * request->model_width * sizeof(float));
      copy.size = (VkDeviceSize)((uint64_t)request->model_width * sizeof(float));
      vkCmdCopyBuffer(command_buffer, block[request->layer_count - 1u].output->buffer,
                      slot->m48_readback.buffer, 1u, &copy);
    }
    prom_m42_write_timestamp(state, slot, command_buffer,
                             VK_PIPELINE_STAGE_TRANSFER_BIT,
                             PROM_M47_QUERY_READBACK_END);
    if (!prom_m43_one_buffer_barrier(command_buffer, &slot->m48_readback,
                                     logical_bytes, VK_ACCESS_TRANSFER_WRITE_BIT,
                                     VK_ACCESS_HOST_READ_BIT,
                                     VK_PIPELINE_STAGE_TRANSFER_BIT,
                                     VK_PIPELINE_STAGE_HOST_BIT)) {
      out_result->detail_code = PROM_M48_DETAIL_COMMAND;
      goto known_fail;
    }
  }
  if (m49b_canary_due != 0u) {
    VkCommandBuffer command_buffer = slot->m48_command_buffers[command_count - 1u];
    if (!prom_m49b_record_stack_canary(
            command_buffer, &block[request->layer_count - 1u], request, slot,
            m49b_execution_identity, optional_readback != 0u ? 1u : 0u)) {
      out_result->detail_code = PROM_M48_DETAIL_COMMAND;
      goto known_fail;
    }
  }
  for (layer = 0u; layer < command_count; ++layer) {
    if (vkEndCommandBuffer(slot->m48_command_buffers[layer]) != VK_SUCCESS) {
      out_result->detail_code = PROM_M48_DETAIL_COMMAND;
      goto known_fail;
    }
  }
  out_result->cpu_recording_ns = prom_reduction_elapsed_ns(recording_begin,
                                                            prom_reduction_now_ns());
  if (vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS) {
    out_result->detail_code = PROM_M48_DETAIL_SUBMIT;
    goto known_fail;
  }
  memset(submits, 0, sizeof(submits));
  memset(wait_stages, 0, sizeof(wait_stages));
  for (layer = 0u; layer < command_count; ++layer) {
    wait_stages[layer] = VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT;
    submits[layer].sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
    submits[layer].commandBufferCount = 1u;
    submits[layer].pCommandBuffers = &slot->m48_command_buffers[layer];
    if (layer != 0u && host_wait_audit == 0u && host_bounce_audit == 0u) {
      submits[layer].waitSemaphoreCount = 1u;
      submits[layer].pWaitSemaphores = &slot->m48_semaphores[layer - 1u];
      submits[layer].pWaitDstStageMask = &wait_stages[layer];
    }
    if (layer + 1u < command_count && host_wait_audit == 0u && host_bounce_audit == 0u) {
      submits[layer].signalSemaphoreCount = 1u;
      submits[layer].pSignalSemaphores = &slot->m48_semaphores[layer];
    }
  }
  submission_begin = prom_reduction_now_ns();
  if (host_wait_audit != 0u || host_bounce_audit != 0u) {
    for (layer = 0u; layer < command_count; ++layer) {
      uint64_t wait_begin;
      if (layer != 0u && vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS) {
        out_result->detail_code = PROM_M48_DETAIL_SUBMIT;
        goto known_fail;
      }
      vk_result = vkQueueSubmit(state->queue, 1u, &submits[layer], slot->fence);
      if (vk_result != VK_SUCCESS) {
        out_result->detail_code = PROM_M48_DETAIL_SUBMIT;
        goto known_fail;
      }
      state->diagnostics.queue_submit_count += 1u;
      slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
      wait_begin = prom_reduction_now_ns();
      vk_result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
      out_result->cpu_wait_ns += prom_reduction_elapsed_ns(wait_begin,
                                                            prom_reduction_now_ns());
      if (vk_result != VK_SUCCESS) {
        slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
        state->diagnostics.quarantine_count += 1u;
        if (m49b_enabled != 0u)
          prom_m49b_quarantine_execution(&state->m49b_controller, request,
                                         m49b_execution_identity);
        out_result->detail_code = PROM_M48_DETAIL_COMPLETION_UNCERTAIN;
        prom_transformer_select_descriptor_bank(slot, &standalone_bank);
        return PROM_ERROR;
      }
      if (host_bounce_audit != 0u && layer + 1u < command_count) {
        uint64_t bounce_copy_begin = prom_reduction_now_ns();
        memcpy(slot->m48_host_initial_upload.mapped, slot->m48_readback.mapped,
               (size_t)logical_bytes);
        out_result->host_bounce_copy_ns +=
            prom_reduction_elapsed_ns(bounce_copy_begin, prom_reduction_now_ns());
      }
    }
  } else {
    vk_result = vkQueueSubmit(state->queue, command_count, submits, slot->fence);
    if (vk_result != VK_SUCCESS) {
      out_result->detail_code = PROM_M48_DETAIL_SUBMIT;
      goto known_fail;
    }
    slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
    state->diagnostics.queue_submit_count += command_count;
  }
  out_result->cpu_submission_ns = prom_reduction_elapsed_ns(submission_begin,
                                                             prom_reduction_now_ns());
  if (request->fault_point == PROM_M48_FAULT_UNCERTAIN_COMPLETION) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    if (m49b_enabled != 0u)
      prom_m49b_quarantine_execution(&state->m49b_controller, request,
                                     m49b_execution_identity);
    out_result->stage = request->layer_count - 1u;
    out_result->detail_code = PROM_M48_DETAIL_COMPLETION_UNCERTAIN;
    prom_transformer_select_descriptor_bank(slot, &standalone_bank);
    return PROM_ERROR;
  }
  if (host_wait_audit == 0u && host_bounce_audit == 0u) {
    uint64_t wait_begin = prom_reduction_now_ns();
    vk_result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
    out_result->cpu_wait_ns = prom_reduction_elapsed_ns(wait_begin,
                                                         prom_reduction_now_ns());
    if (vk_result != VK_SUCCESS) {
      slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
      state->diagnostics.quarantine_count += 1u;
      if (m49b_enabled != 0u)
        prom_m49b_quarantine_execution(&state->m49b_controller, request,
                                       m49b_execution_identity);
      out_result->detail_code = PROM_M48_DETAIL_COMPLETION_UNCERTAIN;
      prom_transformer_select_descriptor_bank(slot, &standalone_bank);
      return PROM_ERROR;
    }
  }
  memset(timestamps, 0, sizeof(timestamps));
  if (state->timestamp_supported == 0u || state->query_pool == VK_NULL_HANDLE) {
    out_result->detail_code = PROM_M48_DETAIL_QUERY;
    goto completed_fail;
  }
  for (layer = 0u; layer < request->layer_count; ++layer) {
    const uint32_t query_base = slot->slot_id * PROM_REDUCTION_QUERY_STRIDE +
                                layer * PROM_M48_QUERY_COUNT_PER_LAYER;
    if (vkGetQueryPoolResults(state->device, state->query_pool, query_base,
                              PROM_M43_QUERY_GROUP_END + 1u,
                              sizeof(uint64_t) * (PROM_M43_QUERY_GROUP_END + 1u),
                              timestamps[layer], sizeof(uint64_t),
                              VK_QUERY_RESULT_64_BIT) != VK_SUCCESS ||
        vkGetQueryPoolResults(state->device, state->query_pool,
                              query_base + PROM_M44_QUERY_BASE, 4u,
                              sizeof(uint64_t) * 4u,
                              timestamps[layer] + PROM_M44_QUERY_BASE,
                              sizeof(uint64_t), VK_QUERY_RESULT_64_BIT) != VK_SUCCESS ||
        vkGetQueryPoolResults(state->device, state->query_pool,
                              query_base + PROM_M45_QUERY_BASE, 2u,
                              sizeof(uint64_t) * 2u,
                              timestamps[layer] + PROM_M45_QUERY_BASE,
                              sizeof(uint64_t), VK_QUERY_RESULT_64_BIT) != VK_SUCCESS ||
        vkGetQueryPoolResults(state->device, state->query_pool,
                              query_base + PROM_M46_QUERY_BASE, 5u,
                              sizeof(uint64_t) * 5u,
                              timestamps[layer] + PROM_M46_QUERY_BASE,
                              sizeof(uint64_t), VK_QUERY_RESULT_64_BIT) != VK_SUCCESS ||
        vkGetQueryPoolResults(state->device, state->query_pool,
                              query_base + PROM_M47_QUERY_BASE,
                              (optional_readback != 0u && layer + 1u == request->layer_count)
                                  ? PROM_M47_QUERY_COUNT : 16u,
                              sizeof(uint64_t) *
                                  ((optional_readback != 0u && layer + 1u == request->layer_count)
                                       ? PROM_M47_QUERY_COUNT : 16u),
                              timestamps[layer] + PROM_M47_QUERY_BASE,
                              sizeof(uint64_t), VK_QUERY_RESULT_64_BIT) != VK_SUCCESS) {
      out_result->stage = layer;
      out_result->detail_code = PROM_M48_DETAIL_QUERY;
      goto completed_fail;
    }
    prom_m48_fill_layer_timings(state, timestamps[layer], &block[layer],
                                &out_result->plan.layer[layer],
                                &out_result->layer[layer]);
    out_result->total_stack_gpu_ns += out_result->layer[layer].total_gpu_ns;
    out_result->dispatch_count += out_result->layer[layer].dispatch_count;
  }
  if (optional_readback != 0u) {
    uint64_t readback_begin = prom_reduction_now_ns();
    memcpy(request->output, slot->m48_readback.mapped, (size_t)logical_bytes);
    out_result->final_readback_ns = prom_reduction_elapsed_ns(readback_begin,
                                                               prom_reduction_now_ns());
    if (!prom_m42_finite_matrix(request->output, elements)) {
      out_result->detail_code = PROM_M48_DETAIL_MISMATCH;
      goto completed_fail;
    }
  }
  if (stage_audit != 0u) {
    memcpy(request->audit_stage_output, slot->m48_readback.mapped,
           (size_t)logical_bytes);
    if (!prom_m42_finite_matrix(request->audit_stage_output, elements)) {
      out_result->detail_code = PROM_M48_DETAIL_MISMATCH;
      goto completed_fail;
    }
  }
  if (m49b_enabled != 0u && m49b_canary_due != 0u) {
    memcpy(out_result->numerical_canary_samples, slot->m49b_canary_readback.mapped,
           sizeof(out_result->numerical_canary_samples));
    out_result->numerical_canary_identity = prom_m40b_hash_u64(
        prom_num_hash_float_bits(out_result->numerical_canary_samples,
                                 PROM_NUM_M49B_MAX_SAMPLES),
        out_result->plan.final_output_generation);
    out_result->numerical_canary_identity = prom_m40b_hash_u64(
        out_result->numerical_canary_identity, m49b_execution_identity);
    out_result->numerical_canary_ran = 1u;
  }
  if (m49b_enabled != 0u && m49b_canary_due != 0u && m49b_internal_witness == 0u) {
    prom_num_m49b_observation observation;
    prom_num_m49b_evidence evidence;
    prom_num_m49b_decision decision;
    uint64_t observation_begin;
    memset(&m49b_paired_estimate, 0, sizeof(m49b_paired_estimate));
    memset(&m49b_witness_result, 0, sizeof(m49b_witness_result));
    if (state->ring_depth >= 2u && request->audit_mode == 0u) {
      uint32_t witness_layer;
      m49b_witness_request = *request;
      m49b_witness_request.audit_mode = 0u;
      m49b_witness_request.audit_stage = PROM_M48_AUDIT_STAGE_NONE;
      m49b_witness_request.audit_stage_output = NULL;
      m49b_witness_request.audit_stage_output_element_count = 0u;
      m49b_witness_request.output = NULL;
      m49b_witness_request.output_element_count = 0u;
      m49b_witness_request.precision_policy = PROM_M42_PRECISION_FP32;
      m49b_witness_request.projection_path = PROM_M47_PROJECTION_A2X4_FP32;
      m49b_witness_request.gating_strategy = PROM_M47_GATING_FUSED_FP32;
      m49b_witness_request.submit_topology = PROM_M48_SUBMIT_ONE_STACK;
      m49b_witness_request.numerical_control_mode = PROM_M48_NUMERICAL_CONTROL_M49B;
      m49b_witness_request.controller_parameter_generation =
          state->m49b_controller.parameter_generation;
      m49b_witness_request.controller_execution_identity = m49b_execution_identity;
      m49b_witness_request.numerical_witness_mode = 1u;
      for (witness_layer = 0u; witness_layer < request->layer_count; ++witness_layer)
        m49b_witness_request.controller_layer_projection_path[witness_layer] =
            PROM_M47_PROJECTION_A2X4_FP32;
      if (prom_reactor_runtime_m48_execute_stack(handle, &m49b_witness_request,
                                                  &m49b_witness_result) != PROM_OK ||
          m49b_witness_result.numerical_canary_ran == 0u ||
          !prom_m49b_estimate_paired_discrepancy(
              out_result->numerical_canary_samples,
              m49b_witness_result.numerical_canary_samples,
              PROM_NUM_M49B_MAX_SAMPLES, prom_m49b_shape_identity(request),
              out_result->plan.replay_id, m49b_witness_result.plan.replay_id,
              state->m49b_controller.parameter_generation, &m49b_paired_estimate)) {
        m49b_witness_failed = 1u;
      } else {
        out_result->numerical_witness_ran = 1u;
        out_result->numerical_witness_replay_id = m49b_witness_result.replay_id;
        out_result->numerical_witness_gpu_ns = m49b_witness_result.total_stack_gpu_ns;
        out_result->numerical_witness_end_to_end_ns = m49b_witness_result.end_to_end_ns;
        out_result->numerical_witness_confidence =
            (float)m49b_paired_estimate.confidence;
      }
    } else {
      /* A single-slot configuration cannot retain the selected output while
         recording the independent witness.  Do not recycle it early or make
         an unsafe comparison; the controller will request bounded audit. */
      m49b_witness_failed = 1u;
    }
    /* Keep the cheap observer measurement separate from the explicitly
       reported witness execution.  The witness's GPU and end-to-end time live
       in their own result fields. */
    observation_begin = prom_reduction_now_ns();
    memset(&observation, 0, sizeof(observation));
    observation.completion_known = 1u;
    observation.current_path = prom_m49b_num_path(
        block[request->layer_count - 1u].ffn_plan.projection_path);
    observation.execution_index = m49b_execution_identity;
    observation.shape_identity = prom_m49b_shape_identity(request);
    observation.output_replay_identity = out_result->plan.replay_id;
    observation.reference_identity = m49b_paired_estimate.paired_identity;
    observation.sampled_values = m49b_witness_failed == 0u
                                    ? m49b_paired_estimate.sample_delta
                                    : out_result->numerical_canary_samples;
    observation.sampled_value_count = PROM_NUM_M49B_MAX_SAMPLES;
    observation.observed_l2_error = m49b_paired_estimate.sampled_l2_error;
    observation.observed_linf_error = m49b_paired_estimate.sampled_linf_error;
    observation.observed_gain = m49b_paired_estimate.estimated_gain;
    observation.confidence = m49b_paired_estimate.confidence;
    observation.reference_suspect = m49b_paired_estimate.reference_suspect;
    observation.force_audit = m49b_witness_failed;
    if (m49b_witness_failed != 0u &&
        state->m49b_controller.state == PROM_NUM_M49B_QUARANTINED) {
      /* An uncertain witness completion already quarantined its owning slot.
         Preserve that transition: no selected-path evidence is accepted until
         reap confirms completion. */
      out_result->numerical_state_before = m49b_state_before;
      out_result->numerical_state_after = state->m49b_controller.state;
      out_result->numerical_action = PROM_NUM_M49B_QUARANTINE;
      out_result->numerical_parameter_generation =
          state->m49b_controller.parameter_generation;
      out_result->numerical_controller_identity =
          state->m49b_controller.controller_state_identity;
    } else {
      prom_num_m49b_advance_execution(&state->m49b_controller,
                                      m49b_execution_identity);
      if (!prom_num_m49b_observe(&state->m49b_controller, &observation,
                                  &evidence, &decision)) {
        out_result->detail_code = PROM_M48_DETAIL_MISMATCH;
        goto completed_fail;
      }
      out_result->numerical_state_before = m49b_state_before;
      out_result->numerical_state_after = decision.state_after;
      out_result->numerical_action = decision.action;
      out_result->numerical_parameter_generation = decision.parameter_generation;
      out_result->numerical_evidence_identity = evidence.evidence_identity;
      out_result->numerical_controller_identity = decision.controller_state_identity;
    }
    out_result->numerical_canary_cpu_ns =
        prom_reduction_elapsed_ns(observation_begin, prom_reduction_now_ns());
  } else if (m49b_enabled != 0u && m49b_internal_witness == 0u) {
    /* A cooldown is counted in completed stack executions, not canary
       samples.  This cannot accept stale evidence because it accepts none. */
    prom_num_m49b_advance_execution(&state->m49b_controller,
                                    m49b_execution_identity);
    out_result->numerical_state_before = m49b_state_before;
    out_result->numerical_state_after = state->m49b_controller.state;
    out_result->numerical_parameter_generation =
        state->m49b_controller.parameter_generation;
    out_result->numerical_controller_identity =
        state->m49b_controller.controller_state_identity;
  }
  out_result->submit_count = command_count;
  out_result->semaphore_count = (host_wait_audit != 0u || host_bounce_audit != 0u)
                                    ? 0u : command_count - 1u;
  out_result->fence_count = 1u;
  out_result->final_readback_count = optional_readback;
  out_result->intermediate_host_copy_count =
      (host_bounce_audit != 0u ? request->layer_count - 1u : 0u) + stage_audit;
  out_result->intermediate_readback_count =
      (host_bounce_audit != 0u ? request->layer_count - 1u : 0u) + stage_audit;
  out_result->selected_projection_path = block[0].ffn_plan.projection_path;
  for (layer = 1u; layer < request->layer_count; ++layer) {
    if (block[layer].ffn_plan.projection_path != out_result->selected_projection_path)
      out_result->selected_projection_path = 0u;
  }
  out_result->retained_bytes = out_result->plan.memory.exact_retained_bytes;
  out_result->persistent_weight_bytes = out_result->plan.memory.persistent_weight_bytes;
  out_result->block_working_set_bytes = out_result->plan.memory.one_block_working_set_bytes;
  out_result->activation_bytes = out_result->plan.memory.activation_bytes;
  out_result->buffer_allocation_count = state->diagnostics.buffer_allocation_count - allocations_before;
  out_result->buffer_reuse_count = state->m48_buffer_reuse_count - reuses_before;
  out_result->descriptor_update_count = state->m48_descriptor_update_count - descriptors_before;
  out_result->pipeline_create_count = state->diagnostics.pipeline_create_count - pipelines_before;
  out_result->command_buffer_reuse_count = slot->m48_command_reuse_count;
  out_result->initial_generation = initial_generation;
  out_result->final_output_generation = out_result->plan.final_output_generation;
  out_result->replay_id = out_result->plan.replay_id;
  out_result->output_view = block[request->layer_count - 1u].output_view;
  out_result->output_view.owning_lifetime_id = out_result->plan.final_output_generation;
  out_result->end_to_end_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  if (out_result->numerical_witness_ran != 0u &&
      out_result->end_to_end_ns >= out_result->numerical_witness_end_to_end_ns)
    out_result->end_to_end_ns -= out_result->numerical_witness_end_to_end_ns;
  out_result->physical_slot_recyclable = 1u;
  out_result->stage = 0u;
  out_result->detail_code = 0;
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  prom_transformer_select_descriptor_bank(slot, &standalone_bank);
  return PROM_OK;

completed_fail:
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->physical_slot_recyclable = 1u;
  prom_transformer_select_descriptor_bank(slot, &standalone_bank);
  return PROM_ERROR;

known_fail:
  if (slot != NULL) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    prom_transformer_select_descriptor_bank(slot, &standalone_bank);
  }
  out_result->end_to_end_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  return PROM_ERROR;
}

static int prom_m45_execute_composed_core(void* handle,
                                          const prom_m45_composed_request* request,
                                          prom_m45_composed_result* out_result,
                                          prom_m46_continuation* continuation) {
  prom_reduction_runtime_state* state;
  prom_reduction_slot* slot;
  prom_vk_runtime_services services_before;
  prom_vk_runtime_services services_after;
  prom_m45_composed_request effective_request;
  prom_m43_plan_request m43_plan_request;
  prom_m44_plan_request m44_plan_request;
  prom_m45_plan_request m45_plan_request;
  prom_m44_composed_request projection_request;
  PrometheusReductionRequest reduction_request;
  PrometheusReductionPlan reduction_plan;
  const prom_vk_buffer* x_f32;
  const prom_vk_buffer* x_f16;
  const prom_vk_buffer* z;
  uint64_t timestamps[PROM_M47_QUERY_BASE + PROM_M47_QUERY_COUNT];
  uint64_t begin_ns = prom_reduction_now_ns();
  uint64_t recording_begin;
  uint64_t submission_begin;
  uint64_t readback_begin;
  uint64_t x_elements;
  uint64_t head_output_elements;
  uint64_t y_generation;
  uint32_t head;
  uint32_t weight;
  uint32_t m43_partial_fault = 0u;
  uint32_t m43_uncertain_fault = 0u;
  uint32_t m44_partial_fault = 0u;
  uint32_t m45_partial_fault = 0u;
  uint32_t m46_partial_fault = 0u;
  uint32_t m47_partial_fault = 0u;
  uint32_t wait_stage = VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT;
  uint32_t split_submit = 0u;
  int m43_record_status;
  int m44_record_status;
  int m45_record_status;
  int m46_record_status = 1;
  int m47_record_status = 1;
  int32_t detail = 0;
  VkSubmitInfo submits[2];
  VkResult result;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  out_result->physical_slot_id = UINT32_MAX;
  if (request == NULL || request->attention.head_count != PROM_M44_HEAD_COUNT ||
      request->attention.input_mode != PROM_M42_INPUT_RESIDENT_X ||
      request->attention.host_x != NULL || request->attention.host_x_element_count != 0u ||
      request->attention.output != NULL ||
      request->attention.execution_strategy == PROM_M43_STRATEGY_EIGHT_SEQUENTIAL_M42 ||
      request->attention.fault_point != PROM_M43_FAULT_NONE ||
      request->aggregation_strategy < PROM_M44_AGGREGATION_INTERLEAVE ||
      request->aggregation_strategy > PROM_M44_AGGREGATION_DIRECT_SEGMENTED ||
      request->projection_path < PROM_M44_PROJECTION_COOPERATIVE ||
      request->projection_path > PROM_M44_PROJECTION_DIRECT_SEGMENTED_FP16 ||
      request->residual_strategy < PROM_M45_STRATEGY_SEPARATE_OUTPUT ||
      request->residual_strategy > PROM_M45_STRATEGY_IN_PLACE_X_AUDIT ||
      request->submit_policy < PROM_M45_SUBMIT_ONE_COMMAND_BUFFER ||
      request->submit_policy > PROM_M45_SUBMIT_TWO_BOUNDED ||
      request->fault_point > PROM_M45_FAULT_UNCERTAIN_COMPLETION ||
      request->required_wo_generation == 0u ||
      (request->output == NULL && request->output_element_count != 0u)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = request != NULL &&
                                      request->residual_strategy == PROM_M45_STRATEGY_IN_PLACE_X_AUDIT
                                  ? PROM_M45_DETAIL_IN_PLACE_X_REJECTED
                                  : PROM_M45_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  if (request->residual_strategy == PROM_M45_STRATEGY_IN_PLACE_X_AUDIT) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M45_DETAIL_IN_PLACE_X_REJECTED;
    return PROM_ERROR;
  }
  if (!prom_m40b_checked_product_u64(request->attention.tokens,
                                     request->attention.model_width, &x_elements) ||
      !prom_m40b_checked_product_u64(request->attention.tokens,
                                     request->attention.head_dim, &head_output_elements) ||
      !prom_m43_checked_scale_u64(head_output_elements, PROM_M44_HEAD_COUNT,
                                  &head_output_elements)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M45_DETAIL_SIZE_OVERFLOW;
    return PROM_ERROR;
  }
  if (request->attention.output_element_count != head_output_elements ||
      (request->output != NULL && request->output_element_count != x_elements)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M45_DETAIL_SHAPE;
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL || prom_reactor_runtime_get_vk_services(handle, &services_before) != PROM_OK) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = state == NULL ? detail : PROM_M45_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  if (state->m43_resident_x_generation == 0u ||
      request->attention.shared_x_generation != state->m43_resident_x_generation ||
      state->m43_resident_x_tokens != request->attention.tokens ||
      state->m43_resident_x_model_width != request->attention.model_width) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M45_DETAIL_STALE_X_GENERATION;
    return PROM_ERROR;
  }
  if (state->m44_wo_generation == 0u ||
      request->required_wo_generation != state->m44_wo_generation ||
      state->m44_wo_head_dim != request->attention.head_dim ||
      state->m44_wo_model_width != request->attention.model_width) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M44_DETAIL_STALE_WO_GENERATION;
    return PROM_ERROR;
  }
  effective_request = *request;
  if (continuation != NULL) {
    effective_request.output = NULL;
    effective_request.output_element_count = 0u;
    effective_request.submit_policy = PROM_M45_SUBMIT_ONE_COMMAND_BUFFER;
    effective_request.fault_point = PROM_M45_FAULT_NONE;
  }
  if (effective_request.projection_path == PROM_M44_PROJECTION_COOPERATIVE &&
      (services_before.cooperative_matrix_feature_enabled == 0u ||
       services_before.subgroup_size != 32u || effective_request.rollback_active != 0u)) {
    if (effective_request.attention.allow_fallback == 0u) {
      out_result->stage = PROM_STAGE_INIT;
      out_result->detail_code = PROM_M44_DETAIL_CAPABILITY;
      return PROM_ERROR;
    }
    effective_request.projection_path = PROM_M44_PROJECTION_CONVENTIONAL_FP16;
  }
  memset(&m43_plan_request, 0, sizeof(m43_plan_request));
  m43_plan_request.head_count = PROM_M44_HEAD_COUNT;
  m43_plan_request.tokens = effective_request.attention.tokens;
  m43_plan_request.model_width = effective_request.attention.model_width;
  m43_plan_request.head_dim = effective_request.attention.head_dim;
  m43_plan_request.scale = effective_request.attention.scale;
  m43_plan_request.scale_explicit = effective_request.attention.scale_explicit;
  m43_plan_request.precision_policy = effective_request.attention.precision_policy;
  m43_plan_request.allow_fallback = effective_request.attention.allow_fallback;
  m43_plan_request.input_mode = PROM_M42_INPUT_RESIDENT_X;
  m43_plan_request.execution_strategy = effective_request.attention.execution_strategy;
  m43_plan_request.cooperative_capability_state = services_before.cooperative_matrix_state;
  m43_plan_request.shared_x_generation = state->m43_resident_x_generation;
  m43_plan_request.shared_x_hash = state->m43_resident_x_hash;
  for (head = 0u; head < PROM_M44_HEAD_COUNT; ++head) {
    m43_plan_request.preferred_path[head] = effective_request.attention.preferred_path[head];
    m43_plan_request.rollback_active[head] = effective_request.attention.rollback_active[head];
    for (weight = 0u; weight < PROM_M43_WEIGHT_KIND_COUNT; ++weight) {
      if (state->m43_weight_generation[head][weight] == 0u ||
          effective_request.attention.required_weight_generation[head][weight] !=
              state->m43_weight_generation[head][weight] ||
          state->m43_weight_model_width[head][weight] != effective_request.attention.model_width ||
          state->m43_weight_head_dim[head][weight] != effective_request.attention.head_dim) {
        out_result->stage = PROM_STAGE_INIT;
        out_result->detail_code = PROM_M43_DETAIL_STALE_WEIGHT_GENERATION;
        return PROM_ERROR;
      }
      m43_plan_request.weight_generation[head][weight] = state->m43_weight_generation[head][weight];
      m43_plan_request.weight_hash[head][weight] = state->m43_weight_hash[head][weight];
    }
  }
  if (prom_m43_attention_plan_build(&m43_plan_request, &out_result->attention.plan) != PROM_OK ||
      !prom_m44_strip_m43_final_readback(&out_result->attention.plan)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M45_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  if (!prom_m42_ensure_pipelines(state) || !prom_m44_ensure_pipelines(state) ||
      !prom_m45_ensure_pipeline(state) ||
      (continuation != NULL &&
       (!prom_m46_ensure_pipelines(state) ||
        (continuation->m47 != NULL && !prom_m47_ensure_pipelines(state))))) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M45_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  for (head = 0u; head < PROM_M44_HEAD_COUNT; ++head) {
    if (!prom_m40b_ensure_sgemm_pipeline(state, out_result->attention.plan.selected_path[head])) {
      out_result->stage = PROM_STAGE_INIT;
      out_result->detail_code = PROM_M45_DETAIL_RESOURCE;
      return PROM_ERROR;
    }
  }
  if (effective_request.projection_path <= PROM_M44_PROJECTION_CONVENTIONAL_FP16 &&
      !prom_m40b_ensure_sgemm_pipeline(state, effective_request.projection_path)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M45_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  if (continuation != NULL && continuation->m47 != NULL &&
      !prom_m40b_ensure_sgemm_pipeline(state, continuation->m47->request->projection_path)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M47_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  memset(&reduction_request, 0, sizeof(reduction_request));
  reduction_request.struct_size = sizeof(reduction_request);
  reduction_request.row_count = effective_request.attention.tokens;
  reduction_request.elements_per_row = effective_request.attention.tokens;
  reduction_request.input_element_count = (uint64_t)effective_request.attention.tokens *
                                           effective_request.attention.tokens;
  reduction_request.output_element_count = reduction_request.input_element_count;
  reduction_request.operation = PROM_REDUCTION_OPERATION_SOFTMAX;
  reduction_request.finalization = PROM_REDUCTION_FINALIZATION_STABLE_SOFTMAX;
  if (prom_reactor_reduction_plan_impl(&reduction_request, &reduction_plan) != PROM_OK) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M45_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  out_result->logical_request_id = state->next_logical_request_id++;
  state->diagnostics.next_logical_request_id = state->next_logical_request_id;
  out_result->attention.logical_request_id = out_result->logical_request_id;
  slot = prom_reduction_acquire_slot(state, out_result->logical_request_id);
  if (slot == NULL) {
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M45_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  out_result->physical_slot_id = slot->slot_id;
  out_result->physical_slot_generation = slot->generation;
  out_result->attention.physical_slot_id = slot->slot_id;
  out_result->attention.physical_slot_generation = slot->generation;
  if (!prom_m43_prepare_execution_buffers(state, slot, &effective_request.attention,
                                           &out_result->attention.plan, 0u)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M45_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  x_f32 = &state->m43_resident_x_f32;
  x_f16 = &state->m43_resident_x_f16;
  if (!prom_m43_setup_descriptors(state, slot, &effective_request.attention,
                                  &out_result->attention, x_f32, x_f16)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M45_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  memset(&m44_plan_request, 0, sizeof(m44_plan_request));
  memcpy(m44_plan_request.head_views, out_result->attention.head_output_view,
         sizeof(m44_plan_request.head_views));
  for (head = 0u; head < PROM_M44_HEAD_COUNT; ++head) {
    m44_plan_request.head_views[head].required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
    out_result->attention.head_output_view[head].required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
  }
  m44_plan_request.head_count = PROM_M44_HEAD_COUNT;
  m44_plan_request.tokens = effective_request.attention.tokens;
  m44_plan_request.head_dim = effective_request.attention.head_dim;
  m44_plan_request.model_width = effective_request.attention.model_width;
  m44_plan_request.precision_policy =
      effective_request.projection_path == PROM_M44_PROJECTION_A2X4_FP32
          ? PROM_M42_PRECISION_FP32 : PROM_M42_PRECISION_F16_ROUNDED;
  m44_plan_request.aggregation_strategy = effective_request.aggregation_strategy;
  m44_plan_request.projection_path = effective_request.projection_path;
  m44_plan_request.submit_plan = PROM_M44_SUBMIT_ONE_COMMAND_BUFFER;
  m44_plan_request.cooperative_capability_state = services_before.cooperative_matrix_state;
  m44_plan_request.rollback_active = effective_request.rollback_active;
  m44_plan_request.wo_generation = state->m44_wo_generation;
  m44_plan_request.wo_hash = state->m44_wo_hash;
  m44_plan_request.m43_aggregate_replay_id = out_result->attention.plan.aggregate_replay_id;
  if (prom_m44_output_projection_plan_build(&m44_plan_request, &out_result->projection_plan) != PROM_OK ||
      out_result->projection_plan.eligibility.eligible == 0u ||
      !prom_m44_prepare_execution_buffers(state, slot, &out_result->projection_plan, 0u,
                                           effective_request.output != NULL ? 1u : 0u) ||
      !prom_m44_setup_descriptors(state, slot, &out_result->projection_plan) ||
      !prom_m45_strip_m44_final_readback(&out_result->projection_plan)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M45_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  y_generation = prom_m40b_hash_u64(out_result->projection_plan.replay_id,
                                    out_result->logical_request_id);
  y_generation = prom_reduction_hash_u32(y_generation, slot->generation);
  if (y_generation == 0u) y_generation = 1u;
  memset(&out_result->x_view, 0, sizeof(out_result->x_view));
  out_result->x_view.buffer = x_f32->buffer;
  out_result->x_view.byte_length = x_f32->size;
  out_result->x_view.element_type = PROM_DEVICE_ELEMENT_F32;
  out_result->x_view.logical_rows = effective_request.attention.tokens;
  out_result->x_view.logical_columns = effective_request.attention.model_width;
  out_result->x_view.row_stride_elements = effective_request.attention.model_width;
  out_result->x_view.layout = PROM_DEVICE_LAYOUT_ROW_MAJOR;
  out_result->x_view.producer_access = PROM_DEVICE_ACCESS_COMPUTE_WRITE;
  out_result->x_view.required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
  out_result->x_view.owning_device = state->device;
  out_result->x_view.owning_lifetime_id = state->m43_resident_x_generation;
  out_result->x_view.owning_slot_id = UINT32_MAX;
  out_result->x_view.owning_slot_generation = (uint32_t)state->m43_resident_x_generation;
  if (out_result->x_view.owning_slot_generation == 0u) out_result->x_view.owning_slot_generation = 1u;
  memset(&out_result->y_view, 0, sizeof(out_result->y_view));
  out_result->y_view.buffer = slot->m44_output.buffer;
  out_result->y_view.byte_length = slot->m44_output.size;
  out_result->y_view.element_type = PROM_DEVICE_ELEMENT_F32;
  out_result->y_view.logical_rows = out_result->projection_plan.tokens;
  out_result->y_view.logical_columns = out_result->projection_plan.model_width;
  out_result->y_view.row_stride_elements = out_result->projection_plan.output_row_stride;
  out_result->y_view.layout = PROM_DEVICE_LAYOUT_ROW_MAJOR;
  out_result->y_view.producer_access = PROM_DEVICE_ACCESS_COMPUTE_WRITE;
  out_result->y_view.required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
  out_result->y_view.owning_device = state->device;
  out_result->y_view.owning_lifetime_id = y_generation;
  out_result->y_view.owning_slot_id = slot->slot_id;
  out_result->y_view.owning_slot_generation = slot->generation;
  memset(&m45_plan_request, 0, sizeof(m45_plan_request));
  m45_plan_request.x_view = out_result->x_view;
  m45_plan_request.y_view = out_result->y_view;
  m45_plan_request.tokens = effective_request.attention.tokens;
  m45_plan_request.model_width = effective_request.attention.model_width;
  m45_plan_request.strategy = effective_request.residual_strategy;
  m45_plan_request.submit_policy = effective_request.submit_policy;
  m45_plan_request.precision_policy = PROM_M45_PRECISION_FP32;
  m45_plan_request.y_exclusive = 1u;
  m45_plan_request.pre_residual_y_consumer_count = 0u;
  m45_plan_request.final_readback = effective_request.output != NULL ? 1u : 0u;
  m45_plan_request.expected_x_generation = state->m43_resident_x_generation;
  m45_plan_request.expected_y_generation = y_generation;
  m45_plan_request.m44_replay_id = out_result->projection_plan.replay_id;
  if (prom_m45_residual_plan_build(&m45_plan_request, &out_result->residual_plan) != PROM_OK ||
      out_result->residual_plan.eligibility.eligible == 0u) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = out_result->residual_plan.eligibility.reason ==
                                      PROM_M45_INELIGIBLE_EXCLUSIVITY
                                  ? PROM_M45_DETAIL_EXCLUSIVITY : PROM_M45_DETAIL_INVALID_VIEW;
    return PROM_ERROR;
  }
  if (effective_request.residual_strategy == PROM_M45_STRATEGY_SEPARATE_OUTPUT &&
      !prom_m45_ensure_buffer(state, &slot->m45_output,
                              (VkDeviceSize)out_result->residual_plan.memory.z_device_bytes,
                              VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M45_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  z = effective_request.residual_strategy == PROM_M45_STRATEGY_IN_PLACE_Y
          ? &slot->m44_output : &slot->m45_output;
  prom_m45_update_descriptor(state, slot, x_f32, &slot->m44_output, z);
  memset(&out_result->z_view, 0, sizeof(out_result->z_view));
  out_result->z_view.buffer = z->buffer;
  out_result->z_view.byte_length = z->size;
  out_result->z_view.element_type = PROM_DEVICE_ELEMENT_F32;
  out_result->z_view.logical_rows = out_result->residual_plan.tokens;
  out_result->z_view.logical_columns = out_result->residual_plan.model_width;
  out_result->z_view.row_stride_elements = out_result->residual_plan.z_row_stride;
  out_result->z_view.layout = PROM_DEVICE_LAYOUT_ROW_MAJOR;
  out_result->z_view.producer_access = effective_request.output != NULL
                                        ? PROM_DEVICE_ACCESS_TRANSFER_READ
                                        : PROM_DEVICE_ACCESS_COMPUTE_WRITE;
  out_result->z_view.required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
  out_result->z_view.owning_device = state->device;
  out_result->z_view.owning_lifetime_id = out_result->residual_plan.z_generation;
  out_result->z_view.owning_slot_id = slot->slot_id;
  out_result->z_view.owning_slot_generation = slot->generation;
  if (continuation != NULL &&
      !prom_m46_prepare_continuation(state, slot, out_result, continuation)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    continuation->result->physical_slot_recyclable = 1u;
    continuation->result->stage = PROM_STAGE_TRANSFER_IN;
    continuation->result->detail_code = PROM_M46_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  if (continuation != NULL && continuation->m47 != NULL &&
      !prom_m47_prepare_continuation(state, slot, continuation, continuation->m47)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    continuation->result->physical_slot_recyclable = 1u;
    continuation->m47->result->physical_slot_recyclable = 1u;
    continuation->m47->result->stage = PROM_STAGE_TRANSFER_IN;
    continuation->m47->result->detail_code = PROM_M47_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  memset(&projection_request, 0, sizeof(projection_request));
  projection_request.attention = effective_request.attention;
  projection_request.aggregation_strategy = effective_request.aggregation_strategy;
  projection_request.projection_path = effective_request.projection_path;
  projection_request.submit_plan = PROM_M44_SUBMIT_ONE_COMMAND_BUFFER;
  projection_request.rollback_active = effective_request.rollback_active;
  projection_request.required_wo_generation = effective_request.required_wo_generation;
  recording_begin = prom_reduction_now_ns();
  m43_record_status = prom_m43_record_grouped_internal(
      state, slot, &effective_request.attention, &out_result->attention.plan, &reduction_plan,
      &m43_partial_fault, &m43_uncertain_fault, 0u, 1u,
      VK_NULL_HANDLE, NULL, 0u, 0u);
  if (m43_record_status != 1 || m43_partial_fault != 0u || m43_uncertain_fault != 0u) {
    if (m43_record_status == 1) (void)vkEndCommandBuffer(slot->command_buffer);
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M45_DETAIL_COMMAND;
    return PROM_ERROR;
  }
  m44_record_status = prom_m44_record_projection_tail(
      state, slot, &projection_request, &out_result->projection_plan,
      slot->command_buffer, 1u, 0u, &m44_partial_fault);
  if (m44_record_status != 1 || m44_partial_fault != 0u) {
    (void)vkEndCommandBuffer(slot->command_buffer);
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_M44_STAGE_OUTPUT_PROJECTION;
    out_result->detail_code = PROM_M45_DETAIL_COMMAND;
    return PROM_ERROR;
  }
  m45_record_status = prom_m45_record_residual_tail(
      state, slot, &effective_request, &out_result->residual_plan, x_f32, z,
      effective_request.submit_policy == PROM_M45_SUBMIT_ONE_COMMAND_BUFFER
          ? slot->command_buffer : slot->consumer_command_buffer,
      effective_request.submit_policy == PROM_M45_SUBMIT_ONE_COMMAND_BUFFER, &m45_partial_fault);
  if (continuation != NULL && m45_record_status == 1) {
    if (continuation->m47 != NULL) {
      m46_record_status = prom_m46_record_tail(
          state, slot, continuation->request, &continuation->result->rmsnorm_plan,
          continuation->z, continuation->n, slot->command_buffer, 1u,
          0u, &m46_partial_fault);
      if (m46_record_status == 1 &&
          continuation->m47->request->submit_policy == PROM_M47_SUBMIT_TWO_BOUNDED) {
        if (vkEndCommandBuffer(slot->command_buffer) != VK_SUCCESS) {
          m47_record_status = 0;
        } else {
          m47_record_status = prom_m47_record_tail(
              state, slot, continuation->m47->request,
              &continuation->m47->result->ffn_plan, continuation->m47->n,
              continuation->m47->output, slot->consumer_command_buffer, 0u,
              0u, 0u, &m47_partial_fault);
        }
      } else if (m46_record_status == 1) {
        m47_record_status = prom_m47_record_tail(
            state, slot, continuation->m47->request,
            &continuation->m47->result->ffn_plan, continuation->m47->n,
            continuation->m47->output, slot->command_buffer, 1u,
            0u, 0u, &m47_partial_fault);
      }
    } else if (continuation->request->submit_policy == PROM_M46_SUBMIT_TWO_BOUNDED) {
      if (vkEndCommandBuffer(slot->command_buffer) != VK_SUCCESS) {
        m46_record_status = 0;
      } else {
        m46_record_status = prom_m46_record_tail(
            state, slot, continuation->request, &continuation->result->rmsnorm_plan,
            continuation->z, continuation->n, slot->consumer_command_buffer, 0u,
            0u, &m46_partial_fault);
      }
    } else {
      m46_record_status = prom_m46_record_tail(
          state, slot, continuation->request, &continuation->result->rmsnorm_plan,
          continuation->z, continuation->n, slot->command_buffer, 1u,
          0u, &m46_partial_fault);
    }
  }
  if (continuation == NULL && effective_request.submit_policy == PROM_M45_SUBMIT_TWO_BOUNDED &&
      vkEndCommandBuffer(slot->command_buffer) != VK_SUCCESS) m45_record_status = 0;
  if (m45_record_status == 0 || m46_record_status == 0 || m47_record_status == 0 ||
      vkEndCommandBuffer(continuation != NULL && continuation->m47 != NULL &&
                             continuation->m47->request->submit_policy == PROM_M47_SUBMIT_TWO_BOUNDED
                           ? slot->consumer_command_buffer
                           : (continuation != NULL &&
                              continuation->request->submit_policy == PROM_M46_SUBMIT_TWO_BOUNDED
                           ? slot->consumer_command_buffer
                           : (effective_request.submit_policy == PROM_M45_SUBMIT_ONE_COMMAND_BUFFER
                                ? slot->command_buffer : slot->consumer_command_buffer))) != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    if (continuation != NULL) {
      continuation->result->physical_slot_recyclable = 1u;
      continuation->result->stage = PROM_M46_STAGE_APPLY;
      continuation->result->detail_code = PROM_M46_DETAIL_COMMAND;
      if (continuation->m47 != NULL) {
        continuation->m47->result->physical_slot_recyclable = 1u;
        continuation->m47->result->stage = PROM_M47_STAGE_SECOND_RESIDUAL;
        continuation->m47->result->detail_code = PROM_M47_DETAIL_COMMAND;
      }
    }
    out_result->stage = continuation != NULL && continuation->m47 != NULL
                          ? PROM_M47_STAGE_SECOND_RESIDUAL
                          : (continuation != NULL ? PROM_M46_STAGE_APPLY : PROM_M45_STAGE_RESIDUAL_ADD);
    out_result->detail_code = continuation != NULL && continuation->m47 != NULL
                                ? PROM_M47_DETAIL_COMMAND
                                : (continuation != NULL ? PROM_M46_DETAIL_COMMAND : PROM_M45_DETAIL_COMMAND);
    return PROM_ERROR;
  }
  out_result->cpu_recording_ns = prom_reduction_elapsed_ns(recording_begin, prom_reduction_now_ns());
  slot->m45_command_reuse_count += 1u;
  if (continuation != NULL) slot->m46_command_reuse_count += 1u;
  if (continuation != NULL && continuation->m47 != NULL) slot->m47_command_reuse_count += 1u;
  if (vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M45_DETAIL_SUBMIT;
    return PROM_ERROR;
  }
  memset(submits, 0, sizeof(submits));
  split_submit = continuation != NULL
                   ? (continuation->m47 != NULL
                        ? (continuation->m47->request->submit_policy == PROM_M47_SUBMIT_TWO_BOUNDED)
                        : (continuation->request->submit_policy == PROM_M46_SUBMIT_TWO_BOUNDED))
                   : (effective_request.submit_policy == PROM_M45_SUBMIT_TWO_BOUNDED);
  submits[0].sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submits[0].commandBufferCount = 1u;
  submits[0].pCommandBuffers = &slot->command_buffer;
  if (split_submit != 0u) {
    submits[0].signalSemaphoreCount = 1u;
    submits[0].pSignalSemaphores = &slot->producer_complete;
    submits[1].sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
    submits[1].waitSemaphoreCount = 1u;
    submits[1].pWaitSemaphores = &slot->producer_complete;
    submits[1].pWaitDstStageMask = &wait_stage;
    submits[1].commandBufferCount = 1u;
    submits[1].pCommandBuffers = &slot->consumer_command_buffer;
  }
  submission_begin = prom_reduction_now_ns();
  result = vkQueueSubmit(state->queue,
                         split_submit != 0u ? 2u : 1u,
                         submits, slot->fence);
  out_result->cpu_submission_ns = prom_reduction_elapsed_ns(submission_begin, prom_reduction_now_ns());
  if (result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M45_DETAIL_SUBMIT;
    return PROM_ERROR;
  }
  slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
  if (continuation != NULL && continuation->m47 != NULL &&
      continuation->m47->request->fault_point == PROM_M47_FAULT_UNCERTAIN_COMPLETION) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    continuation->m47->result->stage = PROM_M47_STAGE_SECOND_RESIDUAL;
    continuation->m47->result->detail_code = PROM_M47_DETAIL_COMPLETION_UNCERTAIN;
    continuation->result->stage = PROM_M47_STAGE_SECOND_RESIDUAL;
    continuation->result->detail_code = PROM_M47_DETAIL_COMPLETION_UNCERTAIN;
    out_result->stage = PROM_M47_STAGE_SECOND_RESIDUAL;
    out_result->detail_code = PROM_M47_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  if (continuation != NULL &&
      continuation->request->fault_point == PROM_M46_FAULT_UNCERTAIN_COMPLETION) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    continuation->result->stage = PROM_M46_STAGE_APPLY;
    continuation->result->detail_code = PROM_M46_DETAIL_COMPLETION_UNCERTAIN;
    out_result->stage = PROM_M46_STAGE_APPLY;
    out_result->detail_code = PROM_M46_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  if (request->fault_point == PROM_M45_FAULT_UNCERTAIN_COMPLETION) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    out_result->stage = PROM_M45_STAGE_RESIDUAL_ADD;
    out_result->detail_code = PROM_M45_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
  if (result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M45_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  if (m45_partial_fault != 0u || request->fault_point == PROM_M45_FAULT_AFTER_RESIDUAL_SUBMISSION) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = m45_partial_fault != 0u ? m45_partial_fault : PROM_M45_STAGE_RESIDUAL_ADD;
    out_result->detail_code = PROM_M45_DETAIL_FAULT_INJECTED;
    return PROM_ERROR;
  }
  if (continuation != NULL &&
      (m46_partial_fault != 0u ||
       continuation->request->fault_point == PROM_M46_FAULT_AFTER_SUBMISSION)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    continuation->result->physical_slot_recyclable = 1u;
    continuation->result->stage = m46_partial_fault != 0u
                                    ? m46_partial_fault : PROM_M46_STAGE_APPLY;
    continuation->result->detail_code = PROM_M46_DETAIL_FAULT_INJECTED;
    out_result->stage = continuation->result->stage;
    out_result->detail_code = PROM_M46_DETAIL_FAULT_INJECTED;
    return PROM_ERROR;
  }
  if (continuation != NULL && continuation->m47 != NULL &&
      (m47_partial_fault != 0u ||
       continuation->m47->request->fault_point == PROM_M47_FAULT_AFTER_RESIDUAL_SUBMISSION)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    continuation->result->physical_slot_recyclable = 1u;
    continuation->m47->result->physical_slot_recyclable = 1u;
    continuation->m47->result->stage = m47_partial_fault != 0u
                                         ? m47_partial_fault : PROM_M47_STAGE_SECOND_RESIDUAL;
    continuation->m47->result->detail_code = PROM_M47_DETAIL_FAULT_INJECTED;
    out_result->stage = continuation->m47->result->stage;
    out_result->detail_code = PROM_M47_DETAIL_FAULT_INJECTED;
    return PROM_ERROR;
  }
  memset(timestamps, 0, sizeof(timestamps));
  if (state->timestamp_supported == 0u || state->query_pool == VK_NULL_HANDLE ||
      vkGetQueryPoolResults(state->device, state->query_pool,
                            slot->slot_id * PROM_REDUCTION_QUERY_STRIDE,
                            PROM_M43_QUERY_GROUP_END + 1u,
                            sizeof(uint64_t) * (PROM_M43_QUERY_GROUP_END + 1u), timestamps,
                            sizeof(uint64_t), VK_QUERY_RESULT_64_BIT) != VK_SUCCESS ||
      vkGetQueryPoolResults(state->device, state->query_pool,
                            slot->slot_id * PROM_REDUCTION_QUERY_STRIDE + PROM_M44_QUERY_BASE,
                            4u, sizeof(uint64_t) * 4u, timestamps + PROM_M44_QUERY_BASE,
                            sizeof(uint64_t), VK_QUERY_RESULT_64_BIT) != VK_SUCCESS ||
      vkGetQueryPoolResults(state->device, state->query_pool,
                            slot->slot_id * PROM_REDUCTION_QUERY_STRIDE + PROM_M45_QUERY_BASE,
                            effective_request.output != NULL ? PROM_M45_QUERY_COUNT : 2u,
                            sizeof(uint64_t) * (effective_request.output != NULL ? PROM_M45_QUERY_COUNT : 2u),
                            timestamps + PROM_M45_QUERY_BASE, sizeof(uint64_t),
                            VK_QUERY_RESULT_64_BIT) != VK_SUCCESS ||
      (continuation != NULL &&
       (vkGetQueryPoolResults(state->device, state->query_pool,
                              slot->slot_id * PROM_REDUCTION_QUERY_STRIDE + PROM_M46_QUERY_BASE,
                              continuation->request->output != NULL ? PROM_M46_QUERY_COUNT : 5u,
                              sizeof(uint64_t) * (continuation->request->output != NULL
                                                    ? PROM_M46_QUERY_COUNT : 5u),
                              timestamps + PROM_M46_QUERY_BASE, sizeof(uint64_t),
                              VK_QUERY_RESULT_64_BIT) != VK_SUCCESS ||
        timestamps[PROM_M46_QUERY_PARTIAL_END] < timestamps[PROM_M46_QUERY_REDUCTION_BEGIN] ||
        timestamps[PROM_M46_QUERY_FINAL_END] < timestamps[PROM_M46_QUERY_PARTIAL_END] ||
        timestamps[PROM_M46_QUERY_APPLY_END] < timestamps[PROM_M46_QUERY_APPLY_BEGIN])) ||
      (continuation != NULL && continuation->m47 != NULL &&
       (vkGetQueryPoolResults(state->device, state->query_pool,
                              slot->slot_id * PROM_REDUCTION_QUERY_STRIDE + PROM_M47_QUERY_BASE,
                              continuation->m47->request->output != NULL ? PROM_M47_QUERY_COUNT : 16u,
                              sizeof(uint64_t) * (continuation->m47->request->output != NULL
                                                    ? PROM_M47_QUERY_COUNT : 16u),
                              timestamps + PROM_M47_QUERY_BASE, sizeof(uint64_t),
                              VK_QUERY_RESULT_64_BIT) != VK_SUCCESS ||
        timestamps[PROM_M47_QUERY_PACK_N_END] < timestamps[PROM_M47_QUERY_PACK_N_BEGIN] ||
        timestamps[PROM_M47_QUERY_GATE_END] < timestamps[PROM_M47_QUERY_GATE_BEGIN] ||
        timestamps[PROM_M47_QUERY_UP_END] < timestamps[PROM_M47_QUERY_UP_BEGIN] ||
        timestamps[PROM_M47_QUERY_ACTIVATION_END] < timestamps[PROM_M47_QUERY_ACTIVATION_BEGIN] ||
        timestamps[PROM_M47_QUERY_MULTIPLY_END] < timestamps[PROM_M47_QUERY_MULTIPLY_BEGIN] ||
        timestamps[PROM_M47_QUERY_HIDDEN_PACK_END] < timestamps[PROM_M47_QUERY_HIDDEN_PACK_BEGIN] ||
        timestamps[PROM_M47_QUERY_DOWN_END] < timestamps[PROM_M47_QUERY_DOWN_BEGIN] ||
        timestamps[PROM_M47_QUERY_RESIDUAL_END] < timestamps[PROM_M47_QUERY_RESIDUAL_BEGIN])) ||
      timestamps[PROM_M44_QUERY_AGGREGATION_END] < timestamps[PROM_M44_QUERY_AGGREGATION_BEGIN] ||
      timestamps[PROM_M44_QUERY_PROJECTION_END] < timestamps[PROM_M44_QUERY_PROJECTION_BEGIN] ||
      timestamps[PROM_M45_QUERY_RESIDUAL_END] < timestamps[PROM_M45_QUERY_RESIDUAL_BEGIN] ||
      !prom_m44_fill_m43_timings(state, timestamps, &out_result->attention)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_TRANSFER_OUT;
    out_result->detail_code = PROM_M45_DETAIL_QUERY;
    if (continuation != NULL) {
      continuation->result->physical_slot_recyclable = 1u;
      continuation->result->stage = PROM_STAGE_TRANSFER_OUT;
      continuation->result->detail_code = PROM_M46_DETAIL_QUERY;
      if (continuation->m47 != NULL) {
        continuation->m47->result->physical_slot_recyclable = 1u;
        continuation->m47->result->stage = PROM_STAGE_TRANSFER_OUT;
        continuation->m47->result->detail_code = PROM_M47_DETAIL_QUERY;
      }
    }
    return PROM_ERROR;
  }
  out_result->aggregation_gpu_ns =
      (uint64_t)((double)(timestamps[PROM_M44_QUERY_AGGREGATION_END] -
                         timestamps[PROM_M44_QUERY_AGGREGATION_BEGIN]) * state->timestamp_period_ns);
  out_result->projection_gpu_ns =
      (uint64_t)((double)(timestamps[PROM_M44_QUERY_PROJECTION_END] -
                         timestamps[PROM_M44_QUERY_PROJECTION_BEGIN]) * state->timestamp_period_ns);
  out_result->m44_gpu_ns =
      (uint64_t)((double)(timestamps[PROM_M44_QUERY_PROJECTION_END] -
                         timestamps[PROM_M44_QUERY_AGGREGATION_BEGIN]) * state->timestamp_period_ns);
  out_result->residual_gpu_ns =
      (uint64_t)((double)(timestamps[PROM_M45_QUERY_RESIDUAL_END] -
                         timestamps[PROM_M45_QUERY_RESIDUAL_BEGIN]) * state->timestamp_period_ns);
  out_result->total_m43_m44_m45_gpu_ns =
      (uint64_t)((double)(timestamps[PROM_M45_QUERY_RESIDUAL_END] - timestamps[3u]) *
                 state->timestamp_period_ns);
  if (effective_request.output != NULL) {
    readback_begin = prom_reduction_now_ns();
    memcpy(effective_request.output, slot->m44_readback.mapped,
           (size_t)out_result->residual_plan.memory.z_readback_bytes);
    out_result->final_readback_ns =
        (uint64_t)((double)(timestamps[PROM_M45_QUERY_READBACK_END] -
                           timestamps[PROM_M45_QUERY_READBACK_BEGIN]) * state->timestamp_period_ns) +
        prom_reduction_elapsed_ns(readback_begin, prom_reduction_now_ns());
  }
  out_result->submit_count = out_result->residual_plan.submit_count;
  out_result->final_readback_count = out_result->residual_plan.final_readback_count;
  out_result->no_intermediate_host_copy = 1u;
  out_result->exact_request_bytes = out_result->attention.plan.memory.exact_retained_bytes +
                                    out_result->projection_plan.memory.exact_request_bytes +
                                    out_result->residual_plan.memory.z_device_bytes +
                                    out_result->residual_plan.memory.z_readback_bytes;
  out_result->retained_bytes = prom_m43_retained_bytes(state, slot) +
                               prom_m44_retained_bytes(state, slot) + prom_m45_retained_bytes(slot);
  out_result->buffer_allocation_count = state->m43_buffer_grow_count + state->m44_buffer_grow_count +
                                        state->m45_buffer_grow_count;
  out_result->buffer_reuse_count = state->m43_buffer_reuse_count + state->m44_buffer_reuse_count +
                                   state->m45_buffer_reuse_count;
  out_result->descriptor_update_count = state->m43_descriptor_update_count +
                                        state->m44_descriptor_update_count +
                                        state->m45_descriptor_update_count;
  out_result->pipeline_create_count = state->m42_pipeline_create_count +
                                      state->m40b_pipeline_create_count +
                                      state->m44_pipeline_create_count +
                                      state->m45_pipeline_create_count +
                                      state->diagnostics.pipeline_create_count;
  out_result->command_buffer_reuse_count = slot->m45_command_reuse_count;
  out_result->x_generation = out_result->residual_plan.x_generation;
  out_result->y_generation = out_result->residual_plan.y_generation;
  out_result->z_generation = out_result->residual_plan.z_generation;
  out_result->validation_error_count_before = services_before.validation_error_count;
  out_result->attention.submit_count = 1u;
  out_result->attention.final_readback_count = 0u;
  out_result->attention.no_intermediate_host_copy = 1u;
  out_result->attention.shared_x_conversion_count = 0u;
  out_result->attention.shared_x_upload_count = 0u;
  out_result->attention.shared_x_consumer_count = PROM_M44_HEAD_COUNT;
  out_result->attention.persistent_weight_count = PROM_M44_HEAD_COUNT * PROM_M43_WEIGHT_KIND_COUNT;
  out_result->attention.qkv_projection_dispatch_count = PROM_M44_HEAD_COUNT * PROM_M43_WEIGHT_KIND_COUNT;
  out_result->attention.shared_x_generation = state->m43_resident_x_generation;
  out_result->attention.physical_slot_recyclable = 1u;
  memcpy(out_result->attention.weight_generation, state->m43_weight_generation,
         sizeof(out_result->attention.weight_generation));
  if (continuation != NULL &&
      !prom_m46_complete_continuation(state, slot, out_result, continuation,
                                      timestamps, begin_ns)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    continuation->result->physical_slot_recyclable = 1u;
    continuation->result->stage = PROM_STAGE_TRANSFER_OUT;
    continuation->result->detail_code = PROM_M46_DETAIL_READBACK;
    return PROM_ERROR;
  }
  if (continuation != NULL && continuation->m47 != NULL &&
      !prom_m47_complete_continuation(state, slot, continuation->m47,
                                      timestamps, begin_ns)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    continuation->result->physical_slot_recyclable = 1u;
    continuation->m47->result->physical_slot_recyclable = 1u;
    continuation->m47->result->stage = PROM_STAGE_TRANSFER_OUT;
    continuation->m47->result->detail_code = PROM_M47_DETAIL_READBACK;
    return PROM_ERROR;
  }
  if (prom_reactor_runtime_get_vk_services(handle, &services_after) == PROM_OK)
    out_result->validation_error_count_after = services_after.validation_error_count;
  out_result->end_to_end_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->physical_slot_recyclable = 1u;
  if (continuation != NULL) continuation->result->physical_slot_recyclable = 1u;
  if (continuation != NULL && continuation->m47 != NULL)
    continuation->m47->result->physical_slot_recyclable = 1u;
  out_result->stage = 0u;
  out_result->detail_code = 0;
  return PROM_OK;
}

int prom_reactor_runtime_m45_execute_composed(void* handle,
                                              const prom_m45_composed_request* request,
                                              prom_m45_composed_result* out_result) {
  return prom_m45_execute_composed_core(handle, request, out_result, NULL);
}

int prom_reactor_runtime_m45_read_resident_x(void* handle,
                                             const prom_m45_resident_x_readback_request* request,
                                             prom_m45_resident_x_readback_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_reduction_slot* slot;
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  VkSubmitInfo submit;
  VkResult result;
  uint64_t elements;
  uint64_t timestamps[2];
  const uint64_t begin_ns = prom_reduction_now_ns();
  int32_t detail = 0;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  out_result->physical_slot_id = UINT32_MAX;
  if (request == NULL || request->output == NULL || request->tokens == 0u ||
      request->tokens > PROM_M42_MAX_TOKENS || request->model_width == 0u ||
      request->model_width > PROM_M42_MAX_MODEL_WIDTH || request->expected_x_generation == 0u ||
      !prom_m40b_checked_product_u64(request->tokens, request->model_width, &elements) ||
      request->output_element_count != elements || elements > UINT64_MAX / sizeof(float)) {
    out_result->detail_code = PROM_M45_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL) {
    out_result->detail_code = detail;
    return PROM_ERROR;
  }
  if (state->m43_resident_x_generation != request->expected_x_generation ||
      state->m43_resident_x_tokens != request->tokens ||
      state->m43_resident_x_model_width != request->model_width) {
    out_result->detail_code = PROM_M45_DETAIL_STALE_X_GENERATION;
    return PROM_ERROR;
  }
  slot = prom_reduction_acquire_slot(state, state->next_logical_request_id++);
  state->diagnostics.next_logical_request_id = state->next_logical_request_id;
  if (slot == NULL) {
    out_result->detail_code = PROM_M45_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  out_result->physical_slot_id = slot->slot_id;
  out_result->physical_slot_generation = slot->generation;
  if (!prom_m45_ensure_buffer(state, &slot->m45_x_readback,
                              (VkDeviceSize)(elements * sizeof(float)),
                              VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                              VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                              1)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->detail_code = PROM_M45_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  if (vkResetCommandBuffer(slot->command_buffer, 0u) != VK_SUCCESS) goto command_fail;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(slot->command_buffer, &begin_info) != VK_SUCCESS) goto command_fail;
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE)
    vkCmdResetQueryPool(slot->command_buffer, state->query_pool,
                        slot->slot_id * PROM_REDUCTION_QUERY_STRIDE + PROM_M45_QUERY_BASE, 2u);
  if (!prom_m43_one_buffer_barrier(slot->command_buffer, &state->m43_resident_x_f32,
                                   (VkDeviceSize)(elements * sizeof(float)),
                                   VK_ACCESS_SHADER_READ_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT)) goto command_fail;
  prom_m42_write_timestamp(state, slot, slot->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                           PROM_M45_QUERY_RESIDUAL_BEGIN);
  memset(&copy, 0, sizeof(copy));
  copy.size = (VkDeviceSize)(elements * sizeof(float));
  vkCmdCopyBuffer(slot->command_buffer, state->m43_resident_x_f32.buffer,
                  slot->m45_x_readback.buffer, 1u, &copy);
  prom_m42_write_timestamp(state, slot, slot->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                           PROM_M45_QUERY_RESIDUAL_END);
  if (!prom_m43_one_buffer_barrier(slot->command_buffer, &slot->m45_x_readback,
                                   (VkDeviceSize)(elements * sizeof(float)),
                                   VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_HOST_READ_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_HOST_BIT) ||
      vkEndCommandBuffer(slot->command_buffer) != VK_SUCCESS) goto command_fail;
  if (vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS) goto command_fail;
  memset(&submit, 0, sizeof(submit));
  submit.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit.commandBufferCount = 1u;
  submit.pCommandBuffers = &slot->command_buffer;
  result = vkQueueSubmit(state->queue, 1u, &submit, slot->fence);
  if (result != VK_SUCCESS) goto command_fail;
  slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
  if (vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX) != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    out_result->detail_code = PROM_M45_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  memset(timestamps, 0, sizeof(timestamps));
  if (state->timestamp_supported == 0u || state->query_pool == VK_NULL_HANDLE ||
      vkGetQueryPoolResults(state->device, state->query_pool,
                            slot->slot_id * PROM_REDUCTION_QUERY_STRIDE + PROM_M45_QUERY_BASE,
                            2u, sizeof(timestamps), timestamps, sizeof(uint64_t),
                            VK_QUERY_RESULT_64_BIT) != VK_SUCCESS || timestamps[1] < timestamps[0]) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->detail_code = PROM_M45_DETAIL_QUERY;
    return PROM_ERROR;
  }
  memcpy(request->output, slot->m45_x_readback.mapped, (size_t)(elements * sizeof(float)));
  out_result->gpu_readback_ns =
      (uint64_t)((double)(timestamps[1] - timestamps[0]) * state->timestamp_period_ns);
  out_result->end_to_end_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  out_result->x_generation = state->m43_resident_x_generation;
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->physical_slot_recyclable = 1u;
  return PROM_OK;

command_fail:
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->physical_slot_recyclable = 1u;
  out_result->detail_code = PROM_M45_DETAIL_COMMAND;
  return PROM_ERROR;
}

int prom_reactor_runtime_m46_execute_composed(void* handle,
                                              const prom_m46_composed_request* request,
                                              prom_m46_composed_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_m45_composed_request upstream_request;
  prom_m46_continuation continuation;
  uint64_t logical_elements;
  int32_t detail = 0;
  int status;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  out_result->physical_slot_id = UINT32_MAX;
  if (request == NULL || request->strategy < PROM_M46_STRATEGY_SEPARATE_OUTPUT ||
      request->strategy > PROM_M46_STRATEGY_IN_PLACE_Z ||
      request->submit_policy < PROM_M46_SUBMIT_ONE_COMMAND_BUFFER ||
      request->submit_policy > PROM_M46_SUBMIT_TWO_BOUNDED ||
      request->requested_reduction_plan > PROM_M46_REDUCTION_FORCE_STAGED ||
      (request->requested_reduction_plan == PROM_M46_REDUCTION_FORCE_FUSED &&
       request->upstream.attention.model_width > 1024u) ||
      request->fault_point > PROM_M46_FAULT_UNCERTAIN_COMPLETION ||
      request->required_weight_generation == 0u ||
      !isfinite(request->epsilon) || request->epsilon <= 0.0f ||
      !prom_m40b_checked_product_u64(request->upstream.attention.tokens,
                                     request->upstream.attention.model_width,
                                     &logical_elements) ||
      (request->output != NULL && request->output_element_count != logical_elements) ||
      (request->output == NULL && request->output_element_count != 0u)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M46_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = detail;
    return PROM_ERROR;
  }
  if (state->m46_weight_generation != request->required_weight_generation ||
      state->m46_weight_model_width != request->upstream.attention.model_width ||
      state->m46_weight_hash == 0u) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M46_DETAIL_STALE_WEIGHT_GENERATION;
    return PROM_ERROR;
  }
  upstream_request = request->upstream;
  upstream_request.output = NULL;
  upstream_request.output_element_count = 0u;
  upstream_request.submit_policy = PROM_M45_SUBMIT_ONE_COMMAND_BUFFER;
  upstream_request.fault_point = PROM_M45_FAULT_NONE;
  memset(&continuation, 0, sizeof(continuation));
  continuation.request = request;
  continuation.result = out_result;
  status = prom_m45_execute_composed_core(handle, &upstream_request,
                                          &out_result->upstream, &continuation);
  if (status != PROM_OK) {
    if (out_result->detail_code == 0) {
      out_result->stage = out_result->upstream.stage;
      out_result->detail_code = out_result->upstream.detail_code;
    }
    out_result->physical_slot_id = out_result->upstream.physical_slot_id;
    out_result->physical_slot_generation = out_result->upstream.physical_slot_generation;
    out_result->physical_slot_recyclable = out_result->upstream.physical_slot_recyclable;
    return PROM_ERROR;
  }
  return PROM_OK;
}

int prom_reactor_runtime_m47_execute_composed(void* handle,
                                              const prom_m47_composed_request* request,
                                              prom_m47_composed_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_vk_runtime_services services;
  prom_m47_composed_request effective;
  prom_m46_composed_request m46_request;
  prom_m45_composed_request m45_request;
  prom_m46_continuation m46_continuation;
  prom_m47_continuation m47_continuation;
  uint64_t logical_elements;
  uint32_t weight;
  int32_t detail = 0;
  int status;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  out_result->physical_slot_id = UINT32_MAX;
  if (request == NULL || request->ffn_width == 0u || request->ffn_width > 8192u ||
      request->projection_path < PROM_M47_PROJECTION_COOPERATIVE ||
      request->projection_path > PROM_M47_PROJECTION_CONVENTIONAL_FP16 ||
      request->gating_strategy < PROM_M47_GATING_SEPARATE ||
      request->gating_strategy > PROM_M47_GATING_FUSED_DIRECT_PACKED ||
      request->residual_strategy < PROM_M47_RESIDUAL_SEPARATE_OUTPUT ||
      request->residual_strategy > PROM_M47_RESIDUAL_IN_PLACE_N_AUDIT ||
      request->submit_policy < PROM_M47_SUBMIT_ONE_COMMAND_BUFFER ||
      request->submit_policy > PROM_M47_SUBMIT_TWO_BOUNDED ||
      request->fault_point > PROM_M47_FAULT_UNCERTAIN_COMPLETION ||
      request->upstream.output != NULL || request->upstream.output_element_count != 0u ||
      !prom_m40b_checked_product_u64(request->upstream.upstream.attention.tokens,
                                     request->upstream.upstream.attention.model_width,
                                     &logical_elements) ||
      (request->output != NULL && request->output_element_count != logical_elements) ||
      (request->output == NULL && request->output_element_count != 0u)) {
    out_result->detail_code = request != NULL &&
                                      request->residual_strategy == PROM_M47_RESIDUAL_IN_PLACE_N_AUDIT
                                  ? PROM_M47_DETAIL_IN_PLACE_N_REJECTED
                                  : PROM_M47_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  if (request->residual_strategy == PROM_M47_RESIDUAL_IN_PLACE_N_AUDIT) {
    out_result->detail_code = PROM_M47_DETAIL_IN_PLACE_N_REJECTED;
    return PROM_ERROR;
  }
  for (weight = 0u; weight < PROM_M47_WEIGHT_COUNT; ++weight) {
    if (request->required_weight_generation[weight] == 0u) {
      out_result->detail_code = PROM_M47_DETAIL_INVALID_REQUEST;
      return PROM_ERROR;
    }
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL || prom_reactor_runtime_get_vk_services(handle, &services) != PROM_OK) {
    out_result->detail_code = state == NULL ? detail : PROM_M47_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  if (state->m46_weight_generation != request->upstream.required_weight_generation ||
      state->m46_weight_model_width != request->upstream.upstream.attention.model_width ||
      state->m46_weight_hash == 0u) {
    out_result->detail_code = PROM_M46_DETAIL_STALE_WEIGHT_GENERATION;
    return PROM_ERROR;
  }
  for (weight = 0u; weight < PROM_M47_WEIGHT_COUNT; ++weight) {
    if (state->m47_weight_generation[weight] != request->required_weight_generation[weight] ||
        state->m47_weight_hash[weight] == 0u ||
        state->m47_weight_model_width[weight] != request->upstream.upstream.attention.model_width ||
        state->m47_weight_ffn_width[weight] != request->ffn_width) {
      out_result->detail_code = PROM_M47_DETAIL_STALE_WEIGHT_GENERATION;
      return PROM_ERROR;
    }
  }
  effective = *request;
  if (effective.projection_path == PROM_M47_PROJECTION_COOPERATIVE &&
      (services.cooperative_matrix_feature_enabled == 0u || services.subgroup_size != 32u)) {
    if (effective.upstream.upstream.attention.allow_fallback == 0u) {
      out_result->detail_code = PROM_M47_DETAIL_PRECISION;
      return PROM_ERROR;
    }
    effective.projection_path = PROM_M47_PROJECTION_CONVENTIONAL_FP16;
  }
  if (effective.gating_strategy == PROM_M47_GATING_FUSED_DIRECT_PACKED &&
      effective.projection_path == PROM_M47_PROJECTION_A2X4_FP32) {
    out_result->detail_code = PROM_M47_DETAIL_GATING;
    return PROM_ERROR;
  }
  m46_request = effective.upstream;
  m46_request.output = NULL;
  m46_request.output_element_count = 0u;
  m46_request.submit_policy = PROM_M46_SUBMIT_ONE_COMMAND_BUFFER;
  m46_request.fault_point = PROM_M46_FAULT_NONE;
  m45_request = m46_request.upstream;
  m45_request.output = NULL;
  m45_request.output_element_count = 0u;
  m45_request.submit_policy = PROM_M45_SUBMIT_ONE_COMMAND_BUFFER;
  m45_request.fault_point = PROM_M45_FAULT_NONE;
  memset(&m46_continuation, 0, sizeof(m46_continuation));
  memset(&m47_continuation, 0, sizeof(m47_continuation));
  m47_continuation.request = &effective;
  m47_continuation.result = out_result;
  m46_continuation.request = &m46_request;
  m46_continuation.result = &out_result->upstream;
  m46_continuation.m47 = &m47_continuation;
  status = prom_m45_execute_composed_core(handle, &m45_request,
                                          &out_result->upstream.upstream,
                                          &m46_continuation);
  if (status != PROM_OK) {
    if (out_result->detail_code == 0) {
      if (out_result->upstream.detail_code != 0) {
        out_result->stage = out_result->upstream.stage;
        out_result->detail_code = out_result->upstream.detail_code;
      } else {
        out_result->stage = out_result->upstream.upstream.stage;
        out_result->detail_code = out_result->upstream.upstream.detail_code;
      }
    }
    out_result->physical_slot_id = out_result->upstream.upstream.physical_slot_id;
    out_result->physical_slot_generation = out_result->upstream.upstream.physical_slot_generation;
    out_result->physical_slot_recyclable = out_result->upstream.upstream.physical_slot_recyclable;
    return PROM_ERROR;
  }
  return PROM_OK;
}

int prom_reactor_runtime_m49a_execute_ffn_suffix(
    void* handle, const prom_m49a_ffn_suffix_request* request,
    prom_m49a_ffn_suffix_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_reduction_slot* slot;
  prom_m46_composed_result upstream_result;
  prom_m46_continuation upstream;
  prom_m47_composed_request ffn_request;
  prom_m47_continuation continuation;
  prom_vk_buffer* capture_source = NULL;
  float* dense_input = NULL;
  const float* logical_input;
  const uint64_t begin_ns = prom_reduction_now_ns();
  uint64_t input_elements;
  uint64_t storage_elements;
  uint64_t capture_elements;
  uint64_t input_bytes;
  uint64_t capture_bytes;
  uint64_t input_hash;
  uint64_t allocations_before;
  uint64_t reuses_before;
  uint64_t timestamps[PROM_M47_QUERY_BASE + PROM_M47_QUERY_COUNT];
  uint32_t source_row_stride = 0u;
  uint32_t source_columns = 0u;
  uint32_t finite = 0u;
  uint32_t row;
  uint32_t weight;
  uint32_t partial_fault = 0u;
  int32_t detail = 0;
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  VkSubmitInfo submit;
  VkResult vk_result;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  out_result->audit_only = 1u;
  out_result->no_product_intermediate_readback_change = 1u;
  if (request == NULL || request->matched_n == NULL || request->capture_output == NULL ||
      request->tokens == 0u || request->model_width == 0u || request->ffn_width == 0u ||
      request->matched_n_row_stride < request->model_width ||
      request->capture_stage < PROM_M49A_CAPTURE_GATE ||
      request->capture_stage > PROM_M49A_CAPTURE_FFN_SUFFIX ||
      request->projection_path < PROM_M47_PROJECTION_COOPERATIVE ||
      request->projection_path > PROM_M47_PROJECTION_CONVENTIONAL_FP16 ||
      (request->down_projection_path != 0u &&
       request->down_projection_path != PROM_M47_PROJECTION_A2X4_FP32) ||
      (request->down_projection_path == PROM_M47_PROJECTION_A2X4_FP32 &&
       request->gating_strategy == PROM_M47_GATING_FUSED_DIRECT_PACKED) ||
      request->gating_strategy < PROM_M47_GATING_SEPARATE ||
      request->gating_strategy > PROM_M47_GATING_FUSED_DIRECT_PACKED ||
      request->residual_strategy < PROM_M47_RESIDUAL_SEPARATE_OUTPUT ||
      request->residual_strategy > PROM_M47_RESIDUAL_IN_PLACE_DOWN ||
      request->input_generation == 0u || request->reference_input_hash == 0u ||
      request->exact_source_hash == 0u ||
      !prom_m40b_checked_product_u64(request->tokens, request->model_width,
                                     &input_elements) ||
      !prom_m40b_checked_product_u64(request->tokens, request->matched_n_row_stride,
                                     &storage_elements) ||
      request->matched_n_element_count != storage_elements) {
    out_result->detail_code = PROM_M47_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  capture_elements = request->capture_stage <= PROM_M49A_CAPTURE_HIDDEN
                         ? (uint64_t)request->tokens * request->ffn_width
                         : input_elements;
  if (request->capture_output_element_count != capture_elements ||
      capture_elements > SIZE_MAX / sizeof(float) ||
      input_elements > SIZE_MAX / sizeof(float) ||
      (request->capture_stage == PROM_M49A_CAPTURE_DOWN &&
       request->residual_strategy != PROM_M47_RESIDUAL_SEPARATE_OUTPUT) ||
      (request->capture_stage == PROM_M49A_CAPTURE_HIDDEN &&
       request->gating_strategy == PROM_M47_GATING_FUSED_DIRECT_PACKED)) {
    out_result->detail_code = PROM_M47_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  (void)prom_m42_hash_finite_matrix(request->matched_n, storage_elements, &finite);
  input_hash = prom_num_hash_float_bits(request->matched_n, storage_elements);
  out_result->input_hash = input_hash;
  if (finite == 0u || input_hash == 0u || input_hash != request->reference_input_hash) {
    out_result->detail_code = PROM_M47_DETAIL_NONFINITE_INPUT;
    return PROM_ERROR;
  }
  out_result->matched_input = 1u;
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL || !prom_m42_ensure_pipelines(state) ||
      !prom_m45_ensure_pipeline(state) || !prom_m47_ensure_pipelines(state) ||
      !prom_m40b_ensure_sgemm_pipeline(state, request->projection_path) ||
      (request->down_projection_path != 0u &&
       !prom_m40b_ensure_sgemm_pipeline(state, request->down_projection_path))) {
    out_result->detail_code = state == NULL ? detail : PROM_M47_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  for (weight = 0u; weight < PROM_M47_WEIGHT_COUNT; ++weight) {
    if (request->required_weight_generation[weight] == 0u ||
        request->required_weight_hash[weight] == 0u ||
        state->m47_weight_generation[weight] != request->required_weight_generation[weight] ||
        state->m47_weight_hash[weight] != request->required_weight_hash[weight] ||
        state->m47_weight_model_width[weight] != request->model_width ||
        state->m47_weight_ffn_width[weight] != request->ffn_width) {
      out_result->detail_code = PROM_M47_DETAIL_STALE_WEIGHT_GENERATION;
      return PROM_ERROR;
    }
  }
  slot = prom_reduction_acquire_slot(state, state->next_logical_request_id++);
  if (slot == NULL) {
    out_result->detail_code = PROM_M47_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  slot->active_query_base = slot->slot_id * PROM_REDUCTION_QUERY_STRIDE;
  allocations_before = state->m47_buffer_grow_count + state->m48_buffer_grow_count;
  reuses_before = state->m47_buffer_reuse_count + state->m48_buffer_reuse_count;
  input_bytes = input_elements * sizeof(float);
  capture_bytes = capture_elements * sizeof(float);
  if (!prom_m48_ensure_buffer(state, &slot->m48_host_initial_upload,
                              (VkDeviceSize)input_bytes,
                              VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT |
                                  VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                              1, NULL) ||
      !prom_m48_ensure_buffer(state, &slot->m48_host_initial,
                              (VkDeviceSize)input_bytes,
                              VK_BUFFER_USAGE_TRANSFER_DST_BIT |
                                  VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
      !prom_m48_ensure_buffer(state, &slot->m48_readback,
                              (VkDeviceSize)capture_bytes,
                              VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                              VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT |
                                  VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                              1, NULL)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->detail_code = PROM_M47_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  logical_input = request->matched_n;
  if (request->matched_n_row_stride != request->model_width) {
    dense_input = (float*)malloc((size_t)input_bytes);
    if (dense_input == NULL) {
      slot->state = PROM_ASYNC_PHYSICAL_READY;
      out_result->detail_code = PROM_M47_DETAIL_RESOURCE;
      return PROM_ERROR;
    }
    for (row = 0u; row < request->tokens; ++row) {
      memcpy(dense_input + (uint64_t)row * request->model_width,
             request->matched_n + (uint64_t)row * request->matched_n_row_stride,
             (size_t)((uint64_t)request->model_width * sizeof(float)));
    }
    logical_input = dense_input;
  }
  memcpy(slot->m48_host_initial_upload.mapped, logical_input, (size_t)input_bytes);
  free(dense_input);
  memset(&upstream_result, 0, sizeof(upstream_result));
  upstream_result.rmsnorm_plan.tokens = request->tokens;
  upstream_result.rmsnorm_plan.model_width = request->model_width;
  upstream_result.rmsnorm_plan.n_row_stride = request->model_width;
  upstream_result.rmsnorm_plan.n_generation = request->input_generation;
  upstream_result.rmsnorm_plan.replay_id =
      prom_m40b_hash_u64(request->exact_source_hash, input_hash);
  memset(&upstream, 0, sizeof(upstream));
  upstream.result = &upstream_result;
  upstream.n = &slot->m48_host_initial;
  memset(&ffn_request, 0, sizeof(ffn_request));
  ffn_request.ffn_width = request->ffn_width;
  ffn_request.projection_path = request->projection_path;
  ffn_request.gating_strategy = request->gating_strategy;
  ffn_request.residual_strategy = request->residual_strategy;
  ffn_request.submit_policy = PROM_M47_SUBMIT_ONE_COMMAND_BUFFER;
  for (weight = 0u; weight < PROM_M47_WEIGHT_COUNT; ++weight)
    ffn_request.required_weight_generation[weight] = request->required_weight_generation[weight];
  memset(&continuation, 0, sizeof(continuation));
  continuation.request = &ffn_request;
  continuation.result = &out_result->ffn;
  if (!prom_m47_prepare_continuation(state, slot, &upstream, &continuation)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->detail_code = PROM_M47_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  if (request->down_projection_path == PROM_M47_PROJECTION_A2X4_FP32) {
    prom_m47_update_descriptor(state, slot->m47_descriptor_sets[5],
                               &slot->m47_hidden,
                               &state->m47_weight_f32[PROM_M47_WEIGHT_DOWN],
                               &slot->m47_down, &slot->m47_down);
  }
  if (request->capture_stage == PROM_M49A_CAPTURE_GATE) {
    capture_source = &slot->m47_gate;
    source_row_stride = out_result->ffn.ffn_plan.gate_row_stride;
    source_columns = request->ffn_width;
  } else if (request->capture_stage == PROM_M49A_CAPTURE_UP) {
    capture_source = &slot->m47_up;
    source_row_stride = out_result->ffn.ffn_plan.up_row_stride;
    source_columns = request->ffn_width;
  } else if (request->capture_stage == PROM_M49A_CAPTURE_HIDDEN) {
    capture_source = &slot->m47_hidden;
    source_row_stride = out_result->ffn.ffn_plan.hidden_row_stride;
    source_columns = request->ffn_width;
  } else if (request->capture_stage == PROM_M49A_CAPTURE_DOWN) {
    capture_source = &slot->m47_down;
    source_row_stride = out_result->ffn.ffn_plan.down_row_stride;
    source_columns = request->model_width;
  } else {
    capture_source = continuation.output;
    source_row_stride = out_result->ffn.ffn_plan.output_row_stride;
    source_columns = request->model_width;
  }
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkResetCommandBuffer(slot->command_buffer, 0u) != VK_SUCCESS ||
      vkBeginCommandBuffer(slot->command_buffer, &begin_info) != VK_SUCCESS)
    goto m49a_ffn_command_fail;
  prom_m42_buffer_barrier(slot->command_buffer, &slot->m48_host_initial_upload,
                          VK_ACCESS_HOST_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                          VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
  memset(&copy, 0, sizeof(copy));
  copy.size = (VkDeviceSize)input_bytes;
  vkCmdCopyBuffer(slot->command_buffer, slot->m48_host_initial_upload.buffer,
                  slot->m48_host_initial.buffer, 1u, &copy);
  if (!prom_m43_one_buffer_barrier(slot->command_buffer, &slot->m48_host_initial,
                                   (VkDeviceSize)input_bytes,
                                   VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT) ||
      prom_m47_record_tail(state, slot, &ffn_request, &out_result->ffn.ffn_plan,
                           &slot->m48_host_initial, continuation.output,
                           slot->command_buffer, 1u, 1u,
                           request->down_projection_path, &partial_fault) != 1 ||
      partial_fault != 0u ||
      !prom_m43_one_buffer_barrier(slot->command_buffer, capture_source,
                                   capture_source->size,
                                   VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT,
                                   VK_ACCESS_TRANSFER_READ_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT) ||
      !prom_m43_one_buffer_barrier(slot->command_buffer, &slot->m48_readback,
                                   (VkDeviceSize)capture_bytes,
                                   VK_ACCESS_HOST_READ_BIT,
                                   VK_ACCESS_TRANSFER_WRITE_BIT,
                                   VK_PIPELINE_STAGE_HOST_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT))
    goto m49a_ffn_command_fail;
  for (row = 0u; row < request->tokens; ++row) {
    memset(&copy, 0, sizeof(copy));
    copy.srcOffset = (VkDeviceSize)((uint64_t)row * source_row_stride * sizeof(float));
    copy.dstOffset = (VkDeviceSize)((uint64_t)row * source_columns * sizeof(float));
    copy.size = (VkDeviceSize)((uint64_t)source_columns * sizeof(float));
    vkCmdCopyBuffer(slot->command_buffer, capture_source->buffer,
                    slot->m48_readback.buffer, 1u, &copy);
  }
  if (!prom_m43_one_buffer_barrier(slot->command_buffer, capture_source,
                                   capture_source->size,
                                   VK_ACCESS_TRANSFER_READ_BIT,
                                   VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT) ||
      !prom_m43_one_buffer_barrier(slot->command_buffer, &slot->m48_readback,
                                   (VkDeviceSize)capture_bytes,
                                   VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_HOST_READ_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT,
                                   VK_PIPELINE_STAGE_HOST_BIT) ||
      vkEndCommandBuffer(slot->command_buffer) != VK_SUCCESS ||
      vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS)
    goto m49a_ffn_command_fail;
  memset(&submit, 0, sizeof(submit));
  submit.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit.commandBufferCount = 1u;
  submit.pCommandBuffers = &slot->command_buffer;
  vk_result = vkQueueSubmit(state->queue, 1u, &submit, slot->fence);
  if (vk_result != VK_SUCCESS) goto m49a_ffn_submit_fail;
  slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
  vk_result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
  if (vk_result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    out_result->detail_code = PROM_M47_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  memset(timestamps, 0, sizeof(timestamps));
  if (state->timestamp_supported == 0u || state->query_pool == VK_NULL_HANDLE ||
      vkGetQueryPoolResults(state->device, state->query_pool,
                            slot->active_query_base + PROM_M47_QUERY_BASE,
                            16u, 16u * sizeof(uint64_t),
                            &timestamps[PROM_M47_QUERY_BASE],
                            sizeof(uint64_t), VK_QUERY_RESULT_64_BIT) != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->detail_code = PROM_M47_DETAIL_QUERY;
    return PROM_ERROR;
  }
  if (!prom_m47_complete_continuation(state, slot, &continuation, timestamps, begin_ns)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->detail_code = PROM_M47_DETAIL_QUERY;
    return PROM_ERROR;
  }
  memcpy(request->capture_output, slot->m48_readback.mapped, (size_t)capture_bytes);
  (void)prom_m42_hash_finite_matrix(request->capture_output, capture_elements, &finite);
  out_result->capture_hash =
      prom_num_hash_float_bits(request->capture_output, capture_elements);
  if (finite == 0u || out_result->capture_hash == 0u) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->detail_code = PROM_M47_DETAIL_READBACK;
    return PROM_ERROR;
  }
  out_result->capture_stage = request->capture_stage;
  out_result->down_projection_path = request->down_projection_path != 0u
                                         ? request->down_projection_path
                                         : request->projection_path;
  out_result->submit_count = 1u;
  out_result->final_readback_count = 1u;
  out_result->intermediate_host_copy_count = 1u;
  out_result->buffer_allocation_count =
      state->m47_buffer_grow_count + state->m48_buffer_grow_count - allocations_before;
  out_result->buffer_reuse_count =
      state->m47_buffer_reuse_count + state->m48_buffer_reuse_count - reuses_before;
  out_result->replay_identity =
      prom_m40b_hash_u64(out_result->ffn.ffn_plan.replay_id, input_hash);
  out_result->replay_identity =
      prom_m40b_hash_u64(out_result->replay_identity, request->capture_stage);
  out_result->replay_identity =
      prom_m40b_hash_u64(out_result->replay_identity,
                         out_result->down_projection_path);
  out_result->replay_identity =
      prom_m40b_hash_u64(out_result->replay_identity, request->exact_source_hash);
  out_result->end_to_end_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  return PROM_OK;

m49a_ffn_command_fail:
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->detail_code = PROM_M47_DETAIL_COMMAND;
  return PROM_ERROR;
m49a_ffn_submit_fail:
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->detail_code = PROM_M47_DETAIL_SUBMIT;
  return PROM_ERROR;
}

int prom_reactor_runtime_m49a_execute_m46(
    void* handle, const prom_m49a_m46_request* request,
    prom_m49a_m46_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_reduction_slot* slot;
  prom_m45_composed_result upstream;
  prom_m46_composed_request rms_request;
  prom_m46_continuation continuation;
  const uint64_t begin_ns = prom_reduction_now_ns();
  uint64_t storage_elements;
  uint64_t logical_elements;
  uint64_t input_bytes;
  uint64_t output_bytes;
  uint64_t inv_rms_bytes;
  uint64_t input_hash;
  uint64_t timestamps[PROM_M46_QUERY_BASE + PROM_M46_QUERY_COUNT];
  uint32_t finite = 0u;
  uint32_t partial_fault = 0u;
  int32_t detail = 0;
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  VkSubmitInfo submit;
  VkResult vk_result;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  out_result->audit_only = 1u;
  out_result->no_product_intermediate_readback_change = 1u;
  if (request == NULL || request->matched_z == NULL || request->output == NULL ||
      request->inv_rms_output == NULL || request->tokens == 0u ||
      request->model_width == 0u || request->z_row_stride < request->model_width ||
      request->strategy < PROM_M46_STRATEGY_SEPARATE_OUTPUT ||
      request->strategy > PROM_M46_STRATEGY_IN_PLACE_Z ||
      request->requested_reduction_plan > PROM_M46_REDUCTION_FORCE_STAGED ||
      !isfinite(request->epsilon) || request->epsilon <= 0.0f ||
      request->input_generation == 0u || request->reference_input_hash == 0u ||
      request->required_weight_generation == 0u || request->required_weight_hash == 0u ||
      request->exact_source_hash == 0u ||
      !prom_m40b_checked_product_u64(request->tokens, request->z_row_stride,
                                     &storage_elements) ||
      !prom_m40b_checked_product_u64(request->tokens, request->model_width,
                                     &logical_elements) ||
      request->matched_storage_element_count != storage_elements ||
      request->output_element_count != logical_elements ||
      request->inv_rms_output_element_count != request->tokens ||
      storage_elements > SIZE_MAX / sizeof(float) || logical_elements > SIZE_MAX / sizeof(float)) {
    out_result->detail_code = PROM_M46_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  (void)prom_m42_hash_finite_matrix(request->matched_z, storage_elements, &finite);
  input_hash = prom_num_hash_float_bits(request->matched_z, storage_elements);
  out_result->input_hash = input_hash;
  if (finite == 0u || input_hash == 0u || input_hash != request->reference_input_hash) {
    out_result->detail_code = PROM_M46_DETAIL_NONFINITE_INPUT;
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL || !prom_m46_ensure_pipelines(state)) {
    out_result->detail_code = state == NULL ? detail : PROM_M46_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  if (state->m46_weight_generation != request->required_weight_generation ||
      state->m46_weight_hash != request->required_weight_hash ||
      state->m46_weight_model_width != request->model_width) {
    out_result->detail_code = PROM_M46_DETAIL_STALE_WEIGHT_GENERATION;
    return PROM_ERROR;
  }
  slot = prom_reduction_acquire_slot(state, state->next_logical_request_id++);
  if (slot == NULL) {
    out_result->detail_code = PROM_M46_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  slot->active_query_base = slot->slot_id * PROM_REDUCTION_QUERY_STRIDE;
  input_bytes = storage_elements * sizeof(float);
  output_bytes = logical_elements * sizeof(float);
  inv_rms_bytes = (uint64_t)request->tokens * sizeof(float);
  if (!prom_m48_ensure_buffer(state, &slot->m48_host_initial_upload,
                              (VkDeviceSize)input_bytes,
                              VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                              VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT |
                                  VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                              1, NULL) ||
      !prom_m48_ensure_buffer(state, &slot->m49a_m46_z,
                              (VkDeviceSize)input_bytes,
                              VK_BUFFER_USAGE_TRANSFER_DST_BIT |
                                  VK_BUFFER_USAGE_TRANSFER_SRC_BIT |
                                  VK_BUFFER_USAGE_STORAGE_BUFFER_BIT,
                              VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, NULL) ||
      !prom_m48_ensure_buffer(state, &slot->m48_readback,
                              (VkDeviceSize)inv_rms_bytes,
                              VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                              VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT |
                                  VK_MEMORY_PROPERTY_HOST_COHERENT_BIT,
                              1, NULL)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->detail_code = PROM_M46_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  memcpy(slot->m48_host_initial_upload.mapped, request->matched_z, (size_t)input_bytes);
  memset(&upstream, 0, sizeof(upstream));
  upstream.logical_request_id = state->next_logical_request_id - 1u;
  upstream.residual_plan.tokens = request->tokens;
  upstream.residual_plan.model_width = request->model_width;
  upstream.residual_plan.z_row_stride = request->z_row_stride;
  upstream.residual_plan.z_generation = request->input_generation;
  upstream.residual_plan.replay_id =
      prom_m40b_hash_u64(request->exact_source_hash, input_hash);
  upstream.z_view.buffer = slot->m49a_m46_z.buffer;
  upstream.z_view.byte_length = slot->m49a_m46_z.size;
  upstream.z_view.element_type = PROM_DEVICE_ELEMENT_F32;
  upstream.z_view.logical_rows = request->tokens;
  upstream.z_view.logical_columns = request->model_width;
  upstream.z_view.row_stride_elements = request->z_row_stride;
  upstream.z_view.layout = PROM_DEVICE_LAYOUT_ROW_MAJOR;
  upstream.z_view.producer_access = PROM_DEVICE_ACCESS_COMPUTE_WRITE;
  upstream.z_view.required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
  upstream.z_view.owning_device = state->device;
  upstream.z_view.owning_lifetime_id = request->input_generation;
  upstream.z_view.owning_slot_id = slot->slot_id;
  upstream.z_view.owning_slot_generation = slot->generation;
  memset(&rms_request, 0, sizeof(rms_request));
  rms_request.upstream.attention.tokens = request->tokens;
  rms_request.upstream.attention.model_width = request->model_width;
  rms_request.output = request->output;
  rms_request.output_element_count = logical_elements;
  rms_request.strategy = request->strategy;
  rms_request.submit_policy = PROM_M46_SUBMIT_ONE_COMMAND_BUFFER;
  rms_request.requested_reduction_plan = request->requested_reduction_plan;
  rms_request.required_weight_generation = request->required_weight_generation;
  rms_request.epsilon = request->epsilon;
  memset(&continuation, 0, sizeof(continuation));
  continuation.request = &rms_request;
  continuation.result = &out_result->rmsnorm;
  if (!prom_m46_prepare_continuation(state, slot, &upstream, &continuation)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->detail_code = PROM_M46_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkResetCommandBuffer(slot->command_buffer, 0u) != VK_SUCCESS ||
      vkBeginCommandBuffer(slot->command_buffer, &begin_info) != VK_SUCCESS)
    goto m49a_m46_command_fail;
  prom_m42_buffer_barrier(slot->command_buffer, &slot->m48_host_initial_upload,
                          VK_ACCESS_HOST_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                          VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
  memset(&copy, 0, sizeof(copy));
  copy.size = (VkDeviceSize)input_bytes;
  vkCmdCopyBuffer(slot->command_buffer, slot->m48_host_initial_upload.buffer,
                  slot->m49a_m46_z.buffer, 1u, &copy);
  if (!prom_m43_one_buffer_barrier(slot->command_buffer, &slot->m49a_m46_z,
                                   (VkDeviceSize)input_bytes,
                                   VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT) ||
      prom_m46_record_tail(state, slot, &rms_request,
                           &out_result->rmsnorm.rmsnorm_plan,
                           continuation.z, continuation.n,
                           slot->command_buffer, 1u, 1u, &partial_fault) != 1 ||
      partial_fault != 0u ||
      !prom_m43_one_buffer_barrier(slot->command_buffer, &slot->m46_inv_rms,
                                   (VkDeviceSize)inv_rms_bytes,
                                   VK_ACCESS_SHADER_READ_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                                   VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT) ||
      !prom_m43_one_buffer_barrier(slot->command_buffer, &slot->m48_readback,
                                   (VkDeviceSize)inv_rms_bytes,
                                   VK_ACCESS_HOST_READ_BIT, VK_ACCESS_TRANSFER_WRITE_BIT,
                                   VK_PIPELINE_STAGE_HOST_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT))
    goto m49a_m46_command_fail;
  memset(&copy, 0, sizeof(copy));
  copy.size = (VkDeviceSize)inv_rms_bytes;
  vkCmdCopyBuffer(slot->command_buffer, slot->m46_inv_rms.buffer,
                  slot->m48_readback.buffer, 1u, &copy);
  if (!prom_m43_one_buffer_barrier(slot->command_buffer, &slot->m48_readback,
                                   (VkDeviceSize)inv_rms_bytes,
                                   VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_HOST_READ_BIT,
                                   VK_PIPELINE_STAGE_TRANSFER_BIT,
                                   VK_PIPELINE_STAGE_HOST_BIT) ||
      vkEndCommandBuffer(slot->command_buffer) != VK_SUCCESS ||
      vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS)
    goto m49a_m46_command_fail;
  memset(&submit, 0, sizeof(submit));
  submit.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit.commandBufferCount = 1u;
  submit.pCommandBuffers = &slot->command_buffer;
  vk_result = vkQueueSubmit(state->queue, 1u, &submit, slot->fence);
  if (vk_result != VK_SUCCESS) goto m49a_m46_submit_fail;
  slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
  vk_result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
  if (vk_result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    out_result->detail_code = PROM_M46_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  memset(timestamps, 0, sizeof(timestamps));
  if (state->timestamp_supported == 0u || state->query_pool == VK_NULL_HANDLE ||
      vkGetQueryPoolResults(state->device, state->query_pool,
                            slot->active_query_base + PROM_M46_QUERY_BASE,
                            PROM_M46_QUERY_COUNT,
                            PROM_M46_QUERY_COUNT * sizeof(uint64_t),
                            &timestamps[PROM_M46_QUERY_BASE], sizeof(uint64_t),
                            VK_QUERY_RESULT_64_BIT) != VK_SUCCESS ||
      !prom_m46_complete_continuation(state, slot, &upstream, &continuation,
                                      timestamps, begin_ns)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->detail_code = PROM_M46_DETAIL_QUERY;
    return PROM_ERROR;
  }
  memcpy(request->inv_rms_output, slot->m48_readback.mapped, (size_t)inv_rms_bytes);
  (void)prom_m42_hash_finite_matrix(request->output, logical_elements, &finite);
  out_result->output_hash = prom_num_hash_float_bits(request->output, logical_elements);
  if (finite == 0u || out_result->output_hash == 0u) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->detail_code = PROM_M46_DETAIL_READBACK;
    return PROM_ERROR;
  }
  (void)prom_m42_hash_finite_matrix(request->inv_rms_output, request->tokens, &finite);
  out_result->inv_rms_hash =
      prom_num_hash_float_bits(request->inv_rms_output, request->tokens);
  if (finite == 0u || out_result->inv_rms_hash == 0u) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->detail_code = PROM_M46_DETAIL_READBACK;
    return PROM_ERROR;
  }
  out_result->matched_input = 1u;
  out_result->input_generation = request->input_generation;
  out_result->weight_generation = request->required_weight_generation;
  out_result->weight_hash = request->required_weight_hash;
  out_result->replay_identity =
      prom_m40b_hash_u64(out_result->rmsnorm.rmsnorm_plan.replay_id, input_hash);
  out_result->replay_identity =
      prom_m40b_hash_u64(out_result->replay_identity, request->exact_source_hash);
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  return PROM_OK;

m49a_m46_command_fail:
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->detail_code = PROM_M46_DETAIL_COMMAND;
  return PROM_ERROR;
m49a_m46_submit_fail:
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->detail_code = PROM_M46_DETAIL_SUBMIT;
  return PROM_ERROR;
}

int prom_reactor_runtime_m44_execute_host_bounce(void* handle,
                                                 const prom_m44_host_bounce_request* request,
                                                 prom_m44_host_bounce_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_reduction_slot* slot;
  prom_vk_runtime_services services;
  prom_m44_plan_request plan_request;
  prom_m44_output_projection_plan plan;
  prom_m44_host_bounce_request effective_request;
  float* concatenated = NULL;
  void* payload = NULL;
  size_t payload_bytes = 0u;
  uint64_t head_elements;
  uint64_t output_elements;
  uint64_t concat_elements;
  uint64_t begin_ns = prom_reduction_now_ns();
  uint64_t cpu_begin;
  uint64_t submit_begin;
  uint64_t readback_begin;
  uint64_t timestamps[PROM_M44_QUERY_COUNT];
  uint64_t hash = 1469598103934665603ull;
  uint32_t concatenated_width;
  uint32_t head;
  uint32_t row;
  uint32_t kernel;
  int32_t detail = 0;
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  VkSubmitInfo submit;
  VkResult result;
  const prom_vk_buffer* input_buffer;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  out_result->physical_slot_id = UINT32_MAX;
  if (request == NULL || request->head_major == NULL || request->output == NULL ||
      request->head_count != PROM_M44_HEAD_COUNT || request->tokens == 0u ||
      request->head_dim == 0u || request->model_width == 0u ||
      request->projection_path < PROM_M44_PROJECTION_COOPERATIVE ||
      request->projection_path > PROM_M44_PROJECTION_CONVENTIONAL_FP16 ||
      request->required_wo_generation == 0u || request->m43_aggregate_replay_id == 0u ||
      !prom_vk_checked_mul_u32(PROM_M44_HEAD_COUNT, request->head_dim, &concatenated_width) ||
      !prom_m40b_checked_product_u64(PROM_M44_HEAD_COUNT, request->tokens, &head_elements) ||
      !prom_m40b_checked_product_u64(head_elements, request->head_dim, &head_elements) ||
      !prom_m40b_checked_product_u64(request->tokens, request->model_width, &output_elements) ||
      !prom_m40b_checked_product_u64(request->tokens, concatenated_width, &concat_elements) ||
      request->head_major_element_count != head_elements ||
      request->output_element_count != output_elements || concat_elements > SIZE_MAX / sizeof(float)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M44_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL || prom_reactor_runtime_get_vk_services(handle, &services) != PROM_OK) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = state == NULL ? detail : PROM_M44_DETAIL_CAPABILITY;
    return PROM_ERROR;
  }
  if (state->m44_wo_generation == 0u || state->m44_wo_generation != request->required_wo_generation ||
      state->m44_wo_head_dim != request->head_dim ||
      state->m44_wo_model_width != request->model_width) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M44_DETAIL_STALE_WO_GENERATION;
    return PROM_ERROR;
  }
  effective_request = *request;
  if (effective_request.projection_path == PROM_M44_PROJECTION_COOPERATIVE &&
      (services.cooperative_matrix_feature_enabled == 0u || services.subgroup_size != 32u))
    effective_request.projection_path = PROM_M44_PROJECTION_CONVENTIONAL_FP16;
  if ((effective_request.projection_path == PROM_M44_PROJECTION_A2X4_FP32 &&
       effective_request.precision_policy != PROM_M42_PRECISION_FP32) ||
      (effective_request.projection_path != PROM_M44_PROJECTION_A2X4_FP32 &&
       effective_request.precision_policy != PROM_M42_PRECISION_F16_ROUNDED)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M44_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  if (!prom_m42_finite_matrix(effective_request.head_major, head_elements) ||
      !prom_m42_ensure_pipelines(state) ||
      !prom_m40b_ensure_sgemm_pipeline(state, effective_request.projection_path)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M44_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  out_result->logical_request_id = state->next_logical_request_id++;
  state->diagnostics.next_logical_request_id = state->next_logical_request_id;
  slot = prom_reduction_acquire_slot(state, out_result->logical_request_id);
  if (slot == NULL) {
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M44_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  out_result->physical_slot_id = slot->slot_id;
  out_result->physical_slot_generation = slot->generation;
  memset(&plan_request, 0, sizeof(plan_request));
  for (head = 0u; head < PROM_M44_HEAD_COUNT; ++head) {
    plan_request.head_views[head].buffer = (VkBuffer)(uintptr_t)(head + 1u);
    plan_request.head_views[head].byte_length =
        (VkDeviceSize)((uint64_t)request->tokens * request->head_dim * sizeof(float));
    plan_request.head_views[head].element_type = PROM_DEVICE_ELEMENT_F32;
    plan_request.head_views[head].logical_rows = request->tokens;
    plan_request.head_views[head].logical_columns = request->head_dim;
    plan_request.head_views[head].row_stride_elements = request->head_dim;
    plan_request.head_views[head].layout = PROM_DEVICE_LAYOUT_ROW_MAJOR;
    plan_request.head_views[head].producer_access = PROM_DEVICE_ACCESS_COMPUTE_WRITE;
    plan_request.head_views[head].required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
    plan_request.head_views[head].owning_device = state->device;
    plan_request.head_views[head].owning_lifetime_id = out_result->logical_request_id;
    plan_request.head_views[head].owning_slot_id = slot->slot_id;
    plan_request.head_views[head].owning_slot_generation = slot->generation;
  }
  plan_request.head_count = PROM_M44_HEAD_COUNT;
  plan_request.tokens = request->tokens;
  plan_request.head_dim = request->head_dim;
  plan_request.model_width = request->model_width;
  plan_request.precision_policy = request->precision_policy;
  plan_request.aggregation_strategy = PROM_M44_AGGREGATION_INTERLEAVE;
  plan_request.projection_path = effective_request.projection_path;
  plan_request.submit_plan = PROM_M44_SUBMIT_ONE_COMMAND_BUFFER;
  plan_request.cooperative_capability_state = services.cooperative_matrix_state;
  plan_request.wo_generation = state->m44_wo_generation;
  plan_request.wo_hash = state->m44_wo_hash;
  plan_request.m43_aggregate_replay_id = request->m43_aggregate_replay_id;
  if (prom_m44_output_projection_plan_build(&plan_request, &plan) != PROM_OK ||
      !prom_m44_prepare_execution_buffers(state, slot, &plan, 1u, 1u)) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M44_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  concatenated = (float*)malloc((size_t)(concat_elements * sizeof(float)));
  if (concatenated == NULL) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M44_DETAIL_RESOURCE;
    return PROM_ERROR;
  }
  cpu_begin = prom_reduction_now_ns();
  for (row = 0u; row < request->tokens; ++row) {
    for (head = 0u; head < PROM_M44_HEAD_COUNT; ++head) {
      memcpy(concatenated + (uint64_t)row * concatenated_width + (uint64_t)head * request->head_dim,
             request->head_major + ((uint64_t)head * request->tokens + row) * request->head_dim,
             (size_t)((uint64_t)request->head_dim * sizeof(float)));
    }
  }
  out_result->cpu_concatenate_ns = prom_reduction_elapsed_ns(cpu_begin, prom_reduction_now_ns());
  kernel = effective_request.projection_path == PROM_M44_PROJECTION_A2X4_FP32
               ? PROM_M40B_KERNEL_A2X4
               : PROM_M40B_KERNEL_CONVENTIONAL_FP16;
  cpu_begin = prom_reduction_now_ns();
  if (!prom_m40b_pack_matrix(concatenated, request->tokens, concatenated_width,
                             effective_request.projection_path == PROM_M44_PROJECTION_A2X4_FP32
                                 ? request->tokens
                                 : plan.padded_tokens,
                             effective_request.projection_path == PROM_M44_PROJECTION_A2X4_FP32
                                 ? concatenated_width
                                 : plan.padded_concatenated_width,
                             kernel, &payload, &payload_bytes)) {
    free(concatenated);
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M44_DETAIL_NONFINITE_INPUT;
    return PROM_ERROR;
  }
  out_result->cpu_pack_ns = prom_reduction_elapsed_ns(cpu_begin, prom_reduction_now_ns());
  free(concatenated);
  if (payload_bytes > slot->m44_concat_upload.size) {
    free(payload);
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_TRANSFER_IN;
    out_result->detail_code = PROM_M44_DETAIL_CAPACITY;
    return PROM_ERROR;
  }
  memcpy(slot->m44_concat_upload.mapped, payload, payload_bytes);
  free(payload);
  input_buffer = effective_request.projection_path == PROM_M44_PROJECTION_A2X4_FP32
                     ? &slot->m44_concat_f32
                     : &slot->m44_concat_f16;
  prom_m44_update_sgemm_descriptor(state, slot->m44_sgemm_descriptor_set, input_buffer,
                                   effective_request.projection_path == PROM_M44_PROJECTION_A2X4_FP32
                                       ? &state->m44_wo_f32
                                       : &state->m44_wo_f16,
                                   &slot->m44_output);
  if (vkResetCommandBuffer(slot->command_buffer, 0u) != VK_SUCCESS) goto m44_host_command_fail;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(slot->command_buffer, &begin_info) != VK_SUCCESS) goto m44_host_command_fail;
  if (state->timestamp_supported != 0u && state->query_pool != VK_NULL_HANDLE)
    vkCmdResetQueryPool(slot->command_buffer, state->query_pool,
                        slot->slot_id * PROM_REDUCTION_QUERY_STRIDE + PROM_M44_QUERY_BASE,
                        PROM_M44_QUERY_COUNT);
  prom_m42_write_timestamp(state, slot, slot->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                           PROM_M44_QUERY_AGGREGATION_BEGIN);
  prom_m42_buffer_barrier(slot->command_buffer, &slot->m44_concat_upload,
                          VK_ACCESS_HOST_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                          VK_PIPELINE_STAGE_HOST_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
  memset(&copy, 0, sizeof(copy));
  copy.size = (VkDeviceSize)payload_bytes;
  vkCmdCopyBuffer(slot->command_buffer, slot->m44_concat_upload.buffer,
                  input_buffer->buffer, 1u, &copy);
  prom_m42_buffer_barrier(slot->command_buffer, input_buffer,
                          VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT,
                          VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
  prom_m42_write_timestamp(state, slot, slot->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                           PROM_M44_QUERY_AGGREGATION_END);
  prom_m42_write_timestamp(state, slot, slot->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                           PROM_M44_QUERY_PROJECTION_BEGIN);
  prom_m42_record_sgemm(state, slot->command_buffer, slot->m44_sgemm_descriptor_set,
                        effective_request.projection_path,
                        effective_request.projection_path == PROM_M44_PROJECTION_A2X4_FP32
                            ? request->tokens
                            : plan.padded_tokens,
                        effective_request.projection_path == PROM_M44_PROJECTION_A2X4_FP32
                            ? request->model_width
                            : plan.padded_model_width,
                        effective_request.projection_path == PROM_M44_PROJECTION_A2X4_FP32
                            ? concatenated_width
                            : plan.padded_concatenated_width);
  prom_m42_write_timestamp(state, slot, slot->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                           PROM_M44_QUERY_PROJECTION_END);
  prom_m42_write_timestamp(state, slot, slot->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                           PROM_M44_QUERY_ACCUMULATION_BEGIN);
  prom_m42_write_timestamp(state, slot, slot->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                           PROM_M44_QUERY_ACCUMULATION_END);
  prom_m42_buffer_barrier(slot->command_buffer, &slot->m44_output,
                          VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT,
                          VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT);
  prom_m42_write_timestamp(state, slot, slot->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                           PROM_M44_QUERY_READBACK_BEGIN);
  for (row = 0u; row < request->tokens; ++row) {
    memset(&copy, 0, sizeof(copy));
    copy.srcOffset = (VkDeviceSize)((uint64_t)row * plan.output_row_stride * sizeof(float));
    copy.dstOffset = (VkDeviceSize)((uint64_t)row * request->model_width * sizeof(float));
    copy.size = (VkDeviceSize)((uint64_t)request->model_width * sizeof(float));
    vkCmdCopyBuffer(slot->command_buffer, slot->m44_output.buffer,
                    slot->m44_readback.buffer, 1u, &copy);
  }
  prom_m42_write_timestamp(state, slot, slot->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                           PROM_M44_QUERY_READBACK_END);
  prom_m42_buffer_barrier(slot->command_buffer, &slot->m44_readback,
                          VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_HOST_READ_BIT,
                          VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_HOST_BIT);
  if (vkEndCommandBuffer(slot->command_buffer) != VK_SUCCESS ||
      vkResetFences(state->device, 1u, &slot->fence) != VK_SUCCESS) goto m44_host_command_fail;
  memset(&submit, 0, sizeof(submit));
  submit.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit.commandBufferCount = 1u;
  submit.pCommandBuffers = &slot->command_buffer;
  submit_begin = prom_reduction_now_ns();
  result = vkQueueSubmit(state->queue, 1u, &submit, slot->fence);
  if (result != VK_SUCCESS) goto m44_host_submit_fail;
  slot->state = PROM_ASYNC_PHYSICAL_SUBMITTED;
  result = vkWaitForFences(state->device, 1u, &slot->fence, VK_TRUE, UINT64_MAX);
  if (result != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_QUARANTINED;
    state->diagnostics.quarantine_count += 1u;
    out_result->stage = PROM_STAGE_SUBMIT;
    out_result->detail_code = PROM_M44_DETAIL_COMPLETION_UNCERTAIN;
    return PROM_ERROR;
  }
  memset(timestamps, 0, sizeof(timestamps));
  if (state->timestamp_supported == 0u || state->query_pool == VK_NULL_HANDLE ||
      vkGetQueryPoolResults(state->device, state->query_pool,
                            slot->slot_id * PROM_REDUCTION_QUERY_STRIDE + PROM_M44_QUERY_BASE,
                            PROM_M44_QUERY_COUNT, sizeof(timestamps), timestamps,
                            sizeof(uint64_t), VK_QUERY_RESULT_64_BIT) != VK_SUCCESS) {
    slot->state = PROM_ASYNC_PHYSICAL_READY;
    out_result->physical_slot_recyclable = 1u;
    out_result->stage = PROM_STAGE_TRANSFER_OUT;
    out_result->detail_code = PROM_M44_DETAIL_QUERY;
    return PROM_ERROR;
  }
  out_result->upload_gpu_ns =
      (uint64_t)((double)(timestamps[1] - timestamps[0]) * state->timestamp_period_ns);
  out_result->projection_gpu_ns =
      (uint64_t)((double)(timestamps[3] - timestamps[2]) * state->timestamp_period_ns);
  readback_begin = prom_reduction_now_ns();
  memcpy(request->output, slot->m44_readback.mapped,
         (size_t)((uint64_t)request->tokens * request->model_width * sizeof(float)));
  out_result->final_readback_ns =
      (uint64_t)((double)(timestamps[7] - timestamps[6]) * state->timestamp_period_ns) +
      prom_reduction_elapsed_ns(readback_begin, prom_reduction_now_ns());
  hash = prom_m40b_hash_u64(hash, plan.replay_id);
  hash = prom_reduction_hash_u32(hash, effective_request.projection_path);
  hash = prom_m40b_hash_u64(hash, request->m43_aggregate_replay_id);
  out_result->replay_id = hash;
  out_result->retained_bytes = prom_m44_retained_bytes(state, slot);
  out_result->submit_count = 1u;
  out_result->final_readback_count = 1u;
  out_result->intermediate_host_copy_count = 1u;
  out_result->end_to_end_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->physical_slot_recyclable = 1u;
  return PROM_OK;

m44_host_command_fail:
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->physical_slot_recyclable = 1u;
  out_result->stage = PROM_STAGE_SUBMIT;
  out_result->detail_code = PROM_M44_DETAIL_COMMAND;
  return PROM_ERROR;
m44_host_submit_fail:
  slot->state = PROM_ASYNC_PHYSICAL_READY;
  out_result->physical_slot_recyclable = 1u;
  out_result->stage = PROM_STAGE_SUBMIT;
  out_result->detail_code = PROM_M44_DETAIL_SUBMIT;
  return PROM_ERROR;
}

int prom_reactor_runtime_m49a_execute_m44(
    void* handle, const prom_m49a_m44_request* request,
    prom_m49a_m44_result* out_result) {
  prom_reduction_runtime_state* state;
  prom_m44_host_bounce_request projection;
  float* dense = NULL;
  const float* logical_input;
  uint64_t storage_elements;
  uint64_t logical_elements;
  uint64_t output_elements;
  uint64_t input_hash;
  uint32_t finite = 0u;
  uint32_t head;
  uint32_t row;
  int32_t detail = 0;
  int status;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  out_result->audit_only = 1u;
  out_result->no_product_intermediate_readback_change = 1u;
  if (request == NULL || request->matched_head_major == NULL || request->output == NULL ||
      request->head_count != PROM_M44_HEAD_COUNT || request->tokens == 0u ||
      request->head_dim == 0u || request->head_row_stride < request->head_dim ||
      request->model_width == 0u || request->input_generation == 0u ||
      request->reference_input_hash == 0u || request->required_wo_generation == 0u ||
      request->required_wo_hash == 0u || request->exact_source_hash == 0u ||
      request->projection_path < PROM_M44_PROJECTION_COOPERATIVE ||
      request->projection_path > PROM_M44_PROJECTION_CONVENTIONAL_FP16 ||
      !prom_m40b_checked_product_u64(request->head_count, request->tokens,
                                     &storage_elements) ||
      !prom_m40b_checked_product_u64(storage_elements, request->head_row_stride,
                                     &storage_elements) ||
      !prom_m40b_checked_product_u64(request->head_count, request->tokens,
                                     &logical_elements) ||
      !prom_m40b_checked_product_u64(logical_elements, request->head_dim,
                                     &logical_elements) ||
      !prom_m40b_checked_product_u64(request->tokens, request->model_width,
                                     &output_elements) ||
      request->matched_storage_element_count != storage_elements ||
      request->output_element_count != output_elements || logical_elements > SIZE_MAX / sizeof(float)) {
    out_result->detail_code = PROM_M44_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  (void)prom_m42_hash_finite_matrix(request->matched_head_major, storage_elements, &finite);
  input_hash = prom_num_hash_float_bits(request->matched_head_major, storage_elements);
  out_result->input_hash = input_hash;
  if (finite == 0u || input_hash == 0u || input_hash != request->reference_input_hash) {
    out_result->detail_code = PROM_M44_DETAIL_NONFINITE_INPUT;
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL || state->m44_wo_generation != request->required_wo_generation ||
      state->m44_wo_hash != request->required_wo_hash) {
    out_result->detail_code = state == NULL ? detail : PROM_M44_DETAIL_STALE_WO_GENERATION;
    return PROM_ERROR;
  }
  logical_input = request->matched_head_major;
  if (request->head_row_stride != request->head_dim) {
    dense = (float*)malloc((size_t)(logical_elements * sizeof(float)));
    if (dense == NULL) {
      out_result->detail_code = PROM_M44_DETAIL_RESOURCE;
      return PROM_ERROR;
    }
    for (head = 0u; head < request->head_count; ++head) {
      for (row = 0u; row < request->tokens; ++row) {
        memcpy(dense + ((uint64_t)head * request->tokens + row) * request->head_dim,
               request->matched_head_major +
                   ((uint64_t)head * request->tokens + row) * request->head_row_stride,
               (size_t)((uint64_t)request->head_dim * sizeof(float)));
      }
    }
    logical_input = dense;
  }
  memset(&projection, 0, sizeof(projection));
  projection.head_major = logical_input;
  projection.head_major_element_count = logical_elements;
  projection.output = request->output;
  projection.output_element_count = output_elements;
  projection.head_count = request->head_count;
  projection.tokens = request->tokens;
  projection.head_dim = request->head_dim;
  projection.model_width = request->model_width;
  projection.precision_policy = request->precision_policy;
  projection.projection_path = request->projection_path;
  projection.required_wo_generation = request->required_wo_generation;
  projection.m43_aggregate_replay_id =
      prom_m40b_hash_u64(request->exact_source_hash, input_hash);
  status = prom_reactor_runtime_m44_execute_host_bounce(handle, &projection,
                                                        &out_result->projection);
  free(dense);
  if (status != PROM_OK) {
    out_result->stage = out_result->projection.stage;
    out_result->detail_code = out_result->projection.detail_code;
    return PROM_ERROR;
  }
  (void)prom_m42_hash_finite_matrix(request->output, output_elements, &finite);
  out_result->output_hash = prom_num_hash_float_bits(request->output, output_elements);
  if (finite == 0u || out_result->output_hash == 0u) {
    out_result->detail_code = PROM_M44_DETAIL_READBACK;
    return PROM_ERROR;
  }
  out_result->matched_input = 1u;
  out_result->input_generation = request->input_generation;
  out_result->wo_generation = request->required_wo_generation;
  out_result->wo_hash = request->required_wo_hash;
  out_result->replay_identity =
      prom_m40b_hash_u64(out_result->projection.replay_id, request->input_generation);
  out_result->replay_identity =
      prom_m40b_hash_u64(out_result->replay_identity, input_hash);
  out_result->replay_identity =
      prom_m40b_hash_u64(out_result->replay_identity, request->exact_source_hash);
  return PROM_OK;
}


/* M0 ownership merge: pure M47/M48 planning and reference code. */
#include "reactor_vulkan.h"

#include <math.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#define PROM_M47_CAPACITY_LIMIT_BYTES (1024ull * 1024ull * 1024ull)
#define PROM_M47_GATE_SHADER_HASH 0x4224253f52d36e32ull
#define PROM_M47_GATE_PACK_SHADER_HASH 0x6de00e90fd1f3249ull

static uint64_t prom_m47_hash_u32(uint64_t hash, uint32_t value) {
  uint32_t byte_index;
  for (byte_index = 0u; byte_index < 4u; ++byte_index) {
    hash ^= (uint64_t)((value >> (byte_index * 8u)) & 0xffu);
    hash *= 1099511628211ull;
  }
  return hash;
}

static uint64_t prom_m47_hash_u64(uint64_t hash, uint64_t value) {
  hash = prom_m47_hash_u32(hash, (uint32_t)value);
  return prom_m47_hash_u32(hash, (uint32_t)(value >> 32u));
}

static int prom_m47_add_u64(uint64_t left, uint64_t right, uint64_t* out) {
  if (out == NULL || left > UINT64_MAX - right) return 0;
  *out = left + right;
  return 1;
}

static int prom_m47_mul_u64(uint64_t left, uint64_t right, uint64_t* out) {
  if (out == NULL || (right != 0u && left > UINT64_MAX / right)) return 0;
  *out = left * right;
  return 1;
}

static int prom_m47_round_up_16(uint32_t value, uint32_t* out) {
  if (out == NULL || value == 0u || value > UINT32_MAX - 15u) return 0;
  *out = (value + 15u) & ~15u;
  return 1;
}

static int prom_m47_accumulate(uint64_t* total, uint64_t value) {
  return prom_m47_add_u64(*total, value, total);
}

static void prom_m47_add_barrier(prom_m47_gated_ffn_plan* plan,
                                 uint32_t buffer_identity,
                                 uint64_t byte_offset,
                                 uint64_t byte_length,
                                 uint32_t source_access,
                                 uint32_t destination_access) {
  prom_m47_barrier_trace* barrier;
  if (plan->barrier_count >= PROM_M47_MAX_BARRIERS) return;
  barrier = &plan->barriers[plan->barrier_count];
  memset(barrier, 0, sizeof(*barrier));
  barrier->sequence = plan->barrier_count;
  barrier->buffer_identity = buffer_identity;
  barrier->byte_offset = byte_offset;
  barrier->byte_length = byte_length;
  barrier->source_stage_mask = VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT;
  barrier->destination_stage_mask = VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT;
  barrier->source_access_mask = source_access;
  barrier->destination_access_mask = destination_access;
  barrier->source_queue_family = VK_QUEUE_FAMILY_IGNORED;
  barrier->destination_queue_family = VK_QUEUE_FAMILY_IGNORED;
  plan->barrier_count += 1u;
}

static void prom_m47_add_stage(prom_m47_gated_ffn_plan* plan,
                               uint32_t operation,
                               uint32_t dispatch_count,
                               uint32_t barrier_begin,
                               uint32_t barrier_count,
                               uint32_t copy_count,
                               uint32_t timestamp_begin,
                               uint32_t timestamp_end) {
  prom_m47_stage_plan* stage;
  if (plan->stage_count >= PROM_M47_MAX_STAGES) return;
  stage = &plan->stages[plan->stage_count];
  memset(stage, 0, sizeof(*stage));
  stage->sequence = plan->stage_count;
  stage->operation = operation;
  stage->dispatch_count = dispatch_count;
  stage->barrier_begin = barrier_begin;
  stage->barrier_count = barrier_count;
  stage->copy_region_count = copy_count;
  stage->timestamp_begin = timestamp_begin;
  stage->timestamp_end = timestamp_end;
  plan->stage_count += 1u;
  plan->dispatch_count += dispatch_count;
  plan->copy_region_count += copy_count;
}

static uint64_t prom_m47_generation(uint64_t seed, uint64_t value0, uint64_t value1) {
  uint64_t hash = prom_m47_hash_u64(seed, value0);
  hash = prom_m47_hash_u64(hash, value1);
  return hash != 0u ? hash : 1u;
}

int prom_m47_gated_ffn_plan_build(const prom_m47_plan_request* request,
                                  prom_m47_gated_ffn_plan* out_plan) {
  uint64_t logical_n_elements;
  uint64_t logical_hidden_elements;
  uint64_t padded_n_elements;
  uint64_t padded_hidden_elements;
  uint64_t weight_elements;
  uint64_t bytes;
  uint64_t total = 0u;
  uint64_t hash = 1469598103934665603ull;
  uint32_t weight;
  uint32_t barrier_begin;
  uint32_t reduced;
  if (out_plan == NULL) return PROM_ERROR;
  memset(out_plan, 0, sizeof(*out_plan));
  out_plan->memory.capacity_limit_bytes = PROM_M47_CAPACITY_LIMIT_BYTES;
  out_plan->eligibility_reason = PROM_M47_INELIGIBLE_VIEW;
  if (request == NULL || request->n_view.buffer == VK_NULL_HANDLE ||
      request->n_view.owning_device == VK_NULL_HANDLE || request->n_view.byte_length == 0u ||
      request->n_view.element_type != PROM_DEVICE_ELEMENT_F32 ||
      request->n_view.layout != PROM_DEVICE_LAYOUT_ROW_MAJOR ||
      request->n_view.producer_access != PROM_DEVICE_ACCESS_COMPUTE_WRITE ||
      request->n_view.required_consumer_access != PROM_DEVICE_ACCESS_COMPUTE_READ ||
      request->n_view.owning_lifetime_id == 0u ||
      request->n_view.owning_slot_generation == 0u) return PROM_ERROR;
  out_plan->eligibility_reason = PROM_M47_INELIGIBLE_SHAPE;
  if (request->tokens == 0u || request->tokens > 1024u ||
      request->model_width == 0u || request->model_width > 4096u ||
      request->ffn_width == 0u || request->ffn_width > 8192u ||
      request->n_view.logical_rows != request->tokens ||
      request->n_view.logical_columns != request->model_width) return PROM_ERROR;
  out_plan->eligibility_reason = PROM_M47_INELIGIBLE_STRIDE;
  if (request->n_view.row_stride_elements < request->model_width ||
      !prom_m47_mul_u64(request->tokens, request->n_view.row_stride_elements, &logical_n_elements) ||
      !prom_m47_mul_u64(logical_n_elements, sizeof(float), &bytes) ||
      bytes > request->n_view.byte_length) return PROM_ERROR;
  out_plan->eligibility_reason = PROM_M47_INELIGIBLE_GENERATION;
  if (request->expected_n_generation == 0u ||
      request->n_view.owning_lifetime_id != request->expected_n_generation ||
      request->m46_replay_id == 0u) return PROM_ERROR;
  out_plan->eligibility_reason = PROM_M47_INELIGIBLE_WEIGHT;
  for (weight = 0u; weight < PROM_M47_WEIGHT_COUNT; ++weight) {
    if (request->weight_generation[weight] == 0u || request->weight_hash[weight] == 0u)
      return PROM_ERROR;
  }
  out_plan->eligibility_reason = PROM_M47_INELIGIBLE_PRECISION;
  if (request->projection_path < PROM_M47_PROJECTION_COOPERATIVE ||
      request->projection_path > PROM_M47_PROJECTION_CONVENTIONAL_FP16) return PROM_ERROR;
  out_plan->eligibility_reason = PROM_M47_INELIGIBLE_GATING;
  if (request->gating_strategy < PROM_M47_GATING_SEPARATE ||
      request->gating_strategy > PROM_M47_GATING_FUSED_DIRECT_PACKED ||
      (request->gating_strategy == PROM_M47_GATING_FUSED_DIRECT_PACKED &&
       request->projection_path == PROM_M47_PROJECTION_A2X4_FP32)) return PROM_ERROR;
  out_plan->eligibility_reason = PROM_M47_INELIGIBLE_RESIDUAL;
  if (request->residual_strategy < PROM_M47_RESIDUAL_SEPARATE_OUTPUT ||
      request->residual_strategy > PROM_M47_RESIDUAL_IN_PLACE_N_AUDIT ||
      request->submit_policy < PROM_M47_SUBMIT_ONE_COMMAND_BUFFER ||
      request->submit_policy > PROM_M47_SUBMIT_TWO_BOUNDED) return PROM_ERROR;
  if (request->residual_strategy == PROM_M47_RESIDUAL_IN_PLACE_N_AUDIT) {
    out_plan->eligibility_reason = PROM_M47_INELIGIBLE_EXCLUSIVITY;
    return PROM_ERROR;
  }
  reduced = request->projection_path != PROM_M47_PROJECTION_A2X4_FP32;
  if (reduced == 0u && request->n_view.row_stride_elements != request->model_width) {
    out_plan->eligibility_reason = PROM_M47_INELIGIBLE_STRIDE;
    return PROM_ERROR;
  }
  if (!prom_m47_round_up_16(request->tokens, &out_plan->padded_tokens) ||
      !prom_m47_round_up_16(request->model_width, &out_plan->padded_model_width) ||
      !prom_m47_round_up_16(request->ffn_width, &out_plan->padded_ffn_width)) return PROM_ERROR;
  out_plan->tokens = request->tokens;
  out_plan->model_width = request->model_width;
  out_plan->ffn_width = request->ffn_width;
  out_plan->n_row_stride = request->n_view.row_stride_elements;
  out_plan->gate_row_stride = reduced != 0u ? out_plan->padded_ffn_width : request->ffn_width;
  out_plan->up_row_stride = out_plan->gate_row_stride;
  out_plan->hidden_row_stride = out_plan->gate_row_stride;
  out_plan->down_row_stride = reduced != 0u ? out_plan->padded_model_width : request->model_width;
  out_plan->output_row_stride = request->residual_strategy == PROM_M47_RESIDUAL_IN_PLACE_DOWN
                                  ? out_plan->down_row_stride : request->model_width;
  out_plan->projection_path = request->projection_path;
  out_plan->gating_strategy = request->gating_strategy;
  out_plan->hidden_storage = request->gating_strategy == PROM_M47_GATING_FUSED_DIRECT_PACKED
                               ? PROM_M47_HIDDEN_PACKED_F16 : PROM_M47_HIDDEN_FP32;
  out_plan->residual_strategy = request->residual_strategy;
  out_plan->submit_policy = request->submit_policy;
  out_plan->submit_count = request->submit_policy == PROM_M47_SUBMIT_TWO_BOUNDED ? 2u : 1u;
  out_plan->final_readback_count = request->final_readback != 0u ? 1u : 0u;
  out_plan->n_generation = request->expected_n_generation;
  out_plan->m46_replay_id = request->m46_replay_id;
  out_plan->gate_shader_hash = PROM_M47_GATE_SHADER_HASH;
  out_plan->gate_pack_shader_hash = PROM_M47_GATE_PACK_SHADER_HASH;
  memcpy(out_plan->weight_generation, request->weight_generation,
         sizeof(out_plan->weight_generation));
  memcpy(out_plan->weight_hash, request->weight_hash, sizeof(out_plan->weight_hash));

  if (!prom_m47_mul_u64(request->tokens, request->model_width, &logical_n_elements) ||
      !prom_m47_mul_u64(request->tokens, request->ffn_width, &logical_hidden_elements) ||
      !prom_m47_mul_u64(out_plan->padded_tokens, out_plan->padded_model_width, &padded_n_elements) ||
      !prom_m47_mul_u64(out_plan->padded_tokens, out_plan->padded_ffn_width,
                        &padded_hidden_elements)) return PROM_ERROR;
  out_plan->memory.n_view_bytes = bytes;
  if (reduced != 0u) {
    if (!prom_m47_mul_u64((padded_n_elements + 1u) / 2u, sizeof(uint32_t),
                          &out_plan->memory.n_packed_bytes)) return PROM_ERROR;
  }
  for (weight = 0u; weight < PROM_M47_WEIGHT_COUNT; ++weight) {
    const uint32_t rows = weight == PROM_M47_WEIGHT_DOWN ? request->ffn_width : request->model_width;
    const uint32_t columns = weight == PROM_M47_WEIGHT_DOWN ? request->model_width : request->ffn_width;
    const uint32_t padded_rows = weight == PROM_M47_WEIGHT_DOWN
                                   ? out_plan->padded_ffn_width : out_plan->padded_model_width;
    const uint32_t padded_columns = weight == PROM_M47_WEIGHT_DOWN
                                      ? out_plan->padded_model_width : out_plan->padded_ffn_width;
    if (!prom_m47_mul_u64(rows, columns, &weight_elements) ||
        !prom_m47_mul_u64(weight_elements, sizeof(float),
                          &out_plan->memory.weight_upload_bytes[weight]) ||
        !prom_m47_mul_u64(weight_elements, sizeof(float),
                          &out_plan->memory.weight_f32_bytes[weight]) ||
        !prom_m47_mul_u64(padded_rows, padded_columns, &weight_elements) ||
        !prom_m47_mul_u64((weight_elements + 1u) / 2u, sizeof(uint32_t),
                          &out_plan->memory.weight_packed_bytes[weight])) return PROM_ERROR;
  }
  bytes = reduced != 0u ? padded_hidden_elements : logical_hidden_elements;
  if (!prom_m47_mul_u64(bytes, sizeof(float), &out_plan->memory.gate_bytes)) return PROM_ERROR;
  out_plan->memory.up_bytes = out_plan->memory.gate_bytes;
  if (request->gating_strategy == PROM_M47_GATING_SEPARATE)
    out_plan->memory.activated_gate_bytes = out_plan->memory.gate_bytes;
  if (request->gating_strategy != PROM_M47_GATING_FUSED_DIRECT_PACKED)
    out_plan->memory.hidden_f32_bytes = out_plan->memory.gate_bytes;
  if (reduced != 0u &&
      !prom_m47_mul_u64((padded_hidden_elements + 1u) / 2u, sizeof(uint32_t),
                        &out_plan->memory.hidden_packed_bytes)) return PROM_ERROR;
  bytes = reduced != 0u ? padded_n_elements : logical_n_elements;
  if (!prom_m47_mul_u64(bytes, sizeof(float), &out_plan->memory.down_bytes)) return PROM_ERROR;
  if (request->residual_strategy == PROM_M47_RESIDUAL_SEPARATE_OUTPUT &&
      !prom_m47_mul_u64(logical_n_elements, sizeof(float),
                        &out_plan->memory.separate_output_bytes)) return PROM_ERROR;
  if (request->final_readback != 0u &&
      !prom_m47_mul_u64(logical_n_elements, sizeof(float),
                        &out_plan->memory.final_readback_bytes)) return PROM_ERROR;
  out_plan->memory.fused_gating_saved_bytes = out_plan->memory.gate_bytes;
  if (!prom_m47_mul_u64(logical_n_elements, sizeof(float),
                        &out_plan->memory.in_place_down_saved_bytes)) return PROM_ERROR;
  out_plan->memory.reusable_descriptor_set_count = 7u;
  out_plan->memory.descriptor_binding_count = 28u;
  if (!prom_m47_accumulate(&total, out_plan->memory.n_view_bytes) ||
      !prom_m47_accumulate(&total, out_plan->memory.n_packed_bytes) ||
      !prom_m47_accumulate(&total, out_plan->memory.gate_bytes) ||
      !prom_m47_accumulate(&total, out_plan->memory.up_bytes) ||
      !prom_m47_accumulate(&total, out_plan->memory.activated_gate_bytes) ||
      !prom_m47_accumulate(&total, out_plan->memory.hidden_f32_bytes) ||
      !prom_m47_accumulate(&total, out_plan->memory.hidden_packed_bytes) ||
      !prom_m47_accumulate(&total, out_plan->memory.down_bytes) ||
      !prom_m47_accumulate(&total, out_plan->memory.separate_output_bytes) ||
      !prom_m47_accumulate(&total, out_plan->memory.final_readback_bytes)) return PROM_ERROR;
  for (weight = 0u; weight < PROM_M47_WEIGHT_COUNT; ++weight) {
    if (!prom_m47_accumulate(&total, out_plan->memory.weight_upload_bytes[weight]) ||
        !prom_m47_accumulate(&total, out_plan->memory.weight_f32_bytes[weight]) ||
        !prom_m47_accumulate(&total, out_plan->memory.weight_packed_bytes[weight])) return PROM_ERROR;
  }
  out_plan->memory.exact_request_bytes = total;
  out_plan->eligibility_reason = PROM_M47_INELIGIBLE_CAPACITY;
  if (total > PROM_M47_CAPACITY_LIMIT_BYTES) return PROM_ERROR;

  barrier_begin = out_plan->barrier_count;
  prom_m47_add_barrier(out_plan, PROM_M47_BUFFER_N, request->n_view.offset,
                       out_plan->memory.n_view_bytes, VK_ACCESS_SHADER_WRITE_BIT,
                       VK_ACCESS_SHADER_READ_BIT);
  prom_m47_add_stage(out_plan, PROM_M47_STAGE_N_READY, 0u, barrier_begin, 1u, 0u, 0u, 1u);
  if (reduced != 0u) {
    barrier_begin = out_plan->barrier_count;
    prom_m47_add_barrier(out_plan, PROM_M47_BUFFER_N_PACKED, 0u,
                         out_plan->memory.n_packed_bytes, VK_ACCESS_SHADER_WRITE_BIT,
                         VK_ACCESS_SHADER_READ_BIT);
    prom_m47_add_stage(out_plan, PROM_M47_STAGE_PACK_N, 1u, barrier_begin, 1u, 0u, 1u, 2u);
  }
  prom_m47_add_stage(out_plan, PROM_M47_STAGE_GATE_PROJECTION, 1u,
                     out_plan->barrier_count, 0u, 0u, 2u, 3u);
  prom_m47_add_stage(out_plan, PROM_M47_STAGE_UP_PROJECTION, 1u,
                     out_plan->barrier_count, 0u, 0u, 3u, 4u);
  barrier_begin = out_plan->barrier_count;
  prom_m47_add_barrier(out_plan, PROM_M47_BUFFER_GATE, 0u, out_plan->memory.gate_bytes,
                       VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT);
  prom_m47_add_barrier(out_plan, PROM_M47_BUFFER_UP, 0u, out_plan->memory.up_bytes,
                       VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT);
  if (request->gating_strategy == PROM_M47_GATING_SEPARATE) {
    prom_m47_add_stage(out_plan, PROM_M47_STAGE_SILU, 1u, barrier_begin, 2u, 0u, 4u, 5u);
    barrier_begin = out_plan->barrier_count;
    prom_m47_add_barrier(out_plan, PROM_M47_BUFFER_ACTIVATED_GATE, 0u,
                         out_plan->memory.activated_gate_bytes, VK_ACCESS_SHADER_WRITE_BIT,
                         VK_ACCESS_SHADER_READ_BIT);
    prom_m47_add_stage(out_plan, PROM_M47_STAGE_GATE_MULTIPLY, 1u,
                       barrier_begin, 1u, 0u, 5u, 6u);
  } else {
    prom_m47_add_stage(out_plan, PROM_M47_STAGE_FUSED_GATE, 1u,
                       barrier_begin, 2u, 0u, 4u, 6u);
  }
  barrier_begin = out_plan->barrier_count;
  if (request->gating_strategy == PROM_M47_GATING_FUSED_DIRECT_PACKED) {
    prom_m47_add_barrier(out_plan, PROM_M47_BUFFER_HIDDEN_PACKED, 0u,
                         out_plan->memory.hidden_packed_bytes, VK_ACCESS_SHADER_WRITE_BIT,
                         VK_ACCESS_SHADER_READ_BIT);
  } else if (reduced != 0u) {
    prom_m47_add_barrier(out_plan, PROM_M47_BUFFER_HIDDEN, 0u,
                         out_plan->memory.hidden_f32_bytes, VK_ACCESS_SHADER_WRITE_BIT,
                         VK_ACCESS_SHADER_READ_BIT);
    prom_m47_add_stage(out_plan, PROM_M47_STAGE_PACK_HIDDEN, 1u,
                       barrier_begin, 1u, 0u, 6u, 7u);
    barrier_begin = out_plan->barrier_count;
    prom_m47_add_barrier(out_plan, PROM_M47_BUFFER_HIDDEN_PACKED, 0u,
                         out_plan->memory.hidden_packed_bytes, VK_ACCESS_SHADER_WRITE_BIT,
                         VK_ACCESS_SHADER_READ_BIT);
  } else {
    prom_m47_add_barrier(out_plan, PROM_M47_BUFFER_HIDDEN, 0u,
                         out_plan->memory.hidden_f32_bytes, VK_ACCESS_SHADER_WRITE_BIT,
                         VK_ACCESS_SHADER_READ_BIT);
  }
  prom_m47_add_stage(out_plan, PROM_M47_STAGE_DOWN_PROJECTION, 1u,
                     barrier_begin, 1u, 0u, 7u, 8u);
  barrier_begin = out_plan->barrier_count;
  prom_m47_add_barrier(out_plan, PROM_M47_BUFFER_DOWN, 0u, out_plan->memory.down_bytes,
                       VK_ACCESS_SHADER_WRITE_BIT,
                       request->residual_strategy == PROM_M47_RESIDUAL_IN_PLACE_DOWN
                         ? VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT
                         : VK_ACCESS_SHADER_READ_BIT);
  prom_m47_add_barrier(out_plan, PROM_M47_BUFFER_N, request->n_view.offset,
                       out_plan->memory.n_view_bytes, VK_ACCESS_SHADER_READ_BIT,
                       VK_ACCESS_SHADER_READ_BIT);
  prom_m47_add_stage(out_plan, PROM_M47_STAGE_SECOND_RESIDUAL, 1u,
                     barrier_begin, 2u, 0u, 8u, 9u);
  barrier_begin = out_plan->barrier_count;
  prom_m47_add_barrier(out_plan, PROM_M47_BUFFER_OUTPUT, 0u,
                       request->residual_strategy == PROM_M47_RESIDUAL_IN_PLACE_DOWN
                         ? out_plan->memory.down_bytes : out_plan->memory.separate_output_bytes,
                       VK_ACCESS_SHADER_WRITE_BIT,
                       request->final_readback != 0u ? VK_ACCESS_TRANSFER_READ_BIT
                                                     : VK_ACCESS_SHADER_READ_BIT);
  if (request->final_readback != 0u) {
    prom_m47_add_stage(out_plan, PROM_M47_STAGE_FINAL_READBACK, 0u,
                       barrier_begin, 1u, request->tokens, 9u, 10u);
  }
  if (out_plan->stage_count > PROM_M47_MAX_STAGES ||
      out_plan->barrier_count > PROM_M47_MAX_BARRIERS) return PROM_ERROR;

  out_plan->gate_generation = prom_m47_generation(hash, request->expected_n_generation,
                                                   request->weight_generation[PROM_M47_WEIGHT_GATE]);
  out_plan->up_generation = prom_m47_generation(out_plan->gate_generation,
                                                 request->expected_n_generation,
                                                 request->weight_generation[PROM_M47_WEIGHT_UP]);
  out_plan->hidden_generation = prom_m47_generation(out_plan->up_generation,
                                                     request->gating_strategy,
                                                     PROM_M47_GATE_SHADER_HASH);
  out_plan->down_generation = prom_m47_generation(out_plan->hidden_generation,
                                                   request->weight_generation[PROM_M47_WEIGHT_DOWN],
                                                   request->projection_path);
  out_plan->output_generation = prom_m47_generation(out_plan->down_generation,
                                                     request->expected_n_generation,
                                                     request->residual_strategy);
  hash = prom_m47_hash_u32(hash, request->tokens);
  hash = prom_m47_hash_u32(hash, request->model_width);
  hash = prom_m47_hash_u32(hash, request->ffn_width);
  hash = prom_m47_hash_u32(hash, request->projection_path);
  hash = prom_m47_hash_u32(hash, request->gating_strategy);
  hash = prom_m47_hash_u32(hash, request->residual_strategy);
  hash = prom_m47_hash_u32(hash, request->submit_policy);
  hash = prom_m47_hash_u64(hash, request->expected_n_generation);
  hash = prom_m47_hash_u64(hash, request->m46_replay_id);
  for (weight = 0u; weight < PROM_M47_WEIGHT_COUNT; ++weight) {
    hash = prom_m47_hash_u64(hash, request->weight_generation[weight]);
    hash = prom_m47_hash_u64(hash, request->weight_hash[weight]);
  }
  hash = prom_m47_hash_u64(hash, out_plan->output_generation);
  out_plan->command_plan_replay_id = prom_m47_generation(hash, out_plan->dispatch_count,
                                                         out_plan->barrier_count);
  out_plan->replay_id = prom_m47_generation(out_plan->command_plan_replay_id,
                                            PROM_M47_GATE_SHADER_HASH,
                                            PROM_M47_GATE_PACK_SHADER_HASH);
  out_plan->eligibility_replay_id = prom_m47_generation(out_plan->replay_id, total,
                                                         PROM_M47_ELIGIBLE);
  out_plan->eligibility_eligible = 1u;
  out_plan->eligibility_reason = PROM_M47_ELIGIBLE;
  return PROM_OK;
}

static float prom_m47_round_f16(float value) {
  return prom_sgemm_fp16_bits_to_float32(prom_sgemm_float32_to_fp16_bits(value));
}

static float prom_m47_silu(float value) {
  if (value >= 0.0f) return value / (1.0f + expf(-value));
  {
    const float exponential = expf(value);
    return value * exponential / (1.0f + exponential);
  }
}

int prom_m47_gated_ffn_cpu_reference(const prom_m47_reference_request* request) {
  uint64_t n_elements;
  uint64_t gate_weight_elements;
  uint64_t down_weight_elements;
  uint64_t output_elements;
  uint32_t token;
  uint32_t column;
  uint32_t reduced;
  if (request == NULL || request->n == NULL || request->wgate == NULL || request->wup == NULL ||
      request->wdown == NULL || request->gate == NULL || request->up == NULL ||
      request->hidden == NULL || request->output == NULL || request->tokens == 0u ||
      request->model_width == 0u || request->ffn_width == 0u ||
      request->n_row_stride < request->model_width ||
      request->output_row_stride < request->model_width ||
      request->projection_path < PROM_M47_PROJECTION_COOPERATIVE ||
      request->projection_path > PROM_M47_PROJECTION_CONVENTIONAL_FP16 ||
      !prom_m47_mul_u64(request->tokens, request->n_row_stride, &n_elements) ||
      !prom_m47_mul_u64(request->model_width, request->ffn_width, &gate_weight_elements) ||
      !prom_m47_mul_u64(request->ffn_width, request->model_width, &down_weight_elements) ||
      !prom_m47_mul_u64(request->tokens, request->output_row_stride, &output_elements) ||
      request->n_element_count < n_elements ||
      request->wgate_element_count != gate_weight_elements ||
      request->wup_element_count != gate_weight_elements ||
      request->wdown_element_count != down_weight_elements ||
      request->output_element_count < output_elements) return PROM_ERROR;
  reduced = request->projection_path != PROM_M47_PROJECTION_A2X4_FP32;
  for (token = 0u; token < request->tokens; ++token) {
    for (column = 0u; column < request->ffn_width; ++column) {
      float gate = 0.0f;
      float up = 0.0f;
      uint32_t inner;
      for (inner = 0u; inner < request->model_width; ++inner) {
        float n = request->n[(uint64_t)token * request->n_row_stride + inner];
        float wg = request->wgate[(uint64_t)inner * request->ffn_width + column];
        float wu = request->wup[(uint64_t)inner * request->ffn_width + column];
        if (!isfinite(n) || !isfinite(wg) || !isfinite(wu)) return PROM_ERROR;
        if (reduced != 0u) { n = prom_m47_round_f16(n); wg = prom_m47_round_f16(wg); wu = prom_m47_round_f16(wu); }
        gate += n * wg;
        up += n * wu;
      }
      if (!isfinite(gate) || !isfinite(up)) return PROM_ERROR;
      request->gate[(uint64_t)token * request->ffn_width + column] = gate;
      request->up[(uint64_t)token * request->ffn_width + column] = up;
      request->hidden[(uint64_t)token * request->ffn_width + column] = prom_m47_silu(gate) * up;
    }
  }
  for (token = 0u; token < request->tokens; ++token) {
    for (column = 0u; column < request->model_width; ++column) {
      float down = 0.0f;
      uint32_t inner;
      for (inner = 0u; inner < request->ffn_width; ++inner) {
        float hidden = request->hidden[(uint64_t)token * request->ffn_width + inner];
        float weight = request->wdown[(uint64_t)inner * request->model_width + column];
        if (!isfinite(hidden) || !isfinite(weight)) return PROM_ERROR;
        if (reduced != 0u) { hidden = prom_m47_round_f16(hidden); weight = prom_m47_round_f16(weight); }
        down += hidden * weight;
      }
      if (!isfinite(down)) return PROM_ERROR;
      if (request->down != NULL) request->down[(uint64_t)token * request->model_width + column] = down;
      request->output[(uint64_t)token * request->output_row_stride + column] =
          request->n[(uint64_t)token * request->n_row_stride + column] + down;
      if (!isfinite(request->output[(uint64_t)token * request->output_row_stride + column]))
        return PROM_ERROR;
    }
  }
  return PROM_OK;
}

int prom_m47_gated_ffn_compare(const float* expected,
                               const float* actual,
                               uint32_t tokens,
                               uint32_t model_width,
                               uint32_t expected_row_stride,
                               uint32_t actual_row_stride,
                               float absolute_tolerance,
                               float relative_tolerance,
                               const prom_m47_gated_ffn_plan* plan,
                               const float* gate,
                               const float* up,
                               const float* hidden,
                               const float* down,
                               prom_m47_mismatch* out_mismatch) {
  uint32_t token;
  uint32_t column;
  if (out_mismatch == NULL) return PROM_ERROR;
  memset(out_mismatch, 0, sizeof(*out_mismatch));
  if (expected == NULL || actual == NULL || plan == NULL || tokens == 0u || model_width == 0u ||
      expected_row_stride < model_width || actual_row_stride < model_width ||
      !isfinite(absolute_tolerance) || !isfinite(relative_tolerance) ||
      absolute_tolerance < 0.0f || relative_tolerance < 0.0f) return PROM_ERROR;
  out_mismatch->strategy = plan->gating_strategy;
  out_mismatch->n_generation = plan->n_generation;
  memcpy(out_mismatch->weight_generation, plan->weight_generation,
         sizeof(out_mismatch->weight_generation));
  out_mismatch->output_generation = plan->output_generation;
  out_mismatch->m46_replay_id = plan->m46_replay_id;
  out_mismatch->m47_replay_id = plan->replay_id;
  for (token = 0u; token < tokens; ++token) {
    for (column = 0u; column < model_width; ++column) {
      const float wanted = expected[(uint64_t)token * expected_row_stride + column];
      const float got = actual[(uint64_t)token * actual_row_stride + column];
      const float absolute = fabsf(wanted - got);
      const float relative = absolute / fmaxf(fabsf(wanted), 1.0e-30f);
      if (!isfinite(wanted) || !isfinite(got) ||
          (absolute > absolute_tolerance && relative > relative_tolerance)) {
        const uint64_t hidden_index = (uint64_t)token * plan->ffn_width +
                                      (column < plan->ffn_width ? column : 0u);
        out_mismatch->stage = PROM_M47_STAGE_SECOND_RESIDUAL;
        out_mismatch->token = token;
        out_mismatch->column = column;
        out_mismatch->expected = wanted;
        out_mismatch->actual = got;
        out_mismatch->absolute_error = absolute;
        out_mismatch->relative_error = relative;
        if (gate != NULL) out_mismatch->gate = gate[hidden_index];
        if (up != NULL) out_mismatch->up = up[hidden_index];
        if (hidden != NULL) out_mismatch->hidden = hidden[hidden_index];
        if (down != NULL) out_mismatch->down = down[(uint64_t)token * model_width + column];
        return PROM_ERROR;
      }
    }
  }
  out_mismatch->matched = 1u;
  return PROM_OK;
}

uint32_t prom_m48_attention_resource_index(uint32_t head, uint32_t weight_kind) {
  if (head >= PROM_M43_HEAD_COUNT || weight_kind >= PROM_M43_WEIGHT_KIND_COUNT)
    return UINT32_MAX;
  return head * PROM_M43_WEIGHT_KIND_COUNT + weight_kind;
}

static int prom_m48_accumulate(uint64_t* total, uint64_t value) {
  return prom_m47_add_u64(*total, value, total);
}

static int prom_m48_scaled_bytes(uint64_t elements, uint64_t bytes_per_element,
                                 uint64_t* out_bytes) {
  return prom_m47_mul_u64(elements, bytes_per_element, out_bytes);
}

static int prom_m48_validate_initial(const prom_m48_plan_request* request,
                                     uint64_t logical_elements,
                                     uint64_t* out_initial_bytes,
                                     uint64_t* out_upload_bytes) {
  uint64_t physical_elements;
  uint64_t index;
  *out_initial_bytes = 0u;
  *out_upload_bytes = 0u;
  if (request->expected_initial_generation == 0u || request->initial_content_hash == 0u)
    return 0;
  if (request->initial_activation_mode == PROM_M48_INITIAL_HOST) {
    if (request->host_initial_activation == NULL ||
        request->host_initial_element_count != logical_elements) return 0;
    for (index = 0u; index < logical_elements; ++index) {
      if (!isfinite(request->host_initial_activation[index])) return 0;
    }
    if (!prom_m48_scaled_bytes(logical_elements, sizeof(float), out_upload_bytes)) return 0;
    *out_initial_bytes = *out_upload_bytes;
    return 1;
  }
  if (request->initial_activation_mode != PROM_M48_INITIAL_RESIDENT) return 0;
  if (request->resident_initial_activation.buffer == VK_NULL_HANDLE ||
      request->resident_initial_activation.owning_device == VK_NULL_HANDLE ||
      request->resident_initial_activation.element_type != PROM_DEVICE_ELEMENT_F32 ||
      request->resident_initial_activation.layout != PROM_DEVICE_LAYOUT_ROW_MAJOR ||
      request->resident_initial_activation.logical_rows != request->tokens ||
      request->resident_initial_activation.logical_columns != request->model_width ||
      request->resident_initial_activation.row_stride_elements < request->model_width ||
      request->resident_initial_activation.producer_access != PROM_DEVICE_ACCESS_COMPUTE_WRITE ||
      request->resident_initial_activation.required_consumer_access != PROM_DEVICE_ACCESS_COMPUTE_READ ||
      request->resident_initial_activation.owning_lifetime_id != request->expected_initial_generation ||
      request->resident_initial_activation.owning_slot_generation == 0u ||
      !prom_m47_mul_u64(request->tokens,
                       request->resident_initial_activation.row_stride_elements,
                       &physical_elements) ||
      !prom_m48_scaled_bytes(physical_elements, sizeof(float), out_initial_bytes) ||
      request->resident_initial_activation.byte_length < *out_initial_bytes) return 0;
  return 1;
}

static int prom_m48_add_persistent_weights(const prom_m48_plan_request* request,
                                           prom_m48_memory_plan* memory) {
  uint64_t attention_elements;
  uint64_t wo_elements;
  uint64_t ffn_elements;
  uint64_t bytes;
  uint64_t per_layer = 0u;
  if (!prom_m47_mul_u64(request->model_width, request->head_dim, &attention_elements) ||
      !prom_m47_mul_u64(attention_elements, PROM_M48_ATTENTION_RESOURCE_COUNT,
                       &attention_elements) ||
      !prom_m48_scaled_bytes(attention_elements, 10u, &bytes) ||
      !prom_m48_accumulate(&per_layer, bytes) ||
      !prom_m47_mul_u64(request->model_width, request->model_width, &wo_elements) ||
      !prom_m48_scaled_bytes(wo_elements, 10u, &bytes) ||
      !prom_m48_accumulate(&per_layer, bytes) ||
      !prom_m48_scaled_bytes(request->model_width, sizeof(float) * 2u, &bytes) ||
      !prom_m48_accumulate(&per_layer, bytes) ||
      !prom_m47_mul_u64(request->model_width, request->ffn_width, &ffn_elements) ||
      !prom_m47_mul_u64(ffn_elements, PROM_M47_WEIGHT_COUNT, &ffn_elements) ||
      !prom_m48_scaled_bytes(ffn_elements, 10u, &bytes) ||
      !prom_m48_accumulate(&per_layer, bytes) ||
      !prom_m47_mul_u64(per_layer, request->layer_count,
                       &memory->persistent_weight_bytes)) return 0;
  memory->persistent_weight_bytes_per_layer = per_layer;
  return 1;
}

static int prom_m48_add_working_set(const prom_m48_plan_request* request,
                                    uint32_t padded_tokens,
                                    uint32_t padded_model_width,
                                    uint32_t padded_head_dim,
                                    uint32_t padded_ffn_width,
                                    prom_m48_memory_plan* memory) {
  uint64_t padded_activation_elements;
  uint64_t padded_head_elements;
  uint64_t padded_score_elements;
  uint64_t logical_score_elements;
  uint64_t padded_ffn_elements;
  uint64_t per_head = 0u;
  uint64_t bytes;
  uint64_t comparison_activation_bytes;
  if (!prom_m47_mul_u64(padded_tokens, padded_model_width,
                       &padded_activation_elements) ||
      !prom_m47_mul_u64(padded_tokens, padded_head_dim, &padded_head_elements) ||
      !prom_m47_mul_u64(padded_tokens, padded_tokens, &padded_score_elements) ||
      !prom_m47_mul_u64(request->tokens, request->tokens, &logical_score_elements) ||
      !prom_m47_mul_u64(padded_tokens, padded_ffn_width, &padded_ffn_elements)) return 0;

  /* Q/K/V FP32 + Q/K/V packed + one FP32 head output. */
  if (!prom_m48_scaled_bytes(padded_head_elements, 22u, &bytes) ||
      !prom_m48_accumulate(&per_head, bytes) ||
      /* Scores FP32 + packed probabilities, plus compact FP32 probabilities. */
      !prom_m48_scaled_bytes(padded_score_elements, 6u, &bytes) ||
      !prom_m48_accumulate(&per_head, bytes) ||
      !prom_m48_scaled_bytes(logical_score_elements, sizeof(float), &bytes) ||
      !prom_m48_accumulate(&per_head, bytes) ||
      !prom_m47_mul_u64(per_head, request->head_count,
                       &memory->attention_working_bytes)) return 0;

  /* Packed C plus the in-place Y -> Residual1 -> N storage. */
  if (!prom_m48_scaled_bytes(padded_activation_elements, 6u,
                            &memory->output_projection_working_bytes) ||
      !prom_m48_scaled_bytes(request->tokens, sizeof(float),
                            &memory->normalization_working_bytes)) return 0;
  if (request->model_width > 1024u) {
    uint64_t partials;
    if (!prom_m47_mul_u64(request->tokens,
                         (request->model_width + 1023u) / 1024u, &partials) ||
        !prom_m48_scaled_bytes(partials, sizeof(float), &bytes) ||
        !prom_m48_accumulate(&memory->normalization_working_bytes, bytes)) return 0;
  }

  /* Packed N + Gate/Up FP32 + direct-packed Hidden.  Down is the next
     external activation role and is counted with activation ownership. */
  if (!prom_m48_scaled_bytes(padded_activation_elements, 2u, &bytes) ||
      !prom_m48_accumulate(&memory->ffn_working_bytes, bytes) ||
      !prom_m48_scaled_bytes(padded_ffn_elements, 10u, &bytes) ||
      !prom_m48_accumulate(&memory->ffn_working_bytes, bytes) ||
      !prom_m48_accumulate(&memory->one_block_working_set_bytes,
                           memory->attention_working_bytes) ||
      !prom_m48_accumulate(&memory->one_block_working_set_bytes,
                           memory->output_projection_working_bytes) ||
      !prom_m48_accumulate(&memory->one_block_working_set_bytes,
                           memory->normalization_working_bytes) ||
      !prom_m48_accumulate(&memory->one_block_working_set_bytes,
                           memory->ffn_working_bytes) ||
      !prom_m48_scaled_bytes(padded_activation_elements, sizeof(float), &bytes)) return 0;

  if (request->activation_strategy == PROM_M48_ACTIVATION_PING_PONG) {
    /* The immutable initial activation is owned separately.  Stack outputs
       alternate between two full padded activation buffers so the next layer
       never aliases the input it is still consuming. */
    if (!prom_m47_mul_u64(bytes, 2u, &memory->activation_bytes)) return 0;
  } else {
    if (!prom_m47_mul_u64(bytes, request->layer_count, &memory->activation_bytes)) return 0;
  }
  if (!prom_m47_mul_u64(bytes, request->layer_count,
                       &memory->per_layer_output_activation_bytes) ||
      !prom_m47_add_u64(memory->initial_activation_bytes,
                       memory->per_layer_output_activation_bytes,
                       &comparison_activation_bytes)) return 0;
  memory->comparison_per_layer_retained_bytes = comparison_activation_bytes;
  if (!prom_m47_add_u64(memory->initial_activation_bytes, memory->activation_bytes, &bytes))
    return 0;
  /* One-layer audit still reserves the product slot's two bounded ping roles;
     that is capacity, not a negative "saving" versus one retained output. */
  memory->ping_pong_saved_bytes = comparison_activation_bytes > bytes
                                      ? comparison_activation_bytes - bytes : 0u;
  return 1;
}

static uint32_t prom_m48_layer_projection_path(const prom_m48_plan_request* request,
                                               uint32_t layer) {
  if (request->numerical_control_mode == PROM_M48_NUMERICAL_CONTROL_M49B &&
      request->controller_layer_projection_path[layer] != 0u)
    return request->controller_layer_projection_path[layer];
  return request->audit_layer_projection_path[layer] != 0u
             ? request->audit_layer_projection_path[layer]
             : request->projection_path;
}

int prom_m48_transformer_stack_plan_build(const prom_m48_plan_request* request,
                                          prom_m48_transformer_stack_plan* out_plan) {
  uint64_t logical_elements;
  uint64_t activation_range_bytes;
  uint64_t slot_bytes = 0u;
  uint64_t total = 0u;
  uint64_t hash = 1469598103934665603ull;
  uint32_t padded_tokens;
  uint32_t padded_model_width;
  uint32_t padded_head_dim;
  uint32_t padded_ffn_width;
  uint32_t layer;
  uint32_t resource;
  if (out_plan == NULL) return PROM_ERROR;
  memset(out_plan, 0, sizeof(*out_plan));
  out_plan->eligibility_reason = PROM_M48_INELIGIBLE_LAYER_COUNT;
  if (request == NULL || request->layer_count == 0u ||
      request->layer_count > PROM_M48_LAYER_COUNT ||
      (request->audit_mode == 0u && request->layer_count != PROM_M48_LAYER_COUNT)) return PROM_ERROR;
  out_plan->eligibility_reason = PROM_M48_INELIGIBLE_SHAPE;
  if (request->tokens == 0u || request->tokens > 1024u ||
      request->model_width == 0u || request->model_width > 4096u ||
      request->head_count != PROM_M43_HEAD_COUNT || request->head_dim == 0u ||
      request->ffn_width == 0u || request->ffn_width > 8192u ||
      request->head_dim > UINT32_MAX / request->head_count ||
      request->head_count * request->head_dim != request->model_width ||
      !prom_m47_mul_u64(request->tokens, request->model_width, &logical_elements) ||
      !prom_m47_round_up_16(request->tokens, &padded_tokens) ||
      !prom_m47_round_up_16(request->model_width, &padded_model_width) ||
      !prom_m47_round_up_16(request->head_dim, &padded_head_dim) ||
      !prom_m47_round_up_16(request->ffn_width, &padded_ffn_width)) return PROM_ERROR;
  out_plan->eligibility_reason = PROM_M48_INELIGIBLE_PRECISION;
  if (request->precision_policy < PROM_M42_PRECISION_F16_ROUNDED ||
      request->precision_policy > PROM_M42_PRECISION_FP32 ||
      request->projection_path < PROM_M47_PROJECTION_COOPERATIVE ||
      request->projection_path > PROM_M47_PROJECTION_CONVENTIONAL_FP16 ||
      (request->precision_policy == PROM_M42_PRECISION_FP32 &&
       request->projection_path != PROM_M47_PROJECTION_A2X4_FP32) ||
      (request->precision_policy == PROM_M42_PRECISION_F16_ROUNDED &&
       request->projection_path == PROM_M47_PROJECTION_A2X4_FP32)) return PROM_ERROR;
  if (request->numerical_control_mode > PROM_M48_NUMERICAL_CONTROL_M49B ||
      (request->numerical_control_mode == PROM_M48_NUMERICAL_CONTROL_NONE &&
       (request->controller_parameter_generation != 0u ||
        request->controller_execution_identity != 0u ||
        request->numerical_witness_mode != 0u))) return PROM_ERROR;
  if (request->numerical_control_mode == PROM_M48_NUMERICAL_CONTROL_M49B &&
      (request->controller_parameter_generation == 0u ||
       request->controller_execution_identity == 0u ||
       request->numerical_witness_mode > 1u)) return PROM_ERROR;
  for (layer = 0u; layer < request->layer_count; ++layer) {
    const uint32_t audit_path = request->audit_layer_projection_path[layer];
    const uint32_t controller_path = request->controller_layer_projection_path[layer];
    if ((request->audit_mode == 0u && audit_path != 0u) ||
        (audit_path != 0u &&
         (audit_path < PROM_M47_PROJECTION_COOPERATIVE ||
          audit_path > PROM_M47_PROJECTION_CONVENTIONAL_FP16)) ||
        (request->numerical_control_mode == PROM_M48_NUMERICAL_CONTROL_NONE &&
         controller_path != 0u) ||
        (request->numerical_control_mode == PROM_M48_NUMERICAL_CONTROL_M49B &&
         audit_path != 0u) ||
        (controller_path != 0u &&
         (controller_path < PROM_M47_PROJECTION_COOPERATIVE ||
          controller_path > PROM_M47_PROJECTION_CONVENTIONAL_FP16))) return PROM_ERROR;
  }
  out_plan->eligibility_reason = PROM_M48_INELIGIBLE_STRATEGY;
  if (request->attention_strategy < PROM_M43_STRATEGY_COMPLETE_HEADS ||
      request->attention_strategy > PROM_M43_STRATEGY_EIGHT_SEQUENTIAL_M42 ||
      request->output_projection_strategy < PROM_M44_AGGREGATION_INTERLEAVE ||
      request->output_projection_strategy > PROM_M44_AGGREGATION_DIRECT_SEGMENTED ||
      request->rmsnorm_strategy < PROM_M46_STRATEGY_SEPARATE_OUTPUT ||
      request->rmsnorm_strategy > PROM_M46_STRATEGY_IN_PLACE_Z ||
      request->gating_strategy < PROM_M47_GATING_SEPARATE ||
      request->gating_strategy > PROM_M47_GATING_FUSED_DIRECT_PACKED ||
      request->residual_strategy < PROM_M47_RESIDUAL_SEPARATE_OUTPUT ||
      request->residual_strategy > PROM_M47_RESIDUAL_IN_PLACE_DOWN ||
      request->activation_strategy < PROM_M48_ACTIVATION_PING_PONG ||
      request->activation_strategy > PROM_M48_ACTIVATION_PER_LAYER ||
      request->submit_topology < PROM_M48_SUBMIT_ONE_STACK ||
      request->submit_topology > PROM_M48_SUBMIT_HOST_BOUNCE_PER_LAYER_AUDIT ||
      (request->submit_topology >= PROM_M48_SUBMIT_HOST_WAIT_PER_LAYER_AUDIT &&
       request->audit_mode == 0u) ||
      request->optional_final_readback > 1u ||
      request->audit_stage > PROM_M48_AUDIT_STAGE_FFN ||
      (request->audit_stage != PROM_M48_AUDIT_STAGE_NONE && request->audit_mode == 0u) ||
      (request->gating_strategy == PROM_M47_GATING_FUSED_DIRECT_PACKED &&
       request->projection_path == PROM_M47_PROJECTION_A2X4_FP32) ||
      (request->activation_strategy == PROM_M48_ACTIVATION_PING_PONG &&
       request->initial_activation_exclusive == 0u)) return PROM_ERROR;
  out_plan->eligibility_reason = PROM_M48_INELIGIBLE_INITIAL_ACTIVATION;
  if (!prom_m48_validate_initial(request, logical_elements,
                                 &out_plan->memory.initial_activation_bytes,
                                 &out_plan->memory.host_initial_upload_bytes)) return PROM_ERROR;
  out_plan->eligibility_reason = PROM_M48_INELIGIBLE_WEIGHT;
  for (layer = 0u; layer < request->layer_count; ++layer) {
    for (resource = 0u; resource < PROM_M48_RESOURCE_COUNT; ++resource) {
      if (request->layer[layer].generation[resource] == 0u ||
          request->layer[layer].content_hash[resource] == 0u) return PROM_ERROR;
    }
  }
  out_plan->eligibility_reason = PROM_M48_INELIGIBLE_OVERFLOW;
  if (!prom_m48_add_persistent_weights(request, &out_plan->memory) ||
      !prom_m48_add_working_set(request, padded_tokens, padded_model_width,
                                padded_head_dim, padded_ffn_width, &out_plan->memory) ||
      !prom_m48_scaled_bytes(logical_elements, sizeof(float),
                             &out_plan->memory.final_readback_bytes)) return PROM_ERROR;
  if (request->optional_final_readback == 0u) out_plan->memory.final_readback_bytes = 0u;
  out_plan->memory.audit_readback_bytes = request->audit_stage != PROM_M48_AUDIT_STAGE_NONE
                                                ? logical_elements * sizeof(float) : 0u;
  out_plan->memory.descriptor_set_count = request->layer_count * 134u;
  out_plan->memory.timestamp_query_count = request->layer_count * PROM_M48_QUERY_COUNT_PER_LAYER;
  /* Descriptor sets and query pools are opaque Vulkan objects, not device-buffer
     views.  Their exact counts are capacity checked; inventing byte sizes would
     make the buffer accounting less truthful. */
  out_plan->memory.descriptor_device_buffer_bytes = 0u;
  out_plan->memory.timestamp_query_device_buffer_bytes = 0u;
  if (!prom_m48_accumulate(&slot_bytes, out_plan->memory.activation_bytes) ||
      !prom_m48_accumulate(&slot_bytes, out_plan->memory.one_block_working_set_bytes) ||
      !prom_m48_accumulate(&slot_bytes, out_plan->memory.final_readback_bytes) ||
      !prom_m48_accumulate(&slot_bytes, out_plan->memory.audit_readback_bytes)) return PROM_ERROR;
  out_plan->memory.exact_stack_slot_bytes = slot_bytes;
  out_plan->memory.quarantine_reserve_bytes = slot_bytes;
  if (!prom_m48_accumulate(&total, out_plan->memory.persistent_weight_bytes) ||
      !prom_m48_accumulate(&total, out_plan->memory.initial_activation_bytes) ||
      !prom_m48_accumulate(&total, out_plan->memory.host_initial_upload_bytes) ||
      !prom_m48_accumulate(&total, slot_bytes) ||
      !prom_m48_accumulate(&total, out_plan->memory.quarantine_reserve_bytes)) return PROM_ERROR;
  out_plan->memory.exact_retained_bytes = total;
  out_plan->memory.capacity_limit_bytes = request->capacity_limit_bytes != 0u
                                              ? request->capacity_limit_bytes
                                              : PROM_M48_CAPACITY_LIMIT_BYTES;
  out_plan->eligibility_reason = PROM_M48_INELIGIBLE_CAPACITY;
  if (out_plan->memory.capacity_limit_bytes > PROM_M48_CAPACITY_LIMIT_BYTES ||
      total > out_plan->memory.capacity_limit_bytes) return PROM_ERROR;

  out_plan->layer_count = request->layer_count;
  out_plan->audit_mode = request->audit_mode;
  out_plan->tokens = request->tokens;
  out_plan->model_width = request->model_width;
  out_plan->head_count = request->head_count;
  out_plan->head_dim = request->head_dim;
  out_plan->ffn_width = request->ffn_width;
  out_plan->precision_policy = request->precision_policy;
  out_plan->projection_path = request->projection_path;
  out_plan->activation_strategy = request->activation_strategy;
  out_plan->submit_topology = request->submit_topology;
  out_plan->submit_count = request->submit_topology == PROM_M48_SUBMIT_ONE_STACK
                               ? 1u : request->layer_count;
  out_plan->semaphore_count = request->submit_topology == PROM_M48_SUBMIT_PER_LAYER
                                  ? request->layer_count - 1u : 0u;
  out_plan->fence_count = 1u;
  out_plan->intermediate_host_copy_count =
      request->audit_stage != PROM_M48_AUDIT_STAGE_NONE ? 1u : 0u;
  out_plan->intermediate_readback_count =
      request->audit_stage != PROM_M48_AUDIT_STAGE_NONE ? 1u : 0u;
  out_plan->final_readback_count = request->optional_final_readback;
  out_plan->warm_allocation_count = 0u;
  out_plan->persistent_resource_count = request->layer_count * PROM_M48_RESOURCE_COUNT;
  out_plan->boundary_count = request->layer_count - 1u;
  out_plan->initial_generation = request->expected_initial_generation;
  out_plan->initial_content_hash = request->initial_content_hash;

  hash = prom_m47_hash_u32(hash, request->layer_count);
  hash = prom_m47_hash_u32(hash, request->tokens);
  hash = prom_m47_hash_u32(hash, request->model_width);
  hash = prom_m47_hash_u32(hash, request->head_count);
  hash = prom_m47_hash_u32(hash, request->head_dim);
  hash = prom_m47_hash_u32(hash, request->ffn_width);
  hash = prom_m47_hash_u32(hash, request->precision_policy);
  hash = prom_m47_hash_u32(hash, request->activation_strategy);
  hash = prom_m47_hash_u32(hash, request->submit_topology);
  hash = prom_m47_hash_u32(hash, request->optional_final_readback);
  hash = prom_m47_hash_u32(hash, request->audit_stage);
  hash = prom_m47_hash_u32(hash, request->numerical_control_mode);
  hash = prom_m47_hash_u64(hash, request->controller_parameter_generation);
  hash = prom_m47_hash_u64(hash, request->controller_execution_identity);
  hash = prom_m47_hash_u32(hash, request->numerical_witness_mode);
  hash = prom_m47_hash_u64(hash, request->expected_initial_generation);
  hash = prom_m47_hash_u64(hash, request->initial_content_hash);
  activation_range_bytes = (uint64_t)padded_tokens * padded_model_width * sizeof(float);
  for (layer = 0u; layer < request->layer_count; ++layer) {
    uint64_t layer_hash = 1469598103934665603ull;
    const uint32_t selected_projection_path =
        prom_m48_layer_projection_path(request, layer);
    const uint32_t input_role = request->activation_strategy == PROM_M48_ACTIVATION_PING_PONG
                                    ? layer % 2u : layer;
    const uint32_t output_role = request->activation_strategy == PROM_M48_ACTIVATION_PING_PONG
                                     ? (layer + 1u) % 2u : layer + 1u;
    memcpy(&out_plan->layer_resources[layer], &request->layer[layer],
           sizeof(out_plan->layer_resources[layer]));
    layer_hash = prom_m47_hash_u32(layer_hash, layer);
    layer_hash = prom_m47_hash_u32(layer_hash, request->tokens);
    layer_hash = prom_m47_hash_u32(layer_hash, request->model_width);
    layer_hash = prom_m47_hash_u32(layer_hash, request->head_dim);
    layer_hash = prom_m47_hash_u32(layer_hash, request->ffn_width);
    layer_hash = prom_m47_hash_u32(layer_hash, selected_projection_path);
    layer_hash = prom_m47_hash_u32(layer_hash, request->attention_strategy);
    layer_hash = prom_m47_hash_u32(layer_hash, request->output_projection_strategy);
    layer_hash = prom_m47_hash_u32(layer_hash, request->rmsnorm_strategy);
    layer_hash = prom_m47_hash_u32(layer_hash, request->gating_strategy);
    layer_hash = prom_m47_hash_u32(layer_hash, request->residual_strategy);
    for (resource = 0u; resource < PROM_M48_RESOURCE_COUNT; ++resource) {
      layer_hash = prom_m47_hash_u64(layer_hash, request->layer[layer].generation[resource]);
      layer_hash = prom_m47_hash_u64(layer_hash, request->layer[layer].content_hash[resource]);
    }
    out_plan->layer[layer].layer = layer;
    out_plan->layer[layer].selected_projection_path = selected_projection_path;
    out_plan->layer[layer].submit_index = request->submit_topology == PROM_M48_SUBMIT_ONE_STACK
                                             ? 0u : layer;
    out_plan->layer[layer].query_begin = layer * PROM_M48_QUERY_COUNT_PER_LAYER;
    out_plan->layer[layer].query_count = PROM_M48_QUERY_COUNT_PER_LAYER;
    out_plan->layer[layer].input_activation_role = input_role;
    out_plan->layer[layer].output_activation_role = output_role;
    out_plan->layer[layer].persistent_resource_count = PROM_M48_RESOURCE_COUNT;
    out_plan->layer[layer].intermediate_readback_count =
        request->audit_stage != PROM_M48_AUDIT_STAGE_NONE && layer + 1u == request->layer_count
            ? 1u : 0u;
    out_plan->layer[layer].input_generation = layer == 0u
                                                  ? request->expected_initial_generation
                                                  : out_plan->layer[layer - 1u].output_generation;
    out_plan->layer[layer].replay_id = prom_m47_generation(layer_hash, layer,
                                                           PROM_M48_RESOURCE_COUNT);
    out_plan->layer[layer].output_generation =
        prom_m47_generation(out_plan->layer[layer].input_generation,
                            out_plan->layer[layer].replay_id,
                            ((uint64_t)request->activation_strategy << 32u) | output_role);
    hash = prom_m47_hash_u64(hash, out_plan->layer[layer].replay_id);
    hash = prom_m47_hash_u64(hash, out_plan->layer[layer].output_generation);
    if (layer + 1u < request->layer_count) {
      prom_m48_boundary_trace* boundary = &out_plan->boundary[layer];
      boundary->boundary = layer;
      boundary->producer_layer = layer;
      boundary->consumer_layer = layer + 1u;
      boundary->physical_activation_role = output_role;
      boundary->byte_offset = 0u;
      boundary->byte_length = activation_range_bytes;
      boundary->source_stage_mask = VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT;
      boundary->destination_stage_mask = VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT;
      boundary->source_access_mask = VK_ACCESS_SHADER_WRITE_BIT;
      boundary->destination_access_mask = VK_ACCESS_SHADER_READ_BIT;
      boundary->source_queue_family = VK_QUEUE_FAMILY_IGNORED;
      boundary->destination_queue_family = VK_QUEUE_FAMILY_IGNORED;
      boundary->content_generation = out_plan->layer[layer].output_generation;
    }
  }
  out_plan->final_output_generation = out_plan->layer[request->layer_count - 1u].output_generation;
  out_plan->command_plan_replay_id = prom_m47_generation(
      hash, ((uint64_t)out_plan->submit_count << 32u) | out_plan->semaphore_count,
      ((uint64_t)out_plan->memory.timestamp_query_count << 32u) |
          out_plan->memory.descriptor_set_count);
  out_plan->replay_id = prom_m47_generation(out_plan->command_plan_replay_id,
                                            out_plan->final_output_generation,
                                            out_plan->memory.exact_retained_bytes);
  out_plan->eligibility_eligible = 1u;
  out_plan->eligibility_reason = PROM_M48_ELIGIBLE;
  out_plan->eligibility_replay_id = prom_m47_generation(out_plan->replay_id,
                                                         total, PROM_M48_ELIGIBLE);
  return PROM_OK;
}

int prom_m48_transformer_stack_cpu_reference(const prom_m48_reference_request* request,
                                              prom_m48_reference_result* out_result) {
  float* activation[2] = {NULL, NULL};
  float* head_output = NULL;
  float* projected = NULL;
  float* residual = NULL;
  float* normalized = NULL;
  float* inv_rms = NULL;
  float* gate = NULL;
  float* up = NULL;
  float* hidden = NULL;
  float* down = NULL;
  const float* current;
  uint64_t model_elements;
  uint64_t head_elements;
  uint64_t attention_weight_elements;
  uint64_t wo_elements;
  uint64_t ffn_elements;
  uint64_t index;
  uint32_t layer;
  int status = PROM_ERROR;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  out_result->failed_layer = UINT32_MAX;
  if (request == NULL || request->initial_activation == NULL || request->output == NULL ||
      request->layer_count != PROM_M48_LAYER_COUNT || request->tokens == 0u ||
      request->model_width == 0u || request->head_count != PROM_M43_HEAD_COUNT ||
      request->head_dim == 0u || request->head_count * request->head_dim != request->model_width ||
      request->ffn_width == 0u || !isfinite(request->epsilon) || request->epsilon <= 0.0f ||
      request->precision_policy < PROM_M42_PRECISION_F16_ROUNDED ||
      request->precision_policy > PROM_M42_PRECISION_FP32 ||
      request->projection_path < PROM_M47_PROJECTION_COOPERATIVE ||
      request->projection_path > PROM_M47_PROJECTION_CONVENTIONAL_FP16 ||
      !prom_m47_mul_u64(request->tokens, request->model_width, &model_elements) ||
      !prom_m47_mul_u64(request->head_count, request->tokens, &head_elements) ||
      !prom_m47_mul_u64(head_elements, request->head_dim, &head_elements) ||
      !prom_m47_mul_u64(request->model_width, request->head_dim,
                       &attention_weight_elements) ||
      !prom_m47_mul_u64(request->model_width, request->model_width, &wo_elements) ||
      !prom_m47_mul_u64(request->tokens, request->ffn_width, &ffn_elements) ||
      request->initial_element_count != model_elements ||
      request->output_element_count != model_elements ||
      model_elements > SIZE_MAX / sizeof(float) || head_elements > SIZE_MAX / sizeof(float) ||
      ffn_elements > SIZE_MAX / sizeof(float)) return PROM_ERROR;
  for (index = 0u; index < model_elements; ++index) {
    if (!isfinite(request->initial_activation[index])) return PROM_ERROR;
  }
  for (layer = 0u; layer < PROM_M48_LAYER_COUNT; ++layer) {
    uint32_t head;
    if (request->layer[layer].wo == NULL || request->layer[layer].rmsnorm_weight == NULL ||
        request->layer[layer].wgate == NULL || request->layer[layer].wup == NULL ||
        request->layer[layer].wdown == NULL) return PROM_ERROR;
    for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
      uint32_t weight;
      for (weight = 0u; weight < PROM_M43_WEIGHT_KIND_COUNT; ++weight) {
        if (request->layer[layer].attention_weight[head][weight] == NULL) return PROM_ERROR;
      }
    }
  }
  activation[0] = (float*)malloc((size_t)(model_elements * sizeof(float)));
  activation[1] = (float*)malloc((size_t)(model_elements * sizeof(float)));
  head_output = (float*)malloc((size_t)(head_elements * sizeof(float)));
  projected = (float*)malloc((size_t)(model_elements * sizeof(float)));
  residual = (float*)malloc((size_t)(model_elements * sizeof(float)));
  normalized = (float*)malloc((size_t)(model_elements * sizeof(float)));
  inv_rms = (float*)malloc((size_t)((uint64_t)request->tokens * sizeof(float)));
  gate = (float*)malloc((size_t)(ffn_elements * sizeof(float)));
  up = (float*)malloc((size_t)(ffn_elements * sizeof(float)));
  hidden = (float*)malloc((size_t)(ffn_elements * sizeof(float)));
  down = (float*)malloc((size_t)(model_elements * sizeof(float)));
  if (activation[0] == NULL || activation[1] == NULL || head_output == NULL ||
      projected == NULL || residual == NULL || normalized == NULL || inv_rms == NULL ||
      gate == NULL || up == NULL || hidden == NULL || down == NULL) goto cleanup;
  current = request->initial_activation;
  for (layer = 0u; layer < PROM_M48_LAYER_COUNT; ++layer) {
    prom_m43_reference_request attention;
    prom_m43_reference_result attention_result;
    prom_m44_reference_request projection;
    prom_m44_reference_result projection_result;
    prom_m45_reference_request residual_request;
    prom_m46_reference_request norm;
    prom_m47_reference_request ffn;
    float* next = layer + 1u == PROM_M48_LAYER_COUNT
                      ? request->output : activation[layer % 2u];
    uint32_t head;
    memset(&attention, 0, sizeof(attention));
    attention.x = current;
    attention.output = head_output;
    attention.x_element_count = model_elements;
    attention.weight_element_count = attention_weight_elements;
    attention.output_element_count = head_elements;
    attention.head_count = request->head_count;
    attention.tokens = request->tokens;
    attention.model_width = request->model_width;
    attention.head_dim = request->head_dim;
    attention.precision_policy = request->precision_policy;
    for (head = 0u; head < PROM_M43_HEAD_COUNT; ++head) {
      uint32_t weight;
      for (weight = 0u; weight < PROM_M43_WEIGHT_KIND_COUNT; ++weight)
        attention.weight[head][weight] = request->layer[layer].attention_weight[head][weight];
    }
    out_result->failed_layer = layer;
    out_result->failed_stage = 43u;
    if (prom_m43_attention_cpu_reference(&attention, &attention_result) != PROM_OK ||
        attention_result.all_finite == 0u) goto cleanup;
    if (request->audit_stage_output[layer][PROM_M48_AUDIT_STAGE_ATTENTION - 1u] != NULL)
      memcpy(request->audit_stage_output[layer][PROM_M48_AUDIT_STAGE_ATTENTION - 1u],
             head_output, (size_t)(model_elements * sizeof(float)));
    memset(&projection, 0, sizeof(projection));
    projection.head_major = head_output;
    projection.wo = request->layer[layer].wo;
    projection.output = projected;
    projection.head_major_element_count = head_elements;
    projection.wo_element_count = wo_elements;
    projection.output_element_count = model_elements;
    projection.head_count = request->head_count;
    projection.tokens = request->tokens;
    projection.head_dim = request->head_dim;
    projection.model_width = request->model_width;
    projection.precision_policy = request->precision_policy;
    out_result->failed_stage = 44u;
    if (prom_m44_output_projection_cpu_reference(&projection, &projection_result) != PROM_OK ||
        projection_result.all_finite == 0u) goto cleanup;
    if (request->audit_stage_output[layer][PROM_M48_AUDIT_STAGE_OUTPUT_PROJECTION - 1u] != NULL)
      memcpy(request->audit_stage_output[layer][PROM_M48_AUDIT_STAGE_OUTPUT_PROJECTION - 1u],
             projected, (size_t)(model_elements * sizeof(float)));
    memset(&residual_request, 0, sizeof(residual_request));
    residual_request.x = current;
    residual_request.y = projected;
    residual_request.z = residual;
    residual_request.x_element_count = model_elements;
    residual_request.y_element_count = model_elements;
    residual_request.z_element_count = model_elements;
    residual_request.tokens = request->tokens;
    residual_request.model_width = request->model_width;
    residual_request.x_row_stride = request->model_width;
    residual_request.y_row_stride = request->model_width;
    residual_request.z_row_stride = request->model_width;
    out_result->failed_stage = 45u;
    if (prom_m45_residual_cpu_reference(&residual_request) != PROM_OK) goto cleanup;
    if (request->audit_stage_output[layer][PROM_M48_AUDIT_STAGE_FIRST_RESIDUAL - 1u] != NULL)
      memcpy(request->audit_stage_output[layer][PROM_M48_AUDIT_STAGE_FIRST_RESIDUAL - 1u],
             residual, (size_t)(model_elements * sizeof(float)));
    memset(&norm, 0, sizeof(norm));
    norm.z = residual;
    norm.weight = request->layer[layer].rmsnorm_weight;
    norm.n = normalized;
    norm.inv_rms = inv_rms;
    norm.z_element_count = model_elements;
    norm.weight_element_count = request->model_width;
    norm.n_element_count = model_elements;
    norm.tokens = request->tokens;
    norm.model_width = request->model_width;
    norm.z_row_stride = request->model_width;
    norm.n_row_stride = request->model_width;
    norm.epsilon = request->epsilon;
    out_result->failed_stage = 46u;
    if (prom_m46_rmsnorm_cpu_reference(&norm) != PROM_OK) goto cleanup;
    if (request->audit_stage_output[layer][PROM_M48_AUDIT_STAGE_RMSNORM - 1u] != NULL)
      memcpy(request->audit_stage_output[layer][PROM_M48_AUDIT_STAGE_RMSNORM - 1u],
             normalized, (size_t)(model_elements * sizeof(float)));
    memset(&ffn, 0, sizeof(ffn));
    ffn.n = normalized;
    ffn.wgate = request->layer[layer].wgate;
    ffn.wup = request->layer[layer].wup;
    ffn.wdown = request->layer[layer].wdown;
    ffn.gate = gate;
    ffn.up = up;
    ffn.hidden = hidden;
    ffn.down = down;
    ffn.output = next;
    ffn.n_element_count = model_elements;
    ffn.wgate_element_count = (uint64_t)request->model_width * request->ffn_width;
    ffn.wup_element_count = ffn.wgate_element_count;
    ffn.wdown_element_count = ffn.wgate_element_count;
    ffn.output_element_count = model_elements;
    ffn.tokens = request->tokens;
    ffn.model_width = request->model_width;
    ffn.ffn_width = request->ffn_width;
    ffn.n_row_stride = request->model_width;
    ffn.output_row_stride = request->model_width;
    ffn.projection_path = request->projection_path;
    out_result->failed_stage = 47u;
    if (prom_m47_gated_ffn_cpu_reference(&ffn) != PROM_OK) goto cleanup;
    if (request->audit_stage_output[layer][PROM_M48_AUDIT_STAGE_FFN - 1u] != NULL)
      memcpy(request->audit_stage_output[layer][PROM_M48_AUDIT_STAGE_FFN - 1u],
             next, (size_t)(model_elements * sizeof(float)));
    if (request->audit_layer_output[layer] != NULL)
      memcpy(request->audit_layer_output[layer], next,
             (size_t)(model_elements * sizeof(float)));
    current = next;
    out_result->completed_layer_count = layer + 1u;
  }
  out_result->failed_layer = UINT32_MAX;
  out_result->failed_stage = 0u;
  out_result->all_finite = 1u;
  status = PROM_OK;
cleanup:
  free(down);
  free(hidden);
  free(up);
  free(gate);
  free(inv_rms);
  free(normalized);
  free(residual);
  free(projected);
  free(head_output);
  free(activation[1]);
  free(activation[0]);
  return status;
}
