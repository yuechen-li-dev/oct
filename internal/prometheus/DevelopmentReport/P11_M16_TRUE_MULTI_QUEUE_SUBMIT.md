# P11 M16 — Hybrid Topology-Gated True Multi-Queue Submit

## 1) M43 handoff summary

M43 recommended candidate **C (hybrid topology-gated submit)**:

- if strict topology/resource/synchronization gates pass, use true multi-queue static-partition submit,
- otherwise preserve serialized bridge fallback,
- keep N-slot, stealing, SPMC/MPMC, lock-free queues, parallel judgment, and perf claims deferred.

## 2) Implemented topology gates

M16 now evaluates the M43 gate contract at runtime before selecting true multi-queue:

- `independent_compute_queue_count >= 2`
- `effective_workers >= 2` (post memory + queue cap)
- `per_worker_command_resources_valid`
- `per_worker_fences_valid`
- `worker_queue_mapping_valid`
- `memory_budget_supports_workers`
- `no_pseudo_shared_queue`
- `no_forced_serialized_flag`
- `queue_family_ownership_handoff_if_needed`

If any gate fails, mode remains serialized and a concrete fallback reason is published.

## 3) Topology classification

M16 classifies batch topology as:

- single compute queue,
- pseudo-shared queue topology,
- parallel-eligible compute topology,
- compute + dedicated transfer queue,
- memory-capped worker topology,
- forced-serialized topology.

Current production discovery remains conservative for queue independence: queue-family queue count is reported, but independent compute queues are only promoted via explicit test hook (`PROM_TESTCFG_FORCE_DIRECT_PATH` + hardware-cap override) for Marionette coverage.

## 4) Worker→queue mapping

Worker mapping is deterministic and static:

- `queue_index = worker_id % independent_compute_queue_count`.

M16 validates mapping bounds, publishes per-worker queue index and queue family diagnostics, and rejects true multi-queue selection when mapping gate fails.

## 5) True multi-queue submit path

When all gates pass:

- execution mode is `PROM_BATCH_EXECUTION_REAL_THREADS_TRUE_MULTI_QUEUE`,
- serialized bridge is bypassed,
- each worker submits its own command buffer/fence on its mapped compute queue,
- per-worker submit/wait/fence diagnostics are maintained,
- hardware parallelism eligibility + claim are both surfaced.

Static partitioning remains unchanged (no stealing).

## 6) Serialized fallback behavior

When any gate fails:

- existing serialized bridge execution path remains active,
- existing M10/M11/M13/M14 serialized invariants remain intact,
- hardware parallelism claim remains false,
- fallback reason diagnostics are explicit.

## 7) Queue-family ownership / sync handling

Current multi-queue support is same-family only.

- If separate-family ownership handoff is required and not supported, true multi-queue is gated off.
- Ownership handoff diagnostics are surfaced in batch diagnostics.

Separate-family true multi-queue submit remains deferred.

## 8) Transfer queue interaction

M16 preserves transfer separation:

- transfer queue is not counted as a compute lane,
- transfer/compute synchronization wait diagnostics are published,
- transfer handoff counters remain surfaced.

## 9) Failure / drain behavior

M16 keeps first-failure-wins behavior and extends queue-drain diagnostics:

- first failure remains authoritative,
- no output commit on failure,
- queue drain counts are reported,
- unsafe-to-reuse is set on selected fence/submit failure classes.

## 10) Diagnostics added

New batch diagnostics include:

- reported compute queue count,
- independent compute queue count,
- true multi-queue selected flag,
- hardware parallelism eligible,
- serialized fallback reason,
- per-worker queue family,
- per-worker fence state,
- queue drain count,
- drain timeout count,
- queue-family ownership handoff count,
- transfer/compute sync wait count,
- unsafe-to-reuse flag.

## 11) Tests added

Added Marionette M16 coverage for:

1. single-queue serialized fallback,
2. independent two-queue hook selection,
3. memory cap fallback,
4. transfer queue not counted as compute lane.

Legacy P11 M6/M7/M8/M10/M11/M13/M14 coverage remains in the same batch suite.

## 12) Deferred scope

Still explicitly deferred:

- N-slot,
- work stealing,
- SPMC / MPMC queues,
- lock-free queues,
- parallel judgment,
- benchmark/performance claims,
- advanced queue scheduling,
- public event stream expansion.

## Language/reference consistency note

This milestone modifies native runtime/tests/docs only and does not change Oct language semantics in `Language/reference`.
