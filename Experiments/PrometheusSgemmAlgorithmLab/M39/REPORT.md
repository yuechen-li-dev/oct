# P11 M4 / Prometheus SGEMM Algorithm Lab M39 — Policy/Dispatch Separation Feasibility Lab

## 1) Architecture question (required first step)

This M39 lab answers whether Prometheus should move toward:

- **single-threaded centralized policy layer** (Dominatus visibility + judgment ownership),
- emitting **immutable dispatch plans**,
- consumed by **parallel dispatch workers** that execute plans without mutating policy state.

### Why immutable dispatch plans instead of worker-side judgment

1. Dominatus/judgment ownership stays single-writer and auditable.
2. Dispatch workers avoid policy-state races and snapshot divergence.
3. Event drain + commit order remains explicit at one boundary.
4. Policy regressions are isolated from queue/scheduler mechanics.

### Why centralized policy might bottleneck

Policy wall-clock lower bound is `N * J`; if that term exceeds parallelized dispatch time, worker count increases do not improve completion.

### Why work stealing may or may not be needed

- Helps when load imbalance is high (heavy-tail task durations).
- Hurts when task durations are uniform/small and steal overhead/backoff dominates.

### What M39 must answer before C implementation

1. Is policy/dispatch separation viable under modeled conditions?
2. Which dispatch model should be implemented first?
3. Should work stealing be first-class now, or deferred?
4. How should worker count be chosen from queue topology + memory budget?

---

## 2) Candidate dispatch models compared

Compared exactly the required set:

- **A**: single-worker baseline.
- **B**: central policy + static partition.
- **C**: central policy + shared SPMC queue.
- **D**: central policy + local queues + work stealing.
- **E**: parallel judgment reference (not preferred architecture).

See model matrix artifact: `m39_dispatch_model_comparison.octagon`.

---

## 3) Model variables

Modeled variables:

- batch size `N`, judgment cost `J`, dispatch cost `D`, workers `W`, compute queues `Q`,
- shared queue overhead `QO`, steal attempt cost `S`, failed steal backoff,
- workload duration distributions: uniform / moderate / heavy-tail,
- per-worker arena memory multiplier + memory budget,
- event emission + drain overhead.

Core bound used:

`throughput bound = max(N * J, N * D / effective_workers)`

with:

`effective_workers = min(W, Q, memory_cap_workers, event_drain_cap_workers)`

and additional overhead terms for queue contention and stealing.

---

## 4) Hardware regimes

Modeled required regimes:

1. **Single compute queue (`Q=1`)**
2. **Two compute queues (`Q=2`)**
3. **Four compute queues (`Q=4`)**
4. **Four compute + transfer queue** (optional regime included)

See topology artifact: `m39_queue_topology_table.octagon`.

Key result: when `W > Q`, extra workers do not produce compute parallelism; they only increase overhead and memory pressure.

---

## 5) Workload distributions

Modeled required workload classes:

1. uniform tasks,
2. moderately variable tasks,
3. heavy-tailed tasks,
4. small fast tasks,
5. large slow tasks.

See steal/imbalance artifact: `m39_steal_overhead_table.octagon`.

---

## 6) Metrics tracked

Tracked per model/regime:

- completion time,
- policy time,
- dispatch time,
- queue overhead,
- steal overhead,
- idle time / imbalance,
- worker utilization,
- speedup vs baseline A,
- memory pressure + per-worker arena footprint,
- event ring pressure,
- batch latency,
- composite product score.

### Product score components

Included weighted penalties for:

- completion time,
- idle time,
- queue/steal overhead,
- memory pressure,
- implementation complexity,
- policy contention risk,
- implementation risk.

---

## 7) Findings by model

## A) Single-worker baseline

- Correctness/control baseline only.
- Predictable but no scaling.
- Needed as graceful fallback on `Q=1`.

## B) Central policy + static partition

- Best first implementation fit.
- Zero post-assignment queue contention.
- Strong for uniform and large-slow workloads.
- Suffers under heavy-tail imbalance.

## C) Central policy + shared SPMC queue

- Best first dynamic balancing option.
- Handles moderate variability better than static.
- Has queue overhead; can lose for very small tasks.

