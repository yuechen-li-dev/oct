# Mathematics M1

## Calculus compatibility and canonical ownership

`Numerics` is the canonical entry point for numerical differentiation and quadrature. `Mathematics.DifferentiateCentral`, `Mathematics.IntegrateTrapezoidal`, and `Mathematics.IntegrateSimpson` remain source-compatible textbook names and now delegate to the corresponding `Numerics` implementations. Their historical zero-width and reversed-interval behavior is preserved.

Use `Mathematics` for elementary helpers and transforms. Use `Numerics` for method choice, convergence controls, diagnostics, roots, differentiation, quadrature, and bounded scalar minimization.

## Surface

`Libraries/Mathematics` provides a narrow deterministic numerical surface:

- `Min(left: Float, right: Float) -> Float`
- `Max(left: Float, right: Float) -> Float`
- `Clamp(value: Float, lower: Float, upper: Float) -> Float`
- `Floor(value: Float) -> Float`
- `Ceil(value: Float) -> Float`
- `Round(value: Float) -> Float`
- `PowExpLnReference(base: Float, exponent: Float) -> Float`
- `Sign(value: Float) -> Int`
- `Hypot(x: Float, y: Float) -> Float`
- `DifferentiateCentral(f: fn(Float) -> Float, x: Float, h: Float) -> Float ! Error`
- `IntegrateTrapezoidal(f: fn(Float) -> Float, a: Float, b: Float, n: Int) -> Float ! Error`
- `IntegrateSimpson(f: fn(Float) -> Float, a: Float, b: Float, n: Int) -> Float ! Error`
- `FastFourierTransform(x: Complex[]) -> Complex[] ! Error`
- `IFFT(X: Complex[]) -> Complex[] ! Error`

Production transforms should use builtin `FFT(x: Complex[]) -> Complex[] ! Error`; `FastFourierTransform` remains the pure Oct reference/oracle path.

Production exponentiation should use builtin `Pow(base, exponent)`; `PowExpLnReference` remains a pure-Oct educational/reference path.

## M1 transform conventions

M1 adds a focused 1D complex transform layer.

- Forward FFT (unnormalized):
  - `X[k] = Σ x[n] * exp(-j*2πkn/N)`
- Inverse FFT (normalized by `1/N`):
  - `x[n] = (1/N) * Σ X[k] * exp(+j*2πkn/N)`

This sign and normalization placement is fixed by tests.

## Input policy

For both `FastFourierTransform` and `IFFT` in M1:

- input length must be `> 0`
- input length must be a power of two
- invalid shape/length returns `Error`
- no silent padding, truncation, or zero-fill

Implementation strategy is radix-2 Cooley-Tukey for power-of-two lengths only.

## Implementation note

Hotfix note: FFT/IFFT now operate directly on `Complex[]` after adding `Len(Complex[])` support in core builtins.

## Behavior notes

- `Min` / `Max` return the lower/higher argument.
- `Clamp` normalizes bounds with `Min`/`Max` first, then clamps.
- `Floor` returns the greatest integer-valued real less than or equal to the input.
- `Ceil` returns the least integer-valued real greater than or equal to the input.
- `Round` uses **half-away-from-zero** (`2.5 -> 3.0`, `-2.5 -> -3.0`).
- `Pow` is a core builtin (`Pow(base: Int|Float, exponent: Int|Float) -> Float`) and is the production exponentiation API.
- `PowExpLnReference` is a pure-Oct reference implementation using `Exp(Ln(base) * exponent)` for educational/testing comparisons. `PowExpLnReference(0.0, negative)` returns `0.0` as a sentinel to avoid invalid log-domain evaluation.
- `Sign` returns `-1`, `0`, or `1`.
- `Hypot` computes `Sqrt(x*x + y*y)`.
- `DifferentiateCentral` uses `(f(x+h) - f(x-h)) / (2h)` and requires `h > 0`.
- `IntegrateTrapezoidal` uses the trapezoidal rule and requires `n > 0`.
- `IntegrateSimpson` uses Simpson's rule and requires `n > 0` and even `n`.
- Integration bound policy is signed and deterministic: `a == b` returns `0.0`, and `a > b` returns the negative of the corresponding `a < b` integral.

## Dimension rules

Mathematics M1 remains conservative:

- scalar helpers and calculus stay scalar `Float`/`Int`
- transforms operate on direct `Complex[]` traces
- no dimension-aware frequency-axis metadata is introduced in M1

## Non-goals

M1 intentionally does **not** include:

- real-FFT specializations
- multidimensional FFT
- STFT or spectrogram frameworks
- window functions
- DCT/DST families
- convolution helpers
- PSD/filtering/resampling abstractions
- wavelets
