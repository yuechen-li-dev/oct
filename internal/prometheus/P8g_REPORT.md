# P8g Report — Pre-M16 Tool Check Pass (Judgment Engine Readiness)

## Scope and intent

This pass is a **pre-M16 readiness audit** only. It does **not** implement M16 controller behavior and does **not** introduce an HFSM runtime.

The question answered here is whether the current C-layer judgment/policy tooling can express M16 guardrails with narrow additions.

## 1) Current policy/state machinery audit

### What exists today

Current reusable policy machinery is concentrated in `reactor_judgment_engine.*`:

- deterministic SGEMM path/compute candidate selection from explicit facts
- explicit feasibility/fallback gating
- explicit async submission accept/reject policy seam
- deterministic winner diagnostics (`winning_candidate_index`, `winning_score`)

The reactor runtime (`reactor_vulkan.c`) currently owns persistent runtime state for:

- Vulkan resources and capabilities
- async lifecycle state (`IDLE/SUBMITTED/READY/FAILED/CONSUMED`)
- in-flight ownership and failure details

### What does not exist today (policy memory)

There is no persistent judgment-engine policy memory across SGEMM decisions:

- no retained prior policy mode
- no dwell/min-commit counters
- no cooldown timers
- no hysteresis enter/exit threshold pairs

The SGEMM judgment function is currently stateless per call.

## 2) Hysteresis readiness judgment

### Current status

The current judgment engine can encode single-threshold switching (`work_units >= threshold`) but not hysteresis bands (separate enter/exit thresholds) because it has no cross-call memory.

### Evidence

`PrometheusJudgmentEngine_HasNoCrossCallHysteresisOrCommitmentMemory` demonstrates immediate threshold-edge switching:

- below staging threshold -> direct
- at staging threshold -> staged
- immediately below threshold again -> direct

This confirms no retained-mode anti-chatter behavior exists yet.

## 3) Min-commit / cooldown readiness judgment

### Current status

Current judgment-engine tooling does **not** support:

- stay-in-mode-for-N-decisions
- retreat hold windows
- recovery cooldown windows
- bounded anti-flip-flop commitment

Reason: no per-mode counters/timers and no retained mode memory in the judgment seam.

## 4) M15 guardrail -> tooling capability mapping

### Already expressible with current tooling

- **bounded overlap / ownership rejection**: already present via async in-flight guards and explicit reject details.
- **waste-budget scoring shape at one instant**: candidate scoring can prefer/penalize choices for one decision cycle.
- **lag-aware pending-waste and burst dampening integration points**: fact-gathering seam exists (new facts can be passed in).

### Not cleanly expressible yet

- **retreat-entry hysteresis**: needs retained prior mode + enter/exit thresholds.
- **budget-threshold anti-chatter**: needs retained mode memory across successive decisions near threshold boundaries.
- **cooldown-limited recovery ramp / hold windows**: needs per-mode dwell/cooldown counters or timers.

## 5) HFSM necessity judgment

For the selected controller shape (aggressive/safe/recovery, bounded overlap, waste budget, retreat/recovery behavior), a full generalized HFSM runtime is **not** required at this stage.

M16 needs bounded policy memory and explicit thresholds/counters, not hierarchical runtime orchestration.

The required behavior is cleanly expressible as:

- explicit mode enum
- retained previous mode
- hysteresis threshold pairs
- min-commit/cooldown counters
- deterministic transition rules

## 6) Selected outcome

**Outcome B** — current judgment engine is close but not sufficient; add narrow reusable hysteresis/min-commit support first, then proceed to M16.

## 7) Smallest clean additions required before M16

### Exact features

Add a tiny reusable policy-memory helper for mode retention:

1. retained mode field (`current_mode`)
2. mode dwell counter (`decisions_in_mode`)
3. cooldown counter(s) (`recovery_cooldown_remaining`, optional per-mode)
4. hysteresis threshold pair support (enter vs exit)

### Exact scope

Keep this narrow and adjacent to the judgment seam:

- either in `reactor_judgment_engine.*` directly
- or a small helper next to it (`reactor_policy_memory.*`) used by the judgment engine

Prefer passing explicit facts + mutable memory struct into a deterministic selector function.

### Exact non-goals

Do **not** build yet:

- generalized HFSM runtime
- broad event framework
- nested state-machine architecture
- cross-domain policy meta-framework

Only add bounded memory primitives required for M16.

## 8) Inconsistency/documentation note

P8f intentionally scoped the judgment seam as a deterministic stateless selector and explicitly deferred runtime orchestration. M16 now requires bounded cross-call policy memory (hysteresis/min-commit), so this pass identifies that as a deliberate next narrow extension rather than an architectural contradiction.
