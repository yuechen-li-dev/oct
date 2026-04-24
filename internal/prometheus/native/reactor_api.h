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
  PROM_ASYNC_STATE_IDLE = 0,
  PROM_ASYNC_STATE_SUBMITTED = 1,
  PROM_ASYNC_STATE_READY = 2,
  PROM_ASYNC_STATE_FAILED = 3,
  PROM_ASYNC_STATE_CONSUMED = 4,
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
  PROM_TESTCFG_FAIL_ASYNC_POLL = 1u << 15,
  PROM_TESTCFG_PACKED4_DEBUG_ORACLE_CHECK = 1u << 16,
  PROM_TESTCFG_FORCE_STRICT_FP32 = 1u << 17,
  PROM_TESTCFG_FORCE_NO_FP16_STORAGE = 1u << 18,
  PROM_TESTCFG_FORCE_FP16_UTILITY_WIN = 1u << 19,
  PROM_TESTCFG_FAIL_COMMAND_END = 1u << 20,
  PROM_TESTCFG_FAIL_RESET_FENCE = 1u << 21,
  PROM_TESTCFG_FAIL_QUEUE_SUBMIT = 1u << 22,
};

enum {
  PROM_DETAIL_INJECTED_UPLOAD_FAILURE = -6001,
  PROM_DETAIL_INJECTED_DISPATCH_FAILURE = -6002,
  PROM_DETAIL_INJECTED_DOWNLOAD_FAILURE = -6003,
  PROM_DETAIL_SIZE_OVERFLOW = -6004,
  PROM_DETAIL_REUSE_IN_FLIGHT = -6005,
  PROM_DETAIL_CAPABILITY_MISMATCH = -6006,
  PROM_DETAIL_SLOT_OVERWRITE_REJECTED = -6007,
  PROM_DETAIL_SLOT_STALE_REJECTED = -6008,
  PROM_DETAIL_SLOT_SWAP_REJECTED = -6009,
  PROM_DETAIL_SLOT_INFLIGHT_REJECTED = -6010,
  PROM_DETAIL_SLOT_INVALID_LAYOUT = -6011,
  PROM_DETAIL_SLOT_ASYNC_OWNERSHIP = -6012,
  PROM_DETAIL_SLOT_BUSY_WAIT_REQUIRED = -6013,
  PROM_DETAIL_PATH_DIRECT = 6101,
  PROM_DETAIL_PATH_STAGED_UPLOAD = 6102,
  PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK = 6103,
  PROM_DETAIL_PATH_FALLBACK_TO_DIRECT = 6104,
  PROM_DETAIL_PATH_DIRECT_TILED = 6105,
  PROM_DETAIL_PATH_STAGED_UPLOAD_TILED = 6106,
  PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK_TILED = 6107,
  PROM_DETAIL_PATH_DIRECT_PACKED4_FP32 = 6108,
  PROM_DETAIL_PATH_DIRECT_FP16_STORAGE_FP32_ACCUM = 6109,
  PROM_DETAIL_ASYNC_NOT_READY = -6108,
  PROM_DETAIL_ASYNC_NO_TASK = -6109,
  PROM_DETAIL_ASYNC_ALREADY_CONSUMED = -6110,
  PROM_DETAIL_ASYNC_INVALID_TASK = -6111,
  PROM_DETAIL_ASYNC_SUBMIT_REJECTED = -6112,
  PROM_DETAIL_ASYNC_SOFTWARE_SUPPRESSED = -6113,
  PROM_DETAIL_ASYNC_FAILED = -6114,
  PROM_DETAIL_ASYNC_UNCONSUMED = -6115,
  PROM_DETAIL_INJECTED_ASYNC_POLL_FAILURE = -6116,
  PROM_DETAIL_PACKED4_PADDING_WASTE = -6201,
  PROM_DETAIL_PACKED4_SMALL_SHAPE = -6202,
  PROM_DETAIL_PACKED4_CAPABILITY_MISSING = -6203,
  PROM_DETAIL_PACKED4_FALLBACK_REQUIRED = -6204,
  PROM_DETAIL_PACKED4_MODE_BUDGET_DENIED = -6205,
  PROM_DETAIL_FP16_STRICT_FP32 = -6301,
  PROM_DETAIL_FP16_TOLERANCE_UNKNOWN = -6302,
  PROM_DETAIL_FP16_TOLERANCE_EXCEEDED = -6303,
  PROM_DETAIL_FP16_SPECIAL_VALUE = -6304,
  PROM_DETAIL_FP16_CAPABILITY_MISSING = -6305,
  PROM_DETAIL_FP16_FALLBACK_REQUIRED = -6306,
  PROM_DETAIL_FP16_NOT_TOP_UTILITY = -6307,
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

typedef struct PrometheusAsyncStatus {
  uint32_t lifecycle_state;
  uint32_t stage;
  int detail_code;
  uint32_t ready;
  uint32_t failed;
  uint32_t consumed;
  uint32_t outstanding_tasks;
} PrometheusAsyncStatus;

typedef struct PrometheusSgemmPolicyDiagnostics {
  uint32_t current_mode;
  uint32_t lookahead;
  uint32_t outstanding_depth;
  uint32_t chunk_size;
  uint32_t chunk_min;
  uint32_t chunk_max;
  uint32_t waste_budget_units;
  uint32_t pending_waste_units;
  uint32_t wasted_work_units_last;
  uint64_t wasted_work_units_total;
  uint64_t decision_count;
  uint64_t retreat_count;
  uint64_t recovery_count;
  uint64_t transition_count;
  uint64_t instability_count;
  uint64_t budget_depletion_count;
  uint64_t safe_mode_decisions;
  uint64_t aggressive_mode_decisions;
  uint64_t recovery_mode_decisions;
  uint64_t lag_early_warning_count;
  uint64_t burst_dampening_count;
  uint64_t bound_violation_count;
  uint32_t packed4_selected_layout_format;
  uint32_t packed4_tail_count_last;
  uint64_t packed4_tail_count_total;
  uint32_t packed4_padded_lane_count_last;
  uint64_t packed4_padded_lane_count_total;
  uint32_t packed4_padding_waste_permille_last;
  uint64_t packed4_mode_budget_denials;
  uint64_t packed4_row_major_check_failures;
  uint64_t packed4_selection_count;
  uint64_t packed4_fallback_reason_padding_waste;
  uint64_t packed4_fallback_reason_small_shape;
  uint64_t packed4_fallback_reason_capability_missing;
  uint64_t packed4_fallback_reason_fallback_required;
  uint64_t packed4_fallback_reason_mode_budget_denied;
  float fp16_max_absolute_error;
  float fp16_max_relative_error;
  float fp16_aggregate_error;
  uint32_t fp16_worst_case_element_index;
  float fp16_k_error_growth;
  float fp16_cancellation_risk;
  uint32_t fp16_tolerance_known;
  uint32_t fp16_tolerance_pass;
  int fp16_fallback_reason_detail;
  uint32_t fp16_selected_candidate;
  uint32_t m29_current_slot_id;
  uint32_t m29_next_slot_id;
  uint32_t m29_slot0_state;
  uint32_t m29_slot1_state;
  uint64_t m29_slot0_generation;
  uint64_t m29_slot1_generation;
  uint32_t m29_slot0_valid;
  uint32_t m29_slot1_valid;
  uint64_t m29_swap_count;
  uint64_t m29_max_wip_depth;
  uint64_t m29_overwrite_rejection_count;
  uint64_t m29_stale_buffer_rejection_count;
  uint64_t m29_shape_invalidation_count;
  uint64_t m29_layout_invalidation_count;
  uint64_t m29_capacity_invalidation_count;
  uint64_t m29_inflight_rejection_count;
  uint64_t m29_cleanup_success_count;
  int m29_failure_slot_id;
  int m29_failure_reason;
} PrometheusSgemmPolicyDiagnostics;

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
PROM_REACTOR_API int prometheus_reactor_runtime_sgemm_submit_async(void* handle,
                                                                   const float* a,
                                                                   const float* b,
                                                                   uint32_t m,
                                                                   uint32_t n,
                                                                   uint32_t k,
                                                                   int* out_task_id,
                                                                   uint32_t* out_stage,
                                                                   int* out_detail_code);
PROM_REACTOR_API int prometheus_reactor_runtime_sgemm_query_async(void* handle,
                                                                  int task_id,
                                                                  PrometheusAsyncStatus* out_status);
PROM_REACTOR_API int prometheus_reactor_runtime_sgemm_consume_async(void* handle,
                                                                    int task_id,
                                                                    float* c,
                                                                    uint32_t c_len,
                                                                    uint32_t* out_stage,
                                                                    int* out_detail_code);
PROM_REACTOR_API int prometheus_reactor_runtime_sgemm_abandon_async(void* handle, int task_id);
PROM_REACTOR_API int prometheus_reactor_runtime_sgemm_policy_diagnostics(void* handle,
                                                                         PrometheusSgemmPolicyDiagnostics* out_diag);

/* Backward-compat aliases for earlier contract drafts. */
PROM_REACTOR_API int prometheus_runtime_create(void* config, void** out_handle);
PROM_REACTOR_API int prometheus_runtime_destroy(void* handle);
PROM_REACTOR_API int prometheus_runtime_probe(void* handle, PrometheusCaps* out_caps);

#ifdef __cplusplus
}
#endif

#endif
