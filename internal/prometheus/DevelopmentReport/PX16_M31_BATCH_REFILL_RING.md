# PX16 M31 — batch refill executor on the persistent Vulkan submission ring

## Result

The hardware path of `prometheus_reactor_runtime_sgemm_batch(...)` now leases
the existing M29/M30 persistent physical submission ring. It is a one-compute-
queue, deterministic central refill loop. Logical P11 worker assignment stays
on every plan; it is diagnostic/event ownership, not Vulkan resource ownership.

The central schedule is ascending entry ID. It admits entries while an empty
ring slot exists, polls submitted batch jobs, harvests ready jobs into
batch-owned staging, and refills a freed slot with the next entry. If polling
makes no progress while work is outstanding, it waits the oldest submission
only. There is no immediate fence wait after each submit.

## Audit of the prior P11 physical path

`prometheus_reactor_runtime_sgemm_batch_impl` previously planned centrally but
then used `prom_batch_worker_resources`: one command pool, command buffer, and
fence per logical worker. `batch_worker_execute_plans` reset, recorded,
submitted, and waited each entry. The simulated/lane fallback instead directly
called reference SGEMM. The two-slot runtime still mapped logical slots to
worker-owned resources. Command resources were created and destroyed per batch.
Outputs were already batch-owned `staged_outputs` and copied to callers only
after success; worker-local event rings and first-failure drain were also
already present.

M31 keeps that implementation for software/topology contract lanes, but
hardware dispatch no longer enters it. It reuses M30 task-owned host-visible
A/B/C bundles, M30 descriptor recording, and M29 slot/fence/query resources.

## Ownership and failure

Every admitted entry owns an M30-style A, B, and writable C bundle. Inputs are
copied before submission; completed C is copied to separate batch staging.
Caller C pointers remain untouched until all entries succeed, then commit is in
ascending entry-ID order.

First failure stops admission. Submitted entries drain before their buffers are
released; observation uncertainty uses the M30a quarantined-slot reaper and
ordered M30 P14/P15 feedback cursor. A physical slot is never assigned to a
worker, and no plan is reassigned or re-planned.

## Diagnostics

`PrometheusSgemmBatchDiagnostics` now exports configured/effective physical
depth, in-flight/high-water state, submit/poll/wait/ring-full/refill/query,
quarantine/reap, feedback counts, and bounded per-entry submission, slot,
completion, and commit evidence.

## Hardware validation and acceptance

The focused hardware lane is named `PrometheusSgemmBatchRefillRing` (the
milestone identifier is deliberately not part of the permanent test or
artifact name). It calls the public batch API with eight independent entries:
128x128x128, 256x256x64, 64x512x128, 512x64x128, 31x29x23, 17x17x17,
192x320x96, and 96x96x96. Caller output is sentinel-initialized before every
run and compared with the CPU oracle after return.

Hardware: NVIDIA GeForce RTX 3070, vendor 4318, device 9352, discrete type,
driver 2500395008, Vulkan API 4211017, software Vulkan false, compute family
0, transfer family 1.

| depth | wall ns | max in-flight | submits | polls | forced waits | full | refills | query harvests | aggregate GPU ns |
|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| 1 | 10,169,300 | 1 | 8 | 16 | 8 | 21 | 7 | 8 | 381,600 |
| 2 | 9,142,100 | 2 | 8 | 19 | 3 | 10 | 6 | 8 | 390,720 |
| 4 | 8,766,000 | 4 | 8 | 8 | 0 | 1 | 4 | 8 | 391,968 |

Depth two improved wall time about 10% over depth one; depth four improved
about 14% over depth one. GPU timing remained broadly stable, so this is feed
and waiting overhead rather than a kernel claim. Completion and commit evidence
are both entry order in this run; the implementation nevertheless stages and
commits independently. The focused failure injects entry 2 after it has been
submitted: it stops further admission, drains existing ownership, preserves all
caller sentinels, keeps entry 2 as first failure, and commits no output.

Artifacts are `out/test-artifacts/prometheus_sgemm_batch_refill_ring.json` and
`.md`. They contain the hardware facts, depth data, completion/commit order,
and failure proof. Native build, the focused lane, M29, M30, M30a, resident,
EVT, and the P11 slow suite (62 pass, one expected skip) passed; both requested
Go lanes passed.

**Acceptance: ACCEPTED.**

## Scope

M31 adds no public async-batch API, worker threads, work stealing, multiple
compute queues, selector retuning, kernels, SDSL-V, FFT/P16, CUDA, or DX12.
