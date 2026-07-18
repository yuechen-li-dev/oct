#include "reactor_vulkan_runtime_internal.h"
#include "reactor_shader_registry.h"

#include <limits.h>

/* EVT-2 M1a deliberately owns one compiled command program.  This is not a
   graph executor: the create request must spell the one closed seven-step
   program below and the only accepted shader is the production resident proof.
   Subsequent model milestones replace that proof portfolio with additional
   fixed program steps; they do not add runtime operator selection. */

#define PROM_MODEL_BLOCK_RESIDENT_SHADER_ID 23u
#define PROM_MODEL_BLOCK_WORKGROUP_SIZE 256u

typedef struct prom_model_block_push_constants {
  uint32_t element_count;
  uint32_t audit_element_count;
} prom_model_block_push_constants;

static const uint32_t k_prom_model_block_steps[PROM_MODEL_BLOCK_MAX_STEPS] = {
    PROM_MODEL_BLOCK_STEP_BIND_PIPELINE,
    PROM_MODEL_BLOCK_STEP_BIND_RESOURCES,
    PROM_MODEL_BLOCK_STEP_PUSH_CONSTANTS,
    PROM_MODEL_BLOCK_STEP_DISPATCH,
    PROM_MODEL_BLOCK_STEP_BARRIER,
    PROM_MODEL_BLOCK_STEP_AUDIT_COPY,
    PROM_MODEL_BLOCK_STEP_OUTPUT_COPY,
};

static uint64_t prom_model_block_hash_u64(uint64_t hash, uint64_t value) {
  return prom_m40b_hash_u64(hash, value);
}

static int prom_model_block_add_bytes(uint64_t* total, uint64_t value) {
  if (total == NULL || value > UINT64_MAX - *total) return 0;
  *total += value;
  return 1;
}

static int prom_model_block_bytes_for_elements(uint64_t elements, uint64_t* bytes) {
  if (bytes == NULL || elements == 0u || elements > UINT64_MAX / sizeof(float)) return 0;
  *bytes = elements * sizeof(float);
  return 1;
}

static int prom_model_block_steps_are_exact(const PrometheusModelBlockCreateRequest* request) {
  uint32_t index;
  if (request == NULL || request->step_count != PROM_MODEL_BLOCK_MAX_STEPS) return 0;
  for (index = 0u; index < PROM_MODEL_BLOCK_MAX_STEPS; ++index) {
    if (request->steps[index] != k_prom_model_block_steps[index]) return 0;
  }
  return 1;
}

static uint64_t prom_model_block_weight_bytes(const prom_model_block_state* block) {
  uint64_t total = 0u;
  uint32_t index;
  if (block == NULL) return 0u;
  for (index = 0u; index < block->weight_count; ++index) {
    if (!prom_model_block_add_bytes(&total, block->weights[index].byte_count)) return 0u;
  }
  return total;
}

static void prom_model_block_fill_evidence(const prom_model_block_state* block,
                                           int32_t detail,
                                           PrometheusModelBlockEvidence* out_evidence) {
  uint64_t persistent_bytes;
  uint64_t reusable_bytes;
  uint64_t audit_bytes;
  uint64_t external_bytes;
  uint64_t total_bytes;
  if (out_evidence == NULL) return;
  memset(out_evidence, 0, sizeof(*out_evidence));
  out_evidence->struct_size = (uint32_t)sizeof(*out_evidence);
  out_evidence->detail_code = detail;
  if (block == NULL) return;
  persistent_bytes = prom_model_block_weight_bytes(block);
  reusable_bytes = (uint64_t)block->input_upload.size + (uint64_t)block->input_device.size +
                   (uint64_t)block->output_device.size + (uint64_t)block->output_readback.size +
                   (uint64_t)block->weight_upload.size;
  audit_bytes = (uint64_t)block->audit_device.size + (uint64_t)block->audit_readback.size;
  external_bytes = block->external_input_bytes + block->external_output_bytes;
  total_bytes = persistent_bytes + reusable_bytes + audit_bytes;
  out_evidence->created = block->created;
  out_evidence->weights_uploaded = block->weights_uploaded;
  out_evidence->quarantined = block->quarantined;
  out_evidence->output_valid = block->output_valid;
  out_evidence->audit_valid = block->audit_valid;
  out_evidence->fixed_step_count = block->step_count;
  out_evidence->weight_count = block->weight_count;
  out_evidence->persistent_bytes = persistent_bytes;
  out_evidence->reusable_bytes = reusable_bytes;
  out_evidence->audit_bytes = audit_bytes;
  out_evidence->external_bytes = external_bytes;
  out_evidence->total_committed_bytes = total_bytes;
  out_evidence->peak_plan_bytes = total_bytes;
  out_evidence->cold_buffer_allocation_count = block->cold_buffer_allocation_count;
  out_evidence->warm_buffer_allocation_count = block->warm_buffer_allocation_count;
  out_evidence->pipeline_create_count = block->pipeline_create_count;
  out_evidence->descriptor_set_count = block->descriptor_set_count;
  out_evidence->weight_upload_count = block->weight_upload_count;
  out_evidence->execution_count = block->execution_count;
  out_evidence->last_execution_ns = block->last_execution_ns;
  out_evidence->execution_plan_identity = block->execution_plan_identity;
  out_evidence->replay_identity = block->replay_identity;
}

