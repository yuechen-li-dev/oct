# P10 M2 — Dominatus Blackboard Core

## 1) M1 architecture pieces implemented in M2

M2 implements the blackboard substrate defined in `P10_M1_DOMINATUS_SUBSYSTEM_DESIGN.md`:

- Explicit visible/staged state split (`prom_dom_blackboard.visible_values` vs `staged_values`).
- Typed domain and key schema with stable enum IDs (`prom_dom_domain`, `prom_dom_key`).
- Typed staged setters and visible getters (`u32/u64/i32/i64/bool`).
- Dirty tracking for key/domain/slot scopes with staged + last-commit masks.
- Commit boundary semantics (`prom_dom_commit`) that promote staged->visible atomically for a generation and clear staged dirty/event state.
- Fixed-capacity staged event ring and committed event window.
- Fixed-capacity trace ring with deterministic overwrite behavior.
- Deterministic init/reset helpers.

## 2) Modules/files added

- `internal/prometheus/native/reactor_dominatus_blackboard.h`
- `internal/prometheus/native/reactor_dominatus_blackboard.c`
- `internal/prometheus/native/Marionette/reactor_dominatus_blackboard_tests.cpp`
- `internal/prometheus/P10_M2_DOMINATUS_BLACKBOARD_CORE.md`

Build integration updates:

- `internal/prometheus/native/build_stub.sh`
- `internal/prometheus/native/build_windows.cmd`

## 3) Key/domain schema implemented

Implemented domains:

- SGEMM
- SLOT
- QUEUE
- MEMORY
- DIAGNOSTICS
- FFT (reserved)

Implemented representative keys include:

- SGEMM shape/layout/precision/path/compute/buffering/fallback reason
- Slot state/generation/valid/current id/next id/failure reason
- Queue compute family/transfer family/dedicated availability/transfer policy/handoff count
- Memory required capacity/budget/headroom/invalidation flags
- Diagnostics reason/counter/last transition
- FFT reserved key

All keys are stable enum constants (no string lookup).

## 4) Visible/staged storage model

Storage is fixed-capacity and C-friendly:

- dual value arrays for visible + staged key cells
- generation counters: visible + staged-next generation
- staged/last-commit dirty masks
- staged event ring + committed event ring window
- trace ring

No heap allocation is used.

## 5) Dirty tracking behavior

- Setters only write staged state.
- Dirty keys are tracked when staged differs from visible.
- Dirty domains/slots are recomputed deterministically from dirty key diffs.
- Same-value writes are a no-op and do not dirty masks.
- Commit copies staged dirty masks into last-commit masks and clears staged masks.

## 6) Commit semantics

`prom_dom_commit` does:

1. increment visible generation and advance staged generation
2. promote staged values to visible values
3. publish staged dirty masks as last-commit dirty masks
4. clear staged dirty masks
5. promote staged events into committed event window with commit generation
6. clear staged event buffer

The M1 anti-tearing invariant is preserved:

- pre-commit visible getters do not see staged writes
- post-commit visible getters read committed values

## 7) Event ring behavior

- Fixed-capacity staged ring accepts ownership/policy/fallback style events.
- Events are invisible to committed readers until commit.
- Commit publishes staged events into a fixed committed window.
- Window wraps deterministically when full.

## 8) Trace ring behavior

- Value-changing setters emit trace entries with source/domain/key/old/new/reason/slot metadata.
- Event staging emits trace entries with event metadata.
- Ring is fixed-capacity and overwrites oldest entries deterministically on overflow.

## 9) Tests added

`reactor_dominatus_blackboard_tests.cpp` adds coverage for:

1. staged writes not visible before commit
2. dirty key/domain tracking and staged dirty clear on commit
3. dirty slot mask tracking
4. same-value write non-dirty behavior
5. event staging visibility and commit promotion
6. trace ring entry population and deterministic wrap
7. generation counter behavior across commits
8. deterministic reset behavior

## 10) Deferred scope for M3+

Intentionally deferred (per M2 scope):

- SGEMM behavior/diagnostics migration to blackboard adapters
- slot HFSM bridge migration to emit production blackboard events
- judgment engine visible-snapshot consumption wiring
- diagnostics API export from blackboard visible snapshot
- N-slot/work-stealing, mailbox model, replay/persistence, or generic Dominatus runtime

## 11) Prometheus-local boundary (not generic runtime)

This core remains Prometheus-local because:

- key/domain schema is explicitly SGEMM/slot/queue/memory/diagnostics shaped
- event kinds and source taxonomy match Prometheus reactor concerns
- capacities and storage model are fixed to current Prometheus needs
- no generic plugin kernel, actor dispatch, or persistence interfaces were introduced

## 12) Inconsistency/documentation gap check

- No direct inconsistency with M1 was found for the implemented substrate.
- M2 currently adds the substrate without wiring existing runtime structs through it yet; this gap is intentional and deferred to M3+.
