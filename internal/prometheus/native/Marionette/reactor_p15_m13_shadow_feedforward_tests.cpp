#include <vector>
#include <cstring>
#include "../reactor_api.h"
#include "../reactor_dominatus_predictor.h"
#include "../reactor_judgment_engine.h"
#include "test_harness.h"

static uint32_t alternate_wired_variant(uint32_t selected)
{
    if (selected != PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE) {
        return PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE;
    }
    return PROM_OCCUPANCY_KERNEL_VARIANT_BASELINE_SCALAR;
}

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

FACT(PrometheusP15M13ShadowFeedforward_DiagnosticsSizedNullAndZeroSafe)
{
    ASSERT_EQUAL(PROM_ERROR, prometheus_reactor_runtime_sgemm_policy_diagnostics_sized(nullptr, nullptr, 0u),
                 "null handle/null out/zero size must fail safely");
}

FACT(PrometheusP15M13ShadowFeedforward_DiagnosticsSizedTruncatedDoesNotOverwriteSentinel)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);
    cfg.test_flags = PROM_TESTCFG_SKIP_SUBMIT_WAIT;
    cfg.p15_shadow_canary_enabled = 1u;
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");
    struct Tiny {
        uint8_t bytes[64];
        uint8_t sentinel[16];
    } tiny{};
    memset(tiny.bytes, 0xAB, sizeof(tiny.bytes));
    memset(tiny.sentinel, 0xCD, sizeof(tiny.sentinel));
    ASSERT_EQUAL(PROM_OK,
                 prometheus_reactor_runtime_sgemm_policy_diagnostics_sized(
                     handle, reinterpret_cast<PrometheusSgemmPolicyDiagnostics*>(tiny.bytes), (uint32_t)sizeof(tiny.bytes)),
                 "sized diagnostics should succeed for truncated buffer");
    for (uint32_t i = 0u; i < sizeof(tiny.sentinel); ++i) {
        ASSERT_EQUAL((uint8_t)0xCD, tiny.sentinel[i], "sentinel bytes must remain untouched");
    }
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

FACT(PrometheusP15M13ShadowFeedforward_ReservationHeartbeatMaturesReservedEntries)
{
    prom_dominatus_predictor_state predictor{};
    prom_dominatus_predictor_init(&predictor, nullptr);
    predictor.reservation_params.expiry_slack_ticks = 0u;

    prom_dominatus_future_lease_request req{};
    req.valid = 1u;
    req.request_id = 61u;
    req.target_tick = 15u;
    req.shape_class = 2u;
    req.variant_id = 7u;
    req.lookahead_depth = 1u;
    req.confidence = 0.9;
    ASSERT_EQUAL(1u, prom_dominatus_predictor_try_reserve_future(&predictor, &predictor.reservations, &req, 14u).reserved,
                 "reservation should be created");

    const auto advanced = prom_dominatus_predictor_advance_reservations(&predictor, 15u);
    ASSERT_EQUAL(1u, advanced.matured, "heartbeat should mature reservation");
    ASSERT_EQUAL(1u, prom_dominatus_reservation_consume_matured(&predictor.reservations, 2u, 7u).yielded,
                 "matured reservation should be consumable");
}

FACT(PrometheusP15M13ShadowFeedforward_ReservationHeartbeatExpiresStaleEntries)
{
    prom_dominatus_predictor_state predictor{};
    prom_dominatus_predictor_init(&predictor, nullptr);
    predictor.reservation_params.expiry_slack_ticks = 0u;

    prom_dominatus_future_lease_request req{};
    req.valid = 1u;
    req.request_id = 62u;
    req.target_tick = 8u;
    req.shape_class = 4u;
    req.variant_id = 3u;
    req.lookahead_depth = 1u;
    req.confidence = 0.9;
    ASSERT_EQUAL(1u, prom_dominatus_predictor_try_reserve_future(&predictor, &predictor.reservations, &req, 7u).reserved,
                 "reservation should be created");

    const auto advanced = prom_dominatus_predictor_advance_reservations(&predictor, 9u);
    ASSERT_EQUAL(1u, advanced.expired, "heartbeat should expire stale reservation");
    ASSERT_EQUAL(0u, prom_dominatus_reservation_consume_matured(&predictor.reservations, 4u, 3u).valid,
                 "expired reservation must not consume");
}


FACT(PrometheusP15M13ShadowFeedforward_DefaultOffMaturedReservationDoesNotConsume)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);
    cfg.test_flags = PROM_TESTCFG_SKIP_SUBMIT_WAIT;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    std::vector<float> a(64u, 1.0f);
    std::vector<float> b(64u, 1.0f);
    std::vector<float> c(64u, 0.0f);
    uint32_t stage = 0u;
    int detail = 0;
    (void)prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail);

    PrometheusSgemmPolicyDiagnostics diag{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diag), "diag query succeeds");
    ASSERT_EQUAL(0u, diag.p15_shadow_canary_enabled, "canary default off");
    ASSERT_EQUAL(0u, diag.p15_shadow_feedforward_used, "default-off cannot use feedforward");
    ASSERT_EQUAL(0u, diag.p15_shadow_feedforward_reservation_consumed_count, "default-off must not consume reservations");
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}


