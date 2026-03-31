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

## M2 edit-stress pass (hybrid layout)

This pass intentionally started from the denser hybrid layout style (anchored box regions for macro placement + local `Row`/`Column` grouping for nearby controls) and then applied a sequence of small and medium edits.

### Edit sequence applied

- Moved one button 2 px left (`Setpoint +` x: `24.0 -> 22.0`).
- Moved one label 3 px down (`Live queue depth` y: `250-ish -> 253.0`).
- Widened a panel slightly by anchor change (command area right anchor `0.37 -> 0.35`; diagnostics left anchor `0.39 -> 0.37`).
- Made one button slightly taller (`Reset` height `28.0 -> 30.0`).
- Shifted one control cluster horizontally (queue/cycle labels moved from x `480.0 -> 486.0`).
- Added one new control in an existing area (`Step` button inside the run-controls row).
- Re-anchored one element/region (`Clear Alarm` moved from bottom-right footer to diagnostics-adjacent anchored placement).
- Made one small region resize more predictably (trend placeholder stretched to `right=0.97` and kept anchored in the lower band).

### How local were the edits?

- Most edits were one-line or one-expression changes in `View` and stayed local to one `Place(...)` call.
- The new `Step` control was local to the existing run-controls `Row` and required one extra button plus one event branch in `Update`.
- Approximate locality: the visual churn is concentrated inside the `View` function; no runtime loop, mount/patch/unmount surface, or state ownership model changes were needed.

### Did “move this button 2 px left” stay trivial?

- Yes. The micro-position edits were single-number diffs.
- No sibling reflow surprises occurred for absolute items.
- No unrelated command/state logic was touched for coordinate-only changes.

### Were unrelated siblings/regions disturbed?

- Mostly no for anchored macro regions and absolute controls.
- One deliberate widening edit required paired anchor updates (command + diagnostics) to keep the split balanced, but this remained local to those two region lines.

### Did repeated coordinate editing become annoying?

- Mildly, yes. Repeated hand-editing of nearby `x/y` literals is still prone to coordinate drift and inconsistent spacing (`22/24`, `156/158`, `486`).
- The pain is manageable at this scale, but the drift pattern is now visible.

### Is hybrid still better than pure box-native?

- Yes, for this panel size.
- Macro placement stayed clear with anchored/absolute boxes, while local grouping avoided excessive coordinate bookkeeping for button clusters.
- A pure box-native rewrite would increase raw coordinate noise for local control groups without clear payoff.

### Missing helper abstraction now justified?

- A tiny helper convention for repeated coordinates or cluster offsets now looks justified (for example, named constants for cluster origin/row spacing) to reduce drift and improve scanability.
- This can remain author-level convention; no new runtime/layout feature is required yet.

### Recommended next step

- Keep hybrid as the default authoring model.
- Next pass should add lightweight coordinate hygiene only (naming and small shared offsets), then repeat a micro-edit batch to verify that line-touch count stays low.
- Do not introduce a new layout engine, grid system, or hooks/effects in the next step.

## Scope guardrails preserved

No hooks, effects, local component state, style/theme system, or layout/constraint framework were introduced. State remains external, events remain explicit tokens, UI functions remain pure, and the same mount/patch/unmount runtime flow was kept.

## M3 coordinate hygiene pass (ControlPanel/M2 in place)

This pass kept the existing hybrid layout model and only refined how coordinates are authored in `View`.

### What changed

- Added a visible local constants block in `View` for:
  - anchored macro-region boundaries (`HeaderTop`, `MainTop`, `CommandAreaRight`, `DiagnosticsLeft`, etc.)
  - control-cluster origin and spacing (`ControlOriginX/Y`, `ControlRowSpacing`, `ControlColumnSpacing`)
  - shared control sizes (`ControlButtonWidth`, `ControlButtonHeight`)
  - diagnostics label cluster origin/line spacing (`DiagnosticsOriginX/Y`, `DiagnosticsLineSpacing`)
- Rewrote absolute control placement to use origin + offset arithmetic rather than repeated literals.
- Normalized obvious nearby drift in the command button grid by using one shared left-column origin and one shared column spacing.

### Before vs after patterns (concrete)

- Before: repeated hand-entered button coordinates (`22/24`, `156/158`, `242/276`) across adjacent controls.
- After: one base position (`ControlOriginX`, `ControlOriginY`) plus `ControlRowSpacing` and `ControlColumnSpacing`.
- Before: diagnostics labels repeated x/width/height with literal y values.
- After: one diagnostics origin plus `DiagnosticsLineSpacing` for the second line.
- Before: anchored values were inline, making cross-region relationships harder to scan.
- After: key anchor boundaries are named once and reused in placement lines.

### Did this reduce coordinate drift?

- Yes. Nearby-but-inconsistent numbers were removed from the clustered control area and replaced with shared spacing expressions.
- Future small alignment tweaks now happen by changing one constant instead of touching multiple literals.

### Is layout easier to scan/read now?

- Yes. Visual intent reads as “region constants, then placements” and control groups read as coordinate patterns instead of a list of unrelated numbers.
- The mental parse load is lower because repeated values have names tied to intent (origin, spacing, size).

### Was edit locality preserved?

- Yes. Edit locality remained intact:
  - micro moves in a cluster are still one-line changes (or one-constant changes)
  - macro-region edits still touch only the corresponding anchor constants/placements
- No runtime loop, state model, event model, or mount/patch flow changes were required.

### Did constants improve or hurt clarity?

- Improved clarity in this file.
- The constant set stayed lightweight and local (simple `let` bindings), so there was no additional abstraction burden.
- The pass did not introduce a new coordinate system; it only made the existing one explicit.

### Emerging best-practice pattern

- Keep hybrid layout (anchored macro regions + local row/column/absolute usage).
- At the top of `View`, define small named coordinate groups by intent:
  - region anchors
  - cluster origins
  - spacing increments
  - shared sizes
- Express nearby controls as `origin + offset` arithmetic for consistency and low-touch edits.

### Further abstraction needed?

- Not yet. This pass recovered readability and reduced drift without runtime features or new layout primitives.
- Recommendation: continue with this authoring convention; only consider stronger abstractions if future panels show repeated patterns that cannot stay readable with local constants.
