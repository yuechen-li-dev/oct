#include "../bridge.h"
#include "../reactor_dominatus_slot_adapter.h"
#include "test_harness.h"

#include <array>
#include <cstdint>
#include <vector>

namespace
{
std::vector<float> deterministic_matrix(std::uint32_t rows, std::uint32_t cols)
{
    std::vector<float> out(rows * cols, 0.0f);
    for (std::size_t i = 0; i < out.size(); ++i) {
        const int v = static_cast<int>(i % 17u) - 8;
        out[i] = static_cast<float>(v) / 4.0f;
    }
    return out;
}

prom_slot_metadata SlotMetadata(
    std::uint32_t slotId,
    std::uint64_t generation,
    std::uint32_t valid,
    int failureReason)
{
    prom_slot_metadata metadata{};
    metadata.slot_id = slotId;
    metadata.generation = generation;
    metadata.valid = valid;
    metadata.failure_reason = failureReason;
    return metadata;
}
}

FACT(PrometheusDominatusSlotAdapter_StagesEventsBeforeCommit)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);
    const prom_slot_metadata metadata = SlotMetadata(0u, 3u, 1u, 0);

    ASSERT_TRUE(
        prom_dom_slot_stage_lifecycle(
            &board,
            PROM_DOM_EVENT_SLOT_READY,
            0u,
            PROM_SLOT_READY,
            &metadata,
            0u,
            0u,
            1u,
            0u,
            77) == 1u,
        "slot stage should succeed");

    ASSERT_EQUAL(1u, prom_dom_staged_event_count(&board), "slot event should be staged");
    ASSERT_EQUAL(0u, prom_dom_committed_event_count(&board), "slot event should not be committed pre-commit");
    ASSERT_TRUE((prom_dom_dirty_slots_staged(&board) & 0x1u) != 0u, "staged dirty slot mask should include slot0");
    ASSERT_EQUAL(0u, prom_dom_dirty_slots_last_commit(&board), "last-commit dirty slots should remain unchanged pre-commit");

    prom_dom_slot_commit(&board);

    ASSERT_EQUAL(0u, prom_dom_staged_event_count(&board), "staged event ring should clear after commit");
    ASSERT_EQUAL(1u, prom_dom_committed_event_count(&board), "committed event ring should contain slot event");
    ASSERT_TRUE((prom_dom_dirty_slots_last_commit(&board) & 0x1u) != 0u, "last-commit dirty slots should include slot0");

    prom_dom_event committed{};
    ASSERT_TRUE(prom_dom_committed_event_at(&board, 0u, &committed) == 1u, "committed slot event should be readable");
    ASSERT_EQUAL(PROM_DOM_EVENT_SLOT_READY, committed.kind, "event kind should match staged kind");
    ASSERT_EQUAL(0u, committed.slot_id, "event slot id should match staged slot");
}

FACT(PrometheusDominatusSlotAdapter_DirtySlotTrackingPerSlot)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);

    const prom_slot_metadata slot0 = SlotMetadata(0u, 1u, 1u, 0);
    const prom_slot_metadata slot1 = SlotMetadata(1u, 1u, 1u, 0);

    ASSERT_TRUE(prom_dom_slot_stage_lifecycle(&board,
                                              PROM_DOM_EVENT_SLOT_PREPARED,
                                              0u,
                                              PROM_SLOT_PREPARING,
                                              &slot0,
                                              0u,
                                              0u,
                                              0u,
                                              0u,
                                              0) == 1u,
                "slot0 stage should succeed");
    ASSERT_TRUE((prom_dom_dirty_slots_staged(&board) & 0x1u) != 0u, "slot0 should be marked dirty");
    ASSERT_TRUE((prom_dom_dirty_slots_staged(&board) & 0x2u) == 0u, "slot1 should remain clean");

    prom_dom_slot_commit(&board);
    ASSERT_TRUE((prom_dom_dirty_slots_last_commit(&board) & 0x1u) != 0u, "slot0 should remain last-commit dirty");

    ASSERT_TRUE(prom_dom_slot_stage_lifecycle(&board,
                                              PROM_DOM_EVENT_SLOT_PREPARED,
                                              1u,
                                              PROM_SLOT_PREPARING,
                                              &slot1,
                                              0u,
                                              0u,
                                              0u,
                                              0u,
                                              0) == 1u,
                "slot1 stage should succeed");
    ASSERT_TRUE((prom_dom_dirty_slots_staged(&board) & 0x2u) != 0u, "slot1 should be marked dirty");
    ASSERT_TRUE((prom_dom_dirty_slots_staged(&board) & 0x1u) == 0u, "slot0 should not be restaged when unchanged");
}

