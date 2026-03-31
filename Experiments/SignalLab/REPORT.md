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
