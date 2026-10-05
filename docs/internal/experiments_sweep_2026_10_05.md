# Experiments sweep

Date: 2026-10-05
Base commit: `774056f` (the repeated-elements pass, reported in
`repeated_elements_2026_10_05.md`)

## Verdict

**Meaningful progression.** Every experiment directory loads. In the
interpreted lane what still fails is thirteen tests of two kinds, neither of
them rot. The compiled lane is not green and this pass does not make it so:
164 of its 167 failures have one cause, and that cause is a decision, not a
repair.

| `Experiments/` | Before | After |
|---|---|---|
| Interpreted | 790 pass, 28 fail, 28 directories not loading | 948 pass, 13 fail, 0 not loading |
| Compiled, every sidecar present | 709 pass, 109 fail, 28 directories not loading | 794 pass, 167 fail, 0 not loading |

Three more directories hold only `[Benchmark]` and `[Artifact]` functions.
`oct test` reports them as having no tests, before and after; they are not
counted above.

The compiled failures rose because 14 directories that used to stop at a type
error now load and reach the missing feature below.

## What was wrong

| Cause | Directories | Repair |
|---|---|---|
| `UIPlaceAbsolute` and `UIPlaceAnchored` gained a z-order argument | 14: `ControlPanel/M1`–`M2`, `SignalLab/M0`–`M3`, `Storefront/M0`–`M6a` | Pass `0`, as `Libraries/UI` does for an unordered box. |
| `Clamp01` became a builtin | 13: `ContinuumComputabilityBoundary/M13`–`M25`, `PrometheusPredictiveLeaseAheadLab/M1` | Remove the local copies. All three variants were the builtin's rule. |
| Emitting functions marked `[Fact]` | 2: `PrometheusSgemmAlgorithmLab/M42`, `M43` (12 functions) | `[Artifact]`, as the other 172 are. `oct artifact` reproduces the recorded outputs unchanged. |
| A test wrote artifacts | 3: `FmBrownNoiseKalman/M1`, `OctErgonomicsLab/M0`, `M1` | See below. |
| A `[Fact]` asserted nothing on the path it expected | 1: `TrialBatchSimulation/M0` | Assert that the batch failed. |
| The directory could only be run as one file from `cmd/oct` | 1: `JsonIntentRecoveryLab/M0` | Corpus paths are relative to the repository root, the recovered sources declare the package of their directory, and the Go test runs the directory from the root. |
| State kept in locals across `suspend` | 1: `RfAdaptiveLinkControllerProbe/M0` | See below. |

### Tests that wrote artifacts

`Artifact.Write*` runs only during `oct artifact`. Three tests predate that.

- **`FmBrownNoiseKalman/M1`** generated its report and then checked that it
  was compact. Its four outputs are now recorded, as the later milestones
  record theirs, and the test reads them.
- **`OctErgonomicsLab/M0`** wrote CSV files to read back. They are test
  inputs, so it writes them with `Csv.Write`.
- **`OctErgonomicsLab/M1`** generated its artifacts and checked that four
  files existed. That is what `oct artifact` reports, so the test is removed.
  The `[Artifact]` entry point stays and evaluates.

### The RF controller

`RunAdaptiveLinkController` held its index, its current mode and its result in
`var`s, in a `while` loop with a `suspend` in it. Two things made that wrong,
and its test passed in the interpreted lane anyway.

- A `suspend` inside a loop resumes after the loop. The controller handled one
  sample and returned.
- State locals do not survive a `suspend`. The interpreter kept them and the
  compiled lane cleared them, so one returned two modes and the other none.

The test asked only for a non-empty result, which two modes satisfy. The flows
now keep their state on the board, one sample is handled on each `Step`, and
the test requires one mode for each sample. The comparison with the naive
controller holds on the whole trace, in both lanes.

## What the sweep found in the toolchain

### Fixed

**A builtin called through its namespace did not compile outside its library.**
`IO.ReadText`, `IO.WriteLines`, `Csv.Write`, `Json.Load` and the other aliases
were refused with "compiled mode does not yet support builtin FileReadText",
though `FileReadText` compiled under its own name. The alias now resolves as
the name does. Contract:
`Language/Testing/CompiledOctxiliary/valid/namespaced_io_aliases.octest`,
which fails compiled on the base commit.

**`Clamp01` was not in the reference.** It is a builtin with a fixture, and
the thirteen probes above broke on it with no document to say why.
`09-builtins.md` now has it, with two `.octfail` contracts for the rules it
states: the argument is a `Float`, and the name cannot be declared.

### Not fixed: decisions for you

All are in `FEEDBACK.md` with reproductions.

