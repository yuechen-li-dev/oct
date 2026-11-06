/* C11 transport only. No Vulkan allocation, recording or submission authority. */
#define PROM_SGEMM_LAYOUT_TRANSPORT
#include "reactor_concept_sgemm.h"
#include "generated/prometheussgemm.generated.h"
#include <stdlib.h>
#include <string.h>

_Static_assert(sizeof(prom_sgemm_job) == sizeof(concept_sgemm_job), "SGEMM job ABI size");
_Static_assert(_Alignof(prom_sgemm_job) == _Alignof(concept_sgemm_job), "SGEMM job ABI alignment");
#define FIELD(name) _Static_assert(offsetof(prom_sgemm_job, name) == offsetof(concept_sgemm_job, name), "SGEMM " #name)
FIELD(buffers); FIELD(shader); FIELD(m); FIELD(n); FIELD(k); FIELD(x); FIELD(y); FIELD(z);
FIELD(dispatches); FIELD(path); FIELD(dedicated); FIELD(computeFamily); FIELD(transferFamily); FIELD(uploadMask); FIELD(readback); FIELD(perturbWord);
#undef FIELD
_Static_assert(sizeof(prom_sgemm_shader) == sizeof(concept_sgemm_shader), "SGEMM shader ABI");
#define FIELD(name) _Static_assert(offsetof(prom_sgemm_shader, name) == offsetof(concept_sgemm_shader, name), "SGEMM shader " #name)
FIELD(pipeline); FIELD(layout); FIELD(descriptors); FIELD(queries); FIELD(queryBase); FIELD(timed);
#undef FIELD
void* prom_concept_sgemm_create(const prom_vk_runtime_services* s) {
    concept_sgemm_executor* owner = malloc(sizeof(*owner));
    if (!owner) return NULL;
    concept_device_borrow compute = {s->physical_device, s->device, s->compute_queue,
        s->compute_queue_family_index, s->concept_enabled_features};
    concept_device_borrow transfer = compute;
    if (s->transfer_queue_available) {
        transfer.queue = s->transfer_queue; transfer.queueFamily = s->transfer_queue_family_index;
    }
    concept_result_sgemm_executor_vulkan_error result = concept_prometheus_sgemm_create_sgemm(compute, transfer);
    if (result.tag != 0) { free(owner); return NULL; }
    *owner = result.payload.ok.value;
    return owner;
}
int prom_concept_sgemm_destroy(void* owner) {
    if (!owner) return 0;
    concept_sgemm_executor* executor = owner;
    int code = concept_prometheus_sgemm_sgemm_code(concept_prometheus_sgemm_drain_sgemm(executor));
    if (code != 0 && code != -4) return code;
    concept_prometheus_sgemm_dispose_sgemm(*executor);
    free(executor);
    return 0;
}
int prom_concept_sgemm_record(void* owner, uint32_t slot, const prom_sgemm_job* input) {
    if (!owner || slot >= 5 || !input) return -3;
    concept_sgemm_job job;
    memcpy(&job, input, sizeof(job));
    return concept_prometheus_sgemm_sgemm_code(concept_prometheus_sgemm_record_sgemm(owner, (int)slot, job));
}
int prom_concept_sgemm_submit(void* owner, uint32_t slot) {
    if (!owner || slot >= 5) return -3;
    return concept_prometheus_sgemm_sgemm_code(concept_prometheus_sgemm_submit_sgemm(owner, (int)slot));
}
int prom_concept_sgemm_wait(void* owner, uint32_t slot) {
    if (!owner || slot >= 5) return -3;
    return concept_prometheus_sgemm_sgemm_code(concept_prometheus_sgemm_wait_sgemm(owner, (int)slot));
}
int prom_concept_sgemm_poll(void* owner, uint32_t slot) {
    if (!owner || slot >= 5) return -3;
    return concept_prometheus_sgemm_poll_sgemm(owner, (int)slot);
}
int prom_concept_sgemm_timings(void* owner, uint32_t slot, uint64_t* record, uint64_t* submit) {
    if (!owner || slot >= 5 || !record || !submit) return -3;
    const concept_sgemm_executor* executor = owner;
    const concept_option_owned_sgemm_work* entry = &executor->slots.data[slot];
    if (entry->tag != 0) return -3;
    *record = entry->payload.some.value.recordNanoseconds;
    *submit = entry->payload.some.value.submitNanoseconds;
    return 0;
}
