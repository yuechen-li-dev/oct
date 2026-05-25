# Machina UI Layout M104 (internal core)

M104 adds an internal-only deterministic layout-row substrate in `internal/machina/layout`.

## Added internal core

- `LayoutRow` as the flat authored/compiled row unit.
- `LayoutDocument` as validated row graph plus deterministic child ordering.
- `ResolvedLayoutDocument` and `ResolvedLayoutNode` as resolved geometry output.
- Core frame kinds for this milestone:
  - `RootFrame`
  - `AbsoluteFrame`
  - `AnchorFrame`
  - `FixedFrame`
  - `FillFrame`
- Minimal arrange support:
  - `StackArrange` with axis, gap, and padding.

## Pipeline

- `CompileRows(rows)` validates and compiles rows into deterministic document form.
- `ResolveDocument(document, rootRect)` resolves geometry deterministically.
- `ResolveRows(rows, rootRect)` convenience entrypoint.

## Scope and compatibility

- Internal only.
- No changes to `Libraries/UI` authoring surface.
- No UIIR ABI changes.
- No runtime mount/patch behavior changes.
- Existing absolute/anchored resolver API remains intact.

## Deferred work

Deferred beyond M104:

- UIIR → layout-row lowering.
- Hit testing.
- Render command streams.
- Gio backend integration.
- Grid/cell/guide/responsive/layer systems.
- Theme/style records and dispatch helpers.
