# Random

`Random` owns reproducible pseudorandom generation and small sampling-oriented helpers: seeding, uniform and Gaussian draws, distribution sampling, coin tosses, and dice.

It intentionally does not own deterministic PDF/CDF/PMF evaluation; use [`Distributions`](../Distributions/README.md) for that. Use [`Statistics`](../Statistics/README.md) to summarize samples and [`Uncertainty`](../Uncertainty/README.md) for measurement-uncertainty propagation. Start with `Random.Stream.octest` for the stream builtins, then `Random.Sampling.octest`, `Random.CoinToss.octest` and `Random.Dice.octest` for the helpers built on them.

The package is suitable for bounded scientific experiments that record their seed. It is not a cryptographic random-number source. For a seed that is not chosen by the program, and for any other draw from the operating system's random source, use [`Entropy`](../Entropy/README.md).

## Streams

Every draw is a pure function of a stream, an index and the draw's parameters. Nothing is threaded between draws.

```oct
let noise = Random.Seeded(seed)
let jitter = Random.Fork(noise, "jitter")
let spikes = Random.Fork(noise, "spike")
for i in 0..n {
    let reading = truth + Random.Normal(jitter, i, 0.0, 0.2) + Random.Spike(spikes, i, 0.1, 3.0)
}
```

- Streams: `Seeded`, `Fork` (by name), `Child` (by index).
- Native draws: `Unit`, `Between`, `IntBetween`, `Normal`.
- Helpers: `Chance`, `Exponential`, `Units`, `Normals`, `Spike`, `FlipCoin`, `FlipCoins`, `RollDie`, `RollDice`, `RollWithAdvantage`, `RollWithDisadvantage`.

A draw never fails with an `Error`. A violated precondition (a negative index, `lo > hi`, a negative `stddev` or `count`, a probability outside `[0, 1]`) stops the program.

## Coming from Random 0.1.0

The generator-state API is removed, not deprecated: `Rng`, `RngSeed`, `RandInt`, `RandFloat01`, `RandFloatRange`, `RandBernoulli`, `RandNormal`, `Gaussian`, every `*Result` record with its `Value` and `Next` fields, and `CryptoRandInt`, `CryptoRandFloat01` and `CryptoRandBytes`. Seeded sequences changed with the generator, so recorded outputs must be regenerated. The table of replacements is in section 9 of the specification.

Specification: `internal/random/Random.md`.
