# FINDINGS: Prometheus Shadow Authority Rake Lab M5

## Question

M5 asks for the dispatch-time feedforward actuator contract after native M13 failed to converge end-to-end.

## Why M5 exists after failed/native M13 convergence

Native M13 had a reservation consume seam but did not fully own SGEMM feedforward dispatch path; this lab specifies deterministic validate-before-consume protocol.

## Contract decisions

Disabled never consumes. Validate shape/variant/capability/fallback/reason-binding/margin before consume. Consume only when feedforward is selected.

## Scenario results with exact counts

scenarioCount=16, expectedBehaviorMet=16, feedforwardUseCount=4, judgmentFallbackCount=12, consumedCount=4, noConsumeOnBlockedCount=12, contractViolationCount=0.

## Validate-before-consume conclusion

Consume must happen after validation and only when dispatch source is ShadowReservation.

## Dispatch precedence recommendation

Feedforward has precedence only after all gates and reservation validations pass; otherwise Judgment is mandatory fallback.

## Lookahead feedback recommendation

Lookahead applies only for enabled+healthy+margin+reason-binding pass with valid matured reservation context; it does not imply feedforward use by itself.

## Prestage diagnostic recommendation

PrestageAllowed remains diagnostic-only in M5; no transfer or pretransfer behavior is modeled.

## Native M13 retry instructions

1) Validate reservation context first. 2) Select ShadowReservation only on full pass. 3) Consume exactly once at dispatch commit. 4) Enforce tie-break by earliest target_tick then stable reservation id if equal.

## Limitations

M5 is Oct lab modeling only; no native Prometheus code changes, no runtime semantics changes, and no production-readiness claim.
