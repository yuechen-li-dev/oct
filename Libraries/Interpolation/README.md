# Interpolation Library

Deterministic 1D and 2D interpolation for oct. Covers scalar lerp primitives,
piecewise linear interpolation with three boundary modes, Lagrange polynomials, natural cubic spline,
and bilinear surface interpolation.

## Components

### Lerp primitives

| Function | Description |
|---|---|
| `Lerp(a, b, t)` | Linear blend from a to b. t outside [0,1] extrapolates. |
| `LerpClamped(a, b, t)` | Same but t is clamped to [0,1]. Never extrapolates. |
| `InverseLerp(a, b, v)` | Given v in [a,b], returns t such that Lerp(a,b,t)=v. |
| `Remap(v, inMin, inMax, outMin, outMax)` | Translate v from one range to another. |

`Remap` is `Lerp(outMin, outMax, InverseLerp(inMin, inMax, v))` composed as
a single call. Use it for control signal scaling, sensor normalization, and
lookup table output remapping.

### Two-point linear interpolation

`LinearInterpolate(x0, y0, x1, y1, x)` — interpolates at x between two known
points. Rejects degenerate intervals (x0 == x1).

### Piecewise linear interpolation

Three boundary modes over a sorted knot sequence (xs, ys):

| Function | Out-of-range behavior |
|---|---|
| `PiecewiseLinear` | Returns `Error` |
| `PiecewiseLinearClamped` | Returns the nearest endpoint value |
| `PiecewiseLinearExtrapolated` | Extends the first/last segment slope |

All three share the same validation: xs must be strictly increasing, at least
two points, equal length xs and ys. Use `PiecewiseLinear` for strict lookup
tables where out-of-range is a caller error. Use `PiecewiseLinearClamped` for
signal processing and sensor fusion where saturation is the correct behavior.
Use `PiecewiseLinearExtrapolated` for physical models where the trend continues
beyond the measured range.

### Cubic spline

Natural cubic spline with C2 continuity (second derivative continuous at every
knot, zero at both endpoints).

```oct
let sp = BuildCubicSpline(xs, ys)!
let v  = EvalCubicSpline(sp, x)!
```

`BuildCubicSpline` precomputes coefficients once. `EvalCubicSpline` evaluates
at any x in the knot range. `EvalCubicSplineClamped` saturates to the endpoint
values outside the range.

For n=2 knots the spline degenerates to a line — `BuildCubicSpline` handles
this without error.

### Lagrange polynomial

`LagrangeInterpolate(xs, ys, x)` evaluates the O(n^2) basis formula directly. Three samples from `y=x^2`, for example, reconstruct the quadratic exactly. The implementation is deliberately readable and intended for small grids; high-degree or tightly spaced interpolation is numerically ill-conditioned, so splines are the better default there.

The spline is strictly more accurate than piecewise linear for smooth data.
For interpolating sin(x) at knots spaced 1 unit apart, the cubic spline error
at midpoints is an order of magnitude smaller than piecewise linear error.

### Bilinear interpolation

`BilinearUnit(q00, q10, q01, q11, tx, ty)` — interpolates on a unit square
given corner values. tx, ty are fractional positions in [0,1].

`BilinearInterpolate(x0, y0, x1, y1, q00, q10, q01, q11, x, y)` — same but
accepts real coordinates. The corners are:
- q00 at (x0, y0) — bottom-left
- q10 at (x1, y0) — bottom-right
- q01 at (x0, y1) — top-left
- q11 at (x1, y1) — top-right

Use bilinear for 2D lookup tables, terrain height queries, and image sampling.

## Boundary and validation policy

- `PiecewiseLinear` rejects x outside [xs[0], xs[n-1]] with `Error`.
- `LinearInterpolate` rejects x0 == x1 with `Error`.
- `BilinearInterpolate` rejects zero-extent dimensions with `Error`.
- `BuildCubicSpline` inherits piecewise validation (strictly increasing xs, ≥2 points).
- `LagrangeInterpolate` requires non-empty matching arrays and distinct x coordinates.
- `InverseLerp` and `Remap` reject equal endpoints with `Error`.

## Type policy

All values are dimensionless `Float`. Dimensional variants are a future
milestone once Matrix/array dimensional support matures.

## Implementation notes

The cubic spline solver uses the Thomas algorithm (tridiagonal matrix algorithm)
for the natural spline system. The backward substitution pass uses a `while`
loop with a decreasing index — the one legitimate use of `while` over `for`
in this library, since oct's `for` loops are forward-only.

## Test coverage

31 contracts covering all functions, boundary modes, error paths, the
spline accuracy comparison against piecewise linear, and the bilinear
corner/edge identity contracts.

## What is not here

Regression/curve fitting, nearest-neighbour, smoothing splines, NURBS,
Bezier curves, trigonometric interpolation, or multivariate scattered-data
interpolation. Those belong in future library expansions.
