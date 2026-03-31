# Errors

## Overview

Error handling is explicit and typed.
Function fallibility is declared in function signatures.
Every fallible expression must be handled.
Handling forms are `?`, `!`, and `match`.

`?` is preferred when you only need propagation.
`match` is preferred when `ok` and `err` need different local behavior.

## Rules

- A function is fallible only when its return type includes `! Error`.
- Fallible expressions cannot be ignored.
- `?` propagates `err` to the current fallible function.
- `?` is invalid in an infallible function.
- `?` requires a fallible expression.
- `match expr { ok(v) => ... err(e) => ... }` requires a fallible expression.
- Fallible `match` must include both `ok` and `err` arms.
- `!` unwrap is explicit handling for a fallible expression.
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
    return match ParseRetries(raw) {
        ok(v) => v
        err(_) => 3
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
    return match ParsePercent(raw) {
        ok(v) => if v >= 90 { 2 } else { 1 }
        err(_) => 0
    }
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
    return match ReadTimeout(raw) {
        ok(v) => v
        err(_) => 30
    }
}
```
