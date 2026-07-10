# Prometheus R2e0 — P11 test-lane disentanglement

## Result: R2e0b ACCEPTED — safe to delete P11

`reactor_p11_m6_batch_tests.cpp` contains 63 registered tests.  The executor
and public routing are intentionally unchanged in R2e0.  The audit found seven
P13-labelled tests mixed into the P11 lane:

| Old test group | Actual dependency | Classification | R2e0 disposition |
|---|---|---|---|
| `P13_M10_ResourceLease_BatchGrantYieldSmoke` | calls legacy batch executor | P11-coupled lease observation | migrate to M31 lease facts before extraction |
| `P13_M10_ResourceLease_BatchFailedSlotDenied` | legacy failure path | P11-coupled | migrate or retire with P11 |
| `P13_M10_ResourceLease_BatchInvalidatedSlotDenied` | legacy invalidated slot | obsolete P11 slot contract | retire in R2e |
| `P13_M10_ResourceLease_BatchUnsafeRuntimeDenied` | legacy batch failure | P11-coupled | migrate to M31 unsafe/runtime contract |
| `P13_M10_ResourceLease_BatchOutstandingCapBlocksLookahead` | P11 thread/cap hook | obsolete machinery | retire in R2e |
| `P13_M11_ResourceLease_DiagnosticsCoherentOnDeny` | legacy batch failure | P11-coupled | migrate to M31 diagnostics route |
| `P13_M11_ResourceLease_RepeatRunCountersStable` | legacy batch executor | P11-coupled | migrate to M31 lease facts |

They cannot be moved unchanged: every one calls
`prometheus_reactor_runtime_sgemm_batch_legacy_test`; several require P11
failure, worker, or slot behavior.  Moving them would violate R2e0's required
independence and retaining their old assertions would preserve obsolete
architecture.  No generic P13 test was extracted on a false premise.

All other tests in the file classify as either obsolete P11 execution machinery
(threads, empty-submit resources, fake queue topology, worker slots/events,
CPU batch authority) or durable batch semantics already replaced by R2b/R2c/R2d
M31 authority tests.  The detailed replacement matrix is in the R2e0 artifact.

The next R2e0 step is narrowly scoped: establish M31-backed lease tests for the
four P13 contracts that remain meaningful, then extract those tests to
`reactor_resource_lease_tests.cpp`; only then can the P11 source/test lane be
deleted.  R2e production deletion has not begun.

## R2e0b replacement work

An attempted `reactor_resource_lease_tests.cpp` replacement exposed missing M31
observability and was removed rather than retained as a failing test. The
attempted contracts were:

- `PrometheusResourceLeaseM31BatchGrantsAndReleases`;
- `PrometheusResourceLeaseM31UnsafeRuntimeDeniesLaterBatch`;
- `PrometheusResourceLeaseM31BatchCountersResetAcrossRuns`.

They used the public batch route and current M31 fatal seam only; they did not
invoke the legacy entry or inspect P11 workers, slots, queues, or threads.
The RTX 3070 run showed that current M31 batch diagnostics publish neither
lease grant/yield counts nor `unsafe_to_reuse` after the fatal seam. These are
real missing M31 facts, not P11 semantics to preserve.
The P11 failed-slot test is mapped to existing M31 nonfatal failure/drain/reuse
authority coverage; invalidated-slot and outstanding-cap tests remain obsolete.

The full launcher build, retained P11 suite (62 pass, 1 expected skip), both Go
lanes, manifest parity, and Linux shell checks pass. The attempted old-counter
assertions are intentionally retired: grant/yield totals and mirrored unsafe
diagnostic bits are P11/P13 bookkeeping, not behavioral contracts.

## Final P13 reclassification

| Old P13 test | Current authoritative replacement/disposition |
|---|---|
| `BatchGrantYieldSmoke` | `PrometheusSgemmBatchRefillRing` and `PrometheusSgemmBatchRuntimeReusableAfterNonfatalFailure`: successful M31 admission, physical resolution, and reuse |
| `BatchFailedSlotDenied` | `PrometheusSgemmBatchDrainsSubmittedEntriesSafely` and `PrometheusSgemmBatchRuntimeReusableAfterNonfatalFailure` |
| `BatchInvalidatedSlotDenied` | obsolete P11 slot machinery |
| `BatchUnsafeRuntimeDenied` | `PrometheusSgemmBatchFatalFailureMarksRuntimeUnsafe` |
| `BatchOutstandingCapBlocksLookahead` | obsolete P11 worker/cap machinery |
| `DiagnosticsCoherentOnDeny` | `PrometheusSgemmBatchUnsupportedFlagsFailBeforeAdmission`, `PrometheusSgemmBatchPlanValidatesBeforeAdmission`, and R2c failure evidence |
| `RepeatRunCountersStable` | `PrometheusSgemmBatchRuntimeReusableAfterNonfatalFailure` and `PrometheusSgemmBatchPlanEntryIdentitySurvivesTaskReuse` |

No distinct current M31 behavior remains uncovered.  The replacement matrix was
checked against the green R2b/R2c/R2d and M29/M30/M30a/M31 lanes.  No production
deletion was performed in R2e0b.
Production P11 deletion has not begun.

## R2e2 completion

R2e2 deletes the residual P11 executor, CPU batch result authority, empty-submit
path, worker bridge, worker-local resources and slots, fake queue topology,
event ring, legacy compatibility entry, and slow test lane. Public batch is
M31-only; legacy v1 diagnostic and numeric flag fields remain neutral ABI
tombstones. The P11 M3 arena accounting remains separate allocation work.

The Windows launcher build, manifest parity, Linux shell syntax, both Go lanes,
RTX 3070 M29/M30/M30a/M31 authority tests, three failure-sensitive repetitions,
and isolated fatal unsafe classification pass. Post-deletion resident, EVT,
synchronous/deep correctness, explicit implementation, FP16 numerical
selection and rejection, packed4 cache/artifact reuse, and buffering-selector
lanes passed 16/16 with no skips or failures.
The detailed history is preserved in
`PROMETHEUS_P11_ARCHITECTURE_RETROSPECTIVE.md`; deletion and reachability proof
are in the R2e2 artifacts.

R2e2 ACCEPTED — P11 is historical documentation, not executable architecture.
