#include "../reactor_dominatus_measurement_filter.h"
#include "test_harness.h"

FACT(PrometheusDominatusMeasurementFilter_StableStreamProducesValidFilteredEvidence)
{
    prom_dominatus_measurement_filter_state s{};
    prom_dominatus_measurement_filter_init(&s, nullptr);

    prom_dominatus_filtered_evidence last{};
    for (std::uint64_t i = 0; i < 12; ++i) {
        const double sample = 10.0 + ((i % 2 == 0) ? 0.05 : -0.05);
        last = prom_dominatus_measurement_filter_update(&s, sample, i + 1);
    }

    ASSERT_EQUAL(1u, last.valid, "stable stream must emit valid evidence");
    ASSERT_TRUE(last.filtered_value > 9.5 && last.filtered_value < 10.5, "filtered value should remain near true stable level");
    ASSERT_TRUE(last.confidence >= 0.45, "confidence should remain at policy-usable level");
}

FACT(PrometheusDominatusMeasurementFilter_SpikeStreamDetectsOutliers)
{
    prom_dominatus_measurement_filter_state s{};
    prom_dominatus_measurement_filter_init(&s, nullptr);

    const double seq[] = {10.0, 10.0, 10.0, 100.0, 10.0, 10.0};
    prom_dominatus_filtered_evidence spike{};
    prom_dominatus_filtered_evidence post{};
    for (std::uint64_t i = 0; i < 6; ++i) {
        const auto e = prom_dominatus_measurement_filter_update(&s, seq[i], i + 1);
        if (i == 3) spike = e;
        if (i == 4) post = e;
    }

    ASSERT_EQUAL(100.0, spike.raw_value, "raw truth must preserve spike sample");
    ASSERT_TRUE(post.filtered_value < 70.0, "filtered path should recover quickly after one-tick spike");
    ASSERT_TRUE(spike.outlier_count > 0u || spike.spike_rate_estimate > 0.0, "spike stream should show outlier or spike estimate");
}

FACT(PrometheusDominatusMeasurementFilter_StepStreamTriggersStepSuspicion)
{
    prom_dominatus_measurement_filter_state s{};
    prom_dominatus_measurement_filter_init(&s, nullptr);

    for (std::uint64_t i = 0; i < 8; ++i) (void)prom_dominatus_measurement_filter_update(&s, 10.0, i + 1);

    prom_dominatus_filtered_evidence step_e{};
    for (std::uint64_t i = 0; i < 8; ++i) {
        step_e = prom_dominatus_measurement_filter_update(&s, 16.0, 100 + i);
        if (step_e.step_change_suspected != 0u) break;
    }

    ASSERT_EQUAL(1u, step_e.step_change_suspected, "sustained level shift should eventually suspect step change");
    ASSERT_TRUE(step_e.filtered_value > 12.0, "filtered value should track toward the new level");
}

FACT(PrometheusDominatusMeasurementFilter_ConfidenceGatingVisible)
{
    prom_dominatus_measurement_filter_state s{};
    prom_dominatus_measurement_filter_init(&s, nullptr);

    (void)prom_dominatus_measurement_filter_update(&s, 10.0, 1u);
    s.policy.min_commit_remaining = 0u;
    s.policy.params.confidence_threshold = 0.95;

    const auto e = prom_dominatus_measurement_filter_update(&s, 20.0, 2u);
    ASSERT_TRUE(e.confidence < s.policy.params.confidence_threshold, "constructed stream should keep confidence below strict threshold");
    ASSERT_EQUAL(1u, e.held_by_confidence, "policy should surface confidence hold diagnostics");
}

FACT(PrometheusDominatusMeasurementFilter_TruthSeparationRawAndFiltered)
{
    prom_dominatus_measurement_filter_state s{};
    prom_dominatus_measurement_filter_init(&s, nullptr);

    for (std::uint64_t i = 0; i < 6; ++i) (void)prom_dominatus_measurement_filter_update(&s, 10.0, i + 1);
    const auto spike = prom_dominatus_measurement_filter_update(&s, 100.0, 7u);
    const auto recover = prom_dominatus_measurement_filter_update(&s, 10.0, 8u);

    ASSERT_EQUAL(100.0, spike.raw_value, "raw value must be preserved in evidence");
    ASSERT_TRUE(recover.filtered_value >= 0.0 && recover.raw_value == 10.0,
                "truth-separated output must retain both raw and filtered channels across recovery");
}

FACT(PrometheusDominatusMeasurementFilter_ResetBehavior)
{
    prom_dominatus_measurement_filter_state s{};
    prom_dominatus_measurement_filter_init(&s, nullptr);

    (void)prom_dominatus_measurement_filter_update(&s, 10.0, 1u);
    (void)prom_dominatus_measurement_filter_update(&s, 11.0, 2u);
    prom_dominatus_measurement_filter_reset(&s);

    ASSERT_EQUAL(0u, s.recent_count, "reset must clear rolling sample count");
    ASSERT_EQUAL(0u, s.initialized, "reset must clear initialized flag");
    ASSERT_EQUAL(0u, s.policy.initialized, "reset must clear policy initialization state");

    const auto first = prom_dominatus_measurement_filter_update(&s, 13.0, 9u);
    ASSERT_EQUAL(1u, first.valid, "first post-reset sample should initialize evidence again");
    ASSERT_EQUAL(1u, first.sample_count, "first post-reset sample count should restart at one");
}