static void prom_model_block_mark_failure(prom_model_block_state* block, int32_t detail) {
  if (block == NULL) return;
  block->output_valid = 0u;
  block->audit_valid = 0u;
  block->last_detail_code = detail;
}

static int prom_model_block_validate_create_request(const PrometheusModelBlockCreateRequest* request,
                                                    int32_t* out_detail) {
  uint64_t total_weight_bytes = 0u;
  uint64_t total_bytes = 0u;
  uint32_t index;
  if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST;
  if (request == NULL || request->struct_size != sizeof(*request) ||
      request->model_contract_identity == 0u || request->weight_identity == 0u ||
      request->shader_portfolio_identity == 0u || request->precision_policy_identity == 0u ||
      request->capability_route_identity == 0u || request->memory_ceiling_bytes == 0u ||
      request->shader_id != PROM_MODEL_BLOCK_RESIDENT_SHADER_ID || request->weight_count == 0u ||
      request->weight_count > PROM_MODEL_BLOCK_MAX_WEIGHTS || request->external_input_bytes == 0u ||
      request->external_output_bytes == 0u || request->audit_bytes == 0u ||
      (request->external_input_bytes % sizeof(float)) != 0u ||
      (request->external_output_bytes % sizeof(float)) != 0u ||
      (request->audit_bytes % sizeof(float)) != 0u ||
      !prom_model_block_steps_are_exact(request)) return 0;
  for (index = 0u; index < request->weight_count; ++index) {
    const PrometheusModelBlockWeightDeclaration* weight = &request->weights[index];
    if (weight->content_identity == 0u || weight->layout_identity == 0u || weight->byte_count == 0u ||
        !prom_model_block_add_bytes(&total_weight_bytes, weight->byte_count)) return 0;
  }
  if (!prom_model_block_add_bytes(&total_bytes, total_weight_bytes) ||
      !prom_model_block_add_bytes(&total_bytes, request->external_input_bytes) ||
      !prom_model_block_add_bytes(&total_bytes, request->external_input_bytes) ||
      !prom_model_block_add_bytes(&total_bytes, request->external_output_bytes) ||
      !prom_model_block_add_bytes(&total_bytes, request->external_output_bytes) ||
      !prom_model_block_add_bytes(&total_bytes, request->audit_bytes) ||
      !prom_model_block_add_bytes(&total_bytes, request->audit_bytes) ||
      !prom_model_block_add_bytes(&total_bytes, total_weight_bytes) ||
      total_bytes > request->memory_ceiling_bytes) return 0;
  return 1;
}

static uint64_t prom_model_block_plan_identity(const prom_model_block_state* block) {
  uint64_t hash = 1469598103934665603ull;
  uint32_t index;
  hash = prom_model_block_hash_u64(hash, block->model_contract_identity);
  hash = prom_model_block_hash_u64(hash, block->weight_identity);
  hash = prom_model_block_hash_u64(hash, block->shader_portfolio_identity);
  hash = prom_model_block_hash_u64(hash, block->precision_policy_identity);
  hash = prom_model_block_hash_u64(hash, block->capability_route_identity);
  hash = prom_model_block_hash_u64(hash, block->shader_id);
  hash = prom_model_block_hash_u64(hash, block->external_input_bytes);
  hash = prom_model_block_hash_u64(hash, block->external_output_bytes);
  hash = prom_model_block_hash_u64(hash, block->declared_audit_bytes);
  for (index = 0u; index < block->weight_count; ++index) {
    hash = prom_model_block_hash_u64(hash, block->weights[index].content_identity);
    hash = prom_model_block_hash_u64(hash, block->weights[index].layout_identity);
    hash = prom_model_block_hash_u64(hash, block->weights[index].byte_count);
  }
  for (index = 0u; index < block->step_count; ++index) {
    hash = prom_model_block_hash_u64(hash, block->steps[index]);
  }
  return hash;
}

