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

/* M30a keeps public task state distinct from physical submission ownership. */
enum {
  PROM_ASYNC_FAILURE_NONE = 0,
  PROM_ASYNC_FAILURE_PRE_SUBMIT = 1,
  PROM_ASYNC_FAILURE_OBSERVATION = 2,
  PROM_ASYNC_FAILURE_QUERY = 3,
  PROM_ASYNC_FAILURE_SUBMISSION = 4,
  PROM_ASYNC_FAILURE_DEVICE_LOST = 5,
};

enum {
  PROM_ASYNC_PHYSICAL_EMPTY = 0,
  PROM_ASYNC_PHYSICAL_PREPARING = 1,
  PROM_ASYNC_PHYSICAL_RECORDED = 2,
  PROM_ASYNC_PHYSICAL_SUBMITTED = 3,
  PROM_ASYNC_PHYSICAL_COMPLETE = 4,
  PROM_ASYNC_PHYSICAL_READY = 5,
  PROM_ASYNC_PHYSICAL_FAILED = 6,
  PROM_ASYNC_PHYSICAL_QUARANTINED = 7,
  PROM_ASYNC_PHYSICAL_FAILED_FATAL = 8,
};

typedef enum prom_p15_shadow_feedforward_block_reason {
  PROM_P15_SHADOW_FEEDFORWARD_BLOCK_NONE = 0,
  PROM_P15_SHADOW_FEEDFORWARD_BLOCK_DISABLED = 1,
  PROM_P15_SHADOW_FEEDFORWARD_BLOCK_NOT_HEALTHY = 2,
  PROM_P15_SHADOW_FEEDFORWARD_BLOCK_MARGIN_FAILED = 3,
  PROM_P15_SHADOW_FEEDFORWARD_BLOCK_REASON_BINDING = 4,
  PROM_P15_SHADOW_FEEDFORWARD_BLOCK_NO_MATURED_RESERVATION = 5,
  PROM_P15_SHADOW_FEEDFORWARD_BLOCK_SHAPE_MISMATCH = 6,
  PROM_P15_SHADOW_FEEDFORWARD_BLOCK_VARIANT_MISMATCH = 7,
  PROM_P15_SHADOW_FEEDFORWARD_BLOCK_CAPABILITY_MISMATCH = 8,
  PROM_P15_SHADOW_FEEDFORWARD_BLOCK_STALE_RESERVATION = 9,
  PROM_P15_SHADOW_FEEDFORWARD_BLOCK_CANCELLED_RESERVATION = 10,
  PROM_P15_SHADOW_FEEDFORWARD_BLOCK_ALREADY_CONSUMED = 11,
  PROM_P15_SHADOW_FEEDFORWARD_BLOCK_FALLBACK_REQUIRED = 12,
  PROM_P15_SHADOW_FEEDFORWARD_BLOCK_RESERVATION_NOT_READY = 13
} prom_p15_shadow_feedforward_block_reason;

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
  /* Retired P11 batch controls. Numeric tombstones remain for test-config ABI
     compatibility; no production or test execution path reads them. */
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
  PROM_OCCUPANCY_VARIANT_PATH_ID_MEMORY_CONSERVATIVE = 5,
  PROM_OCCUPANCY_VARIANT_PATH_ID_SDSL_SCALAR_PLUS = 6,
  PROM_OCCUPANCY_VARIANT_PATH_ID_SDSL_TILE16X16_SHARED_FP32 = 7,
  PROM_OCCUPANCY_VARIANT_PATH_ID_SDSL_REG2X2_TILE16X16_FP32 = 8,
  PROM_OCCUPANCY_VARIANT_PATH_ID_SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32 = 9,
  PROM_OCCUPANCY_VARIANT_PATH_ID_SDSL_REG2X2_TILE16X16_FLOWBOARD_FP32 = 10,
  PROM_OCCUPANCY_VARIANT_PATH_ID_SDSL_REG2X2_TILE16X16_DERIVE_FP32 = 11,
};

/* Separate from the saturated legacy test_flags word.  These are narrow,
   async-only simulation controls and never cause invalid Vulkan calls. */
enum {
  PROM_ASYNC_TESTCFG_FAIL_QUERY_RESULT = 1u << 0,
  PROM_ASYNC_TESTCFG_DEVICE_LOST_AFTER_SUBMIT = 1u << 1,
};

enum {
  PROM_REDUCTION_TESTCFG_FAIL_COMMAND_RECORD = 1u << 0,
  PROM_REDUCTION_TESTCFG_FAIL_QUEUE_SUBMIT = 1u << 1,
  PROM_REDUCTION_TESTCFG_FAIL_COMPLETION_OBSERVATION = 1u << 2,
  PROM_REDUCTION_TESTCFG_TEMPORARY_UNDERSIZED = 1u << 3,
  PROM_REDUCTION_TESTCFG_MALFORMED_STAGE_METADATA = 1u << 4,
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
  PROM_SGEMM_FORCE_DIRECT_REASON_NONE = 0,
  PROM_SGEMM_FORCE_DIRECT_REASON_EXPLICIT_OVERRIDE = 1,
  PROM_SGEMM_FORCE_DIRECT_REASON_SAFE_CONCRETE_HAZARD = 2,
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
  /* M30: public async admission is intentionally non-blocking. */
  PROM_DETAIL_ASYNC_QUEUE_FULL = -6117,
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
  /* R2d: a known legacy batch option is not a production execution option. */
  PROM_DETAIL_BATCH_UNSUPPORTED_OPTION = -6613,
  /* Backward-compat alias used by earlier P8d tests/reports. */
  PROM_DETAIL_PATH_TILED = PROM_DETAIL_PATH_DIRECT_TILED,
};

