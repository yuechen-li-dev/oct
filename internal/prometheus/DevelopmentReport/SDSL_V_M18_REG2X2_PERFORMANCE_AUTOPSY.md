# SDSL-V M18 Reg2x2 Performance Autopsy

M18 is an autopsy milestone for `SDSL_REG2X2_TILE16X16_FP32`.

M17 proved that SDSL-V can express, compile, and correctly execute a source-backed register-blocked 2x2 SGEMM kernel. M18 answers the harder question: is it actually fast, what is slow, and what should we try next.

## Fresh Artifacts

- `out/test-artifacts/prometheus_sgemm_sdslv_m18_reg2x2_autopsy.json`
- `out/test-artifacts/prometheus_sgemm_sdslv_m18_reg2x2_autopsy.md`
- `out/test-artifacts/prometheus_sgemm_px16_evt_results.json`
- `out/test-artifacts/prometheus_sgemm_px16_resident_failure_matrix.json`
- `out/test-artifacts/prometheus_sgemm_sdslv_m17_reg2x2.json`
- `out/sdslv/sgemm_reg2x2_tile16x16_fp32_m18.hlsl`
- `out/sdslv/sgemm_reg2x2_tile16x16_fp32_m18.spv`
- `out/sdslv/sgemm_reg2x2_tile16x16_fp32_m18.spvasm`

## Benchmark Summary

Correctness remains good for the explicit M17 standalone validation shapes. The fresh M17 artifact still passes:

- `16x16x16`
- `8x8x8`
- `17x17x17`
- `31x29x23`
- `64x16x64`
- `16x64x64`
- `64x64x8`
- `128x128x128`

On the fresh EVT explicit comparison shapes, kernel-time results are mixed but no longer ambiguous:

- `SDSL_REG2X2_TILE16X16_FP32` beats `SDSL_TILE16X16_SHARED_FP32` on all 8 fresh EVT comparison shapes.
- It also beats `MEMORY_CONSERVATIVE` on all 8 fresh staged explicit comparison shapes.
- It does not beat `BASELINE_SCALAR` universally. Fresh staged kernel-time wins are only:
  - `square_512x512x512`
  - `skinny_1024x64x1024`
  - `lowk_1024x1024x64`
- Fresh resident kernel-time wins survive on:
  - `skinny_1024x64x1024`
  - `lowk_1024x1024x64`
- Fresh resident results put `BASELINE_SCALAR` slightly ahead again on `square_512x512x512`.

Representative fresh staged kernel numbers:

- `square_512x512x512`
  - `BASELINE_SCALAR`: `1.05616 ms`
  - `SDSL_TILE16X16_SHARED_FP32`: `0.513696 ms`
  - `SDSL_REG2X2_TILE16X16_FP32`: `0.408608 ms`
- `skinny_1024x64x1024`
  - `BASELINE_SCALAR`: `0.477312 ms`
  - `MEMORY_CONSERVATIVE`: `0.905856 ms`
  - `SDSL_REG2X2_TILE16X16_FP32`: `0.374112 ms`
- `lowk_1024x1024x64`
  - `BASELINE_SCALAR`: `0.887392 ms`
  - `SDSL_TILE16X16_SHARED_FP32`: `0.23088 ms`
  - `SDSL_REG2X2_TILE16X16_FP32`: `0.176896 ms`
- `rect_255x129x65`
  - `SDSL_SCALAR_PLUS`: `0.007584 ms`
  - `BASELINE_SCALAR`: `0.010048 ms`
  - `SDSL_REG2X2_TILE16X16_FP32`: `0.011968 ms`

The honest bottom line is:

- M17 moved the source-backed tiled path forward materially versus the earlier SDSL shared-tile kernel.
- M17 also moved ahead of `MEMORY_CONSERVATIVE` in the fresh explicit kernel-time comparison set.
- M17 did not establish a new universal best kernel. `BASELINE_SCALAR` still wins several shapes, especially smaller and more square ones.

## Wall Time Versus Kernel Time

The main end-to-end wall bottleneck is not the kernel itself. Fresh staged EVT decomposition shows readback and sync dominate wall time on larger shapes.

Examples:

- `square_512x512x512`, `SDSL_REG2X2_TILE16X16_FP32`
  - kernel: `0.408608 ms`
  - staged wall: `4.5485 ms`
  - readback: `3.5002 ms`
  - sync wait: `0.5821 ms`
