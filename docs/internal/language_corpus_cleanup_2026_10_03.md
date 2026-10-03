# Language corpus cleanup and FEEDBACK pass

Date: 2026-10-03
Base commit: `9bf39d9`

## Verdict

**MEANINGFUL PROGRESSION.**

- Every directory under `Language/` now passes in both execution lanes, or is
  on a short list with a stated reason. A test keeps it that way.
- Eleven `FEEDBACK.md` entries are resolved. Seven are open and three are
  deferred; each says why.
- `Experiments/` was not touched, by instruction. It is where most of the
  remaining failures are.

It is not SUCCESS because three compiled-lane facts under `Language/` still
fail for a feature the compiled backend does not have (utility `when` with
payload candidates), and because two lane differences are documented, not
removed.

## What was wrong

Nothing ran the `Language` corpus as a whole. CI runs six of its files for
lane parity, and individual Go tests name a few directories. The rest could
stop working unnoticed.

At the base commit, of 127 directories with `.octest` files:

| State | Directories |
|---|---|
| Could not be loaded at all | 10 |
| Package set members that only run from their parent | 3 |
| Artifact fixtures and expected failures that a Go test drives | 9 |
| Loaded, with failing facts in at least one lane | 18 |

and four `.octfail` contracts failed.

## Compiler and tooling defects found

Each was found because a fixture passed in one lane and not the other, or did
not pass at all. Each has a contract under `Language/` now.

| Defect | Lane | Effect |
|---|---|---|
| A fallible `match` arm containing an `if` | compiled | **Wrong answer.** The arm's closing jump was written to the arm's first block and replaced the `if`'s branch. |
| `[1, 2] + [3, 4]` | compiled | Generated Go that did not build. |
| `Assert.Equal(7, F())` with fallible `F` | both | Accepted. Interpreted unwrapped silently; compiled compared the wrapper and failed. Now a compile error. |
| A `[Fact]` with no assertion | compiled | Passed. Interpreted failed it. |
| `ok(_)` and `err(_)` | compiled | Generated Go that did not build, in functions and in flow states. |
| A Go build failure satisfying an `.octfail` | tester | Two contracts passed on the Go compiler's message. See below. |
| Import roots | project | A nested `Packages/` hid the repository's `Libraries/`. |
| `Array.Where(xs)` diagnostics | typecheck | Named the internal builtin, `ArrayWhere`. |

### The two contracts that passed by accident

`length_one_array_not_scalar.octfail` and `nested_rank_broadcast.octfail`
expected `operator + not defined`. The typechecker accepts both programs:
array lengths are not part of a type. They passed because the compiled lane
emitted Go `+` on two slices, and the Go compiler's complaint contains those
words. They had been asserting a backend bug.

Three things follow, all done:

1. The compiled lane implements element-wise array arithmetic, at any nesting
   depth, with the interpreter's length-mismatch error.
2. A failure of the Go toolchain can no longer satisfy a contract. It is
   reported as "the source was accepted, and the generated program did not
   build".
3. The two fixtures are what they always were: run-time failures. That needed
   a test form that did not exist.

## New: `expect runtime error:`

An `.octfail` may begin with `expect runtime error: "text"`. The source must
compile, and running its `Main` must stop with a failure containing the text.
Under `--execution auto` both lanes must fail that way; a named lane is
checked alone. `Language/reference/tooling/31-octest.md` has the rules.

An `.octfail` may now also `import` a library of its repository, so a contract
for misusing a library from another package can be written.

## New: the corpus test

`go test -tags=integration ./cmd/oct -run TestLanguageCorpusRunsInBothLanes`

It runs every `Language` directory with `.octest` files in the interpreted
and compiled lanes, evaluates artifact-only directories with `oct artifact`,
and checks every `.octfail`. It takes about a minute and needs no sidecars.

Directories it does not run are listed in the test with the reason:

| Kind | Count | Why |
|---|---|---|
| Package set members | 3 | Run from the set's root, which the test does. |
| Artifact fixtures | 4 | `internal/tester/artifact_phase_test.go` runs them file by file, some with native grants. |
| Expected failures | 5 | A Go test asserts the failure. |
| Wrapper fixtures | 3 | Need built sidecars and are lane-specific; the toolchain lane runs them. |
| Known compiled gap | 1 | Utility `when` with payload candidates. The test requires the stated error, so the entry must go when the gap closes. |

The list is checked. An entry whose directory is gone, or whose failure no
longer happens, fails the test.

## Decisions

Made with you:

1. **Surface-only facts.** Fourteen Continuum and DifferentialOperators facts
   build representational field forms. Each ends with
   `Assert.True(true, "the field-form surface is accepted and constructs")`.
2. **Fallible `match` is a statement.** The reference is corrected; the
   expression form is not implemented.
3. **Imports.** Left to me. A nested `Packages/` now adds packages and does
   not end the search: the walk continues to the nearest `Libraries/`.
   Nothing that resolved before resolves differently. The
   `Language/Packages/String` stub is gone.

Made alone. Reverse any of them:

