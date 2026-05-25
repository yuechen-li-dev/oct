# MACHINA UI Hit Testing (M106)

M106 adds an internal-only pure hit-testing contract under `internal/machina/hittest`.

## Purpose

Convert root-local pointer coordinates into symbolic UI actions by evaluating:

- `layout.ResolvedLayoutDocument`
- lowering action metadata (`map[layout.NodeID]lowering.Action`)
- optional lowering semantics metadata (`map[layout.NodeID]lowering.Semantics`)

This is renderer/backend independent and does not change current runtime wiring.

## Contract

- `Point` is a backend-independent root-local coordinate (`X`, `Y` float64).
- `BuildIndex(...)` builds deterministic actionable candidates.
- `Index.HitTest(point, semantics)` returns optional `Result` with node id, rect, action, and optional semantics.

## Source of truth for actionability

Action metadata is the only source of actionable nodes.

- Nodes not present in `actions` are ignored.
- Disabled controls are not actionable because M105 lowering omits disabled button actions.

## Ordering / winner policy

Candidate set order is resolved-tree pre-order traversal.
Hit evaluation iterates candidates in reverse traversal order.
Therefore later pre-order actionable nodes win overlaps.

## Bounds policy

Half-open rectangle containment:

- `x >= rect.X`
- `x < rect.X + rect.Width`
- `y >= rect.Y`
- `y < rect.Y + rect.Height`

## Deferred (out of scope)

- keyboard/focus/text-input behavior
- event bubbling/capture/routing
- hover/pressed state handling
- Gio/native backend integration wiring
- render command stream generation
