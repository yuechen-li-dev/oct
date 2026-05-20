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
| DirectMessageBrownNoise | NoFilter | -1.594275625407705 | 1.201472351716887 | 0.2782320940285064 | 8.50543725313827 | 0.9868585875758529 |  |  |
| DirectMessageBrownNoise | LowPass | -1.6762940889562041 | 1.2128712578868324 | 0.26447165589453975 | 8.211386903530668 | 0.9804514806564403 |  |  |
| DirectMessageBrownNoise | FixedWhiteKalman | -1.5381952877905578 | 1.1937400491502101 | 0.28749234752600467 | 0.8698203626951475 | 0.6762288595592513 |  |  |
| DirectMessageBrownNoise | AdaptiveAr1ColoredNoiseKalman | -1.6763087123538822 | 1.2128732998554053 | 0.26446917924746327 | 0.3745165803556255 | 0.5163976461338532 | 0.99 | WhitenessOnly |
| DirectMessageBrownNoise | OscillatorKalman | -1.5519200770606425 | 1.195627798404442 | 0.2852370838398468 | 9.31198954227127 | 0.9941521692207667 |  |  |
| DirectMessageBrownNoise | OscillatorAdaptiveAr1Kalman | -2.4846396882532353 | 1.3311652884748346 | 0.11399948737808573 | 6.47096818792177 | 0.9552239212611237 | 0.99 | WhitenessOnly |
| PhaseDomainFmBrownNoise | NoFilter | -2.936735223137492 | 1.4022865261936657 | 0.016796249225887197 | 2.226793986229638e-39 | -2.1605026745650442e-20 |  |  |
| PhaseDomainFmBrownNoise | LowPass | -2.795693551934556 | 1.3797000422603376 | 0.048213896691511146 | 0.2767807866705221 | 0.38288641927380657 |  |  |
| PhaseDomainFmBrownNoise | FixedWhiteKalman | -2.8512500538173944 | 1.3885531330103777 | 0.03596009840160682 | 0.059043940177654256 | -0.001220908266822723 |  |  |
| PhaseDomainFmBrownNoise | AdaptiveAr1ColoredNoiseKalman | -2.851239694159142 | 1.388551476884414 | 0.035962398019130606 | 0.06070230528213562 | 0.0032294711315824956 | 0.11993182783735826 | NoMeaningfulWin |
| PhaseDomainFmBrownNoise | OscillatorKalman | -2.7271855723628664 | 1.368860774208178 | 0.06311009041522253 | 0.09645004427804055 | 0.12230965994903058 |  |  |
| PhaseDomainFmBrownNoise | OscillatorAdaptiveAr1Kalman | -2.2545559155327637 | 1.2963664882789931 | 0.1597169640319184 | 0.050276282630322806 | 0.056314839991092464 | 0.6875217787963424 | AdaptiveWin |

## Diagnostic comparisons

| mode | leftMethod | rightMethod | exactEqual | rmsDifference | maxAbsDifference | correlation |
| --- | --- | --- | --- | --- | --- | --- |
| DirectMessageBrownNoise | FixedWhiteKalman | OscillatorKalman | false | 1.6423890835351949 | 3.8264493072541983 | -0.34872095085778665 |
| DirectMessageBrownNoise | OscillatorKalman | OscillatorAdaptiveAr1Kalman | false | 1.4534291124484207 | 3.293288607079099 | -0.05622809245630317 |
| DirectMessageBrownNoise | OscillatorAdaptiveAr1Kalman | AdaptiveAr1ColoredNoiseKalman | false | 1.3650962810201595 | 3.6000616754026495 | 0.06825607177246308 |
| PhaseDomainFmBrownNoise | FixedWhiteKalman | OscillatorKalman | false | 1.2595860003449497 | 5.009031044745284 | 0.20672155386750554 |
| PhaseDomainFmBrownNoise | OscillatorKalman | OscillatorAdaptiveAr1Kalman | false | 0.5685650362658787 | 2.4058953438873427 | 0.8383668997679912 |
| PhaseDomainFmBrownNoise | OscillatorAdaptiveAr1Kalman | AdaptiveAr1ColoredNoiseKalman | false | 1.3329672523119285 | 5.190006426335578 | 0.11159915213199313 |

## M1 vs M2a interpretation

> **Info:** Oscillator adaptive labels compare only against OscillatorKalman.
> Tolerance policy is unchanged: snrEpsilonDb=0.01, whitenessRelativeEpsilon=0.01.

## Limitations

- No real audio or IQ/PLL receiver path.
- Single bounded tiny case only.

## Next recommended experiment

M2b: bounded message-model mismatch and larger case if cycle time remains acceptable.
