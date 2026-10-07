# Json v2 — M3: reading, both lanes

Date: 2026-10-06
Ladder: `internal/json/JSON_V2_LADDER.md`
Base commit: `c3a3d76` (M2)

## Verdict

**SUCCESS.** `Json.Load<T>(path)` and `Json.Parse<T>(text)` read JSON as the
type the program declares, in the interpreted and the compiled lane, with
equal values and equal error text. The seven documents of
`Experiments/JsonIntentRecoveryLab` load. The first Json library is untouched
and stays beside the new one until M5.

Two things in this report are not what the ladder said, and one is a
correction to M2:

- The schema of a type is built in one place, not once in each lane
  ("One schema builder").
- Three differences between the two `LoadOctagon` materialisers were fixed,
  not only reported ("What M3 found in the Octagon materialisers").
- **M2 and the first M3 commits did not pass on a fresh checkout.** See
  "Correction".

## Correction

`.gitignore` ignores `*.json`. The 318 JSONTestSuite files that
`go test ./internal/octjson` reads were therefore never committed in M2, and
neither were the 12 fixtures the M3 contracts load. Every run passed in the
working clone, where the files were on disk. On a fresh checkout of
`e2c9cd3` to `61261df` the M2 test fails with "found 0 JSONTestSuite files,
want 318".

It was found when fault injection, run in a separate worktree, failed before
any change was applied. Commit `ceaf0a0` excepts the two directories from
the ignore rule and tracks the files. The verification below was run again
from a clean checkout of the committed tree, and milestones close that way
from here on.

## What M3 adds

| Where | What |
|---|---|
| `internal/builtin/json.go` | The table of the Json builtins. The typechecker, the interpreter and the compiled lane resolve a call from it. |
| `internal/jsontype` | `Of(program, package, type)`: an Oct type as an `octjson.Schema`. |
| `internal/octjson` | `Admit`, the question a decode asks the lane about a value of a refined concept; `LoadAs` and `ParseAs`, which are `Json.Load` and `Json.Parse` up to the materialiser; file errors in the package's own words. |
| `internal/typecheck/json.go` | The check of a Json call: one type argument, one `String` argument, a type with a JSON form, a fallible result. |
| `internal/interpret/json.go` | The call in the interpreter: `octjson` reads, the Octagon materialiser builds the value. |
| `internal/build/emit_go_json.go` | The call in the compiled lane: the generated program imports `internal/octjson`, carries the schema of each type it reads as a literal, and hands the data to its own Octagon materialiser. |
| `internal/dimension` | `Parse`, the inverse of `String`. Octagon data carries a dimension as text. |

