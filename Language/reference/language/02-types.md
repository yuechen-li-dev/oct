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
- Array element type must match exactly, including dimensions and nominal names. The one exception is a declared `Float` or `Complex`; see "Declared types and numeric values" below.
- `Void` is valid only as a function return type.
- A named value concept is a transparent name for an existing concrete type.
- A record-shaped concept is nominal by concept name and uses ordinary record value semantics.
- See [18 Concepts](./18-concepts.md) for the bounded Concepts-M0 surface.

## Declared types and numeric values

A declared type decides what a value is. `let x: Float = 1` declares a
`Float` equal to one; `let x = 1` declares nothing, and `x` is the `Int` the
literal is.

- Where a declaration says `Float`, an `Int` value is that `Float`.
- Where a declaration says `Complex`, an `Int` or a `Float` value is that
  `Complex`, with no imaginary part.
- The same holds for a collection of the same shape: an `Int[]` where
  `Float[]` is declared, at any depth, and a `Vector<Int>` or `Matrix<Int>`
  where the `Float` one is declared. Each element is converted and the
  collection it came from is unchanged. Collections do not convert to
  `Complex`.
- Dimensions are not converted. An `Int<m>` is a `Float<m>` where one is
  declared; an `Int` is not.
- Nothing converts the other way. A `Float` is never an `Int`.

The places that declare a type, and so decide the value that reaches them:

| Place | Example |
|---|---|
| A binding with a type | `let x: Float = 1`, `var total: Float = 0` |
| An assignment to a variable, an element, a row or a cell | `x = 1`, `xs[i] = 1`, `rows[i] = [1, 1]`, `rows[i, j] = 1` |
| An argument, by the parameter's type | `Half(1)` for `fn Half(x: Float)` |
| A result, by the function's return type | `return 1` in `fn One() -> Float` |
| An element of an array literal that is itself declared | `let xs: Float[] = [1, 2.5]` |
| A record field, a table column and a `with` replacement | `Sample { Level: 1 }`, `sample with { Level: 1 }` |
| An enum payload | `Reading.Level(1)` for `Level(Float)` |
| A flow parameter, board field, turn input, `yield` and result | `board.Level = 1` for `Level: Float` |

An assignment has the type of what it assigns to, however that came by its
type: `var x = 1.5` is a `Float` variable, and `x = 1` assigns the `Float`.

An array literal that nothing declares does not mix: `[1, 2.5]` alone is an
error. A builtin states its own argument types, and a generic one takes its
element type from its arguments: `Append(xs, 1)` on a `Float[]` is an error.

The interpreter and compiled execution convert at the same places and give
the same values.

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
