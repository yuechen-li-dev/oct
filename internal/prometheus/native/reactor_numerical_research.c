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
    out_summary->input_l1 += prom_num_abs(input_delta);
    out_summary->output_l1 += prom_num_abs(output_delta);
    if (prom_num_abs(input_delta) > out_summary->input_linfinity)
      out_summary->input_linfinity = prom_num_abs(input_delta);
    if (prom_num_abs(output_delta) > out_summary->output_linfinity)
      out_summary->output_linfinity = prom_num_abs(output_delta);
  }
  if (input_energy == 0.0) return 0;
  out_summary->input_l2 = sqrt(input_energy);
  out_summary->output_l2 = sqrt(output_energy);
  out_summary->l1_gain = out_summary->output_l1 / out_summary->input_l1;
  out_summary->l2_gain = out_summary->output_l2 / out_summary->input_l2;
  out_summary->linfinity_gain = out_summary->output_linfinity /
                                out_summary->input_linfinity;
  out_summary->global_gain = out_summary->l2_gain;
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

int prom_num_suffix_identity_build(const prom_num_suffix_identity_request* request,
                                   prom_num_suffix_identity* out_identity) {
  uint64_t hash = 1469598103934665603ull;
  if (out_identity == NULL) return 0;
  memset(out_identity, 0, sizeof(*out_identity));
  if (request == NULL || request->stage < PROM_NUM_STAGE_OUTPUT_PROJECTION ||
      request->stage > PROM_NUM_STAGE_FFN_SUFFIX ||
      request->path < PROM_NUM_PATH_CPU_FP32 ||
      request->path > PROM_NUM_PATH_CPU_FP64_ORACLE ||
      request->tokens == 0u || request->input_channels == 0u ||
      request->output_channels == 0u || request->precision_contract == 0u ||
      request->input_generation == 0u || request->weight_generation == 0u ||
      request->input_hash == 0u || request->reference_input_hash == 0u ||
      request->source_hash == 0u) return 0;
  out_identity->exact_input_identity = request->input_hash;
  out_identity->matched_input = request->input_hash == request->reference_input_hash ? 1u : 0u;
  if (out_identity->matched_input == 0u) return 0;
  hash = prom_num_hash_u64(hash, PROM_NUM_RESEARCH_SCHEMA_VERSION);
  hash = prom_num_hash_u64(hash, request->stage);
  hash = prom_num_hash_u64(hash, request->path);
  hash = prom_num_hash_u64(hash, request->tokens);
  hash = prom_num_hash_u64(hash, request->input_channels);
  hash = prom_num_hash_u64(hash, request->output_channels);
  hash = prom_num_hash_u64(hash, request->precision_contract);
  hash = prom_num_hash_u64(hash, request->input_generation);
  hash = prom_num_hash_u64(hash, request->weight_generation);
  hash = prom_num_hash_u64(hash, request->input_hash);
  hash = prom_num_hash_u64(hash, request->source_hash);
  out_identity->replay_identity = hash;
  out_identity->valid = 1u;
  return 1;
}

static uint16_t prom_num_float_to_half_rne(float value) {
  uint32_t bits = 0u;
  uint32_t sign;
  uint32_t exponent;
  uint32_t mantissa;
  int32_t half_exponent;
  memcpy(&bits, &value, sizeof(bits));
  sign = (bits >> 16u) & 0x8000u;
  exponent = (bits >> 23u) & 0xffu;
  mantissa = bits & 0x7fffffu;
  if (exponent == 0xffu)
    return (uint16_t)(sign | (mantissa == 0u ? 0x7c00u : 0x7e00u));
  half_exponent = (int32_t)exponent - 127 + 15;
  if (half_exponent >= 31) return (uint16_t)(sign | 0x7c00u);
  if (half_exponent <= 0) {
    uint32_t shifted;
    uint32_t remainder;
    uint32_t halfway;
    uint32_t shift;
    if (half_exponent < -10) return (uint16_t)sign;
    mantissa |= 0x800000u;
    shift = (uint32_t)(14 - half_exponent);
    shifted = mantissa >> shift;
    remainder = mantissa & ((1u << shift) - 1u);
    halfway = 1u << (shift - 1u);
    if (remainder > halfway || (remainder == halfway && (shifted & 1u) != 0u)) shifted += 1u;
    return (uint16_t)(sign | shifted);
  }
  {
    uint32_t rounded = mantissa >> 13u;
    const uint32_t remainder = mantissa & 0x1fffu;
    if (remainder > 0x1000u || (remainder == 0x1000u && (rounded & 1u) != 0u)) {
      rounded += 1u;
      if (rounded == 0x400u) {
        rounded = 0u;
        half_exponent += 1;
        if (half_exponent >= 31) return (uint16_t)(sign | 0x7c00u);
      }
    }
    return (uint16_t)(sign | ((uint32_t)half_exponent << 10u) | rounded);
  }
}

