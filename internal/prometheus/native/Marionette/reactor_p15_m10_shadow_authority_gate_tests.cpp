#include "../reactor_dominatus_predictor.h"
#include "test_harness.h"

FACT(PrometheusP15M10ShadowAuthority_DefaultsBlocked)
{
    prom_dominatus_shadow_calibration_state calibration{};
    prom_dominatus_shadow_calibration_init(&calibration);
    prom_dominatus_shadow_authority_gate gate = prom_dominatus_shadow_authority_gate_evaluate(&calibration);
    ASSERT_EQUAL(1u, gate.valid, "gate should evaluate on initialized calibration");
    ASSERT_EQUAL(PROM_SHADOW_AUTHORITY_UNKNOWN, gate.state, "insufficient samples should remain unknown");
    ASSERT_EQUAL(PROM_SHADOW_AUTHORITY_REASON_INSUFFICIENT_SAMPLES, gate.reason, "reason should report samples");
    ASSERT_EQUAL(0u, gate.canary_allowed, "canary remains blocked");
    ASSERT_EQUAL(0u, gate.authority_enabled, "authority must remain disabled");
    ASSERT_EQUAL(0u, gate.authority_would_act, "would-act remains false");
}

FACT(PrometheusP15M10ShadowAuthority_HealthyAndCanary)
{
    prom_dominatus_shadow_calibration_state healthy{};
    prom_dominatus_shadow_calibration_init(&healthy);
    healthy.sample_count = 10u;
    healthy.match_count = 9u;
    healthy.miss_count = 1u;
    healthy.total_abs_arrival_error_ticks = 4u;
    healthy.confidence = 0.90;
    healthy.lookahead_diagnostic_state = PROM_SHADOW_LOOKAHEAD_HEALTHY;

    prom_dominatus_shadow_authority_gate healthy_gate = prom_dominatus_shadow_authority_gate_evaluate(&healthy);
    ASSERT_EQUAL(PROM_SHADOW_AUTHORITY_HEALTHY, healthy_gate.state, "healthy confidence should be healthy");
    ASSERT_EQUAL(1u, healthy_gate.canary_allowed, "healthy should allow canary");
    ASSERT_TRUE(healthy_gate.recommended_lookahead_depth >= 1u, "healthy should recommend depth");

    prom_dominatus_shadow_calibration_state canary = healthy;
    canary.confidence = 0.68;
    canary.lookahead_diagnostic_state = PROM_SHADOW_LOOKAHEAD_CAUTION;
    prom_dominatus_shadow_authority_gate canary_gate = prom_dominatus_shadow_authority_gate_evaluate(&canary);
    ASSERT_EQUAL(PROM_SHADOW_AUTHORITY_CANARY_ELIGIBLE, canary_gate.state, "mid confidence should be canary-eligible");
}

FACT(PrometheusP15M10ShadowAuthority_BlockedReasons)
{
    prom_dominatus_shadow_calibration_state base{};
    prom_dominatus_shadow_calibration_init(&base);
    base.sample_count = 10u;
    base.match_count = 9u;
    base.miss_count = 1u;
    base.total_abs_arrival_error_ticks = 4u;
    base.confidence = 0.90;
    base.lookahead_diagnostic_state = PROM_SHADOW_LOOKAHEAD_HEALTHY;

    prom_dominatus_shadow_calibration_state low_conf = base;
    low_conf.confidence = 0.30;
    low_conf.lookahead_diagnostic_state = PROM_SHADOW_LOOKAHEAD_CAUTION;
    ASSERT_EQUAL(PROM_SHADOW_AUTHORITY_REASON_LOW_CONFIDENCE,
                 prom_dominatus_shadow_authority_gate_evaluate(&low_conf).reason,
                 "low confidence should block");

    prom_dominatus_shadow_calibration_state high_miss = base;
    high_miss.miss_count = 4u;
    ASSERT_EQUAL(PROM_SHADOW_AUTHORITY_REASON_HIGH_MISS_RATE,
                 prom_dominatus_shadow_authority_gate_evaluate(&high_miss).reason,
                 "high miss rate should block");

    prom_dominatus_shadow_calibration_state high_error = base;
    high_error.total_abs_arrival_error_ticks = 30u;
    ASSERT_EQUAL(PROM_SHADOW_AUTHORITY_REASON_HIGH_ARRIVAL_ERROR,
                 prom_dominatus_shadow_authority_gate_evaluate(&high_error).reason,
                 "high arrival error should block");

    prom_dominatus_shadow_calibration_state disabled = base;
    disabled.lookahead_diagnostic_state = PROM_SHADOW_LOOKAHEAD_DISABLED;
    ASSERT_EQUAL(PROM_SHADOW_AUTHORITY_DISABLED, prom_dominatus_shadow_authority_gate_evaluate(&disabled).state,
                 "disabled lookahead should disable authority");

    prom_dominatus_shadow_calibration_state stale = base;
    stale.stale_count = 1u;
    stale.last_mismatch_kind = PROM_DOM_SHADOW_MISMATCH_STALE;
    ASSERT_EQUAL(PROM_SHADOW_AUTHORITY_REASON_RECENT_STALE,
                 prom_dominatus_shadow_authority_gate_evaluate(&stale).reason,
                 "recent stale should block");
}

FACT(PrometheusP15M10ShadowAuthority_EnabledPropagatesInEvaluation)
{
    prom_dominatus_shadow_calibration_state calibration{};
    prom_dominatus_shadow_calibration_init(&calibration);
    auto disabled = prom_dominatus_shadow_authority_gate_evaluate_with_enabled(&calibration, 0u);
    auto enabled = prom_dominatus_shadow_authority_gate_evaluate_with_enabled(&calibration, 1u);
    ASSERT_EQUAL(0u, disabled.authority_enabled, "disabled configuration remains disabled");
    ASSERT_EQUAL(1u, enabled.authority_enabled, "enabled configuration propagates during evaluation");
}
