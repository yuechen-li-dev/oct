# P8e Report — Async / Non-Blocking Submission Protocol Port

## 1) M10 invariants extracted and ported

From M10 (`REPORT.md`, `M10/*.oct`, `M10/*.octest`) the Vulkan port preserves these invariants:

1. explicit lifecycle states (`idle`, `submitted`, `ready`, `failed`, `consumed`)
2. explicit completion/progress checks (no implicit completion assumptions)
3. readiness-gated consumption (`not-ready` is distinct)
4. single-consume discipline
5. in-flight ownership protection (reject overlap/reuse)
6. explicit failure channel distinct from waiting
7. abandonment safety after completion
8. explicit observability for state/failure/outstanding visibility

## 2) Async API shape implemented

Added a narrow deferred surface in native API:

- `prometheus_reactor_runtime_sgemm_submit_async(...)`
- `prometheus_reactor_runtime_sgemm_query_async(...)`
- `prometheus_reactor_runtime_sgemm_consume_async(...)`
- `prometheus_reactor_runtime_sgemm_abandon_async(...)`

This is intentionally minimal and SGEMM-scoped.

## 3) Lifecycle representation

Runtime now tracks one explicit async token slot with:

- lifecycle state enum (`PROM_ASYNC_STATE_*`)
- task id
- shape/output metadata
- selected path/detail diagnostics
- stage/failure observability fields

## 4) Distinguishing readiness, failure, consumption

The API now surfaces distinct outcomes:

- `PROM_DETAIL_ASYNC_NOT_READY` for use-before-complete
- failure detail via fence/submit diagnostics (`PROM_ASYNC_STATE_FAILED` + detail code)
- `PROM_DETAIL_ASYNC_ALREADY_CONSUMED` for double-consume
- `PROM_DETAIL_ASYNC_INVALID_TASK` / `PROM_DETAIL_ASYNC_NO_TASK` for token misuse

## 5) Ownership hazards prevented/surfaced

- submit while async work is `submitted`/`ready` is rejected (`PROM_DETAIL_REUSE_IN_FLIGHT` or `PROM_DETAIL_ASYNC_UNCONSUMED`)
- backing-resource reuse before completion is blocked structurally
- abandonment after completion is explicit via `..._abandon_async(...)`

## 6) Judgment-engine async policy seam

Async policy selection is now delegated to the judgment seam:

- new facts: `request_async`, `in_flight`, `software_vulkan`
- new decision: allow/reject async + explicit reject detail
- reactor gathers facts and executes decision

Policy retained in judgment engine:

- suppress async on software Vulkan
- reject async when in-flight overlap exists

Reactor retained only mechanics (submit/query/consume/abandon execution).

## 7) Tests proving protocol safety

Added/updated Marionette coverage for:

- safe deferred completion with explicit query + consume
- use-before-complete rejection
- double-consume rejection
- in-flight ownership hazard rejection
- abandonment safety and post-abandon resubmission
- async policy selection in judgment-engine tests (including software suppression)

## 8) Intentionally deferred

- multi-task queue scheduler
- broad promise/future runtime
- FFT async path
- overlap/perf tuning
- multi-token asynchronous batching

## Inconsistency surfaced

M10’s Oct model includes explicit continuation discipline (`remember`/`resume`) and suspension semantics; this C port does not expose a continuation API yet. P8e ports the safety contract (state/progress/readiness/failure/ownership/observability) without introducing a broader continuation runtime surface.
