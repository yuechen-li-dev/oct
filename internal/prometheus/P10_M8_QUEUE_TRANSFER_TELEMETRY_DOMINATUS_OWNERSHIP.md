# P10 M8 — Queue / Transfer Lifecycle Telemetry Dominatus Ownership

## 1) Migration scope

M8 migrates deferred M7 queue/transfer runtime telemetry into Dominatus staged/visible ownership.

In-scope:

- queue handoff and transfer/compute wait counters
- transfer failure slot/reason/count telemetry
- async transfer completion marker + completion generation telemetry
- queue lifecycle event staging for handoff, wait, completion, and failure
- diagnostics export sourcing of migrated runtime telemetry from visible Dominatus state
- compatibility mirror refresh in `slot_diag` from visible snapshot only

Out-of-scope queue behavior/policy logic remains unchanged.

## 2) Telemetry fields migrated

Migrated to Dominatus keys and visible projection:

- `m31_queue_family_handoff_count`
- `m31_transfer_compute_wait_count`
- `m31_transfer_failure_slot_id`
- `m31_transfer_failure_reason`
- transfer failure count (internal telemetry key; not yet public API field)
- `m31_async_transfer_complete`
- async transfer completion generation (internal telemetry key for sequencing)

## 3) Keys/events added or used

### Keys used/expanded

- reused: `PROM_DOM_KEY_QUEUE_HANDOFF_COUNT`
- added:
  - `PROM_DOM_KEY_QUEUE_TRANSFER_COMPUTE_WAIT_COUNT`
  - `PROM_DOM_KEY_QUEUE_TRANSFER_FAILURE_SLOT_ID`
  - `PROM_DOM_KEY_QUEUE_TRANSFER_FAILURE_REASON`
  - `PROM_DOM_KEY_QUEUE_TRANSFER_FAILURE_COUNT`
  - `PROM_DOM_KEY_QUEUE_ASYNC_TRANSFER_COMPLETE`
  - `PROM_DOM_KEY_QUEUE_ASYNC_TRANSFER_COMPLETION_GENERATION`

### Events

- reused: `PROM_DOM_EVENT_QUEUE_HANDOFF`, `PROM_DOM_EVENT_TRANSFER_COMPLETE`
- added:
  - `PROM_DOM_EVENT_TRANSFER_FAILED`
  - `PROM_DOM_EVENT_TRANSFER_WAIT`

### Adapter surface added

- `prom_dom_sgemm_stage_transfer_handoff(...)`
- `prom_dom_sgemm_stage_transfer_wait(...)`
- `prom_dom_sgemm_stage_transfer_failure(...)`
- `prom_dom_sgemm_stage_transfer_complete(...)`
- `prom_dom_sgemm_read_visible_transfer_runtime_telemetry(...)`

## 4) Source-of-truth ownership model

After M8, queue domain ownership is:

- transfer policy facts/decision: Dominatus (from M7)
- transfer runtime telemetry: Dominatus (M8)
- `slot_diag`: compatibility mirrors only for migrated queue telemetry

Runtime flow for migrated telemetry:

1. runtime stages telemetry via adapter APIs
2. commit promotes telemetry to visible state
3. diagnostics export reads visible Dominatus snapshot
4. `slot_diag` mirror fields refresh from visible snapshot

`slot_diag` no longer acts as source-of-truth for migrated transfer telemetry.

## 5) Staged/visible behavior

M8 preserves anti-tearing semantics:

- staged runtime telemetry remains invisible pre-commit
- commit boundary controls external visibility
- dirty queue keys/domains update at commit
- committed event ring + trace ring record lifecycle metadata for handoff/wait/failure/complete

## 6) Diagnostics export behavior

`prom_reactor_runtime_sgemm_policy_diagnostics_impl(...)` now sources migrated M31 runtime telemetry from `prom_dom_sgemm_read_visible_transfer_queue_diagnostics(...)` when visible snapshot exists, including:

- queue handoff count
- transfer/compute wait count
- transfer failure slot/reason
- async transfer completion marker

Fallback to `slot_diag` remains for compatibility if visible transfer snapshot is unavailable.

## 7) Compatibility mirror behavior

For migrated queue telemetry fields:

- `slot_diag` mirror fields are synchronized from visible Dominatus transfer snapshot
- staged writes do not drift compatibility mirror export pre-commit
- post-commit mirror/export values update together from visible projection
- direct `slot_diag` mutation is no longer authoritative for migrated transfer telemetry

## 8) Tests added

Added focused M8 coverage in `reactor_dominatus_sgemm_adapter_tests.cpp`:

1. handoff/wait staged visibility across commit boundary
2. transfer failure visibility + dirty queue domain + failure event/trace metadata
3. async transfer completion marker + generation commit boundary behavior

Runtime compatibility retained via existing M31 transfer queue suite plus full Marionette native suite.

## 9) Deferred scope for M9+

Deferred explicitly:

- upload+readback transfer policy enablement
- path/compute/precision selector migration
- Packed4/FP16 selector migration
- full slot diagnostics ownership migration
- queue scheduler / N-slot work-stealing
- memory suballocation
- FFT migration
- external public event stream API

## 10) Inconsistency/documentation callouts

- M8 introduces queue telemetry keys beyond the original 64-key blackboard budget, requiring multi-word dirty-key tracking. This is now implemented by expanding key-word storage from one 64-bit word to two words.
- Transfer failure count and async completion generation are now tracked internally in Dominatus but remain outside the public diagnostics API surface; this is an intentional visibility gap for future API alignment work.
