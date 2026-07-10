# Prometheus R2c — real M31 batch semantics

Status: **ACCEPTED**.  The real M31 authority suite passes on the RTX 3070
three consecutive times, and full native/P11/Go validation is green.  R2d has
not begun.

## Runtime model

`prom_sgemm_batch_entry_plan` remains immutable caller-order planning data.
`prom_sgemm_batch_entry_runtime` is allocated once per plan entry for a batch.
It records stable entry ID, plan generation, logical lane, state, submission
sequence, ring slot/generation, failure observation, and feedback disposition.
It owns no Vulkan handle, task pointer, or physical slot.  This retains the
M29 shared ring and M30/M30a task lifecycle: a recycled task never becomes
batch identity.

| State | Meaning |
|---|---|
| PLANNED | plan exists; no physical ownership |
| ADMITTED | task/ring ownership is being acquired |
| SUBMITTED | Vulkan submission succeeded; sequence and slot generation recorded |
| COMPLETED | output copied into batch staging only |
| FAILED | selected batch-cause entry failure |
| SKIPPED | never admitted after admission stopped |
| DRAINED | admitted ownership has been retired or reaped |
| COMMITTED | staged result copied to caller C |

## Failure and stop-admission rules

M31 now has an explicit reducer with permanent phase ranks:

1. staging allocation
2. task allocation
3. command record/resource preparation
4. submit
5. completion observation
6. result copy to staging
7. commit

Candidates compare `(entry_id, phase_rank, observation_sequence)`.  The
admission-stop freeze point is immediately before failure drain: the chosen
primary entry/detail is copied from the reducer, and all still-PLANNED entries
from `next` onward become SKIPPED.  Drain/reap observations occur after that
freeze and cannot overwrite the primary identity.  M30a quarantine remains the
owner of uncertain physical completion.

## Atomicity, feedback, and diagnostics

Output staging is allocated before admission and caller buffers are copied only
after every entry reaches COMPLETED.  The successful copy loop remains in
ascending entry ID and records COMMITTED only after each copy.  Failure skips
that loop.  Existing v1 diagnostics remain ABI-compatible: worker fields are
logical-lane aggregates, while M31 submission, completion, commit, ring, and
feedback counters remain observed physical facts.  No lane owns a queue,
thread, command resource, or ring slot.

Feedback still uses the existing M30 ordered completion processor and M30a
quarantine/reap path.  The runtime record reserves per-entry committed/skipped
fields for precise attribution; wiring those fields and exposing only truthful
bounded evidence is part of the remaining test-backed work.

## Validation and known gaps

The repository-local `build_windows_launcher.cmd` was run from a normal shell.
It located Visual Studio, initialized x64 MSVC, and completed the native build
in 131.036 seconds; its deterministic build log is
`out/test-artifacts/prometheus_native_windows_build.log`.  The current partial
implementation compiled.  Focused baseline tests passed: all R2b plan tests,
M29, M30, M30a, M31 transfer, M31 refill, resident, EVT, and the unchanged
three-test P11 slow suite.  No R2c authority-test or RTX-3070 evidence result
is claimed yet.

Focused R2c tests now cover deterministic dual-candidate reduction,
stop-admission/skipped tail evidence, multi-in-flight drain, sentinel atomicity,
entry-order commit, nonfatal observation-failure reuse, post-submit fatal-loss
rejection, valid-completion feedback, and logical-lane metadata.  The focused
set plus `PrometheusSgemmBatchRefillRing` passed three consecutive RTX 3070
runs.  Evidence is written to:

- `out/test-artifacts/prometheus_r2c_batch_semantics.json`
- `out/test-artifacts/prometheus_r2c_batch_semantics.md`

The Windows launcher completed the final native build in 131.702 seconds. Both
required Go lanes passed:

```
go test ./internal/prometheus/... ./cmd/oct
go test ./internal/... ./cmd/oct
```

## Shared SGEMM reuse triage

`PrometheusReactor_BufferReuseSafety_FP16ThenPacked4SameShape` failed alone
five times, so it was neither full-suite ordering contamination nor an R2c
batch regression.  The fresh synchronous runtime never enters the M31 batch
refill path.  Its FP16 result had max absolute error `0.00201321` against the
strict `1e-4` oracle contract.  The root cause was a pre-existing layout/
precision selector defect: it published a zero-error, tolerance-pass FP16
fact without examining the actual FP16 packed payload.

The fix computes a conservative input-quantization product-error bound from
the exact FP16 payload and rejects FP16 when that bound exceeds the established
absolute contract.  No oracle tolerance was relaxed.  The reuse-transition
tests now use exactly representable FP16 inputs, so they continue to prove
FP16-to-baseline/Packed4 artifact invalidation without making a false precision
claim.  The buffer artifact key already contains layout, precision, dimensions,
padded-K and required bytes; incompatible FP16/Packed4 artifacts are rebuilt.

The M31 transfer readiness failure was separately traced to a fixed 2,000-poll
busy loop.  It was replaced with deadline-based nonblocking polling, which is
stable under full-suite load.

Final validation:

- launcher-driven native build: pass (131.896 seconds);
- `marionette_tests.exe`: 292 passed, 28 expected skips, 0 failed;
- `marionette_slow_tests.exe` P11 suite: 3 passed;
- R2c authority set, including failure-sensitive cases: three consecutive
  RTX 3070 passes;
- R2b plan tests; M29/M30/M30a/M31; resident; EVT: pass;
- both required Go lanes: pass.

P11 routing and implementation are unchanged.  R2d routing work has not
begun.

## R2d readiness assessment

Ready for the separately approved R2d routing decision.  This milestone does
not make that routing change.
