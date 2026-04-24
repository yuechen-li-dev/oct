# M34 Report — Pull-Lag Pressure + Serial JIT Survival Rake Lab

## 1) M33 recap (required first step)

M33 locked the selection policy before implementation:

1. **FixedDoubleDefault remains the default** because when memory permits, it is the most robust normal-throughput path with low starvation risk and simpler steady-state behavior.
2. **PullLagPressure is selected only when fixed-double is memory-infeasible, pull-lag is memory-feasible, transfer variance is low/moderate, and compute predictability is stable.**
3. **SerialJitSurvival is selected when fixed-double and pull-lag are blocked but serial JIT remains memory-feasible.**
4. **Serial JIT is simpler** (single-slot, no overlap), **but still needs sanity checks** for lifecycle, invalidation, busy/retry behavior, and failure cleanup.
5. **Pull-lag needs stricter execution-contract rakes** because timing mistakes (late/early staging), variance misclassification, and drift can create starvation or erase memory-pressure benefits.

## 2) PullLagPressure contract

Pull-lag is implementation-safe only under these execution conditions:

- fixed-double feasibility gate fails
- pull-lag feasibility gate passes without any temporary budget overshoot
- transfer variance is classified low/moderate and monitored for spikes
- compute predictability remains stable or drift is detected quickly
- explicit slot ownership (no overwrite-before-ready; no consume-before-ready)
- pull-lag WIP depth remains capped to the mode budget (two-slot model only)

Mandatory pull-lag fallback reasons:

- `PULL_LAG_LATE_STAGE_STARVATION`
- `PULL_LAG_MEMORY_EDGE_REJECTED`
- `PULL_LAG_VARIANCE_MISS`
- `PULL_LAG_COMPUTE_UNSTABLE`
- `PULL_LAG_WIP_WASTE_EXCEEDED`

## 3) SerialJitSurvival contract

Serial JIT is implementation-safe as survival fallback with strict invariants:

- active slot count <= 1
- WIP depth <= 1
- committed memory <= serial requirement
- no prepared peer slot and no overlap
- shape/layout/precision churn must still trigger invalidation and capacity checks
- failures are explicit and require cleanup before retry

## 4) Pull-lag rake findings

### Timing rakes

- **Late staging / starvation** case produced non-zero starvation and explicit retreat trigger.
- **Early staging / WIP waste** case produced ready-but-unused inflation and explicit retreat trigger.
- **Compute drift** case showed stale demand prediction leading to late completion and fallback.

### Memory-edge rakes

- At boundary-fit budget, stage/allocate stays legal with zero ceiling violation.
- Under-budget edge rejects stage start before violation.
- “No temporary over-budget” held in all edge rows (zero ceiling violations).

### Variance misclassification rakes

- Cases misclassified as low/moderate but executing with high variance were detected by late-stage/starvation metrics.
- Contract requires retreat path (serial fallback) and explicit `PULL_LAG_VARIANCE_MISS` diagnostics.

## 5) Serial JIT rake findings

- One-at-a-time contract held in all rows (`active slot count = 1`, `WIP depth = 1`).
- Memory-minimal contract held (`committed memory = serial requirement`).
- Sequencing stayed explicit across repeated chunks and churn cases.
- Failure path remained explicit: failed chunk requires cleanup before retry; no peer-slot poisoning concept.

## 6) Cross-mode transition findings

Legal transitions defined:

- **PullLagPressure → SerialJitSurvival** when pull-lag safety trigger fires and fixed-double remains infeasible.
- **SerialJitSurvival → PullLagPressure** only when full M33 gates are restored.
- **PullLagPressure/SerialJitSurvival → FixedDoubleDefault** when fixed-double feasibility recovers.
- **Any mode → NO_BUFFERING_MODE_FEASIBLE** when all three modes are infeasible.

Disallowed behavior:

- no silent promotion to fixed-double while fixed-double is still infeasible
- no partial mode selection in hard-failure state

## 7) Mandatory diagnostics / reason codes

Per decision/transition telemetry must include:

- selected mode
- transition reason
- rejected modes
- feasibility flags
- fallback reason
- hard-failure reason

Pull-lag timing/memory telemetry must include:

- predicted demand time
- estimated transfer lead time
- safety margin
- stage start/completion timestamps
- late-stage and early-stage counts
- starvation time
- ready-but-unused time
- committed memory and ceiling-violation count

Serial telemetry must include:

- active slot count
- WIP depth
- committed memory
- sequential step count
- busy/retry count
- failure/cleanup count

## 8) Final implementation contract

Artifacts produced for implementation handoff:

- `m34_pull_lag_timing_rake.octagon`
- `m34_pull_lag_memory_edge_table.octagon`
- `m34_pull_lag_variance_miss_table.octagon`
- `m34_serial_jit_contract_table.octagon`
- `m34_cross_mode_transition_table.octagon`
- `m34_final_contract.octagon`

M34 conclusion: pull-lag and serial-jit are both implementable with guarded contracts and explicit reason-coded fallback logic.

## 9) M35 recommendation

M35 should implement the three-mode selector runtime exactly as:

1. **FixedDoubleDefault** by default when feasible.
2. **PullLagPressure** only through M33 entry gates plus M34 timing/memory ownership safeguards.
3. **SerialJitSurvival** as explicit survival fallback.
4. **`NO_BUFFERING_MODE_FEASIBLE`** hard-stop when all gates fail.

Also wire mandatory telemetry fields and reason codes directly into decision outputs so regressions are diagnosable.

---

## Required final answers

1. **Is PullLagPressure safe to implement?** Yes, as a guarded pressure mode.
2. **Under what exact execution conditions?** Fixed-double infeasible, pull-lag feasible without budget overshoot, low/moderate transfer variance, stable or tracked compute predictability, explicit slot ownership, bounded WIP.
3. **What failure modes must trigger fallback?** `PULL_LAG_LATE_STAGE_STARVATION`, `PULL_LAG_MEMORY_EDGE_REJECTED`, `PULL_LAG_VARIANCE_MISS`, `PULL_LAG_COMPUTE_UNSTABLE`, `PULL_LAG_WIP_WASTE_EXCEEDED`.
4. **Is SerialJitSurvival safe to implement?** Yes, as strict survival fallback.
5. **What invariants must serial JIT enforce?** One-at-a-time active slot/WIP depth, memory-minimal commitment, no overlap, invalidation/capacity checks on churn, explicit cleanup after failure.
6. **What cross-mode transitions are legal?** Pull-lag→serial on safety breach, serial→pull-lag on restored M33 gates, pull/serial→fixed-double on restored memory, any→hard-failure when none feasible.
7. **What diagnostics/reason codes are mandatory?** Mode/transition/feasibility/fallback/hard-failure fields plus timing/memory and serial lifecycle metrics; reason codes listed in sections 2 and 7.
8. **Is another rake lab needed before implementation?** No broad new rake is required; targeted runtime instrumentation checks are sufficient.
9. **What should M35 build?** The actionable three-mode selector with enforced contracts and mandatory reason-coded telemetry.
