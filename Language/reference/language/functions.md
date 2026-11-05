# Functions

[Reference home](../README.md) · [Coverage](../coverage.md)

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
- Each argument type must match the corresponding parameter type. A declared `Float` takes an `Int` as that `Float`; see "Declared types and numeric values" in [Types](types.md).
- Builtin names cannot be redeclared.
- Reusable exact-typed functions may use `template fn Name<T>(...)`; see [Parametric templates](templates-and-selectors.md).

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

## Async functions (ASYNC-M0)

For resumable functions, see [Async functions](../runtime/async.md). Start with
ordinary synchronous functions unless the computation must suspend.

## Entry functions and multiple results

Use `Main` as the entry function of an executable package so it works with
both `oct run` and `oct build`. The native backend also accepts lowercase
`main`, but the interpreter currently requires `Main`. Tests instead select
attributed functions. Ordinary functions can call named functions
declared later and can recurse; there are no methods, overload sets, variadic
parameters, default arguments, or tuple return syntax in this source surface.
Use a named concept or record to return several related values.
