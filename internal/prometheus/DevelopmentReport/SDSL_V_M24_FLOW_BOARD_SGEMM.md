# SDSL-V M24 Flow-Board SGEMM

M24 adds `SDSL_REG2X2_TILE16X16_FLOWBOARD_FP32`, an explicit benchmark-only Prometheus SGEMM variant that keeps the M20 exact-tail kernel geometry and dispatch metadata but rewrites the source around M21-M23 `board` / `flow` / `state`.

## Kernel

- Source: `internal/prometheus/shaders/sdslv/sgemm_reg2x2_tile16x16_flowboard_fp32.sdslv`
- Variant: `PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_FLOWBOARD_FP32`
- Baseline behavior source: `internal/prometheus/shaders/sdslv/sgemm_reg2x2_tile16x16_exacttail_fp32.sdslv`
- Explicit-only wiring: production selector remains unchanged

Preserved from M20:

- `numthreads(8, 8, 1)`
- `tile_m = 16`, `tile_n = 16`, `tile_k = 16`
- `outputs_per_invocation_m = 2`, `outputs_per_invocation_n = 2`
- `reg_tile<f32,2u,2u>` accumulator shape
- cooperative four-elements-per-thread tile load mapping
- K-tile loop structure
- barrier order
- exact/tail split
- push-constant ABI
- bindings and generated metadata

## Source Shape

M24 introduces two board types:

```sdslv
board LoadCoord {
    linear: u32;
    tileRow: u32;
    tileCol: u32;
    aRow: u32;
    aCol: u32;
    bRow: u32;
    bCol: u32;
    validA: bool;
    validB: bool;
}

board StoreCoord {
    oi: u32;
    oj: u32;
    row: u32;
    col: u32;
    valid: bool;
}
```

The K-tile body is expressed as a bounded sequential `flow TileLoad` with these states:

1. `LoadLanes`
2. `SyncAfterLoad`
3. `Accumulate`
4. `SyncBeforeNextTile`

Stores are expressed as a second bounded `flow StoreOutput` with a mutable `Store` board.

This keeps the execution model honest:

- no persistent Octomata state
- no `goto`
- no `remember` / `resume` / `suspend`
- no `when policy`
- states execute once in source order and lower to ordinary structured statements

## Readability Comparison

Line counts:

- M20 exact-tail source: 252 lines
- M24 flow-board source: 225 lines

Repeated derived expression count from the exact-tail source:

- `localThreadLinear * 4u + ...`: 80 occurrences in M20, 1 occurrence in M24
- `groupBaseRow + ((localThreadLinear * 4u + ...) / C.Tile.K)`: 12 occurrences in M20, replaced by `Load.aRow`
- `tileBaseK + ((localThreadLinear * 4u + ...) % C.Tile.K)`: 24 occurrences in M20, replaced by `Load.aCol` / `Load.bRow`
- `baseRow + oi` and `baseCol + oj`: 6 combined occurrences in M20, replaced by `Store.row` / `Store.col`

Qualitative result:

- M20 is behaviorally correct but reads like manually expanded index algebra.
- M24 reads more like bounded GPU phase pseudocode.
- Load/store validity logic now lives on one named execution surface instead of being restated at every access.

## Generated Backend Observations

Generated HLSL observations:

- HLSL emits local `struct LoadCoord` and `struct StoreCoord`.
- `flow` / `state` syntax does not survive into HLSL.
- The exact path still lowers to direct loads under `if (fullTile)`.
- The tail path still lowers to guarded fallback-zero loads.
- Barrier order is unchanged: one `GroupMemoryBarrierWithGroupSync()` after cooperative load and one after accumulation.
- The same 2x2 accumulation shape is preserved.
- The same four output stores are preserved behind `if (fullOutputTile)` or guarded tail writes.

Important backend note:

- A first draft that directly assigned `TileA[...] = read ... when ... else 0.0` inside the flow state mis-lowered in HLSL tail code and left stale shared-tile values when a guard was false.
- The final M24 shader keeps the board-based coordinates but materializes explicit `aValue` / `bValue` fallback-zero temporaries before writing `TileA` / `TileB`.
- No new language feature was required; this was a source-level correctness fix within the existing M21-M23 surface.