enum {
  PROM_SHADOW_STATE_UNKNOWN = 0,
  PROM_SHADOW_STATE_IDLE = 1,
  PROM_SHADOW_STATE_FORECAST_ISSUED = 2,
  PROM_SHADOW_STATE_FUTURE_LEASE_REQUESTED = 3,
  PROM_SHADOW_STATE_RESERVED = 4,
  PROM_SHADOW_STATE_PRESTAGE_ELIGIBLE = 5,
  PROM_SHADOW_STATE_PREDICTED_READY = 6,
  PROM_SHADOW_STATE_MATURED = 7,
  PROM_SHADOW_STATE_CANCELLED = 8,
  PROM_SHADOW_STATE_STALE = 9,
  PROM_SHADOW_STATE_FALLBACK = 10,
};

enum {
  PROM_SHADOW_MISMATCH_NONE = 0,
  PROM_SHADOW_MISMATCH_MATCH = 1,
  PROM_SHADOW_MISMATCH_LATE = 2,
  PROM_SHADOW_MISMATCH_EARLY = 3,
  PROM_SHADOW_MISMATCH_PHYSICAL_NOT_READY = 4,
  PROM_SHADOW_MISMATCH_SHADOW_NOT_READY = 5,
  PROM_SHADOW_MISMATCH_CANCELLED = 6,
  PROM_SHADOW_MISMATCH_STALE = 7,
  PROM_SHADOW_MISMATCH_FALLBACK = 8,
  PROM_SHADOW_MISMATCH_HARD_GATE = 9,
  PROM_SHADOW_MISMATCH_INVALID_PREDICTION = 10,
};


typedef struct PrometheusComplex32 {
  float real;
  float imag;
} PrometheusComplex32;

enum {
  PROM_FFT_FLAG_FORWARD = 1u << 0,
  PROM_FFT_FLAG_INVERSE = 1u << 1,
  PROM_FFT_FLAG_INVERSE_NORMALIZE = 1u << 2,
  PROM_FFT_FLAG_BENCHMARK_ALLOW_NON_PRODUCTION_PATH = 1u << 3,
};

enum {
  PROM_FFT_PATH_NONE = 0,
  PROM_FFT_PATH_UNAVAILABLE = 1,
  PROM_FFT_PATH_CPU_ORACLE_RESERVED = 2,
  PROM_FFT_PATH_VULKAN_RADIX2_RESERVED = 3,
};

enum {
  PROM_FFT_PATH_STATUS_UNAVAILABLE = 0,
  PROM_FFT_PATH_STATUS_REGISTERED = 1,
  PROM_FFT_PATH_STATUS_BENCHMARK_ENABLED = 2,
  PROM_FFT_PATH_STATUS_PRODUCTION_ENABLED = 3,
};

enum {
  PROM_FFT_BENCHMARK_VARIANT_NONE = 0,
  PROM_FFT_BENCHMARK_VARIANT_RADIX2 = 2,
};

enum {
  PROM_FFT_DETAIL_UNAVAILABLE = -6701,
  PROM_FFT_DETAIL_INVALID_REQUEST = -6702,
  PROM_FFT_DETAIL_NULL_INPUT = -6703,
  PROM_FFT_DETAIL_NULL_OUTPUT = -6704,
  PROM_FFT_DETAIL_ZERO_ELEMENT_COUNT = -6705,
  PROM_FFT_DETAIL_NON_POWER_OF_TWO = -6706,
  PROM_FFT_DETAIL_INVALID_DIRECTION_FLAGS = -6707,
  PROM_FFT_DETAIL_INVALID_STRIDE = -6708,
  PROM_FFT_DETAIL_SIZE_OVERFLOW = -6709,
  PROM_FFT_DETAIL_ZERO_BATCH_COUNT = -6710,
  PROM_FFT_DETAIL_INVERSE_NORMALIZE_REQUIRES_INVERSE = -6711,
};

/* M39b is deliberately row-wise.  Arbitrary axes, dynamic shapes, and
   non-FP32 storage are not part of this ABI. */
enum {
  PROM_REDUCTION_OPERATION_SUM = 1u,
  PROM_REDUCTION_OPERATION_MAX = 2u,
  PROM_REDUCTION_OPERATION_SOFTMAX = 3u,
};

enum {
  PROM_REDUCTION_FINALIZATION_NONE = 0u,
  PROM_REDUCTION_FINALIZATION_STABLE_SOFTMAX = 1u,
};

enum {
  PROM_REDUCTION_STRATEGY_AUTO = 0u,
  PROM_REDUCTION_STRATEGY_FUSED_SINGLE_WORKGROUP = 1u,
  PROM_REDUCTION_STRATEGY_COMPOSED = 2u,
  PROM_REDUCTION_STRATEGY_PACKED_SHORT_ROWS = 3u,
};

enum {
  PROM_REDUCTION_STAGE_ROW_SUM = 1u,
  PROM_REDUCTION_STAGE_ROW_MAX = 2u,
  PROM_REDUCTION_STAGE_SOFTMAX_EXP_SUM = 3u,
  PROM_REDUCTION_STAGE_SOFTMAX_NORMALIZE = 4u,
  PROM_REDUCTION_STAGE_SOFTMAX_FUSED = 5u,
  PROM_REDUCTION_STAGE_ROW_SUM_PACKED_SHORT = 6u,
  PROM_REDUCTION_STAGE_SOFTMAX_PACKED_SHORT = 7u,
};

