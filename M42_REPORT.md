# M42 — Octomata Core A Proof Packages Report

## 1) What was built

- `Packages/TurretControl`
  - Game-shaped turret controller proof with explicit `Search`, `Track`, `Engage`, and `Recover` states.
  - Uses direct `if` + `goto` + `suspend` Core A patterns, plus runtime `Step`, `Active`, and `Result` tests.
- `Packages/TankController`
  - Industrial fill/settle/drain controller proof with explicit `Idle`, `Filling`, `Settling`, `Draining`, and `Fault` states.
  - Includes safety-oriented branching and a tested emergency-stop fault path.

## 2) What Core A did well

- Readability: state blocks made intent visible without helper framework layers.
- Explicitness: transition conditions are clear because `goto` is direct and local.
- Stepwise control feel: `Step(...)` + `Active(...)` made progression easy to reason about in tests.
- Usefulness: both game and industrial controller shapes were expressible with just Core A.

## 3) What friction appeared

- Per-step changing external inputs are not naturally modeled inside a single flow instance without additional structure, so these proofs use fixed scenario records per run.
- Testing longer scenario scripts requires multiple dedicated flow instances to represent different operational situations.

## 4) Whether the next Octomata feature is clearer

- Core A is sufficient for small, explicit deterministic controllers right now.
- `when` now looks justified as the next addition to improve guard readability in event/condition-heavy controllers.
- Stack HFSM semantics are not required for these proof shapes yet, but become relevant once nested interruption/recovery behavior grows.
