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
