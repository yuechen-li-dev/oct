# Control Flow

## Overview

Control flow is explicit and type-checked.
`if`, `switch`, `for`, and `while` provide branching and looping.
`switch` is expression-oriented and is preferred for decision ladders.
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

Preferred when selecting between multiple outcomes:

```oct
fn ShippingTier(weight: Int) -> Int {
    return switch {
        case weight < 1 => 1
        case weight < 5 => 2
        else => 3
    }
}
```

Avoid this shape when expressing the same decision ladder:

```oct
fn ShippingTier(weight: Int) -> Int {
    if weight < 1 {
        return 1
    } else {
        if weight < 5 {
            return 2
        } else {
            return 3
        }
    }
}
```

Use local `if` blocks when they are not a ladder:

```oct
fn ClampNonNegative(v: Int) -> Int {
    if v < 0 {
        return 0
    }
    return v
}
```

## Loop selection guidance

Use `for i in start..end [step k]` for known-range and regular-step iteration.
Use `while` when loop termination depends on a condition that evolves during execution.
Using `while` to manually emulate a `for` loop is legal, but discouraged when the loop is structured iteration.

Preferred (`for` for known-range / step iteration):

```oct
fn SumEvenUnder(limit: Int) -> Int {
    var sum = 0
    for i in 0..limit step 2 {
        sum = sum + i
    }
    return sum
}
```

Legal but discouraged (`while` emulating `for`; preferred form is `for`):

```oct
fn SumEvenUnder(limit: Int) -> Int {
    var sum = 0
    var i = 0
    while i < limit {
        sum = sum + i
        i = i + 2
    }
    return sum
}
```

Preferred (`while` for condition-driven loops):

```oct
fn CountdownUntilZero(n: Int) -> Int {
    var current = n
    while current > 0 {
        current = current - 1
    }
    return current
}
```

## Examples

Valid (subject switch expression):

```oct
fn RetryBudget(code: Int) -> Int {
    return switch code {
        case 408 => 3
        case 429 => 5
        else => 0
    }
}
```

Valid (condition switch expression):

```oct
fn Segment(age: Int) -> Int {
    return switch {
        case age < 13 => 0
        case age < 18 => 1
        else => 2
    }
}
```

Valid (`for ... step`):

```oct
fn SumEvenUnder(limit: Int) -> Int {
    var sum = 0
    for i in 0..limit step 2 {
        sum = sum + i
    }
    return sum
}
```

Valid (`while`):

```oct
fn CountdownStart(n: Int) -> Int {
    var current = n
    while current > 0 {
        current = current - 1
    }
    return current
}
```

Invalid (nested decision ladder):

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
