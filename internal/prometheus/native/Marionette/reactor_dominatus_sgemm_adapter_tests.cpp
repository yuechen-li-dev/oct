#include "../reactor_dominatus_sgemm_adapter.h"
#include "../reactor_vulkan_sgemm_internal.h"
#include "test_harness.h"

#include <cstdint>
#include <vulkan/vulkan.h>

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

    prom_buffering_selector_facts MakeFullFacts(std::uint32_t budget,
                                                std::uint32_t required_fixed,
                                                std::uint32_t required_pull_lag,
                                                std::uint32_t required_serial,
                                                prom_variance_class variance,
                                                prom_predictability_class predictability,
                                                std::uint32_t starvation_risk_high,
                                                std::uint32_t fallback_available)
    {
        prom_buffering_selector_facts facts{};
        facts.memory_budget_slots_permille = budget;
        facts.required_fixed_slots_permille = required_fixed;
        facts.required_pull_lag_peak_slots_permille = required_pull_lag;
        facts.required_serial_slots_permille = required_serial;
        facts.fixed_double_headroom_slots_permille = static_cast<std::int32_t>(budget) - static_cast<std::int32_t>(required_fixed);
        facts.pull_lag_headroom_slots_permille = static_cast<std::int32_t>(budget) - static_cast<std::int32_t>(required_pull_lag);
        facts.serial_jit_headroom_slots_permille = static_cast<std::int32_t>(budget) - static_cast<std::int32_t>(required_serial);
        facts.transfer_variance_class = variance;
        facts.compute_predictability_class = predictability;
        facts.starvation_risk_high = starvation_risk_high;
        facts.pull_lag_wip_waste_exceeded = 0u;
        facts.fallback_available = fallback_available;
        return facts;
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
    ASSERT_EQUAL(PROM_DOM_KEY_SGEMM_M35_NO_FEASIBLE_DETAIL, trace.key,
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

FACT(PrometheusDominatusSgemmAdapter_M6FullInputSnapshotIsolation)
{
    prom_dom_blackboard board{};
    prom_dom_sgemm_buffering_projection projection{};
    const prom_buffering_selector_facts facts_a = MakeFullFacts(2600u, 2000u, 1500u, 1000u, PROM_VARIANCE_LOW, PROM_PREDICTABILITY_STABLE, 0u, 1u);
    const prom_buffering_selector_facts facts_b = MakeFullFacts(1200u, 2100u, 1600u, 1500u, PROM_VARIANCE_HIGH, PROM_PREDICTABILITY_UNSTABLE, 1u, 0u);
    prom_dom_blackboard_init(&board);

    ASSERT_TRUE(prom_dom_sgemm_stage_m35_facts(&board, &facts_a) == 1u, "staging full input set A should succeed");
    prom_dom_sgemm_commit(&board);

    ASSERT_TRUE(prom_dom_sgemm_stage_m35_facts(&board, &facts_b) == 1u, "staging full input set B should succeed");
    ASSERT_TRUE(prom_dom_sgemm_build_buffering_selector_facts_from_visible(&board, &facts_b, &projection) == 1u,
                "projection build before commit should succeed");
    ASSERT_EQUAL(facts_a.memory_budget_slots_permille, projection.facts.memory_budget_slots_permille, "pre-commit projection must retain visible A budget");
    ASSERT_EQUAL(facts_a.required_pull_lag_peak_slots_permille, projection.facts.required_pull_lag_peak_slots_permille,
                 "pre-commit projection must retain visible A required pull-lag slots");
    ASSERT_EQUAL(static_cast<std::uint32_t>(facts_a.transfer_variance_class), static_cast<std::uint32_t>(projection.facts.transfer_variance_class),
                 "pre-commit projection must retain visible A variance class");
    ASSERT_EQUAL(facts_a.fallback_available, projection.facts.fallback_available, "pre-commit projection must retain visible A fallback flag");

    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_build_buffering_selector_facts_from_visible(&board, &facts_a, &projection) == 1u,
                "projection build after commit should succeed");
    ASSERT_EQUAL(facts_b.memory_budget_slots_permille, projection.facts.memory_budget_slots_permille, "post-commit projection must read committed B budget");
    ASSERT_EQUAL(facts_b.required_pull_lag_peak_slots_permille, projection.facts.required_pull_lag_peak_slots_permille,
                 "post-commit projection must read committed B required pull-lag slots");
    ASSERT_EQUAL(static_cast<std::uint32_t>(facts_b.transfer_variance_class), static_cast<std::uint32_t>(projection.facts.transfer_variance_class),
                 "post-commit projection must read committed B variance class");
    ASSERT_EQUAL(facts_b.fallback_available, projection.facts.fallback_available, "post-commit projection must read committed B fallback flag");
}

FACT(PrometheusDominatusSgemmAdapter_M6MigratedInputsAffectDecisionOnlyAfterCommit)
{
    prom_dom_blackboard board{};
    prom_dom_sgemm_buffering_projection projection{};
    const prom_buffering_selector_facts base = MakeFullFacts(2600u, 2000u, 1500u, 1000u, PROM_VARIANCE_LOW, PROM_PREDICTABILITY_STABLE, 0u, 1u);
    const prom_buffering_selector_facts memory_change = MakeFullFacts(900u, 2000u, 1500u, 1000u, PROM_VARIANCE_LOW, PROM_PREDICTABILITY_STABLE, 0u, 1u);
    const prom_buffering_selector_facts variance_change = MakeFullFacts(2600u, 2800u, 1500u, 1000u, PROM_VARIANCE_HIGH, PROM_PREDICTABILITY_STABLE, 0u, 1u);
    const prom_buffering_selector_facts predictability_change =
        MakeFullFacts(2600u, 2800u, 1500u, 1000u, PROM_VARIANCE_LOW, PROM_PREDICTABILITY_UNSTABLE, 0u, 1u);
    const prom_buffering_selector_facts starvation_change =
        MakeFullFacts(2600u, 2800u, 1500u, 1000u, PROM_VARIANCE_LOW, PROM_PREDICTABILITY_STABLE, 1u, 1u);
    const prom_buffering_selector_facts fallback_change = MakeFullFacts(1100u, 2200u, 2000u, 1000u, PROM_VARIANCE_LOW, PROM_PREDICTABILITY_STABLE, 0u, 0u);
    const prom_buffering_selector_facts scenarios[] = { memory_change, variance_change, predictability_change, starvation_change, fallback_change };
    prom_buffering_selector_decision base_decision{};
    prom_dom_blackboard_init(&board);

    ASSERT_TRUE(prom_dom_sgemm_stage_m35_facts(&board, &base) == 1u, "baseline facts staging should succeed");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_build_buffering_selector_facts_from_visible(&board, &base, &projection) == 1u,
                "baseline projection should succeed");
    prom_judgment_engine_select_buffering_mode(&projection.facts, &base_decision);

    for (const prom_buffering_selector_facts& scenario : scenarios)
    {
        prom_buffering_selector_decision before_commit{};
        prom_buffering_selector_decision after_commit{};
        prom_buffering_selector_decision expected{};

        ASSERT_TRUE(prom_dom_sgemm_stage_m35_facts(&board, &scenario) == 1u, "scenario facts staging should succeed");
        ASSERT_TRUE(prom_dom_sgemm_build_buffering_selector_facts_from_visible(&board, &scenario, &projection) == 1u,
                    "scenario projection before commit should succeed");
        prom_judgment_engine_select_buffering_mode(&projection.facts, &before_commit);
        ASSERT_EQUAL(base_decision.selected_mode, before_commit.selected_mode, "staged scenario must not affect decision before commit");

        prom_dom_sgemm_commit(&board);
        ASSERT_TRUE(prom_dom_sgemm_build_buffering_selector_facts_from_visible(&board, &base, &projection) == 1u,
                    "scenario projection after commit should succeed");
        prom_judgment_engine_select_buffering_mode(&projection.facts, &after_commit);
        expected = RunBufferingDecision(scenario);
        ASSERT_EQUAL(expected.selected_mode, after_commit.selected_mode, "committed scenario must affect next decision");
        ASSERT_EQUAL(expected.final_reason_code, after_commit.final_reason_code, "committed scenario must affect next decision reason");

        ASSERT_TRUE(prom_dom_sgemm_stage_m35_facts(&board, &base) == 1u, "baseline restage should succeed");
        prom_dom_sgemm_commit(&board);
    }
}

