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
- `+` and `-` require matching dimensions.
- `*` multiplies dimensions.
- `/` divides dimensions.
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

Invalid:

```oct
package Main

fn Main() -> Int {
    return 1m + 2s
}
```
