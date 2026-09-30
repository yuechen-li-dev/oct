#pragma once
/* Test-side compositions of production control steps.
 *
 * Production calls the individual steps directly; these helpers only exist so
 * tests can drive a whole decision in one call. They are not part of the
 * reactor and must not be linked into it. */

#include "../reactor_judgment_engine.h"
#include "../reactor_dominatus_predictor.h"
#include "../reactor_dominatus_sgemm_adapter.h"

static inline void prom_judgment_engine_select_sgemm_mode(const prom_judgment_facts* facts,
                                                          prom_judgment_decision* out_decision) {
  prom_judgment_layout_precision_decision layout_precision_decision;
  prom_judgment_engine_select_layout_precision(facts, &layout_precision_decision);
  prom_judgment_engine_select_sgemm_mode_with_layout_precision(facts, &layout_precision_decision,
                                                               out_decision);
}

static inline prom_dominatus_shadow_authority_gate prom_dominatus_shadow_authority_gate_evaluate(
    const prom_dominatus_shadow_calibration_state* calibration) {
  return prom_dominatus_shadow_authority_gate_evaluate_with_enabled(calibration, 0u);
}

static inline uint32_t prom_dom_sgemm_stage_m35(prom_dom_blackboard* board,
                                                const prom_buffering_selector_facts* facts,
                                                const prom_buffering_selector_decision* decision) {
  if (prom_dom_sgemm_stage_m35_facts(board, facts) == 0u) {
    return 0u;
  }
  return prom_dom_sgemm_stage_m35_decision(board, decision, 0u);
}
