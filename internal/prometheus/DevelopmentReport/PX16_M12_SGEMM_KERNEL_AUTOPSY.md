# Px16 M12 SGEMM Kernel Variant Autopsy

Status: analysis-only milestone. No production dispatch authority, selector scores, P15 behavior, FFT/P16 work, benchmark semantics, or SGEMM runtime behavior were changed.

## Inputs Inspected

- `internal/prometheus/native/reactor_vulkan_sgemm.c`
- `internal/prometheus/native/reactor_vulkan_memory_conservative.comp`
- `internal/prometheus/native/reactor_vulkan_tiled_spirv.h`
- `internal/prometheus/native/reactor_vulkan_srt_2accum_k_spirv.h`
- `internal/prometheus/native/reactor_vulkan_b2x2_row_major_biased_spirv.h`
- `internal/prometheus/native/reactor_vulkan_a2x4_row_biased_accum8_spirv.h`
- `internal/prometheus/native/reactor_vulkan_memory_conservative_spirv.h`
- `internal/prometheus/native/Marionette/reactor_px16_evt_benchmark_tests.cpp`
- `out/test-artifacts/prometheus_sgemm_px16_evt_report.md`
- `out/test-artifacts/prometheus_sgemm_px16_evt_results.json`
- `docs/PROMETHEUS_SGEMM_PX16_EVT.md`
- `Experiments/PrometheusSgemmAlgorithmLab/M49/REPORT.md`
- `internal/prometheus/native/README memory conservative.md`

Note: the requested `README_memory_conservative.md` spelling was not present. The repo has the same note under `internal/prometheus/native/README memory conservative.md`.

Temporary SPIR-V autopsy files were generated under `out/px16_m12_spirv_autopsy/` using `spirv-dis` and `spirv-cross`. They are derived scratch artifacts and were not source changes.

## Kernel Inventory

| Variant | Source file | Generated header | Pipeline field | Workgroup size | Outputs per invocation | Tile shape | Uses shared memory? | Barriers? | Accumulators per thread | Bounds checks | Notes |
| --- | --- | --- | --- | --- | ---: | --- | --- | --- | ---: | --- | --- |
| `BASELINE_SCALAR` | Inline SPIR-V comment/source summary in `reactor_vulkan_sgemm.c`; no standalone `.comp` | none; inline `k_prom_sgemm_spirv[]` | `pipeline` | 8x8x1 | 1 | 1x1 output, full K loop | No | No | 1 | Early row/col return | Naive row-major SGEMM. This is the resident comparison baseline and often beats the named tiled variants. |
| `SMALL_REGISTER_TILE` | No readable checked-in `.comp`/HLSL/Slang source | `reactor_vulkan_srt_2accum_k_spirv.h` | `srt_2accum_k_pipeline` | 8x8x1 | 1 | 1x1 output, K stepped by 2 | No | No | 2 | Early row/col return plus tail K check | Name is accurate for register tiling, not output tiling. Uses two independent accumulators for K ILP. |
| `BALANCED_2X2_ACCUM4` | No readable checked-in `.comp`/HLSL/Slang source | `reactor_vulkan_b2x2_row_major_biased_spirv.h` | `b2x2_row_major_biased_pipeline` | 8x8x1 | up to 4 | 2x2 output per invocation | No | No | 4 | Per-K A/B bounds checks and per-output store checks | Host dispatch is not scaled for 2x2 output footprint, so roughly 3/4 of invocations are out-of-range work on divisible shapes. |
| `AGGRESSIVE_4X4_ACCUM8` | No readable checked-in `.comp`/HLSL/Slang source | `reactor_vulkan_a2x4_row_biased_accum8_spirv.h` | `a2x4_row_biased_accum8_pipeline` | 8x8x1 | up to 8 | Actual SPIR-V is 2x4 output, not 4x4 | No | No | 8 | Per-K A/B bounds checks and per-output store checks | Variant name and path id say aggressive 4x4, but generated code is A2x4 row-biased accum8. Host dispatch is not scaled for 2x4 output footprint, so roughly 7/8 of invocations are out-of-range work on divisible shapes. |
| `MEMORY_CONSERVATIVE` | `reactor_vulkan_memory_conservative.comp` | `reactor_vulkan_memory_conservative_spirv.h` | `memory_conservative_pipeline` | 8x8x1 | 1 | 1x1 output, K unrolled by 8 plus scalar tail | No | No | 1 | Early row/col return plus scalar K tail loop | Only current SGEMM variant with checked-in readable shader source. Built from GLSL with `glslangValidator`, `spirv-val`, and targeted `spirv-opt` per the local README note. |

