# P11 M15 / Prometheus SGEMM Algorithm Lab M43 — True Multi-Queue Submit Readiness Lab

## 1) P11 M14 handoff summary

### What M14 physically split

M14 established physically distinct per-worker command ownership identities and concrete resources where topology permits:

- per-worker `VkCommandPool`,
- per-worker `VkCommandBuffer`,
- per-worker `VkFence`.

The M43 executable model mirrors that split as deterministic worker-owned command pool/buffer/fence identities plus per-worker queue mapping and in-flight/failure state.

### Why submit is still serialized in M14

M14 still routes submits through a serialized bridge path. This validates ownership boundaries and failure/drain controls but cannot truthfully claim hardware queue-level parallelism.

### What true multi-queue submit adds

True multi-queue adds independent per-worker queue submit/fence timelines behind strict topology gates:

- independent compute queue requirement,
- valid worker→queue mapping,
- explicit queue-family ownership handoff when families differ,
- queue-scoped drain of all in-flight queues,
- first-failure-wins metadata across queues.

### Why N-slot remains deferred

N-slot increases runtime state, synchronization edges, and failure fan-out. M43 model outcomes show the next convergent step is enabling gated true multi-queue first, then validating real behavior before expanding slot multiplicity.

### Why work stealing remains deferred/optional

Stealing/SPMC/MPMC adds scheduling complexity without being required for the M15 readiness question. M43 keeps static partition and demonstrates actionable readiness contracts without queue ownership ambiguity.

## 2) Submit mode candidates

Modeled candidates:

- **A** serialized bridge baseline,
- **B** true multi-queue static partition,
- **C** hybrid topology-gated submit,
- **D** multi-queue with dedicated transfer interaction,
- **E** N-slot/work-stealing reference only.

Result: **C-hybrid-topology-gated** is selected as the computed M16 direction.

## 3) Topology model

M43 models:

1. single compute queue,
2. multiple independent queues in same family,
3. pseudo-shared same-family queues,
4. separate compute families,
5. compute + dedicated transfer queue,
6. multi-queue topology with memory budget limiting workers.

## 4) True multi-queue gates

M43 enforces this contract:

```text
independent_compute_queue_count >= 2
&& effective_workers >= 2
&& per_worker_command_resources_valid
&& per_worker_fences_valid
&& worker_queue_mapping_valid
&& memory_budget_supports_workers
&& no_pseudo_shared_queue
&& no_forced_serialized_flag
&& queue_family_ownership_handoff_if_needed
```

If any gate fails, execution mode is serialized submit bridge with explicit fallback reason.

## 5) Worker/queue mapping contract

Required deterministic per-worker state:

- worker id,
- queue id,
- queue family,
- command pool id,
- command buffer id,
- fence id,
- in-flight state,
- failure state,
- submit count.

Contract rules:

- worker submits only to assigned queue,
- worker records only owned command buffer,
- worker waits owned fence except coordinator-level drain,
- no buffer/fence sharing across workers,
- mapping is deterministic across reruns.

## 6) Synchronization model

M43 models:

- independent queue submit/fence timelines,
- drain waits all active compute queues,
- queue-family ownership handoff requirement for separate families,
- transfer completion sync before compute reads,
- compute completion sync before output commit,
- transfer queue not counted as additional compute lane,
- Dominatus safety via event emission and drain-boundary commit only.

## 7) Failure/drain findings

Modeled failure rakes:

1. before-submit failure,
2. after-submit failure,
3. fence wait failure,
4. one queue hangs while another finishes,
5. device lost global failure,
6. transfer failure while compute queues active,
7. drain timeout unresolved in-flight work,
8. dual failure with first-failure-wins metadata.

Outcome contract:

- preserve first failure metadata,
- stop new submits,
- drain all queue scopes,
- no output commit on any failure,
- unsafe-to-reuse on device lost / queue-hang / drain-timeout.

## 8) Diagnostics contract

Mandatory fields:

- execution mode,
- topology class,
- reported compute queue count,
- independent compute queue count,
- effective workers,
- hardware parallelism eligibility + claim,
- fallback reason,
- worker→queue mapping,
- worker queue family,
- worker command pool/buffer/fence ids,
- worker in-flight/fence states,
- worker submit counts,
- queue drain count,
- drain timeout count,
- queue-family ownership handoff count,
- transfer/compute sync wait count,
- output committed,
- unsafe-to-reuse.

## 9) Final recommendation

**Yes, M15 indicates true multi-queue submit is ready for implementation behind strict topology gates.**

Recommended path for M16:

- implement **C-hybrid-topology-gated** submit,
- enable true multi-queue only when all gates pass,
- keep serialized bridge fallback for single-queue and pseudo-shared devices,
- keep static partition (no stealing).

## 10) P11 M16 direction

Implement next:

1. topology gate evaluation at runtime,
2. deterministic worker→queue assignment,
3. per-worker independent submit/fence on eligible topology,
4. queue-family ownership handoff enforcement,
5. transfer/compute synchronization counters and diagnostics,
6. queue-scoped drain and first-failure-wins metadata retention,
7. serialized bridge fallback path unchanged for ineligible topologies.

## 11) Deferred scope

Remain deferred:

- N-slot implementation,
- work stealing,
- SPMC/MPMC queue runtime,
- lock-free queueing,
- parallel judgment,
- benchmark-driven claims.

## Required final answers

1. **Is true multi-queue submit ready for implementation?**
   - Yes, with hard topology/resource/sync gates.
2. **Should P11 M16 implement true multi-queue or another intermediate bridge?**
   - Implement hybrid topology-gated true multi-queue (candidate C), retaining serialized fallback.
3. **What exact topology gates must pass?**
   - The explicit gate conjunction in Section 4.
4. **How should single-queue and pseudo-shared devices fall back?**
   - Forced serialized bridge; no hardware parallelism claim.
5. **How is worker→queue mapping defined?**
   - Deterministic static mapping with per-worker queue family + command/fence ownership identities.
6. **How does transfer interaction stay separate from compute parallelism?**
   - Transfer queue count is tracked separately and never treated as additional compute lanes.
7. **How should failure/drain work across queues?**
   - First-failure-wins, halt new submits, drain all queues, no output commit on failure, unsafe reuse on unresolved/global failures.
8. **What diagnostics are mandatory?**
   - The complete field list in Section 8.
9. **What remains deferred?**
   - N-slot, stealing/SPMC/MPMC, lock-free queues, parallel judgment, perf benchmarks.
10. **Is N-slot ready after true multi-queue?**
   - Not yet; requires a post-M16 validation pass before readiness.

## Language/reference consistency note

This milestone adds experiment model/test artifacts under `Experiments/` and does not alter language semantics in `Language/`. No direct inconsistency with `Language/reference` was found in this change.
