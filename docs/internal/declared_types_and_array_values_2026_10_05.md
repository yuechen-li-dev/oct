# Declared types, arrays as values, and single-case theories

Date: 2026-10-05
Base commit: `de4cc37` (the experiments sweep, reported in
`experiments_sweep_2026_10_05.md`)

## Verdict

**Success** for the three things asked for, and **meaningful progression**
for the compiled lane's handling of arrays, where the pass found more than it
set out to fix.

| Asked | Result |
|---|---|
| `let x: Float = 1` declares a `Float`; the interpreter was wrong | The interpreter converts wherever the typechecker admits the value. The compiled lane converts at the places where it used to fail. 30 facts in both lanes. |
| A `[Theory]` with a `[CycleTime]` needs no `[InlineData]` | Implemented in the parser and the runner. 5 cases in both lanes. |
| Fix the experiments that fail | `ContinuumComputabilityBoundary/M16` passes; 11 slow tests are single-case theories. One test is left failing interpreted, on purpose. |

Not asked for, found on the way and fixed: in the compiled lane an array kept
by a board field, a board row, a state local, a record field, a `with`
replacement or an enum payload shared storage with the variable it came from.
Ten facts and three runtime contracts that failed compiled before now pass.

## Declared types

`let x: Float = 1` followed by `x / 2` was `0` interpreted and `0.5` compiled.
Your decision: the declaration says what `x` is, so the interpreter was wrong.

The typechecker's rule did not change (`isAssignable`): an `Int` is admitted
where a `Float` is declared, and an `Int` or a `Float` where a `Complex` is;
for collections of the same shape, `Int` elements where `Float` ones are
declared. What changed is that both lanes now produce the declared value at
every place the typechecker applies that rule.

| Place | Interpreted before | Compiled before |
|---|---|---|
| Typed `let` and `var`; arguments; results; array literals; record fields; table columns | kept the `Int` | converted |
| Assignment to a `Float` variable from a literal or a variable | kept the `Int` | converted |
| Element or cell assigned from an `Int` variable; a row from an `Int` array | kept the `Int` | did not build |
| `Int[][]` or deeper, `Vector<Int>`, `Matrix<Int>` where the `Float` one is declared | kept the `Int`s | did not build |
| `with` replacement of an array field | kept the `Int`s | did not build |
| Enum payload | kept the `Int` | Go panic at the first use |
| Flow: board field, board element or row, typed local, assignment to a local, `yield`, result | kept the `Int` | did not build |
| Flow argument from a variable | kept the `Int` | did not build |
| `Step(flow, input)` | kept the `Int` | stopped with "wrong flow turn input type" |
| `let xs: Float[] = [1, 2.5]` | runtime invariant violation | converted |

The compiled failures were loud. The interpreted ones were silent: the program
ran and computed with integers.

The interpreter has no static types, so it converts in two ways
(`internal/interpret/conform.go`): by the declared type where one is at hand,
and for an assignment whose target has no written type, by the value being
replaced. A `var` written with a type now remembers it, which is what lets
`var xs: Float[] = []` followed by `xs = [1, 1]` come out right.

**One gap I know of.** A variable with an inferred type that holds an empty
array has nothing to say what its elements are: `var xs = EmptyFloats()`
followed by `xs = [1, 2]`, where the function returned an empty `Float[]`,
keeps the `Int`s interpreted. It needs a type on the `var` or a non-empty
array. I have not found it in the repository.

## Arrays as values in the compiled lane

`var ys = xs` has copied since 2026-06-15
(`value_copy_semantics_ofix1.octest`). The compiled lane did not extend that
to the other places a value is kept.

| Kept by | Before, compiled |
|---|---|
| `board.Field = xs` | A later `board.Field[i] = v` also changed `xs`, including the caller's array behind a flow parameter. |
| `board.Other = Append(board.Trace, x)` | The two fields could share one array. |
| `board.Grid[i] = row` | Shared `row`; and a row of the wrong length was stored, where the interpreter and a local row assignment stop. |
| `let before = board.Field` in a state | Followed the board's later writes. |
| `Pack { Values: xs }`, `r with { Values: xs }`, `Carrier.Full(xs)` | A later `xs[0] = v` changed the record or the payload. Also for a `Matrix` field. |
| `Append(r.Values, x)` after `xs` grew | Could overwrite an element of `xs`. |

