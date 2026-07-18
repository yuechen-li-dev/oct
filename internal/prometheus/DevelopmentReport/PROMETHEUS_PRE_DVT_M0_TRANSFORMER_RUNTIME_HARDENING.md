# Prometheus Pre-DVT M0: Transformer Runtime Hardening

## Status

Convergence outcome: **MEANINGFUL PROGRESSION**  
Milestone state: **IN PROGRESS**  
DVT state: **NOT READY**

This is the source-ownership baseline for M0.  It is deliberately a ledger,
not a claim that a source-file move preserves runtime behavior.  The live
Vulkan transformer implementation remains coupled to the M39b reduction
runtime state and must be extracted through a narrow private runtime contract.

## Pre-refactor responsibility inventory

| Fused-reduction range | Current responsibility | Required owner |
|---|---|---|
| 36-512 | M39b/M40b constants, shared runtime state, Vulkan slot and pipeline storage | reduction core plus a narrow shared runtime contract |
| 783-2913 | M42-M46 plans, CPU references, comparison helpers | transformer block planning/reference |
| 2924-4770 | M39b plans, lifecycle, slots, descriptors, command recording, M40b compatibility | fused reduction subsystem |
| 4770-7278 | M42-M47 pipeline/resource preparation and standalone recording | transformer block subsystem |
| 7279-7615 | M48 persistent layer weights and initial activation preparation | transformer stack subsystem |
| 7616-10531 | M43-M47 grouped attention, projection, residual, RMSNorm, and FFN recording | transformer block subsystem |
| 10532-12247 | fixed four-block stack, ping-pong, descriptor banks, submission, replay, faults, quarantine/reap, and M49b live integration | transformer stack subsystem |
| 12248-14331 | standalone composed continuations and M49a audit routes | transformer block subsystem; audit-only routes remain explicit |
| 14332-14772 | M39b execution, diagnostics, CPU oracle, benchmark helper | fused reduction subsystem |

The classification above is based on callers, slot ownership, pipeline
ownership, and destruction order.  In particular, the M48 code cannot be
textually separated today because `prom_reduction_runtime_state` and
`prom_reduction_slot` co-own reduction and transformer buffers, descriptor
pools, command buffers, fences, and the M49b controller.

## Preserved baseline ledger

- Public APIs remain declared by `reactor_api.h`; no public API rename is in scope.
- Production reduction shader IDs are `16..22`; implementation IDs are
  `1001..1007`.
- The fixed stack is bounded by `PROM_M48_LAYER_COUNT`; it owns four command
  buffers, three semaphores, two ping-pong activations, and a slot-local
  controller canary readback.
- M42-M47 query ranges, descriptor-bank capacities, push-constant layouts,
  fault names, resource generations, and replay hash inputs are preserved as
  source contracts.  No replay identity has been regenerated for this work.
- Historical M49b evidence identifies `reactor_vulkan_fused_reduction.c` as
  fixed-stack owner and `reactor_vulkan_transformer_control.c` as pure policy.

## Required extraction order

1. Establish a private runtime contract that gives transformer block/stack
   code explicit access to shared Vulkan services, slots, and reduction softmax
   recording without duplicating `prom_reduction_runtime_state`.
2. Move M42-M47 resource preparation and block recording into one block owner.
3. Move M48 preparation, stack submit/completion, fault, quarantine, and reap
   ownership into one stack owner.
4. Move M49b live integration beside its policy unit only after the stack
   owner is independent of fused-reduction state.
5. Delete transitional wrappers only after Windows and Linux builds plus the
   focused native lifecycle suites pass.

## Lifecycle that an extraction must preserve

`runtime state -> shared Vulkan pools/layouts/pipelines -> persistent layer
parameters -> bounded slots -> slot command buffers/semaphores/fences ->
slot working buffers -> host readbacks`.

An uncertain completion quarantines the entire slot until its fence is reaped;
known record-time failures recycle it.  Descriptor banks are slot-local and
must not be mutated while their command buffer can be in flight.

## DVT source navigation (current baseline)

| Failure | Current source owner | Primary test surface |
|---|---|---|
| short-row softmax regression | `reactor_vulkan_fused_reduction.c` plan selector | `Marionette/reactor_reduction_tests.cpp` |
| RMSNorm arithmetic mismatch | fused runtime M46 recorder plus `reactor_vulkan_transformer.c` reference | `Marionette/reactor_attention_tests.cpp` |
| descriptor reuse across blocks | fused runtime M48 slot/descriptor bank | `Marionette/reactor_attention_tests.cpp` |
| canary mismatch | fused M49b live integration plus `reactor_vulkan_transformer_control.c` | `Marionette/reactor_m49_numerical_research_tests.cpp` |
| quarantine/reap failure | fused M48 stack lifecycle | `Marionette/reactor_attention_tests.cpp` |
| generated SPIR-V capability mismatch | `shaders/manifest.json` and SDSL-V production source | shader manifest/workspace checks |

This table becomes the final DVT handoff map only after the owners named in
the middle column have been physically extracted.

## Unsupported claims

No Windows or Linux rebuild, hardware smoke, warm-allocation proof, replay
comparison, or deterministic post-extraction artifact is claimed by this
baseline document.  Those facts must be captured after live code moves.
