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
