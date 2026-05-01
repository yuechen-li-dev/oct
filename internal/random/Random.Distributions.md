# Random.Distributions

Random.Distributions is built on Random.Core deterministic state threading.

## Deterministic behavior expectations

- `Jitter`, `Spike`, and `DriftStep` are draw operations and therefore advance RNG state through `Next`.
- Degenerate parameter cases are still deterministic and preserve Random.Core contracts.
- Same seed and same threaded calls produce the same sequence.

## M8 non-degeneracy coverage

`Libraries/Random/Random.Distributions.octest` now verifies:

- `Jitter(0.05)` produces at least one non-zero value over repeated draws.
- `Spike(0.1, 4.0)` eventually produces non-zero spikes with fixed seed.
- `DriftStep` changes state/value across multiple steps.
- Gaussian zero-stddev and Exponential deterministic reproducibility still hold.

## Public type boundary

Distributions continue to expose only Oct public types (`Rng`, `Int`, `Float`, `Bool`, arrays/records) with no width-specific numeric leakage.
