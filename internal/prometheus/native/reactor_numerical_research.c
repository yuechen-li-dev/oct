#include "reactor_numerical_research.h"

#include <float.h>
#include <math.h>
#include <stdlib.h>
#include <string.h>

static uint64_t prom_num_hash_u64(uint64_t hash, uint64_t value) {
  uint32_t byte_index;
  for (byte_index = 0u; byte_index < 8u; ++byte_index) {
    hash ^= (value >> (byte_index * 8u)) & 0xffu;
    hash *= 1099511628211ull;
  }
  return hash;
}

static uint64_t prom_num_next_u64(uint64_t* state) {
  uint64_t value = *state;
  value ^= value >> 12u;
  value ^= value << 25u;
  value ^= value >> 27u;
  *state = value;
  return value * 2685821657736338717ull;
}

static double prom_num_abs(double value) {
  return value < 0.0 ? -value : value;
}

static int prom_num_compare_double(const void* left, const void* right) {
  const double a = *(const double*)left;
  const double b = *(const double*)right;
  if (a < b) return -1;
  if (a > b) return 1;
  return 0;
}

static double prom_num_percentile(const double* sorted, uint64_t count, uint32_t numerator) {
  uint64_t index;
  if (count == 0u) return 0.0;
  index = ((count - 1u) * numerator + 50u) / 100u;
  return sorted[index];
}

uint64_t prom_num_hash_float_bits(const float* values, uint64_t count) {
  uint64_t hash = 1469598103934665603ull;
  uint64_t index;
  if (values == NULL && count != 0u) return 0u;
  for (index = 0u; index < count; ++index) {
    uint32_t bits = 0u;
    memcpy(&bits, &values[index], sizeof(bits));
    hash = prom_num_hash_u64(hash, bits);
  }
  return hash;
}

uint64_t prom_num_experiment_plan_identity(uint32_t schema_version,
                                           const uint32_t* paths,
                                           uint32_t path_count,
                                           const uint32_t* stages,
                                           uint32_t stage_count,
                                           uint64_t corpus_identity) {
  uint64_t hash = 1469598103934665603ull;
  uint32_t index;
  if (schema_version == 0u || paths == NULL || path_count == 0u ||
      stages == NULL || stage_count == 0u) return 0u;
  hash = prom_num_hash_u64(hash, schema_version);
  hash = prom_num_hash_u64(hash, path_count);
  for (index = 0u; index < path_count; ++index) hash = prom_num_hash_u64(hash, paths[index]);
  hash = prom_num_hash_u64(hash, stage_count);
  for (index = 0u; index < stage_count; ++index) hash = prom_num_hash_u64(hash, stages[index]);
  hash = prom_num_hash_u64(hash, corpus_identity);
  return hash;
}

uint32_t prom_num_corpus_split_for_case(uint64_t case_identity) {
  const uint64_t mixed = prom_num_hash_u64(1469598103934665603ull, case_identity);
  return mixed % 5u == 0u ? PROM_NUM_SPLIT_HELD_OUT : PROM_NUM_SPLIT_IDENTIFICATION;
}

