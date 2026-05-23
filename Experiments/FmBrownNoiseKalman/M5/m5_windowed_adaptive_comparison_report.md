# FM Brown-Noise Kalman M5

## Windowed AR(1) adaptive comparison

## Architecture

Octomata board remained scalar current-state only; recovered/innovation/aTrace/window estimates stayed external and were consumed as deterministic scalar stream input.

## Summary

| key | value |
| --- | --- |
| totalCases | 27 |
| scalarAdaptiveWin | 15 |
| windowedAdaptiveWin | 1 |
| scalarMeanDeltaOutputSNRDb | -0.10323719568627238 |
| windowedMeanDeltaOutputSNRDb | -0.01281063531133515 |
| scalarMeanWhitenessRatio | 0.4599244600302107 |
| windowedMeanWhitenessRatio | 0.9206405349794723 |
| windowedBetterRecovery | 10 |
| windowedBetterWhiteness | 0 |
| equivalencePassCount | 27 |

## Boundedness

> **Note:** Bounded 27-case sweep. No FM/IQ/audio receiver realism. Results should not be over-generalized.
