# Builtins

## Overview

Builtin names are reserved.
Calls are checked statically for arity and type constraints.

This page documents the user-facing builtin surface and its organization.
For `.octest`-only assert helpers, see [31 octest](../tooling/31-octest.md).
For matrix and tensor-focused language surface, see [16 vectors and matrices](./16-vectors-and-matrices.md) and [tensors](../tensors.md).

## 1) Core utilities

- `Print(x: AnySupportedValue) -> Int`
  - Requires exactly one argument.
  - Prints the value and returns status code `Int`.
- `Len(x: T[]) -> Int` and `Len(x: String) -> Int`.
  - `Len` accepts any array element type `T`.
- `Append(xs: T[], value: T) -> T[]`.
  - First argument must be an array.
  - Value type must match array element type.

## 2) Numeric / math

- `Abs(x: Int | Int<D>) -> Int | Int<D>`.
- `Abs(x: Float | Float<D>) -> Float | Float<D>`.
- `Abs(z: Complex) -> Float`.
- `Sqrt(x: Int | Float | Int<D> | Float<D>) -> Float<sqrt(D)>`.
  - Requires even dimension exponents for dimensioned input.
- `Sin(x: Int | Float) -> Float`.
  - Input must be dimensionless or an explicit degree literal.
- `Cos(x: Int | Float) -> Float`.
  - Input must be dimensionless or an explicit degree literal.
- `Tan(x: Int | Float) -> Float`.
  - Input must be dimensionless or an explicit degree literal.
- `Asin(x: Int | Float) -> Float`.
  - Input must be dimensionless.
  - Runtime input domain is `[-1, 1]`.
- `Acos(x: Int | Float) -> Float`.
  - Input must be dimensionless.
  - Runtime input domain is `[-1, 1]`.
- `Atan(x: Int | Float) -> Float`.
  - Input must be dimensionless.
- `Atan2(y: Int | Float, x: Int | Float) -> Float`.
  - Inputs must be dimensionless.
- `Exp(x: Int | Float) -> Float`.
  - Input must be dimensionless.
- `Exp(z: Complex) -> Complex`.
- `Ln(x: Int | Float) -> Float`.
  - Input must be dimensionless.
  - Runtime domain is positive values.
- `Ln(z: Complex) -> Complex`.
  - Uses principal logarithm with `Im(Ln(z)) = Arg(z)` in `[-Pi(), Pi()]`.
- `Log10(x: Int | Float) -> Float`.
  - Input must be dimensionless.
  - Runtime domain is positive values.
- `Sinh(x: Int | Float) -> Float`.
  - Input must be dimensionless.
- `Cosh(x: Int | Float) -> Float`.
  - Input must be dimensionless.
- `Tanh(x: Int | Float) -> Float`.
  - Input must be dimensionless.
- `Pi() -> Float` and `E() -> Float`.

## 3) Complex numbers

- `Complex(re: Int | Float, im: Int | Float) -> Complex`.
  - Arguments must be dimensionless.
- `ComplexPolar(r: Int | Float, theta: Int | Float) -> Complex`.
  - Arguments must be dimensionless.
- `I() -> Complex`.
- `Real(z: Complex) -> Float`.
- `Imag(z: Complex) -> Float`.
- `Conj(z: Complex) -> Complex`.
- `Arg(z: Complex) -> Float`.

## 4) Conversion & formatting

- `Float(value: Int) -> Float`.
  - Explicit numeric widening conversion (`Int`/`Int<D>` to `Float`/`Float<D>`).
  - `Float -> Int` conversion is intentionally not provided.
- `ToString(value: Int | Float | Bool) -> String`.
- `FormatFloat(value: Float, precision: Int) -> String`.
  - Preferred when fixed decimal precision is required.

### Conversion vs formatting guidance

- Use `ToString(x)` for plain explicit conversion.
- Use `FormatFloat(x, precision)` when display precision matters.
- Use `Float(x)` only for explicit `Int -> Float` conversion.
- No implicit numeric/string conversion is performed in concatenation or other expressions.

## 5) String helpers

- `Contains(s: String, part: String) -> Bool`.
- `StartsWith(s: String, prefix: String) -> Bool`.
- `EndsWith(s: String, suffix: String) -> Bool`.
- `Trim(s: String) -> String`.
- `Lower(s: String) -> String`.
- `Upper(s: String) -> String`.
- `Join(parts: String[], sep: String) -> String`.

## 6) Plotting

- `PlotLine(x: Int[]|Float[], y: Int[]|Float[], path: String) -> Int`.
  - Rejects dimensioned arrays.
- `PlotScatter(x: Int[]|Float[], y: Int[]|Float[], path: String) -> Int`.
  - Rejects dimensioned arrays.

Notes:
- Plotting is implemented via a thin runtime wrapper over `gonum/plot`, not as a core language primitive.
- Compiled-mode behavior depends on compiled runtime wrapper support.
- Current compiled parity corpus records plotting as not yet supported in compiled mode.

## 7) Data / Octagon I/O

- `LoadOctagon<T>(path: String) -> T[]`.
- `WriteOctagon<T>(path: String, data: T[]) -> Int`.

## Compiled Mode Support (derived from corpus + compiler implementation)

Source of truth used here:
- compiled parity corpus/tests in `internal/build/compiler_test.go`
- compiled lowering support in `internal/build/compiler.go`

Current status (for this builtin page scope):

- **Supported in compiled mode:**
  - `Print`, `Len`, `Append`
  - `ToString`, `Float`
  - string helpers: `Contains`, `StartsWith`, `EndsWith`, `Trim`, `Lower`, `Upper`, `Join`
  - matrix constructor surface (see note in [16 vectors and matrices](./16-vectors-and-matrices.md) for corpus-verified vs code-implemented split)
  - `LoadOctagon`, `WriteOctagon`
- **Supported with restrictions in compiled mode:**
  - `Abs` on scalar `Int`/`Float`-family values supported; unsupported argument types are rejected.
- **Interpreter-only / deferred in compiled mode (known from corpus and compiler diagnostics):**
  - math family not explicitly lowered in compiled mode (`Sqrt`, trig, exp/log, hyperbolic)
  - complex-number family (`Complex`, `ComplexPolar`, `I`, `Real`, `Imag`, `Conj`, `Arg`)
  - plotting builtins (`PlotLine`, `PlotScatter`)

When in doubt, treat the parity corpus as SSOT and validate against `internal/build/compiler.go`.
