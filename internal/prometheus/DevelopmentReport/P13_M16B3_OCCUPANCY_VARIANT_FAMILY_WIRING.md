# P13 M16b-3 — Occupancy Variant Family Wiring + Promotion Seam

## 1) M16b-2 handoff
M16b-2 wired `small-register-tile -> SRT-2accum-K` and proved benchmark seam path identity while production dispatch remained unchanged.

## 2) B2x2 shader mapping
Benchmark request `balanced-2x2-accum4` now dispatches the embedded SPIR-V kernel `k_prom_sgemm_b2x2_row_major_biased_spirv` with dedicated pipeline selection.

## 3) A2x4 shader mapping
Benchmark request `aggressive-4x4-accum8` now dispatches the embedded SPIR-V kernel `k_prom_sgemm_a2x4_row_biased_accum8_spirv` with dedicated pipeline selection.

## 4) MC-baseline-strict alias
Benchmark request `memory-conservative` is explicitly wired as `MC-baseline-strict` baseline alias:
- executed variant identity remains `memory-conservative`
- path id is baseline
- fallback reason is `mc_baseline_strict_alias`

## 5) Diagnostics / path identity
Added full identity for all recipe variants in benchmark seam:
- baseline-scalar -> baseline
- memory-conservative -> wired_alias baseline
- small-register-tile -> srt_2accum_k
- balanced-2x2-accum4 -> b2x2_row_major_biased
- aggressive-4x4-accum8 -> a2x4_row_biased_accum8

## 6) DVT/PVT/production promotion seam
The runtime now explicitly exports lifecycle fields for each benchmark request:
- `variant_benchmark_enabled`
- `variant_dvt_validated`
- `variant_pvt_validated`
- `variant_production_eligible`
- `variant_dispatch_enabled`

Historical M16b-3 truth model at the time of wiring:
- baseline: benchmark enabled + dvt/pvt validated + production eligible + dispatch enabled.
- non-baseline variants: benchmark enabled, dvt/pvt false, production eligible false, dispatch enabled false.

Px16 M3 supersedes that lifecycle gate:
- all wired variants are now production eligible and dispatch enabled under EVT semantics,
- DVT/PVT lifecycle fields remain observational telemetry only,
- `occupancy_apply_safety_clamp` remains the real safety gate,
- judgment-engine selection remains the sole production dispatch authority.

## 7) Correctness coverage
Marionette benchmark-lane tests now validate B2x2 and A2x4 wired path identity and CPU-oracle correctness behavior, and verify MC alias diagnostics.

## 8) Production dispatch unchanged
`prometheus_reactor_runtime_sgemm(...)` remains baseline/default production dispatch. Non-baseline actuation remains benchmark-lane only.

## 9) Deferred scope
Still deferred by design:
- production dispatch actuation for non-baseline variants
- DVT local 3070 run
- PVT cloud/multi-GPU validation
- performance tuning and claims
- autotune/response-surface work

## Inconsistency surfaced
`PROM_OCCUPANCY_VARIANT_PATH_STATUS_ALIAS_OR_NOT_WIRED` naming now semantically acts as a wired alias state for `memory-conservative`; the enum label predates this split and could be renamed later for clarity.
