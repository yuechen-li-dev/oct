# Builtins

## Overview

This page lists core builtins used in the v1 language reference. Builtins are reserved names. Calls are statically checked for arity and type constraints.

## Rules

- Builtin names cannot be redeclared.
- `Print(x: AnySupportedValue) -> Int`
  - accepts one argument
  - prints value and returns status code `Int`
- `Len(x: String | Int[] | Float[] | Bool[]) -> Int`
- `Abs(x: Int | Int<D>) -> Int | Int<D>`
- `Abs(x: Float | Float<D>) -> Float | Float<D>`
- `Sqrt(x: Int | Float | Int<D> | Float<D>) -> Float<sqrt(D)>`
  - requires even dimension exponents for dimensioned input
- `Sin(x: Int | Float) -> Float`
  - input must be dimensionless scalar or explicit degree literal
- `Cos(x: Int | Float) -> Float`
  - input must be dimensionless scalar or explicit degree literal
- `PlotLine(x: Int[]|Float[], y: Int[]|Float[], path: String) -> Int`
  - rejects dimensioned arrays
- `PlotScatter(x: Int[]|Float[], y: Int[]|Float[], path: String) -> Int`
  - rejects dimensioned arrays

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
