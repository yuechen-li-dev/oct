#include "../reactor_dominatus_blackboard.h"
#include "test_harness.h"

#include <cstdint>

namespace
{
    constexpr std::uint64_t KeyBitFromIndex(std::uint32_t index)
    {
        return std::uint64_t{ 1 } << index;
    }

    constexpr std::uint32_t DomainBit(prom_dom_domain domain)
    {
        return std::uint32_t{ 1 } << (static_cast<std::uint32_t>(domain) - 1u);
    }
}

FACT(PrometheusDominatusBlackboard_StagedWriteNotVisibleBeforeCommit)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);

    ASSERT_TRUE(prom_dom_set_u32(&board, PROM_DOM_SOURCE_REACTOR, PROM_DOM_KEY_SGEMM_PATH_MODE, 0u, 2u, 0) == 1u,
                "staged setter should accept SGEMM path mode");

    std::uint32_t pathMode = 99u;
    ASSERT_TRUE(prom_dom_get_u32(&board, PROM_DOM_KEY_SGEMM_PATH_MODE, 0u, &pathMode) == 0u,
                "visible getter should still be unset before commit");

    prom_dom_commit(&board);

    ASSERT_TRUE(prom_dom_get_u32(&board, PROM_DOM_KEY_SGEMM_PATH_MODE, 0u, &pathMode) == 1u,
                "visible getter should read committed value after commit");
    ASSERT_EQUAL(2u, pathMode, "committed path mode should match staged write");
}

FACT(PrometheusDominatusBlackboard_DirtyKeyAndDomainTracking)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);

    ASSERT_TRUE(prom_dom_set_u32(&board, PROM_DOM_SOURCE_JUDGMENT, PROM_DOM_KEY_SGEMM_COMPUTE_MODE, 0u, 4u, 0) == 1u,
                "staged setter should succeed");

    const std::uint64_t stagedDirtyKeys = prom_dom_dirty_keys_staged_word(&board, 0u);
    const std::uint32_t stagedDirtyDomains = prom_dom_dirty_domains_staged(&board);
    ASSERT_TRUE((stagedDirtyKeys & KeyBitFromIndex(4u)) != 0u, "SGEMM compute mode key should be marked staged dirty");
    ASSERT_TRUE((stagedDirtyDomains & DomainBit(PROM_DOM_DOMAIN_SGEMM)) != 0u,
                "SGEMM domain should be marked staged dirty");
    ASSERT_TRUE((stagedDirtyDomains & DomainBit(PROM_DOM_DOMAIN_QUEUE)) == 0u,
                "unrelated queue domain should not be marked dirty");

    prom_dom_commit(&board);

    const std::uint64_t lastCommitDirtyKeys = prom_dom_dirty_keys_last_commit_word(&board, 0u);
    const std::uint32_t lastCommitDirtyDomains = prom_dom_dirty_domains_last_commit(&board);
    ASSERT_TRUE((lastCommitDirtyKeys & KeyBitFromIndex(4u)) != 0u,
                "last-commit dirty keys should include SGEMM compute mode");
    ASSERT_TRUE((lastCommitDirtyDomains & DomainBit(PROM_DOM_DOMAIN_SGEMM)) != 0u,
                "last-commit dirty domains should include SGEMM");
    ASSERT_EQUAL(0u, prom_dom_dirty_keys_staged_word(&board, 0u), "staged dirty key mask should clear after commit");
    ASSERT_EQUAL(0u, prom_dom_dirty_domains_staged(&board), "staged dirty domain mask should clear after commit");
}

FACT(PrometheusDominatusBlackboard_DirtySlotTracking)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);

    ASSERT_TRUE(prom_dom_set_u32(&board, PROM_DOM_SOURCE_SLOT_HFSM, PROM_DOM_KEY_SLOT_STATE, 0u, 3u, 0) == 1u,
                "slot-scoped setter should succeed");

    const std::uint32_t stagedDirtySlots = prom_dom_dirty_slots_staged(&board);
    ASSERT_TRUE((stagedDirtySlots & 0x1u) != 0u, "slot 0 should be marked dirty");
    ASSERT_TRUE((stagedDirtySlots & 0x2u) == 0u, "slot 1 should remain clean");
}

