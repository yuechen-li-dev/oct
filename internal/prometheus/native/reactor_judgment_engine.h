#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_JUDGMENT_ENGINE_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_JUDGMENT_ENGINE_H

#include <stdint.h>

#include "reactor_api.h"
#include "reactor_policy_memory.h"

#ifdef __cplusplus
extern "C" {
#endif

typedef enum prom_vk_path_mode {
  PROM_VK_PATH_DIRECT = 1,
  PROM_VK_PATH_STAGED_UPLOAD = 2,
  PROM_VK_PATH_STAGED_UPLOAD_READBACK = 3,
} prom_vk_path_mode;

typedef enum prom_vk_compute_mode {
  PROM_VK_COMPUTE_BASELINE = 1,
  PROM_VK_COMPUTE_TILED = 2,
  PROM_VK_COMPUTE_PACKED4_FP32 = 3,
  PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM = 4,
} prom_vk_compute_mode;

typedef enum prom_packed4_reject_reason {
  PROM_PACKED4_REJECT_NONE = 0,
  PROM_PACKED4_REJECT_PADDING_WASTE = 1,
  PROM_PACKED4_REJECT_SMALL_SHAPE = 2,
  PROM_PACKED4_REJECT_CAPABILITY_MISSING = 3,
  PROM_PACKED4_REJECT_FALLBACK_REQUIRED = 4,
  PROM_PACKED4_REJECT_MODE_BUDGET_DENIED = 5,
} prom_packed4_reject_reason;

typedef enum prom_fp16_reject_reason {
  PROM_FP16_REJECT_NONE = 0,
  PROM_FP16_REJECT_STRICT_FP32 = 1,
  PROM_FP16_REJECT_TOLERANCE_UNKNOWN = 2,
  PROM_FP16_REJECT_TOLERANCE_EXCEEDED = 3,
  PROM_FP16_REJECT_SPECIAL_VALUE = 4,
  PROM_FP16_REJECT_CAPABILITY_MISSING = 5,
  PROM_FP16_REJECT_FALLBACK_REQUIRED = 6,
  PROM_FP16_REJECT_NOT_TOP_UTILITY = 7,
} prom_fp16_reject_reason;

typedef struct prom_judgment_facts {
  uint32_t m;
  uint32_t n;
  uint32_t k;
  uint64_t work_units;
  uint32_t can_stage;
  uint32_t can_direct;
  uint32_t allow_fallback;
  uint32_t readback_required;
  uint32_t force_direct;
  uint32_t force_staged;
  uint32_t force_tiled;
  uint32_t tiled_shape;
  uint32_t software_vulkan;
  prom_policy_mode policy_mode;
  uint32_t packed4_available;
  uint32_t packed4_small_shape;
  uint32_t packed4_padding_waste_permille;
  uint32_t packed4_mode_budget_permille;
  uint32_t packed4_row_major_valid;
  uint32_t packed4_tail_valid;
  uint32_t strict_fp32;
  uint32_t tolerance_known;
  uint32_t tolerance_pass;
  uint32_t has_special_values;
  uint32_t capability_fp16_storage;
  uint32_t fallback_available;
  int fp16_utility_score;
  uint32_t transfer_queue_dedicated_available;
  uint32_t transfer_queue_families_differ;
  uint32_t transfer_queue_supported;
  uint32_t transfer_overlap_slot_valid;
  uint32_t transfer_workload_large_enough;
  uint32_t transfer_fallback_available;
  uint32_t transfer_queue_disabled_by_config;
} prom_judgment_facts;

typedef struct prom_judgment_decision {
  uint32_t success;
  int error_detail;
  prom_vk_path_mode requested_path;
  prom_vk_path_mode selected_path;
  prom_vk_compute_mode compute_mode;
  int final_detail;
  uint32_t used_fallback_to_direct;
  uint32_t winning_candidate_index;
  int winning_score;
  uint32_t packed4_selected;
  prom_packed4_reject_reason packed4_reject_reason;
  uint32_t fp16_selected;
  prom_fp16_reject_reason fp16_reject_reason;
  uint32_t use_dedicated_transfer_queue_upload;
  uint32_t transfer_fallback_reason;
} prom_judgment_decision;

typedef struct prom_judgment_layout_precision_decision {
  uint32_t packed4_selected;
  prom_packed4_reject_reason packed4_reject_reason;
  uint32_t fp16_selected;
  prom_fp16_reject_reason fp16_reject_reason;
} prom_judgment_layout_precision_decision;

typedef enum prom_buffering_mode {
  PROM_BUFFERING_MODE_FIXED_DOUBLE_DEFAULT = 1,
  PROM_BUFFERING_MODE_PULL_LAG_PRESSURE = 2,
  PROM_BUFFERING_MODE_SERIAL_JIT_SURVIVAL = 3,
  PROM_BUFFERING_MODE_NONE = 4,
} prom_buffering_mode;

typedef enum prom_buffering_reason_code {
  PROM_BUFFERING_REASON_NONE = 0,
  PROM_BUFFERING_REASON_FIXED_DOUBLE_SELECTED = 1,
  PROM_BUFFERING_REASON_FIXED_DOUBLE_MEMORY_INSUFFICIENT = 2,
  PROM_BUFFERING_REASON_PULL_LAG_SELECTED = 3,
  PROM_BUFFERING_REASON_PULL_LAG_MEMORY_INSUFFICIENT = 4,
  PROM_BUFFERING_REASON_PULL_LAG_LATE_STAGE_STARVATION = 5,
  PROM_BUFFERING_REASON_PULL_LAG_MEMORY_EDGE_REJECTED = 6,
  PROM_BUFFERING_REASON_PULL_LAG_VARIANCE_MISS = 7,
  PROM_BUFFERING_REASON_PULL_LAG_COMPUTE_UNSTABLE = 8,
  PROM_BUFFERING_REASON_PULL_LAG_WIP_WASTE_EXCEEDED = 9,
  PROM_BUFFERING_REASON_SERIAL_JIT_SELECTED = 10,
  PROM_BUFFERING_REASON_SERIAL_JIT_MEMORY_INSUFFICIENT = 11,
  PROM_BUFFERING_REASON_NO_BUFFERING_MODE_FEASIBLE = 12,
} prom_buffering_reason_code;

typedef enum prom_variance_class {
  PROM_VARIANCE_LOW = 1,
  PROM_VARIANCE_MODERATE = 2,
  PROM_VARIANCE_HIGH = 3,
} prom_variance_class;

typedef enum prom_predictability_class {
  PROM_PREDICTABILITY_STABLE = 1,
  PROM_PREDICTABILITY_TRACKED = 2,
  PROM_PREDICTABILITY_UNSTABLE = 3,
} prom_predictability_class;

typedef struct prom_buffering_selector_facts {
  uint32_t memory_budget_slots_permille;
  uint32_t required_fixed_slots_permille;
  uint32_t required_pull_lag_peak_slots_permille;
  uint32_t required_serial_slots_permille;
  int32_t fixed_double_headroom_slots_permille;
  int32_t pull_lag_headroom_slots_permille;
  int32_t serial_jit_headroom_slots_permille;
  prom_variance_class transfer_variance_class;
  prom_predictability_class compute_predictability_class;
  uint32_t starvation_risk_high;
  uint32_t pull_lag_wip_waste_exceeded;
  uint32_t fallback_available;
} prom_buffering_selector_facts;

typedef struct prom_buffering_selector_decision {
  uint32_t success;
  prom_buffering_mode selected_mode;
  prom_buffering_reason_code reason_code;
  prom_buffering_reason_code final_reason_code;
  prom_buffering_reason_code fixed_double_rejection_reason;
  prom_buffering_reason_code pull_lag_rejection_reason;
  prom_buffering_reason_code serial_jit_rejection_reason;
  uint32_t fixed_feasible;
  uint32_t pull_lag_feasible;
  uint32_t serial_feasible;
  uint32_t fixed_rejected;
  uint32_t pull_lag_rejected;
  uint32_t serial_rejected;
  int fixed_score;
  int pull_lag_score;
  int serial_score;
} prom_buffering_selector_decision;

typedef struct prom_judgment_async_facts {
  uint32_t request_async;
  uint32_t in_flight;
  uint32_t software_vulkan;
} prom_judgment_async_facts;

typedef struct prom_judgment_async_decision {
  uint32_t success;
  uint32_t execute_async;
  int reject_detail;
} prom_judgment_async_decision;

enum {
  PROM_JUDGMENT_STAGING_WORK_THRESHOLD = 16384u,
  PROM_JUDGMENT_TILED_WORK_THRESHOLD = 131072u,
};

void prom_judgment_engine_select_sgemm_mode(const prom_judgment_facts* facts, prom_judgment_decision* out_decision);
void prom_judgment_engine_select_layout_precision(const prom_judgment_facts* facts,
                                                  prom_judgment_layout_precision_decision* out_decision);
void prom_judgment_engine_select_sgemm_mode_with_layout_precision(
    const prom_judgment_facts* facts,
    const prom_judgment_layout_precision_decision* layout_precision_decision,
    prom_judgment_decision* out_decision);
void prom_judgment_engine_select_async_submission(const prom_judgment_async_facts* facts,
                                                  prom_judgment_async_decision* out_decision);
prom_policy_mode prom_judgment_engine_update_policy_mode(prom_policy_memory* memory,
                                                         const prom_policy_facts* facts,
                                                         const prom_policy_thresholds* thresholds);
void prom_judgment_engine_select_buffering_mode(const prom_buffering_selector_facts* facts,
                                                prom_buffering_selector_decision* out_decision);

#ifdef __cplusplus
}
#endif

#endif
