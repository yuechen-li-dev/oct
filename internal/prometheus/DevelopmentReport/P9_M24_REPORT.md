# P9 M24 — `FP16StorageFP32Accum` Vulkan Path (Policy-Gated)

## 1) Implementation overview

M24 adds a real Vulkan compute candidate for `FP16StorageFP32Accum` under the existing SGEMM reactor path.

- Added new compute mode: `PROM_VK_COMPUTE_FP16_STORAGE_FP32_ACCUM`.
- Added dedicated Vulkan compute pipeline from `reactor_vulkan_fp16_spirv.h`.
- Added packed FP16 upload representation (`uint32` words carrying two IEEE-754 half values) for A/B.
- Shader expands FP16 values to FP32 via `unpackHalf2x16`, accumulates in FP32, and writes canonical FP32 row-major C.

This preserves the M23 contract that FP16 is a policy-gated candidate, not an implicit fast path.

## 2) M23 policy-to-code mapping

M23 hard gates are represented as explicit judgment facts and rejection reasons:

1. `StrictFP32 == false` → `strict_fp32` gate (`PROM_FP16_REJECT_STRICT_FP32`)
2. `ToleranceKnown == true` → `tolerance_known` gate (`PROM_FP16_REJECT_TOLERANCE_UNKNOWN`)
3. `TolerancePass == true` → `tolerance_pass` gate (`PROM_FP16_REJECT_TOLERANCE_EXCEEDED`)
4. `HasSpecialValues == false` → `has_special_values` gate (`PROM_FP16_REJECT_SPECIAL_VALUE`)
5. `CapabilityFP16Storage == true` → `capability_fp16_storage` gate (`PROM_FP16_REJECT_CAPABILITY_MISSING`)
6. `FallbackAvailable == true` → `fallback_available` gate (`PROM_FP16_REJECT_FALLBACK_REQUIRED`)

If gates pass, FP16 still must win utility to be selected; otherwise it is rejected with `PROM_FP16_REJECT_NOT_TOP_UTILITY`.

## 3) Conversion strategy

- Upload-side quantization: deterministic `float32 -> fp16 bits` conversion with round-to-nearest behavior.
- Storage layout: pairwise packed halves into `uint32` words.
- Shader-side expansion: `unpackHalf2x16` per fetched word/lane.
- Accumulation/output: FP32 MAC, FP32 output, canonical row-major C.

## 4) Shader design

The M24 shader:

- reads packed FP16 A/B from storage buffers (`uint[]`),
- reconstructs FP32 operands per element using `unpackHalf2x16`,
- executes the same scalar accumulation order (`kk = 0..k-1`) used by baseline SGEMM,
- writes FP32 output to C.

No vendor-specific extension path is used.

## 5) Judgment engine integration

Judgment engine updates include:

- explicit FP16 compute mode and detail code (`PROM_DETAIL_PATH_DIRECT_FP16_STORAGE_FP32_ACCUM`),
- explicit FP16 rejection enum and deterministic reason mapping,
- hard-gate enforcement before FP16 can be selected,
- utility arbitration between candidates (FP16 only wins when eligible and top utility).

## 6) Diagnostics implementation

`PrometheusSgemmPolicyDiagnostics` now reports FP16 diagnostics fields:

- `fp16_max_absolute_error`
- `fp16_max_relative_error`
- `fp16_aggregate_error`
- `fp16_worst_case_element_index`
- `fp16_k_error_growth`
- `fp16_cancellation_risk`
- `fp16_tolerance_known`
- `fp16_tolerance_pass`
- `fp16_fallback_reason_detail`
- `fp16_selected_candidate`

Fallback reasons are surfaced through explicit detail codes (`PROM_DETAIL_FP16_*`).

## 7) Test coverage

Added/updated tests:

- `PrometheusJudgmentEngine_FP16PolicyGatesAreExplicitAndDeterministic`
- `PrometheusJudgmentEngine_FP16CanWinOnlyWhenTopUtilityAndAllGatesPass`
- `PrometheusReactor_FP16DiagnosticsArePopulatedAndReasonCoded`

These cover gate enforcement, utility-conditioned selection, and diagnostics/reason observability.

## 8) Known limitations

- Current FP16 tolerance modeling is correctness-first CPU-side modeling; thresholds are fixed constants and not yet externally configured.
- `CapabilityFP16Storage` is currently reactor capability + policy-flag-driven and does not yet include richer per-device feature-tier profiling.
- Candidate utility remains heuristic and deliberately conservative.
