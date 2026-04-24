#include "../reactor_slot_hfsm.h"
#include "test_harness.h"

#include <array>
#include <cstdint>

FACT(PrometheusSlotHfsm_LegalLifecycleSequence)
{
    prom_slot_hfsm machine{};
    prom_slot_hfsm_init(&machine, 7u);

    ASSERT_EQUAL(PROM_SLOT_EMPTY, prom_slot_hfsm_current_state(&machine), "slot machine should initialize at EMPTY");
    ASSERT_TRUE(prom_slot_hfsm_transition(&machine, PROM_SLOT_PREPARING) == 1u, "EMPTY -> PREPARING should be legal");
    ASSERT_TRUE(prom_slot_hfsm_transition(&machine, PROM_SLOT_READY) == 1u, "PREPARING -> READY should be legal");
    ASSERT_TRUE(prom_slot_hfsm_transition(&machine, PROM_SLOT_CURRENT) == 1u, "READY -> CURRENT should be legal");
    ASSERT_TRUE(prom_slot_hfsm_transition(&machine, PROM_SLOT_IN_FLIGHT) == 1u, "CURRENT -> IN_FLIGHT should be legal");
    ASSERT_TRUE(prom_slot_hfsm_transition(&machine, PROM_SLOT_CONSUMED) == 1u, "IN_FLIGHT -> CONSUMED should be legal");
    ASSERT_TRUE(prom_slot_hfsm_transition(&machine, PROM_SLOT_EMPTY) == 1u, "CONSUMED -> EMPTY should be legal");

    ASSERT_EQUAL(PROM_SLOT_EMPTY, prom_slot_hfsm_current_state(&machine), "lifecycle sequence should end at EMPTY");
    ASSERT_EQUAL(6u, prom_slot_hfsm_get_diagnostics(&machine)->transition_count, "legal sequence should count every transition");
}

FACT(PrometheusSlotHfsm_IllegalTransitionsRejectedDeterministically)
{
    prom_slot_hfsm machine{};
    prom_slot_hfsm_init(&machine, 1u);

    ASSERT_TRUE(prom_slot_hfsm_transition(&machine, PROM_SLOT_CURRENT) == 0u, "EMPTY -> CURRENT should be rejected");
    ASSERT_EQUAL(PROM_SLOT_EMPTY, prom_slot_hfsm_current_state(&machine), "illegal transition should not corrupt current state");

    ASSERT_TRUE(prom_slot_hfsm_transition(&machine, PROM_SLOT_PREPARING) == 1u, "EMPTY -> PREPARING should be legal");
    ASSERT_TRUE(prom_slot_hfsm_transition(&machine, PROM_SLOT_READY) == 1u, "PREPARING -> READY should be legal");
    ASSERT_TRUE(prom_slot_hfsm_transition(&machine, PROM_SLOT_IN_FLIGHT) == 0u, "READY -> IN_FLIGHT should be rejected");

    ASSERT_TRUE(prom_slot_hfsm_transition(&machine, PROM_SLOT_CURRENT) == 1u, "READY -> CURRENT should be legal");
    ASSERT_TRUE(prom_slot_hfsm_transition(&machine, PROM_SLOT_IN_FLIGHT) == 1u, "CURRENT -> IN_FLIGHT should be legal");
    ASSERT_TRUE(prom_slot_hfsm_transition(&machine, PROM_SLOT_CONSUMED) == 1u, "IN_FLIGHT -> CONSUMED should be legal");
    ASSERT_TRUE(prom_slot_hfsm_transition(&machine, PROM_SLOT_CURRENT) == 0u, "CONSUMED -> CURRENT without reset should be rejected");

    const prom_slot_hfsm_diagnostics* diagnostics = prom_slot_hfsm_get_diagnostics(&machine);
    ASSERT_EQUAL(3u, diagnostics->invalid_transition_count, "three illegal transitions should be counted");
    ASSERT_EQUAL(PROM_SLOT_CONSUMED, diagnostics->last_invalid_from, "last invalid from-state should be tracked");
    ASSERT_EQUAL(PROM_SLOT_CURRENT, diagnostics->last_invalid_to, "last invalid to-state should be tracked");
}

FACT(PrometheusSlotHfsm_FailurePathRequiresCleanup)
{
    prom_slot_hfsm machine{};
    prom_slot_hfsm_init(&machine, 2u);

    ASSERT_TRUE(prom_slot_hfsm_transition(&machine, PROM_SLOT_PREPARING) == 1u, "seed transition should be legal");
    ASSERT_TRUE(prom_slot_hfsm_fail(&machine, 91) == 1u, "any state should transition to FAILED");
    ASSERT_EQUAL(PROM_SLOT_FAILED, prom_slot_hfsm_current_state(&machine), "fail helper should move machine to FAILED");

    ASSERT_TRUE(prom_slot_hfsm_transition(&machine, PROM_SLOT_PREPARING) == 0u, "FAILED should reject normal lifecycle transitions before cleanup");
    ASSERT_TRUE(prom_slot_hfsm_cleanup(&machine) == 1u, "FAILED should allow cleanup to EMPTY");
    ASSERT_EQUAL(PROM_SLOT_EMPTY, prom_slot_hfsm_current_state(&machine), "cleanup should return machine to EMPTY");
    ASSERT_TRUE(prom_slot_hfsm_transition(&machine, PROM_SLOT_PREPARING) == 1u, "normal lifecycle should resume after cleanup");

    const prom_slot_hfsm_diagnostics* diagnostics = prom_slot_hfsm_get_diagnostics(&machine);
    ASSERT_EQUAL(1u, diagnostics->failure_count, "failure transitions should be counted");
    ASSERT_EQUAL(1u, diagnostics->cleanup_count, "cleanup transitions should be counted");
}

