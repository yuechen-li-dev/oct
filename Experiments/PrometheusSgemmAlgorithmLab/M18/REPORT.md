# Prometheus SGEMM Algorithm Lab — M18

## Rake Lab for `Packed4FP32` and `FP16StorageFP32Accum`

M18 is a correctness/policy rake lab only. It does not implement Vulkan kernels, does not claim timing wins, and does not use vendor-specific assumptions.

## 1) M17 handoff summary

M17 handed off two near-term candidates with distinct risk profiles:

- `Packed4FP32` is a layout/vectorization candidate with primary risk in tail/padding/indexing correctness.
- `FP16StorageFP32Accum` is a storage/bandwidth candidate with primary risk in quiet numerical degradation and tolerance-policy ambiguity.

M18 therefore rake-tests both directly and separately before implementation commitment.

## 2) Candidates under rake

- Candidate A: `Packed4FP32`
- Candidate B: `FP16StorageFP32Accum`

Deferred combination (`Packed4FP16StorageFP32Accum`) remains out of scope except for policy interaction notes.

## 3) Risks tested for `Packed4FP32`

M18 evaluates tails, padding, and fallback behavior using deterministic rows covering:

- non-multiple-of-4 tails (including 1, 2, 3, 5, 7, 15, 17),
- combined tail scenarios across M/N/K,
- rectangular cases (`M >> N`, `N >> M`),
- mode-aware padding budget behavior (`AGGRESSIVE`, `SAFE`, `RECOVERY`),
- capability-gated fallback to `ScalarFP32`,
- row-major canonical output check status (`true` in the model).

Tracked metrics:

- `TailCount`
- `PaddedLaneCount`
- `PaddingWastePct`
- `PackedEligible`
- `ModeBudgetPass`
- `IndexingRiskClass`
- `RowMajorCanonicalOutputCheck`
- `FallbackReason`

## 4) Risks tested for `FP16StorageFP32Accum`

M18 models FP32→FP16 storage→FP32 drift behavior with policy gates for:

- strict FP32 contracts,
- explicit tolerance presence,
- tolerance pass/fail,
- capability/fallback requirements,
- special value conservative rejection,
- K-sensitive growth pressure,
- cancellation-prone conditions.

Tracked metrics:

- `MaxAbsoluteErrorMilli`
- `MaxRelativeErrorPPM`
- `AggregateErrorMilli`
- `KSensitiveErrorGrowthMilli`
- `TolerancePass`
- `CapabilityGatePass`
- `SpecialValueRejected`
- `FallbackReason`

Policy result: missing tolerance is a hard block; strict FP32 is a hard block; special values are hard blocked for FP16 storage in M18.

## 5) Cross-candidate policy findings

Cross-decision rows examine strict correctness, both-eligible, packed-blocked, capability-missing, and special-value contexts.

Findings:

- strict FP32 allows `Packed4FP32` when packed gates pass and blocks `FP16StorageFP32Accum`,
- when both are eligible, M18 policy picks `Packed4FP32` first,
- any failed gate results in explicit `ScalarFP32` fallback with reason codes,
- no vendor-specific path is modeled or required.

## 6) Required judgment-engine gates

Mandatory hard gates before native rollout:

1. `Packed4FP32` requires: packed capability, fallback availability, non-tiny shape, mode-budget pass.
2. `FP16StorageFP32Accum` requires: FP16 storage capability, fallback availability, strict-FP32 off, tolerance explicitly provided, tolerance pass, no special values.
3. Missing capabilities must block corresponding candidates.
4. Fallback to `ScalarFP32` must always exist and be observable.

## 7) Required diagnostics / observability counters

Mandatory counters/log reasons in future native path:

- `packed4_tail_count`
- `packed4_padded_lane_count`
- `packed4_mode_budget_denials`
- `packed4_row_major_check_failures`
- `fp16_max_abs_error`
- `fp16_max_rel_error`
- `fp16_aggregate_error`
- `fp16_k_growth`
- `fp16_special_value_rejections`
- `fallback_reason_code_count`

Recommended reason-code family:

- `PACKED4_PADDING_WASTE`
- `PACKED4_SMALL_SHAPE`
- `FP16_STRICT_FP32`
- `FP16_TOLERANCE_UNKNOWN`
- `FP16_TOLERANCE_EXCEEDED`
- `FP16_SPECIAL_VALUE`
- `CAPABILITY_MISSING`

## 8) Final recommendation

### Implement first

- `Packed4FP32`, with mandatory tail/padding gates and explicit fallback reasons.

### Rake more (before implementation)

- `FP16StorageFP32Accum`, until tolerance contract and error-observability policy are fully enforced in judgment outputs.

### Defer

- `Packed4FP16StorageFP32Accum` (combined risk remains too entangled for first implementation pass).

### Reject for now

- No permanent rejection in M18, but `FP16StorageFP32Accum` remains blocked by default when tolerance is missing/strict/special-value contexts apply.

## Required final answers

- Is `Packed4FP32` safe enough to implement first? **Yes, conditionally** (with hard gates and diagnostics above).
- Is `FP16StorageFP32Accum` safe enough to implement soon? **Not yet by default**; safe only behind explicit tolerance opt-in and conservative special-value rejection.
- What exact gates are mandatory? **Capability + fallback + strict correctness + tolerance-known + tolerance-pass + special-value block + mode-aware packed budget.**
- What should M19 be? **Implement `Packed4FP32` only with M18 gate/diagnostic parity and explicit reason-code observability.**

## Artifacts

- `m18_packed4_tail_padding_rake_table.octagon`
- `m18_fp16_error_tolerance_rake_table.octagon`
- `m18_cross_candidate_decision_table.octagon`
- `m18_final_recommendation.octagon`
