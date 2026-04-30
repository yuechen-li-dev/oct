# Random.Core (M0 API Contract)

## Scope

This document defines the M0 API contract for `Libraries/Random/Random.Core.oct`.
M0 is design-only and intentionally does not include runtime RNG implementation logic.

## Design rationale

### Why RNG state is explicit

Deterministic randomness is stateful. Making `Rng` an explicit value in function signatures means:

- call sites show exactly where randomness enters a computation,
- state transitions are visible (`rng2, value = Func(rng1, ...)`),
- tests can replay paths by controlling seed and call order.

### Why there is no global RNG

Global mutable RNG state breaks reproducibility and makes hidden coupling easy.
This API deliberately has no global deterministic RNG instance.
All deterministic sampling requires an explicit `Rng` argument.

### Why deterministic and crypto are separated

Deterministic simulation and secure randomness have different goals:

- deterministic RNG: reproducible, test-friendly, simulation-oriented,
- cryptographic RNG: non-deterministic and fallible by nature.

The API keeps these separate with different function families and failure behavior.

## API surface summary

Deterministic (`Rng` threaded, non-fallible signatures):

- `RngSeed(seed: Int) -> Rng`
- `RandUint64(rng: Rng) -> (Rng, Int)`
- `RandFloat01(rng: Rng) -> (Rng, Float)` in `[0.0, 1.0)`
- `RandFloatRange(rng: Rng, min: Float, max: Float) -> (Rng, Float)` in `[min, max)`
- `RandInt(rng: Rng, min: Int, max: Int) -> (Rng, Int)` in `[min, max]`
- `RandBernoulli(rng: Rng, p: Float) -> (Rng, Bool)` where `0.0 <= p <= 1.0`
- `RandNormal(rng: Rng, mean: Float, stddev: Float) -> (Rng, Float)` where `stddev >= 0.0`; if `stddev == 0.0`, result is `mean`

Deterministic validation policy: invalid input is a runtime error (non-fallible API by design).

Crypto (`! Error`, no `Rng` parameter):

- `CryptoRandUint64() -> Int ! Error`
- `CryptoRandBytes(count: Int) -> Bytes ! Error`
- `CryptoRandInt(min: Int, max: Int) -> Int ! Error`
- `CryptoRandFloat01() -> Float ! Error`

Crypto validation policy: argument and runtime failures are surfaced via `Error`.

## Usage examples

Deterministic:

```oct
package Main

import Random

fn Main() -> Int {
    var rng = Random.RngSeed(42)

    rng, x = Random.RandFloat01(rng)
    rng, y = Random.RandFloatRange(rng, -1.0, 1.0)
    rng, z = Random.RandNormal(rng, 0.0, 1.0)
    rng, n = Random.RandInt(rng, 1, 6)
    rng, b = Random.RandBernoulli(rng, 0.2)

    if b {
        return n
    }

    return Int(x + y + z) + n * 0
}
```

Crypto:

```oct
package Main

import Random

fn Main() -> Int ! Error {
    let token = Random.CryptoRandBytes(32)?
    return LenBytes(token)
}
```

## Determinism guarantee

Within a given Oct version:

- same seed,
- same deterministic random API calls,
- same call order,

must produce the same sequence of outputs.

## Future implementation note (M1 target)

Target algorithm plan for M1:

- seeding: SplitMix64,
- generation: xoshiro256**.

This is the intended implementation target for M1.
Any future algorithm change must be explicit and versioned.

## Reference alignment and known inconsistency

The task request used lowercase names like `int`, `float`, `bool`, `byte[]`, and a `uint64` return.
Current Oct reference and existing library code use `Int`, `Float`, `Bool`, and `Bytes`.
M0 follows the current reference authority (`Language/reference`) and uses canonical current names accordingly.

Also, the requested `uint64` type is not currently documented in `Language/reference`.
This is a documentation/spec gap to resolve before a strict `uint64` surface can be made canonical.
