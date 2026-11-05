/* C11 transport only: Concept owns storage, generation publication, dispatch,
   pending submissions and drain. The existing pure planner owns plan policy. */
#include "reactor_concept_rmsnorm.h"
#include <stddef.h>
#include "generated/prometheusrmsnorm.generated.h"
#include <stdlib.h>
#include <string.h>

_Static_assert(sizeof(concept_norm_reduce_push) == 32, "RMSNorm push size");
_Static_assert(offsetof(concept_norm_reduce_push, epsilon) == 28, "RMSNorm epsilon offset");

static uint64_t hash_u64(uint64_t hash, uint64_t value) {
    for (unsigned i = 0; i < 8; ++i) { hash ^= (uint8_t)(value >> (i * 8)); hash *= 1099511628211ull; }
    return hash;
}
static int code(concept_norm_executor* e, concept_result_void_vulkan_error result) {
    return concept_prometheus_rms_norm_norm_code(e, result);
}
static uint32_t slot_index(uint32_t heads) { return heads == 8u ? 1u : heads == 1u ? 2u : 0u; }
static concept_norm_slot* slot_at(concept_norm_executor* e, uint32_t index) {
    return &e->slots.data[index].payload.some.value;
}
static prom_device_buffer_view view_of(const concept_norm_executor* e,
    const concept_norm_slot* slot, VkBuffer buffer, uint64_t bytes,
    uint32_t rows, uint32_t width, uint32_t stride, uint64_t generation, uint32_t index) {
    prom_device_buffer_view view = {0};
    view.buffer = buffer; view.byte_length = bytes; view.element_type = PROM_DEVICE_ELEMENT_F32;
    view.logical_rows = rows; view.logical_columns = width; view.row_stride_elements = stride;
    view.layout = PROM_DEVICE_LAYOUT_ROW_MAJOR; view.producer_access = PROM_DEVICE_ACCESS_COMPUTE_WRITE;
    view.required_consumer_access = PROM_DEVICE_ACCESS_COMPUTE_READ;
    view.owning_device = e->context.view.device; view.owning_lifetime_id = generation;
    /* Separate physical allocator from the legacy model ring. */
    view.owning_slot_id = 0x40000000u + index; view.owning_slot_generation = slot->generation;
    return view;
}

void* prom_concept_norm_create(void* host, const prom_vk_runtime_services* s, int32_t* detail) {
    concept_norm_executor* storage = malloc(sizeof(*storage));
    const concept_device_borrow device = {s->physical_device, s->device, s->compute_queue,
        s->compute_queue_family_index, s->concept_enabled_features};
    concept_result_norm_executor_vulkan_error created;
    if (!storage) { *detail = -1; return NULL; }
    created = concept_prometheus_rms_norm_create_norm_executor(host, device, s->reduction_test_flags,
        s->timestamp_query_supported != 0u);
    if (created.tag != 0) {
        *detail = concept_prometheus_rms_norm_code_from_error(created.payload.error.error);
        free(storage); return NULL;
    }
    *storage = created.payload.ok.value; *detail = 0; return storage;
}
int prom_concept_norm_destroy(void* opaque) {
    concept_norm_executor* e = opaque;
    if (!e) return PROM_OK;
    if (code(e, concept_prometheus_rms_norm_drain_norm(e)) != 0 && !e->deviceLost) return PROM_ERROR;
    /* Terminal device loss also establishes that submitted work cannot use resources. */
    concept_prometheus_rms_norm_dispose_norm(*e); free(e); return PROM_OK;
}
int prom_concept_norm_weight(void* opaque, const prom_m46_weight_prepare_request* request,
    prom_m46_weight_prepare_result* out) {
    concept_norm_executor* e = opaque;
    const concept_readonly_span_float values = {request->values, (size_t)request->element_count};
    out->observed_generation = e->weightGeneration; out->requested_generation = request->generation;
    const int detail = code(e, concept_prometheus_rms_norm_replace_weight(e, values, request->generation, out->hash));
    if (detail) { out->detail_code = detail == -7406 ? detail : PROM_M46_DETAIL_RESOURCE; out->stage = PROM_STAGE_TRANSFER_IN; return PROM_ERROR; }
    out->generation = e->weightGeneration; out->replaced = out->observed_generation != 0;
    out->retained_bytes = (uint64_t)e->weight.count * sizeof(float);
    return PROM_OK;
}

