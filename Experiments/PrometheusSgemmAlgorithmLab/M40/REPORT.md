# P11 M5 / Prometheus SGEMM Algorithm Lab M40 — Static Partition Batch Dispatch Contract Lab

## 1) M39 handoff summary

Labels: **P11 M5** and **Experiments/PrometheusSgemmAlgorithmLab/M40**.

M39 proved (via executable model) that centralized policy + immutable dispatch plans is viable when judgment is not dominant, and that effective workers must be capped by hardware queue topology and per-worker arena memory budget. It also showed static partition is the lowest-complexity first implementation, while shared SPMC can be a follow-up candidate and work stealing should remain deferred until heavy-tail imbalance plus low stealing overhead are both true.

Why static partition first:
- lowest implementation complexity and ownership ambiguity,
- directly compatible with per-worker arena/slot ownership,
- easier to audit atomic failure and ordered-join semantics.

Why SPMC/work stealing remain deferred:
- both add runtime contention/complexity before the first correctness contract is implemented,
- stealing/queue-sharing are optimization paths, not prerequisites for immutable-plan correctness.

Why workers execute plans and do not run judgment:
- preserves policy/dispatch separation,
- keeps blackboard reads centralized in policy layer,
- avoids inconsistent interpretation across workers.

What M40 must prove pre-native:
- immutable plan contract fields,
- effective worker cap contract,
- static partition assignment behavior,
- ordered output + atomic failure + drain behavior,
- event ring overflow contract,
- explicit M6 implementation target and deferred scope.

## 2) Dispatch plan model

`M40DispatchPlan` models immutable plan fields:
- `EntryId, M, N, K, WorkUnits, InputOffset, OutputOffset`,
- `SelectedPath, ComputeMode, BufferingMode, TransferPolicy, LayoutPrecisionMode`,
- `ArenaRequiredBytes, ExpectedOutputElements, PlanGeneration, FailurePolicy`.

Invariant implemented: workers execute only prebuilt plans (`PlanGeneration=40`) and do not reinterpret policy.

## 3) Worker model

`M40WorkerContract` covers:
- worker id,
- effective queue id,
- assigned plan list,
- local arena budget,
- local slot budget,
- event ring capacity,
- in-flight/active/completed/failure state.

First implementation contract keeps arena and slot ownership strictly worker-local (no cross-worker arena borrowing).

## 4) Partition policies

Compared static policies:
- **Round-robin**: `worker_id = entry_id % effective_workers`
- **Contiguous chunk**: `worker_id = floor(entry_id * effective_workers / batch_size)`

Model findings:
- round-robin reduces imbalance on moderate variability,
- contiguous improves shape locality on shape-clustered sequences,
- both are valid; round-robin is the default first implementation path.

## 5) Batch result ordering

Contract:
- each plan maps to its own output index,
- workers can finish out of order,
- caller-visible output is committed only at join and only on success,
- committed output order is by `EntryId`.

Model includes explicit completion order vs committed ordered output table.

## 6) Atomic failure model

Modeled states:
- `PENDING -> RUNNING -> SUCCEEDED`
- `PENDING -> RUNNING -> FAILING -> DRAINING -> FAILED`

First failure records:
- failed entry id,
- failed worker id,
- failure detail,
- failure stage.

Failure cases modeled:
1. before submit,
2. after submit,
3. in-flight while other workers may continue draining,
4. during drain cleanup,
5. cleanup success after drain.

No partial success output is committed on failure.

## 7) Event ring model

Workers emit lifecycle events only via per-worker rings:
- plan started,
- plan submitted,
- plan completed,
- plan failed,
- worker idle (diagnostic),
- worker drained,
- batch failure observed.

Policy selected in M40:
- **critical overflow fails batch explicitly**,
- diagnostics overflow is counted and may be lossy.

This was tested under normal, near-capacity, and overflow scenarios.

## 8) Per-worker arena/slot interaction

Contract:
- per-worker arena bytes contribute directly to worker cap,
- per-worker slot budget remains local ownership,
- no cross-worker arena usage in first implementation,
- memory cap can force single-worker path.

## 9) Dominatus integration contract

- **Policy layer**: reads visible blackboard snapshot, performs judgment, builds immutable plans.
- **Workers**: execute immutable plans and emit worker-ring events only.
- **Commit/drain layer**: drains worker events, stages Dominatus updates, commits at explicit boundary.

Preserved rule:
- reactor writes staged state,
- judgment reads visible state,
- commit promotes staged to visible.

## 10) Findings

- First implementation should use central policy + immutable plans + static partition.
- Round-robin is the best default for general variability and simpler fairness.
- Contiguous should be supported as a selectable static mode for locality-oriented workloads.
- Atomic batch failure with explicit drain is mandatory before native implementation.
- Event rings are mandatory; critical overflow must be explicit failure.

## 11) Final implementation contract

M40 final contract artifact (`m40_final_contract.octagon`) defines:
1. dispatch plan content,
2. first policy choice,
3. optional contiguous support,
4. effective worker count rule,
5. ordered output rule,
6. atomic state machine,
7. drain rule,
8. event overflow policy,
9. arena/slot ownership rule,
10. Dominatus integration rule.

## 12) P11 M6 recommendation

P11 M6 should implement native:
- central policy immutable-plan builder,
- round-robin static partition batch execution,
- per-worker arena/slot ownership,
- per-worker event rings with critical-overflow failure,
- atomic join commit and failure-drain behavior,
- contiguous static partition mode as optional policy switch.

## 13) Deferred scope

Remain deferred (explicitly):
- shared SPMC queue runtime,
- work stealing runtime,
- MPMC / lock-free queue structures,
- parallel judgment,
- N-slot stealing scheduler.

## Language/reference consistency note

No intentional syntax/style divergence from `Language/reference` was introduced in this M40 Oct model. If future native implementation details require semantics not represented in `Language/reference`, that should be surfaced as a documentation gap instead of inferred from experiment code.
