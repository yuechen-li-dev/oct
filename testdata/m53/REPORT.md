# M53 — Utility `when` Pressure Test

This run pressure-tests utility-shaped `when` as a **small deterministic decision layer** against realistic controller/AI authoring. It compares current ordered guarded choice with conceptual score-based choice (plus optional hysteresis/min-commit), without assuming implementation exists.

Probes used for ordered baseline examples: `testdata/m53/utility_when_pressure_examples.octest`.

---

## 1. Workload-by-Workload Findings

### A. Competing Tactical Priorities (game-shaped)

#### Workload
Chase vs attack vs recover vs retreat with fluctuating visibility/range/ammo/health.

#### Ordered `when` Read
Readable when priorities are fixed, but guard ordering starts carrying hidden weighting intent (e.g., retreat and recover are hard-coded above attack/chase). Near-equal tactical choices are hard to express without nesting and duplicated guard combinations.

#### Utility `when` Read
A score table makes intent explicit:

```oct
let next = when utility {
    case Retreat when lowHealth score 100
    case Recover when lowAmmo score 70
    case Attack when inRange score 60
    case Chase when targetVisible score 40
    else Search
}
goto next
```

This reads as “relative desirability” instead of “procedural ordering trick.”

#### Outcome
**Score-Based Utility Clearly Helps**

#### Main Reason
Competing tactics are naturally comparative; scoring surfaces that comparison directly and avoids brittle order-only encoding.

---

### B. Industrial Stability / Anti-Thrashing (controller-shaped)

#### Workload
Heat/hold/cool/fault controller with noisy thresholds around hot/cool boundaries.

#### Ordered `when` Read
Fault-first ordering is clear, but stability logic becomes verbose: manual state-memory guards, duplicated threshold bands, and ad hoc “don’t switch yet” checks scattered across states.

#### Utility `when` Read
Scoring plus local stickiness policy captures intent:

```oct
let mode = when policy { hysteresis: 8 min_commit: 3 } {
    case Fault when faulted score 100
    case Cool when tempHigh score 60
    case Hold when tempNominal score 55
    else Hold
}
goto mode
```

#### Outcome
**Hysteresis/Min-Commit Clearly Helps**

#### Main Reason
The core pain is oscillation under noisy signals, not branch readability. Stability knobs local to decision sites are materially cleaner than re-encoding stickiness manually.

---

### C. Looks Utility-ish but ordered is enough

#### Workload
Simple fault-first alert/chase/attack sequencing with static obvious priorities.

#### Ordered `when` Read
Direct and clear: `fault > engage > alert > idle`. No ambiguity and no near-ties.

#### Utility `when` Read
Adds numbers that do not carry extra meaning; readers still infer static priority, now through arbitrary score values.

#### Outcome
**Ordered `when` Is Enough**

#### Main Reason
No comparative tradeoff or instability exists; utility layer is decorative overhead.

---

### D. Oscillation / Stickiness Cases

#### Workload
Two near-equal modes flipping each step (e.g., keep target vs switch target) with brief spikes.

#### Ordered `when` Read
Requires bespoke memory checks and explicit temporal guards in each branch. Easy to get wrong and hard to audit for all transitions.

#### Utility `when` Read
Deterministic best-score selection + hysteresis + minimum commitment duration directly encode desired “stay unless clearly beaten” behavior.

#### Outcome
**Hysteresis/Min-Commit Clearly Helps**

#### Main Reason
This is the strongest language-level pressure: stability is a first-class control concern that ordered-first-true cannot represent tersely.

---

### E. Pathological Overreach Check

#### Workload
Large rule matrices, sprawling weighted tables, and pseudo-framework decision graphs.

#### Ordered `when` Read
Verbose but bounded; pressure is obvious when complexity gets too high.

#### Utility `when` Read
Can become opaque quickly (“magic numbers everywhere”), encouraging blackboard/event-like architecture pressure.

#### Outcome
**Too Broad / Should Not Be Encouraged**

#### Main Reason
Past a small local decision surface, utility rulesets become framework-shaped and violate Octomata’s minimal control-language goals.

---

## 2. Strongest Real Pressure

**Oscillation/stickiness under near-equal choices (Workload D)** is the strongest real pressure. Ordered choice expresses priority but not stability policy; utility with hysteresis/min-commit captures “remain active unless meaningfully outscored” directly and deterministically.

---

## 3. Strongest False Pressure

**Static fault-first sequencing (Workload C)** is the strongest false pressure. It can look “AI-ish,” but fixed priorities are already best expressed as ordered guards. Utility adds noise, not value.

---

## 4. Minimal Useful Surface

Chosen option:

**score-based `when` + tie-break + hysteresis + min-commit**

Justification:
- Score-only helps tactical comparisons but does not solve control thrash.
- Deterministic tie-break is mandatory for replayability and auditability.
- Hysteresis handles small oscillations between close alternatives.
- Min-commit blocks one-tick spikes from causing immediate mode churn.

Anything broader (external memory, rule engines, callback scoring) crosses into framework territory.

---

## 5. Recommended Policy Shape

**Explicitly attached as local policy metadata** (not deeply inline per case, not global runtime policy).

Rationale:
- Keeps policy visible at the exact decision site.
- Keeps `case` lines focused on guards and scores.
- Avoids hidden ambient behavior while preserving readability.

---

## 6. Hard Non-Goals

Utility `when` must **not** become:
- a blackboard-based decision system
- an event-bus/reactive framework
- stochastic selection or fuzzy inference
- user-defined callback scoring runtime
- behavior-tree replacement
- a general planner/goal engine

It should remain a small deterministic local-choice primitive.

---

## 7. Recommended Next Step

**Implement full first-step utility (score + tie-break + hysteresis + min-commit).**

Reason: this is the smallest complete shape that addresses both comparative-choice readability and real stability pressure without introducing framework-scale semantics.
