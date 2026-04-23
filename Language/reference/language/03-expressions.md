# Expressions

## Overview

Expressions are statically typed.
Evaluation order is deterministic.
Operators are defined only for specific operand types.
Dimensions participate in expression typing.

## Rules

- Implicit conversion is not allowed.
- Truthiness is not allowed. Conditions must be `Bool`.
- Arithmetic operators are `+`, `-`, `*`, `/`, `%`.
- Comparison operators are `==`, `!=`, `<`, `<=`, `>`, `>=`.
- Logical operators are `and`, `or`, `not`.
- Unary numeric negation is `-expr` for `Int`/`Float` expressions.
- `+` on `String` performs concatenation.
- Array arithmetic is element-wise for matching numeric array types.
- Array arithmetic requires equal element types.
- Array arithmetic requires equal runtime lengths.
- `+` and `-` require matching dimensions.
- `*` and `/` compose dimensions.
- `%` is integer-only Euclidean modulo (`Int % Int -> Int`) and requires dimensionless operands.
- Euclidean modulo guarantees `0 <= (a % b) < Abs(b)` for any non-zero integer `b`.
- `%` rejects `Float`, mixed numeric operands, and container operands.
- Comparisons require compatible operand types.
- Ordered comparison is undefined for `Bool`, `String`, `Complex`, records, enums, and arrays.
- Complex arithmetic supports `+`, `-`, `*`, `/` for `Complex` operands.
- Real numeric scalars (`Int`/`Float`) promote to `Complex` only for `+`, `-`, `*`, `/` when paired with `Complex`.
- Complex comparison supports equality only (`==`, `!=`).
- Unary `-` negates an `Int`/`Float` expression and preserves type and dimension.
- Unary `-` is distinct from binary subtraction and binds to its immediate operand.

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

Unary minus examples:

```oct
package Main

fn Main() -> Float {
    let a = 1.5
    let b = 0.25
    return -(a + b)
}
```

Euclidean modulo examples:

```oct
package Main

fn Main() -> Int {
    let a = -5 % 3
    let b = 5 % -3
    // Euclidean modulo is always non-negative for non-zero divisor.
    // This differs from C-style remainder where the sign can follow the dividend.
    return a + b
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
