# Core builtins

[Reference home](../README.md) · [Coverage](../coverage.md)

## Overview

Builtin names are reserved.
Calls are checked statically for arity and type constraints.

This page is intentionally limited to the **core builtin language/runtime surface**.
For practical file/data/time/text/archive/compression/hash APIs, see [standard libraries](standard-libraries.md).
For the Prometheus experimental subsystem, see [Prometheus](../runtime/prometheus.md).
For `.octest`-only assert helpers, see [octest](../tooling/testing.md).
For matrix and tensor-focused language surface, see [vectors, matrices, and tensors](vectors-matrices-and-tensors.md).

## 1) Core utilities

- `Print(x: AnySupportedValue) -> Int`
  - Requires exactly one argument.
  - Prints the value and returns status code `Int`.
- `Len(x: T[]) -> Int` and `Len(x: String) -> Int`.
- `Append(xs: T[], value: T) -> T[]`.
- `Array.CrossSection(values: T[], range: Range) -> T[]`.
  - Compiler-owned polymorphism; this does not introduce user-defined generics.
  - Accepts 1D arrays only, not scalar `String`, vectors, matrices, tensors, maps, or dictionaries.
  - Returns a new array copy and preserves the exact element type, including SI dimensions and record/enum element types.
  - It is the readable M0 replacement for Python-style slicing; `xs[1:3]` and `xs[1..3]` are not valid.
  - Negative indices, reverse ranges, lazy views, `Array.TryCrossSection`, and aliases such as `Array.Copy`, `Array.Take`, `Array.Drop`, and `Array.Window` are deferred/not part of M0.
- `Array.Where(values: T[], mask: Bool[]) -> T[]`.
  - Compiler-owned polymorphism; this does not introduce user-defined generics and does not expose `Array.Where<T>`.
  - Accepts 1D arrays with a `Bool[]` mask of equal runtime length.
  - Returns a fresh array containing values whose corresponding mask element is `true`, preserving exact element type and SI dimensions.
  - Scalar `Bool` masks and length-1 mask broadcasting are not supported; use the explicit mask operation rather than `values[mask]` indexing.
- `Vector.tabulate(length: Int, generator: fn(Int) -> T) -> Vector<T>`.
  - Eager ascending-index generation for numeric scalar `T`; explicit captures carry generator context.
- `Matrix.tabulate(rows: Int, cols: Int, generator: fn(Int, Int) -> T) -> Matrix<T>`.
  - Eager row-major generation for numeric scalar `T`; explicit captures carry generator context.

## 2) Numeric / math

- `Abs(x: Int | Int<D>) -> Int | Int<D>`.
- `Abs(x: Float | Float<D>) -> Float | Float<D>`.
- `Abs(z: Complex) -> Float`.
- `Sqrt(x: Int | Float | Int<D> | Float<D>) -> Float<sqrt(D)>`.
- `Sin`, `Cos`, `Tan`, `Asin`, `Acos`, `Atan`, `Atan2` (dimensionless constraints apply).
- `Exp`, `Ln`, `Pow`, `Log10`, `Sinh`, `Cosh`, `Tanh`.
- `Pi() -> Float` and `E() -> Float`.
- `Clamp01(x: Float) -> Float`.
  - Returns `0.0` below zero, `1.0` above one, and `x` otherwise. The argument is a dimensionless `Float`; an `Int` is not accepted.
  - `Clamp01` is a builtin name, so a program cannot declare a function called `Clamp01`.

## 3) Complex numbers

- `Complex(re: Int | Float, im: Int | Float) -> Complex`.
- `ComplexPolar(r: Int | Float, theta: Int | Float) -> Complex`.
- `I() -> Complex`, `Real(z: Complex) -> Float`, `Imag(z: Complex) -> Float`, `Conj(z: Complex) -> Complex`, `Arg(z: Complex) -> Float`.

## 4) Conversion & formatting

- `Float(value: Int) -> Float`.
  - Explicit numeric widening conversion (`Int`/`Int<D>` to `Float`/`Float<D>`).
- `FloorToInt(x: Float) -> Int`.
  - Explicit conversion using floor (`toward -infinity`).
- `CeilToInt(x: Float) -> Int`.
  - Explicit conversion using ceil (`toward +infinity`).
- `RoundToInt(x: Float) -> Int`.
  - Explicit conversion using **nearest** with **halves away from zero**.
- `BaseUnit(x: Float | Float<D>) -> Float` strips the static unit dimension and returns the unchanged numeric payload. `BaseValue` remains accepted as the older spelling.
- `ToString(value: Int | Float | Bool) -> String`.
- `String.From<T>(value) -> String` for compiler-known scalar `T` only (`Int`, `Float`, `Bool`, `String`) in interpreted and compiled execution.
- `FormatFloat(value: Float, precision: Int) -> String`.

### Conversion guidance

- `Int` is a type, **not** a conversion function.
- If you write `Int(...)`, use the diagnostic guidance and pick an explicit policy builtin:
  - `FloorToInt(x)`
  - `CeilToInt(x)`
  - `RoundToInt(x)`
