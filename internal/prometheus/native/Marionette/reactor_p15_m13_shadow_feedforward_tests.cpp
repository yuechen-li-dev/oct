#include "../reactor_api.h"
#include "../reactor_dominatus_predictor.h"
#include "test_harness.h"

FACT(PrometheusP15M13ShadowFeedforward_DefaultOffDoesNotEnableAuthority)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);
    cfg.test_flags = PROM_TESTCFG_SKIP_SUBMIT_WAIT;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");
    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diag query succeeds");
    ASSERT_EQUAL(0u, diag.p15_shadow_canary_enabled, "default canary off");
    ASSERT_EQUAL(0u, diag.p15_shadow_authority_enabled, "authority must remain off when feature flag is off");
    ASSERT_EQUAL(0u, diag.p15_shadow_feedforward_enabled, "feedforward disabled by default");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusP15M13ShadowFeedforward_EnabledFlagPropagatesToAuthority)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);
    cfg.test_flags = PROM_TESTCFG_SKIP_SUBMIT_WAIT;
    cfg.p15_shadow_canary_enabled = 1u;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");
    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diag query succeeds");
    ASSERT_EQUAL(1u, diag.p15_shadow_canary_enabled, "enabled canary exported");
    ASSERT_EQUAL(1u, diag.p15_shadow_authority_enabled, "authority should track canary feature flag");
    ASSERT_EQUAL(0u, diag.p15_shadow_feedforward_used, "no dispatch means no feedforward use");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusP15M13ShadowFeedforward_MatureReservationConsumeOnce)
{
    prom_dominatus_reservation_state_set reservations{};
    const auto params = prom_dominatus_reservation_default_params();
    prom_dominatus_reservation_init(&reservations, &params);

    prom_dominatus_future_lease_request req{};
    req.valid = 1u;
    req.request_id = 17u;
    req.target_tick = 12u;
    req.shape_class = 3u;
    req.variant_id = 9u;
    req.lookahead_depth = 1u;
    req.confidence = 0.9;
    const auto reserved = prom_dominatus_reservation_request_from_future_lease(&reservations, &params, &req, 10u);
    ASSERT_EQUAL(1u, reserved.reserved, "reservation should be created");

    const auto matured = prom_dominatus_reservation_mature(&reservations, 12u);
    ASSERT_EQUAL(1u, matured.matured, "reservation should mature");

    const auto consumed = prom_dominatus_reservation_consume_matured(&reservations, 3u, 9u);
    ASSERT_EQUAL(1u, consumed.yielded, "matching matured reservation should be consumed");

    const auto consumed_again = prom_dominatus_reservation_consume_matured(&reservations, 3u, 9u);
    ASSERT_EQUAL(0u, consumed_again.valid, "already consumed reservation must not be consumed twice");
}

FACT(PrometheusP15M13ShadowFeedforward_MatureReservationShapeVariantMismatchFallsBack)
{
    prom_dominatus_reservation_state_set reservations{};
    const auto params = prom_dominatus_reservation_default_params();
    prom_dominatus_reservation_init(&reservations, &params);

    prom_dominatus_future_lease_request req{};
    req.valid = 1u;
    req.request_id = 29u;
    req.target_tick = 42u;
    req.shape_class = 5u;
    req.variant_id = 4u;
    req.lookahead_depth = 1u;
    req.confidence = 0.9;
    ASSERT_EQUAL(1u, prom_dominatus_reservation_request_from_future_lease(&reservations, &params, &req, 40u).reserved,
                 "reservation should be created");
    ASSERT_EQUAL(1u, prom_dominatus_reservation_mature(&reservations, 42u).matured, "reservation should mature");

    ASSERT_EQUAL(0u, prom_dominatus_reservation_consume_matured(&reservations, 6u, 4u).valid,
                 "shape mismatch should not consume");
    ASSERT_EQUAL(0u, prom_dominatus_reservation_consume_matured(&reservations, 5u, 8u).valid,
                 "variant mismatch should not consume");
}


