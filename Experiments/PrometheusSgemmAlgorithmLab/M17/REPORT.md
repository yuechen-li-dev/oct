# Prometheus SGEMM Algorithm Lab — M17

## Data Layout / Precision Candidate Discovery Lab

M17 is a pure Oct simulation and analysis milestone. It does not claim hardware timings and does not implement Vulkan kernels.

## 1) Candidate list and rationale

Compared candidate families:

1. `ScalarFP32` — correctness-first baseline and mandatory fallback.
2. `Packed4FP32` — vectorized packed buffer layout without arithmetic precision change.
3. `FP16StorageFP32Accum` — bandwidth reduction via storage quantization with FP32 accumulation retained.
4. `Packed4FP16StorageFP32Accum` — combined packed layout + FP16 storage reduction.
5. `FP16StorageFP16Arithmetic` — maximal bandwidth/throughput ambition with highest numerical risk.

Naming intentionally avoids image semantics (`RGBA`) because these are SGEMM buffer layouts.

## 2) Factors studied

M17 modeled and gated candidates using:

- shape factors: small/medium/large effects, square vs rectangular behavior, 4-alignment and non-multiples-of-4, padding waste.
- memory factors: conceptual transfer volume, shared-footprint estimate, conversion/copy burden.
- precision factors: strict FP32 contracts, tolerance for storage quantization, opt-in requirement for FP16 arithmetic.
- capability factors: packed layout feasibility, FP16 storage support, FP16 arithmetic support, fallback availability.
- controller factors: `aggressive`, `safe`, and `recovery` mode-specific packed-padding budgets.

## 3) Metric definitions (structural only)

M17 emits non-hardware metrics:

- `TransferVolumeUnits` — conceptual input/output traffic proxy.
- `PaddingWasteUnits` — sum of 4-lane padding deltas over M/N/K dimensions.
- `ConversionCostUnits` — conceptual conversion work for quantized paths.
- `LayoutComplexityScore` — complexity introduced by packing/tails.
- `SharedFootprintUnits` — conceptual local-memory pressure proxy.
- `ArithmeticPrecisionRiskScore` — risk from precision-changing arithmetic/storage.
- `FallbackRequired` — whether safe fallback must be present.
- `ImplementationComplexityScore` and `CorrectnessRiskScore` — implementation and defect-risk proxies.

These metrics are for policy reasoning only and are not GPU timings.

## 4) Per-candidate findings

- **ScalarFP32**
  - Use: **yes**.
  - Why: always eligible and precision-safe.
  - Role: baseline and mandatory fallback.

- **Packed4FP32**
  - Use: **yes** when packed feasibility and fallback are present and padding waste is within mode budget.
  - Best fit: medium/large shapes with low tail waste.
  - Main risks: non-multiple-of-4 indexing/tail handling bugs.

- **FP16StorageFP32Accum**
  - Use: **yes** when tolerance explicitly allows FP16 storage and strict FP32 contract is not required.
  - Main value: lower transfer pressure while preserving FP32 accumulation behavior.
  - Main risks: storage conversion drift and tolerance ambiguity.

- **Packed4FP16StorageFP32Accum**
  - Use: **only experimental**.
  - Main value: larger transfer reduction opportunity.
  - Main risks: compounded padding + conversion complexity and reduced diagnosability.

- **FP16StorageFP16Arithmetic**
  - Use: **only behind explicit precision opt-in**.
  - Main value: maximal reduction path.
  - Main risks: high numerical drift risk and silent correctness degradation.

## 5) Recommended use conditions

- `Packed4FP32`: use if shape is not tiny, packed4 is feasible, fallback exists, and padding waste is low enough for current controller mode.
- `FP16StorageFP32Accum`: use if FP16 storage capability exists, tolerance permits storage quantization, strict FP32 is not required, and fallback exists.
- `FP16StorageFP16Arithmetic`: never auto-enable; require explicit opt-in plus capability and fallback gates.

## 6) Rejection conditions

Reject candidate selection when any hard gate fails:

- strict FP32 correctness contract blocks all FP16-storage and FP16-arithmetic options.
- missing capability (`Packed4Feasible`, `FP16StorageAvailable`, `FP16ArithmeticAvailable`).
- fallback path unavailable for non-baseline candidates.
- high packed padding waste or tiny shapes for packed candidates.

## 7) Judgment-engine policy sketch

M17 output is shaped for future C judgment-engine intake:

- candidate identity strings (`ScalarFP32`, `Packed4FP32`, etc.)
- eligibility facts and denial reasons
- use disposition (`yes`, `no`, `only experimental`, `only behind explicit precision opt-in`)
- hard gates: strict precision, capabilities, fallback, mode-aware padding budget
- fallback chain:
  - `Packed4FP32 -> ScalarFP32`
  - `FP16StorageFP32Accum -> ScalarFP32`
  - `Packed4FP16StorageFP32Accum -> FP16StorageFP32Accum -> ScalarFP32`
  - `FP16StorageFP16Arithmetic -> FP16StorageFP32Accum -> ScalarFP32`
- observability counters for later implementation:
  - quantization error metrics
  - padding/tail guard metrics
  - fallback trigger counts
  - precision-contract violation counters

## 8) High-risk candidates and M18 rake targets

M17 flags these for rake-first work before implementation broadening:

1. `FP16StorageFP32Accum` (high)
   - conversion roundtrip drift
   - tolerance classification ambiguity
   - magnitude edge sweeps and CPU/GPU drift checks

2. `Packed4FP16StorageFP32Accum` (very high)
   - combined padding + precision defect patterns
   - ordering mistakes (convert-before-pack vs pack-before-convert)
   - non-multiple-of-4 tail correctness

3. `FP16StorageFP16Arithmetic` (critical)
   - shape-dependent error growth
   - non-associativity amplification
   - silent degradation detection under opt-in workloads

## 9) Recommended next milestone

**M18 should be a precision/layout risk rake lab** focused first on `FP16StorageFP32Accum`, with explicit tail/padding checks carried in parallel for packed candidates.

### Required final status call

- **Test first for implementation:** `Packed4FP32` (best near-term risk/reward with unchanged arithmetic precision).
- **Needs M18 rake lab first:** `FP16StorageFP32Accum` (priority), then `Packed4FP16StorageFP32Accum` and `FP16StorageFP16Arithmetic`.
- **Deferred:** `Packed4FP16StorageFP32Accum`, `FP16StorageFP16Arithmetic`.
- **Rejected for now:** no permanent rejection; `FP16StorageFP16Arithmetic` is rejected by default unless explicit precision opt-in and M18 evidence exist.

## Artifacts

- `m17_candidate_table.octagon`
- `m17_condition_matrix.octagon`
- `m17_risk_handoff.octagon`
- `m17_recommendation.octagon`
