# Control Flow

## Overview

Control flow is explicit and type-checked.
`if`, `switch`, `for`, and `while` provide branching and looping.
`switch` is expression-oriented.
Conditions never use implicit coercion.

## Rules

- `if` conditions must be `Bool`.
- `if` statement form may omit `else`.
- `if` expression form requires `else`.
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

### Decision ladder policy

- Nested `else { if ... }` decision ladders are rejected.
- This is the nested decision-ladder rule.
- The replacement form is condition-switch (`switch { case ... else ... }`).
- Ordinary nested local `if` blocks are still valid when they are not decision ladders.

## Examples

Valid:

```oct
fn Main() -> Int {
    var total = 0

    if true {
        total = 1
    }

    let pick = if total > 0 { 10 } else { 20 }
    return pick
}
```

Invalid:

```oct
fn Main() -> Int {
    if true {
        return 1
    } else {
        if false {
            return 2
        } else {
            return 3
        }
    }
}
```