static int prom_model_block_create_buffer(prom_reduction_runtime_state* state,
                                          prom_model_block_state* block,
                                          prom_vk_buffer* buffer,
                                          VkDeviceSize size,
                                          VkBufferUsageFlags usage,
                                          VkMemoryPropertyFlags properties,
                                          int map_memory) {
  VkResult result;
  if (state == NULL || block == NULL || buffer == NULL || size == 0u) return 0;
  result = prom_vk_create_buffer(state->physical_device, state->device, block->test_flags, size,
                                 usage, properties, map_memory, buffer);
  if (result != VK_SUCCESS) return 0;
  block->cold_buffer_allocation_count += 1u;
  return 1;
}

static int prom_model_block_create_descriptor_resources(prom_reduction_runtime_state* state,
                                                        prom_model_block_state* block) {
  VkDescriptorSetLayoutBinding bindings[3];
  VkDescriptorSetLayoutCreateInfo layout_info;
  VkDescriptorPoolSize pool_size;
  VkDescriptorPoolCreateInfo pool_info;
  VkDescriptorSetAllocateInfo set_info;
  VkPushConstantRange push_range;
  VkPipelineLayoutCreateInfo pipeline_layout_info;
  VkDescriptorBufferInfo buffer_infos[3];
  VkWriteDescriptorSet writes[3];
  uint32_t index;
  if (state == NULL || block == NULL) return 0;
  memset(bindings, 0, sizeof(bindings));
  for (index = 0u; index < 3u; ++index) {
    bindings[index].binding = index;
    bindings[index].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
    bindings[index].descriptorCount = 1u;
    bindings[index].stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
  }
  memset(&layout_info, 0, sizeof(layout_info));
  layout_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO;
  layout_info.bindingCount = 3u;
  layout_info.pBindings = bindings;
  if (vkCreateDescriptorSetLayout(state->device, &layout_info, NULL, &block->descriptor_set_layout) != VK_SUCCESS) return 0;
  memset(&pool_size, 0, sizeof(pool_size));
  pool_size.type = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
  pool_size.descriptorCount = 3u;
  memset(&pool_info, 0, sizeof(pool_info));
  pool_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO;
  pool_info.maxSets = 1u;
  pool_info.poolSizeCount = 1u;
  pool_info.pPoolSizes = &pool_size;
  if (vkCreateDescriptorPool(state->device, &pool_info, NULL, &block->descriptor_pool) != VK_SUCCESS) return 0;
  memset(&set_info, 0, sizeof(set_info));
  set_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO;
  set_info.descriptorPool = block->descriptor_pool;
  set_info.descriptorSetCount = 1u;
  set_info.pSetLayouts = &block->descriptor_set_layout;
  if (vkAllocateDescriptorSets(state->device, &set_info, &block->descriptor_set) != VK_SUCCESS) return 0;
  block->descriptor_set_count = 1u;
  memset(&push_range, 0, sizeof(push_range));
  push_range.stageFlags = VK_SHADER_STAGE_COMPUTE_BIT;
  push_range.size = sizeof(prom_model_block_push_constants);
  memset(&pipeline_layout_info, 0, sizeof(pipeline_layout_info));
  pipeline_layout_info.sType = VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO;
  pipeline_layout_info.setLayoutCount = 1u;
  pipeline_layout_info.pSetLayouts = &block->descriptor_set_layout;
  pipeline_layout_info.pushConstantRangeCount = 1u;
  pipeline_layout_info.pPushConstantRanges = &push_range;
  if (vkCreatePipelineLayout(state->device, &pipeline_layout_info, NULL, &block->pipeline_layout) != VK_SUCCESS) return 0;
  memset(buffer_infos, 0, sizeof(buffer_infos));
  buffer_infos[0].buffer = block->input_device.buffer;
  buffer_infos[0].range = block->input_device.size;
  buffer_infos[1].buffer = block->output_device.buffer;
  buffer_infos[1].range = block->output_device.size;
  buffer_infos[2].buffer = block->audit_device.buffer;
  buffer_infos[2].range = block->audit_device.size;
  memset(writes, 0, sizeof(writes));
  for (index = 0u; index < 3u; ++index) {
    writes[index].sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET;
    writes[index].dstSet = block->descriptor_set;
    writes[index].dstBinding = index;
    writes[index].descriptorCount = 1u;
    writes[index].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER;
    writes[index].pBufferInfo = &buffer_infos[index];
  }
  vkUpdateDescriptorSets(state->device, 3u, writes, 0u, NULL);
  return 1;
}