static float prom_num_half_to_float(uint16_t value) {
  const uint32_t sign = ((uint32_t)value & 0x8000u) << 16u;
  uint32_t exponent = ((uint32_t)value >> 10u) & 0x1fu;
  uint32_t mantissa = (uint32_t)value & 0x3ffu;
  uint32_t bits;
  float result;
  if (exponent == 0u) {
    if (mantissa == 0u) bits = sign;
    else {
      int32_t adjusted = -14;
      while ((mantissa & 0x400u) == 0u) { mantissa <<= 1u; adjusted -= 1; }
      mantissa &= 0x3ffu;
      bits = sign | ((uint32_t)(adjusted + 127) << 23u) | (mantissa << 13u);
    }
  } else if (exponent == 31u) {
    bits = sign | 0x7f800000u | (mantissa << 13u);
  } else {
    bits = sign | ((exponent - 15u + 127u) << 23u) | (mantissa << 13u);
  }
  memcpy(&result, &bits, sizeof(result));
  return result;
}

static float prom_num_round_fp16(float value) {
  return prom_num_half_to_float(prom_num_float_to_half_rne(value));
}

int prom_num_generate_perturbation(uint32_t family, uint64_t seed,
                                   double magnitude, const float* base,
                                   const float* residual,
                                   const float* natural_discrepancy,
                                   float* perturbation, uint32_t tokens,
                                   uint32_t channels,
                                   prom_num_perturbation_summary* out_summary) {
  uint64_t count;
  uint64_t index;
  uint64_t state = seed == 0u ? 0xa0761d6478bd642full : seed;
  double raw_l2 = 0.0;
  double scale = magnitude;
  if (out_summary == NULL) return 0;
  memset(out_summary, 0, sizeof(*out_summary));
  if (perturbation == NULL || tokens == 0u || channels == 0u ||
      magnitude <= 0.0 || !isfinite(magnitude) ||
      family < PROM_NUM_PERTURB_ONE_COORDINATE ||
      family > PROM_NUM_PERTURB_NATURAL_LAYER_DISCREPANCY) return 0;
  count = (uint64_t)tokens * channels;
  if (count / channels != tokens) return 0;
  memset(perturbation, 0, (size_t)(count * sizeof(float)));
  if (family == PROM_NUM_PERTURB_ONE_COORDINATE) {
    perturbation[prom_num_next_u64(&state) % count] = 1.0f;
  } else if (family == PROM_NUM_PERTURB_ONE_TOKEN) {
    const uint32_t token = (uint32_t)(prom_num_next_u64(&state) % tokens);
    for (index = (uint64_t)token * channels; index < (uint64_t)(token + 1u) * channels; ++index)
      perturbation[index] = (index & 1u) == 0u ? 1.0f : -1.0f;
  } else if (family == PROM_NUM_PERTURB_ONE_CHANNEL) {
    const uint32_t channel = (uint32_t)(prom_num_next_u64(&state) % channels);
    for (index = channel; index < count; index += channels)
      perturbation[index] = ((index / channels) & 1u) == 0u ? 1.0f : -1.0f;
  } else if (family == PROM_NUM_PERTURB_ALTERNATING_SIGN) {
    for (index = 0u; index < count; ++index) perturbation[index] = (index & 1u) == 0u ? 1.0f : -1.0f;
  } else if (family == PROM_NUM_PERTURB_DENSE_SIGNED) {
    for (index = 0u; index < count; ++index)
      perturbation[index] = (prom_num_next_u64(&state) & 1u) == 0u ? 1.0f : -1.0f;
  } else if (family == PROM_NUM_PERTURB_ALIGNED_RESIDUAL ||
             family == PROM_NUM_PERTURB_DECORRELATED_RESIDUAL) {
    double projection = 0.0;
    double residual_energy = 0.0;
    if (residual == NULL) return 0;
    for (index = 0u; index < count; ++index) {
      if (!isfinite(residual[index])) return 0;
      if (family == PROM_NUM_PERTURB_ALIGNED_RESIDUAL) perturbation[index] = residual[index];
      else perturbation[index] = (prom_num_next_u64(&state) & 1u) == 0u ? 1.0f : -1.0f;
      projection += (double)perturbation[index] * residual[index];
      residual_energy += (double)residual[index] * residual[index];
    }
    if (family == PROM_NUM_PERTURB_DECORRELATED_RESIDUAL && residual_energy > 0.0)
      for (index = 0u; index < count; ++index)
        perturbation[index] = (float)((double)perturbation[index] -
                              projection / residual_energy * residual[index]);
  } else if (family == PROM_NUM_PERTURB_FP16_BIN_BOUNDARY) {
    if (base == NULL) return 0;
    for (index = 0u; index < count; ++index) {
      const float rounded = prom_num_round_fp16(base[index]);
      float toward = nextafterf(rounded, base[index] >= rounded ? INFINITY : -INFINITY);
      if (toward == rounded) toward = nextafterf(rounded, INFINITY);
      perturbation[index] = toward - base[index];
    }
  } else if (family == PROM_NUM_PERTURB_SPARSE_OUTLIER) {
    const uint64_t stride = count < 17u ? count : 17u;
    for (index = prom_num_next_u64(&state) % stride; index < count; index += stride)
      perturbation[index] = ((index / stride) & 1u) == 0u ? 8.0f : -8.0f;
  } else {
    if (natural_discrepancy == NULL) return 0;
    for (index = 0u; index < count; ++index) perturbation[index] = natural_discrepancy[index];
  }
  for (index = 0u; index < count; ++index) raw_l2 += (double)perturbation[index] * perturbation[index];
  if (raw_l2 == 0.0 || !isfinite(raw_l2)) return 0;
  scale /= sqrt(raw_l2);
  for (index = 0u; index < count; ++index) {
    const double value = (double)perturbation[index] * scale;
    const double absolute = prom_num_abs(value);
    perturbation[index] = (float)value;
    if (perturbation[index] != 0.0f) out_summary->nonzero_count += 1u;
    out_summary->l1_norm += absolute;
    out_summary->l2_norm += value * value;
    if (absolute > out_summary->linfinity_norm) out_summary->linfinity_norm = absolute;
  }
  out_summary->l2_norm = sqrt(out_summary->l2_norm);
  out_summary->family = family;
  out_summary->seed = seed;
  out_summary->requested_magnitude = magnitude;
  out_summary->identity = prom_num_hash_u64(prom_num_hash_float_bits(perturbation, count), family);
  out_summary->identity = prom_num_hash_u64(out_summary->identity, seed);
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

int prom_num_fp64_dot_oracle(const float* left, const float* right,
                             uint64_t count, uint32_t round_operands_to_fp16,
                             prom_num_fp64_dot_witness* out_witness) {
  uint64_t index;
  float fp32 = 0.0f;
  double fp64 = 0.0;
  if (out_witness == NULL) return 0;
  memset(out_witness, 0, sizeof(*out_witness));
  if (left == NULL || right == NULL || count == 0u || round_operands_to_fp16 > 1u)
    return 0;
  for (index = 0u; index < count; ++index) {
    const float a = round_operands_to_fp16 != 0u ? prom_num_round_fp16(left[index]) : left[index];
    const float b = round_operands_to_fp16 != 0u ? prom_num_round_fp16(right[index]) : right[index];
    if (!isfinite(a) || !isfinite(b)) return 0;
    fp32 = fp32 + a * b;
    fp64 += (double)a * (double)b;
  }
  out_witness->fp32_accumulation = fp32;
  out_witness->fp64_accumulation = fp64;
  out_witness->absolute_accumulation_difference = prom_num_abs((double)fp32 - fp64);
  out_witness->operand_count = count;
  out_witness->operands_rounded_to_fp16 = round_operands_to_fp16;
  out_witness->valid = 1u;
  return 1;
}

int prom_num_fp64_rms_oracle(const float* values, uint64_t count,
                             double epsilon,
                             prom_num_fp64_rms_witness* out_witness) {
  uint64_t index;
  float fp32 = 0.0f;
  double fp64 = 0.0;
  float fp32_mean;
  if (out_witness == NULL) return 0;
  memset(out_witness, 0, sizeof(*out_witness));
  if (values == NULL || count == 0u || epsilon < 0.0 || !isfinite(epsilon)) return 0;
  for (index = 0u; index < count; ++index) {
    const float value = values[index];
    if (!isfinite(value)) return 0;
    fp32 = fp32 + value * value;
    fp64 += (double)value * value;
  }
  fp32_mean = fp32 / (float)count;
  out_witness->fp32_sum_of_squares = fp32;
  out_witness->fp64_sum_of_squares = fp64;
  out_witness->fp32_inv_rms = 1.0 / sqrt((double)fp32_mean + epsilon);
  out_witness->fp64_inv_rms = 1.0 / sqrt(fp64 / (double)count + epsilon);
  out_witness->inv_rms_absolute_difference =
      prom_num_abs(out_witness->fp32_inv_rms - out_witness->fp64_inv_rms);
  out_witness->element_count = count;
  out_witness->valid = 1u;
  return 1;
}

int prom_num_envelope_fit(const prom_num_envelope_sample* samples,
                          uint64_t sample_count, uint32_t path,
                          uint32_t stage, uint32_t minimum_tokens,
                          uint32_t maximum_tokens, uint32_t minimum_width,
                          uint32_t maximum_width, prom_num_envelope* out_envelope,
                          prom_num_envelope_fit_summary* out_summary) {
  uint64_t index;
  double maximum_disturbance = 0.0;
  double maximum_bias = 0.0;
  double maximum_gain = 0.0;
  uint64_t identity = 1469598103934665603ull;
  if (out_envelope == NULL || out_summary == NULL) return 0;
  memset(out_envelope, 0, sizeof(*out_envelope));
  memset(out_summary, 0, sizeof(*out_summary));
  if (samples == NULL || sample_count == 0u || path == 0u || stage == 0u ||
      minimum_tokens == 0u || minimum_width == 0u ||
      maximum_tokens < minimum_tokens || maximum_width < minimum_width) return 0;
  for (index = 0u; index < sample_count; ++index) {
    const prom_num_envelope_sample* sample = &samples[index];
    if ((sample->split != PROM_NUM_SPLIT_IDENTIFICATION &&
         sample->split != PROM_NUM_SPLIT_HELD_OUT) ||
        sample->input_error < 0.0 || sample->output_error < 0.0 ||
        sample->local_disturbance < 0.0 || !isfinite(sample->signed_bias)) return 0;
    if (sample->split == PROM_NUM_SPLIT_IDENTIFICATION) {
      const double bias = prom_num_abs(sample->signed_bias);
      out_summary->identification_count += 1u;
      if (sample->local_disturbance > maximum_disturbance)
        maximum_disturbance = sample->local_disturbance;
      if (bias > maximum_bias) maximum_bias = bias;
    } else {
      out_summary->held_out_count += 1u;
    }
  }
  if (out_summary->identification_count == 0u || out_summary->held_out_count == 0u) return 0;
  for (index = 0u; index < sample_count; ++index) {
    const prom_num_envelope_sample* sample = &samples[index];
    if (sample->split == PROM_NUM_SPLIT_IDENTIFICATION && sample->input_error > 0.0) {
      double required = (sample->output_error - maximum_disturbance - maximum_bias) /
                        sample->input_error;
      if (required < 0.0) required = 0.0;
      if (required > maximum_gain) maximum_gain = required;
    }
  }
  out_envelope->path = path;
  out_envelope->stage = stage;
  out_envelope->minimum_tokens = minimum_tokens;
  out_envelope->maximum_tokens = maximum_tokens;
  out_envelope->minimum_width = minimum_width;
  out_envelope->maximum_width = maximum_width;
  out_envelope->local_disturbance_bound = maximum_disturbance;
  out_envelope->gain_bound = maximum_gain;
  out_envelope->bias_bound = maximum_bias;
  out_envelope->held_out_confidence = 1.0;
  identity = prom_num_hash_u64(identity, path);
  identity = prom_num_hash_u64(identity, stage);
  identity = prom_num_hash_u64(identity, out_summary->identification_count);
  out_envelope->identity = identity;
  for (index = 0u; index < sample_count; ++index) {
    const prom_num_envelope_sample* sample = &samples[index];
    if (sample->split == PROM_NUM_SPLIT_HELD_OUT) {
      const double bound = maximum_gain * sample->input_error +
                           maximum_disturbance + maximum_bias;
      const double excess = sample->output_error - bound;
      if (excess > 0.0) {
        out_summary->held_out_failure_count += 1u;
        if (excess > out_summary->worst_held_out_excess)
          out_summary->worst_held_out_excess = excess;
      }
    }
  }
  out_summary->held_out_pass_fraction =
      1.0 - (double)out_summary->held_out_failure_count /
                (double)out_summary->held_out_count;
  out_envelope->held_out_confidence = out_summary->held_out_pass_fraction;
  out_summary->valid = 1u;
  return 1;
}

int prom_num_mitigation_eligible(const prom_num_mitigation_evidence* evidence,
                                 double maximum_held_out_regression,
                                 uint32_t* out_eligible) {
  if (out_eligible == NULL) return 0;
  *out_eligible = 0u;
  if (evidence == NULL || maximum_held_out_regression < 0.0 ||
      evidence->identification_count == 0u || evidence->held_out_count == 0u ||
      evidence->identification_baseline_error < 0.0 ||
      evidence->identification_mitigated_error < 0.0 ||
      evidence->held_out_baseline_error < 0.0 ||
      evidence->held_out_mitigated_error < 0.0 ||
      evidence->latency_microseconds < 0.0) return 0;
  *out_eligible = evidence->identification_mitigated_error <
                      evidence->identification_baseline_error &&
                  evidence->held_out_mitigated_error < evidence->held_out_baseline_error &&
                  evidence->worst_held_out_regression <= maximum_held_out_regression
                      ? 1u : 0u;
  return 1;
}

int prom_num_canary_calibrate(const double* canary_scores,
                              const double* full_tensor_errors,
                              uint64_t count, double canary_threshold,
                              double error_threshold,
                              prom_num_canary_calibration* out_calibration) {
  uint64_t index;
  double canary_mean = 0.0;
  double error_mean = 0.0;
  double numerator = 0.0;
  double canary_energy = 0.0;
  double error_energy = 0.0;
  if (out_calibration == NULL) return 0;
  memset(out_calibration, 0, sizeof(*out_calibration));
  if (canary_scores == NULL || full_tensor_errors == NULL || count == 0u ||
      canary_threshold < 0.0 || error_threshold < 0.0) return 0;
  for (index = 0u; index < count; ++index) {
    if (!isfinite(canary_scores[index]) || !isfinite(full_tensor_errors[index])) return 0;
    canary_mean += canary_scores[index];
    error_mean += full_tensor_errors[index];
  }
  canary_mean /= (double)count;
  error_mean /= (double)count;
  for (index = 0u; index < count; ++index) {
    const uint32_t predicted = canary_scores[index] >= canary_threshold ? 1u : 0u;
    const uint32_t actual = full_tensor_errors[index] >= error_threshold ? 1u : 0u;
    const double centered_canary = canary_scores[index] - canary_mean;
    const double centered_error = full_tensor_errors[index] - error_mean;
    if (predicted != 0u && actual != 0u) out_calibration->true_positive += 1u;
    else if (predicted == 0u && actual == 0u) out_calibration->true_negative += 1u;
    else if (predicted != 0u) out_calibration->false_positive += 1u;
    else out_calibration->false_negative += 1u;
    numerator += centered_canary * centered_error;
    canary_energy += centered_canary * centered_canary;
    error_energy += centered_error * centered_error;
  }
  out_calibration->sample_count = count;
  if (canary_energy > 0.0 && error_energy > 0.0)
    out_calibration->pearson_correlation = numerator / sqrt(canary_energy * error_energy);
  if (out_calibration->false_positive + out_calibration->true_negative > 0u)
    out_calibration->false_positive_rate =
        (double)out_calibration->false_positive /
        (double)(out_calibration->false_positive + out_calibration->true_negative);
  if (out_calibration->false_negative + out_calibration->true_positive > 0u)
    out_calibration->false_negative_rate =
        (double)out_calibration->false_negative /
        (double)(out_calibration->false_negative + out_calibration->true_positive);
  out_calibration->valid = 1u;
  return 1;
}
