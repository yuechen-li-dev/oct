#ifndef PROMETHEUS_REACTOR_NUMERICAL_RESEARCH_H
#define PROMETHEUS_REACTOR_NUMERICAL_RESEARCH_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

/* M49 is an audit-only numerical system-identification layer.  These types do
   not participate in product dispatch or selector authority. */

#define PROM_NUM_RESEARCH_SCHEMA_VERSION 1u
#define PROM_NUM_MAX_CANDIDATES 8u
#define PROM_NUM_MAX_CONSIDERATIONS 6u

typedef enum prom_num_path {
  PROM_NUM_PATH_CPU_FP32 = 1u,
  PROM_NUM_PATH_GPU_A2X4_FP32 = 2u,
  PROM_NUM_PATH_GPU_CONVENTIONAL_FP16 = 3u,
  PROM_NUM_PATH_GPU_COOPERATIVE_FP16 = 4u,
} prom_num_path;

typedef enum prom_num_stage {
  PROM_NUM_STAGE_BLOCK_INPUT = 1u,
  PROM_NUM_STAGE_ATTENTION = 2u,
  PROM_NUM_STAGE_OUTPUT_PROJECTION = 3u,
  PROM_NUM_STAGE_FIRST_RESIDUAL = 4u,
  PROM_NUM_STAGE_RMSNORM_INV_RMS = 5u,
  PROM_NUM_STAGE_RMSNORM_OUTPUT = 6u,
  PROM_NUM_STAGE_GATE = 7u,
  PROM_NUM_STAGE_UP = 8u,
  PROM_NUM_STAGE_HIDDEN = 9u,
  PROM_NUM_STAGE_DOWN = 10u,
  PROM_NUM_STAGE_SECOND_RESIDUAL = 11u,
  PROM_NUM_STAGE_COMPLETE_BLOCK = 12u,
} prom_num_stage;

typedef enum prom_num_input_family {
  PROM_NUM_INPUT_LOW_AMPLITUDE = 1u,
  PROM_NUM_INPUT_WIDE_AMPLITUDE = 2u,
  PROM_NUM_INPUT_ALTERNATING_SIGN = 3u,
  PROM_NUM_INPUT_SPARSE = 4u,
  PROM_NUM_INPUT_CANCELLATION_HEAVY = 5u,
  PROM_NUM_INPUT_MONOTONIC_RAMP = 6u,
  PROM_NUM_INPUT_NEAR_FP16_MIDPOINT = 7u,
  PROM_NUM_INPUT_OUTLIER_CHANNEL = 8u,
  PROM_NUM_INPUT_M48_LEGACY = 9u,
} prom_num_input_family;

typedef enum prom_num_corpus_split {
  PROM_NUM_SPLIT_IDENTIFICATION = 1u,
  PROM_NUM_SPLIT_HELD_OUT = 2u,
} prom_num_corpus_split;

typedef enum prom_num_determinism_class {
  PROM_NUM_DETERMINISM_UNIDENTIFIED = 0u,
  PROM_NUM_DETERMINISM_BITWISE = 1u,
  PROM_NUM_DETERMINISM_ENVELOPE = 2u,
  PROM_NUM_DETERMINISM_NONDETERMINISTIC = 3u,
  PROM_NUM_DETERMINISM_INVALID = 4u,
} prom_num_determinism_class;

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

typedef struct prom_num_correlation_summary {
  double residual_reference_correlation;
  double residual_lag1_autocorrelation;
  double signed_persistence_fraction;
  double channel_recurrence_fraction;
  uint32_t valid;
} prom_num_correlation_summary;

typedef struct prom_num_gain_summary {
  double input_l2;
  double output_l2;
  double global_gain;
  double maximum_token_gain;
  double maximum_channel_gain;
  double minimum_token_gain;
  uint32_t maximum_token;
  uint32_t maximum_channel;
  uint32_t valid;
} prom_num_gain_summary;

