# Random v2 — M5: `Entropy` package

Date: 2026-10-02
Ladder: `internal/random/RANDOM_V2_LADDER.md`
Base commit: `a1f69b6`

## Verdict

**SUCCESS.** The four `Entropy` builtins of ladder section 3.5 work in the
interpreted and compiled lanes from one Go implementation, artifact evaluation
and capability discovery reject them, and the `Crypto*` coin and dice helpers
are gone from `Random`.

## What a user can now write

```oct
import Random

fn NoiseForThisRun() -> Random.Stream ! Error {
    let seed = Entropy.Seed()?
    Print(seed)
    return Random.Seeded(seed)
}

let roll    = Entropy.IntBetween(1, 6)!     // Int, closed range, no modulo bias
let draw    = Entropy.Unit()!               // Float in [0, 1)
let payload = Entropy.Bytes(32)!            // Bytes
```

`Entropy` needs no `import`.

## Decisions made in this milestone

These go beyond the ladder's text. Each is a judgment call that a reviewer may
want to reverse.

1. **`Entropy` is a compiler-owned namespace, like `Artifact`.** The ladder
   wrote section 3.5 as `package Entropy` with four `fn` declarations. All four
   are natives, so there is nothing to declare in Oct. Making the namespace
   compiler-owned means no import is needed, `.octfail` contracts can be
   written (an `.octfail` cannot import a library), and the three `Language`
   fixtures that call `Random.CryptoRandBytes` without an import can move in
   M6 without gaining one. `Libraries/Entropy` holds a marker file, as
   `Libraries/Artifact` does, so `import Entropy` still resolves. The naming
   rule is the Random v2 rule: only the qualified spelling outside the
   package, both spellings inside it, no redeclaration inside it. Section 3.5
   of the ladder now says this.
2. **A missing function in a compiler-owned namespace is reported as a missing
   function.** `Entropy.Int()` used to produce `unknown package 'Entropy'`,
   which is false for a namespace that needs no import. It now produces
   `package 'Entropy' has no function 'Int'`. The same change applies to
   `Array` and `Artifact`. Nothing in the repository depended on the old text.
3. **Two test seams in `internal/octrandom`.** The only `Error` an Entropy
   builtin returns is a failed read of the operating system's source, and the
   real source cannot be made to fail. `SetEntropySourceForTest` replaces the
   source in process, for the interpreted lane. The build tag `octentropyfail`
   compiles in a source that always fails, for the compiled lane, whose
   programs run in their own process. Neither is used outside tests. A program
   built with the tag gets errors, never weaker randomness.

## What changed

| File | Change |
|---|---|
| `internal/octrandom/entropy.go` (new) | `EntropySeed`, `EntropyUnit`, `EntropyIntBetween`, `EntropyBytes` over `crypto/rand`; `IsPrecondition`; `SetEntropySourceForTest` |
| `internal/octrandom/entropy_fail.go` (new) | Failing source under the `octentropyfail` build tag |
| `internal/builtin/random.go` | Table rows gain `Namespace`; four `entropy(...)` rows; lookups are by namespace because `Unit` and `IntBetween` exist in both packages. `IsRandomSymbol` is removed. |
| `internal/builtin/namespaces.go` | `Entropy` is compiler-owned |
| `internal/typecheck/typecheck.go` | The table-driven check handles both namespaces, skips the import check for a compiler-owned one and carries fallibility. Decision 2. |
| `internal/interpret` | `evalEntropyBuiltin`; the discovery guard resolves by calling package |
| `internal/build` | `emitEntropyCall` and four glue helpers; generated programs import `octrandom` |
| `Libraries/Entropy` (new) | Marker file, manifest, README, test |
| `Libraries/Random/Random.{CoinToss,Dice}.oct` | `CryptoFlipCoin`, `CryptoFlipBiasedCoin`, `CryptoFlipCoins`, `CryptoRollDie`, `CryptoRollD20`, `CryptoRollDice` and their two helpers removed, with their two test facts |

The interpreter and the generated program both call `internal/octrandom`, so
lane parity holds by construction, as for Random v2.

