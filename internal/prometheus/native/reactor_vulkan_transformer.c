#include "reactor_vulkan.h"

#include <math.h>
#include <stdint.h>
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

static uint16_t prom_m47_float_to_half(float value) {
  uint32_t bits;
  uint32_t sign;
  uint32_t exponent;
  uint32_t mantissa;
  uint32_t half_exponent;
  uint32_t half_mantissa;
  memcpy(&bits, &value, sizeof(bits));
  sign = (bits >> 16u) & 0x8000u;
  exponent = (bits >> 23u) & 0xffu;
  mantissa = bits & 0x7fffffu;
  if (exponent == 0xffu) return (uint16_t)(sign | (mantissa != 0u ? 0x7e00u : 0x7c00u));
  if (exponent > 142u) return (uint16_t)(sign | 0x7c00u);
  if (exponent < 113u) {
    uint32_t shift;
    uint32_t rounded;
    if (exponent < 103u) return (uint16_t)sign;
    mantissa |= 0x800000u;
    shift = 126u - exponent;
    rounded = mantissa >> shift;
    if (((mantissa >> (shift - 1u)) & 1u) != 0u &&
        ((mantissa & ((1u << (shift - 1u)) - 1u)) != 0u || (rounded & 1u) != 0u)) rounded += 1u;
    return (uint16_t)(sign | rounded);
  }
  half_exponent = exponent - 112u;
  half_mantissa = mantissa >> 13u;
  if ((mantissa & 0x1000u) != 0u &&
      ((mantissa & 0xfffu) != 0x1000u || (half_mantissa & 1u) != 0u)) {
    half_mantissa += 1u;
    if (half_mantissa == 0x400u) {
      half_mantissa = 0u;
      half_exponent += 1u;
      if (half_exponent >= 31u) return (uint16_t)(sign | 0x7c00u);
    }
  }
  return (uint16_t)(sign | (half_exponent << 10u) | half_mantissa);
}

static float prom_m47_half_to_float(uint16_t value) {
  uint32_t sign = ((uint32_t)value & 0x8000u) << 16u;
  uint32_t exponent = ((uint32_t)value >> 10u) & 0x1fu;
  uint32_t mantissa = (uint32_t)value & 0x3ffu;
  uint32_t bits;
  float result;
  if (exponent == 0u) {
    if (mantissa == 0u) bits = sign;
    else {
      exponent = 113u;
      while ((mantissa & 0x400u) == 0u) { mantissa <<= 1u; exponent -= 1u; }
      bits = sign | (exponent << 23u) | ((mantissa & 0x3ffu) << 13u);
    }
  } else if (exponent == 31u) {
    bits = sign | 0x7f800000u | (mantissa << 13u);
  } else {
    bits = sign | ((exponent + 112u) << 23u) | (mantissa << 13u);
  }
  memcpy(&result, &bits, sizeof(result));
  return result;
}

static float prom_m47_round_f16(float value) {
  return prom_m47_half_to_float(prom_m47_float_to_half(value));
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
