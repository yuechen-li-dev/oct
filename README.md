# Oct v0

Oct is a compiled-first, statically typed, function-first language for scientific programming.
Oct v0 focuses on a small, explicit core: native arrays, explicit error handling, SI dimensions, and PNG plotting.

## Oct v0 feature set

- Packages are explicit: each `.oct` file must start with `package Name`, followed by zero or more `import Name` lines.
- Package imports are Oct-native and explicit (no aliases/wildcards), and cross-package calls are qualified (`Geometry.Distance()`).
- Package resolution rule (v0): one directory = one package, and imports resolve to sibling directories under the program root.
- Program entrypoint is `Main.Main()`.
- Static base types: `Int`, `Float`, `Bool`, `String`, and built-in `Error`.
- Native one-dimensional arrays: `Int[]`, `Float[]`, `Bool[]`, including array arithmetic and indexing.
- Control flow: `if`/`else`, `for i in 0..n` with optional `step`, and expression `switch` with literal `case` arms plus required `else`.
- Built-ins: `Len`, `Abs`, `Sqrt`, `Sin`, `Cos`, `PlotLine`, `PlotScatter`, and `error("message")`.

## Error handling model

Fallible functions declare `! Error` in their signature.

- `?` propagates an `Error` from a fallible expression and is only valid inside a fallible function.
- `!` unwraps a fallible expression and crashes with a fatal runtime error when it contains `Error`.
- `match` handles fallible values explicitly:
  - `ok(value) => { ... }`
  - `err(e) => { ... }`
- Error values are constructed with `error("message")`.

Error results are never silently ignored: they must be handled with `?`, `!`, or `match`.

## SI dimensions and units

Oct supports SI base dimensions on numeric types (`Int` and `Float`):

- `m` (meter), `kg` (kilogram), `s` (second), `A` (ampere), `K` (kelvin), `mol` (mole), `cd` (candela)
- Dimensioned literals such as `5m`, `2.5s`, and derived dimensions like `m/s` or `m^2`.
- Dimensions participate in type identity, so mismatched dimensions are type errors.
- Addition/subtraction require matching dimensions.
- Multiplication/division compose dimensions.
- `Sin` and `Cos` require dimensionless input.
- No implicit unit conversion exists in v0.

## Plotting

Oct v0 provides two plotting built-ins:

- `PlotLine(x, y, path)`
- `PlotScatter(x, y, path)`

Plotting contract:

- Output path must end with `.png` (PNG-only).
- `x` and `y` must be arrays of numeric, dimensionless values.
- Dimensioned arrays are rejected.
- `x` and `y` must be non-empty and have equal length.

## M22a proof package: Numerics (Root Finding v0)

M22a adds a native `Numerics` package in Oct source under `testdata/m22a/valid/Numerics` with:

- root-finding methods: `Bisection` and `NewtonRaphson`
- structured solver configuration via `SolveOptions`
- structured outcomes via `RootResult`
- explicit termination status via `RootReason`
- a small fixed problem selector (`ProblemSqrt2()` / `ProblemCubic()`) for `x*x - 2` and `x*x*x - x - 2`

This milestone is a proof that package-based Oct code can express real iterative numerical algorithms with records/enums, loops, convergence checks, explicit fallible errors, and cross-package reuse from `Main`.

Implementation friction observed in M22a:

- Without first-class functions, solver APIs use explicit problem selectors and package-owned residual/derivative functions.
- Cross-package enum value flow is still awkward in v0, so this proof package uses integer problem IDs while keeping a dedicated termination enum for package status design.

## M22b proof package: Mechanics (Unit-Safe 2D Statics / Kinematics Core)

M22b adds a native `Mechanics` package in Oct source under `testdata/m22b/valid/Mechanics` with:

- domain records: `Vec2Force`, `Displacement2`, `Stiffness2`, and `EquilibriumReport`
- enum-driven helpers via `Axis` and `DominantAxis`
- unit-safe 2D statics helpers: `AddForce`, `MagnitudeSquared`, and `ResultantReport`
- a matrix/vector-backed mechanics helper: `InternalForce` computes `K @ d` for a 2x2 stiffness-like model

