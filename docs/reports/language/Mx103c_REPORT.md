# Mx103c Report — Utility Wrapper Wave (Archive, Compression, Hash, Regex, Time)

## 1) Modules implemented

Implemented wrapper-backed utility modules:

- `Archive.Zip`
- `Compression.Gzip`
- `Hash.Core`
- `Text.Regex`
- `Time.Core`

## 2) Mx103a substrate reuse

All wrapper handlers reuse the existing Mx103a wrapper substrate:

- invocation/arity via `newWrapperCall(...)` + `expectArity(...)`
- argument decode via shared call helpers (`stringArg(...)`, `intArg(...)`, `bytesArg(...)`)
- result lifting via shared helpers (`wrapperBoolResult`, `wrapperStringResult`, `wrapperStringArrayResult`, `wrapperBytesResult`, `wrapperIntResult`)
- standardized error mapping via `wrapperErrorf(...)` + `wrapperErrorResult(...)`
- common registry composition via `newWrapperBuiltinRegistry(...)`

No module reintroduced ad hoc wrapper error envelope behavior.

## 3) Substrate extensions needed

One narrow helper was added to the shared wrapper call substrate:

- `call.stringArrayArg(index)` for consistent `String[]` decoding and element validation

This was used by zip archive creation input handling so wrappers avoid per-handler bespoke list decoding.

## 4) Naming/layout consistency

Layout follows the same convention as existing Oct library files:

- file naming: `<Package>.<Module>.oct`
- package declaration: `package <TopLevelPackage>`
- package directories with manifests: `Libraries/Archive`, `Libraries/Compression`, `Libraries/Hash`, `Libraries/Text`, `Libraries/Time`

Each module API uses consistent verb-oriented names (`ListEntries`, `ExtractAll`, `CompressBytes`, `Sha256Text`, `IsMatch`, `NowIso8601`).

## 5) API rationale (included / excluded)

Included minimal high-ROI surfaces:

- zip listing/extraction (+ simple file-based creation for deterministic local tests)
- gzip byte and file roundtrip helpers
- SHA-256 byte/text/file hashing
- regex match/find/replace/split
- ISO-8601 + unix-seconds time helpers

Excluded deliberately:

- advanced zip authoring options/metadata controls
- non-gzip compression formats
- broad hash algorithm catalogs
- regex engine internals/options
- large time/date framework semantics

## 6) Test coverage summary

Added focused package tests for:

- happy paths across all five modules
- roundtrips for zip/gzip/time formatting sanity
- deterministic hash output known vectors
- regex deterministic behavior
- error paths (missing file/archive, invalid gzip, invalid regex, invalid time)

Also added command-level Go integration coverage that checks pass markers from each new package test root.

## 7) Documentation updates

Added README docs for each new package directory:

- available functions
- backing Go standard library package
- common failure modes

## 8) Remaining for future waves

Potential future wrapper waves may add:

- targeted archive entry extraction/create options
- additional hash algorithms where justified
- optional time parsing/format variants beyond RFC3339

## Explicit inconsistency notes

The user request used `Unit` in several signatures, while `Language/reference/language/02-types.md` defines `Void` (not `Unit`). For side-effecting wrapper helpers, this wave keeps the established Mx103b wrapper style (`Int ! Error`) to preserve call-site ergonomics in non-fallible facts while still remaining within the reference type surface.
