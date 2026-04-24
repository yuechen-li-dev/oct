# Prometheus SGEMM Algorithm Lab — M35 (Three-Mode Buffering Selector Implementation)

## 1) Mandatory synthesis from M32/M33/M34 and M29

1. **M33 three-mode policy (implemented as the selection contract).**
   - `FixedDoubleDefault` is the default when fixed-double is memory-feasible.
   - `PullLagPressure` is only legal when fixed-double is memory-infeasible, pull-lag memory feasibility passes, transfer variance is not high, and compute predictability is stable/tracked.
   - `SerialJitSurvival` is only legal when fixed-double and pull-lag are blocked while serial remains feasible.
   - Hard failure remains explicit as `NO_BUFFERING_MODE_FEASIBLE`.

2. **M34 implementation contract (implemented as guarded fallback semantics).**
   - Pull-lag is guarded by explicit timing/safety rejection hooks and reason-coded fallback semantics:
     - `PULL_LAG_LATE_STAGE_STARVATION`
     - `PULL_LAG_MEMORY_EDGE_REJECTED`
     - `PULL_LAG_VARIANCE_MISS`
     - `PULL_LAG_COMPUTE_UNSTABLE`
     - `PULL_LAG_WIP_WASTE_EXCEEDED`
   - Serial JIT remains strict one-at-a-time survival behavior with WIP depth bounded at `<= 1`, no prepared peer slot overlap, and explicit cleanup tracking.

3. **M29 machinery reused (unchanged core lifecycle authority).**
   - Existing slot HFSM lifecycle and metadata authority is reused for all modes.
   - Existing fixed-double orchestration path remains the default path; no rewrite of M29 state machinery was performed.
   - Existing invalidation, cleanup, and async-ownership checks remain the legality authority.

4. **Intentionally out-of-scope in M35.**
   - No triple-buffer implementation.
   - No push-lookahead or adaptive-kanban reinsertion.
   - No Kalman/control-theory framework.
   - No allocator redesign or memory suballocation system.
   - No new kernels or hardware timing claims.

## 2) Implemented selector design

The runtime now computes explicit buffering facts and calls a dedicated judgment-engine selector seam:

- input facts include memory budget/headroom and per-mode required slots,
- pull-lag safety gates include variance, predictability, starvation-risk, and waste-bound checks,
- selection order and scoring preserve policy intent:
  - fixed-double wins when feasible,
  - pull-lag wins only in guarded pressure windows,
  - serial is survival fallback only,
  - explicit hard-failure otherwise.

## 3) Judgment engine integration

A native buffering selector was added to `reactor_judgment_engine`:

- explicit facts struct (`prom_buffering_selector_facts`),
- explicit decision struct (`prom_buffering_selector_decision`),
- explicit mode enum and reason-code enum,
- explicit feasibility/rejection/score fields for diagnostics and tests.

Reactor SGEMM path now calls this selector and rejects no-feasible-mode with an explicit detail code.

## 4) Slot HFSM integration by mode

- **FixedDoubleDefault:** reuses existing two-slot fixed-double lifecycle.
- **PullLagPressure:** keeps bounded slot budget and legal lifecycle transitions through HFSM.
- **SerialJitSurvival:** enforces single-active-slot behavior by cleaning peer slot before scheduling, preserving one-at-a-time behavior and WIP `<= 1`.

No raw-flag lifecycle bypass was introduced.

## 5) Diagnostics added

`PrometheusSgemmPolicyDiagnostics` now includes M35 selector observability:

- selected mode, reason code, feasibility/rejection flags, per-mode scores,
- transition/rejection counters,
- memory budget/required/headroom fields,
- pull-lag timing and safety counters,
- serial one-at-a-time counters.

## 6) Tests added

1. **Judgment engine selector tests** (`reactor_judgment_engine_tests.cpp`):
   - fixed default selection,
   - pull-lag pressure selection,
   - serial fallback selection,
   - explicit no-feasible-mode hard failure.

2. **Runtime integration tests** (`reactor_m35_buffering_selector_tests.cpp`):
   - fixed-double remains default in feasible regime,
   - serial survival selection and WIP bound,
   - explicit no-feasible-mode runtime failure detail.

## 7) Deferred scope and follow-up notes

- Pull-lag timing counters are currently lightweight runtime telemetry hooks, not a full predictive scheduler framework.
- Memory budget facts are policy-level gating estimates, not allocator-backed hard byte accounting.
- These deferrals are intentional and consistent with M35 non-goals.

## 8) Inconsistency and documentation-gap note

No syntax/style inconsistency against `Language/reference` was encountered during this native implementation pass. No additional language-contract files were introduced.
