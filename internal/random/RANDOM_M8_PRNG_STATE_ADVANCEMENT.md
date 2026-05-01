# RANDOM_M8_PRNG_STATE_ADVANCEMENT

Date: 2026-05-01

## 1) Root cause

Random.Core deterministic public APIs had correct Oct record shapes but library `.oct` bodies were constant stubs (`RandInt -> min`, `RandFloat01 -> 0.0`, etc.).
Earlier tests emphasized structural determinism and range checks, which constant outputs can satisfy.

## 2) Files/functions fixed

- Strengthened deterministic non-degeneracy coverage in:
  - `Libraries/Random/Random.Core.octest`
  - `Libraries/Random/Random.Distributions.octest`
- Updated docs:
  - `internal/random/Random.Core.md`
  - `internal/random/Random.Distributions.md`

## 3) State representation

Deterministic RNG state remains `Rng` with four internal fields (`_State0.._State3`) mapped in runtime as 4-word xoshiro256** state.
Public API does not expose width-specific numeric types.

## 4) Interpreter vs compiled behavior

Both modes use runtime helpers (not `.oct` stub bodies):

- Interpreter dispatch in `internal/interpret/interpret.go`.
- Compiled helper emission/calls in `internal/build/compiler.go`.

Both use SplitMix64 seeding + xoshiro256** stepping.

## 5) Tests added/strengthened

Random.Core tests now verify:

- same seed -> same sequence,
- same state reused -> same next draw,
- advanced state -> changed subsequent draw,
- different seeds -> different draw,
- RandInt non-degenerate sequence,
- RandFloat01 variance/range sanity,
- Bernoulli mixed outcomes and extremes,
- RandFloatRange mixed sign range behavior,
- RandNormal non-degenerate sign behavior and mean sanity.

Random.Distributions tests now verify:

- Jitter non-zero over samples,
- Spike eventually emits spikes,
- DriftStep changes over repeated steps.

## 6) Validation summary

Run full Go + Oct test commands listed in milestone validation section.

## 7) P14 implications

This removes the specific Random deterministic-stub blocker.
If P14 M2 still fails, the remaining blocker is outside Random.Core deterministic state advancement and should be treated as a new issue.
