# Prometheus SGEMM Algorithm Lab — M23

## FP16StorageFP32Accum Judgment-Engine / Observability Parity Lab

M23 is a policy wiring and observability correctness lab. It does not add Vulkan execution, new numerical rakes, or performance claims.

## 1) M22 handoff recap

M22 established that `FP16StorageFP32Accum` is numerically implementable only behind hard policy gates:

- never automatic,
- explicit tolerance opt-in required,
- strict FP32 / unknown tolerance / tolerance exceeded / special values / missing capability / missing fallback must reject,
- diagnostics and reason codes must be explicit.

M23 consumes this handoff as policy facts and verifies deterministic policy execution with `when utility` candidate selection.

## 2) Policy model structure

M23 introduces:

- a scenario record carrying policy facts (`StrictFP32`, `ToleranceKnown`, `ToleranceValue`, tolerance limits, measured errors, K-growth, cancellation risk, special-value flag, capability, fallback availability, shape class, controller mode),
- derived gate evaluation (`M23GateResultsFor`) with explicit hard gates,
- utility candidate competition (`M23SelectCandidate`) across:
  - `ScalarFP32`
  - `Packed4FP32`
  - `FP16StorageFP32Accum`
- deterministic fallback reason precedence (`M23FallbackReason`),
- explicit diagnostic payload emitted in every decision row.

The FP16 option is guarded by `when gates.FP16Eligible` so it cannot win by score when any hard gate fails.

## 3) Gate enforcement validation

Required hard gates are modeled as explicit booleans and combined into `FP16Eligible`:

1. `StrictFP32GatePass = not StrictFP32`
2. `ToleranceKnownGatePass = ToleranceKnown`
3. `TolerancePass = (MeasuredMaxAbsError <= ToleranceAbsLimit) and (MeasuredMaxRelError <= ToleranceRelLimitPPM)`
4. `SpecialValueGatePass = not HasSpecialValues`
5. `CapabilityGatePass = CapabilityFP16Storage`
6. `FallbackAvailableGatePass = FallbackAvailable`

Validation scenarios confirm deterministic rejection for:

- strict FP32,
- missing tolerance,
- tolerance exceeded,
- special values,
- capability missing,
- fallback unavailable.

## 4) Fallback behavior validation

Fallback is explicit in all cases:

- hard-gate failure paths use stable reason codes:
  - `FP16_STRICT_FP32`
  - `FP16_TOLERANCE_UNKNOWN`
  - `FP16_TOLERANCE_EXCEEDED`
  - `FP16_SPECIAL_VALUE`
  - `FP16_CAPABILITY_MISSING`
  - `FALLBACK_REQUIRED`
- non-failure-but-not-selected path uses `FP16_NOT_TOP_UTILITY`, making candidate competition observable rather than ambiguous.

`SelectedCandidate`, `RejectedCandidates`, and `FallbackReason` are emitted together for each scenario.

## 5) Diagnostic completeness validation

Each scenario emits `DiagnosticPayload` with:

- `MaxAbsoluteError`
- `MaxRelativeError`
- `AggregateError`
- `WorstCaseElementIndex`
- `KErrorGrowth`
- `CancellationRisk`
- `TolerancePass`
- `FallbackReason`
- `SelectedCandidate`

This mirrors M22 mandatory observability and keeps policy outcomes auditable.

## 6) Ambiguity analysis

M23 removes ambiguity by design:

- FP16 availability is binary (`FP16Eligible`) and independent from utility scoring,
- reason-code precedence is fixed and deterministic,
- mixed-candidate competition is explainable via candidate score table and explicit `FP16_NOT_TOP_UTILITY`.

No decision relies on implicit defaults.

## 7) Edge-case handling

Edge scenarios required by M23 are present and validated:

- strict FP32 hard reject,
- missing tolerance hard reject,
- tolerance exceeded hard reject,
- tolerance pass can select FP16,
- special values hard reject,
- capability missing hard reject,
- fallback unavailable hard reject,
- large-K/high-growth rejects when tolerance does not pass,
- cancellation-heavy rejects when tolerance does not pass,
- mixed packed-vs-fp16 competition with explainable priority.

## 8) Final policy contract

The final contract artifact records the policy decision answer set:

- FP16 storage is policy-safe only behind hard gates,
- non-default behavior is retained,
- explicit opt-in tolerance remains mandatory,
- all failure modes stay observable,
- deterministic fallback is required and represented.

## Required final answers

1. **Can FP16 storage be safely enforced by policy?**
   - **Yes, conditionally.** Safe only when all hard gates pass and policy outputs are preserved.
2. **Are all failure modes observable?**
   - **Yes.** Every rejection path emits explicit reason code + gate states + diagnostics.
3. **Is fallback always deterministic?**
   - **Yes.** Ordered gate precedence and explicit selected candidate make fallback deterministic.
4. **Can silent correctness degradation occur?**
   - **No, under this contract.** FP16 cannot be selected if any hard gate fails.
5. **Is the judgment engine representation sufficient?**
   - **Yes.** `when utility` + hard gate predicates + structured diagnostics is sufficient.
6. **What is required before implementation?**
   - Runtime judgment engine must preserve this gate matrix, reason-code set, diagnostic payload fields, and non-default FP16 policy exactly.

## Artifacts

- `m23_policy_decision_table.octagon`
- `m23_gate_matrix.octagon`
- `m23_candidate_selection_table.octagon`
- `m23_fallback_reason_table.octagon`
- `m23_diagnostic_payload_table.octagon`
- `m23_final_policy_contract.octagon`
