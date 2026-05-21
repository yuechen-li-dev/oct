# Builtins

## Overview

Builtin names are reserved.
Calls are checked statically for arity and type constraints.

This page is intentionally limited to the **core builtin language/runtime surface**.
For practical file/data/time/text/archive/compression/hash APIs, see [17 standard libraries](./17-standard-libraries.md).
For the Prometheus experimental subsystem, see [23 Prometheus](../runtime/23-prometheus.md).
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
- `Pow(base: Int | Float, exponent: Int | Float) -> Float`.
  - Both inputs must be dimensionless.
  - Runtime behavior follows IEEE754/Go `math.Pow` semantics (e.g. `0^0 == 1`, domain edge-cases may return `NaN`/`+Inf`).
  - `^` operator syntax is not part of this surface; use `Pow(base, exponent)`.
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
  - Generic `Float -> Int` conversion is intentionally not provided.
- `FloorToInt(x: Float) -> Int`.
  - Explicit conversion using floor (`toward -infinity`).
  - Examples: `FloorToInt(2.9) == 2`, `FloorToInt(0.0 - 2.1) == -3`.
- `CeilToInt(x: Float) -> Int`.
  - Explicit conversion using ceil (`toward +infinity`).
  - Examples: `CeilToInt(2.1) == 3`, `CeilToInt(0.0 - 2.9) == -2`.
- `RoundToInt(x: Float) -> Int`.
  - Explicit conversion using round-to-nearest with halves away from zero (Go `math.Round`).
  - Examples: `RoundToInt(3.4) == 3`, `RoundToInt(3.5) == 4`, `RoundToInt(0.0 - 3.5) == 0 - 4`.
- `BaseValue(x: Float | Float<D>) -> Float`.
  - Explicitly strips unit annotation and returns canonical/base-unit numeric value.
  - Dimensionless `Float` is accepted and returned unchanged.
- `ToString(value: Int | Float | Bool) -> String`.
- `String.From<T>(value) -> String` for compiler-known `T` only (`Int`, `Float`, `Bool`, `String` in M0).
- `FormatFloat(value: Float, precision: Int) -> String`.
  - Preferred when fixed decimal precision is required.

### Conversion vs formatting guidance

- Use `ToString(x)` for plain explicit conversion.
- Use `FormatFloat(x, precision)` when display precision matters.
- Use `Float(x)` only for explicit `Int -> Float` conversion.
- `Int` is a type, not a conversion function.
- Use `FloorToInt(x)`, `CeilToInt(x)`, or `RoundToInt(x)` for explicit float-to-int rounding policy.
- For sample counts, `let sampleCount = FloorToInt(sampleRate * duration)` is usually intended.
- Use `BaseValue(x)` only when intentionally discarding units.
- No implicit numeric/string conversion is performed in concatenation or other expressions.
- `String.From<T>` requires an explicit type argument and does not introduce user-defined generic support.

## 5) String helpers

- `Contains(s: String, part: String) -> Bool`.
- `StartsWith(s: String, prefix: String) -> Bool`.
- `EndsWith(s: String, suffix: String) -> Bool`.
- `Trim(s: String) -> String`.
- `Lower(s: String) -> String`.
- `Upper(s: String) -> String`.
- `Join(parts: String[], sep: String) -> String`.

## 6) What is intentionally *not* on this page

The following moved to dedicated reference pages to keep the core surface legible:

- Standard-library and wrapper-backed APIs (file/path/directory/json/csv/zip/gzip/hash/image/regex/time/xlsx/plotting).
- Backend support builtins used to implement those standard-library modules.
- Prometheus-specific surfaces (`PROMETHEUS` blocks and `PrometheusMatMul`) which are experimental and tracked separately.

### Plotting note

- `PlotLine` and `PlotScatter` remain builtin convenience plotting helpers and do **not** require any library import.
- For advanced plotting controls (size/labels/histogram), use `Plot.Core` from [17 standard libraries](./17-standard-libraries.md).

## Compiled mode support (core surface)

Source of truth used here:
- compiled parity corpus/tests in `internal/build/compiler_test.go`
- compiled lowering support in `internal/build/compiler.go`

Current status (for this core-builtin page scope):

- **Supported in compiled mode:**
  - `Print`, `Len`, `Append`
  - `ToString`, `Float`, `FormatFloat`
  - string helpers: `Contains`, `StartsWith`, `EndsWith`, `Trim`, `Lower`, `Upper`, `Join`
- **Supported with restrictions in compiled mode:**
  - `Abs` on scalar `Int`/`Float`-family values supported; unsupported argument types are rejected.
- **Interpreter-only / deferred in compiled mode (known from corpus and compiler diagnostics):**
  - math family not explicitly lowered in compiled mode (`Sqrt`, trig, exp/log, hyperbolic)
  - complex-number family (`Complex`, `ComplexPolar`, `I`, `Real`, `Imag`, `Conj`, `Arg`)

When in doubt, treat the parity corpus as SSOT and validate against `internal/build/compiler.go`.


## Preconditions

- `Require(condition, message)` enforces non-recoverable runtime preconditions in production code.
- `message` is mandatory and must explain the violated invariant.
- If `condition` is false, execution fails immediately with a runtime error including `message`.
- Use `Require` for programmer errors/invalid arguments, not for recoverable failures.
- Recoverable failures remain `! Error` values.
- `Assert.*` remains test-only (`.octest`).

Example:

```oct
fn RollDie(rng: Rng, sides: Int) -> DieRollResult {
    Require(sides >= 2, "RollDie requires sides >= 2")

    let r = Random.RandInt(rng, 1, sides)
    return DieRollResult {
        Next: r.Next
        Value: r.Value
    }
}
```
