# Random

Status: current as of 2026-10-03 (Random 0.2.0).

This is the specification of the `Random` library and the `Entropy` namespace.
It replaces `Random.Core.md`, `Random.CoinToss.md`, `Random.Dice.md` and
`Random.Distributions.md`, which described the record-result generator that
Random 0.2.0 removed. The history of the change, with its evidence, is in
`RANDOM_V2_LADDER.md` and `RANDOM_V2_M1.md` through `RANDOM_V2_M6.md`.

## 1. Model

Every draw is a pure function of a stream, an index and the draw's parameters.
No generator state exists, so nothing is threaded between draws and nothing
can be forgotten.

```oct
import Random

fn Readings(truth: Float, seed: Int, n: Int) -> Float[] {
    let noise = Random.Seeded(seed)
    let jitter = Random.Fork(noise, "jitter")
    let spikes = Random.Fork(noise, "spike")
    var readings: Float[] = []
    for i in 0..n {
        readings = Append(readings, truth + Random.Normal(jitter, i, 0.0, 0.2) + Random.Spike(spikes, i, 0.1, 3.0))
    }
    return readings
}
```

Use one `Fork` per independent source of randomness and the loop index as the
draw index. Setting the jitter's standard deviation to zero does not move a
single spike.

`Random` is deterministic. It is not a cryptographic source. A seed the
program does not choose comes from `Entropy` (section 6).

## 2. Types and names

```oct
package Random

record Stream { _Key: Int }
```

`_Key` holds the 64-bit generator key. Every `Int` is a valid key; there is no
degenerate stream. Writing `Stream { _Key: k }` is permitted but is not the
public way to make a stream. Source-level opacity is not enforced.

Seven functions are native: `Seeded`, `Fork`, `Child`, `Unit`, `Between`,
`IntBetween`, `Normal`.

- Outside package Random they exist only in the qualified spelling
  (`Random.Unit`), and the calling package must `import Random`. The bare
  words are not reserved: any other package may declare its own `Unit` or
  `Normal`.
- Inside package Random both spellings name the builtin, and the package may
  not declare a function with one of those names.
- They have no Oct declarations. `Random.Stream.oct` declares only the record.

No other name in the library is a builtin or an alias of one. Everything in
section 5 runs its Oct body.

## 3. Bit generator

