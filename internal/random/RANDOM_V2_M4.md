# Random v2 — M4: Oct library layer

Date: 2026-10-02
Ladder: `internal/random/RANDOM_V2_LADDER.md`
Base commit: `9cbf2e7`

## Verdict

**SUCCESS.** The v2 library layer of ladder section 3.4 is implemented in Oct
on top of the native stream primitives and passes in the interpreted and
compiled lanes. It replaces the seeded v1 library layer. No compiler code
changed in this milestone.

## What a user can now write

```oct
let noise  = Random.Seeded(seed)
let jitter = Random.Fork(noise, "jitter")
let spikes = Random.Fork(noise, "spike")
for i in 0..n {
    let reading = truth + Random.Normal(jitter, i, 0.0, 0.2) + Random.Spike(spikes, i, 0.1, 3.0)
}

let hand  = Random.RollDice(dice, turn, 5, 6)        // Int[]
let flips = Random.FlipCoins(coins, round, 10)       // CoinSide[]
let hit   = Random.Chance(events, i, 0.25)           // Bool
```

## What changed

| File | Change |
|---|---|
| `Libraries/Random/Random.Sampling.oct` (new) | `Chance`, `Exponential`, `Units`, `Normals`, `Spike` |
| `Libraries/Random/Random.CoinToss.oct` | v2 `FlipCoin`, `FlipCoins`. `CoinSide`, `CountHeads`, `CountTails`, `CoinSideToString` unchanged. |
| `Libraries/Random/Random.Dice.oct` | v2 `RollDie`, `RollDice`, `RollWithAdvantage`, `RollWithDisadvantage` |
| `Libraries/Random/Random.Distributions.oct`, `.octest` | Deleted |
| `Experiments/PrometheusMeasurementFilteringLab/M2` | See "The one outside caller" |
| Tests and docs | See below |

Every function is as the ladder defines it. `Spike`, `FlipCoins` and `RollDice`
take their several draws from `Child(s, i)`, so the draws at one index never
overlap the draws at another.

### How the M3 open issue was resolved

M3 found that the v2 functions could not sit beside v1: six names
(`Exponential`, `Spike`, `FlipCoin`, `FlipCoins`, `RollDie`, `RollDice`) were
already v1 functions with different signatures, and Oct has no overloading. M4
therefore replaces the seeded v1 library layer outright.

The proposal in the M3 report was to migrate the experiments in M4 as well.
That turned out to be unnecessary. The experiments call the v1 *natives*
(`RngSeed`, `RandNormal`, `RandBernoulli`, `Gaussian`), which M4 does not
touch. Only one file outside the library called the removed layer. So the
experiment migration stays in M6 as originally planned.

### Removed

`Uniform`, `Bernoulli`, `Jitter`, `DriftStep`, the v1 `Exponential` and `Spike`,
`FlipBiasedCoin`, the v1 `FlipCoin` and `FlipCoins`, the v1 `RollDie` and
`RollDice`, `RollD4`..`RollD100`, `RollDiceSum`, `RollD20Advantage`,
`RollD20Disadvantage`, and the records `CoinFlipResult`, `CoinFlipManyResult`,
`DieRollResult` and `DiceRollResult`.

### Kept, unchanged

- The v1 natives and their stubs and records in `Random.Core.oct`: removed in
  M6.
- The `Crypto*` coin and dice helpers, moved verbatim within their files:
  removed in M5.
- `Gaussian` remains callable. It is a native alias of `RandNormal`; the Oct
  stub that sat in `Random.Distributions.oct` never ran (audit finding F5) and
  was deleted with that file.

### The one outside caller

`Experiments/PrometheusMeasurementFilteringLab/M2` called `Random.Jitter`,
`Random.Spike` and `Random.DriftStep`. That experiment characterizes Random
v1, so the three composites are now local functions in the experiment
(`M2JitterV1`, `M2SpikeV1`, `M2DriftStepV1`), written exactly as v1 defined
them on the same v1 native draws. Its tests pass in both lanes and `oct
artifact` reports all three recorded `.octagon` files `UNCHANGED`. M6 migrates
it to v2 with the other experiments.

## Tests

| Where | What |
|---|---|
| `Random.Sampling.octest` (11 facts) | Expected values for each function; each function against its definition; bulk–scalar identity for `Units` and `Normals` (I2); prefix property; `Spike` draws from the child stream; statistical bands. |
| `Random.CoinToss.octest` (7 facts) | Expected sides; `FlipCoins` against `FlipCoin(Child(s, i), k)`; fairness band; the unchanged helpers and crypto smoke test. |
| `Random.Dice.octest` (8 facts) | Expected faces; `RollDice` against `RollDie(Child(s, i), k)`; every face of several dice appears; face counts; advantage and disadvantage are the higher and lower of the same two dice. |
| `Libraries/RandomUsage` (+2 facts) | The library layer called from another package, including `Random.CoinSide`; the measurement loop with one noise source turned off. |
| `testdata/random_stream_preconditions` (+14 facts, 22 total) | Each library precondition, in both lanes, through the M3 Go test. |

