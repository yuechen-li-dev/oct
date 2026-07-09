## Px16 M1

Px16 M1 wires the SGEMM occupancy judgment-engine decision into the production dispatch path. Production `prometheus_reactor_runtime_sgemm()` calls now bind the selector's clamped `selected_variant` as the dispatch variant used for tiled pipeline selection instead of staying pinned to baseline at the public API boundary.

The architecture rule for this milestone is:

- The judgment engine is the sole production dispatch authority for SGEMM occupancy variant selection.
- `occupancy_apply_safety_clamp` remains the live safety gate on that decision.
- P15 feedforward and matured reservations remain prestage, latency-hiding, and telemetry only; they do not override the production dispatch variant.
- Promotion lifecycle fields such as DVT, PVT, production eligibility, and dispatch-enabled remain diagnostic metadata only for this milestone and do not gate the new production dispatch wiring.

Deferred work:

- Hooking up the memory-conservative SPIR-V kernel is intentionally deferred to a later Px16 milestone.

## Px16 M4

Px16 M4 removes the blanket SAFE-mode SGEMM direct-path suppression that previously set `force_direct` solely because the controller was in `PROM_POLICY_MODE_SAFE`. SAFE mode now means guardrails, diagnostics, and concrete hazard fallback rather than automatic slow-path dispatch.

The architecture rule for this milestone is:

- SAFE policy may still reach tiled SGEMM production dispatch when the shape is eligible, the selected occupancy variant is wired, and no concrete hazard requires direct fallback.
- The judgment engine remains the production dispatch authority.
- `occupancy_apply_safety_clamp` remains the live engineering safety gate for occupancy variants.
- Direct fallback remains available for explicit overrides and concrete hazards, with path-level diagnostics exposing force-direct reason and selected path/compute state.
- DVT/PVT/promotion lifecycle fields remain telemetry only.
- P15 mismatch correction remains deferred.
- Selector performance tuning remains deferred.

## Px16 M5

Px16 M5 ports the Shadow Authority rake lab M5 feedforward validation model into the native SGEMM/P15 path. Native P15 reconciliation now compares the matured reserved/prestaged occupancy variant against the live judgment-engine-selected occupancy variant once per real SGEMM call, after the production decision exists.

The architecture rule for this milestone is:

- The judgment engine remains the sole production dispatch authority.
- P15 feedforward remains prestage, latency-hiding, and telemetry only.
- Matching matured reservations record a reconciliation hit and consume once.
- Variant mismatch never overrides dispatch; the live SGEMM call still requests and executes the judgment-engine-selected variant.
- Mismatch now feeds the existing predictor correction/confidence machinery and retires the stale reservation path instead of silently collapsing into a generic no-reservation bucket.
- Native block reasons now follow the Shadow Authority rake lab M5 taxonomy, including `VariantMismatch`, `StaleReservation`, `CancelledReservation`, `AlreadyConsumed`, and `ReservationNotReady`.

Deviation note:

- Native reuse of the existing reservation state machine still materializes stale mismatch cleanup as reservation expiry/cancellation transitions rather than adding a new reservation state enum.

## Px16 M6

Px16 M6 hardens the native SGEMM production diagnostics surface so one post-call snapshot can answer, without inference, what the selector recommended, what safety selected, what dispatch variant/path was requested, what path actually executed, whether direct was forced, and whether P15 agreed with the live decision.

The architecture rule for this milestone is:

- Diagnostics are the witness, not the dispatch authority.
- The judgment engine remains the sole production dispatch authority.
- `occupancy_apply_safety_clamp` remains the live safety gate.
- P15 remains prestage/telemetry/correction only and does not override the live dispatch variant.
- Wired EVT variants remain production-eligible and dispatch-enabled even when DVT/PVT/promotion lifecycle telemetry is false.
- SAFE policy remains hazard/feasibility based; it is not a blanket direct-path override.

Compact truth table:

