# P8i / M16 Report — Native Candidate C Implementation

## Scope outcome

M16 implements the selected **Candidate C (waste-budgeted speculation)** in the native C reactor path, using the existing judgment engine seam plus the pre-M16 policy-memory helper. This pass keeps the implementation bounded and avoids HFSM/runtime overreach.

## 1) M14 extraction: core controller shape used

From M14, M16 carries forward:

- bounded overlap behavior (not unbounded speculation),
- modest lookahead (`1..2`),
- bounded outstanding depth (`<= 2`),
- explicit speculative waste budgeting and retreat semantics,
- deterministic/inspectable decision behavior suitable for production policy seams.

## 2) M15 extraction: required guardrails implemented

From M15, M16 implements the required guardrail classes in native code:

- retreat-entry hysteresis / anti-chatter (policy threshold bands via policy memory),
- cooldown-limited recovery hold/ramp (`recovery_window=3` via policy memory timers),
- lag-aware pending-waste early warning (pending-waste proxy channel + warning counter),
- burst-aware chunk dampening (shape-change burst detection lowers chunk toward floor).

## 3) Mapping to native implementation

Native implementation wiring:

- `reactor_vulkan.c` now owns a bounded SGEMM controller state per runtime instance.
- Each SGEMM call computes bounded waste proxies, updates pending-waste lag state, and resolves mode through `prom_judgment_engine_update_policy_mode(...)`.
- Mode drives bounded knobs:
  - AGGRESSIVE => `lookahead=2`, `depth=2`, `chunk=16`
  - SAFE => reduced speculative posture (`lookahead=1`, `depth=1`) with stronger chunk dampening
  - RECOVERY => cooldown-limited ramp from conservative values back toward defaults.
- Resolved mode feeds reactor path selection by constraining judgment facts (SAFE forces conservative direct-path posture; tiled forcing is suppressed in SAFE).

## 4) Judgment engine + policy memory usage

- **Judgment engine** remains the deterministic decision seam:
  - `prom_judgment_engine_select_sgemm_mode(...)` resolves path/compute candidate.
  - `prom_judgment_engine_update_policy_mode(...)` is used for all cross-call mode transitions.
- **Policy memory** provides retained mode, hysteresis bands, min-commit dwell, cooldown, and recovery hold; M16 does not re-implement these ad hoc in the reactor.

## 5) Defaults adopted

M15 defaults adopted for M16:

- `lookahead=2`,
- `outstanding_depth=2`,
- `chunk=16` with bounds `8..32`,
- `waste_budget=64`,
- `retreat=25%` (`250 permille`),
- `recover=12%` (`120 permille`),
- `recovery_window=3`.

Implemented hysteresis uses explicit retreat/recover threshold bands around those defaults.

## 6) Observability added

Added native diagnostics surface:

- API: `prometheus_reactor_runtime_sgemm_policy_diagnostics(...)`
- struct: `PrometheusSgemmPolicyDiagnostics`

Includes:

- current mode and active bounded knobs (`lookahead`, `depth`, `chunk`, bounds),
- decision/transition counters,
- retreat/recovery counters,
- instability counter,
- wasted-work proxy (last + total),
- budget depletion count,
- safe/aggressive/recovery time proxies (decision counts),
- lag early-warning count,
- burst dampening count,
- bound violation count (expected zero).

## 7) Intentionally left out of M16

Per scope guardrails, intentionally not included:

- estimator-complexity stack revival (Kalman/predictor sophistication),
- lookahead > 2 or depth > 2,
- HFSM runtime framework,
- cross-controller generalization outside SGEMM,
- FFT/unrelated controller expansion,
- auto-retuning/meta-policy from long-run history.

## 8) Validation status and next step

M16 adds native tests for:

- bounded mode/knob observability,
- retreat + lag-warning behavior under pressure,
- guardrail counter integrity.

Next step is real hardware/native validation runs (Windows + Vulkan and representative regimes) to confirm the bounded policy shape and observability under production-like load traces.

## surfaced inconsistency

Lab M15 asks for wall-time-style metrics (e.g., completion/retreated time) as first-class counters; current native seam does not yet carry a stable monotonic timing envelope in this diagnostics API. M16 therefore reports bounded decision-count proxies for mode occupancy and preserves explicit waste/budget counters as the primary controller observability channel.