FACT(PrometheusDominatusSlotAdapter_RepresentativeLifecycleSequence)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);

    std::array<prom_dom_event_kind, 6u> expectedKinds{
        PROM_DOM_EVENT_SLOT_PREPARED,
        PROM_DOM_EVENT_SLOT_READY,
        PROM_DOM_EVENT_SLOT_PROMOTED_CURRENT,
        PROM_DOM_EVENT_SLOT_SUBMITTED,
        PROM_DOM_EVENT_SLOT_COMPLETE,
        PROM_DOM_EVENT_SLOT_CONSUMED,
    };

    std::array<prom_slot_state, 6u> states{
        PROM_SLOT_PREPARING,
        PROM_SLOT_READY,
        PROM_SLOT_CURRENT,
        PROM_SLOT_IN_FLIGHT,
        PROM_SLOT_CONSUMED,
        PROM_SLOT_EMPTY,
    };

    for (std::size_t i = 0; i < expectedKinds.size(); ++i) {
        const prom_slot_metadata metadata = SlotMetadata(0u, static_cast<std::uint64_t>(i + 1u), i < 5u ? 1u : 0u, 0);
        ASSERT_TRUE(prom_dom_slot_stage_lifecycle(&board,
                                                  expectedKinds[i],
                                                  0u,
                                                  states[i],
                                                  &metadata,
                                                  1u,
                                                  0u,
                                                  1u,
                                                  1u,
                                                  0) == 1u,
                    "lifecycle stage step should succeed");
        prom_dom_slot_commit(&board);
    }

    ASSERT_EQUAL(expectedKinds.size(), static_cast<std::size_t>(prom_dom_committed_event_count(&board)),
                 "committed event count should match lifecycle sequence");

    for (std::size_t i = 0; i < expectedKinds.size(); ++i) {
        prom_dom_event event{};
        ASSERT_TRUE(prom_dom_committed_event_at(&board, static_cast<std::uint32_t>(i), &event) == 1u,
                    "committed event should be readable");
        ASSERT_EQUAL(expectedKinds[i], event.kind, "committed kind should preserve lifecycle order");
    }

    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_SLOT_STATE) == 1u,
                "slot state key should be dirty in last commit");
    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_SLOT_GENERATION) == 1u,
                "slot generation key should be dirty in last commit");
    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_SLOT_VALID) == 1u,
                "slot valid key should be dirty in last commit");
}

FACT(PrometheusDominatusSlotAdapter_FailureCleanupAndTrace)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);

    const prom_slot_metadata failed = SlotMetadata(1u, 11u, 0u, 404);
    ASSERT_TRUE(prom_dom_slot_stage_lifecycle(&board,
                                              PROM_DOM_EVENT_SLOT_FAILED,
                                              1u,
                                              PROM_SLOT_FAILED,
                                              &failed,
                                              0u,
                                              0u,
                                              0u,
                                              0u,
                                              404) == 1u,
                "failure stage should succeed");
    prom_dom_slot_commit(&board);

    const prom_slot_metadata cleaned = SlotMetadata(1u, 12u, 0u, 0);
    ASSERT_TRUE(prom_dom_slot_stage_lifecycle(&board,
                                              PROM_DOM_EVENT_SLOT_CLEANUP,
                                              1u,
                                              PROM_SLOT_EMPTY,
                                              &cleaned,
                                              0u,
                                              0u,
                                              0u,
                                              0u,
                                              0) == 1u,
                "cleanup stage should succeed");
    prom_dom_slot_commit(&board);

    prom_dom_slot_commit_snapshot snapshot{};
    ASSERT_TRUE(prom_dom_slot_read_last_commit(&board, 1u, &snapshot) == 1u, "last commit slot snapshot should be readable");
    ASSERT_EQUAL(PROM_DOM_EVENT_SLOT_CLEANUP, snapshot.last_event.kind, "last committed slot event should be cleanup");
    ASSERT_EQUAL(1u, snapshot.last_event.slot_id, "cleanup event should preserve slot id");

    const std::uint32_t traceCount = prom_dom_trace_count(&board);
    ASSERT_TRUE(traceCount >= 2u, "failure and cleanup should leave trace entries");
    prom_dom_trace_entry newest{};
    ASSERT_TRUE(prom_dom_trace_at(&board, traceCount - 1u, &newest) == 1u, "latest trace should be readable");
    ASSERT_EQUAL(PROM_DOM_EVENT_SLOT_CLEANUP, newest.event_kind, "latest trace should represent cleanup event");
}