Additional non-inventory note: `reactor_vulkan_tiled_spirv.h` still defines a legacy shared-memory 8x8 tiled pipeline (`tiled_pipeline`). It is the fallback for tiled compute when a specific wired variant is not selected. It is not one of the five requested explicit occupancy variants, but it matters as historical evidence: it uses `shared float tileA[8][8]`, `shared float tileB[8][8]`, and two barriers per K tile.

## Static Kernel Analysis

### BASELINE_SCALAR

- Global ID mapping: `global_x -> row`, `global_y -> col`.
- Work per invocation: one `C[row, col]`.
- A loads: `A[row * k + kk]`; contiguous along K within one invocation, but across neighboring row lanes A is stride-K if the subgroup maps across `x`.
- B loads: `B[kk * n + col]`; coalesces well when neighboring lanes vary in `y`/column, poorly when lanes vary in `x`/row.
- Global loads per output: `2 * K` scalar loads.
- Stores per output: one scalar store.
- Shared memory/barriers: none.
- Register pressure: low, one accumulator plus loop/index temporaries.
- Bounds: one early row/column guard; no inner bounds checks.
- Non-divisible handling: extra global invocations return before the K loop.
- Likely occupancy impact: good occupancy and low register pressure; limited data reuse and poor A coalescing on row-major A for row-varying lanes.

Why it is not terrible: it does little clever work and therefore little wasted work. On small/medium resident shapes, avoiding per-K bounds logic, output-tile overdispatch, shared-memory barriers, and high accumulator pressure is enough to beat the nominally more specialized variants.

### SMALL_REGISTER_TILE

- Global ID mapping: `global_x -> row`, `global_y -> col`.
- Work per invocation: one `C[row, col]`.
- A loads: same address pattern as baseline.
- B loads: same address pattern as baseline.
- Global loads per output: `2 * K` scalar loads, split into pairs.
- Stores per output: one scalar store.
- Shared memory/barriers: none.
- Register pressure: low-to-moderate, two accumulators plus `k1`.
- Bounds: one early row/column guard; one odd-K tail check.
- Non-divisible handling: row/column return plus odd-K scalar tail.
- Likely occupancy impact: still good occupancy. The two independent accumulators can expose more instruction-level parallelism than baseline or memory-conservative's single dependency chain.

SRT is best understood as a scalar global-memory kernel with K unroll-by-2, not a true tiled SGEMM. Its wide-shape resident win is consistent with this: it keeps low overhead while adding enough K ILP for a `64x1024x1024` shape where many columns make B access favorable.

### BALANCED_2X2_ACCUM4

- Global ID mapping: `baseRow = global_x * 2`, `baseCol = global_y * 2`.
- Work per invocation: up to four outputs, `C[baseRow + {0,1}, baseCol + {0,1}]`.
- A loads: two scalar A values per K, one for each row. These are reused across two output columns.
- B loads: two scalar B values per K, adjacent columns. These are reused across two output rows.
- Global loads per produced output, for fully in-bounds useful invocations: `K` loads total per output-equivalent (`4 * K` loads for four outputs). That is better arithmetic intensity than baseline on paper.
- Stores per output: one scalar store, up to four stores per useful invocation.
- Shared memory/barriers: none.
- Register pressure: four accumulators plus A/B temporaries and many address/bounds temporaries.
- Bounds: A/B bounds checks inside every K iteration, then separate store bounds checks for each output.
- Non-divisible handling: robust but branch-heavy; tails are handled by zeroing out-of-range A/B loads and guarding stores.
- Likely occupancy impact: lower than SRT/baseline due to accumulator and temporary pressure.

