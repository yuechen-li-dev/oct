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
