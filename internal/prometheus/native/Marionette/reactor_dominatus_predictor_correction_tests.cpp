#include "../reactor_api.h"
#include "../reactor_dominatus_predictor.h"
#include "test_harness.h"

static prom_dominatus_physical_observation ready_observation()
{
    prom_dominatus_physical_observation p{};
    p.slot_valid = 1u;
    p.memory_budget_ok = 1u;
    p.outstanding_depth_cap = 4u;
    p.actual_ready = 1u;
    return p;
}

FACT(PrometheusDominatusPredictorCorrection_CorrectMaturityMaturesReservation)
{
    prom_dominatus_predictor_state s{};
    prom_dominatus_predictor_init(&s, nullptr);
    s.params.max_lookahead_depth = 1u;
    prom_dominatus_predictor_evidence e{1u, 0u, 10.0, 10.0, 0.9, 8u, 0u};
    auto p = ready_observation();

    prom_dominatus_prediction_entry issued{};
    ASSERT_EQUAL(1u, prom_dominatus_predictor_issue(&s, &e, &p, 10u, &issued), "prediction issued");
    const auto reserve = prom_dominatus_predictor_try_reserve_future(&s, &s.reservations, &s.future_lease_seam.last_request, 10u);
    ASSERT_EQUAL(1u, reserve.reserved, "reservation created");
    const auto ev = prom_dominatus_predictor_mature(&s, &p, 11u);
    ASSERT_EQUAL(1u, ev.valid, "correction valid");
    ASSERT_EQUAL(0u, ev.state_mismatch, "correct maturity");
    ASSERT_EQUAL(1u, s.reservations.matured_count, "reservation matured");
    ASSERT_EQUAL(0u, s.reservations.active_count, "active decreases");
    ASSERT_EQUAL(PROM_DOM_FUTURE_LEASE_MATURED, s.future_lease_seam.last_request.state, "future lease matured");
}

FACT(PrometheusDominatusPredictorCorrection_MissExpiresReservation)
{
    prom_dominatus_predictor_state s{};
    prom_dominatus_predictor_init(&s, nullptr);
    s.params.max_lookahead_depth = 1u;
    prom_dominatus_predictor_evidence e{1u, 0u, 10.0, 10.0, 0.9, 8u, 0u};
    auto p = ready_observation();
    prom_dominatus_prediction_entry issued{};
    ASSERT_EQUAL(1u, prom_dominatus_predictor_issue(&s, &e, &p, 10u, &issued), "prediction issued");
    ASSERT_EQUAL(1u, prom_dominatus_predictor_try_reserve_future(&s, &s.reservations, &s.future_lease_seam.last_request, 10u).reserved, "reservation created");
    p.actual_ready = 0u;
    const auto before = s.prediction_confidence;
    const auto ev = prom_dominatus_predictor_mature(&s, &p, 11u);
    ASSERT_EQUAL(1u, ev.state_mismatch, "miss detected");
    ASSERT_TRUE(s.prediction_confidence < before, "confidence reduced");
    ASSERT_EQUAL(1u, s.reservations.expired_count, "reservation expired");
    ASSERT_EQUAL(PROM_DOM_FUTURE_LEASE_CANCELLED, s.future_lease_seam.last_request.state, "future lease cancelled");
}

FACT(PrometheusDominatusPredictorCorrection_HardGateCancelsReservation)
{
    prom_dominatus_predictor_state s{};
    prom_dominatus_predictor_init(&s, nullptr);
    s.params.max_lookahead_depth = 1u;
    prom_dominatus_predictor_evidence e{1u, 0u, 10.0, 10.0, 0.9, 8u, 0u};
    auto p = ready_observation();
    ASSERT_EQUAL(1u, prom_dominatus_predictor_issue(&s, &e, &p, 10u, nullptr), "prediction issued");
    ASSERT_EQUAL(1u, prom_dominatus_predictor_try_reserve_future(&s, &s.reservations, &s.future_lease_seam.last_request, 10u).reserved, "reservation created");
    p.runtime_unsafe = 1u;
    const auto ev = prom_dominatus_predictor_mature(&s, &p, 11u);
    ASSERT_EQUAL(PROM_DOM_CORRECTION_ACTION_MARK_STALE, ev.action, "hard gate marks stale");
    ASSERT_EQUAL(1u, s.fallback_active, "fallback active");
    ASSERT_EQUAL(1u, s.reservations.cancelled_count, "reservation cancelled");
}

FACT(PrometheusDominatusPredictorCorrection_NoReservationNoop)
{
    prom_dominatus_predictor_state s{};
    prom_dominatus_predictor_init(&s, nullptr);
    s.params.max_lookahead_depth = 1u;
    prom_dominatus_predictor_evidence e{1u, 0u, 10.0, 10.0, 0.9, 8u, 0u};
    auto p = ready_observation();
    ASSERT_EQUAL(1u, prom_dominatus_predictor_issue(&s, &e, &p, 10u, nullptr), "prediction issued");
    s.future_lease_seam.last_request.valid = 0u;
    s.ring[s.ring_head].lease_request_id = 0u;
    p.actual_ready = 0u;
    const auto ev = prom_dominatus_predictor_mature(&s, &p, 11u);
    ASSERT_EQUAL(1u, ev.valid, "correction still emitted");
    ASSERT_EQUAL(0u, s.reservations.cancelled_count + s.reservations.expired_count + s.reservations.matured_count, "reservation no-op");
}