static int prom_model_block_create_pipeline(prom_reduction_runtime_state* state,
                                            prom_model_block_state* block) {
  const prom_shader_asset* asset;
  VkShaderModuleCreateInfo module_info;
  VkPipelineShaderStageCreateInfo stage_info;
  VkComputePipelineCreateInfo pipeline_info;
  if (state == NULL || block == NULL) return 0;
  asset = prom_shader_registry_find_shader(block->shader_id);
  if (asset == NULL || asset->stage != PROM_SHADER_STAGE_COMPUTE ||
      asset->authority != PROM_SHADER_AUTHORITY_PRODUCTION || asset->source_language != PROM_SHADER_SOURCE_SDSLV ||
      asset->descriptor_binding_count != 3u || asset->push_constant_bytes != sizeof(prom_model_block_push_constants) ||
      asset->spirv_words == NULL || asset->spirv_size_bytes == 0u || asset->entry_point == NULL ||
      (block->test_flags & PROM_TESTCFG_FAIL_PIPELINE_CREATE) != 0u) return 0;
  memset(&module_info, 0, sizeof(module_info));
  module_info.sType = VK_STRUCTURE_TYPE_SHADER_MODULE_CREATE_INFO;
  module_info.codeSize = asset->spirv_size_bytes;
  module_info.pCode = asset->spirv_words;
  if (vkCreateShaderModule(state->device, &module_info, NULL, &block->pipeline.shader_module) != VK_SUCCESS) return 0;
  memset(&stage_info, 0, sizeof(stage_info));
  stage_info.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO;
  stage_info.stage = VK_SHADER_STAGE_COMPUTE_BIT;
  stage_info.module = block->pipeline.shader_module;
  stage_info.pName = asset->entry_point;
  memset(&pipeline_info, 0, sizeof(pipeline_info));
  pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO;
  pipeline_info.stage = stage_info;
  pipeline_info.layout = block->pipeline_layout;
  if (vkCreateComputePipelines(state->device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL,
                               &block->pipeline.pipeline) != VK_SUCCESS) return 0;
  block->pipeline.shader_id = asset->shader_id;
  block->pipeline_create_count = 1u;
  return 1;
}

static int prom_model_block_create_command_resources(prom_reduction_runtime_state* state,
                                                     prom_model_block_state* block) {
  VkCommandBufferAllocateInfo command_info;
  VkFenceCreateInfo fence_info;
  if (state == NULL || block == NULL) return 0;
  memset(&command_info, 0, sizeof(command_info));
  command_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO;
  command_info.commandPool = state->command_pool;
  command_info.level = VK_COMMAND_BUFFER_LEVEL_PRIMARY;
  command_info.commandBufferCount = 1u;
  if (vkAllocateCommandBuffers(state->device, &command_info, &block->command_buffer) != VK_SUCCESS) return 0;
  memset(&fence_info, 0, sizeof(fence_info));
  fence_info.sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO;
  fence_info.flags = VK_FENCE_CREATE_SIGNALED_BIT;
  return vkCreateFence(state->device, &fence_info, NULL, &block->fence) == VK_SUCCESS;
}

static int prom_model_block_submit_and_wait(prom_reduction_runtime_state* state,
                                            prom_model_block_state* block,
                                            int32_t* out_detail) {
  VkSubmitInfo submit_info;
  VkResult result;
  if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_QUEUE_SUBMIT_FAILED;
  if (state == NULL || block == NULL || block->command_buffer == VK_NULL_HANDLE || block->fence == VK_NULL_HANDLE) return 0;
  result = vkResetFences(state->device, 1u, &block->fence);
  if (result != VK_SUCCESS) {
    if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_COMMAND_RECORD_FAILED;
    return 0;
  }
  memset(&submit_info, 0, sizeof(submit_info));
  submit_info.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO;
  submit_info.commandBufferCount = 1u;
  submit_info.pCommandBuffers = &block->command_buffer;
  if ((block->test_flags & PROM_TESTCFG_FAIL_QUEUE_SUBMIT) != 0u ||
      prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_QUEUE_SUBMIT)) {
    if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_QUEUE_SUBMIT_FAILED;
    return 0;
  }
  result = vkQueueSubmit(state->queue, 1u, &submit_info, block->fence);
  if (result != VK_SUCCESS) return 0;
  if ((block->test_flags & PROM_TESTCFG_SKIP_SUBMIT_WAIT) != 0u ||
      prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_COMPLETION_OBSERVATION)) {
    block->quarantined = 1u;
    if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN;
    return 0;
  }
  result = vkWaitForFences(state->device, 1u, &block->fence, VK_TRUE, UINT64_MAX);
  if (result != VK_SUCCESS) {
    block->quarantined = 1u;
    if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN;
    return 0;
  }
  return 1;
}

static int prom_model_block_reap(prom_reduction_runtime_state* state,
                                 prom_model_block_state* block) {
  if (state == NULL || block == NULL) return 0;
  if (block->quarantined == 0u) return 1;
  if (block->fence == VK_NULL_HANDLE ||
      vkWaitForFences(state->device, 1u, &block->fence, VK_TRUE, UINT64_MAX) != VK_SUCCESS) return 0;
  block->quarantined = 0u;
  block->output_valid = 0u;
  block->audit_valid = 0u;
  return 1;
}

