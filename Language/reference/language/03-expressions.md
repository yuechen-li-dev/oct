# Expressions

## Overview

Expressions are statically typed and evaluated deterministically. Operators are defined only for specific operand types. Boolean logic is explicit. Numeric dimensions participate in expression typing.

## Rules

- No implicit conversions are performed.
- No truthiness exists. Conditions must be `Bool`.
- Arithmetic operators: `+`, `-`, `*`, `/`.
- Comparison operators: `==`, `!=`, `<`, `<=`, `>`, `>=`.
- Logical operators: `and`, `or`, `not`.
- `+` on `String` is string concatenation.
- Array arithmetic is element-wise for matching numeric array types.
- Array arithmetic requires equal element types and equal lengths at runtime.
- Addition and subtraction require matching dimensions.
- Multiplication and division compose dimensions.
- Comparisons require compatible operand types.
- Ordered comparison is not defined for `Bool`, `String`, records, enums, or arrays.

## Examples

Valid:

```oct
fn Main() -> Int<m/s> {
    let d = 10m
    let t = 2s
    return d / t
}
```

Invalid:

```oct
fn Main() -> Int {
    if 1 { return 1 }
    return 0
}
```
