# Prometheus concurrency future directions

> **No implementation is authorized by this document.** It is a design/TODO
> record for work justified by future evidence, not a roadmap commitment.

## Current architecture

The authoritative baseline is one host refill coordinator, one real Vulkan
compute queue, and one shared physical submission ring. Logical lanes are
immutable batch metadata only. `reactor_batch.c` layers multi-entry staging and
publication over the shared async/ring machinery because that is the smallest
real ownership model with proven lifecycle behavior.

## Vocabulary

- **batch:** multi-entry planning and atomic ordered publication.
- **async task:** a public token lifecycle record.
- **scheduler:** admission and ordering policy.
- **physical ring:** reusable physical submission slots.
- **executor:** a real physical execution authority, such as a Vulkan queue
  plus its local resources.
- **logical lane:** immutable planning metadata, never a resource owner.
- **host recording worker:** a possible CPU command-preparation participant.
- **Vulkan queue:** actual device submission authority.
- **operation / compute implementation / shader / pipeline / dispatch:**
  respectively requested work, its selected realization, program, bound
  executable state, and one recorded GPU invocation.

Batch owns multi-entry publication semantics; async owns token lifecycle; a
scheduler owns admission/order policy; a ring owns reusable slots; an executor,
if introduced, owns a real queue and local resources.

## Parallel host command recording

Possible future shape:

```text
central scheduler -> multiple host recording workers -> one shared submission authority -> one Vulkan queue
```

This is justified only if profiling finds command recording/host preparation a
bottleneck, Vulkan command-pool external synchronization requires worker-local
pools, and measured speedup exceeds complexity. Identity must remain stable;
workers must not own lanes or results, and one publication contract remains.

## Queue-owned executors

Worker-local Vulkan resources become legitimate only when each worker is a
real independently owned physical executor:

```text
executor 0: queue 0, local command pool/ring/resources
executor 1: queue 1, local command pool/ring/resources
```

Prerequisites are real multiple-queue authority, hardware support, measured
benefit, explicit queue-family/ownership semantics, failure isolation,
resource-lifetime rules, no empty/decorative submissions, and one global batch
publication contract.

## Logical jobs and work stealing

The conceptual model is many lightweight immutable logical jobs, bounded
executor-local ready deques, idle executors stealing eligible jobs, and
physical ownership beginning only after executor admission. This is not a copy
of Go runtime internals. Deques may have owner-local push/pop with bounded
steal attempts, or a globally coordinated eligibility layer; selection needs
evidence before design.

Stealing changes placement, never identity. Plans remain immutable;
deterministic mode remains available; dependencies and ordering stay explicit;
failure reduction is placement-independent; lanes never weld to executors;
publication is atomic and ordered; and no resource is stolen after physical
submission without explicit transfer semantics.

## Deterministic and adaptive modes

Deterministic mode uses stable admission and placement for reproducible traces,
correctness debugging, and benchmark comparison. Adaptive mode may choose
executors dynamically, steal work, and apply latency/throughput policy, while
retaining identical numerical and publication contracts. Neither mode is
implemented here.

## Policy and multiple operations

Future policy may consider latency versus throughput, estimated dispatch
length, memory pressure, starvation bounds, operation-class quotas,
device-specific queue strategy, deadline/priority hints, and fairness between
clients. Policy must never become physical-ownership identity.

Potential operations include SGEMM, FFT, reductions, copies, and future
compute/graphics tasks. Do not introduce a generic operation graph before a
second real operation proves it; any job representation must be driven by real
FFT or reduction execution, not speculation.

## Failure and lifecycle requirements

Any future architecture preserves deterministic primary failure,
secondary/fatal safety classification, stop admission, safe drain/reap/
quarantine, no early recycle, atomic publication, ordered commit, token and
slot generation safety, safe teardown, and explicit device-loss behavior.

## Rejected anti-patterns

Never revive logical lanes welded to physical resources, fake multi-queue
state, empty submissions as authority proof, CPU fallback hidden in native
success, event overflow affecting correctness, thread count as queue count,
worker count as ring depth, broad managers/factories/providers without real
variation, or speculative generic frameworks before evidence.

## Experimental gates

Before implementation require profiling evidence, a minimal reproducer,
hardware support matrix, measurable bottleneck, baseline comparison,
correctness-authority tests, failure injection, teardown proof, rollback path,
and an explicit complexity budget.

## Nonbinding possible milestones

- C0: host feed-path profiling.
- C1: parallel command-recording experiment.
- C2: single-queue recording-worker prototype.
- C3: real multi-queue capability audit.
- C4: queue-owned executor prototype.
- C5: deterministic executor placement.
- C6: adaptive work stealing.
- C7: multi-operation scheduling after real FFT.

These are ideas, not commitments.

## TODO

- Profile current host bottlenecks and command-recording materiality.
- Audit real queue multiplicity by device and measure shared-ring saturation.
- Design executor ownership only after evidence; preserve deterministic mode
  and logical/physical separation.
- Revisit after a compute implementation registry and real FFT execution.