static int prom_model_block_record_upload(prom_reduction_runtime_state* state,
                                          prom_model_block_state* block,
                                          uint32_t weight_index) {
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copy;
  VkResult result;
  int32_t detail;
  if (state == NULL || block == NULL || weight_index >= block->weight_count) return 0;
  if (vkResetCommandBuffer(block->command_buffer, 0u) != VK_SUCCESS) return 0;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(block->command_buffer, &begin_info) != VK_SUCCESS) return 0;
  memset(&copy, 0, sizeof(copy));
  copy.size = (VkDeviceSize)block->weights[weight_index].byte_count;
  vkCmdCopyBuffer(block->command_buffer, block->weight_upload.buffer,
                  block->weights[weight_index].device.buffer, 1u, &copy);
  result = (block->test_flags & PROM_TESTCFG_FAIL_COMMAND_END) != 0u
               ? VK_ERROR_INITIALIZATION_FAILED
               : vkEndCommandBuffer(block->command_buffer);
  if (result != VK_SUCCESS) return 0;
  return prom_model_block_submit_and_wait(state, block, &detail);
}

static int prom_model_block_record_execute(prom_reduction_runtime_state* state,
                                           prom_model_block_state* block,
                                           uint32_t element_count,
                                           uint32_t audit_element_count,
                                           uint32_t audit_enabled,
                                           int32_t* out_detail) {
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy input_copy;
  VkBufferCopy output_copy;
  VkBufferMemoryBarrier barrier;
  prom_model_block_push_constants constants;
  VkResult result;
  if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_COMMAND_RECORD_FAILED;
  if (state == NULL || block == NULL || element_count == 0u) return 0;
  if (prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_COMMAND_RECORD)) return 0;
  if (vkResetCommandBuffer(block->command_buffer, 0u) != VK_SUCCESS) return 0;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  if (vkBeginCommandBuffer(block->command_buffer, &begin_info) != VK_SUCCESS) return 0;
  memset(&input_copy, 0, sizeof(input_copy));
  input_copy.size = block->external_input_bytes;
  vkCmdCopyBuffer(block->command_buffer, block->input_upload.buffer, block->input_device.buffer, 1u, &input_copy);
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = block->input_device.buffer;
  barrier.size = VK_WHOLE_SIZE;
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  vkCmdBindPipeline(block->command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, block->pipeline.pipeline);
  vkCmdBindDescriptorSets(block->command_buffer, VK_PIPELINE_BIND_POINT_COMPUTE, block->pipeline_layout,
                          0u, 1u, &block->descriptor_set, 0u, NULL);
  constants.element_count = element_count;
  constants.audit_element_count = audit_enabled != 0u ? audit_element_count : 0u;
  vkCmdPushConstants(block->command_buffer, block->pipeline_layout, VK_SHADER_STAGE_COMPUTE_BIT,
                     0u, sizeof(constants), &constants);
  if ((block->test_flags & PROM_TESTCFG_FAIL_DISPATCH) != 0u) return 0;
  vkCmdDispatch(block->command_buffer, prom_reduction_ceil_div_u32(element_count, PROM_MODEL_BLOCK_WORKGROUP_SIZE), 1u, 1u);
  memset(&barrier, 0, sizeof(barrier));
  barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barrier.srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
  barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
  barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barrier.buffer = block->output_device.buffer;
  barrier.size = VK_WHOLE_SIZE;
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
  memset(&output_copy, 0, sizeof(output_copy));
  output_copy.size = block->external_output_bytes;
  vkCmdCopyBuffer(block->command_buffer, block->output_device.buffer, block->output_readback.buffer, 1u, &output_copy);
  if (audit_enabled != 0u) {
    barrier.buffer = block->audit_device.buffer;
    vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                         VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
    output_copy.size = block->declared_audit_bytes;
    vkCmdCopyBuffer(block->command_buffer, block->audit_device.buffer, block->audit_readback.buffer, 1u, &output_copy);
  }
  result = (block->test_flags & PROM_TESTCFG_FAIL_COMMAND_END) != 0u
               ? VK_ERROR_INITIALIZATION_FAILED
               : vkEndCommandBuffer(block->command_buffer);
  if (result != VK_SUCCESS) return 0;
  return prom_model_block_submit_and_wait(state, block, out_detail);
}

