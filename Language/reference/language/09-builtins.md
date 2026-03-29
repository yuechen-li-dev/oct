# Builtins

## Overview

This page defines core builtins used by the v1 reference.
Builtin names are reserved.
Calls are checked statically for arity and type constraints.

## Rules

- Builtin names cannot be redeclared.
- `Print(x: AnySupportedValue) -> Int`
  - Requires exactly one argument.
  - Prints the value and returns a status code `Int`.
- `Len(x: String | Int[] | Float[] | Bool[]) -> Int`.
- `Abs(x: Int | Int<D>) -> Int | Int<D>`.
- `Abs(x: Float | Float<D>) -> Float | Float<D>`.
- `Sqrt(x: Int | Float | Int<D> | Float<D>) -> Float<sqrt(D)>`.
  - Requires even dimension exponents for dimensioned input.
- `Sin(x: Int | Float) -> Float`.
  - Input must be dimensionless or an explicit degree literal.
- `Cos(x: Int | Float) -> Float`.
  - Input must be dimensionless or an explicit degree literal.
- `PlotLine(x: Int[]|Float[], y: Int[]|Float[], path: String) -> Int`.
  - Rejects dimensioned arrays.
- `PlotScatter(x: Int[]|Float[], y: Int[]|Float[], path: String) -> Int`.
  - Rejects dimensioned arrays.

## Examples

Valid:

```oct
fn Main() -> Int {
    let xs = [0, 1, 2]
    let ys = [0, 1, 4]
    return PlotScatter(xs, ys, "scatter.png")
}
```

Invalid:

```oct
fn Main() -> Int {
    return Len(1)
}
```
