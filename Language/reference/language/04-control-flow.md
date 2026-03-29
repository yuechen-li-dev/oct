# Control Flow

## Overview

Control flow is explicit and type-checked.
`if`, `switch`, `for`, and `while` provide branching and looping.
`switch` is expression-oriented.
Conditions never use implicit coercion.

## Rules

- `if` conditions must be `Bool`.
- `if` expressions require `else`.
- `if` expression branches must produce one result type.
- Subject `switch` compares one subject value against strict case types.
- Condition `switch` (`switch { ... }`) requires `Bool` case conditions.
- Condition `switch` requires an `else` arm.
- `switch` arms must produce one result type.
- `switch` does not fall through.
- `for i in start..end` uses inclusive start and exclusive end bounds.
- `for` bounds must be `Int`.
- `step` is optional.
- `step` must be `Int` and greater than zero.
- `while` conditions must be `Bool`.

## Examples

Valid:

```oct
fn Main() -> Int {
    var sum = 0
    for i in 0..10 step 2 {
        sum = sum + i
    }
    return sum
}
```

Invalid:

```oct
fn Main() -> Int {
    return switch {
        case true => 1
    }
}
```
