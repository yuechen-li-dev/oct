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
  PROM_TESTCFG_DISABLE_TRANSFER_QUEUE = 1u << 23,
  PROM_TESTCFG_FORCE_NO_DEDICATED_TRANSFER = 1u << 24,
  PROM_TESTCFG_FORCE_SHARED_TRANSFER = 1u << 25,
  PROM_TESTCFG_FAIL_TRANSFER_SUBMIT = 1u << 26,
  PROM_TESTCFG_DISABLE_SELECTOR_CACHE = 1u << 27,
  PROM_TESTCFG_P11_ARENA_FORCE_INFLIGHT = 1u << 28,
  PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS = 1u << 29,
  PROM_TESTCFG_P11_BATCH_FORCE_LANE_SIMULATED = 1u << 30,
  PROM_TESTCFG_P11_BATCH_TEST_FORCE_WRONG_RESOURCE_OWNER = 1u << 31,
};

enum {
  PROM_OCCUPANCY_VARIANT_PATH_STATUS_BASELINE = 0,
  PROM_OCCUPANCY_VARIANT_PATH_STATUS_ALIAS_OR_NOT_WIRED = 1,
  PROM_OCCUPANCY_VARIANT_PATH_STATUS_NOT_WIRED = 2,
  PROM_OCCUPANCY_VARIANT_PATH_STATUS_WIRED = 3,
};

enum {
  PROM_OCCUPANCY_VARIANT_PATH_ID_BASELINE = 0,
  PROM_OCCUPANCY_VARIANT_PATH_ID_NOT_WIRED = 1,
  PROM_OCCUPANCY_VARIANT_PATH_ID_SRT_2ACCUM_K = 2,
  PROM_OCCUPANCY_VARIANT_PATH_ID_B2X2_ROW_MAJOR_BIASED = 3,
  PROM_OCCUPANCY_VARIANT_PATH_ID_A2X4_ROW_BIASED_ACCUM8 = 4,
};

enum {
  PROM_OCCUPANCY_VARIANT_FALLBACK_NONE = 0,
  PROM_OCCUPANCY_VARIANT_FALLBACK_PATH_NOT_WIRED = 1,
  PROM_OCCUPANCY_VARIANT_FALLBACK_MC_BASELINE_STRICT_ALIAS = 2,
};

enum {
  PROM_TRANSFER_FALLBACK_NONE = 0,
  PROM_TRANSFER_FALLBACK_NO_DEDICATED_QUEUE = 1,
  PROM_TRANSFER_FALLBACK_PSEUDO_SHARED_QUEUE = 2,
  PROM_TRANSFER_FALLBACK_SMALL_SHAPE_LOW_BENEFIT = 3,
  PROM_TRANSFER_FALLBACK_SYNC_OWNERSHIP_UNSUPPORTED = 4,
  PROM_TRANSFER_FALLBACK_REQUIRED = 5,
  PROM_TRANSFER_FALLBACK_DISABLED_BY_CONFIG = 6,
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
  PROM_DETAIL_BUFFERING_PULL_LAG_LATE_STAGE_STARVATION = -6401,
  PROM_DETAIL_BUFFERING_PULL_LAG_MEMORY_EDGE_REJECTED = -6402,
  PROM_DETAIL_BUFFERING_PULL_LAG_VARIANCE_MISS = -6403,
  PROM_DETAIL_BUFFERING_PULL_LAG_COMPUTE_UNSTABLE = -6404,
  PROM_DETAIL_BUFFERING_PULL_LAG_WIP_WASTE_EXCEEDED = -6405,
  PROM_DETAIL_BUFFERING_NO_MODE_FEASIBLE = -6406,
  PROM_DETAIL_ARENA_BUDGET_REJECTED = -6501,
  PROM_DETAIL_ARENA_OWNERSHIP_REJECTED = -6502,
  PROM_DETAIL_ARENA_NAMESPACE_MISMATCH = -6503,
  PROM_DETAIL_BATCH_ZERO_WORKERS = -6601,
  PROM_DETAIL_BATCH_PLAN_INVALID = -6602,
  PROM_DETAIL_BATCH_EVENT_RING_OVERFLOW = -6603,
  PROM_DETAIL_BATCH_EXECUTION_FAILED = -6604,
  PROM_DETAIL_BATCH_RESOURCE_OWNERSHIP_VIOLATION = -6605,
  PROM_DETAIL_BATCH_COMMAND_RESOURCE_CREATE_FAILED = -6606,
  PROM_DETAIL_BATCH_COMMAND_RECORD_FAILED = -6607,
  PROM_DETAIL_BATCH_FENCE_RESET_FAILED = -6608,
  PROM_DETAIL_BATCH_FENCE_WAIT_FAILED = -6609,
  PROM_DETAIL_BATCH_QUEUE_SUBMIT_FAILED = -6610,
  PROM_DETAIL_BATCH_DEVICE_LOST = -6611,
  PROM_DETAIL_BATCH_DRAIN_TIMEOUT = -6612,
  /* Backward-compat alias used by earlier P8d tests/reports. */
  PROM_DETAIL_PATH_TILED = PROM_DETAIL_PATH_DIRECT_TILED,
};

