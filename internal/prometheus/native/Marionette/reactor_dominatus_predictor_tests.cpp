#include "../reactor_dominatus_predictor.h"
#include "test_harness.h"

FACT(PrometheusDominatusPredictor_DefaultParams)
{
    const auto p = prom_dominatus_predictor_default_params();
    ASSERT_EQUAL(0.45, p.confidence_threshold_depth1, "depth1 threshold should match M1");
    ASSERT_EQUAL(0.75, p.confidence_threshold_depth2, "depth2 threshold should match M1");
    ASSERT_EQUAL(2u, p.max_lookahead_depth, "max depth should be 2");
    ASSERT_TRUE(PROM_DOM_PREDICTION_RING_CAP > 0u, "prediction ring cap must be bounded and non-zero");
}

FACT(PrometheusDominatusPredictor_DepthSelection)
{
    prom_dominatus_predictor_state s{};
    prom_dominatus_predictor_init(&s, nullptr);
    prom_dominatus_predictor_evidence e{};
    prom_dominatus_physical_observation p{};
    p.slot_valid = 1u;
    p.memory_budget_ok = 1u;
    p.outstanding_depth_cap = 4u;

    ASSERT_EQUAL(0u, prom_dominatus_predictor_select_depth(&s, &e, &p), "invalid evidence should force depth 0");
    e.valid = 1u; e.warmup = 1u; e.confidence = 0.9;
    ASSERT_EQUAL(0u, prom_dominatus_predictor_select_depth(&s, &e, &p), "warmup should force depth 0");
    e.warmup = 0u; e.confidence = 0.2;
    ASSERT_EQUAL(0u, prom_dominatus_predictor_select_depth(&s, &e, &p), "low confidence should force depth 0");
    e.confidence = 0.5;
    ASSERT_EQUAL(1u, prom_dominatus_predictor_select_depth(&s, &e, &p), "mid confidence should select depth 1");
    e.confidence = 0.9;
    ASSERT_EQUAL(2u, prom_dominatus_predictor_select_depth(&s, &e, &p), "high confidence should select depth 2");
    p.runtime_unsafe = 1u;
    ASSERT_EQUAL(0u, prom_dominatus_predictor_select_depth(&s, &e, &p), "hard gate should force depth 0");
}

FACT(PrometheusDominatusPredictor_IssuePrediction)
{
    prom_dominatus_predictor_state s{};
    prom_dominatus_predictor_init(&s, nullptr);
    prom_dominatus_predictor_evidence e{1u, 0u, 10.0, 10.0, 0.9, 8u, 0u};
    prom_dominatus_physical_observation p{};
    p.slot_valid = 1u; p.memory_budget_ok = 1u; p.outstanding_depth_cap = 4u;
    prom_dominatus_prediction_entry issued{};

    const auto ok = prom_dominatus_predictor_issue(&s, &e, &p, 20u, &issued);
    ASSERT_EQUAL(1u, ok, "high confidence and no gate should issue");
    ASSERT_EQUAL(22u, issued.target_tick, "depth 2 should target tick+2");
    ASSERT_EQUAL(1u, s.ring_count, "ring count should increase");
    ASSERT_EQUAL(1u, issued.active, "issued entry should be active");
    ASSERT_EQUAL(PROM_DOM_FUTURE_LEASE_REQUESTED, issued.future_lease_state, "future lease should be diagnostic requested state");
}

FACT(PrometheusDominatusPredictor_RingCapacity)
{
    prom_dominatus_predictor_state s{};
    prom_dominatus_predictor_init(&s, nullptr);
    prom_dominatus_predictor_evidence e{1u, 0u, 10.0, 10.0, 0.9, 8u, 0u};
    prom_dominatus_physical_observation p{};
    p.slot_valid = 1u; p.memory_budget_ok = 1u; p.outstanding_depth_cap = 64u;

    for (std::uint32_t i = 0; i < PROM_DOM_PREDICTION_RING_CAP; ++i) {
        ASSERT_EQUAL(1u, prom_dominatus_predictor_issue(&s, &e, &p, i + 1u, nullptr), "fill ring should succeed");
    }
    ASSERT_EQUAL(PROM_DOM_PREDICTION_RING_CAP, s.ring_count, "ring should reach capacity");
    ASSERT_EQUAL(0u, prom_dominatus_predictor_issue(&s, &e, &p, 100u, nullptr), "issue past cap should fail safely");
    ASSERT_EQUAL(PROM_DOM_PREDICTION_RING_CAP, s.ring_count, "ring count should remain capped");
    ASSERT_TRUE(s.predictor_stale != 0u || s.fallback_active != 0u, "overflow attempt should set stale/fallback diagnostics");
}

