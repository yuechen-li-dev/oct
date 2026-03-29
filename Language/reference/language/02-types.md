# Types

## Overview

Oct uses static, explicit types. Primitive, array, record, and enum types are first-class. Type identity is exact, including dimensions on numeric types. No structural equivalence is applied for records or enums.

## Rules

- Primitive types: `Int`, `Float`, `Bool`, `String`, `Void`, `Error`.
- Only `Int` and `Float` may carry dimensions (`Int<m>`, `Float<m/s>`).
- Arrays are one-dimensional and homogeneous (`T[]`).
- Record identity is nominal by record name.
- Enum identity is nominal by enum name.
- Two records with matching fields are still different types if names differ.
- Two enums with matching variants are still different types if names differ.
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
