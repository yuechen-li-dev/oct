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

int prometheus_reactor_runtime_vulkan_device_diagnostics(void* handle, PrometheusVulkanDeviceDiagnostics* out_diag) {
  return prom_reactor_runtime_vulkan_device_diagnostics_impl(handle, out_diag);
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

int prometheus_reactor_runtime_sgemm_resident_benchmark(
    void* handle,
    const PrometheusSgemmResidentBenchmarkRequest* request,
    PrometheusSgemmResidentBenchmarkResult* out_result) {
  return prom_reactor_runtime_sgemm_resident_benchmark_impl(handle, request, out_result);
}

int prometheus_reactor_runtime_sgemm_batch(void* handle,
                                           const PrometheusSgemmBatchEntry* entries,
                                           uint32_t entry_count,
                                           uint32_t flags,
                                           uint32_t* out_stage,
                                           int* out_detail_code) {
  return prom_reactor_runtime_sgemm_batch_impl(handle, entries, entry_count, flags, out_stage, out_detail_code);
}

int prometheus_reactor_runtime_sgemm_batch_m31_test(void* handle,
                                                     const PrometheusSgemmBatchEntry* entries,
                                                     uint32_t entry_count,
                                                     uint32_t flags,
                                                     uint32_t* out_stage,
                                                     int* out_detail_code) {
  return prom_reactor_runtime_sgemm_batch_m31_test_impl(handle, entries, entry_count, flags, out_stage, out_detail_code);
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

int prometheus_reactor_runtime_sgemm_async_diagnostics(void* handle, PrometheusSgemmAsyncDiagnostics* out_diag) {
  return prom_reactor_runtime_sgemm_async_diagnostics_impl(handle, out_diag);
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

int prometheus_reactor_reduction_plan(const PrometheusReductionRequest* request,
                                      PrometheusReductionPlan* out_plan) {
  return prom_reactor_reduction_plan_impl(request, out_plan);
}

int prometheus_reactor_runtime_reduction(void* handle,
                                         const PrometheusReductionRequest* request,
                                         PrometheusReductionExecutionResult* out_result) {
  return prom_reactor_runtime_reduction_impl(handle, request, out_result);
}

int prometheus_reactor_runtime_reduction_diagnostics(void* handle,
                                                     PrometheusReductionDiagnostics* out_diag) {
  return prom_reactor_runtime_reduction_diagnostics_impl(handle, out_diag);
}

int prometheus_reactor_runtime_reduction_benchmark(void* handle,
                                                   const PrometheusReductionBenchmarkRequest* request,
                                                   PrometheusReductionBenchmarkResult* out_result) {
  return prom_reactor_runtime_reduction_benchmark_impl(handle, request, out_result);
}

int prometheus_reactor_runtime_model_block_create(
    void* handle, const PrometheusModelBlockCreateRequest* request, uint64_t* out_block_id,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_model_block_create_impl(handle, request, out_block_id, out_evidence);
}

int prometheus_reactor_runtime_model_block_upload_weights(
    void* handle, uint64_t block_id, const PrometheusModelBlockWeightUpload* uploads,
    uint32_t upload_count, PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_model_block_upload_weights_impl(handle, block_id, uploads, upload_count,
                                                               out_evidence);
}

int prometheus_reactor_runtime_model_block_execute(
    void* handle, uint64_t block_id, const PrometheusModelBlockExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_model_block_execute_impl(handle, block_id, request, out_evidence);
}

int prometheus_reactor_runtime_model_block_execute_m1b(
    void* handle, uint64_t block_id, const PrometheusModelBlockM1BExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_model_block_execute_m1b_impl(handle, block_id, request, out_evidence);
}

int prometheus_reactor_runtime_model_block_execute_m1c(
    void* handle, uint64_t block_id, const PrometheusModelBlockM1CExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_model_block_execute_m1c_impl(handle, block_id, request, out_evidence);
}

int prometheus_reactor_runtime_model_block_execute_m1d(
    void* handle, uint64_t block_id, const PrometheusModelBlockM1DExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_model_block_execute_m1d_impl(handle, block_id, request, out_evidence);
}

int prometheus_reactor_runtime_noise_refiner0_execute(
    void* handle, uint64_t block_id, const PrometheusNoiseRefiner0ExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_noise_refiner0_execute_impl(handle, block_id, request, out_evidence);
}

int prometheus_reactor_runtime_noise_refiner1_execute(
    void* handle, uint64_t block_id, const PrometheusNoiseRefiner1ExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_noise_refiner1_execute_impl(handle, block_id, request, out_evidence);
}

int prometheus_reactor_runtime_noise_refiner_rebind(
    void* handle, uint64_t block_id, const PrometheusNoiseRefinerRebindRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_noise_refiner_rebind_impl(handle, block_id, request, out_evidence);
}

int prometheus_reactor_runtime_noise_refiner_execute_resident(
    void* handle, uint64_t block_id, const PrometheusNoiseRefinerResidentExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_noise_refiner_execute_resident_impl(handle, block_id, request, out_evidence);
}

int prometheus_reactor_runtime_noise_refiner_execute_static_audit(
    void* handle, uint64_t block_id, const PrometheusNoiseRefinerStaticAuditRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_noise_refiner_execute_static_audit_impl(handle, block_id, request, out_evidence);
}

int prometheus_reactor_runtime_noise_refiner_audit_final(
    void* handle, uint64_t block_id, const PrometheusNoiseRefinerFinalAuditRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_noise_refiner_audit_final_impl(handle, block_id, request, out_evidence);
}

int prometheus_reactor_runtime_context_refiner_create(
    void* handle, const PrometheusContextRefinerCreateRequest* request, uint64_t* out_block_id,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_context_refiner_create_impl(handle, request, out_block_id, out_evidence);
}

int prometheus_reactor_runtime_context_refiner_rebind(
    void* handle, uint64_t block_id, const PrometheusContextRefinerRebindRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_context_refiner_rebind_impl(handle, block_id, request, out_evidence);
}

int prometheus_reactor_runtime_context_refiner0_execute(
    void* handle, uint64_t block_id, const PrometheusContextRefiner0ExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_context_refiner0_execute_impl(handle, block_id, request, out_evidence);
}

int prometheus_reactor_runtime_context_refiner_execute_resident(
    void* handle, uint64_t block_id, const PrometheusContextRefinerResidentExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_context_refiner_execute_resident_impl(handle, block_id, request, out_evidence);
}

int prometheus_reactor_runtime_context_refiner_execute_static_audit(
    void* handle, uint64_t block_id, const PrometheusContextRefinerStaticAuditRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_context_refiner_execute_static_audit_impl(handle, block_id, request, out_evidence);
}

int prometheus_reactor_runtime_context_refiner_audit_final(
    void* handle, uint64_t block_id, const PrometheusContextRefinerFinalAuditRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_context_refiner_audit_final_impl(handle, block_id, request, out_evidence);
}

int prometheus_reactor_runtime_main_transformer_create(
    void* handle, const PrometheusMainTransformerCreateRequest* request, uint64_t* out_block_id,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_main_transformer_create_impl(handle, request, out_block_id, out_evidence);
}

int prometheus_reactor_runtime_main_transformer_execute(
    void* handle, uint64_t block_id, const PrometheusMainTransformerExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_main_transformer_execute_impl(handle, block_id, request, out_evidence);
}

int prometheus_reactor_runtime_main_transformer_audit_final(
    void* handle, uint64_t block_id, const PrometheusMainTransformerFinalAuditRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_main_transformer_audit_final_impl(handle, block_id, request, out_evidence);
}

int prometheus_reactor_runtime_model_block_get_evidence(void* handle, uint64_t block_id,
                                                        PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_model_block_get_evidence_impl(handle, block_id, out_evidence);
}

int prometheus_reactor_runtime_model_block_destroy(void* handle, uint64_t block_id) {
  return prom_reactor_runtime_model_block_destroy_impl(handle, block_id);
}

int prometheus_reactor_runtime_compiled_model_session_create(
    void* handle, const PrometheusCompiledModelSessionCreateRequest* request, uint64_t* out_session_id,
    PrometheusCompiledModelSessionEvidence* out_evidence) {
  return prom_reactor_runtime_compiled_model_session_create_impl(handle, request, out_session_id, out_evidence);
}

int prometheus_reactor_runtime_compiled_model_session_capture_completed(
    void* handle, uint64_t session_id, uint64_t completed_block_id,
    const PrometheusCompiledModelSessionCaptureRequest* request,
    PrometheusCompiledModelSessionEvidence* out_evidence) {
  return prom_reactor_runtime_compiled_model_session_capture_completed_impl(
      handle, session_id, completed_block_id, request, out_evidence);
}

int prometheus_reactor_runtime_compiled_model_session_compose_joint(
    void* handle, uint64_t session_id, const PrometheusCompiledModelSessionComposeRequest* request,
    PrometheusCompiledModelSessionEvidence* out_evidence) {
  return prom_reactor_runtime_compiled_model_session_compose_joint_impl(handle, session_id, request, out_evidence);
}

int prometheus_reactor_runtime_compiled_model_session_get_evidence(
    void* handle, uint64_t session_id, PrometheusCompiledModelSessionEvidence* out_evidence) {
  return prom_reactor_runtime_compiled_model_session_get_evidence_impl(handle, session_id, out_evidence);
}

int prometheus_reactor_runtime_compiled_model_session_destroy(void* handle, uint64_t session_id) {
  return prom_reactor_runtime_compiled_model_session_destroy_impl(handle, session_id);
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
