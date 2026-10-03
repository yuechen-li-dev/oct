# FM Brown-Noise Kalman M4b

## Focused scalar incremental adaptive AR(1) sweep

## Question

Does scalar incremental adaptive AR(1) generally improve residual whiteness, recovered-message fidelity, both, or neither across a bounded deterministic grid?

## Sweep grid

| key | value |
| --- | --- |
| seeds | 12345,23456,34567 |
| inputSNRDb | -18,-12,-6 |
| messageHz | 10,25,50 |
| sampleRate | 2000 |
| duration | 0.5 |
| sampleCount | 1000 |
| totalCases | 27 |

## Label policy

| key | value |
| --- | --- |
| snrEpsilonDb | 0.01 |
| whitenessRelativeEpsilon | 0.01 |
| labels | AdaptiveWin/RecoveryOnly/WhitenessOnly/NoMeaningfulWin |

## Overall results

| key | value |
| --- | --- |
| adaptiveWin | 11 |
| recoveryOnly | 0 |
| whitenessOnly | 16 |
| noMeaningfulWin | 0 |
| meanDeltaOutputSNRDb | -0.016860697246958038 |
| meanWhitenessRatio | 0.5343315017946046 |
| finalAMin | 0.8937193102307632 |
| finalAMax | 0.99 |
| finalAMean | 0.9853669893480004 |
| clampTotal | 11369 |

## Interpretation

> **Note:** This is a bounded 27-case toy sweep on direct-message brown noise without FM/IQ realism.
> Results indicate whether whitening gains are systematic and whether they co-occur with recovery gains under the M4 scalar incremental adaptation semantics.
