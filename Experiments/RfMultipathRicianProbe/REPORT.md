# REPORT

## 1. Goal

Evaluate whether RF M1 enables richer deterministic channel modeling (shadowing + Rician + TDL) while keeping sequence composition readable and lightweight.

## 2. Baseline

Part A builds a 16-element baseline from free-space path loss and received power, then computes deterministic thermal-noise and SNR series. It establishes a clean monotonic reference:

- path loss increases with distance
- received power decreases with distance
- sequence shape is preserved across all baseline outputs

## 3. Shadowing

Part B applies deterministic log-normal shadowing offsets in dB to the baseline received-power sequence.

Observed effect:

- positive shadowing dB values reduce received power
- negative shadowing dB values increase received power
- behavior is elementwise and fully deterministic

This captures large-scale variation without introducing randomness.

## 4. Rician

Part C applies deterministic Rician fading to the shadowed sequence using the same scatter samples with two K factors:

- high K (LOS-dominant)
- low K (scatter-dominant / Rayleigh-like)

Observed effect:

- high-K gains remain closer to LOS-dominant behavior
- low-K gains deviate more and track Rayleigh-like behavior more closely
- output remains elementwise and shape-preserving

## 5. Multipath

Part D applies a tapped-delay-line (TDL) power channel with explicit delays and tap gains.

Observed effect:

- output length extends by max delay
- delayed taps add trailing energy
- sequence shape evolves across the tail due to delay spread

The toy TDL check shows exact deterministic per-sample contributions.

## 6. Composition

The experiment explicitly composes the full M1 chain as:

`path loss → shadowing → Rician → TDL`

in `BuildFullPipeline`, with each stage exposed as named records and validated against direct stage calls.

## 7. Observations

What feels clean:

- sequence-first helpers keep each step explicit
- deterministic dB/gain inputs make comparisons repeatable
- stage-by-stage records keep the richer model inspectable

What feels heavy:

- setting long deterministic input vectors (shadowing/scatter) is verbose
- sentinel-return conventions still require shape-aware tests

Overall, RF M1 still feels small and composable rather than simulator-heavy.

## 8. Conclusion

RF M1 succeeds for this scope: it adds meaningful realism (large-scale shadowing, LOS/scatter trade-offs, and delay spread) without breaking deterministic pipeline composition.

Next pressure point: better sequence-construction ergonomics (e.g., compact deterministic pattern builders), not larger channel frameworks.
