# FM Brown-Noise Kalman M1

M1 is a diagnostic comparison of NoFilter, LowPass, FixedWhiteKalman, and AdaptiveAr1ColoredNoiseKalman under -12 dB brown-noise conditions.

## Settings

| key | value |
| --- | --- |
| sampleRate | 2000 |
| duration | 0.5 |
| messageHz | 25 |
| carrierHz | 250 |
| frequencyDeviationHz | 50 |
| inputSNRDb | -12 |
| seed | 12345 |
| whitenessLagCount | 1 |
| artifactRowGuardThreshold | 1000 |

## Modes

- DirectMessageBrownNoise
- PhaseDomainFmBrownNoise

## Metrics

| mode | method | outputSNRDb | nrmse | correlation | whitenessCost | lag1InnovationAutocorrelation | finalA | diagnosticLabel |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| DirectMessageBrownNoise | NoFilter | -2.2255749979045936 | 1.292048302728665 | 0.16530559170632125 | 9.149422694427303 | 0.9921779868737625 |  |  |
| DirectMessageBrownNoise | LowPass | -2.3163528560320885 | 1.3056225512904625 | 0.1476748767791902 | 8.988272272207544 | 0.9889910584204826 |  |  |
| DirectMessageBrownNoise | FixedWhiteKalman | -2.186782912711394 | 1.2862907473137837 | 0.17272805668582092 | 0.8618491645432041 | 0.6427568572252381 |  |  |
| DirectMessageBrownNoise | AdaptiveAr1ColoredNoiseKalman | -2.3138729519029515 | 1.3052498368396817 | 0.14816143171329338 | 0.33514324241056753 | 0.4715528351179375 | 0.99 | WhitenessOnly |
| PhaseDomainFmBrownNoise | NoFilter | -2.8384601297734875 | 1.3865100008865832 | 0.03879500871882527 | 4.53496727110105e-33 | -3.18291822754975e-17 |  |  |
| PhaseDomainFmBrownNoise | LowPass | -2.713604744322598 | 1.3667221634586868 | 0.06603526395353887 | 0.2900803017178723 | 0.3521794097259579 |  |  |
| PhaseDomainFmBrownNoise | FixedWhiteKalman | -2.649176843576948 | 1.3566219579766325 | 0.07978843156598518 | 0.04814857684399423 | -0.09842673913865566 |  |  |
| PhaseDomainFmBrownNoise | AdaptiveAr1ColoredNoiseKalman | -2.6516660176843097 | 1.357010790013904 | 0.07926085789108217 | 0.05144898270348701 | -0.023245399526932723 | -0.6980862765908904 | NoMeaningfulWin |

## Diagnostic interpretation

> **Warning:** Direct mode: adaptive can improve innovation whiteness without improving recovery over FixedWhiteKalman.
> Phase-domain mode: adaptive may show negligible outputSNR deltas and should remain diagnostic-only.
> M1b uses tolerance-aware labels: snrEpsilonDb=0.01 and whitenessRelativeEpsilon=0.01.

## M1b Output-Wiring Diagnostics

No exact output reuse detected between Adaptive and LowPass/FixedWhiteKalman in direct/phase modes.

| mode | leftMethod | rightMethod | exactEqual | rmsDifference | maxAbsDifference | correlation |
| --- | --- | --- | --- | --- | --- | --- |
| DirectMessageBrownNoise | NoFilter | LowPass | false | 0.11468028874282046 | 0.4791251827527044 | 0.993424215686932 |
| DirectMessageBrownNoise | LowPass | FixedWhiteKalman | false | 0.09625535334952538 | 0.4366244188433793 | 0.99536745347578 |
| DirectMessageBrownNoise | FixedWhiteKalman | AdaptiveAr1ColoredNoiseKalman | false | 0.09991235733705667 | 0.3953550192195628 | 0.9950087604256754 |
| DirectMessageBrownNoise | LowPass | AdaptiveAr1ColoredNoiseKalman | false | 0.04277579907228824 | 0.4395946049761118 | 0.9990851155068635 |
| PhaseDomainFmBrownNoise | NoFilter | LowPass | false | 0.7901598212464462 | 2.581763231891965 | 0.6878237284438931 |
| PhaseDomainFmBrownNoise | LowPass | FixedWhiteKalman | false | 0.32248025033091804 | 1.1662528854428733 | 0.9480032440732542 |
| PhaseDomainFmBrownNoise | FixedWhiteKalman | AdaptiveAr1ColoredNoiseKalman | false | 0.05956426350435917 | 0.3422285941250438 | 0.9982260492565922 |
| PhaseDomainFmBrownNoise | LowPass | AdaptiveAr1ColoredNoiseKalman | false | 0.3267802621714155 | 1.12357391161099 | 0.9466073301275911 |

## Artifacts

- m1_report.octagon
- m1_report.md
- metrics.csv
- metrics.json
- recovered_signals_sample.csv: skipped unless <=1000 rows
- innovations_sample.csv: skipped unless <=1000 rows

## Limitations

- Phase-domain FM path is not real carrier/IQ FM.
- M1 does not use real audio.
- Adaptive AR(1) model is diagnostic only.
- Whitening innovations does not guarantee better recovery.
- Message model may be too weak or mismatched.
- Phase-domain noise path can be partially whitened by differentiation.
