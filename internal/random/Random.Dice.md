# Random.Dice (M3)

`Random.Dice` is a pure Oct convenience layer on top of `Random.Core`.

## API overview

### Deterministic

- `RollDie(rng: Rng, sides: Int) -> DieRollResult`
- `RollD4/D6/D8/D10/D12/D20/D100(rng: Rng) -> DieRollResult`
- `RollDice(rng: Rng, count: Int, sides: Int) -> DiceRollResult`
- `RollDiceSum(rng: Rng, count: Int, sides: Int) -> DieRollResult`
- `RollD20Advantage(rng: Rng) -> DiceRollResult`
- `RollD20Disadvantage(rng: Rng) -> DiceRollResult`

Result records:

- `DieRollResult { Next, Value }`
- `DiceRollResult { Next, Values, Total }`

### Crypto

- `CryptoRollDie!(sides: Int) -> Int`
- `CryptoRollD20!() -> Int`
- `CryptoRollDice!(count: Int, sides: Int) -> Int[]`

## Deterministic examples

```oct
let rng0 = Random.RngSeed(42)
let first = Random.RollDie(rng0, 6)
let second = Random.RollDie(first.Next, 6)
```

## Standard dice examples

```oct
let rng = Random.RngSeed(42)
let d20 = Random.RollD20(rng)
let d100 = Random.RollD100(d20.Next)
```

## Multiple dice examples

```oct
let rng = Random.RngSeed(42)
let pool = Random.RollDice(rng, 4, 6)
let total = pool.Total
let sumOnly = Random.RollDiceSum(pool.Next, 8, 6)
```

## Advantage / disadvantage examples

```oct
let rng0 = Random.RngSeed(42)
let d20 = Random.RollD20(rng0)
let adv = Random.RollD20Advantage(d20.Next)
let dis = Random.RollD20Disadvantage(adv.Next)
```

`RollD20Advantage` and `RollD20Disadvantage` each roll two d20 values. `Values` stores both rolls, and `Total` stores the selected max/min value.

## Crypto examples

```oct
let d6 = Random.CryptoRollDie!(6)
let d20 = Random.CryptoRollD20!()
let pool = Random.CryptoRollDice!(3, 6)
```

Crypto helpers are intentionally non-deterministic and should only be range/smoke tested.

## Validation rules

Deterministic:

- `RollDie`: requires `sides >= 2` via `Require`
- `RollDice`: requires `count >= 0` and `sides >= 2` via `Require`
- `RollDiceSum`: inherits `RollDice` preconditions

Deterministic invalid inputs are non-recoverable programmer errors and fail loudly.

Valid edge behavior is preserved: `RollDice(rng, 0, sides)` is valid and returns `Values = []`, `Total = 0`, and unchanged deterministic `Next`.

Crypto:

- `CryptoRollDie`: fallible error if `sides < 2`
- `CryptoRollDice`: fallible error if `count < 0` or `sides < 2`

Crypto helpers remain fallible (`! Error`) for invalid arguments.

## Notes

- This module uses `Random.Core` (`RandInt`, `CryptoRandInt`) for all random draws.
- No global RNG state is introduced; every deterministic helper explicitly threads `Rng` through `Next`.
- Keep-highest / keep-lowest were deferred in M3 to keep implementation minimal while language-level sorting helpers remain unverified in this module.