typedef struct PrometheusSgemmBatchEntry {
  const float* a;
  const float* b;
  float* c;
  uint32_t m;
  uint32_t n;
  uint32_t k;
} PrometheusSgemmBatchEntry;

typedef struct PrometheusSgemmBatchDiagnostics {
  uint32_t last_batch_entry_count;
  uint32_t requested_workers;
  uint32_t effective_workers;
  uint32_t hardware_queue_cap;
  uint32_t memory_worker_cap;
  uint32_t worker_cap_reason;
  uint32_t partition_policy;
  uint32_t batch_state;
  uint32_t failed_entry_id;
  uint32_t failed_worker_id;
  uint32_t failure_stage;
  int32_t failure_detail;
  uint32_t failure_count;
  uint32_t first_failure_stable;
  uint32_t event_overflow_count;
  uint32_t event_drain_count;
  uint32_t output_committed;
  uint32_t plan_generation;
  uint32_t worker_judgment_count;
  uint32_t execution_mode;
  uint32_t worker_resource_mode;
  uint32_t queue_topology_classification;
  uint32_t queue_mapping_mode;
  uint32_t lane_worker_count;
  uint32_t real_worker_thread_count;
  uint32_t serialized_vulkan;
  uint32_t serialized_bridge_enter_count;
  uint32_t serialized_execution_count;
  uint32_t serialized_wait_count;
  uint32_t max_concurrent_serialized_entries;
  uint32_t hardware_parallelism_claimed;
  uint32_t resource_ownership_violation_count;
  uint32_t resource_creation_failure_count;
  uint32_t worker_active_mask;
  uint32_t worker_assigned_count[8];
  uint32_t worker_completed_count[8];
  uint32_t worker_event_count[8];
  uint32_t worker_queue_index[8];
  uint32_t worker_submit_count[8];
  uint32_t worker_wait_count[8];
  uint32_t worker_in_flight[8];
  uint32_t worker_slot_id[8];
  uint32_t worker_output_staging_id[8];
  uint32_t worker_arena_bank_id[8];
  uint32_t worker_command_pool_id[8];
  uint32_t worker_command_buffer_id[8];
  uint32_t worker_fence_id[8];
  uint32_t worker_command_pool_valid[8];
  uint32_t worker_command_buffer_valid[8];
  uint32_t worker_fence_valid[8];
  uint32_t worker_reset_count[8];
  uint32_t worker_record_count[8];
  uint32_t worker_failure_stage[8];
  int32_t worker_failure_detail[8];
  uint32_t reported_compute_queue_count;
  uint32_t independent_compute_queue_count;
  uint32_t true_multi_queue_selected;
  uint32_t hardware_parallelism_eligible;
  uint32_t serialized_fallback_reason;
  uint32_t per_worker_queue_family[8];
  uint32_t per_worker_fence_state[8];
  uint32_t queue_drain_count;
  uint32_t drain_timeout_count;
  uint32_t queue_family_ownership_handoff_count;
  uint32_t transfer_compute_sync_wait_count;
  uint32_t unsafe_to_reuse;
  uint32_t slots_per_worker_target;
  uint32_t effective_slots_per_worker;
  uint32_t total_slot_count;
  uint32_t slot_cap_reason;
  uint32_t slot_refill_count;
  uint32_t slot_full_scan_poll_count;
  uint32_t slot_attention_poll_count;
  uint32_t slot_polling_avoided_count;
  uint32_t slot_failure_count;
  uint32_t slot_drain_count;
  uint32_t slot_boundary_generation;
  uint32_t slot_dirty_mask;
  uint32_t slot_ready_mask;
  uint32_t slot_failed_mask;
  uint32_t slot_invalidated_mask;
  uint32_t slot_attention_mask;
  uint32_t slot_owner_worker_id[16];
  uint32_t slot_state[16];
  uint32_t slot_generation[16];
  uint32_t slot_entry_id[16];
  uint32_t slot_queue_id[16];
  uint32_t slot_command_resource_id[16];
  uint32_t slot_arena_id[16];
  uint32_t slot_output_staging_id[16];
  uint32_t slot_in_flight[16];
  uint32_t slot_ready[16];
  uint32_t slot_invalidated[16];
  uint32_t slot_failure_stage[16];
  int32_t slot_failure_detail[16];
  uint64_t p13_m10_lease_request_count;
  uint64_t p13_m10_lease_grant_count;
  uint64_t p13_m10_lease_deny_count;
  uint64_t p13_m10_lease_yield_count;
  uint32_t p13_m10_lease_last_state;
  uint32_t p13_m10_lease_last_deny_reason;
  uint32_t p13_m10_lookahead_requested;
  uint32_t p13_m10_lookahead_allowed;
  uint32_t p13_m10_lookahead_blocked_reason;
  uint32_t p13_m10_selected_recipe_variant;
} PrometheusSgemmBatchDiagnostics;

