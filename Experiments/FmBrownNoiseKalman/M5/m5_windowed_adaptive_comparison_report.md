# FM Brown-Noise Kalman M5

## Windowed AR(1) adaptive comparison

## Architecture

Octomata board remained scalar current-state only; recovered/innovation/aTrace/window estimates stayed external and were consumed as deterministic scalar stream input.

## Summary

| key | value |
| --- | --- |
| totalCases | 27 |
| scalarAdaptiveWin | 11 |
| windowedAdaptiveWin | 1 |
| scalarMeanDeltaOutputSNRDb | -0.016860697246958038 |
| windowedMeanDeltaOutputSNRDb | -0.018297180959784037 |
| scalarMeanWhitenessRatio | 0.5343315017946046 |
| windowedMeanWhitenessRatio | 0.929768192962623 |
| windowedBetterRecovery | 11 |
| windowedBetterWhiteness | 1 |
| equivalencePassCount | 27 |

## Boundedness

> **Note:** Bounded 27-case sweep. No FM/IQ/audio receiver realism. Results should not be over-generalized.