- `lowk_1024x1024x64`, `SDSL_REG2X2_TILE16X16_FP32`
  - kernel: `0.176896 ms`
  - staged wall: `17.6575 ms`
  - readback: `16.9901 ms`
  - sync wait: `0.4178 ms`
- `wide_64x1024x1024`, `SDSL_REG2X2_TILE16X16_FP32`
  - kernel: `0.884448 ms`
  - staged wall: `4.2747 ms`
  - sync wait: `2.7303 ms`
  - readback: `0.8876 ms`

So:

- staged wall numbers are useful for path diagnosis,
- resident kernel numbers are better for kernel-to-kernel comparison,
- correctness is not polluting benchmark timing because the performance lane still keeps validation separate.

There is still some staged-versus-resident measurement spread on a few variants, so not every row should be over-interpreted to the third decimal place. But the broad trend is stable enough for diagnosis.

## Generated HLSL Observations

Fresh HLSL confirms the intended structure:

- workgroup tiles:
  - `groupshared float TileA[16 * 16];`
  - `groupshared float TileB[16 * 16];`
- explicit accumulator:
  - `float Acc[4];`
- guarded loads:
  - 4 `aValue*` temps
  - 4 `bValue*` temps
- reduction loop:
  - `[unroll]`
  - `for (uint kk = 0u; kk < 16u; kk += 1)`
- barriers:
  - one `GroupMemoryBarrierWithGroupSync()` after cooperative loads
  - one `GroupMemoryBarrierWithGroupSync()` before the next tile
- output stores:
  - 4 guarded scalar stores

Structural assessment:

1. `reg_tile<2,2>` does lower to a local HLSL array (`float Acc[4]`), but all accesses are constant-indexed after `comptime for`.
2. Guarded reads stay outside the inner accumulation loop.
3. Tails still generate 8 branchy guarded load blocks per K tile, even for exact-shape work where the branch is predictably taken.
4. Cooperative load mapping is regular: each thread loads exactly 4 A values and 4 B values into a contiguous 16x16 tile mapping.
5. Arithmetic per tile is still only 2x2 outputs per thread with 2 barriers per K tile.

## SPIR-V Observations

Fresh disassembly is available and removes one major suspicion.

Positive findings:

- descriptor bindings remain storage-buffer based:
  - `A` binding 0
  - `B` binding 1
  - `C` binding 2
- push constants remain 12 bytes with `m/n/k` offsets `0/4/8`
- workgroup tiles remain `Workgroup` arrays of 256 floats each
- there is no `OpTypeImage Buffer` regression
- there is only one `OpLoopMerge`, for the outer runtime tile loop
- there are 2 `OpControlBarrier` instructions, matching the HLSL barriers

Most important backend finding:

- the accumulator does not survive into SPIR-V as a `Function` storage array
- there is no function-local `float[4]` style array object in the disassembly
- instead, the accumulator values are scalarized into separate float SSA/phi values

That means the current evidence does not support the hypothesis that `reg_tile` scalarization failure is the main reason this kernel underperforms.

Less positive findings:

- the guarded load lowering still creates a long branchy prelude before the shared-tile stores
- the SPIR-V contains many workgroup `OpAccessChain` uses through `TileA` and `TileB`
- the inner K loop is effectively unrolled into repeated workgroup accesses rather than a compact loop

So the backend is structurally reasonable, but still noisy and shared-memory heavy.

## Metadata And Dispatch Sanity

Fresh sanity checks are consistent:

- generated metadata remains:
  - `numthreads = 8,8,1`
  - `tile_m = 16`
  - `tile_n = 16`
  - `tile_k = 16`
  - `outputs_per_invocation_m = 2`
  - `outputs_per_invocation_n = 2`
- dispatch geometry still comes from metadata-derived logical coverage, not raw thread count
- explicit runtime binding still maps the M17 variant to the dedicated reg2x2 pipeline
- production selector behavior remains unchanged

Important selector fact:

- `reactor_judgment_engine.c` still treats production-valid occupancy variants as the range ending at `PROM_OCCUPANCY_KERNEL_VARIANT_SDSL_TILE16X16_SHARED_FP32`
- the explicit `SDSL_REG2X2_TILE16X16_FP32` enum value exists, but it is not in the production selector validity range

So M17 is still benchmark-only, as intended.

