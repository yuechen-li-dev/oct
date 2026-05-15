#include "../reactor_dominatus_predictor.h"
#include "test_harness.h"

FACT(PrometheusDominatusFutureLeaseSeam_RequestIssuedFromPrediction)
{
    prom_dominatus_future_lease_seam_state seam{};
    prom_dominatus_future_lease_seam_init(&seam);
    prom_dominatus_prediction_entry p{};
    p.active = 1u; p.lookahead_depth = 2u; p.target_tick = 42u; p.prediction_confidence = 0.8;
    const auto d = prom_dominatus_future_lease_request_issue(&seam, &p, 40u);
    ASSERT_EQUAL(1u, d.valid, "request should be issued");
    ASSERT_TRUE(d.request_id != 0u, "request id should be assigned");
    ASSERT_EQUAL(PROM_DOM_FUTURE_LEASE_REQUESTED, d.new_state, "state should be requested");
    ASSERT_EQUAL(1u, seam.requested_count, "requested count increments");
}

FACT(PrometheusDominatusFutureLeaseSeam_NoRequestForInactiveOrDepthZero)
{
    prom_dominatus_future_lease_seam_state seam{};
    prom_dominatus_future_lease_seam_init(&seam);
    prom_dominatus_prediction_entry p{};
    const auto d0 = prom_dominatus_future_lease_request_issue(&seam, &p, 0u);
    ASSERT_EQUAL(0u, d0.valid, "inactive prediction should not issue");
    p.active = 1u;
    const auto d1 = prom_dominatus_future_lease_request_issue(&seam, &p, 0u);
    ASSERT_EQUAL(0u, d1.valid, "depth zero prediction should not issue");
    ASSERT_EQUAL(0u, seam.requested_count, "requested count unchanged");
}

FACT(PrometheusDominatusFutureLeaseSeam_Transitions)
{
    prom_dominatus_future_lease_seam_state seam{};
    prom_dominatus_future_lease_seam_init(&seam);
    prom_dominatus_prediction_entry p{};
    p.active = 1u; p.lookahead_depth = 1u; p.target_tick = 9u; p.prediction_confidence = 0.6;
    const auto issue = prom_dominatus_future_lease_request_issue(&seam, &p, 8u);
    const auto grant = prom_dominatus_future_lease_grant(&seam, issue.request_id, 8u);
    ASSERT_EQUAL(PROM_DOM_FUTURE_LEASE_GRANTED, grant.new_state, "grant transition");
    ASSERT_EQUAL(1u, seam.granted_count, "granted count increments");
    const auto deny = prom_dominatus_future_lease_deny(&seam, issue.request_id, 7u, 8u);
    ASSERT_EQUAL(PROM_DOM_FUTURE_LEASE_DENIED, deny.new_state, "deny transition");
    ASSERT_EQUAL(1u, seam.denied_count, "denied count increments");
    ASSERT_EQUAL(7u, seam.last_request.deny_reason, "deny reason preserved");
    const auto cancel = prom_dominatus_future_lease_cancel(&seam, issue.request_id, 3u, 8u);
    ASSERT_EQUAL(PROM_DOM_FUTURE_LEASE_CANCELLED, cancel.new_state, "cancel transition");
    ASSERT_EQUAL(1u, seam.cancelled_count, "cancel count increments");
    ASSERT_EQUAL(3u, seam.last_request.cancel_reason, "cancel reason preserved");
    const auto mature = prom_dominatus_future_lease_mature(&seam, issue.request_id, 9u);
    ASSERT_EQUAL(PROM_DOM_FUTURE_LEASE_MATURED, mature.new_state, "mature transition");
    ASSERT_EQUAL(1u, seam.matured_count, "mature count increments");
}

FACT(PrometheusDominatusFutureLeaseSeam_Reset)
{
    prom_dominatus_future_lease_seam_state seam{};
    prom_dominatus_future_lease_seam_init(&seam);
    prom_dominatus_prediction_entry p{};
    p.active = 1u; p.lookahead_depth = 1u;
    (void)prom_dominatus_future_lease_request_issue(&seam, &p, 1u);
    prom_dominatus_future_lease_seam_reset(&seam);
    ASSERT_EQUAL(0u, seam.requested_count, "reset clears counters");
    ASSERT_EQUAL(0u, seam.next_request_id, "reset clears request id");
    ASSERT_EQUAL(0u, seam.last_request.valid, "reset clears last request");
}

FACT(PrometheusDominatusFutureLeaseSeam_PredictorIntegrationSmoke)
{
    prom_dominatus_predictor_state s{};
    prom_dominatus_predictor_init(&s, nullptr);
    prom_dominatus_predictor_evidence e{1u, 0u, 10.0, 10.0, 0.9, 8u, 0u};
    prom_dominatus_physical_observation p{};
    p.slot_valid = 1u; p.memory_budget_ok = 1u; p.outstanding_depth_cap = 4u;
    prom_dominatus_prediction_entry issued{};
    ASSERT_EQUAL(1u, prom_dominatus_predictor_issue(&s, &e, &p, 10u, &issued), "issue succeeds");
    ASSERT_TRUE(issued.lease_request_id != 0u, "predictor writes seam request id into prediction");
    ASSERT_EQUAL(1u, s.future_lease_seam.requested_count, "seam diagnostics updated");
}
