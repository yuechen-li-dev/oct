# Prometheus Stage 6 — closed model execution-plan boundary

## 1. Starting checkpoint and scope

Stage 6 started from the clean Stage 5 checkpoint
`aa84d2ff3c3e86a96a5150e0760cd91114385cfc` (`prometheus: extract mechanical allocation cleanup`).
HEAD matched that commit and the worktree was clean before this investigation.

This is an evidence-only boundary decision.  No production source, public ABI,
generated projection, model lock, shader, package, allocation, binding, weight,
or numerical contract changed.

## 2. Architectural constitution and closed-plan definition

Dominatus-in-C remains the Prometheus control kernel: semantic lifecycle,
admission, authorization, committed choice, progress, retry, and completion
must live in or pass through Dominatus.  Vulkan mechanisms execute an already
authorized operation and report facts; they do not schedule or advance model
work.

A Stage 6 closed plan would be finite, lock-topology-authoritative, fully
validated before execution, immutable after construction, and distinct from
logical bindings, allocations, committed execution facts, and mutable progress.
It would describe operation identity, order/dependencies, logical roles,
weight references, and static requirements.  It would not be a graph compiler,
scheduler, allocator, policy engine, generic IR, or public API.

## 3. Deferred-live boundary

`OCT_EVT2_CACHE`, `OCT_EVT2_ORACLE`, `G4E2B_CHECKPOINT_ROOT`, and a configured
live reactor were not available for this pass.  Payload-independent structural
tests ran; no live Gemma, checkpoint-backed Z-Image, fresh payload teardown, or
Linux Vulkan execution is claimed.

## 4. Pre-change execution inventory and topology authority

The current Z-Image authority is `models/zimage-turbo/lock-tagon.octagon`,
its generated private projections `resolved_descriptor.h` and
`resolved_audit_schedule.h`, and the fixed native implementation in
`reactor_vulkan_model_block.c`.  The lock identity is
`71ef202b4e34b562bd0d8526d1e0c674640cbba02fb7c484d8dadf981c8b226e`.
It contains two NoiseRefiners, two ContextRefiners, and 30 MainTransformers.
The generated descriptive projection retains 29 repeated
`Successor: "MainTransformer1"` entries; it is not normalized here.

The observable weight-retarget sequence is finite: positions 0--33 select
NoiseRefiner0, NoiseRefiner1, ContextRefiner0, ContextRefiner1, then
MainTransformer0--29.  `compiled_model_owner_create_impl` binds the initial
NoiseRefiner0 and sets `retarget_position` to 1.  Both
`compiled_model_retarget_impl` and `compiled_model_prefetch_impl` independently
reconstruct positions 1--33 with positional branch ladders; successful
retarget/activation increments the Vulkan-session `retarget_position`.
`compiled_model_evaluation_reset_impl` returns that cursor to 0.

The full execution path is not that weight sequence alone.  Public veneer calls
separately create/capture/compose/execute the closed block facades, while
`reactor_vulkan_model_block.c` validates resident stream generations and records
the Vulkan workflow.  Gemma's M46/M49 APIs live in
`reactor_vulkan_transformer.c`; the M46-to-M49 required weight generation/hash
handoff, including `-7406`, is separate from the Z-Image session cursor.

## 5. Fact classification

| Fact | Classification | Current owner |
| --- | --- | --- |
| lock identity, resolved block family/local ID/parameter set, fixed stream roles, main dimensions | static topology fact | lock and generated descriptor projection |
| 34-position retarget order and duplicated selector ladders | static selection fact, currently implicit | Vulkan model-block implementation |
| repeated `MainTransformer1` successors | descriptive/disputed generated topology projection | lock/projection; preserved unchanged |
| upload content/layout identity, generation/hash, and stream generation | weight/content or SGEMM/model binding fact | model/transformer implementation |
| Vulkan buffers, descriptor sets, command resources, submission, fence, timing, teardown | mechanical Vulkan fact | model block and `prom_vk_runtime` |
| selected main-attention route and execution profile | committed execution/policy fact | current model-session code |
| `retarget_position`, active block, completion/quarantine | mutable semantic progress | currently Vulkan model-session state |
| M46/M49 lifecycle and `-7406` | semantic control/progress and weight fact | transformer runtime/API path |
| Stage 4 SGEMM handoff | committed per-operation mechanism fact | Dominatus SGEMM adapter then SGEMM |

The two selector ladders are real duplication.  More importantly, the mutable
Z-Image progress cursor is inside `prom_compiled_model_session_state`, which is
in the Vulkan model mechanism.  The only existing `prom_dom_blackboard` is
owned by the SGEMM runtime path.  There is no model-plan authorization,
completion, failure, or cursor adapter, and no Dominatus event/visible snapshot
that a model mechanism consumes before advancing.

## 6. Plan-extraction decision and evidence

