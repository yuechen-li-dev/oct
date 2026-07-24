# Prometheus Stage 4 — resource state and execution handoff

## 1. Starting checkpoint and scope

Started from clean checkpoint `807c05d5017319bd2b002ab951dd14e3107b2263`
(`prometheus: classify Stage 3 M34b witness`). This pass clarifies the existing
SGEMM resource-state boundary only. It does not introduce a scheduler, allocator,
registry, reactor abstraction, or new resource lifecycle.

## 2. Architectural constitution

**Dominatus decides and coordinates. Vulkan mechanisms execute and report facts.**

Dominatus remains the authority for the committed SGEMM path/compute, layout/
precision, transfer, buffering, and lease/admission decisions. `prom_vk_runtime`
remains a common Vulkan mechanism owner. SGEMM remains the typed owner of its
activation roles, command resources, descriptor use, dispatch, submission,
timing, readback, and execution-specific cleanup.

## 3. Deferred-live boundary

No validated Gemma checkpoint was available. Required-live Gemma, fresh
payload-backed allocation/teardown, checkpoint-dependent Z-Image, and Linux
Vulkan remain unclaimed. Missing payload is setup, not an architectural result.

## 4. Pre-change resource-state inventory

| Concept | Current authority / writers | Consumers and lifetime | Classification |
| --- | --- | --- | --- |
| Runtime handle | private `g_active_handles` table; add/remove/validate in `reactor_vulkan_sgemm.c` | all public SGEMM entry points; removed before free | mechanical bookkeeping |
| SGEMM A/B/C and upload arenas | `prom_typed_arena`, artifact keys, and `prom_vk_buffer` helpers | direct or staged buffers through command completion; reverse cleanup | mechanical allocation bookkeeping |
| SGEMM role/slot | `prom_slot_hfsm` and Dominatus lease facts/decision | selected work slot, prepare/swap/submit/complete/release | semantic transition through Dominatus; local C mechanics retained |
| Path, precision, transfer, buffering | Dominatus staged facts, committed visible snapshots, judgment output | SGEMM buffer choice and command path | authoritative control state |
| Descriptor inputs and pipeline | SGEMM maps selected typed A/B/C resources to existing bindings 0/1/2 | command recording only | SGEMM-specific mechanical binding |
| Submission, fence, timing, readback | SGEMM | command buffer through completion/readback | SGEMM-specific execution state |
| M46/M49 weights, identity/hash/generation | transformer/M46/M49 path, not scalar SGEMM | required-weight validation and model execution | typed model/weight state; deliberately untouched |

No existing SGEMM scalar-buffer field is treated as a model weight identity. No
logical tensor identity is treated as an arena allocation, a Vulkan buffer, a
descriptor binding, or an in-flight lease.

## 5. Control state versus mechanical state

Control state is the committed Dominatus decision and its traceable transition:
path/compute mode, layout/precision acceptance, transfer policy, buffering mode,
and resource-lease grant. Mechanical state is the concrete buffer/arena record,
Vulkan handle, mapped range, command buffer, descriptor set, fence, and cleanup
record. Vulkan capability and completion observations remain facts reported into
the established Dominatus surfaces; no Vulkan mechanism selects policy.

## 6. Identity, generation, hash, and binding map

| Meaning | Representation | Deliberate non-equivalence |
| --- | --- | --- |
| SGEMM artifact compatibility | `prom_buffer_artifact_key` | not a content hash or logical tensor name |
| Arena reuse epoch | `prom_typed_arena.generation` | not an M46 weight generation or slot generation |
| Submission incarnation | `prom_sgemm_submission_slot.generation` | not a descriptor identity or allocation identity |
| Async ownership | task ID plus physical slot/generation | not a lease decision or resource role |
| Descriptor binding | bindings 0/1/2 and `VkDescriptorBufferInfo` | not ownership or residency policy |
| Model weight identity/hash/generation | M46/M49 typed model contract | not generalized into the SGEMM arena substrate |

The unresolved M46-to-M49 generation/hash handoff, including `-7406`, remains
unchanged and outside this scalar-SGEMM ownership clarification.

## 7. Dominatus authority before and after

Before, M35 buffering was committed to the blackboard and mirrored to diagnostics,
but the following mechanical branch still read the local `buffering_decision`.
After, that branch reads `buffering_snapshot`, the committed visible Dominatus
snapshot, for success, rejection detail, and selected mode. The values and order
are otherwise unchanged.

Path/compute, layout/precision, transfer, and lease already followed their
existing stage → commit → visible projection/decision → commit path and remain
so. The mechanism still has no policy fallback hidden in the handoff.

## 8. Mechanical resource ownership before and after

`prom_vk_runtime` still owns common instance/device/queue/command-pool handles,
capabilities, validation facts, and deterministic owner cleanup. SGEMM still owns
its buffers, descriptor resources, pipelines, command buffers, fences,
submissions, timing, dispatch, readback, and reverse teardown before common-owner
cleanup. Allocation sizes, memory types, residency, reuse, pinning, and cleanup
order were not changed.

## 9. SGEMM-specific roles deliberately retained

The A, B, C, and upload arena roles, direct/staged buffer choices, descriptor
bindings, compute mode, selected shader variant, and submission-ring slot remain
SGEMM-specific. They were not made a supposedly universal FFT/reduction/ray-
tracing resource model. Model activations, transformer weights, acceleration
structures, and other reactor resources remain distinct typed concerns.

