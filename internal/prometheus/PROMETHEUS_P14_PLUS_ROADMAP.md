# Prometheus P14+ Roadmap — Measurement, Prediction, Vendor Paths, and PVT

## Status at handoff

P13 established the first complete Prometheus acceleration-control spine:

- benchmark-only occupancy variant family exists and executed on real RTX 3070 hardware,
- hybrid control architecture exists: feedforward selector + Dominatus/HFSM lease controller + utility policy,
- lifecycle/promotion seam exists: benchmark/DVT/PVT/production eligibility are separate truths,
- DVT-2 proved the variant family correct on local NVIDIA hardware,
- diagnostics truth separation landed so future runs distinguish capability, recommendation, request, execution, usage, and measurement.

P13 should be treated as closed for the occupancy-variant bring-up path.

The next phases should not rush into production dispatch. The immediate problem is now evidence quality: before Prometheus predicts, reserves, prefetches, or tunes, it must know what measurement signal is safe to trust.

---

## Proposed progression

```text
P14 — Measurement filtering / evidence robustness
P15 — Predictive lease-ahead / Smith predictor / bounded actuator
P16 — Vendor-specific execution paths as additive capabilities
P17 — PVT evidence collection and score tuning across devices
```

This roadmap intentionally separates:

```text
measurement quality
prediction/control
backend capabilities
cross-device policy tuning
```

Do not collapse these phases. That is how systems start optimizing noise and then call it intelligence.

---

# P14 — Measurement Filtering / Evidence Robustness

## Purpose

P14 should answer:

```text
What measurement can Prometheus safely feed into future prediction and selection logic?
```

It should not answer:

```text
What is the best prefetch strategy?
Which variant is fastest?
Which backend should production dispatch use?
```

P14 is a signal-quality phase.

## Motivation

Raw GPU observations are not truth. They are:

```text
measurement = underlying signal + jitter + driver behavior + queue contention + thermal/clock variation + artifact noise
```

If Prometheus uses raw timings directly, the controller risks:

- oscillating between variants,
- overreacting to spikes,
- mistaking driver jitter for performance signal,
- tuning toward a local device/runtime artifact,
- feeding false latency estimates into the Smith predictor.

P14 exists to prevent P15 from building a predictor on garbage.

## Required inputs

P14 should use the Random library and current unit support:

- `Random.Core` for deterministic seeded randomness,
- `Random.Distributions` for Gaussian noise, jitter, spikes, and drift,
- `Pow`, `Hz`, and `s^-1` for filtering/control formulas,
- typed empty arrays and `Require` for clean Oct-shaped source.

## Candidate filters to model

At minimum, the P14 lab should compare:

### 1. Raw measurement

Baseline. No filtering.

Expected failure mode:

```text
high reactivity, high noise sensitivity
```

### 2. Exponential moving average / low-pass

Simple and likely first implementation candidate.

```text
estimate_next = alpha * measurement + (1 - alpha) * estimate
```

Questions:

- what alpha range gives stable decisions?
- how much lag is acceptable?
- does it suppress spikes enough?

### 3. Sliding median

Useful for rejecting outliers.

Questions:

- window size?
- latency cost?
- does it preserve real step changes?

### 4. Trimmed mean / winsorized mean

Possible compromise between EMA and median.

Questions:

- is it worth extra complexity?
- does it behave better under bursty driver jitter?

### 5. Hysteresis bands

Useful for selection stability.

Question:

```text
Can we prevent variant/controller oscillation without hiding real changes?
```

### 6. Simple Kalman filter

Only if the lab shows a clear advantage over EMA/median.

Kalman is plausible because this is a measurement-estimation problem, but it should not be introduced just because it is elegant. It must earn the complexity.

## P14 output fields

The filtered evidence object should likely expose:

```text
raw_value
filtered_value
jitter_estimate
outlier_detected
confidence
sample_count
last_update_tick
filter_mode
```

Optional later:

```text
trend_estimate
drift_estimate
variance_estimate
```

## P14 acceptance target

P14 should produce an actionable answer:

```text
Use filter X for initial Prometheus evidence.
Use parameters/ranges Y.
Reject/defer filters Z.
Expose fields A/B/C to the runtime.
```

P14 should prefer one correct, robust, boring answer over theoretical optimality.

---

# P15 — Predictive Lease-Ahead / Smith Predictor

## Purpose

P15 should turn lookahead from a diagnostic into a bounded actuator.

The correct framing is not simply “prefetch.” The correct framing is:

```text
predict future readiness
request future resource lease
actuate only what is safe to cancel
correct prediction error synchronously
```

Prefetch is only one possible actuator under this controller.

## Core concept

Gemini’s useful framing: Smith predictor in HFSM means two timelines:

```text
physical timeline:
  actual queue/fence/transfer/slot state

predictive timeline:
  delay-free shadow model of expected readiness
```

Prometheus should map this into existing Dominatus concepts:

```text
Physical HFSM:
  actual slot/queue/lease state

Shadow HFSM:
  predicted slot readiness / future WIP state

Delayed prediction buffer:
  circular history of prior shadow predictions

Correction event:
  actual_state - delayed_prediction
```

The correction event is the critical safety mechanism. It prevents the predictor from becoming an ungrounded fantasy machine.

## Preferred terminology

Use:

```text
predictive lease-ahead
future lease
bounded lookahead actuator
```

