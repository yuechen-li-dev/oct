# Analysis Library

## Shelf boundary

Use `Analysis` for derivatives, accumulation, and shape features of already sampled data. Use [`Numerics`](../Numerics/README.md) to differentiate or integrate a function, [`Signal`](../Signal/README.md) for convolution/correlation, and [`Statistics`](../Statistics/README.md) for population summaries. Start with `DiffCentral`, `CumulativeTrapezoid`, and the examples below.

Numerical analysis of discrete data series for oct.

**Identity:** Analysis operates on `Float[]` arrays of observed measurements.
This is distinct from `Mathematics.Calculus` (which differentiates and
integrates mathematical functions) and `Statistics` (which computes population
descriptors). Analysis fills the space between raw data collection and
statistical summary — the operations you reach for when you have a sampled
signal and need to understand its shape, rate of change, or accumulated value.

## Components

### Finite differences

Approximate derivatives from sampled data, not from functions.

| Function | Output length | Notes |
|---|---|---|
| `DiffForward(xs, ys)` | n−1 | One-sided; uses left endpoint of each interval |
| `DiffCentral(xs, ys)` | n−2 | Interior points only; more accurate for smooth data |
| `Diff2Central(xs, ys)` | n−2 | Second derivative; exact for quadratic data |

`DiffCentral` is exact for quadratic data (zero truncation error) while
`DiffForward` has first-order error. For `y = x²`, central diff gives the
exact derivative `2x` at interior points; forward diff overshoots by `h`.

All three accept non-uniform `xs` spacing and require strictly increasing `xs`.

### Numerical integration of data

Integrate a sampled signal rather than a mathematical function.

- `IntegrateTrapezoidal(xs, ys)` — scalar total area under the curve.
  Exact for constant and linear signals. Non-uniform spacing handled correctly.
- `CumulativeIntegral(xs, ys)` — running area from `xs[0]` to each sample.
  Output length equals input length. First element is always 0.
  Last element equals `IntegrateTrapezoidal`.

Use `CumulativeIntegral` to compute displacement from a sampled velocity
signal, or energy from a sampled power signal.

### Cumulative and running operations

- `CumulativeSum(ys)` — running prefix sum. `out[i] = sum of ys[0..i]`.
- `RunningMin(ys)` — running minimum seen so far. Non-increasing.
- `RunningMax(ys)` — running maximum seen so far. Non-decreasing.

All three preserve input length and require non-empty input.

### Series normalization and transforms

| Function | Output | Use when |
|---|---|---|
| `Normalize(ys)` | Values in [0, 1] | Comparing signals on different scales |
| `Center(ys)` | Zero mean | Removing DC offset before spectral analysis |
| `Standardize(ys)` | Zero mean, unit variance (z-score) | Comparing signals with different magnitudes and scales |

`Normalize` and `Standardize` both reject constant series (range = 0 and
stddev = 0 respectively).

### Feature detection

- `LocalMaxima(ys)` — indices of local peaks. A point is a peak if it exceeds
  both neighbors. Endpoints are never returned.
- `LocalMinima(ys)` — indices of local valleys. Same convention.
- `ZeroCrossings(ys)` — indices just before each sign change. A crossing
  occurs between `i` and `i+1` when `ys[i] * ys[i+1] < 0`.
- `ArgMax(ys)` — index of the global maximum.
- `ArgMin(ys)` — index of the global minimum.

All feature detection functions return `Int[] ! Error`, erroring when no
features are found (`LocalMaxima`/`LocalMinima`/`ZeroCrossings`) or when input
is empty.

### Series comparison

- `RMSE(actual, predicted)` — root mean square error. Penalizes large errors
  more heavily than MAE.
- `MAE(actual, predicted)` — mean absolute error. Linear penalty.

`RMSE >= MAE` always (by the power mean inequality). The gap between them
indicates how much large outlier errors dominate: when RMSE ≈ MAE, errors are
uniform; when RMSE >> MAE, a few large errors dominate.

## Relationship to other libraries

| Question | Library |
|---|---|
| What is the derivative of f(x) at this point? | `Mathematics.Calculus` |
| What is the approximate derivative of this sampled signal? | `Analysis` |
| What is the mean/variance of this dataset? | `Statistics` |
| What is the running cumulative sum of this signal? | `Analysis` |
| What is the integral of f(x) from a to b? | `Mathematics.Calculus` |
| What is the area under this measured signal? | `Analysis` |

## Validation policy

All functions reject empty input. `DiffForward`, `DiffCentral`, `Diff2Central`,
`IntegrateTrapezoidal`, and `CumulativeIntegral` require strictly increasing
`xs`. `ValidatePairedSeries` is called internally wherever `xs` and `ys` must
have equal length.

## Type policy

All values are dimensionless `Float`. Dimensional variants are a future
milestone.

## Test coverage

36 contracts covering all functions, boundary conditions, exactness properties
(trapezoidal exact for linear, central diff exact for quadratic, second diff
zero for linear), error cases, and composition tests.
