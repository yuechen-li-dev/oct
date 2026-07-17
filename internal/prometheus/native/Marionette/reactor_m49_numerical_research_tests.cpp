#include "../reactor_numerical_research.h"
#include "test_harness.h"

#include <array>
#include <cmath>
#include <cstdint>
#include <fstream>
#include <iterator>
#include <limits>
#include <string>
#include <vector>

FACT(PrometheusM49ExperimentPlanIdentityAndCorpusSplitAreDeterministic)
{
    const std::array<std::uint32_t, 4u> paths{{
        PROM_NUM_PATH_CPU_FP32,
        PROM_NUM_PATH_GPU_A2X4_FP32,
        PROM_NUM_PATH_GPU_CONVENTIONAL_FP16,
        PROM_NUM_PATH_GPU_COOPERATIVE_FP16,
    }};
    const std::array<std::uint32_t, 5u> stages{{
        PROM_NUM_STAGE_ATTENTION,
        PROM_NUM_STAGE_OUTPUT_PROJECTION,
        PROM_NUM_STAGE_FIRST_RESIDUAL,
        PROM_NUM_STAGE_RMSNORM_OUTPUT,
        PROM_NUM_STAGE_COMPLETE_BLOCK,
    }};
    const std::uint64_t first = prom_num_experiment_plan_identity(
        PROM_NUM_RESEARCH_SCHEMA_VERSION, paths.data(),
        static_cast<std::uint32_t>(paths.size()), stages.data(),
        static_cast<std::uint32_t>(stages.size()), 490001u);
    const std::uint64_t repeated = prom_num_experiment_plan_identity(
        PROM_NUM_RESEARCH_SCHEMA_VERSION, paths.data(),
        static_cast<std::uint32_t>(paths.size()), stages.data(),
        static_cast<std::uint32_t>(stages.size()), 490001u);
    ASSERT_TRUE(first != 0u, "M49 plan identity is nonzero");
    ASSERT_EQUAL(first, repeated, "identical research plans have identical identity");
    ASSERT_TRUE(first != prom_num_experiment_plan_identity(
                             PROM_NUM_RESEARCH_SCHEMA_VERSION, paths.data(),
                             static_cast<std::uint32_t>(paths.size()), stages.data(),
                             static_cast<std::uint32_t>(stages.size()), 490002u),
                "corpus identity participates in research-plan identity");
    for (std::uint64_t caseIdentity = 1u; caseIdentity <= 100u; ++caseIdentity) {
        const std::uint32_t split = prom_num_corpus_split_for_case(caseIdentity);
        ASSERT_EQUAL(split, prom_num_corpus_split_for_case(caseIdentity),
                     "identification/held-out assignment is repeatable");
        ASSERT_TRUE(split == PROM_NUM_SPLIT_IDENTIFICATION ||
                        split == PROM_NUM_SPLIT_HELD_OUT,
                    "every case belongs to exactly one corpus split");
    }
}

FACT(PrometheusM49DeterministicCorpusFamiliesAreDistinct)
{
    constexpr std::uint32_t tokens = 4u;
    constexpr std::uint32_t channels = 16u;
    std::array<float, tokens * channels> first{};
    std::array<float, tokens * channels> repeated{};
    std::array<float, tokens * channels> other{};
    for (std::uint32_t family = PROM_NUM_INPUT_LOW_AMPLITUDE;
         family <= PROM_NUM_INPUT_M48_LEGACY; ++family) {
        ASSERT_TRUE(prom_num_generate_input(family, 4900u, first.data(), tokens, channels) != 0,
                    "each bounded M49 input family generates");
        ASSERT_TRUE(prom_num_generate_input(family, 4900u, repeated.data(), tokens, channels) != 0,
                    "each bounded M49 input family repeats");
        ASSERT_EQUAL(prom_num_hash_float_bits(first.data(), first.size()),
                     prom_num_hash_float_bits(repeated.data(), repeated.size()),
                     "same family and seed reproduce exact bits");
    }
    ASSERT_TRUE(prom_num_generate_input(PROM_NUM_INPUT_LOW_AMPLITUDE, 1u, first.data(),
                                        tokens, channels) != 0,
                "low-amplitude family generates");
    ASSERT_TRUE(prom_num_generate_input(PROM_NUM_INPUT_CANCELLATION_HEAVY, 1u, other.data(),
                                        tokens, channels) != 0,
                "cancellation family generates");
    ASSERT_TRUE(prom_num_hash_float_bits(first.data(), first.size()) !=
                    prom_num_hash_float_bits(other.data(), other.size()),
                "families do not collapse to one synthetic tensor");
}

