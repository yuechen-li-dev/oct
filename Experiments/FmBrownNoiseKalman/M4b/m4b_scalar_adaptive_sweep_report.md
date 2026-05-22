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
| adaptiveWin | 15 |
| recoveryOnly | 0 |
| whitenessOnly | 12 |
| noMeaningfulWin | 0 |
| meanDeltaOutputSNRDb | -0.10323719568627238 |
| meanWhitenessRatio | 0.4599244600302107 |
| finalAMin | 0.8796228912983445 |
| finalAMax | 0.99 |
| finalAMean | 0.9727125543420385 |
| clampTotal | 12512 |

## Interpretation

> **Note:** This is a bounded 27-case toy sweep on direct-message brown noise without FM/IQ realism.
> Results indicate whether whitening gains are systematic and whether they co-occur with recovery gains under the M4 scalar incremental adaptation semantics.
