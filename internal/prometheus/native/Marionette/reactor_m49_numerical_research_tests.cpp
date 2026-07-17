#include "../reactor_numerical_research.h"
#include "../reactor_vulkan_transformer_control.h"
#include "test_harness.h"

#include <array>
#include <cmath>
#include <cstdio>
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

FACT(PrometheusM49aSuffixIdentityRequiresExactMatchedInput)
{
    prom_num_suffix_identity_request request{};
    request.stage = PROM_NUM_STAGE_FFN_SUFFIX;
    request.path = PROM_NUM_PATH_GPU_COOPERATIVE_FP16;
    request.tokens = 8u;
    request.input_channels = 32u;
    request.output_channels = 32u;
    request.precision_contract = 16u;
    request.input_generation = 49001u;
    request.weight_generation = 49002u;
    request.input_hash = 0x12345678u;
    request.reference_input_hash = request.input_hash;
    request.source_hash = 0xabcdefu;
    prom_num_suffix_identity first{};
    prom_num_suffix_identity repeated{};
    ASSERT_TRUE(prom_num_suffix_identity_build(&request, &first) != 0,
                "matched-input suffix identity builds");
    ASSERT_TRUE(prom_num_suffix_identity_build(&request, &repeated) != 0,
                "matched-input suffix identity repeats");
    ASSERT_EQUAL(first.replay_identity, repeated.replay_identity,
                 "suffix replay identity is deterministic");
    ASSERT_EQUAL(1u, first.matched_input, "exact input identity is explicit");
    request.reference_input_hash += 1u;
    prom_num_suffix_identity rejected{};
    ASSERT_TRUE(prom_num_suffix_identity_build(&request, &rejected) == 0,
                "inherited input discrepancy cannot masquerade as local D");
}

FACT(PrometheusM49aPerturbationFamiliesAndNormsAreDeterministic)
{
    constexpr std::uint32_t tokens = 4u;
    constexpr std::uint32_t channels = 8u;
    constexpr std::size_t count = tokens * channels;
    std::array<float, count> base{};
    std::array<float, count> residual{};
    std::array<float, count> natural{};
    std::array<float, count> first{};
    std::array<float, count> repeated{};
    for (std::size_t i = 0u; i < count; ++i) {
        base[i] = 1.0f + static_cast<float>(i) / 64.0f;
        residual[i] = (i & 1u) == 0u ? 0.01f : -0.02f;
        natural[i] = static_cast<float>(static_cast<int>(i % 7u) - 3) / 1000.0f;
    }
    for (std::uint32_t family = PROM_NUM_PERTURB_ONE_COORDINATE;
         family <= PROM_NUM_PERTURB_NATURAL_LAYER_DISCREPANCY; ++family) {
        prom_num_perturbation_summary a{};
        prom_num_perturbation_summary b{};
        ASSERT_TRUE(prom_num_generate_perturbation(
                        family, 490100u + family, 0.125, base.data(), residual.data(),
                        natural.data(), first.data(), tokens, channels, &a) != 0,
                    "each required deterministic perturbation family generates");
        ASSERT_TRUE(prom_num_generate_perturbation(
                        family, 490100u + family, 0.125, base.data(), residual.data(),
                        natural.data(), repeated.data(), tokens, channels, &b) != 0,
                    "each required perturbation family repeats");
        ASSERT_EQUAL(a.identity, b.identity, "perturbation identity repeats exactly");
        ASSERT_EQUAL(prom_num_hash_float_bits(first.data(), count),
                     prom_num_hash_float_bits(repeated.data(), count),
                     "perturbation values repeat bitwise");
        ASSERT_TRUE(std::abs(a.l2_norm - 0.125) < 1.0e-6,
                    "requested perturbation magnitude is its L2 norm");
        ASSERT_TRUE(a.nonzero_count > 0u && a.l1_norm >= a.l2_norm &&
                        a.l2_norm >= a.linfinity_norm,
                    "all perturbation norms and support are recorded");
    }
}

