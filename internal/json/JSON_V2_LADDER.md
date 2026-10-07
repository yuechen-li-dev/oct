# Json v2 — Milestone Ladder Contract

Status: **ACCEPTED 2026-10-05.** M1, M2 and M3 are closed; see
`internal/json/JSON_V2_M1.md`, `JSON_V2_M2.md` and `JSON_V2_M3.md`. M4 is
next.
Base commit: `d44566d` (main).

This document is the source of truth for the `Json` rewrite while the ladder
runs. It supersedes the "IO.Json import posture (Mx104)" section of
`Libraries/IO/README.md` and the design conclusions of
`Experiments/JsonIntentRecoveryLab` M3 and M4.

Four points in section 2 were my reading of a call you made, or a choice you
did not make. They were marked **review**, and were accepted as written with
the rest of the document (D12 to D15).

---

## 1. Why

The thesis stands: most JSON that a program imports is a table, a mapping or a
nested record written as objects. The current implementation does not act on
it. It classifies a document and returns a label.

```oct
let recovered = IO.ImportJson("people.json")?
// recovered.Kind      == "table.columnar"
// recovered.Canonical == "table.columnar: {columns: inferred, sparse_optional: deterministic}"
// recovered.Raw       == the compact JSON text
```

Audit findings that drive this ladder (2026-10-05):

| # | Finding | Consequence |
|---|---|---|
| F1 | `ImportJson` returns `JsonRecovered { Kind, Canonical, Raw }`. `Canonical` is a fixed string written in `IO.Json.oct` | Nothing is recovered. No record or table ever comes out of an import; the tests assert that the string contains `"table.columnar"` |
| F2 | The experiment's recovered files (`M0/recovery`, `M1/recovery`) are hand-typed Oct literals. Its test checks root kinds and that normalising twice gives the same text | The recoveries the reports argue from were never checked against the corpus |
| F3 | The public structured form is `JsonRawGraph`: a flat node list with `Id` and `ParentId`, a `Kind` that is a `String`, and four value slots on every node | The low-level representation is the API. Every lookup scans all nodes, and "not found" returns `Nodes[0]` |
| F4 | The typechecker accepts the type argument by name: `JsonRawGraph` or `IO.JsonRawGraph` | Any record with that name qualifies; the experiment redeclares it locally |
| F5 | Loading alters data without saying so: numbers are `Float`, object members are sorted, a repeated key keeps its last value | `9007199254740993` is not preserved; document order is lost; a duplicate key is not reported |
| F6 | `JsonLower` and `JsonLoadStructured` are interpreted only | The two `JsonIntentRecoveryLab` tests that fail compiled |
| F7 | The compiled lane starts `octxiliary-json`, whose whole job is `json.Compact` | A process and a wire protocol for a function in Go's standard library |
| F8 | Error text differs by lane and is Go's: `JsonNormalize: InvalidData: invalid character '}' looking for beginning of value` interpreted, `invalid character '}' …` compiled | No file position, no path into the document, and no runtime contract can state one text for both lanes |
| F9 | `JsonParse`, `JsonStringify` and `JsonNormalize` are one function. `Json.Object` returns its argument | Three names that promise parsing and serialising do neither |
| F10 | Two packages own JSON. `Libraries/Json` forwards to `Libraries/IO`, where `Load`, `Save`, `Parse` and `Stringify` mean JSON | `IO.Load(path)` reads JSON and nothing else |
| F11 | What programs do with JSON is write it, by concatenating strings: `Json.Object("{\"totalCases\":" + String.From<Int>(n) + …)`, 32 uses in 19 files. The 11 reads are `IO.Load`, mostly followed by `Len(...) > 0` | An unescaped quote in a value is a runtime error at the save; a mistyped key is silent |
| F12 | Classification alone costs 5.6 s interpreted for 4,000 rows (250 KB) | The recovery policy is written in Oct over the node list |

Measured on `d44566d`, linux/amd64, 2 cores.

## 2. Frozen decisions

