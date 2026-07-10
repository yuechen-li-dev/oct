# Prometheus Px16 M29 — persistent SGEMM submission ring

## Scope and ownership

M29 adds a resident-only, single-compute-queue physical submission ring.  Runtime creation allocates four persistent physical bundles; a diagnostic selects depth 1, 2, or 4 (the intended default is 2).  Each bundle has its own command buffer, fence, descriptor set, and timestamp-query pair.  Legacy command/fence/descriptor/query handles remain owned by the synchronous and existing public async paths.

The resident ring intentionally binds the same prepared device-local A/B/C buffers in every slot.  This is valid only for this diagnostic because each SGEMM fully overwrites C, dispatches remain in one queue's deterministic submission order, and readback happens after drain.  It is not public multi-token output ownership.

## Lifecycle

`EMPTY -> PREPARING -> RECORDED -> SUBMITTED -> COMPLETE -> READY -> EMPTY` is represented by the M29 slot state. Acquisition is rotating, increments a generation, and assigns a monotonically increasing submission sequence. `FAILED` is terminal for the current diagnostic run and is never silently recycled.

Submit resets only the acquired slot's fence and returns immediately. Poll uses `vkGetFenceStatus`; after signal, it reads only that slot's timestamp pair with no WAIT flag and transitions through COMPLETE to READY. When the producer is full, it polls all slots and waits only the oldest outstanding sequence if none completed. Cleanup performs `vkDeviceWaitIdle` before destroying slot-owned fences, pool-owned command buffers, descriptors, and query pool.

The focused Marionette lane `PrometheusSgemmPx16M29SubmissionRing` emits `out/test-artifacts/prometheus_sgemm_px16_m29_submission_ring.{json,md}` for depth 1/2/4 and asserts depth greater than one for ring depths greater than one.

## Old versus new resident feed path

M28: update one descriptor set, record one command buffer (possibly many dispatches), submit, wait one fence, read query 0/1.

M29: acquire a physical slot, update only its descriptor, record exactly one dispatch and its private query pair, submit its private fence, then later poll/harvest. This proves multiple *queue submissions* can be outstanding; it is not command-buffer batching.

## Current async progress implementation

`update_async_progress(...)` is in `reactor_vulkan_sgemm.c`, called by async query, consume, and abandon-related entry paths. It only runs when the singleton `rt->async_state` is `SUBMITTED`.

1. It polls: it uses `vkGetFenceStatus`, never `vkWaitForFences`.
2. It first polls the singleton `rt->transfer_submit_fence` when a transfer queue was used, then polls the singleton compute `rt->submit_fence`.
3. It does not read timestamps or query-pool state at all. A successful compute fence makes the task ready; errors or injected poll failure make it failed.
4. On compute completion it clears `rt->in_flight_submit`, calls `prom_slot_mark_complete` for `rt->slot_diag.async_slot_id`, and calls `set_async_state(READY, ...)`; failures call `prom_slot_mark_failure` and set FAILED.
5. It does no readback/copies. Output readback is deferred to async consume.
6. `set_async_state` stages/commits the Dominatus-visible async snapshot. Transfer completion/failure telemetry stages separate Dominatus transfer snapshots. This function does not perform P14 measurement-filter updates or P15 predictor/correction updates.
7. It assumes exactly one task id/state, one async slot id (within the existing two logical slots), one compute fence, one optional transfer fence, one `in_flight_submit` bit, and one final-detail field.
8. M30 must replace those singleton fields with per-token task records owning (or referencing) an M29 physical slot generation; poll/harvest slots independently; retain immutable task decision metadata; perform per-task output/readback ownership; publish task-specific lifecycle snapshots; and commit P14/P15 completion feedback strictly in submission-sequence order without blocking admission.

## Limitations and M30 recommendation

M29 deliberately does not migrate production synchronous SGEMM, the public async ABI, batch execution, selector behavior, P14/P15 policy semantics, kernels, or queue topology. M30 should consolidate duplicate resident recording only after it introduces per-task writable-output ownership and ordered completion attribution. No multi-queue claim is made here.

