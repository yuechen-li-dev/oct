# Repeated elements: `value ... count` and `value ...`

Date: 2026-10-05
Base commit: `e9834a7` (main, after the pass reported in
`wrapper_single_definition_2026_10_04.md`)

## Verdict

**Success.** The approved form is implemented in both lanes with the same
results and the same runtime errors, and it has contracts under
`Language/Expressions/RepeatedElements`.

| Form | Meaning |
|---|---|
| `[value ... count]` | `count` elements, each the value. Mixes with ordinary elements and other segments: `[1 ... 2, 2 ... 3, 9]`. |
| `[value ...]` | Fills an array whose length is already fixed: a column of a `record table` literal, a replacement column in a table `with`, a row assigned with `rows[i] = [...]`. |
| `vector[value ... count]`, `matrix[[value ... count] ...]` | The same element form in a vector literal and in a matrix row. |
| `matrix[[row] ... count]` | The row, `count` times. |

`...` is a new token with no other use. It does not spread a collection and
it is not a range, and the parser says so wherever it turns up out of place.

## What was approved, and what I decided on the way

The proposal you approved fixed seven points. They are implemented as
written:

- the count is an `Int` evaluated at run time, a negative count is an error
  (at compile time when it is a constant), and zero gives a typed empty array;
- `value ...` is allowed in the three places above and is a compile error that
  says to write a count anywhere else;
- the count is evaluated once, then the value once for each element;
- segments and a trailing fill combine, as in `[1 ... 2, 2 ... 3, 0 ...]`;
- `...` is valid only after an element of a bracket literal;
- vector and matrix literals take it;
- `oct fmt` always writes the spaces around it.

Six things the proposal did not settle. Each is a decision I made and you may
want to reverse.

| Question | Decision | Reason |
|---|---|---|
| A matrix row after a repeated row | The `[` after a count starts the next row: `matrix[[1.0, 0.0] ... n [5.0, 6.0]]`. A count that indexes something is parenthesized: `... (counts[i])`. | Matrix rows already follow one another with nothing between them, and no comma is accepted there. The other reading turns a row on the next line into an index of the count. Both errors a writer can meet say what to write. |
| A count of zero in a vector or matrix | An empty vector, or a matrix with no rows or with rows of no elements. `vector[]` is still rejected. | `Vector.tabulate(0, f)` and `Matrix.zeros<T>(0, n)` already give these values in both lanes. An error only when `n` happens to be zero would be a trap. This lifts an existing restriction for the counted form. |
| A row of a board field | `board.Grid[i] = [value ...]` works as a local `rows[i]` does. | It is the same statement to a reader. |
| Rows of deeper arrays | A compile error that says to write a count. | Neither lane implements whole-row assignment below depth two. |
| The order of a table literal's columns | The columns that state their length are evaluated first, in the order written; the filled columns follow, in the order written. | A filled column cannot be built before the row count is known. The order matters only to a program whose columns have effects. |
| `.octagon` data | `...` is rejected: a data file writes every element. | The format is shared byte for byte with Concept, and nothing there asked for it. |

`matrix[[value ... cols] ... rows]` overlaps `Matrix.fill(rows, cols, value)`.
They differ in one thing, which the reference states: the literal evaluates
the value once for each element.

## Defects found on the way

Both are in `main` and are fixed here.

| Defect | Lane | Effect | Contract |
|---|---|---|---|
| `?` in an element of an array, vector or matrix literal | interpreted | The program stopped with "runtime invariant violation: unhandled error reached array literal element". The compiled lane returned the error. | `Language/Errors/Fallible/valid/propagate_from_a_literal_element.octest` |
| A builtin that appears in a flow only in a board index assignment, a `yield` or an expression statement | compiled | The runtime helper the builtin needs was left out and the generated program did not build. `board.Rows[0] = Array.CrossSection(board.Values, 0..2)` is enough. | `Language/ControlFlow/OctomataBoardIndexedAssignment/valid/builtin_only_in_a_statement.octest` |

The first was in the function this change rewrites. The second surfaced
because a filled board row is such a statement.

## Inconsistencies surfaced and not resolved

All four are in `FEEDBACK.md` with a reproduction. None was introduced here.

1. **An `Int` where a `Float` is declared.** `let x: Float = 1` followed by
   `x / 2` is `0` interpreted and `0.5` compiled. The typechecker accepts the
   `Int`, the compiled lane converts it, the interpreter keeps it, and
   `Language/reference/language/03-expressions.md` says "Implicit conversion is
   not allowed". The same holds for an `Int` variable, an argument, and an
   `Int` array literal used as `Float[]`. This is the most serious of the four:
   the two lanes compute different numbers from an accepted program. It needs
   your decision on which of the three is the language. No contract in this
   pass depends on it.
2. **Whole-row assignment to a board field is unchecked in the compiled
   lane.** `board.Grid[1] = [9.5]` on a row of two fails interpreted with
   `row length mismatch` and succeeds compiled, leaving a row of one. The
   reference requires the lengths to match.
