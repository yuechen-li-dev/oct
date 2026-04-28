#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_DOMINATUS_SGEMM_ADAPTER_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_DOMINATUS_SGEMM_ADAPTER_H

#include <stdint.h>

#include "reactor_dominatus_blackboard.h"
#include "reactor_judgment_engine.h"

#ifdef __cplusplus
extern "C" {
#endif

typedef struct prom_dom_sgemm_m35_snapshot {
  uint32_t success;
  uint32_t selected_mode;
  uint32_t fixed_feasible;
  uint32_t pull_lag_feasible;
  uint32_t serial_feasible;
  uint32_t fixed_rejected;
  uint32_t pull_lag_rejected;
  uint32_t serial_rejected;
  uint32_t no_feasible_mode_detail;
  int32_t fixed_score;
  int32_t pull_lag_score;
  int32_t serial_score;
  uint32_t reason_code;
  uint32_t final_reason_code;
  uint32_t fixed_double_rejection_reason;
  uint32_t pull_lag_rejection_reason;
  uint32_t serial_jit_rejection_reason;
  int32_t fixed_double_headroom_slots_permille;
  int32_t pull_lag_headroom_slots_permille;
  int32_t serial_jit_headroom_slots_permille;
  uint64_t memory_budget_slots_permille;
  uint32_t required_fixed_slots_permille;
  uint32_t required_pull_lag_peak_slots_permille;
  uint32_t required_serial_slots_permille;
  uint32_t transfer_variance_class;
  uint32_t compute_predictability_class;
  uint32_t starvation_risk_high;
  uint32_t pull_lag_wip_waste_exceeded;
  uint32_t fallback_available;
} prom_dom_sgemm_m35_snapshot;

typedef struct prom_dom_sgemm_buffering_projection {
  prom_buffering_selector_facts facts;
  uint64_t visible_generation;
  uint64_t dependent_dirty_key_mask_last_commit;
  uint32_t from_visible_snapshot;
} prom_dom_sgemm_buffering_projection;

typedef struct prom_dom_transfer_queue_facts {
  uint32_t dedicated_transfer_available;
  uint32_t transfer_queue_family_index;
  uint32_t compute_queue_family_index;
  uint32_t queue_families_differ;
  uint32_t transfer_queue_supported;
  uint32_t transfer_queue_disabled_by_config;
  uint32_t transfer_workload_large_enough;
  uint32_t transfer_sync_ownership_supported;
  uint32_t transfer_fallback_available;
  uint32_t upload_only_policy_eligible;
  uint32_t upload_readback_supported;
} prom_dom_transfer_queue_facts;

typedef struct prom_dom_transfer_queue_projection {
  prom_dom_transfer_queue_facts facts;
  uint64_t visible_generation;
  uint64_t dependent_dirty_key_mask_last_commit;
  uint32_t from_visible_snapshot;
} prom_dom_transfer_queue_projection;

typedef struct prom_dom_transfer_queue_decision {
  uint32_t transfer_policy_selected;
  uint32_t selected_transfer_policy;
  uint32_t transfer_queue_used;
  uint32_t transfer_fallback_reason;
} prom_dom_transfer_queue_decision;

typedef struct prom_dom_transfer_queue_snapshot {
  uint32_t transfer_policy_selected;
  uint32_t selected_transfer_policy;
  uint32_t transfer_queue_used;
  uint32_t transfer_fallback_reason;
  uint32_t dedicated_transfer_available;
  uint32_t transfer_queue_family_index;
  uint32_t compute_queue_family_index;
  uint32_t queue_families_differ;
  uint32_t transfer_queue_supported;
  uint32_t upload_only_policy_eligible;
  uint32_t upload_readback_supported;
  uint64_t queue_family_handoff_count;
  uint64_t transfer_compute_wait_count;
  int32_t transfer_failure_slot_id;
  int32_t transfer_failure_reason;
  uint64_t transfer_failure_count;
  uint32_t async_transfer_complete;
  uint64_t async_transfer_completion_generation;
} prom_dom_transfer_queue_snapshot;

typedef struct prom_dom_transfer_runtime_telemetry {
  uint64_t queue_family_handoff_count;
  uint64_t transfer_compute_wait_count;
  int32_t transfer_failure_slot_id;
  int32_t transfer_failure_reason;
  uint64_t transfer_failure_count;
  uint32_t async_transfer_complete;
  uint64_t async_transfer_completion_generation;
} prom_dom_transfer_runtime_telemetry;

typedef struct prom_dom_sgemm_layout_precision_facts {
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
  int32_t fp16_utility_score;
} prom_dom_sgemm_layout_precision_facts;

typedef struct prom_dom_sgemm_layout_precision_projection {
  prom_dom_sgemm_layout_precision_facts facts;
  uint64_t visible_generation;
  uint64_t dependent_dirty_key_mask_last_commit;
  uint32_t from_visible_snapshot;
} prom_dom_sgemm_layout_precision_projection;

typedef struct prom_dom_sgemm_layout_precision_decision {
  uint32_t packed4_selected;
  uint32_t packed4_reject_reason;
  uint32_t fp16_selected;
  uint32_t fp16_reject_reason;
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
  int32_t fp16_fallback_reason_detail;
  uint32_t fp16_selected_candidate;
} prom_dom_sgemm_layout_precision_decision;

typedef struct prom_dom_sgemm_layout_precision_snapshot {
  prom_dom_sgemm_layout_precision_facts facts;
  prom_dom_sgemm_layout_precision_decision decision;
} prom_dom_sgemm_layout_precision_snapshot;

typedef struct prom_dom_sgemm_path_compute_facts {
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
  uint32_t policy_mode;
} prom_dom_sgemm_path_compute_facts;

/*
 * Dependency-bit index contract for prom_dom_sgemm_path_compute_projection::dependent_dirty_key_mask_last_commit.
 * Keep this enum aligned with path_compute_dependency_mask_last_commit(...) in reactor_dominatus_sgemm_adapter.c.
 */
typedef enum prom_dom_sgemm_path_compute_dependency_bit {
  PROM_DOM_PATH_COMPUTE_DEP_SHAPE_M = 0,
  PROM_DOM_PATH_COMPUTE_DEP_SHAPE_N = 1,
  PROM_DOM_PATH_COMPUTE_DEP_SHAPE_K = 2,
  PROM_DOM_PATH_COMPUTE_DEP_WORK_UNITS = 3,
  PROM_DOM_PATH_COMPUTE_DEP_CAN_STAGE = 4,
  PROM_DOM_PATH_COMPUTE_DEP_CAN_DIRECT = 5,
  PROM_DOM_PATH_COMPUTE_DEP_ALLOW_FALLBACK = 6,
  PROM_DOM_PATH_COMPUTE_DEP_READBACK_REQUIRED = 7,
  PROM_DOM_PATH_COMPUTE_DEP_FORCE_DIRECT = 8,
  PROM_DOM_PATH_COMPUTE_DEP_FORCE_STAGED = 9,
  PROM_DOM_PATH_COMPUTE_DEP_FORCE_TILED = 10,
  PROM_DOM_PATH_COMPUTE_DEP_TILED_SHAPE = 11,
  PROM_DOM_PATH_COMPUTE_DEP_SOFTWARE_VULKAN = 12,
  PROM_DOM_PATH_COMPUTE_DEP_POLICY_MODE = 13,
} prom_dom_sgemm_path_compute_dependency_bit;

typedef struct prom_dom_sgemm_path_compute_projection {
  prom_dom_sgemm_path_compute_facts facts;
  uint64_t visible_generation;
  uint64_t dependent_dirty_key_mask_last_commit;
  uint32_t from_visible_snapshot;
} prom_dom_sgemm_path_compute_projection;

typedef struct prom_dom_sgemm_path_compute_decision {
  uint32_t success;
  int32_t error_detail;
  uint32_t requested_path;
  uint32_t selected_path;
  uint32_t compute_mode;
  int32_t final_detail;
  uint32_t used_fallback_to_direct;
  uint32_t winning_candidate_index;
  int32_t winning_score;
} prom_dom_sgemm_path_compute_decision;

typedef struct prom_dom_sgemm_path_compute_snapshot {
  prom_dom_sgemm_path_compute_facts facts;
  prom_dom_sgemm_path_compute_decision decision;
  uint32_t packed4_selected;
  uint32_t packed4_reject_reason;
  uint32_t fp16_selected;
  uint32_t fp16_reject_reason;
  uint32_t use_dedicated_transfer_queue_upload;
  uint32_t transfer_fallback_reason;
} prom_dom_sgemm_path_compute_snapshot;

typedef struct prom_dom_async_snapshot {
  int32_t task_id;
  uint32_t lifecycle_state;
  uint32_t stage;
  int32_t detail_code;
  uint32_t ready;
  uint32_t failed;
  uint32_t consumed;
  uint32_t outstanding_tasks;
  uint32_t failure_stage;
  int32_t failure_detail;
  int32_t submit_detail;
  int32_t query_detail;
  int32_t slot_id;
  uint64_t slot_generation;
  uint32_t owns_slot;
  uint32_t transfer_complete;
  uint32_t compute_complete;
  uint32_t readback_complete;
} prom_dom_async_snapshot;

typedef struct prom_dom_sgemm_resource_lease_projection {
  prom_resource_lease_facts facts;
  uint64_t visible_generation;
  uint64_t dependent_dirty_key_mask_last_commit;
  uint32_t from_visible_snapshot;
} prom_dom_sgemm_resource_lease_projection;

typedef struct prom_dom_sgemm_resource_lease_snapshot {
  prom_resource_lease_facts facts;
  prom_resource_lease_decision decision;
  uint64_t granted_count;
  uint64_t denied_count;
  uint64_t backpressure_count;
  uint64_t yield_count;
  uint64_t failed_count;
  uint32_t lookahead_blocked_reason;
} prom_dom_sgemm_resource_lease_snapshot;

uint32_t prom_dom_sgemm_stage_m35(prom_dom_blackboard* board,
                                  const prom_buffering_selector_facts* facts,
                                  const prom_buffering_selector_decision* decision);
uint32_t prom_dom_sgemm_stage_m35_facts(prom_dom_blackboard* board, const prom_buffering_selector_facts* facts);
uint32_t prom_dom_sgemm_stage_m35_decision(prom_dom_blackboard* board,
                                           const prom_buffering_selector_decision* decision,
                                           uint32_t no_feasible_mode_detail);

void prom_dom_sgemm_commit(prom_dom_blackboard* board);

uint32_t prom_dom_sgemm_read_visible_m35(const prom_dom_blackboard* board, prom_dom_sgemm_m35_snapshot* out_snapshot);

uint32_t prom_dom_sgemm_build_buffering_selector_facts_from_visible(
    const prom_dom_blackboard* board,
    const prom_buffering_selector_facts* fallback_facts,
    prom_dom_sgemm_buffering_projection* out_projection);

uint32_t prom_dom_sgemm_stage_transfer_queue_facts(prom_dom_blackboard* board, const prom_dom_transfer_queue_facts* facts);
uint32_t prom_dom_sgemm_build_transfer_queue_facts_from_visible(const prom_dom_blackboard* board,
                                                                const prom_dom_transfer_queue_facts* fallback_facts,
                                                                prom_dom_transfer_queue_projection* out_projection);
uint32_t prom_dom_sgemm_stage_transfer_queue_decision(prom_dom_blackboard* board,
                                                      const prom_dom_transfer_queue_decision* decision);
uint32_t prom_dom_sgemm_read_visible_transfer_queue_diagnostics(const prom_dom_blackboard* board,
                                                                prom_dom_transfer_queue_snapshot* out_snapshot);
uint32_t prom_dom_sgemm_stage_transfer_handoff(prom_dom_blackboard* board,
                                               uint64_t handoff_count,
                                               uint32_t slot_id,
                                               int32_t reason_code);
uint32_t prom_dom_sgemm_stage_transfer_wait(prom_dom_blackboard* board,
                                            uint64_t wait_count,
                                            uint32_t slot_id,
                                            int32_t reason_code);
uint32_t prom_dom_sgemm_stage_transfer_failure(prom_dom_blackboard* board,
                                               int32_t slot_id,
                                               int32_t reason_code,
                                               uint64_t failure_count);
uint32_t prom_dom_sgemm_stage_transfer_complete(prom_dom_blackboard* board,
                                                uint32_t async_transfer_complete,
                                                uint64_t completion_generation,
                                                uint32_t slot_id,
                                                int32_t reason_code);
uint32_t prom_dom_sgemm_read_visible_transfer_runtime_telemetry(const prom_dom_blackboard* board,
                                                                prom_dom_transfer_runtime_telemetry* out_snapshot);
uint32_t prom_dom_sgemm_stage_layout_precision_facts(prom_dom_blackboard* board,
                                                     const prom_dom_sgemm_layout_precision_facts* facts);
uint32_t prom_dom_sgemm_build_layout_precision_facts_from_visible(
    const prom_dom_blackboard* board,
    const prom_dom_sgemm_layout_precision_facts* fallback_facts,
    prom_dom_sgemm_layout_precision_projection* out_projection);
uint32_t prom_dom_sgemm_stage_layout_precision_decision(prom_dom_blackboard* board,
                                                        const prom_dom_sgemm_layout_precision_decision* decision);
uint32_t prom_dom_sgemm_read_visible_layout_precision_diagnostics(const prom_dom_blackboard* board,
                                                                  prom_dom_sgemm_layout_precision_snapshot* out_snapshot);
uint32_t prom_dom_sgemm_stage_path_compute_facts(prom_dom_blackboard* board,
                                                 const prom_dom_sgemm_path_compute_facts* facts);
uint32_t prom_dom_sgemm_build_path_compute_facts_from_visible(
    const prom_dom_blackboard* board,
    const prom_dom_sgemm_path_compute_facts* fallback_facts,
    prom_dom_sgemm_path_compute_projection* out_projection);
uint32_t prom_dom_sgemm_stage_path_compute_decision(prom_dom_blackboard* board,
                                                    const prom_dom_sgemm_path_compute_decision* decision);
uint32_t prom_dom_sgemm_read_visible_path_compute_diagnostics(const prom_dom_blackboard* board,
                                                              prom_dom_sgemm_path_compute_snapshot* out_snapshot);
uint32_t prom_dom_sgemm_stage_async_snapshot(prom_dom_blackboard* board,
                                             const prom_dom_async_snapshot* snapshot,
                                             prom_dom_event_kind event_kind,
                                             int32_t reason_code);
uint32_t prom_dom_sgemm_read_visible_async_snapshot(const prom_dom_blackboard* board, prom_dom_async_snapshot* out_snapshot);
uint32_t prom_dom_sgemm_stage_resource_lease_facts(prom_dom_blackboard* board, const prom_resource_lease_facts* facts);
uint32_t prom_dom_sgemm_build_resource_lease_facts_from_visible(
    const prom_dom_blackboard* board,
    const prom_resource_lease_facts* fallback_facts,
    prom_dom_sgemm_resource_lease_projection* out_projection);
uint32_t prom_dom_sgemm_stage_resource_lease_decision(prom_dom_blackboard* board,
                                                      const prom_resource_lease_decision* decision);
uint32_t prom_dom_sgemm_read_visible_resource_lease_diagnostics(const prom_dom_blackboard* board,
                                                                prom_dom_sgemm_resource_lease_snapshot* out_snapshot);

#ifdef __cplusplus
}
#endif

#endif
