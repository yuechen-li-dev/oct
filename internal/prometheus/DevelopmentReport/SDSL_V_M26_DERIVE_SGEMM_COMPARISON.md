# SDSL-V M26 Derive SGEMM Comparison

M26 adds `SDSL_REG2X2_TILE16X16_DERIVE_FP32`, an explicit benchmark-only Prometheus SGEMM variant that keeps the M20 exact-tail kernel geometry and exact/tail behavior, but rewrites load/store coordinate derivation around ordered immutable `derive`.

## Kernel

- Source: `internal/prometheus/shaders/sdslv/sgemm_reg2x2_tile16x16_derive_fp32.sdslv`
- Variant: `PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_REG2X2_TILE16X16_DERIVE_FP32`
- Baselines:
  - `internal/prometheus/shaders/sdslv/sgemm_reg2x2_tile16x16_exacttail_fp32.sdslv`
  - `internal/prometheus/shaders/sdslv/sgemm_reg2x2_tile16x16_flowboard_fp32.sdslv`
- Explicit-only wiring: production selector remains unchanged

Preserved from M20/M24:

- `numthreads(8, 8, 1)`
- `tile_m = 16`, `tile_n = 16`, `tile_k = 16`
- `outputs_per_invocation_m = 2`, `outputs_per_invocation_n = 2`
- `reg_tile<f32,2u,2u>` accumulator shape
- cooperative four-elements-per-thread load mapping
- runtime K-tile loop
- barrier order
- accumulation order
- exact/tail split
- push-constant ABI
- bindings and generated dispatch metadata

## Source Shape

M26 uses immutable records:

```sdslv
record LoadCoord {
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

record StoreCoord {
    row: u32;
    col: u32;
    valid: bool;
}
```

The K-tile loop stays direct and linear:

- `derive` names the dependent load facts once per lane
- accumulation stays on the plain M20 direct loop shape
- stores use a second `derive` bundle for per-output bounds facts
- no mutable board is used
- no `flow` / `state` is used

Important correctness note:

- The first draft used direct guarded `read ... when ... else 0.0` assignments into `TileA` / `TileB`.
- Current HLSL lowering still turns that tail form into conditional shared-memory stores without an `else`, which can leave stale workgroup values alive.
- Final M26 keeps `derive` for coordinates but materializes explicit `aValue` / `bValue` fallback-zero temporaries before writing shared tiles.
- This is the same narrow correctness discipline M24 needed; it is not new language-surface work.

## Readability Comparison

Fresh source metrics from `out/test-artifacts/prometheus_sgemm_sdslv_m26_derive.json`:

| source | lines | `localThreadLinear * 4u +` | flows | states | board decls | derives | mutable coord assigns |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| M20 exact-tail | 252 | 80 | 0 | 0 | 0 | 0 | 0 |
| M24 flow-board | 225 | 1 | 2 | 5 | 2 | 0 | 53 |
| M26 derive | 188 | 1 | 0 | 0 | 0 | 2 | 0 |

Anti-ceremony review:

1. Repeated code that disappeared:
   - M26 removes the expanded M20 load/store coordinate algebra.
   - The per-lane `linear`, `tileRow`, `tileCol`, `aRow`, `aCol`, `bRow`, `bCol`, and validity facts are each written once.
2. Invariants that became clearer:
   - later coordinate facts explicitly depend on earlier facts inside one ordered immutable construction
   - load/store validity remains attached to the same nominal bundle as the coordinates it guards
3. New concepts the reader must understand:
   - M20: almost none, but the coordinate repetition is heavy
   - M24: `board`, `flow`, `state`, mutable board updates, and phase structure
   - M26: `record` plus ordered immutable `derive`
4. Helper-function comparison:
   - a helper would remove repetition, but it would split the coordinate logic away from the hot-path code and hide source-order dependence across multiple returned facts
5. Does the abstraction pay rent?
   - for coordinate-heavy linear code, yes: M26 is materially shorter than M20 and far less ceremonial than M24

Honest readability conclusion:

- M20 has the least vocabulary but the most repeated machinery.
- M24 has the strongest phase/story structure, but it introduces the most ceremony for what is still a mostly linear kernel.
- M26 is the clearest authoring surface of the three for this exact workload.

## Generated Backend Comparison

Fresh generated HLSL line counts:

- M20 exact-tail: 152
- M24 flow-board: 284
- M26 derive: 322

Fresh SPIR-V header word counts:

- M20 exact-tail: `4418`
- M24 flow-board: `4299`
- M26 derive: `4297`

Observed backend structure:

- M26 HLSL emits deterministic derive temporaries such as `__sdslv_derive_linear_*`, `__sdslv_derive_tileRow_*`, and record locals like `LoadCoord load__ct0`.
- `derive` lowers in source order exactly as intended.
- DXC does not keep the source-level `derive` spelling, but the temp sequence is easy to inspect.
- Barrier count/order is unchanged: one `GroupMemoryBarrierWithGroupSync()` after load and one after accumulation.
- Resource bindings, push constants, dispatch metadata, and workgroup shared tiles remain unchanged.
- No new storage-buffer type or ABI regression appeared.
- No extra phase control structure was introduced beyond the per-lane derive temps.

Important backend caveat:

- Source clarity did not translate into smaller generated HLSL.
- M26 HLSL is larger than both M20 and M24 because the derive temps plus record materialization are explicit in the emitted source.
- SPIR-V remains sane and slightly smaller than M24, but smaller SPIR-V alone is not evidence of faster execution.

## Correctness

Focused fact:

- `out\prometheus\native\marionette_tests.exe PrometheusSgemmM26SdslReg2x2Derive`

Artifacts:

- `out/test-artifacts/prometheus_sgemm_sdslv_m26_derive.json`
- `out/test-artifacts/prometheus_sgemm_sdslv_m26_derive.md`

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

The odd-tail failures in the first draft (`17x17x17`, `31x29x23`) disappeared once the tail path restored explicit fallback-zero temps before shared-tile writes.

## Focused Performance

Focused resident comparison from the fresh M26 artifact:

| shape | M20 ms | M24 ms | M26 ms | M20/M26 | M24/M26 |
| --- | ---: | ---: | ---: | ---: | ---: |
| exact_16x16x16 | 0.003232 | 0.002816 | 0.00288 | 1.122 | 0.978 |
| exact_32x32x32 | 0.0128 | 0.008704 | 0.009152 | 1.399 | 0.951 |
| small_8x8x8 | 0.005984 | 0.008416 | 0.006208 | 0.964 | 1.356 |
| odd_17x17x17 | 0.00784 | 0.00864 | 0.0096 | 0.817 | 0.900 |
| odd_31x29x23 | 0.009984 | 0.011648 | 0.011616 | 0.860 | 1.003 |
| skinny_64x16x64 | 0.014432 | 0.013632 | 0.01312 | 1.100 | 1.039 |
| wide_16x64x64 | 0.022432 | 0.023424 | 0.018368 | 1.221 | 1.275 |
| lowk_64x64x8 | 0.005824 | 0.005696 | 0.005632 | 1.034 | 1.011 |
| medium_128x128x128 | 0.027072 | 0.028096 | 0.037664 | 0.719 | 0.746 |

Focused interpretation:

- M26 is not a uniform regression.
- It wins or roughly ties on several exact/small/rectangular/low-K focused shapes.
- It loses on awkward tails and on the focused `128^3` medium shape.
- The honest focused result is mixed, not cleanly neutral.

## EVT Explicit Comparison

Selected resident explicit rows from `out/test-artifacts/prometheus_sgemm_px16_evt_report.md`:

| shape | M20 ms | M24 ms | M26 ms |
| --- | ---: | ---: | ---: |
| small_64x64x64 | 0.017312 | 0.017536 | 0.010336 |
| square_128x128x128 | 0.026464 | 0.025792 | 0.034752 |
| square_256x256x256 | 0.113696 | 0.125216 | 0.132256 |
| square_512x512x512 | 0.739744 | 0.742752 | 1.60266 |
| skinny_1024x64x1024 | 0.486112 | 0.474304 | 1.31574 |
| wide_64x1024x1024 | 0.669856 | 1.35085 | 0.528864 |
| lowk_1024x1024x64 | 0.561024 | 0.592128 | 0.547968 |

EVT interpretation:

- M26 can be very strong on some wide or low-K rows.
- M26 is clearly worse than M20/M24 on several larger square or skinny rows.
- The performance result is therefore shape-dependent enough that M26 should not be described as generally neutral.

## Recommendation

M26 succeeds as an authoring-style comparison milestone:

- `derive` materially improves source clarity for linear coordinate-heavy kernels
- repeated coordinate expressions are sharply reduced
- generated HLSL/SPIR-V remain sane and ABI-equivalent
- correctness passes after restoring explicit fallback-zero tail temps

But M26 does **not** earn an unconditional “preferred default” recommendation yet.

Current guidance:

- use direct code for tiny/simple kernels
- use `derive` for linear dependent fact bundles where source clarity matters and performance evidence is acceptable for that kernel family
- use `flow` / `board` / `state` when there is real phase-local mutable scratch or phase grouping to express

Selector and production recommendation:

- keep M26 explicit benchmark-only
- do not retune selector scoring
- do not production-promote M26 from this milestone alone

The authoring recommendation is positive. The production-performance recommendation remains cautious.
