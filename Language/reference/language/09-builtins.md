# Builtins

## Overview

This page defines core builtins used by the v1 reference.
Builtin names are reserved.
Calls are checked statically for arity and type constraints.

## Rules

- Builtin names cannot be redeclared.
- `Print(x: AnySupportedValue) -> Int`
  - Requires exactly one argument.
  - Prints the value and returns a status code `Int`.
- `Len(x: String | Int[] | Float[] | Bool[] | Complex[]) -> Int`.
- `Append(xs: T[], value: T) -> T[]`.
  - First argument must be an array.
  - Value type must exactly match the array element type.
- `Abs(x: Int | Int<D>) -> Int | Int<D>`.
- `Abs(x: Float | Float<D>) -> Float | Float<D>`.
- `Abs(z: Complex) -> Float`.
- `Complex(re: Int | Float, im: Int | Float) -> Complex`.
  - Both arguments must be dimensionless.
- `ComplexPolar(r: Int | Float, theta: Int | Float) -> Complex`.
  - Both arguments must be dimensionless.
- `I() -> Complex`.
- `Real(z: Complex) -> Float`.
- `Imag(z: Complex) -> Float`.
- `Conj(z: Complex) -> Complex`.
- `Arg(z: Complex) -> Float`.
  - Returns principal argument in radians with range `[-Pi(), Pi()]`.
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
  - Both inputs must be dimensionless.
- `Exp(x: Int | Float) -> Float`.
  - Input must be dimensionless.
- `Exp(z: Complex) -> Complex`.
- `Ln(x: Int | Float) -> Float`.
  - Input must be dimensionless.
  - Runtime input domain is positive values only.
- `Ln(z: Complex) -> Complex`.
  - Uses principal logarithm with `Im(Ln(z)) = Arg(z)` in `[-Pi(), Pi()]`.
- `Log10(x: Int | Float) -> Float`.
  - Input must be dimensionless.
  - Runtime input domain is positive values only.
- `Sinh(x: Int | Float) -> Float`.
  - Input must be dimensionless.
- `Cosh(x: Int | Float) -> Float`.
  - Input must be dimensionless.
- `Tanh(x: Int | Float) -> Float`.
  - Input must be dimensionless.
- `Pi() -> Float`.
  - Requires zero arguments.
- `E() -> Float`.
  - Requires zero arguments.
- `FormatFloat(value: Float, precision: Int) -> String`.
  - Preferred when rendering display text from floating-point values.
  - Use this when UI/status strings need fixed precision.
- `Float(value: Int) -> Float`.
  - Explicit numeric widening conversion.
  - Converts `Int` (including dimensioned `Int<D>`) to `Float` (`Float<D>`).
  - `Float -> Int` conversion is intentionally not provided in the current language.
- `ToString(value: Int | Float | Bool) -> String`.
  - Plain explicit conversion for common scalar display paths.
  - Does not apply formatting policy beyond standard value rendering.
- `Contains(s: String, part: String) -> Bool`.
  - Returns true when `part` occurs in `s`.
- `StartsWith(s: String, prefix: String) -> Bool`.
  - Returns true when `s` begins with `prefix`.
- `EndsWith(s: String, suffix: String) -> Bool`.
  - Returns true when `s` ends with `suffix`.
- `Trim(s: String) -> String`.
  - Removes leading and trailing whitespace.
- `Lower(s: String) -> String`.
  - Lowercases text for lightweight UI normalization.
- `Upper(s: String) -> String`.
  - Uppercases text for lightweight UI normalization.
- `Join(parts: String[], sep: String) -> String`.
  - Concatenates `parts` with `sep` inserted between elements.
- `PlotLine(x: Int[]|Float[], y: Int[]|Float[], path: String) -> Int`.
  - Rejects dimensioned arrays.
- `PlotScatter(x: Int[]|Float[], y: Int[]|Float[], path: String) -> Int`.
  - Rejects dimensioned arrays.
- `Assert.True(condition: Bool, message: String) -> Void`.
- `Assert.False(condition: Bool, message: String) -> Void`.
- `Assert.Equal(expected: T, actual: T, message: String) -> Void`.
- `Assert.Near(expected: Float, actual: Float, tolerance: Float, message: String) -> Void`.
  - Argument order is `expected`, then `actual`, then tolerance/message.

### Conversion vs formatting guidance

Use `ToString(x)` for explicit plain conversion (`Int`, `Float`, `Bool`).
Use `FormatFloat(x, precision)` when you need fixed float formatting.
Use `Float(x)` for explicit `Int -> Float` conversion only.
`Float -> Int` is intentionally unavailable in this version.
No implicit conversion is performed in concatenation or other expressions.

Examples:
- `Float(3)` -> `3.0`
- `Float(3m) / 2.0` -> `1.5m`

### String helper examples

- `Contains("storefront", "front")` -> `true`
- `StartsWith("status: ok", "status")` -> `true`
- `EndsWith("status: ok", "ok")` -> `true`
- `Trim("  ready  ")` -> `"ready"`
- `Lower("MiXeD")` -> `"mixed"`
- `Upper("MiXeD")` -> `"MIXED"`
- `Join(["desk", "gear", "sale"], " / ")` -> `"desk / gear / sale"`

## Examples

Valid:

```oct
package Main

fn Main() -> Int {
    let theta = Pi() / 4
    let xs = [1, 2]
    let ys = Append(xs, 3)
    let label = "theta=" + ToString(theta)
    let precise = "theta_fixed=" + FormatFloat(theta, 3)
    let canonical = Lower(Trim("  READY  "))
    let slug = Join(["desk", "gear", "sale"], " / ")
    Print(ToString(Contains(slug, "gear")))
    Print(ToString(StartsWith(canonical, "rea")))
    Print(ToString(EndsWith(canonical, "dy")))
    Print(label)
    Print(precise)
    Print(Atan2(1, 1) + Tan(theta))
    return Len(ys)
}
```

Invalid:

```oct
package Main

fn Main() -> Float {
    return Ln(0)
}
```
