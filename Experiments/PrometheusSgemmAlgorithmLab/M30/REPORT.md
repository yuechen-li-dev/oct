# Prometheus SGEMM Algorithm Lab — M30 (Dedicated Transfer Queue Rake Lab)

## 1) Protocol model (required first step)

### What “dedicated transfer queue” means in this model
A dedicated transfer queue means a transfer-capable queue family that can execute staging copy work independently enough from compute to produce meaningful overlap in a fixed-double slot pipeline.

### What work moves to transfer queue
- Host->device uploads for A/B staging.
- Optional device->host readback for C when running upload+readback policy.

### What remains on compute queue
- SGEMM dispatch and compute-local synchronization.
- Baseline single-queue path copies when transfer queue policy is not selected.

### Synchronization edges in scope
1. Host write -> transfer read.
2. Transfer write -> compute read.
3. Compute write -> readback transfer read (only when readback enabled).
4. Readback transfer write -> host read.
5. Queue-family ownership release/acquire when queue families differ.
6. Async ready signal after all relevant queue work completes.

### Fixed-double slot interaction
Each slot tracks: slot id, lifecycle state, transfer state, compute state, queue ownership, generation, shape/layout metadata, ready-for-compute, transfer in-flight, compute in-flight, and failure state. Modeled lifecycle states are: `empty`, `preparing transfer`, `transfer in-flight`, `transfer complete`, `ready for compute`, `compute in-flight`, `consumed`, `failed`.

### Intentionally not modeled
No Vulkan API object details, barrier bit fields, command-buffer lifetime internals, allocator internals, hardware timings, triple buffering, or async architecture redesign.

## 2) Queue configurations modeled

1. **No dedicated transfer queue**: compute queue handles transfer+compute (fallback baseline).
2. **Dedicated transfer queue available**: transfer queue handles staging, compute queue handles SGEMM, explicit sync+ownership handoff required.
3. **Shared queue family / pseudo-transfer queue**: transfer capability exists but overlap is not assumed; gating can force single-queue behavior.

## 3) Fixed-double slot interaction findings

- Slot N compute can overlap with slot N+1 upload only when transfer queue independence and sync contract both hold.
- Slot reuse is rejected while transfer or compute is in-flight.
- Transfer failure in next slot is isolated and does not poison current compute slot.
- WIP depth remains bounded at 2 in all modeled policies.

## 4) Synchronization edges and hazards

M30 raked ten required hazards, including missing transfer->compute sync, missing compute->readback sync, ownership mistakes, overwrite races, fallback misuse, pseudo-transfer overclaims, readback confusion, and failure isolation.

Result: all ten hazards are modeled as detectable and blockable before corruption when required edges and ownership rules are enforced.

## 5) Policy comparison

Modeled policies:
- A: single-queue baseline
- B: transfer queue upload-only
- C: transfer queue upload+readback
- D: transfer queue fixed-double overlap
- E: transfer queue fallback gating
- F: optional small-shape disable

Key conclusions:
- D has the strongest modeled structural benefit (overlap and completion reduction) when independence is real.
- B is lower-risk than C and should be implemented first.
- E is mandatory in production to prevent overclaim and incorrect enablement.
- F is reasonable as an optional guard for low-benefit/small-shape regimes.

## 6) Failure-mode findings

- Missing sync edges are protocol hazards and must hard-fail the path.
- Queue-family ownership transitions must be explicit when families differ.
- Overwrite while in-flight (transfer or compute) must be rejected.
- Async readiness must join transfer+compute(+readback) completion, not just compute completion.
- Pseudo-transfer queues must not be treated as dedicated overlap wins.

## 7) Diagnostics/observability contract

Native implementation must expose:
- transfer queue used/not used,
- selected queue policy,
- dedicated queue availability,
- fallback reason,
- ownership handoff count,
- transfer/compute sync wait count,
- transfer failure slot,
- compute failure slot,
- async readiness state,
- slot id + generation,
- upload-only vs upload+readback mode.

## 8) Final recommendation

Dedicated transfer queue orchestration is worth implementing **conditionally**:
- enable only when a truly independent dedicated transfer queue exists,
- expected overlap exceeds synchronization overhead,
- fixed-double slot ownership/sync invariants are valid,
- and fallback remains explicit and observable.

## 9) M31 direction

Implement native **upload-only** dedicated transfer queue path first, with:
- explicit semaphore/ownership handoff instrumentation,
- strict single-queue fallback gates,
- diagnostics required above,
- conformance tests for all mandatory sync edges and ownership rules,
then evaluate upload+readback separately as a higher-risk extension.

---

## Required final answers

1. **Is a dedicated transfer queue worth implementing?** Yes, but only when transfer and compute are truly independent and gating confirms net benefit.
2. **Under what conditions should it be used?** Dedicated queue exists, overlap is expected, sync/ownership cost is acceptable, and fixed-double slot ownership remains valid.
3. **Should upload-only and upload+readback be separate policies?** Yes. Upload-only is lower risk and should land first.
4. **What synchronization edges are mandatory?** Host->transfer, transfer->compute, compute->readback (if enabled), readback->host, plus async-ready after all relevant queue work.
5. **What queue-family ownership rules are mandatory?** Explicit release/acquire handoff whenever transfer and compute queue families differ; no queue may consume before acquiring ownership.
6. **What diagnostics are mandatory?** Policy selection, queue usage/availability, fallback reason, ownership handoff/wait counts, slot+generation, readiness state, failure slot and mode.
7. **What should M31 implement or rake next?** Implement native upload-only dedicated transfer queue path with full sync/ownership diagnostics and fallback conformance tests, then re-rake upload+readback.

## Language/reference consistency note

No direct inconsistency was observed between this M30 Oct protocol model and `Language/reference` syntax/style expectations during this milestone.
