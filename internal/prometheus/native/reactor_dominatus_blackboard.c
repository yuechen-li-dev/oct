#include "reactor_dominatus_blackboard.h"

#include <string.h>

enum {
  PROM_DOM_KEY_INDEX_SGEMM_SHAPE = 0,
  PROM_DOM_KEY_INDEX_SGEMM_LAYOUT = 1,
  PROM_DOM_KEY_INDEX_SGEMM_PRECISION = 2,
  PROM_DOM_KEY_INDEX_SGEMM_PATH_MODE = 3,
  PROM_DOM_KEY_INDEX_SGEMM_COMPUTE_MODE = 4,
  PROM_DOM_KEY_INDEX_SGEMM_BUFFERING_MODE = 5,
  PROM_DOM_KEY_INDEX_SGEMM_FALLBACK_REASON = 6,
  PROM_DOM_KEY_INDEX_SGEMM_M35_FIXED_FEASIBLE = 7,
  PROM_DOM_KEY_INDEX_SGEMM_M35_PULL_LAG_FEASIBLE = 8,
  PROM_DOM_KEY_INDEX_SGEMM_M35_SERIAL_FEASIBLE = 9,
  PROM_DOM_KEY_INDEX_SGEMM_M35_FIXED_SCORE = 10,
  PROM_DOM_KEY_INDEX_SGEMM_M35_PULL_LAG_SCORE = 11,
  PROM_DOM_KEY_INDEX_SGEMM_M35_SERIAL_SCORE = 12,
  PROM_DOM_KEY_INDEX_SGEMM_M35_REASON_CODE = 13,
  PROM_DOM_KEY_INDEX_SGEMM_M35_FINAL_REASON_CODE = 14,
  PROM_DOM_KEY_INDEX_SGEMM_M35_FIXED_REJECTION_REASON = 15,
  PROM_DOM_KEY_INDEX_SGEMM_M35_PULL_LAG_REJECTION_REASON = 16,
  PROM_DOM_KEY_INDEX_SGEMM_M35_SERIAL_REJECTION_REASON = 17,
  PROM_DOM_KEY_INDEX_SGEMM_M35_TRANSFER_VARIANCE_CLASS = 18,
  PROM_DOM_KEY_INDEX_SGEMM_M35_COMPUTE_PREDICTABILITY_CLASS = 19,
  PROM_DOM_KEY_INDEX_SGEMM_M35_STARVATION_RISK_HIGH = 20,
  PROM_DOM_KEY_INDEX_SGEMM_M35_PULL_LAG_WIP_WASTE_EXCEEDED = 21,
  PROM_DOM_KEY_INDEX_SGEMM_M35_FALLBACK_AVAILABLE = 22,
  PROM_DOM_KEY_INDEX_SGEMM_M35_FIXED_REJECTED = 23,
  PROM_DOM_KEY_INDEX_SGEMM_M35_PULL_LAG_REJECTED = 24,
  PROM_DOM_KEY_INDEX_SGEMM_M35_SERIAL_REJECTED = 25,
  PROM_DOM_KEY_INDEX_SGEMM_M35_SUCCESS = 26,
  PROM_DOM_KEY_INDEX_SGEMM_M35_NO_FEASIBLE_DETAIL = 27,
  PROM_DOM_KEY_INDEX_SGEMM_PACKED4_AVAILABLE = 28,
  PROM_DOM_KEY_INDEX_SGEMM_PACKED4_SMALL_SHAPE = 29,
  PROM_DOM_KEY_INDEX_SGEMM_PACKED4_PADDING_WASTE_PERMILLE = 30,
  PROM_DOM_KEY_INDEX_SGEMM_PACKED4_MODE_BUDGET_PERMILLE = 31,
  PROM_DOM_KEY_INDEX_SGEMM_PACKED4_ROW_MAJOR_VALID = 32,
  PROM_DOM_KEY_INDEX_SGEMM_PACKED4_TAIL_VALID = 33,
  PROM_DOM_KEY_INDEX_SGEMM_FP16_STRICT_FP32 = 34,
  PROM_DOM_KEY_INDEX_SGEMM_FP16_TOLERANCE_KNOWN = 35,
  PROM_DOM_KEY_INDEX_SGEMM_FP16_TOLERANCE_PASS = 36,
  PROM_DOM_KEY_INDEX_SGEMM_FP16_HAS_SPECIAL_VALUES = 37,
  PROM_DOM_KEY_INDEX_SGEMM_FP16_CAPABILITY_STORAGE = 38,
  PROM_DOM_KEY_INDEX_SGEMM_FP16_FALLBACK_AVAILABLE = 39,
  PROM_DOM_KEY_INDEX_SGEMM_FP16_UTILITY_SCORE = 40,
  PROM_DOM_KEY_INDEX_SGEMM_PACKED4_SELECTED = 41,
  PROM_DOM_KEY_INDEX_SGEMM_PACKED4_REJECT_REASON = 42,
  PROM_DOM_KEY_INDEX_SGEMM_FP16_SELECTED = 43,
  PROM_DOM_KEY_INDEX_SGEMM_FP16_REJECT_REASON = 44,
  PROM_DOM_KEY_INDEX_SGEMM_FACT_SHAPE_M = 45,
  PROM_DOM_KEY_INDEX_SGEMM_FACT_SHAPE_N = 46,
  PROM_DOM_KEY_INDEX_SGEMM_FACT_SHAPE_K = 47,
  PROM_DOM_KEY_INDEX_SGEMM_FACT_WORK_UNITS = 48,
  PROM_DOM_KEY_INDEX_SGEMM_FACT_CAN_STAGE = 49,
  PROM_DOM_KEY_INDEX_SGEMM_FACT_CAN_DIRECT = 50,
  PROM_DOM_KEY_INDEX_SGEMM_FACT_ALLOW_FALLBACK = 51,
  PROM_DOM_KEY_INDEX_SGEMM_FACT_READBACK_REQUIRED = 52,
  PROM_DOM_KEY_INDEX_SGEMM_FACT_FORCE_DIRECT = 53,
  PROM_DOM_KEY_INDEX_SGEMM_FACT_FORCE_STAGED = 54,
  PROM_DOM_KEY_INDEX_SGEMM_FACT_FORCE_TILED = 55,
  PROM_DOM_KEY_INDEX_SGEMM_FACT_TILED_SHAPE = 56,
  PROM_DOM_KEY_INDEX_SGEMM_FACT_SOFTWARE_VULKAN = 57,
  PROM_DOM_KEY_INDEX_SGEMM_FACT_POLICY_MODE = 58,
  PROM_DOM_KEY_INDEX_SGEMM_JUDGMENT_SUCCESS = 59,
  PROM_DOM_KEY_INDEX_SGEMM_JUDGMENT_ERROR_DETAIL = 60,
  PROM_DOM_KEY_INDEX_SGEMM_JUDGMENT_REQUESTED_PATH = 61,
  PROM_DOM_KEY_INDEX_SGEMM_JUDGMENT_SELECTED_PATH = 62,
  PROM_DOM_KEY_INDEX_SGEMM_JUDGMENT_COMPUTE_MODE = 63,
  PROM_DOM_KEY_INDEX_SGEMM_JUDGMENT_FINAL_DETAIL = 64,
  PROM_DOM_KEY_INDEX_SGEMM_JUDGMENT_USED_FALLBACK_TO_DIRECT = 65,
  PROM_DOM_KEY_INDEX_SGEMM_JUDGMENT_WINNING_CANDIDATE_INDEX = 66,
  PROM_DOM_KEY_INDEX_SGEMM_JUDGMENT_WINNING_SCORE = 67,
  PROM_DOM_KEY_INDEX_SLOT_STATE = 68,
  PROM_DOM_KEY_INDEX_SLOT_GENERATION = 69,
  PROM_DOM_KEY_INDEX_SLOT_VALID = 70,
  PROM_DOM_KEY_INDEX_SLOT_CURRENT_ID = 71,
  PROM_DOM_KEY_INDEX_SLOT_NEXT_ID = 72,
  PROM_DOM_KEY_INDEX_SLOT_FAILURE_REASON = 73,
  PROM_DOM_KEY_INDEX_QUEUE_COMPUTE_FAMILY = 74,
  PROM_DOM_KEY_INDEX_QUEUE_TRANSFER_FAMILY = 75,
  PROM_DOM_KEY_INDEX_QUEUE_DEDICATED_AVAILABLE = 76,
  PROM_DOM_KEY_INDEX_QUEUE_TRANSFER_POLICY = 77,
  PROM_DOM_KEY_INDEX_QUEUE_HANDOFF_COUNT = 78,
  PROM_DOM_KEY_INDEX_QUEUE_FAMILIES_DIFFER = 79,
  PROM_DOM_KEY_INDEX_QUEUE_TRANSFER_SUPPORTED = 80,
  PROM_DOM_KEY_INDEX_QUEUE_TRANSFER_DISABLED_BY_CONFIG = 81,
  PROM_DOM_KEY_INDEX_QUEUE_TRANSFER_WORKLOAD_LARGE_ENOUGH = 82,
  PROM_DOM_KEY_INDEX_QUEUE_TRANSFER_SYNC_OWNERSHIP_SUPPORTED = 83,
  PROM_DOM_KEY_INDEX_QUEUE_TRANSFER_FALLBACK_AVAILABLE = 84,
  PROM_DOM_KEY_INDEX_QUEUE_TRANSFER_UPLOAD_ONLY_ELIGIBLE = 85,
  PROM_DOM_KEY_INDEX_QUEUE_TRANSFER_UPLOAD_READBACK_SUPPORTED = 86,
  PROM_DOM_KEY_INDEX_QUEUE_TRANSFER_POLICY_SELECTED = 87,
  PROM_DOM_KEY_INDEX_QUEUE_TRANSFER_FALLBACK_REASON = 88,
  PROM_DOM_KEY_INDEX_QUEUE_TRANSFER_QUEUE_USED = 89,
  PROM_DOM_KEY_INDEX_QUEUE_TRANSFER_COMPUTE_WAIT_COUNT = 90,
  PROM_DOM_KEY_INDEX_QUEUE_TRANSFER_FAILURE_SLOT_ID = 91,
  PROM_DOM_KEY_INDEX_QUEUE_TRANSFER_FAILURE_REASON = 92,
  PROM_DOM_KEY_INDEX_QUEUE_TRANSFER_FAILURE_COUNT = 93,
  PROM_DOM_KEY_INDEX_QUEUE_ASYNC_TRANSFER_COMPLETE = 94,
  PROM_DOM_KEY_INDEX_QUEUE_ASYNC_TRANSFER_COMPLETION_GENERATION = 95,
  PROM_DOM_KEY_INDEX_MEMORY_REQUIRED_CAPACITY = 96,
  PROM_DOM_KEY_INDEX_MEMORY_BUDGET = 97,
  PROM_DOM_KEY_INDEX_MEMORY_HEADROOM = 98,
  PROM_DOM_KEY_INDEX_MEMORY_INVALIDATION_FLAGS = 99,
  PROM_DOM_KEY_INDEX_MEMORY_M35_FIXED_HEADROOM = 100,
  PROM_DOM_KEY_INDEX_MEMORY_M35_PULL_LAG_HEADROOM = 101,
  PROM_DOM_KEY_INDEX_MEMORY_M35_SERIAL_HEADROOM = 102,
  PROM_DOM_KEY_INDEX_MEMORY_M35_REQUIRED_FIXED = 103,
  PROM_DOM_KEY_INDEX_MEMORY_M35_REQUIRED_PULL_LAG = 104,
  PROM_DOM_KEY_INDEX_MEMORY_M35_REQUIRED_SERIAL = 105,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_REASON_CODE = 106,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_COUNTER = 107,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_LAST_TRANSITION = 108,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_PACKED4_SELECTED_LAYOUT_FORMAT = 109,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_PACKED4_TAIL_COUNT_LAST = 110,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_PACKED4_TAIL_COUNT_TOTAL = 111,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_PACKED4_PADDED_LANE_COUNT_LAST = 112,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_PACKED4_PADDED_LANE_COUNT_TOTAL = 113,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_PACKED4_PADDING_WASTE_PERMILLE_LAST = 114,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_PACKED4_MODE_BUDGET_DENIALS = 115,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_PACKED4_ROW_MAJOR_CHECK_FAILURES = 116,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_PACKED4_SELECTION_COUNT = 117,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_PACKED4_FALLBACK_REASON_PADDING_WASTE = 118,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_PACKED4_FALLBACK_REASON_SMALL_SHAPE = 119,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_PACKED4_FALLBACK_REASON_CAPABILITY_MISSING = 120,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_PACKED4_FALLBACK_REASON_FALLBACK_REQUIRED = 121,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_PACKED4_FALLBACK_REASON_MODE_BUDGET_DENIED = 122,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_FP16_MAX_ABSOLUTE_ERROR_BITS = 123,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_FP16_MAX_RELATIVE_ERROR_BITS = 124,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_FP16_AGGREGATE_ERROR_BITS = 125,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_FP16_WORST_CASE_ELEMENT_INDEX = 126,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_FP16_K_ERROR_GROWTH_BITS = 127,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_FP16_CANCELLATION_RISK_BITS = 128,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_FP16_TOLERANCE_KNOWN = 129,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_FP16_TOLERANCE_PASS = 130,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_FP16_FALLBACK_REASON_DETAIL = 131,
  PROM_DOM_KEY_INDEX_DIAGNOSTICS_FP16_SELECTED_CANDIDATE = 132,
  PROM_DOM_KEY_INDEX_FFT_RESERVED = 133,
  PROM_DOM_KEY_INDEX_COUNT = 134,
};

