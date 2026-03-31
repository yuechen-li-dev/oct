# Oct State Model (Preliminary)

This note captures the current design boundary observed in the SignalLab M0–M5 experiments. It documents emerging structure; it does **not** claim final language/runtime design.

## State domains

### A) Records
**Use for:** numeric/data state, time/value/history buffers, deterministic payload pipelines.

**Properties:**
- immutable by default
- explicit
- strong fit for data-heavy lanes

Records remain the primary representation for durable, replayable data flow.

### B) Octomata
**Use for:** behavioral/control progression, mode transitions, interruption/resume semantics, policy/arbitration logic.

**Properties:**
- explicit `flow` / `state` / `when` semantics
- strong fit for behavior contracts
- not intended as universal data storage

Octomata is the behavioral contract layer, not a replacement for structured data records.

### C) Blackboard (proposed, not implemented)
**Use for:** behavior-local working memory, control/display/alert status, fault latches, resume targets, policy/hysteresis/commit memory.

**Properties:**
- flow-local
- typed
- fixed-shape
- explicit
- not a general mutable object model

The blackboard is an emerging complement to records and Octomata, intended for local machine memory that is awkward in pure immutable records.

## Preliminary architectural boundary

For nontrivial interactive applications, the current default architecture is:
- **Records** for numeric/data lanes
- **Octomata** for behavioral/control contracts
- **Blackboard** for behavior-local working memory

This boundary is an empirical conclusion from SignalLab probes, not yet a universal law.

## Provisional blackboard shape recommendation (from M5)

Preferred shape:
- one explicit, typed board per flow instance
- fixed fields declared up front
- write access only inside flow/state contexts
- explicit boundary handoff at flow edges
- no dynamic keys
- not usable as a general-purpose mutable escape hatch

Rejected directions:
- generic mutable locals everywhere
- typed slot/key registry model
- framework-style context object that hides writes behind methods

## Dirty-key tracking note (future implementation guidance)

If/when blackboard is implemented, internal dirty-key tracking is the preferred runtime architecture for scalability/performance. The tracking should remain an internal runtime behavior, not a noisy source-level responsibility, unless future evidence shows source-level exposure is necessary.

## Why this matters

This separation avoids forcing behavior-local machine state into either (1) broad generic mutation or (2) purely immutable data records. It keeps data pipelines deterministic, behavioral contracts explicit, and local control memory intentionally scoped.

## Status and caution

- This is a **preliminary** design conclusion.
- It is based on SignalLab **M0–M5** experimentation.
- No implementation is being claimed here.
- Future experiments may refine the boundary and blackboard shape.