int prom_num_generate_input(uint32_t family, uint64_t seed, float* values,
                            uint32_t tokens, uint32_t channels) {
  uint64_t count;
  uint64_t index;
  uint64_t state = seed == 0u ? 0x9e3779b97f4a7c15ull : seed;
  if (values == NULL || tokens == 0u || channels == 0u ||
      family < PROM_NUM_INPUT_LOW_AMPLITUDE || family > PROM_NUM_INPUT_M48_LEGACY)
    return 0;
  count = (uint64_t)tokens * channels;
  if (count / channels != tokens) return 0;
  for (index = 0u; index < count; ++index) {
    const uint64_t random = prom_num_next_u64(&state);
    const int32_t centered = (int32_t)((random >> 32u) % 2001u) - 1000;
    const uint32_t channel = (uint32_t)(index % channels);
    float value = 0.0f;
    if (family == PROM_NUM_INPUT_LOW_AMPLITUDE) {
      value = (float)centered / 32768.0f;
    } else if (family == PROM_NUM_INPUT_WIDE_AMPLITUDE) {
      value = (float)centered / 32.0f;
    } else if (family == PROM_NUM_INPUT_ALTERNATING_SIGN) {
      value = (index & 1u) == 0u ? 0.75f : -0.75f;
      value += (float)(centered % 7) / 8192.0f;
    } else if (family == PROM_NUM_INPUT_SPARSE) {
      value = random % 17u == 0u ? (float)centered / 256.0f : 0.0f;
    } else if (family == PROM_NUM_INPUT_CANCELLATION_HEAVY) {
      value = (index & 1u) == 0u ? 1.0f : -1.0f;
      value += (float)(centered % 5) / 65536.0f;
    } else if (family == PROM_NUM_INPUT_MONOTONIC_RAMP) {
      value = count == 1u ? 0.0f : (float)((double)index / (double)(count - 1u) * 2.0 - 1.0);
    } else if (family == PROM_NUM_INPUT_NEAR_FP16_MIDPOINT) {
      const int32_t base = (int32_t)(index % 31u) - 15;
      const float direction = (random & 1u) == 0u ? -0x1.0p-24f : 0x1.0p-24f;
      value = 1.0f + (float)base * 0x1.0p-10f + 0x1.0p-11f + direction;
    } else if (family == PROM_NUM_INPUT_OUTLIER_CHANNEL) {
      value = (float)centered / 4096.0f;
      if (channel == channels / 3u) value *= 64.0f;
    } else {
      value = (float)((int64_t)index - 7) / 64.0f;
    }
    values[index] = value;
  }
  return 1;
}

