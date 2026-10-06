# Json v2 — M2: `internal/octjson` core

Date: 2026-10-06
Ladder: `internal/json/JSON_V2_LADDER.md`
Base commit: `a1345bc`

## Verdict

**SUCCESS.** `internal/octjson` parses JSON, describes an Oct type as a
schema, decodes a document as a schema into Octagon data, and encodes Octagon
data as text. It is Go only. Nothing imports it yet, and no Oct program
behaves differently.

One item is open for M3; see "Open for M3".

## What is in the package

| File | Does |
|---|---|
| `parse.go`, `document.go` | `Parse`: strict RFC 8259 text to a `Document`, a tree that keeps member order, a key written twice as two members, the text of every number, and the offset of every value and key. `Position` gives line and column. |
| `schema.go` | `Schema`: kind, name, dimension, element, fields, variants. `Check` says why a type is not JSON-representable and names the part. `FoldName` is the one place that says when a key and a field are the same name. |
| `data.go` | `Data`: Octagon data, lane neutral. A table is a record of whole columns, as its Octagon literal is; an option is an enum named `Option`. |
| `decode.go` | `Decode`: document and schema to `Data`. Sections 3.3, 3.4 and 3.5. |
| `encode.go` | `Encode`: `Data` and schema to text. Section 3.6. |
| `errors.go` | `Error` and `Error.Text(operation, source)`, the one place a message is put together. Section 3.8. |

About 1400 lines in eight files (`doc.go` is the package comment), with no
dependency outside the standard library and none on `encoding/json`.

## Choices the ladder left open

Each is the plainest reading I could find, and each has a test.

- **A column counts characters, not bytes,** and a byte order mark is not a
  column.
- **Order of complaints about one object:** a key written twice or two
  members for one field, at that key; then every unknown member; then every
  missing field; then the fields' own values, in declaration order.
- **`missing` names the field as the record declares it.** The ladder's
  example read `missing "active"`. The document does not say how it would
  have spelled a key it left out, so the message says `missing "Active"`.
- **A keyed table.** "The field is itself read from an object" means a
  record, a keyed table, or an option of either.
- **The layout of an array follows its element type, not its values.** An
  `Option<Site>[]` is written one element to a line even when every element
  is `null`, so a file's shape does not change with its data.
- **A Float is written with its digits between 1e-6 and 1e21** (`1500000.0`,
  `0.000001`) and with an exponent outside that range (`1e+21`, `1e-07`).
  Go's shortest `%g` would write `1.5e+06`.
- **A String that is not UTF-8 cannot be written,** and is reported like a
  NaN: `Unwritable`, with the path.
- **A number smaller than the smallest Float loads as `0.0`.** Section 3.3
  asks only that it be finite.
- **JSONTestSuite's free cases.** Every well-formed number parses, however
  large, because the tree keeps its text and the declared type decides. Text
  that is not UTF-8, and a `\u` escape that is half a surrogate pair, are
  rejected.

## Tests

`go test ./internal/octjson`: 29 tests, 100% of statements.

| Where | What |
|---|---|
| `parse_test.go` | All 318 files of JSONTestSuite (95 that must parse, 188 that must not, 35 free, each of those stated). Member order, repeated keys, number text. Positions with multi-byte characters, tabs, CRLF and a byte order mark. The depth limit, for arrays, objects and both. 44 syntax errors, each with its whole message. |
| `decode_test.go` | Every row of 3.3 accepted and refused, with the limits of the 64-bit range and an integer above 2^53. Every rule of 3.4 and 3.5, including both keyed forms and the columnar form that is not a table. The three load messages of 3.8 at the positions the ladder shows. |
| `encode_test.go` | One document that shows the whole layout. Floats, strings, what cannot be written, and a value that is not what its schema says. **I2 as a property:** 3000 generated types, each with a generated value, written, read back equal, and written again to the same bytes. |
| `schema_test.go` | `FoldName`, `Check` for every refusal, a type that names itself, the forms of a message, paths for keys that need quoting. |
| `testdata/JSONTestSuite` | Copied unchanged from nst/JSONTestSuite at `1ef36fa`, MIT, with `SOURCE.md` and the licence. |

Fault injection, mechanical: every comparison, logical operator, boolean,
`+ 1`, `++`, `continue` and a few constants in the seven files that hold
code was changed one at a time, 317 changes in all. 5 did not build and 311 were
caught. The first pass missed 19, which led to one fix (a column after a
byte order mark was off by one) and 14 more test cases. The one left is
equivalent: clamping an offset that equals the length of the text changes
nothing.

Nothing outside the package was run, as the ladder's amended verification
rule says: nothing outside it changed.

## Open for M3

**`Vector<T>` and `Matrix<T>` are not Octagon data.** An `.octagon` file
cannot hold a vector or a matrix literal, and both lanes' `LoadOctagon`
materialisers refuse a vector or matrix as the expected type. `Decode` gives
a vector as an array of numbers and a matrix as an array of rows, checked
for equal length. M3 has to teach each materialiser to take that array where
a vector or a matrix is declared. That is a small extension of each, and it
is the kind of difference the ladder's risk table said M3 would surface.

## Next

M3: `Json.Load<T>` and `Json.Parse<T>` in both lanes. Each lane builds a
`Schema` from its own type information, calls `Check` at compile time, and
hands the `Data` to its materialiser.
