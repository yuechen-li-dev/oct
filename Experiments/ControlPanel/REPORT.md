# ControlPanel M0/M1 Report

## What was easy

- The core loop (`View(state) -> UI`, emit explicit token, update state, patch UI) is straightforward and matches the M0 architecture well.
- Small control layouts are easy to express with `Column` + `Row` + `Text` + `Button`.
- Event tokens are easy to reason about in tests because they are plain strings and visible at every step.

## What was awkward in M0

- There is no first-class float/int-to-string formatting surface in this flow, so even a tiny panel needs hand-written label conversion helpers for numeric labels.
- Event tokens are stringly-typed; typo safety is entirely manual.
- Disabled/conditional button affordances are unavailable at the widget level, so the app must accept-and-ignore blocked actions like `Start` while power is off.

## M1 ergonomics pass outcome

- Numeric labels now use `FormatFloat(value, precision)` with deterministic fixed precision (`Setpoint`, `Output`).
- The panel keeps explicit string tokens but uses named constants (`StartEvent`, etc.) to reduce typo-prone literals.
- `Button` now includes explicit `enabled: Bool`; disabled controls are rejected by runtime event emission and do not enqueue events.
- The event/state/rerender architecture is unchanged: explicit emit -> drain -> update -> patch.

## Scope guardrails preserved

No hooks, effects, local component state, style/theme system, or layout redesign were introduced.