FACT(PrometheusM49aGainReportsL1L2AndLInfinitySeparately)
{
    const std::array<float, 4u> inputA{{0, 0, 0, 0}};
    const std::array<float, 4u> inputB{{1, -2, 0, 0}};
    const std::array<float, 4u> outputA{{0, 0, 0, 0}};
    const std::array<float, 4u> outputB{{3, -2, 4, 0}};
    prom_num_gain_summary gain{};
    ASSERT_TRUE(prom_num_summarize_gain(inputA.data(), inputB.data(),
                                        outputA.data(), outputB.data(),
                                        2u, 2u, &gain) != 0,
                "multinorm gain summary succeeds");
    ASSERT_TRUE(std::abs(gain.l1_gain - 3.0) < 1.0e-12,
                "L1 gain is retained independently");
    ASSERT_TRUE(std::abs(gain.l2_gain - std::sqrt(29.0 / 5.0)) < 1.0e-12,
                "L2 gain is retained independently");
    ASSERT_TRUE(std::abs(gain.linfinity_gain - 2.0) < 1.0e-12,
                "L-infinity gain is retained independently");
    ASSERT_TRUE(gain.l1_gain != gain.l2_gain && gain.l2_gain != gain.linfinity_gain,
                "multimodal gain is not collapsed to one scalar");
}

FACT(PrometheusM49aFp64SelectedDotAndRmsWitnessesSeparateAccumulation)
{
    const std::array<float, 6u> left{{1.0e8f, 1.0f, -1.0e8f, 0.3333f, 0.3333f, 0.3333f}};
    const std::array<float, 6u> fp16SafeLeft{{1.0e4f, 1.0f, -1.0e4f, 0.3333f, 0.3333f, 0.3333f}};
    const std::array<float, 6u> right{{1, 1, 1, 1, 1, 1}};
    prom_num_fp64_dot_witness fp32Operands{};
    prom_num_fp64_dot_witness fp16Operands{};
    ASSERT_TRUE(prom_num_fp64_dot_oracle(left.data(), right.data(), left.size(), 0u,
                                         &fp32Operands) != 0,
                "selected FP64 dot witness evaluates FP32 operands");
    ASSERT_TRUE(prom_num_fp64_dot_oracle(fp16SafeLeft.data(), right.data(), fp16SafeLeft.size(), 1u,
                                         &fp16Operands) != 0,
                "selected FP64 dot witness evaluates canonical FP16 operands");
    ASSERT_TRUE(fp32Operands.absolute_accumulation_difference > 0.0,
                "FP32 scalar accumulation error remains measurable");
    ASSERT_TRUE(fp16Operands.fp64_accumulation != 0.9999,
                "operand quantization is separate from accumulation order");

    prom_num_fp64_rms_witness rms{};
    ASSERT_TRUE(prom_num_fp64_rms_oracle(left.data(), left.size(), 1.0e-5, &rms) != 0,
                "selected RMSNorm FP64 witness evaluates");
    ASSERT_TRUE(rms.fp64_sum_of_squares > 0.0 && rms.fp32_inv_rms > 0.0 &&
                    rms.fp64_inv_rms > 0.0,
                "sum-of-squares and InvRms authorities are explicit");
    std::fprintf(stderr,
                 "M49a FP64 dot_fp32acc=%g dot_fp64acc=%g abs_accum_diff=%g fp16_dot_fp32acc=%g fp16_dot_fp64acc=%g rms_sumsq_fp32=%g rms_sumsq_fp64=%g invrms_fp32=%g invrms_fp64=%g\n",
                 fp32Operands.fp32_accumulation, fp32Operands.fp64_accumulation,
                 fp32Operands.absolute_accumulation_difference,
                 fp16Operands.fp32_accumulation, fp16Operands.fp64_accumulation,
                 rms.fp32_sum_of_squares, rms.fp64_sum_of_squares,
                 rms.fp32_inv_rms, rms.fp64_inv_rms);
}

