# Px16 M13 SDSL Scalar Plus

Status: benchmark-only explicit variant wiring. Production selector scoring and default variant selection are unchanged.

## What landed

- First Prometheus SGEMM kernel authored in SDSL-V:
  - `internal/prometheus/shaders/sdslv/sgemm_scalar_baseline_plus.sdslv`
- First checked-in Prometheus SGEMM SPIR-V header generated through the SDSL-V toolchain:
  - `internal/prometheus/native/reactor_vulkan_sgemm_scalar_plus_spirv.h`
- Opt-in regeneration script:
  - `internal/prometheus/native/generate_sdslv_shaders.ps1`
- New benchmark-only explicit occupancy variant:
  - `PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_SCALAR_PLUS`
- New path identity:
  - `PROM_OCCUPANCY_VARIANT_PATH_ID_SDSL_SCALAR_PLUS`

## Kernel shape

`SDSL_SCALAR_PLUS` is intentionally simple:

- one `C[row, col]` element per invocation;
- no workgroup/shared memory;
- no barriers;
- row-major `A`, `B`, `C`;
- push constants `m`, `n`, `k`;
- dispatch mapping `global x -> row`, `global y -> col`;
- low persistent register pressure with a small fixed inner unroll over `K`.

This is not the future shared-memory tiled kernel. It is a clean, source-backed scalar baseline-plus reference.

## Regeneration

```powershell
powershell -ExecutionPolicy Bypass -File internal/prometheus/native/generate_sdslv_shaders.ps1
```

That script requires a working `dxc` resolution path for regeneration, but ordinary native builds do not.

## Verification intent

M13 is considered successful when:

- SDSL-V check / VD-MIR / HLSL emission succeeds for the SGEMM source;
- DXC/SPIR-V/header generation succeeds;
- Prometheus creates and destroys the dedicated pipeline from the generated header;
- explicit benchmark and resident comparison paths can request `SDSL_SCALAR_PLUS`;
- diagnostics report it as `WIRED`;
- correctness lanes pass on representative square, odd-K, and rectangular shapes.

## Initial resident comparison snapshot

From `out/test-artifacts/prometheus_sgemm_px16_evt_report.md` after M13 wiring:

- `small_64x64x64`
  - `BASELINE_SCALAR`: `0.00944 ms`
  - `SDSL_SCALAR_PLUS`: `0.015104 ms`
  - `MEMORY_CONSERVATIVE`: `0.009152 ms`
- `square_128x128x128`
  - `BASELINE_SCALAR`: `0.01952 ms`
  - `SDSL_SCALAR_PLUS`: `0.068032 ms`
  - `MEMORY_CONSERVATIVE`: `0.0248 ms`
- `rect_255x129x65`
  - `BASELINE_SCALAR`: `0.069056 ms`
  - `SDSL_SCALAR_PLUS`: `0.253952 ms`
  - `MEMORY_CONSERVATIVE`: `0.050656 ms`

That is acceptable for M13. The milestone goal is source-backed correctness and end-to-end generation/wiring, not immediate selector promotion or performance leadership.
