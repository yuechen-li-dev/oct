# Functions

## Overview

Function signatures are explicit.
Parameter and return types are required.
Calls are checked for arity and argument types.
Fallibility is part of the function signature.

## Rules

- Declaration form is `fn Name(params) -> ReturnType { ... }`.
- Fallible declaration form is `fn Name(...) -> T ! Error { ... }`.
- Every parameter has an explicit type.
- Every function has an explicit return type.
- Non-`Void` functions must return a value on every path.
- `Void` functions may return with `return` or by reaching the end of the body.
- Calls must provide exactly the declared number of arguments.
- Each argument type must match the corresponding parameter type.
- Builtin names cannot be redeclared.

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