int prom_num_summarize_error(const float* reference, const float* actual,
                             uint32_t tokens, uint32_t channels,
                             double near_zero_floor, double absolute_bound,
                             double relative_bound, double* scratch,
                             uint64_t scratch_count,
                             prom_num_error_summary* out_summary) {
  uint64_t count;
  uint64_t index;
  double reference_l2 = 0.0;
  double actual_l2 = 0.0;
  double dot = 0.0;
  double residual_l2 = 0.0;
  if (out_summary == NULL) return 0;
  memset(out_summary, 0, sizeof(*out_summary));
  if (reference == NULL || actual == NULL || scratch == NULL || tokens == 0u ||
      channels == 0u || near_zero_floor <= 0.0 || absolute_bound < 0.0 ||
      relative_bound < 0.0) return 0;
  count = (uint64_t)tokens * channels;
  if (count / channels != tokens || scratch_count < count) return 0;
  out_summary->element_count = count;
  for (index = 0u; index < count; ++index) {
    const double expected = reference[index];
    const double observed = actual[index];
    const double residual = observed - expected;
    const double absolute = prom_num_abs(residual);
    const double denominator = prom_num_abs(expected) < near_zero_floor
                                   ? near_zero_floor
                                   : prom_num_abs(expected);
    const double relative = absolute / denominator;
    if (!isfinite(expected) || !isfinite(observed)) return 0;
    scratch[index] = absolute;
    out_summary->l1_norm += absolute;
    residual_l2 += residual * residual;
    out_summary->signed_mean_bias += residual;
    reference_l2 += expected * expected;
    actual_l2 += observed * observed;
    dot += expected * observed;
    if (absolute > out_summary->maximum_absolute_error) out_summary->maximum_absolute_error = absolute;
    if (relative > out_summary->maximum_relative_error) out_summary->maximum_relative_error = relative;
    if (prom_num_abs(expected) < near_zero_floor) out_summary->near_zero_reference_count += 1u;
    if (absolute > absolute_bound && relative > relative_bound) out_summary->exceeding_count += 1u;
    if (residual > 0.0) out_summary->positive_residual_count += 1u;
    else if (residual < 0.0) out_summary->negative_residual_count += 1u;
  }
  out_summary->mean_absolute_error = out_summary->l1_norm / (double)count;
  out_summary->rms_error = sqrt(residual_l2 / (double)count);
  out_summary->l2_norm = sqrt(residual_l2);
  out_summary->linfinity_norm = out_summary->maximum_absolute_error;
  out_summary->signed_mean_bias /= (double)count;
  if (reference_l2 == 0.0 && actual_l2 == 0.0) out_summary->cosine_similarity = 1.0;
  else if (reference_l2 == 0.0 || actual_l2 == 0.0) out_summary->cosine_similarity = 0.0;
  else out_summary->cosine_similarity = dot / sqrt(reference_l2 * actual_l2);
  qsort(scratch, (size_t)count, sizeof(double), prom_num_compare_double);
  out_summary->p50_absolute_error = prom_num_percentile(scratch, count, 50u);
  out_summary->p90_absolute_error = prom_num_percentile(scratch, count, 90u);
  out_summary->p95_absolute_error = prom_num_percentile(scratch, count, 95u);
  out_summary->p99_absolute_error = prom_num_percentile(scratch, count, 99u);
  if (residual_l2 > 0.0) {
    uint32_t token;
    uint32_t channel;
    for (token = 0u; token < tokens; ++token) {
      double energy = 0.0;
      for (channel = 0u; channel < channels; ++channel) {
        const uint64_t location = (uint64_t)token * channels + channel;
        const double residual = (double)actual[location] - reference[location];
        energy += residual * residual;
      }
      if (energy / residual_l2 > out_summary->worst_token_l2_fraction) {
        out_summary->worst_token_l2_fraction = energy / residual_l2;
        out_summary->worst_token = token;
      }
    }
    for (channel = 0u; channel < channels; ++channel) {
      double energy = 0.0;
      for (token = 0u; token < tokens; ++token) {
        const uint64_t location = (uint64_t)token * channels + channel;
        const double residual = (double)actual[location] - reference[location];
        energy += residual * residual;
      }
      if (energy / residual_l2 > out_summary->worst_channel_l2_fraction) {
        out_summary->worst_channel_l2_fraction = energy / residual_l2;
        out_summary->worst_channel = channel;
      }
    }
  }
  out_summary->valid = 1u;
  return 1;
}

int prom_num_summarize_correlation(const float* reference, const float* actual,
                                   uint32_t tokens, uint32_t channels,
                                   prom_num_correlation_summary* out_summary) {
  uint64_t count;
  uint64_t index;
  double reference_mean = 0.0;
  double residual_mean = 0.0;
  double numerator = 0.0;
  double reference_energy = 0.0;
  double residual_energy = 0.0;
  double lag_numerator = 0.0;
  double lag_left_energy = 0.0;
  double lag_right_energy = 0.0;
  uint64_t same_sign = 0u;
  uint64_t recurrence = 0u;
  if (out_summary == NULL) return 0;
  memset(out_summary, 0, sizeof(*out_summary));
  if (reference == NULL || actual == NULL || tokens == 0u || channels == 0u) return 0;
  count = (uint64_t)tokens * channels;
  if (count / channels != tokens) return 0;
  for (index = 0u; index < count; ++index) {
    if (!isfinite(reference[index]) || !isfinite(actual[index])) return 0;
    reference_mean += reference[index];
    residual_mean += (double)actual[index] - reference[index];
  }
  reference_mean /= (double)count;
  residual_mean /= (double)count;
  for (index = 0u; index < count; ++index) {
    const double centered_reference = reference[index] - reference_mean;
    const double centered_residual = ((double)actual[index] - reference[index]) - residual_mean;
    numerator += centered_reference * centered_residual;
    reference_energy += centered_reference * centered_reference;
    residual_energy += centered_residual * centered_residual;
    if (index > 0u) {
      const double previous = ((double)actual[index - 1u] - reference[index - 1u]) - residual_mean;
      lag_numerator += previous * centered_residual;
      lag_left_energy += previous * previous;
      lag_right_energy += centered_residual * centered_residual;
      if ((previous > 0.0 && centered_residual > 0.0) ||
          (previous < 0.0 && centered_residual < 0.0)) same_sign += 1u;
    }
    if (index >= channels) {
      const double prior_channel = (double)actual[index - channels] - reference[index - channels];
      const double current = (double)actual[index] - reference[index];
      if ((prior_channel > 0.0 && current > 0.0) ||
          (prior_channel < 0.0 && current < 0.0)) recurrence += 1u;
    }
  }
  if (reference_energy > 0.0 && residual_energy > 0.0)
    out_summary->residual_reference_correlation = numerator / sqrt(reference_energy * residual_energy);
  if (lag_left_energy > 0.0 && lag_right_energy > 0.0)
    out_summary->residual_lag1_autocorrelation =
        lag_numerator / sqrt(lag_left_energy * lag_right_energy);
  if (count > 1u) out_summary->signed_persistence_fraction = (double)same_sign / (double)(count - 1u);
  if (tokens > 1u)
    out_summary->channel_recurrence_fraction =
        (double)recurrence / ((double)(tokens - 1u) * channels);
  out_summary->valid = 1u;
  return 1;
}

