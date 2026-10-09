# FM Brown-Noise Kalman M2a

M2a tests known-frequency oscillator message-state diagnostics versus M1 position/velocity filtering.

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

## Method descriptions

- NoFilter
- LowPass
- FixedWhiteKalman
- AdaptiveAr1ColoredNoiseKalman
- OscillatorKalman
- OscillatorAdaptiveAr1Kalman

## Metrics

| mode | method | outputSNRDb | nrmse | correlation | whitenessCost | lag1InnovationAutocorrelation | finalA | diagnosticLabel |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| DirectMessageBrownNoise | NoFilter | -2.2255749979045936 | 1.292048302728665 | 0.16530559170632125 | 9.149422694427303 | 0.9921779868737625 |  |  |
| DirectMessageBrownNoise | LowPass | -2.3163528560320885 | 1.3056225512904625 | 0.1476748767791902 | 8.988272272207544 | 0.9889910584204826 |  |  |
| DirectMessageBrownNoise | FixedWhiteKalman | -2.186782912711394 | 1.2862907473137837 | 0.17272805668582092 | 0.8618491645432041 | 0.6427568572252381 |  |  |
| DirectMessageBrownNoise | AdaptiveAr1ColoredNoiseKalman | -2.3138729519029515 | 1.3052498368396817 | 0.14816143171329338 | 0.33514324241056753 | 0.4715528351179375 | 0.99 | WhitenessOnly |
| DirectMessageBrownNoise | OscillatorKalman | -1.718919014216599 | 1.2188379013437953 | 0.25721708512244335 | 9.599543406797823 | 0.997035168875719 |  |  |
| DirectMessageBrownNoise | OscillatorAdaptiveAr1Kalman | -1.851325402396853 | 1.2375600180149662 | 0.23422260090386954 | 8.715648289623195 | 0.994566164761471 | 0.99 | WhitenessOnly |
| PhaseDomainFmBrownNoise | NoFilter | -2.8384601297734875 | 1.3865100008865832 | 0.03879500871882527 | 4.53496727110105e-33 | -3.18291822754975e-17 |  |  |
| PhaseDomainFmBrownNoise | LowPass | -2.713604744322598 | 1.3667221634586868 | 0.06603526395353887 | 0.2900803017178723 | 0.3521794097259579 |  |  |
| PhaseDomainFmBrownNoise | FixedWhiteKalman | -2.649176843576948 | 1.3566219579766325 | 0.07978843156598518 | 0.04814857684399423 | -0.09842673913865566 |  |  |
| PhaseDomainFmBrownNoise | AdaptiveAr1ColoredNoiseKalman | -2.6516660176843097 | 1.357010790013904 | 0.07926085789108217 | 0.05144898270348701 | -0.023245399526932723 | -0.6980862765908904 | NoMeaningfulWin |
| PhaseDomainFmBrownNoise | OscillatorKalman | -2.3170783655849156 | 1.3057316110683583 | 0.14753247992671317 | 0.07865988821332207 | 0.03552682019391636 |  |  |
| PhaseDomainFmBrownNoise | OscillatorAdaptiveAr1Kalman | -2.173428422232734 | 1.2843146044336469 | 0.17526799841757362 | 0.07866722666469063 | 0.051436807969424125 | -0.5498052721538035 | RecoveryOnly |

## Diagnostic comparisons

| mode | leftMethod | rightMethod | exactEqual | rmsDifference | maxAbsDifference | correlation |
| --- | --- | --- | --- | --- | --- | --- |
| DirectMessageBrownNoise | FixedWhiteKalman | OscillatorKalman | false | 1.754428932321289 | 5.250988825635504 | -0.5390104392830076 |
| DirectMessageBrownNoise | OscillatorKalman | OscillatorAdaptiveAr1Kalman | false | 1.2674444645456215 | 3.4933134208627603 | 0.19679226464633118 |
| DirectMessageBrownNoise | OscillatorAdaptiveAr1Kalman | AdaptiveAr1ColoredNoiseKalman | false | 1.5225155728121516 | 3.9870872812533684 | -0.15902683472775705 |
| PhaseDomainFmBrownNoise | FixedWhiteKalman | OscillatorKalman | false | 1.232552711582626 | 5.155846253316041 | 0.24040690658515615 |
| PhaseDomainFmBrownNoise | OscillatorKalman | OscillatorAdaptiveAr1Kalman | false | 0.13720435113898552 | 0.7354085522931749 | 0.9905874830142646 |
| PhaseDomainFmBrownNoise | OscillatorAdaptiveAr1Kalman | AdaptiveAr1ColoredNoiseKalman | false | 1.2355952004165127 | 4.987035720835311 | 0.23665225035383725 |

## M1 vs M2a interpretation

> **Info:** Oscillator adaptive labels compare only against OscillatorKalman.
> Tolerance policy is unchanged: snrEpsilonDb=0.01, whitenessRelativeEpsilon=0.01.

## Limitations

- No real audio or IQ/PLL receiver path.
- Single bounded tiny case only.

## Next recommended experiment

M2b: bounded message-model mismatch and larger case if cycle time remains acceptable.