Critical dispatch issue: production and resident dispatch both use `(m+7)/8, (n+7)/8` workgroups for every variant. B2x2 therefore launches roughly one invocation per output element even though each useful invocation covers four outputs. On divisible shapes, only about one quarter of invocations produce useful output; the rest still enter the K loop and execute bounds branches. This can dominate any intended reuse benefit.

### AGGRESSIVE_4X4_ACCUM8

- Global ID mapping: `baseRow = global_x * 2`, `baseCol = global_y * 4`.
- Work per invocation: up to eight outputs, `2x4`.
- A loads: two scalar A values per K.
- B loads: four adjacent scalar B values per K.
- Global loads per produced output, for fully in-bounds useful invocations: `0.75 * K` loads per output-equivalent (`6 * K` loads for eight outputs), better on paper than B2x2.
- Stores per output: one scalar store, up to eight stores per useful invocation.
- Shared memory/barriers: none.
- Register pressure: high, eight accumulators plus six per-K A/B temporaries and branch/address temporaries.
- Bounds: six A/B bounds paths inside every K iteration and guarded stores for each output.
- Non-divisible handling: robust but very branch-heavy.
- Likely occupancy impact: poor relative to all other current kernels.

Critical dispatch issue: the host still dispatches as though the kernel produced one output per invocation. On divisible shapes, only about one eighth of invocations are useful for a 2x4-output kernel. The rest are out-of-range branch work. This is the simplest static explanation for why aggressive is generally poor even where arithmetic intensity should have helped.

### MEMORY_CONSERVATIVE

- Global ID mapping: `global_x -> row`, `global_y -> col`.
- Work per invocation: one `C[row, col]`.
- A loads: contiguous along K within one invocation, same row-major pattern as baseline.
- B loads: column access through `B[kk * n + col]`, same coalescing constraints as baseline.
- Global loads per output: `2 * K` scalar loads.
- Stores per output: one scalar store.
- Shared memory/barriers: none.
- Register pressure: very low persistent pressure, one accumulator. The unrolled K body adds instruction scheduling surface without adding persistent accumulator state.
- Bounds: one early row/column guard; K rounded down by 8 plus scalar tail loop.
- Non-divisible handling: odd/non-multiple K handled by the tail loop; M/N tails return early.
- Likely occupancy impact: best among non-baseline variants.

Memory-conservative's real hardware competitiveness is credible. It is not doing more algorithmic reuse than baseline, but it has low register pressure, no workgroup memory, no barriers, no per-K output bounds checks, and a reproducible optimized GLSL-to-SPIR-V path.

### Legacy Shared-Memory Tiled Pipeline

- Global ID mapping: `global_x -> row`, `global_y -> col`.
- Work per invocation: one output.
- Shared memory: `tileA[8][8]` and `tileB[8][8]`.
- Barriers: two `OpControlBarrier` operations per K tile.
- Tile geometry: 8x8 outputs and K tile 8.
- Static concern: the tile is too small to amortize two barriers well on many shapes. It also still computes one output per invocation, so each thread performs little arithmetic between barriers.
- Status: not one of the explicit five variants, but it shows that the oldest "tiled" direction likely paid synchronization overhead without enough tile reuse.

## Resident Benchmark Interpretation

Latest artifact inspected: `out/test-artifacts/prometheus_sgemm_px16_evt_report.md`, timestamp `2026-07-09T03:43:36Z`, device `NVIDIA GeForce RTX 3070`, Vulkan hardware backend.

Resident explicit comparison highlights:

| Shape | Fastest resident explicit variant | Notable timings |
| --- | --- | --- |
| `small_64x64x64` | baseline and memory-conservative tie | baseline 0.004928 ms, MC 0.004928 ms, SRT 0.008 ms |
| `square_128x128x128` | baseline | baseline 0.0096 ms, MC 0.011488 ms, SRT 0.014496 ms |
| `square_256x256x256` | baseline | baseline 0.259456 ms, MC 0.549184 ms, SRT 0.605792 ms |
| `square_512x512x512` | memory-conservative | MC 2.79565 ms, baseline 2.98083 ms, SRT 4.10762 ms |
| `skinny_1024x64x1024` | baseline | baseline 1.20403 ms, aggressive 1.65072 ms, MC 2.1801 ms |
| `wide_64x1024x1024` | SRT | SRT 0.368672 ms, baseline 0.799904 ms, MC 1.82598 ms |
| `lowk_1024x1024x64` | baseline | baseline 1.51002 ms, MC 2.0439 ms, SRT 2.33792 ms, B2x2 3.72707 ms |
| `rect_255x129x65` | memory-conservative, near tie with SRT | MC 0.05088 ms, SRT 0.051264 ms, baseline 0.0688 ms |

### Why baseline beats SRT/B2x2/aggressive on small/medium squares

Baseline has the fewest moving parts: one accumulator, one row/column guard, no per-K bounds checks, no extra output footprint, and no workgroup synchronization. For small and medium squares, the K loop is not long enough for the register-blocked kernels to repay extra control flow. B2x2 and aggressive also suffer the variant/dispatch geometry mismatch, causing large amounts of out-of-range work.

### Why baseline beats tiled variants on some rectangular shapes

The current multi-output kernels are not shape-aware at dispatch time. On skinny and low-K shapes, B2x2/aggressive still overdispatch by their output footprint while running per-K branch logic. Baseline's lack of reuse is less damaging than launching 4x or 8x too many logical invocations. On `skinny_1024x64x1024`, baseline also avoids the high branch/store footprint that hurts the row-biased kernels when N is small.

### Why memory-conservative wins or is competitive on `rect_255x129x65`

The explicit resident rows show MC at 0.05088 ms and SRT at 0.051264 ms, effectively a tie, both ahead of baseline. This shape is awkward in all three dimensions: M and N are not multiples of common tile sizes, and K is 65. MC handles it with one early M/N guard, K unroll-by-8, and one scalar tail iteration. It avoids B2x2/aggressive's per-K output bounds checks and avoids SRT's second persistent accumulator. That matches the current data.

Important anomaly: the resident production row for the same shape reports MC at 0.007616 ms while explicit MC reports 0.05088 ms. The report's selector-vs-fastest resident table compares the production MC timing against explicit MC and shows a nonsensical 0.149686 slowdown ratio while saying both are the same variant. This should not be hidden. A future benchmark lane should repeat same-variant resident production vs explicit runs under identical iteration/warmup conditions.

### Why SRT wins on `wide_64x1024x1024` resident data

SRT computes one output per invocation like baseline and MC, so it avoids the B2x2/aggressive overdispatch problem. Its two independent accumulators split the K loop into two accumulation chains, which can expose more instruction-level parallelism than baseline or MC's single accumulator chain. With only 64 rows but 1024 columns and K=1024, the column dimension gives favorable B access and enough K depth to repay SRT's small extra register footprint. The earlier staged/timing result that favored MC is likely dominated by staged path noise, timing mode differences, or warm-state effects; resident explicit data should be treated as the kernel source of truth.

### Why aggressive is generally poor

The generated aggressive kernel is a 2x4-output, eight-accumulator kernel. That would only make sense with dispatch dimensions scaled by output footprint and careful register occupancy. Current dispatch does not scale; it launches as though each invocation emits one output. On divisible shapes, about 7/8 of invocations are out-of-range. The useful invocations still carry eight accumulators, six per-K A/B bound paths, and eight guarded stores. The resident data is exactly what this static structure predicts.