One cleanup rode along. `usesRandomHelpers` in the compiled backend did not
exclude v2 rows, so since M3 any program that called a v2 draw also had the
v1 xoshiro helper block and its imports emitted. That was harmless and
unneeded; it is now limited to v1 calls.

### Behavior

- Every call is fallible. The only `Error` is a failed read of the source.
- `lo > hi` and `count < 0` are non-recoverable runtime errors, checked before
  anything is read. `match` does not see them.
- `IntBetween` rejects and redraws, with a fresh word per try, so it has no
  modulo bias. `IntBetween(x, x)` returns `x` but still reads the source.
- `Seed` is any 64-bit `Int`, negative values included.
- `Bytes(0)` reads nothing and cannot fail.
- Artifact evaluation and capability discovery reject the call before it
  reads.

### Not removed

The v1 natives `CryptoRandInt`, `CryptoRandFloat01` and `CryptoRandBytes`
remain until M6, with the rest of v1. Three `Language` fixtures still call
`Random.CryptoRandBytes`; the ladder's M6 now lists them.

## Corrections to the ladder

- M5 asked for "`.octfail` for domain violations". Those are runtime
  failures and `.octfail` is compile-time only. This is the mistake M3 already
  corrected in its own section. The cases are in the precondition fixture.
- M5 said "bulk helpers use loops, not recursion". Section 3.5 has no Oct
  helpers; `Bytes` is native. The line applied to nothing.
- The Entropy README was scheduled for M6. It is written now, because the
  package ships now.

## Correction to the M4 report

The interpreted row of the M4 evidence table was read before the interpreted
sweep had finished: 340 of 348 directories. The report said 2148 pass / 59
fail before and 2162 / 58 after. The completed files say 2359 / 60 and 2373 /
59. The M4 comparison and verdict do not change, because the eight missing
directories do not use Random, and the lines that differ are the same.
`RANDOM_V2_M4.md` has an addendum. The baseline figures below come from the
completed files.

## Tests

| Where | What |
|---|---|
| `Language/Builtins/Entropy/valid` (7 facts) | Types, closed range, every face of a d6 reached, degenerate, negative and 63-bit ranges, `Unit` in `[0, 1)`, `Bytes` lengths, `!`, `?` and `match`, and that the bare names stay free for other packages. No import. |
| `Language/Builtins/Entropy/invalid` (16 `.octfail`) | Unhandled fallible result, `?` in an infallible function, argument count, each parameter type, a fallible argument, type arguments, result types, the unqualified name outside the package, an unknown function, redeclaration inside the package. |
| `Libraries/Entropy` (2 facts) | The unqualified spelling inside `package Entropy`. |
| `Libraries/RandomUsage` (+1 fact) | A stream seeded from `Entropy.Seed()` replays, with `import Entropy`. |
| `testdata/random_stream_preconditions` (+4 facts, 26 total) | `lo > hi` and `count < 0` stop both lanes with the same runtime error, and `match` does not catch them. |
| `testdata/entropy_source_failure` (6 facts) | With a source that always fails, in both lanes: every builtin returns an `Error`; `Assert.Error`, `match` and `?` see it; `!` stops the program with it; a precondition is still a runtime error. |
| `Language/Tooling/Artifacts/invalid/ambient_entropy_*` (4) | Artifact evaluation rejects each builtin and publishes nothing staged before it. |
| `Language/Tooling/ConceptCapabilitiesM2/invalid/entropy_discovery` | Capability discovery rejects `Entropy.Seed`. |
| `internal/octrandom/entropy_test.go` (8 Go tests) | With an injected source: one word per draw, the rejection loop on known words, range edges, classification of precondition against source failure, a short read. |
| `internal/builtin/random_test.go` | The table matches section 3.5 exactly; kind follows namespace, which is what the two guards rely on; namespace-scoped lookup. |

The ambient-randomness guard had no test before this milestone, for v1 or
otherwise.

Draws from the real source cannot be pinned to values. The Oct tests assert
ranges and coverage, and each fact that can fail by chance states its
probability (the largest is 2^-64). Exact values are checked in Go against an
injected source.

