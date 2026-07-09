# Px16 M14 SDSL Tile16x16 Shared FP32

Status: benchmark-only explicit variant wiring. Production selector scoring and default production dispatch remain unchanged.

## What landed

- First Prometheus SGEMM shared-memory tiled kernel authored in SDSL-V:
  - `internal/prometheus/shaders/sdslv/sgemm_tile16x16_shared_fp32.sdslv`
- Checked-in generated SPIR-V header for the tiled kernel:
  - `internal/prometheus/native/reactor_vulkan_sgemm_tile16x16_shared_fp32_spirv.h`
- Regeneration script updated to emit both current Prometheus SDSL-V SGEMM headers:
  - `internal/prometheus/native/generate_sdslv_shaders.ps1`
- New benchmark-only explicit occupancy variant:
  - `PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_TILE16X16_SHARED_FP32`
- New path identity:
  - `PROM_OCCUPANCY_VARIANT_PATH_ID_SDSL_TILE16X16_SHARED_FP32`

## Kernel shape

`SDSL_TILE16X16_SHARED_FP32` is the first real shared-memory tiled SGEMM kernel in the SDSL-V lane:

- one `C[row, col]` element per invocation;
- `numthreads(16, 16, 1)`;
- one `16x16` output tile per workgroup;
- one cooperative `A` load and one cooperative `B` load per invocation per `K` tile;
- `workgroup` TileA/TileB storage;
- two `WorkgroupMemoryBarrierWithSync()` barriers per `K` tile;
- row-major `A`, `B`, `C`;
- push constants `m`, `n`, `k`;
- dispatch mapping `group/thread x -> row`, `group/thread y -> col`;
- safe tail handling for non-multiple `M`, `N`, and `K`.

The shader currently uses:

- `THREADS_X = 16`
- `THREADS_Y = 16`
- `OUTPUTS_PER_INVOCATION_M = 1`
- `OUTPUTS_PER_INVOCATION_N = 1`
- `TILE_M = 16`
- `TILE_N = 16`
- `TILE_K = 16`
- `UNROLL_K = 16`

`UNROLL_K` here means fixed inner-`kk` unroll width for the compile-time `0..TILE_K` loop, not outer runtime tile-loop unrolling.

## Dispatch metadata convention

Host dispatch remains metadata-driven. Prometheus does not hand-code `16x16` workgroup coverage in C for this variant.

The generated header carries the shader's concrete dispatch/config metadata, and native dispatch uses the generated helper convention:

- `groups_x = ceil_div(M, THREADS_X * OUTPUTS_PER_INVOCATION_M)`
- `groups_y = ceil_div(N, THREADS_Y * OUTPUTS_PER_INVOCATION_N)`

For this kernel, that is intentionally equivalent to:

- `groups_x = ceil_div(M, 16)`
- `groups_y = ceil_div(N, 16)`

because each `16x16` workgroup computes one `16x16` logical output tile.

## Regeneration

```powershell
powershell -ExecutionPolicy Bypass -File internal/prometheus/native/generate_sdslv_shaders.ps1
```

That script requires a working `dxc` resolution path for regeneration, but ordinary native builds consume the checked-in generated headers and do not require `dxc`.

## Wiring summary

- Vulkan creates and destroys a dedicated shader module and compute pipeline from the generated header.
- Explicit benchmark dispatch and resident explicit comparison can request `SDSL_TILE16X16_SHARED_FP32`.
- Diagnostics/path truth report it as `WIRED`.
- Production selector authority is unchanged. This milestone does not add the new variant to production-selector tuning or promotion logic.

## Verification run

The shader lane was exercised with:

```powershell
go run ./cmd/oct sdslv check internal/prometheus/shaders/sdslv/sgemm_tile16x16_shared_fp32.sdslv
go run ./cmd/oct sdslv emit-vdmir internal/prometheus/shaders/sdslv/sgemm_tile16x16_shared_fp32.sdslv
go run ./cmd/oct sdslv emit-hlsl internal/prometheus/shaders/sdslv/sgemm_tile16x16_shared_fp32.sdslv -o out/sdslv/sgemm_tile16x16_shared_fp32.hlsl
go run ./cmd/oct sdslv compile-spv internal/prometheus/shaders/sdslv/sgemm_tile16x16_shared_fp32.sdslv -o out/sdslv/sgemm_tile16x16_shared_fp32.spv
```

Native verification used:

```bat
cmd /c "call \"C:\Program Files\Microsoft Visual Studio\18\Community\Common7\Tools\VsDevCmd.bat\" -arch=x64 -host_arch=x64 >nul && internal\prometheus\native\build_windows.cmd"
out\prometheus\native\marionette_tests.exe PrometheusSgemmPx16Resident
out\prometheus\native\marionette_benchmarks.exe PrometheusSgemmPx16Evt
```

## Current correctness evidence

The variant is end-to-end wired and produces correct results in the explicit/resident comparison rows exercised by the generated EVT artifact, including representative square and rectangular cases such as:

- `small_64x64x64`
- `square_128x128x128`
- `rect_255x129x65`

The broad correctness-validation lane still shows runtime failures on this machine for some odd-`K` and larger-shape explicit validation cases, including the new variant on shapes such as:

- `oddk_64x64x65`
- `square_256x256x256`
- `skinny_1024x64x1024`

That failure pattern is not unique to the new SDSL-V tiled kernel; other explicit variants in the same lane also fail there. M14 therefore proves the source-backed tiled kernel path and benchmark wiring, but does not claim that every heavy correctness-validation case is fully closed out yet.

## Initial resident explicit comparison snapshot

From `out/test-artifacts/prometheus_sgemm_px16_evt_report.md` after M14 wiring:

- `small_64x64x64`
  - `SDSL_TILE16X16_SHARED_FP32`: `0.01056 ms`, `49.6485 GFLOP/s`
  - `SDSL_SCALAR_PLUS`: `0.015456 ms`, `33.9213 GFLOP/s`
  - `SMALL_REGISTER_TILE`: `0.015328 ms`, `34.2046 GFLOP/s`
- `square_128x128x128`
  - `SDSL_TILE16X16_SHARED_FP32`: `0.025152 ms`, `166.758 GFLOP/s`
  - `SDSL_SCALAR_PLUS`: `0.068032 ms`, `61.6519 GFLOP/s`
  - `MEMORY_CONSERVATIVE`: `0.024672 ms`, `170.003 GFLOP/s`
- `square_512x512x512`
  - `SDSL_TILE16X16_SHARED_FP32`: `2.91069 ms`, `92.2241 GFLOP/s`
  - `SMALL_REGISTER_TILE`: `4.11046 ms`, `65.3054 GFLOP/s`
  - `MEMORY_CONSERVATIVE`: `3.99222 ms`, `67.2396 GFLOP/s`
- `lowk_1024x1024x64`
  - `SDSL_TILE16X16_SHARED_FP32`: `1.42659 ms`, `94.0828 GFLOP/s`
  - `BALANCED_2X2_ACCUM4`: `3.49459 ms`, `38.4073 GFLOP/s`
  - `MEMORY_CONSERVATIVE`: `2.04339 ms`, `65.6838 GFLOP/s`

That is enough for M14. The milestone goal is a real source-backed shared-memory tiled kernel with generated dispatch metadata and honest comparison data, not immediate production promotion.