FACT(PrometheusDominatusSgemmAdapter_M6DecisionOutputStagingVisibility)
{
    prom_dom_blackboard board{};
    prom_dom_sgemm_m35_snapshot snapshot{};
    const prom_buffering_selector_facts facts = MakeFullFacts(2000u, 2200u, 1400u, 1000u, PROM_VARIANCE_MODERATE, PROM_PREDICTABILITY_STABLE, 0u, 1u);
    prom_buffering_selector_decision decision_a{};
    prom_buffering_selector_decision decision_b{};
    prom_dom_blackboard_init(&board);

    ASSERT_TRUE(prom_dom_sgemm_stage_m35_facts(&board, &facts) == 1u, "facts staging should succeed");
    prom_dom_sgemm_commit(&board);
    decision_a = RunBufferingDecision(facts);
    decision_b = decision_a;
    decision_b.selected_mode = PROM_BUFFERING_MODE_SERIAL_JIT_SURVIVAL;
    decision_b.final_reason_code = PROM_BUFFERING_REASON_SERIAL_JIT_SELECTED;
    decision_b.reason_code = PROM_BUFFERING_REASON_SERIAL_JIT_SELECTED;
    decision_b.serial_feasible = 1u;
    decision_b.serial_rejected = 0u;

    ASSERT_TRUE(prom_dom_sgemm_stage_m35_decision(&board, &decision_a, 0u) == 1u, "decision A staging should succeed");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_read_visible_m35(&board, &snapshot) == 1u, "decision A snapshot should be visible");
    ASSERT_EQUAL(static_cast<std::uint32_t>(decision_a.selected_mode), snapshot.selected_mode, "decision A must be visible after commit");

    ASSERT_TRUE(prom_dom_sgemm_stage_m35_decision(&board, &decision_b, 777u) == 1u, "decision B staging should succeed");
    ASSERT_TRUE(prom_dom_sgemm_read_visible_m35(&board, &snapshot) == 1u, "visible snapshot should remain readable before commit");
    ASSERT_EQUAL(static_cast<std::uint32_t>(decision_a.selected_mode), snapshot.selected_mode,
                 "staged decision B must not affect visible snapshot before commit");

    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_read_visible_m35(&board, &snapshot) == 1u, "decision B snapshot should be visible after commit");
    ASSERT_EQUAL(static_cast<std::uint32_t>(decision_b.selected_mode), snapshot.selected_mode, "decision B must be visible after commit");
}

FACT(PrometheusDominatusSgemmAdapter_M6DirtyCoverageAndCompatibilityMirrorNoDrift)
{
    prom_dom_blackboard board{};
    prom_dom_sgemm_m35_snapshot visible_a{};
    prom_dom_sgemm_m35_snapshot visible_b{};
    const prom_buffering_selector_facts facts_a = MakeFullFacts(2600u, 2000u, 1500u, 1000u, PROM_VARIANCE_LOW, PROM_PREDICTABILITY_STABLE, 0u, 1u);
    const prom_buffering_selector_facts facts_b = MakeFullFacts(900u, 2200u, 1700u, 1000u, PROM_VARIANCE_HIGH, PROM_PREDICTABILITY_UNSTABLE, 1u, 0u);
    prom_buffering_selector_decision decision_a{};
    prom_buffering_selector_decision decision_b{};
    prom_dom_blackboard_init(&board);

    decision_a = RunBufferingDecision(facts_a);
    ASSERT_TRUE(prom_dom_sgemm_stage_m35(&board, &facts_a, &decision_a) == 1u, "combined stage A should succeed");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_read_visible_m35(&board, &visible_a) == 1u, "visible A should be readable");

    decision_b = RunBufferingDecision(facts_b);
    ASSERT_TRUE(prom_dom_sgemm_stage_m35(&board, &facts_b, &decision_b) == 1u, "combined stage B should succeed");
    ASSERT_TRUE(prom_dom_dirty_key_staged(&board, PROM_DOM_KEY_MEMORY_BUDGET) == 1u, "budget key should be staged dirty");
    ASSERT_TRUE(prom_dom_dirty_key_staged(&board, PROM_DOM_KEY_MEMORY_M35_REQUIRED_FIXED) == 1u, "required fixed key should be staged dirty");
    ASSERT_TRUE(prom_dom_dirty_key_staged(&board, PROM_DOM_KEY_SGEMM_M35_TRANSFER_VARIANCE_CLASS) == 1u, "variance key should be staged dirty");
    ASSERT_TRUE(prom_dom_dirty_key_staged(&board, PROM_DOM_KEY_SGEMM_M35_COMPUTE_PREDICTABILITY_CLASS) == 1u,
                "predictability key should be staged dirty");
    ASSERT_TRUE(prom_dom_dirty_key_staged(&board, PROM_DOM_KEY_SGEMM_M35_STARVATION_RISK_HIGH) == 1u, "starvation key should be staged dirty");
    ASSERT_TRUE(prom_dom_dirty_key_staged(&board, PROM_DOM_KEY_SGEMM_M35_FALLBACK_AVAILABLE) == 1u, "fallback key should be staged dirty");
    ASSERT_TRUE(prom_dom_dirty_key_staged(&board, PROM_DOM_KEY_SGEMM_M35_FIXED_REJECTED) == 1u, "fixed rejected key should be staged dirty");

    ASSERT_TRUE(prom_dom_sgemm_read_visible_m35(&board, &visible_b) == 1u, "visible read before commit should succeed");
    ASSERT_EQUAL(visible_a.selected_mode, visible_b.selected_mode, "compatibility mirror source must stay pinned to old visible state pre-commit");

    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_MEMORY_BUDGET) == 1u, "budget key should be dirty at commit");
    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_MEMORY_M35_REQUIRED_FIXED) == 1u, "required fixed key should be dirty at commit");
    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_SGEMM_M35_TRANSFER_VARIANCE_CLASS) == 1u, "variance key should be dirty at commit");
    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_SGEMM_M35_COMPUTE_PREDICTABILITY_CLASS) == 1u,
                "predictability key should be dirty at commit");
    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_SGEMM_M35_STARVATION_RISK_HIGH) == 1u, "starvation key should be dirty at commit");
    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_SGEMM_M35_FALLBACK_AVAILABLE) == 1u, "fallback key should be dirty at commit");
    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_SGEMM_M35_FIXED_REJECTED) == 1u, "fixed rejected key should be dirty at commit");

    ASSERT_TRUE(prom_dom_sgemm_read_visible_m35(&board, &visible_b) == 1u, "visible read after commit should succeed");
    ASSERT_EQUAL(static_cast<std::uint32_t>(decision_b.selected_mode), visible_b.selected_mode, "committed visible state should update compatibility mirror source");

    ASSERT_TRUE(prom_dom_sgemm_stage_m35(&board, &facts_b, &decision_b) == 1u, "same-value stage should still succeed");
    ASSERT_EQUAL(0u, prom_dom_dirty_keys_staged_word(&board, 0u), "same-value write must not dirty staged keys");
}

namespace
{
    prom_dom_transfer_queue_facts MakeTransferFacts(std::uint32_t dedicated, std::uint32_t differs, std::uint32_t disabled, std::uint32_t large_workload)
    {
        prom_dom_transfer_queue_facts facts{};
        facts.dedicated_transfer_available = dedicated;
        facts.transfer_queue_family_index = dedicated != 0u ? 7u : 3u;
        facts.compute_queue_family_index = 3u;
        facts.queue_families_differ = differs;
        facts.transfer_queue_supported = dedicated;
        facts.transfer_queue_disabled_by_config = disabled;
        facts.transfer_workload_large_enough = large_workload;
        facts.transfer_sync_ownership_supported = dedicated;
        facts.transfer_fallback_available = 1u;
        facts.upload_only_policy_eligible = 1u;
        facts.upload_readback_supported = 0u;
        return facts;
    }

