# OCT-SCIENCE-LIBRARIES-M0 — Executable Textbook Modernization and Science Expansion

## 1. Verdict

**Success**

The built-in library tree was audited, nine scientific areas were materially improved, thin simulation scaffolding became a coherent bounded library, all new paths passed both compiled and interpreted execution, and the repository-wide Go test/vet gates remained green.

## 2. Philosophy

This pass treats a library routine as executable explanation. Implementations expose the equation or algorithm, validate genuine scientific domains, state numerical limitations, and avoid heavyweight dependencies. Existing high-performance or specialized areas were not duplicated merely to increase file count.

## 3. Library inventory

The audit covered 50 direct `Libraries/` directories: 154 `.oct` files (including manifests), 117 `.octest` files, 15 `.octfail` files, 40 library READMEs, and 942 `[Fact]` contracts.

| Classification | Libraries / finding |
| --- | --- |
| Modern and healthy scientific core | Analysis, ChemistryNmr, Complex, DifferentialEquations, Distributions, Geometry, Interpolation, LinearAlgebra, Mathematics.Transforms, Mechanics, Numerics, Octomata, Optimization, Random, RF, Signal, Statistics, Tensor2D, Thermofluids, Uncertainty, Units |
| Modern and healthy infrastructure | Archive, Artifact, Compression, Csv, Deployment, Hash, Image, IO read paths, Json, Loop, Make, Markdown, Pdf, Plot, Text, UI |
| Useful but thin before this pass | Algorithms, Physics, Simulation; all three received substantive additions |
| Useful but still thin or narrow | HelloScience (intentional scaffold), String, Structures, Time, Wireless |
| Useful but old-style / follow-up candidate | Wireless retains an explicit untyped-Hz modernization TODO; some scalar Statistics APIs use assertion-backed preconditions rather than fallible signatures |
| Overlapping, not safely redundant | Mathematics.Calculus overlaps Numerics differentiation/quadrature; Numerics.Optimization overlaps the larger Optimization package. Compatibility prevents casual deletion |
| Experimental / harness-oriented | ArtifactUsage, MakeHostPrivileged, MakeOctestPlan, IfErrNotEqualNil; these are fixtures or integration surfaces rather than general science APIs |
| Explicit remaining stub | `IO.WriteTable` remains documented as not implemented; it is an external table-transport concern and was not disguised as completed science functionality |
| Dead/obsolete | No source was sufficiently proven dead to remove safely |

## 4. Modernization strategy

Changes were narrow additions or correctness repairs in established packages. Exact templates removed type duplication only where the algorithm stayed ordinary. Refined Concepts now own reusable single-value domains; relational and algorithmic failures remain fallible. `batch` is used only for independent ordered mapping. `with` configures immutable simulation scenarios.

A follow-up validation audit found that `StaticAssert` is not a general library-precondition mechanism: it executes only in the compiler-owned artifact/compiled-data static phase. Ordinary `Require(parameter condition)` is also invalid because parameters are outside its bounded proof evaluator. This conflicts with the older `internal/libraries/LIBRARY_MODERNIZATION_M7_RETURN_ERROR_AUDIT.md`, which labels parameter guards as direct `Require` candidates. The current `Language/reference/language/18-concepts.md` is authoritative, so this pass uses refined Concept constructors for reusable scalar domains instead.

## 5. Algorithms

`Algorithms` grew from one templated search helper into a compact discrete-algorithm chapter:

- `CountWhere<T>` preserves exact predicate types;
- ascending-array `BinarySearch` documents its caller-owned sortedness contract;
- Euclidean `GCD` and overflow-conscious `LCM` define sign and zero behavior;
- `IsPrime` is the readable O(sqrt(n)) trial-division algorithm;
- `PrimesThrough` is the O(n log log n) Sieve of Eratosthenes.

Known exact cases, empty input, signs, zeros, composites, primes, and invalid sieve bounds are contracted.

## 6. Numerical methods

Adaptive Simpson integration no longer reports a constant evaluation count of three or unconditional convergence. Recursive panels now return value, evaluations, and convergence. The public result counts all function evaluations and returns `Converged = false` when maximum depth is exhausted before the error estimate meets tolerance.

The existing roots, finite differences, Gauss-Legendre, Simpson/trapezoid, golden-section, and Brent implementations were inventoried as already substantial and were not duplicated.

## 7. Statistics/probability

Statistics gained `WeightedMean` with non-empty, matching-shape, non-negative-weight, and positive-total-weight errors. Both numerator and denominator use compensated summation to reduce low-order loss. A refined non-empty-array Concept was tested but removed because its generated Go indexing path caused compiled fallback; this parity gap is recorded rather than hidden.

Distributions gained Bernoulli, binomial, and Poisson PMFs plus `LogFactorial`. `Probability`, `PositiveDistributionScale`, and `NonNegativeCount` admit parameters once, making evaluation infallible afterward. Binomial coefficients use a symmetric multiplicative form instead of integer factorials; Poisson mass uses a recurrence. Documentation explicitly limits these routines to moderate parameters and points extreme tails toward log-domain algorithms.

## 8. Linear algebra