Philox4x32-10 (Salmon et al., SC'11; Random123), with multipliers
`0xD2511F53` and `0xCD9E8D57`, Weyl constants `0x9E3779B9` and `0xBB67AE85`,
and 10 rounds. It is the only bit generator.

- Key words: `k0 = low32(_Key)`, `k1 = high32(_Key)`.
- An `Int` value `n` enters the counter as `low32(uint64(n))`, `high32(uint64(n))`.
- The four output words form two 64-bit words: `w0 = o0 | o1<<32`, `w1 = o2 | o3<<32`.

Counter layout. Word 3 is a domain tag, so a draw, a child and a fork can
never share a counter:

| Use | `c0` | `c1` | `c2` | `c3` |
|---|---|---|---|---|
| Draw at index `i`, block `j` | `low32(i)` | `high32(i)` | `j` | `0` |
| `Child(s, i)` | `low32(i)` | `high32(i)` | `0` | `1` |
| `Fork(s, label)`, `h = FNV1a64(utf8(label))` | `low32(h)` | `high32(h)` | `0` | `2` |

A derived key is `w0` of block 0 for that counter.

## 4. Native functions

Implemented once, in Go, in `internal/octrandom`. The interpreter calls that
package and generated programs import it, so the two execution lanes cannot
differ.

Every function that takes an index stops the program if the index is negative.

| Signature | Definition |
|---|---|
| `Seeded(seed: Int) -> Stream` | `_Key = seed`. |
| `Fork(s: Stream, label: String) -> Stream` | Derived key, domain 2. An empty label is allowed. |
| `Child(s: Stream, i: Int) -> Stream` | Derived key, domain 1. |
| `Unit(s: Stream, i: Int) -> Float` | `(w0 >> 11) · 2⁻⁵³` from block 0. Range `[0, 1)`. Exact. |
| `Between(s: Stream, i: Int, lo: Float, hi: Float) -> Float` | Requires `lo` and `hi` finite, `lo <= hi` and `hi - lo` finite. Returns `lo` if `lo == hi`. Otherwise `r = lo + (hi - lo)·Unit`, and if `r >= hi` then `r = nextafter(hi, lo)`. Range `[lo, hi)`. |
| `IntBetween(s: Stream, i: Int, lo: Int, hi: Int) -> Int` | Requires `lo <= hi`. Both ends are included. `span = uint64(hi) - uint64(lo) + 1`, wrapping. If `span == 0` the result is `int64(w0)` of block 0. Otherwise words are consumed in the order `b0.w0, b0.w1, b1.w0, b1.w1, …`; the first `w < 2⁶⁴ − (2⁶⁴ mod span)` is accepted and the result is `lo + int64(w mod span)`. No modulo bias. |
| `Normal(s: Stream, i: Int, mean: Float, stddev: Float) -> Float` | Requires `stddev >= 0` and both finite. From block 0: `u1 = ((w0 >> 11) + 1)·2⁻⁵³` in `(0, 1]`, `u2 = (w1 >> 11)·2⁻⁵³`, `z = sqrt(−2 ln u1)·cos(2π u2)`. The result is `mean + stddev·z`. `stddev == 0` gives `mean` through the same path. |

The arguments are dimensionless. A `Float<m>` bound is a type error; draw a
plain `Float` and attach the unit to the result.

## 5. Library functions

Written in Oct, in `Random.Sampling.oct`, `Random.CoinToss.oct` and
`Random.Dice.oct`.

| Signature | Definition |
|---|---|
| `Chance(s, i, p: Float) -> Bool` | Requires `0 <= p <= 1`. `Unit(s, i) < p`. |
| `Exponential(s, i, rate: Float) -> Float` | Requires `rate > 0`. `−Ln(1 − Unit(s, i)) / rate`. |
| `Units(s, count: Int) -> Float[]` | Requires `count >= 0`. Element `k` is `Unit(s, k)`. |
| `Normals(s, count, mean, stddev) -> Float[]` | Requires `count >= 0`. Element `k` is `Normal(s, k, mean, stddev)`. |
| `Spike(s, i, probability, amplitude) -> Float` | Requires `amplitude >= 0`. With `c = Child(s, i)`: if `Chance(c, 0, probability)` the result is `amplitude` or `-amplitude` by `Chance(c, 1, 0.5)`, otherwise `0.0`. |
| `FlipCoin(s, i) -> CoinSide` | `Heads` if `Chance(s, i, 0.5)`. |
| `FlipCoins(s, i, count) -> CoinSide[]` | Requires `count >= 0`. Element `k` is `FlipCoin(Child(s, i), k)`. |
| `RollDie(s, i, sides) -> Int` | Requires `sides >= 2`. `IntBetween(s, i, 1, sides)`. |
| `RollDice(s, i, count, sides) -> Int[]` | Requires `count >= 0` and `sides >= 2`. Element `k` is `RollDie(Child(s, i), k)`. |
| `RollWithAdvantage(s, i, sides) -> Int` | The larger of `RollDice(s, i, 2, sides)`. |
| `RollWithDisadvantage(s, i, sides) -> Int` | The smaller of `RollDice(s, i, 2, sides)`. |
| `CountHeads`, `CountTails`, `CoinSideToString`, `enum CoinSide { Heads Tails }` | Helpers over coin results. |

A function that makes several draws for one index (`Spike`, `FlipCoins`,
`RollDice`) takes them from `Child(s, i)`, so its draws never collide with
draws at other indices of `s`.

## 6. Entropy

```oct
Entropy.Seed() -> Int ! Error
Entropy.IntBetween(lo: Int, hi: Int) -> Int ! Error
Entropy.Unit() -> Float ! Error
Entropy.Bytes(count: Int) -> Bytes ! Error
```

`Entropy` reads the operating system's random source. It is the one place
nondeterminism enters an Oct program.

- It is a compiler-owned namespace, like `Artifact`: four builtins, no Oct
  declarations, no `import` needed. `Libraries/Entropy` exists so that
  `import Entropy` resolves and so the package has a manifest and tests. The
  naming rule of section 2 applies.
- Every call is fallible. The only `Error` is a failed read of the source.
- `IntBetween` is uniform on the closed range and has no modulo bias. `Unit`
  is uniform on `[0, 1)` and is a multiple of `2⁻⁵³`. `Seed` is any `Int`.
- Artifact evaluation and capability discovery reject every `Entropy` call
  before it reads anything.

Record the value of `Entropy.Seed()`. Every draw after `Random.Seeded(seed)`
is a function of it, so a logged seed replays the run.

## 7. Failures

| Cause | Result |
|---|---|
| Wrong argument count or type, a fallible or dimensioned argument, type arguments, a bare native name outside its package, a missing `import Random` | Compile error. |
| Negative index, `lo > hi`, negative `stddev`, negative `count`, `p` outside `[0, 1]`, `rate <= 0`, `sides < 2`, negative `amplitude`, non-finite `Float` argument | The program stops with a runtime error. It is not an `Error` value and `match` does not see it. |
| The operating system's random source cannot be read | An ordinary `Error`, from `Entropy` only. |

`Random` never returns `Error`.

## 8. Invariants

- **I1 Purity.** Equal arguments give equal results, in any evaluation order.
- **I2 Bulk–scalar identity.** `Units(s, n)[k] == Unit(s, k)` and
  `Normals(s, n, m, sd)[k] == Normal(s, k, m, sd)`.
- **I3 Parameter isolation.** Changing the parameters of a draw on one
  `(stream, index)` never changes any other draw.
- **I4 Lane parity.** Interpreted and compiled execution give bit-identical
  results on the same `GOOS/GOARCH`. There is one Go implementation.
- **I5 Cross-platform determinism.** Outputs derived from integers (`Fork`,
  `Child`, `IntBetween`, `Unit`, `Between`, `Chance`, coins, dice) are
  bit-identical on every platform. Outputs that pass through a transcendental
  function (`Normal`, `Exponential`) are guaranteed identical only within one
  `GOARCH`, because Go may fuse multiply-adds on arm64, ppc64le and s390x,
  including inside `math.Log` and `math.Cos`. `internal/octrandom` rounds its
  own expressions explicitly so that no fusion happens in its code. This has
  been run on linux/amd64 only.
- **I6 No aliases.** Only the seven names of section 4 are native.

## 9. What Random 0.2.0 removed

Seeded sequences changed with the generator. A program that recorded outputs
under Random 0.1.0 must regenerate them.

| 0.1.0 | 0.2.0 |
|---|---|
| `RngSeed(seed)`, `Rng` | `Seeded(seed)`, `Stream` |
| `RandFloat01(rng)` | `Unit(s, i)` |
| `RandFloatRange`, `Uniform` | `Between(s, i, lo, hi)` |
| `RandInt(rng, lo, hi)` | `IntBetween(s, i, lo, hi)` |
| `RandNormal`, `Gaussian` | `Normal(s, i, mean, stddev)` |
| `RandBernoulli`, `Bernoulli`, `FlipBiasedCoin` | `Chance(s, i, p)` |
| `Jitter(rng, a)` | `Between(s, i, -a, a)` |
| `DriftStep(rng, x, sd)` | `x + Normal(s, i, 0.0, sd)` |
| `RollD4` … `RollD100` | `RollDie(s, i, sides)` |
| `RollDiceSum` | Add the elements of `RollDice(s, i, count, sides)` in a loop. Oct has no `Sum` over `Int[]`. |
| `RollD20Advantage`, `RollD20Disadvantage` | `RollWithAdvantage(s, i, 20)`, `RollWithDisadvantage(s, i, 20)` |
| `draw.Value`, `draw.Next`, every `*Result` record | Gone. A draw returns its value. |
| `CryptoRandInt`, `CryptoRandFloat01`, `CryptoRandBytes` | `Entropy.IntBetween`, `Entropy.Unit`, `Entropy.Bytes` |
| `CryptoFlipCoin*`, `CryptoRollDie*` | None. Seed a stream from `Entropy.Seed()`, or call `Entropy.IntBetween`. |

The old names are not reserved and not deprecated; they are undefined.

## 10. Where it lives

| Path | Contents |
|---|---|
| `internal/octrandom` | Philox, FNV-1a, key derivation, the four draws, the Entropy functions. Go tests hold the Random123 known-answer vectors. |
| `internal/builtin/random.go` | The one table of native names, namespaces and signatures that the typechecker, the interpreter and the compiled backend read. |
| `Libraries/Random` | `Random.Stream.oct` (the record), `Random.Sampling.oct`, `Random.CoinToss.oct`, `Random.Dice.oct`, their `.octest` files, and `.octfail` compile-time contracts. |
| `Libraries/RandomUsage` | Contracts for use from another package. |
| `Libraries/Entropy`, `Language/Builtins/Entropy` | The Entropy marker package and its contracts. |
| `testdata/random_stream_preconditions`, `testdata/entropy_source_failure` | Runtime failures, which neither `.octest` nor `.octfail` can assert. Run in both lanes by Go tests under `cmd/oct` in the `integration` lane. |

## 11. Not provided

- `Pick` and `Shuffle`. They need generic library functions.
- Distributions beyond uniform, normal and exponential.
- A GPU implementation of the same streams.
- Source-level opacity for `Stream`.
