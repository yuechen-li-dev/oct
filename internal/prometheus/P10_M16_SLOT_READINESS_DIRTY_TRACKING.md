# P10 M16 — Slot Readiness Dirty-Mask Tracking + Boundary Coalescing

## 1) M36 handoff summary

M36 Track C required per-boundary slot readiness tracking prior to N-slot/work-stealing:

- `dirty_slot_mask`
- `ready_slot_mask`
- `failed_slot_mask`
- `invalidated_slot_mask`
- `attention_slot_mask = ready ∪ failed ∪ invalidated`
- boundary coalescing across multiple commits in one SGEMM call

M16 implements that contract in Dominatus slot adapter ownership, without changing slot HFSM legality, fixed-double execution behavior, async lifecycle behavior, or Vulkan dispatch behavior.

## 2) Readiness model

M16 adds a boundary-scoped readiness accumulator on `prom_dom_blackboard`:

- `slot_readiness_boundary_generation`
- `slot_readiness_dirty_slot_mask`
- `slot_readiness_ready_slot_mask`
- `slot_readiness_failed_slot_mask`
- `slot_readiness_invalidated_slot_mask`
- `slot_readiness_attention_slot_mask`
- `slot_readiness_overflow_spill_count`
- `slot_readiness_duplicate_ready_event_count`
- `slot_readiness_empty_boundary_commit_count`

The adapter exposes:

- `prom_dom_slot_readiness_read_visible(...)`
- `prom_dom_slot_readiness_clear_boundary(...)`

## 3) Transitions tracked

Tracked readiness-relevant slot lifecycle transitions are derived from existing Dominatus slot events (no parallel flag system):

- `PREPARING -> READY` via `PROM_DOM_EVENT_SLOT_READY`
- `CURRENT / IN_FLIGHT -> CONSUMED/complete` via `PROM_DOM_EVENT_SLOT_COMPLETE` and `PROM_DOM_EVENT_SLOT_CONSUMED`
- `* -> FAILED` via `PROM_DOM_EVENT_SLOT_FAILED`
- `CLEANUP -> EMPTY` via `PROM_DOM_EVENT_SLOT_CLEANUP`
- explicit invalidation via `PROM_DOM_EVENT_SLOT_INVALIDATED`
- additional mapped slot events (`PREPARED`, `PROMOTED_CURRENT`, `SUBMITTED`) participate in dirty/ready-state coalescing.

## 4) Coalescing semantics

Coalescing is boundary-scoped and commit-accumulating:

1. `prom_dom_slot_commit(...)` records prior committed-event count.
2. It executes `prom_dom_commit(...)` (existing staged→visible promotion).
3. It scans only newly committed events and folds slot lifecycle changes into readiness masks.

Rules:

- dirty mask accumulates all touched slots (`dirty |= slot`), across multiple commits.
- `READY` sets `ready` and `attention`.
- duplicate `READY` for same slot increments duplicate counter; mask remains single-bit (coalesced).
- `FAILED` clears `ready`, sets `failed`, keeps `attention`.
- `INVALIDATED` clears `ready`, sets `invalidated`, keeps `attention`.
- `PREPARED/PROMOTED_CURRENT/SUBMITTED/COMPLETE/CONSUMED` clear `ready` for the slot.
- `CLEANUP` clears `ready`, `failed`, `invalidated` for that slot, and recomputes `attention` as union.
- non-slot commits do not modify readiness masks.

Boundary advance/reset:

- `prom_dom_slot_readiness_clear_boundary(...)` increments boundary generation and clears boundary masks.
- counters (`overflow`, duplicate-ready, empty-boundary-commit) are retained as diagnostics accumulators.

## 5) Data structures and scalability note

Current implementation uses fixed 32-bit masks (`slot_id < 32`).

Future-ready behavior added now:

- if a tracked slot event arrives with `slot_id >= 32`, the event is counted in `slot_readiness_overflow_spill_count`.

Deferred for N-slot:

- actual spill-list storage of out-of-mask slot IDs.
- widened mask/word-set ABI.

## 6) Diagnostics added

`PrometheusSgemmPolicyDiagnostics` now exports:

- `p10_m16_slot_readiness_boundary_generation`
- `p10_m16_slot_readiness_dirty_slot_mask`
- `p10_m16_slot_readiness_ready_slot_mask`
- `p10_m16_slot_readiness_failed_slot_mask`
- `p10_m16_slot_readiness_invalidated_slot_mask`
- `p10_m16_slot_readiness_attention_slot_mask`
- `p10_m16_slot_readiness_overflow_spill_count`
- `p10_m16_slot_readiness_duplicate_ready_event_count`
- `p10_m16_slot_readiness_empty_boundary_commit_count`

## 7) Tests added

Added focused Marionette tests in `reactor_dominatus_slot_adapter_tests.cpp`:

1. single ready transition mask coverage
2. two slots ready in same boundary
3. ready→failed dominance
4. invalidated attention survives non-slot commit
5. failed cleanup-to-empty behavior
6. duplicate ready coalescing + boundary clear generation advance
7. runtime fixed-double smoke now checks M16 readiness diagnostics and attention-union invariant

Existing M4/M11 slot adapter tests remain and pass under the same flow.

## 8) Current two-slot compatibility

M16 is observability/coalescing only.

No changes were made to:

- slot HFSM legal transitions
- fixed-double slot ownership logic
- async ownership/lifecycle semantics
- Vulkan command/dispatch path behavior

Current two-slot runtime behavior remains unchanged; M16 only adds better readiness attention visibility.

## 9) Future N-slot/work-stealing usage

M16 prepares scheduler-scale work by enabling boundary-local sparse inspection:

- schedulers can inspect `attention_slot_mask` (and `dirty_slot_mask`) instead of polling all slots.
- terminal conditions (`failed`, `invalidated`) are preserved across intermediate commits until boundary clear.
- duplicate-ready emission is coalesced to single-slot attention bits.

## 10) Deferred scope

Still deferred (explicit):

- N-slot/work-stealing scheduler implementation
- concurrent queue stealing
- dynamic spill-list allocation
- public external event stream API
- async scheduler redesign
- FFT readiness migration

## 11) Inconsistency/documentation-gap callouts

1. M36 recommends dirty-mask + optional spill-list. M16 currently implements overflow counting but not persisted spill-list slot IDs; this is intentionally deferred to N-slot ABI work.
2. Current Dominatus slot readiness accumulation is driven by `prom_dom_slot_commit(...)` (slot adapter boundary), while generic `prom_dom_commit(...)` remains domain-agnostic. This preserves current behavior but should be documented as a deliberate ownership boundary for slot-readiness accumulation.
