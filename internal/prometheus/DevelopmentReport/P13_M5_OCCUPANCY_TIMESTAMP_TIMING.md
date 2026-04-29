# P13 M5 — Occupancy Benchmark Timestamp Timing Integration

## 1) M4 handoff summary

P13 M4 delivered a native Marionette occupancy benchmark harness with deterministic shape cases, deterministic inputs, CPU SGEMM oracle validation, warmup/measured iterations, variant-availability behavior, acceptance gates, and artifact emission (`prometheus.sgemm.occupancy_benchmark.v1`).

M4 intentionally used:

- `timing_source = cpu_wall_clock`
- `timing_confidence = low`

because SGEMM runtime calls did not expose a trustworthy Vulkan timestamp-query timing path into the harness.

## 2) Timing capability audit

Audit result against required areas:

1. `timestampPeriod` stored in runtime
   - **Before M5:** missing (period was probed but not persisted for harness timing).
   - **M5 status:** **available with small extension** (`timestamp_period_ns` captured and exported).
2. Physical-device timestamp support queried
   - **Before M5:** partially available (physical properties read; no explicit timing capability export).
   - **M5 status:** **available with small extension** (period + queue family valid bits captured and exported).
3. Queue timestamp capability checked
   - **Before M5:** missing in diagnostics path.
   - **M5 status:** **available with small extension** (`timestamp_valid_bits` captured from selected compute queue family).
4. Command-buffer timestamp reset/write path
   - **Before M5:** missing.
   - **M5 status:** **available now** (`vkCmdResetQueryPool`, pre/post-dispatch `vkCmdWriteTimestamp`).
5. Harness access without public API bloat
   - **Before M5:** missing.
   - **M5 status:** **available with small extension** (internal diagnostics extension via `PrometheusSgemmPolicyDiagnostics`; no new broad public benchmark entrypoint).
6. Safe exposure of last-call GPU timing diagnostics
   - **Before M5:** missing.
   - **M5 status:** **available now** (`last_gpu_duration_ns`, validity flag, failure reason, availability metadata).

Deferred in this audit:

- Benchmark publication/commissioning workflows.
- Runtime dispatch actuation.
- Autotune / response-surface fitting.

## 3) Timestamp timing design

M5 uses the smallest safe extension aligned to **Option A**:

- Runtime records last SGEMM timestamp-query duration when available.
- Runtime exports timing metadata through `prometheus_reactor_runtime_sgemm_policy_diagnostics`.
- Harness reads timing diagnostics after each measured iteration.
- Harness uses GPU timing when all measured iterations provide valid timestamp durations; otherwise it explicitly falls back to CPU wall-clock timing.

This keeps timing integration internal to existing runtime diagnostics plumbing and avoids adding a broad new public benchmark API.

## 4) Vulkan timestamp-query behavior

When timestamp timing is available, SGEMM path now:

1. Creates a timestamp query pool (2 slots) during runtime init.
2. Resets query slots in command recording.
3. Writes pre-dispatch timestamp.
4. Dispatches SGEMM kernel.
5. Writes post-dispatch timestamp.
6. Submits command buffer and waits for completion in synchronous path.
7. Reads query results with `vkGetQueryPoolResults`.
8. Converts timestamp ticks to nanoseconds using `timestampPeriod`.
9. Records valid/invalid timing diagnostics and reason code.

Checks and failure handling implemented:

- unsupported capability,
- query-pool creation unavailable,
- invalid/non-positive timestamp period,
- unavailable query result,
- invalid timestamp ordering,
- command failure path marking timing invalid.

## 5) Fallback behavior

Fallback remains explicit and conservative:

- If Vulkan timestamp timing is unavailable or invalid for measured iterations:
  - `timing_source = cpu_wall_clock`
  - `timing_confidence = low`
  - actuation gate remains blocked.

Harness also includes focused test hooks to simulate unavailable/invalid timestamp scenarios and a high-confidence path simulation when environment capability is absent.

## 6) Artifact schema updates

Kept schema id as backward-compatible v1:

- `timing_source`
- `timing_confidence`
- `timestamp_available`
- `timestamp_failure_reason`
- `gpu_duration_ns_min`
- `gpu_duration_ns_mean`
- `gpu_duration_ns_median`
- `timing_stability_permille`

No schema version bump was required because additions are additive.

## 7) Acceptance gate behavior

Gate logic remains correctness-first and diagnostics-first:

- correctness failure still blocks actuation,
- low timing confidence blocks actuation,
- diagnostics mismatch blocks actuation,
- unavailable/non-implemented variant blocks actuation,
- baseline fallback cannot become actuation-ready.

M5 changes only timing evidence quality; it does not force actuation.

## 8) Tests added

Added M5-focused Marionette facts:

1. timestamp unavailable fallback,
2. timestamp available high-confidence path (real when possible, simulated fallback otherwise),
3. invalid timestamp result blocks confidence,
4. timing does not override correctness failure,
5. artifact schema includes timestamp fields,
6. smoke mode remains CI-safe.

Existing SGEMM and full native suite coverage were re-run to ensure no behavior regressions.

## 9) Behavior intentionally unchanged

Unchanged by M5:

- no new occupancy kernels,
- no runtime dispatch switching to occupancy variants,
- no SGEMM numerical behavior changes,
- no batch concurrency contract changes,
- no public performance claim path.

## 10) Deferred scope

Still deferred:

- optimized occupancy kernel variants,
- runtime dispatch actuation,
- runtime autotune,
- response-surface fitting,
- per-device commissioning,
- public performance claims,
- benchmark publication workflow.

## Consistency note

M46 preferred timestamp-query timing and required explicit low-confidence fallback. M4 implemented fallback only. M5 closes this gap by wiring timestamp-query timing into the harness via diagnostics while preserving explicit fallback semantics.
