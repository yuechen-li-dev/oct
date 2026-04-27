#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_DOMINATUS_BLACKBOARD_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_DOMINATUS_BLACKBOARD_H

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

enum {
  PROM_DOM_MAX_SLOTS = 8u,
  PROM_DOM_MAX_EVENTS = 32u,
  PROM_DOM_MAX_TRACE = 64u,
  PROM_DOM_KEY_WORDS = 3u,
  PROM_DOM_STORAGE_CAPACITY = 256u * PROM_DOM_MAX_SLOTS,
};

typedef enum prom_dom_domain {
  PROM_DOM_DOMAIN_INVALID = 0,
  PROM_DOM_DOMAIN_SGEMM = 1,
  PROM_DOM_DOMAIN_SLOT = 2,
  PROM_DOM_DOMAIN_QUEUE = 3,
  PROM_DOM_DOMAIN_MEMORY = 4,
  PROM_DOM_DOMAIN_DIAGNOSTICS = 5,
  PROM_DOM_DOMAIN_FFT = 6,
} prom_dom_domain;

typedef enum prom_dom_key {
  PROM_DOM_KEY_INVALID = 0,

  PROM_DOM_KEY_SGEMM_SHAPE = 0x0101,
  PROM_DOM_KEY_SGEMM_LAYOUT = 0x0102,
  PROM_DOM_KEY_SGEMM_PRECISION = 0x0103,
  PROM_DOM_KEY_SGEMM_PATH_MODE = 0x0104,
  PROM_DOM_KEY_SGEMM_COMPUTE_MODE = 0x0105,
  PROM_DOM_KEY_SGEMM_BUFFERING_MODE = 0x0106,
  PROM_DOM_KEY_SGEMM_FALLBACK_REASON = 0x0107,
  PROM_DOM_KEY_SGEMM_M35_FIXED_FEASIBLE = 0x0108,
  PROM_DOM_KEY_SGEMM_M35_PULL_LAG_FEASIBLE = 0x0109,
  PROM_DOM_KEY_SGEMM_M35_SERIAL_FEASIBLE = 0x010A,
  PROM_DOM_KEY_SGEMM_M35_FIXED_SCORE = 0x010B,
  PROM_DOM_KEY_SGEMM_M35_PULL_LAG_SCORE = 0x010C,
  PROM_DOM_KEY_SGEMM_M35_SERIAL_SCORE = 0x010D,
  PROM_DOM_KEY_SGEMM_M35_REASON_CODE = 0x010E,
  PROM_DOM_KEY_SGEMM_M35_FINAL_REASON_CODE = 0x010F,
  PROM_DOM_KEY_SGEMM_M35_FIXED_REJECTION_REASON = 0x0110,
  PROM_DOM_KEY_SGEMM_M35_PULL_LAG_REJECTION_REASON = 0x0111,
  PROM_DOM_KEY_SGEMM_M35_SERIAL_REJECTION_REASON = 0x0112,
  PROM_DOM_KEY_SGEMM_M35_TRANSFER_VARIANCE_CLASS = 0x0113,
  PROM_DOM_KEY_SGEMM_M35_COMPUTE_PREDICTABILITY_CLASS = 0x0114,
  PROM_DOM_KEY_SGEMM_M35_STARVATION_RISK_HIGH = 0x0115,
  PROM_DOM_KEY_SGEMM_M35_PULL_LAG_WIP_WASTE_EXCEEDED = 0x0116,
  PROM_DOM_KEY_SGEMM_M35_FALLBACK_AVAILABLE = 0x0117,
  PROM_DOM_KEY_SGEMM_M35_FIXED_REJECTED = 0x0118,
  PROM_DOM_KEY_SGEMM_M35_PULL_LAG_REJECTED = 0x0119,
  PROM_DOM_KEY_SGEMM_M35_SERIAL_REJECTED = 0x011A,
  PROM_DOM_KEY_SGEMM_M35_SUCCESS = 0x011B,
  PROM_DOM_KEY_SGEMM_M35_NO_FEASIBLE_DETAIL = 0x011C,
  PROM_DOM_KEY_SGEMM_PACKED4_AVAILABLE = 0x011D,
  PROM_DOM_KEY_SGEMM_PACKED4_SMALL_SHAPE = 0x011E,
  PROM_DOM_KEY_SGEMM_PACKED4_PADDING_WASTE_PERMILLE = 0x011F,
  PROM_DOM_KEY_SGEMM_PACKED4_MODE_BUDGET_PERMILLE = 0x0120,
  PROM_DOM_KEY_SGEMM_PACKED4_ROW_MAJOR_VALID = 0x0121,
  PROM_DOM_KEY_SGEMM_PACKED4_TAIL_VALID = 0x0122,
  PROM_DOM_KEY_SGEMM_FP16_STRICT_FP32 = 0x0123,
  PROM_DOM_KEY_SGEMM_FP16_TOLERANCE_KNOWN = 0x0124,
  PROM_DOM_KEY_SGEMM_FP16_TOLERANCE_PASS = 0x0125,
  PROM_DOM_KEY_SGEMM_FP16_HAS_SPECIAL_VALUES = 0x0126,
  PROM_DOM_KEY_SGEMM_FP16_CAPABILITY_STORAGE = 0x0127,
  PROM_DOM_KEY_SGEMM_FP16_FALLBACK_AVAILABLE = 0x0128,
  PROM_DOM_KEY_SGEMM_FP16_UTILITY_SCORE = 0x0129,
  PROM_DOM_KEY_SGEMM_PACKED4_SELECTED = 0x012A,
  PROM_DOM_KEY_SGEMM_PACKED4_REJECT_REASON = 0x012B,
  PROM_DOM_KEY_SGEMM_FP16_SELECTED = 0x012C,
  PROM_DOM_KEY_SGEMM_FP16_REJECT_REASON = 0x012D,
  PROM_DOM_KEY_SGEMM_FACT_SHAPE_M = 0x012E,
  PROM_DOM_KEY_SGEMM_FACT_SHAPE_N = 0x012F,
  PROM_DOM_KEY_SGEMM_FACT_SHAPE_K = 0x0130,
  PROM_DOM_KEY_SGEMM_FACT_WORK_UNITS = 0x0131,
  PROM_DOM_KEY_SGEMM_FACT_CAN_STAGE = 0x0132,
  PROM_DOM_KEY_SGEMM_FACT_CAN_DIRECT = 0x0133,
  PROM_DOM_KEY_SGEMM_FACT_ALLOW_FALLBACK = 0x0134,
  PROM_DOM_KEY_SGEMM_FACT_READBACK_REQUIRED = 0x0135,
  PROM_DOM_KEY_SGEMM_FACT_FORCE_DIRECT = 0x0136,
  PROM_DOM_KEY_SGEMM_FACT_FORCE_STAGED = 0x0137,
  PROM_DOM_KEY_SGEMM_FACT_FORCE_TILED = 0x0138,
  PROM_DOM_KEY_SGEMM_FACT_TILED_SHAPE = 0x0139,
  PROM_DOM_KEY_SGEMM_FACT_SOFTWARE_VULKAN = 0x013A,
  PROM_DOM_KEY_SGEMM_FACT_POLICY_MODE = 0x013B,
  PROM_DOM_KEY_SGEMM_JUDGMENT_SUCCESS = 0x013C,
  PROM_DOM_KEY_SGEMM_JUDGMENT_ERROR_DETAIL = 0x013D,
  PROM_DOM_KEY_SGEMM_JUDGMENT_REQUESTED_PATH = 0x013E,
  PROM_DOM_KEY_SGEMM_JUDGMENT_SELECTED_PATH = 0x013F,
  PROM_DOM_KEY_SGEMM_JUDGMENT_COMPUTE_MODE = 0x0140,
  PROM_DOM_KEY_SGEMM_JUDGMENT_FINAL_DETAIL = 0x0141,
  PROM_DOM_KEY_SGEMM_JUDGMENT_USED_FALLBACK_TO_DIRECT = 0x0142,
  PROM_DOM_KEY_SGEMM_JUDGMENT_WINNING_CANDIDATE_INDEX = 0x0143,
  PROM_DOM_KEY_SGEMM_JUDGMENT_WINNING_SCORE = 0x0144,
  PROM_DOM_KEY_DIAGNOSTICS_PACKED4_SELECTED_LAYOUT_FORMAT = 0x0504,
  PROM_DOM_KEY_DIAGNOSTICS_PACKED4_TAIL_COUNT_LAST = 0x0505,
  PROM_DOM_KEY_DIAGNOSTICS_PACKED4_TAIL_COUNT_TOTAL = 0x0506,
  PROM_DOM_KEY_DIAGNOSTICS_PACKED4_PADDED_LANE_COUNT_LAST = 0x0507,
  PROM_DOM_KEY_DIAGNOSTICS_PACKED4_PADDED_LANE_COUNT_TOTAL = 0x0508,
  PROM_DOM_KEY_DIAGNOSTICS_PACKED4_PADDING_WASTE_PERMILLE_LAST = 0x0509,
  PROM_DOM_KEY_DIAGNOSTICS_PACKED4_MODE_BUDGET_DENIALS = 0x050A,
  PROM_DOM_KEY_DIAGNOSTICS_PACKED4_ROW_MAJOR_CHECK_FAILURES = 0x050B,
  PROM_DOM_KEY_DIAGNOSTICS_PACKED4_SELECTION_COUNT = 0x050C,
  PROM_DOM_KEY_DIAGNOSTICS_PACKED4_FALLBACK_REASON_PADDING_WASTE = 0x050D,
  PROM_DOM_KEY_DIAGNOSTICS_PACKED4_FALLBACK_REASON_SMALL_SHAPE = 0x050E,
  PROM_DOM_KEY_DIAGNOSTICS_PACKED4_FALLBACK_REASON_CAPABILITY_MISSING = 0x050F,
  PROM_DOM_KEY_DIAGNOSTICS_PACKED4_FALLBACK_REASON_FALLBACK_REQUIRED = 0x0510,
  PROM_DOM_KEY_DIAGNOSTICS_PACKED4_FALLBACK_REASON_MODE_BUDGET_DENIED = 0x0511,
  PROM_DOM_KEY_DIAGNOSTICS_FP16_MAX_ABSOLUTE_ERROR_BITS = 0x0512,
  PROM_DOM_KEY_DIAGNOSTICS_FP16_MAX_RELATIVE_ERROR_BITS = 0x0513,
  PROM_DOM_KEY_DIAGNOSTICS_FP16_AGGREGATE_ERROR_BITS = 0x0514,
  PROM_DOM_KEY_DIAGNOSTICS_FP16_WORST_CASE_ELEMENT_INDEX = 0x0515,
  PROM_DOM_KEY_DIAGNOSTICS_FP16_K_ERROR_GROWTH_BITS = 0x0516,
  PROM_DOM_KEY_DIAGNOSTICS_FP16_CANCELLATION_RISK_BITS = 0x0517,
  PROM_DOM_KEY_DIAGNOSTICS_FP16_TOLERANCE_KNOWN = 0x0518,
  PROM_DOM_KEY_DIAGNOSTICS_FP16_TOLERANCE_PASS = 0x0519,
  PROM_DOM_KEY_DIAGNOSTICS_FP16_FALLBACK_REASON_DETAIL = 0x051A,
  PROM_DOM_KEY_DIAGNOSTICS_FP16_SELECTED_CANDIDATE = 0x051B,

  PROM_DOM_KEY_SLOT_STATE = 0x0201,
  PROM_DOM_KEY_SLOT_GENERATION = 0x0202,
  PROM_DOM_KEY_SLOT_VALID = 0x0203,
  PROM_DOM_KEY_SLOT_CURRENT_ID = 0x0204,
  PROM_DOM_KEY_SLOT_NEXT_ID = 0x0205,
  PROM_DOM_KEY_SLOT_FAILURE_REASON = 0x0206,

  PROM_DOM_KEY_QUEUE_COMPUTE_FAMILY = 0x0301,
  PROM_DOM_KEY_QUEUE_TRANSFER_FAMILY = 0x0302,
  PROM_DOM_KEY_QUEUE_DEDICATED_AVAILABLE = 0x0303,
  PROM_DOM_KEY_QUEUE_TRANSFER_POLICY = 0x0304,
  PROM_DOM_KEY_QUEUE_HANDOFF_COUNT = 0x0305,
  PROM_DOM_KEY_QUEUE_FAMILIES_DIFFER = 0x0306,
  PROM_DOM_KEY_QUEUE_TRANSFER_SUPPORTED = 0x0307,
  PROM_DOM_KEY_QUEUE_TRANSFER_DISABLED_BY_CONFIG = 0x0308,
  PROM_DOM_KEY_QUEUE_TRANSFER_WORKLOAD_LARGE_ENOUGH = 0x0309,
  PROM_DOM_KEY_QUEUE_TRANSFER_SYNC_OWNERSHIP_SUPPORTED = 0x030A,
  PROM_DOM_KEY_QUEUE_TRANSFER_FALLBACK_AVAILABLE = 0x030B,
  PROM_DOM_KEY_QUEUE_TRANSFER_UPLOAD_ONLY_ELIGIBLE = 0x030C,
  PROM_DOM_KEY_QUEUE_TRANSFER_UPLOAD_READBACK_SUPPORTED = 0x030D,
  PROM_DOM_KEY_QUEUE_TRANSFER_POLICY_SELECTED = 0x030E,
  PROM_DOM_KEY_QUEUE_TRANSFER_FALLBACK_REASON = 0x030F,
  PROM_DOM_KEY_QUEUE_TRANSFER_QUEUE_USED = 0x0310,
  PROM_DOM_KEY_QUEUE_TRANSFER_COMPUTE_WAIT_COUNT = 0x0311,
  PROM_DOM_KEY_QUEUE_TRANSFER_FAILURE_SLOT_ID = 0x0312,
  PROM_DOM_KEY_QUEUE_TRANSFER_FAILURE_REASON = 0x0313,
  PROM_DOM_KEY_QUEUE_TRANSFER_FAILURE_COUNT = 0x0314,
  PROM_DOM_KEY_QUEUE_ASYNC_TRANSFER_COMPLETE = 0x0315,
  PROM_DOM_KEY_QUEUE_ASYNC_TRANSFER_COMPLETION_GENERATION = 0x0316,

  PROM_DOM_KEY_MEMORY_REQUIRED_CAPACITY = 0x0401,
  PROM_DOM_KEY_MEMORY_BUDGET = 0x0402,
  PROM_DOM_KEY_MEMORY_HEADROOM = 0x0403,
  PROM_DOM_KEY_MEMORY_INVALIDATION_FLAGS = 0x0404,
  PROM_DOM_KEY_MEMORY_M35_FIXED_HEADROOM = 0x0405,
  PROM_DOM_KEY_MEMORY_M35_PULL_LAG_HEADROOM = 0x0406,
  PROM_DOM_KEY_MEMORY_M35_SERIAL_HEADROOM = 0x0407,
  PROM_DOM_KEY_MEMORY_M35_REQUIRED_FIXED = 0x0408,
  PROM_DOM_KEY_MEMORY_M35_REQUIRED_PULL_LAG = 0x0409,
  PROM_DOM_KEY_MEMORY_M35_REQUIRED_SERIAL = 0x040A,

  PROM_DOM_KEY_DIAGNOSTICS_REASON_CODE = 0x0501,
  PROM_DOM_KEY_DIAGNOSTICS_COUNTER = 0x0502,
  PROM_DOM_KEY_DIAGNOSTICS_LAST_TRANSITION = 0x0503,

  PROM_DOM_KEY_FFT_RESERVED = 0x0601,
} prom_dom_key;

