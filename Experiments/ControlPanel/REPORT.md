# ControlPanel M0 Report

## What was easy

- The core loop (`View(state) -> UI`, emit explicit token, update state, patch UI) is straightforward and matches the M0 architecture well.
- Small control layouts are easy to express with `Column` + `Row` + `Text` + `Button`.
- Event tokens are easy to reason about in tests because they are plain strings and visible at every step.

## What was awkward

- There is no first-class float/int-to-string formatting surface in this flow, so even a tiny panel needs hand-written label conversion helpers for numeric labels.
- Event tokens are stringly-typed; typo safety is entirely manual.
- Disabled/conditional button affordances are unavailable at the widget level, so the app must accept-and-ignore blocked actions like `Start` while power is off.

## What felt too verbose

- Rebuilding full records for each event branch is repetitive in M0 state reducers.
- Writing and maintaining ad-hoc decimal label mapping for display values is noisy relative to the app’s intent.
- Full explicit event plumbing in tests (emit/drain/update/patch for each step) is good for correctness but verbose for even tiny behavior scripts.

## Missing UI element/pattern that would help most next

The highest-value minimal addition would be a tiny built-in numeric formatting helper (for at least `Float -> String` with fixed precision).

This would remove non-domain boilerplate that currently dominates simple status text panels.

## Is current UI M0 enough for a basic control panel without hacks?

Yes, it is enough to build a basic explicit-state control panel and prove the event/state/rerender loop.

However, the authoring friction is noticeable around:

1. numeric text formatting,
2. stringly event tokens,
3. lack of disabled-state controls.

These do not block M0 viability, but they materially affect ergonomics for larger UI examples.