enum {
  PROM_BATCH_FLAG_PARTITION_CONTIGUOUS = 1u << 8,
  PROM_BATCH_FLAG_FAIL_AFTER_FIRST_SUBMIT = 1u << 9,
  /*
   * Test-only hook bits for M7 hardening coverage.
   * Layout:
   *   bits 10..13: hardware queue cap override (0 keeps runtime default cap)
   *   bits 14..15: per-worker arena bytes scale (0=64MiB, 1=32MiB, 2=16MiB, 3=8MiB)
   *   bits 16..21: worker event ring capacity override (0 keeps default 64)
   *   bit 22: force dual failure for entry 0 and entry 1 (if present)
   *   bit 23: delay entry 0 execution to force out-of-order completion in tests
   *   bits 24..31: fail-on-entry-id+1 injection (0 disables targeted failure)
   */
  PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT = 10u,
  PROM_BATCH_FLAG_TEST_HW_CAP_MASK = 0xFu << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT,
  PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT = 14u,
  PROM_BATCH_FLAG_TEST_ARENA_SCALE_MASK = 0x3u << PROM_BATCH_FLAG_TEST_ARENA_SCALE_SHIFT,
  PROM_BATCH_FLAG_TEST_EVENT_CAPACITY_SHIFT = 16u,
  PROM_BATCH_FLAG_TEST_EVENT_CAPACITY_MASK = 0x3Fu << PROM_BATCH_FLAG_TEST_EVENT_CAPACITY_SHIFT,
  PROM_BATCH_FLAG_TEST_DUAL_FAIL_FIRST_TWO = 1u << 22,
  PROM_BATCH_FLAG_TEST_DELAY_ENTRY0 = 1u << 23,
  PROM_BATCH_FLAG_TEST_FAIL_ENTRY_SHIFT = 24u,
  PROM_BATCH_FLAG_TEST_FAIL_ENTRY_MASK = 0xFFu << PROM_BATCH_FLAG_TEST_FAIL_ENTRY_SHIFT,
};

enum {
  PROM_BATCH_PARTITION_ROUND_ROBIN = 1u,
  PROM_BATCH_PARTITION_CONTIGUOUS = 2u,
};