FACT(PrometheusDominatusSlotAdapter_M16SingleReadyTransitionTracksReadinessMasks)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);

    const prom_slot_metadata metadata = SlotMetadata(0u, 9u, 1u, 0);
    ASSERT_TRUE(prom_dom_slot_stage_lifecycle(&board,
                                              PROM_DOM_EVENT_SLOT_READY,
                                              0u,
                                              PROM_SLOT_READY,
                                              &metadata,
                                              0u,
                                              0u,
                                              0u,
                                              0u,
                                              0) == 1u,
                "slot ready stage should succeed");
    prom_dom_slot_commit(&board);

    prom_dom_slot_readiness_snapshot readiness{};
    ASSERT_TRUE(prom_dom_slot_readiness_read_visible(&board, &readiness) == 1u, "readiness snapshot should be readable");
    ASSERT_TRUE((readiness.dirty_slot_mask & 0x1u) != 0u, "dirty mask should include slot0");
    ASSERT_TRUE((readiness.ready_slot_mask & 0x1u) != 0u, "ready mask should include slot0");
    ASSERT_TRUE((readiness.attention_slot_mask & 0x1u) != 0u, "attention mask should include slot0");
    ASSERT_TRUE((readiness.attention_slot_mask & 0x2u) == 0u, "attention mask should not include unrelated slot1");
}

FACT(PrometheusDominatusSlotAdapter_M16MultipleSlotsReadyInBoundary)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);
    const prom_slot_metadata slot0Ready = SlotMetadata(0u, 3u, 1u, 0);
    const prom_slot_metadata slot1Ready = SlotMetadata(1u, 4u, 1u, 0);

    ASSERT_TRUE(prom_dom_slot_stage_lifecycle(&board,
                                              PROM_DOM_EVENT_SLOT_READY,
                                              0u,
                                              PROM_SLOT_READY,
                                              &slot0Ready,
                                              0u,
                                              0u,
                                              0u,
                                              0u,
                                              0) == 1u,
                "slot0 ready stage should succeed");
    prom_dom_slot_commit(&board);
    ASSERT_TRUE(prom_dom_slot_stage_lifecycle(&board,
                                              PROM_DOM_EVENT_SLOT_READY,
                                              1u,
                                              PROM_SLOT_READY,
                                              &slot1Ready,
                                              0u,
                                              0u,
                                              0u,
                                              0u,
                                              0) == 1u,
                "slot1 ready stage should succeed");
    prom_dom_slot_commit(&board);

    prom_dom_slot_readiness_snapshot readiness{};
    ASSERT_TRUE(prom_dom_slot_readiness_read_visible(&board, &readiness) == 1u, "readiness snapshot should be readable");
    ASSERT_TRUE((readiness.ready_slot_mask & 0x3u) == 0x3u, "ready mask should include both slots");
    ASSERT_TRUE((readiness.attention_slot_mask & 0x3u) == 0x3u, "attention mask should include both slots");
}

FACT(PrometheusDominatusSlotAdapter_M16FailureDominatesAttention)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);
    const prom_slot_metadata ready = SlotMetadata(0u, 8u, 1u, 0);
    const prom_slot_metadata failed = SlotMetadata(0u, 8u, 0u, 404);

    ASSERT_TRUE(prom_dom_slot_stage_lifecycle(&board,
                                              PROM_DOM_EVENT_SLOT_READY,
                                              0u,
                                              PROM_SLOT_READY,
                                              &ready,
                                              0u,
                                              0u,
                                              0u,
                                              0u,
                                              0) == 1u,
                "ready stage should succeed");
    prom_dom_slot_commit(&board);
    ASSERT_TRUE(prom_dom_slot_stage_lifecycle(&board,
                                              PROM_DOM_EVENT_SLOT_FAILED,
                                              0u,
                                              PROM_SLOT_FAILED,
                                              &failed,
                                              0u,
                                              0u,
                                              0u,
                                              0u,
                                              404) == 1u,
                "failure stage should succeed");
    prom_dom_slot_commit(&board);

    prom_dom_slot_readiness_snapshot readiness{};
    ASSERT_TRUE(prom_dom_slot_readiness_read_visible(&board, &readiness) == 1u, "readiness snapshot should be readable");
    ASSERT_TRUE((readiness.failed_slot_mask & 0x1u) != 0u, "failed mask should include slot0");
    ASSERT_TRUE((readiness.attention_slot_mask & 0x1u) != 0u, "attention mask should include slot0 failure");
    ASSERT_TRUE((readiness.ready_slot_mask & 0x1u) == 0u, "ready mask should clear slot0 after failure");
}

