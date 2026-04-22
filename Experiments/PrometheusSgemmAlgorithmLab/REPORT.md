# Prometheus SGEMM Algorithm Lab

## Purpose

Prometheus SGEMM Algorithm Lab is a correctness-first Oct experiment track for matrix-multiplication design work. It is intended for scientific prototyping, algorithm exploration, and policy exploration before low-level implementation work.

## Why this is separate from the benchmark harness

This lab captures reference algorithm behavior and design reasoning in Oct. The benchmark harness answers performance questions. Keeping them separate prevents benchmark concerns from distorting baseline algorithm validation.

## M0 scope

M0 is the baseline capture milestone.

It establishes the experiment scaffold and records a reference matmul path by lifting the current `LinearAlgebra.Core.MatMul()` baseline and its supporting `.octest` into this experiment.

## M0 baseline source and coding-shape update

The M0 implementation is a semantic lift of `LinearAlgebra.Core.MatMul()` plus required helper functions (`FlatIndex` and matrix validation).

The copied baseline is rewritten to match current style conventions by replacing `while` loops that encode structured iteration with `for` loops, without changing behavior.

## Forward plan

Future milestones in this lab will evaluate algorithm variants and controller-policy ideas in Oct first, then use proven raw SGEMM paths and policy outcomes as port-ready references for later Reactor implementation.

## M1 scope

M1 introduces the first controlled SGEMM algorithm variants in this lab while keeping correctness as the primary objective.

### Variants introduced

- `MatMul_IKJ`: loop-order variant using `i -> k -> j`
- `MatMul_KIJ`: loop-order variant using `k -> i -> j`
- `MatMul_Blocked`: single-level blocked variant over `i` and `j` with baseline inner ordering
- `MatMul_Blocked_IKJ`: single-level blocked variant over `i` and `j` with `i -> k -> j` inner ordering

### Factors explored

- **Loop order**: compared baseline `i -> j -> k` against `i -> k -> j` and `k -> i -> j`
- **Blocking**: added one block-size parameter and applied one-level `i`/`j` tiling with explicit edge-tile handling

### Correctness confirmation

M1 `.octest` coverage verifies cross-variant equality against baseline across:

- small square shapes (`2x2`, `3x3`)
- rectangular shape (`3x5 * 5x2`)
- medium square shape (`16x16`)

The test coverage also verifies:

- blocked variant behavior for block sizes `1`, `2`, and `4`, including non-divisible dimensions
- consistent rejection of incompatible shapes across all variants
- rejection of invalid blocked parameters (`blockSize <= 0`)

### Structural observations (non-performance)

- Loop-order variants are now isolated as explicit, readable kernels with identical I/O contracts.
- Blocked variants currently tile only output-space dimensions (`i`, `j`) and keep multiply-accumulate semantics straightforward.
- Shared validation and indexing helpers reduce drift risk between variants and establish a repeatable pattern for later milestones.

## M2 scope

M2 expands the SGEMM algorithm space with K-dimension blocking and a conceptual staged path while preserving correctness-first validation.

### Variants introduced

- `MatMul_KBlocked`: K-blocked accumulation over `k` chunks with baseline-style inner reduction per output element
- `MatMul_KBlocked_IKJ`: K-blocked accumulation with `i -> k -> j` ordering inside each K chunk
- `MatMul_Staged`: conceptual three-phase structure (load/copy, compute, accumulate) using explicit temporary chunk buffers
- `MatMul_Staged_IKJ`: conceptual three-phase structure with `i -> k -> j` compute ordering

### Factors explored

- **K-blocking**: introduced explicit chunking over K with edge-chunk handling (`kBlock` not required to divide K)
- **Conceptual staging**: separated the algorithm into explicit phases to model data preparation, chunk-local compute, and chunk accumulation without introducing hardware simulation

### Correctness confirmation

M2 `.octest` coverage verifies baseline equality for all M2 variants across:

- small square shapes (`2x2`, `3x3`)
- rectangular shape (`3x5 * 5x2`)
- medium square shape (`16x16`)

The coverage also verifies:

- K-block edge cases for `kBlock = 1`, `2`, and `4`
- non-divisible chunk handling and accumulation correctness on rectangular inputs
- consistent rejection of incompatible matrix shapes
- consistent rejection of invalid K-block parameters (`kBlock <= 0`)

### Structural observations (non-performance)

- K-chunk accumulation makes partial-sum boundaries explicit and easier to reason about than monolithic K traversal.
- Direct K-blocked variants keep data access inline, while staged variants expose clearer phase boundaries at the cost of additional temporary structures.
- Staging centralizes chunk copy logic (`StageAChunk`, `StageBChunk`), which improves reuse and keeps per-variant compute loops focused on ordering differences.
- K-blocking combined with loop-order variants (`ijk`-style local reduction vs `ikj`) changes code shape without changing semantic output contracts.

## M2a scope

M2a ports the M0 baseline and M1 variant suite to matrix-native inputs and indexing where language support is available, while preserving algorithm structure and tests.

### What was ported to matrix-native surface

- M0 `MatMulBaseline` now accepts `Matrix<Float>` inputs, uses `m.rows`/`m.cols`, and reads elements through `m[r, c]` in the core multiply loop.
- M1 variants (`MatMulBaseline`, `MatMul_IKJ`, `MatMul_KIJ`, `MatMul_Blocked`, `MatMul_Blocked_IKJ`) now use `Matrix<Float>` input contracts and matrix-native read indexing in algorithm bodies.
- M0/M1 `.octest` fixtures now construct matrices with matrix literals and `Matrix.tabulate(...)` (for medium-size deterministic coverage).

### Helpers removed or simplified

- Simplified helper surface by removing flat-array shape plumbing from function signatures (`aRows/aCols/bRows/bCols`) and relying on matrix shape fields.
- Removed `ValidateMatrixData(...)` from M0/M1; shape checks now use matrix metadata directly.
- Kept `FlatIndex(...)` and zero-buffer helpers only for mutable accumulation/storage paths, due current matrix write limitations (see inconsistency note).
- Kept and reused shape-level helpers that remain true contract checks: `ValidateMatMulInputs(...)`, `ValidateMatrixShape(...)`, `ValidateBlockSize(...)`, and `MinInt(...)`.

### How algorithm structure was preserved

- Loop-order intent is unchanged:
  - baseline remains `i -> j -> k`
  - `MatMul_IKJ` remains `i -> k -> j`
  - `MatMul_KIJ` remains `k -> i -> j`
- Blocked variants still tile output-space (`i`, `j`) with explicit edge handling and preserve their original inner-loop ordering differences.
- Error contracts under test remain unchanged in meaning (shape mismatch rejection and invalid block-size rejection).

### Intentionally not rewritten in M2a

- M2 variants were intentionally left untouched to keep scope narrowed to M0/M1 surface normalization.
- No benchmark-harness, runtime, Reactor, or optimization work was included.

### Inconsistency note surfaced

Two explicit gaps were surfaced during the port and left visible by design:

- Matrix element **write** parity (`m[r, c] = value`) has now been addressed by Mx101d, so M0/M1 accumulation no longer needs flat row-major output buffers.
- Anonymous callback functions are still rejected in this environment, so generic row-major array -> matrix conversion via captured callbacks remains unavailable.

M0/M1 now use matrix-native inputs, reads, **and writes** with shape metadata. M2 remains unchanged and still flat-array-based by milestone scope.
