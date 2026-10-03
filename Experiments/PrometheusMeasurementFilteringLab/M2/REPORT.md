# P14 M2 Lab Report — Random Noise Smoke

## Why M2 exists

P14 M2 verifies that Random, used from an experiment package, is
non-degenerate, so that the P14 labs can rely on it without re-checking its
internals every time.

## What it checks

Rewritten on 2026-10-03 for Random v2. Every draw is a function of a stream,
an index and the draw's parameters; nothing is threaded from one draw to the
next. Each collector draws from its own stream, forked by name from the seed.

Native draws and library helpers exercised from this package:

- `Random.Seeded`, `Random.Fork`
- `Random.Unit`, `Random.Units`
- `Random.Between`
- `Random.Normal`, `Random.Normals`
- `Random.Chance`
- `Random.Spike`

## Smoke-test results

- Uniform `[0, 1)`: bounded, varied, mean in a broad sanity band.
- Range `[-1, 1)`: bounded, both signs present, not constant.
- Chance: mixed outcomes at `p = 0.5`; `p = 0` is always false and `p = 1`
  always true.
- Normal: not constant, both signs present, mean in a broad sanity band.
- Jitter (`Between(-a, a)`): bounded in `[-0.05, 0.05)`, not all zero.
- Spike: mostly zeros, with spikes of both signs.
- Drift (a sum of `Normal` steps): changes over time and repeats for the same
  seed.
- Streams: the same seed reproduces a sequence; a different seed diverges;
  the same stream and index repeat a draw; the next index differs; forks with
  different names differ.
- Independence: turning the jitter off leaves the spikes exactly where they
  were, and a spike at a low probability is the same spike at a higher one.
  Random v1 could promise neither.

## Recorded outputs

Regenerated on 2026-10-03 with Random v2. `m2_final_verdict.octagon` is
unchanged: all six checks pass. The two summaries hold different numbers
because the draws are different.

`m2_random_smoke_summary.octagon` records an observed probability of `0.6`
for 200 draws at `p = 0.5`. That is 120 successes, 2.8 standard deviations
from 100. It is one seed. Over 2000 seeds the same count has mean 100.15 and
standard deviation 6.95, against 100 and 7.07 expected.

## Verdict

**Random v2 is usable for P14 labs** from experiment code.

## History

Before 2026-10-03 this milestone characterized Random v1: `RngSeed`,
`RandFloat01`, `RandFloatRange`, `RandBernoulli`, `RandNormal`, and the
composites `Jitter`, `Spike` and `DriftStep`. It reseeded per sample to avoid
threading generator state across a package boundary, where assigning
`draw.Next` back to a variable failed with `expected Random.Rng, got Rng`.
Random v2 has no state to thread, and that problem is gone with it.