typedef struct prom_dom_key_info {
  prom_dom_key key;
  prom_dom_domain domain;
  uint32_t slot_scoped;
} prom_dom_key_info;

static const prom_dom_key_info k_key_info[PROM_DOM_KEY_INDEX_COUNT] = {
    {PROM_DOM_KEY_SGEMM_SHAPE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_LAYOUT, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_PRECISION, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_PATH_MODE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_COMPUTE_MODE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_BUFFERING_MODE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FALLBACK_REASON, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_M35_FIXED_FEASIBLE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_M35_PULL_LAG_FEASIBLE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_M35_SERIAL_FEASIBLE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_M35_FIXED_SCORE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_M35_PULL_LAG_SCORE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_M35_SERIAL_SCORE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_M35_REASON_CODE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_M35_FINAL_REASON_CODE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_M35_FIXED_REJECTION_REASON, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_M35_PULL_LAG_REJECTION_REASON, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_M35_SERIAL_REJECTION_REASON, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_M35_TRANSFER_VARIANCE_CLASS, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_M35_COMPUTE_PREDICTABILITY_CLASS, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_M35_STARVATION_RISK_HIGH, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_M35_PULL_LAG_WIP_WASTE_EXCEEDED, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_M35_FALLBACK_AVAILABLE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_M35_FIXED_REJECTED, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_M35_PULL_LAG_REJECTED, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_M35_SERIAL_REJECTED, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_M35_SUCCESS, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_M35_NO_FEASIBLE_DETAIL, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_PACKED4_AVAILABLE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_PACKED4_SMALL_SHAPE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_PACKED4_PADDING_WASTE_PERMILLE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_PACKED4_MODE_BUDGET_PERMILLE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_PACKED4_ROW_MAJOR_VALID, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_PACKED4_TAIL_VALID, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FP16_STRICT_FP32, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FP16_TOLERANCE_KNOWN, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FP16_TOLERANCE_PASS, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FP16_HAS_SPECIAL_VALUES, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FP16_CAPABILITY_STORAGE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FP16_FALLBACK_AVAILABLE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FP16_UTILITY_SCORE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_PACKED4_SELECTED, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_PACKED4_REJECT_REASON, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FP16_SELECTED, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FP16_REJECT_REASON, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FACT_SHAPE_M, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FACT_SHAPE_N, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FACT_SHAPE_K, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FACT_WORK_UNITS, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FACT_CAN_STAGE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FACT_CAN_DIRECT, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FACT_ALLOW_FALLBACK, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FACT_READBACK_REQUIRED, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FACT_FORCE_DIRECT, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FACT_FORCE_STAGED, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FACT_FORCE_TILED, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FACT_TILED_SHAPE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FACT_SOFTWARE_VULKAN, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_FACT_POLICY_MODE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_JUDGMENT_SUCCESS, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_JUDGMENT_ERROR_DETAIL, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_JUDGMENT_REQUESTED_PATH, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_JUDGMENT_SELECTED_PATH, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_JUDGMENT_COMPUTE_MODE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_JUDGMENT_FINAL_DETAIL, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_JUDGMENT_USED_FALLBACK_TO_DIRECT, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_JUDGMENT_WINNING_CANDIDATE_INDEX, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SGEMM_JUDGMENT_WINNING_SCORE, PROM_DOM_DOMAIN_SGEMM, 0u},
    {PROM_DOM_KEY_SLOT_STATE, PROM_DOM_DOMAIN_SLOT, 1u},
    {PROM_DOM_KEY_SLOT_GENERATION, PROM_DOM_DOMAIN_SLOT, 1u},
    {PROM_DOM_KEY_SLOT_VALID, PROM_DOM_DOMAIN_SLOT, 1u},
    {PROM_DOM_KEY_SLOT_CURRENT_ID, PROM_DOM_DOMAIN_SLOT, 0u},
    {PROM_DOM_KEY_SLOT_NEXT_ID, PROM_DOM_DOMAIN_SLOT, 0u},
    {PROM_DOM_KEY_SLOT_FAILURE_REASON, PROM_DOM_DOMAIN_SLOT, 1u},
    {PROM_DOM_KEY_QUEUE_COMPUTE_FAMILY, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_QUEUE_TRANSFER_FAMILY, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_QUEUE_DEDICATED_AVAILABLE, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_QUEUE_TRANSFER_POLICY, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_QUEUE_HANDOFF_COUNT, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_QUEUE_FAMILIES_DIFFER, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_QUEUE_TRANSFER_SUPPORTED, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_QUEUE_TRANSFER_DISABLED_BY_CONFIG, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_QUEUE_TRANSFER_WORKLOAD_LARGE_ENOUGH, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_QUEUE_TRANSFER_SYNC_OWNERSHIP_SUPPORTED, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_QUEUE_TRANSFER_FALLBACK_AVAILABLE, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_QUEUE_TRANSFER_UPLOAD_ONLY_ELIGIBLE, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_QUEUE_TRANSFER_UPLOAD_READBACK_SUPPORTED, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_QUEUE_TRANSFER_POLICY_SELECTED, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_QUEUE_TRANSFER_FALLBACK_REASON, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_QUEUE_TRANSFER_QUEUE_USED, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_QUEUE_TRANSFER_COMPUTE_WAIT_COUNT, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_QUEUE_TRANSFER_FAILURE_SLOT_ID, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_QUEUE_TRANSFER_FAILURE_REASON, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_QUEUE_TRANSFER_FAILURE_COUNT, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_QUEUE_ASYNC_TRANSFER_COMPLETE, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_QUEUE_ASYNC_TRANSFER_COMPLETION_GENERATION, PROM_DOM_DOMAIN_QUEUE, 0u},
    {PROM_DOM_KEY_MEMORY_REQUIRED_CAPACITY, PROM_DOM_DOMAIN_MEMORY, 0u},
    {PROM_DOM_KEY_MEMORY_BUDGET, PROM_DOM_DOMAIN_MEMORY, 0u},
    {PROM_DOM_KEY_MEMORY_HEADROOM, PROM_DOM_DOMAIN_MEMORY, 0u},
    {PROM_DOM_KEY_MEMORY_INVALIDATION_FLAGS, PROM_DOM_DOMAIN_MEMORY, 0u},
    {PROM_DOM_KEY_MEMORY_M35_FIXED_HEADROOM, PROM_DOM_DOMAIN_MEMORY, 0u},
    {PROM_DOM_KEY_MEMORY_M35_PULL_LAG_HEADROOM, PROM_DOM_DOMAIN_MEMORY, 0u},
    {PROM_DOM_KEY_MEMORY_M35_SERIAL_HEADROOM, PROM_DOM_DOMAIN_MEMORY, 0u},
    {PROM_DOM_KEY_MEMORY_M35_REQUIRED_FIXED, PROM_DOM_DOMAIN_MEMORY, 0u},
    {PROM_DOM_KEY_MEMORY_M35_REQUIRED_PULL_LAG, PROM_DOM_DOMAIN_MEMORY, 0u},
    {PROM_DOM_KEY_MEMORY_M35_REQUIRED_SERIAL, PROM_DOM_DOMAIN_MEMORY, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_REASON_CODE, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_COUNTER, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_LAST_TRANSITION, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_PACKED4_SELECTED_LAYOUT_FORMAT, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_PACKED4_TAIL_COUNT_LAST, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_PACKED4_TAIL_COUNT_TOTAL, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_PACKED4_PADDED_LANE_COUNT_LAST, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_PACKED4_PADDED_LANE_COUNT_TOTAL, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_PACKED4_PADDING_WASTE_PERMILLE_LAST, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_PACKED4_MODE_BUDGET_DENIALS, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_PACKED4_ROW_MAJOR_CHECK_FAILURES, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_PACKED4_SELECTION_COUNT, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_PACKED4_FALLBACK_REASON_PADDING_WASTE, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_PACKED4_FALLBACK_REASON_SMALL_SHAPE, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_PACKED4_FALLBACK_REASON_CAPABILITY_MISSING, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_PACKED4_FALLBACK_REASON_FALLBACK_REQUIRED, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_PACKED4_FALLBACK_REASON_MODE_BUDGET_DENIED, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_FP16_MAX_ABSOLUTE_ERROR_BITS, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_FP16_MAX_RELATIVE_ERROR_BITS, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_FP16_AGGREGATE_ERROR_BITS, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_FP16_WORST_CASE_ELEMENT_INDEX, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_FP16_K_ERROR_GROWTH_BITS, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_FP16_CANCELLATION_RISK_BITS, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_FP16_TOLERANCE_KNOWN, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_FP16_TOLERANCE_PASS, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_FP16_FALLBACK_REASON_DETAIL, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_DIAGNOSTICS_FP16_SELECTED_CANDIDATE, PROM_DOM_DOMAIN_DIAGNOSTICS, 0u},
    {PROM_DOM_KEY_FFT_RESERVED, PROM_DOM_DOMAIN_FFT, 0u},
};

