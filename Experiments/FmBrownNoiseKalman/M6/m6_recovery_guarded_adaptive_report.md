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
| scalarAdaptiveWin | 11 |
| windowedAdaptiveWin | 1 |
| guardedAdaptiveWin | 9 |
| guardedRecoveryOnly | 0 |
| guardedWhitenessOnly | 17 |
| guardedNoMeaningfulWin | 1 |

## Guard counters

| key | value |
| --- | --- |
| guardAcceptedTotal | 12561 |
| guardAttenuatedTotal | 749 |
| guardRejectedTotal | 13690 |
| guardTriggerTotal | 749 |

## Means

| key | value |
| --- | --- |
| scalarMeanDeltaOutputSNRDb | -0.016860697246958038 |
| windowedMeanDeltaOutputSNRDb | -0.018297180959784037 |
| guardedMeanDeltaOutputSNRDb | -0.019065477359533994 |
| scalarMeanWhitenessRatio | 0.5343315017946046 |
| windowedMeanWhitenessRatio | 0.929768192962623 |
| guardedMeanWhitenessRatio | 0.6005838206223174 |

## Interpretation

This M6 implementation is oracle-assisted synthetic design-lab work. It tests whether adding a recovery guard to scalar adaptation shifts the whitening-recovery tradeoff. It is not deployable as-is and not real FM/IQ/audio evidence.