void prom_model_block_cleanup_state(prom_reduction_runtime_state* state) {
  prom_model_block_state* block;
  uint64_t next_block_id;
  uint32_t index;
  if (state == NULL) return;
  block = &state->model_block;
  next_block_id = block->next_block_id;
  if (state->device != VK_NULL_HANDLE && block->fence != VK_NULL_HANDLE && block->quarantined != 0u) {
    (void)vkWaitForFences(state->device, 1u, &block->fence, VK_TRUE, UINT64_MAX);
  }
  if (state->device != VK_NULL_HANDLE) {
    for (index = 0u; index < PROM_MODEL_BLOCK_MAX_WEIGHTS; ++index) {
      prom_vk_destroy_buffer(state->device, &block->weights[index].device);
    }
    prom_vk_destroy_buffer(state->device, &block->weight_upload);
    prom_vk_destroy_buffer(state->device, &block->audit_readback);
    prom_vk_destroy_buffer(state->device, &block->audit_device);
    prom_vk_destroy_buffer(state->device, &block->output_readback);
    prom_vk_destroy_buffer(state->device, &block->output_device);
    prom_vk_destroy_buffer(state->device, &block->input_device);
    prom_vk_destroy_buffer(state->device, &block->input_upload);
    prom_reduction_destroy_pipeline(state->device, &block->pipeline);
    if (block->fence != VK_NULL_HANDLE) vkDestroyFence(state->device, block->fence, NULL);
    if (block->command_buffer != VK_NULL_HANDLE && state->command_pool != VK_NULL_HANDLE) {
      vkFreeCommandBuffers(state->device, state->command_pool, 1u, &block->command_buffer);
    }
    if (block->pipeline_layout != VK_NULL_HANDLE) vkDestroyPipelineLayout(state->device, block->pipeline_layout, NULL);
    if (block->descriptor_pool != VK_NULL_HANDLE) vkDestroyDescriptorPool(state->device, block->descriptor_pool, NULL);
    if (block->descriptor_set_layout != VK_NULL_HANDLE) vkDestroyDescriptorSetLayout(state->device, block->descriptor_set_layout, NULL);
  }
  memset(block, 0, sizeof(*block));
  block->next_block_id = next_block_id == 0u ? 1u : next_block_id;
}

