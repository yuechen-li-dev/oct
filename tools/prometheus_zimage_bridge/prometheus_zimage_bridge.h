#ifndef OCT_TOOLS_PROMETHEUS_ZIMAGE_BRIDGE_H
#define OCT_TOOLS_PROMETHEUS_ZIMAGE_BRIDGE_H

#include <stdint.h>

#if defined(_WIN32)
#define PROM_ZIMAGE_BRIDGE_API __declspec(dllimport)
#else
#define PROM_ZIMAGE_BRIDGE_API
#endif

#ifdef __cplusplus
extern "C" {
#endif

typedef struct PrometheusZImageSessionCreateRequest {
  uint32_t struct_size;
  const char* reactor_dll_path;
  const char* compiled_model_lock_path;
  const char* payload_root;
  int32_t device_index;
  uint32_t execution_profile; /* 1=MinimumMemory, 2=Prefetch */
} PrometheusZImageSessionCreateRequest;

typedef struct PrometheusZImageExecuteRequest {
  uint32_t struct_size;
  const void* image_bf16;
  uint64_t image_bytes;
  uint32_t image_batch;
  uint32_t image_tokens;
  uint32_t image_width;
  const float* context_fp32;
  uint64_t context_bytes;
  uint32_t context_batch;
  uint32_t context_tokens;
  uint32_t context_width;
  const void* timestep_bf16;
  uint64_t timestep_bytes;
  uint32_t timestep_batch;
  uint32_t timestep_width;
  float* output_image_fp32;
  uint64_t output_image_bytes;
} PrometheusZImageExecuteRequest;

typedef struct PrometheusZImageExecuteEvidence {
  uint32_t struct_size;
  uint32_t evaluation_index;
  uint32_t main_layer_count;
  uint32_t context_reused;
  uint64_t wall_time_ns;
  uint64_t model_execution_ns;
  uint64_t parameter_rebind_ns;
  uint64_t uploaded_weight_bytes;
  uint64_t model_allocation_ceiling_bytes;
  uint64_t persistent_bytes;
  uint64_t reusable_bytes;
  uint64_t audit_bytes;
  uint64_t host_package_cache_bytes;
  uint64_t host_package_cache_hits;
  uint64_t prefetch_transfer_ns;
  uint64_t prefetch_overlap_ns;
  uint64_t prefetch_wait_ns;
  uint32_t prefetch_count;
  uint32_t reserved0;
  /* Ordered as NoiseRefiner0, NoiseRefiner1, ContextRefiner0,
     ContextRefiner1, MainTransformer0..29.  These are host-visible
     per-block probes; last_execution_ns is supplied by the native reactor. */
  uint64_t stage_execution_ns[34];
  uint64_t stage_gpu_execution_ns[34];
  uint64_t stage_rebind_ns[34];
  uint64_t stage_payload_read_ns[34];
  uint64_t stage_uploaded_weight_bytes[34];
  /* DVT-2 M3 correlated MainTransformer traces. Stage order is documented in
     dvt2_m3_timing_schema.json; main_native_counters uses 19 counters/layer. */
  uint64_t main_correlation_id[30];
  uint64_t main_cpu_begin_ns[30];
  uint64_t main_cpu_end_ns[30];
  uint64_t main_parameter_generation[30];
  uint64_t main_execution_generation[30];
  uint32_t main_active_weight_window[30];
  uint64_t main_gpu_total_begin_tick[30];
  uint64_t main_gpu_total_end_tick[30];
  uint64_t main_gpu_total_ns[30];
  uint64_t main_gpu_compute_begin_tick[30];
  uint64_t main_gpu_compute_end_tick[30];
  uint64_t main_gpu_compute_ns[30];
  uint64_t main_gpu_ingress_transfer_ns[30];
  uint64_t main_gpu_joint_copy_ns[30];
  uint64_t main_stage_gpu_begin_tick[390];
  uint64_t main_stage_gpu_end_tick[390];
  uint64_t main_stage_gpu_ns[390];
  uint64_t main_active_target_validation_ns[30];
  uint64_t main_command_reset_ns[30];
  uint64_t main_command_begin_ns[30];
  uint64_t main_command_record_ns[30];
  uint64_t main_command_end_ns[30];
  uint64_t main_queue_submit_ns[30];
  uint64_t main_fence_wait_ns[30];
  uint64_t main_descriptor_update_ns[30];
  uint64_t main_staging_memcpy_ns[30];
  uint64_t main_native_counters[570];
  uint64_t final_readback_gpu_ns;
  uint64_t final_readback_host_ns;
} PrometheusZImageExecuteEvidence;

PROM_ZIMAGE_BRIDGE_API uint32_t prometheus_zimage_bridge_abi_version(void);
PROM_ZIMAGE_BRIDGE_API int prometheus_zimage_session_create(
    PrometheusZImageSessionCreateRequest* request, uint64_t* out_handle);
PROM_ZIMAGE_BRIDGE_API int prometheus_zimage_session_execute(
    uint64_t handle, PrometheusZImageExecuteRequest* request,
    PrometheusZImageExecuteEvidence* evidence);
PROM_ZIMAGE_BRIDGE_API int prometheus_zimage_session_destroy(uint64_t handle);
PROM_ZIMAGE_BRIDGE_API uint64_t prometheus_zimage_last_error(
    uint64_t handle, char* destination, uint64_t capacity);

#ifdef __cplusplus
}
#endif

#endif
