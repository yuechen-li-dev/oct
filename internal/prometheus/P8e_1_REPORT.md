# P8e.1 REPORT — Async Failure-Channel Hardening Before Hardware Validation

## Scope

This pass is intentionally surgical and only hardens async lifecycle visibility and two nearby correctness assumptions.

## 1) Original bug

`prom_reactor_runtime_sgemm_impl(...)` previously auto-reset both `PROM_ASYNC_STATE_FAILED` and `PROM_ASYNC_STATE_CONSUMED` to `PROM_ASYNC_STATE_IDLE` at the start of a later submit attempt.

That allowed this silent failure fold:

1. async task fails
2. caller does not explicitly acknowledge failure
3. next submit clears failed slot implicitly and proceeds

## 2) Lifecycle rule change made

Submit preflight now treats async states as:

- `FAILED`: reject submit immediately with explicit failed detail (`async_failure_detail` when set, otherwise `PROM_DETAIL_ASYNC_FAILED`)
- `CONSUMED`: keep existing auto-clear-to-idle behavior
- `SUBMITTED`/`READY`: keep existing unconsumed/in-flight rejection behavior

This removes silent reset of `FAILED` while preserving current behavior for an explicitly consumed slot.

## 3) Explicit failure handling path

A failed slot remains query-visible (`lifecycle_state=FAILED`, `failed=1`) until the caller explicitly handles it via `prometheus_reactor_runtime_sgemm_abandon_async(...)`.

After explicit abandon, the slot is `CONSUMED`; the existing consumed auto-clear on next submit makes the slot reusable.

## 4) Tests proving failure is not silently cleared

Added `PrometheusReactor_AsyncFailureRemainsVisibleUntilExplicitAbandon`:

- induces async poll failure (`PROM_TESTCFG_FAIL_ASYNC_POLL`)
- verifies query reports `FAILED` + explicit failure detail
- verifies consume fails with failure detail (not `PROM_DETAIL_ASYNC_NOT_READY`)
- verifies resubmit before handling is rejected with the same failure detail
- verifies explicit `abandon_async` restores legal resubmit

Existing async safety tests remain intact for:

- use-before-complete (`PROM_DETAIL_ASYNC_NOT_READY`)
- double-consume rejection (`PROM_DETAIL_ASYNC_ALREADY_CONSUMED`)
- in-flight overlap rejection
- abandonment semantics

## 5) Dispatch convention clarification

Added an explicit dispatch/indexing comment in Vulkan SGEMM submit path:

- dispatch `x` covers rows (`m`)
- dispatch `y` covers columns (`n`)
- shader indexing must match this host convention

## 6) Staged device-local C overwrite assumption clarification

Added explicit note in staged path barrier setup:

- staged device-local `C` is not pre-zeroed
- this is correct because current SGEMM kernels overwrite every final `C` element directly

## 7) Deferred to hardware validation

Still deferred (unchanged in this pass):

- hardware performance benchmarking/tuning
- broader async policy redesign
- queue/scheduler redesign
- judgment-engine scoring/policy expansion
