#include "reactor_vulkan.h"
#include "reactor_shader_package.h"

#include <string.h>

#define PROM_FFT_DIRECTION_FORWARD 1u
#define PROM_FFT_DIRECTION_INVERSE 2u
#define PROM_FFT_MAX_PLAN_PASSES 20u
#define PROM_FFT_MAX_ELEMENT_COUNT (1u << PROM_FFT_MAX_PLAN_PASSES)
#define PROM_FFT_RADIX_BASELINE 2u
#define PROM_FFT_TWIDDLE_MODE_NATIVE_SIN_COS 2u
#define PROM_FFT_SHADER_BIT_REVERSE 52u
#define PROM_FFT_SHADER_BUTTERFLY 53u

#define PROM_FFT_BUFFER_ROLE_NONE 0u
#define PROM_FFT_BUFFER_ROLE_INPUT 1u
#define PROM_FFT_BUFFER_ROLE_PING 2u
#define PROM_FFT_BUFFER_ROLE_PONG 3u
#define PROM_FFT_BUFFER_ROLE_OUTPUT 4u

typedef struct prom_fft_plan_pass { uint32_t span, half_span, radix, source_role, destination_role; } prom_fft_plan_pass;
typedef struct prom_fft_plan {
  uint32_t element_count, batch_count, effective_stride_elements, direction;
  uint32_t log2_element_count, pass_count, bit_reversal_required, final_output_role, ping_pong_swap_count;
  prom_fft_plan_pass passes[PROM_FFT_MAX_PLAN_PASSES];
} prom_fft_plan;
typedef struct prom_fft_runtime_diag_slot { void* handle; PrometheusFftDiagnostics diag; } prom_fft_runtime_diag_slot;
typedef struct prom_fft_bit_reverse_push { uint32_t element_count, batch_count, stride_elements, reserved; } prom_fft_bit_reverse_push;
typedef struct prom_fft_butterfly_push { uint32_t element_count, batch_count, stride_elements, half_span, direction, normalize_inverse, reserved0, reserved1; } prom_fft_butterfly_push;

static prom_fft_runtime_diag_slot g_fft_diag_slots[32];

static PrometheusFftDiagnostics prom_fft_default_diag(void) {
  PrometheusFftDiagnostics diag; memset(&diag, 0, sizeof(diag));
  diag.struct_size = (uint32_t)sizeof(diag); diag.api_declared = 1u;
  diag.last_failure_detail = PROM_FFT_DETAIL_UNAVAILABLE;
  diag.last_validation_status = PROM_FFT_PATH_STATUS_UNAVAILABLE;
  diag.executed_path_id = PROM_FFT_PATH_UNAVAILABLE;
  return diag;
}

static PrometheusFftDiagnostics* prom_fft_diag_for_handle(void* handle) {
  uint32_t i; prom_fft_runtime_diag_slot* empty = NULL;
  for (i = 0u; i < 32u; ++i) { if (g_fft_diag_slots[i].handle == handle) return &g_fft_diag_slots[i].diag; if (empty == NULL && g_fft_diag_slots[i].handle == NULL) empty = &g_fft_diag_slots[i]; }
  if (empty == NULL) return NULL;
  empty->handle = handle; empty->diag = prom_fft_default_diag(); return &empty->diag;
}

static void prom_fft_stage_request(PrometheusFftDiagnostics* diag, const PrometheusFftRequest* request) {
  if (request == NULL) return;
  diag->last_element_count = request->element_count; diag->last_batch_count = request->batch_count;
  diag->last_stride_elements = request->stride_elements; diag->last_flags = request->flags;
  diag->last_effective_stride_elements = request->stride_elements == 0u ? request->element_count : request->stride_elements;
}
static uint32_t prom_fft_is_power_of_two(uint32_t value) { return value != 0u && (value & (value - 1u)) == 0u; }
static uint32_t prom_fft_log2_u32(uint32_t value) { uint32_t count = 0u; while (value > 1u) { value >>= 1u; ++count; } return count; }

