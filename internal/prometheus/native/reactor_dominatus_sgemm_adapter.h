#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_DOMINATUS_SGEMM_ADAPTER_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_DOMINATUS_SGEMM_ADAPTER_H

#include <stdint.h>

#include "reactor_dominatus_blackboard.h"
#include "reactor_judgment_engine.h"

#ifdef __cplusplus
extern "C" {
#endif

typedef struct prom_dom_sgemm_m35_snapshot {
  uint32_t selected_mode;
  uint32_t fixed_feasible;
  uint32_t pull_lag_feasible;
  uint32_t serial_feasible;
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
} prom_dom_sgemm_m35_snapshot;

uint32_t prom_dom_sgemm_stage_m35(prom_dom_blackboard* board,
                                  const prom_buffering_selector_facts* facts,
                                  const prom_buffering_selector_decision* decision);

void prom_dom_sgemm_commit(prom_dom_blackboard* board);

uint32_t prom_dom_sgemm_read_visible_m35(const prom_dom_blackboard* board, prom_dom_sgemm_m35_snapshot* out_snapshot);

#ifdef __cplusplus
}
#endif

#endif
