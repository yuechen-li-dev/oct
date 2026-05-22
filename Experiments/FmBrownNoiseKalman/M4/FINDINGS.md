# FM Brown-Noise Kalman M4 Findings

## Question

M4 asked whether an adaptive AR(1)-style Kalman estimator can be represented as a **real Octomata scalar-board loop** with adaptation as an explicit phase, while remaining numerically equivalent to a procedural adaptive baseline on a tiny deterministic case.

## Representation result

M4 answers this representation question with a clear **yes** for the tested variant.

- The estimator is implemented as explicit Octomata states: `Initialize`, `Predict`, `Observe`, `ComputeGain`, `Correct`, `AdaptNoiseModel`, `Record`, `Advance`, `Done`.
- `AdaptNoiseModel` is a first-class visible state, not hidden in a monolithic numeric loop.
- `BoardSnapshot(machine)!` is used after `Step(machine)` to export scalar board values into external arrays (`recovered`, `innovation`, `aTrace`).
- The board remains scalar/current-state only (no board arrays).

Architecturally, this preserves the M3 design pattern and demonstrates that adaptive estimation logic can remain inspectable without changing Octomata runtime semantics.

## Equivalence result (procedural adaptive vs Octomata adaptive)

On the deterministic M4 tiny case, equivalence is exact within tolerance:

- tolerance: `1e-9`
- max recovered diff: `0`
- max innovation diff: `0`
- max a-trace diff: `0`
- finalA diff: `0`
- clamp count: procedural `0`, Octomata `0`

Interpretation: the Octomata state-machine representation is **numerically faithful** to the procedural adaptive implementation for this case. The representation itself did not perturb algorithm behavior.

## Science result (fixed vs adaptive in this tiny deterministic case)

### Measured values

- fixed outputSNRDb: `-12.130953357228389`
- adaptive outputSNRDb: `-12.1218669837822`
- delta outputSNRDb: `+0.009086373446189455 dB`
- fixed NRMSE: `1.1937400491502088`
- adaptive NRMSE: `1.2124871520622893`
- fixed whitenessCost: `0.8698203626951433`
- adaptive whitenessCost: `0.3555717282901707`
- whiteness ratio (adaptive/fixed): `0.408787542278294`
- fixed correlation: `0.28749234752600245`
- adaptive correlation: `0.26493745304046656`
- adaptive finalA: `0.9696708109653571`
- clamp count: `0`
- label: `WhitenessOnly`

### Interpretation

In this tiny deterministic case, adaptive behavior strongly improves innovation whiteness (whiteness cost drops by ~59%), but does **not** show a meaningful recovery/SNR win under the configured label threshold (`snrEpsilonDb = 0.01`). The SNR delta is positive but only `~0.0091 dB`, i.e., below the meaningful-win threshold. NRMSE and correlation are slightly worse for adaptive than fixed in this case.

So the most defensible reading is:

- adaptive helped residual whitening substantially,
- recovery quality did not clearly improve,
- and by this milestone’s policy this classifies as **WhitenessOnly**, not AdaptiveWin.

## What the data means

The core success of M4 is **representation fidelity** (adaptive Kalman structure made explicit in Octomata with exact procedural equivalence), not a broad claim of adaptive superiority.

Scientifically, this single-case result suggests the scalar incremental adaptation mechanism can fit colored residual structure (final `A ≈ 0.97` without clamp saturation), but that fit does not automatically translate to better recovered-message fidelity on this setup. This is plausible: reducing residual autocorrelation and improving message reconstruction are related but not identical objectives.

## Important caveat: scalar incremental vs windowed adaptation

M4 uses a scalar incremental lag-1 update:

`a <- clamp(a + learningRate * e[k-1] * e[k])`

Earlier/Shared adaptive logic includes a **windowed lag autocorrelation** update path (e.g., windowed estimation behavior used in prior milestones). Therefore:

- M4 should **not** be interpreted as an exact re-run of the earlier windowed adaptive science path.
- M4 is specifically evidence that an **Octomata-compatible scalar-board adaptive structure** can be represented and verified.

## Limitations

- single deterministic tiny case (DirectMessageBrownNoise, 1000 samples)
- no parameter sweep
- direct-message brown-noise setup only
- no IQ/carrier/receiver realism
- no real audio pipeline
- no robustness/generalization claim
- adaptation form restricted to scalar-board incremental update in this milestone

## Recommendation

Next step depends on objective:

1. **Representation objective:** keep this architecture and extend adaptive state/trace instrumentation while preserving scalar-board + external accumulators.
2. **Science objective:** run a focused M4b sweep (SNR, seeds, message bands) for scalar incremental adaptive vs fixed.
3. **Continuity-with-M2b objective:** add a windowed adaptation variant using external innovation history (still no board arrays), then compare against a windowed procedural baseline.

For now, M4 should be considered a strong representation milestone with a nuanced science outcome: **whiteness gains without a meaningful tiny-case recovery win**.
