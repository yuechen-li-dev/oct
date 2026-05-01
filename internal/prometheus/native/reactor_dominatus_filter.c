#include "reactor_dominatus_filter.h"

#include <math.h>
#include <stddef.h>
#include <string.h>

static prom_dominatus_filter_output invalid_output(void) {
  prom_dominatus_filter_output out;
  memset(&out, 0, sizeof(out));
  return out;
}

static uint32_t is_supported_window(uint32_t window) { return window == 3u || window == 5u || window == 9u; }

static uint32_t validate_params(const prom_dominatus_filter_params* params) {
  if (params == NULL) {
    return 0u;
  }
  switch (params->kind) {
    case PROM_DOM_FILTER_KIND_EMA:
      return params->alpha > 0.0 && params->alpha <= 1.0;
    case PROM_DOM_FILTER_KIND_MEDIAN:
      return is_supported_window(params->window);
    case PROM_DOM_FILTER_KIND_HYSTERESIS:
      return params->band >= 0.0;
    case PROM_DOM_FILTER_KIND_HYBRID_MEDIAN_EMA:
      return is_supported_window(params->window) && params->alpha > 0.0 && params->alpha <= 1.0;
    default:
      return 0u;
  }
}

static double compute_median(const double* values, uint32_t count) {
  double copy[PROM_DOMINATUS_FILTER_MAX_WINDOW];
  uint32_t i;
  uint32_t j;
  for (i = 0u; i < count; ++i) {
    copy[i] = values[i];
  }
  for (i = 1u; i < count; ++i) {
    const double key = copy[i];
    j = i;
    while (j > 0u && copy[j - 1u] > key) {
      copy[j] = copy[j - 1u];
      --j;
    }
    copy[j] = key;
  }
  return copy[count / 2u];
}

prom_dominatus_filter_params prom_dominatus_filter_params_ema(double alpha) {
  prom_dominatus_filter_params params;
  memset(&params, 0, sizeof(params));
  params.kind = PROM_DOM_FILTER_KIND_EMA;
  params.alpha = alpha;
  return params;
}

prom_dominatus_filter_params prom_dominatus_filter_params_median(uint32_t window) {
  prom_dominatus_filter_params params;
  memset(&params, 0, sizeof(params));
  params.kind = PROM_DOM_FILTER_KIND_MEDIAN;
  params.window = window;
  return params;
}

prom_dominatus_filter_params prom_dominatus_filter_params_hysteresis(double band) {
  prom_dominatus_filter_params params;
  memset(&params, 0, sizeof(params));
  params.kind = PROM_DOM_FILTER_KIND_HYSTERESIS;
  params.band = band;
  return params;
}

prom_dominatus_filter_params prom_dominatus_filter_params_hybrid_median_ema(uint32_t window, double alpha) {
  prom_dominatus_filter_params params;
  memset(&params, 0, sizeof(params));
  params.kind = PROM_DOM_FILTER_KIND_HYBRID_MEDIAN_EMA;
  params.window = window;
  params.alpha = alpha;
  return params;
}

void prom_dominatus_filter_init(prom_dominatus_filter_state* state, const prom_dominatus_filter_params* params) {
  if (state == NULL) {
    return;
  }
  memset(state, 0, sizeof(*state));
  if (validate_params(params) == 0u) {
    state->kind = PROM_DOM_FILTER_KIND_NONE;
    return;
  }
  state->kind = params->kind;
  state->params = *params;
}

void prom_dominatus_filter_warm_start(prom_dominatus_filter_state* state,
                                      const prom_dominatus_filter_params* params,
                                      double prior_estimate) {
  prom_dominatus_filter_init(state, params);
  if (state == NULL || state->kind == PROM_DOM_FILTER_KIND_NONE) {
    return;
  }
  state->initialized = 1u;
  state->estimate = prior_estimate;
}

void prom_dominatus_filter_reset(prom_dominatus_filter_state* state) {
  prom_dominatus_filter_params params;
  if (state == NULL) {
    return;
  }
  params = state->params;
  prom_dominatus_filter_init(state, &params);
}

prom_dominatus_filter_output prom_dominatus_filter_update(prom_dominatus_filter_state* state,
                                                           double measurement,
                                                           uint64_t tick) {
  prom_dominatus_filter_output out = invalid_output();
  double source_estimate = measurement;
  uint32_t updated = 1u;

  if (state == NULL || state->kind == PROM_DOM_FILTER_KIND_NONE) {
    return out;
  }

  if (state->initialized == 0u) {
    state->initialized = 1u;
    state->estimate = measurement;
    if (state->kind == PROM_DOM_FILTER_KIND_MEDIAN || state->kind == PROM_DOM_FILTER_KIND_HYBRID_MEDIAN_EMA) {
      state->window_buffer[0] = measurement;
      state->window_count = 1u;
      state->window_index = 1u % state->params.window;
    }
  } else {
    if (state->kind == PROM_DOM_FILTER_KIND_MEDIAN || state->kind == PROM_DOM_FILTER_KIND_HYBRID_MEDIAN_EMA) {
      state->window_buffer[state->window_index] = measurement;
      state->window_index = (state->window_index + 1u) % state->params.window;
      if (state->window_count < state->params.window) {
        state->window_count += 1u;
      }
      source_estimate = compute_median(state->window_buffer, state->window_count);
    }

    switch (state->kind) {
      case PROM_DOM_FILTER_KIND_EMA:
        state->estimate = (state->params.alpha * measurement) + ((1.0 - state->params.alpha) * state->estimate);
        break;
      case PROM_DOM_FILTER_KIND_MEDIAN:
        state->estimate = source_estimate;
        break;
      case PROM_DOM_FILTER_KIND_HYSTERESIS:
        if (fabs(measurement - state->estimate) > state->params.band) {
          state->estimate = measurement;
          state->update_count += 1u;
          updated = 1u;
        } else {
          state->hold_count += 1u;
          updated = 0u;
        }
        break;
      case PROM_DOM_FILTER_KIND_HYBRID_MEDIAN_EMA:
        state->estimate = (state->params.alpha * source_estimate) + ((1.0 - state->params.alpha) * state->estimate);
        break;
      default:
        return out;
    }
  }

  state->sample_count += 1u;
  state->last_update_tick = tick;
  state->last_residual = measurement - state->estimate;
  state->stability_proxy = fabs(state->last_residual);

  out.valid = 1u;
  out.estimate = state->estimate;
  out.residual = state->last_residual;
  out.stability_proxy = state->stability_proxy;
  out.sample_count = state->sample_count;
  out.updated = updated;
  out.held = updated == 0u ? 1u : 0u;
  out.warmup = state->sample_count < 3u ? 1u : 0u;
  return out;
}