4. **`OctomataResumeM57` was corrected to the implementation.** It asserted an
   extra step between a `resume` and the resumed state's `suspend`. Both lanes
   agree that a turn runs through `goto` and `resume` to the next `suspend`,
   and the reference now says `resume` behaves as `goto` does. If the fixture
   was right and both lanes are wrong, this is the change to undo.
5. **Two facts were retired, one came back.** `Functions/Calls` tested
   namespaced IO, Csv and Json calls, which need sidecars and write files;
   those stay under `Libraries/`. The Markdown fact is restored, with the
   output the helpers actually produce. Its old expectation (three lines for a
   one-line callout) was wrong.
6. **`CompiledSelectedReachable/reachable` changed its subject.** It was meant
   to fail by reaching `Markdown.Report`, which the compiled lane now
   supports. It reaches `UIMount` instead and is an `.octfail`.
7. **The corpus test is in the `integration` lane**, so CI runs it. It adds
   about a minute. On a platform where a fixture fails, CI will say so.
8. **`Libraries/Markdown` got a `manifest.oct`** with `Authors: ["Claude"]`,
   because I wrote the file and do not know who wrote the library. Correct it.
9. **`StaticAssert.*` needed no expansion.** It already typechecks in ordinary
   source. The assertion contracts use it, because `Assert.Equal` is a
   test-file form and an `.octfail` is checked as ordinary source.

## Not done

- **`Array.Sum`.** Started and stopped. A global `Sum` collides with
  functions of that name in the repository. A namespaced one must return a
  typed zero for an empty array, and the interpreter does not have the static
  element type at a call.
- **Record identity across packages.** The suggested fix is unsafe: it would
  rename a caller's own record that passes through a library. It needs one
  canonical type name in the interpreter.
- **Sibling test files.** The compiled lane does not see declarations in a
  sibling `.octest`; the interpreted lane does. Documented, not unified.
- **Utility `when` in the compiled lane.** Every candidate value is evaluated
  before one is selected.
- **Experiments.** Not touched.

## Evidence

linux/amd64, Go 1.25.0, no wrapper sidecars unless stated.

| Check | Base (`9bf39d9`) | Now |
|---|---|---|
| `TestLanguageCorpusRunsInBothLanes` | did not exist | passes, 63 s |
| `.octfail` under `Language` | 408 pass, 4 fail | 422 pass, 0 fail |
| `.octfail` under `Libraries` | 63 pass | 65 pass |
| `Language`, interpreted, per directory | 386 pass, 22 fail | 464 pass, 4 fail |
| `Language`, compiled, per directory | 385 pass, 23 fail | 451 pass, 17 fail |
| `Language`, compiled, with sidecars built | not measured | 461 pass, 7 fail |
| Whole sweep, interpreted, 351 directories | 2394 pass, 54 fail | 2477 pass, 37 fail |
| Whole sweep, compiled | 2228 pass, 220 fail | 2285 pass, 229 fail |
| `go test ./...` | 69 ok, 1 fail | 69 ok, same 1 fail |
| `go test -tags=integration ./...` | 68 ok, 2 fail | 68 ok, same 2 fail |

What still fails under `Language` is all on the corpus test's list:

- Interpreted, 4: three wrapper tests that are compiled-only or need a
  sidecar, and one expected failure another test drives.
- Compiled without sidecars, 17: eleven wrapper tests, three expected
  failures, and the three utility `when` facts.
- Compiled with sidecars, 7: the three expected failures, the three utility
  `when` facts, and one interpreted-only wrapper test.

Tests that passed at the base and fail now, across the whole sweep:

- **Compiled, 13.** Twelve facts in `Experiments/PrometheusSgemmAlgorithmLab`
  M42 and M43 and one in `TrialBatchSimulation/M0`. Each asserts nothing. They
  already failed interpreted; the compiled lane now applies the same rule.
- **Interpreted, 1.** `SymbolicRegression.EndToEndFitAndRolloutRecoversKnownSystem`
  exceeded its 30 s limit. Run alone it does the same with the base binary
  and with this one, so it is at the limit on this machine and not slower.
- `Libraries/ArtifactUsage` did not load at the base, because
  `Libraries/Markdown` had no manifest. It loads now. Its two facts pass
  interpreted and fail compiled: the compiled lane has no `CsvWrite`.

The Go lane failures are the ones in every earlier report: `internal/sdslv/test`
needs `dxc`, and `internal/document` needs LaTeX.

### Sidecars

Earlier reports in this session listed "missing wrapper sidecars" as an
environment limit. That was wrong: `go run ./tools/build_sidecars` builds
them here in five seconds. With them, the three wrapper directories under
`Language/` pass in the lane each is written for.

### Fault injection

26 deliberate faults across the changes above, one at a time. Two survived the
first pass: the `err` arm's closing jump written to its first block, and the
repository's `Libraries/` searched before a nested `Packages/`. Tests were
added and all 26 are caught. The corpus test was checked separately with a
broken fixture and three stale list entries; it reported all four.

## Not run

- Windows and arm64. The corpus test starts the `oct` binary from the
  repository root; that path has not been exercised on Windows.
- The `toolchain`-tagged Go lane.