FACT(PrometheusSlotHfsm_StackBehaviorBoundedAndDeterministic)
{
    prom_slot_hfsm machine{};
    prom_slot_hfsm_init(&machine, 5u);

    ASSERT_TRUE(prom_slot_hfsm_push_state(&machine, PROM_SLOT_PREPARING) == 1u, "push should work inside bounds");
    ASSERT_TRUE(prom_slot_hfsm_replace_state(&machine, PROM_SLOT_READY) == 1u, "replace should update top of stack");
    ASSERT_TRUE(prom_slot_hfsm_contains(&machine, PROM_SLOT_READY) == 1u, "contains should observe replaced top state");
    ASSERT_TRUE(prom_slot_hfsm_pop_state(&machine) == 1u, "pop should restore prior state when depth > 1");
    ASSERT_TRUE(prom_slot_hfsm_pop_state(&machine) == 0u, "underflow should be rejected at base depth");

    for (std::uint32_t i = prom_slot_hfsm_depth(&machine); i < PROM_SLOT_HFSM_MAX_DEPTH; ++i) {
        ASSERT_TRUE(prom_slot_hfsm_push_state(&machine, PROM_SLOT_PREPARING) == 1u, "push should fill stack until max depth");
    }

    ASSERT_TRUE(prom_slot_hfsm_push_state(&machine, PROM_SLOT_READY) == 0u, "overflow push should be rejected deterministically");
    ASSERT_EQUAL(PROM_SLOT_HFSM_MAX_DEPTH, prom_slot_hfsm_get_diagnostics(&machine)->max_stack_depth_reached, "max stack depth should be tracked");
}

FACT(PrometheusSlotHfsm_DiagnosticsAndDeterminism)
{
    auto run_sequence = []() {
        prom_slot_hfsm machine{};
        prom_slot_hfsm_init(&machine, 3u);
        prom_slot_hfsm_transition(&machine, PROM_SLOT_PREPARING);
        prom_slot_hfsm_transition(&machine, PROM_SLOT_READY);
        prom_slot_hfsm_transition(&machine, PROM_SLOT_IN_FLIGHT);
        prom_slot_hfsm_fail(&machine, 44);
        prom_slot_hfsm_cleanup(&machine);
        return machine;
    };

    const prom_slot_hfsm first = run_sequence();
    const prom_slot_hfsm second = run_sequence();

    const prom_slot_hfsm_diagnostics* firstDiagnostics = prom_slot_hfsm_get_diagnostics(&first);
    const prom_slot_hfsm_diagnostics* secondDiagnostics = prom_slot_hfsm_get_diagnostics(&second);

    ASSERT_EQUAL(prom_slot_hfsm_current_state(&first), prom_slot_hfsm_current_state(&second), "same sequence should end in same state");
    ASSERT_EQUAL(firstDiagnostics->transition_count, secondDiagnostics->transition_count, "same sequence should produce same transition count");
    ASSERT_EQUAL(firstDiagnostics->invalid_transition_count, secondDiagnostics->invalid_transition_count, "same sequence should produce same invalid transition count");
    ASSERT_EQUAL(firstDiagnostics->last_invalid_from, secondDiagnostics->last_invalid_from, "same sequence should preserve last invalid from-state");
    ASSERT_EQUAL(firstDiagnostics->last_invalid_to, secondDiagnostics->last_invalid_to, "same sequence should preserve last invalid to-state");
    ASSERT_EQUAL(1u, firstDiagnostics->invalid_transition_count, "sequence should contain exactly one invalid transition");
}

FACT(PrometheusSlotHfsm_MetadataShapeForM29)
{
    prom_slot_hfsm machine{};
    prom_slot_hfsm_init(&machine, 11u);

    prom_slot_metadata metadata{};
    metadata.slot_id = 11u;
    metadata.generation = 8u;
    metadata.valid = 1u;
    metadata.shape = prom_slot_shape_metadata{64u, 128u, 32u};
    metadata.layout = prom_slot_layout_metadata{2u, 3u};
    metadata.required_capacity_bytes = 8192u;
    metadata.failure_reason = 0;

    prom_slot_hfsm_set_metadata(&machine, &metadata);
    const prom_slot_metadata* current = prom_slot_hfsm_metadata(&machine);
    ASSERT_EQUAL(11u, current->slot_id, "metadata should expose slot id for M29 slot ownership checks");
    ASSERT_EQUAL(8u, current->generation, "metadata should expose generation for stale-slot detection");
    ASSERT_EQUAL(8192u, current->required_capacity_bytes, "metadata should expose required capacity for shape invalidation");

    prom_slot_hfsm_mark_invalidated(&machine, 123);
    current = prom_slot_hfsm_metadata(&machine);
    ASSERT_EQUAL(0u, current->valid, "invalidated metadata should clear valid flag");
    ASSERT_EQUAL(123, current->failure_reason, "invalidated metadata should retain failure reason");
}
