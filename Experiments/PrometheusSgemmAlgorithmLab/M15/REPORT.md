# Prometheus SGEMM Algorithm Lab — M15

## Rake Lab for Candidate C (Waste-Budgeted Speculation)

## 0) Representation audit for M15 follow-up

### 0.1 Structures identified as truly table-shaped

- Scenario catalog (repeated same-schema scenario rows) -> normalized to a **columnar table** in code.
- Failure-mode summary grid (fixed schema with repeated rows) -> normalized to a **columnar table** in code.
- Risk target list (same schema, repeated entries) -> retained as row records for readability, but now emitted with aligned table intent.

### 0.2 Structures that stayed compositional (non-tabular)

- Controller configuration (`M15ControllerUnderTest`) remains a single compositional record.
- Verdict/handoff bundle remains a single compositional record (`M15Verdict`) because it is one decision payload, not a repeated table.
- Per-step simulation state remains procedural/stateful by nature (not a static table).

### 0.3 Enum + `match` adoption audit

- Adopted for controller mode modeling (`Aggressive | Safe | Recovery(Int)`), with `match` payload binding used for recovery countdown progression.
- Adopted for verdict classification (`Survives | SurvivesWithModifications | Fails`) with `match` to derive textual verdict.
- Not adopted for scenario family tags (strings were clearer for artifact readability and table scanability).

### 0.4 Result of cleanup

- Representation shape improved (table-like structures now explicitly columnar where appropriate).
- No substantive conclusion change from M15: verdict remains **survives with modifications**.

## 1) Controller under test (required setup restatement)

M15 directly rake-tests the exact M14 winner:

- **Controller family:** `C-waste-budgeted-speculation`
- **Controller shape preserved:** bounded overlap, modest lookahead, bounded outstanding depth, explicit waste budget accounting, explicit retreat, explicit recovery

### 1.1 Exposed controls

- `chunk` default `16` (bounded `8..32`)
- `lookahead` default `2` (bounded `0..2`)
- `outstanding depth` default `2` (bounded `1..2`)
- `waste budget` default `64 units`
- retreat threshold `25%` waste
- recover threshold `12%` waste
- recovery window `3` steps
- waste-accounting lag model `2` steps

### 1.2 M14 handoff risks attacked directly

1. oscillation/thrash near regime boundaries
2. over-retreat / under-retreat
3. pathological chunk behavior under bursty transfer setup
4. sticky/laggy recovery after abrupt shifts

### 1.3 Additional required risks attacked

5. budget cliff behavior
6. waste accounting lag
7. false calm after lucky streaks
8. mixed-regime path dependence
9. boundedness integrity

### 1.4 M15 survival meaning for M16 readiness

Controller is considered M16-ready only if:

- core bounds remain intact (no bound violations),
- instability remains finite and legible,
- waste retreats are effective but not permanently sticky,
- budget/depletion behavior degrades gracefully under stress,
- path dependence is visible and capped with guardrails.

M15 verdict: **survives with modifications**.

---

## 2) Rake scenarios

M15 includes the required stress families and concrete scenarios:

- **A. Boundary jitter:**
  - `A1-boundary-jitter-transfer-balanced`
  - `A2-boundary-jitter-balanced-predictability`
- **B. Abrupt shifts:**
  - `B1-abrupt-transfer-to-compute`
- **C. Bursty transfer-cost:**
  - `C1-bursty-transfer-setup`
- **D. Waste spikes:**
  - `D1-waste-spike-and-lag`
- **E. Recovery windows / false calm:**
  - `E1-false-calm-then-crash`
- **Mixed path dependence pair:**
  - `F1-path-transfer-history-to-balanced`
  - `F2-path-compute-history-to-balanced`

Why these exist: each scenario intentionally amplifies one or more failure modes from the M14 handoff and required additional M15 concerns.

---

## 3) Failure-mode findings

### 3.1 Oscillation / thrash

- Boundary jitter produces noticeable transition bursts near retreat threshold.
- Still bounded and legible, but chatter appears around threshold edges.

### 3.2 Over-retreat / under-retreat

- Retreat reliably caps worst waste growth.
- Some scenarios spend high time in safe mode after depletion (over-retreat tendency).

### 3.3 Bursty transfer pathology

- Chunk remains bounded.
- Completion-time variance rises under burst-period setup spikes.

### 3.4 Sticky/laggy recovery

- Recovery mode returns lookahead after bad phases.
- Recovery window can still lag abrupt improvements.

### 3.5 Budget cliff behavior

- Near-threshold chatter and repeated cliff hits observed.
- No numeric runaway; behavior is bounded but noisy.

### 3.6 Waste accounting lag

- Lag overshoot is measurable before retreat engages.
- Indicates guardrail gap for pending waste early warning.

### 3.7 False calm after lucky streaks

- Lucky streak can briefly reopen aggressive mode.
- Crash phase then forces retreat; robust but not fully smooth.

### 3.8 Mixed-regime path dependence

- Same balanced destination behaves differently by entry history.
- Timing delta is material enough to require observability in M16.

### 3.9 Boundedness integrity

- Lookahead/depth/chunk/budget remained inside intended bounds in this rake run.

---

## 4) What held up

- The selected controller family stayed intact under stress.
- Hard boundedness remained preserved.
- Retreat mechanics prevented unbounded speculative waste.
- State transitions remained inspectable (aggressive/safe/recovery), suitable for judgment-engine integration.

---

## 5) What broke or became questionable

- Budget-edge chatter under boundary jitter.
- Accounting lag overshoot before retreat.
- Recovery lag after abrupt improvements.
- History-sensitive outcomes entering identical destination regimes.

These are practical guardrail gaps, not total controller failure.

---

## 6) Required M16 adjustments

M16 should **implement with guardrail changes**:

1. add retreat-entry hysteresis (2 windows) around budget threshold,
2. add lag-aware pending-waste early warning,
3. add cooldown-limited recovery ramp,
4. add burst-aware chunk dampening near setup spikes.

### Fixed defaults to carry into M16

- `lookahead=2`
- `outstanding_depth=2`
- `chunk=16` with `8..32` bounds
- `waste_budget=64`
- `retreat=25%`, `recover=12%`
- `recovery_window=3`

### Mandatory observability counters

- completion time, transfer stall, compute idle
- total transfer work, wasted transfer work, peak waste burst
- retreat/recovery counts, transitions/instability count
- retreated time percent
- budget depletion count, budget cliff hits
- accounting lag overshoot units
- path dependence delta
- bound violation count

### Explicitly not implemented yet

- estimator-heavy predictor stack
- dynamic depth > 2
- lookahead > 2
- auto-retuning from cross-scenario history

---

## 7) Final verdict

**survives with modifications**

Candidate C remains the correct practical implementation target, but M16 must include the listed guardrail upgrades before broad use.

---

## 8) Artifacts

- `m15_controller_setup.octagon`
- `m15_risk_targets.octagon`
- `m15_scenario_catalog_columnar.octagon`
- `m15_rake_results.octagon`
- `m15_failure_mode_table_columnar.octagon`
- `m15_failure_mode_findings.octagon`
- `m15_verdict.octagon`

---

## 9) Octomata usage note

M15 uses an explicit bounded mode lifecycle (`aggressive`, `safe`, `recovery`) with transition counters and recovery windows, aligned with Octomata-style state reasoning while keeping scenario tables readable.

### surfaced inconsistency / documentation gap

As in M13, this package path still does not directly link imported `Octomata` package symbols in this environment. M15 therefore keeps Octomata-style lifecycle modeling local in this experiment package and surfaces this linkage gap explicitly instead of silently hiding it.