    prom_judgment_decision RunTransferDecision(const prom_dom_transfer_queue_facts& transfer)
    {
        prom_judgment_facts facts{};
        prom_judgment_decision decision{};
        facts.m = 96u;
        facts.n = 96u;
        facts.k = 96u;
        facts.work_units = static_cast<std::uint64_t>(96u) * 96u * 96u;
        facts.can_stage = 1u;
        facts.can_direct = 1u;
        facts.allow_fallback = 1u;
        facts.readback_required = 0u;
        facts.force_staged = 1u;
        facts.fallback_available = 1u;
        facts.transfer_queue_dedicated_available = transfer.dedicated_transfer_available;
        facts.transfer_queue_families_differ = transfer.queue_families_differ;
        facts.transfer_queue_supported = transfer.transfer_queue_supported;
        facts.transfer_overlap_slot_valid = transfer.transfer_sync_ownership_supported;
        facts.transfer_workload_large_enough = transfer.transfer_workload_large_enough;
        facts.transfer_fallback_available = transfer.transfer_fallback_available;
        facts.transfer_queue_disabled_by_config = transfer.transfer_queue_disabled_by_config;
        prom_judgment_engine_select_sgemm_mode(&facts, &decision);
        return decision;
    }
}

FACT(PrometheusDominatusSgemmAdapter_M7TransferInputSnapshotIsolation)
{
    prom_dom_blackboard board{};
    prom_dom_transfer_queue_projection projection{};
    const prom_dom_transfer_queue_facts facts_a = MakeTransferFacts(1u, 1u, 0u, 1u);
    const prom_dom_transfer_queue_facts facts_b = MakeTransferFacts(0u, 0u, 1u, 0u);
    prom_dom_blackboard_init(&board);

    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_queue_facts(&board, &facts_a) == 1u, "staging transfer facts A should succeed");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_queue_facts(&board, &facts_b) == 1u, "staging transfer facts B should succeed");
    ASSERT_TRUE(prom_dom_sgemm_build_transfer_queue_facts_from_visible(&board, &facts_b, &projection) == 1u,
                "projection build before commit should succeed");
    ASSERT_EQUAL(facts_a.dedicated_transfer_available, projection.facts.dedicated_transfer_available,
                 "projection before commit must remain pinned to visible A");
    ASSERT_EQUAL(facts_a.transfer_queue_disabled_by_config, projection.facts.transfer_queue_disabled_by_config,
                 "projection before commit must keep visible config gate");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_build_transfer_queue_facts_from_visible(&board, &facts_a, &projection) == 1u,
                "projection build after commit should succeed");
    ASSERT_EQUAL(facts_b.dedicated_transfer_available, projection.facts.dedicated_transfer_available,
                 "projection after commit must read committed B facts");
}

FACT(PrometheusDominatusSgemmAdapter_M7TransferInputsAffectDecisionOnlyAfterCommit)
{
    prom_dom_blackboard board{};
    prom_dom_transfer_queue_projection projection{};
    const prom_dom_transfer_queue_facts base = MakeTransferFacts(1u, 1u, 0u, 1u);
    const prom_dom_transfer_queue_facts disabled = MakeTransferFacts(1u, 1u, 1u, 1u);
    prom_judgment_decision before{};
    prom_judgment_decision during{};
    prom_judgment_decision after{};
    prom_dom_blackboard_init(&board);

    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_queue_facts(&board, &base) == 1u, "baseline transfer facts should stage");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_build_transfer_queue_facts_from_visible(&board, &base, &projection) == 1u,
                "baseline projection should succeed");
    before = RunTransferDecision(projection.facts);

    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_queue_facts(&board, &disabled) == 1u, "changed transfer facts should stage");
    ASSERT_TRUE(prom_dom_sgemm_build_transfer_queue_facts_from_visible(&board, &disabled, &projection) == 1u,
                "projection before commit should succeed");
    during = RunTransferDecision(projection.facts);
    ASSERT_EQUAL(before.use_dedicated_transfer_queue_upload, during.use_dedicated_transfer_queue_upload,
                 "staged transfer fact change must not affect decision before commit");

    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_build_transfer_queue_facts_from_visible(&board, &base, &projection) == 1u,
                "projection after commit should succeed");
    after = RunTransferDecision(projection.facts);
    ASSERT_EQUAL(0u, after.use_dedicated_transfer_queue_upload, "committed disabled transfer fact must affect next decision");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_TRANSFER_FALLBACK_DISABLED_BY_CONFIG), after.transfer_fallback_reason,
                 "committed transfer fact must update fallback reason");
}

FACT(PrometheusDominatusSgemmAdapter_M7TransferDecisionOutputStagingVisibility)
{
    prom_dom_blackboard board{};
    prom_dom_transfer_queue_snapshot snapshot{};
    const prom_dom_transfer_queue_facts facts = MakeTransferFacts(1u, 1u, 0u, 1u);
    prom_dom_transfer_queue_decision decision_a{};
    prom_dom_transfer_queue_decision decision_b{};
    prom_dom_blackboard_init(&board);
    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_queue_facts(&board, &facts) == 1u, "facts stage should succeed");
    prom_dom_sgemm_commit(&board);

    decision_a.transfer_policy_selected = 1u;
    decision_a.selected_transfer_policy = 1u;
    decision_a.transfer_queue_used = 1u;
    decision_a.transfer_fallback_reason = PROM_TRANSFER_FALLBACK_NONE;
    decision_b = decision_a;
    decision_b.transfer_queue_used = 0u;
    decision_b.transfer_fallback_reason = PROM_TRANSFER_FALLBACK_REQUIRED;

    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_queue_decision(&board, &decision_a) == 1u, "decision A staging should succeed");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_read_visible_transfer_queue_diagnostics(&board, &snapshot) == 1u, "visible decision A should read");
    ASSERT_EQUAL(decision_a.transfer_queue_used, snapshot.transfer_queue_used, "decision A should be visible after commit");

    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_queue_decision(&board, &decision_b) == 1u, "decision B staging should succeed");
    ASSERT_TRUE(prom_dom_sgemm_read_visible_transfer_queue_diagnostics(&board, &snapshot) == 1u, "pre-commit visible read should succeed");
    ASSERT_EQUAL(decision_a.transfer_queue_used, snapshot.transfer_queue_used, "staged decision B must not affect visible state");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_read_visible_transfer_queue_diagnostics(&board, &snapshot) == 1u, "post-commit visible read should succeed");
    ASSERT_EQUAL(decision_b.transfer_queue_used, snapshot.transfer_queue_used, "decision B must be visible after commit");
}

