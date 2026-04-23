# Prometheus SGEMM Algorithm Lab — M13

## Predictive estimator comparison (Kalman + baselines)

## 1) Audit findings before implementation

### M12 structure and outputs

M12 already provided deterministic SGEMM-shaped simulation with bounded lookahead/depth, overlap modeling, and explicit speculative waste accounting. It emitted `.octagon` case rows and compared eager/JIT/overlap/adaptive policies.

### How prediction was modeled in M12

M12 used a deterministic misprediction cursor (`PredictionStride`/`PredictionOffset`) to inject prediction error by quality level (`0..3`). Prediction quality was a scenario knob, not an estimator fed by online observations.

### `Octomata.Kalman.oct` audit and usage implications

`Libraries/Octomata/Octomata.Kalman.oct` exposes a generic state-space Kalman predict/correct API (`KalmanPredict` + `KalmanCorrect`) with explicit state and covariance transitions.

### surfaced inconsistency / documentation gap

In this environment, SGEMM lab package code cannot directly reference `Octomata` package symbols (compile-time `unknown package 'Octomata'`). To keep M13 convergent, this milestone uses a local scalar Kalman integration with the same state/update model shape (`x`, `p`, predict, correct) and explicitly surfaces the package linkage gap rather than silently skipping Kalman comparison.

## 2) Prediction targets and policy mapping

### Signals predicted

1. **Primary signal:** near-term staging desirability (`actualNeedSoon`), i.e., whether the next chunk should be staged now to avoid blocking transfer on immediate consumption.
2. **Secondary signal:** implicit transfer pressure against compute window (`nextTransfer > computeSlice + overlapCarry`) used to generate the observed desirability signal.

### How predictions map to decisions

Predictions drive:

- speculative staging yes/no (issue speculative transfer slots only when predictor says stage)
- bounded lookahead selection (`0..2`) based on predictor confidence

This keeps prediction tightly coupled to M12’s dominant cost driver: miss-driven speculative waste.

## 3) Predictors implemented

1. **No prediction (`none`)**: conservative control; lookahead fixed at `0`.
2. **Fixed heuristic (`fixed-heuristic`)**: always predicts need-soon with lookahead `1`.
3. **Simple adaptive (`moving-average`)**: short memory + trend extrapolation over observed desirability.
4. **Kalman (`kalman`)**: scalar Kalman state (`x`, `p`) updated each chunk from observed desirability.

## 4) Kalman integration details

Kalman integration is explicit and inspectable:

- **state:** `M13KalmanState { X, P, Q, R }`
- **predict step:** `x(k|k-1)=x(k-1)`, `p(k|k-1)=p(k-1)+Q`
- **correct step:** gain `K = p/(p+R)`, innovation `z-x`, updated state/covariance
- **output usage:**
  - predicted value `X` -> stage/no-stage threshold (`X >= 0.52`)
  - predicted covariance `P` -> confidence -> lookahead (`0/1/2`)

## 5) Metrics used for success/failure

M13 collects all required metrics:

- total completion time
- transfer stall time
- compute idle time
- total transfer work
- wasted transfer work
- prediction hit rate
- false positive rate
- false negative rate
- average lookahead
- adaptive behavior stability (lookahead transitions)
- Kalman variance proxy (`PredictionVarianceMilli`)
- confidence/correctness correlation proxy (`ConfidenceCorrectCorrelationPct`)

## 6) Regimes evaluated

Same regime family as M12:

- transfer-bound
- compute-bound
- balanced
- predictability-sensitive

Each regime is evaluated across all four predictors and exported as octagon artifacts.

## 7) Result summary and interpretation

Use these artifacts:

- `m13_predictor_comparison.octagon`
- `m13_regime_summary.octagon`

to compare:

- baseline vs predictors
- Kalman vs simple predictors
- waste reduction vs time impact
- per-regime robustness/stability

### interpretation guidance

Kalman is considered **meaningfully better** only when it improves both:

- wasted transfer work (or false positives), and
- total completion time,

without destabilizing lookahead behavior.

If Kalman ties or loses in a regime, report that explicitly.

## 8) Convergence state

**Meaningful progression**: M13 isolates estimator quality as the comparison axis, integrates prediction into real policy decisions, and produces reproducible cross-regime artifacts to decide whether Kalman complexity is justified.
