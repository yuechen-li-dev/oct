# Compiled data M0

## 1. Verdict

Success

The M5-blocking representation bugs are closed and a bounded first-class
publication path now emits typed Oct data as deterministic static Go. The
10,000-row backend characterization is about 0.063 seconds on the measured
host, versus the external M5 `Artifact.WriteLines` result of roughly 93
seconds. The new path is not a serializer and emits no runtime reconstruction.

## 2. M5 gap reconstruction

| External gap | Root cause | M0 disposition |
|---|---|---|
| Interpreted/compiled nominal table mismatch | The interpreted Octagon loader applied a table schema's cell type directly to a column expression; compiled storage applied the implicit array depth. | Fixed by applying column depth exactly once in interpreted loading and preserving logical field types in compiled metadata. |
| Compiled enum table panic | Oct enums compile to `{Tag, Payload}` structs, but reflection materialization called `SetInt` on the struct. | Fixed by validating the nominal enum and setting its `Tag` field. |
| Refined cells were erased or rejected | Compiled reflection recursed with Go types such as `float64`, losing the logical refined type; interpreted loading had no refinement registry. | Logical field/element types now survive recursion and both paths invoke authoritative refinement constructors. |
| Nominal table was awkward as artifact input | The artifact path only offered textual sinks. | `Artifact.WriteCompiledData` consumes the typed nominal value directly. |
| 93-second string emission | Every line and conversion executed in the Oct interpreter before a text write. | The Go emitter traverses typed data once and formats one backend-owned source unit. |
| No table `with` | The checker explicitly rejected record-table sources. | Complete-column immutable replacement is supported with exact type and extent checks. |
| Whole-table invariants required `[Fact]` | `Require` intentionally accepts only bounded refinement expressions and `Assert` is test-oriented. | General `StaticAssert.*` calls can live in ordinary helpers and execute under a compiler-owned static phase. |
| Weak derivation | No SQL/dataframe abstraction exists, and none is justified here. | The specimen reuses normal loops, row projection, `Append`, and functions to derive a static index. |
| Handwritten identity/freshness | Application code owned snapshot constants and bespoke checks. | The emitter owns canonical logical/schema hashes and row count; artifact publication reports generated-source SHA-256 and `produced`/`unchanged`. |
| Octagon documentation contradiction | A supposedly valid data-only example included `package Main`. | Fixed. |

No Copeland/TableScript source or documentation was present in the accessible
repository, so the historical description was treated only as design input.

## 3. Unified data model

An ordinary record is a nominal product. A record table is the same nominal
schema with one compiler-owned storage rule: every declared cell type gains one
column array depth. Row projection removes exactly that depth and produces an
internal immutable row record. Octagon carries the nominal table literal with
complete column arrays. Materialization validates that shape into the ordinary
semantic value. Compiled-data IR retains the same nominal type graph and emits
backend-native declarations and literals; it does not introduce a runtime data
model.

## 4. `with` design

Supported:

- records and record-shaped Concepts: existing field replacement;
- record tables: one or more complete-column replacements;
- nested composition, such as updating a projected ordinary record before
  constructing/replacing a containing value.

Table `with` returns the same nominal table type, evaluates its source once,
preserves unspecified columns, checks replacement types statically, checks
dynamic extent against the source once, and leaves the source unchanged.
Interpreted execution creates a new record field map; compiled execution creates
a new Go table struct. Unchanged column slice headers/data are shared. This is
not observable aliasing because Oct arrays remain values and table columns are
not mutable through the table.

Rejected/deferred:

- row-key syntax and in-place row/column assignment;
- append/delete/mutable tables;
- payload-enum `with`.

Payload enums were investigated but not added. Preserving the variant is clear,
but Oct's current payload representation is `any` in the Go backend and the
language has no general named payload-field structure. Adding `with` now would
create enum-specific dynamic behavior instead of the common bounded structural
rule. Variant changes remain explicit construction.

