# P15 M0 — Predictive Lease-Ahead / Smith Predictor Design Contract

## Scope and phase boundary

P15 M0 defines the native design contract for predictive lease-ahead in Prometheus/Dominatus.

This milestone is design-only and does not implement runtime behavior.

P15 M0 explicitly excludes:

- production prefetch behavior,
- vendor-specific backend path work (P16 scope),
- PVT evidence tuning (P17 scope).

Roadmap phase order remains:

```text
P14 — measurement quality
P15 — prediction/control
P16 — vendor capability paths
P17 — PVT evidence tuning
```

## 1) Problem statement

Prometheus currently makes lease/control decisions from present or recently observed state. In GPU/queue/memory systems, decision latency means a "current-state" decision often arrives late relative to when work becomes ready.

Control problem:

```text
GPU/memory/queue latency causes decisions based on current state to arrive late.
Prometheus needs bounded lookahead so it can reserve or prepare resources for when work is predicted to be ready.
```

Design objective:

```text
predict future readiness
request future resource lease
actuate only what is safe to cancel
correct prediction error synchronously
fall back when confidence is low
```

Prediction is advisory, not authority. A future lease is distinct from a current lease.

## 2) Inputs from P14 filtered evidence

P15 consumes P14 filtered evidence via the existing measurement-filter seam (`prom_dominatus_filtered_evidence`) and keeps raw and filtered dimensions separate.

Minimum inputs:

- raw timing (`raw_value`),
- filtered timing (`filtered_value`),
- filter confidence (`confidence`),
- filter warmup (`filter_warmup`),
- selected filter (`selected_filter`),
- outlier count (`outlier_count`),
- sample count (`sample_count`),
- measurement validity (`valid`).

Rules:

1. Use filtered evidence as predictor input.
2. Preserve raw evidence for truth/debug and correction auditing.
3. If confidence is low, reduce lookahead depth or disable predictive lease-ahead.
4. If evidence is warming up or invalid, predictor remains in hold/fallback mode.

## 3) Predictor state model (native contract)

Proposed native struct concepts (bounded, fixed-capacity):

```c
typedef struct prom_predictive_leaseahead_state {
  /* Shadow model state */
  uint32_t shadow_slot_state;
  uint64_t predicted_ready_tick;
  uint64_t predicted_arrival_tick;

  /* Control */
  uint32_t lookahead_depth;
  double prediction_confidence;

  /* Delayed prediction */
  prom_delayed_prediction_entry delayed_prediction_ring[PROM_PRED_RING_CAP];
  uint32_t delayed_ring_head;
  uint32_t delayed_ring_count;

  /* Correction */
  int64_t last_prediction_error;
  uint64_t correction_count;
  uint64_t stale_prediction_count;

  /* Future lease lifecycle counters */
  uint64_t future_lease_requested;
  uint64_t future_lease_granted;
  uint64_t future_lease_cancelled;

  /* Safety/fallback */
  uint32_t predictor_stale;
  uint32_t fallback_active;
  uint32_t fallback_reason;
} prom_predictive_leaseahead_state;
```

Design notes:

- Keep no-heap, fixed-size storage to align with native Dominatus bounded-state patterns.
- Include shape/layout/variant identity tags in entries to prevent cross-shape contamination.
- Keep predictor confidence independent from filter confidence (derived, not aliased).

## 4) Physical vs shadow state model

P15 uses two explicit timelines.

### Physical timeline HFSM

Tracks actual runtime facts, including:

- slot prepared,
- transfer staged,
- submit in flight,
- fence complete,
- lease held,
- lease yielded,
- slot failed,
- slot invalidated.

### Predictive timeline (shadow model)

Tracks delay-free expected readiness:

- if an order/request is placed now, when should readiness occur,
- what future lease should be requested for tick `T + L`,
- what conditions should be true when delayed prediction matures.

Separation rule:

- Physical state is truth.
- Shadow state is hypothesis.
- Correction event reconciles shadow with physical truth.

## 5) Delayed prediction buffer design

Use fixed-size circular buffer (bounded, no heap allocation).

Each delayed prediction entry:

- `issued_tick`,
- `target_tick`,
- `predicted_ready_state`,
- `predicted_slot_id`,
- `predicted_shape_class`,
- `predicted_variant`,
- `predicted_latency`,
- `prediction_confidence`,
- `lease_request_id` (if future lease was requested).

Contract:

- Capacity fixed at compile time (e.g., 8 or 16 entries at first pass).
- On overflow risk, block new predictive issuance (safety gate), never unbounded growth.
- Mature entries are reconciled exactly once with physical state.

## 6) Correction event contract

Correction event compares matured delayed prediction against physical state snapshot.

Suggested correction record fields:

- `prediction_matured`,
- `actual_ready`,
- `predicted_ready`,
- `arrival_error_ticks`,
- `state_mismatch`,
- `confidence_delta`,
- `correction_action`.

Correction actions:

- no correction,
- lower confidence,
- reduce lookahead depth,
- cancel future lease,
- hold current policy,
- fall back to current lease behavior,
- mark predictor stale.

Correction path requirements:

- Synchronous at prediction maturity boundary.
- Deterministic and bounded-time.
- Always emits diagnostics, even for no-op correction.

## 7) Future lease model

Future lease semantics are separate from current lease semantics.

- Current lease asks: "may I run now?"
- Future lease asks: "should I reserve capacity for tick T?"

Proposed future lease request fields:

- `future_lease_requested`,
- `future_lease_target_tick`,
- `future_lease_resource_kind`,
- `future_lease_slot_id`,
- `future_lease_confidence`,
- `future_lease_state`,
- `future_lease_cancel_reason`.

Future lease states:

- `none`,
- `requested`,
- `granted`,
- `denied`,
- `cancelled`,
- `matured`,
- `yielded`,
- `expired`.

M0 rule: future lease requests are cancellable hints until matured.

## 8) Bounded lookahead actuator staging

Staged actuator plan:

- **P15a**: predictive model lab / design,
- **P15b**: pre-plan + reservation actuator,
- **P15c**: pre-stage actuator,
- **P15d**: pre-transfer actuator.

M0 recommendation:

- Start with **pre-plan + reservation** because it is safest and cancellable.
- Do not start with full transfer prefetch.

Actuator contract:

- Actuator can only issue actions that have explicit cancel path.
- Unsafe/non-cancellable actions are disallowed until later evidence proves viability.

## 9) Safety gates (classified)

### Hard gates

- runtime unsafe,
- slot failed,
- slot invalidated,
- outstanding depth at cap,
- future entry unavailable,
- memory budget exceeded,
- filtered evidence invalid,
- resource lease hard deny.

### Soft confidence gates

- prediction confidence below threshold,
- filtered evidence warmup.

### Actuator-specific gates

- transfer unavailable for transfer actuator,
- shape/layout/precision mismatch.

Gate behavior:

- Hard gate => block future lease/lookahead and return to baseline lease behavior.
- Soft gate => reduce lookahead depth, possibly to zero.
- Actuator-specific gate => disable only the affected actuator stage.

## 10) Diagnostics and truth separation contract

P15 must preserve separate truth dimensions:

- raw measurement,
- filtered evidence,
- prediction,
- future lease request,
- future lease grant/deny,
- actual runtime state,
- correction event,
- selector recommendation,
- requested variant,
- executed variant,
- feature capability,
- feature usage.

Proposed P15 diagnostic fields:

- `prediction_valid`,
- `prediction_confidence`,
- `lookahead_depth`,
- `predicted_ready_tick`,
- `actual_ready_tick`,
- `prediction_error_ticks`,
- `future_lease_state`,
- `future_lease_target_tick`,
- `correction_count`,
- `fallback_reason`.