- `px16_m6_selector_recommended_variant`: the selector recommendation before clamp. In M6 this mirrors `p13_m2_occupancy_unclamped_variant`.
- `px16_m6_selector_selected_variant`: the clamped live selector decision. In M6 this mirrors `p13_m2_occupancy_selected_variant`.
- `px16_m6_requested_dispatch_variant`: the occupancy variant handed to dispatch after selector authority is applied.
- `px16_m6_executed_dispatch_variant`: the occupancy variant identity actually bound by the executed compute mode. If the call ends up on a non-tiled compute path, this reports baseline rather than pretending a tiled occupancy pipeline ran.
- `px16_m6_requested_path` / `px16_m6_selected_path` / `px16_m6_executed_path`: requested, chosen, and actually used Vulkan path identity.
- `px16_m6_requested_compute_mode` / `px16_m6_selected_compute_mode` / `px16_m6_executed_compute_mode`: explicit compute-mode truth. M6 does not model a separate requested compute mode, so the requested field mirrors the selected mode.
- `px16_m6_force_direct_requested`: explicit caller/test-seam direct request.
- `px16_m6_force_direct_applied`: direct was actually forced by either explicit override or concrete fallback/hazard handling.
- `px16_m6_force_direct_reason`: `EXPLICIT_OVERRIDE`, `SAFE_CONCRETE_HAZARD`, or `NONE`. SAFE alone must still report `NONE`.
- `px16_m6_variant_path_status`, `px16_m6_variant_production_eligible`, `px16_m6_variant_dispatch_enabled`: factual EVT wiring/path truth for the requested occupancy variant.
- `px16_m6_variant_dvt_validated`, `px16_m6_variant_pvt_validated`, `px16_m6_variant_lifecycle_telemetry_only`: DVT/PVT/promotion lifecycle fields remain telemetry only and do not gate a wired variant.
- `px16_m6_p15_reservation_present`, `px16_m6_p15_reservation_matured`, `px16_m6_p15_reservation_consumed`: whether P15 had and used a relevant reservation.
- `px16_m6_p15_reserved_variant_id`: the P15 reserved/prestaged occupancy variant.
- `px16_m6_p15_live_selected_variant_id`: the live judgment-engine-selected occupancy variant used for reconciliation.
- `px16_m6_p15_reconciliation_match`: whether P15 matched the live decision.
- `px16_m6_p15_block_reason`, `px16_m6_p15_correction_action`, `px16_m6_p15_reservation_stale_or_expired`: mismatch/block/correction truth when P15 diverges from the live decision.
- `px16_m6_p15_confidence_before` / `px16_m6_p15_confidence_after`: cheap before/after confidence snapshots around reconciliation/correction.

## Px16 M7

Px16 M7 adds the local Windows EVT benchmark/report lane that measures the production SGEMM path, validates correctness, captures timing, and emits deterministic artifacts that expose the full M6 truth surface for each representative shape.

The architecture rule for this milestone is:

- The lane measures and reports current behavior; it does not retune selector heuristics.
- Main EVT results must call `prometheus_reactor_runtime_sgemm(...)`.
- Explicit benchmark-variant dispatch remains optional and separate from the production lane.
- The judgment engine remains the production dispatch authority.
- P15 remains prestage/telemetry/correction only.
- Diagnostics remain witness data, not dispatch authority.

### How to run

Build the native Windows artifacts from a Visual Studio developer shell:

```bat
cmd /c "call \"C:\Program Files\Microsoft Visual Studio\18\Community\Common7\Tools\VsDevCmd.bat\" -arch=x64 -host_arch=x64 >nul && internal\prometheus\native\build_windows.cmd"
```

Run the focused regression checks called out by the milestone:

```bat
out\prometheus\native\marionette_tests.exe PrometheusReactor_Px16M6_ProductionDiagnosticsTruthSurface
out\prometheus\native\marionette_tests.exe PrometheusReactor_P13_M16B5_SafePolicyAllowsEligibleTiledDispatch
out\prometheus\native\marionette_tests.exe PrometheusDominatusPredictorCorrection_ReconciliationVariantMismatchExpiresReservationAndLowersConfidence
```

