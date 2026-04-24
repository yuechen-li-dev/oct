# P10 M4 — Slot HFSM Event Bridge

## 1) Bridge scope

M4 adds a narrow Dominatus slot adapter that records existing slot HFSM lifecycle transitions as staged Dominatus key updates + events, then commits them through the same staged/visible promotion boundary used in M2/M3.

This milestone does **not** redesign slot lifecycle rules. `reactor_slot_hfsm` remains transition authority; the adapter only records transitions and ownership-relevant facts.

## 2) Files/modules added

Added:

- `internal/prometheus/native/reactor_dominatus_slot_adapter.h`
- `internal/prometheus/native/reactor_dominatus_slot_adapter.c`
- `internal/prometheus/native/Marionette/reactor_dominatus_slot_adapter_tests.cpp`
- `internal/prometheus/P10_M4_SLOT_HFSM_EVENT_BRIDGE.md`

Updated integration:

- `internal/prometheus/native/reactor_vulkan.c`
- `internal/prometheus/native/reactor_dominatus_blackboard.h`
- `internal/prometheus/native/reactor_dominatus_blackboard.c`
- `internal/prometheus/native/reactor_api.h`
- `internal/prometheus/native/build_stub.sh`
- `internal/prometheus/native/build_windows.cmd`

## 3) Slot events mapped

Mapped bridge events:

- slot prepared → `PROM_DOM_EVENT_SLOT_PREPARED`
- slot ready → `PROM_DOM_EVENT_SLOT_READY`
- slot promoted/swapped current → `PROM_DOM_EVENT_SLOT_PROMOTED_CURRENT`
- slot submitted/in-flight → `PROM_DOM_EVENT_SLOT_SUBMITTED`
- slot completed (GPU completion transition to consumed) → `PROM_DOM_EVENT_SLOT_COMPLETE`
- slot consumed (consumed → empty release) → `PROM_DOM_EVENT_SLOT_CONSUMED`
- slot failed → `PROM_DOM_EVENT_SLOT_FAILED`
- slot cleanup → `PROM_DOM_EVENT_SLOT_CLEANUP`
- slot invalidated → `PROM_DOM_EVENT_SLOT_INVALIDATED`

## 4) Dirty keys/slots marked

For each staged lifecycle record, adapter stages:

- `PROM_DOM_KEY_SLOT_STATE` (slot-scoped)
- `PROM_DOM_KEY_SLOT_GENERATION` (slot-scoped)
- `PROM_DOM_KEY_SLOT_VALID` (slot-scoped)
- `PROM_DOM_KEY_SLOT_FAILURE_REASON` (slot-scoped in M4)
- `PROM_DOM_KEY_SLOT_CURRENT_ID` (global slot domain key when swap/current changes provided)
- `PROM_DOM_KEY_SLOT_NEXT_ID` (global slot domain key when handoff/next changes provided)

Dirty slot masks are produced by blackboard key-diff logic and promoted into `dirty_slots_last_commit` at commit.

## 5) Staged/visible semantics

M4 preserves M2 semantics:

- adapter writes stage into blackboard staged values + staged event ring
- visible reads remain unchanged pre-commit
- commit promotes staged values/events and dirty slot masks
- staged dirty/event state is cleared after commit

Adapter tests explicitly verify staged-invisible/committed-visible behavior for events and dirty slot masks.

## 6) Runtime integration points

Narrow runtime bridge hooks were added where slot HFSM transitions already occur:

- prepare path (`PROM_SLOT_PREPARING` / `PROM_SLOT_READY`)
- swap to current (`PROM_SLOT_CURRENT`)
- submit/in-flight (`PROM_SLOT_IN_FLIGHT`)
- completion path (`PROM_SLOT_CONSUMED` then `PROM_SLOT_EMPTY`)
- failure (`PROM_SLOT_FAILED`)
- cleanup-to-empty helper
- invalidation marker path

No legal-transition table changes were made in `reactor_slot_hfsm`.

## 7) Tests added

New M4 tests (`reactor_dominatus_slot_adapter_tests.cpp`) cover:

1. staged event invisibility before commit + visibility after commit
2. dirty slot tracking (target slot set, unrelated slot not set)
3. representative lifecycle sequence ordering and dirty key behavior
4. failure + cleanup event/trace behavior with reason metadata
5. runtime SGEMM smoke asserting committed slot event + dirty slot mask through diagnostics

Compatibility expectation:

- existing M29 fixed-double tests still pass
- existing M35 buffering selector tests still pass

## 8) Behavior intentionally unchanged

- slot HFSM legal transition authority and sequencing
- fixed-double ownership behavior
- Vulkan submission/dispatch semantics
- scheduler and async policy behavior
- no N-slot ownership expansion

## 9) Deferred scope for M5+

Deferred intentionally:

- full migration of all slot diagnostic ownership to Dominatus visible state
- replacing `slot_diag` with blackboard-first ownership for all M29 fields
- broader queue/transfer event migration and cross-domain correlation
- N-slot orchestration and concurrency expansion
- public external event stream API

## 10) Inconsistency/documentation gap callout

M4 required slot failure reason to be dirty-tracked per slot. Prior M2/M3 key metadata treated `PROM_DOM_KEY_SLOT_FAILURE_REASON` as non-slot-scoped. M4 updates this key to slot-scoped so dirty-slot semantics match lifecycle ownership requirements.
