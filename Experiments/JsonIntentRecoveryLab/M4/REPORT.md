# JSON Intent Recovery Lab — M4 Report

## 1) Structural reuse audit

### Existing structural node/value surfaces inspected

- `.octagon` parser accepts scalar literals (`Int`, `Float`, `Bool`, `String`), arrays, and record literals, validated by `parse.BuildDataValue(...)` + `validateDataValue(...)` in `internal/parse/data.go`.
- Runtime materialization for `.octagon` values already maps those AST nodes into runtime values (`ValueArray`, `ValueRecord`, scalar `Value*`) via `materializeOctagonValue(...)` in `internal/interpret/octagon_load.go`.
- `.octagon` loading (`internal/octagon/octagon.go`) already provides a thin parse boundary into AST expressions.

### Reuse achieved in M4

M4 reuses the existing Oct/.octagon structural world directly:

- Reused scalar node/value forms: string/number/bool.
- Reused container forms: arrays and records.
- Reused runtime materialization path: lowered JSON is converted to AST record/array/scalar nodes, then materialized through existing typed-record machinery.

### Gaps found

1. JSON object keys are quoted strings and can include characters not valid as Oct field identifiers (e.g., `kebab-key`).
2. JSON has `null`; current Oct language surface does not define general native null semantics in `Language/reference`.

### Thin compatibility layer sufficiency

Yes, a thin backend bridge is enough for this milestone:

- backend parses JSON and lowers to compatibility records/arrays/scalars,
- Oct recovery policy remains in Oct,
- no separate JSON semantic runtime is introduced.

### Null addition decision

A dedicated compatibility sentinel is used:

- `JsonRawGraph { Kind: "null", IsNull: true, ... }`.

This is explicitly compatibility-scoped and does **not** add general native null semantics.

---

## 2) Prototype raw-lowering design

### Backend boundary

Added two builtins:

- `JsonLower<JsonRawGraph>(text: String) -> JsonRawGraph ! Error`
- `JsonLoadStructured<JsonRawGraph>(path: String) -> JsonRawGraph ! Error`

Implementation details:

1. Parse JSON with Go `encoding/json` (`UseNumber` enabled).
2. Lower parsed values into AST records/arrays/scalars using these compatibility records:
   - `JsonRawGraph`
   - `JsonRawGraphNode`
3. Materialize through existing typed value bridge (`materializeOctagonValue`) rather than inventing a parallel runtime.

### Compatibility node family

`JsonRawGraphNode` rows are kind-tagged:

- `object`
- `array`
- `string` (`StringValue`)
- `number` (`NumberValue`)
- `bool` (`BoolValue`)
- `null` (`IsNull: true`)

Parent/child structure is expressed by `ParentId`; object keys and array indexes are carried in `Key`.

Object keys map to `JsonRawGraphNode.Key: String`, which cleanly handles quoted/non-identifier keys.

---

## 3) Null handling decision

JSON `null` lowers to a compatibility node only:

- `Kind == "null"`
- `IsNull == true`

The rest of the language remains unchanged:

- no new `null` literal in Oct syntax,
- no nullable type-system feature,
- no Dynamic introduction.

---

## 4) Oct-side recovery comparison (M3 vs M4)

## M3

- Raw import returned compact JSON string.
- Recovery predicates were key-pattern string checks (`Contains(raw, "...")`).

## M4

- Raw import now also yields `JsonRawGraph` structured compatibility data.
- Recovery predicates inspect structure (graph-structural predicates over `ParentId`/`Key` links).
- `when utility` arbitration remains Oct-side and deterministic.

## Effect

- More robust than textual matching.
- Recovery policy reads as shape inspection rather than substring heuristics.
- Better deterministic behavior for non-identifier keys and explicit nulls.

---

## 5) Corpus impact

Re-evaluated M3 corpus paths through structured raw lowering:

1. `people` table: still `table.columnar`, now shape-checked as object→array→object.
2. `dispatch/mapping`: still `mapping.table`, now key-presence checked structurally.
3. `matrix/grid`: still `grid.nested_array`, now verifies `sensor_grid.readings` as array shape + numeric row/col nodes.
4. `sparse/optional tickets`: still `table.columnar`; explicit `null` ticket fields are now visible as compatibility null nodes.
5. `UI-like structured`: still `config.nested_record`, now driven by object field structure.
6. `tagged operations`: still `tagged.decomposed`, now verifies first operation object contains `type` key structurally.
7. `config object`: still `config.nested_record` with object-shape checks.

Result: classification outcomes are preserved, while predicates are structurally honest and less brittle.

---

## 6) Architectural conclusion

For the bounded M4 question, reuse is viable:

- existing Oct/.octagon structural representation is sufficient as raw compatibility substrate,
- thin backend parse+lower bridge is workable,
- Oct-side recovery policy remains cleanly in Oct.

A separate JSON semantic universe is **not** required for this stage.

---

## 7) Deferred boundaries

Still out of scope after M4:

- full JSON importer into arbitrary user-defined domain records,
- automatic schema inference / typed recovery synthesis,
- YAML/TOML/TOON generalization,
- any `Dynamic` design,
- introducing native language-level null semantics.

---

## Language/reference consistency notes

- `when utility` usage aligns with `Language/reference/runtime/21-octomata.md` expression-form guidance.
- M4 introduces compatibility `Kind == "null"` node handling in library code, while `Language/reference` does not define a general Oct null value. This is intentional and documented as a compatibility-only sentinel boundary.