Run the EVT lane directly:

```bat
out\prometheus\native\marionette_benchmarks.exe PrometheusSgemmPx16Evt
```

Optional 2048-cube coverage is intentionally off by default to keep the lane locally repeatable. Enable it explicitly when desired:

```bat
set OCT_PROMETHEUS_PX16_EVT_ENABLE_2048=1
out\prometheus\native\marionette_benchmarks.exe PrometheusSgemmPx16Evt
```

### Artifact paths

The lane writes:

- `out/test-artifacts/prometheus_sgemm_px16_evt_results.json`
- `out/test-artifacts/prometheus_sgemm_px16_evt_report.md`

These generated artifacts should not be committed by default.

### Report interpretation

- `Production SGEMM Results` is the main EVT table. Every row comes from the production API path, not from explicit benchmark variants.
- `path` reports requested/selected/executed Vulkan path truth.
- `compute_mode` reports requested/selected/executed compute truth, including whether execution stayed tiled.
- `variant` reports selector recommended/selected plus requested/executed dispatch variant truth and the EVT wiring status.
- `force_direct` reports whether direct execution was requested or actually applied, plus the concrete reason.
- `p15` reports whether a matured reservation existed, whether it matched the live selected variant, and what correction action happened if it did not.
- `correctness.reference_mode` explains whether the row used the dense CPU oracle or the deterministic separable large-shape oracle. Tolerance remains the repository SGEMM tolerance style.
- `timing.timing_source` distinguishes `vulkan_timestamp_query` from `cpu_wall_clock`. If timestamps are unavailable or invalid for any measured iteration, the row falls back to wall-clock timing and the report says so explicitly.

### Anomalies

The lane flags explicit anomalies, including:

- selected/requested/executed variant mismatches,
- unexpected direct fallback,
- SAFE force-direct without a concrete reason,
- wired variants not reported as wired / production-eligible / dispatch-enabled,
- P15 mismatch without correction action,
- correctness failure,
- large eligible shapes that did not execute tiled,
- suspicious GFLOP/s drops versus neighboring successful shapes.

These flags are reporting aids only. They do not alter runtime dispatch behavior.

### Known limitation

If Vulkan timestamp queries are unavailable or invalid, timing falls back to CPU wall-clock. In that case the lane still reports correctness and diagnostics truth, but absolute performance numbers should be interpreted as lower-confidence host-observed timings rather than pure GPU execution timings.

## Px16 M8

Px16 M8 extends the EVT lane so the report can separate production-selector behavior from explicit-variant comparison and separate GPU kernel time from end-to-end staged-path wall time.

The architecture rule for this milestone is:

- The production EVT lane still measures `prometheus_reactor_runtime_sgemm(...)`.
- Explicit variant comparison uses `prometheus_reactor_runtime_sgemm_benchmark_variant(...)` and is reported as explicit comparison only, not as production selector behavior.
- The judgment engine remains the production dispatch authority.
- P15 remains prestage/telemetry/correction only.
- Diagnostics remain factual witness data and do not retune dispatch.

### Hardware guard

The M8 lane now records the exact Vulkan backend and adapter identity in the artifact and refuses to characterize performance on the wrong device.

- Expected local adapter: `NVIDIA GeForce RTX 3070`
- Expected backend: hardware `VULKAN`
- Rejected backends/devices:
  - `PROM_BACKEND_VULKAN_SOFTWARE`
  - CPU Vulkan devices
  - `llvmpipe`
  - any non-3070 adapter selected by the runtime

If the runtime is not on the expected local 3070 hardware path, the lane writes guarded artifacts and skips instead of silently publishing misleading numbers.

### How to run M8

Build the native Windows artifacts from a Visual Studio developer shell:

```bat
cmd /c "call \"C:\Program Files\Microsoft Visual Studio\18\Community\Common7\Tools\VsDevCmd.bat\" -arch=x64 -host_arch=x64 >nul && internal\prometheus\native\build_windows.cmd"
```

