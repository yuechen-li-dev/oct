# Prometheus SGEMM Algorithm Lab — M22

## Rake Lab for `FP16StorageFP32Accum`

M22 is a correctness/policy rake lab only. It does not implement Vulkan FP16, does not introduce FP16 arithmetic, and does not make timing claims.

## 1) M17/M18 handoff recap

M17 and M18 flagged `FP16StorageFP32Accum` as promising but policy-sensitive:

- representation change is real (`FP32 -> FP16 storage -> FP32`) even if accumulation/output stay FP32,
- drift risk grows with K and cancellation,
- tolerance ambiguity creates silent-correctness risk,
- strict FP32 and unknown tolerance must be hard blockers.

M22 narrows scope to those unresolved numerical/policy questions only.

## 2) FP16 storage model

M22 uses deterministic integer-scaled modeling:

- value units are milli-FP32 surrogates,
- conversion model is deterministic quantization (`M22FP16QuantizeMilli`),
- near-zero underflow-like behavior maps tiny magnitudes to zero,
- overflow-like magnitudes clamp to conservative finite range,
- all measured outputs remain in scalar-integer diagnostics (`MaxAbsoluteError`, `MaxRelativeError`, `AggregateError`, `WorstCaseElementIndex`).

This keeps results reproducible and policy-comparable without introducing floating-point nondeterminism in the rake.

## 3) Conversion drift findings

Roundtrip table (`m22_fp16_roundtrip_table.octagon`) shows deterministic non-zero drift in normal, near-zero, and mixed profiles.

Observed pattern:

- absolute and relative error are bounded but non-zero,
- worst-case index is stable per scenario,
- aggregate drift is measurable even when individual values are small.

Conclusion: roundtrip drift is predictable enough for gating, but not negligible enough for automatic defaulting.

## 4) Magnitude edge findings

Magnitude-edge table (`m22_fp16_magnitude_edge_table.octagon`) covers:

- very small values,
- large values,
- mixed magnitudes,
- negative-heavy values.

Findings:

- tiny/near-zero lanes show the highest relative sensitivity,
- large-value lanes remain bounded but still quantized,
- mixed-sign/mixed-scale data increases aggregate drift visibility.

## 5) K-growth findings

K-growth table (`m22_fp16_k_growth_table.octagon`) exercises small/medium/large/very-large K.

Findings:

- `KErrorGrowth` increases monotonically with K for a fixed profile,
- non-zero conversion drift compounds with longer reductions,
- policy must treat K as an explicit risk amplifier, not a hidden constant.

## 6) Cancellation findings

Cancellation table (`m22_fp16_cancellation_table.octagon`) compares lighter vs heavier cancellation profiles.

Findings:

- cancellation-heavy vectors produce higher cancellation risk and more visible relative drift,
- even with FP32 accumulation, storage quantization materially affects near-canceling sums,
- tolerance checks must account for both absolute and relative metrics.

## 7) Tolerance-policy findings

Policy-gate table (`m22_fp16_policy_gate_table.octagon`) enforces explicit regimes:

- strict FP32,
- tolerance known vs unknown,
- tolerance pass vs exceeded,
- special values present,
- capability/fallback availability.

Results:

- strict FP32 blocks FP16 storage (`FP16_STRICT_FP32`),
- missing tolerance blocks FP16 storage (`FP16_TOLERANCE_UNKNOWN`),
- too-tight tolerance blocks FP16 storage (`FP16_TOLERANCE_EXCEEDED`),
- eligible only when all hard gates pass.

## 8) Special-value policy

M22 default policy is conservative:

- special values are treated as hard reject for FP16 storage,
- reason code is explicit (`FP16_SPECIAL_VALUE`),
- selection falls back to `ScalarFP32`.

This is intentionally strict until explicit special-value handling semantics are added in a later milestone.

## 9) Required judgment-engine gates

M22 requires all gates below to pass before selecting `FP16StorageFP32Accum`:

1. `StrictFP32GatePass` (`not strict_fp32`)
2. `ToleranceKnownGatePass` (tolerance explicitly provided)
3. `TolerancePass` (measured/modelled max abs + max rel within policy)
4. `SpecialValueGatePass` (no special values in M22 policy)
5. `CapabilityGatePass` (`fp16_capability && fallback_available`)

If any gate fails, final candidate is `ScalarFP32` with explicit fallback reason.

## 10) Required diagnostics/observability

M22 marks these fields mandatory in selection diagnostics:

- `MaxAbsoluteError`
- `MaxRelativeError`
- `AggregateError`
- `WorstCaseElementIndex`
- `KErrorGrowth`
- `CancellationRisk`
- `TolerancePass`
- `FallbackReason`
- `SelectedCandidate`

Required reason-code set in this milestone:

- `FP16_STRICT_FP32`
- `FP16_TOLERANCE_UNKNOWN`
- `FP16_TOLERANCE_EXCEEDED`
- `FP16_SPECIAL_VALUE`
- `FP16_CAPABILITY_MISSING`
- `FALLBACK_REQUIRED`

No unknown rejection reason appears in M22 gate rows.

## 11) Final recommendation

M22 recommendation (`m22_fp16_final_recommendation.octagon`):

- implementability: **yes, but gated**,
- default behavior: **not automatic**,
- policy contract: **explicit tolerance opt-in is mandatory**,
- strict FP32 / unknown tolerance / special-value contexts: **must hard-block**.

## Required final answers

1. **Should `FP16StorageFP32Accum` be implemented soon?**
   - **Yes, conditionally.** It is implementable behind hard gates and diagnostics.
2. **Should it ever be automatic?**
   - **No, not in M22 policy.** Automatic selection is unsafe without explicit tolerance and context gating.
3. **Should it require explicit tolerance opt-in?**
   - **Yes.** Tolerance must be explicit and must pass measured/modelled risk.
4. **What exact gates are mandatory?**
   - strict-FP32-off, tolerance-known, tolerance-pass, special-value-pass, capability+fallback-pass.
5. **What diagnostics are mandatory?**
   - max abs/rel error, aggregate error, worst-case element index, K growth, cancellation risk, tolerance-pass, fallback reason, selected candidate.
6. **What should the next milestone be?**
   - **M23:** wire these FP16 storage gates/reason-codes into judgment-engine selection output and verify runtime observability parity before any default-selection discussion.

## Artifacts

- `m22_fp16_roundtrip_table.octagon`
- `m22_fp16_magnitude_edge_table.octagon`
- `m22_fp16_k_growth_table.octagon`
- `m22_fp16_cancellation_table.octagon`
- `m22_fp16_policy_gate_table.octagon`
- `m22_fp16_final_recommendation.octagon`