Truth rule:

- Never collapse prediction diagnostics into runtime truth fields.
- Never overwrite raw/filtered evidence with predictor-derived values.

## 11) HFSM / Dominatus integration seams

P15 integrates via explicit seams, not monolithic predictor insertion.

- **Blackboard facts seam**: stage predictor/future-lease facts as independent keys.
- **Judgment Engine seam**: score lease-ahead suitability from confidence, gate status, and resource pressure.
- **HFSM/stack seam**: add compact events, avoid state explosion.
- **Lease-control seam**: future lease request path separate from immediate lease decision path.
- **Filtered evidence seam**: predictor input from P14 filtered evidence stream.
- **Future predictor seam**: delayed buffer + correction lifecycle contained in dedicated state object.

Candidate events/states (compact initial vocabulary):

- `ForecastLeaseRequested`,
- `ForecastLeaseGranted`,
- `ForecastLeaseDenied`,
- `PrePlanReserved`,
- `PreStageReady`,
- `PreTransferSubmitted`,
- `PredictionArrived`,
- `PredictionMissed`,
- `StallCorrection`,
- `LeaseYielded`,
- `PredictionCancelled`.

M0 recommendation: start with subset used by pre-plan + reservation.

## 12) Metronome / staggered train model position

Gemini modulo-L staggered train model is promising but deferred.

Deferred rationale:

- GPU latency `L` is not stable enough in first implementation.
- `L` varies with shape, queue contention, driver behavior, transfer path, thermal/clock state, and resource pressure.

Therefore, first implementation should use:

- small bounded lookahead depth,
- correction-driven confidence adjustment,
- strict gates.

Not first implementation:

- full modulo-L train scheduling.

## 13) P15 follow-up milestone decomposition

Recommended staged plan:

- **P15 M1** — predictive lease-ahead simulation lab.
- **P15 M2** — native predictor state + delayed prediction buffer.
- **P15 M3** — future lease request seam (diagnostic-only).
- **P15 M4** — pre-plan + reservation actuator.
- **P15 M5** — correction-event integration.
- **P15 M6** — pre-stage actuator.
- **P15 M7** — pre-transfer actuator only if evidence supports it.

Progression rule:

- Each milestone must show bounded cancellation safety before advancing actuator aggressiveness.

## 14) Acceptance risks and mitigations

1. **Predictor overtrusts noisy evidence**  
   Mitigation: confidence gates, warmup gate, correction penalties, and fallback floor.

2. **Future lease hoards resources**  
   Mitigation: lease caps, expiration, yield/cancel policy, deny-on-pressure.

3. **Lookahead depth too aggressive**  
   Mitigation: small default depth, adaptive depth reduction on miss/error.

4. **Correction path under-specified**  
   Mitigation: mandatory correction record and action map before any actuator enable.

5. **Filtered evidence warmup ignored**  
   Mitigation: hard startup warmup gating in M2/M3 seam.

6. **SGEMM-specific assumptions leak into generic Dominatus layer**  
   Mitigation: predictor core keyed by generic slot/resource facts; SGEMM mapped via adapter seam only.

7. **Actuator cannot be safely cancelled**  
   Mitigation: phase entry criterion requires explicit cancellation path and idempotent rollback behavior.

## Inconsistency and documentation gap notes

- No direct inconsistency found between P14 documents and this P15 M0 scope.
- Documentation gap to track in implementation milestones: current blackboard key catalog already contains SGEMM lease lookahead keying, but does not yet define a complete future lease state lifecycle contract (requested/granted/denied/cancelled/matured/yielded/expired) as first-class diagnostics vocabulary.

## Convergence statement

This milestone ends in **Success** for design/report scope:

- complete predictive lease-ahead design contract,
- bounded delayed prediction + correction path,
- future lease model,
- safety gates and diagnostics truth separation,
- actionable M1–M7 implementation decomposition.
