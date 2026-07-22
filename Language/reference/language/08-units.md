# Units

## Overview

Oct has a built-in SI dimension system for numeric types.
Dimensions are checked statically.
Arithmetic and comparisons enforce dimension compatibility.
Unit behavior is explicit and deterministic.

## Rules

- Base units are `m`, `kg`, `s`, `A`, `K`, `mol`, `cd`, `px`, and `ui`.
- `px` is the absolute placement/sizing unit used by Machina UI absolute layout builtins.
- `ui` is the normalized anchored coordinate unit used by Machina UI anchored layout builtins.
- `deg` is accepted for angle literals used with trigonometric builtins.
- `C` is accepted as an authoring-only literal suffix for absolute temperature and lowers to Kelvin (`0C == 273.15K`).
- Only `Int` and `Float` may be dimension-qualified.
- Dimensions are part of type identity.
- `Int<m>` and `Int<s>` are different types.
- Arrays carry dimensions per element type (`Float<m>[]` is a sequence of measured values).
- Vectors/matrices carry dimensions per element type as mathematical objects (`Vector<Float<m>>`, `Matrix<Float<kg/s^2>>`).
- `+` and `-` require matching dimensions.
- `*` multiplies dimensions.
- `/` divides dimensions.
- Matrix multiplication `@` on dimensioned linear objects propagates dimensions at compile time:
  - `Matrix<Float<D1>> @ Vector<Float<D2>> -> Vector<Float<D1*D2>>`
  - `Matrix<Float<D1>> @ Matrix<Float<D2>> -> Matrix<Float<D1*D2>>`
- Comparisons require compatible dimensions.
- Trigonometric, logarithmic, and exponential builtins that require dimensionless input reject dimensioned arguments.
- `Sqrt` requires even exponents across all dimensions.
- Implicit unit conversion is not allowed.
- `C` is not a base unit name and is not valid in type-level unit expressions (for example `Float<C>` is invalid).
- Unit exponents are signed integers (for example `s^2`, `s^-1`, `m*s^-2`).
- Use inverse-unit notation with signed exponents (for example `Float<s^-1>`); `<1/s>` is not supported in this milestone.
- `Hz` is a named alias for `s^-1` and is dimensionally compatible with `Float<s^-1>`.
- Transparent concepts may give a unit-bearing concrete type a domain name, for example `concept Speed = Float<m/s>`. The name adds no conversion, wrapper, allocation, or runtime object; dimensional arithmetic remains authoritative.

## Unit stripping

`BaseUnit(value)` strips the static unit dimension from a `Float` value and returns the bare `Float` numeric payload. It is a builtin, not a namespaced package function, and does not require type arguments. The older spelling `BaseValue(value)` remains accepted.

`BaseUnit` does **not** convert units, choose a display unit, or format a value. It only erases the type-level dimension after normal literal/lowering semantics have produced the stored numeric value. For example, `BaseUnit(300.0K)` returns `300.0`; Celsius literals first follow the existing Celsius-to-Kelvin literal semantics, so `BaseUnit(0C)` returns `273.15`.

```oct
package Main

fn TemperaturePayload(t: Float<K>) -> Float {
    return BaseUnit(t)
}

fn LengthPayload(x: Float<m>) -> Float {
    return BaseUnit(x)
}
```

`BaseUnit` accepts `Float<D>` and dimensionless `Float` values. It rejects strings, records, arrays, vectors, matrices, and other non-`Float` values. Dimensioned `Int<D>` stripping is not part of the v0.1 contract.

## Examples

Valid:

```oct
package Main

fn Main() -> Float<m> {
    return Sqrt(4m^2)
}
```

```oct
package Main

fn Main() -> Float<Hz> {
    let f: Float<Hz> = 60.0Hz
    let same: Float<s^-1> = f
    return same
}
```

```oct
package Main

fn Main() -> Float<K> {
    return 180C
}
```

```oct
package Main

fn Main() -> Vector<Float<kg*m/s^2>> {
    let stiffness = Matrix.tabulate(2, 2, DiagonalStiffness)
    let displacement = vector[4.0m, 5.0m]
    return stiffness @ displacement
}

fn DiagonalStiffness(r: Int, c: Int) -> Float<kg/s^2> {
    if r != c { return 0.0kg/s^2 }
    if r == 0 { return 2.0kg/s^2 }
    return 3.0kg/s^2
}
```

Invalid:

```oct
package Main

fn Main() -> Int {
    return 1m + 2s
}
```