FACT(PrometheusDominatusSgemmAdapter_M7TransferDirtyCoverageAndMirrorNoDrift)
{
    prom_dom_blackboard board{};
    prom_dom_transfer_queue_snapshot visible_a{};
    prom_dom_transfer_queue_snapshot visible_b{};
    const prom_dom_transfer_queue_facts facts_a = MakeTransferFacts(1u, 1u, 0u, 1u);
    const prom_dom_transfer_queue_facts facts_b = MakeTransferFacts(0u, 0u, 1u, 0u);
    prom_dom_transfer_queue_decision decision{};
    prom_dom_blackboard_init(&board);

    decision.transfer_policy_selected = 1u;
    decision.selected_transfer_policy = 1u;
    decision.transfer_queue_used = 1u;
    decision.transfer_fallback_reason = PROM_TRANSFER_FALLBACK_NONE;
    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_queue_facts(&board, &facts_a) == 1u, "initial transfer facts stage should succeed");
    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_queue_decision(&board, &decision) == 1u, "initial transfer decision stage should succeed");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_read_visible_transfer_queue_diagnostics(&board, &visible_a) == 1u, "visible A should be readable");

    decision.transfer_policy_selected = 0u;
    decision.selected_transfer_policy = 0u;
    decision.transfer_queue_used = 0u;
    decision.transfer_fallback_reason = PROM_TRANSFER_FALLBACK_DISABLED_BY_CONFIG;
    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_queue_facts(&board, &facts_b) == 1u, "mutated transfer facts stage should succeed");
    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_queue_decision(&board, &decision) == 1u, "mutated transfer decision stage should succeed");
    ASSERT_TRUE(prom_dom_dirty_key_staged(&board, PROM_DOM_KEY_QUEUE_DEDICATED_AVAILABLE) == 1u, "dedicated key should be staged dirty");
    ASSERT_TRUE(prom_dom_dirty_key_staged(&board, PROM_DOM_KEY_QUEUE_TRANSFER_DISABLED_BY_CONFIG) == 1u, "disabled key should be staged dirty");
    ASSERT_TRUE(prom_dom_dirty_key_staged(&board, PROM_DOM_KEY_QUEUE_TRANSFER_POLICY_SELECTED) == 1u, "policy-selected key should be staged dirty");
    ASSERT_TRUE(prom_dom_dirty_key_staged(&board, PROM_DOM_KEY_QUEUE_TRANSFER_FALLBACK_REASON) == 1u, "fallback key should be staged dirty");

    ASSERT_TRUE(prom_dom_sgemm_read_visible_transfer_queue_diagnostics(&board, &visible_b) == 1u, "visible pre-commit read should succeed");
    ASSERT_EQUAL(visible_a.transfer_queue_used, visible_b.transfer_queue_used, "mirror must stay pinned to visible pre-commit state");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_QUEUE_DEDICATED_AVAILABLE) == 1u, "dedicated key should be dirty at commit");
    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_QUEUE_TRANSFER_POLICY_SELECTED) == 1u,
                "policy-selected key should be dirty at commit");
    ASSERT_TRUE(prom_dom_sgemm_read_visible_transfer_queue_diagnostics(&board, &visible_b) == 1u, "visible post-commit read should succeed");
    ASSERT_EQUAL(decision.transfer_queue_used, visible_b.transfer_queue_used, "visible state should update after commit");

    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_queue_facts(&board, &facts_b) == 1u, "same-value facts stage should succeed");
    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_queue_decision(&board, &decision) == 1u, "same-value decision stage should succeed");
    ASSERT_EQUAL(0u, prom_dom_dirty_keys_staged_word(&board, 0u), "same-value writes must not dirty staged keys");
}

FACT(PrometheusDominatusSgemmAdapter_M8TransferTelemetryHandoffWaitStagedVisibility)
{
    prom_dom_blackboard board{};
    prom_dom_transfer_runtime_telemetry telemetry{};
    prom_dom_blackboard_init(&board);

    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_handoff(&board, 4u, 0u, 0) == 1u, "initial handoff should stage");
    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_wait(&board, 2u, 0u, 0) == 1u, "initial wait should stage");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_read_visible_transfer_runtime_telemetry(&board, &telemetry) == 1u, "visible telemetry should read");
    ASSERT_EQUAL(4u, telemetry.queue_family_handoff_count, "initial handoff should be visible");
    ASSERT_EQUAL(2u, telemetry.transfer_compute_wait_count, "initial wait should be visible");

    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_handoff(&board, 8u, 0u, 0) == 1u, "updated handoff should stage");
    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_wait(&board, 5u, 0u, 0) == 1u, "updated wait should stage");
    ASSERT_TRUE(prom_dom_sgemm_read_visible_transfer_runtime_telemetry(&board, &telemetry) == 1u, "pre-commit visible telemetry should read");
    ASSERT_EQUAL(4u, telemetry.queue_family_handoff_count, "staged handoff must remain invisible before commit");
    ASSERT_EQUAL(2u, telemetry.transfer_compute_wait_count, "staged wait must remain invisible before commit");

    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_read_visible_transfer_runtime_telemetry(&board, &telemetry) == 1u, "post-commit visible telemetry should read");
    ASSERT_EQUAL(8u, telemetry.queue_family_handoff_count, "committed handoff should become visible");
    ASSERT_EQUAL(5u, telemetry.transfer_compute_wait_count, "committed wait should become visible");
}

FACT(PrometheusDominatusSgemmAdapter_M8TransferFailureTelemetryTracksDirtyAndEventMetadata)
{
    prom_dom_blackboard board{};
    prom_dom_transfer_runtime_telemetry telemetry{};
    prom_dom_event event{};
    prom_dom_trace_entry trace{};
    prom_dom_blackboard_init(&board);
    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_handoff(&board, 0u, 0u, 0) == 1u, "baseline handoff should stage");
    prom_dom_sgemm_commit(&board);

    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_failure(&board, 1, VK_ERROR_DEVICE_LOST, 1u) == 1u, "failure telemetry should stage");
    ASSERT_TRUE(prom_dom_dirty_key_staged(&board, PROM_DOM_KEY_QUEUE_TRANSFER_FAILURE_REASON) == 1u,
                "failure reason key should be dirty while staged");
    ASSERT_TRUE(prom_dom_sgemm_read_visible_transfer_runtime_telemetry(&board, &telemetry) == 1u,
                "pre-commit visible telemetry read should still succeed from baseline");
    ASSERT_EQUAL(-1, telemetry.transfer_failure_slot_id, "staged failure slot id must remain invisible before commit");
    ASSERT_EQUAL(0, telemetry.transfer_failure_reason, "staged failure reason must remain invisible before commit");

    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_read_visible_transfer_runtime_telemetry(&board, &telemetry) == 1u,
                "failure telemetry should be visible after commit");
    ASSERT_EQUAL(1, telemetry.transfer_failure_slot_id, "failure slot id should be visible after commit");
    ASSERT_EQUAL(static_cast<std::int32_t>(VK_ERROR_DEVICE_LOST), telemetry.transfer_failure_reason,
                 "failure reason should be visible after commit");
    ASSERT_EQUAL(1u, telemetry.transfer_failure_count, "failure count should be visible after commit");
    ASSERT_TRUE((prom_dom_dirty_domains_last_commit(&board) & DomainBit(PROM_DOM_DOMAIN_QUEUE)) != 0u,
                "queue domain should be dirty at commit");
    ASSERT_TRUE(prom_dom_committed_event_count(&board) > 0u, "failure staging should emit a committed event");
    ASSERT_TRUE(prom_dom_committed_event_at(&board, prom_dom_committed_event_count(&board) - 1u, &event) == 1u,
                "latest committed event should be readable");
    ASSERT_EQUAL(PROM_DOM_EVENT_TRANSFER_FAILED, event.kind, "latest committed event should be transfer failed");
    ASSERT_EQUAL(1u, event.slot_id, "failed event should carry slot metadata");
    ASSERT_EQUAL(static_cast<std::int32_t>(VK_ERROR_DEVICE_LOST), event.reason_code, "failed event should carry reason metadata");
    ASSERT_TRUE(prom_dom_trace_count(&board) > 0u, "failure staging should emit trace entries");
    ASSERT_TRUE(prom_dom_trace_at(&board, prom_dom_trace_count(&board) - 1u, &trace) == 1u, "latest trace should be readable");
    ASSERT_EQUAL(PROM_DOM_KEY_QUEUE_TRANSFER_FAILURE_REASON, trace.key, "last trace key should track failure event metadata");
}

FACT(PrometheusDominatusSgemmAdapter_M8TransferAsyncCompletionCommitBoundary)
{
    prom_dom_blackboard board{};
    prom_dom_transfer_runtime_telemetry telemetry{};
    prom_dom_blackboard_init(&board);
    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_handoff(&board, 0u, 0u, 0) == 1u, "baseline handoff should stage");
    prom_dom_sgemm_commit(&board);

    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_complete(&board, 0u, 10u, 0u, 0) == 1u, "stage incomplete marker should succeed");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_read_visible_transfer_runtime_telemetry(&board, &telemetry) == 1u, "visible telemetry should read");
    ASSERT_EQUAL(0u, telemetry.async_transfer_complete, "incomplete marker should be visible after commit");
    ASSERT_EQUAL(10u, telemetry.async_transfer_completion_generation, "completion generation should be visible after commit");

    ASSERT_TRUE(prom_dom_sgemm_stage_transfer_complete(&board, 1u, 11u, 0u, 0) == 1u, "stage completion marker should succeed");
    ASSERT_TRUE(prom_dom_sgemm_read_visible_transfer_runtime_telemetry(&board, &telemetry) == 1u, "pre-commit visible telemetry should read");
    ASSERT_EQUAL(0u, telemetry.async_transfer_complete, "staged completion marker must remain invisible before commit");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_read_visible_transfer_runtime_telemetry(&board, &telemetry) == 1u, "post-commit visible telemetry should read");
    ASSERT_EQUAL(1u, telemetry.async_transfer_complete, "completion marker should become visible after commit");
    ASSERT_EQUAL(11u, telemetry.async_transfer_completion_generation, "completion generation should advance after commit");
}

