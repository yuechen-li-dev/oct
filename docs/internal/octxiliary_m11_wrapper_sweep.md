# Octxiliary M11 Wrapper Sweep

M11 swept the remaining wrapper-style standard-library candidates against the M6 generic Octxiliary transport set: `Void`, `Int`, `Float`, `Bool`, `String`, `String[]`, and `Bytes`.

## Results

| Package | Functions attempted | Migrated? | Sidecar | Reason if blocked | Missing concept | Suggested future milestone |
| --- | --- | --- | --- | --- | --- | --- |
| `Libraries/Archive` | `ListEntries(String) -> String[] ! Error`, `ExtractAll(String, String) -> Int ! Error`, `CreateFromFiles(String, String[]) -> Int ! Error` | Yes | `cmd/octxiliary-archive` | Fits M6 scalar/list transports and Go `archive/zip` without persistent handles. | none | M11 complete |
| `Libraries/Json` | `Save(String, String) -> Int ! Error`, `Load(String) -> String ! Error`, `Object(String) -> String` | Partial safe migration | `cmd/octxiliary-json` | File helpers fit M6. `Object` remains direct Oct because it is pure identity and does not need a sidecar. Broader structured JSON helpers live under `Libraries/IO` and require record/dynamic graph transport. | `needs_record_transport` / `needs_nested_array_transport` for structured JSON graph helpers | M12 structured-data transport design |
| `Libraries/Csv` | `Write(String, String[][]) -> Int ! Error`, `Read(String) -> String[][] ! Error` | No | none | Public API is row-major `String[][]`, which is outside M6 (`String[]` only; no nested arrays). | `needs_nested_array_transport` | M12 structured-data transport design |
| `Libraries/Markdown` | Markdown block helpers, table helpers, report helpers | No | none | Many scalar/list helpers could be direct compiled helpers, but table/report composition uses `String[][]` block lists and columnar record-of-`String[]` values; generic wrapper lowering cannot transport these shapes. | `needs_record_transport` / `needs_nested_array_transport` | M12 direct helper or structured transport pass |
| `Libraries/Pdf` | Page creation, draw text/image, save | No | none | Public API exposes `PdfPage` and `ImageHandle` records around stateful handles, plus `TextStyle` record arguments. | `needs_handle_transport` / `needs_record_transport` | Handle-transport PDF/Image milestone |
| `Libraries/Plot` | `Line`, `Scatter`, `Histogram` | No | none | Public API requires `Float[]` data plus `Size` and `Labels` records. M6 has scalar `Float`, not `Float[]`, and no record transport. | `needs_float_array_transport` / `needs_record_transport` | Numeric-array wrapper transport milestone |
| `Libraries/Image` | `Load`, `Save`, `Width`, `Height`, `Format` | No | none | Public API centers on an `ImageHandle` record/opaque identity. | `needs_handle_transport` | Handle-transport PDF/Image milestone |
| `Libraries/Hash` | Existing hash helpers | Already migrated | `cmd/octxiliary-hash` | Verified as prior M7 coverage; not redone. | none | complete |
| `Libraries/Compression` | Existing gzip helpers | Already migrated | `cmd/octxiliary-compression` | Verified as prior M8 coverage; not redone. | none | complete |
| `Libraries/Time` | Existing time helpers | Already migrated | `cmd/octxiliary-time` | Verified as prior M9 coverage; not redone. | none | complete |
| `Libraries/Text` | Existing regex helpers | Already migrated | `cmd/octxiliary-text` | Verified as prior M10 coverage; not redone. | none | complete |

## Sidecars added

- `octxiliary-archive`: generic family `Archive`, functions `ZipListEntries`, `ZipExtractAll`, and `ZipCreateFromFiles`.
- `octxiliary-json`: generic family `Json`, functions `JsonSave` and `JsonLoad`.

## M6 transport coverage used

- `Archive.ListEntries`: `String -> String[]`.
- `Archive.ExtractAll`: `String, String -> Int`.
- `Archive.CreateFromFiles`: `String, String[] -> Int`.
- `Json.Load`: `String -> String`.
- `Json.Save`: `String, String -> Int`.

## Remaining wrapper backlog

- Nested-array row/table data: `Libraries/Csv`, Markdown report block composition, IO CSV surfaces.
- Record/dynamic structured data: Markdown table records and IO structured JSON graph recovery.
- Opaque handles: `Libraries/Image` and `Libraries/Pdf`.
- Numeric arrays: `Libraries/Plot` `Float[]` plot inputs.

## M13 update

`Csv` is no longer blocked for raw row-major workflows: M13 adds the narrow `String[][]` transport and migrates `Libraries/Csv.Read`/`Write` to `octxiliary-csv`. IO row-major CSV aliases have focused compiled coverage. Structured CSV table/matrix helpers and all record/handle/numeric-array transports remain out of scope.
