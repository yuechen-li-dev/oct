# P13 M16b-2 — SRT-2accum-K benchmark seam wiring

## 1) M16b-1 handoff
M16b-1 introduced a benchmark-only variant request seam and lifecycle diagnostics while keeping production SGEMM dispatch unchanged.

## 2) SRT shader mapping
This milestone wires benchmark request `small-register-tile` to the real embedded SPIR-V shader asset `k_prom_sgemm_srt_2accum_k_spirv` via a dedicated Vulkan compute pipeline.

## 3) Diagnostics and path identity
For `small-register-tile` benchmark requests, diagnostics now report:
- `requested_occupancy_variant = small-register-tile`
- `executed_occupancy_variant = small-register-tile`
- `variant_path_status = wired`
- `variant_path_id = srt_2accum_k`
- `fallback_reason = none`

A dedicated non-baseline path id is used for SRT.

## 4) Correctness tests
Added Marionette benchmark-lane tests that exercise SRT execution and verify output against CPU oracle across CI-safe shapes, including odd-K coverage:
- `1x1x1`
- `3x7x5`
- `8x8x9`
- `16x16x17`
- small wide-short
- small tall-skinny

## 5) Production dispatch unchanged
`prometheus_reactor_runtime_sgemm(...)` remains baseline-driven and does not activate SRT pathing.

## 6) Variants still deferred
`balanced-2x2-accum4` and `aggressive-4x4-accum8` remain benchmark-lane not-wired for this milestone.

## 7) Validation results
Validated via native stub build and Marionette benchmark/test execution commands for benchmark and normal SGEMM lanes.
