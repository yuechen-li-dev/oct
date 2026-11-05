#ifndef OCT_PROMETHEUS_CONCEPT_RMSNORM_H
#define OCT_PROMETHEUS_CONCEPT_RMSNORM_H
#include "reactor_vulkan.h"
#ifdef __cplusplus
extern "C" {
#endif
/* Generated private class layouts stay inside the C11 transport facade. */
void* prom_concept_norm_create(void* host, const prom_vk_runtime_services* services, int32_t* detail);
int prom_concept_norm_destroy(void* executor);
int prom_concept_norm_weight(void* executor, const prom_m46_weight_prepare_request* request, prom_m46_weight_prepare_result* out);
int prom_concept_norm_execute(void* executor, const prom_m49a_m46_request* request, prom_m49a_m46_result* out);
int prom_concept_norm_resident(void* executor, uint32_t heads, prom_device_buffer_view* view);
void prom_concept_norm_consume(void* executor);
int prom_reactor_runtime_concept_norm_detach(void* host);
int prom_reactor_runtime_concept_norm_resident(void* host, uint32_t heads, prom_device_buffer_view* view);
void prom_reactor_runtime_concept_norm_consume(void* host);
void* prom_reactor_runtime_concept_norm_state(void* host);
int prom_reactor_runtime_set_concept_norm_state(void* host, void* state);
#ifdef __cplusplus
}
#endif
#endif
