#include "../reactor_dominatus_sgemm_adapter.h"
#include "test_harness.h"

#include <cstdint>

namespace
{
    constexpr std::uint32_t DomainBit(prom_dom_domain domain)
    {
        return std::uint32_t{ 1 } << (static_cast<std::uint32_t>(domain) - 1u);
    }

    prom_buffering_selector_facts MakeFacts()
    {
        prom_buffering_selector_facts facts{};
        facts.memory_budget_slots_permille = 1400u;
        facts.required_fixed_slots_permille = 2000u;
        facts.required_pull_lag_peak_slots_permille = 1500u;
        facts.required_serial_slots_permille = 1000u;
        facts.fixed_double_headroom_slots_permille = -600;
        facts.pull_lag_headroom_slots_permille = -100;
        facts.serial_jit_headroom_slots_permille = 400;
        facts.transfer_variance_class = PROM_VARIANCE_HIGH;
        facts.compute_predictability_class = PROM_PREDICTABILITY_UNSTABLE;
        facts.starvation_risk_high = 1u;
        facts.pull_lag_wip_waste_exceeded = 1u;
        facts.fallback_available = 1u;
        return facts;
    }

    prom_buffering_selector_decision MakeDecision()
    {
        prom_buffering_selector_decision decision{};
        decision.success = 1u;
        decision.selected_mode = PROM_BUFFERING_MODE_SERIAL_JIT_SURVIVAL;
        decision.reason_code = PROM_BUFFERING_REASON_SERIAL_JIT_SELECTED;
        decision.final_reason_code = PROM_BUFFERING_REASON_SERIAL_JIT_SELECTED;
        decision.fixed_double_rejection_reason = PROM_BUFFERING_REASON_FIXED_DOUBLE_MEMORY_INSUFFICIENT;
        decision.pull_lag_rejection_reason = PROM_BUFFERING_REASON_PULL_LAG_MEMORY_INSUFFICIENT;
        decision.serial_jit_rejection_reason = PROM_BUFFERING_REASON_NONE;
        decision.fixed_feasible = 0u;
        decision.pull_lag_feasible = 0u;
        decision.serial_feasible = 1u;
        decision.fixed_rejected = 1u;
        decision.pull_lag_rejected = 1u;
        decision.serial_rejected = 0u;
        decision.fixed_score = -15;
        decision.pull_lag_score = -2;
        decision.serial_score = 58;
        return decision;
    }

    prom_buffering_selector_facts MakeBudgetFacts(std::uint32_t budget, std::int32_t fixed_headroom, std::int32_t pull_headroom, std::int32_t serial_headroom)
    {
        prom_buffering_selector_facts facts{};
        facts.memory_budget_slots_permille = budget;
        facts.required_fixed_slots_permille = 2000u;
        facts.required_pull_lag_peak_slots_permille = 1500u;
        facts.required_serial_slots_permille = 1000u;
        facts.fixed_double_headroom_slots_permille = fixed_headroom;
        facts.pull_lag_headroom_slots_permille = pull_headroom;
        facts.serial_jit_headroom_slots_permille = serial_headroom;
        facts.transfer_variance_class = PROM_VARIANCE_MODERATE;
        facts.compute_predictability_class = PROM_PREDICTABILITY_STABLE;
        facts.starvation_risk_high = 0u;
        facts.pull_lag_wip_waste_exceeded = 0u;
        facts.fallback_available = 1u;
        return facts;
    }

    prom_buffering_selector_decision RunBufferingDecision(const prom_buffering_selector_facts& facts)
    {
        prom_buffering_selector_decision decision{};
        prom_judgment_engine_select_buffering_mode(&facts, &decision);
        return decision;
    }
}

FACT(PrometheusDominatusSgemmAdapter_M35StagedInvisibleUntilCommit)
{
    prom_dom_blackboard board{};
    prom_dom_sgemm_m35_snapshot snapshot{};
    const prom_buffering_selector_facts facts = MakeFacts();
    const prom_buffering_selector_decision decision = MakeDecision();
    prom_dom_blackboard_init(&board);

    ASSERT_TRUE(prom_dom_sgemm_stage_m35(&board, &facts, &decision) == 1u, "staging M35 slice should succeed");
    ASSERT_TRUE(prom_dom_sgemm_read_visible_m35(&board, &snapshot) == 0u,
                "visible snapshot should remain absent before commit");

    prom_dom_sgemm_commit(&board);

    ASSERT_TRUE(prom_dom_sgemm_read_visible_m35(&board, &snapshot) == 1u,
                "visible snapshot should be readable after commit");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_BUFFERING_MODE_SERIAL_JIT_SURVIVAL), snapshot.selected_mode,
                 "selected mode should match committed decision");
    ASSERT_EQUAL(1400u, snapshot.memory_budget_slots_permille, "memory budget should round-trip through blackboard");
}

