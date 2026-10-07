# Json v2 — M4: writing, both lanes

Date: 2026-10-07
Ladder: `internal/json/JSON_V2_LADDER.md`
Base commit: `989f7a1` (M3)

## Verdict

**SUCCESS.** `Json.Save(path, value)`, `Json.Text(value)` and
`Artifact.WriteJson(path, value)` write a typed value as JSON. The
interpreted and the compiled lane write equal bytes, and
`Json.Parse<T>(Json.Text(value))` is `value` in both.

Three things here are not in the ladder's M4, and one closes a gap M3 left:

- **A language rule changed.** `F()?` may now stand as a statement
  ("`Json.Save(path, value)?`").
- **A writer has a hidden type argument** that the typechecker fills in
  ("How a writer knows its type").
- **A String cannot be saved with `Json.Save` until M5** ("Beside the first
  library").
- **M3 let `Json.Load` run during artifact evaluation.** It is refused now,
  as the first library's is ("Artifact evaluation").

## What M4 adds

| Where | What |
|---|---|
| `internal/builtin/json.go` | The table gains the three writers and says, for each builtin, what it does and whether the first library has a function of its name. |
| `internal/octjson/write.go` | `TextAs` and `SaveAs`; `Stops`, which says whether a failure stops the program; file errors in the package's own words. `Encode` is M2's. |
| `internal/parse`, `internal/ast` | The type slot of a writer call, and `ast.CallType`, the one way a lane asks what type a Json call is made at. |
| `internal/typecheck/json.go` | The check of a writer: its arguments, a value type with a JSON form, and the type written into the slot. |
| `internal/interpret/json.go` | The writers in the interpreter, the conversion of a value to Octagon data, and the artifact-phase rules. |
| `internal/build/emit_go_json.go` | The writers in the compiled lane, and the same conversion over the generated program's values. |
| `internal/jsontype` | One row of a record table is described as a record. |

Neither lane holds a rule about JSON: each turns its value into Octagon data
by the schema and hands it to `octjson`.

## `Json.Save(path, value)?`

D14 made `Json.Save` return `Void ! Error`. The language refused the call
that follows from that: `Save(path, value)?` as a statement was a
"standalone expression", while `Save(path, value)!` was accepted, and
`let _ = Save(path, value)?` is refused because a `Void` result is not a
value. A `Void ! Error` call could be propagated only through a `match`.

The typechecker now permits a propagated call as a statement, as it permits
an unwrapped one. Both lanes already ran it; the change is one case in one
function. It is a language change, so it has its own commit (`2d1a7ef`), a
contract under `Language/Errors/Fallible`, and a line in `06-errors.md`.

Still open, in `FEEDBACK.md`: a `Void ! Error` function cannot
`return error("...")`; it can only fail by propagating.

## How a writer knows its type

A reader is told its type, `Json.Load<Ticket>(path)`. A writer takes the type
of its value, and the lanes cannot work that out: the interpreter has no
static types, and an empty array or `Option.None` does not say what it is.
The layout of an array also follows its element type, not its values.

So the typechecker tells them, the way M1 does for `Option.None`. The parser
gives a writer call one empty type argument; the typechecker writes the type
of the value into it; and both lanes read the type of any Json call from its
type argument (`ast.CallType`). A written type argument on a writer,
`Json.Text<Float>(1)`, is refused: the ladder's surface has none.

## Beside the first library

`Json.Save(path, text)` and `Artifact.WriteJson(path, text)` exist in the
first library, taking a String of JSON text. Until M5 removes it, a call
whose value is a String is still the first library's, and any other value is
the new builtin's. The typechecker decides, in one place, and a call of the
first library's form simply carries no type.

The cost is that a String cannot be saved as a JSON string with `Json.Save`
until M5. `Json.Text` has no first-library form and writes a String as one.

## What cannot be written

- **A value JSON cannot hold** (a NaN, an infinity) stops the program, with
  the place of the value, before any file is opened. It is not an `Error`
  (D14), so `Json.Text` is not fallible, and the `err` arm of a `match` on
  `Json.Save` does not run for it.

  ```
  runtime error: Json.Save: out/reading.json: $.Levels[0]: an infinity has no JSON form
  ```

- **A file that cannot be written** is an `Error`, in `octjson`'s words:
  "the directory does not exist", "this is a directory, not a file", "the
  file cannot be written: permission denied", "the file cannot be written".
  `Json.Save` replaces the file and makes no directories.

## Artifact evaluation

`Artifact.WriteJson(path, value)` publishes the text through the Artifact
capability, as `Artifact.WriteText` publishes its text. It exists in the
interpreter only, like every `Artifact.*` builtin; the compiled lane refuses
to build an ordinary program that reaches it.

Artifact evaluation and capability discovery refuse `Json.Load` and
`Json.Save`, which touch a file the program names, with the texts they give
the first library's functions. M3 had added the discovery refusal for
`Json.Load` and not the artifact one, so a typed load could read any file
during artifact evaluation. `Json.Parse` and `Json.Text` touch no file and
are allowed.

## Found in the compiled Octagon materialiser

A value of a refined concept over a dimensioned base
(`concept Depth = Float<m>`) was refused by the compiled materialiser, which
read the declared dimension out of the concept's name. The round trip of
such a field found it. Fixed in its own commit (`1e569bb`) with an Octagon
contract in both lanes; the interpreter already loaded it. This is the
fourth difference of this kind; the first three are in the M3 report.

## Contracts

`Language/Builtins/Json`, every directory in both lanes:

| | Holds |
|---|---|
| `valid/json_writing.octest` (14 facts) | A byte golden for every rule of 3.6: scalars, the forms of a Float, String escapes, dimensions, options, the layout of arrays by element type, records, tables and one row, vectors and matrices. The round trip of a record of tables, records, vectors, matrices and options. `Json.Save`: reads back, replaces, and is an Error for a file that cannot be written. |
| `invalid` (19 new `.octfail`) | 12 at compile time: arguments, a written type argument, a type with no JSON form, an option with no type, fallibility, `Artifact.WriteJson` outside the phase. 4 at run time: a NaN, an infinity that is not an Error, and the two file errors. 3 during artifact evaluation: `Json.Load`, `Json.Save`, and a NaN, which publishes nothing. |
| `artifact/` | An `[Artifact]` function that publishes a record and a table. `internal/tester/artifact_json_test.go` evaluates it and compares each published file with a hand-written golden file in `data/`, byte for byte. |
| `written/` | Where the `Json.Save` contracts write. Only its README is tracked. |
| `valid/json_beside_the_first_library.octest` (2 facts) | A String given to `Json.Save` is still the first library's JSON text; a String given to `Json.Text` is a JSON string. Goes in M5. |

Also `Language/Errors/Fallible/valid/propagation_as_a_statement.octest`,
`Language/Data/Octagon/Load/valid/load_octagon_refined_dimension.octest`, and
`Language/Tooling/ConceptCapabilitiesM2/invalid/json_save_discovery.octfail`.

The golden texts were written by hand from section 3.6 and matched what both
lanes write on the first run.

## Verification

Run from a clean checkout of the committed tree.

- **Go tests:** `octjson`, `jsontype`, `dimension`, `builtin`, `parse`,
  `typecheck`, `interpret`, `build`, `project`, `tester`, `ocfmt`: all pass.
  `octjson` and `jsontype` are at 100% of statements.
- **Contracts, both lanes:** every directory above passes interpreted and
  compiled with no fallback, through `oct test` and through
  `TestLanguageCorpusRunsInBothLanes` for the Json, Octagon, Fallible and
  Artifacts trees and for every `.octfail` under `Language`.
- **Every directory that uses JSON, old or new, both lanes, against the M1
  baseline:** 40 directories, among them every experiment and library that
  calls the first library, whose calls now pass through the parser's slot
  and the typechecker's decision. No test changed status.
- **Outside the repository:** `oct run` and `oct build` of a program that
  saves and loads; the built program runs on its own.

Fault injection:

| Code | Changes | Caught | Left |
|---|---|---|---|
| `octjson/write.go` (mechanical) | 6 | 6 | |
| `jsontype` (mechanical) | 22 | 22 | |
| `builtin/json.go` (mechanical) | 8 | 6 | 2 did not build |
| Parser, typechecker, interpreter, compiled lane, materialiser (42 chosen by hand, run against the contracts in the lane each lives in) | 42 | 39 | 3 not observable |

The three left change nothing a program or the toolchain can see: the
dimension carried on a number being written, which the encoder does not
read; the name of the result type the compiled lane gives `Json.Save`, whose
value the typechecker lets no program use; and the record of an artifact
write, which nothing in the repository reads.

One change survived the first pass and should not have: making every String
given to `Json.Save` the new builtin's. Nothing checked the rule that keeps
the first library working beside the new builtins. Two contracts were added
for it (`valid/json_beside_the_first_library.octest`, and a byte check of
`Artifact.WriteJson` given a String in `internal/tester`); both go in M5.

## Found outside Json

Not changed; each is in `FEEDBACK.md`.

- A `Void ! Error` function cannot `return error("...")`.
- `Sqrt` and `Ln` outside their domain stop the interpreted lane and give a
  NaN or an infinity in the compiled lane.

## Next

M5: migration and removal. The 36 Oct files that use the first library move
to typed values, the first library and `cmd/octxiliary-json` are removed, and
the three coexistence rules go with them: `Json.Load(path)` with no type
argument, and a String given to `Json.Save` or `Artifact.WriteJson`.
