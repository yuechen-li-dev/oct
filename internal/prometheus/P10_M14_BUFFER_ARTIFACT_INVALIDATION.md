# P10 M14 — Buffer Artifact Dependency Invalidation

## 1) M36 handoff summary

M36 established that coarse whole-shape invalidation (`(m,n,k)` style) caused avoidable churn and could still miss representation/capacity hazards. M14 implements per-artifact invalidation keys and compatibility checks for SGEMM buffers:

- `A` depends on `(m, k, compute_or_padded_k, layout, precision, required_bytes)`.
- `B` depends on `(k, n, compute_or_padded_k, layout, precision, required_bytes)`.
- `C` depends on `(m, n, layout, precision, required_bytes)`.

## 2) Current reuse/invalidation audit (pre-M14 -> M14)

Before M14, runtime buffer reuse for both direct and staged paths was guarded by a shared `(m,n,k)` shape check plus per-buffer capacity checks. This could over-invalidate on isolated `m` or `n` changes.

M14 changes reuse to per-artifact dependency compatibility + capacity sufficiency:

- direct path: independent A/B/C artifact checks;
- staged path: independent A/B/C artifact checks over paired staging/device/readback buffers.

## 3) Artifact key design

M14 introduces native artifact metadata (`prom_buffer_artifact_key`) and an invalidation reason enum.

Key fields:

- dependency dims (`m/n/k` + `compute_or_padded_k` as applicable),
- representation (`layout`, `precision`),
- `required_bytes`.

Compatibility for reuse is explicit and artifact-specific:

1. dependency surface compatible for that artifact,
2. representation compatible,
3. allocation capacity `>= required_bytes`.

## 4) A/B/C dependency rules mapping

Implemented behavior:

- `m`-only change: invalidates A/C, allows B reuse when key+capacity compatible.
- `n`-only change: invalidates B/C, allows A reuse when key+capacity compatible.
- `k`-only change: invalidates A/B; C reuses when representation/capacity remain compatible.
- layout/precision transition: invalidates impacted artifacts.

Staged artifacts share the same invalidation surface as their logical artifact:

- A artifact => `staged_upload_a` + `staged_device_a`
- B artifact => `staged_upload_b` + `staged_device_b`
- C artifact => `staged_device_c` + `staged_readback_c`

## 5) Capacity/layout/precision handling

- Capacity shortfall is a hard invalidation reason regardless of logical shape match.
- Layout/precision mismatch is tracked as explicit layout/precision invalidation.
- Same logical shape with representation change is blocked from unsafe reuse by key compatibility checks.

## 6) Dominatus / dirty integration

M14 keeps integration narrow:

- invalidation/reuse counters are exported through existing diagnostics surface;
- existing slot/runtime telemetry flow remains unchanged;
- no broad key-catalog migration was done in M14.

## 7) Diagnostics added

Added M14 diagnostics fields:

- per-artifact invalidation counts: A/B/C
- per-artifact reuse counts: A/B/C
- false invalidation avoided count
- capacity invalidation count
- layout/precision invalidation count
- last invalidation reason per artifact (A/B/C)

## 8) Tests added

Added native Marionette coverage for:

1. M-only artifact invalidation behavior
2. N-only artifact invalidation behavior
3. K-only artifact invalidation behavior
4. layout transition invalidation accounting
5. precision transition / same-shape safety behavior

These tests validate per-artifact invalidation/reuse counters and reason tracking.

## 9) Behavior compatibility

M14 is implementation-local to buffer invalidation/reuse contracts. Existing Packed4/FP16/M29/M31/Dominatus selector logic is preserved and exercised by existing suite plus new targeted tests.

## 10) Deferred scope

Explicitly deferred:

- allocator redesign/suballocation framework
- selector cache expansion beyond existing scope
- slot readiness dirty-mask protocol implementation
- N-slot/work-stealing
- FFT buffer invalidation
- broad public allocator API changes

## Required summary deliverable

1. **Where reuse/invalidation was keyed before:** shared `(m,n,k)` shape + capacity checks per path.
2. **M36 A/B/C mapping:** A/B/C contracts now map directly to direct and staged artifact groups listed above.
3. **Migrated artifacts in M14:** direct `A/B/C` and staged `upload/device/readback` artifacts that share A/B/C surfaces.
4. **Deferred:** selector expansion, slot readiness protocol, allocator redesign, wider subsystem migrations.
5. **Unsafe same-shape/different-layout reuse prevention:** artifact compatibility includes layout/precision dependency checks; capacity shortfall blocks reuse even on same logical shape.

## Inconsistency / documentation-gap note

M36 called out missing first-class Dominatus A/B/C dependency keys. M14 implements this natively in Vulkan runtime metadata and diagnostics, but does not yet extend Dominatus key catalog with explicit A/B/C artifact keys.