FACT(PrometheusM49NormsBiasPercentilesAndNearZeroRelativeError)
{
    const std::array<float, 4u> reference{{0.0f, 1.0f, -2.0f, 4.0f}};
    const std::array<float, 4u> observed{{0.1f, 1.2f, -1.5f, 3.0f}};
    std::array<double, 4u> scratch{};
    prom_num_error_summary summary{};
    ASSERT_TRUE(prom_num_summarize_error(reference.data(), observed.data(), 2u, 2u,
                                         0.25, 0.4, 0.3, scratch.data(), scratch.size(),
                                         &summary) != 0,
                "bounded error summary succeeds");
    ASSERT_EQUAL(1u, summary.valid, "error summary is explicitly valid");
    ASSERT_EQUAL(static_cast<std::uint64_t>(4u), summary.element_count,
                 "all logical elements are measured");
    ASSERT_EQUAL(static_cast<std::uint64_t>(1u), summary.near_zero_reference_count,
                 "near-zero reference handling is counted");
    ASSERT_TRUE(std::abs(summary.l1_norm - 1.8) < 1.0e-6, "L1 norm is correct");
    ASSERT_TRUE(std::abs(summary.rms_error - std::sqrt(1.3 / 4.0)) < 1.0e-6,
                "RMS error is correct");
    ASSERT_TRUE(std::abs(summary.signed_mean_bias + 0.05) < 1.0e-6,
                "signed bias is not discarded");
    ASSERT_TRUE(summary.maximum_relative_error < 1.0,
                "near-zero denominator floor prevents meaningless explosion");
    ASSERT_TRUE(summary.p50_absolute_error <= summary.p90_absolute_error &&
                    summary.p90_absolute_error <= summary.p99_absolute_error,
                "absolute-error percentiles are ordered");
    ASSERT_TRUE(summary.worst_token_l2_fraction > 0.0 &&
                    summary.worst_channel_l2_fraction > 0.0,
                "token and channel concentration are reported");
}

FACT(PrometheusM49GainAndCorrelationPreserveDirectionAndConcentration)
{
    const std::array<float, 8u> inputA{{0, 0, 0, 0, 0, 0, 0, 0}};
    const std::array<float, 8u> inputB{{1, -1, 2, -2, 1, -1, 2, -2}};
    const std::array<float, 8u> outputA{{0, 0, 0, 0, 0, 0, 0, 0}};
    const std::array<float, 8u> outputB{{2, -2, 4, -4, 2, -2, 4, -4}};
    prom_num_gain_summary gain{};
    ASSERT_TRUE(prom_num_summarize_gain(inputA.data(), inputB.data(), outputA.data(),
                                        outputB.data(), 2u, 4u, &gain) != 0,
                "gain summary succeeds");
    ASSERT_TRUE(std::abs(gain.global_gain - 2.0) < 1.0e-12,
                "global disturbance gain is exact for a linear witness");
    ASSERT_TRUE(std::abs(gain.maximum_token_gain - 2.0) < 1.0e-12 &&
                    std::abs(gain.maximum_channel_gain - 2.0) < 1.0e-12,
                "token and channel gain remain visible");

    const std::array<float, 8u> reference{{1, 2, 3, 4, 1, 2, 3, 4}};
    const std::array<float, 8u> observed{{1.1f, 2.2f, 3.3f, 4.4f,
                                         1.1f, 2.2f, 3.3f, 4.4f}};
    prom_num_correlation_summary correlation{};
    ASSERT_TRUE(prom_num_summarize_correlation(reference.data(), observed.data(),
                                               2u, 4u, &correlation) != 0,
                "structured correlation summary succeeds");
    ASSERT_TRUE(correlation.residual_reference_correlation > 0.9,
                "magnitude-dependent residual structure is detected");
    ASSERT_TRUE(correlation.channel_recurrence_fraction > 0.9,
                "channel-sign recurrence across depth-like rows is detected");
}