int prom_concept_norm_execute(void* opaque, const prom_m49a_m46_request* r, prom_m49a_m46_result* out) {
    concept_norm_executor* e = opaque;
    prom_m46_composed_result* result = &out->rmsnorm;
    const uint32_t index = slot_index(r->direct_rope ? r->rope_heads : 0u);
    concept_norm_job job = {r->tokens, r->model_width, r->z_row_stride,
        r->strategy == PROM_M46_STRATEGY_IN_PLACE_Z ? r->z_row_stride : r->model_width,
        1u, 0u, r->strategy == PROM_M46_STRATEGY_IN_PLACE_Z,
        r->bf16_roundtrip_input, r->bf16_roundtrip_output, r->direct_rope ? r->rope_heads : 0u, r->epsilon};
    if (e->weightGeneration != r->required_weight_generation || e->weightHash != r->required_weight_hash || e->width != r->model_width) {
        out->detail_code = PROM_M46_DETAIL_STALE_WEIGHT_GENERATION; return PROM_ERROR;
    }
    if (code(e, concept_prometheus_rms_norm_prepare_norm(e, index, job)) != 0) {
        out->detail_code = PROM_M46_DETAIL_RESOURCE; return PROM_ERROR;
    }
    concept_norm_slot* slot = slot_at(e, index);
    const VkBuffer input = r->bf16_roundtrip_input ? slot->rounded.buffer : slot->input.buffer;
    prom_m46_plan_request plan_request = {0};
    plan_request.z_view = view_of(e, slot, input, r->matched_storage_element_count * sizeof(float),
        r->tokens, r->model_width, r->z_row_stride, r->input_generation, index);
    plan_request.tokens = r->tokens; plan_request.model_width = r->model_width;
    plan_request.epsilon = r->epsilon; plan_request.strategy = r->strategy;
    plan_request.submit_policy = PROM_M46_SUBMIT_ONE_COMMAND_BUFFER; plan_request.z_exclusive = 1u;
    plan_request.final_readback = r->direct_rope == 0u;
    plan_request.requested_reduction_plan = r->requested_reduction_plan;
    plan_request.expected_z_generation = r->input_generation;
    plan_request.weight_generation = e->weightGeneration; plan_request.weight_hash = e->weightHash;
    plan_request.m45_replay_id = hash_u64(r->exact_source_hash, out->input_hash);
    if (prom_m46_rmsnorm_plan_build(&plan_request, &result->rmsnorm_plan) != PROM_OK || !result->rmsnorm_plan.eligibility_eligible) {
        out->detail_code = PROM_M46_DETAIL_INVALID_VIEW; return PROM_ERROR;
    }
    const prom_rmsnorm_plan* plan = &result->rmsnorm_plan;
    job.partials = plan->partials_per_row; job.staged = plan->reduction_plan == PROM_M46_REDUCTION_STAGED;
    job.outputStride = plan->n_row_stride;
    const concept_readonly_span_float input_values = {r->matched_z, (size_t)r->matched_storage_element_count};
    const concept_readonly_span_float cosine = {r->rope_cosine, r->direct_rope ? (size_t)r->rope_cosine_element_count : 0u};
    const concept_readonly_span_float sine = {r->rope_sine, r->direct_rope ? (size_t)r->rope_sine_element_count : 0u};
    const concept_span_float output = {r->direct_rope ? r->rope_output : r->output,
        (size_t)(r->direct_rope ? r->rope_output_element_count : r->output_element_count)};
    const concept_span_float inverse = {r->inv_rms_output, r->direct_rope ? 0u : (size_t)r->inv_rms_output_element_count};
    const concept_result_norm_timing_vulkan_error executed = concept_prometheus_rms_norm_execute_norm(e, index, job,
        input_values, cosine, sine, output, inverse);
    if (executed.tag != 0) {
        const int error = concept_prometheus_rms_norm_observe_norm_error(e, executed.payload.error.error);
        out->detail_code = error == -7414 || (slot->pending.tag == 0 && !slot->pending.payload.some.value.waited)
            ? PROM_M46_DETAIL_COMPLETION_UNCERTAIN : PROM_M46_DETAIL_COMMAND;
        return PROM_ERROR;
    }
    const concept_norm_timing timing = executed.payload.ok.value;
    result->reduction_gpu_ns = timing.partial;
    result->final_reduction_gpu_ns = job.staged ? timing.finalReduction : 0u;
    result->inv_rms_gpu_ns = timing.partial + timing.finalReduction;
    result->apply_gpu_ns = timing.apply; result->m46_gpu_ns = timing.total;
    result->total_m43_m44_m45_m46_gpu_ns = timing.total;
    result->logical_request_id = e->nextRequest - 1u;
    result->physical_slot_id = 0x40000000u + index; result->physical_slot_generation = slot->generation;
    result->physical_slot_recyclable = 1u; result->submit_count = 1u;
    result->final_readback_count = output.length != 0u; result->no_intermediate_host_copy = 1u;
    result->exact_request_bytes = plan->memory.exact_request_bytes;
    result->retained_bytes = concept_prometheus_rms_norm_retained_norm(e);
    result->buffer_allocation_count = e->allocations; result->buffer_reuse_count = e->reuses;
    result->descriptor_update_count = e->descriptors; result->pipeline_create_count = 4u;
    result->z_generation = plan->z_generation; result->weight_generation = e->weightGeneration; result->n_generation = plan->n_generation;
    const VkBuffer normalized = job.inPlace ? input : slot->output.buffer;
    result->n_view = view_of(e, slot, normalized, (uint64_t)job.rows * job.outputStride * sizeof(float),
        job.rows, job.width, job.outputStride, plan->n_generation, index);
    out->matched_input = 1u; out->input_generation = r->input_generation;
    out->weight_generation = e->weightGeneration; out->weight_hash = e->weightHash;
    out->replay_identity = hash_u64(hash_u64(plan->replay_id, out->input_hash), r->exact_source_hash);
    if (r->direct_rope) {
        out->resident_rope_source_bound = 1u; out->normalized_readback_count = 0u;
        out->rope_source_byte_range = (uint64_t)job.rows * job.width * sizeof(float);
        out->rope_destination_byte_range = out->rope_source_byte_range;
        out->rope_descriptor_update_count = e->ropeDescriptors; out->rope_pipeline_create_count = 1u;
    }
    return PROM_OK;
}
int prom_concept_norm_resident(void* opaque, uint32_t heads, prom_device_buffer_view* view) {
    concept_norm_executor* e = opaque;
    const uint32_t index = slot_index(heads);
    if (!view || index == 0 || !concept_prometheus_rms_norm_has_slot(&e->slots.data[index])) return PROM_ERROR;
    const concept_norm_slot* slot = slot_at(e, index);
    if (!slot->resident) return PROM_ERROR;
    *view = view_of(e, slot, slot->rope.buffer, (uint64_t)15 * heads * 256 * sizeof(float),
        15 * heads, 256, 256, slot->generation, index);
    return PROM_OK;
}
void prom_concept_norm_consume(void* opaque) {
    concept_prometheus_rms_norm_consume_resident(opaque);
}
