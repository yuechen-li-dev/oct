# Wrapper authoring guide (Mx103a)

This guide defines the standard pattern for new Go-backed Oct wrapper modules.

## 1. Structure

For a new wrapper module (example: `IO.Csv`):

1. Add runtime builtin handler(s) under `internal/interpret/`.
2. Register handlers via `newWrapperBuiltinRegistry(...)` composition.
3. Add builtin name(s) in `internal/builtin/builtin.go`.
4. Add typechecker contract in `internal/typecheck/typecheck.go`.
5. Add thin Oct API in `Libraries/<Family>/`.
6. Add Oct facts (`.octest`) for happy/error behavior.
7. Update library README and milestone report.

## 2. Runtime helpers (required)

Use wrapper helpers instead of ad hoc extraction:

- `newWrapperCall(...)`
- `expectArity(...)`
- `stringArg(...)`
- `intArg(...)`
- `floatArg(...)`
- `wrapperIntResult(...)`
- `wrapperStringResult(...)`
- `wrapperErrorf(...)`
- `wrapperErrorResult(...)`

## 3. Error contract

Map wrapper failures with standard categories:

- `InvalidArgument`
- `InvalidHandle`
- `NotFound`
- `Conflict`
- `InvalidData`
- `BackendFailure`

Oct-visible error shape must stay:

`<BuiltinName>: <ErrorKind>: <message>`

## 4. Test shape

Each wrapper module should have:

- **happy path** fact(s)
- **invalid/error path** fact(s)
- deterministic assertions for output where possible

Infrastructure behavior (argument/result/error helper behavior) should be unit-tested in `internal/interpret`.

## 5. What not to do

- Do not hand-roll argument decoding in each wrapper handler.
- Do not invent per-wrapper error formatting.
- Do not expose broad foreign API mirrors by default.
- Do not skip `.octest` coverage for wrapper user behavior.