FACT(PrometheusDominatusSgemmAdapter_M9LayoutPrecisionInputSnapshotIsolation)
{
    prom_dom_blackboard board{};
    prom_dom_sgemm_layout_precision_facts a{};
    prom_dom_sgemm_layout_precision_facts b{};
    prom_dom_sgemm_layout_precision_projection projection{};
    prom_dom_blackboard_init(&board);

    a.packed4_available = 1u; a.packed4_small_shape = 0u; a.packed4_padding_waste_permille = 30u; a.packed4_mode_budget_permille = 120u;
    a.packed4_row_major_valid = 1u; a.packed4_tail_valid = 1u; a.strict_fp32 = 0u; a.tolerance_known = 1u; a.tolerance_pass = 1u;
    a.has_special_values = 0u; a.capability_fp16_storage = 1u; a.fallback_available = 1u; a.fp16_utility_score = 700;
    b = a; b.packed4_padding_waste_permille = 410u; b.strict_fp32 = 1u; b.fp16_utility_score = -22;

    ASSERT_TRUE(prom_dom_sgemm_stage_layout_precision_facts(&board, &a) == 1u, "stage A facts should succeed");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_stage_layout_precision_facts(&board, &b) == 1u, "stage B facts should succeed");
    ASSERT_TRUE(prom_dom_sgemm_build_layout_precision_facts_from_visible(&board, &b, &projection) == 1u, "visible projection should build");
    ASSERT_EQUAL(30u, projection.facts.packed4_padding_waste_permille, "projection before commit should retain A");
    ASSERT_EQUAL(0u, projection.facts.strict_fp32, "projection before commit should retain A strict flag");

    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_build_layout_precision_facts_from_visible(&board, &a, &projection) == 1u, "projection after commit should build");
    ASSERT_EQUAL(410u, projection.facts.packed4_padding_waste_permille, "projection after commit should read B");
    ASSERT_EQUAL(1u, projection.facts.strict_fp32, "projection after commit should read B strict flag");
}

FACT(PrometheusDominatusSgemmAdapter_M9LayoutPrecisionDecisionStagingVisibilityAndDirty)
{
    prom_dom_blackboard board{};
    prom_dom_sgemm_layout_precision_facts facts{};
    prom_dom_sgemm_layout_precision_decision decision{};
    prom_dom_sgemm_layout_precision_snapshot snapshot{};
    prom_dom_blackboard_init(&board);
    facts.packed4_available = 1u; facts.packed4_tail_valid = 1u; facts.capability_fp16_storage = 1u; facts.fallback_available = 1u;
    ASSERT_TRUE(prom_dom_sgemm_stage_layout_precision_facts(&board, &facts) == 1u, "fact stage should succeed");
    prom_dom_sgemm_commit(&board);

    decision.packed4_selected = 1u;
    decision.packed4_reject_reason = PROM_PACKED4_REJECT_NONE;
    decision.fp16_selected = 0u;
    decision.fp16_reject_reason = PROM_FP16_REJECT_TOLERANCE_EXCEEDED;
    decision.packed4_selected_layout_format = 2u;
    decision.packed4_tail_count_last = 3u;
    decision.packed4_tail_count_total = 7u;
    decision.packed4_padded_lane_count_last = 8u;
    decision.packed4_padded_lane_count_total = 22u;
    decision.packed4_padding_waste_permille_last = 120u;
    decision.fp16_selected_candidate = 1u;
    decision.fp16_fallback_reason_detail = PROM_DETAIL_FP16_TOLERANCE_EXCEEDED;
    ASSERT_TRUE(prom_dom_sgemm_stage_layout_precision_decision(&board, &decision) == 1u, "decision stage should succeed");
    ASSERT_TRUE(prom_dom_sgemm_read_visible_layout_precision_diagnostics(&board, &snapshot) == 0u,
                "pre-commit visible snapshot should remain unavailable for staged decision");
    ASSERT_TRUE(prom_dom_dirty_key_staged(&board, PROM_DOM_KEY_SGEMM_PACKED4_SELECTED) == 1u, "packed4 decision key should be dirty");
    ASSERT_TRUE(prom_dom_dirty_key_staged(&board, PROM_DOM_KEY_DIAGNOSTICS_FP16_SELECTED_CANDIDATE) == 1u, "fp16 diagnostics key should be dirty");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_read_visible_layout_precision_diagnostics(&board, &snapshot) == 1u, "post-commit snapshot should be visible");
    ASSERT_EQUAL(1u, snapshot.decision.packed4_selected, "visible decision should match commit");
    ASSERT_EQUAL(2u, snapshot.decision.packed4_selected_layout_format, "visible packed4 layout format should match commit");
    ASSERT_EQUAL(1u, prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_SGEMM_PACKED4_SELECTED), "last-commit dirty should include packed4 selected");
}

FACT(PrometheusDominatusSgemmAdapter_M10PathComputeInputSnapshotIsolation)
{
    prom_dom_blackboard board{};
    prom_dom_sgemm_path_compute_facts a{};
    prom_dom_sgemm_path_compute_facts b{};
    prom_dom_sgemm_path_compute_projection projection{};
    prom_dom_blackboard_init(&board);
    a.m = 128u; a.n = 128u; a.k = 64u; a.work_units = 1048576u; a.can_stage = 1u; a.can_direct = 1u;
    a.allow_fallback = 1u; a.readback_required = 1u; a.force_direct = 0u; a.force_direct_reason = PROM_SGEMM_FORCE_DIRECT_REASON_NONE; a.force_staged = 0u; a.force_tiled = 0u;
    a.tiled_shape = 1u; a.software_vulkan = 0u; a.policy_mode = PROM_POLICY_MODE_AGGRESSIVE;
    b = a;
    b.m = 32u; b.work_units = 32768u; b.can_stage = 0u; b.force_direct = 1u; b.force_direct_reason = PROM_SGEMM_FORCE_DIRECT_REASON_EXPLICIT_OVERRIDE; b.tiled_shape = 0u; b.policy_mode = PROM_POLICY_MODE_SAFE;

    ASSERT_TRUE(prom_dom_sgemm_stage_path_compute_facts(&board, &a) == 1u, "staging facts A should succeed");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_stage_path_compute_facts(&board, &b) == 1u, "staging facts B should succeed");
    ASSERT_TRUE(prom_dom_sgemm_build_path_compute_facts_from_visible(&board, &b, &projection) == 1u, "pre-commit projection should build");
    ASSERT_EQUAL(a.m, projection.facts.m, "pre-commit projection should preserve visible A");
    ASSERT_EQUAL(a.force_direct, projection.facts.force_direct, "pre-commit projection should preserve visible A force-direct");
    ASSERT_EQUAL(a.force_direct_reason, projection.facts.force_direct_reason, "pre-commit projection should preserve visible A force-direct reason");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_build_path_compute_facts_from_visible(&board, &a, &projection) == 1u, "post-commit projection should build");
    ASSERT_EQUAL(b.m, projection.facts.m, "post-commit projection should read committed B");
    ASSERT_EQUAL(b.force_direct, projection.facts.force_direct, "post-commit projection should read committed B force-direct");
    ASSERT_EQUAL(b.force_direct_reason, projection.facts.force_direct_reason, "post-commit projection should read committed B force-direct reason");
}

