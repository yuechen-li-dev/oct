# IO

## Purpose

`IO` is the family package for thin, practical I/O wrappers over stable Go libraries.

## Current surface

### IO.Xlsx

- `CreateWorkbook()`
- `AddSheet(workbook, name)`
- `SetCellString(workbook, sheet, cell, value)`
- `SetCellFloat(workbook, sheet, cell, value)`
- `SaveWorkbook(workbook, path)`

### IO.File

- `ReadText(path) -> String ! Error`
- `WriteText(path, text) -> Int ! Error`
- `ReadLines(path) -> String[] ! Error`
- `WriteLines(path, lines: String[]) -> Int ! Error`
- `ReadBytes(path) -> Bytes ! Error`
- `WriteBytes(path, data: Bytes) -> Int ! Error`
- `Exists(path) -> Bool`
- `Delete(path) -> Int ! Error`

### IO.Path

- `JoinPath(parts) -> String`
- `BaseName(path) -> String`
- `Extension(path) -> String`
- `Stem(path) -> String`
- `Parent(path) -> String`
- `Clean(path) -> String`

### IO.Directory

- `List(path) -> String[] ! Error`
- `Make(path) -> Int ! Error`
- `MakeAll(path) -> Int ! Error`
- `RemoveAll(path) -> Int ! Error`

### IO.Csv

- `Read(path) -> String[][] ! Error`
- `ReadRows(path) -> String[][] ! Error` (explicit raw row-major import; alias of `Read`)
- `ReadTable(path) -> record of String[] columns ! Error` (header row required; values remain String for M0)
- `ReadMatrix(path) -> Float[][] ! Error` (no header inference; rectangular numeric grid only)
- `Write(path, rows) -> Int ! Error`
- `WriteRows(path, rows: String[][]) -> Int ! Error` (explicit row-major export; alias of `Write`)
- `WriteMatrix(path, matrix: Float[][]) -> Int ! Error`
- `WriteTable(path, table) -> Int ! Error` (**not implemented in M0**; explicit placeholder)

#### Deterministic text/line semantics

- `WriteText` overwrites existing files and creates missing parent directories.
- `ReadText` returns exact file content (including newlines) and fails on missing files.
- `WriteLines` writes lines joined by `\n` and appends one trailing `\n` when `lines` is non-empty.
- `ReadLines` splits on `\n`, preserving empty lines; terminal newline does not create an extra trailing empty element.

#### Deterministic CSV semantics

- CSV storage remains physically row-major; import intent is ambiguous unless you choose an explicit read API.
- Prefer `ReadTable` for named spreadsheet-like tables and `ReadMatrix` for raw numeric grids.
- Use `ReadRows` when you need lossless string rows from the CSV parser.
- `Write`/`WriteRows` use standard CSV escaping for commas, quotes, and embedded newlines.
- `Write` creates missing parent directories.
- Row order is preserved exactly as provided.

## Common failure cases

- missing path
- invalid csv
- invalid wrapper argument shape

All wrapper errors use standardized wrapper error kinds via the Mx103a substrate.

## Note on type surface

`Bytes` is available as a narrow binary boundary type for wrapper compatibility surfaces (file payloads now, additional transport wrappers later). It is intentionally not a dynamic catch-all and does not introduce `Dynamic`.


## JSON

`IO` has no JSON functions. JSON is read and written as declared types by
the `Json` builtins (`Json.Load<T>`, `Json.Parse<T>`, `Json.Save`,
`Json.Text`): see [`Libraries/Json`](../Json/README.md). The JSON files under
`testdata/` are sample documents. No test reads them now; they are kept for
`oct json infer`, the last milestone of `internal/json/JSON_V2_LADDER.md`.
