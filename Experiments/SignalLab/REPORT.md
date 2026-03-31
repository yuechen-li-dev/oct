# SignalLab Report

## M0 findings

### Was the current UI system sufficient to build this app?

Yes. The current UI primitives were sufficient to build a real, UI-backed application with explicit state, explicit event tokens, deterministic update logic, and a mount/emit/drain/update/patch loop. No runtime changes were needed.

### Where did layout feel natural vs awkward?

Natural:

- AnchoredBox worked well for high-level page slicing (header, control area, readout area, display placeholder).
- Row/Column worked well for local semantic grouping of controls and readouts.

Awkward:

- Keeping the same control intent represented in both grouped rows and explicit absolute placements adds duplication pressure.
- Cross-region alignment still requires manual visual arithmetic when two anchored areas need to line up with absolute clusters.

### Did coordinate hygiene help in a larger layout?

Yes. A local constants block (anchors, origins, spacing, shared sizes) significantly reduced drift and made edits local:

- Moving a control cluster is a one-constant change.
- Adjusting spacing is one-value tuning instead of many scattered literal edits.
- Readability improved because placements describe intent (`origin + offset`) instead of unrelated numbers.

### Were any UI primitives obviously missing?

Concrete gaps observed in M0:

1. A small first-class "labeled value" helper primitive (or equivalent compositional shortcut) would reduce repetitive readout row boilerplate.
2. Optional button-group helper semantics (author-side utility, not runtime feature) would reduce repeated event/enable wiring patterns for mutually exclusive selections.
3. A built-in lightweight numeric badge/readout element could reduce text-only formatting repetition.

None of these are blockers; they are ergonomic friction points.

### Did hybrid layout still feel like the right model?

Yes. Hybrid remained the right model:

- Anchors define macro composition.
- Row/Column handles local flow.
- AbsoluteBox handles precision touch-up points.

This combination gave predictable edit locality without introducing a heavier layout engine.

### What is the single most painful part of building this UI?

The most painful part was duplicated coordinate and control declarations when preserving both grouped semantic layout and precise absolute placements in one view. It stays manageable for M0 size, but becomes the dominant maintenance cost as control count increases.

## Recommendation

Proceed to **SignalLab M1** while keeping the existing runtime and layout model unchanged.

M1 should focus on:

- adding a minimal plot area implementation on top of the current model,
- evaluating whether the same coordinate hygiene conventions still scale,
- introducing only author-level helper conventions (not runtime features) for repeated control/readout patterns.

## M1 findings

### Did the current runtime/layout model remain sufficient?

Yes. M1 adds a richer surface (controls + readouts + mini plot region + sample history) while preserving the M0 architecture exactly:

- explicit external state,
- explicit event tokens,
- pure `Update(state, event) -> state`,
- pure `View(state) -> UI`,
- unchanged mount/emit/drain/update/patch loop.

No runtime API changes were required.

### Did the plot/display area fit naturally into the current UI system?

Yes. A minimal plot-like readout was implemented as an anchored display region with textual marker lines derived from bounded history state. This fit naturally into existing `AnchoredBox + Column + Text` composition, with no new rendering primitives.

### Did author-level helper conventions reduce duplication meaningfully?

Yes, at modest but real scale. Three helpers reduced repeated boilerplate:

- `LabeledValue(label, valueText)` for repeated readout rows,
- `SignalSelectButton(label, token, selected, enabled)` for mutually exclusive signal selection controls,
- `ReadoutBox(title, lines)` for stable readout grouping.

The helpers kept event wiring explicit while reducing copy-paste of label/event/enable patterns.

### Which helper patterns felt worth keeping?

Most useful in practice:

1. `SignalSelectButton` (highest value): centralizes selected/disabled behavior and makes intent obvious.
2. `LabeledValue`: lowers repetitive readout row construction.
3. `ReadoutBox`: helpful for structure, but lower leverage than the first two.

### Was any new runtime/widget feature actually justified?

Not yet. M1 did not justify runtime or widget expansion. Existing primitives were sufficient for this app growth step.

### What is now the single biggest pain point in building a richer app?

Manual record-copy updates are now the dominant cost. Because state is explicit and immutable, each event branch repeats full record reconstruction, including history fields. This is correct and transparent, but verbose as state shape grows.

## Updated recommendation

Proceed with one more app-level growth step before runtime changes.

