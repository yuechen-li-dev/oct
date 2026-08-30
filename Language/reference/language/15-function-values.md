# Function Values

## Overview

Oct supports named functions and anonymous function expressions as ordinary,
exactly typed function values. Anonymous functions may carry an explicit,
immutable capture environment introduced by `with`.

An anonymous function value is conceptually `code + explicit environment`.
Oct never infers that environment from lexical references.

## Syntax

```oct
fn(x: Int) -> Int {
    return x * x
}
```

```oct
fn(x: Int) -> Int with {
    factor: multiplier
    bias: offset
} {
    return x * factor + bias
}
```

Fallible anonymous functions use the ordinary fallible signature shape:

```oct
fn(value: Int) -> Int ! Error {
    if value < 0 { return error("negative") }
    return value
}
```

## Rules

- Function value type form is `fn(T1, T2, ...) -> R`.
- Named and anonymous function values use the same exact function types.
- Exact matching includes parameter types, return type, and fallibility.
- The capture environment is not part of the user-visible function type.
- Every lexical outer local or parameter used by an anonymous function must be
  named in its `with` environment. There is no implicit capture inference.
- Capture expressions are resolved in the surrounding scope and evaluated
  exactly once, from top to bottom, when the function value is constructed.
- Each captured value is a snapshot. Later reassignment of its source binding
  does not change the function's environment.
- Capture names may rename source bindings and are immutable inside the body.
- A capture name cannot duplicate another capture or a function parameter.
- Package functions, imported names, types, constants, and builtins are global
  bindings and do not need capture entries.
- Nested anonymous functions are supported. Each function independently lists
  every outer local it uses; capture is not transitive.
- Anonymous functions have no implicit self name. Named functions remain the
  direct recursion mechanism.
- Reference captures, shared mutable capture cells, and capture inference are
  not supported.

The `with` in a function signature constructs a capture environment before the
function body. It is distinct from postfix record update: `recordValue with {
Field: value }` remains an immutable `RecordUpdateExpr` and has unchanged
semantics.

## Escaping captured functions

Captured functions may outlive the invocation that constructs them because the
environment stores values, not a link to the creator's local scope:

```oct
fn MakeAdder(offset: Int) -> fn(Int) -> Int {
    return fn(value: Int) -> Int with {
        offset: offset
    } {
        return value + offset
    }
}
```

## Scientific callback example

```oct
let target = [3.0, -1.5, 2.25]
let objective = fn(point: Float[]) -> Float with {
    target: target
} {
    var loss = 0.0
    for i in 0 .. Len(point) {
        let residual = point[i] - target[i]
        loss = loss + residual * residual
    }
    return loss
}
```

The implementation may lower this to generated function code plus a statically
typed environment. Synthetic environment types are not visible in Oct source.
