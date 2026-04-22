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

## M4b scope

M4b is the Windows-native Prometheus reality check for the suspicious M4 shapes, with an explicit emphasis on proving what the current code can and cannot validate truthfully.

### Windows-native path validation

- Added `Experiments/PrometheusSgemmAlgorithmLab/M4b` as a narrow Prometheus-path benchmark surface for the six suspicious shapes:
  - `16x16x16`
  - `32x32x32`
  - `16x64 * 64x8`
  - `8x128 * 128x16`
  - `8x8 * 256`
  - `16x16 * 512`
- Each benchmark executes matrix multiply inside `PROMETHEUS { ... }`, which is the only currently compiled path that lowers to native Prometheus SGEMM.
- Native runs are intended to be executed sequentially and repeated (`>= 3`) with `OCT_PROMETHEUS_REACTOR` pointed at the built Windows DLL.

### Critical architectural constraint surfaced

M4 strategy kernels (`Baseline`, `IKJ`, `KIJ`, `Blocked`, `BlockedIKJ`, `KBlocked`, `KBlockedIKJ`, `StagedIKJ`) are written as pure Oct loop kernels.

Current compiled-mode lowering sends work to native Prometheus only for the matrix `@` operator inside a `PROMETHEUS` block:

- `internal/build/compiler.go`: `MatMulMM`
- `internal/build/compiler.go`: `PrometheusMatMulMM`

That means:

- the existing M4 strategy sweep does **not** execute on the Vulkan Prometheus path
- `oct bench Experiments/PrometheusSgemmAlgorithmLab/M4` measures compiled CPU execution for those custom kernels
- M4b can currently validate the **Prometheus path on the suspicious shapes**, but it cannot yet produce truthful **per-strategy native Prometheus rankings** without new lowering/runtime support

### Confirmed vs rejected assumptions

- Confirmed: native Windows Prometheus execution is a real, testable path for the suspicious shape set when `PROMETHEUS { a @ b }` is used.
- Rejected: the current M4 custom-strategy benchmark surface is a valid proxy for native Prometheus strategy behavior. It is not.

### Updated interpretation for M5

- No new strategy-family scoring signal should be inferred from “native Prometheus” unless those strategy families can actually lower into Prometheus.
- The strongest trustworthy M4b signal today is environment/path truthfulness:
  - `BackendUsed`
  - `Status`
  - `Environment`
  - repeated wall-time behavior for the real native `@` path on suspicious shapes
- Future `when utility` scoring should only consume strategy-vs-shape signals from a benchmark surface that genuinely runs those strategies on the intended backend.

### M4b execution notes and observed results

- Machine:
  - Windows
  - NVIDIA GeForce RTX 3070
  - Vulkan loader present (`vulkan-1.dll`)
- Reactor:
  - `OCT_PROMETHEUS_REACTOR` pointed at `out/prometheus/native/prometheus_reactor.dll`
- Go/native bridge requirement surfaced during execution:
  - `CGO_ENABLED=1` is required on Windows for the real DLL loader path (`windows && cgo`)
  - in this shell, `go env CGO_ENABLED` initially reported `0`
  - with `CGO_ENABLED=0`, Prometheus reported a truthful but misleading fallback symptom for this task: `fallback(prometheus_unavailable)` with note `prometheus reactor load failed`
  - after enabling cgo and using the UCRT GCC toolchain on `PATH`, native Prometheus execution became available and reported `BackendUsed=prometheus` with `Environment=windows_native_vulkan`

### Shapes tested

- `16x16x16`
- `32x32x32`
- `16x64 * 64x8`
- `8x128 * 128x16`
- `8x8 * 256`
- `16x16 * 512`

Each shape was run three times sequentially through the Prometheus `@` path in `Experiments/PrometheusSgemmAlgorithmLab/M4b`.

### Native Prometheus path observations

Per-shape reported native wall times (`ReportedWallNs`) across the three runs:

