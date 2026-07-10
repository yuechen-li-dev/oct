# PX16 M30a — async quarantine and physical-slot reaping

M30a makes the ownership boundary explicit: a public async task becoming
`FAILED` or being abandoned is not evidence that Vulkan has stopped using its
command buffer, descriptor set, query pair, fence, or task-owned buffers.

## Lifecycle contract

Public records use `IDLE`, `SUBMITTED`, `READY`, `FAILED`, and `CONSUMED`.
Physical slots independently use `EMPTY`, `PREPARING`, `RECORDED`,
`SUBMITTED`, `COMPLETE`, `READY`, `QUARANTINED`, and `FAILED_FATAL`.
An observation error moves the public record to `FAILED`, creates its ordered
skipped-feedback event, and moves a still-uncertain physical slot to
`QUARANTINED`. Only the reaper, after `vkGetFenceStatus` confirms completion,
can return that slot to `EMPTY`.

| Failure class | Public state | GPU may own resources | Reuse | Cleanup |
| --- | --- | --- | --- | --- |
| Pre-submit allocation/record/fence-reset/submit failure | FAILED/released | No | immediate after local cleanup | destroy task buffers, `EMPTY` |
| `VK_NOT_READY` | SUBMITTED | Yes | no | retain submission |
| Poll/fence observation uncertainty | FAILED | Possibly | no: QUARANTINED | nonblocking reap or destroy drain |
| Query failure after signaled fence | FAILED | No | after ownership resolution | skip feedback, retire normally |
| Device lost/global failure | FAILED | unknown | never ordinary reuse | mark runtime unsafe; device teardown |

## Failure-class validation matrix

| Class | Injection/evidence | Expected physical result | Status |
| --- | --- | --- | --- |
| Validation | zero dimensions/null inputs reject before allocation | EMPTY; reusable | covered by API validation paths |
| Buffer allocation | `PROM_TESTCFG_FAIL_BUFFER_ALLOC` | no submission; local cleanup | covered by existing allocator injection |
| Command recording | `PROM_TESTCFG_FAIL_COMMAND_END` before submission | EMPTY; reusable | pre-submit hook wired for async |
| Fence reset | `PROM_TESTCFG_FAIL_RESET_FENCE` before queue submit | EMPTY; reusable | pre-submit hook wired for async |
| Queue submit | `PROM_TESTCFG_FAIL_QUEUE_SUBMIT` is injected **before** `vkQueueSubmit` | EMPTY; reusable | pre-submit hook wired for async |
| Normal not-ready | ordinary async polling | SUBMITTED; not reusable | covered by M30/M30a admission paths |
| Observation | one-shot `PROM_TESTCFG_FAIL_ASYNC_POLL` | QUARANTINED until reap | RTX 3070 M30a lane passes |
| Query result after confirmed fence | no dedicated safe injector | COMPLETE/FAILED, no quarantine | documented as unexercised |
| Device lost | no safe test injector in public async path | FAILED_FATAL; runtime unsafe | documented as unexercised; no fabricated device loss |

## Abandon and feedback

Abandoning a failed quarantined task releases caller interest but preserves a
minimal task tombstone and its buffers until the fence is known. Its ID remains
failure-query-visible while pending; after reap it is stale. Submitted-task
abandon remains rejected: there is no GPU cancellation. Ready and
physically-complete failures can retire normally.

The P14/P15 sequence cursor receives exactly one terminal skipped event when
observation failure is declared. Later reaping deliberately discards timing
instead of producing duplicate evidence, so an uncertain task cannot deadlock
later feedback.

## Reap and destruction

Submit, query, consume, abandon, and diagnostics run a nonblocking reaper.
It polls quarantined fences without spinning; `VK_NOT_READY` leaves ownership
unchanged. Runtime cleanup invokes its blocking drain before `vkDeviceWaitIdle`
and before any slot-owned buffers, descriptors, command resources, fences, or
queries are destroyed. Device-lost results mark ordinary reuse unsafe.

M31 remains out of scope: no batch migration, worker scheduler, multi-queue
work, selector tuning, kernel/SDSL-V, FFT/P16, CUDA, or DX12 changes are part
of this work.

Hardware evidence for the focused lane is NVIDIA GeForce RTX 3070 (vendor
4318, device 9352, discrete type 2), hardware Vulkan, compute family 0 and
transfer family 1. The lane was run as
`marionette_tests.exe PrometheusSgemmPx16M30aAsyncQuarantineReap`.

Acceptance judgment: **ACCEPTED**. M30b safely exercises the two former
coverage gaps, and the complete hardware/native/Go validation listed below is
green. The machine-readable artifact sets `acceptance_evidence_complete: true`.

## M30b test-only completion seams

M30b adds a separate `async_test_flags` word so the legacy 32-bit test flag
space is not repurposed. `PROM_ASYNC_TESTCFG_FAIL_QUERY_RESULT` is checked
only after `vkGetFenceStatus` has signaled and before query harvesting; it
uses a deterministic error result rather than making an invalid Vulkan call.
The task is `FAILED` with class `QUERY`, physical completion remains confirmed,
the slot is not quarantined, feedback is skipped once, and normal reuse is
allowed after abandon.

`PROM_ASYNC_TESTCFG_DEVICE_LOST_AFTER_SUBMIT` is an internal classification
seam, not a simulated driver reset. After a real async submission it marks all
submitted public records `DEVICE_LOST`, transitions their physical slots to
`FAILED_FATAL`, advances skipped feedback, and marks the runtime unsafe for
ordinary submission. Destruction then owns the teardown path; no command
buffer, descriptor, query pair, or fence is recycled.

The RTX 3070 M30a/M30b lane passes the observation, query-result, and
device-loss rows. Focused native validation passed: M30a, M30, Async (12/12),
M29 submission ring, resident, EVT correctness, and P11 (11/11). Go validation
also passed without a foreground timeout: `go test ./internal/prometheus/... ./cmd/oct`
(`cmd/oct` 76.274s) and `go test ./internal/... ./cmd/oct` (`cmd/oct` 78.159s).