FACT(PrometheusDominatusSlotAdapter_M16InvalidationSurvivesNonSlotCommit)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);
    const prom_slot_metadata invalidated = SlotMetadata(1u, 19u, 0u, PROM_DETAIL_SLOT_STALE_REJECTED);

    ASSERT_TRUE(prom_dom_slot_stage_lifecycle(&board,
                                              PROM_DOM_EVENT_SLOT_INVALIDATED,
                                              1u,
                                              PROM_SLOT_READY,
                                              &invalidated,
                                              0u,
                                              0u,
                                              0u,
                                              0u,
                                              PROM_DETAIL_SLOT_STALE_REJECTED) == 1u,
                "invalidated stage should succeed");
    prom_dom_slot_commit(&board);

    ASSERT_TRUE(prom_dom_set_u32(&board,
                                 PROM_DOM_SOURCE_JUDGMENT,
                                 PROM_DOM_KEY_SGEMM_JUDGMENT_SUCCESS,
                                 0u,
                                 1u,
                                 0) == 1u,
                "non-slot staged write should succeed");
    prom_dom_commit(&board);

    prom_dom_slot_readiness_snapshot readiness{};
    ASSERT_TRUE(prom_dom_slot_readiness_read_visible(&board, &readiness) == 1u, "readiness snapshot should be readable");
    ASSERT_TRUE((readiness.invalidated_slot_mask & 0x2u) != 0u, "invalidated mask should retain slot1");
    ASSERT_TRUE((readiness.attention_slot_mask & 0x2u) != 0u, "attention mask should retain invalidated slot1");
}

FACT(PrometheusDominatusSlotAdapter_M16CleanupToEmptyClearsFailureAttentionButKeepsDirty)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);
    const prom_slot_metadata failed = SlotMetadata(1u, 22u, 0u, 500);
    const prom_slot_metadata cleaned = SlotMetadata(1u, 23u, 0u, 0);

    ASSERT_TRUE(prom_dom_slot_stage_lifecycle(&board,
                                              PROM_DOM_EVENT_SLOT_FAILED,
                                              1u,
                                              PROM_SLOT_FAILED,
                                              &failed,
                                              0u,
                                              0u,
                                              0u,
                                              0u,
                                              500) == 1u,
                "failure stage should succeed");
    prom_dom_slot_commit(&board);
    ASSERT_TRUE(prom_dom_slot_stage_lifecycle(&board,
                                              PROM_DOM_EVENT_SLOT_CLEANUP,
                                              1u,
                                              PROM_SLOT_EMPTY,
                                              &cleaned,
                                              0u,
                                              0u,
                                              0u,
                                              0u,
                                              0) == 1u,
                "cleanup stage should succeed");
    prom_dom_slot_commit(&board);

    prom_dom_slot_readiness_snapshot readiness{};
    ASSERT_TRUE(prom_dom_slot_readiness_read_visible(&board, &readiness) == 1u, "readiness snapshot should be readable");
    ASSERT_TRUE((readiness.failed_slot_mask & 0x2u) == 0u, "cleanup-to-empty should clear failed mask for slot1");
    ASSERT_TRUE((readiness.attention_slot_mask & 0x2u) == 0u, "cleanup-to-empty should clear attention slot1");
    ASSERT_TRUE((readiness.dirty_slot_mask & 0x2u) != 0u, "cleanup transition should preserve dirty tracking for slot1");
}

