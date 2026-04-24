# P9 M21 — Vulkan Shader Implementation for `Packed4FP32`

## 1) Shader design overview

M21 adds a dedicated Vulkan compute pipeline for `Packed4FP32` (`k_prom_sgemm_packed4_spirv`) and wires it into the existing reactor pipeline selection. The shader keeps the existing dispatch mapping (`x -> row`, `y -> column`) and computes one output element per invocation.

Packed layout contract used by the shader:

- A is uploaded as row-major packed lanes: logical shape `M x K4`
- B is uploaded as column-major packed lanes: logical shape `N x K4`
- C remains canonical row-major `M x N`
- push constant `k` is `K4 = round_up_4(K)` for packed mode

The kernel performs scalar accumulation in `kk=0..K4-1` order to preserve exact scalar-oracle equality.

## 2) M19 invariant mapping to shader/runtime logic

1. **Row-major equivalence**  
   - Shader accumulation order is scalar and deterministic.  
   - Optional debug/oracle gate (`PROM_TESTCFG_PACKED4_DEBUG_ORACLE_CHECK`) compares against CPU row-major and records mismatch counters.

2. **Padding isolation**  
   - Host packers explicitly zero-fill padded lanes for both A and B.  
   - Shader multiplies through `K4`, so padded lanes act as zero and cannot contaminate output.

3. **Tail correctness**  
   - `K` tails are represented via `K4` plus explicit zero-filled padded lanes.  
   - `M/N` tails are guarded by `row>=m || col>=n` early return.

4. **No layout leakage**  
   - Packed layout is internal upload-only representation.  
   - Public output buffer remains canonical row-major FP32.

5. **Deterministic fallback compatibility**  
   - Packed selection remains judgment-engine driven.  
   - Existing baseline/tiled/fallback routes are unchanged and remain observable.

## 3) Tail and padding handling strategy

- **Packing:**
  - `prom_pack_a_packed4_rowmajor(...)` zero-fills and copies real `K` values per row.
  - `prom_pack_b_packed4_colmajor(...)` zero-fills and copies real `K` values per column.
- **Execution:** shader loops over `K4`; padded lanes contribute `0.0f` exactly.
- **Bounds:** output stores only occur for valid row/col indices.

## 4) Reactor integration

- Added `packed4_pipeline` lifecycle handling in runtime init/cleanup.
- `PROM_VK_COMPUTE_PACKED4_FP32` now executes through Vulkan dispatch (not CPU-only emulation).
- Buffer sizing/allocation now uses `K4` for packed mode A/B while keeping C row-major size unchanged.

## 5) Diagnostics added/updated

- Existing packed4 diagnostics are preserved:
  - selection count
  - tail counts
  - padded-lane counts
  - fallback reasons
- `packed4_row_major_check_failures` is now tied to explicit debug-oracle mode (`PROM_TESTCFG_PACKED4_DEBUG_ORACLE_CHECK`) rather than always-on correction behavior.

## 6) Test coverage delivered in M21

- Existing packed4 tests continue to pass for:
  - packed selection path
  - fallback observability
  - tail/rectangular exact-oracle checks
- Added full `M/N/K mod 4` sweep test (`8+mod`) verifying exact equality across all 64 combinations.

## 7) Known limitations

- Current packed shader is correctness-first, scalar-accumulation structure (non-optimized).
- No subgroup/cooperative-matrix/vendor extension usage by design.
- Performance tuning is intentionally deferred to later milestones.