| ID | Decision |
|---|---|
| D1 | **The declared type is the intent.** Recovery is schema-directed loading: `Json.Load<T>(path)`. The type the program declares says whether an array of objects is a table or a list of records. Nothing is guessed at run time. |
| D2 | **JSON lowers to the Octagon data model** and is materialised by the path `LoadOctagon<T>` uses. Refinement admission, dimension checks and table construction checks are the existing ones. |
| D3 | **`Option<T>` is a builtin enum**, `None` or `Some(T)`. It is how an absent or `null` value is declared. (Your call 1.) |
| D4 | **Unknown members are an error** that lists them. (Your call 2.) |
| D5 | **Keys match fields with case and separators ignored**: `read_timeout_ms` matches `ReadTimeoutMs`. A collision is an error. Writing uses the field name as declared. (Your call 3.) |
| D6 | **A number loads into a dimensioned field in the SI unit the field declares.** `Float<s>` takes `1.5` as 1.5 seconds. (Your call 4.) |
| D7 | **No untyped JSON value.** A program that does not know the shape runs `oct json infer` and declares it. (Your call 5.) |
| D8 | **Tagged arrays are deferred.** A `"type": "credit"` discriminator into a payload enum needs its own decision. (Your call 6.) |
| D9 | **One Go implementation**, `internal/octjson`, imported by the interpreter and by generated programs (precedent: `internal/octrandom`). No sidecar. No second copy. |
| D10 | **Inference is tooling.** `oct json infer` proposes declarations from a document, with `internal/judgment` for the ambiguous cases. It never runs inside a program. |
| D11 | **Clean break.** No v1 name survives M5. Recorded JSON artifacts change bytes and are regenerated. |
| D12 | **`Option` variants are qualified**, as every Oct enum's are: `Option.Some(x)`, `Option.None`. Bare `Some(x)` and `None` are not accepted. The type argument comes from the declared type at the site, or is written: `Option<Float>.None`. It is not inferred from the payload. See 3.1. |
| D13 | **No prefixed units in this ladder.** A millisecond count is a plain number: `ReadTimeoutMs: Float`, then `config.ReadTimeoutMs * 1e-3s` where seconds are wanted. That expression works today in both lanes. An `<ms>` unit is a language decision about prefixes; when it is made, D6 applies to it unchanged. |
| D14 | **`Json.Save` returns `Void ! Error`**, not the `Int` status the file builtins return. A value JSON cannot hold (`NaN`, an infinity) stops the program with a message that names the place; it is not an `Error`. |
| D15 | **A table is read from an array of objects or from a keyed object, and from nothing else.** The columnar form `{"id": [...], "name": [...]}` loads into an ordinary record with array fields. Reading it as a `record table` would have to guess between "keys are columns" and "keys are rows". |

## 3. Normative specification

### 3.1 `Option<T>`

```oct
record table Ticket {
    Id:       String
    Assignee: Option<String>
}

fn AssigneeOr(assignee: Option<String>, fallback: String) -> String {
    return match assignee {
        case Option.Some(name) => name
        case Option.None => fallback
    }
}

let nobody: Option<String> = Option.None
let somebody = Option<String>.Some("sam")
```

- `Option<T>` is a compiler-owned enum with the variants `None` and `Some(T)`.
  It needs no declaration and no import. `Option` cannot be declared by a
  program.
- `T` is any type a record field may have, except `Void`.
- It is an enum in every respect: qualified variants, exhaustive `match` and
  `switch`, `==` and `!=`, use as a field, a table cell, an array element, a
  board field, a parameter and a result. *(M1: this read "`==` and `!=` where
  `T` has them". An enum compares for every payload type, so an option does;
  the comparison is by value at any depth.)*
- `Option<A>` and `Option<B>` are different types.
- The type argument is taken from the type the site declares: a typed binding,
  a parameter, a result, a field, a table cell, an element of a declared array.
  *(M1: also an assignment, an enum payload, the other operand of `==`, `!=`
  and `Assert.Equal`, the second argument of `Append`, a flow turn input and
  `yield`, and a candidate of `when utility Option<T>`; and an `if`, `match` or
  `switch` expression passes the type to its arms. The full list is in
  `Language/reference/language/12-enums.md`.)*
  Where the site declares none it is written, `Option<Float>.Some(1.5)`.
  `let x = Option.Some(1.5)` is an error that says to write one or the other.
  This is the rule template applications already follow.
- `?` and `!` do not apply. Absence is not an error.
- It is Octagon data, written as any payload enum is: `Option.Some(42)`,
  `Option.None`.
- No helper functions in this ladder. `match` is the one way to open it.

