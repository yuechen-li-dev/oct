#include "../reactor_dominatus_prestage.h"
#include "../reactor_dominatus_predictor.h"
#include "test_harness.h"

static prom_dominatus_prestage_input valid_input() {
    prom_dominatus_prestage_input in{};
    in.valid = 1u;
    in.request_id = 77u;
    in.current_tick = 10u;
    in.target_tick = 12u;
    in.reservation_is_reserved = 1u;
    in.confidence = 0.90;
    in.warmup = 0u;
    in.recent_miss_count = 0u;
    in.runtime_unsafe = 0u;
    in.slot_valid = 1u;
    in.memory_budget_ok = 1u;
    in.outstanding_depth = 1u;
    in.outstanding_depth_cap = 4u;
    in.resource_pressure_low = 1u;
    return in;
}

FACT(PrometheusDominatusPreStage_DefaultParamsSafe)
{
    const auto p = prom_dominatus_prestage_default_params();
    ASSERT_EQUAL(0u, p.action_enabled, "action default-off");
    ASSERT_NEAR(0.75, p.confidence_threshold, 1e-9, "confidence threshold");
    ASSERT_EQUAL(5u, p.recent_miss_window, "recent miss window");
    ASSERT_EQUAL(2u, p.max_lead_ticks, "max lead ticks");
}

FACT(PrometheusDominatusPreStage_AllPassFeatureDisabled)
{
    auto p = prom_dominatus_prestage_default_params();
    const auto in = valid_input();
    const auto d = prom_dominatus_prestage_evaluate(&p, &in);
    ASSERT_EQUAL(1u, d.valid, "valid");
    ASSERT_EQUAL(1u, d.allowed, "allowed even when disabled");
    ASSERT_EQUAL(PROM_DOM_PRESTAGE_ELIGIBLE, d.state, "eligible state");
    ASSERT_EQUAL(0u, d.submitted, "no side effect while disabled");
    ASSERT_TRUE((d.block_reasons & PROM_DOM_PRESTAGE_BLOCK_FEATURE_DISABLED) != 0u, "feature disabled flagged");
}

FACT(PrometheusDominatusPreStage_LowConfidenceBlocks)
{
    auto in = valid_input();
    in.confidence = 0.70;
    const auto d = prom_dominatus_prestage_evaluate(nullptr, &in);
    ASSERT_EQUAL(0u, d.allowed, "blocked");
    ASSERT_TRUE((d.block_reasons & PROM_DOM_PRESTAGE_BLOCK_CONFIDENCE) != 0u, "confidence reason");
}

FACT(PrometheusDominatusPreStage_WarmupBlocks)
{
    auto in = valid_input();
    in.warmup = 1u;
    const auto d = prom_dominatus_prestage_evaluate(nullptr, &in);
    ASSERT_TRUE((d.block_reasons & PROM_DOM_PRESTAGE_BLOCK_WARMUP) != 0u, "warmup reason");
}

FACT(PrometheusDominatusPreStage_ReservationBlocks)
{
    auto in = valid_input();
    in.reservation_is_reserved = 0u;
    const auto d = prom_dominatus_prestage_evaluate(nullptr, &in);
    ASSERT_TRUE((d.block_reasons & PROM_DOM_PRESTAGE_BLOCK_RESERVATION) != 0u, "reservation reason");
}

FACT(PrometheusDominatusPreStage_RecentMissBlocks)
{
    auto in = valid_input();
    in.recent_miss_count = 1u;
    const auto d = prom_dominatus_prestage_evaluate(nullptr, &in);
    ASSERT_TRUE((d.block_reasons & PROM_DOM_PRESTAGE_BLOCK_RECENT_MISS) != 0u, "recent miss reason");
}

FACT(PrometheusDominatusPreStage_HardGateBlocks)
{
    auto in = valid_input();
    in.runtime_unsafe = 1u;
    const auto d = prom_dominatus_prestage_evaluate(nullptr, &in);
    ASSERT_TRUE((d.block_reasons & PROM_DOM_PRESTAGE_BLOCK_HARD_GATE) != 0u, "hard gate reason");
}

FACT(PrometheusDominatusPreStage_ResourcePressureBlocks)
{
    auto in = valid_input();
    in.outstanding_depth = 2u;
    in.outstanding_depth_cap = 2u;
    const auto d = prom_dominatus_prestage_evaluate(nullptr, &in);
    ASSERT_TRUE((d.block_reasons & PROM_DOM_PRESTAGE_BLOCK_RESOURCE_PRESSURE) != 0u, "resource reason");
}

FACT(PrometheusDominatusPreStage_LeadTimeBlocks)
{
    auto in1 = valid_input();
    in1.target_tick = in1.current_tick;
    const auto d1 = prom_dominatus_prestage_evaluate(nullptr, &in1);
    ASSERT_TRUE((d1.block_reasons & PROM_DOM_PRESTAGE_BLOCK_LEAD_TIME) != 0u, "non-future lead blocked");

    auto in2 = valid_input();
    in2.target_tick = in2.current_tick + 3u;
    const auto d2 = prom_dominatus_prestage_evaluate(nullptr, &in2);
    ASSERT_TRUE((d2.block_reasons & PROM_DOM_PRESTAGE_BLOCK_LEAD_TIME) != 0u, "too-far lead blocked");
}

FACT(PrometheusDominatusPreStage_ActionEnabledSubmits)
{
    auto p = prom_dominatus_prestage_default_params();
    p.action_enabled = 1u;
    const auto in = valid_input();
    const auto d = prom_dominatus_prestage_evaluate(&p, &in);
    ASSERT_EQUAL(1u, d.allowed, "allowed");
    ASSERT_EQUAL(1u, d.submitted, "submitted diagnostic state");
    ASSERT_EQUAL(PROM_DOM_PRESTAGE_SUBMITTED, d.state, "state submitted");
}

FACT(PrometheusDominatusPreStage_MultipleReasonsAccumulate)
{
    auto in = valid_input();
    in.confidence = 0.1;
    in.warmup = 1u;
    in.target_tick = in.current_tick;
    const auto d = prom_dominatus_prestage_evaluate(nullptr, &in);
    ASSERT_TRUE((d.block_reasons & PROM_DOM_PRESTAGE_BLOCK_CONFIDENCE) != 0u, "confidence bit");
    ASSERT_TRUE((d.block_reasons & PROM_DOM_PRESTAGE_BLOCK_WARMUP) != 0u, "warmup bit");
    ASSERT_TRUE((d.block_reasons & PROM_DOM_PRESTAGE_BLOCK_LEAD_TIME) != 0u, "lead time bit");
}

FACT(PrometheusDominatusPreStage_NoResourceLeaseMutation)
{
    prom_dominatus_predictor_state s{};
    prom_dominatus_predictor_init(&s, nullptr);
    const auto before_granted = s.future_lease_granted;
    const auto before_requested = s.future_lease_requested;
    const auto before_reserved = s.reservations.reserved_count;
    const auto in = valid_input();
    (void)prom_dominatus_prestage_evaluate(nullptr, &in);
    ASSERT_EQUAL(before_granted, s.future_lease_granted, "no lease grant mutation");
    ASSERT_EQUAL(before_requested, s.future_lease_requested, "no lease request mutation");
    ASSERT_EQUAL(before_reserved, s.reservations.reserved_count, "no reservation mutation");
}
