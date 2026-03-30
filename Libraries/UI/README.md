# UI M0

## Purpose

`Libraries/UI` provides a tiny declarative UI layer for Oct applications.

## M0 surface

- `UI` (first-class opaque type)
- `Text(content: String) -> UI`
- `Button(label: String, event: String) -> UI`
- `Row(children: UI[]) -> UI`
- `Column(children: UI[]) -> UI`

## Architecture

- Components are ordinary Oct functions returning `UI`.
- Runtime and reconciler (`mount` / `patch` / `unmount`) are implemented in Go internals.
- State is external to components (`state -> UI`).
- Events are explicit tokens (`String`) bridged through runtime dispatch.

## Non-goals (M0)

Not a full frontend framework. M0 intentionally excludes hooks, effects, local component state,
routing, forms abstractions, styling/theming, and browser-shaped public DOM APIs.
