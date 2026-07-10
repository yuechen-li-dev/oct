# PX16 M30 — multi-token public async SGEMM

M30 moves the public SGEMM async entry points off the legacy singleton submit
fence and onto the M29 physical submission slots.  The public task table is
bounded to four records; normal public admission uses two physical slots.

Each public ID encodes a table index and a generation.  Lookup verifies both,
so a released/reused record cannot be addressed by a stale ID.  A task owns
three host-visible storage buffers (A, B, and C) until it is consumed or
abandoned.  Input data is copied before submit and C remains task-owned, which
makes reverse consumption safe and avoids writable-buffer aliasing.

Admission polls the two physical fences once and returns
`PROM_DETAIL_ASYNC_QUEUE_FULL` when no slot is available; it never waits.
Query only polls its associated fence.  Submitted tasks cannot be abandoned;
ready and failed tasks can be explicitly abandoned.  Failed records remain
visible and block new submissions until explicitly abandoned, preserving the
existing safety contract.

The compatibility Dominatus async snapshot remains a last-event mirror; the
per-task `PrometheusAsyncStatus` result is authoritative for multi-token state.
The conservative M30 path uses the compute queue only and does not reuse the
legacy transfer fence or semaphore.  Batch refill remains M31 work.

## Ordered completion evidence

Every task freezes its public ID, task sequence, shape, path/compute metadata,
physical-slot generation, and timestamp result at submission/completion.  M30
uses a dedicated `next_async_feedback_sequence` cursor: completion observation
may be out of order, but P14/P15 commit only when the next task sequence is
terminal.  P14 consumes a valid per-task GPU duration through the existing
measurement filter. P15 consumes the resulting filtered evidence and physical
readiness observation through its predictor/correction path. Failed tasks and
tasks without a valid timestamp commit an explicit skipped event; they advance
the cursor without inventing a measurement. This never blocks admission.

`PrometheusSgemmAsyncDiagnostics` exposes bounded capacity, lifecycle counts,
queue-full/stale-ID counters, max in-flight depth, feedback counters/cursor,
and one record per task-table position. Public async semantics are bounded and
non-blocking: submit returns queue-full rather than waiting, query polls,
consume is task-specific, submitted-task abandon is rejected, and a failed
task remains visible until explicit abandon.

## Hardware evidence and P11 compatibility

The focused M30 lane queries `PrometheusVulkanDeviceDiagnostics` directly from
the selected Vulkan physical device and writes name, vendor/device IDs, device
type, driver/API versions, software flag, and selected compute/transfer queue
families into its artifacts. It skips as *hardware proof unavailable* unless
the selected device is a non-software discrete `NVIDIA GeForce RTX 3070`.

The P11 M13 topology assertion was stale, not an M30 queue regression. Enum
`2` is `PROM_BATCH_QUEUE_TOPOLOGY_PSEUDO_SHARED`: several reported compute
queues collapsing to one independent compute lane. Enum `5` is
`PROM_BATCH_QUEUE_TOPOLOGY_COMPUTE_PLUS_TRANSFER`: the runtime has an enabled
dedicated transfer family distinct from the compute family. The M31 transfer
queue discovery fact correctly selects `5`; M30 neither changes queue-family
selection nor enables multi-queue execution. The test now asserts the named
compute-plus-transfer topology.
# M30a follow-up

M30a supersedes the former implication that abandoning a failed public task
immediately made its submission slot reusable. Observation-failed submissions
now enter physical-slot quarantine and are reclaimed only after fence-confirmed
completion; see `PX16_M30A_ASYNC_QUARANTINE_REAP.md`.

The RTX 3070 M30a lane proves sticky observation failure, different-slot
replacement, explicit queue-full pressure, ordered skipped feedback, safe
reaping, stale IDs, and destruction drain. M30b completes final M30/M30a
acceptance with the remaining query-result and device-lost class evidence plus
green P11 and Go validation.

M30b closes those lifecycle seams with async-only test controls: post-fence
query failure remains physically safe and reusable, while deterministic
post-submit device-loss classification transitions the runtime to fatal,
non-reusable teardown ownership. The M30a hardware lane records both cases in
the M30 JSON artifact.
