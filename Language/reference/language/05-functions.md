# Functions

## Async functions (ASYNC-M0)

Prefix `async` declares a resumable function and prefix `await` suspends until
an async/FLOW computation completes:

```oct
async fn FetchThenProcess(A: Int) -> Int {
    let X = A + 1
    let Y = await Delayed(40)
    return X + Y
}
```

The written return type is the eventual result type. Calling an async function
constructs the existing FLOW-instance handle; it does not execute the body
synchronously. M0 permits `await` only as the complete initializer of a
`let`/`var`, directly on an `async fn` or `flow` call. Arbitrary values are not
awaitable.

The compiler lowers async source to ordinary Octomata states, persistent
fields, `Step`, `Complete`, `Result`, `goto`, `suspend`, and `return`. Generated
names are deterministic and visible in MIR dumps. See
[21 Octomata](../runtime/21-octomata.md) and
[`docs/internal/async_m0.md`](../../../docs/internal/async_m0.md).

M0 rejects await in loops, stored-handle await, async recursion, fallible or
generic async functions, `return await`, and async generators. It provides no
scheduler, cancellation, timeout, stream, join/race, Future trait, or custom
awaiter protocol.

## Overview

Function signatures are explicit.
Parameter and return types are required.
Calls are checked for arity and argument types.
Fallibility is part of the function signature.

## Rules

- Declaration form is `fn Name(params) -> ReturnType { ... }` (source also accepts `=>` as the arrow spelling).
- Fallible declaration form is `fn Name(...) -> T ! Error { ... }` (source also accepts `=>` as the arrow spelling).
- Every parameter has an explicit type.
- Every function has an explicit return type.
- Non-`Void` functions must return a value on every path.
- `Void` functions may return with `return` or by reaching the end of the body.
- Calls must provide exactly the declared number of arguments.
- Each argument type must match the corresponding parameter type.
- Builtin names cannot be redeclared.
- Reusable exact-typed functions may use `template fn Name<T>(...)`; see [19 Parametric templates](./19-parametrics.md).

## OctGo companion imports

An OctGo `*.contracts.oct` companion may declare a selected free Go function
with the narrow bodyless form:

```oct
go fn StrictlyAbove(value: Int, threshold: Int) -> Bool
```

This is an OctGo host binding, not a general foreign-function declaration.
The declaration is valid only in `*.contracts.oct`, preserves the same Go and
Oct name, is non-fallible, and has no Oct implementation body. The OctGo host
must validate the exported Go function and exact supported `go/types`
signature before deriving static wrapper metadata. Imported calls are
compiled-only and cannot be evaluated by compile-time `Require`.

## Examples

Valid:

```oct
package Main

fn Add(x: Int, y: Int) -> Int {
    return x + y
}

fn Main() -> Int {
    return Add(1, 2)
}
```

Invalid:

```oct
package Main

fn Add(x: Int, y: Int) -> Int {
    return x + y
}

fn Main() -> Int {
    return Add(1)
}
```
