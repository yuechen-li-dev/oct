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
- One-dimensional numeric arrays support narrow scalar broadcast for `+`, `-`, `*`, and `/` when exactly one operand is a scalar: `T[] op S` and `S op T[]` produce an array with the same length as the array operand. The element type and unit dimension are exactly the result of applying the existing scalar operation to one element and the scalar. For example, `Float<m>[] * Float<s>` produces `Float<m*s>[]`, and `Float<m>[] / Float<s>` produces `Float<m/s>[]`.
- One-dimensional numeric arrays support scalar comparisons (`==`, `!=`, `<`, `<=`, `>`, `>=`) when exactly one operand is a scalar. The result is `Bool[]` with the same length as the array operand.
- Array-array operations remain strict element-wise operations over equal lengths. A length-1 array is not a scalar.
- Oct does not perform rank broadcasting, shape inference, silent reshaping, or NumPy-style broadcasting for arrays. `Float[][]` shapes such as `(n, 1)` and `(1, m)` are not combined into `(n, m)`.
- Logical mask filtering is not part of array operators; use a future `Array.Where`-style API when that exists.
- Array arithmetic requires equal element types.
- Array arithmetic requires equal runtime lengths.
- `+` and `-` require matching dimensions.
- `*` and `/` compose dimensions.
- `%` is integer-only Euclidean modulo (`Int % Int -> Int`) and requires dimensionless operands.
- Euclidean modulo guarantees `0 <= (a % b) < Abs(b)` for any non-zero integer `b`.
- `%` rejects `Float`, mixed numeric operands, and container operands.
- Comparisons require compatible operand types.
- Ordered comparison is undefined for `Bool`, `String`, `Complex`, records, enums, and array-array operands. Numeric array-scalar comparisons are element-wise and return `Bool[]`.
- Complex arithmetic supports `+`, `-`, `*`, `/` for `Complex` operands.
- Real numeric scalars (`Int`/`Float`) promote to `Complex` only for `+`, `-`, `*`, `/` when paired with `Complex`.
- Complex comparison supports equality only (`==`, `!=`).
- Unary `-` negates an `Int`/`Float` expression and preserves type and dimension.
- Unary `-` is distinct from binary subtraction and binds to its immediate operand.

## Range expressions

`Range` is a compiler-owned immutable value type. Range expressions are ordinary expressions wherever the typechecker allows `Range` values, including `let` bindings, function arguments, function returns, and record fields.

M0 range expression forms are:

```oct
start..end
start..
..end
..
start..end step n
```

Present `start`, `end`, and `step` expressions must be `Int`. Omitted endpoints are preserved in the `Range` value and resolved by the consumer. An omitted step means the default step is `1`; consumers require positive steps. Open-ended stepped ranges are deferred in M0, so these forms are invalid: `start.. step n`, `..end step n`, and `.. step n`.

Range values do not add bracket slicing. `xs[1..3]` and Python-like colon slicing are not part of range expressions. Use consumer APIs such as `Array.CrossSection(values, range)` for 1D array cross-section copies.

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