Neither lane holds a rule about JSON. Outside `internal/octjson` the only
uses of the package are `Check` (typechecker), `LoadAs` and `ParseAs` (each
lane), the kinds of `Data` (each lane's conversion to its materialiser's
input) and the kinds of `Schema` (the compiled lane's literal).

## One schema builder

The ladder had each lane build the schema of `T` from its own type
information. `internal/jsontype` builds it once, from the program's
declarations, and the typechecker and both lanes call it.

- Three builders would have had to agree on field order, on the name a
  message gives a type, on what is a table, and on the package a name is
  resolved in. Lane parity is meant to hold by construction (I1, D9), and one
  builder cannot disagree with itself.
- The typechecker could not have built it from what it knows. It sees the
  declarations of the packages a file imports and no others, and a type
  reaches further: `Depot.Site` holds `Net.Address`, and a file that imports
  only `Depot` can load a `Depot.Site`. `jsontype` follows the type through
  the whole program.

A type that names itself (a record with an array of itself) is a schema that
points back to itself. The compiled lane writes such a schema as a literal by
making every node first and filling them in after.

## Refined concepts

A value of a refined concept is its base type in the document. `octjson`
asks the lane whether the value is admitted as it reads it, so a refusal has
the value's path and position, and the text is assembled in one place:

```
Json.Load: service.json: $.http.port (line 6, column 13): refined concept Port: a port is below 65536
```

Each lane answers by running the concept's checked constructor, the same
function `Port(raw)` runs. The materialiser then admits the value again when
it builds it; that second admission cannot fail, and costs one more
evaluation of the requirements for each refined value.

## What M3 found in the Octagon materialisers

The ladder's risk table said a difference between the two materialisers
would show up here and be reported, not patched around. Three showed up.
Json needs all three to work, so each was fixed in the materialiser itself,
in its own commit (`73ffe50`) with Octagon contracts in both lanes:

| | Interpreted, before | Compiled, before | Now, both |
|---|---|---|---|
| A field declared `Vector<T>` or `Matrix<T>` | Refused the type | Loaded an array; refused a dimensioned element | Loads the array |
| A matrix with rows of two lengths | (refused the type) | Loaded it | Refused, one text |
| A refined array concept | Admitted by its concept | Not admitted; panicked on any element | Loaded as its base, admitted whole |

Still different, and in `FEEDBACK.md`: `LoadOctagon<Vector<T>>` as the type
argument itself is refused by the typechecker, and `WriteOctagon` of a vector
is refused by the interpreter and written by the compiled lane.

## Contracts

`Language/Builtins/Json`, every directory in both lanes:

| Directory | Holds |
|---|---|
| `valid` (31 facts) | Every representable type through `Json.Parse`; records, key matching, strictness, nesting, a record that holds itself, refined fields; both table shapes, tables as fields; a Json call in a flow state. |
| `packages` (4 facts) | A type of an imported package, reaching a package the file does not import; a Json call inside an imported package; a concept of another package refusing. |
| `corpus` (7 facts) | The acceptance corpus, below. |
| `invalid` (39 `.octfail`) | 20 at compile time: types with no JSON form, each naming the part; argument count and types; fallibility. 19 at run time, each stating the whole text of one refusal for both lanes. |
| `data` | The JSON documents the contracts load. |

Also `Language/Data/Octagon/Load` (3 facts, 1 `.octfail`) for the
materialiser changes, and
`Language/Tooling/ConceptCapabilitiesM2/invalid/json_load_discovery.octfail`:
capability discovery refuses `Json.Load`, which reads a file, and not
`Json.Parse`.

Not written: the redeclaration contract the ladder listed for M3.
`Libraries/Json` declares `Load` until M5, so there is nothing to refuse yet.
The ladder now lists it under M5.

### The acceptance corpus

| Document | Declared as | |
|---|---|---|
| 01 people | `record table Person` in a record | Loads; every cell asserted |
| 02 config | Nested records, snake_case keys | Loads; equals the value written in the test |
| 03 dispatch | Two keyed `record table`s | Loads; every cell asserted |
| 04 matrix | `Matrix<Float>` in a record | Loads; every element asserted |
| 05 optional | `record table` with `Option` cells | Loads; null, absent and present asserted for each column |
| 06 ui-like | Tagged array | The payload-enum declaration is a compile error (D8) |
| 07 tagged | Tagged array | The payload-enum declaration is a compile error (D8) |

06 and 07 do load when declared without the tag: every member that some
objects lack is an `Option`. The corpus test shows both. That is a reading
the program can use today; it does not say which members belong to which
tag, which is what D8 deferred.

## Verification

Run from a clean checkout of the committed tree, as the ladder's amended rule
bounds it: the code M3 adds, the Json contracts in both lanes, the Go tests of
every package touched, and fault injection on the added code.

- **Go tests:** `internal/octjson`, `jsontype`, `dimension`, `builtin`,
  `typecheck`, `interpret`, `build`, `project`: all pass. `octjson` and
  `jsontype` are at 100% of statements.
- **Contracts, both lanes:** every directory above passes interpreted and
  compiled, with no fallback to the interpreter.
- **Directories M3 could have disturbed, both lanes, against the M1
  baseline:** the other Octagon, Option and Entropy contracts,
  `Libraries/Json`, `Libraries/IO`, and every experiment that calls
  `LoadOctagon` (25 directories in all). No test changed status. The first
  library's compiled failures without a sidecar are the same ones.
- **Outside the repository:** `oct run` and `oct build` of a program that
  calls `Json.Load`, from a directory that is not in the repository; the
  built program runs on its own.
- **Profiles:** the Verilog profile and the WebAssembly target refuse a Json
  call before emission, as they refuse any fallible value.

Fault injection:

| Code | Changes | Caught | Left |
|---|---|---|---|
| `octjson`: `read.go`, `decode.go`, `errors.go` (mechanical) | 90 | 90 | |
| `jsontype` (mechanical) | 19 | 19 | |
| `builtin/json.go` (mechanical) | 10 | 10 | |
| `dimension.Parse` (mechanical) | 27 | 27 | |
| Typechecker, interpreter, compiled lane, materialisers (45 chosen by hand, run against the contracts in the lane each lives in) | 45 | 41 | 4 equivalent |

The four equivalent changes each drop something the materialiser does not
read, or change one word of a message: the `HasUnit` mark on a number, the
"inferred" mark on an option's type argument, the "has a payload" mark on a
`Some`, and the name the compiled lane gives the type of a matrix row. The
first three are kept because they make the data the shape the Octagon parser
gives.

The first pass of the hand-chosen changes led to contract lines that were
missing: a Float read digit for digit, the dimension of the elements of a
vector, a matrix and an array, a loaded vector being a vector, an array of
vectors, and capability discovery.

## Found outside Json

Not changed; each is in `FEEDBACK.md` with how to see it.

- Indexing a value of a refined array concept gives the concept's type, not
  the element's, and does not build in the compiled lane.
- The compiled lane does not build `v + v` on two vectors unless the program
  uses another linear-algebra operation.
- `0.1 + 0.2 != 0.3` is true interpreted and false compiled: Go folds the
  literals exactly.
- Two refined concepts of one name in two packages do not build in the
  compiled lane once the program loads Octagon or JSON data.
- A file cannot read the fields of a value whose type is declared in a
  package it does not import, and the message does not say so. A transparent
  alias cannot be named from another package, which the reference does not
  say.
- `oct run` prints `<invalid>` after a fallible `Void` `Main`.
- `internal/interpret/ui_runtime.go` is not gofmt-clean on `main`.

## Next

M4: `Json.Save`, `Json.Text`, and `Artifact.WriteJson` taking a value.
`octjson.Encode` exists and is tested since M2; M4 gives each lane the
conversion from its value to Octagon data and the three builtins.