FACT(PrometheusDominatusPredictorCorrection_TargetedCancellationOnly)
{
    prom_dominatus_predictor_state s{};
    prom_dominatus_predictor_init(&s, nullptr);
    s.params.max_lookahead_depth = 1u;
    prom_dominatus_predictor_evidence e{1u, 0u, 10.0, 10.0, 0.9, 8u, 0u};
    auto p = ready_observation();
    ASSERT_EQUAL(1u, prom_dominatus_predictor_issue(&s, &e, &p, 10u, nullptr), "first issued");
    const auto req1 = s.future_lease_seam.last_request;
    ASSERT_EQUAL(1u, prom_dominatus_predictor_try_reserve_future(&s, &s.reservations, &req1, 10u).reserved, "first reserved");
    ASSERT_EQUAL(1u, prom_dominatus_predictor_issue(&s, &e, &p, 11u, nullptr), "second issued");
    const auto req2 = s.future_lease_seam.last_request;
    ASSERT_EQUAL(1u, prom_dominatus_predictor_try_reserve_future(&s, &s.reservations, &req2, 11u).reserved, "second reserved");

    p.actual_ready = 0u;
    (void)prom_dominatus_predictor_mature(&s, &p, 11u);
    ASSERT_EQUAL(1u, s.reservations.expired_count, "one expired");
    ASSERT_EQUAL(1u, s.reservations.active_count, "one active remains");
}

FACT(PrometheusDominatusPredictorCorrection_ConfidenceRecovery)
{
    prom_dominatus_predictor_state s{};
    prom_dominatus_predictor_init(&s, nullptr);
    s.params.max_lookahead_depth = 1u;
    prom_dominatus_predictor_evidence e{1u, 0u, 10.0, 10.0, 0.9, 8u, 0u};
    auto p = ready_observation();

    ASSERT_EQUAL(1u, prom_dominatus_predictor_issue(&s, &e, &p, 10u, nullptr), "issued miss seed");
    p.actual_ready = 0u;
    (void)prom_dominatus_predictor_mature(&s, &p, 11u);
    const auto low = s.prediction_confidence;

    p.actual_ready = 1u;
    ASSERT_EQUAL(1u, prom_dominatus_predictor_issue(&s, &e, &p, 12u, nullptr), "issued recovery 1");
    (void)prom_dominatus_predictor_mature(&s, &p, 13u);
    ASSERT_EQUAL(1u, prom_dominatus_predictor_issue(&s, &e, &p, 14u, nullptr), "issued recovery 2");
    (void)prom_dominatus_predictor_mature(&s, &p, 15u);
    ASSERT_TRUE(s.prediction_confidence > low, "confidence recovered");
}

FACT(PrometheusDominatusPredictorCorrection_ReconciliationVariantMismatchExpiresReservationAndLowersConfidence)
{
    prom_dominatus_predictor_state s{};
    prom_dominatus_predictor_init(&s, nullptr);

    prom_dominatus_future_lease_request req{};
    req.valid = 1u;
    req.request_id = 71u;
    req.target_tick = 12u;
    req.shape_class = 3u;
    req.variant_id = 4u;
    req.lookahead_depth = 1u;
    req.confidence = 0.9;

    s.future_lease_seam.last_request = req;
    s.future_lease_seam.last_request.state = PROM_DOM_FUTURE_LEASE_GRANTED;
    ASSERT_EQUAL(1u,
                 prom_dominatus_predictor_try_reserve_future(&s, &s.reservations, &req, 10u).reserved,
                 "reservation created");
    ASSERT_EQUAL(1u, prom_dominatus_reservation_mature(&s.reservations, 12u).matured, "reservation matured");

    const auto before = s.prediction_confidence;
    const auto correction = prom_dominatus_predictor_apply_reconciliation_to_reservation(
        &s,
        &s.reservations,
        req.request_id,
        PROM_DOM_CORRECTION_ACTION_LOWER_CONFIDENCE,
        PROM_P15_SHADOW_FEEDFORWARD_BLOCK_VARIANT_MISMATCH,
        12u);

    ASSERT_EQUAL(1u, correction.expired, "variant mismatch should expire matured reservation");
    ASSERT_EQUAL(PROM_P15_SHADOW_FEEDFORWARD_BLOCK_VARIANT_MISMATCH, correction.reason, "reason preserved");
    ASSERT_TRUE(s.prediction_confidence < before, "confidence reduced");
    ASSERT_EQUAL(1u, s.correction_count, "correction counted");
}