## 5. Static assertion design

`Require` remains the bounded type/refinement admission expression. It did not
gain loops, table indexing, I/O, or general evaluation. `Assert` remains the
runtime/test assertion family.

`StaticAssert.True/False/Equal/Near/Error` is lexically general: calls may live
in ordinary deterministic helpers. They execute only when that call graph runs
under a compiler-owned static evaluation phase. M0's concrete phase is
`oct artifact`; ordinary runtime execution reports that no static evaluator is
active rather than silently becoming a runtime assertion. Artifact capability
rules continue to reject ambient I/O, time, and undeclared mutation. Failures
include the static entry function and user message.

## 6. Octagon parity

| Shape | Before | After |
|---|---|---|
| Primitive nominal table | Compiled success; interpreted column/cell mismatch | Both pass |
| Tag-only enum cell | Compiled reflection panic | Both retain nominal enum and pass |
| Refined scalar cell | Interpreted mismatch; compiled logical type erased | Both run the Concept refinement admission and pass |
| Malformed column/cell | Generic mismatch or panic | Table name, column, array element, and expected/received types are reported |
| Artifact/interpreter | Nominal table unsuitable | The dogfood artifact loads the nominal table directly |

The new language fact `LoadNominalRecordTableWithEnumAndRefinementCells` ran in
compiled mode without fallback. Three older Octagon facts still fall back for
pre-existing compiled parser gaps involving dimension suffixes, comma-free
arrays, and comma-free records; they are unrelated to nominal tables and are
listed under remaining limitations.

## 7. Compiled-data backend

```text
Octagon
  -> typed nominal Oct value
  -> ordinary deterministic derivation + StaticAssert
  -> internal/compileddata typed IR
  -> deterministic Go declarations/literals + canonical identities
  -> go/format
  -> staged Artifact output
```

`Artifact.WriteCompiledData(path, symbol, value)` is the bounded entry. Initial
shapes are scalar values, arrays, records, tag-only enums, refined scalar
Concepts, and record tables. A root table emits a fixed row array because that
is the direct static query shape demonstrated by M5. Independently derived
arrays (for example row-ID indexes) use the same emitter.

## 8. Generated-Go audit

The specimen emits:

```go
type Status int
type PositivePrice = float64
type CatalogRow struct { /* nominal typed fields */ }
var CatalogData = [...]CatalogRow{ /* literals */ }
var PublishedIDs = [...]int{2, 3, 5, 6}
```

Audit results:

- fixed arrays: yes;
- nominal row and enum types: yes;
- precomputed index: yes;
- `init`: absent;
- reflection: absent;
- decoder/parser: absent;
- append/map construction: absent;
- runtime constructors: absent.

The copied first-party Go reader built and directly read row 2, refined price,
enum status, and the derived index without any Oct runtime code.

## 9. Publication performance

Host: Windows amd64, AMD Ryzen 7 7700X, Go benchmark process.

| Measurement | Result |
|---|---:|
| External M5 interpreted 10k line emission baseline | ~93 s |
| 10k typed-IR structural validation + canonical identity | 7.010 ms |
| 10k backend literal emission | 14.758 ms |
| Go formatting | 52.009 ms |
| End-to-end `EmitGo` benchmark | 63.428 ms/op |
| Generated source | 697,234 bytes |
| Throughput | 10.99 MB/s |
| Go build of specimen reader | 659.87 ms |
| Windows executable | 2,474,496 bytes |
| Cold/steady process launch through PowerShell | 91 ms cold; 9.4-15.8 ms steady |
| Direct static query | 0.1887 ns/op, 0 B/op, 0 allocs/op |

The 10k benchmark excludes dataset construction from its timer and exercises
the complete backend validation/hash/emission/format path. The small real Oct
dogfood path (typed Octagon load, `with`, index derivation, three static
assertions, and two outputs) reported 26 ms inside the first artifact run. Its
second run reported 5 ms and both outputs `UNCHANGED`. The current CLI exposes
the small real path as one aggregate rather than separate typed-load and Oct
derivation timers; adding intrusive timing APIs to deterministic Oct was not
justified for M0.

