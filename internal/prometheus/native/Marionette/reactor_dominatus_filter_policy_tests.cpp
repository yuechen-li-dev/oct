#include "../reactor_dominatus_filter_policy.h"
#include "test_harness.h"

static prom_dominatus_measurement_facts make_facts(double spike, double jitter, double confidence) {
    prom_dominatus_measurement_facts f{};
    f.spike_rate_estimate = spike;
    f.jitter_estimate = jitter;
    f.confidence = confidence;
    f.sample_count = 16u;
    return f;
}

FACT(PrometheusDominatusFilterPolicy_StableSelectsSmooth)
{
    prom_dominatus_filter_policy_state s{};
    prom_dominatus_filter_policy_init(&s, nullptr);
    const auto f = make_facts(0.05, 0.10, 0.9);
    const auto d = prom_dominatus_filter_policy_update(&s, 10.0, &f, 1u);
    ASSERT_EQUAL(PROM_DOM_FILTER_KIND_HYSTERESIS, d.selected_kind, "stable stream should choose smooth hysteresis default");
}

FACT(PrometheusDominatusFilterPolicy_SpikeHeavySelectsRobust)
{
    prom_dominatus_filter_policy_state s{};
    prom_dominatus_filter_policy_init(&s, nullptr);
    auto f = make_facts(0.05, 0.1, 0.9);
    (void)prom_dominatus_filter_policy_update(&s, 10.0, &f, 1u);
    s.min_commit_remaining = 0u;
    f = make_facts(0.9, 0.7, 0.9);
    const auto d = prom_dominatus_filter_policy_update(&s, 12.0, &f, 2u);
    ASSERT_TRUE(d.selected_kind == PROM_DOM_FILTER_KIND_MEDIAN || d.selected_kind == PROM_DOM_FILTER_KIND_HYBRID_MEDIAN_EMA,
                "spike heavy stream should choose robust filter");
}

FACT(PrometheusDominatusFilterPolicy_MinCommitPreventsThrash)
{
    prom_dominatus_filter_policy_state s{};
    prom_dominatus_filter_policy_init(&s, nullptr);
    auto stable = make_facts(0.05, 0.1, 0.9);
    (void)prom_dominatus_filter_policy_update(&s, 10.0, &stable, 1u);
    s.min_commit_remaining = 0u;
    auto spike = make_facts(0.95, 0.8, 0.9);
    const auto sw = prom_dominatus_filter_policy_update(&s, 20.0, &spike, 2u);
    ASSERT_EQUAL(1u, sw.switched, "must switch to robust under heavy spike");
    const auto held = prom_dominatus_filter_policy_update(&s, 10.0, &stable, 3u);
    ASSERT_EQUAL(1u, held.held_by_min_commit, "min-commit should block immediate switch back");
}

FACT(PrometheusDominatusFilterPolicy_SwitchMarginPreventsWeakSwitch)
{
    prom_dominatus_filter_policy_state s{};
    prom_dominatus_filter_policy_init(&s, nullptr);
    auto stable = make_facts(0.05, 0.1, 0.9);
    (void)prom_dominatus_filter_policy_update(&s, 10.0, &stable, 1u);
    s.params.switch_margin = 1.0;
    s.min_commit_remaining = 0u;
    auto modest = make_facts(0.4, 0.2, 0.9);
    const auto d = prom_dominatus_filter_policy_update(&s, 10.1, &modest, 2u);
    ASSERT_EQUAL(1u, d.held_by_margin, "large margin should block weak switch");
}

FACT(PrometheusDominatusFilterPolicy_LowConfidenceBlocksSwitch)
{
    prom_dominatus_filter_policy_state s{};
    prom_dominatus_filter_policy_init(&s, nullptr);
    auto stable = make_facts(0.05, 0.1, 0.9);
    (void)prom_dominatus_filter_policy_update(&s, 10.0, &stable, 1u);
    s.min_commit_remaining = 0u;
    auto spike_low_conf = make_facts(0.95, 0.8, 0.2);
    const auto d = prom_dominatus_filter_policy_update(&s, 20.0, &spike_low_conf, 2u);
    ASSERT_EQUAL(1u, d.held_by_confidence, "low confidence should block switch");
}

FACT(PrometheusDominatusFilterPolicy_WarmTransferAvoidsJump)
{
    prom_dominatus_filter_policy_state s{};
    prom_dominatus_filter_policy_init(&s, nullptr);
    auto stable = make_facts(0.05, 0.1, 0.9);
    for (std::uint64_t i = 0; i < 4; ++i) (void)prom_dominatus_filter_policy_update(&s, 10.0, &stable, i);
    const double before = s.last_output;
    s.min_commit_remaining = 0u;
    auto spike = make_facts(0.95, 0.8, 0.9);
    const auto d = prom_dominatus_filter_policy_update(&s, 10.0, &spike, 10u);
    ASSERT_EQUAL(1u, d.warm_transferred, "switch should warm transfer state");
    ASSERT_TRUE(d.filter_output.estimate > before - 0.01 && d.filter_output.estimate < before + 0.01,
                "warm transfer should keep first estimate near previous output");
}

FACT(PrometheusDominatusFilterPolicy_StepAndDriftPreferTracking)
{
    prom_dominatus_filter_policy_state s{};
    prom_dominatus_filter_policy_init(&s, nullptr);
    auto f = make_facts(0.1, 0.2, 0.9);
    f.step_change_suspected = 1u;
    const auto step = prom_dominatus_filter_policy_update(&s, 12.0, &f, 1u);
    ASSERT_EQUAL(PROM_DOM_FILTER_KIND_EMA, step.selected_kind, "step changes should choose fast tracking EMA");

    prom_dominatus_filter_policy_reset(&s);
    f = make_facts(0.1, 0.2, 0.9);
    f.drift_suspected = 1u;
    const auto drift = prom_dominatus_filter_policy_update(&s, 12.0, &f, 2u);
    ASSERT_EQUAL(PROM_DOM_FILTER_KIND_EMA, drift.selected_kind, "drift should choose EMA tracking");
}

FACT(PrometheusDominatusFilterPolicy_ResetBehavior)
{
    prom_dominatus_filter_policy_state s{};
    prom_dominatus_filter_policy_init(&s, nullptr);
    auto f = make_facts(0.05, 0.1, 0.9);
    (void)prom_dominatus_filter_policy_update(&s, 10.0, &f, 1u);
    prom_dominatus_filter_policy_reset(&s);
    ASSERT_EQUAL(0u, s.initialized, "reset must clear initialization");
    ASSERT_EQUAL(0u, s.switch_count, "reset must clear switch count");
    ASSERT_EQUAL(0u, s.min_commit_remaining, "reset must clear commit window");
}