SPIR-V sanity:

- M20 header word count: `4418`
- M24 header word count: `4299`
- M24 preserves the same metadata values as M20:
  - `numthreads_x = 8`
  - `outputs_per_invocation_m = 2`
  - `tile_m = 16`
  - `tile_n = 16`
  - `tile_k = 16`
  - `unroll_k = 16`
- Disassembled SPIR-V shows workgroup variables only for `TileA` and `TileB`; the board structs do not survive as separate workgroup/storage resources.

That is the desired backend shape: board state becomes ordinary function-local execution state instead of memory-like shader storage.

## Correctness

Focused M24 correctness/comparison fact:

- `out\prometheus\native\marionette_tests.exe PrometheusSgemmM24SdslReg2x2FlowBoard`

Artifact outputs:

- `out/test-artifacts/prometheus_sgemm_sdslv_m24_flowboard.json`
- `out/test-artifacts/prometheus_sgemm_sdslv_m24_flowboard.md`

Validated shapes:

- `16x16x16`
- `32x32x32`
- `8x8x8`
- `17x17x17`
- `31x29x23`
- `64x16x64`
- `16x64x64`
- `64x64x8`
- `128x128x128`

The odd-tail shapes that initially exposed stale shared-tile reuse:

- `17x17x17`
- `31x29x23`

now pass in both staged and resident explicit validation.

## Performance

Focused M20 vs M24 artifact highlights:

| shape | M20 resident ms | M24 resident ms | M20/M24 ratio |
| --- | ---: | ---: | ---: |
| exact_16x16x16 | 0.003392 | 0.003616 | 0.938 |
| exact_32x32x32 | 0.004416 | 0.004224 | 1.045 |
| small_8x8x8 | 0.006176 | 0.006624 | 0.932 |
| odd_17x17x17 | 0.008448 | 0.011008 | 0.767 |
| odd_31x29x23 | 0.011648 | 0.01296 | 0.899 |
| skinny_64x16x64 | 0.01584 | 0.019008 | 0.833 |
| wide_16x64x64 | 0.018368 | 0.019232 | 0.955 |
| lowk_64x64x8 | 0.010432 | 0.012448 | 0.838 |
| medium_128x128x128 | 0.053056 | 0.055552 | 0.955 |

Focused interpretation:

- M24 is mostly neutral/slower than M20 on the small and awkward validation shapes.
- The refactor should not be described as a focused correctness-lane speedup.

EVT resident explicit comparison highlights from `out/test-artifacts/prometheus_sgemm_px16_evt_report.md`:

- `small_64x64x64`: M20 `0.01088 ms`, M24 `0.011936 ms`
- `square_128x128x128`: M20 `0.072608 ms`, M24 `0.08288 ms`
- `square_256x256x256`: M20 `0.43232 ms`, M24 `0.432672 ms`
- `square_512x512x512`: M20 `3.04595 ms`, M24 `2.84358 ms`
- `skinny_1024x64x1024`: M20 `1.56512 ms`, M24 `1.55459 ms`
- `wide_64x1024x1024`: M20 `1.5625 ms`, M24 `1.55018 ms`
- `lowk_1024x1024x64`: M20 `1.47018 ms`, M24 `1.47056 ms`

EVT interpretation:

- M24 is clearly not a forced regression everywhere.
- Larger steady-state resident shapes are roughly neutral, with a few slightly favorable rows.
- The honest summary is still "source clarity win, performance roughly neutral to mildly shape-dependent."

## Recommendation

M24 succeeds as a real-kernel validation of M21-M23:

- the new source shape is materially clearer than M20;
- exact-tail behavior and metadata remain intact;
- generated HLSL/SPIR-V remain structurally sane;
- correctness passes after restoring explicit fallback-zero tail values;
- explicit EVT and focused artifacts now include the flow-board kernel.

M24 should remain explicit benchmark-only for now. The abstraction proved useful for authoring, but the performance signal is mixed and does not justify selector retuning or production promotion.
