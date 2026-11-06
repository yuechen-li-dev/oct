/* C11 layout transport. Concept owns every GPU resource and submission. */
#include "reactor_concept_fft.h"
#include "generated/prometheusfft.generated.h"
#include <stdlib.h>

_Static_assert(sizeof(concept_fft_job) == 32, "FFT job ABI");
void prom_concept_fft_stats(void* owner, uint64_t* allocations, uint64_t* reuses) {
    const concept_fft_executor* executor = owner;
    *allocations = executor->allocations; *reuses = executor->reuses;
}
void* prom_concept_fft_create(void* host, const prom_vk_runtime_services* s) {
    concept_fft_executor* storage = malloc(sizeof(*storage));
    if (!storage) return NULL;
    const concept_device_borrow device = {s->physical_device, s->device, s->compute_queue,
        s->compute_queue_family_index, s->concept_enabled_features};
    concept_result_fft_executor_vulkan_error created = concept_prometheus_fft_create_fft(host, device);
    if (created.tag != 0) { free(storage); return NULL; }
    *storage = created.payload.ok.value;
    return storage;
}
int prom_concept_fft_destroy(void* owner) {
    concept_fft_executor* e = owner;
    if (!e) return PROM_OK;
    if (concept_prometheus_fft_fft_code(e, concept_prometheus_fft_drain_fft(e)) != 0 && !e->deviceLost)
        return PROM_ERROR;
    concept_prometheus_fft_dispose_fft(*e);
    free(e);
    return PROM_OK;
}
int prom_concept_fft_run(void* owner, const void* input, void* output, const void* twiddles,
    uint64_t bytes, uint32_t count, uint32_t batches, uint32_t stride, uint32_t inverse,
    uint32_t normalize, uint32_t fp64, uint32_t flags) {
    concept_fft_executor* e = owner;
    uint32_t bits = 0;
    for (uint32_t n = count; n > 1; n >>= 1) ++bits;
    const concept_fft_job job = {count, batches, stride, inverse, normalize, fp64, bits, flags};
    const concept_readonly_span_byte source = {input, (size_t)bytes};
    const concept_readonly_span_byte table = {twiddles, fp64 ? (size_t)count * sizeof(double) : 0};
    const concept_span_byte destination = {output, (size_t)bytes};
    return concept_prometheus_fft_fft_code(e, concept_prometheus_fft_run_fft(e, job, source, table, destination));
}