3. **A compiled runtime error that carries a code prints a Go stack trace.**
   `runtime error [OCT-RTBL003]: ...` arrives as a `panic` with goroutine
   frames; `runtime error: ...` is one line.
4. **The compiled test runner's file names depend on the checkout path.** A
   fixture of mine with a long name failed in a deep worktree with "file name
   too long" and was renamed. On Windows the temporary directory counts
   toward the limit as well.

## Evidence

Machine: linux/amd64, 2 cores. "Before" is `e9834a7`. "After" is `eacbd2a`,
the last commit before this report.

| Check | Before | After |
|---|---|---|
| Whole sweep, interpreted, no sidecar | 2497 pass, 43 fail, 3 skip | 2537 pass, 43 fail, 3 skip |
| Whole sweep, compiled, no sidecar | 2315 pass, 227 fail, 1 skip | 2355 pass, 227 fail, 1 skip |
| Whole sweep, compiled, every sidecar present | 2374 pass, 168 fail, 1 skip | 2414 pass, 168 fail, 1 skip |
| The 12 wrapper libraries, each from its own directory, sidecars present | interpreted 89 pass, 5 fail; compiled 71 pass, 23 fail | the same, test for test |
| `go test ./...` | 70 ok, 2 fail | 70 ok, 2 fail |
| `go test -tags=integration ./...` | 70 ok, 2 fail | 70 ok, 2 fail |
| `go test -tags=toolchain ./...` | 71 ok, 1 fail | 71 ok, 1 fail |
| Slow wrapper lane | 65 pass | 65 pass |

- **No existing test changed status** in any of the three sweeps. Each gains
  the same 40 tests, all new and all passing: 35 for repeated elements and 5
  for the two defects.
- `TestLanguageCorpusRunsInBothLanes`, in the integration lane, runs the new
  directory in both lanes and every `.octfail`; under its `auto` mode a runtime
  `.octfail` must fail with the expected text in each lane.
- The Go failures are the ones from before. `internal/sdslv/test` needs `dxc`
  and `internal/document` needs LaTeX. `cmd/oct-mcp` timed out in the default
  lane, before and after, with the rest of the suite running beside it; it
  passes alone and passed in the integration and toolchain lanes.
- The formatter's corpus test (integration lane) formats every Oct source in
  the repository in both modes and passed with the new token and the new
  fixtures.
- The Verilog profile and the WebAssembly target needed nothing. Neither has
  dynamic arrays, and each refuses a repeated element with the diagnostic it
  already had for an array or a vector. This was checked by hand, not by a
  fixture.

### Contracts

| | Count |
|---|---|
| Facts under `RepeatedElements/valid`, each run in both lanes | 35 |
| `.octfail` under `RepeatedElements/invalid`: rejected at compile time | 31 |
| `.octfail` under `RepeatedElements/invalid`: `Main` must stop, in both lanes | 12 |
| Facts for the two defects | 5 |
| Formatter golden case, both modes | 1 |
| `.octagon` invalid fixture | 1 |

The four runtime messages are the same text in both lanes; the `.octfail`
files state each in full.

One contract has an unusual shape.
`invalid/program_reaches_functions_called_only_from_a_repeated_element.octfail`
is a program that must run to its last statement, and it is an `.octfail`
because that is the only form that runs a `Main` in both lanes. Its last
statement is the assertion the file expects.

### Fault injection

74 faults, one at a time, in a separate worktree: 13 in the lexer and parser,
15 in the typechecker, 14 in the interpreter, 11 in the compiled lowering, 7 in
the generated runtime, 5 in flows, 7 in the passes that walk expressions, and 2
in the formatter.

| | Count |
|---|---|
| Caught by the contracts as first written | 56 |
| Did not build as written; rewritten and caught | 5 |
| Not caught because my runner ran the runtime `.octfail` files in one lane; caught when run in both | 7 |
| Not caught; a contract was added and now catches it | 2 |
| Not caught; the code was redundant and is removed | 1 |
| Not caught, with no observable effect | 3 |

- **The two that needed a contract.** The compile-time contract for a filled
  column longer than its table had been overwritten by the runtime contract of
  the same file name; it is restored under its own name. And nothing showed
  that a row fill checks its index before its elements; with elements before
  the `...`, the two orders give different errors.
- **The redundant code** was a condition in the formatter that excluded the
  first row of a matrix and a row after `]`. Earlier rules already decide
  both. Another fault was written for the code that remains, and is caught.
- **The three with no effect** remove a repeated value, a count, or a matrix
  row count from the walk that seeds the compiled lane's set of reachable
  functions. The lowered code is scanned for calls again afterwards
  (`enqueueLoweredCalls`), which finds them. The walk is kept in step with the
  others; nothing depends on it today.

## Not done

- Decisions 1 to 3 under "Inconsistencies" above.
- `value ...` for anything but tables and rows. Oct has no array type with a
  length in it; if it gains one, the declaration can fix the length. This is
  in `FEEDBACK.md` as deferred.
- Row fill below depth two.
- `...` in `.octagon` data.
- `Experiments/` was not touched. It is the next task.
