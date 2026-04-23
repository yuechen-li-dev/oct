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
- `ReadBytes(path) -> Int[] ! Error`
- `WriteBytes(path, data) -> Int ! Error`
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

### IO.Json

- `NormalizeJson(text) -> String ! Error`
- `Parse(text) -> String ! Error`
- `Stringify(value) -> String ! Error`
- `Load(path) -> String ! Error`
- `Save(path, value) -> Int ! Error`

### IO.Csv

- `Read(path) -> String[][] ! Error`
- `Write(path, rows) -> Int ! Error`

## Common failure cases

- missing path
- invalid json
- invalid csv
- invalid wrapper argument shape

All wrapper errors use standardized wrapper error kinds via the Mx103a substrate.

## Note on current type surface

The language reference does not currently define `Dynamic` or `Bytes` primitives; this wrapper wave uses `String` and `Int[]` contracts for JSON and bytes payloads.