Expected values were computed in Go from `internal/octrandom`, outside the Oct
library code, so they check the Oct definitions against the specification and
not against themselves. All of them matched in both lanes on the first run.

Ladder sanity bands, all met: mean of `Units(s, 10000)` in `[0.48, 0.52]`;
mean and variance of `Normals(s, 10000, 0, 1)` within `±0.05` and `±0.1`;
each face of `RollDice(s, 0, 6000, 6)` between 850 and 1150.

## Evidence

All runs on linux/amd64, Go 1.25.0. "Baseline" is `9cbf2e7`. The compiler is
the same in both columns; only Oct sources, one Go test and docs changed.

| Check | Baseline | After M4 |
|---|---|---|
| `oct test Libraries/Random`, interpreted / compiled / auto | 53 passed | 64 passed in each mode |
| `oct test Libraries/RandomUsage`, each mode | 6 passed | 8 passed |
| Preconditions test (`-tags=integration`), both lanes | 8 facts | 22 facts, pass |
| `go test ./...` and `go test -tags=integration ./...` | 1 and 2 packages fail | same packages, same failing tests |
| Per-directory sweep, compiled, every directory under `Libraries`, `Language` and `Experiments` that holds an `.octest` (348) | 2191 pass, 228 fail | 2204 pass, 228 fail |
| Per-directory sweep, interpreted | 2148 pass, 59 fail | 2162 pass, 58 fail |
| `PrometheusMeasurementFilteringLab/M2` tests, both lanes, and recorded outputs | 6 passed | 6 passed; outputs unchanged |

Compiled sweep: the only lines that differ are the 12 removed v1 library tests
and the 25 added tests. Interpreted sweep: the same, plus one test
(`FmBrownNoiseKalman.M4bGridSizeIs27`) that exceeded its 30 s cycle limit in
the baseline run while the machine was loaded and passes with both trees when
run alone.

The failures in both columns are the ones reported in M3: missing tools and
sidecars in this environment, UI builtins the compiled lane does not support,
the compiled-lane panic in `PrometheusMeasurementFilteringLab/M4`, and
`FmBrownNoiseKalman/M1`.

**Fault injection.** 33 deliberate faults were injected into the Oct library
one at a time: wrong comparisons, missing negation, off-by-one indices, swapped
parameters, a draw taken from the parent stream instead of the child, a sign
that reuses the trigger draw, inverted or biased coins, zero-based dice,
advantage and disadvantage swapped, and each precondition removed or weakened.
32 were caught at once. One survived: `Chance` using `<=` where the
specification says `<`. A test now pins it (`Chance(s, i, Unit(s, i))` is
false), and all 33 are caught.

## Exit check

No `Assert.True(false, ...)` dispatch stub remains in the v2 sources
(`Random.Stream.oct`, `Random.Sampling.oct`, `Random.CoinToss.oct`,
`Random.Dice.oct`). The nine that remain are the v1 native stubs in
`Random.Core.oct`.

## Notes

- Oct `Assert.Equal` does not accept arrays, so the tests compare arrays
  element by element through small helpers.
- The v1 code used `0.0 - x` for negation. The v2 library uses unary `-x`,
  which `Language/reference/language/03-expressions.md` documents.
- `Libraries/Random/README.md` pointed at the deleted
  `Random.Distributions.octest`; it now describes the v2 surface and says v1 is
  being retired. The three v1 pages under `internal/random/` carry a
  superseded banner. The manifest version and the full docs pass stay in M6.
- The two statements in the ladder's section 3.4 that left preconditions
  implicit (`Normals` count, `Spike` amplitude, `FlipCoins` count, `RollDice`
  count and sides) are now written out.

## Not run

- Windows and arm64.
- The `toolchain`-tagged Go lane and the slow wrapper lanes.

## Addendum (2026-10-02, from M5)

The interpreted row of the evidence table above is wrong. Its figures, 2148
pass / 59 fail before and 2162 pass / 58 fail after, were read from the sweep
files when the interpreted sweep had covered 340 of the 348 directories. The
completed files give 2359 pass / 60 fail before and 2373 pass / 59 fail after.
The eight directories that were missing are `Libraries/Tensor2D` through
`Libraries/Wireless`; none of them uses Random.

The comparison itself is unchanged: the lines that differ between the two
completed files are the 12 removed and 25 added Random tests and the one
load-sensitive test named above. The compiled row was read from completed
files and is correct. M5 checks for the sweep's completion marker before it
reads a count.
