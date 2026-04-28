# P11 M18 / Prometheus SGEMM Algorithm Lab M44 — N-Slot Runtime Readiness and Contract Lab

## 1) M16/M17 handoff summary (required first step)

### What M16/M17 proved

M16/M17 proved the hardened post-M11 execution topology can safely run true multi-queue submit when topology/resource/sync gates pass, while preserving serialized fallback, first-failure drain behavior, and ordered atomic output commit.

### Why N-slot was deferred until after multi-queue

N-slot changes in-flight fan-out and queue-feeding behavior. Without a hardened multi-queue gate and fallback truthfulness, slot-count decisions would be confounded by submit topology uncertainty and could overclaim hardware parallelism.

### What slot readiness dirty tracking already provides

M36 semantics already provide dirty/readiness invalidation tracking, coalesced duplicate readiness, and boundary-driven “changed since last check” clearing. M44 extends this into explicit slot masks (`dirty`, `ready`, `failed`, `invalidated`, `attention`) for refill attention.

### What typed arenas already provide

M38 provides typed arena ownership, artifact compatibility/invalidation rules, and budget-aware capacity transitions. M44 consumes these as per-worker ownership boundaries and budget caps that directly constrain effective slots per worker.

### What M44 had to prove before implementation

M44 had to prove executable contracts for slot ownership, readiness-driven refill, memory/budget slot caps, queue utilization impact for S=1/2/4, failure/drain fan-out under multi-slot workers, and a computed recommendation for P11 M19.

---

## 2) Candidate N-slot models

Compared candidates:

- **A**: 1 slot per worker baseline.
- **B**: 2 worker-local slots per worker.
- **C**: 4 worker-local slots per worker.
- **D**: shared slot pool across workers (reference/deferred).
- **E**: work-stealing N-slot reference (deferred).

Model result: candidate **B** wins computed product score under required penalties (memory pressure, fan-out, complexity, cross-worker ownership risk).

---

## 3) Slot ownership model

Modeled slot fields include owner worker, generation, assigned plan/entry id, queue id, arena ownership, failure state, and lifecycle state transitions.

First implementation contract is explicit:

- `slot.owner_worker_id` is exclusive.
- No cross-worker borrowing in implementation candidate.
- Shared pool is modeled only as reference/deferred complexity row.

---

## 4) Readiness / attention model

M44 boundary model computes and persists:

- `dirty_slot_mask`
- `ready_slot_mask`
- `failed_slot_mask`
- `invalidated_slot_mask`
- `attention_slot_mask = ready ∪ failed ∪ invalidated`

Validated behaviors:

- dirty accumulation persists across non-slot commits,
- duplicate ready events coalesce,
- failed/invalidated remain in attention,
- boundary clear resets dirty (change window) while preserving terminal masks.

---

## 5) Refill / scheduling model

Scheduling remains static partition:

- each worker consumes only its assigned plan list,
- each worker refills only its owned slots,
- no work stealing,
- no cross-worker slot mutation.

M44 compares full-scan polling vs attention-driven polling and reports explicit `polling_avoided` and `attention_poll_count`.

---

## 6) Typed arena interaction

Modeled effects by slot count:

- per-worker arena base memory + per-slot artifact memory,
- peak committed memory and pressure,
- budget-driven effective slot cap,
- explicit cap reason (`memory-budget-capped`).

Contract:

- per-worker local arenas stay default,
- no cross-worker arena borrowing,
- if budget cannot sustain requested S, reduce effective slots with explicit diagnostics.

---

## 7) Queue interaction findings

Model covers single-queue fallback and multi-queue eligible topology.

Findings:

- **S=2 vs S=1**: non-regressing queue utilization with better refill buffering in the model.
- **S=4**: modest utilization gain does not offset memory/complexity penalty under default envelope.
- single-queue fallback remains compatible with the same slot contract (no hardware overclaim).

---

## 8) Failure / drain findings

Modeled scenarios:

1. fail before submit,
2. fail after submit,
3. dual failures (first failure wins),
4. queue-local failure with other active slots,
5. invalidated-ready slot,
6. drain timeout,
7. device/global failure.

Resulting contract:

- first failure wins,
- stop refill after failure observation,
- drain active/in-flight slots or mark unsafe,
- keep output uncommitted on failure,
- keep failed/invalidated slots in attention,
- release reusable ownership only after safe drain.

---

## 9) Diagnostics contract

Mandatory diagnostics defined by M44:

- worker count, slots per worker, total slots,
- per-slot owner worker id/state/generation/entry id/queue id,
- dirty/ready/failed/invalidated/attention masks,
- boundary generation,
- refill count,
- full-scan polls, attention polls, polling avoided,
- slot failure count, drain slot count,
- unsafe-to-reuse flag,
- memory budget slot-cap reason.

---

## 10) Final recommendation

Direct answers:

1. **Is N-slot ready for native implementation?** **Yes, for bounded first-step N-slot.**
2. **How many slots per worker first?** **S=2.**
3. **Worker-local or shared?** **Worker-local.**
4. **How does dirty tracking drive refill?** Refill inspects only attention-set slots at boundaries plus worker-local empty slots; avoids full-slot polling.
5. **Does S=2 improve enough over S=1?** **Yes** in model score/utilization while staying moderate in memory/risk.
6. **Is S=4 worth implementing now?** **No** under modeled envelope; defer pending stronger gain evidence.
7. **How does budget cap slots?** Effective slots per worker are reduced by per-worker budget envelope with explicit cap reason.
8. **How should failure/drain work?** First-failure-wins, halt refill, drain/mark unsafe, no commit on failure, cleanup after safe drain.
9. **What diagnostics are mandatory?** Masks, ownership, generations, refill/polling counters, failure/drain counters, safety/budget reasons.
10. **What remains deferred?** Shared pool, S=4+, work stealing, SPMC/MPMC, lock-free runtime redesign.
11. **Is work stealing deferred?** **Yes.**

---

## 11) P11 M19 direction

Implement **worker-local S=2** as first native N-slot path:

- preserve static partition and immutable plans,
- add per-worker two-slot runtime state,
- gate refill by readiness attention masks,
- keep typed arena ownership local,
- preserve first-failure-wins drain and atomic ordered commit,
- ship full M44 diagnostics contract.

---

## 12) Deferred scope

Explicitly deferred:

- S=4+ productionization,
- shared cross-worker slot pool,
- work stealing,
- SPMC/MPMC queueing,
- lock-free runtime work,
- judgment/distributed policy mutation.

---

## Language/reference consistency note

This milestone adds executable experiment model/tests/artifacts under `Experiments/` and does not alter language semantics in `Language/`. No direct inconsistency with `Language/reference` was observed in this change.
