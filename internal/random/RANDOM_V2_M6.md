# Random v2 — M6: migration and v1 removal

Date: 2026-10-03
Ladder: `internal/random/RANDOM_V2_LADDER.md`
Base commit: `c325f94`

## Verdict

**SUCCESS**, with two qualifications stated up front.

- Random v1 is gone from the language, the libraries and the experiments.
  `Random` is version 0.2.0. No test that passed at the base commit fails now.
- The ladder's exit said `go test ./...`, `oct test Libraries` and
  `oct test Experiments` are "green". They were not green at the base commit
  and are not green now, for reasons outside Random (below). The exit applied
  is no regression against the base commit.
- One experiment directory, `FmBrownNoiseKalman/M2`, still holds recorded
  outputs made with Random v1. Its artifact entry points do not run at the base
  commit, so there was nothing to regenerate them with.

## What changed

### Experiments

| Experiment | Change |
|---|---|
| `FmBrownNoiseKalman/M0`, `Shared` | `GenerateWhiteNoise` is `Random.Normals(Random.Fork(Random.Seeded(seed), "white-noise"), n, 0.0, 1.0)`. The F3 idiom is deleted. |
| `PrometheusMeasurementFilteringLab/M1`, `M3`, `M4` | One stream per noise source, forked by name from the scenario seed (`"jitter"`, `"spike"`); `Random.Normal` and `Random.Spike` at the sample index. |
| `PrometheusMeasurementFilteringLab/M2` | Rewritten against v2. `M2JitterV1`, `M2SpikeV1` and `M2DriftStepV1` are gone. The milestone now characterizes streams: purity, and independence of sources. |

Seeds are unchanged. Noise realizations are not, so every recorded number that
depends on noise changed. Before regenerating anything, each set of recorded
files was reproduced byte for byte from the v1 code at the base commit. The
differences in this change therefore come from the generator and from nothing
else.

What moved:

| Experiment | Before | After |
|---|---|---|
| Lab M1, six regimes | — | Four recommendations identical. `slow-drift`: recommendation unchanged, quality-only winner `trimmed-5` → `median-3`. `mixed-hostile`: recommendation `median-5` → `ema-0.4`. |
| Lab M2 | — | Verdict unchanged. |
| Lab M3 | — | Selected implementation set changed in four of six regimes. |
| Lab M4 | — | Selected policy unchanged (`dominatus-mincommit-hysteresis`). Best non-oracle composite changed in one scenario, `spike-heavy-to-stable`. |
| Kalman M4b, scalar adaptive wins of 27 | 15 | 11 |
| Kalman M4b, mean change in output SNR | −0.103 dB | −0.017 dB |
| Kalman M6, guarded adaptive wins of 27 | 15 | 9 |
| Kalman M6, equivalence passes | 0 | 2 |

The reading of each experiment is the same as before. The counts that moved
are counts over one noise realization per case; the Kalman win count falling
from 15 to 9 says the count was never a rate. Each experiment's report has a
"Random v2 migration" section with the detail, and `FmBrownNoiseKalman/M4/FINDINGS.md`,
which is written by hand, is updated.

### Removed

- Oct: `Libraries/Random/Random.Core.oct` (`Rng` and the `*Result` records),
  `Random.Core.octest`, the two `CompiledDispatch.*` tests, and
  `Random.Stream.invalid.LegacyStateArgument.octfail`.
- Go: the v1 rows of the builtin table, the `Legacy` flag, the three
  arity-check forms, `ImplementedBy`, `LookupRandom`; `checkLegacyRandomBuiltinCall`
  in the typechecker; the xoshiro, SplitMix, Box–Muller and crypto helpers in
  the interpreter; the 86-line `__octRandomHelpers` source string and the v1
  emission cases in the compiled backend. About 500 lines of Go, net of test
  files.
- Docs: `Random.Core.md`, `Random.CoinToss.md`, `Random.Dice.md` and
  `Random.Distributions.md`, replaced by `internal/random/Random.md`.

The builtin table now holds eleven rows: seven for `Random`, four for
`Entropy`.

### Moved

- Three `Language/Testing` fixtures call `Entropy.Bytes` where they called
  `Random.CryptoRandBytes`.
