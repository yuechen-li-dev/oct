# MACHINA UI Gio Backend Spike (M109)

M109 adds an optional Gio native backend spike for Machina UI.

## Scope and ownership

Gio is backend-only in this milestone:

- owns window lifecycle, pointer input, and drawing submission.
- consumes Machina render commands.
- sends pointer coordinates to `session.PointerDown`.

Gio does **not** own:

- app state
- UIIR lowering
- layout resolution
- hit-testing semantics
- action dispatch semantics

Those remain in the existing deterministic session pipeline from M108.

## What this spike proves

- A real native window loop can consume Machina render output.
- Pointer click coordinates can flow back through session hit testing and dispatch.
- A counter demo can rerender through state changes without handing semantic ownership to Gio.

## Build and run

This backend is isolated behind the `machina_gio` build tag.

Run backend tests:

```bash
go test -tags machina_gio ./internal/machina/backend/gio
```

Run demo:

```bash
go run -tags machina_gio ./cmd/machina-gio-smoke
```

## CI posture

Default CI lanes do not require a native window or GPU.

- Core packages continue to build and test without `machina_gio`.
- Gio backend code compiles and tests only in an explicit tagged/manual lane.

## Current limitations

- Drawing is intentionally minimal (rectangles + basic text label drawing).
- Theme/styling fidelity is not production-targeted in this spike.
- No keyboard/focus/text-input path is provided beyond pointer/click flow.
- Automated GUI interaction tests are not added to normal CI.

## Future work before production backend

- Map backend drawing closer to style semantics when style contracts are stabilized.
- Harden clip-stack behavior and visual parity checks against snapshots.
- Add resize/viewport plumbing from native window size events.
- Expand input coverage (hover, keyboard, focus) without changing semantic ownership boundaries.
