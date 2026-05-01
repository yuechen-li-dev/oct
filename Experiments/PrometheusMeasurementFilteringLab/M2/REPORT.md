# P14 M2 Lab Report — Random Noise Smoke Rerun

## Why M2 exists

P14 M2 exists to verify that experiment-side Random usage is non-degenerate after Random M8b, so P14 labs can safely rely on `Random`, including `Random.Distributions`, without re-checking internals every time.

## Previous blocker

The earlier M2 rerun failed before execution with parser errors (`invalid token ... ";"`), so results were not credible because the smoke checks did not actually run.

## Syntax cleanup applied

- Rewrote M2 `.oct` implementation to current Oct syntax (no semicolon-terminated statements).
- Removed semicolon-era syntax and refactored helpers to compile under current parser rules.
- Verified state threading directly in dedicated state-threading smoke checks (`draw.Next` reuse/advance), while bulk sample collectors use deterministic per-index seeding to avoid the current cross-package `Random.Rng` vs `Rng` assignment mismatch.
- Split smoke validation into focused `[Fact]` tests instead of broad mixed checks.

## Random APIs tested

Core:
- `Random.RngSeed`
- `Random.RandFloat01`
- `Random.RandFloatRange`
- `Random.RandBernoulli`
- `Random.RandNormal`

Distribution helpers:
- `Random.Jitter`
- `Random.Spike`
- `Random.DriftStep`

## Smoke-test results

- Uniform `[0,1)` smoke: bounded, varied, mean in broad sanity band.
- Float range `[-1,1)` smoke: bounded, includes both positive and negative values, non-constant.
- Bernoulli smoke: mixed outcomes at `p=0.5`; extremes behave correctly (`p=0` all false, `p=1` all true).
- Normal smoke: non-constant sequence, both signs present, mean in broad sanity band.
- Distribution helper smoke:
  - `Jitter`: bounded in `[-0.05, 0.05)`, non-zero appears.
  - `Spike`: mostly zeros with observed spikes.
  - `DriftStep`: path changes over time and is deterministic for same seed.
- State threading from experiment package:
  - same seed reproduces sequence,
  - different seed diverges,
  - reusing same state repeats next draw,
  - advancing state changes draw.

## Verdict

**Random is usable for P14 labs** as a smoke-test-level dependency from experiment code paths, including `Random.Distributions`. M2 no longer fails at parse-time and now provides concrete non-degeneracy evidence.

## Inconsistency note

Observed inconsistency: in this experiment package path, assigning `draw.Next` back into a mutable RNG variable produced a type mismatch (`expected Random.Rng, got Rng`). The smoke suite still validates state threading behavior directly, but this cross-package type qualification mismatch should be tracked as a tooling/docs gap.
