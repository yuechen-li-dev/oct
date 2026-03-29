# REPORT

## 1) Goal

This experiment compares two technical shapes for the same control problem:

- a language-native control model (Octomata)
- an external state-machine DSL style embedded in a host language

The objective is a runnable, concrete comparison that starts at finite-state parity and then grows into controller-grade behavior.

## 2) Baseline finite-state example (Phase A)

`Mx69/external_dsl_comparison_mx69.oct` includes `MatterParityMachine`, a three-state matter model (`Solid`, `Liquid`, `Gas`) with transitions `melt`, `freeze`, `vaporize`, `condense`.

This phase is intentionally small: it demonstrates parity with classic FSM-style transition tables and ordered guards.

## 3) Extended control case (Phase B)

The same milestone extends the scenario using native Octomata constructs:

- `MatterNativeController` uses utility `when` with `hysteresis` and `min_commit` policy.
- `MatterUtilityMinCommitProbe` and `MatterUtilitySwitchAfterCommitProbe` isolate anti-thrashing behavior.
- `MatterServiceInterruptionDemo` demonstrates interruption and restoration with `remember` / `resume`.

This keeps one conceptual model while moving from plain state transitions to stability-aware controller decisions.

## 4) Observed differences

### Control-logic locality

In this experiment, selection logic, transition logic, interruption logic, and completion outcomes all remain in the flow/state bodies.

### Host-language ceremony

No builder scaffolding is required before writing control logic. The flow directly states behavior rather than constructing it indirectly through host callbacks.

### Decision-model richness

The model expresses:

- ordered `when` transitions (parity FSM)
- utility scoring decisions
- hysteresis and minimum-commit windows
- interruption and resume routing

without switching frameworks or abstractions.

### Observability

Tests use native probes to validate runtime behavior:

- `Active(...)`
- `Complete(...)`
- `Result(...)`
- `StateHistory(...)`
- `ResumeTarget(...)`

### Architectural scaling

The same language-native surface scales from a basic matter FSM to anti-thrashing mode control and interruption handling without a model handoff.

## 5) Conclusion

A language-native control model can subsume simple external-DSL-style FSM expression and extend naturally into richer controller semantics. In this experiment, the progression from basic transitions to stability policy and resumable interruption stays within one coherent execution model and one test surface.