static uint32_t key_to_index(prom_dom_key key, uint32_t* out_index) {
  uint32_t i;
  if (out_index == 0) {
    return 0u;
  }

  for (i = 0u; i < PROM_DOM_KEY_INDEX_COUNT; ++i) {
    if (k_key_info[i].key == key) {
      *out_index = i;
      return 1u;
    }
  }

  return 0u;
}

static uint32_t key_slot_limit(prom_dom_key key) {
  if (key == PROM_DOM_KEY_SLOT_STATE || key == PROM_DOM_KEY_SLOT_GENERATION || key == PROM_DOM_KEY_SLOT_VALID ||
      key == PROM_DOM_KEY_SLOT_FAILURE_REASON) {
    return PROM_DOM_MAX_SLOTS;
  }
  return 1u;
}

static uint32_t storage_index_for_key(prom_dom_key key, uint32_t slot_id, uint32_t* out_storage_index, uint32_t* out_key_index) {
  uint32_t key_index;
  if (key_to_index(key, &key_index) == 0u || out_storage_index == 0 || out_key_index == 0) {
    return 0u;
  }

  if (k_key_info[key_index].slot_scoped != 0u) {
    if (slot_id >= PROM_DOM_MAX_SLOTS) {
      return 0u;
    }
    *out_storage_index = key_index * PROM_DOM_MAX_SLOTS + slot_id;
  } else {
    if (slot_id != 0u) {
      return 0u;
    }
    *out_storage_index = key_index * PROM_DOM_MAX_SLOTS;
  }
  *out_key_index = key_index;
  return 1u;
}