## D) Central policy + local queues + work stealing

- Best heavy-tail resilience when imbalance is high.
- Carries steal/backoff overhead and highest scheduler complexity among policy-owned designs.
- Not justified as first implementation without heavy-tail evidence.

## E) Parallel judgment reference

- Throughput can look attractive in some synthetic regimes,
- but policy-state ownership/contention and Dominatus snapshot consistency risk are high.
- Keep as reference only, not product direction.

---

## 8) Rakes / failure modes exercised

All required failure modes were explicitly evaluated:

1. **Judgment bottleneck**: if `N*J` dominates, dispatch parallelism has low return.
2. **Queue topology false scaling**: if `W > Q`, do not claim hardware parallel speedup.
3. **Steal overhead cliff**: stealing loses when task cost is too small.
4. **Static imbalance**: heavy-tail tasks strand static workers.
5. **Shared queue contention**: SPMC overhead can outweigh balancing gains.
6. **Memory multiplier**: per-worker arenas cap feasible workers under budget.
7. **Event ring pressure**: event drain capacity can become an independent cap.
8. **W=1 graceful degradation**: architecture must collapse cleanly on single-queue hardware.

Artifacts:

- `m39_judgment_bottleneck_table.octagon`
- `m39_queue_topology_table.octagon`
- `m39_steal_overhead_table.octagon`
- `m39_memory_pressure_table.octagon`

---

## 9) Final recommendation (actionable)

### Decision

Proceed with **centralized policy + immutable dispatch plans**.

### First implementation

1. Implement **Model B** first: central policy + static partition.
2. Add **Model C** next if moderate variability causes measurable idle imbalance.
3. Defer **Model D** work stealing until heavy-tail workloads show sustained imbalance where steal benefit exceeds overhead.

### Worker count rule

Use:

`W = min(Q_compute, workers_allowed_by_memory_budget, workers_allowed_by_event_drain)`

Do not exceed compute queue count unless there is proven non-compute overlap value.

### Single queue rule

On `Q=1`, force `W=1` and bypass dynamic queue/steal paths.

### Deferred items (explicit)

Do **not** implement yet:

- work-stealing runtime,
- MPMC queue,
- N-slot runtime,
- batch API,
- allocator implementation changes.

See recommendation artifact: `m39_final_recommendation.octagon`.

---

## 10) Next milestone direction (P11 M5 / Lab M40)

P11 M5 / Lab M40 should:

1. implement a compact executable prototype of **B static partition** in Prometheus dispatch plumbing,
2. add instrumentation for policy time vs dispatch time vs event drain time,
3. add optional **C shared SPMC** path behind a feature gate,
4. collect real traces for heavy-tail incidence and imbalance,
5. define a concrete threshold for escalating to work stealing.

Do **not** start N-slot/work-stealing runtime implementation in M40 unless measured traces cross the threshold.

---

## Required final answers

1. **Is centralized policy + immutable dispatch plans viable?** Yes, when policy time is not the dominant term and worker count is capped by topology/memory.
2. **When does centralized policy bottleneck?** When `N*J >= N*D/effective_workers`.
3. **Which dispatch model should be implemented first?** Model B (central policy + static partition).
4. **Is work stealing necessary for the first implementation?** No; defer.
5. **Under what workload does work stealing become worth it?** Heavy-tailed durations with sustained high imbalance and dispatch tasks much larger than steal overhead.
6. **How should worker count be chosen from queue topology?** Cap by compute queues first, then memory/event-drain caps: `W=min(...)`.
7. **How should design degrade on single-queue hardware?** Hard-collapse to `W=1`, no fake parallel dispatch paths.
8. **How does per-worker arena memory pressure affect worker count?** It is a hard cap; high arena multipliers quickly make `W=4+` infeasible under constrained budgets.
9. **What should P11 M5 / Lab M40 do next?** Build B-first prototype + instrumentation, then optional C-gated path.
10. **What should absolutely not be implemented yet?** N-slot runtime, work-stealing runtime, MPMC queue, batch API, allocator rewrites.

---

## Language/reference consistency note

This milestone adds report/artifact documents only (no new Oct language implementation semantics). No language-reference inconsistency was introduced in code paths.