FACT(PrometheusDominatusBlackboard_SameValueWriteDoesNotDirty)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);

    ASSERT_TRUE(prom_dom_set_u32(&board, PROM_DOM_SOURCE_REACTOR, PROM_DOM_KEY_QUEUE_COMPUTE_FAMILY, 0u, 1u, 0) == 1u,
                "initial write should succeed");
    prom_dom_commit(&board);

    ASSERT_TRUE(prom_dom_set_u32(&board, PROM_DOM_SOURCE_REACTOR, PROM_DOM_KEY_QUEUE_COMPUTE_FAMILY, 0u, 1u, 0) == 1u,
                "same-value write should still return success");
    ASSERT_EQUAL(0u, prom_dom_dirty_keys_staged_word(&board, 0u), "same-value write should not mark key dirty");
    ASSERT_EQUAL(0u, prom_dom_dirty_domains_staged(&board), "same-value write should not mark domain dirty");
}

FACT(PrometheusDominatusBlackboard_EventStagingAndCommit)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);

    prom_dom_event event{};
    event.kind = PROM_DOM_EVENT_SLOT_READY;
    event.source = PROM_DOM_SOURCE_SLOT_HFSM;
    event.domain = PROM_DOM_DOMAIN_SLOT;
    event.key = PROM_DOM_KEY_SLOT_STATE;
    event.slot_id = 0u;
    event.reason_code = 77;

    ASSERT_TRUE(prom_dom_stage_event(&board, &event) == 1u, "staging an event should succeed");
    ASSERT_EQUAL(1u, prom_dom_staged_event_count(&board), "staged event ring should contain event before commit");
    ASSERT_EQUAL(0u, prom_dom_committed_event_count(&board), "committed event window should be empty before commit");

    prom_dom_commit(&board);

    ASSERT_EQUAL(0u, prom_dom_staged_event_count(&board), "staged event ring should clear on commit");
    ASSERT_EQUAL(1u, prom_dom_committed_event_count(&board), "committed event window should contain promoted event");

    prom_dom_event committed{};
    ASSERT_TRUE(prom_dom_committed_event_at(&board, 0u, &committed) == 1u, "committed event should be readable");
    ASSERT_EQUAL(PROM_DOM_EVENT_SLOT_READY, committed.kind, "committed event should keep kind");
    ASSERT_EQUAL(0u, committed.slot_id, "committed event should keep slot id");
    ASSERT_EQUAL(77, committed.reason_code, "committed event should keep reason code");
}

FACT(PrometheusDominatusBlackboard_TraceRingWrapDeterministic)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);

    for (std::uint32_t i = 0u; i < PROM_DOM_MAX_TRACE + 3u; ++i) {
        ASSERT_TRUE(prom_dom_set_u32(&board,
                                     PROM_DOM_SOURCE_REACTOR,
                                     PROM_DOM_KEY_DIAGNOSTICS_COUNTER,
                                     0u,
                                     i + 1u,
                                     static_cast<std::int32_t>(900 + i)) == 1u,
                    "setter should produce trace entries");
    }

    ASSERT_EQUAL(PROM_DOM_MAX_TRACE, prom_dom_trace_count(&board), "trace ring should cap at fixed capacity");

    prom_dom_trace_entry oldest{};
    prom_dom_trace_entry newest{};
    ASSERT_TRUE(prom_dom_trace_at(&board, 0u, &oldest) == 1u, "oldest retained trace should be readable");
    ASSERT_TRUE(prom_dom_trace_at(&board, prom_dom_trace_count(&board) - 1u, &newest) == 1u,
                "newest retained trace should be readable");

    ASSERT_EQUAL(PROM_DOM_KEY_DIAGNOSTICS_COUNTER, oldest.key, "oldest retained trace should keep key metadata");
    ASSERT_EQUAL(PROM_DOM_SOURCE_REACTOR, oldest.source, "oldest retained trace should keep source metadata");
    ASSERT_EQUAL(PROM_DOM_KEY_DIAGNOSTICS_COUNTER, newest.key, "newest retained trace should keep key metadata");
    ASSERT_EQUAL(static_cast<std::int32_t>(900 + PROM_DOM_MAX_TRACE + 2u), newest.reason_code,
                 "newest retained trace should keep reason metadata");
}

