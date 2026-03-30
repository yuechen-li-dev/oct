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
   - Backend and state errors map to Oct errors as `<BuiltinName>: <message>`.
   - Invalid handle/state is surfaced as fallible Oct errors, not runtime panics.

## Wrapper design expectations

- Wrappers stay thin and Oct-shaped.
- Wrappers expose selective, useful operations rather than mirroring foreign APIs wholesale.
- Wrappers preserve explicit Oct fallibility (`! Error`) and deterministic failure behavior.
- Wrappers remain curated and hand-authored by maintainers.

## Current proof case

`IO.Xlsx` now runs on this Bridge M0 substrate for:

- workbook handle allocation and lookup
- wrapper builtin registration/dispatch
- deterministic backend-to-Oct error mapping
