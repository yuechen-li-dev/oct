#include "../reactor_dominatus_filter.h"
#include "test_harness.h"

FACT(PrometheusDominatusFilter_EmaConstantInputStable)
{
    prom_dominatus_filter_state state{};
    const prom_dominatus_filter_params params = prom_dominatus_filter_params_ema(0.2);
    prom_dominatus_filter_init(&state, &params);

    for (std::uint64_t tick = 0; tick < 10; ++tick) {
        const prom_dominatus_filter_output out = prom_dominatus_filter_update(&state, 42.0, tick);
        ASSERT_EQUAL(1u, out.valid, "ema output must remain valid");
        ASSERT_TRUE(out.estimate > 41.999 && out.estimate < 42.001, "ema must remain near constant input");
    }
}

FACT(PrometheusDominatusFilter_EmaStepFastBeatsSlow)
{
    prom_dominatus_filter_state slow{};
    prom_dominatus_filter_state fast{};
    const prom_dominatus_filter_params slow_params = prom_dominatus_filter_params_ema(0.1);
    const prom_dominatus_filter_params fast_params = prom_dominatus_filter_params_ema(0.6);
    prom_dominatus_filter_init(&slow, &slow_params);
    prom_dominatus_filter_init(&fast, &fast_params);

    for (std::uint64_t i = 0; i < 3; ++i) {
        (void)prom_dominatus_filter_update(&slow, 0.0, i);
        (void)prom_dominatus_filter_update(&fast, 0.0, i);
    }
    const prom_dominatus_filter_output slow_out = prom_dominatus_filter_update(&slow, 10.0, 4u);
    const prom_dominatus_filter_output fast_out = prom_dominatus_filter_update(&fast, 10.0, 4u);
    ASSERT_TRUE(fast_out.estimate > slow_out.estimate, "higher alpha must move faster on step");
}

FACT(PrometheusDominatusFilter_MedianSuppressesSpike)
{
    prom_dominatus_filter_state state{};
    const prom_dominatus_filter_params params = prom_dominatus_filter_params_median(3u);
    prom_dominatus_filter_init(&state, &params);
    (void)prom_dominatus_filter_update(&state, 10.0, 0u);
    (void)prom_dominatus_filter_update(&state, 10.0, 1u);
    const prom_dominatus_filter_output out = prom_dominatus_filter_update(&state, 100.0, 2u);
    ASSERT_TRUE(out.estimate < 11.0, "median3 should suppress isolated spike");
}

FACT(PrometheusDominatusFilter_HysteresisHoldAndUpdate)
{
    prom_dominatus_filter_state state{};
    const prom_dominatus_filter_params params = prom_dominatus_filter_params_hysteresis(1.0);
    prom_dominatus_filter_init(&state, &params);
    (void)prom_dominatus_filter_update(&state, 10.0, 0u);
    const prom_dominatus_filter_output held = prom_dominatus_filter_update(&state, 10.5, 1u);
    const prom_dominatus_filter_output updated = prom_dominatus_filter_update(&state, 13.0, 2u);
    ASSERT_EQUAL(1u, held.held, "inside deadband should hold");
    ASSERT_EQUAL(1u, updated.updated, "outside deadband should update");
}

FACT(PrometheusDominatusFilter_HybridSmoothsAndSuppresses)
{
    prom_dominatus_filter_state state{};
    const prom_dominatus_filter_params params = prom_dominatus_filter_params_hybrid_median_ema(3u, 0.2);
    prom_dominatus_filter_init(&state, &params);
    (void)prom_dominatus_filter_update(&state, 10.0, 0u);
    (void)prom_dominatus_filter_update(&state, 10.0, 1u);
    const prom_dominatus_filter_output spike = prom_dominatus_filter_update(&state, 100.0, 2u);
    ASSERT_TRUE(spike.estimate < 30.0, "hybrid should suppress spike and smooth output");
}

FACT(PrometheusDominatusFilter_ResetAndWarmStart)
{
    prom_dominatus_filter_state state{};
    const prom_dominatus_filter_params params = prom_dominatus_filter_params_ema(0.2);
    prom_dominatus_filter_init(&state, &params);
    (void)prom_dominatus_filter_update(&state, 10.0, 0u);
    prom_dominatus_filter_reset(&state);
    const prom_dominatus_filter_output after_reset = prom_dominatus_filter_update(&state, 99.0, 1u);
    ASSERT_TRUE(after_reset.estimate > 98.99, "first post-reset sample should initialize estimate");

    prom_dominatus_filter_warm_start(&state, &params, 50.0);
    const prom_dominatus_filter_output warm = prom_dominatus_filter_update(&state, 50.0, 2u);
    ASSERT_TRUE(warm.estimate > 49.99 && warm.estimate < 50.01, "warm-start should keep prior estimate without cold jump");
}

