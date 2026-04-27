# P11 M9 / Prometheus SGEMM Algorithm Lab M41 — Real Worker Execution Readiness Lab

## 1) M8 handoff summary (required first step)

### What M8 implemented

M8 established a hardened multi-lane execution skeleton with centralized planning and immutable dispatch plans, plus per-worker runtime state, lane-simulated static partitioning, per-worker diagnostics/event accounting, ordered output commit, and atomic failure/drain behavior.

### What remains lane-simulated

M8 still executes all worker lanes in one host thread model. It does not exercise true native worker synchronization, true concurrent event-ring writes, real thread visibility boundaries, or true multi-queue queue-submission topology behavior.

### Why true workers are the next risk boundary

The next correctness boundary is memory ownership under concurrent worker execution: thread-touch rules, event-ring pressure behavior, first-failure visibility propagation, drain sequencing, and Dominatus staged/visible boundary correctness under true concurrency.

### Why work stealing / SPMC remain deferred

Work stealing/SPMC are throughput topology upgrades, not required to validate minimum-safe worker correctness contracts. M41 therefore treats them as reference-only scope and keeps them explicitly deferred.

### What M41 decides before native implementation

M41 decides whether Prometheus is ready for real workers, chooses the smallest safe first native candidate, and codifies non-negotiable worker/event/failure/ordering/arena/queue contracts for P11 M10.

---

## 2) Candidate execution models

M41 compares:

- **A** lane-simulated workers (M8 baseline),
- **B** real worker threads + serialized Vulkan execution bridge,
- **C** real worker threads + per-worker queue/resource path,
- **D** real worker threads + shared arena bank (lock required),
- **E** work-stealing/SPMC reference row (deferred).

---

## 3) Model variables

The executable model includes deterministic integer variables for:

- batch size,
- worker count,
- hardware queue count,
- memory budget,
- per-worker arena bytes,
- per-worker event-ring capacity,
- event drain interval,
- dispatch duration distribution,
- critical and diagnostics events per task,
- failure mode/injected failure entries,
- mutex/lock cost,
- queue-submit serialization cost,
- Dominatus commit/drain cost,
- output staging bytes.

---

## 4) Hazard / rake findings

1. **Thread-safety boundary:** any worker policy-memory mutation or direct Dominatus write is flagged as illegal access and forces failure.
2. **Event ring pressure:** critical overflow hard-fails the batch; diagnostics overflow is counted as lossy pressure.
3. **Atomic failure under concurrency:** first failure is preserved; failure path records propagation and drain latency; output remains uncommitted.
4. **Ordered output:** completion can be out-of-order; commit is atomic and entry-id ordered.
5. **Arena ownership:** shared arena without lock is rejected/failed; lock adds measurable overhead.
6. **Queue topology:** one compute queue must serialize queue execution (or collapse effective Vulkan parallelism) and must not claim hardware parallelism.

---

## 5) Thread-safety contract

Workers may touch only immutable plans plus worker-local state (ring, staging, local arena/slot assignments). Workers must never run judgment, mutate policy memory, mutate Dominatus directly, or touch another worker’s arena/slot unless in explicit shared-arena mode with ownership lock.

---

## 6) Event-ring contract

- Critical-event overflow is a hard batch failure.
- Diagnostics overflow is allowed to be lossy only if dropped count is tracked.
- Drain/join layer is the only bridge from worker rings to Dominatus visibility.

---

## 7) Atomic failure/drain contract

- First failure wins and is preserved (entry/worker/detail).
- Workers stop starting new plans after observing failure.
- In-flight work drains.
- Cleanup occurs post-drain.
- Output commit remains false on failure.

---

## 8) Queue topology findings

- Single queue: real threads are still useful for thread-safety validation, but Vulkan submit must be serialized and diagnostics must not claim hardware parallelism.
- Multi-queue: true parallel claims are allowed only up to effective worker caps from queue topology and memory.

---

## 9) Arena ownership findings

- Per-worker local arenas are default-safe.
- Shared arena mode requires explicit ownership lock.
- Memory budget caps effective workers directly.

---

## 10) Final recommendation

### Direct answers

1. **Is Prometheus ready for real worker execution?**
   - **Yes, conditionally**: ready for a controlled real-thread bridge that preserves existing policy/plan/failure/ordering contracts.
2. **Which real-worker candidate should be implemented first?**
   - **B (real threads + serialized Vulkan execution bridge)**.
3. **Should P11 M10 implement real threads or serialized-thread bridge first?**
   - **Serialized-thread bridge first**.
4. **What must remain forbidden to workers?**
   - Judgment, policy-memory mutation, direct Dominatus writes, and cross-worker arena/slot touching without shared lock mode.
5. **What event overflow policy should native code enforce?**
   - Critical overflow hard-fail; diagnostics overflow counted lossy.
6. **How should failure propagation/drain work?**
   - First-failure-wins, stop new starts, drain in-flight, no commit on failure, cleanup after drain.
7. **How should output ordering remain atomic?**
   - Worker completions may reorder internally, but publish once atomically in entry-id order.
8. **How should single-queue hardware degrade?**
   - Keep threads if needed, serialize queue submit or collapse effective Vulkan worker path; claim no hardware parallelism.
9. **What per-worker arena ownership rule is required?**
   - Local arenas are exclusive; shared arena requires explicit lock ownership.
10. **What should remain deferred?**
   - Work stealing, SPMC/MPMC, lock-free queue runtime, parallel judgment, unlocked cross-worker arena borrowing.

---

## 11) P11 M10 direction

Implement candidate **B** as the smallest safe native step:

- real worker threads,
- serialized Vulkan queue submission bridge,
- strict worker touch boundaries,
- per-worker event rings with explicit overflow contracts,
- first-failure-wins drain/commit behavior,
- unchanged ordered atomic output commit and Dominatus staged→visible boundary.

Gate candidate **C** behind demonstrated queue topology and arena-ownership readiness.

---

## 12) Deferred scope

M41 intentionally defers:

- work stealing,
- SPMC/MPMC queue design,
- lock-free queue implementation,
- parallel judgment,
- broad performance sweeps,
- runtime behavior changes outside readiness contracts.

---

## Language/reference consistency note

This milestone adds executable experiment model/tests under `Experiments/` and does not modify language semantics. No inconsistency with `Language/reference` was identified in this change.