typedef enum prom_dom_value_type {
  PROM_DOM_VALUE_UNSET = 0,
  PROM_DOM_VALUE_U32 = 1,
  PROM_DOM_VALUE_U64 = 2,
  PROM_DOM_VALUE_I32 = 3,
  PROM_DOM_VALUE_I64 = 4,
  PROM_DOM_VALUE_BOOL = 5,
} prom_dom_value_type;

typedef enum prom_dom_source {
  PROM_DOM_SOURCE_UNKNOWN = 0,
  PROM_DOM_SOURCE_REACTOR = 1,
  PROM_DOM_SOURCE_JUDGMENT = 2,
  PROM_DOM_SOURCE_SLOT_HFSM = 3,
  PROM_DOM_SOURCE_POLICY = 4,
  PROM_DOM_SOURCE_QUEUE = 5,
  PROM_DOM_SOURCE_MEMORY = 6,
  PROM_DOM_SOURCE_DIAGNOSTICS = 7,
} prom_dom_source;

typedef enum prom_dom_event_kind {
  PROM_DOM_EVENT_NONE = 0,
  PROM_DOM_EVENT_SLOT_PREPARED = 1,
  PROM_DOM_EVENT_SLOT_READY = 2,
  PROM_DOM_EVENT_SLOT_SUBMITTED = 3,
  PROM_DOM_EVENT_SLOT_COMPLETE = 4,
  PROM_DOM_EVENT_SLOT_FAILED = 5,
  PROM_DOM_EVENT_TRANSFER_COMPLETE = 6,
  PROM_DOM_EVENT_QUEUE_HANDOFF = 7,
  PROM_DOM_EVENT_POLICY_SELECTED_MODE = 8,
  PROM_DOM_EVENT_FALLBACK_EMITTED = 9,
  PROM_DOM_EVENT_SLOT_PROMOTED_CURRENT = 10,
  PROM_DOM_EVENT_SLOT_CONSUMED = 11,
  PROM_DOM_EVENT_SLOT_CLEANUP = 12,
  PROM_DOM_EVENT_SLOT_INVALIDATED = 13,
  PROM_DOM_EVENT_TRANSFER_FAILED = 14,
  PROM_DOM_EVENT_TRANSFER_WAIT = 15,
} prom_dom_event_kind;