Run the focused regression checks:

```bat
out\prometheus\native\marionette_tests.exe PrometheusReactor_Px16M6_ProductionDiagnosticsTruthSurface
out\prometheus\native\marionette_tests.exe PrometheusReactor_P13_M16B5_SafePolicyAllowsEligibleTiledDispatch
out\prometheus\native\marionette_tests.exe PrometheusDominatusPredictorCorrection_ReconciliationVariantMismatchExpiresReservationAndLowersConfidence
```

Run the EVT lane:

```bat
out\prometheus\native\marionette_benchmarks.exe PrometheusSgemmPx16Evt
```

Optional knobs:

```bat
set OCT_PROMETHEUS_PX16_EVT_ENABLE_2048=1
out\prometheus\native\marionette_benchmarks.exe PrometheusSgemmPx16Evt
```

```bat
set OCT_PROMETHEUS_PX16_EVT_ENABLE_EXPLICIT_1024_CUBE=1
out\prometheus\native\marionette_benchmarks.exe PrometheusSgemmPx16Evt
```

## Px16 M10a Note

M10a adds reduction loop attributes to SDSL-V so tile-style shader math can express `[unroll] sum` and `[loop] product` without dropping the backend hint.

The `sgemm_tile16x16_shared_fp32.sdslv` inner fixed `TILE_K` accumulation loop was evaluated for a source-level refactor to `[unroll] sum`, but the production shader should stay on the explicit `[unroll] for` form unless the native correctness and EVT lanes remain green after regeneration.

The architecture and runtime rules are unchanged:

- the outer runtime tile loop remains explicitly `[loop]`;
- production dispatch authority does not change;
- selector tuning, P15, FFT/P16, and dispatch metadata do not change;
- correctness validation remains separate from benchmark timing.

### Artifact paths

The lane still writes the main production artifact pair:

- `out/test-artifacts/prometheus_sgemm_px16_evt_results.json`
- `out/test-artifacts/prometheus_sgemm_px16_evt_report.md`

These generated artifacts should not be committed by default.

### Report interpretation

The M8 report keeps the M7 `Production SGEMM Results` table and adds:

- `Timing Decomposition`
  - `total_wall_ms` is the median end-to-end host-observed call duration.
  - `kernel_gpu_ms` is the median Vulkan timestamp kernel duration when every measured iteration produced a valid GPU timestamp.
  - `upload_ms`, `readback_ms`, and `sync_wait_ms` are host-observed wall slices from the current SGEMM runtime path. They are factual diagnostics, not inferred estimates.
  - `end_to_end_gflops` uses `total_wall_ms`.
  - `kernel_only_gflops` uses `kernel_gpu_ms`.
- `Explicit Variant Comparison`
  - compares `BASELINE_SCALAR`, `SMALL_REGISTER_TILE`, `BALANCED_2X2_ACCUM4`, `AGGRESSIVE_4X4_ACCUM8`, and `MEMORY_CONSERVATIVE`
  - every row still requires correctness
- `Selector vs Fastest Variant`
  - compares the production executed variant against the fastest correct explicit variant on the same shape
  - uses kernel timing when valid for the full comparison set, otherwise total wall timing
- `Performance Diagnosis`
  - summarizes whether current evidence points to transfer/staging overhead, kernel-side slowdown, selector choice, or unresolved timing gaps

### Known limitations

- Resident/persistent no-readback mode is not implemented in M8; the report records `resident_device_mode_available=false`.
- If GPU timestamps are unavailable or invalid for a case, `kernel_only_gflops` is unavailable for that case and the report says so explicitly instead of faking kernel-only timing.

## Px16 M14

Px16 M14 adds the first source-backed SDSL-V shared-memory tiled SGEMM kernel to the explicit-comparison lane without changing production selector authority.

The architecture rule for this milestone is:

- The new kernel is benchmark-only and explicit-variant only.
- Production `prometheus_reactor_runtime_sgemm(...)` selection remains unchanged.
- The judgment engine remains the sole production dispatch authority.
- Dispatch geometry for the new SDSL-V tiled variant must come from generated metadata beside the checked-in SPIR-V header, not from hand-coded host constants.
- Correctness validation remains separate from benchmark timing.

### New explicit variant

- Occupancy variant: `PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_TILE16X16_SHARED_FP32`
- Path identity: `PROM_OCCUPANCY_VARIANT_PATH_ID_SDSL_TILE16X16_SHARED_FP32`
- SDSL-V source: `internal/prometheus/shaders/sdslv/sgemm_tile16x16_shared_fp32.sdslv`
- Generated header: `internal/prometheus/native/reactor_vulkan_sgemm_tile16x16_shared_fp32_spirv.h`

### Dispatch convention

The generated metadata for `SDSL_TILE16X16_SHARED_FP32` uses:

- `THREADS_X = 16`
- `THREADS_Y = 16`
- `OUTPUTS_PER_INVOCATION_M = 1`
- `OUTPUTS_PER_INVOCATION_N = 1`
- `TILE_M = 16`
- `TILE_N = 16`
- `TILE_K = 16`
- `UNROLL_K = 16`

Prometheus host dispatch continues to use the generated coverage convention:

- `groups_x = ceil_div(M, THREADS_X * OUTPUTS_PER_INVOCATION_M)`
- `groups_y = ceil_div(N, THREADS_Y * OUTPUTS_PER_INVOCATION_N)`

For this kernel that resolves to `ceil_div(M, 16)` by `ceil_div(N, 16)`, but that `16` remains a shader-generated fact rather than a native hand-coded constant.

### Regeneration

```powershell
powershell -ExecutionPolicy Bypass -File internal/prometheus/native/generate_sdslv_shaders.ps1
```

Ordinary native builds still consume checked-in generated headers and do not require `dxc` unless a shader is being regenerated.

### Report interpretation for M14

The existing `Resident Explicit Variant Comparison`, `Explicit Variant Comparison`, and `Selector vs Fastest Variant` sections now include `SDSL_TILE16X16_SHARED_FP32` alongside the earlier explicit SGEMM variants.

This milestone is considered successful when the report shows:

- the new SDSL-V tiled variant is requestable and reported as `WIRED`;
- explicit/resident comparison rows include it;
- its dispatch geometry is metadata-driven;
- benchmark output is reported honestly even if the kernel is not the fastest option.

Current repo evidence shows exactly that. The new kernel is present in resident and explicit comparison tables, and it can beat several older explicit kernels on resident kernel time for some shapes, but no selector promotion has been attempted.

## Px16 M9

Px16 M9 separates SGEMM performance benchmarking from CPU correctness/oracle validation.

The architecture rule for this milestone is:

- Unit/FACT tests validate correctness.
- Benchmarks measure performance.
- CPU oracle/reference work is not part of default benchmark timing.
- The default benchmark reports production GPU SGEMM operation timing, not GPU SGEMM plus CPU oracle plus validation bookkeeping.
- The judgment engine remains the production dispatch authority; M9 does not tune selector heuristics, optimize kernels, change FFT/P16 work, or add resident-buffer APIs.

### What changed

The default Px16 benchmark lane is now a standard benchmark:

```bat
out\prometheus\native\marionette_benchmarks.exe PrometheusSgemmPx16Evt
```

That command runs `PrometheusSgemmPx16Evt_ProductionPerformanceLane`. It prepares deterministic inputs, calls the production SGEMM path and explicit comparison variants, records timing, and writes the performance artifact pair. It does not build dense CPU oracle output or run full output validation for every benchmark shape by default.

Correctness moved to a separate FACT lane:

```bat
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16Evt_CorrectnessValidationLane
```

The validation lane uses focused small/medium shapes plus a structured large case. It verifies production SGEMM correctness, explicit variants, memory-conservative coverage through the wired variant set, odd-K behavior, and requested/executed diagnostic identity without making every large performance case pay for a dense CPU reference.