## Hardware validation

Validated on **NVIDIA GeForce RTX 3070** (Vulkan 1.4.329, driver 596.144.0) on 2026-07-10. The Visual Studio environment came from `C:\Program Files\Microsoft Visual Studio\18\Community\Common7\Tools\VsDevCmd.bat`.

Commands executed:

```text
cmd /c "call \"C:\Program Files\Microsoft Visual Studio\18\Community\Common7\Tools\VsDevCmd.bat\" -arch=x64 -host_arch=x64 >nul && internal\prometheus\native\build_windows.cmd"
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16M29SubmissionRing
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16Resident
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16Evt_CorrectnessValidationLane
out\prometheus\native\marionette_tests.exe PrometheusReactor_AsyncDeferredCompletionIsExplicitlyObservable
out\prometheus\native\marionette_tests.exe PrometheusReactor_AsyncUseBeforeCompleteAndDoubleConsumeAreRejected
out\prometheus\native\marionette_tests.exe PrometheusReactor_AsyncInFlightOwnershipAndAbandonmentAreSafe
out\prometheus\native\marionette_tests.exe PrometheusReactor_AsyncFailureRemainsVisibleUntilExplicitAbandon
go test ./internal/prometheus/... ./cmd/oct
go test ./internal/... ./cmd/oct
```

All commands passed. The M29 lane is a normal Marionette `FACT`, so the actual focused lane is in `marionette_tests.exe`, not the benchmark executable.

| depth | physical slots | max in flight | submits | polls | forced waits | ring full | recycles | query harvests | wall ns/dispatch | GPU ns/dispatch | correctness | failures | final slot states |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---:|---|
| 1 | 4 | 1 | 34 | 32 | 34 | 32 | 33 | 34 | 425,503 | 366,364 | pass | 0 | READY, EMPTY, EMPTY, EMPTY |
| 2 | 4 | 2 | 34 | 92 | 34 | 30 | 33 | 34 | 378,459 | 366,544 | pass | 0 | READY, READY, EMPTY, EMPTY |
| 4 | 4 | 4 | 34 | 203 | 34 | 28 | 32 | 34 | 378,390 | 366,044 | pass | 0 | READY, READY, READY, READY |

Artifacts: `out/test-artifacts/prometheus_sgemm_px16_m29_submission_ring.json` and `out/test-artifacts/prometheus_sgemm_px16_m29_submission_ring.md`.

Depth 2 improved wall time per dispatch by **11.06%** over depth 1. Depth 4 improved by **11.07%**. GPU timing stayed stable (366.0–366.5 microseconds per dispatch), so the gain is CPU feed/synchronization overlap rather than a kernel change. Depth 4 did not materially improve beyond depth 2; it increased non-blocking polls (92 to 203) while forced waits remained the bounded progress/drain mechanism. Ring-full events declined as depth rose. This agrees with M28: host-side feed depth matters while kernel time remains essentially stable.

One reporting-only correction was made during validation: GPU timing now reports the mean across all harvested slot query pairs rather than the final harvested slot. Submission, lifetime, and synchronization behavior were unchanged. The focused run had zero resource-reuse failures; state ownership prevents reset, descriptor update, and query reuse until a slot is harvested READY.

**Acceptance: ACCEPTED.** The physical ring reached true depth 2 and 4 with distinct Vulkan submissions, produced correct results, preserved resident/EVT/singleton-async compatibility, and passed both requested Go lanes. The remaining limitation is intentionally scoped: M29 is resident-only and does not provide public multi-token async output ownership.
# M30a ownership note

The M29 ring's `EMPTY` transition is now additionally guarded for public async
work: a logical M30 task failure does not authorize command-buffer, descriptor,
query-pair, or fence reuse. M30a introduces `QUARANTINED` until its fence is
confirmed by the async reaper.