FACT(PrometheusDominatusSgemmAdapter_M10PathComputeFactsAffectDecisionOnlyAfterCommit)
{
    prom_dom_blackboard board{};
    prom_dom_sgemm_path_compute_facts base{};
    prom_dom_sgemm_path_compute_facts changed{};
    prom_dom_sgemm_path_compute_projection projection{};
    prom_judgment_facts facts{};
    prom_judgment_decision before{};
    prom_judgment_decision during{};
    prom_judgment_decision after{};
    prom_dom_blackboard_init(&board);
    base.m = 128u; base.n = 128u; base.k = 64u; base.work_units = 1048576u; base.can_stage = 1u; base.can_direct = 1u;
    base.allow_fallback = 1u; base.readback_required = 1u; base.force_direct = 0u; base.force_direct_reason = PROM_SGEMM_FORCE_DIRECT_REASON_NONE; base.force_staged = 0u; base.force_tiled = 1u;
    base.tiled_shape = 1u; base.software_vulkan = 0u; base.policy_mode = PROM_POLICY_MODE_AGGRESSIVE;
    changed = base; changed.force_direct = 1u; changed.force_direct_reason = PROM_SGEMM_FORCE_DIRECT_REASON_EXPLICIT_OVERRIDE; changed.force_staged = 0u; changed.force_tiled = 0u; changed.tiled_shape = 0u;
    changed.readback_required = 0u; changed.can_stage = 0u; changed.software_vulkan = 1u;

    ASSERT_TRUE(prom_dom_sgemm_stage_path_compute_facts(&board, &base) == 1u, "baseline facts should stage");
    prom_dom_sgemm_commit(&board);

    ASSERT_TRUE(prom_dom_sgemm_build_path_compute_facts_from_visible(&board, &base, &projection) == 1u, "baseline projection should build");
    facts.can_stage = projection.facts.can_stage; facts.can_direct = projection.facts.can_direct; facts.allow_fallback = projection.facts.allow_fallback;
    facts.readback_required = projection.facts.readback_required; facts.force_direct = projection.facts.force_direct;
    facts.force_staged = projection.facts.force_staged; facts.force_tiled = projection.facts.force_tiled; facts.tiled_shape = projection.facts.tiled_shape;
    facts.software_vulkan = projection.facts.software_vulkan; facts.policy_mode = static_cast<prom_policy_mode>(projection.facts.policy_mode);
    facts.work_units = projection.facts.work_units; facts.fallback_available = 1u; facts.transfer_fallback_available = 1u;
    facts.transfer_queue_supported = 1u; facts.transfer_overlap_slot_valid = 1u; facts.transfer_workload_large_enough = 1u;
    facts.transfer_queue_dedicated_available = 1u; facts.transfer_queue_families_differ = 1u;
    facts.packed4_available = 0u; facts.capability_fp16_storage = 0u;
    prom_judgment_engine_select_sgemm_mode(&facts, &before);

    ASSERT_TRUE(prom_dom_sgemm_stage_path_compute_facts(&board, &changed) == 1u, "changed facts should stage");
    ASSERT_TRUE(prom_dom_sgemm_build_path_compute_facts_from_visible(&board, &changed, &projection) == 1u, "pre-commit projection should build");
    facts.can_stage = projection.facts.can_stage; facts.readback_required = projection.facts.readback_required; facts.force_direct = projection.facts.force_direct;
    facts.force_tiled = projection.facts.force_tiled; facts.tiled_shape = projection.facts.tiled_shape; facts.software_vulkan = projection.facts.software_vulkan;
    prom_judgment_engine_select_sgemm_mode(&facts, &during);
    ASSERT_EQUAL(before.selected_path, during.selected_path, "staged path/compute mutations must not affect decision before commit");
    ASSERT_EQUAL(before.compute_mode, during.compute_mode, "staged path/compute mutations must not affect compute mode before commit");

    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_build_path_compute_facts_from_visible(&board, &base, &projection) == 1u, "post-commit projection should build");
    facts.can_stage = projection.facts.can_stage; facts.readback_required = projection.facts.readback_required; facts.force_direct = projection.facts.force_direct;
    facts.force_tiled = projection.facts.force_tiled; facts.tiled_shape = projection.facts.tiled_shape; facts.software_vulkan = projection.facts.software_vulkan;
    prom_judgment_engine_select_sgemm_mode(&facts, &after);
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_VK_PATH_DIRECT), static_cast<std::uint32_t>(after.selected_path), "committed force-direct should affect next decision");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_VK_COMPUTE_BASELINE), static_cast<std::uint32_t>(after.compute_mode), "committed tiled/path flags should affect next decision");
}

FACT(PrometheusDominatusSgemmAdapter_M10PathComputeDecisionOutputStaging)
{
    prom_dom_blackboard board{};
    prom_dom_sgemm_path_compute_facts facts{};
    prom_dom_sgemm_path_compute_decision decision_a{};
    prom_dom_sgemm_path_compute_decision decision_b{};
    prom_dom_sgemm_path_compute_snapshot snapshot{};
    prom_dom_blackboard_init(&board);
    facts.m = 64u; facts.n = 64u; facts.k = 64u; facts.work_units = 262144u; facts.can_stage = 1u; facts.can_direct = 1u;
    facts.allow_fallback = 1u; facts.readback_required = 1u; facts.policy_mode = PROM_POLICY_MODE_AGGRESSIVE;
    ASSERT_TRUE(prom_dom_sgemm_stage_path_compute_facts(&board, &facts) == 1u, "facts stage should succeed");
    prom_dom_sgemm_commit(&board);
    decision_a.success = 1u; decision_a.requested_path = PROM_VK_PATH_STAGED_UPLOAD_READBACK; decision_a.selected_path = PROM_VK_PATH_STAGED_UPLOAD_READBACK;
    decision_a.compute_mode = PROM_VK_COMPUTE_TILED; decision_a.final_detail = PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK_TILED;
    decision_a.winning_candidate_index = 5u; decision_a.winning_score = 530;
    decision_b = decision_a; decision_b.selected_path = PROM_VK_PATH_DIRECT; decision_b.compute_mode = PROM_VK_COMPUTE_BASELINE; decision_b.winning_score = 205;
    ASSERT_TRUE(prom_dom_sgemm_stage_path_compute_decision(&board, &decision_a) == 1u, "decision A stage should succeed");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_read_visible_path_compute_diagnostics(&board, &snapshot) == 1u, "decision A should be visible after commit");
    ASSERT_EQUAL(decision_a.winning_score, snapshot.decision.winning_score, "decision A winning score should round-trip");
    ASSERT_TRUE(prom_dom_sgemm_stage_path_compute_decision(&board, &decision_b) == 1u, "decision B stage should succeed");
    ASSERT_TRUE(prom_dom_sgemm_read_visible_path_compute_diagnostics(&board, &snapshot) == 1u, "pre-commit visible read should succeed");
    ASSERT_EQUAL(decision_a.winning_score, snapshot.decision.winning_score, "staged decision B must remain invisible before commit");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_read_visible_path_compute_diagnostics(&board, &snapshot) == 1u, "decision B should be visible after commit");
    ASSERT_EQUAL(decision_b.winning_score, snapshot.decision.winning_score, "decision B should be visible after commit");
}