All are copies now. `board.Field = Append(board.Field, x)` and
`local = Append(local, x)` still append in place, as the same statement does
in a function.

Two things came with it:

- A row index out of range reports `index 5 out of bounds for array of length
  2` in both lanes. The compiled helper worded it differently, so no runtime
  contract could state it.
- The pass that decides which runtime helpers a compiled program carries did
  not read a flow's expression statements, nor the value an expression ends
  in. A program whose only conversion or copy sat there did not build.

**Cost.** A record built with an array field now copies the array, through the
same reflective copy a `let` uses. The compiled sweeps below show no test that
went from passing to failing or timing out, but nothing here measures a loop
that builds records around a large array.

## Single-case theories

```oct
[Theory]
[CycleTime(180.0s)]
fn SweepCoversTheWholeGrid() -> Void ! Error {
    let report = RunSweep()?
    Assert.Equal(27, Len(report.Cases), "grid")
}
```

A theory with a `[CycleTime]` and no parameters is one case, reported as
`Package.Function` with no row index. Everything else is as it was:

- a theory with parameters needs rows, and one with rows needs parameters;
- a theory with neither parameters nor a `[CycleTime]` is an error, and the
  message names both ways out;
- `[CycleTime]` on a `[Fact]` is still an error.

## Experiments

### `ContinuumComputabilityBoundary/M16`

The earlier report said this test "asserts a verdict its probe does not
reach". **That was wrong**, and I have corrected the entry in `FEEDBACK.md`.

The probe's transport sweep was written as `var nextOx = ox`, then writes to
`nextOx` while reading `ox`. When it was written those were one array, so each
pass ran in place. After arrays became values the same source ran each pass
from a snapshot. The tangents that meet at the centre of the circle then
cancel, the one interior cell gets no orientation, and the verdict flips.

Evidence: I built the toolchain of 2026-06-04 (`615ecf6`) and ran today's M16
sources with it. The test passes there, and the twelve numbers of the report
differ from what today's toolchain computed from the same source in nine places, one of them the verdict.

The sweep is now written in place, which is what it did and what `REPORT.md`
records. Both lanes reproduce the twelve values of the June toolchain to the
last digit.

What this says about the rest of `Experiments/`: anything written before
2026-06-15 that writes to a copy while reading the original now computes
something else, and only a strict enough test notices. M16 is the only
directory with that shape (`var nextX = x`, then writes to `nextX`). I did not
compare recorded numbers of other experiments against the current toolchain.

### Slow tests

| Tests | Interpreted, each | Whole directory compiled | Cycle time |
|---|---|---|---|
| `FmBrownNoiseKalman/M4b`, 3 | 39 s | 0.9 s | 180 s |
| `FmBrownNoiseKalman/M5`, 3 | 46 s | 1.8 s | 180 s |
| `FmBrownNoiseKalman/M6`, 2 | 68 s | 1.0 s | 300 s |
| `PrometheusSgemmAlgorithmLab/M32`, 3 | about 100 s | 3.3 s | 480 s |

Measured on 2 cores. The cycle time is about four times the measurement.

### Left failing: `PrometheusSgemmAlgorithmLab/M19`

`M19RectangularStressHoldsIndexingInvariants` did not finish in 900 s
interpreted. The whole directory takes 0.6 s compiled. I did not give it a
cycle time, because no honest number exists: it is still a `[Fact]` and fails
interpreted at 30 s.

The cause is the interpreter, not the test. An interpreter value is 592 bytes
and an element assignment copies the whole array, so filling an array by index
is quadratic: 2,000 elements 1.7 s, 4,000 elements 7.5 s, 8,000 elements 26 s.
The test's largest matrix is 193 by 129. The same cost is why the eleven tests
above take a minute where the compiled lane takes a second. This is in
`FEEDBACK.md` with the measurements; I tried the cheap change (copy the outer
slice only) and it does nothing, because for scalars it is the same copy.

## Inconsistencies surfaced and not resolved

All are in `FEEDBACK.md` with reproductions.

1. **The reference says "Implicit conversion is not allowed", and `1.5 + 1`
   is accepted.** Mixed `Int` and `Float` arithmetic and comparison give a
   `Float` result in both lanes. One contract pins it, for `/` only. I
   reworded the reference line only as far as the declared-type rule; whether
   `1.5 + 1` is the language is yours to decide.
