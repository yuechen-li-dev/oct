# P10 M11 — Full Slot Diagnostics Dominatus Ownership Migration

## 1) migration scope

M11 migrates the M29 fixed-double slot diagnostics export slice to Dominatus visible ownership while preserving:

- slot HFSM lifecycle legality authority in `reactor_slot_hfsm`
- fixed-double behavior and transition semantics
- M4 slot event bridge API/observability shape

This migration is ownership-only: Dominatus now owns exported slot diagnostics truth; runtime `slot_diag` remains compatibility mirror/scratch.

## 2) slot diagnostics migrated

Migrated to Dominatus-owned visible snapshot export:

- slot identity/state block:
  - `m29_current_slot_id`
  - `m29_next_slot_id`
  - `m29_slot0_state` / `m29_slot1_state`
  - `m29_slot0_generation` / `m29_slot1_generation`
  - `m29_slot0_valid` / `m29_slot1_valid`
- slot counters:
  - `m29_swap_count`
  - `m29_max_wip_depth`
  - `m29_overwrite_rejection_count`
  - `m29_stale_buffer_rejection_count`
  - `m29_shape_invalidation_count`
  - `m29_layout_invalidation_count`
  - `m29_capacity_invalidation_count`
  - `m29_inflight_rejection_count`
  - `m29_cleanup_success_count`
- failure diagnostics:
  - `m29_failure_slot_id`
  - `m29_failure_reason`

## 3) keys added/used

Reused M4 slot keys:

- `PROM_DOM_KEY_SLOT_STATE`
- `PROM_DOM_KEY_SLOT_GENERATION`
- `PROM_DOM_KEY_SLOT_VALID`
- `PROM_DOM_KEY_SLOT_CURRENT_ID`
- `PROM_DOM_KEY_SLOT_NEXT_ID`

Added M11 slot keys:

- `PROM_DOM_KEY_SLOT_SWAP_COUNT`
- `PROM_DOM_KEY_SLOT_MAX_WIP_DEPTH`
- `PROM_DOM_KEY_SLOT_OVERWRITE_REJECTION_COUNT`
- `PROM_DOM_KEY_SLOT_STALE_BUFFER_REJECTION_COUNT`
- `PROM_DOM_KEY_SLOT_SHAPE_INVALIDATION_COUNT`
- `PROM_DOM_KEY_SLOT_LAYOUT_INVALIDATION_COUNT`
- `PROM_DOM_KEY_SLOT_CAPACITY_INVALIDATION_COUNT`
- `PROM_DOM_KEY_SLOT_INFLIGHT_REJECTION_COUNT`
- `PROM_DOM_KEY_SLOT_CLEANUP_SUCCESS_COUNT`
- `PROM_DOM_KEY_SLOT_FAILURE_SLOT_ID`
- `PROM_DOM_KEY_SLOT_FAILURE_REASON_GLOBAL`

## 4) source-of-truth ownership model

Final ownership flow for migrated fields:

1. Runtime mutates slot HFSM + local counters as operational scratch.
2. Runtime stages full slot diagnostics snapshot through `prom_dom_slot_stage_runtime_diag(...)`.
3. Commit promotes staged slot diagnostics and slot event updates to visible state.
4. `prom_reactor_runtime_sgemm_policy_diagnostics_impl(...)` exports migrated M29 fields from `prom_dom_slot_read_visible_runtime_diag(...)`.
5. Runtime `slot_diag` is mirror-only for these fields (updated from visible snapshot in diagnostics export path), not export authority.

## 5) staged/visible behavior

M11 preserves staged/visible isolation:

- staged slot diagnostics writes remain invisible pre-commit
- commit makes staged diagnostics visible atomically
- same-value writes do not mark keys dirty
- per-slot state/generation/validity writes continue slot dirty-mask behavior via slot-scoped keys

## 6) event/commit visibility contract

M9 follow-up contract remains intact:

- SGEMM may issue multiple commits per call
- exported slot event projection cannot assume “last commit was slot”
- `prom_dom_slot_read_last_commit(...)` scans committed event history backward for latest slot lifecycle event
- exported M4 slot event fields (`p10_m4_*`) come from that retained slot event contract
- migrated M29 slot diagnostics come from visible state snapshot, not last-commit-only assumptions

## 7) diagnostics export behavior

`prom_reactor_runtime_sgemm_policy_diagnostics_impl(...)` now:

- reads migrated M29 fields from `prom_dom_slot_read_visible_runtime_diag(...)` when available
- mirrors visible values back into `rt->slot_diag` compatibility fields
- falls back to legacy fields only if visible snapshot read is unavailable

Public diagnostics struct shape remains unchanged.

## 8) compatibility mirror behavior

`rt->slot_diag` remains for runtime-local convenience and compatibility, but migrated exported fields are Dominatus-owned.

- visible snapshot read wins for export
- staged but uncommitted runtime writes do not leak into exported migrated fields
- direct legacy mirror drift cannot override exported migrated fields while visible snapshot exists

## 9) tests added

Added M11 adapter tests in `reactor_dominatus_slot_adapter_tests.cpp`:

1. runtime slot diagnostics staged/visible isolation across commit
2. per-slot staged state dirty tracking + same-value non-dirty behavior
3. failure slot/reason invisible pre-commit and visible post-commit

Existing M4 runtime smoke remains and now also covers continued coexistence with migrated diagnostics domains.

## 10) deferred scope for M12+

Explicitly deferred:

- async lifecycle ownership migration beyond current bridge/counters
- N-slot/work stealing
- decision caching
- memory suballocation migration
- FFT migration
- public external event stream API

## 11) inconsistency/documentation gap callout

`PROM_DOM_KEY_SLOT_FAILURE_REASON` (slot-scoped lifecycle metadata from M4) and
`PROM_DOM_KEY_SLOT_FAILURE_REASON_GLOBAL` (exported M29 failure reason) intentionally coexist.

This is a compatibility distinction between per-slot lifecycle reason and exported “last failure” reason semantics; the dual-key split should be documented in Dominatus key docs to avoid accidental conflation.