FACT(PrometheusM49aEnvelopeFitNeverLearnsFromHeldOutRecords)
{
    std::array<prom_num_envelope_sample, 5u> samples{{
        {PROM_NUM_SPLIT_IDENTIFICATION, 0.0, 0.10, 0.10, 0.0},
        {PROM_NUM_SPLIT_IDENTIFICATION, 0.10, 0.25, 0.10, 0.0},
        {PROM_NUM_SPLIT_IDENTIFICATION, 0.20, 0.40, 0.10, 0.0},
        {PROM_NUM_SPLIT_HELD_OUT, 0.15, 0.325, 99.0, 99.0},
        {PROM_NUM_SPLIT_HELD_OUT, 0.30, 0.80, 99.0, 99.0},
    }};
    prom_num_envelope envelope{};
    prom_num_envelope_fit_summary summary{};
    ASSERT_TRUE(prom_num_envelope_fit(samples.data(), samples.size(),
                                      PROM_NUM_PATH_GPU_COOPERATIVE_FP16,
                                      PROM_NUM_STAGE_FFN_SUFFIX, 1u, 256u,
                                      8u, 4096u, &envelope, &summary) != 0,
                "identification-only empirical envelope fits");
    ASSERT_TRUE(std::abs(envelope.local_disturbance_bound - 0.10) < 1.0e-12 &&
                    std::abs(envelope.gain_bound - 1.5) < 1.0e-12,
                "held-out disturbance values never tune the envelope");
    ASSERT_EQUAL(static_cast<std::uint64_t>(3u), summary.identification_count,
                 "identification support count is explicit");
    ASSERT_EQUAL(static_cast<std::uint64_t>(2u), summary.held_out_count,
                 "held-out support count is explicit");
    ASSERT_EQUAL(static_cast<std::uint64_t>(1u), summary.held_out_failure_count,
                 "held-out failure is reported rather than tuning the fit");
}

FACT(PrometheusM49aMitigationEligibilityRequiresHeldOutBenefit)
{
    prom_num_mitigation_evidence evidence{};
    evidence.identification_baseline_error = 1.0;
    evidence.identification_mitigated_error = 0.5;
    evidence.held_out_baseline_error = 1.2;
    evidence.held_out_mitigated_error = 0.9;
    evidence.latency_microseconds = 20.0;
    evidence.identification_count = 12u;
    evidence.held_out_count = 4u;
    std::uint32_t eligible = 0u;
    ASSERT_TRUE(prom_num_mitigation_eligible(&evidence, 0.0, &eligible) != 0,
                "bounded mitigation evidence evaluates");
    ASSERT_EQUAL(1u, eligible, "identification and held-out improvement is eligible");
    evidence.held_out_mitigated_error = 1.3;
    ASSERT_TRUE(prom_num_mitigation_eligible(&evidence, 0.0, &eligible) != 0,
                "held-out regression evaluates");
    ASSERT_EQUAL(0u, eligible, "primary-only improvement is rejected");
}

FACT(PrometheusM49aCanaryCalibrationReportsCorrelationAndFalseRates)
{
    const std::array<double, 6u> canary{{0.1, 0.2, 0.8, 0.9, 0.7, 0.3}};
    const std::array<double, 6u> audit{{0.1, 0.6, 0.9, 1.0, 0.2, 0.3}};
    prom_num_canary_calibration calibration{};
    ASSERT_TRUE(prom_num_canary_calibrate(canary.data(), audit.data(), canary.size(),
                                          0.5, 0.5, &calibration) != 0,
                "canary/full-audit calibration evaluates");
    ASSERT_EQUAL(static_cast<std::uint64_t>(2u), calibration.true_positive,
                 "true positives are explicit");
    ASSERT_EQUAL(static_cast<std::uint64_t>(1u), calibration.false_positive,
                 "false positives are explicit");
    ASSERT_EQUAL(static_cast<std::uint64_t>(1u), calibration.false_negative,
                 "false negatives are explicit");
    ASSERT_TRUE(calibration.pearson_correlation > 0.0 &&
                    calibration.false_positive_rate > 0.0 &&
                    calibration.false_negative_rate > 0.0,
                "correlation and both false rates remain visible");
}

