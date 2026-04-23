# Mx104 REPORT — Productionizing `IO.Json` Intent Recovery

## 1) Required audit and promotion mapping

### Current production `IO.Json` surface audited

Inspected current production files:

- `Libraries/IO/IO.Json.oct`
- `Libraries/IO/IO.Json.octest`
- `Libraries/IO/README.md`
- `internal/interpret/wrapper_json.go`
- `internal/interpret/json_structured.go`

### Prototype/lab surfaces promoted

Promoted from M3/M4 into production posture:

- two-tier JSON import API split:
  - raw compatibility import (`ImportRawJson`, `ImportRawJsonGraph`)
  - default native intent recovery import (`ImportJson`)
- bounded Oct-side arbitration using `when utility`
- structural predicates over `JsonRawGraph` compatibility nodes
- deterministic canonical recovery family (`table.columnar`, `mapping.table`, `grid.nested_array`, `tagged.decomposed`, `config.nested_record`)
- conservative fallback for ambiguous overlap (`record.raw.ambiguous`)

### Corpus/experiment-specific behavior generalized

Generalized away from corpus-only key checks such as:

- `people`, `tickets`, `event_handlers`, `retry_policy`, `sensor_grid`, `operations`, `service`, `page`

to structural rules such as:

- any root child array of objects for table recovery
- any flat object with simple scalar/null payloads for mapping recovery
- any rectangular numeric array for grid exception
- any stable tagged object-array using a deterministic `type` string key
- nested compositional object/array structures for config recovery

### Raw JSON lowering surface kept

Kept exactly as thin backend compatibility substrate:

- `JsonLower<JsonRawGraph>(text)`
- `JsonLoadStructured<JsonRawGraph>(path)`
- `JsonRawGraph` / `JsonRawGraphNode` with explicit `Kind` and `IsNull`

### Intentionally not productionized in Mx104

Out of scope (explicitly unchanged):

- no `Dynamic`
- no general schema inference engine
- no YAML/TOML/TOON importer
- no type-system redesign
- no backend Go-side intent policy engine
- no native language-level null semantics

## 2) Production JSON surfaces after Mx104

### Raw compatibility path

- `ImportRawJson(path) -> String ! Error`
- `ImportRawJsonGraph(path) -> JsonRawGraph ! Error`
- `LowerJsonToRawGraph(text) -> JsonRawGraph ! Error`

### Intended default path

- `ImportJson(path) -> JsonRecovered ! Error`

Default behavior:

- load compact raw JSON + structured compatibility graph
- run deterministic Oct-side recovery policy
- emit deterministic kind + canonical summary + raw JSON payload

## 3) Backend/Oct split preservation

### Backend (Go) remains thin

- parse JSON via `encoding/json`
- lower to compatibility graph rows/nodes
- expose builtins for structured lowering/import
- preserve deterministic ordering (sorted object keys)

### Oct library owns policy

- all recovery predicates in `Libraries/IO/IO.Json.oct`
- bounded candidate arbitration in `RecoverJsonIntent(...)` via `when utility`
- canonical emitters and conservative ambiguity fallback in Oct code

## 4) Formalized recovery policy (production defaults)

1. Homogeneous object arrays → `table.columnar` (default table canonical).
2. Mapping objects with simple payloads → `mapping.table`.
3. Rectangular numeric arrays → `grid.nested_array` (explicit exception to table default).
4. Nested compositional config objects → `config.nested_record`.
5. Stable tagged arrays (`type` string present across rows) → `tagged.decomposed`.
6. Sparse/optional table data remains in deterministic table path (`table.columnar` canonical posture includes sparse/optional note).

## 5) Ambiguity boundaries and conservative behavior

If multiple recovery families match the same input, classification is intentionally conservative:

- `record.raw.ambiguous`
- canonical guidance points to `ImportRawJson(...)`/`ImportRawJsonGraph(...)` for explicit custom handling

No fuzzy guessing is introduced.

## 6) Null handling statement

JSON `null` remains compatibility-only in raw graph nodes:

- `Kind == "null"`
- `IsNull == true`

Mx104 does not add general native null semantics and does not introduce `Dynamic`.

## 7) Test/documentation progression

Production tests now cover:

- intended default path (`ImportJson`)
- raw compatibility path (`ImportRawJson`/`ImportRawJsonGraph`)
- core classes (table, mapping, grid, nested config, tagged, sparse/optional)
- determinism
- ambiguity fallback
- compatibility-only null handling

Documentation now states explicit production JSON posture in:

- `Libraries/IO/README.md`
- `Language/reference/language/09-builtins.md`

## 8) Consistency notes

- `Language/reference` still states JSON as compatibility-oriented wrappers and `.octagon` as native format; Mx104 aligns with this.
- Existing experiment corpus `example_03_dispatch.json` now classifies as `config.nested_record` under generalized conservative production rules because it is nested object composition rather than a flat mapping object.
  - This is an intentional behavior shift from corpus-specific M3/M4 detection and is kept visible here.