enum {
  PROM_REDUCTION_TEMPORARY_NONE = 0u,
  PROM_REDUCTION_TEMPORARY_PARTIALS = 1u,
  PROM_REDUCTION_TEMPORARY_ROW_MAX = 2u,
  PROM_REDUCTION_TEMPORARY_ROW_SUM = 3u,
};

enum {
  PROM_REDUCTION_FLAG_FORCE_FUSED = 1u << 0u,
  PROM_REDUCTION_FLAG_FORCE_COMPOSED = 1u << 1u,
};

enum {
  PROM_REDUCTION_DETAIL_INVALID_REQUEST = -6801,
  PROM_REDUCTION_DETAIL_NULL_INPUT = -6802,
  PROM_REDUCTION_DETAIL_NULL_OUTPUT = -6803,
  PROM_REDUCTION_DETAIL_ZERO_ROW_COUNT = -6804,
  PROM_REDUCTION_DETAIL_ZERO_ROW_WIDTH = -6805,
  PROM_REDUCTION_DETAIL_UNSUPPORTED_OPERATION = -6806,
  PROM_REDUCTION_DETAIL_INVALID_FINALIZATION = -6807,
  PROM_REDUCTION_DETAIL_SIZE_OVERFLOW = -6808,
  PROM_REDUCTION_DETAIL_INPUT_SIZE_MISMATCH = -6809,
  PROM_REDUCTION_DETAIL_OUTPUT_SIZE_MISMATCH = -6810,
  PROM_REDUCTION_DETAIL_ROW_LIMIT = -6811,
  PROM_REDUCTION_DETAIL_WIDTH_LIMIT = -6812,
  PROM_REDUCTION_DETAIL_ELEMENT_LIMIT = -6813,
  PROM_REDUCTION_DETAIL_NONFINITE_INPUT = -6814,
  PROM_REDUCTION_DETAIL_UNSUPPORTED_STRATEGY = -6815,
  PROM_REDUCTION_DETAIL_MALFORMED_PLAN = -6816,
  PROM_REDUCTION_DETAIL_TEMPORARY_UNDERSIZED = -6817,
  PROM_REDUCTION_DETAIL_RESOURCE_CREATE_FAILED = -6818,
  PROM_REDUCTION_DETAIL_PIPELINE_CREATE_FAILED = -6819,
  PROM_REDUCTION_DETAIL_COMMAND_RECORD_FAILED = -6820,
  PROM_REDUCTION_DETAIL_QUEUE_SUBMIT_FAILED = -6821,
  PROM_REDUCTION_DETAIL_COMPLETION_UNCERTAIN = -6822,
  PROM_REDUCTION_DETAIL_QUERY_FAILED = -6823,
  PROM_REDUCTION_DETAIL_READBACK_FAILED = -6824,
  PROM_REDUCTION_DETAIL_RUNTIME_UNAVAILABLE = -6825,
};

enum {
  PROM_REDUCTION_SHADER_ROW_SUM = 16u,
  PROM_REDUCTION_SHADER_ROW_MAX = 17u,
  PROM_REDUCTION_SHADER_SOFTMAX_EXP_SUM = 18u,
  PROM_REDUCTION_SHADER_SOFTMAX_NORMALIZE = 19u,
  PROM_REDUCTION_SHADER_SOFTMAX_FUSED = 20u,
  PROM_REDUCTION_SHADER_ROW_SUM_PACKED_SHORT = 21u,
  PROM_REDUCTION_SHADER_SOFTMAX_PACKED_SHORT = 22u,
};

enum {
  PROM_REDUCTION_IMPLEMENTATION_ROW_SUM = 1001u,
  PROM_REDUCTION_IMPLEMENTATION_ROW_MAX = 1002u,
  PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_EXP_SUM = 1003u,
  PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_NORMALIZE = 1004u,
  PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_FUSED = 1005u,
  PROM_REDUCTION_IMPLEMENTATION_ROW_SUM_PACKED_SHORT = 1006u,
  PROM_REDUCTION_IMPLEMENTATION_SOFTMAX_PACKED_SHORT = 1007u,
};

#define PROM_REDUCTION_MAX_STAGES 8u
#define PROM_REDUCTION_MAX_ROWS 1024u
#define PROM_REDUCTION_MAX_ELEMENTS_PER_ROW 1048576u
#define PROM_REDUCTION_MAX_TOTAL_ELEMENTS 16777216u
#define PROM_REDUCTION_LOCAL_SIZE 256u
#define PROM_REDUCTION_ELEMENTS_PER_PARTIAL 1024u
#define PROM_REDUCTION_SINGLE_STAGE_THRESHOLD 1024u
#define PROM_REDUCTION_PACKED_SHORT_WIDTH_MAX 128u
#define PROM_REDUCTION_PACKED_SHORT_SUM_MIN_ROWS 512u
#define PROM_REDUCTION_PACKED_SHORT_SUM_WIDE_MIN_ROWS 1024u
#define PROM_REDUCTION_PACKED_SHORT_ROWS_PER_GROUP 8u
#define PROM_REDUCTION_PACKED_SHORT_LANES_PER_ROW 32u

typedef struct PrometheusReductionRequest {
  uint32_t struct_size;
  const float* input;
  float* output;
  uint32_t row_count;
  uint32_t elements_per_row;
  uint64_t input_element_count;
  uint64_t output_element_count;
  uint32_t operation;
  uint32_t finalization;
  uint32_t flags;
} PrometheusReductionRequest;

typedef struct PrometheusReductionStageDispatch {
  uint32_t stage_role;
  uint32_t shader_id;
  uint32_t implementation_id;
  uint32_t groups_x;
  uint32_t groups_y;
  uint32_t groups_z;
  uint32_t input_elements_per_row;
  uint32_t output_partials_per_row;
  uint32_t temporary_role;
  uint64_t temporary_bytes_written;
} PrometheusReductionStageDispatch;