static uint32_t domain_bit(prom_dom_domain domain) {
  if (domain == PROM_DOM_DOMAIN_INVALID || domain > PROM_DOM_DOMAIN_FFT) {
    return 0u;
  }
  return 1u << (uint32_t)(domain - 1u);
}

static uint32_t values_equal(const prom_dom_value* a, const prom_dom_value* b) {
  if (a == 0 || b == 0 || a->type != b->type) {
    return 0u;
  }

  if (a->type == PROM_DOM_VALUE_U32) {
    return a->data.u32 == b->data.u32;
  }
  if (a->type == PROM_DOM_VALUE_U64) {
    return a->data.u64 == b->data.u64;
  }
  if (a->type == PROM_DOM_VALUE_I32) {
    return a->data.i32 == b->data.i32;
  }
  if (a->type == PROM_DOM_VALUE_I64) {
    return a->data.i64 == b->data.i64;
  }
  if (a->type == PROM_DOM_VALUE_BOOL) {
    return a->data.boolean == b->data.boolean;
  }
  return 1u;
}

static prom_dom_domain key_domain(prom_dom_key key) {
  uint32_t key_index;
  if (key_to_index(key, &key_index) == 0u) {
    return PROM_DOM_DOMAIN_INVALID;
  }
  return k_key_info[key_index].domain;
}