enum {
  PROM_BATCH_SLOT_STATE_EMPTY = 0u,
  PROM_BATCH_SLOT_STATE_PREPARING = 1u,
  PROM_BATCH_SLOT_STATE_READY = 2u,
  PROM_BATCH_SLOT_STATE_IN_FLIGHT = 3u,
  PROM_BATCH_SLOT_STATE_COMPLETE = 4u,
  PROM_BATCH_SLOT_STATE_FAILED = 5u,
  PROM_BATCH_SLOT_STATE_CLEANUP = 6u,
  PROM_BATCH_SLOT_STATE_INVALIDATED = 7u,
};

enum {
  PROM_BATCH_SLOT_CAP_REASON_NONE = 0u,
  PROM_BATCH_SLOT_CAP_REASON_MEMORY_BUDGET = 1u,
};

enum {
  PROM_BATCH_STATE_PENDING = 1u,
  PROM_BATCH_STATE_RUNNING = 2u,
  PROM_BATCH_STATE_FAILING = 3u,
  PROM_BATCH_STATE_DRAINING = 4u,
  PROM_BATCH_STATE_FAILED = 5u,
  PROM_BATCH_STATE_SUCCEEDED = 6u,
};

enum {
  PROM_BATCH_CAP_REASON_NONE = 0u,
  PROM_BATCH_CAP_REASON_HARDWARE_QUEUE = 1u,
  PROM_BATCH_CAP_REASON_MEMORY_BUDGET = 2u,
  PROM_BATCH_CAP_REASON_SINGLE_QUEUE_CONSERVATIVE = 3u,
};

enum {
  PROM_BATCH_EXECUTION_SINGLE_WORKER = 1u,
  PROM_BATCH_EXECUTION_LANE_SIMULATED = 2u,
  PROM_BATCH_EXECUTION_REAL_THREADS_SERIALIZED_VULKAN = 3u,
  PROM_BATCH_EXECUTION_REAL_THREADS_TRUE_MULTI_QUEUE = 4u,
};

enum {
  PROM_BATCH_WORKER_RESOURCE_DEDICATED = 1u,
  PROM_BATCH_WORKER_RESOURCE_SHARED = 2u,
  PROM_BATCH_WORKER_RESOURCE_SIMULATED = 3u,
  PROM_BATCH_WORKER_RESOURCE_PHYSICAL_PER_WORKER = PROM_BATCH_WORKER_RESOURCE_DEDICATED,
  PROM_BATCH_WORKER_RESOURCE_MODE_SHARED = PROM_BATCH_WORKER_RESOURCE_SHARED,
  PROM_BATCH_WORKER_RESOURCE_MODE_SIMULATED_PER_WORKER = PROM_BATCH_WORKER_RESOURCE_SIMULATED,
};

enum {
  PROM_BATCH_QUEUE_TOPOLOGY_SINGLE_QUEUE = 1u,
  PROM_BATCH_QUEUE_TOPOLOGY_PSEUDO_SHARED = 2u,
  PROM_BATCH_QUEUE_TOPOLOGY_PARALLEL_ELIGIBLE = 3u,
  PROM_BATCH_QUEUE_TOPOLOGY_SEPARATE_COMPUTE_FAMILIES = 4u,
  PROM_BATCH_QUEUE_TOPOLOGY_COMPUTE_PLUS_TRANSFER = 5u,
  PROM_BATCH_QUEUE_TOPOLOGY_MEMORY_CAPPED = 6u,
  PROM_BATCH_QUEUE_TOPOLOGY_FORCED_SERIALIZED = 7u,
};

enum {
  PROM_BATCH_QUEUE_MAPPING_SINGLE_QUEUE_SERIALIZED = 1u,
  PROM_BATCH_QUEUE_MAPPING_PER_WORKER_MAPPED_SERIALIZED = 2u,
  PROM_BATCH_QUEUE_MAPPING_PARALLEL_ELIGIBLE_DISABLED = 3u,
  PROM_BATCH_QUEUE_MAPPING_PARALLEL_STATIC_PARTITION = 4u,
};

