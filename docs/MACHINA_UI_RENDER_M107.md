# MACHINA UI Render Command Stream (M107)

M107 adds an internal, backend-neutral render command seam under `internal/machina/render`.

## Purpose

Machina now produces deterministic render commands from:
- resolved layout geometry (`internal/machina/layout`)
- lowering semantics (`internal/machina/lowering`)
- lowering style placeholder metadata (currently accepted but not yet interpreted)

This is internal-only and does not change interpreter/runtime/WebView/WASM behavior.

## Backend-neutral command model

The command model currently includes:
- `BeginFrame`
- `EndFrame`
- `FillRect`
- `DrawText`
- `PushClip`
- `PopClip`

M107 emits `DrawText` for text/button semantics labels. Actions are not render commands; they remain input/runtime metadata for hit testing and dispatch.

## Deterministic ordering

`BuildCommands` emits commands in deterministic pre-order traversal of the resolved tree.
Sibling visit order is deterministic by:
1. `Z` ascending
2. `Order` ascending
3. deterministic ID tie-break

This policy is intentionally simple and snapshot-friendly; deeper paint/layer policies are deferred.

## Snapshot backend role in CI

The snapshot recorder validates command lifecycle and outputs stable textual snapshots.
It enforces:
- exactly one frame lifecycle (`BeginFrame` -> `EndFrame`)
- no nested frame begin
- no commands after `EndFrame`
- balanced clip stack
- stable float formatting

This enables CI to validate renderer behavior deterministically without pixel output.

## Deferred work

- Gio/native renderer consumption is deferred.
- Pixel rendering and text rasterization are deferred.
- Rich style/theme interpretation is deferred; style metadata remains placeholder-only in M107.