FACT(PrometheusP15M13ShadowFeedforward_ConsumeChoosesEarliestMaturedDeterministically)
{
    prom_dominatus_reservation_state_set reservations{};
    const auto params = prom_dominatus_reservation_default_params();
    prom_dominatus_reservation_init(&reservations, &params);

    prom_dominatus_future_lease_request early{};
    early.valid = 1u; early.request_id = 31u; early.target_tick = 12u; early.shape_class = 3u; early.variant_id = 9u; early.lookahead_depth = 1u; early.confidence = 0.9;
    prom_dominatus_future_lease_request late = early;
    late.request_id = 32u; late.target_tick = 12u;

    ASSERT_EQUAL(1u, prom_dominatus_reservation_request_from_future_lease(&reservations, &params, &late, 10u).reserved,
                 "late reservation created");
    ASSERT_EQUAL(1u, prom_dominatus_reservation_request_from_future_lease(&reservations, &params, &early, 10u).reserved,
                 "early reservation created");

    ASSERT_EQUAL(1u, prom_dominatus_reservation_mature(&reservations, 12u).matured, "a reservation matured");
    ASSERT_EQUAL(1u, prom_dominatus_reservation_mature(&reservations, 12u).matured, "second reservation matured");

    const auto consumed_first = prom_dominatus_reservation_consume_matured(&reservations, 3u, 9u);
    ASSERT_EQUAL(1u, consumed_first.valid, "first consume valid");
    ASSERT_EQUAL(32u, consumed_first.request_id, "equal target_tick breaks ties by insertion order");

    const auto consumed_second = prom_dominatus_reservation_consume_matured(&reservations, 3u, 9u);
    ASSERT_EQUAL(1u, consumed_second.valid, "second consume valid");
    ASSERT_EQUAL(31u, consumed_second.request_id, "remaining matured reservation consumed second");
}

FACT(PrometheusP15M13ShadowFeedforward_NoMaturedReservationDoesNotConsume)
{
    prom_dominatus_reservation_state_set reservations{};
    const auto params = prom_dominatus_reservation_default_params();
    prom_dominatus_reservation_init(&reservations, &params);

    prom_dominatus_future_lease_request req{};
    req.valid = 1u;
    req.request_id = 41u;
    req.target_tick = 12u;
    req.shape_class = 2u;
    req.variant_id = 1u;
    req.lookahead_depth = 1u;
    req.confidence = 0.95;
    ASSERT_EQUAL(1u, prom_dominatus_reservation_request_from_future_lease(&reservations, &params, &req, 10u).reserved,
                 "reservation should be created");

    const auto consumed = prom_dominatus_reservation_consume_matured(&reservations, 2u, 1u);
    ASSERT_EQUAL(0u, consumed.valid, "non-matured reservation must not consume");
}

FACT(PrometheusP15M13ShadowFeedforward_StaleReservationExpiresAndCannotConsume)
{
    prom_dominatus_reservation_state_set reservations{};
    auto params = prom_dominatus_reservation_default_params();
    params.expiry_slack_ticks = 0u;
    prom_dominatus_reservation_init(&reservations, &params);

    prom_dominatus_future_lease_request req{};
    req.valid = 1u;
    req.request_id = 51u;
    req.target_tick = 12u;
    req.shape_class = 8u;
    req.variant_id = 3u;
    req.lookahead_depth = 1u;
    req.confidence = 0.91;
    ASSERT_EQUAL(1u, prom_dominatus_reservation_request_from_future_lease(&reservations, &params, &req, 10u).reserved,
                 "reservation should be created");

    ASSERT_EQUAL(1u, prom_dominatus_reservation_expire_stale(&reservations, &params, 14u).expired,
                 "reservation should expire when stale");
    ASSERT_EQUAL(0u, prom_dominatus_reservation_consume_matured(&reservations, 8u, 3u).valid,
                 "expired reservation must not be consumable");
}
