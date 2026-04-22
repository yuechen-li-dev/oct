#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_API_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_API_H

#include <stdint.h>

#if defined(_WIN32)
#if defined(PROMETHEUS_REACTOR_BUILD_DLL)
#define PROM_REACTOR_API __declspec(dllexport)
#elif defined(PROMETHEUS_REACTOR_USE_DLL)
#define PROM_REACTOR_API __declspec(dllimport)
#else
#define PROM_REACTOR_API
#endif
#else
#define PROM_REACTOR_API
#endif

#ifdef __cplusplus
extern "C" {
#endif

enum {
  PROM_OK = 0,
  PROM_ERROR = 1,
  PROM_INVALID_HANDLE = 2,
  PROM_INTERNAL_ERROR = 3,
};

enum {
  PROM_BACKEND_UNKNOWN = 0,
  PROM_BACKEND_STUB = 1,
  PROM_BACKEND_VULKAN = 2,
  PROM_BACKEND_VULKAN_SOFTWARE = 3,
};

enum {
  PROM_REASON_NONE = 0,
  PROM_REASON_STUB_UNAVAILABLE = 1,
  PROM_REASON_VULKAN_UNAVAILABLE = 2,
};

enum {
  PROM_STAGE_NONE = 0,
  PROM_STAGE_INIT = 1,
  PROM_STAGE_TRANSFER_IN = 2,
  PROM_STAGE_SUBMIT = 3,
  PROM_STAGE_TRANSFER_OUT = 4,
  PROM_STAGE_CLEANUP = 5,
};

enum {
  PROM_TESTCFG_FAIL_DEVICE_CREATE = 1u << 0,
  PROM_TESTCFG_FAIL_PIPELINE_CREATE = 1u << 1,
  PROM_TESTCFG_FAIL_BUFFER_ALLOC = 1u << 2,
  PROM_TESTCFG_FAIL_UPLOAD = 1u << 3,
  PROM_TESTCFG_FAIL_DISPATCH = 1u << 4,
  PROM_TESTCFG_FAIL_DOWNLOAD = 1u << 5,
  PROM_TESTCFG_FORCE_NO_MEMORY_TYPE = 1u << 6,
  PROM_TESTCFG_SKIP_VULKAN_INIT = 1u << 7,
  PROM_TESTCFG_SKIP_SUBMIT_WAIT = 1u << 8,
  PROM_TESTCFG_FORCE_NO_DEVICE_LOCAL_MEMORY = 1u << 9,
  PROM_TESTCFG_FORCE_STAGED_PATH = 1u << 10,
  PROM_TESTCFG_FORCE_DIRECT_PATH = 1u << 11,
  PROM_TESTCFG_FORCE_UPLOAD_ONLY = 1u << 12,
  PROM_TESTCFG_DISABLE_STAGING_FALLBACK = 1u << 13,
  PROM_TESTCFG_FORCE_TILED_PATH = 1u << 14,
};

enum {
  PROM_DETAIL_INJECTED_UPLOAD_FAILURE = -6001,
  PROM_DETAIL_INJECTED_DISPATCH_FAILURE = -6002,
  PROM_DETAIL_INJECTED_DOWNLOAD_FAILURE = -6003,
  PROM_DETAIL_SIZE_OVERFLOW = -6004,
  PROM_DETAIL_REUSE_IN_FLIGHT = -6005,
  PROM_DETAIL_CAPABILITY_MISMATCH = -6006,
  PROM_DETAIL_PATH_DIRECT = 6101,
  PROM_DETAIL_PATH_STAGED_UPLOAD = 6102,
  PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK = 6103,
  PROM_DETAIL_PATH_FALLBACK_TO_DIRECT = 6104,
  PROM_DETAIL_PATH_DIRECT_TILED = 6105,
  PROM_DETAIL_PATH_STAGED_UPLOAD_TILED = 6106,
  PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK_TILED = 6107,
  /* Backward-compat alias used by earlier P8d tests/reports. */
  PROM_DETAIL_PATH_TILED = PROM_DETAIL_PATH_DIRECT_TILED,
};

typedef struct PrometheusCaps {
  uint32_t available;
  uint32_t backend_type;
  uint32_t reason_code;
} PrometheusCaps;

typedef struct PrometheusReactorConfig {
  uint32_t struct_size;
  uint32_t test_flags;
} PrometheusReactorConfig;

PROM_REACTOR_API uint32_t prometheus_reactor_abi_version(void);
PROM_REACTOR_API int prometheus_reactor_runtime_create(void* config, void** out_handle);
PROM_REACTOR_API int prometheus_reactor_runtime_destroy(void* handle);
PROM_REACTOR_API int prometheus_reactor_runtime_probe(void* handle, PrometheusCaps* out_caps);
PROM_REACTOR_API int prometheus_reactor_runtime_sgemm(void* handle,
                                                      const float* a,
                                                      const float* b,
                                                      float* c,
                                                      uint32_t m,
                                                      uint32_t n,
                                                      uint32_t k,
                                                      uint32_t* out_stage,
                                                      int* out_detail_code);

/* Backward-compat aliases for earlier contract drafts. */
PROM_REACTOR_API int prometheus_runtime_create(void* config, void** out_handle);
PROM_REACTOR_API int prometheus_runtime_destroy(void* handle);
PROM_REACTOR_API int prometheus_runtime_probe(void* handle, PrometheusCaps* out_caps);

#ifdef __cplusplus
}
#endif

#endif