int prom_num_summarize_gain(const float* input_a, const float* input_b,
                            const float* output_a, const float* output_b,
                            uint32_t tokens, uint32_t channels,
                            prom_num_gain_summary* out_summary) {
  uint64_t count;
  uint64_t index;
  double input_energy = 0.0;
  double output_energy = 0.0;
  uint32_t token;
  uint32_t channel;
  if (out_summary == NULL) return 0;
  memset(out_summary, 0, sizeof(*out_summary));
  if (input_a == NULL || input_b == NULL || output_a == NULL || output_b == NULL ||
      tokens == 0u || channels == 0u) return 0;
  count = (uint64_t)tokens * channels;
  if (count / channels != tokens) return 0;
  for (index = 0u; index < count; ++index) {
    const double input_delta = (double)input_b[index] - input_a[index];
    const double output_delta = (double)output_b[index] - output_a[index];
    if (!isfinite(input_delta) || !isfinite(output_delta)) return 0;
    input_energy += input_delta * input_delta;
    output_energy += output_delta * output_delta;
  }
  if (input_energy == 0.0) return 0;
  out_summary->input_l2 = sqrt(input_energy);
  out_summary->output_l2 = sqrt(output_energy);
  out_summary->global_gain = out_summary->output_l2 / out_summary->input_l2;
  out_summary->minimum_token_gain = DBL_MAX;
  for (token = 0u; token < tokens; ++token) {
    double local_input = 0.0;
    double local_output = 0.0;
    for (channel = 0u; channel < channels; ++channel) {
      const uint64_t location = (uint64_t)token * channels + channel;
      const double input_delta = (double)input_b[location] - input_a[location];
      const double output_delta = (double)output_b[location] - output_a[location];
      local_input += input_delta * input_delta;
      local_output += output_delta * output_delta;
    }
    if (local_input > 0.0) {
      const double gain = sqrt(local_output / local_input);
      if (gain > out_summary->maximum_token_gain) {
        out_summary->maximum_token_gain = gain;
        out_summary->maximum_token = token;
      }
      if (gain < out_summary->minimum_token_gain) out_summary->minimum_token_gain = gain;
    }
  }
  for (channel = 0u; channel < channels; ++channel) {
    double local_input = 0.0;
    double local_output = 0.0;
    for (token = 0u; token < tokens; ++token) {
      const uint64_t location = (uint64_t)token * channels + channel;
      const double input_delta = (double)input_b[location] - input_a[location];
      const double output_delta = (double)output_b[location] - output_a[location];
      local_input += input_delta * input_delta;
      local_output += output_delta * output_delta;
    }
    if (local_input > 0.0) {
      const double gain = sqrt(local_output / local_input);
      if (gain > out_summary->maximum_channel_gain) {
        out_summary->maximum_channel_gain = gain;
        out_summary->maximum_channel = channel;
      }
    }
  }
  if (out_summary->minimum_token_gain == DBL_MAX) out_summary->minimum_token_gain = 0.0;
  out_summary->valid = 1u;
  return 1;
}