## 10. Derivation helpers

No SQL-like surface or parallel static library was added. The specimen uses the
existing general operations that were sufficient for the pressure shape:

- `Len(table)`;
- row projection `table[i]`;
- range `for` loops;
- enum comparison;
- `Append` to build a typed row-ID projection.

This produces `PublishedIDs` as already-validated typed data consumed by the
backend. Generic sort/group/unique APIs remain a library gap if later dogfood
shows repeated demand; this milestone does not claim SQL, joins, or a planner.

## 11. Snapshot identity

The emitter computes:

- `SymbolLogicalHash`: SHA-256 over canonical semantic type-directed values;
- `SymbolSchemaHash`: SHA-256 over canonical nominal schema/field/variant data;
- `SymbolRowCount`: compiler-owned extent for tables.

These exclude paths, timestamps, Go formatting, and machine identity. The
artifact system separately records SHA-256 over generated bytes. Re-publication
compares those bytes and reports `UNCHANGED` without changing the output file,
separating logical snapshot identity from build-artifact identity.

## 12. Compiler/language pressure findings

| Finding | Classification |
|---|---|
| Table Octagon cell/column depth split | Bug |
| Enum reflection `SetInt` panic | Bug |
| Logical refined type lost during compiled recursion | Bug |
| Contextual array mismatch called every element “refined” | Bug |
| Complete-column table evolution | Missing general abstraction |
| Publication invariant phase | Missing general abstraction |
| Backend-native typed static emission | Compiler backend gap |
| Logical/schema/artifact identity split | Compiler backend gap |
| Sort/group/unique conveniences | Library gap |
| Payload-enum `with` on `any` payload representation | Not worth solving in M0 |
| Octagon package declaration example | Documentation gap |

## 13. Compatibility

Green evidence:

- record-table language corpus: 9/9 negative contracts plus the direct valid
  file's 3/3 compiled facts;
- Octagon load file: 8/8 facts, including the motivating nominal table in
  compiled mode;
- compiled-data specimen fact: compiled, zero fallback;
- artifact failure/capability tests, including duplicate ID, unsorted index,
  and extent mismatch diagnostics;
- targeted Go packages: `compileddata`, `typecheck`, `interpret`, `build`, and
  `tester` all pass;
- generated reader Go test passes;
- deterministic static emitter test passes.

The repository-wide `go test ./...` reached the broader suite. After the
specimen reader was moved under `internal/compileddata/testdata` (so generated
symbols are not expected during normal package discovery), the only observed
unrelated failures were existing Concept-Vulkan `CV3001` stale/hand-edited
generated-output checks across its checked fixtures. This task did not modify
those sources or generated files.

## 14. Remaining limitations

- M0's current compiler-owned static evaluator is the artifact phase. The
  `StaticAssert` language surface is general and ordinary helpers may contain
  it, but a separate `oct check --static` entry lane does not yet exist.
- Compiled-data publication rejects payload enums. Tag-only enums are complete.
- Root empty arrays cannot infer an element type from the interpreter value;
  nominal table columns and typed record fields do not have this ambiguity.
- The emitted root table is row-oriented. Emitting both row and selected
  columnar projections in one declaration plan is future backend work.
- There are no generic sort/group/unique table library helpers yet.
- Three older compiled Octagon grammar cases still fall back as described in
  section 6.
- The emitter records tool output hashes but does not yet include an explicit
  Oct compiler version constant in generated source.

## 15. Exactly one next recommendation

Return to Database-Scheduler M6 and replace the interpreted string publication
path with `Artifact.WriteCompiledData`, keeping any newly repeated sort/group
operation as evidence for a later, narrowly scoped Oct library milestone.
