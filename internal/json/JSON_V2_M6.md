# Json v2 — M6: `oct json infer`

Date: 2026-10-07
Ladder: `internal/json/JSON_V2_LADDER.md`
Base commit: `1f1240a` (M5)

## Verdict

**SUCCESS.** `oct json infer <file.json> [--name <Name>] [--explain]` prints
the declarations a JSON document loads into, as Oct source that can be
pasted. For 26 of the 28 documents the ladder names, the output is pasted
into a contract and the document loads with it in both lanes. The other two
are the tagged arrays of the corpus, which Json does not read (D8); the
command says so with the place.

Four things are not in the ladder's M6:

- **A compiled-lane fault was fixed.** `LoadOctagon` and `Json.Load` of a
  record with a field named `name`, `token_0` or `名前` panicked compiled.
  See "Fields without a leading capital".
- **A design gap was found and is open.** A key such as `$schema` or `@type`
  matches no field name, so an object that has one cannot be loaded as a
  record at all. See "Keys no field can match".
- **There are two judgments, not one.** The ladder named record or keyed
  table. Table or tagged array has the same nature and is the second.
- **The goldens are contracts.** The ladder asked for golden output and, for
  each document that loads, a check that the declarations compile and load
  it. Both are one file per document under `Language/Tooling/JsonInfer`.

## What M6 adds

| Where | What |
|---|---|
| `internal/jsoninfer` | The inference: about 1150 lines with their comments, on `internal/octjson` and `internal/judgment` |
| `internal/cli/json.go` | The command: arguments, the file, the streams, the exit status |
| `internal/octjson` | `MemberPath` and `ElementPath`: the spelling of a place, exported so a tool names a place as a Json error does |
| `Language/Tooling/JsonInfer` | 26 contracts in 6 directories |
| `Language/reference/tooling/35-cli.md` | The command and its rules |

`internal/jsoninfer` holds no rule about what a declaration reads. Those are
`octjson`'s, and the output is checked against them.

## How it reads a document

1. **Shapes.** The document is read into shapes: what the values seen at one
   place have in common. The rows of an array and the instances of an object
   are folded into one, so a member that is `null` or absent somewhere is
   known to be optional, a place with `1` and `2.5` is a `Float`, and a place
   with a number and a string has no declaration.
2. **Two choices** go through `internal/judgment`, below.
3. **Names and text.** A declaration is named after its key, a field after
   its key in Oct's capitalisation (`read_timeout_ms` is `ReadTimeoutMs`,
   which Json matches back). A name that is taken, or is a builtin type, is
   prefixed with its parent's or numbered. The root keeps the name it was
   given.

Everything except the two choices is a rule with one answer and is written as
one: a number with no fraction is an `Int`; rows of numbers all one length
are a `Matrix<Float>`; an array of objects is a table where a table may be
declared, and an array of a record in a cell.

## The two choices

Each has bounded candidates, a stated reason for a candidate that cannot be,
named weighted considerations, and a trace that `--explain` prints.

**Record or keyed table**, for an object where a table may be declared:

| Consideration | Weight | Counts for |
|---|---|---|
| Keys read as field names (`read_timeout_ms`), or as data (`user.created`, `u-100`, `2024`) | 2 | Record, or keyed table |
| Values differ in type, or fold into one | 0.5 | Record, or keyed table |
| Few members, or many (eight and more count fully) | 1 | Record, or keyed table |

A record cannot be when a key matches no field name or two keys are one
field. A keyed table cannot be when the values do not fold into one type. A
record wins a tie.

**Table or tagged array**, for two objects or more:

| Consideration | Weight | Counts for |
|---|---|---|
| Members every object has | 2 | Table |
| Objects differ only by members left out | 2 | Table |
| Objects have members the others lack, both ways | 2 | Tagged array |
| A member named `type`, `kind`, `tag`, `variant` or `op` | 1 | Tagged array |
| The tag decides the members (half weight when no tag value repeats) | 0.5 | Tagged array |

A tagged array cannot be when every object has the same members, or no
member is a plain word in every object that decides the others. A table wins
a tie. A tagged array is a refusal.

The weights were set on the corpus and checked on the cases: `type` alone
does not make a tagged array (a table with a `type` column and one optional
member stays a table), and exclusive members alone do not either.

## The corpus

