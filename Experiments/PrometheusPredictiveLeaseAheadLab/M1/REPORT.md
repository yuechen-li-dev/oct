# P15 M1 — Predictive Lease-Ahead Bounded Lookahead Simulation

## 1. Problem statement
M1 evaluates bounded predictive lease-ahead behavior before native runtime integration. The core control question is whether lookahead depths `{0,1,2}` can improve opportunity capture without unacceptable miss/waste under noisy, warmup, or unsafe conditions.

## 2. Relation to P15 M0
This lab directly instantiates the M0 contract: physical state is truth, shadow prediction is hypothesis, and correction events reconcile the two at maturity. M1 remains simulation-only (no native state changes).

## 3. Scenario definitions
Seven scenarios were simulated:
- stable
- warmup
- low-confidence
- step-change
- prediction-miss
- resource-pressure
- runtime-unsafe

## 4. Policy candidate definitions
- **A baseline**: always depth 0.
- **B greedy**: depth 2 when confidence gate passes.
- **C conservative**: depth 1 only at high confidence and no recent miss.
- **D adaptive Dominatus**: depth `{0,1,2}` with confidence + correction + hard safety gates.

## 5. State model
The lab uses explicit records:
- `FilteredEvidenceSample`
- `PhysicalObservation`
- `PredictionEntry`
- `CorrectionEvent`
- `FutureLeaseState`

Transition shape follows Dominatus framing:
`PredictorState-like (confidence/recent miss/depth) + filtered evidence + physical observation -> depth + lease behavior + correction impact`.

## 6. Correction rules
At prediction maturity:
- predicted ready + actual not ready -> miss penalty, confidence decrement, reduced depth tendency.
- predicted ready + actual ready -> confidence increment and depth recovery.
- hard unsafe gates -> fallback behavior and depth 0.

## 7. Future lease lifecycle
M1 uses a bounded simplified lifecycle for policy shape validation:
`None -> Requested -> Granted -> Matured`,
and gate-driven `Requested/Granted -> Cancelled` or blocked/denied behavior under resource pressure/unsafe state.

## 8. Metrics/scoring
Per scenario/policy:
- opportunity score
- safety score
- miss penalty
- future lease waste
- fallback rate
- thrash count
- transparent composite score

## 9. Computed results
Observed pattern from deterministic simulation:
- Stable: adaptive and greedy capture opportunity; baseline does not.
- Warmup and low-confidence: adaptive correctly holds depth 0.
- Prediction-miss: greedy overcommits and incurs higher miss/waste; adaptive dampens depth with penalties.
- Resource/unsafe: adaptive blocks actuation via hard gates.

## 10. Final recommendation
Recommend adaptive policy for M2 starter contract:
- depth 0 when `confidence < 0.45`, warmup, invalid evidence, or hard gate active.
- depth 1 when `confidence >= 0.45` and no hard gate.
- depth 2 when `confidence >= 0.75`, low outliers, and no recent miss.
- correction penalties decrease confidence and reduce depth; recovery allowed after stable correct maturities.

## 11. M2 native implementation contract (proposed)
Required native fields (bounded):
- predictor confidence
- lookahead depth `{0,1,2}`
- recent miss / correction counters
- delayed prediction ring (fixed capacity)
- future lease lifecycle counters and last state
- fallback reason + stale marker

## 12. Limitations
- Synthetic deterministic traces only; no hardware traces.
- Simplified lease lifecycle and correction semantics.
- `PredictionEntry`/`CorrectionEvent` are represented structurally but not as a full per-tick persisted ring in this first lab.

## 13. Next milestone recommendation
M2 should implement bounded native predictor state + diagnostic-first future lease seam, preserving synchronous correction and hard safety gates before any aggressive actuator path.

## Inconsistency notes
- P15 M0 describes explicit delayed prediction ring mechanics with richer fields than the current M1 simplified simulation model; this is a known fidelity gap to close in M2.
- Existing experiment code style in repository often uses compact one-line constructs; this lab follows language constructs from `Language/reference` while keeping compact sections where already idiomatic in experiments.
