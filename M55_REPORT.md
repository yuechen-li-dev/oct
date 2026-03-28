# M55 — Octomata Utility Proof Packages + Stack-Semantics Pressure Probe

## 1. What Was Built

### A) Game-shaped utility proof: `SquadUtilityProofM55`
A tactical squad controller package with explicit Octomata states (`Patrol`, `Evaluate`, `EngageClose`, `EngageFlank`, `Reload`, `Hold`, `Retreat`) plus a utility-driven decision site in `Evaluate`.

The package includes:
- a utility controller using `when policy { hysteresis, min_commit }`
- an ordered-only comparison controller
- native `.octest` coverage for progression, tie behavior, stability, and determinism

### B) Industrial-controller-shaped utility proof: `ThermalPlantUtilityProofM55`
A thermal process controller package with explicit states (`Boot`, `Control`, `Heat`, `Cool`, `Hold`, `Fault`) plus a utility-driven control site in `Control`.

The package includes:
- a utility controller with score ranking, hysteresis, and min-commit
- an ordered-only comparison controller
- native `.octest` coverage for progression, anti-thrashing behavior, safety precedence, and determinism

## 2. What Utility Improved

1. **Score-based choice removed brittle guard ordering in real authored decisions.**
   - In the game package, utility picks `EngageFlank` when flank urgency materially exceeds close-threat urgency, while ordered-only logic remains biased to first true close-threat guard.
   - In the industrial package, utility picks `Cool` when cooling urgency dominates, while ordered-only logic still picks `Heat` due to source order.

2. **Hysteresis improved stability under noisy near-tie conditions.**
   - In both packages, a small score lead by an alternate mode did not cause immediate flip-flop.

3. **Min-commit made mode holding explicit and readable.**
   - Both controllers express “stay committed briefly before reconsidering” as policy, rather than ad-hoc guard gymnastics.

4. **Authoring quality improved.**
   - Controller intent reads as “rank candidate actions by urgency and stability policy” instead of manually encoding order-sensitive branching logic.

## 3. What Friction Remains

Only friction observed while authoring these proofs:

1. **Flat-state interruption handling requires explicit return wiring.**
   Temporary interruptions (for example, investigate/recover micro-behaviors) still require manually routing back through the normal control state.

2. **No native “resume prior mode” concept.**
   If an interruption should restore the immediately prior tactical mode, flat `goto` requires explicit reconstruction logic rather than direct context restoration.

3. **Policy is local to decision sites (good), but cross-site context restore is still manual.**
   Utility solves local selection quality, not context-stack behavior across nested sub-behaviors.

## 4. Stack-Semantics Pressure Assessment

**Some localized stack pressure, but not enough to justify a feature**

Why:
- Utility `when` addressed the major awkwardness around priority selection and anti-thrashing.
- Remaining awkwardness appears mainly in interruption/resume structure, not in decision scoring.
- In these two proofs, that pressure is real but limited in scope and did not force unmaintainable designs.

Distinction:
- **Solved by utility:** score ordering, tie stability, and small-oscillation dampening.
- **Not solved by utility:** “temporarily push sub-behavior, then resume prior behavior exactly” patterns.

## 5. Strongest Real Stack Candidate (if any)

Strongest candidate: **game-side temporary interruption while preserving tactical context**.

Concretely, a short investigate/reposition behavior that should resume whichever engagement mode (`EngageClose` vs `EngageFlank`) was active before interruption. In flat replacement form this requires explicit return mapping logic; stack semantics could express this more directly.

## 6. Recommended Next Step

**Run one dedicated stack-semantics pressure test**

Rationale: utility now clearly improves authored controllers, and only localized interruption-resume pressure remains. One targeted pressure test can validate whether that pressure is strong enough, repeated enough, and costly enough to justify introducing stack semantics.
