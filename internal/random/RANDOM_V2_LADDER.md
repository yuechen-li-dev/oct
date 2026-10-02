# Random v2 — Milestone Ladder Contract

Status: **ACCEPTED 2026-10-01.** M0–M5 are closed (`RANDOM_V2_M1.md` through `RANDOM_V2_M5.md`); M6 is not started.
Date: 2026-10-01

This document supersedes `internal/random/Random.Core.md` as the
source of truth for the `Random` API shape. The record-result / `Next` threading
model it describes is retired by this ladder.

---

## 1. Why

The v1 API threads a mutable xoshiro256** state through an immutable language:

```oct
let draw = Random.RandNormal(rng, 0.0, sigma)
rng = draw.Next
use(draw.Value)
```

Audit findings that drive this ladder (2026-10-01):

| # | Finding | Consequence |
|---|---|---|
| F1 | Every draw is three lines, and if `rng = draw.Next` is omitted the same draw silently repeats | Correlated draws with no diagnostic; the library's own tests reuse state |
| F2 | Draw consumption depends on parameter values (`stddev == 0`, `p ∈ {0,1}`, `min == max`, `Spike` outcome) | Zeroing one noise source shifts every later draw; e.g. M4 lab spikes move when a segment's jitter is 0 |
| F3 | `Random.Gaussian(Random.RngSeed(seed), 0.0, 0.0).Next` in 4 experiment files does nothing | Cargo-cult workaround left over from the dispatch bugs |
| F4 | `Rng` is a public record of four signed `Int`s; the all-zero state is constructible and is a xoshiro fixed point | Degenerate generator reachable from user code; the documented "opaque `type Rng`" is false |
| F5 | `Gaussian` is a builtin alias of `RandNormal`, so its Oct body (and precondition) never runs | Library source misstates the runtime behavior |
| F6 | Duplicate name tiers (`RandNormal`/`Gaussian`, `RandFloatRange`/`Uniform`, `RandBernoulli`/`Bernoulli`) using Go `math/rand` naming | Redundant surface; `Random.RandInt` says "random" twice |
| F7 | Seven result records, two structurally identical; `DiceRollResult.Total` means "selected die" for advantage | Naming drift |
| F8 | Seeded and crypto APIs share one package with different contracts (abort vs `! Error` for the same programmer error; loops vs recursion) | Two libraries in one namespace |
| F9 | Random builtin names are hand-maintained string lists in ~8 places across `builtin`, `interpret`, `typecheck`, `build` | Every API change touches all of them, so the API froze in its first shape |
| F10 | xoshiro / SplitMix / Box–Muller exist twice: once as interpreter Go, once as a Go source string in `emit_go_runtime.go` | Interpreted and compiled streams can drift silently |

## 2. Frozen decisions

