# Prometheus SGEMM Algorithm Lab — M20

## Native `Packed4FP32` Implementation

## 1) What was implemented

M20 adds a native `Packed4FP32` SGEMM path inside the native Prometheus reactor and wires selection through the judgment engine (not ad hoc branching).

Implemented:

- New compute mode: `PROM_VK_COMPUTE_PACKED4_FP32`
- New success detail: `PROM_DETAIL_PATH_DIRECT_PACKED4_FP32`
- Packed4 fallback/rejection details:
  - `PROM_DETAIL_PACKED4_PADDING_WASTE`
  - `PROM_DETAIL_PACKED4_SMALL_SHAPE`
  - `PROM_DETAIL_PACKED4_CAPABILITY_MISSING`
  - `PROM_DETAIL_PACKED4_FALLBACK_REQUIRED`
  - `PROM_DETAIL_PACKED4_MODE_BUDGET_DENIED`
- Packed4 pack/compute routine with FP32 storage and FP32 arithmetic only.
- Deterministic fallback to scalar baseline mode when any hard gate fails.

## 2) M19 invariants mapped to native code

1. **Exact row-major equivalence**
   - Packed4 output is compared against scalar row-major oracle; any mismatch is corrected and counted (`packed4_row_major_check_failures`).
2. **Padding isolation**
   - Packed A/B allocations are zero-initialized; padded K lanes are explicitly zero.
3. **Tail completeness**
   - K is rounded to 4 with zero-padded tails; M/N are iterated over exact user extents.
4. **Deterministic fallback**
   - Judgment engine emits explicit packed-reject reason, then selects scalar baseline fallback.
5. **Reason-code observability**
   - Packed reject reasons map to explicit detail codes and fallback counters.
6. **Mode-aware packed gating**
   - Budget thresholds are mode-dependent (AGGRESSIVE > SAFE > RECOVERY).
7. **Tiny-shape rejection**
   - Explicit tiny-shape gate (`m<4 || n<4 || k<4`).

## 3) Judgment-engine integration

Packed4 eligibility facts were added to `prom_judgment_facts`, and selection/rejection surfaced via `prom_judgment_decision`:

- policy mode
- packed capability fact
- tiny-shape fact
- padding-waste metric
- mode budget
- row-major/tail validity facts
- packed selection bit
- packed reject reason enum

This keeps packed selection policy-driven and deterministic.

## 4) Reason-code observability

M20 adds explicit packed fallback details and reason counters in diagnostics:

- padding waste
- small shape
- capability missing
- fallback required
- mode budget denied

Unknown packed rejection reasons are not emitted.

## 5) Tail/padding strategy

- Pack along K into 4-lane groups (`ceil(k/4)*4`).
- A packed as row-major rows with padded K lanes set to zero.
- B packed as column-oriented K-slices per output column with padded K lanes set to zero.
- Compute consumes 4-lane blocks while preserving scalar accumulation order.
- Output remains canonical row-major FP32.

## 6) Tests added

Added/updated native Marionette tests for:

1. packed path selection on eligible shapes
2. tiny-shape fallback with explicit reason
3. padding-waste / mode-budget fallback reason observability
4. mode-aware denials via judgment-engine facts
5. non-multiple-of-4 tails (M/N/K)
6. combined tails
7. rectangular tall/wide shapes
8. repeated-call correctness stability
9. fallback reason-code counters in diagnostics
10. existing baseline/tiled/staged flows remain covered by prior suites

## 7) Deferred items

- Vulkan shader-side packed4 kernels are deferred; current packed4 path is native host-side packed compute integrated with the same judgment/controller architecture.
- FP16 storage/arithmetic candidates remain out of scope per M20 non-goals.

## Inconsistency surfaced

M19 describes M20 as C/Vulkan implementation-ready. M20 integrates packed selection and packed native execution in C with policy parity, but does not yet add a dedicated Vulkan packed shader pipeline. This is an implementation-shape gap vs. a strict “packed shader” interpretation and is intentionally deferred.
