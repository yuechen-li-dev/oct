#include "../reactor_dominatus_predictor.h"
#include "test_harness.h"

FACT(PrometheusDominatusReservation_ValidFutureRequestReserves)
{
    prom_dominatus_reservation_state_set s{};
    const auto params = prom_dominatus_reservation_default_params();
    prom_dominatus_reservation_init(&s, &params);
    prom_dominatus_future_lease_request r{};
    r.valid = 1u; r.request_id = 11u; r.target_tick = 12u; r.lookahead_depth = 1u; r.confidence = 0.9;
    const auto d = prom_dominatus_reservation_request_from_future_lease(&s, &params, &r, 10u);
    ASSERT_EQUAL(1u, d.valid, "valid decision");
    ASSERT_EQUAL(1u, d.reserved, "reserved");
    ASSERT_EQUAL(PROM_DOM_RESERVATION_RESERVED, d.new_state, "state reserved");
    ASSERT_EQUAL(1u, s.active_count, "active +1");
    ASSERT_EQUAL(1u, s.reserved_count, "reserved count +1");
}

FACT(PrometheusDominatusReservation_SafetyGates)
{
    prom_dominatus_reservation_state_set s{};
    auto params = prom_dominatus_reservation_default_params();
    prom_dominatus_reservation_init(&s, &params);
    prom_dominatus_future_lease_request r{};
    r.valid = 1u; r.request_id = 1u; r.target_tick = 11u; r.lookahead_depth = 1u; r.confidence = 0.2;
    ASSERT_EQUAL(1u, prom_dominatus_reservation_request_from_future_lease(&s, &params, &r, 10u).denied, "low confidence denied");
    r.confidence = 0.8; r.lookahead_depth = 0u;
    ASSERT_EQUAL(1u, prom_dominatus_reservation_request_from_future_lease(&s, &params, &r, 10u).denied, "depth zero denied");
    r.lookahead_depth = 1u; r.target_tick = 10u;
    ASSERT_EQUAL(1u, prom_dominatus_reservation_request_from_future_lease(&s, &params, &r, 10u).denied, "target not future denied");
}

FACT(PrometheusDominatusReservation_CapacityCancelMature)
{
    prom_dominatus_reservation_state_set s{};
    auto params = prom_dominatus_reservation_default_params();
    params.capacity = 1u;
    prom_dominatus_reservation_init(&s, &params);
    prom_dominatus_future_lease_request r1{}; r1.valid = 1u; r1.request_id = 21u; r1.target_tick = 12u; r1.lookahead_depth = 1u; r1.confidence = 0.9;
    prom_dominatus_future_lease_request r2{}; r2.valid = 1u; r2.request_id = 22u; r2.target_tick = 12u; r2.lookahead_depth = 1u; r2.confidence = 0.9;
    ASSERT_EQUAL(1u, prom_dominatus_reservation_request_from_future_lease(&s, &params, &r1, 10u).reserved, "first reserved");
    ASSERT_EQUAL(1u, prom_dominatus_reservation_request_from_future_lease(&s, &params, &r2, 10u).denied, "capacity full denied");
    const auto c = prom_dominatus_reservation_cancel(&s, 21u, 7u, 10u);
    ASSERT_EQUAL(PROM_DOM_RESERVATION_CANCELLED, c.new_state, "cancelled");
    ASSERT_EQUAL(0u, s.active_count, "active decremented");

    ASSERT_EQUAL(1u, prom_dominatus_reservation_request_from_future_lease(&s, &params, &r1, 10u).reserved, "re-reserve after cancel");
    ASSERT_EQUAL(0u, prom_dominatus_reservation_mature(&s, 11u).matured, "not mature early");
    ASSERT_EQUAL(1u, prom_dominatus_reservation_mature(&s, 12u).matured, "matures on target tick");
}

FACT(PrometheusDominatusReservation_PredictorIntegrationSmoke)
{
    prom_dominatus_predictor_state p{};
    prom_dominatus_predictor_init(&p, nullptr);
    prom_dominatus_prediction_entry entry{};
    entry.active = 1u; entry.lookahead_depth = 1u; entry.target_tick = 12u; entry.prediction_confidence = 0.9;
    const auto issue = prom_dominatus_future_lease_request_issue(&p.future_lease_seam, &entry, 10u);
    ASSERT_EQUAL(1u, issue.valid, "issue valid");
    auto req = p.future_lease_seam.last_request;
    const auto d = prom_dominatus_predictor_try_reserve_future(&p, &p.reservations, &req, 10u);
    ASSERT_EQUAL(1u, d.reserved, "predictor reserve");
    ASSERT_EQUAL(0u, p.future_lease_granted, "immediate lease counters unchanged");
}