void prom_num_determinism_init(prom_num_determinism_tracker* tracker) {
  if (tracker != NULL) memset(tracker, 0, sizeof(*tracker));
}

int prom_num_determinism_update(prom_num_determinism_tracker* tracker,
                                const float* baseline, const float* sample,
                                uint64_t count) {
  uint64_t index;
  uint64_t hash;
  if (tracker == NULL || baseline == NULL || sample == NULL || count == 0u) return 0;
  hash = prom_num_hash_float_bits(sample, count);
  if (tracker->sample_count == 0u) tracker->first_hash = hash;
  else if (hash != tracker->first_hash) tracker->distinct_from_first_count += 1u;
  for (index = 0u; index < count; ++index) {
    const double difference = prom_num_abs((double)sample[index] - baseline[index]);
    if (!isfinite(sample[index])) tracker->saw_invalid = 1u;
    if (difference > tracker->maximum_absolute_deviation)
      tracker->maximum_absolute_deviation = difference;
  }
  tracker->sample_count += 1u;
  return 1;
}

uint32_t prom_num_determinism_classify(const prom_num_determinism_tracker* tracker,
                                       double fixed_envelope) {
  if (tracker == NULL || tracker->sample_count == 0u || fixed_envelope < 0.0)
    return PROM_NUM_DETERMINISM_UNIDENTIFIED;
  if (tracker->saw_invalid != 0u) return PROM_NUM_DETERMINISM_INVALID;
  if (tracker->distinct_from_first_count == 0u && tracker->maximum_absolute_deviation == 0.0)
    return PROM_NUM_DETERMINISM_BITWISE;
  if (tracker->maximum_absolute_deviation <= fixed_envelope)
    return PROM_NUM_DETERMINISM_ENVELOPE;
  return PROM_NUM_DETERMINISM_NONDETERMINISTIC;
}

int prom_num_canary_measure(const float* values, uint32_t tokens,
                            uint32_t channels, uint64_t seed,
                            prom_num_canary_summary* out_summary) {
  uint64_t count;
  uint64_t index;
  uint64_t state = seed == 0u ? 0xd1b54a32d192ed03ull : seed;
  if (out_summary == NULL) return 0;
  memset(out_summary, 0, sizeof(*out_summary));
  if (values == NULL || tokens == 0u || channels == 0u) return 0;
  count = (uint64_t)tokens * channels;
  if (count / channels != tokens) return 0;
  for (index = 0u; index < count; ++index) {
    const double value = values[index];
    const uint64_t random = prom_num_next_u64(&state);
    const double sign_a = (random & 1u) == 0u ? -1.0 : 1.0;
    const double sign_b = (random & 2u) == 0u ? -1.0 : 1.0;
    const double absolute = prom_num_abs(value);
    if (!isfinite(value)) return 0;
    out_summary->l1_norm += absolute;
    out_summary->l2_norm += value * value;
    if (absolute > out_summary->maximum_absolute_value) out_summary->maximum_absolute_value = absolute;
    out_summary->signed_projection_a += sign_a * value;
    out_summary->signed_projection_b += sign_b * value;
    out_summary->absolute_projection += (1.0 + (double)(random % 7u)) * absolute;
  }
  out_summary->l2_norm = sqrt(out_summary->l2_norm);
  out_summary->bit_hash = prom_num_hash_float_bits(values, count);
  out_summary->valid = 1u;
  return 1;
}

