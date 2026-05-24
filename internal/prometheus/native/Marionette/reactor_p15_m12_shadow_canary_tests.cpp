#include "../reactor_api.h"
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

static prom_dominatus_shadow_calibration_state healthy_calibration()
{
    prom_dominatus_shadow_calibration_state c{};
    prom_dominatus_shadow_calibration_init(&c);
    c.sample_count = 10u;
    c.match_count = 9u;
    c.miss_count = 1u;
    c.total_abs_arrival_error_ticks = 1u;
    c.confidence = 0.9;
    c.lookahead_diagnostic_state = PROM_SHADOW_LOOKAHEAD_HEALTHY;
    return c;
}

FACT(PrometheusP15M12ShadowCanary_DefaultsDisabledNoOp)
{
    prom_dominatus_shadow_canary_state state{};
    prom_dominatus_shadow_canary_init(&state);
    auto params = prom_dominatus_shadow_canary_default_params();
    auto cal = healthy_calibration();
    auto gate = prom_dominatus_shadow_authority_gate_evaluate(&cal);
    auto snap = make_snapshot(1u, 2u, 3u);
    ASSERT_EQUAL(0u, params.enabled, "default canary must be disabled");
    ASSERT_EQUAL(0u, prom_dominatus_shadow_canary_should_attempt(&state, &params, &gate, &cal, &snap), "disabled canary no-op");
    ASSERT_EQUAL(0u, state.action_applied_count, "no action applied");
    ASSERT_EQUAL(0u, state.reservation_attempt_count, "reservation not attempted in should-attempt stage");
    ASSERT_EQUAL(1u, state.block_disabled_count, "disabled block counted");
}

FACT(PrometheusP15M12ShadowCanary_EnabledHealthyAttemptsAndDedup)
{
    prom_dominatus_shadow_canary_state state{};
    prom_dominatus_shadow_canary_init(&state);
    auto params = prom_dominatus_shadow_canary_default_params();
    params.enabled = 1u;
    auto cal = healthy_calibration();
    auto gate = prom_dominatus_shadow_authority_gate_evaluate(&cal);
    auto snap = make_snapshot(10u, 11u, 12u);
    ASSERT_EQUAL(1u, prom_dominatus_shadow_canary_should_attempt(&state, &params, &gate, &cal, &snap), "healthy enabled canary should attempt");
    ASSERT_EQUAL(1u, state.action_allowed_count, "allowed increments");

    state.last_applied_issued_tick = snap.issued_tick;
    state.last_applied_target_tick = snap.target_tick;
    state.last_applied_predicted_ready_tick = snap.predicted_ready_tick;
    ASSERT_EQUAL(0u, prom_dominatus_shadow_canary_should_attempt(&state, &params, &gate, &cal, &snap), "same key should dedup");
}

FACT(PrometheusP15M12ShadowCanary_BlockedReasons)
{
    prom_dominatus_shadow_canary_state state{};
    prom_dominatus_shadow_canary_init(&state);
    auto params = prom_dominatus_shadow_canary_default_params();
    params.enabled = 1u;
    auto cal = healthy_calibration();
    auto snap = make_snapshot(20u, 21u, 22u);

    cal.confidence = 0.78;
    auto gate = prom_dominatus_shadow_authority_gate_evaluate(&cal);
    ASSERT_EQUAL(0u, prom_dominatus_shadow_canary_should_attempt(&state, &params, &gate, &cal, &snap), "margin should block");
    ASSERT_EQUAL(0u, state.healthy_margin_passed, "margin flag blocked");
    ASSERT_EQUAL(1u, state.block_low_confidence_count, "margin maps to low-confidence block");

    cal = healthy_calibration();
    cal.stale_count = 1u;
    cal.last_mismatch_kind = PROM_DOM_SHADOW_MISMATCH_STALE;
    gate = prom_dominatus_shadow_authority_gate_evaluate(&cal);
    snap = make_snapshot(21u, 22u, 23u);
    ASSERT_EQUAL(0u, prom_dominatus_shadow_canary_should_attempt(&state, &params, &gate, &cal, &snap), "stale should block");
    ASSERT_EQUAL(1u, state.block_recent_stale_count, "stale block counted");

    cal = healthy_calibration();
    cal.lookahead_diagnostic_state = PROM_SHADOW_LOOKAHEAD_DISABLED;
    gate = prom_dominatus_shadow_authority_gate_evaluate(&cal);
    snap = make_snapshot(22u, 23u, 24u);
    ASSERT_EQUAL(0u, prom_dominatus_shadow_canary_should_attempt(&state, &params, &gate, &cal, &snap), "lookahead-disabled fallback should block");
    ASSERT_TRUE(state.block_recent_fallback_count + state.block_high_miss_rate_count >= 1u, "fallback/disabled block counted");

    cal = healthy_calibration();
    cal.total_abs_arrival_error_ticks = 100u;
    gate = prom_dominatus_shadow_authority_gate_evaluate(&cal);
    snap = make_snapshot(23u, 24u, 25u);
    ASSERT_EQUAL(0u, prom_dominatus_shadow_canary_should_attempt(&state, &params, &gate, &cal, &snap), "arrival error should block");
    ASSERT_EQUAL(1u, state.block_high_arrival_error_count, "arrival error block counted");

    cal = healthy_calibration();
    cal.confidence = 0.2;
    gate = prom_dominatus_shadow_authority_gate_evaluate(&cal);
    snap = make_snapshot(24u, 25u, 26u);
    ASSERT_EQUAL(0u, prom_dominatus_shadow_canary_should_attempt(&state, &params, &gate, &cal, &snap), "low confidence block");
    ASSERT_TRUE(state.block_low_confidence_count >= 2u, "low confidence counted");

    cal = healthy_calibration();
    cal.match_count = 1u;
    cal.miss_count = 9u;
    gate = prom_dominatus_shadow_authority_gate_evaluate(&cal);
    snap = make_snapshot(25u, 26u, 27u);
    ASSERT_EQUAL(0u, prom_dominatus_shadow_canary_should_attempt(&state, &params, &gate, &cal, &snap), "high miss rate block");
    ASSERT_EQUAL(1u, state.block_high_miss_rate_count, "miss-rate block counted");

    cal = healthy_calibration();
    cal.sample_count = 1u;
    gate = prom_dominatus_shadow_authority_gate_evaluate(&cal);
    snap = make_snapshot(26u, 27u, 28u);
    ASSERT_EQUAL(0u, prom_dominatus_shadow_canary_should_attempt(&state, &params, &gate, &cal, &snap), "insufficient samples block");
    ASSERT_EQUAL(1u, state.block_insufficient_samples_count, "sample block counted");
}

FACT(PrometheusReactorP15M12ShadowCanary_DiagnosticsAndInvalidTiming)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);
    cfg.test_flags = PROM_TESTCFG_SKIP_SUBMIT_WAIT;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");
    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diag query succeeds");
    ASSERT_EQUAL(0u, diag.p15_shadow_canary_enabled, "default-off exported");
    ASSERT_EQUAL(0u, diag.p15_shadow_canary_evaluation_count, "invalid timing should not evaluate canary");
    ASSERT_EQUAL(0u, diag.p15_shadow_canary_action_applied_count, "invalid timing no actuation");
    ASSERT_EQUAL(0u, diag.p15_shadow_canary_reservation_attempt_count, "invalid timing no reservation attempts");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}