- Canonical sample-count pattern:
  - `let sampleCount = FloorToInt(sampleRate * duration)`
- Use `Float(x)` only for explicit `Int -> Float` conversion.
- Use `FormatFloat(x, precision)` when display precision matters.
- Use `String.From<T>(x)` as the preferred namespaced conversion in report/library code.
- Use `FormatFloat(x, precision)` when dimensionless `Float` display precision matters.
- `String.From<T>` is a closed compiler-known conversion, separate from [user-authored templates](templates-and-selectors.md).
- `String.From<T>` intentionally does not support enums, records, arrays, or dimensioned numeric values such as `Float<K>`; unit-aware formatting is future/separate.

### Constrained compiler-known type arguments

User-authored [typed templates](templates-and-selectors.md) are explicit specializations.
Compiler-known builtins separately use explicit type arguments for closed
conversion/decoding contracts, such as `String.From<T>` and `LoadOctagon<T>`.
They do not introduce trait lookup or a general runtime generic system.

## 5) String helpers (core)

Core builtin/string surfaces include:

- `Contains`, `StartsWith`, `EndsWith`, `Trim`, `Lower`, `Upper`, `Join`
- `String.Concat(parts: String[]) -> String`

Prefer the namespaced String library authoring style from [standard libraries](standard-libraries.md) and `Libraries/String/README.md`.

## 6) What is intentionally *not* on this page

The following moved to dedicated reference pages to keep the core surface legible:

- Standard-library and wrapper-backed APIs (file/path/directory/json/csv/zip/gzip/hash/image/regex/time/xlsx/plotting).
- Backend support builtins used to implement those standard-library modules.
- Prometheus-specific surfaces (`PROMETHEUS` blocks and `PrometheusMatMul`) which are experimental and tracked separately.

## Compiled mode support

This page is **not** the compiled-support tracker.
For current compiled status (green surfaces, deferred categories, harness status), use:

- `docs/COMPILED_SUPPORT.md` (authoritative tracker)

Validate uncertain cases with explicit `oct test --execution compiled` runs against the target fixture/library.

## Preconditions

- `Require(condition)` or `Require(condition, explanation)` is a compiler-owned compile-time assertion.
- `condition` must be `Bool` and must be evaluable from the bounded compile-time expression subset documented in [Concepts](concepts-and-refinements.md).
- A true requirement is erased. A false requirement is a source-located compile diagnostic and includes `explanation` when supplied.
- A requirement depending on parameters, mutable state, arbitrary calls, I/O, time, randomness, or other runtime values is rejected as non-evaluable.
- `Require` never emits a Go call, conditional, helper, or panic. Interpreted execution observes the same erasure after type checking.
- Runtime argument validation should use ordinary `if` plus fallible `Error` handling where recovery is part of the API, or an explicit unwrap at a deliberate non-recoverable boundary.
- `Assert.True` is also available to ordinary source as the explicit non-recoverable runtime-validation compatibility path; the remaining `Assert.*` helpers stay test-only (`.octest`).

## Fourier transform

`FFT(values: Complex[]) -> Complex[] ! Error` computes the forward,
unnormalized transform with negative exponential sign. The input must be
nonempty and its length a power of two. Invalid input is a recoverable error;
handle the call just like any other fallible function. The transform preserves
input length and leaves its source array unchanged.

`Complex` inputs and outputs retain binary64 real and imaginary components on
both CPU and Vulkan. Ordinary calls calibrate complete CPU/GPU calls for the
current shape and reuse that decision within a program run. Calibration has a
first-call cost; it does not imply that every transform benefits from a GPU.
Inside a [`PROMETHEUS` block](../runtime/prometheus.md), `FFT()` requires Vulkan
execution and reports an error when that execution cannot complete. There is
no precision downgrade to the legacy Complex32 native API.

The builtin is the production forward transform. `Mathematics` keeps a readable
reference implementation and `Mathematics.IFFT` for inverse reconstruction;
see the [source contracts](../../../Libraries/Mathematics/Mathematics.Transforms.oct).
A real-only input still constructs `Complex` elements explicitly.

<!-- verified-example: ../../ReferenceExamples/valid/fourier_transform.octest -->
```oct
package ReferenceExamples

[Fact]
fn FourierTransformHasAnExplicitBoundary() -> Void ! Error {
    let impulse = [Complex(1.0, 0.0), Complex(0.0, 0.0) ... 3]
    let spectrum = FFT(impulse)?
    Assert.Equal(4, Len(spectrum), "one output for each input")
    for i in 0..Len(spectrum) {
        Assert.Equal(1.0, Real(spectrum[i]), "unit impulse has constant real spectrum")
        Assert.Equal(0.0, Imag(spectrum[i]), "unit impulse has zero imaginary spectrum")
    }
    let empty: Complex[] = []
    Assert.Error(FFT(empty), "an empty transform is rejected")
    Assert.Error(FFT([Complex(0.0, 0.0) ... 3]), "length must be a power of two")
}
```
