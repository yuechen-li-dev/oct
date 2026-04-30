# Random.Core (M1 runtime)

Random M1 provides deterministic state-threaded RNG and crypto RNG.

## API usage

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

Crypto APIs are fallible and non-deterministic:

```oct
token = Random.CryptoRandBytes(32)?
```

## Deterministic guarantees

Within one Oct version, same seed + same call order yields same sequence.
No global RNG is used; state is explicit through `Rng` tuple-threading.

## Crypto vs seeded

- Seeded (`RngSeed`, `Rand*`) is deterministic and reproducible.
- Crypto (`CryptoRand*`) uses Go `crypto/rand`, has no `Rng` parameter, and is fallible.

## Internal algorithm note

- Seeding: SplitMix64
- Generator: xoshiro256** (4x uint64 internal state)

Internal uint64 state is not part of the public API.
