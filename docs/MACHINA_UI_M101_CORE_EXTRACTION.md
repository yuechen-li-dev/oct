# M101 Machina UI core extraction (progress note)

This change introduces initial `internal/machina` core packages and keeps current behavior intact:

- `internal/machina/uiir`: extracted UIIR model and canonical JSON ABI helpers (`machina.uiir.v1`), node-id assignment, cloning, structural signatures, and event-token search.
- `internal/machina/layout`: extracted pure layout/box resolution helpers and layout constants.

## What remains unchanged intentionally

- Interpreter builtins and runtime behavior still run through `internal/interpret/ui_runtime.go`.
- WASM/WebView host path is unchanged.
- User-facing Oct UI API and JSON ABI shape are unchanged.

## Why this intermediate step

The interpreter currently has many direct references across runtime and tests; this keeps compatibility while establishing the target package boundaries first.

## Next step

Wire `internal/interpret` and WASM runtime/lowering to consume the new core packages directly, then remove duplicated interpreter-local core implementations.

## M101b1 Bridge/Parity Step

The extracted core package now has conversion/parity coverage against the current interpreter-local implementation. This intentionally does not switch `ValueUI` yet. The next step is to switch `ValueUI` and production builtins once parity is proven.

## M101b2 Production Switch

M101b2 switched production interpreter UI values and WASM runtime/lowering to consume `internal/machina/uiir` and `internal/machina/layout` directly. The interpreter now acts as an Oct adapter over Machina UI core rather than owning production UIIR semantics.

## M101b3 Cleanup After Production Switch

M101b3 removed obsolete bridge/interpreter-local UIIR scaffolding after production switched to `internal/machina/uiir`. The interpreter now contains only adapter/session code for Machina UI, while UIIR/layout semantics live in `internal/machina`.
