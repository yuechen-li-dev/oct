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

typedef enum prom_occupancy_device_band {
  PROM_OCCUPANCY_DEVICE_BAND_REGISTER_CONSTRAINED = 1,
  PROM_OCCUPANCY_DEVICE_BAND_BALANCED = 2,
  PROM_OCCUPANCY_DEVICE_BAND_COMPUTE_RICH = 3,
  PROM_OCCUPANCY_DEVICE_BAND_MEMORY_RICH = 4,
} prom_occupancy_device_band;

typedef enum prom_occupancy_shape_class {
  PROM_OCCUPANCY_SHAPE_CLASS_SMALL_SQUARE = 1,
  PROM_OCCUPANCY_SHAPE_CLASS_MEDIUM_SQUARE = 2,
  PROM_OCCUPANCY_SHAPE_CLASS_LARGE_SQUARE = 3,
  PROM_OCCUPANCY_SHAPE_CLASS_TALL_SKINNY = 4,
  PROM_OCCUPANCY_SHAPE_CLASS_WIDE_SHORT = 5,
  PROM_OCCUPANCY_SHAPE_CLASS_K_HEAVY = 6,
  PROM_OCCUPANCY_SHAPE_CLASS_ML_FFN_LIKE = 7,
} prom_occupancy_shape_class;

typedef enum prom_occupancy_kernel_variant {
  PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR = 1,
  PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE = 2,
  PROM_OCCUPANCY_KERNEL_VARIANT_SMALL_REGISTER_TILE = 3,
  PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4 = 4,
  PROM_OCCUPANCY_KERNEL_VARIANT_AGGRESSIVE_4X4_ACCUM8 = 5,
  PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_SCALAR_PLUS = 6,
  PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_TILE16X16_SHARED_FP32 = 7,
  PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FP32 = 8,
  PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32 = 9,
  PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FLOWBOARD_FP32 = 10,
} prom_occupancy_kernel_variant;

typedef enum prom_occupancy_reason_code {
  PROM_OCCUPANCY_REASON_NONE = 0,
  PROM_OCCUPANCY_REASON_DEFAULT_BAND_SELECTION = 1,
  PROM_OCCUPANCY_REASON_MANUAL_OVERRIDE_USED = 2,
  PROM_OCCUPANCY_REASON_LOW_REGISTER_CLAMP = 3,
  PROM_OCCUPANCY_REASON_SHARED_MEMORY_CLAMP = 4,
  PROM_OCCUPANCY_REASON_SHAPE_SMALL_CLAMP = 5,
  PROM_OCCUPANCY_REASON_FALLBACK_BASELINE = 6,
  PROM_OCCUPANCY_REASON_UNKNOWN_DEVICE_FALLBACK = 7,
  PROM_OCCUPANCY_REASON_OVERRIDE_REJECTED = 8,
} prom_occupancy_reason_code;

typedef struct prom_occupancy_selector_facts {
  uint32_t register_file_class;
  uint32_t shared_memory_class;
  uint32_t memory_bandwidth_class;
  uint32_t fp32_throughput_class;
  uint32_t max_workgroup_class;
  uint32_t queue_capability_class;
  uint32_t has_exact_profile;
  uint32_t manual_override_variant;
  uint32_t manual_override_enabled;
  uint32_t m;
  uint32_t n;
  uint32_t k;
  uint64_t work_units;
} prom_occupancy_selector_facts;

typedef struct prom_occupancy_selector_decision {
  uint32_t success;
  uint32_t device_band;
  uint32_t shape_class;
  uint32_t selected_variant;
  uint32_t unclamped_variant;
  uint32_t clamp_reason;
  uint32_t override_used;
  uint32_t fallback_used;
} prom_occupancy_selector_decision;

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

typedef enum prom_lease_resource_class {
  PROM_LEASE_RESOURCE_CLASS_COMPUTE = 1,
  PROM_LEASE_RESOURCE_CLASS_TRANSFER = 2,
  PROM_LEASE_RESOURCE_CLASS_MEMORY_BANDWIDTH = 3,
} prom_lease_resource_class;

