# Units

## Overview

Oct has a built-in SI dimension system on numeric types. Dimensions are tracked by the type checker. Arithmetic and comparisons enforce dimension compatibility. Unit behavior is deterministic and explicit.

## Rules

- Base units: `m`, `kg`, `s`, `A`, `K`, `mol`, `cd`.
- `deg` is accepted for angle literals in trig usage.
- Only `Int` and `Float` may be dimension-qualified.
- Dimensions are part of type identity.
- `Int<m>` and `Int<s>` are different types.
- `+` and `-` require matching dimensions.
- `*` multiplies dimensions.
- `/` divides dimensions.
- Comparison requires compatible dimensions.
- Trig/log/exp builtins that require dimensionless input reject dimensioned arguments.
- `Sqrt` requires even exponents across dimensions.
- No implicit unit conversion exists.

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
