# Random.Distributions (M5)

`Random.Distributions` is a pure Oct layer on top of `Random.Core` for simulation-oriented noise and distribution helpers.

It is intended for deterministic labs like P14 measurement robustness where you need composable noise sources (jitter, spikes, drift, Gaussian noise) without hidden RNG state.

## API overview

All APIs use explicit RNG threading through `RandFloatResult` / `RandBoolResult` records:

- `Next`: next RNG state
- `Value`: sampled value

Provided helpers:

- `Uniform(rng, min, max) -> RandFloatResult`
- `Gaussian(rng, mean, stddev) -> RandFloatResult`
- `Exponential(rng, lambda) -> RandFloatResult`
- `Jitter(rng, amplitude) -> RandFloatResult`
- `Spike(rng, probability, amplitude) -> RandFloatResult`
- `DriftStep(rng, current, stepStddev) -> RandFloatResult`
- `Bernoulli(rng, p) -> RandBoolResult` (thin alias of `RandBernoulli`)

## P14-style measurement noise example

```oct
var rng = Random.RngSeed(123)

let jitter = Random.Jitter(rng, 0.05)
rng = jitter.Next

let spike = Random.Spike(rng, 0.01, 10.0)
rng = spike.Next

let measured = trueValue + jitter.Value + spike.Value
```

## State threading pattern

```oct
var rng = Random.RngSeed(42)

let u = Random.Uniform(rng, -1.0, 1.0)
rng = u.Next

let g = Random.Gaussian(rng, 0.0, 0.1)
rng = g.Next

let d = Random.DriftStep(rng, baseline, 0.02)
rng = d.Next
```

## Validation rules

Deterministic invalid inputs fail using `Require`:

- `Uniform`: `min <= max`
- `Gaussian`: `stddev >= 0`
- `Exponential`: `lambda > 0`
- `Jitter`: `amplitude >= 0`
- `Spike`: `0 <= probability <= 1` and `amplitude >= 0`
- `DriftStep`: `stepStddev >= 0`

## Deferred scope

Intentionally deferred in M5:

- Triangular distribution
- Poisson distribution
- Statistical conformance/quality test suite

M5 focuses on practical simulation helpers needed for measurement-noise workloads, with explicit deterministic state transitions and no hidden mutable RNG.

### Invalid-input contract coverage note

`go run ./cmd/oct test Libraries/Random` currently executes package-local `.octfail` files in isolated temporary roots, which prevents cross-file references/imports needed to directly call `Random.Distributions` helpers from standalone `.octfail` fixtures in this package.

Because of that harness behavior, invalid deterministic input contracts are enforced in implementation via `Require(...)` and documented here, but are not yet asserted via per-helper `.octfail` fixtures under `Libraries/Random` in M5.