typedef struct PrometheusReductionPlan {
  uint32_t struct_size;
  uint32_t operation;
  uint32_t row_count;
  uint32_t elements_per_row;
  uint32_t strategy;
  uint32_t local_size;
  uint32_t elements_per_partial;
  uint32_t partial_count;
  uint32_t stage_count;
  uint32_t temporary_alignment_bytes;
  uint64_t temporary_bytes;
  uint64_t replay_id;
  PrometheusReductionStageDispatch stages[PROM_REDUCTION_MAX_STAGES];
} PrometheusReductionPlan;

typedef struct PrometheusReductionExecutionResult {
  uint32_t struct_size;
  uint32_t stage;
  int32_t detail_code;
  uint64_t logical_request_id;
  uint32_t physical_slot_id;
  uint32_t physical_slot_generation;
  uint32_t physical_slot_recyclable;
  uint32_t gpu_timestamp_valid;
  uint64_t gpu_duration_ns;
  uint64_t end_to_end_ns;
  uint64_t first_nonfinite_index;
  uint32_t validation_error_count_before;
  uint32_t validation_error_count_after;
  PrometheusReductionPlan plan;
} PrometheusReductionExecutionResult;

typedef struct PrometheusReductionDiagnostics {
  uint32_t struct_size;
  uint32_t initialized;
  uint32_t production_enabled;
  uint32_t experimental_enabled;
  uint32_t configured_ring_depth;
  uint32_t physical_slot_count;
  uint32_t acquire_cursor;
  uint32_t outstanding_slots;
  uint32_t quarantined_slots;
  uint64_t next_logical_request_id;
  uint64_t total_requests;
  uint64_t successful_requests;
  uint64_t logical_failure_count;
  uint64_t physical_recycle_count;
  uint64_t quarantine_count;
  uint64_t reap_count;
  uint64_t pipeline_create_count;
  uint64_t descriptor_update_count;
  uint64_t command_record_count;
  uint64_t queue_submit_count;
  uint64_t buffer_allocation_count;
  uint64_t buffer_reuse_count;
  uint64_t temporary_capacity_bytes;
  uint64_t last_replay_id;
  uint64_t last_gpu_duration_ns;
  uint64_t last_end_to_end_ns;
  uint32_t last_stage_count;
  uint32_t last_physical_slot_id;
  uint32_t last_physical_slot_generation;
  int32_t last_detail_code;
  uint32_t validation_enabled;
  uint32_t validation_error_count;
} PrometheusReductionDiagnostics;

typedef struct PrometheusReductionBenchmarkRequest {
  uint32_t struct_size;
  PrometheusReductionRequest reduction;
  uint32_t warmup_iterations;
  uint32_t measured_iterations;
} PrometheusReductionBenchmarkRequest;

typedef struct PrometheusReductionBenchmarkResult {
  uint32_t struct_size;
  uint32_t completed_iterations;
  uint32_t correctness_passed;
  uint32_t validation_passed;
  uint32_t device_lost;
  uint32_t stage_count;
  uint64_t temporary_bytes;
  uint64_t replay_id;
  uint64_t gpu_min_ns;
  uint64_t gpu_median_ns;
  uint64_t gpu_max_ns;
  uint64_t end_to_end_min_ns;
  uint64_t end_to_end_median_ns;
  uint64_t end_to_end_max_ns;
  uint32_t first_mismatch_row;
  uint32_t first_mismatch_column;
  float first_expected;
  float first_actual;
  float first_absolute_error;
  float first_relative_error;
  int32_t detail_code;
} PrometheusReductionBenchmarkResult;

typedef struct PrometheusFftRequest {
  uint32_t struct_size;
  const PrometheusComplex32* input;
  PrometheusComplex32* output;
  uint32_t element_count;
  uint32_t batch_count;
  uint32_t stride_elements;
  uint32_t flags;
} PrometheusFftRequest;

typedef struct PrometheusFftDiagnostics {
  uint32_t struct_size;
  uint32_t api_declared;
  uint32_t capability_reported;
  uint32_t production_enabled;
  uint32_t benchmark_enabled;
  uint32_t last_element_count;
  uint32_t last_batch_count;
  uint32_t last_stride_elements;
  uint32_t last_effective_stride_elements;
  uint32_t last_flags;
  uint32_t last_direction;
  uint32_t last_validation_status;
  int32_t last_failure_detail;
  uint32_t requested_path_id;
  uint32_t executed_path_id;
  uint32_t requested_radix;
  uint32_t executed_radix;
  uint32_t fallback_reason;
  uint32_t plan_pass_count;
  uint32_t ping_pong_swap_count;
  uint32_t final_output_role;
  uint64_t ping_arena_reuse_count;
  uint64_t ping_arena_grow_count;
  uint64_t pong_arena_reuse_count;
  uint64_t pong_arena_grow_count;
  uint64_t twiddle_arena_reuse_count;
  uint64_t twiddle_arena_grow_count;
  uint32_t selector_cache_valid;
  uint32_t plan_valid;
  uint32_t plan_element_count;
  uint32_t plan_log2_element_count;
  uint32_t plan_first_span;
  uint32_t plan_last_span;
  uint32_t plan_radix_mask;
  uint32_t plan_bit_reversal_required;
  uint32_t plan_first_source_role;
  uint32_t plan_first_destination_role;
  uint32_t plan_direction;
  uint32_t plan_twiddle_mode;
  uint64_t selector_cache_reuse_count;
  uint64_t selector_cache_recompute_count;
  uint64_t selector_cache_invalidation_count;
  uint64_t selector_cache_last_dependency_mask;
} PrometheusFftDiagnostics;

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
  /* PX16 M31: the batch owns logical plans; these describe the shared M29
     physical ring used to execute them.  Arrays are deliberately bounded like
     the existing diagnostics export, not an ABI-visible allocation protocol. */
  uint32_t physical_ring_depth_configured;
  uint32_t physical_ring_depth_effective;
  uint32_t current_in_flight;
  uint32_t max_in_flight;
  uint64_t total_submits;
  uint64_t total_polls;
  uint64_t total_forced_waits;
  uint64_t ring_full_count;
  uint64_t refill_count;
  uint64_t query_harvest_count;
  uint64_t quarantine_count;
  uint64_t reap_count;
  uint64_t feedback_committed_count;
  uint64_t feedback_skipped_count;
  uint32_t m31_completion_count;
  uint32_t m31_commit_count;
  uint64_t m31_submission_sequence[64];
  uint32_t m31_physical_slot_id[64];
  uint32_t m31_completion_status[64];
  uint64_t m31_gpu_duration_ns[64];
  uint32_t m31_completion_order[64];
  uint32_t m31_commit_order[64];
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
  /* ABI alias: retained P11 separate-family topology request. R2d rejects it
     through the public batch entry; it never fabricates a second compute lane. */
  PROM_BATCH_FLAG_TEST_SEPARATE_COMPUTE_FAMILY = 1u << PROM_BATCH_FLAG_TEST_HW_CAP_SHIFT,
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

