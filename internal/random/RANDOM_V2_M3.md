# Random v2 — M3: native stream primitives in both lanes

Date: 2026-10-01
Ladder: `internal/random/RANDOM_V2_LADDER.md`
Base commit: `51e53a2`

## Verdict

**SUCCESS.** The seven Random v2 primitives of ladder section 3.3 (`Seeded`,
`Fork`, `Child`, `Unit`, `Between`, `IntBetween`, `Normal`) work in the
interpreted and compiled lanes, produce the same expected values in both, and
are type-checked for argument count and argument types. Both lanes call the one
Go implementation in `internal/octrandom`. The v1 API is still present and its
behavior is unchanged.

One open issue for M4 was found and needs a decision; see "Open issue for M4".

## What a user can now write

```oct
import Random

let noise  = Random.Seeded(42)
let jitter = Random.Fork(noise, "jitter")
let x      = Random.Normal(jitter, i, 0.0, 0.5)   // pure function of (stream, i)
```

Rules that M3 fixes in place (also added to the ladder, section 3.1):

- Outside package Random the builtins exist only in the qualified spelling and
  need `import Random`. The bare words `Unit`, `Normal`, `Between`, `Fork`,
  `Child`, `Seeded` and `IntBetween` are **not** reserved: another package may
  declare functions with those names.
- Inside package Random both spellings name the builtin, and the package cannot
  declare a function with one of those names.
- The natives have no Oct declarations and no stub bodies.
  `Libraries/Random/Random.Stream.oct` declares only `record Stream`.
- The Float parameters are dimensionless `Float`. A dimensioned argument such
  as `1.0m` is rejected.

## What changed

| File | Change |
|---|---|
| `internal/builtin/random.go` | Seven v2 rows with parameter types. A `Legacy` flag on the v1 rows, which keeps the v1 quirks (unqualified name reserved everywhere, stub allowed, arguments unchecked) confined to v1. |
| `internal/typecheck/typecheck.go` | `checkRandomStreamCall`: import, type arguments, argument count, every argument type, result type. The checker now knows which package it is checking. Redeclaring a v2 builtin inside package Random is rejected. |
| `internal/interpret/random_builtin.go`, `interpret.go` | `evalRandomStreamBuiltin` calls `internal/octrandom` and reports a violated precondition as `runtime error: ...`. |
| `internal/build/emit_go_random.go`, `emit_go.go` | Generated programs import `internal/octrandom`. A small emitted adapter converts `Random_Stream` to and from the package's key type and panics with the same `runtime error: ...` text. It contains no generation logic. |
| `internal/build/emit_go.go` | **Compiled-lane bug fix, not specific to Random:** see "Compiled record-copy bug". |
| `Libraries/Random/Random.Stream.oct` | `record Stream { _Key: Int }` and documentation. |

Lowering needed no change: the table lookups added in M2 already resolve the
new names and their result types.

Exit check: `internal/build/emit_go_runtime.go` contains no Philox code and no
reference to `octrandom`; the only generator code in the emitted prelude is
still the v1 xoshiro block.

## Tests added

| Where | What |
|---|---|
| `Libraries/Random/Random.Stream.octest` (14 facts) | Expected values for every primitive, including indices above 32 bits, the largest index, a non-ASCII label and the rejection path of `IntBetween`; purity and order independence (I1); parameter isolation in the measurement-lab shape (I3); fork and child independence; range checks; both spellings inside package Random; `Stream` as an ordinary record. |
| `Libraries/Random/Random.Stream.invalid.*.octfail` (17) | Argument count, each parameter type (stream, index, float, dimensioned float, label, seed, int bound, stddev, child index), a v1 `Rng` passed as a stream, type arguments, an unhandled fallible argument, `?` on an infallible draw, redeclaration, and the qualified spelling. |
| `Libraries/RandomUsage` (new package; 4 facts, 2 `.octfail`) | Use from another package: the qualified builtins and their types, streams in records and arrays, a stream on a flow board drawing by tick, bare names not reserved; a missing import; an unqualified name. Modelled on `Libraries/ArtifactUsage`. |
| `testdata/random_stream_preconditions` + `cmd/oct/random_stream_preconditions_test.go` (integration lane) | Eight facts, each violating one runtime precondition. The Go test requires both lanes to stop every fact with the same `runtime error: random: ...` text. |
| `Language/Testing/CompiledArrayLowering/valid/record_copy_keeps_every_field.octest` (3 facts) | Contract for the record-copy bug below. |
| `internal/builtin/random_test.go`, `coverage_test.go` | The v2 signatures match ladder 3.3 exactly; name reservation rules; table invariants. |

