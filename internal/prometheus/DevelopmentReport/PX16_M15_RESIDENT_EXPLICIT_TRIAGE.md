# Px16 M15 Resident Explicit Triage

## Outcome

Meaningful progression.

The original resident explicit failure pattern from the broad Px16 EVT report was reproduced as a reporting/runtime-state problem, then narrowed:

- the originally reported `square_256x256x256` and `skinny_1024x64x1024` resident explicit failures do not reproduce when each explicit row runs on a fresh runtime handle;
- the earlier broad report was conflating isolated explicit-row failure with later collateral fallout after a device-loss event on a reused runtime/process;
- the broad EVT benchmark still exposes one real remaining explicit failure outside the minimum reproduction matrix:
  - `lowk_1024x1024x64`
  - explicit `SDSL_SCALAR_PLUS`
  - `PROM_STAGE_SUBMIT`
  - `VK_ERROR_DEVICE_LOST`

After that event, later explicit rows in the same benchmark process can degrade to runtime initialization failure (`vulkan_runtime_unavailable`), so those later rows are not trustworthy root-cause evidence.

## Changes

- Added `PrometheusSgemmPx16ResidentExplicitFailureMatrix`.
- Added focused diagnostic artifacts:
  - `out/test-artifacts/prometheus_sgemm_px16_resident_failure_matrix.json`
  - `out/test-artifacts/prometheus_sgemm_px16_resident_failure_matrix.md`
- Changed explicit comparison rows in the EVT harness to use fresh runtime handles per row.
- Preserved production resident rows on the existing production lane handle so selector/production behavior is still measured in-context.
- Tightened resident benchmark buffer-size checking so `A = M*K`, `B = K*N`, and `C = M*N` are all validated through the existing checked byte-size helpers before resident allocation/readback.

## Path Comparison

- Production resident setup path:
  - `prometheus_reactor_runtime_sgemm(...)`
  - selector authority intact
  - normal production dispatch choice
- Explicit resident setup path:
  - `prometheus_reactor_runtime_sgemm_benchmark_variant(...)`
  - forced staged upload-only setup
  - timed redispatch through `prom_sgemm_resident_dispatch_once(...)`
- Important harness difference before M15:
  - explicit rows reused one runtime/process state across many comparisons
  - once one row hit `VK_ERROR_DEVICE_LOST`, later rows could look broadly broken
- Important harness difference after M15:
  - explicit rows use fresh runtime handles per row
  - the report now separates isolated failure from collateral fallout

## Focused Matrix Finding

The fresh-runtime matrix for the requested minimum shapes:

- `small_64x64x64`
- `square_128x128x128`
- `square_256x256x256`
- `skinny_1024x64x1024`

and the requested variants:

- `BASELINE_SCALAR`
- `SMALL_REGISTER_TILE`
- `MEMORY_CONSERVATIVE`
- `SDSL_SCALAR_PLUS`
- `SDSL_TILE16X16_SHARED_FP32`

shows those rows passing in isolation on the current Windows Vulkan path.

That is the key M15 result: the original `square_256x256x256` / `skinny_1024x64x1024` resident explicit failures were not isolated per-row failures once the harness stopped reusing poisoned explicit state.

## Remaining Blocker

The current next blocker is:

- isolate why `lowk_1024x1024x64` explicit `SDSL_SCALAR_PLUS` loses the device during measured dispatch;
- confirm whether the same issue exists in resident explicit mode for that shape/variant when run first in a fresh process;
- only after that row is fixed should later `vulkan_runtime_unavailable` rows in the broad benchmark be reinterpreted.

## Commands

```bat
cmd /c "call ""C:\Program Files\Microsoft Visual Studio\18\Community\Common7\Tools\VsDevCmd.bat"" -arch=x64 -host_arch=x64 >nul && internal\prometheus\native\build_windows.cmd"
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16Resident
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16ResidentExplicitFailureMatrix
out\prometheus\native\marionette_benchmarks.exe PrometheusSgemmPx16Evt
```