enum {
  PROM_SGEMM_RESIDENT_MODE_PRODUCTION_SELECTOR = 0u,
  PROM_SGEMM_RESIDENT_MODE_EXPLICIT_VARIANT = 1u,
};

enum {
  PROM_SGEMM_RESIDENT_FLAG_VALIDATE_READBACK = 1u << 0,
  /* Internal PX16 M29 diagnostic: submit one dispatch per persistent physical slot. */
  PROM_SGEMM_RESIDENT_FLAG_M29_SUBMISSION_RING = 1u << 1,
};

typedef struct PrometheusCaps {
  uint32_t available;
  uint32_t backend_type;
  uint32_t reason_code;
} PrometheusCaps;

enum {
  PROM_VK_DEVICE_TYPE_OTHER = 0u,
  PROM_VK_DEVICE_TYPE_INTEGRATED_GPU = 1u,
  PROM_VK_DEVICE_TYPE_DISCRETE_GPU = 2u,
  PROM_VK_DEVICE_TYPE_VIRTUAL_GPU = 3u,
  PROM_VK_DEVICE_TYPE_CPU = 4u,
};

typedef struct PrometheusVulkanDeviceDiagnostics {
  char device_name[256];
  uint32_t vendor_id;
  uint32_t device_id;
  uint32_t device_type;
  uint32_t driver_version;
  uint32_t api_version;
  uint32_t software_vulkan;
  uint32_t compute_queue_family;
  uint32_t transfer_queue_family;
} PrometheusVulkanDeviceDiagnostics;

