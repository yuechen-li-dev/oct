# Octxiliary M12 Structured Transport Design Audit

## 1. Executive summary

**Recommended next expansion:** M13 should add exactly one new generic Octxiliary transport shape: `String[][]`, represented in Go as `[][]string`, then migrate the CSV row-table surfaces (`Libraries/Csv` and the underlying `IO.Csv` wrappers) to a dedicated CSV sidecar.

**What it unlocks:**

- `Csv.Read(path) -> String[][] ! Error` and `Csv.Write(path, rows: String[][]) -> Int ! Error`.
- `IO.Read(path) -> String[][] ! Error` and `IO.Write(path, rows: String[][]) -> Int ! Error` as the current `IO.Csv` row-major surface.
- A reusable, deterministic protocol pattern for one level of typed nesting without committing to records, handles, or dynamic graphs.
- A possible later Markdown row-helper path if a public row-major Markdown helper is intentionally added, but M13 should not redesign Markdown APIs.

**What remains deferred:**

- Records and record-of-list transport for Markdown table records, Plot style/label records, JSON raw graph records, and table-shaped structured IO.
- Handles for Image, Pdf, and IO XLSX workbooks.
- `Float[]` / `Float[][]` for Plot and numeric matrix CSV helpers.
- Dynamic JSON/Octagon-ish graph transport or `Any`.
- Compiled Complex, compiled Einstein notation, generated-Go hardening, sidecar build orchestration, native permission prompts, and lockfiles.

**Why this is the smallest correct next step:** `String[][]` is the only remaining candidate that directly unlocks a real standard-library wrapper package with no public API redesign and no lifecycle/type-identity semantics. It is a narrow extension of the existing `String[]` transport, keeps payloads typed, preserves CSV row boundaries, and avoids turning Octxiliary into an untyped JSON carrier. `Float[]` is similarly simple, but the inspected Plot API also requires scalarized record fields before it becomes productively migratable. Records and handles unlock more surfaces but introduce materially larger semantic contracts.

## 2. Current M6 transport contract

M6 generic wrapper lowering currently admits only this transport set:

| Transport | Go-side payload today | Typical use |
| --- | --- | --- |
| `Void` | `octxiliary.Value{Kind: ValueVoid}` / generated void result | side-effect-only operations |
| `Int` | `Value.Int` / `int` | status codes, Unix seconds |
| `Float` | `Value.Float` / `float64` | scalar numeric arguments/results |
| `Bool` | `Value.Bool` / `bool` | predicates |
| `String` | `Value.String` / `string` | paths, text, normalized JSON strings |
| `String[]` | `Value.Strings` / `[]string` | lines, regex matches, archive file lists |
| `Bytes` | `Value.Bytes` / `[]byte` | binary file/hash/compression payloads |

The contract is enforced in compiler lowering through `isOctxiliaryTransportType`, which accepts only `Void`, `Int`, `Float`, `Bool`, `String`, `String[]`, and `Bytes`. The generic call path constructs `octxiliary.Value` arguments, calls `__octOctxiliaryGenericCall`, checks the returned `ValueKind`, and extracts the typed payload back into generated Go values.

M6 and the follow-on wrapper migrations already support these families well:

- **Hash:** SHA-256 text, bytes, and file wrappers fit `String`/`Bytes` arguments and `String` returns.
- **Compression:** gzip bytes and file wrappers fit `Bytes`, `String`, and `Int`.
- **Time:** ISO-8601 and Unix-second helpers fit `String` and `Int`.
- **Text:** regex helpers fit `String`, `Bool`, and `String[]`.
- **Archive:** zip entry listing and extract/create helpers fit `String`, `String[]`, and `Int`.
- **Json file/string subset:** `Json.Load` and `Json.Save` fit `String` and `Int`; `Json.Object` remains pure Oct identity rather than a sidecar need.

This is enough for scalar, line-list, byte-buffer, and file-list wrapper surfaces. It is not enough for row-major tables, numeric series, record-shaped options, stateful resources, or structured graph values.

## 3. Remaining wrapper backlog table

