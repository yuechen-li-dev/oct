#pragma once
/* Test-side error statistics for comparing device output against a CPU oracle.
 * Moved out of the reactor with the retired numerical-research module; only
 * tests use it. */

#include <math.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

typedef struct prom_num_error_summary {
  uint64_t element_count;
  uint64_t near_zero_reference_count;
  uint64_t exceeding_count;
  uint64_t positive_residual_count;
  uint64_t negative_residual_count;
  double maximum_absolute_error;
  double maximum_relative_error;
  double mean_absolute_error;
  double rms_error;
  double l1_norm;
  double l2_norm;
  double linfinity_norm;
  double cosine_similarity;
  double signed_mean_bias;
  double p50_absolute_error;
  double p90_absolute_error;
  double p95_absolute_error;
  double p99_absolute_error;
  double worst_token_l2_fraction;
  double worst_channel_l2_fraction;
  uint32_t worst_token;
  uint32_t worst_channel;
  uint32_t valid;
} prom_num_error_summary;

static inline double prom_num_abs(double value) {
  return value < 0.0 ? -value : value;
}

static inline int prom_num_compare_double(const void* left, const void* right) {
  const double a = *(const double*)left;
  const double b = *(const double*)right;
  if (a < b) return -1;
  if (a > b) return 1;
  return 0;
}

static inline double prom_num_percentile(const double* sorted, uint64_t count, uint32_t numerator) {
  uint64_t index;
  if (count == 0u) return 0.0;
  index = ((count - 1u) * numerator + 50u) / 100u;
  return sorted[index];
}

static inline int prom_num_summarize_error(const float* reference, const float* actual,
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