### Why B2x2 underperforms on `lowk_1024x1024x64`

Low K gives B2x2 only 64 loop iterations to amortize four accumulators and per-K bounds branches. The current host dispatch also launches about 4x the useful invocation count. Baseline's `2*K` scalar loads per output are wasteful on paper, but for low K it avoids enough branch/register/overdispatch cost to win decisively.

### Extra benchmark dimensions needed

The current data is sufficient to explain why the existing variants are weak, but not sufficient to design final selector thresholds for a replacement family. Needed diagnostic sweeps:

- Same-variant resident production vs resident explicit repeatability, especially `rect_255x129x65`.
- Dispatch-geometry experiment for B2x2/aggressive using scaled workgroup counts, measured as a diagnostic branch only.
- M/N divisibility sweep around tile boundaries: 63/64/65, 127/128/129, 255/256/257.
- K sweep at fixed M/N: 16, 32, 64, 65, 128, 256, 512, 1024.
- Aspect-ratio sweep separating tall (`M >> N`) from wide (`N >> M`) at the same FLOP count.
- Optional subgroup-size and occupancy counters if available through vendor tooling; current report records subgroup size as 0.

## Keep, Tune, Replace, or Retire

| Variant | Recommendation | Rationale |
| --- | --- | --- |
| SRT | Use only for narrow shapes until replaced; tune only if M13 needs a quick bridge | It is the healthiest current non-MC kernel and wins `wide_64x1024x1024`, but it is still a scalar global-memory kernel with no true tile reuse. It can remain a comparison/reference variant. |
| B2x2 | Retire from selector preference until rewritten or dispatch-scaled | Static dispatch mismatch and per-K bounds checks explain the resident failures. The idea of 2x2 register blocking is not bad, but this implementation shape is not worth hand-tuning first. |
| Aggressive | Retire from selector preference until rewritten | It has the worst overdispatch ratio, high register pressure, and branch-heavy K loop. Also, the implementation is 2x4 despite the public aggressive 4x4 name. |
| Memory-conservative | Keep and tune as the conservative scalar family member | It is source-backed, reproducible, low pressure, and empirically competitive. It should be retained as a fallback/reference and may be part of a future scalar/awkward-shape family. |

## DXC / Slang Evaluation

Current state:

- Current kernels are SPIR-V embedded in C headers.
- The SPIR-V disassemblies identify GLSL.std.450, and memory-conservative has a checked-in GLSL `.comp` source.
- SRT, B2x2, aggressive, baseline scalar, packed4, and fp16 do not have checked-in readable shader source next to their headers.
- The normal Windows native build consumes checked-in headers directly. It does not invoke `glslangValidator`, `dxc`, `slangc`, `spirv-val`, or `spirv-opt`.
- Local machine tools are present through the Vulkan SDK: `glslangValidator.exe`, `spirv-dis.exe`, `spirv-cross.exe`, `dxc.exe`, and `slangc.exe`.
- Repo build scripts do not currently provide a DXC or Slang shader-generation lane.

Would DXC/Slang help?

- Compiler optimization could help with scalar cleanup, unrolling, and source reproducibility, as the memory-conservative note already showed with `spirv-opt`.
- The bigger problem is algorithm/layout: current B2x2/aggressive dispatch geometry, branch placement, tile shape, and memory access design dominate compiler choice.
- HLSL or Slang would make it easier to author a matrix of variants with shared constants and templates, especially for 16x16, 32x8, low-K, and rectangular kernels.
- Slang is attractive if future multi-backend shader authoring matters, but it is a larger dependency decision than this SGEMM fix requires.
- DXC is the narrower next step if the goal is one source-backed optimized Vulkan compute kernel from HLSL.

What adding a lane would require:

