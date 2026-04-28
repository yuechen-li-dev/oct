# P13 M9 — Dominatus Resource-Lease Controller Integration

## 1) M49 handoff summary

M49 compared feedforward-only variant selection, HFSM-only resource control, and a hybrid model. The hybrid won by combining recipe specialization with runtime flow control. The key handoff is: kernel variants are local recipes; runtime lease control is the real controller.

## 2) Why existing Dominatus/HFSM/Judgment/Blackboard machinery is used

M9 adds lease control as a Dominatus slice using existing patterns:

- facts staged into the blackboard,
- committed to visible state,
- deterministic judgment decision,
- decision staged + committed,
- visible diagnostics consumed by tests.

No parallel scheduler framework was added.

## 3) Lease lifecycle model

Implemented lifecycle states:

- `NONE`
- `REQUESTED`
- `GRANTED`
- `HELD`
- `YIELDED`
- `DENIED`
- `FAILED`

M9 focuses on request/grant/deny/yield seams and diagnostics.

## 4) Lease facts and decisions

Added compact lease facts and decisions to judgment-engine style contracts:

- worker/slot/entry identity
- selected occupancy recipe variant
- resource class request (compute/transfer/memory-bandwidth)
- pressure classes
- outstanding depth + limits
- lookahead requested + cap
- slot readiness/failure/invalidation masks
- unsafe-to-reuse and transfer overlap gates

Decision outputs include grant/deny state, reason code, lookahead allowed/blocked, backpressure, allowed outstanding depth, and selected recipe variant.

## 5) Judgment Engine integration

Added deterministic lease arbiter:

- `prom_judgment_engine_decide_resource_lease(...)`

Deterministic deny reasons include:

- slot failed
- slot invalidated
- unsafe runtime
- outstanding-depth cap
- transfer unavailable
- resource pressure

Yield requests emit `YIELDED` state deterministically.

## 6) Blackboard / Dominatus integration

Added SGEMM Dominatus keys for lease facts, lease decisions, and counters:

- grant/deny/backpressure/yield counters
- lookahead blocked reason
- lease decision state + reason

The adapter exposes:

- fact staging
- visible fact projection
- decision staging
- visible diagnostics snapshot

## 7) Slot HFSM integration

M9 uses existing slot-readiness masks from blackboard commit boundaries as lease inputs:

- `ready_slot_mask`
- `failed_slot_mask`
- `invalidated_slot_mask`
- `attention_slot_mask`

Failed/invalidated slot masks deny lease grants.

## 8) Occupancy selector / recipe integration

Lease facts carry `selected_recipe_variant` from existing occupancy decision contracts. M9 does not actuate kernel dispatch changes.

## 9) Lookahead bounded actuator behavior

Lookahead is controller-bounded:

- allowed only when `current_outstanding_depth < lookahead_limit`
- blocked at cap, with explicit blocked reason diagnostics

## 10) Diagnostics added

Visible diagnostics include:

- lease state, grant, deny reason
- worker/slot/entry identity
- outstanding depth and allowed depth
- lookahead requested/allowed/blocked reason
- selected recipe variant
- granted/denied/backpressure/yield/failed counters

## 11) Tests added

Native Marionette tests cover:

- happy-path grant
- failed/invalidated deny cases
- outstanding-depth backpressure and lookahead blocking
- unsafe-runtime deny
- yield transition + yield counter
- staged-visible Dominatus commit behavior and deterministic judgment path

## 12) Behavior intentionally unchanged / minimal actuation

M9 is diagnostics-first with minimal safe actuation:

- grant/deny/yield controller seam
- no dispatch rewrite
- no new kernel implementation
- no broad scheduling refactor

## 13) Deferred scope

Explicitly deferred:

- new SGEMM kernels
- dispatch actuation based on occupancy recipe
- full transfer prefetch / Smith Predictor
- work stealing / SPMC / MPMC
- response-surface tuning / autotune
- benchmark-based policy shifts and public perf claims
