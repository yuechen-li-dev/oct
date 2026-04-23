# JSON Intent Recovery Lab — M0 Report

## 1) Corpus overview

1. `example_01_people.json`: table-like repeated person records with a stable key set.
2. `example_02_config.json`: nested service configuration object with logical sub-records.
3. `example_03_dispatch.json`: dispatch/mapping data represented as JSON objects keyed by event name.
4. `example_04_matrix.json`: rectangular numeric arrays that behave like a matrix/grid.
5. `example_05_optional.json`: ticket records with null/missing fields.
6. `example_06_ui_like.json`: UI-like nested page structure with typed sections and repeated cards.
7. `example_07_tagged.json`: tagged records (`type`) representing different operation families.

## 2) Observed structure patterns

Across the corpus we repeatedly see:

- Homogeneous object arrays (`people`, `tickets`, product cards)
- Nested records (`service.http`, `service.features`, `page.sections[*].content`)
- Key/value mappings (`event_handlers`, `retry_policy`)
- Rectangular numeric arrays (`readings` as 3 x 4)
- Optional/null-heavy fields (`assignee`, `due_date`, `sprint`)
- Tagged records (`type: credit|debit|transfer`)

## 3) Recovery transformations

### Homogeneous object arrays
- Source JSON shape: array of objects with mostly identical keys.
- Recovered Oct shape: array of nominal records (e.g., `Person[]`, `Ticket[]`).
- Reasoning: intent is row-oriented table data, not ad hoc dynamic objects.

### Nested config records
- Source JSON shape: deep object with grouped settings.
- Recovered Oct shape: nested records (`ServiceConfig` with `HttpConfig`, `FeatureFlags`, `ServiceLimits`).
- Reasoning: config domains are naturally compositional record types.

### Key/value mapping objects
- Source JSON shape: object map from event key to handler/retry value.
- Recovered Oct shape: normalized table (`HandlerRoute[]`) with explicit columns.
- Reasoning: mapping tables become easier to validate/join/filter when explicit as rows.

### Rectangular numeric arrays
- Source JSON shape: `Float[][]` with consistent row length.
- Recovered Oct shape: explicit grid record containing dimensions + rectangular readings.
- Reasoning: JSON arrays are carrying matrix intent, not arbitrary nested containers.

### Optional/null-heavy objects
- Source JSON shape: missing or null keys.
- Recovered Oct shape: consistent record shape with explicit presence flags (`HasDueDate`, etc.).
- Reasoning: keeps static structure without introducing `Dynamic`.

### Tagged objects
- Source JSON shape: single list mixed by discriminator (`type`).
- Recovered Oct shape: split into typed arrays (`Credits`, `Debits`, `Transfers`).
- Reasoning: discriminator is effectively an encoded sum type; split restores concrete shapes.

## 4) Tables wearing a fake mustache

- `example_01_people`: plain table (rows with shared columns).
- `example_03_dispatch`: dispatch table + retry table encoded as JSON key maps.
- `example_04_matrix`: placement table (row/column coordinates) encoded as arrays.
- `example_06_ui_like`: placement/layout tables (`slot`, `kind`) plus product table.
- `example_07_tagged`: union-like ledger table disguised as one mixed object array.

JSON obscures these by overusing object literals and key-value bags instead of explicit typed rows.

## 5) Safe automatic recovery candidates

Likely safe with deterministic structural rules:

1. Homogeneous object arrays -> array of records.
2. Stable identical key sets across rows -> one nominal row record.
3. Rectangular numeric nested arrays -> matrix/grid candidate.
4. Flat mapping objects (`string -> string|number|bool`) -> mapping table rows `{Key, Value}`.
5. Deep config objects with stable children -> nested record decomposition.

## 6) Ambiguity / unsafe inference

- Inconsistent key sets in one array:
  - Status: **requires caution**.
- Null-heavy sparse objects where null meaning is unclear (unknown vs empty vs not-applicable):
  - Status: **requires schema hint (future)**.
- Mixed-type arrays or weakly tagged objects:
  - Status: **should not auto-recover**.
- Tagged arrays where discriminator values are unstable/user-defined:
  - Status: **requires caution**.

## 7) “Stupid code” feasibility

A significant portion of common JSON intent recovery appears feasible with simple deterministic rules:

- Detect homogeneous arrays by key-set and primitive type consistency.
- Detect rectangular numeric arrays by row-length equality and numeric element checks.
- Detect mapping tables by object values constrained to primitive scalar types.
- Detect nested config records by stable object structure depth.

This should cover a meaningful chunk of practical payloads before any advanced inference.

## 8) Conclusions

- Much real-world JSON in this corpus is structured data in disguise (tables, mappings, nested records, layouts).
- Intent recovery into native Oct shapes is viable without `Dynamic` for many cases.
- `Dynamic` is not required to represent core intent; explicit record/array structures recover most value.
- M1 should focus on a deterministic recovery rule catalog + confidence levels + “do not auto-recover” gates for ambiguous structures.

## Noted consistency check against Language/reference

This experiment intentionally used records and arrays aligned with `Language/reference` as the authoritative model. No `.octagon` parsing/import behavior was implemented here; this is corpus + manual recovery only.
