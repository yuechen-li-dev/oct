# Distributions M0

## Scope

`Libraries/Distributions` is a deterministic **distribution evaluation** library.
It does **not** provide sampling, inference, fitting, significance tests, or any RNG surface.

## Function surface

- `NormalPdf(x: Float, mean: Float, sigma: PositiveDistributionScale) -> Float`
- `NormalCdf(x: Float, mean: Float, sigma: PositiveDistributionScale) -> Float`
- `UniformPdf(x: Float, a: Float, b: Float) -> Float ! Error`
- `UniformCdf(x: Float, a: Float, b: Float) -> Float ! Error`
- `ExponentialPdf(x: Float, lambda: PositiveDistributionScale) -> Float`
- `ExponentialCdf(x: Float, lambda: PositiveDistributionScale) -> Float`
- `BernoulliPmf(k: Int, probability: Probability) -> Float`
- `BinomialPmf(k: Int, n: NonNegativeCount, probability: Probability) -> Float`
- `PoissonPmf(k: Int, lambda: PositiveDistributionScale) -> Float`
- `LogFactorial(n: NonNegativeCount) -> Float`

## Parameter and domain policy

- M0 is **dimensionless `Float` inputs only**.
- Invalid distribution parameters reject during refined-Concept admission (statically when known, fallibly when constructed from runtime data):
  - `sigma <= 0` for Normal
  - `a >= b` for Uniform
  - `lambda <= 0` for Exponential
  - probability outside `[0, 1]` for Bernoulli/Binomial
  - `n < 0` for Binomial
  - `lambda <= 0` for Poisson
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

## Discrete textbook examples

For five fair independent trials, exactly two successes have probability
`C(5,2) / 2^5 = 0.3125`:

```oct
let mass = BinomialPmf(2, 5, 0.5)
```

Binomial coefficients are accumulated multiplicatively rather than through integer factorials. `PoissonPmf` uses the recurrence `exp(-lambda) * product(lambda/i)`. Both are reference-quality routines for moderate parameters; extreme tails need log-domain distribution algorithms.

`Probability`, `PositiveDistributionScale`, and `NonNegativeCount` are refined Concepts. Literal arguments are proved at compile time; unknown runtime values use explicit checked admission such as `Probability(raw)?`. Distribution evaluation is infallible once its parameters have been admitted. Relational bounds such as Uniform's `a < b` remain fallible because they relate two independent values rather than defining either value alone.
