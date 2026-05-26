#include "reactor_api.h"

#include "reactor_vulkan.h"

#define PROMETHEUS_REACTOR_ABI_V1 1u

uint32_t prometheus_reactor_abi_version(void) {
  return PROMETHEUS_REACTOR_ABI_V1;
}

int prometheus_reactor_runtime_create(void* config, void** out_handle) {
  return prom_reactor_runtime_create_impl(config, out_handle);
}

int prometheus_reactor_runtime_destroy(void* handle) {
  return prom_reactor_runtime_destroy_impl(handle);
}

int prometheus_reactor_runtime_probe(void* handle, PrometheusCaps* out_caps) {
  return prom_reactor_runtime_probe_impl(handle, out_caps);
}

int prometheus_reactor_runtime_sgemm(void* handle,
                                     const float* a,
                                     const float* b,
                                     float* c,
                                     uint32_t m,
                                     uint32_t n,
                                     uint32_t k,
                                     uint32_t* out_stage,
                                     int* out_detail_code) {
  return prom_reactor_runtime_sgemm_impl(handle, a, b, c, m, n, k, out_stage, out_detail_code);
}

int prometheus_reactor_runtime_sgemm_benchmark_variant(void* handle,
                                                       const float* a,
                                                       const float* b,
                                                       float* c,
                                                       uint32_t m,
                                                       uint32_t n,
                                                       uint32_t k,
                                                       uint32_t requested_variant,
                                                       uint32_t* out_stage,
                                                       int* out_detail_code) {
  return prom_reactor_runtime_sgemm_benchmark_variant_impl(
      handle, a, b, c, m, n, k, requested_variant, out_stage, out_detail_code);
}

int prometheus_reactor_runtime_sgemm_batch(void* handle,
                                           const PrometheusSgemmBatchEntry* entries,
                                           uint32_t entry_count,
                                           uint32_t flags,
                                           uint32_t* out_stage,
                                           int* out_detail_code) {
  return prom_reactor_runtime_sgemm_batch_impl(handle, entries, entry_count, flags, out_stage, out_detail_code);
}

int prometheus_reactor_runtime_sgemm_submit_async(void* handle,
                                                  const float* a,
                                                  const float* b,
                                                  uint32_t m,
                                                  uint32_t n,
                                                  uint32_t k,
                                                  int* out_task_id,
                                                  uint32_t* out_stage,
                                                  int* out_detail_code) {
  return prom_reactor_runtime_sgemm_submit_async_impl(handle, a, b, m, n, k, out_task_id, out_stage, out_detail_code);
}

int prometheus_reactor_runtime_sgemm_query_async(void* handle, int task_id, PrometheusAsyncStatus* out_status) {
  return prom_reactor_runtime_sgemm_query_async_impl(handle, task_id, out_status);
}

int prometheus_reactor_runtime_sgemm_consume_async(void* handle,
                                                   int task_id,
                                                   float* c,
                                                   uint32_t c_len,
                                                   uint32_t* out_stage,
                                                   int* out_detail_code) {
  return prom_reactor_runtime_sgemm_consume_async_impl(handle, task_id, c, c_len, out_stage, out_detail_code);
}

int prometheus_reactor_runtime_sgemm_abandon_async(void* handle, int task_id) {
  return prom_reactor_runtime_sgemm_abandon_async_impl(handle, task_id);
}

int prometheus_reactor_runtime_sgemm_policy_diagnostics(void* handle, PrometheusSgemmPolicyDiagnostics* out_diag) {
  return prom_reactor_runtime_sgemm_policy_diagnostics_impl(handle, out_diag);
}

int prometheus_reactor_runtime_sgemm_policy_diagnostics_sized(void* handle,
                                                              PrometheusSgemmPolicyDiagnostics* out_diag,
                                                              uint32_t out_size) {
  return prom_reactor_runtime_sgemm_policy_diagnostics_sized_impl(handle, out_diag, out_size);
}


int prometheus_reactor_runtime_p15_test_seed_matured_reservation(void* handle,
                                                                 uint32_t shape_class,
                                                                 uint32_t variant_id,
                                                                 uint64_t target_tick) {
  return prom_reactor_runtime_p15_test_seed_matured_reservation_impl(handle, shape_class, variant_id, target_tick);
}

int prometheus_reactor_runtime_sgemm_batch_diagnostics(void* handle, PrometheusSgemmBatchDiagnostics* out_diag) {
  return prom_reactor_runtime_sgemm_batch_diagnostics_impl(handle, out_diag);
}


int prometheus_reactor_runtime_fft(void* handle,
                                   const PrometheusFftRequest* request,
                                   uint32_t* out_stage,
                                   int* out_detail_code) {
  return prom_reactor_runtime_fft_impl(handle, request, out_stage, out_detail_code);
}

int prometheus_reactor_runtime_fft_benchmark_variant(void* handle,
                                                     const PrometheusFftRequest* request,
                                                     uint32_t requested_variant,
                                                     uint32_t* out_stage,
                                                     int* out_detail_code) {
  return prom_reactor_runtime_fft_benchmark_variant_impl(handle, request, requested_variant, out_stage, out_detail_code);
}

int prometheus_reactor_runtime_fft_diagnostics(void* handle, PrometheusFftDiagnostics* out_diag) {
  return prom_reactor_runtime_fft_diagnostics_impl(handle, out_diag);
}

int prometheus_reactor_runtime_fft_diagnostics_sized(void* handle, PrometheusFftDiagnostics* out_diag, uint32_t out_size) {
  return prom_reactor_runtime_fft_diagnostics_sized_impl(handle, out_diag, out_size);
}

int prometheus_runtime_create(void* config, void** out_handle) {
  return prometheus_reactor_runtime_create(config, out_handle);
}

int prometheus_runtime_destroy(void* handle) {
  return prometheus_reactor_runtime_destroy(handle);
}

int prometheus_runtime_probe(void* handle, PrometheusCaps* out_caps) {
  return prometheus_reactor_runtime_probe(handle, out_caps);
}