FACT(PrometheusDominatusSlotAdapter_M16DuplicateReadyCoalescingAndBoundaryAdvance)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);
    const prom_slot_metadata ready = SlotMetadata(0u, 30u, 1u, 0);

    ASSERT_TRUE(prom_dom_slot_stage_lifecycle(&board,
                                              PROM_DOM_EVENT_SLOT_READY,
                                              0u,
                                              PROM_SLOT_READY,
                                              &ready,
                                              0u,
                                              0u,
                                              0u,
                                              0u,
                                              0) == 1u,
                "first ready stage should succeed");
    prom_dom_slot_commit(&board);
    ASSERT_TRUE(prom_dom_slot_stage_lifecycle(&board,
                                              PROM_DOM_EVENT_SLOT_READY,
                                              0u,
                                              PROM_SLOT_READY,
                                              &ready,
                                              0u,
                                              0u,
                                              0u,
                                              0u,
                                              0) == 1u,
                "duplicate ready stage should succeed");
    prom_dom_slot_commit(&board);

    prom_dom_slot_readiness_snapshot readiness{};
    ASSERT_TRUE(prom_dom_slot_readiness_read_visible(&board, &readiness) == 1u, "readiness snapshot should be readable");
    ASSERT_TRUE((readiness.attention_slot_mask & 0x1u) != 0u, "duplicate ready should still yield one attention bit");
    ASSERT_TRUE(readiness.duplicate_ready_event_count >= 1u, "duplicate ready counter should increment");

    const std::uint64_t priorGeneration = readiness.boundary_generation;
    prom_dom_slot_readiness_clear_boundary(&board);
    ASSERT_TRUE(prom_dom_slot_readiness_read_visible(&board, &readiness) == 1u, "readiness snapshot should be readable after clear");
    ASSERT_EQUAL(priorGeneration + 1u, readiness.boundary_generation, "boundary generation should advance on clear");
    ASSERT_EQUAL(0u, readiness.dirty_slot_mask, "dirty mask should clear at boundary advance");
    ASSERT_EQUAL(0u, readiness.attention_slot_mask, "attention mask should clear at boundary advance");
}

FACT(PrometheusDominatusSlotAdapter_RuntimeSmokeFixedDoubleProducesCommittedSlotEvent)
{
    void* handle = nullptr;
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_create(nullptr, &handle), "runtime create should succeed");

    PrometheusCaps caps{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_probe(handle, &caps), "probe should succeed");
    if (caps.available == 0u) {
        SKIP("Vulkan runtime unavailable; runtime slot bridge smoke cannot be asserted");
    }

    const auto a = deterministic_matrix(8u, 8u);
    const auto b = deterministic_matrix(8u, 8u);
    std::vector<float> c(64u, 0.0f);
    std::uint32_t stage = PROM_STAGE_NONE;
    int detail = 0;

    ASSERT_EQUAL(PROM_OK,
                 prometheus_reactor_runtime_sgemm(handle, a.data(), b.data(), c.data(), 8u, 8u, 8u, &stage, &detail),
                 "SGEMM runtime path should succeed for slot bridge smoke");

    PrometheusSgemmPolicyDiagnostics diagnostics{};
    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_sgemm_policy_diagnostics(handle, &diagnostics),
                 "diagnostics should succeed");

    ASSERT_TRUE(diagnostics.p10_m4_last_slot_event_kind != PROM_DOM_EVENT_NONE,
                "runtime path should emit at least one committed slot event");
    ASSERT_TRUE(diagnostics.p10_m4_last_slot_event_kind == PROM_DOM_EVENT_SLOT_PREPARED ||
                    diagnostics.p10_m4_last_slot_event_kind == PROM_DOM_EVENT_SLOT_READY ||
                    diagnostics.p10_m4_last_slot_event_kind == PROM_DOM_EVENT_SLOT_SUBMITTED ||
                    diagnostics.p10_m4_last_slot_event_kind == PROM_DOM_EVENT_SLOT_COMPLETE ||
                    diagnostics.p10_m4_last_slot_event_kind == PROM_DOM_EVENT_SLOT_FAILED ||
                    diagnostics.p10_m4_last_slot_event_kind == PROM_DOM_EVENT_SLOT_PROMOTED_CURRENT ||
                    diagnostics.p10_m4_last_slot_event_kind == PROM_DOM_EVENT_SLOT_CONSUMED ||
                    diagnostics.p10_m4_last_slot_event_kind == PROM_DOM_EVENT_SLOT_CLEANUP ||
                    diagnostics.p10_m4_last_slot_event_kind == PROM_DOM_EVENT_SLOT_INVALIDATED,
                "runtime path should report a slot lifecycle event kind");
    ASSERT_TRUE(diagnostics.p10_m4_last_slot_event_slot_id < 2u,
                "runtime path should report slot id within fixed-double ownership domain");
    ASSERT_TRUE((diagnostics.p10_m4_last_commit_dirty_slot_mask & 0x3u) != 0u,
                "runtime path should report committed dirty slot mask for one fixed-double slot");
    ASSERT_TRUE((diagnostics.p10_m4_last_commit_dirty_slot_mask &
                 (1u << diagnostics.p10_m4_last_slot_event_slot_id)) != 0u,
                "dirty slot mask should include the reported slot lifecycle event slot");
    ASSERT_TRUE((diagnostics.p10_m16_slot_readiness_dirty_slot_mask & 0x3u) != 0u,
                "M16 readiness dirty mask should include at least one fixed-double slot");
    ASSERT_EQUAL(diagnostics.p10_m16_slot_readiness_attention_slot_mask,
                 diagnostics.p10_m16_slot_readiness_ready_slot_mask |
                     diagnostics.p10_m16_slot_readiness_failed_slot_mask |
                     diagnostics.p10_m16_slot_readiness_invalidated_slot_mask,
                 "M16 attention mask should remain union of ready/failed/invalidated");
    ASSERT_TRUE(diagnostics.packed4_selected_layout_format != 0u,
                "packed4 diagnostics should remain visible after slot event commit churn");
    ASSERT_TRUE(diagnostics.fp16_tolerance_known != 0u, "fp16 diagnostics should remain visible after slot event commit churn");

    ASSERT_EQUAL(PROM_OK, prometheus_reactor_runtime_destroy(handle), "destroy should succeed");
}

