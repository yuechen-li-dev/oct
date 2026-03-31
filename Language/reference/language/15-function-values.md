# Function Values

## Overview

Oct supports typed function values through named functions.
Function value parameters use explicit `fn(...) -> ...` types.
Function values are passed by function name.

## Rules

- Function value type form is `fn(T1, T2, ...) -> R`.
- Function value parameters are declared in ordinary function signatures.
- Passing a function value uses a named function identifier.
- Anonymous functions and lambda expressions are not supported.
- Function value arguments must match the expected signature exactly.
- Exact match includes parameter types, return type, and fallibility.
- Non-function values cannot be called as functions.

## Examples

Valid:

```oct
package Main

fn Double(x: Int) -> Int {
    return x * 2
}

fn ApplyOne(x: Int, f: fn(Int) -> Int) -> Int {
    return f(x)
}

fn Main() -> Int {
    return ApplyOne(4, Double)
}
```

Invalid:

```oct
package Main

fn ApplyOne(x: Int, f: fn(Int) -> Int) -> Int {
    return f(x)
}

fn Main() -> Int {
    return ApplyOne(4, fn(y: Int) -> Int { return y * 2 })
}
```