Avoid treating “Smith predictor” as a monolithic blob. Split it into:

```text
predictor model
lease-ahead controller
actuator
correction path
```

## First P15 implementation target

Do not begin with full transfer prefetch.

Start with the safest actuator from prior M51-style reasoning:

```text
pre-plan + reservation
```

Likely staged progression:

```text
P15a: predictive model lab / lease-ahead design
P15b: pre-plan + reservation actuator
P15c: pre-stage actuator
P15d: pre-transfer actuator, only after evidence supports it
```

## Smith predictor state model

Potential runtime fields:

```text
shadow_slot_state
predicted_ready_tick
predicted_arrival_tick
lookahead_depth
prediction_confidence
delayed_prediction_ring
last_prediction_error
correction_count
stale_prediction_count
future_lease_requested
future_lease_granted
future_lease_cancelled
```

## HFSM states / events

Candidate states/events:

```text
ForecastLeaseRequested
ForecastLeaseGranted
ForecastLeaseDenied
PrePlanReserved
PreStageReady
PreTransferSubmitted
PredictionArrived
PredictionMissed
StallCorrection
LeaseYielded
PredictionCancelled
```

Keep the state vocabulary compact at first.

## Safety gates

Future lease / lookahead must be blocked when:

```text
runtime unsafe
affected slot failed
slot invalidated
outstanding depth at cap
future entry unavailable
memory budget exceeded
prediction confidence below threshold
transfer unavailable for transfer actuator
shape/layout/precision mismatch
```

## Measurement dependency

P15 must consume filtered evidence from P14, not raw durations.

If evidence confidence is low:

```text
reduce lookahead depth
or disable actuator
or fall back to current lease behavior
```

## Metronome / staggered train model

Gemini’s modulo-L “staggered train” idea is promising, but should not be the first implementation.

It models dead time as a pipeline:

```text
L phase-shifted trains
train_id = tick mod L
one train injects, one finishes, others reside
```

This is valuable for a later P15 stage if latency L becomes stable enough. For GPU runtime first pass, latency may vary with:

- shape,
- queue contention,
- driver behavior,
- transfer path,
- thermal/clock state,
- resource pressure.

So first implementation should use small bounded lookahead depth rather than full modulo-L scheduling.

---

# P16 — Vendor-Specific Backend Paths

## Purpose

P16 should introduce vendor-specific execution paths as additive capabilities, not as hardcoded device decisions.

Candidate paths:

```text
Vulkan generic path
NVIDIA PTX path
AMD HIP C path
possibly future vendor/library path
```

## Governing principle

Capability and selection must remain separate:

```text
capability = what can run
judgment = what should run now
```

Adding PTX/HIP must not mean hardcoding vendor dispatch in a brittle way.

Instead:

```text
register capability
expose diagnostics
mark benchmark/DVT/PVT state
let Judgment Engine select only when eligible
```

## Promotion state

Vendor paths should use the same lifecycle model established in P13:

```text
benchmark_enabled
dvt_validated
pvt_validated
production_eligible
dispatch_enabled
```

## Non-goals for P16

Do not try to out-CUDA CUDA in one shot.

P16 should prove that vendor paths can be plugged into the same controller and evidence system.

---

# P17 — PVT Evidence Tuning Across Devices

## Purpose

P17 should collect evidence across device classes and update selector/controller priors without overfitting to any single GPU.

PVT is not:

```text
one cloud GPU benchmark decides production behavior
```

PVT is:

```text
multi-device evidence collection
selector prior adjustment
confidence scoring
promotion decision support
```

## Required principle

No single device becomes gospel.

Evidence should be grouped by:

```text
device vendor
device class
memory/compute band
queue topology
runtime feature availability
shape class
variant/backend path
```

## Output

P17 should produce:

```text
validated evidence tables
selector prior updates
confidence thresholds
promotion recommendations
rollback rules
```

Do not enable production dispatch automatically based on one run.

---

# Notes on correctness vs optimization

The entire Prometheus trajectory should keep this hierarchy:

```text
correctness
truthful diagnostics
robust measurement
bounded control
cross-device evidence
performance optimization
```

Performance never outranks correctness or truth.

Benchmarks are evidence, not authority.

---

# Immediate reminder before resuming P14

Before starting P14, finish library modernization audit/cleanup so future Codex authors do not copy historical workaround patterns.

Known modernization themes:

```text
Pow instead of Exp(Ln(x) * y)
Hz / s^-1 instead of dimensionless frequencies
Require instead of silent normalization
for loops instead of manual while counters
typed [] instead of old empty-array workarounds
Random result records instead of historical tuple-threading
negative literals instead of 0.0 - x fossils
```

Libraries are examples. If they are stale, the model learns stale Oct.

---

# Recommended next milestone sequence

```text
Library Modernization Audit
Library Modernization Implementation Passes
P14 M1 — Measurement Filtering Strategy Lab
P14 M2 — Evidence Object / Filter API Design
P14 M3 — Runtime Evidence Filter Implementation
P15 M1 — Predictive Lease-Ahead / Smith Predictor Lab
P15 M2 — Bounded Future-Lease Runtime Seam
P15 M3 — Pre-plan + Reservation Actuator
P16 — Vendor Capability Paths
P17 — PVT Evidence Tuning
```

Keep each milestone small enough that Codex can land Outcome A, or at least expose a meaningful Outcome B without pretending.
