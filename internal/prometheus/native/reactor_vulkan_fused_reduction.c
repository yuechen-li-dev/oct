/* Public reduction plan serialization, validation/oracles, and the legacy
   model resource lifecycle. Production standalone reduction execution lives
   in reactors/PrometheusExecution.concept and its generated deployment. */
#include "reactor_vulkan_runtime_internal.h"
#include "reactor_shader_registry.h"
#include "reactor_shader_package.h"
#include "generated/prometheusreduction.generated.h"

#include <math.h>
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>

/* The Concept records cross a generated C boundary. Keep public numeric
   vocabulary changes explicit instead of silently reinterpreting a schedule. */
_Static_assert(PROM_REDUCTION_OPERATION_SUM == 1u && PROM_REDUCTION_OPERATION_MAX == 2u &&
               PROM_REDUCTION_OPERATION_SOFTMAX == 3u, "Concept reduction operation ABI");
_Static_assert(PROM_REDUCTION_STRATEGY_FUSED_SINGLE_WORKGROUP == 1u &&
               PROM_REDUCTION_STRATEGY_COMPOSED == 2u && PROM_REDUCTION_STRATEGY_PACKED_SHORT_ROWS == 3u,
               "Concept reduction strategy ABI");
_Static_assert(PROM_REDUCTION_STAGE_ROW_SUM == 1u && PROM_REDUCTION_STAGE_ROW_MAX == 2u &&
               PROM_REDUCTION_STAGE_SOFTMAX_EXP_SUM == 3u && PROM_REDUCTION_STAGE_SOFTMAX_NORMALIZE == 4u &&
               PROM_REDUCTION_STAGE_SOFTMAX_FUSED == 5u && PROM_REDUCTION_STAGE_ROW_SUM_PACKED_SHORT == 6u &&
               PROM_REDUCTION_STAGE_SOFTMAX_PACKED_SHORT == 7u, "Concept reduction stage ABI");
_Static_assert(PROM_REDUCTION_TEMPORARY_NONE == 0u && PROM_REDUCTION_TEMPORARY_PARTIALS == 1u &&
               PROM_REDUCTION_TEMPORARY_ROW_MAX == 2u && PROM_REDUCTION_TEMPORARY_ROW_SUM == 3u,
               "Concept reduction temporary ABI");
_Static_assert(PROM_REDUCTION_MAX_STAGES == 8u && PROM_REDUCTION_LOCAL_SIZE == 256u &&
               PROM_REDUCTION_ELEMENTS_PER_PARTIAL == 1024u && PROM_REDUCTION_SINGLE_STAGE_THRESHOLD == 1024u &&
               sizeof(float) == 4u, "Concept reduction shape ABI");
_Static_assert(PROM_REDUCTION_MAX_ROWS == 1024u && PROM_REDUCTION_MAX_ELEMENTS_PER_ROW == 1048576u &&
               PROM_REDUCTION_MAX_TOTAL_ELEMENTS == 16777216u && PROM_REDUCTION_FORCE_FUSED_MAX_ELEMENTS_PER_ROW == 1056u,
               "Concept reduction admitted envelope");
_Static_assert(PROM_REDUCTION_FLAG_FORCE_FUSED == 1u && PROM_REDUCTION_FLAG_FORCE_COMPOSED == 2u &&
               PROM_REDUCTION_FINALIZATION_NONE == 0u && PROM_REDUCTION_FINALIZATION_STABLE_SOFTMAX == 1u,
               "Concept reduction request ABI");
_Static_assert(PROM_REDUCTION_PACKED_SHORT_WIDTH_MAX == 128u && PROM_REDUCTION_PACKED_SHORT_SUM_MIN_ROWS == 512u &&
               PROM_REDUCTION_PACKED_SHORT_SUM_WIDE_MIN_ROWS == 1024u && PROM_REDUCTION_PACKED_SHORT_ROWS_PER_GROUP == 8u,
               "Concept reduction qualified crossover");

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

/* Concept owns all numeric admission and scheduling decisions. This adapter
   checks caller pointers and serializes the result into the stable public ABI. */
static concept_reduction_shape prom_reduction_shape(const PrometheusReductionRequest* request) {
  const concept_reduction_shape shape = {
      request->row_count, request->elements_per_row, request->operation,
      request->finalization, request->flags,
      request->input_element_count, request->output_element_count};
  return shape;
}

static int prom_reduction_validate_request(const PrometheusReductionRequest* request,
                                           uint32_t require_pointers,
                                           uint64_t* out_total_elements,
                                           uint64_t* out_output_elements,
                                           int32_t* out_detail) {
  int32_t detail;
  if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_INVALID_REQUEST;
  if (request == NULL || request->struct_size < sizeof(*request)) return 0;
  if (require_pointers != 0u && request->input == NULL) {
    if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_NULL_INPUT;
    return 0;
  }
  if (require_pointers != 0u && request->output == NULL) {
    if (out_detail != NULL) *out_detail = PROM_REDUCTION_DETAIL_NULL_OUTPUT;
    return 0;
  }
  detail = concept_prometheus_reduction_validate_shape(prom_reduction_shape(request));
  if (out_detail != NULL) *out_detail = detail;
  if (detail != 0) return 0;
  if (out_total_elements != NULL) *out_total_elements = request->input_element_count;
  if (out_output_elements != NULL) *out_output_elements = request->output_element_count;
  return 1;
}

