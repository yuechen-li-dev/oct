# Prometheus SGEMM Algorithm Lab — M19

## Packed4FP32 Implementation-Readiness Rake Lab

M19 is a surgical correctness lab for `Packed4FP32` only. It does not implement Vulkan/native code and does not claim performance outcomes.

## 1) M18 handoff recap

M18 concluded `Packed4FP32` is the first implementation candidate, but only if:

- tail/padding handling is explicit,
- row-major output semantics are preserved exactly,
- fallback behavior is deterministic and fully observable,
- reason-code completeness is guaranteed.

M19 closes the remaining ambiguity: can this be implemented without subtle correctness surprises.

## 2) Tail/padding findings

M19 runs all 64 combinations of `(M mod 4, N mod 4, K mod 4)` using deterministic shapes and exact integer SGEMM oracle comparison.

Findings:

- all single-tail, dual-tail, and triple-tail states are covered,
- no dropped or double-counted contributions were observed,
- padded-lane accounting is explicit for every case,
- eligibility and fallback are produced deterministically.

Artifact:

- `m19_tail_exhaustive_table.octagon`

## 3) Padding contamination attack results

M19 injects deliberate non-zero garbage into padded lanes for packed A/B surfaces and re-runs the model with lane masking.

Findings:

- contamination does not alter final outputs,
- no contamination-induced mismatches were observed,
- padded lanes are effectively isolated to zero contribution.

Artifact:

- `m19_padding_contamination_table.octagon`

## 4) Row-major correctness results

For each tested shape, M19 computes:

1. scalar row-major SGEMM,
2. packed4 model SGEMM,
3. exact equality check.

Findings:

- mismatch count is zero across tested corpus,
- first mismatch location remains `none` in all passing rows,
- row-major canonical output semantics are preserved.

Artifact:

- `m19_row_major_validation_table.octagon`

## 5) Pack/unpack invariants

M19 checks round-trip behavior for packed surfaces:

- `unpack(pack(X)) == X`,
- canonical row-major ordering is preserved.

Findings:

- round-trip invariants pass for A/B/C modeled layouts,
- no ordering drift was observed.

Artifact:

- `m19_pack_unpack_invariants.octagon`

## 6) Rectangular stress findings

M19 includes tall/wide and small/large-K stress shapes.

Findings:

- row-major and contamination checks stay stable,
- no square-shape assumptions were required.

Artifact:

- `m19_rectangular_stress_table.octagon`

## 7) Mode-aware gating findings

M19 enforces mode budgets with fixed policy:

- `AGGRESSIVE` budget > `SAFE` budget > `RECOVERY` budget.

For a shared shape, `AGGRESSIVE` may pass while `SAFE`/`RECOVERY` deny.

Artifact:

- `m19_mode_budget_table.octagon`

## 8) Fallback completeness

M19 exercises rejection mappings and ensures explicit reasons:

- `PACKED4_PADDING_WASTE`
- `PACKED4_SMALL_SHAPE`
- `CAPABILITY_MISSING`
- `FALLBACK_REQUIRED`
- `MODE_BUDGET_DENIED`

No unknown fallback reason is allowed in rake outputs.

Artifact:

- `m19_fallback_reason_table.octagon`

## 9) Final invariants for implementation (C/Vulkan contract)

The implementation must enforce all of the following:

1. **Exact row-major equivalence**: packed path output must equal scalar FP32 SGEMM exactly for accepted shapes.
2. **Padding isolation**: padded lanes must contribute exactly zero, regardless of stored bytes.
3. **Tail completeness**: all true elements must be used exactly once; no tail drop or over-read contribution.
4. **Deterministic fallback**: any ineligible case must route to `ScalarFP32`.
5. **Reason-code observability**: all rejection paths emit an explicit code from the required set.
6. **Mode-aware gating**: budget check must happen before selecting packed execution.
7. **Tiny-shape rejection**: explicit lower-bound policy must reject too-small pack candidates.

## Required final answers

### 1) Is Packed4FP32 fully understood?

**Yes**, within M19 scope. Tail/padding/indexing/fallback behavior is fully enumerated in deterministic rake outputs.

### 2) Is it safe to implement?

**Yes, conditionally.** Safe if and only if the invariants above are treated as hard implementation contracts, not best-effort checks.

### 3) What invariants must implementation enforce?

See Section 9 invariant list; these are the required native contract items.

### 4) What are remaining risks?

Residual risk is implementation defect risk (index arithmetic, masked-lane enforcement, writeback offsets) rather than policy ambiguity.

### 5) What should M20 be?

Proceed to **M20 Packed4FP32 implementation**, with M19 invariants and reason-code diagnostics wired directly into the native path.

## Artifacts

- `m19_tail_exhaustive_table.octagon`
- `m19_padding_contamination_table.octagon`
- `m19_row_major_validation_table.octagon`
- `m19_pack_unpack_invariants.octagon`
- `m19_rectangular_stress_table.octagon`
- `m19_mode_budget_table.octagon`
- `m19_fallback_reason_table.octagon`
- `m19_final_readiness.octagon`