static uint32_t key_word_index(uint32_t key_index) {
  return key_index / 64u;
}

static uint64_t key_bit(uint32_t key_index) {
  return 1ull << (key_index % 64u);
}

static void recompute_domain_and_slot_dirty(prom_dom_blackboard* board) {
  uint32_t key_index;
  board->dirty_domains_staged = 0u;
  board->dirty_slots_staged = 0u;

  for (key_index = 0u; key_index < PROM_DOM_KEY_INDEX_COUNT; ++key_index) {
    const uint64_t bit = key_bit(key_index);
    const uint32_t word_index = key_word_index(key_index);
    if (word_index >= PROM_DOM_KEY_WORDS || (board->dirty_keys_staged[word_index] & bit) == 0u) {
      continue;
    }

    board->dirty_domains_staged |= domain_bit(k_key_info[key_index].domain);
    if (k_key_info[key_index].slot_scoped != 0u) {
      uint32_t slot_id;
      for (slot_id = 0u; slot_id < key_slot_limit(k_key_info[key_index].key); ++slot_id) {
        const uint32_t storage_index = key_index * PROM_DOM_MAX_SLOTS + slot_id;
        if (values_equal(&board->staged_values[storage_index], &board->visible_values[storage_index]) == 0u) {
          board->dirty_slots_staged |= (1u << slot_id);
        }
      }
    }
  }
}