FACT(PrometheusM49aArtifactSchemaSeparatesCompletedAndUnsupportedEvidence)
{
    const std::string path = std::string(MARIONETTE_TEST_REPO_ROOT) +
        "/internal/prometheus/DevelopmentReport/artifacts/M49a/"
        "controlled_stage_gain_and_mitigation_rtx3070.json";
    std::ifstream input(path, std::ios::binary);
    ASSERT_TRUE(input.good(), "the M49a machine-readable artifact is readable");
    const std::string artifact((std::istreambuf_iterator<char>(input)),
                               std::istreambuf_iterator<char>());
    for (const std::string& required : {
             "prometheus.m49a.controlled-stage-gain-and-mitigation.v2",
             "matched_input_disturbance_records",
             "controlled_perturbations",
             "gain_records",
             "fp64_witness_records",
             "identification_held_out_split",
             "mitigation_ab",
             "envelopes",
             "canary_calibration",
             "numerical_control_parameters",
             "shadow_hsfm",
             "rollout_stages",
             "m49b_one_shot",
             "oct_import_provenance",
             "unsupported_claims",
             "exact_source_hashes",
         }) {
        ASSERT_TRUE(artifact.find(required) != std::string::npos,
                    "M49a artifact retains each required evidence family");
    }
    ASSERT_TRUE(artifact.find("\"normal_product_execution_changed\": false") !=
                    std::string::npos,
                "M49a artifact records unchanged normal product execution");
    ASSERT_TRUE(artifact.find("\"milestone_state\": \"complete\"") !=
                    std::string::npos,
                "M49a artifact records the bounded architecture decision");
    ASSERT_TRUE(artifact.find("\"held_out_records_completed\": 5") !=
                    std::string::npos,
                "M49a artifact exposes the bounded held-out hardware matrix");
    ASSERT_TRUE(artifact.find("\"primary_mitigation\": \"a2x4_fp32_complete_block_checkpoint_interval_2\"") !=
                    std::string::npos &&
                    artifact.find("\"product_authority\": false") != std::string::npos,
                "the selected mitigation remains experimental and outside product authority");
}

