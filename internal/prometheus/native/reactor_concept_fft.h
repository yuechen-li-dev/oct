#ifndef OCT_REACTOR_CONCEPT_FFT_H
#define OCT_REACTOR_CONCEPT_FFT_H
#include "reactor_vulkan.h"
#ifdef __cplusplus
extern "C" {
#endif
void* prom_reactor_runtime_concept_fft_state(void* host);
int prom_reactor_runtime_set_concept_fft_state(void* host, void* state);
int prom_reactor_runtime_concept_fft_detach(void* host);
int prom_reactor_runtime_concept_fft_stats(void* host, uint64_t* allocations, uint64_t* reuses);
void prom_concept_fft_stats(void* owner, uint64_t* allocations, uint64_t* reuses);
void* prom_concept_fft_create(void* host, const prom_vk_runtime_services* services);
int prom_concept_fft_destroy(void* owner);
int prom_concept_fft_run(void* owner, const void* input, void* output, const void* twiddles,
    uint64_t bytes, uint32_t count, uint32_t batches, uint32_t stride, uint32_t inverse,
    uint32_t normalize, uint32_t fp64, uint32_t flags);
int prom_concept_fft_execute(void* host, const PrometheusFftRequest* request,
    uint64_t bytes, uint32_t* stage, int* detail);
/* Contiguous, forward, unnormalized binary64 complex pairs. No fallback.
   The caller supplies count live input/output pairs; input is read-only. */
int prom_reactor_runtime_fft64_impl(void* host, const double* input, double* output,
    uint32_t count, uint32_t* stage, int* detail);
#ifdef __cplusplus
}
#endif
#endif
