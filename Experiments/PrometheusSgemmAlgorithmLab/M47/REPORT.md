# P13 M6 / Prometheus SGEMM Algorithm Lab M47 — Occupancy Variant Recipe Simulation with Safety Margins

## 1) Updated hypothesis

Prometheus should not choose first SGEMM occupancy recipes by exact per-GPU Pareto maxima. The updated hypothesis is:

`device_band + shape_class + safety_margins -> robust first-implementation recipe`

### Why exact Pareto optimization is the wrong first target

- Exact Pareto fitting optimizes to one measured device/runtime condition, which is brittle for first productization.
- P13 currently needs resilient band-level behavior, not benchmark couture tied to one commissioning setup.

### Why same-model variance makes per-GPU tuning risky

- Same SKU cards can differ in usable register/shared-memory headroom, thermal/boost behavior, and driver/runtime overhead.
- A strategy that needs exact per-device tuning has high overfitting risk and poor portability for first rollout.

### Why band-level recipes with factor-of-safety are preferred

- Band-level recipes can be guarded with explicit clamps and safety margins that tolerate degraded conditions.
- This approach preserves deterministic selector control and avoids occupancy cliff failures under adverse margins.

### Why real-hardware benchmarking is deferred

The user constraint is explicit and non-negotiable: no real-hardware benchmark validation before the SGEMM reactor is mostly complete.

### What M47 must prove

M47 must compute (not hardcode):
1. the safest/useful first recipe,
2. required safety margins and clamp rules,
3. which variants are too aggressive now,
4. how variance changes selection pressure,
5. why exact per-GPU tuning loses now,
6. concrete P13 M7 implementation direction.

## 2) Why Pareto/exact tuning is deferred

M47 includes an explicit candidate strategy `exact-per-gpu-tuning-first` and penalizes it via:

- high overfitting risk,
- exact-tuning dependency penalty,
- immediate benchmark dependency penalty under the no-hardware-benchmark-now constraint.

This forces robust score selection toward margin-safe, implementation-ready recipes.

## 3) Variant recipe model

Modeled recipes:

- `baseline-scalar`
- `memory-conservative`
- `small-register-tile`
- `balanced-2x2-accum4`
- `aggressive-4x4-accum8`

Each recipe has structural fields for register tile class, accumulator count, estimated registers/thread, shared memory/workgroup, intensity class, ILP adequacy, occupancy pressure, implementation complexity, safety factor, minimum supported band, allowed shapes, and clamp conditions.

These are structural estimates only, not measured performance claims.

## 4) Device variance / safety margin model

The executable model evaluates each variant across deterministic degraded scenarios:

- balanced nominal,
- balanced low margin,
- compute-rich with reduced usable registers,
- memory-rich with reduced shared memory,
- register-constrained low-end.

Each scenario models uncertainty in:

- usable register budget,
- usable shared-memory budget,
- effective bandwidth,
- effective compute,
- occupancy headroom,
- driver/runtime overhead.

Safety checks apply guard-band subtraction (`safetyMarginPermille`) before demand checks, then compute cliff risk and variance penalties.

## 5) Shape class effects

Modeled shape classes:

- small-square
- medium-square
- large-square
- tall-skinny
- wide-short
- K-heavy
- ML/FFN-like

Shape influence drives intensity benefit, register tolerance, shared pressure, occupancy sensitivity, and aggressive-tiling usefulness bias.

## 6) Scoring model

`robust_product_score` combines:

- expected usefulness,
- shape coverage,
- safety factor,
- worst-case safety,
- implementation complexity,
- variance sensitivity,
- overfitting risk,
- benchmark dependency risk,
- explicit penalties for exact-tuning dependency and immediate benchmark dependence,
- worst-case pass/fail penalty.

This intentionally rewards robust, clamp-friendly strategies and punishes brittle ones.

## 7) Strategy comparison

Compared candidates:

A. no variant yet
B. memory-conservative first
C. small-register first
D. balanced first
E. aggressive first
F. two-stage pair (`small-register-tile + balanced-2x2-accum4`)
plus explicit stress candidate: exact-per-gpu-tuning-first.

Result from computed model tables: the top robust score is **small-register-first** under current coefficients; the two-stage pair remains a close follow-on candidate for M7/M8 sequencing once baseline safety behavior is landed.

## 8) Final recommendation

### Direct answers

1. **Which occupancy variant should Prometheus implement first?**
   - `small-register-tile` is the computed first-implementation winner.
2. **Should P13 M7 implement one variant or a small pair?**
   - Implement one variant in M7 (`small-register-tile`) and stage `balanced-2x2-accum4` as the immediate second variant once M7 guardrails are in place.
3. **What safety margins/clamps are required?**
   - At least ~120 permille guard-band on register/shared/occupancy budgets; clamp balanced→small-register and aggressive→balanced/memory-conservative when margins degrade.
4. **Which variants are too risky for first implementation?**
   - `aggressive-4x4-accum8` as first/default strategy, and any exact-per-GPU tuned-first path.
5. **How does device variance affect recommendation?**
   - Degraded register/shared-headroom scenarios heavily increase cliff and variance penalties for aggressive variants; pair strategy preserves coverage with safer fallback behavior.
6. **Why is per-GPU Pareto tuning not used?**
   - It scores poorly due to overfitting and exact-tuning dependency risk under cross-device variance.
7. **Why is real-hardware benchmarking deferred?**
   - Explicit user constraint; M47 is simulation-only and treats immediate benchmark dependence as a risk penalty.
8. **What exactly should P13 M7 implement?**
   - Implement benchmark-only recipe plumbing for `small-register-tile`, wire clamp policy and fallback rules, keep dispatch actuation off, and queue `balanced-2x2-accum4` as the immediate follow-on.
9. **What remains deferred?**
   - Real hardware commissioning, runtime autotune, per-GPU response-surface fitting, and dispatch-path actuation.

## 9) P13 M7 implementation direction

P13 M7 should implement:

- selector-controlled benchmark-only enablement for `small-register-tile`,
- explicit clamp guards using margin thresholds,
- fallback to `memory-conservative`/`baseline-scalar` under degraded capability,
- unchanged runtime dispatch behavior (diagnostic/benchmark scope only),
- and a prepared follow-on path to add `balanced-2x2-accum4` after M7 safety telemetry confirms stable behavior.

## 10) Deferred scope

Still deferred after M47:

- native kernel optimization rollout,
- real-hardware benchmark validation,
- per-device tuning tables,
- autotune and response-surface commissioning,
- production dispatch actuation.

## Documentation consistency note

M45 reported a temporary gap where enum-typed record fields were avoided in this lab path. M47 now uses enum-typed record fields directly in recipe/scenario/strategy records, consistent with current `Language/reference` enum/record rules.