FACT(PrometheusDominatusSlotAdapter_LastSlotEventSurvivesLaterNonSlotCommit)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);

    const prom_slot_metadata metadata = SlotMetadata(0u, 33u, 1u, 0);
    ASSERT_TRUE(prom_dom_slot_stage_lifecycle(&board,
                                              PROM_DOM_EVENT_SLOT_READY,
                                              0u,
                                              PROM_SLOT_READY,
                                              &metadata,
                                              1u,
                                              0u,
                                              1u,
                                              1u,
                                              0) == 1u,
                "slot stage should succeed");
    prom_dom_slot_commit(&board);

    ASSERT_TRUE(prom_dom_set_u32(&board,
                                 PROM_DOM_SOURCE_JUDGMENT,
                                 PROM_DOM_KEY_SGEMM_PACKED4_AVAILABLE,
                                 0u,
                                 1u,
                                 0) == 1u,
                "non-slot stage should succeed");
    prom_dom_commit(&board);

    prom_dom_slot_commit_snapshot snapshot{};
    ASSERT_TRUE(prom_dom_slot_read_last_commit(&board, 0u, &snapshot) == 1u, "slot snapshot should remain queryable");
    ASSERT_EQUAL(PROM_DOM_EVENT_SLOT_READY, snapshot.last_event.kind,
                 "slot snapshot should retain most recent committed slot lifecycle event after non-slot commit");
    ASSERT_EQUAL(0u, snapshot.last_event.slot_id, "slot snapshot should preserve slot id");
    ASSERT_TRUE((snapshot.last_commit_dirty_slot_mask & 0x1u) != 0u,
                "slot snapshot dirty mask should include slot touched by retained slot lifecycle event");
}

