# IO

## Purpose

`IO` is the family package for thin, practical I/O wrappers.

## IO.Xlsx M0 scope

M0 is write-side only:

- create a workbook
- add sheets
- set string cells
- set float cells
- save `.xlsx` files

## Current surface

- `CreateWorkbook()`
- `AddSheet(workbook, name)`
- `SetCellString(workbook, sheet, cell, value)`
- `SetCellFloat(workbook, sheet, cell, value)`
- `SaveWorkbook(workbook, path)`
- `NormalizeJson(text)`

## IO.Json Mx103a golden wrapper scope

Mx103a adds a tiny JSON wrapper meant to be the canonical pattern for future wrapper waves:

- one builtin bridge (`JsonNormalize`)
- one thin Oct wrapper (`NormalizeJson`)
- deterministic fallible behavior (`String ! Error`)
- compact golden tests for happy/error paths

## Non-goals

This is not a full spreadsheet framework and not a general abstraction over all spreadsheet systems.

M0 does **not** include formulas, styling, charting, pivot tables, workbook reading, or streaming.

## Wrapper note

`IO.Xlsx` demonstrates a thin Oct-shaped wrapper over an external Go XLSX backend, focused on immediate practical output.

Runtime integration now uses the internal Bridge M0 wrapper substrate (maintainer-facing handle store + wrapper builtin dispatch), while keeping the Oct surface unchanged.

Mx103a extends the substrate with shared wrapper-call helpers for argument decoding, result lifting helpers, and standardized wrapper error categories.
