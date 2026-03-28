# M44 — Octomata Core B Proof Packages Report

## 1) What was built

- `Packages/SquadTacticsCoreB`
  - Game-shaped tactics controller proof with `Idle`, `Alert`, `Chase`, `Attack`, and `Recover`.
  - Uses `when` as the primary guarded-choice surface in multiple states.
  - Tests cover progression, suspension/resume, realistic first-match ordering, else fallback, safety fallback, and deterministic equivalence.
- `Packages/ReactorCycleCoreB`
  - Industrial-controller-shaped reactor batch controller with `Standby`, `Fill`, `Heat`, `Hold`, `Drain`, and `Fault`.
  - Uses `when` for branch-heavy safety and process transitions where guard ordering matters.
  - Tests cover progression, completion, stable-running standby behavior, safety/fault branches, realistic first-match ordering, and deterministic equivalence.

## 2) What `when` improved

- `when` reduced guard noise in branch-heavy states (`Alert`, `Heat`) by making ordered intent visible in one vertical block instead of repeated `if` ladders.
- Safety-first ordering is easier to scan: critical/fault guards appear first and are mechanically prioritized by source order.
- Controller/tactics readability improved because state bodies now read like rule tables (`case ... -> action`) rather than nested imperative checks.
- Mandatory `else` made default behavior explicit, which improved clarity for long-running controller states (`Idle`, `Standby`).

## 3) What friction remains

- Inputs are still fixed per flow instance in these proofs, so scenario variation across time requires additional instances rather than mutating a single run script.
- `when` branch actions are intentionally narrow, so any multi-action branch choreography still has to be represented through state transitions rather than local branch blocks.

## 4) Whether the next Octomata step is now clearer

- Core B now appears materially better for guard-heavy controller authoring, and no immediate additional control feature is required to express these golden-path proofs.
- The next most valuable step looks less like new branching constructs and more like execution ergonomics/observability polish (for example better runtime traceability during multi-step controller tests).