The audit found dot products, norms, matrix products, transpose, trace, LU factorization/solve/inverse, determinant, power iteration, Jacobi eigensolving, and Jacobi SVD with 48 passing contracts. No duplicate BLAS layer or new matrix abstraction was justified.

## 9. Calculus/ODEs

The scalar ODE progression is now complete for M0: Euler, explicit midpoint/RK2, and classical RK4 each expose step and solve forms. `ODEStep` rejects zero while permitting backward integration, and `ODEStepCount` requires at least one step; solver bodies are infallible after admission. A shared `y' = y` contract proves the expected one-step error ordering `RK4 < midpoint < Euler`.

## 10. Optimization

The larger Optimization package already contains gradient descent, momentum, Armijo/Wolfe line search, Gauss-Newton, Levenberg-Marquardt, Nelder-Mead, and curve fitting. Numerics contains textbook scalar golden-section and Brent minimization, including a genuine FLOW implementation. This pass documented the overlap rather than adding another solver framework.

## 11. Signal/interpolation

Signal already provides convolution, correlation, moving averages, four windows, FIR design, and spectrum helpers; Mathematics already provides a readable radix-2 FFT oracle plus builtin parity.

Interpolation gained direct O(n²) Lagrange polynomial evaluation for small grids. It rejects malformed and duplicate grids, reproduces known quadratics, and returns stored knot values exactly before basis arithmetic. A redundant two-pass cubic-spline segment scan was reduced to one clear pass.

## 12. Geometry/combinatorics

Geometry gained unit-safe `Point2D`, signed orientation, shoelace polygon area, and polygon centroid. Contracts cover turn direction, winding independence, exact rectangle values, malformed shapes, and zero-area centroid rejection. Self-intersection remains an explicit non-goal.

Combinatorics/discrete coverage lives in the existing Algorithms package through GCD/LCM, primality, sieve, and binary search rather than a new category.

## 13. Physics/SI-units examples

Physics grew beyond constants with executable mechanics equations for force, momentum, kinetic energy, gravitational potential energy, work, average power, and spring potential energy. `PositiveMass` and `PositiveDuration` are reusable refined Concepts. An `.octfail` contract proves that time cannot be passed as velocity to kinetic energy.

Geometry polygon coordinates and outputs likewise preserve `m`, `m²`, and `m³` dimensions through the formulas.

## 14. Template/modern-Oct usage

| Feature | Library/example | Why it improved clarity/safety |
| --- | --- | --- |
| `template` | `Algorithms.CountWhere<T>` alongside `FirstWhere<T>` | One readable algorithm retains exact predicate typing without per-type copies |
| Concept / `Require` | Algorithms `SieveLimit`; Distributions `Probability`, `PositiveDistributionScale`, `NonNegativeCount`; ODE `ODEStep`/`ODEStepCount`; Simulation positive config fields; Physics and Geometry refinements | Reusable scalar domains are proved statically or checked once at runtime admission |
| SI units | Physics mechanics and polygon geometry | Dimensional mistakes become type errors and derived units remain visible |
| `FLOW` | Existing `Numerics.BrentFlow` and Simulation's retained resumable example | State is explicit where iterative control/resumption is genuinely the concept |
| `batch` | `Statistics.ZScores`, `Simulation.EvaluateParameterSweep` | Independent ordered mapping is stated directly and deterministically |
| `with` | Simulation `FixedStepConfig` scenario variants | A base scenario can be refined immutably without a configuration hierarchy |

No query was introduced: none of the selected bounded algorithms naturally yielded a streaming multi-value continuation, and eager arrays/traces were clearer.

`StaticAssert` was deliberately not used: current Oct restricts it to artifact/compiled-data static evaluation. Cross-argument conditions such as `a < b`, matching array lengths, ordered grids, non-singularity, and convergence cannot be expressed as refined single-value Concepts and remain honest fallible checks.

## 15. Stub completions

- Simulation's trace-only scaffold became a usable fixed-step runner and trace-analysis library.
- Algorithms' one-function surface gained a coherent discrete foundation.
- Physics' constants-only limitation was replaced with a bounded mechanics chapter.
- Adaptive Simpson's diagnostic fields became truthful rather than placeholder-like.
- Guard-style sieve, distribution, ODE, and simulation-config domains were replaced by refined Concepts; the algorithms no longer repeat those branches.
- The unrelated `IO.WriteTable` placeholder remains explicitly documented, satisfying the stub policy without pretending an external transport was in science scope.

## 16. New discretionary science additions

- **Simulation redesign:** requested by the user and earned by the existing incomplete trace scaffold. `RunFixedStep`, `FinalStep`, `OutputIntegral`, and ordered parameter sweeps make it practically usable without competing with ODE solvers.
- **Weighted mean:** foundational for measured data and allowed a concrete numerical-stability improvement.
- **Polygon centroid:** small, unit-safe, and naturally paired with shoelace area.
- **Lagrange interpolation:** a compact executable derivation useful for small textbook grids.
- **Prime/sieve algorithms:** bounded, exact, instructional discrete science utilities.

## 17. Numerical-stability notes

