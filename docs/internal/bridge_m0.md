# Bridge M0: Internal Go Wrapper Substrate

Bridge M0 defines the minimum internal substrate for maintainers to build curated Go-backed wrappers with consistent runtime behavior.

## What Bridge M0 is

- Internal runtime substrate for handle-backed wrappers.
- Shared pattern for wrapper builtin registration and dispatch.
- Shared error mapping convention from backend failures to Oct `! Error`.

## What Bridge M0 is not

- Not a public FFI.
- Not arbitrary Go package import.
- Not auto-generated wrappers.
- Not reflection-driven dynamic binding.

## Standard patterns

1. **Handle-backed resource values**
   - Oct-visible records carry explicit `Int` handles.
   - Backend Go objects stay in interpreter-owned stores.

2. **Handle store**
   - Allocate deterministic numeric handles.
   - Resolve handle -> backend object.
   - Return deterministic invalid-handle errors.
   - Optional release path exists for future lifecycle milestones.

3. **Wrapper builtin registration**
   - Wrapper builtins are registered through a dedicated internal registry.
   - Dispatch is explicit by builtin name to typed handler functions.

4. **Error mapping convention**
   - Backend and state errors map to Oct errors as `<BuiltinName>: <ErrorKind>: <message>`.
   - Invalid handle/state is surfaced as fallible Oct errors, not runtime panics.
   - Wrapper error categories are standardized (`InvalidArgument`, `InvalidHandle`, `NotFound`, `Conflict`, `InvalidData`, `BackendFailure`).

5. **Wrapper call helpers**
   - Wrapper handlers should use `wrapperCall` helpers for argument arity checks and argument decoding.
   - Use `stringArg`, `intArg`, and `floatArg` helpers instead of ad hoc per-wrapper extraction logic.
   - Use `wrapperIntResult` / `wrapperStringResult` for common lifted values.

6. **Testing/docs shape**
   - Each wrapper module should include Oct-level `.octest` facts with:
     - happy path
     - explicit error path
     - deterministic output assertions where possible
   - Runtime helper behavior should be unit-tested in Go (`internal/interpret/wrapper_bridge_test.go`).
   - Library-facing wrapper docs live with the wrapper package README (see `Libraries/IO/README.md`).

## Wrapper design expectations

- Wrappers stay thin and Oct-shaped.
- Wrappers expose selective, useful operations rather than mirroring foreign APIs wholesale.
- Wrappers preserve explicit Oct fallibility (`! Error`) and deterministic failure behavior.
- Wrappers remain curated and hand-authored by maintainers.

## Current proof case

`IO.Xlsx` and `IO.Json` now run on this Bridge M0 substrate for:

- workbook handle allocation and lookup
- wrapper builtin registration/dispatch
- deterministic backend-to-Oct error mapping
- shared wrapper call argument/result helpers