typedef struct prom_num_determinism_tracker {
  uint64_t sample_count;
  uint64_t first_hash;
  uint64_t distinct_from_first_count;
  double maximum_absolute_deviation;
  uint32_t saw_invalid;
} prom_num_determinism_tracker;

typedef struct prom_num_canary_summary {
  double l1_norm;
  double l2_norm;
  double maximum_absolute_value;
  double signed_projection_a;
  double signed_projection_b;
  double absolute_projection;
  uint64_t bit_hash;
  uint32_t valid;
} prom_num_canary_summary;

typedef enum prom_num_regime {
  PROM_NUM_REGIME_UNIDENTIFIED = 0u,
  PROM_NUM_REGIME_NOMINAL = 1u,
  PROM_NUM_REGIME_BOUNDED_DRIFT = 2u,
  PROM_NUM_REGIME_HIGH_INJECTION = 3u,
  PROM_NUM_REGIME_HIGH_GAIN = 4u,
  PROM_NUM_REGIME_REFERENCE_SUSPECT = 5u,
  PROM_NUM_REGIME_PRECISION_PROMOTION_RECOMMENDED = 6u,
  PROM_NUM_REGIME_BACKEND_FALLBACK_RECOMMENDED = 7u,
  PROM_NUM_REGIME_AUDIT_REQUIRED = 8u,
  PROM_NUM_REGIME_QUARANTINED = 9u,
} prom_num_regime;

typedef struct prom_num_observer_params {
  double nominal_local_error;
  double high_injection_error;
  double high_gain;
  double severe_gain;
  double concentration_limit;
  uint32_t enter_ticks;
  uint32_t clear_ticks;
} prom_num_observer_params;

typedef struct prom_num_observer_evidence {
  uint32_t valid;
  uint32_t implementation_defect;
  uint32_t reference_disagreement;
  uint32_t envelope_exceeded;
  uint32_t fallback_available;
  uint32_t precision_promotion_available;
  uint32_t hardware_fault;
  uint32_t deterministic_class;
  double local_disturbance_l2;
  double inherited_error_l2;
  double gain;
  double correlation_concentration;
} prom_num_observer_evidence;

typedef struct prom_num_observer_state {
  uint32_t regime;
  uint32_t pending_regime;
  uint32_t pending_ticks;
  uint32_t clear_ticks;
  uint64_t evaluation_count;
  uint64_t transition_count;
} prom_num_observer_state;

typedef enum prom_num_action {
  PROM_NUM_ACTION_ACCEPT = 1u,
  PROM_NUM_ACTION_COOPERATIVE_FP16 = 2u,
  PROM_NUM_ACTION_CONVENTIONAL_FP16 = 3u,
  PROM_NUM_ACTION_A2X4_FP32 = 4u,
  PROM_NUM_ACTION_SELECTIVE_FP32_STAGE = 5u,
  PROM_NUM_ACTION_PERIODIC_FP32_CHECKPOINT = 6u,
  PROM_NUM_ACTION_STAGE_AUDIT = 7u,
  PROM_NUM_ACTION_REJECT_OR_QUARANTINE = 8u,
} prom_num_action;

typedef enum prom_num_consideration {
  PROM_NUM_CONSIDERATION_RISK_REDUCTION = 1u,
  PROM_NUM_CONSIDERATION_LATENCY_COST = 2u,
  PROM_NUM_CONSIDERATION_MEMORY_COST = 3u,
  PROM_NUM_CONSIDERATION_PORTABILITY = 4u,
  PROM_NUM_CONSIDERATION_COMPLEXITY = 5u,
  PROM_NUM_CONSIDERATION_EVIDENCE_CONFIDENCE = 6u,
} prom_num_consideration;

typedef struct prom_num_candidate {
  uint32_t action;
  uint32_t eligible;
  uint32_t ineligible_reason;
  double predicted_error;
  double latency_microseconds;
  uint64_t retained_bytes;
  double portability;
  double complexity;
  double confidence;
  double score;
  double contribution[PROM_NUM_MAX_CONSIDERATIONS];
} prom_num_candidate;

