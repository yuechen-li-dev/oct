# ControlPanel M0/M1/M2 Report

## What was easy in M0

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

## M1 box-native friction test (UI M2 foundation)

Concrete authoring findings from the denser panel in `M1/`:

- **What became easier with box-native layout:**
  - Major region placement was direct: top strip, left command region, right diagnostics region, and a large lower placeholder were each one `Place(AnchoredBox(...), ...)` line.
  - Several precision edits were trivial coordinate tweaks (for example, nudging the setpoint buttons and diagnostics readouts by changing one `AbsoluteBox(x, y, ...)` value).
  - Adding a new explicit control required only one new `Place(AbsoluteBox(...), Button(...))` line; no row reflow side-effects.

- **“Move this button 2 px left” style edits:**
  - Yes, this is now straightforward: update `x` from `158.0` to `156.0` in one place.
  - Similar micro-edits (“down 3 px”, “10 px wider”) are single-number diffs and did not require restructuring siblings.

- **What remained awkward:**
  - Dense absolute controls require manually scanning for overlaps; there is no snap/alignment aid.
  - Repeated numeric constants can drift without naming conventions (for example, multiple `24.0/158.0/480.0` values).

- **Anchored boxes intuition:**
  - Anchored regions were intuitive for stable frame sections and predictable resize behavior.
  - Anchoring a bottom-right control (`Clear Alarm`) was easy and more robust than hard-coded bottom coordinates.

- **Hybrid vs pure box-native:**
  - Hybrid was better in this pass: anchored/absolute boxes handled macro placement, while local `Row`/`Column` inside regions reduced noise for button clusters and readout stacks.
  - Pure box-native everywhere would have increased coordinate bookkeeping for small grouped controls.

- **Recommended next UI step:**
  - Keep the current box-native direction and standardize a **hybrid authoring convention** (box-native for regions + Row/Column for local grouping).
  - Next probe should test maintainability by applying a scripted set of micro layout edits (nudge, resize, re-anchor) and measuring lines touched and accidental regressions.

## Scope guardrails preserved

No hooks, effects, local component state, style/theme system, or layout/constraint framework were introduced. State remains external, events remain explicit tokens, UI functions remain pure, and the same mount/patch/unmount runtime flow was kept.
