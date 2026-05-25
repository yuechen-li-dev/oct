# Machina UI Layout M113 (internal grid/cell core)

M113 adds deterministic internal grid layout primitives in `internal/machina/layout`.

## Added contract

- `CellFrame` for direct children of a grid-arranged parent.
  - `Column`, `Row` are zero-based and must be `>= 0`.
  - `ColumnSpan`, `RowSpan` must be `> 0`.
- `GridArrange` for parent arrangement.
  - `Columns` and `Rows` are required and must both be non-empty.
  - `ColumnGap`/`RowGap` and `Padding` values must be finite and non-negative.
- `GridTrack` supports:
  - `GridTrackFixed` with finite non-negative `Size`.
  - `GridTrackFill` with finite positive `Weight`.

## Resolution rules

- Grid content area is parent rect minus padding.
- Negative content area is an error.
- Track gaps consume space before fill distribution.
- Remaining space after fixed tracks and gaps is distributed to fill tracks by weight.
- Negative remaining space is an error.
- Grid child order remains deterministic (compiled child order rules still apply).
- Direct grid children must use `CellFrame`; other frame kinds are rejected.
- Child cells must remain in-bounds:
  - `Column + ColumnSpan <= len(Columns)`
  - `Row + RowSpan <= len(Rows)`
- Spans include intermediate gaps.

## Error classes (stable-ish)

- invalid grid columns/rows
- invalid grid track size/weight
- invalid grid gap/padding
- negative grid content size
- negative grid remaining space
- invalid grid child frame kind
- invalid cell frame values
- grid cell out of range

## Explicit non-goals in M113

- No auto-placement.
- No CSS Grid clone behavior.
- No new public `UI.GridArrange` / `UI.Cell` authoring API in this milestone.
- No UIIR ABI change.
- No interpreter/runtime/backend behavior changes.

## Future work

- Add Oct `UI.*` authoring surface for grid/cell.
- Add lowering support once UIIR carries explicit track/cell metadata.
- Migrate Storefront to grid placement after internal contracts are exposed.


## M114 authoring bridge

M114 adds Oct-facing nested-array authoring via `UI.GridRows(UI[][])`. It lowers to internal `GridArrange` + `CellFrame` with equal fill tracks (weight=1), no spans, no auto-placement, no CSS-grid semantics.
