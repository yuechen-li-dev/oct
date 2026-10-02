# Random

`Random` owns reproducible pseudorandom generation and small sampling-oriented helpers: seeding, uniform and Gaussian draws, distribution sampling, coin tosses, and dice.

It intentionally does not own deterministic PDF/CDF/PMF evaluation; use [`Distributions`](../Distributions/README.md) for that. Use [`Statistics`](../Statistics/README.md) to summarize samples and [`Uncertainty`](../Uncertainty/README.md) for measurement-uncertainty propagation. Start with `Random.Stream.octest` for the stream builtins, then `Random.Sampling.octest`, `Random.CoinToss.octest` and `Random.Dice.octest` for the helpers built on them.

The package is suitable for bounded scientific experiments that record their seed. It is not a cryptographic random-number source.

## Streams (Random v2)

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

The Random v1 generator (`RngSeed`, `RandInt`, `RandFloat01`, `RandFloatRange`, `RandBernoulli`, `RandNormal`, `Gaussian`) and the `Crypto*` functions are still present and are being retired. Specification and status: `internal/random/RANDOM_V2_LADDER.md`.