FACT(PrometheusM49RepeatedExecutionClassificationSeparatesDisagreementFromNoise)
{
    const std::array<float, 4u> baseline{{1.0f, 2.0f, 3.0f, 4.0f}};
    prom_num_determinism_tracker exact{};
    prom_num_determinism_init(&exact);
    for (std::uint32_t run = 0u; run < 100u; ++run)
        ASSERT_TRUE(prom_num_determinism_update(&exact, baseline.data(), baseline.data(),
                                               baseline.size()) != 0,
                    "identical output participates in repeat characterization");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_NUM_DETERMINISM_BITWISE),
                 prom_num_determinism_classify(&exact, 0.0),
                 "100 exact repeats classify as bitwise deterministic");

    std::array<float, 4u> nearby = baseline;
    nearby[2] += 1.0e-5f;
    prom_num_determinism_tracker envelope{};
    prom_num_determinism_init(&envelope);
    ASSERT_TRUE(prom_num_determinism_update(&envelope, baseline.data(), baseline.data(),
                                           baseline.size()) != 0,
                "envelope baseline records");
    ASSERT_TRUE(prom_num_determinism_update(&envelope, baseline.data(), nearby.data(),
                                           nearby.size()) != 0,
                "nearby output records");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_NUM_DETERMINISM_ENVELOPE),
                 prom_num_determinism_classify(&envelope, 2.0e-5),
                 "bounded run variance is not mislabeled as path discrepancy");
}

FACT(PrometheusM49NumericalObserverUsesExplicitTransitionsAndHysteresis)
{
    prom_num_observer_state state{};
    prom_num_observer_init(&state);
    const prom_num_observer_params params = prom_num_observer_default_params();
    prom_num_observer_evidence nominal{};
    nominal.valid = 1u;
    nominal.deterministic_class = PROM_NUM_DETERMINISM_BITWISE;
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_NUM_REGIME_NOMINAL),
                 prom_num_observer_update(&state, &params, &nominal),
                 "first valid evidence identifies the nominal regime");

    prom_num_observer_evidence highGain = nominal;
    highGain.gain = 1.5;
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_NUM_REGIME_NOMINAL),
                 prom_num_observer_update(&state, &params, &highGain),
                 "one high-gain sample does not thrash the observer");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_NUM_REGIME_HIGH_GAIN),
                 prom_num_observer_update(&state, &params, &highGain),
                 "repeated high-gain evidence enters the explicit regime");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_NUM_REGIME_HIGH_GAIN),
                 prom_num_observer_update(&state, &params, &nominal),
                 "one nominal sample does not clear a risk regime");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_NUM_REGIME_HIGH_GAIN),
                 prom_num_observer_update(&state, &params, &nominal),
                 "clear hysteresis requires its complete dwell");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_NUM_REGIME_NOMINAL),
                 prom_num_observer_update(&state, &params, &nominal),
                 "the configured clear dwell returns to nominal");

    prom_num_observer_evidence suspect = nominal;
    suspect.reference_disagreement = 1u;
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_NUM_REGIME_REFERENCE_SUSPECT),
                 prom_num_observer_update(&state, &params, &suspect),
                 "reference disagreement is distinct from high injection");
    prom_num_observer_evidence fault = nominal;
    fault.hardware_fault = 1u;
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_NUM_REGIME_QUARANTINED),
                 prom_num_observer_update(&state, &params, &fault),
                 "hardware/runtime faults quarantine immediately");
}