prom_num_observer_params prom_num_observer_default_params(void) {
  prom_num_observer_params params;
  params.nominal_local_error = 1.0e-3;
  params.high_injection_error = 1.0e-2;
  params.high_gain = 1.25;
  params.severe_gain = 2.0;
  params.concentration_limit = 0.50;
  params.enter_ticks = 2u;
  params.clear_ticks = 3u;
  return params;
}

void prom_num_observer_init(prom_num_observer_state* state) {
  if (state == NULL) return;
  memset(state, 0, sizeof(*state));
  state->regime = PROM_NUM_REGIME_UNIDENTIFIED;
  state->pending_regime = PROM_NUM_REGIME_UNIDENTIFIED;
}

static uint32_t prom_num_observer_desired(const prom_num_observer_params* params,
                                          const prom_num_observer_evidence* evidence) {
  if (evidence->hardware_fault != 0u ||
      evidence->deterministic_class == PROM_NUM_DETERMINISM_INVALID)
    return PROM_NUM_REGIME_QUARANTINED;
  if (evidence->valid == 0u) return PROM_NUM_REGIME_UNIDENTIFIED;
  if (evidence->reference_disagreement != 0u) return PROM_NUM_REGIME_REFERENCE_SUSPECT;
  if (evidence->implementation_defect != 0u) return PROM_NUM_REGIME_AUDIT_REQUIRED;
  if (evidence->envelope_exceeded != 0u && evidence->gain >= params->severe_gain) {
    if (evidence->fallback_available != 0u)
      return PROM_NUM_REGIME_BACKEND_FALLBACK_RECOMMENDED;
    return PROM_NUM_REGIME_AUDIT_REQUIRED;
  }
  if (evidence->local_disturbance_l2 >= params->high_injection_error) {
    if (evidence->precision_promotion_available != 0u)
      return PROM_NUM_REGIME_PRECISION_PROMOTION_RECOMMENDED;
    return PROM_NUM_REGIME_HIGH_INJECTION;
  }
  if (evidence->gain >= params->high_gain ||
      evidence->correlation_concentration >= params->concentration_limit)
    return PROM_NUM_REGIME_HIGH_GAIN;
  if (evidence->local_disturbance_l2 > params->nominal_local_error ||
      evidence->inherited_error_l2 > params->nominal_local_error)
    return PROM_NUM_REGIME_BOUNDED_DRIFT;
  return PROM_NUM_REGIME_NOMINAL;
}

uint32_t prom_num_observer_update(prom_num_observer_state* state,
                                  const prom_num_observer_params* params,
                                  const prom_num_observer_evidence* evidence) {
  uint32_t desired;
  uint32_t immediate;
  if (state == NULL || params == NULL || evidence == NULL || params->enter_ticks == 0u ||
      params->clear_ticks == 0u || params->nominal_local_error < 0.0 ||
      params->high_injection_error < params->nominal_local_error ||
      params->high_gain <= 0.0 || params->severe_gain < params->high_gain) return PROM_NUM_REGIME_UNIDENTIFIED;
  state->evaluation_count += 1u;
  desired = prom_num_observer_desired(params, evidence);
  immediate = desired == PROM_NUM_REGIME_QUARANTINED ||
              desired == PROM_NUM_REGIME_REFERENCE_SUSPECT ||
              desired == PROM_NUM_REGIME_AUDIT_REQUIRED;
  if (desired == state->regime) {
    state->pending_regime = desired;
    state->pending_ticks = 0u;
    state->clear_ticks = 0u;
    return state->regime;
  }
  if (immediate != 0u) {
    state->regime = desired;
    state->pending_regime = desired;
    state->pending_ticks = 0u;
    state->clear_ticks = 0u;
    state->transition_count += 1u;
    return state->regime;
  }
  if (desired == PROM_NUM_REGIME_NOMINAL && state->regime != PROM_NUM_REGIME_UNIDENTIFIED) {
    state->clear_ticks += 1u;
    if (state->clear_ticks >= params->clear_ticks) {
      state->regime = desired;
      state->pending_regime = desired;
      state->pending_ticks = 0u;
      state->clear_ticks = 0u;
      state->transition_count += 1u;
    }
    return state->regime;
  }
  state->clear_ticks = 0u;
  if (state->pending_regime != desired) {
    state->pending_regime = desired;
    state->pending_ticks = 1u;
  } else {
    state->pending_ticks += 1u;
  }
  if (state->pending_ticks >= params->enter_ticks || state->regime == PROM_NUM_REGIME_UNIDENTIFIED) {
    state->regime = desired;
    state->pending_ticks = 0u;
    state->transition_count += 1u;
  }
  return state->regime;
}