FACT(PrometheusDominatusSgemmAdapter_M10PathComputeDirtyCoverage)
{
    prom_dom_blackboard board{};
    prom_dom_sgemm_path_compute_facts a{};
    prom_dom_sgemm_path_compute_facts b{};
    prom_dom_sgemm_path_compute_decision decision{};
    prom_dom_sgemm_path_compute_projection projection{};
    prom_dom_blackboard_init(&board);
    a.m = 64u; a.n = 64u; a.k = 64u; a.work_units = 262144u; a.can_stage = 1u; a.can_direct = 1u; a.allow_fallback = 1u;
    a.force_direct_reason = PROM_SGEMM_FORCE_DIRECT_REASON_NONE;
    b = a; b.force_direct = 1u; b.force_direct_reason = PROM_SGEMM_FORCE_DIRECT_REASON_EXPLICIT_OVERRIDE; b.readback_required = 1u; b.tiled_shape = 1u; b.software_vulkan = 1u;
    ASSERT_TRUE(prom_dom_sgemm_stage_path_compute_facts(&board, &a) == 1u, "initial facts should stage");
    prom_dom_sgemm_commit(&board);
    decision.success = 1u; decision.requested_path = PROM_VK_PATH_DIRECT; decision.selected_path = PROM_VK_PATH_DIRECT;
    decision.compute_mode = PROM_VK_COMPUTE_BASELINE; decision.final_detail = PROM_DETAIL_PATH_DIRECT; decision.winning_score = 25;
    ASSERT_TRUE(prom_dom_sgemm_stage_path_compute_facts(&board, &b) == 1u, "mutated facts should stage");
    ASSERT_TRUE(prom_dom_sgemm_stage_path_compute_decision(&board, &decision) == 1u, "decision should stage");
    ASSERT_TRUE(prom_dom_dirty_key_staged(&board, PROM_DOM_KEY_SGEMM_FACT_FORCE_DIRECT) == 1u, "force-direct key should be staged dirty");
    ASSERT_TRUE(prom_dom_dirty_key_staged(&board, PROM_DOM_KEY_SGEMM_FACT_FORCE_DIRECT_REASON) == 1u, "force-direct-reason key should be staged dirty");
    ASSERT_TRUE(prom_dom_dirty_key_staged(&board, PROM_DOM_KEY_SGEMM_JUDGMENT_WINNING_SCORE) == 1u, "winning-score key should be staged dirty");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_SGEMM_FACT_FORCE_DIRECT) == 1u, "force-direct key should be dirty on commit");
    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_SGEMM_FACT_FORCE_DIRECT_REASON) == 1u, "force-direct-reason key should be dirty on commit");
    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_SGEMM_JUDGMENT_WINNING_SCORE) == 1u, "winning-score key should be dirty on commit");
    ASSERT_TRUE(prom_dom_sgemm_build_path_compute_facts_from_visible(&board, &a, &projection) == 1u, "projection should build");
    ASSERT_TRUE(projection.dependent_dirty_key_mask_last_commit != 0u, "path/compute dependency dirty mask should be non-zero");
    ASSERT_TRUE(prom_dom_sgemm_stage_path_compute_facts(&board, &b) == 1u, "same-value facts should stage");
    ASSERT_TRUE(prom_dom_sgemm_stage_path_compute_decision(&board, &decision) == 1u, "same-value decision should stage");
    ASSERT_EQUAL(0u, prom_dom_dirty_keys_staged_word(&board, 0u), "same-value writes should not dirty staged mask word 0");
}

FACT(PrometheusDominatusSgemmAdapter_M10PathComputeCompatibilityMirrorNoDrift)
{
    prom_dom_blackboard board{};
    prom_dom_sgemm_path_compute_facts facts{};
    prom_dom_sgemm_path_compute_decision decision{};
    prom_dom_sgemm_path_compute_snapshot before{};
    prom_dom_sgemm_path_compute_snapshot after{};
    prom_dom_blackboard_init(&board);
    facts.m = 128u; facts.n = 64u; facts.k = 64u; facts.work_units = 524288u; facts.can_stage = 1u; facts.can_direct = 1u; facts.allow_fallback = 1u;
    ASSERT_TRUE(prom_dom_sgemm_stage_path_compute_facts(&board, &facts) == 1u, "facts should stage");
    prom_dom_sgemm_commit(&board);
    decision.success = 1u; decision.requested_path = PROM_VK_PATH_STAGED_UPLOAD; decision.selected_path = PROM_VK_PATH_STAGED_UPLOAD;
    decision.compute_mode = PROM_VK_COMPUTE_BASELINE; decision.final_detail = PROM_DETAIL_PATH_STAGED_UPLOAD; decision.winning_score = 501;
    ASSERT_TRUE(prom_dom_sgemm_stage_path_compute_decision(&board, &decision) == 1u, "decision should stage");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_read_visible_path_compute_diagnostics(&board, &before) == 1u, "baseline visible snapshot should read");
    decision.selected_path = PROM_VK_PATH_DIRECT; decision.compute_mode = PROM_VK_COMPUTE_TILED; decision.final_detail = PROM_DETAIL_PATH_DIRECT_TILED;
    ASSERT_TRUE(prom_dom_sgemm_stage_path_compute_decision(&board, &decision) == 1u, "mutated decision should stage");
    ASSERT_TRUE(prom_dom_sgemm_read_visible_path_compute_diagnostics(&board, &after) == 1u, "pre-commit visible snapshot should read");
    ASSERT_EQUAL(before.decision.selected_path, after.decision.selected_path, "visible mirror must remain pinned before commit");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_read_visible_path_compute_diagnostics(&board, &after) == 1u, "post-commit visible snapshot should read");
    ASSERT_EQUAL(decision.selected_path, after.decision.selected_path, "visible mirror should advance after commit");
}

FACT(PrometheusDominatusSgemmAdapter_M12AsyncSnapshotIsolation)
{
    prom_dom_blackboard board{};
    prom_dom_async_snapshot state_a{};
    prom_dom_async_snapshot state_b{};
    prom_dom_async_snapshot visible{};
    prom_dom_blackboard_init(&board);

    state_a.task_id = 7;
    state_a.lifecycle_state = PROM_ASYNC_STATE_SUBMITTED;
    state_a.stage = PROM_STAGE_SUBMIT;
    state_a.detail_code = PROM_DETAIL_PATH_DIRECT;
    state_a.outstanding_tasks = 1u;
    state_a.slot_id = 1;
    state_a.owns_slot = 1u;
    ASSERT_TRUE(prom_dom_sgemm_stage_async_snapshot(&board, &state_a, PROM_DOM_EVENT_ASYNC_SUBMITTED, 0) == 1u, "async state A should stage");
    prom_dom_sgemm_commit(&board);

    ASSERT_TRUE(prom_dom_sgemm_read_visible_async_snapshot(&board, &visible) == 1u, "visible async snapshot should read");
    ASSERT_EQUAL(state_a.task_id, visible.task_id, "visible snapshot should match committed A");

    state_b = state_a;
    state_b.lifecycle_state = PROM_ASYNC_STATE_READY;
    state_b.ready = 1u;
    state_b.outstanding_tasks = 0u;
    state_b.compute_complete = 1u;
    ASSERT_TRUE(prom_dom_sgemm_stage_async_snapshot(&board, &state_b, PROM_DOM_EVENT_ASYNC_READY, 0) == 1u, "async state B should stage");
    ASSERT_TRUE(prom_dom_sgemm_read_visible_async_snapshot(&board, &visible) == 1u, "pre-commit visible snapshot should read");
    ASSERT_EQUAL(state_a.lifecycle_state, visible.lifecycle_state, "staged B must be invisible before commit");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_read_visible_async_snapshot(&board, &visible) == 1u, "post-commit visible snapshot should read");
    ASSERT_EQUAL(state_b.lifecycle_state, visible.lifecycle_state, "committed B should become visible");
}

FACT(PrometheusDominatusSgemmAdapter_M12AsyncTransitionAndTransferReadinessFields)
{
    prom_dom_blackboard board{};
    prom_dom_async_snapshot snapshot{};
    prom_dom_blackboard_init(&board);

    snapshot.task_id = 9;
    snapshot.lifecycle_state = PROM_ASYNC_STATE_SUBMITTED;
    snapshot.stage = PROM_STAGE_SUBMIT;
    snapshot.detail_code = PROM_DETAIL_PATH_STAGED_UPLOAD_READBACK;
    snapshot.outstanding_tasks = 1u;
    snapshot.slot_id = 0;
    snapshot.slot_generation = 5u;
    snapshot.owns_slot = 1u;
    snapshot.transfer_complete = 0u;
    snapshot.compute_complete = 1u;
    ASSERT_TRUE(prom_dom_sgemm_stage_async_snapshot(&board, &snapshot, PROM_DOM_EVENT_ASYNC_SUBMITTED, 0) == 1u, "submitted state should stage");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_read_visible_async_snapshot(&board, &snapshot) == 1u, "submitted snapshot should read");
    ASSERT_EQUAL(0u, snapshot.ready, "compute-only completion should not force ready");
    ASSERT_EQUAL(0u, snapshot.transfer_complete, "transfer completion should remain explicit");

    snapshot.lifecycle_state = PROM_ASYNC_STATE_READY;
    snapshot.ready = 1u;
    snapshot.outstanding_tasks = 0u;
    snapshot.transfer_complete = 1u;
    snapshot.compute_complete = 1u;
    snapshot.readback_complete = 1u;
    ASSERT_TRUE(prom_dom_sgemm_stage_async_snapshot(&board, &snapshot, PROM_DOM_EVENT_ASYNC_READY, 0) == 1u, "ready state should stage");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_read_visible_async_snapshot(&board, &snapshot) == 1u, "ready snapshot should read");
    ASSERT_EQUAL(1u, snapshot.ready, "ready flag should be explicit");
    ASSERT_EQUAL(1u, snapshot.transfer_complete, "ready snapshot should retain transfer completion marker");
}

