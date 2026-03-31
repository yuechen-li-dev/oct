# Types

## Overview

Oct uses static, explicit types.
Primitive, array, record, and enum types are first-class.
Type identity is exact, including numeric dimensions.
Record and enum identity is nominal.

## Rules

- Primitive types are `Int`, `Float`, `Complex`, `Bool`, `String`, `UI`, `Void`, and `Error`.
- `UI` is the value type returned by UI-building builtins; it represents declarative UI composition values (not browser objects and not ad-hoc records).
- Only `Int` and `Float` may carry dimensions (`Int<m>`, `Float<m/s>`). `Complex` is always dimensionless in M0/M0a.
- Arrays are one-dimensional and homogeneous (`T[]`).
- Record identity is defined by record name.
- Enum identity is defined by enum name.
- Two records with matching fields are different types when names differ.
- Two enums with matching variants are different types when names differ.
- Array element type must match exactly, including dimensions and nominal names.
- `Void` is valid only as a function return type.

## Examples

Valid:

```oct
record Point { X: Int Y: Int }
enum Mode { Fast Slow }

fn Main() -> Int {
    let xs = [1, 2, 3]
    let p = Point { X: 1 Y: 2 }
    let m = Mode.Fast
    return xs[0] + p.X + switch m { case Mode.Fast => 1 else => 0 }
}
```

Invalid:

```oct
record A { X: Int }
record B { X: Int }

fn Main() -> A {
    let b = B { X: 1 }
    return b
}
```
