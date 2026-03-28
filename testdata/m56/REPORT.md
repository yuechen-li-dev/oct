# M56 — Interruption/Resume Pressure Test: Resume Slot vs Stack Semantics

This pressure run compares four conceptual approaches against realistic authored controller patterns:

- **A**: Flat explicit return wiring
- **B**: Manual remembered-state variable
- **C**: Single-slot resume mechanism (conceptual)
- **D**: Full stack semantics (conceptual `push/pop/replace`)

Scope artifacts:
- `testdata/m56/controller_sketches.md`
- `testdata/m56/comparison_matrix.md`

No implementation is proposed here.

## 1. Workload-by-Workload Findings

### Workload A — Tactical Interruption (Game-Shaped)

#### Workload
Authored tactical controllers where an NPC temporarily diverts (`InvestigateNoise`, `Reload`, `TakeCover`) and should resume the exact prior tactical mode (`Flank`, `Suppress`, `BurstFire`, etc.).

#### Flat Wiring Read
Understandable in small cases, but edge explosion appears quickly when many tactical modes can be interrupted by several short-lived behaviors. Return wiring becomes repetitive and noisy.

#### Manual Resume Variable Read
Works today with explicit state (`resume_target`) and guard logic. Truthful, but repetitive save/clear/validate handling is error-prone and verbose under many interruption points.

#### Single-Slot Resume Read
Strong fit. A bounded “resume the one interrupted mode” shape matches this tactical pressure directly without introducing open-ended hierarchy.

#### Full Stack Read
Handles nested chains elegantly, but adds hidden-depth semantics and encourages architecture centered on stack mechanics rather than authored tactical clarity.

#### Outcome
**Single-slot resume mechanism clearly helps**

#### Main Reason
This workload has frequent **localized** interruption/resume pressure where one remembered return target is usually sufficient and materially reduces boilerplate without opening full HFSM complexity.

---

### Workload B — Industrial Temporary Override (Controller-Shaped)

#### Workload
Industrial controllers with temporary override behavior (`Inspection`, `SafetyHold`, `CalibrationCheck`) and potential return to prior operating mode if preconditions still hold.

#### Flat Wiring Read
Often acceptable and audit-friendly because many transitions are intentionally deterministic and explicitly reviewed.

#### Manual Resume Variable Read
Generally sufficient. Explicit remembered mode plus validity checks expresses conservative resume behavior while preserving safety intent.

#### Single-Slot Resume Read
Helpful in repeated override-heavy designs, but not universally necessary. Value depends on how often short temporary overrides recur.

#### Full Stack Read
Usually excessive for industrial flows; reduces audit readability and can obscure safety-critical transition intent.

#### Outcome
**Manual remembered-state variable is enough**

#### Main Reason
Industrial patterns often prioritize auditable explicitness over compactness; a manual remembered target already covers most legitimate resume needs.

---

### Workload C — False Pressure Check

#### Workload
Cases that look like “resume previous,” but should restart deterministically (`fault -> RecoverInit`, operator rebaseline requests).

#### Flat Wiring Read
Best expression. Explicit restarts communicate safety and intent unambiguously.

#### Manual Resume Variable Read
Can accidentally preserve stale context when restart should be mandatory.

#### Single-Slot Resume Read
May be misapplied as convenience, encouraging incorrect resume behavior in fault paths.

#### Full Stack Read
Most dangerous overreach here; stack affordance normalizes returning into contexts that should be abandoned.

#### Outcome
**Flat explicit wiring is enough**

#### Main Reason
These patterns are not true resume requirements; they are explicit restart requirements.

---

### Workload D — Pathological Overreach Check

#### Workload
Deeply nested interruption chains and framework-shaped control trees that emerge when stack operations are widely available.

#### Flat Wiring Read
Higher friction, but friction usefully discourages over-architected nesting.

#### Manual Resume Variable Read
Still explicit and bounded; authors must justify each remembered context.

#### Single-Slot Resume Read
Bounded mechanism naturally limits depth and keeps control flow close to domain intent.

#### Full Stack Read
Too permissive. Encourages invisible ambient control context and architecture-first designs (“push everything”) rather than clear authored controller truth.

#### Outcome
**This pattern should not shape Octomata**

#### Main Reason
Designing around pathological depth would optimize for framework-style HFSM behavior that conflicts with Oct’s explicit minimal direction.

## 2. Strongest Real Resume Pressure

The strongest credible pressure is **tactical temporary diversion with exact prior-mode restoration**:

- engage in a tactical mode
- briefly divert for interruption behavior (noise check, dodge, reload, cover)
- return to the exact prior tactical mode

This occurs frequently enough in authored game-shaped controllers that pure flat wiring feels disproportionately noisy relative to behavioral intent.

## 3. Strongest False Stack Pressure

The strongest false pressure is **fault/rebaseline flows that appear to want “return where you were,” but should restart explicitly**.

These are correctness- and safety-oriented transitions where resume semantics are usually wrong; stack-like affordances would make misuse easier.

## 4. Minimal Useful Mechanism

**Choice: Add a single-slot resume mechanism**

Why this is the smallest truthful step:

1. It directly addresses the strongest real pressure (localized one-level interruption/resume).
2. It avoids broad hidden-context semantics from full stacks.
3. It can remain explicit and observable (slot set/clear/read behavior must be visible in control logic and telemetry).
4. It leaves restart-first fault handling flat and explicit.

## 5. Hard Non-Goals

If a single-slot direction is pursued, it must **not** become:

- **Behavior tree/framework culture**: no migration toward implicit large control frameworks.
- **Arbitrary nested control architecture**: no general push-depth semantics by stealth.
- **Hidden ambient control state**: resume context must remain explicit/inspectable, not magical.
- **Concurrency/scheduler coupling**: no interaction model with async schedulers, tasks, or event buses.

## 6. Recommended Next Step

**Design a single-slot resume feature**

Design target should stay narrow:

- one remembered return target only
- explicit set/clear/use semantics
- deterministic behavior under ordered `when`
- clear observability hooks so resume behavior stays auditable

If that narrow design cannot remain explicit, fall back to guidance-first manual resume patterns instead of expanding to full stacks.