static void update_dirty_tracking_for_value(prom_dom_blackboard* board, uint32_t key_index) {
  uint32_t slot_count = key_slot_limit(k_key_info[key_index].key);
  uint32_t has_diff = 0u;
  uint32_t slot_id;

  for (slot_id = 0u; slot_id < slot_count; ++slot_id) {
    const uint32_t storage_index = key_index * PROM_DOM_MAX_SLOTS + slot_id;
    if (values_equal(&board->staged_values[storage_index], &board->visible_values[storage_index]) == 0u) {
      has_diff = 1u;
      break;
    }
  }

  if (has_diff != 0u) {
    const uint32_t word_index = key_word_index(key_index);
    if (word_index < PROM_DOM_KEY_WORDS) {
      board->dirty_keys_staged[word_index] |= key_bit(key_index);
    }
  } else {
    const uint32_t word_index = key_word_index(key_index);
    if (word_index < PROM_DOM_KEY_WORDS) {
      board->dirty_keys_staged[word_index] &= ~key_bit(key_index);
    }
  }

  recompute_domain_and_slot_dirty(board);
}

static void append_trace(prom_dom_blackboard* board, const prom_dom_trace_entry* entry) {
  uint32_t position;
  if (board == 0 || entry == 0 || PROM_DOM_MAX_TRACE == 0u) {
    return;
  }

  if (board->trace_count < PROM_DOM_MAX_TRACE) {
    position = (board->trace_start + board->trace_count) % PROM_DOM_MAX_TRACE;
    board->trace_ring[position] = *entry;
    board->trace_count += 1u;
    return;
  }

  board->trace_ring[board->trace_start] = *entry;
  board->trace_start = (board->trace_start + 1u) % PROM_DOM_MAX_TRACE;
}

static uint32_t set_value(prom_dom_blackboard* board,
                          prom_dom_source source,
                          prom_dom_key key,
                          uint32_t slot_id,
                          prom_dom_value value,
                          int32_t reason_code) {
  uint32_t storage_index;
  uint32_t key_index;
  prom_dom_value old_value;
  prom_dom_trace_entry trace;

  if (board == 0) {
    return 0u;
  }
  if (storage_index_for_key(key, slot_id, &storage_index, &key_index) == 0u) {
    return 0u;
  }

  old_value = board->staged_values[storage_index];
  if (values_equal(&old_value, &value) != 0u) {
    return 1u;
  }

  board->staged_values[storage_index] = value;
  update_dirty_tracking_for_value(board, key_index);

  trace.generation = board->staged_generation;
  trace.sequence = board->sequence_counter;
  trace.source = source;
  trace.domain = key_domain(key);
  trace.key = key;
  trace.event_kind = PROM_DOM_EVENT_NONE;
  trace.slot_id = slot_id;
  trace.reason_code = reason_code;
  trace.old_value = old_value;
  trace.new_value = value;
  append_trace(board, &trace);

  board->sequence_counter += 1u;
  return 1u;
}

void prom_dom_blackboard_init(prom_dom_blackboard* board) {
  if (board == 0) {
    return;
  }

  memset(board, 0, sizeof(*board));
  board->staged_generation = 1u;
}

void prom_dom_blackboard_reset(prom_dom_blackboard* board) {
  prom_dom_blackboard_init(board);
}

uint32_t prom_dom_set_u32(prom_dom_blackboard* board,
                          prom_dom_source source,
                          prom_dom_key key,
                          uint32_t slot_id,
                          uint32_t value,
                          int32_t reason_code) {
  prom_dom_value boxed;
  boxed.type = PROM_DOM_VALUE_U32;
  boxed.data.u32 = value;
  return set_value(board, source, key, slot_id, boxed, reason_code);
}

uint32_t prom_dom_set_u64(prom_dom_blackboard* board,
                          prom_dom_source source,
                          prom_dom_key key,
                          uint32_t slot_id,
                          uint64_t value,
                          int32_t reason_code) {
  prom_dom_value boxed;
  boxed.type = PROM_DOM_VALUE_U64;
  boxed.data.u64 = value;
  return set_value(board, source, key, slot_id, boxed, reason_code);
}