## Ranked Hypotheses

Rank 1: the current `16x16x16` workgroup tile with `2x2` outputs per thread still does too little useful math per shared-memory phase on the shapes where `BASELINE_SCALAR` wins.

Evidence:

- reg2x2 beats the earlier SDSL shared-tile kernel on every fresh EVT comparison shape, so register blocking helped.
- despite that, baseline still wins fresh kernel-time comparisons on `64x64x64`, `128x128x128`, `256x256x256`, `64x1024x1024`, and `255x129x65`.
- the kernel still pays 2 barriers per K tile and a full shared-memory load/store phase for only 4 outputs per thread.

Why it matters:

- this points to tile geometry and arithmetic-per-barrier limits, not a total backend collapse.

Suggested M19:

- prioritize the next kernel experiment around tile geometry and reuse, not scalarization.
- the strongest candidate is an asymmetric next kernel in the `16x32/32x16` family, with special interest in the wide-shape gap.

Rank 2: staged wall time is dominated by host-visible readback/sync, so wall_ms is not a trustworthy kernel ranking metric.

Evidence:

- readback and sync dwarf kernel time on the large staged rows.
- resident kernel results and staged kernel results are much more informative than staged wall time.

Why it matters:

- it is easy to tell the wrong optimization story if wall time is treated as the main kernel score.

Suggested M19:

- keep resident kernel timing and staged GPU timestamp timing as the primary optimization KPI.
- treat staged wall only as a path/overhead diagnostic.

Rank 3: guarded-read lowering still bloats the pre-load section with per-element branches and fallback temps.

Evidence:

- HLSL still emits 8 guarded load temp blocks per tile.
- SPIR-V still shows the branch-heavy guarded-load prelude ahead of the workgroup stores.

Why it matters:

- even if the branch is predictable, it is repeated for every K tile and every thread.
- this especially hurts the “full tile” steady state where the fallback path is almost never needed.

Suggested M19:

- if the next kernel milestone is compiler-side rather than kernel-side, choose guarded-access lowering improvement with a full-tile fast path.
- keep the guarded-read versus guarded-write semantic distinction exactly as fixed in M17.

Rank 4: cooperative tile loads are not the main failure.

Evidence:

- each thread loads exactly 4 A and 4 B values.
- the mapping is contiguous and structurally simple.
- the kernel can already win large low-K and tall/skinny cases, which would be unlikely if cooperative loading were fundamentally broken.

Why it matters:

- it lowers the priority of spending M19 on a load-lane remap first.

Suggested M19:

- do not make cooperative load mapping the first M19 bet.

Rank 5: reg_tile scalarization failure is not supported by the fresh backend evidence.

Evidence:

- SPIR-V does not preserve the accumulator as a function-local array.
- accumulator state appears scalarized into SSA/phi form.

Why it matters:

- this removes one of the main fears from M17.

Suggested M19:

- do not spend M19 on reg_tile scalarization first.

## Recommended M19

M19 should be a next-kernel geometry milestone, not a scalarization milestone.

Recommended direction:

- first choice: `16x32/32x16` asymmetric tiled reg-blocked kernel work, aimed at increasing useful work per shared-memory phase and probing the current wide-shape weakness
- second choice, if compiler work is preferred before another kernel: guarded-access lowering improvement with a full-tile fast path

Not recommended as the first M19 step:

- reg_tile scalarization work
- production selector retuning
- dispatch policy changes

## Follow-up: M20 ExactTail

M20 followed the second M18 option by using M19 runtime guard `when` for a shader-level exact/tail split:

- `internal/prometheus/DevelopmentReport/SDSL_V_M20_EXACTTAIL_SGEMM.md`

The result confirms that the exact path can remove guarded-read branch noise in generated HLSL, but the performance result is mixed and does not justify production promotion or selector retuning.

## Honest Conclusion

M17 did move performance forward, but only in a bounded way.

- Yes: it produced a source-backed reg-blocked kernel that clearly outperforms the older SDSL shared-tile kernel.
- Yes: it also beats `MEMORY_CONSERVATIVE` across the fresh staged explicit comparison set.
- No: it is not the new universal winner.
- No: it does not justify production promotion or selector retuning.

That is a useful outcome. The kernel is real, the generated backend is structurally sane, the guarded-read semantic fix held, and the next optimization question is now narrower and evidence-backed.