static int prom_fft_validate_request(const PrometheusFftRequest* request, int* out_detail, uint32_t* out_direction, uint64_t* out_bytes) {
  uint64_t physical_elements;
  if (request == NULL || request->struct_size < sizeof(*request)) { *out_detail = PROM_FFT_DETAIL_INVALID_REQUEST; return 0; }
  if (request->input == NULL) { *out_detail = PROM_FFT_DETAIL_NULL_INPUT; return 0; }
  if (request->output == NULL) { *out_detail = PROM_FFT_DETAIL_NULL_OUTPUT; return 0; }
  if (request->element_count == 0u) { *out_detail = PROM_FFT_DETAIL_ZERO_ELEMENT_COUNT; return 0; }
  if (!prom_fft_is_power_of_two(request->element_count)) { *out_detail = PROM_FFT_DETAIL_NON_POWER_OF_TWO; return 0; }
  if (request->element_count > PROM_FFT_MAX_ELEMENT_COUNT) { *out_detail = PROM_FFT_DETAIL_UNSUPPORTED_SIZE; return 0; }
  if (request->batch_count == 0u) { *out_detail = PROM_FFT_DETAIL_ZERO_BATCH_COUNT; return 0; }
  if ((request->flags & PROM_FFT_FLAG_FORWARD) != 0u && (request->flags & PROM_FFT_FLAG_INVERSE) != 0u) { *out_detail = PROM_FFT_DETAIL_INVALID_DIRECTION_FLAGS; return 0; }
  if ((request->flags & PROM_FFT_FLAG_INVERSE_NORMALIZE) != 0u && (request->flags & PROM_FFT_FLAG_INVERSE) == 0u) { *out_detail = PROM_FFT_DETAIL_INVERSE_NORMALIZE_REQUIRES_INVERSE; return 0; }
  if (request->stride_elements != 0u && request->stride_elements < request->element_count) { *out_detail = PROM_FFT_DETAIL_INVALID_STRIDE; return 0; }
  if (request->stride_elements != 0u && request->batch_count > UINT32_MAX / request->stride_elements) { *out_detail = PROM_FFT_DETAIL_SIZE_OVERFLOW; return 0; }
  { uint32_t stride = request->stride_elements == 0u ? request->element_count : request->stride_elements;
    physical_elements = (uint64_t)(request->batch_count - 1u) * (uint64_t)stride + (uint64_t)request->element_count; }
  if (physical_elements == 0u || physical_elements > UINT64_MAX / sizeof(PrometheusComplex32) || physical_elements * sizeof(PrometheusComplex32) > UINT32_MAX) { *out_detail = PROM_FFT_DETAIL_SIZE_OVERFLOW; return 0; }
  *out_direction = (request->flags & PROM_FFT_FLAG_INVERSE) != 0u ? PROM_FFT_DIRECTION_INVERSE : PROM_FFT_DIRECTION_FORWARD;
  *out_bytes = physical_elements * sizeof(PrometheusComplex32); return 1;
}

static void prom_fft_build_plan(const PrometheusFftRequest* request, uint32_t direction, prom_fft_plan* out_plan) {
  uint32_t i, src = PROM_FFT_BUFFER_ROLE_PING, dst = PROM_FFT_BUFFER_ROLE_PONG;
  memset(out_plan, 0, sizeof(*out_plan)); out_plan->element_count = request->element_count; out_plan->batch_count = request->batch_count;
  out_plan->effective_stride_elements = request->stride_elements == 0u ? request->element_count : request->stride_elements;
  out_plan->direction = direction; out_plan->log2_element_count = prom_fft_log2_u32(request->element_count); out_plan->pass_count = out_plan->log2_element_count;
  out_plan->bit_reversal_required = request->element_count > 1u ? 1u : 0u; out_plan->final_output_role = PROM_FFT_BUFFER_ROLE_PING;
  for (i = 0u; i < out_plan->pass_count; ++i) { prom_fft_plan_pass* pass = &out_plan->passes[i]; pass->span = 1u << (i + 1u); pass->half_span = pass->span >> 1u; pass->radix = PROM_FFT_RADIX_BASELINE; pass->source_role = src; pass->destination_role = dst; src = dst; dst = dst == PROM_FFT_BUFFER_ROLE_PING ? PROM_FFT_BUFFER_ROLE_PONG : PROM_FFT_BUFFER_ROLE_PING; }
  if (out_plan->pass_count > 0u) out_plan->final_output_role = out_plan->passes[out_plan->pass_count - 1u].destination_role;
  out_plan->ping_pong_swap_count = out_plan->pass_count;
}

