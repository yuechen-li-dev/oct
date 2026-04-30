# Statistics M0

## Surface

`Libraries/Statistics` provides descriptive statistics over finite `Float[]` collections:

- `Mean(xs: Float[]) -> Float`
- `Variance(xs: Float[]) -> Float`
- `StandardDeviation(xs: Float[]) -> Float`
- `Median(xs: Float[]) -> Float`
- `Percentile(xs: Float[], p: Float) -> Float`

## Definitions

- `Variance` uses the **population** definition: `sum((x - mean)^2) / n`.
- `StandardDeviation` is `Sqrt(Variance(xs))`.
- `Median` sorts ascending; for even length it returns the average of the two middle values.
- `Percentile` accepts `p` in `[0, 100]` and uses linear interpolation on rank `r = (p/100) * (n-1)`.

## Edge-case policy

- Empty input is rejected with `Error` for all functions.
- Single-element input is valid:
  - `Mean`, `Median`, and `Percentile` return that value.
  - `Variance` and `StandardDeviation` return `0.0`.

## Dimension behavior

Statistics M0 currently targets `Float[]` (dimensionless scalars) only.
Dimensioned or mixed-dimension arrays are rejected by typing and are covered by invalid tests.
No function silently drops units.
