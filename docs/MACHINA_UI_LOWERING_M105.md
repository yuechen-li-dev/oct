# MACHINA UI Lowering (M105)

M105 adds an internal-only lowering contract that converts semantic UIIR (`internal/machina/uiir`) into layout substrate rows (`internal/machina/layout`) plus metadata maps.

## Lowering result contract

`internal/machina/lowering` defines:

- `Result.Rows []layout.LayoutRow`
- `Result.Actions map[layout.NodeID]Action`
- `Result.Semantics map[layout.NodeID]Semantics`
- `Result.Styles map[layout.NodeID]Style` (placeholder for M112)

This contract is deterministic and keyed by `layout.NodeID`.

## Metadata in M105

- **Actions**: enabled `Button` nodes with non-empty `Event` lower to action metadata.
- **Semantics**:
  - text: role=`text`, label=text content
  - button: role=`button`, label=button label, disabled/focusable derived from `Enabled`
  - container-like nodes: role=`container`
- **Styles**: empty placeholder map only. Full style records are deferred to M112.

## Temporary sizing policy (deterministic)

Until measurement and renderer integration milestones, lowering uses deterministic fallback sizes:

- text: `width = max(1, len(text)*8)`, `height = 20`
- button: `width = max(80, len(label)*8 + 24)`, `height = 32`

These are internal defaults intended for resolver integration/testing only.

## Scope and compatibility

- Internal-only milestone; no runtime wiring yet.
- No changes to `machina.uiir.v1` ABI/JSON.
- No behavior changes in current interpreter mount/patch/runtime path.
- No hit testing/render command generation in M105.
- No grid/cell implementation in M105 (deferred to M113).

## Next milestones

- M106: hit testing contract
- M107: render command stream contract
- M112: style records and styling contract
- M113: grid/cell lowering and resolution model