static void prom_fft_apply_plan_diag(PrometheusFftDiagnostics* diag, const prom_fft_plan* plan) {
  diag->plan_valid = 1u; diag->plan_element_count = plan->element_count; diag->plan_log2_element_count = plan->log2_element_count;
  diag->plan_pass_count = plan->pass_count; diag->ping_pong_swap_count = plan->ping_pong_swap_count; diag->final_output_role = plan->final_output_role;
  diag->plan_first_span = plan->pass_count > 0u ? plan->passes[0].span : 0u; diag->plan_last_span = plan->pass_count > 0u ? plan->passes[plan->pass_count - 1u].span : 0u;
  diag->plan_radix_mask = plan->pass_count > 0u ? (1u << PROM_FFT_RADIX_BASELINE) : 0u; diag->plan_bit_reversal_required = plan->bit_reversal_required;
  diag->plan_first_source_role = plan->pass_count > 0u ? plan->passes[0].source_role : PROM_FFT_BUFFER_ROLE_INPUT;
  diag->plan_first_destination_role = plan->pass_count > 0u ? plan->passes[0].destination_role : PROM_FFT_BUFFER_ROLE_PING;
  diag->plan_direction = plan->direction; diag->plan_twiddle_mode = PROM_FFT_TWIDDLE_MODE_NATIVE_SIN_COS;
}

static void prom_fft_barrier(VkCommandBuffer command, VkBuffer buffer, VkAccessFlags source_access, VkAccessFlags destination_access, VkPipelineStageFlags source_stage, VkPipelineStageFlags destination_stage) {
  VkBufferMemoryBarrier barrier; memset(&barrier, 0, sizeof(barrier)); barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER; barrier.srcAccessMask = source_access; barrier.dstAccessMask = destination_access; barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED; barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED; barrier.buffer = buffer; barrier.offset = 0u; barrier.size = VK_WHOLE_SIZE;
  vkCmdPipelineBarrier(command, source_stage, destination_stage, 0u, 0u, NULL, 1u, &barrier, 0u, NULL);
}

static VkResult prom_fft_make_pipeline(prom_shader_package* package, VkDevice device, VkPipelineLayout layout, const char* variant_id, VkPipeline* out_pipeline) {
  VkShaderModule module = VK_NULL_HANDLE; VkComputePipelineCreateInfo pipeline_info; VkResult result; const char* entry = NULL; prom_shader_package_diagnostic diagnostic;
  memset(out_pipeline, 0, sizeof(*out_pipeline));
  if (!prom_shader_package_create_module(package, device, variant_id, &module, &entry, &diagnostic)) return VK_ERROR_INITIALIZATION_FAILED;
  memset(&pipeline_info, 0, sizeof(pipeline_info)); pipeline_info.sType = VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO; pipeline_info.stage.sType = VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO; pipeline_info.stage.stage = VK_SHADER_STAGE_COMPUTE_BIT; pipeline_info.stage.module = module; pipeline_info.stage.pName = entry; pipeline_info.layout = layout;
  result = vkCreateComputePipelines(device, VK_NULL_HANDLE, 1u, &pipeline_info, NULL, out_pipeline); vkDestroyShaderModule(device, module, NULL); return result;
}

static void prom_fft_write_descriptors(VkDevice device, VkDescriptorSet set, VkBuffer input, VkBuffer output, VkDeviceSize bytes) {
  VkDescriptorBufferInfo buffers[2]; VkWriteDescriptorSet writes[2]; memset(buffers, 0, sizeof(buffers)); memset(writes, 0, sizeof(writes));
  buffers[0].buffer = input; buffers[0].range = bytes; buffers[1].buffer = output; buffers[1].range = bytes;
  for (uint32_t i = 0u; i < 2u; ++i) { writes[i].sType = VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET; writes[i].dstSet = set; writes[i].dstBinding = i; writes[i].descriptorCount = 1u; writes[i].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER; writes[i].pBufferInfo = &buffers[i]; }
  vkUpdateDescriptorSets(device, 2u, writes, 0u, NULL);
}

