# M6 Findings

## Scope

Bounded 27-case deterministic direct-message brown-noise sweep with fixed, scalar, windowed, and recovery-guarded scalar policies.

## Guarded policy

| key | value |
| --- | --- |
| recoveryWindowSize | 32 |
| epsRecovery | 0.005 |
| guardScale | 0.5 |
| clampRange | [-0.99,0.99] |

## Label counts

| key | value |
| --- | --- |
| scalarAdaptiveWin | 15 |
| windowedAdaptiveWin | 1 |
| guardedAdaptiveWin | 15 |
| guardedRecoveryOnly | 0 |
| guardedWhitenessOnly | 12 |
| guardedNoMeaningfulWin | 0 |

## Guard counters

| key | value |
| --- | --- |
| guardAcceptedTotal | 12269 |
| guardAttenuatedTotal | 1305 |
| guardRejectedTotal | 13426 |
| guardTriggerTotal | 1305 |

## Means

| key | value |
| --- | --- |
| scalarMeanDeltaOutputSNRDb | -0.10323719568627238 |
| windowedMeanDeltaOutputSNRDb | -0.01281063531133515 |
| guardedMeanDeltaOutputSNRDb | -0.1087721568272463 |
| scalarMeanWhitenessRatio | 0.4599244600302107 |
| windowedMeanWhitenessRatio | 0.9206405349794723 |
| guardedMeanWhitenessRatio | 0.4998985063737361 |

## Interpretation

This M6 implementation is oracle-assisted synthetic design-lab work. It tests whether adding a recovery guard to scalar adaptation shifts the whitening-recovery tradeoff. It is not deployable as-is and not real FM/IQ/audio evidence.
