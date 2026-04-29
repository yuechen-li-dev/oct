# P13 M10 — Resource-Lease Controller Runtime Smoke Integration

## 1) M9 handoff summary

M9 introduced the Dominatus lease seam: lease facts/decisions, deterministic arbiter, blackboard staging/visible projection, reason codes, and lease diagnostics/counters. It intentionally stopped short of wiring lease request/grant/yield into live SGEMM and batch execution flow.

## 2) Runtime integration points

M10 threads lease lifecycle checks into:

- synchronous SGEMM runtime path (`prom_reactor_runtime_sgemm_impl`) using staged->commit->visible lease facts/decision and visible diagnostics,
- batch/slot runtime loop (`prom_reactor_runtime_sgemm_batch_impl`) using lease facts/decision evaluation at slot execution points,
- policy/batch diagnostics export fields for lease runtime observability.

## 3) Lease lifecycle in single SGEMM

Flow now is:

1. Build lease facts from runtime/slot state and selected occupancy recipe.
2. Stage lease facts to Dominatus blackboard.
3. Commit and build visible lease projection.
4. Run deterministic lease decision.
5. Stage decision and commit.
6. If denied, fail before SGEMM critical section.
7. If granted, run existing SGEMM path unchanged.
8. Issue explicit yield facts/decision after successful execution and commit.

## 4) Lease lifecycle in batch/slot path

Per planned slot execution in the lane-simulated batch loop:

1. Build lease facts (worker/slot/entry, masks, lookahead, recipe variant).
2. Run lease decision before slot submission/execution.
3. Denied lease transitions batch to failing path (no output commit).
4. Granted lease allows existing execution path.
5. Yield decision is emitted after slot completion.

This remains a smoke-level integration and does not replace batch scheduler architecture.

## 5) Minimal actuation behavior

Active behavior now includes:

- deny on failed-slot mask,
- deny on invalidated-slot mask,
- deny on unsafe-runtime hook path,
- deny on outstanding-depth cap hook path,
- explicit yield accounting after successful SGEMM and batch slot completion.

## 6) Lookahead bounded diagnostics

Lease facts/decision carry lookahead requested/limit/allowed. M10 exports blocked reason and selected recipe variant in runtime diagnostics to keep lookahead bounded and inspectable.

## 7) Diagnostics exported

Added runtime-visible lease fields in:

- `PrometheusSgemmPolicyDiagnostics`
- `PrometheusSgemmBatchDiagnostics`

including request/grant/deny/yield counts, last state, last deny reason, lookahead requested/allowed/blocked reason, and selected recipe variant.

## 8) Tests added

Added Marionette smoke tests for:

- single SGEMM lease grant/yield,
- batch lease grant/yield,
- batch failed-slot deny,
- batch invalidated-slot deny,
- batch unsafe-runtime deny,
- batch outstanding-depth lookahead block.

Naming includes `P13_M10` and `ResourceLease` for milestone filter hygiene.

## 9) Behavior intentionally unchanged

Intentionally unchanged:

- no new SGEMM kernels,
- no dispatch rewrite,
- no occupancy-actuated kernel switching,
- no transfer prefetch implementation,
- no work-stealing or queueing architecture rewrite,
- no performance claims.

Safe SGEMM/batch execution semantics remain the same except for lease diagnostics plumbing and narrow deny hook paths.

## 10) Deferred scope

Still deferred:

- new SGEMM kernel variants,
- occupancy-based dispatch actuation,
- full transfer prefetch / Smith Predictor,
- work stealing,
- SPMC/MPMC,
- runtime autotune,
- response-surface fitting,
- benchmark-driven policy updates,
- public performance claims.
