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
| compiledBoardSnapshot | Supported: interpreted+compiled for M0 scalar-board fields |

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
| firstRecovered | 3.098805654522869 |
| lastRecovered | -6.568273276967075 |
| firstInnovation | 4.802297018828295 |
| lastInnovation | 0.3267652339824352 |

## Limitations

> **Note:** Fixed white-Kalman baseline only in M3 first pass.
> Tiny deterministic direct-message equivalence case only.
> No adaptive AR(1) variant yet.
> No real carrier/IQ/FM receiver path in M3.
> No real audio pipeline in M3.
> No board arrays were used by design (arrays remain unsupported on board fields).

## Recommendation

| key | value |
| --- | --- |
| next | Add adaptive AR(1) Octomata variant reusing this scalar-board + BoardSnapshot architecture (compiled-supported for scalar board fields). |
