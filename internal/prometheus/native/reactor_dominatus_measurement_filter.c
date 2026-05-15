#include "reactor_dominatus_measurement_filter.h"

#include <math.h>
#include <stddef.h>
#include <string.h>

static double clamp01(double x) {
  if (x < 0.0) return 0.0;
  if (x > 1.0) return 1.0;
  return x;
}

void prom_dominatus_measurement_filter_init(prom_dominatus_measurement_filter_state* state,
                                            const prom_dominatus_filter_policy_params* params) {
  if (state == NULL) return;
  memset(state, 0, sizeof(*state));
  prom_dominatus_filter_policy_init(&state->policy, params);
}

void prom_dominatus_measurement_filter_reset(prom_dominatus_measurement_filter_state* state) {
  if (state == NULL) return;
  memset(state->recent_raw_buffer, 0, sizeof(state->recent_raw_buffer));
  memset(state->recent_filtered_buffer, 0, sizeof(state->recent_filtered_buffer));
  memset(&state->facts, 0, sizeof(state->facts));
  prom_dominatus_filter_policy_reset(&state->policy);
  state->recent_count = 0u;
  state->recent_index = 0u;
  state->initialized = 0u;
  state->last_tick = 0u;
}

prom_dominatus_measurement_facts prom_dominatus_measurement_filter_compute_facts(
    const prom_dominatus_measurement_filter_state* state,
    double raw_value) {
  prom_dominatus_measurement_facts facts;
  double abs_sum = 0.0;
  double sq_sum = 0.0;
  double mean = 0.0;
  double prev = 0.0;
  uint32_t spikes = 0u;
  uint32_t outliers = 0u;
  uint32_t i;
  memset(&facts, 0, sizeof(facts));

  if (state == NULL || state->recent_count == 0u) {
    facts.sample_count = 1u;
    facts.confidence = 0.35;
    return facts;
  }

  prev = state->recent_raw_buffer[(state->recent_index + PROM_DOM_MEASUREMENT_WINDOW_MAX - 1u) % PROM_DOM_MEASUREMENT_WINDOW_MAX];
  facts.recent_abs_residual = fabs(raw_value - prev);

  for (i = 0u; i < state->recent_count; ++i) {
    const uint32_t idx = (state->recent_index + PROM_DOM_MEASUREMENT_WINDOW_MAX - state->recent_count + i) % PROM_DOM_MEASUREMENT_WINDOW_MAX;
    const double v = state->recent_raw_buffer[idx];
    mean += v;
  }
  mean /= (double)state->recent_count;

  for (i = 0u; i < state->recent_count; ++i) {
    const uint32_t idx = (state->recent_index + PROM_DOM_MEASUREMENT_WINDOW_MAX - state->recent_count + i) % PROM_DOM_MEASUREMENT_WINDOW_MAX;
    const double v = state->recent_raw_buffer[idx];
    const double diff = v - mean;
    sq_sum += diff * diff;
  }
  facts.jitter_estimate = clamp01(sqrt(sq_sum / (double)state->recent_count) / (fabs(mean) + 1e-6));
  if (fabs(raw_value - mean) > fabs(mean) * 0.35 + 1e-6) {
    spikes += 1u;
    outliers += 1u;
  }

  for (i = 1u; i < state->recent_count; ++i) {
    const uint32_t idx_now = (state->recent_index + PROM_DOM_MEASUREMENT_WINDOW_MAX - state->recent_count + i) % PROM_DOM_MEASUREMENT_WINDOW_MAX;
    const uint32_t idx_prev = (state->recent_index + PROM_DOM_MEASUREMENT_WINDOW_MAX - state->recent_count + i - 1u) % PROM_DOM_MEASUREMENT_WINDOW_MAX;
    const double delta = fabs(state->recent_raw_buffer[idx_now] - state->recent_raw_buffer[idx_prev]);
    abs_sum += delta;
    if (delta > (fabs(mean) * 0.25 + 1e-6)) spikes += 1u;
    if (delta > (fabs(mean) * 0.45 + 1e-6)) outliers += 1u;
  }

  facts.sample_count = state->recent_count + 1u;
  facts.recent_output_variation = abs_sum / (double)(state->recent_count > 1u ? (state->recent_count - 1u) : 1u);
  facts.spike_rate_estimate = clamp01((double)spikes / (double)(state->recent_count > 1u ? state->recent_count : 1u));
  facts.outlier_count = outliers;
  facts.step_change_suspected = facts.sample_count >= 4u && facts.recent_abs_residual > fabs(mean) * 0.30;
  facts.drift_suspected = facts.sample_count >= 6u && facts.step_change_suspected == 0u && facts.jitter_estimate < 0.2 && facts.recent_abs_residual > fabs(mean) * 0.08;
  facts.confidence = clamp01(0.25 + ((double)state->recent_count / (double)PROM_DOM_MEASUREMENT_WINDOW_MAX) * 0.6 - facts.jitter_estimate * 0.25 - clamp01((double)outliers * 0.1));

  return facts;
}

prom_dominatus_filtered_evidence prom_dominatus_measurement_filter_update(prom_dominatus_measurement_filter_state* state,
                                                                           double raw_value,
                                                                           uint64_t tick) {
  prom_dominatus_filtered_evidence out;
  prom_dominatus_filter_decision decision;
  memset(&out, 0, sizeof(out));
  if (state == NULL) return out;

  state->facts = prom_dominatus_measurement_filter_compute_facts(state, raw_value);
  decision = prom_dominatus_filter_policy_update(&state->policy, raw_value, &state->facts, tick);

  out.valid = decision.filter_output.valid;
  out.raw_value = raw_value;
  out.filtered_value = decision.filter_output.estimate;
  out.residual = raw_value - decision.filter_output.estimate;
  out.selected_filter = decision.selected_kind;
  out.previous_filter = decision.previous_kind;
  out.filter_switched = decision.switched;
  out.filter_warmup = decision.filter_output.warmup;
  out.held_by_min_commit = decision.held_by_min_commit;
  out.held_by_margin = decision.held_by_margin;
  out.held_by_confidence = decision.held_by_confidence;
  out.warm_transferred = decision.warm_transferred;
  out.confidence = state->facts.confidence;
  out.selected_utility = decision.selected_utility;
  out.previous_utility = decision.previous_utility;
  out.sample_count = state->facts.sample_count;
  out.outlier_count = state->facts.outlier_count;
  out.step_change_suspected = state->facts.step_change_suspected;
  out.drift_suspected = state->facts.drift_suspected;
  out.spike_rate_estimate = state->facts.spike_rate_estimate;
  out.jitter_estimate = state->facts.jitter_estimate;

  state->recent_raw_buffer[state->recent_index] = raw_value;
  state->recent_filtered_buffer[state->recent_index] = out.filtered_value;
  state->recent_index = (state->recent_index + 1u) % PROM_DOM_MEASUREMENT_WINDOW_MAX;
  if (state->recent_count < PROM_DOM_MEASUREMENT_WINDOW_MAX) state->recent_count += 1u;
  state->initialized = 1u;
  state->last_tick = tick;

  return out;
}