2. **An array read out of bounds is a Go panic in the compiled lane**, with
   goroutine frames; the interpreter reports `index 5 out of bounds for array
   of length 2`. Only the row helpers agree.
3. **A state local cannot be assigned by index in a compiled flow.**
4. **In the interpreted lane a matrix assigned to a board field shares storage
   with the local it came from.** It is the one in-place write the interpreter
   has. I did not fix it, because item 3 means no contract can cover both
   lanes yet.
5. **Interpreted element assignment is quadratic** (above).

Documentation gaps closed here, both for behaviour that already had
contracts: arrays are values (`07-arrays.md`), and the declared-type rule
(`02-types.md`).

## Evidence

Machine: linux/amd64, 2 cores. "Before" is `98ceb8f`, the last commit of the
earlier report that changes code. "After" is `05fbb87`, the last commit here
that changes code.

| Check | Before | After |
|---|---|---|
| Whole sweep, interpreted, no sidecar | 2697 pass, 28 fail, 3 skip | 2757 pass, 16 fail, 3 skip |
| Whole sweep, compiled, no sidecar | 2439 pass, 288 fail, 1 skip | 2488 pass, 287 fail, 1 skip |
| Whole sweep, compiled, every sidecar present | 2503 pass, 224 fail, 1 skip | 2552 pass, 223 fail, 1 skip |
| `Experiments/`, interpreted | 948 pass, 13 fail | 960 pass, 1 fail |
| `Experiments/`, compiled, every sidecar present | 794 pass, 167 fail | 795 pass, 166 fail |
| The 12 wrapper libraries, each from its own directory | interpreted 89 pass, 5 fail; compiled 71 pass, 23 fail | the same, test for test |
| `go test ./...` | 70 ok, 2 fail | 71 ok, 1 fail |
| `go test -tags=integration ./...` | 70 ok, 2 fail | 70 ok, 2 fail |
| `go test -tags=toolchain ./...` | 71 ok, 1 fail | 71 ok, 1 fail |
| Slow wrapper lane | 65 pass | 65 pass |

- **No test went from passing to failing** in any sweep.
- The interpreted sweep gains 60 passes: 48 new tests and the 12 experiment
  tests that failed. The compiled sweeps gain 49: the same 48 and M16.
- The 48 new tests: 31 for declared types, 5 for single-case theories, 6 for
  arrays on the board, 6 for arrays in records and enums. Sixteen `.octfail`
  contracts are new as well, ten rejected at compile time and six that must
  stop or run to their end in both lanes;
  `TestLanguageCorpusRunsInBothLanes` runs them.
- The Go failures are the ones from before: `internal/sdslv/test` needs `dxc`
  and `internal/document` needs LaTeX. `cmd/oct-mcp`, which times out in the
  default lane when the machine is busy, passed this time; nothing here
  touches it.
- The compiled lane's remaining `Experiments/` failures are the 164 UI tests,
  2 for `JsonLoadStructured`, both from the earlier report.

### Fault injection

71 faults, one at a time, in a separate worktree: 35 in the interpreter's
conversion, 20 in the compiled conversion, 10 in the compiled copies and row
check, 6 in the theory change.

| | Count |
|---|---|
| Caught by the contracts as first written | 61 |
| Did not build as written; rewritten and caught | 3 |
| Not caught because my runner ran the runtime `.octfail` files in one lane; caught when run in both | 3 |
| Not caught; a contract was added and now catches it | 2 |
| Not caught; the code was redundant and is removed | 1 |
| Not caught, with no observable effect | 1 |

- **The runner mistake is the one I made in the last pass**, and I made it
  again: a runtime `.octfail` has to be run with no `--execution` flag to be
  checked in both lanes.
- **The two that needed a contract.** Nothing assigned an `Int[][]` variable
  to an inferred `Float[][]` variable, which is the one conversion that goes
  two arrays deep without a declared type. And nothing showed that a compiled
  program carries the copy helper when its only copy is a state local's.
- **The redundant code** gave an indexed assignment its type from the
  declaration of the variable. The array itself always says: an index names
  an element that exists.
- **The one with no effect** drops the check that a board self-append names
  the board. Its only other source would be a record field of the same name,
  and a record field never has spare capacity to append into.

## Not done

- The five items above.
- UI in the compiled lane, a state local read after `suspend`, and a nested
  `suspend` or `yield`, from the earlier report. They are unchanged.
- `Experiments/` sources were changed only where they failed.
