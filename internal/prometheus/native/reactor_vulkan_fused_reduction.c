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

/* M39b is a row-wise FP32 reactor.  All dispatches for one request are recorded
   into one slot-owned command buffer and submitted behind one fence. */

#define PROM_REDUCTION_STATE_MAGIC 0x52333942u
#define PROM_REDUCTION_RING_MAX_DEPTH 4u
#define PROM_REDUCTION_PIPELINE_COUNT 5u
#define PROM_REDUCTION_MIN_BINDING_BYTES ((VkDeviceSize)sizeof(float))

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
  VkFence fence;
  VkDescriptorSet descriptor_sets[PROM_REDUCTION_MAX_STAGES];
  prom_vk_buffer input;
  prom_vk_buffer output;
  prom_vk_buffer scratch;
  prom_vk_buffer row_max;
  prom_vk_buffer row_sum;
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
  VkCommandBuffer command_buffers[PROM_REDUCTION_RING_MAX_DEPTH];
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
    if (slot->fence != VK_NULL_HANDLE) vkDestroyFence(device, slot->fence, NULL);
    if (slot->command_buffer != VK_NULL_HANDLE) command_buffers[command_count++] = slot->command_buffer;
  }
  if (command_count > 0u && state->command_pool != VK_NULL_HANDLE) {
    vkFreeCommandBuffers(device, state->command_pool, command_count, command_buffers);
  }
  for (pipeline_index = 0u; pipeline_index < PROM_REDUCTION_PIPELINE_COUNT; ++pipeline_index) {
    prom_reduction_destroy_pipeline(device, &state->pipelines[pipeline_index]);
  }
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
  VkDescriptorSetLayout layouts[PROM_REDUCTION_RING_MAX_DEPTH * PROM_REDUCTION_MAX_STAGES];
  VkDescriptorSet descriptor_sets[PROM_REDUCTION_RING_MAX_DEPTH * PROM_REDUCTION_MAX_STAGES];
  VkDescriptorSetAllocateInfo descriptor_allocate_info;
  VkCommandBuffer command_buffers[PROM_REDUCTION_RING_MAX_DEPTH];
  VkCommandBufferAllocateInfo command_allocate_info;
  VkFenceCreateInfo fence_info;
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
  pool_size.descriptorCount = 4u * state->ring_depth * PROM_REDUCTION_MAX_STAGES;
  memset(&pool_info, 0, sizeof(pool_info));
  pool_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO;
  pool_info.maxSets = state->ring_depth * PROM_REDUCTION_MAX_STAGES;
  pool_info.poolSizeCount = 1u;
  pool_info.pPoolSizes = &pool_size;
  result = vkCreateDescriptorPool(state->device, &pool_info, NULL, &state->descriptor_pool);
  if (result != VK_SUCCESS) goto fail;
  for (descriptor_index = 0u; descriptor_index < state->ring_depth * PROM_REDUCTION_MAX_STAGES; ++descriptor_index) {
    layouts[descriptor_index] = state->descriptor_set_layout;
  }
  memset(&descriptor_allocate_info, 0, sizeof(descriptor_allocate_info));
  descriptor_allocate_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO;
  descriptor_allocate_info.descriptorPool = state->descriptor_pool;
  descriptor_allocate_info.descriptorSetCount = state->ring_depth * PROM_REDUCTION_MAX_STAGES;
  descriptor_allocate_info.pSetLayouts = layouts;
  result = vkAllocateDescriptorSets(state->device, &descriptor_allocate_info, descriptor_sets);
  if (result != VK_SUCCESS) goto fail;

  memset(&command_allocate_info, 0, sizeof(command_allocate_info));
  command_allocate_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO;
  command_allocate_info.commandPool = state->command_pool;
  command_allocate_info.level = VK_COMMAND_BUFFER_LEVEL_PRIMARY;
  command_allocate_info.commandBufferCount = state->ring_depth;
  result = vkAllocateCommandBuffers(state->device, &command_allocate_info, command_buffers);
  if (result != VK_SUCCESS) goto fail;
  memset(&fence_info, 0, sizeof(fence_info));
  fence_info.sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO;
  for (slot_index = 0u; slot_index < state->ring_depth; ++slot_index) {
    uint32_t stage_index;
    prom_reduction_slot* slot = &state->slots[slot_index];
    slot->slot_id = slot_index;
    slot->state = PROM_ASYNC_PHYSICAL_EMPTY;
    slot->command_buffer = command_buffers[slot_index];
    for (stage_index = 0u; stage_index < PROM_REDUCTION_MAX_STAGES; ++stage_index) {
      slot->descriptor_sets[stage_index] = descriptor_sets[slot_index * PROM_REDUCTION_MAX_STAGES + stage_index];
    }
    result = vkCreateFence(state->device, &fence_info, NULL, &slot->fence);
    if (result != VK_SUCCESS) goto fail;
  }
  if (state->timestamp_supported != 0u && state->timestamp_period_ns > 0.0f) {
    memset(&query_info, 0, sizeof(query_info));
    query_info.sType = VK_STRUCTURE_TYPE_QUERY_POOL_CREATE_INFO;
    query_info.queryType = VK_QUERY_TYPE_TIMESTAMP;
    query_info.queryCount = 2u * state->ring_depth;
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

static void prom_reduction_stage_bindings(const prom_reduction_slot* slot,
                                          const PrometheusReductionPlan* plan,
                                          uint32_t stage_index,
                                          prom_reduction_buffer_bindings* out) {
  const PrometheusReductionStageDispatch* stage = &plan->stages[stage_index];
  out->input = &slot->input;
  out->auxiliary0 = &slot->row_max;
  out->auxiliary1 = &slot->row_sum;
  out->output = &slot->output;
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
    vkCmdResetQueryPool(slot->command_buffer, state->query_pool, slot->slot_id * 2u, 2u);
    vkCmdWriteTimestamp(slot->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                        state->query_pool, slot->slot_id * 2u);
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
                        state->query_pool, slot->slot_id * 2u + 1u);
  }
  result = vkEndCommandBuffer(slot->command_buffer);
  if (result != VK_SUCCESS) return 0;
  slot->state = PROM_ASYNC_PHYSICAL_RECORDED;
  state->diagnostics.command_record_count += 1u;
  return 1;
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
    vk_result = vkGetQueryPoolResults(state->device, state->query_pool, slot->slot_id * 2u, 2u,
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
