# Language corpus: lane attributes, expected failures, utility `when`

Date: 2026-10-03
Base commit: `e2637b9` (the merge of the cleanup pass reported in
`language_corpus_cleanup_2026_10_03.md`)

## Verdict

**SUCCESS** for the `Language/` corpus.

Every directory under `Language/` that holds tests passes in the interpreted
lane and in the compiled lane. The corpus test's list of known compiled gaps
is empty, and its list of directories it cannot run holds one entry.

`Experiments/` was not touched, by instruction, and is where the remaining
failures are.

## What changed

| Request | Result |
|---|---|
| `Libraries/Markdown` authors | `["Codex", "Claude"]` |
| `Libraries/IfErrNotEqualNil` | Rewritten as the identity template. It parses, has tests in both lanes and a negative contract. |
| `testdata/m34a` | Rewritten with `Append` and moved to `Language/ControlFlow/Loops/valid`. Its report is `collection_iteration_pressure_m34a.md`. |
| `matrix` as a value name | The parser reads a literal only for `matrix[[` and `matrix[]`. |
| `[Interpreted]` / `[Compiled]` | Added, with a required reason. |
| Expected failures as `.octfail` | Done: 23 fixtures. `.octfail` gained an artifact form and several expectation lines. |
| Sidecar cache | `internal/sidecarcache`. Built once, reused until a source changes. |
| Utility `when` in the compiled lane | Compiles, and evaluates in the order the reference gives. |

## Defects found on the way

Each has a contract now.

| Defect | Lane | Effect |
|---|---|---|
| Every utility `when` evaluated each case's value and score, and `else`, before selecting | compiled | **Wrong answer.** A case whose condition was false, or an `else` that was not needed, could fail the program or propagate an error. Three cases in a function and two in a flow state; none was covered by a fixture. |
| Enum-targeted `when utility` with payload candidates | compiled | Refused. This was the known gap. |
| A wrong manifest in an imported package | both | Ignored without a message when the program's directory required no manifest. The package lost its wrapper declarations and its stub bodies ran. |
| `Artifact.WriteText` reached from an ordinary program | compiled | Reported as "does not yet support builtin ArtifactWriteText". |
| A value named `matrix` | both | Could not be indexed. |

### Utility `when`

The reference already specified the order, and the interpreter followed it:
conditions in source order, a score and a value only for a case whose
condition holds, `else` only when none holds; the enum-targeted form evaluates
the selected value alone. The compiled lane built a list of every candidate
first.

A standalone `when utility` is now lowered to ordinary blocks, in functions
and in flow states. `when policy` keeps its runtime selection, gathers its
candidates in source order and takes `else` as a thunk.

`internal/judgment` was not used. The selection runs in the generated program,
which cannot import the compiler's packages.

One version of this change also made the plain form evaluate only the selected
value, in both lanes. That contradicted the reference, so it was reverted; the
difference between the two standalone forms is recorded in `FEEDBACK.md` as an
open question.

### Manifests

The first version of the manifest check held every existing manifest to
account. Eight experiment milestones and two `cmd/oct` tests then failed: a
milestone borrows a family manifest that names the family, and a file selected
on its own is specified to run beside a wrong manifest. The check now covers
imported packages only.

## Lane attributes

`[Interpreted("reason")]` and `[Compiled("reason")]` apply to a `[Fact]` or a
`[Theory]`. The other lane reports the test as `SKIP` and does not build it.
Under `--execution auto` a `[Compiled]` test does not fall back.

Outside the attributes' own contract, three facts use them, in the two generic
wrapper fixtures. The two lanes are
specified to differ there: a function that the manifest names and that has a
source body runs the body interpreted and the sidecar call compiled. That
split is itself an open question in `FEEDBACK.md`.

The reference says when not to use the attributes: a feature one lane is
missing is not a reason.

## The corpus test's lists

| List | Before | Now |
|---|---|---|
| Package sets run from their root | 3 | 3 |
| Directories another test owns | 12 | 1 |
| Known compiled gaps | 1 | 0 |
| Directories run with cached sidecars | 0 | 3 |

The remaining owned directory is `Language/Tooling/ConceptCapabilitiesM2/valid`.
Two of its artifacts are expected to be refused, and they need a manifest
beside them and native approvals from the host. An `.octfail` cannot state
either.