| Package/API | Current public shape | Required transport | M6-compatible? | Recommended action | Future milestone |
| --- | --- | --- | --- | --- | --- |
| `Csv.Read` / `Csv.Write` | `Read(String) -> String[][] ! Error`; `Write(String, String[][]) -> Int ! Error` | `String[][]` | No | Add `String[][]`, add CSV manifest/sidecar, migrate as the first nested-array wrapper | **M13: `String[][]` + CSV migration** |
| `IO.Csv` `Read` / `Write` | `Read(String) -> String[][] ! Error`; `Write(String, String[][]) -> Int ! Error` | `String[][]` | No | Keep public row-major API; either route through CSV family wrappers or a CSV-capable IO wrapper after transport exists | **M13 or immediate follow-up in same CSV-focused milestone** |
| `IO.Csv` `ReadRows` / `WriteRows` (documented) | Explicit aliases of row-major `Read`/`Write` in README | `String[][]` | No | Implement/migrate only if the public Oct files expose these aliases in the target milestone; do not invent broad table APIs in transport work | CSV follow-up if API is present |
| `IO.Csv` `ReadTable` / `WriteTable` (documented) | Record of `String[]` columns; `WriteTable` documented as not implemented | record-of-`String[]` plus field ordering | No | Defer; requires record shape metadata and deterministic field order | Record transport milestone |
| `IO.Csv` `ReadMatrix` / `WriteMatrix` (documented) | `Float[][]` numeric grid | `Float[][]` | No | Defer; not unlocked by `String[][]`; needs numeric nested arrays and matrix validation | Numeric-array milestone |
| Markdown block helpers | Many helpers return `String[]` and accept `String`/`String[]`; `Report`/`Section` compose block lists (`String[][]` in observed tests) | Mixed: M6 for scalar/list helpers, `String[][]` for block composition | Partially | Do not migrate as wrapper yet; direct compiled helper lowering may cover scalar/list helpers, but wrapper transport should not redesign Markdown | Markdown compiled-helper or structured pass |
| Markdown table helpers | `Table(record-of-String[] columns) -> String[]`; `TableWithColumns(record, String[]) -> String[]` | record-of-`String[]` with deterministic column order | No | Defer; row-major `String[][]` is explicitly not the canonical public `Table` contract | Record transport milestone |
| Markdown report/table smoke surfaces | `Report([block1, block2, ...]) -> String[]`; tables use columnar records | `String[][]` for report block composition and records for tables | No | `String[][]` alone may help `Report`/`Section`, but not canonical tables; do not combine with CSV M13 unless a separate direct-helper subset is deliberately scoped | Markdown structured milestone |
| Plot `Line` / `Scatter` | `Float[]`, `Float[]`, path, `Size` record, `Labels` record | `Float[]` plus record/scalarized options | No | Defer; `Float[]` alone is insufficient for the inspected public APIs | Numeric-array + record/scalar-options milestone |
| Plot `Histogram` | `Float[]`, `Int`, path, `Size`, `Labels` | `Float[]`, `Int`, `String`, records | No | Defer with `Line`/`Scatter`; consider scalarized sidecar wire names only after deciding record strategy | Numeric-array + record/scalar-options milestone |
| Pdf `NewPage` / `Draw*` / `Save` | `PdfPage` record with handle; `TextStyle` record; `ImageHandle` record | opaque handles plus records/scalarized record fields | No | Defer; requires ownership/lifecycle semantics | Handle-transport milestone |
| Image `Load` / `Save` / `Width` / `Height` / `Format` | `ImageHandle` record wrapping an opaque handle | opaque handles | No | Defer; handle lifecycle and leak policy should be designed before implementation | Handle-transport milestone |
| IO JSON normalized string helpers | `NormalizeJson`, `Parse`, `Stringify`, `Load`, `Save` over `String` | `String`, `Int` | Mostly yes | Already partly covered by `Libraries/Json`; IO package manifest/wrapper posture can be handled separately without new transport | Wrapper metadata/API alignment follow-up |
| IO JSON structured graph helpers | `JsonRawGraph` containing `JsonRawGraphNode[]`; `JsonRecovered` records | record arrays / typed graph transport or explicitly typed JSON graph adapter | No | Defer; do not use dynamic `Any` as shortcut | Record/graph transport design milestone |
| IO XLSX helpers | `Workbook` record wrapping `Handle: Int`; cell setters and save | opaque workbook handle plus scalar args | No | Defer; same lifecycle issues as handles, despite scalar cell APIs | Handle-transport milestone |

Inspection note: `Libraries/Csv` currently has no `manifest.oct`, and `Libraries/IO/manifest.oct` has no `Wrappers` section. M13 must include manifest work for the CSV migration, but that is package metadata for the selected wrapper, not a package-manager sidecar-build system.

## 4. Candidate transport expansions

### A. `String[][]`

`String[][]` unlocks the CSV row-table shape directly. Both `Libraries/Csv` and the current `Libraries/IO/IO.Csv.oct` implementation expose row-major `String[][]` reads/writes. This candidate is a low-risk extension of `String[]`: it adds exactly one more typed lane and does not require field names, type identity, object materialization, ownership, or `Any`.

