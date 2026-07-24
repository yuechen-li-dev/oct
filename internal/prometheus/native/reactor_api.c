#include "reactor_api.h"

#include "reactor_vulkan.h"
#include "reactor_numerical_research.h"

#include <string.h>

static int prom_reactor_api_checked_product_u64(uint64_t left, uint64_t right,
                                                uint64_t* out_value) {
  if (out_value == NULL || (right != 0u && left > UINT64_MAX / right)) return 0;
  *out_value = left * right;
  return 1;
}

uint32_t prometheus_reactor_abi_version(void) {
  return PROM_REACTOR_ABI_V1;
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

int prometheus_reactor_runtime_ray_query_triangle_scene_create(
    void* handle, const PrometheusRayQueryTriangleSceneCreateRequest* request, uint64_t* out_scene_id) {
  return prom_ray_query_triangle_scene_create_impl(handle, request, out_scene_id);
}

int prometheus_reactor_runtime_ray_query_triangle_scene_probe(
    void* handle, uint64_t scene_id, PrometheusRayQueryProbeResult* out_result) {
  return prom_ray_query_triangle_scene_probe_impl(handle, scene_id, out_result);
}

int prometheus_reactor_runtime_ray_query_triangle_scene_destroy(void* handle, uint64_t scene_id) {
  return prom_ray_query_triangle_scene_destroy_impl(handle, scene_id);
}

int prometheus_reactor_runtime_ray_query_scene_create(
    void* handle, const PrometheusRayQuerySceneCreateRequest* request, uint64_t* out_scene_id) {
  return prom_ray_query_scene_create_impl(handle, request, out_scene_id);
}

int prometheus_reactor_runtime_ray_query_scene_trace(
    void* handle, uint64_t scene_id, const PrometheusRayQueryRawRequest* request,
    PrometheusRayQueryRawHit* out_hit) {
  return prom_ray_query_scene_trace_impl(handle, scene_id, request, out_hit);
}

int prometheus_reactor_runtime_ray_query_scene_destroy(void* handle, uint64_t scene_id) {
  return prom_ray_query_scene_destroy_impl(handle, scene_id);
}

int prometheus_reactor_runtime_ray_query_scene_create_empty(void* handle, uint64_t* out_scene_id) {
  return prom_ray_query_scene_create_empty_impl(handle, out_scene_id);
}

int prometheus_ray_query_runtime_create(const PrometheusRayQueryRuntimeConfig* config, void** out_handle) {
  PrometheusReactorConfig reactor_config;
  if (config == NULL || config->struct_size < sizeof(*config)) return PROM_ERROR;
  memset(&reactor_config, 0, sizeof(reactor_config));
  reactor_config.struct_size = sizeof(reactor_config);
  reactor_config.shader_package_root = config->shader_package_root;
  return prom_reactor_runtime_create_impl(&reactor_config, out_handle);
}

int prometheus_reactor_runtime_ray_query_scene_add_triangles(
    void* handle, uint64_t scene_id, const PrometheusRayQueryTriangle* triangles, uint32_t triangle_count) {
  return prom_ray_query_scene_add_triangles_impl(handle, scene_id, triangles, triangle_count);
}

int prometheus_reactor_runtime_ray_query_scene_add_spheres(
    void* handle, uint64_t scene_id, const PrometheusRayQuerySphere* spheres, uint32_t sphere_count) {
  return prom_ray_query_scene_add_spheres_impl(handle, scene_id, spheres, sphere_count);
}

int prometheus_reactor_runtime_ray_query_scene_commit(void* handle, uint64_t scene_id) {
  return prom_ray_query_scene_commit_impl(handle, scene_id);
}

int prometheus_reactor_runtime_ray_query_scene_submit_batch(
    void* handle, uint64_t scene_id, const PrometheusRayQueryBatchRequest* request,
    PrometheusRayQueryBatchResult* out_result) {
  return prom_ray_query_scene_submit_batch_impl(handle, scene_id, request, out_result);
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

int prometheus_reactor_runtime_row_wise_softmax(
    void* handle,
    const PrometheusRowWiseSoftmaxRequest* request,
    PrometheusRowWiseSoftmaxResult* out_result) {
  return prom_reactor_runtime_row_wise_softmax_impl(handle, request, out_result);
}

static int prometheus_reactor_runtime_gemma4e2b_m1_rmsnorm_impl(
    void* handle,
    const PrometheusGemma4E2BM1InputRmsNormRequest* request,
    PrometheusGemma4E2BM1InputRmsNormResult* out_result,
    uint32_t bf16_roundtrip_input,
    uint32_t bf16_roundtrip_output) {
  prom_m46_weight_prepare_request weight_request;
  prom_m46_weight_prepare_result weight_result;
  prom_m49a_m46_request execute_request;
  prom_m49a_m46_result execute_result;
  uint64_t input_hash;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  out_result->struct_size = sizeof(*out_result);
  if (request == NULL || request->struct_size < sizeof(*request) ||
      request->input == NULL || request->weight == NULL || request->output == NULL ||
      request->inv_rms_output == NULL || request->tokens == 0u ||
      request->model_width == 0u || request->input_row_stride < request->model_width ||
      request->input_generation == 0u || request->weight_generation == 0u ||
      request->exact_source_hash == 0u) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M46_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  memset(&weight_request, 0, sizeof(weight_request));
  weight_request.values = request->weight;
  weight_request.element_count = request->weight_element_count;
  weight_request.model_width = request->model_width;
  weight_request.generation = request->weight_generation;
  memset(&weight_result, 0, sizeof(weight_result));
  if (prom_reactor_runtime_m46_prepare_weight(handle, &weight_request, &weight_result) != PROM_OK) {
    out_result->stage = weight_result.stage;
    out_result->detail_code = weight_result.detail_code;
    out_result->weight_hash = weight_result.hash;
    out_result->retained_bytes = weight_result.retained_bytes;
    out_result->end_to_end_ns = weight_result.preparation_ns;
    return PROM_ERROR;
  }
  input_hash = prom_num_hash_float_bits(request->input, request->input_element_count);
  memset(&execute_request, 0, sizeof(execute_request));
  execute_request.matched_z = request->input;
  execute_request.matched_storage_element_count = request->input_element_count;
  execute_request.output = request->output;
  execute_request.output_element_count = request->output_element_count;
  execute_request.inv_rms_output = request->inv_rms_output;
  execute_request.inv_rms_output_element_count = request->inv_rms_output_element_count;
  execute_request.tokens = request->tokens;
  execute_request.model_width = request->model_width;
  execute_request.z_row_stride = request->input_row_stride;
  execute_request.strategy = PROM_M46_STRATEGY_SEPARATE_OUTPUT;
  execute_request.requested_reduction_plan = PROM_M46_REDUCTION_AUTO;
  execute_request.epsilon = request->epsilon;
  execute_request.input_generation = request->input_generation;
  execute_request.reference_input_hash = input_hash;
  execute_request.required_weight_generation = weight_result.generation;
  execute_request.required_weight_hash = weight_result.hash;
  execute_request.exact_source_hash = request->exact_source_hash;
  execute_request.bf16_roundtrip_input = bf16_roundtrip_input;
  execute_request.bf16_roundtrip_output = bf16_roundtrip_output;
  memset(&execute_result, 0, sizeof(execute_result));
  if (prom_reactor_runtime_m49a_execute_m46(handle, &execute_request, &execute_result) != PROM_OK) {
    out_result->stage = execute_result.stage;
    out_result->detail_code = execute_result.detail_code;
    out_result->matched_input = execute_result.matched_input;
    out_result->input_hash = execute_result.input_hash;
    out_result->weight_hash = execute_result.weight_hash;
    out_result->retained_bytes = execute_result.rmsnorm.retained_bytes;
    out_result->buffer_allocation_count = execute_result.rmsnorm.buffer_allocation_count;
    out_result->buffer_reuse_count = execute_result.rmsnorm.buffer_reuse_count;
    out_result->descriptor_update_count = execute_result.rmsnorm.descriptor_update_count;
    out_result->pipeline_create_count = execute_result.rmsnorm.pipeline_create_count;
    out_result->command_buffer_reuse_count = execute_result.rmsnorm.command_buffer_reuse_count;
    out_result->reduction_gpu_ns = execute_result.rmsnorm.reduction_gpu_ns;
    out_result->final_reduction_gpu_ns = execute_result.rmsnorm.final_reduction_gpu_ns;
    out_result->inv_rms_gpu_ns = execute_result.rmsnorm.inv_rms_gpu_ns;
    out_result->apply_gpu_ns = execute_result.rmsnorm.apply_gpu_ns;
    out_result->end_to_end_ns = execute_result.rmsnorm.end_to_end_ns;
    return PROM_ERROR;
  }
  out_result->stage = execute_result.stage;
  out_result->detail_code = execute_result.detail_code;
  out_result->output_written = 1u;
  out_result->matched_input = execute_result.matched_input;
  out_result->input_hash = execute_result.input_hash;
  out_result->weight_hash = execute_result.weight_hash;
  out_result->output_hash = execute_result.output_hash;
  out_result->inv_rms_hash = execute_result.inv_rms_hash;
  out_result->submit_count = execute_result.rmsnorm.submit_count;
  out_result->final_readback_count = execute_result.rmsnorm.final_readback_count;
  out_result->no_product_intermediate_readback_change = execute_result.no_product_intermediate_readback_change;
  out_result->retained_bytes = execute_result.rmsnorm.retained_bytes;
  out_result->buffer_allocation_count = execute_result.rmsnorm.buffer_allocation_count;
  out_result->buffer_reuse_count = execute_result.rmsnorm.buffer_reuse_count;
  out_result->descriptor_update_count = execute_result.rmsnorm.descriptor_update_count;
  out_result->pipeline_create_count = execute_result.rmsnorm.pipeline_create_count;
  out_result->command_buffer_reuse_count = execute_result.rmsnorm.command_buffer_reuse_count;
  out_result->reduction_gpu_ns = execute_result.rmsnorm.reduction_gpu_ns;
  out_result->final_reduction_gpu_ns = execute_result.rmsnorm.final_reduction_gpu_ns;
  out_result->inv_rms_gpu_ns = execute_result.rmsnorm.inv_rms_gpu_ns;
  out_result->apply_gpu_ns = execute_result.rmsnorm.apply_gpu_ns;
  out_result->end_to_end_ns = execute_result.rmsnorm.end_to_end_ns;
  return PROM_OK;
}

int prometheus_reactor_runtime_gemma4e2b_m1_input_rmsnorm(
    void* handle,
    const PrometheusGemma4E2BM1InputRmsNormRequest* request,
    PrometheusGemma4E2BM1InputRmsNormResult* out_result) {
  return prometheus_reactor_runtime_gemma4e2b_m1_rmsnorm_impl(handle, request,
                                                               out_result, 0u, 0u);
}

int prometheus_reactor_runtime_gemma4e2b_m1_projection_activation_rmsnorm(
    void* handle,
    const PrometheusGemma4E2BM1InputRmsNormRequest* request,
    PrometheusGemma4E2BM1InputRmsNormResult* out_result) {
  return prometheus_reactor_runtime_gemma4e2b_m1_rmsnorm_impl(handle, request,
                                                               out_result, 0u, 1u);
}

int prometheus_reactor_runtime_gemma4e2b_m1_head_rmsnorm(
    void* handle,
    const PrometheusGemma4E2BM1HeadRmsNormRequest* request,
    PrometheusGemma4E2BM1HeadRmsNormResult* out_result) {
  /* This retains M46's demonstrated arithmetic but admits only the M1 Q/K
     geometry. Its resident FP32 -> BF16 -> FP32 stage is deliberately here,
     before the first per-head reduction, rather than a downstream repair. */
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  out_result->struct_size = sizeof(*out_result);
  if (request == NULL || request->struct_size < sizeof(*request) ||
      request->input == NULL || request->weight == NULL || request->output == NULL ||
      request->inv_rms_output == NULL || request->tokens == 0u || request->tokens > 120u ||
      request->model_width != 256u || request->input_row_stride != 256u ||
      request->input_element_count != (uint64_t)request->tokens * 256u ||
      request->weight_element_count != 256u ||
      request->output_element_count != (uint64_t)request->tokens * 256u ||
      request->inv_rms_output_element_count != request->tokens ||
      request->epsilon != 0.000001f) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M46_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  return prometheus_reactor_runtime_gemma4e2b_m1_rmsnorm_impl(handle, request,
                                                               out_result, 1u, 1u);
}

int prometheus_reactor_runtime_gemma4e2b_m1_rope(
    void* handle,
    const PrometheusGemma4E2BM1RopeRequest* request,
    PrometheusGemma4E2BM1RopeResult* out_result) {
  return prom_reactor_runtime_gemma4e2b_m1_rope(handle, request, out_result);
}

int prometheus_reactor_runtime_gemma4e2b_m1_head_rmsnorm_rope(
    void* handle,
    const PrometheusGemma4E2BM1HeadRmsNormRopeRequest* request,
    PrometheusGemma4E2BM1HeadRmsNormRopeResult* out_result) {
  prom_m46_weight_prepare_request weight_request;
  prom_m46_weight_prepare_result weight_result;
  prom_m49a_m46_request execute_request;
  prom_m49a_m46_result execute_result;
  uint64_t rows;
  uint64_t input_hash;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  out_result->struct_size = sizeof(*out_result);
  if (request == NULL || request->struct_size < sizeof(*request) ||
      request->input == NULL || request->weight == NULL || request->cosine == NULL ||
      request->sine == NULL || (request->output == NULL && request->output_element_count != 0u) ||
      request->tokens != 15u ||
      (request->heads != 1u && request->heads != 8u) || request->head_width != 256u ||
      request->epsilon != 0.000001f || request->input_generation == 0u ||
      request->weight_generation == 0u || request->table_generation == 0u ||
      request->exact_source_hash == 0u) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M46_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  rows = (uint64_t)request->tokens * request->heads;
  if (request->input_element_count != rows * request->head_width ||
      request->weight_element_count != request->head_width ||
      request->cosine_element_count != (uint64_t)request->tokens * request->head_width ||
      request->sine_element_count != (uint64_t)request->tokens * request->head_width ||
      (request->output != NULL && request->output_element_count != request->input_element_count)) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M46_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  memset(&weight_request, 0, sizeof(weight_request));
  weight_request.values = request->weight;
  weight_request.element_count = request->weight_element_count;
  weight_request.model_width = request->head_width;
  weight_request.generation = request->weight_generation;
  memset(&weight_result, 0, sizeof(weight_result));
  if (prom_reactor_runtime_m46_prepare_weight(handle, &weight_request, &weight_result) != PROM_OK) {
    out_result->stage = weight_result.stage;
    out_result->detail_code = weight_result.detail_code;
    out_result->weight_hash = weight_result.hash;
    out_result->observed_weight_generation = weight_result.observed_generation;
    out_result->requested_weight_generation = weight_result.requested_generation;
    return PROM_ERROR;
  }
  input_hash = prom_num_hash_float_bits(request->input, request->input_element_count);
  memset(&execute_request, 0, sizeof(execute_request));
  execute_request.matched_z = request->input;
  execute_request.matched_storage_element_count = request->input_element_count;
  execute_request.tokens = (uint32_t)rows;
  execute_request.model_width = request->head_width;
  execute_request.z_row_stride = request->head_width;
  execute_request.strategy = PROM_M46_STRATEGY_SEPARATE_OUTPUT;
  execute_request.requested_reduction_plan = PROM_M46_REDUCTION_AUTO;
  execute_request.epsilon = request->epsilon;
  execute_request.input_generation = request->input_generation;
  execute_request.reference_input_hash = input_hash;
  execute_request.required_weight_generation = weight_result.generation;
  execute_request.required_weight_hash = weight_result.hash;
  execute_request.exact_source_hash = request->exact_source_hash;
  execute_request.bf16_roundtrip_input = 1u;
  execute_request.bf16_roundtrip_output = 1u;
  execute_request.direct_rope = 1u;
  execute_request.rope_cosine = request->cosine;
  execute_request.rope_sine = request->sine;
  execute_request.rope_output = request->output;
  execute_request.rope_cosine_element_count = request->cosine_element_count;
  execute_request.rope_sine_element_count = request->sine_element_count;
  execute_request.rope_output_element_count = request->output_element_count;
  execute_request.rope_tokens = request->tokens;
  execute_request.rope_heads = request->heads;
  execute_request.rope_table_generation = request->table_generation;
  memset(&execute_result, 0, sizeof(execute_result));
  if (prom_reactor_runtime_m49a_execute_m46(handle, &execute_request, &execute_result) != PROM_OK) {
    out_result->stage = execute_result.stage;
    out_result->detail_code = execute_result.detail_code;
    return PROM_ERROR;
  }
  out_result->stage = execute_result.stage;
  out_result->detail_code = execute_result.detail_code;
  out_result->output_written = request->output != NULL ? 1u : 0u;
  out_result->dispatch_count = 1u;
  out_result->resident_source_bound = execute_result.resident_rope_source_bound;
  out_result->normalized_readback_count = execute_result.normalized_readback_count;
  out_result->source_byte_range = execute_result.rope_source_byte_range;
  out_result->destination_byte_range = execute_result.rope_destination_byte_range;
  out_result->input_hash = execute_result.input_hash;
  out_result->weight_hash = execute_result.weight_hash;
  out_result->observed_weight_generation = weight_result.observed_generation;
  out_result->requested_weight_generation = weight_result.requested_generation;
  out_result->output_hash = execute_result.output_hash;
  out_result->buffer_allocation_count = execute_result.rmsnorm.buffer_allocation_count;
  out_result->buffer_reuse_count = execute_result.rmsnorm.buffer_reuse_count;
  out_result->descriptor_update_count = execute_result.rope_descriptor_update_count;
  out_result->pipeline_create_count = execute_result.rope_pipeline_create_count;
  out_result->command_buffer_reuse_count = execute_result.rmsnorm.command_buffer_reuse_count;
  return PROM_OK;
}

int prometheus_reactor_runtime_gemma4e2b_m1_attention_scores(
    void* handle, const PrometheusGemma4E2BM1AttentionScoresRequest* request,
    PrometheusGemma4E2BM1AttentionScoresResult* out_result) {
  PrometheusGemma4E2BM1HeadRmsNormRopeRequest positional;
  PrometheusGemma4E2BM1HeadRmsNormRopeResult positional_result;
  uint64_t query_elements;
  uint64_t key_elements;
  uint64_t score_elements;
  uint32_t preparation_index;
  if (out_result == NULL) return PROM_ERROR;
  memset(out_result, 0, sizeof(*out_result));
  out_result->struct_size = sizeof(*out_result);
  if (request == NULL || request->struct_size < sizeof(*request) ||
      request->query_input == NULL || request->key_input == NULL ||
      request->query_weight == NULL || request->key_weight == NULL ||
      request->cosine == NULL || request->sine == NULL || request->scores == NULL ||
      request->tokens != 15u || request->query_heads != 8u || request->key_heads != 1u ||
      request->head_width != 256u || request->epsilon != 0.000001f ||
      request->scale != 0.0625f || request->preparation_order > 1u ||
      request->query_input_generation == 0u ||
      request->key_input_generation == 0u || request->query_weight_generation == 0u ||
      request->key_weight_generation == 0u || request->table_generation == 0u ||
      request->query_exact_source_hash == 0u || request->key_exact_source_hash == 0u ||
      !prom_reactor_api_checked_product_u64(request->tokens, request->query_heads, &query_elements) ||
      !prom_reactor_api_checked_product_u64(query_elements, request->head_width, &query_elements) ||
      !prom_reactor_api_checked_product_u64(request->tokens, request->key_heads, &key_elements) ||
      !prom_reactor_api_checked_product_u64(key_elements, request->head_width, &key_elements) ||
      !prom_reactor_api_checked_product_u64(request->query_heads, request->tokens, &score_elements) ||
      !prom_reactor_api_checked_product_u64(score_elements, request->tokens, &score_elements) ||
      request->query_input_element_count != query_elements ||
      request->key_input_element_count != key_elements ||
      request->query_weight_element_count != request->head_width ||
      request->key_weight_element_count != request->head_width ||
      request->cosine_element_count != (uint64_t)request->tokens * request->head_width ||
      request->sine_element_count != (uint64_t)request->tokens * request->head_width ||
      request->score_element_count != score_elements) {
    out_result->stage = PROM_STAGE_INIT;
    out_result->detail_code = PROM_M46_DETAIL_INVALID_REQUEST;
    return PROM_ERROR;
  }
  memset(&positional, 0, sizeof(positional));
  positional.struct_size = sizeof(positional);
  positional.cosine = request->cosine;
  positional.sine = request->sine;
  positional.cosine_element_count = request->cosine_element_count;
  positional.sine_element_count = request->sine_element_count;
  positional.tokens = request->tokens;
  positional.head_width = request->head_width;
  positional.epsilon = request->epsilon;
  positional.table_generation = request->table_generation;
  for (preparation_index = 0u; preparation_index < 2u; ++preparation_index) {
    const uint32_t prepare_key = request->preparation_order == 0u
                                     ? preparation_index == 1u
                                     : preparation_index == 0u;
    positional.input = prepare_key != 0u ? request->key_input : request->query_input;
    positional.weight = prepare_key != 0u ? request->key_weight : request->query_weight;
    positional.input_element_count = prepare_key != 0u ? key_elements : query_elements;
    positional.weight_element_count = request->head_width;
    positional.heads = prepare_key != 0u ? request->key_heads : request->query_heads;
    positional.input_generation = prepare_key != 0u ? request->key_input_generation
                                                     : request->query_input_generation;
    positional.weight_generation = prepare_key != 0u ? request->key_weight_generation
                                                      : request->query_weight_generation;
    positional.exact_source_hash = prepare_key != 0u ? request->key_exact_source_hash
                                                      : request->query_exact_source_hash;
    memset(&positional_result, 0, sizeof(positional_result));
    if (prometheus_reactor_runtime_gemma4e2b_m1_head_rmsnorm_rope(
            handle, &positional, &positional_result) != PROM_OK) {
      out_result->stage = positional_result.stage;
      out_result->detail_code = positional_result.detail_code;
      out_result->observed_weight_generation = positional_result.observed_weight_generation;
      out_result->requested_weight_generation = positional_result.requested_weight_generation;
      return PROM_ERROR;
    }
    out_result->positional_dispatch_count = preparation_index + 1u;
  }
  if (prom_reactor_runtime_gemma4e2b_m1_attention_scores(handle, request, out_result) != PROM_OK)
    return PROM_ERROR;
  out_result->positional_dispatch_count = 2u;
  return PROM_OK;
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

int prometheus_reactor_runtime_main_transformer_rebind(
    void* handle, uint64_t block_id, const PrometheusMainTransformerRebindRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_main_transformer_rebind_impl(handle, block_id, request, out_evidence);
}

int prometheus_reactor_runtime_main_transformer_execute(
    void* handle, uint64_t block_id, const PrometheusMainTransformerExecuteRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_main_transformer_execute_impl(handle, block_id, request, out_evidence);
}

int prometheus_reactor_runtime_main_transformer_execute_static_audit(
    void* handle, uint64_t block_id, const PrometheusMainTransformerStaticAuditRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_main_transformer_execute_static_audit_impl(handle, block_id, request, out_evidence);
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

int prometheus_reactor_runtime_compiled_model_session_set_main_attention_route(
    void* handle, uint64_t session_id, uint32_t main_attention_route_policy,
    PrometheusCompiledModelSessionEvidence* out_evidence) {
  return prom_reactor_runtime_compiled_model_session_set_main_attention_route_impl(
      handle, session_id, main_attention_route_policy, out_evidence);
}

int prometheus_reactor_runtime_compiled_model_session_destroy(void* handle, uint64_t session_id) {
  return prom_reactor_runtime_compiled_model_session_destroy_impl(handle, session_id);
}

int prometheus_reactor_runtime_compiled_model_owner_create(
    void* handle, const PrometheusNoiseRefinerRebindRequest* request, uint64_t* out_block_id,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_compiled_model_owner_create_impl(handle, request, out_block_id, out_evidence);
}

int prometheus_reactor_runtime_compiled_model_retarget(
    void* handle, const PrometheusCompiledModelRetargetRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_compiled_model_retarget_impl(handle, request, out_evidence);
}

int prometheus_reactor_runtime_compiled_model_prefetch(
    void* handle, const PrometheusCompiledModelPrefetchRequest* request,
    PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_compiled_model_prefetch_impl(handle, request, out_evidence);
}

int prometheus_reactor_runtime_compiled_model_activate_prefetch(
    void* handle, uint64_t session_id, PrometheusModelBlockEvidence* out_evidence) {
  return prom_reactor_runtime_compiled_model_activate_prefetch_impl(handle, session_id, out_evidence);
}

int prometheus_reactor_runtime_compiled_model_evaluation_reset(
    void* handle, uint64_t session_id, PrometheusCompiledModelSessionEvidence* out_evidence) {
  return prom_reactor_runtime_compiled_model_evaluation_reset_impl(handle, session_id, out_evidence);
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