FACT(PrometheusM49bControllerParametersEvidenceAndBoundedAuthority)
{
    prom_num_m49b_controller controller{};
    prom_num_m49b_init(&controller);
    ASSERT_EQUAL(0u, controller.parameters.rollout_stage,
                 "M49b defaults to observer-only authority");
    ASSERT_EQUAL(4u, controller.parameters.canary_interval,
                 "M49b owns the authored canary cadence in one parameter record");
    ASSERT_TRUE(prom_num_m49b_validate_parameters(&controller.parameters) != 0,
                "M49b default parameters validate");
    prom_num_m49b_parameters invalid = controller.parameters;
    invalid.audit_sample_count = 3u;
    ASSERT_TRUE(prom_num_m49b_validate_parameters(&invalid) == 0,
                "M49b rejects out-of-range parameters conservatively");

    std::array<std::uint32_t, 16u> coordinates{};
    ASSERT_TRUE(prom_num_m49b_derive_coordinates(0x49b0u, 0x123u, 128u, 16u,
                                                  coordinates.data()) != 0,
                "M49b derives bounded coordinates from shape and seed");
    for (std::size_t i = 0u; i < coordinates.size(); ++i) {
        ASSERT_TRUE(coordinates[i] < 128u, "every canary coordinate is in bounds");
        for (std::size_t j = 0u; j < i; ++j)
            ASSERT_TRUE(coordinates[i] != coordinates[j],
                        "the normal-size canary has no duplicate coordinates");
    }

    const std::array<float, 16u> witnessSamples{{0.10f, -0.10f, 0.20f, -0.20f,
                                                   0.30f, -0.30f, 0.40f, -0.40f,
                                                   0.50f, -0.50f, 0.60f, -0.60f,
                                                   0.70f, -0.70f, 0.80f, -0.80f}};
    std::array<float, 16u> selectedSamples = witnessSamples;
    selectedSamples[3u] += 0.01f;
    selectedSamples[11u] -= 0.02f;
    prom_m49b_paired_estimate paired{};
    prom_m49b_paired_estimate repeatedPaired{};
    ASSERT_TRUE(prom_m49b_estimate_paired_discrepancy(
                    selectedSamples.data(), witnessSamples.data(),
                    static_cast<std::uint32_t>(selectedSamples.size()),
                    0x88u, 0x99u, 0xa2a4u, 7u, &paired) != 0,
                "M49b derives bounded paired discrepancy evidence");
    ASSERT_TRUE(prom_m49b_estimate_paired_discrepancy(
                    selectedSamples.data(), witnessSamples.data(),
                    static_cast<std::uint32_t>(selectedSamples.size()),
                    0x88u, 0x99u, 0xa2a4u, 7u, &repeatedPaired) != 0,
                "paired evidence remains replay-identifiable on repeat");
    ASSERT_TRUE(paired.sampled_l2_error > 0.02 && paired.sampled_linf_error >= 0.019,
                "paired selected/witness samples expose discrepancy regimes");
    ASSERT_TRUE(paired.confidence >= 0.75 && paired.reference_suspect == 0u,
                "a complete finite same-request witness clears the confidence floor");
    ASSERT_EQUAL(paired.paired_identity, repeatedPaired.paired_identity,
                 "paired evidence identity is deterministic");

    const std::array<float, 16u> samples{{0.1f, -0.1f, 0.2f, -0.2f,
                                           0.3f, -0.3f, 0.4f, -0.4f,
                                           0.5f, -0.5f, 0.6f, -0.6f,
                                           0.7f, -0.7f, 0.8f, -0.8f}};
    prom_num_m49b_observation observation{};
    observation.completion_known = 1u;
    observation.current_path = PROM_NUM_PATH_GPU_COOPERATIVE_FP16;
    observation.execution_index = 1u;
    observation.shape_identity = 0x88u;
    observation.output_replay_identity = 0x99u;
    observation.reference_identity = 0x77u;
    observation.sampled_values = samples.data();
    observation.sampled_value_count = static_cast<std::uint32_t>(samples.size());
    observation.observed_l2_error = 0.01;
    observation.observed_linf_error = 0.01;
    observation.observed_gain = 1.0;
    observation.confidence = 0.9;
    prom_num_m49b_evidence evidence{};
    prom_num_m49b_decision decision{};
    ASSERT_TRUE(prom_num_m49b_observe(&controller, &observation, &evidence, &decision) != 0,
                "M49b observer records finite evidence");
    ASSERT_TRUE(sizeof(evidence) <= 256u, "M49b evidence stays inside the readback budget");
    ASSERT_EQUAL(1u, decision.shadow_only, "Stage 0 emits no applied authority");
    ASSERT_EQUAL(observation.output_replay_identity, decision.execution_replay_identity,
                 "shadow evidence does not change execution replay identity");

    prom_num_m49b_parameters stage2 = controller.parameters;
    stage2.rollout_stage = 2u;
    ASSERT_TRUE(prom_num_m49b_update_parameters(&controller, &stage2) != 0,
                "M49b accepts a live request-boundary parameter update");
    const std::uint64_t parameterGeneration = controller.parameter_generation;
    observation.observed_l2_error = 0.2;
    observation.observed_linf_error = 0.2;
    observation.observed_gain = 2.0;
    for (std::uint64_t execution = 2u; execution < 6u; ++execution) {
        observation.execution_index = execution;
        ASSERT_TRUE(prom_num_m49b_observe(&controller, &observation, &evidence, &decision) != 0,
                    "M49b high-gain hysteresis observes deterministically");
    }
    ASSERT_EQUAL(parameterGeneration, evidence.parameter_generation,
                 "one completed execution reports its pinned parameter generation");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_NUM_M49B_APPLY_CHECKPOINT), decision.action,
                 "Stage 2 eventually authorizes the fixed checkpoint action");
    ASSERT_EQUAL(PROM_NUM_PATH_GPU_COOPERATIVE_FP16, decision.block_path[0],
                 "checkpoint pattern preserves cooperative block zero");
    ASSERT_EQUAL(PROM_NUM_PATH_GPU_A2X4_FP32, decision.block_path[1],
                 "checkpoint pattern promotes every second block only");
    ASSERT_TRUE(decision.execution_replay_identity != observation.output_replay_identity,
                "applied Stage 2 authority changes execution replay identity");

    controller.state = PROM_NUM_M49B_FALLBACK_RECOMMENDED;
    controller.cooldown_remaining = 2u;
    prom_num_m49b_advance_execution(&controller, 6u);
    ASSERT_EQUAL(1u, controller.cooldown_remaining,
                 "a known completion decrements fallback cooldown without fabricating evidence");
    prom_num_m49b_advance_execution(&controller, 7u);
    ASSERT_EQUAL(0u, controller.cooldown_remaining,
                 "fallback cooldown ends deterministically at an execution boundary");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_NUM_M49B_CHECKPOINT_RECOMMENDED),
                 controller.state,
                 "fallback recovery returns through the bounded checkpoint state");
}

