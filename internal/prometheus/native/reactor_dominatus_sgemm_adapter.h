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
} prom_dom_transfer_queue_snapshot;

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

#ifdef __cplusplus
}
#endif

#endif
