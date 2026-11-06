/* Compile-time research executor only. Included once by reactor_vulkan_model_block.c
   after its private helpers; never a separate translation unit or production
   runtime fallback. Keep experimental qualification separate from Concept M8. */
#if !defined(PROM_MAIN_EXPERIMENTAL_EXECUTOR)
#error "Main research executor requires an explicit experimental build"
#endif

typedef struct prom_model_block_main_qk_constants {
  float epsilon;
  uint32_t token_count;
  uint32_t head_count;
  uint32_t head_width;
  uint32_t segment_offset;
  uint32_t image_tokens;
} prom_model_block_main_qk_constants;

static int prom_main_transformer_create_experimental_pipelines(prom_reduction_runtime_state* state,
                                                  prom_model_block_state* block) {
  const prom_vk_buffer* ingress_buffers[] = {&block->input_bf16_device,
                                              &block->timestep_bf16_device,
                                              &block->input_device,
                                              &block->timestep_device};
  const prom_vk_buffer* adaln_buffers[] = {&block->timestep_device, &block->weights[1u].device,
                                           &block->weights[0u].device, &block->adaln_projection,
                                           &block->attention_scale, &block->attention_gate,
                                           &block->mlp_scale, &block->mlp_gate};
  const prom_vk_buffer* norm_buffers[] = {&block->input_device, &block->weights[6u].device,
                                          &block->attention_scale, &block->modulated,
                                          &block->norm_audit};
  const prom_vk_buffer* qkv_buffers[] = {&block->modulated, &block->weights[5u].device, &block->qkv};
  const prom_vk_buffer* q_buffers[] = {&block->qkv, &block->weights[4u].device,
                                       &block->norm_audit};
  const prom_vk_buffer* k_buffers[] = {&block->qkv, &block->weights[2u].device,
                                       &block->norm_audit};
  const prom_vk_buffer* attention_buffers[] = {&block->qkv, &block->attention, &block->audit_device};
  const prom_vk_buffer* projection_buffers[] = {&block->attention, &block->weights[3u].device,
                                                 &block->attention_projection};
  const prom_vk_buffer* residual_buffers[] = {&block->attention_projection, &block->weights[7u].device,
                                               &block->attention_gate, &block->input_device,
                                               &block->attention_residual};
  const prom_vk_buffer* ffn_norm_buffers[] = {&block->attention_residual, &block->weights[11u].device,
                                              &block->mlp_scale, &block->norm_audit, &block->modulated};
  const prom_vk_buffer* ffn_projection_buffers[] = {&block->modulated, &block->weights[8u].device,
                                                     &block->weights[10u].device, &block->attention,
                                                     &block->attention_projection, &block->norm_audit,
                                                     &block->qkv};
  const prom_vk_buffer* gate_buffers[] = {&block->qkv, &block->attention,
                                          &block->attention_projection, &block->norm_audit};
  const prom_vk_buffer* w2_buffers[] = {&block->qkv, &block->weights[9u].device,
                                        &block->weights[12u].device, &block->mlp_gate,
                                        &block->attention_residual, &block->input_device,
                                        &block->attention};
#if defined(PROMETHEUS_DVT2_M6A_COOPERATIVE_W1W3_EXPERIMENT)
  const prom_vk_buffer* m6a_pack_buffers[] = {&block->modulated, &block->m6a_modulated_fp16};
  const prom_vk_buffer* m6a_w1_buffers[] = {&block->m6a_modulated_fp16, &block->weights[8u].device,
                                            &block->qkv};
  const prom_vk_buffer* m6a_w3_buffers[] = {&block->m6a_modulated_fp16, &block->weights[10u].device,
                                            &block->m6a_w3_fp32};
#endif
  const uint32_t attention_shader_id = state->compiled_session.main_attention_shader_id;
  if (attention_shader_id != PROM_MODEL_BLOCK_MAIN_ATTENTION_SERIAL_SHADER_ID &&
      attention_shader_id != PROM_MODEL_BLOCK_MAIN_ATTENTION_SUBGROUP_OWNED32_SHADER_ID &&
      attention_shader_id != PROM_MODEL_BLOCK_MAIN_ATTENTION_BUILTIN_TOPOLOGY_SHADER_ID) return 0;
  block->main_attention_shader_id = attention_shader_id;
  int created = prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[0u],
                                               PROM_MODEL_BLOCK_M1B_INGRESS_SHADER_ID, ingress_buffers, 4u,
                                               sizeof(prom_model_block_m1b_ingress_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[1u],
                                               PROM_MODEL_BLOCK_M1B_ADALN_SHADER_ID, adaln_buffers, 8u,
                                               sizeof(prom_model_block_m1b_adaln_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[2u],
                                               PROM_MODEL_BLOCK_M1B_NORM_SHADER_ID, norm_buffers, 5u,
                                               sizeof(prom_model_block_m1b_norm_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[3u],
                                               PROM_MODEL_BLOCK_M1B_QKV_SHADER_ID, qkv_buffers, 3u,
                                               sizeof(prom_model_block_m1b_qkv_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[4u],
                                               PROM_MODEL_BLOCK_MAIN_QK_ROPE_SHADER_ID, q_buffers, 3u,
                                               sizeof(prom_model_block_main_qk_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1b_pipelines[5u],
                                               PROM_MODEL_BLOCK_MAIN_QK_ROPE_SHADER_ID, k_buffers, 3u,
                                               sizeof(prom_model_block_main_qk_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1c_pipelines[0u],
                                               attention_shader_id, attention_buffers, 3u,
                                               sizeof(prom_model_block_m1c_attention_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1c_pipelines[1u],
                                               PROM_MODEL_BLOCK_M1C_PROJECTION_SHADER_ID, projection_buffers, 3u,
                                               sizeof(prom_model_block_m1b_qkv_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1c_pipelines[2u],
                                               PROM_MODEL_BLOCK_M1C_RESIDUAL_SHADER_ID, residual_buffers, 5u,
                                               sizeof(prom_model_block_m1b_norm_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1d_pipelines[0u],
                                               PROM_MODEL_BLOCK_M1D_NORM_SHADER_ID, ffn_norm_buffers, 5u,
                                               sizeof(prom_model_block_m1b_norm_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1d_pipelines[1u],
                                               PROM_MODEL_BLOCK_MAIN_W1_W3_SHADER_ID, ffn_projection_buffers, 7u,
                                               sizeof(prom_model_block_m1b_qkv_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1d_pipelines[2u],
                                               PROM_MODEL_BLOCK_MAIN_GATE_SHADER_ID, gate_buffers, 4u,
                                               sizeof(prom_model_block_m1d_gate_constants)) &&
         prom_model_block_m1b_create_pipeline(state, block, &block->m1d_pipelines[3u],
                                               PROM_MODEL_BLOCK_M1D_W2_RESIDUAL_SHADER_ID, w2_buffers, 7u,
                                               sizeof(prom_model_block_m1d_w2_constants));
#if defined(PROMETHEUS_DVT2_M6A_COOPERATIVE_W1W3_EXPERIMENT)
  if (created) created =
      prom_model_block_m1b_create_pipeline(state, block, &block->m6a_pack_pipeline,
                                            PROM_MODEL_BLOCK_M6A_PACK_SHADER_ID, m6a_pack_buffers, 2u, 8u) &&
      prom_model_block_m1b_create_pipeline(state, block, &block->m6a_w1_pipeline,
                                            PROM_MODEL_BLOCK_M6A_COOPERATIVE_SHADER_ID, m6a_w1_buffers, 3u, 12u) &&
      prom_model_block_m1b_create_pipeline(state, block, &block->m6a_w3_pipeline,
                                            PROM_MODEL_BLOCK_M6A_COOPERATIVE_SHADER_ID, m6a_w3_buffers, 3u, 12u);
#endif
  return created;
}


static int prom_main_transformer_record_experimental(prom_reduction_runtime_state* state,
                                                prom_model_block_state* block,
                                                prom_compiled_model_session_state* session,
                                                uint64_t requested_joint_generation,
                                                uint32_t resident_chain_mode,
                                                int32_t* out_detail) {
  prom_compiled_model_stream_slot* joint;
  VkCommandBufferBeginInfo begin_info;
  VkBufferCopy copies[3];
  VkBufferMemoryBarrier barriers[2];
  prom_model_block_m1b_ingress_constants ingress = {0u, PROM_MODEL_BLOCK_M1B_TIMESTEP_ELEMENTS};
  prom_model_block_m1b_adaln_constants adaln = {15360u, 256u};
  prom_model_block_m1b_norm_constants norm = {1.0e-5f, PROM_MODEL_BLOCK_MAIN_TOKENS,
                                               PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS, 0u};
  prom_model_block_m1b_qkv_constants qkv = {PROM_MODEL_BLOCK_MAIN_TOKENS,
                                             PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS, 11520u, 0u};
  prom_model_block_main_qk_constants q_head = {1.0e-5f, PROM_MODEL_BLOCK_MAIN_TOKENS,
                                                30u, 128u, 0u,
                                                PROM_MODEL_BLOCK_MAIN_IMAGE_TOKENS};
  prom_model_block_main_qk_constants k_head = {1.0e-5f, PROM_MODEL_BLOCK_MAIN_TOKENS,
                                                30u, 128u, 3840u,
                                                PROM_MODEL_BLOCK_MAIN_IMAGE_TOKENS};
  prom_model_block_m1c_attention_constants attention = {PROM_MODEL_BLOCK_MAIN_TOKENS, 30u, 128u, 11520u};
  prom_model_block_m1b_qkv_constants projection = {PROM_MODEL_BLOCK_MAIN_TOKENS,
                                                    PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS, 3840u, 0u};
  prom_model_block_m1b_qkv_constants hidden = {PROM_MODEL_BLOCK_MAIN_TOKENS,
                                                PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS, 10240u, 0u};
  prom_model_block_m1d_gate_constants gate = {PROM_MODEL_BLOCK_MAIN_HIDDEN_ELEMENTS, 0u, 0u, 0u};
  prom_model_block_m1d_w2_constants w2 = {1.0e-5f, PROM_MODEL_BLOCK_MAIN_TOKENS,
                                           PROM_MODEL_BLOCK_M1B_VECTOR_ELEMENTS, 10240u};
  VkResult result;
  uint64_t phase_begin_ns;
  uint64_t record_begin_ns;
  if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_COMMAND_RECORD_FAILED;
  if (state == NULL || block == NULL || session == NULL ||
      prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_COMMAND_RECORD)) return 0;
  joint = prom_compiled_session_slot(session, PROM_ZIMAGE_STREAM_JOINT_WORKING);
  if (joint == NULL || joint->valid == 0u || joint->generation != requested_joint_generation ||
      joint->device.size != PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_MAIN_MODEL_ELEMENTS) ||
      block->input_device.size != joint->device.size || block->timestep_upload.mapped == NULL) return 0;
  phase_begin_ns = prom_reduction_now_ns();
  if (vkResetCommandBuffer(block->command_buffer, 0u) != VK_SUCCESS) return 0;
  block->last_command_reset_ns = prom_reduction_elapsed_ns(phase_begin_ns, prom_reduction_now_ns());
  block->vk_reset_command_buffer_count += 1u;
  memset(&begin_info, 0, sizeof(begin_info));
  begin_info.sType = VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO;
  begin_info.flags = VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT;
  phase_begin_ns = prom_reduction_now_ns();
  if (vkBeginCommandBuffer(block->command_buffer, &begin_info) != VK_SUCCESS) return 0;
  block->last_command_begin_ns = prom_reduction_elapsed_ns(phase_begin_ns, prom_reduction_now_ns());
  record_begin_ns = prom_reduction_now_ns();
  if (block->m1b_timestamp_supported != 0u) {
    vkCmdResetQueryPool(block->command_buffer, block->m1b_timestamp_query_pool, 0u,
                        PROM_MODEL_BLOCK_MAIN_QUERY_COUNT);
    vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                        block->m1b_timestamp_query_pool, 0u);
  }
  memset(copies, 0, sizeof(copies));
  copies[0].size = joint->device.size;
  copies[1].size = block->timestep_upload.size;
  vkCmdCopyBuffer(block->command_buffer, joint->device.buffer, block->input_device.buffer, 1u, &copies[0]);
  vkCmdCopyBuffer(block->command_buffer, block->timestep_upload.buffer,
                  block->timestep_bf16_device.buffer, 1u, &copies[1]);
  memset(barriers, 0, sizeof(barriers));
  barriers[0].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
  barriers[0].srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
  barriers[0].dstAccessMask = VK_ACCESS_SHADER_READ_BIT | VK_ACCESS_SHADER_WRITE_BIT;
  barriers[0].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barriers[0].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
  barriers[0].buffer = block->input_device.buffer;
  barriers[0].size = block->input_device.size;
  barriers[1] = barriers[0];
  barriers[1].buffer = block->timestep_bf16_device.buffer;
  barriers[1].size = block->timestep_bf16_device.size;
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                       VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0u, 0u, NULL, 2u, barriers, 0u, NULL);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(
      block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
      block->m1b_timestamp_query_pool, 1u);
  if ((block->test_flags & PROM_TESTCFG_FAIL_DISPATCH) != 0u) {
    if (out_detail != NULL) *out_detail = PROM_MODEL_BLOCK_DETAIL_INGRESS_DISPATCH_FAILED;
    return 0;
  }
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[0u],
                                         &ingress, sizeof(ingress), 1u, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 2u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[1u],
                                         &adaln, sizeof(adaln), 60u, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 3u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[2u],
                                         &norm, sizeof(norm), PROM_MODEL_BLOCK_MAIN_TOKENS, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 4u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[3u],
                                         &qkv, sizeof(qkv), 66u, 720u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 5u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[4u],
                                         &q_head, sizeof(q_head), 31680u, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 6u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1b_pipelines[5u],
                                         &k_head, sizeof(k_head), 31680u, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 7u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1c_pipelines[0u],
                                         &attention, sizeof(attention),
                                         (block->main_attention_shader_id == PROM_MODEL_BLOCK_MAIN_ATTENTION_SUBGROUP_OWNED32_SHADER_ID ||
                                          block->main_attention_shader_id == PROM_MODEL_BLOCK_MAIN_ATTENTION_BUILTIN_TOPOLOGY_SHADER_ID
                                             ? PROM_MODEL_BLOCK_MAIN_ATTENTION_SUBGROUP_OWNED32_GROUPS
                                             : PROM_MODEL_BLOCK_MAIN_ATTENTION_SERIAL_GROUPS), 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 8u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1c_pipelines[1u],
                                         &projection, sizeof(projection), 66u, 240u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 9u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1c_pipelines[2u],
                                         &norm, sizeof(norm), PROM_MODEL_BLOCK_MAIN_TOKENS, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 10u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[0u],
                                         &norm, sizeof(norm), PROM_MODEL_BLOCK_MAIN_TOKENS, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 11u);
  prom_reduction_record_barrier(block->command_buffer);
#if defined(PROMETHEUS_DVT2_M6A_COOPERATIVE_W1W3_EXPERIMENT)
  {
    const char* m6a_route = getenv("OCT_DVT2_M6A_W1W3_ROUTE");
    const int use_cooperative = m6a_route != NULL && strcmp(m6a_route, "cooperative") == 0;
    if (use_cooperative) {
      const prom_model_block_m6a_pack_constants pack = {
          PROM_MODEL_BLOCK_MAIN_MODEL_ELEMENTS,
          PROM_MODEL_BLOCK_MAIN_MODEL_ELEMENTS / 2u};
      const prom_model_block_m6a_cooperative_constants cooperative = {
          PROM_MODEL_BLOCK_MAIN_TOKENS, 10240u, 3840u};
      prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m6a_pack_pipeline,
                                             &pack, sizeof(pack), 7920u, 1u, 1u);
      if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(
          block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
          block->m1b_timestamp_query_pool, 16u);
      prom_reduction_record_barrier(block->command_buffer);
      prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m6a_w1_pipeline,
                                             &cooperative, sizeof(cooperative), 66u, 640u, 1u);
      prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m6a_w3_pipeline,
                                             &cooperative, sizeof(cooperative), 66u, 640u, 1u);
      if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(
          block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
          block->m1b_timestamp_query_pool, 17u);
      {
        VkBufferMemoryBarrier to_transfer[2];
        VkBufferMemoryBarrier to_compute[3];
        VkBufferCopy segments[3];
        const VkDeviceSize segment_bytes =
            PROM_MODEL_BLOCK_M1B_FP32_BYTES(PROM_MODEL_BLOCK_MAIN_MODEL_ELEMENTS);
        memset(to_transfer, 0, sizeof(to_transfer));
        for (uint32_t source = 0u; source < 2u; ++source) {
          to_transfer[source].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
          to_transfer[source].srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
          to_transfer[source].dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
          to_transfer[source].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
          to_transfer[source].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
        }
        to_transfer[0].buffer = block->m6a_w3_fp32.buffer;
        to_transfer[0].size = block->m6a_w3_fp32.size;
        to_transfer[1].buffer = block->qkv.buffer;
        to_transfer[1].size = block->qkv.size;
        vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                             VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 2u,
                             to_transfer, 0u, NULL);
        memset(segments, 0, sizeof(segments));
        segments[0].size = segment_bytes;
        segments[1].srcOffset = segment_bytes;
        segments[1].size = segment_bytes;
        segments[2].srcOffset = segment_bytes * 2u;
        segments[2].size = block->m6a_w3_fp32.size - segment_bytes * 2u;
        vkCmdCopyBuffer(block->command_buffer, block->m6a_w3_fp32.buffer,
                        block->attention.buffer, 1u, &segments[0]);
        vkCmdCopyBuffer(block->command_buffer, block->m6a_w3_fp32.buffer,
                        block->attention_projection.buffer, 1u, &segments[1]);
        vkCmdCopyBuffer(block->command_buffer, block->m6a_w3_fp32.buffer,
                        block->norm_audit.buffer, 1u, &segments[2]);
        if (block->m6a_raw_audit.buffer != VK_NULL_HANDLE) {
          VkBufferCopy audit_copy;
          memset(&audit_copy, 0, sizeof(audit_copy));
          audit_copy.size = block->m6a_w3_fp32.size;
          vkCmdCopyBuffer(block->command_buffer, block->qkv.buffer,
                          block->m6a_raw_audit.buffer, 1u, &audit_copy);
          audit_copy.dstOffset = block->m6a_w3_fp32.size;
          vkCmdCopyBuffer(block->command_buffer, block->m6a_w3_fp32.buffer,
                          block->m6a_raw_audit.buffer, 1u, &audit_copy);
        }
        memset(to_compute, 0, sizeof(to_compute));
        for (uint32_t segment = 0u; segment < 3u; ++segment) {
          to_compute[segment].sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
          to_compute[segment].srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
          to_compute[segment].dstAccessMask = VK_ACCESS_SHADER_READ_BIT;
          to_compute[segment].srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
          to_compute[segment].dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
        }
        to_compute[0].buffer = block->attention.buffer;
        to_compute[0].size = block->attention.size;
        to_compute[1].buffer = block->attention_projection.buffer;
        to_compute[1].size = block->attention_projection.size;
        to_compute[2].buffer = block->norm_audit.buffer;
        to_compute[2].size = block->norm_audit.size;
        vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                             VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, 0u, 0u, NULL, 3u,
                             to_compute, 0u, NULL);
        if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(
            block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
            block->m1b_timestamp_query_pool, 18u);
      }
    } else {
      prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[1u],
                                             &hidden, sizeof(hidden), 66u, 640u, 1u);
      if (block->m1b_timestamp_supported != 0u) {
        vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                            block->m1b_timestamp_query_pool, 16u);
        vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                            block->m1b_timestamp_query_pool, 17u);
        vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                            block->m1b_timestamp_query_pool, 18u);
      }
    }
  }