- Add a source directory for SGEMM shader sources.
- Add deterministic shader-to-header generation that records compiler command, target environment, and validation status.
- Run `spirv-val` after generation and optionally targeted `spirv-opt`.
- Keep generated headers checked in so normal builds remain independent of shader tool availability.
- Add an opt-in regeneration script or Make target; do not silently regenerate during ordinary native builds.
- Document Windows and Linux tool discovery. On Windows, Vulkan SDK may provide `dxc`/`slangc`; on Linux CI this is not guaranteed.
- Decide whether CI validates generated headers from source, or only validates checked-in headers plus an optional shader-regeneration lane.

Recommendation: use DXC/HLSL or GLSL source generation for new kernels, but do not expect DXC/Slang alone to rescue the current algorithms. The first optimization win should come from a new kernel family with correct dispatch geometry and access patterns.

## Proposed New Kernel Family

### `SCALAR_COALESCED_BASELINE_PLUS`

- Target shapes: tiny, low-risk fallback, awkward tails, validation reference.
- Tile/workgroup geometry: 8x8 or 16x8, one output per invocation.
- Expected memory access: same row-major contract, but explicit mapping chosen to favor contiguous B/C stores across columns.
- Expected occupancy: high.
- Why it should beat current kernels: keeps baseline's low overhead while making source reproducible and allowing controlled unroll/FMA scheduling.
- Risks: still no cross-output reuse; may not beat current baseline without careful lane mapping.

### `TILE16X16_SHARED_FP32`

- Target shapes: medium/large square and balanced M/N shapes.
- Tile/workgroup geometry: 16x16 output tile, likely 16x16 or 8x16 workgroup depending on register/shared-memory budget; K tile 16 or 32.
- Expected memory access: cooperative contiguous loads of A and B tiles into shared memory, then multiple FMAs per barrier.
- Expected occupancy: moderate; shared memory small enough to keep occupancy reasonable if register blocking is conservative.
- Why it should beat current kernels: real tile reuse and dispatch geometry match, unlike current B2x2/aggressive.
- Risks: two barriers per K tile can still hurt if tile is too small or K is low; source needs careful layout.

### `RECT32X8_WIDE`

- Target shapes: wide/short shapes such as `64x1024x1024`.
- Tile/workgroup geometry: 32 columns x 8 rows, or similar, with work distributed so neighboring lanes load/store adjacent columns.
- Expected memory access: coalesced B and C along N, modest A reuse across many columns.
- Expected occupancy: high-to-moderate.
- Why it should beat current kernels: follows the shape where SRT already wins, but adds explicit column-major/coalesced output geometry and controlled reuse.
- Risks: may underperform tall shapes; should be a specialized recipe, not a universal default.

### `RECT8X32_TALL`

- Target shapes: tall/skinny shapes such as `1024x64x1024`.
- Tile/workgroup geometry: 8 columns x 32 rows, or smaller row-batched design if A coalescing is problematic.
- Expected memory access: favor A row locality and avoid wide-column assumptions.
- Expected occupancy: high if register blocking stays modest.
- Why it should beat current kernels: baseline currently wins tall/skinny because the specialized variants waste work. A correct tall kernel can preserve low overhead while improving reuse.
- Risks: row-major A access across many rows can be strided; may need a different lane layout than wide.

### `LOWK_16X16_NO_SHARED_OR_LIGHT_SHARED`

- Target shapes: K <= 64 or K just above a small tile boundary.
- Tile/workgroup geometry: 16x16 output tile with K fully unrolled or lightly tiled.
- Expected memory access: minimize barriers; use register blocking only if it removes repeated loads without raising occupancy cost too much.
- Expected occupancy: high.
- Why it should beat current kernels: avoids B2x2/aggressive per-K branch overhead and avoids shared-memory barriers that low K cannot amortize.
- Risks: too many specialized low-K variants can complicate selection; keep one reference kernel first.

### `MEMORY_CONSERVATIVE_SCALAR_FAMILY`