- `16x16x16`: `501700`, `635100`, `559700`
- `32x32x32`: `664100`, `599500`, `636500`
- `16x64 * 64x8`: `503400`, `501500`, `1049500`
- `8x128 * 128x16`: `1213800`, `1157600`, `681700`
- `8x8 * 256`: `501600`, `501700`, `569500`
- `16x16 * 512`: `684100`, `1019000`, `517900`

Common metadata across all eighteen native shape runs:

- `BackendUsed: prometheus`
- `Status: ok`
- `Environment: windows_native_vulkan`

### Comparison vs cloud M4a

- Confirmed:
  - the Windows-native Prometheus path is live and stable enough to execute the suspicious-shape set repeatedly on real hardware
  - native kernel wall times are much smaller than the end-to-end compiled benchmark `DurationNs`, so cloud and compiled boundary timings should not be treated as hardware-proxy timings
- Rejected:
  - the assumption that `Experiments/PrometheusSgemmAlgorithmLab/M4` already represented “Prometheus reality”
- Still unresolved:
  - cloud chooser-vs-observed mismatches for `KBlocked`, `KBlockedIKJ`, `KIJ`, and `Baseline`
  - true crossover behavior among retained M3 strategies on native Prometheus
  - staged-family competitiveness on native Prometheus

### Updated interpretation of K-blocking and staged viability

- `KBlocked` / `KBlockedIKJ`:
  - M4b does **not** yet validate or reject K-block usefulness on native Prometheus, because the current native path exercises only builtin SGEMM (`@`) and not the custom M3 kernels
- `StagedIKJ`:
  - staged viability remains unevaluated on native Prometheus for the same reason
  - the only trustworthy staged signal remains the cloud/compiled directional signal, which is explicitly insufficient for native policy conclusions

### Candidate signals for M5 scoring after M4b

- Safe to use:
  - backend availability as an explicit gating signal
  - environment classification (`windows_native_vulkan` vs fallback/unavailable)
  - repeated native wall-time behavior for the builtin Prometheus SGEMM path on suspicious shapes
- Not safe to use yet:
  - per-strategy native scoring among `Baseline`, `IKJ`, `KIJ`, `Blocked`, `BlockedIKJ`, `KBlocked`, `KBlockedIKJ`, `StagedIKJ`
  - any native staged penalty or K-block bonus derived from the current M4b path-validation surface

## M4c scope

M4b exposed a concrete architecture gap: the retained M3 strategy family in `M4` was measuring compiled CPU loop kernels, while the real Prometheus path was only proven for builtin `@` inside `PROMETHEUS { ... }` in `M4b`.

### Bridge added

M4c introduces a narrow strategy-to-Prometheus execution bridge for the lab:

- new builtin surface: `PrometheusMatMul(left: Matrix<Float>, right: Matrix<Float>) -> Matrix<Float>`
- compiler lowering of that builtin directly to `PrometheusMatMulMM` in MIR
- strategy refactor in `M4` to keep strategy control structure in Oct while delegating SGEMM compute steps through that bridge

This keeps Oct as the strategy/control layer while moving actual matrix multiply compute onto the Prometheus-backed runtime path.

### Strategy-family consequences after bridging

After delegation, several previous loop-order distinctions no longer describe distinct Prometheus execution shapes:

- direct-family variants are now execution aliases over the same bridge call:
  - `Baseline`, `IKJ`, `KIJ`
- blocked-family loop-order aliases are likewise collapsed:
  - `Blocked` and `BlockedIKJ`
- K-block staging aliases are collapsed:
  - `KBlocked`, `KBlockedIKJ`, `StagedIKJ`

Meaningful retained structure is now at delegation granularity:

1. full-matrix single-call delegation
2. output-block decomposition (multiple delegated calls over row/col tiles)
3. K-block decomposition with accumulation (multiple delegated calls over K chunks)

This is an intentional simplification: M4c prunes fake distinctions that only existed when scalar multiply-accumulate was implemented directly in Oct loops.

### What measurement is now possible

`Experiments/PrometheusSgemmAlgorithmLab/M4` now invokes Prometheus-backed compute from the strategy path itself, so future Windows-native runs can compare real backend-targeted strategy families instead of CPU-loop proxies.

