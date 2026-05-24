# Prometheus P15 M8 — Shadow HFSM Diagnostic Model

## Purpose
P15 M8 introduces a diagnostic-only shadow lifecycle model that mirrors the predictive lease-ahead path and compares delayed shadow readiness against physical readiness observations. It adds no dispatch/control authority.

## State vocabulary
Shadow HFSM states:
- UNKNOWN
- IDLE
- FORECAST_ISSUED
- FUTURE_LEASE_REQUESTED
- RESERVED
- PRESTAGE_ELIGIBLE
- PREDICTED_READY
- MATURED
- CANCELLED
- STALE
- FALLBACK

Mismatch/outcome kinds:
- NONE
- MATCH
- LATE
- EARLY
- PHYSICAL_NOT_READY
- SHADOW_NOT_READY
- CANCELLED
- STALE
- FALLBACK
- HARD_GATE
- INVALID_PREDICTION

Matched/missed are modeled as outcomes (`mismatch_kind`, `matched`) rather than separate lifecycle states.

## Transition model
Inputs are existing P15 separated facts:
- predictor issue/mature events,
- future lease + reservation decisions,
- prestage diagnostics,
- fallback/correction status,
- current tick.

The model advances deterministically through forecast/lease/reservation/prestage/maturity phases and classifies cancelled, stale, and fallback conditions without changing any runtime authority.

## Physical-vs-shadow comparison
At maturity, shadow expected readiness is compared with physical readiness from correction/mature events.
- `arrival_error_ticks = actual_ready_tick - predicted_ready_tick` where available.
- `mismatch_kind` classifies match, early/late readiness, and misses.

## SGEMM integration point
SGEMM policy diagnostics now export a separate `p15_shadow_*` snapshot family. Existing predictor, lease, reservation, prestage, and correction fields remain unchanged and separate.

## Test coverage
Added Vulkan-free Marionette tests for:
- defaults,
- issue/lease-reservation/prestage transitions,
- match/miss classification,
- arrival error classification,
- cancelled/stale/fallback states.

## Explicit non-goals
- no dispatch authority,
- no pre-stage action enablement,
- no selector behavior change,
- no production dispatch change.