## Evidence

All runs on linux/amd64, Go 1.25.0. "Baseline" is `a1f69b6`.

| Check | Baseline | After M5 |
|---|---|---|
| `oct test Libraries/Random`, interpreted / compiled / auto | 64 passed | 62 passed in each mode (two `Crypto*` facts removed) |
| `oct test Libraries/RandomUsage`, each mode | 8 passed | 9 passed |
| `oct test Libraries/Entropy`, each mode | — | 2 passed |
| `oct test Language/Builtins/Entropy/valid`, each mode | — | 7 passed |
| `.octfail` under `Language` (root run) | 392 pass, 4 fail | 408 pass, same 4 fail |
| `.octfail` under `Libraries` (root run) | 62 pass | 62 pass |
| Preconditions test (`-tags=integration`), both lanes | 22 facts | 26 facts, pass |
| Source-failure test, both lanes | — | pass |
| `go test ./...` | 69 ok, 1 fail | 69 ok, same 1 fail |
| `go test -tags=integration ./...` | 68 ok, 2 fail | 68 ok, same 2 fail |
| Per-directory sweep, compiled | 2204 pass, 228 fail, 348 directories | 2212 pass, 228 fail, 350 directories |
| Per-directory sweep, interpreted | 2373 pass, 59 fail | 2379 pass, 61 fail |

Compiled sweep: the lines that differ are the 2 removed and 10 added tests,
and the first duplicate-declaration message in one fixture directory that
`oct test` cannot run as a directory anyway.

Interpreted sweep: the same lines, plus two tests in
`Experiments/FmBrownNoiseKalman/M4b` that exceeded their 30 s cycle limit
while the Go lanes ran beside the sweep. That directory passes 3 of 3 with
both trees when run alone. It is the load-sensitive directory reported in M3
and M4.

Go lanes: the first run, made while the interpreted sweep was running, had
one more failure, a timeout in `cmd/oct-mcp`
(`TestStdioGoldenFlowAndArtifactRetrieval`, `OCT-MCP-TIMEOUT`). It passes
alone, and the figures above are from a rerun on an idle machine.

The failures in both columns are the ones reported in M3 and M4: missing
tools and sidecars in this environment, UI builtins the compiled lane does
not support, the compiled-lane panic in `PrometheusMeasurementFilteringLab/M4`,
and `FmBrownNoiseKalman/M1`.

**Fault injection.** 48 deliberate faults, one at a time, in the Go
implementation, the table, the typechecker, the interpreter and the compiled
backend: preconditions removed, off-by-one span, rejection skipped, bounds
swapped, a constant or a wrong draw returned, errors swallowed, a precondition
turned into an `Error` and the reverse, fallibility dropped, the import made
required, the bare names reserved, each guard switched off, a missing emission
case.

The first pass found one real gap. Nothing exercised a source failure in the
compiled lane, so two faults there were not caught: the generated program
treating a source failure as fatal, and dropping the error. The compiled half
of the source-failure test, with the `octentropyfail` tag, was added for that.
All 48 are now caught.

## Found on the way, not fixed

Recorded in `FEEDBACK.md`:

- `Language/reference/language/06-errors.md` shows fallible `match` as an
  expression, `return match F() { ok(v) => v  err(_) => 3 }`, in three examples
  marked valid. The parser rejects them. Only the statement form with block
  arms works. The Entropy tests use the statement form.
- In the compiled lane, `err(_) => { ... }` generates Go that does not build.
  The interpreted lane accepts it. The Entropy tests name the binding.
- `Entropy` is not in `Language/reference`, and neither is `Random`. The
  reference's capability list names a future `Crypto.Random` family, which is
  what `Entropy` would fall under. M6's docs item now includes the reference.

`internal/random/Random.CoinToss.md` and `Random.Dice.md` still describe the
removed `Crypto*` helpers. They carry the superseded banner from M4 and are
retired in M6.

## Not run

- Windows and arm64.
- The `toolchain`-tagged Go lane and the slow wrapper lanes.
- A failure of the real operating system source. The failure path is tested
  with an injected source only.
