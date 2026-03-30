# Mathematics M0

## Surface

`Libraries/Mathematics` provides a narrow foundational helper set:

- `Min(left: Float, right: Float) -> Float`
- `Max(left: Float, right: Float) -> Float`
- `Clamp(value: Float, lower: Float, upper: Float) -> Float`
- `Floor(value: Float) -> Float`
- `Ceil(value: Float) -> Float`
- `Round(value: Float) -> Float`
- `Pow(base: Float, exponent: Int) -> Float`
- `Sign(value: Float) -> Int`
- `Hypot(x: Float, y: Float) -> Float`
- `DifferentiateCentral(f: fn(Float) -> Float, x: Float, h: Float) -> Float ! Error`
- `IntegrateTrapezoidal(f: fn(Float) -> Float, a: Float, b: Float, n: Int) -> Float ! Error`
- `IntegrateSimpson(f: fn(Float) -> Float, a: Float, b: Float, n: Int) -> Float ! Error`

## Behavior

- `Min` / `Max` return the lower/higher argument.
- `Clamp` normalizes bounds with `Min`/`Max` first, then clamps.
- `Floor` returns the greatest integer-valued real less than or equal to the input.
- `Ceil` returns the least integer-valued real greater than or equal to the input.
- `Round` uses **half-away-from-zero** (`2.5 -> 3.0`, `-2.5 -> -3.0`).
- `Pow` uses integer exponents only. Negative exponents return reciprocals; `Pow(0.0, negative)` returns `0.0` as a sentinel.
- `Sign` returns `-1`, `0`, or `1`.
- `Hypot` computes `Sqrt(x*x + y*y)`.
- `DifferentiateCentral` uses `(f(x+h) - f(x-h)) / (2h)` and requires `h > 0`.
- `IntegrateTrapezoidal` uses the trapezoidal rule and requires `n > 0`.
- `IntegrateSimpson` uses Simpson's rule and requires `n > 0` and even `n`.
- Integration bound policy is signed and deterministic: `a == b` returns `0.0`, and `a > b` returns the negative of the corresponding `a < b` integral.

## Dimension rules

Mathematics M0 is conservative: all inputs are dimensionless `Float` except `Pow` exponent (`Int`) and `Sign` result (`Int`).
Dimensioned arguments are rejected by type checking.
Calculus M0 is scalar-only: differentiation and integration accept scalar `Float -> Float` functions and return scalar `Float` results.

## Non-goals

Mathematics.Calculus M0 does not include symbolic calculus, automatic differentiation, adaptive quadrature, multidimensional integration, ODE solvers, or root finding.