| Document | Printed | |
|---|---|---|
| 01 people | `record table People`, in a record | Loads |
| 02 config | Five nested records | Loads |
| 03 dispatch | Two keyed tables, `Key` and `Value` | Loads |
| 04 matrix | A record with `Readings: Matrix<Float>` | Loads |
| 05 optional | `record table Tickets` with three `Option<String>` | Loads |
| 06 ui-like | The page, and `$.page.sections`: a tagged array, `"kind"` | Refused |
| 07 tagged | `$.operations`: a tagged array, `"type"` | Refused |

For the 14 summaries the experiments record, the inferred records have the
fields the experiments declare by hand. The differences from a person's
declarations are the ones inference cannot make: `Tickets` is not `Ticket`,
a keyed table's columns are `Key` and `Value`, and `Priority` stays a
`String` (three values in three rows are not few).

## Fields without a leading capital

Inference names a field after its key, so I checked that any key can be a
field. In the compiled lane a record with a field that does not begin with a
capital letter could not be loaded:

```
panic: reflect: reflect.Value.Set using value obtained using unexported field
```

An Oct field is a Go field of the same name in the generated program, and
reflection sets only an exported one. Declaring, constructing, reading and
writing such a record all worked; only the materialiser `LoadOctagon` and
`Json.Load` share failed. The interpreter loaded it.

This mattered beyond inference. Json writes a field name as declared, so M5
told people to declare `token_0` for a snake_case schema, and migrated two
artifacts that way. They only write, which is why nothing failed.

The materialiser now sets such a field through its address (`81f4109`).
Contracts in both lanes: `Language/Builtins/Json/valid/json_field_names.octest`
and `Language/Data/Octagon/Load/valid/load_octagon_lower_case_fields.octest`.
Inference names fields in any script: `größe` is `Größe`, `名前` stays `名前`.

## Keys no field can match

Json matches a key to a field with `_`, `-`, `.`, spaces and case ignored. A
field name is letters and digits. So `$schema`, `@type`, `$ref` and `3d`
match no field, and because an unknown member is an error (D4), an object
with such a key cannot be loaded as a record whatever is declared:

```
Json.Parse: $ (line 1, column 1): unknown member "$schema"; Doc has Schema, Name
```

It loads only as a keyed table, which needs all its values to be one type.
JSON Schema, JSON-LD and OpenAPI documents have these keys. Inference reports
the object as having no declaration. This is a decision about the language
or the matching rule, not a fault to patch; it is in `FEEDBACK.md` and in the
closing report.

## Tests

| | |
|---|---|
| `Language/Tooling/JsonInfer` (26 facts, both lanes) | For each document: what the command prints, pasted as it is, and a fact that loads the document with it and checks values |
| `TestLadderDocuments` | Holds the command to what was pasted; holds the two refused documents to `testdata/refused`; fails if one of the 28 documents has neither |
| `TestCases` (70 cases) | One document for each rule and edge, with a golden file of the output and the `--explain` trace. Where nothing is refused, the printed text is read back and the document decoded with it by `octjson.Decode` |
| `TestGeneratedDocumentsLoadWithWhatIsInferred` | 4000 documents from a fixed seed. The inference is the same twice; every result said to load does (2772 did, 1228 had a refusal) |
| `TestFieldName`, `TestReadsAsFieldName`, `TestRootName`, `TestBuiltinTypeNamesAreNotDeclared` | The naming rules |
| `internal/cli` `TestJSONInfer` | Arguments, what goes to which stream, and when the command fails |

`internal/jsoninfer` is at 100% of statements.

The Go check decodes with `octjson`; it does not compile Oct. As one-off
evidence, each of the cases that loads was also pasted into a test of its own
and loaded by Oct itself in both lanes: all passed.

## Verification

Run from a clean checkout of the committed tree. M6 closes the ladder, so
this is the whole-tree verification the ladder asked for, against the
baseline taken when M1 closed.

- **Whole-tree sweeps**, 362 directories, each test compared with its M1
  result:

  | Lane | Pass | Fail | Went to FAIL |
  |---|---|---|---|
  | Interpreted | 2888 | 15 | 0 |
  | Compiled, no sidecars | 2634 | 271 | 0 |
  | Compiled, with sidecars | 2692 | 213 | 0 |

  97 tests were added since M1 and 20 are gone, all of them the first
  library's. Every failure failed at M1 too. None is in a Json directory.
  With sidecars, about 190 of the 213 are UI builtins the compiled lane does
  not have (`Button`, `AbsoluteBox`); the 15 interpreted ones want a sidecar
  or a fixture that a run from the repository root does not find, and one a
  cycle-time bound.