This milestone is a proof that packages, records/enums, SI dimensions, comparisons/logical flow, and matrix/vector operations compose naturally in small mechanics-oriented Oct code.

Implementation friction observed in M22b:

- A single generic `Vec2<T>` shape is still not available, so this proof uses explicit domain-specific records (`Vec2Force`, `Displacement2`).
- Matrix shape constraints are runtime/value-level rather than encoded in the type, so API documentation and tests carry shape intent (`2x2` with `2x1`).

## M22c proof package: Analysis (Data -> Compute -> Plot Workflow v0)

M22c adds a native `Analysis` package in Oct source under `testdata/m22c/valid/Analysis` with:

- structured workflow output via `AnalysisResult { X, Y, Z }`
- one linear end-to-end function: `GenerateAndCompute(start, increment)`
- explicit array-oriented analysis flow for a small fixed-size dataset (`x = start + i*increment`, `y = x*x - 2`, `z = x*x`)
- direct plotting from package-produced arrays using built-in `PlotLine`/`PlotScatter` in `Main`

This milestone is a proof that Oct can express a readable data -> compute -> structured result -> plot flow with packages, records, loops, mutable locals, indexing, `Len`, and plotting.

Implementation friction observed in M22c:

- v0 arrays do not support dynamic append, so fixed-size preallocation is still required for loop-driven datasets.

## M22cr proof refresh: Analysis (Post-M23 Array Ergonomics Recheck)

M22cr refreshes the same `Analysis` package and workflow after M23 array element assignment landed.

- `GenerateAndCompute` now fills arrays directly inside the loop with `x[i] = ...`, `y[i] = ...`, and `z[i] = ...`.
- the workflow shape remains intentionally unchanged: generate data -> compute values -> return `AnalysisResult` -> plot from `Main`.
- tests continue to validate result correctness, plotting output generation, package integration/build behavior, and now include a proof-oriented check that the package source uses direct element assignment.

Ergonomics assessment (M22cr):

- **Material improvement:** yes. The core population loop is now straightforward Oct code and no longer relies on whole-array reassignment helpers.
- **Earlier awkwardness removed:** mostly yes for fixed-size workflows; direct indexed writes make the proof package read naturally.
- **Residual friction:** meaningful but narrower. Dynamic-growth workflows remain awkward because append/push is still unavailable, so this proof continues using fixed-size arrays.

## M22d proof package: Signal (Small Signal Processing Workflow v0)

M22d adds a native `Signal` package in Oct source under `testdata/m22d/valid/Signal` with:

- structured workflow output via `SignalAnalysisResult { X, Raw, Smoothed }`
- deterministic sample generation via `GenerateSignalValues()`
- explicit moving-average filtering via `MovingAverage(values, window)`
- one end-to-end analysis function via `AnalyzeSignal(start, delta, window)` that prepares axis values, applies filtering, and returns a record
- direct plotting from package-produced arrays in `Main` using `PlotLine`

This milestone is a proof that Oct can express a small, readable signal workflow (generate -> smooth -> return structured arrays -> plot) using packages, records, loops, mutable locals, indexing, array assignment, `Len`, and strings for output paths.

Implementation friction observed in M22d:

- The workflow is clean for fixed-size arrays, but dynamic dataset growth still requires predeclared sizing because append/push is not available in v0.

## M22e proof package: Structures (Small Structural / Truss-Lite v0)

M22e adds a native `Structures` package in Oct source under `testdata/m22e/valid/Structures` with:

- structured element input via `BarElement2D { Area, YoungsModulus, Length }`
- structured output via `AxialElementResponse { Stiffness, Displacement, InternalForce }`
- a matrix-producing mechanics helper `AxialStiffness(element)` for a 2x2 axial bar element local matrix (`AE/L` form)
- a matrix @ vector response path `AxialForceFromDisplacement(element, displacement)`
- an end-to-end package API `AnalyzeAxialElement(element, u1, u2)` used by `Main`

This milestone is a proof that Oct can express a small, physically meaningful structural computation with packages, records, SI dimensions, matrices, vectors, and clear engineering formulas.

Implementation friction observed in M22e:

- Matrix/vector shape intent is still validated by usage/tests instead of being encoded at the type level (fixed 2-entry displacement for a 2x2 local stiffness workflow).
