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
