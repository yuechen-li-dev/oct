# Control Flow

## Overview

Control flow is explicit and type-checked.
`if`, `switch`, `match`, `for`, and `while` provide branching and looping.
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
- `match` is expression-only and is used for enum variant analysis with optional payload binding.
- Enum `match` must be exhaustive.
- `switch`/`when` arrow positions accept both `->` and `=>`; no semantic distinction is attached to spelling.
- `for i in start..end` uses inclusive start and exclusive end bounds.
- `for` ranges must be closed: both `start` and `end` are required. Open-ended `Range` values are ordinary expressions but are not valid `for` loop ranges in M0.
- `for` bounds must be `Int`.
- `step` is optional.
- `step` must be `Int` and greater than zero.
- `flow`, `state`, and `step` are contextual keywords: they remain structural in `flow` declarations, `state` declarations, and `for .. step ..` clauses, but are valid as ordinary identifiers in non-ambiguous binding/reference positions.
- `while` conditions must be `Bool`.

### Decision surfaces by context

Outside flow state bodies, use ordinary program control flow:

- `if`
- `switch`
- `match` (enum payload binding and variant-sensitive branching)
- `when utility` (expression form)

Inside `flow/state` bodies, use Octomata decision surfaces:

- guard `when { ... -> ... }`
- `when policy { ... } { ... }`

When a flow-only form is used outside a flow state, diagnostics should steer you back to `switch` or `when utility`.

Role split:

- `switch`: literal/value dispatch and tag-only enum branching.
- `match`: associated-data enum payload binding.
- `when`: utility/policy and Octomata decision surfaces.

### Decision ladder policy

- Nested `else { if ... }` decision ladders are rejected.
- This is the nested decision-ladder rule.
- The replacement form is condition-switch (`switch { case ... else ... }`).
- Ordinary nested local `if` blocks are still valid when they are not decision ladders.

Preferred when selecting between multiple outcomes:

```oct
package Main

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
package Main

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
package Main

fn ClampNonNegative(v: Int) -> Int {
    if v < 0 {
        return 0
    }
    return v
}
```

## Loop selection guidance

Use `for i in start..end [step k]` for ascending known-range and regular-step iteration. Use `for i in start..end descend k` for explicit descending iteration; bare `descend` is shorthand for `descend 1`. Although `Range` expressions may omit endpoints in expression positions, `for` loops require closed ranges in M0.

`for` ranges are half-open in both directions: ascending loops visit `start` while `i < end`, and descending loops visit `start` while `i > end`. Ascending `step k` and descending `descend k` require a positive `Int` magnitude. `step` and `descend` are mutually exclusive; negative `step` is invalid, so write `descend <positive magnitude>` when counting down. Ascending loops require `start <= end`; descending loops require `start >= end`. Equal bounds produce zero iterations.

Use `while` when loop termination depends on a condition that evolves during execution.
Using `while` to manually emulate a `for` loop is legal, but discouraged when the loop is structured iteration.

Preferred (`for` for known-range / step iteration):

```oct
package Main

fn SumEvenUnder(limit: Int) -> Int {
    var sum = 0
    for i in 0..limit step 2 {
        sum = sum + i
    }
    return sum
}
```


Preferred (`for` for descending half-open iteration):

```oct
package Main

fn SumDown() -> Int {
    var sum = 0
    for i in 5..0 descend 1 {
        sum = sum + i
    }
    return sum // 15: visits 5, 4, 3, 2, 1
}
```

Legal but discouraged (`while` emulating `for`; preferred form is `for`):

```oct
package Main

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
package Main

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
package Main

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
package Main

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
package Main

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
package Main

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
package Main

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

## Unsupported loop-control words and explicit loop state

`continue` and `break` are not loop-control keywords in Oct. A bare statement
spelled `continue` or `break` is rejected with a dedicated diagnostic; these
spellings are not globally reserved as ordinary identifiers in expression and
binding contexts.

For simple loop skipping, keep the skip condition explicit with a guard:

```oct
package Main

fn SumExceptTwo() -> Int {
    var sum = 0
    for i in 0..5 {
        if i != 2 {
            sum = sum + i
        }
    }
    return sum
}
```

For early termination, put the stop condition in the `while` condition when that
is the natural shape, or use `Loop.Stop(state)` with first-party Loop state
helpers when the loop position should remain visible as data.

`Loop` M0 is an explicit helper library, not a hidden iterator or generator
protocol. It supports increasing integer ranges only, half-open `[Start, End)`
bounds, positive steps only, explicit immutable record rebinding, interpreted
and compiled execution, and no persistence/checkpointing. `Loop.Range(0, 0)` and
`Loop.Range(3, 0)` are inactive in M0. `Loop.RangeStep` rejects zero or negative
steps.

```oct
package Main

import Loop

fn SumRange() -> Int {
    var loop = Loop.Range(0, 5)
    var sum = 0

    while Loop.IsActive(loop) {
        sum = sum + Loop.Current(loop)
        loop = Loop.Advance(loop)
    }

    return sum
}
```

Early stop remains an explicit state transition:

```oct
package Main

import Loop

fn SumBeforeThree() -> Int {
    var loop = Loop.Range(0, 5)
    var sum = 0

    while Loop.IsActive(loop) {
        let i = Loop.Current(loop)
        if i == 3 {
            loop = Loop.Stop(loop)
        } else {
            sum = sum + i
            loop = Loop.Advance(loop)
        }
    }

    return sum
}
```

Skip-body patterns should use a guard for simple loops, or
`Loop.Advance(state)` when the loop position is explicit helper state:

```oct
package Main

import Loop

fn ShouldSkip(i: Int) -> Bool {
    return i == 2
}

fn SumWithSkip() -> Int {
    var loop = Loop.Range(0, 5)
    var sum = 0

    while Loop.IsActive(loop) {
        let i = Loop.Current(loop)
        if not ShouldSkip(i) {
            sum = sum + i
        }
        loop = Loop.Advance(loop)
    }

    return sum
}
```

`Loop.Advance` is intentionally distinct from Octomata `resume`: `Advance` is a
pure record-state transition to the next loop position, while Octomata `resume`
jumps to a remembered flow state. For resumable or generator-like local flows
that need explicit control-state history, board state, `remember`, or `resume`,
use Octomata rather than ordinary loop keywords.
