# Errors

## Overview

Error handling is explicit and typed.
Function fallibility is declared in function signatures.
Every fallible expression must be handled.
Handling forms are `?`, `!`, and fallible `match`.

`?` is preferred when you only need propagation.
Fallible `match` remains available when `ok` and `err` need different local behavior.
Note: enum variant payload binding uses the enum `match` form documented in [12 Enums](./12-enums.md); that is a separate control-flow surface.

## Rules

- A function is fallible only when its return type includes `! Error`.
- Fallible expressions cannot be ignored.
- `?` propagates `err` to the current fallible function.
- `?` is invalid in an infallible function.
- `?` requires a fallible expression.
- Fallible `match expr { ok(v) => { ... } err(e) => { ... } }` requires a fallible expression (`->` is also accepted for the arm arrow).
- Fallible `match` is a statement. It does not produce a value: each arm is a block, and an arm returns, assigns to a variable declared before the `match`, or falls through to the statement after it.
- Fallible `match` must include both arms, `ok` first and `err` second.
- An arm that does not need its binding discards it with `_`: `ok(_)` or `err(_)`.
- `!` unwrap is explicit handling for a fallible expression.
- A fallible call may stand as a statement with `?` or `!`: `Save(path, value)?`.
  The call is made for its effect, its value, if it has one, is discarded, and
  with `?` its error goes to the caller. This is how a `Void ! Error` call is
  propagated; it has no value to bind.
- Returning a fallible value from an infallible function is invalid.

## Examples

Valid (`?` propagation):

```oct
package Main

fn ReadPort() -> Int ! Error {
    return 443
}

fn Main() -> Int ! Error {
    let p = ReadPort()?
    return p
}
```

Valid (`match` on fallible expression):

```oct
package Main

fn ParseRetries(raw: String) -> Int ! Error {
    if raw == "0" {
        return 0
    }
    if raw == "1" {
        return 1
    }
    return error("invalid retries")
}

fn RetriesOrDefault(raw: String) -> Int {
    match ParseRetries(raw) {
        ok(v) => { return v }
        err(_) => { return 3 }
    }
}
```

Valid (`match` is clearer than `?` when branching):

```oct
package Main

fn ParsePercent(raw: String) -> Int ! Error {
    if raw == "95" {
        return 95
    }
    if raw == "40" {
        return 40
    }
    return error("invalid percent")
}

fn Bucket(raw: String) -> Int {
    var bucket = 0
    match ParsePercent(raw) {
        ok(v) => {
            if v >= 90 {
                bucket = 2
            } else {
                bucket = 1
            }
        }
        err(_) => {
            bucket = 0
        }
    }
    return bucket
}
```

Valid (`!` in context):

```oct
package Main

fn ParsePort(raw: String) -> Int ! Error {
    if raw == "8080" {
        return 8080
    }
    return error("invalid port")
}

fn MustPort() -> Int {
    let p = ParsePort("8080")!
    return p
}
```

Invalid (fallible handling in infallible function):

```oct
package Main

fn Fail() -> Int ! Error {
    return error("bad")
}

fn Main() -> Int {
    let x = Fail()?
    return x
}
```

This is legal, but the preferred form is `match` when you need an in-function fallback:

```oct
package Main

fn ReadTimeout(raw: String) -> Int ! Error {
    if raw == "10" {
        return 10
    }
    return error("invalid timeout")
}

fn TimeoutOr(raw: String) -> Int {
    let t = ReadTimeout(raw)!
    return t
}
```

Preferred correction:

```oct
package Main

fn ReadTimeout(raw: String) -> Int ! Error {
    if raw == "10" {
        return 10
    }
    return error("invalid timeout")
}

fn TimeoutOr(raw: String) -> Int {
    match ReadTimeout(raw) {
        ok(v) => { return v }
        err(_) => { return 30 }
    }
}
```
