# P13 M16b-1 — Occupancy Variant Dispatch Seam (Benchmark-Only)

## 1) Pre-change build verification
- Command: `bash internal/prometheus/native/build_stub.sh`
- Result: passed before seam edits.

## 2) Seam added
- Added benchmark-only SGEMM entrypoint:
  - `prometheus_reactor_runtime_sgemm_benchmark_variant(...)`
  - internal forwarder: `prom_reactor_runtime_sgemm_benchmark_variant_impl(...)`
- Normal `prometheus_reactor_runtime_sgemm(...)` remains baseline-only and unchanged in behavior.

## 3) Variant lifecycle fields
Added policy diagnostics fields for M16b-1 lifecycle/dispatch truthfulness:
- requested_occupancy_variant
- executed_occupancy_variant
- variant_registered
- variant_benchmark_enabled
- variant_production_eligible
- variant_dispatch_enabled
- variant_path_status
- variant_path_id
- fallback_reason

M16b-1 state model:
- baseline-scalar: registered/benchmark-enabled/production-eligible/dispatch-enabled
- memory-conservative: registered + benchmark-enabled, production/dispatch disabled, alias-or-not-wired
- small-register-tile / balanced-2x2-accum4 / aggressive-4x4-accum8: registered + benchmark-enabled, production/dispatch disabled, not-wired

## 4) Benchmark override mechanism
- Benchmark harness now requests variants via `prometheus_reactor_runtime_sgemm_benchmark_variant(...)`.
- This provides a strict benchmark lane override seam while preventing accidental production path requests from normal API usage.

## 5) Diagnostics added
- Runtime now records benchmark seam diagnostics on each benchmark-variant call.
- Non-baseline requests are explicitly reported with `fallback_reason = variant_path_not_wired` semantics and baseline executed variant.

## 6) Tests added/updated
- Updated occupancy benchmark harness to route calls through benchmark-variant API and consume seam diagnostics.
- Existing benchmark and SGEMM Marionette suites pass with seam active.

## 7) Production dispatch unchanged
- Normal `prometheus_reactor_runtime_sgemm(...)` still routes baseline execution and does not accept benchmark override.
- No new SPIR-V variant execution path is actuated in M16b-1.

## 8) Remaining for M16b-2 and M16b-3
- M16b-2: wire first real variant path (SRT-2accum-K) to real kernel dispatch + diagnostics transition from not-wired to wired.
- M16b-3: wire B2x2/A2x4 and finalize MC alias behavior with truthful path IDs and fallback semantics.

## Inconsistency surfaced
- `internal/prometheus/P13_M16_OCCUPANCY_VARIANT_IMPLEMENTATION.md` claims broader M16 behavior (including non-baseline benchmark identity handling) than this narrowed M16b-1 seam-only scope. M16b-1 intentionally keeps all non-baseline runtime execution on baseline and marks paths not wired.