uint32_t prom_dom_set_i32(prom_dom_blackboard* board,
                          prom_dom_source source,
                          prom_dom_key key,
                          uint32_t slot_id,
                          int32_t value,
                          int32_t reason_code) {
  prom_dom_value boxed;
  boxed.type = PROM_DOM_VALUE_I32;
  boxed.data.i32 = value;
  return set_value(board, source, key, slot_id, boxed, reason_code);
}

uint32_t prom_dom_set_i64(prom_dom_blackboard* board,
                          prom_dom_source source,
                          prom_dom_key key,
                          uint32_t slot_id,
                          int64_t value,
                          int32_t reason_code) {
  prom_dom_value boxed;
  boxed.type = PROM_DOM_VALUE_I64;
  boxed.data.i64 = value;
  return set_value(board, source, key, slot_id, boxed, reason_code);
}

uint32_t prom_dom_set_bool(prom_dom_blackboard* board,
                           prom_dom_source source,
                           prom_dom_key key,
                           uint32_t slot_id,
                           uint32_t value,
                           int32_t reason_code) {
  prom_dom_value boxed;
  boxed.type = PROM_DOM_VALUE_BOOL;
  boxed.data.boolean = value != 0u ? 1u : 0u;
  return set_value(board, source, key, slot_id, boxed, reason_code);
}

static uint32_t get_value(const prom_dom_blackboard* board,
                          prom_dom_key key,
                          uint32_t slot_id,
                          prom_dom_value_type expected_type,
                          prom_dom_value* out_value) {
  uint32_t storage_index;
  uint32_t key_index;
  if (board == 0 || out_value == 0) {
    return 0u;
  }

  if (storage_index_for_key(key, slot_id, &storage_index, &key_index) == 0u) {
    return 0u;
  }
  (void)key_index;

  *out_value = board->visible_values[storage_index];
  if (out_value->type != expected_type) {
    return 0u;
  }
  return 1u;
}

uint32_t prom_dom_get_u32(const prom_dom_blackboard* board, prom_dom_key key, uint32_t slot_id, uint32_t* out_value) {
  prom_dom_value boxed;
  if (out_value == 0) {
    return 0u;
  }
  if (get_value(board, key, slot_id, PROM_DOM_VALUE_U32, &boxed) == 0u) {
    return 0u;
  }
  *out_value = boxed.data.u32;
  return 1u;
}

uint32_t prom_dom_get_u64(const prom_dom_blackboard* board, prom_dom_key key, uint32_t slot_id, uint64_t* out_value) {
  prom_dom_value boxed;
  if (out_value == 0) {
    return 0u;
  }
  if (get_value(board, key, slot_id, PROM_DOM_VALUE_U64, &boxed) == 0u) {
    return 0u;
  }
  *out_value = boxed.data.u64;
  return 1u;
}

uint32_t prom_dom_get_i32(const prom_dom_blackboard* board, prom_dom_key key, uint32_t slot_id, int32_t* out_value) {
  prom_dom_value boxed;
  if (out_value == 0) {
    return 0u;
  }
  if (get_value(board, key, slot_id, PROM_DOM_VALUE_I32, &boxed) == 0u) {
    return 0u;
  }
  *out_value = boxed.data.i32;
  return 1u;
}

uint32_t prom_dom_get_i64(const prom_dom_blackboard* board, prom_dom_key key, uint32_t slot_id, int64_t* out_value) {
  prom_dom_value boxed;
  if (out_value == 0) {
    return 0u;
  }
  if (get_value(board, key, slot_id, PROM_DOM_VALUE_I64, &boxed) == 0u) {
    return 0u;
  }
  *out_value = boxed.data.i64;
  return 1u;
}

uint32_t prom_dom_get_bool(const prom_dom_blackboard* board, prom_dom_key key, uint32_t slot_id, uint32_t* out_value) {
  prom_dom_value boxed;
  if (out_value == 0) {
    return 0u;
  }
  if (get_value(board, key, slot_id, PROM_DOM_VALUE_BOOL, &boxed) == 0u) {
    return 0u;
  }
  *out_value = boxed.data.boolean;
  return 1u;
}

uint32_t prom_dom_stage_event(prom_dom_blackboard* board, const prom_dom_event* event) {
  prom_dom_event staged;
  prom_dom_trace_entry trace;
  if (board == 0 || event == 0) {
    return 0u;
  }

  if (board->staged_event_count >= PROM_DOM_MAX_EVENTS) {
    return 0u;
  }

  staged = *event;
  staged.sequence = board->sequence_counter;
  staged.generation = board->staged_generation;
  board->staged_events[board->staged_event_count] = staged;
  board->staged_event_count += 1u;

  trace.generation = board->staged_generation;
  trace.sequence = board->sequence_counter;
  trace.source = staged.source;
  trace.domain = staged.domain;
  trace.key = staged.key;
  trace.event_kind = staged.kind;
  trace.slot_id = staged.slot_id;
  trace.reason_code = staged.reason_code;
  memset(&trace.old_value, 0, sizeof(trace.old_value));
  memset(&trace.new_value, 0, sizeof(trace.new_value));
  append_trace(board, &trace);

  board->sequence_counter += 1u;
  return 1u;
}