The main risk is precedent. If named as “generic nested arrays” too early, it can grow into a loosely specified dynamic container. The mitigation is to add exactly `String[][]` as a named transport in M13, with explicit CSV semantics and protocol tests for empty/ragged cases.

### B. `Float[]`

`Float[]` is also conceptually simple and would serve Plot numeric series inputs and future scientific wrappers. However, the inspected `Plot.Line`, `Plot.Scatter`, and `Plot.Histogram` APIs also require `Size` and `Labels` records at the public API boundary. The current Oct stubs scalarize those records before calling builtins (`PlotRenderLine`, etc.), which could eventually become wrapper wire functions, but a useful Plot migration still needs a deliberate policy for records versus scalarized wrapper wire metadata. `Float[]` alone therefore unlocks less immediate standard-library coverage than `String[][]`.

### C. `Int[]`

`Int[]` may become useful for future numeric helper APIs, pixel coordinate lists, or image metadata batches, but the inspected Image/Pdf APIs are handle-centered and the current backlog does not contain a high-value `Int[]`-only wrapper package. It is not the smallest next step that unlocks a complete package.

### D. Simple nested arrays generally (`T[]` / selected `T[][]`)

A general nested-array framework for supported scalar `T` would reduce one-off transport additions later (`String[][]`, `Float[]`, `Float[][]`, `Int[]`, etc.). It also increases implementation and specification surface now: parser recursion, kind naming, generated-Go mapping, empty nested type inference, and sidecar expectations all become more general. M12 should avoid this until at least one narrow nested transport has validated the protocol shape.

### E. Simple records with scalar/list fields

Records could unlock Markdown table records, Plot label/size records, JSON graph nodes, and many table-like IO surfaces. They also require nontrivial semantic choices:

- How manifest metadata identifies record types and packages.
- Whether wire field order is declaration order, manifest order, or encoded name order.
- How sidecars materialize records without importing Oct compiler type state.
- Whether units such as `Int<px>` are erased or preserved.
- How record arrays are represented and validated.

This is valuable but not the smallest safe next transport. It should follow a dedicated design milestone rather than piggyback on CSV.

### F. Opaque handles

Handles are necessary for Image, Pdf, and IO XLSX. They are not just `Int`: they need sidecar ownership, family-local namespaces, close semantics or process-lifetime semantics, invalid-handle diagnostics, leak policy, restart behavior, and cross-family interaction policy (for example Pdf drawing an Image handle). Handles should not be added until those lifecycle rules are explicit.

### G. Dynamic JSON / Octagon-ish graph / `Any`

A dynamic graph transport could unlock structured JSON quickly, but it is exactly the “untyped swamp” risk. It would blur JSON-as-data, Octagon-as-native serialization, and wrapper ABI types. It would also encourage sidecars to bypass typed public APIs. M12 should reject `Any` as the next step and keep structured JSON behind a future typed record/graph design.

## 5. Recommended next milestone

**Recommended milestone: M13 — add `String[][]` transport and migrate Csv.**

Scope:

1. Extend the Octxiliary protocol by one value kind for `String[][]`.
2. Extend compiler generic wrapper lowering for manifest-declared `String[][]` args/results.
3. Add wrapper metadata for `Libraries/Csv` and/or the selected `IO.Csv` route.
4. Add an `octxiliary-csv` sidecar using Go `encoding/csv`.
5. Add focused protocol, compiler, sidecar, and compiled CSV tests.
6. Run regression tests for previously migrated M6/M7-M11 wrapper families.

This milestone should not include Markdown, Plot, Pdf, Image, handles, records, dynamic graph transport, package-manager sidecar builds, generated-Go hardening, or public API redesign.

## 6. Protocol design proposal for M13 `String[][]`

### ValueKind name

Prefer the literal kind string **`String[][]`** over `StringMatrix`.

Rationale:

- It matches Oct type spelling and manifest type strings.
- It keeps the transport obviously typed, not domain-specific.
- It extends the existing `String[]` naming convention.

Implementation names can still be Go-friendly, for example `ValueStringMatrix` or `ValueStringArrayArray`; the wire `kind` should remain `String[][]`.

### Preferred textual encoding

Use a typed value envelope parallel to existing `String[]` values:

```text
OctxiliaryValue { kind: "String[][]" strings2: [ [ "a" "b" ] [ "c" "d" ] ] }
```

Notes:

- `strings2` is intentionally distinct from `strings` to avoid overloading the `String[]` field.
- Rows are quoted string arrays inside one outer array.
- This follows the existing whitespace-separated Octxiliary payload style rather than introducing JSON commas.
- The parser should reject malformed row delimiters rather than best-effort flattening.

