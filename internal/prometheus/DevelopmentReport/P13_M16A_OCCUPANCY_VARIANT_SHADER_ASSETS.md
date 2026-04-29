# P13 M16a — Occupancy Variant Shader Assets (SPIR-V)

## 1. M52 recipe source

Source of truth:
- `Experiments/PrometheusSgemmAlgorithmLab/M52/REPORT.md`
- `Experiments/PrometheusSgemmAlgorithmLab/M52/m52_final_recipe_map.octagon`
- `Experiments/PrometheusSgemmAlgorithmLab/M48/REPORT.md` (SRT contract)

Recipe map:
- baseline-scalar -> existing baseline/current path
- memory-conservative -> MC-baseline-strict
- small-register-tile -> SRT-2accum-K
- balanced-2x2-accum4 -> B2x2-row-major-biased
- aggressive-4x4-accum8 -> A2x4-row-biased-accum8

## 2. Shader assets added

Added embedded SPIR-V headers (not yet wired into runtime dispatch):
- `internal/prometheus/native/reactor_vulkan_srt_2accum_k_spirv.h`
- `internal/prometheus/native/reactor_vulkan_b2x2_row_major_biased_spirv.h`
- `internal/prometheus/native/reactor_vulkan_a2x4_row_biased_accum8_spirv.h`

MC-baseline-strict remains documented as baseline alias for M16a.

## 3. Shader contract per variant

### SRT-2accum-K
- One output element per invocation (`row,col` mapping preserved).
- Two independent K accumulators (`acc0`, `acc1`) with even/odd split.
- Final output is `acc0 + acc1`.

### B2x2-row-major-biased
- Invocation computes 2x2 tile at `(2*gid.x, 2*gid.y)`.
- Four accumulators (`c00,c01,c10,c11`).
- Row-major-biased stores with explicit tail masks.

### A2x4-row-biased-accum8
- Invocation computes 2x4 tile at `(2*gid.x, 4*gid.y)`.
- Eight accumulators.
- Row-major-biased stores with explicit tail masks.

## 4. Odd/tail bounds handling

- SRT: odd-K handled by post-loop remainder term.
- B2x2/A2x4: M/N tails handled by masked loads and masked stores.
- All kernels guard global row/col writes against `m,n` bounds.

## 5. M16b wiring guidance

M16b should:
1. add benchmark-only occupancy variant override seam,
2. map variant -> shader module/pipeline selection,
3. keep production path unchanged by default,
4. add diagnostics proving requested/executed variant path,
5. add correctness tests per variant.

## 6. Validation performed

- Installed `glslangValidator` and compiled GLSL compute shaders to SPIR-V binaries offline.
- Generated embedded `uint32_t` SPIR-V arrays from binaries.
- Built native target to verify new headers compile in repository context.

## 7. Deferred scope

Deferred intentionally:
- runtime dispatch seam,
- benchmark harness wiring,
- runtime diagnostics extension,
- correctness execution tests requiring variant actuation,
- any performance claims/tuning.
