# Random.CoinToss

`Random.CoinToss` is a pure Oct library layer built on top of `Random.Core` draws and state threading.

## API overview

- `CoinSide` enum (`Heads`, `Tails`)
- `CoinFlipResult` record (`Next`, `Side`)
- `CoinFlipManyResult` record (`Next`, `Flips`)
- Deterministic APIs:
  - `FlipCoin(rng)`
  - `FlipBiasedCoin(rng, pHeads)`
  - `FlipCoins(rng, count)`
  - `CountHeads(flips)` / `CountTails(flips)`
  - `CoinSideToText(side)`
- Crypto APIs:
  - `CryptoFlipCoin!()`
  - `CryptoFlipBiasedCoin!(pHeads)`
  - `CryptoFlipCoins!(count)`

## Deterministic examples

```oct
let rng0 = Random.RngSeed(42)
let flip1 = Random.FlipCoin(rng0)
let flip2 = Random.FlipCoin(flip1.Next)
```

## Biased examples

```oct
let rng0 = Random.RngSeed(42)
let heads = Random.FlipBiasedCoin(rng0, 1.0)
let tails = Random.FlipBiasedCoin(rng0, 0.0)
```

## Bulk flips

```oct
let rng0 = Random.RngSeed(42)
let many = Random.FlipCoins(rng0, 10)
let h = Random.CountHeads(many.Flips)
let t = Random.CountTails(many.Flips)
```

## Crypto examples

```oct
let side = Random.CryptoFlipCoin!()
let biased = Random.CryptoFlipBiasedCoin!(0.7)
let many = Random.CryptoFlipCoins!(4)
```

## Enum consumption guidance

Use `switch` (or `match` where payload binding is needed) to consume `CoinSide` exhaustively:

```oct
switch side {
    case Random.CoinSide.Heads => "Heads"
    case Random.CoinSide.Tails => "Tails"
}
```

## Core dependency statement

All deterministic coin tosses call `Random.RandBernoulli` and thread `Next` from the returned result record.
