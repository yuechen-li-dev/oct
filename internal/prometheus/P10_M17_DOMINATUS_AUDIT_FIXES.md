# P10 M17 — Dominatus Audit Fix Pass (Dirty-Key / Readiness / Ownership Contracts)

## 1) Audit summary (required first step)

### 1.1 What the audit confirmed as correct

The audit confirmed the current Dominatus migration shape is structurally sound:

- SGEMM layout/precision/path/buffering/transfer decision facts are staged/committed via Dominatus ownership boundaries.
- Slot lifecycle event staging/commit and M16 readiness dirty-mask coalescing are Dominatus-owned.
- Async lifecycle visibility is exported through Dominatus snapshots/diagnostics.
- M13/M15 selector caching contracts are generally correct and preserve staged→visible semantics.
- M16 readiness behavior (dirty/ready/failed/invalidated/attention accumulation) is functionally correct.

### 1.2 Layout/precision dependency-mask issue identified

The audit flagged that `layout_precision_path_guard_dirty_mask` was being built in `reactor_vulkan.c` using hardcoded bit indices (`1ull << 0u`, `1ull << 1u`, ...), implicitly coupled to ordering in `path_compute_dependency_mask_last_commit(...)`.

Risk: if path/compute dependency ordering changes, layout/precision guard selection could silently drift and cache invalidation behavior could become stale or over-eager.

M17 resolves this by using named dependency-bit constants shared with the path/compute dependency builder.

### 1.3 Readiness-counter behavior needing documentation

The audit noted `prom_dom_slot_readiness_clear_boundary(...)` correctly resets boundary-scoped masks, while intentionally preserving cumulative diagnostics counters:

- `overflow_spill_count`
- `duplicate_ready_event_count`
- `empty_boundary_commit_count`

This was behaviorally correct but under-documented, so M17 adds explicit API comments, implementation comments, and stronger test assertions proving this contract.

### 1.4 Runtime fields intentionally remaining legacy-owned

The audit identified categories that should stay runtime/direct-owned in M17:

1. **Controller integrator internals** (`rt->sgemm_controller.*`, pending waste units, policy-memory/integrator counters): these are implementation-local control state; Dominatus owns exported facts/decisions emitted from them.
2. **Init-time capability constants** (`rt->capability_fp16_storage`, `rt->software_vulkan`, `rt->dedicated_transfer_available`): these are stable runtime capabilities projected into Dominatus facts; no dirty tracking migration needed unless runtime hot-swap/device-mutation is introduced.
3. **Atomic async internals** (`rt->async_state`, `rt->async_task_id`, runtime slot ownership internals): these remain runtime-owned for atomic lifecycle correctness; Dominatus owns observability/snapshot exports.

M17 documents these ownership boundaries with targeted code comments and this report section.

### 1.5 Why memory suballocation should precede N-slot/work-stealing

The audit recommendation is retained: implement memory suballocation before N-slot/work-stealing because N-slot multiplies buffer artifact cardinality (per-slot A/B/C staging/device/readback variants), and naive per-buffer allocations would amplify allocation churn, fragmentation pressure, and memory-budget instability.

Suballocation first gives bounded allocation pressure, predictable reuse bins, and cleaner per-slot invalidation/retirement accounting before concurrency multiplies state.

## 2) Dependency-mask named constants fix

Implemented:

- Added named enum constants for path/compute dependency-bit contract in `reactor_dominatus_sgemm_adapter.h`:
  - `prom_dom_sgemm_path_compute_dependency_bit`
  - values `PROM_DOM_PATH_COMPUTE_DEP_SHAPE_M ... PROM_DOM_PATH_COMPUTE_DEP_POLICY_MODE`
- Updated `path_compute_dependency_mask_last_commit(...)` in `reactor_dominatus_sgemm_adapter.c` to set bits using those named constants.
- Updated `layout_precision_path_guard_dirty_mask` construction in `reactor_vulkan.c` to read only named path/compute dependency bits instead of hardcoded numeric indices.

Behavior is unchanged; only the mapping is made explicit and less fragile.

## 3) Readiness boundary vs lifetime counter semantics

Documented and tested semantics:

- Boundary-scoped (cleared by `prom_dom_slot_readiness_clear_boundary(...)`):
  - `boundary_generation` (increments)
  - `dirty_slot_mask`
  - `ready_slot_mask`
  - `failed_slot_mask`
  - `invalidated_slot_mask`
  - `attention_slot_mask`
- Lifetime counters (preserved across boundary clears):
  - `overflow_spill_count`
  - `duplicate_ready_event_count`
  - `empty_boundary_commit_count`

M17 adds:

- API contract comments in `reactor_dominatus_slot_adapter.h`.
- explicit implementation comment in `reactor_dominatus_slot_adapter.c`.
- strengthened test assertions in `reactor_dominatus_slot_adapter_tests.cpp` verifying both boundary-mask reset and lifetime-counter persistence.

## 4) Intentionally legacy-owned fields

M17 does not migrate additional runtime fields into Dominatus ownership.

Documented ownership split:

- **Remain runtime-owned in M17:**
  - controller integrator internals (`prom_sgemm_controller_state` and companion policy/integrator counters),
  - init-time capability constants,
  - atomic async runtime internals.
- **Dominatus-owned in M17:**
  - staged/visible facts used for selectors,
  - committed selector decisions,
  - slot lifecycle events and readiness visibility/diagnostics projections.

Migration preconditions for the deferred runtime-owned fields would require:

- explicit atomicity contract and write discipline for async internals,
- clear capability hot-swap model for currently static device facts,
- controller-integrator decomposition where Dominatus can own inputs/outputs without embedding integrator mechanics.

## 5) N-slot prerequisites (deferred)

N-slot/work-stealing remains explicitly deferred. Prerequisites remain:

1. slot-count generalization beyond fixed two-slot assumptions,
2. per-slot command buffers/fences/resource ownership,
3. queue/work-steal ring model with starvation/fairness guarantees,
4. lock/staged-write discipline for cross-slot blackboard access,
5. cross-slot dependency propagation for selection/invalidation,
6. per-slot buffer invalidation + retirement policy consistency,
7. memory suballocation substrate to absorb allocation cardinality growth.

## 6) Why memory suballocation comes before N-slot

Ordering rationale:

- N-slot multiplies active/staged artifacts and retirement windows.
- Without suballocation, allocator pressure and fragmentation become a confounding variable while validating scheduler correctness.
- Suballocation first makes memory behavior deterministic enough to isolate N-slot scheduling correctness and dependency-coalescing logic.

Therefore the recommended sequencing is:

1. suballocation + artifact-lifetime discipline,
2. then N-slot/work-stealing execution/synchronization expansion.

## 7) Tests and validation

M17 validation expectations:

1. M15 layout/precision selector cache tests pass.
2. M16 slot readiness tests pass.
3. full native suite passes.
4. readiness boundary-clear test now explicitly verifies lifetime counter persistence.

## 8) Inconsistency/documentation-gap note

As in M14/M15/M16 reports, Dominatus still does not expose first-class A/B/C artifact dependency keys as native key-catalog entries; M14/M17 continue to enforce artifact dependency contracts in runtime-native metadata and diagnostics while preserving behavior.