typedef enum prom_lease_state {
  PROM_LEASE_STATE_NONE = 1,
  PROM_LEASE_STATE_REQUESTED = 2,
  PROM_LEASE_STATE_GRANTED = 3,
  PROM_LEASE_STATE_HELD = 4,
  PROM_LEASE_STATE_YIELDED = 5,
  PROM_LEASE_STATE_DENIED = 6,
  PROM_LEASE_STATE_FAILED = 7,
} prom_lease_state;

typedef enum prom_lease_reason_code {
  PROM_LEASE_REASON_NONE = 0,
  PROM_LEASE_REASON_HARD_DENY_SAFETY_OR_CAP = 1,
  PROM_LEASE_REASON_HARD_YIELD_CRITICAL_SECTION_COMPLETE = 2,
  PROM_LEASE_REASON_NO_YIELD_WITHOUT_HELD_LEASE = 3,
  PROM_LEASE_REASON_HARD_BLOCK_LOOKAHEAD_LIMIT_OR_TRANSFER = 4,
  PROM_LEASE_REASON_UTILITY_GRANT_READY_AND_SAFE = 5,
  PROM_LEASE_REASON_UTILITY_BACKPRESSURE_PRESSURE_OR_CONTENTION = 6,
  PROM_LEASE_REASON_UTILITY_BACKPRESSURE_DEFAULT = 7,
  PROM_LEASE_REASON_UTILITY_ALLOW_LOOKAHEAD_LATENCY_DOMINANT = 8,
  PROM_LEASE_REASON_FAILED = 9,
  PROM_LEASE_REASON_GRANTED = 20,
  PROM_LEASE_REASON_DENIED_OUTSTANDING_LIMIT = 21,
  PROM_LEASE_REASON_DENIED_UNSAFE_RUNTIME = 22,
  PROM_LEASE_REASON_DENIED_SLOT_INVALIDATED = 23,
  PROM_LEASE_REASON_DENIED_SLOT_FAILED = 24,
  PROM_LEASE_REASON_DENIED_RESOURCE_PRESSURE = 25,
  PROM_LEASE_REASON_DENIED_TRANSFER_UNAVAILABLE = 26,
  PROM_LEASE_REASON_YIELDED = 27,
} prom_lease_reason_code;

typedef struct prom_resource_lease_facts {
  uint32_t worker_id;
  uint32_t slot_id;
  uint32_t entry_id;
  uint32_t shape_class;
  uint32_t device_band;
  uint32_t selected_recipe_variant;
  uint32_t requested_resource_class;
  uint32_t register_pressure_class;
  uint32_t shared_memory_pressure_class;
  uint32_t memory_bandwidth_pressure_class;
  uint32_t compute_pressure_class;
  uint32_t pipeline_latency_pressure_class;
  uint32_t current_outstanding_depth;
  uint32_t max_outstanding_depth;
  uint32_t lookahead_requested;
  uint32_t lookahead_limit;
  uint32_t slot_attention_mask;
  uint32_t ready_slot_mask;
  uint32_t failed_slot_mask;
  uint32_t invalidated_slot_mask;
  uint32_t transfer_overlap_available;
  uint32_t true_multi_queue_selected;
  uint32_t unsafe_to_reuse;
  uint32_t yield_requested;
  uint32_t lease_held;
  uint32_t single_call_mode;
} prom_resource_lease_facts;

typedef struct prom_resource_lease_decision {
  uint32_t success;
  uint32_t lease_state;
  uint32_t grant;
  uint32_t deny_reason;
  uint32_t resource_class;
  uint32_t worker_id;
  uint32_t slot_id;
  uint32_t entry_id;
  uint32_t allowed_outstanding_depth;
  uint32_t lookahead_allowed;
  uint32_t backpressure_applied;
  uint32_t yield_required;
  uint32_t selected_recipe_variant;
  int32_t detail;
} prom_resource_lease_decision;

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
void prom_judgment_engine_select_occupancy_variant(const prom_occupancy_selector_facts* facts,
                                                   prom_occupancy_selector_decision* out_decision);
void prom_judgment_engine_decide_resource_lease(const prom_resource_lease_facts* facts,
                                                prom_resource_lease_decision* out_decision);

#ifdef __cplusplus
}
#endif

#endif