static uint32_t prom_num_candidate_allowed(uint32_t regime, const prom_num_candidate* candidate,
                                           double configured_error_envelope) {
  if (candidate->eligible == 0u) return 0u;
  if (regime == PROM_NUM_REGIME_QUARANTINED)
    return candidate->action == PROM_NUM_ACTION_REJECT_OR_QUARANTINE;
  if (regime == PROM_NUM_REGIME_REFERENCE_SUSPECT || regime == PROM_NUM_REGIME_AUDIT_REQUIRED)
    return candidate->action == PROM_NUM_ACTION_STAGE_AUDIT ||
           candidate->action == PROM_NUM_ACTION_REJECT_OR_QUARANTINE;
  if (candidate->action != PROM_NUM_ACTION_STAGE_AUDIT &&
      candidate->action != PROM_NUM_ACTION_REJECT_OR_QUARANTINE &&
      candidate->predicted_error > configured_error_envelope) return 0u;
  return 1u;
}

int prom_num_shadow_select(uint32_t authoritative_action, uint32_t regime,
                           double configured_error_envelope,
                           prom_num_candidate* candidates,
                           uint32_t candidate_count,
                           prom_num_shadow_decision* out_decision) {
  uint32_t index;
  uint32_t selected = UINT32_MAX;
  double selected_score = -DBL_MAX;
  uint64_t identity = 1469598103934665603ull;
  if (out_decision == NULL) return 0;
  memset(out_decision, 0, sizeof(*out_decision));
  if (candidates == NULL || candidate_count == 0u || candidate_count > PROM_NUM_MAX_CANDIDATES ||
      configured_error_envelope < 0.0) return 0;
  out_decision->authoritative_action = authoritative_action;
  out_decision->candidate_count = candidate_count;
  identity = prom_num_hash_u64(identity, regime);
  identity = prom_num_hash_u64(identity, authoritative_action);
  for (index = 0u; index < candidate_count; ++index) {
    prom_num_candidate candidate = candidates[index];
    const uint32_t allowed = prom_num_candidate_allowed(regime, &candidate,
                                                         configured_error_envelope);
    candidate.contribution[PROM_NUM_CONSIDERATION_RISK_REDUCTION - 1u] =
        configured_error_envelope == 0.0
            ? (candidate.predicted_error == 0.0 ? 100.0 : -100.0)
            : 100.0 * (configured_error_envelope - candidate.predicted_error) /
                  configured_error_envelope;
    candidate.contribution[PROM_NUM_CONSIDERATION_LATENCY_COST - 1u] =
        -0.05 * candidate.latency_microseconds;
    candidate.contribution[PROM_NUM_CONSIDERATION_MEMORY_COST - 1u] =
        -0.5 * ((double)candidate.retained_bytes / (1024.0 * 1024.0));
    candidate.contribution[PROM_NUM_CONSIDERATION_PORTABILITY - 1u] =
        10.0 * candidate.portability;
    candidate.contribution[PROM_NUM_CONSIDERATION_COMPLEXITY - 1u] =
        -5.0 * candidate.complexity;
    candidate.contribution[PROM_NUM_CONSIDERATION_EVIDENCE_CONFIDENCE - 1u] =
        15.0 * candidate.confidence;
    candidate.score = 0.0;
    {
      uint32_t consideration;
      for (consideration = 0u; consideration < PROM_NUM_MAX_CONSIDERATIONS; ++consideration)
        candidate.score += candidate.contribution[consideration];
    }
    if (allowed == 0u) {
      candidate.eligible = 0u;
      if (candidate.ineligible_reason == 0u) candidate.ineligible_reason = 1u;
    }
    out_decision->candidate[index] = candidate;
    identity = prom_num_hash_u64(identity, candidate.action);
    identity = prom_num_hash_u64(identity, candidate.eligible);
    if (candidate.eligible != 0u &&
        (selected == UINT32_MAX || candidate.score > selected_score ||
         (candidate.score == selected_score && candidate.action < candidates[selected].action))) {
      selected = index;
      selected_score = candidate.score;
    }
  }
  if (selected == UINT32_MAX) return 0;
  out_decision->proposed_action = out_decision->candidate[selected].action;
  out_decision->would_change_authority =
      out_decision->proposed_action == authoritative_action ? 0u : 1u;
  out_decision->decision_identity = prom_num_hash_u64(identity, out_decision->proposed_action);
  return 1;
}