- Target shapes: awkward, odd, register-constrained, or cases where tile overhead dominates.
- Tile/workgroup geometry: one output per invocation, 8x8 or 16x8, K unroll tuned by profile.
- Expected memory access: direct global memory, no shared memory, low persistent register state.
- Expected occupancy: high.
- Why it should beat current kernels: current MC is already competitive; source-backed variants can test unroll-4 vs unroll-8 and lane geometry without changing semantics.
- Risks: limited algorithmic reuse; should not become the only optimization path.

## Recommended Next Milestone

Choose exactly one next implementation milestone:

**Px16 M13B - Add new HLSL/DXC generated 16x16 tiled SGEMM kernel.**

Why this one:

- Hand-optimizing B2x2/aggressive first would spend effort on kernels with incorrect dispatch geometry and branch-heavy inner loops.
- Retiring B2x2/aggressive would make selector behavior less misleading, but it would not add a faster kernel.
- A pure benchmark sweep would be useful, especially for the `rect_255x129x65` same-variant anomaly, but the static evidence already identifies enough design debt to begin a controlled replacement.
- A full Slang lane is broader than needed for the next concrete SGEMM win.
- A source-backed 16x16 tiled kernel gives the repo a reproducible shader-generation path and a real tiled baseline against which SRT, MC, and baseline scalar can be compared.

M13B should be scoped narrowly:

- Add one source-backed 16x16 FP32 SGEMM kernel and generated SPIR-V header.
- Keep generated headers checked in.
- Add opt-in regeneration/validation script using DXC and `spirv-val`; do not require DXC for ordinary native build.
- Add resident explicit comparison coverage for the new kernel.
- Do not retune selector scores until resident data proves the new kernel.

## Non-Goals Confirmed

- No shader optimization was implemented in M12.
- No selector scoring was changed.
- No production dispatch authority was changed.
- No P15 behavior was changed.
- No FFT/P16 work was changed.
- No benchmark semantics were changed.
- No DXC/Slang lane was added in M12.

## Commands Run

```powershell
Get-ChildItem -Force
rg --files -g "README.md" -g "*.comp" -g "*.h" -g "*.c" -g "*.cpp" -g "*.bat" -g "*.ps1" -g "*.md" -g "*.json" | rg "(sgemm|Sgemm|shader|spirv|glslang|dxc|slang|PROMETHEUS_SGEMM|px16|PX16|README_memory_conservative|reactor_vulkan_sgemm|reactor_px16_evt_benchmark_tests|M49)"
Get-ChildItem -Path primer -Force
git status --short
Get-Content README.md
Get-Content primer\vulkan-primer.md
Get-Content primer\cpp-primer.md
rg --files | rg "(\.comp$|\.glsl$|\.hlsl$|\.slang$|spirv|shader|sgemm|Sgemm)"
Get-Content internal\prometheus\native\reactor_vulkan_sgemm.c
Get-Content internal\prometheus\native\reactor_vulkan_memory_conservative.comp
Get-Content internal\prometheus\native\Marionette\reactor_px16_evt_benchmark_tests.cpp
Get-Content docs\PROMETHEUS_SGEMM_PX16_EVT.md
Get-Command spirv-dis, spirv-cross, glslangValidator, dxc, slangc -ErrorAction SilentlyContinue
Get-Content out\test-artifacts\prometheus_sgemm_px16_evt_report.md
Get-Content Experiments\PrometheusSgemmAlgorithmLab\M49\REPORT.md
spirv-dis out\px16_m12_spirv_autopsy\*.spv
spirv-cross out\px16_m12_spirv_autopsy\*.spv --version 450
Select-String -Path out\px16_m12_spirv_autopsy\*.spvasm -Pattern "OpControlBarrier|OpMemoryBarrier|Workgroup|OpVariable|LocalSize|OpExecutionMode|OpLoopMerge"
Get-Content "internal\prometheus\native\README memory conservative.md"
```

Validation commands are recorded after the report edit in the final task summary.