FACT(PrometheusM49bFixedStackAdapterKeepsAuditAndAuthoritySeparate)
{
    prom_num_m49b_controller controller{};
    prom_num_m49b_init(&controller);
    prom_num_m49b_parameters stage2 = controller.parameters;
    stage2.rollout_stage = 2u;
    ASSERT_TRUE(prom_num_m49b_update_parameters(&controller, &stage2) != 0,
                "Stage 2 parameters publish into the fixed-stack adapter");
    controller.state = PROM_NUM_M49B_CHECKPOINT_RECOMMENDED;
    prom_m48_stack_request request{};
    request.layer_count = PROM_M48_LAYER_COUNT;
    request.projection_path = PROM_M47_PROJECTION_COOPERATIVE;
    prom_m49b_apply_fixed_stack_policy(&controller, &request, 0x49b2u);
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M48_NUMERICAL_CONTROL_M49B),
                 request.numerical_control_mode,
                 "checkpoint authority uses the first-class controller path field");
    ASSERT_EQUAL(0u, request.audit_layer_projection_path[1u],
                 "controller authority does not borrow audit override storage");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M47_PROJECTION_A2X4_FP32),
                 request.controller_layer_projection_path[1u],
                 "interval-two checkpoint promotes block one");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M47_PROJECTION_A2X4_FP32),
                 request.controller_layer_projection_path[3u],
                 "interval-two checkpoint promotes block three");

    prom_num_m49b_parameters stage3 = stage2;
    stage3.rollout_stage = 3u;
    ASSERT_TRUE(prom_num_m49b_update_parameters(&controller, &stage3) != 0,
                "Stage 3 parameters publish at a request boundary");
    controller.state = PROM_NUM_M49B_FALLBACK_RECOMMENDED;
    controller.cooldown_remaining = 8u;
    request = {};
    request.layer_count = PROM_M48_LAYER_COUNT;
    request.projection_path = PROM_M47_PROJECTION_COOPERATIVE;
    prom_m49b_apply_fixed_stack_policy(&controller, &request, 0x49b3u);
    for (const std::uint32_t path : request.controller_layer_projection_path) {
        ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_M47_PROJECTION_A2X4_FP32), path,
                     "active fallback gives every complete block the A2x4 path");
    }
}
