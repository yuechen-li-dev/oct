# JSON Intent Recovery Lab — M3 Report

## Purpose

M3 validates whether the M1/M2 synthesis can be expressed as deterministic, executable Oct code over the existing wrapper-level JSON compatibility surface.

## Mapping: lab synthesis → implementation targets

### M1 decision rules implemented directly

1. **Table detection**: homogeneous object arrays are detected as table-shaped and recovered to table-style canonical forms (`table.columnar`).
2. **Mapping detection**: object maps (dispatch/retry maps) are detected and recovered as key/value mapping tables (`mapping.table`).
3. **Grid detection**: rectangular numeric arrays under `readings` with explicit `rows/cols` metadata are recovered as grid (`grid.nested_array`).
4. **Code-shape recovery guard**: nested config/UI composition is recovered as nested records (`config.nested_record`) instead of forcing table decomposition.
5. **Tagged decomposition**: tagged operation arrays are recognized and recovered as deterministic per-tag decomposition (`tagged.decomposed`).

### M2 representation rules implemented directly

1. **Canonical default for table-shaped data** is columnar (`table.columnar` with column lists in canonical summary).
2. **Grid exception** remains nested arrays (`grid.nested_array` canonical summary includes `[[Float]]`).
3. **Rows representation** is not the default in this prototype; row-first view is deferred.

### Raw JSON representation consumed by Oct code

- `ImportRawJson(path)` uses wrapper-backed `JsonLoad` and returns a **compact JSON `String`**.
- Recovery operates over that compact string using deterministic shape predicates and policy arbitration (`when utility`).

### Deferred due to current raw/language surface limits

1. No structured JSON AST/type is exposed to Oct yet (raw compatibility remains string payload).
2. Full schema-level inference (arbitrary nested shape extraction) is deferred.
3. Sparse optional typing beyond corpus-specific table summaries is deferred.
4. Canonical materialization into fully typed native table structs is deferred; M3 emits deterministic canonical summaries.

## Prototype surfaces

Implemented in `Libraries/IO/IO.Json.oct`:

- `ImportRawJson(path: String) -> String ! Error`
- `ImportJson(path: String) -> JsonRecovered ! Error`
- `RecoverJsonIntent(raw: String) -> JsonRecovered`

`ImportJson` pipeline: raw import → deterministic policy classification (`when utility`) → canonical representation summary.

## `when utility` policy usage

`RecoverJsonIntent` uses bounded `when utility` arbitration over explicit candidates:

- `table.columnar`
- `mapping.table`
- `grid.nested_array`
- `tagged.decomposed`
- `config.nested_record`
- fallback `record.raw`

Scores encode bounded policy priorities discovered in M1/M2 (table/mapping/grid/tagged/config ordering), while predicates remain explicit and deterministic.

## Corpus validation outcomes

Validated via `Libraries/IO/IO.Json.octest` against M0 corpus:

- ✅ people table → `table.columnar`
- ✅ dispatch/mapping → `mapping.table`
- ✅ matrix/grid → `grid.nested_array`
- ✅ optional/sparse tickets → `table.columnar`
- ✅ UI-like structured case → `config.nested_record`
- ✅ tagged operations → `tagged.decomposed`
- ✅ config object → `config.nested_record`
- ✅ determinism check (same input twice) preserved

## Partial recovery boundaries

1. Recovery predicates are deterministic but currently **key-pattern based** over compact JSON strings.
2. Canonical output is a deterministic **summary string**, not a first-class typed table/grid value.
3. Tagged decomposition validates stable tags by known-shape detection, not generic tag-key discovery.

## Ambiguity boundaries that remain

1. Overlapping candidate shapes outside the corpus may require richer policy metadata.
2. Generic homogeneous-array detection cannot be robust without structured raw JSON access.
3. Optional/sparse typing policy (null vs absent distinction at schema level) needs richer raw representation.

## Implication for future non-experimental `ImportJson(...)`

M3 demonstrates that deterministic recovery policy can live in Oct code and be expressed with inspectable `when utility` arbitration. For productionization, the main missing capability is a richer raw representation (typed JSON node surface) so that Oct-level policy can reason structurally without brittle string-shape predicates.

## Noted consistency gap vs ideal M1/M2 end-state

M1/M2 conclusions imply structural recovery over JSON shape; current implementation is constrained by string-only raw compatibility exposure. This is an explicit documentation/implementation gap and should be closed by exposing structured raw JSON nodes while preserving Oct-level policy ownership.
