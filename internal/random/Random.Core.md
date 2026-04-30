# Random.Core (M1a runtime)

Random M1a provides deterministic state-threaded RNG and fallible crypto RNG.

## Public API

```oct
package Random

type Rng

RngSeed(seed: Int) -> Rng

RandInt(rng: Rng, min: Int, max: Int) -> (Rng, Int)
RandFloat01(rng: Rng) -> (Rng, Float)
RandFloatRange(rng: Rng, min: Float, max: Float) -> (Rng, Float)
RandBernoulli(rng: Rng, p: Float) -> (Rng, Bool)
RandNormal(rng: Rng, mean: Float, stddev: Float) -> (Rng, Float)

CryptoRandInt!(min: Int, max: Int) -> Int
CryptoRandFloat01!() -> Float
CryptoRandBytes!(count: Int) -> Bytes
```

## Canonical state-threading pattern

```oct
var rng = Random.RngSeed(42)
rng, x = Random.RandInt(rng, 1, 6)
rng, y = Random.RandFloat01(rng)
```

Deterministic Random APIs are infallible and require explicit state threading through `var` reassignment.

## Deterministic guarantee

Within one Oct version, the same seed and same call order produce the same sequence.
No global RNG state is used.

## Crypto vs seeded

- Seeded (`RngSeed`, `Rand*`) is deterministic/reproducible and infallible.
- Crypto (`CryptoRand*`) uses Go `crypto/rand`, is non-deterministic, and is fallible (`!`).

## Validation rules

- `RandInt`: inclusive `[min, max]`, requires `min <= max`.
- `RandFloat01`: `[0.0, 1.0)`.
- `RandFloatRange`: `[min, max)`, requires `min <= max`; `min == max` returns `min`.
- `RandBernoulli`: `0.0 <= p <= 1.0`; exact edges map to `false`/`true`.
- `RandNormal`: requires `stddev >= 0`; `stddev == 0` returns `mean`.
- `CryptoRandInt!`: inclusive `[min, max]`, invalid bounds return an error.
- `CryptoRandBytes!`: requires `count >= 0`; `count == 0` returns empty `Bytes`.

## Implementation note

- Seed expansion: SplitMix64.
- Generator step: xoshiro256**.
- Normal distribution: Box-Muller transform.

The implementation may use 4×`uint64` internally, but numeric-width internals are not exposed in the public API (which uses only `Rng`, `Int`, `Float`, `Bool`, and `Bytes`).