static int prom_fft_execute_vulkan(prom_shader_package* package, const prom_vk_runtime_services* services, const PrometheusFftRequest* request, const prom_fft_plan* plan, uint64_t bytes, uint32_t* out_stage, int* out_detail) {
  prom_vk_buffer input_stage, output_stage, input_device, ping, pong; VkDescriptorSetLayout set_layout = VK_NULL_HANDLE; VkDescriptorPool pool = VK_NULL_HANDLE; VkDescriptorSet set = VK_NULL_HANDLE; VkPipelineLayout pipeline_layout = VK_NULL_HANDLE; VkPipeline bit_pipeline = VK_NULL_HANDLE, butterfly_pipeline = VK_NULL_HANDLE; VkCommandBuffer command = VK_NULL_HANDLE; VkFence fence = VK_NULL_HANDLE; int detail = PROM_FFT_DETAIL_ALLOCATION_FAILED; int result_status = PROM_ERROR; VkResult result;
  memset(&input_stage, 0, sizeof(input_stage)); memset(&output_stage, 0, sizeof(output_stage)); memset(&input_device, 0, sizeof(input_device)); memset(&ping, 0, sizeof(ping)); memset(&pong, 0, sizeof(pong));
  if (services->backend_available == 0u || services->device == VK_NULL_HANDLE || services->compute_queue == VK_NULL_HANDLE || services->compute_command_pool == VK_NULL_HANDLE) { *out_detail = PROM_FFT_DETAIL_VULKAN_UNAVAILABLE; *out_stage = PROM_STAGE_INIT; return PROM_ERROR; }
  if (package == NULL) { *out_detail = PROM_FFT_DETAIL_SHADER_UNAVAILABLE; *out_stage = PROM_STAGE_INIT; return PROM_ERROR; }
  result = prom_vk_create_buffer(services->physical_device, services->device, services->test_flags, bytes, VK_BUFFER_USAGE_TRANSFER_SRC_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT, VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1, &input_stage); if (result != VK_SUCCESS) goto done;
  result = prom_vk_create_buffer(services->physical_device, services->device, services->test_flags, bytes, VK_BUFFER_USAGE_TRANSFER_SRC_BIT | VK_BUFFER_USAGE_TRANSFER_DST_BIT, VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1, &output_stage); if (result != VK_SUCCESS) goto done;
  memcpy(input_stage.mapped, request->input, (size_t)bytes); memcpy(output_stage.mapped, request->output, (size_t)bytes);
  if ((services->test_flags & PROM_TESTCFG_FAIL_UPLOAD) != 0u) { detail = PROM_FFT_DETAIL_UPLOAD_FAILED; *out_stage = PROM_STAGE_TRANSFER_IN; goto done; }
  result = prom_vk_create_buffer(services->physical_device, services->device, services->test_flags, bytes, VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &input_device); if (result != VK_SUCCESS) goto done;
  result = prom_vk_create_buffer(services->physical_device, services->device, services->test_flags, bytes, VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &ping); if (result != VK_SUCCESS) goto done;
  result = prom_vk_create_buffer(services->physical_device, services->device, services->test_flags, bytes, VK_BUFFER_USAGE_TRANSFER_DST_BIT | VK_BUFFER_USAGE_TRANSFER_SRC_BIT | VK_BUFFER_USAGE_STORAGE_BUFFER_BIT, VK_MEMORY_PROPERTY_DEVICE_LOCAL_BIT, 0, &pong); if (result != VK_SUCCESS) goto done;
  { VkDescriptorSetLayoutBinding bindings[2]; VkDescriptorSetLayoutCreateInfo layout_info; VkDescriptorPoolSize pool_size; VkDescriptorPoolCreateInfo pool_info; VkDescriptorSetAllocateInfo set_info; VkPushConstantRange push; VkPipelineLayoutCreateInfo pipe_layout_info; VkCommandBufferAllocateInfo command_info; VkFenceCreateInfo fence_info;
    memset(bindings, 0, sizeof(bindings)); for (uint32_t i = 0u; i < 2u; ++i) { bindings[i].binding = i; bindings[i].descriptorType = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER; bindings[i].descriptorCount = 1u; bindings[i].stageFlags = VK_SHADER_STAGE_COMPUTE_BIT; }
    memset(&layout_info, 0, sizeof(layout_info)); layout_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO; layout_info.bindingCount = 2u; layout_info.pBindings = bindings; result = vkCreateDescriptorSetLayout(services->device, &layout_info, NULL, &set_layout); if (result != VK_SUCCESS) goto done;
    memset(&pool_size, 0, sizeof(pool_size)); pool_size.type = VK_DESCRIPTOR_TYPE_STORAGE_BUFFER; pool_size.descriptorCount = 2u; memset(&pool_info, 0, sizeof(pool_info)); pool_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO; pool_info.maxSets = 1u; pool_info.poolSizeCount = 1u; pool_info.pPoolSizes = &pool_size; result = vkCreateDescriptorPool(services->device, &pool_info, NULL, &pool); if (result != VK_SUCCESS) goto done;
    memset(&set_info, 0, sizeof(set_info)); set_info.sType = VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO; set_info.descriptorPool = pool; set_info.descriptorSetCount = 1u; set_info.pSetLayouts = &set_layout; result = vkAllocateDescriptorSets(services->device, &set_info, &set); if (result != VK_SUCCESS) goto done;
    memset(&push, 0, sizeof(push)); push.stageFlags = VK_SHADER_STAGE_COMPUTE_BIT; push.size = sizeof(prom_fft_butterfly_push); memset(&pipe_layout_info, 0, sizeof(pipe_layout_info)); pipe_layout_info.sType = VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO; pipe_layout_info.setLayoutCount = 1u; pipe_layout_info.pSetLayouts = &set_layout; pipe_layout_info.pushConstantRangeCount = 1u; pipe_layout_info.pPushConstantRanges = &push; result = vkCreatePipelineLayout(services->device, &pipe_layout_info, NULL, &pipeline_layout); if (result != VK_SUCCESS) goto done;
    if ((services->test_flags & PROM_TESTCFG_FAIL_PIPELINE_CREATE) != 0u) { detail = PROM_FFT_DETAIL_DISPATCH_FAILED; *out_stage = PROM_STAGE_INIT; goto done; }
    result = prom_fft_make_pipeline(package, services->device, pipeline_layout, "kernel-52-default", &bit_pipeline); if (result != VK_SUCCESS) { detail = PROM_FFT_DETAIL_SHADER_UNAVAILABLE; goto done; }
    result = prom_fft_make_pipeline(package, services->device, pipeline_layout, "kernel-53-default", &butterfly_pipeline); if (result != VK_SUCCESS) { detail = PROM_FFT_DETAIL_SHADER_UNAVAILABLE; goto done; }
    memset(&command_info, 0, sizeof(command_info)); command_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO; command_info.commandPool = services->compute_command_pool; command_info.level = VK_COMMAND_BUFFER_LEVEL_PRIMARY; command_info.commandBufferCount = 1u; result = vkAllocateCommandBuffers(services->device, &command_info, &command); if (result != VK_SUCCESS) goto done;
    memset(&fence_info, 0, sizeof(fence_info)); fence_info.sType = VK_STRUCTURE_TYPE_FENCE_CREATE_INFO; result = vkCreateFence(services->device, &fence_info, NULL, &fence); if (result != VK_SUCCESS) goto done;
  }
  { VkCommandBufferBeginInfo begin; VkBufferCopy copy; VkSubmitInfo submit; prom_fft_bit_reverse_push bit_push; prom_fft_butterfly_push butterfly_push; prom_vk_buffer* source = &ping; prom_vk_buffer* destination = &pong;
    memset(&begin, 0, sizeof(begin)); begin.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO; result = vkBeginCommandBuffer(command, &begin); if (result != VK_SUCCESS) { detail = PROM_FFT_DETAIL_DISPATCH_FAILED; *out_stage = PROM_STAGE_SUBMIT; goto done; }
    memset(&copy, 0, sizeof(copy)); copy.size = bytes; vkCmdCopyBuffer(command, input_stage.buffer, input_device.buffer, 1u, &copy); vkCmdCopyBuffer(command, output_stage.buffer, ping.buffer, 1u, &copy); vkCmdCopyBuffer(command, output_stage.buffer, pong.buffer, 1u, &copy);
    prom_fft_barrier(command, input_device.buffer, VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT); prom_fft_barrier(command, ping.buffer, VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_SHADER_WRITE_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT); prom_fft_barrier(command, pong.buffer, VK_ACCESS_TRANSFER_WRITE_BIT, VK_ACCESS_SHADER_WRITE_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
    prom_fft_write_descriptors(services->device, set, input_device.buffer, ping.buffer, bytes); memset(&bit_push, 0, sizeof(bit_push)); bit_push.element_count = plan->element_count; bit_push.batch_count = plan->batch_count; bit_push.stride_elements = plan->effective_stride_elements; vkCmdBindPipeline(command, VK_PIPELINE_BIND_POINT_COMPUTE, bit_pipeline); vkCmdBindDescriptorSets(command, VK_PIPELINE_BIND_POINT_COMPUTE, pipeline_layout, 0u, 1u, &set, 0u, NULL); vkCmdPushConstants(command, pipeline_layout, VK_SHADER_STAGE_COMPUTE_BIT, 0u, sizeof(bit_push), &bit_push); vkCmdDispatch(command, (plan->element_count * plan->batch_count + 255u) / 256u, 1u, 1u);
    prom_fft_barrier(command, ping.buffer, VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT);
    for (uint32_t pass_index = 0u; pass_index < plan->pass_count; ++pass_index) { const prom_fft_plan_pass* pass = &plan->passes[pass_index]; if (pass->source_role == PROM_FFT_BUFFER_ROLE_PONG) source = &pong; else source = &ping; if (pass->destination_role == PROM_FFT_BUFFER_ROLE_PONG) destination = &pong; else destination = &ping; prom_fft_write_descriptors(services->device, set, source->buffer, destination->buffer, bytes); memset(&butterfly_push, 0, sizeof(butterfly_push)); butterfly_push.element_count = plan->element_count; butterfly_push.batch_count = plan->batch_count; butterfly_push.stride_elements = plan->effective_stride_elements; butterfly_push.half_span = pass->half_span; butterfly_push.direction = plan->direction; butterfly_push.normalize_inverse = (plan->direction == PROM_FFT_DIRECTION_INVERSE && pass_index + 1u == plan->pass_count) ? 1u : 0u; vkCmdBindPipeline(command, VK_PIPELINE_BIND_POINT_COMPUTE, butterfly_pipeline); vkCmdBindDescriptorSets(command, VK_PIPELINE_BIND_POINT_COMPUTE, pipeline_layout, 0u, 1u, &set, 0u, NULL); vkCmdPushConstants(command, pipeline_layout, VK_SHADER_STAGE_COMPUTE_BIT, 0u, sizeof(butterfly_push), &butterfly_push); vkCmdDispatch(command, (plan->element_count * plan->batch_count + 255u) / 256u, 1u, 1u); prom_fft_barrier(command, destination->buffer, VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_SHADER_READ_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT); }
    if ((services->test_flags & PROM_TESTCFG_FAIL_DISPATCH) != 0u) { detail = PROM_FFT_DETAIL_DISPATCH_FAILED; *out_stage = PROM_STAGE_SUBMIT; goto done; }
    source = plan->final_output_role == PROM_FFT_BUFFER_ROLE_PONG ? &pong : &ping; prom_fft_barrier(command, source->buffer, VK_ACCESS_SHADER_WRITE_BIT, VK_ACCESS_TRANSFER_READ_BIT, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, VK_PIPELINE_STAGE_TRANSFER_BIT); vkCmdCopyBuffer(command, source->buffer, output_stage.buffer, 1u, &copy);
    result = vkEndCommandBuffer(command); if ((services->test_flags & PROM_TESTCFG_FAIL_COMMAND_END) != 0u) result = VK_ERROR_INITIALIZATION_FAILED; if (result != VK_SUCCESS) { detail = PROM_FFT_DETAIL_DISPATCH_FAILED; *out_stage = PROM_STAGE_SUBMIT; goto done; }
    memset(&submit, 0, sizeof(submit)); submit.sType = VK_STRUCTURE_TYPE_SUBMIT_INFO; submit.commandBufferCount = 1u; submit.pCommandBuffers = &command; result = vkQueueSubmit(services->compute_queue, 1u, &submit, fence); if ((services->test_flags & PROM_TESTCFG_FAIL_QUEUE_SUBMIT) != 0u) result = VK_ERROR_INITIALIZATION_FAILED; if (result != VK_SUCCESS) { detail = PROM_FFT_DETAIL_DISPATCH_FAILED; *out_stage = PROM_STAGE_SUBMIT; goto done; } result = vkWaitForFences(services->device, 1u, &fence, VK_TRUE, UINT64_MAX); if (result != VK_SUCCESS) { detail = PROM_FFT_DETAIL_DISPATCH_FAILED; *out_stage = PROM_STAGE_SUBMIT; goto done; }
  }
  if ((services->test_flags & PROM_TESTCFG_FAIL_DOWNLOAD) != 0u) { detail = PROM_FFT_DETAIL_READBACK_FAILED; *out_stage = PROM_STAGE_TRANSFER_OUT; goto done; }
  memcpy(request->output, output_stage.mapped, (size_t)bytes); *out_stage = PROM_STAGE_TRANSFER_OUT; *out_detail = 0; result_status = PROM_OK;
done:
  if (result_status != PROM_OK) { if (*out_stage == PROM_STAGE_NONE) *out_stage = PROM_STAGE_INIT; *out_detail = detail; }
  if (fence != VK_NULL_HANDLE) vkDestroyFence(services->device, fence, NULL); if (command != VK_NULL_HANDLE) vkFreeCommandBuffers(services->device, services->compute_command_pool, 1u, &command); if (bit_pipeline != VK_NULL_HANDLE) vkDestroyPipeline(services->device, bit_pipeline, NULL); if (butterfly_pipeline != VK_NULL_HANDLE) vkDestroyPipeline(services->device, butterfly_pipeline, NULL); if (pipeline_layout != VK_NULL_HANDLE) vkDestroyPipelineLayout(services->device, pipeline_layout, NULL); if (pool != VK_NULL_HANDLE) vkDestroyDescriptorPool(services->device, pool, NULL); if (set_layout != VK_NULL_HANDLE) vkDestroyDescriptorSetLayout(services->device, set_layout, NULL); prom_vk_destroy_buffer(services->device, &pong); prom_vk_destroy_buffer(services->device, &ping); prom_vk_destroy_buffer(services->device, &input_device); prom_vk_destroy_buffer(services->device, &output_stage); prom_vk_destroy_buffer(services->device, &input_stage); return result_status;
}

int prom_reactor_runtime_fft_impl(void* handle, const PrometheusFftRequest* request, uint32_t* out_stage, int* out_detail_code) {
  PrometheusFftDiagnostics* diag; prom_fft_plan plan; prom_vk_runtime_services services; prom_shader_package* package = NULL; int detail = PROM_FFT_DETAIL_UNAVAILABLE; uint32_t direction = PROM_FFT_DIRECTION_FORWARD; uint64_t bytes = 0u; int status;
  if (!prom_reactor_runtime_validate_handle(handle)) { prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INVALID_HANDLE); return PROM_INVALID_HANDLE; }
  diag = prom_fft_diag_for_handle(handle); if (diag == NULL) { prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_INTERNAL_ERROR); return PROM_INTERNAL_ERROR; }
  *diag = prom_fft_default_diag(); prom_fft_stage_request(diag, request);
  if (!prom_fft_validate_request(request, &detail, &direction, &bytes)) { diag->last_direction = direction; diag->last_failure_detail = detail; prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, detail); return PROM_ERROR; }
  prom_fft_build_plan(request, direction, &plan); prom_fft_apply_plan_diag(diag, &plan); diag->last_direction = direction; diag->requested_path_id = PROM_FFT_PATH_VULKAN_RADIX2_RESERVED; diag->requested_radix = PROM_FFT_RADIX_BASELINE;
  if (prom_reactor_runtime_get_vk_services(handle, &services) != PROM_OK) { diag->last_failure_detail = PROM_FFT_DETAIL_VULKAN_UNAVAILABLE; prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_FFT_DETAIL_VULKAN_UNAVAILABLE); return PROM_ERROR; }
  if (prom_reactor_runtime_get_shader_package(handle, &package) != PROM_OK) { diag->last_failure_detail = PROM_FFT_DETAIL_SHADER_UNAVAILABLE; prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_FFT_DETAIL_SHADER_UNAVAILABLE); return PROM_ERROR; }
  /* M5/M6 has a production-generated shader portfolio and a real Vulkan
     execution route. A particular call can still fail later at allocation,
     upload, dispatch, or readback; that outcome is recorded separately. */
  diag->capability_reported = 1u; diag->production_enabled = 1u; diag->benchmark_enabled = 1u; diag->last_validation_status = PROM_FFT_PATH_STATUS_BENCHMARK_ENABLED;
  status = prom_fft_execute_vulkan(package, &services, request, &plan, bytes, out_stage, out_detail_code);
  if (status == PROM_OK) { diag->executed_path_id = PROM_FFT_PATH_VULKAN_RADIX2_RESERVED; diag->executed_radix = PROM_FFT_RADIX_BASELINE; diag->last_failure_detail = 0; return PROM_OK; }
  diag->executed_path_id = PROM_FFT_PATH_UNAVAILABLE; diag->last_failure_detail = out_detail_code != NULL ? *out_detail_code : PROM_FFT_DETAIL_DISPATCH_FAILED; return PROM_ERROR;
}

