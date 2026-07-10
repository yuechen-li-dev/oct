# Prometheus R2d — batch authority switch

## Result

The public `prometheus_reactor_runtime_sgemm_batch` route now accepts only the
logical planning subset and always calls the M31 shared-ring engine.  It has no
fallthrough to P11.  Known legacy values are intentionally *unsupported* in a
production call; they return `PROM_ERROR`, `PROM_STAGE_INIT`, and
`PROM_DETAIL_BATCH_UNSUPPORTED_OPTION` (`-6613`) before planning or physical
admission.  Caller outputs are consequently untouched and the runtime remains
reusable.

The retained P11 implementation is called only by the explicitly named native
test compatibility entry `prometheus_reactor_runtime_sgemm_batch_legacy_test`.
M31 failure injection similarly uses `prometheus_reactor_runtime_sgemm_batch_m31_test`.
Those entries are test-only declarations, not production routing options.

## Authoritative public flag inventory

| value / range | historical route and meaning | R2d production treatment | diagnostics |
|---|---|---|---|
| `0x000000ff` | P11 worker count | supported requested logical width | requested/planned logical width |
| `0x00000100` | P11 contiguous worker partition | supported contiguous logical partition | partition policy/lane map |
| `0x00000200` | fail after submit | test-only M31 seam; production reject | unsupported detail, zero submits |
| `0x00003c00` | fake hardware cap | legacy/test-only; reject | unsupported detail |
| `0x0000c000` | fake arena scale | legacy/test-only; reject | unsupported detail |
| `0x003f0000` | fake event capacity | legacy/test-only; reject | unsupported detail |
| `0x00400000` | dual-failure injection | test-only M31 seam; reject | unsupported detail |
| `0x00800000` | delayed entry injection | legacy/test-only; reject | unsupported detail |
| `0xff000000` | encoded fail-entry injection | test-only M31 seam; reject | unsupported detail |
| topology/queue enum values | P11 simulated topology | ABI tombstones; no production selector | M31 reports one observed shared queue only |

The sole production mask is
`PROM_BATCH_PRODUCTION_FLAG_MASK = 0x000000ff | PROM_BATCH_FLAG_PARTITION_CONTIGUOUS`.
Zero selects default logical width and round-robin.  A nonzero width is capped
deterministically by the documented logical maximum and entry count; it does
not create threads, queues, lane-owned slots, or worker-local resources.

## Call-graph proof

```text
public batch entry
  -> supported-mask validation
  -> immutable prom_sgemm_batch_plan
  -> prom_sgemm_batch_refill_ring (M31)
  -> M30 task ownership -> M29 physical ring -> Vulkan SGEMM

explicit legacy test entry
  -> retained P11 executor -> batch_reference_sgemm / empty-submit machinery
```

`batch_reference_sgemm`, P11 empty-submit helpers, and the worker-thread bridge
remain in the retained executor only.  No production branch calls that
executor, including after an M31 error.  Unsupported flags are rejected rather
than cleared, simulated, or retried through P11.

## ABI and diagnostics

Existing flags, enum values, function signature, and
`PrometheusSgemmBatchDiagnostics` layout are unchanged.  The only new public
detail value is appended as `-6613`.  M31 fills plan, lane, ring, submission,
completion, drain/feedback and ordered-commit evidence.  Legacy physical-worker
fields remain neutral or compatibility-only and must not be read as threads or
per-worker Vulkan resources.  Unsupported calls report the stable failure
detail and neutral execution counters.

## Acceptance validation (RTX 3070)

Observed hardware: **NVIDIA GeForce RTX 3070**, driver **596.36**.

The complete existing `PrometheusSgemmBatch` public/M31 suite passed on the
hardware: 17 passed, 0 skipped, 0 failed.  This includes zero flags, contiguous
planning, width-plus-contiguous public routing, pre-admission injection-bit
rejection, M31 submission/commit evidence, atomicity, ordered feedback,
nonfatal reuse, and fatal unsafe rejection.  The retained P11 compatibility
suite passed separately: 62 passed, 1 expected skip, 0 failed.

The failure/timing-sensitive nonfatal subset (first failure, stop admission,
drain, atomicity, and reuse) passed **5/5 on each of three consecutive runs**.
The fatal unsafe test passed in an isolated process.  It is intentionally not
combined before the nonfatal repetitions because its device-loss test seam
poisons the process-level Vulkan test state.

Static Linux parity also passed on Windows: `go run ./tools/prometheus_native_manifest -check`,
`bash -n internal/prometheus/native/build_linux.sh`, and
`bash -n internal/prometheus/native/native_sources_linux.sh`.

## Completion validation (RTX 3070)

The permanent public-route tests now cover widths 1, 2, and 4; width above
entry count; and the retained separate-compute-family topology alias.  All
four focused authority cases passed on the RTX 3070 (including the preexisting
supported-flags/P11-isolation proof).  The topology alias preserves its old
numeric bit and is rejected by public mask validation with `-6613`, zero
submits, untouched output, and a subsequent valid M31 batch.

The full named `PrometheusSgemmBatch` filter was rerun.  Its lexical order runs
the fatal device-loss test before later Vulkan tests, and that deliberately
process-global seam makes those later tests skip.  The complete nonfatal
hardware subset, including the new cases, passes when executed before fatal;
fatal passes separately.  Failure-sensitive cases passed 5/5 in each of three
consecutive runs.  This is an execution-order property of the test seam, not a
production fallback or a skipped authority assertion.

Both Go lanes, the launcher-driven Windows native build, retained P11 suite,
Linux manifest parity, and Linux shell syntax checks pass.

## Acceptance conclusion: R2d ACCEPTED

Every supported public batch request now executes M31 or returns explicit
error, the retained P11 executor is test-only, and all required R2d authority
coverage has permanent hardware tests. **R2e has not begun.**

## Retained P11 and R2e work

The focused M29/M31 suite uses the normal public entry for production authority
cases and the named M31 seam for fault injection.  The retained P11 suite now
uses the named legacy seam.  R2e may delete the P11 executor, CPU result
authority, empty-submit helpers, and worker bridge after the compatibility
suite has been retired; none is deleted in R2d.

This supersedes the R2b report's old statement that a width larger than the
entry count leaves empty logical lanes: R2d caps planned width by entry count
as the approved public meaning requires.
