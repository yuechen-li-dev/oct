# Prometheus SGEMM Algorithm Lab — M33 (Memory-Pressure Policy Synthesis for Buffering Modes)

## 1) Required first step — M32 conclusion restated

M33 starts from the M32 result and does **not** reopen the broad candidate bakeoff.

1. **Why fixed double remains default**
   - M32 showed fixed double is robust and practical once memory is available above near-cap pressure.
   - Under plentiful or comfortably feasible memory, fixed double keeps the normal throughput path without pressure-mode complexity.

2. **Why pull-lag remains useful only under memory pressure**
   - Pull-lag is not the default path; it is useful when fixed double is blocked by memory pressure while transfer variance stays low/moderate and compute predictability remains stable.
   - M32 recommended reviving pull-lag only as a guarded memory-pressure mode.

3. **Why serial JIT is the survival fallback**
   - Serial JIT stayed the most broadly feasible strategy under hard memory ceilings.
   - It is slower, but it avoids outright failure when higher-throughput modes are blocked.

4. **Why other candidates are deferred/rejected for now**
   - Fixed triple and push lookahead remained too memory-heavy/fragile under pressure.
   - Broad adaptive Kanban did not justify additional scope in this milestone.
   - M33 intentionally excludes those paths and focuses only on the three retained modes.

5. **What facts the policy must observe**
   - Memory feasibility (`MemoryBudgetSlots`, per-mode required slots, and headroom).
   - Stability/safety qualifiers (`TransferVarianceClass`, `ComputePredictabilityClass`, `StarvationRisk`).
   - Context qualifiers (`ComputeDominanceClass`, `FallbackAvailable`).

## 2) Candidate modes retained

Only the following are modeled:

- `FixedDoubleDefault`
- `PullLagPressure`
- `SerialJitSurvival`

No new candidate search was performed.

## 3) Candidates intentionally deferred/rejected

The following are intentionally out-of-scope in M33:

- `FixedTriple`
- `PushLookahead`
- broader `AdaptiveKanban`
- Kalman/estimator retries
- any newly invented buffering mode

## 4) Policy facts

M33 models policy over these explicit facts per scenario:

- `MemoryBudgetSlots`
- `RequiredDoubleSlots`
- `RequiredPullLagPeakSlots`
- `RequiredSerialSlots`
- `MemoryHeadroomSlots`
- `TransferVarianceClass` (`low`, `moderate`, `high`)
- `ComputePredictabilityClass` (`stable`, `unstable`)
- `ComputeDominanceClass` (`compute_dominant`, `balanced`, `transfer_sensitive`)
- `StarvationRisk` (`low`, `moderate`, `high`)
- `FallbackAvailable`

## 5) `when utility` decision model

M33 uses `when utility` to arbitrate among the three retained modes.

Policy shape implemented:

- hard-reject infeasible modes by memory feasibility gates,
- apply additional pull-lag safety gates (`TransferVarianceClass != high`, `ComputePredictabilityClass == stable`),
- score remaining candidates,
- choose winner via `when utility`,
- return explicit reason code on fallback/hard failure.

This keeps policy readable and auditable without nested-if maze logic.

## 6) Scenario findings (compact dataset)

M33 intentionally uses 10 scenarios only:

1. plentiful memory + low variance → `FixedDoubleDefault`
2. plentiful memory + high variance → `FixedDoubleDefault`
3. tight memory + low variance → `FixedDoubleDefault`
4. tight memory + moderate variance → `FixedDoubleDefault`
5. tight memory + high variance → `FixedDoubleDefault`
6. critical memory + low variance → `SerialJitSurvival`
7. critical memory + high variance → `SerialJitSurvival`
8. double infeasible + pull-lag feasible → `PullLagPressure`
9. pull-lag infeasible + serial feasible → `SerialJitSurvival`
10. no feasible mode → `NO_BUFFERING_MODE_FEASIBLE`

This satisfies the compact-confidence requirement without a broad sweep.

## 7) Final policy contract

M33 contract:

- **Default:** fixed double whenever feasible and not under hard pressure exclusion.
- **Pressure mode:** pull-lag only after fixed-double memory failure and only with low/moderate variance + stable compute.
- **Survival mode:** serial JIT when earlier modes are blocked but serial remains feasible.
- **Hard failure:** explicit `NO_BUFFERING_MODE_FEASIBLE` when all three are infeasible.

Required reason-code set includes:

- `FIXED_DOUBLE_MEMORY_INSUFFICIENT`
- `PULL_LAG_MEMORY_INSUFFICIENT`
- `PULL_LAG_VARIANCE_TOO_HIGH`
- `PULL_LAG_COMPUTE_UNPREDICTABLE`
- `SERIAL_JIT_MEMORY_INSUFFICIENT`
- `NO_BUFFERING_MODE_FEASIBLE`

## 8) Implementation recommendation

Implement a runtime judgment engine that:

1. computes the M33 policy facts each dispatch,
2. evaluates hard gates first,
3. runs the three-mode `when utility` arbitration,
4. emits mandatory diagnostics (selected mode, rejected modes, scores, feasibility flags, starvation risk, reason code).

No additional broad rake lab is required before implementation unless runtime telemetry contradicts M33 assumptions.

## Required final answers

1. **When should fixed double be selected?**
   - When fixed-double memory feasibility passes and pressure-mode safety does not force fallback.
2. **When should pull-lag pressure mode be selected?**
   - Only when fixed double is memory-infeasible, pull-lag is memory-feasible, variance is low/moderate, and compute predictability is stable.
3. **When should serial JIT survival mode be selected?**
   - When fixed double and pull-lag are rejected but serial JIT remains memory-feasible with fallback enabled.
4. **What are the hard gates?**
   - Per-mode memory feasibility, pull-lag variance/predictability safety gates, and fallback availability for serial survival.
5. **What diagnostics/reason codes are mandatory?**
   - Selected mode, rejected modes, per-mode scores, feasibility flags, starvation-risk class, and explicit reason code from the required set.
6. **Is another rake lab needed before implementation?**
   - No broad lab; only revisit if production telemetry contradicts these compact-policy assumptions.
7. **What should the implementation milestone build?**
   - A three-mode policy selector using `when utility` with hard gates + score arbitration + stable diagnostics/reason-code emission.

## Artifacts emitted

- `m33_policy_scenario_table.octagon`
- `m33_mode_score_table.octagon`
- `m33_fallback_reason_table.octagon`
- `m33_final_policy_contract.octagon`

## Inconsistency note

No syntax/style inconsistency against `Language/reference` was encountered while implementing this policy-synthesis lab.