typedef struct prom_dom_value {
  prom_dom_value_type type;
  union {
    uint32_t u32;
    uint64_t u64;
    int32_t i32;
    int64_t i64;
    uint32_t boolean;
  } data;
} prom_dom_value;

typedef struct prom_dom_event {
  uint64_t generation;
  uint64_t sequence;
  prom_dom_event_kind kind;
  prom_dom_source source;
  prom_dom_domain domain;
  prom_dom_key key;
  uint32_t slot_id;
  int32_t reason_code;
} prom_dom_event;

typedef struct prom_dom_trace_entry {
  uint64_t generation;
  uint64_t sequence;
  prom_dom_source source;
  prom_dom_domain domain;
  prom_dom_key key;
  prom_dom_event_kind event_kind;
  uint32_t slot_id;
  int32_t reason_code;
  prom_dom_value old_value;
  prom_dom_value new_value;
} prom_dom_trace_entry;

typedef struct prom_dom_blackboard {
  prom_dom_value visible_values[PROM_DOM_STORAGE_CAPACITY];
  prom_dom_value staged_values[PROM_DOM_STORAGE_CAPACITY];
  uint64_t visible_generation;
  uint64_t staged_generation;
  uint64_t sequence_counter;
  uint64_t dirty_keys_staged[PROM_DOM_KEY_WORDS];
  uint64_t dirty_keys_last_commit[PROM_DOM_KEY_WORDS];
  uint32_t dirty_domains_staged;
  uint32_t dirty_domains_last_commit;
  uint32_t dirty_slots_staged;
  uint32_t dirty_slots_last_commit;
  prom_dom_event staged_events[PROM_DOM_MAX_EVENTS];
  uint32_t staged_event_count;
  prom_dom_event committed_events[PROM_DOM_MAX_EVENTS];
  uint32_t committed_event_start;
  uint32_t committed_event_count;
  prom_dom_trace_entry trace_ring[PROM_DOM_MAX_TRACE];
  uint32_t trace_start;
  uint32_t trace_count;
} prom_dom_blackboard;