int prom_reactor_reduction_plan_impl(const PrometheusReductionRequest* request,
                                     PrometheusReductionPlan* out_plan) {
  static const uint32_t shaders[] = {0u,
      PROM_REDUCTION_SHADER_ROW_SUM, PROM_REDUCTION_SHADER_ROW_MAX,
      PROM_REDUCTION_SHADER_SOFTMAX_EXP_SUM, PROM_REDUCTION_SHADER_SOFTMAX_NORMALIZE,
      PROM_REDUCTION_SHADER_SOFTMAX_FUSED, PROM_REDUCTION_SHADER_ROW_SUM_PACKED_SHORT,
      PROM_REDUCTION_SHADER_SOFTMAX_PACKED_SHORT};
  static const uint32_t implementations[] = {0u,
      PROM_REDUCTION_IMPLEMENTATION_ROW_SUM, PROM_REDUCTION_IMPLEMENTATION_ROW_MAX,
      PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_EXP_SUM, PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_NORMALIZE,
      PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_FUSED, PROM_REDUCTION_IMPLEMENTATION_ROW_SUM_PACKED_SHORT,
      PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_PACKED_SHORT};
  concept_reduction_schedule schedule;
  uint32_t index;
  if (out_plan == NULL) return PROM_ERROR;
  memset(out_plan, 0, sizeof(*out_plan));
  out_plan->struct_size = sizeof(*out_plan);
  if (!prom_reduction_validate_request(request, 0u, NULL, NULL, NULL)) return PROM_ERROR;
  schedule = concept_prometheus_reduction_build_schedule(prom_reduction_shape(request));
  if (schedule.count == 0u || schedule.count > PROM_REDUCTION_MAX_STAGES) return PROM_ERROR;
  out_plan->operation = request->operation;
  out_plan->row_count = request->row_count;
  out_plan->elements_per_row = request->elements_per_row;
  out_plan->strategy = schedule.strategy;
  out_plan->local_size = PROM_REDUCTION_LOCAL_SIZE;
  out_plan->elements_per_partial = PROM_REDUCTION_ELEMENTS_PER_PARTIAL;
  out_plan->partial_count = schedule.partials;
  out_plan->stage_count = schedule.count;
  out_plan->temporary_alignment_bytes = sizeof(float);
  out_plan->temporary_bytes = schedule.temporaryBytes;
  for (index = 0u; index < schedule.count; ++index) {
    const concept_reduction_stage* source = &schedule.stages.data[index];
    PrometheusReductionStageDispatch* target = &out_plan->stages[index];
    if (source->role == 0u || source->role >= sizeof(shaders) / sizeof(shaders[0])) return PROM_ERROR;
    target->stage_role = source->role;
    target->shader_id = shaders[source->role];
    target->implementation_id = implementations[source->role];
    target->groups_x = source->groups;
    target->groups_y = 1u;
    target->groups_z = 1u;
    target->input_elements_per_row = source->inputWidth;
    target->output_partials_per_row = source->outputPartials;
    target->temporary_role = source->temporaryRole;
    target->temporary_bytes_written = source->temporaryBytes;
  }
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
    prom_vk_destroy_buffer(device, &slot->m45_x_readback);
    prom_vk_destroy_buffer(device, &slot->m45_output);
    prom_vk_destroy_buffer(device, &slot->m44_readback);
    prom_vk_destroy_buffer(device, &slot->m44_output);
    prom_vk_destroy_buffer(device, &slot->m44_concat_f16);
    prom_vk_destroy_buffer(device, &slot->m44_concat_f32);
    prom_vk_destroy_buffer(device, &slot->m44_concat_upload);
    prom_vk_destroy_buffer(device, &slot->m48_readback);
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
  for (pipeline_index = 0u; pipeline_index < PROM_M47_PIPELINE_COUNT; ++pipeline_index) {
    prom_reduction_destroy_pipeline(device, &state->m47_pipelines[pipeline_index]);
  }
  for (pipeline_index = 0u; pipeline_index < PROM_M45_PIPELINE_COUNT; ++pipeline_index) {
    prom_reduction_destroy_pipeline(device, &state->m45_pipelines[pipeline_index]);
  }
  for (pipeline_index = 0u; pipeline_index < PROM_M44_PIPELINE_COUNT; ++pipeline_index) {
    prom_reduction_destroy_pipeline(device, &state->m44_pipelines[pipeline_index]);
  }
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
