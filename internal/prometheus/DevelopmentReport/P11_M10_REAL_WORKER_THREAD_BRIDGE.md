# P11 M10 — Real Worker Thread Bridge with Serialized Vulkan Execution

## 1) M41 handoff summary

M41 recommended candidate **B** as the first native concurrency step: real worker threads with a serialized Vulkan execution bridge. The recommendation explicitly required preserving M6/M7/M8 contracts: centralized immutable planning, worker no-judgment/no-Dominatus-mutation boundaries, per-worker event rings, first-failure-wins, drain-before-finalization, and ordered atomic output commit.

## 2) Implementation scope

M10 adds a controlled real-thread execution mode for batch dispatch when:

- `effective_workers > 1`, and
- runtime test capability `PROM_TESTCFG_P11_BATCH_ENABLE_REAL_THREADS` is enabled, and
- `PROM_TESTCFG_P11_BATCH_FORCE_LANE_SIMULATED` is not set.

Otherwise the M8 lane-simulated path remains active.

Follow-up alignment: enablement now matches this contract exactly (unrelated `test_flags` do not implicitly enable real-thread mode).

## 3) Worker thread model

Workers still execute immutable `prom_batch_plan` records produced centrally in the planning phase. M10 adds:

- native worker thread launch/join for each effective worker,
- per-worker thread context carrying only assigned execution inputs,
- shared failure state with mutex-protected first-failure ownership.

No worker-side policy selection is introduced.

## 4) Serialized Vulkan bridge

Actual plan SGEMM execution remains serialized through a dedicated bridge mutex:

- worker acquires serialized gate,
- executes SGEMM plan body,
- releases gate.

M10 publishes bridge diagnostics:

- `serialized_vulkan`,
- `serialized_execution_count`,
- `serialized_wait_count`,
- `max_concurrent_serialized_entries` (must remain `<= 1` in this mode),
- `hardware_parallelism_claimed` (forced false in serialized mode).

## 5) Worker touch boundaries

Workers are limited to:

- immutable plans,
- worker-local runtime state,
- worker-local event ring + counters,
- worker-local completion counters,
- per-entry staging output writes,
- serialized execution bridge entry.

Workers still do **not**:

- run judgment,
- mutate policy memory,
- call Dominatus mutation/commit APIs,
- reinterpret selected plan policy fields,
- borrow cross-worker arenas.

## 6) Batch state synchronization

M10 adds mutex-protected shared failure coordination:

- first failure transition from RUNNING to FAILING is single-winner,
- failure entry/worker/stage/detail preserve first writer,
- workers check shared state before starting new plans,
- in-flight workers finish current serialized section and then stop taking new work.

## 7) Event ring synchronization

Per-worker rings remain single-writer per worker. Join/drain still occurs after worker completion. Critical overflow remains batch-failing with explicit detail while preserving overflow accounting.

## 8) Output commit behavior

Output contract is unchanged:

- each entry computes into staged output,
- caller-visible `c` buffers commit only on full success,
- commit order remains entry-id order,
- failure preserves uncommitted caller outputs.

## 9) Diagnostics added

Batch diagnostics now expose:

- execution mode (`single_worker`, `lane_simulated_multi_worker`, `real_threads_serialized_vulkan`),
- lane worker count,
- real worker thread count,
- serialized Vulkan bridge flags/counters,
- max serialized bridge concurrency observed,
- explicit hardware parallelism claimed flag.

## 10) Tests added

Added M10 Marionette coverage for:

1. real-thread mode selection + serialized bridge diagnostics truthfulness,
2. no worker judgment in real-thread mode,
3. serialized bridge max concurrency `<= 1`,
4. first-failure preservation and no output commit in real-thread mode,
5. lane-simulated fallback override remains valid.

Existing M6/M7/M8 batch coverage remains in place.

## 11) Deferred scope

Still deferred exactly as M41 specified:

- true multi-queue Vulkan parallel execution,
- SPMC / MPMC shared queues,
- work stealing,
- lock-free queue structures,
- parallel judgment,
- cross-worker arena borrowing/shared arena lock modes,
- performance tuning,
- public batch event stream.

## Language/reference consistency note

This milestone changes native runtime + native Marionette coverage only; no `Language/reference` semantic contract changes were required.
