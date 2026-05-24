# P15 M11 — Shadow would-act diagnostics (reason-binding arbitration)

## Purpose

M11 adds diagnostic-only would-act/would-block counters that estimate how often shadow authority would have acted if enabled, without granting any runtime authority.

## Relation to M8/M9/M10 and rake lab M3

- M8 provides per-outcome shadow snapshot facts.
- M9 provides calibration and de-dup keyed by issued/target/predicted ticks.
- M10 provides authority gate state/reason.
- M11 consumes M8/M9/M10 and applies reason-binding-style suppression to avoid HEALTHY over-promotion under recent stale/fallback and high arrival-error evidence.

## State and counters

Added `prom_dominatus_shadow_would_act_state` with bounded fixed-width counters for:

- evaluations, would-act, would-block, and gate-state class counts
- blocked-reason counts (low confidence, high miss rate, high arrival error, recent fallback, recent stale, insufficient samples, invalid calibration, lookahead disabled)
- suppression counters for healthy-overpromotion guards
- last decision fields and dedup keys

## Reason-binding arbitration

M11 reason arbitration order (diagnostic-only):

1. invalid calibration
2. lookahead disabled / recent fallback
3. recent stale
4. high arrival error
5. high miss rate
6. low confidence
7. insufficient samples
8. none

If M10 reports healthy/canary but a binding reason is active, M11 suppresses would-act and increments suppression/block counters.

## De-dup rule

M11 counts once per prediction key:

- `issued_tick + target_tick + predicted_ready_tick`

Repeated exports with the same key do not increment counters.

## SGEMM integration point

M11 runs immediately after M10 gate evaluation in the valid-timing P15 path and before diagnostics export.
Invalid timing does not call the update path, so M11 counters are not advanced on invalid timing.

## Tests

Marionette tests cover:

- defaults
- healthy/canary would-act
- blocked low-confidence
- high-arrival-error suppression
- stale/fallback binding suppression
- de-dup behavior

## Explicit non-goals

- no dispatch authority changes
- no selector tuning changes
- no lease authority changes
- no prestage/pretransfer authority changes
- no production behavior changes