typedef struct prom_num_shadow_decision {
  uint32_t proposed_action;
  uint32_t authoritative_action;
  uint32_t candidate_count;
  uint32_t would_change_authority;
  uint64_t decision_identity;
  prom_num_candidate candidate[PROM_NUM_MAX_CANDIDATES];
} prom_num_shadow_decision;

typedef struct prom_num_envelope {
  uint32_t path;
  uint32_t stage;
  uint32_t minimum_tokens;
  uint32_t maximum_tokens;
  uint32_t minimum_width;
  uint32_t maximum_width;
  double local_disturbance_bound;
  double gain_bound;
  double bias_bound;
  double held_out_confidence;
  uint64_t identity;
} prom_num_envelope;

typedef struct prom_num_envelope_result {
  uint32_t supported;
  uint32_t within_envelope;
  double bound;
  double observed_error;
} prom_num_envelope_result;

typedef struct prom_num_bias_model {
  double bias;
  uint64_t fitted_count;
  uint32_t valid;
} prom_num_bias_model;

uint64_t prom_num_hash_float_bits(const float* values, uint64_t count);
uint64_t prom_num_experiment_plan_identity(uint32_t schema_version,
                                           const uint32_t* paths,
                                           uint32_t path_count,
                                           const uint32_t* stages,
                                           uint32_t stage_count,
                                           uint64_t corpus_identity);
uint32_t prom_num_corpus_split_for_case(uint64_t case_identity);
int prom_num_generate_input(uint32_t family, uint64_t seed, float* values,
                            uint32_t tokens, uint32_t channels);
int prom_num_summarize_error(const float* reference, const float* actual,
                             uint32_t tokens, uint32_t channels,
                             double near_zero_floor, double absolute_bound,
                             double relative_bound, double* scratch,
                             uint64_t scratch_count,
                             prom_num_error_summary* out_summary);
int prom_num_summarize_correlation(const float* reference, const float* actual,
                                   uint32_t tokens, uint32_t channels,
                                   prom_num_correlation_summary* out_summary);
int prom_num_summarize_gain(const float* input_a, const float* input_b,
                            const float* output_a, const float* output_b,
                            uint32_t tokens, uint32_t channels,
                            prom_num_gain_summary* out_summary);
void prom_num_determinism_init(prom_num_determinism_tracker* tracker);
int prom_num_determinism_update(prom_num_determinism_tracker* tracker,
                                const float* baseline, const float* sample,
                                uint64_t count);
uint32_t prom_num_determinism_classify(const prom_num_determinism_tracker* tracker,
                                       double fixed_envelope);
int prom_num_canary_measure(const float* values, uint32_t tokens,
                            uint32_t channels, uint64_t seed,
                            prom_num_canary_summary* out_summary);
prom_num_observer_params prom_num_observer_default_params(void);
void prom_num_observer_init(prom_num_observer_state* state);
uint32_t prom_num_observer_update(prom_num_observer_state* state,
                                  const prom_num_observer_params* params,
                                  const prom_num_observer_evidence* evidence);
int prom_num_shadow_select(uint32_t authoritative_action, uint32_t regime,
                           double configured_error_envelope,
                           prom_num_candidate* candidates,
                           uint32_t candidate_count,
                           prom_num_shadow_decision* out_decision);
int prom_num_envelope_evaluate(const prom_num_envelope* envelope,
                               uint32_t path, uint32_t stage,
                               uint32_t tokens, uint32_t width,
                               double input_error, double observed_error,
                               prom_num_envelope_result* out_result);
int prom_num_bias_fit(const float* reference, const float* observed,
                      uint64_t count, prom_num_bias_model* out_model);
int prom_num_bias_evaluate(const prom_num_bias_model* model,
                           const float* reference, const float* observed,
                           uint64_t count, double* out_uncorrected_rms,
                           double* out_corrected_rms);

#ifdef __cplusplus
}
#endif

#endif
