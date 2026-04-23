# JSON Intent Recovery Lab — M1 Report

## 1) Why M0 did not go far enough

M0 made a useful first move: it translated ad hoc JSON objects and arrays into typed Oct records/arrays. But many outputs stopped at a halfway shape:

- zero-argument functions,
- returning literal data,
- with no input dependence,
- no transformation,
- and therefore no true logic.

That means M0 often recovered **data shape** but not **code shape**. In practice, several M0 outputs are authored corpora/tables disguised as procedures.

### Explicit M0 early-stop audit (required)

| Example | M0 shape | Stopped too early? | Why |
|---|---|---:|---|
| `example_01_people` | `fn RecoveredPeople() -> Person[]` | Yes | Static person rows; function wrapper is fake logic. |
| `example_02_config` | `fn RecoveredConfig() -> ServiceConfig` | Mostly no | Primarily compositional nested config record; table pressure is low. |
| `example_03_dispatch` | `fn RecoveredDispatchTable() -> HandlerRoute[]` | Yes | Two distinct maps (`event_handlers`, `retry_policy`) were merged into one synthetic row table. |
| `example_04_matrix` | `fn RecoveredSensorGrid() -> SensorGrid` | Yes | Matrix/grid intent exists; M0 kept nested arrays but did not expose tabular placement/value structure. |
| `example_05_optional` | `fn RecoveredTickets() -> Ticket[]` | Yes | Static ticket corpus with optional columns; function wrapper is fake logic. |
| `example_06_ui_like` | `fn RecoveredHomePageLayout() -> HomePageLayout` | Yes | Mixed layout/catalog/mapping data remains under one nested record and procedural wrapper. |
| `example_07_tagged` | `fn RecoveredOperations() -> LedgerOperations` | Partial | Good discriminator split, but still literal static corpus wrapped as function. |

## 2) Storefront proof/reference

Storefront M0→M7 shows a clear normalization trajectory:

- table-shaped truth should be represented as tables,
- layout/placement is data and benefits from explicit placement tables,
- simple exact-key transitions should be dispatch tables,
- pure label mappings should be mapping tables,
- static authored facts should live in data sections while behavior remains procedural.

This is explicit in the Storefront short paper findings and canonical source structure (`Data`, `Placement`, `Dispatch`, `Behavior`, `Composition`, `Surface`). JSON M1 recovery should mirror that same decomposition pressure.

## 3) Record vs table vs mapping criteria (“record or badly misshapen table?”)

### A. True record indicators

Recover as nested records when most of these hold:

1. fields are semantic parts (not row columns),
2. sub-objects represent compositional domains,
3. low cardinality / not repeated homogeneous rows,
4. relationships are containment, not key-index joins.

Typical: service configuration (`http`, `features`, `limits`).

### B. Table-in-disguise indicators

Recover as row/column tables when most of these hold:

1. repeated homogeneous entries,
2. stable key sets across entries,
3. row-oriented operations are plausible (filter/sort/join/lookup),
4. key names behave like columns.

Typical: people/tickets/product catalogs.

### C. Mapping-table indicators

Recover as key/value or dispatch tables when:

1. JSON object keys carry domain entities (event/category/code),
2. values are scalar/simple payloads,
3. behavior is exact-key resolution.

Typical: `event_handlers`, retry policies, labels.

### D. Grid/layout-table indicators

Recover as placement/value tables when:

1. arrays are rectangular (row/col regularity),
2. section lists encode `kind` + `slot`,
3. coordinates/slots/indexes are first-class intent.

Typical: sensor matrix, UI section placement.

### E. Mixed-structure indicators

Use mixed decomposition when one payload combines:

- page metadata (record),
- placement rows (table),
- catalogs (table),
- keyed lookups (mapping).

## 4) Code-shape recovery rules (static data vs logic)

1. If output is constant, input-independent, and literal-only: classify as **authored static data**.
2. If shape is row-oriented: classify as **table/corpus declaration target**.
3. If shape is exact-key resolution facts: classify as **mapping/dispatch declaration target**.
4. Use functions only when there is true behavior:
   - input-dependent transformation,
   - derived computation,
   - validation/state transition/composition logic.