FACT(PrometheusDominatusSlotAdapter_RuntimeDiagStagedVisibleIsolation)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);

    prom_dom_slot_runtime_diag_snapshot a{};
    a.current_slot_id = 0u;
    a.next_slot_id = 1u;
    a.slot_state[0] = PROM_SLOT_READY;
    a.slot_state[1] = PROM_SLOT_EMPTY;
    a.slot_generation[0] = 3u;
    a.slot_generation[1] = 9u;
    a.slot_valid[0] = 1u;
    a.slot_valid[1] = 0u;
    a.swap_count = 7u;
    a.max_wip_depth = 2u;
    a.failure_slot_id = -1;
    ASSERT_TRUE(prom_dom_slot_stage_runtime_diag(&board, &a, 0) == 1u, "stage snapshot A should succeed");
    prom_dom_slot_commit(&board);

    prom_dom_slot_runtime_diag_snapshot visible{};
    ASSERT_TRUE(prom_dom_slot_read_visible_runtime_diag(&board, &visible) == 1u, "read visible A should succeed");
    ASSERT_EQUAL(7u, visible.swap_count, "visible should expose committed snapshot A");

    prom_dom_slot_runtime_diag_snapshot b = a;
    b.swap_count = 11u;
    b.current_slot_id = 1u;
    ASSERT_TRUE(prom_dom_slot_stage_runtime_diag(&board, &b, 0) == 1u, "stage snapshot B should succeed");
    ASSERT_TRUE(prom_dom_slot_read_visible_runtime_diag(&board, &visible) == 1u, "pre-commit visible read should still succeed");
    ASSERT_EQUAL(7u, visible.swap_count, "staged snapshot B must remain invisible before commit");

    prom_dom_slot_commit(&board);
    ASSERT_TRUE(prom_dom_slot_read_visible_runtime_diag(&board, &visible) == 1u, "post-commit visible read should succeed");
    ASSERT_EQUAL(11u, visible.swap_count, "snapshot B should become visible after commit");
    ASSERT_EQUAL(1u, visible.current_slot_id, "snapshot B current slot should become visible after commit");
}

FACT(PrometheusDominatusSlotAdapter_RuntimeDiagDirtyAndSameValueBehavior)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);

    prom_dom_slot_runtime_diag_snapshot snapshot{};
    snapshot.current_slot_id = 0u;
    snapshot.next_slot_id = 1u;
    snapshot.slot_state[0] = PROM_SLOT_PREPARING;
    snapshot.slot_state[1] = PROM_SLOT_READY;
    snapshot.slot_generation[0] = 1u;
    snapshot.slot_generation[1] = 2u;
    snapshot.slot_valid[0] = 1u;
    snapshot.slot_valid[1] = 1u;
    snapshot.overwrite_rejection_count = 5u;
    snapshot.failure_slot_id = -1;

    ASSERT_TRUE(prom_dom_slot_stage_runtime_diag(&board, &snapshot, 0) == 1u, "initial stage should succeed");
    ASSERT_TRUE((prom_dom_dirty_slots_staged(&board) & 0x3u) == 0x3u, "per-slot state writes should mark both slots dirty");
    prom_dom_slot_commit(&board);
    ASSERT_TRUE(prom_dom_dirty_key_last_commit(&board, PROM_DOM_KEY_SLOT_OVERWRITE_REJECTION_COUNT) == 1u,
                "counter key should be marked dirty after change");

    ASSERT_TRUE(prom_dom_slot_stage_runtime_diag(&board, &snapshot, 0) == 1u, "same-value stage should succeed");
    ASSERT_TRUE(prom_dom_dirty_key_staged(&board, PROM_DOM_KEY_SLOT_OVERWRITE_REJECTION_COUNT) == 0u,
                "same-value counter write must not restage dirty key");
}

FACT(PrometheusDominatusSlotAdapter_RuntimeDiagFailureVisibilityAfterCommit)
{
    prom_dom_blackboard board{};
    prom_dom_blackboard_init(&board);

    prom_dom_slot_runtime_diag_snapshot snapshot{};
    snapshot.current_slot_id = 0u;
    snapshot.next_slot_id = 1u;
    snapshot.slot_state[0] = PROM_SLOT_FAILED;
    snapshot.slot_state[1] = PROM_SLOT_EMPTY;
    snapshot.slot_generation[0] = 5u;
    snapshot.slot_generation[1] = 1u;
    snapshot.slot_valid[0] = 0u;
    snapshot.slot_valid[1] = 0u;
    snapshot.failure_slot_id = 0;
    snapshot.failure_reason = 404;

    ASSERT_TRUE(prom_dom_slot_stage_runtime_diag(&board, &snapshot, 404) == 1u, "failure snapshot stage should succeed");

    prom_dom_slot_runtime_diag_snapshot visible{};
    ASSERT_TRUE(prom_dom_slot_read_visible_runtime_diag(&board, &visible) == 0u,
                "failure slot diagnostics should remain invisible until first commit");

    prom_dom_slot_commit(&board);
    ASSERT_TRUE(prom_dom_slot_read_visible_runtime_diag(&board, &visible) == 1u,
                "failure slot diagnostics should become visible after commit");
    ASSERT_EQUAL(0, visible.failure_slot_id, "visible failure slot id should match committed snapshot");
    ASSERT_EQUAL(404, visible.failure_reason, "visible failure reason should match committed snapshot");
}