FACT(PrometheusDominatusFilter_InvalidParamsRejected)
{
    prom_dominatus_filter_state state{};
    prom_dominatus_filter_params bad_alpha = prom_dominatus_filter_params_ema(0.0);
    prom_dominatus_filter_init(&state, &bad_alpha);
    ASSERT_EQUAL(PROM_DOM_FILTER_KIND_NONE, state.kind, "invalid alpha should reject initialization");
    const prom_dominatus_filter_output out = prom_dominatus_filter_update(&state, 1.0, 0u);
    ASSERT_EQUAL(0u, out.valid, "rejected filter should emit invalid output");
}

FACT(PrometheusDominatusFilter_MedianWarmupUsesFullWindow)
{
    prom_dominatus_filter_state state{};
    const prom_dominatus_filter_params params = prom_dominatus_filter_params_median(9u);
    prom_dominatus_filter_init(&state, &params);

    prom_dominatus_filter_output out{};
    for (std::uint64_t i = 0; i < 9; ++i) {
        out = prom_dominatus_filter_update(&state, 10.0 + static_cast<double>(i), i);
        if (i < 8u) {
            ASSERT_EQUAL(1u, out.warmup, "median9 should remain in warmup until ninth sample");
        }
    }
    ASSERT_EQUAL(0u, out.warmup, "median9 should exit warmup at ninth sample");
}

FACT(PrometheusDominatusFilter_HybridWarmupUsesFullWindow)
{
    prom_dominatus_filter_state state{};
    const prom_dominatus_filter_params params = prom_dominatus_filter_params_hybrid_median_ema(5u, 0.2);
    prom_dominatus_filter_init(&state, &params);

    prom_dominatus_filter_output out{};
    for (std::uint64_t i = 0; i < 5; ++i) {
        out = prom_dominatus_filter_update(&state, 20.0 + static_cast<double>(i), i);
        if (i < 4u) {
            ASSERT_EQUAL(1u, out.warmup, "hybrid median5 should remain in warmup until fifth sample");
        }
    }
    ASSERT_EQUAL(0u, out.warmup, "hybrid median5 should exit warmup at fifth sample");
}

FACT(PrometheusDominatusFilter_WarmStartMedianSeedsWindow)
{
    prom_dominatus_filter_state state{};
    const prom_dominatus_filter_params params = prom_dominatus_filter_params_median(5u);
    prom_dominatus_filter_warm_start(&state, &params, 10.0);

    const prom_dominatus_filter_output out = prom_dominatus_filter_update(&state, 100.0, 1u);
    ASSERT_TRUE(out.estimate > 9.99 && out.estimate < 10.01, "warm-started median should suppress first spike from seeded window");
    ASSERT_EQUAL(0u, out.warmup, "warm-started median window should be ready immediately");
}

FACT(PrometheusDominatusFilter_WarmStartHybridSeedsMedianAndEma)
{
    prom_dominatus_filter_state state{};
    const prom_dominatus_filter_params params = prom_dominatus_filter_params_hybrid_median_ema(3u, 0.2);
    prom_dominatus_filter_warm_start(&state, &params, 10.0);

    const prom_dominatus_filter_output out = prom_dominatus_filter_update(&state, 100.0, 1u);
    ASSERT_TRUE(out.estimate > 9.99 && out.estimate < 10.01,
                "warm-started hybrid should keep EMA consistent with seeded median window on first spike");
    ASSERT_EQUAL(0u, out.warmup, "warm-started hybrid window should be ready immediately");
}

FACT(PrometheusDominatusFilter_EmaAndHysteresisWarmupUnchanged)
{
    {
        prom_dominatus_filter_state state{};
        const prom_dominatus_filter_params params = prom_dominatus_filter_params_ema(0.2);
        prom_dominatus_filter_init(&state, &params);
        ASSERT_EQUAL(1u, prom_dominatus_filter_update(&state, 1.0, 0u).warmup, "ema should warm up for first sample");
        ASSERT_EQUAL(1u, prom_dominatus_filter_update(&state, 1.0, 1u).warmup, "ema should warm up for second sample");
        ASSERT_EQUAL(0u, prom_dominatus_filter_update(&state, 1.0, 2u).warmup, "ema should exit warmup at third sample");
    }
    {
        prom_dominatus_filter_state state{};
        const prom_dominatus_filter_params params = prom_dominatus_filter_params_hysteresis(0.5);
        prom_dominatus_filter_init(&state, &params);
        ASSERT_EQUAL(1u, prom_dominatus_filter_update(&state, 1.0, 0u).warmup,
                     "hysteresis should warm up for first sample");
        ASSERT_EQUAL(1u, prom_dominatus_filter_update(&state, 1.0, 1u).warmup,
                     "hysteresis should warm up for second sample");
        ASSERT_EQUAL(0u, prom_dominatus_filter_update(&state, 1.0, 2u).warmup,
                     "hysteresis should exit warmup at third sample");
    }
}
