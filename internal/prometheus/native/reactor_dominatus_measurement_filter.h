#ifndef OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_DOMINATUS_MEASUREMENT_FILTER_H
#define OCT_INTERNAL_PROMETHEUS_NATIVE_REACTOR_DOMINATUS_MEASUREMENT_FILTER_H

#include "reactor_dominatus_filter_policy.h"

#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define PROM_DOM_MEASUREMENT_WINDOW_MAX 16u

typedef struct prom_dominatus_measurement_sample {
  double raw_value;
  uint64_t tick;
  uint32_t valid;
  uint32_t source_kind;
} prom_dominatus_measurement_sample;

typedef struct prom_dominatus_filtered_evidence {
  uint32_t valid;
  double raw_value;
  double filtered_value;
  double residual;
  prom_dominatus_filter_kind selected_filter;
  prom_dominatus_filter_kind previous_filter;
  uint32_t filter_switched;
  uint32_t filter_warmup;
  uint32_t held_by_min_commit;
  uint32_t held_by_margin;
  uint32_t held_by_confidence;
  uint32_t warm_transferred;
  double confidence;
  double selected_utility;
  double previous_utility;
  uint32_t sample_count;
  uint32_t outlier_count;
  uint32_t step_change_suspected;
  uint32_t drift_suspected;
  double spike_rate_estimate;
  double jitter_estimate;
} prom_dominatus_filtered_evidence;

typedef struct prom_dominatus_measurement_filter_state {
  prom_dominatus_filter_policy_state policy;
  prom_dominatus_measurement_facts facts;
  double recent_raw_buffer[PROM_DOM_MEASUREMENT_WINDOW_MAX];
  double recent_filtered_buffer[PROM_DOM_MEASUREMENT_WINDOW_MAX];
  uint32_t recent_count;
  uint32_t recent_index;
  uint32_t initialized;
  uint64_t last_tick;
} prom_dominatus_measurement_filter_state;

void prom_dominatus_measurement_filter_init(prom_dominatus_measurement_filter_state* state,
                                            const prom_dominatus_filter_policy_params* params);
void prom_dominatus_measurement_filter_reset(prom_dominatus_measurement_filter_state* state);
prom_dominatus_measurement_facts prom_dominatus_measurement_filter_compute_facts(
    const prom_dominatus_measurement_filter_state* state,
    double raw_value);
prom_dominatus_filtered_evidence prom_dominatus_measurement_filter_update(prom_dominatus_measurement_filter_state* state,
                                                                           double raw_value,
                                                                           uint64_t tick);

#ifdef __cplusplus
}
#endif

#endif