- Continue validating richer app behavior in M2 with current runtime/layout.
- Prefer additional author-level helper conventions (especially for repetitive update/record wiring and grouped controls) before introducing new primitives.
- Revisit runtime changes only if multiple experiments converge on the same hard blocker.

## M2 findings

### What helper/state patterns were tried?

M2 kept the same state shape as M1 and introduced explicit author-level state-copy helpers:

- `WithRunning(model, value)`
- `WithNoiseEnabled(model, enabled)`
- `WithSelectedSignal(model, signal)`
- `WithTime(model, time)`
- `WithValue(model, value)`
- `WithTickCount(model, tickCount)`
- `WithHistory(model, count, h0, h1, h2, h3)`

`Update` now composes these helpers instead of repeating full record literals per event branch.

### Did they reduce Update verbosity?

Yes, materially inside `Update`. Event branches now read as intent-first transformations:

- start/stop: `Recompute(WithRunning(...))`
- toggle/select: `Recompute(WithNoiseEnabled(...))`, `Recompute(WithSelectedSignal(...))`
- step: unchanged high-level flow (`AdvanceOne -> Recompute -> RecordSample`) but `AdvanceOne` now uses helpers.

This removed repeated field-by-field reconstruction from the `Update` branches.

### Did readability improve?

Yes for event handling. The event logic is shorter and clearer because each branch names exactly which field changes.

Tradeoff: boilerplate did not disappear; it moved into helper definitions. This is acceptable for the probe because:

- boilerplate is centralized,
- naming carries intent,
- behavior remains explicit and pure,
- update flow is easier to scan quickly.

### Were helpers alone enough, or was state reshaping needed?

Helpers alone were enough for this milestone. No state reshaping was required to make `Update` readable at current app size.

### What is now the single biggest pain point?

The biggest remaining pain is maintaining many near-identical helper constructors as state fields grow. `Update` is cleaner, but helper maintenance still has record-copy drift risk.

### Is a language feature now justified, or can this stay author-level for now?

For current SignalLab scale, this can stay author-level for now.

M2 evidence suggests explicit immutable state remains workable with disciplined helper conventions, and immediate language/runtime changes are not yet justified. A language-level record update feature should only be reconsidered if multiple larger experiments show helper maintenance cost becoming dominant.

## M3 findings

### What control-state structure moved into Octomata?

M3 moved control-mode transitions out of plain helper branching into an explicit Octomata flow:

- `ControlModeTransition(current, event) -> String`
- states: `Idle`, `Running`, `Paused`
- transition selection uses `when` in each state with explicit guard ordering.

`Update` now delegates start/stop/pause/resume mode changes through `NextControlMode`, which steps the flow and applies the returned control mode.

### What remained ordinary record state?

The experiment kept numeric/display/application data in explicit immutable record fields:

- `Time`
- `Value`
- `TickCount`
- `SelectedSignal`
- `NoiseEnabled`
- `HistoryCount` + history slots

Step/recompute/history logic remained regular record-based state transformation.

### Did Octomata reduce complexity?

Partially.

- It made allowed control transitions materially clearer by concentrating them in one flow surface (`Idle/Running/Paused`) instead of scattering mode expectations across event branches.
- It reduced boolean-centric intent branching in `Update` for control events.

But total code volume did not drop dramatically because record-copy helpers still dominate data updates.

### Did readability of control transitions improve?

Yes.

The allowed transition graph is now explicit and reviewable in one place. It is easier to answer questions like “can paused resume?” or “what events are ignored in idle?” without scanning the whole reducer.

### Did update verbosity reduce enough to matter?

Moderately.

`Update` became cleaner for control events, but data-path verbosity is still primarily governed by explicit record updates and history management. So the gain is meaningful for control logic, not a broad verbosity solution.

### What became more awkward?

- A small adapter (`NextControlMode`) is needed to instantiate/step/read a flow result for each control event.
- The flow is excellent for control semantics but does not help with dense numeric/history record-copy boilerplate.

### Is this a better model than M2 helper-only reducer style?

For apps with non-trivial control modes, yes.

M3 suggests a hybrid split is cleaner than helper-only reducer style:

- Octomata for control-state progression/contracts.
- Records for numeric/data state.

### Should future interactive apps prefer this split?

Recommendation: **Yes, prefer the hybrid split by default when control-state transitions are a meaningful part of behavior.**

If an app has very shallow control state, helper-only style remains acceptable. But once mode transitions matter, Octomata provides clearer control contracts with low architectural disruption.
