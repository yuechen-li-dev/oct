# FM Brown-Noise Kalman M4

## Octomata adaptive AR(1) scalar-board estimator loop

## Goal

| key | value |
| --- | --- |
| goal | Can adaptive AR(1) colored-noise Kalman estimation be represented as explicit Octomata states with visible adaptation while matching a procedural baseline on a deterministic tiny case? |

## Representation

| key | value |
| --- | --- |
| representation | RealOctomataScalarBoardAdaptiveAR1 |
| boardArrays | none |
| externalAccumulators | recovered/innovation/aTrace external via BoardSnapshot(machine)! |

## State-machine phases

- Initialize
- Predict
- Observe
- ComputeGain
- Correct
- AdaptNoiseModel
- Record
- Advance
- Done

## Procedural-vs-Octomata adaptive equivalence

| key | value |
| --- | --- |
| tolerance | 1e-09 |
| maxRecoveredAbsDiff | 0 |
| maxInnovationAbsDiff | 0 |
| maxATraceAbsDiff | 0 |
| finalADiff | 0 |
| clampCountProcedural | 0 |
| clampCountOctomata | 0 |

## Fixed-vs-adaptive science comparison

| key | value |
| --- | --- |
| fixedOutputSNRDb | -12.055415448531248 |
| adaptiveOutputSNRDb | -12.088284342616662 |
| deltaOutputSNRDb | -0.03286889408541427 |
| fixedWhitenessCost | 0.8618491645432044 |
| adaptiveWhitenessCost | 0.304157183711639 |
| whitenessRatio | 0.3529123148509344 |
| label | WhitenessOnly |
| finalA | 0.9870536688383321 |

## Trace summary

| key | value |
| --- | --- |
| doneReached | true |
| sampleCount | 1000 |
| ticks | 7003 |
| initialize | 1 |
| predict | 1001 |
| observe | 1000 |
| computeGain | 1000 |
| correct | 1000 |
| adaptNoiseModel | 1000 |
| record | 1000 |
| advance | 1000 |
| done | 1 |
| firstState | Initialize |
| lastState | Done |
| recoveredCount | 1000 |
| innovationCount | 1000 |
| aTraceCount | 1000 |
| firstInnovation | 4.802297018828295 |
| lastInnovation | 0.019945107942414886 |
| firstRecovered | 3.098805654522869 |
| lastRecovered | -5.738415361203399 |
| firstA | 0 |
| finalA | 0.9870536688383321 |
| clampCount | 0 |

## Limitations

> **Note:** M4 uses scalar incremental lag-1 adaptation a <- clamp(a + learningRate*e[k-1]*e[k]) for scalar-board visibility.
> Shared adaptive windowed update (window=64) is a different variant and is explicitly not claimed as equivalent.

## Next recommendation

| key | value |
| --- | --- |
| next | Try windowed adaptation with external innovation history in driver while preserving scalar-only board fields. |