### Deterministic empty cases

- Empty outer array: `strings2: [ ]` represents zero rows.
- Empty row: `strings2: [ [ ] ]` represents one row with zero columns.
- Empty strings: quoted as `""` inside rows.

### Ragged rows

Recommendation: **allow ragged rows in the transport** and preserve them exactly.

Rationale:

- Go `encoding/csv.Reader` can be configured with `FieldsPerRecord = -1` to allow variable field counts.
- CSV is physically row-major and does not require rectangularity.
- Transport should preserve row boundaries and cell content; validation for rectangular tables belongs in higher-level `ReadTable`, `ReadMatrix`, or package-specific APIs.
- Sidecars must not silently pad, truncate, transpose, or normalize ragged rows unless the public API explicitly documents such behavior.

## 7. Compiler lowering proposal

M13 compiler work should be narrow and table-driven.

### Manifest type string validation

- Add `String[][]` to the accepted generic Octxiliary transport set.
- Keep unsupported nested spellings rejected (`String[][][]`, `Float[][]`, `Any`, records, handles) until their milestones.
- Manifest and stub return/argument equality should remain exact: a `String[][]` manifest entry must match a `String[][]` Oct stub type.

### Oct type string mapping

- Preferred Oct type spelling: `String[][]`.
- If the parser/type-string machinery internally normalizes nested arrays differently, M13 must document that exact normalized spelling and require manifests to use that spelling.
- Do not introduce an alias such as `StringMatrix` in public Oct API unless language reference explicitly adopts it.

### Go type mapping

- `String[][]` maps to `[][]string` through the existing array/matrix-aware `goType` path.
- Fallible results use the existing generated result naming path. `goSafeName("String[][]")` currently implies a name like `StringSliceSlice`, so the result type should be `octResult_StringSliceSlice` or an equivalent deterministic name.

### Generated `octxiliary.Value` construction

- Add a new `octxiliary.Value` field for `[][]string` (for example `Strings2 [][]string`).
- Add `octxiliaryValueExpr("String[][]", expr)` to construct `octxiliary.Value{Kind: octxiliary.ValueStringArrayArray, Strings2: expr}`.
- Keep scalar/list construction unchanged.

### Extraction from response

- Add `octxiliaryValueExtractExpr("String[][]", "__value")` returning `__value.Strings2` (or the chosen field name).
- Keep response kind checking in `__octOctxiliaryGenericCall`; this prevents a sidecar from returning `String[]` where `String[][]` was expected.

### Needed tests

- Manifest/compiled wrapper fixture that declares a `String[][]` arg and result.
- Generated Go compile test proving `octResult_StringSliceSlice` (or chosen equivalent) is emitted and used correctly.
- Negative manifest/lowering test proving unsupported neighbors such as `Float[][]` or `String[][][]` still fail until explicitly added.
- Regression tests showing old M6 transports still encode, parse, lower, and extract unchanged.

## 8. Sidecar authoring guidance

For `String[][]` sidecars:

- Validate `req.HasArgs` before dispatch.
- Validate argument count and exact `ValueKind` for every argument.
- Return ordinary wrapper failures as `ok: false` sidecar errors, not panics.
- Avoid logging to stdout; stdout is protocol frames only.
- Preserve deterministic output order and exact row/cell content.
- Use explicit helper functions such as `stringMatrixValue(rows [][]string)` and `expect(args, kinds...)` to match existing sidecar style.
- Treat parser, filesystem, permission, and CSV encoding errors as sidecar errors.

For CSV specifically:

- Use Go `encoding/csv`.
- `Read` returns `String[][]`.
- `Write` accepts `String[][]`.
- Preserve the public API names and fallibility.
- Preserve row order and cell values as parsed/emitted by `encoding/csv`.
- Configure read behavior deliberately. For raw `Read`/`ReadRows`, allow ragged rows if that is the documented raw CSV behavior; reserve rectangular validation for future matrix/table APIs.
- Create missing parent directories on write only if the interpreted `IO.Csv`/Csv behavior already promises that. If behavior is inconsistent, surface it in the implementation milestone rather than silently changing semantics.

## 9. Blockers deferred explicitly