**Rejected for this stage.**  A fixed private 34-entry table could replace the
two retarget/prefetch ladders and validate descriptor-family/local-ID/parameter
set/weight-count facts.  That would be a topology lookup refactor only.  It
would leave the actual execution API ordering external, leave the cursor in the
Vulkan mechanism, and provide no Dominatus authorization or completion edge.
Calling that table a closed *execution* plan would conceal the exact authority
gap Stage 6 is required to close.

Moving the cursor to Dominatus safely requires a dedicated, bounded model
authorization/observation seam with established lifecycle evidence across
create, capture, compose, retarget/prefetch activation, execution, failure,
reset, and teardown.  It cannot be substituted with a static table, and it
cannot be inferred from the SGEMM blackboard without changing unproven model
semantics.  No generic graph framework, scheduler, policy, or allocation layer
is justified.

## 7. Representation, validation, and immutability result

No representation was introduced.  Consequently, no plan claims immutability,
pre-execution validation, malformed-reference rejection, or a new destruction
contract.  Existing lock/descriptor validation remains authoritative and the
existing lock is immutable checked-in input.  The next owner boundary must
first prove a model operation authorization record whose static reference can be
validated before mechanism invocation and whose progress is Dominatus-owned.

## 8. Preserved execution, handoff, ownership, and lifecycle boundaries

No supported model path was migrated or retained by a new plan; all current
paths remain byte-for-byte on their existing route.  Operation order,
dependencies, repeated successor projection, tensor shapes/strides/offsets,
logical bindings, weight identities, generations, hashes, and binding behavior
are unchanged.

Stage 4's private `prom_sgemm_execution_handoff` was not touched.  Its committed
dimensions, bindings, variant, descriptor inputs, dispatch, and synchronization
facts remain bounded between Dominatus and SGEMM.  Stage 3 retains
`prom_vk_runtime` as common Vulkan owner.  Stage 5's policy-free buffer
creation/cleanup substrate is unchanged; no model semantic state enters it.
Existing dependent-object-before-runtime teardown and repeated cleanup behavior
are unchanged.

## 9. ABI, generated, package, and inherited witness preservation

The Stage 0 authority check reports 84 exports and ABI digest
`89053790ac5a18d29a21141527e017efc2faa03932d3adc2307891fdb8da0262`.
`prometheus.core@1`, package/source counts, generated/static projections, and
kernel identities `kernel-68-default` and `kernel-69-default` are unchanged.

The inherited M34b result remains an expected deterministic failure: m=3,
n=17, k=7, selected variant 4, all three final-column cells expected
`1.6458333730697632` and observed `0`; the five A2x4 footprint assertions
remain unchanged.  M46/M49 authority and `-7406` were not altered.

## 10. Tests and validation

| Lane | Result |
| --- | --- |
| clean Stage 5 checkpoint/worktree before changes | PASS |
| `go run ./tools/prometheus_stage0 -check` | PASS |
| native-manifest and compiled-model-lock checks | PASS |
| required-live skip-detection self-test | PASS |
| `go test ./internal/prometheus/... ./tools/prometheus_native_manifest ./tools/compiled_model_lock -count=1` | PASS |
| Windows native build | PASS (launcher exceeded command timeout but recorded exit code 0) |
| ABI/detail snapshot | PASS |
| focused Dominatus Stage 4 adapter tests | PASS, 27 |
| focused Stage 5 allocation/cleanup witness | PASS |
| payload-independent M2C model-block tests | SKIP: Vulkan unavailable; real lane also needs EVT-2 payloads |
| M34b production variants | EXPECTED INHERITED FAIL: exact 3-cell fingerprint |
| M34b A2x4 footprint | EXPECTED INHERITED FAIL: exact five assertions |
| payload-independent Gemma and Z-Image Go tests | PASS as part of Prometheus Go lane |
| live Gemma, checkpoint-backed Z-Image, fresh payload teardown, Linux | NOT RUN |
| `git diff --check` | PASS after documentation update |

## 11. Production files changed, rollback, and remaining coupling

No production files changed.  The only changes are this report, the evidence
index, and the current reviewer handoff.  Reverting this documentation-only
commit restores the prior reviewer boundary without affecting execution.

The named remaining coupling is the Vulkan-owned `retarget_position` and
separate API veneer ordering.  It is deliberately visible rather than replaced
by a shadow plan or a second scheduler.

## 12. Exact Stage 7 candidate boundary (not begun)

Stage 7 may investigate one narrow private Dominatus model-operation
authorization/observation seam for the already finite Z-Image session path.
It must first characterize existing create/capture/compose/retarget/execute/
failure/reset ordering, then let Dominatus authorize exactly one existing
operation and consume its reported completion before advancing.  Only after
that evidence exists may it extract the 34-entry static lookup into an
immutable closed execution plan.  It must not change topology, repeated
successor projection, allocations, bindings, weights, routes, ABI, shaders,
packages, kernels, M46/M49, `-7406`, or numerical behavior.