#else
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[1u],
                                         &hidden, sizeof(hidden), 66u, 640u, 1u);
#endif
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 12u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[2u],
                                         &gate, sizeof(gate), 42240u, 1u, 1u);
#if defined(PROMETHEUS_DVT2_M6A_COOPERATIVE_W1W3_EXPERIMENT)
  if (block->m6a_raw_audit.buffer != VK_NULL_HANDLE) {
    VkBufferMemoryBarrier capture_barrier;
    VkBufferCopy capture_copy;
    memset(&capture_barrier, 0, sizeof(capture_barrier));
    capture_barrier.sType = VK_STRUCTURE_TYPE_BUFFER_MEMORY_BARRIER;
    capture_barrier.srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
    capture_barrier.dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
    capture_barrier.srcQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    capture_barrier.dstQueueFamilyIndex = VK_QUEUE_FAMILY_IGNORED;
    capture_barrier.buffer = block->qkv.buffer;
    capture_barrier.size = block->m6a_w3_fp32.size;
    vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                         VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 1u,
                         &capture_barrier, 0u, NULL);
    memset(&capture_copy, 0, sizeof(capture_copy));
    capture_copy.dstOffset = block->m6a_w3_fp32.size * 2u;
    capture_copy.size = block->m6a_w3_fp32.size;
    vkCmdCopyBuffer(block->command_buffer, block->qkv.buffer,
                    block->m6a_raw_audit.buffer, 1u, &capture_copy);
    capture_barrier.srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
    capture_barrier.dstAccessMask = VK_ACCESS_HOST_READ_BIT;
    capture_barrier.buffer = block->m6a_raw_audit.buffer;
    capture_barrier.size = block->m6a_raw_audit.size;
    vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                         VK_PIPELINE_STAGE_HOST_BIT, 0u, 0u, NULL, 1u,
                         &capture_barrier, 0u, NULL);
  }