FACT(PrometheusM49ShadowPolicyIsDeterministicAndNeverChangesAuthority)
{
    std::array<prom_num_candidate, 3u> candidates{};
    candidates[0].action = PROM_NUM_ACTION_ACCEPT;
    candidates[0].eligible = 1u;
    candidates[0].predicted_error = 0.02;
    candidates[0].latency_microseconds = 0.0;
    candidates[0].portability = 1.0;
    candidates[0].confidence = 0.95;
    candidates[1].action = PROM_NUM_ACTION_CONVENTIONAL_FP16;
    candidates[1].eligible = 1u;
    candidates[1].predicted_error = 0.005;
    candidates[1].latency_microseconds = 100.0;
    candidates[1].portability = 0.9;
    candidates[1].complexity = 1.0;
    candidates[1].confidence = 0.9;
    candidates[2].action = PROM_NUM_ACTION_A2X4_FP32;
    candidates[2].eligible = 1u;
    candidates[2].predicted_error = 0.001;
    candidates[2].latency_microseconds = 300.0;
    candidates[2].portability = 1.0;
    candidates[2].complexity = 0.5;
    candidates[2].confidence = 0.95;
    prom_num_shadow_decision first{};
    prom_num_shadow_decision repeated{};
    ASSERT_TRUE(prom_num_shadow_select(PROM_NUM_ACTION_COOPERATIVE_FP16,
                                       PROM_NUM_REGIME_HIGH_GAIN, 0.01,
                                       candidates.data(),
                                       static_cast<std::uint32_t>(candidates.size()), &first) != 0,
                "bounded shadow policy chooses among eligible candidates");
    ASSERT_TRUE(prom_num_shadow_select(PROM_NUM_ACTION_COOPERATIVE_FP16,
                                       PROM_NUM_REGIME_HIGH_GAIN, 0.01,
                                       candidates.data(),
                                       static_cast<std::uint32_t>(candidates.size()), &repeated) != 0,
                "bounded shadow policy repeats");
    ASSERT_EQUAL(first.proposed_action, repeated.proposed_action,
                 "shadow recommendation is deterministic");
    ASSERT_EQUAL(first.decision_identity, repeated.decision_identity,
                 "shadow trace identity is deterministic");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_NUM_ACTION_COOPERATIVE_FP16),
                 first.authoritative_action,
                 "normal product authority remains recorded and unchanged");
    ASSERT_EQUAL(1u, first.would_change_authority,
                 "a shadow-only counter records the proposed difference");
}

FACT(PrometheusM49EnvelopeEvaluationRejectsUnsupportedExtrapolation)
{
    prom_num_envelope envelope{};
    envelope.path = PROM_NUM_PATH_GPU_CONVENTIONAL_FP16;
    envelope.stage = PROM_NUM_STAGE_COMPLETE_BLOCK;
    envelope.minimum_tokens = 16u;
    envelope.maximum_tokens = 256u;
    envelope.minimum_width = 128u;
    envelope.maximum_width = 1024u;
    envelope.local_disturbance_bound = 0.01;
    envelope.gain_bound = 1.5;
    envelope.bias_bound = 0.002;
    envelope.held_out_confidence = 0.95;
    prom_num_envelope_result result{};
    ASSERT_TRUE(prom_num_envelope_evaluate(&envelope,
                                           PROM_NUM_PATH_GPU_CONVENTIONAL_FP16,
                                           PROM_NUM_STAGE_COMPLETE_BLOCK,
                                           128u, 1024u, 0.02, 0.04, &result) != 0,
                "supported envelope evaluates");
    ASSERT_EQUAL(1u, result.supported, "identified shape/path/stage is supported");
    ASSERT_EQUAL(1u, result.within_envelope,
                 "stage bound follows gain*input plus disturbance and bias");
    ASSERT_TRUE(prom_num_envelope_evaluate(&envelope,
                                           PROM_NUM_PATH_GPU_COOPERATIVE_FP16,
                                           PROM_NUM_STAGE_COMPLETE_BLOCK,
                                           128u, 1024u, 0.02, 0.01, &result) != 0,
                "cross-path query is handled without fabricating a bound");
    ASSERT_EQUAL(0u, result.supported, "unsupported path extrapolation is rejected");
    ASSERT_TRUE(prom_num_envelope_evaluate(&envelope,
                                           PROM_NUM_PATH_GPU_CONVENTIONAL_FP16,
                                           PROM_NUM_STAGE_COMPLETE_BLOCK,
                                           1024u, 1024u, 0.02, 0.01, &result) != 0,
                "out-of-corpus shape query is handled");
    ASSERT_EQUAL(0u, result.supported, "unsupported shape extrapolation is rejected");
}