FACT(PrometheusDominatusSgemmAdapter_M35DirtyTrackingAndSameValueWrites)
{
    prom_dom_blackboard board{};
    const prom_buffering_selector_facts facts = MakeFacts();
    const prom_buffering_selector_decision decision = MakeDecision();
    prom_dom_blackboard_init(&board);

    ASSERT_TRUE(prom_dom_sgemm_stage_m35(&board, &facts, &decision) == 1u, "staging M35 slice should succeed");
    ASSERT_TRUE(prom_dom_dirty_key_staged(&board, PROM_DOM_KEY_SGEMM_BUFFERING_MODE) == 1u,
                "selected mode key should be staged dirty");
    ASSERT_TRUE(prom_dom_dirty_key_staged(&board, PROM_DOM_KEY_MEMORY_M35_SERIAL_HEADROOM) == 1u,
                "memory headroom key should be staged dirty");
    ASSERT_TRUE((prom_dom_dirty_domains_staged(&board) & DomainBit(PROM_DOM_DOMAIN_SGEMM)) != 0u,
                "SGEMM domain should be staged dirty");
    ASSERT_TRUE((prom_dom_dirty_domains_staged(&board) & DomainBit(PROM_DOM_DOMAIN_MEMORY)) != 0u,
                "MEMORY domain should be staged dirty");

    prom_dom_sgemm_commit(&board);

    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_SGEMM_BUFFERING_MODE) == 1u,
                "last-commit dirty mask should include selected mode key");
    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_MEMORY_M35_SERIAL_HEADROOM) == 1u,
                "last-commit dirty mask should include memory headroom key");
    ASSERT_EQUAL(0u, prom_dom_dirty_keys_staged_word(&board, 0u), "staged dirty keys should clear after commit");

    ASSERT_TRUE(prom_dom_sgemm_stage_m35(&board, &facts, &decision) == 1u,
                "same-value staging should still be accepted");
    ASSERT_EQUAL(0u, prom_dom_dirty_keys_staged_word(&board, 0u),
                 "same-value staging should not set staged dirty keys");
}

FACT(PrometheusDominatusSgemmAdapter_M35TraceAndResetBehavior)
{
    prom_dom_blackboard board{};
    prom_dom_sgemm_m35_snapshot snapshot{};
    prom_dom_trace_entry trace{};
    const prom_buffering_selector_facts facts = MakeFacts();
    const prom_buffering_selector_decision decision = MakeDecision();
    prom_dom_blackboard_init(&board);

    ASSERT_TRUE(prom_dom_sgemm_stage_m35(&board, &facts, &decision) == 1u, "staging M35 slice should succeed");
    ASSERT_TRUE(prom_dom_trace_count(&board) > 0u, "staging should emit trace entries");
    ASSERT_TRUE(prom_dom_trace_at(&board, prom_dom_trace_count(&board) - 1u, &trace) == 1u,
                "latest trace entry should be readable");
    ASSERT_EQUAL(PROM_DOM_KEY_MEMORY_M35_SERIAL_HEADROOM, trace.key,
                 "last traced key should match last staged adapter write");

    prom_dom_blackboard_reset(&board);

    ASSERT_EQUAL(0u, prom_dom_trace_count(&board), "reset should clear adapter trace history");
    ASSERT_EQUAL(0u, prom_dom_dirty_domains_staged(&board), "reset should clear staged domains");
    ASSERT_TRUE(prom_dom_sgemm_read_visible_m35(&board, &snapshot) == 0u,
                "reset board should not expose committed M35 snapshot");
}

FACT(PrometheusDominatusSgemmAdapter_M5VisibleProjectionSnapshotIsolationAcrossCommitBoundary)
{
    prom_dom_blackboard board{};
    prom_dom_sgemm_buffering_projection projection{};
    const prom_buffering_selector_facts facts_a = MakeBudgetFacts(2600u, 600, 1100, 1600);
    const prom_buffering_selector_facts facts_b = MakeBudgetFacts(900u, -1100, -600, -100);
    prom_buffering_selector_decision decision{};
    prom_dom_blackboard_init(&board);

    decision = RunBufferingDecision(facts_a);
    ASSERT_TRUE(prom_dom_sgemm_stage_m35(&board, &facts_a, &decision) == 1u, "staging A should succeed");
    prom_dom_sgemm_commit(&board);

    ASSERT_TRUE(prom_dom_sgemm_build_buffering_selector_facts_from_visible(&board, &facts_b, &projection) == 1u,
                "projection build should succeed");
    ASSERT_EQUAL(1u, projection.from_visible_snapshot, "projection should use committed visible snapshot");
    ASSERT_EQUAL(2600u, projection.facts.memory_budget_slots_permille, "pre-commit projection should read visible value A");

    decision = RunBufferingDecision(facts_b);
    ASSERT_TRUE(prom_dom_sgemm_stage_m35(&board, &facts_b, &decision) == 1u, "staging B should succeed");

    ASSERT_TRUE(prom_dom_sgemm_build_buffering_selector_facts_from_visible(&board, &facts_b, &projection) == 1u,
                "projection build before commit should succeed");
    ASSERT_EQUAL(2600u, projection.facts.memory_budget_slots_permille,
                 "staged value B must not affect current visible projection before commit");

    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_build_buffering_selector_facts_from_visible(&board, &facts_a, &projection) == 1u,
                "projection build after commit should succeed");
    ASSERT_EQUAL(900u, projection.facts.memory_budget_slots_permille, "post-commit projection should read visible value B");
}

