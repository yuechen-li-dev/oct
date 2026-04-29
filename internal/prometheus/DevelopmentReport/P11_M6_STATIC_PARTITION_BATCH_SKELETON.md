# P11 M6 — Static Partition Batch Dispatch Skeleton (Native)

## 1) M40 handoff summary

M40 established the executable contract for first native batch dispatch as:

- central, single-threaded policy planning that builds immutable per-entry plans,
- effective worker selection capped by requested workers, hardware compute-queue capacity, and memory budget,
- static partitioning (default round-robin, optional contiguous),
- workers execute immutable plans only,
- ordered commit-by-entry-id output visibility,
- atomic failure with `PENDING -> RUNNING -> FAILING -> DRAINING -> FAILED` drain semantics,
- per-worker event rings with critical-overflow explicit failure,
- Dominatus separation (policy reads visible state, workers execute+emit, commit/drain stages updates).

## 2) API / skeleton scope

This milestone adds a narrow public batch surface in native reactor ABI:

- `PrometheusSgemmBatchEntry`
- `prometheus_reactor_runtime_sgemm_batch(...)`
- `PrometheusSgemmBatchDiagnostics`
- `prometheus_reactor_runtime_sgemm_batch_diagnostics(...)`

Scope is intentionally skeleton-first: a safe, auditable static-partition batch execution path with explicit diagnostics and failure semantics.

## 3) Dispatch plan model

M6 introduces immutable `prom_batch_plan` records containing:

- entry/worker identifiers,
- shape (`m/n/k`) + input/output pointers,
- selected path / compute / buffering / transfer decisions,
- layout/precision mode marker,
- required arena-bytes estimate,
- expected output elements,
- fixed `plan_generation = 40`,
- failure-policy marker.

Workers read only plan fields and execute them; they do not run judgment.

## 4) Policy / planning phase

Planning is centralized and single-threaded:

1. validate entry pointers/shapes,
2. compute work units,
3. run judgment engine selection (`layout+precision`, `path+compute`, buffering selector),
4. write immutable plan records,
5. allocate per-entry staged output buffers.

No worker-side policy recomputation is used.

## 5) Worker-count selection

Implemented effective-worker contract:

`effective = min(requested, hardware_queue_cap, memory_worker_cap)`

Where in M6 skeleton:

- `requested` comes from low 8 bits of `flags` (`0 => 1`),
- `hardware_queue_cap` is conservatively fixed to `1` (explicit conservative single-queue cap),
- `memory_worker_cap = floor(arena_budget_limit_bytes / per_worker_arena_bytes)` with `per_worker_arena_bytes = 64 MiB`.

If effective workers becomes zero, batch fails explicitly with `PROM_DETAIL_BATCH_ZERO_WORKERS`.

## 6) Partition policy

Implemented deterministic static policies:

- default round-robin: `worker_id = entry_id % effective_workers`,
- optional contiguous: `worker_id = floor(entry_id * effective_workers / entry_count)` (enabled via `PROM_BATCH_FLAG_PARTITION_CONTIGUOUS`).

Assignment is deterministic and total over entries.

## 7) Ordered output handling

M6 stages each plan output in per-entry temporary buffers and commits to caller `c` pointers only on full success.

- success: staged buffers copied to caller outputs in entry-id order,
- failure: no commit performed (`output_committed = 0`).

This enforces atomic caller-visible output behavior in the skeleton path.

## 8) Atomic failure / drain model

M6 tracks states:

- `PENDING`, `RUNNING`, `FAILING`, `DRAINING`, `FAILED`, `SUCCEEDED`.

First failure captures:

- failed entry id,
- failed worker id,
- failure stage,
- failure detail.

On failure, execution stops taking new work and transitions through drain to failed final state with uncommitted output.

## 9) Worker event rings

Per-worker fixed-capacity event rings are implemented in batch execution for:

- plan started,
- plan submitted,
- plan completed,
- plan failed (failure path marker),
- worker-drain accounting.

Critical overflow fails batch explicitly with `PROM_DETAIL_BATCH_EVENT_RING_OVERFLOW`; overflow count is exposed in diagnostics.

## 10) Dominatus integration

M6 keeps the Dominatus boundary intact:

- planning/judgment is centralized before execution,
- workers do not call `prom_dom_set_*`,
- workers do not run judgment,
- this skeleton surfaces lifecycle via batch diagnostics and event-ring accounting while preserving existing Dominatus paths for single SGEMM.

## 11) Tests added

Added Marionette tests:

1. round-robin batch success + diagnostics visibility,
2. invalid-plan failure is atomic and output remains uncommitted,
3. critical event-ring overflow fails explicitly and increments overflow diagnostics.

## 12) Deferred scope

Explicitly deferred beyond M6 skeleton:

- shared SPMC runtime queue,
- work stealing,
- MPMC/lock-free queue structures,
- parallel judgment,
- true N-slot stealing scheduler,
- advanced hardware multi-queue dispatch,
- performance tuning pass.

## Existing single-SGEMM compatibility

`prometheus_reactor_runtime_sgemm(...)` path and its semantics are unchanged by this milestone; M6 adds a separate batch entry point only.

## Inconsistency / documentation gap note

M40 calls for commit/drain-layer Dominatus staging from worker-ring events. M6 exposes worker-ring evidence via batch diagnostics and keeps worker-side blackboard mutation disallowed, but does not yet publish dedicated Dominatus batch event keys. This is an intentional visibility gap to close in a later milestone.