1. **UI in the compiled lane.** 164 compiled failures in 22 directories of
   `ControlPanel`, `SignalLab` and `Storefront` are "compiled mode does not yet
   support builtin UIButton". The rule in `31-octest.md` is that a test for a
   feature a lane lacks fails there until the feature exists, so I did not
   restrict them. If UI is meant to stay interpreted, that is a specified
   difference and `[Interpreted]` is the right mark; the reference's own
   example of the attribute is a UI test.
2. **A state local read after `suspend` is accepted and the lanes disagree.**
   `var x = 7  suspend  return x` is `7` interpreted and `0` compiled. The
   reference says locals do not cross `suspend`; the typechecker enforces that
   for `yield` in one block and for nothing else. After the repair above, no
   source in the repository reads a local after a `suspend`, so enforcing it
   would break nothing here.
3. **A nested `suspend` or `yield` resumes after the statement that contains
   it.** In `if ready { yield 1  board.A = 50 }` the assignment never runs, in
   either lane. A fixture pins this for `suspend`. The reference does not
   state it, and for `yield` says the continuation is "immediately after the
   yield". Statements after a nested boundary are silently dead.
4. **`ContinuumComputabilityBoundary/M16` asserts a verdict its probe does not
   reach.** Both lanes compute the same numbers and they say the boundary alone
   suffices. The test is left failing. It had not run for at least four
   months, because the directory did not load.

A fifth is friction, not a defect: a failed `Assert.Equal` prints neither
value. I wrote throwaway programs to see them several times in this pass.

## What still fails

### Interpreted: 13 tests

| Tests | Where | Why |
|---|---|---|
| 12 | `FmBrownNoiseKalman/M4b`, `M5`, `M6`; `PrometheusSgemmAlgorithmLab/M19`, `M32` | Each exceeds the default cycle time of 30 s on this 2-core machine. All pass compiled. Not changed: whether they fit the budget depends on the machine. |
| 1 | `ContinuumComputabilityBoundary/M16` | Item 4 above. |

### Compiled: 167 tests

| Tests | Where | Why |
|---|---|---|
| 164 | 22 UI directories | Item 1 above. |
| 2 | `JsonIntentRecoveryLab/M0` | `JsonLoadStructured` has no compiled implementation. It is one of the eight builtins listed in `wrapper_single_definition_2026_10_04.md`. |
| 1 | `ContinuumComputabilityBoundary/M16` | Item 4 above. |

## Evidence

Machine: linux/amd64, 2 cores. "Before" is `774056f`. "After" is `98ceb8f`,
the last commit that changes code or experiments.

| Check | Before | After |
|---|---|---|
| Whole sweep, interpreted, no sidecar | 2537 pass, 43 fail, 3 skip | 2697 pass, 28 fail, 3 skip |
| Whole sweep, compiled, no sidecar | 2355 pass, 227 fail, 1 skip | 2439 pass, 288 fail, 1 skip |
| Whole sweep, compiled, every sidecar present | 2414 pass, 168 fail, 1 skip | 2503 pass, 224 fail, 1 skip |
| The 12 wrapper libraries, each from its own directory, sidecars present | interpreted 89 pass, 5 fail; compiled 71 pass, 23 fail | the same, test for test |
| `go test ./...` | 70 ok, 2 fail | 70 ok, 2 fail |
| `go test -tags=integration ./...` | 70 ok, 2 fail | 70 ok, 2 fail |
| `go test -tags=toolchain ./...` | 71 ok, 1 fail | 71 ok, 1 fail |
| Slow wrapper lane | 65 pass | 65 pass |

- **No test went from passing to failing** in any sweep.
- **Outside `Experiments/`** the interpreted sweep is unchanged apart from
  the two new facts of the alias contract. In the compiled sweeps
  `Libraries/ArtifactUsage`, whose test file calls `Csv.Write` through its
  namespace, stops failing to build: with every sidecar present both of its
  tests pass, and without sidecars one does and the other needs the `csv`
  sidecar.
- **Without sidecars** the two facts of the alias contract fail compiled, as
  the other facts of their directory do: they exist to reach the `io`
  sidecar. `TestLanguageCorpusRunsInBothLanes` runs that directory with its
  sidecars and passes.
- The compiled failure count rises by the UI tests of the 14 directories that
  now load, as explained at the top.
- The Go failures are the ones from before: `internal/sdslv/test` needs
  `dxc`, `internal/document` needs LaTeX, and `cmd/oct-mcp` times out in the
  default lane beside the rest of the suite and passes in the other lanes.
- The alias fix was checked in both directions: its contract fails compiled
  with a binary built from the base commit and passes with this one.

## Not done

- The four decisions above.
- The 12 tests over their cycle time.
- `Experiments/` sources were changed only where they failed. They were not
  otherwise brought up to the current reference, and `oct fmt` was not run
  over them.
