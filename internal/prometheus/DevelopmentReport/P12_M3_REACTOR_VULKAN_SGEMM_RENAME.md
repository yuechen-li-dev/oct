# P12 M3 — Reactor Vulkan SGEMM Rename Report

## 1) M2 handoff summary

P12 M2 extracted low-risk shared Vulkan helpers into `internal/prometheus/native/reactor_vulkan_common.c` and left the remaining Vulkan reactor implementation as a single SGEMM-focused compilation unit in `internal/prometheus/native/reactor_vulkan.c`.

M3 completes the identity cleanup by renaming that SGEMM unit to match the topology plan while preserving runtime behavior, symbol names, and ABI forwarding.

## 2) File rename performed

Performed source identity rename:

- `internal/prometheus/native/reactor_vulkan.c`
- → `internal/prometheus/native/reactor_vulkan_sgemm.c`

No functional rewrites were applied as part of the rename.

## 3) Build/source-list updates

Updated active native build source lists to compile SGEMM under the new filename:

- `internal/prometheus/native/build_stub.sh`
  - now compiles `reactor_vulkan_common.c` + `reactor_vulkan_sgemm.c`
  - no longer references `reactor_vulkan.c`
- `internal/prometheus/native/build_windows.cmd`
  - now compiles `reactor_vulkan_common.c` + `reactor_vulkan_sgemm.c`
  - no longer references `reactor_vulkan.c`

Marionette/native test binary source lists in these build helpers were updated in lockstep.

## 4) SGEMM section banners added

Added in-file navigational banners to `internal/prometheus/native/reactor_vulkan_sgemm.c` with behavior-preserving comment-only edits.

Sections added/aligned:

- SGEMM Includes / Platform Glue
- SGEMM Runtime State
- Vulkan Common Integration
- SGEMM Dominatus Integration
- SGEMM Policy / Judgment Fact Building
- SGEMM Typed Arena / Buffer Artifact Ownership
- SGEMM Layout Precision: Packed4 / FP16
- SGEMM Transfer Queue Integration
- SGEMM Single-Call Execution
- SGEMM Async Lifecycle
- SGEMM Batch Dispatch / Worker Runtime
- SGEMM Multi-Queue Submit
- SGEMM Diagnostics Export
- Public SGEMM ABI Entrypoints

Ordering reflects existing compile/locality flow; no risky function reshuffle was introduced.

## 5) References updated / historical references left

Active compile paths now reference `reactor_vulkan_sgemm.c`.

Repository search still finds `reactor_vulkan.c` references in historical reports/design notes (P8/P9/P10/P12 M1/M2 era docs and experiment reports). These were intentionally left unchanged because they describe prior milestones in past tense.

No active build script or active compile path points to `reactor_vulkan.c` after M3.

## 6) Public impl symbol preservation and behavior validation

### Public impl symbols preserved

`prom_reactor_runtime_*_impl` symbol names and ABI entrypoint behavior were preserved; `reactor_api.c` forwarding contract remains unchanged.

### Validation run

- `bash internal/prometheus/native/build_stub.sh` succeeded.
- Required targeted Marionette runs succeeded:
  - `P11_M20`, `P11_M19`, `P11_M17`, `P11_M16`, `P11_M14`, `P11_M11`, `P11_M10`, `P11_M8`, `P11_M7`, `P11_M6`, `PrometheusReactor_Sgemm`.
- Full `out/prometheus/native/marionette_tests` suite succeeded (with environment-dependent skips and zero failures).
- Search evidence:
  - `rg "reactor_vulkan\.c"` shows historical/documentary references only.
  - `rg "reactor_vulkan_sgemm\.c"` shows active build/source-list references.

Net result: M3 completed as behavior-preserving file identity + readability cleanup.

## 7) Deferred scope

Explicitly deferred (unchanged from plan):

- adding `reactor_vulkan_fft.c` stub,
- adding `reactor_vulkan_fused_reduction.c` stub,
- further helper extraction beyond current low-risk common plumbing,
- SGEMM internal splitting into additional files,
- public API/ABI changes,
- SGEMM behavior changes.