FACT(PrometheusDominatusSgemmAdapter_M5VisibleProjectionDecisionCompatibility)
{
    prom_dom_blackboard board{};
    prom_dom_sgemm_buffering_projection projection{};
    const prom_buffering_selector_facts facts = MakeBudgetFacts(1800u, -200, 300, 800);
    prom_buffering_selector_decision expected{};
    prom_buffering_selector_decision projected{};
    prom_dom_blackboard_init(&board);

    expected = RunBufferingDecision(facts);
    ASSERT_TRUE(prom_dom_sgemm_stage_m35(&board, &facts, &expected) == 1u, "staging should succeed");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_build_buffering_selector_facts_from_visible(&board, &facts, &projection) == 1u,
                "projection build should succeed");

    prom_judgment_engine_select_buffering_mode(&projection.facts, &projected);
    ASSERT_EQUAL(expected.selected_mode, projected.selected_mode,
                 "projected visible facts should preserve buffering-mode selection for migrated fields");
    ASSERT_EQUAL(expected.final_reason_code, projected.final_reason_code,
                 "projected visible facts should preserve final reason for migrated fields");
}

FACT(PrometheusDominatusSgemmAdapter_M5VisibleProjectionDirtyDependencyMaskAndNoInStepDrift)
{
    prom_dom_blackboard board{};
    prom_dom_sgemm_buffering_projection projection{};
    const prom_buffering_selector_facts facts_a = MakeBudgetFacts(2400u, 400, 900, 1400);
    const prom_buffering_selector_facts facts_b = MakeBudgetFacts(700u, -1300, -800, -300);
    prom_buffering_selector_decision stage_decision{};
    prom_buffering_selector_decision decision_before{};
    prom_buffering_selector_decision decision_during{};
    prom_dom_blackboard_init(&board);

    stage_decision = RunBufferingDecision(facts_a);
    ASSERT_TRUE(prom_dom_sgemm_stage_m35(&board, &facts_a, &stage_decision) == 1u, "initial staging should succeed");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_build_buffering_selector_facts_from_visible(&board, &facts_b, &projection) == 1u,
                "projection build should succeed");
    prom_judgment_engine_select_buffering_mode(&projection.facts, &decision_before);

    stage_decision = RunBufferingDecision(facts_b);
    ASSERT_TRUE(prom_dom_sgemm_stage_m35(&board, &facts_b, &stage_decision) == 1u, "second staging should succeed");
    ASSERT_TRUE(prom_dom_sgemm_build_buffering_selector_facts_from_visible(&board, &facts_b, &projection) == 1u,
                "projection build before second commit should succeed");
    prom_judgment_engine_select_buffering_mode(&projection.facts, &decision_during);
    ASSERT_EQUAL(decision_before.selected_mode, decision_during.selected_mode,
                 "decision must remain pinned to visible snapshot while new staged values exist");

    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_MEMORY_BUDGET) == 1u,
                "last-commit dirty should include memory budget dependency");
    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_MEMORY_M35_FIXED_HEADROOM) == 1u,
                "last-commit dirty should include fixed headroom dependency");
    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_MEMORY_M35_PULL_LAG_HEADROOM) == 1u,
                "last-commit dirty should include pull-lag headroom dependency");
    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_MEMORY_M35_SERIAL_HEADROOM) == 1u,
                "last-commit dirty should include serial headroom dependency");
    ASSERT_TRUE(prom_dom_sgemm_build_buffering_selector_facts_from_visible(&board, &facts_a, &projection) == 1u,
                "projection build after second commit should succeed");
    ASSERT_EQUAL(static_cast<std::uint64_t>(0x0Fu), projection.dependent_dirty_key_mask_last_commit,
                 "projection should report dirty dependency bitmask for migrated M35 visible inputs");
}