### Correctness and contract status

- matrix shape mismatch behavior is preserved
- invalid `blockSize` / `kBlock` behavior is preserved
- strategy parity tests continue to validate against baseline outputs
- lowering/compiled tests now explicitly assert that the bridge path emits `PrometheusMatMulMM` and reports backend/fallback status strings

### Out of scope that remains out of scope

M4c intentionally does **not**:

- lower arbitrary Oct loops to Prometheus
- port strategy code to C/Reactor
- introduce broad GPU-lowering infrastructure
- introduce scoring (`when utility`) or controller policy work

### Inconsistency/documentation gap surfaced

`PrometheusMatMul(...)` is introduced as a minimal bridge surface for this experiment milestone, but `Language/reference` does not currently document this builtin. This is a language-reference gap that should be resolved explicitly in a follow-up documentation pass.

## M4d scope

M4d is the first truthful Windows-native hardware comparison after M4c bridge collapse.

The goal was not to optimize kernels or rewrite policy. The goal was to run only the still-meaningful post-M4c families on real Prometheus-backed compute and see which families survive contact with actual hardware.

### Environment used

- Windows
- native NVIDIA Vulkan environment
- `OCT_PROMETHEUS_REACTOR=internal/prometheus/reactor/prometheus_reactor.dll`
- `CGO_ENABLED=1`
- `CC=C:\Users\yuech\mingw64\bin\gcc.exe`
- `CXX=C:\Users\yuech\mingw64\bin\g++.exe`
- fresh CLI built from current source by `tools/prometheus/run_m4d_windows_native.ps1`

The checked-in repo binary `oct.exe` was stale relative to current language support and failed to parse current `Matrix.zeros<T>(...)` syntax, so M4d intentionally uses a freshly built CLI from the current source tree for truthful execution.

### Retained strategy set tested

M4d respects the M4c collapse and benchmarks only the three backend-distinct retained families:

- **SingleCall**: full-matrix single-call delegation
  - benchmark representative: `SingleCallRep`
  - chooser aliases collapsed here: `Baseline`, `IKJ`, `KIJ`
- **Blocked**: output-block decomposition
  - benchmark representative: `BlockedRep`
  - chooser aliases collapsed here: `Blocked`, `BlockedIKJ`
- **KDecomposition**: K-block decomposition with accumulation
  - benchmark representative: `KDecompositionRep`
  - chooser aliases collapsed here: `KBlocked`, `KBlockedIKJ`, `StagedIKJ`

`StagedIKJ` is therefore not measured as a separate row in M4d, because after M4c it is not a separate Prometheus execution shape.

### Shape set tested

The required six-shape M4b suspicious set was retained unchanged:

- `16x16x16`
- `32x32x32`
- `16x64 * 64x8`
- `8x128 * 128x16`
- `8x8 * 256`
- `16x16 * 512`

### Output artifacts

M4d writes its Windows-native run artifacts under:

- `out/prometheus/sgemm_lab_m4d/summary.json`
- `out/prometheus/sgemm_lab_m4d/summary.md`
- per-run `.octagon` and stdout captures for each shape/family/run
- copied chooser artifact: `out/prometheus/sgemm_lab_m4d/m4d_choose_strategy.octagon`

### Truthfulness checks

- `go run ./cmd/oct test Experiments/PrometheusSgemmAlgorithmLab/M4` passed (`7 passed, 0 failed`)
- `go test ./cmd/oct -run TestWindowsBenchM4RetainedFamiliesUsePrometheusBackend -count=1` passed under `CGO_ENABLED=1`
- every M4d measured row reported:
  - `BackendUsed=prometheus`
  - `Status=ok`
  - `Environment=windows_native_vulkan`

This confirms M4d results are not CPU-loop proxy timings and are not silent fallback data.

### Results summary

Per-shape fastest retained family by median `ReportedWallNs` in the measured three-run slice:

- `16x16x16` -> `Blocked` (`1182200 ns`)
- `32x32x32` -> `SingleCall` (`1017900 ns`)
- `16x64 * 64x8` -> `KDecomposition` (`1020600 ns`)
- `8x128 * 128x16` -> `KDecomposition` (`1023700 ns`)
- `8x8 * 256` -> `KDecomposition` (`1155600 ns`)
- `16x16 * 512` -> `KDecomposition` (`1018900 ns`)

Chooser vs observed-fastest after M4c alias collapse:

- `16x16x16`: chooser=`KDecomposition`, observed-fastest=`Blocked`
- `32x32x32`: chooser=`KDecomposition`, observed-fastest=`SingleCall`
- `16x64 * 64x8`: chooser=`KDecomposition`, observed-fastest=`KDecomposition`
- `8x128 * 128x16`: chooser=`KDecomposition`, observed-fastest=`KDecomposition`
- `8x8 * 256`: chooser=`KDecomposition`, observed-fastest=`KDecomposition`
- `16x16 * 512`: chooser=`KDecomposition`, observed-fastest=`KDecomposition`

Net result: the current chooser is directionally wrong on **2 / 6** required shapes. It still over-selects K-decomposition in the small balanced square region, but the family is much more viable on real hardware than the earlier cloud proxy results suggested.

### Chooser mismatches

The strongest concrete chooser errors are:

- `16x16x16`: chooser overcommits to `KDecomposition`; real hardware prefers output blocking
- `32x32x32`: chooser overcommits to `KDecomposition`; real hardware prefers the single-call family

So the present heuristic is not broadly broken, but its K-dominance intuition is still too aggressive in small-to-mid square regions.

### Staged viability conclusion

After M4c, staged is no longer a separately measurable backend family:

- `StagedIKJ` is an execution alias of `KDecomposition`
- M4d therefore does **not** produce an independent staged score
- staged should not remain a separate candidate dimension for M5 unless the backend path becomes structurally distinct again

That means staged is no longer justified as an independently scored strategy family.

### K-block usefulness conclusion

K-aware decomposition is real and frequently useful on real Prometheus hardware, but it is not a universal default.

Evidence from M4d:

- it wins on `16x64 * 64x8`
- it wins on `8x128 * 128x16`
- it wins on `8x8 * 256`
- it wins on `16x16 * 512`
- it loses on `16x16x16`
- it loses on `32x32x32`

Conclusion:

- K-decomposition remains a real candidate family
- K-aware preference is justified on most of the M4d suspicious set, especially the rectangular and K-heavy cases
- the current chooser still applies that preference too broadly for small balanced square shapes

### Stability / variance

Rankings are usable, but not cleanly stable across the whole six-shape set.

- every required shape showed some winner or full-order variation across the three-run slice
- inner-wall deltas are often sub-millisecond and should be treated as informative rather than absolute

This means M5 should include a confidence or variance penalty rather than assuming every observed ranking is equally strong.

### Recommended signals for M5 scoring

M4d suggests the next scoring pass should use collapse-aware family signals rather than pre-M4c raw strategy names:

1. single-call vs blocked vs K-decomposition family identity
2. K-dominance, but moderated rather than treated as an automatic K-decomposition win
3. output tiling pressure (`m`, `n`, output area, tile count)
4. K-chunk pressure (`k`, estimated K-chunk count)
5. rectangular aspect signals, especially the `16x64 * 64x8` style region where K-decomposition repeatedly surfaced near the front
6. ranking confidence / variance penalty derived from repeated-run stability

### Strategy family pruning / demotion recommendation

M4d supports the following post-hardware conclusions:

- **Keep `SingleCall`** as a real candidate family
- **Keep `Blocked`** as a real candidate family, but as a narrow small-square specialist unless future data broadens its region
- **Keep `KDecomposition`** as a real candidate family
- **Demote/prune `StagedIKJ` as an independent family** from future scoring, because it is not backend-distinct after M4c

No optimization or chooser rewrite is introduced in M4d. The milestone ends with truthful hardware evidence that narrows M5 to family-level utility scoring rather than raw pre-bridge strategy labels.