enum {
  PROM_BATCH_FALLBACK_REASON_NONE = 0u,
  PROM_BATCH_FALLBACK_REASON_BASELINE_SERIALIZED = 1u,
  PROM_BATCH_FALLBACK_REASON_INDEPENDENT_QUEUE_LT_2 = 2u,
  PROM_BATCH_FALLBACK_REASON_EFFECTIVE_WORKERS_LT_2 = 3u,
  PROM_BATCH_FALLBACK_REASON_COMMAND_RESOURCES_INVALID = 4u,
  PROM_BATCH_FALLBACK_REASON_FENCES_INVALID = 5u,
  PROM_BATCH_FALLBACK_REASON_QUEUE_MAPPING_INVALID = 6u,
  PROM_BATCH_FALLBACK_REASON_MEMORY_CAP = 7u,
  PROM_BATCH_FALLBACK_REASON_PSEUDO_SHARED = 8u,
  PROM_BATCH_FALLBACK_REASON_FORCED_SERIALIZED = 9u,
  PROM_BATCH_FALLBACK_REASON_QUEUE_FAMILY_OWNERSHIP_HANDOFF_REQUIRED = 10u,
};

enum {
  PROM_SGEMM_GPU_TIMING_FAILURE_NONE = 0u,
  PROM_SGEMM_GPU_TIMING_FAILURE_UNSUPPORTED = 1u,
  PROM_SGEMM_GPU_TIMING_FAILURE_QUERY_POOL_UNAVAILABLE = 2u,
  PROM_SGEMM_GPU_TIMING_FAILURE_INVALID_PERIOD = 3u,
  PROM_SGEMM_GPU_TIMING_FAILURE_QUERY_UNAVAILABLE = 4u,
  PROM_SGEMM_GPU_TIMING_FAILURE_INVALID_ORDER = 5u,
  PROM_SGEMM_GPU_TIMING_FAILURE_COMMAND_FAILED = 6u,
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
  uint64_t m14_a_invalidation_count;
  uint64_t m14_b_invalidation_count;
  uint64_t m14_c_invalidation_count;
  uint64_t m14_a_reuse_count;
  uint64_t m14_b_reuse_count;
  uint64_t m14_c_reuse_count;
  uint64_t m14_false_invalidation_avoided_count;
  uint64_t m14_capacity_invalidation_count;
  uint64_t m14_layout_precision_invalidation_count;
  uint32_t m14_a_last_invalidation_reason;
  uint32_t m14_b_last_invalidation_reason;
  uint32_t m14_c_last_invalidation_reason;
  uint64_t p11_m3_arena_a_capacity_bytes;
  uint64_t p11_m3_arena_b_capacity_bytes;
  uint64_t p11_m3_arena_c_capacity_bytes;
  uint64_t p11_m3_arena_upload_capacity_bytes;
  uint64_t p11_m3_arena_a_required_bytes;
  uint64_t p11_m3_arena_b_required_bytes;
  uint64_t p11_m3_arena_c_required_bytes;
  uint64_t p11_m3_arena_upload_required_bytes;
  uint64_t p11_m3_arena_a_generation;
  uint64_t p11_m3_arena_b_generation;
  uint64_t p11_m3_arena_c_generation;
  uint64_t p11_m3_arena_upload_generation;
  uint64_t p11_m3_arena_a_reuse_count;
  uint64_t p11_m3_arena_b_reuse_count;
  uint64_t p11_m3_arena_c_reuse_count;
  uint64_t p11_m3_arena_upload_reuse_count;
  uint64_t p11_m3_arena_a_grow_count;
  uint64_t p11_m3_arena_b_grow_count;
  uint64_t p11_m3_arena_c_grow_count;
  uint64_t p11_m3_arena_upload_grow_count;
  uint64_t p11_m3_arena_a_shrink_count;
  uint64_t p11_m3_arena_b_shrink_count;
  uint64_t p11_m3_arena_c_shrink_count;
  uint64_t p11_m3_arena_upload_shrink_count;
  uint64_t p11_m3_arena_a_rebuild_count;
  uint64_t p11_m3_arena_b_rebuild_count;
  uint64_t p11_m3_arena_c_rebuild_count;
  uint64_t p11_m3_arena_upload_rebuild_count;
  uint64_t p11_m3_arena_grow_count;
  uint64_t p11_m3_arena_shrink_count;
  uint64_t p11_m3_arena_rebuild_count;
  uint64_t p11_m3_arena_budget_rejection_count;
  uint64_t p11_m3_arena_ownership_rejection_count;
  uint64_t p11_m3_arena_namespace_rejection_count;
  uint64_t p11_m3_arena_total_committed_bytes;
  uint64_t p11_m3_arena_projected_committed_bytes;
  uint64_t p11_m3_arena_budget_limit_bytes;
  int32_t p11_m3_arena_last_failure_reason;
  uint64_t m29_inflight_rejection_count;
  uint64_t m29_cleanup_success_count;
  int m29_failure_slot_id;
  int m29_failure_reason;
  uint32_t m31_transfer_queue_used;
  uint32_t m31_transfer_policy_selected;
  uint32_t m31_dedicated_transfer_available;
  uint32_t m31_transfer_queue_family_index;
  uint32_t m31_compute_queue_family_index;
  uint32_t m31_queue_families_differ;
  uint64_t m31_queue_family_handoff_count;
  uint64_t m31_transfer_compute_wait_count;
  uint32_t m31_transfer_fallback_reason;
  int m31_transfer_failure_slot_id;
  int m31_transfer_failure_reason;
  uint32_t m31_upload_policy_marker;
  uint32_t m31_async_transfer_complete;
  uint32_t m35_selected_buffering_mode;
  uint32_t m35_fixed_feasible;
  uint32_t m35_pull_lag_feasible;
  uint32_t m35_serial_feasible;
  uint32_t m35_fixed_score;
  uint32_t m35_pull_lag_score;
  uint32_t m35_serial_score;
  uint32_t m35_fixed_rejected;
  uint32_t m35_pull_lag_rejected;
  uint32_t m35_serial_rejected;
  uint32_t m35_reason_code;
  uint32_t m35_final_reason_code;
  uint32_t m35_fixed_double_rejection_reason;
  uint32_t m35_pull_lag_rejection_reason;
  uint32_t m35_serial_jit_rejection_reason;
  uint32_t m35_transition_count;
  uint32_t m35_rejection_count;
  uint64_t m35_memory_budget_slots_permille;
  uint64_t m35_required_fixed_slots_permille;
  uint64_t m35_required_pull_lag_slots_permille;
  uint64_t m35_required_serial_slots_permille;
  int64_t m35_fixed_double_headroom_slots_permille;
  int64_t m35_pull_lag_headroom_slots_permille;
  int64_t m35_serial_jit_headroom_slots_permille;
  uint64_t m35_budget_rejection_count;
  /* Proxy-unit fields below are structural work-unit diagnostics, not wall-clock timing. */
  uint64_t m35_pull_lag_predicted_demand_proxy_units;
  uint64_t m35_pull_lag_transfer_lead_proxy_units;
  uint64_t m35_pull_lag_safety_margin_proxy_units;
  uint64_t m35_pull_lag_stage_start_proxy_units;
  uint64_t m35_pull_lag_stage_complete_proxy_units;
  uint64_t m35_pull_lag_late_stage_count;
  uint64_t m35_pull_lag_early_stage_count;
  uint64_t m35_pull_lag_starvation_proxy_units;
  uint64_t m35_pull_lag_ready_unused_proxy_units;
  uint64_t m35_pull_lag_wip_waste_exceeded_count;
  uint64_t m35_serial_active_slot_count;
  uint64_t m35_serial_wip_depth;
  uint64_t m35_serial_sequential_step_count;
  uint64_t m35_serial_busy_retry_count;
  uint64_t m35_serial_failure_cleanup_count;
  uint32_t p13_m2_occupancy_device_band;
  uint32_t p13_m2_occupancy_shape_class;
  uint32_t p13_m2_occupancy_selected_variant;
  uint32_t p13_m2_occupancy_unclamped_variant;
  uint32_t p13_m2_occupancy_clamp_reason;
  uint32_t p13_m2_occupancy_override_used;
  uint32_t p13_m2_occupancy_fallback_used;
  uint32_t p13_m16b1_requested_occupancy_variant;
  uint32_t p13_m16b1_executed_occupancy_variant;
  uint32_t p13_m16b1_variant_registered;
  uint32_t p13_m16b1_variant_benchmark_enabled;
  uint32_t p13_m16b1_variant_dvt_validated;
  uint32_t p13_m16b1_variant_pvt_validated;
  uint32_t p13_m16b1_variant_production_eligible;
  uint32_t p13_m16b1_variant_dispatch_enabled;
  uint32_t p13_m16b1_variant_path_status;
  uint32_t p13_m16b1_variant_path_id;
  uint32_t p13_m16b1_fallback_reason;
  uint32_t p13_m5_timestamp_available;
  uint32_t p13_m5_last_gpu_timing_valid;
  uint32_t p13_m5_last_gpu_timing_failure_reason;
  uint64_t p13_m5_last_gpu_duration_ns;
  uint32_t p14_m8_filter_evidence_valid;
  double p14_m8_raw_gpu_duration_ns;
  double p14_m8_filtered_gpu_duration_ns;
  double p14_m8_filter_residual;
  double p14_m8_filter_confidence;
  uint32_t p14_m8_filter_selected_kind;
  uint32_t p14_m8_filter_previous_kind;
  uint32_t p14_m8_filter_switched;
  uint32_t p14_m8_filter_warmup;
  uint32_t p14_m8_filter_held_by_min_commit;
  uint32_t p14_m8_filter_held_by_margin;
  uint32_t p14_m8_filter_held_by_confidence;
  uint32_t p14_m8_filter_warm_transferred;
  uint32_t p14_m8_filter_sample_count;
  uint32_t p14_m8_filter_outlier_count;
  uint32_t p15_predictor_valid;
  double p15_prediction_confidence;
  uint32_t p15_lookahead_depth;
  uint32_t p15_prediction_issued;
  uint32_t p15_prediction_matured;
  uint64_t p15_predicted_ready_tick;
  uint64_t p15_actual_ready_tick;
  int64_t p15_prediction_error_ticks;
  uint64_t p15_correction_count;
  uint32_t p15_correction_action;
  uint32_t p15_fallback_active;
  uint32_t p15_fallback_reason;
  uint32_t p15_future_lease_valid;
  uint64_t p15_future_lease_request_id;
  uint32_t p15_future_lease_state;
  uint64_t p15_future_lease_target_tick;
  double p15_future_lease_confidence;
  uint32_t p15_future_lease_reason;
  uint32_t p15_reservation_valid;
  uint64_t p15_reservation_request_id;
  uint32_t p15_reservation_state;
  uint32_t p15_reservation_reserved;
  uint32_t p15_reservation_denied;
  uint32_t p15_reservation_cancelled;
  uint32_t p15_reservation_matured;
  uint32_t p15_reservation_expired;
  uint32_t p15_reservation_reason;
  uint32_t p15_reservation_active_count;
  uint32_t p15_prestage_valid;
  uint32_t p15_prestage_state;
  uint32_t p15_prestage_allowed;
  uint32_t p15_prestage_submitted;
  uint32_t p15_prestage_block_reasons;
  double p15_prestage_confidence;
  uint64_t p15_prestage_target_tick;
  uint32_t p15_prestage_lead_ticks;
  double p15_prestage_cost_estimate;
  double p15_prestage_benefit_estimate;
  uint32_t p13_m5_timestamp_valid_bits;
  float p13_m5_timestamp_period_ns;
  uint32_t p10_m4_last_slot_event_kind;
  uint32_t p10_m4_last_slot_event_slot_id;
  int32_t p10_m4_last_slot_event_reason;
  uint32_t p10_m4_last_commit_dirty_slot_mask;
  uint64_t p10_m16_slot_readiness_boundary_generation;
  uint32_t p10_m16_slot_readiness_dirty_slot_mask;
  uint32_t p10_m16_slot_readiness_ready_slot_mask;
  uint32_t p10_m16_slot_readiness_failed_slot_mask;
  uint32_t p10_m16_slot_readiness_invalidated_slot_mask;
  uint32_t p10_m16_slot_readiness_attention_slot_mask;
  uint32_t p10_m16_slot_readiness_overflow_spill_count;
  uint64_t p10_m16_slot_readiness_duplicate_ready_event_count;
  uint64_t p10_m16_slot_readiness_empty_boundary_commit_count;
  uint32_t p10_m13_m35_selector_cache_enabled;
  uint32_t p10_m13_m35_selector_cache_valid;
  uint64_t p10_m13_m35_selector_reuse_count;
  uint64_t p10_m13_m35_selector_recompute_count;
  uint64_t p10_m13_m35_selector_invalidation_count;
  uint64_t p10_m13_m35_selector_last_dirty_dependency_mask;
  uint64_t p10_m13_m35_selector_last_visible_generation;
  uint32_t p10_m13_m35_selector_last_decision_reused;
  uint32_t p10_m13_transfer_selector_cache_enabled;
  uint32_t p10_m13_transfer_selector_cache_valid;
  uint64_t p10_m13_transfer_selector_reuse_count;
  uint64_t p10_m13_transfer_selector_recompute_count;
  uint64_t p10_m13_transfer_selector_invalidation_count;
  uint64_t p10_m13_transfer_selector_last_dirty_dependency_mask;
  uint64_t p10_m13_transfer_selector_last_visible_generation;
  uint32_t p10_m13_transfer_selector_last_decision_reused;
  uint32_t p10_m15_layout_precision_selector_cache_enabled;
  uint32_t p10_m15_layout_precision_selector_cache_valid;
  uint64_t p10_m15_layout_precision_selector_reuse_count;
  uint64_t p10_m15_layout_precision_selector_recompute_count;
  uint64_t p10_m15_layout_precision_selector_invalidation_count;
  uint64_t p10_m15_layout_precision_selector_last_dirty_dependency_mask;
  uint64_t p10_m15_layout_precision_selector_last_visible_generation;
  uint32_t p10_m15_layout_precision_selector_last_decision_reused;
  uint64_t p13_m10_lease_request_count;
  uint64_t p13_m10_lease_grant_count;
  uint64_t p13_m10_lease_deny_count;
  uint64_t p13_m10_lease_yield_count;
  uint32_t p13_m10_lease_last_state;
  uint32_t p13_m10_lease_last_deny_reason;
  uint32_t p13_m10_lookahead_requested;
  uint32_t p13_m10_lookahead_allowed;
  uint32_t p13_m10_lookahead_blocked_reason;
  uint32_t p13_m10_selected_recipe_variant;
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
PROM_REACTOR_API int prometheus_reactor_runtime_sgemm_benchmark_variant(void* handle,
                                                                         const float* a,
                                                                         const float* b,
                                                                         float* c,
                                                                         uint32_t m,
                                                                         uint32_t n,
                                                                         uint32_t k,
                                                                         uint32_t requested_variant,
                                                                         uint32_t* out_stage,
                                                                         int* out_detail_code);
PROM_REACTOR_API int prometheus_reactor_runtime_sgemm_batch(void* handle,
                                                            const PrometheusSgemmBatchEntry* entries,
                                                            uint32_t entry_count,
                                                            uint32_t flags,
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
PROM_REACTOR_API int prometheus_reactor_runtime_sgemm_batch_diagnostics(void* handle,
                                                                        PrometheusSgemmBatchDiagnostics* out_diag);

/* Backward-compat aliases for earlier contract drafts. */
PROM_REACTOR_API int prometheus_runtime_create(void* config, void** out_handle);
PROM_REACTOR_API int prometheus_runtime_destroy(void* handle);
PROM_REACTOR_API int prometheus_runtime_probe(void* handle, PrometheusCaps* out_caps);

#ifdef __cplusplus
}
#endif

#endif
