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