typedef struct PrometheusReactorConfig {
  uint32_t struct_size;
  uint32_t test_flags;
  uint32_t p15_shadow_canary_enabled;
  uint32_t async_test_flags;
  /* Test-only M31 override. Zero preserves the production default depth two. */
  uint32_t batch_ring_depth;
  /* M39b keeps reduction fault injection separate from the saturated SGEMM
     test flag word.  Zero is the production behavior. */
  uint32_t reduction_test_flags;
  /* Zero preserves the reduction production default depth two. */
  uint32_t reduction_ring_depth;
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

typedef struct PrometheusSgemmAsyncTaskDiagnostics {
  int32_t task_id;
  uint32_t generation;
  uint32_t lifecycle_state;
  uint32_t physical_slot_id;
  uint32_t physical_slot_generation;
  uint64_t submission_sequence;
  uint32_t timing_valid;
  uint64_t gpu_duration_ns;
  uint32_t feedback_pending;
  uint32_t feedback_committed;
  int32_t failure_detail;
  uint32_t failure_class;
  uint32_t abandoned;
  uint32_t physical_slot_state;
  uint32_t quarantined;
  uint32_t physical_completion_confirmed;
  uint32_t reap_pending;
  uint32_t reap_completed;
} PrometheusSgemmAsyncTaskDiagnostics;

typedef struct PrometheusSgemmAsyncDiagnostics {
  uint32_t task_capacity;
  uint32_t active_task_count;
  uint32_t submitted_count;
  uint32_t ready_count;
  uint32_t failed_count;
  uint32_t consumed_count;
  uint32_t abandoned_count;
  uint64_t queue_full_count;
  uint64_t stale_id_rejection_count;
  uint32_t max_in_flight;
  uint32_t feedback_pending_count;
  uint64_t feedback_committed_count;
  uint64_t feedback_skipped_count;
  uint64_t next_feedback_sequence;
  uint32_t quarantined_slot_count;
  uint64_t quarantine_event_count;
  uint64_t reap_poll_count;
  uint64_t reap_success_count;
  uint64_t reap_wait_count;
  uint64_t reap_failure_count;
  uint32_t max_quarantine_depth;
  uint32_t runtime_unsafe_to_reuse;
  PrometheusSgemmAsyncTaskDiagnostics tasks[4];
} PrometheusSgemmAsyncDiagnostics;

typedef struct PrometheusSgemmResidentBenchmarkRequest {
  uint32_t struct_size;
  const float* a;
  const float* b;
  float* c;
  uint32_t m;
  uint32_t n;
  uint32_t k;
  uint32_t mode;
  uint32_t requested_variant;
  uint32_t warmup_iterations;
  uint32_t iterations;
  /* M28 diagnostic only: 0/1 preserves serial submit/wait behavior; >1 records
     this many resident dispatches into one command buffer and waits once. */
  uint32_t diagnostic_batch_depth;
  uint32_t flags;
} PrometheusSgemmResidentBenchmarkRequest;

typedef struct PrometheusSgemmResidentBenchmarkResult {
  uint32_t struct_size;
  uint32_t resident_mode_available;
  uint32_t resident_mode_used;
  uint32_t requested_variant;
  uint32_t production_selected_variant;
  uint32_t executed_variant;
  uint32_t executed_compute_mode;
  uint32_t setup_stage;
  int32_t setup_detail_code;
  uint32_t final_stage;
  int32_t final_detail_code;
  uint32_t iterations;
  uint32_t gpu_timestamp_valid;
  uint32_t gpu_timing_failure_reason;
  uint64_t setup_wall_ns;
  uint64_t upload_once_wall_ns;
  uint64_t total_loop_wall_ns;
  uint64_t kernel_min_ns;
  uint64_t kernel_median_ns;
  uint64_t kernel_p95_ns;
  uint64_t readback_once_wall_ns;
  uint64_t validation_wall_ns;
  uint64_t dispatch_submit_wall_ns_median;
  uint64_t sync_wait_wall_ns_median;
  uint32_t diagnostic_batch_depth;
  uint32_t queue_submissions;
  uint32_t fence_waits;
  uint32_t command_buffer_recordings;
  uint32_t command_buffer_resets;
  uint32_t descriptor_updates;
  uint64_t query_result_wall_ns_median;
  /* PX16 M29 resident-ring diagnostics; zero unless the ring flag is selected. */
  uint32_t configured_ring_depth;
  uint32_t physical_slot_count;
  uint32_t max_in_flight_depth;
  uint64_t ring_poll_count;
  uint64_t ring_forced_wait_count;
  uint64_t ring_query_harvest_count;
  uint64_t ring_full_count;
  uint64_t ring_slot_recycle_count;
  uint64_t ring_failure_count;
  uint32_t ring_final_slot_state[4];
} PrometheusSgemmResidentBenchmarkResult;

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
  uint32_t p13_m16b5_force_direct_requested;
  uint32_t p13_m16b5_force_direct_applied;
  uint32_t p13_m16b5_force_direct_reason;
  uint32_t p13_m16b5_requested_path;
  uint32_t p13_m16b5_selected_path;
  uint32_t p13_m16b5_compute_mode;
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
  uint32_t p15_shadow_valid;
  uint32_t p15_shadow_state;
  uint32_t p15_shadow_physical_state;
  uint64_t p15_shadow_issued_tick;
  uint64_t p15_shadow_target_tick;
  uint64_t p15_shadow_predicted_ready_tick;
  uint64_t p15_shadow_actual_ready_tick;
  int64_t p15_shadow_arrival_error_ticks;
  double p15_shadow_prediction_confidence;
  uint32_t p15_shadow_mismatch_kind;
  uint32_t p15_shadow_matched;
  uint32_t p15_shadow_stale;
  uint32_t p15_shadow_cancelled;
  uint32_t p15_shadow_fallback;
  uint32_t p15_shadow_correction_action;
  uint64_t p15_shadow_correction_count;
  uint64_t p15_shadow_stale_count;
  uint64_t p15_shadow_miss_count;
  uint32_t p15_shadow_calibration_valid;
  uint64_t p15_shadow_calibration_sample_count;
  uint64_t p15_shadow_calibration_match_count;
  uint64_t p15_shadow_calibration_miss_count;
  uint64_t p15_shadow_calibration_early_count;
  uint64_t p15_shadow_calibration_late_count;
  uint64_t p15_shadow_calibration_stale_count;
  uint64_t p15_shadow_calibration_fallback_count;
  double p15_shadow_calibration_confidence;
  double p15_shadow_calibration_mean_abs_arrival_error_ticks;
  uint32_t p15_shadow_calibration_last_mismatch_kind;
  uint32_t p15_shadow_lookahead_state;
  uint32_t p15_shadow_authority_valid;
  uint32_t p15_shadow_authority_state;
  uint32_t p15_shadow_authority_reason;
  uint32_t p15_shadow_authority_canary_allowed;
  uint32_t p15_shadow_authority_would_act;
  uint32_t p15_shadow_authority_enabled;
  uint32_t p15_shadow_authority_recommended_lookahead_depth;
  uint32_t p15_shadow_authority_confidence_gate_passed;
  uint32_t p15_shadow_authority_sample_gate_passed;
  uint32_t p15_shadow_authority_miss_rate_gate_passed;
  uint32_t p15_shadow_authority_arrival_error_gate_passed;
  uint32_t p15_shadow_authority_lookahead_gate_passed;
  double p15_shadow_authority_match_rate;
  double p15_shadow_authority_miss_rate;
  double p15_shadow_authority_mean_abs_arrival_error_ticks;
  uint32_t p15_shadow_would_act_valid;
  uint64_t p15_shadow_would_act_evaluation_count;
  uint64_t p15_shadow_would_act_count;
  uint64_t p15_shadow_would_block_count;
  uint64_t p15_shadow_would_unknown_count;
  uint64_t p15_shadow_would_disabled_count;
  uint64_t p15_shadow_would_canary_count;
  uint64_t p15_shadow_would_healthy_count;
  uint64_t p15_shadow_would_block_low_confidence_count;
  uint64_t p15_shadow_would_block_high_miss_rate_count;
  uint64_t p15_shadow_would_block_high_arrival_error_count;
  uint64_t p15_shadow_would_block_recent_fallback_count;
  uint64_t p15_shadow_would_block_recent_stale_count;
  uint64_t p15_shadow_would_block_insufficient_samples_count;
  uint64_t p15_shadow_would_block_invalid_calibration_count;
  uint64_t p15_shadow_would_block_lookahead_disabled_count;
  uint64_t p15_shadow_would_healthy_suppressed_by_recent_fallback_count;
  uint64_t p15_shadow_would_healthy_suppressed_by_recent_stale_count;
  uint64_t p15_shadow_would_healthy_suppressed_by_arrival_error_count;
  uint32_t p15_shadow_would_last_would_act;
  uint32_t p15_shadow_would_last_reason;
  uint32_t p15_shadow_would_last_gate_state;
  uint32_t p15_shadow_would_last_recommended_lookahead_depth;
  uint32_t p15_shadow_canary_valid;
  uint32_t p15_shadow_canary_enabled;
  uint32_t p15_shadow_canary_last_action_allowed;
  uint32_t p15_shadow_canary_last_action_kind;
  uint32_t p15_shadow_canary_last_block_reason;
  uint32_t p15_shadow_canary_requested_lookahead_depth;
  uint32_t p15_shadow_canary_healthy_margin_passed;
  uint32_t p15_shadow_canary_reason_binding_passed;
  uint64_t p15_shadow_canary_evaluation_count;
  uint64_t p15_shadow_canary_action_allowed_count;
  uint64_t p15_shadow_canary_action_applied_count;
  uint64_t p15_shadow_canary_action_blocked_count;
  uint64_t p15_shadow_canary_reservation_attempt_count;
  uint64_t p15_shadow_canary_reservation_success_count;
  uint64_t p15_shadow_canary_reservation_rejected_count;
  uint64_t p15_shadow_canary_block_low_confidence_count;
  uint64_t p15_shadow_canary_block_high_miss_rate_count;
  uint64_t p15_shadow_canary_block_high_arrival_error_count;
  uint64_t p15_shadow_canary_block_recent_fallback_count;
  uint64_t p15_shadow_canary_block_recent_stale_count;
  uint64_t p15_shadow_canary_block_insufficient_samples_count;
  uint64_t p15_shadow_canary_block_disabled_count;
  uint64_t p15_shadow_canary_block_no_future_lease_count;
  uint64_t p15_shadow_canary_block_reservation_failed_count;
  uint32_t p15_shadow_feedforward_valid;
  uint32_t p15_shadow_feedforward_enabled;
  uint32_t p15_shadow_feedforward_used;
  uint32_t p15_shadow_feedforward_source;
  uint32_t p15_shadow_feedforward_reservation_present;
  uint32_t p15_shadow_feedforward_reservation_matured;
  uint32_t p15_shadow_feedforward_block_reason;
  uint32_t p15_shadow_feedforward_reserved_variant_id;
  uint32_t p15_shadow_feedforward_selected_variant_id;
  uint32_t p15_shadow_feedforward_reconciliation_match;
  uint32_t p15_shadow_feedforward_correction_action;
  uint64_t p15_shadow_feedforward_fallback_to_judgment_count;
  uint64_t p15_shadow_feedforward_reservation_consumed_count;
  uint64_t p15_shadow_feedforward_no_matured_reservation_count;
  uint64_t p15_shadow_feedforward_shape_mismatch_count;
  uint64_t p15_shadow_feedforward_variant_mismatch_count;
  uint64_t p15_shadow_feedforward_stale_reservation_count;
  uint64_t p15_shadow_feedforward_reason_binding_block_count;
  uint64_t p15_shadow_feedforward_margin_block_count;
  uint64_t p15_shadow_feedforward_dedup_block_count;
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
  uint32_t px16_m6_selector_recommended_variant;
  uint32_t px16_m6_selector_selected_variant;
  uint32_t px16_m6_requested_dispatch_variant;
  uint32_t px16_m6_executed_dispatch_variant;
  uint32_t px16_m6_requested_path;
  uint32_t px16_m6_selected_path;
  uint32_t px16_m6_executed_path;
  uint32_t px16_m6_requested_compute_mode;
  uint32_t px16_m6_selected_compute_mode;
  uint32_t px16_m6_executed_compute_mode;
  uint32_t px16_m6_force_direct_requested;
  uint32_t px16_m6_force_direct_applied;
  uint32_t px16_m6_force_direct_reason;
  uint32_t px16_m6_policy_mode;
  uint32_t px16_m6_variant_path_status;
  uint32_t px16_m6_variant_production_eligible;
  uint32_t px16_m6_variant_dispatch_enabled;
  uint32_t px16_m6_variant_dvt_validated;
  uint32_t px16_m6_variant_pvt_validated;
  uint32_t px16_m6_variant_lifecycle_telemetry_only;
  uint32_t px16_m6_p15_reservation_present;
  uint32_t px16_m6_p15_reservation_matured;
  uint32_t px16_m6_p15_reservation_consumed;
  uint32_t px16_m6_p15_reserved_variant_id;
  uint32_t px16_m6_p15_live_selected_variant_id;
  uint32_t px16_m6_p15_reconciliation_match;
  uint32_t px16_m6_p15_block_reason;
  uint32_t px16_m6_p15_correction_action;
  uint32_t px16_m6_p15_reservation_stale_or_expired;
  double px16_m6_p15_confidence_before;
  double px16_m6_p15_confidence_after;
  uint64_t px16_m8_last_upload_wall_ns;
  uint64_t px16_m8_last_pre_dispatch_wall_ns;
  uint64_t px16_m8_last_command_record_wall_ns;
  uint64_t px16_m8_last_dispatch_submit_wall_ns;
  uint64_t px16_m8_last_sync_wait_wall_ns;
  uint64_t px16_m8_last_post_sync_wall_ns;
  uint64_t px16_m8_last_readback_wall_ns;
  uint64_t px16_m8_last_post_readback_wall_ns;
  uint64_t px16_m8_last_total_wall_ns;
  uint32_t px16_m8_last_gpu_timestamp_valid;
  uint32_t px16_m8_resident_device_mode_available;
  uint32_t px16_m8_last_executed_explicit_variant_request;
  uint64_t px16_m17_last_tolerance_eval_wall_ns;
  uint32_t px16_m17_last_tolerance_eval_in_dispatch;
  uint32_t px16_m17_last_tolerance_eval_source;
} PrometheusSgemmPolicyDiagnostics;