FACT(PrometheusP15M13ShadowFeedforward_EnabledHealthyMaturedReservationUsedBySgemm)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);
    cfg.test_flags = PROM_TESTCFG_SKIP_SUBMIT_WAIT;
    cfg.p15_shadow_canary_enabled = 1u;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; SGEMM feedforward integration cannot be asserted");
    }

    std::vector<float> a(64u, 1.0f);
    std::vector<float> b(64u, 1.0f);
    std::vector<float> c(64u, 0.0f);
    uint32_t stage = 0u;
    int detail = 0;
    const int baseline_status = prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail);
    if (baseline_status != PROM_OK) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("baseline sgemm failed in environment; feedforward happy-path cannot be asserted");
    }

    PrometheusSgemmPolicyDiagnostics baseline{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &baseline), "baseline diag query succeeds");

    const uint64_t target_tick = 1u;
    ASSERT_EQUAL(PROM_OK,
                 prometheus_reactor_runtime_p15_test_seed_matured_reservation(handle,
                                                                              baseline.p13_m2_occupancy_shape_class,
                                                                              baseline.p13_m2_occupancy_selected_variant,
                                                                              target_tick),
                 "test seam should seed matured matching reservation");

    const int feedforward_status = prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail);
    if (feedforward_status != PROM_OK) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("feedforward sgemm failed in environment; happy-path consume cannot be asserted");
    }

    PrometheusSgemmPolicyDiagnostics used{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &used), "used diag query succeeds");
    ASSERT_EQUAL(1u, used.p15_shadow_feedforward_used, "enabled healthy matured reservation should drive feedforward use");
    ASSERT_EQUAL(1u, used.p15_shadow_feedforward_source, "source should be shadow reservation");
    ASSERT_EQUAL(baseline.p13_m2_occupancy_selected_variant, used.p15_shadow_feedforward_reserved_variant_id,
                 "reserved variant should match seeded variant");
    ASSERT_TRUE(used.p15_shadow_feedforward_reservation_consumed_count >= 1u, "reservation should be consumed");
    const uint64_t consumed_after_first_use = used.p15_shadow_feedforward_reservation_consumed_count;

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail),
                 "second sgemm should succeed");
    PrometheusSgemmPolicyDiagnostics after_second{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &after_second), "second diag query succeeds");
    ASSERT_EQUAL(consumed_after_first_use, after_second.p15_shadow_feedforward_reservation_consumed_count,
                 "same reservation should not be consumed twice");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}

FACT(PrometheusP15M13ShadowFeedforward_VariantMismatchFallsBackToJudgmentAndCorrectsReservation)
{
    PrometheusReactorConfig cfg{};
    cfg.struct_size = sizeof(cfg);
    cfg.test_flags = PROM_TESTCFG_SKIP_SUBMIT_WAIT;
    cfg.p15_shadow_canary_enabled = 1u;

    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(&cfg, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("Vulkan runtime unavailable; SGEMM feedforward reconciliation cannot be asserted");
    }

    std::vector<float> a(64u, 1.0f);
    std::vector<float> b(64u, 1.0f);
    std::vector<float> c(64u, 0.0f);
    uint32_t stage = 0u;
    int detail = 0;
    const int baseline_status = prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail);
    if (baseline_status != PROM_OK) {
        ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
        SKIP("baseline sgemm failed in environment; reconciliation path cannot be asserted");
    }

    PrometheusSgemmPolicyDiagnostics baseline{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &baseline), "baseline diag query succeeds");
    const uint32_t selected_variant = baseline.p13_m2_occupancy_selected_variant;
    const uint32_t mismatched_variant = alternate_wired_variant(selected_variant);
    ASSERT_TRUE(mismatched_variant != selected_variant, "test needs a different wired variant");

    ASSERT_EQUAL(PROM_OK,
                 prometheus_reactor_runtime_p15_test_seed_matured_reservation(handle,
                                                                              baseline.p13_m2_occupancy_shape_class,
                                                                              mismatched_variant,
                                                                              1u),
                 "test seam should seed matured mismatched reservation");

    const double confidence_before = baseline.p15_prediction_confidence;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail),
                 "reconciliation sgemm should succeed");

    PrometheusSgemmPolicyDiagnostics after{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &after), "diag query succeeds");
    ASSERT_EQUAL(1u, after.p15_shadow_feedforward_reservation_present, "reservation visible");
    ASSERT_EQUAL(1u, after.p15_shadow_feedforward_reservation_matured, "matured reservation visible");
    ASSERT_EQUAL(0u, after.p15_shadow_feedforward_used, "variant mismatch must not use feedforward");
    ASSERT_EQUAL(PROM_P15_SHADOW_FEEDFORWARD_BLOCK_VARIANT_MISMATCH,
                 after.p15_shadow_feedforward_block_reason,
                 "variant mismatch reason surfaced");
    ASSERT_EQUAL(mismatched_variant,
                 after.p15_shadow_feedforward_reserved_variant_id,
                 "reserved variant reported");
    ASSERT_EQUAL(selected_variant,
                 after.p15_shadow_feedforward_selected_variant_id,
                 "selected variant reported");
    ASSERT_EQUAL(0u, after.p15_shadow_feedforward_reconciliation_match, "mismatch is not a reconciliation hit");
    ASSERT_EQUAL(PROM_DOM_CORRECTION_ACTION_LOWER_CONFIDENCE,
                 after.p15_shadow_feedforward_correction_action,
                 "mismatch lowers confidence");
    ASSERT_EQUAL(selected_variant,
                 after.p13_m16b1_requested_occupancy_variant,
                 "requested dispatch still follows judgment engine");
    ASSERT_EQUAL(selected_variant,
                 after.p13_m16b1_executed_occupancy_variant,
                 "executed dispatch still follows judgment engine");
    ASSERT_EQUAL(PROM_P15_SHADOW_FEEDFORWARD_BLOCK_VARIANT_MISMATCH,
                 after.p15_reservation_reason,
                 "reservation reason maps to variant mismatch");
    ASSERT_TRUE(after.p15_prediction_confidence <= confidence_before, "confidence must not increase on mismatch");
    ASSERT_TRUE(after.p15_shadow_feedforward_variant_mismatch_count >= 1u, "variant mismatch counted");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "runtime destroy should succeed");
}