### 3.2 Surface

```oct
Json.Load<T>(path: String)  -> T ! Error      // read a file as a T
Json.Parse<T>(text: String) -> T ! Error      // read text as a T
Json.Save(path: String, value: T) -> Void ! Error
Json.Text(value: T) -> String
Artifact.WriteJson(path: String, value: T)    // during `oct artifact`
```

`Json` is a compiler-owned namespace, like `Entropy` and `Artifact`: the
functions are builtins with no Oct declarations, and calling them needs no
`import`. `Libraries/Json` exists so that `import Json` resolves and so that
the package has a manifest, a README and tests. The naming rule is the one
that governs `Random` and `Entropy`.

`T` must be JSON-representable (3.3). A `T` that is not is a compile error
that names the part that is not.

*(M3: until M5 removes the first library, `Json.Load(path)` with no type
argument is still that library's function and needs `import Json`. With a
type argument it is this one.)*

### 3.3 JSON-representable types

| Oct type | JSON |
|---|---|
| `Bool` | `true`, `false` |
| `Int`, `Int<D>` | A number with no fraction and no exponent, in the 64-bit range |
| `Float`, `Float<D>` | Any number that is finite as a 64-bit float |
| `String` | A string |
| A tag-only enum | A string that names a variant, matched as keys are (3.4) |
| `Option<T>` | `null` is `None`; anything else is `Some` of a `T`. As a record field or a table cell, an absent member is `None` too |
| `record` | An object |
| `T[]` | An array |
| `Vector<T>` | An array of numbers |
| `Matrix<T>` | An array of arrays of numbers, all the same length |
| `record table` | An array of objects, or a keyed object (3.5) |
| A refined concept | Its base type, admitted through the concept's requirements |

Not representable: `Complex`, `Bytes`, `Range`, `UI`, `Error`, function
values, tuples, flow instances, a payload enum other than `Option`, and
`Option<Option<T>>`.

A value of a refined concept is admitted as it is read, so a refusal has the
value's path and position. A refined array is admitted whole. The key of a
keyed table is admitted when its column is a refined `String`. *(M3.)*

`T` is followed into every package it reaches, whether or not the file that
makes the call imports that package. *(M3.)*

There is no conversion between kinds. `"42"` is not an `Int`, `1` is not a
`Bool`, and `1.0` is not an `Int`. An `Int` literal is a `Float` where a
`Float` is declared, which is the language's rule for a declared type.

### 3.4 Records and keys

- A JSON key and a field name match when they are equal after removing `_`,
  `-`, `.` and spaces and ignoring case. `read_timeout_ms`, `readTimeoutMs`
  and `ReadTimeoutMs` are one name.
- Every field needs a member, except a field of type `Option<T>`.
- A member that matches no field is an error. The message lists every unknown
  member of that object and the fields the record has.
- Two members of one object that match the same field are an error. So is a
  repeated key.
- Two fields of one record that match each other make the record not
  JSON-representable: a compile error at the `Json` call.

### 3.5 Tables

A `record table R` reads from two shapes.

**An array of objects.** Each object is a row, read as a record whose fields
are `R`'s cells. Rows are in document order.

```json
[ {"id": "T-1", "assignee": "sam"}, {"id": "T-2", "assignee": null}, {"id": "T-3"} ]
```

**A keyed object.** `R`'s first field must be a `String`. Each member is a
row; its key is the first cell. The member's value supplies the rest:

- If `R` has one other field and the value is not an object, or that field is
  itself read from an object, the value is that cell.
- Otherwise the value is an object whose members are the remaining cells.

```json
{ "invoice.failed": 5, "user.deleted": 1 }
{ "u-100": {"name": "Avery", "active": true}, "u-101": {"name": "Mina", "active": false} }
```

The first is `record table Retry { Event: String  Retries: Int }`, the second
`record table Person { Id: String  Name: String  Active: Bool }`.

### 3.6 Writing

- A record writes as an object whose keys are the field names as declared, in
  declaration order. A `record table` writes as an array of row objects.
  `Option.None` writes as `null`; the member is not omitted.
- Output is UTF-8 with two-space indentation and one final newline. An array
  whose elements are all scalars is written on one line.
