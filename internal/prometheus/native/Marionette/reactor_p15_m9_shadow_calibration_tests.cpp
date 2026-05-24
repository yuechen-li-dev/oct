#include "../reactor_dominatus_predictor.h"
#include "test_harness.h"

static prom_dominatus_shadow_snapshot base_snapshot(prom_dominatus_shadow_mismatch_kind kind, int64_t error_ticks)
{
    prom_dominatus_shadow_snapshot snap{};
    snap.valid = 1u;
    snap.issued_tick = 10u;
    snap.target_tick = 12u;
    snap.predicted_ready_tick = 12u;
    snap.mismatch_kind = kind;
    snap.arrival_error_ticks = error_ticks;
    return snap;
}

FACT(PrometheusP15M9ShadowCalibration_DefaultsNeutral)
{
    prom_dominatus_shadow_calibration_state state{};
    prom_dominatus_shadow_calibration_init(&state);
    ASSERT_EQUAL(1u, state.valid, "calibration valid");
    ASSERT_EQUAL(0u, state.sample_count, "no samples");
    ASSERT_TRUE(state.confidence == 0.5, "neutral confidence");
    ASSERT_EQUAL(PROM_SHADOW_LOOKAHEAD_UNKNOWN, state.lookahead_diagnostic_state, "unknown lookahead");
}

FACT(PrometheusP15M9ShadowCalibration_MatchMissArrivalAndDedup)
{
    prom_dominatus_shadow_calibration_state state{};
    prom_dominatus_shadow_calibration_init(&state);

    auto match = base_snapshot(PROM_DOM_SHADOW_MISMATCH_MATCH, 0);
    prom_dominatus_shadow_calibration_update(&state, &match);
    ASSERT_EQUAL(1u, state.sample_count, "sample increments");
    ASSERT_EQUAL(1u, state.match_count, "match increments");
    ASSERT_TRUE(state.confidence > 0.5, "match increases confidence");

    prom_dominatus_shadow_calibration_update(&state, &match);
    ASSERT_EQUAL(1u, state.sample_count, "dedup prevents double count");

    auto miss = base_snapshot(PROM_DOM_SHADOW_MISMATCH_PHYSICAL_NOT_READY, 0);
    miss.issued_tick = 13u;
    miss.target_tick = 14u;
    miss.predicted_ready_tick = 14u;
    prom_dominatus_shadow_calibration_update(&state, &miss);
    ASSERT_EQUAL(2u, state.sample_count, "miss counted");
    ASSERT_EQUAL(1u, state.miss_count, "miss increments");
    ASSERT_EQUAL(1u, state.physical_not_ready_count, "pnr increments");
    ASSERT_TRUE(state.confidence < 0.55, "miss decreases confidence");

    auto late = base_snapshot(PROM_DOM_SHADOW_MISMATCH_LATE, 3);
    late.issued_tick = 15u;
    late.target_tick = 16u;
    late.predicted_ready_tick = 16u;
    prom_dominatus_shadow_calibration_update(&state, &late);
    ASSERT_EQUAL(1u, state.late_count, "late increments");
    ASSERT_EQUAL(3u, state.last_arrival_error_ticks, "last error updated");
    ASSERT_EQUAL(3u, state.max_abs_arrival_error_ticks, "max abs tracks");
    ASSERT_TRUE(state.total_abs_arrival_error_ticks >= 3u, "abs accumulation");
}

FACT(PrometheusP15M9ShadowCalibration_StateClassification)
{
    prom_dominatus_shadow_calibration_state state{};
    prom_dominatus_shadow_calibration_init(&state);

    auto fallback = base_snapshot(PROM_DOM_SHADOW_MISMATCH_FALLBACK, 0);
    prom_dominatus_shadow_calibration_update(&state, &fallback);
    ASSERT_EQUAL(PROM_SHADOW_LOOKAHEAD_DISABLED, state.lookahead_diagnostic_state, "fallback disables lookahead diagnostics");

    prom_dominatus_shadow_calibration_reset(&state);
    auto m1 = base_snapshot(PROM_DOM_SHADOW_MISMATCH_MATCH, 0); m1.issued_tick = 1u; m1.target_tick = 2u; m1.predicted_ready_tick = 2u;
    auto m2 = base_snapshot(PROM_DOM_SHADOW_MISMATCH_MATCH, 0); m2.issued_tick = 3u; m2.target_tick = 4u; m2.predicted_ready_tick = 4u;
    auto m3 = base_snapshot(PROM_DOM_SHADOW_MISMATCH_MATCH, 0); m3.issued_tick = 5u; m3.target_tick = 6u; m3.predicted_ready_tick = 6u;
    prom_dominatus_shadow_calibration_update(&state, &m1);
    prom_dominatus_shadow_calibration_update(&state, &m2);
    prom_dominatus_shadow_calibration_update(&state, &m3);
    auto m4 = base_snapshot(PROM_DOM_SHADOW_MISMATCH_MATCH, 0); m4.issued_tick = 7u; m4.target_tick = 8u; m4.predicted_ready_tick = 8u;
    auto m5 = base_snapshot(PROM_DOM_SHADOW_MISMATCH_MATCH, 0); m5.issued_tick = 9u; m5.target_tick = 10u; m5.predicted_ready_tick = 10u;
    prom_dominatus_shadow_calibration_update(&state, &m4);
    prom_dominatus_shadow_calibration_update(&state, &m5);
    ASSERT_EQUAL(PROM_SHADOW_LOOKAHEAD_HEALTHY, state.lookahead_diagnostic_state, "high confidence healthy");

    auto bad = base_snapshot(PROM_DOM_SHADOW_MISMATCH_PHYSICAL_NOT_READY, 0);
    bad.issued_tick = 11u; bad.target_tick = 12u; bad.predicted_ready_tick = 12u;
    for (int i = 0; i < 6; ++i) {
        bad.issued_tick += 2u;
        bad.target_tick += 2u;
        bad.predicted_ready_tick += 2u;
        prom_dominatus_shadow_calibration_update(&state, &bad);
    }
    ASSERT_TRUE(state.confidence < 0.45, "confidence drops after misses");
    ASSERT_EQUAL(PROM_SHADOW_LOOKAHEAD_UNRELIABLE, state.lookahead_diagnostic_state, "low confidence unreliable");
}