FACT(PrometheusM49CanaryCannotHideStructuredErrorBySignedCancellation)
{
    const std::array<float, 8u> structured{{8.0f, -8.0f, 8.0f, -8.0f,
                                           8.0f, -8.0f, 8.0f, -8.0f}};
    prom_num_canary_summary canary{};
    ASSERT_TRUE(prom_num_canary_measure(structured.data(), 2u, 4u, 49u, &canary) != 0,
                "bounded numerical canary measures a structured residual");
    ASSERT_TRUE(canary.l1_norm == 64.0 && canary.maximum_absolute_value == 8.0,
                "norm channels remain sensitive when a signed sum could cancel");
    ASSERT_TRUE(canary.absolute_projection > 0.0 && canary.bit_hash != 0u,
                "absolute sketch and bit identity complement signed projections");
}

FACT(PrometheusM49CompensationBiasIsRejectedWhenHeldOutErrorWorsens)
{
    const std::array<float, 4u> identificationReference{{0, 1, 2, 3}};
    const std::array<float, 4u> identificationObserved{{0.5f, 1.5f, 2.5f, 3.5f}};
    prom_num_bias_model model{};
    ASSERT_TRUE(prom_num_bias_fit(identificationReference.data(), identificationObserved.data(),
                                  identificationReference.size(), &model) != 0,
                "identification-only scalar bias model fits");
    ASSERT_TRUE(std::abs(model.bias - 0.5) < 1.0e-12, "fit bias is explicit");

    const std::array<float, 4u> heldOutReference{{4, 5, 6, 7}};
    const std::array<float, 4u> heldOutObserved{{3.5f, 4.5f, 5.5f, 6.5f}};
    double uncorrected = 0.0;
    double corrected = 0.0;
    ASSERT_TRUE(prom_num_bias_evaluate(&model, heldOutReference.data(), heldOutObserved.data(),
                                      heldOutReference.size(), &uncorrected, &corrected) != 0,
                "held-out compensation evaluation succeeds");
    ASSERT_TRUE(corrected > uncorrected,
                "a training-sign bias that worsens held-out data is mechanically rejectable");
}

FACT(PrometheusM49ArtifactSchemaRetainsRawEvidenceAndUnsupportedClaims)
{
    const std::string path = std::string(MARIONETTE_TEST_REPO_ROOT) +
        "/internal/prometheus/DevelopmentReport/artifacts/M49/"
        "numerical_heterogeneity_rtx3070.json";
    std::ifstream input(path, std::ios::binary);
    ASSERT_TRUE(input.good(), "the committed M49 research artifact is readable");
    const std::string artifact((std::istreambuf_iterator<char>(input)),
                               std::istreambuf_iterator<char>());
    for (const std::string& required : {
             "prometheus.m49.numerical-heterogeneity.v1",
             "experiment_plan_identity",
             "corpus_definitions",
             "path_definitions",
             "repeated_run_determinism",
             "matched_input_disturbance_records",
             "inherited_gain_records",
             "depth_trajectories",
             "raw_observations",
             "observer_states",
             "shadow_controller_recommendations",
             "canary_results",
             "proposed_envelopes",
             "unsupported_claims",
             "exact_source_hashes",
         }) {
        ASSERT_TRUE(artifact.find(required) != std::string::npos,
                    "artifact schema retains every required M49 evidence family");
    }
    ASSERT_TRUE(artifact.find("\"normal_product_execution_changed\": false") !=
                    std::string::npos,
                "research artifact records the unchanged product path");
    ASSERT_TRUE(artifact.find("\"milestone_state\": \"in_progress\"") !=
                    std::string::npos,
                "artifact does not overclaim completion before the hardware matrix closes");
}
