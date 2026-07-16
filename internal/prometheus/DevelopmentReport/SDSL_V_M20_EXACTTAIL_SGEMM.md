# SDSL-V M20 ExactTail Reg2x2 SGEMM

M20 adds `SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32`, an explicit benchmark-only variant derived from the M17 reg2x2 SGEMM kernel.

The goal is narrow: use the M19 runtime guard `when` statement to express the full-tile steady state separately from boundary tails, so the common exact path can use direct shared-tile preloads instead of guarded-read fallback temp blocks.

## Kernel Design

- Source: `internal/prometheus/shaders/sdslv/production/sgemm/sgemm_reg2x2_tile16x16_exacttail_fp32.sdslv`
- Variant: `PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_EXACTTAIL_FP32`
- Workgroup tile: 16x16 output tile
- Workgroup shape: `numthreads(8, 8, 1)`
- K tile: 16
- Per-thread output: `reg_tile<f32, 2u, 2u>`
- Accumulator: `reg_tile<f32, 2u, 2u>`
- Metadata:
  - `tile_m = 16`
  - `tile_n = 16`
  - `tile_k = 16`
  - `outputs_per_invocation_m = 2`
  - `outputs_per_invocation_n = 2`
  - `unroll_k = 16`

The cooperative load mapping remains the M17 four-elements-per-thread mapping. The production selector is unchanged and does not choose M20.

## Exact/Tail Conditions

The output tile condition is:

```text
groupBaseRow + C.Tile.M <= params.m &&
groupBaseCol + C.Tile.N <= params.n
```

Each K tile also checks:

```text
tileBaseK + C.Tile.K <= params.k
```

So `fullTile` means A rows, B columns, C output coverage, and current K tile are all fully in bounds. This is intentionally conservative.

## Runtime `when` Structure

Inside the K tile loop, M20 uses:

```sdslv
when {
    case fullTile -> { direct A/B loads into TileA/TileB }
    else -> { guarded read ... else 0.0 tail loads }
}
```

After accumulation, it also splits stores:

```sdslv
when {
    case fullOutputTile -> { direct C stores }
    else -> { guarded writes }
}
```

## Generated HLSL Observation

Generated HLSL has the intended branch shape:

- an outer `if (fullTile)` around cooperative loads
- direct exact-path loads from `A[...]` and `B[...]`
- no guarded-read fallback temps in the exact path
- guarded fallback temp blocks only in the `else` tail path
- `if (fullOutputTile)` direct stores, with guarded stores only in the tail path
- barriers remain one after cooperative load and one after accumulation
- no source-level `when {` spelling reaches HLSL

The generated SPIR-V header records `word_count = 4418u`.

## Correctness Results

Fresh focused M20 validation passed:

- `16x16x16`
- `32x32x32`
- `8x8x8`
- `17x17x17`
- `31x29x23`
- `64x16x64`
- `16x64x64`
- `64x64x8`
- `128x128x128`

Artifacts:

- `out/test-artifacts/prometheus_sgemm_sdslv_m20_exacttail.json`
- `out/test-artifacts/prometheus_sgemm_sdslv_m20_exacttail.md`

## M17 Versus M20

Fresh focused artifact results on the RTX 3070 run:

| shape | M17 resident kernel ms | M20 resident kernel ms | M17 GFLOP/s | M20 GFLOP/s |
| --- | ---: | ---: | ---: | ---: |
| exact_16x16x16 | 0.003584 | 0.003456 | 2.28571 | 2.37037 |
| exact_32x32x32 | 0.004064 | 0.004288 | 16.126 | 15.2836 |
| small_8x8x8 | 0.005984 | 0.006496 | 0.171123 | 0.157635 |
| odd_17x17x17 | 0.009216 | 0.0096 | 1.06619 | 1.02354 |
| odd_31x29x23 | 0.01136 | 0.011744 | 3.64032 | 3.52129 |
| skinny_64x16x64 | 0.017312 | 0.017792 | 7.57116 | 7.36691 |
| wide_16x64x64 | 0.01936 | 0.02 | 6.77025 | 6.5536 |
| lowk_64x64x8 | 0.01216 | 0.01264 | 5.38947 | 5.18481 |
| medium_128x128x128 | 0.058912 | 0.060768 | 71.1961 | 69.0216 |

The focused staged wall numbers improved for M20 on every listed shape, but resident kernel time is the cleaner kernel-to-kernel signal and is mostly neutral/slower except `exact_16x16x16`.

EVT explicit comparison shows the same mixed result:

- `small_64x64x64`: M20 staged kernel `0.005472 ms` vs M17 `0.0056 ms`
- `square_128x128x128`: M20 `0.03584 ms` vs M17 `0.033024 ms`
- `square_256x256x256`: M20 `0.328672 ms` vs M17 `0.310624 ms`
- `square_512x512x512`: M20 `3.04842 ms` vs M17 `3.04698 ms`
- `wide_64x1024x1024`: M20 `1.5608 ms` vs M17 `1.5655 ms`
- `lowk_1024x1024x64`: M20 `1.46646 ms` vs M17 `1.46794 ms`

## Conclusion

M20 succeeds as a language/kernel experiment:

- the exact path is source-expressed with runtime guard `when`
- generated HLSL removes guarded-read branch noise from the exact path
- tail safety is preserved
- direct-store fast path is practical
- correctness passes the exact, small, odd-tail, skinny/wide, low-K, and medium shapes
- explicit benchmark/report integration distinguishes M17 and M20

M20 does not establish a broad performance win. The branch removal is real, but kernel time is mostly neutral/slower in the focused correctness artifact and only narrowly favorable on a few EVT rows.

## Recommendation

Do not production-promote M20 and do not retune the selector from this result.

The next useful step is still geometry/reuse work from the M18 recommendation, especially a `16x32/32x16` style kernel that increases useful math per shared-memory phase. Compiler-side guarded-access lowering may still be useful, but M20 shows that shader-level full/tail splitting alone is not enough to move the main performance frontier.

Follow-up:

- M24 keeps the same geometry and exact/tail behavior but rewrites the source around M21-M23 `board` / `flow` / `state`.
- See `internal/prometheus/DevelopmentReport/SDSL_V_M24_FLOW_BOARD_SGEMM.md`.