FACT(PrometheusDominatusBlackboard_GenerationCountersOnCommit)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);

    ASSERT_EQUAL(0u, board.visible_generation, "visible generation should initialize to 0");
    ASSERT_EQUAL(1u, board.staged_generation, "staged generation should initialize to next commit generation");

    ASSERT_TRUE(prom_dom_set_u64(&board,
                                 PROM_DOM_SOURCE_MEMORY,
                                 PROM_DOM_KEY_MEMORY_BUDGET,
                                 0u,
                                 4096u,
                                 1) == 1u,
                "staged write should succeed");
    prom_dom_commit(&board);

    ASSERT_EQUAL(1u, board.visible_generation, "visible generation should increment after first commit");
    ASSERT_EQUAL(2u, board.staged_generation, "staged generation should track next commit generation");

    prom_dom_event event{};
    event.kind = PROM_DOM_EVENT_QUEUE_HANDOFF;
    event.source = PROM_DOM_SOURCE_QUEUE;
    event.domain = PROM_DOM_DOMAIN_QUEUE;
    event.key = PROM_DOM_KEY_QUEUE_HANDOFF_COUNT;
    event.reason_code = 5;
    ASSERT_TRUE(prom_dom_stage_event(&board, &event) == 1u, "event staging should succeed");

    prom_dom_commit(&board);

    prom_dom_event committed{};
    ASSERT_TRUE(prom_dom_committed_event_at(&board, 0u, &committed) == 1u, "committed event should be readable");
    ASSERT_EQUAL(2u, committed.generation, "committed event generation should match commit generation");
}

FACT(PrometheusDominatusBlackboard_ResetDeterministic)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);

    ASSERT_TRUE(prom_dom_set_i32(&board,
                                 PROM_DOM_SOURCE_DIAGNOSTICS,
                                 PROM_DOM_KEY_DIAGNOSTICS_REASON_CODE,
                                 0u,
                                 -12,
                                 -12) == 1u,
                "set before reset should succeed");

    prom_dom_event event{};
    event.kind = PROM_DOM_EVENT_FALLBACK_EMITTED;
    event.source = PROM_DOM_SOURCE_DIAGNOSTICS;
    event.domain = PROM_DOM_DOMAIN_DIAGNOSTICS;
    event.key = PROM_DOM_KEY_DIAGNOSTICS_REASON_CODE;
    event.reason_code = -12;
    ASSERT_TRUE(prom_dom_stage_event(&board, &event) == 1u, "event before reset should succeed");

    prom_dom_blackboard_reset(&board);

    ASSERT_EQUAL(0u, board.visible_generation, "reset should clear visible generation");
    ASSERT_EQUAL(1u, board.staged_generation, "reset should restore staged generation");
    ASSERT_EQUAL(0u, prom_dom_dirty_keys_staged_word(&board, 0u), "reset should clear staged dirty keys");
    ASSERT_EQUAL(0u, prom_dom_dirty_domains_staged(&board), "reset should clear staged dirty domains");
    ASSERT_EQUAL(0u, prom_dom_dirty_slots_staged(&board), "reset should clear staged dirty slots");
    ASSERT_EQUAL(0u, prom_dom_staged_event_count(&board), "reset should clear staged events");
    ASSERT_EQUAL(0u, prom_dom_committed_event_count(&board), "reset should clear committed events");
    ASSERT_EQUAL(0u, prom_dom_trace_count(&board), "reset should clear trace ring");
}