int prom_reactor_runtime_model_block_create_impl(
    void* handle, const PrometheusModelBlockCreateRequest* request, uint64_t* out_block_id,
    PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  prom_vk_runtime_services services;
  uint64_t max_weight_bytes = 0u;
  uint64_t next_block_id;
  uint32_t index;
  int32_t detail = PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST;
  if (out_block_id != NULL) *out_block_id = 0u;
  if (!prom_model_block_validate_create_request(request, &detail) || out_block_id == NULL ||
      !prom_reactor_runtime_validate_handle(handle)) {
    prom_model_block_fill_evidence(NULL, detail, out_evidence);
    return PROM_ERROR;
  }
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL || prom_reactor_runtime_get_vk_services(handle, &services) != PROM_OK) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_RESOURCE_CREATE_FAILED, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  if (block->created != 0u) {
    prom_model_block_fill_evidence(block, PROM_MODEL_BLOCK_DETAIL_ALREADY_CREATED, out_evidence);
    return PROM_ERROR;
  }
  next_block_id = block->next_block_id;
  memset(block, 0, sizeof(*block));
  block->next_block_id = next_block_id == 0u ? 1u : next_block_id;
  block->test_flags = services.test_flags | state->model_block_create_test_flags;
  state->model_block_create_test_flags = 0u;
  block->model_contract_identity = request->model_contract_identity;
  block->weight_identity = request->weight_identity;
  block->shader_portfolio_identity = request->shader_portfolio_identity;
  block->precision_policy_identity = request->precision_policy_identity;
  block->capability_route_identity = request->capability_route_identity;
  block->memory_ceiling_bytes = request->memory_ceiling_bytes;
  block->external_input_bytes = request->external_input_bytes;
  block->external_output_bytes = request->external_output_bytes;
  block->declared_audit_bytes = request->audit_bytes;
  block->shader_id = request->shader_id;
  block->weight_count = request->weight_count;
  block->step_count = request->step_count;
  memcpy(block->steps, request->steps, sizeof(block->steps));
  for (index = 0u; index < block->weight_count; ++index) {
    block->weights[index].content_identity = request->weights[index].content_identity;
    block->weights[index].layout_identity = request->weights[index].layout_identity;
    block->weights[index].byte_count = request->weights[index].byte_count;
    if (block->weights[index].byte_count > max_weight_bytes) max_weight_bytes = block->weights[index].byte_count;
  }
  if (!prom_model_block_create_buffer(state, block, &block->input_upload, (VkDeviceSize)block->external_input_bytes,
                                      VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                                      VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1) ||
      !prom_model_block_create_buffer(state, block, &block->input_device, (VkDeviceSize)block->external_input_bytes,
                                      VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                                      VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_model_block_create_buffer(state, block, &block->output_device, (VkDeviceSize)block->external_output_bytes,
                                      VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                                      VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_model_block_create_buffer(state, block, &block->output_readback, (VkDeviceSize)block->external_output_bytes,
                                      VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                                      VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1) ||
      !prom_model_block_create_buffer(state, block, &block->audit_device, (VkDeviceSize)block->declared_audit_bytes,
                                      VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                                      VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0) ||
      !prom_model_block_create_buffer(state, block, &block->audit_readback, (VkDeviceSize)block->declared_audit_bytes,
                                      VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                                      VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1) ||
      !prom_model_block_create_buffer(state, block, &block->weight_upload, (VkDeviceSize)max_weight_bytes,
                                      VK_BUFFER_USAGE_TRANSFER_SRC_BIT,
                                      VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1)) goto fail;
  for (index = 0u; index < block->weight_count; ++index) {
    if (!prom_model_block_create_buffer(state, block, &block->weights[index].device,
                                        (VkDeviceSize)block->weights[index].byte_count,
                                        VK_BUFFER_USAGE_STORAGE_BUFFER_BIT | VK_BUFFER_USAGE_TRANSFER_DST_BIT,
                                        VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0)) goto fail;
  }
  if (!prom_model_block_create_descriptor_resources(state, block)) goto pipeline_fail;
  if (!prom_model_block_create_pipeline(state, block)) goto pipeline_fail;
  if (!prom_model_block_create_command_resources(state, block)) goto fail;
  block->block_id = block->next_block_id++;
  block->created = 1u;
  block->execution_plan_identity = prom_model_block_plan_identity(block);
  block->last_detail_code = 0;
  *out_block_id = block->block_id;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;

pipeline_fail:
  detail = PROM_MODEL_BLOCK_DETAIL_PIPELINE_CREATE_FAILED;
  goto fail;
fail:
  prom_model_block_cleanup_state(state);
  prom_model_block_fill_evidence(NULL, detail == 0 ? PROM_MODEL_BLOCK_DETAIL_RESOURCE_CREATE_FAILED : detail, out_evidence);
  return PROM_ERROR;
}

int prom_reactor_runtime_model_block_upload_weights_impl(
    void* handle, uint64_t block_id, const PrometheusModelBlockWeightUpload* uploads,
    uint32_t upload_count, PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  uint32_t index;
  if (!prom_reactor_runtime_validate_handle(handle) || uploads == NULL) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  if (block->weights_uploaded != 0u || block->quarantined != 0u || upload_count != block->weight_count ||
      (block->test_flags & PROM_TESTCFG_FAIL_UPLOAD) != 0u) {
    prom_model_block_mark_failure(block, (block->test_flags & PROM_TESTCFG_FAIL_UPLOAD) != 0u
                                             ? PROM_MODEL_BLOCK_DETAIL_UPLOAD_FAILED
                                             : PROM_MODEL_BLOCK_DETAIL_WEIGHT_MISMATCH);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  for (index = 0u; index < upload_count; ++index) {
    const PrometheusModelBlockWeightUpload* upload = &uploads[index];
    if (upload->binding_index != index || upload->bytes == NULL ||
        upload->byte_count != block->weights[index].byte_count ||
        upload->content_identity != block->weights[index].content_identity ||
        upload->layout_identity != block->weights[index].layout_identity ||
        upload->byte_count > (uint64_t)block->weight_upload.size) {
      prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_WEIGHT_MISMATCH);
      prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
      return PROM_ERROR;
    }
  }
  for (index = 0u; index < upload_count; ++index) {
    memcpy(block->weight_upload.mapped, uploads[index].bytes, (size_t)uploads[index].byte_count);
    if (!prom_model_block_record_upload(state, block, index)) {
      prom_model_block_mark_failure(block, block->quarantined != 0u
                                               ? PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN
                                               : PROM_MODEL_BLOCK_DETAIL_UPLOAD_FAILED);
      prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
      return PROM_ERROR;
    }
    block->weights[index].uploaded = 1u;
    block->weight_upload_count += 1u;
  }
  block->weights_uploaded = 1u;
  block->last_detail_code = 0;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
}

int prom_reactor_runtime_model_block_execute_impl(
    void* handle, uint64_t block_id, const PrometheusModelBlockExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  prom_model_block_state* block;
  uint64_t input_bytes;
  uint64_t audit_bytes;
  uint64_t begin_ns;
  uint32_t audit_elements;
  int32_t execution_detail;
  if (!prom_reactor_runtime_validate_handle(handle) || request == NULL ||
      request->struct_size != sizeof(*request) || request->input == NULL || request->output == NULL ||
      request->input_identity == 0u || request->audit_enabled > 1u ||
      !prom_model_block_bytes_for_elements(request->element_count, &input_bytes)) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST, out_evidence);
    return PROM_ERROR;
  }
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  block = &state->model_block;
  if (block->weights_uploaded == 0u || input_bytes != block->external_input_bytes ||
      input_bytes != block->external_output_bytes || request->element_count > 4194304u) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_STALE_OUTPUT);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  if (!prom_model_block_reap(state, block)) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_COMPLETION_UNCERTAIN);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  audit_elements = (uint32_t)(block->declared_audit_bytes / sizeof(float));
  audit_bytes = request->audit_element_capacity * sizeof(float);
  if (request->audit_enabled != 0u && (request->audit_output == NULL || audit_bytes < block->declared_audit_bytes)) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_AUDIT_FAILED);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  block->output_valid = 0u;
  block->audit_valid = 0u;
  memcpy(block->input_upload.mapped, request->input, (size_t)input_bytes);
  begin_ns = prom_reduction_now_ns();
  if (!prom_model_block_record_execute(state, block, (uint32_t)request->element_count, audit_elements,
                                       request->audit_enabled, &execution_detail)) {
    prom_model_block_mark_failure(block, execution_detail);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  if ((block->test_flags & PROM_TESTCFG_FAIL_DOWNLOAD) != 0u) {
    prom_model_block_mark_failure(block, PROM_MODEL_BLOCK_DETAIL_AUDIT_FAILED);
    prom_model_block_fill_evidence(block, block->last_detail_code, out_evidence);
    return PROM_ERROR;
  }
  memcpy(request->output, block->output_readback.mapped, (size_t)block->external_output_bytes);
  block->output_valid = 1u;
  if (request->audit_enabled != 0u) {
    memcpy(request->audit_output, block->audit_readback.mapped, (size_t)block->declared_audit_bytes);
    block->audit_valid = 1u;
  }
  block->execution_count += 1u;
  block->last_execution_ns = prom_reduction_elapsed_ns(begin_ns, prom_reduction_now_ns());
  block->replay_identity = prom_model_block_hash_u64(block->execution_plan_identity, request->input_identity);
  block->replay_identity = prom_model_block_hash_u64(block->replay_identity, request->audit_enabled);
  block->last_detail_code = 0;
  prom_model_block_fill_evidence(block, 0, out_evidence);
  return PROM_OK;
}

