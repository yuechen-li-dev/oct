# P11 M12 / Prometheus SGEMM Algorithm Lab M42 — Multi-Queue Resource Ownership Readiness Lab

## 1) M10/M11 handoff summary

### What M10 implemented

M10 introduced real worker threads with immutable centrally planned dispatch, while keeping Vulkan execution serialized through a single bridge gate. Worker boundaries stayed strict (no worker judgment, no direct Dominatus mutation), and ordered atomic output commit remained unchanged.

### What M11 hardened

M11 validated race-facing safety around the serialized bridge path: dual-failure first-winner stability, overflow behavior, failure while another worker is active, out-of-order completion with ordered commit, repeated run reset correctness, destroy-after-failure safety, and diagnostics truthfulness.

### Why serialized bridge is safe but not true hardware parallelism

Serialized bridge mode validates concurrency ownership boundaries and failure/drain behavior under real threads, but queue submit still converges through one compute path. That preserves safety but cannot claim true queue-level parallel Vulkan execution.

### What true per-worker queue/resource execution adds

It adds explicit per-worker queue/resource ownership and independent queue submission where topology allows: per-worker command resources, per-worker queue mapping, per-worker completion ownership, and queue-scoped drain/failure coordination.

### What M42 must prove before native implementation

M42 must prove ownership contracts, queue-topology eligibility for hardware-parallel claims, multi-queue failure/drain safety, Dominatus staging safety, serialized fallback clarity, and an actionable M13 candidate order.

## 2) Candidate execution/resource models

M42 model compares:

- **A** serialized bridge baseline,
- **B** per-worker command resources with single compute queue submit serialization,
- **C** true per-worker compute queues/resources,
- **D** per-worker compute resources plus dedicated transfer interaction,
- **E** shared arena/resource lock path.

## 3) Queue topology model

Modeled topologies:

1. **single-compute**: C collapses to one effective worker, no hardware-parallel claim,
2. **two-compute**: C can claim up to two compute workers,
3. **four-compute**: scaling bounded by worker cap and memory cap,
4. **compute-plus-transfer**: D allows ownership-gated transfer overlap,
5. **pseudo-shared**: treated as non-independent queues; no hardware-parallel claim.

## 4) Per-worker resource bundle contract

Required per-worker ownership for true multi-queue execution:

- worker id,
- compute queue id/family assignment,
- command pool,
- command buffer,
- submit fence/timeline marker,
- per-worker event ring,
- per-worker output staging,
- per-worker arena bank or shared-lock handle,
- slot ownership identity,
- failure state,
- in-flight state.

## 5) Arena/slot ownership model

### Local path

Local per-worker arenas/slots are the default readiness path. Memory cost scales by `effective_workers * per_worker_arena_bytes`, so budget directly caps effective workers.

### Shared path

Shared arena path requires explicit lock ownership. Access without lock is modeled as ownership violation and hard failure. Lock contention is explicitly surfaced in metrics.

### Direction

M13 should implement local per-worker resources first; shared arena lock mode remains deferred fallback for memory pressure.

## 6) Synchronization/safety hazards

Modeled hazard classes:

- command resource ownership collision,
- queue submit ownership violation,
- fence ownership violation,
- output staging ownership violation,
- shared arena access without lock,
- worker Dominatus direct mutation violation.

Any ownership violation fails readiness run with explicit failure detail.

## 7) Failure/drain findings

Modeled scenarios:

1. failure before submit,
2. failure after submit,
3. dual failure (first-failure-wins),
4. fence wait failure,
5. device lost global failure,
6. shared arena failure while lock missing,
7. drain timeout unresolved in-flight work.

Contract outcome:

- first failure metadata preserved,
- drain is queue-scoped and required before reuse,
- output remains uncommitted on any failure,
- drain timeout marks resources unsafe-to-reuse and requires cleanup.

## 8) Diagnostics contract

M42 requires publishing:

- execution mode,
- queue topology classification,
- hardware queue count,
- effective workers,
- hardware parallelism claim flag,
- worker→queue mapping,
- per-worker in-flight/fence/event/failure state,
- queue drain count,
- drain timeout count,
- shared lock contention count,
- output committed flag.

## 9) Final recommendation

Model-based recommendation:

- **first M13 implementation candidate:** **B (per-worker command ownership + serialized single-queue submit)**,
- **gate true multi-queue claims (C/D)** behind independent compute queue topology and memory headroom,
- keep single-queue and pseudo-shared topologies on serialized bridge fallback.

## 10) P11 M13 direction

Implement in this order:

1. per-worker command pool/buffer/fence ownership and slot identity,
2. explicit worker→queue mapping diagnostics,
3. serialized single-queue fallback retained,
4. enable true multi-queue execution only on eligible independent queue topology,
5. keep Dominatus commit in drain/join boundary only.

## 11) Deferred scope

Deferred unchanged:

- native multi-queue runtime implementation details,
- work stealing,
- SPMC/MPMC queue designs,
- lock-free queue runtime,
- parallel judgment,
- broad scheduler redesign,
- performance benchmarking sweeps.

## Required final answers

1. **Is Prometheus ready for true per-worker queue/resource execution?**
   - Conditionally yes at contract level; runtime implementation should proceed only after M13 establishes per-worker command-resource ownership and topology gating.
2. **Which candidate should M13 implement first?**
   - Candidate **B**.
3. **Is per-worker command-resource ownership needed before true multi-queue?**
   - Yes, mandatory prerequisite.
4. **Should single-queue hardware keep serialized bridge mode?**
   - Yes.
5. **What queue topology is required to claim hardware parallelism?**
   - At least two independent compute queues (not pseudo-shared), plus memory/resource eligibility.
6. **What per-worker resources are mandatory?**
   - Queue assignment, command pool/buffer, submit fence/timeline marker, event ring, output staging, slot identity, arena ownership handle, in-flight/failure state.
7. **How should failure/drain work across queues?**
   - First-failure-wins, stop new starts, drain/fail all in-flight queues before reuse, no output commit on failure.
8. **How should transfer queue interaction be handled?**
   - Optional overlap only in explicit compute+transfer topology and only with ownership-preserving queue/fence contracts.
9. **What diagnostics are required?**
   - Execution/topology/parallelism/mapping/per-worker states/drain timeout/lock contention/output commit fields listed above.
10. **What remains deferred?**
   - Multi-queue native implementation details, stealing/MPMC/lock-free runtime, parallel judgment, scheduler redesign, broad perf work.

## Language/reference consistency note

This milestone adds executable experiment model/test artifacts under `Experiments/` and does not alter language semantics. No inconsistency with `Language/reference` was identified in this change.
