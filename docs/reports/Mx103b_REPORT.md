# Mx103b Report — First-Wave Core IO Wrappers

## 1) Modules implemented

Implemented the first-wave wrapper modules on top of the Mx103a substrate:

- `IO.File`
- `IO.Path`
- `IO.Directory`
- `IO.Json` (expanded)
- `IO.Csv`

## 2) How Mx103a substrate was used

All wrapper runtime handlers are implemented via the shared substrate:

- invocation plumbing via `newWrapperCall(...)`
- arity checks via `expectArity(...)`
- argument decoding via `stringArg(...)`, `intArg(...)`, and `evalArg(...)`
- result lifting via shared helpers (`wrapperIntResult`, `wrapperStringResult`, plus shared array/bool helpers added in substrate)
- failure mapping through `wrapperErrorf(...)` + `wrapperErrorResult(...)`

## 3) Confirmation on ad hoc logic

No wrapper introduced bespoke per-module error envelope formatting; all wrapper-visible failures route through the standard wrapper error surface.

Argument decoding that is not covered by scalar helpers (e.g., `String[]`, `String[][]`, byte-array-as-`Int[]`) is still executed inside wrapper handlers, but mapped through standard wrapper error kinds and wrapper error result projection.

## 4) API surface rationale (included / excluded)

Included:

- minimal high-ROI file, path, directory, JSON, and CSV wrappers
- deterministic JSON compaction behavior where practical
- predictable CSV table shape (`String[][]`)

Excluded deliberately:

- advanced fs options (permissions knobs, symlink APIs, metadata APIs)
- advanced JSON/CSV options (streaming, custom delimiters, schema helpers)
- non-core IO families (HTTP, archive/compression, DB/image)

## 5) Test coverage summary

Added Oct facts covering:

- happy paths for file, path, directory, json, csv
- error behavior for missing files/dirs and invalid json/csv
- roundtrips for json save/load and csv write/read

Also wired a command-level Go test that verifies new fact pass markers are present for Mx103b.

## 6) What remains for Mx103c

- expand wrapper argument/result helpers for nested/list data to reduce repeated list decoding
- introduce richer JSON value support when/if reference-level type support exists
- optional CSV features (delimiter/header helpers) if requested by user demand

## Explicit inconsistency notes

`Language/reference/language/02-types.md` does not currently define `Dynamic` or `Bytes` primitive types. Mx103b therefore uses `String` and `Int[]` contracts for JSON/bytes payloads in this wave. This is a deliberate scope-compatible fallback and should be reconciled in a future language-reference/API alignment pass.

`Language/reference/language/05-functions.md` does not define variadic function parameters, so `IO.Path.JoinPath` is currently exposed as `JoinPath(parts: String[])` rather than `Join(a, b, ...rest)`.
