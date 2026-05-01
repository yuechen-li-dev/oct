#include "reactor_dominatus_filter_policy.h"

#include <math.h>
#include <stddef.h>
#include <string.h>

static prom_dominatus_filter_params params_for_kind(prom_dominatus_filter_kind kind) {
  switch (kind) {
    case PROM_DOM_FILTER_KIND_EMA:
      return prom_dominatus_filter_params_ema(0.2);
    case PROM_DOM_FILTER_KIND_MEDIAN:
      return prom_dominatus_filter_params_median(5u);
    case PROM_DOM_FILTER_KIND_HYSTERESIS:
      return prom_dominatus_filter_params_hysteresis(0.5);
    case PROM_DOM_FILTER_KIND_HYBRID_MEDIAN_EMA:
      return prom_dominatus_filter_params_hybrid_median_ema(5u, 0.4);
    default:
      return prom_dominatus_filter_params_hysteresis(-1.0);
  }
}

static double clamp01(double x) {
  if (x < 0.0) return 0.0;
  if (x > 1.0) return 1.0;
  return x;
}

static double score_kind(prom_dominatus_filter_kind kind, const prom_dominatus_measurement_facts* facts) {
  const double spike = clamp01(facts->spike_rate_estimate);
  const double jitter = clamp01(facts->jitter_estimate);
  const double stable_bonus = (1.0 - spike) * (1.0 - jitter);
  const double confidence = clamp01(facts->confidence);
  const double weak_evidence_penalty = (1.0 - confidence) * 0.15;

  if (kind == PROM_DOM_FILTER_KIND_HYSTERESIS) {
    return stable_bonus + (facts->drift_suspected ? -0.25 : 0.0) + (facts->step_change_suspected ? -0.15 : 0.0) - weak_evidence_penalty;
  }
  if (kind == PROM_DOM_FILTER_KIND_MEDIAN) {
    return (spike * 0.85) + (jitter * 0.35) + (facts->outlier_count > 0u ? 0.1 : 0.0) - (facts->step_change_suspected ? 0.1 : 0.0) - weak_evidence_penalty;
  }
  if (kind == PROM_DOM_FILTER_KIND_HYBRID_MEDIAN_EMA) {
    return (spike * 0.75) + (jitter * 0.30) + (facts->step_change_suspected ? 0.35 : 0.0) + (facts->drift_suspected ? 0.2 : 0.0) + (confidence * 0.2);
  }
  if (kind == PROM_DOM_FILTER_KIND_EMA) {
    return (facts->step_change_suspected ? 0.5 : 0.0) + (facts->drift_suspected ? 0.45 : 0.0) + ((1.0 - spike) * 0.25) + (confidence * 0.1);
  }
  return -1.0;
}

prom_dominatus_filter_policy_params prom_dominatus_filter_policy_default_params(void) {
  prom_dominatus_filter_policy_params params;
  memset(&params, 0, sizeof(params));
  params.min_commit_ticks = 8u;
  params.switch_margin = 0.15;
  params.confidence_threshold = 0.45;
  params.stable_filter = PROM_DOM_FILTER_KIND_HYSTERESIS;
  params.spike_filter = PROM_DOM_FILTER_KIND_MEDIAN;
  params.step_filter = PROM_DOM_FILTER_KIND_EMA;
  params.drift_filter = PROM_DOM_FILTER_KIND_EMA;
  params.mixed_filter = PROM_DOM_FILTER_KIND_HYBRID_MEDIAN_EMA;
  return params;
}

void prom_dominatus_filter_policy_init(prom_dominatus_filter_policy_state* state,
                                       const prom_dominatus_filter_policy_params* params) {
  if (state == NULL) return;
  memset(state, 0, sizeof(*state));
  state->params = (params == NULL) ? prom_dominatus_filter_policy_default_params() : *params;
}

void prom_dominatus_filter_policy_reset(prom_dominatus_filter_policy_state* state) {
  if (state == NULL) return;
  state->current_kind = PROM_DOM_FILTER_KIND_NONE;
  prom_dominatus_filter_reset(&state->current_filter);
  state->initialized = 0u;
  state->min_commit_remaining = 0u;
  state->last_switch_tick = 0u;
  state->switch_count = 0u;
  state->last_output = 0.0;
  state->last_confidence = 0.0;
}

prom_dominatus_filter_decision prom_dominatus_filter_policy_update(prom_dominatus_filter_policy_state* state,
                                                                    double measurement,
                                                                    const prom_dominatus_measurement_facts* facts,
                                                                    uint64_t tick) {
  prom_dominatus_filter_decision d;
  prom_dominatus_measurement_facts local_facts;
  prom_dominatus_filter_kind preferred;
  memset(&d, 0, sizeof(d));
  if (state == NULL) return d;
  if (facts == NULL) {
    memset(&local_facts, 0, sizeof(local_facts));
    local_facts.confidence = 1.0;
    facts = &local_facts;
  }

  if (facts->step_change_suspected != 0u) preferred = state->params.step_filter;
  else if (facts->drift_suspected != 0u) preferred = state->params.drift_filter;
  else if (facts->spike_rate_estimate >= 0.6) preferred = state->params.spike_filter;
  else if (facts->spike_rate_estimate >= 0.35 || facts->jitter_estimate >= 0.6) preferred = state->params.mixed_filter;
  else preferred = state->params.stable_filter;

  d.previous_kind = state->current_kind;
  d.selected_kind = state->current_kind == PROM_DOM_FILTER_KIND_NONE ? preferred : state->current_kind;
  d.selected_utility = score_kind(preferred, facts);
  d.previous_utility = score_kind(state->current_kind, facts);

  if (state->initialized == 0u) {
    prom_dominatus_filter_params p = params_for_kind(preferred);
    prom_dominatus_filter_init(&state->current_filter, &p);
    state->current_kind = preferred;
    state->initialized = 1u;
    d.selected_kind = preferred;
  } else if (preferred != state->current_kind) {
    if (facts->confidence < state->params.confidence_threshold) {
      d.held_by_confidence = 1u;
    } else if (state->min_commit_remaining > 0u) {
      d.held_by_min_commit = 1u;
    } else if (d.selected_utility < d.previous_utility + state->params.switch_margin) {
      d.held_by_margin = 1u;
    } else {
      prom_dominatus_filter_params p = params_for_kind(preferred);
      prom_dominatus_filter_warm_start(&state->current_filter, &p, state->last_output);
      state->current_kind = preferred;
      state->switch_count += 1u;
      state->last_switch_tick = tick;
      state->min_commit_remaining = state->params.min_commit_ticks;
      d.selected_kind = preferred;
      d.switched = 1u;
      d.warm_transferred = 1u;
    }
  }

  d.filter_output = prom_dominatus_filter_update(&state->current_filter, measurement, tick);
  state->last_output = d.filter_output.estimate;
  state->last_confidence = facts->confidence;
  if (state->min_commit_remaining > 0u) state->min_commit_remaining -= 1u;
  d.selected_kind = state->current_kind;
  return d;
}
