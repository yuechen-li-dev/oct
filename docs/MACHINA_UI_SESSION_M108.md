# MACHINA UI Session Loop (M108)

## Purpose

M108 introduces an internal, headless, backend-neutral native session contract for Machina UI (Machine Native UI). The session composes the deterministic Machina pipeline into a host loop that can later be connected to native backends.

This milestone does **not** introduce Gio, windowing, or pixel rendering.

## Session pipeline composition

For each rebuild, session executes:

1. `App.Project(state)` -> semantic `uiir.Node`
2. `lowering.Lower` -> `lowering.Result`
3. `layout.ResolveRows` -> `layout.ResolvedLayoutDocument`
4. `hittest.BuildIndex` -> symbolic hit index
5. `render.BuildCommands` -> backend-neutral command stream
6. `render.RecordSnapshot` -> deterministic textual snapshot

Given same state and viewport, repeated rebuilds yield a stable snapshot.

## Pointer event cycle

On pointer down:

1. Hit-test pointer coordinates against current hit index.
2. If miss: no dispatch, state unchanged.
3. If hit: resolve symbolic `lowering.Action`.
4. Call `App.Dispatch(state, action)`.
5. Replace session state with returned state.
6. Rebuild full pipeline.

This keeps action dispatch deterministic and independent from backend runtime details.

## Responsibility split

Session owns:

- project/lower/layout/hittest/render-command/snapshot lifecycle
- deterministic pointer->action->dispatch->rebuild cycle
- cached current state and current pipeline artifacts

Backends (future) own:

- window/system event ingestion
- mapping backend pointer events to session pointer events
- consuming command streams or snapshots for rendering

## Explicit non-goals (M108)

- No Gio backend integration (planned for M109).
- No window lifecycle management.
- No pixel renderer.
- No keyboard/focus/text input.
- No event bubbling/capture/routing.
- No Octomata stack push/pop assumptions.
- No changes to `Libraries/UI` authoring surface.

## Forward direction

- M109 can plug a native backend (for example Gio) into this contract by forwarding pointer coordinates into session and consuming render commands.
- Storefront migration remains a later milestone.