## Decisions made alone

Reverse any of them.

1. **`IfErrNotEqualNil` is a template**, so a call names its type:
   `IfErrNotEqualNil<Int>(ParsePort(raw)?)`.
2. **Multi-file failures are packages beside the fixture** (`Packages/<Name>/`)
   that the fixture imports. The four wrapper mismatch directories and the
   template provenance directory were restructured that way.
3. **The artifact form checks publication.** A fixture fails if evaluation
   fails as expected and still leaves an output.
4. **The Go tests that asserted these failures are removed.** Two host-side
   checks keep synthetic inputs under `testdata/artifact_phase`.
5. **`Artifact.Write*` outside the artifact phase** is rejected at different
   times by the two lanes. The compiled half is an `.octfail`; the interpreted
   half stayed in a Go test.
6. **The sidecar cache key covers all sidecar sources**, so a change to one
   sidecar rebuilds whichever are asked for next. The cache is in the user
   cache directory.
7. **The existing slow wrapper tests use the cache too**, in place of a
   temporary directory per test process.

## Open questions

Recorded in `FEEDBACK.md`:

- A value named `vector` cannot be indexed; `vector[i]` is a literal. Looking
  ahead cannot settle it.
- A function with a source body and a manifest entry means different things in
  the two lanes.
- The two standalone forms of `when utility` evaluate values differently, and
  the plain form accepts policy fields that do nothing.
- An out-of-bounds index has a different message in each lane.
- `Assert.Equal` does not accept arrays.
- A test run leaves four files in the working tree: three untracked outputs
  at the repository root and one tracked image rewritten.

## Evidence

linux/amd64, Go 1.25.0. Sweeps run with no `OCT_WRAPPER_PATH`.

| Check | Base (`e2637b9`) | Now |
|---|---|---|
| `TestLanguageCorpusRunsInBothLanes` | passes, 12 directories and 1 gap excepted | passes, 1 directory excepted, about 67 s |
| `Language`, interpreted | 464 pass, 4 fail | 474 pass, 1 fail, 5 skip |
| `Language`, compiled | 451 pass, 17 fail | 468 pass, 10 fail, 2 skip |
| Whole sweep, interpreted | 2477 pass, 37 fail | 2488 pass, 37 fail, 5 skip |
| Whole sweep, compiled | 2285 pass, 229 fail | 2306 pass, 222 fail, 2 skip |
| `go test ./...` | 69 ok, 1 fail | 71 ok, same 1 fail |
| `go test -tags=integration ./...` | 68 ok, 2 fail | 70 ok, same 2 fail |

- The `Language` failures in the sweeps are all wrapper tests with no sidecar
  available: 1 interpreted, 10 compiled. The corpus test runs those three
  directories with cached sidecars, and they pass.
- No test that passed at the base fails compiled.
- Interpreted, three facts of `Experiments/FmBrownNoiseKalman/M5` failed in the
  sweep, which ran while fault injection was running. Run alone, the milestone
  passes with the base binary and with this one.
- The Go lane failures are the ones in every earlier report:
  `internal/sdslv/test` needs `dxc`, and `internal/document` needs LaTeX.
- All 57 Oct files added or changed pass `oct fmt --check`.

### Fault injection

51 deliberate faults across the changes above, one at a time, in a separate
worktree. Of the 49 written first, 42 were caught on the first pass. The seven
that survived:

- Five flow faults, because the check ran the parent directory, which runs
  `.octfail` files only. With the right target four were caught.
- The fifth, a non-scalar `when policy` evaluating `else` eagerly, needed a
  fixture; a record-valued policy was added.
- Two formatter faults (later expectation lines dropped; an artifact fixture
  formatted as ordinary source); a formatter test was added.

The manifest faults were rewritten when the manifest rule was narrowed.

All 51 are caught on the final code, none by a build error.

## Not run

- Windows and arm64. The sidecar cache uses `os.UserCacheDir` and an atomic
  rename; neither was exercised on Windows.
- The `toolchain`-tagged Go lane and the slow wrapper lanes
  (`OCT_SLOW_TESTS=1`), which now take their sidecars from the cache.
