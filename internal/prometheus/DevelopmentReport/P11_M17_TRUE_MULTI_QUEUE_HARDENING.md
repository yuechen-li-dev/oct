# P11 M17 — True Multi-Queue Submit Hardening

## 1) M16 handoff summary

M16 introduced topology-gated true multi-queue submit for batch SGEMM with deterministic worker→queue mapping, per-worker submit/fence resources, and explicit serialized fallback reason reporting.

Implemented in M16:

- compute queue discovery with reported vs independent queue counts,
- true-multi gate evaluation,
- true-multi execution mode selection when all gates pass,
- serialized bridge fallback when any gate fails,
- transfer queue separation from compute-lane counts,
- first-failure metadata retention and drain counters.

## 2) Audit findings

Classification of M16 path before M17 patching:

1. gate evaluation — **implemented and tested**,
2. queue topology classification — **implemented and tested**,
3. worker→queue mapping — **implemented and tested**,
4. per-worker submit path — **implemented but under-tested** (cross-queue failure pressure),
5. per-worker fence reset/wait path — **implemented but under-tested** (wait-path deterministic failure isolation missing),
6. first-failure-wins across queues — **implemented but under-tested**,
7. drain behavior across queues — **implemented but under-tested** (timeout branch absent),
8. unsafe-to-reuse behavior — **implemented but under-tested** (not explicit for timeout/global-failure classes),
9. transfer queue separation — **implemented and tested**,
10. diagnostics publication — **implemented but under-tested** (truthfulness under repeated mixed outcomes and fallback transitions).

## 3) failure-after-submit handling

M17 hardens injected failure timing for `PROM_BATCH_FLAG_FAIL_AFTER_FIRST_SUBMIT` so failure is captured after successful queue submit, preserving first-failure metadata while other worker activity can remain in-flight.

Behavior preserved:

- first-failure-wins metadata,
- no output commit on failure,
- failed terminal batch state,
- queue drain diagnostics populated.

## 4) per-worker fence failure handling

M17 keeps reset failure behavior and adds deterministic wait-failure injection for batch hardening tests.

Coverage now includes:

- worker-scoped fence reset failure,
- worker-scoped fence wait failure,
- no output commit on failure,
- first-failure metadata stability.

## 5) drain timeout / unsafe reuse behavior

M17 introduces deterministic drain-timeout test injection and explicit timeout failure detail.

On forced timeout:

- batch enters failed terminal state,
- `drain_timeout_count` increments,
- `unsafe_to_reuse` is set,
- output remains uncommitted,
- failure detail reports timeout.

## 6) device-lost/global failure status

M17 adds deterministic global-failure/device-lost-style injection for true-multi hardening and marks failed runs unsafe-to-reuse.

On injected device/global failure:

- explicit failure detail is surfaced,
- output remains uncommitted,
- `unsafe_to_reuse` is set,
- failure remains first-failure-stable.

## 7) repeated batch reset behavior

M17 adds same-runtime sequence coverage:

1. true-multi success,
2. true-multi failure,
3. true-multi success.

Validated:

- output commit flag resets per batch,
- failure metadata resets per batch,
- worker→queue mapping remains stable across equivalent runs,
- true-multi eligibility/selection and fallback diagnostics remain truthful.

## 8) queue mapping stability

M17 validates deterministic static mapping stability across repeated equivalent batches:

- `worker_queue_index[w]` remains stable across runs,
- no invalid queue id published,
- transfer queue remains excluded from compute-lane claims,
- pseudo-shared topology still falls back without hardware parallelism claim.

## 9) fallback gate behavior

M17 extends fallback-gate tests for:

- memory-cap gate,
- command-resource-invalid gate,
- forced-serialized gate,
- pseudo-shared/single-queue fallback behavior preservation.

Serialized bridge fallback remains intact and explicit.

## 10) diagnostics truthfulness

M17 hardens diagnostics assertions across true-multi and fallback runs, including:

- execution mode,
- true-multi selected/eligible behavior,
- hardware parallelism claim,
- serialized fallback reason,
- worker queue mapping fields,
- drain count and drain-timeout count,
- output committed,
- unsafe-to-reuse.

## 11) tests added

Added Marionette coverage for:

1. failure-after-submit while another worker may be in-flight,
2. worker-scoped fence reset + wait failure behavior,
3. drain-timeout unsafe-to-reuse semantics,
4. global/device-lost-style failure dominance + unsafe marker,
5. repeated success/failure/success true-multi runs,
6. queue mapping stability across equivalent runs,
7. fallback reason truthfulness (memory cap, command-resources-invalid, forced serialized).

## 12) unchanged/deferred scope

Still explicitly deferred:

- N-slot implementation,
- work stealing,
- SPMC/MPMC queues,
- lock-free queues,
- parallel judgment,
- scheduler redesign,
- performance claims/benchmark claims,
- public event-stream expansion.

## Required summary deliverable

1. **what M16 implemented**
   - topology-gated true multi-queue selection with serialized fallback and queue diagnostics.
2. **what M17 hardens**
   - post-submit failure pressure, fence reset/wait failures, drain timeout semantics, global/device failure semantics, repeated-run stability, and fallback/diagnostic truthfulness.
3. **which true-multi behaviors remain topology-gated**
   - true multi-queue submit remains selected only when full gate conjunction passes (independent queues, workers, resources/fences/mapping validity, memory support, no pseudo-shared, no forced serialized, ownership handoff requirements).
4. **how serialized fallback remains intact**
   - any failed gate still routes through serialized bridge mode with explicit fallback reason and no hardware parallelism claim.
5. **what remains deferred**
   - N-slot, stealing/SPMC/MPMC/lock-free scheduling, parallel judgment, and performance scope remain deferred.

## Language/reference consistency note

This milestone changes native Prometheus runtime/tests/docs only and does not alter Oct language semantics under `Language/reference`.
