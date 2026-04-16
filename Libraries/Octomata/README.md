# Octomata Library Family

Reusable discrete-time control, estimation, and behavioral coordination
components layered on top of the Octomata runtime. The Octomata library
is the control library for oct — traditional PID, state-space, Kalman
estimation, and FSM/utility-based coordination all live here as a unified
family rather than as competing ecosystems.

## Design principles

All components follow the same authoring style:

- **Pure stateless update functions.** No hidden memory. State is explicit,
  carried in records, and threaded manually across calls — the same pattern
  as `PIDUpdate`, `KalmanPredict`, and `StabilizeValue`.
- **Composable by design.** A Kalman corrected state feeds naturally into
  a PID setpoint. A `CandidateSet` built from `LinearScore` + `WeightedScore`
  feeds naturally into `SelectBest`. Components compose because they share
  the same explicit-state contract.
- **Fallible where inputs can be invalid.** Functions that can fail return
  `T ! Error` and validate their inputs explicitly.
- **`for` loops for structured iteration, `while` for condition-driven loops.**
  Follows the oct language reference style guidance throughout.

## Components

### `Octomata.Filters`

Discrete-time signal conditioning.

- `LowPassUpdate(alpha, input, previousOutput)` — first-order exponential
  smoothing. `alpha=0` holds previous output, `alpha=1` passes through.

### `Octomata.PID`

Discrete-time PID controller.

- `PIDUpdate(kp, ki, kd, setpoint, measurement, integralState, previousError, dt)`
  — one tick of proportional-integral-derivative control with explicit state.
  Returns `PIDUpdateResult { Output, IntegralState, PreviousError }`.

### `Octomata.AntiWindup`

PID extensions and signal conditioning primitives.

- `ClampedPIDUpdate(...)` — PID with output saturation limits and conditional
  integration anti-windup. The integral freezes when output is saturated and
  winding in the wrong direction, preventing large overshoot on setpoint changes.
- `Clamp(value, min, max)` — saturate a value to a range.
- `Deadband(value, threshold)` — zero out signals within a noise band.
- `RateLimit(current, target, maxRatePerTick)` — limit how fast a value can
  change per tick, protecting actuators from step commands.

### `Octomata.StateSpace`

Discrete-time linear state-space systems.

- `StateSpaceUpdate(A, B, C, D, x, u, n, m, p)` — one tick of
  `x[k+1] = A*x + B*u`, `y = C*x + D*u`. Returns next state and output.
- `MatVecMulFlat`, `VecAdd`, `VecSub`, `VecScale` — flat matrix/vector
  helpers used by state-space and Kalman.

A double integrator (kinematic position/velocity model) is the canonical
test: 10 ticks at 10 m/s² gives v=10, x≈5.

### `Octomata.Kalman`

Discrete-time Kalman filter.

- `KalmanPredict(A, B, Q, x, P, u, n, m)` — predict step:
  `x_pred = A*x + B*u`, `P_pred = A*P*A^T + Q`.
- `KalmanCorrect(H, R, xPred, pPred, z, n, p)` — correct step with
  Joseph-form covariance update for numerical stability. Supports scalar
  (p=1) and 2D (p=2) observation spaces in M0.
- Flat matrix utilities: `MatMulFlat`, `MatTransposeFlat`, `MatAddFlat`,
  `MatSubFlat`, `IdentityFlat`.

### `Octomata.Arbitration`

Scored arbitration over runtime-sized candidate sets.

`when utility` and `when policy` require statically written cases. This
module provides the same selection semantics over dynamically built arrays —
for coordination patterns where the number of candidates is determined at
runtime (N active operations, N agents, N resource requests).

- `MakeCandidateSet(ids, scores, active)` — build a candidate set from
  parallel arrays. Validates equal lengths.
- `SelectBest(cs)` — one-shot highest-score selection. Ties resolve to
  lowest index, matching `when utility` language behavior.
- `SelectWithCommitment(cs, commitment, hysteresis, minCommit)` — mirrors
  `when policy` semantics: challenger must beat current score by more than
  `hysteresis`, and current is held for at least `minCommit` ticks.
- `LinearScore(value, inputMin, inputMax, scoreMin, scoreMax)` — map a
  Float measurement into a score range. Returns `Float` for intermediate use;
  caller constructs `Int` scores for `MakeCandidateSet`.
- `InvertScore(score, maxScore)` — flip score order (prefer lower).
- `WeightedScore(scoreA, weightA, scoreB, weightB)` — combine two scores
  with integer weights.

### `Octomata.Commitment`

Standalone commitment and stabilization primitives. These externalize
patterns that appear repeatedly in board memory usage — making them
testable in isolation and usable outside flows.

- `StabilizeValue(next, commitment, hysteresis, minCommit)` — debounce
  a scalar Int decision against transient flips. Same stability semantics as
  `when policy` but for a single value rather than an arbitration set.
- `TickCooldown(remaining)` / `StartCooldown(ticks)` — explicit cooldown
  timer. Makes the `CooldownTicks: Int` board pattern testable.
- `DetectEdge(signal, previous)` — rising/falling edge detection on Bool
  signals. Fires only on the transition tick.
- `UpdateLatch(current, set, reset)` — fault latch with reset-priority
  semantics. Models fault latches, acknowledge flags, one-shot triggers.

## Composition patterns

### Filter into controller

```oct
let filtered = LowPassUpdate(0.3, rawMeasurement, prevFiltered)!
let pid = PIDUpdate(kp, ki, kd, setpoint, filtered, integral, prevError, dt)!
```

### Estimator into controller

```oct
// Predict
let pred = KalmanPredict(A, B, Q, x, P, u, n, m)!
// Correct with new measurement
let corr = KalmanCorrect(H, R, pred.XPred, pred.PPred, z, n, p)!
// Use corrected state as measurement for PID
let pid = PIDUpdate(kp, ki, kd, setpoint, corr.X[0], integral, prevError, dt)!
```

### Runtime arbitration

```oct
// Build candidate set from dynamic inputs
let scores = [
    WeightedScore(urgency1, 2, proximity1, 1)!,
    WeightedScore(urgency2, 2, proximity2, 1)!,
    WeightedScore(urgency3, 2, proximity3, 1)!
]
let cs = MakeCandidateSet([1, 2, 3], scores, [op1Active, op2Active, op3Active])!
let winner = SelectWithCommitment(cs, commitment, hysteresis, minCommit)
commitment = winner.Next
```

### Stabilized mode with edge detection

```oct
let stable = StabilizeValue(computedMode, modeCommitment, 0, 3)!
let edge = DetectEdge(stable.Changed, prevChanged)
modeCommitment = stable.Next
// edge.Rising fires exactly once on the tick a new mode is accepted
```

## Scope and honest limitations

**M0 scope (current):** All values dimensionless `Float` or `Int`.
Kalman supports p=1 and p=2 observation dimensions only. No continuous-time
math, transfer functions, LQR, or MPC.

**Intentional non-goals:** This is not a control framework — there is no
scheduler, no topology, no wiring abstraction. Components are functions.
Octomata flows are the scheduling substrate. The library enriches Octomata
rather than replacing it.

**`when policy` vs `SelectWithCommitment`:** For statically known candidate
sets inside a flow, `when policy` is idiomatic oct and should be preferred.
`SelectWithCommitment` fills the gap when the candidate set is runtime-sized
or when you need commitment behavior outside a flow context.

**Dimensional types:** Future milestones will add dimensioned variants
(`Float<m/s>`, `Float<Pa>`) once the matrix/array type system supports them.
The physics is correct now; the units will be enforced later.

## Test coverage

92 contracts across 7 files. All passing.