### Artifact paths

The performance benchmark writes:

- `out/test-artifacts/prometheus_sgemm_px16_evt_results.json`
- `out/test-artifacts/prometheus_sgemm_px16_evt_report.md`

The correctness lane writes:

- `out/test-artifacts/prometheus_sgemm_px16_evt_validation_results.json`
- `out/test-artifacts/prometheus_sgemm_px16_evt_validation_report.md`

These files are generated benchmark/test artifacts, not source edits, and should not be committed by default.

### Report interpretation

The M9 report schema is `prometheus.sgemm.px16.evt.v3`.

- `run_mode=performance_benchmark` means CPU oracle/reference work was not run in the benchmark lane.
- `validation_status_source=not_run_in_benchmark_mode` means correctness status is intentionally delegated to the FACT validation lane.
- `benchmark_total_ms` is the measured SGEMM operation wall time for the production API call.
- `kernel_gpu_ms` is the Vulkan timestamp kernel duration when available.
- `upload_ms`, `readback_ms`, `dispatch_submit_ms`, and `sync_wait_ms` are host-observed runtime buckets from the current staged production API path.
- `oracle_ms`, `validation_readback_ms`, and `validation_ms` are zero in default performance mode. They are populated only when validation is explicitly requested.
- `unaccounted_host_ms` reports the remaining host-observed wall time not explained by the known runtime buckets.

The current production API can still use staged upload/readback paths. M9 does not invent resident device buffers, so reports should be read with:

```text
resident_device_mode_available=false
production API still includes staged path
```

### Selector-vs-fastest semantics

M9 removes the ambiguous `picked fastest?` presentation. The report now splits:

- `picked_same_variant_as_fastest_explicit`
- `production_slower_than_fastest_explicit`
- `production_vs_fastest_ratio`

This distinguishes variant identity from measured speed. Production can run faster than an explicit row due to timing noise, warm state, or path differences even when it did not execute the same variant as the fastest explicit comparison row.

## Px16 M10

Px16 M10 feeds real RTX 3070 DVT evidence back into the production SGEMM occupancy selector. This is a selector-policy milestone only: it does not optimize SPIR-V kernels, change FFT/P16 work, add resident-buffer APIs, or move dispatch authority away from the judgment engine.

The selector now uses centralized, hand-tunable utility scores in `reactor_judgment_engine.c`. Those constants are DVT-tunable policy values, so future hardware evidence can adjust variant preference without rewriting the selector's control flow.

The main DVT correction is that `MEMORY_CONSERVATIVE` is no longer treated as only a weak/register-constrained-device fallback. RTX 3070 measurements showed it can win or stay competitive on high-capability discrete GPUs for:

- wide or short/wide shapes such as `64x1024x1024`,
- rectangular or skinny-ish shapes,
- low-K shapes where larger register-blocked kernels do not amortize cleanly,
- odd or awkward dimensions such as `255x129x65`,
- some small/medium shapes where footprint and overhead dominate.

The selector still keeps the other wired variants in the candidate set: `BASELINE_SCALAR`, `SMALL_REGISTER_TILE`, `BALANCED_2X2_ACCUM4`, and `AGGRESSIVE_4X4_ACCUM8`. Large square and FFN-like compute-rich shapes can still prefer aggressive or balanced variants, and small square shapes can still prefer SRT. `MEMORY_CONSERVATIVE` is considered from general facts such as device band, register/shared-memory tolerance, shape class, aspect ratio, low-K status, and odd/awkward dimensions; the selector does not hardcode the RTX 3070 device name.

`occupancy_apply_safety_clamp` remains the authoritative safety gate after scoring. If a selected or manually overridden variant is unsafe for the facts, clamp behavior still demotes or rejects it. DVT/PVT/promotion lifecycle fields remain telemetry only, and P15 remains prestage/telemetry/correction rather than a production selector.