int prom_num_envelope_evaluate(const prom_num_envelope* envelope,
                               uint32_t path, uint32_t stage,
                               uint32_t tokens, uint32_t width,
                               double input_error, double observed_error,
                               prom_num_envelope_result* out_result) {
  if (out_result == NULL) return 0;
  memset(out_result, 0, sizeof(*out_result));
  if (envelope == NULL || input_error < 0.0 || observed_error < 0.0 ||
      envelope->local_disturbance_bound < 0.0 || envelope->gain_bound < 0.0 ||
      envelope->bias_bound < 0.0 || envelope->held_out_confidence <= 0.0 ||
      envelope->held_out_confidence > 1.0) return 0;
  out_result->observed_error = observed_error;
  if (path != envelope->path || stage != envelope->stage ||
      tokens < envelope->minimum_tokens || tokens > envelope->maximum_tokens ||
      width < envelope->minimum_width || width > envelope->maximum_width) return 1;
  out_result->supported = 1u;
  out_result->bound = envelope->gain_bound * input_error +
                      envelope->local_disturbance_bound + envelope->bias_bound;
  out_result->within_envelope = observed_error <= out_result->bound ? 1u : 0u;
  return 1;
}

int prom_num_bias_fit(const float* reference, const float* observed,
                      uint64_t count, prom_num_bias_model* out_model) {
  uint64_t index;
  double bias = 0.0;
  if (out_model == NULL) return 0;
  memset(out_model, 0, sizeof(*out_model));
  if (reference == NULL || observed == NULL || count == 0u) return 0;
  for (index = 0u; index < count; ++index) {
    if (!isfinite(reference[index]) || !isfinite(observed[index])) return 0;
    bias += (double)observed[index] - reference[index];
  }
  out_model->bias = bias / (double)count;
  out_model->fitted_count = count;
  out_model->valid = 1u;
  return 1;
}

int prom_num_bias_evaluate(const prom_num_bias_model* model,
                           const float* reference, const float* observed,
                           uint64_t count, double* out_uncorrected_rms,
                           double* out_corrected_rms) {
  uint64_t index;
  double uncorrected = 0.0;
  double corrected = 0.0;
  if (out_uncorrected_rms == NULL || out_corrected_rms == NULL) return 0;
  *out_uncorrected_rms = 0.0;
  *out_corrected_rms = 0.0;
  if (model == NULL || model->valid == 0u || reference == NULL || observed == NULL || count == 0u)
    return 0;
  for (index = 0u; index < count; ++index) {
    const double residual = (double)observed[index] - reference[index];
    if (!isfinite(residual)) return 0;
    uncorrected += residual * residual;
    corrected += (residual - model->bias) * (residual - model->bias);
  }
  *out_uncorrected_rms = sqrt(uncorrected / (double)count);
  *out_corrected_rms = sqrt(corrected / (double)count);
  return 1;
}
