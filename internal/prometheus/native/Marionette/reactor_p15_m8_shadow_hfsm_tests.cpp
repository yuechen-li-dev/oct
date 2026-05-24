#include "../reactor_dominatus_predictor.h"
#include "test_harness.h"

static prom_dominatus_predictor_state base_predictor()
{
    prom_dominatus_predictor_state s{};
    prom_dominatus_predictor_init(&s, nullptr);
    s.prediction_confidence = 0.8;
    return s;
}

static prom_dominatus_prediction_entry issued(uint64_t issued_tick, uint64_t target_tick)
{
    prom_dominatus_prediction_entry e{};
    e.active = 1u;
    e.issued_tick = issued_tick;
    e.target_tick = target_tick;
    e.future_lease_state = PROM_DOM_FUTURE_LEASE_REQUESTED;
    return e;
}

FACT(PrometheusP15M8Shadow_DefaultIdleWithoutPrediction)
{
    auto s = base_predictor();
    auto snap = prom_dominatus_shadow_snapshot_evaluate(&s, nullptr, nullptr, nullptr, 0u, 10u);
    ASSERT_EQUAL(1u, snap.valid, "snapshot valid");
    ASSERT_EQUAL(PROM_DOM_SHADOW_STATE_IDLE, snap.shadow_state, "idle without prediction");
    ASSERT_EQUAL(PROM_DOM_SHADOW_MISMATCH_NONE, snap.mismatch_kind, "no fake mismatch");
}

FACT(PrometheusP15M8Shadow_ReservationAndPrestageStates)
{
    auto s = base_predictor();
    auto pred = issued(10u, 12u);
    prom_dominatus_reservation_decision reserve{};
    reserve.valid = 1u;
    reserve.reserved = 1u;
    auto snap_reserved = prom_dominatus_shadow_snapshot_evaluate(&s, &pred, nullptr, &reserve, 0u, 11u);
    ASSERT_EQUAL(PROM_DOM_SHADOW_STATE_RESERVED, snap_reserved.shadow_state, "reservation reflected");

    auto snap_prestage = prom_dominatus_shadow_snapshot_evaluate(&s, &pred, nullptr, &reserve, 1u, 11u);
    ASSERT_EQUAL(PROM_DOM_SHADOW_STATE_PRESTAGE_ELIGIBLE, snap_prestage.shadow_state, "prestage reflected");
}

FACT(PrometheusP15M8Shadow_MatchMissAndArrivalError)
{
    auto s = base_predictor();
    auto pred = issued(10u, 12u);
    prom_dominatus_correction_event c{};
    c.valid = 1u;
    c.prediction_matured = 1u;
    c.predicted_ready = 1u;
    c.actual_ready = 1u;
    c.tick = 12u;
    c.target_tick = 12u;
    c.arrival_error_ticks = 0;
    auto match = prom_dominatus_shadow_snapshot_evaluate(&s, &pred, &c, nullptr, 0u, 12u);
    ASSERT_EQUAL(1u, match.matched, "match flagged");
    ASSERT_EQUAL(PROM_DOM_SHADOW_MISMATCH_MATCH, match.mismatch_kind, "match mismatch kind");

    c.actual_ready = 0u;
    auto miss = prom_dominatus_shadow_snapshot_evaluate(&s, &pred, &c, nullptr, 0u, 12u);
    ASSERT_EQUAL(PROM_DOM_SHADOW_MISMATCH_PHYSICAL_NOT_READY, miss.mismatch_kind, "miss classified");
    ASSERT_EQUAL(1u, miss.miss_count, "miss count increments");

    c.actual_ready = 1u;
    c.arrival_error_ticks = 2;
    auto late = prom_dominatus_shadow_snapshot_evaluate(&s, &pred, &c, nullptr, 0u, 14u);
    ASSERT_EQUAL(PROM_DOM_SHADOW_MISMATCH_LATE, late.mismatch_kind, "late classification");
}

FACT(PrometheusP15M8Shadow_CancelStaleFallback)
{
    auto s = base_predictor();
    auto pred = issued(10u, 11u);
    prom_dominatus_reservation_decision reserve{};
    reserve.valid = 1u;
    reserve.cancelled = 1u;
    auto cancelled = prom_dominatus_shadow_snapshot_evaluate(&s, &pred, nullptr, &reserve, 0u, 11u);
    ASSERT_EQUAL(PROM_DOM_SHADOW_STATE_CANCELLED, cancelled.shadow_state, "cancel state");

    auto stale = prom_dominatus_shadow_snapshot_evaluate(&s, &pred, nullptr, nullptr, 0u, 20u);
    ASSERT_EQUAL(PROM_DOM_SHADOW_STATE_STALE, stale.shadow_state, "stale state");

    s.fallback_active = 1u;
    auto fallback = prom_dominatus_shadow_snapshot_evaluate(&s, &pred, nullptr, nullptr, 0u, 11u);
    ASSERT_EQUAL(PROM_DOM_SHADOW_STATE_FALLBACK, fallback.shadow_state, "fallback state");
}
