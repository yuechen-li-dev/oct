# FM Brown-Noise Kalman M3

## Real Octomata scalar-board Kalman representation using BoardSnapshot

## Goal

| key | value |
| --- | --- |
| goal | Can a Kalman-style estimator loop be expressed as explicit Octomata flow states while matching a procedural baseline on a deterministic tiny case? |

## Representation

| key | value |
| --- | --- |
| representation | RealOctomataScalarBoardWithBoardSnapshot |
| boardObservation | BoardSnapshot(machine)! read-only observation |
| arraysPlacement | Recovered/innovation arrays are external accumulators |
| compiledBoardSnapshot | Deferred: interpreted-supported, compiled-not-yet |

## State-machine phases

- Initialize
- Predict
- Observe
- ComputeGain
- Correct
- Record
- Advance
- Done

## Equivalence metrics

| key | value |
| --- | --- |
| tolerance | 1e-09 |
| maxRecoveredAbsDiff | 0 |
| maxInnovationAbsDiff | 0 |

## Trace summary

| key | value |
| --- | --- |
| doneReached | true |
| sampleCount | 1000 |
| tickCount | 6003 |
| recoveredCount | 1000 |
| innovationCount | 1000 |
| firstState | Initialize |
| lastState | Done |
| firstRecovered | -0.6862324838892547 |
| lastRecovered | 0.6543966399001935 |
| firstInnovation | -1.0634717304050871 |
| lastInnovation | 0.07280233233753985 |

## Limitations

> **Note:** Fixed white-Kalman baseline only in M3 first pass.
> No board arrays were used by design.
> Compiled BoardSnapshot support is intentionally deferred.

## Recommendation

| key | value |
| --- | --- |
| next | Add adaptive AR(1) Octomata variant reusing this scalar-board + BoardSnapshot architecture once compiled BoardSnapshot support lands. |