- **Pdf/Image handle transport deferred:** handles need lifecycle, ownership, invalidation, restart, leak, and close semantics.
- **IO XLSX deferred:** workbook handles have the same lifecycle problem as Image/Pdf.
- **Record transport deferred:** Markdown tables, Plot labels/sizes, JSON graph nodes, and table APIs need field-order and type-identity rules.
- **Plot deferred:** `Float[]` is necessary but insufficient for the inspected public Plot APIs; record/scalar-option policy is still needed.
- **Markdown deferred:** `String[][]` may help report block composition, but canonical table helpers use columnar records and row-major tables are explicitly rejected by existing guidance.
- **Structured Json/IO graph transport deferred:** `JsonRawGraph` and `JsonRecovered` need typed records or a graph-specific design, not `Any`.
- **Dynamic graph / `Any` deferred:** avoid untyped transport that could become a JSON swamp.
- **Compiled Complex deferred:** not related to wrapper transport and should remain in compiled numeric/language coverage work.
- **Compiled Einstein notation deferred:** not related to wrapper transport.
- **Broad generated-Go numeric/type bug hardening deferred:** do not combine language-codegen hardening with transport expansion.
- **Package-manager sidecar builds, native permission prompts, and lockfiles deferred:** deployment and security workflows are separate from adding one typed value lane.
- **UI live bridge deferred:** belongs to the UI/reactor project, not Octxiliary wrapper transport.

## 10. Tests required for M13

If M13 implements `String[][]` + CSV, require:

1. **Protocol round-trip tests**
   - Encode/parse `String[][]` request args.
   - Encode/parse `String[][]` response values.
   - Empty outer array.
   - Empty row.
   - Ragged rows if allowed.
   - Strings requiring quotes/escapes.
2. **Compiler generic wrapper fixture**
   - Manifest declares `Args: ["String[][]"]` and `Return: "String[][]"`.
   - Compiled runner constructs, sends, receives, and compares nested rows.
   - Missing sidecar diagnostic remains fallible and readable.
3. **CSV sidecar tests**
   - Read simple CSV.
   - Write simple CSV.
   - Escaped comma/quote/newline cell round trip.
   - Empty file or empty rows behavior, matching documented public semantics.
   - Ragged row behavior, matching M13 decision.
4. **Compiled CSV focused tests**
   - `Csv.Read` compiled with `octxiliary-csv` available.
   - `Csv.Write` compiled with `octxiliary-csv` available.
   - IO CSV aliases if they are migrated in M13.
5. **Regression tests**
   - Hash, Compression, Time, Text, Archive, Json wrapper tests continue to pass.
   - Existing `internal/octxiliary`, `internal/pkgmgr`, and `internal/project` tests continue to pass.

## 11. Risk map

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Protocol parser complexity grows brittle | Bad payloads may parse incorrectly or regress existing kinds | Add isolated parser tests for every new matrix edge case and existing kinds; keep `String[][]` parser narrow |
| Nested arrays become accidental dynamic transport | Future work may smuggle arbitrary structures through nested containers | Name and validate exactly `String[][]` in M13; reject general `T[][]` until designed |
| Records are pulled in implicitly | Markdown/Plot/JSON pressure may expand scope | Explicitly defer records and keep CSV row-major only |
| Handles are modeled as plain `Int` | Leaks and cross-family invalid handles become unavoidable | Do not migrate Image/Pdf/XLSX in M13; design lifecycle first |
| CSV ragged semantics surprise users | Table consumers may expect rectangular data | Document raw `Read` as row-preserving; reserve rectangular validation for future `ReadTable`/`ReadMatrix` |
| Public API differs from transport assumption | Implementation could migrate APIs that do not exist or are documented only | Before M13 coding, inspect Oct files and tests again; migrate only present public APIs or intentionally add missing stubs in a language/API milestone |
| `goSafeName`/result naming mismatch | Generated Go compile failures | Add compile fixture for `String[][]` fallible return and inspect generated type name |
| Manifest availability gap | CSV currently lacks a manifest and IO has no wrapper metadata | Include only the minimal wrapper metadata needed for selected CSV package; do not add sidecar build orchestration |

## 12. Final recommendation

**Exactly one recommended next implementation milestone:** **M13 — add `String[][]` transport and migrate CSV row-major read/write wrappers.**

`String[][]` is the smallest correct post-M6 expansion because it is a typed, bounded extension of `String[]` that directly unlocks `Csv.Read`/`Csv.Write` and the current `IO.Csv` row-major surfaces without records, handles, dynamic `Any`, or standard-library API redesign. It gives Octxiliary one carefully specified structured payload lane while preserving the architectural boundary: Go implements the transport and sidecars; Oct packages express typed wrapper contracts.

**M13 non-goals:** no records, no handles, no dynamic graph/`Any`, no Plot migration, no Markdown migration unless separately scoped as direct helper lowering, no Pdf/Image/XLSX migration, no compiled Complex or Einstein notation work, no broad generated-Go hardening, no package-manager sidecar builds, no native permission prompts, no lockfiles, and no public standard-library API redesign.