The benchmark/report lane remains an honest measurement surface. Selector scoring changes may reduce selector-vs-fastest mismatches on MC-favored shapes, but the report still compares the production executed variant against the fastest correct explicit variant and continues to surface mismatches instead of hiding them.

## Px16 M11

Px16 M11 adds a benchmark-only resident device-buffer SGEMM mode. The new diagnostic path uploads A/B once through the existing staged device-local buffer machinery, keeps A/B/C resident across timed iterations, dispatches the selected SGEMM kernel repeatedly, and reads C back only after the timed loop when validation is explicitly requested.

The architecture rule for this milestone is:

- Production `prometheus_reactor_runtime_sgemm(...)` remains the public production path and dispatch authority.
- The judgment engine remains the production selector authority.
- Resident mode is benchmark/diagnostic surface area, not a normal Oct user API.
- Explicit resident variant comparison is comparison-only telemetry.
- Correctness validation remains separate from default benchmark timing.
- Selector tuning, shader optimization, FFT/P16 work, and CUDA/vendor-specific paths are out of scope.

### Staged vs resident

The report now distinguishes:

- staged production end-to-end timing from `prometheus_reactor_runtime_sgemm(...)`;
- resident production selector timing, where the selector chooses the variant but A/B/C stay device-resident during timed iterations;
- resident explicit variant timing for `BASELINE_SCALAR`, `SMALL_REGISTER_TILE`, `BALANCED_2X2_ACCUM4`, `AGGRESSIVE_4X4_ACCUM8`, and `MEMORY_CONSERVATIVE`;
- Vulkan timestamp kernel timing when query results are valid.

Resident timing currently uses one submit/wait per timed dispatch because the backend timestamp query pool provides one start/end timestamp pair. This still removes repeated upload/readback from timed iterations and gives a steady-state device-resident comparison surface.

### How to run M11

Build native artifacts:

```bat
cmd /c "call \"C:\Program Files\Microsoft Visual Studio\18\Community\Common7\Tools\VsDevCmd.bat\" -arch=x64 -host_arch=x64 >nul && internal\prometheus\native\build_windows.cmd"
```

Run the focused resident infrastructure test:

```bat
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16Resident
```

Run the correctness lane:

```bat
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16Evt_CorrectnessValidationLane
```

Run the benchmark/report lane:

```bat
out\prometheus\native\marionette_benchmarks.exe PrometheusSgemmPx16Evt
```

### Artifact fields

The main performance benchmark still writes:

- `out/test-artifacts/prometheus_sgemm_px16_evt_results.json`
- `out/test-artifacts/prometheus_sgemm_px16_evt_report.md`

New resident fields include:

- `resident_device_mode_available`
- `resident_production`
- `resident_variant_comparison`
- `selector_vs_fastest_resident`
- `resident_upload_once_ms`
- `resident_setup_ms`
- `resident_kernel_median_ms`
- `resident_kernel_min_ms`
- `resident_kernel_p95_ms`
- `resident_total_loop_ms`
- `resident_iterations`
- `resident_readback_once_ms`
- `resident_validation_ms`
- `resident_kernel_only_gflops`
- `resident_loop_gflops`
- `gpu_timestamp_valid`

The Markdown report adds:

- `Resident Device Benchmark`
- `Resident Explicit Variant Comparison`
- `Selector vs Fastest Resident Variant`

Generated artifacts under `out/test-artifacts/` are benchmark/test outputs and should not be committed by default.

### Correctness discipline

Default benchmark mode does not run CPU oracle validation inside the timed loop. Resident mode performs no per-iteration readback. The FACT validation lane may request one final resident readback after timing, then runs the CPU oracle outside the native timed path.

If resident mode is unavailable, the report says so explicitly instead of faking resident numbers.

## Px16 M13

Px16 M13 adds the first SDSL-V-authored Prometheus SGEMM kernel as a benchmark-only explicit occupancy variant. This milestone proves the end-to-end source-backed lane:

`SDSL-V -> VD-MIR -> HLSL -> DXC/SPIR-V -> checked-in header -> Vulkan pipeline -> benchmark resident comparison`

The new explicit variant is:

- `PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_SCALAR_PLUS`

The new path identity is:

- `PROM_OCCUPANCY_VARIANT_PATH_ID_SDSL_SCALAR_PLUS`

The architecture rule for this milestone is:

- The new SDSL-V kernel is benchmark/comparison surface only.
- Production selector candidates and selector scoring are unchanged.
- `BASELINE_SCALAR` remains the production scalar baseline identity.
- Existing resident and explicit comparison variants remain intact.
- Shared-memory tiled SDSL-V SGEMM remains deferred.

### Source-backed kernel artifacts

- SDSL-V source:
  - `internal/prometheus/shaders/sdslv/sgemm_scalar_baseline_plus.sdslv`
- Checked-in generated header:
  - `internal/prometheus/native/reactor_vulkan_sgemm_scalar_plus_spirv.h`
- Opt-in regeneration script:
  - `internal/prometheus/native/generate_sdslv_shaders.ps1`

The scalar-plus kernel keeps the current Prometheus SGEMM ABI:

- binding `0`: row-major `A`
- binding `1`: row-major `B`
- binding `2`: row-major `C`
- push constants `m`, `n`, `k` at offsets `0`, `4`, `8`
- dispatch mapping `global x -> row`, `global y -> col`

### Regeneration

Ordinary native builds consume the checked-in generated header and do not require DXC. Regeneration is opt-in:

```powershell
powershell -ExecutionPolicy Bypass -File internal/prometheus/native/generate_sdslv_shaders.ps1
```

That script:

- checks the SDSL-V source;
- emits HLSL and SPIR-V into `out/sdslv/`;
- regenerates `reactor_vulkan_sgemm_scalar_plus_spirv.h`;
- removes temporary native-side `.hlsl` / `.spv` sidecars so only the checked-in header remains under `internal/prometheus/native/`.

### Benchmark and correctness status

M13 extends the explicit variant comparison and resident explicit comparison tables to include `SDSL_SCALAR_PLUS`. Correctness remains separate from default timing:

- benchmark/report lane:
  - `out\prometheus\native\marionette_benchmarks.exe PrometheusSgemmPx16Evt`
- correctness lane:
  - `out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16Evt_CorrectnessValidationLane`

Coverage now includes the new source-backed variant across representative square, odd-K, and rectangular shapes such as:

- `64x64x64`
- `64x64x65`
- `255x129x65`

M13 intentionally does not retune selector scoring to prefer the new variant yet. If resident comparison shows it beating the current selector choice on some shapes, that is useful evidence for later milestones rather than a defect in this milestone.

## Px16 M13a

Px16 M13a closes the host/shader dispatch source-of-truth gap for SDSL-generated SGEMM kernels before real tiled kernels are introduced.

The architecture rule for this milestone is:

- generated SDSL-V shader headers must carry dispatch metadata derived from the same compute entry/config that generated the shader;
- Prometheus host dispatch for `SDSL_SCALAR_PLUS` must consume those generated constants instead of hardcoded `8x8` / one-output assumptions;
- benchmark-only SDSL wiring remains benchmark-only;
- selector scoring, production dispatch authority, P15 behavior, FFT/P16 work, and legacy variant behavior remain unchanged.

Current SDSL-generated headers now emit:

- `numthreads_x/y/z`
- semantic SGEMM coverage constants such as `outputs_per_invocation_m/n`, `tile_m/n`, and `unroll_k`
- deterministic `config_*` constants for the concrete compile config

For SGEMM dispatch, the native runtime still uses the existing row/column convention:

- dispatch `x` covers rows (`m`)
- dispatch `y` covers columns (`n`)

The SDSL path now derives `groups_x` and `groups_y` from generated metadata so future multi-output kernels do not repeat the Px16 M12 B2x2/aggressive overdispatch drift bug.
