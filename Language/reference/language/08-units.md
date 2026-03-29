# Units

## Overview

Oct has a built-in SI dimension system for numeric types.
Dimensions are checked statically.
Arithmetic and comparisons enforce dimension compatibility.
Unit behavior is explicit and deterministic.

## Rules

- Base units are `m`, `kg`, `s`, `A`, `K`, `mol`, and `cd`.
- `deg` is accepted for angle literals used with trigonometric builtins.
- Only `Int` and `Float` may be dimension-qualified.
- Dimensions are part of type identity.
- `Int<m>` and `Int<s>` are different types.
- `+` and `-` require matching dimensions.
- `*` multiplies dimensions.
- `/` divides dimensions.
- Comparisons require compatible dimensions.
- Trigonometric, logarithmic, and exponential builtins that require dimensionless input reject dimensioned arguments.
- `Sqrt` requires even exponents across all dimensions.
- Implicit unit conversion is not allowed.

## Examples

Valid:

```oct
fn Main() -> Float<m> {
    return Sqrt(4m^2)
}
```

Invalid:

```oct
fn Main() -> Int {
    return 1m + 2s
}
```
