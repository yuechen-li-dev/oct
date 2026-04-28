# P13 M3 / Prometheus SGEMM Algorithm Lab M46 — Occupancy Variant Benchmark Harness Design

## 1) M45/M2 handoff summary

### 1.1 What M45 proved

M45 validated that a deterministic feedforward control law is viable for first-pass occupancy control:

`device_band + shape_class -> kernel_variant`.

It established a compact band set, shape classes, and variant vocabulary, plus explicit safety clamping and manual override seam, while deferring per-GPU response-surface profiling and runtime autotune.

### 1.2 What M2 implemented

P13 M2 implemented the production selector seam in native code:

- occupancy band/shape/variant enums and reason codes,
- deterministic selector API and facts/decision structures,
- safety clamp + override acceptance/rejection,
- runtime diagnostics export for selected/unclamped variant, band, shape, clamp/fallback/override,
- no SGEMM dispatch path switch yet (diagnostics-only integration).

### 1.3 Why M46 harness design is required before variants

Without an explicit benchmark contract, any future kernel-variant claim would be ungrounded:

- correctness checks could drift,
- timing quality could be overclaimed,
- unavailable variants could be misreported,
- selector diagnostics could be wrong without a gate,
- runtime dispatch actuation would lack objective readiness criteria.

M46 therefore defines the courtroom contract before defendants (new kernels) exist.

### 1.4 What M46 designs

M46 delivers an executable Oct model for:

- benchmark stages,
- mode definitions,
- shape coverage,
- variant availability handling,
- correctness/timing confidence policy,
- artifact schema contract,
- acceptance gates and actuation-readiness logic.

### 1.5 What remains deferred

Deferred by design:

- Vulkan SGEMM variant implementation,
- native benchmark harness implementation,
- runtime dispatch switching,
- autotune/commissioning/response-surface fitting,
- any performance claims.

## 2) Existing infrastructure audit

### 2.1 Audited components

- M45 lab artifacts/tests and selector mapping model.
- M2 selector seam docs and native selector/diagnostics structures.
- Native runtime correctness oracle and tolerance comparator (`compareAgainstOracle`).
- Existing Prometheus benchmark harness report and benchmark command artifact conventions.
- Marionette benchmark support for repeated execution.
- Runtime diagnostics fields carrying P13 M2 occupancy metadata.
- Timestamp-query support search in native reactor sources.

### 2.2 Classification

#### Usable as-is

- CPU SGEMM reference generation + elementwise oracle compare (abs/rel + non-finite handling).
- M2 selector diagnostics fields and reason-code exports.
- Oct benchmark/artifact plumbing (`WriteOctagon`) and deterministic artifact conventions.
- Marionette benchmark harness ability to run repeated iterations and filtered benches.

#### Usable with small extension

- Existing benchmark artifact structure can be extended with occupancy benchmark schema fields (device band/shape/variant/fallback/confidence).
- Existing correctness diagnostics can be widened with explicit failing index + max error surface in occupancy variant reports.

#### Missing and required

- Native Vulkan timestamp-query timing path for SGEMM variant benchmarking (reset/write/collect/query-availability path).
- Native occupancy variant benchmark runner that sweeps shape classes and variant candidates.
- Native availability-aware variant execution plumbing (implemented/stubbed/skip/fallback modes).
- Acceptance gate evaluator that determines actuation readiness from correctness + timing + diagnostics quality.

#### Deferred

- Runtime autotune.
- Per-GPU response-surface commissioning.
- Automatic policy fitting.
- Production dispatch switch to occupancy variants.

## 3) Benchmark stages (required contract)

M46 defines the future harness pipeline as:

1. runtime creation/probe,
2. device capability capture,
3. occupancy band/shape/variant selection,
4. deterministic input generation,
5. CPU reference output generation,
6. warmup iterations,
7. timed measured iterations,
8. timestamp-query collection (or explicit low-confidence fallback),
9. correctness comparison,
10. diagnostics capture,
11. artifact emission,
12. pass/fail + actuation-readiness classification.

## 4) Shape set

M46 includes all required M45 classes with concrete `(m,n,k)` cases:

- small-square: `(128,128,128)`
- medium-square: `(512,512,512)`
- large-square: `(2048,2048,2048)`
- tall-skinny: `(2048,256,512)`
- wide-short: `(256,2048,512)`
- k-heavy: `(512,512,4096)`
- ML/FFN-like: `(4096,11008,4096)`

Smoke mode filters to a compact CI-safe subset via `SmokeEligible` flags.

## 5) Variant handling

M46 models the M2 variant vocabulary:

- baseline-scalar,
- memory-conservative,
- small-register-tile,
- balanced-2x2-accum4,
- aggressive-4x4-accum8.

Availability behavior:

- **implemented** variants can be benchmarked normally,
- **unavailable** variants are either:
  - `skipped` (`tested_variant = unavailable`) in strict modes, or
  - `fallback-to-baseline` with explicit reason in permissive modes.

Contract guarantee: unavailable variants are never marked successful.

Manual override requests are captured and carried in case metadata.

## 6) Correctness policy

M46 contract requires:

- CPU reference SGEMM oracle per case,
- element-wise comparison,
- absolute + relative tolerance gates,
- NaN/Inf explicit rejection,
- max absolute + max relative error capture,
- first failing index capture,
- correctness gate dominance over timing (timing cannot override correctness failure).

FP16 forward note remains explicit: tolerance profile must respect FP16-storage/FP32-accum behavior and should reuse existing FP16 diagnostics policy instead of blindly using strict FP32 tolerance.

## 7) Timing policy

M46 defines timing structure:

- warmup and measured iteration counts by mode,
- preferred source: Vulkan timestamp query,
- fallback source: CPU wall clock only when explicitly labeled,
- confidence grading (`high` with timestamp query, `low` with wall-clock fallback),
- statistics fields: mean/median/min and stability proxy,
- derived metrics: GFLOP/s, memory byte estimate, arithmetic intensity estimate.

No overclaim rule: low-confidence timing cannot pass actuation gate.

## 8) Artifact schema

M46 uses schema id:

`prometheus.sgemm.occupancy_benchmark.v1`

Top-level sections:

- `device` (identity + occupancy band + capability classes),
- `run` (mode + iteration counts + timing source/confidence),
- `cases[]` (shape/variant/correctness/timing/diagnostics/fallback metadata),
- `final_recommendation` (gate outcomes + reason).

Schema field coverage is exported as `m46_artifact_schema.octagon`.

## 9) Benchmark modes

M46 contract modes:

1. **smoke**: tiny CI-safe pass; few iterations; fallback allowed.
2. **characterization**: broader sweep; timestamp query required in intended native implementation; unavailable variants skipped (strict mode).
3. **comparison**: selected-vs-baseline decision mode; fallback allowed but not actuatable.

## 10) Pass/fail gates

Actuation readiness requires all:

- correctness gate pass,
- timing confidence gate pass,
- timing improvement threshold pass,
- stability gate pass,
- diagnostics alignment pass,
- tested variant must be real candidate (not baseline fallback/unavailable).

This is encoded in model logic and exported via acceptance/final recommendation artifacts.

## 11) Model/tests/artifacts

### 11.1 Executable model files

- `prometheus_sgemm_algorithm_lab_m46.oct`
- `prometheus_sgemm_algorithm_lab_m46.octest`

### 11.2 Behavioral tests included

M46 tests cover required contract points:

1. smoke mode CI-safe compactness,
2. characterization shape-class coverage,
3. unavailable-variant non-success,
4. correctness-failure gate precedence,
5. low timing confidence blocks actuation,
6. baseline-threshold gate,
7. diagnostics mismatch gate,
8. artifact schema required fields,
9. manual override representation,
10. computed recommendation,
11. determinism.

### 11.3 Generated artifacts

- `m46_benchmark_modes.octagon`
- `m46_shape_cases.octagon`
- `m46_artifact_schema.octagon`
- `m46_acceptance_gates.octagon`
- `m46_variant_availability_table.octagon`
- `m46_final_recommendation.octagon`

All artifacts are emitted from model functions (not static files).

## 12) P13 M4 direction

P13 M4 should implement the **small native occupancy benchmark harness** to this contract:

- integrate deterministic case generation and CPU oracle path,
- add Vulkan timestamp-query timing path (and explicit fallback/confidence tagging),
- execute selected variant candidates with availability handling,
- emit `prometheus.sgemm.occupancy_benchmark.v1` artifacts,
- apply actuation gates and report readiness,
- keep runtime dispatch unchanged until a non-baseline candidate passes gates.

## 13) Deferred scope

Still deferred after M46:

- kernel variant implementation,
- native shader specialization/generator work,
- runtime autotuning,
- response-surface commissioning,
- production dispatch actuation.

## Required final answers

1. **What benchmark harness should Prometheus build before kernel variants?**
   A small native harness implementing the 12-stage contract above with strict correctness/timing-confidence/diagnostics gates and versioned artifacts.

2. **What shapes must be included?**
   The seven M45 classes with concrete representatives listed in Section 4.

3. **How should correctness be checked?**
   CPU oracle + element-wise compare with abs/rel tolerances, NaN/Inf rejection, max error + first failing index capture.

4. **How should timing be measured and confidence-scored?**
   Prefer Vulkan timestamp query; if unavailable, mark wall-clock fallback as low confidence and block actuation.

5. **How should unavailable variants be handled?**
   Skip or fallback-to-baseline with explicit reason by mode; never mark unavailable variants as successful.

6. **What artifact schema should be emitted?**
   `prometheus.sgemm.occupancy_benchmark.v1` with `device`, `run`, `cases[]`, and final recommendation fields.

7. **What gates make a variant eligible for runtime dispatch?**
   Correctness pass AND sufficient timing confidence AND threshold improvement vs baseline AND stability pass AND diagnostics match.

8. **What should P13 M4 implement next?**
   Native harness implementation for this contract, especially timestamp-query instrumentation + artifact emission + gate evaluator.

9. **What remains deferred?**
   Actual optimized kernels, dispatch switch, autotune, and commissioning workflows.

## Language/reference consistency note

This milestone is experiment/model/report work under `Experiments/` and does not alter `Language/` semantics. No new inconsistency with `Language/reference` was introduced in this patch.