void prom_dom_commit(prom_dom_blackboard* board) {
  uint32_t i;
  if (board == 0) {
    return;
  }

  board->visible_generation += 1u;
  board->staged_generation = board->visible_generation + 1u;

  memcpy(board->visible_values, board->staged_values, sizeof(board->visible_values));

  memcpy(board->dirty_keys_last_commit, board->dirty_keys_staged, sizeof(board->dirty_keys_last_commit));
  board->dirty_domains_last_commit = board->dirty_domains_staged;
  board->dirty_slots_last_commit = board->dirty_slots_staged;

  memset(board->dirty_keys_staged, 0, sizeof(board->dirty_keys_staged));
  board->dirty_domains_staged = 0u;
  board->dirty_slots_staged = 0u;

  for (i = 0u; i < board->staged_event_count; ++i) {
    prom_dom_event committed = board->staged_events[i];
    committed.generation = board->visible_generation;

    if (board->committed_event_count < PROM_DOM_MAX_EVENTS) {
      const uint32_t position = (board->committed_event_start + board->committed_event_count) % PROM_DOM_MAX_EVENTS;
      board->committed_events[position] = committed;
      board->committed_event_count += 1u;
    } else {
      board->committed_events[board->committed_event_start] = committed;
      board->committed_event_start = (board->committed_event_start + 1u) % PROM_DOM_MAX_EVENTS;
    }
  }

  board->staged_event_count = 0u;
}

uint64_t prom_dom_dirty_keys_staged_word(const prom_dom_blackboard* board, uint32_t word_index) {
  if (board == 0 || word_index >= PROM_DOM_KEY_WORDS) {
    return 0u;
  }
  return board->dirty_keys_staged[word_index];
}

uint64_t prom_dom_dirty_keys_last_commit_word(const prom_dom_blackboard* board, uint32_t word_index) {
  if (board == 0 || word_index >= PROM_DOM_KEY_WORDS) {
    return 0u;
  }
  return board->dirty_keys_last_commit[word_index];
}

uint32_t prom_dom_dirty_key_staged(const prom_dom_blackboard* board, prom_dom_key key) {
  uint32_t key_index;
  if (board == 0 || key_to_index(key, &key_index) == 0u) {
    return 0u;
  }
  {
    const uint32_t word_index = key_word_index(key_index);
    if (word_index >= PROM_DOM_KEY_WORDS) {
      return 0u;
    }
    return (board->dirty_keys_staged[word_index] & key_bit(key_index)) != 0u ? 1u : 0u;
  }
}

uint32_t prom_dom_dirty_key_last_commit(const prom_dom_blackboard* board, prom_dom_key key) {
  uint32_t key_index;
  if (board == 0 || key_to_index(key, &key_index) == 0u) {
    return 0u;
  }
  {
    const uint32_t word_index = key_word_index(key_index);
    if (word_index >= PROM_DOM_KEY_WORDS) {
      return 0u;
    }
    return (board->dirty_keys_last_commit[word_index] & key_bit(key_index)) != 0u ? 1u : 0u;
  }
}

uint32_t prom_dom_dirty_domains_staged(const prom_dom_blackboard* board) {
  if (board == 0) {
    return 0u;
  }
  return board->dirty_domains_staged;
}

uint32_t prom_dom_dirty_domains_last_commit(const prom_dom_blackboard* board) {
  if (board == 0) {
    return 0u;
  }
  return board->dirty_domains_last_commit;
}

uint32_t prom_dom_dirty_slots_staged(const prom_dom_blackboard* board) {
  if (board == 0) {
    return 0u;
  }
  return board->dirty_slots_staged;
}

uint32_t prom_dom_dirty_slots_last_commit(const prom_dom_blackboard* board) {
  if (board == 0) {
    return 0u;
  }
  return board->dirty_slots_last_commit;
}

uint32_t prom_dom_staged_event_count(const prom_dom_blackboard* board) {
  if (board == 0) {
    return 0u;
  }
  return board->staged_event_count;
}

uint32_t prom_dom_committed_event_count(const prom_dom_blackboard* board) {
  if (board == 0) {
    return 0u;
  }
  return board->committed_event_count;
}

uint32_t prom_dom_committed_event_at(const prom_dom_blackboard* board, uint32_t index, prom_dom_event* out_event) {
  uint32_t position;
  if (board == 0 || out_event == 0 || index >= board->committed_event_count) {
    return 0u;
  }

  position = (board->committed_event_start + index) % PROM_DOM_MAX_EVENTS;
  *out_event = board->committed_events[position];
  return 1u;
}

uint32_t prom_dom_trace_count(const prom_dom_blackboard* board) {
  if (board == 0) {
    return 0u;
  }
  return board->trace_count;
}

uint32_t prom_dom_trace_at(const prom_dom_blackboard* board, uint32_t index, prom_dom_trace_entry* out_entry) {
  uint32_t position;
  if (board == 0 || out_entry == 0 || index >= board->trace_count) {
    return 0u;
  }

  position = (board->trace_start + index) % PROM_DOM_MAX_TRACE;
  *out_entry = board->trace_ring[position];
  return 1u;
}
