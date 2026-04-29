# P13 — Prometheus Kernel Control / Characterization

## M16 EVT Report — Occupancy Variant Family (Benchmark-Only) Closeout

## 1) Scope

M16 completes the **EVT (Engineering Validation Test)** phase for the full occupancy variant family:

* real SPIR-V shader assets exist,
* benchmark-only dispatch seam is wired,
* all variants execute through runtime,
* correctness is verified via CPU oracle,
* production dispatch remains unchanged.

No performance validation, no hardware tuning, no policy actuation.

---

## 2) Implemented Variant Map

Final EVT variant map:

```text
baseline-scalar       -> baseline pipeline (production-enabled)
memory-conservative   -> MC-baseline-strict (explicit alias)
small-register-tile   -> SRT-2accum-K
balanced-2x2-accum4   -> B2x2-row-major-biased
aggressive-4x4-accum8 -> A2x4-row-biased-accum8
```

All non-baseline variants are:

```text
benchmark_enabled = true
production_eligible = false
dispatch_enabled = false
```

---

## 3) Execution Integrity

### 3.1 Real Execution Paths

All non-baseline variants now execute **distinct shader pipelines**:

```text
SRT   → srt_2accum_k
B2x2  → b2x2_row_major_biased
A2x4  → a2x4_row_biased_accum8
MC    → baseline (explicit alias)
```

No fallback cheating (except MC by design).

### 3.2 Path Identity

Diagnostics confirm:

```text
requested_variant == executed_variant
variant_path_id != baseline (for SRT/B2x2/A2x4)
fallback_reason == none (except MC)
```

---

## 4) Correctness Validation

All variants validated against CPU oracle:

* small-square
* medium-square
* K-heavy
* wide-short
* tall-skinny
* irregular / tail-heavy shapes
* odd-K cases

Results:

```text
All variants produce correct output
No NaNs or silent corruption
Tail handling verified
```

Correctness is the only EVT success criterion.

---

## 5) Controller Integration Status

The full hybrid control stack is now connected to real compute:

```text
feedforward selector
→ lease controller (utility + hard gates)
→ HFSM lifecycle
→ variant dispatch seam
→ real shader execution (benchmark-only)
```

Important:

* controller decisions are exercised,
* dispatch remains policy-disabled for non-baseline,
* no accidental production actuation.

---

## 6) Lifecycle / Promotion Seam

Explicit lifecycle fields now exist:

```text
benchmark_enabled
dvt_validated
pvt_validated
production_eligible
dispatch_enabled
```

Current state:

```text
baseline:
  production_eligible = true
  dispatch_enabled = true

non-baseline:
  benchmark_enabled = true
  dvt_validated = false
  pvt_validated = false
  production_eligible = false
  dispatch_enabled = false
```

This enables clean transition:

```text
EVT → DVT → PVT → Production
```

without code surgery.

---

## 7) Test Suite Status

Test lanes:

```text
marionette_tests       → correctness (fast)
marionette_slow_tests  → stress
marionette_benchmarks  → variant + harness
```

Validation:

```text
build_stub.sh → PASS
marionette_tests → PASS
marionette_benchmarks → PASS
SGEMM-focused tests → PASS
```

Expected skips are environment-dependent.

---

## 8) Limitations (Intentional)

Not implemented in EVT:

```text
performance validation
hardware tuning
dispatch actuation
lookahead actuator (M51)
autotune / response-surface
multi-device validation
```

Aggressive variant remains:

```text
benchmark-only
shape-gated
high-risk by design
```

---

## 9) Risk Assessment

### Low risk

* correctness
* path identity
* controller integration
* lifecycle structure

### Medium risk

* aggressive variant stability under large shapes
* memory pressure under real hardware
* register pressure cliffs (unmeasured)

### Known deferred risks

* cross-device behavior
* transfer overlap / lookahead correctness
* performance regressions

---

## 10) EVT Conclusion

M16 achieves EVT goals:

```text
All variant recipes are real
All paths are executable
All outputs are correct
Controller integrates with real compute
Lifecycle promotion path is defined
```

No fake implementations remain.

---

## 11) Next Phase

Proceed to:

## P13 DVT — Local Hardware Validation (RTX 3070)

Goals:

```text
verify correctness on real GPU
confirm no driver/runtime issues
observe coarse timing (non-authoritative)
generate DVT artifact report
```

Then:

```text
PVT → multi-device/cloud validation
Production → policy-controlled dispatch enablement
```

---

## 12) Philosophy

EVT proves the system exists and behaves correctly.

It does not prove it is fast.

That comes later.

This system is now **real**.
