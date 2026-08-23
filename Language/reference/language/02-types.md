# Types

## Overview

Oct uses static, explicit types.
Primitive, array, record, enum, and concept-described value shapes are first-class.
Type identity is exact, including numeric dimensions.
Record and enum identity is nominal.
Bounded template applications are monomorphized to ordinary exact types before type checking and execution.

## Rules

- Primitive and compiler-owned builtin types are `Int`, `Float`, `Complex`, `Bool`, `String`, `Bytes`, `Range`, `UI`, `Void`, and `Error`.
- `UI` is an opaque builtin type for declarative UI composition.
- `UI` values are produced/consumed by UI library functions.
- `UI` is not a browser object and not a normal record you can reshape with fields.
- `Bytes` is a narrow binary transport/storage boundary type intended for wrapper-backed compatibility APIs (for example file byte I/O).
- `Bytes` is not a dynamic object container and does not imply `Dynamic` semantics.
- `Range` is a compiler-owned immutable value produced by range expressions; see `03-expressions.md`.
- Only `Int` and `Float` may carry dimensions (`Int<m>`, `Float<m/s>`). `Complex` is always dimensionless in M0/M0a.
- Arrays are homogeneous containers (`T[]`, `T[][]`, ...).
- Record identity is defined by record name.
- Enum identity is defined by enum name.
- Two records with matching fields are different types when names differ.
- Two enums with matching variants are different types when names differ.
- Two applications of one template with different concrete type arguments are distinct nominal types.
- Array element type must match exactly, including dimensions and nominal names.
- `Void` is valid only as a function return type.
- A named value concept is a transparent name for an existing concrete type.
- A record-shaped concept is nominal by concept name and uses ordinary record value semantics.
- See [18 Concepts](./18-concepts.md) for the bounded Concepts-M0 surface.

## Examples

Valid:

```oct
package Main

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
package Main

record A { X: Int }
record B { X: Int }

fn Main() -> A {
    let b = B { X: 1 }
    return b
}
```