- Weighted mean uses compensated sums for weighted values and weights.
- Lagrange interpolation returns exact stored knots and warns against high-degree/closely spaced grids.
- Binomial symmetry reduces coefficient work; factorial construction is avoided.
- Poisson recurrence avoids factorial overflow but may underflow in extreme tails.
- Adaptive Simpson reuses prior samples, reports actual evaluations, and exposes depth exhaustion.
- Polygon centroid rejects zero signed area instead of dividing by it.
- Existing Newton and secant derivative-denominator thresholds remain explicit.

## 18. Error-model improvements

Invalid probabilities/scales/counts, sieve bounds, ODE steps/counts, simulation config values, mass, and duration now fail at refined-Concept admission. Genuine runtime failures remain fallible: malformed or relational grids, mismatched sample shapes, invalid weights, non-monotonic traces, empty final traces, undefined polygon centroids, singular/unsafe numerical states, and non-bracketing roots. Adaptive non-convergence stays an inspectable `Converged = false` result.

## 19. Test coverage

The nine changed areas now pass 177 targeted contracts: 174 compiled cases plus three expected-failure contracts. Coverage includes analytic values, exact small cases, tolerance comparisons, invalid domains, unit errors, convergence/depth behavior, deterministic parameter order, and simulation trace invariants.

Repository gates:

- `go test ./...` — pass.
- `go vet ./...` — pass.
- `go run ./cmd/oct test Libraries --execution auto --json` — 15/15 cross-library `.octfail` contracts pass (the root target respects per-package manifest boundaries, so positive package suites were run individually).
- `git diff --check` — pass.

## 20. Interpreter/compiler parity

Each changed package was run individually with `--execution auto`; all positive cases compiled with zero interpreted fallbacks. Each changed package was also run with `--execution interpreted`. The new physics dimension-error contract passed in the invalid corpus. Execution identity was `gooct-cli` throughout. Two refined-scalar backend boundary gaps (`Pow` and `for` range endpoints) were resolved locally by extracting the admitted base value through ordinary arithmetic. A refined-array attempt produced invalid generated Go and was removed rather than accepted as fallback.

## 21. LLM readability check

A fresh agent received only Numerics integration, Interpolation source/tests, and DifferentialEquations ODE source—no internal compiler docs or language reference.

It correctly explained adaptive Simpson's panel comparison, Richardson correction, tolerance splitting, sample reuse, evaluation accounting, and all-children convergence rule. It safely added exact-knot return behavior to Lagrange interpolation without allowing a knot query to hide duplicate input, then compiled 31/31 interpolation contracts with zero fallback. It also wrote a correct `y' = -2y` midpoint example and predicted monotone positive decay to about `0.1374` at `t=1` versus exact `e^-2 ≈ 0.1353`.

Ambiguities found from isolated files were package import syntax, formal range endpoint semantics, `IntegrationResult.Iterations` naming an evaluation count, and how callers should treat a non-converged best estimate. The first two belong in language/package documentation; renaming the public field would be an unnecessary compatibility break, so the library comment and README now state its meaning.

## 22. API compatibility

Most scientific additions are additive. This follow-up intentionally modernizes several pre-1.0 signatures: `PrimesThrough`, continuous/discrete distribution evaluators, and Euler/midpoint/RK4 now accept refined Concepts and become infallible after admission. Migration is mechanical: remove trailing `!` from statically valid literal calls; for runtime data, construct `SieveLimit(raw)?`, `Probability(raw)?`, `PositiveDistributionScale(raw)?`, `NonNegativeCount(raw)?`, `ODEStep(raw)?`, or `ODEStepCount(raw)?` once before calling. Uniform's relational bounds remain fallible. `AppendStep` now rejects non-increasing time, and Adaptive Simpson corrects diagnostic values without changing its record. No dependencies were added.

## 23. What was deliberately NOT added

No symbolic algebra, automatic differentiation, sparse/BLAS framework, industrial optimizer, stiff/adaptive ODE suite, FFT duplication, distribution zoo, CAD kernel, event scheduler, plotting dependency, random nondeterministic test, native helper, or new language feature was introduced. Query was not used decoratively.

## 24. Remaining weak library areas

- Wireless still carries an explicit scalar-Hz-to-`Float<Hz>` modernization TODO.
- Mathematics.Calculus/Numerics and Numerics.Optimization/Optimization overlap and can confuse discovery.
- String, Structures, and Time remain intentionally narrow.
- Simulation is scalar and fixed-step; vector state should wait for a separate contract rather than ad-hoc nested arrays.
- Extreme-tail probability and high-degree interpolation need more stable specialized algorithms if real use cases arise.
- `IO.WriteTable` remains an explicit transport placeholder.
- Refined array Concepts with indexing currently fail generated-Go parity even though the reference describes them as supported; non-empty sample/grid refinements should wait for that compiler gap to close.

## 25. Exactly one next recommendation

Run one compatibility-first **scientific API consolidation milestone** that establishes canonical entry points and migration aliases for the Mathematics.Calculus/Numerics and Numerics.Optimization/Optimization overlaps, without deleting existing APIs.
