# Prometheus SGEMM Algorithm Lab — M25

## Pull-Based Lag-Compensated Buffering Lab

M25 is a pure-Oct simulation lab to compare fixed buffering against pull-based lag compensation and adaptive Kanban-style WIP control. It does not make Vulkan or hardware timing claims.

## 1) Conceptual simulation model (required first step)

### Definitions used by M25

1. **WIP in this simulation**
   - Chunks currently in transfer plus chunks transfer-complete but not yet consumed by compute.
   - The currently computing chunk is treated as consumption-in-progress, not inventory.
2. **Starvation**
   - Any simulation tick where compute is idle while downstream still has unmet chunk demand.
3. **Ready but unused**
   - A chunk has completed transfer and waits in the ready queue before compute starts consuming it.
4. **Lead-time / lag compensation**
   - Pull controller computes expected demand time from current compute progress and starts transfer using:
   - `start_stage_time = expected_demand_time - estimated_transfer_time - safety_margin`
5. **Safety margin**
   - Extra time subtracted from expected demand to absorb transfer variance and estimate error.
6. **Intentionally out-of-scope in M25**
   - Vulkan queue semantics, PCIe/cache/coherency behavior, host-driver overheads, async runtime internals, and hardware-specific wall-clock claims.

## 2) Candidate strategies implemented

- **A: Serial JIT** (`A-serial-jit`)
- **B: Fixed double buffering** (`B-fixed-double`)
- **C: Fixed triple buffering** (`C-fixed-triple`)
- **D: Push lookahead** (`D-push-lookahead`)
- **E: Pull lag-compensated staging** (`E-pull-lag-comp`)
- **F: Adaptive Kanban staging** (`F-adaptive-kanban`)

Adaptive Kanban mode decisions are implemented with an Octomata flow (`M25AdaptiveKanbanPolicy`) using `when policy` hysteresis and `min_commit` controls, then stepped each control tick.

## 3) Workload regimes covered

M25 evaluates all strategies under:

1. stable-balanced
2. transfer-bound
3. compute-bound
4. jittery-transfer
5. burst-shock
6. calm-then-burst

## 4) Metrics captured

M25 emits all required signals:

- completion time, compute starvation, transfer idle
- ready-but-unused time, total/avg/peak WIP depth
- memory pressure proxy (peak WIP × chunk size)
- stage-too-early and stage-too-late counts
- safety margin usage
- adaptive WIP-cap changes and mode transitions

Composite metrics are also included:

- flow efficiency score
- WIP waste score
- starvation penalty score
- memory pressure penalty
- product score (composite)

## 5) Required rake/failure-mode findings

1. **Optimistic lead-time estimate (too late staging)**
   - Exposed by high starvation in serial JIT, especially transfer-bound and burst regimes.
2. **Pessimistic lead-time estimate (too early staging)**
   - Exposed in compute-bound where fixed triple and push lookahead accumulate large ready-idle inventory.
3. **Jitter shock**
   - Burst-shock and calm-then-burst retain starvation risk for all policies; fixed double remains robust in this model.
4. **WIP cap oscillation**
   - Adaptive strategy shows non-zero cap/mode transitions under calm-then-burst, confirming responsive behavior.
5. **Memory pressure blind spot**
   - Triple buffering improves little in this model while sharply increasing compute-bound ready-idle inventory.
6. **Chunk-size interaction**
   - Chunk size directly scales transfer and compute durations in all regimes, affecting lag window viability.
7. **False stability**
   - Calm-then-burst regime specifically probes this; adaptive transitions engage but do not outperform fixed double in product score under current tuning.

## 6) Policy-question answers (required)

1. **Is fixed double buffering enough?**
   - In this simulation, yes as a robust baseline: it matches or exceeds alternatives across jitter/burst product score rows.
2. **Does fixed triple buffering justify memory cost?**
   - Not as default: compute-bound ready-idle and memory pressure rise materially without commensurate product-score gain.
3. **Does pull lag compensation reduce WIP without unacceptable starvation?**
   - Yes in compute-bound (higher product score than fixed double/triple and much lower ready-idle than push/triple) while matching fixed double elsewhere.
4. **Does adaptive Kanban improve over fixed buffering?**
   - Not as global default with current tuning; it responds (cap/mode changes) but does not beat fixed double in jitter-heavy regimes.
5. **What facts would a real judgment engine need?**
   - Online transfer lead-time error distribution, burst detector confidence, memory-pressure budget, starvation SLO, and cap/margin transition costs.
6. **Is this worth carrying toward implementation?**
   - Yes: carry pull lag-compensated staging forward with fixed-double-like guardrails and memory-pressure gating.

## 7) Final recommendation (required explicit answers)

- **Best strategy overall:** `E-pull-lag-comp`
- **Best strategy under jitter/burst regimes:** `B-fixed-double`
- **Adaptive Kanban worth it:** only if jitter-recovery behavior is product-critical and tuned with stronger anti-oscillation policy.
- **Triple buffering justified:** no as default; keep only as temporary guarded mode.
- **Carry forward:** pull lag compensation + explicit memory pressure metrics + optional adaptive escape hatch.
- **Reject/defer:** serial JIT and naive push lookahead as defaults; defer triple buffering to exceptional shock handling.

## 8) M26 direction

M26 should implement a controller prototype on the pull-lag shape (with fixed-double fallback behavior) and run a focused anti-oscillation rake for adaptive cap/margin tuning, not a broad re-open of buffering depth debate.

## Artifacts

- `m25_strategy_comparison.octagon`
- `m25_wip_inventory_table.octagon`
- `m25_starvation_table.octagon`
- `m25_jitter_stress_table.octagon`
- `m25_adaptive_kanban_table.octagon`
- `m25_final_recommendation.octagon`