FACT(PrometheusDominatusSgemmAdapter_P13M9_ResourceLeaseStagedVisibleAndLifecycleDiagnostics)
{
    prom_dom_blackboard board{};
    prom_resource_lease_facts facts{};
    prom_dom_sgemm_resource_lease_projection projection{};
    prom_resource_lease_decision decision{};
    prom_dom_sgemm_resource_lease_snapshot snapshot{};
    prom_dom_blackboard_init(&board);

    facts.worker_id = 4u;
    facts.slot_id = 1u;
    facts.entry_id = 31u;
    facts.selected_recipe_variant = static_cast<std::uint32_t>(PROM_OCCUPANCY_KERNEL_VARIANT_BALANCED_2X2_ACCUM4);
    facts.requested_resource_class = static_cast<std::uint32_t>(PROM_LEASE_RESOURCE_CLASS_COMPUTE);
    facts.current_outstanding_depth = 1u;
    facts.max_outstanding_depth = 2u;
    facts.lookahead_requested = 1u;
    facts.lookahead_limit = 2u;
    facts.transfer_overlap_available = 1u;
    facts.true_multi_queue_selected = 1u;

    ASSERT_TRUE(prom_dom_sgemm_stage_resource_lease_facts(&board, &facts) == 1u, "lease facts should stage");
    ASSERT_TRUE(prom_dom_sgemm_read_visible_resource_lease_diagnostics(&board, &snapshot) == 0u,
                "lease diagnostics should remain invisible before commit");
    prom_dom_sgemm_commit(&board);

    ASSERT_TRUE(prom_dom_sgemm_build_resource_lease_facts_from_visible(&board, &facts, &projection) == 1u,
                "lease facts projection should use visible snapshot");
    prom_judgment_engine_decide_resource_lease(&projection.facts, &decision);
    ASSERT_TRUE(prom_dom_sgemm_stage_resource_lease_decision(&board, &decision) == 1u, "lease decision should stage");
    prom_dom_sgemm_commit(&board);

    ASSERT_TRUE(prom_dom_sgemm_read_visible_resource_lease_diagnostics(&board, &snapshot) == 1u,
                "lease diagnostics should be visible after decision commit");
    ASSERT_EQUAL(1u, snapshot.decision.grant, "happy-path lease should grant");
    ASSERT_EQUAL(1u, snapshot.granted_count, "grant counter should increment");
    ASSERT_EQUAL(facts.selected_recipe_variant, snapshot.facts.selected_recipe_variant, "recipe variant should round-trip");

    facts.current_outstanding_depth = 2u;
    ASSERT_TRUE(prom_dom_sgemm_stage_resource_lease_facts(&board, &facts) == 1u, "depth-cap facts should stage");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_build_resource_lease_facts_from_visible(&board, &facts, &projection) == 1u,
                "depth-cap projection should build");
    prom_judgment_engine_decide_resource_lease(&projection.facts, &decision);
    ASSERT_TRUE(prom_dom_sgemm_stage_resource_lease_decision(&board, &decision) == 1u, "depth-cap decision should stage");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_read_visible_resource_lease_diagnostics(&board, &snapshot) == 1u, "depth-cap diagnostics should read");
    ASSERT_EQUAL(0u, snapshot.decision.grant, "depth-cap lease should be denied");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_LEASE_REASON_DENIED_OUTSTANDING_LIMIT), snapshot.decision.deny_reason,
                 "depth-cap deny reason should be explicit");
    ASSERT_EQUAL(0u, snapshot.decision.lookahead_allowed, "lookahead should be blocked at cap");
    ASSERT_EQUAL(1u, snapshot.denied_count, "deny counter should increment");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_LEASE_REASON_DENIED_OUTSTANDING_LIMIT), snapshot.lookahead_blocked_reason,
                 "lookahead blocked reason should be visible");

    facts.current_outstanding_depth = 1u;
    facts.yield_requested = 1u;
    ASSERT_TRUE(prom_dom_sgemm_stage_resource_lease_facts(&board, &facts) == 1u, "yield facts should stage");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_build_resource_lease_facts_from_visible(&board, &facts, &projection) == 1u,
                "yield projection should build");
    prom_judgment_engine_decide_resource_lease(&projection.facts, &decision);
    ASSERT_TRUE(prom_dom_sgemm_stage_resource_lease_decision(&board, &decision) == 1u, "yield decision should stage");
    prom_dom_sgemm_commit(&board);
    ASSERT_TRUE(prom_dom_sgemm_read_visible_resource_lease_diagnostics(&board, &snapshot) == 1u, "yield diagnostics should read");
    ASSERT_EQUAL(static_cast<std::uint32_t>(PROM_LEASE_STATE_YIELDED), snapshot.decision.lease_state, "yield state should be visible");
    ASSERT_EQUAL(1u, snapshot.yield_count, "yield counter should increment");
}

FACT(PrometheusStage4_ExecutionHandoffPreservesResolvedMechanicalValues)
{
    VkDescriptorBufferInfo inputs[3]{};
    prom_sgemm_dispatch_geometry geometry{};
    inputs[0].offset = 16u;
    inputs[0].range = 64u;
    inputs[1].offset = 32u;
    inputs[1].range = 128u;
    inputs[2].offset = 48u;
    inputs[2].range = 192u;
    geometry.groups_x = 3u;
    geometry.groups_y = 5u;
    geometry.groups_z = 1u;
    geometry.logical_m_per_group = 8u;
    geometry.logical_n_per_group = 8u;

    const prom_sgemm_execution_handoff handoff = prom_sgemm_make_execution_handoff(
        3u, 17u, 7u, 7u,
        static_cast<std::uint32_t>(PROM_VK_PATH_STAGED_UPLOAD_READBACK),
        static_cast<std::uint32_t>(PROM_VK_COMPUTE_TILED),
        4u, 1u, 1u, VK_NULL_HANDLE, VK_NULL_HANDLE, inputs, &geometry);

    ASSERT_EQUAL(3u, handoff.m, "handoff must preserve resolved M");
    ASSERT_EQUAL(17u, handoff.n, "handoff must preserve resolved N");
    ASSERT_EQUAL(7u, handoff.k, "handoff must preserve logical K");
    ASSERT_EQUAL(7u, handoff.compute_k, "handoff must preserve mechanical K");
    ASSERT_EQUAL(4u, handoff.selected_variant, "handoff must preserve the selected variant without reselection");
    ASSERT_EQUAL(1u, handoff.slot_id, "handoff must preserve the already-selected SGEMM slot");
    ASSERT_EQUAL(1u, handoff.wait_for_transfer, "handoff must preserve the synchronization dependency");
    ASSERT_EQUAL(inputs[1].offset, handoff.descriptor_inputs[1].offset, "handoff must preserve descriptor offsets");
    ASSERT_EQUAL(inputs[2].range, handoff.descriptor_inputs[2].range, "handoff must preserve descriptor ranges");
    ASSERT_EQUAL(geometry.groups_x, handoff.dispatch_geometry.groups_x, "handoff must preserve dispatch X");
    ASSERT_EQUAL(geometry.groups_y, handoff.dispatch_geometry.groups_y, "handoff must preserve dispatch Y");
}
