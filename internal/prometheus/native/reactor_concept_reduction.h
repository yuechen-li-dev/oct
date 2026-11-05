#ifndef OCT_PROMETHEUS_CONCEPT_REDUCTION_H
#define OCT_PROMETHEUS_CONCEPT_REDUCTION_H
#include "reactor_vulkan.h"
#ifdef __cplusplus
extern "C" {
#endif
/* Opaque Concept ownership crosses this facade; C++ never sees generated
   private class layout or imported C11 atomic representations. */
void* prom_concept_reduction_create(void* host, const prom_vk_runtime_services* device, int32_t* detail);
int prom_concept_reduction_destroy(void* executor);
int prom_concept_reduction_validate(const PrometheusReductionRequest* request, PrometheusReductionExecutionResult* out);
int prom_concept_reduction_execute(void* executor, const PrometheusReductionRequest* request, PrometheusReductionExecutionResult* out);
void prom_concept_reduction_stats(void* executor, PrometheusReductionDiagnostics* out);
#ifdef __cplusplus
}
#endif
#endif
