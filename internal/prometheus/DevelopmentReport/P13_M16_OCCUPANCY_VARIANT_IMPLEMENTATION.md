# P13 M16 — Occupancy Variant Implementation (Benchmark-Only)

## 1) M52 handoff summary

M52 finalized the occupancy recipe mapping:

- baseline-scalar → existing baseline/current path
- memory-conservative → MC-baseline-strict
- small-register-tile → SRT-2accum-K
- balanced-2x2-accum4 → B2x2-row-major-biased
- aggressive-4x4-accum8 → A2x4-row-biased-accum8

M16 originally implemented these as benchmark-visible variants with correctness-first behavior and no dispatch actuation.

## 2) Implemented variants

Implemented and benchmark-available variants:

- baseline-scalar
- memory-conservative
- small-register-tile
- balanced-2x2-accum4
- aggressive-4x4-accum8

That original benchmark-only limitation is now historical. After later wiring milestones plus Px16 M3 EVT semantics cleanup, all wired variants are dispatch-enabled / production-eligible under EVT rules, while DVT/PVT lifecycle fields remain telemetry only.

## 3) Implementation notes per variant

### baseline-scalar
Runs baseline runtime SGEMM path directly.

### memory-conservative (MC-baseline-strict)
Benchmark identity is available. Execution remains baseline runtime path under benchmark-only dispatch-disabled fallback.

### small-register-tile (SRT-2accum-K)
Benchmark identity is available. Execution remains baseline runtime path under benchmark-only dispatch-disabled fallback.

### balanced-2x2-accum4 (B2x2-row-major-biased)
Benchmark identity is available. Execution remains baseline runtime path under benchmark-only dispatch-disabled fallback.

### aggressive-4x4-accum8 (A2x4-row-biased-accum8)
Benchmark identity is available. Execution remains baseline runtime path under benchmark-only dispatch-disabled fallback. Additional shape gate is surfaced in fallback diagnostics for smaller shapes.

## 4) Fallback/tail handling

Fallback behavior is explicit in the benchmark harness:

- unavailable variant → unavailable fallback
- available but dispatch-disabled variant → baseline fallback with reason `dispatch_disabled_benchmark_only`
- aggressive variant on non-large shapes → baseline fallback with reason `aggressive_shape_gate_fallback`

No silent correctness degradation is allowed. Real safety gating remains the judgment-engine clamp path rather than DVT/PVT promotion booleans.

## 5) Correctness validation results

Validation runs required in M16 were executed; correctness remains CPU-oracle-based and bounded by existing tolerance checks with recorded max absolute/relative error and first failing index in artifacts.

## 6) Benchmark harness integration

Harness now reports:

- `selected_variant`
- `tested_variant`
- `variant_available`
- `variant_dispatch_enabled`
- `fallback_reason`
- correctness and error metrics

This preserves benchmark observability while keeping dispatch actuation disabled.

## 7) Limitations

- No performance claims or tuning are included.
- This limitation applied to the original M16 implementation state and was removed by later wiring milestones.

## 8) Deferred work

Deferred unchanged:

- optimized kernel-body implementation and tuning
- runtime dispatch actuation
- hardware benchmarking and response-surface fitting
- autotune
- prefetch actuator implementation
- multi-device validation
