# Functions

## Overview

Functions are declared with explicit parameter and return types. Calls are checked for arity and argument types. Return behavior is explicit. Function fallibility is part of the signature.

## Rules

- Declaration form: `fn Name(params) -> ReturnType { ... }`.
- Fallible declaration form: `fn Name(...) -> T ! Error { ... }`.
- Parameter types are required.
- Return type is required.
- Non-`Void` functions must return a value on all paths.
- `Void` functions return with `return` or by reaching end of body.
- Calls must pass exactly the declared number of arguments.
- Each argument type must match the declared parameter type.
- Built-in function names cannot be redeclared.

## Examples

Valid:

```oct
fn Add(x: Int, y: Int) -> Int {
    return x + y
}

fn Main() -> Int {
    return Add(1, 2)
}
```

Invalid:

```oct
fn Add(x: Int, y: Int) -> Int {
    return x + y
}

fn Main() -> Int {
    return Add(1)
}
```
