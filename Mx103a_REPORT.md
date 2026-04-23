# Mx103a report — wrapper backend infrastructure substrate

## 1) Existing wrapper patterns audited

Audited runtime wrapper-like surfaces before changes:

- `IO.Xlsx` wrapper builtins (`XlsxCreateWorkbook`, `XlsxAddSheet`, `XlsxSetCellString`, `XlsxSetCellFloat`, `XlsxSaveWorkbook`) in `internal/interpret/interpret.go`.
- Bridge M0 substrate (`wrapperHandleStore`, wrapper builtin registry, `wrapperErrorResult`) in `internal/interpret/wrapper_bridge.go`.
- UI builtins in `internal/interpret/ui_runtime.go` as another wrapper-like backend surface over runtime-managed handles/state.

## 2) Inconsistencies/ad hoc behavior found

- Argument decoding was partially duplicated and ad hoc (`evalWorkbookAndSheetArgs`, `evalWorkbookSheetCellArgs`, direct `evalExpr` sequences in wrapper handlers).
- Result lifting was inconsistent (`evalResult{value: ...}` hand-built in each wrapper function).
- Error mapping format was mostly consistent but lacked standardized error categories (everything collapsed to plain message strings).
- Wrapper registration shape existed but only accepted a single map at construction, making multi-wrapper composition ad hoc.

## 3) Reusable substrate introduced in Mx103a

- Added `wrapperCall` helper for wrapper handlers:
  - arity checks (`expectArity`)
  - argument evaluation + passthrough fallible propagation (`evalArg`)
  - typed extraction helpers (`stringArg`, `intArg`, `floatArg`)
- Added common result lifting helpers:
  - `wrapperIntResult`
  - `wrapperStringResult`
- Added standardized wrapper error contract:
  - typed `wrapperErrorKind` categories
  - `wrapperErrorf(...)` for consistent wrapper error emission
  - `wrapperErrorResult(...)` keeps stable `Builtin: Kind: message` shape
- Expanded wrapper registry composition:
  - `newWrapperBuiltinRegistry` now composes multiple handler sets for easy future module batching.

## 4) Golden wrapper example

Added `IO.Json` as the small canonical wrapper:

- runtime builtin: `JsonNormalize` (Go `encoding/json` compact path)
- Oct surface: `NormalizeJson(text: String) -> String ! Error`
- demonstrates:
  - naming shape (`IO.<Module>`, wrapper builtin + thin Oct function)
  - argument decoding via `wrapperCall.stringArg`
  - result lifting via `wrapperStringResult`
  - deterministic error mapping via `wrapperErrorf(wrapperErrorInvalidData, ...)`
  - tests via focused `.octest` facts

## 5) What Mx103a now enables (Mx103b/c...)

With this substrate, future wrapper waves can be added as repetitive assembly work:

- add builtin(s) and register in composed wrapper registry
- use shared argument/result/error helpers
- add thin Oct wrapper module
- add standard happy/error `.octest` facts
- document surface in library README + bridge notes

Targets like file/path/directory, CSV, regex, hashing, time, and HTTP client now have a consistent backend template.

## 6) Out of scope (intentionally unchanged)

- No broad wrapper wave implementation (only one golden wrapper added).
- No plugin/reflection/auto-generation framework.
- No compiled-mode wrapper lowering expansion.
- No redesign of unrelated runtime/compiler systems.

## Language/reference consistency note

- `Xlsx*` and `JsonNormalize` wrapper builtins are runtime/typechecker surfaces, but they are not currently documented in `Language/reference/language/09-builtins.md`.
- Per repository policy, this is a documentation gap and should be resolved in a follow-up language-reference update.