| ID | Decision |
|---|---|
| D1 | **Counter-based generation.** Every draw is a pure function of `(Stream, index, params)`. No RNG state is threaded or returned. |
| D2 | **Philox4x32-10** (Salmon et al., SC'11 / Random123) is the sole bit generator. |
| D3 | **String labels** for `Fork`, hashed with FNV-1a 64 over UTF-8 bytes. |
| D4 | **Parameter order:** `(stream, index, params...)`. |
| D5 | **Native core is Go only**, in one package `internal/octrandom`, imported by both the interpreter and generated programs (precedent: `internal/prometheus`, `internal/octxiliary`, `internal/dimension`). No C/C++. No second hand-written copy. |
| D6 | **Crypto randomness moves to a separate `Entropy` package.** `Random` becomes fully deterministic. |
| D7 | **Runtime preconditions use `Assert.True`** (the sanctioned non-recoverable runtime path per `09-builtins.md`). `Require` is compile-time only and is **not** used for argument validation. `! Error` is reserved for genuine environmental failure (OS entropy). |
| D8 | **Clean break.** No deprecated v1 wrappers survive M6. Seeded sequences change; recorded experiment outputs are regenerated. |

## 3. Normative specification

### 3.1 Types

```oct
package Random

record Stream { _Key: Int }   // _Key holds the 64-bit Philox key (as Int bits)
```

Every `Int` value of `_Key` is a valid key. There is no degenerate state.
Constructing `Stream { _Key: k }` directly is permitted, but it is not the public
way to make a stream. (Enforcing source-level opacity is out of scope.)

Naming rules for the native primitives of §3.3:

- Outside package Random they exist only in the qualified spelling
  (`Random.Unit`), and the calling package must `import Random`. The bare words
  (`Unit`, `Normal`, `Between`, `Fork`, `Child`, `Seeded`, `IntBetween`) are not
  reserved: any other package may declare its own function with one of those
  names.
- Inside package Random both spellings name the builtin, and the package may
  not declare a function with one of those names.
- The natives have no Oct declarations and no stub bodies. `Random.Stream.oct`
  declares only the record `Stream`.

### 3.2 Bit generator

- `Philox4x32-10(ctr: [4]uint32, key: [2]uint32) -> [4]uint32`, exactly as in
  Random123 (multipliers `0xD2511F53`, `0xCD9E8D57`; Weyl constants
  `0x9E3779B9`, `0xBB67AE85`; 10 rounds).
- Key words: `k0 = low32(_Key)`, `k1 = high32(_Key)`.
- An `Int` value `n` maps to counter words as `low32(uint64(n))`, `high32(uint64(n))`.
- Output words `o0..o3` form two 64-bit words: `w0 = o0 | o1<<32`, `w1 = o2 | o3<<32`.

Counter layout, with the domain tag in word 3:

| Use | `c0` | `c1` | `c2` | `c3` (domain) |
|---|---|---|---|---|
| Draw at index `i`, block `j` | `low32(i)` | `high32(i)` | `j` | `0` |
| `Child(s, i)` | `low32(i)` | `high32(i)` | `0` | `1` |
| `Fork(s, label)` with `h = FNV1a64(utf8(label))` | `low32(h)` | `high32(h)` | `0` | `2` |

A derived key is `w0` of block 0 for the corresponding counter.

### 3.3 Native primitives (Go, `internal/octrandom`)

All native primitives fail non-recoverably on `i < 0`.

| Oct signature | Definition |
|---|---|
| `Seeded(seed: Int) -> Stream` | `_Key = seed` |
| `Fork(s: Stream, label: String) -> Stream` | Derived key, domain 2. Empty labels are allowed. |
| `Child(s: Stream, i: Int) -> Stream` | Derived key, domain 1 |
| `Unit(s: Stream, i: Int) -> Float` | `(w0 >> 11) · 2⁻⁵³` from block 0. Range `[0, 1)`. Exact. |
| `Between(s: Stream, i: Int, lo: Float, hi: Float) -> Float` | Requires `lo` and `hi` finite, `lo <= hi`, and `hi - lo` finite. Returns `lo` if `lo == hi`. Otherwise `r = lo + (hi - lo)·Unit`; if `r >= hi`, then `r = nextafter(hi, lo)`. Range `[lo, hi)`. |
| `IntBetween(s: Stream, i: Int, lo: Int, hi: Int) -> Int` | Requires `lo <= hi`; inclusive. `span = uint64(hi) - uint64(lo) + 1` (wrapping). If `span == 0`, returns `int64(w0)` of block 0. Otherwise consume words in order `b0.w0, b0.w1, b1.w0, b1.w1, …`, accept the first `w < 2⁶⁴ − (2⁶⁴ mod span)`, return `lo + int64(w mod span)`. Unbiased. |
| `Normal(s: Stream, i: Int, mean: Float, stddev: Float) -> Float` | Requires `stddev >= 0` and both finite. From block 0: `u1 = ((w0 >> 11) + 1)·2⁻⁵³ ∈ (0,1]`, `u2 = (w1 >> 11)·2⁻⁵³`, `z = sqrt(−2 ln u1)·cos(2π u2)`; returns `mean + stddev·z`. `stddev == 0` returns `mean` with no special-case path. |

### 3.4 Oct library layer (pure Oct over the native primitives)

| Signature | Definition |
|---|---|
| `Chance(s, i, p: Float) -> Bool` | `Assert.True(0 <= p <= 1)`; `Unit(s, i) < p` |
| `Exponential(s, i, rate: Float) -> Float` | `Assert.True(rate > 0)`; `−Ln(1 − Unit(s, i)) / rate` |
| `Units(s, count: Int) -> Float[]` | `Assert.True(count >= 0)`; element `k = Unit(s, k)` |
| `Normals(s, count, mean, stddev) -> Float[]` | `Assert.True(count >= 0)`; element `k = Normal(s, k, mean, stddev)` |
| `Spike(s, i, probability, amplitude) -> Float` | `Assert.True(amplitude >= 0)`; `c = Child(s, i)`; if `Chance(c, 0, probability)` then `±amplitude` by `Chance(c, 1, 0.5)`, else `0.0` |
| `FlipCoin(s, i) -> CoinSide` | `Heads` iff `Chance(s, i, 0.5)` |
| `FlipCoins(s, i, count) -> CoinSide[]` | `Assert.True(count >= 0)`; element `k = FlipCoin(Child(s, i), k)` |
| `RollDie(s, i, sides) -> Int` | `Assert.True(sides >= 2)`; `IntBetween(s, i, 1, sides)` |
| `RollDice(s, i, count, sides) -> Int[]` | `Assert.True(count >= 0)` and `Assert.True(sides >= 2)`; element `k = RollDie(Child(s, i), k)` |
| `RollWithAdvantage(s, i, sides) -> Int` | max of `RollDice(s, i, 2, sides)` |
| `RollWithDisadvantage(s, i, sides) -> Int` | min of `RollDice(s, i, 2, sides)` |
| `CountHeads`, `CountTails`, `CoinSideToString`, `enum CoinSide` | unchanged |

Kept under these names: `Exponential`, `Spike`.
Removed with no replacement (each is a one-liner in the new API):

- `Jitter` → `Between(s, i, −a, a)`
- `DriftStep` → `x + Normal(s, i, 0.0, sd)`
- `Uniform` → `Between`
- `Bernoulli` → `Chance`
- `FlipBiasedCoin` → `Chance`
- `RollD4`..`RollD100` → `RollDie`
- `RollDiceSum` → `Sum(RollDice(...))`

### 3.5 `Entropy` package

```oct
Entropy.Seed() -> Int ! Error                       // for Random.Seeded(Entropy.Seed()!); record the value
Entropy.IntBetween(lo: Int, hi: Int) -> Int ! Error // uniform on the closed range, no modulo bias
Entropy.Unit() -> Float ! Error                     // uniform on [0, 1), a multiple of 2^-53
Entropy.Bytes(count: Int) -> Bytes ! Error
```

`Entropy` is a compiler-owned namespace, like `Artifact`: the four functions
are builtins with no Oct declarations, and calling them needs no `import`.
`Libraries/Entropy` exists so that `import Entropy` resolves and so that the
package has a manifest, a README and tests. The naming rule is the one that
governs the Random v2 builtins: outside package Entropy only the qualified
spelling names a builtin and the bare names are not reserved; inside it both
spellings do, and the package cannot redeclare them.

Domain violations (`lo > hi`, `count < 0`) are non-recoverable, not `Error`.
Only OS entropy failure is an `Error`. Artifact evaluation and capability
discovery reject every `Entropy.*` call before it reads anything, carrying
over the existing ambient-randomness guard. `CryptoFlipCoin*` and
`CryptoRollDie*` are removed and have no replacement: seed a stream and use
the Random library, or call `Entropy.IntBetween`.

### 3.6 Global invariants

- **I1 Purity.** Equal arguments give equal results, in any evaluation order.
- **I2 Bulk–scalar identity.** `Units(s, n)[k] == Unit(s, k)` and `Normals(s, n, m, sd)[k] == Normal(s, k, m, sd)`.
- **I3 Parameter isolation.** Changing the parameters of a draw on one `(stream, index)` never changes any other draw.
- **I4 Lane parity.** Interpreted and compiled execution produce bit-identical results on the same `GOOS/GOARCH`. This holds by construction: there is one Go implementation.
- **I5 Cross-platform determinism.**
  - Integer-derived outputs (`Fork`, `Child`, `IntBetween`, `Unit`, `Between`, `Chance`, coins, dice) are bit-identical on every platform.
  - Transcendental outputs (`Normal`, `Exponential`) are only guaranteed identical within one `GOARCH`. Go may fuse multiply-adds on arm64/ppc64le/s390x, including inside `math.Log` and `math.Cos`.
  - The `internal/octrandom` code applies explicit `float64(...)` rounding to its own expressions so no fusion happens in our code.
- **I6 No library name is a builtin alias.** Only §3.3 names are native. Everything else runs its Oct body. (Fixes F5.)

## 4. Milestones

Each milestone has a verdict line, `SUCCESS` / `PARTIAL` / `BLOCKED`, recorded in
`internal/random/RANDOM_V2_M<n>.md` when it closes. Milestones run in order. No
milestone may weaken an existing compiled-lane assertion to pass (see AGENTS.md).

### M0 — Contract freeze
- **Scope:** Review and accept this document. Commit it as `internal/random/RANDOM_V2_LADDER.md`. Add a supersession banner to `Random.Core.md`.
- **Exit:** Document accepted. No code changes.
- **Verdict:** SUCCESS (accepted 2026-10-01).

### M1 — `internal/octrandom` Go core
- **Scope:**
  - Philox4x32-10 and FNV-1a 64.
  - Key derivation (§3.2).
  - `Unit`, `Between`, `IntBetween`, `Normal` (§3.3) as plain Go functions over `uint64` keys.
  - No Oct wiring.
- **Tests:** Go tests in `internal/octrandom`. This is host-side implementation validation, which the AGENTS.md exception permits. They cover:
  - Random123 published philox4x32-10 known-answer vectors, copied verbatim from upstream `kat_vectors` with a source citation.
  - FNV-1a 64 reference vectors.
  - Edge cases: `IntBetween` with full `Int` range, `lo == hi`, and `span = 2⁶³ + 1` (worst rejection rate); `Between` upper-bound clamp; `Normal` with `u1 = 1`.
- **Exit:** `go test ./internal/octrandom` is green. No other package imports `octrandom` yet.
- **Verdict:** SUCCESS — see `internal/random/RANDOM_V2_M1.md`.

### M2 — Builtin registry consolidation (no behavior change)
- **Scope:**
  - Replace the scattered Random name lists (F9) with one table in `internal/builtin`: qualified name, unqualified alias policy, signature, and the name of the implementing builtin. Execution stays in the interpreter and the backend, keyed by that name, as `internal/builtin/definition.go` already requires for every other builtin.
  - `typecheck`, `interpret`, `build/lower*` and `emit_go*` consult the table.
  - The v1 API is still the one being served.
- **Exit:**
  - Random builtin names appear in exactly one table plus their implementations (checked by grep).
  - `go test ./internal/builtin ./internal/typecheck ./internal/interpret ./internal/build` is green.
  - `oct test Libraries/Random` is green in both the default and `--execution compiled` lanes, with no test edits.
- **Rationale:** Separating plumbing from the API change keeps one variable per milestone. If M3 breaks, the registry is already proven.
- **Verdict:** SUCCESS — see `internal/random/RANDOM_V2_M2.md`.

### M3 — Native v2 primitives wired, both lanes
- **Scope:**
  - Register the §3.3 natives through the M2 table.
  - The table rows for the v2 natives carry parameter types, and the typechecker checks both argument count and argument types from them. v1 checks neither argument types nor the entropy builtins' argument count (M2 report, "v1 defects"); v2 must not inherit that.
  - The interpreter calls `octrandom` directly. Generated programs import `github.com/yuechen-li-dev/oct/internal/octrandom` through the existing staged-build path.
  - Add a new `Random.Stream.oct` declaring the record `Stream`. The natives are compiler-owned builtins with no Oct declarations. The v1 API stays side by side, untouched.
- **Tests:**
  - `Libraries/Random/Random.Stream.octest`, run in both lanes:
    - Golden values for each primitive at a fixed set of `(seed, label, index)` inputs. These are a regression lock taken from M1 output, not an independent correctness proof; M1's KATs are that proof.
    - I1 purity.
    - I3 parameter isolation, with the M4-lab shape as the explicit regression: `Normal(..., sd = 0)` on one fork leaves draws on another fork unchanged.
    - Fork independence: different labels give different streams; the same label gives the same stream.
    - Range checks.
  - `Libraries/Random/Random.Stream.invalid.*.octfail`: compile-time contracts for argument count, every parameter type, type arguments, fallible arguments and redeclaration.
  - `Libraries/RandomUsage`: use from another package, including that the bare builtin names are not reserved there, and `.octfail` contracts for a missing import and an unqualified name.
  - Runtime preconditions (`i < 0`, `lo > hi`, `stddev < 0`): a fixture under `testdata/random_stream_preconditions` driven by `cmd/oct/random_stream_preconditions_test.go` (integration lane), which requires both lanes to stop with the same error. The original text of this milestone asked for `.octfail` contracts here; that was wrong, because `.octfail` is compile-time only and these are runtime failures. Non-finite arguments cannot be written as Oct literals and are covered by the M1 Go tests.
- **Exit:**
  - Both lanes green and producing identical golden values.
  - `grep` finds no Philox code in `emit_go_runtime.go`.
- **Verdict:** SUCCESS — see `internal/random/RANDOM_V2_M3.md`.

### M4 — Oct library layer
- **Scope:**
  - Implement §3.4 in Oct: `Random.Sampling.oct` (Chance, Exponential, Units, Normals, Spike), plus rewritten `Random.CoinToss.oct` and `Random.Dice.oct`.
  - The v2 functions replace the seeded v1 library layer; they cannot sit beside it. Oct has no overloading, and six §3.4 names were already v1 functions in package Random with different signatures (`Exponential`, `Spike`, `FlipCoin`, `FlipCoins`, `RollDie`, `RollDice`). Removed with it: `Uniform`, `Bernoulli`, `Jitter`, `DriftStep`, `FlipBiasedCoin`, `RollD4`..`RollD100`, `RollDiceSum`, `RollD20Advantage`, `RollD20Disadvantage` and the four library result records.
  - The v1 natives (`RngSeed`, `Rand*`, `Gaussian`, `CryptoRand*`) and the `Crypto*` coin and dice helpers stay, unchanged, for M5 and M6.
  - The one caller of the removed layer outside the library, `Experiments/PrometheusMeasurementFilteringLab/M2`, keeps `Jitter`, `Spike` and `DriftStep` as local functions defined exactly as v1 defined them, so its recorded results do not change. It moves to v2 with the other experiments in M6.
- **Tests:**
  - Expected values for every library function, computed from `internal/octrandom` outside the Oct code.
  - I2 bulk–scalar identity.
  - `Child` sub-indexing for `Spike`, `FlipCoins` and `RollDice`.
  - Advantage ≥ disadvantage at the same `(s, i)`.
  - Sanity bands:
    - mean of `Units(s, 10000)` in `[0.48, 0.52]`
    - mean and variance of `Normals(s, 10000, 0, 1)` within `±0.05` / `±0.1`
    - each face count of `RollDice(s, 0, 6000, 6)` within `[850, 1150]`
  - Runtime preconditions of the library layer, added to the M3 fixture and Go test.
- **Exit:** Both lanes green. No `Assert.True(false, ...)` dispatch stubs remain in v2 sources.
- **Verdict:** SUCCESS — see `internal/random/RANDOM_V2_M4.md`.

### M5 — `Entropy` package
- **Scope:**
  - Add `Libraries/Entropy` (§3.5), with natives registered through the M2 table and implemented in `octrandom` over `crypto/rand`.
  - The table rows carry parameter types, and the typechecker checks argument count, argument types and fallibility from them, as for the Random v2 natives.
  - The artifact ambient-randomness guard and the capability-discovery guard cover `Entropy.*`.
  - Remove the `Crypto*` coin and dice helpers that M4 left in `Random.CoinToss.oct` and `Random.Dice.oct`.
  - The original text of this milestone also said "bulk helpers use loops, not recursion". §3.5 has no Oct helpers: the only bulk function, `Bytes`, is native. The line had nothing to apply to.
- **Tests:**
  - `Language/Builtins/Entropy/valid`: types, ranges and every handling form from another package without an import, in both lanes.
  - `Language/Builtins/Entropy/invalid/*.octfail`: compile-time contracts for fallibility, argument count, every parameter type, result types, type arguments, the unqualified name outside the package, an unknown function and redeclaration.
  - `Libraries/Entropy`: the unqualified spelling inside the package. `Libraries/RandomUsage`: seeding and replaying a stream, with `import Entropy`.
  - Runtime preconditions (`lo > hi`, `count < 0`): added to the M3 fixture and Go test, including that `match` does not catch them. The original text asked for `.octfail` here; that was wrong for the reason given under M3.
  - Artifact-evaluation rejection for each of the four builtins, and capability-discovery rejection for `Entropy.Seed`.
  - Go tests in `internal/octrandom` with an injected source: the rejection loop, classification of precondition against source failure, and a short read.
  - Source failure as an ordinary `Error`: `testdata/entropy_source_failure`, run in both lanes by `cmd/oct/entropy_source_failure_test.go` and `entropy_source_failure_compiled_test.go` against a source that always fails. The interpreted lane replaces the source in process. The compiled lane builds the generated program with the `octentropyfail` build tag, under which `internal/octrandom` reads from a failing source.
- **Exit:** Both lanes green.
- **Verdict:** SUCCESS — see `internal/random/RANDOM_V2_M5.md`.

### M6 — Migration and v1 removal
- **Scope:**
  - Migrate `Experiments/FmBrownNoiseKalman/{M0,Shared}` and `Experiments/PrometheusMeasurementFilteringLab/M1–M4` to v2, using one `Fork` per noise source and deleting the F3 idiom.
  - In `PrometheusMeasurementFilteringLab/M2`, replace the local `M2JitterV1`, `M2SpikeV1` and `M2DriftStepV1` functions that M4 introduced.
  - Regenerate `m2_random_*_summary.octagon` and any other recorded outputs, noting in each experiment's report that the regeneration came from the Random v2 stream change.
  - Delete:
    - `Rng`, `Rand*`, `RngSeed`, all `*Result` records, `Crypto*` from `Random`
    - the `Legacy` flag and the three arity-check forms in the builtin table, which exist only for v1
    - the v1 natives, the xoshiro code, and the v1 `CompiledDispatch.*` tests
  - Move the three fixtures that call `Random.CryptoRandBytes` to `Entropy.Bytes`: `Language/Testing/CompiledOctxiliary/valid/generic_wrapper_m6.octest`, `Language/Testing/CompiledOctxiliary/valid/file_bytes_and_directory_m4.octest` and `Language/Testing/InterpretedOctxiliary/valid/interpreted_generic_wrapper_w7b.octest`. They call it without `import Random`, which v1 allows and v2 does not.
  - Update `Language/Types/Tuples/invalid/random_tuple_threading_rejected.octfail` so it no longer depends on `Random` (rename it to a neutral tuple contract).
  - Docs:
    - Retire `internal/random/Random.{Core,CoinToss,Dice,Distributions}.md` into one `Random.md` generated from this spec.
    - Update `Libraries/Random/README.md` and `Libraries/Entropy/README.md` (written in M5).
    - Add Random and Entropy to `Language/reference` (see `FEEDBACK.md`).
    - Mark the "Random API updates" section of `LIBRARY_MODERNIZATION_AFTER_POW_UNITS_RANDOM.md` superseded.
    - Bump the manifest to `0.2.0` and add a CHANGELOG entry.
- **Exit:**
  - `rg "RngSeed|RandInt|RandFloat01|RandNormal|\.Next\b" --type-add 'oct:*.{oct,octest,octfail}' -t oct` finds no Random v1 usages. Unrelated `.Next` fields are allowed.
  - `go test ./...` is green.
  - `oct test Libraries` and `oct test Experiments` are green in both lanes.

## 5. Explicitly out of scope

- `template fn Pick<T>` / `Shuffle<T>`. These are the next ladder, once the template application story for library packages is exercised.
- SDSL-V / Prometheus Philox kernel for GPU stream parity. It will be validated against the same M1 KATs and M3 goldens when attempted.
- Source-level type opacity for `Stream`.
- Additional distributions (Poisson, Gamma, Beta, …).
- Performance work. Native bulk `Units`/`Normals` only if a benchmark justifies it.

## 6. Risks

| Risk | Mitigation |
|---|---|
| Generated-program import of `internal/octrandom` hits a staging edge case | The precedent path already exists; M3 exit requires a compiled-lane run |
| M3 goldens taken from our own implementation are circular | Correctness rests on M1's external KATs; goldens only lock lane parity and regressions |
| Cross-arch float drift in `Normal`/`Exponential` | Scoped honestly in I5; integer-derived outputs carry the strong guarantee |
| Experiment conclusions shift after regeneration | Expected (D8); regenerated outputs are reviewed for qualitative agreement, not bit equality |
