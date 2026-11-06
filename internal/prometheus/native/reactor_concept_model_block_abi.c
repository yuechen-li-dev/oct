/* C11 transport: generated private ownership types never cross into C++. */
#include "reactor_concept_model_block.h"
#include "reactor_vulkan_runtime_internal.h"
#include "generated/prometheusmodelblock.generated.h"
#include "generated/prometheusmodelblock.exports.h"
#include <stdlib.h>

typedef struct block_attachment { concept_block_owner owner; void* host; } block_attachment;

void* prom_concept_block_create(void* host, const prom_vk_runtime_services* s) {
    block_attachment* a = malloc(sizeof(*a));
    concept_device_borrow device = {s->physical_device, s->device, s->compute_queue,
        s->compute_queue_family_index, s->concept_enabled_features};
    concept_result_block_owner_vulkan_error result;
    if (!a) return NULL;
    result = concept_prometheus_model_block_create_block(device, s->timestamp_query_supported != 0u);
    if (result.tag != 0) { free(a); return NULL; }
    a->owner = result.payload.ok.value; a->host = host;
    if (prom_concept_block_configure(host, s->device) != 0) {
        concept_prometheus_model_block_dispose_block(a->owner); free(a); return NULL;
    }
    return a;
}

int prom_concept_block_allocate(void* opaque, uint64_t bytes, uint32_t placement, prom_vk_buffer* view) {
    block_attachment* a = opaque;
    if (!bytes || bytes > SIZE_MAX || placement > 3u) return 0;
    const int index = PromBlockAllocate(&a->owner, (size_t)bytes, placement);
    if (index < 0) return 0;
    memset(view, 0, sizeof(*view));
    view->buffer = PromBlockBuffer(&a->owner, index);
    view->memory = PromBlockMemory(&a->owner, index);
    view->size = bytes;
    if (placement != 0 && vkMapMemory(PromBlockDevice(&a->owner), view->memory, 0, bytes, 0, &view->mapped) != VK_SUCCESS) {
        if (!PromBlockRelease(&a->owner, index)) abort();
        memset(view, 0, sizeof(*view)); return 0;
    }
    return 1;
}
int prom_concept_block_release(void* opaque, VkBuffer buffer) {
    block_attachment* a = opaque;
    if (buffer == VK_NULL_HANDLE) return 1;
    const int index = PromBlockFindBuffer(&a->owner, buffer);
    if (index >= 0) {
        if (!PromBlockRelease(&a->owner, index)) abort(); /* Refuse release without completion. */
        return 1;
    }
    return 0; /* A legacy prefetch allocation, owned by the legacy path. */
}
int prom_concept_block_destroy(void* opaque) {
    block_attachment* a = opaque;
    if (!a) return PROM_OK;
    if (PromBlockDrain(&a->owner) != 0 && !PromBlockDeviceLost(&a->owner)) return PROM_ERROR;
    concept_prometheus_model_block_dispose_block(a->owner); free(a); return PROM_OK;
}
int prom_concept_block_drain(prom_model_block_state* block) {
    block_attachment* a = block->concept_owner;
    if (!a) return 1;
    const uint64_t waits = PromBlockWaits(&a->owner);
    const int code = PromBlockDrain(&a->owner);
    block->vk_fence_wait_count += PromBlockWaits(&a->owner) - waits;
    return code == 0 || PromBlockDeviceLost(&a->owner);
}
static int load(prom_model_block_state* block, int slot, uint32_t kernel, uint32_t bindings, uint32_t push) {
    block_attachment* a = block->concept_owner;
    if ((block->test_flags & PROM_TESTCFG_FAIL_PIPELINE_CREATE) != 0u) return 0;
    if (!PromBlockLoad(&a->owner, a->host, slot, kernel, bindings, push)) return 0;
    block->pipeline_create_count++;
    block->vk_create_shader_module_count++; block->vk_create_compute_pipelines_count++;
    return 1;
}
int prom_concept_block_identity_pipeline(prom_model_block_state* block) { return load(block, 6, 23, 3, 8); }
int prom_concept_block_pipelines(prom_model_block_state* block) {
    static const uint32_t ids[] = {29, 24, 25, 26, 27, 28};
    static const uint32_t bindings[] = {4, 8, 5, 3, 3, 3};
    static const uint32_t push[] = {8, 8, 16, 16, 16, 16};
    for (int i = 0; i < 6; ++i) {
        if (!load(block, i, ids[i], bindings[i], push[i])) return 0;
        block->m1b_pipelines[i].shader_id = ids[i];
        block->m1b_pipelines[i].binding_count = bindings[i];
        block->m1b_pipelines[i].push_constant_bytes = push[i];
        block->m1b_pipelines[i].concept_block = block;
        block->m1b_pipelines[i].concept_role = (uint32_t)i;
    }
    return 1;
}
static concept_buffer_range range(const prom_vk_buffer* view) {
    const concept_buffer_range result = {view->buffer, view->size}; return result;
}
static concept_block_frame frame_of(const prom_model_block_state* block) {
    const prom_vk_buffer* views[] = {&block->input_upload, &block->input_bf16_device, &block->input_device,
        &block->timestep_upload, &block->timestep_bf16_device, &block->timestep_device,
        &block->resident_boundary_device, &block->adaln_projection, &block->attention_scale,
        &block->attention_gate, &block->mlp_scale, &block->mlp_gate, &block->modulated,
        &block->norm_audit, &block->qkv, &block->attention, &block->audit_readback,
        &block->output_device, &block->output_readback, &block->audit_device, &block->weight_upload};
    concept_block_frame frame;
    for (int i = 0; i < 21; ++i) frame.buffers.data[i] = range(views[i]);
    for (int i = 0; i < 13; ++i) frame.weights.data[i] = range(&block->weights[i].device);
    return frame;
}
void prom_concept_block_record_stage(prom_model_block_state* block, VkCommandBuffer commands, uint32_t role) {
    block_attachment* a = block->concept_owner;
    concept_recording borrowed = {0};
    const concept_projection_stage stage = {role - 1u};
    if (!a || role < 1u || role > 5u) abort(); /* Internal closed-portfolio invariant. */
    borrowed.device = PromBlockDevice(&a->owner);
    borrowed.commands = commands;
    borrowed.submitted = true; /* This reference can never free the caller's recording. */
    concept_prometheus_model_block_record_projection_stage(&borrowed, &a->owner, frame_of(block), stage);
}
int prom_concept_block_run(prom_reduction_runtime_state* state, prom_model_block_state* block,
    uint32_t mode, uint32_t elements, uint32_t stage, uint32_t resident, int32_t* detail) {
    block_attachment* a = block->concept_owner;
    const concept_block_frame frame = frame_of(block);
    uint64_t begin = prom_reduction_now_ns();
    const uint64_t recordings = PromBlockRecordings(&a->owner), submissions = PromBlockSubmissions(&a->owner), waits = PromBlockWaits(&a->owner);
    if (mode > CONCEPT_BLOCK_PROGRAM_INGRESS) { *detail = PROM_MODEL_BLOCK_DETAIL_INVALID_REQUEST; return 0; }
    uint32_t flags = 0;
    if ((state->reduction_test_flags & PROM_REDUCTION_TESTCFG_FAIL_COMMAND_RECORD) != 0u) flags |= 1u;
    if ((block->test_flags & PROM_TESTCFG_FAIL_QUEUE_SUBMIT) != 0u ||
        (state->reduction_test_flags & PROM_REDUCTION_TESTCFG_FAIL_QUEUE_SUBMIT) != 0u) flags |= 2u;
    if ((block->test_flags & PROM_TESTCFG_SKIP_SUBMIT_WAIT) != 0u ||
        (state->reduction_test_flags & PROM_REDUCTION_TESTCFG_FAIL_COMPLETION_OBSERVATION) != 0u) flags |= 4u;
    if ((block->test_flags & PROM_TESTCFG_FAIL_DISPATCH) != 0u) flags |= 8u;
    if ((block->test_flags & PROM_TESTCFG_FAIL_COMMAND_END) != 0u) flags |= 16u;
    PromBlockSetFlags(&a->owner, flags);
    *detail = PromBlockRun(&a->owner, &frame, mode, elements, stage, resident);
    flags = PromBlockFlags(&a->owner);
    /* A combined injection survives until its actual phase is reached. */
    if ((flags & 1u) == 0u) (void)prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_COMMAND_RECORD);
    if ((flags & 2u) == 0u) (void)prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_QUEUE_SUBMIT);
    if ((flags & 4u) == 0u) (void)prom_reduction_take_test_flag(state, PROM_REDUCTION_TESTCFG_FAIL_COMPLETION_OBSERVATION);
    block->vk_allocate_command_buffers_count += PromBlockRecordings(&a->owner) - recordings;
    block->vk_queue_submit_count += PromBlockSubmissions(&a->owner) - submissions;
    block->vk_fence_wait_count += PromBlockWaits(&a->owner) - waits;
    block->quarantined = PromBlockQuarantined(&a->owner);
    if (*detail == 0) {
        if (mode == 1) {
            block->gpu_compute_ns = 0;
            for (int i = 0; i < 6; ++i) { block->m1b_boundary_gpu_ns[i] = PromBlockTiming(&a->owner, i); block->gpu_compute_ns += block->m1b_boundary_gpu_ns[i]; }
        }
        block->last_execution_ns = prom_reduction_elapsed_ns(begin, prom_reduction_now_ns());
    }
    return *detail == 0;
}
