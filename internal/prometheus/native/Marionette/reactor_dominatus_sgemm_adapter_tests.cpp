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
