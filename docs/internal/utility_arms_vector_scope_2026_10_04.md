# Utility `when` arms, `vector` by scope, and wrapper functions

Date: 2026-10-04
Base commit: `785c83a` (the lane-attribute pass reported in
`language_corpus_lanes_2026_10_03.md`)

> Two things this report leaves open were settled afterwards, in
> `wrapper_single_definition_2026_10_04.md`: the wrapper rule was implemented
> as option A below, and `hysteresis` now measures against the committed
> arm's current score. The text below is the state at this report's commit.

## Verdict

Four changes were approved. Three are done and one is stopped.

| Change | State |
|---|---|
| One evaluation rule for every utility `when`; no policy fields on a standalone form | **Success** |
| `vector[...]` resolved by scope | **Success** |
| `when policy` commits to the arm, not to its value | **Success** |
| A wrapper function has one definition | **Honest stop.** No code changed. The rule cannot be enforced by deleting one of the two definitions; see [Wrapper functions](#wrapper-functions). |

## What changed

### Utility `when`

Every utility `when` now follows one rule: conditions in source order, a score
only for a condition that holds, then the value of the one arm selected. The
plain standalone form and `when policy` used to evaluate the value of every
case whose condition held.

`hysteresis` and `min_commit` on a standalone `when utility` are a parse error.
They did nothing there. One program used them,
`Experiments/PrometheusSgemmAlgorithmLab/M4`, and its results are unchanged
without them.

### `when policy` commits to the arm

A site used to remember the value it selected and look for an equal value at
the next evaluation. It now remembers the arm: the position of the case, or
`else`.

| Situation | By value (before) | By arm (now) |
|---|---|---|
| The committed arm's value changes between evaluations | Commitment lost; `hysteresis` and `min_commit` do not hold it | Held; the arm's current value is delivered |
| Two arms produce equal values | One commitment; moving between them does not reset the commit age | Two commitments |
| `else` was delivered and a case produces the same value | The case counts as the committed choice and can be held | `else` is never held against a case |
| Values that are not scalars | The site could not be checkpointed | Every site can be |

The interpreter, the generated Go and the Verilog profile implement the same
rule. Flow checkpoints record the arm: interpreter checkpoint version 4,
compiled payload version 2. Earlier checkpoints are refused. The Verilog
profile's site register is `UtilitySite<N>Arm`.

The 24 directories that use `when policy` give the same results as before in
both lanes. No existing program depended on the difference.

### `vector[...]` by scope

Where a parameter, a `let` or `var`, a loop variable, a `batch` item, a match
binding or a function value's capture named `vector` is in scope,
`vector[...]` indexes it. Everywhere else it is the literal. Scope is lexical:
a binding is visible from the statement after it to the end of its block, and
a function value sees its parameters and captures only. The parser tracks
this; `matrix` still needs only its one token of lookahead.

## Defects found on the way

| Defect | Lane | Introduced | State |
|---|---|---|---|
| A standalone `when utility` in a flow state did not build under the Verilog profile ("Verilog M2 FLOW expressions must lower to one acyclic MIR block") | Verilog | By the previous pass, `785c83a`, which moved utility `when` in flows to block lowering. No Verilog fixture used the form, so nothing failed. | Fixed. `Language/Profiles/VerilogM2/valid/utility_standalone` is the fixture. |
| A `when policy` inside a larger expression generated Go that did not build | compiled | Before this work | Fixed. Covered by `commitment_is_to_the_arm.octest`. |
| A `when policy` site whose values were not scalars could not be checkpointed | compiled | Before this work | Gone with the change to arms. |

One behaviour was kept and is recorded in `FEEDBACK.md` as open. `hysteresis`
compares the leading arm's score with the score recorded for the committed
arm, not with that arm's score at this evaluation. With `hysteresis: 2`, an arm
committed at 10 whose score has fallen to 1 is held against a rival at 5. Both
lanes do this, and did before.

## Wrapper functions

The approved rule was: a wrapper function has one definition, in the manifest
or in source, and a name with both is a compile error. The plan was to delete
one side of each of the 45 standard-library functions that have both.

Neither side can be deleted, because the two are separate implementations and
each lane has only one of them.

| | Source body | Manifest entry |
|---|---|---|
| Runs in | the interpreted lane | the compiled lane |
| Implemented by | a builtin inside `oct` (`internal/interpret/wrapper_*.go`, about 1,800 lines, and the workbook builtins in `interpret.go`; links `fpdf`, `gonum/plot` and `excelize`) | a first-party sidecar (`cmd/octxiliary-*`, about 2,700 lines) |
| During artifact evaluation | governed by the artifact effect rules | a native operation that needs a grant |
| Needs sidecars | no | yes |

Evidence, with every sidecar built:

| Library | Functions with both | Compiled, manifest entries ignored | Arguments of builtin and wire function |
|---|---|---|---|
| `Archive` | 3 | fails: no compiled `ZipListEntries` | same |
| `Compression` | 4 | fails: no compiled `GzipCompressBytes` | same |
| `Hash` | 3 | fails: no compiled `HashSha256Text` | same |
| `Text` | 4 | fails: no compiled `RegexIsMatch` | same |
| `Time` | 5 | fails: no compiled `TimeFormatIso8601` | same |
| `Csv` | 2 | passes | forwards to `IO` |
| `Json` | 2 | passes | forwards to `IO` |
| `IO` | 7 | 5 more tests fail: no compiled `XlsxAddSheet` | 2 same, 5 differ |
| `Image` | 6 | fails: no compiled `ImageEncodePng` | differ |
| `Pdf` | 6 | fails: no compiled `PdfDrawText` | differ |
| `Plot` | 3 | fails: no compiled `PlotRenderHistogram` | differ |

"Differ" means the builtin takes a bare `Int` handle and separate scalars
where the wire function takes a typed handle and records. `PdfDrawTextStyled`
is the clearest case: eight arguments as a builtin, five on the wire.

`Make` is the only library that follows the rule already. Its 15 functions are
named by the manifest alone and both lanes dispatch them to the sidecar.

Two smaller findings from the same reading:

- The manifests of the eleven libraries declare `GoModuleDir: "octxiliary"`.
  None of those directories exists; `oct pkg wrappers` plans a module path
  that is not there. The sidecars come from `cmd/octxiliary-*`.
- The typechecker skips a manifest entry when a source function of the same
  name exists, so it never compares the two signatures.

### Options

**A. Source is the definition for the standard libraries.** They become
ordinary Oct libraries over builtins, which is how
`Language/reference/language/17-standard-libraries.md` already describes them.
The compiled lane learns to run those builtins through the first-party
sidecars, as it does today for `CsvRead` and `FileReadText`
(`__octGenericFallible` in the generated runtime). Manifest wrapper
functions remain for native code outside the toolchain, with no source body,
dispatched to the sidecar in both lanes. Then a name with both is an error.

- The interpreted lane keeps running without sidecars, and artifacts behave as
  they do now.
- A direct call to one of these builtins starts to compile.
- Cost: a table entry for each of the 21 functions whose arguments already
  match; for the 20 that differ, either the four sidecars (`xlsx`, `image`,
  `pdf`, `plot`) accept the builtin's arguments or the compiled lane adapts
  them. The eleven manifests lose their `Wrappers`.
- Two Go implementations of each function remain, one per lane.

**B. The manifest is the definition.** The 45 source bodies go, and the
interpreted lane dispatches to the sidecars as the compiled lane does.

- One Go implementation of each function. The in-process builtins and their
  three third-party modules can leave `oct`.
- Every interpreted run that touches `IO`, `Csv`, `Json`, `Plot` and the rest
  needs sidecars. The default test lane was made free of sidecars on purpose;
  this undoes that.
- Those calls become native operations during artifact evaluation and need
  grants.

**C. Keep both and say so.** Make "interpreted runs the body, compiled runs the
sidecar" a declared form. This is the current behaviour with a name.

Recommendation: **A**, in two steps. First the seven libraries whose arguments
match or forward (`Archive`, `Compression`, `Hash`, `Text`, `Time`, `Csv`,
`Json`, and `IO.Read`/`IO.Write`). Then the four handle families, where the
sidecar protocol has to be settled. The error for a name with both lands with
the second step.

## Evidence

Machine: linux/amd64, 2 cores. Baseline: the binary and results of `785c83a`.
The "after" column is the final commit of this pass.

| Lane | Before | After |
|---|---|---|
| `Language/`, interpreted | 474 pass, 1 fail, 5 skip | 482 pass, 1 fail, 5 skip |
| `Language/`, compiled | 468 pass, 10 fail, 2 skip | 476 pass, 10 fail, 2 skip |
| Whole sweep, interpreted | 2488 pass, 37 fail, 5 skip | 2493 pass, 40 fail, 5 skip |
| Whole sweep, compiled | 2306 pass, 222 fail, 2 skip | 2314 pass, 222 fail, 2 skip |
| `go test ./...` | 71 ok, 1 fail | 71 ok, 1 fail |
| `go test -tags=integration ./...` | 70 ok, 2 fail | 70 ok, 2 fail |

- Twelve tests were added and four removed or renamed, in each lane.
- The three additional interpreted failures are the tests of
  `Experiments/FmBrownNoiseKalman/M4b`, which exceed their 30-second cycle
  time. They fail the same way with the `785c83a` binary when run alone on
  this machine now.
- No other test changed status in either lane.
- `cmd/oct-mcp` timed out once in an earlier run of the default Go lane while
  another job was running. It passes alone and passed in the final run.
- The Go failures are the ones from before: `internal/sdslv/test` needs `dxc`
  and `internal/document` needs LaTeX.
- The ten compiled `Language/` failures are wrapper tests run without
  sidecars, as before.
- Both Verilog testbenches pass in Icarus Verilog 12. The integration test
  that runs them is written for WSL and does not run on Linux, so this was done
  by hand.

### Fault injection

40 faults were written into the parser, the interpreter, the checkpoint code,
the lowering and the generated Go runtime, one at a time, in a separate
worktree.

| | Count |
|---|---|
| Caught by the tests as first written | 29 |
| Not caught; a contract was added and now catches it | 6 |
| Did not build as written; rewritten and caught | 3 |
| Not caught, left so on purpose | 2 |

The six that needed a contract: a capture's value read in the function
value's scope; an enum payload binding named `vector`; a lead equal to
`hysteresis`, in each lane; a restored checkpoint with an arm that cannot
exist; and, compiled only, a case with a score below zero losing to `else`.

The two left alone are the same fault in each lane: the recorded score is not
refreshed while the committed arm stays. That is the behaviour recorded as
open above, and a test would fix it in place before it is decided.

Ten more faults were written into the two Verilog goldens and run against
their testbenches. Seven were caught at first. The policy testbench gained
four turns and now catches two more; the last is equivalent to the original,
because the default already selects `else`.

## Not done

- The wrapper rule, as above.
- `Experiments/` was not modernized. One file changed there, to remove the
  policy fields that are now rejected.
- The `hysteresis` comparison described above was not changed.
