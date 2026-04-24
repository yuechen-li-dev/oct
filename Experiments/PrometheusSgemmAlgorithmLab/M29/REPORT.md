# Prometheus SGEMM Algorithm Lab — M29

## 1) M27 + M28 synthesis used for implementation

- **M27 conclusion carried forward:** hybrid pull/double was not carried; fixed double buffering is the practical default because it stayed more robust under burst/jitter while keeping orchestration complexity bounded.
- **M28 implementation gate:** fixed double is safe only with explicit slot lifecycle, generation/validity guards, deterministic ready→current swap, mandatory shape/layout/capacity invalidation, async ownership ledger, failure isolation, and diagnostics.

## 2) What was implemented

M29 wires fixed double buffering directly into the native reactor (`reactor_vulkan.c`) with:

1. exactly two explicit slots (`slots[2]`) backed by `reactor_slot_hfsm`,
2. deterministic per-call `PREPARING -> READY -> CURRENT` handoff,
3. `CURRENT -> IN_FLIGHT -> CONSUMED -> EMPTY` completion path,
4. explicit failure capture via `FAILED` state and failure slot/reason diagnostics,
5. bounded WIP tracking and max-WIP observability,
6. per-slot metadata checks for shape/layout/precision/capacity compatibility.

## 3) How slot HFSM is used

Each slot uses `prom_slot_hfsm` as lifecycle authority. M29 routes lifecycle changes through:

- `prom_slot_hfsm_transition` for legal lifecycle edges,
- `prom_slot_hfsm_set_metadata` for slot generation/validity metadata,
- `prom_slot_hfsm_mark_invalidated` for stale/layout/capacity mismatch invalidation,
- `prom_slot_hfsm_fail` for failure isolation,
- `prom_slot_hfsm_cleanup` and guarded empty-reset for recovery.

No new ad hoc flag lifecycle was introduced for slot legality.

## 4) Judgment-engine vs policy-memory vs slot-HFSM boundaries

- **Judgment engine remains responsible** for path + compute-mode selection (`direct/staged`, baseline/tiled/packed4/fp16).
- **Policy memory remains responsible** for retained mode/cooldown/hysteresis updates.
- **Slot HFSM remains responsible** for slot lifecycle legality, swap/handoff legality, ownership state, and failure/cleanup progression.

M29 keeps these concerns separate; slot legality is not encoded inside policy mode logic.

## 5) Slot lifecycle and swap model

Fixed-double orchestration model implemented:

- slots are fixed to `0` and `1`,
- runtime tracks `current_slot_id` and `next_slot_id`,
- only `READY` can swap to `CURRENT`,
- swap increments a dedicated swap counter,
- in-flight/current overwrite attempts are rejected and counted,
- observed WIP depth is tracked and bounded (`max_wip_depth <= 2` in tests).

## 6) Invalidation rules enforced

Before slot `READY`:

- shape mismatch (`m/n/k`) invalidates,
- layout mismatch (path+compute mode code) invalidates,
- precision mismatch invalidates,
- required byte capacity growth invalidates.

Invalidation increments dedicated counters (shape/layout/capacity) and stale-rejection accounting.

## 7) Async ownership model

M29 adds explicit slot ownership ledger hooks:

- submit marks slot `IN_FLIGHT`,
- completion marks `CONSUMED -> EMPTY`,
- async failure marks slot `FAILED` with slot id + reason,
- abandon path performs explicit slot cleanup/release.

Double ownership conflicts are rejected and counted (overwrite/in-flight rejection counters).

## 8) Diagnostics added

`PrometheusSgemmPolicyDiagnostics` now includes M29 slot diagnostics:

- `m29_current_slot_id`, `m29_next_slot_id`,
- per-slot state, generation, validity,
- `m29_swap_count`, `m29_max_wip_depth`,
- overwrite/stale/shape/layout/capacity/inflight rejection counters,
- `m29_failure_slot_id`, `m29_failure_reason`.

## 9) Tests added

Marionette M29 tests were added in:

- `internal/prometheus/native/Marionette/reactor_m29_fixed_double_tests.cpp`

Coverage includes:

1. steady-flow swap and bounded WIP,
2. shape invalidation counter behavior,
3. async failure ownership + abandon cleanup/recovery path,
4. diagnostics truthfulness for slot counters/state/reason fields.

## 10) Intentionally deferred / out of scope

M29 intentionally does **not** implement:

- pull-lag/hybrid controller behavior,
- triple buffering,
- generalized runtime framework callbacks/mailboxes,
- new judgment candidates or GPU perf tuning passes.

This milestone remains narrow: fixed-double orchestration and safety guardrails only.