PROM_REACTOR_API uint32_t prometheus_reactor_abi_version(void);
PROM_REACTOR_API int prometheus_reactor_runtime_create(void* config, void** out_handle);
PROM_REACTOR_API int prometheus_reactor_runtime_destroy(void* handle);
PROM_REACTOR_API int prometheus_reactor_runtime_probe(void* handle, PrometheusCaps* out_caps);
PROM_REACTOR_API int prometheus_reactor_runtime_vulkan_device_diagnostics(void* handle,
                                                                           PrometheusVulkanDeviceDiagnostics* out_diag);
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
PROM_REACTOR_API int prometheus_reactor_runtime_sgemm_resident_benchmark(
    void* handle,
    const PrometheusSgemmResidentBenchmarkRequest* request,
    PrometheusSgemmResidentBenchmarkResult* out_result);
PROM_REACTOR_API int prometheus_reactor_runtime_sgemm_batch(void* handle,
                                                            const PrometheusSgemmBatchEntry* entries,
                                                            uint32_t entry_count,
                                                            uint32_t flags,
                                                            uint32_t* out_stage,
                                                            int* out_detail_code);
/* Test-only M31 fault-injection entry. Production flags never select it. */
int prometheus_reactor_runtime_sgemm_batch_m31_test(void* handle,
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
PROM_REACTOR_API int prometheus_reactor_runtime_sgemm_async_diagnostics(void* handle,
                                                                         PrometheusSgemmAsyncDiagnostics* out_diag);
PROM_REACTOR_API int prometheus_reactor_runtime_sgemm_consume_async(void* handle,
                                                                    int task_id,
                                                                    float* c,
                                                                    uint32_t c_len,
                                                                    uint32_t* out_stage,
                                                                    int* out_detail_code);
PROM_REACTOR_API int prometheus_reactor_runtime_sgemm_abandon_async(void* handle, int task_id);
PROM_REACTOR_API int prometheus_reactor_runtime_sgemm_policy_diagnostics(void* handle,
                                                                         PrometheusSgemmPolicyDiagnostics* out_diag);
PROM_REACTOR_API int prometheus_reactor_runtime_sgemm_policy_diagnostics_sized(void* handle,
                                                                               PrometheusSgemmPolicyDiagnostics* out_diag,
                                                                               uint32_t out_size);
PROM_REACTOR_API int prometheus_reactor_runtime_p15_test_seed_matured_reservation(void* handle,
                                                                                    uint32_t shape_class,
                                                                                    uint32_t variant_id,
                                                                                    uint64_t target_tick);
PROM_REACTOR_API int prometheus_reactor_runtime_sgemm_batch_diagnostics(void* handle,
                                                                        PrometheusSgemmBatchDiagnostics* out_diag);

PROM_REACTOR_API int prometheus_reactor_runtime_fft(void* handle,
                                                    const PrometheusFftRequest* request,
                                                    uint32_t* out_stage,
                                                    int* out_detail_code);
PROM_REACTOR_API int prometheus_reactor_runtime_fft_benchmark_variant(void* handle,
                                                                      const PrometheusFftRequest* request,
                                                                      uint32_t requested_variant,
                                                                      uint32_t* out_stage,
                                                                      int* out_detail_code);
PROM_REACTOR_API int prometheus_reactor_runtime_fft_diagnostics(void* handle,
                                                                PrometheusFftDiagnostics* out_diag);
PROM_REACTOR_API int prometheus_reactor_runtime_fft_diagnostics_sized(void* handle,
                                                                      PrometheusFftDiagnostics* out_diag,
                                                                      uint32_t out_size);

PROM_REACTOR_API int prometheus_reactor_reduction_plan(const PrometheusReductionRequest* request,
                                                        PrometheusReductionPlan* out_plan);
PROM_REACTOR_API int prometheus_reactor_runtime_reduction(void* handle,
                                                          const PrometheusReductionRequest* request,
                                                          PrometheusReductionExecutionResult* out_result);
PROM_REACTOR_API int prometheus_reactor_runtime_reduction_diagnostics(void* handle,
                                                                      PrometheusReductionDiagnostics* out_diag);
PROM_REACTOR_API int prometheus_reactor_runtime_reduction_benchmark(
    void* handle,
    const PrometheusReductionBenchmarkRequest* request,
    PrometheusReductionBenchmarkResult* out_result);

/* Backward-compat aliases for earlier contract drafts. */
PROM_REACTOR_API int prometheus_runtime_create(void* config, void** out_handle);
PROM_REACTOR_API int prometheus_runtime_destroy(void* handle);
PROM_REACTOR_API int prometheus_runtime_probe(void* handle, PrometheusCaps* out_caps);

#ifdef __cplusplus
}
#endif

#endif
