# Expressions

## Overview

Expressions are statically typed.
Evaluation order is deterministic.
Operators are defined only for specific operand types.
Dimensions participate in expression typing.

## Rules

- Implicit conversion is not allowed.
- Truthiness is not allowed. Conditions must be `Bool`.
- Arithmetic operators are `+`, `-`, `*`, `/`.
- Comparison operators are `==`, `!=`, `<`, `<=`, `>`, `>=`.
- Logical operators are `and`, `or`, `not`.
- `+` on `String` performs concatenation.
- Array arithmetic is element-wise for matching numeric array types.
- Array arithmetic requires equal element types.
- Array arithmetic requires equal runtime lengths.
- `+` and `-` require matching dimensions.
- `*` and `/` compose dimensions.
- Comparisons require compatible operand types.
- Ordered comparison is undefined for `Bool`, `String`, `Complex`, records, enums, and arrays.
- Complex arithmetic supports `+`, `-`, `*`, `/` for `Complex` operands.
- Real numeric scalars (`Int`/`Float`) promote to `Complex` only for `+`, `-`, `*`, `/` when paired with `Complex`.
- Complex comparison supports equality only (`==`, `!=`).

## Examples

Valid:

```oct
package Main

fn Main() -> Int<m/s> {
    let d = 10m
    let t = 2s
    return d / t
}
```

Invalid:

```oct
package Main

fn Main() -> Int {
    if 1 { return 1 }
    return 0
}
```
