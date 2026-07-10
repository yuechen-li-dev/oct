# P10 M12 — Async Lifecycle Dominatus Ownership Migration

## M30 addendum — aggregate snapshot semantics

M30 permits multiple public tasks. The existing Dominatus async snapshot is a
compatibility last-event mirror only; it must not be interpreted as a global
task state. `PrometheusAsyncStatus` is authoritative per task and
`PrometheusSgemmAsyncDiagnostics` exposes aggregate counts. M30 completion
feedback is sequenced separately from lifecycle visibility so an early
completion cannot update P14/P15 ahead of an earlier submitted task.

## 1) migration scope

M12 migrates async lifecycle status ownership into Dominatus staged/visible state while preserving the existing single-task async behavior.

In-scope:

- async task identity/lifecycle/status fields
- async lifecycle transition event staging
- transfer-aware async readiness markers
- slot association metadata for async task ownership
- query/consume/abandon read-path projection from Dominatus visible async snapshot
- legacy runtime async fields demoted to compatibility mirrors

Out-of-scope behavior (deferred) remains unchanged.

## 2) async fields migrated

Migrated to Dominatus async domain keys:

- `task_id`
- `lifecycle_state` (`IDLE`, `SUBMITTED`, `READY`, `FAILED`, `CONSUMED`)
- `stage`
- `detail_code`
- `ready`
- `failed`
- `consumed`
- `outstanding_tasks`
- `failure_stage`
- `failure_detail`
- `submit_detail`
- `query_detail`
- `slot_id`
- `slot_generation`
- `owns_slot`
- `transfer_complete`
- `compute_complete`
- `readback_complete`

## 3) async events/transitions staged

Added async lifecycle event kinds:

- `PROM_DOM_EVENT_ASYNC_SUBMITTED`
- `PROM_DOM_EVENT_ASYNC_NOT_READY`
- `PROM_DOM_EVENT_ASYNC_READY`
- `PROM_DOM_EVENT_ASYNC_FAILED`
- `PROM_DOM_EVENT_ASYNC_CONSUMED`
- `PROM_DOM_EVENT_ASYNC_ABANDONED`
- `PROM_DOM_EVENT_ASYNC_INVALID_TASK`
- `PROM_DOM_EVENT_ASYNC_ALREADY_CONSUMED`
- `PROM_DOM_EVENT_ASYNC_UNCONSUMED_REJECTED`

Runtime transitions now stage async snapshot updates through `prom_dom_sgemm_stage_async_snapshot(...)` then commit.

## 4) keys/domains added

Added domain:

- `PROM_DOM_DOMAIN_ASYNC`

Added keys:

- `PROM_DOM_KEY_ASYNC_TASK_ID`
- `PROM_DOM_KEY_ASYNC_LIFECYCLE_STATE`
- `PROM_DOM_KEY_ASYNC_STAGE`
- `PROM_DOM_KEY_ASYNC_DETAIL`
- `PROM_DOM_KEY_ASYNC_READY`
- `PROM_DOM_KEY_ASYNC_FAILED`
- `PROM_DOM_KEY_ASYNC_CONSUMED`
- `PROM_DOM_KEY_ASYNC_OUTSTANDING_TASKS`
- `PROM_DOM_KEY_ASYNC_FAILURE_STAGE`
- `PROM_DOM_KEY_ASYNC_FAILURE_DETAIL`
- `PROM_DOM_KEY_ASYNC_SUBMIT_DETAIL`
- `PROM_DOM_KEY_ASYNC_QUERY_DETAIL`
- `PROM_DOM_KEY_ASYNC_SLOT_ID`
- `PROM_DOM_KEY_ASYNC_SLOT_GENERATION`
- `PROM_DOM_KEY_ASYNC_OWNS_SLOT`
- `PROM_DOM_KEY_ASYNC_TRANSFER_COMPLETE`
- `PROM_DOM_KEY_ASYNC_COMPUTE_COMPLETE`
- `PROM_DOM_KEY_ASYNC_READBACK_COMPLETE`

## 5) source-of-truth ownership model

For migrated async lifecycle fields:

1. Runtime stages async snapshot writes through Dominatus async adapter surface.
2. Commit makes async state externally visible.
3. Query/consume/abandon read visible async snapshot (or mirror from it).
4. Legacy fields (`rt->async_state`, `rt->async_stage`, `rt->async_failure_detail`, etc.) are compatibility mirrors synchronized from visible snapshot, not export authority.

## 6) staged/visible behavior

M12 preserves commit gating:

- staged async updates remain invisible pre-commit
- visible snapshot changes only after commit
- query projection uses visible async snapshot
- same-value writes remain non-dirty at blackboard layer

## 7) transfer-aware readiness

Transfer-aware readiness from M31 is preserved:

- `transfer_complete` is carried in async snapshot
- compute completion is represented explicitly (`compute_complete`)
- ready state is only published when runtime transition reaches ready (existing behavior unchanged)
- transfer/compute readiness reasoning remains diagnosable via committed async markers and existing M31 diagnostics

## 8) slot ownership interaction

Async snapshot now carries slot ownership metadata:

- `slot_id`
- `slot_generation`
- `owns_slot`

This keeps async lifecycle visibility aligned with M29/M11 slot ownership rules and protects stale consume/ownership ambiguity across transitions.

## 9) diagnostics/API compatibility

Public `PrometheusAsyncStatus` shape is unchanged.

`query_async` now projects from Dominatus visible async snapshot fields (`lifecycle_state`, `stage`, `detail_code`, readiness bits, outstanding count), preserving prior contract details.

`consume_async` and `abandon_async` preserve existing rejection semantics (`invalid task`, `not ready`, `already consumed`, failed-state behavior) while staging corresponding async lifecycle events.

## 10) tests added

Added focused M12 adapter tests in `reactor_dominatus_sgemm_adapter_tests.cpp`:

1. async snapshot isolation across commit boundary
2. submitted→ready transfer/readiness marker projection

Existing async runtime Marionette coverage remains intact in:

- `reactor_stub_tests.cpp` (submit/query/consume/failure/abandon/double-consume)
- `reactor_m31_transfer_queue_tests.cpp` (transfer-aware async readiness)
- `reactor_m29_fixed_double_tests.cpp` (slot ownership/failure cleanup interactions)

## 11) deferred scope for M13+

Deferred explicitly:

- N-slot/work stealing
- decision caching / dirty-key optimization
- memory suballocation
- FFT migration
- external event stream API
- persistence/replay
- broad async scheduler redesign

## 12) inconsistency/documentation callout

No `Language/reference` inconsistency was introduced by this native C migration (scope is `internal/prometheus/native`).

Documentation gap to track: async detail fields now include both `detail_code` and per-channel detail keys (`submit_detail`, `query_detail`, `failure_detail`) in Dominatus, but public API still exports a single `detail_code` channel.