int prom_reactor_runtime_model_block_get_evidence_impl(void* handle, uint64_t block_id,
                                                        PrometheusModelBlockEvidence* out_evidence) {
  prom_reduction_runtime_state* state;
  if (out_evidence == NULL || !prom_reactor_runtime_validate_handle(handle)) return PROM_ERROR;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) {
    prom_model_block_fill_evidence(NULL, PROM_MODEL_BLOCK_DETAIL_NOT_FOUND, out_evidence);
    return PROM_ERROR;
  }
  prom_model_block_fill_evidence(&state->model_block, state->model_block.last_detail_code, out_evidence);
  return PROM_OK;
}

int prom_reactor_runtime_model_block_destroy_impl(void* handle, uint64_t block_id) {
  prom_reduction_runtime_state* state;
  if (!prom_reactor_runtime_validate_handle(handle)) return PROM_ERROR;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) return PROM_ERROR;
  prom_model_block_cleanup_state(state);
  return PROM_OK;
}

int prom_reactor_runtime_model_block_test_inject_impl(void* handle, uint64_t block_id,
                                                      uint32_t reduction_test_flags) {
  prom_reduction_runtime_state* state;
  if (!prom_reactor_runtime_validate_handle(handle) || reduction_test_flags == 0u) return PROM_ERROR;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) return PROM_ERROR;
  state->reduction_test_flags |= reduction_test_flags;
  return PROM_OK;
}

int prom_reactor_runtime_model_block_test_inject_create_fault_impl(void* handle,
                                                                   uint32_t test_flags) {
  prom_reduction_runtime_state* state;
  int32_t detail = 0;
  if (!prom_reactor_runtime_validate_handle(handle) || test_flags == 0u ||
      (test_flags & ~(PROM_TESTCFG_FAIL_PIPELINE_CREATE | PROM_TESTCFG_FAIL_UPLOAD)) != 0u) return PROM_ERROR;
  state = prom_reduction_ensure_state(handle, &detail);
  if (state == NULL || state->model_block.created != 0u) return PROM_ERROR;
  state->model_block_create_test_flags |= test_flags;
  return PROM_OK;
}

int prom_reactor_runtime_model_block_test_inject_execution_fault_impl(void* handle,
                                                                      uint64_t block_id,
                                                                      uint32_t test_flags) {
  prom_reduction_runtime_state* state;
  if (!prom_reactor_runtime_validate_handle(handle) || test_flags != PROM_TESTCFG_FAIL_DOWNLOAD) return PROM_ERROR;
  state = (prom_reduction_runtime_state*)prom_reactor_runtime_reduction_state(handle);
  if (state == NULL || state->model_block.created == 0u || state->model_block.block_id != block_id) return PROM_ERROR;
  state->model_block.test_flags |= test_flags;
  return PROM_OK;
}
