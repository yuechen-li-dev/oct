# M56 Comparison Matrix — A/B/C/D Approaches

Legend:
- **A** = Flat explicit return wiring
- **B** = Manual remembered-state variable
- **C** = Conceptual single-slot resume mechanism
- **D** = Conceptual full stack semantics

## A. Tactical Interruption (Game-Shaped)

### Readability
- **A:** Clear for one-off interruptions; degrades when many tactical modes all need explicit return edges.
- **B:** Moderate; explicit data helps but mode-mapping boilerplate accumulates.
- **C:** Strong; expresses “temporary diversion then resume last tactical mode” directly.
- **D:** Mixed; concise for nested chains, but implicit stack movement can obscure intent.

### Explicitness
- **A:** Highest explicitness; every edge is authored.
- **B:** High; resume target is ordinary state, fully inspectable.
- **C:** Good if slot is explicit and observable.
- **D:** Lower; hidden depth/context unless heavily instrumented.

### Boilerplate
- **A:** High in multi-mode tactical systems.
- **B:** Medium-high (save/restore guards, invalidation, fallback).
- **C:** Low-medium.
- **D:** Low for nested interruption flows.

### Misuse Risk
- **A:** Low.
- **B:** Medium (stale resume target bugs).
- **C:** Medium-low (single-slot bounds abuse).
- **D:** High (over-architected stack trees).

### Minimality
- **A/B:** Sometimes too manual for this pressure.
- **C:** Best fit to observed tactical pressure.
- **D:** Usually larger than needed.

**Outcome:** **Single-slot resume mechanism clearly helps**.

## B. Industrial Temporary Override (Controller-Shaped)

### Readability
- **A:** Often adequate because many industrial transitions are deterministic and audited.
- **B:** Good when “resume prior mode if valid” is needed.
- **C:** Good for temporary overrides with clear return intent.
- **D:** Usually unnecessary complexity.

### Explicitness
- **A/B:** Strong and audit-friendly.
- **C:** Acceptable if guarded by explicit validity checks.
- **D:** Weakens auditability unless constrained.

### Boilerplate
- **A:** Moderate.
- **B:** Moderate.
- **C:** Lower than B in repeated override patterns.
- **D:** Low but invites architectural sprawl.

### Misuse Risk
- **A:** Low.
- **B:** Medium (forgotten invalidation after faults).
- **C:** Medium-low (must still require precondition checks before resume).
- **D:** High (can bypass explicit safety reasoning).

### Minimality
- **A/B:** Often enough.
- **C:** Helpful for repeated temporary overrides.
- **D:** Too big for dominant cases.

**Outcome:** **Manual remembered-state variable is enough** (with C as optional ergonomic help in repetitive override-heavy controllers).

## C. False Pressure Check

### Readability
- **A:** Clearest; explicit restart paths communicate safety intent.
- **B:** Can blur whether restart vs resume was intended.
- **C:** Risks overusing resume where restart should be mandatory.
- **D:** Worst; stack affordance can normalize unsafe “return where you were.”

### Explicitness
- **A:** Best.
- **B/C/D:** Progressively more opportunity to hide wrong intent.

### Boilerplate
- **A:** Low for restart patterns.
- **B/C/D:** Adds unnecessary mechanism.

### Misuse Risk
- **A:** Lowest.
- **B:** Medium.
- **C:** Medium-high in safety/fault contexts.
- **D:** Highest.

### Minimality
- **A:** Correctly minimal.
- **B/C/D:** Overreach.

**Outcome:** **Flat explicit wiring is enough**.

## D. Pathological Overreach Check

### Readability
- **A/B:** Force authors to stay close to concrete domain transitions.
- **C:** Bounded; cannot model arbitrarily deep nesting.
- **D:** Encourages deeply nested, mechanism-shaped flow.

### Explicitness
- **A/B/C:** Maintain visible control intent.
- **D:** Hidden ambient stack context grows with depth.

### Boilerplate
- **A/B:** Higher, but friction acts as a healthy brake.
- **C:** Moderate.
- **D:** Lowest, which is exactly why overuse risk rises.

### Misuse Risk
- **A/B/C:** Contained.
- **D:** Severe over-architecture risk.

### Minimality
- **A/B/C:** Compatible with Oct’s explicit minimal style.
- **D:** Exceeds the stated problem.

**Outcome:** **This pattern should not shape Octomata**.
