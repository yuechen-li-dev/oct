# SDSL-V M17 Register-Blocked SGEMM

M17 adds `SDSL_REG2X2_TILE16X16_FP32`, the first explicit benchmark-only source-backed 2x2 register-blocked SGEMM kernel for Prometheus.

## Design

- Workgroup tile: 16x16 output tile with `Tile.K = 16`
- Workgroup shape: `numthreads(8, 8, 1)`
- Per-thread output: `reg_tile<f32, 2u, 2u>`
- Source abstractions used:
  - `matrix_view<f32>` for A/B/C
  - `tile<f32, R, C>` for shared tiles
  - `reg_tile<f32, 2u, 2u>` for accumulators
  - `comptime for` for register-tile structure and cooperative load lanes
  - guarded `read ... when ... else 0.0`
  - guarded `write ... when ...`

Each 64-thread workgroup cooperatively loads the full 16x16 A tile and 16x16 B tile with four elements per thread, then each thread accumulates a 2x2 block into a structured register tile.

## Metadata and Wiring

- Dispatch geometry comes from generated SDSL-V metadata rather than host hardcoding.
- Generated metadata records:
  - threads: 8x8x1
  - outputs per invocation: 2x2
  - tile: 16x16x16
  - unroll_k: 16
- Runtime wiring is explicit benchmark/report only.
- Production selector policy is unchanged and does not choose this variant by default.

## Validation Scope

Focused correctness coverage for the explicit M17 variant targets:

- 16x16x16
- 8x8x8
- 17x17x17
- 31x29x23
- 64x16x64
- 16x64x64
- 64x64x8
- 128x128x128

## Known Limits

- This milestone is about correctness, metadata consistency, and iteration speed first.
- Performance may still trail older hand-written kernels or memory-conservative paths.
- If the kernel underperforms, likely causes include barrier cost, shared-memory access pattern, reg-tile non-scalarization, or register pressure.

## Follow-up

Performance diagnosis, generated-code inspection, and the M19 recommendation now live in:

- `internal/prometheus/DevelopmentReport/SDSL_V_M18_REG2X2_PERFORMANCE_AUTOPSY.md`