- `Language/Types/Tuples/invalid/random_tuple_threading_rejected.octfail` is
  `state_threading_destructuring_rejected.octfail` and no longer mentions
  Random.

### Docs

- `internal/random/Random.md`: the specification, with a table from each v1
  name to its replacement.
- `Language/reference/language/17-standard-libraries.md`: `Random`, `Entropy`
  and "Compiler-owned namespaces" sections. The examples in them were run in
  both lanes.
- `Language/reference/tooling/31-octest.md` and `runtime/21-octomata.md`: two
  sentences that said "crypto-random" and "RNG state" now name `Entropy` and
  streams.
- READMEs of `Random`, `Random/tests`, `RandomUsage`, `Entropy`.
- `Random` manifest and registry entry at `0.2.0`; `RandomUsage` depends on
  `0.2.0`. CHANGELOG entry.

## Found on the way

### Fixed: the compiled lane evaluated both branches of an `if` expression

`PrometheusMeasurementFilteringLab/M4` has failed in the compiled lane since
before this ladder (0 of 7, `index out of range [-1]`), and M3, M4 and M5
reported it as unrelated. It is unrelated to Random, and it is a compiler bug:
`lowerIfExpr` lowered the then and else expressions into the block that held
the condition and used the branch only to pick a result. So
`if i > 0 { xs[i - 1] } else { 0.0 }` indexed at `-1`.

The fix lowers each branch in its own block. The existing contract
`IfExpressionSkipsNonSelectedBranchEvaluation` failed in the compiled lane for
this reason and now passes. A new fixture adds six facts (indexing, unwrapping
and dividing in an untaken then or else branch, and nesting in either).
`Language/ControlFlow/IfExpression/valid` went from 4 pass / 5 fail to
11 pass in the compiled lane, and Lab M4 from 0 / 7 to 7 / 0.

This is outside the ladder's scope. It is here because M6 had to run that
experiment in both lanes to regenerate it.

### Not fixed

- **Kalman artifact entry points.** `oct artifact` rejects the entry points of
  M3, M4, M4b, M5 and M6 at the base commit: each writes a file and then reads
  it back. The read-backs are removed, which is what made regeneration
  possible. M2 has a second fault (a duplicate output path) and is left alone.
- **The ladder's `RollDiceSum` replacement was wrong.** It said
  `Sum(RollDice(...))`. Oct has no `Sum` over `Int[]`. The ladder and
  `Random.md` now say "a loop".
- **Two diagnostics for one mistake.** A missing `import Random` gives an
  import hint when the function is a builtin and `unknown package 'Random'`
  otherwise.

All three are in `FEEDBACK.md`.

## Decisions a reviewer may want to reverse

1. **The if-expression fix is in this change.** It could be its own commit; it
   is delivered as a separate group of files for that reason.
2. **Read-backs removed from five Kalman artifact entry points.** The
   alternative was to leave those outputs on v1 noise, as M2 is.
3. **Historical reports keep their text.** `RANDOM_M1B`, `RANDOM_M8` and
   `RANDOM_M8B` describe v1 and cite deleted documents. Each got a one-line
   banner, not a rewrite. The same goes for the tuple reports under
   `internal/language` and `Experiments/RandomApiBakeoff`, which are untouched.
4. **`Entropy` is not tied to the `Crypto.Random` capability family** in the
   reference. Ordinary execution allows `Entropy` with no grant, so the tie
   would be a language decision.
5. **The removed names have two contracts, not ten.** One `.octfail` inside
   package Random (`RngSeed` is undefined) and one from another package. A Go
   test checks that none of the ten v1 names resolves.

## Tests added

| Where | What |
|---|---|
| `Libraries/Random/Random.Stream.invalid.RemovedGeneratorState.octfail` | `RngSeed` is undefined inside package Random. |
| `Libraries/RandomUsage/Random.Usage.invalid.RemovedV1Builtin.octfail` | A v1 builtin called with no import, which v1 allowed, is rejected. |
| `internal/builtin/random_test.go` | No v1 name is reserved or resolves, inside package Random or from another package. |
| `Language/ControlFlow/IfExpression/valid/if_expression_evaluates_only_taken_branch.octest` (6 facts) | See above. |
| `PrometheusMeasurementFilteringLab/M2` (+2 facts) | Sources do not depend on each other's parameters; two sources at one seed share no draw. |
| `FmBrownNoiseKalman/M0` (+1 fact) | A seed replays its noise; another seed gives other noise. |