## 10. Handle and arena consolidation

No structural consolidation was required. The handle table already has one local
validation/lookup/add/remove implementation, and `destroy_all_execution_buffers`
already centralizes deterministic reverse buffer cleanup. Replacing either with a
generic registry would widen scope without proving duplicated behavior. Arena keys
and role checks remain local mechanical validation, not a semantic state machine.

## 11. Bounded execution handoff

After the existing Dominatus decision snapshots, SGEMM creates one private,
immutable-by-convention `prom_sgemm_execution_handoff`. It contains only resolved
mechanical facts: M/N/logical-K/compute-K, selected path/mode/variant, selected
slot, transfer wait dependency, already-selected pipeline and descriptor set,
descriptor buffer ranges, and dispatch geometry. Command binding, push constants,
dispatch, and the submission wait dependency consume this value. The pre-existing
Dominatus lease grant remains the final admission edge before dispatch; command
recording order was not moved.

The handoff neither scores candidates nor changes variants, bindings, capacity,
residency, leases, or policy. Existing audit-only override behavior remains the
same pre-existing mechanical pipeline/geometry input.

## 12. Construction, execution, and destruction preservation

Construction still configures the Stage 3 common runtime first and SGEMM-owned
resources second. Execution keeps existing descriptor update, transfer recording,
barriers, command recording, fence submission, completion, readback, and lease-
yield order. Destruction remains SGEMM execution resources first, then the Stage 3
common runtime owner. Partial construction uses the existing deterministic cleanup
path; no resource is newly released while referenced by SGEMM execution.

## 13. Exact production files changed

| File | Change |
| --- | --- |
| `internal/prometheus/native/reactor_vulkan_sgemm_internal.h` | private bounded handoff value and explicit identity distinctions |
| `internal/prometheus/native/reactor_vulkan_sgemm.c` | consume committed M35 snapshot; create and consume the handoff at lease-to-dispatch |

Test-only: `internal/prometheus/native/Marionette/reactor_dominatus_sgemm_adapter_tests.cpp`.
No production file was deleted.

## 14. ABI, generated, shader, and package preservation

The canonical authority check remains PASS: 84 exported symbols, signature digest
`89053790ac5a18d29a21141527e017efc2faa03932d3adc2307891fdb8da0262`, 69
public structs, unchanged detail authority including `-7406`, package
`prometheus.core@1`, and kernel identities `kernel-68-default` and
`kernel-69-default`. No generated header, shader source/bytes, model projection,
manifest, lock, package membership, or native-source manifest changed. The Stage
3 package manifest reference hash remains
`A110CEBC3ABC737BB450C53D5F2A5ED46CDD7C48DFD300688A0AB567A64EF19C`.

## 15. Inherited M34b fingerprint

`PrometheusM34bValidationEnabledProductionVariants` remains an **EXPECTED
INHERITED FAIL**: `m=3`, `n=17`, `k=7`, variant 4, final column of all three
rows, expected `1.6458333730697632`, observed `0` (three assertions). The
related A2x4 footprint witness remains the same five assertions: expected
Z/row/column coverage `1/2/4/16/32`, observed `2/4/16/32/128`. Neither witness
was changed, suppressed, or reinterpreted.

## 16. Validation performed

| Lane | Result |
| --- | --- |
| checkpoint / clean baseline | PASS before edits |
| repository authority and Stage 0 static check | PASS |
| required-live skip self-test | PASS |
| Windows native build | PASS (existing compiler warnings only) |
| focused Stage 4 handoff test | PASS |
| focused Dominatus SGEMM adapter tests | PASS, 27 tests |
| Stage 0 ABI/detail snapshot | PASS |
| M34b production variants | EXPECTED INHERITED FAIL, exact fingerprint above |
| A2x4 footprint witness | EXPECTED INHERITED FAIL, exact fingerprint above |
| live Gemma / checkpoint Z-Image / fresh payload teardown | SKIP / NOT RUN |
| Linux live Vulkan | NOT RUN |

## 17. Observation limitations

The available no-payload suite proves source-level and focused native transition
preservation, but cannot prove fresh checkpoint-backed weight teardown or assign
the M46/M49 `-7406` writer. It also does not turn a Windows live lane into a Linux
claim.

## 18. Rollback boundary

Revert this single Stage 4 commit to remove the private handoff value, restore
the local M35 decision read, and remove the focused test/report/index/handoff
updates. No public ABI, generated output, shader/package data, payload, or model
projection is part of the rollback boundary.

## 19. Remaining coupling and ambiguity

SGEMM still intentionally owns its typed role binding and full GPU execution
workflow. The M46/M49 active-versus-required weight generation/hash ambiguity,
Q/K role lease semantics, fresh teardown evidence, package/static-registry
projection gap, and repeated `MainTransformer1` successor projection remain
unresolved and untouched.

## 20. Exact Stage 5 candidate boundary

Stage 5 may only investigate a small shared *mechanical* resource substrate if
real shared allocation/cleanup duplication is demonstrated with preserved
payload-backed tests. It must not begin a generic reactor resource model, change
weight semantics, allocation/residency policy, or any existing Dominatus decision.

## Result

PROMETHEUS STAGE 4: SUCCESS
