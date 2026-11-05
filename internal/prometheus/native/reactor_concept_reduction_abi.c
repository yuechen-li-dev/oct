/* Mechanical C11 facade over Concept-owned state. No scheduling or lifetime
   policy lives here. The generated module decides admission, reuse and drain. */
#include "reactor_concept_reduction.h"
#include "generated/prometheusexecution.generated.h"
#include <stdlib.h>

static concept_reduction_shape shape_of(const PrometheusReductionRequest* r) {
    const concept_reduction_shape s = {r->row_count, r->elements_per_row, r->operation,
        r->finalization, r->flags, r->input_element_count, r->output_element_count};
    return s;
}
static int outcome_of(concept_execution_outcome value, PrometheusReductionExecutionResult* out) {
    out->detail_code = value.detail; out->stage = value.stage;
    out->physical_slot_id = value.slot; out->physical_slot_generation = value.generation;
    out->physical_slot_recyclable = value.recyclable; out->gpu_timestamp_valid = value.timed;
    out->logical_request_id = value.request; out->gpu_duration_ns = value.gpuNanoseconds;
    out->first_nonfinite_index = value.firstNonfinite;
    return value.detail == 0 ? PROM_OK : PROM_ERROR;
}
void* prom_concept_reduction_create(void* host, const prom_vk_runtime_services* s, int32_t* detail) {
    concept_reduction_executor* storage = malloc(sizeof(*storage));
    concept_device_borrow device = {s->physical_device, s->device, s->compute_queue,
        s->compute_queue_family_index, s->concept_enabled_features};
    concept_result_reduction_executor_vulkan_error created;
    if (!storage) { *detail = -1; return NULL; }
    created = concept_prometheus_execution_create_executor((concept_prom_reduction_host)host,
        device, s->reduction_ring_depth, s->reduction_test_flags, s->timestamp_query_supported != 0u);
    if (created.tag != 0) {
        *detail = concept_prometheus_execution_code_from_error(created.payload.error.error);
        free(storage); return NULL;
    }
    *storage = created.payload.ok.value;
    *detail = 0;
    return storage;
}
int prom_concept_reduction_destroy(void* opaque) {
    concept_reduction_executor* executor = opaque;
    if (!executor) return PROM_OK;
    if (concept_prometheus_execution_drain_executor(executor) != 0) return PROM_ERROR;
    concept_prometheus_execution_dispose_executor(*executor);
    free(executor);
    return PROM_OK;
}
int prom_concept_reduction_validate(const PrometheusReductionRequest* r, PrometheusReductionExecutionResult* out) {
    const concept_readonly_span_float input = {r->input, (size_t)r->input_element_count};
    const concept_span_float output = {r->output, (size_t)r->output_element_count};
    return outcome_of(concept_prometheus_execution_validate_input(shape_of(r), input, output), out);
}
int prom_concept_reduction_execute(void* executor, const PrometheusReductionRequest* r, PrometheusReductionExecutionResult* out) {
    const concept_readonly_span_float input = {r->input, (size_t)r->input_element_count};
    const concept_span_float output = {r->output, (size_t)r->output_element_count};
    return outcome_of(concept_prometheus_execution_execute(executor, shape_of(r), input, output), out);
}
void prom_concept_reduction_stats(void* opaque, PrometheusReductionDiagnostics* out) {
    const concept_reduction_executor* e = opaque;
    out->initialized = 1u; out->production_enabled = 1u;
    out->configured_ring_depth = e->depth; out->physical_slot_count = e->depth;
    out->next_logical_request_id = e->nextRequest; out->total_requests = e->requests;
    out->successful_requests = e->successes; out->logical_failure_count = e->failures;
    out->physical_recycle_count = e->recycles; out->quarantine_count = e->quarantines;
    out->reap_count = e->reaped; out->pipeline_create_count = 7u;
    out->descriptor_update_count = e->descriptorUpdates;
    out->command_record_count = e->recordings; out->queue_submit_count = e->submissions;
    out->buffer_allocation_count = e->allocations; out->buffer_reuse_count = e->reuses;
    out->temporary_capacity_bytes = e->temporaryCapacity; out->last_detail_code = e->lastDetail;
    out->outstanding_slots = concept_prometheus_execution_outstanding_slots(e, false);
    out->quarantined_slots = concept_prometheus_execution_outstanding_slots(e, true);
}