## Evidence

linux/amd64, Go 1.25.0. "Base" is `c325f94`.

| Check | Base | After M6 |
|---|---|---|
| Per-directory sweep, compiled, 352 directories | 2220 pass, 228 fail | 2228 pass, 220 fail |
| Per-directory sweep, interpreted | 2394 pass, 54 fail | 2394 pass, 54 fail |
| `.octfail` under `Language` (root run) | 408 pass, 4 fail | 408 pass, same 4 fail |
| `.octfail` under `Libraries` (root run) | 62 pass | 63 pass |
| `go test ./...` | 69 ok, 1 fail | 69 ok, same 1 fail |
| `go test -tags=integration ./...` | 68 ok, 2 fail | 68 ok, same 2 fail |
| `oct test Libraries/Random`, each mode | 62 | 56 (six v1 facts and one v1 `.octfail` removed, one `.octfail` added) |
| `oct test Libraries/RandomUsage`, each mode | 9 | 10 |
| Exit grep for v1 names in Oct sources | — | Two hits, both the "removed" contracts. `.Next` hits are Octomata fields. |

The sweeps were taken before the last four facts were added: two in the
if-expression fixture, one in Lab M2 and one in Kalman M0. Those three
directories were rerun afterwards in both lanes and pass. The totals in the
table do not include those four facts.

Compiled sweep, line by line: 8 tests removed with v1, 8 added, and 8 that
failed now pass (seven in Lab M4, one if-expression contract). Nothing that
passed fails. Interpreted sweep: the same 8 removed and 8 added, and no other
line differs.

The Go lane baseline was measured on the M5 tree, which differs from `c325f94`
by one experiment directory.

### What still fails, before and after

- `internal/sdslv/test` (no `dxc` here) and, in the integration lane,
  `internal/document` (no LaTeX here).
- Compiled lane: tests that need wrapper sidecars, and UI builtins the
  compiled lane does not support.
- `FmBrownNoiseKalman/M1`, `OctErgonomicsLab/M0` and `M1`: a `[Fact]` calls
  `Artifact.Write*`.
- `FmBrownNoiseKalman/M6`, interpreted: both tests exceed the 30 s cycle
  limit on this machine, with either tree, run alone. They pass compiled.
- `PrometheusSgemmAlgorithmLab` M19 and M32 (the same limit), M42 and M43
  (tests with no assertions), `TrialBatchSimulation/M0`.
- Four `.octfail` under `Language` for `Array.Where` and `Array.CrossSection`.

### Fault injection

Twelve deliberate faults, one at a time.

| Area | Faults | First pass |
|---|---|---|
| If-expression lowering | then branch lowered before the branch; else branch lowered in the then block; either branch's jump written to the branch's entry block; targets swapped | 2 of 5 survived |
| v1 removal | a v1 row back in the table; package Random allowed to redeclare a builtin; the import check off | all caught |
| Lab M2 | every source on one stream; the seed ignored | 1 of 2 survived |
| Kalman M0 | noise ignores the seed; noise with zero variance | 1 of 2 survived |

The four survivors showed real gaps. The if-expression fixture only put the
unsafe expression in the else branch. Lab M2's independence test holds on a
shared stream, because a counter-based draw is isolated from other draws'
parameters with or without `Fork`; what `Fork` adds is that two sources do not
read the same draw, and nothing checked that. Kalman M0 never checked that its
noise depends on its seed. Tests were added for each, and all twelve are now
caught.

Not injected: faults in the migrated generators of Lab M1, M3 and M4. Their
tests check shapes and sanity bounds, not which stream a draw comes from, so
swapping two fork labels there would not be caught. The recorded outputs are
the record for those.

## Not run

- Windows and arm64.
- The `toolchain`-tagged Go lane and the slow wrapper lanes.
- `oct artifact` for `FmBrownNoiseKalman/M2`.
