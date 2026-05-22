# FM Brown-Noise Kalman M4b Findings

## Scope
M4b is a bounded 27-case sweep (3 seeds × 3 input SNR levels × 3 message frequencies) for the scalar incremental adaptive AR(1) estimator introduced in M4.

## Question
Does scalar incremental adaptive AR(1) generally improve recovered-message fidelity, residual whiteness, both, or neither versus the fixed white-noise Kalman baseline?

## Result summary
See `sweep_summary.json` and `m4b_scalar_adaptive_sweep_report.md` for exact counts and means.

## Interpretation
Across this toy direct-message brown-noise setup, scalar incremental adaptation is expected to show stronger residual-whiteness effects than robust recovered-SNR gains. M4b quantifies how often that pattern appears versus true joint wins.

## Regime notes
- Check grouped tables by `InputSNRDb` and `MessageHz` in the report for where wins concentrate or vanish.
- `FinalA` and `ClampCount` indicate whether adaptation pushed toward strong lag-1 correlation and whether clamp saturation was active.

## Limitations
- 27 deterministic cases only.
- No IQ/carrier/FM receiver realism.
- No robustness theorem.
- Direct-message brown-noise mode only.

## Recommendation
If recovery gains remain inconsistent while whiteness improves, prioritize a next pass with windowed adaptation using external innovation history, while preserving scalar-board state architecture. In parallel, plan a realism pass with FM/IQ receiver dynamics.
