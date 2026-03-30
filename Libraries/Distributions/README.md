# Distributions M0

## Scope

`Libraries/Distributions` is a deterministic **distribution evaluation** library.
It does **not** provide sampling, inference, fitting, significance tests, or any RNG surface.

## Function surface

- `NormalPdf(x: Float, mean: Float, sigma: Float) -> Float ! Error`
- `NormalCdf(x: Float, mean: Float, sigma: Float) -> Float ! Error`
- `UniformPdf(x: Float, a: Float, b: Float) -> Float ! Error`
- `UniformCdf(x: Float, a: Float, b: Float) -> Float ! Error`
- `ExponentialPdf(x: Float, lambda: Float) -> Float ! Error`
- `ExponentialCdf(x: Float, lambda: Float) -> Float ! Error`

## Parameter and domain policy

- M0 is **dimensionless `Float` inputs only**.
- Invalid distribution parameters reject with `Error`:
  - `sigma <= 0` for Normal
  - `a >= b` for Uniform
  - `lambda <= 0` for Exponential
- Out-of-support values use mathematical distribution behavior:
  - `UniformPdf` outside `[a, b]` returns `0.0`
  - `UniformCdf` returns `0.0` for `x < a`, `1.0` for `x > b`, and linear interpolation on `[a, b]`
  - `ExponentialPdf` and `ExponentialCdf` return `0.0` for `x < 0`

Boundary behavior is explicit:
- `UniformPdf(a, a, b)` and `UniformPdf(b, a, b)` return `1 / (b - a)`
- `UniformCdf(a, a, b) = 0.0`
- `UniformCdf(b, a, b) = 1.0`

## Normal CDF approximation

`NormalCdf` is implemented via the identity `Φ(z) = 0.5 * (1 + erf(z / sqrt(2)))` and a compact deterministic approximation for `erf` (Abramowitz & Stegun 7.1.26).
This keeps M0 small while providing stable practical accuracy for routine numeric workflows.
