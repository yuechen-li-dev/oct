# Builtins

## Overview

Builtin names are reserved.
Calls are checked statically for arity and type constraints.

This page is intentionally limited to the **core builtin language/runtime surface**.
For practical file/data/time/text/archive/compression/hash APIs, see [17 standard libraries](./17-standard-libraries.md).
For the Prometheus experimental subsystem, see [23 Prometheus](../runtime/23-prometheus.md).
For `.octest`-only assert helpers, see [31 octest](../tooling/31-octest.md).
For matrix and tensor-focused language surface, see [16 vectors, matrices, and tensors](./16-vectors-and-matrices.md); [tensors](../tensors.md) is retained as a compatibility pointer.

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

## 2) Numeric / math

- `Abs(x: Int | Int<D>) -> Int | Int<D>`.
- `Abs(x: Float | Float<D>) -> Float | Float<D>`.
- `Abs(z: Complex) -> Float`.
- `Sqrt(x: Int | Float | Int<D> | Float<D>) -> Float<sqrt(D)>`.
- `Sin`, `Cos`, `Tan`, `Asin`, `Acos`, `Atan`, `Atan2` (dimensionless constraints apply).
- `Exp`, `Ln`, `Pow`, `Log10`, `Sinh`, `Cosh`, `Tanh`.
- `Pi() -> Float` and `E() -> Float`.

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
- `BaseValue(x: Float | Float<D>) -> Float`.
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
- `String.From<T>` requires explicit type arguments but does **not** introduce user-defined generics.
- `String.From<T>` intentionally does not support enums, records, arrays, or dimensioned numeric values such as `Float<K>`; unit-aware formatting is future/separate.

### Constrained compiler-known type arguments

Oct does not support user-defined generics.
A few compiler-owned builtins may use explicit type arguments for closed conversion/decoding contracts.
Current examples in the reference surface are `String.From<T>` and `ReadOctagon<T>`.

## 5) String helpers (core)

Core builtin/string surfaces include:

- `Contains`, `StartsWith`, `EndsWith`, `Trim`, `Lower`, `Upper`, `Join`
- `String.Concat(parts: String[]) -> String`

Prefer the namespaced String library authoring style from [17 standard libraries](./17-standard-libraries.md) and `Libraries/String/README.md`.

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

- `Require(condition, message)` enforces non-recoverable runtime preconditions in production code.
- `message` is mandatory and must explain the violated invariant.
- If `condition` is false, execution fails immediately with a runtime error including `message`.
- Use `Require` for programmer errors/invalid arguments, not for recoverable failures.
- Recoverable failures remain `! Error` values.
- `Assert.*` remains test-only (`.octest`).
