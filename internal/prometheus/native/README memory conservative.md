# MEMORY_CONSERVATIVE kernel — build notes and drop-in instructions

## What this is

`reactor_vulkan_memory_conservative_spirv.h` — a compiled, optimized, validated SPIR-V
compute shader for `PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE`, in the same
format as its four wired siblings (`reactor_vulkan_srt_2accum_k_spirv.h`,
`reactor_vulkan_b2x2_row_major_biased_spirv.h`, etc.). `reactor_vulkan_memory_conservative.comp`
is the GLSL source it was compiled from — keep this checked in alongside the header,
none of the sibling `_spirv.h` files have a source `.comp`/`.hlsl`/`.slang` checked in
next to them, which means their SPIR-V is currently unreproducible from source. Worth
fixing for all five kernels while you're in here, not just this one.

## Design

No `shared` workgroup memory, exactly one `float` accumulator per invocation — the two
things that actually cost register/shared-memory budget in the sibling tiled kernels
(2x2/4x4 register blocking, shared-memory tile staging). K-loop is unrolled ×8 to match
`PROM_VK_TILE_K` for straight-line load/FMA scheduling, but this doesn't add persistent
register state (accumulator count stays at 1), so it doesn't work against the
memory-conservative goal — it's free ILP, not a tradeoff.

Buffer/dispatch convention matches every sibling exactly: A row-major MxK, B row-major
KxN, C row-major MxN, `global_x -> row`, `global_y -> col`, local size 8x8, push constant
`{m, n, k}` as 3x uint32 at offsets 0/4/8 (12 bytes total, matches `PROM_VK_SHADER_PUSH_BYTES`).

## Toolchain used

- `glslangValidator -V --target-env vulkan1.0 -S comp` — GLSL 450 → SPIR-V 1.0
- `spirv-val` — validated clean, both pre- and post-optimization
- `spirv-opt` — targeted pass (not full `-O`), flags chosen for a small single-entry-point
  kernel: `--inline-entry-points-exhaustive --eliminate-dead-branches
  --eliminate-local-single-block --eliminate-local-single-store --scalar-replacement=100
  --ccp --loop-unroll --redundancy-elimination --combine-access-chains
  --simplify-instructions --vector-dce --merge-blocks --eliminate-dead-code-aggressive`
  — result: 5936 -> 4356 bytes (~27% reduction), mainly from access-chain/load
  redundancy elimination after scalar replacement broke apart the loop-carried accumulator.
- Post-optimization disassembly re-checked against the interface contract (entry point
  name, local size, push-constant offsets, binding/set numbers) — all intact.

Neither `dxc` nor `slang` were available or installed for this; `glslangValidator` was
sufficient given the shader is plain GLSL compute with no HLSL/Slang-specific features.
If you have a reason to standardize the whole kernel family on DXC/Slang later (e.g. to
share HLSL source with a future DirectX or Metal-via-Slang backend), that's a bigger,
separate migration across all five kernels, not specific to this one.

## Drop-in steps

1. Copy `reactor_vulkan_memory_conservative_spirv.h` into
   `internal/prometheus/native/`, alongside the sibling `_spirv.h` files.
2. In `reactor_vulkan_sgemm.c`, add a `VkPipeline memory_conservative_pipeline;`
   runtime field (mirroring `srt_2accum_k_pipeline` etc.), create it at init time from
   `k_prom_sgemm_memory_conservative_spirv` the same way the other four tiled-family
   pipelines are created, and add it to whatever teardown/destroy path frees the
   sibling pipelines.
3. In the pipeline-selection `if/else` chain (§3.5 of the SGEMM audit,
   `reactor_vulkan_sgemm.c:5693-5701`), add a branch:
   ```c
   } else if (requested_variant == PROM_OCCUPANCY_KERNEL_VARIANT_MEMORY_CONSERVATIVE) {
     selected_pipeline = rt->memory_conservative_pipeline;
   ```
4. Update `prom_occ_variant_path_status()` (`reactor_vulkan_sgemm.c:4733`) — the
   `MEMORY_CONSERVATIVE` case currently returns `PROM_OCCUPANCY_VARIANT_PATH_STATUS_ALIAS_OR_NOT_WIRED`;
   once step 3 lands, this should return the same "wired" status the other three get.
5. This closes the gap noted in the SGEMM audit (§3.7/Step 4) — combined with the
   dispatch-authority wiring in that audit's §5 Step 2, all five defined variants
   (not four) will have real, distinct, dispatchable SPIR-V once both patches land.

## Not done here, deliberately

- No correctness/perf benchmarking against the CPU reference or the other four
  variants — that's exactly the DVT-phase "run it on real hardware and see what
  breaks" work per your corrected EVT/DVT/PVT model (audit §2), not something to
  fake from this sandbox, which has no GPU.
- `occupancy_apply_safety_clamp` (`reactor_judgment_engine.c:701`) never clamps
  *toward* `MEMORY_CONSERVATIVE` today — it only clamps `AGGRESSIVE_4X4_ACCUM8` down to
  `BALANCED_2X2_ACCUM4`/`SMALL_REGISTER_TILE` under register/shared-memory pressure.
  Now that a real memory-conservative kernel exists, it's worth deciding whether the
  clamp logic should route there under sufficiently severe constraints instead of
  stopping at `SMALL_REGISTER_TILE` — a judgment call for you, not something this
  kernel build should decide unilaterally.
