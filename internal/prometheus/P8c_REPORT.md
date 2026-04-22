# P8c Report — Vulkan Device-Local Memory + Staging Buffer Protocol Port

## Scope completed

P8c ports the M8 staged-memory protocol into `reactor_vulkan.c` with an explicit direct-vs-staged path split, explicit fallback behavior, and explicit ordering/visibility edges for upload, compute, and readback.

## M8 invariants extracted and Vulkan mapping

### Invariants extracted from M8

1. Path selection must be capability- and workload-aware, not blunt always-staged.
2. Staged upload ordering must be explicit before compute.
3. Readback is a distinct protocol from upload-only and must not be forced.
4. Missing staged capability must fallback explicitly when allowed or fail explicitly when not.
5. Tiny/small workloads should default to direct host-visible unless staging is justified.
6. Unsafe causes should be observable with explicit diagnostics.

### Vulkan runtime mapping

- **Path split + capability/shape gating**
  - Added explicit path modes: direct, staged-upload, staged-upload+readback.
  - Default policy: small shapes stay direct; large shapes use staged path when device-local capability exists.
  - Test flags allow forcing direct/staged mode to validate protocol edges.
- **Explicit staged ordering**
  - Host-write → transfer-read barriers on upload staging buffers.
  - Transfer-write → shader-read barriers on device-local A/B; transfer-write → shader-write on device-local C before dispatch.
  - Compute-write → transfer-read on device-local C before readback copy.
  - Transfer-write → host-read barrier on readback staging buffer before CPU consumption.
- **Upload-only vs upload+readback separation**
  - `PROM_TESTCFG_FORCE_UPLOAD_ONLY` selects staged upload-only flow with success at submit stage and no transfer-out/readback.
  - Standard SGEMM call path uses staged upload+readback when staged mode is selected and host output is required.
- **Explicit fallback/mismatch**
  - Staged request with unavailable device-local capability falls back to direct only if fallback is enabled and direct host-visible capability exists.
  - Otherwise fails with `PROM_DETAIL_CAPABILITY_MISMATCH`.
- **Observability**
  - Success detail codes now expose selected path: direct, staged upload, staged upload+readback, and fallback-to-direct.
  - Capability mismatch has a dedicated detail code.

## Paths implemented

1. **Direct path**
   - Host-visible storage buffers for A/B/C.
   - Host→compute and compute→host visibility barriers.
2. **Staged upload path**
   - Host-visible staging A/B, device-local A/B/C.
   - Host write + transfer copy + transfer→compute visibility edges.
   - No readback copy in upload-only mode.
3. **Staged upload + readback path**
   - Staged upload path plus compute→transfer barrier, device-local→readback copy, transfer→host visibility, and host copy-out.

## Tests proving protocol behavior

Marionette coverage now includes:

- Tiny-shape path-selection sanity (`PrometheusReactor_PathSelectionAvoidsBlindStagingForTinyShapes`).
- Forced staged upload+readback correctness and explicit path observability (`PrometheusReactor_ForcedStagedUploadReadbackPathIsObservable`).
- Staged upload-only path without forced readback (`PrometheusReactor_StagedUploadOnlyPathSkipsReadbackWhenNotRequired`).
- Device-local capability fallback and fallback-disallowed mismatch failure (`PrometheusReactor_StagedPathCapabilityFallbackAndMismatchAreExplicit`).
- Existing SGEMM correctness/reuse/error tests updated to accept explicit success-path diagnostics.

## Deferred to later P8 milestones

- Barrier minimization and aggressive synchronization tuning (current barriers are intentionally conservative).
- Async submission and non-blocking readback architecture.
- Tiled/blocked SGEMM strategy integration.
- Broader allocator/heap-placement policy beyond this SGEMM staging protocol.

## Inconsistency surfaced

M8 is a protocol-level model (`readiness`, `visibility`, `fallback`) and is intentionally not a Vulkan API simulator. P8c maps those protocol invariants into concrete Vulkan buffer classes, transfer commands, and pipeline barriers; this is a deliberate layer translation, not a contradiction.