5. If current Oct surface requires function wrappers for top-level materialization, mark wrapper as **representation artifact**, not semantic logic.

## 5) Per-example M1 findings

| Example | Original JSON shape | M0 recovery shape | M1 recovery shape | Classification | Ambiguity notes |
|---|---|---|---|---|---|
| `example_01_people` | `people[]` object rows + nested skills | `Person[]` from zero-arg function | `PeopleTable` + `PersonSkillTable` inside `PeopleCorpus` | Table + mixed nested column | `skills` may be normalized further to junction rows if needed. |
| `example_02_config` | nested `service` object | nested records from zero-arg function | unchanged core nested record (`ServiceConfig`) | Record | Strongly compositional; table normalization would harm semantics. |
| `example_03_dispatch` | two keyed maps | merged `HandlerRoute[]` | split `StringDispatchTable` + `RetryPolicyTable` | Mapping tables | M0 conflated two maps into synthetic merged rows. |
| `example_04_matrix` | rectangular numeric arrays with row/col metadata | `SensorGrid` with `Float[][]` | flattened value table + explicit row/col placement table | Grid/layout table | Dual representation retained to preserve intent clarity. |
| `example_05_optional` | ticket rows with null/missing fields | row records with presence flags | base `TicketTable` + separate optional-column table | Table (sparse columns) | Null semantics still ambiguous (unknown vs absent vs N/A). |
| `example_06_ui_like` | page metadata + sections(`kind`,`slot`) + section content | nested layout record with embedded cards | `HomePageCorpus` split into meta, section placement, hero, product catalog/grid, banner | Mixed (record + layout + catalog table) | Strong Storefront-like decomposition opportunity. |
| `example_07_tagged` | tagged operation rows | split arrays per variant | per-variant column tables in `LedgerCorpus` | Mixed/tagged tables | Could also be one sparse super-table; split is safer deterministic shape. |

## 6) Revised recovered forms (M1 artifacts)

Revised forms are provided under `M1/recovery/` and push normalization/tabulation one step beyond M0 while preserving true record structure where appropriate.

## 7) Safe deterministic recovery candidates

Likely safe with deterministic rules from this corpus:

1. homogeneous object arrays -> row table,
2. repeated nullable columns -> base table + optional-column table,
3. flat scalar object maps -> key/value table,
4. rectangular arrays -> grid table + coordinate/placement table,
5. section lists with stable `kind`/`slot` keys -> placement table,
6. tagged arrays with stable discriminator vocabulary -> per-tag subtables,
7. deeply compositional singleton objects -> nested records.

## 8) Ambiguity boundaries

Unsafe to decide automatically without schema/hints:

1. meaning of `null` (missing vs unknown vs intentionally empty),
2. unstable discriminators (free-form user strings),
3. small arrays that could be either ordered tuples or row sets,
4. mixed-key arrays where partial sparsity may indicate evolving schemas,
5. map objects where key order/priority semantics are external.

## 9) Deterministic “badly misshapen table” decision rule set (synthesis)

Use this gate sequence:

1. **Repetition gate**: same object key set appears in >=2 elements? -> candidate table.
2. **Column stability gate**: primitive/array value types stable per key? -> stronger table confidence.
3. **Key-as-entity gate**: object keys are dynamic domain tokens (events/codes)? -> mapping table.
4. **Rectangularity gate**: nested arrays equal row lengths? -> grid/placement table.
5. **Compositional gate**: object children are named subsystems (http/features/limits) and not repeated rows? -> true record.
6. **Execution gate**: no input dependence + literal-only construction? -> static authored data, not logic.

## 10) Language/reference consistency note

This milestone follows `Language/reference` for current Oct constructs (records, arrays, functions). A tension is visible: M1 prefers declaration-first static authored data, but current examples still use function materialization to carry literal corpus values. This is surfaced intentionally as a code-shape gap between desired recovery form and currently exercised surface constructs.

