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

## M3 scope

M3 organizes the accumulated SGEMM variants into an explicit selection surface and introduces a first deterministic policy prototype based on matrix structure only.

### Variant families (before pruning)

- **Direct family**:
  - `MatMulBaseline`
  - `MatMul_IKJ`
  - `MatMul_KIJ`
  - `MatMul_Blocked`
  - `MatMul_Blocked_IKJ`
  - `MatMul_KBlocked`
  - `MatMul_KBlocked_IKJ`
- **Staged family**:
  - `MatMul_Staged`
  - `MatMul_Staged_IKJ`

### Strategy surface introduced

M3 introduces an experiment-level enum surface:

- `MatMulStrategy.Baseline`
- `MatMulStrategy.IKJ`
- `MatMulStrategy.KIJ`
- `MatMulStrategy.Blocked`
- `MatMulStrategy.BlockedIKJ`
- `MatMulStrategy.KBlocked`
- `MatMulStrategy.KBlockedIKJ`
- `MatMulStrategy.StagedIKJ`

`MatMulWithStrategy(...)` centralizes dispatch to retained implementations and keeps per-variant behavior unchanged.

### Heuristic policy prototype (`ChooseStrategy`)

The initial chooser is deterministic and uses only structural matrix signals:

- `m`, `n`, `k` dimensions
- output area (`m * n`)
- K dominance relative to outer dimensions

Current policy intent:

- very small problems choose `IKJ`
- K-dominant problems choose `KBlockedIKJ`
- larger output surfaces choose `BlockedIKJ` unless K is also large
- medium balanced cases choose `KBlocked` when K/reduction pressure is elevated
- fallback chooses `Blocked`

No timing, hardware, or Octomata assumptions are used.

### Variant pruning and justification

M3 prunes `MatMul_Staged` and keeps `MatMul_Staged_IKJ` as the staged-family representative.

Justification:

- `MatMul_Staged` and `MatMul_Staged_IKJ` share the same staging phases (load/copy, compute, accumulate).
- The only difference is local loop order during compute.
- Loop-order differentiation is already represented in the direct family (`Baseline`/`IKJ`/`KIJ`) and K-blocked direct family (`KBlocked`/`KBlocked_IKJ`).
- Retaining both staged variants duplicates that axis inside the same staged mechanism without adding a distinct structural family.

This keeps staged exploration present while reducing redundant combinatorics in the selection surface.

### Correctness confirmation

M3 `.octest` coverage validates:

- strategy-dispatch equivalence to baseline across retained strategies and representative square/rectangular shapes
- deterministic and expected structural behavior of `ChooseStrategy(...)`
- direct-family and staged-family cross-consistency (`KBlocked_IKJ` vs `Staged_IKJ`)
- edge-case rejections for shape mismatch and invalid block parameters via strategy dispatch

### Structural observations (non-performance)

- The strategy surface makes algorithm intent explicit and gives a single integration point for future policy work.
- Direct-family variants remain the most granular loop-structure knobs.
- Staged-family structure remains useful as a distinct mechanism, but one representative staged kernel is sufficient at this phase.
- Uncertainty remains around where staged paths should be selected in future policy iterations; M3 therefore keeps staged selection explicit but conservative.

### Inconsistency note surfaced

M2 was originally flat-array based, while M0/M1 had already moved to matrix-native inputs/shape metadata.

M3 aligns M2 with matrix-native indexing and shapes to match current language-supported matrix read/write conventions and keep the lab surface consistent.

## M4 scope

M4 introduces controlled execution measurement to validate M3 structural assumptions without turning the lab into a full benchmark suite.

### Measurement setup

- **Cloud directional probe (completed):**
  - Measurement surface: `Experiments/PrometheusSgemmAlgorithmLab/M4`
  - Strategy wrapper: `MeasureStrategy(...)` routes through `MatMulWithStrategy(...)` and keeps benchmark bodies structurally minimal.
  - Bench command used: `go run ./cmd/oct bench Experiments/PrometheusSgemmAlgorithmLab/M4 --octagon-out out/prometheus/sgemm_lab_m4/cloud_m4_bench.octagon`
  - Strategy-selection snapshot command: `go run ./cmd/oct artifact Experiments/PrometheusSgemmAlgorithmLab/M4` (emits `m4_choose_strategy.octagon`)
- **Windows native checkpoint (planned, not executed in this environment):**
  - Same M4 corpus and strategy sweep should be re-run under real Windows + native NVIDIA Vulkan Prometheus environment.
  - Use this as confirmation only for crossover and dispatch/setup realism; do not transplant cloud timings directly.

### Shape sets tested

M4 uses the fixed DOE shape set:

- Small: `2x2`, `4x4`
- Medium: `16x16`, `32x32`
- Rectangular: `16x64 * 64x8`, `8x128 * 128x16`
- K-dominant: `8x8 * 256`, `16x16 * 512`

Each shape sweeps all retained strategies:

- Baseline, IKJ, KIJ, Blocked, BlockedIKJ, KBlocked, KBlockedIKJ, StagedIKJ

Structured cloud output is recorded in:

- `out/prometheus/sgemm_lab_m4/cloud_m4_bench.octagon`
- `Experiments/PrometheusSgemmAlgorithmLab/M4/CLOUD_MEASUREMENTS.md`
- `Experiments/PrometheusSgemmAlgorithmLab/M4/m4_choose_strategy.octagon`

### Observed trends (cloud directional only)

- Staged strategy (`StagedIKJ`) is generally non-winning across medium, rectangular, and K-dominant probes in this run.
- `KBlocked`/`KIJ` frequently appear near the front for K-heavy and rectangular cases, supporting the idea that K-structure can matter.
- Small-shape timings are tightly clustered and noisy; no robust claim should be made from absolute differences there.

### Crossover observations (directional)

- In this cloud run, direct variants remain favored for most shapes.
- K-dominant regions show clearer preference shifts toward K-aware direct variants (`KBlocked`, `KBlockedIKJ`, sometimes `KIJ`) versus baseline/blocked forms.
- No convincing staged crossover was observed in cloud data.

### Heuristic mismatches identified

From `ChooseStrategy` vs observed cloud winners:

- `S16x16`: chooser picked `KBlocked`, fastest observed was `Baseline`.
- `S32x32`: chooser picked `KBlockedIKJ`, fastest observed was `KIJ`.
- `R16x64x8`: chooser picked `KBlockedIKJ`, fastest observed was `KIJ`.
- `R8x128x16`: chooser picked `KBlockedIKJ`, fastest observed was `Baseline`.
- `K8x8x256`: chooser picked `KBlockedIKJ`, fastest observed was `KBlocked`.
- `K16x16x512`: chooser picked `KBlockedIKJ`, fastest observed was `Baseline`.

These are exactly the kind of mismatches M4 was intended to expose before introducing scoring.

### Candidate signals for future M5 scoring

- Relative K pressure (`k / max(m, n)`)
- Output area (`m * n`) plus rectangularity ratio (`max(m, n) / min(m, n)`)
- Staging penalty signal (temporary-buffer overhead signature)
- Strategy family penalties/bonuses inferred from repeated directional wins, not single-point absolute timings

### Inconsistency/documentation gap surfaced

The M4 request asks for a direct in-language `MeasureStrategy(...) -> duration` API. Current `Language/reference` builtins do not document a timing builtin for obtaining wall-clock duration inside Oct code. M4 therefore uses `oct bench` boundary timing (`DurationNs` in `.octagon`) as the measurement source of truth and keeps `MeasureStrategy(...)` as an execution wrapper rather than an in-language timer.
