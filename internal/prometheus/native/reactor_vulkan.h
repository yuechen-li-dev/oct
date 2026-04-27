#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_VULKAN_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_VULKAN_H

#include "reactor_api.h"

#ifdef __cplusplus
extern "C" {
#endif

int prom_reactor_runtime_create_impl(void* config, void** out_handle);
int prom_reactor_runtime_destroy_impl(void* handle);
int prom_reactor_runtime_probe_impl(void* handle, PrometheusCaps* out_caps);
int prom_reactor_runtime_sgemm_impl(void* handle,
                                    const float* a,
                                    const float* b,
                                    float* c,
                                    uint32_t m,
                                    uint32_t n,
                                    uint32_t k,
                                    uint32_t* out_stage,
                                    int* out_detail_code);
int prom_reactor_runtime_sgemm_batch_impl(void* handle,
                                          const PrometheusSgemmBatchEntry* entries,
                                          uint32_t entry_count,
                                          uint32_t flags,
                                          uint32_t* out_stage,
                                          int* out_detail_code);
int prom_reactor_runtime_sgemm_submit_async_impl(void* handle,
                                                 const float* a,
                                                 const float* b,
                                                 uint32_t m,
                                                 uint32_t n,
                                                 uint32_t k,
                                                 int* out_task_id,
                                                 uint32_t* out_stage,
                                                 int* out_detail_code);
int prom_reactor_runtime_sgemm_query_async_impl(void* handle, int task_id, PrometheusAsyncStatus* out_status);
int prom_reactor_runtime_sgemm_consume_async_impl(void* handle,
                                                  int task_id,
                                                  float* c,
                                                  uint32_t c_len,
                                                  uint32_t* out_stage,
                                                  int* out_detail_code);
int prom_reactor_runtime_sgemm_abandon_async_impl(void* handle, int task_id);
int prom_reactor_runtime_sgemm_policy_diagnostics_impl(void* handle, PrometheusSgemmPolicyDiagnostics* out_diag);
int prom_reactor_runtime_sgemm_batch_diagnostics_impl(void* handle, PrometheusSgemmBatchDiagnostics* out_diag);

#ifdef __cplusplus
}
#endif

#endif
