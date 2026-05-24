#include "../reactor_dominatus_predictor.h"
#include "test_harness.h"

static prom_dominatus_shadow_snapshot make_snapshot(uint64_t issued, uint64_t target, uint64_t predicted)
{
    prom_dominatus_shadow_snapshot s{};
    s.valid = 1u;
    s.issued_tick = issued;
    s.target_tick = target;
    s.predicted_ready_tick = predicted;
    s.mismatch_kind = PROM_DOM_SHADOW_MISMATCH_MATCH;
    return s;
}

FACT(PrometheusP15M11ShadowWouldAct_Defaults)
{
    prom_dominatus_shadow_would_act_state state{};
    prom_dominatus_shadow_would_act_init(&state);
    ASSERT_EQUAL(1u, state.valid, "state should initialize valid");
    ASSERT_EQUAL(0u, state.evaluation_count, "evaluation count defaults zero");
    ASSERT_EQUAL(0u, state.would_act_count, "would-act defaults zero");
}

FACT(PrometheusP15M11ShadowWouldAct_HealthyCanaryAndBlocked)
{
    prom_dominatus_shadow_would_act_state state{};
    prom_dominatus_shadow_would_act_init(&state);

    prom_dominatus_shadow_calibration_state calibration{};
    prom_dominatus_shadow_calibration_init(&calibration);
    calibration.sample_count = 10u;
    calibration.match_count = 9u;
    calibration.miss_count = 1u;
    calibration.total_abs_arrival_error_ticks = 2u;
    calibration.confidence = 0.9;
    calibration.lookahead_diagnostic_state = PROM_SHADOW_LOOKAHEAD_HEALTHY;

    prom_dominatus_shadow_authority_gate gate = prom_dominatus_shadow_authority_gate_evaluate(&calibration);
    prom_dominatus_shadow_snapshot snap = make_snapshot(1u, 2u, 3u);
    prom_dominatus_shadow_would_act_update(&state, &gate, &calibration, &snap);
    ASSERT_EQUAL(1u, state.would_act_count, "healthy should would-act");
    ASSERT_EQUAL(1u, state.would_healthy_count, "healthy counter increments");

    calibration.confidence = 0.65;
    calibration.lookahead_diagnostic_state = PROM_SHADOW_LOOKAHEAD_CAUTION;
    gate = prom_dominatus_shadow_authority_gate_evaluate(&calibration);
    snap = make_snapshot(2u, 3u, 4u);
    prom_dominatus_shadow_would_act_update(&state, &gate, &calibration, &snap);
    ASSERT_EQUAL(2u, state.would_act_count, "canary should would-act");
    ASSERT_EQUAL(1u, state.would_canary_count, "canary counter increments");

    calibration.confidence = 0.2;
    gate = prom_dominatus_shadow_authority_gate_evaluate(&calibration);
    snap = make_snapshot(3u, 4u, 5u);
    prom_dominatus_shadow_would_act_update(&state, &gate, &calibration, &snap);
    ASSERT_EQUAL(1u, state.would_block_count, "blocked counter increments");
    ASSERT_EQUAL(1u, state.blocked_low_confidence_count, "low confidence reason counter increments");
}

FACT(PrometheusP15M11ShadowWouldAct_ReasonBindingAndDedup)
{
    prom_dominatus_shadow_would_act_state state{};
    prom_dominatus_shadow_would_act_init(&state);

    prom_dominatus_shadow_calibration_state calibration{};
    prom_dominatus_shadow_calibration_init(&calibration);
    calibration.sample_count = 10u;
    calibration.match_count = 9u;
    calibration.miss_count = 1u;
    calibration.total_abs_arrival_error_ticks = 20u;
    calibration.confidence = 0.9;
    calibration.lookahead_diagnostic_state = PROM_SHADOW_LOOKAHEAD_HEALTHY;

    prom_dominatus_shadow_authority_gate gate = prom_dominatus_shadow_authority_gate_evaluate(&calibration);
    prom_dominatus_shadow_snapshot snap = make_snapshot(9u, 10u, 11u);
    prom_dominatus_shadow_would_act_update(&state, &gate, &calibration, &snap);
    ASSERT_EQUAL(1u, state.would_block_count, "high arrival error should block");
    ASSERT_EQUAL(1u, state.blocked_high_arrival_error_count, "arrival error reason counter increments");

    calibration.total_abs_arrival_error_ticks = 1u;
    calibration.stale_count = 1u;
    calibration.last_mismatch_kind = PROM_DOM_SHADOW_MISMATCH_STALE;
    gate = prom_dominatus_shadow_authority_gate_evaluate(&calibration);
    snap = make_snapshot(10u, 11u, 12u);
    prom_dominatus_shadow_would_act_update(&state, &gate, &calibration, &snap);
    ASSERT_EQUAL(1u, state.blocked_recent_stale_count, "recent stale binding should block");

    calibration.lookahead_diagnostic_state = PROM_SHADOW_LOOKAHEAD_DISABLED;
    calibration.fallback_count = 1u;
    calibration.last_mismatch_kind = PROM_DOM_SHADOW_MISMATCH_FALLBACK;
    gate = prom_dominatus_shadow_authority_gate_evaluate(&calibration);
    snap = make_snapshot(11u, 12u, 13u);
    prom_dominatus_shadow_would_act_update(&state, &gate, &calibration, &snap);
    ASSERT_TRUE(state.blocked_recent_fallback_count + state.blocked_lookahead_disabled_count >= 1u,
                "fallback/disabled binding should block");

    const uint64_t before = state.evaluation_count;
    prom_dominatus_shadow_would_act_update(&state, &gate, &calibration, &snap);
    ASSERT_EQUAL(before, state.evaluation_count, "same prediction key should dedup");
}
