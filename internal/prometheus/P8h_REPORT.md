# P8h Report — Pre-M16 Narrow Policy Memory Addition

## 1) What was added

This pass adds a tiny reusable policy-memory helper in native C:

- `reactor_policy_memory.h`
- `reactor_policy_memory.c`

And a judgment-engine seam wrapper:

- `prom_judgment_engine_update_policy_mode(...)` in `reactor_judgment_engine.*`

The helper introduces:

- explicit mode enum (`AGGRESSIVE`, `SAFE`, `RECOVERY`)
- explicit policy memory (`current_mode`, `decisions_in_mode`, `cooldown_remaining`, `recovery_cooldown_remaining`)
- deterministic update function with no hidden globals

## 2) Mapping to M15 guardrails

The new helper directly supports required M15 guardrail classes:

- retreat-entry anti-chatter via hysteresis bands
- min-commit dwell to block rapid flip-flop
- cooldown/hold windows for retreat and recovery transitions

## 3) Hysteresis implementation

Hysteresis is mode-aware and threshold-pair based:

- `retreat_enter_permille` / `retreat_exit_permille`
- `recovery_enter_permille` / `recovery_exit_permille`

Transition decisions depend on retained prior mode (`current_mode`) plus current facts.

## 4) Min-commit implementation

`decisions_in_mode` is incremented each decision (saturating).  
Mode transitions are blocked until `decisions_in_mode >= min_commit_decisions`, except hard overrides.

## 5) Cooldown implementation

Simple integer countdowns are used:

- `cooldown_remaining` (safe retreat cooldown / aggressive re-entry gate)
- `recovery_cooldown_remaining` (minimum recovery hold window)

Both timers decrement saturating each decision and gate transitions.

## 6) Why HFSM was not required

M16-required stabilization is achieved with bounded state memory + threshold logic + counters.  
No hierarchy, nesting, event runtime, or generic HFSM machinery is necessary for this step.

## 7) How this enables M16

M16 can now implement the waste-budgeted speculation controller with explicit cross-call memory primitives already available:

- retained mode
- hysteresis
- min-commit
- cooldown/hold

without expanding into a generalized state-machine runtime.