int prom_reactor_runtime_fft_benchmark_variant_impl(void* handle, const PrometheusFftRequest* request, uint32_t requested_variant, uint32_t* out_stage, int* out_detail_code) {
  if (requested_variant != PROM_FFT_BENCHMARK_VARIANT_RADIX2) { prom_vk_set_status(out_stage, out_detail_code, PROM_STAGE_INIT, PROM_FFT_DETAIL_PLAN_DISPATCH_DISAGREEMENT); return PROM_ERROR; }
  return prom_reactor_runtime_fft_impl(handle, request, out_stage, out_detail_code);
}

void prom_fft_diag_forget_handle(void* handle) { for (uint32_t i = 0u; handle != NULL && i < 32u; ++i) if (g_fft_diag_slots[i].handle == handle) { g_fft_diag_slots[i].handle = NULL; g_fft_diag_slots[i].diag = prom_fft_default_diag(); return; } }
int prom_reactor_runtime_fft_diagnostics_sized_impl(void* handle, PrometheusFftDiagnostics* out_diag, uint32_t out_size) { PrometheusFftDiagnostics* src; if (out_diag == NULL || out_size == 0u) return PROM_ERROR; if (!prom_reactor_runtime_validate_handle(handle)) return PROM_INVALID_HANDLE; src = prom_fft_diag_for_handle(handle); if (src == NULL) return PROM_INTERNAL_ERROR; if (out_size > sizeof(*out_diag)) out_size = (uint32_t)sizeof(*out_diag); memset(out_diag, 0, out_size); memcpy(out_diag, src, out_size); return PROM_OK; }
int prom_reactor_runtime_fft_diagnostics_impl(void* handle, PrometheusFftDiagnostics* out_diag) { return prom_reactor_runtime_fft_diagnostics_sized_impl(handle, out_diag, (uint32_t)sizeof(*out_diag)); }
