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

## Behavior

- `Min` / `Max` return the lower/higher argument.
- `Clamp` normalizes bounds with `Min`/`Max` first, then clamps.
- `Floor` returns the greatest integer-valued real less than or equal to the input.
- `Ceil` returns the least integer-valued real greater than or equal to the input.
- `Round` uses **half-away-from-zero** (`2.5 -> 3.0`, `-2.5 -> -3.0`).
- `Pow` uses integer exponents only. Negative exponents return reciprocals; `Pow(0.0, negative)` returns `0.0` as a sentinel.
- `Sign` returns `-1`, `0`, or `1`.
- `Hypot` computes `Sqrt(x*x + y*y)`.

## Dimension rules

Mathematics M0 is conservative: all inputs are dimensionless `Float` except `Pow` exponent (`Int`) and `Sign` result (`Int`).
Dimensioned arguments are rejected by type checking.
