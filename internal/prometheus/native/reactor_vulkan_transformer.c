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
  out_plan->memory.descriptor_set_count = request->layer_count * 134u;
  out_plan->memory.timestamp_query_count = request->layer_count * PROM_M48_QUERY_COUNT_PER_LAYER;
  /* Descriptor sets and query pools are opaque Vulkan objects, not device-buffer
     views.  Their exact counts are capacity checked; inventing byte sizes would
     make the buffer accounting less truthful. */
  out_plan->memory.descriptor_device_buffer_bytes = 0u;
  out_plan->memory.timestamp_query_device_buffer_bytes = 0u;
  if (!prom_m48_accumulate(&slot_bytes, out_plan->memory.activation_bytes) ||
      !prom_m48_accumulate(&slot_bytes, out_plan->memory.one_block_working_set_bytes) ||
      !prom_m48_accumulate(&slot_bytes, out_plan->memory.final_readback_bytes)) return PROM_ERROR;
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
  out_plan->intermediate_host_copy_count = 0u;
  out_plan->intermediate_readback_count = 0u;
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
  hash = prom_m47_hash_u64(hash, request->expected_initial_generation);
  hash = prom_m47_hash_u64(hash, request->initial_content_hash);
  activation_range_bytes = (uint64_t)padded_tokens * padded_model_width * sizeof(float);
  for (layer = 0u; layer < request->layer_count; ++layer) {
    uint64_t layer_hash = 1469598103934665603ull;
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
    layer_hash = prom_m47_hash_u32(layer_hash, request->projection_path);
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
    out_plan->layer[layer].submit_index = request->submit_topology == PROM_M48_SUBMIT_ONE_STACK
                                             ? 0u : layer;
    out_plan->layer[layer].query_begin = layer * PROM_M48_QUERY_COUNT_PER_LAYER;
    out_plan->layer[layer].query_count = PROM_M48_QUERY_COUNT_PER_LAYER;
    out_plan->layer[layer].input_activation_role = input_role;
    out_plan->layer[layer].output_activation_role = output_role;
    out_plan->layer[layer].persistent_resource_count = PROM_M48_RESOURCE_COUNT;
    out_plan->layer[layer].intermediate_readback_count = 0u;
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
