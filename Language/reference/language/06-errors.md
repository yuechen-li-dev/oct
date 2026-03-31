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
fn ParseRetries(raw: String) -> Int ! Error {
    return parseInt(raw)
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
fn ParsePercent(raw: String) -> Int ! Error {
    return parseInt(raw)
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
fn MustPort() -> Int {
    let p = parseInt("8080")!
    return p
}
```

Invalid (fallible handling in infallible function):

```oct
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
fn ReadTimeout(raw: String) -> Int ! Error {
    return parseInt(raw)
}

fn TimeoutOr(raw: String) -> Int {
    let t = ReadTimeout(raw)!
    return t
}
```

Preferred correction:

```oct
fn ReadTimeout(raw: String) -> Int ! Error {
    return parseInt(raw)
}

fn TimeoutOr(raw: String) -> Int {
    return match ReadTimeout(raw) {
        ok(v) => v
        err(_) => 30
    }
}
```