Expected values come from `internal/octrandom`, whose Philox core is checked
against the published Random123 vectors (M1). They lock the layout and lane
parity; they are not an independent proof of correctness. `Normal` is compared
with a tolerance of 1e-12 because its last bits may differ between processor
architectures (ladder I5); every other draw is compared exactly.

### Correction to the ladder

The M3 text asked for `.octfail` contracts for `i < 0`, `lo > hi` and
`stddev < 0`. That was wrong: `.octfail` is compile-time only, and those are
runtime failures. They are covered by the fixture and Go test above instead.
Non-finite arguments cannot be written as Oct literals and stay covered by the
M1 Go tests. The ladder text is corrected.

## Compiled record-copy bug (found and fixed)

Before M3, the compiled lane silently zeroed any record field whose name is not
an exported Go identifier (one starting with `_` or a lower-case letter)
whenever the record was copied as part of an array: an array literal, or
`let copy = records`. The generated `__octCloneValue` rebuilt structs through
reflection and skipped fields it could not set. The interpreter kept the
values, so the lanes disagreed.

It surfaced because `Stream._Key` is such a field: an array of streams became an
array of zero-key streams in the compiled lane only. The v1 `Rng` record
(`_State0`..`_State3`) had the same exposure.

Fix: one line. The clone now starts from a full struct copy and then
deep-clones the fields reflection can set. The new Language fixture fails in the
compiled lane before the change and passes in both lanes after it.

## Evidence

All runs on linux/amd64, Go 1.25.0. "Baseline" is the compiler built from
`51e53a2`.

| Check | Baseline | After M3 |
|---|---|---|
| `oct test Libraries/Random`, interpreted / compiled / auto | 22 passed | 53 passed in each mode |
| `oct test Libraries/RandomUsage`, interpreted / compiled / auto | — | 6 passed in each mode |
| Preconditions test (`-tags=integration`), both lanes | — | pass |
| `go test ./...` | 69 ok, 1 fail | same packages, same failing tests |
| `go test -tags=integration ./...` | 68 ok, 2 fail | same packages, same failing tests |
| Per-directory sweep, compiled, every directory under `Libraries` and `Language` that holds an `.octest` | 187 directories: 1447 pass, 125 fail | 188 directories: 1487 pass, 125 fail |
| Per-directory sweep, interpreted, same directories | 1540 pass, 32 fail | 1580 pass, 32 fail |
| `oct test Experiments/FmBrownNoiseKalman`, `.../PrometheusMeasurementFilteringLab`, both lanes | see below | identical pass/fail sets |

In both sweeps no existing PASS or FAIL line changed; the only differences are
the 40 added passes and the new `Libraries/RandomUsage` directory.

**v1 unchanged.** The 94 differential probe outputs from M2 were re-run against
the baseline and M3 compilers. 93 are byte-identical. The remaining one is a Go
compiler message that v1 leaks to the user (M2 report, v1 defect 1); it differs
only in a generated-file line number, 624 to 625, because the clone helper
gained a line.

**Fault injection.** 21 deliberate faults were injected one at a time and every
one was caught by at least one test: import check removed, argument type check
removed, arity check removed, redeclaration allowed, stream type misnamed;
interpreter dropping or rewording the runtime error, misnaming the stream
value, swapping `Between` bounds, ignoring the fork label; the emitted adapter
dropping the error, swapping `Normal` parameters, shifting the child index,
rewording the error, truncating an `IntBetween` bound; the clone fix reverted;
v2 names reserved globally; a wrong parameter type and a wrong result type in
the table; and two changes inside `internal/octrandom`.