void prom_dom_blackboard_init(prom_dom_blackboard* board);
void prom_dom_blackboard_reset(prom_dom_blackboard* board);

uint32_t prom_dom_set_u32(prom_dom_blackboard* board,
                          prom_dom_source source,
                          prom_dom_key key,
                          uint32_t slot_id,
                          uint32_t value,
                          int32_t reason_code);
uint32_t prom_dom_set_u64(prom_dom_blackboard* board,
                          prom_dom_source source,
                          prom_dom_key key,
                          uint32_t slot_id,
                          uint64_t value,
                          int32_t reason_code);
uint32_t prom_dom_set_i32(prom_dom_blackboard* board,
                          prom_dom_source source,
                          prom_dom_key key,
                          uint32_t slot_id,
                          int32_t value,
                          int32_t reason_code);
uint32_t prom_dom_set_i64(prom_dom_blackboard* board,
                          prom_dom_source source,
                          prom_dom_key key,
                          uint32_t slot_id,
                          int64_t value,
                          int32_t reason_code);
uint32_t prom_dom_set_bool(prom_dom_blackboard* board,
                           prom_dom_source source,
                           prom_dom_key key,
                           uint32_t slot_id,
                           uint32_t value,
                           int32_t reason_code);

uint32_t prom_dom_get_u32(const prom_dom_blackboard* board, prom_dom_key key, uint32_t slot_id, uint32_t* out_value);
uint32_t prom_dom_get_u64(const prom_dom_blackboard* board, prom_dom_key key, uint32_t slot_id, uint64_t* out_value);
uint32_t prom_dom_get_i32(const prom_dom_blackboard* board, prom_dom_key key, uint32_t slot_id, int32_t* out_value);
uint32_t prom_dom_get_i64(const prom_dom_blackboard* board, prom_dom_key key, uint32_t slot_id, int64_t* out_value);
uint32_t prom_dom_get_bool(const prom_dom_blackboard* board, prom_dom_key key, uint32_t slot_id, uint32_t* out_value);