- **Go, four lanes** (default, integration, toolchain, slow wrapper): the
  failing tests are the ones that failed at M1, `internal/sdslv/test` and one
  LaTeX test in `internal/document`, which need tools this machine lacks. See
  "Correction" for the one exception found and fixed.
- **Artifacts** of the 20 directories that write JSON: as after M5. 19
  evaluate, 60 published files are identical to the recorded ones, and the
  two that differ are the two M5 left as recorded on purpose.
- **Changes made after that run began** (the layout of a comment, and the
  correction below) were verified again from a clean checkout: the
  integration lane of `cmd/oct`, which runs every `Language` directory in
  both lanes, passes; so do the tests of `jsoninfer`, `cli`, `octjson` and
  `build`.

### Correction to M5

M5 removed `Experiments/JsonIntentRecoveryLab/M0/corpus_validation.octest`
and did not remove `cmd/oct/json_intent_recovery_lab_test.go`, a Go test in
the integration lane that runs it and expects its two facts. **That test
failed from M5 until this milestone.** M5's report said the Go tests of every
package it touched passed; they did, in the default lane, and the integration
lane of `cmd/oct` was not among the bounded tests the ladder asks of a
milestone. I searched for the names of what was removed and not for the path
of the test that was removed.

The whole-tree run found it, which is what that run is for. The Go test is
removed (`e08381a`); the corpus is loaded by `Language/Builtins/Json/corpus`
and `Language/Tooling/JsonInfer/corpus`.

### Size

Not a test, a measurement, on one document of 23 MB: 200,000 rows of five
cells.

| | Time | Peak memory |
|---|---|---|
| `oct json infer` | 2.7 s | 0.6 GB |
| `Json.Load` then `Json.Save`, compiled | 3.1 s | 1.1 GB |
| `Json.Load` then `Json.Save`, interpreted | 16.1 s | 2.2 GB |

Both lanes wrote the same bytes, and what was written holds what was read.
The memory is 25 to 95 times the file: the text becomes a document tree,
then Octagon data, then the materialiser's input, then the value. That is
fine for a configuration or a summary and wrong for a large data file. It is
in the closing report.

Fault injection, mechanical, every change run against the package's tests:

| Code | Changes | Caught | Left |
|---|---|---|---|
| `jsoninfer`: `shape.go`, `decide.go` | 133 | 125 | 5 did not build, 3 equivalent |
| `jsoninfer`: `declare.go`, `print.go` | 79 | 79 | |
| `cli/json.go` | 24 | 24 | |

The three equivalent changes are a clamp at its own bound, twice, and one
more turn of a loop that has nothing left to do.

The first passes left 33 changes uncaught, the three equivalent ones among
them. The other 30 led to 13 more cases and two tests, and to three
simplifications: a comparison of kinds that said nothing
the folding did not, a test of length that could not fail, and a flag
returned beside a type that was already empty when there was none.

## Limits

- **An object of names is printed as a record.** `{"alice": 3, "bob": 5}` and
  `{"width": 3, "height": 5}` have the same signals. Keys that are visibly
  data make a keyed table; names do not. `--explain` shows the score.
- **Names are the keys as written.** `tickets` is `Tickets`; nothing is made
  singular.
- **One document.** A member that happens never to be `null` in this
  document is not an `Option`, and three values in three rows are not an
  enum.
- **A tagged array is refused, not approximated.** It does load as a table
  with every tag's members optional, as M3 showed; the command does not
  print that reading, because it loses which members belong to which tag.

## Found outside Json

Each is in `FEEDBACK.md`.

- `record Range { ... }` is accepted and the name still means the builtin
  `Range`. Every other builtin type name is refused.
- `Libraries/SymbolicRegression`'s end-to-end test failed interpreted at the
  M1 baseline and passes now. Nothing in this ladder touches it; it has a
  cycle-time bound and the two runs were under different load.

## Next

The ladder is closed. What is left open, and what I would do next, is in
`internal/json/JSON_V2_CLOSING.md`.