**Other backends.** `oct build --target wasm` rejects a program that imports
Random, as it did before (the WASM M0 backend does not support the Random
records). Not a regression; v2 is not available on that backend.

### Failures that exist before and after

- `go test`: `internal/sdslv/test` fails because `dxc` is not installed here;
  under `-tags=integration`, `internal/document` also fails one LaTeX-to-PDF
  test for a missing tool.
- Compiled sweep: most of the 125 failures are packages that need Octxiliary
  sidecars, which are not built in this environment (IO, Pdf, Csv, Hash), and
  UI, whose builtins the compiled lane does not support. The rest are a few
  Language fixtures that fail in both compilers; the interpreted sweep has 32
  such failures.
- Experiments: `PrometheusMeasurementFilteringLab` M4 panics in the compiled
  lane (7 tests) and `FmBrownNoiseKalman` M1 fails in both lanes, as reported
  in M2.
- `SymbolicRegression.EndToEndFitAndRolloutRecoversKnownSystem` takes about
  23 s interpreted against a 30 s cycle limit. It passes when run alone with
  either compiler and timed out in two sweeps that ran while the machine was
  loaded.

### Interpreter speed

M3 adds one check to the interpreter's call path, so the cost was measured on
that same call-heavy test. Mean user CPU time over five alternating runs was
29.0 s for the baseline and 29.0 s after M3, with single runs spread across
about 28.6 to 29.7 s for both. No difference is measurable.

## Correction to the M2 evidence

The M2 report cited `oct test Language --all-packages` (392 passed, 4 failed)
as corpus evidence. A run at a root directory executes the `.octfail` fixtures
below it but only the facts of the root package, so that figure covered
`.octfail` contracts only. To close the gap, the per-directory sweeps above
were also run with the pre-M2 compiler (`3d1cdfff`). M2 changed nothing:
compiled 1447 pass / 125 fail in both; interpreted 1540 / 32 against 1539 / 33,
the one difference being the load-sensitive `SymbolicRegression` test above.

## Findings

1. **Open issue for M4** (decision needed). The ladder says v1 stays for
   callers during M4. That cannot hold: Oct has no overloading, and six
   section 3.4 names are already v1 functions in package Random with different
   signatures: `Exponential`, `Spike`, `FlipCoin`, `FlipCoins`, `RollDie`,
   `RollDice`. Proposed: M4 replaces the v1 library layer and migrates its
   callers in the same milestone; M6 keeps the removal of the v1 natives, the
   docs and the regeneration of recorded outputs.
2. **Interpreter record identity** (pre-existing, not fixed). The interpreter
   names a record value by the spelling used where it was built, and equality
   compares that name. A record returned by a library function therefore does
   not equal a literal of the same type written by the caller: it fails
   interpreted and passes compiled. Random v2 avoids it by naming builtin-made
   streams the way a literal in the calling package would be named. Recorded in
   `FEEDBACK.md`.
3. **Test-corpus gaps** recorded in `FEEDBACK.md`: an `.octest` cannot assert a
   non-recoverable runtime failure, and an `.octfail` cannot import a
   repository library.
4. **Documentation inconsistencies**, surfaced and not silently resolved:
   `Language/reference` has no page for Random or for compiler-owned library
   builtins; `Libraries/Random/tests/README.md` says production Random files
   must not use `Assert`, while they do and the reference allows `Assert.True`.
5. `Stream._Key` is a Go `int` in generated programs. That matches the existing
   compiled representation of Oct `Int` and inherits the deferred item in
   `FEEDBACK.md` about `Int` width on 32-bit targets.

## Not run

- Windows and arm64. Everything above ran on linux/amd64.
- The `toolchain`-tagged Go lane and the slow wrapper lanes; no wrapper or
  Octxiliary code was touched.
