# Interpolation M0

## Scope

`Libraries/Interpolation` provides deterministic **1D linear interpolation only**:

- `Lerp(a: Float, b: Float, t: Float) -> Float`
- `LinearInterpolate(x0: Float, y0: Float, x1: Float, y1: Float, x: Float) -> Float ! Error`
- `PiecewiseLinear(xs: Float[], ys: Float[], x: Float) -> Float ! Error`

Out of scope for M0: splines, regression/fitting, smoothing, nearest-neighbor, multidimensional interpolation.

## Boundary policy

`PiecewiseLinear` uses an explicit **reject out-of-range** policy:

- if `x < xs[0]` or `x > xs[last]`, it returns `Error`
- exact knot queries (`x == xs[i]`) return `ys[i]`

## Type and dimension policy

Interpolation M0 currently targets **dimensionless `Float` values only**.
This keeps behavior explicit and avoids unit-erasing conversions.
Any non-`Float` argument (including incompatible dimensions) is rejected by typing.

## Invalid-input policy

- `LinearInterpolate` rejects degenerate intervals (`x0 == x1`)
- `PiecewiseLinear` rejects:
  - empty inputs
  - mismatched lengths
  - fewer than 2 points
  - non-strictly-increasing `xs` (including duplicates)
  - out-of-range queries
