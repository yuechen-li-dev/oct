# P11 M8 — Native Multi-Worker Static Partition Execution

## 1) M6/M7 handoff summary

M6 added the first native static-partition batch skeleton with:

- immutable per-entry plans (`plan_generation = 40`),
- centralized policy/planning,
- static partition assignment (round-robin default, contiguous optional),
- per-worker event rings,
- ordered output staging and atomic commit-on-success,
- failure/drain state transitions,
- conservative worker caps and diagnostics.

M7 hardened that skeleton with:

- single-entry equivalence coverage,
- partition determinism coverage,
- worker cap reason truthfulness,
- zero-worker memory failure behavior,
- late failure no-partial-commit coverage,
- event-drain seam explicitness,
- explicit no-worker-judgment/no-Dominatus-mutation assertions.

## 2) M8 implementation scope

M8 moves from static accounting to real multi-worker static-partition execution lanes when `effective_workers > 1`.

Implemented in scope:

- explicit worker-state objects,
- assignment-aware multi-lane execution loop,
- per-worker event emission and accounting,
- per-worker assigned/completed/event diagnostics,
- truthful execution-mode diagnostics (single-worker vs lane-simulated),
- retained atomic commit and fail/drain semantics.

## 3) Worker model

Each worker has explicit local state:

- `worker_id`,
- `assigned_count`,
- `completed_count`,
- `event_count` (diagnostics publication via ring count),
- local scan cursor over immutable plans,
- local `active` flag,
- local failure-observed/failure detail fields,
- explicit worker resource mode marker.

Workers execute only immutable plans assigned by central policy partitioning.

## 4) Worker count / cap behavior

M8 preserves cap calculation and reason publication:

```text
effective_workers = min(requested_workers, hardware_queue_cap, memory_worker_cap)
```

Behavior:

- `effective_workers <= 1` => single-worker execution mode,
- `hardware_queue_cap == 1` remains conservative single-queue degradation,
- memory cap can reduce workers,
- zero workers fails explicitly with `PROM_DETAIL_BATCH_ZERO_WORKERS`.

Deterministic Marionette hooks remain for hardware cap override, arena-byte scale, event ring capacity, and targeted failure entry injection.

## 5) Partition behavior

M8 preserves M7 policy behavior:

- round-robin default: `entry_id % effective_workers`,
- contiguous optional: `floor(entry_id * effective_workers / entry_count)`.

Assignments are deterministic and consumed by the same lane execution logic for 1, 2, 4, etc. workers.

## 6) Execution model

M8 implements **lane-simulated multi-worker execution** for `effective_workers > 1`:

- worker-local state is independent,
- workers are stepped in lane rounds,
- each worker executes only its assigned immutable plans,
- failure observation prevents further plan starts.

This is intentionally not claimed as hardware-parallel queue execution.

Diagnostic field `execution_mode` explicitly reports:

- `PROM_BATCH_EXECUTION_SINGLE_WORKER`, or
- `PROM_BATCH_EXECUTION_LANE_SIMULATED`.

## 7) Ordered output and failure behavior

M8 preserves M7 output and failure contracts:

- per-entry staged output buffers,
- caller-visible commit only on full success,
- commit order by entry id,
- no partial commit on any failure,
- first failure entry/worker/stage/detail recorded,
- fail path drains worker rings and ends in `FAILED`.

## 8) Event-ring behavior

M8 preserves one ring per worker and extends per-worker accounting publication.

Lifecycle events include:

- plan started/submitted/completed,
- worker idle,
- batch failure observed,
- worker drained.

Critical ring overflow remains explicit batch failure (`PROM_DETAIL_BATCH_EVENT_RING_OVERFLOW`).

Drain seam remains explicit and continues to classify diagnostics-only vs Dominatus-deferred event categories.

## 9) Per-worker arena / slot ownership seams

M8 makes worker ownership explicit while remaining conservative:

- worker resource mode is published as `PROM_BATCH_WORKER_RESOURCE_SIMULATED`,
- no cross-worker arena borrowing,
- no cross-worker slot stealing,
- memory worker cap still computed from per-worker arena footprint.

Current runtime does not introduce dedicated per-worker Vulkan arena banks yet; this is intentionally deferred.

## 10) Dominatus integration status

M8 preserves the M6/M7 boundary:

- policy reads visible state and builds immutable plans centrally,
- workers do not run judgment,
- workers do not write Dominatus directly,
- event drain seam remains the batch lifecycle ownership boundary.

Deferred Dominatus batch event publication is still explicit.

## 11) Tests added / updated

Added M8-focused Marionette coverage for:

1. multi-worker (>1) lane execution diagnostics and assignment/completion accounting,
2. contiguous multi-worker failure path preserving deterministic worker mapping and atomic no-commit behavior.

Existing M6/M7 tests remain and were re-run.

## 12) What remains deferred

Still deferred by design:

- native worker threads,
- true multi-queue hardware parallel submission,
- shared SPMC queue runtime,
- MPMC / lock-free queues,
- work stealing,
- dynamic worker pool resizing,
- N-slot stealing scheduler,
- cross-worker arena borrowing,
- public batch event stream,
- performance tuning pass.

## 13) Inconsistency / golden-path avoidance notes

- M8 avoids a brittle hardcoded one-worker execution loop by introducing explicit per-worker state and lane stepping that works for arbitrary static partition counts.
- M8 remains truthful: diagnostics call out lane-simulated execution and simulated worker resource ownership instead of claiming hardware parallel speedup.
- Hardware queue discovery remains conservative (`1` unless deterministic test override is used), which is intentional until multi-queue runtime integration is implemented.