#endif
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 13u);
  prom_reduction_record_barrier(block->command_buffer);
  prom_model_block_m1b_bind_and_dispatch(block->command_buffer, &block->m1d_pipelines[3u],
                                         &w2, sizeof(w2), PROM_MODEL_BLOCK_MAIN_TOKENS, 1u, 1u);
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT, block->m1b_timestamp_query_pool, 14u);
  prom_reduction_record_barrier(block->command_buffer);
  /* W2 writes its projected intermediate to input_device and the final FP32
     gated residual to attention. Chain mode copies that final buffer back to
     the lock-owned JointWorking slot before completion, making attention and
     and JointWorking the two fixed ping/pong states for the next layer. */
  if (resident_chain_mode != 0u) {
  barriers[0].srcAccessMask = VK_ACCESS_SHADER_WRITE_BIT;
  barriers[0].dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT;
  barriers[0].buffer = block->attention.buffer;
  barriers[0].size = block->attention.size;
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       VK_PIPELINE_STAGE_TRANSFER_BIT, 0u, 0u, NULL, 1u, barriers, 0u, NULL);
  memset(&copies[2], 0, sizeof(copies[2]));
  copies[2].size = joint->device.size;
  vkCmdCopyBuffer(block->command_buffer, block->attention.buffer, joint->device.buffer, 1u, &copies[2]);
  barriers[0].srcAccessMask = VK_ACCESS_TRANSFER_WRITE_BIT;
  barriers[0].dstAccessMask = VK_ACCESS_TRANSFER_READ_BIT | VK_ACCESS_SHADER_READ_BIT;
  barriers[0].buffer = joint->device.buffer;
  barriers[0].size = joint->device.size;
  vkCmdPipelineBarrier(block->command_buffer, VK_PIPELINE_STAGE_TRANSFER_BIT,
                       VK_PIPELINE_STAGE_TRANSFER_BIT | VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
                       0u, 0u, NULL, 1u, barriers, 0u, NULL);
  }
  if (block->m1b_timestamp_supported != 0u) vkCmdWriteTimestamp(
      block->command_buffer,
      resident_chain_mode != 0u ? VK_PIPELINE_STAGE_TRANSFER_BIT : VK_PIPELINE_STAGE_COMPUTE_SHADER_BIT,
      block->m1b_timestamp_query_pool, 15u);
  block->last_command_record_ns = prom_reduction_elapsed_ns(record_begin_ns, prom_reduction_now_ns());
  phase_begin_ns = prom_reduction_now_ns();
  result = (block->test_flags & PROM_TESTCFG_FAIL_COMMAND_END) != 0u
               ? VK_ERROR_INITIALIZATION_FAILED
               : vkEndCommandBuffer(block->command_buffer);
  block->last_command_end_ns = prom_reduction_elapsed_ns(phase_begin_ns, prom_reduction_now_ns());
  if (result != VK_SUCCESS) return 0;
  return prom_model_block_submit_and_wait(state, block, out_detail);
}

