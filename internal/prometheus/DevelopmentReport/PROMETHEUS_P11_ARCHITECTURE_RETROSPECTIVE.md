# Prometheus P11 architecture retrospective

P11 is historical architecture. R2e2 removed its executable implementation;
this document preserves the reasoning needed to understand and avoid reviving
its accidental contracts.

## Intent and actual operation

P11 tried to make SGEMM batches deterministic and concurrent. It assigned
caller-order entries to logical workers, supported round-robin and contiguous
partitions, allocated per-worker staging and optional command resources, then
ran a lane scheduler. It added worker threads, a serialized Vulkan bridge,
queue-topology experiments, two slots per worker, event rings, and shared
first-failure bookkeeping. It staged outputs before copying them in entry
order, which was a valuable atomicity goal.

In practice its public test entry built `prom_batch_plan` records, allocated
worker/event/slot arrays, selected simulated or threaded execution, optionally
recorded empty command buffers, and produced successful batch results with
`batch_reference_sgemm` on the CPU. Diagnostics described worker resources,
queue maps, slots, events, and synthetic topology. Failure injection could
change worker starts, resource ownership, event capacity, queue caps, fence
behavior, or the result path. Cleanup joined workers and destroyed their
resources. That made a simulator useful for exploration, but not real Vulkan
batch authority.

## What survives

The durable contracts were stable caller-order identity, deterministic logical
partitioning, deterministic first failure, stop admission, skipped tail,
safe drain, atomic publication, ordered commit, runtime reuse, and truthful
attribution. They survive as R2b immutable plans, R2c entry runtime state and
failure reducer, M31 centralized refill, M29 slots, M30 task ownership, M30a
quarantine/reap, R2d public routing, and the permanent M31 authority tests.

## What was rejected

Logical workers welded planning to physical ownership. Worker-local pools,
buffers, fences, descriptors, and slots had no independently owned queue
authority. Empty submissions were not SGEMM. CPU reference results were an
oracle but not native execution. Simulated multi-queue topology, queue-count
as-width, and event-overflow-as-correctness confused instrumentation with
authority. Thread and queue counts were not a scheduling-width contract.

```text
P11 (deleted)
entries -> worker plans -> worker threads / simulated lanes
        -> worker-local resources -> optional empty submit
        -> CPU reference result -> staged copy

M31 (current)
entries -> immutable plans -> entry-order centralized admission
        -> M30 task -> M29 shared slot -> Vulkan SGEMM
        -> M30a resolution -> staged ordered commit
```

## Migration table

| P11 concept | Old implementation | Retained semantic | Replacement/test | Deletion reason |
|---|---|---|---|---|
| planner/lane map | worker plan array | deterministic partition | `PrometheusSgemmBatchPlanDeterministic*` | workers were false ownership |
| first failure | shared worker state | stable primary cause | `PrometheusSgemmBatchFirstFailureDeterministic` | reducer is explicit |
| admission/drain | worker loop/join | stop, skipped tail, safe lifecycle | R2c batch tests, M30a | real task lifecycle exists |
| output staging | worker buffers | atomic ordered commit | refill/commit tests | retained by M31 |
| worker resources | per-worker Vulkan objects | none | M29 generation tests | slots own physical resources |
| topology bridge | simulated queues/threads | actual queue facts only | unsupported-topology test | no physical authority |
| CPU batch result | `batch_reference_sgemm` | test oracle only | Vulkan correctness tests | not native result authority |
| event ring | bounded worker events | attribution from real facts | M31 diagnostics | overflow must not decide correctness |

## Reimplementation guidance

Worker-local Vulkan resources can return only when a worker owns a real,
independently owned queue or recording executor and its measured benefit
outweighs the ownership cost. Real multi-queue work requires real queue
authority, measured benefit, and explicit lifetime/synchronization policy.
Host threads require a demonstrated host recording or scheduling bottleneck.
Bounded tracing may be added only when it cannot become result authority. CPU
SGEMM remains a test oracle. Logical lanes must always remain separate from
physical ownership.

## Evidence index and lessons

Historical evidence: `P11_M6_STATIC_PARTITION_BATCH_SKELETON.md`,
`P11_M10_REAL_WORKER_THREAD_BRIDGE.md`, `P11_M16_TRUE_MULTI_QUEUE_SUBMIT.md`,
the P11 validation reports, R2a, R2b, R2c, R2d, R2e0/R2e0b,
`PROMETHEUS_R2E1_M31_BATCH_EXTRACTION.md`, and
`PROMETHEUS_R2E_P11_REMOVAL.md`. Permanent replacement evidence lives in the
M29/M30/M30a/M31 and `PrometheusSgemmBatch*` Marionette tests.

Diagnostics are not execution authority. Tests can preserve implementation
accidents. Logical scheduling and physical ownership must remain separate.
Parallel replacement was safer than in-place mutation; real seams made the
R2e1 split and R2e2 deletion possible. Preserve knowledge, not dead machinery,
and prove authority with physical work and outputs.
