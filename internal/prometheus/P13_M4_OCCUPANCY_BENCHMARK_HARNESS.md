# P13 M4 — Occupancy Benchmark Harness

## 1) M46 handoff summary

M46 required a native occupancy benchmark harness with this contract:

- deterministic shape cases and deterministic input generation,
- CPU oracle correctness comparison,
- warmup + measured iteration flow,
- preferred Vulkan timestamp-query timing with explicit low-confidence fallback,
- selector diagnostics capture and alignment checks,
- versioned artifact output using `prometheus.sgemm.occupancy_benchmark.v1`,
- acceptance gates that block actuation unless correctness, timing confidence, stability, improvement threshold, diagnostics, and implemented variant checks all pass,
- and no SGEMM dispatch switch in runtime until a variant passes the gates.

## 2) Harness surface implemented

Implemented a native harness in Marionette tests:

- `internal/prometheus/native/Marionette/reactor_p13_m4_occupancy_benchmark_tests.cpp`

This harness executes through the existing runtime SGEMM entrypoint (`prometheus_reactor_runtime_sgemm`) and policy diagnostics entrypoint (`prometheus_reactor_runtime_sgemm_policy_diagnostics`) without modifying runtime dispatch behavior.

## 3) Benchmark modes

Implemented three modes:

- **smoke**: compact CI-safe shape subset, minimal iterations.
- **characterization**: full M46 shape-class coverage; unavailable variants are skipped.
- **comparison**: selected variant request against baseline behavior with fallback handling and acceptance gate evaluation.

## 4) Shape cases

Implemented M46 shape classes and canonical shape tuples:

- small-square `(128,128,128)`
- medium-square `(512,512,512)`
- large-square `(2048,2048,2048)`
- tall-skinny `(2048,256,512)`
- wide-short `(256,2048,512)`
- K-heavy `(512,512,4096)`
- ML/FFN-like `(4096,11008,4096)`

Smoke mode restricts to compact shape(s) only.

## 5) Deterministic input and oracle

Harness generates deterministic A/B matrices from a bounded, salt-mixed index function (shape-specific but repeatable) and computes a CPU SGEMM oracle for each case.

Correctness capture includes:

- pass/fail,
- max absolute error,
- max relative error,
- first failing index,
- aggregate absolute error,
- explicit tolerances.

Correctness failure always blocks actuation.

## 6) Timing source and confidence

Current M4 harness timing source is:

- `timing_source = cpu_wall_clock`
- `timing_confidence = low`

Rationale: runtime API currently exposes no explicit Vulkan timestamp-query capture surface for SGEMM case timing in Marionette harness calls, so M4 uses explicit low-confidence fallback and blocks actuation readiness accordingly.

## 7) Variant availability handling

Harness understands M2 variant vocabulary:

- baseline-scalar,
- memory-conservative,
- small-register-tile,
- balanced-2x2-accum4,
- aggressive-4x4-accum8.

M4 marks only baseline as currently implemented/executable for candidate gating.

Unavailable variants are handled explicitly:

- **characterization**: skipped with reason,
- **comparison/smoke**: fallback to baseline with explicit reason.

Unavailable variants are never marked successful candidates.

## 8) Artifact schema

Harness emits JSON artifact content with schema id:

- `prometheus.sgemm.occupancy_benchmark.v1`

Artifact includes:

- device section,
- run section,
- per-case shape/variant/correctness/timing/diagnostics fields,
- final recommendation section.

Artifacts are emitted via Marionette artifact output (`WriteTextArtifact`) for test inspection.

## 9) Acceptance gates

Implemented gate evaluator enforces M46-style logic:

actuation-ready requires all of:

- correctness pass,
- high timing confidence,
- stability threshold pass,
- improvement threshold pass,
- diagnostics alignment,
- real implemented non-baseline candidate.

Given M4 state (no optimized implemented variants and low timing confidence fallback), final recommendation remains non-actuatable by design.

## 10) Tests added

Added focused Marionette tests covering:

1. smoke mode compact-case behavior,
2. artifact schema fields presence,
3. correctness failure blocks actuation,
4. low timing confidence blocks actuation,
5. unavailable variant not marked successful,
6. diagnostics alignment required,
7. baseline correctness succeeds,
8. runtime dispatch behavior unchanged,
9. determinism of smoke metadata/recommendation.

## 11) Behavior intentionally unchanged

No runtime SGEMM dispatch changes were introduced.
No new occupancy kernels were implemented.
Harness is measurement/evidence infrastructure only.

## 12) Deferred scope

Deferred explicitly (unchanged from M46 intent):

- optimized occupancy kernel variant implementation,
- runtime dispatch actuation to occupancy variants,
- runtime autotune / response-surface fitting,
- per-device commissioning flow,
- performance claims/publication,
- benchmark-driven dispatch switching.

## Consistency note

M46 prefers Vulkan timestamp queries when available; M4 currently records explicit CPU wall-clock low-confidence timing due to missing runtime timestamp-query API exposure in this harness path. This is surfaced intentionally as a deferred integration gap instead of overclaiming confidence.
