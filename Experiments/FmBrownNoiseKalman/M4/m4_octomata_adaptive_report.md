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
| fixedOutputSNRDb | -12.130953357228389 |
| adaptiveOutputSNRDb | -12.1218669837822 |
| deltaOutputSNRDb | 0.009086373446189455 |
| fixedWhitenessCost | 0.8698203626951433 |
| adaptiveWhitenessCost | 0.3555717282901707 |
| whitenessRatio | 0.408787542278294 |
| label | WhitenessOnly |
| finalA | 0.9696708109653571 |

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
| firstInnovation | -1.0634717304050871 |
| lastInnovation | -0.16097276888250633 |
| firstRecovered | -0.6862324838892547 |
| lastRecovered | 1.7743488085160892 |
| firstA | 0 |
| finalA | 0.9696708109653571 |
| clampCount | 0 |

## Limitations

> **Note:** M4 uses scalar incremental lag-1 adaptation a <- clamp(a + learningRate*e[k-1]*e[k]) for scalar-board visibility.
> Shared adaptive windowed update (window=64) is a different variant and is explicitly not claimed as equivalent.

## Next recommendation

| key | value |
| --- | --- |
| next | Try windowed adaptation with external innovation history in driver while preserving scalar-only board fields. |
