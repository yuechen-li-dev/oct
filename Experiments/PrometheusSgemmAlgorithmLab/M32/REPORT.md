# Prometheus SGEMM Algorithm Lab — M32 (Memory-Ceiling Pull-Lag Feasibility)

## 0) Required first step: why M25/M26/M27 did not fully close this question

1. **How M25/M26 modeled WIP before**
   - M25 treated WIP largely as soft cost terms (WIP waste, memory-pressure proxy, ready-idle, product-score penalties), not a hard feasibility gate.
   - M26/M27 continued controller and robustness framing built on those soft penalties.
2. **Why soft WIP penalties may understate pull-lag advantage**
   - A strategy can still “compete” in soft scoring while carrying inventory levels that would fail on real VRAM limits.
   - Hard-cap infeasibility is binary; soft penalties do not force that binary boundary.
3. **What “hard memory ceiling” means here**
   - At each step, committed memory is computed from staged/in-transfer/in-flight/ready inventory plus output/readback reserve.
   - If a stage/allocation would exceed budget, strategy is marked `Feasible = false` with a first infeasible reason.
4. **Why fixed double may fail near capacity**
   - Fixed double assumes sustained two-slot inventory. Under near-cap budgets, reserve + two active slots may not fit.
5. **Why pull-lag may win for predictable compute + low-variance transfer**
   - Pull-lag can hold lower average inventory by staging closer to demand deadlines; when transfer variance is low and compute is predictable, this can preserve throughput while reducing committed memory spikes.

## 1) Scope and method

M32 is a **pure-Oct feasibility simulation** retesting the M25 candidates under hard memory ceilings. It is not a Vulkan benchmark and makes no hardware timing claims.

Retested candidates:
- A serial JIT
- B fixed double
- C fixed triple
- D push lookahead
- E pull-lag compensated staging
- F adaptive Kanban

## 2) Hard memory ceiling model (M32)

Per-tick committed memory tracks:
- in-transfer slot memory
- ready-but-unused slot memory
- in-flight compute slot memory
- output/readback reserve
- peak committed memory
- average committed memory
- memory inventory-time
- ready-unused memory-time

Hard rule used in simulation:
- if projected or committed memory exceeds budget, mark infeasible and retain first reason (`stage-allocation-over-budget` or `committed-memory-over-budget`).

## 3) Workload regime used

Base regime focuses on the expected pull-lag-friendly zone:
- sequential repeated same-shape chunks
- predictable compute
- low-variance transfer
- compute-dominant and balanced variants

Included stress variants:
- low variance
- moderate variance
- high variance
- compute-dominant
- balanced
- transfer-sensitive
- warmup/stabilization period
- near-capacity budget pressure

## 4) Budget sweep

Normalized budgets:
- 1.00 slot
- 1.25 slots
- 1.50 slots
- 1.75 slots
- 2.00 slots
- 3.00 slots

## 5) Budget sweep findings (feasibility-first)

Across 5 regimes per budget:
- **A serial JIT:** feasible at all budgets (`5/5` per budget)
- **B fixed double:** infeasible through 1.50; feasible from 1.75+ (`0/5,0/5,0/5,5/5,5/5,5/5`)
- **C fixed triple:** infeasible through 1.50; partially feasible at 1.75/2.00; fully at 3.00 (`0/5,0/5,0/5,4/5,4/5,5/5`)
- **D push lookahead:** infeasible through 1.50; only partial feasibility even at 3.00 (`0/5,0/5,0/5,4/5,4/5,4/5`)
- **E pull-lag:** infeasible through 1.50; feasible from 1.75+ (`0/5,0/5,0/5,5/5,5/5,5/5`)
- **F adaptive Kanban:** same feasibility envelope as pull-lag in this model (`0/5,0/5,0/5,5/5,5/5,5/5`)

## 6) Per-strategy conclusions

- **Serial JIT:** memory-survival fallback; strongest feasibility but weak throughput economics.
- **Fixed double:** strong practical baseline once budget reaches ~1.75 in this synthetic model.
- **Fixed triple:** usually too inventory-heavy under pressure; only justified when caps are loose.
- **Push lookahead:** most memory-wasteful/fragile in this hard-cap framing.
- **Pull-lag:** no unique feasibility unlock below fixed-double in this particular parameterization, but better product score than fixed-double in tight feasible bands.
- **Adaptive Kanban:** no clear surprise win here; useful only when strictly cap-safe.

## 7) Required final answers

1. **Does pull-lag beat fixed double under hard ceilings?**
   - In this simulation: **yes on score in tight feasible bands**, but **no unique feasibility unlock** below fixed-double.
2. **If yes, under what conditions?**
   - Tight feasible budgets (~1.75–2.0 slots), sequential predictable compute, lower transfer variance.
3. **Does fixed double remain default when memory is plentiful?**
   - **Yes**, once budget is comfortably above the near-cap threshold; keep it as baseline default.
4. **Does adaptive Kanban become useful under hard caps?**
   - **Conditionally**; only if it obeys hard caps and avoids oscillatory over-expansion.
5. **Is triple buffering feasible/worth it under pressure?**
   - Generally **not** under near-cap pressure; feasibility and value improve only with looser budgets.
6. **What should Prometheus implement or rake next?**
   - Keep fixed-double default, keep serial fallback, and run M33 threshold-rake for activating pull-lag under explicit pressure conditions.
7. **Should pull-lag be revived as guarded memory-pressure mode?**
   - **Yes**, as a guarded mode (not default), gated by budget pressure and variance regime checks.

## 8) M33 recommendation

M33 should isolate activation policy, not redesign controllers:
- define explicit “memory-pressure mode” entry/exit thresholds,
- gate pull-lag by budget headroom + transfer-variance confidence,
- require hard-cap safety proofs first,
- preserve fixed-double as default outside pressure mode.

## 9) Artifacts emitted

- `m32_memory_budget_sweep.octagon`
- `m32_strategy_feasibility_table.octagon`
- `m32_memory_inventory_table.octagon`
- `m32_sequential_workload_table.octagon`
- `m32_variance_sensitivity_table.octagon`
- `m32_final_recommendation.octagon`

## 10) Inconsistency note

M32 follows M25 candidate scope exactly (A–F) and introduces hard feasibility gating by design. No `Language/reference` syntax inconsistency was encountered during this pass.