- A `Float` is written in the shortest form that reads back as the same value,
  and always with a fraction or an exponent: `1.0`, not `1`. Its digits are
  written out between 1e-6 and 1e21 (`1500000.0`, `0.000001`); outside that
  range it takes an exponent (`1e+21`, `1e-07`).
- The layout of an array follows its element type, not its values, so a
  file's shape does not change with its data.
- Strings escape `"`, `\` and control characters, and nothing else.
- **Round trip:** for every JSON-representable `v` of type `T`,
  `Json.Parse<T>(Json.Text(v))` equals `v`.

### 3.7 Documents

Strict RFC 8259: no comments, no trailing commas, no content after the value.
A leading UTF-8 byte order mark is skipped. Nesting deeper than 512 is an
error.

### 3.8 Errors

One text, produced in one place, identical in both lanes:

```
Json.Load: tickets.json: $[1].assignee (line 9, column 17): expected String or null, found a number
Json.Load: config.json: $.service.http (line 4, column 13): unknown members "prot", "tls"; HttpConfig has Host, Port, ReadTimeoutMs
Json.Load: people.json: $.people[2] (line 14, column 5): missing "Active"
Json.Parse: (line 1, column 7): expected a value, found '}'
Json.Load: service.json: $.http.port (line 6, column 13): refined concept Port: a port is below 65536
Json.Load: missing.json: the file does not exist
```

A column counts characters, not bytes, and a byte order mark is not a column.
A missing field is named as the record declares it: the document does not say
how it would have spelled a key it left out. One object's complaints come in
this order: a key written twice, or two members for one field; then every
unknown member; then every missing field; then the fields' own values, in
declaration order.

A file that cannot be read is an `Error` that names the path and the reason in
Oct's words. No message contains Go's. *(M3: the reasons are "the file does
not exist", "this is a directory, not a file", "the file cannot be read:
permission denied" and "the file cannot be read".)*

A refusal by a refined concept is the text of the concept's checked
constructor, at the place of the value. *(M3.)*

### 3.9 Native core (Go, `internal/octjson`)

| Part | Does |
|---|---|
| Parser | Text to a tree that keeps member order, number text and the position of every value. Written here, not `encoding/json`: positions, duplicate keys and exact integers need it |
| Schema | A lane-neutral description of `T`: kind, field names and order, cell types, variants, and the refined concept a value is admitted to |
| Decode | Tree and schema to an Octagon data value, applying 3.3 to 3.5. It asks the lane, through `Admit`, whether a value of a refined concept is admitted |
| Encode | Octagon data value and schema to text, applying 3.6 |
| Infer | Tree to proposed declarations (3.11) |

Each lane hands the decoded value to the materialiser it already uses for
`LoadOctagon`. Neither lane contains a rule from 3.3 to 3.6.

*(M3: this read "each lane builds the schema from its own type information".
The schema is built in one place instead, `internal/jsontype`, from the
program's declarations, and the typechecker, the interpreter and the compiled
lane call it. Three builders would have had to agree on field order, on the
names a message gives, and on which package a name is resolved in; one cannot
disagree with itself. The typechecker's own type information would not have
been enough in any case: it knows the packages a file imports, and a type
reaches further.)*

### 3.10 Removed

- Builtins: `JsonNormalize`, `JsonParse`, `JsonStringify`, `JsonLoad`,
  `JsonSave`, `JsonLower`, `JsonLoadStructured`.
- `IO`: `NormalizeJson`, `Parse`, `Stringify`, `Load`, `Save`, `ImportJson`,
  `ImportRawJson`, `ImportRawJsonGraph`, `LowerJsonToRawGraph`,
  `RecoverJsonIntent`, the records `JsonRawGraph`, `JsonRawGraphNode`,
  `JsonRecovered`, `JsonNodeLookup`, and every `LooksLike*`, `Canonical*` and
  node helper in `IO.Json.oct`.
- `Json.Object`.
- `cmd/octxiliary-json` and its entries in the sidecar build, the registry and
  the reference.
- The typechecker's special case for a type named `JsonRawGraph`.

`String.EscapeJson` and `String.QuoteJson` stay; they are string functions.

### 3.11 `oct json infer`

```
oct json infer tickets.json [--name Ticket] [--explain]
```

Prints the `record`, `record table` and `enum` declarations the document
loads into, and the `Json.Load<...>` line. Output is deterministic.

- An array of objects becomes a `record table`. A member that is absent or
  `null` in some rows becomes `Option<T>`.
- An object is a record, or a keyed table. That choice has several signals and
  no single one decides: how many members there are, whether the keys are
  identifiers, whether the values share a type. It goes through
  `internal/judgment`, and `--explain` prints the trace.
- Equal-length arrays of numbers become `Matrix<Float>`; other nested arrays
  stay arrays.
- A string that takes few distinct values is proposed as a `String`, with the
  enum it could be in a comment. Inference does not invent enums.
- A shape it cannot express (a tagged array, mixed element types) is reported
  with its path, and no declaration is printed for it.

### 3.12 Global invariants

- **I1 Lane parity.** Interpreted and compiled execution give equal values,
  equal bytes and equal error text. This holds by construction (D9).
- **I2 Round trip.** As 3.6.
- **I3 No silent change.** A document either loads into exactly what it says,
  or fails with its position. No precision is lost, no member is dropped, no
  order is changed.
- **I4 No guess at run time.** Every rule in 3.3 to 3.5 is decided by `T`.
  None looks at the document to choose between two readings.
- **I5 Determinism.** The same value writes the same bytes on every platform.

## 4. Milestones

Each milestone has a verdict line, `SUCCESS` / `PARTIAL` / `BLOCKED`, recorded
in `internal/json/JSON_V2_M<n>.md` when it closes. Milestones run in order. No
milestone may weaken an existing compiled-lane assertion to pass (see
AGENTS.md).

Verification, as amended on 2026-10-06. M1 changed the language, and closed
with the whole-tree sweeps in both lanes, the four Go lanes and fault
injection. M2 to M6 are local to the Json library, so each closes with
bounded tests: the tests of the code it adds, the Json library's own
contracts in both lanes, the Go tests of every package it touches, and fault
injection on the code it adds. M5 moves 36 Oct files off the v1 surface and
regenerates recorded artifacts, so it also runs the directories it touches.
The whole-tree sweeps run once more when the ladder closes.

### M0 — Contract freeze
- **Scope:** Review and accept this document.
- **Exit:** Document accepted, with the four **review** rows settled. No code
  changes.
- **Verdict:** SUCCESS. Accepted 2026-10-05 as written.

### M1 — `Option<T>`
- **Scope:**
  - 3.1, in the typechecker and both lanes, and as Octagon data in both
    loaders and the writer.
  - `oct fmt` and the editor grammar.
  - Reference: `12-enums.md`, `02-types.md`, `34-octagon.md`.
  - The Verilog profile and the WebAssembly target refuse it as they refuse
    any payload enum; checked, not assumed.
- **Tests:**
  - `Language/Types/Option/valid`, both lanes: each place a type may appear,
    `match` and `switch`, equality, nesting in records, tables and arrays, a
    board field, an Octagon round trip.
  - `Language/Types/Option/invalid/*.octfail`: no type at the site, a wrong
    payload type, a non-exhaustive `match`, bare `Some` and `None`, declaring
    `Option`, `?` and `!` on one, `Option<Void>`.
- **Exit:** Both lanes green. `TestLanguageCorpusRunsInBothLanes` green.
- **Risk:** This is the one milestone that changes the language. If the
  mechanism turns out to need general template enums, it stops and reports;
  it does not widen on its own.

### M2 — `internal/octjson` core
- **Scope:** 3.9 except Infer: parser, schema, decode, encode. No Oct wiring.
- **Tests:** Go tests in `internal/octjson`, which is host-side implementation
  validation under the AGENTS.md exception.
  - The parser against the accept and reject cases of JSONTestSuite, copied
    with a source citation.
  - Every row of 3.3 and every rule of 3.4 and 3.5, accepted and refused.
  - Positions and paths in every error of 3.8.
  - Exact integers at the 64-bit limits; a duplicate key; member order.
  - I2 as a property over generated values.
- **Exit:** `go test ./internal/octjson` green. No other package imports it.
- **Verdict:** SUCCESS, 2026-10-06. See `JSON_V2_M2.md`. Sections 3.6 and 3.8
  were amended with what the milestone had to decide.

### M3 — Reading, both lanes
- **Scope:**
  - `Json.Load<T>` and `Json.Parse<T>`, registered through the builtin
    definition table. The interpreter calls `octjson`; generated programs
    import it through the staged-build path.
  - The v1 surface stays beside it, untouched.
- **Tests:**
  - `Language/Builtins/Json/valid`, both lanes: every representable type, both
    table shapes, `Option` in fields and cells, key matching, refined
    concepts, dimensioned fields.
  - `Language/Builtins/Json/invalid/*.octfail`: compile-time contracts for a
    type that is not representable, argument count and types, and
    fallibility; runtime contracts, each stating one error text of 3.8 for
    both lanes. *(M3: the redeclaration contract moves to M5. `Libraries/Json`
    declares `Load` until then, so there is nothing to refuse yet.)*
  - **Acceptance corpus.** `Experiments/JsonIntentRecoveryLab` corpus files 01
    to 05 load into `record` and `record table` declarations and equal values
    written out in the test. Files 06 and 07 are tagged arrays; the test
    records that they are refused and why (D8).
- **Exit:** Both lanes green with equal results. `grep` finds no rule of 3.3
  to 3.5 outside `internal/octjson`.
- **Verdict:** SUCCESS, 2026-10-06. See `JSON_V2_M3.md`.

### M4 — Writing, both lanes
- **Scope:** `Json.Save`, `Json.Text`, and `Artifact.WriteJson` taking a
  value. The artifact write capability and its staging are the existing ones.
- **Tests:** Byte goldens for 3.6; I2 through both lanes; a value JSON cannot
  hold; the artifact path during `oct artifact`.
- **Exit:** Both lanes green, writing equal bytes.

### M5 — Migration and removal
- **Scope:**
  - Remove everything in 3.10.
  - Move the 36 Oct files that use the v1 surface to typed values: 29 under
    `Experiments` in seven experiments, 6 under `Libraries`, 1 under
    `Language`. Each hand-built object becomes a record.
  - Regenerate the recorded `.json` artifacts they write. Bytes change:
    indentation, and key spelling from `totalCases` to `TotalCases`.
  - Tests that read a summary back assert its fields, not `Len(...) > 0`.
  - Reference, `Libraries/IO/README.md`, `docs/COMPILED_SUPPORT.md`,
    CHANGELOG.
  - Close `Experiments/JsonIntentRecoveryLab` with a final note that points
    here. Its reports stay as the record of how the design was reached.
- **Exit:** `grep` finds no name from 3.10 outside this document and the
  milestone reports. No experiment builds JSON from strings. Sweeps show no
  test going from passing to failing.

### M6 — `oct json infer`
- **Scope:** 3.11.
- **Tests:** Golden output for the seven corpus files, the 14 other JSON
  files under `Experiments` and the 7 under `Libraries/IO/testdata`. For each file whose output has no refused shape,
  the printed declarations compile and `Json.Load` of that file succeeds with
  them.
- **Exit:** The command is in `35-cli.md`. The loop "infer, paste, load" works
  on every corpus file it accepts.

## 5. Out of scope

- Tagged arrays into payload enums (D8).
- Prefixed and scaled units (D13).
- Columnar objects as tables (D15).
- `Option` helpers, and `Option` in `?` or `!`.
- General template enums.
- `Csv`. Typed loading into a `record table` is the same idea and a separate
  ladder.
- JSON Lines, streaming, and documents too large to hold in memory.
- The interpreter's cost per element write, recorded in `FEEDBACK.md`. A
  large table loads in one construction, so loading does not meet it; a
  program that then fills arrays by index does.

## 6. Risks

| Risk | Handling |
|---|---|
| `Option<T>` is larger than it looks | M1 is first and alone. Nothing else starts until it closes |
| The compiled lane's Octagon materialiser is a second implementation, kept as a Go string in `emit_go_runtime.go` | D2 feeds both materialisers one decoded value, so a JSON rule cannot differ between them. A difference that already exists between the two for Octagon will show up in M3 and is reported, not patched around. *(M3: three showed up, in vectors and matrices, ragged matrices and refined arrays. Json needs all three, so each was fixed in the materialiser itself, with Octagon contracts in both lanes; see `JSON_V2_M3.md`.)* |
| Strict unknown-member checking makes third-party payloads tedious | That is D4's cost. `oct json infer` writes the full declaration, which is the intended answer |
| Recorded artifacts change | D11. One regeneration commit in M5, with the reason in each experiment's report |