FACT(PrometheusDominatusPredictor_MatureCorrectPrediction)
{
    prom_dominatus_predictor_state s{};
    prom_dominatus_predictor_init(&s, nullptr);
    s.params.max_lookahead_depth = 1u;
    prom_dominatus_predictor_evidence e{1u, 0u, 10.0, 10.0, 0.9, 8u, 0u};
    prom_dominatus_physical_observation p{};
    p.slot_valid = 1u; p.memory_budget_ok = 1u; p.outstanding_depth_cap = 4u;
    ASSERT_EQUAL(1u, prom_dominatus_predictor_issue(&s, &e, &p, 10u, nullptr), "issue should succeed");
    const double before = s.prediction_confidence;
    p.actual_ready = 1u;
    const auto ev = prom_dominatus_predictor_mature(&s, &p, 11u);
    ASSERT_EQUAL(1u, ev.valid, "mature should emit correction event");
    ASSERT_EQUAL(1u, ev.prediction_matured, "prediction should mature");
    ASSERT_EQUAL(0u, ev.state_mismatch, "correct prediction should not mismatch");
    ASSERT_TRUE(s.prediction_confidence >= before, "confidence should not decrease on correct prediction");
    ASSERT_EQUAL(0u, s.ring_count, "matured entry should be removed from ring");
}

FACT(PrometheusDominatusPredictor_MatureMissReducesConfidenceAndDepth)
{
    prom_dominatus_predictor_state s{};
    prom_dominatus_predictor_init(&s, nullptr);
    s.params.max_lookahead_depth = 1u;
    prom_dominatus_predictor_evidence e{1u, 0u, 10.0, 10.0, 0.9, 8u, 0u};
    prom_dominatus_physical_observation p{};
    p.slot_valid = 1u; p.memory_budget_ok = 1u; p.outstanding_depth_cap = 4u;
    ASSERT_EQUAL(1u, prom_dominatus_predictor_issue(&s, &e, &p, 10u, nullptr), "issue should succeed");
    const double before = s.prediction_confidence;
    p.actual_ready = 0u;
    const auto ev = prom_dominatus_predictor_mature(&s, &p, 11u);
    ASSERT_EQUAL(1u, ev.state_mismatch, "miss should mismatch");
    ASSERT_TRUE(ev.action == PROM_DOM_CORRECTION_ACTION_REDUCE_DEPTH || ev.action == PROM_DOM_CORRECTION_ACTION_LOWER_CONFIDENCE,
                "miss should reduce depth or lower confidence");
    ASSERT_TRUE(s.prediction_confidence < before, "confidence should decrease on miss");
    ASSERT_EQUAL(1u, s.correction_count, "correction count should increment");
}

FACT(PrometheusDominatusPredictor_UpdateMaturesBeforeIssue)
{
    prom_dominatus_predictor_state s{};
    prom_dominatus_predictor_init(&s, nullptr);
    s.params.max_lookahead_depth = 1u;
    prom_dominatus_predictor_evidence e{1u, 0u, 10.0, 10.0, 0.9, 8u, 0u};
    prom_dominatus_physical_observation p{};
    p.slot_valid = 1u; p.memory_budget_ok = 1u; p.outstanding_depth_cap = 4u;

    ASSERT_EQUAL(1u, prom_dominatus_predictor_issue(&s, &e, &p, 10u, nullptr), "seed issue should succeed");
    p.actual_ready = 1u;
    prom_dominatus_prediction_entry issued{};
    const auto ev = prom_dominatus_predictor_update(&s, &e, &p, 11u, &issued);
    ASSERT_EQUAL(1u, ev.valid, "update should first mature existing entry");
    ASSERT_EQUAL(1u, issued.active, "update should then issue new entry");
    ASSERT_EQUAL(12u, issued.target_tick, "new issue should be based on current tick after mature");
}

FACT(PrometheusDominatusPredictor_Reset)
{
    prom_dominatus_predictor_state s{};
    prom_dominatus_predictor_init(&s, nullptr);
    prom_dominatus_predictor_evidence e{1u, 0u, 10.0, 10.0, 0.9, 8u, 0u};
    prom_dominatus_physical_observation p{};
    p.slot_valid = 1u; p.memory_budget_ok = 1u; p.outstanding_depth_cap = 4u;
    (void)prom_dominatus_predictor_issue(&s, &e, &p, 1u, nullptr);
    prom_dominatus_predictor_reset(&s);

    ASSERT_EQUAL(0u, s.ring_count, "reset should clear ring count");
    ASSERT_EQUAL(0u, s.initialized, "reset should clear initialized flag");
    ASSERT_EQUAL(0u, s.correction_count, "reset should clear correction counters");
    ASSERT_EQUAL(s.params.confidence_threshold_depth1, s.prediction_confidence, "reset should restore default confidence baseline");
}
