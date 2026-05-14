#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_DOMINATUS_FILTER_POLICY_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_DOMINATUS_FILTER_POLICY_H

#include "reactor_dominatus_filter.h"

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef struct prom_dominatus_measurement_facts {
  uint32_t sample_count;
  double recent_abs_residual;
  double recent_output_variation;
  double spike_rate_estimate;
  double jitter_estimate;
  uint32_t step_change_suspected;
  uint32_t drift_suspected;
  double confidence;
  uint32_t outlier_count;
} prom_dominatus_measurement_facts;

typedef struct prom_dominatus_filter_policy_params {
  uint32_t min_commit_ticks;
  double switch_margin;
  double confidence_threshold;
  prom_dominatus_filter_kind stable_filter;
  prom_dominatus_filter_kind spike_filter;
  prom_dominatus_filter_kind step_filter;
  prom_dominatus_filter_kind drift_filter;
  prom_dominatus_filter_kind mixed_filter;
} prom_dominatus_filter_policy_params;

typedef struct prom_dominatus_filter_policy_state {
  prom_dominatus_filter_policy_params params;
  prom_dominatus_filter_kind current_kind;
  prom_dominatus_filter_state current_filter;
  uint32_t initialized;
  uint32_t min_commit_remaining;
  uint64_t last_switch_tick;
  uint32_t switch_count;
  double last_output;
  double last_confidence;
} prom_dominatus_filter_policy_state;

typedef struct prom_dominatus_filter_decision {
  prom_dominatus_filter_kind selected_kind;
  prom_dominatus_filter_kind previous_kind;
  uint32_t switched;
  uint32_t held_by_min_commit;
  uint32_t held_by_margin;
  uint32_t held_by_confidence;
  uint32_t warm_transferred;
  double selected_utility;
  double previous_utility;
  prom_dominatus_filter_output filter_output;
} prom_dominatus_filter_decision;

prom_dominatus_filter_policy_params prom_dominatus_filter_policy_default_params(void);
void prom_dominatus_filter_policy_init(prom_dominatus_filter_policy_state* state,
                                       const prom_dominatus_filter_policy_params* params);
void prom_dominatus_filter_policy_reset(prom_dominatus_filter_policy_state* state);
prom_dominatus_filter_decision prom_dominatus_filter_policy_update(prom_dominatus_filter_policy_state* state,
                                                                    double measurement,
                                                                    const prom_dominatus_measurement_facts* facts,
                                                                    uint64_t tick);

#ifdef __cplusplus
}
#endif

#endif