uint32_t prom_dom_stage_event(prom_dom_blackboard* board, const prom_dom_event* event);
void prom_dom_commit(prom_dom_blackboard* board);

uint64_t prom_dom_dirty_keys_staged_word(const prom_dom_blackboard* board, uint32_t word_index);
uint64_t prom_dom_dirty_keys_last_commit_word(const prom_dom_blackboard* board, uint32_t word_index);
uint32_t prom_dom_dirty_key_staged(const prom_dom_blackboard* board, prom_dom_key key);
uint32_t prom_dom_dirty_key_last_commit(const prom_dom_blackboard* board, prom_dom_key key);
uint32_t prom_dom_dirty_domains_staged(const prom_dom_blackboard* board);
uint32_t prom_dom_dirty_domains_last_commit(const prom_dom_blackboard* board);
uint32_t prom_dom_dirty_slots_staged(const prom_dom_blackboard* board);
uint32_t prom_dom_dirty_slots_last_commit(const prom_dom_blackboard* board);

uint32_t prom_dom_staged_event_count(const prom_dom_blackboard* board);
uint32_t prom_dom_committed_event_count(const prom_dom_blackboard* board);
uint32_t prom_dom_committed_event_at(const prom_dom_blackboard* board, uint32_t index, prom_dom_event* out_event);

uint32_t prom_dom_trace_count(const prom_dom_blackboard* board);
uint32_t prom_dom_trace_at(const prom_dom_blackboard* board, uint32_t index, prom_dom_trace_entry* out_entry);

#ifdef __cplusplus
}
#endif

#endif
